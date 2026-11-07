package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/processorattestation"
)

// ARRLOOP-AUTHBRIDGE-1: AuthorizeConfirmedAgentLoopLoad is the confirm-phase
// authorization production face for capability-wheel load actions. These tests
// pin both directions of the bridge contract at the harness boundary: a
// server-minted stamp for a promoted identity passes the unchanged gate, and
// an unstamped or unresolvable load stays blocked.

func TestAuthorizeConfirmedAgentLoopLoadPassesGateForPromotedIdentity(t *testing.T) {
	const (
		trackID    = "1007"
		identifier = "VST3-AgentLoop-AuthBridge-1"
	)
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)

	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "bridge-1"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	// Negative pin first: without a stamp the agent-loop load still dies at
	// the unchanged gate (277 smokerun1 shape).
	if _, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Source: "agentloop", Confirmed: true,
		Args: map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": fixture.path, "plugin_identifier": identifier}}); err == nil || !strings.Contains(err.Error(), "missing non-forgeable exact selection authorization") {
		t.Fatalf("unstamped agent-loop load must stay blocked at the gate, err=%v", err)
	}
	if dispatched := rackAddNodeCommands(kernel.commands); len(dispatched) != 0 {
		t.Fatalf("unstamped load reached Kernel: %+v", dispatched)
	}

	stamped, admittedPath, err := AuthorizeConfirmedAgentLoopLoad(map[string]any{"user_message": "给这条 MIDI 轨加一个钢琴音源"}, trackID, fixture.path, identifier)
	if err != nil {
		t.Fatalf("confirmed load authorization failed: %v", err)
	}
	if admittedPath != fixture.path {
		t.Fatalf("admitted path = %q, want %q", admittedPath, fixture.path)
	}
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Source: "agentloop", Confirmed: true,
		Args:    map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": fixture.path, "plugin_identifier": identifier},
		Context: stamped})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("confirmed stamped load should pass the gate, resp=%+v err=%v", resp, err)
	}
	dispatched := rackAddNodeCommands(kernel.commands)
	if len(dispatched) != 1 {
		t.Fatalf("stamped load did not reach Kernel exactly once: %+v", kernel.commands)
	}
	if got := strings.TrimSpace(firstString(dispatched[0], "plugin_path")); got != fixture.path {
		t.Fatalf("dispatched plugin_path = %q, want the admitted path %q", got, fixture.path)
	}
	if got := strings.TrimSpace(firstString(dispatched[0], "plugin_identifier")); got != identifier {
		t.Fatalf("dispatched plugin_identifier = %q, want %q", got, identifier)
	}
}

func TestAuthorizeConfirmedAgentLoopLoadPinsAdmittedPathForSeparatorVariants(t *testing.T) {
	const (
		trackID    = "1007"
		identifier = "VST3-AgentLoop-AuthBridge-2"
	)
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "bridge-2"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	// The model may name the same installed identity with forward slashes; the
	// mint resolves it and returns the admitted install path, and dispatching
	// the admitted path (the chat bridge pins it) passes the gate unchanged.
	stamped, admittedPath, err := AuthorizeConfirmedAgentLoopLoad(nil, trackID, filepath.ToSlash(fixture.path), identifier)
	if err != nil {
		t.Fatalf("separator-variant selection failed to resolve: %v", err)
	}
	if admittedPath != fixture.path {
		t.Fatalf("admitted path = %q, want %q", admittedPath, fixture.path)
	}
	resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Source: "agentloop", Confirmed: true,
		Args:    map[string]any{"cmd": "rack_add_node", "track_id": trackID, "plugin_path": admittedPath, "plugin_identifier": identifier},
		Context: stamped})
	if err != nil || resp.Status != "ok" {
		t.Fatalf("pinned admitted-path dispatch should pass, resp=%+v err=%v", resp, err)
	}
}

func TestAuthorizeConfirmedAgentLoopLoadFailsClosed(t *testing.T) {
	const (
		trackID    = "1007"
		identifier = "VST3-AgentLoop-AuthBridge-3"
	)
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)

	if _, _, err := AuthorizeConfirmedAgentLoopLoad(nil, "", fixture.path, identifier); err == nil || !strings.Contains(err.Error(), "requires an exact track_id, plugin_path, and plugin_identifier") {
		t.Fatalf("missing track must fail closed, err=%v", err)
	}
	if _, _, err := AuthorizeConfirmedAgentLoopLoad(nil, trackID, fixture.path, "VST3-Not-Admitted-Anywhere"); err == nil || !strings.Contains(err.Error(), "no current promoted admission") {
		t.Fatalf("unadmitted selection must fail closed, err=%v", err)
	}

	// One identifier+path promoted in two families is ambiguous even with the
	// path pinned: fail closed instead of picking a favorite.
	store, err := processorattestation.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PromoteCurrent(processorattestation.IssueSpec{
		Subject: fixture.subject, BinaryFingerprint: fixture.fingerprint, ProcessorFamily: processorattestation.FamilyBroadbandCompressor,
		Coverage: []processorattestation.Coverage{{Action: "adjust", Axis: "activation_intensity"}},
		Evidence: []processorattestation.EvidenceRef{loadGateEvidence()},
	}, "authbridge-test-ambiguous"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AuthorizeConfirmedAgentLoopLoad(nil, trackID, fixture.path, identifier); err == nil || !strings.Contains(err.Error(), "one exact processor identity") {
		t.Fatalf("ambiguous identity must fail closed, err=%v", err)
	}

	// A stale binary (admission fingerprint no longer matches the file on
	// disk) must not mint an authorization.
	staleFixture := newPCAFullAccessFixture(t, "VST3-AgentLoop-AuthBridge-4", processorattestation.FamilyStaticEQ)
	binary := filepath.Join(staleFixture.path, "Contents", "x86_64-win", "PCA Full Access EQ.vst3")
	if err := os.WriteFile(binary, []byte("authbridge-binary-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := AuthorizeConfirmedAgentLoopLoad(nil, trackID, staleFixture.path, "VST3-AgentLoop-AuthBridge-4"); err == nil || !strings.Contains(err.Error(), "no current promoted admission") {
		t.Fatalf("stale binary must fail closed, err=%v", err)
	}
}

func TestAuthorizeConfirmedAgentLoopLoadStampBindsExactTrack(t *testing.T) {
	const identifier = "VST3-AgentLoop-AuthBridge-5"
	fixture := newPCAFullAccessFixture(t, identifier, processorattestation.FamilyStaticEQ)
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	stamped, _, err := AuthorizeConfirmedAgentLoopLoad(nil, "1007", fixture.path, identifier)
	if err != nil {
		t.Fatal(err)
	}
	// A dispatch that swaps the track after confirmation is a changed selection:
	// the unchanged gate rejects it.
	if _, err := h.Invoke(context.Background(), InvokeRequest{Tool: "rack.add_node", Source: "agentloop", Confirmed: true,
		Args:    map[string]any{"cmd": "rack_add_node", "track_id": "9999", "plugin_path": fixture.path, "plugin_identifier": identifier},
		Context: stamped}); err == nil || !strings.Contains(err.Error(), "exact identifier, path, or track changed after authorization") {
		t.Fatalf("track swap after confirmation must be rejected, err=%v", err)
	}
	if dispatched := rackAddNodeCommands(kernel.commands); len(dispatched) != 0 {
		t.Fatalf("track-swapped load reached Kernel: %+v", dispatched)
	}
}
