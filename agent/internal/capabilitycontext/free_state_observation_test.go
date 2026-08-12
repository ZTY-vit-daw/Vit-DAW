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
	if len(catalog.Views) != 17 {
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

func TestNormalizeFreeStateObservationUsesBoundedFullMultiViewDefault(t *testing.T) {
	req := NormalizeFreeStateObservationRequest(FreeStateObservationRequest{ViewIDs: []string{"project.structure", "mix.frequency_relationship"}})
	if req.MaxDisclosureBytes != 64*1024 {
		t.Fatalf("default disclosure budget = %d, want %d", req.MaxDisclosureBytes, 64*1024)
	}
	if req.MaxDisclosureBytes > 64*1024 {
		t.Fatalf("default disclosure budget exceeded bounded ceiling: %d", req.MaxDisclosureBytes)
	}
}

func TestRejectedFreeStateObservationScopesMixedViewSet(t *testing.T) {
	req := FreeStateObservationRequest{RequestID: "mixed-reject", ViewIDs: []string{
		"mix.masking_relationship", "mix.multitrack_relationship", "mix.frequency_relationship",
	}}
	bundle := RejectedFreeStateObservationScoped(req, []string{"mix.masking_relationship"}, "mix.masking_relationship: availability=deferred")
	if bundle.Status != "rejected" || bundle.RejectionScope != "exact_view_set" {
		t.Fatalf("rejection scope = %#v", bundle)
	}
	if len(bundle.BlockingViewIDs) != 1 || bundle.BlockingViewIDs[0] != "mix.masking_relationship" {
		t.Fatalf("blocking views = %#v", bundle.BlockingViewIDs)
	}
	if len(bundle.NonBlockingViewIDs) != 2 {
		t.Fatalf("non-blocking views = %#v", bundle.NonBlockingViewIDs)
	}
	if bundle.AuditReceipt.RejectionScope != "exact_view_set" || len(bundle.AuditReceipt.NonBlockingViewIDs) != 2 {
		t.Fatalf("audit rejection metadata = %#v", bundle.AuditReceipt)
	}
}

func TestProjectStructureViewCompactsExpandedMOMProjection(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1", "intent": "project_structure_observation",
		"project_structure": map[string]any{
			"status": "ready", "project_id": "project-1", "track_id": "1007", "duration_seconds": 20.0,
			"expanded_internal_payload": strings.Repeat("not-for-llm", 100000),
		},
		"trust_quality": map[string]any{"overall_status": "ready", "expanded_internal_payload": strings.Repeat("not-for-llm", 100000)},
	}
	items["project.tracks.summary"] = map[string]any{
		"status": "ready", "track_count": 6,
		"tracks": []any{map[string]any{"track_id": "1007", "name": "Vocal", "expanded_internal_payload": strings.Repeat("not-for-llm", 100000)}},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"project.structure"}}, read)
	if bundle.Omissions != nil || bundle.Status != "ready" || bundle.Views["project.structure"] == nil {
		t.Fatalf("project structure view was not disclosed: %+v", bundle)
	}
	data, _ := json.Marshal(bundle.Views)
	if strings.Contains(string(data), "expanded_internal_payload") || strings.Contains(string(data), "not-for-llm") {
		t.Fatalf("expanded project structure leaked through compact view: %s", data)
	}
}

