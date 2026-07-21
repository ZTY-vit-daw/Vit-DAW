package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type countingPluginEffectBaseExecutor struct {
	calls int
}

func (e *countingPluginEffectBaseExecutor) RunToolCall(_ context.Context, _ executorpkg.Input) (executorpkg.Result, error) {
	e.calls++
	return executorpkg.Result{Status: "unexpected_base_execution"}, nil
}

func pluginEffectProposalTestResponse(conversationID string, goal agentruntime.Goal) ChatResponse {
	return ChatResponse{
		ConversationID:    conversationID,
		GoalID:            goal.GoalID,
		RunID:             goal.RunID,
		Reply:             "proposal ready",
		NeedsConfirmation: true,
		PlanID:            "proposal_b4_1",
		Preview:           "governed plugin effect proposal",
		Workflow:          "capability_runtime_v1",
		GoalStatus:        string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id":        "session_b4_1",
			"proposal_id":       "proposal_b4_1",
			"proposal_revision": int64(3),
			"action_set_hash":   "action_hash_b4_1",
			"project_cut_hash":  "cut_hash_b4_1",
		},
	}
}

func TestAgentLoopRawPluginApplyIsTakenOverByB4Proposal(t *testing.T) {
	base := &countingPluginEffectBaseExecutor{}
	server := &Server{pluginEffectControlRuntimeOverride: func(_ context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
		apply := pluginEffectApplyArgs(req.Context)
		if cleanContextText(apply["track_id"]) != "1007" || cleanContextText(apply["plugin_id"]) != "1013" || cleanContextText(apply["control"]) != "eq.cut_region" {
			t.Fatalf("B4 apply args = %#v", apply)
		}
		return pluginEffectProposalTestResponse(conversationID, goal)
	}}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}
	out, err := executor.RunToolCall(context.Background(), executorpkg.Input{
		GoalID: "goal_b4", RunID: "run_b4",
		ToolCall: planner.ToolCall{ID: "call_b4", Tool: "plugin_grabber.apply_control", Args: map[string]any{
			"track_id": "1007", "plugin_id": "1013", "control": "eq.cut_region", "target": map[string]any{"freq_hz": 200.0, "gain_db": -2.5, "q": 1.0},
		}},
		Context: map[string]any{"conversation_id": "chat_b4", "user_message": "cut mud"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if base.calls != 0 {
		t.Fatalf("base executor was called %d times", base.calls)
	}
	if out.Status != "needs_confirmation" || !out.RequiresConfirmation || out.CommandName != pluginEffectControlCapabilityID {
		t.Fatalf("unexpected executor result: %#v", out)
	}
	for key, want := range map[string]any{
		"proposal_id": "proposal_b4_1", "proposal_revision": int64(3), "action_set_hash": "action_hash_b4_1", "session_id": "session_b4_1",
	} {
		if got := out.Result[key]; got != want {
			t.Fatalf("workflow %s = %#v, want %#v; result=%#v", key, got, want, out.Result)
		}
	}
}

func TestHTTPRawPluginApplyConfirmedStillOnlyCreatesB4Proposal(t *testing.T) {
	mutations := 0
	server := &Server{pluginEffectControlRuntimeOverride: func(_ context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
		if req.Context["raw_tool_confirmation_ignored"] != true {
			t.Fatalf("confirmed raw call was not marked ignored: %#v", req.Context)
		}
		return pluginEffectProposalTestResponse(conversationID, goal)
	}}
	body, err := json.Marshal(harness.InvokeRequest{
		Tool: "plugin_grabber.apply_control", Confirmed: true, GoalID: "goal_http_b4", RunID: "run_http_b4", ToolCallID: "call_http_b4",
		Args:    map[string]any{"track_id": "1007", "plugin_id": "1013", "control": "eq.cut_region", "target": map[string]any{"freq_hz": 200.0, "gain_db": -2.5, "q": 1.0}},
		Context: map[string]any{"conversation_id": "chat_http_b4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/agent/invoke", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response harness.InvokeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if mutations != 0 {
		t.Fatalf("unexpected mutation count %d", mutations)
	}
	if response.Status != "needs_confirmation" || !response.RequiresConfirmation || response.Result["raw_tool_confirmation_ignored"] != true {
		t.Fatalf("raw confirmation was not ignored: %#v", response)
	}
	for _, key := range []string{"proposal_id", "proposal_revision", "action_set_hash", "session_id"} {
		if response.Result[key] == nil || response.Result[key] == "" {
			t.Fatalf("missing %s in response: %#v", key, response.Result)
		}
	}
}
