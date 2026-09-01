package executionverifiers

import (
	"context"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

func TestStaticPanBalanceVerifierSeparatesStructuralAndAcousticResults(t *testing.T) {
	actionSet := orchestration.ActionSet{Actions: []orchestration.Action{{ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": 0.1}}}}
	receipts := []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}}
	reader := fakeStateReader{State: &kernel.VSPStateResult{
		Response:    map[string]any{"type": "state.snapshot"},
		LegacyState: map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "pan": 0.1}}},
	}}
	withoutAcoustic, err := (StaticPanBalance{State: reader}).Verify(context.Background(), actionSet, receipts)
	if err != nil {
		t.Fatal(err)
	}
	if withoutAcoustic.Structural != "pass" || withoutAcoustic.Status != "inconclusive" {
		t.Fatalf("structural-only verification must remain inconclusive: %#v", withoutAcoustic)
	}
	withAcoustic, err := (StaticPanBalance{State: reader, Acoustic: passingAcoustic{}}).Verify(context.Background(), actionSet, receipts)
	if err != nil || withAcoustic.Status != "pass" {
		t.Fatalf("full verification failed: %#v err=%v", withAcoustic, err)
	}
}

func panPostconditionState(tracks []any) *kernel.VSPStateResult {
	return &kernel.VSPStateResult{Response: map[string]any{"type": "state.snapshot"}, LegacyState: map[string]any{"tracks": tracks}}
}

func TestStaticPanBalanceVerifierFailsClosedOnPanPostcondition(t *testing.T) {
	receipts := []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}}
	for _, test := range []struct {
		name   string
		state  *kernel.VSPStateResult
		action orchestration.Action
	}{
		{"readback drifted", panPostconditionState([]any{map[string]any{"track_id": "t1", "pan": 0.4}}),
			orchestration.Action{ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": 0.1}}},
		{"missing target arg", panPostconditionState([]any{map[string]any{"track_id": "t1", "pan": 0.1}}),
			orchestration.Action{ID: "a1", TargetRef: "t1", Args: map[string]any{}}},
		{"track missing", panPostconditionState([]any{}),
			orchestration.Action{ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": 0.1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			actionSet := orchestration.ActionSet{Actions: []orchestration.Action{test.action}}
			result, err := (StaticPanBalance{State: fakeStateReader{State: test.state}}).Verify(context.Background(), actionSet, receipts)
			if err == nil || result.Structural != "fail" {
				t.Fatalf("pan postcondition violation accepted: result=%#v err=%v", result, err)
			}
		})
	}
}
