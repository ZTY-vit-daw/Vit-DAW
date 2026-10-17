package processorattestation

// GENSHELL-PCA-1: generation_shell is the load-admission-only v2 family (件四
// §2). Its coverage vocabulary v1 is the empty set: the family is admitted to
// the v2 store and family-level admission queries, every typed control axis is
// rejected, and attestations for it carry zero coverage entries.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeGenerationShellBundleFixture(t *testing.T, root string) string {
	t.Helper()
	bundle := filepath.Join(root, "GenShell.vst3")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "GenShell"), []byte("genshell-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestGenerationShellCoverageVocabularyIsEmptySet(t *testing.T) {
	if !IsV2Family(FamilyGenerationShell) {
		t.Fatal("generation_shell must be a PCA v2 family for load admission routing")
	}
	if axes := V2CoverageAxes(FamilyGenerationShell); len(axes) != 0 {
		t.Fatalf("generation_shell coverage vocabulary must be the empty set, got %v", axes)
	}
	for _, axis := range []string{"output_ceiling", "decay_time", "load_only", "inspect_only"} {
		if err := ValidateV2Coverage(FamilyGenerationShell, Coverage{Action: "adjust", Axis: axis}); err == nil {
			t.Fatalf("generation_shell accepted typed control axis %q", axis)
		}
	}
	if roles := V2CoverageForRoles(FamilyGenerationShell, []string{"threshold", "mix", "decay", "load"}); len(roles) != 0 {
		t.Fatalf("generation_shell must map no topology role to a control axis, got %+v", roles)
	}
}

func TestGenerationShellBundleFingerprintIsRepeatable(t *testing.T) {
	bundle := writeGenerationShellBundleFixture(t, t.TempDir())
	first, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("bundle fingerprint must be repeatable: %q vs %q", first, second)
	}
}

func TestGenerationShellIssuesPromotesAndAdmitsWithZeroCoverage(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeGenerationShellBundleFixture(t, root)
	fingerprint, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	subject := Subject{Name: "GenShell", Manufacturer: "Vit", Format: "VST3", Identifier: "genshell-v1", InstalledPath: bundle}
	key, err := BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	spec := IssueSpecV2{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyGenerationShell,
		Evidence: []EvidenceRef{{ReceiptID: "genshell-build-receipt", Kind: "generation_shell.load", SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)}}}

	typed := spec
	typed.Coverage = []Coverage{{Action: "adjust", Axis: "load_only"}}
	if _, err := NewAttestationV2(typed, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("generation_shell attestation with typed coverage was accepted")
	}

	promoted, err := store.PromoteCurrent(spec, "genshell_load_admission")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != StatusPromoted || len(promoted.Coverage) != 0 {
		t.Fatalf("promoted=%+v", promoted)
	}

	library, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := QueryLibraryAdmissionV2(library, key, fingerprint, FamilyGenerationShell)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted.Eligible || admitted.Reason != "promoted_binary_admitted" || admitted.Attestation.AttestationID != promoted.AttestationID {
		t.Fatalf("admission result=%+v", admitted)
	}

	// The action-coverage query face fails closed: no valid coverage exists.
	if _, err := store.Query(QueryV2{SubjectKey: key, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyGenerationShell, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "load_only"}}}); err == nil {
		t.Fatal("generation_shell action coverage query was accepted")
	}

	// A different current binary is not the admitted subject.
	if result, err := QueryLibraryAdmissionV2(library, key, "sha256:"+strings.Repeat("c", 64), FamilyGenerationShell); err != nil || result.Eligible || result.EffectiveStatus != StatusStale || result.Reason != "binary_fingerprint_changed" {
		t.Fatalf("fingerprint mismatch result=%+v err=%v", result, err)
	}

	revoked, err := store.Revoke(promoted.AttestationID, "genshell_load_admission_revoked")
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Status != StatusRevoked {
		t.Fatalf("revoked=%+v", revoked)
	}
	library, _, err = store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if result, err := QueryLibraryAdmissionV2(library, key, fingerprint, FamilyGenerationShell); err != nil || result.Eligible || result.Reason != "attestation_revoked" {
		t.Fatalf("revoked admission result=%+v err=%v", result, err)
	}
}

func TestGenerationShellDoesNotWeakenTypedFamilyCoverageRequirements(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Limiter.vst3")
	if err := os.WriteFile(pluginPath, []byte("limiter-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	spec := IssueSpecV2{Subject: Subject{Name: "Limiter", Manufacturer: "Vendor", Format: "VST3", Identifier: "limiter-v1", InstalledPath: pluginPath}, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter,
		Evidence: []EvidenceRef{{ReceiptID: "receipt-v2", Kind: "processor.v2", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)}}}
	if _, err := NewAttestationV2(spec, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("typed family issued with empty coverage")
	}
	if IsV2Family("unknown_family_v9") {
		t.Fatal("unknown family must not be admitted as a v2 family")
	}
	if _, err := NewAttestationV2(IssueSpecV2{ProcessorFamily: "unknown_family_v9"}, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("unknown family issued an attestation")
	}
}
