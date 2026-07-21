package agentloop

import (
	"testing"

	"vit-daw-agent/internal/planner"
)

func TestNamedPluginAcousticApplyBypassesObserveFirstPolicy(t *testing.T) {
	state := &runState{
		input: Input{
			UserText:     "用 TDR Nova 切掉 200Hz 附近的浑浊",
			AllowedTools: []string{"mix.observe", "plugin_grabber.apply_control"},
			Context: map[string]any{
				"selected_track_id":  "1007",
				"selected_plugin_id": "1013",
			},
		},
	}
	call := planner.ToolCall{
		Tool: "plugin_grabber.apply_control",
		Args: map[string]any{
			"track_id":  "1007",
			"plugin_id": "1013",
			"control":   "eq.cut_region",
			"target":    map[string]any{"freq_hz": 200.0, "gain_db": -2.5},
		},
	}

	if messageLoopNeedsDeterministicMixObservation(state) {
		t.Fatal("explicit named plug-in action was incorrectly diverted into deterministic observe-first")
	}
	if issue := messageLoopToolGuardIssue(state, call, false); issue != "" {
		t.Fatalf("explicit named plug-in apply was blocked before L4 controls: %s", issue)
	}
}

func TestNamedPluginApplyStillRequiresCompleteLiveTarget(t *testing.T) {
	state := &runState{input: Input{UserText: "用 TDR Nova 切掉 200Hz 附近的浑浊"}}
	call := planner.ToolCall{
		Tool: "plugin_grabber.apply_control",
		Args: map[string]any{"plugin_id": "1013", "control": "eq.cut_region"},
	}
	if issue := messageLoopToolGuardIssue(state, call, false); issue == "" {
		t.Fatal("incomplete plug-in apply target must be denied before execution")
	}
}
