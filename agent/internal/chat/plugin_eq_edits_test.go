package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

func TestEQControlRefRoundTripAndTamperDetection(t *testing.T) {
	want := eqControlReference{Version: "eq-control-ref/v1", TrackID: "track-1", PluginID: "plugin-2",
		TopologyGeneration: "eqt1_abc", Section: "3", Shape: "bell"}
	encoded, err := encodeEQControlRef(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeEQControlRef(encoded)
	if err != nil || got != want {
		t.Fatalf("round trip got=%+v err=%v want=%+v", got, err, want)
	}
	tampered := encoded[:len(encoded)-1] + map[bool]string{true: "0", false: "1"}[encoded[len(encoded)-1] != '0']
	if _, err := decodeEQControlRef(tampered); err == nil {
		t.Fatal("tampered control_ref must be rejected")
	}
}

func TestParseEQEditRequestsRequiresRefsAndStandaloneUndo(t *testing.T) {
	if _, err := parseEQEditRequests(map[string]any{"edits": []map[string]any{{"action": "modify", "gain_db": -3}}}); err == nil || eqControlFailureCode(err) != "control_ref_required" {
		t.Fatalf("modify without ref err=%v", err)
	}
	if _, err := parseEQEditRequests(map[string]any{"edits": []map[string]any{{"action": "undo"}}}); err == nil || eqControlFailureCode(err) != "operation_ref_required" {
		t.Fatalf("undo without ref err=%v", err)
	}
	if _, err := parseEQEditRequests(map[string]any{"edits": []map[string]any{{
		"action": "upsert", "shape": "notch", "frequency_hz": 1000, "gain_db": -3,
	}}}); err == nil || eqControlFailureCode(err) != "unsupported_shape" {
		t.Fatalf("notch must be outside public protocol, err=%v", err)
	}
}

func TestParseEQEditRequestsRejectsNonFiniteValues(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := parseEQEditRequests(map[string]any{"edits": []map[string]any{{
			"action": "upsert", "shape": "bell", "frequency_hz": 1000.0, "gain_db": value,
		}}})
		if err == nil || eqControlFailureCode(err) != "invalid_field" {
			t.Fatalf("non-finite value %v err=%v", value, err)
		}
	}
}

func TestPlanAtomicEQEditsReservesSectionsAndMovesAllActivationsLast(t *testing.T) {
	summary := atomicEQTestSummary()
	gain := -3.0
	firstFrequency, secondFrequency := 1000.0, 5000.0
	edits := []eqEditRequest{
		{Action: "upsert", Shape: "bell", FrequencyHz: &firstFrequency, GainDB: &gain},
		{Action: "upsert", Shape: "bell", FrequencyHz: &secondFrequency, GainDB: &gain},
	}
	reserved := map[string]bool{}
	plans := []eqPlannedEdit{}
	for _, edit := range edits {
		plan, err := planEQEdit(summary, edit, "t", "p", "eqt1_test", reserved)
		if err != nil {
			t.Fatal(err)
		}
		reserved[plan.SectionKey] = true
		plans = append(plans, plan)
	}
	if plans[0].SectionKey == plans[1].SectionKey {
		t.Fatalf("two edits reused one section: %+v", plans)
	}
	writes, err := combineAtomicEQWrites(plans)
	if err != nil {
		t.Fatal(err)
	}
	seenActivation := false
	for _, write := range writes {
		activation := write.Role == "used" || write.Role == "disabled" || write.Role == "removed"
		if activation {
			seenActivation = true
		} else if seenActivation {
			t.Fatalf("non-activation write followed activation: %+v", writes)
		}
	}
	if !seenActivation {
		t.Fatalf("expected slot activations in %+v", writes)
	}
}

func TestPlanEQUpsertRejectsExplicitUnavailableQBeforeWrites(t *testing.T) {
	summary := atomicEQTestSummary()
	sections := mapRowsValue(summary["sections"])
	delete(sections[0], "q_bindings")
	frequency, gain, q := 1000.0, -3.0, 0.5
	_, err := planEQUpsert(summary, eqEditRequest{Action: "upsert", Shape: "bell", FrequencyHz: &frequency,
		GainDB: &gain, Q: &q}, "t", "p", "eqt1_test", map[string]bool{"2": true})
	if err == nil || eqControlFailureCode(err) != "explicit_field_unavailable" {
		t.Fatalf("missing explicit Q err=%v", err)
	}
}

