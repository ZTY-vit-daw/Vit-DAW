package toolpolicy

import "testing"

func TestSpectralDynamicsInspectIsNotMutation(t *testing.T) {
	if IsPluginMutationTool("plugin_grabber.inspect_spectral_dynamics") || IsPluginMutationTool("plugin_grabber_inspect_spectral_dynamics") {
		t.Fatal("spectral dynamics inspect must remain read-only")
	}
}
