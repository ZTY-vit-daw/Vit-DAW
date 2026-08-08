package chat

import (
	"testing"

	"vit-daw-agent/internal/harness"
)

func TestPluginGrabberInspectSpectralDynamicsInvokeCommand(t *testing.T) {
	cmd, ok := pluginGrabberInspectSpectralDynamicsInvokeCommand(harness.InvokeRequest{
		Tool: pluginGrabberInspectSpectralDynamicsTool,
		Args: map[string]any{"track_id": "track-1", "plugin_id": "plugin-1"},
	})
	if !ok || firstNonEmptyText(cmd, "cmd") != pluginGrabberInspectSpectralDynamicsCommand ||
		firstNonEmptyText(cmd, "track_id") != "track-1" || firstNonEmptyText(cmd, "plugin_id") != "plugin-1" {
		t.Fatalf("cmd=%v ok=%v", cmd, ok)
	}
}
