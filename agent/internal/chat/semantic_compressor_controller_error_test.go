package chat

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

// RED (CTRLERR-1): the apply-error exit of executeSemanticCompressorTicket
// must persist a structured controller_error receipt inside
// WorkflowData["execution_receipt"], so the raw controller error survives the
// free-state loop clone (freeStateAcousticActionOutcome books only
// WorkflowData["execution_receipt"]) instead of living only in
// execution.message / ChatResponse.Error, which the loop does not persist.
// This D1-gated (bracketed) axis pins request_id_present=true and the
// kernel-real before revision captured by the open bracket.
func TestSemanticCompressorApplyErrorReceiptPersistsControllerErrorForD1(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	loop.D1State = map[string]any{"before_render": semanticSettlementReadyBeforeRender(t, "5")}
	// The net set_params_batch rides the ID-carrying transport (as in the real
	// stack once the bracket pins request_id) and fails with a kernel-level
	// VSP error payload on call 1; the controller's full-preimage rollback
	// (anonymous call 2) succeeds, so the apply error is the raw failure text
	// plus the truthful rollback note.
	fake := &semanticCompressorSettlementKernel{fakeCompressorKernel: *newFakeCompressorKernel()}
	fake.failBatchAt = map[int]bool{1: true}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	server.semanticSettlementStateOverride = &semanticSettlementStateForTest{states: []*kernel.VSPStateResult{
		semanticSettlementSnapshot(5), semanticSettlementSnapshot(5),
	}}
	server.storeFreeStateLoop(loop)

	ticket := semanticCompressorSettlementTicket(t, server, "semexec_ctrl_err")
	interaction := semanticCompressorSettlementInteraction(loop, ticket)
	response := server.executeSemanticCompressorTicket(context.Background(), interaction, ticket)
	if response.GoalStatus != "failed" || response.StopReason != "compressor_atomic_execution_failed" {
		t.Fatalf("apply failure boundary changed: status=%s stop=%s data=%v", response.GoalStatus, response.StopReason, response.WorkflowData)
	}
	if !strings.Contains(response.Error, "injected compressor failure") {
		t.Fatalf("original controller error lost: %q", response.Error)
	}
	if len(fake.requestIDs) != 1 || fake.requestIDs[0] != ticket.TicketID {
		t.Fatalf("accounted net write did not pin the bracket request id: %v want %s", fake.requestIDs, ticket.TicketID)
	}
	if response.WorkflowData["mutation_performed"] != false {
		t.Fatalf("mutation_performed changed on the apply-error path: %v", response.WorkflowData["mutation_performed"])
	}
	if message := firstStringFromMap(mapValue(response.WorkflowData["execution"]), "message"); message != response.Error {
		t.Fatalf("execution.message no longer transparent: %q vs %q", message, response.Error)
	}

	receipt := firstMapFromAny(response.WorkflowData["execution_receipt"])
	if len(receipt) == 0 {
		t.Fatalf("apply error booked no execution_receipt (controller_error not persisted): data=%v", response.WorkflowData)
	}
	if receipt["schema_version"] != semanticCompressorReceiptSchema || receipt["status"] != "failed" ||
		receipt["failure_code"] != "compressor_atomic_execution_failed" {
		t.Fatalf("failure receipt shape=%+v", receipt)
	}
	if receipt["ticket_id"] != ticket.TicketID || receipt["track_id"] != ticket.TrackID || receipt["plugin_id"] != ticket.PluginID {
		t.Fatalf("failure receipt ids=%+v want ticket/track/plugin of the executed ticket", receipt)
	}
	if receipt["before_revision"] != "5" {
		t.Fatalf("before_revision=%v want the open bracket's kernel-real revision 5", receipt["before_revision"])
	}
	controllerError := mapValue(receipt["controller_error"])
	if firstStringFromMap(controllerError, "message") != response.Error {
		t.Fatalf("controller_error.message=%q is not the raw err.Error()=%q", controllerError["message"], response.Error)
	}
	if controllerError["request_id_present"] != true {
		t.Fatalf("request_id_present=%v want true (bracket open, request_id pinned)", controllerError["request_id_present"])
	}
	if actionID := firstNonEmptyText(controllerError, "bracket_action_id"); actionID == "" || !strings.HasPrefix(actionID, "d1_semantic_") {
		t.Fatalf("bracket_action_id=%q want the opened bracket action id", actionID)
	}
	if controllerError["controller_result_present"] != false || controllerError["restore_ref_present"] != false {
		t.Fatalf("controller result presence flags fabricated: %+v", controllerError)
	}

	// The loop-accounting axis: the failed action's receipt must survive the
	// clone in freeStateAcousticActionOutcome and satisfy Intervention.Validate
	// (no more "intervention receipt is required").
	processorType, actionStatus, booked, attempted := freeStateAcousticActionOutcome(interaction, response)
	if !attempted || processorType != "compressor" || actionStatus != "failed" {
		t.Fatalf("outcome=%s/%s attempted=%v", processorType, actionStatus, attempted)
	}
	if len(booked) == 0 || firstStringFromMap(mapValue(booked["controller_error"]), "message") != response.Error {
		t.Fatalf("receipt did not survive the loop clone: %+v", booked)
	}
	server.recordFreeStateExperimentAction(&loop, processorType, actionStatus, booked)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Interventions) != 1 || len(round.Interventions[0].Receipt) == 0 {
		t.Fatalf("failed action was not booked with its receipt: %+v", round.Interventions)
	}
	if err := round.Interventions[0].Validate(); err != nil {
		t.Fatalf("booked failed intervention still invalid: %v receipt=%+v", err, round.Interventions[0].Receipt)
	}
}

