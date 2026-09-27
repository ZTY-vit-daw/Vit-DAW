package harness

// materialize_onpath_test.go — MAT-E on 态读端切换语义测试（红先行）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §7.1 on 行 / §2.2 observe 读端
// 触发表 / §8 MAT-E 行；QUERY_ENGINE §2.6（actual_cost_class+recomputed 响应
// 字段）。红线：FinalizeObservationContext 原路径零改动——命中装配是它之前的
// 旁路（装配输入就绪后跳过 dom finalize），miss 走原路径。
//
// 三个断言组（对齐卡面门）：
//  1. miss 轮：空库 consult miss → finalize 原路径现算 → 回填（现算产物全量入
//     evidence CAS，行挂 evidence:// 句柄 upsert）→ 响应字段 compile/1；
//  2. 命中轮：同输入第二次 observe → consult 命中 current 行集 → 装配跳过 dom
//     finalize（服务的是回填世代实例——GeneratedAt 是 miss 轮的时间戳，非本轮）
//     → 响应字段 index/0；命中轮后影子轮重算不得冲掉行的 CAS 句柄；
//  3. 回退演练：on→shadow→off 三态——shadow/off 读端不消费（走原路径、响应无
//     物化字段），dom 投影内容与现状同源（ProjectionID 内容稳定相等）。

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/materialize"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
)

// matEFixtureObservation 构造一轮真实 v2 RequestObservation（对齐
// TestMaterializeShadowFullStateInput 的输入形态：轨级行双轴齐备、args 携带
// feature snapshot）。mode 控制 consult 注入；freshSource 每轮新建（consult
// 结果是 per-request 态）。
func matEObservationRequest(t *testing.T, sessionID string, src mixboard.MaterializedDOMSource) mixboard.WriteResult {
	t.Helper()
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
	roots, ok := projectstore.Current()
	if !ok {
		t.Fatalf("前提失效：projectstore 未激活")
	}
	req := mixboard.Request{
		MixSessionID: sessionID,
		TargetRef:    mixboard.TargetRef{Kind: "track", ID: "track_1"},
		ProjectState: map[string]any{
			"project_uuid": roots.ProjectUUID, "project_revision": "r1", "duration_seconds": 12.0,
		},
		Args: map[string]any{
			"feature_snapshot": map[string]any{
				"schema_version":           "mixboard_feature_snapshot.v1",
				"track_waveform_envelopes": []any{trackWaveform},
				"band_energy_summaries":    []any{bandSummary},
			},
		},
	}
	if src != nil {
		req.MaterializedDOM = src
	}
	result, err := mixboard.NewStore("").RequestObservation(req)
	if err != nil {
		t.Fatalf("RequestObservation 失败: %s", err)
	}
	if result.Observation.DOMProjection == nil {
		t.Fatalf("前提失效：观察包缺 DOMProjection")
	}
	return result
}