func TestPlanEQReferencedEditRejectsStaleRefAndResidentRemove(t *testing.T) {
	summary := atomicEQTestSummary()
	section := mapRowsValue(summary["sections"])[0]
	section["deallocatable"] = false
	setEQTestAction(section, "remove", false, []string{"section_not_deallocatable"})
	ref, err := encodeEQControlRef(eqControlReference{Version: "eq-control-ref/v1", TrackID: "t", PluginID: "p",
		TopologyGeneration: "eqt1_old", Section: "1", Shape: "bell"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = planEQReferencedEdit(summary, eqEditRequest{Action: "disable", ControlRef: ref}, "t", "p", "eqt1_new", nil)
	if err == nil || eqControlFailureCode(err) != "stale_control_ref" {
		t.Fatalf("stale ref err=%v", err)
	}
	ref, _ = encodeEQControlRef(eqControlReference{Version: "eq-control-ref/v1", TrackID: "t", PluginID: "p",
		TopologyGeneration: "eqt1_new", Section: "1", Shape: "bell"})
	_, err = planEQReferencedEdit(summary, eqEditRequest{Action: "remove", ControlRef: ref}, "t", "p", "eqt1_new", nil)
	if err == nil || eqControlFailureCode(err) != "section_not_deallocatable" {
		t.Fatalf("resident remove err=%v", err)
	}
}

func TestCombineAtomicEQWritesRejectsParameterConflict(t *testing.T) {
	plans := []eqPlannedEdit{
		{Writes: []eqWriteStep{{ParamID: "gain", NormalizedValue: 0.25}}},
		{Writes: []eqWriteStep{{ParamID: "gain", NormalizedValue: 0.75}}},
	}
	if _, err := combineAtomicEQWrites(plans); err == nil || !strings.Contains(err.Error(), "parameter_conflict") {
		t.Fatalf("conflicting write err=%v", err)
	}
}

func TestEQWritesForRoleRejectsContinuousAndDiscreteOutOfRange(t *testing.T) {
	continuous := map[string]any{"gain_bindings": []map[string]any{{"param_id": "gain",
		"domain": map[string]any{"min": -12.0, "max": 12.0}}}}
	if _, err := eqWritesForRole(continuous, "gain", "gain", 18, -24, 24, "linear"); err == nil ||
		!strings.Contains(err.Error(), "outside") {
		t.Fatalf("continuous out-of-range err=%v", err)
	}
	discrete := map[string]any{"slope_bindings": []map[string]any{{"param_id": "slope", "domain": map[string]any{"scale": "enum"},
		"reachable_values": []map[string]any{{"normalized": 0.0, "physical": 12.0}, {"normalized": 1.0, "physical": 24.0}}}}}
	if _, err := eqWritesForRole(discrete, "slope", "slope", 48, 6, 96, "linear"); err == nil ||
		!strings.Contains(err.Error(), "outside") {
		t.Fatalf("discrete out-of-range err=%v", err)
	}
	rows, err := eqWritesForRole(discrete, "slope", "slope", 18, 6, 96, "linear")
	if err != nil || len(rows) != 1 || !rows[0].Quantized || rows[0].NormalizedValue != 0 {
		t.Fatalf("in-range discrete quantization rows=%+v err=%v", rows, err)
	}
	exactRows, err := eqWritesForRole(discrete, "slope", "slope", 12, 6, 96, "linear")
	if err != nil || len(exactRows) != 1 || exactRows[0].Quantized || !exactRows[0].DiscretePhysical {
		t.Fatalf("exact discrete write must skip continuous correction: rows=%+v err=%v", exactRows, err)
	}
}

func TestStoreEQOperationEvictsOldestByCreationTime(t *testing.T) {
	server := &Server{eqOperations: map[string]eqOperationRecord{}}
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	for index := 0; index < eqOperationJournalCapacity; index++ {
		key := fmt.Sprintf("eqop_%03d", eqOperationJournalCapacity-index)
		server.eqOperations[key] = eqOperationRecord{OperationRef: key, CreatedAt: base.Add(time.Duration(index) * time.Second)}
	}
	server.storeEQOperation(eqOperationRecord{OperationRef: "eqop_new", CreatedAt: base.Add(time.Hour)})
	if _, exists := server.eqOperations["eqop_256"]; exists {
		t.Fatal("oldest operation was not evicted")
	}
	if _, exists := server.eqOperations["eqop_001"]; !exists {
		t.Fatal("lexicographically smallest but newest existing operation was evicted")
	}
}

func TestApplyEQEditsHTTPRouteExecutesAtomicTransactionAndUndo(t *testing.T) {
	transport := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = transport
	body, _ := json.Marshal(map[string]any{"tool": pluginGrabberApplyEQEditsTool, "args": map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "atomic": true,
		"edits": []map[string]any{{"action": "upsert", "shape": "bell", "frequency_hz": 3400.0, "gain_db": -3.0}},
	}})
	recorder := httptest.NewRecorder()
	server.handleInvoke(recorder, httptest.NewRequest(http.MethodPost, "/api/invoke", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	result := mapValue(response["result"])
	operationRef := firstNonEmptyText(result, "operation_ref")
	edits := mapRowsValue(result["edits"])
	if response["status"] != "ok" || operationRef == "" || len(edits) != 1 ||
		firstNonEmptyText(edits[0], "control_ref") == "" || len(transport.batchCalls) != 1 {
		t.Fatalf("response=%+v batches=%+v", response, transport.batchCalls)
	}
	rollback := mapValue(result["rollback"])
	if rollback["expires_on_restart"] != true || rollback["lifetime"] != "process" {
		t.Fatalf("rollback expiry metadata=%+v", rollback)
	}
	undo, err := server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
			"action": "undo", "operation_ref": operationRef,
		}}}, nil)
	if err != nil || undo["action"] != "undo" || len(transport.batchCalls) != 2 {
		t.Fatalf("undo=%+v err=%v batches=%d", undo, err, len(transport.batchCalls))
	}
}

