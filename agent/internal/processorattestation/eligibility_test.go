package processorattestation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQueryInstalledEligibilityV1AndV2Families(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "v2.json"))
	v1, _ := NewStore("")
	v2, _ := NewStoreV2("")
	tests := []struct {
		family   string
		coverage Coverage
		v2       bool
	}{
		{FamilyStaticEQ, Coverage{Action: "upsert", Shape: "bell"}, false},
		{FamilyBroadbandCompressor, Coverage{Action: "adjust", Axis: "activation_intensity"}, false},
		{FamilyLimiter, Coverage{Action: "adjust", Axis: "protection_intensity"}, true},
		{FamilyGateExpander, Coverage{Action: "adjust", Axis: "activation_threshold"}, true},
		{FamilyDeEsser, Coverage{Action: "adjust", Axis: "sibilance_reduction"}, true},
		{FamilyTransient, Coverage{Action: "adjust", Axis: "envelope_emphasis"}, true},
		{FamilyMultiband, Coverage{Action: "adjust", Axis: "band_dynamics"}, true},
	}
	for index, test := range tests {
		path := filepath.Join(root, test.family+".vst3")
		if err := os.WriteFile(path, []byte(test.family), 0o600); err != nil {
			t.Fatal(err)
		}
		subject := Subject{Name: test.family, Manufacturer: "Vendor", Format: "VST3", Identifier: "id-" + test.family, InstalledPath: path}
		fingerprint, _ := FingerprintPath(path)
		if test.v2 {
			_, _ = v2.PromoteCurrent(IssueSpecV2{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: test.family, Coverage: []Coverage{test.coverage}, Evidence: []EvidenceRef{eligibilityEvidence(index)}}, "test")
		} else {
			_, _ = v1.PromoteCurrent(IssueSpec{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: test.family, Coverage: []Coverage{test.coverage}, Evidence: []EvidenceRef{eligibilityEvidence(index)}}, "test")
		}
		result, err := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: test.family, RequiredCoverage: []Coverage{test.coverage}})
		if err != nil || !result.Eligible || result.AttestationID == "" || result.SubjectKey == "" {
			t.Fatalf("%s result=%+v err=%v", test.family, result, err)
		}
	}
}

func TestQueryInstalledEligibilityFailsClosed(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "v2.json"))
	path := filepath.Join(root, "Limiter.vst3")
	if err := os.WriteFile(path, []byte("limiter-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := Subject{Name: "Limiter", Manufacturer: "Vendor", Format: "VST3", Identifier: "limiter-id", InstalledPath: path}
	requirement := EligibilityRequirement{ProcessorFamily: FamilyLimiter, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "output_ceiling"}}}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, requirement); result.Eligible || result.Reason != "no_attestation" {
		t.Fatalf("missing=%+v", result)
	}
	store, _ := NewStoreV2("")
	fingerprint, _ := FingerprintPath(path)
	issued, err := store.Issue(IssueSpecV2{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter, Coverage: []Coverage{{Action: "adjust", Axis: "output_ceiling"}}, Evidence: []EvidenceRef{eligibilityEvidence(10)}})
	if err != nil {
		t.Fatal(err)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, requirement); result.Eligible || result.Reason != "attestation_issued" {
		t.Fatalf("issued=%+v", result)
	}
	if _, err := store.Promote(issued.AttestationID, "test"); err != nil {
		t.Fatal(err)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: FamilyLimiter, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "peak_mode"}}}); result.Eligible || result.Reason != "required_action_not_covered" {
		t.Fatalf("coverage=%+v", result)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: FamilyDeEsser, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "sibilance_reduction"}}}); result.Eligible || result.Reason != "no_attestation" {
		t.Fatalf("family=%+v", result)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: "clipper", RequiredCoverage: []Coverage{{Action: "adjust", Axis: "output_ceiling"}}}); result.Eligible || !strings.HasPrefix(result.Reason, "unsupported_processor_family") {
		t.Fatalf("clipper=%+v", result)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: "spectral_dynamics", RequiredCoverage: []Coverage{{Action: "adjust", Axis: "band_dynamics"}}}); result.Eligible || !strings.HasPrefix(result.Reason, "unsupported_processor_family") {
		t.Fatalf("spectral=%+v", result)
	}
	if err := os.WriteFile(path, []byte("limiter-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, requirement); result.Eligible || result.Reason != "binary_fingerprint_changed" {
		t.Fatalf("fingerprint=%+v", result)
	}
	if _, err := store.MarkStale(issued.AttestationID, "stale"); err == nil { // promoted -> stale is valid
		if err := os.WriteFile(path, []byte("limiter-v1"), 0o600); err != nil {
			t.Fatal(err)
		}
		if result, _ := QueryInstalled(InstalledSubject{Subject: subject}, requirement); result.Eligible || result.Reason != "attestation_stale" {
			t.Fatalf("stale=%+v", result)
		}
	}
}

func TestQueryInstalledRevokedRejected(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "v2.json"))
	path := filepath.Join(root, "Gate.vst3")
	_ = os.WriteFile(path, []byte("gate"), 0o600)
	subject := Subject{Name: "Gate", Format: "VST3", Identifier: "gate-id", InstalledPath: path}
	fingerprint, _ := FingerprintPath(path)
	store, _ := NewStoreV2("")
	att, _ := store.PromoteCurrent(IssueSpecV2{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyGateExpander, Coverage: []Coverage{{Action: "adjust", Axis: "activation_threshold"}}, Evidence: []EvidenceRef{eligibilityEvidence(20)}}, "test")
	_, _ = store.Revoke(att.AttestationID, "revoked")
	result, _ := QueryInstalled(InstalledSubject{Subject: subject}, EligibilityRequirement{ProcessorFamily: FamilyGateExpander, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "activation_threshold"}}})
	if result.Eligible || result.Reason != "attestation_revoked" {
		t.Fatalf("revoked=%+v", result)
	}
}

func eligibilityEvidence(index int) EvidenceRef {
	return EvidenceRef{ReceiptID: "receipt-" + string(rune('a'+index)), Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)}
}
