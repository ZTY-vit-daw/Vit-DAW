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

// TestMaterializeShadowMeasurementCarriedRound：MAT-D2 合成盲区补测——MAT-C
// 电池的观察包 MixPackage 恒空，MixPackage 测量回退轴在合成面永不激活（真栈
// S2 红形态因此漏网）。本用例补一个带测量的观察轮：snapshot 目标键空 +
// current_metrics.waveform 命中 → 物化侧登记 measurement_carried+non_precomputable
// 注记，ZeroDivergence 闸门排除该行并单列计数——ShadowMeasurementCarriedExcluded>=1
// 且 ShadowDivergences==0（分歧行已入排除计数，不静默）。
func TestMaterializeShadowMeasurementCarriedRound(t *testing.T) {
	previous, had := os.LookupEnv("VIT_DAW_MATERIALIZATION")
	if err := os.Setenv("VIT_DAW_MATERIALIZATION", "shadow"); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv("VIT_DAW_MATERIALIZATION", previous) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_DAW_MATERIALIZATION") })
	}

	h := NewWithSender(nil, nil, nil) // shadow flag → 物化层接线；shadow project/logger 缺省即可
	if h.materializeStore == nil {
		t.Fatalf("shadow 态物化库未接线")
	}

	// 回退激活轮：目标 T3；snapshot 无 T3 目标键行；MixPackage 测量命中。
	observation := mixboard.ObservationPacket{
		ObservationID: "obs_matd2_h1", MixSessionID: "mix_matd2", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary: map[string]any{"feature_snapshot": map[string]any{
			"schema_version": "mixboard_feature_snapshot.v1",
		}},
		MixPackage: map[string]any{"current_metrics": map[string]any{
			"waveform": map[string]any{
				"status": "ready", "track_id": "T3", "source_revision": "sr_live",
				"rms_dbfs": -18.0, "peak_dbfs": -4.0, "headroom_db": -0.4,
				"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
				"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
				"time_segments": []any{map[string]any{"start_seconds": 0.0, "end_seconds": 12.0, "rms_dbfs": -18.0, "peak_dbfs": -4.0}},
			},
		}},
	}
	projection := dom.Build(mixboard.DOMInputFromObservation(observation, mixboard.Request{}))
	observation.DOMProjection = &projection

	h.materializeObserveRound(&observation)

	m := h.materializeStore.Metrics()
	if m.ShadowMeasurementCarriedExcluded < 1 {
		t.Fatalf("回退激活轮应产生排除计数 ShadowMeasurementCarriedExcluded>=1: got=%d", m.ShadowMeasurementCarriedExcluded)
	}
	if m.ShadowDivergences != 0 {
		t.Fatalf("measurement_carried 行被闸门排除后 ShadowDivergences 应为 0: got=%d", m.ShadowDivergences)
	}
}