func TestApplyEQEditsBatchFailureAndReadbackFailureRestoreFullPreimage(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*fakeEQKernel)
	}{
		{name: "batch failure after partial mutation", configure: func(fake *fakeEQKernel) { fake.failBatchAt[1] = true }},
		{name: "fresh readback failure", configure: func(fake *fakeEQKernel) { fake.failReadAt[2] = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := newFakeEQKernel()
			tc.configure(transport)
			before := transport.snapshot()
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = transport
			_, err := server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
				"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
					"action": "upsert", "shape": "bell", "frequency_hz": 3400.0, "gain_db": -3.0,
				}}}, nil)
			if err == nil || !strings.Contains(err.Error(), "full preimage restored") {
				t.Fatalf("err=%v", err)
			}
			if !fakeEQSnapshotsEqual(before, transport.snapshot()) {
				t.Fatalf("preimage=%v after=%v", before, transport.snapshot())
			}
			if len(transport.batchCalls) != 2 {
				t.Fatalf("batch calls=%d, want mutation plus rollback", len(transport.batchCalls))
			}
		})
	}
}

func TestApplyEQEditsRejectsAndRestoresUnplannedParameterSideEffect(t *testing.T) {
	transport := newFakeEQKernel()
	transport.sideEffectAt[1] = map[string]float64{"parent": 0}
	before := transport.snapshot()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = transport
	_, err := server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
			"action": "upsert", "shape": "bell", "frequency_hz": 3400.0, "gain_db": -3.0,
		}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "unplanned parameter changes: parent") ||
		!strings.Contains(err.Error(), "full preimage restored") ||
		eqControlFailureCode(err) != "unplanned_parameter_change" {
		t.Fatalf("side-effect err=%v", err)
	}
	if !fakeEQSnapshotsEqual(before, transport.snapshot()) {
		t.Fatalf("preimage=%v after=%v", before, transport.snapshot())
	}
	if len(transport.batchCalls) != 2 {
		t.Fatalf("batch calls=%d, want mutation plus rollback", len(transport.batchCalls))
	}
	rollback := transport.batchCalls[1]
	if firstNonEmptyText(rollback[len(rollback)-1], "parameter_id") != "parent" {
		t.Fatalf("side-effect parameter must restore after planned activation/state: %+v", rollback)
	}
}