// matESetupOn 激活 v2 projectstore 并构造 on 态 harness。
func matESetupOn(t *testing.T) *Harness {
	t.Helper()
	t.Setenv("VIT_MIXBOARD_ROOT", "")
	projectstore.Deactivate()
	t.Cleanup(projectstore.Deactivate)
	root := t.TempDir()
	projectPath := root + "/song.vit"
	if _, _, err := projectstore.Activate(projectPath, "vitproj_mat_e"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VIT_DAW_MATERIALIZATION", "on")
	h := NewWithSender(nil, nil, nil)
	if h.materializeStore == nil {
		t.Fatalf("on 态物化库未接线")
	}
	if h.materializeMode != materialize.ModeOn {
		t.Fatalf("on 态 mode 字段未记录: %q", h.materializeMode)
	}
	return h
}

// TestMaterializeOnReadMissBackfillsAndServeFields：miss 轮语义——空库 consult
// miss → 原路径 finalize（响应 compile/1）→ 回填把现算产物全量入 evidence CAS
// 并挂句柄 upsert（CurrentPrecomputableRow 可见 current+evidence:// 句柄）。
func TestMaterializeOnReadMissBackfillsAndServeFields(t *testing.T) {
	h := matESetupOn(t)

	consult := h.materializeDOMSource()
	if consult == nil {
		t.Fatalf("on 态读端 consult 未装配")
	}
	result := matEObservationRequest(t, "mix_mat_e_miss", consult)

	// 响应字段：miss → compile + recomputed=1（QUERY_ENGINE §2.6）。
	costClass, recomputed, visible := h.materializeOnResponseFields(consult)
	if !visible {
		t.Fatalf("on 态响应字段不可见")
	}
	if costClass != "compile" || recomputed != 1 {
		t.Fatalf("miss 轮响应字段应 compile/1: got %s/%d", costClass, recomputed)
	}

	// 读端收尾（回填）后：目标轨 dom 行 current 且携带 evidence 句柄。
	h.materializeOnPathTail(&result.Observation, consult)
	row, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1")
	if !ok {
		t.Fatalf("miss 轮回填后应存在 dom 行")
	}
	if !strings.HasPrefix(row.Handle, "evidence://") {
		t.Fatalf("回填行应携带 evidence:// 句柄（命中装配的内容层）: %q", row.Handle)
	}
	// consult 计数：misses>=1。
	if m := h.materializeStore.Metrics(); m.PerKind["dom"].Misses < 1 {
		t.Fatalf("consult miss 未计数: %+v", m.PerKind["dom"])
	}
}

// TestMaterializeOnReadHitAssemblesFromMaterializedRow：命中轮语义——同输入第
// 二次 observe 命中 current 行集 → 装配跳过 dom finalize（服务实例的
// GeneratedAt 是 miss 轮回填世代的时间戳，非本轮——finalize 若重跑必换代）
// → 响应 index/0；命中轮零回填；随后影子轮重算不冲掉行的 CAS 句柄。
func TestMaterializeOnReadHitAssemblesFromMaterializedRow(t *testing.T) {
	h := matESetupOn(t)

	// 轮 1（miss+回填）。
	consult1 := h.materializeDOMSource()
	result1 := matEObservationRequest(t, "mix_mat_e_hit", consult1)
	h.materializeOnPathTail(&result1.Observation, consult1)
	backfilledGeneratedAt := result1.Observation.DOMProjection.GeneratedAt
	if strings.TrimSpace(backfilledGeneratedAt) == "" {
		t.Fatalf("前提失效：回填世代投影缺 GeneratedAt")
	}

	// 轮 2（命中装配）：服务实例 == 回填世代（GeneratedAt 不换代=finalize 被跳过）。
	consult2 := h.materializeDOMSource()
	result2 := matEObservationRequest(t, "mix_mat_e_hit", consult2)
	if got := result2.Observation.DOMProjection.GeneratedAt; got != backfilledGeneratedAt {
		t.Fatalf("命中轮应装配回填世代实例（跳过 dom finalize）: served=%q fresh_finalize_would_be_newer=%q", got, backfilledGeneratedAt)
	}
	if result2.Observation.DOMProjection.ProjectionID != result1.Observation.DOMProjection.ProjectionID {
		t.Fatalf("命中轮服务实例的内容身份应与回填世代一致（ProjectionID 内容稳定）")
	}
	costClass, recomputed, visible := h.materializeOnResponseFields(consult2)
	if !visible || costClass != "index" || recomputed != 0 {
		t.Fatalf("命中轮响应字段应 index/0: got %s/%d visible=%t", costClass, recomputed, visible)
	}
	rowBefore, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1")
	if !ok {
		t.Fatalf("命中轮前应存在 dom 行")
	}

	// 命中轮收尾：零回填（行句柄不换新——内容已在库）。
	h.materializeOnPathTail(&result2.Observation, consult2)
	rowAfter, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1")
	if !ok {
		t.Fatalf("命中轮后应存在 dom 行")
	}
	if rowAfter.Handle != rowBefore.Handle {
		t.Fatalf("命中轮零回填：行句柄不得换新 before=%s after=%s", rowBefore.Handle, rowAfter.Handle)
	}

	// 影子轮（on 态同 shadow 行为）重算不得冲掉 CAS 句柄——内容同源行保留句柄。
	h.materializeObserveRound(&result2.Observation)
	rowShadow, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1")
	if !ok {
		t.Fatalf("影子轮后应存在 dom 行")
	}
	if rowShadow.Handle != rowBefore.Handle {
		t.Fatalf("影子轮重算冲掉了读端回填句柄（同内容行应保留）: before=%s after=%s", rowBefore.Handle, rowShadow.Handle)
	}
	// consult 计数：hits>=1。
	if m := h.materializeStore.Metrics(); m.PerKind["dom"].Hits < 1 {
		t.Fatalf("consult hit 未计数: %+v", m.PerKind["dom"])
	}
}

// TestMaterializeOnReadStaleMissesAfterInvalidation：失效后命中退化为 miss——
// 变更事件把行标 stale → consult 如实 miss（响应 compile/1）→ 回填恢复 current。
func TestMaterializeOnReadStaleMissesAfterInvalidation(t *testing.T) {
	h := matESetupOn(t)

	consult1 := h.materializeDOMSource()
	result1 := matEObservationRequest(t, "mix_mat_e_stale", consult1)
	h.materializeOnPathTail(&result1.Observation, consult1)
	if _, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1"); !ok {
		t.Fatalf("回填后应存在 current dom 行")
	}

	// 注入失效（表 B：track.level → dom）。
	if _, err := h.materializeStore.MarkStale(materialize.RowMatch{Kinds: []string{"dom"}, ScopeKind: "track", ScopeValues: []string{"track_1"}}, "chg_mat_e_1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1"); ok {
		t.Fatalf("标脏后行不应再按 current 命中")
	}

	consult2 := h.materializeDOMSource()
	result2 := matEObservationRequest(t, "mix_mat_e_stale", consult2)
	if got, want := result2.Observation.DOMProjection.GeneratedAt, result1.Observation.DOMProjection.GeneratedAt; got == want {
		t.Fatalf("stale 轮应走原路径重算（GeneratedAt 应换代）")
	}
	costClass, recomputed, visible := h.materializeOnResponseFields(consult2)
	if !visible || costClass != "compile" || recomputed != 1 {
		t.Fatalf("stale 轮响应字段应 compile/1: got %s/%d visible=%t", costClass, recomputed, visible)
	}
	h.materializeOnPathTail(&result2.Observation, consult2)
	if _, ok := h.materializeStore.CurrentPrecomputableRow("dom", "track", "track_1"); !ok {
		t.Fatalf("stale 轮回填后应恢复 current dom 行")
	}
}

