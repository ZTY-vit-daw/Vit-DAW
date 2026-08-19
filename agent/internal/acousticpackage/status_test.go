package acousticpackage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreConcurrentUpsertsKeepStatusJSONReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acoustic_package_status.json")
	store := NewStore(path)
	const count = 24
	var group sync.WaitGroup
	errs := make(chan error, count)
	for index := 0; index < count; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			status := BuildStatus(Identity{ProjectID: "p", TrackID: fmt.Sprintf("track_%d", index), SourceRevision: "source_1"}, map[string]any{
				"waveform_envelope": map[string]any{"status": "ready", "track_id": fmt.Sprintf("track_%d", index), "source_revision": "source_1"},
			}, "2026-08-16T00:00:00Z", "test")
			if _, err := store.Upsert(status); err != nil {
				errs <- err
			}
		}(index)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	snapshot, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Packages) != count {
		t.Fatalf("concurrent upserts kept %d packages, want %d", len(snapshot.Packages), count)
	}
}

func TestBuildStatusMapsPartialSpectralCoverageAndDeferredFutureFeatures(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1", DurationSec: 20}
	status := BuildStatus(identity, map[string]any{
		"waveform_envelope":       map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "rms": 0.2, "peak_abs": 0.8, "time_segments": []any{map[string]any{"start_seconds": 0, "end_seconds": 5}}},
		"spectrogram_tiles":       map[string]any{"status": "partial", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "tile_count_seen": 2, "tile_count_expected": 8, "coverage_seconds": 10},
		"band_energy_summary":     map[string]any{"status": "partial", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "coverage_seconds": 10},
		"stereo_relation_summary": map[string]any{"status": "partial", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "coverage_seconds": 10},
	}, "2026-06-22T00:00:00Z", "test")
	if status.SchemaVersion != SchemaVersion || status.PackageLayers["l1_static"].Status != StatusReady {
		t.Fatalf("status = %+v", status)
	}
	l3 := status.PackageLayers["l3_deep"]
	if l3.Status != StatusPartial {
		t.Fatalf("l3 = %+v", l3)
	}
	progress := l3.Features["spectrogram_tiles"].Progress
	if progress.TileCountSeen != 2 || progress.TileCountExpected != 8 || progress.CoverageRatio != 0.25 {
		t.Fatalf("progress = %+v", progress)
	}
	if l3.Features["lufs_analysis"].Status != StatusDeferred ||
		l3.Features["masking_analysis"].Status != StatusDeferred ||
		l3.Features["reference_match"].Status != StatusDeferred {
		t.Fatalf("future features = %+v", l3.Features)
	}
}

func TestBuildStatusPromotesRealtimeRowsIntoL2Layer(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1", DurationSec: 20}
	status := BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "rms": 0.2, "peak_abs": 0.8},
		"realtime_band_energy_summary": map[string]any{
			"status":     "ready",
			"layer":      "l2_realtime",
			"source":     "live_level_meter_spectrum",
			"track_id":   "track_1",
			"clip_id":    "clip_1",
			"request_id": "req_live",
			"bands":      map[string]any{"bass": map[string]any{"unit_energy": 0.4}},
		},
		"realtime_stereo_relation_summary": map[string]any{
			"status":               "ready",
			"layer":                "l2_realtime",
			"source":               "live_level_meter_stereo",
			"track_id":             "track_1",
			"clip_id":              "clip_1",
			"request_id":           "req_live",
			"correlation_estimate": 0.8,
		},
	}, "2026-06-22T00:00:00Z", "test")
	l2 := status.PackageLayers["l2_realtime"]
	if l2.Status != StatusReady {
		t.Fatalf("l2 status = %q features=%+v", l2.Status, l2.Features)
	}
	if got := l2.Features["live_meter"].Status; got != StatusReady {
		t.Fatalf("live_meter status = %q feature=%+v", got, l2.Features["live_meter"])
	}
	if got := l2.Features["realtime_spectrum"].Status; got != StatusReady {
		t.Fatalf("realtime_spectrum status = %q feature=%+v", got, l2.Features["realtime_spectrum"])
	}
	if got := l2.Features["realtime_stereo_correlation"].Status; got != StatusReady {
		t.Fatalf("realtime_stereo_correlation status = %q feature=%+v", got, l2.Features["realtime_stereo_correlation"])
	}
	if got := l2.Features["post_fx_meter"].Status; got != StatusDeferred {
		t.Fatalf("post_fx_meter should remain deferred, got %q", got)
	}
}

func TestBuildStatusPromotesL2RenderProbeIntoPostFXMeter(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1", ClipRevision: "cliprev_1", RenderRevision: "render_1", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"l2_render_probe": map[string]any{
			"status":                  "ready",
			"feature_type":            "l2_render_probe",
			"layer":                   "l2_realtime",
			"source":                  "l2_render_probe",
			"track_id":                "track_1",
			"clip_id":                 "clip_1",
			"source_revision":         "rev_1",
			"clip_revision":           "cliprev_1",
			"render_revision":         "render_1",
			"track_state_fingerprint": "trackstate_1234",
			"tap_point":               "track_post_fader",
			"render_mode":             "offline_probe",
			"duration_seconds":        10.0,
			"sample_rate":             48000.0,
			"channel_count":           2,
			"peak_abs":                0.5,
			"rms":                     0.35,
			"quality_evidence":        map[string]any{"nonzero": true, "sum_abs": 10.0, "max_abs": 0.5, "nan_inf_count": 0, "coverage": 1.0, "latency_compensated": true, "tail_captured": true, "deterministic": true},
			"bands":                   map[string]any{"bass": map[string]any{"unit_energy": 0.9}},
			"correlation_state":       "highly_correlated",
			"evidence_ref":            "dad.l2_render_probe:render_1",
		},
	}, "2026-06-22T00:00:00Z", "test")

	l2 := status.PackageLayers["l2_realtime"]
	if got := l2.Features["render_probe"].Status; got != StatusReady {
		t.Fatalf("render_probe status = %q feature=%+v", got, l2.Features["render_probe"])
	}
	postFX := l2.Features["post_fx_meter"]
	if postFX.Status != StatusReady || postFX.Source != "l2_render_probe" {
		t.Fatalf("post_fx_meter = %+v", postFX)
	}
	if postFX.Ref["tap_point"] != "track_post_fader" || postFX.Ref["render_revision"] != "render_1" || postFX.Ref["track_state_fingerprint"] != "trackstate_1234" {
		t.Fatalf("post_fx_meter ref missing tap/render identity: %#v", postFX.Ref)
	}
}

