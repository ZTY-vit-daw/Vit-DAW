package executionverifiers

import (
	"context"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type passingPanAcoustic struct{}

func (passingPanAcoustic) VerifyPanLayout(context.Context, orchestration.ActionSet) (AcousticResult, error) {
	return AcousticResult{Status: "pass", EvidenceRefs: []string{"MOM:post-pan-observe"}}, nil
}

func TestPanLayoutVerifierSeparatesStructuralAcousticAndTaste(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": -0.5}}}}
	receipts := []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}}
	reader := fakeStateReader{State: &kernel.VSPStateResult{
		Response: map[string]any{"type": "state.snapshot"}, LegacyState: map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "pan": -0.5}}},
	}}
	result, err := (PanLayout{State: reader, Acoustic: passingPanAcoustic{}}).Verify(context.Background(), actionSet, receipts)
	if err != nil || result.Status != "pass" || result.Structural != "pass" || result.Acoustic != "pass" || result.UserAcceptance != "unknown" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
