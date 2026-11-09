package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/shadow"
)

// FIX-GOV-INTERACTION-TTL-1 three-state coverage for the governance
// confirmation (improvement_proposal) interaction face:
//
//	state 1 — a timely approve reaches a live card and routes (never the
//	          catch-all); a repeated approve gets the explicit
//	          already-retired receipt (idempotent回告, no fabricated goal status);
//	state 2 — the run9-r2 shape: the chat tail crosses a workspace identity
//	          boundary and restore replaces the answerable map with the target
//	          workspace's older disk snapshot. The in-flight card survives
//	          (保活) and the late approve routes. A card older than a newer
//	          snapshot that lacks it (another owner's decision) stays retired
//	          and the approve gets the explicit not-found receipt;
//	state 3 — after a card is retired, a fresh proposal card in the same
//	          conversation is answerable and unaffected by the retired one.

func newInteractionRetirementTestServer(t *testing.T) *Server {
	t.Helper()
	return New(nil, shadow.New(nil), nil)
}

func storeRetirementTestInteraction(t *testing.T, s *Server, id, conversationID string, createdAt time.Time) {
	t.Helper()
	request := AgentInteractionRequest{
		ID: id, Kind: "improvement_proposal_confirmation", Type: "improvement_proposal_confirmation",
		Source: "vit_agent", Workflow: improvementProposalWorkflow, Stage: "pending_confirmation",
		Title: "改善性提案待确认", Body: "test proposal", Status: "waiting_for_user",
		ConversationID: conversationID, Payload: map[string]any{}, Data: map[string]any{},
	}
	s.storePendingInteraction(request, request.Payload)
	s.mu.Lock()
	if row, ok := s.interactions[id]; ok {
		row.CreatedAt = createdAt
		s.interactions[id] = row
	}
	s.mu.Unlock()
}

