package executionports

import (
	"context"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

func TestPanLayoutVSPPortUsesCASStableRequestAndReadback(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{
		stateResult("epoch-1", 7, "before"), stateResult("epoch-1", 8, "after"),
	}}
	port := &PanLayoutVSPPort{Client: client}
	cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong", Hash: "cut-1"}
	actionSet := orchestration.ActionSet{ID: "as", CapabilityID: "static_mix.pan_layout.v0", ProjectCutHash: "cut-1", Actions: []orchestration.Action{{
		ID: "a1", Command: "track_pan_set", TargetRef: "t1", BeforeFingerprint: "track:t1:pan:0", Args: map[string]any{"target_pan": -0.5},
	}}}
	if err := port.Preflight(context.Background(), actionSet, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:s1:a1")
	if err != nil || receipt.AppliedRevision != "8" || !receipt.EffectivelyOnce {
		t.Fatalf("receipt=%#v err=%v", receipt, err)
	}
	if client.commands[0]["cmd"] != "set_pan" || client.commands[0]["pan"] != -0.5 || client.commands[0]["base_revision"] != int64(7) || client.requests[0] != "execution:s1:a1" {
		t.Fatalf("missing B3 CAS/idempotency: command=%#v requests=%#v", client.commands[0], client.requests)
	}
}

func TestPanLayoutVSPPortReconcilesWithoutRetry(t *testing.T) {
	state := stateResult("epoch-1", 9, "after")
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "pan": -0.5}}}
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{state}}
	receipt, err := (&PanLayoutVSPPort{Client: client}).Reconcile(context.Background(), orchestration.Action{
		ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": -0.5},
	}, "execution:s1:a1", orchestration.ProjectCut{ProjectEpoch: "epoch-1"})
	if err != nil || receipt.Status != "applied" || len(client.commands) != 0 {
		t.Fatalf("reconcile retried or failed: receipt=%#v commands=%#v err=%v", receipt, client.commands, err)
	}
}
