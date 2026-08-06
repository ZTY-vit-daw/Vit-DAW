package tools

import "testing"

func TestCompressorToolsAreRegisteredWithStableRisk(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_compressor")
	if !ok || inspect.CommandName != "plugin_grabber_inspect_compressor" || inspect.RiskLevel != RiskDirect || inspect.MutatesProject {
		t.Fatalf("inspect compressor metadata = %+v ok=%v", inspect, ok)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_compressor_controls")
	if !ok || apply.CommandName != "plugin_grabber_apply_compressor_controls" || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply compressor metadata = %+v ok=%v", apply, ok)
	}
	hint := argHint("plugin_grabber_apply_compressor_controls")
	for _, required := range []string{"control_ref", "value_db", "ratio", "value_ms", "enum_label"} {
		if !containsText(hint, required) {
			t.Fatalf("apply compressor args hint omitted %q: %s", required, hint)
		}
	}
}

func containsText(text, needle string) bool {
	for index := 0; index+len(needle) <= len(text); index++ {
		if text[index:index+len(needle)] == needle {
			return true
		}
	}
	return false
}
