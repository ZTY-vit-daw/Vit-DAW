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
