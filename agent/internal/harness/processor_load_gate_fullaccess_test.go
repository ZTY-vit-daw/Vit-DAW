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

// PCA-FULLACCESS-1: under authority=full_project_access the Agent-side load gate
// must not dead-end a rack.add_node that names exactly one current PCA-admitted
// processor. The gate keeps its invariant (only a promoted, fingerprint-current
// admitted binary may reach the Kernel) but the exact selection is resolved by
// the server from the promoted PCA catalog instead of from a client payload.

type pcaFullAccessFixture struct {
	root       string
	path       string
	identifier string
	subject    processorattestation.Subject
	key        string
	fingerprint string
	attestation processorattestation.Attestation
}

func newPCAFullAccessFixture(t *testing.T, identifier string, family string) pcaFullAccessFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_V2_PATH", filepath.Join(root, "pca-v2.json"))

	bundle := filepath.Join(root, "PCA Full Access EQ.vst3")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "x86_64-win"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "x86_64-win", "PCA Full Access EQ.vst3"), []byte("pca-fullaccess-binary-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	subject := processorattestation.Subject{Name: "PCA Full Access EQ", Manufacturer: "Vendor", Format: "VST3", Identifier: identifier, InstalledPath: bundle}
	fingerprint, err := processorattestation.FingerprintPath(bundle)
	if err != nil {
		t.Fatal(err)
	}
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	attestation, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: subject, BinaryFingerprint: fingerprint, ProcessorFamily: family,
		Coverage: []processorattestation.Coverage{{Action: "upsert", Shape: "bell"}},
		Evidence: []processorattestation.EvidenceRef{loadGateEvidence()},
	}, "pca-fullaccess-test")
	if err != nil {
		t.Fatal(err)
	}
	key, err := processorattestation.BuildSubjectKey(subject)
	if err != nil {
		t.Fatal(err)
	}
	return pcaFullAccessFixture{root: root, path: bundle, identifier: identifier, subject: subject, key: key, fingerprint: fingerprint, attestation: attestation}
}

func rackAddNodeCommands(commands []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(commands))
	for _, command := range commands {
		if strings.EqualFold(strings.TrimSpace(firstString(command, "cmd")), "rack_add_node") {
			out = append(out, command)
		}
	}
	return out
}

func fullProjectAccessContext() map[string]any {
	return map[string]any{"authority_mode": "full_project_access", "authority_mode_explicit": true}
}


func TestFullProjectAccessLoadsAdmittedProcessorWithoutUserSelectionAuthorization(t *testing.T) {
	const identifier = "VST3-PCA-FullAccess-Probe-1"
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	args := map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_identifier": identifier, "x": 0.0, "y": 0.0, "zone_id": "Z3"}

	manualKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "p1"}}}
	manual := New(nil, nil, nil)
	manual.kernel = manualKernel
	manualResponse, manualErr := manual.Invoke(context.Background(), InvokeRequest{Tool: "rack_add_node", Args: args, Context: map[string]any{"authority_mode": "manual_confirmation", "authority_mode_explicit": true}, Source: "agentloop", Confirmed: true})
	if manualErr == nil || !strings.Contains(manualResponse.Error, "missing non-forgeable exact selection authorization") {
		t.Fatalf("manual mode must stay byte-identical: resp=%+v err=%v", manualResponse, manualErr)
	}
	if len(manualKernel.commands) != 0 {
		t.Fatalf("manual mode load reached Kernel: %+v", manualKernel.commands)
	}

	fullKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "p2"}}}
	full := New(nil, nil, nil)
	full.kernel = fullKernel
	fullResponse, fullErr := full.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: args, Context: fullProjectAccessContext(), Source: "agentloop", Confirmed: true})
	if fullErr != nil || fullResponse.Status != "ok" {
		t.Fatalf("full access admitted load resp=%+v err=%v", fullResponse, fullErr)
	}
	if len(fullKernel.commands) != 1 {
		t.Fatalf("full access admitted load did not reach Kernel exactly once: %+v", fullKernel.commands)
	}
	dispatched := fullKernel.commands[0]
	if got := strings.TrimSpace(firstString(dispatched, "plugin_path")); got != fixture.path {
		t.Fatalf("dispatched plugin_path = %q, want the exact PCA-admitted installed path %q", got, fixture.path)
	}
	if got := strings.TrimSpace(firstString(dispatched, "plugin_identifier")); got != identifier {
		t.Fatalf("dispatched plugin_identifier = %q, want %q", got, identifier)
	}
}

