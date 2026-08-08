package tools

import "testing"

func TestLimiterCatalogContracts(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_limiter")
	if !ok || inspect.CommandName != "plugin_grabber_inspect_limiter" || inspect.RiskLevel != RiskDirect || inspect.MutatesProject {
		t.Fatalf("inspect=%+v ok=%v", inspect, ok)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_limiter_controls")
	if !ok || apply.CommandName != "plugin_grabber_apply_limiter_controls" || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply=%+v ok=%v", apply, ok)
	}
	if hint := argHint("plugin_grabber_apply_limiter_controls"); hint == "" || hint == argHint("plugin_grabber_apply_compressor_controls") {
		t.Fatalf("limiter arg hint=%q", hint)
	}
}
