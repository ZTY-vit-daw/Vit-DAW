package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
)

func TestCompactStripSilenceResponsePayloadRemovesRawRegions(t *testing.T) {
	region := map[string]any{
		"clip_id":       "clip_a",
		"track_id":      "track_a",
		"start_seconds": 0.0,
		"end_seconds":   0.25,
	}
	executed := []map[string]any{{
		"status":       "ok",
		"tool":         "clip.strip_silence.suggest",
		"command_name": "clip.strip_silence.suggest",
		"result": map[string]any{
			"status":               "ok",
			"schema_version":       "clip.strip_silence.suggest.v0",
			"scope":                "all_project",
			"pending_action_count": 1,
			"analysis": map[string]any{
				"strip_region_count": 1,
				"analyses": []any{map[string]any{
					"clip_id":       "clip_a",
					"track_id":      "track_a",
					"analysis_id":   "analysis_a",
					"strip_regions": []any{region},
				}},
			},
			"pending_actions": []any{map[string]any{
				"tool_name": "clip.strip_silence.apply",
				"args": map[string]any{
					"clip_id":       "clip_a",
					"track_id":      "track_a",
					"analysis_id":   "analysis_a",
					"strip_regions": []any{region},
				},
			}},
		},
	}}

	compactExecuted := compactAgentLoopExecutedForResponse(executed)
	data, err := json.Marshal(compactExecuted)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("compact executed response leaked raw strip_regions: %s", string(data))
	}
	if !strings.Contains(string(data), `"pending_action_preview"`) {
		t.Fatalf("compact executed response missing action preview: %s", string(data))
	}

	decisions := []policy.Decision{{
		Name: "clip.strip_silence.apply_batch",
		Risk: policy.RiskConfirm,
		Command: map[string]any{
			"cmd":             "clip.strip_silence.apply_batch",
			"pending_actions": executed[0]["result"].(map[string]any)["pending_actions"],
		},
	}}
	compactDecisions := compactAgentLoopDecisionsForResponse(decisions)
	data, err = json.Marshal(compactDecisions)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("compact decision leaked raw strip_regions: %s", string(data))
	}
	if !strings.Contains(string(data), `"pending_action_count"`) {
		t.Fatalf("compact decision missing pending_action_count: %s", string(data))
	}
}

func TestStripSilencePendingPlanHydrateUsesCompactCommands(t *testing.T) {
	decisions := testStripSilenceBatchDecisions()
	s := New(nil, nil, nil)
	s.pending["plan_strip"] = PendingPlan{
		ID:           "plan_strip",
		Decisions:    decisions,
		Context:      map[string]any{"goal_id": "goal_strip"},
		WorkflowData: map[string]any{"conversation_id": "conv_strip"},
	}
	resp := ChatResponse{
		ConversationID:    "conv_strip",
		NeedsConfirmation: true,
	}
	s.hydrateConfirmationResponse(&resp)

	data, err := json.Marshal(resp.Commands)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("hydrated pending plan leaked raw strip_regions: %s", string(data))
	}
	if !strings.Contains(string(data), `"pending_action_count"`) {
		t.Fatalf("hydrated pending plan missing pending_action_count: %s", string(data))
	}
}

func TestStripSilenceConfirmationInteractionUsesCompactCommands(t *testing.T) {
	s := New(nil, nil, nil)
	resp := ChatResponse{
		ConversationID:    "conv_strip",
		GoalID:            "goal_strip",
		RunID:             "run_strip",
		Reply:             "needs confirmation",
		NeedsConfirmation: true,
		PlanID:            "plan_strip",
		Commands:          testStripSilenceBatchDecisions(),
	}
	req := s.confirmationInteractionRequest(resp)
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("confirmation interaction leaked raw strip_regions: %s", string(data))
	}
	if !strings.Contains(string(data), `"pending_action_count"`) {
		t.Fatalf("confirmation interaction missing pending_action_count: %s", string(data))
	}
}

func TestStripSilenceChatResponseTransportCompactsExistingInteractionPayload(t *testing.T) {
	resp := ChatResponse{
		Commands: testStripSilenceBatchDecisions(),
		InteractionRequests: []AgentInteractionRequest{{
			ID:      "interaction_strip",
			Payload: map[string]any{"commands": testStripSilenceBatchDecisions()},
			Data:    map[string]any{"commands": testStripSilenceBatchDecisions()},
		}},
	}
	compactStripSilenceChatResponseForTransport(&resp)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("transport response leaked raw strip_regions: %s", string(data))
	}
	if strings.Count(string(data), `"pending_action_count"`) < 3 {
		t.Fatalf("transport response did not compact all command locations: %s", string(data))
	}
}