func TestFullProjectAccessLoadStillRejectsUnadmittedOrForgedIdentity(t *testing.T) {
	const identifier = "VST3-PCA-FullAccess-Probe-2"
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)

	expectDenied := func(t *testing.T, h *Harness, kernel *fakeKernelClient, args map[string]any, want string) {
		t.Helper()
		resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: args, Context: fullProjectAccessContext(), Source: "agentloop", Confirmed: true})
		if err == nil || !strings.Contains(resp.Error, want) {
			t.Fatalf("expected denial %q, got resp=%+v err=%v", want, resp, err)
		}
		if dispatched := rackAddNodeCommands(kernel.commands); len(dispatched) != 0 {
			t.Fatalf("denied load reached Kernel: %+v", dispatched)
		}
	}

	unadmittedKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	unadmitted := New(nil, nil, nil)
	unadmitted.kernel = unadmittedKernel
	expectDenied(t, unadmitted, unadmittedKernel,
		map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_identifier": "VST3-Not-Admitted-Anywhere"},
		"no_promoted_current_pca_admission_for_identifier")

	// A caller-pinned path that no promoted record owns must never be silently
	// reassigned to the admitted path.
	pinnedKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	pinned := New(nil, nil, nil)
	pinned.kernel = pinnedKernel
	expectDenied(t, pinned, pinnedKernel,
		map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_path": filepath.Join(fixture.root, "Third-Party Path.vst3"), "plugin_identifier": identifier},
		"no_promoted_current_pca_admission_for_identifier")

	// One identifier promoted in two admitted families against two different
	// installed binaries makes the identifier-only selection ambiguous: the gate
	// must fail closed instead of picking a favorite.
	secondPath := filepath.Join(fixture.root, "PCA Full Access Other.vst3")
	if err := os.WriteFile(secondPath, []byte("pca-fullaccess-binary-other"), 0o600); err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := processorattestation.FingerprintPath(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	storeV2, err := processorattestation.NewStoreV2("")
	if err != nil {
		t.Fatal(err)
	}
	secondSubject := fixture.subject
	secondSubject.InstalledPath = secondPath
	if _, err := storeV2.PromoteCurrent(processorattestation.IssueSpecV2{
		Subject: secondSubject, BinaryFingerprint: secondFingerprint, ProcessorFamily: processorattestation.FamilyLimiter,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "output_ceiling"}},
		Evidence: []processorattestation.EvidenceRef{loadGateEvidence()},
	}, "pca-fullaccess-test-second-family"); err != nil {
		t.Fatal(err)
	}
	ambiguousKernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	ambiguous := New(nil, nil, nil)
	ambiguous.kernel = ambiguousKernel
	expectDenied(t, ambiguous, ambiguousKernel,
		map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_identifier": identifier},
		"one exact processor identity")
}

func TestFullProjectAccessLoadRejectsBinaryThatNoLongerMatchesItsAdmission(t *testing.T) {
	const identifier = "VST3-PCA-FullAccess-Probe-2b"
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	binary := filepath.Join(fixture.path, "Contents", "x86_64-win", "PCA Full Access EQ.vst3")
	if err := os.WriteFile(binary, []byte("pca-fullaccess-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{
		Tool: "rack.add_node", Source: "agentloop", Confirmed: true,
		Args:    map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_identifier": identifier},
		Context: fullProjectAccessContext(),
	})
	if err == nil || !strings.Contains(resp.Error, "pca_load_gate:") {
		t.Fatalf("changed binary under full access resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("changed binary reached Kernel: %+v", kernel.commands)
	}
}

func TestFullProjectAccessLoadFailsClosedOnAmbiguousCatalogIdentity(t *testing.T) {
	const identifier = "VST3-PCA-FullAccess-Probe-3"
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: fixture.subject, BinaryFingerprint: fixture.fingerprint, ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}},
		Evidence: []processorattestation.EvidenceRef{loadGateEvidence()},
	}, "pca-fullaccess-test-ambiguous"); err != nil {
		t.Fatal(err)
	}
	h := New(nil, nil, nil)
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	h.kernel = kernel
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: map[string]any{"cmd": "rack_add_node", "track_id": "1007", "plugin_identifier": identifier}, Context: fullProjectAccessContext(), Source: "agentloop", Confirmed: true})
	if err == nil || !strings.Contains(resp.Error, "pca_load_gate:") {
		t.Fatalf("ambiguous catalog identity must fail closed: resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("ambiguous identity reached Kernel: %+v", kernel.commands)
	}
}

func TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization(t *testing.T) {
	const (
		trackID    = "1007"
		identifier = "VST3-PCA-FullAccess-Probe-4"
	)
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "p5"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel
	authorized := AuthorizeProcessorSelectionLoad(fullProjectAccessContext(), trackID, fixture.path, identifier,
		processorattestation.EligibilityRequirement{ProcessorFamily: processorattestation.FamilyStaticEQ},
		fixture.key, fixture.fingerprint, fixture.attestation.AttestationID)
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": fixture.path, "plugin_identifier": identifier}, Context: authorized, Source: "agentloop", Confirmed: true})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("existing selection authorization regressed: resp=%+v err=%v", resp, err)
	}

	// A forged concrete authorization value must still be compared exactly.
	other := AuthorizeProcessorSelectionLoad(fullProjectAccessContext(), "9999", fixture.path, identifier,
		processorattestation.EligibilityRequirement{ProcessorFamily: processorattestation.FamilyStaticEQ},
		fixture.key, fixture.fingerprint, fixture.attestation.AttestationID)
	before := len(kernel.commands)
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Args: map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": fixture.path, "plugin_identifier": identifier}, Context: other, Source: "agentloop", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "exact identifier, path, or track changed after authorization") {
		t.Fatalf("mismatched authorization was not compared exactly: resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) != before {
		t.Fatalf("mismatched authorization reached Kernel: %+v", kernel.commands)
	}
}

var _ = time.Now
