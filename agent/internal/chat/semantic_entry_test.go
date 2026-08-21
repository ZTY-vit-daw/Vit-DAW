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

func TestDecodeSemanticEntryDecisionRejectsFamilyAndUnknownFields(t *testing.T) {
	available := []string{semanticEntryScopeNone, semanticEntryScopeCurrentSelection, semanticEntryScopeProjectContext}
	valid := `{"schema_version":"semantic_entry_decision.v1","route":"open_semantic","target_scope":"current_selection","control_mode":"semantic_loop","user_authorization":"action_requested","confidence":0.94,"reason":"open acoustic outcome"}`
	decision, err := decodeSemanticEntryDecision(valid, available)
	if err != nil || decision.Route != semanticEntryRouteOpenSemantic {
		t.Fatalf("valid decision rejected: %+v err=%v", decision, err)
	}
	withFamily := strings.TrimSuffix(valid, "}") + `,"processor_family":"eq"}`
	if _, err := decodeSemanticEntryDecision(withFamily, available); err == nil {
		t.Fatal("family field was accepted by semantic entry protocol")
	}
}

func TestDecodeSemanticEntryDecisionValidatesScopeRouteAndAuthorization(t *testing.T) {
	available := []string{semanticEntryScopeNone, semanticEntryScopeProjectContext}
	base := `{"schema_version":"semantic_entry_decision.v1","route":"open_semantic","target_scope":"current_selection","control_mode":"semantic_loop","user_authorization":"action_requested","confidence":0.94,"reason":"open acoustic outcome"}`
	if _, err := decodeSemanticEntryDecision(base, available); err == nil || !strings.Contains(err.Error(), "target_scope") {
		t.Fatalf("unavailable current selection was not rejected: %v", err)
	}
	wrongMode := strings.Replace(base, `"target_scope":"current_selection"`, `"target_scope":"project_context"`, 1)
	wrongMode = strings.Replace(wrongMode, `"control_mode":"semantic_loop"`, `"control_mode":"typed_control"`, 1)
	if _, err := decodeSemanticEntryDecision(wrongMode, []string{semanticEntryScopeNone, semanticEntryScopeProjectContext}); err == nil {
		t.Fatal("route/control mismatch was accepted")
	}
	unresolved := `{"schema_version":"semantic_entry_decision.v1","route":"unresolved","target_scope":"none","control_mode":"none","user_authorization":"none","confidence":0.2,"reason":"ambiguous","rejection_reason":"target is unclear"}`
	if _, err := decodeSemanticEntryDecision(unresolved, available); err != nil {
		t.Fatalf("valid unresolved decision rejected: %v", err)
	}
}

func TestSemanticEntryPlannerDoesNotInjectFamilyOrObservationHints(t *testing.T) {
	var requestBody map[string]any
	serverHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &requestBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":\"semantic_entry_decision.v1\",\"route\":\"open_semantic\",\"target_scope\":\"current_selection\",\"control_mode\":\"semantic_loop\",\"user_authorization\":\"action_requested\",\"confidence\":0.9,\"reason\":\"acoustic outcome\"}"}}]}`))
	}))
	defer serverHTTP.Close()
	server := &Server{llm: &llm.Client{HTTPClient: serverHTTP.Client()}}
	decision, err := server.planSemanticEntry(context.Background(), "conversation-1", "make the vocal clearer", map[string]any{"selected_track_id": "track-1"}, config.EngineConfig{BaseURL: serverHTTP.URL + "/v1", APIKey: "test", DefaultModel: "test"})
	if err != nil {
		t.Fatalf("planSemanticEntry: %v", err)
	}
	if decision.Route != semanticEntryRouteOpenSemantic {
		t.Fatalf("route = %q", decision.Route)
	}
	prompt := stringValue(requestBody["messages"])
	for _, forbidden := range []string{"processor_type", "requested_view_ids", "plugin_path", "parameter_id", "vendor"} {
		if strings.Contains(prompt, `"`+forbidden+`"`) {
			t.Fatalf("entry prompt injected forbidden field %q: %s", forbidden, prompt)
		}
	}
}

func TestSemanticEntryNeverSelectsControllerForNaturalOrExplicitWorkflow(t *testing.T) {
	serverHTTP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema_version\":\"semantic_entry_decision.v1\",\"route\":\"open_semantic\",\"target_scope\":\"project_context\",\"control_mode\":\"semantic_loop\",\"user_authorization\":\"action_requested\",\"confidence\":0.94,\"reason\":\"overall project mix\"}"}}]}`))
	}))
	defer serverHTTP.Close()
	server := &Server{llm: &llm.Client{HTTPClient: serverHTTP.Client()}}
	cfg := config.EngineConfig{BaseURL: serverHTTP.URL + "/v1", APIKey: "test", DefaultModel: "test"}

	natural, err := server.planSemanticEntry(context.Background(), "conversation-natural",
		"检查完整工程的整体混音，并处理有足够证据的问题", nil, cfg)
	if err != nil || natural.Controller != "" {
		t.Fatalf("semantic entry selected a controller before observation: decision=%+v err=%v", natural, err)
	}
	explicit, err := server.planSemanticEntry(context.Background(), "conversation-fixed",
		"请运行 ProjectMix 固定工作流", nil, cfg)
	if err != nil || explicit.Controller != "" {
		t.Fatalf("semantic entry selected a controller for an explicit workflow request: decision=%+v err=%v", explicit, err)
	}
}

func TestSemanticEntryRejectsModelSelectedController(t *testing.T) {
	withController := `{"schema_version":"semantic_entry_decision.v1","route":"open_semantic","controller":"project_mix_workflow","target_scope":"project_context","control_mode":"semantic_loop","user_authorization":"action_requested","confidence":0.9,"reason":"model chose workflow"}`
	if _, err := decodeSemanticEntryDecision(withController, []string{semanticEntryScopeNone, semanticEntryScopeProjectContext}); err == nil || !strings.Contains(err.Error(), "controller") {
		t.Fatalf("model-selected controller was accepted: %v", err)
	}
}

func TestSemanticEntryPromptKeepsDeferredImprovementProposalInOpenSemanticRoute(t *testing.T) {
	prompt := semanticEntrySystemPrompt()
	for _, required := range []string{
		"bounded, reversible improvement proposal",
		"open_semantic with action_requested",
		"admission is not a project mutation",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("semantic entry prompt lost deferred-improvement rule %q", required)
		}
	}
}

func TestBlindProjectSmokeSemanticEntryFailureIsFailClosed(t *testing.T) {
	if !blindProjectSmokeContext(map[string]any{"blind_experiment": true, "interaction_path": "blind_project_smoke"}) {
		t.Fatal("blind project smoke context was not recognized")
	}
	if blindProjectSmokeContext(map[string]any{"blind_experiment": true, "interaction_path": "ordinary_chat"}) {
		t.Fatal("ordinary chat was treated as blind project smoke")
	}
	response := semanticEntryServiceFailureResponse("smoke-case", "ordinary_agent", context.DeadlineExceeded)
	if response.StopReason != "transient_llm_error" || response.GoalStatus != "waiting_continue" {
		t.Fatalf("unexpected fail-closed response: %+v", response)
	}
	if response.Error == "" || response.Workflow != "semantic_entry" {
		t.Fatalf("missing service failure evidence: %+v", response)
	}
}

func stringValue(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