func TestBuildStatusStalesOldL2RenderProbeOnRenderRevisionChange(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1", ClipRevision: "cliprev_1", RenderRevision: "render_new", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"l2_render_probe": map[string]any{
			"status":            "ready",
			"feature_type":      "l2_render_probe",
			"layer":             "l2_realtime",
			"source":            "l2_render_probe",
			"track_id":          "track_1",
			"clip_id":           "clip_1",
			"source_revision":   "rev_1",
			"clip_revision":     "cliprev_1",
			"render_revision":   "render_old",
			"tap_point":         "track_post_fader",
			"render_mode":       "offline_probe",
			"quality_evidence":  map[string]any{"nonzero": true, "sum_abs": 10.0, "max_abs": 0.5, "nan_inf_count": 0, "coverage": 1.0},
			"bands":             map[string]any{"bass": map[string]any{"unit_energy": 0.9}},
			"correlation_state": "highly_correlated",
		},
	}, "2026-06-22T00:00:00Z", "test")

	l2 := status.PackageLayers["l2_realtime"].Features
	for _, name := range []string{"render_probe", "post_fx_meter"} {
		feature := l2[name]
		if feature.Status != StatusStale || feature.Reason != "render_revision_mismatch" {
			t.Fatalf("%s should be stale after render revision change: %+v", name, feature)
		}
		if len(feature.Ref) != 0 {
			t.Fatalf("%s kept stale render evidence ref: %#v", name, feature.Ref)
		}
	}
}

func TestBuildStatusKeepsReadyL3RowsAcrossRenderRevisionChange(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1", ClipRevision: "cliprev_1", RenderRevision: "render_new", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"band_energy_summary": map[string]any{
			"status":              "ready",
			"feature_type":        "band_energy_summary",
			"layer":               "l3_deep",
			"source":              "kernel_l3_offline_analyzer",
			"track_id":            "track_1",
			"clip_id":             "clip_1",
			"source_revision":     "rev_1",
			"clip_revision":       "cliprev_1",
			"render_revision":     "render_old",
			"quality_status":      "ready",
			"tile_count_seen":     4,
			"tile_count_expected": 4,
			"bands":               map[string]any{"bass": map[string]any{"unit_energy": 0.35}},
		},
		"stereo_relation_summary": map[string]any{
			"status":               "ready",
			"feature_type":         "stereo_relation_summary",
			"layer":                "l3_deep",
			"source":               "kernel_l3_offline_analyzer",
			"track_id":             "track_1",
			"clip_id":              "clip_1",
			"source_revision":      "rev_1",
			"clip_revision":        "cliprev_1",
			"render_revision":      "render_old",
			"quality_status":       "ready",
			"tile_count_seen":      4,
			"tile_count_expected":  4,
			"correlation_estimate": 0.7,
		},
		"loudness_summary": map[string]any{
			"status":              "ready",
			"feature_type":        "loudness_summary",
			"layer":               "l3_deep",
			"source":              "kernel_l3_offline_analyzer",
			"track_id":            "track_1",
			"clip_id":             "clip_1",
			"source_revision":     "rev_1",
			"clip_revision":       "cliprev_1",
			"render_revision":     "render_old",
			"quality_status":      "ready",
			"tile_count_seen":     4,
			"tile_count_expected": 4,
			"approximate_lufs":    -14.2,
			"approximate":         true,
		},
	}, "2026-06-22T00:00:00Z", "test")

	l3 := status.PackageLayers["l3_deep"].Features
	for _, name := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		feature := l3[name]
		if feature.Status != StatusReady {
			t.Fatalf("%s should stay ready across render revision changes: %+v", name, feature)
		}
		if feature.Reason == "render_revision_mismatch" || len(feature.Ref) == 0 {
			t.Fatalf("%s lost ready L3 evidence: %+v", name, feature)
		}
	}
}

func TestBuildStatusReusesExactSourceFileL3AcrossProjectIdentity(t *testing.T) {
	identity := Identity{ProjectID: "vitproj_current", TrackID: "track_1", ClipID: "clip_1", SourcePath: `D:\audio\one.wav`, SourceRevision: "source-rev-1", ClipRevision: "clip-rev-1", RenderRevision: "render-new", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"band_energy_summary": map[string]any{
			"status": "ready", "feature_type": "band_energy_summary", "layer": "l3_deep", "source": "kernel_l3_offline_analyzer",
			"project_id": "current", "track_id": "track_1", "clip_id": "clip_1", "source_path": `D:\audio\one.wav`,
			"source_revision": "source-rev-1", "clip_revision": "clip-rev-1", "render_revision": "render-old",
			"quality_status": "ready", "bands": map[string]any{"bass": map[string]any{"unit_energy": .35}},
		},
		"l2_render_probe": map[string]any{
			"status": "ready", "feature_type": "l2_render_probe", "layer": "l2_realtime", "source": "l2_render_probe",
			"project_id": "current", "track_id": "track_1", "clip_id": "clip_1", "source_path": `D:\audio\one.wav`,
			"source_revision": "source-rev-1", "clip_revision": "clip-rev-1", "render_revision": "render-old", "bands": map[string]any{"bass": map[string]any{"unit_energy": .9}},
		},
	}, "2026-07-30T00:00:00Z", "test")
	if got := status.PackageLayers["l3_deep"].Features["band_energy_summary"]; got.Status != StatusReady || len(got.Ref) == 0 {
		t.Fatalf("exact source-file L3 should survive project relabel: %+v", got)
	}
	if got := status.PackageLayers["l2_realtime"].Features["render_probe"]; got.Status != StatusStale || len(got.Ref) != 0 {
		t.Fatalf("render-dependent L2 must not use project/source-file exception: %+v", got)
	}
}

