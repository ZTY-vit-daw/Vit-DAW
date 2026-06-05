package agentloop

import (
	"testing"

	"vit-daw-agent/internal/planner"
)

func TestCoercePluginGrabberLearningToolCallForChineseLearningGoal(t *testing.T) {
	call := planner.ToolCall{
		Tool: "plugin.get_parameters",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
		},
	}
	goal := "\u5b66\u4e60\u5f53\u524d\u63d2\u4ef6\u7684\u5e38\u7528\u63a7\u5236"

	got := coercePluginGrabberLearningToolCall(goal, call)

	if got.Tool != pluginGrabberLearnToolName {
		t.Fatalf("tool = %q, want %q", got.Tool, pluginGrabberLearnToolName)
	}
	if got.Args["track_id"] != "1007" || got.Args["plugin_id"] != "1013" {
		t.Fatalf("target args were not preserved: %#v", got.Args)
	}
	if got.Args["intent"] != goal {
		t.Fatalf("intent = %#v", got.Args["intent"])
	}
	if got.Command != nil {
		t.Fatalf("command should be cleared after tool rewrite: %#v", got.Command)
	}
}

func TestCoercePluginGrabberLearningToolCallLeavesOrdinaryReadsAlone(t *testing.T) {
	call := planner.ToolCall{Tool: "plugin.get_parameters", Args: map[string]any{"plugin_id": "1013"}}

	got := coercePluginGrabberLearningToolCall("\u67e5\u770b\u5f53\u524d\u63d2\u4ef6\u53c2\u6570", call)

	if got.Tool != call.Tool {
		t.Fatalf("tool = %q, want unchanged %q", got.Tool, call.Tool)
	}
}

func TestCoercePluginGrabberLearningToolCallLeavesLearnedControlsQueriesAlone(t *testing.T) {
	call := planner.ToolCall{Tool: "plugin.get_parameters", Args: map[string]any{"plugin_id": "1013"}}

	got := coercePluginGrabberLearningToolCall("\u5f53\u524d\u63d2\u4ef6\u6709\u54ea\u4e9b\u5df2\u5b66\u4e60\u7684\u5e38\u7528\u63a7\u5236\uff1f", call)

	if got.Tool != call.Tool {
		t.Fatalf("tool = %q, want unchanged %q", got.Tool, call.Tool)
	}
}

func TestCoercePluginGrabberRuntimeToolCallForMudCut(t *testing.T) {
	call := planner.ToolCall{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"param_id":  "band_2_gain",
			"value":     -0.2,
		},
	}

	got := coercePluginGrabberRuntimeToolCall("Use the EQ plugin to gently reduce mud around 500Hz.", call)

	if got.Tool != pluginGrabberApplyToolName {
		t.Fatalf("tool = %q, want %q", got.Tool, pluginGrabberApplyToolName)
	}
	if got.Args["track_id"] != "1007" || got.Args["plugin_id"] != "1013" {
		t.Fatalf("target args were not preserved: %#v", got.Args)
	}
	if got.Args["control"] != "eq.cut_region" {
		t.Fatalf("control = %#v", got.Args["control"])
	}
	target, ok := got.Args["target"].(map[string]any)
	if !ok {
		t.Fatalf("target missing: %#v", got.Args)
	}
	if target["freq_hz"] != float64(500) || target["gain_db"] != float64(-2.5) || target["amount"] != "gentle" {
		t.Fatalf("target = %#v", target)
	}
	if got.Command != nil {
		t.Fatalf("command should be cleared after tool rewrite: %#v", got.Command)
	}
}

