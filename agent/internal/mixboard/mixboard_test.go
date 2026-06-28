package mixboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/mom"
)

func TestRequestObservationWritesBoardAndContextPack(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.Now = func() time.Time { return time.Date(2026, 6, 13, 1, 2, 3, 0, time.UTC) }

	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_test",
		Round:        1,
		GoalText:     "mix vocal",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1", Label: "Vocal"},
		ProjectState: map[string]any{
			"tracks": []any{
				map[string]any{
					"track_id": "track_1",
					"clips": []any{
						map[string]any{"clip_id": "clip_1", "start_seconds": 0, "length_seconds": 12},
					},
				},
			},
		},
		Args: map[string]any{
			"segment_seconds":       2.0,
			"feature_snapshot_path": filepath.Join(root, "missing_feature_snapshot.json"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q", result.Status)
	}
	for _, path := range []string{result.BoardPath, result.ObservationPath, result.ContextPackPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
		if filepath.Dir(path) == root {
			t.Fatalf("path should be session scoped: %s", path)
		}
	}
	if result.Observation.TimeRuler.DurationSeconds != 12 {
		t.Fatalf("duration = %v", result.Observation.TimeRuler.DurationSeconds)
	}
	if result.ContextPack.LatestObservation["timeline_digest"] != nil || result.ContextPack.LatestObservation["global_summary"] != nil {
		t.Fatalf("context pack leaked raw observation summary fields: %#v", result.ContextPack.LatestObservation)
	}
	if result.ContextPack.LatestObservation["digest"] != nil || result.ContextPack.LatestObservation["catalog"] != nil {
		t.Fatalf("context pack should expose digest/catalog through read_hints, got: %#v", result.ContextPack.LatestObservation)
	}
	if result.ContextPack.ActiveProblemMap != nil || result.ContextPack.RelevantSections != nil {
		t.Fatalf("context pack should not carry raw board problem/section rows: %#v", result.ContextPack)
	}
	if result.ContextPack.LatestObservation["mom_projection"] == nil || result.ContextPack.LatestObservation["llm_context"] == nil {
		t.Fatalf("context pack missing MOM context contract: %#v", result.ContextPack.LatestObservation)
	}
	if len(result.Observation.Digest) == 0 || len(result.Observation.Catalog.Entries) == 0 {
		t.Fatalf("observation missing digest/catalog: %#v", result.Observation)
	}
}

func TestDefaultRootPrefersVitDawDevRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "VitApp", "Workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_DEV_ROOT", root)
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	want := filepath.Join(root, "VitApp", "Workspace", "Artifacts", "mixboard")
	if got := DefaultRoot(); got != want {
		t.Fatalf("DefaultRoot = %q, want %q", got, want)
	}
}

func TestObservationUnavailableWithoutTimeRulerSource(t *testing.T) {
	obs := BuildObservation(Request{
		MixSessionID: "mix_empty",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		Args:         map[string]any{},
	}, time.Now().UTC().Format(time.RFC3339Nano))
	if obs.Status != "unavailable" {
		t.Fatalf("status = %q", obs.Status)
	}
	if obs.TimeRuler.SegmentSeconds <= 0 || obs.TimeRuler.FrameSeconds <= 0 {
		t.Fatalf("time ruler defaults missing: %#v", obs.TimeRuler)
	}
}

func TestObservationConsumesFeatureSnapshot(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-13T08:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.25,"peak_abs":0.8,"float_count":128,
			"time_segments":[
				{"start_seconds":0,"end_seconds":2,"rms":0.1,"rms_dbfs":-20,"peak_abs":0.4,"peak_dbfs":-7.959,"crest_db":12.041,"energy_state":"medium"},
				{"start_seconds":2,"end_seconds":4,"rms":0.01,"rms_dbfs":-40,"peak_abs":0.05,"peak_dbfs":-26.021,"crest_db":13.979,"energy_state":"low"}
			]},
		"band_energy_summary":{"status":"ready","source":"live_level_meter_spectrum","bands":{
			"sub":{"unit_energy":0.1,"energy_db":-20,"min_hz":20,"max_hz":60},
			"bass":{"unit_energy":0.2,"energy_db":-13.979,"min_hz":60,"max_hz":160},
			"low_mid":{"unit_energy":0.4,"energy_db":-7.959,"min_hz":160,"max_hz":500},
			"mid":{"unit_energy":0.3,"energy_db":-10.458,"min_hz":500,"max_hz":2000},
			"presence":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":2000,"max_hz":6000},
			"air":{"unit_energy":0.15,"energy_db":-16.478,"min_hz":6000,"max_hz":16000}
		}},
		"stereo_relation_summary":{"status":"ready","source":"live_level_meter_stereo","left_level_db":-14.2,"right_level_db":-15.1,"balance_db":0.9,"balance_unit":-0.05,"balance_state":"centered","phase_deviation":0.08,"phase_negative_ratio":0.02,"correlation_estimate":0.84,"correlation_state":"stable","bin_count":96},
		"spectrogram_tiles":{"status":"ready","track_id":"track_1","clip_id":"clip_1","tile_count_seen":2,"tile_count_expected":2,"total_duration":12}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_featured",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"segment_seconds":       2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" {
		t.Fatalf("status = %q", result.Status)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform status = %q", got)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "ready" {
		t.Fatalf("spectrogram status = %q", got)
	}
	if len(result.Board.OpenBlockers) != 0 {
		t.Fatalf("blockers = %#v", result.Board.OpenBlockers)
	}
	if result.Observation.GlobalSummary["peak_dbfs"] == nil {
		t.Fatalf("expected peak_dbfs in summary: %#v", result.Observation.GlobalSummary)
	}
	if got := result.Observation.TimelineDigest[0]["energy_state"]; got != "medium" {
		t.Fatalf("timeline digest did not consume time segments: %#v", result.Observation.TimelineDigest)
	}
	if result.Board.PackageStatus["environment"] != "ready" || result.Board.PackageStatus["mix"] != "baseline_ready" {
		t.Fatalf("package status = %#v", result.Board.PackageStatus)
	}
	if result.ContextPack.LatestObservation["digest"] != nil || result.ContextPack.LatestObservation["catalog"] != nil {
		t.Fatalf("context pack should keep digest/catalog behind read_hints: %#v", result.ContextPack.LatestObservation)
	}
	if result.ContextPack.LatestObservation["mix_package"] != nil || result.ContextPack.LatestObservation["environment_package"] != nil || result.ContextPack.LatestObservation["deep_package"] != nil || result.ContextPack.LatestObservation["global_summary"] != nil {
		t.Fatalf("context pack should disclose packages through catalog reads, got: %#v", result.ContextPack.LatestObservation)
	}
	mixPkg := result.Observation.MixPackage
	sourceCaps, _ := mixPkg["source_capabilities"].(map[string]string)
	if sourceCaps["band_energy"] != "ready" {
		t.Fatalf("mix package source caps = %#v", mixPkg["source_capabilities"])
	}
	if sourceCaps["stereo_correlation"] != "ready" {
		t.Fatalf("mix package source caps = %#v", mixPkg["source_capabilities"])
	}
	metrics, _ := mixPkg["current_metrics"].(map[string]any)
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["correlation_state"] != "stable" || stereo["balance_state"] != "centered" {
		t.Fatalf("stereo relation metrics missing: %#v", stereo)
	}
	for _, raw := range mixPkg["missing_metrics"].([]string) {
		if raw == "band_energy_summary" {
			t.Fatalf("band energy should not be missing: %#v", mixPkg)
		}
		if raw == "stereo_correlation" {
			t.Fatalf("stereo correlation should not be missing: %#v", mixPkg)
		}
	}
}

func TestRealtimeMetricsDoNotOverridePreparedBandStereo(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-23T16:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7,"float_count":128},
		"spectrogram_tiles":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"kernel_prepared_telemetry","source_revision":"rev_prepared","tile_count_seen":2,"tile_count_expected":2,"coverage_seconds":10},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"spectral_tile_derived","materialized_by":"kernel_prepared_telemetry","source_revision":"rev_prepared","coverage_seconds":10,
			"bands":{"bass":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":60,"max_hz":160}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"spectral_tile_derived","materialized_by":"kernel_prepared_telemetry","source_revision":"rev_prepared","coverage_seconds":10,
			"balance_db":0.1,"balance_state":"centered","correlation_estimate":0.82,"correlation_state":"stable"},
		"realtime_band_energy_summary":{"status":"ready","feature_type":"realtime_band_energy_summary","layer":"l2_realtime","track_id":"track_1","clip_id":"clip_1","source":"live_level_meter_spectrum",
			"bands":{"bass":{"unit_energy":0.8,"energy_db":-1.938,"min_hz":60,"max_hz":160}}},
		"realtime_stereo_relation_summary":{"status":"ready","feature_type":"realtime_stereo_relation_summary","layer":"l2_realtime","track_id":"track_1","clip_id":"clip_1","source":"live_level_meter_stereo",
			"balance_db":-0.4,"balance_state":"centered","correlation_estimate":0.65,"correlation_state":"watch"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_l2_split",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 10},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	current := mapValue(result.Observation.MixPackage["current_metrics"])
	realtime := mapValue(result.Observation.MixPackage["realtime_metrics"])
	currentBand := mapValue(current["band_energy"])
	realtimeBand := mapValue(realtime["band_energy"])
	if got := cleanAnyString(currentBand["source"]); got != "spectral_tile_derived" {
		t.Fatalf("prepared band source overridden by realtime source: %#v", currentBand)
	}
	if got := cleanAnyString(realtimeBand["source"]); got != "live_level_meter_spectrum" {
		t.Fatalf("realtime band source missing: %#v", realtimeBand)
	}
	currentStereo := mapValue(current["stereo_relation"])
	realtimeStereo := mapValue(realtime["stereo_relation"])
	if got := cleanAnyString(currentStereo["source"]); got != "spectral_tile_derived" {
		t.Fatalf("prepared stereo source overridden by realtime source: %#v", currentStereo)
	}
	if got := cleanAnyString(realtimeStereo["source"]); got != "live_level_meter_stereo" {
		t.Fatalf("realtime stereo source missing: %#v", realtimeStereo)
	}
	caps := result.Observation.SourceCapabilities
	if caps["band_energy"] != "ready" || caps["realtime_band_energy"] != "ready" {
		t.Fatalf("capabilities did not keep L3/L2 separately: %#v", caps)
	}
}

