package materialize

// timing_carried_test.go — MAT-D4 红先行（timing-carried 登记型处置）。
//
// 规格：card 2026-09-29-MAT-D4 + MAT-D3 取证/验收裁定②——真栈 S2 剩余分歧=
// 观察投影（finalize 世代输入）与物化行（尾挂世代输入）的时序两态；G2 闭环
// 口径=收敛态（非瞬时）。登记语义（与 measurement-carried 同款：排除+计数
// 单列，不静默）：
//
//	对账时（ReconcileShadowWithTiming，deps 非空）对会判分歧的 dom 行做当前
//	输入同路径重放（BuildDOMRowForTrack，与 DOMAdapter.Build 严格同链）——
//	重放==物化行 而观察行异 = 两侧输入世代差（timing-carried）→ 排除+单列
//	计数 Metrics.ShadowTimingCarriedExcluded、不计分歧、不进 ReconcileRows；
//	重放缺行或与物化行不符 = 判定不可靠 → 宁计分歧不误排除（宁少勿滥——
//	对账口径不是失效传播）。
//
// 合成复刻形态（MAT-D3 取证 §3.3 三角重放的合成版）：finalize 世代观察 →
// 遥测到达换代（applyWaveformArrival）→ 尾挂世代重算物化 → 对账。

import (
	"testing"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/mixboard"
)

// matD4ObservationFromDeps 用一份 DepInputs 世代构造观察包（观察侧投影=
// dom.Build(DOMInputFromObservation)，与物化适配器同链不同时点）。label 与
// deps.ProjectState 轨行同源（MAT-D3 口径）。
func matD4ObservationFromDeps(deps DepInputs, observationID string) mixboard.ObservationPacket {
	obs := mixboard.ObservationPacket{
		ObservationID: observationID, MixSessionID: "mix_matd4", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3", Label: "lead vox"},
		ProjectPackage: map[string]any{"project_revision": deps.ProjectRevision},
		GlobalSummary:  map[string]any{"feature_snapshot": deps.FeatureSnapshot},
	}
	projection := dom.Build(mixboard.DOMInputFromObservation(obs, mixboard.Request{}))
	obs.DOMProjection = &projection
	return obs
}

