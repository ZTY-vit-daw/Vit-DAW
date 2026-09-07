package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/semanticeffect"
	"vit-daw-agent/internal/shadow"
)

// The D1 evaluator (scripts/free_state_d1_smoke.py:1224-1241, :1483) asserts the
// native port receipt shape on the booked intervention receipt regardless of
// which workflow produced the forward mutation. This Go mirror returns one
// entry per failed assertion so tests can pin exactly which contract keys a
// receipt is missing.
func d1ReceiptNativeShapeFailures(receipt map[string]any) []string {
	var failures []string
	textKey := func(key string) bool {
		value, ok := receipt[key].(string)
		return ok && strings.TrimSpace(value) != ""
	}
	before, after := firstNonEmptyText(receipt, "before_revision"), firstNonEmptyText(receipt, "after_revision", "applied_revision")
	if before == "" || after == "" || before == after {
		failures = append(failures, "D1 receipt requires distinct before/after revisions")
	}
	// Python mirror of `receipt.get(key) not in (None, "")`: present, non-nil,
	// and a string value must be non-blank (numeric readbacks pass as-is).
	for _, key := range []string{"transaction_id", "idempotency_key", "actual_readback_value"} {
		value, present := receipt[key]
		blank := !present || value == nil
		if text, isText := value.(string); isText && strings.TrimSpace(text) == "" {
			blank = true
		}
		if blank {
			failures = append(failures, fmt.Sprintf("D1 execution receipt missing %s", key))
		}
	}
	for _, key := range []string{"plugin_id", "param_id"} {
		if !textKey(key) {
			failures = append(failures, fmt.Sprintf("broadband_compression execution receipt missing %s", key))
		}
	}
	if receipt["readback_verified"] != true {
		failures = append(failures, "D1 actual readback was not verified")
	}
	return failures
}

func d1ReceiptFailureContains(t *testing.T, failures []string, want string) {
	t.Helper()
	for _, failure := range failures {
		if strings.Contains(failure, want) {
			return
		}
	}
	t.Fatalf("shape failures %v do not mention %q", failures, want)
}

// RED pin (RECEIPT-1): the historical semantic_effect.compressor_execution_receipt.v1
// shape — exactly what executeSemanticCompressorTicket booked before the
// settlement bridge — cannot satisfy the D1 evaluator's native-port receipt
// contract. The real-stack run 20260907_185746 failed deterministically on the
// first of these assertions ("D1 receipt requires distinct before/after
// revisions") after a genuinely applied mutation.
func TestCompressorReceiptV1ShapeFailsD1NativeContract(t *testing.T) {
	receipt := map[string]any{"schema_version": "semantic_effect.compressor_execution_receipt.v1", "status": "executed",
		"ticket_id": "semexec_v1", "track_id": "track-1", "plugin_id": "comp-1", "topology_generation": "gen-1",
		"parameter_audit": map[string]any{"changed": 1}, "controller_result": map[string]any{"status": "exact"},
		"rollback": map[string]any{"status": "not_needed", "verified": true}}
	failures := d1ReceiptNativeShapeFailures(receipt)
	d1ReceiptFailureContains(t, failures, "distinct before/after revisions")
	d1ReceiptFailureContains(t, failures, "transaction_id")
	d1ReceiptFailureContains(t, failures, "idempotency_key")
	d1ReceiptFailureContains(t, failures, "actual_readback_value")
	d1ReceiptFailureContains(t, failures, "param_id")
	d1ReceiptFailureContains(t, failures, "readback")
}

// semanticCompressorSettlementKernel extends the compressor fake with the
// ID-carrying VSP transport so the accounted execution can echo a kernel-real
// transaction identity (mirrors the kernel's makeBatchTransactionId).
type semanticCompressorSettlementKernel struct {
	fakeCompressorKernel
	requestIDs []string
}

func (fake *semanticCompressorSettlementKernel) SendVSPCommandWithIDs(ctx context.Context, command string, args map[string]any, requestID, transactionID string) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	fake.requestIDs = append(fake.requestIDs, requestID)
	result, err := fake.SendVSPCommand(ctx, command, args)
	if err != nil {
		return nil, err
	}
	result.TransactionID = "tx_" + requestID
	return result, nil
}

