package executionverifiers

import (
	"context"
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestSPALProviderRegistrationVerifierSeparatesStructuralFromMusicalResult(t *testing.T) {
	action := orchestration.Action{ID: "register-1", Command: spalReferenceEQProviderRegistrationCommand, TargetRef: "track:bass"}
	set := orchestration.ActionSet{
		ID: "set-1", CapabilityID: spalReferenceEQProviderRegistrationCapabilityID, ProjectCutHash: "cut", Actions: []orchestration.Action{action},
	}
	set.Hash = set.ComputeHash()
	result, err := (SPALProviderRegistration{}).Verify(context.Background(), set, []orchestration.ActionReceipt{{
		ActionID: action.ID, Status: "applied", EffectivelyOnce: true, EvidenceRefs: []string{"spal.provider_record:record-1"},
		Details: map[string]any{"structural_readback": "pass", "provider_record_id": "record-1", "provider_instance_id": "instance-1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "pass" || result.Structural != "pass" || result.Acoustic != "not_applicable" || result.UserAcceptance != "unknown" {
		t.Fatalf("unexpected verification result: %#v", result)
	}
}
