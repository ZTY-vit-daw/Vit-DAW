package executionports

import (
	"context"
	"fmt"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

type fakeSPALVSPClient struct {
	snapshots    []*kernel.VSPStateResult
	legacy       []*kernel.VSPCommandResult
	batches      []*kernel.VSPCommandResult
	legacyCalls  []map[string]any
	batchCalls   []map[string]any
	batchRequest []string
}

func (f *fakeSPALVSPClient) VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error) {
	if len(f.snapshots) == 0 {
		return nil, fmt.Errorf("unexpected snapshot")
	}
	result := f.snapshots[0]
	f.snapshots = f.snapshots[1:]
	return result, nil
}

func (f *fakeSPALVSPClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, _, _ string) (*kernel.VSPCommandResult, error) {
	f.legacyCalls = append(f.legacyCalls, command)
	if len(f.legacy) == 0 {
		return nil, fmt.Errorf("unexpected legacy command")
	}
	result := f.legacy[0]
	f.legacy = f.legacy[1:]
	return result, nil
}

func (f *fakeSPALVSPClient) SendVSPCommandWithIDs(_ context.Context, command string, args map[string]any, requestID, transactionID string) (*kernel.VSPCommandResult, error) {
	f.batchCalls = append(f.batchCalls, map[string]any{"command": command, "args": args, "transaction_id": transactionID})
	f.batchRequest = append(f.batchRequest, requestID)
	if len(f.batches) == 0 {
		return nil, fmt.Errorf("unexpected batch command")
	}
	result := f.batches[0]
	f.batches = f.batches[1:]
	return result, nil
}

func TestSPALVSPPortWritesReadbacksAndPersistsPhysicalReceipt(t *testing.T) {
	manifest, action, cut := testSPALManifestAction(t)
	client := &fakeSPALVSPClient{
		snapshots: []*kernel.VSPStateResult{
			spalVSPState("epoch-1", 7),
			spalVSPState("epoch-1", 8),
		},
		legacy: []*kernel.VSPCommandResult{
			spalReadback(manifest.Preimage),
			spalReadback(manifest.Writes),
		},
		batches: []*kernel.VSPCommandResult{spalBatchOK("tx-write")},
	}
	port := &SPALVSPPort{Client: client}
	set := orchestration.ActionSet{ID: "set", CapabilityID: "static_mix.low_end_relation.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:s1:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || receipt.Details["structural_readback"] != "pass" || receipt.Details["provider_id"] != spal.ExperimentalTDRNovaProviderID {
		t.Fatalf("unexpected SPAL receipt: %#v", receipt)
	}
	if len(client.batchCalls) != 1 || client.batchCalls[0]["command"] != "plugin.set_params_batch" || client.batchRequest[0] != "execution:s1:a1" {
		t.Fatalf("SPAL did not use a typed batch with the stable request id: %#v", client.batchCalls)
	}
	args := client.batchCalls[0]["args"].(map[string]any)
	if args["base_revision"] != int64(7) || args["plugin_id"] != "plugin-1" || len(args["parameters"].([]any)) != len(manifest.Writes) {
		t.Fatalf("SPAL write payload omitted CAS/binding information: %#v", args)
	}
}

func TestSPALVSPControlledRoundtripRestoresFrozenPreimage(t *testing.T) {
	manifest, action, cut := testSPALManifestAction(t)
	client := &fakeSPALVSPClient{
		snapshots: []*kernel.VSPStateResult{
			spalVSPState("epoch-1", 7), // preflight
			spalVSPState("epoch-1", 8), // apply revision
			spalVSPState("epoch-1", 8), // rollback CAS revision
		},
		legacy: []*kernel.VSPCommandResult{
			spalReadback(manifest.Preimage),
			spalReadback(manifest.Writes),
			spalReadback(manifest.Writes), // explicit restore safety read
			spalReadback(manifest.Preimage),
		},
		batches: []*kernel.VSPCommandResult{
			spalBatchOK("tx-apply"),
			spalBatchOK("tx-restore"),
		},
	}
	port := &SPALVSPControlledRoundtripPort{Client: client}
	set := orchestration.ActionSet{ID: "set", CapabilityID: "static_mix.low_end_relation.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:roundtrip:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || receipt.Details["structural_readback"] != "pass" || receipt.Details["controlled_roundtrip"] != true || receipt.Details["controlled_roundtrip_restore_status"] != "restored" || receipt.Details["controlled_roundtrip_restored_preimage"] != true {
		t.Fatalf("controlled roundtrip receipt = %#v", receipt)
	}
	if len(client.batchCalls) != 2 {
		t.Fatalf("expected apply and automatic restore batches, got %#v", client.batchCalls)
	}
	restore := client.batchCalls[1]["args"].(map[string]any)
	if restore["base_revision"] != int64(8) || !sameSPALRows(restore["parameters"].([]any), manifest.Preimage) {
		t.Fatalf("automatic restore did not use the frozen preimage: %#v", restore)
	}
}

