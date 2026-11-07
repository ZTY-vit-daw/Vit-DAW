package mixboard_test

// FIXTURE-WE-HYGIENE-1 (card 294) offline read-path verification.
// Injected via `go test -overlay` ONLY — the repository source tree is not
// modified (card red line: zero source changes). Replays the 293-established
// read path (mixboard.BuildObservation over the fixture snapshot + acoustic
// bridge package + the kernel project-state capture from the 265 run) in four
// scenarios:
//
//	v1_control_orig : pristine fixture snapshot, lr=Sep-5 probe_1007
//	                 -> WE view ready _1011 via acoustic bridge (265 ground truth)
//	v2_control_flip : pristine snapshot, lr flipped to the real 266 probe_1027
//	                 -> WE view missing+mismatch on bare slot _1055 (266 ground truth)
//	v3_hyg          : hygienized snapshot (card-named removals), lr unchanged
//	                 -> WE view ready _1011 (stable path preserved)
//	v4_hyg_flip     : hygienized snapshot, same lr flip (post-hygiene worst case)
//	                 -> no _1055 bare-slot float-out may remain possible

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/mixboard"
)

const rigDir = `D:/Vit_DAW_ws_294hyg/rig`

func loadJSONMap(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rigDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return out
}

func weView(t *testing.T, variant string, acoustic map[string]any) map[string]any {
	t.Helper()
	projectState := loadJSONMap(t, "project_state_265.json")
	req := mixboard.Request{
		ProjectState: projectState,
		Args: map[string]any{
			"feature_snapshot_path":    filepath.Join(rigDir, variant),
			"acoustic_package_status":  acoustic,
			"mix_session_id":           "rig_294",
			"goal_text":                "fixture hygiene offline replay",
			"duration_seconds":         float64(20),
		},
	}
	obs := mixboard.BuildObservation(req, "2026-11-07T00:00:00Z")
	global, _ := obs.GlobalSummary["feature_snapshot"].(map[string]any)
	if global == nil {
		t.Fatalf("%s: no feature_snapshot in global summary", variant)
	}
	we, _ := global["waveform_envelope"].(map[string]any)
	if we == nil {
		t.Fatalf("%s: no waveform_envelope view", variant)
	}
	tweRows, _ := global["track_waveform_envelopes"].([]any)
	for _, raw := range tweRows {
		row, _ := raw.(map[string]any)
		if row == nil {
			continue
		}
		if str(row["clip_id"]) == "1055" || str(row["track_id"]) == "1007" && str(row["clip_id"]) == "1055" {
			t.Errorf("%s: twe view still exposes (1007,1055) row", variant)
		}
	}
	t.Logf("%s: we_status=%s we_req=%s we_track=%s we_clip=%s reason=%s twe_rows=%d obs_status=%s",
		variant, str(we["status"]), str(we["request_id"]), str(we["track_id"]), str(we["clip_id"]), str(we["reason"]), len(tweRows), obs.Status)
	return we
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func TestWEHygieneReadPathScenarios(t *testing.T) {
	acoustic := loadJSONMap(t, "acoustic_pkg_1007_1011.json")

	// v1 control: pristine fixture + Sep-5 lr -> bridge open -> _1011 ready.
	we1 := weView(t, "v1_control_orig.json", acoustic)
	if str(we1["status"]) != "ready" || str(we1["request_id"]) != "kernel_prepared_waveform_envelope_1011" {
		t.Errorf("v1 control failed: expected ready/_1011 (265 ground truth), got status=%s req=%s", str(we1["status"]), str(we1["request_id"]))
	}

	// v2 control: lr flipped to the real 266 probe identity -> bridge off ->
	// bare slot _1055 floats out as honest mismatch stub (266 ground truth).
	we2 := weView(t, "v2_control_flip.json", acoustic)
	if str(we2["status"]) != "missing" || str(we2["reason"]) != "project_material_identity_mismatch" || str(we2["request_id"]) != "kernel_prepared_waveform_envelope_1055" {
		t.Errorf("v2 control failed: expected missing+mismatch on _1055 (266 ground truth), got status=%s reason=%s req=%s", str(we2["status"]), str(we2["reason"]), str(we2["request_id"]))
	}

	// v3: hygienized fixture, lr unchanged -> stable ready path preserved.
	we3 := weView(t, "v3_postswap.json", acoustic)
	if str(we3["status"]) != "ready" || str(we3["request_id"]) != "kernel_prepared_waveform_envelope_1011" {
		t.Errorf("v3 failed: expected ready/_1011 unchanged after hygiene, got status=%s req=%s reason=%s", str(we3["status"]), str(we3["request_id"]), str(we3["reason"]))
	}

	// v4: hygienized fixture + lr flip (worst case) -> the _1055 bare-slot
	// float-out must be impossible: no 1055 identity may surface in the WE view.
	we4 := weView(t, "v4_postswap.json", acoustic)
	if str(we4["request_id"]) == "kernel_prepared_waveform_envelope_1055" || str(we4["clip_id"]) == "1055" {
		t.Errorf("v4 failed: _1055 bare slot still floats out after hygiene: status=%s reason=%s req=%s clip=%s", str(we4["status"]), str(we4["reason"]), str(we4["request_id"]), str(we4["clip_id"]))
	}
	if str(we4["status"]) == "missing" && str(we4["reason"]) == "project_material_identity_mismatch" {
		t.Errorf("v4 failed: mismatch stub still produced (on %s/%s) after hygiene", str(we4["request_id"]), str(we4["clip_id"]))
	}
}
