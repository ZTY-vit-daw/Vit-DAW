package mom

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectFrequencyRelationshipProjectionContract(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	input.ProjectPackage["project_uuid"] = "project-frequency"
	input.ProjectPackage["project_epoch"] = "epoch-1"
	input.ProjectPackage["project_revision"] = "17"
	input.ProjectPackage["project_state_hash"] = "hash-17"
	for _, track := range rowsFromAny(input.ProjectPackage["tracks"]) {
		band := mapValue(track["band_energy"])
		band["source"] = "kernel_l3_offline_analyzer"
		band["tap_point"] = "source_file_pre_fx"
		band["source_revision"] = "source-rev"
		band["clip_revision"] = "clip-rev"
		band["sample_rate"] = 44100.0
		band["channel_count"] = 2
		band["coverage_seconds"] = 12.0
		band["analyzer_version"] = "fixture-v1"
	}

	proj := Build(input)
	if proj.MOMVersion != "v1.5" || proj.Intent != IntentProjectFrequencyObservation {
		t.Fatalf("projection identity = version=%q intent=%q", proj.MOMVersion, proj.Intent)
	}
	if proj.FrequencyRelationship == nil {
		t.Fatal("frequency relationship projection missing")
	}
	relation := *proj.FrequencyRelationship
	if relation.SchemaVersion != FrequencyRelationshipSchema || relation.Status != StatusReady || relation.TapPoint != "source_file_pre_fx" {
		t.Fatalf("frequency relationship envelope = %#v", relation)
	}
	if relation.ProjectCutRef != "project-state:project-frequency:epoch-1:17:hash-17" {
		t.Fatalf("project cut ref = %q", relation.ProjectCutRef)
	}
	if len(relation.TrackProfiles) != 2 || relation.Coverage["decision_tracks_truncated"] != false {
		t.Fatalf("coverage/profiles = %#v profiles=%d", relation.Coverage, len(relation.TrackProfiles))
	}
	if text(relation.PersistenceSummary["status"]) != StatusMissing || boolValue(relation.PersistenceSummary["can_distinguish_persistent_from_transient_overlap"]) {
		t.Fatalf("persistence must remain explicit unavailable without temporal evidence: %#v", relation.PersistenceSummary)
	}
	if proj.TrustQuality.CanSupportActionPreflight {
		t.Fatalf("read-only frequency projection must not grant action authority: %#v", proj.TrustQuality)
	}
	if !containsString(relation.Limitations, "energy_overlap_is_candidate_only_not_psychoacoustic_masking_fact") || !containsString(relation.Limitations, "source_file_pre_fx_does_not_represent_current_post_plugin_signal") {
		t.Fatalf("limitations = %#v", relation.Limitations)
	}
	data, err := json.Marshal(ContextProjection(proj))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbiddenKey := range []string{`"eq_parameters"`, `"plugin_choice"`, `"proposal"`, `"pending"`, `"raw_samples"`, `"spectrogram_tiles"`, `"time_segments"`} {
		if strings.Contains(string(data), forbiddenKey) {
			t.Fatalf("context leaked forbidden field %s in %s", forbiddenKey, data)
		}
	}
}

func TestFrequencyRelationshipDecisionTracksAreNeverUITopNTruncated(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	tracks := []any{}
	for i := 1; i <= 7; i++ {
		tracks = append(tracks, map[string]any{
			"track_id": "track_" + text(i), "name": "Track " + text(i), "active_state": "active",
			"band_energy": map[string]any{"status": "ready", "source": "kernel_l3_offline_analyzer", "tap_point": "source_file_pre_fx",
				"source_revision": "source-rev", "clip_revision": "clip-rev", "sample_rate": 48000.0, "channel_count": 2,
				"coverage_seconds": 12.0, "analyzer_version": "fixture-v1", "bands": map[string]any{
					"bass": map[string]any{"unit_energy": 0.40 - float64(i)*0.01, "energy_db": -8.0 - float64(i)},
				}},
		})
	}
	input.ProjectPackage["tracks"] = tracks
	relation := Build(input).FrequencyRelationship
	if relation == nil {
		t.Fatal("frequency relationship missing")
	}

	var bass map[string]any
	for _, region := range relation.FrequencyRegions {
		if text(region["region"]) == "bass" {
			bass = region
			break
		}
	}
	if got := len(rowsFromAny(bass["decision_tracks"])); got != 7 || boolValue(bass["decision_tracks_truncated"]) {
		t.Fatalf("decision tracks were UI-truncated: %#v", bass)
	}
	if len(relation.TrackProfiles) != 7 || int(number(relation.Coverage["eligible_track_count"])) != 7 {
		t.Fatalf("whole-project coverage = %#v", relation.Coverage)
	}
}

func TestFrequencyRelationshipFreshnessAndTapMismatchAreNotPromoted(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	tracks[0]["frequency_evidence"] = frequencyTestEvidence("ready", "track_post_fader", "source-rev", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .3}})
	tracks[1]["frequency_evidence"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .29}})
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	relation := Build(input).FrequencyRelationship
	if relation == nil || relation.Status != StatusSuspect || relation.TapPoint != "mixed" {
		t.Fatalf("mixed tap was promoted: %#v", relation)
	}
	if boolValue(relation.Coverage["supports_same_tap_compare"]) || int(number(relation.Coverage["conflict_candidate_count"])) != 0 || len(relation.FrequencyRegions) != 0 {
		t.Fatalf("mixed tap must not support cross-track relations: %#v", relation.Coverage)
	}

	tracks[1]["frequency_evidence"] = frequencyTestEvidence("stale", "track_post_fader", "source-rev", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .29}})
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	relation = Build(input).FrequencyRelationship
	if relation == nil || relation.Status != StatusPartial || int(number(relation.Coverage["missing_track_count"])) != 1 {
		t.Fatalf("stale coverage was promoted: %#v", relation)
	}
}