func TestProjectStructureViewKeepsTopologyReadyWhenTechnicalProjectionIsPartial(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["observation.tim_projection"] = map[string]any{
		"status": "partial",
		"coverage": map[string]any{
			"sample_rate":   map[string]any{"status": "missing"},
			"channel_count": map[string]any{"status": "missing"},
			"bit_depth":     map[string]any{"status": "missing"},
		},
	}
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1.5",
		"intent":      "project_structure_observation",
		"project_structure": map[string]any{
			"status":      "ready",
			"track_count": 6,
		},
	}

	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"project.structure"}}, read)
	view, _ := bundle.Views["project.structure"].(map[string]any)
	if bundle.Status != "ready" || view == nil || view["status"] != "ready" {
		t.Fatalf("usable project topology was degraded by technical metadata: %+v", bundle)
	}
	facts := view["facts"].(map[string]any)
	if facts["observation.tim_projection"] == nil {
		t.Fatalf("partial technical projection was omitted instead of preserved: %+v", facts)
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

func TestObservationReceiptAuditsModelAndExecutedViews(t *testing.T) {
	req := FreeStateObservationRequest{
		RequestID:       "audit-1",
		OriginalViewIDs: []string{"track.peak_structure", "track.activity_structure"},
		ViewIDs:         []string{"track.peak_structure", "track.activity_structure"},
		RequestedBy:     "model",
		Scope:           "selected_track",
		TargetRef:       mixboard.TargetRef{Kind: "track", ID: "1007"},
	}
	bundle := AssembleFreeStateObservation(req, testFreeStateReadResult())
	receipt := bundle.AuditReceipt
	if receipt.SchemaVersion != FreeStateObservationReceiptSchema || receipt.RequestedBy != "model" || receipt.Status == "rejected" {
		t.Fatalf("receipt identity/status = %+v", receipt)
	}
	if !receipt.ViewSetMatches || !sameStringSet(receipt.ModelRequestedViewIDs, receipt.ActualExecutedViewIDs) {
		t.Fatalf("receipt view audit mismatch = %+v", receipt)
	}
	if receipt.Scope != "selected_track" || stringValue(receipt.Freshness["class"]) != "current_observation" {
		t.Fatalf("receipt scope/freshness = %+v", receipt)
	}
}

func TestObservationReceiptRejectsUnknownOrMissingModelViews(t *testing.T) {
	unknown := AssembleFreeStateObservation(FreeStateObservationRequest{
		OriginalViewIDs: []string{"track.peak_structure", "track.not_in_catalog"},
		ViewIDs:         []string{"track.peak_structure", "track.not_in_catalog"},
		RequestedBy:     "model",
	}, testFreeStateReadResult())
	if unknown.AuditReceipt.ViewSetMatches || unknown.AuditReceipt.Status != "rejected" || len(unknown.AuditReceipt.RejectionReasons) == 0 {
		t.Fatalf("unknown view was not rejected: %+v", unknown.AuditReceipt)
	}
	missing := NormalizeFreeStateObservationRequest(FreeStateObservationRequest{})
	if len(missing.ViewIDs) != 0 || len(FreeStateObservationReadKeys(missing, "1007")) != 1 {
		t.Fatalf("empty request was defaulted or loaded extra keys: %+v keys=%v", missing, FreeStateObservationReadKeys(missing, "1007"))
	}
}

func TestDOMViewsAreNeutralExplicitAndDimensionScoped(t *testing.T) {
	catalog := FreeStateObservationCatalogFor(mixboard.TargetRef{Kind: "track", ID: "1007"})
	catalogData, _ := json.Marshal(catalog)
	catalogText := strings.ToLower(string(catalogData))
	for _, forbidden := range []string{"limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics", "clipper"} {
		if strings.Contains(catalogText, forbidden) {
			t.Fatalf("DOM catalog injected processor family %q: %s", forbidden, catalogText)
		}
	}

	views := map[string]string{
		"track.peak_structure":        "peak_structure",
		"track.activity_structure":    "activity_structure",
		"track.frequency_time_events": "frequency_time_events",
		"track.transient_structure":   "transient_structure",
		"track.band_dynamics":         "band_dynamics",
	}
	for viewID, dimension := range views {
		keys := FreeStateObservationReadKeys(FreeStateObservationRequest{ViewIDs: []string{viewID}}, "1007")
		if !testContainsString(keys, "observation.dom_projection") {
			t.Fatalf("%s read keys = %v", viewID, keys)
		}
		bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{viewID}}, testFreeStateReadResult())
		data, _ := json.Marshal(bundle.Views[viewID])
		text := string(data)
		lowerText := strings.ToLower(text)
		if !strings.Contains(text, `"`+dimension+`"`) {
			t.Fatalf("%s omitted requested dimension: %s", viewID, text)
		}
		for _, forbidden := range []string{"limiter", "gate_threshold", "de_esser", "sibilance", "transient_shaper", "multiband_dynamics", "clipper", "plugin_instance_id", "parameter_id"} {
			if strings.Contains(lowerText, forbidden) {
				t.Fatalf("%s injected processor/control hint %q: %s", viewID, forbidden, text)
			}
		}
		for _, other := range views {
			if other != dimension && strings.Contains(text, `"`+other+`"`) {
				t.Fatalf("%s disclosed unrequested dimension %s: %s", viewID, other, text)
			}
		}
	}
}

