package tools

import "testing"

func TestSpectralDynamicsInspectCatalogIsReadOnly(t *testing.T) {
	spec, ok := DefaultCatalog().LookupTool("plugin_grabber.inspect_spectral_dynamics")
	if !ok || spec.CommandName != "plugin_grabber_inspect_spectral_dynamics" || spec.RiskLevel != RiskDirect || spec.MutatesProject || spec.SupportsUndo {
		t.Fatalf("spec=%+v ok=%v", spec, ok)
	}
}