// The compressor typed controller must surface the kernel transaction identity
// when the caller pins a request id, and keep the historical result shape when
// it does not (RECEIPT-1, mirroring the five dynamic-family controller
// contracts).
func TestCompressorControllerCarriesKernelTransactionWhenRequested(t *testing.T) {
	fake := &semanticCompressorSettlementKernel{fakeCompressorKernel: *newFakeCompressorKernel()}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake

	_, summary, err := server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
		t.Fatal(err)
	}
	ref := compressorTestControlRef(summary, "threshold")
	if ref == "" {
		t.Fatal("typed surface did not disclose a threshold control_ref")
	}

	plain, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}},
	}, map[string]any{})
	if err != nil || firstNonEmptyText(plain, "status") != "exact" {
		t.Fatalf("plain apply=%+v err=%v", plain, err)
	}
	if _, present := plain["transaction_id"]; present {
		t.Fatalf("unrequested accounting leaked into the controller result: %v", plain["transaction_id"])
	}
	if _, present := plain["idempotency_key"]; present {
		t.Fatalf("unrequested idempotency leaked into the controller result: %v", plain["idempotency_key"])
	}
	if len(fake.requestIDs) != 0 {
		t.Fatalf("anonymous write rode the accounted transport: %v", fake.requestIDs)
	}

	fake = &semanticCompressorSettlementKernel{fakeCompressorKernel: *newFakeCompressorKernel()}
	server.eqKernelOverride = fake
	_, summary, err = server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
		t.Fatal(err)
	}
	ref = compressorTestControlRef(summary, "threshold")
	accounted, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "atomic": true, "request_id": "semexec_settle_test",
		"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}},
	}, map[string]any{})
	if err != nil || firstNonEmptyText(accounted, "status") != "exact" {
		t.Fatalf("accounted apply=%+v err=%v", accounted, err)
	}
	if accounted["idempotency_key"] != "semexec_settle_test" {
		t.Fatalf("idempotency_key=%v", accounted["idempotency_key"])
	}
	transaction, _ := accounted["transaction_id"].(string)
	if strings.TrimSpace(transaction) == "" {
		t.Fatalf("kernel transaction id missing from accounted result: %+v", accounted)
	}
	if len(fake.requestIDs) != 1 || fake.requestIDs[0] != "semexec_settle_test" {
		t.Fatalf("net write request ids=%v", fake.requestIDs)
	}
}

// §11 persistence compatibility: a persisted v1 receipt (no native keys) must
// round-trip through the experiment intervention record with its missing keys
// still missing — absent keys mean "not evaluable", never synthesized
// defaults — while a v2 receipt round-trips with every key intact.
func TestCompressorReceiptRoundTripPreservesV1MissingKeySemantics(t *testing.T) {
	v1 := map[string]any{"schema_version": "semantic_effect.compressor_execution_receipt.v1", "status": "executed",
		"ticket_id": "semexec_v1", "track_id": "track-1", "plugin_id": "comp-1", "topology_generation": "gen-1",
		"parameter_audit": map[string]any{"changed": 1}, "controller_result": map[string]any{"status": "exact"},
		"rollback": map[string]any{"status": "not_needed", "verified": true}}
	v2 := map[string]any{"schema_version": semanticCompressorReceiptSchema, "status": "executed",
		"ticket_id": "semexec_v2", "track_id": "track-1", "plugin_id": "comp-1", "topology_generation": "gen-1",
		"before_revision": "5", "after_revision": "6", "transaction_id": "tx_semexec_v2", "idempotency_key": "semexec_v2",
		"param_id": "threshold", "actual_readback_value": -15.0, "readback_verified": true,
		"receipt_id": "d1_semantic_semexec_v2", "settlement_source": "semantic_compressor_settlement_bracket.v1"}
	for name, receipt := range map[string]map[string]any{"v1": v1, "v2": v2} {
		intervention := experiment.Intervention{ID: "action-" + name, Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, Receipt: receipt}
		encoded, err := json.Marshal(intervention)
		if err != nil {
			t.Fatal(err)
		}
		var decoded experiment.Intervention
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if name == "v2" {
			if decoded.Receipt["schema_version"] != semanticCompressorReceiptSchema {
				t.Fatalf("v2 schema changed on round trip: %v", decoded.Receipt["schema_version"])
			}
			if failures := d1ReceiptNativeShapeFailures(decoded.Receipt); len(failures) != 0 {
				t.Fatalf("v2 receipt lost native keys on round trip: %v (%v)", decoded.Receipt, failures)
			}
			continue
		}
		for _, key := range []string{"before_revision", "after_revision", "transaction_id", "idempotency_key", "actual_readback_value", "param_id", "readback_verified"} {
			if _, present := decoded.Receipt[key]; present {
				t.Fatalf("v1 receipt grew %s on round trip (fabricated default): %v", key, decoded.Receipt[key])
			}
		}
		if failures := d1ReceiptNativeShapeFailures(decoded.Receipt); len(failures) == 0 {
			t.Fatal("v1 receipt unexpectedly satisfies the native contract")
		}
	}
}

