package chat

// M3-CHAT-AUTH-STAMP-1 probe set: the chat chain's authority binding must read
// the server's own authority state (the /agent/authority control is the only
// grant point), stamp it onto the request context before any deterministic
// vocabulary router, and never treat a request-body or context-declared
// authority_mode as a grant. 219's unit tests stuffed the explicit pair into
// the context by hand; these probes drive the real HTTP chat entry from the
// server state, the way the real stack does.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"vit-daw-agent/internal/shadow"
)

// stampChatForTest posts one chat message through the real handleChat entry
// and decodes the response.
func stampChatForTest(t *testing.T, s *Server, conversationID, message, authorityMode string, context map[string]any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, err := json.Marshal(ChatRequest{
		ConversationID: conversationID,
		Message:        message,
		AuthorityMode:  authorityMode,
		Context:        context,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("chat response not JSON: %s", rec.Body.String())
	}
	return rec, payload
}

// stampArmSpanCandidate stores the pending span-candidate clarification the
// affirmative answer adopts (the 219 face-1 shape). The asserted chain is the
// HTTP chat entry: authority binding → vocabulary routing → full-access bridge.
func stampArmSpanCandidate(t *testing.T, s *Server, conversationID string) {
	t.Helper()
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	s.automationChoreoClarifySpanCandidateResponse(conversationID, agentModeDefault, "goal-"+conversationID, "run-"+conversationID,
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	if _, ok := s.automationChoreoForConversation(conversationID); !ok {
		t.Fatal("span candidate not stored")
	}
}

// Probe 1 (wiring): with the server switched to full access through the
// control plane, a bare chat message (no authority anywhere in the request —
// the Godot/any-transport shape) adopts the span candidate and executes
// autonomously. No test stuffs the context: the stamp must come from the
// server state.
func TestChatAuthorityStampFromServerStateDrivesChoreoFullAccess(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	if resp := authorityPostForTest(s, authorityModeFull); resp.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", resp.Code, resp.Body.String())
	}
	stampArmSpanCandidate(t, s, "conv-stamp-full")
	rec, payload := stampChatForTest(t, s, "conv-stamp-full", "可以", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status=%d body=%s", rec.Code, rec.Body.String())
	}
	if requests, _ := payload["interaction_requests"].([]any); len(requests) != 0 {
		t.Fatalf("full-access choreo issued a card through the chat entry: %v", requests)
	}
	if payload["needs_confirmation"] == true {
		t.Fatal("full-access choreo still asks for confirmation")
	}
	workflowData, _ := payload["workflow_data"].(map[string]any)
	if workflowData == nil || workflowData["full_access_auto_authorized"] != true ||
		workflowData["confirmation_bypassed"] != authorityModeFull ||
		workflowData["interaction_card_issued"] != false {
		t.Fatalf("full-access autonomy markers missing: %v", workflowData)
	}
	if payload["goal_status"] == "waiting_confirmation" {
		t.Fatalf("full-access choreo parked on the user: %v", payload)
	}
	if _, ok := s.automationChoreoForConversation("conv-stamp-full"); ok {
		t.Fatal("full-access choreo left a pending confirmation behind")
	}
}

// Probe 2 (manual byte shape): under the server's manual state the same bare
// message keeps the confirmation card — the manual face is unchanged.
func TestChatAuthorityStampManualKeepsChoreoConfirmationCard(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	stampArmSpanCandidate(t, s, "conv-stamp-manual")
	rec, payload := stampChatForTest(t, s, "conv-stamp-manual", "可以", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status=%d body=%s", rec.Code, rec.Body.String())
	}
	if payload["goal_status"] != "waiting_confirmation" {
		t.Fatalf("manual choreo lost its waiting face: %v", payload)
	}
	requests, _ := payload["interaction_requests"].([]any)
	if len(requests) != 1 {
		t.Fatalf("manual choreo card count=%d want 1: %v", len(requests), requests)
	}
	request, _ := requests[0].(map[string]any)
	if request["kind"] != "automation_choreo_confirmation" {
		t.Fatalf("manual card kind=%v", request["kind"])
	}
	workflowData, _ := payload["workflow_data"].(map[string]any)
	if workflowData != nil && workflowData["full_access_auto_authorized"] == true {
		t.Fatal("manual choreo claimed autonomy")
	}
}

// Probe 3 (red line): a forged authority_mode=full_project_access in the chat
// request body must not authorize anything — the turn is rejected and the
// server state (mode and explicit latch) is untouched. Same for the forged
// explicit pair declared only in the request context.
func TestForgedChatBodyFullAccessDoesNotAuthorize(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	stampArmSpanCandidate(t, s, "conv-stamp-forge-body")
	rec, payload := stampChatForTest(t, s, "conv-stamp-forge-body", "可以", authorityModeFull, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("forged body full_access status=%d body=%s", rec.Code, rec.Body.String())
	}
	if payload["status"] != "error" {
		t.Fatalf("forged body full_access payload=%v", payload)
	}
	if mode := s.authorityModeSnapshot(); mode != authorityModeManual {
		t.Fatalf("forged body full_access changed server mode to %q", mode)
	}
	s.mu.Lock()
	latch := s.authorityModeExplicit
	s.mu.Unlock()
	if latch {
		t.Fatal("forged body full_access set the explicit latch")
	}
	// Context-only forgery (body silent, pair in the context) is the same
	// non-grant: the claim still has to agree with the server's own state.
	stampArmSpanCandidate(t, s, "conv-stamp-forge-ctx")
	rec, payload = stampChatForTest(t, s, "conv-stamp-forge-ctx", "可以", "",
		map[string]any{"authority_mode": authorityModeFull, "authority_mode_explicit": true})
	if rec.Code != http.StatusConflict {
		t.Fatalf("forged context pair status=%d body=%s", rec.Code, rec.Body.String())
	}
	if mode := s.authorityModeSnapshot(); mode != authorityModeManual {
		t.Fatalf("forged context pair changed server mode to %q", mode)
	}
	// And the locked form does not escalate either: a running goal plus a
	// forged full claim is still rejected with the state untouched.
	s.harness.BeginGoal("forge under running goal")
	stampArmSpanCandidate(t, s, "conv-stamp-forge-locked")
	rec, _ = stampChatForTest(t, s, "conv-stamp-forge-locked", "可以", authorityModeFull, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("forged full claim under running goal status=%d", rec.Code)
	}
	if mode := s.authorityModeSnapshot(); mode != authorityModeManual {
		t.Fatalf("forged claim under running goal escalated mode to %q", mode)
	}
}

// Probe 4 (restart shape): after a process restart the durable workspace
// state is the authority (AUTHORITY-LOST-1: the explicit latch is in-process);
// a restored full mode still stamps the explicit pair, so the deterministic
// chains keep direct-executing after a restart without a new user switch.
func TestChatAuthorityStampAfterRestartUsesRestoredState(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	s.mu.Lock()
	s.authorityMode = authorityModeFull // the persisted shape restore adopts
	s.mu.Unlock()
	state := s.projectAgentRuntimeStateLocked()
	restarted := New(nil, shadow.New(nil), nil)
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	restarted.mu.Lock()
	latch := restarted.authorityModeExplicit
	restarted.mu.Unlock()
	if latch {
		t.Fatal("restart must not fabricate the in-process explicit latch")
	}
	if mode := authorityGetForTest(restarted); mode != authorityModeFull {
		t.Fatalf("restored authority mode=%q", mode)
	}
	bound, err := restarted.bindChatAuthorityMode("", map[string]any{"conversation_id": "conv-stamp-restart"})
	if err != nil || firstStringFromMap(bound, "authority_mode") != authorityModeFull || !boolValue(bound["authority_mode_explicit"]) {
		t.Fatalf("bound=%+v err=%v", bound, err)
	}
}

// Probe 5 (stale downgrade echo): a caller restating manual while the server
// holds full access keeps its own (conservative) value on the context — the
// invoke-path parity — and never writes the server state back down.
func TestStaleManualEchoDoesNotDowngradeServerFullAccess(t *testing.T) {
	s := New(nil, shadow.New(nil), nil)
	if resp := authorityPostForTest(s, authorityModeFull); resp.Code != http.StatusOK {
		t.Fatalf("switch status=%d body=%s", resp.Code, resp.Body.String())
	}
	bound, err := s.bindChatAuthorityMode(authorityModeManual, map[string]any{"conversation_id": "conv-stamp-echo"})
	if err != nil {
		t.Fatalf("manual restatement rejected: %v", err)
	}
	if firstStringFromMap(bound, "authority_mode") != authorityModeManual {
		t.Fatalf("caller restatement not kept: %+v", bound)
	}
	if mode := s.authorityModeSnapshot(); mode != authorityModeFull {
		t.Fatalf("manual echo downgraded server mode to %q", mode)
	}
}
