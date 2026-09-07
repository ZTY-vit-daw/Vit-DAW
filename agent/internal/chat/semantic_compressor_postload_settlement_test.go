package chat

// BRACKET-1 (2026-09-08): the load-first (load_required) D1 compressor flow
// hands the free-state intent over the post-load channel instead of the
// canonical free_state_semantic_processor_intent anchor the shared settlement
// core's owner predicate reads. VER-1 R4 (artifacts/free_state_d1_s1/
// 20260907_220935) reached the execution/receipt segment with the post-load
// key present in every request context while the canonical key had zero hits,
// so the bracket never opened and the v2 receipt lacked the native shape keys
// (OBS-1 x3, evaluator receipt assertion failure). These tests use the R4
// measured key shapes — never the pre-load canonical fixture alone — and pin
// that the compressor wrapper normalizes the post-load handoff into the
// canonical anchor while every other interaction keeps the historical shape.

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/shadow"
)

// compressorPostLoadHandoffContext builds the request context markers the
// real R4 execution interactions carried (measured from 20260907_220935
// responses 2-5 request_context): the compressor load-first handoff scalars,
// the durable loop map, and — for the exact R4 shape — the post-load intent
// key present with a nil value plus the family-bearing intent under
// processor_selection_route.semantic_processor_intent.
func compressorPostLoadHandoffContext(loop freeStateReasoningLoop, carryIntent map[string]any, r4Exact bool) map[string]any {
	requestContext := map[string]any{
		"conversation_id":                       loop.ConversationID,
		"goal_id":                               loop.GoalID,
		"run_id":                                loop.RunID,
		"free_state_reasoning_loop":             freeStateLoopMap(loop),
		"semantic_compressor_post_load_handoff": true,
		"semantic_compressor_post_load_goal":    "restore natural drums dynamics",
		"semantic_post_load_handoff":            true,
		"semantic_post_load_family":             processorintent.FamilyBroadbandCompressor,
		"semantic_post_load_planner":            "semantic_compressor",
		"semantic_post_load_goal":               "restore natural drums dynamics",
		"processor_selection_route": map[string]any{
			"action_domain": "broadband_compression",
			"route":         "processor_selection",
			"semantic_processor_intent": map[string]any{
				"family": processorintent.FamilyBroadbandCompressor,
				"intent": "restore natural drums dynamics",
			},
		},
	}
	if carryIntent != nil {
		requestContext["semantic_post_load_semantic_processor_intent"] = cloneContext(carryIntent)
	} else if r4Exact {
		// R4 measured shape: the key rides the request context with a nil
		// value (plugin_recommendation.go forwarded an absent payload map);
		// the family-bearing intent lives under the processor-selection route
		// record instead.
		requestContext["semantic_post_load_semantic_processor_intent"] = nil
	}
	return requestContext
}

func compressorPostLoadBracketFixture(t *testing.T, loop freeStateReasoningLoop, requestContext map[string]any) (*Server, PendingInteraction, compressorExecutionTicket) {
	t.Helper()
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	server := New(nil, nil, nil)
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5),
	}}
	server.storeFreeStateLoop(loop)
	interaction := PendingInteraction{
		ID: "interaction-compressor-postload", Kind: "confirmation", Workflow: semanticCompressorExecutionWorkflow,
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		RequestContext: requestContext,
	}
	ticket := compressorExecutionTicket{TicketID: "semexec_postload_settle", TrackID: "track-1", PluginID: "comp-1"}
	return server, interaction, ticket
}

// RED/GREEN (BRACKET-1, nominal post-load shape): a D1 compressor execution
// whose interaction carries the intent only over the post-load channel — the
// key shape the load-first flow hands to the execution stage — must open the
// settlement bracket. Before the fix this reproduces VER-1 R4 (bracket nil);
// after the fix the same test turns GREEN.
func TestSemanticCompressorSettlementBracketOpensForPostLoadIntentShape(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	requestContext := compressorPostLoadHandoffContext(loop, map[string]any{
		"schema_version": "semantic_processor_intent.v1", "family": processorintent.FamilyBroadbandCompressor,
		"status": "resolved", "processor_type": "compressor",
	}, false)
	server, interaction, ticket := compressorPostLoadBracketFixture(t, loop, requestContext)

	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if bracket == nil {
		t.Fatal("post-load intent shape produced no settlement bracket (VER-1 R4 bracket==nil reproduced)")
	}
	if bracket.BeforeRevision != 5 || bracket.RequestID == "" || bracket.ActionID == "" {
		t.Fatalf("bracket=%+v", bracket)
	}
	if !strings.HasPrefix(bracket.ActionID, "d1_semantic_") {
		t.Fatalf("bracket action id=%q does not follow the D1 journal identity", bracket.ActionID)
	}
}

