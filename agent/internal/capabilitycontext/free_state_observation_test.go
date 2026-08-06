package capabilitycontext

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
)

func TestFreeStateObservationCatalogIsSemanticAndBounded(t *testing.T) {
	catalog := FreeStateObservationCatalogFor(mixboard.TargetRef{Kind: "track", ID: "1007"})
	if catalog.SchemaVersion != FreeStateObservationCatalogSchema || catalog.Boundary != "semantic_views_only" {
		t.Fatalf("catalog header = %+v", catalog)
	}
	if len(catalog.Views) != 12 {
		t.Fatalf("catalog view count = %d", len(catalog.Views))
	}
	for _, view := range catalog.Views {
		if view.ViewID == "" || len(view.Questions) == 0 || view.CostLatencyClass == "" || view.QualityCeiling == "" {
			t.Fatalf("incomplete semantic view: %+v", view)
		}
		if strings.Contains(view.ViewID, ".raw.") {
			t.Fatalf("raw key leaked as semantic view: %+v", view)
		}
	}
}

func TestAssembleFreeStateObservationBindsRevisionAndBlocksRawPayloads(t *testing.T) {
	read := testFreeStateReadResult()
	req := FreeStateObservationRequest{
		RequestID: "req-1", ViewIDs: []string{"project.structure", "track.time_dynamics"},
		MaxDisclosureBytes: 16 * 1024, MaxItems: 4,
	}
	bundle := AssembleFreeStateObservation(req, read)
	if !bundle.ReadOnly || bundle.MutationAuthority || bundle.ProjectBinding["project_revision"] != "42" || bundle.TargetRef["id"] != "1007" {
		t.Fatalf("bundle binding/authority = %+v", bundle)
	}
	if len(bundle.Views) != 2 || bundle.DisclosureBytes <= 0 || bundle.DisclosureBytes > bundle.MaxDisclosureBytes {
		t.Fatalf("bundle disclosure = %+v", bundle)
	}
	data, _ := json.Marshal(bundle.Views)
	for _, forbidden := range []string{"raw_pcm", "waveform_array", "time_segments", "C:/secret.wav"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("forbidden payload %q leaked: %s", forbidden, data)
		}
	}
}

func TestAssembleFreeStateObservationReportsBudgetDeferredAndForbidden(t *testing.T) {
	read := testFreeStateReadResult()
	req := FreeStateObservationRequest{
		ViewIDs:            []string{"project.structure", "mix.masking_relationship", "track.raw.waveform"},
		MaxDisclosureBytes: 256,
	}
	bundle := AssembleFreeStateObservation(req, read)
	if bundle.Omissions["project.structure"] != orchestration.OmissionBudget {
		t.Fatalf("budget omission = %+v", bundle.Omissions)
	}
	if bundle.Omissions["mix.masking_relationship"] != orchestration.OmissionUnavailable {
		t.Fatalf("deferred omission = %+v", bundle.Omissions)
	}
	if bundle.Omissions["track.raw.waveform"] != orchestration.OmissionForbidden {
		t.Fatalf("forbidden omission = %+v", bundle.Omissions)
	}
	if bundle.DisclosureBytes > bundle.MaxDisclosureBytes {
		t.Fatalf("disclosure exceeded budget: %+v", bundle)
	}
}

func TestFreeStateObservationSupportsSequentialViewRequests(t *testing.T) {
	read := testFreeStateReadResult()
	first := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"track.basic_energy"}}, read)
	second := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"track.stereo_space"}}, read)
	if first.ObservationID != second.ObservationID || first.BundleID != second.BundleID {
		t.Fatalf("sequential requests lost observation binding: first=%+v second=%+v", first, second)
	}
	if first.Views["track.basic_energy"] == nil || second.Views["track.stereo_space"] == nil {
		t.Fatalf("sequential views missing: first=%+v second=%+v", first.Views, second.Views)
	}
}

func TestTrackTimeDynamicsSeparatesFamilySelectionFromProcessorPlanning(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["observation.com_projection"] = map[string]any{
		"schema_version": "com.projection.v1", "com_version": "v1", "mode": "source_only", "status": "partial",
		"source_dynamics": map[string]any{
			"status": "partial", "declared_scale": "macro_program",
			"activity": map[string]any{"valid_segment_count": 4, "coverage_ratio": 1.0},
			"time_scale_coverage": []any{
				map[string]any{"scale": "macro_program", "status": "ready", "segment_count": 4},
				map[string]any{"scale": "micro_transient", "status": "missing"},
				map[string]any{"scale": "short_gain_motion", "status": "missing", "reason": "paired_fine_envelope_required"},
			},
		},
		"trust_quality": map[string]any{
			"can_support_source_description":     true,
			"can_support_semantic_planning":      false,
			"can_support_post_action_evaluation": false,
		},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"track.time_dynamics"}}, read)
	data, _ := json.Marshal(bundle.Views)
	raw := string(data)
	if !strings.Contains(raw, `"current_decision":"processor_family_selection"`) ||
		!strings.Contains(raw, `"status":"supported_with_limitations"`) {
		t.Fatalf("selection support was not disclosed: %s", raw)
	}
	for _, forbidden := range []string{"can_support_semantic_planning", "paired_fine_envelope_required", "short_gain_motion"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("downstream planning boundary %q leaked into family selection: %s", forbidden, raw)
		}
	}
}

