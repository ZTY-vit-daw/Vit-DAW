package executionverifiers

import (
	"context"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

// StaticPanBalance verifies one bounded track pan adjustment (FAM3-S1) with
// the same fresh post-action acoustic observation pattern as StaticBalance.
// Its structural postcondition reads the pan position from the VSP track
// snapshot (pan/pan_value/balance, 0.001 tolerance like every native D1
// readback); the acoustic side is the domain-agnostic VerifyStaticBalance
// contract, unchanged — the pan-specific balance semantics live in the
// observation face (track.stereo_space), not in this verifier.
type StaticPanBalance struct {
	State    StateReader
	Acoustic AcousticVerifier
}

func (v StaticPanBalance) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if v.State == nil {
		return result, fmt.Errorf("state reader is required")
	}
	if len(receipts) != len(actionSet.Actions) {
		result.Structural = "fail"
		return result, fmt.Errorf("receipt coverage mismatch")
	}
	for _, receipt := range receipts {
		if !strings.EqualFold(receipt.Status, "applied") {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s was not applied", receipt.ActionID)
		}
	}
	state, err := v.State.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return result, fmt.Errorf("read structural postcondition: %w", err)
	}
	tracks := indexTracks(state.LegacyState["tracks"])
	for _, action := range actionSet.Actions {
		row := tracks[action.TargetRef]
		if row == nil {
			result.Structural = "fail"
			return result, fmt.Errorf("target track %s missing after execution", action.TargetRef)
		}
		target, ok := number(action.Args["target_pan"])
		if !ok {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s target_pan missing", action.ID)
		}
		actual, ok := firstNumber(row, "pan", "pan_value", "balance")
		if !ok || math.Abs(actual-target) > 0.001 {
			result.Structural = "fail"
			return result, fmt.Errorf("track %s pan postcondition failed: got %.3f want %.3f", action.TargetRef, actual, target)
		}
	}
	result.Structural = "pass"
	result.EvidenceRefs = []string{"vsp.state.snapshot:postcondition"}
	if v.Acoustic == nil {
		result.Acoustic = "unsupported"
		return result, nil
	}
	acoustic, err := v.Acoustic.VerifyStaticBalance(ctx, actionSet)
	result.Acoustic = strings.TrimSpace(acoustic.Status)
	result.EvidenceRefs = append(result.EvidenceRefs, acoustic.EvidenceRefs...)
	result.Summary = strings.TrimSpace(acoustic.Summary)
	result.ObservationID = strings.TrimSpace(acoustic.ObservationID)
	result.ObservationRevision = strings.TrimSpace(acoustic.ObservationRevision)
	result.Fresh = acoustic.Fresh
	result.PostAction = result.ObservationID != "" && acoustic.Fresh
	result.SpecialistRelationship = strings.TrimSpace(acoustic.RelationshipStatus)
	result.SpecialistSummary = strings.TrimSpace(acoustic.RelationshipSummary)
	if err != nil {
		return result, err
	}
	if strings.EqualFold(result.Acoustic, "pass") {
		result.Status = "pass"
	} else if strings.EqualFold(result.Acoustic, "fail") {
		result.Status = "fail"
	} else {
		result.Status = "inconclusive"
	}
	return result, nil
}
