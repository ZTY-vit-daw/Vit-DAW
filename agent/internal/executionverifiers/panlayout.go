package executionverifiers

import (
	"context"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

type PanAcousticVerifier interface {
	VerifyPanLayout(context.Context, orchestration.ActionSet) (AcousticResult, error)
}

type PanLayout struct {
	State    StateReader
	Acoustic PanAcousticVerifier
}

func (v PanLayout) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
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
		actual, actualOK := firstNumber(row, "pan", "pan_value", "balance")
		if !ok || !actualOK || math.Abs(actual-target) > 0.001 {
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
	acoustic, err := v.Acoustic.VerifyPanLayout(ctx, actionSet)
	result.Acoustic = strings.TrimSpace(acoustic.Status)
	result.EvidenceRefs = append(result.EvidenceRefs, acoustic.EvidenceRefs...)
	result.Summary = strings.TrimSpace(acoustic.Summary)
	if err != nil {
		return result, err
	}
	switch strings.ToLower(result.Acoustic) {
	case "pass":
		result.Status = "pass"
	case "fail":
		result.Status = "fail"
	default:
		result.Status = "inconclusive"
	}
	return result, nil
}