func TestDOMViewIsNotLoadedWithoutExplicitRequest(t *testing.T) {
	keys := FreeStateObservationReadKeys(FreeStateObservationRequest{ViewIDs: []string{"track.basic_energy"}}, "1007")
	if testContainsString(keys, "observation.dom_projection") {
		t.Fatalf("basic energy request auto-loaded DOM: %v", keys)
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"track.basic_energy"}}, testFreeStateReadResult())
	data, _ := json.Marshal(bundle.Views)
	if strings.Contains(string(data), "dom_projection") || len(bundle.Views) != 1 {
		t.Fatalf("unrequested DOM view was disclosed: %s", data)
	}
}

func TestDOMRootStaleCannotBeOverriddenByReadyDimension(t *testing.T) {
	read := testFreeStateReadResult()
	domProjection := anyMap(read["items"].(map[string]any)["observation.dom_projection"])
	domProjection["status"] = "stale"
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"track.peak_structure"}}, read)
	if bundle.Omissions["track.peak_structure"] != orchestration.OmissionStale || bundle.Views["track.peak_structure"] != nil {
		t.Fatalf("stale DOM root was widened by ready dimension: %+v", bundle)
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

func TestMultitrackRelationshipDisclosesBoundedCandidateRows(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1.5", "intent": "project_multitrack_relation_observation",
		"multitrack_relation": map[string]any{
			"status": "ready", "track_count": 2,
			"compared_tracks": []any{map[string]any{"track_id": "a"}, map[string]any{"track_id": "b"}},
			"band_conflict_candidates": []any{map[string]any{
				"type": "mid_band_overlap_candidate", "status": "candidate", "band": "mid",
				"tracks": []any{map[string]any{"track_id": "a", "name": "A", "unit_energy": 0.6}, map[string]any{"track_id": "b", "name": "B", "unit_energy": 0.55}},
			}},
			"phase_risk_tracks": []any{}, "band_occupancy": []any{},
			"level_distribution": map[string]any{"status": "ready"},
		},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"mix.multitrack_relationship"}}, read)
	data, _ := json.Marshal(bundle.Views)
	if !strings.Contains(string(data), "mid_band_overlap_candidate") || !strings.Contains(string(data), `"track_id":"a"`) {
		t.Fatalf("bounded candidate rows missing: %s", data)
	}
}

func TestMultitrackRelationshipKeepsCoreReadyWhenAuxiliaryRowsArePartial(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	items["project.relationship_inputs"] = map[string]any{"status": "ready", "track_count": 6, "tracks_with_level_db": 6}
	items["project.rankings.level"] = map[string]any{"status": "partial", "rows": nil}
	items["project.rankings.peak"] = map[string]any{"status": "partial", "rows": nil}
	items["project.risks.headroom"] = map[string]any{"status": "partial", "rows": nil}
	items["project.attention.first"] = map[string]any{"status": "partial"}
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1.5", "intent": "project_multitrack_relation_observation",
		"multitrack_relation": map[string]any{
			"status": "ready", "track_count": 6, "compared_tracks": []any{map[string]any{"track_id": "a"}},
		},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{ViewIDs: []string{"mix.multitrack_relationship"}}, read)
	if bundle.Status != "ready" || bundle.Views["mix.multitrack_relationship"].(map[string]any)["status"] != "ready" {
		t.Fatalf("core multitrack relation was downgraded by auxiliary partial rows: %+v", bundle)
	}
}

