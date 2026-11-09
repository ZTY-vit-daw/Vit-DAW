package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// FIX-PLAN-STALE-STATUS-1 three-state coverage for the pending-plan decision
// face (resolvePendingPlanDecision / handleConfirm), mirroring the
// FIX-GOV-INTERACTION-TTL-1 interaction_retirement_test form:
//
//	state 1 — a timely approve executes the plan and completes the goal; a
//	          repeated approve gets the explicit already-retired receipt
//	          (idempotent回告, no fabricated goal status);
//	state 2 — a stale/late answer (plan already consumed, explicitly expired,
//	          or lost to a workspace-switch snapshot replace) answers with
//	          "not executed + reason" (retired vs lost), never the old
//	          catch-all that fabricated goal_status=completed;
//	state 3 — a fresh plan parked after a retirement is answerable and
//	          unaffected by the retired one; an unknown plan id gets the
//	          explicit not-found receipt instead of a bare error.

func newPlanStaleStatusTestServer(t *testing.T) *Server {
	t.Helper()
	return New(nil, shadow.New(nil), nil)
}

func storePlanStaleTestPlan(t *testing.T, s *Server, planID string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[planID] = PendingPlan{
		ID:        planID,
		CreatedAt: time.Now(),
		Context:   map[string]any{"goal_id": "goal_plan_stale", "run_id": "run_plan_stale"},
		Workflow:  "goal_ui_smoke",
	}
}

func confirmPlan(t *testing.T, s *Server, planID string) (int, map[string]any) {
	t.Helper()
	return s.resolvePendingPlanDecision(context.Background(), planID, "approve")
}

// state 1: the timely approve executes (goal_ui_smoke completes), and the
// repeated approve gets an explicit already-retired receipt instead of the
// pre-fix fabricated goal_status=completed catch-all.
func TestPlanDecisionTimelyApproveExecutesAndRepeatIsExplicit(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	storePlanStaleTestPlan(t, server, "plan_stale_live")

	status, response := confirmPlan(t, server, "plan_stale_live")
	if status != http.StatusOK {
		t.Fatalf("status = %d response=%+v", status, response)
	}
	if got := response["goal_status"]; got != string(agentruntime.StatusCompleted) {
		t.Fatalf("timely approve should report the completed goal, got goal_status=%v response=%+v", got, response)
	}
	if message := fmt.Sprint(response["message"]); strings.Contains(message, "未执行") {
		t.Fatalf("timely approve hit a catch-all receipt: %+v", response)
	}
	server.mu.Lock()
	_, stillPending := server.pending["plan_stale_live"]
	server.mu.Unlock()
	if stillPending {
		t.Fatal("plan should be consumed by the timely approve")
	}

	status, repeat := confirmPlan(t, server, "plan_stale_live")
	if status != http.StatusOK || repeat["stop_reason"] != "plan_already_retired" {
		t.Fatalf("repeated approve: status=%d response=%+v", status, repeat)
	}
	if got := repeat["goal_status"]; got == string(agentruntime.StatusCompleted) || fmt.Sprint(got) != "" {
		t.Fatalf("repeated approve fabricated goal status %v (330 failure shape)", got)
	}
	if message := fmt.Sprint(repeat["message"]); !strings.Contains(message, "未执行") || !strings.Contains(message, "consumed") {
		t.Fatalf("repeated approve receipt not explicit: %+v", repeat)
	}
}

// state 2a (explicit expiry, guard shape): a plan expired by
// expirePendingPlan answers with the retirement reason, idempotently.
func TestPlanDecisionExpiredPlanAnswersWithRetirementReason(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	storePlanStaleTestPlan(t, server, "plan_stale_guard")
	server.expirePendingPlan("plan_stale_guard")

	status, response := confirmPlan(t, server, "plan_stale_guard")
	if status != http.StatusOK || response["stop_reason"] != "plan_already_retired" {
		t.Fatalf("expired plan confirm: status=%d response=%+v", status, response)
	}
	if got := response["goal_status"]; fmt.Sprint(got) != "" {
		t.Fatalf("expired-plan receipt fabricated goal status %v", got)
	}
	if message := fmt.Sprint(response["message"]); !strings.Contains(message, "expired") || !strings.Contains(message, "未执行") {
		t.Fatalf("expired-plan receipt missing reason: %+v", response)
	}
}

// state 2b (alias remnant, the legacy stale window): a plan whose canonical
// entry is gone but alias remnants survive was already handled elsewhere —
// the confirm gets the already-retired receipt, not the ambiguous catch-all.
func TestPlanDecisionStaleAliasRemnantAnswersAlreadyRetired(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	server.mu.Lock()
	server.pending["conv_stale_alias"] = PendingPlan{
		ID:        "plan_stale_alias",
		CreatedAt: time.Now(),
		Context:   map[string]any{"goal_id": "goal_plan_stale", "run_id": "run_plan_stale"},
		Workflow:  "goal_ui_smoke",
	}
	server.mu.Unlock()

	status, response := confirmPlan(t, server, "plan_stale_alias")
	if status != http.StatusOK || response["stop_reason"] != "plan_already_retired" {
		t.Fatalf("stale alias confirm: status=%d response=%+v", status, response)
	}
	if got := response["goal_status"]; fmt.Sprint(got) != "" {
		t.Fatalf("stale-alias receipt fabricated goal status %v", got)
	}
	if message := fmt.Sprint(response["message"]); !strings.Contains(message, "未执行") {
		t.Fatalf("stale-alias receipt not explicit: %+v", response)
	}
}