// TestMaterializeOnReadSwitchRollbackDrill：回退演练（单测面）——on（命中装配
// 发生）→ shadow（读端不消费：原路径、响应字段不可见）→ off（零接线现状）。
// shadow/off 两态 dom 投影内容同源（ProjectionID 内容稳定相等）；off 态
// materializeStore 为 nil。
func TestMaterializeOnReadSwitchRollbackDrill(t *testing.T) {
	// —— on 段：命中装配确实发生。 ——
	hOn := matESetupOn(t)
	consult1 := hOn.materializeDOMSource()
	result1 := matEObservationRequest(t, "mix_mat_e_drill", consult1)
	hOn.materializeOnPathTail(&result1.Observation, consult1)
	backfilledGeneratedAt := result1.Observation.DOMProjection.GeneratedAt
	consult2 := hOn.materializeDOMSource()
	result2 := matEObservationRequest(t, "mix_mat_e_drill", consult2)
	hOn.materializeOnPathTail(&result2.Observation, consult2)
	if result2.Observation.DOMProjection.GeneratedAt != backfilledGeneratedAt {
		t.Fatalf("on 段命中装配未发生（演练前提）")
	}

	// —— shadow 段：读端不消费（consult 未装配、响应字段不可见、原路径现算）。 ——
	t.Setenv("VIT_DAW_MATERIALIZATION", "shadow")
	hShadow := NewWithSender(nil, nil, nil)
	if hShadow.materializeStore == nil {
		t.Fatalf("shadow 态物化库未接线（物化层行为应同 on）")
	}
	if src := hShadow.materializeDOMSource(); src != nil {
		t.Fatalf("shadow 态读端 consult 不应装配（读端不消费是 shadow 承诺）")
	}
	shadowResult := matEObservationRequest(t, "mix_mat_e_drill", nil)
	if got := shadowResult.Observation.DOMProjection.GeneratedAt; got == backfilledGeneratedAt {
		t.Fatalf("shadow 段应走原路径现算（GeneratedAt 应是本轮，非 on 段回填世代）")
	}
	if costClass, recomputed, visible := hShadow.materializeOnResponseFields(nil); visible {
		t.Fatalf("shadow 态响应字段不应可见: %s/%d", costClass, recomputed)
	}

	// —— off 段：零接线现状（无物化库、无 consult、原路径现算）。 ——
	t.Setenv("VIT_DAW_MATERIALIZATION", "off")
	hOff := NewWithSender(nil, nil, nil)
	if hOff.materializeStore != nil {
		t.Fatalf("off 态物化库不得接线（现状逐字节一致）")
	}
	if src := hOff.materializeDOMSource(); src != nil {
		t.Fatalf("off 态读端 consult 不应装配")
	}
	offResult := matEObservationRequest(t, "mix_mat_e_drill", nil)

	// 回退后行为=现状：shadow 与 off 两态的 dom 投影内容身份一致——用物化层
	// 内容身份（contentIdentityHash，volatile 剥离——影子对账同口径）比较；
	// stableProjectionID 不剥离 observation_id（逐观察随机），不能用作跨观察
	// 比较。on 段命中服务的实例同内容身份（material_reuse 的内容等价性）。
	matEDomRowHash := func(projection dom.Projection) string {
		row, ok := materialize.DOMRowFromProjection("track_1", projection)
		if !ok {
			t.Fatalf("dom 投影行化失败")
		}
		return row.Ref.Hash
	}
	shadowHash := matEDomRowHash(*shadowResult.Observation.DOMProjection)
	offHash := matEDomRowHash(*offResult.Observation.DOMProjection)
	servedHash := matEDomRowHash(*result2.Observation.DOMProjection)
	if shadowHash != offHash {
		t.Fatalf("回退后 shadow/off 两态 dom 投影内容身份应一致（现状口径）: shadow=%s off=%s", shadowHash, offHash)
	}
	if servedHash != offHash {
		t.Fatalf("on 段命中服务实例的内容身份应与现状一致（material_reuse 内容等价）: served=%s off=%s", servedHash, offHash)
	}
	if offResult.Observation.DOMProjection.GeneratedAt == backfilledGeneratedAt {
		t.Fatalf("off 段应走原路径现算")
	}
}
