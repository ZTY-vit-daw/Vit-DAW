package chat

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
)

func activeFreeStateInvokeContext() map[string]any {
	return map[string]any{"free_state_reasoning_loop": map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "make it stable",
	}}
}

func TestFreeStateHTTPInvokeGuardBlocksTypedApplyAndLoad(t *testing.T) {
	for _, req := range []harness.InvokeRequest{
		{Tool: "plugin_grabber.apply_eq_edits", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.set_eq_point", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_compressor_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_limiter_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_gate_expander_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_de_esser_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_transient_shaper_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.apply_multiband_controls", Context: activeFreeStateInvokeContext()},
		{Tool: "rack.add_node", Context: activeFreeStateInvokeContext()},
		{Tool: "plugin.set_parameter", Context: activeFreeStateInvokeContext()},
		{Tool: "daw.invoke", Args: map[string]any{"cmd": "set_plugin_param"}, Context: activeFreeStateInvokeContext()},
		{Tool: "ccb.observation_request", Args: map[string]any{"payload": map[string]any{"command": "plugin.set_parameter"}}, Context: activeFreeStateInvokeContext()},
	} {
		if reason := freeStateInvokeMutationReason(req); !strings.Contains(reason, "active free-state") {
			t.Fatalf("request %+v was not blocked: %q", req, reason)
		}
	}
}

func TestFreeStateHTTPInvokeGuardAllowsReadOnlyObservation(t *testing.T) {
	for _, req := range []harness.InvokeRequest{
		{Tool: "ccb.observation_catalog", Context: activeFreeStateInvokeContext()},
		{Tool: "ccb.observation_request", Args: map[string]any{"view_ids": []any{"track.peak_structure"}}, Context: activeFreeStateInvokeContext()},
		{Tool: "plugin_grabber.inspect_limiter", Context: activeFreeStateInvokeContext()},
	} {
		if reason := freeStateInvokeMutationReason(req); reason != "" {
			t.Fatalf("read-only request %+v was blocked: %q", req, reason)
		}
	}
}

func TestFreeStateHTTPInvokeGuardDoesNotPersistAfterLoopEnds(t *testing.T) {
	req := harness.InvokeRequest{Tool: "plugin.set_parameter", Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{"schema_version": "free_state_reasoning_loop.v1", "status": "completed", "original_intent": "make it stable"},
	}}
	if reason := freeStateInvokeMutationReason(req); reason != "" {
		t.Fatalf("completed loop retained mutation barrier: %q", reason)
	}
}

func TestInvokeHTTPAppliesFreeStateGuardBeforeTypedWorkflowDispatch(t *testing.T) {
	server := New(nil, nil, nil)
	body, err := json.Marshal(map[string]any{
		"tool":    "plugin_grabber.apply_limiter_controls",
		"context": activeFreeStateInvokeContext(),
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/agent/invoke", bytes.NewReader(body)))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "free_state_mutation_forbidden") {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestInvokeHTTPRequiresConfirmationBeforeTypedWorkflowDispatch(t *testing.T) {
	server := New(nil, nil, nil)
	for _, tool := range []string{
		"plugin_grabber.apply_eq_edits",
		"plugin_grabber.set_eq_point",
		"plugin_grabber.apply_compressor_controls",
		"plugin_grabber.apply_limiter_controls",
		"plugin_grabber.apply_gate_expander_controls",
		"plugin_grabber.apply_de_esser_controls",
		"plugin_grabber.apply_transient_shaper_controls",
		"plugin_grabber.apply_multiband_controls",
	} {
		body, err := json.Marshal(map[string]any{"tool": tool})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		server.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/agent/invoke", bytes.NewReader(body)))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "needs_confirmation") || strings.Contains(recorder.Body.String(), "target_unavailable") {
			t.Fatalf("tool=%s status=%d body=%s", tool, recorder.Code, recorder.Body.String())
		}
	}
}

func TestWorkflowExecutorAppliesFreeStateGuardBeforeTypedDispatch(t *testing.T) {
	server := New(nil, nil, nil)
	executor := pluginGrabberWorkflowExecutor{server: server}
	result, err := executor.RunToolCall(nil, executorInputForFreeStateGuard(server, "plugin_grabber.apply_de_esser_controls"))
	if err != nil || result.Status != "error" || result.CommandName != "free_state_mutation_forbidden" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestWorkflowExecutorRequiresConfirmationBeforeTypedDispatch(t *testing.T) {
	server := New(nil, nil, nil)
	workflowExecutor := pluginGrabberWorkflowExecutor{server: server}
	result, err := workflowExecutor.RunToolCall(nil, executor.Input{ToolCall: planner.ToolCall{Tool: "plugin_grabber.apply_limiter_controls"}})
	if err != nil || result.Status != "needs_confirmation" || !result.RequiresConfirmation || result.Result["mutation_performed"] != false {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func executorInputForFreeStateGuard(_ *Server, tool string) executor.Input {
	return executor.Input{ToolCall: planner.ToolCall{Tool: tool}, Context: activeFreeStateInvokeContext()}
}
