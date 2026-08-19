package processorregistry

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/processorintent"
)

func TestDefaultRegistryHasNoPhraseRoutingSurface(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.List()) != 9 {
		t.Fatalf("registered families=%d", len(r.List()))
	}
	for _, definition := range r.List() {
		encoded := strings.ToLower(definition.Recognizer + " " + definition.Planner)
		if strings.Contains(encoded, "keyword") || strings.Contains(encoded, "phrase") || strings.Contains(encoded, "trigger") {
			t.Fatalf("registry component identifier implies phrase routing: %+v", definition)
		}
	}
	if definition, ok := r.Resolve(processorintent.FamilySpectralDynamics); !ok || !definition.InspectOnly {
		t.Fatal("spectral dynamics must remain inspect-only")
	}
}

func TestRegistryValidatesCoverageWithoutAddingDefaults(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateCoverage(processorintent.FamilyLimiter, []string{"output_ceiling"}); err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateCoverage(processorintent.FamilyLimiter, nil); err != nil {
		t.Fatalf("empty coverage should be accepted as an explicit caller decision, not defaulted: %v", err)
	}
	if err := r.ValidateCoverage(processorintent.FamilyLimiter, []string{"not_a_limiter_axis"}); err == nil {
		t.Fatal("unknown limiter coverage was accepted")
	}
}

func TestRegistrySeparatesSemanticCoverageFromPCAProof(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ValidateCoverage(processorintent.FamilyDeEsser, []string{"threshold", "frequency_focus", "range"}); err != nil {
		t.Fatalf("de-esser semantic coverage was not accepted: %v", err)
	}
	proof, err := r.PCARequiredCoverage(processorintent.FamilyDeEsser, []string{"threshold", "frequency_focus", "range"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"threshold_sensitivity": true, "detector_focus": true, "sibilance_reduction": true}
	if len(proof) != len(want) {
		t.Fatalf("PCA proof=%+v", proof)
	}
	for _, item := range proof {
		if !want[item.Axis] || item.Action != "adjust" {
			t.Fatalf("unexpected PCA proof item=%+v", item)
		}
	}
	if _, err := r.PCARequiredCoverage(processorintent.FamilyDeEsser, []string{"threshold", "unproven_axis"}); err == nil {
		t.Fatal("unproven semantic coverage was converted")
	}
	if _, err := r.PCARequiredCoverage(processorintent.FamilyStaticEQ, []string{"frequency"}); err == nil {
		t.Fatal("generic EQ semantic axis acquired an implicit PCA proof")
	}
}

func TestRegistryRejectsPCAFamilySharingAcrossBoundaryFamilies(t *testing.T) {
	registry, err := New()
	if err != nil {
		t.Fatal(err)
	}
	clipper := Definition{
		Family:             processorintent.FamilyClipper,
		PCAFamily:          "limiter",
		Recognizer:         "plugingrabber.clipper_boundary",
		CoverageVocabulary: []string{"inspect_only"},
		Planner:            "inspect_only",
		Materializer:       "none",
		TypedExecutor:      "none",
		ReceiptProjector:   "clipper_boundary_receipt",
		ObservationViews:   []string{"track.peak_structure"},
		InspectOnly:        true,
	}
	if err := registry.Register(clipper); err == nil || !strings.Contains(err.Error(), "cannot share executable PCA") {
		t.Fatalf("clipper was allowed to share limiter PCA: %v", err)
	}
	limiter := Definition{
		Family:             processorintent.FamilyLimiter,
		PCAFamily:          processorintent.FamilyDeEsser,
		Recognizer:         "plugingrabber.limiter_topology",
		CoverageVocabulary: []string{"ceiling"},
		Planner:            "semantic_limiter",
		Materializer:       "limiter_materializer",
		TypedExecutor:      "plugin_grabber.apply_limiter_controls",
		ReceiptProjector:   "limiter_receipt",
		ObservationViews:   []string{"track.peak_structure"},
	}
	if err := registry.Register(limiter); err == nil || !strings.Contains(err.Error(), "must use its own PCA family") {
		t.Fatalf("limiter was allowed to borrow de-esser PCA: %v", err)
	}
}