func TestStoreReplacesRenderRevisionWithoutStalingSourceFileL3(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "acoustic_package_status.json"))
	old := BuildStatus(Identity{ProjectID: "p1", TrackID: "t1", ClipID: "c1", SourceRevision: "source-1", ClipRevision: "clip-1", RenderRevision: "render-old"}, map[string]any{
		"band_energy_summary": map[string]any{"status": "ready", "feature_type": "band_energy_summary", "source": "kernel_l3_offline_analyzer", "project_id": "p1", "track_id": "t1", "clip_id": "c1", "source_revision": "source-1", "clip_revision": "clip-1", "bands": map[string]any{"bass": map[string]any{"unit_energy": .3}}},
	}, "2026-07-30T00:00:00Z", "test")
	if _, err := store.Upsert(old); err != nil {
		t.Fatal(err)
	}
	derived := BuildStatus(Identity{ProjectID: "p1", TrackID: "t1", ClipID: "c1", SourceRevision: "source-1", ClipRevision: "clip-1", RenderRevision: "render-new"}, nil, "2026-07-30T00:01:00Z", "test")
	merged := MergeStatus(old, derived)
	written, err := store.Upsert(merged)
	if err != nil {
		t.Fatal(err)
	}
	if len(written.Packages) != 1 {
		t.Fatalf("render revision created duplicate package identities: %+v", written.Packages)
	}
	feature := written.Packages[0].PackageLayers["l3_deep"].Features["band_energy_summary"]
	if feature.Status != StatusReady || len(feature.Ref) == 0 || written.Packages[0].RenderRevision != "render-new" {
		t.Fatalf("source-file L3 did not survive package render update: package=%+v feature=%+v", written.Packages[0], feature)
	}
}

func TestBuildStatusDefersStaleRealtimeRowsForCurrentPlayback(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_current", SourceRevision: "rev_current", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_current", "source_revision": "rev_current", "rms": 0.2, "peak_abs": 0.8},
		"realtime_band_energy_summary": map[string]any{
			"status":       "ready",
			"layer":        "l2_realtime",
			"feature_type": "realtime_band_energy_summary",
			"source":       "live_level_meter_spectrum",
			"source_kind":  "realtime_level_meter",
			"track_id":     "track_1",
			"clip_id":      "clip_old",
			"request_id":   "req_old_live",
			"bands":        map[string]any{"bass": map[string]any{"unit_energy": 0.4}},
		},
		"realtime_stereo_relation_summary": map[string]any{
			"status":               "ready",
			"layer":                "l2_realtime",
			"feature_type":         "realtime_stereo_relation_summary",
			"source":               "live_level_meter_stereo",
			"source_kind":          "realtime_level_meter",
			"track_id":             "track_1",
			"clip_id":              "clip_old",
			"request_id":           "req_old_live",
			"correlation_estimate": 0.8,
		},
		"band_energy_summary": map[string]any{
			"status":          "ready",
			"track_id":        "track_1",
			"clip_id":         "clip_old",
			"source_revision": "rev_old",
		},
	}, "2026-06-22T00:00:00Z", "test")
	l2 := status.PackageLayers["l2_realtime"]
	for _, name := range []string{"live_meter", "realtime_spectrum", "realtime_stereo_correlation"} {
		if got := l2.Features[name].Status; got != StatusDeferred {
			t.Fatalf("%s status = %q feature=%+v", name, got, l2.Features[name])
		}
	}
	if got := status.PackageLayers["l3_deep"].Features["band_energy_summary"].Status; got != StatusStale {
		t.Fatalf("L3 stale guard was weakened: %q feature=%+v", got, status.PackageLayers["l3_deep"].Features["band_energy_summary"])
	}
}

func TestBuildStatusMarksRevisionMismatchStale(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_new"}
	status := BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_old"},
	}, "2026-06-22T00:00:00Z", "test")
	if got := status.PackageLayers["l1_static"].Features["waveform_envelope"].Status; got != StatusStale {
		t.Fatalf("waveform status = %q", got)
	}
}