func TestFrequencyRelationshipsComparableRequiresSameProjectTapAndWholeTrackScope(t *testing.T) {
	base := FrequencyRelationship{
		SchemaVersion: FrequencyRelationshipSchema, Status: StatusReady, TapPoint: "track_post_fader",
		Scope: map[string]any{"project_id": "p1", "track_ids": []string{"t1", "t2"}},
	}
	after := base
	after.ProjectCutRef = "project-cut:after"
	if ok, reasons := FrequencyRelationshipsComparable(base, after); !ok || len(reasons) != 0 {
		t.Fatalf("same contract should compare: ok=%v reasons=%v", ok, reasons)
	}
	after.TapPoint = "source_file_pre_fx"
	if ok, reasons := FrequencyRelationshipsComparable(base, after); ok || !containsString(reasons, "tap_point_mismatch_or_unknown") {
		t.Fatalf("tap mismatch should block compare: ok=%v reasons=%v", ok, reasons)
	}
}

func TestLegacyMOMIntentDoesNotMaterializeFrequencyRelationship(t *testing.T) {
	proj := Build(testInput(map[string]any{"mom_intent": IntentProjectMultitrackObservation}))
	if proj.FrequencyRelationship != nil {
		t.Fatalf("legacy projection grew a new payload: %#v", proj.FrequencyRelationship)
	}
	data, _ := json.Marshal(ContextProjection(proj))
	if strings.Contains(string(data), `"frequency_relationship"`) {
		t.Fatalf("legacy context changed shape: %s", data)
	}
}

func frequencyTestEvidence(status, tapPoint, sourceRevision string, coverageSeconds float64, bands map[string]any) map[string]any {
	return map[string]any{
		"status": status, "tap_point": tapPoint, "source_revision": sourceRevision, "clip_revision": "clip-rev",
		"sample_rate": 44100.0, "channel_count": 2, "coverage_seconds": coverageSeconds,
		"analyzer_version": "fixture-v1", "bands": bands,
	}
}

func TestFrequencyRelationshipAllowsDifferentMaterialRevisionAcrossTracks(t *testing.T) {
	// Material identity revisions (source/clip/render) are per-track facts
	// that differ between tracks by design. Cross-track frequency comparison
	// must not require them to be uniform; only the measurement conditions
	// (tap, time window, sample format, analyzer) must match. Without this,
	// every real multi-track project would be permanently incomparable.
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	tracks[0]["band_energy"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev-1", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .3}})
	tracks[1]["band_energy"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev-2", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .29}})
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	relation := Build(input).FrequencyRelationship
	if relation == nil || relation.Status != StatusReady || len(relation.ConflictCandidates) == 0 || len(relation.FrequencyRegions) == 0 {
		t.Fatalf("different material revision blocked cross-track comparison: %#v", relation)
	}
	if containsString(relation.Limitations, "measurement_condition_mismatch_fields=source_revision") ||
		containsString(relation.Limitations, "measurement_conditions_not_comparable_cross_track_relations_withheld") {
		t.Fatalf("material revision leaked into comparability disclosure: %#v", relation.Limitations)
	}
}

func TestFrequencyRelationshipStillRequiresMaterialRevisionPresent(t *testing.T) {
	// The material revision must exist per track (evidence completeness), even
	// though it no longer has to be uniform across tracks.
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	evidence := frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .3}})
	delete(evidence, "source_revision")
	tracks[0]["band_energy"] = evidence
	tracks[1]["band_energy"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev", 12.0, map[string]any{"bass": map[string]any{"unit_energy": .29}})
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	relation := Build(input).FrequencyRelationship
	if relation == nil || relation.Status != StatusSuspect {
		t.Fatalf("missing source revision was treated as comparable: %#v", relation)
	}
	if !containsString(relation.Limitations, "measurement_condition_fields_incomplete") {
		t.Fatalf("missing revision was not disclosed as incomplete: %#v", relation.Limitations)
	}
}

func TestFrequencyRelationshipRejectsDifferentTimeRange(t *testing.T) {
	input := testInput(map[string]any{"mom_intent": IntentProjectFrequencyObservation})
	tracks := rowsFromAny(input.ProjectPackage["tracks"])
	tracks[0]["band_energy"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev", 20.0, map[string]any{"bass": map[string]any{"unit_energy": .3}})
	tracks[1]["band_energy"] = frequencyTestEvidence("ready", "source_file_pre_fx", "source-rev", 10.0, map[string]any{"bass": map[string]any{"unit_energy": .29}})
	input.ProjectPackage["tracks"] = []any{tracks[0], tracks[1]}
	relation := Build(input).FrequencyRelationship
	if relation == nil || relation.Status != StatusSuspect || len(relation.ConflictCandidates) != 0 {
		t.Fatalf("different time range was treated as comparable: %#v", relation)
	}
	if !containsString(relation.Limitations, "measurement_condition_mismatch_fields=end_seconds") {
		t.Fatalf("time range mismatch was not disclosed: %#v", relation.Limitations)
	}
}
