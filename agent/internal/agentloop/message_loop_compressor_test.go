package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

type compressorMessageExecutor struct {
	calls     []planner.ToolCall
	failApply bool
}

func (fake *compressorMessageExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	fake.calls = append(fake.calls, in.ToolCall)
	switch in.ToolCall.Tool {
	case "plugin_grabber.inspect_compressor":
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool,
			CommandName: "plugin_grabber_inspect_compressor", Status: "ok", Result: map[string]any{
				"status": "ok", "control_topology": map[string]any{"generation": "ct1_test"},
				"compressor_stage": map[string]any{"control_paths": []map[string]any{{
					"operating_point": []map[string]any{{"role": "threshold", "control_ref": "ccr_threshold"}},
					"transfer":        []map[string]any{{"role": "ratio", "control_ref": "ccr_ratio"}},
				}}},
			}}, nil
	case "plugin_grabber.apply_compressor_controls":
		if fake.failApply {
			return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool,
				CommandName: "plugin_grabber_apply_compressor_controls", Status: "error",
				Error: "invalid_control_ref: stale ref", Result: map[string]any{
					"status": "rejected", "rejection_code": "invalid_control_ref", "parameters_changed": false,
				}}, nil
		}
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool,
			CommandName: "plugin_grabber_apply_compressor_controls", Status: "ok", Result: map[string]any{
				"status": "exact", "atomic": true, "controls": []map[string]any{
					{"role": "threshold", "status": "exact", "actual_readback": []map[string]any{{"role": "threshold", "value_text": "-12.00 dB"}}},
					{"role": "ratio", "status": "exact", "actual_readback": []map[string]any{{"role": "ratio", "value_text": "4.00:1"}}},
				},
			}}, nil
	default:
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "error", Error: "unexpected tool"}, nil
	}
}

func TestMessageLoopExplicitCompressorParametersUseTypedPathAndReadbackReply(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"读取压缩器控制拓扑。","tool_calls":[{"id":"inspect","tool":"plugin_grabber.inspect_compressor","args":{"track_id":"1007","plugin_id":"1013"}}]}`,
		`{"final":false,"reply":"应用明确参数。","tool_calls":[{"id":"apply","tool":"plugin_grabber.apply_compressor_controls","args":{"track_id":"1007","plugin_id":"1013","atomic":true,"controls":[{"control_ref":"ccr_threshold","value_db":-12},{"control_ref":"ccr_ratio","ratio":4}]}}]}`,
	}}
	exec := &compressorMessageExecutor{}
	loop := &MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec, Budget: Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2}}
	res := loop.Start(context.Background(), Input{
		UserText:     "把 Threshold 设置为 -12 dB，Ratio 设置为 4:1",
		AllowedTools: []string{"plugin_grabber.inspect_compressor", "plugin_grabber.apply_compressor_controls", "plugin_grabber.explain_controls", "plugin.set_parameter"},
		Context:      map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"},
	})
	if res.Status != "completed" || len(exec.calls) != 2 {
		t.Fatalf("result status=%q reply=%q error=%q calls=%#v", res.Status, res.Reply, res.Error, exec.calls)
	}
	if exec.calls[0].Tool != "plugin_grabber.inspect_compressor" || exec.calls[1].Tool != "plugin_grabber.apply_compressor_controls" {
		t.Fatalf("wrong compressor route: %#v", exec.calls)
	}
	for _, forbidden := range []string{"plugin_grabber.explain_controls", "plugin.set_parameter"} {
		for _, call := range exec.calls {
			if call.Tool == forbidden {
				t.Fatalf("typed route used forbidden fallback %s", forbidden)
			}
		}
	}
	for _, want := range []string{"typed", "exact", "Threshold 实际读回 -12.00 dB", "Ratio 实际读回 4.00:1"} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("truthful readback reply missing %q: %s", want, res.Reply)
		}
	}
}

func TestMessageLoopFailedTypedCompressorApplyStopsWithoutGenericFallback(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"读取压缩器控制拓扑。","tool_calls":[{"id":"inspect","tool":"plugin_grabber.inspect_compressor","args":{"track_id":"1007","plugin_id":"1013"}}]}`,
		`{"final":false,"reply":"应用明确参数。","tool_calls":[{"id":"apply","tool":"plugin_grabber.apply_compressor_controls","args":{"track_id":"1007","plugin_id":"1013","atomic":true,"controls":[{"control_ref":"stale","value_db":-12}]}}]}`,
		`{"final":false,"reply":"改用通用参数。","tool_calls":[{"id":"fallback","tool":"plugin.set_parameter","args":{"track_id":"1007","plugin_id":"1013","param_id":"1","value":0.8}}]}`,
		`{"final":true,"reply":"已成功设置。","tool_calls":[]}`,
	}}
	exec := &compressorMessageExecutor{failApply: true}
	loop := &MessageLoop{Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec, Budget: Budget{MaxTurns: 6, MaxToolCalls: 4, MaxConsecutiveErrors: 2}}
	res := loop.Start(context.Background(), Input{
		UserText:     "把 Threshold 设置为 -12 dB",
		AllowedTools: []string{"plugin_grabber.inspect_compressor", "plugin_grabber.apply_compressor_controls", "plugin.set_parameter"},
		Context:      map[string]any{"selected_track_id": "1007", "selected_plugin_id": "1013"},
	})
	if res.Status != "completed" || len(exec.calls) != 2 {
		t.Fatalf("result status=%q reply=%q error=%q calls=%#v trace=%#v", res.Status, res.Reply, res.Error, exec.calls, res.Trace)
	}
	if strings.Contains(res.Reply, "已成功") || !strings.Contains(res.Reply, "未完成") || !strings.Contains(res.Reply, "没有使用通用参数写入回退") {
		t.Fatalf("failure was not reported truthfully: %s", res.Reply)
	}
	if len(client.responses) != 2 {
		t.Fatalf("typed failure should stop immediately without asking the model for retry/fallback, remaining responses=%d", len(client.responses))
	}
}

