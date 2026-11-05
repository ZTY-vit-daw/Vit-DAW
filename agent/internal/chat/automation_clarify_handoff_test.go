package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/shadow"
)

// M3-CLARIFY-LLM-HANDOFF-1 (card 225) acceptance pins: the five
// information-missing forms (marker miss / span candidate / bare span /
// category miss / param miss) hand their ask decision to a real LLM turn
// whose request carries the capability disclosure (section-marker family)
// and the deterministic candidates; the 219 fixed asks survive verbatim as
// the fail-safe with an honest disclose; a model-proposed capability (marker
// writing) routes into the governed command chain — a proposal, never a bare
// write; the pending clarification state machine is not broken by the
// migration.

// clarifyHandoffTestLLM spins a fake OpenAI-shape engine that captures every
// request body and answers with the given reply/commands envelope.
func clarifyHandoffTestLLM(t *testing.T, reply string, commands []map[string]any, captured *[]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if captured != nil {
			*captured = append(*captured, string(body))
		}
		// The chat envelope rides the message content as a JSON *string*
		// (the production engines return string content).
		content, _ := json.Marshal(map[string]any{"reply": reply, "commands": commands})
		outer, _ := json.Marshal(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": string(content)}}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(outer)
	}))
	t.Cleanup(server.Close)
	return server
}

// clarifyHandoffPointServer points the chat server's LLM engine env at the
// fake engine and installs its HTTP client. Env values must stay non-empty:
// config.Load() merges the on-disk engine config first and only overrides
// with non-empty env values — an empty env would fall through to the real
// local engine file (observed once during this card's test bring-up; the
// stray real call is disclosed in the card receipt).
func clarifyHandoffPointServer(t *testing.T, server *Server, model *httptest.Server) {
	t.Helper()
	t.Setenv("VIT_AGENT_LLM_BASE_URL", model.URL+"/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "test-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "test-model")
	server.llm = &llm.Client{HTTPClient: model.Client()}
}

