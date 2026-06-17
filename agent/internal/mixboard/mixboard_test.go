package mixboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
	if len(result.ContextPack.LatestObservation["timeline_digest"].([]map[string]any)) == 0 {
		t.Fatalf("context pack missing timeline digest: %#v", result.ContextPack)
	}
	if result.ContextPack.LatestObservation["digest"] == nil || result.ContextPack.LatestObservation["catalog"] == nil {
		t.Fatalf("context pack missing observation digest/catalog: %#v", result.ContextPack.LatestObservation)
	}
	if len(result.Observation.Digest) == 0 || len(result.Observation.Catalog.Entries) == 0 {
		t.Fatalf("observation missing digest/catalog: %#v", result.Observation)
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
	if result.ContextPack.LatestObservation["digest"] == nil || result.ContextPack.LatestObservation["catalog"] == nil {
		t.Fatalf("context pack missing digest/catalog: %#v", result.ContextPack.LatestObservation)
	}
	if result.ContextPack.LatestObservation["mix_package"] != nil || result.ContextPack.LatestObservation["environment_package"] != nil || result.ContextPack.LatestObservation["deep_package"] != nil {
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
					"clips":      []any{map[string]any{"clip_id": "clip_v", "length_seconds": 8.0}},
				},
				map[string]any{
					"track_id":   "bass_1",
					"track_name": "Bass",
					"clips":      []any{map[string]any{"clip_id": "clip_b", "length_seconds": 8.0}},
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
	}
	loudness := mapRowsAny(project["loudness_ranking"])
	if len(loudness) != 2 || loudness[0]["track_id"] != "vocal_1" {
		t.Fatalf("loudness ranking = %#v", loudness)
	}
	headroom := mapRowsAny(project["headroom_risk"])
	if len(headroom) != 2 || headroom[0]["track_id"] != "bass_1" || headroom[0]["risk"] != "high" {
		t.Fatalf("headroom ranking = %#v", headroom)
	}
	digest := result.Observation.Digest
	if digest["active_acoustic_track_count"] != 2 || digest["project_loudness_ranking_excerpt"] == nil || digest["likely_first_attention_target"] == nil {
		t.Fatalf("digest = %#v", digest)
	}
	read, err := store.Read(ReadRequest{
		ObservationID: result.Observation.ObservationID,
		Keys:          []string{"project.acoustic.tracks", "project.rankings.loudness", "project.attention.first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := read["items"].(map[string]any)
	acousticRows := items["project.acoustic.tracks"].(map[string]any)
	if acousticRows["status"] != "ready" {
		t.Fatalf("acoustic read = %#v", acousticRows)
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
	if len(mapRowsAny(rankings["loudness"])) != 2 {
		t.Fatalf("derived rankings = %#v", rankings)
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
