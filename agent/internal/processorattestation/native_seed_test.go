package processorattestation

// GENSHELL-PCA-1 (方案 B): registry-driven permanent load admission for
// native load-only families. The seed must be idempotent across restarts,
// must honor a runtime revocation inside the process lifetime, must re-seed
// a new record on the next pass (session-scoped revocation), and must
// supersede rather than duplicate when the bundle fingerprint changes.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeNativeSeedBundle(t *testing.T, root, payload string) string {
	t.Helper()
	bundle := filepath.Join(root, "GenShell.vst3")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "GenShell"), []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func nativeSeedSpecFor(bundle string) NativeSeedSpec {
	return NativeSeedSpec{
		ProcessorFamily: FamilyGenerationShell,
		BundlePath:      bundle,
		Identifier:      "VST3-GenShell-15b5622d-b818da1c",
		Name:            "GenShell",
		Manufacturer:    "Vit Audio",
		Format:          "VST3",
	}
}

func readNativeSeedLibrary(t *testing.T, store *StoreV2) LibraryV2 {
	t.Helper()
	library, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	return library
}

func TestNativeSeedIssuesPromotesAndAdmits(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeNativeSeedBundle(t, root, "genshell-v1")
	spec := nativeSeedSpecFor(bundle)
	fingerprint, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 16, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Seeded || result.Attestation.Status != StatusPromoted {
		t.Fatalf("first seed result=%+v", result)
	}
	if result.Attestation.BinaryFingerprint != fingerprint {
		t.Fatalf("seed bound fingerprint=%s want=%s", result.Attestation.BinaryFingerprint, fingerprint)
	}
	if len(result.Attestation.Coverage) != 0 {
		t.Fatalf("native seed must carry zero coverage, got %+v", result.Attestation.Coverage)
	}
	if result.Attestation.StatusReason != "native_registry_seeded" {
		t.Fatalf("promotion reason=%q", result.Attestation.StatusReason)
	}

	key, err := BuildSubjectKey(Subject{Name: "GenShell", Manufacturer: "Vit Audio", Format: "VST3", Identifier: spec.Identifier, InstalledPath: bundle})
	if err != nil {
		t.Fatal(err)
	}
	library := readNativeSeedLibrary(t, store)
	admission, err := QueryLibraryAdmissionV2(library, key, fingerprint, FamilyGenerationShell)
	if err != nil {
		t.Fatal(err)
	}
	if !admission.Eligible || admission.Reason != "promoted_binary_admitted" {
		t.Fatalf("admission after seed=%+v", admission)
	}
}

func TestNativeSeedIsIdempotentAcrossRestarts(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeNativeSeedBundle(t, root, "genshell-v1")
	spec := nativeSeedSpecFor(bundle)

	first, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 16, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	before := readNativeSeedLibrary(t, store)

	second, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	after := readNativeSeedLibrary(t, store)
	if second.Seeded {
		t.Fatal("second seed pass wrote a record")
	}
	if second.Attestation.AttestationID != first.Attestation.AttestationID {
		t.Fatalf("idempotent seed changed attestation id: %q vs %q", second.Attestation.AttestationID, first.Attestation.AttestationID)
	}
	if after.Revision != before.Revision || len(after.Attestations) != len(before.Attestations) {
		t.Fatalf("idempotent seed mutated the library: revision %d->%d entries %d->%d", before.Revision, after.Revision, len(before.Attestations), len(after.Attestations))
	}
}