func TestApplyEQEditsDoesNotClaimRestoredWhenRollbackFails(t *testing.T) {
	transport := newFakeEQKernel()
	transport.failBatchAt[1] = true
	transport.failBatchAt[2] = true
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = transport
	_, err := server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
			"action": "upsert", "shape": "bell", "frequency_hz": 3400.0, "gain_db": -3.0,
		}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "rollback failed") || strings.Contains(err.Error(), "full preimage restored") {
		t.Fatalf("err=%v", err)
	}
}

func TestUndoRejectsOperationStateConflictWithoutWriting(t *testing.T) {
	transport := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = transport
	result, err := server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
			"action": "upsert", "shape": "bell", "frequency_hz": 3400.0, "gain_db": -3.0,
		}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	transport.params["b1g"] = 0.9
	writesBefore := len(transport.batchCalls)
	_, err = server.applyPluginGrabberEQEdits(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "edits": []map[string]any{{
			"action": "undo", "operation_ref": result["operation_ref"],
		}}}, nil)
	if err == nil || eqControlFailureCode(err) != "operation_state_conflict" {
		t.Fatalf("conflict err=%v", err)
	}
	if len(transport.batchCalls) != writesBefore {
		t.Fatal("conflicted undo wrote parameters")
	}
}

func TestSetEQPointCompatibilityAdapterReturnsReferences(t *testing.T) {
	transport := newFakeEQKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = transport
	response := server.runPluginGrabberSetEQPointWorkflow(context.Background(), "conv", map[string]any{
		"track_id": "track-1", "plugin_id": "eq-1", "freq_hz": 3400.0, "gain_db": -3.0, "shape": "bell",
	}, nil)
	if response.Error != "" || len(response.ExecutedKernelReply) != 1 {
		t.Fatalf("response=%+v", response)
	}
	result := mapValue(response.ExecutedKernelReply[0]["result"])
	if firstNonEmptyText(result, "operation_ref") == "" ||
		firstNonEmptyText(mapRowsValue(result["edits"])[0], "control_ref") == "" ||
		result["compatibility_adapter"] != pluginGrabberSetEQPointTool {
		t.Fatalf("compat result=%+v", result)
	}
}

