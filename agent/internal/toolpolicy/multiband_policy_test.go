package toolpolicy

import "testing"

func TestMultibandMutationPolicy(t *testing.T) {
	if !IsPluginMutationTool("plugin_grabber.apply_multiband_controls") || !IsPluginMutationTool("plugin_grabber_apply_multiband_controls") {
		t.Fatal("multiband apply must be classified as a plugin mutation")
	}
	if IsPluginMutationTool("plugin_grabber.inspect_multiband") || IsPluginMutationTool("plugin_grabber_inspect_multiband") {
		t.Fatal("multiband inspect must remain read-only")
	}
}