func TestNativeSeedHonorsRevocationThenReseedsNextPass(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeNativeSeedBundle(t, root, "genshell-v1")
	spec := nativeSeedSpecFor(bundle)
	fingerprint, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	key, err := BuildSubjectKey(Subject{Name: "GenShell", Manufacturer: "Vit Audio", Format: "VST3", Identifier: spec.Identifier, InstalledPath: bundle})
	if err != nil {
		t.Fatal(err)
	}

	first, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 16, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	// Runtime revocation: within the process lifetime no re-seed happens and
	// admission fails closed.
	if _, err := store.Revoke(first.Attestation.AttestationID, "runtime_revocation_probe"); err != nil {
		t.Fatal(err)
	}
	library := readNativeSeedLibrary(t, store)
	admission, err := QueryLibraryAdmissionV2(library, key, fingerprint, FamilyGenerationShell)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Eligible || admission.Reason != "attestation_revoked" {
		t.Fatalf("revoked admission result=%+v", admission)
	}

	// Next startup (next seeding pass) re-seeds a fresh promoted record; the
	// revoked residue stays behind harmlessly.
	reseed, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !reseed.Seeded || reseed.Attestation.Status != StatusPromoted {
		t.Fatalf("re-seed result=%+v", reseed)
	}
	if reseed.Attestation.AttestationID == first.Attestation.AttestationID {
		t.Fatal("re-seed must issue a distinct attestation record")
	}
	library = readNativeSeedLibrary(t, store)
	if admission, err := QueryLibraryAdmissionV2(library, key, fingerprint, FamilyGenerationShell); err != nil || !admission.Eligible || admission.Attestation.AttestationID != reseed.Attestation.AttestationID {
		t.Fatalf("admission after re-seed=%+v err=%v", admission, err)
	}
}

func TestNativeSeedSupersedesWhenBundleFingerprintChanges(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeNativeSeedBundle(t, root, "genshell-v1")
	spec := nativeSeedSpecFor(bundle)

	first, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 16, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}

	// Rebuild the bundle in place: same identity, different bytes.
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "GenShell"), []byte("genshell-v2-upgraded"), 0o600); err != nil {
		t.Fatal(err)
	}
	newFingerprint, err := FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if newFingerprint == first.Attestation.BinaryFingerprint {
		t.Fatal("bundle rewrite produced the same fingerprint")
	}

	upgrade, err := SeedNativeLoadOnly(store, spec, time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !upgrade.Seeded || upgrade.Attestation.BinaryFingerprint != newFingerprint {
		t.Fatalf("upgrade seed result=%+v", upgrade)
	}
	library := readNativeSeedLibrary(t, store)
	stale := 0
	promoted := 0
	for _, attestation := range library.Attestations {
		switch {
		case attestation.AttestationID == first.Attestation.AttestationID && attestation.Status == StatusStale:
			stale++
		case attestation.AttestationID == upgrade.Attestation.AttestationID && attestation.Status == StatusPromoted:
			promoted++
		}
	}
	if stale != 1 || promoted != 1 {
		t.Fatalf("supersede shape wrong: stale=%d promoted=%d library=%+v", stale, promoted, library.Attestations)
	}
	key, err := BuildSubjectKey(Subject{Name: "GenShell", Manufacturer: "Vit Audio", Format: "VST3", Identifier: spec.Identifier, InstalledPath: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if admission, err := QueryLibraryAdmissionV2(library, key, newFingerprint, FamilyGenerationShell); err != nil || !admission.Eligible {
		t.Fatalf("admission after upgrade=%+v err=%v", admission, err)
	}
}

func TestNativeSeedRejectsTypedFamiliesAndInvalidSpecs(t *testing.T) {
	root := t.TempDir()
	store, err := NewStoreV2(filepath.Join(root, "attestations.v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle := writeNativeSeedBundle(t, root, "genshell-v1")

	typed := nativeSeedSpecFor(bundle)
	typed.ProcessorFamily = FamilyLimiter
	if _, err := SeedNativeLoadOnly(store, typed, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "typed coverage vocabulary") {
		t.Fatalf("typed family seed error=%v", err)
	}

	unknown := nativeSeedSpecFor(bundle)
	unknown.ProcessorFamily = "unknown_native_family"
	if _, err := SeedNativeLoadOnly(store, unknown, time.Now().UTC()); err == nil {
		t.Fatal("unknown family seed was accepted")
	}

	missingPath := nativeSeedSpecFor(filepath.Join(root, "Absent.vst3"))
	if _, err := SeedNativeLoadOnly(store, missingPath, time.Now().UTC()); err == nil {
		t.Fatal("missing bundle seed was accepted")
	}

	noIdentifier := nativeSeedSpecFor(bundle)
	noIdentifier.Identifier = ""
	if _, err := SeedNativeLoadOnly(store, noIdentifier, time.Now().UTC()); err == nil {
		t.Fatal("identifier-less seed was accepted")
	}
}
