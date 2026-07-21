package executionverifiers

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

const (
	spalReferenceEQProviderRegistrationCapabilityID = "spal.reference_eq_provider_registration.v0"
	spalReferenceEQProviderRegistrationCommand      = "spal.reference_eq_provider.register"
)

// SPALProviderRegistration verifies a durable Provider registration.  It has
// no acoustic claim: the only automated result is that the exact conformed
// record was persisted after a current parameter readback.  User acceptance is
// intentionally unknown because registration itself is not a mixing result.
type SPALProviderRegistration struct{}

func (SPALProviderRegistration) Verify(_ context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "fail", Structural: "fail", Acoustic: "not_applicable", UserAcceptance: "unknown"}
	if actionSet.CapabilityID != spalReferenceEQProviderRegistrationCapabilityID || len(actionSet.Actions) != 1 || len(receipts) != 1 {
		return result, fmt.Errorf("SPAL Provider registration receipt coverage mismatch")
	}
	action := actionSet.Actions[0]
	if action.Command != spalReferenceEQProviderRegistrationCommand {
		return result, fmt.Errorf("SPAL Provider registration action command is invalid")
	}
	receipt := receipts[0]
	if receipt.ActionID != action.ID || !strings.EqualFold(receipt.Status, "applied") {
		return result, fmt.Errorf("SPAL Provider registration action was not applied")
	}
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(receipt.Details["structural_readback"])), "pass") {
		return result, fmt.Errorf("SPAL Provider registration has no passing structural readback")
	}
	if strings.TrimSpace(fmt.Sprint(receipt.Details["provider_record_id"])) == "" || strings.TrimSpace(fmt.Sprint(receipt.Details["provider_instance_id"])) == "" {
		return result, fmt.Errorf("SPAL Provider registration receipt omits its durable binding")
	}
	result.Status = "pass"
	result.Structural = "pass"
	result.EvidenceRefs = append([]string(nil), receipt.EvidenceRefs...)
	result.Summary = "verified Provider record persisted after current parameter readback; no musical result is implied"
	return result, nil
}