func respondInteraction(t *testing.T, s *Server, interactionID string) (int, ChatResponse) {
	t.Helper()
	body := `{"interaction_id":"` + interactionID + `","decision":"approve"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/interaction/respond", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.handleInteractionRespond(rec, req)
	var resp ChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return rec.Code, resp
}

// state 1: the timely approve routes through the improvement-proposal
// continuation, and the repeated approve gets an explicit already-retired
// receipt instead of the pre-fix ambiguous catch-all.
func TestInteractionRespondTimelyApproveRoutesAndRepeatIsExplicit(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeRetirementTestInteraction(t, server, "interaction_ttl_live", "conv_ttl", time.Now())

	code, resp := respondInteraction(t, server, "interaction_ttl_live")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	if strings.Contains(resp.Reply, "已处理或已过期") || strings.Contains(resp.Reply, "未找到待处理交互") {
		t.Fatalf("timely approve hit a catch-all receipt: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "已确认") {
		t.Fatalf("expected the improvement-proposal continuation to speak, got %+v", resp)
	}

	code, repeat := respondInteraction(t, server, "interaction_ttl_live")
	if code != http.StatusOK || repeat.StopReason != "interaction_already_retired" {
		t.Fatalf("repeated approve: code=%d resp=%+v", code, repeat)
	}
	if repeat.GoalStatus != "" {
		t.Fatalf("repeated approve fabricated goal status %q (run9 r2 failure shape)", repeat.GoalStatus)
	}
	if !strings.Contains(repeat.Reply, "未执行") || !strings.Contains(repeat.Reply, "consumed") {
		t.Fatalf("repeated approve receipt not explicit: %+v", repeat)
	}
}

// state 2 (run9 r2 replay, unit level): the workspace-switch snapshot replace
// must not swallow the in-flight card; the late approve still routes.
func TestInteractionSurvivesWorkspaceSwitchSnapshotReplace(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeRetirementTestInteraction(t, server, "interaction_ttl_r2", "conv_ttl_r2", time.Now())

	// The r2 shape: the target workspace's persisted snapshot predates the
	// card (its last write happened under the previous workspace identity)
	// and does not contain the card.
	server.mu.Lock()
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{SavedAt: time.Now().Add(-time.Hour)})
	server.mu.Unlock()

	if _, ok := server.peekPendingInteraction("interaction_ttl_r2"); !ok {
		t.Fatal("in-flight confirmation card lost to the workspace-switch snapshot replace (run9 r2 regression)")
	}
	code, resp := respondInteraction(t, server, "interaction_ttl_r2")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	if strings.Contains(resp.Reply, "已处理或已过期") || strings.Contains(resp.Reply, "未找到待处理交互") {
		t.Fatalf("late approve swallowed by catch-all: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "已确认") {
		t.Fatalf("expected the improvement-proposal continuation to speak, got %+v", resp)
	}
}

// state 2 (cross-owner decision): a snapshot newer than the card that still
// lacks it is another owner's resolution — the card stays retired and the
// approve gets the explicit not-found receipt (fail-closed, no fabricated
// goal_status=completed).
func TestInteractionOlderThanNewerSnapshotStaysRetiredWithExplicitReceipt(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeRetirementTestInteraction(t, server, "interaction_ttl_old", "conv_ttl_old", time.Now().Add(-2*time.Hour))

	server.mu.Lock()
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{SavedAt: time.Now().Add(-time.Hour)})
	server.mu.Unlock()

	if _, ok := server.peekPendingInteraction("interaction_ttl_old"); ok {
		t.Fatal("card predating a newer owner-decided snapshot must not be resurrected")
	}
	code, resp := respondInteraction(t, server, "interaction_ttl_old")
	if code != http.StatusOK || resp.StopReason != "interaction_not_found" {
		t.Fatalf("code=%d resp=%+v", code, resp)
	}
	if resp.GoalStatus != "" {
		t.Fatalf("not-found receipt fabricated goal status %q", resp.GoalStatus)
	}
	if !strings.Contains(resp.Reply, "未执行") || !strings.Contains(resp.Reply, "未找到待处理交互") {
		t.Fatalf("not-found receipt not explicit: %+v", resp)
	}
}

// guard-expired cards answer with the retirement reason (idempotent回告 for
// the explicit-expiry paths, not the ambiguous catch-all).
func TestInteractionGuardExpiryAnswersWithRetirementReason(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeRetirementTestInteraction(t, server, "interaction_ttl_guard", "conv_ttl_guard", time.Now())

	if removed := server.expirePendingInteractionsForConversation("conv_ttl_guard", improvementProposalWorkflow); removed != 1 {
		t.Fatalf("expire removed %d interactions", removed)
	}
	code, resp := respondInteraction(t, server, "interaction_ttl_guard")
	if code != http.StatusOK || resp.StopReason != "interaction_already_retired" {
		t.Fatalf("code=%d resp=%+v", code, resp)
	}
	if !strings.Contains(resp.Reply, "expired_for_conversation") {
		t.Fatalf("receipt missing retirement reason: %+v", resp)
	}
	if resp.GoalStatus != "" {
		t.Fatalf("guard-expiry receipt fabricated goal status %q", resp.GoalStatus)
	}
}

// state 3: a fresh proposal card parked after a retirement is answerable and
// unaffected by the retired card or its receipt.
func TestNewProposalAfterRetirementIsAnswerable(t *testing.T) {
	server := newInteractionRetirementTestServer(t)
	storeRetirementTestInteraction(t, server, "interaction_ttl_first", "conv_ttl", time.Now())
	if removed := server.expirePendingInteractionsForConversation("conv_ttl", improvementProposalWorkflow); removed != 1 {
		t.Fatalf("expire removed %d interactions", removed)
	}
	if _, first := respondInteraction(t, server, "interaction_ttl_first"); first.StopReason != "interaction_already_retired" {
		t.Fatalf("first card receipt = %+v", first)
	}

	storeRetirementTestInteraction(t, server, "interaction_ttl_second", "conv_ttl", time.Now())
	if _, ok := server.pendingImprovementProposalForConversation("conv_ttl"); !ok {
		t.Fatal("fresh proposal card not visible to the conversation")
	}
	code, resp := respondInteraction(t, server, "interaction_ttl_second")
	if code != http.StatusOK {
		t.Fatalf("status = %d body=%+v", code, resp)
	}
	if strings.Contains(resp.Reply, "已处理或已过期") || strings.Contains(resp.Reply, "未找到待处理交互") {
		t.Fatalf("fresh card polluted by the retired one: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "已确认") {
		t.Fatalf("expected the improvement-proposal continuation to speak, got %+v", resp)
	}
}
