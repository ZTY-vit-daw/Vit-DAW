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

type fakeGateExpanderKernel struct {
	params      map[string]float64
	batchCalls  [][]map[string]any
	mutateOther bool
	failBatchAt map[int]bool
}

func newFakeGateExpanderKernel() *fakeGateExpanderKernel {
	return &fakeGateExpanderKernel{params: map[string]float64{
		"threshold": .5, "range": .75, "attack": .25, "hold": .5, "release": .5,
		"hpf": .25, "lpf": .25, "midi": .1, "channel": .2, "unplanned": .1,
	}, failBatchAt: map[int]bool{}}
}

func (fake *fakeGateExpanderKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	rows := []map[string]any{
		fake.gateParameter("threshold", "Gate Threshold", []string{"-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"}),
		fake.gateParameter("range", "Reduction", []string{"-40 dB", "-30 dB", "-20 dB", "-10 dB", "0 dB"}),
		fake.gateParameter("attack", "Attack", []string{"1 ms", "10 ms", "50 ms", "200 ms", "500 ms"}),
		fake.gateParameter("hold", "Hold", []string{"0 ms", "10 ms", "100 ms", "250 ms", "500 ms"}),
		fake.gateParameter("release", "Release", []string{"1 ms", "10 ms", "100 ms", "500 ms", "2000 ms"}),
		fake.gateParameter("hpf", "SC HPF", []string{"20 Hz", "100 Hz", "1000 Hz", "5000 Hz", "10000 Hz"}),
		fake.gateParameter("lpf", "SC LPF", []string{"1000 Hz", "3000 Hz", "5000 Hz", "10000 Hz", "20000 Hz"}),
		fake.gateParameter("midi", "MIDI In Note", []string{"C2", "C3", "C4", "C5", "C6"}),
		fake.gateParameter("channel", "Channel 1 Sidechain", []string{"Input 1", "Input 2", "Input 3", "Input 4", "Input 5"}),
		fake.gateParameter("unplanned", "Meter State", []string{"0", ".25", ".5", ".75", "1"}),
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "gate-1", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeGateExpanderKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	copied := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, errors.New("invalid fake gate write")
		}
		fake.params[id] = value
		copied = append(copied, map[string]any{"parameter_id": id, "normalized_value": value})
	}
	fake.batchCalls = append(fake.batchCalls, copied)
	if fake.mutateOther && len(fake.batchCalls) == 1 {
		fake.params["unplanned"] = .9
	}
	if fake.failBatchAt[len(fake.batchCalls)] {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected gate failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeGateExpanderKernel) gateParameter(id, name string, labels []string) map[string]any {
	normalized := fake.params[id]
	index := int(math.Round(normalized * float64(len(labels)-1)))
	if index < 0 {
		index = 0
	}
	if index >= len(labels) {
		index = len(labels) - 1
	}
	samples := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		samples = append(samples, map[string]any{"normalized_value": float64(i) / float64(len(labels)-1), "text": label})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true, "normalized_value": normalized, "value_text": labels[index],
		"display_probe": map[string]any{"mode": "samples", "label": name, "samples": samples}}
}

func (fake *fakeGateExpanderKernel) snapshot() map[string]float64 {
	out := map[string]float64{}
	for key, value := range fake.params {
		out[key] = value
	}
	return out
}

func gateTestControlRef(summary map[string]any, role string) string {
	stage := mapValue(summary["gate_expander_stage"])
	for _, section := range []string{"detector", "operating_point", "gain_action", "direction_control", "timing", "mode", "output"} {
		for _, row := range mapRowsValue(stage[section]) {
			if firstNonEmptyText(row, "role") == role {
				return firstNonEmptyText(row, "control_ref")
			}
		}
	}
	return ""
}

func TestApplyGateExpanderControlsRestoresFullPreimageOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sideEffect bool
		fail       map[int]bool
	}{{name: "side effect", sideEffect: true}, {name: "batch failure", fail: map[int]bool{1: true}}} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeGateExpanderKernel()
			fake.mutateOther, fake.failBatchAt = tc.sideEffect, tc.fail
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			_, summary, err := server.readLiveGateExpanderControlSurface(context.Background(), "track-1", "gate-1")
			if err != nil {
				t.Fatal(err)
			}
			ref := gateTestControlRef(summary, "threshold")
			if ref == "" || gateTestControlRef(summary, "sidechain_highpass") == "" {
				t.Fatalf("missing typed refs: %+v", summary)
			}
			before := fake.snapshot()
			result, err := server.applyPluginGrabberGateExpanderControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "atomic": true,
				"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}}, nil)
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

