package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestL3FeatureRequestSnapshotPreservesExistingL2ProbeCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixboard_feature_snapshot.json")
	existing := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"l2_render_probe": map[string]any{
			"status": "ready", "track_id": "t1", "request_id": "l2-t1", "tap_point": "track_post_fader", "render_revision": "render-1",
		},
		"l2_render_probes": []any{
			map[string]any{"status": "ready", "track_id": "t1", "request_id": "l2-t1", "tap_point": "track_post_fader", "render_revision": "render-1"},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"request_id": "l3-project", "status": "requested", "project_id": "p1",
		"resolved_target": map[string]any{"kind": "project", "id": "current"},
	}
	writeMixboardFeatureRequestSnapshot(map[string]any{"feature_snapshot_path": path}, packet)
	written := readMixboardFeatureSnapshotFile(path)
	primary, _ := written["l2_render_probe"].(map[string]any)
	if firstString(primary, "request_id") != "l2-t1" {
		t.Fatalf("L3 request erased primary L2 probe: %#v", written)
	}
	rows := mapRowsFromAny(written["l2_render_probes"])
	if len(rows) != 1 || firstString(rows[0], "track_id") != "t1" {
		t.Fatalf("L3 request erased L2 probe cache: %#v", written)
	}
}

func TestL2ProbeCacheRequiresExactTrackSignalFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixboard_feature_snapshot.json")
	state := map[string]any{
		"project_uuid": "p1",
		"tracks": []any{map[string]any{
			"track_id": "t1", "volume_db": -2.0,
			"plugins": []any{map[string]any{"plugin_id": "eq1", "parameters": map[string]any{"gain": .4}}},
		}},
	}
	fingerprint := mixObservationTrackStateFingerprint(state, "t1")
	snapshot := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"l2_render_probes": []any{map[string]any{
			"status": "ready", "track_id": "t1", "request_id": "l2-t1", "tap_point": "track_post_fader",
			"render_revision": "render-1", "track_state_fingerprint": fingerprint, "evidence_ref": "probe:t1:1",
			"bands": map[string]any{"bass": map[string]any{"unit_energy": .3}},
		}},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := map[string]any{"feature_snapshot_path": path}
	if row := cachedL2RenderProbeForState(cmd, "t1", "track_post_fader", fingerprint); len(row) == 0 {
		t.Fatal("exact current-revision L2 probe was not reused")
	}
	state["tracks"].([]any)[0].(map[string]any)["volume_db"] = -1.0
	changed := mixObservationTrackStateFingerprint(state, "t1")
	if changed == fingerprint {
		t.Fatal("track signal change did not invalidate fingerprint")
	}
	if row := cachedL2RenderProbeForState(cmd, "t1", "track_post_fader", changed); len(row) != 0 {
		t.Fatalf("stale L2 probe was reused: %#v", row)
	}
}

