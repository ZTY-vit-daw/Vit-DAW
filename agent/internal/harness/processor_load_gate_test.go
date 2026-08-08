package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/processorattestation"
)

func TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(root, "pca-v1.json")
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", storePath)
	path := filepath.Join(root, "EQ.vst3")
	if err := os.WriteFile(path, []byte("eq-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "EQ", Manufacturer: "Vendor", Format: "VST3", Identifier: "eq-id", InstalledPath: path}
	fingerprint, _ := processorattestation.FingerprintPath(path)
	store, _ := processorattestation.NewStore(storePath)
	att, err := store.PromoteCurrent(processorattestation.IssueSpec{Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: processorattestation.FamilyStaticEQ, Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}}, Evidence: []processorattestation.EvidenceRef{loadGateEvidence()}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	key, _ := processorattestation.BuildSubjectKey(subject)
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "plugin-1"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	args := map[string]any{"track_id": "track-1", "plugin_path": path, "plugin_identifier": subject.Identifier, "plugin_name": subject.Name}
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Source: "agentloop", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "missing non-forgeable") {
		t.Fatalf("direct bypass resp=%+v err=%v", resp, err)
	}
	auth := AuthorizeProcessorSelectionLoad(nil, "track-1", path, subject.Identifier, processorattestation.EligibilityRequirement{ProcessorFamily: processorattestation.FamilyStaticEQ, RequiredCoverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}}}, key, fingerprint, att.AttestationID)
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Context: auth, Source: "agentloop", Confirmed: true})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("eligible load resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) == 0 {
		t.Fatal("eligible load did not reach Kernel")
	}

	if err := os.WriteFile(path, []byte("eq-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := len(kernel.commands)
	resp, err = h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: args, Context: auth, Source: "agentloop", Confirmed: true})
	if err == nil || !strings.Contains(resp.Error, "fingerprint changed before load") {
		t.Fatalf("TOCTOU resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) != before {
		t.Fatal("changed binary reached Kernel")
	}
}

func TestProcessorLoadGateLeavesManualPathAndPreservesCertificationException(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Unattested.vst3")
	if err := os.WriteFile(path, []byte("unattested"), 0o600); err != nil {
		t.Fatal(err)
	}
	fingerprint, _ := processorattestation.FingerprintPath(path)
	subject := processorattestation.Subject{Name: "Unattested", Format: "VST3", Identifier: "unattested-id", InstalledPath: path}
	key, _ := processorattestation.BuildSubjectKey(subject)
	args := map[string]any{"track_id": "temporary-track", "plugin_path": path, "plugin_identifier": subject.Identifier}
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}, {"status": "ok"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	// Source-less Harness calls model the non-Agent/manual transport boundary;
	// the real UI calls the C++ service directly and never enters this gate.
	if _, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Confirmed: true}); err != nil {
		t.Fatalf("manual path blocked: %v", err)
	}
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Source: "pcactl.certify_processor", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "missing non-forgeable") {
		t.Fatalf("forged certification source resp=%+v err=%v", resp, err)
	}
	auth := AuthorizeProcessorCertificationLoad(nil, "temporary-track", path, subject.Identifier, processorattestation.FamilyLimiter, key, fingerprint)
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Context: auth, Source: "pcactl.certify_processor", Confirmed: true}); err != nil || resp.Status != "ok" {
		t.Fatalf("certification exception resp=%+v err=%v", resp, err)
	}
}

func loadGateEvidence() processorattestation.EvidenceRef {
	return processorattestation.EvidenceRef{ReceiptID: "load-gate", Kind: "test", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)}
}
