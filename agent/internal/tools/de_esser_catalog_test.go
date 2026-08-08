package tools

import "testing"

func TestDeEsserCatalogContractIsReadOnly(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_de_esser")
	if !ok || inspect.CommandName != "plugin_grabber_inspect_de_esser" || inspect.RiskLevel != RiskDirect || inspect.MutatesProject || inspect.SupportsUndo {
		t.Fatalf("inspect=%+v ok=%v", inspect, ok)
	}
	if hint := argHint("plugin_grabber_inspect_de_esser"); hint != "track_id:string plugin_id:string optional intent:string" {
		t.Fatalf("De-esser arg hint=%q", hint)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_de_esser_controls")
	if !ok || apply.CommandName != "plugin_grabber_apply_de_esser_controls" || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply=%+v ok=%v", apply, ok)
	}
	if hint := argHint("plugin_grabber_apply_de_esser_controls"); hint == "" || hint == argHint("plugin_grabber_apply_limiter_controls") {
		t.Fatalf("De-esser apply arg hint=%q", hint)
	}
}