func TestMultitrackRelationshipCompactsMOMProjectionWithinDefaultBudget(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["project.relationship_inputs"] = map[string]any{"status": "ready", "track_count": 12, "tracks_with_level_db": 12}
	items["project.rankings.level"] = map[string]any{"status": "ready", "rows": []any{map[string]any{"track_id": "vocal", "value": -18.0}}}
	items["project.rankings.peak"] = map[string]any{"status": "ready", "rows": []any{map[string]any{"track_id": "vocal", "value": -3.0}}}
	items["project.risks.headroom"] = map[string]any{"status": "ready", "rows": []any{map[string]any{"track_id": "vocal", "headroom_db": 3.0}}}
	items["project.attention.first"] = map[string]any{"status": "ready", "track_id": "vocal", "reason": "lowest_headroom"}
	largeRows := make([]any, 24)
	for index := range largeRows {
		largeRows[index] = map[string]any{
			"track_id":        fmt.Sprintf("track-%02d", index),
			"decision_tracks": strings.Repeat("repeated_relation_payload", 120),
		}
	}
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1.4", "intent": "project_multitrack_relation_observation",
		"multitrack_relation": map[string]any{
			"status": "ready", "freshness": "fresh", "track_count": 24,
			"compared_tracks": largeRows, "band_occupancy": largeRows,
			"band_conflict_candidates": largeRows, "phase_risk_tracks": largeRows,
			"level_distribution": map[string]any{"status": "ready", "track_count": 24, "spread_db": 16.5,
				"highest": map[string]any{"track_id": "vocal", "value": -18.0},
				"lowest":  map[string]any{"track_id": "room", "value": -34.5}},
			"limitations": []any{"local_projection_only"}, "evidence_refs": []any{"observation:obs-1"},
		},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{
		ViewIDs: []string{"mix.multitrack_relationship"}, MaxDisclosureBytes: 16 * 1024, MaxItems: 12,
	}, read)
	if bundle.Omissions != nil || bundle.Status != "ready" || bundle.Views["mix.multitrack_relationship"] == nil {
		t.Fatalf("multitrack view was not disclosed: %+v", bundle)
	}
	if bundle.DisclosureBytes >= bundle.MaxDisclosureBytes {
		t.Fatalf("compact relation used %d/%d bytes", bundle.DisclosureBytes, bundle.MaxDisclosureBytes)
	}
	data, _ := json.Marshal(bundle.Views)
	if strings.Contains(string(data), "repeated_relation_payload") || strings.Contains(string(data), "decision_tracks") {
		t.Fatalf("expanded MOM relation leaked through compact view: %s", data)
	}
	if !strings.Contains(string(data), `"band_conflict_count":24`) || !strings.Contains(string(data), `"compared_track_count":24`) {
		t.Fatalf("compact relation counts missing: %s", data)
	}
}

func testFreeStateReadResult() map[string]any {
	return map[string]any{
		"observation_id": "obs-1",
		"mix_session_id": "mix-1",
		"items": map[string]any{
			"observation.binding": map[string]any{
				"observation_id": "obs-1", "mix_session_id": "mix-1", "status": "ready", "created_at": "2026-08-05T00:00:00Z",
				"target_ref":      map[string]any{"kind": "track", "id": "1007", "label": "Vocal"},
				"project_binding": map[string]any{"project_uuid": "project-1", "project_epoch": "epoch-1", "project_revision": "42", "project_state_hash": "hash-42"},
				"evidence_refs":   []any{"evidence://one"},
			},
			"project.static.summary":              map[string]any{"status": "ready", "track_count": 2},
			"project.tracks.summary":              map[string]any{"status": "ready", "tracks": []any{map[string]any{"track_id": "1007", "name": "Vocal"}}, "raw_pcm": []any{1, 2, 3}},
			"observation.tim_projection":          map[string]any{"status": "ready", "technical_summary": map[string]any{"track_count": 2}, "file_path": "C:/secret.wav"},
			"track.1007.static.identity":          map[string]any{"track_identity": map[string]any{"track_id": "1007", "name": "Vocal"}},
			"track.1007.fast.levels":              map[string]any{"status": "ready", "rms_dbfs": -18.0, "peak_dbfs": -3.0, "waveform_array": []any{0.1, 0.2}},
			"track.1007.slow.time_energy.summary": map[string]any{"status": "ready", "rows": []any{map[string]any{"start_seconds": 0.0, "rms_dbfs": -18.0}}},
			"observation.com_projection":          map[string]any{"status": "ready", "mode": "source_only", "source_dynamics": map[string]any{"status": "ready", "time_segments": []any{map[string]any{"rms_dbfs": -18.0}}}, "llm_context": map[string]any{"summary": "bounded"}},
			"track.1007.slow.stereo.summary":      map[string]any{"status": "ready", "correlation_estimate": 0.8},
		},
	}
}
