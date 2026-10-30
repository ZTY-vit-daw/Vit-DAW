package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
)

// M3-C4-AUTO-CLARIFY-UX-1 (card 219) acceptance pins: the span a user already
// said is offered back as a confirmable candidate instead of being dropped
// (faces 1+3), an affirmative word during a pending clarification gets the
// differentiated guidance instead of a verbatim replay (face 2), the choreo
// confirmation card auto-executes under the explicit full-access pair (face
// 4), and the authority switch discloses whether it actually persisted
// (face 5). 不猜原则不破: a candidate is adopted only by an explicit
// affirmative reply, never by default.

func clarifyUXTestTarget() automationChoreoTarget {
	return automationChoreoTarget{TrackID: "1007", TrackLabel: "Bass", PluginID: "plugin-vol", PluginName: "Volume & Pan", ParamID: "volume"}
}

func TestParseAutomationRegionBareSpanCandidate(t *testing.T) {
	// Face 3: a bare "N 秒" rides the clarification channel as a candidate.
	region := parseAutomationRegionIntent("做 30 秒的渐强")
	if region.Source != automationRegionNeedsClarify || region.BareSpanSeconds != 30 {
		t.Fatalf("bare span parse = %+v, want clarify with 30s candidate", region)
	}
	// No number, no candidate.
	region = parseAutomationRegionIntent("让存在感慢慢强起来")
	if region.Source != automationRegionNeedsClarify || region.BareSpanSeconds != 0 {
		t.Fatalf("windowless parse = %+v, want clarify without candidate", region)
	}
	// Absolute windows and section words resolve before the bare channel.
	region = parseAutomationRegionIntent("前 30 秒")
	if region.Source != automationRegionSourceAbsolute {
		t.Fatalf("前 30 秒 parse = %+v, want absolute", region)
	}
	region = parseAutomationRegionIntent("副歌那 30 秒让存在感慢慢强起来")
	if region.Source != automationRegionSourceMarker || region.SectionSeconds != 30 || region.BareSpanSeconds != 0 {
		t.Fatalf("section parse = %+v, want marker/30s and no bare span", region)
	}
}