// matD4TimingFixture：finalize 世代观察行 + 尾挂世代物化库（时序两态的合成
// 复刻）。返回 fresh（finalize 投影行）、尾挂 deps 与已重算的 store。
func matD4TimingFixture(t *testing.T) (Row, DepInputs, *Store) {
	t.Helper()
	project := newMatCSynthProject()
	depsFinalize := project.DepInputs()
	obs := matD4ObservationFromDeps(depsFinalize, "obs_matd4_0001")
	fresh, ok := DOMRowFromObservation("T3", &obs)
	if !ok {
		t.Fatalf("DOMRowFromObservation 未产行（finalize 世代观察）")
	}
	// finalize 后遥测到达：输入换代（观察投影仍是 finalize 世代=时序两态）。
	project.applyWaveformArrival("T3", "req_matd4_1")
	depsTail := project.DepInputs()

	store := NewStore()
	if err := RegisterDefaultAdapters(store, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := store.RecomputeLazy(depsTail); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	return fresh, depsTail, store
}

// TestShadowTimingCarriedExcludedOnGenerationGap：世代差分歧 → 排除+单列计数，
// 不计分歧、不进 ReconcileRows（登记可见，不静默）。
func TestShadowTimingCarriedExcludedOnGenerationGap(t *testing.T) {
	fresh, depsTail, store := matD4TimingFixture(t)

	res := store.ReconcileShadowWithTiming([]Row{fresh}, &depsTail)
	if res.Divergences != 0 {
		t.Fatalf("世代差分歧应被 timing 闸门排除: divergences=%d details=%v", res.Divergences, res.Details)
	}
	if res.TimingCarried != 1 {
		t.Fatalf("TimingCarried 应为 1: got=%d", res.TimingCarried)
	}
	m := store.Metrics()
	if m.ShadowTimingCarriedExcluded != 1 {
		t.Fatalf("Metrics.ShadowTimingCarriedExcluded 应为 1: got=%d", m.ShadowTimingCarriedExcluded)
	}
	if m.ShadowDivergences != 0 {
		t.Fatalf("排除行不得计入 ShadowDivergences: got=%d", m.ShadowDivergences)
	}
	if m.ReconcileRows != 0 {
		t.Fatalf("排除行不应进 ReconcileRows（计数单列）: got=%d", m.ReconcileRows)
	}
}

// TestShadowTimingUnreliableCountsAsDivergence：判定不可靠两形态——(a) 对账时
// 输入又前进（重放≠物化行）；(b) 未提供 deps（旧口径）——都宁计分歧不误排除。
func TestShadowTimingUnreliableCountsAsDivergence(t *testing.T) {
	fresh, _, store := matD4TimingFixture(t)

	// (a) 第三世代输入：重放产物≠物化行（物化行落在旧世代）→ 无法归因 timing。
	project := newMatCSynthProject()
	project.applyWaveformArrival("T3", "req_matd4_2")
	project.features["T3"] = matCWaveformRow("T3", "sr3", -18.0, -2.5)
	depsNewer := project.DepInputs()
	res := store.ReconcileShadowWithTiming([]Row{fresh}, &depsNewer)
	if res.Divergences != 1 {
		t.Fatalf("重放≠物化行时应计分歧（宁计分歧不误排除）: divergences=%d", res.Divergences)
	}
	if res.TimingCarried != 0 {
		t.Fatalf("判定不可靠不得排除: TimingCarried=%d", res.TimingCarried)
	}

	// (b) deps 缺省：不参与 timing 判定，旧口径分歧照计。
	fresh2, _, store2 := matD4TimingFixture(t)
	res2 := store2.ReconcileShadowWithTiming([]Row{fresh2}, nil)
	if res2.Divergences != 1 {
		t.Fatalf("nil deps 应保持旧口径计分歧: divergences=%d", res2.Divergences)
	}
	if res2.TimingCarried != 0 {
		t.Fatalf("nil deps 不得做 timing 排除: TimingCarried=%d", res2.TimingCarried)
	}
	if m := store2.Metrics(); m.ShadowTimingCarriedExcluded != 0 {
		t.Fatalf("排除计数应为 0: got=%d", m.ShadowTimingCarriedExcluded)
	}
}

// TestShadowConvergenceRoundZeroDivergence：收敛轮——两侧同世代输入 → 真比过
// （ReconcileRows+1）、零分歧、零排除（收敛态= G2-D 闭环口径，排除计数不涨）。
func TestShadowConvergenceRoundZeroDivergence(t *testing.T) {
	_, depsTail, store := matD4TimingFixture(t)

	obsConv := matD4ObservationFromDeps(depsTail, "obs_matd4_0002")
	freshConv, ok := DOMRowFromObservation("T3", &obsConv)
	if !ok {
		t.Fatalf("DOMRowFromObservation 未产行（收敛轮观察）")
	}
	res := store.ReconcileShadowWithTiming([]Row{freshConv}, &depsTail)
	if res.Divergences != 0 {
		t.Fatalf("收敛轮应零分歧: divergences=%d details=%v", res.Divergences, res.Details)
	}
	if res.TimingCarried != 0 {
		t.Fatalf("收敛轮不应产生 timing 排除（排除计数不涨才叫收敛）: TimingCarried=%d", res.TimingCarried)
	}
	m := store.Metrics()
	if m.ReconcileRows != 1 {
		t.Fatalf("收敛行应真比过（非空洞零分歧）: ReconcileRows=%d", m.ReconcileRows)
	}
	if m.ShadowDivergences != 0 || m.ShadowTimingCarriedExcluded != 0 {
		t.Fatalf("收敛轮后累计分歧/排除应为 0: div=%d timing=%d", m.ShadowDivergences, m.ShadowTimingCarriedExcluded)
	}
}

// TestShadowTimingStoredMissingCountsAsDivergence：物化侧无行（stored-missing）
// 不做 timing 归因——两态未证，宁计分歧（漏标检测面不得被闸门吞掉）。
func TestShadowTimingStoredMissingCountsAsDivergence(t *testing.T) {
	fresh, depsTail, _ := matD4TimingFixture(t)

	emptyStore := NewStore()
	if err := RegisterDefaultAdapters(emptyStore, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	res := emptyStore.ReconcileShadowWithTiming([]Row{fresh}, &depsTail)
	if res.Divergences != 1 {
		t.Fatalf("stored-missing 应计分歧: divergences=%d", res.Divergences)
	}
	if res.TimingCarried != 0 {
		t.Fatalf("stored-missing 不得 timing 排除: TimingCarried=%d", res.TimingCarried)
	}
}
