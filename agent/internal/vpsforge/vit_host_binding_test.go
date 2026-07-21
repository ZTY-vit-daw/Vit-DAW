package vpsforge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/vps"
)

func TestRecordVitHostBindingCapturesCompatibleProjectionWithoutCredential(t *testing.T) {
	root := filepath.Join(t.TempDir(), "staging", "preflight", "binding")
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	minimum, maximum, current := 0.0, 1.0, 0.0
	fingerprint, err := vps.BuildPluginFingerprint("sha256:adapter-install", []vps.ParameterSurfaceDescriptor{{ID: "0", Type: "enum", Min: &minimum, Max: &maximum, EnumValues: []string{"Off", "On"}, DisplayDomain: "mode", Scale: "enum"}})
	if err != nil {
		t.Fatal(err)
	}
	identity := vps.PluginIdentity{Manufacturer: "Vit Test", Name: "Observed VST3", Format: "VST3", Version: "1.0", InstallPath: `C:\\plugins\\observed.vst3`, Fingerprint: fingerprint}
	now := time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC)
	if _, err := Init(InitRequest{Root: root, Identity: identity, Capabilities: []string{"equalizer.v2"}, Now: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := IngestSurface(root, SurfaceSnapshot{
		SchemaVersion:  SurfaceSchema,
		PluginIdentity: identity,
		CapturedAt:     now,
		Parameters: []SurfaceParameter{{
			ID: "0", Name: "Mode", NormalizedValue: &current, DefaultNormalizedValue: &current,
			Automation: "automatable", IDProvenance: "vst3_hosted_parameter_id", StableID: true, HostControllable: true,
			Observed: map[string]any{"is_discrete": true, "is_boolean": false},
		}},
	}, "test_adapter", now); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/invoke" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request["tool"] != "plugin.get_parameters" {
			t.Fatalf("tool = %#v", request["tool"])
		}
		response := map[string]any{
			"status": "ok",
			"result": map[string]any{
				"track_id": "track_1", "plugin_id": "plugin_1",
				"plugin_identity": map[string]any{
					"manufacturer": "Vit Test", "plugin_name": "Observed VST3", "plugin_format": "VST3", "version": "1.0", "plugin_path": identity.InstallPath,
				},
				"parameters": []map[string]any{{
					"id": "0", "name": "Mode", "host_controllable": true, "is_discrete": true, "is_boolean": false, "min": 0.0, "max": 1.0,
					"display_probe": map[string]any{"label": "Mode", "discrete_labels": []map[string]any{{"index": 0, "label": "Off"}, {"index": 1, "label": "On"}}},
				}},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	result, err := RecordVitHostBinding(context.Background(), VitHostBindingRequest{Root: root, AgentURL: server.URL, TrackID: "track_1", PluginID: "plugin_1", RequiredParameterIDs: []string{"0"}, Now: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Compatible || result.Status != "observed_host_projection_compatible" || result.HostFingerprint.ParameterSurface == "" {
		t.Fatalf("binding result = %#v", result)
	}
	status, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Draft.ProviderCredentials) != 0 {
		t.Fatalf("host binding issued a credential: %#v", status.Draft.ProviderCredentials)
	}
	var artifact vitHostBindingArtifact
	if err := readJSON(filepath.Join(root, vitHostBindingFile), &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Status != result.Status || len(artifact.RequiredParameters) != 1 || artifact.RequiredParameters[0].Status != "matched_observed" {
		t.Fatalf("binding artifact = %#v", artifact)
	}
}
