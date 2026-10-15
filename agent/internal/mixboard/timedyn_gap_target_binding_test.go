package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// M2-EXP-TIMEDYN-GAP-1 修复腿：CCB 轨道视图组装必须按观察请求目标轨读
// TrackWaveformEnvelopes/StereoRelationSummaries 行。导入路径上快照单数槽/
// latest_request 绑定「最后完成填充的轨」，而请求目标轨的行 6/6 在场——
// 组装不得让错轨素材进入目标轨视图键。

// timedynGapSnapshot 复刻 A 轮 run 终态快照：6 轨行在场，latest_request 与
// 单数 WE 槽都绑定 1032（vocals，导入填充尾），1017（guitar）行带段在场。
func timedynGapSnapshot(includeTargetRows bool) featureSnapshot {
	targetWE := map[string]any{
		"status":             "ready",
		"feature_type":       "waveform_envelope",
		"track_id":           "1017",
		"clip_id":            "1021",
		"request_id":         "m2tg_import_fill_1017",
		"source":             "kernel_prepared_telemetry",
		"source_revision":    "rev_guitar",
		"source_fingerprint": "fp_guitar",
		"file_path":          "D:/audio/guitar.wav",
		"duration_seconds":   20.0,
		"time_segments": []map[string]any{
			{"start_seconds": 0.0, "end_seconds": 5.0, "rms_dbfs": -30.1, "peak_dbfs": -12.1, "crest_db": 18.0, "energy_state": "medium"},
			{"start_seconds": 5.0, "end_seconds": 10.0, "rms_dbfs": -27.2, "peak_dbfs": -10.2, "crest_db": 17.0, "energy_state": "medium"},
		},
	}
	otherWE := map[string]any{
		"status":             "ready",
		"feature_type":       "waveform_envelope",
		"track_id":           "1032",
		"clip_id":            "1036",
		"request_id":         "m2tg_import_fill_1032",
		"source":             "kernel_prepared_telemetry",
		"source_revision":    "rev_vocals",
		"source_fingerprint": "fp_vocals",
		"file_path":          "D:/audio/vocals.wav",
		"duration_seconds":   20.0,
		"time_segments": []map[string]any{
			{"start_seconds": 0.0, "end_seconds": 5.0, "rms_dbfs": -20.3, "peak_dbfs": -6.3, "crest_db": 14.0, "energy_state": "high"},
			{"start_seconds": 5.0, "end_seconds": 10.0, "rms_dbfs": -21.4, "peak_dbfs": -7.4, "crest_db": 14.0, "energy_state": "high"},
			{"start_seconds": 10.0, "end_seconds": 15.0, "rms_dbfs": -22.5, "peak_dbfs": -8.5, "crest_db": 14.0, "energy_state": "high"},
		},
	}
	trackRows := []map[string]any{otherWE}
	if includeTargetRows {
		trackRows = []map[string]any{targetWE, otherWE}
	}
	stereoTarget := map[string]any{
		"status": "ready", "feature_type": "stereo_relation_summary",
		"track_id": "1017", "clip_id": "1021", "request_id": "m2tg_import_fill_1017",
		"source_revision": "rev_guitar", "file_path": "D:/audio/guitar.wav",
		"balance_db": -1.5, "balance_state": "slight_left", "correlation_estimate": 0.82,
	}
	stereoOther := map[string]any{
		"status": "ready", "feature_type": "stereo_relation_summary",
		"track_id": "1032", "clip_id": "1036", "request_id": "m2tg_import_fill_1032",
		"source_revision": "rev_vocals", "file_path": "D:/audio/vocals.wav",
		"balance_db": 5.945, "balance_state": "right_heavy", "correlation_estimate": 0.11,
	}
	stereoRows := []map[string]any{stereoOther}
	if includeTargetRows {
		stereoRows = []map[string]any{stereoTarget, stereoOther}
	}
	snapshot := featureSnapshot{
		SchemaVersion: "mixboard_feature_snapshot.v1",
		UpdatedAt:     "2026-10-15T13:33:00Z",
		// latest_request = 导入填充尾：waveform 基线，resolved_target=1032。
		LatestRequest: map[string]any{
			"schema_version": "mixboard_feature_request.v1",
			"request_id":     "m2tg_import_fill_1032",
			"status":         "materialized",
			"resolved_target": map[string]any{
				"track_id": "1032", "clip_id": "1036", "file_path": "D:/audio/vocals.wav",
			},
			"requested_features": []any{
				map[string]any{"feature_type": "waveform_envelope", "request_id": "m2tg_import_fill_1032", "track_id": "1032"},
				map[string]any{"feature_type": "l3_acoustic_summary", "request_id": "m2tg_import_fill_1032", "track_id": "1032"},
			},
			"track_feature_targets": []any{
				map[string]any{"track_id": "1032", "clip_id": "1036"},
			},
		},
		WaveformEnvelope:              otherWE,
		TrackWaveformEnvelopes:        trackRows,
		SpectrogramTiles:              map[string]any{"status": "missing"},
		BandEnergySummary:             map[string]any{"status": "missing"},
		StereoRelationSummary:         stereoOther,
		LoudnessSummary:               map[string]any{"status": "missing"},
		RealtimeBandEnergySummary:     map[string]any{"status": "missing"},
		RealtimeStereoRelationSummary: map[string]any{"status": "missing"},
		L2RenderProbe:                 map[string]any{"status": "missing"},
	}
	if includeTargetRows {
		snapshot.StereoRelationSummaries = stereoRows
	} else {
		snapshot.StereoRelationSummaries = stereoRows
	}
	return snapshot
}

