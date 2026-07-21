package vpsforge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

func TestProQ3EqualizerV2PromotionDocumentSeparatesIdentityAndVitProjection(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	independent := vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:adapter-surface", DisplaySurface: "sha256:adapter-display"}
	source := vps.NewDraft(vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0", Fingerprint: independent}, now)
	for id := range map[string]bool{"0": true, "1": true, "2": true, "3": true, "7": true, "8": true, "9": true} {
		source.ControlSurface.Mappings = append(source.ControlSurface.Mappings, vps.ControlSurfaceMapping{ParameterID: id, Label: "observed"})
	}
	host := vps.HostProjectionBinding{
		SchemaVersion: vps.HostProjectionBindingSchemaVersion, HostID: vps.VitHostProjectionID,
		Fingerprint:          vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:vit-surface", DisplaySurface: "sha256:vit-display"},
		RequiredParameterIDs: []string{"0", "1", "2", "3", "7", "8", "9"}, EvidenceRefs: []string{"vpsforge:vit_host_binding.json"}, Trust: "observed", Status: "observed_host_projection_compatible", CapturedAt: now,
	}
	evidence := []string{"vpsforge:pro_q_3_eq_core_probe_results.json", "vpsforge:vit_host_binding.json", "vpsforge:archived/human_witness_evidence.json"}
	candidate, credentialID, err := proQ3EqualizerV2PromotionDocument(source, host, evidence, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("promotion candidate must validate: %v", err)
	}
	if candidate.PluginIdentity.Fingerprint != independent || len(candidate.HostProjectionBindings) != 1 || candidate.HostProjectionBindings[0].Fingerprint.ParameterSurface != "sha256:vit-surface" {
		t.Fatalf("identity and host projection were mixed: %#v", candidate)
	}
	if len(candidate.ProviderCredentials) != 1 || !candidate.ProviderCredentials[0].Dispatchable() || candidate.ProviderCredentials[0].ID != credentialID {
		t.Fatalf("candidate credential missing: %#v", candidate.ProviderCredentials)
	}
	conformance := candidate.ProviderCredentials[0].Conformance
	if conformance.EQV2Binding == nil || len(conformance.ConformedSchemas) != 2 || conformance.EQV2Binding.Bands["b1"].ResponseShape.Values["bell"] != 0 || conformance.EQV2Binding.HighPass == nil || conformance.EQV2Binding.LowPass == nil {
		t.Fatalf("candidate does not persist the bounded EQ v2 action binding: %#v", conformance)
	}
	hasSchema := func(want string) bool {
		for _, schema := range conformance.ConformedSchemas {
			if schema == want {
				return true
			}
		}
		return false
	}
	if !hasSchema(spal.EQBandPatchControlID) || !hasSchema(spal.EQPassFilterPatchControlID) {
		t.Fatalf("candidate schema matrix = %#v", conformance.ConformedSchemas)
	}
	for _, mapping := range candidate.ControlSurface.Mappings {
		if mapping.ParameterID == "8" && mapping.Confirmed {
			t.Fatalf("observed host mapping was incorrectly promoted to independently confirmed: %#v", mapping)
		}
	}
}

func TestProQ3PromotionStagingDeactivationOnlyChangesMatchingBridge(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "r4")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	stagingArtifact := filepath.Join(root, proQ3StaticEQStagingVPSFile)
	if err := os.WriteFile(stagingArtifact, []byte(`{"schema_version":"test"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	activationPath := filepath.Join(filepath.Dir(filepath.Dir(root)), proQ3StaticEQStagingActivationFile)
	activation := map[string]any{
		"schema_version":   proQ3StaticEQStagingActivationSchema,
		"enabled":          true,
		"staging_vps_path": filepath.ToSlash(filepath.Join("preflight", "r4", proQ3StaticEQStagingVPSFile)),
		"status":           "staging_vps_implemented_not_conformed",
		"unrelated_field":  "must_survive",
	}
	if err := writeJSON(activationPath, activation); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	plan, err := proQ3PromotionPrepareStagingDeactivation(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || !plan.WasEnabled || plan.Path != activationPath {
		t.Fatalf("matching bridge plan = %#v", plan)
	}
	if err := plan.apply(); err != nil {
		t.Fatal(err)
	}
	var disabled map[string]any
	if err := readJSON(activationPath, &disabled); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := disabled["enabled"].(bool); enabled || disabled["status"] != "superseded_by_local_catalog" || disabled["unrelated_field"] != "must_survive" {
		t.Fatalf("disabled activation = %#v", disabled)
	}
	if _, found := disabled["superseded_by"]; !found {
		t.Fatalf("disabled activation has no promotion provenance: %#v", disabled)
	}
	if err := plan.restore(); err != nil {
		t.Fatal(err)
	}
	var restored map[string]any
	if err := readJSON(activationPath, &restored); err != nil {
		t.Fatal(err)
	}
	if enabled, _ := restored["enabled"].(bool); !enabled || restored["status"] != "staging_vps_implemented_not_conformed" {
		t.Fatalf("restored activation = %#v", restored)
	}

	activation["staging_vps_path"] = filepath.ToSlash(filepath.Join("preflight", "other", proQ3StaticEQStagingVPSFile))
	if err := writeJSON(activationPath, activation); err != nil {
		t.Fatal(err)
	}
	plan, err = proQ3PromotionPrepareStagingDeactivation(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("unrelated staging pointer must not be modified: %#v", plan)
	}
}

func TestProQ3PromotionFalseStaleRecoveryIssuesSeparateCredential(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	independent := vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:adapter-surface", DisplaySurface: "sha256:adapter-display"}
	source := vps.NewDraft(vps.PluginIdentity{Manufacturer: "FabFilter", Name: "Pro-Q 3", Format: "VST3", Version: "3.2.3.0", InstallPath: `C:\\VST3\\Pro-Q 3.vst3`, Fingerprint: independent}, now)
	for _, id := range []string{"0", "1", "2", "3", "7", "8", "9"} {
		source.ControlSurface.Mappings = append(source.ControlSurface.Mappings, vps.ControlSurfaceMapping{ParameterID: id, Label: "observed"})
	}
	host := vps.HostProjectionBinding{
		SchemaVersion: vps.HostProjectionBindingSchemaVersion, HostID: vps.VitHostProjectionID,
		Fingerprint:          vps.PluginFingerprint{Installation: "sha256:plugin", ParameterSurface: "sha256:vit-surface", DisplaySurface: "sha256:vit-display"},
		RequiredParameterIDs: []string{"0", "1", "2", "3", "7", "8", "9"}, EvidenceRefs: []string{"vpsforge:vit_host_binding.json"}, Trust: "observed", Status: "observed_host_projection_compatible", CapturedAt: now,
	}
	candidate, credentialID, err := proQ3EqualizerV2PromotionDocument(source, host, []string{"vpsforge:vit_host_binding.json"}, now)
	if err != nil {
		t.Fatal(err)
	}
	existing := candidate
	existing.ProviderCredentials = append([]vps.ProviderCredential(nil), candidate.ProviderCredentials...)
	existing.Status = vps.VPSStatusStale
	existing.ProviderCredentials[0].Status = vps.CredentialStale
	existing.ProviderCredentials[0].InvalidationReason = "plugin installation fingerprint changed"
	existing.ProviderCredentials[0].UpdatedAt = now.Add(time.Minute)
	recovered, recoveryCredentialID, err := proQ3PromotionFalseStaleRecoveryDocument(existing, candidate, credentialID, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != vps.VPSStatusVerified || recoveryCredentialID == credentialID || len(recovered.ProviderCredentials) != 2 {
		t.Fatalf("unexpected recovered document: credential=%q document=%#v", recoveryCredentialID, recovered)
	}
	if recovered.ProviderCredentials[0].Status != vps.CredentialStale || recovered.ProviderCredentials[1].ID != recoveryCredentialID || !recovered.ProviderCredentials[1].Dispatchable() {
		t.Fatalf("credential history was not preserved/reissued: %#v", recovered.ProviderCredentials)
	}
	if err := recovered.Validate(); err != nil {
		t.Fatalf("recovered document must validate: %v", err)
	}
}