// RED/GREEN (BRACKET-1, exact R4 measured shape): the request context carries
// the post-load intent key as a nil value (as persisted in every R4 response)
// and the family-bearing intent under the processor-selection route record.
// The owner predicate must still recognize this as the load-first D1
// compressor flow and open the bracket.
func TestSemanticCompressorSettlementBracketOpensForR4ExactPostLoadShape(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	requestContext := compressorPostLoadHandoffContext(loop, nil, true)
	server, interaction, ticket := compressorPostLoadBracketFixture(t, loop, requestContext)

	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if bracket == nil {
		t.Fatal("R4 exact post-load shape produced no settlement bracket (bracket==nil reproduced)")
	}
	if bracket.BeforeRevision != 5 {
		t.Fatalf("before revision=%d want the live snapshot revision 5", bracket.BeforeRevision)
	}
}

// PIN: an interaction carrying only the canonical pre-load anchor keeps
// opening the bracket exactly as before — the wrapper must not change the
// historical path.
func TestSemanticCompressorSettlementOldKeyShapeUnchanged(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	requestContext := map[string]any{
		"free_state_semantic_processor_intent": map[string]any{
			"schema_version": "semantic_processor_intent.v1", "family": processorintent.FamilyBroadbandCompressor,
			"status": "resolved", "processor_type": "compressor",
		},
	}
	server, interaction, ticket := compressorPostLoadBracketFixture(t, loop, requestContext)

	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if bracket == nil {
		t.Fatal("canonical free-state anchor stopped opening the bracket")
	}
	if bracket.BeforeRevision != 5 {
		t.Fatalf("before revision=%d want 5", bracket.BeforeRevision)
	}
}

// PIN: no intent channel (neither canonical anchor, nor post-load handoff with
// a family-bearing intent) still yields nil with zero snapshots consumed —
// ordinary/C2-style compressor executions keep the historical unbracketed
// shape.
func TestSemanticCompressorSettlementNoIntentChannelStillNil(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	requestContext := map[string]any{
		"conversation_id":           loop.ConversationID,
		"free_state_reasoning_loop": freeStateLoopMap(loop),
		"selected_track_id":         "track-1",
	}
	server, interaction, ticket := compressorPostLoadBracketFixture(t, loop, requestContext)

	bracket, err := server.beginSemanticCompressorSettlement(context.Background(), interaction, ticket)
	if err != nil {
		t.Fatalf("interaction without an intent channel must skip the bracket, not fail: %v", err)
	}
	if bracket != nil {
		t.Fatalf("interaction without an intent channel produced a bracket: %+v", bracket)
	}
	if override := server.semanticSettlementStateOverride.(*semanticSettlementStateForTest); override.index != 0 {
		t.Fatalf("non-owner interaction consumed snapshots: %d", override.index)
	}
}

// GREEN end-to-end (BRACKET-1): a D1 compressor execution in the exact R4
// post-load shape books the v2 receipt with the native shape keys — the
// evaluator's deterministic receipt assertions pass without touching the
// evaluator. Mirrors the RECEIPT-1 GREEN test but with the R4 key shape.
func TestSemanticCompressorExecutionSettlesReceiptWithR4PostLoadShapeForD1(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	fake := &semanticCompressorSettlementKernel{fakeCompressorKernel: *newFakeCompressorKernel()}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5), semanticSettlementSnapshot(6),
	}}
	server.storeFreeStateLoop(loop)

	ticket := semanticCompressorSettlementTicket(t, server, "semexec_r4_postload_settle")
	requestContext := compressorPostLoadHandoffContext(loop, nil, true)
	requestContext["c2_source_only_reversible"] = true
	interaction := PendingInteraction{
		ID: "interaction-compressor-r4-postload", Kind: "confirmation", Workflow: semanticCompressorExecutionWorkflow,
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		RequestContext: requestContext,
	}
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
	if receipt["transaction_id"] != "tx_semexec_r4_postload_settle" || receipt["idempotency_key"] != "semexec_r4_postload_settle" {
		t.Fatalf("transaction identity=%v/%v", receipt["transaction_id"], receipt["idempotency_key"])
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
}
