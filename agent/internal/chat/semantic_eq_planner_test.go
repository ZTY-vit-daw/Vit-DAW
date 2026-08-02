package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
)

func semanticEQPlannerTestAction(trackID, pluginID string) string {
	return `{
		"schema_version":"semantic_effect_action.v1",
		"action_type":"eq_edit",
		"payload_schema":"semantic_effect.eq_plan.v1",
		"target":{"track_id":"` + trackID + `","plugin_id":"` + pluginID + `"},
		"user_goal":"减少一些浑浊",
		"evidence_decision":{"choice":"not_needed","basis":"user_report","reason":"用户报告足以支持保守静态 EQ 方案"},
		"eq_plan":{"schema_version":"semantic_effect.eq_plan.v1","atomic":true,"atoms":[{
			"atom_id":"mud_control","action":"upsert","shape":"bell","frequency_hz":280,"gain_db":-1.5,"q":1.1,
			"purpose":"减少低中频浑浊","field_origins":{"frequency_hz":"llm_selected","gain_db":"llm_selected","q":"llm_selected"},"confidence":"medium"
		}]}
	}`
}

func semanticEQPlannerTestServer(t *testing.T, responses []string) (*Server, config.EngineConfig, *int, *[]string) {
	t.Helper()
	calls := 0
	bodies := []string{}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		bodies = append(bodies, string(body))
		index := calls
		calls++
		if index >= len(responses) {
			t.Errorf("unexpected planner request %d", calls)
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": responses[index]}}},
		})
	}))
	t.Cleanup(httpServer.Close)
	server := &Server{llm: &llm.Client{HTTPClient: httpServer.Client()}}
	cfg := config.EngineConfig{BaseURL: httpServer.URL + "/v1", APIKey: "test", DefaultModel: "test-model"}
	return server, cfg, &calls, &bodies
}

func semanticEQPlannerTestContext() map[string]any {
	return map[string]any{
		"selected_track_id":  "1007",
		"selected_plugin_id": "1015",
		"generic_eq_topology": map[string]any{
			"generation": "eqt_test",
			"sections":   []any{map[string]any{"section": "1", "reachable_shapes": []any{"bell"}}},
		},
	}
}

