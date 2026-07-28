package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/eqcontrolgraph"
)

func TestParseDisplayNumberUsesFrequencyUnitOnly(t *testing.T) {
	if value, ok := parseDisplayNumber("3.82k", "Hz"); !ok || value != 3820 {
		t.Fatalf("frequency=%g ok=%v", value, ok)
	}
	if value, ok := parseDisplayNumber("3.82k", "dB"); !ok || value != 3.82 {
		t.Fatalf("gain=%g ok=%v", value, ok)
	}
}

func TestValidateSmokeEvidenceRequiresAllSafetyProofs(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "summary.json")
	summary := map[string]interface{}{
		"schema_version": "vit.eq_vps.control_graph.smoke_result.v1", "status": "passed", "plugin": "Fixture EQ",
		"A_without_vps": map[string]interface{}{"topology": nil},
		"B_with_vps": map[string]interface{}{
			"topology": map[string]interface{}{"mapping_source": "vps_control_graph"},
			"tests": []interface{}{map[string]interface{}{
				"status": "exact", "actual_readback": []interface{}{map[string]interface{}{"param_id": "1"}},
				"zero_drift_after_undo": true,
				"formal_undo":           map[string]interface{}{"status": "exact", "rollback": map[string]interface{}{"verified": true}},
			}},
		},
		"C_after_unload": map[string]interface{}{"topology": nil}, "final_zero_drift": true,
		"audit": map[string]interface{}{"audio_probe_count": 0.0, "learning_call_count": 0.0, "profile_call_count": 0.0, "spal_call_count": 0.0, "b4_call_count": 0.0},
	}
	writeTestJSON(t, path, summary)
	document := eqcontrolgraph.Document{Plugin: eqcontrolgraph.PluginIdentity{Name: "Fixture EQ"}}
	if evidence, err := validateSmokeEvidence(path, document); err != nil || len(evidence) != 2 {
		t.Fatalf("evidence=%v err=%v", evidence, err)
	}
	summary["final_zero_drift"] = false
	writeTestJSON(t, path, summary)
	if _, err := validateSmokeEvidence(path, document); err == nil {
		t.Fatal("missing final zero drift was accepted")
	}
}

func writeTestJSON(t *testing.T, path string, value interface{}) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