func TestBuildStatusAcceptsKernelSourceRevisionDescriptorForLegacyIdentity(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "test_100hz_10s.wav")
	if err := os.WriteFile(sourcePath, []byte("fake-wave-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	kernelRevision := fmt.Sprintf("%s|size=%d|mtime=%d|length=10.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	kernelClipRevision := "clip=1013|track=1007|source=" + kernelRevision + "|offset=0.0000|length=10.0000"
	identity := Identity{
		ProjectID:         "current",
		TrackID:           "1007",
		ClipID:            "1013",
		SourcePath:        sourcePath,
		SourceRevision:    "rev_legacy",
		SourceFingerprint: "rev_legacy",
		ClipRevision:      "cliprev_legacy",
		DurationSec:       10,
	}
	snapshot := map[string]any{
		"waveform_envelope": map[string]any{
			"status":              "ready",
			"quality_status":      "ready",
			"quality_reason":      "ok",
			"track_id":            "1007",
			"clip_id":             "1013",
			"source_path":         sourcePath,
			"source_revision":     kernelRevision,
			"source_fingerprint":  kernelRevision,
			"clip_revision":       kernelClipRevision,
			"duration_seconds":    10,
			"tile_count_seen":     2,
			"tile_count_expected": 2,
			"rms":                 0.353529,
			"peak_abs":            0.499969,
			"time_segments":       []any{map[string]any{"start_seconds": 0, "end_seconds": 5}},
		},
		"spectrogram_tiles": map[string]any{
			"status":              "ready",
			"quality_status":      "ready",
			"quality_reason":      "ok",
			"track_id":            "1007",
			"clip_id":             "1013",
			"source_path":         sourcePath,
			"source_revision":     kernelRevision,
			"source_fingerprint":  kernelRevision,
			"clip_revision":       kernelClipRevision,
			"duration_seconds":    10,
			"tile_count_seen":     2,
			"tile_count_expected": 2,
		},
		"band_energy_summaries": []any{
			map[string]any{
				"status":              "ready",
				"quality_status":      "ready",
				"quality_reason":      "ok",
				"track_id":            "1007",
				"clip_id":             "1013",
				"source_path":         sourcePath,
				"source_revision":     kernelRevision,
				"source_fingerprint":  kernelRevision,
				"clip_revision":       kernelClipRevision,
				"duration_seconds":    10,
				"tile_count_seen":     2,
				"tile_count_expected": 2,
				"tile_count_parsed":   2,
				"bands":               map[string]any{"bass": map[string]any{"energy_db": -34.699}},
			},
		},
		"stereo_relation_summaries": []any{
			map[string]any{
				"status":               "ready",
				"quality_status":       "ready",
				"quality_reason":       "ok",
				"track_id":             "1007",
				"clip_id":              "1013",
				"source_path":          sourcePath,
				"source_revision":      kernelRevision,
				"source_fingerprint":   kernelRevision,
				"clip_revision":        kernelClipRevision,
				"duration_seconds":     10,
				"tile_count_seen":      2,
				"tile_count_expected":  2,
				"tile_count_parsed":    2,
				"balance_state":        "centered",
				"correlation_estimate": 1,
			},
		},
	}

	status := BuildStatus(identity, snapshot, "2026-06-22T00:00:00Z", "test")
	if status.SourceRevision != kernelRevision || status.SourceFingerprint != kernelRevision || status.ClipRevision != kernelClipRevision {
		t.Fatalf("status did not adopt kernel identity: %+v", status)
	}
	if got := status.PackageLayers["l1_static"].Features["waveform_envelope"].Status; got != StatusReady {
		t.Fatalf("waveform status = %q feature=%+v", got, status.PackageLayers["l1_static"].Features["waveform_envelope"])
	}
	l3 := status.PackageLayers["l3_deep"].Features
	if got := l3["band_energy_summary"].Status; got != StatusReady {
		t.Fatalf("band status = %q feature=%+v", got, l3["band_energy_summary"])
	}
	if got := l3["stereo_relation_summary"].Status; got != StatusReady {
		t.Fatalf("stereo status = %q feature=%+v", got, l3["stereo_relation_summary"])
	}
	if reason := l3["band_energy_summary"].Reason; reason == "source_revision_mismatch" {
		t.Fatalf("band kept mismatch reason: %+v", l3["band_energy_summary"])
	}
}

func TestStoreFindMatchesKernelDescriptorAgainstLegacyRevision(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "test_100hz_10s.wav")
	if err := os.WriteFile(sourcePath, []byte("fake-wave-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	kernelRevision := fmt.Sprintf("%s|size=%d|mtime=%d|length=10.0000", sourcePath, info.Size(), info.ModTime().UTC().UnixMilli())
	store := NewStore(filepath.Join(t.TempDir(), "acoustic_package_status.json"))
	status := BuildStatus(Identity{
		ProjectID:      "current",
		TrackID:        "1007",
		ClipID:         "1013",
		SourcePath:     sourcePath,
		SourceRevision: kernelRevision,
		DurationSec:    10,
	}, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "1007", "clip_id": "1013", "source_path": sourcePath, "source_revision": kernelRevision, "duration_seconds": 10, "rms": 0.2, "peak_abs": 0.7},
	}, "2026-06-22T00:00:00Z", "test")
	if _, err := store.Upsert(status); err != nil {
		t.Fatal(err)
	}
	found, ok, err := store.Find(Identity{
		ProjectID:         "current",
		TrackID:           "1007",
		ClipID:            "1013",
		SourcePath:        sourcePath,
		SourceRevision:    "rev_legacy",
		SourceFingerprint: "rev_legacy",
		DurationSec:       10,
	})
	if err != nil || !ok {
		t.Fatalf("find with legacy revision ok=%v err=%v", ok, err)
	}
	if found.SourceRevision != kernelRevision {
		t.Fatalf("found revision = %q, want %q", found.SourceRevision, kernelRevision)
	}
}

func TestComputeSourceRevisionIgnoresSessionAndPlacement(t *testing.T) {
	base := Identity{ProjectID: "current", SessionID: "mix_a", TrackID: "track_a", ClipID: "clip_a", SourceHash: "sha256:abc", SourcePath: `D:\Vit_DAW\same.wav`, DurationSec: 12.345}
	again := base
	again.SessionID = "mix_b"
	again.TrackID = "track_b"
	again.ClipID = "clip_b"
	if got, want := ComputeSourceRevision(again), ComputeSourceRevision(base); got != want {
		t.Fatalf("source revision changed with session/placement: got %q want %q", got, want)
	}
	if ComputeClipRevision(again) == ComputeClipRevision(base) {
		t.Fatalf("clip revision should distinguish placement")
	}
}