func TestApplyEQEditsIsExposedToAgentToolSet(t *testing.T) {
	found := false
	for _, tool := range agentLoopPluginTools() {
		if tool == pluginGrabberApplyEQEditsTool {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s missing from agent plugin tools", pluginGrabberApplyEQEditsTool)
	}
}

func atomicEQTestSummary() map[string]any {
	sections := []map[string]any{}
	for index, current := range []float64{1000, 5000} {
		key := string(rune('1' + index))
		sections = append(sections, map[string]any{
			"section": key, "complete": true, "dedicated_kind": "bell", "reachable_kinds": []string{"bell"},
			"addressing": "allocatable", "deallocatable": true, "active": false, "activation_strategy": "explicit_binding",
			"activation": map[string]any{"strategy": "explicit_binding", "active": false},
			"shape_capabilities": []map[string]any{{"shape": "bell", "actions": map[string]any{
				"upsert": true, "modify": true, "disable": true, "remove": true, "undo": true,
			}, "rejection_codes": map[string]any{}}},
			"frequency_bindings": []map[string]any{{"param_id": "f" + key, "channel": "shared", "current_physical": current,
				"domain": map[string]any{"min": 20.0, "max": 20000.0, "scale": "log"}}},
			"gain_bindings": []map[string]any{{"param_id": "g" + key, "channel": "shared",
				"domain": map[string]any{"min": -12.0, "max": 12.0, "scale": "linear"}}},
			"q_bindings": []map[string]any{{"param_id": "q" + key, "channel": "shared",
				"domain": map[string]any{"min": 0.1, "max": 10.0, "scale": "log"}}},
			"activation_bindings": []map[string]any{{"param_id": "u" + key, "channel": "shared", "activation_kind": "used",
				"reachable_values": []map[string]any{{"normalized": 0.0, "label": "Unused"}, {"normalized": 1.0, "label": "Used"}}}},
		})
	}
	return map[string]any{"eq_model": "free_floating", "sections": sections, "supported_filter_kinds": []string{"bell"},
		"control_topology": map[string]any{"generation": "eqt1_test"}}
}

func setEQTestAction(section map[string]any, action string, supported bool, codes []string) {
	capability := mapRowsValue(section["shape_capabilities"])[0]
	mapValue(capability["actions"])[action] = supported
	mapValue(capability["rejection_codes"])[action] = codes
}

type fakeEQKernel struct {
	params       map[string]float64
	batchCalls   [][]map[string]any
	readCalls    int
	failBatchAt  map[int]bool
	failReadAt   map[int]bool
	sideEffectAt map[int]map[string]float64
}

func newFakeEQKernel() *fakeEQKernel {
	return &fakeEQKernel{
		params: map[string]float64{
			"b1f": 0.10, "b1g": 0.50, "b1q": 0.20,
			"b2f": 0.40, "b2g": 0.50, "b2q": 0.20,
			"parent": 1.0,
		},
		failBatchAt: map[int]bool{}, failReadAt: map[int]bool{}, sideEffectAt: map[int]map[string]float64{},
	}
}

func (fake *fakeEQKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	fake.readCalls++
	if fake.failReadAt[fake.readCalls] {
		return nil, "", errors.New("injected readback failure")
	}
	rows := make([]map[string]any, 0, 6)
	for band := 1; band <= 2; band++ {
		prefix := fmt.Sprintf("b%d", band)
		rows = append(rows,
			fake.eqParameter(prefix+"f", fmt.Sprintf("Band %d Frequency", band), "frequency"),
			fake.eqParameter(prefix+"g", fmt.Sprintf("Band %d Gain", band), "gain"),
			fake.eqParameter(prefix+"q", fmt.Sprintf("Band %d Q", band), "q"),
		)
	}
	rows = append(rows, fake.eqParameter("parent", "Parent State", "state"))
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "eq-1",
		"template_role": "eq", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeEQKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	parameters := mapRowsValue(args["parameters"])
	copyRows := make([]map[string]any, 0, len(parameters))
	for _, row := range parameters {
		copyRow := map[string]any{"parameter_id": row["parameter_id"], "normalized_value": row["normalized_value"]}
		copyRows = append(copyRows, copyRow)
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if !ok || id == "" {
			return nil, fmt.Errorf("invalid fake batch row %v", row)
		}
		fake.params[id] = value
	}
	fake.batchCalls = append(fake.batchCalls, copyRows)
	for id, value := range fake.sideEffectAt[len(fake.batchCalls)] {
		fake.params[id] = value
	}
	if fake.failBatchAt[len(fake.batchCalls)] {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected partial batch failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeEQKernel) eqParameter(id, name, role string) map[string]any {
	normalized := fake.params[id]
	minimum, maximum, unit := 0.1, 10.0, ""
	switch role {
	case "frequency":
		minimum, maximum, unit = 20, 20000, " Hz"
	case "gain":
		minimum, maximum, unit = -12, 12, " dB"
	}
	physical := minimum + normalized*(maximum-minimum)
	samples := make([]map[string]any, 0, 5)
	for _, point := range []float64{0, 0.25, 0.5, 0.75, 1} {
		value := minimum + point*(maximum-minimum)
		samples = append(samples, map[string]any{"normalized_value": point,
			"text": fmt.Sprintf("%.6f%s", value, unit)})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true,
		"normalized_value": normalized, "value_text": fmt.Sprintf("%.6f%s", physical, unit),
		"display_probe": map[string]any{"mode": "samples", "samples": samples}}
}

func (fake *fakeEQKernel) snapshot() map[string]float64 {
	out := make(map[string]float64, len(fake.params))
	for key, value := range fake.params {
		out[key] = value
	}
	return out
}

func fakeEQSnapshotsEqual(left, right map[string]float64) bool {
	if len(left) != len(right) {
		return false
	}
	for key, expected := range left {
		if actual, ok := right[key]; !ok || math.Abs(actual-expected) > 1e-9 {
			return false
		}
	}
	return true
}