func semanticCompressorSettlementInteraction(loop freeStateReasoningLoop, ticket compressorExecutionTicket) PendingInteraction {
	return PendingInteraction{
		ID: "interaction-compressor-settle", Kind: "confirmation", Workflow: semanticCompressorExecutionWorkflow,
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		RequestContext: map[string]any{
			"free_state_semantic_processor_intent": map[string]any{
				"schema_version": "semantic_processor_intent.v1", "family": "broadband_compression",
				"status": "resolved", "processor_type": "compressor",
			},
			// The unit seam skips the harness-backed paired COM probes; the
			// settlement keys under test are orthogonal to the COM mode and the
			// paired+D1 combination is exercised by the real-stack p03.
			"c2_source_only_reversible": true,
		},
		Data: map[string]any{"ticket": ticket},
	}
}

func semanticCompressorSettlementTicket(t *testing.T, server *Server, ticketID string) compressorExecutionTicket {
	t.Helper()
	digest, summary, err := server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
		t.Fatal(err)
	}
	ref := compressorTestControlRef(summary, "threshold")
	if ref == "" {
		t.Fatal("typed surface did not disclose a threshold control_ref")
	}
	pathKey := firstNonEmptyText(mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0], "path_key")
	target := -15.0
	return compressorExecutionTicket{
		SchemaVersion: semanticCompressorExecutionSchema, TicketID: ticketID, ConversationID: "conversation-d1-comp",
		GoalID: "goal-d1-comp", RunID: "run-d1-comp", TrackID: "track-1", PluginID: "comp-1",
		TopologyClass:      firstNonEmptyText(summary, "classification"),
		TopologyGeneration: firstNonEmptyText(mapValue(summary["control_topology"]), "generation"),
		ParameterCount:     digest.ParameterCount, BaselineFingerprint: compressorParameterBaselineFingerprint(digest),
		Controls: []compressorExecutionControl{{ProposalID: "p1", Axis: "operating_point", PathKey: pathKey, Role: "threshold",
			ControlRef: ref, Target: semanticeffect.CompressorControlTarget{ValueDB: &target}}},
		CreatedAt: "2026-09-07T00:00:00Z",
	}
}

// GREEN (RECEIPT-1): a D1-gated semantic compressor execution must book an
// intervention receipt in the native port shape — kernel-real before/after
// revision bracket from VSP state snapshots, the kernel-derived transaction
// identity of the net write, the fresh-readback value (never the requested
// target), and the control_ref-decoded param_id — so the D1 evaluator's
// assertions pass with the evaluator itself untouched.
func TestSemanticCompressorExecutionSettlesReceiptWithNativeShapeKeysForD1(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	fake := &semanticCompressorSettlementKernel{fakeCompressorKernel: *newFakeCompressorKernel()}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(6),
	}}
	server.storeFreeStateLoop(loop)

	ticket := semanticCompressorSettlementTicket(t, server, "semexec_settle_test")
	interaction := semanticCompressorSettlementInteraction(loop, ticket)
	response := server.executeSemanticCompressorTicket(context.Background(), interaction, ticket)
	if response.GoalStatus != "completed" || strings.TrimSpace(response.Error) != "" {
		t.Fatalf("execution did not complete: status=%s err=%s data=%v", response.GoalStatus, response.Error, response.WorkflowData)
	}
	receipt := firstMapFromAny(response.WorkflowData["execution_receipt"])
	if receipt["schema_version"] != semanticCompressorReceiptSchema {
		t.Fatalf("receipt schema=%v want %s", receipt["schema_version"], semanticCompressorReceiptSchema)
	}
	if before, after := receipt["before_revision"], receipt["after_revision"]; before != "5" || after != "6" {
		t.Fatalf("revision bracket=%v/%v want 5/6 from the live snapshots", before, after)
	}
	if receipt["transaction_id"] != "tx_semexec_settle_test" || receipt["idempotency_key"] != "semexec_settle_test" {
		t.Fatalf("transaction identity=%v/%v", receipt["transaction_id"], receipt["idempotency_key"])
	}
	if len(fake.requestIDs) != 1 || fake.requestIDs[0] != "semexec_settle_test" {
		t.Fatalf("net write did not ride the accounted transport: %v", fake.requestIDs)
	}
	if receipt["param_id"] != "threshold" {
		t.Fatalf("param_id=%v want the control_ref-decoded threshold", receipt["param_id"])
	}
	if readback, _ := receipt["actual_readback_value"].(float64); readback != -15.0 {
		t.Fatalf("actual_readback_value=%v want the fresh post-write readback -15", receipt["actual_readback_value"])
	}
	if receipt["readback_verified"] != true {
		t.Fatalf("readback_verified=%v", receipt["readback_verified"])
	}
	if receipt["receipt_id"] == "" {
		t.Fatal("receipt_id missing: the booked intervention could not correlate to the journaled action")
	}
	if failures := d1ReceiptNativeShapeFailures(receipt); len(failures) != 0 {
		t.Fatalf("v2 receipt still fails the D1 native shape contract: %v", failures)
	}

	// The booked intervention receipt must carry the same shape with zero
	// accounting-side changes (freeStateAcousticActionOutcome clones the
	// execution receipt verbatim into the experiment round).
	processorType, actionStatus, booked, attempted := freeStateAcousticActionOutcome(interaction, response)
	if !attempted || processorType != "compressor" || actionStatus != "applied" {
		t.Fatalf("outcome=%s/%s attempted=%v", processorType, actionStatus, attempted)
	}
	server.recordFreeStateExperimentAction(&loop, processorType, actionStatus, booked)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Interventions) != 1 {
		t.Fatalf("round interventions=%d want the single forward mutation", len(round.Interventions))
	}
	if failures := d1ReceiptNativeShapeFailures(round.Interventions[0].Receipt); len(failures) != 0 {
		t.Fatalf("booked intervention receipt fails the D1 native shape contract: %v", failures)
	}
	if round.Interventions[0].ID != firstNonEmptyText(receipt, "receipt_id") {
		t.Fatalf("booked action id=%s does not correlate to receipt_id=%s", round.Interventions[0].ID, receipt["receipt_id"])
	}
}

