package executionports

import (
	"context"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/workflows/plugingrabber"
)

// ThresholdDeltaToleranceDB mirrors the chat EQ machine's threshold physical
// tolerance (eqPhysicalTolerance): the achieved readback must land within this
// band of current+delta for the receipt to count as applied.
const ThresholdDeltaToleranceDB = 0.15

// deltaSemantics marks actions whose target_value is a bounded physical delta
// (physical target = current + target_value) instead of an absolute target.
// The broadband compression domain is the first consumer: its threshold
// display probe is structurally degenerate (read_only_value_to_string answers
// the CURRENT text at every synthetic normalized sample — VSC-2 measured
// 5x "+11.8" on 2026-08-28), so curve inversion is impossible and the machine
// discovers the local normalized->physical slope transactionally instead.
const deltaSemantics = "delta_db"

type deltaChannelPlan struct {
	Channels        []eqGainChannel
	CurrentPhysical float64
	TargetPhysical  float64
	Slope           float64
	Probe           map[string]any
}

func (plan *deltaChannelPlan) audit() map[string]any {
	if plan == nil {
		return nil
	}
	return map[string]any{
		"current_physical":        plan.CurrentPhysical,
		"target_physical":         plan.TargetPhysical,
		"slope_db_per_normalized": plan.Slope,
		"probe":                   plan.Probe,
	}
}

// rebaseAfterWrite refreshes the CAS base after a probe-class kernel write
// (the same rebase the instantiate path performs: every write-like command
// advances the revision and the next write must present the fresh base).
func (p *StaticEQVSPPort) rebaseAfterWrite(ctx context.Context) error {
	snapshot, err := p.Client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || snapshot == nil || !snapshot.OK() {
		return fmt.Errorf("delta probe rebase snapshot failed: %w", err)
	}
	if snapshot.ProjectEpoch != p.projectEpoch {
		return fmt.Errorf("project epoch changed during delta probe")
	}
	if snapshot.Revision <= p.baseRevision {
		return fmt.Errorf("delta probe did not advance the project revision")
	}
	p.baseRevision = snapshot.Revision
	return nil
}

func (p *StaticEQVSPPort) writeParameterBatch(ctx context.Context, trackID, pluginID, requestID, txID string, entries ...map[string]any) error {
	result, err := p.Client.SendVSPCommandWithIDs(ctx, "plugin.set_params_batch", map[string]any{
		"track_id":      trackID,
		"plugin_id":     pluginID,
		"parameters":    entries,
		"readback":      true,
		"base_revision": p.baseRevision,
	}, requestID, txID)
	if err != nil {
		return err
	}
	if failure := eqTypedCommandFailure(result); failure != "" {
		return fmt.Errorf("%s", failure)
	}
	return nil
}

// planDeltaChannels plans every channel at normalized
// current + delta/slope, discovering the slope with one small reversible
// probe write on the primary channel: write, read the live surface text,
// restore. The probe and restore are real kernel mutations inside this one
// orchestration action (the instantiate rebasing pattern), each rebasing the
// CAS base for the next write. If the display text never responds, no
// normalized target can be derived and the action fails closed without a
// net parameter move.
func (p *StaticEQVSPPort) planDeltaChannels(ctx context.Context, trackID, pluginID, requestID, txID string, surface map[string]plugingrabber.ParameterInfo, deltaDB float64, paramIDs ...string) ([]eqGainChannel, *deltaChannelPlan, error) {
	primary := strings.TrimSpace(paramIDs[0])
	info, ok := surface[primary]
	if !ok || primary == "" {
		return nil, nil, fmt.Errorf("delta parameter %q was not present in the plugin parameter surface", primary)
	}
	currentNormalized, ok := numeric(info.NormalizedValue)
	if !ok {
		return nil, nil, fmt.Errorf("delta parameter %q has no numeric normalized value", primary)
	}
	currentPhysical, ok := plugingrabber.ParseCompressorPhysical("threshold", info.ValueText)
	if !ok {
		return nil, nil, fmt.Errorf("delta parameter %q display %q is not a parsable threshold value", primary, info.ValueText)
	}
	targetPhysical := currentPhysical + deltaDB
	for _, offset := range []float64{-0.25, 0.25} {
		probeNormalized := math.Max(0, math.Min(1, currentNormalized+offset))
		if probeNormalized == currentNormalized {
			continue
		}
		if err := p.writeParameterBatch(ctx, trackID, pluginID, requestID+":probe", txID+":probe",
			map[string]any{"parameter_id": primary, "normalized_value": probeNormalized}); err != nil {
			return nil, nil, fmt.Errorf("delta probe write failed: %w", err)
		}
		if err := p.rebaseAfterWrite(ctx); err != nil {
			return nil, nil, err
		}
		probePhysical, probeParsed := probeThresholdPhysical(ctx, p, trackID, pluginID, primary)
		restoreErr := p.writeParameterBatch(ctx, trackID, pluginID, requestID+":probe-restore", txID+":probe-restore",
			map[string]any{"parameter_id": primary, "normalized_value": currentNormalized})
		if restoreErr != nil {
			return nil, nil, fmt.Errorf("delta probe restore failed: %w", restoreErr)
		}
		if err := p.rebaseAfterWrite(ctx); err != nil {
			return nil, nil, err
		}
		if !probeParsed || probePhysical == currentPhysical {
			continue
		}
		slope := (probePhysical - currentPhysical) / (probeNormalized - currentNormalized)
		if slope == 0 || math.IsNaN(slope) || math.IsInf(slope, 0) {
			continue
		}
		targetNormalized := currentNormalized + (targetPhysical-currentPhysical)/slope
		if targetNormalized < 0 || targetNormalized > 1 {
			return nil, nil, fmt.Errorf("delta target %.4g dB (current %.4g + %.4g) is outside the reachable normalized range", targetPhysical, currentPhysical, deltaDB)
		}
		channels := make([]eqGainChannel, 0, len(paramIDs))
		for _, paramID := range paramIDs {
			if id := strings.TrimSpace(paramID); id != "" {
				channels = append(channels, eqGainChannel{ParamID: id, RequestedNormalized: targetNormalized})
			}
		}
		plan := &deltaChannelPlan{
			Channels: channels, CurrentPhysical: currentPhysical, TargetPhysical: targetPhysical, Slope: slope,
			Probe: map[string]any{"parameter_id": primary, "probe_normalized": probeNormalized,
				"probe_physical": probePhysical, "current_normalized": currentNormalized, "restored": true},
		}
		return channels, plan, nil
	}
	return nil, nil, fmt.Errorf("parameter %q display does not respond to writes; physical calibration unavailable", primary)
}

func probeThresholdPhysical(ctx context.Context, p *StaticEQVSPPort, trackID, pluginID, paramID string) (float64, bool) {
	fresh, err := p.eqParameterSurface(ctx, trackID, pluginID)
	if err != nil {
		return 0, false
	}
	info, ok := fresh[paramID]
	if !ok {
		return 0, false
	}
	return plugingrabber.ParseCompressorPhysical("threshold", info.ValueText)
}
