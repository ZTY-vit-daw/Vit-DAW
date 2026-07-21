package vps

import "testing"

func TestEqualizerV2BadgeContractDefinesReusableActionsAndAuditSequence(t *testing.T) {
	contract := EqualizerV2BadgeContract()
	if err := contract.Validate(); err != nil {
		t.Fatalf("equalizer.v2 badge contract: %v", err)
	}
	passFilter, found := contract.Action("eq.pass_filter.patch")
	if !found || passFilter.SchemaID != "eq.pass_filter.patch.v2" {
		t.Fatalf("pass-filter action = %#v, found=%v", passFilter, found)
	}
	if len(passFilter.AuthoringAudit) != 5 || passFilter.RuntimeTransaction[len(passFilter.RuntimeTransaction)-1] != "guarded_manual_rollback" {
		t.Fatalf("pass-filter sequence is incomplete: %#v", passFilter)
	}
}

func TestVPSActionImplementationRequiresTheBadgeFeatureMatrix(t *testing.T) {
	contract := EqualizerV2BadgeContract()
	implementation := VPSActionImplementation{
		BadgeID: contract.BadgeID, BadgeVersion: contract.Version,
		ActionID: "eq.static_band.patch", SchemaID: "eq.band.patch.v2",
		Status: "staging_ready", BindingRef: "proq3.band1.static",
		Features: []BadgeFeatureMatrixEntry{
			{ActionID: "eq.static_band.patch", FeatureID: "static_band", Status: BadgeFeatureStatusStagingReady},
			{ActionID: "eq.static_band.patch", FeatureID: "bell", Status: BadgeFeatureStatusStagingReady},
		},
	}
	if err := implementation.ValidateAgainst(contract); err != nil {
		t.Fatalf("valid staged implementation rejected: %v", err)
	}
	implementation.Features = implementation.Features[:1]
	if err := implementation.ValidateAgainst(contract); err == nil {
		t.Fatal("implementation missing bell was accepted")
	}
}

func TestVPSActionImplementationDoesNotTreatOneSlopeAsAllSlopeValues(t *testing.T) {
	implementation := VPSActionImplementation{
		BadgeID: EqualizerCapabilityID, BadgeVersion: EqualizerProfileVersion,
		ActionID: "eq.pass_filter.patch", SchemaID: "eq.pass_filter.patch.v2", Status: BadgeFeatureStatusStagingReady, BindingRef: "spal.eq_v2.staging_binding",
		Features: []BadgeFeatureMatrixEntry{
			{ActionID: "eq.pass_filter.patch", FeatureID: "highpass", Status: BadgeFeatureStatusStagingReady},
			{ActionID: "eq.pass_filter.patch", FeatureID: "slope_12_db_per_octave", Status: BadgeFeatureStatusStagingReady},
			{ActionID: "eq.pass_filter.patch", FeatureID: "slope_24_db_per_octave", Status: BadgeFeatureStatusUnsupported},
		},
	}
	if !implementation.SupportsRequiredFeatures([]string{"highpass", "slope_12_db_per_octave"}, BadgeFeatureStatusStagingReady) {
		t.Fatal("explicitly staged high-pass 12 dB/oct feature was rejected")
	}
	if implementation.SupportsRequiredFeatures([]string{"highpass", "slope_24_db_per_octave"}, BadgeFeatureStatusStagingReady) {
		t.Fatal("unsupported 24 dB/oct feature was accepted from the 12 dB/oct row")
	}
}
