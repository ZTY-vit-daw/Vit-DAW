package executionports

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
)

// fakeEQVSPClient extends the shared fake with plugin-parameter readback and
// instantiate_plugin responses.
type fakeEQVSPClient struct {
	fakeVSPClient
	paramValues    map[string]float64 // "plugin/param" -> current value
	instantiated   map[string]string  // identifier -> plugin id
	instantiateErr string
	readbackErr    string
	swallowWrites  bool
}

func (f *fakeEQVSPClient) SendVSPLegacyCommandWithIDs(ctx context.Context, command map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	switch command["cmd"] {
	case "instantiate_plugin":
		if f.instantiateErr != "" {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": f.instantiateErr}}, nil
		}
		identifier := strings.TrimSpace(command["plugin_identifier"].(string))
		pluginID := f.instantiated[identifier]
		if pluginID == "" {
			pluginID = "plg_" + identifier + "_1"
			f.instantiated[identifier] = pluginID
			_ = pluginID
		}
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "plugin_id": pluginID}}, nil
	case "get_plugin_parameters":
		if f.readbackErr != "" {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": f.readbackErr}}, nil
		}
		value := f.paramValues[command["plugin_id"].(string)+"/"+command["param_id"].(string)]
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{
			"status":     "ok",
			"parameters": []any{map[string]any{"param_id": command["param_id"], "value": value}},
		}}, nil
	case "set_plugin_param":
		if !f.swallowWrites {
			f.paramValues[command["plugin_id"].(string)+"/"+command["param_id"].(string)] = command["value"].(float64)
		}
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "transaction_id": txID}}, nil
	}
	return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "transaction_id": txID}}, nil
}

func eqSnapshots() []*kernel.VSPStateResult {
	return []*kernel.VSPStateResult{
		stateResultWithGain("epoch-eq", 7, "before", "t1", 0),
		stateResultWithGain("epoch-eq", 8, "after", "t1", 0),
	}
}

func eqCut() orchestration.ProjectCut {
	return orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "epoch-eq", BaseProjectRevision: "7", Consistency: "strong", Hash: "cut-eq"}
}

func eqAction() orchestration.Action {
	return orchestration.Action{ID: "a1", Command: "static_eq_band_adjust", TargetRef: "t1",
		BeforeFingerprint: "track:t1:eq:plg_1:band_gain_db:0", Args: map[string]any{"plugin_id": "plg_1", "param_id": "band_gain_db", "target_value": -1.5}}
}

func TestStaticEQVSPPortAppliesWithCASAndReadback(t *testing.T) {
	client := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{"plg_1/band_gain_db": 0}, instantiated: map[string]string{}}
	client.fakeVSPClient.snapshots = append(client.fakeVSPClient.snapshots, eqSnapshots()...)
	port := &StaticEQVSPPort{Client: client}
	actionSet := orchestration.ActionSet{ID: "as", CapabilityID: "static_mix.static_eq.v0", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{eqAction()}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:eq:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || receipt.AppliedRevision != "8" || !receipt.EffectivelyOnce {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if client.commands[len(client.commands)-2]["base_revision"] != int64(7) {
		t.Fatalf("set_plugin_param missing CAS base_revision: %#v", client.commands)
	}
	if receipt.Details["actual_readback_value"] != -1.5 || receipt.Details["readback_verified"] != true ||
		receipt.Details["before_readback_value"] != 0.0 || receipt.Details["plugin_instantiated_by_action"] != false {
		t.Fatalf("readback metadata: %#v", receipt.Details)
	}
}

func TestStaticEQVSPPortInstantiatesWhenPluginIDMissing(t *testing.T) {
	client := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{}, instantiated: map[string]string{}}
	port := &StaticEQVSPPort{Client: client}
	action := eqAction()
	delete(action.Args, "plugin_id")
	action.Args["plugin_identifier"] = "juce_eq"
	actionSet := orchestration.ActionSet{ID: "as", CapabilityID: "static_mix.static_eq.v0", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:eq:a1")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if receipt.Details["plugin_instantiated_by_action"] != true || receipt.Details["plugin_id"] != "plg_juce_eq_1" {
		t.Fatalf("instantiation metadata: %#v", receipt.Details)
	}
	if client.commands[0]["cmd"] != "instantiate_plugin" || client.commands[0]["plugin_identifier"] != "juce_eq" {
		t.Fatalf("expected instantiate command first: %#v", client.commands[0])
	}
}

func TestStaticEQVSPPortRejectsInstantiateFailureAndReadbackMismatch(t *testing.T) {
	client := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{"plg_1/band_gain_db": 0}, instantiated: map[string]string{}, instantiateErr: "plugin unavailable"}
	port := &StaticEQVSPPort{Client: client}
	action := eqAction()
	delete(action.Args, "plugin_id")
	action.Args["plugin_identifier"] = "juce_eq"
	actionSet := orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Apply(context.Background(), action, "execution:eq:a1"); err == nil || !strings.Contains(err.Error(), "plugin instantiation failed") {
		t.Fatalf("expected instantiation failure, got %v", err)
	}

	mismatch := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{"plg_1/band_gain_db": 0.25}, instantiated: map[string]string{}, swallowWrites: true}
	mismatch.fakeVSPClient.snapshots = append(mismatch.fakeVSPClient.snapshots, eqSnapshots()...)
	port2 := &StaticEQVSPPort{Client: mismatch}
	actionSet2 := orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{eqAction()}}
	if err := port2.Preflight(context.Background(), actionSet2, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port2.Apply(context.Background(), eqAction(), "execution:eq:a1")
	if err == nil || receipt.Status != "applied_unreconciled" {
		t.Fatalf("expected unreconciled readback mismatch, got %#v / %v", receipt, err)
	}
}

func TestStaticEQVSPPortPreflightGuards(t *testing.T) {
	client := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{"plg_1/band_gain_db": 0}, instantiated: map[string]string{}}
	port := &StaticEQVSPPort{Client: client}
	// wrong command
	action := eqAction()
	action.Command = "track_gain_adjust"
	set := orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, eqCut()); err == nil {
		t.Fatal("wrong command accepted")
	}
	// missing both plugin identities
	action = eqAction()
	delete(action.Args, "plugin_id")
	set = orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, eqCut()); err == nil {
		t.Fatal("missing plugin identity accepted")
	}
	// stale cut
	action = eqAction()
	set = orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	stale := eqCut()
	stale.BaseProjectRevision = "9"
	if err := port.Preflight(context.Background(), set, stale); err == nil {
		t.Fatal("stale cut accepted")
	}
}
