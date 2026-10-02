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
	case "rack_add_node":
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
		return &kernel.VSPCommandResult{TransactionID: txID, LegacyReply: map[string]any{"status": "ok", "plugin_id": pluginID, "rack_item_id": "rack_1"}}, nil
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
	// F4A nail 3 (reuse semantics, no regression): an action that carries a
	// resolved plugin_id must issue zero load commands of either form.
	for _, command := range client.commands {
		if command["cmd"] == "rack_add_node" || command["cmd"] == "instantiate_plugin" {
			t.Fatalf("reuse must not load a plugin: %#v", command)
		}
	}
}

func TestStaticEQVSPPortLoadsRackNodeWhenPluginIDMissing(t *testing.T) {
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
	// F4A: the governed load issues rack_add_node (rack-wrapped form, visible
	// in the Godot rack like a manual drag) with the geometry fields the
	// kernel handler requires; the reply's plugin_id feeds the parameter write.
	load := client.commands[0]
	if load["cmd"] != "rack_add_node" || load["track_id"] != "t1" || load["plugin_identifier"] != "juce_eq" {
		t.Fatalf("expected rack_add_node load first: %#v", load)
	}
	x, xOK := load["x"].(float64)
	y, yOK := load["y"].(float64)
	if !xOK || !yOK || load["zone_id"] != "Z3" || load["auto_connect"] != true {
		t.Fatalf("rack_add_node load must carry numeric x/y, zone and auto_connect: %#v", load)
	}
	if x == 0 || y == 0 {
		t.Fatalf("rack node placement must be a real canvas position, got x=%v y=%v", x, y)
	}
	if client.requests[0] != "execution:eq:a1:instantiate" {
		t.Fatalf("load keeps its idempotency-key suffix: %v", client.requests)
	}
}

