package toolpolicy

import "testing"

func TestDeEsserInspectIsNotAPluginMutation(t *testing.T) {
	if IsPluginMutationTool("plugin_grabber.inspect_de_esser") || IsPluginMutationTool("plugin_grabber_inspect_de_esser") {
		t.Fatal("De-esser inspect must remain read-only")
	}
}

func TestDeEsserApplyIsAPluginMutation(t *testing.T) {
	if !IsPluginMutationTool("plugin_grabber.apply_de_esser_controls") || !IsPluginMutationTool("plugin_grabber_apply_de_esser_controls") {
		t.Fatal("De-esser apply must be classified as a plugin mutation")
	}
}
