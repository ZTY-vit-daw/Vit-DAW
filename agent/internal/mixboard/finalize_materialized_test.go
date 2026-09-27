package mixboard

// finalize_materialized_test.go — MAT-E 命中旁路 finalize 的编排同构锁定
// （红先行）。FinalizeObservationContextSkipDOM 必须与原函数除"两处 dom
// finalize 调用及其 note 计数"外逐行为同构：digest/catalog/mom/tim/fxm/com
// 全部照常编排。本测试以同输入对拍两路径的全部 finalize 产物，锁同步演化
// （原函数后续改动若不同步旁路副本，这里红）。

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func matEBuildParityRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		MixSessionID: "mix_mat_e_parity",
		TargetRef:    TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{"project_revision": "r1", "duration_seconds": 12.0},
		Args: map[string]any{
			"feature_snapshot": map[string]any{
				"schema_version": "mixboard_feature_snapshot.v1",
				"track_waveform_envelopes": []any{map[string]any{
					"status": "ready", "track_id": "track_1", "source_revision": "sr1",
					"rms_dbfs": -18.0, "peak_dbfs": -4.0, "headroom_db": -0.4,
					"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
					"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
					"time_segments": []any{map[string]any{
						"start_seconds": 0.0, "end_seconds": 6.0, "rms_dbfs": -18.0, "peak_dbfs": -4.0,
					}},
				}},
				"band_energy_summaries": []any{map[string]any{
					"status": "ready", "track_id": "track_1", "source_revision": "sr1",
					"coverage_ratio": 1.0, "duration_seconds": 12.0,
					"bands": map[string]any{
						"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0},
					},
				}},
			},
		},
	}
}

// TestFinalizeSkipDOMParityLock：同输入、同观察身份下，原路径 finalize 与
// "预置 dom 后跳过 dom finalize"的旁路路径产出逐值一致（dom 用原路径产物
// 预置——旁路只跳计算不跳编排）。
func TestFinalizeSkipDOMParityLock(t *testing.T) {
	prev, had := os.LookupEnv("VIT_MIXBOARD_ROOT")
	if err := os.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(t.TempDir(), "mixboard")); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv("VIT_MIXBOARD_ROOT", prev) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_MIXBOARD_ROOT") })
	}

	req := matEBuildParityRequest(t)
	now := "2026-09-29T00:00:00Z"

	snapshotA := loadFeatureSnapshot(req.Args)
	obsA := buildObservation(req, now, &snapshotA)
	FinalizeObservationContext(&obsA, req, now)
	if obsA.DOMProjection == nil {
		t.Fatalf("前提失效：原路径未产出 DOMProjection")
	}

	snapshotB := loadFeatureSnapshot(req.Args)
	obsB := buildObservation(req, now, &snapshotB)
	// 观察身份对齐（buildObservation 的 observation_id 随机——digest 含它）。
	obsB.ObservationID = obsA.ObservationID
	// 命中装配形态：dom 投影由物化行预置（取原路径产物作等价实例）。
	served := *obsA.DOMProjection
	obsB.DOMProjection = &served
	FinalizeObservationContextSkipDOM(&obsB, req, now)

	if !reflect.DeepEqual(obsA.Digest, obsB.Digest) {
		t.Fatalf("旁路路径 digest 与原路径不一致（编排同构被破坏）")
	}
	if !reflect.DeepEqual(obsA.Catalog, obsB.Catalog) {
		t.Fatalf("旁路路径 catalog 与原路径不一致（编排同构被破坏）")
	}
	for name, pair := range map[string][2]any{
		"mom": {obsA.MOMProjection, obsB.MOMProjection},
		"tim": {obsA.TIMProjection, obsB.TIMProjection},
		"fxm": {obsA.FXMProjection, obsB.FXMProjection},
		"com": {obsA.COMProjection, obsB.COMProjection},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Fatalf("旁路路径 %s 投影与原路径不一致（编排同构被破坏）", name)
		}
	}
	if !reflect.DeepEqual(obsA.DOMProjection, obsB.DOMProjection) {
		t.Fatalf("旁路路径改写了预置的 dom 投影（命中装配应原样透传）")
	}
}
