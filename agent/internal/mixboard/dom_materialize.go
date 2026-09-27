package mixboard

// dom_materialize.go — MAT-C 物化层 dom 适配器的装配导出（零行为新增文件）。
//
// 物化层 dom 适配器（agent/internal/materialize/adapters.go DOMAdapter）与观察
// 路径共享同一 dom 输入装配（domInputFromObservation 原函数零改动）：影子对账
// （MATERIALIZATION §7.1）两侧同口径是 ShadowDivergences==0 可解释的前提——
// 分歧只应来自输入域或传播，不来自装配分叉。

import "vit-daw-agent/internal/dom"

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
