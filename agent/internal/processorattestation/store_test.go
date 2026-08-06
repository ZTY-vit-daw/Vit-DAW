package processorattestation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreIssuePromoteQueryAndTransitions(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 8, 6, 4, 0, 0, 0, time.UTC)
	store.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	issued, err := store.Issue(fixtureSpec())
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := store.Issue(fixtureSpec())
	if err != nil || duplicate.AttestationID != issued.AttestationID {
		t.Fatalf("idempotent issue failed: duplicate=%+v err=%v", duplicate, err)
	}
	query := Query{SubjectKey: issued.Subject.SubjectKey, BinaryFingerprint: issued.BinaryFingerprint,
		ProcessorFamily: FamilyStaticEQ, RequiredCoverage: []Coverage{{Action: "upsert", Shape: "bell"}}}
	result, err := store.Query(query)
	if err != nil || result.Eligible || result.EffectiveStatus != StatusIssued {
		t.Fatalf("issued query=%+v err=%v", result, err)
	}
	promoted, err := store.Promote(issued.AttestationID, "replay_threshold_met")
	if err != nil || promoted.Status != StatusPromoted {
		t.Fatalf("promote=%+v err=%v", promoted, err)
	}
	result, err = store.Query(query)
	if err != nil || !result.Eligible || result.Reason != "promoted_action_covered" {
		t.Fatalf("promoted query=%+v err=%v", result, err)
	}
	query.RequiredCoverage = []Coverage{{Action: "remove", Shape: "bell"}}
	result, err = store.Query(query)
	if err != nil || result.Eligible || result.Reason != "required_action_not_covered" || len(result.MissingCoverage) != 1 {
		t.Fatalf("coverage query=%+v err=%v", result, err)
	}
	stale, err := store.MarkStale(issued.AttestationID, "receipt_superseded")
	if err != nil || stale.Status != StatusStale {
		t.Fatalf("stale=%+v err=%v", stale, err)
	}
	if _, err := store.Promote(issued.AttestationID, "invalid_repromotion"); err == nil {
		t.Fatal("stale attestation was promoted")
	}
	revoked, err := store.Revoke(issued.AttestationID, "operator_revoked")
	if err != nil || revoked.Status != StatusRevoked {
		t.Fatalf("revoke=%+v err=%v", revoked, err)
	}
}

func TestQueryTreatsBinaryChangeAsEffectiveStaleWithoutMutation(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), FileName))
	issued, err := store.Issue(fixtureSpec())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(issued.AttestationID, "evidence_passed"); err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(Query{SubjectKey: issued.Subject.SubjectKey,
		BinaryFingerprint: "sha256:" + strings.Repeat("c", 64), ProcessorFamily: FamilyStaticEQ,
		RequiredCoverage: []Coverage{{Action: "upsert", Shape: "bell"}}})
	if err != nil || result.Eligible || result.EffectiveStatus != StatusStale || result.Reason != "binary_fingerprint_changed" {
		t.Fatalf("fingerprint query=%+v err=%v", result, err)
	}
	library, _, err := store.Read()
	if err != nil || library.Attestations[0].Status != StatusPromoted {
		t.Fatalf("query mutated stored state: %+v err=%v", library.Attestations, err)
	}
}

func TestQueryRequiresValidCurrentActionCoverage(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), FileName))
	issued, err := store.Issue(fixtureSpec())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(issued.AttestationID, "evidence_passed"); err != nil {
		t.Fatal(err)
	}
	base := Query{SubjectKey: issued.Subject.SubjectKey, BinaryFingerprint: issued.BinaryFingerprint,
		ProcessorFamily: FamilyStaticEQ}
	if result, err := store.Query(base); err == nil || result.Eligible {
		t.Fatalf("empty current action coverage was accepted: result=%+v err=%v", result, err)
	}
	base.RequiredCoverage = []Coverage{{Action: "upsert", Shape: "dynamic_bell"}}
	if result, err := store.Query(base); err == nil || result.Eligible {
		t.Fatalf("invalid current action coverage was accepted: result=%+v err=%v", result, err)
	}
}