func TestFrequencyRelationshipCompactsProjectInputsWithinDefaultBudget(t *testing.T) {
	read := testFreeStateReadResult()
	items := read["items"].(map[string]any)
	largeRows := make([]any, 24)
	for index := range largeRows {
		largeRows[index] = map[string]any{
			"track_id":       fmt.Sprintf("track-%02d", index),
			"status":         "ready",
			"tap_point":      "track_post_fader",
			"evidence_layer": strings.Repeat("unbounded-source-detail", 80),
			"evidence_ref":   strings.Repeat("evidence-ref-", 40),
		}
	}
	items["project.frequency_relationship_inputs"] = map[string]any{
		"schema_version":            "dad.project_frequency_relationship_inputs.v1",
		"status":                    "ready",
		"track_count":               24,
		"usable_track_count":        24,
		"tap_points":                []any{"track_post_fader"},
		"tracks":                    largeRows,
		"decision_tracks_truncated": false,
	}
	items["observation.mom_projection"] = map[string]any{
		"mom_version": "v1.4", "intent": "project_frequency_relationship_observation",
		"frequency_relationship": map[string]any{
			"status": "ready", "freshness": "fresh", "coverage": map[string]any{"track_count": 24},
		},
	}
	bundle := AssembleFreeStateObservation(FreeStateObservationRequest{
		ViewIDs: []string{"mix.frequency_relationship"}, MaxDisclosureBytes: 16 * 1024, MaxItems: 12,
	}, read)
	if bundle.Omissions != nil || bundle.Status != "ready" || bundle.Views["mix.frequency_relationship"] == nil {
		t.Fatalf("frequency view was not disclosed: %+v", bundle)
	}
	if bundle.DisclosureBytes >= bundle.MaxDisclosureBytes {
		t.Fatalf("compact frequency view used %d/%d bytes", bundle.DisclosureBytes, bundle.MaxDisclosureBytes)
	}
	data, _ := json.Marshal(bundle.Views)
	if strings.Contains(string(data), "unbounded-source-detail") || strings.Contains(string(data), "evidence-ref-evidence-ref") {
		t.Fatalf("expanded frequency inputs leaked through compact view: %s", data)
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
			"observation.dom_projection": map[string]any{
				"schema_version": "dom.projection.v1", "dom_version": "v1", "projection_id": "dom-1", "mode": "source_only", "status": "partial",
				"peak_structure":        map[string]any{"status": "ready", "peak_marker": true},
				"activity_structure":    map[string]any{"status": "ready", "activity_marker": true},
				"frequency_time_events": map[string]any{"status": "partial", "frequency_time_marker": true},
				"transient_structure":   map[string]any{"status": "partial", "transient_marker": true},
				"band_dynamics":         map[string]any{"status": "partial", "band_marker": true},
				"dimension_readiness": []any{
					map[string]any{"dimension": "peak_structure", "status": "ready"},
					map[string]any{"dimension": "activity_structure", "status": "ready"},
					map[string]any{"dimension": "frequency_time_events", "status": "partial"},
					map[string]any{"dimension": "transient_structure", "status": "partial"},
					map[string]any{"dimension": "band_dynamics", "status": "partial"},
				},
				"trust_quality": map[string]any{"overall_status": "partial", "can_support_source_description": true, "can_support_family_selection": true, "can_support_behavior_observation": false, "can_support_post_action_evaluation": false},
				"limitations":   []any{"unrequested_global_dimension_limitation"},
			},
			"track.1007.slow.stereo.summary": map[string]any{"status": "ready", "correlation_estimate": 0.8},
		},
	}
}

func testContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