func TestRealtimeProjectionKeepsL2CatalogAndReadKeys(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7,"float_count":128},
		"spectrogram_tiles":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"kernel_prepared_telemetry","source_revision":"rev_prepared","tile_count_seen":2,"tile_count_expected":2,"coverage_seconds":10},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"spectral_tile_derived","source_revision":"rev_prepared","coverage_seconds":10,
			"bands":{"bass":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":60,"max_hz":160}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"spectral_tile_derived","source_revision":"rev_prepared","coverage_seconds":10,
			"balance_db":0.1,"balance_state":"centered","correlation_estimate":0.82,"correlation_state":"stable"},
		"realtime_band_energy_summary":{"status":"ready","feature_type":"realtime_band_energy_summary","layer":"l2_realtime","track_id":"track_1","clip_id":"clip_1","source":"live_level_meter_spectrum",
			"bands":{"bass":{"unit_energy":0.8,"energy_db":-1.938,"min_hz":60,"max_hz":160}}},
		"realtime_stereo_relation_summary":{"status":"ready","feature_type":"realtime_stereo_relation_summary","layer":"l2_realtime","track_id":"track_1","clip_id":"clip_1","source":"live_level_meter_stereo",
			"balance_db":-0.4,"balance_state":"centered","correlation_estimate":0.65,"correlation_state":"watch"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_l2_projection",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 10},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"projection":            "frequency_stereo",
			"include_raw":           false,
			"capture_mode":          "realtime_playback",
			"requested_layer":       "l2_realtime",
			"feature_keys":          []any{"realtime_band_energy_summary", "realtime_stereo_relation_summary", "band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	global := result.Observation.GlobalSummary
	if got := cleanAnyString(mapValue(global["realtime_band_energy_summary"])["source"]); got != "live_level_meter_spectrum" {
		t.Fatalf("projection missing realtime band summary: %#v", global)
	}
	if got := cleanAnyString(mapValue(global["realtime_stereo_relation_summary"])["source"]); got != "live_level_meter_stereo" {
		t.Fatalf("projection missing realtime stereo summary: %#v", global)
	}
	catalogText := fmt.Sprint(result.Observation.Catalog.Entries)
	for _, want := range []string{".realtime.band_energy.summary", ".realtime.stereo.summary"} {
		if !strings.Contains(catalogText, want) {
			t.Fatalf("projection catalog missing %s: %s", want, catalogText)
		}
	}
	read, err := store.Read(ReadRequest{
		MixSessionID: result.Observation.MixSessionID,
		Keys: []string{
			"track.track_1.realtime.band_energy.summary",
			"track.track_1.realtime.stereo.summary",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(read["items"])
	if got := cleanAnyString(mapValue(items["track.track_1.realtime.band_energy.summary"])["source"]); got != "live_level_meter_spectrum" {
		t.Fatalf("mix.read realtime band source = %q items=%#v", got, items)
	}
	if got := cleanAnyString(mapValue(items["track.track_1.realtime.stereo.summary"])["source"]); got != "live_level_meter_stereo" {
		t.Fatalf("mix.read realtime stereo source = %q items=%#v", got, items)
	}
	if result.Observation.MOMProjection == nil {
		t.Fatalf("missing MOM projection")
	}
	if got := result.Observation.MOMProjection.Intent; got != "realtime_band_stereo_observation" {
		t.Fatalf("MOM realtime intent = %q", got)
	}
	if got := result.Observation.MOMProjection.Layers.TimbreFrequency.Facts["primary_layer"]; got != "l2_realtime" {
		t.Fatalf("MOM realtime primary layer = %v", got)
	}
}

func TestRealtimeProjectionPromotesReadyRowsOverDeferredSingleton(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{
			"request_id":"req_live",
			"resolved_target":{"track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","source_path":"D:/audio/Paper Crown.mp3"}
		},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","rms":0.2,"peak_abs":0.7,"float_count":128},
		"spectrogram_tiles":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","tile_count_seen":44,"tile_count_expected":44,"coverage_ratio":1},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current",
			"bands":{"bass":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":60,"max_hz":160}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current",
			"balance_db":0.1,"balance_state":"centered","correlation_estimate":0.82,"correlation_state":"stable"},
		"realtime_band_energy_summary":{"status":"deferred","reason":"phase_4_5_l2_realtime_requires_playback"},
		"realtime_stereo_relation_summary":{"status":"deferred","reason":"phase_4_5_l2_realtime_requires_playback"},
		"realtime_band_energy_summaries":[{
			"schema_version":"dad_feature.v1.1",
			"status":"ready",
			"feature_type":"realtime_band_energy_summary",
			"layer":"l2_realtime",
			"track_id":"track_1",
			"clip_id":"clip_1",
			"file_path":"D:/audio/Paper Crown.mp3",
			"source_revision":"rev_current",
			"source":"live_level_meter_spectrum",
			"capture_mode":"realtime_playback",
			"tap_point":"post_fader_live_meter",
			"quality_status":"ready",
			"quality_reason":"live_level_meter_payload_nonzero",
			"bands":{"bass":{"unit_energy":0.8,"energy_db":-1.938,"min_hz":60,"max_hz":160}}
		}],
		"realtime_stereo_relation_summaries":[{
			"schema_version":"dad_feature.v1.1",
			"status":"ready",
			"feature_type":"realtime_stereo_relation_summary",
			"layer":"l2_realtime",
			"track_id":"track_1",
			"clip_id":"clip_1",
			"file_path":"D:/audio/Paper Crown.mp3",
			"source_revision":"rev_current",
			"source":"live_level_meter_stereo",
			"capture_mode":"realtime_playback",
			"tap_point":"post_fader_live_meter",
			"quality_status":"ready",
			"quality_reason":"live_level_meter_payload_nonzero",
			"balance_db":1.488,
			"balance_state":"right_heavy",
			"correlation_estimate":0.677,
			"correlation_state":"stable"
		}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_l2_projection_rows",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 10},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"projection":            "frequency_stereo",
			"include_raw":           false,
			"feature_keys":          []any{"realtime_band_energy_summary", "realtime_stereo_relation_summary", "band_energy_summary", "stereo_relation_summary", "spectrogram_tiles"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	global := result.Observation.GlobalSummary
	realtimeBand := mapValue(global["realtime_band_energy_summary"])
	if got := cleanAnyString(realtimeBand["status"]); got != "ready" {
		t.Fatalf("projection kept deferred realtime band singleton: %#v", global)
	}
	if got := cleanAnyString(realtimeBand["capture_mode"]); got != "realtime_playback" {
		t.Fatalf("projection dropped realtime band evidence fields: %#v", realtimeBand)
	}
	if got := numberFromMap(mapValue(mapValue(realtimeBand["bands"])["bass"]), "energy_db"); got != -1.938 {
		t.Fatalf("projection did not expose ready realtime band values: %#v", realtimeBand)
	}
	realtimeStereo := mapValue(global["realtime_stereo_relation_summary"])
	if got := cleanAnyString(realtimeStereo["status"]); got != "ready" {
		t.Fatalf("projection kept deferred realtime stereo singleton: %#v", global)
	}
	if got := cleanAnyString(realtimeStereo["quality_reason"]); got != "live_level_meter_payload_nonzero" {
		t.Fatalf("projection dropped realtime stereo quality evidence: %#v", realtimeStereo)
	}
	if got := numberFromMap(realtimeStereo, "correlation_estimate"); got != 0.677 {
		t.Fatalf("projection did not expose ready realtime stereo values: %#v", realtimeStereo)
	}
	caps := result.Observation.SourceCapabilities
	if caps["realtime_band_energy"] != "ready" || caps["realtime_stereo_relation"] != "ready" {
		t.Fatalf("projection capabilities are not ready: %#v", caps)
	}
	realtimeMetrics := mapValue(result.Observation.MixPackage["realtime_metrics"])
	if got := cleanAnyString(mapValue(realtimeMetrics["band_energy"])["status"]); got != "ready" {
		t.Fatalf("mix_package realtime band not promoted: %#v", realtimeMetrics)
	}
	if got := cleanAnyString(mapValue(realtimeMetrics["stereo_relation"])["status"]); got != "ready" {
		t.Fatalf("mix_package realtime stereo not promoted: %#v", realtimeMetrics)
	}
}

func TestGeneralObservationContextDoesNotExpandRealtimeValues(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{"request_id":"req_general","resolved_target":{"track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","tile_count_seen":4,"tile_count_expected":4,"coverage_ratio":1},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","source":"spectral_tile_derived",
			"bands":{"bass":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":60,"max_hz":160}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","source":"spectral_tile_derived",
			"balance_db":0.1,"balance_state":"centered","correlation_estimate":0.82,"correlation_state":"stable"},
		"realtime_band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","source":"live_level_meter_spectrum","tap_point":"unknown_live_meter",
			"bands":{"bass":{"unit_energy":0.88,"energy_db":-1.11,"min_hz":60,"max_hz":160}}},
		"realtime_stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","source":"live_level_meter_stereo","tap_point":"unknown_live_meter",
			"balance_db":1.234,"balance_state":"right_heavy","correlation_estimate":0.654,"correlation_state":"stable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_general_context_contract",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		GoalText:     "观察一下当前工程的频段和声像状态，不要修改。",
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"projection":            "frequency_stereo",
			"include_raw":           false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.MOMProjection.Intent; got != "general_band_stereo_observation" {
		t.Fatalf("intent = %q", got)
	}
	if got := result.Observation.MOMProjection.Layers.TimbreFrequency.Facts["primary_layer"]; got != "l3_deep" {
		t.Fatalf("primary layer = %v", got)
	}
	fullObservation, _ := json.Marshal(result.Observation)
	if !strings.Contains(string(fullObservation), "live_level_meter_spectrum") {
		t.Fatalf("audit observation should retain realtime evidence when present: %s", string(fullObservation))
	}
	contextData, _ := json.Marshal(result.ContextPack)
	contextText := string(contextData)
	for _, forbidden := range []string{"unit_energy\":0.88", "energy_db\":-1.11", "balance_db\":1.234", "correlation_estimate\":0.654", "realtime_metrics", "global_summary", "timeline_digest"} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("general context leaked realtime/raw field %s in %s", forbidden, contextText)
		}
	}
	for _, want := range []string{"general_band_stereo_observation", "timbre_frequency.l3_full_song", "space_stereo.l3_full_song", "L2 realtime available only as status"} {
		if !strings.Contains(contextText, want) {
			t.Fatalf("general context missing %s in %s", want, contextText)
		}
	}
}

func TestFrequencyStereoProjectionOmitsRawWaveformTimeSegments(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","duration_seconds":12,"rms":0.2,"peak_abs":0.7,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.7,"energy_state":"high"}]},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","duration_seconds":12,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.7,"energy_state":"high"}]}
		],
		"spectrogram_tiles":{"status":"partial","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","duration_seconds":12,"tile_count_seen":3,"tile_count_expected":8,"coverage_seconds":6},
		"band_energy_summary":{"status":"partial","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","duration_seconds":12,"coverage_seconds":6,
			"bands":{"bass":{"status":"partial","energy_db":-18,"unit_energy":0.12}}},
		"stereo_relation_summary":{"status":"partial","track_id":"track_1","clip_id":"clip_1","source_revision":"rev_current","duration_seconds":12,"coverage_seconds":6,
			"balance_db":0.2,"balance_state":"centered","correlation_estimate":0.72,"correlation_state":"stable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	status := map[string]any{
		"schema_version":   "acoustic_package_status.v0",
		"status":           "partial",
		"project_id":       "current",
		"session_id":       "mix_projection",
		"track_id":         "track_1",
		"clip_id":          "clip_1",
		"source_revision":  "rev_current",
		"duration_seconds": 12,
		"source_identity":  map[string]any{"project_id": "current", "session_id": "mix_projection", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_current", "duration_seconds": 12},
		"package_layers": map[string]any{
			"l1_static": map[string]any{"status": "ready", "features": map[string]any{
				"waveform_envelope": map[string]any{"status": "ready"},
				"peak_rms_summary":  map[string]any{"status": "ready"},
				"time_energy":       map[string]any{"status": "ready"},
			}},
			"l2_realtime": map[string]any{"status": "deferred"},
			"l3_deep": map[string]any{"status": "partial", "features": map[string]any{
				"spectrogram_tiles":       map[string]any{"status": "partial", "progress": map[string]any{"tile_count_seen": 3, "tile_count_expected": 8, "coverage_seconds": 6, "coverage_ratio": 0.5}},
				"band_energy_summary":     map[string]any{"status": "partial"},
				"stereo_relation_summary": map[string]any{"status": "partial"},
			}},
		},
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_projection",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path":   snapshotPath,
			"projection":              "frequency_stereo",
			"include_raw":             false,
			"feature_keys":            []any{"band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"},
			"acoustic_package_status": status,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result.Observation)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{`"time_segments":`, `"track_waveform_envelopes":`, "raw.time_energy.range", "waveform_fallback", "l1_static_fallback"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("projection leaked %s in %s", forbidden, text)
		}
	}
	for _, required := range []string{"band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"} {
		if !strings.Contains(text, required) {
			t.Fatalf("projection missing %s in %s", required, text)
		}
	}
	if result.Observation.MOMProjection == nil {
		t.Fatalf("missing MOM projection")
	}
	if result.Observation.MOMProjection.MOMVersion != mom.Version {
		t.Fatalf("MOM version = %q", result.Observation.MOMProjection.MOMVersion)
	}
	if result.Observation.MOMProjection.LLMContext.DoNotIncludeRawPackage != true {
		t.Fatalf("MOM raw package guard not set: %#v", result.Observation.MOMProjection.LLMContext)
	}
	if got := result.Observation.MOMProjection.Layers.TimbreFrequency.Facts["primary_layer"]; got != "l3_deep" {
		t.Fatalf("MOM general primary layer = %v", got)
	}
	if result.Observation.MOMProjection.IntentPolicy.Name == "" {
		t.Fatalf("MOM intent policy missing: %#v", result.Observation.MOMProjection)
	}
	if result.Observation.MOMProjection.ProjectMixProfile.TrackCount != 0 {
		t.Fatalf("single-track projection should not fabricate project tracks: %#v", result.Observation.MOMProjection.ProjectMixProfile)
	}
	if result.ContextPack.LatestObservation["mom_projection"] == nil || result.ContextPack.LatestObservation["llm_context"] == nil {
		t.Fatalf("context pack missing MOM projection: %#v", result.ContextPack.LatestObservation)
	}
	contextData, _ := json.Marshal(result.ContextPack)
	contextText := string(contextData)
	for _, forbidden := range []string{`"time_segments":`, `"track_waveform_envelopes":`, "raw.time_energy.range", `"spectrogram_tile_rows":`, "full_acoustic_package", "global_summary", "timeline_digest"} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("context pack leaked raw field %s in %s", forbidden, contextText)
		}
	}
	catalogText := fmt.Sprint(result.Observation.Catalog.Entries)
	if !strings.Contains(catalogText, "observation.mom_projection") {
		t.Fatalf("catalog missing MOM projection entry: %s", catalogText)
	}
	read, err := store.Read(ReadRequest{
		MixSessionID: result.Observation.MixSessionID,
		Keys:         []string{"observation.mom_projection"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(read["items"])
	if mapValue(items["observation.mom_projection"])["mom_version"] != mom.Version {
		t.Fatalf("mix.read missing MOM projection: %#v", items)
	}
	if len(data) > 48000 {
		t.Fatalf("projection payload too large: %d bytes", len(data))
	}
}

func TestProjectObserveBuildsCurrentMOMMultitrackProjection(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-25T08:00:00Z",
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"vocal_1","clip_id":"clip_v","rms":0.2,"peak_abs":0.6},
			{"status":"ready","track_id":"bass_1","clip_id":"clip_b","rms":0.25,"peak_abs":0.7}
		],
		"band_energy_summaries":[
			{"status":"ready","track_id":"vocal_1","clip_id":"clip_v","bands":{
				"presence":{"unit_energy":0.32,"energy_db":-9.897,"min_hz":2000,"max_hz":6000},
				"mid":{"unit_energy":0.20,"energy_db":-13.979,"min_hz":500,"max_hz":2000}
			}},
			{"status":"ready","track_id":"bass_1","clip_id":"clip_b","bands":{
				"bass":{"unit_energy":0.41,"energy_db":-7.744,"min_hz":60,"max_hz":160},
				"sub":{"unit_energy":0.28,"energy_db":-11.057,"min_hz":20,"max_hz":60}
			}}
		],
		"stereo_relation_summaries":[
			{"status":"ready","track_id":"vocal_1","clip_id":"clip_v","balance_db":0.3,"balance_state":"centered","correlation_estimate":0.80,"correlation_state":"stable","phase_negative_ratio":0.02},
			{"status":"ready","track_id":"bass_1","clip_id":"clip_b","balance_db":-0.2,"balance_state":"centered","correlation_estimate":0.93,"correlation_state":"stable","phase_negative_ratio":0.01}
		],
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_project_mom_v12",
		GoalText:     "比较一下各轨频段占用和声像关系，不要修改。",
		TargetRef:    TargetRef{Kind: "project", ID: "current", Label: "Current project"},
		ListenScope: ListenScope{
			Source: ListenSourceScope{Mode: "full_project"},
		},
		ProjectState: map[string]any{
			"tracks": []any{
				map[string]any{
					"track_id":    "vocal_1",
					"track_name":  "Lead Vocal",
					"level_db":    -18.0,
					"peak_dbfs":   -3.0,
					"headroom_db": 3.0,
					"clips":       []any{map[string]any{"clip_id": "clip_v", "clip_name": "lead_vocal", "length_seconds": 12.0}},
				},
				map[string]any{
					"track_id":    "bass_1",
					"track_name":  "Bass",
					"level_db":    -16.0,
					"peak_dbfs":   -2.0,
					"headroom_db": 2.0,
					"clips":       []any{map[string]any{"clip_id": "clip_b", "clip_name": "bass", "length_seconds": 12.0}},
				},
			},
		},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"projection":            "frequency_stereo",
			"include_raw":           false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.MOMProjection == nil {
		t.Fatalf("missing MOM projection")
	}
	proj := result.Observation.MOMProjection
	if proj.MOMVersion != mom.Version {
		t.Fatalf("mom version = %q", proj.MOMVersion)
	}
	if proj.Intent != "project_multitrack_relation_observation" {
		t.Fatalf("intent = %q", proj.Intent)
	}
	if proj.ProjectMixProfile.TrackCount != 2 {
		t.Fatalf("project mix profile = %#v", proj.ProjectMixProfile)
	}
	if proj.MultitrackRelation.TrackCount != 2 || proj.MultitrackRelation.Status != "ready" {
		t.Fatalf("multitrack relation = %#v", proj.MultitrackRelation)
	}
	projectTracks := mapRowsAny(result.Observation.ProjectPackage["tracks"])
	if len(projectTracks) != 2 {
		t.Fatalf("project tracks = %#v", result.Observation.ProjectPackage)
	}
	for _, track := range projectTracks {
		if mapValue(track["band_energy"])["status"] != "ready" {
			t.Fatalf("track band energy missing: %#v", track)
		}
		if mapValue(track["stereo_relation"])["status"] != "ready" {
			t.Fatalf("track stereo relation missing: %#v", track)
		}
	}
	contextData, err := json.Marshal(result.ContextPack)
	if err != nil {
		t.Fatal(err)
	}
	contextText := string(contextData)
	for _, forbidden := range []string{`"time_segments":`, `"track_waveform_envelopes":`, `"spectrogram_tile_rows":`} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("context pack leaked raw field %s in %s", forbidden, contextText)
		}
	}
	if result.ContextPack.LatestObservation["mom_projection"] == nil {
		t.Fatalf("context pack missing projection: %#v", result.ContextPack.LatestObservation)
	}
}