func TestMessageLoopPromptSeparatesTypedCompressorFromGenericFallback(t *testing.T) {
	prompt := messageLoopSystemPrompt(&runState{input: Input{AllowedTools: []string{
		"plugin_grabber.inspect_compressor", "plugin_grabber.apply_compressor_controls", "plugin.set_parameter",
	}}})
	for _, want := range []string{"plugin_grabber.inspect_compressor", "plugin_grabber.apply_compressor_controls", "generation-scoped control_ref", "do not call plugin.set_parameter"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("compressor prompt contract missing %q", want)
		}
	}
	if strings.Contains(prompt, "For a non-EQ parameter, use set_plugin_param") {
		t.Fatalf("prompt retained conflicting global non-EQ fallback rule")
	}
}

func TestCompactMessageLoopExecutionPreservesCompressorControlRefs(t *testing.T) {
	record := map[string]any{
		"status": "ok", "tool": "plugin_grabber.inspect_compressor", "command_name": "plugin_grabber_inspect_compressor",
		"result": map[string]any{
			"status": "ok", "track_id": "1007", "plugin_id": "1013", "classification": "threshold_driven",
			"control_topology": map[string]any{"schema_version": "compressor-control-topology/v1", "generation": "ct1_test"},
			"compressor_stage": map[string]any{"control_paths": []map[string]any{{"path_key": "main",
				"operating_point": []map[string]any{{"role": "threshold", "param_id": "1", "control_ref": "ccr_threshold", "current_text": "-12.00 dB", "domain": map[string]any{"unit": "dB", "min": -60.0, "max": 0.0}}},
				"transfer":        []map[string]any{{"role": "ratio", "param_id": "2", "control_ref": "ccr_ratio", "current_text": "4.00:1", "domain": map[string]any{"unit": "ratio", "min": 1.0, "max": 100.0}}},
			}}},
		},
	}
	compact := compactMessageLoopExecution(record)
	summary := messageLoopMapValue(compact["compressor_control_summary"])
	controls := messageLoopMapRows(summary["controls"])
	if len(controls) != 2 || firstMapText(controls[0], "control_ref") != "ccr_threshold" || firstMapText(controls[1], "control_ref") != "ccr_ratio" {
		t.Fatalf("compact compressor refs missing: %#v", compact)
	}
	if _, exists := compact["result_summary"]; exists {
		t.Fatalf("generic result summarizer should not compete with compressor projection: %#v", compact)
	}
}