func TestSPALVSPControlledRoundtripRefusesConcurrentCleanup(t *testing.T) {
	manifest, action, cut := testSPALManifestAction(t)
	concurrent := append([]spal.PhysicalParameter(nil), manifest.Writes...)
	concurrent[2].Value = 777
	client := &fakeSPALVSPClient{
		snapshots: []*kernel.VSPStateResult{
			spalVSPState("epoch-1", 7),
			spalVSPState("epoch-1", 8),
			spalVSPState("epoch-1", 8),
		},
		legacy: []*kernel.VSPCommandResult{
			spalReadback(manifest.Preimage),
			spalReadback(manifest.Writes),
			spalReadback(concurrent),
		},
		batches: []*kernel.VSPCommandResult{spalBatchOK("tx-apply")},
	}
	port := &SPALVSPControlledRoundtripPort{Client: client}
	set := orchestration.ActionSet{ID: "set", CapabilityID: "static_mix.low_end_relation.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:roundtrip:a1")
	if err == nil || receipt.Status != "partial_failure" || receipt.Details["controlled_roundtrip_restore_status"] != "unsafe" || len(client.batchCalls) != 1 {
		t.Fatalf("controlled roundtrip should fail closed on a concurrent edit: receipt=%#v batches=%#v err=%v", receipt, client.batchCalls, err)
	}
}

func TestSPALVSPPortNilApplyFailsClosed(t *testing.T) {
	var port *SPALVSPPort
	receipt, err := port.Apply(context.Background(), orchestration.Action{ID: "action-1"}, "execution:s1:a1")
	if err == nil || receipt.Status != "failed" {
		t.Fatalf("nil SPAL VSP port must fail closed: receipt=%#v err=%v", receipt, err)
	}
}

func TestSPALVSPInspectorCapturesOnlyFrozenWritePreimage(t *testing.T) {
	manifest, _, _ := testSPALManifestAction(t)
	client := &fakeSPALVSPClient{legacy: []*kernel.VSPCommandResult{spalReadback(manifest.Preimage)}}
	got, err := (SPALVSPInspector{Client: client}).CaptureSPALPreimage(context.Background(), manifest.Binding, manifest.Writes)
	if err != nil || !samePhysicalParameters(got, manifest.Preimage) {
		t.Fatalf("SPAL inspector failed to capture the frozen preimage: got=%#v err=%v", got, err)
	}
}

func TestSPALVSPPortCompensatesPartialBatchFailure(t *testing.T) {
	manifest, action, cut := testSPALManifestAction(t)
	client := &fakeSPALVSPClient{
		snapshots: []*kernel.VSPStateResult{
			spalVSPState("epoch-1", 7),
			spalVSPState("epoch-1", 8), // rollback must use the post-failure revision.
		},
		legacy: []*kernel.VSPCommandResult{
			spalReadback(manifest.Preimage),
			spalReadback(manifest.Preimage), // safety read before compensation
			spalReadback(manifest.Preimage),
		},
		batches: []*kernel.VSPCommandResult{
			{Response: map[string]any{"type": "command.error", "error": map[string]any{"message": "parameter 2 failed"}}, Payload: map[string]any{"status": "partial_failure"}, TransactionID: "tx-failed"},
			spalBatchOK("tx-rollback"),
		},
	}
	port := &SPALVSPPort{Client: client}
	set := orchestration.ActionSet{ID: "set", CapabilityID: "static_mix.low_end_relation.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:s1:a1")
	if err == nil || receipt.Status != "rolled_back" || receipt.Details["rollback_status"] != "restored" {
		t.Fatalf("partial batch was not compensated: receipt=%#v err=%v", receipt, err)
	}
	if len(client.batchCalls) != 2 {
		t.Fatalf("expected write and rollback calls, got %#v", client.batchCalls)
	}
	rollbackArgs := client.batchCalls[1]["args"].(map[string]any)
	if rollbackArgs["base_revision"] != int64(8) || !sameSPALRows(rollbackArgs["parameters"].([]any), manifest.Preimage) {
		t.Fatalf("rollback did not restore the frozen preimage: %#v", rollbackArgs)
	}
}

