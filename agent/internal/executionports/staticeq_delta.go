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
	// CurrentNormalized is the pre-action normalized position of the primary
	// channel; the refinement loop restores to it on exhaustion.
	CurrentNormalized float64
	TargetPhysical    float64
	Slope             float64
	Probe             map[string]any
	// Stepped is non-nil when the primary parameter is a discrete stepped
	// control: the plan writes the nearest reachable grid position directly
	// (no slope probe — an off-grid probe value snaps and measures a
	// meaningless average) and the refinement loop must not run (the nearest
	// position is already the physical optimum a stepped control can hold).
	Stepped *eqSteppedPlan
}

// eqSteppedPlan records how a stepped control's nearest-position write maps
// onto the requested delta. The deviation is inherent to the control's
// granularity (e.g. an API-550A -1.5 dB request lands on the -2 dB detent);
// it is disclosed on the receipt, never silently absorbed.
type eqSteppedPlan struct {
	PrimaryParamID    string
	Positions         int
	RequestedPhysical float64
	AchievedPhysical  float64
	DeviationDB       float64
}

func (plan *eqSteppedPlan) audit() map[string]any {
	if plan == nil {
		return nil
	}
	return map[string]any{
		"parameter_id":       plan.PrimaryParamID,
		"grid_positions":     plan.Positions,
		"requested_physical": plan.RequestedPhysical,
		"achieved_physical":  plan.AchievedPhysical,
		"deviation_db":       plan.DeviationDB,
	}
}

func (plan *deltaChannelPlan) audit() map[string]any {
	if plan == nil {
		return nil
	}
	out := map[string]any{
		"current_physical":        plan.CurrentPhysical,
		"target_physical":         plan.TargetPhysical,
		"slope_db_per_normalized": plan.Slope,
		"probe":                   plan.Probe,
	}
	if plan.Stepped != nil {
		out["stepped"] = plan.Stepped.audit()
	}
	return out
}

// eqSteppedPhysicalToleranceDB bounds how far a stepped readback may sit from
// the planned grid position's own physical value: both sides parse the same
// kernel display text family, so this only absorbs formatting wobble — a
// readback on any other detent fails.
const eqSteppedPhysicalToleranceDB = 0.05

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
	if grid, stepped := eqSteppedGrid(info); stepped {
		// Stepped control: the only reachable positions are the grid's own.
		// Write the nearest one directly; the slope probe below is useless
		// here (its off-grid probe value snaps to a detent, so the measured
		// "slope" is an artifact of the grid, not of a taper).
		position := eqNearestSteppedPosition(grid, targetPhysical)
		channels := make([]eqGainChannel, 0, len(paramIDs))
		for _, paramID := range paramIDs {
			if id := strings.TrimSpace(paramID); id != "" {
				channels = append(channels, eqGainChannel{ParamID: id, RequestedNormalized: position.Normalized})
			}
		}
		return channels, &deltaChannelPlan{
			Channels: channels, CurrentPhysical: currentPhysical, CurrentNormalized: currentNormalized,
			TargetPhysical: targetPhysical, Slope: 0,
			Probe: map[string]any{"parameter_id": primary, "planning": "stepped_grid", "grid_positions": len(grid)},
			Stepped: &eqSteppedPlan{
				PrimaryParamID: primary, Positions: len(grid),
				RequestedPhysical: targetPhysical, AchievedPhysical: position.Physical,
				DeviationDB: math.Abs(position.Physical - targetPhysical),
			},
		}, nil
	}
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
			Channels: channels, CurrentPhysical: currentPhysical, CurrentNormalized: currentNormalized,
			TargetPhysical: targetPhysical, Slope: slope,
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

// deltaRefineMaxIterations bounds the post-write secant convergence loop.
const deltaRefineMaxIterations = 4

// refineDeltaChannels converges a written delta onto its physical target. The
// planning probe measures one average slope over a ±0.25 normalized window,
// but real compressor display tapers bend near their limits, so the first
// write can land off target (VSC-2 threshold, 2026-08-30 p03 run: 10.8 dB
// target, 10.1 dB achieved from the +11.8 dB ceiling). Each iteration
// secant-projects through the two nearest measured (normalized, physical)
// points, writes every channel at the projected position, rebases the CAS
// base, and re-reads the live display text. Exhaustion restores the pre-action
// normalized value so a failed action leaves no net parameter move, matching
// the probe discipline.
func (p *StaticEQVSPPort) refineDeltaChannels(ctx context.Context, trackID, pluginID, requestID, txID string, plan *deltaChannelPlan, requested []eqGainChannel, primary string) (float64, float64, []map[string]any, error) {
	if len(requested) == 0 {
		return 0, 0, nil, fmt.Errorf("delta refinement has no channels to write")
	}
	// The main write advanced the kernel revision without rebasing the port's
	// CAS base (the pre-refinement flow performed no further writes); rebase
	// first so the kernel accepts the refinement writes.
	if err := p.rebaseAfterWrite(ctx); err != nil {
		return 0, 0, nil, fmt.Errorf("delta refine rebase failed: %w", err)
	}
	nA, physA := plan.CurrentNormalized, plan.CurrentPhysical
	nB, physB := requested[0].RequestedNormalized, math.NaN()
	if parsed, ok := probeThresholdPhysical(ctx, p, trackID, pluginID, primary); ok {
		physB = parsed
	} else {
		return 0, 0, nil, fmt.Errorf("delta refinement could not read the achieved threshold display")
	}
	writeAll := func(normalized float64, label string) error {
		entries := make([]map[string]any, 0, len(requested))
		for _, channel := range requested {
			entries = append(entries, map[string]any{"parameter_id": channel.ParamID, "normalized_value": normalized})
		}
		return p.writeParameterBatch(ctx, trackID, pluginID, requestID+":"+label, txID+":"+label, entries...)
	}
	trace := make([]map[string]any, 0, deltaRefineMaxIterations)
	for iteration := 1; iteration <= deltaRefineMaxIterations; iteration++ {
		if math.Abs(physB-plan.TargetPhysical) <= ThresholdDeltaToleranceDB {
			return nB, physB, trace, nil
		}
		slope := (physB - physA) / (nB - nA)
		if slope == 0 || math.IsNaN(slope) || math.IsInf(slope, 0) {
			break
		}
		next := math.Max(0, math.Min(1, nB+(plan.TargetPhysical-physB)/slope))
		if next == nB {
			break
		}
		if err := writeAll(next, fmt.Sprintf("refine-%d", iteration)); err != nil {
			return nB, physB, trace, fmt.Errorf("delta refine write failed: %w", err)
		}
		if err := p.rebaseAfterWrite(ctx); err != nil {
			return nB, physB, trace, err
		}
		achieved, parsed := probeThresholdPhysical(ctx, p, trackID, pluginID, primary)
		if !parsed {
			break
		}
		nA, physA = nB, physB
		nB, physB = next, achieved
		trace = append(trace, map[string]any{"iteration": iteration, "normalized": next, "physical": achieved})
	}
	if err := writeAll(plan.CurrentNormalized, "refine-restore"); err != nil {
		return nB, physB, trace, fmt.Errorf("delta refine restore failed: %w", err)
	}
	if err := p.rebaseAfterWrite(ctx); err != nil {
		return nB, physB, trace, err
	}
	return nB, physB, trace, fmt.Errorf("delta physical target %.4g dB not achieved after %d refinement iterations (readback %.4g dB); restored the pre-action position", plan.TargetPhysical, deltaRefineMaxIterations, physB)
}