func TestPlanOrdinaryAgentSemanticEQReturnsTypedAction(t *testing.T) {
	server, cfg, calls, _ := semanticEQPlannerTestServer(t, []string{semanticEQPlannerTestAction("1007", "1015")})
	action, err := server.planOrdinaryAgentSemanticEQ(context.Background(), "conversation-1", "减少一些浑浊", semanticEQPlannerTestContext(), nil, cfg)
	if err != nil {
		t.Fatalf("planOrdinaryAgentSemanticEQ: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("LLM calls = %d, want 1", *calls)
	}
	if action == nil || action.Target.TrackID != "1007" || action.Target.PluginID != "1015" || action.EQPlan == nil || len(action.EQPlan.Atoms) != 1 {
		t.Fatalf("action = %#v", action)
	}
}

func TestPlanOrdinaryAgentSemanticEQRepairsInvalidJSONOnce(t *testing.T) {
	server, cfg, calls, bodies := semanticEQPlannerTestServer(t, []string{"not JSON", semanticEQPlannerTestAction("1007", "1015")})
	action, err := server.planOrdinaryAgentSemanticEQ(context.Background(), "conversation-1", "减少一些浑浊", semanticEQPlannerTestContext(), nil, cfg)
	if err != nil {
		t.Fatalf("planOrdinaryAgentSemanticEQ: %v", err)
	}
	if action == nil || *calls != 2 {
		t.Fatalf("action=%#v calls=%d, want repaired action after 2 calls", action, *calls)
	}
	if len(*bodies) != 2 || !strings.Contains((*bodies)[1], "previous semantic EQ JSON was invalid") {
		t.Fatalf("repair request missing invalid-output feedback: %#v", *bodies)
	}
}

func TestPlanOrdinaryAgentSemanticEQBindsOmittedDeterministicTarget(t *testing.T) {
	withoutTarget := strings.Replace(
		semanticEQPlannerTestAction("1007", "1015"),
		`"target":{"track_id":"1007","plugin_id":"1015"},`,
		"",
		1,
	)
	server, cfg, calls, _ := semanticEQPlannerTestServer(t, []string{withoutTarget})
	action, err := server.planOrdinaryAgentSemanticEQ(context.Background(), "conversation-1", "提高一些高频", semanticEQPlannerTestContext(), nil, cfg)
	if err != nil {
		t.Fatalf("planOrdinaryAgentSemanticEQ: %v", err)
	}
	if *calls != 1 || action == nil || action.Target.TrackID != "1007" || action.Target.PluginID != "1015" {
		t.Fatalf("action=%#v calls=%d, want deterministic exact target binding", action, *calls)
	}
}

func TestDecodeSemanticEQLLMActionAcceptsAgentWrapperAndBindsUserGoal(t *testing.T) {
	inner := semanticEQPlannerTestAction("1007", "1015")
	inner = strings.Replace(inner, `"user_goal":"减少一些浑浊",`, "", 1)
	inner = strings.Replace(inner, `"evidence_decision":`, `"evidence":`, 1)
	action, err := decodeSemanticEQLLMActionForTarget(`{"semantic_action":`+inner+`}`, "1007", "1015", "提高一些高频")
	if err != nil {
		t.Fatalf("decode wrapped action: %v", err)
	}
	if action.UserGoal != "提高一些高频" || action.Evidence.Choice != "not_needed" || action.Target.TrackID != "1007" || action.Target.PluginID != "1015" || action.EQPlan == nil {
		t.Fatalf("action=%#v", action)
	}
}

func TestPlanOrdinaryAgentSemanticEQBindsOmittedUpsertVerb(t *testing.T) {
	withoutAction := strings.Replace(semanticEQPlannerTestAction("1007", "1015"), `"action":"upsert",`, "", 1)
	server, cfg, calls, _ := semanticEQPlannerTestServer(t, []string{withoutAction})
	action, err := server.planOrdinaryAgentSemanticEQ(context.Background(), "conversation-1", "减少一些浑浊", semanticEQPlannerTestContext(), nil, cfg)
	if err != nil {
		t.Fatalf("planOrdinaryAgentSemanticEQ: %v", err)
	}
	if *calls != 1 || action == nil || action.EQPlan == nil || len(action.EQPlan.Atoms) != 1 || action.EQPlan.Atoms[0].Action != "upsert" {
		t.Fatalf("action=%#v calls=%d, want deterministic upsert binding", action, *calls)
	}
}

func TestDecodeSemanticEQLLMActionDoesNotBindIncompleteAtom(t *testing.T) {
	incomplete := strings.Replace(semanticEQPlannerTestAction("1007", "1015"), `"action":"upsert","shape":"bell","frequency_hz":280,`, "", 1)
	action, err := decodeSemanticEQLLMActionForTarget(incomplete, "1007", "1015", "减少一些浑浊")
	if err == nil {
		t.Fatalf("incomplete atom accepted: %#v", action)
	}
}

func TestPlanOrdinaryAgentSemanticEQRejectsChangedExactTarget(t *testing.T) {
	server, cfg, calls, _ := semanticEQPlannerTestServer(t, []string{
		semanticEQPlannerTestAction("invented-track", "1015"),
		semanticEQPlannerTestAction("1007", "invented-plugin"),
	})
	action, err := server.planOrdinaryAgentSemanticEQ(context.Background(), "conversation-1", "减少一些浑浊", semanticEQPlannerTestContext(), nil, cfg)
	if err == nil || action != nil {
		t.Fatalf("changed exact target accepted: action=%#v err=%v", action, err)
	}
	if *calls != 2 || !strings.Contains(err.Error(), "changed exact target") {
		t.Fatalf("calls=%d err=%v", *calls, err)
	}
}

func TestOrdinaryAgentSemanticEQDiscussionDoesNotTriggerPlanner(t *testing.T) {
	ctx := semanticEQPlannerTestContext()
	if ordinaryAgentSemanticEQActionable("为什么听起来浑", ctx) {
		t.Fatal("discussion-only request was classified as an actionable semantic EQ edit")
	}
	if !ordinaryAgentSemanticEQActionable("减少一些浑浊", ctx) {
		t.Fatal("semantic EQ edit request was not classified as actionable")
	}
}

func TestOrdinaryAgentSemanticEQNoPluginHandoffRequiresConcreteAction(t *testing.T) {
	ctx := map[string]any{"selected_track_id": "1007"}
	if !ordinaryAgentSemanticEQNeedsPluginSelection("帮我把低频降低一些", ctx) {
		t.Fatal("concrete selected-track EQ action did not require plugin selection")
	}
	if ordinaryAgentSemanticEQNeedsPluginSelection("你推荐用哪个 EQ？", ctx) {
		t.Fatal("recommendation-only request was classified as a concrete EQ action")
	}
	ctx["selected_plugin_id"] = "1015"
	if ordinaryAgentSemanticEQNeedsPluginSelection("帮我把低频降低一些", ctx) {
		t.Fatal("loaded selected EQ was incorrectly routed back to plugin selection")
	}
}
