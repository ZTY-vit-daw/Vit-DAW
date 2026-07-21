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
