package toolpolicy

import "testing"

func TestPluginPolicyRules(t *testing.T) {
	tests := []struct {
		name string
		call ToolCall
	}{
		{name: "retired virtual control", call: ToolCall{Name: "plugin_grabber.apply_control"}},
		{name: "retired profile route", call: ToolCall{Name: "plugin_grabber.get_project_profiles"}},
		{name: "retired alternate spelling", call: ToolCall{Name: "plugin.grabber.apply"}},
		{name: "retired SPAL route", call: ToolCall{Name: "spal.reference_eq_provider.register"}},
		{name: "retired VPS route", call: ToolCall{Name: "vps.verify"}},
		{name: "retired VPSForge route", call: ToolCall{Name: "vpsforge.generate"}},
		{name: "retired plugin VPS route", call: ToolCall{Name: "plugin_vps.apply"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(TurnContext{MutationBarrier: true}, tt.call)
			if got.Verdict != Deny || got.Rule != "retired_plugin_control_surface" {
				t.Fatalf("Decide() = %#v", got)
			}
		})
	}
}
func TestLowMudNeedsObservationOnlyWithoutNamedPlugin(t *testing.T) {
	if !LowMudNeedsObservation("帮我处理一下低频浑浊", nil) {
		t.Fatal("broad low-mud request should still require observation")
	}
	if LowMudNeedsObservation("用 TDR Nova 切掉 200Hz 附近的浑浊", nil) {
		t.Fatal("explicitly named plug-in action must not be reclassified as broad prep")
	}
}

func TestCollectPluginNames(t *testing.T) {
	got := CollectPluginNames(map[string]any{
		"tracks":               []any{map[string]any{"plugins": []any{map[string]any{"plugin_name": "FabFilter Pro-Q 3"}}}},
		"selected_plugin_name": "TDR Nova",
	})
	if len(got) != 2 || !MentionsPlugin("use Pro-Q 3", got) || !MentionsPlugin("通过 TDR Nova", got) {
		t.Fatalf("unexpected collected names: %#v", got)
	}
}

func TestSelectedPluginNameIsRecognizedForExplicitControl(t *testing.T) {
	got := CollectPluginNames(map[string]any{
		"selected_plugin_id":   "1018",
		"selected_plugin_name": "FabFilter Pro-Q 3",
	})
	if !MentionsPlugin("用 Pro-Q 3 将 3400 Hz 降低 3 dB，Q 设置为 0.5。", got) {
		t.Fatalf("selected plugin name was not recognized: %#v", got)
	}
	if !ExplicitPluginRequest("用 Pro-Q 3 将 3400 Hz 降低 3 dB，Q 设置为 0.5。", got) {
		t.Fatalf("selected Pro-Q request was not recognized as explicit control: %#v", got)
	}
}

func TestExplicitPluginRequestLegacyShadowConsistency(t *testing.T) {
	// These cases were already classified as explicit by the pre-centralization
	// message-loop heuristic. Keeping them here as a shadow oracle prevents the
	// new package from silently changing established routing while call sites move.
	legacyCases := []struct {
		name string
		text string
		want bool
	}{
		{name: "explicit plugin load", text: "请直接加载 TDR Nova 插件", want: true},
		{name: "raw parameter write", text: "Set param_id threshold to 0.42 normalized", want: true},
		{name: "broad mud treatment", text: "帮我处理低频浑浊", want: false},
	}
	for _, tt := range legacyCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExplicitPluginRequest(tt.text, nil); got != tt.want {
				t.Fatalf("ExplicitPluginRequest(%q) = %v, legacy shadow wants %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestExplicitPluginRequestAcceptedNamedPluginDivergence(t *testing.T) {
	// This is the intentional Phase 2 behavior change: the old broad-mix keyword
	// gate did not recognize a named third-party plug-in plus an acoustic action.
	if !ExplicitPluginRequest("用 TDR Nova 切掉 200Hz 附近的浑浊", nil) {
		t.Fatal("named plug-in acoustic action should be the accepted legacy divergence")
	}
}
