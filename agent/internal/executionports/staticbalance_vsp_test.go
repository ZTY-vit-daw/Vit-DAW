package executionports

import (
	"context"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

type fakeVSPClient struct {
	snapshots []*kernel.VSPStateResult
	commands  []map[string]any
	requests  []string
}

func (f *fakeVSPClient) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	state := f.snapshots[0]
	if len(f.snapshots) > 1 {
		f.snapshots = f.snapshots[1:]
	}
	return state, nil
}

func (f *fakeVSPClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, requestID, _ string) (*kernel.VSPCommandResult, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, requestID)
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok"}}, nil
}

func TestStaticBalanceVSPPortUsesCASStableRequestAndReadback(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{
		stateResult("epoch-1", 7, "before"), stateResult("epoch-1", 8, "after"),
	}}
	port := &StaticBalanceVSPPort{Client: client}
	cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong", Hash: "cut-1"}
	actionSet := orchestration.ActionSet{ID: "as", CapabilityID: "static_mix.static_balance.v0", ProjectCutHash: "cut-1", Actions: []orchestration.Action{{
		ID: "a1", Command: "track_gain_adjust", TargetRef: "t1", BeforeFingerprint: "track:t1:fader_db:0", Args: map[string]any{"target_db": -1.0},
	}}}
	if err := port.Preflight(context.Background(), actionSet, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:s1:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.AppliedRevision != "8" || !receipt.EffectivelyOnce {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if client.commands[0]["base_revision"] != int64(7) || client.requests[0] != "execution:s1:a1" {
		t.Fatalf("missing CAS or stable request ID: command=%#v requests=%#v", client.commands[0], client.requests)
	}
}

func TestStaticBalanceVSPPortReconcilesFromRealStateWithoutRetry(t *testing.T) {
	state := stateResult("epoch-1", 9, "after")
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "volume_db": -1.0}}}
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{state}}
	port := &StaticBalanceVSPPort{Client: client}
	receipt, err := port.Reconcile(context.Background(), orchestration.Action{
		ID: "a1", TargetRef: "t1", Args: map[string]any{"target_db": -1.0},
	}, "execution:s1:a1", orchestration.ProjectCut{ProjectEpoch: "epoch-1"})
	if err != nil || receipt.Status != "applied" || len(client.commands) != 0 {
		t.Fatalf("reconcile retried or failed: receipt=%#v commands=%#v err=%v", receipt, client.commands, err)
	}
}

func stateResult(epoch string, revision int64, hash string) *kernel.VSPStateResult {
	return &kernel.VSPStateResult{
		Response:     map[string]any{"type": "state.snapshot"},
		Payload:      map[string]any{"snapshot_hash": hash},
		ProjectEpoch: epoch, Revision: revision, SnapshotHash: hash,
	}
}
