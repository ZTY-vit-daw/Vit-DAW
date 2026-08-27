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
	txs       []string
}

func (f *fakeVSPClient) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	state := f.snapshots[0]
	if len(f.snapshots) > 1 {
		f.snapshots = f.snapshots[1:]
	}
	return state, nil
}

func (f *fakeVSPClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "transaction_id": txID}}, nil
}

func (f *fakeVSPClient) SendVSPCommandWithIDs(_ context.Context, command string, args map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	captured := map[string]any{"command": command}
	for key, value := range args {
		captured[key] = value
	}
	f.commands = append(f.commands, captured)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	return &kernel.VSPCommandResult{TransactionID: txID, Command: command, Payload: map[string]any{"status": "ok", "command": command}}, nil
}

func TestStaticBalanceVSPPortUsesCASStableRequestAndReadback(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{
		stateResultWithGain("epoch-1", 7, "before", "t1", 0), stateResultWithGain("epoch-1", 8, "after", "t1", -1),
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
	if receipt.Details["before_revision"] != "7" || receipt.Details["after_revision"] != "8" ||
		receipt.Details["transaction_id"] != "tx_execution:s1:a1" || receipt.Details["idempotency_key"] != "execution:s1:a1" ||
		receipt.Details["actual_readback_db"] != -1.0 || receipt.Details["readback_verified"] != true {
		t.Fatalf("D1 receipt metadata=%#v", receipt.Details)
	}
}

func TestStaticBalanceVSPPortRejectsNonAdvancingRevisionAndMissingReadback(t *testing.T) {
	for _, test := range []struct {
		name  string
		after *kernel.VSPStateResult
	}{
		{"non advancing revision", stateResultWithGain("epoch-1", 7, "same", "t1", -1)},
		{"missing readback", stateResult("epoch-1", 8, "after")},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{stateResultWithGain("epoch-1", 7, "before", "t1", 0), test.after}}
			port := &StaticBalanceVSPPort{Client: client}
			cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong", Hash: "cut-1"}
			set := orchestration.ActionSet{ID: "as", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{{ID: "a1", Command: "track_gain_adjust", TargetRef: "t1", BeforeFingerprint: "t1:0", Args: map[string]any{"target_db": -1.0}}}}
			if err := port.Preflight(context.Background(), set, cut); err != nil {
				t.Fatal(err)
			}
			if _, err := port.Apply(context.Background(), set.Actions[0], "d1:a1"); err == nil {
				t.Fatal("invalid post-mutation state accepted")
			}
		})
	}
}

func TestStaticBalanceVSPPortReconcilesFromRealStateWithoutRetry(t *testing.T) {
	state := stateResult("epoch-1", 9, "after")
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": "t1", "volume_db": -1.0}}}
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{state}}
	port := &StaticBalanceVSPPort{Client: client}
	receipt, err := port.Reconcile(context.Background(), orchestration.Action{
		ID: "a1", TargetRef: "t1", Args: map[string]any{"target_db": -1.0},
	}, "execution:s1:a1", orchestration.ProjectCut{ProjectEpoch: "epoch-1", BaseProjectRevision: "7"})
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

func stateResultWithGain(epoch string, revision int64, hash, trackID string, gain float64) *kernel.VSPStateResult {
	state := stateResult(epoch, revision, hash)
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": trackID, "volume_db": gain}}}
	return state
}