func TestSpanCandidateClarifyAsksInsteadOfDropping(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	// Face 1: section hit + marker missing + span present.
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	resp := server.automationChoreoClarifySpanCandidateResponse("conv-span", agentModeDefault, "goal-span", "run-span",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	for _, want := range []string{"副歌", "没有找到对应区间", "用开头 30 秒可以吗", "回「可以」"} {
		if !strings.Contains(resp.Reply, want) {
			t.Errorf("face-1 clarify reply %q missing %q", resp.Reply, want)
		}
	}
	if resp.StopReason != "automation_choreo_clarify_region_span_candidate" {
		t.Errorf("face-1 clarify stop reason = %q", resp.StopReason)
	}
	stored, ok := server.automationChoreoForConversation("conv-span")
	if !ok || firstStringFromMap(stored.Clarify, "kind") != "region_span_candidate" {
		t.Fatalf("span candidate clarify not stored: %#v", stored)
	}
	if stored.TrackID != "1007" || stored.ParamID != "volume" || stored.Category != automationCategoryLevel {
		t.Fatalf("stored clarify lost the resolved target context: %+v", stored)
	}
	// Face 3: bare span question names the 0-start candidate explicitly.
	resp = server.automationChoreoClarifySpanCandidateResponse("conv-bare", agentModeDefault, "goal-bare", "run-bare",
		"做 30 秒的渐强", intent, clarifyUXTestTarget(), "", 30)
	if !strings.Contains(resp.Reply, "是从 0 开始的 30 秒吗") {
		t.Errorf("face-3 clarify reply %q missing the 0-start candidate ask", resp.Reply)
	}
}

func TestSpanCandidateAdoptionRequiresExplicitAffirmative(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	server.automationChoreoClarifySpanCandidateResponse("conv-adopt", agentModeDefault, "goal-adopt", "run-adopt",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-adopt",
		ChatRequest{ConversationID: "conv-adopt", Message: "可以", Context: map[string]any{}}, agentModeDefault)
	if !handled {
		t.Fatal("affirmative answer was not routed to the pending span candidate")
	}
	if resp.StopReason != "automation_choreo_awaiting_confirmation" || len(resp.InteractionRequests) != 1 ||
		resp.InteractionRequests[0].Kind != "automation_choreo_confirmation" {
		t.Fatalf("adopted candidate did not reach the confirmation card: stop=%s requests=%#v", resp.StopReason, resp.InteractionRequests)
	}
	stored, ok := server.automationChoreoForConversation("conv-adopt")
	if !ok {
		t.Fatal("adopted proposal not stored")
	}
	if stored.RegionStart != 0 || stored.RegionEnd != 30 {
		t.Fatalf("adopted region = %.1f-%.1f, want 0-30", stored.RegionStart, stored.RegionEnd)
	}
	if stored.TrackID != "1007" || stored.PluginName != "Volume & Pan" || stored.ParamID != "volume" {
		t.Fatalf("adopted proposal lost the stored target context: %+v", stored)
	}
	if len(stored.Points) != 2 || stored.Points[0]["t"].(float64) != 0 || stored.Points[1]["t"].(float64) != 30 {
		t.Fatalf("adopted points not rebuilt over 0-30: %+v", stored.Points)
	}
	// An absolute region answer takes the merge path, never the adoption.
	server2 := New(nil, shadow.New(nil), nil)
	server2.automationChoreoClarifySpanCandidateResponse("conv-abs", agentModeDefault, "goal-abs", "run-abs",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	resp, _ = server2.handlePendingAutomationChoreoChat(context.Background(), "conv-abs",
		ChatRequest{ConversationID: "conv-abs", Message: "0 到 20 秒", Context: map[string]any{"selected_track_id": "1007"}}, agentModeDefault)
	if resp.StopReason == "automation_choreo_awaiting_confirmation" {
		t.Fatalf("absolute region answer was swallowed by the adoption path: %#v", resp)
	}
	// Interrogative 可以吗 is a question, not an acceptance: no adoption.
	server3 := New(nil, shadow.New(nil), nil)
	server3.automationChoreoClarifySpanCandidateResponse("conv-q", agentModeDefault, "goal-q", "run-q",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	resp, _ = server3.handlePendingAutomationChoreoChat(context.Background(), "conv-q",
		ChatRequest{ConversationID: "conv-q", Message: "可以吗", Context: map[string]any{"selected_track_id": "1007"}}, agentModeDefault)
	if resp.StopReason == "automation_choreo_awaiting_confirmation" {
		t.Fatalf("interrogative 可以吗 adopted the candidate: %#v", resp)
	}
}

func TestClarifyPendingAffirmativeGetsGuidanceNotReplay(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	intent := parseAutomationIntent("让这条轨的存在感慢慢强起来")
	first := server.automationChoreoClarifyResponse("conv-guide", agentModeDefault, "goal-guide", "run-guide",
		"让这条轨的存在感慢慢强起来", intent, "这段变化从哪一秒到哪一秒？请给出区间（例如「0 到 30 秒」）。", "region")
	storedBefore, _ := server.automationChoreoForConversation("conv-guide")
	for _, answer := range []string{"确认写入", "写入", "可以吗", "好的"} {
		resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-guide",
			ChatRequest{ConversationID: "conv-guide", Message: answer}, agentModeDefault)
		if !handled {
			t.Fatalf("%q was not routed to the pending clarification", answer)
		}
		if resp.StopReason != "automation_choreo_clarify_affirm_guidance" {
			t.Fatalf("%q reply stop=%s, want guidance", answer, resp.StopReason)
		}
		if !strings.Contains(resp.Reply, "还在等时间区间") || !strings.Contains(resp.Reply, "尚未生成") {
			t.Errorf("%q guidance reply %q missing the still-waiting disclosure", answer, resp.Reply)
		}
		if strings.Contains(resp.Reply, first.Reply) {
			t.Errorf("%q guidance replayed the original ask verbatim", answer)
		}
		storedAfter, ok := server.automationChoreoForConversation("conv-guide")
		if !ok || storedAfter.ID != storedBefore.ID {
			t.Fatalf("%q guidance replaced or expired the pending clarification", answer)
		}
	}
	// A real region answer takes the merge path (with no kernel the level
	// plugin resolve fails honestly downstream — the pinned property is that
	// the answer is NOT intercepted as an affirmative).
	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-guide",
		ChatRequest{ConversationID: "conv-guide", Message: "0 到 30 秒", Context: map[string]any{"selected_track_id": "1007"}}, agentModeDefault)
	if !handled || resp.StopReason == "automation_choreo_clarify_affirm_guidance" {
		t.Fatalf("real region answer intercepted as affirmative: handled=%t stop=%s", handled, resp.StopReason)
	}
}

func TestAffirmFamiliesAreTight(t *testing.T) {
	for _, msg := range []string{"可以", "好的", "确认写入", "写入", "可以吗", "嗯", "ok"} {
		if !automationChoreoAffirmAnswer(msg) {
			t.Errorf("automationChoreoAffirmAnswer(%q) = false", msg)
		}
	}
	for _, msg := range []string{"0 到 30 秒", "改成 10 到 40 秒", "用副歌那一段", "电平", "把存在感调强一点"} {
		if automationChoreoAffirmAnswer(msg) {
			t.Errorf("automationChoreoAffirmAnswer(%q) = true for a content answer", msg)
		}
	}
	for _, msg := range []string{"可以", "好的", "同意", "确认写入", "就这样"} {
		if !automationChoreoCandidateAffirm(msg) {
			t.Errorf("automationChoreoCandidateAffirm(%q) = false", msg)
		}
	}
	if automationChoreoCandidateAffirm("可以吗") {
		t.Errorf("interrogative 可以吗 must not adopt a candidate")
	}
	if automationChoreoCandidateAffirm("0 到 30 秒的区间应该也行") {
		t.Errorf("long sentence must not adopt a candidate")
	}
}

func TestFullAccessChoreoConfirmationGate(t *testing.T) {
	if !fullAccessChoreoConfirmationContext(fullAccessTestContext(nil)) {
		t.Fatal("explicit full-access pair refused")
	}
	if fullAccessChoreoConfirmationContext(manualTestContext(nil)) {
		t.Fatal("manual_confirmation treated as a full-access grant")
	}
	if fullAccessChoreoConfirmationContext(map[string]any{"authority_mode": authorityModeFull}) {
		t.Fatal("bare mode string without the explicit flag was treated as a grant")
	}
	if fullAccessChoreoConfirmationContext(nil) {
		t.Fatal("empty context was treated as a grant")
	}
}

func TestFullAccessChoreoSpanCandidateExecutesWithoutConfirmationCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	server.automationChoreoClarifySpanCandidateResponse("conv-fa", agentModeDefault, "goal-fa", "run-fa",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-fa",
		ChatRequest{ConversationID: "conv-fa", Message: "可以", Context: fullAccessTestContext(nil)}, agentModeDefault)
	if !handled {
		t.Fatal("full-access adoption was not routed")
	}
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation {
		t.Fatalf("full-access choreo issued a confirmation card: %#v", resp.InteractionRequests)
	}
	if !boolValue(resp.WorkflowData["full_access_auto_authorized"]) ||
		firstStringFromMap(resp.WorkflowData, "confirmation_bypassed") != "full_project_access" ||
		boolValue(resp.WorkflowData["interaction_card_issued"]) {
		t.Fatalf("full-access choreo omitted autonomy markers: %#v", resp.WorkflowData)
	}
	// With no kernel behind the test server the governed leg fails honestly;
	// the pinned property is an execution outcome, never a parked card.
	if resp.GoalStatus == "waiting_confirmation" {
		t.Fatalf("full-access choreo still waits for the user: %#v", resp)
	}
	if _, ok := server.automationChoreoForConversation("conv-fa"); ok {
		t.Fatal("full-access choreo left a pending confirmation behind")
	}
}

