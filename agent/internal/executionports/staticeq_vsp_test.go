package executionports

import (
	"context"
	"fmt"
	"math"
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
		stateResultWithGain("epoch-eq", 9, "after", "t1", 0),
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

// ---- parameter-driven (normalized batch) mode -------------------------------

type fakeNBParam struct{ normalized float64 }

// fakeNBVSPClient simulates a real VST3 plugin host for the normalized batch
// pipeline: path instantiation, a full parameter surface with display-domain
// metadata, the typed plugin.set_params_batch command, and normalized
// readback. Domains are [-24, +24] dB linear unless stated otherwise.
type fakeNBVSPClient struct {
	fakeVSPClient
	domainMin, domainMax float64
	withCandidate        bool   // expose display_domain_candidate; probe samples are always present
	frozenPhysicalText   bool   // live value_text ignores writes (degenerate threshold display)
	curveExponent        float64 // >0 bends physical = min + span*norm^k and switches text to the "+x.xx" form
	displayQuantum       float64 // >0 rounds the curved display text to this dB step (coarse compressor readouts)
	params               map[string]*fakeNBParam
	batchArgs            map[string]any
	batchFailure         string // when set, the typed batch answers partial_failure
	swallowWrites        bool   // kernel keeps old values despite ok status
	readDrift            float64
	failSurface          bool
}

func (f *fakeNBVSPClient) physicalFor(norm float64) float64 {
	if f.frozenPhysicalText {
		return f.domainMax
	}
	if f.curveExponent > 0 {
		return f.domainMin + (f.domainMax-f.domainMin)*math.Pow(norm, f.curveExponent)
	}
	return f.domainMin + norm*(f.domainMax-f.domainMin)
}

func newFakeNBVSPClient(ch1, ch2 string, domainMin, domainMax float64, withCandidate bool) *fakeNBVSPClient {
	return &fakeNBVSPClient{
		fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		domainMin:     domainMin, domainMax: domainMax,
		withCandidate: withCandidate,
		params: map[string]*fakeNBParam{
			ch1: {normalized: 0.5}, ch2: {normalized: 0.5},
		},
	}
}

func (f *fakeNBVSPClient) surfaceRow(paramID string) map[string]any {
	state := f.params[paramID]
	value := f.domainMin + state.normalized*(f.domainMax-f.domainMin)
	text := fmt.Sprintf("%.2f dB", value)
	if f.frozenPhysicalText || f.curveExponent > 0 {
		value = f.physicalFor(state.normalized)
		text = fmt.Sprintf("%+.2f", value)
		if f.displayQuantum > 0 {
			text = fmt.Sprintf("%+.2f", math.Round(value/f.displayQuantum)*f.displayQuantum)
		}
	}
	samples := []any{}
	for _, sampleNormalized := range []float64{0, 0.25, 0.5, 0.75, 1} {
		sampleValue := f.domainMin + sampleNormalized*(f.domainMax-f.domainMin)
		samples = append(samples, map[string]any{
			"normalized_value": sampleNormalized,
			"text":             fmt.Sprintf("%.2f dB", sampleValue),
		})
	}
	row := map[string]any{
		"id": paramID, "param_id": paramID,
		"normalized_value": state.normalized, "value_text": text,
		"display_probe": map[string]any{"mode": "read_only_value_to_string", "samples": samples},
	}
	if f.withCandidate {
		row["display_domain_candidate"] = map[string]any{
			"text": fmt.Sprintf("%g~%g dB", f.domainMin, f.domainMax), "unit": "dB",
			"min": f.domainMin, "max": f.domainMax, "scale": "linear",
		}
	}
	return row
}

func (f *fakeNBVSPClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	f.commands = append(f.commands, command)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	switch command["cmd"] {
	case "instantiate_plugin":
		if _, hasIdentifier := command["plugin_identifier"]; hasIdentifier {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": "identifier must stay empty on the path route"}}, nil
		}
		if strings.TrimSpace(fmt.Sprint(command["plugin_path"])) == "" {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": "plugin_path missing"}}, nil
		}
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "plugin_id": "plg_fixture_1"}}, nil
	case "get_plugin_parameters":
		if _, include := command["include_parameters"]; !include {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": "surface requires include_parameters"}}, nil
		}
		if f.failSurface {
			return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "error", "message": "plugin vanished"}}, nil
		}
		rows := []any{}
		for paramID, state := range f.params {
			row := f.surfaceRow(paramID)
			row["normalized_value"] = state.normalized + f.readDrift
			rows = append(rows, row)
		}
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "parameters": rows}}, nil
	}
	return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok"}}, nil
}

