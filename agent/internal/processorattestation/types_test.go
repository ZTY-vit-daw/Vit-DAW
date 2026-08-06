package processorattestation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewAttestationIsDeterministicAndContainsNoExecutionMapping(t *testing.T) {
	now := time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)
	spec := fixtureSpec()
	left, err := NewAttestation(spec, now)
	if err != nil {
		t.Fatal(err)
	}
	spec.Coverage[0], spec.Coverage[1] = spec.Coverage[1], spec.Coverage[0]
	right, err := NewAttestation(spec, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if left.AttestationID != right.AttestationID || left.PayloadDigest != right.PayloadDigest {
		t.Fatalf("deterministic identity changed: left=%s right=%s", left.AttestationID, right.AttestationID)
	}
	encoded, _ := json.Marshal(left)
	for _, forbidden := range []string{"param_id", "parameter_mapping", "profile", "vps", "spal", "topology_generation"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("attestation leaked forbidden field %q: %s", forbidden, encoded)
		}
	}
}

func TestSubjectKeyPrefersExactIdentifier(t *testing.T) {
	first, err := BuildSubjectKey(Subject{Name: "Shell Member", Manufacturer: "Vendor", Format: "VST3", Identifier: "exact-member", InstalledPath: `C:\A.vst3`})
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildSubjectKey(Subject{Name: "Renamed Display", Manufacturer: "Other Label", Format: "vst3", Identifier: "exact-member", InstalledPath: `D:\Moved.vst3`})
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("identifier-bound subject key changed: %s != %s", first, second)
	}
}

func TestNewAttestationRejectsForgedSubjectKey(t *testing.T) {
	spec := fixtureSpec()
	spec.Subject.SubjectKey = "pcs1_forged"
	if _, err := NewAttestation(spec, time.Now()); err == nil {
		t.Fatal("forged subject key was accepted")
	}
}

func TestValidationRejectsUnsupportedFamiliesAndCoverage(t *testing.T) {
	spec := fixtureSpec()
	spec.ProcessorFamily = "limiter"
	if _, err := NewAttestation(spec, time.Now()); err == nil {
		t.Fatal("limiter attestation was accepted")
	}
	spec = fixtureSpec()
	spec.Coverage = []Coverage{{Action: "upsert", Shape: "dynamic_bell"}}
	if _, err := NewAttestation(spec, time.Now()); err == nil {
		t.Fatal("dynamic EQ coverage was accepted")
	}
	spec = fixtureSpec()
	spec.BinaryFingerprint = "inventory-version-1"
	if _, err := NewAttestation(spec, time.Now()); err == nil {
		t.Fatal("non-cryptographic binary fingerprint was accepted")
	}
}

func fixtureSpec() IssueSpec {
	return IssueSpec{
		Subject:           Subject{Name: "Example EQ", Manufacturer: "Example", Format: "VST3", Identifier: "example-eq", InstalledPath: `C:\VST3\Example.vst3`},
		BinaryFingerprint: "sha256:" + strings.Repeat("a", 64), ProcessorFamily: FamilyStaticEQ,
		Coverage: []Coverage{{Action: "upsert", Shape: "bell"}, {Action: "modify", Shape: "bell"}},
		Evidence: []EvidenceRef{{ReceiptID: "receipt-1", Kind: "eq_regression_receipt", SHA256: "sha256:" + strings.Repeat("b", 64),
			ObservedAt: time.Date(2026, 8, 5, 1, 2, 3, 0, time.UTC), CorpusRecord: "eq/receipt-1"}},
	}
}