func TestCoercePluginGrabberRuntimeToolCallForPresenceBoost(t *testing.T) {
	call := planner.ToolCall{
		Tool: "set_plugin_param",
		Args: map[string]any{"track_id": "1007", "plugin_id": "1013"},
	}

	got := coercePluginGrabberRuntimeToolCall("Boost 4kHz presence by 3dB on this plugin.", call)

	if got.Tool != pluginGrabberApplyToolName {
		t.Fatalf("tool = %q, want %q", got.Tool, pluginGrabberApplyToolName)
	}
	if got.Args["control"] != "eq.boost_region" {
		t.Fatalf("control = %#v", got.Args["control"])
	}
	target := got.Args["target"].(map[string]any)
	if target["freq_hz"] != float64(4000) || target["gain_db"] != float64(3) {
		t.Fatalf("target = %#v", target)
	}
}

func TestCoercePluginGrabberRuntimeToolCallForDelayMs(t *testing.T) {
	call := planner.ToolCall{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"param_id":  "delay",
			"value":     0.5,
		},
	}

	got := coercePluginGrabberRuntimeToolCall("把这个 delay 插件的延迟设为 1000ms。", call)

	if got.Tool != pluginGrabberApplyToolName {
		t.Fatalf("tool = %q, want %q", got.Tool, pluginGrabberApplyToolName)
	}
	if got.Args["control"] != "Echo Length" {
		t.Fatalf("control = %#v", got.Args["control"])
	}
	target := got.Args["target"].(map[string]any)
	if target["amount"] != float64(1000) {
		t.Fatalf("target = %#v", target)
	}
}

func TestCoercePluginGrabberRuntimeToolCallLeavesExactParamIDAlone(t *testing.T) {
	call := planner.ToolCall{
		Tool: "plugin.set_parameter",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"param_id":  "threshold",
			"value":     -12,
		},
	}

	got := coercePluginGrabberRuntimeToolCall("Set param_id threshold to -12 raw.", call)

	if got.Tool != call.Tool {
		t.Fatalf("tool = %q, want unchanged %q", got.Tool, call.Tool)
	}
}

func TestNormalizePluginGrabberApplyControlForRelativeChineseDb(t *testing.T) {
	call := planner.ToolCall{
		Tool: pluginGrabberApplyToolName,
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"control":   "teach.output_gain",
			"target":    map[string]any{},
		},
	}

	got := coercePluginGrabberRuntimeToolCall("\u628a\u521a\u5b66\u7684\u8f93\u51fa\u589e\u76ca\u964d\u4f4e8dB", call)

	if got.Tool != pluginGrabberApplyToolName {
		t.Fatalf("tool = %q", got.Tool)
	}
	if got.Args["value_mode"] != "relative_delta" {
		t.Fatalf("value_mode = %#v", got.Args["value_mode"])
	}
	target := got.Args["target"].(map[string]any)
	if target["gain_db"] != float64(-8) {
		t.Fatalf("target = %#v", target)
	}
}

func TestNormalizePluginGrabberApplyControlForChinesePercentAmount(t *testing.T) {
	call := planner.ToolCall{
		Tool: pluginGrabberApplyToolName,
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"control":   "teach.\u5e72\u6e7f\u6bd4",
			"target":    map[string]any{},
		},
	}

	got := coercePluginGrabberRuntimeToolCall("\u628a\u521a\u5b66\u7684\u5e72\u6e7f\u6bd4\u63a7\u5236\u8bbe\u7f6e\u523028%", call)

	if got.Tool != pluginGrabberApplyToolName {
		t.Fatalf("tool = %q", got.Tool)
	}
	target := got.Args["target"].(map[string]any)
	if target["amount"] != float64(28) {
		t.Fatalf("target = %#v", target)
	}
}

func TestNormalizePluginGrabberApplyControlReplacesDefaultEqForDelay(t *testing.T) {
	call := planner.ToolCall{
		Tool: pluginGrabberApplyToolName,
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"control":   "eq.set_region",
			"target":    map[string]any{},
		},
	}

	got := coercePluginGrabberRuntimeToolCall("把回声长度设置为 2s", call)

	if got.Args["control"] != "Echo Length" {
		t.Fatalf("control = %#v", got.Args["control"])
	}
	target := got.Args["target"].(map[string]any)
	if target["amount"] != float64(2000) {
		t.Fatalf("target = %#v", target)
	}
}