func timedynGapProjectState() map[string]any {
	return map[string]any{
		"project_uuid": "vitproj_m2tg",
		"tracks": []any{
			map[string]any{
				"track_id": "1017", "track_name": "guitar",
				"clips": []any{map[string]any{
					"clip_id": "1021", "source_revision": "rev_guitar", "file_path": "D:/audio/guitar.wav",
				}},
			},
			map[string]any{
				"track_id": "1032", "track_name": "vocals",
				"clips": []any{map[string]any{
					"clip_id": "1036", "source_revision": "rev_vocals", "file_path": "D:/audio/vocals.wav",
				}},
			},
		},
	}
}

func writeTimedynGapSnapshot(t *testing.T, snapshot featureSnapshot) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mixboard_feature_snapshot.json")
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
	return path
}

// A 轮场景（修复后必须通过的身份断言）：请求 1017 → 视图返回 1017 自身的
// 时域段与 stereo 素材，而非 1032。
func TestRequestObservationBindsTrackViewEvidenceToRequestedTarget(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.Now = func() time.Time { return time.Date(2026, 10, 15, 13, 33, 0, 0, time.UTC) }
	snapshotPath := writeTimedynGapSnapshot(t, timedynGapSnapshot(true))

	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_m2tg_fix",
		Round:        1,
		GoalText:     "guitar time dynamics",
		TargetRef:    TargetRef{Kind: "track", ID: "1017", Label: "guitar"},
		ListenScope:  ListenScope{Source: ListenSourceScope{Mode: "selected_track"}},
		ProjectState: timedynGapProjectState(),
		Args: map[string]any{
			"track_id":              "1017",
			"clip_id":               "1021",
			"scope":                 "selected_track",
			"feature_snapshot_path": snapshotPath,
			"duration_seconds":      20.0,
		},
	})
	if err != nil {
		t.Fatalf("RequestObservation: %v", err)
	}
	observation := result.Observation

	// 时域段=1017 自身（2 段、-30.1dB 起），不是 1032 的 3 段。
	read, err := store.Read(ReadRequest{
		ObservationID: observation.ObservationID,
		MixSessionID:  observation.MixSessionID,
		Keys:          []string{"track.1017.slow.time_energy.summary", "track.1017.slow.stereo.summary"},
		MaxItems:      48,
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	items, _ := read["items"].(map[string]any)
	timeEnergy, _ := items["track.1017.slow.time_energy.summary"].(map[string]any)
	if timeEnergy == nil {
		t.Fatalf("time_energy view item missing: %v", items)
	}
	rows := mapRowsAny(timeEnergy["rows"])
	if len(rows) != 2 {
		t.Fatalf("time_energy rows = %d (want 2 segments from track 1017); body: %#v", len(rows), timeEnergy)
	}
	if got := numberFromMap(rows[0], "rms_dbfs"); got != -30.1 {
		t.Fatalf("time_energy first segment rms_dbfs = %v (want -30.1, guitar's own segment)", got)
	}

	// stereo 身份断言：素材属于 1017 自身。
	stereo, _ := items["track.1017.slow.stereo.summary"].(map[string]any)
	if stereo == nil {
		t.Fatalf("stereo view item missing: %v", items)
	}
	if got := cleanAnyString(stereo["track_id"]); got != "1017" {
		t.Fatalf("stereo view bound track_id = %q (want 1017, not vocals/1032)", got)
	}
	if got := cleanAnyString(stereo["clip_id"]); got != "1021" {
		t.Fatalf("stereo view bound clip_id = %q (want 1021)", got)
	}
	if got := numberFromMap(stereo, "balance_db"); got != -1.5 {
		t.Fatalf("stereo balance_db = %v (want -1.5, guitar's own row; 5.945 is vocals)", got)
	}

	// 单数 WE 槽绑回目标轨（caps 就绪，COM 源动力学可用）。
	if got := observation.SourceCapabilities["waveform_envelope"]; got != "ready" {
		t.Fatalf("caps waveform_envelope = %q (want ready bound to 1017)", got)
	}
	global, _ := observation.GlobalSummary["feature_snapshot"].(map[string]any)
	if global == nil {
		t.Fatalf("observation feature snapshot evidence missing")
	}
	singular, _ := global["waveform_envelope"].(map[string]any)
	if got := cleanAnyString(singular["track_id"]); got != "1017" {
		t.Fatalf("singular WE slot track_id = %q (want 1017)", got)
	}
}

// COM source_only 也吃同一目标绑定（com_projection.go 读组装后单数 WE 槽）：
// 1017 观察的 source_dynamics 不得是 missing。
func TestRequestObservationCOMEvidenceFollowsRequestedTarget(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.Now = func() time.Time { return time.Date(2026, 10, 15, 13, 33, 0, 0, time.UTC) }
	snapshotPath := writeTimedynGapSnapshot(t, timedynGapSnapshot(true))

	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_m2tg_com",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "1017", Label: "guitar"},
		ListenScope:  ListenScope{Source: ListenSourceScope{Mode: "selected_track"}},
		ProjectState: timedynGapProjectState(),
		Args: map[string]any{
			"track_id":              "1017",
			"clip_id":               "1021",
			"scope":                 "selected_track",
			"com_mode":              "source_only",
			"feature_snapshot_path": snapshotPath,
			"duration_seconds":      20.0,
		},
	})
	if err != nil {
		t.Fatalf("RequestObservation: %v", err)
	}
	com := result.Observation.COMProjection
	if com == nil {
		t.Fatalf("com_projection missing for source_only observation")
	}
	if com.Status == "missing" {
		t.Fatalf("com_projection status = missing; source dynamics must follow the requested target's segments")
	}
	if com.SourceDynamics == nil || com.SourceDynamics.Activity.ValidSegmentCount == 0 {
		t.Fatalf("com source_dynamics has no valid segments; body: %#v", com.SourceDynamics)
	}
}

