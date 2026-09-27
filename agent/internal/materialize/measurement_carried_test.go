package materialize

// measurement_carried_test.go — MAT-D2 红先行（登记型处置，方案②零行为变化）。
//
// 规格：card 2026-09-29-MAT-D2 + MAT-D 验收裁定②——S2 红=dom 行经
// MixPackage.current_metrics 回退轴激活时与物化侧不同源（DepInputs 不携带
// MixPackage 测量）。登记语义：
//
//	回退轴激活形态（snapshot 目标键空且 metrics 命中，判定与观察路径装配
//	domInputFromObservation 同链）→ 该行标记 non_precomputable+measurement_carried
//	注记（freshness/limitations 语义面）；ZeroDivergence 对账闸门排除该形态行
//	（对账跳过+计数单列 Metrics.ShadowMeasurementCarriedExcluded——不静默、
//	不计分歧、不进 ReconcileRows）。
//
// MAT-C 合成盲区声明：既有合成观察 MixPackage 恒空，回退轴在测试面永不激活
// ——本文件补带测量的用例（真栈 S2 红形态的合成复刻）。

import (
	"testing"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/mixboard"
)

// matD2MeasurementObservation 构造一个 MixPackage 回退激活轮的观察包：目标
// track:T3；feature snapshot 无 T3 目标键行（顶层 waveform_envelope 与
// track_waveform_envelopes 皆不含目标行）；MixPackage.current_metrics.waveform
// 命中（带 track_id 的实时测量行）。这是 MAT-D 取证的真栈分歧形态。
func matD2MeasurementObservation() mixboard.ObservationPacket {
	obs := mixboard.ObservationPacket{
		ObservationID: "obs_matd2_0001", MixSessionID: "mix_matd2", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary: map[string]any{"feature_snapshot": map[string]any{
			"schema_version": "mixboard_feature_snapshot.v1",
			// 无 track_waveform_envelopes、无顶层 waveform_envelope——snapshot 目标键空。
		}},
		MixPackage: map[string]any{"current_metrics": map[string]any{
			"waveform": matCWaveformRow("T3", "sr_live", -18.0, -4.0),
		}},
	}
	// 观察路径同链装配出 DOMProjection（FinalizeObservationContext 的 dom 半边
	// 与 DOMInputFromObservation 共享 domInputFromObservation——同口径前提）。
	projection := dom.Build(mixboard.DOMInputFromObservation(obs, mixboard.Request{}))
	obs.DOMProjection = &projection
	return obs
}

// TestDOMRowFromObservationRegistersMeasurementCarried：回退激活轮 → 行带
// non_precomputable+measurement_carried 注记；对账闸门排除该行（不计分歧、
// 计数单列 ShadowMeasurementCarriedExcluded、不进 ReconcileRows）。
func TestDOMRowFromObservationRegistersMeasurementCarried(t *testing.T) {
	obs := matD2MeasurementObservation()

	row, ok := DOMRowFromObservation("T3", &obs)
	if !ok {
		t.Fatalf("DOMRowFromObservation 未产行（projection status=%q）", obs.DOMProjection.Status)
	}
	if !IsMeasurementCarried(row) {
		t.Fatalf("回退激活轮的行应带 measurement_carried 注记: payload=%v", row.Payload)
	}
	if row.Payload[PayloadKeyNonPrecomputable] != true {
		t.Fatalf("行应带 non_precomputable 注记: payload=%v", row.Payload)
	}
	if row.Payload[PayloadKeyMeasurementCarried] != true {
		t.Fatalf("行应带 measurement_carried 注记: payload=%v", row.Payload)
	}

	// 物化侧：同 snapshot（无 T3 波形行）重算——dom/T3 无行（真栈 S2 红的
	// stored-missing 形态）；即使有行（band-only）hash 也必不等。两种形态都
	// 必须被闸门排除而不是计分歧。
	store := NewStore()
	if err := RegisterDefaultAdapters(store, []string{"dom"}); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	snapshot, _ := obs.GlobalSummary["feature_snapshot"].(map[string]any)
	if err := store.RecomputeLazy(DepInputs{ProjectRevision: "r1", FeatureSnapshot: snapshot}); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}

	diverged, details := store.ReconcileShadowDetailed([]Row{row})
	if diverged != 0 {
		t.Fatalf("measurement_carried 行应被闸门排除而非计分歧: diverged=%d details=%v", diverged, details)
	}
	m := store.Metrics()
	if m.ShadowMeasurementCarriedExcluded != 1 {
		t.Fatalf("Metrics.ShadowMeasurementCarriedExcluded 应为 1: got=%d", m.ShadowMeasurementCarriedExcluded)
	}
	if m.ShadowDivergences != 0 {
		t.Fatalf("排除行不得计入 ShadowDivergences: got=%d", m.ShadowDivergences)
	}
	if m.ReconcileRows != 0 {
		t.Fatalf("排除行不应进 ReconcileRows（计数单列）: got=%d", m.ReconcileRows)
	}
}

