package tools

import "testing"

func TestGateExpanderCatalogContracts(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_gate_expander")
	if !ok || inspect.CommandName != "plugin_grabber_inspect_gate_expander" || inspect.RiskLevel != RiskDirect || inspect.MutatesProject {
		t.Fatalf("inspect=%+v ok=%v", inspect, ok)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_gate_expander_controls")
	if !ok || apply.CommandName != "plugin_grabber_apply_gate_expander_controls" || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply=%+v ok=%v", apply, ok)
	}
	if hint := argHint("plugin_grabber_apply_gate_expander_controls"); hint == "" || hint == argHint("plugin_grabber_apply_compressor_controls") {
		t.Fatalf("gate/expander arg hint=%q", hint)
	}
}