// TestMaterializeShadowTimingCarriedRound：MAT-D4 时序两态合成复刻——观察包的
// DOMProjection 是 finalize 世代输入算的，GlobalSummary.feature_snapshot 已是
// 尾挂世代（MAT-D3 取证 §3.3 形态）：影子轮应把该分歧登记为 timing-carried
// （排除+单列计数 ShadowTimingCarriedExcluded>=1，不计分歧）；随后同世代收敛
// 轮零分歧、排除计数不涨、真比过（ReconcileRows 递增）——G2-D 收敛态闭环的
// 合成最小形态。
func TestMaterializeShadowTimingCarriedRound(t *testing.T) {
	previous, had := os.LookupEnv("VIT_DAW_MATERIALIZATION")
	if err := os.Setenv("VIT_DAW_MATERIALIZATION", "shadow"); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv("VIT_DAW_MATERIALIZATION", previous) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_DAW_MATERIALIZATION") })
	}

	matD4Waveform := func(sourceRevision string, rms, peak float64) map[string]any {
		return map[string]any{
			"status": "ready", "track_id": "T3", "source_revision": sourceRevision,
			"rms_dbfs": rms, "peak_dbfs": peak, "headroom_db": -peak,
			"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
			"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
			"time_segments": []any{map[string]any{"start_seconds": 0.0, "end_seconds": 12.0, "rms_dbfs": rms, "peak_dbfs": peak}},
		}
	}
	snapshotWith := func(row map[string]any) map[string]any {
		return map[string]any{
			"schema_version":           "mixboard_feature_snapshot.v1",
			"track_waveform_envelopes": []any{row},
		}
	}

	h := NewWithSender(nil, nil, nil) // shadow flag → 物化层接线
	if h.materializeStore == nil {
		t.Fatalf("shadow 态物化库未接线")
	}

	// finalize 世代投影（观察包投影停在旧世代）。
	finalizeObs := mixboard.ObservationPacket{
		ObservationID: "obs_matd4_h1", MixSessionID: "mix_matd4", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary:  map[string]any{"feature_snapshot": snapshotWith(matD4Waveform("sr1", -20.5, -3.2))},
	}
	projection := dom.Build(mixboard.DOMInputFromObservation(finalizeObs, mixboard.Request{}))

	// 尾挂世代（遥测已到达换代）：观察包携带 finalize 投影+新世代 snapshot。
	tailObs := finalizeObs
	tailObs.ObservationID = "obs_matd4_h2"
	tailObs.GlobalSummary = map[string]any{"feature_snapshot": snapshotWith(matD4Waveform("sr2", -19.0, -2.8))}
	tailObs.DOMProjection = &projection

	h.materializeObserveRound(&tailObs)
	m := h.materializeStore.Metrics()
	if m.ShadowTimingCarriedExcluded < 1 {
		t.Fatalf("时序两态分歧应登记 timing_carried 排除计数>=1: got=%d", m.ShadowTimingCarriedExcluded)
	}
	if m.ShadowDivergences != 0 {
		t.Fatalf("timing-carried 行被闸门排除后 ShadowDivergences 应为 0: got=%d", m.ShadowDivergences)
	}

	// 收敛轮：观察投影与尾挂 snapshot 同世代 → 零分歧、排除计数不涨、真比过。
	convergedObs := mixboard.ObservationPacket{
		ObservationID: "obs_matd4_h3", MixSessionID: "mix_matd4", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary:  map[string]any{"feature_snapshot": snapshotWith(matD4Waveform("sr2", -19.0, -2.8))},
	}
	convergedProjection := dom.Build(mixboard.DOMInputFromObservation(convergedObs, mixboard.Request{}))
	convergedObs.DOMProjection = &convergedProjection

	reconcileRowsBefore := m.ReconcileRows
	timingBefore := m.ShadowTimingCarriedExcluded
	h.materializeObserveRound(&convergedObs)
	m = h.materializeStore.Metrics()
	if m.ShadowDivergences != 0 {
		t.Fatalf("收敛轮应零分歧: got=%d", m.ShadowDivergences)
	}
	if m.ShadowTimingCarriedExcluded != timingBefore {
		t.Fatalf("收敛轮排除计数不得上涨（排除计数不涨才叫收敛）: before=%d after=%d", timingBefore, m.ShadowTimingCarriedExcluded)
	}
	if m.ReconcileRows <= reconcileRowsBefore {
		t.Fatalf("收敛行应真比过（非空洞零分歧）: before=%d after=%d", reconcileRowsBefore, m.ReconcileRows)
	}
}