// TestDOMRowFromObservationExcludesMixedBatch：混合批——一行 measurement_carried
// + 一行正常同源行：排除 1、比过 1、分歧 0（闸门排除不吞掉正常对账面）。
func TestDOMRowFromObservationExcludesMixedBatch(t *testing.T) {
	project := newMatCSynthProject()
	store := NewStore()
	if err := RegisterDefaultAdapters(store, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := store.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}

	normal := mustDOMRowForSynth(t, project, "T3") // 物化同源同坐标行：零分歧
	measured := MarkMeasurementCarried(normal)     // 同坐标但带测量注记（排除形态）

	diverged, details := store.ReconcileShadowDetailed([]Row{measured, normal})
	if diverged != 0 {
		t.Fatalf("混合批不应计分歧: diverged=%d details=%v", diverged, details)
	}
	m := store.Metrics()
	if m.ShadowMeasurementCarriedExcluded != 1 {
		t.Fatalf("排除计数应为 1: got=%d", m.ShadowMeasurementCarriedExcluded)
	}
	if m.ReconcileRows != 1 {
		t.Fatalf("正常行比对数应为 1（排除行单列不计入）: got=%d", m.ReconcileRows)
	}
	if m.ShadowDivergences != 0 {
		t.Fatalf("ShadowDivergences 应为 0: got=%d", m.ShadowDivergences)
	}
}

// TestDOMRowFromObservationSnapshotSourcedNotMarked：反例——snapshot 目标键
// 命中时 MixPackage 测量在场也不注记（回退轴未激活：装配链 snapshot 优先），
// 正常对账零分歧、排除计数为零。登记判定不得过触发（真栈零分歧面不得被
// 闸门静默吞掉）。
func TestDOMRowFromObservationSnapshotSourcedNotMarked(t *testing.T) {
	snapshotRow := matCWaveformRow("T3", "sr1", -20.5, -3.2)
	obs := mixboard.ObservationPacket{
		ObservationID: "obs_matd2_0002", MixSessionID: "mix_matd2", Status: "ready",
		TargetRef:      mixboard.TargetRef{Kind: "track", ID: "T3"},
		ProjectPackage: map[string]any{"project_revision": "r1"},
		GlobalSummary: map[string]any{"feature_snapshot": map[string]any{
			"schema_version":            "mixboard_feature_snapshot.v1",
			"waveform_envelope":         snapshotRow, // 顶层命中目标（track_id=T3）
			"track_waveform_envelopes":  []any{snapshotRow},
		}},
		MixPackage: map[string]any{"current_metrics": map[string]any{
			"waveform": matCWaveformRow("T3", "sr_live", -18.0, -4.0), // 在场但未被消费
		}},
	}
	projection := dom.Build(mixboard.DOMInputFromObservation(obs, mixboard.Request{}))
	obs.DOMProjection = &projection

	row, ok := DOMRowFromObservation("T3", &obs)
	if !ok {
		t.Fatalf("DOMRowFromObservation 未产行")
	}
	if IsMeasurementCarried(row) {
		t.Fatalf("snapshot 目标键命中的行不得被注记（回退轴未激活）: payload=%v", row.Payload)
	}

	project := newMatCSynthProject()
	store := NewStore()
	if err := RegisterDefaultAdapters(store, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	if err := store.RecomputeLazy(project.DepInputs()); err != nil {
		t.Fatalf("RecomputeLazy: %v", err)
	}
	if diverged := store.ReconcileShadow([]Row{row}); diverged != 0 {
		t.Fatalf("snapshot 同源行对账应零分歧: got=%d", diverged)
	}
	m := store.Metrics()
	if m.ShadowMeasurementCarriedExcluded != 0 {
		t.Fatalf("排除计数应为 0: got=%d", m.ShadowMeasurementCarriedExcluded)
	}
	if m.ReconcileRows != 1 {
		t.Fatalf("正常行应进比对行数: got=%d", m.ReconcileRows)
	}
}
