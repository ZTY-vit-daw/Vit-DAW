package mixboard

// dom_materialize.go — MAT-C 物化层 dom 适配器的装配导出（零行为新增文件）。
//
// 物化层 dom 适配器（agent/internal/materialize/adapters.go DOMAdapter）与观察
// 路径共享同一 dom 输入装配（domInputFromObservation 原函数零改动）：影子对账
//（MATERIALIZATION §7.1）两侧同口径是 ShadowDivergences==0 可解释的前提——
// 分歧只应来自输入域或传播，不来自装配分叉。

import (
	"encoding/json"
	"strings"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/projectstore"
)

// DOMInputFromObservation 导出观察路径的 dom 输入装配（per-target）。适配器以
// 合成 ObservationPacket（TargetRef=轨、GlobalSummary 携带 feature snapshot、
// ProjectPackage 携带 revision）调用，走与 FinalizeObservationContext→
// finalizeDOMProjection 完全相同的装配链。
func DOMInputFromObservation(obs ObservationPacket, req Request) dom.Input {
	return domInputFromObservation(obs, req)
}

// DOMMeasurementFallbackActive 报告该观察包的 dom 输入装配是否经 MixPackage
// 测量回退轴激活（MAT-D 取证形态：snapshot 目标键空且 metrics 命中——最终
// waveform/bandSummary 至少一支取自 MixPackage.current_metrics）。
//
// 只读探针（MAT-D2 登记型判定依据面）：与 domInputFromObservation 共用同一
// 回退链与目标匹配谓词，逐支复演判定最终输入源；不构造 dom.Input、零行为。
// 物化侧 DepInputs 不携带 MixPackage 测量——回退轴激活即两侧不同源，该轮
// dom 行不可由物化侧重算复现（non_precomputable）。
func DOMMeasurementFallbackActive(obs ObservationPacket) bool {
	snapshot := mapValue(obs.GlobalSummary["feature_snapshot"])
	metrics := mapValue(obs.MixPackage["current_metrics"])

	waveform := mapValue(snapshot["waveform_envelope"])
	waveformFromMetrics := false
	if len(waveform) == 0 {
		waveform = mapValue(metrics["waveform"])
		waveformFromMetrics = len(waveform) > 0
	}
	bandSummary := mapValue(snapshot["band_energy_summary"])
	bandFromMetrics := false
	if len(bandSummary) == 0 {
		bandSummary = mapValue(metrics["band_energy"])
		bandFromMetrics = len(bandSummary) > 0
	}
	if targetNeedsMatchingRow(obs.TargetRef) {
		if !featureRowMatchesTarget(waveform, obs.TargetRef) {
			waveform = featureRowForTarget(snapshot["track_waveform_envelopes"], obs.TargetRef)
			waveformFromMetrics = false
		}
		if !featureRowMatchesTarget(bandSummary, obs.TargetRef) {
			bandSummary = featureRowForTarget(snapshot["band_energy_summaries"], obs.TargetRef)
			bandFromMetrics = false
		}
	}
	if len(waveform) == 0 {
		waveform = mapValue(metrics["waveform"])
		waveformFromMetrics = len(waveform) > 0
	}
	if len(bandSummary) == 0 {
		bandSummary = mapValue(metrics["band_energy"])
		bandFromMetrics = len(bandSummary) > 0
	}
	return waveformFromMetrics || bandFromMetrics
}

// FeatureSnapshotEvidence 回溯观察包证据链取完整态 feature snapshot（MAT-E0：
// 物化读端的输入源切换——v2 持久化投影把返回包 inline snapshot 替换为剥离态
// State B，完整态 State A raw 由 PutEvidence 存进证据 blob；本函数与
// lazyFeatureSnapshot（mix_read `.raw.` 键回溯，catalog.go）同一判定链）。
// 只读，不改观察路径行为。返回 (snapshot, applicable)：
//
//	applicable=false：v2 持久化不适用于该包（无激活 store / 工程身份不匹配）——
//	  legacy/dev 路径的返回包就是 finalize 原对象，inline 即完整态；
//	applicable=true & snapshot=nil：v2 语义下证据回溯失败（evidence 关闭/
//	  blob 预算清理/读取失败）——inline 是剥离态，调用方回退 inline 时应登记
//	  缺口（物化输入降级，不静默）。
func FeatureSnapshotEvidence(obs ObservationPacket) (snapshot map[string]any, applicable bool) {
	roots, ok := projectstore.Current()
	if !ok || obs.ProjectUUID == "" || !strings.EqualFold(projectstore.SafeName(obs.ProjectUUID), roots.ProjectUUID) {
		return nil, false
	}
	for _, ref := range obs.EvidenceRefs {
		if !strings.HasPrefix(strings.TrimSpace(ref), "evidence://") {
			continue
		}
		blob, err := projectstore.GetEvidence(roots, ref)
		if err != nil || blob.Kind != "feature_snapshot" {
			continue
		}
		if content, ok := blob.Content.(map[string]any); ok {
			return content, true
		}
		data, _ := json.Marshal(blob.Content)
		var parsed map[string]any
		if json.Unmarshal(data, &parsed) == nil {
			return parsed, true
		}
	}
	return nil, true
}