func TestApplyGateExpanderControlsSupportsDetectorFrequencyAndIndependentRestore(t *testing.T) {
	fake := newFakeGateExpanderKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveGateExpanderControlSurface(context.Background(), "track-1", "gate-1")
	if err != nil {
		t.Fatal(err)
	}
	before := fake.snapshot()
	hpf := gateTestControlRef(summary, "sidechain_highpass")
	result, err := server.applyPluginGrabberGateExpanderControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": hpf, "frequency_hz": 500.0}}}, nil)
	if err != nil || firstNonEmptyText(result, "status") != "exact" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	restore := firstNonEmptyText(result, "restore_ref")
	if !strings.HasPrefix(restore, "g1rr1_") {
		t.Fatalf("restore=%q", restore)
	}
	if _, err := decodeCompressorRestoreRef(restore); err == nil {
		t.Fatal("gate restore ref decoded as compressor restore ref")
	}
	if _, err := server.applyPluginGrabberGateExpanderControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "atomic": true, "restore_ref": restore}, nil); err != nil {
		t.Fatal(err)
	}
	if !fakeSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("restore drift before=%v after=%v", before, fake.snapshot())
	}
}

func TestGateExpanderDirectionRejectsCompressorLabel(t *testing.T) {
	if gateExpanderDirectionLabelAllowed("Compressor") || gateExpanderDirectionLabelAllowed("Upward") || gateExpanderDirectionLabelAllowed("Upward Expander") {
		t.Fatal("compressor/upward direction must fail closed")
	}
	if !gateExpanderDirectionLabelAllowed("Gate") || !gateExpanderDirectionLabelAllowed("Downward Expander") {
		t.Fatal("proved gate/expander direction was rejected")
	}
}

func TestGenericWriteGuardProtectsTypedGateButNotG8UnsupportedCapabilities(t *testing.T) {
	fake := newFakeGateExpanderKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	owned, blocked := server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), harness.InvokeRequest{Tool: "plugin.set_parameter",
		Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "param_id": "threshold", "value": .2}})
	if !blocked || firstNonEmptyText(owned.Result, "rejection_code") != "typed_gate_expander_control_required" {
		t.Fatalf("owned=%+v blocked=%v", owned, blocked)
	}
	_, blocked = server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), harness.InvokeRequest{Tool: "plugin.set_parameter",
		Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "param_id": "midi", "value": .2}})
	if blocked {
		t.Fatal("MIDI unsupported capability leaked into typed ownership")
	}
	_, blocked = server.guardGateExpanderOwnedGenericParameterWrite(context.Background(), harness.InvokeRequest{Tool: "plugin.set_parameter",
		Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "param_id": "channel", "value": .2}})
	if blocked {
		t.Fatal("multichannel unsupported capability leaked into typed ownership")
	}
}

func TestPluginGrabberWorkflowExecutorBridgesGateExpanderInspectAndApply(t *testing.T) {
	fake := newFakeGateExpanderKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	base := &recordingToolExecutor{}
	executor := pluginGrabberWorkflowExecutor{server: server, base: base}
	inspect, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{ID: "inspect", Tool: pluginGrabberInspectGateExpanderTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1"}}})
	if err != nil || inspect.Status != "ok" || gateTestControlRef(inspect.Result, "threshold") == "" {
		t.Fatalf("inspect=%#v err=%v", inspect, err)
	}
	ref := gateTestControlRef(inspect.Result, "threshold")
	apply, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{ID: "apply", Tool: pluginGrabberApplyGateExpanderTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "gate-1", "atomic": true,
			"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}}}})
	if err != nil || apply.Status != "ok" || firstNonEmptyText(apply.Result, "status") != "exact" {
		t.Fatalf("apply=%#v err=%v", apply, err)
	}
	if len(base.calls) != 0 || len(fake.batchCalls) != 1 {
		t.Fatalf("gate typed call escaped or was not atomic: base=%d batches=%#v", len(base.calls), fake.batchCalls)
	}
}
