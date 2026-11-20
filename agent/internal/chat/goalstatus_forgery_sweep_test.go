package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	agentruntime "vit-daw-agent/internal/runtime"
)

// FIX-GOALSTATUS-FORGERY-SWEEP-1 coverage for the last three canned workflow
// fallbacks (section-automation-choreo / native mix tick / mix treatment) and
// the no-candidate mix tick receipt, mirroring the interaction_retirement_test
// (327) and plan_stale_status_test (330) forms:
//
//	state 1 — a timely decision routes to a live workflow continuation and
//	          never reaches the fallback receipts (covered by the existing
//	          mix-tick applied/confirmed suites, e.g. the
//	          mix_tick_applied_reobserved flow in server_test.go: the
//	          fallbacks are only reachable when the continuation is gone);
//	state 2 — the card was consumed but its workflow continuation is missing
//	          (candidate expired, or lost to a workspace switch/restart): the
//	          answer states "not executed + reason", carries an EMPTY
//	          goal_status (never the old fabricated completed), and the
//	          finalize projection must not refill it;
//	state 3 — a repeated respond for the consumed card gets the
//	          interaction_already_retired receipt from the retirement log, and
//	          a fresh card in the same conversation stays answerable.

func storeForgerySweepInteraction(t *testing.T, s *Server, id, conversationID, kind, workflow string) {
	t.Helper()
	request := AgentInteractionRequest{
		ID: id, Kind: kind, Type: kind,
		Source: "vit_agent", Workflow: workflow, Stage: "pending_confirmation",
		Title: "test confirmation", Body: "test body", Status: "waiting_for_user",
		ConversationID: conversationID, Payload: map[string]any{"track_id": "tr_sweep"}, Data: map[string]any{},
	}
	s.storePendingInteraction(request, request.Payload)
}

