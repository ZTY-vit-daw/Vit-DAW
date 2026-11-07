package mixboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MIXBOARD-TRACKKEY-1 单测反钉。夹具蓝本=265 实盘观察包
// obs_20261106T152825_863f2de0d657（reverify1 工作区亲核）：
//   - TargetRef={kind:"project",id:"current"}（full_project scope）；
//   - feature snapshot 内联 stereo_relation_summaries 19 行 ready，覆盖
//     1007(×4)/1012/1017/1022/1027/1032 六轨，track 1039 无行；
//   - 修复前 mix.read 逐轨键（track.<id>.slow.stereo.summary）恒走
//     default → catalog_key_not_found → d2_check 7/7 missing。

const trackKeyStereoFixtureSnapshot = `{
	"schema_version":"mixboard_feature_snapshot.v1",
	"updated_at":"2026-09-05T13:32:20.966+08:00",
	"stereo_relation_summaries":[
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1007","clip_id":"1011","capture_mode":"offline_full_song","balance_db":-1.474863830506248,"balance_state":"centered","correlation_estimate":0.688632253089265,"correlation_state":"coherent","quality_status":"ready","updated_at":"2026-08-10T12:24:44.649+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1007","clip_id":"1011","capture_mode":"offline_full_song","balance_db":-1.474863830506248,"balance_state":"centered","correlation_estimate":0.688632253089265,"correlation_state":"coherent","quality_status":"ready","quality_evidence":{"sample_count":1764000},"updated_at":"2026-09-05T13:32:20.966+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1012","clip_id":"1013","capture_mode":"offline_full_song","balance_db":0.21,"balance_state":"centered","correlation_estimate":0.31,"correlation_state":"coherent","quality_status":"ready","updated_at":"2026-09-05T13:32:20.966+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1017","clip_id":"1018","capture_mode":"offline_full_song","balance_db":-0.4,"balance_state":"centered","correlation_estimate":0.95,"correlation_state":"coherent","quality_status":"ready","updated_at":"2026-09-05T13:32:20.966+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1022","clip_id":"1023","capture_mode":"offline_full_song","balance_db":2.2,"balance_state":"right_biased","correlation_estimate":0.12,"correlation_state":"unstable","quality_status":"ready","updated_at":"2026-09-05T13:32:20.966+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1027","clip_id":"1028","capture_mode":"offline_full_song","balance_db":-0.1,"balance_state":"centered","correlation_estimate":0.55,"correlation_state":"coherent","quality_status":"ready","updated_at":"2026-09-05T13:32:20.966+08:00"},
		{"status":"ready","feature_type":"stereo_relation_summary","track_id":"1032","clip_id":"1033","capture_mode":"offline_full_song","balance_db":0.0,"balance_state":"centered","correlation_estimate":0.77,"correlation_state":"coherent","quality_status":"ready","updated_at":"2026-09-05T13:32:20.966+08:00"}
	],
	"realtime_stereo_relation_summaries":[
		{"status":"ready","feature_type":"realtime_stereo_relation_summary","track_id":"1012","clip_id":"1013","capture_mode":"playback_live","balance_db":0.3,"balance_state":"centered","correlation_estimate":0.44,"correlation_state":"coherent","updated_at":"2026-09-05T13:40:00.000+08:00"}
	],
	"waveform_envelope":{"status":"missing"},
	"spectrogram_tiles":{"status":"missing"}
}`

// trackKeyStereoProjectState mirrors the 265 project: seven tracks
// (1007…1039) with one clip each; track 1039 is the no-snapshot-row negative
// anchor from the real packet. Clip ids mirror the snapshot rows so the
// project-material freshness filter (trackWaveformRowMatchesProjectState)
// keeps every fixture row.
func trackKeyStereoProjectState() map[string]any {
	clipOf := map[string]string{
		"1007": "1011", "1012": "1013", "1017": "1018", "1022": "1023",
		"1027": "1028", "1032": "1033", "1039": "1040",
	}
	tracks := []any{}
	for _, id := range []string{"1007", "1012", "1017", "1022", "1027", "1032", "1039"} {
		clipID := clipOf[id]
		clip := map[string]any{"clip_id": clipID, "clip_name": "clip_" + clipID, "length_seconds": 20.0}
		tracks = append(tracks, map[string]any{
			"track_id": id, "track_name": "Track " + id, "level_db": -18.0,
			"peak_dbfs": -3.0, "headroom_db": 3.0, "clips": []any{clip},
		})
	}
	return map[string]any{"tracks": tracks, "duration_seconds": 20.0}
}

