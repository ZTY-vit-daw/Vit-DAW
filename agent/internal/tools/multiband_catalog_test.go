package tools

import "testing"

func TestMultibandToolsInCatalog(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_multiband")
	if !ok || inspect.CommandName != "plugin_grabber_inspect_multiband" || inspect.RiskLevel != RiskDirect || inspect.MutatesProject {
		t.Fatalf("inspect=%+v ok=%v", inspect, ok)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_multiband_controls")
	if !ok || apply.CommandName != "plugin_grabber_apply_multiband_controls" || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply=%+v ok=%v", apply, ok)
	}
	if hint := argHint("plugin_grabber_apply_multiband_controls"); hint == "" || hint == argHint("plugin_grabber_apply_compressor_controls") {
		t.Fatalf("multiband argument hint=%q", hint)
	}
}