// F4A nail 1: both identity forms load through rack_add_node with the same
// geometry fields — the known-list identifier route and the real-plugin
// plugin_path route (which must keep the identifier absent so the kernel
// resolves by path).
func TestStaticEQVSPPortLoadRackAddNodePayloadCoversBothIdentityForms(t *testing.T) {
	identifierClient := &fakeEQVSPClient{fakeVSPClient: fakeVSPClient{snapshots: eqSnapshots()},
		paramValues: map[string]float64{}, instantiated: map[string]string{}}
	port := &StaticEQVSPPort{Client: identifierClient}
	action := eqAction()
	delete(action.Args, "plugin_id")
	action.Args["plugin_identifier"] = "juce_eq"
	set := orchestration.ActionSet{ID: "as", ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), set, eqCut()); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Apply(context.Background(), action, "k"); err != nil {
		t.Fatal(err)
	}
	identifierLoad := identifierClient.commands[0]
	if identifierLoad["cmd"] != "rack_add_node" || identifierLoad["plugin_identifier"] != "juce_eq" {
		t.Fatalf("identifier-form load: %#v", identifierLoad)
	}
	if _, ok := identifierLoad["x"].(float64); !ok {
		t.Fatalf("identifier-form load must carry numeric x: %#v", identifierLoad)
	}
	if _, ok := identifierLoad["zone_id"].(string); !ok || identifierLoad["zone_id"] == "" {
		t.Fatalf("identifier-form load must carry a zone: %#v", identifierLoad)
	}

	pathClient := newFakeNBVSPClient("p315_c1", "p315_c2", -24, 24, true)
	pathPort := &StaticEQVSPPort{Client: pathClient}
	pathAction := nbAction()
	pathSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{pathAction}}
	if err := pathPort.Preflight(context.Background(), pathSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	if _, err := pathPort.Apply(context.Background(), pathAction, "k"); err != nil {
		t.Fatal(err)
	}
	pathLoad := pathClient.commands[0]
	if pathLoad["cmd"] != "rack_add_node" || pathLoad["plugin_path"] != "C:/plugins/Fixture EQ.vst3" {
		t.Fatalf("path-form load: %#v", pathLoad)
	}
	if _, present := pathLoad["plugin_identifier"]; present {
		t.Fatalf("path-form load must keep the identifier absent: %#v", pathLoad)
	}
	if _, ok := pathLoad["y"].(float64); !ok || pathLoad["auto_connect"] != true || pathLoad["zone_id"] == "" {
		t.Fatalf("path-form load must carry y, auto_connect and zone: %#v", pathLoad)
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

// fakeNBStep is one position of a discrete stepped control exactly as the
// kernel reports it: the state's normalized position (float32 promoted, like
// the real VST3 host round-trips it) and its display text.
type fakeNBStep struct {
	Normalized float64
	Text       string
}

// steppedGrid550A mirrors the API-550A Stereo Mid Gain surface measured on
// the real kernel (D1-EQ-READBACK-550A-1, forensic1_20261002_111423): eleven
// detents with non-linear dB spacing, no display-domain candidate, raw range
// 0..1.
var steppedGrid550A = []fakeNBStep{
	{0.0, "-12 dB "}, {float64(float32(0.1)), "-9 dB "}, {float64(float32(0.2)), "-6 dB "},
	{float64(float32(0.3)), "-4 dB "}, {float64(float32(0.4)), "-2 dB "}, {0.5, "0 dB "},
	{float64(float32(0.6)), "+2 dB "}, {float64(float32(0.7)), "+4 dB "}, {float64(float32(0.8)), "+6 dB "},
	{float64(float32(0.9)), "+9 dB "}, {1.0, "+12 dB "},
}

// fakeNBVSPClient simulates a real VST3 plugin host for the normalized batch
// pipeline: path instantiation, a full parameter surface with display-domain
// metadata, the typed plugin.set_params_batch command, and normalized
// readback. Domains are [-24, +24] dB linear unless stated otherwise. A
// non-empty stepped grid turns every parameter into a discrete stepped
// control: writes snap to the nearest detent and the surface reports the
// kernel's discrete metadata.
type fakeNBVSPClient struct {
	fakeVSPClient
	domainMin, domainMax float64
	withCandidate        bool   // expose display_domain_candidate; probe samples are always present
	frozenPhysicalText   bool   // live value_text ignores writes (degenerate threshold display)
	curveExponent        float64 // >0 bends physical = min + span*norm^k and switches text to the "+x.xx" form
	displayQuantum       float64 // >0 rounds the curved display text to this dB step (coarse compressor readouts)
	stepped              []fakeNBStep
	params               map[string]*fakeNBParam
	batchArgs            map[string]any
	batchFailure         string // when set, the typed batch answers partial_failure
	swallowWrites        bool   // kernel keeps old values despite ok status
	readDrift            float64
	failSurface          bool
}

// snapToStep emulates a stepped control's write behavior: any requested
// normalized value lands on the nearest reachable detent.
func (f *fakeNBVSPClient) snapToStep(normalized float64) float64 {
	best, bestGap := f.stepped[0].Normalized, math.Abs(normalized-f.stepped[0].Normalized)
	for _, step := range f.stepped[1:] {
		if gap := math.Abs(normalized - step.Normalized); gap < bestGap {
			best, bestGap = step.Normalized, gap
		}
	}
	return best
}

// stepForNormalized resolves the detent a normalized position holds on.
func (f *fakeNBVSPClient) stepForNormalized(normalized float64) fakeNBStep {
	step := f.stepped[0]
	gap := math.Abs(normalized - step.Normalized)
	for _, candidate := range f.stepped[1:] {
		if g := math.Abs(normalized - candidate.Normalized); g < gap {
			step, gap = candidate, g
		}
	}
	return step
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
	if len(f.stepped) > 0 {
		step := f.stepForNormalized(state.normalized)
		samples := []any{}
		for _, sampleNormalized := range []float64{0, 0.25, 0.5, 0.75, 1} {
			samples = append(samples, map[string]any{
				"normalized_value": sampleNormalized,
				"text":             f.stepForNormalized(sampleNormalized).Text,
			})
		}
		labels := []any{}
		for index, position := range f.stepped {
			labels = append(labels, map[string]any{
				"index": index, "value": position.Normalized, "label": position.Text,
			})
		}
		return map[string]any{
			"id": paramID, "param_id": paramID,
			"normalized_value": state.normalized, "value_text": step.Text,
			"is_discrete": true, "num_steps": len(f.stepped),
			"display_probe": map[string]any{"mode": "read_only_value_to_string", "samples": samples, "discrete_labels": labels},
		}
	}
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
	case "rack_add_node":
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
					value := entry["normalized_value"].(float64)
					if len(f.stepped) > 0 {
						value = f.snapToStep(value)
					}
					state.normalized = value
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

// ---- stepped vs continuous absolute-semantics regression (550A vs Q3) --------

// staticEQAbsoluteAction targets one band gain parameter with an absolute dB
// value through the normalized batch pipeline.
func staticEQAbsoluteAction(paramID string, targetDB float64) orchestration.Action {
	return orchestration.Action{ID: "a-abs", Command: staticEQActionCommand, TargetRef: "t1",
		BeforeFingerprint: "track:t1:eq:plg_1:pending",
		Args: map[string]any{
			"write_mode": WriteModeNormalizedBatchV1,
			"plugin_path": "/Library/Audio/Plug-Ins/VST3/WaveShell1-VST3 17.1.vst3",
			"param_id":    paramID,
			"target_value": targetDB,
		}}
}

// The 550A family on the absolute path: the requested dB maps onto the
// nearest detent's normalized position instead of a continuous inversion.
func TestStaticEQVSPPortSteppedAbsoluteSnapsToNearestDetent(t *testing.T) {
	client := newFakeNBVSPClient("2", "2", 0, 1, false)
	client.stepped = steppedGrid550A
	client.params["2"].normalized = 0.5
	port := &StaticEQVSPPort{Client: client}
	action := staticEQAbsoluteAction("2", -2)
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:eq:a-abs")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("stepped absolute must apply: receipt=%+v err=%v", receipt, err)
	}
	parameters, _ := client.batchArgs["parameters"].([]map[string]any)
	if len(parameters) != 1 {
		t.Fatalf("single-channel absolute batch expected: %+v", client.batchArgs)
	}
	wantNormalized := float64(float32(0.4)) // -2 dB detent position
	if got := parameters[0]["normalized_value"].(float64); math.Abs(got-wantNormalized) > 1e-12 {
		t.Fatalf("requested normalized %v want detent %v", got, wantNormalized)
	}
	if got := receipt.Details["actual_readback_value"].(float64); math.Abs(got-(-2)) > eqSteppedPhysicalToleranceDB {
		t.Fatalf("actual_readback_value=%v want -2", receipt.Details["actual_readback_value"])
	}
	if receipt.Details["readback_verified"] != true {
		t.Fatalf("readback must verify: %+v", receipt.Details)
	}
}

// The Q3 family control guard: a continuous gain parameter keeps the exact
// linear mapping (byte-identical legacy pipeline, no stepped metadata).
func TestStaticEQVSPPortContinuousAbsoluteKeepsLinearMapping(t *testing.T) {
	client := newFakeNBVSPClient("12", "12", -18, 18, true) // Q3 Band 3 Gain: continuous, linear ±18 dB
	port := &StaticEQVSPPort{Client: client}
	action := staticEQAbsoluteAction("12", -3)
	actionSet := orchestration.ActionSet{ProjectCutHash: "cut-eq", Actions: []orchestration.Action{action}}
	if err := port.Preflight(context.Background(), actionSet, eqCut()); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), action, "execution:eq:a-abs")
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("continuous absolute must apply: receipt=%+v err=%v", receipt, err)
	}
	parameters, _ := client.batchArgs["parameters"].([]map[string]any)
	wantNormalized := (-3.0 - (-18.0)) / 36.0
	if got := parameters[0]["normalized_value"].(float64); math.Abs(got-wantNormalized) > 1e-12 {
		t.Fatalf("requested normalized %v want linear %v", got, wantNormalized)
	}
	if _, has := receipt.Details["delta_calibration"]; has {
		t.Fatalf("continuous absolute must not carry delta metadata: %+v", receipt.Details)
	}
	if got := receipt.Details["actual_readback_value"].(float64); math.Abs(got-(-3)) > 0.01 {
		t.Fatalf("actual_readback_value=%v want -3", receipt.Details["actual_readback_value"])
	}
}
