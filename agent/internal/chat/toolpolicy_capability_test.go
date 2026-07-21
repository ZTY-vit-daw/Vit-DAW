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
