package executionports

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

func stateResultWithPan(epoch string, revision int64, hash, trackID string, pan float64) *kernel.VSPStateResult {
	state := stateResult(epoch, revision, hash)
	state.LegacyState = map[string]any{"tracks": []any{map[string]any{"track_id": trackID, "pan": pan}}}
	return state
}

func trackPanCutForTest() orchestration.ProjectCut {
	return orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong", Hash: "cut-1"}
}

func trackPanActionSetForTest(target float64) orchestration.ActionSet {
	return orchestration.ActionSet{ID: "as", CapabilityID: "static_mix.pan.v0", ProjectCutHash: "cut-1", Actions: []orchestration.Action{{
		ID: "a1", Command: "track_pan_adjust", TargetRef: "t1", BeforeFingerprint: "track:t1:pan:fader_pan:pending", Args: map[string]any{"target_pan": target},
	}}}
}

type erroringVSPClient struct{ fakeVSPClient }

func (f *erroringVSPClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": "track not found"}}, nil
}

func TestTrackPanVSPPortUsesCASStableRequestAndReadback(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{
		stateResultWithPan("epoch-1", 7, "before", "t1", 0), stateResultWithPan("epoch-1", 8, "after", "t1", 0.1),
	}}
	port := &TrackPanVSPPort{Client: client}
	set := trackPanActionSetForTest(0.1)
	if err := port.Preflight(context.Background(), set, trackPanCutForTest()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), set.Actions[0], "execution:s1:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.AppliedRevision != "8" || !receipt.EffectivelyOnce {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if client.commands[0]["cmd"] != "set_pan" || client.commands[0]["pan"] != 0.1 || client.commands[0]["base_revision"] != int64(7) || client.requests[0] != "execution:s1:a1" {
		t.Fatalf("missing CAS or stable request ID: command=%#v requests=%#v", client.commands[0], client.requests)
	}
	if receipt.Details["before_revision"] != "7" || receipt.Details["after_revision"] != "8" ||
		receipt.Details["transaction_id"] != "tx_execution:s1:a1" || receipt.Details["idempotency_key"] != "execution:s1:a1" ||
		receipt.Details["requested_target_pan"] != 0.1 || receipt.Details["actual_readback_pan"] != 0.1 || receipt.Details["readback_verified"] != true ||
		receipt.Details["before_readback_pan"] != 0.0 || receipt.Details["before_readback_available"] != true {
		t.Fatalf("D1 pan receipt metadata=%#v", receipt.Details)
	}
}

// The clamp boundary is fail-closed (survey ruling 2): the kernel silently
// jlimits an out-of-range pan, so a current 0.9 + delta +0.15 target of 1.05
// would write 1.0 and explode the readback assert after the mutation. The
// Preflight must reject the action before any command reaches the kernel;
// exactly +/-1 stays reachable.
func TestTrackPanVSPPortPreflightRejectsClampOutOfBoundsTarget(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{stateResultWithPan("epoch-1", 7, "before", "t1", 0.9)}}
	port := &TrackPanVSPPort{Client: client}
	if err := port.Preflight(context.Background(), trackPanActionSetForTest(1.05), trackPanCutForTest()); err == nil || !strings.Contains(err.Error(), "target_pan") {
		t.Fatalf("clamp-out-of-bounds target accepted at preflight: err=%v", err)
	}
	if len(client.commands) != 0 {
		t.Fatalf("out-of-bounds pan action reached the kernel: %#v", client.commands)
	}
	if err := port.Preflight(context.Background(), trackPanActionSetForTest(1.0), trackPanCutForTest()); err != nil {
		t.Fatalf("boundary target 1.0 rejected: %v", err)
	}
}

func TestTrackPanVSPPortRejectsNonAdvancingRevisionAndBadReadback(t *testing.T) {
	for _, test := range []struct {
		name  string
		after *kernel.VSPStateResult
	}{
		{"non advancing revision", stateResultWithPan("epoch-1", 7, "same", "t1", 0.1)},
		{"missing readback", stateResult("epoch-1", 8, "after")},
		{"readback mismatch", stateResultWithPan("epoch-1", 8, "after", "t1", 0.25)},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{stateResultWithPan("epoch-1", 7, "before", "t1", 0), test.after}}
			port := &TrackPanVSPPort{Client: client}
			set := trackPanActionSetForTest(0.1)
			if err := port.Preflight(context.Background(), set, trackPanCutForTest()); err != nil {
				t.Fatal(err)
			}
			receipt, err := port.Apply(context.Background(), set.Actions[0], "d1:a1")
			if err == nil || receipt.Status != "applied_unreconciled" {
				t.Fatalf("invalid post-mutation state accepted: receipt=%#v err=%v", receipt, err)
			}
		})
	}
}

func TestTrackPanVSPPortRejectReceiptKeepsFailedShape(t *testing.T) {
	client := &erroringVSPClient{fakeVSPClient{snapshots: []*kernel.VSPStateResult{stateResultWithPan("epoch-1", 7, "before", "t1", 0)}}}
	port := &TrackPanVSPPort{Client: client}
	set := trackPanActionSetForTest(0.1)
	if err := port.Preflight(context.Background(), set, trackPanCutForTest()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), set.Actions[0], "d1:a1")
	if err == nil || receipt.Status != "failed" || receipt.Error == "" || receipt.AppliedRevision != "" {
		t.Fatalf("kernel rejection lost its receipt shape: receipt=%#v err=%v", receipt, err)
	}
}

func TestTrackPanVSPPortReconcilesFromRealStateWithoutRetry(t *testing.T) {
	client := &fakeVSPClient{snapshots: []*kernel.VSPStateResult{stateResultWithPan("epoch-1", 9, "after", "t1", 0.1)}}
	port := &TrackPanVSPPort{Client: client}
	receipt, err := port.Reconcile(context.Background(), orchestration.Action{
		ID: "a1", TargetRef: "t1", Args: map[string]any{"target_pan": 0.1},
	}, "execution:s1:a1", orchestration.ProjectCut{ProjectEpoch: "epoch-1", BaseProjectRevision: "7"})
	if err != nil || receipt.Status != "applied" || len(client.commands) != 0 {
		t.Fatalf("reconcile retried or failed: receipt=%#v commands=%#v err=%v", receipt, client.commands, err)
	}
	if receipt.Details["readback_verified"] != true || receipt.Details["reconciled"] != true {
		t.Fatalf("reconcile receipt metadata=%#v", receipt.Details)
	}
}