func trackKeyStereoStore(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "feature_snapshot.json")
	if err := os.WriteFile(snapshotPath, []byte(trackKeyStereoFixtureSnapshot), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(root)
	result, err := store.RequestObservation(Request{
		MixSessionID: "mix_d1d3_trackkey_292",
		GoalText:     "d2 翻译检查",
		TargetRef:    TargetRef{Kind: "project", ID: "current", Label: "Current project", Source: "mix_observation_scope", Confidence: "high"},
		ListenScope:  ListenScope{Source: ListenSourceScope{Mode: "full_project"}},
		ProjectState: trackKeyStereoProjectState(),
		Args: map[string]any{
			"feature_snapshot_path": snapshotPath,
			"scope":                 "full_project",
			"project_context":       true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Observation.TargetRef.ID != "current" || result.Observation.TargetRef.Kind != "project" {
		t.Fatalf("fixture observation target_ref = %+v (want full_project {project,current})", result.Observation.TargetRef)
	}
	return store
}

// 正锚（265 实盘形状）：真实 track id 键 → ready 读数。六轨有行
// （track 1039 无行，负锚另测），track.1007 键值=实盘 corr 0.689 /
// balance -1.475（round3 后）。
func TestMixReadTrackKeyedStereoServesFullProjectSnapshotRows(t *testing.T) {
	store := trackKeyStereoStore(t)
	expected := map[string]float64{
		"1007": 0.689, "1012": 0.31, "1017": 0.95, "1022": 0.12, "1027": 0.55, "1032": 0.77,
	}
	// 键序确定（map 迭代随机）。
	ordered := []string{"1007", "1012", "1017", "1022", "1027", "1032"}
	keys := make([]string, 0, len(ordered))
	for _, trackID := range ordered {
		keys = append(keys, "track."+trackID+".slow.stereo.summary")
	}
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: keys})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ok" {
		t.Fatalf("read status = %v", result["status"])
	}
	items := mapValue(result["items"])
	for _, trackID := range ordered {
		item := mapValue(items["track."+trackID+".slow.stereo.summary"])
		if len(item) == 0 {
			t.Fatalf("track %s: item missing entirely: %#v", trackID, items)
		}
		if item["status"] != "ready" {
			t.Fatalf("track %s: status = %v (%s), want ready", trackID, item["status"], item["reason"])
		}
		correlation, ok := item["correlation_estimate"].(float64)
		if !ok || correlation != expected[trackID] {
			t.Fatalf("track %s: correlation_estimate = %#v, want %v", trackID, item["correlation_estimate"], expected[trackID])
		}
		if cleanAnyString(item["track_id"]) != trackID {
			t.Fatalf("track %s: disclosed row bound to %q (identity misbinding)", trackID, cleanAnyString(item["track_id"]))
		}
	}
	// 实盘值反钉（265 obs_152825 track 1007：corr 0.688632… → 0.689，
	// balance -1.474863… → -1.475）。
	item1007 := mapValue(items["track.1007.slow.stereo.summary"])
	if balance, ok := item1007["balance_db"].(float64); !ok || balance != -1.475 {
		t.Fatalf("track 1007 balance_db = %#v, want -1.475", item1007["balance_db"])
	}
	if item1007["correlation_state"] != "coherent" || item1007["balance_state"] != "centered" {
		t.Fatalf("track 1007 states = %v/%v", item1007["correlation_state"], item1007["balance_state"])
	}
}

// 负锚（265 实盘形状）：无行轨道（track 1039——实盘 snapshot 无该轨行）必须
// 返回诚实的逐轨 missing（键已路由，reason=stereo_relation_row_missing_for_track），
// 不再是修复前的 catalog_key_not_found 恒 miss。
func TestMixReadTrackKeyedStereoMissingTrackIsHonestPerTrackMissing(t *testing.T) {
	store := trackKeyStereoStore(t)
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: []string{"track.1039.slow.stereo.summary"}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result["items"])
	item := mapValue(items["track.1039.slow.stereo.summary"])
	if len(item) == 0 {
		t.Fatalf("item missing entirely: %#v", items)
	}
	if item["status"] != "missing" {
		t.Fatalf("status = %v, want missing", item["status"])
	}
	if item["reason"] == "catalog_key_not_found" {
		t.Fatalf("reason = catalog_key_not_found (pre-fix constant miss); want per-track honest reason")
	}
	if item["reason"] != "stereo_relation_row_missing_for_track" {
		t.Fatalf("reason = %v", item["reason"])
	}
	if cleanAnyString(item["track_id"]) != "1039" {
		t.Fatalf("track_id = %v", item["track_id"])
	}
}