// The unbracketed axis (no free-state D1 owner in the request context, so
// bracket==nil and executionRequest carries no request_id) must land the same
// structured controller_error receipt with request_id_present=false and an
// explicit "absent" before_revision instead of guessing one.
func TestSemanticCompressorApplyErrorReceiptWithoutBracketMarksRequestIDAbsent(t *testing.T) {
	loop := d1CompressionLoopForTest(t, "5")
	fake := newFakeCompressorKernel()
	fake.failBatchAt = map[int]bool{1: true}
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	server.storeFreeStateLoop(loop)

	ticket := semanticCompressorSettlementTicket(t, server, "semexec_ctrl_err_unbracketed")
	interaction := semanticCompressorSettlementInteraction(loop, ticket)
	delete(interaction.RequestContext, "free_state_semantic_processor_intent")
	response := server.executeSemanticCompressorTicket(context.Background(), interaction, ticket)
	if response.GoalStatus != "failed" || response.StopReason != "compressor_atomic_execution_failed" {
		t.Fatalf("apply failure boundary changed: status=%s stop=%s data=%v", response.GoalStatus, response.StopReason, response.WorkflowData)
	}
	if !strings.Contains(response.Error, "injected compressor failure") {
		t.Fatalf("original controller error lost: %q", response.Error)
	}
	receipt := firstMapFromAny(response.WorkflowData["execution_receipt"])
	if len(receipt) == 0 {
		t.Fatalf("apply error booked no execution_receipt (controller_error not persisted): data=%v", response.WorkflowData)
	}
	if receipt["before_revision"] != "absent" {
		t.Fatalf("before_revision=%v want explicit absent (no bracket)", receipt["before_revision"])
	}
	controllerError := mapValue(receipt["controller_error"])
	if firstStringFromMap(controllerError, "message") != response.Error {
		t.Fatalf("controller_error.message=%q is not the raw err.Error()=%q", controllerError["message"], response.Error)
	}
	if controllerError["request_id_present"] != false {
		t.Fatalf("request_id_present=%v want false (bracket nil, no request_id)", controllerError["request_id_present"])
	}
	if actionID := firstNonEmptyText(controllerError, "bracket_action_id"); actionID != "" {
		t.Fatalf("bracket_action_id=%q fabricated for an unbracketed execution", actionID)
	}
	if controllerError["controller_result_present"] != false || controllerError["restore_ref_present"] != false {
		t.Fatalf("controller result presence flags fabricated: %+v", controllerError)
	}

	processorType, actionStatus, booked, attempted := freeStateAcousticActionOutcome(interaction, response)
	if !attempted || processorType != "compressor" || actionStatus != "failed" || len(booked) == 0 {
		t.Fatalf("outcome=%s/%s attempted=%v booked=%+v", processorType, actionStatus, attempted, booked)
	}
	server.recordFreeStateExperimentAction(&loop, processorType, actionStatus, booked)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if len(round.Interventions) != 1 || len(round.Interventions[0].Receipt) == 0 {
		t.Fatalf("failed action was not booked with its receipt: %+v", round.Interventions)
	}
	if err := round.Interventions[0].Validate(); err != nil {
		t.Fatalf("booked failed intervention still invalid: %v receipt=%+v", err, round.Interventions[0].Receipt)
	}
}
