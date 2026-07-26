package chat

import "testing"

func TestAgentLoopCapabilityNamesIncludesPluginForKnownThirdPartyName(t *testing.T) {
	got := agentLoopCapabilityNames("use Pro-Q 3 to cut 200Hz mud", map[string]any{
		"tracks": []any{map[string]any{
			"plugins": []any{map[string]any{"plugin_name": "FabFilter Pro-Q 3", "plugin_id": "1018"}},
		}},
	})
	for _, name := range got {
		if name == "plugin" {
			return
		}
	}
	t.Fatalf("plugin capability package was not selected for a known third-party plug-in name: %#v", got)
}

func TestAgentLoopCapabilityNamesIncludesSelectedProQForChineseEQRequest(t *testing.T) {
	got := agentLoopCapabilityNames("用 Pro-Q 3 将 3400 Hz 降低 3 dB，Q 设置为 0.5。", map[string]any{
		"selected_plugin_id":   "1018",
		"selected_plugin_name": "Pro-Q 3",
	})
	if !containsToolName(got, "plugin") {
		t.Fatalf("selected Pro-Q Chinese EQ request did not select plugin capability: %#v", got)
	}
}
