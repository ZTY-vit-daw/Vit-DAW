package toolpolicy

import "testing"

func TestTransientShaperApplyIsPluginMutation(t *testing.T) {
	for _, name := range []string{"plugin_grabber.apply_transient_shaper_controls", "plugin_grabber_apply_transient_shaper_controls"} {
		if !IsPluginMutationTool(name) {
			t.Fatalf("%s must be a plugin mutation tool", name)
		}
	}
	if IsPluginMutationTool("plugin_grabber.inspect_transient_shaper") {
		t.Fatal("inspect must remain read-only")
	}
}
