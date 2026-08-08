package tools

import (
	"strings"
	"testing"
)

func TestTransientShaperToolsAreCataloged(t *testing.T) {
	catalog := DefaultCatalog()
	inspect, ok := catalog.LookupTool("plugin_grabber.inspect_transient_shaper")
	if !ok || inspect.RiskLevel != RiskDirect || inspect.MutatesProject {
		t.Fatalf("inspect=%+v ok=%v", inspect, ok)
	}
	apply, ok := catalog.LookupTool("plugin_grabber.apply_transient_shaper_controls")
	if !ok || apply.RiskLevel != RiskUndoable || !apply.MutatesProject || !apply.SupportsUndo {
		t.Fatalf("apply=%+v ok=%v", apply, ok)
	}
	hint := argHint("plugin_grabber_apply_transient_shaper_controls")
	if !strings.Contains(hint, "control_ref") || !strings.Contains(hint, "percent") {
		t.Fatalf("apply arg hint=%q", hint)
	}
}
