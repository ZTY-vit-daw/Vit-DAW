package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
)

// FIX-PCA-CERTAUTH-TOKEN-1: the static_eq certification channel runs outside
// the in-agent runner, and its subjects cannot resolve against the promoted
// PCA catalog before certification (the full-access path would require exactly
// that promotion). A locally armed token — environment variable or token file
// — is the operator's act that mints the certification-surface authorization
// for those exact loads.
func writeCertAuthTokenSemanticsIndex(t *testing.T, path string, entry pluginsemantics.Entry) {
	t.Helper()
	index := pluginsemantics.Index{SchemaVersion: pluginsemantics.SchemaVersion, BuiltAt: time.Now().UTC(), Entries: []pluginsemantics.Entry{entry}}
	encoded, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCertificationTokenArmsUnpromotedStaticEQCertificationLoad(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	semanticsPath := filepath.Join(root, "semantics.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	// isolate the token-file probe from the operator's real ~/.vit arming file
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)

	path := filepath.Join(root, "EQ.vst3")
	if err := os.WriteFile(path, []byte("eq-certauth"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := pluginsemantics.Entry{ID: "eq-1", Name: "EQ", Manufacturer: "Vendor", Format: "VST3",
		Identifier: "eq-certauth-id", PluginPath: path, UpdatedAt: time.Now().UTC()}
	writeCertAuthTokenSemanticsIndex(t, semanticsPath, entry)

	args := map[string]any{"track_id": "temporary-track", "plugin_path": path, "plugin_identifier": entry.Identifier}
	fullAccess := map[string]any{"authority_mode": "full_project_access", "authority_mode_explicit": true}
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "plugin_id": "plugin-1"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	// Unarmed token: the circular full-access rejection stands unchanged.
	t.Setenv("VIT_PCA_CERTAUTH_TOKEN", "")
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Context: fullAccess, Source: "http", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "autonomous full-access load rejected") {
		t.Fatalf("unarmed unpromoted load resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) != 0 {
		t.Fatal("unarmed unpromoted load reached Kernel")
	}

	// Armed token: the exact unpromoted load is authorized on the
	// certification surface and reaches the Kernel.
	t.Setenv("VIT_PCA_CERTAUTH_TOKEN", "operator-armed")
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Context: fullAccess, Source: "http", Confirmed: true}); err != nil || resp.Status != "ok" {
		t.Fatalf("armed certification load resp=%+v err=%v", resp, err)
	}
	if len(kernel.commands) == 0 {
		t.Fatal("armed certification load did not reach Kernel")
	}
}

func TestCertificationTokenDoesNotWeakenOtherLoadGatePaths(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_PROCESSOR_ATTESTATIONS_PATH", filepath.Join(root, "pca-v1.json"))
	semanticsPath := filepath.Join(root, "semantics.json")
	t.Setenv("VIT_PLUGIN_SEMANTICS_PATH", semanticsPath)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)

	path := filepath.Join(root, "Unattested.vst3")
	if err := os.WriteFile(path, []byte("unattested"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := pluginsemantics.Entry{ID: "unattested-1", Name: "Unattested", Format: "VST3",
		Identifier: "unattested-id", PluginPath: path, UpdatedAt: time.Now().UTC()}
	writeCertAuthTokenSemanticsIndex(t, semanticsPath, entry)

	// Armed throughout: the token must not change any other gate decision.
	t.Setenv("VIT_PCA_CERTAUTH_TOKEN", "operator-armed")
	args := map[string]any{"track_id": "track-1", "plugin_path": path, "plugin_identifier": entry.Identifier}
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}, {"status": "ok"}}}
	h := New(nil, nil, nil)
	h.kernel = kernel

	// Forged certification source without a minted authorization stays closed.
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Source: "pcactl.certify_processor", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "missing non-forgeable") {
		t.Fatalf("forged certification source resp=%+v err=%v", resp, err)
	}

	// Source-less manual transport keeps bypassing the gate entirely.
	if _, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Confirmed: true}); err != nil {
		t.Fatalf("manual path blocked: %v", err)
	}

	// Full access without authority_mode stays closed (the token is not a
	// standing authority grant).
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: args, Source: "http", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "missing non-forgeable") {
		t.Fatalf("token granted authority resp=%+v err=%v", resp, err)
	}

	// Armed token with an identifier the semantic library does not know keeps
	// the original full-access rejection (fail closed, no wider error).
	unknown := map[string]any{"track_id": "track-1", "plugin_path": path, "plugin_identifier": "not-indexed"}
	fullAccess := map[string]any{"authority_mode": "full_project_access", "authority_mode_explicit": true}
	if resp, err := h.Invoke(context.Background(), InvokeRequest{Tool: "plugin.load_to_rack", Args: unknown, Context: fullAccess, Source: "http", Confirmed: true}); err == nil || !strings.Contains(resp.Error, "autonomous full-access load rejected") {
		t.Fatalf("unknown identifier armed load resp=%+v err=%v", resp, err)
	}
}
