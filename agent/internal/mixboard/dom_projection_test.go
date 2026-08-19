package mixboard

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"vit-daw-agent/internal/dom"
)

func TestObservationConnectsDOMSourceOnlyCatalogReadContextAndPersistence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", root)
	store := NewStore("")
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix-dom-source", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"feature_snapshot": comSourceFeatureSnapshot("ready")},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection := result.Observation.DOMProjection
	if projection == nil || projection.Mode != dom.ModeSourceOnly || projection.Status != dom.StatusPartial {
		t.Fatalf("DOM source projection = %+v", projection)
	}
	if projection.ObservationID != result.Observation.ObservationID || projection.MixSessionID != "mix-dom-source" {
		t.Fatalf("DOM observation identity = %+v", projection)
	}
	if projection.ProcessorScope != nil || projection.TrustQuality.CanSupportBehaviorObservation || projection.TrustQuality.CanSupportPostActionEvaluation {
		t.Fatalf("DOM source-only authority widened = %+v", projection)
	}
	if !hasCatalogEntry(result.Observation.Catalog, "observation.dom_projection", "partial") {
		t.Fatalf("DOM catalog entry missing: %+v", result.Observation.Catalog.Entries)
	}
	if result.ContextPack.LatestObservation["dom_projection"] == nil || result.ContextPack.LatestObservation["dynamics_observation_context"] == nil {
		t.Fatalf("DOM context pack missing: %+v", result.ContextPack.LatestObservation)
	}
	read, err := store.Read(ReadRequest{MixSessionID: "mix-dom-source", ObservationID: result.Observation.ObservationID,
		Keys: []string{"observation.dom_projection"}})
	if err != nil {
		t.Fatal(err)
	}
	item := mapValue(mapValue(read["items"])["observation.dom_projection"])
	if cleanAnyString(item["projection_id"]) != projection.ProjectionID || cleanAnyString(item["mode"]) != dom.ModeSourceOnly {
		t.Fatalf("persisted DOM read = %+v", item)
	}
	persisted, err := os.ReadFile(result.ObservationPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(persisted), projection.ProjectionID) || !strings.Contains(string(persisted), `"dom_projection"`) {
		t.Fatal("canonical observation omitted DOM projection")
	}
	assertNoDOMContextLeak(t, dom.ContextProjection(*projection))
	assertNoDOMContextLeak(t, result.ContextPack.LatestObservation["dom_projection"])
}

