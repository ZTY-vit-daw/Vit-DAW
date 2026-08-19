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
	"vit-daw-agent/internal/shadow"
)

type recordingToolExecutor struct {
	calls []executorpkg.Input
}

func (fake *recordingToolExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	fake.calls = append(fake.calls, in)
	return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, CommandName: "base",
		Status: "ok", Result: map[string]any{"status": "ok"}}, nil
}

func TestPluginGrabberWorkflowExecutorBridgesCompressorInspectAndApply(t *testing.T) {
	kernel := newFakeCompressorKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = kernel
	base := &recordingToolExecutor{}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}

	inspect, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "inspect", Tool: pluginGrabberInspectCompressorTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "comp-1"},
	}, Confirmed: true})
	if err != nil || inspect.Status != "ok" || inspect.CommandName != pluginGrabberInspectCompressorCommand {
		t.Fatalf("inspect result=%#v err=%v", inspect, err)
	}
	thresholdRef := compressorTestControlRef(inspect.Result, "threshold")
	ratioRef := compressorTestControlRef(inspect.Result, "ratio")
	if thresholdRef == "" || ratioRef == "" {
		t.Fatalf("inspect omitted generation-scoped refs: %#v", inspect.Result)
	}

	apply, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "apply", Tool: pluginGrabberApplyCompressorTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "comp-1", "atomic": true, "controls": []map[string]any{
			{"control_ref": thresholdRef, "value_db": -15.0},
			{"control_ref": ratioRef, "ratio": 4.0},
		}},
	}, Confirmed: true})
	if err != nil || apply.Status != "ok" || apply.CommandName != pluginGrabberApplyCompressorCommand {
		t.Fatalf("apply result=%#v err=%v", apply, err)
	}
	if status := firstNonEmptyText(apply.Result, "status"); status != "exact" && status != "quantized" {
		t.Fatalf("apply status=%q result=%#v", status, apply.Result)
	}
	for _, control := range mapRowsValue(apply.Result["controls"]) {
		if len(mapRowsValue(control["actual_readback"])) == 0 {
			t.Fatalf("typed readback missing: %#v", apply.Result)
		}
	}
	if len(base.calls) != 0 {
		t.Fatalf("typed compressor calls escaped to base executor: %#v", base.calls)
	}
	if len(kernel.batchCalls) != 1 || len(kernel.batchCalls[0]) != 2 {
		t.Fatalf("compressor writes were not one atomic batch: %#v", kernel.batchCalls)
	}
}

func TestPluginGrabberWorkflowExecutorBridgesDawInvokeWrappedCompressorCall(t *testing.T) {
	kernel := newFakeCompressorKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = kernel
	base := &recordingToolExecutor{}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}
	out, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "wrapped", Tool: "daw.invoke", Args: map[string]any{
			"cmd": pluginGrabberInspectCompressorCommand, "track_id": "track-1", "plugin_id": "comp-1",
		},
	}})
	if err != nil || out.Status != "ok" || out.Tool != pluginGrabberInspectCompressorTool || compressorTestControlRef(out.Result, "threshold") == "" {
		t.Fatalf("wrapped inspect result=%#v err=%v", out, err)
	}
	if len(base.calls) != 0 {
		t.Fatalf("wrapped typed compressor call escaped to base: %#v", base.calls)
	}
}

func TestPluginGrabberWorkflowExecutorRejectsBareCompressorRefsWithoutMutation(t *testing.T) {
	kernel := newFakeCompressorKernel()
	before := kernel.snapshot()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = kernel
	executor := pluginGrabberWorkflowExecutor{server: server, base: &recordingToolExecutor{}}

	out, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "apply", Tool: pluginGrabberApplyCompressorTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "comp-1", "atomic": true,
			"controls": []map[string]any{{"control_ref": "1", "value_db": -12.0}}},
	}, Confirmed: true})
	if err == nil || out.Status != "error" || firstNonEmptyText(out.Result, "rejection_code") != "invalid_control_ref" {
		t.Fatalf("bare ref result=%#v err=%v", out, err)
	}
	if len(kernel.batchCalls) != 0 || !fakeSnapshotsEqual(before, kernel.snapshot()) {
		t.Fatalf("bare ref mutated compressor: before=%v after=%v calls=%#v", before, kernel.snapshot(), kernel.batchCalls)
	}
}

func TestPluginGrabberWorkflowExecutorBlocksGenericWritesOwnedByCompressor(t *testing.T) {
	kernel := newFakeCompressorKernel()
	before := kernel.snapshot()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = kernel
	base := &recordingToolExecutor{}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}

	out, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "fallback", Tool: "plugin.set_parameter",
		Args: map[string]any{"track_id": "track-1", "plugin_id": "comp-1", "param_id": "threshold", "value": 0.8},
	}})
	if err != nil || out.Status != "error" || firstNonEmptyText(out.Result, "rejection_code") != "typed_compressor_control_required" {
		t.Fatalf("fallback result=%#v err=%v", out, err)
	}
	if len(base.calls) != 0 || len(kernel.batchCalls) != 0 || !fakeSnapshotsEqual(before, kernel.snapshot()) {
		t.Fatalf("owned generic write escaped guard: base=%d batches=%#v before=%v after=%v", len(base.calls), kernel.batchCalls, before, kernel.snapshot())
	}

	allowed, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "unowned", Tool: "plugin.set_parameter",
		Args: map[string]any{"track_id": "track-1", "plugin_id": "comp-1", "param_id": "non_topology_aux", "value": 0.2},
	}})
	if err != nil || allowed.Status != "ok" || len(base.calls) != 1 {
		t.Fatalf("unowned generic parameter should remain available: result=%#v err=%v base=%d", allowed, err, len(base.calls))
	}
}

func TestInvokeHTTPBlocksGenericWritesOwnedByCompressor(t *testing.T) {
	kernel := newFakeCompressorKernel()
	before := kernel.snapshot()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = kernel
	body, err := json.Marshal(map[string]any{"tool": "plugin.set_parameter", "args": map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "param_id": "threshold", "value": 0.1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/agent/invoke", bytes.NewReader(body))
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response harness.InvokeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "error" || firstNonEmptyText(response.Result, "rejection_code") != "typed_compressor_control_required" {
		t.Fatalf("response=%#v", response)
	}
	if len(kernel.batchCalls) != 0 || !fakeSnapshotsEqual(before, kernel.snapshot()) {
		t.Fatalf("HTTP fallback guard allowed mutation: before=%v after=%v calls=%#v", before, kernel.snapshot(), kernel.batchCalls)
	}
}

func compressorTestControlRef(summary map[string]any, role string) string {
	stage := mapValue(summary["compressor_stage"])
	for _, path := range mapRowsValue(stage["control_paths"]) {
		for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
			for _, binding := range mapRowsValue(path[section]) {
				if firstNonEmptyText(binding, "role") == role {
					return firstNonEmptyText(binding, "control_ref")
				}
			}
		}
	}
	return ""
}
