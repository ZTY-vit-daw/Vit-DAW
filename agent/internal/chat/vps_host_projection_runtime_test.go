package chat

import (
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/vps"
)

func TestVPSRuntimeUsesObservedVitProjectionWithoutStalingIndependentCredential(t *testing.T) {
	independent := vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:adapter-surface", DisplaySurface: "sha256:adapter-display"}
	vitProjection := vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:vit-surface", DisplaySurface: "sha256:vit-display"}
	document := vps.VPSDocument{HostProjectionBindings: []vps.HostProjectionBinding{{
		SchemaVersion: vps.HostProjectionBindingSchemaVersion, HostID: vps.VitHostProjectionID, Fingerprint: vitProjection,
		RequiredParameterIDs: []string{"0"}, EvidenceRefs: []string{"vpsforge:vit_host_binding.json"}, Trust: "observed", Status: "observed_host_projection_compatible",
	}}}
	credential := vps.ProviderCredential{PluginFingerprint: independent}
	if matched, reason, stale := vpsRuntimeCredentialMatchesCurrentVitHost(document, credential, vitProjection); !matched || reason != "" || stale {
		t.Fatalf("Vit projection guard must match without staling credential: matched=%v reason=%q stale=%v", matched, reason, stale)
	}
	changedProjection := vitProjection
	changedProjection.ParameterSurface = "sha256:vit-surface-changed"
	if matched, reason, stale := vpsRuntimeCredentialMatchesCurrentVitHost(document, credential, changedProjection); matched || reason != "vit_host_projection_binding_mismatch" || stale {
		t.Fatalf("host projection change must block but not stale independent credential: matched=%v reason=%q stale=%v", matched, reason, stale)
	}
	changedInstallation := vitProjection
	changedInstallation.Installation = "sha256:new-plugin"
	if matched, reason, stale := vpsRuntimeCredentialMatchesCurrentVitHost(document, credential, changedInstallation); matched || reason != "provider_installation_fingerprint_stale" || !stale {
		t.Fatalf("installation change must stale credential: matched=%v reason=%q stale=%v", matched, reason, stale)
	}
}

func TestVPSRuntimeInstallationFingerprintUsesAdapterCompatibleFileHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Observed EQ.vst3")
	if err := os.WriteFile(path, []byte("adapter-compatible-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	want, err := vps.BuildFileFingerprint(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := vpsRuntimeInstallationFingerprint(pluginParameterDigest{PluginIdentity: map[string]any{"plugin_path": path}})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("runtime installation fingerprint = %q, want adapter-compatible %q", got, want)
	}
}
