package toolpolicy

import "testing"

func TestLimiterTypedApplyIsMutationButInspectIsNot(t *testing.T) {
	if !IsPluginMutationTool("plugin_grabber.apply_limiter_controls") || !IsPluginMutationTool("plugin_grabber_apply_limiter_controls") {
		t.Fatal("limiter apply must be a plugin mutation")
	}
	if IsPluginMutationTool("plugin_grabber.inspect_limiter") || IsPluginMutationTool("plugin_grabber_inspect_limiter") {
		t.Fatal("limiter inspect must remain read-only")
	}
}