func TestManualChoreoSpanCandidateKeepsConfirmationCard(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	server.automationChoreoClarifySpanCandidateResponse("conv-manual", agentModeDefault, "goal-manual", "run-manual",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, clarifyUXTestTarget(), "副歌", 30)
	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-manual",
		ChatRequest{ConversationID: "conv-manual", Message: "可以", Context: manualTestContext(nil)}, agentModeDefault)
	if !handled {
		t.Fatal("manual adoption was not routed")
	}
	if resp.GoalStatus != "waiting_confirmation" || len(resp.InteractionRequests) != 1 ||
		resp.InteractionRequests[0].Kind != "automation_choreo_confirmation" {
		t.Fatalf("manual choreo lost its confirmation card: status=%s requests=%#v", resp.GoalStatus, resp.InteractionRequests)
	}
	if boolValue(resp.WorkflowData["full_access_auto_authorized"]) {
		t.Fatalf("manual choreo claimed autonomy: %#v", resp.WorkflowData)
	}
}

// ---- Face 5: the authority switch discloses persistence ----

func TestAuthorityModeSwitchDisclosesPersistence(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/agent/authority", server.handleAuthorityMode)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Post(srv.URL+"/agent/authority", "application/json", strings.NewReader(`{"authority_mode":"full_project_access"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("switch POST status = %d", resp.StatusCode)
	}
	body := map[string]any{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if firstStringFromMap(body, "authority_mode") != authorityModeFull {
		t.Fatalf("switch response mode = %v", body["authority_mode"])
	}
	// No active project workspace on the bare test server: the switch lived
	// in memory only, and the response must say so instead of a bare ok.
	if boolValue(body["workspace_ready"]) || boolValue(body["persisted"]) {
		t.Fatalf("switch response claims durability without a workspace: %#v", body)
	}
	if server.authorityModeSnapshot() != authorityModeFull {
		t.Fatalf("in-memory mode = %q", server.authorityModeSnapshot())
	}
}

