package toolpolicy

import "testing"

func TestCompressorApplyIsPluginMutation(t *testing.T) {
	if !IsPluginMutationTool("plugin_grabber.apply_compressor_controls") || !IsPluginMutationTool("plugin_grabber_apply_compressor_controls") {
		t.Fatal("compressor apply must be governed as a plugin mutation")
	}
	if IsPluginMutationTool("plugin_grabber.inspect_compressor") {
		t.Fatal("compressor inspect must remain read-only")
	}
}