func (f *fakeNBVSPClient) SendVSPCommandWithIDs(_ context.Context, command string, args map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	captured := map[string]any{"command": command}
	for key, value := range args {
		captured[key] = value
	}
	f.commands = append(f.commands, captured)
	f.requests = append(f.requests, requestID)
	f.txs = append(f.txs, txID)
	if command == "plugin.set_params_batch" {
		f.batchArgs = args
		if f.batchFailure != "" {
			return &kernel.VSPCommandResult{TransactionID: txID, Command: command, Payload: map[string]any{"status": "partial_failure", "message": f.batchFailure}}, nil
		}
		if !f.swallowWrites {
			for _, entry := range args["parameters"].([]map[string]any) {
				if state, ok := f.params[fmt.Sprint(entry["parameter_id"])]; ok {
					state.normalized = entry["normalized_value"].(float64)
				}
			}
		}
		return &kernel.VSPCommandResult{TransactionID: txID, Command: command, Payload: map[string]any{"status": "ok"}}, nil
	}
	return &kernel.VSPCommandResult{TransactionID: txID, Command: command, Payload: map[string]any{"status": "ok"}}, nil
}

func nbAction() orchestration.Action {
	return orchestration.Action{ID: "a-nb", Command: staticEQActionCommand, TargetRef: "t1",
		BeforeFingerprint: "track:t1:eq:p315_c1:pending",
		Args: map[string]any{
			"write_mode": WriteModeNormalizedBatchV1, "plugin_path": "C:/plugins/Fixture EQ.vst3",
			"param_id": "p315_c1", "param_id_ch2": "p315_c2", "target_value": -1.5,
		}}
}

func TestStaticEQVSPPortNormalizedBatchDualChannelSingleMutation(t *testing.T) {
	client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
	port := &StaticEQVSPPort{Client: client}
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{nbAction()}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "execution:eq:a-nb")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || receipt.AppliedRevision != "9" || !receipt.EffectivelyOnce {
		t.Fatalf("receipt=%+v", receipt)
	}
	mutations := 0
	for _, request := range client.requests {
		if request == "execution:eq:a-nb" {
			mutations++
		}
	}
	if mutations != 1 {
		t.Fatalf("exactly one forward mutation expected, got %d (%v)", mutations, client.requests)
	}
	if client.requests[0] != "execution:eq:a-nb:instantiate" {
		t.Fatalf("instantiate must precede the mutation: %v", client.requests)
	}
	parameters, ok := client.batchArgs["parameters"].([]map[string]any)
	if !ok || len(parameters) != 2 || client.batchArgs["base_revision"] != int64(8) {
		t.Fatalf("batch=%+v", client.batchArgs)
	}
	requestedOne := parameters[0]["normalized_value"].(float64)
	requestedTwo := parameters[1]["normalized_value"].(float64)
	const wantNormalized = (-1.5 - (-24)) / 48 // linear [-24,+24] domain
	if math.Abs(requestedOne-wantNormalized) > 1e-12 || math.Abs(requestedTwo-wantNormalized) > 1e-12 {
		t.Fatalf("requested normalized %v/%v want %v", requestedOne, requestedTwo, wantNormalized)
	}
	if receipt.Details["readback_verified"] != true || receipt.Details["write_mode"] != WriteModeNormalizedBatchV1 ||
		receipt.Details["plugin_instantiated_by_action"] != true || receipt.Details["param_id_ch2"] != "p315_c2" {
		t.Fatalf("details=%+v", receipt.Details)
	}
	channels, _ := receipt.Details["normalized_channels"].([]map[string]any)
	if len(channels) != 2 {
		t.Fatalf("channel records=%+v", receipt.Details["normalized_channels"])
	}
	if physical, ok := receipt.Details["actual_readback_value"].(float64); !ok || math.Abs(physical-(-1.5)) > 0.01 {
		t.Fatalf("physical readback value=%v", receipt.Details["actual_readback_value"])
	}
}

func TestStaticEQVSPPortNormalizedBatchCurveFallbackWithoutCandidate(t *testing.T) {
	client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, false)
	port := &StaticEQVSPPort{Client: client}
	action := nbAction()
	action.Args["target_value"] = -6.0
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:eq:a-nb")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}
	parameters, _ := client.batchArgs["parameters"].([]map[string]any)
	got := parameters[0]["normalized_value"].(float64)
	const wantNormalized = (-6.0 - (-24)) / 48 // linearly spaced samples behave as power law k=1
	if math.Abs(got-wantNormalized) > 1e-9 {
		t.Fatalf("curve inversion produced %v want %v", got, wantNormalized)
	}
}

