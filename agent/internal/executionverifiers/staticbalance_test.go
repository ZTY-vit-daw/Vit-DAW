package executionverifiers

import (
	"context"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type fakeStateReader struct{ State *kernel.VSPStateResult }

func (f fakeStateReader) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	return f.State, nil
}

type passingAcoustic struct{}

func (passingAcoustic) VerifyStaticBalance(context.Context, orchestration.ActionSet) (AcousticResult, error) {
	return AcousticResult{Status: "pass", EvidenceRefs: []string{"MOM:post-observe"}}, nil
}

func TestStaticBalanceVerifierSeparatesStructuralAndAcousticResults(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1", Args: map[string]any{"target_db": -1.0}}}}
	receipts := []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}}
	reader := fakeStateReader{State: &kernel.VSPStateResult{
		Response:    map[string]any{"type": "state.snapshot"},
		LegacyState: map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "volume_db": -1.0}}},
	}}
	withoutAcoustic, err := (StaticBalance{State: reader}).Verify(context.Background(), actionSet, receipts)
	if err != nil {
		t.Fatal(err)
	}
	if withoutAcoustic.Structural != "pass" || withoutAcoustic.Status != "inconclusive" {
		t.Fatalf("structural-only verification must remain inconclusive: %#v", withoutAcoustic)
	}
	withAcoustic, err := (StaticBalance{State: reader, Acoustic: passingAcoustic{}}).Verify(context.Background(), actionSet, receipts)
	if err != nil || withAcoustic.Status != "pass" {
		t.Fatalf("full verification failed: %#v err=%v", withAcoustic, err)
	}
}