func TestQueryLibraryMatchesStoreQuery(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), FileName))
	issued, err := store.PromoteCurrent(fixtureSpec(), "evidence_passed")
	if err != nil {
		t.Fatal(err)
	}
	library, _, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	query := Query{SubjectKey: issued.Subject.SubjectKey, BinaryFingerprint: issued.BinaryFingerprint,
		ProcessorFamily: FamilyStaticEQ, RequiredCoverage: []Coverage{{Action: "upsert", Shape: "bell"}}}
	direct, err := QueryLibrary(library, query)
	if err != nil || !direct.Eligible {
		t.Fatalf("direct query=%+v err=%v", direct, err)
	}
	stored, err := store.Query(query)
	if err != nil || direct.Eligible != stored.Eligible || direct.Attestation.AttestationID != stored.Attestation.AttestationID {
		t.Fatalf("direct=%+v stored=%+v err=%v", direct, stored, err)
	}
}

func TestQueryPrefersCurrentFingerprintAcrossVersions(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), FileName))
	oldSpec := fixtureSpec()
	old, err := store.Issue(oldSpec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(old.AttestationID, "old_version_passed"); err != nil {
		t.Fatal(err)
	}
	currentSpec := fixtureSpec()
	currentSpec.BinaryFingerprint = "sha256:" + strings.Repeat("c", 64)
	current, err := store.Issue(currentSpec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(current.AttestationID, "current_version_passed"); err != nil {
		t.Fatal(err)
	}
	result, err := store.Query(Query{SubjectKey: current.Subject.SubjectKey, BinaryFingerprint: current.BinaryFingerprint,
		ProcessorFamily: FamilyStaticEQ, RequiredCoverage: []Coverage{{Action: "upsert", Shape: "bell"}}})
	if err != nil || !result.Eligible || result.Attestation.AttestationID != current.AttestationID {
		t.Fatalf("current version query=%+v err=%v", result, err)
	}
}

func TestStoreRecoversFromLastGoodBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	store, _ := NewStore(path)
	issued, err := store.Issue(fixtureSpec())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Promote(issued.AttestationID, "evidence_passed"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o600); err != nil {
		t.Fatal(err)
	}
	library, report, err := store.Read()
	if err != nil || !report.RecoveredFromBackup || len(library.Attestations) != 1 {
		t.Fatalf("backup recovery library=%+v report=%+v err=%v", library, report, err)
	}
}

func TestPromoteCurrentStalesPriorBadgeAtomically(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), FileName))
	clock := time.Date(2026, 8, 6, 6, 0, 0, 0, time.UTC)
	store.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	old, err := store.PromoteCurrent(fixtureSpec(), "strong_receipt_passed")
	if err != nil || old.Status != StatusPromoted {
		t.Fatalf("first promotion=%+v err=%v", old, err)
	}
	currentSpec := fixtureSpec()
	currentSpec.BinaryFingerprint = "sha256:" + strings.Repeat("c", 64)
	current, err := store.PromoteCurrent(currentSpec, "strong_receipt_passed")
	if err != nil || current.Status != StatusPromoted {
		t.Fatalf("current promotion=%+v err=%v", current, err)
	}
	library, _, err := store.Read()
	if err != nil || len(library.Attestations) != 2 {
		t.Fatalf("library=%+v err=%v", library, err)
	}
	for _, attestation := range library.Attestations {
		if attestation.AttestationID == old.AttestationID &&
			(attestation.Status != StatusStale || attestation.StatusReason != "binary_fingerprint_superseded") {
			t.Fatalf("old badge was not retired: %+v", attestation)
		}
	}
}

func TestStrictDecodeRejectsForbiddenUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	store, _ := NewStore(path)
	if err := os.WriteFile(path, []byte(`{"schema_version":"processor_control_attestations.v1","revision":1,"updated_at":"2026-08-06T00:00:00Z","attestations":[],"param_id":"forbidden"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Read(); err == nil || !strings.Contains(err.Error(), "param_id") {
		t.Fatalf("strict decode did not reject param_id: %v", err)
	}
}

func TestDefaultPathOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "pca.json")
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", want)
	got, err := DefaultPath()
	if err != nil || got != want {
		t.Fatalf("DefaultPath=%q err=%v want=%q", got, err, want)
	}
}