func TestSingleTargetL2RequestPreservesProjectL3HistoryRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mixboard_feature_snapshot.json")
	existing := map[string]any{
		"schema_version": "mixboard_feature_snapshot.v1",
		"band_energy_summaries": []any{
			map[string]any{"status": "ready", "feature_type": "band_energy_summary", "source": "kernel_l3_offline_analyzer", "project_id": "current", "track_id": "t1", "clip_id": "c1", "source_revision": "source-1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}},
			map[string]any{"status": "ready", "feature_type": "band_energy_summary", "source": "kernel_l3_offline_analyzer", "project_id": "current", "track_id": "t2", "clip_id": "c2", "source_revision": "source-2", "bands": map[string]any{"bass": map[string]any{"unit_energy": .2}}},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	packet := map[string]any{
		"request_id": "l2-t1", "status": "requested", "project_id": "vitproj-current",
		"resolved_target":    map[string]any{"kind": "track", "track_id": "t1", "clip_id": "c1", "source_revision": "source-1"},
		"requested_features": []any{map[string]any{"feature_type": "l2_render_probe", "track_id": "t1", "clip_id": "c1"}},
	}
	writeMixboardFeatureRequestSnapshot(map[string]any{"feature_snapshot_path": path}, packet)
	written := readMixboardFeatureSnapshotFile(path)
	rows := mapRowsFromAny(written["band_energy_summaries"])
	if len(rows) != 2 || firstString(rows[0], "track_id") != "t1" || firstString(rows[1], "track_id") != "t2" {
		t.Fatalf("single-target L2 request erased project L3 history: %#v", written)
	}
}

func TestCollectL2RenderProbeBatchRenderedPathDoesNotCreateObservation(t *testing.T) {
	root := t.TempDir()
	t.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(root, "mixboard"))
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok", "project_uuid": "p1", "tracks": []any{map[string]any{"track_id": "1007", "track_name": "Drums", "track_type": "hybrid", "is_audio_track": true, "clips": []any{map[string]any{"id": "clip_a", "length_seconds": 2.0}}}}}}}
	h := NewWithSender(kernel, shadowProjectWithClips(), nil)
	h.l2ProbeCollect = func(_ context.Context, _ map[string]any, requestID, trackID, clipID string) (map[string]any, map[string]any, error) {
		return map[string]any{
			"schema_version": "dad_l2_render_probe.v1", "status": "ready", "quality_status": "ready",
			"feature_type": "l2_render_probe", "source": "l2_render_probe", "request_id": requestID,
			"track_id": trackID, "clip_id": clipID, "tap_point": "track_post_fader", "render_mode": "offline_probe",
			"render_revision": "render-1", "evidence_ref": "dad.l2_render_probe:render-1",
			"bands": map[string]any{"bass": map[string]any{"unit_energy": .3}},
		}, map[string]any{"status": "ok"}, nil
	}
	result, err := h.CollectL2RenderProbeBatch(context.Background(), L2RenderProbeBatchRequest{SessionID: "c1-target", GoalText: "run C1", TrackIDs: []string{"1007"}, TapPoint: "track_post_fader", ForceFresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" || len(result.RenderedTrackIDs) != 1 || result.RenderedTrackIDs[0] != "1007" || len(result.CacheHitTrackIDs) != 0 {
		t.Fatalf("rendered batch result = %#v", result)
	}
	var observations []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Base(filepath.Dir(path)) == "observations" {
			observations = append(observations, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 0 {
		t.Fatalf("compact L2 batch created MixBoard observations: %#v", observations)
	}
}

func TestCollectL2RenderProbeBatchUsesExactCacheWithoutRender(t *testing.T) {
	root := t.TempDir()
	featurePath := filepath.Join(root, "mixboard_feature_snapshot.json")
	h := NewWithSender(nil, shadowProjectWithClips(), nil)
	state := h.UserStateSummary(context.Background())
	fingerprint := mixObservationTrackStateFingerprint(state, "1007")
	if fingerprint == "" {
		t.Fatalf("test state omitted track fingerprint: %#v", state)
	}
	snapshot := map[string]any{"schema_version": "mixboard_feature_snapshot.v1", "l2_render_probes": []any{map[string]any{
		"status": "ready", "track_id": "1007", "clip_id": "clip_a", "tap_point": "track_post_fader", "request_id": "cached-1",
		"render_revision": "render-1", "track_state_fingerprint": fingerprint, "evidence_ref": "dad.l2_render_probe:render-1",
		"bands": map[string]any{"bass": map[string]any{"unit_energy": .3}},
	}}}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(featurePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	h.l2ProbeCollect = func(context.Context, map[string]any, string, string, string) (map[string]any, map[string]any, error) {
		t.Fatal("exact cache hit unexpectedly rendered")
		return nil, nil, nil
	}
	result, err := h.CollectL2RenderProbeBatch(context.Background(), L2RenderProbeBatchRequest{SessionID: "c1-cache", TrackIDs: []string{"1007"}, TapPoint: "track_post_fader", FeatureSnapshotPath: featurePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.RenderedTrackIDs) != 0 || len(result.CacheHitTrackIDs) != 1 || result.CacheHitTrackIDs[0] != "1007" {
		t.Fatalf("cache-only batch = %#v", result)
	}
}
