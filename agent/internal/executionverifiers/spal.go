package executionverifiers

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

// SPALSignalProbe is the seam between SPAL and the L2 offline-render probe.
// The production probe runner may be backed by DAD/MOM; the verifier only
// accepts its same-tap direction result and never interprets it as musical
// acceptance.
type SPALSignalProbe interface {
	VerifySPALSignal(context.Context, spal.ExecutionManifest, orchestration.ActionReceipt) (spal.SignalVerification, error)
}

// ReceiptSPALSignalProbe is the default durable verifier for the SPAL L2
// capture port. It reads evidence from a persisted Action Receipt instead of
// holding a transient before/after pair in memory.
type ReceiptSPALSignalProbe struct{}

func (ReceiptSPALSignalProbe) VerifySPALSignal(_ context.Context, manifest spal.ExecutionManifest, receipt orchestration.ActionReceipt) (spal.SignalVerification, error) {
	raw, ok := receipt.Details["spal_signal_evidence"]
	if !ok {
		return spal.SignalVerification{Status: "inconclusive", Summary: "no durable same-tap signal evidence was captured"}, nil
	}
	evidence, err := spal.SignalProbeEvidenceFromAny(raw)
	if err != nil {
		return spal.SignalVerification{Status: "inconclusive", Summary: "durable same-tap signal evidence is invalid"}, err
	}
	return evidence.Verify(manifest.Instruction.ExpectedSignalChange), nil
}

// SPAL verifies a frozen semantic parameter action in three distinct layers:
// parameter readback from the Receipt, optional signal direction evidence, and
// a deliberately unknown musical/user result.
type SPAL struct {
	Signal                     SPALSignalProbe
	RequireControlledRoundtrip bool
}

func (v SPAL) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if len(actionSet.Actions) == 0 || len(receipts) != len(actionSet.Actions) {
		result.Status = "fail"
		result.Structural = "fail"
		return result, fmt.Errorf("SPAL receipt coverage mismatch")
	}
	receiptByAction := make(map[string]orchestration.ActionReceipt, len(receipts))
	for _, receipt := range receipts {
		receiptByAction[receipt.ActionID] = receipt
	}
	manifests := make([]spal.ExecutionManifest, 0, len(actionSet.Actions))
	for _, action := range actionSet.Actions {
		receipt, ok := receiptByAction[action.ID]
		if !ok || !strings.EqualFold(receipt.Status, "applied") {
			result.Status = "fail"
			result.Structural = "fail"
			return result, fmt.Errorf("SPAL action %s was not structurally applied", action.ID)
		}
		if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(receipt.Details["structural_readback"])), "pass") {
			result.Status = "fail"
			result.Structural = "fail"
			return result, fmt.Errorf("SPAL action %s has no passing parameter readback", action.ID)
		}
		manifest, err := spal.ManifestFromAction(action)
		if err != nil {
			result.Status = "fail"
			result.Structural = "fail"
			return result, err
		}
		if receipt.Details["spal_manifest_id"] != manifest.ID || receipt.Details["binding_id"] != manifest.Binding.ID {
			result.Status = "fail"
			result.Structural = "fail"
			return result, fmt.Errorf("SPAL receipt does not match frozen manifest for action %s", action.ID)
		}
		if v.RequireControlledRoundtrip {
			if !controlledRoundtripRestored(receipt) {
				result.Status = "fail"
				result.Structural = "fail"
				return result, fmt.Errorf("SPAL controlled acceptance action %s did not restore its frozen preimage", action.ID)
			}
		}
		result.EvidenceRefs = appendVerifierRefs(result.EvidenceRefs, receipt.EvidenceRefs...)
		manifests = append(manifests, manifest)
	}
	result.Structural = "pass"
	signalRequested := false
	for _, manifest := range manifests {
		if manifest.Instruction.ExpectedSignalChange.IsRequested() {
			signalRequested = true
			break
		}
	}
	if !signalRequested {
		result.Acoustic = "not_requested"
		result.Summary = "parameter readback passed; no signal-direction check was requested; musical/user acceptance remains unknown"
		return result, nil
	}
	if v.Signal == nil {
		result.Acoustic = "unsupported"
		result.Summary = "parameter readback passed; no L2 same-tap signal probe is attached; musical acceptance remains unknown"
		return result, nil
	}
	for index, manifest := range manifests {
		if !manifest.Instruction.ExpectedSignalChange.IsRequested() {
			continue
		}
		probe, err := v.Signal.VerifySPALSignal(ctx, manifest, receiptByAction[actionSet.Actions[index].ID])
		result.EvidenceRefs = appendVerifierRefs(result.EvidenceRefs, probe.EvidenceRefs...)
		if err != nil {
			result.Acoustic = "inconclusive"
			result.Summary = withUnknownMusicalAcceptance("parameter readback passed; signal probe unavailable: " + err.Error())
			return result, nil
		}
		switch strings.ToLower(strings.TrimSpace(probe.Status)) {
		case "pass":
			result.Acoustic = "pass"
			result.Summary = probe.Summary
		case "mismatch", "fail":
			// The physical action and its readback already passed. A signal
			// direction mismatch is important evidence for review, but it is
			// not a reason to relabel successful parameter execution as a
			// failed mutation or to retry it automatically.
			result.Acoustic = "mismatch"
			result.Summary = withUnknownMusicalAcceptance(probe.Summary)
			return result, nil
		default:
			result.Acoustic = "inconclusive"
			result.Summary = withUnknownMusicalAcceptance(probe.Summary)
			return result, nil
		}
	}
	// Even a successful same-tap direction check says nothing about whether the
	// mix is artistically better. Keep the overall lifecycle in needs-review
	// until a user supplies a separate musical acceptance judgement.
	result.Status = "inconclusive"
	result.Summary = withUnknownMusicalAcceptance(result.Summary)
	return result, nil
}

func controlledRoundtripRestored(receipt orchestration.ActionReceipt) bool {
	if receipt.Details == nil {
		return false
	}
	confirmed, ok := receipt.Details["controlled_roundtrip_restored_preimage"].(bool)
	if !ok || !confirmed {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(receipt.Details["controlled_roundtrip_restore_status"])))
	return status == "restored"
}

func withUnknownMusicalAcceptance(summary string) string {
	summary = strings.TrimSpace(summary)
	const suffix = "musical/user acceptance remains unknown"
	if strings.Contains(strings.ToLower(summary), suffix) {
		return summary
	}
	if summary == "" {
		return "parameter readback passed; " + suffix
	}
	return summary + "; " + suffix
}

func appendVerifierRefs(base []string, values ...string) []string {
	seen := make(map[string]bool, len(base)+len(values))
	for _, value := range base {
		seen[value] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			base = append(base, value)
		}
	}
	return base
}