func TestSPALVSPPortDoesNotRollbackAcrossConcurrentParameterChange(t *testing.T) {
	manifest, action, cut := testSPALManifestAction(t)
	concurrent := append([]spal.PhysicalParameter(nil), manifest.Preimage...)
	concurrent[2].Value = 777
	client := &fakeSPALVSPClient{
		snapshots: []*kernel.VSPStateResult{spalVSPState("epoch-1", 7), spalVSPState("epoch-1", 8)},
		legacy:    []*kernel.VSPCommandResult{spalReadback(manifest.Preimage), spalReadback(concurrent)},
		batches:   []*kernel.VSPCommandResult{{Response: map[string]any{"type": "command.error"}, Payload: map[string]any{"status": "partial_failure"}}},
	}
	port := &SPALVSPPort{Client: client}
	set := orchestration.ActionSet{ID: "set", CapabilityID: "static_mix.low_end_relation.v0", ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:s1:a1")
	if err == nil || receipt.Status != "partial_failure" || receipt.Details["rollback_status"] != "unsafe" || len(client.batchCalls) != 1 {
		t.Fatalf("unsafe concurrent value was rolled back or hidden: receipt=%#v calls=%#v err=%v", receipt, client.batchCalls, err)
	}
}

func testSPALManifestAction(t *testing.T) (spal.ExecutionManifest, orchestration.Action, orchestration.ProjectCut) {
	t.Helper()
	adapter, err := spal.NewExperimentalTDRNovaAdapter()
	if err != nil {
		t.Fatal(err)
	}
	maxGain := 3.0
	instruction := spal.Instruction{
		SchemaID:             spal.StaticBellControlID,
		TargetRef:            "track:bass",
		Parameters:           map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2},
		SafetyBounds:         spal.SafetyBounds{MaxAbsoluteGainDB: &maxGain},
		EvidenceRefs:         []string{"mom.low_end_overlap:obs-1"},
		ExpectedSignalChange: spal.SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"},
	}
	binding, err := adapter.Bind(spal.ProviderInstance{
		ID: "instance-1", ProviderID: spal.ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "plugin-1",
		PluginSignature: spal.ExperimentalTDRNovaSignature, Status: spal.InstanceVerified,
		Metadata: map[string]string{"band_slot": "band2", "static_bell_ready": "true"},
	}, instruction)
	if err != nil {
		t.Fatal(err)
	}
	preimage := []spal.PhysicalParameter{
		{ParameterID: "13", Value: 0, Unit: "toggle"}, {ParameterID: "18", Value: 1, Unit: "toggle"},
		{ParameterID: "16", Value: 100, Unit: "Hz"}, {ParameterID: "14", Value: 0, Unit: "dB"}, {ParameterID: "15", Value: 1, Unit: "Q"},
	}
	manifest, err := spal.CompileManifest(spal.Resolution{Status: spal.ResolutionBound, Binding: &binding}, instruction, preimage, adapter)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	set, err := manifest.ToActionSet("static_mix.low_end_relation.v0", cut)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, set.Actions[0], cut
}

func spalReadback(parameters []spal.PhysicalParameter) *kernel.VSPCommandResult {
	rows := make([]any, 0, len(parameters))
	for _, parameter := range parameters {
		rows = append(rows, map[string]any{"parameter_id": parameter.ParameterID, "value": parameter.Value})
	}
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "parameters": rows}}
}

func spalBatchOK(transactionID string) *kernel.VSPCommandResult {
	return &kernel.VSPCommandResult{Response: map[string]any{"type": "command.response"}, Payload: map[string]any{"status": "ok", "changed_params": []any{}}, TransactionID: transactionID}
}

func spalVSPState(epoch string, revision int64) *kernel.VSPStateResult {
	return &kernel.VSPStateResult{Response: map[string]any{"type": "state.snapshot"}, Payload: map[string]any{"status": "ok"}, ProjectEpoch: epoch, Revision: revision}
}

func sameSPALRows(rows []any, expected []spal.PhysicalParameter) bool {
	if len(rows) != len(expected) {
		return false
	}
	for index, row := range rows {
		mapped := row.(map[string]any)
		if mapped["parameter_id"] != expected[index].ParameterID || mapped["value"] != expected[index].Value {
			return false
		}
	}
	return true
}

func samePhysicalParameters(left, right []spal.PhysicalParameter) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ParameterID != right[index].ParameterID || left[index].Value != right[index].Value {
			return false
		}
	}
	return true
}

func TestSPALVSPPortRefusesSemanticDisplayWritesOutsideB4(t *testing.T) {
	client := &fakeSPALVSPClient{}
	port := &SPALVSPPort{Client: client}
	manifest := spal.ExecutionManifest{Binding: spal.RuntimeBinding{Instance: spal.ProviderInstance{TrackID: "track-1", PluginID: "plugin-1"}}}
	_, err := port.sendBatch(context.Background(), manifest, []spal.PhysicalParameter{{
		ParameterID: "freq", Value: 632, ValueMode: spal.PhysicalValueModeDisplay, BindingRef: "frequency_hz", Unit: "Hz",
	}}, 7, "request", "transaction")
	if err == nil || len(client.batchCalls) != 0 {
		t.Fatalf("semantic write escaped to raw parameter batch: err=%v calls=%#v", err, client.batchCalls)
	}
}