// 目标轨无行=诚实 omission 口径不变：不端出别的轨的素材。
func TestRequestObservationTargetWithoutRowsKeepsHonestOmission(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	store.Now = func() time.Time { return time.Date(2026, 10, 15, 13, 33, 0, 0, time.UTC) }
	snapshotPath := writeTimedynGapSnapshot(t, timedynGapSnapshot(false))

	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_m2tg_omit",
		Round:        1,
		TargetRef:    TargetRef{Kind: "track", ID: "1017", Label: "guitar"},
		ListenScope:  ListenScope{Source: ListenSourceScope{Mode: "selected_track"}},
		ProjectState: timedynGapProjectState(),
		Args: map[string]any{
			"track_id":              "1017",
			"clip_id":               "1021",
			"scope":                 "selected_track",
			"feature_snapshot_path": snapshotPath,
			"duration_seconds":      20.0,
		},
	})
	if err != nil {
		t.Fatalf("RequestObservation: %v", err)
	}
	observation := result.Observation

	read, err := store.Read(ReadRequest{
		ObservationID: observation.ObservationID,
		MixSessionID:  observation.MixSessionID,
		Keys:          []string{"track.1017.slow.time_energy.summary", "track.1017.slow.stereo.summary"},
		MaxItems:      48,
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	items, _ := read["items"].(map[string]any)
	timeEnergy, _ := items["track.1017.slow.time_energy.summary"].(map[string]any)
	if timeEnergy == nil {
		t.Fatalf("time_energy view item missing")
	}
	if rows := mapRowsAny(timeEnergy["rows"]); len(rows) != 0 {
		t.Fatalf("time_energy rows = %d (want 0; target 1017 has no rows and vocals' segments must not leak)", len(rows))
	}
	stereo, _ := items["track.1017.slow.stereo.summary"].(map[string]any)
	if stereo == nil {
		t.Fatalf("stereo view item missing")
	}
	if got := cleanAnyString(stereo["status"]); got != "missing" {
		t.Fatalf("stereo view status = %q (want honest missing stub, not vocals material)", got)
	}
	if got := cleanAnyString(stereo["judged_track_id"]); got != "1032" {
		t.Fatalf("stereo stub judged_track_id = %q (want displaced 1032 identity disclosed)", got)
	}
	if got := observation.SourceCapabilities["waveform_envelope"]; got != "missing" {
		t.Fatalf("caps waveform_envelope = %q (want missing; no row for requested target)", got)
	}
}

// 读侧身份守卫：遗留观察（修复前物化、stereo 槽装着 1032）在 track 视图键下
// 不得作为 1017 的证据通过；project 域观察不受影响。
func TestTrackScopedStereoSummaryReadGuard(t *testing.T) {
	misbound := map[string]any{
		"status": "ready", "track_id": "1032", "clip_id": "1036", "balance_db": 5.945,
	}
	trackObs := ObservationPacket{TargetRef: TargetRef{Kind: "track", ID: "1017"}}
	guarded := mapValue(trackScopedStereoSummary(misbound, trackObs))
	if got := cleanAnyString(guarded["status"]); got != "missing" {
		t.Fatalf("guarded stereo status = %q (want missing)", got)
	}
	if got := cleanAnyString(guarded["judged_track_id"]); got != "1032" {
		t.Fatalf("guarded stereo judged_track_id = %q (want 1032)", got)
	}
	if got := cleanAnyString(guarded["track_id"]); got != "1017" {
		t.Fatalf("guarded stereo target track_id = %q (want 1017)", got)
	}

	ownRow := map[string]any{"status": "ready", "track_id": "1017", "clip_id": "1021", "balance_db": -1.5}
	if passed := mapValue(trackScopedStereoSummary(ownRow, trackObs)); cleanAnyString(passed["status"]) != "ready" {
		t.Fatalf("target-owned stereo row must pass the guard unchanged: %#v", passed)
	}

	projectObs := ObservationPacket{TargetRef: TargetRef{Kind: "project", ID: "current"}}
	if passed := mapValue(trackScopedStereoSummary(misbound, projectObs)); cleanAnyString(passed["track_id"]) != "1032" {
		t.Fatalf("project-scope observation must keep row-based stereo material: %#v", passed)
	}

	// 无身份行（track_id 空）不得被守卫误伤。
	anonymous := map[string]any{"status": "ready", "balance_db": 0.4}
	if passed := mapValue(trackScopedStereoSummary(anonymous, trackObs)); cleanAnyString(passed["status"]) != "ready" {
		t.Fatalf("anonymous stereo row must pass the guard: %#v", passed)
	}
}
