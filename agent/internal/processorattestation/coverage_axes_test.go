package processorattestation

import (
	"os"
	"strings"
	"testing"
	"time"
)

// PCA-1 bridge pin: the certified-coverage query is the domain-authoritative
// read side of the load receipt — only a promoted attestation proves what the
// exact binary is certified to control.
func TestPromotedAttestationCoverageAxesReturnsPromotedCertificateAxes(t *testing.T) {
	path := t.TempDir() + "\\Bridge.vst3"
	if err := os.WriteFile(path, []byte("coverage-axes-bridge-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", t.TempDir()+"\\attestations.v1.json")
	fingerprint, err := FingerprintPath(path)
	if err != nil {
		t.Fatal(err)
	}
	subject := Subject{Name: "Bridge Compressor", Format: "VST3", Identifier: "bridge-comp-v1", InstalledPath: path}
	store, err := NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := store.PromoteCurrent(IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyBroadbandCompressor,
		Coverage: []Coverage{{Action: "adjust", Axis: "transfer_severity"}, {Action: "adjust", Axis: "activation_intensity"},
			{Action: "adjust", Axis: "activation_intensity"}},
		Evidence: []EvidenceRef{{ReceiptID: "coverage-axes-bridge", Kind: "test", SHA256: "sha256:" + strings.Repeat("b", 64), ObservedAt: time.Now().UTC()}},
	}, "coverage_axes_bridge_test")
	if err != nil {
		t.Fatal(err)
	}
	axes, err := PromotedAttestationCoverageAxes(attestation.AttestationID)
	if err != nil || strings.Join(axes, ",") != "activation_intensity,transfer_severity" {
		t.Fatalf("promoted certificate axes=%v err=%v", axes, err)
	}
	if unknown, err := PromotedAttestationCoverageAxes("pca1_unknown"); err != nil || len(unknown) != 0 {
		t.Fatalf("unknown attestation must certify nothing: axes=%v err=%v", unknown, err)
	}
	issued, err := store.Issue(IssueSpec{
		Subject: Subject{Name: "Bridge Compressor Two", Format: "VST3", Identifier: "bridge-comp-v2", InstalledPath: path},
		BinaryFingerprint: fingerprint, ProcessorFamily: FamilyBroadbandCompressor,
		Coverage: []Coverage{{Action: "adjust", Axis: "transient_timing"}},
		Evidence: []EvidenceRef{{ReceiptID: "coverage-axes-issued", Kind: "test", SHA256: "sha256:" + strings.Repeat("c", 64), ObservedAt: time.Now().UTC()}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if axes, err := PromotedAttestationCoverageAxes(issued.AttestationID); err != nil || len(axes) != 0 {
		t.Fatalf("issued-only attestation must not prove certified axes: axes=%v err=%v", axes, err)
	}
}