func TestReadySnapshotWithStableSourceIdentitySurvivesNewRequestID(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{"request_id":"req_new","resolved_target":{"track_id":"track_1","clip_id":"clip_1"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source_revision":"rev_stable","source_fingerprint":"rev_stable","source_hash":"sha256:abc","duration_seconds":12,"rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source_revision":"rev_stable","source_fingerprint":"rev_stable","tile_count_seen":2,"tile_count_expected":2},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source_revision":"rev_stable","source_fingerprint":"rev_stable","bands":{"bass":{"energy_db":-18}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source_revision":"rev_stable","source_fingerprint":"rev_stable","correlation_state":"stable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_snapshot_freshness",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{
			"duration_seconds": 12,
			"tracks": []any{map[string]any{
				"track_id": "track_1",
				"clips":    []any{map[string]any{"id": "clip_1", "length_seconds": 12}},
			}},
		},
		Args: map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform capability = %q", got)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "ready" {
		t.Fatalf("spectrogram capability = %q", got)
	}
	metrics := result.Observation.MixPackage["current_metrics"].(map[string]any)
	if got := metrics["band_energy"].(map[string]any)["status"]; got != "ready" {
		t.Fatalf("band status = %v metrics=%+v", got, metrics)
	}
}

func TestFullProjectReadyTrackWaveformsSurviveLatestSingleTarget(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{"request_id":"kernel_prepared_waveform_envelope_clip_4","resolved_target":{"track_id":"track_4","clip_id":"clip_4"}},
		"waveform_envelope":{"status":"ready","track_id":"track_4","clip_id":"clip_4","request_id":"kernel_prepared_waveform_envelope_clip_4","source_revision":"rev_4","source_fingerprint":"rev_4","duration_seconds":12,"rms":0.4,"peak_abs":0.8},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","source_revision":"rev_1","source_fingerprint":"rev_1","duration_seconds":12,"rms":0.1,"peak_abs":0.5},
			{"status":"ready","track_id":"track_2","clip_id":"clip_2","request_id":"kernel_prepared_waveform_envelope_clip_2","source_revision":"rev_2","source_fingerprint":"rev_2","duration_seconds":12,"rms":0.2,"peak_abs":0.6},
			{"status":"ready","track_id":"track_3","clip_id":"clip_3","request_id":"kernel_prepared_waveform_envelope_clip_3","source_revision":"rev_3","source_fingerprint":"rev_3","duration_seconds":12,"rms":0.3,"peak_abs":0.7},
			{"status":"ready","track_id":"track_4","clip_id":"clip_4","request_id":"kernel_prepared_waveform_envelope_clip_4","source_revision":"rev_4","source_fingerprint":"rev_4","duration_seconds":12,"rms":0.4,"peak_abs":0.8}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_full_project_waveforms",
		TargetRef:    TargetRef{Kind: "project", ID: "current"},
		ListenScope:  ListenScope{Source: ListenSourceScope{Mode: "full_project"}},
		ProjectState: map[string]any{
			"duration_seconds": 12,
			"tracks": []any{
				map[string]any{"track_id": "track_1", "clips": []any{map[string]any{"clip_id": "clip_1", "length_seconds": 12}}},
				map[string]any{"track_id": "track_2", "clips": []any{map[string]any{"clip_id": "clip_2", "length_seconds": 12}}},
				map[string]any{"track_id": "track_3", "clips": []any{map[string]any{"clip_id": "clip_3", "length_seconds": 12}}},
				map[string]any{"track_id": "track_4", "clips": []any{map[string]any{"clip_id": "clip_4", "length_seconds": 12}}},
			},
		},
		Args: map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.ProjectPackage["active_acoustic_track_count"]; got != 4 {
		t.Fatalf("active_acoustic_track_count = %v, project_package=%#v", got, result.Observation.ProjectPackage)
	}
	tracks := result.Observation.ProjectPackage["tracks"].([]map[string]any)
	for _, track := range tracks {
		acoustic := track["acoustic"].(map[string]any)
		if acoustic["status"] != "ready" {
			t.Fatalf("track %v acoustic = %#v", track["track_id"], acoustic)
		}
	}
}

func TestCompactFeatureRequestInfersKernelPreparedRequestedFeatures(t *testing.T) {
	got := compactFeatureRequest(map[string]any{
		"schema_version":  "mixboard_feature_request.v1",
		"request_id":      "kernel_prepared_waveform_envelope_1014",
		"status":          "materialized",
		"resolved_target": map[string]any{"track_id": "1007", "clip_id": "1014"},
	})
	rows := mapRowsAny(got["requested_features"])
	if len(rows) != 2 {
		t.Fatalf("requested_features = %+v", got["requested_features"])
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[cleanAnyString(row["feature_type"])] = true
		if cleanAnyString(row["request_id"]) != "kernel_prepared_waveform_envelope_1014" ||
			cleanAnyString(row["track_id"]) != "1007" ||
			cleanAnyString(row["clip_id"]) != "1014" {
			t.Fatalf("bad inferred row: %+v", row)
		}
	}
	if !seen["waveform_envelope"] || !seen["spectral_field"] {
		t.Fatalf("missing inferred features: %+v", rows)
	}
}

func TestAcousticPackageBuildingProjectionDropsStaleEvidenceRef(t *testing.T) {
	snap := featureSnapshot{
		LatestRequest: map[string]any{"request_id": "req_current"},
	}
	status := map[string]any{
		"schema_version":   "acoustic_package_status.v0",
		"project_id":       "current",
		"track_id":         "track_current",
		"clip_id":          "clip_current",
		"source_revision":  "rev_current",
		"source_path":      `D:\Vit_DAW\current.wav`,
		"duration_seconds": 10,
		"package_layers": map[string]any{
			"l3_deep": map[string]any{"features": map[string]any{
				"spectrogram_tiles": map[string]any{
					"status": "building",
					"source": "mix.observe_background_fill",
					"reason": "background_fill_requested_after_read_first_status",
					"progress": map[string]any{
						"updated_at": "2026-06-22T00:00:00Z",
						"reason":     "background_fill_requested_after_read_first_status",
					},
					"ref": map[string]any{
						"request_id":      "req_old",
						"track_id":        "track_old",
						"clip_id":         "clip_old",
						"source_revision": "rev_old",
						"tile_count_seen": 44,
					},
				},
			}},
		},
	}
	applyAcousticPackageStatusToFeatureSnapshot(&snap, status)
	row := snap.SpectrogramTiles
	if row["status"] != "building" || row["request_id"] != "req_current" {
		t.Fatalf("projected row request/status = %+v", row)
	}
	if row["track_id"] != "track_current" || row["clip_id"] != "clip_current" || row["source_revision"] != "rev_current" {
		t.Fatalf("projected row identity = %+v", row)
	}
	if fmt.Sprint(row) == "" || strings.Contains(fmt.Sprint(row), "req_old") || strings.Contains(fmt.Sprint(row), "track_old") || strings.Contains(fmt.Sprint(row), "44") {
		t.Fatalf("projected row retained stale evidence: %+v", row)
	}
}

func TestAcousticPackageStatusDoesNotOverrideDifferentLatestTarget(t *testing.T) {
	snap := featureSnapshot{
		LatestRequest: map[string]any{
			"request_id":      "req_clip_2",
			"resolved_target": map[string]any{"track_id": "track_2", "clip_id": "clip_2", "source_path": `D:\Vit_DAW\two.wav`},
		},
		WaveformEnvelope:        map[string]any{"status": "ready", "track_id": "track_2", "clip_id": "clip_2", "request_id": "req_clip_2"},
		BandEnergySummary:       map[string]any{"status": "missing", "track_id": "track_2", "clip_id": "clip_2", "request_id": "req_clip_2"},
		StereoRelationSummary:   map[string]any{"status": "missing", "track_id": "track_2", "clip_id": "clip_2", "request_id": "req_clip_2"},
		TrackWaveformEnvelopes:  []map[string]any{},
		SpectrogramTileRows:     []map[string]any{},
		BandEnergySummaries:     []map[string]any{},
		StereoRelationSummaries: []map[string]any{},
	}
	status := map[string]any{
		"schema_version":   "acoustic_package_status.v0",
		"status":           "ready",
		"track_id":         "track_1",
		"clip_id":          "clip_1",
		"source_path":      `D:\Vit_DAW\one.wav`,
		"source_revision":  "rev_one",
		"duration_seconds": 10,
		"source_identity":  map[string]any{"track_id": "track_1", "clip_id": "clip_1", "source_path": `D:\Vit_DAW\one.wav`, "source_revision": "rev_one"},
		"package_layers": map[string]any{
			"l3_deep": map[string]any{"features": map[string]any{
				"band_energy_summary": map[string]any{
					"status": "ready",
					"ref":    map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "request_id": "req_clip_1", "bands": map[string]any{"bass": map[string]any{"energy_db": -12}}},
				},
				"stereo_relation_summary": map[string]any{
					"status": "ready",
					"ref":    map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "request_id": "req_clip_1", "correlation_state": "stable"},
				},
			}},
		},
	}
	applyAcousticPackageStatusToFeatureSnapshot(&snap, status)
	if snap.BandEnergySummary["status"] != "missing" || snap.BandEnergySummary["request_id"] != "req_clip_2" {
		t.Fatalf("mismatched acoustic package overrode band row: %+v", snap.BandEnergySummary)
	}
	if snap.StereoRelationSummary["status"] != "missing" || snap.StereoRelationSummary["request_id"] != "req_clip_2" {
		t.Fatalf("mismatched acoustic package overrode stereo row: %+v", snap.StereoRelationSummary)
	}
}