func TestStaticEQVSPPortNormalizedBatchFailures(t *testing.T) {
	runAction := func(mutation func(*fakeNBVSPClient)) (orchestration.ActionReceipt, error) {
		client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
		mutation(client)
		port := &StaticEQVSPPort{Client: client}
		action := nbAction()
		actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
		if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
			t.Fatal(err)
		}
		return port.Apply(context.Background(), action, "k")
	}

	receipt, err := runAction(func(c *fakeNBVSPClient) { c.failSurface = true })
	if err == nil || receipt.Status != "failed" || !strings.Contains(err.Error(), "plugin parameter surface read failed") {
		t.Fatalf("surface failure not surfaced: receipt=%+v err=%v", receipt, err)
	}

	receipt, err = runAction(func(c *fakeNBVSPClient) { c.batchFailure = "parameter p315_c2 refused" })
	if err == nil || receipt.Status != "failed" || !strings.Contains(err.Error(), "parameter p315_c2 refused") {
		t.Fatalf("batch failure envelope lost: receipt=%+v err=%v", receipt, err)
	}

	receipt, err = runAction(func(c *fakeNBVSPClient) { c.readDrift = 0.001 })
	if err == nil || receipt.Status != "applied_unreconciled" || !strings.Contains(err.Error(), "readback did not match") {
		t.Fatalf("post-write drift must be unreconciled: receipt=%+v err=%v", receipt, err)
	}

	// A plugin surface without any display metadata admits no dB mapping.
	bare := &bareSurfaceClient{host: newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)}
	barePort := &StaticEQVSPPort{Client: bare}
	bareAction := nbAction()
	bareSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{bareAction}}
	if err := barePort.Preflight(context.Background(), bareSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err = barePort.Apply(context.Background(), bareAction, "k")
	if err == nil || receipt.Status != "failed" || !strings.Contains(err.Error(), "no usable dB display domain") {
		t.Fatalf("unusable domain not rejected: receipt=%+v err=%v", receipt, err)
	}
}

// bareSurfaceClient strips the display metadata from every parameter row so
// the port must fail closed instead of guessing a machine-specific range.
type bareSurfaceClient struct{ host *fakeNBVSPClient }

func (f *bareSurfaceClient) VSPStateSnapshot(ctx context.Context, scope string) (*kernel.VSPStateResult, error) {
	return f.host.VSPStateSnapshot(ctx, scope)
}

func (f *bareSurfaceClient) SendVSPLegacyCommandWithIDs(_ context.Context, command map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	switch command["cmd"] {
	case "get_plugin_parameters":
		f.host.commands = append(f.host.commands, command)
		f.host.requests = append(f.host.requests, requestID)
		f.host.txs = append(f.host.txs, txID)
		rows := []any{}
		for paramID := range f.host.params {
			rows = append(rows, map[string]any{"id": paramID, "param_id": paramID, "normalized_value": 0.5})
		}
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "parameters": rows}}, nil
	}
	return f.host.SendVSPLegacyCommandWithIDs(context.Background(), command, requestID, txID)
}

func (f *bareSurfaceClient) SendVSPCommandWithIDs(ctx context.Context, command string, args map[string]any, requestID, txID string) (*kernel.VSPCommandResult, error) {
	return f.host.SendVSPCommandWithIDs(ctx, command, args, requestID, txID)
}

func TestStaticEQVSPPortCustomCommandNameInjection(t *testing.T) {
	client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
	port := &StaticEQVSPPort{Client: client, CommandName: "eq_band_nudge"}
	action := nbAction()
	action.Command = "something_else"
	set := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, eqCut()); err == nil {
		t.Fatal("foreign command accepted by injected name gate")
	}
	action.Command = "eq_band_nudge"
	set.Actions = []orchestration.Action{action}
	if err := port.Preflight(context.Background(), set, eqCut()); err != nil {
		t.Fatalf("configured command rejected: %v", err)
	}
}

func TestStaticEQVSPPortReconcileStaysNotAppliedForPathOnlyActions(t *testing.T) {
	client := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
	port := &StaticEQVSPPort{Client: client}
	action := nbAction()
	set := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Reconcile(context.Background(), action, "k", eqCut())
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "not_applied" {
		t.Fatalf("path-only actions cannot reconcile durably, got %+v", receipt)
	}
}