// state 2c (workspace-switch snapshot replace, run9 r2 shape on the plan
// face): the in-memory plan registry is replaced wholesale and the late
// approve finds nothing — it must answer "not executed + not found", never
// claim the goal completed.
func TestPlanDecisionLostToWorkspaceReplaceAnswersNotFoundExplicit(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	storePlanStaleTestPlan(t, server, "plan_stale_lost")
	server.mu.Lock()
	server.pending = map[string]PendingPlan{}
	server.mu.Unlock()

	status, response := confirmPlan(t, server, "plan_stale_lost")
	if status != http.StatusOK || response["stop_reason"] != "plan_not_found" {
		t.Fatalf("lost plan confirm: status=%d response=%+v", status, response)
	}
	if got := response["goal_status"]; got == string(agentruntime.StatusCompleted) || fmt.Sprint(got) != "" {
		t.Fatalf("lost-plan receipt fabricated goal status %v (330 failure shape)", got)
	}
	message := fmt.Sprint(response["message"])
	if !strings.Contains(message, "未找到待确认计划") || !strings.Contains(message, "未执行") || !strings.Contains(message, "目标未因此完成") {
		t.Fatalf("lost-plan receipt not explicit: %+v", response)
	}
}

// state 3a: an unknown plan id gets the explicit not-found receipt.
func TestPlanDecisionUnknownPlanAnswersNotFoundExplicit(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)

	status, response := confirmPlan(t, server, "plan_never_existed")
	if status != http.StatusOK || response["stop_reason"] != "plan_not_found" {
		t.Fatalf("unknown plan confirm: status=%d response=%+v", status, response)
	}
	if got := response["goal_status"]; got == string(agentruntime.StatusCompleted) || fmt.Sprint(got) != "" {
		t.Fatalf("unknown-plan receipt fabricated goal status %v", got)
	}
	message := fmt.Sprint(response["message"])
	if !strings.Contains(message, "未找到待确认计划") || !strings.Contains(message, "未执行") {
		t.Fatalf("unknown-plan receipt not explicit: %+v", response)
	}
}

// state 3b: a fresh plan parked after a retirement is answerable and
// unaffected by the retired plan's receipt.
func TestNewPlanAfterRetirementIsAnswerable(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	storePlanStaleTestPlan(t, server, "plan_stale_first")
	server.expirePendingPlan("plan_stale_first")

	if _, first := confirmPlan(t, server, "plan_stale_first"); first["stop_reason"] != "plan_already_retired" {
		t.Fatalf("first plan receipt = %+v", first)
	}

	storePlanStaleTestPlan(t, server, "plan_stale_second")
	status, second := confirmPlan(t, server, "plan_stale_second")
	if status != http.StatusOK {
		t.Fatalf("fresh plan confirm status = %d response=%+v", status, second)
	}
	if second["stop_reason"] == "plan_already_retired" || second["stop_reason"] == "plan_not_found" {
		t.Fatalf("fresh plan polluted by the retired one: %+v", second)
	}
	if got := second["goal_status"]; got != string(agentruntime.StatusCompleted) {
		t.Fatalf("fresh plan should execute and complete, got goal_status=%v response=%+v", got, second)
	}
}

// The /agent/confirm HTTP face answers an unknown plan with the explicit
// receipt (pre-fix: bare 404 plan not found; and stale: fabricated
// goal_status=completed).
func TestHandleConfirmUnknownPlanReturnsExplicitReceipt(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)
	body := bytes.NewBufferString(`{"plan_id":"plan_missing_confirm","decision":"approve"}`)
	req := httptest.NewRequest(http.MethodPost, "/agent/confirm", body)
	rec := httptest.NewRecorder()
	server.handleConfirm(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["stop_reason"] != "plan_not_found" {
		t.Fatalf("response = %+v", response)
	}
	if got := fmt.Sprint(response["goal_status"]); got == string(agentruntime.StatusCompleted) || got != "" {
		t.Fatalf("confirm receipt fabricated goal status %q (330 failure shape)", got)
	}
	if message := fmt.Sprint(response["message"]); !strings.Contains(message, "未执行") {
		t.Fatalf("confirm receipt not explicit: %s", message)
	}
}

// The chat projection path (chatResponseFromPendingPlanDecision) must not
// refill an empty goal_status into completed for the unanswerable-plan
// receipts — the decision was not executed, no path may claim completion.
func TestChatResponseFromPlanDecisionReceiptDoesNotFabricateGoalStatus(t *testing.T) {
	server := newPlanStaleStatusTestServer(t)

	_, notFound := confirmPlan(t, server, "plan_unknown_chat")
	resp := chatResponseFromPendingPlanDecision("conv_plan_stale", notFound, agentModeDefault)
	if resp.GoalStatus != "" {
		t.Fatalf("not-found receipt must carry no goal status, got %q (fabricated by the conversion fallback)", resp.GoalStatus)
	}
	if resp.StopReason != "plan_not_found" {
		t.Fatalf("converted receipt stop reason = %q", resp.StopReason)
	}

	storePlanStaleTestPlan(t, server, "plan_retired_chat")
	server.expirePendingPlan("plan_retired_chat")
	_, retired := confirmPlan(t, server, "plan_retired_chat")
	resp = chatResponseFromPendingPlanDecision("conv_plan_stale", retired, agentModeDefault)
	if resp.GoalStatus != "" {
		t.Fatalf("already-retired receipt must carry no goal status, got %q (fabricated by the conversion fallback)", resp.GoalStatus)
	}
	if resp.StopReason != "plan_already_retired" {
		t.Fatalf("converted receipt stop reason = %q", resp.StopReason)
	}
}
