package harness

// materialize_shadow_test.go — MAT-C 三态 flag 纯旁路锁定（红先行）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §7.1（off=现状逐字节一致；
// FinalizeObservationContext 原路径零改动是回退路径的设计核心）。
//
// 两态对拍（MAT-B TestNotifierNilIsByteIdentical 同款纪律）：off（env 未设，
// 零接线）vs shadow（VIT_DAW_MATERIALIZATION=shadow 全接线：flag 装配+Notifier
// 三挂点+影子轮重算/登记/对账）跑同一输入电池，全部既有可观察输出 canonical
// JSON 逐字节一致。shadow 态只多写物化库（新旁路面，不在对拍面内）——物化侧
// 以 Metrics 非空性守卫证明影子轮真的做了事，否则对拍空洞。

import (
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
	"vit-daw-agent/internal/shadow"
)

// matCShadowSummary 是影子轮后的物化侧摘要（非空性守卫用；off 态全零）。
type matCShadowSummary struct {
	RowsUpserted int64
	Divergences  int64
}

// runMatCShadowBattery 跑 MAT-B 电池+一轮影子 observe，返回既有可观察面 +
// 影子轮后的物化 Metrics（off 态为全零摘要）。
func runMatCShadowBattery(t *testing.T, mode string) (map[string][]byte, matCShadowSummary) {
	t.Helper()
	for _, kv := range [][2]string{{"VIT_DAW_MATERIALIZATION", mode}} {
		previous, had := os.LookupEnv(kv[0])
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			t.Fatal(err)
		}
		if had {
			t.Cleanup(func() { _ = os.Setenv(kv[0], previous) })
		} else {
			t.Cleanup(func() { _ = os.Unsetenv(kv[0]) })
		}
	}

	tmp := t.TempDir()
	matBBatteryTmp = tmp
	t.Cleanup(func() { matBBatteryTmp = "" })
	previousRoot, hadRoot := os.LookupEnv("VIT_MIXBOARD_ROOT")
	if err := os.Setenv("VIT_MIXBOARD_ROOT", filepath.Join(tmp, "mixboard")); err != nil {
		t.Fatal(err)
	}
	if hadRoot {
		t.Cleanup(func() { _ = os.Setenv("VIT_MIXBOARD_ROOT", previousRoot) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_MIXBOARD_ROOT") })
	}

	projectPath := filepath.Join(tmp, "proj", "song.vit")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}

	projectShadow := shadow.New(nil)
	h := NewWithSender(nil, projectShadow, nil) // 构造期读 flag（MAT-C 装配点）
	if _, err := h.ActivateProjectStore(projectPath, "matc_shadow_identity"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(projectstore.Deactivate)

	// MAT-B 电池本体（三挂点路径）。
	projectShadow.Initialize(matBProjectFull(-6.0, "r1"))
	projectShadow.ApplyDelta(matBDelta())
	for _, event := range matBTelemetryBattery() {
		h.IngestKernelTelemetry(event)
	}

	// MAT-C 影子轮：合成 observe 产物（旁路挂点——只写物化库）。
	rms, peak := -20.5, -3.2
	domProjection := dom.Build(dom.Input{
		Mode: dom.ModeSourceOnly, ObservationID: "obs_matc_1", MixSessionID: "mix_matc",
		CreatedAt: "2026-09-28T00:00:00Z",
		TargetRef: map[string]any{"kind": "track", "id": "T3"},
		Source: dom.SourceEvidence{Status: dom.StatusReady, Duration: 12.0,
			RMSDBFS: &rms, PeakDBFS: &peak,
			TimeSegments: []dom.TimeSegment{{StartSeconds: 0, EndSeconds: 12, RMSDBFS: &rms, PeakDBFS: &peak}}},
	})
	observation := mixboard.ObservationPacket{
		ObservationID: "obs_matc_1", MixSessionID: "mix_matc", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary: map[string]any{"feature_snapshot": map[string]any{
			"schema_version": "mixboard_feature_snapshot.v1",
			"track_waveform_envelopes": []any{map[string]any{
				"status": "ready", "track_id": "T3", "source_revision": "sr1",
				"rms_dbfs": -20.5, "peak_dbfs": -3.2, "headroom_db": -0.8,
				"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
				"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
				"time_segments": []any{map[string]any{"start_seconds": 0.0, "end_seconds": 12.0, "rms_dbfs": -20.5, "peak_dbfs": -3.2}},
			}},
		}},
		DOMProjection: &domProjection,
	}
	h.materializeObserveRound(&observation)

	observables := map[string][]byte{
		"shadow_snapshot": matBCanonical(t, projectShadow.Snapshot()),
		"shadow_summary":  matBCanonical(t, projectShadow.Summary()),
		"shadow_receipt":  matBCanonical(t, projectShadow.LatestChangeReceipt()),
		"shadow_window":   matBCanonical(t, projectShadow.ChangeWindow(2)),
		"feature_snapshot": matBReadFileCanonical(t,
			filepath.Join(tmp, "mixboard_feature_snapshot.json")),
	}
	h.featureMu.Lock()
	observables["waveform_collectors"] = matBCanonical(t, h.waveforms)
	observables["spectral_collectors"] = matBCanonical(t, h.spectrals)
	h.featureMu.Unlock()

	summary := matCShadowSummary{}
	if h.materializeStore != nil {
		metrics := h.materializeStore.Metrics()
		for _, kind := range h.materializeStore.AdapterKinds() {
			summary.RowsUpserted += metrics.PerKind[kind].RowsUpserted
		}
		summary.Divergences = metrics.ShadowDivergences
	}
	return observables, summary
}

// TestMaterializationOffIsByteIdentical：off vs shadow 两态，既有可观察面逐字节
// 一致（三态 flag 的 off=现状，§7.1）；shadow 态物化非空（非空性守卫）。
func TestMaterializationOffIsByteIdentical(t *testing.T) {
	offObs, offSummary := runMatCShadowBattery(t, "")
	shadowObs, shadowSummary := runMatCShadowBattery(t, "shadow")

	if len(offObs) != len(shadowObs) {
		t.Fatalf("两态可观察面键数不一致 off=%d shadow=%d", len(offObs), len(shadowObs))
	}
	for key, offBytes := range offObs {
		shadowBytes, ok := shadowObs[key]
		if !ok {
			t.Fatalf("shadow 态缺可观察面 %q", key)
		}
		if string(offBytes) != string(shadowBytes) {
			t.Fatalf("纯旁路承诺被破坏：%s 两态不一致\noff=%s\nshadow=%s",
				key, matBTruncateForDiff(offBytes), matBTruncateForDiff(shadowBytes))
		}
	}
	// 非空性守卫：shadow 态影子轮必须真的物化过（否则上面的对拍什么都没证明）。
	if shadowSummary.RowsUpserted == 0 {
		t.Fatalf("shadow 态零物化行（对拍空洞）")
	}
	if offSummary.RowsUpserted != 0 || offSummary.Divergences != 0 {
		t.Fatalf("off 态不应有任何物化活动：%+v", offSummary)
	}
}