// The settlement readback must come from the controller's fresh post-write
// readback row, never from the requested target: a controller row that
// requested -18 dB but actually read back -15 dB must book -15.
func TestSemanticCompressorSettlementActualReadbackNeverSubstitutesRequested(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(6),
	}}
	server.storeFreeStateLoop(loop)

	ticket := compressorExecutionTicket{TicketID: "semexec_readback_pin", TrackID: "track-1", PluginID: "comp-1"}
	interaction := PendingInteraction{ID: "interaction-readback", ConversationID: loop.ConversationID,
		RequestContext: map[string]any{"free_state_semantic_processor_intent": map[string]any{"family": "broadband_compression"}}}
	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil || bracket == nil {
		t.Fatalf("begin err=%v bracket=%v", err, bracket)
	}
	controller := map[string]any{
		"status": "exact", "atomic": true, "track_id": "track-1", "plugin_id": "comp-1",
		"transaction_id": "tx_kernel_real", "idempotency_key": bracket.RequestID,
		"writes": []map[string]any{{"param_id": "threshold", "role": "threshold",
			"requested_physical": -18.0, "actual_physical": -15.0, "actual_normalized": 0.75}},
	}
	settlement, ok := server.completeSemanticCompressorSettlement(context.Background(), bracket, ticket, controller)
	if !ok {
		t.Fatal("settlement completion failed")
	}
	if settlement["actual_readback_value"] != -15.0 {
		t.Fatalf("actual_readback_value=%v was substituted from the requested target", settlement["actual_readback_value"])
	}
}

// A mutation whose kernel settlement cannot be proven (the after snapshot did
// not advance the revision) must not book native shape keys: the receipt keeps
// its unverified marker and the D1 chain refuses downstream instead of
// trusting an unproven revision.
func TestSemanticCompressorSettlementStallRefusesNativeKeys(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(5),
	}}
	server.storeFreeStateLoop(loop)

	ticket := compressorExecutionTicket{TicketID: "semexec_stall", TrackID: "track-1", PluginID: "comp-1"}
	interaction := PendingInteraction{ID: "interaction-stall", ConversationID: loop.ConversationID,
		RequestContext: map[string]any{"free_state_semantic_processor_intent": map[string]any{"family": "broadband_compression"}}}
	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil || bracket == nil {
		t.Fatalf("begin err=%v bracket=%v", err, bracket)
	}
	controller := map[string]any{"transaction_id": "tx_kernel_real", "writes": []map[string]any{{"param_id": "threshold", "actual_normalized": 0.75}}}
	if settlement, ok := server.completeSemanticCompressorSettlement(context.Background(), bracket, ticket, controller); ok {
		t.Fatalf("stalled revision produced settlement: %v", settlement)
	}
	if actions := server.harness.Actions(10); len(actions) != 0 {
		t.Fatalf("unproven settlement was journaled: %+v", actions)
	}
}
