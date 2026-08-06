package processorauthority

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

func TestCertifyCompressorProducesStrongImportableReceiptWithoutLLM(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Example Compressor.vst3")
	writeFixture(t, pluginPath, []byte("compressor-binary"))
	entry := fixtureEntry("Example Compressor", "Vendor", "example-compressor", pluginPath)

	var mu sync.Mutex
	tools := []string{}
	deleteCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		tool := firstTextMap(payload, "tool")
		if firstTextMap(payload, "source") != "pcactl.certify_compressor" {
			t.Fatalf("unexpected invoke source: %+v", payload)
		}
		mu.Lock()
		tools = append(tools, tool)
		mu.Unlock()
		result := map[string]any{"status": "exact"}
		switch tool {
		case "track.add_audio":
			result["track_id"] = "track-1"
		case "plugin.load_to_rack":
			result["plugin_id"] = "plugin-1"
		case "plugin.get_parameters":
			result["parameters"] = []any{map[string]any{"param_id": "threshold", "normalized_value": 0.5}}
		case "plugin_grabber.inspect_compressor":
			result["control_topology"] = map[string]any{"generation": "gen-1"}
			result["compressor_stage"] = map[string]any{"control_paths": []any{map[string]any{
				"operating_point": []any{map[string]any{
					"role": "threshold", "param_id": "threshold", "control_ref": "ccr-threshold", "current_physical": -10.0,
					"domain": map[string]any{"unit": "db", "confidence": 0.95, "min": -60.0, "max": 0.0},
				}},
			}}}
		case "plugin_grabber.apply_compressor_controls":
			args, _ := payload["args"].(map[string]any)
			if _, restoring := args["restore_ref"]; restoring {
				result["status"] = "exact"
			} else {
				result["controls"] = []any{map[string]any{"actual_readback": map[string]any{"display_value": -60.0}}}
				result["writes"] = []any{map[string]any{"status": "exact"}}
				result["restore_ref"] = "restore-1"
			}
		case "track.delete":
			deleteCalls++
			if deleteCalls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "error", "error": "Cannot delete the last audio track"})
				return
			}
			result["deleted"] = true
		default:
			t.Fatalf("unexpected tool %q", tool)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
	}))
	defer server.Close()

	report, err := CertifyCompressor(LocalCertificationOptions{
		AgentHTTP: server.URL, Entry: entry, OutputDir: filepath.Join(root, "receipt"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Verdict != "passed" || report.Audit["llm_call_count"] != 0 {
		t.Fatalf("report=%+v", report)
	}
	wantTools := []string{"track.add_audio", "plugin.load_to_rack", "plugin.get_parameters", "plugin_grabber.inspect_compressor", "plugin_grabber.inspect_compressor", "plugin_grabber.apply_compressor_controls", "plugin_grabber.apply_compressor_controls", "plugin.get_parameters", "track.delete", "track.add_audio", "track.delete"}
	if !reflect.DeepEqual(tools, wantTools) {
		t.Fatalf("tools=%v want=%v", tools, wantTools)
	}

	store, _ := processorattestation.NewStore(filepath.Join(root, "attestations.json"))
	imported, err := ImportReceipts(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{report.SummaryPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Promoted) != 1 || len(imported.Promoted[0].Coverage) != 1 || imported.Promoted[0].Coverage[0].Axis != "activation_intensity" {
		t.Fatalf("import=%+v", imported)
	}
	encoded, _ := json.Marshal(imported.Promoted[0])
	if string(encoded) == "" || containsForbiddenBadgeField(encoded) {
		t.Fatalf("badge leaked certification topology: %s", encoded)
	}
}

func containsForbiddenBadgeField(encoded []byte) bool {
	var value map[string]any
	if json.Unmarshal(encoded, &value) != nil {
		return true
	}
	for _, forbidden := range []string{"param_id", "topology_generation", "profile", "vps", "spal"} {
		if jsonContainsKey(value, forbidden) {
			return true
		}
	}
	return false
}

func jsonContainsKey(value any, forbidden string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == forbidden || jsonContainsKey(child, forbidden) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonContainsKey(child, forbidden) {
				return true
			}
		}
	}
	return false
}