func TestDOMFreshnessFailsClosedForStaleSource(t *testing.T) {
	obs := BuildObservation(Request{MixSessionID: "mix-dom-stale", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"feature_snapshot": comSourceFeatureSnapshot("stale")}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil || obs.DOMProjection.Status != dom.StatusStale || domProjectionFreshness(obs) != "stale" {
		t.Fatalf("stale DOM projection = %+v", obs.DOMProjection)
	}
	if obs.DOMProjection.TrustQuality.CanSupportFamilySelection || len(obs.DOMProjection.TrustQuality.BlockedReasons) == 0 {
		t.Fatalf("stale DOM evidence remained decision-capable: %+v", obs.DOMProjection.TrustQuality)
	}
}

func TestDOMProjectionConsumesGenericFineEvidenceFromFeatureSnapshot(t *testing.T) {
	level, contrast := -18.0, 8.0
	onset, body, sustain := -8.0, -15.0, -20.0
	attack, decay := 7.0, 5.0
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"waveform_envelope": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "clip_1", "duration_seconds": 4.0, "sample_rate": 48000.0, "channel_count": 2,
			"rms_dbfs": -20.0, "peak_dbfs": -1.0, "headroom_db": 1.0, "crest_db": 12.0,
			"time_segments": []any{
				map[string]any{"start_seconds": 0.0, "end_seconds": 2.0, "rms_dbfs": -18.0, "peak_dbfs": -2.0, "crest_db": 16.0, "energy_state": "active"},
				map[string]any{"start_seconds": 2.0, "end_seconds": 4.0, "rms_dbfs": -46.0, "peak_dbfs": -30.0, "crest_db": 16.0, "energy_state": "low"},
			},
			"noise_floor_evidence": map[string]any{"status": "ready", "estimate_dbfs": -52.0, "p10_dbfs": -55.0, "p50_dbfs": -48.0, "method": "bounded_rms_percentile_100ms", "window_count": 20},
		},
		"band_energy_summary": map[string]any{
			"status": "ready", "bands": map[string]any{
				"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0},
				"mid":  map[string]any{"status": "ready", "min_hz": 500.0, "max_hz": 2000.0, "energy_db": -20.0},
			},
			"frequency_time_events": map[string]any{"status": "ready", "coverage": 1.0, "events": []any{map[string]any{"start_seconds": 1.0, "end_seconds": 1.02, "band_id": "presence_high", "min_hz": 4500.0, "max_hz": 11000.0, "level_dbfs": level, "contrast_db": contrast}}},
			"transient_events":      map[string]any{"status": "ready", "coverage": 1.0, "events": []any{map[string]any{"onset_seconds": 1.0, "body_end_seconds": 1.08, "sustain_end_seconds": 1.35, "onset_dbfs": onset, "body_dbfs": body, "sustain_dbfs": sustain, "attack_body_contrast_db": attack, "sustain_decay_db": decay}}},
			"band_dynamics":         map[string]any{"status": "ready", "bands": []any{map[string]any{"id": "bass", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -40.0, "p50": -22.0, "p90": -10.0, "max": -5.0}, "crest_distribution": map[string]any{"count": 20, "min": 8.0, "p50": 12.0, "p90": 16.0, "max": 20.0}}, map[string]any{"id": "mid", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -38.0, "p50": -24.0, "p90": -14.0, "max": -7.0}, "crest_distribution": map[string]any{"count": 20, "min": 7.0, "p50": 11.0, "p90": 15.0, "max": 19.0}}}},
		},
	}
	obs := BuildObservation(Request{MixSessionID: "mix-dom-fine", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: map[string]any{"feature_snapshot": snapshot}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil || obs.DOMProjection.Status != dom.StatusReady {
		t.Fatalf("fine feature snapshot did not produce ready DOM: %+v", obs.DOMProjection)
	}
	if obs.DOMProjection.ActivityStructure.NoiseFloorStatus != dom.StatusReady || obs.DOMProjection.FrequencyTimeEvents.Status != dom.StatusReady || obs.DOMProjection.TransientStructure.Status != dom.StatusReady || obs.DOMProjection.BandDynamics.Status != dom.StatusReady {
		t.Fatalf("fine dimensions were not forwarded: %+v", obs.DOMProjection)
	}
}

func TestDOMProjectionPrefersFineEvidenceOverLaterWholeWindowRow(t *testing.T) {
	fine := map[string]any{
		"status": "ready", "track_id": "1007", "clip_id": "clip_1", "source_revision": "rev-1",
		"bands": map[string]any{
			"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0},
			"mid":  map[string]any{"status": "ready", "min_hz": 500.0, "max_hz": 2000.0, "energy_db": -20.0},
		},
		"band_dynamics": map[string]any{"status": "ready", "bands": []any{
			map[string]any{
				"id": "bass", "status": "ready",
				"time_distribution":  map[string]any{"count": 20, "min": -40.0, "p50": -22.0, "p90": -10.0, "max": -5.0},
				"crest_distribution": map[string]any{"count": 20, "min": 8.0, "p50": 12.0, "p90": 16.0, "max": 20.0},
			},
			map[string]any{
				"id": "mid", "status": "ready",
				"time_distribution":  map[string]any{"count": 20, "min": -38.0, "p50": -24.0, "p90": -14.0, "max": -7.0},
				"crest_distribution": map[string]any{"count": 20, "min": 7.0, "p50": 11.0, "p90": 15.0, "max": 19.0},
			},
		}},
	}
	wholeWindow := map[string]any{
		"status": "ready", "track_id": "1007", "clip_id": "clip_1", "source_revision": "rev-1", "updated_at": "later",
		"bands": map[string]any{"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0}},
	}
	snapshot := map[string]any{
		"schema_version":      "mixboard_feature_snapshot.v1",
		"latest_request":      map[string]any{"resolved_target": map[string]any{"track_id": "1007", "clip_id": "clip_1", "source_revision": "rev-1"}},
		"band_energy_summary": wholeWindow, "band_energy_summaries": []any{fine, wholeWindow},
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "1007", "clip_id": "clip_1", "source_revision": "rev-1", "duration_seconds": 4.0, "sample_rate": 48000.0, "channel_count": 2, "rms_dbfs": -20.0, "peak_dbfs": -1.0},
	}
	obs := BuildObservation(Request{MixSessionID: "mix-dom-best-fine", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: map[string]any{"feature_snapshot": snapshot}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil || obs.DOMProjection.BandDynamics == nil || obs.DOMProjection.BandDynamics.Status != dom.StatusReady {
		t.Fatalf("later whole-window row displaced fine DOM evidence: %+v", obs.DOMProjection)
	}
}

func TestBandStereoSpecialProjectionDoesNotAutoAttachDOM(t *testing.T) {
	obs := BuildObservation(Request{MixSessionID: "mix-band-stereo", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"projection": "band_stereo", "include_raw": false, "feature_snapshot": comSourceFeatureSnapshot("ready")}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection != nil || hasCatalogEntry(obs.Catalog, "observation.dom_projection", "partial") {
		t.Fatalf("specialized band/stereo projection auto-attached DOM: %+v", obs.DOMProjection)
	}
}

func assertNoDOMContextLeak(t *testing.T, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	for _, forbidden := range []string{"time_segments", "raw_samples", "plugin_instance_id", "topology_generation", "processor_state_hash", "parameter_id", "processor_family"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("DOM context leaked %q: %s", forbidden, text)
		}
	}
}

func TestDOMProjectionConditionsCarryUnifiedMeasurementFacts(t *testing.T) {
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"waveform_envelope": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "clip_1", "duration_seconds": 4.0, "sample_rate": 48000.0, "channel_count": 2,
			"source_revision": "source-rev", "clip_revision": "clip-rev", "coverage_ratio": 1.0,
			"rms_dbfs": -20.0, "peak_dbfs": -1.0, "headroom_db": 1.0, "crest_db": 12.0,
			"noise_floor_evidence": map[string]any{"status": "ready", "estimate_dbfs": -52.0, "p10_dbfs": -55.0, "p50_dbfs": -48.0, "method": "bounded_rms_percentile_100ms", "window_count": 20},
			"time_segments": []any{
				map[string]any{"start_seconds": 0.0, "end_seconds": 2.0, "rms_dbfs": -18.0, "peak_dbfs": -2.0, "crest_db": 16.0, "energy_state": "active"},
				map[string]any{"start_seconds": 2.0, "end_seconds": 4.0, "rms_dbfs": -46.0, "peak_dbfs": -30.0, "crest_db": 16.0, "energy_state": "low"},
			},
		},
		"band_energy_summary": map[string]any{
			"status": "ready", "track_id": "1007", "clip_id": "clip_1", "tap_point": "source_file_pre_fx", "render_mode": "offline_analysis",
			"source_revision": "source-rev", "clip_revision": "clip-rev", "sample_rate": 48000.0, "channel_count": 2,
			"coverage_ratio": 1.0, "coverage_seconds": 4.0, "window_ms": 10.0, "hop_ms": 5.0, "analyzer_version": "dad_l3_offline_analyzer.v1",
			"bands": map[string]any{
				"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0},
				"mid":  map[string]any{"status": "ready", "min_hz": 500.0, "max_hz": 2000.0, "energy_db": -20.0},
			},
			"frequency_time_events": map[string]any{"status": "ready", "coverage": 1.0, "event_count_available": true, "events": []any{map[string]any{"start_seconds": 1.0, "end_seconds": 1.02, "band_id": "mid", "min_hz": 500.0, "max_hz": 2000.0, "level_dbfs": -18.0, "contrast_db": 8.0}}},
			"transient_events":      map[string]any{"status": "ready", "coverage": 1.0, "events": []any{map[string]any{"onset_seconds": 1.0, "body_end_seconds": 1.08, "sustain_end_seconds": 1.35, "onset_dbfs": -8.0, "body_dbfs": -15.0, "sustain_dbfs": -20.0, "attack_body_contrast_db": 7.0, "sustain_decay_db": 5.0}}},
			"band_dynamics": map[string]any{"status": "ready", "bands": []any{
				map[string]any{"id": "bass", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -40.0, "p50": -22.0, "p90": -10.0, "max": -5.0}, "crest_distribution": map[string]any{"count": 20, "min": 8.0, "p50": 12.0, "p90": 16.0, "max": 20.0}},
				map[string]any{"id": "mid", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -38.0, "p50": -24.0, "p90": -14.0, "max": -7.0}, "crest_distribution": map[string]any{"count": 20, "min": 7.0, "p50": 11.0, "p90": 15.0, "max": 19.0}},
			}},
		},
	}
	obs := BuildObservation(Request{
		MixSessionID: "mix-dom-conditions", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		ProjectState: map[string]any{"project_revision": "project-rev", "project_uuid": "project-1"},
		Args:         map[string]any{"feature_snapshot": snapshot},
	}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil {
		t.Fatal("DOM projection missing")
	}
	conditions := obs.DOMProjection.Conditions
	if conditions.TapPoint != "source_file_pre_fx" || conditions.ProjectRevision != "project-rev" || conditions.SampleRate != 48000 || conditions.ChannelCount != 2 || conditions.WindowMS != 10 || conditions.HopMS != 5 || conditions.RenderMode != "offline_analysis" || conditions.MeasurementKey == "" {
		t.Fatalf("measurement conditions were not unified: %+v", conditions)
	}
	if obs.DOMProjection.Status != dom.StatusReady {
		t.Fatalf("complete conditioned DOM evidence was not ready: %+v", obs.DOMProjection)
	}
}

