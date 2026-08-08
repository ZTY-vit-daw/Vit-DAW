package toolpolicy

import "testing"

func TestGateExpanderTypedApplyIsMutationButInspectIsNot(t *testing.T) {
	if !IsPluginMutationTool("plugin_grabber.apply_gate_expander_controls") || !IsPluginMutationTool("plugin_grabber_apply_gate_expander_controls") {
		t.Fatal("gate/expander apply must be a plugin mutation")
	}
	if IsPluginMutationTool("plugin_grabber.inspect_gate_expander") || IsPluginMutationTool("plugin_grabber_inspect_gate_expander") {
		t.Fatal("gate/expander inspect must remain read-only")
	}
}
