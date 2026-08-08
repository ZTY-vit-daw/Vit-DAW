package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/shadow"
)

type fakeLimiterKernel struct {
	params      map[string]float64
	batchCalls  [][]map[string]any
	mutateOther bool
	failBatchAt map[int]bool
}

func newFakeLimiterKernel() *fakeLimiterKernel {
	return &fakeLimiterKernel{params: map[string]float64{"threshold": .5, "ceiling": .75, "release": .5, "unplanned": .1}, failBatchAt: map[int]bool{}}
}

func (fake *fakeLimiterKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	rows := []map[string]any{
		fake.parameter("threshold", "Threshold", []string{"-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"}),
		fake.parameter("ceiling", "Ceiling", []string{"-12 dB", "-9 dB", "-6 dB", "-3 dB", "0 dB"}),
		fake.parameter("release", "Release", []string{"1 ms", "10 ms", "100 ms", "500 ms", "1000 ms"}),
		fake.parameter("unplanned", "Meter State", []string{"0", ".25", ".5", ".75", "1"}),
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "limit-1", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeLimiterKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	copied := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, errors.New("invalid fake limiter write")
		}
		fake.params[id] = value
		copied = append(copied, map[string]any{"parameter_id": id, "normalized_value": value})
	}
	fake.batchCalls = append(fake.batchCalls, copied)
	if fake.mutateOther && len(fake.batchCalls) == 1 {
		fake.params["unplanned"] = .9
	}
	if fake.failBatchAt[len(fake.batchCalls)] {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected limiter failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeLimiterKernel) parameter(id, name string, labels []string) map[string]any {
	normalized := fake.params[id]
	current := labels[int(math.Round(normalized*float64(len(labels)-1)))]
	samples := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		samples = append(samples, map[string]any{"normalized_value": float64(i) / float64(len(labels)-1), "text": label})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true, "normalized_value": normalized, "value_text": current, "display_probe": map[string]any{"mode": "samples", "label": name, "samples": samples}}
}

func (fake *fakeLimiterKernel) snapshot() map[string]float64 {
	out := map[string]float64{}
	for k, v := range fake.params {
		out[k] = v
	}
	return out
}

func limiterTestControlRef(summary map[string]any, role string) string {
	for _, stage := range mapRowsValue(summary["limiter_stages"]) {
		for _, section := range []string{"operating_point", "safety", "timing", "detector", "mode", "output"} {
			for _, binding := range mapRowsValue(stage[section]) {
				if firstNonEmptyText(binding, "role") == role {
					return firstNonEmptyText(binding, "control_ref")
				}
			}
		}
	}
	return ""
}

func TestApplyLimiterControlsRestoresFullPreimageOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sideEffect bool
		fail       map[int]bool
	}{
		{name: "side effect", sideEffect: true}, {name: "batch failure", fail: map[int]bool{1: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeLimiterKernel()
			fake.mutateOther, fake.failBatchAt = tc.sideEffect, tc.fail
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			_, summary, err := server.readLiveLimiterControlSurface(context.Background(), "track-1", "limit-1")
			if err != nil {
				t.Fatal(err)
			}
			ref := limiterTestControlRef(summary, "threshold")
			before := fake.snapshot()
			result, err := server.applyPluginGrabberLimiterControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "limit-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}}, nil)
			if err == nil || (result != nil && firstNonEmptyText(result, "status") != "rejected") {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.sideEffect && !strings.Contains(err.Error(), "unplanned_parameter_change") {
				t.Fatalf("err=%v", err)
			}
			if !fakeSnapshotsEqual(before, fake.snapshot()) {
				t.Fatalf("full preimage not restored before=%v after=%v", before, fake.snapshot())
			}
			if len(fake.batchCalls) != 2 {
				t.Fatalf("batch calls=%d", len(fake.batchCalls))
			}
		})
	}
}

func TestApplyLimiterControlsPublishesIndependentRestoreRef(t *testing.T) {
	fake := newFakeLimiterKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveLimiterControlSurface(context.Background(), "track-1", "limit-1")
	if err != nil {
		t.Fatal(err)
	}
	ref := limiterTestControlRef(summary, "ceiling")
	before := fake.snapshot()
	applied, err := server.applyPluginGrabberLimiterControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "limit-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "value_db": -9.0}}}, nil)
	if err != nil {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	restore := firstNonEmptyText(applied, "restore_ref")
	if !strings.HasPrefix(restore, "l1rr1_") {
		t.Fatalf("restore=%q", restore)
	}
	if _, err := decodeCompressorRestoreRef(restore); err == nil {
		t.Fatal("limiter restore ref decoded as compressor restore ref")
	}
	restored, err := server.applyPluginGrabberLimiterControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "limit-1", "atomic": true, "restore_ref": restore}, nil)
	if err != nil || restored["restored"] != true {
		t.Fatalf("restore=%+v err=%v", restored, err)
	}
	if !fakeSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("restore drift before=%v after=%v", before, fake.snapshot())
	}
}

func TestGenericWriteGuardProtectsLimiterButNotAuxiliary(t *testing.T) {
	fake := newFakeLimiterKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	owned, blocked := server.guardLimiterOwnedGenericParameterWrite(context.Background(), harnessInvokeRequestForLimiterParam("threshold"))
	if !blocked || firstNonEmptyText(owned.Result, "rejection_code") != "typed_limiter_control_required" {
		t.Fatalf("owned=%+v blocked=%v", owned, blocked)
	}
	_, blocked = server.guardLimiterOwnedGenericParameterWrite(context.Background(), harnessInvokeRequestForLimiterParam("unplanned"))
	if blocked {
		t.Fatal("unowned auxiliary parameter was blocked")
	}
}

func TestPluginGrabberWorkflowExecutorBridgesLimiterInspectAndApply(t *testing.T) {
	fake := newFakeLimiterKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	base := &recordingToolExecutor{}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}
	inspect, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "inspect", Tool: pluginGrabberInspectLimiterTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "limit-1"},
	}})
	if err != nil || inspect.Status != "ok" || limiterTestControlRef(inspect.Result, "threshold") == "" {
		t.Fatalf("inspect=%#v err=%v", inspect, err)
	}
	ref := limiterTestControlRef(inspect.Result, "threshold")
	apply, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{
		ID: "apply", Tool: pluginGrabberApplyLimiterTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "limit-1", "atomic": true,
			"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}},
	}})
	if err != nil || apply.Status != "ok" || firstNonEmptyText(apply.Result, "status") != "exact" {
		t.Fatalf("apply=%#v err=%v", apply, err)
	}
	if len(base.calls) != 0 || len(fake.batchCalls) != 1 {
		t.Fatalf("limiter typed call escaped or was not atomic: base=%d batches=%#v", len(base.calls), fake.batchCalls)
	}
}

func harnessInvokeRequestForLimiterParam(paramID string) harness.InvokeRequest {
	return harness.InvokeRequest{Tool: "plugin.set_parameter", Args: map[string]any{"track_id": "track-1", "plugin_id": "limit-1", "param_id": paramID, "value": .2}}
}
