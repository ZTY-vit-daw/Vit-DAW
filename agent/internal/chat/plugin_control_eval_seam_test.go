package chat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/shadow"
)

// PLUGIN-CONTROL-EVAL-SEAM-1 S1 (typed-takeover bypass), B4/C4 ablation-hook
// test pattern: the fixture decision is the same model-classified
// explicit_control answer in every leg — only the env differs — so the pair
// pins the exact contract: default unset keeps the production typed takeover
// (controller=direct_typed_action, the P-baseline T2/T3/T4 first-action
// behavior); VIT_PLUGIN_CONTROL_EVAL_SEAM=1 reclassifies the control request
// to route=other / control_mode=ordinary_agent so capacity routing assigns
// the ordinary conversation controller (generic tool loop, no typed planner),
// with write authorization preserved so the ordinary loop's mutation barrier
// stays open. Only the exact value "1" arms the hook.

func seamS1ExplicitControlDecision() semanticEntryDecision {
	return semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteExplicitControl,
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlTyped,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.95, Reason: "user asks for a concrete control move",
	}
}

func TestSeamS1EntryRewriteClosedKeepsDecisionIdentical(t *testing.T) {
	original := seamS1ExplicitControlDecision()
	got := applyPluginControlEvalSeamEntryRewrite(original)
	if !reflect.DeepEqual(original, got) {
		t.Fatalf("closed seam must return the decision unchanged: original=%+v got=%+v", original, got)
	}
	for _, route := range []string{semanticEntryRouteDiscussion, semanticEntryRouteObservation, semanticEntryRouteOpenSemantic, semanticEntryRouteOther} {
		decision := seamS1ExplicitControlDecision()
		decision.Route = route
		if got := applyPluginControlEvalSeamEntryRewrite(decision); !reflect.DeepEqual(decision, got) {
			t.Fatalf("closed seam must not touch route %s: %+v", route, got)
		}
	}
}

func TestSeamS1EntryRewriteOpenDivertsOnlyExplicitControl(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	got := applyPluginControlEvalSeamEntryRewrite(seamS1ExplicitControlDecision())
	if got.Route != semanticEntryRouteOther || got.ControlMode != semanticEntryControlOrdinary {
		t.Fatalf("open seam must reclassify explicit_control to other/ordinary_agent: %+v", got)
	}
	if got.UserAuthorization != semanticEntryAuthorizationAction || got.TargetScope != semanticEntryScopeProjectContext || got.Confidence != 0.95 {
		t.Fatalf("open seam must preserve authorization/scope/confidence: %+v", got)
	}
	if got.Reason == "user asks for a concrete control move" {
		t.Fatalf("open seam must rewrite the reason for auditability: %+v", got)
	}
	for _, route := range []string{semanticEntryRouteDiscussion, semanticEntryRouteObservation, semanticEntryRouteOpenSemantic} {
		decision := seamS1ExplicitControlDecision()
		decision.Route = route
		if got := applyPluginControlEvalSeamEntryRewrite(decision); !reflect.DeepEqual(decision, got) {
			t.Fatalf("open seam must not touch route %s: %+v", route, got)
		}
	}
}

func TestSeamS1OnlyExactOneArmsTheRewrite(t *testing.T) {
	for _, value := range []string{"", "0", "true", "off"} {
		t.Setenv(pluginControlEvalSeamEnv, value)
		if pluginControlEvalSeamOpen() {
			t.Fatalf("env value %q must not open the seam", value)
		}
		decision := seamS1ExplicitControlDecision()
		if got := applyPluginControlEvalSeamEntryRewrite(decision); !reflect.DeepEqual(decision, got) {
			t.Fatalf("env value %q must keep the typed route: %+v", value, got)
		}
	}
}

// seamS1ClassificationSequence replays the production classification block
// (runAgentLoopChat goalrunner_chat.go: semantic entry → seam rewrite →
// observation-first capability routing) with a model double that always
// classifies the request as explicit_control.
func seamS1ClassificationSequence(t *testing.T, server *Server, cfg config.EngineConfig, request string) semanticEntryDecision {
	t.Helper()
	chatContext, identity := server.ensureCapabilityRoutingTask(request, nil)
	decision, err := server.planSemanticEntry(context.Background(), "conversation-seam-s1", request, chatContext, cfg)
	if err != nil {
		t.Fatalf("semantic entry classification failed: %v", err)
	}
	if decision.Route != semanticEntryRouteExplicitControl {
		t.Fatalf("fixture model must classify as explicit_control: %+v", decision)
	}
	decision = applyPluginControlEvalSeamEntryRewrite(decision)
	entry, _, err := server.planObservationFirstCapabilityRoute(context.Background(), "conversation-seam-s1", request, chatContext, decision, identity)
	if err != nil {
		t.Fatalf("capability routing failed: %v", err)
	}
	return entry
}

func seamS1Server(t *testing.T) (*Server, config.EngineConfig) {
	t.Helper()
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &payload)
		answer := `{"schema_version":"semantic_entry_decision.v1","route":"explicit_control","target_scope":"project_context","control_mode":"typed_control","user_authorization":"action_requested","confidence":0.95,"reason":"user asks for a concrete control move"}`
		if len(payload.Messages) > 0 && payload.Messages[len(payload.Messages)-1].Role == "user" &&
			payload.Messages[len(payload.Messages)-1].Content != "" && len(payload.Messages) > 2 {
			answer = `{"schema_version":"semantic_entry_decision.v1","route":"other","target_scope":"project_context","control_mode":"ordinary_agent","user_authorization":"action_requested","confidence":0.9,"reason":"ordinary request"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":` + jsonStringForTest(t, answer) + `}}]}`))
	}))
	t.Cleanup(model.Close)
	project := shadow.New(nil)
	project.Initialize(capacityTestProject(3, 60, 1, 0, "rev-seam-s1"))
	server := New(nil, project, nil)
	server.llm = &llm.Client{HTTPClient: model.Client()}
	cfg := config.EngineConfig{BaseURL: model.URL + "/v1", APIKey: "test", DefaultModel: "test"}
	return server, cfg
}

func jsonStringForTest(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal answer: %v", err)
	}
	return string(encoded)
}

// A-arm condition proof, S1 face: seam open → the control-class request is
// NOT taken over by the typed planner.
func TestSeamS1OpenControlRequestSkipsTypedTakeover(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	server, cfg := seamS1Server(t)
	entry := seamS1ClassificationSequence(t, server, cfg, "把 Pro-R 的 Mix 参数调到 35%")
	if entry.Controller != string(orchestrationcontroller.OrdinaryConversation) {
		t.Fatalf("open seam must route control requests to the ordinary agent loop, got controller=%s", entry.Controller)
	}
	if entry.Route != semanticEntryRouteOther || entry.ControlMode != semanticEntryControlOrdinary {
		t.Fatalf("open seam entry must carry the ordinary-agent contract: %+v", entry)
	}
	if entry.UserAuthorization != semanticEntryAuthorizationAction {
		t.Fatalf("open seam must preserve action authorization for the ordinary loop: %+v", entry)
	}
}

// Closed seam pins the P-baseline typed takeover (controller=direct_typed_action).
func TestSeamS1ClosedKeepsTypedTakeover(t *testing.T) {
	server, cfg := seamS1Server(t)
	entry := seamS1ClassificationSequence(t, server, cfg, "把 Pro-R 的 Mix 参数调到 35%")
	if entry.Controller != string(orchestrationcontroller.DirectTypedAction) {
		t.Fatalf("closed seam must keep the typed takeover, got controller=%s", entry.Controller)
	}
}