func respondInteractionWithDecision(t *testing.T, s *Server, interactionID, decision string) (int, ChatResponse) {
	t.Helper()
	body := `{"interaction_id":"` + interactionID + `","decision":"` + decision + `"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.handleInteractionRespond(rec, req)
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return rec.Code, resp
}

// assertForgerySweepReceipt pins the shared receipt contract: explicit
// "not executed + reason" wording, no canned catch-all text, empty goal
// status, structured workflow_data markers.
func assertForgerySweepReceipt(t *testing.T, stage string, resp ChatResponse) {
	t.Helper()
	if resp.StopReason != "interaction_continuation_missing" {
		t.Fatalf("%s: stop reason = %q, want interaction_continuation_missing (%+v)", stage, resp.StopReason, resp)
	}
	if resp.GoalStatus != "" {
		t.Fatalf("%s: receipt fabricated goal status %q (pre-fix failure shape)", stage, resp.GoalStatus)
	}
	if strings.Contains(resp.Reply, "已处理或已过期") {
		t.Fatalf("%s: receipt still carries the canned catch-all text: %q", stage, resp.Reply)
	}
	if !strings.Contains(resp.Reply, "未执行") || !strings.Contains(resp.Reply, "目标未因此完成") {
		t.Fatalf("%s: receipt not explicit about non-execution: %q", stage, resp.Reply)
	}
	if resp.WorkflowData["decision_executed"] != false || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("%s: receipt workflow_data must mark decision_executed=false and mutation_performed=false, got %+v", stage, resp.WorkflowData)
	}
	if resp.WorkflowData["status"] != "continuation_missing" {
		t.Fatalf("%s: receipt workflow_data status = %v", stage, resp.WorkflowData["status"])
	}
}

// F1: an answered section-automation-choreo card whose choreography candidate
// is gone (expired or lost to a workspace switch/restart) answers with the
// explicit receipt instead of the canned "已处理或已过期" + fabricated
// goal_status=completed.
func TestAutomationChoreoContinuationMissingAnswersExplicitReceipt(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeForgerySweepInteraction(t, server, "interaction_sweep_choreo", "conv_sweep_choreo", "automation_choreo_confirmation", "section_automation_choreo")

	code, resp := respondInteraction(t, server, "interaction_sweep_choreo")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	assertForgerySweepReceipt(t, "automation-choreo fallback", resp)
	if resp.Workflow != "section_automation_choreo" {
		t.Fatalf("receipt lost the card workflow: %+v", resp)
	}
}

// F2: an answered mix tick card whose pending candidate is gone answers with
// the explicit receipt. The decision text here is a non-apply word ("稍后再说"),
// so the respond face falls through to the fallback (the explicit-apply wording
// takes the F2b no-candidate receipt covered below).
func TestMixTickRespondContinuationMissingAnswersExplicitReceipt(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeForgerySweepInteraction(t, server, "interaction_sweep_tick", "conv_sweep_tick", "mix_tick_confirmation", "mix_tick")

	code, resp := respondInteractionWithDecision(t, server, "interaction_sweep_tick", "稍后再说")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	assertForgerySweepReceipt(t, "mix-tick respond fallback", resp)
}

// F3: an answered mix treatment card whose pending treatment is gone answers
// with the explicit receipt.
func TestMixTreatmentContinuationMissingAnswersExplicitReceipt(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeForgerySweepInteraction(t, server, "interaction_sweep_treat", "conv_sweep_treat", "mix_treatment_confirmation", "mix_treatment")

	code, resp := respondInteraction(t, server, "interaction_sweep_treat")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	assertForgerySweepReceipt(t, "mix-treatment fallback", resp)
}

// F2b: the mix tick confirmation face's own no-candidate receipt (reachable
// for an explicit apply wording with no pending candidate) must claim no goal
// status — nothing was executed.
func TestMixTickNoCandidateReceiptClaimsNoGoalStatus(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	resp, handled := server.handlePendingMixTickChat(context.Background(), "conv_sweep_nocand", ChatRequest{
		Message: "可以执行",
		Context: map[string]any{"conversation_id": "conv_sweep_nocand"},
	}, agentModeDefault)
	if !handled {
		t.Fatalf("explicit apply without a candidate must be answered explicitly, got handled=false resp=%+v", resp)
	}
	if resp.StopReason != "no_pending_mix_tick_candidate" {
		t.Fatalf("stop reason = %q", resp.StopReason)
	}
	if resp.GoalStatus != "" {
		t.Fatalf("no-candidate receipt fabricated goal status %q (pre-fix failure shape)", resp.GoalStatus)
	}
	if !strings.Contains(resp.Reply, "没有可执行的待确认混音动作") {
		t.Fatalf("receipt lost its explicit wording: %q", resp.Reply)
	}
}

// state 3: after the continuation-missing receipt, a repeated respond gets the
// already-retired receipt from the retirement log (both without fabricated
// goal status), and a fresh card in the same conversation stays answerable.
func TestRepeatedRespondAfterContinuationMissingGetsRetiredReceipt(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeForgerySweepInteraction(t, server, "interaction_sweep_repeat", "conv_sweep_repeat", "mix_treatment_confirmation", "mix_treatment")

	_, first := respondInteraction(t, server, "interaction_sweep_repeat")
	assertForgerySweepReceipt(t, "first answer", first)

	code, second := respondInteraction(t, server, "interaction_sweep_repeat")
	if code != http.StatusOK || second.StopReason != "interaction_already_retired" {
		t.Fatalf("repeated answer: code=%d resp=%+v", code, second)
	}
	if second.GoalStatus != "" {
		t.Fatalf("repeated answer fabricated goal status %q", second.GoalStatus)
	}
	if !strings.Contains(second.Reply, "consumed") {
		t.Fatalf("repeated answer receipt missing the retirement reason: %+v", second)
	}

	storeForgerySweepInteraction(t, server, "interaction_sweep_fresh", "conv_sweep_repeat", "mix_treatment_confirmation", "mix_treatment")
	_, fresh := respondInteraction(t, server, "interaction_sweep_fresh")
	assertForgerySweepReceipt(t, "fresh card", fresh)
}

// F4 (projection guard): finalizeInteractionChatResponse must not refill the
// empty goal_status of an unanswerable-decision receipt (and the control turn
// keeps the designed refill, proving the guard — not a global change — did it).
func TestFinalizeInteractionReceiptDoesNotRefillGoalStatus(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	receipt := server.interactionContinuationMissingReceipt(PendingInteraction{
		ID: "interaction_sweep_fin", ConversationID: "conv_sweep_fin", Workflow: "mix_tick",
	}, "混音单步确认")
	if receipt.GoalStatus != "" || receipt.StopReason != "interaction_continuation_missing" {
		t.Fatalf("receipt shape broken: %+v", receipt)
	}
	server.finalizeInteractionChatResponse(context.Background(), &receipt, PendingInteraction{ConversationID: "conv_sweep_fin"}, "approve")
	if receipt.GoalStatus != "" {
		t.Fatalf("finalize refilled the receipt goal status to %q (chat-projection forgery face)", receipt.GoalStatus)
	}

	normal := ChatResponse{ConversationID: "conv_sweep_fin", Reply: "已执行。", Workflow: "mix_tick"}
	server.finalizeInteractionChatResponse(context.Background(), &normal, PendingInteraction{ConversationID: "conv_sweep_fin"}, "可以执行")
	if normal.GoalStatus != string(agentruntime.StatusCompleted) {
		t.Fatalf("control turn lost the designed goal_status refill: %+v", normal)
	}
}

// The exemption predicate covers every unanswerable-decision receipt from the
// 327/330/334 family (plan face, interaction catch-all, interaction
// continuation fallbacks, no-candidate mix tick) and nothing else.
func TestReceiptCarriesNoGoalStatusCoversAllUnanswerableReceipts(t *testing.T) {
	for _, reason := range []string{
		"plan_already_retired", "plan_not_found",
		"interaction_already_retired", "interaction_not_found",
		"interaction_continuation_missing", "no_pending_mix_tick_candidate",
	} {
		if !receiptCarriesNoGoalStatus(reason) {
			t.Fatalf("receipt stop reason %q must be exempt from the goal_status refill", reason)
		}
	}
	for _, reason := range []string{"", "mix_tick_applied_reobserved", "mix_treatment_resolved_offer", "free_state_com_evidence_refresh_required"} {
		if receiptCarriesNoGoalStatus(reason) {
			t.Fatalf("normal turn stop reason %q must keep the designed goal_status projection", reason)
		}
	}
}
