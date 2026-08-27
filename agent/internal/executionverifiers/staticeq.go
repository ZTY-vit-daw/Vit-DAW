package executionverifiers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/orchestration"
)

// StaticEQ verifies one bounded static EQ band adjustment (D2-1) using the
// same fresh post-action acoustic observation pattern as StaticBalance. Its
// structural parameter readback postcondition is receipt-driven: the
// StaticEQVSPPort already performed the get_plugin_parameters readback under
// the same idempotency key as the mutation (plugin parameters are not visible
// in the VSP track snapshot), so this verifier asserts that evidence is
// self-consistent and bound to the current project revision.
type StaticEQ struct {
	State    StateReader
	Acoustic AcousticVerifierEQ
}

// AcousticVerifierEQ is the static_eq variant of AcousticVerifier; it shares
// the HarnessAcoustic fresh-observation machinery with static_eq wording.
type AcousticVerifierEQ interface {
	VerifyStaticEQ(context.Context, orchestration.ActionSet) (AcousticResult, error)
}

func (v StaticEQ) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
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
	for index, action := range actionSet.Actions {
		receipt := receipts[index]
		target, ok := number(action.Args["target_value"])
		if !ok {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s target_value missing", action.ID)
		}
		if receipt.Details["write_mode"] == executionports.WriteModeNormalizedBatchV1 {
			if !eqNormalizedChannelsVerified(receipt.Details["normalized_channels"]) {
				result.Structural = "fail"
				return result, fmt.Errorf("action %s plugin parameter normalized readback postcondition failed", action.ID)
			}
		} else {
			actual, ok := number(receipt.Details["actual_readback_value"])
			if !ok || receipt.Details["readback_verified"] != true || math.Abs(actual-target) > 0.001 {
				result.Structural = "fail"
				return result, fmt.Errorf("action %s plugin parameter readback postcondition failed", action.ID)
			}
		}
		appliedRevision, parseErr := strconv.ParseInt(strings.TrimSpace(receipt.AppliedRevision), 10, 64)
		if parseErr != nil || appliedRevision <= 0 || state.Revision != appliedRevision {
			result.Structural = "fail"
			return result, fmt.Errorf("action %s applied revision does not match the current project revision", action.ID)
		}
	}
	result.Structural = "pass"
	result.EvidenceRefs = []string{"vsp.receipt.readback:postcondition"}
	if v.Acoustic == nil {
		result.Acoustic = "unsupported"
		return result, nil
	}
	acoustic, err := v.Acoustic.VerifyStaticEQ(ctx, actionSet)
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

// eqNormalizedChannelsVerified checks the port's per-channel normalized
// readback records: every whitelisted gain channel must report an actual
// normalized value within the same 1e-4 tolerance the mature chat EQ
// transaction uses. The physical value_text parse stays informational.
func eqNormalizedChannelsVerified(value any) bool {
	rows, ok := value.([]any)
	if !ok || len(rows) == 0 {
		return false
	}
	for _, rowValue := range rows {
		row, ok := rowValue.(map[string]any)
		if !ok {
			return false
		}
		requested, requestedOK := number(row["requested_normalized"])
		actual, actualOK := number(row["actual_normalized"])
		if !requestedOK || !actualOK || math.Abs(actual-requested) > executionports.EqNormalizedTolerance {
			return false
		}
	}
	return true
}

// VerifyStaticEQ performs the same independent fresh post-action observation
// required for the D2-1 bounded static EQ band adjustment: a new observation
// id, explicit fresh CCB evidence, and revision binding. It reuses the B2
// typed static-level relationship projection (the changed band gain is a
// static level change) with D2-1 wording, since a single-band EQ action
// carries no B2 hierarchy metadata.
func (v HarnessAcoustic) VerifyStaticEQ(ctx context.Context, actionSet orchestration.ActionSet) (AcousticResult, error) {
	return v.verifyFreshMOM(ctx, actionSet, "static_level_relationship", verifyStaticEQMOM)
}

// verifyStaticEQMOM mirrors verifyStaticBalanceMOM with D2-1 wording. It
// consumes the same typed MOM projection and never falls through to generic
// multitrack fields or raw acoustic packages.
func verifyStaticEQMOM(relation map[string]any, actionSet orchestration.ActionSet) (string, string) {
	status := verifierStatus(relation)
	if status != mom.StatusReady && status != mom.StatusApprox && status != mom.StatusPartial {
		return "inconclusive", "D2-1 MOM static-level relationship is " + firstVerifierText(status, "missing")
	}
	compared := verifierRows(relation["tracks"])
	coverage := verifierMap(relation["coverage"])
	trackCount := verifierInt(coverage["track_count"])
	if trackCount == 0 {
		trackCount = len(compared)
	}
	if trackCount < 2 || len(compared) < 2 {
		return "inconclusive", "D2-1 MOM static-level projection omitted a comparable full-project track inventory"
	}
	usableCount := 0
	for _, row := range compared {
		rowStatus := verifierStatus(row)
		if rowStatus != mom.StatusReady && rowStatus != mom.StatusApprox {
			continue
		}
		if _, ok := verifierFirstFloat(row, "effective_static_rms_dbfs"); ok {
			usableCount++
		}
	}
	if float64(usableCount)/float64(trackCount) < 0.95 {
		return "inconclusive", fmt.Sprintf("D2-1 MOM usable static-level coverage is %d/%d", usableCount, trackCount)
	}
	indexed := verifierTrackIndex(compared)
	for _, action := range actionSet.Actions {
		row := indexed[strings.TrimSpace(action.TargetRef)]
		if row == nil {
			return "inconclusive", "D2-1 fresh MOM static-level projection omitted action target " + strings.TrimSpace(action.TargetRef)
		}
		if rowStatus := verifierStatus(row); rowStatus != mom.StatusReady && rowStatus != mom.StatusApprox {
			return "inconclusive", "D2-1 fresh MOM static-level target " + strings.TrimSpace(action.TargetRef) + " is " + firstVerifierText(rowStatus, "missing")
		}
	}
	return "pass", fmt.Sprintf("D2-1 typed static-level relationship verified for %d/%d usable tracks", usableCount, trackCount)
}