// TestMaterializeShadowFullStateInput：MAT-E0 红先行——真栈取证形态的合成复刻。
// OBS-RECON 定案（W1 两态）：v2 路径下 RequestObservation 返回包的 inline
// feature_snapshot 是剥离态（State B：time_segments 递归删除+evidence_ref 注入），
// 而 DOMProjection 是 finalize 完整态（State A）算的。既有合成用例手工构造
// 观察包、从不真实走 v2 投影路径——State B 分歧因此漏网（G2 ruling"有价值的红"）。
// 本用例以激活的 projectstore 驱动真实 RequestObservation 复现两态，断言：
//  1. 输入面：materializeDepInputs 的 feature snapshot 是完整态（目标轨
//     envelope 行携带 time_segments）；
//  2. 派生面：物化侧 dom 重算的 activity_structure 非 missing（time_segments
//     派生族在场——修前红点：State B 被当作物化输入必 missing）；
//  3. 轮面（S2 微缩）：影子轮零分歧、timing-carried 排除==0、真比过
//     （修前：物化行（State B 重算）≠观察行（State A）→ timing-carried>=1）。
func TestMaterializeShadowFullStateInput(t *testing.T) {
	// v2 环境（真栈取证形态前提）：legacy root 关闭 + projectstore 激活。
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := filepath.Join(root, "song.vit")
	roots, _, err := projectstore.Activate(projectPath, "vitproj_mat_e0")
	if err != nil {
		t.Fatal(err)
	}
	previous, had := os.LookupEnv("VIT_DAW_MATERIALIZATION")
	if err := os.Setenv("VIT_DAW_MATERIALIZATION", "shadow"); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv("VIT_DAW_MATERIALIZATION", previous) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv("VIT_DAW_MATERIALIZATION") })
	}
	h := NewWithSender(nil, nil, nil) // shadow flag → 物化层接线（shadow project 缺省）
	if h.materializeStore == nil {
		t.Fatalf("shadow 态物化库未接线")
	}

	// 真实 v2 观察轮：feature snapshot 经 args 注入（含 time_segments 载荷），
	// RequestObservation 内部 finalize（State A）→ v2 持久化投影（State B）。
	// 轨级行形态对齐真栈 fixture（track_waveform_envelopes+band_energy_summaries
	// 双轴齐备——否则 band 轴空转 MixPackage 测量回退，行被 measurement_carried
	// 排除，到不了 dom 真比过）。
	timeSegments := make([]any, 4)
	for i := range timeSegments {
		energyState := "active"
		if i >= 2 {
			energyState = "low"
		}
		timeSegments[i] = map[string]any{
			"start_seconds": float64(i * 3), "end_seconds": float64((i + 1) * 3),
			"rms_dbfs": -18.0, "peak_dbfs": -4.0, "energy_state": energyState,
		}
	}
	trackWaveform := map[string]any{
		"status": "ready", "track_id": "track_1", "source_revision": "sr1",
		"rms_dbfs": -18.0, "peak_dbfs": -4.0, "headroom_db": -0.4,
		"duration_seconds": 12.0, "sample_rate": 48000.0, "channel_count": 2,
		"analyzed_sample_count": 576000, "window_ms": 200.0, "hop_ms": 100.0,
		"time_segments": timeSegments,
	}
	bandSummary := map[string]any{
		"status": "ready", "track_id": "track_1", "source_revision": "sr1",
		"coverage_ratio": 1.0, "duration_seconds": 12.0,
		"bands": map[string]any{
			"sub":  map[string]any{"status": "ready", "min_hz": 20.0, "max_hz": 60.0, "unit_energy": 0.1},
			"bass": map[string]any{"status": "ready", "min_hz": 60.0, "max_hz": 250.0, "unit_energy": 0.3},
			"mid":  map[string]any{"status": "ready", "min_hz": 250.0, "max_hz": 2000.0, "unit_energy": 0.2},
			"air":  map[string]any{"status": "ready", "min_hz": 8000.0, "max_hz": 20000.0, "unit_energy": 0.05},
		},
	}
	result, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: "mix_mat_e0",
		TargetRef:    mixboard.TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{
			"project_uuid": roots.ProjectUUID, "project_path": projectPath,
			"project_revision": "r1", "duration_seconds": 12.0,
		},
		Args: map[string]any{
			"feature_snapshot": map[string]any{
				"schema_version":           "mixboard_feature_snapshot.v1",
				"track_waveform_envelopes": []any{trackWaveform},
				"band_energy_summaries":    []any{bandSummary},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	obs := result.Observation

	// 取证形态前提锁定（前提不成立=用例失效，红绿结论都不可信）：inline=State B
	//（time_segments 已剥离、evidence_ref 注入）；观察投影=State A（activity
	// 结构非 missing——S2 分歧的观察侧形态）。
	inline, _ := obs.GlobalSummary["feature_snapshot"].(map[string]any)
	if inline == nil {
		t.Fatalf("前提失效：返回包缺 inline feature_snapshot")
	}
	if ref, _ := inline["evidence_ref"].(string); ref == "" {
		t.Fatalf("前提失效：inline feature_snapshot 无 evidence_ref（未走 v2 投影路径？）")
	}
	inlineRows, _ := inline["track_waveform_envelopes"].([]any)
	if len(inlineRows) == 0 {
		t.Fatalf("前提失效：State B 顶层 track_waveform_envelopes 键不在场（与 W1 剥离机制不符）")
	}
	inlineWaveform, _ := inlineRows[0].(map[string]any)
	if _, stripped := inlineWaveform["time_segments"]; stripped {
		t.Fatalf("前提失效：inline envelope 行仍含 time_segments（不是 State B？）")
	}
	if obs.DOMProjection == nil || obs.DOMProjection.ActivityStructure == nil ||
		obs.DOMProjection.ActivityStructure.Status == dom.StatusMissing {
		t.Fatalf("前提失效：观察侧 DOMProjection 缺 activity_structure（State A 形态不成立）")
	}

	// 断言 1（输入面）：物化输入快照必须是完整态——目标轨 envelope 行携带
	// time_segments（State A raw；修前红：deps 吃 State B 瘦身包）。
	deps := h.materializeDepInputs(&obs)
	if deps.FeatureSnapshot == nil {
		t.Fatalf("物化输入缺 feature snapshot")
	}
	fullRows, _ := deps.FeatureSnapshot["track_waveform_envelopes"].([]any)
	if len(fullRows) == 0 {
		t.Fatalf("物化输入快照缺 track_waveform_envelopes 行: %v", deps.FeatureSnapshot)
	}
	fullWaveform, _ := fullRows[0].(map[string]any)
	if segments, ok := fullWaveform["time_segments"].([]any); !ok || len(segments) == 0 {
		t.Fatalf("物化输入快照的 envelope 行缺 time_segments（State B 瘦身包被当作物化输入——OBS-RECON W1 两态缺陷）")
	}

	// 断言 2（派生面）：物化侧 dom 重算的 activity_structure 非 missing
	//（time_segments 派生族在场——真栈取证形态"观察侧 ready、物化侧 missing"）。
	projection := dom.Build(mixboard.DOMInputFromObservation(mixboard.ObservationPacket{
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "track_1"},
		GlobalSummary:  map[string]any{"feature_snapshot": deps.FeatureSnapshot},
		ProjectPackage: map[string]any{"project_revision": deps.ProjectRevision},
	}, mixboard.Request{}))
	if projection.ActivityStructure == nil || projection.ActivityStructure.Status == dom.StatusMissing {
		t.Fatalf("物化输入缺 time_segments 派生字段：activity_structure=missing（State B 瘦身包被当作物化输入）")
	}

	// 断言 3（轮面，S2 微缩）：同源后影子轮零分歧、零 timing 排除、真比过
	//（修前：物化行（State B 重算）≠观察行（State A）→ timing-carried 排除>=1）。
	h.materializeObserveRound(&obs)
	m := h.materializeStore.Metrics()
	if m.ShadowDivergences != 0 {
		t.Fatalf("同源后影子轮应零分歧: got=%d", m.ShadowDivergences)
	}
	if m.ShadowTimingCarriedExcluded != 0 {
		t.Fatalf("同源后 timing-carried 排除应归零: got=%d", m.ShadowTimingCarriedExcluded)
	}
	if m.ReconcileRows == 0 {
		t.Fatalf("零分歧零排除必须来自真比过（非空洞）: metrics=%+v", m)
	}
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
