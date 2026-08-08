package processorauthority

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

func TestCertifyProcessorProducesStrongPCA2ReceiptForEveryV2Family(t *testing.T) {
	cases := []struct {
		family, role, axis, inspect, apply string
	}{
		{processorattestation.FamilyLimiter, "threshold", "protection_intensity", "plugin_grabber.inspect_limiter", "plugin_grabber.apply_limiter_controls"},
		{processorattestation.FamilyGateExpander, "threshold", "activation_threshold", "plugin_grabber.inspect_gate_expander", "plugin_grabber.apply_gate_expander_controls"},
		{processorattestation.FamilyDeEsser, "reduction_range", "sibilance_reduction", "plugin_grabber.inspect_de_esser", "plugin_grabber.apply_de_esser_controls"},
		{processorattestation.FamilyTransient, "attack_amount", "envelope_emphasis", "plugin_grabber.inspect_transient_shaper", "plugin_grabber.apply_transient_shaper_controls"},
		{processorattestation.FamilyMultiband, "threshold", "band_dynamics", "plugin_grabber.inspect_multiband", "plugin_grabber.apply_multiband_controls"},
	}
	for _, tc := range cases {
		t.Run(tc.family, func(t *testing.T) {
			root := t.TempDir()
			pluginPath := filepath.Join(root, "Processor.vst3")
			writeFixture(t, pluginPath, []byte(tc.family))
			entry := fixtureEntry("Processor", "Vendor", tc.family+"-id", pluginPath)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/health" {
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
					return
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if firstTextMap(payload, "source") != "pcactl.certify_processor" {
					t.Fatalf("source=%+v", payload)
				}
				tool := firstTextMap(payload, "tool")
				result := map[string]any{"status": "exact"}
				switch tool {
				case "track.add_audio":
					result["track_id"] = "track-1"
				case "plugin.load_to_rack":
					result["plugin_id"] = "plugin-1"
				case "plugin.get_parameters":
					result["parameters"] = []any{map[string]any{"param_id": "control", "normalized_value": .5}}
				case tc.inspect:
					result["control_topology"] = map[string]any{"generation": "gen-v2"}
					result["stage"] = map[string]any{"controls": []any{
						map[string]any{"role": tc.role, "param_id": "control", "control_ref": "ref-1", "current_physical": -10.0,
							"domain": map[string]any{"unit": "db", "confidence": .95, "min": -60.0, "max": 0.0}},
					}}
				case tc.apply:
					args, _ := payload["args"].(map[string]any)
					if _, restoring := args["restore_ref"]; restoring {
						result["status"] = "exact"
					} else {
						result["controls"] = []any{map[string]any{"actual_readback": []any{map[string]any{"param_id": "control", "normalized_value": 0.0}}}}
						result["writes"] = []any{map[string]any{"status": "exact"}}
						result["restore_ref"] = "restore-v2"
					}
				case "track.delete":
					result["deleted"] = true
				default:
					t.Fatalf("unexpected tool %q", tool)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
			}))
			defer server.Close()

			report, err := CertifyProcessor(LocalProcessorCertificationOptions{AgentHTTP: server.URL, Entry: entry,
				OutputDir: filepath.Join(root, "receipt"), ProcessorFamily: tc.family, InspectTool: tc.inspect,
				ApplyTool: tc.apply, Timeout: 2 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			if report.Verdict != "passed" || report.Audit["llm_call_count"] != 0 {
				t.Fatalf("report=%+v", report)
			}
			store, _ := processorattestation.NewStoreV2(filepath.Join(root, "attestations.v2.json"))
			imported, err := ImportReceiptsV2(store, pluginsemantics.Index{Entries: []pluginsemantics.Entry{entry}}, []string{report.SummaryPath})
			if err != nil {
				t.Fatal(err)
			}
			if len(imported.Promoted) != 1 || imported.Promoted[0].ProcessorFamily != tc.family || len(imported.Promoted[0].Coverage) != 1 || imported.Promoted[0].Coverage[0].Axis != tc.axis {
				t.Fatalf("import=%+v", imported)
			}
			encoded, _ := json.Marshal(imported.Promoted[0])
			if containsForbiddenBadgeField(encoded) {
				t.Fatalf("v2 badge leaked topology: %s", encoded)
			}
		})
	}
}

func TestV2CertificationRejectsEmptyReadbackAndUnknownToolPair(t *testing.T) {
	if allReadback([]map[string]any{{"actual_readback": []any{}}}) {
		t.Fatal("empty actual_readback was accepted")
	}
	if allReadback([]map[string]any{{"actual_readback": []map[string]any{}}}) {
		t.Fatal("empty typed actual_readback was accepted")
	}
	if err := validateV2CertificationTools("unknown_family", "inspect", "apply"); err == nil {
		t.Fatal("unknown family was accepted by certification tool pairing")
	}
	if err := validateV2CertificationTools(processorattestation.FamilyDeEsser, "plugin_grabber.inspect_limiter", "plugin_grabber.apply_limiter_controls"); err == nil {
		t.Fatal("cross-family certification tools were accepted")
	}
}

func TestCertifyProcessorRejectsUnsuccessfulTypedApplyStatus(t *testing.T) {
	root := t.TempDir()
	pluginPath := filepath.Join(root, "Processor.vst3")
	writeFixture(t, pluginPath, []byte("limiter"))
	entry := fixtureEntry("Processor", "Vendor", "limiter-id", pluginPath)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		result := map[string]any{"status": "exact"}
		switch firstTextMap(payload, "tool") {
		case "track.add_audio":
			result["track_id"] = "track-1"
		case "plugin.load_to_rack":
			result["plugin_id"] = "plugin-1"
		case "plugin.get_parameters":
			result["parameters"] = []any{map[string]any{"param_id": "control", "normalized_value": .5}}
		case "plugin_grabber.inspect_limiter":
			result["control_topology"] = map[string]any{"generation": "gen-v2"}
			result["limiter_stages"] = []any{map[string]any{"operating_point": []any{map[string]any{
				"role": "threshold", "param_id": "control", "control_ref": "ref-1", "current_physical": -10.0,
				"domain": map[string]any{"unit": "db", "confidence": .95, "min": -60.0, "max": 0.0},
			}}}}
		case "plugin_grabber.apply_limiter_controls":
			result["status"] = "rejected"
			result["controls"] = []any{map[string]any{"actual_readback": []any{map[string]any{"param_id": "control", "normalized_value": .5}}}}
			result["writes"] = []any{map[string]any{"status": "rejected"}}
		case "track.delete":
			result["deleted"] = true
		default:
			t.Fatalf("unexpected tool %q", firstTextMap(payload, "tool"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "result": result})
	}))
	defer server.Close()

	report, err := CertifyProcessor(LocalProcessorCertificationOptions{
		AgentHTTP: server.URL, Entry: entry, OutputDir: filepath.Join(root, "receipt"), ProcessorFamily: processorattestation.FamilyLimiter,
		InspectTool: "plugin_grabber.inspect_limiter", ApplyTool: "plugin_grabber.apply_limiter_controls", Timeout: 2 * time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "unsuccessful status") {
		t.Fatalf("err=%v", err)
	}
	if report.Verdict != "failed" {
		t.Fatalf("report=%+v", report)
	}
}
