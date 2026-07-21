package vps

import (
	"testing"
	"time"
)

func TestDraftAcceptsPlannedFXMMeasurementWithoutGrantingCredential(t *testing.T) {
	document := NewDraft(PluginIdentity{Manufacturer: "Example", Name: "Famous Compressor", Format: "VST3", Version: "1.0"}, time.Now())
	document.FXMMeasurements = []FXMMeasurementRequirement{DefaultFXMMeasurementRequirement("dynamics.compressor.v1")}
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(document.ProviderCredentials) != 0 || document.Status != VPSStatusDraft {
		t.Fatalf("FXM authoring requirement granted authority: %+v", document)
	}
}

func TestConformedFXMMeasurementRequiresExternalEvidenceRef(t *testing.T) {
	requirement := DefaultFXMMeasurementRequirement(EqualizerCapabilityID)
	requirement.Status = "conformed"
	if err := requirement.Validate(); err == nil {
		t.Fatal("conformed FXM requirement without evidence should fail")
	}
	requirement.EvidenceRefs = []string{"fxm:projection_1"}
	if err := requirement.Validate(); err != nil {
		t.Fatal(err)
	}
}