func TestDOMMaterializesWhenRequestedAlongsideBandStereoProjection(t *testing.T) {
	snapshot := comSourceFeatureSnapshot("ready")
	snapshot["band_energy_summary"] = map[string]any{
		"status": "ready", "track_id": "1007", "clip_id": "2001", "tap_point": "source_file_pre_fx",
		"source_revision": "source-1", "clip_revision": "clip-1", "sample_rate": 48000.0, "channel_count": 2,
		"coverage_ratio": 1.0, "coverage_seconds": 4.0, "window_ms": 10.0, "hop_ms": 5.0, "analyzer_version": "dad_l3_offline_analyzer.v1",
		"bands": map[string]any{
			"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0},
			"mid":  map[string]any{"status": "ready", "min_hz": 500.0, "max_hz": 2000.0, "energy_db": -20.0},
		},
		"noise_floor_evidence":  map[string]any{"status": "ready", "estimate_dbfs": -52.0, "p10_dbfs": -55.0, "p50_dbfs": -48.0, "method": "bounded_rms_percentile_100ms", "window_count": 20},
		"frequency_time_events": map[string]any{"status": "ready", "coverage": 1.0, "event_count_available": true, "events": []any{map[string]any{"start_seconds": 1.0, "end_seconds": 1.02, "band_id": "mid", "min_hz": 500.0, "max_hz": 2000.0, "level_dbfs": -18.0, "contrast_db": 8.0}}},
		"transient_events":      map[string]any{"status": "ready", "coverage": 1.0, "events": []any{map[string]any{"onset_seconds": 1.0, "body_end_seconds": 1.08, "sustain_end_seconds": 1.35, "onset_dbfs": -8.0, "body_dbfs": -15.0, "sustain_dbfs": -20.0, "attack_body_contrast_db": 7.0, "sustain_decay_db": 5.0}}},
		"band_dynamics": map[string]any{"status": "ready", "bands": []any{
			map[string]any{"id": "bass", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -40.0, "p50": -22.0, "p90": -10.0, "max": -5.0}, "crest_distribution": map[string]any{"count": 20, "min": 8.0, "p50": 12.0, "p90": 16.0, "max": 20.0}},
			map[string]any{"id": "mid", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -38.0, "p50": -24.0, "p90": -14.0, "max": -7.0}, "crest_distribution": map[string]any{"count": 20, "min": 7.0, "p50": 11.0, "p90": 15.0, "max": 19.0}},
		}},
	}
	obs := BuildObservation(Request{MixSessionID: "mix-dom-band", TargetRef: TargetRef{Kind: "track", ID: "1007"},
		Args: map[string]any{"projection": "frequency_stereo", "include_raw": false, "dom_mode": "source_only", "feature_snapshot": snapshot}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil || obs.DOMProjection.Status != dom.StatusReady {
		t.Fatalf("requested DOM was suppressed by band-stereo projection: %+v", obs.DOMProjection)
	}
	if !hasCatalogEntry(obs.Catalog, "observation.dom_projection", "fresh") {
		t.Fatalf("requested DOM catalog entry missing: %+v", obs.Catalog.Entries)
	}
}

func TestDOMProjectionSelectsTargetMatchingEvidenceRows(t *testing.T) {
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"waveform_envelope": map[string]any{
			"status": "ready", "track_id": "1032", "clip_id": "clip_vocals", "source_revision": "source-vocals", "clip_revision": "clip-vocals",
			"duration_seconds": 4.0, "sample_rate": 48000.0, "channel_count": 2, "rms_dbfs": -20.0, "peak_dbfs": -1.0,
		},
		"track_waveform_envelopes": []any{
			map[string]any{
				"status": "ready", "track_id": "1007", "clip_id": "clip_bass", "source_revision": "source-bass", "clip_revision": "clip-bass",
				"duration_seconds": 4.0, "sample_rate": 48000.0, "channel_count": 2, "rms_dbfs": -24.0, "peak_dbfs": -2.0,
				"time_segments": []any{
					map[string]any{"start_seconds": 0.0, "end_seconds": 2.0, "rms_dbfs": -22.0, "peak_dbfs": -3.0, "crest_db": 16.0, "energy_state": "active"},
					map[string]any{"start_seconds": 2.0, "end_seconds": 4.0, "rms_dbfs": -46.0, "peak_dbfs": -30.0, "crest_db": 16.0, "energy_state": "low"},
				},
			},
		},
		"band_energy_summary": map[string]any{"status": "ready", "track_id": "1032", "source_revision": "source-vocals", "clip_revision": "clip-vocals"},
		"band_energy_summaries": []any{
			map[string]any{
				"status": "ready", "track_id": "1007", "clip_id": "clip_bass", "tap_point": "source_file_pre_fx", "source_revision": "source-bass", "clip_revision": "clip-bass",
				"sample_rate": 48000.0, "channel_count": 2, "coverage_ratio": 1.0, "coverage_seconds": 4.0, "window_ms": 10.0, "hop_ms": 5.0,
				"analyzer_version": "dad_l3_offline_analyzer.v1", "render_mode": "offline_analysis",
				"bands": map[string]any{
					"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "energy_db": -14.0},
					"mid":  map[string]any{"status": "ready", "min_hz": 500.0, "max_hz": 2000.0, "energy_db": -20.0},
				},
				"noise_floor_evidence":  map[string]any{"status": "ready", "estimate_dbfs": -52.0, "p10_dbfs": -55.0, "p50_dbfs": -48.0, "method": "bounded_rms_percentile_100ms", "window_count": 20},
				"frequency_time_events": map[string]any{"status": "ready", "coverage": 1.0, "event_count_available": true, "events": []any{map[string]any{"start_seconds": 1.0, "end_seconds": 1.02, "band_id": "mid", "min_hz": 500.0, "max_hz": 2000.0, "level_dbfs": -18.0, "contrast_db": 8.0}}},
				"transient_events":      map[string]any{"status": "ready", "coverage": 1.0, "events": []any{map[string]any{"onset_seconds": 1.0, "body_end_seconds": 1.08, "sustain_end_seconds": 1.35, "onset_dbfs": -8.0, "body_dbfs": -15.0, "sustain_dbfs": -20.0, "attack_body_contrast_db": 7.0, "sustain_decay_db": 5.0}}},
				"band_dynamics": map[string]any{"status": "ready", "bands": []any{
					map[string]any{"id": "bass", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -40.0, "p50": -22.0, "p90": -10.0, "max": -5.0}, "crest_distribution": map[string]any{"count": 20, "min": 8.0, "p50": 12.0, "p90": 16.0, "max": 20.0}},
					map[string]any{"id": "mid", "status": "ready", "time_distribution": map[string]any{"count": 20, "min": -38.0, "p50": -24.0, "p90": -14.0, "max": -7.0}, "crest_distribution": map[string]any{"count": 20, "min": 7.0, "p50": 11.0, "p90": 15.0, "max": 19.0}},
				}},
			},
		},
	}
	obs := BuildObservation(Request{MixSessionID: "mix-dom-target-match", TargetRef: TargetRef{Kind: "track", ID: "1007"}, Args: map[string]any{"feature_snapshot": snapshot}}, "2026-08-08T00:00:00Z")
	if obs.DOMProjection == nil {
		t.Fatal("DOM projection missing")
	}
	if obs.DOMProjection.Conditions.SourceRevision != "source-bass" || obs.DOMProjection.Conditions.TapPoint != "source_file_pre_fx" || obs.DOMProjection.Conditions.EndSeconds != 4 {
		t.Fatalf("target-matching evidence was not selected: %+v", obs.DOMProjection.Conditions)
	}
	if obs.DOMProjection.ActivityStructure.ValidSegmentCount != 2 || obs.DOMProjection.ActivityStructure.NoiseFloorStatus != dom.StatusReady || obs.DOMProjection.FrequencyTimeEvents.Status != dom.StatusReady || obs.DOMProjection.TransientStructure.Status != dom.StatusReady || obs.DOMProjection.BandDynamics.Status != dom.StatusReady {
		t.Fatalf("target-matching fine evidence was not consumed: %+v", obs.DOMProjection)
	}
}
