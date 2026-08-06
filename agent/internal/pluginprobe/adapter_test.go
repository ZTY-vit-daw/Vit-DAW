package pluginprobe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestObservationAPIHasNoParameterOrStateMutationRoutes(t *testing.T) {
	server := NewVST3HostAdapter("unused").Handler()
	for _, path := range []string{
		"/v1/plugin/parameters/write",
		"/v1/plugin/state/save",
		"/v1/plugin/state/restore",
		"/v1/plugin/state/roundtrip",
		"/v1/plugin/rollback",
	} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("mutation route %s remains reachable: status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestNormalizeVST3WorkerSnapshotUsesSharedEnumFingerprintGrammar(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"identity": map[string]any{
			"manufacturer": "Vit Test", "name": "Observed VST3", "format": "VST3", "version": "1.0",
			"install_path": `C:\\plugins\\observed.vst3`, "file_fingerprint": "sha256:test-install",
		},
		"parameters": []map[string]any{
			{
				"id": "0", "id_provenance": "vst3_hosted_parameter_id", "stable_id": true,
				"host_label": "Mode", "normalized_value": 0.0, "display_value": "Bell", "default_normalized_value": 0.0,
				"automation": "automatable", "is_discrete": true, "is_boolean": false, "is_meta": false,
				"num_steps": 3, "category": 0, "display_choices": []string{"Bell", "Low Cut", "High Cut"},
			},
			{
				"id": "1", "id_provenance": "vst3_hosted_parameter_id", "stable_id": true,
				"host_label": "Frequency", "unit": "Hz", "normalized_value": 0.5, "display_value": "1000 Hz", "default_normalized_value": 0.5,
				"automation": "automatable", "is_discrete": false, "is_boolean": false, "is_meta": false,
				"num_steps": 0, "category": 0, "display_choices": []string{},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NormalizeVST3WorkerSnapshot(raw, nil)
	if err != nil {
		t.Fatalf("NormalizeVST3WorkerSnapshot: %v", err)
	}
	minimum, maximum := 0.0, 1.0
	expected, err := buildPluginFingerprint("sha256:test-install", []parameterSurfaceDescriptor{
		{ID: "0", Type: "enum", Min: &minimum, Max: &maximum, EnumValues: []string{"Bell", "Low Cut", "High Cut"}, DisplayDomain: "normalized_to_host_display", Scale: "normalized"},
		{ID: "1", Type: "continuous", Min: &minimum, Max: &maximum, DisplayDomain: "normalized_to_host_display", Unit: "Hz", Scale: "normalized"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Identity.Fingerprint.ParameterSurface; got != expected.ParameterSurface {
		t.Fatalf("parameter surface uses a divergent VST3 discrete grammar: got %s want %s", got, expected.ParameterSurface)
	}
}