// 键面边界：非 stereo 的 track-keyed 键与未知键维持 default miss
// （catalog_key_not_found），不被新路由误吞。
func TestMixReadTrackKeyedStereoDoesNotSwallowUnknownKeys(t *testing.T) {
	store := trackKeyStereoStore(t)
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: []string{
		"track.1007.slow.stereo.detail",
		"track.1007.fast.levels",
		"track.1007.slow.band_energy.summary",
		"track..slow.stereo.summary",
		"tracks.1007.slow.stereo.summary",
		"nonsense.key",
	}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result["items"])
	for _, key := range []string{
		"track.1007.slow.stereo.detail",
		"track.1007.fast.levels",
		"track.1007.slow.band_energy.summary",
		"track..slow.stereo.summary",
		"tracks.1007.slow.stereo.summary",
		"nonsense.key",
	} {
		item := mapValue(items[key])
		if len(item) == 0 {
			t.Fatalf("key %q: item missing entirely: %#v", key, items)
		}
		if item["reason"] != "catalog_key_not_found" {
			t.Fatalf("key %q: reason = %v, want catalog_key_not_found (default miss unchanged)", key, item["reason"])
		}
	}
}

// 回归锚：观察包自身目标键（track.current.slow.stereo.summary，full_project
// 观察 TargetRef id）仍走原精确 case 臂读 current_metrics.stereo_relation
// （实盘 265 包该槽为 singular 提升 ready，clip 1011）。
func TestMixReadTrackKeyedStereoOwnTargetKeyUnchanged(t *testing.T) {
	store := trackKeyStereoStore(t)
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: []string{"track.current.slow.stereo.summary"}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result["items"])
	item := mapValue(items["track.current.slow.stereo.summary"])
	if len(item) == 0 {
		t.Fatalf("item missing entirely: %#v", items)
	}
	if item["status"] != "ready" {
		t.Fatalf("own-target alias status = %v (%v); singular slot semantics must stay unchanged", item["status"], item["reason"])
	}
	if cleanAnyString(item["clip_id"]) != "1011" {
		t.Fatalf("own-target alias clip_id = %v, want 1011 (promoted singular slot)", item["clip_id"])
	}
}

// realtime stereo 面：同一 TargetRef 语义路由（行集=
// realtime_stereo_relation_summaries；实盘 265 包该行集为空 → 逐轨诚实
// missing；有行轨道（1012）→ ready 读数）。
func TestMixReadTrackKeyedRealtimeStereoRoutesSnapshotRows(t *testing.T) {
	store := trackKeyStereoStore(t)
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: []string{
		"track.1012.realtime.stereo.summary",
		"track.1007.realtime.stereo.summary",
	}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result["items"])
	withRow := mapValue(items["track.1012.realtime.stereo.summary"])
	if withRow["status"] != "ready" {
		t.Fatalf("track 1012 realtime status = %v (%v), want ready", withRow["status"], withRow["reason"])
	}
	if correlation, ok := withRow["correlation_estimate"].(float64); !ok || correlation != 0.44 {
		t.Fatalf("track 1012 realtime correlation = %#v, want 0.44", withRow["correlation_estimate"])
	}
	noRow := mapValue(items["track.1007.realtime.stereo.summary"])
	if noRow["status"] != "missing" || noRow["reason"] != "realtime_stereo_relation_row_missing_for_track" {
		t.Fatalf("track 1007 realtime = %#v, want honest per-track missing", noRow)
	}
}