func TestStoreReadCatalogEntriesAndTimeRange(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-13T08:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.25,"peak_abs":0.8,"float_count":128,
			"time_segments":[
				{"start_seconds":0,"end_seconds":2,"rms":0.1,"rms_dbfs":-20,"peak_abs":0.4,"peak_dbfs":-7.959,"crest_db":12.041,"energy_state":"medium"},
				{"start_seconds":2,"end_seconds":4,"rms":0.01,"rms_dbfs":-40,"peak_abs":0.05,"peak_dbfs":-26.021,"crest_db":13.979,"energy_state":"low"}
			]},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_read",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"segment_seconds":       2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	read, err := store.Read(ReadRequest{
		ObservationID: result.Observation.ObservationID,
		Keys: []string{
			"observation.digest",
			"track.track_1.fast.levels",
			"track.track_1.raw.time_energy.range",
		},
		RangeStart: 2.01,
		RangeEnd:   4,
		MaxItems:   8,
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	if items["observation.digest"] == nil || items["track.track_1.fast.levels"] == nil {
		t.Fatalf("read missing digest/levels: %#v", items)
	}
	ranged := items["track.track_1.raw.time_energy.range"].(map[string]any)
	rows := ranged["rows"].([]map[string]any)
	if len(rows) != 1 || rows[0]["energy_state"] != "low" {
		t.Fatalf("unexpected ranged rows: %#v", ranged)
	}
}

func TestObservationPreservesMissingAcousticReasons(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-21T08:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"missing","reason":"no_spectral_tile_ready_received"},
		"band_energy_summary":{"status":"missing","reason":"spectral_field_missing: no_spectral_tile_ready_received"},
		"stereo_relation_summary":{"status":"missing","reason":"spectral_field_missing: no_spectral_tile_ready_received"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_missing_acoustic_reasons",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := result.Observation.MixPackage["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "missing" || band["reason"] != "spectral_field_missing: no_spectral_tile_ready_received" {
		t.Fatalf("band metrics = %#v", band)
	}
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["status"] != "missing" || stereo["reason"] != "spectral_field_missing: no_spectral_tile_ready_received" {
		t.Fatalf("stereo metrics = %#v", stereo)
	}
	caps, _ := result.Observation.DeepPackage["source_capabilities"].(map[string]string)
	if caps["masking_analysis"] != "deferred" || caps["reference_match"] != "deferred" || caps["lufs_analysis"] != "deferred" || caps["post_fx_probe"] != "unavailable" {
		t.Fatalf("deep caps = %#v", caps)
	}
	limits := result.Observation.ProjectPackage["limitations"].([]string)
	for _, want := range []string{"lufs_analysis_deferred_phase_5", "masking_analysis_deferred_phase_5", "reference_match_deferred_phase_5", "post_fx_probe_unavailable_phase_4_1"} {
		found := false
		for _, got := range limits {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("limitations missing %s: %#v", want, limits)
		}
	}
}

