package vpsforge

import (
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

func TestProQ3StaticEQDraftTestCandidateIsCredentialFreeAndTestOnly(t *testing.T) {
	controls := []proQ3CoreEQObservedControl{
		{Role: "band_used", ID: "0", HostLabel: "Band 1 Used"},
		{Role: "band_enabled", ID: "1", HostLabel: "Band 1 Enabled"},
		{Role: "band_frequency", ID: "2", HostLabel: "Band 1 Frequency"},
		{Role: "band_gain", ID: "3", HostLabel: "Band 1 Gain"},
		{Role: "band_q", ID: "7", HostLabel: "Band 1 Q"},
		{Role: "band_shape", ID: "8", HostLabel: "Band 1 Shape"},
	}
	mappings, err := proQ3StaticEQDraftTestMappings(controls, []proQ3CoreEQChoice{{ObservedDisplay: "Bell"}, {ObservedDisplay: "Low Cut"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 6 || mappings[5].Key != "pro_q_3_test_band_1.shape" || len(mappings[5].ObservedValues) != 2 {
		t.Fatalf("unexpected test-only mappings: %#v", mappings)
	}
	source := vps.NewDraft(vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0"}, time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC))
	for _, mapping := range mappings {
		source.ControlSurface.Mappings = append(source.ControlSurface.Mappings, vps.ControlSurfaceMapping{ParameterID: mapping.ParameterID, Label: mapping.ObservedLabel})
	}
	candidate := proQ3StaticEQDraftTestDocument(source, mappings, time.Date(2026, 7, 18, 1, 0, 0, 0, time.UTC))
	if err := candidate.Validate(); err != nil {
		t.Fatalf("candidate must remain a valid Draft: %v", err)
	}
	if candidate.Status != vps.VPSStatusDraft || len(candidate.ProviderCredentials) != 0 {
		t.Fatalf("candidate gained authority: %#v", candidate)
	}
	for _, mapping := range candidate.ControlSurface.Mappings {
		if mapping.BindingStatus != proQ3StaticEQDraftTestBindingStatus || mapping.ExecutionScope != proQ3StaticEQDraftTestExecutionScope || mapping.Confirmed {
			t.Fatalf("candidate mapping lost test-only boundary: %#v", mapping)
		}
	}
}

func TestProQ3StaticEQDraftTestRuntimeLibraryIsSeparate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "draft_test_runtime", "vps_library_v3.json")
	candidate := vps.NewDraft(vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0"}, time.Now().UTC())
	if err := proQ3StaticEQDraftTestWriteLibrary(path, candidate); err != nil {
		t.Fatal(err)
	}
	library, err := vps.NewLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := library.List()
	if err != nil || len(documents) != 1 || documents[0].ID != candidate.ID || len(documents[0].ProviderCredentials) != 0 {
		t.Fatalf("isolated Draft-test library = %#v err=%v", documents, err)
	}
}

func TestProQ3StagingBindingUsesActionScopedSlopeCoverage(t *testing.T) {
	binding, schemas := proQ3StaticEQStagingBinding()
	if len(schemas) != 2 || binding.Bands["b1"].Allocated == nil || binding.HighPass == nil || binding.HighPass.ResponseShape == nil {
		t.Fatalf("staging binding does not model the Band allocation pass-filter route: %#v schemas=%#v", binding, schemas)
	}
	if binding.HighPass.SlopeDBPerOctave.Values["12"] != 1.0/9.0 {
		t.Fatalf("observed 12 dB/oct slope mapping = %#v", binding.HighPass.SlopeDBPerOctave)
	}
	if binding.HighPass.SlopeDBPerOctave.Values["24"] != 3.0/9.0 {
		t.Fatalf("24 dB/oct audit candidate mapping = %#v", binding.HighPass.SlopeDBPerOctave)
	}
	implementations := proQ3StaticEQStagingActionImplementations()
	if len(implementations) != 2 || implementations[1].SchemaID != spal.EQPassFilterPatchControlID || !implementations[1].SupportsRequiredFeatures([]string{"highpass", "slope_24_db_per_octave"}, vps.BadgeFeatureStatusStagingReady) {
		t.Fatalf("staging action feature matrix = %#v", implementations)
	}
}

func TestProQ3SlopeAuditMappingRemainsTestOnly(t *testing.T) {
	source := vps.NewDraft(vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0"}, time.Now().UTC())
	source.ControlSurface.Mappings = []vps.ControlSurfaceMapping{{ParameterID: "9", Label: "Band 1 Slope"}}
	mapping, err := proQ3StaticEQSlopeAuditMapping(source)
	if err != nil || mapping.ParameterID != "9" || mapping.BindingStatus != proQ3StaticEQDraftTestBindingStatus || mapping.ExecutionScope != proQ3StaticEQDraftTestExecutionScope {
		t.Fatalf("slope audit mapping=%#v err=%v", mapping, err)
	}
}