func TestSourceRevisionMatchesIdentityUsesDescriptorAndCurrentFileMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	if err := os.WriteFile(path, []byte("exact material"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := fmt.Sprintf("%s|size=%d|mtime=%d|length=10.0000", path, info.Size(), info.ModTime().UTC().UnixMilli())
	if !SourceRevisionMatchesIdentity(descriptor, Identity{SourcePath: path}) {
		t.Fatal("exact descriptor/file identity was rejected")
	}
	if SourceRevisionMatchesIdentity(descriptor, Identity{SourcePath: filepath.Join(t.TempDir(), "other.wav")}) {
		t.Fatal("different current path was accepted")
	}
}

func TestStoreUpsertKeepsStaleRowsSeparateFromCurrentRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acoustic_package_status.json")
	store := NewStore(path)
	store.Now = func() time.Time { return time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC) }
	oldStatus := BuildStatus(Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_old"}, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_old"},
	}, "2026-06-22T00:00:00Z", "test")
	if _, err := store.Upsert(oldStatus); err != nil {
		t.Fatal(err)
	}
	newStatus := BuildStatus(Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_new"}, nil, "2026-06-22T00:00:01Z", "test")
	if _, err := store.Upsert(newStatus); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"source_revision": "rev_old"`) || !strings.Contains(string(data), `"status": "stale"`) {
		t.Fatalf("store did not retain stale row: %s", string(data))
	}
	if found, ok, err := store.Find(Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_new"}); err != nil || !ok || found.SourceRevision != "rev_new" {
		t.Fatalf("find current = %+v ok=%v err=%v", found, ok, err)
	}
}

func TestStoreFindStableRevisionAcrossObservationSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acoustic_package_status.json")
	store := NewStore(path)
	identityA := Identity{ProjectID: "current", SessionID: "mix_a", TrackID: "track_1", ClipID: "clip_1", SourceHash: "sha256:abc", SourceRevision: "rev_stable", SourceFingerprint: "rev_stable", ClipRevision: "cliprev_1"}
	status := BuildStatus(identityA, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_stable", "rms": 0.2, "peak_abs": 0.7},
	}, "2026-06-22T00:00:00Z", "test")
	if _, err := store.Upsert(status); err != nil {
		t.Fatal(err)
	}
	identityB := identityA
	identityB.SessionID = "mix_b"
	found, ok, err := store.Find(identityB)
	if err != nil || !ok {
		t.Fatalf("find across session ok=%v err=%v", ok, err)
	}
	if found.SourceRevision != "rev_stable" || found.SessionID != "mix_a" {
		t.Fatalf("found = %+v", found)
	}
}

func TestDefaultStorePathPrefersVitDawDevRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "VitApp", "Workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_DEV_ROOT", root)
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	t.Setenv("VIT_ACOUSTIC_PACKAGE_STATUS_PATH", "")
	want := filepath.Join(root, "VitApp", "Workspace", "Artifacts", "acoustic_package_status.json")
	if got := DefaultStorePath(nil); got != want {
		t.Fatalf("DefaultStorePath = %q, want %q", got, want)
	}
}

func TestBuildStatusDoesNotPromoteRowsMissingSourceIdentity(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_current", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"band_energy_summary": map[string]any{"status": "ready", "track_id": "track_1", "clip_id": "clip_1"},
	}, "2026-06-22T00:00:00Z", "test")
	feature := status.PackageLayers["l3_deep"].Features["band_energy_summary"]
	if feature.Status != StatusStale || feature.Reason != "incomplete_source_identity" {
		t.Fatalf("feature = %+v", feature)
	}
}

func TestBuildStatusRejectsCrossSourceReadyRowsAndSelectsCurrentPartialRow(t *testing.T) {
	identity := Identity{
		ProjectID:      "current",
		SessionID:      "mix_paper_crown",
		TrackID:        "1007",
		ClipID:         "clip_a",
		SourcePath:     `D:\Vit_DAW\Paper Crown.mp3`,
		SourceRevision: "rev_paper",
		DurationSec:    219,
	}
	status := BuildStatus(identity, map[string]any{
		"band_energy_summary": map[string]any{
			"status":           "ready",
			"track_id":         "1007",
			"clip_id":          "clip_a",
			"source_path":      `D:\Vit_DAW\test_100hz_10s.wav`,
			"source_revision":  "rev_100hz",
			"duration_seconds": 10,
			"bands":            map[string]any{"bass": map[string]any{"energy_db": -6.02}},
		},
		"band_energy_summaries": []any{
			map[string]any{
				"status":           "partial",
				"track_id":         "1007",
				"clip_id":          "clip_a",
				"source_path":      `D:\Vit_DAW\Paper Crown.mp3`,
				"source_revision":  "rev_paper",
				"duration_seconds": 219,
				"coverage_seconds": 60,
				"bands":            map[string]any{"bass": map[string]any{"energy_db": -18}},
			},
		},
		"stereo_relation_summary": map[string]any{
			"status":           "ready",
			"track_id":         "1007",
			"clip_id":          "clip_a",
			"source_path":      `D:\Vit_DAW\test_100hz_10s.wav`,
			"source_revision":  "rev_100hz",
			"duration_seconds": 10,
		},
	}, "2026-06-22T00:00:00Z", "test")

	l3 := status.PackageLayers["l3_deep"].Features
	if got := l3["band_energy_summary"].Status; got != StatusPartial {
		t.Fatalf("band status = %q feature=%+v", got, l3["band_energy_summary"])
	}
	if ref := l3["band_energy_summary"].Ref; strings.Contains(fmt.Sprint(ref), "test_100hz_10s") || strings.Contains(fmt.Sprint(ref), "-6.02") {
		t.Fatalf("old source leaked into current band ref: %+v", ref)
	}
	if got := l3["stereo_relation_summary"].Status; got != StatusStale {
		t.Fatalf("stereo status = %q feature=%+v", got, l3["stereo_relation_summary"])
	}
}

func TestIdentityFromMapsKeepsExplicitSourceIdentityTogether(t *testing.T) {
	projectState := map[string]any{
		"tracks": []any{
			map[string]any{
				"track_id": "1007",
				"clips": []any{
					map[string]any{
						"clip_id":        "1014",
						"file_path":      `D:\Vit_DAW\test_100hz_10s.wav`,
						"length_seconds": 10.0,
					},
				},
			},
		},
	}
	resolved := map[string]any{
		"track_id":         "1007",
		"clip_id":          "1014",
		"file_path":        `D:\Vit_DAW\test_100hz_10s.wav`,
		"duration_seconds": 10.0,
	}
	args := map[string]any{
		"mix_session_id":    "mix_source_identity",
		"track_id":          "1007",
		"clip_id":           "clip_a",
		"file_path":         `D:\Vit_DAW\Paper Crown.mp3`,
		"source_revision":   "rev_paper",
		"duration_seconds":  219.0,
		"analyzer_revision": "agent_lightweight_o1",
	}

	identity := IdentityFromMaps(projectState, resolved, args)

	if identity.TrackID != "1007" || identity.ClipID != "clip_a" {
		t.Fatalf("target identity = track %q clip %q", identity.TrackID, identity.ClipID)
	}
	if identity.SourcePath != `D:\Vit_DAW\Paper Crown.mp3` || identity.SourceRevision != "rev_paper" || identity.SourceFingerprint != "rev_paper" {
		t.Fatalf("source identity was mixed with resolved target: %+v", identity)
	}
	if identity.DurationSec != 219 {
		t.Fatalf("duration = %v, want 219", identity.DurationSec)
	}
}

func TestNormalizeFeatureStatusDoesNotReuseGenericCurrentRequestAcrossProjects(t *testing.T) {
	identity := Identity{
		ProjectID:  "vitproj_goal5_live",
		TrackID:    "1032",
		ClipID:     "1036",
		SourcePath: "C:/fixtures/g5_02/stems/Vocals.wav",
	}
	legacy := map[string]any{
		"status":     "requested",
		"project_id": "current",
		"request_id": "old_project_request",
		"track_id":   "1032",
		"clip_id":    "1036",
	}
	if got := normalizeFeatureStatus(legacy, identity); got != StatusStale {
		t.Fatalf("generic prior-project request status=%q, want stale", got)
	}
	legacyWithoutProject := map[string]any{
		"status":     "requested",
		"request_id": "old_unstamped_request",
		"track_id":   "1032",
		"clip_id":    "1036",
	}
	if got := normalizeFeatureStatus(legacyWithoutProject, identity); got != StatusStale {
		t.Fatalf("unstamped prior-project request status=%q, want stale", got)
	}

	current := map[string]any{
		"status":     "requested",
		"project_id": "vitproj_goal5_live",
		"request_id": "current_project_request",
		"track_id":   "1032",
		"clip_id":    "1036",
	}
	if got := normalizeFeatureStatus(current, identity); got != StatusBuilding {
		t.Fatalf("specific current-project request status=%q, want building", got)
	}
}

func TestMissingDeferredAndFailedFeaturesDoNotBecomeReady(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1"}
	status := BuildStatus(identity, map[string]any{
		"spectrogram_tiles": map[string]any{"status": "error", "track_id": "track_1", "clip_id": "clip_1", "source_revision": "rev_1", "reason": "spectral_tile_parse_failed"},
	}, "2026-06-22T00:00:00Z", "test")
	if status.Status == StatusReady || status.PackageLayers["l1_static"].Status == StatusReady || status.PackageLayers["l3_deep"].Status == StatusReady {
		t.Fatalf("missing/failed package was promoted to ready: %+v", status)
	}
	l3 := status.PackageLayers["l3_deep"].Features
	if l3["spectrogram_tiles"].Status != StatusFailed {
		t.Fatalf("spectrogram_tiles status = %q", l3["spectrogram_tiles"].Status)
	}
	if l3["band_energy_summary"].Status == StatusReady || l3["stereo_relation_summary"].Status == StatusReady {
		t.Fatalf("missing derived summaries became ready: %+v", l3)
	}
	for _, name := range []string{"lufs_analysis", "masking_analysis", "reference_match"} {
		if l3[name].Status != StatusDeferred {
			t.Fatalf("%s status = %q", name, l3[name].Status)
		}
	}
}

func TestBuildStatusDoesNotTreatOldBlockedRowAsCurrentFailure(t *testing.T) {
	identity := Identity{
		ProjectID:      "current",
		SessionID:      "session_current",
		TrackID:        "1007",
		ClipID:         "1011",
		SourcePath:     `D:\Vit_DAW\test_100hz_10s.wav`,
		SourceRevision: "rev_current",
		DurationSec:    10,
	}
	status := BuildStatus(identity, map[string]any{
		"spectrogram_tiles": map[string]any{
			"status":     "blocked",
			"track_id":   "Track 1",
			"clip_id":    "",
			"reason":     "clip_source_required_for_current_feature_bakers",
			"request_id": "old_observe_without_source_identity",
		},
	}, "2026-06-22T00:00:00Z", "test")
	feature := status.PackageLayers["l3_deep"].Features["spectrogram_tiles"]
	if feature.Status != StatusStale {
		t.Fatalf("old blocked row was treated as current failure: %+v", feature)
	}
	warming := MarkBackgroundRequested(status, "test_background", "warm_requested", "2026-06-22T00:00:01Z")
	feature = warming.PackageLayers["l3_deep"].Features["spectrogram_tiles"]
	if feature.Status != StatusBuilding {
		t.Fatalf("stale old blocked row did not become background building: %+v", feature)
	}
}

func TestBuildStatusPrefersCurrentTrackClipRowOverOtherTrackWithSourceIdentity(t *testing.T) {
	identity := Identity{
		ProjectID:         "current",
		TrackID:           "1007",
		ClipID:            "1014",
		SourcePath:        `D:\Vit_DAW\test_100hz_10s.wav`,
		SourceRevision:    "rev_9e0abaf74c7153dd",
		SourceFingerprint: "rev_9e0abaf74c7153dd",
		DurationSec:       10,
	}
	status := BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{
			"status":          "ready",
			"track_id":        "1010",
			"clip_id":         "1016",
			"source_revision": "rev_other",
			"rms":             0.9,
			"peak_abs":        0.99,
			"request_id":      "kernel_prepared_waveform_envelope_1014",
		},
		"track_waveform_envelopes": []any{
			map[string]any{
				"status":     "ready",
				"track_id":   "1007",
				"clip_id":    "1014",
				"rms":        0.35,
				"peak_abs":   0.5,
				"request_id": "kernel_prepared_waveform_envelope_1014",
				"time_segments": []any{
					map[string]any{"start_seconds": 0, "end_seconds": 10, "rms": 0.35},
				},
			},
		},
	}, "2026-06-22T00:00:00Z", "test")

	l1 := status.PackageLayers["l1_static"].Features
	if got := l1["waveform_envelope"].Status; got != StatusReady {
		t.Fatalf("waveform status = %q feature=%+v", got, l1["waveform_envelope"])
	}
	if refTrack := cleanString(l1["peak_rms_summary"].Ref["track_id"]); refTrack != "1007" {
		t.Fatalf("selected other track ref: %+v", l1["peak_rms_summary"].Ref)
	}
	if rms := numberValue(l1["peak_rms_summary"].Ref["rms"]); rms != 0.35 {
		t.Fatalf("selected rms = %v ref=%+v", rms, l1["peak_rms_summary"].Ref)
	}
}

func TestBuildStatusAcceptsCurrentRequestRowsWithoutFullSourceIdentity(t *testing.T) {
	identity := Identity{
		ProjectID:         "current",
		TrackID:           "1007",
		ClipID:            "1014",
		SourcePath:        `D:\Vit_DAW\test_100hz_10s.wav`,
		SourceRevision:    "rev_9e0abaf74c7153dd",
		SourceFingerprint: "rev_9e0abaf74c7153dd",
		DurationSec:       10,
	}
	status := BuildStatus(identity, map[string]any{
		"band_energy_summary": map[string]any{
			"status":     "ready",
			"track_id":   "1007",
			"clip_id":    "1014",
			"request_id": "kernel_prepared_waveform_envelope_1014",
			"source":     "live_level_meter_spectrum",
			"bands": map[string]any{
				"bass": map[string]any{"unit_energy": 0.25, "energy_db": -12.0},
			},
		},
		"stereo_relation_summary": map[string]any{
			"status":               "ready",
			"track_id":             "1007",
			"clip_id":              "1014",
			"request_id":           "kernel_prepared_waveform_envelope_1014",
			"source":               "live_level_meter_stereo",
			"correlation_estimate": 0.9,
		},
	}, "2026-06-22T00:00:00Z", "test")

	l3 := status.PackageLayers["l3_deep"].Features
	if got := l3["band_energy_summary"].Status; got != StatusReady {
		t.Fatalf("band status = %q feature=%+v", got, l3["band_energy_summary"])
	}
	if got := l3["stereo_relation_summary"].Status; got != StatusReady {
		t.Fatalf("stereo status = %q feature=%+v", got, l3["stereo_relation_summary"])
	}
}

func TestBuildStatusAddsReasonForBuildingFeatures(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1"}
	status := BuildStatus(identity, map[string]any{
		"spectrogram_tiles": map[string]any{"status": "building", "track_id": "track_1", "clip_id": "clip_1"},
	}, "2026-06-22T00:00:00Z", "test")
	feature := status.PackageLayers["l3_deep"].Features["spectrogram_tiles"]
	if feature.Status != StatusBuilding || feature.Reason == "" || feature.Progress.Reason == "" {
		t.Fatalf("building feature missing reason: %+v", feature)
	}
}

func TestMarkBackgroundRequestedBuildsOnlyEligibleL1AndL3Features(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_1"}
	status := BuildStatus(identity, nil, "2026-06-22T00:00:00Z", "test")
	status = MarkBackgroundRequested(status, "test_background", "warm_requested", "2026-06-22T00:00:01Z")
	for _, name := range []string{"waveform_envelope", "peak_rms_summary", "time_energy"} {
		if got := status.PackageLayers["l1_static"].Features[name].Status; got != StatusBuilding {
			t.Fatalf("l1 %s status = %q", name, got)
		}
	}
	for _, name := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary"} {
		if got := status.PackageLayers["l3_deep"].Features[name].Status; got != StatusBuilding {
			t.Fatalf("l3 %s status = %q", name, got)
		}
	}
	for _, name := range []string{"lufs_analysis", "masking_analysis", "reference_match"} {
		if got := status.PackageLayers["l3_deep"].Features[name].Status; got != StatusDeferred {
			t.Fatalf("future %s status = %q", name, got)
		}
	}
}

func TestMarkBackgroundRequestedClearsStaleEvidenceRefs(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "track_1", ClipID: "clip_1", SourceRevision: "rev_new", DurationSec: 10}
	status := BuildStatus(identity, map[string]any{
		"spectrogram_tiles": map[string]any{
			"status":              "ready",
			"track_id":            "old_track",
			"clip_id":             "old_clip",
			"source_revision":     "rev_old",
			"request_id":          "old_request",
			"tile_count_seen":     44,
			"tile_count_expected": 44,
			"total_duration":      219,
		},
	}, "2026-06-22T00:00:00Z", "test")
	if got := status.PackageLayers["l3_deep"].Features["spectrogram_tiles"].Status; got != StatusStale {
		t.Fatalf("precondition status = %q", got)
	}
	status = MarkBackgroundRequested(status, "test_background", "warm_requested", "2026-06-22T00:00:01Z")
	feature := status.PackageLayers["l3_deep"].Features["spectrogram_tiles"]
	if feature.Status != StatusBuilding {
		t.Fatalf("feature status = %+v", feature)
	}
	if len(feature.Ref) != 0 || feature.Progress.TileCountSeen != 0 || feature.Progress.CoverageSeconds != 0 {
		t.Fatalf("stale evidence was retained: %+v", feature)
	}
	if feature.Progress.Reason != "warm_requested" {
		t.Fatalf("progress reason = %+v", feature.Progress)
	}
}

func TestMergeStatusDoesNotLetStoredStaleRefsOverrideCurrentDerivedEvidence(t *testing.T) {
	identity := Identity{
		ProjectID:      "current",
		TrackID:        "1007",
		ClipID:         "1013",
		SourcePath:     `D:\Vit_DAW\Paper Crown.mp3`,
		SourceRevision: "rev_paper",
		DurationSec:    219,
	}
	derived := BuildStatus(identity, map[string]any{
		"spectrogram_tiles": map[string]any{
			"status":              "partial",
			"track_id":            "1007",
			"clip_id":             "1013",
			"source_path":         `D:\Vit_DAW\Paper Crown.mp3`,
			"source_revision":     "rev_paper",
			"duration_seconds":    219,
			"request_id":          "kernel_prepared_spectral_field_1013",
			"tile_count_seen":     44,
			"tile_count_expected": 44,
			"coverage_seconds":    219,
		},
		"band_energy_summary": map[string]any{
			"status":           "partial",
			"track_id":         "1007",
			"clip_id":          "1013",
			"source_path":      `D:\Vit_DAW\Paper Crown.mp3`,
			"source_revision":  "rev_paper",
			"duration_seconds": 219,
			"request_id":       "kernel_prepared_spectral_field_1013",
			"coverage_seconds": 219,
			"bands":            map[string]any{"bass": map[string]any{"unit_energy": 0.31}},
		},
	}, "2026-06-22T00:00:01Z", "derived")
	stored := Status{
		SchemaVersion:  SchemaVersion,
		Status:         StatusReady,
		ProjectID:      "current",
		TrackID:        "1007",
		ClipID:         "1013",
		SourcePath:     `D:\Vit_DAW\Paper Crown.mp3`,
		SourceRevision: "rev_paper",
		DurationSec:    219,
		PackageLayers: map[string]LayerStatus{
			"l3_deep": {
				Status: StatusReady,
				Features: map[string]FeatureStatus{
					"spectrogram_tiles": {
						Status: StatusReady,
						Source: "old_store",
						Progress: Progress{
							TileCountSeen:     44,
							TileCountExpected: 44,
							CoverageSeconds:   219,
						},
						Ref: map[string]any{
							"status":              "ready",
							"track_id":            "1007",
							"clip_id":             "1011",
							"source_revision":     "rev_old",
							"request_id":          "kernel_prepared_waveform_envelope_1011",
							"tile_count_seen":     44,
							"tile_count_expected": 44,
						},
					},
					"band_energy_summary": {
						Status: StatusReady,
						Source: "old_store",
						Ref: map[string]any{
							"status":     "ready",
							"track_id":   "1007",
							"clip_id":    "1013",
							"request_id": "old_observation_without_source_identity",
							"bands":      map[string]any{"bass": map[string]any{"unit_energy": 0}},
						},
					},
				},
			},
		},
	}

	merged := MergeStatus(stored, derived)
	l3 := merged.PackageLayers["l3_deep"].Features
	if req := cleanString(l3["spectrogram_tiles"].Ref["request_id"]); req == "kernel_prepared_waveform_envelope_1011" {
		t.Fatalf("stored stale spectrogram ref overrode current evidence: %+v", l3["spectrogram_tiles"])
	}
	if clip := cleanString(l3["spectrogram_tiles"].Ref["clip_id"]); clip != "1013" {
		t.Fatalf("spectrogram ref clip = %q feature=%+v", clip, l3["spectrogram_tiles"])
	}
	if req := cleanString(l3["band_energy_summary"].Ref["request_id"]); req == "old_observation_without_source_identity" {
		t.Fatalf("stored source-less band ref overrode current evidence: %+v", l3["band_energy_summary"])
	}
	if got := l3["band_energy_summary"].Status; got != StatusPartial {
		t.Fatalf("band status = %q feature=%+v", got, l3["band_energy_summary"])
	}
}

func TestMergeStatusDoesNotLetStoredStaleRealtimeOverrideDeferred(t *testing.T) {
	identity := Identity{ProjectID: "current", TrackID: "1007", ClipID: "1014", SourceRevision: "rev_current", DurationSec: 10}
	derived := BuildStatus(identity, map[string]any{
		"waveform_envelope": map[string]any{"status": "ready", "track_id": "1007", "clip_id": "1014", "source_revision": "rev_current", "rms": 0.2, "peak_abs": 0.8},
	}, "2026-06-22T00:00:01Z", "derived")
	stored := Status{
		SchemaVersion:  SchemaVersion,
		Status:         StatusStale,
		ProjectID:      "current",
		TrackID:        "1007",
		ClipID:         "1014",
		SourceRevision: "rev_current",
		DurationSec:    10,
		PackageLayers: map[string]LayerStatus{
			"l2_realtime": {
				Status: StatusStale,
				Features: map[string]FeatureStatus{
					"live_meter": {
						Status:    StatusStale,
						Source:    "live_level_meter_spectrum",
						Reason:    "clip_mismatch",
						UpdatedAt: "2026-06-23T16:21:50Z",
						Progress:  Progress{Reason: "clip_mismatch", UpdatedAt: "2026-06-23T16:21:50Z"},
					},
					"realtime_spectrum": {
						Status:    StatusStale,
						Source:    "live_level_meter_spectrum",
						Reason:    "clip_mismatch",
						UpdatedAt: "2026-06-23T16:21:50Z",
						Progress:  Progress{Reason: "clip_mismatch", UpdatedAt: "2026-06-23T16:21:50Z"},
					},
					"realtime_stereo_correlation": {
						Status:    StatusStale,
						Source:    "live_level_meter_stereo",
						Reason:    "clip_mismatch",
						UpdatedAt: "2026-06-23T16:21:50Z",
						Progress:  Progress{Reason: "clip_mismatch", UpdatedAt: "2026-06-23T16:21:50Z"},
					},
				},
			},
		},
	}

	merged := MergeStatus(stored, derived)
	l2 := merged.PackageLayers["l2_realtime"]
	if l2.Status != StatusDeferred {
		t.Fatalf("l2 status = %q features=%+v", l2.Status, l2.Features)
	}
	for _, name := range []string{"live_meter", "realtime_spectrum", "realtime_stereo_correlation"} {
		if got := l2.Features[name].Status; got != StatusDeferred {
			t.Fatalf("%s status = %q feature=%+v", name, got, l2.Features[name])
		}
	}
}
