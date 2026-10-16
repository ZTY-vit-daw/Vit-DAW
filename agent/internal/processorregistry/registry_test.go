package processorregistry

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
)

func TestDefaultRegistryHasNoPhraseRoutingSurface(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	// M2-TIMEFX-1 adds the reverb/delay time-domain families to the nine
	// dynamic/spectral rows; GENSHELL-PCA-1 adds the generation_shell
	// load-admission row.
	if len(r.List()) != 12 {
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

// GENSHELL-PCA-1: the generation_shell row is load-admission only. The PCA
// coverage vocabulary v1 is the empty set (件四 §2), so the row registers no
// coverage proof and typed control routing must fail closed.
func TestGenerationShellRowIsLoadAdmissionOnly(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	definition, ok := r.Resolve(processorintent.FamilyGenerationShell)
	if !ok {
		t.Fatal("generation_shell is not registered")
	}
	if definition.InspectOnly {
		t.Fatal("generation_shell must not be an inspect-only boundary family")
	}
	if len(definition.CoverageVocabulary) != 1 || definition.CoverageVocabulary[0] != "load_only" {
		t.Fatalf("generation_shell semantic vocabulary=%v", definition.CoverageVocabulary)
	}
	if len(definition.CoverageProofs) != 0 {
		t.Fatalf("generation_shell registered typed control proofs: %+v", definition.CoverageProofs)
	}
	if _, err := r.PCARequiredCoverage(processorintent.FamilyGenerationShell, []string{"load_only"}); err == nil {
		t.Fatal("generation_shell semantic marker was converted into a PCA coverage grant")
	}
	if err := r.ValidateCoverage(processorintent.FamilyGenerationShell, []string{"threshold"}); err == nil {
		t.Fatal("generation_shell accepted a typed control semantic axis")
	}
}

// 三处词表一致性：generation_shell 在 processorintent 家族词表、
// processorattestation v2 家族表（coverage 空集）与 processorregistry 注册行
// 三处在场且互指一致；未登记家族在三处全部 fail-closed。
func TestGenerationShellVocabularyIsConsistentAcrossThreeSurfaces(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if !processorintent.IsSupportedFamily(processorintent.FamilyGenerationShell) {
		t.Fatal("generation_shell missing from the intent family vocabulary")
	}
	if !processorattestation.IsV2Family(processorattestation.FamilyGenerationShell) {
		t.Fatal("generation_shell missing from the attestation v2 family table")
	}
	if axes := processorattestation.V2CoverageAxes(processorattestation.FamilyGenerationShell); len(axes) != 0 {
		t.Fatalf("generation_shell attestation coverage vocabulary must be empty, got %v", axes)
	}
	definition, ok := r.Resolve(processorintent.FamilyGenerationShell)
	if !ok || definition.PCAFamily != processorattestation.FamilyGenerationShell {
		t.Fatalf("generation_shell registry row inconsistent: %+v ok=%v", definition, ok)
	}
	for _, family := range r.List() {
		if !processorintent.IsSupportedFamily(family.Family) {
			t.Fatalf("registered family %q is not in the intent vocabulary", family.Family)
		}
	}
	if processorintent.IsSupportedFamily("generation_shell_v2") ||
		processorattestation.IsV2Family("generation_shell_v2") {
		t.Fatal("unknown family variant was admitted by a vocabulary")
	}
	if _, ok := r.Resolve("generation_shell_v2"); ok {
		t.Fatal("unknown family variant resolved in the registry")
	}
}

func TestGenerationShellDeclaresNativeSeedFact(t *testing.T) {
	r, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	seeds := r.NativeSeeds()
	if len(seeds) != 1 {
		t.Fatalf("expected exactly one native seed declaration, got %+v", seeds)
	}
	seed := seeds[0]
	if seed.Family != processorintent.FamilyGenerationShell {
		t.Fatalf("native seed family=%q", seed.Family)
	}
	if seed.Fact.Identifier != "VST3-GenShell-15b5622d-b818da1c" || seed.Fact.Name != "GenShell" || seed.Fact.Manufacturer != "Vit Audio" || seed.Fact.Format != "VST3" {
		t.Fatalf("native seed fact=%+v", seed.Fact)
	}
	if !strings.HasPrefix(seed.Fact.BundlePath, "~/Library/Audio/Plug-Ins/VST3/GenShell.vst3") {
		t.Fatalf("native seed bundle path=%q", seed.Fact.BundlePath)
	}
}

func TestRegistryRejectsNativeSeedOnTypedFamilies(t *testing.T) {
	invalid := []Definition{
		{Family: processorintent.FamilyReverb, PCAFamily: processorattestation.FamilyReverb, Recognizer: "x", CoverageVocabulary: []string{"decay"}, Planner: "p", Materializer: "m", TypedExecutor: "t", ReceiptProjector: "r", ObservationViews: []string{"v"}, NativeSeed: &NativeSeedFact{BundlePath: "/tmp/B.vst3", Identifier: "VST3-B-1", Name: "B", Manufacturer: "V", Format: "VST3"}},
		{Family: processorintent.FamilyGenerationShell, PCAFamily: processorattestation.FamilyGenerationShell, Recognizer: "x", CoverageVocabulary: []string{"load_only"}, Planner: "load_admission", Materializer: "none", TypedExecutor: "none", ReceiptProjector: "generation_job_audit", ObservationViews: []string{"mix.multitrack_relationship"}, NativeSeed: &NativeSeedFact{BundlePath: "", Identifier: "VST3-B-1", Name: "B", Manufacturer: "V", Format: "VST3"}},
	}
	for index, definition := range invalid {
		if _, err := New(definition); err == nil {
			t.Fatalf("invalid native seed definition %d was accepted", index)
		}
	}
}