// TestAuthorityModePersistenceRoundTrip is the face-5 chain proof: the webui
// switch write face (handleAuthorityMode → projectAgentRuntimeState.AuthorityMode)
// and the agent-start restore read face (restoreProjectAgentRuntimeStateLocked
// adopted branch) ride the same durable file — full access written by the
// switch survives a restart-shaped reload; a manual disk value adopts as
// manual (the 2026-10-29 incident shape when the switch never reached disk).
func TestAuthorityModePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "proj.vit")
	projectUUID := "uuid-authority-roundtrip"

	writeState := func(mode string) {
		t.Helper()
		data, err := json.Marshal(projectAgentRuntimeState{SavedAt: time.Now().UTC(), AuthorityMode: mode})
		if err != nil {
			t.Fatal(err)
		}
		if err := history.WriteAgentRuntimeState(projectPath, projectUUID, data); err != nil {
			t.Fatal(err)
		}
	}
	restore := func(t *testing.T) *Server {
		t.Helper()
		raw, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
		if err != nil {
			t.Fatal(err)
		}
		var state projectAgentRuntimeState
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		server := &Server{logger: nil}
		server.restoreProjectAgentRuntimeStateLocked(state)
		return server
	}

	writeState(authorityModeFull)
	if server := restore(t); server.authorityModeSnapshot() != authorityModeFull {
		t.Fatalf("full access did not survive the restart-shaped reload: %q", server.authorityModeSnapshot())
	}
	// The incident shape: disk carries manual (the switch never reached it),
	// a fresh agent adopts manual at startup.
	writeState(authorityModeManual)
	if server := restore(t); server.authorityModeSnapshot() != authorityModeManual {
		t.Fatalf("manual disk value did not adopt: %q", server.authorityModeSnapshot())
	}
}
