package vpsforge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestPreflightWritesStagingOnlyObservedArtifacts(t *testing.T) {
	current := 0.5
	defaultValue := 0.5
	snapshot := HostSnapshot{
		Identity: vps.PluginIdentity{Manufacturer: "Vit Test", Name: "Observed Plugin", Format: "VST3", Version: "1.0", InstallPath: `C:\plugins\observed.vst3`, Fingerprint: vps.PluginFingerprint{Installation: "sha256:install", ParameterSurface: "sha256:parameters", DisplaySurface: "sha256:display"}},
		Surface: SurfaceSnapshot{
			SchemaVersion:  SurfaceSchema,
			PluginIdentity: vps.PluginIdentity{Manufacturer: "Vit Test", Name: "Observed Plugin", Format: "VST3", Version: "1.0", InstallPath: `C:\plugins\observed.vst3`, Fingerprint: vps.PluginFingerprint{Installation: "sha256:install", ParameterSurface: "sha256:parameters", DisplaySurface: "sha256:display"}},
			Parameters:     []SurfaceParameter{{ID: "100", Name: "Observed control", NormalizedValue: &current, DefaultNormalizedValue: &defaultValue, HostControllable: true, Automation: "automatable", StableID: true, IDProvenance: "vst3_hosted_parameter_id", DisplayDomain: vps.DisplayDomain{Text: "0.0", Scale: "normalized"}}},
			CapturedAt:     time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		},
		AdapterSnapshot: json.RawMessage(`{"identity":{"name":"Observed Plugin"},"parameters":[{"id":"100"}]}`),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/plugin/snapshot":
			writeHTTP(w, http.StatusOK, snapshot)
		case "/v1/plugin/parameters/write":
			writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "result": map[string]any{
				"transaction_id": "tx-1",
				"fresh_readback": map[string]any{"parameters": []map[string]any{{"id": "100", "normalized_value": 0.51}}},
			}})
		case "/v1/plugin/rollback":
			writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "result": map[string]any{"rollback_verified": true}})
		case "/v1/plugin/state/save":
			writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "result": map[string]any{"state_id": "state-1"}})
		case "/v1/plugin/state/roundtrip":
			writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "result": map[string]any{"parameter_readback_matches_preimage": true}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	root := filepath.Join(t.TempDir(), "staging")
	result, err := Preflight(context.Background(), PreflightRequest{
		Root: root, HostURL: server.URL, CandidateBadge: "equalizer.v2", Category: "spectral_processing", Task: "equalizer.correct_tone",
		RunParameterProbe: true, RunFXMBaseline: false, Now: time.Date(2026, 7, 17, 0, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status.Validation.InstallReady || len(result.Status.Draft.ProviderCredentials) != 0 {
		t.Fatalf("preflight must not create installation authority: %+v", result.Status)
	}
	for _, name := range []string{pluginIdentityFile, surfaceFile, parameterProbeFile, stateRoundtripFile, fxmBaselineFile, ledgerFile, groupsFile, humanGapsFile, witnessRoundsFile, conformanceFile, draftFile, candidateBadgesFile, badgeActionContractFile, featureGapsFile, badgeSkeletonFile} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("missing preflight artifact %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, candidateBadgesFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"agent-inferred"`) || !strings.Contains(string(data), `"routing_eligible": false`) {
		t.Fatalf("candidate badge promoted or missing provenance: %s", data)
	}
	contract, err := os.ReadFile(filepath.Join(root, badgeActionContractFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contract), `"authoring_contract_available_not_conformed"`) || !strings.Contains(string(contract), `"eq.pass_filter.patch.v2"`) {
		t.Fatalf("badge action contract missing equalizer action grammar: %s", contract)
	}
	probe, err := os.ReadFile(filepath.Join(root, parameterProbeFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(probe), `"status": "observed_passed"`) || !strings.Contains(string(probe), `"state_roundtrip_after_write"`) {
		t.Fatalf("parameter probe did not preserve the controlled write/state/rollback evidence: %s", probe)
	}
}