func TestChatResponseTransportOmitsRepeatedLargeProjectHistory(t *testing.T) {
	largeGraph := map[string]any{"nodes": []any{map[string]any{"message_data": strings.Repeat("x", 17<<20)}}}
	resp := ChatResponse{
		ProjectHistory: map[string]any{
			"available":          true,
			"project_path":       `D:\\songs\\mix.vit`,
			"active_branch":      "main",
			"conversation_graph": largeGraph,
			"conversation_messages": []map[string]any{
				{"role": "assistant", "content": "old proposal", "message_data": largeGraph},
				{"role": "user", "content": "可以执行", "node_id": "node-user"},
				{"role": "assistant", "content": "执行与验证均通过。", "node_id": "node-agent", "message_kind": "result", "logical_message_id": "result-1", "message_data": largeGraph},
			},
		},
		AgentPlan: &AgentPlan{ProjectHistory: map[string]any{
			"available":          true,
			"project_path":       `D:\\songs\\mix.vit`,
			"conversation_graph": largeGraph,
			"conversation_messages": []map[string]any{
				{"role": "user", "content": "可以执行"},
				{"role": "assistant", "content": "执行与验证均通过。"},
			},
		}},
		ExecutedKernelReply: []map[string]any{{
			"status": "ok",
			"tool":   "project.audio_analysis_status",
			"result": map[string]any{"status": "ok"},
			"project_history": map[string]any{
				"conversation_graph": largeGraph,
			},
		}},
	}

	compactStripSilenceChatResponseForTransport(&resp)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "conversation_graph") {
		t.Fatal("transport response retained a conversation graph")
	}
	messages := dictionaryRowsFromAny(resp.ProjectHistory["conversation_messages"])
	if len(messages) != 2 {
		t.Fatalf("chat transport response did not retain the two-message tail: %+v", messages)
	}
	if got := firstStringFromMap(messages[0], "role"); got != "user" {
		t.Fatalf("first retained role = %q, want user", got)
	}
	if got := firstStringFromMap(messages[0], "content"); got != "可以执行" {
		t.Fatalf("first retained content = %q", got)
	}
	if got := firstStringFromMap(messages[1], "role"); got != "assistant" {
		t.Fatalf("last retained role = %q, want assistant", got)
	}
	if got := firstStringFromMap(messages[1], "content"); got != "执行与验证均通过。" {
		t.Fatalf("last retained content = %q", got)
	}
	if got := firstStringFromMap(messages[1], "logical_message_id"); got != "result-1" {
		t.Fatalf("last retained logical_message_id = %q", got)
	}
	if _, ok := messages[1]["message_data"]; ok {
		t.Fatal("chat transport response retained large message_data")
	}
	planMessages := dictionaryRowsFromAny(resp.AgentPlan.ProjectHistory["conversation_messages"])
	if len(planMessages) != 2 {
		t.Fatalf("agent plan did not retain the two-message tail: %+v", planMessages)
	}
	if len(data) >= 1<<20 {
		t.Fatalf("chat transport response is unexpectedly large: %d bytes", len(data))
	}
	if len(resp.ExecutedKernelReply) != 1 || firstStringFromMap(resp.ExecutedKernelReply[0], "tool") != "project.audio_analysis_status" {
		t.Fatalf("tool receipt was not preserved: %+v", resp.ExecutedKernelReply)
	}
	if firstStringFromMap(mapValue(resp.ExecutedKernelReply[0]["result"]), "status") != "ok" {
		t.Fatalf("tool result was not preserved: %+v", resp.ExecutedKernelReply[0])
	}
}

func TestStripSilenceInvokeResponseTransportCompactsResult(t *testing.T) {
	region := map[string]any{
		"clip_id":       "clip_a",
		"track_id":      "track_a",
		"start_seconds": 0.0,
		"end_seconds":   0.25,
	}
	resp := harness.InvokeResponse{
		Status:      "ok",
		Tool:        "clip.strip_silence.suggest",
		CommandName: "clip.strip_silence.suggest",
		Result: map[string]any{
			"status":         "ok",
			"schema_version": "clip.strip_silence.suggest.v0",
			"pending_actions": []any{map[string]any{
				"tool_name": "clip.strip_silence.apply",
				"args": map[string]any{
					"clip_id":       "clip_a",
					"track_id":      "track_a",
					"analysis_id":   "analysis_a",
					"strip_regions": []any{region},
				},
			}},
		},
	}
	compactResp := compactStripSilenceInvokeResponseForTransport(resp)
	data, err := json.Marshal(compactResp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"strip_regions"`) {
		t.Fatalf("invoke response leaked raw strip_regions: %s", string(data))
	}
	if !strings.Contains(string(data), `"pending_action_count"`) {
		t.Fatalf("invoke response missing pending_action_count: %s", string(data))
	}
}

func testStripSilenceBatchDecisions() []policy.Decision {
	region := map[string]any{
		"clip_id":       "clip_a",
		"track_id":      "track_a",
		"start_seconds": 0.0,
		"end_seconds":   0.25,
	}
	return []policy.Decision{{
		Name: "clip.strip_silence.apply_batch",
		Risk: policy.RiskConfirm,
		Command: map[string]any{
			"cmd": "clip.strip_silence.apply_batch",
			"pending_actions": []any{map[string]any{
				"tool_name": "clip.strip_silence.apply",
				"args": map[string]any{
					"clip_id":       "clip_a",
					"track_id":      "track_a",
					"analysis_id":   "analysis_a",
					"strip_regions": []any{region},
				},
			}},
		},
	}}
}