// clarifyHandoffBrokenEngine points the engine at a fake that always fails
// (HTTP 500): the deterministic fail-safe face — the hand-off turn errors
// out and the fixed 219 ask answers with the honest disclose.
func clarifyHandoffBrokenEngine(t *testing.T, server *Server) {
	t.Helper()
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"engine down"}`))
	}))
	t.Cleanup(broken.Close)
	clarifyHandoffPointServer(t, server, broken)
}

func clarifyHandoffTestServer() *Server {
	return New(nil, shadow.New(nil), nil)
}

// TestClarifyHandoffFiveFormsTakeLLMTurnWithCatalogAndCandidates walks the
// five card-face information-missing forms through their migrated wrappers:
// each must run a real (fake-engine) LLM turn whose request carries the
// capability disclosure (project.markers.*) plus the form's own candidates,
// and must answer with the model's wording stored on a pending clarification.
func TestClarifyHandoffFiveFormsTakeLLMTurnWithCatalogAndCandidates(t *testing.T) {
	const modelAsk = "这段变化想作用在哪个范围？我可以先帮这段打上段落标记，要吗？"
	intent := parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来")
	if !intent.TimeVarying {
		t.Fatalf("fixture phrase must arm: %+v", intent)
	}
	target := clarifyUXTestTarget()

	cases := []struct {
		name string
		kind string
		run  func(server *Server) ChatResponse
		// request-body needles proving the hand-off payload carried the form
		requestNeedles  []string
		clarifyNeedles  []string
		spanCandidate   bool
		sectionCaptured string
	}{
		{
			name: "category miss",
			kind: "category",
			run: func(server *Server) ChatResponse {
				return server.automationChoreoClarifyHandoffResponse(context.Background(), "conv-h-1", agentModeDefault, "goal-h", "run-h",
					"30 秒之后动起来", intent, "category", "这段变化想调哪一类参数？", map[string]any{"category_words": []string{"电平", "空间", "音色"}})
			},
			requestNeedles: []string{"category_words", "电平"},
			clarifyNeedles: []string{"category"},
		},
		{
			name: "param miss",
			kind: "param",
			run: func(server *Server) ChatResponse {
				return server.automationChoreoClarifyHandoffResponse(context.Background(), "conv-h-2", agentModeDefault, "goal-h", "run-h",
					"让这条轨的空间感慢慢铺满", intent, "param", "轨上插件：Reverb。", map[string]any{"track_plugins": []string{"Reverb"}, "category": "space"})
			},
			requestNeedles: []string{"track_plugins", "Reverb"},
			clarifyNeedles: []string{"param"},
		},
		{
			name: "marker miss without span",
			kind: "region",
			run: func(server *Server) ChatResponse {
				return server.automationChoreoClarifyHandoffResponse(context.Background(), "conv-h-3", agentModeDefault, "goal-h", "run-h",
					"副歌那段，让这条轨的存在感慢慢强起来", intent, "region", "话术里的「副歌」在工程段落标记里没有找到对应区间。", map[string]any{"section_name": "副歌"})
			},
			requestNeedles: []string{"section_name", "副歌"},
			clarifyNeedles: []string{"region"},
		},
		{
			name: "span candidate (marker miss with span)",
			kind: "region_span_candidate",
			run: func(server *Server) ChatResponse {
				return server.automationChoreoSpanCandidateHandoffResponse(context.Background(), "conv-h-4", agentModeDefault, "goal-h", "run-h",
					"副歌那 30 秒，让这条轨的存在感慢慢强起来", intent, target, "副歌", 30)
			},
			requestNeedles: []string{"span_seconds", "副歌", "adopt_region_seconds"},
			clarifyNeedles: []string{"region_span_candidate"},
			spanCandidate:  true,
		},
		{
			name: "bare span (face-3 form)",
			kind: "region_span_candidate",
			run: func(server *Server) ChatResponse {
				return server.automationChoreoSpanCandidateHandoffResponse(context.Background(), "conv-h-5", agentModeDefault, "goal-h", "run-h",
					"做 30 秒的渐强", intent, target, "", 30)
			},
			requestNeedles: []string{"span_seconds", "adopt_region_seconds"},
			clarifyNeedles: []string{"region_span_candidate"},
			spanCandidate:  true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var captured []string
			model := clarifyHandoffTestLLM(t, modelAsk, nil, &captured)
			server := clarifyHandoffTestServer()
			clarifyHandoffPointServer(t, server, model)

			resp := testCase.run(server)

			if resp.Reply != modelAsk {
				t.Fatalf("reply = %q, want the model wording %q", resp.Reply, modelAsk)
			}
			if resp.WorkflowData["clarify_llm"] != "handoff" {
				t.Fatalf("clarify_llm = %v, want handoff (WorkflowData=%+v)", resp.WorkflowData["clarify_llm"], resp.WorkflowData)
			}
			if resp.StopReason != "automation_choreo_clarify_"+testCase.kind {
				t.Fatalf("stop reason = %q, want kind %q", resp.StopReason, testCase.kind)
			}
			if len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].Body != modelAsk {
				t.Fatalf("clarify card body must carry the model wording: %+v", resp.InteractionRequests)
			}
			if len(captured) == 0 {
				t.Fatalf("no LLM turn was made")
			}
			request := captured[len(captured)-1]
			for _, needle := range append([]string{"project.markers.upsert", "project.markers.apply_section_markers"}, testCase.requestNeedles...) {
				if !strings.Contains(request, needle) {
					t.Errorf("hand-off request missing %q (request head: %.400s)", needle, request)
				}
			}
			if !strings.Contains(request, "你是 Vit-DAW 的混音助手") {
				t.Errorf("hand-off request missing the protocol instruction")
			}

			conversationID := firstNonEmpty(resp.ConversationID, "")
			stored, ok := server.automationChoreoForConversation(conversationID)
			if !ok {
				t.Fatalf("pending clarification not stored for %q", conversationID)
			}
			if firstStringFromMap(stored.Clarify, "kind") != testCase.kind {
				t.Fatalf("stored kind = %q, want %q", firstStringFromMap(stored.Clarify, "kind"), testCase.kind)
			}
			if firstStringFromMap(stored.Clarify, "question") != modelAsk {
				t.Fatalf("stored question = %q, want the model wording", firstStringFromMap(stored.Clarify, "question"))
			}
			if stored.Clarify["llm_handoff"] != true {
				t.Fatalf("stored clarify must record the llm hand-off: %#v", stored.Clarify)
			}
			if testCase.spanCandidate {
				if span, spanOK := automationChoreoNumber(stored.Clarify["span_seconds"]); !spanOK || span != 30 {
					t.Fatalf("stored span_seconds = %#v, want 30 (adoption chain input)", stored.Clarify["span_seconds"])
				}
				if stored.TrackID != target.TrackID || stored.ParamID != target.ParamID {
					t.Fatalf("stored span candidate lost the resolved target: %+v", stored)
				}
			}
		})
	}
}

// TestClarifyHandoffFallsBackToFixedAskWithDisclose pins the fail-safe: with
// the engine config unavailable the five forms answer with the 219 fixed
// wording verbatim, the fallback is disclosed on WorkflowData, and the
// pending state machine keeps the same inputs (kind + span candidate) so the
// answer/adoption chain is unchanged.
func TestClarifyHandoffFallsBackToFixedAskWithDisclose(t *testing.T) {
	server := clarifyHandoffTestServer()
	clarifyHandoffBrokenEngine(t, server)

	const fixedAsk = "这段变化想调哪一类参数？电平类（存在感/能量/音量）、空间类（远近/湿度）还是音色类（亮闷薄）？说「电平」「空间」或「音色」，或直接重说完整话术。"
	resp := server.automationChoreoClarifyHandoffResponse(context.Background(), "conv-fb-1", agentModeDefault, "goal-fb", "run-fb",
		"30 秒之后动起来", parseAutomationIntent("30 秒之后动起来"), "category", fixedAsk, map[string]any{"category_words": []string{"电平", "空间", "音色"}})
	if resp.Reply != fixedAsk {
		t.Fatalf("fallback reply = %q, want the 219 fixed ask verbatim", resp.Reply)
	}
	outcome, _ := resp.WorkflowData["clarify_llm"].(string)
	if !strings.HasPrefix(outcome, "fallback:") {
		t.Fatalf("clarify_llm = %q, want a fallback:<reason> disclose", outcome)
	}
	stored, ok := server.automationChoreoForConversation("conv-fb-1")
	if !ok || firstStringFromMap(stored.Clarify, "kind") != "category" {
		t.Fatalf("fallback pending clarify not stored: %#v", stored)
	}
	if stored.Clarify["llm_handoff"] != nil {
		t.Fatalf("fallback pending must not claim an llm hand-off: %#v", stored.Clarify)
	}

	// Span-candidate fallback keeps the adoption-chain inputs byte-compatible
	// with the 219 face-1/3 ask (span + section + resolved target).
	spanResp := server.automationChoreoSpanCandidateHandoffResponse(context.Background(), "conv-fb-2", agentModeDefault, "goal-fb", "run-fb",
		"做 30 秒的渐强", parseAutomationIntent("做 30 秒的渐强"), clarifyUXTestTarget(), "", 30)
	if !strings.Contains(spanResp.Reply, "是从 0 开始的 30 秒吗") {
		t.Fatalf("span fallback reply = %q, want the 219 face-3 ask", spanResp.Reply)
	}
	if outcome, _ := spanResp.WorkflowData["clarify_llm"].(string); !strings.HasPrefix(outcome, "fallback:") {
		t.Fatalf("span fallback clarify_llm = %q, want fallback:<reason>", outcome)
	}
	spanStored, ok := server.automationChoreoForConversation("conv-fb-2")
	if !ok {
		t.Fatalf("span fallback pending not stored")
	}
	if span, spanOK := automationChoreoNumber(spanStored.Clarify["span_seconds"]); !spanOK || span != 30 {
		t.Fatalf("span fallback lost the adoption-chain span: %#v", spanStored.Clarify)
	}
	// The adoption table still works on a fallback-pending span candidate:
	// an explicit affirmative adopts region 0..30 (pending state machine not
	// broken by the migration).
	adoptResp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-fb-2", ChatRequest{
		ConversationID: "conv-fb-2", Message: "可以", Context: map[string]any{"agent_mode": agentModeDefault},
	}, agentModeDefault)
	if !handled {
		t.Fatalf("affirmative on the fallback span candidate was not handled")
	}
	if adoptResp.StopReason == "automation_choreo_clarify_affirm_guidance" {
		t.Fatalf("affirmative must adopt the span candidate, got the guidance branch: %+v", adoptResp)
	}
	if adoptResp.Workflow != automationChoreoWorkflow {
		t.Fatalf("adoption reply workflow = %q, want the choreography confirmation face", adoptResp.Workflow)
	}
}

// TestClarifyHandoffModelMarkerProposalRoutesGovernedChain pins the
// capability-proposal face: when the model answers the hand-off with a
// section-marker command (its judgment call on "副歌 without a marker"), the
// command routes through the governed chain — for the RiskConfirm marker
// apply command that means a confirmation card, never a bare write.
func TestClarifyHandoffModelMarkerProposalRoutesGovernedChain(t *testing.T) {
	modelReply := "工程里还没有「副歌」的段落标记。我先听一遍把副歌标记打上，好吗？"
	commands := []map[string]any{{
		"cmd":     "project.markers.apply_section_markers",
		"markers": []map[string]any{{"name": "副歌", "start_seconds": 30.0, "end_seconds": 60.0}},
	}}
	var captured []string
	model := clarifyHandoffTestLLM(t, modelReply, commands, &captured)
	server := clarifyHandoffTestServer()
	clarifyHandoffPointServer(t, server, model)

	resp := server.automationChoreoSpanCandidateHandoffResponse(context.Background(), "conv-cmd-1", agentModeDefault, "goal-cmd", "run-cmd",
		"副歌那 30 秒，让这条轨的存在感慢慢强起来", parseAutomationIntent("副歌那 30 秒，让这条轨的存在感慢慢强起来"),
		clarifyUXTestTarget(), "副歌", 30)

	if !resp.NeedsConfirmation {
		t.Fatalf("marker proposal must land on the governed confirmation card, got %+v", resp)
	}
	if resp.PlanID == "" || len(resp.Commands) == 0 {
		t.Fatalf("governed chain produced no pending plan: plan=%q commands=%d", resp.PlanID, len(resp.Commands))
	}
	if resp.WorkflowData["clarify_llm"] != "handoff" {
		t.Fatalf("governed route disclose = %v", resp.WorkflowData["clarify_llm"])
	}
	commandNamed := false
	for _, decision := range resp.Commands {
		if strings.Contains(strings.ToLower(decision.Command["cmd"].(string)+fmtSafe(decision.Command["tool"])), "markers") {
			commandNamed = true
		}
	}
	if !commandNamed {
		t.Fatalf("governed decisions lost the marker command: %+v", resp.Commands)
	}
}

func fmtSafe(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// TestClarifyHandoffAffirmConsentRoutesMarkerProposal pins the face-2
// migration: after an LLM-worded clarification proposed marking the chorus, a
// bare affirmative is the model's judgment call — the model may request the
// marker capability, which routes through the governed chain.
func TestClarifyHandoffAffirmConsentRoutesMarkerProposal(t *testing.T) {
	commands := []map[string]any{{
		"cmd":     "project.markers.apply_section_markers",
		"markers": []map[string]any{{"name": "副歌", "start_seconds": 30.0, "end_seconds": 60.0}},
	}}
	model := clarifyHandoffTestLLM(t, "好的，我先打上副歌标记。", commands, nil)
	server := clarifyHandoffTestServer()
	clarifyHandoffPointServer(t, server, model)

	server.storeAutomationChoreoCandidate("conv-affirm-1", &automationChoreoCandidate{
		ID: "automation_choreo_test", CreatedAt: time.Now().UTC(), Status: "pending_confirmation",
		GoalID: "goal-affirm", RunID: "run-affirm", UserText: "副歌那 30 秒，让这条轨的存在感慢慢强起来",
		Category: automationCategoryLevel,
		Clarify: map[string]any{
			"kind": "region", "question": "要不要我先听一遍把副歌标记打上？",
			"intent_time_varying": true, "llm_handoff": true,
		},
	})

	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-affirm-1", ChatRequest{
		ConversationID: "conv-affirm-1", Message: "可以", Context: map[string]any{"agent_mode": agentModeDefault},
	}, agentModeDefault)
	if !handled {
		t.Fatalf("affirmative turn not handled")
	}
	if !resp.NeedsConfirmation || resp.PlanID == "" {
		t.Fatalf("consent must route the marker proposal into the governed confirmation chain: %+v", resp)
	}
	if outcome := resp.WorkflowData["clarify_llm"]; outcome != "handoff_affirm_commands" {
		t.Fatalf("affirm hand-off disclose = %v, want handoff_affirm_commands", outcome)
	}
}

// TestClarifyHandoffAffirmTextJudgmentKeepsPending pins the face-2 text
// branch: when the model answers an affirmative with more words instead of a
// capability request, the pending clarification stays alive (the 219 guidance
// contract) and the answer carries the honest disclose.
func TestClarifyHandoffAffirmTextJudgmentKeepsPending(t *testing.T) {
	model := clarifyHandoffTestLLM(t, "好的，那我等你给我一个时间区间，比如「0 到 30 秒」。", nil, nil)
	server := clarifyHandoffTestServer()
	clarifyHandoffPointServer(t, server, model)

	server.storeAutomationChoreoCandidate("conv-affirm-2", &automationChoreoCandidate{
		ID: "automation_choreo_test2", CreatedAt: time.Now().UTC(), Status: "pending_confirmation",
		GoalID: "goal-affirm2", RunID: "run-affirm2", UserText: "副歌那 30 秒，让这条轨的存在感慢慢强起来",
		Category: automationCategoryLevel,
		Clarify: map[string]any{
			"kind": "region", "question": "要不要我先听一遍把副歌标记打上？",
			"intent_time_varying": true, "llm_handoff": true,
		},
	})

	resp, handled := server.handlePendingAutomationChoreoChat(context.Background(), "conv-affirm-2", ChatRequest{
		ConversationID: "conv-affirm-2", Message: "嗯", Context: map[string]any{"agent_mode": agentModeDefault},
	}, agentModeDefault)
	if !handled {
		t.Fatalf("affirmative turn not handled")
	}
	if resp.StopReason != "automation_choreo_clarify_affirm_guidance" {
		t.Fatalf("stop reason = %q, want the affirm-guidance face", resp.StopReason)
	}
	if resp.WorkflowData["clarify_llm"] != "handoff" {
		t.Fatalf("affirm text disclose = %v", resp.WorkflowData["clarify_llm"])
	}
	if _, stillPending := server.automationChoreoForConversation("conv-affirm-2"); !stillPending {
		t.Fatalf("pending clarification must stay alive after a text judgment answer")
	}
}

// TestClarifyEntryRoutesCategoryMissThroughHandoff pins the wiring end to
// end: an armed phrase with a missing category enters the migrated exit from
// the deterministic resolve pass (selected track context, real handler) and
// answers with the model's wording.
func TestClarifyEntryRoutesCategoryMissThroughHandoff(t *testing.T) {
	const modelAsk = "要动哪一类参数？电平可以直接做；空间或音色需要轨上挂插件。"
	model := clarifyHandoffTestLLM(t, modelAsk, nil, nil)
	server := clarifyHandoffTestServer()
	clarifyHandoffPointServer(t, server, model)

	resp, handled := server.handleAutomationChoreoChat(context.Background(), "conv-entry-1", ChatRequest{
		ConversationID: "conv-entry-1",
		Message:        "30 秒之后动起来",
		Context:        map[string]any{"agent_mode": agentModeDefault, "selected_track_id": "1007"},
	}, agentModeDefault)
	if !handled {
		t.Fatalf("armed category-miss phrase was not handled by the choreography entry")
	}
	if resp.Reply != modelAsk {
		t.Fatalf("entry reply = %q, want the model wording", resp.Reply)
	}
	if resp.WorkflowData["clarify_llm"] != "handoff" {
		t.Fatalf("entry disclose = %v, want handoff", resp.WorkflowData["clarify_llm"])
	}
	stored, ok := server.automationChoreoForConversation("conv-entry-1")
	if !ok || firstStringFromMap(stored.Clarify, "kind") != "category" {
		t.Fatalf("entry pending clarify not stored as category miss: %#v", stored)
	}
}