func TestObservationDowngradesStaleBridgeRowsForLatestRequest(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-21T08:00:00Z",
		"latest_request":{"request_id":"req_current","resolved_target":{"track_id":"track_1","clip_id":"clip_1"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source":"kernel_tile_ready_direct_collector","tile_count_seen":2,"tile_count_expected":2},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_old","source":"live_level_meter_spectrum","bands":{"low_mid":{"unit_energy":0.4,"energy_db":-7.959,"min_hz":160,"max_hz":500}}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"live_level_meter_stereo","left_level_db":-14,"right_level_db":-15,"balance_db":1,"balance_state":"centered","correlation_estimate":0.8,"correlation_state":"stable"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_stale_bridge",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "missing" {
		t.Fatalf("spectrogram capability = %q", got)
	}
	metrics, _ := result.Observation.MixPackage["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "missing" || band["reason"] != "stale_feature_snapshot_for_current_request" || band["request_id"] != "req_current" {
		t.Fatalf("stale band row was not downgraded: %#v", band)
	}
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["status"] != "missing" || stereo["reason"] != "stale_feature_snapshot_for_current_request" || stereo["request_id"] != "req_current" {
		t.Fatalf("stale stereo row was not downgraded: %#v", stereo)
	}
	featureSnapshot, _ := result.Observation.GlobalSummary["feature_snapshot"].(map[string]any)
	spectral, _ := featureSnapshot["spectrogram_tiles"].(map[string]any)
	if spectral["status"] != "missing" || spectral["reason"] != "stale_feature_snapshot_for_current_request" || spectral["request_id"] != "req_current" {
		t.Fatalf("stale spectral row was not downgraded: %#v", spectral)
	}
}

func TestObservationRelabelsMissingBridgeRowsButPreservesReadyL3Lineage(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-27T08:00:00Z",
		"latest_request":{"request_id":"kernel_prepared_spectral_field_clip_1","resolved_target":{"track_id":"track_1","clip_id":"clip_1"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_spectral_field_clip_1","source":"kernel_tile_ready_direct_collector","tile_count_seen":2,"tile_count_expected":2},
		"band_energy_summary":{"status":"missing","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","reason":"awaiting_current_target_band_summary"},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"older_l3_stereo_request","source":"kernel_l3_offline_analyzer","source_revision":"rev_current","correlation_estimate":0.8,"correlation_state":"stable"},
		"loudness_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"kernel_l3_offline_analyzer","source_revision":"rev_current","approximate_lufs":-18.4}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_missing_bridge_relabel",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := result.Observation.MixPackage["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "missing" || band["reason"] != "awaiting_current_target_band_summary" || band["request_id"] != "kernel_prepared_spectral_field_clip_1" {
		t.Fatalf("missing band row was not relabelled for current request: %#v", band)
	}
	featureSnapshot, _ := result.Observation.GlobalSummary["feature_snapshot"].(map[string]any)
	snapshotBand, _ := featureSnapshot["band_energy_summary"].(map[string]any)
	if snapshotBand["feature_type"] != "band_energy_summary" || snapshotBand["request_id"] != "kernel_prepared_spectral_field_clip_1" {
		t.Fatalf("snapshot missing band row was not relabelled: %#v", snapshotBand)
	}
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["status"] != "ready" || stereo["request_id"] != "older_l3_stereo_request" || stereo["source_revision"] != "rev_current" {
		t.Fatalf("ready stereo L3 row should keep material lineage instead of current request_id: %#v", stereo)
	}
	loudness, _ := metrics["loudness"].(map[string]any)
	if loudness["status"] != "ready" || loudness["request_id"] != "kernel_prepared_spectral_field_clip_1" || loudness["source_revision"] != "rev_current" {
		t.Fatalf("ready L3 row without request_id should be attributed to current request: %#v", loudness)
	}
}

func TestObservationPromotesReadyL3HistoryRowsForCurrentTarget(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-27T08:00:00Z",
		"latest_request":{"request_id":"kernel_prepared_spectral_field_clip_1","resolved_target":{"track_id":"track_1","clip_id":"clip_1","source_path":"D:/audio/current.wav","duration_seconds":3}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","source_path":"D:/audio/current.wav","duration_seconds":3,"rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_spectral_field_clip_1","source":"kernel_tile_ready_direct_collector","source_path":"D:/audio/current.wav","duration_seconds":3,"tile_count_seen":1,"tile_count_expected":1},
		"band_energy_summary":{"status":"missing","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","reason":"awaiting_current_target_band_summary"},
		"band_energy_summaries":[{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"kernel_l3_offline_analyzer","source_path":"D:/audio/current.wav","source_revision":"rev_current","duration_seconds":3,"bands":{"bass":{"energy_db":-12}}}],
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_stereo_relation_summary_clip_1","source":"kernel_l3_offline_analyzer","source_path":"D:/audio/current.wav","source_revision":"rev_current","duration_seconds":3,"correlation_state":"stable"},
		"loudness_summary":{"status":"missing","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","reason":"awaiting_current_target_loudness_summary"},
		"loudness_summaries":[{"status":"ready","track_id":"track_1","clip_id":"clip_1","source":"kernel_l3_offline_analyzer","source_path":"D:/audio/current.wav","source_revision":"rev_current","duration_seconds":3,"approximate_lufs":-18.4}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_l3_history_promote",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 3},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := result.Observation.MixPackage["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "ready" || band["request_id"] != "kernel_prepared_spectral_field_clip_1" || band["source_revision"] != "rev_current" {
		t.Fatalf("ready L3 band history row was not promoted for current target: %#v", band)
	}
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["status"] != "ready" || stereo["request_id"] != "kernel_prepared_spectral_field_clip_1" || stereo["source_revision"] != "rev_current" {
		t.Fatalf("ready sibling L3 stereo request was not normalized for current target: %#v", stereo)
	}
	loudness, _ := metrics["loudness"].(map[string]any)
	if loudness["status"] != "ready" || loudness["request_id"] != "kernel_prepared_spectral_field_clip_1" || loudness["source_revision"] != "rev_current" {
		t.Fatalf("ready L3 loudness history row was not promoted for current target: %#v", loudness)
	}
}

func TestFullProjectBridgeRowMatchesCurrentRequestAndCompactsScope(t *testing.T) {
	row := map[string]any{
		"status":              "partial",
		"feature_type":        "spectral_field",
		"request_id":          "req_project",
		"scope":               "full_project",
		"target_count":        2,
		"track_ids":           []any{"1007", "1012"},
		"tile_count_seen":     5,
		"tile_count_expected": 7,
	}
	target := map[string]any{"track_id": "1007", "clip_id": "1011"}
	if !bridgeRowMatchesRequest(row, "req_project", target) {
		t.Fatalf("full-project bridge row should match by request_id: %+v", row)
	}
	compacted := compactFeatureRow(row)
	for _, key := range []string{"feature_type", "scope", "target_count", "track_ids", "tile_count_seen", "tile_count_expected"} {
		if _, ok := compacted[key]; !ok {
			t.Fatalf("compact row dropped %s: %+v", key, compacted)
		}
	}
}

func TestObservationKeepsFreshBridgeRowsForLatestRequest(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-21T08:00:00Z",
		"latest_request":{"request_id":"req_fresh","resolved_target":{"track_id":"track_1","clip_id":"clip_1"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"req_fresh","source":"kernel_tile_ready_direct_collector","tile_count_seen":2,"tile_count_expected":2,"resolution_frame_width":256,"resolution_frequency_bins":128},
		"band_energy_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_fresh","source":"live_level_meter_spectrum","bands":{
			"sub":{"unit_energy":0.1,"energy_db":-20,"min_hz":20,"max_hz":60},
			"bass":{"unit_energy":0.2,"energy_db":-13.979,"min_hz":60,"max_hz":160},
			"low_mid":{"unit_energy":0.4,"energy_db":-7.959,"min_hz":160,"max_hz":500},
			"mid":{"unit_energy":0.3,"energy_db":-10.458,"min_hz":500,"max_hz":2000},
			"presence":{"unit_energy":0.25,"energy_db":-12.041,"min_hz":2000,"max_hz":6000},
			"air":{"unit_energy":0.15,"energy_db":-16.478,"min_hz":6000,"max_hz":16000}
		}},
		"stereo_relation_summary":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_fresh","source":"live_level_meter_stereo","left_level_db":-14,"right_level_db":-15,"balance_db":1,"balance_unit":-0.05,"balance_state":"centered","phase_deviation":0.08,"phase_negative_ratio":0.02,"correlation_estimate":0.8,"correlation_state":"stable","bin_count":96}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_fresh_bridge",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "ready" {
		t.Fatalf("spectrogram capability = %q", got)
	}
	mixPkg := result.Observation.MixPackage
	sourceCaps := mixPkg["source_capabilities"].(map[string]string)
	if sourceCaps["band_energy"] != "ready" || sourceCaps["stereo_correlation"] != "ready" {
		t.Fatalf("mix package source caps = %#v", sourceCaps)
	}
	metrics, _ := mixPkg["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "ready" || band["request_id"] != "req_fresh" || band["source"] != "live_level_meter_spectrum" {
		t.Fatalf("fresh band row was not consumed: %#v", band)
	}
	stereo, _ := metrics["stereo_relation"].(map[string]any)
	if stereo["status"] != "ready" || stereo["request_id"] != "req_fresh" || stereo["correlation_state"] != "stable" {
		t.Fatalf("fresh stereo row was not consumed: %#v", stereo)
	}
	featureSnapshot, _ := result.Observation.GlobalSummary["feature_snapshot"].(map[string]any)
	spectral, _ := featureSnapshot["spectrogram_tiles"].(map[string]any)
	if spectral["status"] != "ready" || spectral["request_id"] != "req_fresh" || int(numberFromMap(spectral, "tile_count_seen")) != 2 {
		t.Fatalf("fresh spectral row was not retained: %#v", spectral)
	}
}

func TestObservationInfersLatestRequestFromMaterializedBridgeRows(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"latest_request":{},
		"waveform_envelope":{"status":"ready","feature_type":"waveform_envelope","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_waveform_envelope_clip_1","source":"kernel_audio_feature_data_ready","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_spectral_field_clip_1","source":"kernel_tile_ready_direct_collector","tile_count_seen":2,"tile_count_expected":2},
		"band_energy_summary":{"status":"ready","feature_type":"band_energy_summary","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_spectral_field_clip_1","source":"spectral_tile_derived","bands":{"bass":{"unit_energy":0.2,"energy_db":-13.979}}},
		"stereo_relation_summary":{"status":"ready","feature_type":"stereo_relation_summary","track_id":"track_1","clip_id":"clip_1","request_id":"kernel_prepared_spectral_field_clip_1","source":"spectral_tile_derived","balance_db":0,"correlation_estimate":1}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := NewStore(root).RequestObservation(Request{
		MixSessionID: "mix_inferred_latest_request",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 10},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	featureSnapshot := mapValue(result.Observation.GlobalSummary["feature_snapshot"])
	latest := mapValue(featureSnapshot["latest_request"])
	if got := cleanAnyString(latest["request_id"]); got != "kernel_prepared_spectral_field_clip_1" {
		t.Fatalf("latest_request was not inferred from bridge rows: %#v", latest)
	}
	requested := mapRowsAny(latest["requested_features"])
	seen := map[string]bool{}
	for _, row := range requested {
		seen[cleanAnyString(row["feature_type"])] = true
	}
	if !seen["waveform_envelope"] || !seen["spectral_field"] {
		t.Fatalf("inferred latest_request missing requested features: %#v", latest)
	}
}

func TestProjectPackageSummarizesTracksAndRankings(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_project",
		TargetRef:    TargetRef{Kind: "track", ID: "vocal_1", Label: "Lead Vocal"},
		ProjectState: map[string]any{
			"tracks": []any{
				map[string]any{
					"track_id":         "vocal_1",
					"track_name":       "Lead Vocal",
					"user_track_index": 1,
					"level_db":         -10.5,
					"peak_dbfs":        -2.0,
					"headroom_db":      2.0,
					"plugins":          []any{map[string]any{"plugin_id": "eq_1", "plugin_name": "Vocal EQ"}},
					"clips":            []any{map[string]any{"clip_id": "clip_v", "length_seconds": 8.0}},
				},
				map[string]any{
					"track_id":         "bass_1",
					"track_name":       "Bass",
					"user_track_index": 2,
					"level_db":         -8.0,
					"peak_dbfs":        -1.0,
					"headroom_db":      1.0,
					"clips":            []any{map[string]any{"clip_id": "clip_b", "length_seconds": 8.0}},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.ProjectPackage["track_count"] != 2 {
		t.Fatalf("project package = %#v", result.Observation.ProjectPackage)
	}
	read, err := store.Read(ReadRequest{
		ObservationID: result.Observation.ObservationID,
		Keys:          []string{"project.tracks.summary", "project.rankings.level", "project.risks.headroom"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	tracks := items["project.tracks.summary"].(map[string]any)
	rows := tracks["tracks"].([]map[string]any)
	if len(rows) != 2 || rows[0]["role_guess"] != "vocal" || rows[1]["role_guess"] != "bass" {
		t.Fatalf("track summaries = %#v", rows)
	}
	level := items["project.rankings.level"].(map[string]any)
	levelRows := level["rows"].([]map[string]any)
	if len(levelRows) != 2 || levelRows[0]["track_id"] != "bass_1" {
		t.Fatalf("level ranking = %#v", level)
	}
	derived, err := store.Derive(DeriveRequest{
		ObservationID: result.Observation.ObservationID,
		Type:          "rank_tracks",
	})
	if err != nil {
		t.Fatal(err)
	}
	if derived["status"] != "ready" {
		t.Fatalf("derived = %#v", derived)
	}
	focusRelation, err := store.Derive(DeriveRequest{
		ObservationID: result.Observation.ObservationID,
		Type:          "focus_vs_project",
		Focus:         map[string]any{"track_id": "vocal_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	relationship := focusRelation["relationship"].(map[string]any)
	if relationship["status"] != "ready" {
		t.Fatalf("focus relation = %#v", relationship)
	}
	facts := relationship["facts"].(map[string]any)
	focusTrack := facts["focus_track"].(map[string]any)
	if focusTrack["track_id"] != "vocal_1" || focusTrack["role_guess"] != "vocal" {
		t.Fatalf("focus track = %#v", focusTrack)
	}
	judgement := relationship["derived_judgement"].(map[string]any)
	if judgement["focus_level_rank"] == nil || judgement["first_attention_candidate"] == nil {
		t.Fatalf("derived judgement = %#v", judgement)
	}
	abRelation, err := store.Derive(DeriveRequest{
		ObservationID: result.Observation.ObservationID,
		Type:          "a_vs_b",
		A:             map[string]any{"track_id": "vocal_1"},
		B:             map[string]any{"track_id": "bass_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ab := abRelation["relationship"].(map[string]any)
	abFacts := ab["facts"].(map[string]any)
	peerTrack := abFacts["peer_track"].(map[string]any)
	if peerTrack["track_id"] != "bass_1" {
		t.Fatalf("peer track = %#v", peerTrack)
	}
}

func TestProjectPackageUsesPerTrackWaveformAcoustics(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-17T08:00:00Z",
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"vocal_1","clip_id":"clip_v","rms":0.2,"peak_abs":0.5,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.5,"energy_state":"high"}]},
			{"status":"ready","track_id":"bass_1","clip_id":"clip_b","rms":0.1,"peak_abs":0.9,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.1,"peak_abs":0.9,"energy_state":"medium"}]}
		],
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_project_acoustic",
		TargetRef:    TargetRef{Kind: "project", ID: "current", Label: "Current project"},
		ListenScope: ListenScope{
			Source: ListenSourceScope{Mode: "full_project"},
		},
		ProjectState: map[string]any{
			"tracks": []any{
				map[string]any{
					"track_id":   "vocal_1",
					"track_name": "Lead Vocal",
					"clips":      []any{map[string]any{"clip_id": "clip_v", "clip_name": "lead_vocal_take", "file_path": "D:/audio/lead_vocal.wav", "length_seconds": 8.0}},
				},
				map[string]any{
					"track_id":   "bass_1",
					"track_name": "Bass",
					"clips":      []any{map[string]any{"clip_id": "clip_b", "clip_name": "bass_take", "file_path": "D:/audio/bass.wav", "length_seconds": 8.0}},
				},
			},
		},
		Args: map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := result.Observation.ProjectPackage
	if project["active_acoustic_track_count"] != 2 {
		t.Fatalf("project package = %#v", project)
	}
	tracks := mapRowsAny(project["tracks"])
	if len(tracks) != 2 {
		t.Fatalf("tracks = %#v", tracks)
	}
	for _, track := range tracks {
		acoustic, _ := track["acoustic"].(map[string]any)
		if featureStatus(acoustic) != "ready" || track["rms_dbfs"] == nil || track["peak_dbfs"] == nil || track["headroom_db"] == nil || track["crest_db"] == nil {
			t.Fatalf("track acoustic missing metrics: %#v", track)
		}
		if acoustic["primary_clip_id"] == nil || acoustic["primary_clip_name"] == nil || acoustic["source_path"] == nil {
			t.Fatalf("track acoustic missing primary clip source: %#v", acoustic)
		}
	}
	loudness := mapRowsAny(project["loudness_ranking"])
	if len(loudness) != 2 || loudness[0]["track_id"] != "vocal_1" {
		t.Fatalf("loudness ranking = %#v", loudness)
	}
	headroom := mapRowsAny(project["headroom_risk"])
	if len(headroom) != 2 || headroom[0]["track_id"] != "bass_1" || headroom[0]["risk"] != "high" {
		t.Fatalf("headroom ranking = %#v", headroom)
	}
	peakRisk := mapRowsAny(project["peak_risk_ranking"])
	if len(peakRisk) != 2 || peakRisk[0]["track_id"] != "bass_1" || peakRisk[0]["risk"] != "high" {
		t.Fatalf("peak risk ranking = %#v", peakRisk)
	}
	digest := result.Observation.Digest
	if digest["active_acoustic_track_count"] != 2 || digest["project_loudness_ranking_excerpt"] == nil || digest["project_peak_risk_ranking_excerpt"] == nil || digest["likely_first_attention_target"] == nil {
		t.Fatalf("digest = %#v", digest)
	}
	read, err := store.Read(ReadRequest{
		ObservationID: result.Observation.ObservationID,
		Keys:          []string{"project.acoustic.tracks", "project.rankings.loudness", "project.risks.peak", "project.attention.first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	acousticRows := items["project.acoustic.tracks"].(map[string]any)
	if acousticRows["status"] != "ready" {
		t.Fatalf("acoustic read = %#v", acousticRows)
	}
	readTracks := mapRowsAny(acousticRows["tracks"])
	if len(readTracks) != 2 {
		t.Fatalf("acoustic read tracks = %#v", acousticRows)
	}
	readAcoustic, _ := readTracks[0]["acoustic"].(map[string]any)
	if readAcoustic["primary_clip_id"] == nil || readAcoustic["source_path"] == nil {
		t.Fatalf("compact acoustic read missing source fields: %#v", readAcoustic)
	}
	peakRiskRead := items["project.risks.peak"].(map[string]any)
	if peakRiskRead["status"] != "ready" || len(mapRowsAny(peakRiskRead["rows"])) != 2 {
		t.Fatalf("peak risk read = %#v", peakRiskRead)
	}
	derived, err := store.Derive(DeriveRequest{
		ObservationID: result.Observation.ObservationID,
		Type:          "rank_tracks",
	})
	if err != nil {
		t.Fatal(err)
	}
	relationship := derived["relationship"].(map[string]any)
	rankings := relationship["rankings"].(map[string]any)
	if len(mapRowsAny(rankings["loudness"])) != 2 || len(mapRowsAny(rankings["peak_risk"])) != 2 {
		t.Fatalf("derived rankings = %#v", rankings)
	}
}

func TestFullProjectIgnoresNonAuthoritativeBlockedWaveformRequest(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-22T08:00:00Z",
		"latest_request":{
			"request_id":"old_blocked_selection",
			"status":"blocked",
			"reason":"clip_source_required_for_current_feature_bakers",
			"resolved_target":{"kind":"selection","id":"Track 1","track_id":"Track 1"}
		},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"older_req_1","rms":0.2,"peak_abs":0.5,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.5,"energy_state":"high"}]},
			{"status":"ready","track_id":"track_2","clip_id":"clip_2","request_id":"older_req_2","rms":0.1,"peak_abs":0.7,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.1,"peak_abs":0.7,"energy_state":"medium"}]}
		],
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_project_non_authoritative_blocked",
		TargetRef:    TargetRef{Kind: "project", ID: "current", Label: "Current project"},
		ListenScope: ListenScope{
			Source: ListenSourceScope{Mode: "full_project"},
		},
		ProjectState: map[string]any{
			"tracks": []any{
				map[string]any{
					"track_id":   "track_1",
					"track_name": "Track 1",
					"clips":      []any{map[string]any{"clip_id": "clip_1", "clip_name": "one", "file_path": "D:/audio/one.wav", "length_seconds": 8.0}},
				},
				map[string]any{
					"track_id":   "track_2",
					"track_name": "Track 2",
					"clips":      []any{map[string]any{"clip_id": "clip_2", "clip_name": "two", "file_path": "D:/audio/two.wav", "length_seconds": 8.0}},
				},
			},
		},
		Args: map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := result.Observation.ProjectPackage
	if project["active_acoustic_track_count"] != 2 {
		t.Fatalf("project package = %#v", project)
	}
	if got := result.Observation.SourceCapabilities["track_waveform_envelopes"]; got != "ready" {
		t.Fatalf("track waveform capability = %q", got)
	}
}

func TestSnapshotFiltersStaleWaveformRowsForCurrentRequest(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-22T08:00:00Z",
		"latest_request":{"request_id":"req_current","resolved_target":{"kind":"track","track_id":"track_1","clip_id":"clip_current"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_old","request_id":"req_old","rms":0.6,"peak_abs":0.99,"rms_dbfs":-4.4,"peak_dbfs":-0.1},
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"track_1","clip_id":"clip_old","request_id":"req_old","rms":0.6,"peak_abs":0.99,"rms_dbfs":-4.4,"peak_dbfs":-0.1,
				"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.6,"peak_abs":0.99,"energy_state":"high"}]},
			{"status":"ready","track_id":"track_2","clip_id":"clip_other","request_id":"req_old","rms":0.5,"peak_abs":0.95,"rms_dbfs":-6,"peak_dbfs":-0.4}
		],
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_stale_waveform_rows",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{
			"duration_seconds": 12,
			"tracks": []any{map[string]any{
				"track_id":   "track_1",
				"track_name": "Current Track",
				"clips":      []any{map[string]any{"clip_id": "clip_current", "length_seconds": 12}},
			}},
		},
		Args: map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["track_waveform_envelopes"]; got != "missing" {
		t.Fatalf("track waveform capability = %q", got)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "missing" {
		t.Fatalf("waveform capability = %q", got)
	}
	project := result.Observation.ProjectPackage
	if project["active_acoustic_track_count"] != 0 {
		t.Fatalf("stale acoustic row was consumed: %#v", project)
	}
	tracks := mapRowsAny(project["tracks"])
	if len(tracks) != 1 {
		t.Fatalf("tracks = %#v", tracks)
	}
	acoustic, _ := tracks[0]["acoustic"].(map[string]any)
	if featureStatus(acoustic) == "ready" || tracks[0]["rms_dbfs"] != nil || tracks[0]["peak_dbfs"] != nil {
		t.Fatalf("stale waveform metrics leaked into current track: %#v", tracks[0])
	}
	featureSnapshot, _ := result.Observation.GlobalSummary["feature_snapshot"].(map[string]any)
	if rows := mapRowsAny(featureSnapshot["track_waveform_envelopes"]); len(rows) != 0 {
		t.Fatalf("stale track waveform rows were retained for current observation: %#v", rows)
	}
	waveform, _ := featureSnapshot["waveform_envelope"].(map[string]any)
	if waveform["status"] != "missing" || waveform["reason"] != "stale_feature_snapshot_for_current_request" || waveform["request_id"] != "req_current" {
		t.Fatalf("stale waveform envelope was not downgraded: %#v", waveform)
	}
}

func TestSpectralPartialDoesNotClaimBandEnergyReady(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-22T08:00:00Z",
		"latest_request":{"request_id":"req_current","resolved_target":{"kind":"track","track_id":"track_1","clip_id":"clip_1"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","request_id":"req_current","rms":0.2,"peak_abs":0.7},
		"spectrogram_tiles":{"status":"partial","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1","request_id":"req_current","tile_count_seen":2,"tile_count_expected":8},
		"band_energy_summary":{"status":"missing","track_id":"track_1","clip_id":"clip_1","request_id":"req_current","reason":"no_live_spectrum_payload_for_current_request"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_partial_spectral",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "partial" {
		t.Fatalf("spectrogram capability = %q", got)
	}
	mixPkg := result.Observation.MixPackage
	sourceCaps := mixPkg["source_capabilities"].(map[string]string)
	if sourceCaps["band_energy"] != "missing" {
		t.Fatalf("band energy source caps = %#v", sourceCaps)
	}
	metrics, _ := mixPkg["current_metrics"].(map[string]any)
	band, _ := metrics["band_energy"].(map[string]any)
	if band["status"] != "missing" {
		t.Fatalf("band energy metrics = %#v", band)
	}
	if result.Board.PackageStatus["deep"] != "async_partial" {
		t.Fatalf("package status = %#v", result.Board.PackageStatus)
	}
	seenPartial := false
	for _, hotspot := range result.Observation.Hotspots {
		if hotspot["tag"] == "deep_band_observation_available" {
			t.Fatalf("partial spectral row claimed deep band availability: %#v", result.Observation.Hotspots)
		}
		if hotspot["tag"] == "spectral_tiles_partial_band_summary_missing" {
			seenPartial = true
		}
	}
	if !seenPartial {
		t.Fatalf("partial spectral hotspot missing: %#v", result.Observation.Hotspots)
	}
}

func TestSpectralReadyDoesNotClaimBandEnergyReadyWhenBandIsStale(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-23T13:52:47Z",
		"latest_request":{"request_id":"kernel_prepared_waveform_envelope_1013","resolved_target":{"kind":"clip","track_id":"track_1","clip_id":"clip_1013","source_path":"D:/Vit_DAW/Paper Crown.mp3"}},
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1013","request_id":"kernel_prepared_waveform_envelope_1013","rms":0.2,"peak_abs":0.7,"source_path":"D:/Vit_DAW/Paper Crown.mp3"},
		"spectrogram_tiles":{"status":"ready","feature_type":"spectral_field","track_id":"track_1","clip_id":"clip_1013","request_id":"kernel_prepared_waveform_envelope_1013","tile_count_seen":44,"tile_count_expected":44,"total_duration":219.384,"source_path":"D:/Vit_DAW/Paper Crown.mp3"},
		"band_energy_summary":{"status":"stale","track_id":"track_1","clip_id":"clip_1013","request_id":"kernel_prepared_waveform_envelope_1013","reason":"clip_mismatch"},
		"stereo_relation_summary":{"status":"stale","track_id":"track_1","clip_id":"clip_1013","request_id":"kernel_prepared_waveform_envelope_1013","reason":"clip_mismatch"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_ready_spectral_stale_band",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 219.384},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "ready" {
		t.Fatalf("spectrogram capability = %q", got)
	}
	if got := result.Observation.SourceCapabilities["band_energy"]; got != "stale" {
		t.Fatalf("band capability = %q", got)
	}
	deepCaps := result.Observation.DeepPackage["source_capabilities"].(map[string]string)
	if got := deepCaps["deep_band_observation"]; got != "stale" {
		t.Fatalf("deep band capability = %q caps=%#v", got, deepCaps)
	}
	if got := deepCaps["spectrogram_tiles"]; got != "ready" {
		t.Fatalf("deep spectrogram capability = %q caps=%#v", got, deepCaps)
	}
	seenSpectralOnly := false
	for _, hotspot := range result.Observation.Hotspots {
		if hotspot["tag"] == "deep_band_observation_available" {
			t.Fatalf("ready spectral row claimed deep band availability: %#v", result.Observation.Hotspots)
		}
		if hotspot["tag"] == "spectral_tiles_ready_band_summary_missing" {
			seenSpectralOnly = true
		}
	}
	if !seenSpectralOnly {
		t.Fatalf("spectral-only hotspot missing: %#v", result.Observation.Hotspots)
	}
}

func TestObservationReadyWithLightweightEnvelopeOnly(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-13T08:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.25,"peak_abs":0.8,"float_count":128,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.1,"peak_abs":0.5,"energy_state":"medium"}]},
		"spectrogram_tiles":{"status":"missing"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_envelope_only",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"segment_seconds":       2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" {
		t.Fatalf("status = %q", result.Status)
	}
	if got := result.Observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("waveform status = %q", got)
	}
	if got := result.Observation.SourceCapabilities["spectrogram_tiles"]; got != "missing" {
		t.Fatalf("spectrogram status = %q", got)
	}
	if len(result.Board.OpenBlockers) != 0 {
		t.Fatalf("blockers = %#v", result.Board.OpenBlockers)
	}
	if result.Board.PackageStatus["environment"] != "ready" || result.Board.PackageStatus["mix"] != "baseline_ready" || result.Board.PackageStatus["deep"] != "async_missing" {
		t.Fatalf("package status = %#v", result.Board.PackageStatus)
	}
	mixPkg := result.Observation.MixPackage
	if mixPkg["status"] != "baseline_ready" {
		t.Fatalf("mix package = %#v", mixPkg)
	}
	metrics, _ := mixPkg["current_metrics"].(map[string]any)
	waveform, _ := metrics["waveform"].(map[string]any)
	if waveform["crest_db"] == nil || waveform["headroom_db"] == nil {
		t.Fatalf("waveform metrics missing derived values: %#v", waveform)
	}
	if sourceCaps, _ := mixPkg["source_capabilities"].(map[string]string); sourceCaps["time_energy"] != "ready" {
		t.Fatalf("mix package source caps = %#v", mixPkg["source_capabilities"])
	}
}

func TestBoardDoesNotReportReaderBlockerWhenBandStereoEvidenceReady(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 15, 0, 0, time.UTC).Format(time.RFC3339Nano)
	obs := ObservationPacket{
		Status:             "ready",
		ObservationID:      "obs_ready_band_stereo",
		MixSessionID:       "mix_ready_band_stereo",
		SourceCapabilities: map[string]string{"waveform_envelope": "missing"},
		GlobalSummary: map[string]any{
			"band_energy_summary": map[string]any{
				"status": "ready",
				"bands": map[string]any{
					"bass": map[string]any{"energy_db": -10.0},
				},
			},
			"stereo_relation_summary": map[string]any{
				"status":               "ready",
				"balance_state":        "centered",
				"correlation_estimate": 0.7,
			},
		},
	}

	board := buildBoard(Request{MixSessionID: "mix_ready_band_stereo"}, obs, filepath.Join(t.TempDir(), "obs.json"), now)

	if len(board.OpenBlockers) != 0 {
		t.Fatalf("blockers = %#v", board.OpenBlockers)
	}
}

func TestBoardReportsReaderBlockerWhenNoAcousticEvidenceReady(t *testing.T) {
	now := time.Date(2026, 6, 24, 8, 16, 0, 0, time.UTC).Format(time.RFC3339Nano)
	obs := ObservationPacket{
		Status:             "ready",
		ObservationID:      "obs_no_evidence",
		MixSessionID:       "mix_no_evidence",
		SourceCapabilities: map[string]string{"waveform_envelope": "missing"},
		GlobalSummary: map[string]any{
			"band_energy_summary": map[string]any{"status": "stale"},
		},
	}

	board := buildBoard(Request{MixSessionID: "mix_no_evidence"}, obs, filepath.Join(t.TempDir(), "obs.json"), now)

	if len(board.OpenBlockers) != 1 || board.OpenBlockers[0] != "audio_feature_reader_not_connected" {
		t.Fatalf("blockers = %#v", board.OpenBlockers)
	}
}

func TestRequestObservationComputesBeforeAfterDeltaAcrossRounds(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-13T08:00:00Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.1,"peak_abs":0.5,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.1,"peak_abs":0.5,"energy_state":"medium"}]},
		"band_energy_summary":{"status":"ready","source":"live_level_meter_spectrum","bands":{
			"sub":{"unit_energy":0.1,"energy_db":-20,"min_hz":20,"max_hz":60},
			"bass":{"unit_energy":0.2,"energy_db":-13.979,"min_hz":60,"max_hz":160},
			"low_mid":{"unit_energy":0.3,"energy_db":-10.458,"min_hz":160,"max_hz":500},
			"mid":{"unit_energy":0.4,"energy_db":-7.959,"min_hz":500,"max_hz":2000},
			"presence":{"unit_energy":0.2,"energy_db":-13.979,"min_hz":2000,"max_hz":6000},
			"air":{"unit_energy":0.1,"energy_db":-20,"min_hz":6000,"max_hz":16000}
		}},
		"stereo_relation_summary":{"status":"ready","source":"live_level_meter_stereo","left_level_db":-14,"right_level_db":-14,"balance_db":0,"balance_unit":0,"balance_state":"centered","phase_deviation":0.1,"phase_negative_ratio":0.02,"correlation_estimate":0.8,"correlation_state":"stable","bin_count":96}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	first, err := store.RequestObservation(Request{
		MixSessionID: "mix_delta",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"segment_seconds":       2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstMetrics := first.Observation.MixPackage["current_metrics"].(map[string]any)
	firstDelta := firstMetrics["before_after_delta"].(map[string]any)
	if firstDelta["status"] != "pending" {
		t.Fatalf("first delta = %#v", firstDelta)
	}
	if err := os.WriteFile(snapshotPath, []byte(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-13T08:00:02Z",
		"waveform_envelope":{"status":"ready","track_id":"track_1","clip_id":"clip_1","rms":0.2,"peak_abs":0.7,
			"time_segments":[{"start_seconds":0,"end_seconds":2,"rms":0.2,"peak_abs":0.7,"energy_state":"high"}]},
		"band_energy_summary":{"status":"ready","source":"live_level_meter_spectrum","bands":{
			"sub":{"unit_energy":0.1,"energy_db":-20,"min_hz":20,"max_hz":60},
			"bass":{"unit_energy":0.3,"energy_db":-10.458,"min_hz":60,"max_hz":160},
			"low_mid":{"unit_energy":0.3,"energy_db":-10.458,"min_hz":160,"max_hz":500},
			"mid":{"unit_energy":0.5,"energy_db":-6.021,"min_hz":500,"max_hz":2000},
			"presence":{"unit_energy":0.2,"energy_db":-13.979,"min_hz":2000,"max_hz":6000},
			"air":{"unit_energy":0.1,"energy_db":-20,"min_hz":6000,"max_hz":16000}
		}},
		"stereo_relation_summary":{"status":"ready","source":"live_level_meter_stereo","left_level_db":-13,"right_level_db":-15,"balance_db":2,"balance_unit":-0.1,"balance_state":"left_heavy","phase_deviation":0.18,"phase_negative_ratio":0.04,"correlation_estimate":0.64,"correlation_state":"stable","bin_count":96}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := store.RequestObservation(Request{
		MixSessionID: "mix_delta",
		Round:        2,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"segment_seconds":       2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mixPkg := second.Observation.MixPackage
	metrics := mixPkg["current_metrics"].(map[string]any)
	delta := metrics["before_after_delta"].(map[string]any)
	if delta["status"] != "ready" {
		t.Fatalf("delta = %#v", delta)
	}
	waveform := delta["waveform"].(map[string]any)
	peak := waveform["peak_dbfs"].(map[string]any)
	if peak["delta"] == nil || peak["delta"].(float64) <= 0 {
		t.Fatalf("peak delta = %#v", peak)
	}
	if caps := mixPkg["source_capabilities"].(map[string]string); caps["before_after_delta"] != "ready" {
		t.Fatalf("caps = %#v", caps)
	}
	derived, err := store.Derive(DeriveRequest{
		ObservationID: second.Observation.ObservationID,
		Type:          "before_after",
	})
	if err != nil {
		t.Fatal(err)
	}
	relationship := derived["relationship"].(map[string]any)
	if relationship["status"] != "ready" {
		t.Fatalf("derived relationship = %#v", relationship)
	}
	for _, raw := range mixPkg["missing_metrics"].([]string) {
		if raw == "before_after_delta" {
			t.Fatalf("before_after_delta should not be missing: %#v", mixPkg)
		}
	}
}

func TestRequestObservationComputesABResultFromL2RenderProbeAcrossRounds(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_before", -12.0, -2.0, 0.1, 0.86, -18.0)), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	first, err := store.RequestObservation(Request{
		MixSessionID: "mix_ab",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstAB := mapValue(mapValue(first.Observation.MixPackage["current_metrics"])["ab_result"])
	if firstAB["status"] != "missing" || firstAB["reason"] != "no_previous_mix_observation" {
		t.Fatalf("first ab_result = %#v", firstAB)
	}
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_after", -10.5, -1.4, 0.7, 0.74, -15.5)), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := store.RequestObservation(Request{
		MixSessionID: "mix_ab",
		Round:        2,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	metrics := mapValue(second.Observation.MixPackage["current_metrics"])
	ab := mapValue(metrics["ab_result"])
	if ab["status"] != "ready" {
		t.Fatalf("ab_result = %#v", ab)
	}
	if ab["tap_point"] != "track_post_fader" || ab["before_render_revision"] != "render_before" || ab["after_render_revision"] != "render_after" {
		t.Fatalf("ab identity = %#v", ab)
	}
	delta := mapValue(ab["delta"])
	levels := mapValue(delta["levels"])
	if rms := mapValue(levels["rms_dbfs"]); rms["delta"] != 1.5 {
		t.Fatalf("rms delta = %#v", rms)
	}
	if caps := second.Observation.MixPackage["source_capabilities"].(map[string]string); caps["ab_result"] != "ready" {
		t.Fatalf("caps = %#v", caps)
	}
	derived, err := store.Derive(DeriveRequest{ObservationID: second.Observation.ObservationID, Type: "ab_result"})
	if err != nil {
		t.Fatal(err)
	}
	relationship := mapValue(derived["relationship"])
	if relationship["status"] != "ready" {
		t.Fatalf("derived ab_result = %#v", relationship)
	}
	data, _ := json.Marshal(second.Observation.MOMProjection.LLMContext)
	for _, forbidden := range []string{"raw_samples", "spectral_tiles", "render_file_path", "probe.wav", "quality_evidence"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("AB LLM context leaked raw render probe field %s in %s", forbidden, string(data))
		}
	}
}

func TestRequestObservationComputesABResultFromExplicitPreviousObservationAcrossSessions(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_before", -12.0, -2.0, 0.1, 0.86, -18.0)), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	first, err := store.RequestObservation(Request{
		MixSessionID: "mix_plugin_prep_before",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_after", -11.0, -1.8, 0.2, 0.82, -20.0)), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := store.RequestObservation(Request{
		MixSessionID: "mix_plugin_prep_after",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args: map[string]any{
			"feature_snapshot_path":   snapshotPath,
			"previous_observation":    "observation:" + first.Observation.ObservationID,
			"previous_observation_id": first.Observation.ObservationID,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ab := mapValue(mapValue(second.Observation.MixPackage["current_metrics"])["ab_result"])
	if ab["status"] != "ready" {
		t.Fatalf("ab_result = %#v", ab)
	}
	if ab["before_observation_id"] != first.Observation.ObservationID || ab["after_observation_id"] != second.Observation.ObservationID {
		t.Fatalf("ab observation ids = %#v", ab)
	}
	if ab["before_render_revision"] != "render_before" || ab["after_render_revision"] != "render_after" {
		t.Fatalf("ab render revisions = %#v", ab)
	}
}

func TestRequestObservationMarksABResultStaleWhenRenderRevisionIsReused(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_same", -12.0, -2.0, 0.0, 0.8, -18.0)), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	if _, err := store.RequestObservation(Request{
		MixSessionID: "mix_ab_stale",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte(testL2RenderProbeSnapshot("render_same", -9.0, -1.0, 1.0, 0.7, -14.0)), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := store.RequestObservation(Request{
		MixSessionID: "mix_ab_stale",
		Round:        2,
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"duration_seconds": 12},
		Args:         map[string]any{"feature_snapshot_path": snapshotPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	ab := mapValue(mapValue(second.Observation.MixPackage["current_metrics"])["ab_result"])
	if ab["status"] != "stale" || ab["reason"] != "render_revision_not_changed" {
		t.Fatalf("ab_result should be stale for reused render_revision: %#v", ab)
	}
	if caps := second.Observation.MixPackage["source_capabilities"].(map[string]string); caps["ab_result"] == "ready" {
		t.Fatalf("stale ab_result must not be ready: %#v", caps)
	}
}

func testL2RenderProbeSnapshot(renderRevision string, rmsDB, peakDB, balanceDB, correlation, bassDB float64) string {
	return fmt.Sprintf(`{
		"schema_version":"mixboard_feature_snapshot.v1",
		"updated_at":"2026-06-26T08:00:00Z",
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"},
		"band_energy_summary":{"status":"missing"},
		"stereo_relation_summary":{"status":"missing"},
		"l2_render_probe":{
			"schema_version":"dad_l2_render_probe.v1",
			"status":"ready",
			"feature_type":"l2_render_probe",
			"source":"l2_render_probe",
			"track_id":"track_1",
			"clip_id":"clip_1",
			"tap_point":"track_post_fader",
			"render_mode":"offline_probe",
			"source_revision":"source_rev_1",
			"clip_revision":"clip_rev_1",
			"render_revision":%q,
			"duration_seconds":12,
			"sample_rate":48000,
			"channel_count":2,
			"rms_dbfs":%0.3f,
			"peak_dbfs":%0.3f,
			"headroom_db":%0.3f,
			"crest_db":9.0,
			"balance_db":%0.3f,
			"correlation_estimate":%0.3f,
			"balance_state":"centered",
			"correlation_state":"stable",
			"bands":{"bass":{"energy_db":%0.3f,"unit_energy":0.24,"min_hz":60,"max_hz":160}},
			"quality_evidence":{"nonzero":true,"sum_abs":10.0,"max_abs":0.5,"nan_inf_count":0,"coverage":1.0,"latency_compensated":true,"tail_captured":true,"deterministic":true},
			"evidence_ref":"dad.l2_render_probe:%s",
			"raw_samples":[1,2,3],
			"spectral_tiles":[{"bin":1}],
			"render_file_path":"D:/tmp/probe.wav"
		}
	}`, renderRevision, rmsDB, peakDB, -peakDB, balanceDB, correlation, bassDB, renderRevision)
}
