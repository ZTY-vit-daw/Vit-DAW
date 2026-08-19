package mom

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectMaskingRelationshipProjectionIsDirectionalCandidateOnly(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectMaskingObservation})
	input.ProjectPackage["masking_relationship_inputs"] = map[string]any{
		"schema_version": "dad.masking_measurement.v1",
		"measurement_id": "mask-1",
		"status":         "ready",
		"freshness":      "fresh",
		"model": map[string]any{
			"model_version":  "vit_relative_energetic_masking_risk.v1",
			"candidate_only": true,
		},
		"project_binding": map[string]any{"project_uuid": "p01", "project_revision": "17", "project_state_hash": "hash-17"},
		"conditions": map[string]any{
			"tap_point": "track_post_fader", "range_start_seconds": 0.0, "range_end_seconds": 12.0,
			"tail_seconds": 0.0, "sample_rate": 48000.0, "analyzer_revision": "masking-frames-v1", "synchronized": true, "frame_count_per_track": 128,
		},
		"coverage": map[string]any{"track_count": 2, "eligible_track_count": 2, "candidate_count": 1},
		"pair_bands": []any{
			map[string]any{
				"masker_track_id": "track_2", "masker_track_name": "Bass", "target_track_id": "track_1", "target_track_name": "Lead Vocal",
				"band_id": "low_mid", "min_hz": 250.0, "max_hz": 500.0, "active_frame_count": 80, "risk_frame_count": 48,
				"risk_coverage_ratio": 0.6, "median_margin_db": 1.2, "p90_margin_db": 4.8, "max_margin_db": 7.1, "status": "candidate",
			},
			map[string]any{"masker_track_id": "track_1", "target_track_id": "track_2", "band_id": "air", "status": "clear"},
		},
		"evidence_refs": []any{"evidence://probe-track-1", "evidence://probe-track-2", "dad.masking_measurement:mask-1"},
		"limitations":   []any{"relative_dbfs_model_is_not_calibrated_spl"},
		"frames":        []any{map[string]any{"levels_dbfs": map[string]any{"low_mid": -12.0}}},
	}

	proj := Build(input)
	if proj.Intent != IntentProjectMaskingObservation || proj.MaskingRelationship == nil {
		t.Fatalf("masking projection identity = intent=%q projection=%#v", proj.Intent, proj.MaskingRelationship)
	}
	relation := *proj.MaskingRelationship
	if relation.SchemaVersion != MaskingRelationshipSchema || relation.Status != StatusReady || !relation.CandidateOnly {
		t.Fatalf("masking envelope = %#v", relation)
	}
	if len(relation.Candidates) != 1 || relation.Candidates[0].MaskerTrackID != "track_2" || relation.Candidates[0].TargetTrackID != "track_1" {
		t.Fatalf("directional candidates = %#v", relation.Candidates)
	}
	if tail, ok := relation.Conditions["tail_seconds"]; !ok || number(tail) != 0 {
		t.Fatalf("masking tail condition missing: %#v", relation.Conditions)
	}
	if proj.TrustQuality.CanSupportActionPreflight || !proj.TrustQuality.CanSupportSuggestion {
		t.Fatalf("masking trust authority = %#v", proj.TrustQuality)
	}
	data, err := json.Marshal(ContextProjection(proj))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{`"frames"`, `"levels_dbfs"`, `"plugin_choice"`, `"parameter"`, `"mutation_authority"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("masking context leaked %s: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "directional_improvement_risk_candidates_only") || !strings.Contains(text, "not_deterministic_mix_defects") {
		t.Fatalf("candidate-only contract missing: %s", text)
	}
}

func TestProjectMaskingRelationshipFailsClosedWithoutSynchronizedConditions(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectMaskingObservation})
	input.ProjectPackage["masking_relationship_inputs"] = map[string]any{
		"status": "ready", "model": map[string]any{"model_version": "v1", "candidate_only": true},
		"conditions": map[string]any{"tap_point": "track_post_fader", "synchronized": false},
		"coverage":   map[string]any{"track_count": 2},
	}
	relation := Build(input).MaskingRelationship
	if relation == nil || relation.Status != StatusSuspect {
		t.Fatalf("unsynchronized measurement did not fail closed: %#v", relation)
	}
}