// 证据快照参数兜底（`.raw.` 同款回溯链的参数路径）：内联 snapshot 不含行集
// （hard-projected 形态）时，已解析的 evidenceSnapshot 行集可服务逐轨读取。
func TestReadTrackKeyedStereoFallsBackToEvidenceSnapshot(t *testing.T) {
	obs := ObservationPacket{
		ObservationID: "obs_trackkey_evidence",
		MixSessionID:  "mix_d1d3_trackkey_292",
		TargetRef:     TargetRef{Kind: "project", ID: "current"},
		GlobalSummary: map[string]any{
			"feature_snapshot": map[string]any{"status": "projected", "evidence_ref": "evidence://objects/ff/feature_snapshot.json"},
		},
	}
	evidenceSnapshot := map[string]any{
		"stereo_relation_summaries": []any{map[string]any{
			"status": "ready", "track_id": "1027", "clip_id": "1028",
			"correlation_estimate": 0.55, "correlation_state": "coherent",
			"balance_db": -0.1, "balance_state": "centered",
		}},
	}
	value, ok := readObservationKey(obs, "track.1027.slow.stereo.summary", ReadRequest{}, evidenceSnapshot)
	if !ok {
		t.Fatalf("key not routed via evidence snapshot fallback")
	}
	item := mapValue(value)
	if item["status"] != "ready" {
		t.Fatalf("status = %v (%v)", item["status"], item["reason"])
	}
	if correlation, yes := item["correlation_estimate"].(float64); !yes || correlation != 0.55 {
		t.Fatalf("correlation = %#v, want 0.55", item["correlation_estimate"])
	}
	// 同一包内无行轨道仍诚实 missing（证据快照无 1039 行）。
	value, ok = readObservationKey(obs, "track.1039.slow.stereo.summary", ReadRequest{}, evidenceSnapshot)
	if !ok {
		t.Fatalf("no-row key must stay routable (honest missing), not default miss")
	}
	if missing := mapValue(value); missing["reason"] != "stereo_relation_row_missing_for_track" {
		t.Fatalf("reason = %v", missing["reason"])
	}
}

// 身份绑定负锚：行集只含他轨行时，请求轨不披露他轨材质（宁可逐轨 missing，
// 不做位移披露——与 trackScopedStereoSummary/promoteBestL3FeatureRows 的
// identity 守卫同一语义）。
func TestReadTrackKeyedStereoNeverDisclosesDisplacedRows(t *testing.T) {
	obs := ObservationPacket{
		ObservationID: "obs_trackkey_displaced",
		MixSessionID:  "mix_d1d3_trackkey_292",
		TargetRef:     TargetRef{Kind: "project", ID: "current"},
		GlobalSummary: map[string]any{
			"feature_snapshot": map[string]any{
				"stereo_relation_summaries": []any{
					map[string]any{"status": "ready", "track_id": "1007", "clip_id": "1011", "correlation_estimate": 0.689, "correlation_state": "coherent"},
					map[string]any{"status": "ready", "track_id": "", "clip_id": "1099", "correlation_estimate": 0.9, "correlation_state": "coherent"},
				},
			},
		},
	}
	for _, trackID := range []string{"1039", "1012"} {
		value, ok := readObservationKey(obs, "track."+trackID+".slow.stereo.summary", ReadRequest{})
		if !ok {
			t.Fatalf("track %s: key not routable", trackID)
		}
		item := mapValue(value)
		if item["status"] != "missing" || item["reason"] != "stereo_relation_row_missing_for_track" {
			t.Fatalf("track %s: %#v (displaced or unbound row must not be disclosed)", trackID, item)
		}
	}
}

// 观察包 JSON 往返钉：逐轨 stereo 披露经持久化观察文件读回仍成立
// （Store.Read 走 findObservation → readObservationFile 反序列化路径，
// 夹具 store 的每个 Read 都是磁盘读回——此处再显式核一次披露负载的
// 轨道绑定不丢失）。
func TestMixReadTrackKeyedStereoSurvivesObservationRoundTrip(t *testing.T) {
	store := trackKeyStereoStore(t)
	result, err := store.Read(ReadRequest{MixSessionID: "mix_d1d3_trackkey_292", Keys: []string{"track.1017.slow.stereo.summary"}})
	if err != nil {
		t.Fatal(err)
	}
	items := mapValue(result["items"])
	item := mapValue(items["track.1017.slow.stereo.summary"])
	if item["status"] != "ready" {
		t.Fatalf("round-trip read failed: %#v", item)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "1017") {
		t.Fatalf("round-trip payload lost track binding: %s", encoded)
	}
}
