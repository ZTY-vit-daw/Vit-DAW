package mixboard

// finalize_materialized.go — MAT-E on 态读端旁路（docs/MATERIALIZATION_V1_DESIGN.md
// §7.1 on 行 / §2.2 observe 读端触发表）。
//
// 红线：FinalizeObservationContext 原路径零改动（回退路径的设计核心）——命中
// 装配发生在它之前（RequestObservation 装配输入就绪后、finalize 调用点旁路：
// 预置 obs.DOMProjection 后走本文件的跳 dom 变体），miss 走原路径。off/shadow
// 态（Request.MaterializedDOM nil）与 paired 模式、band-stereo 请求的调用形态
// 与现状逐字节一致。

import (
	"strings"
	"time"

	"vit-daw-agent/internal/dom"
)

// MaterializedDOMSource 是 on 态读端的物化 consult 契约（消费侧接口；nil=原
// 路径）。harness 以物化库+evidence CAS 实现；MaterializedDOM 返回目标轨的
// 物化 dom 投影，ok=false=miss/stale（调用方走原路径现算）。domMode 是本请求
// 的有效 dom 模式（effectiveDOMMode 同口径）——F9：paired/change_delta 类请求
// 携带测量不预计算，实现方按模式自行 miss。
type MaterializedDOMSource interface {
	MaterializedDOM(target TargetRef, domMode string) (dom.Projection, bool)
}

// effectiveDOMMode 返回请求的有效 dom 模式（domInputFromObservation 同口径：
// dom_mode 参数缺省 source_only）。
func effectiveDOMMode(req Request) string {
	mode := strings.ToLower(strings.TrimSpace(cleanAnyString(req.Args["dom_mode"])))
	if mode == "" {
		return dom.ModeSourceOnly
	}
	return mode
}

// FinalizeObservationContextSkipDOM 是命中装配的旁路 finalize：dom 投影已由
// 物化行装配（obs.DOMProjection 预置），原函数的两处 finalizeDOMProjection
// 调用与其 note 计数一并跳过（recomputed=0 的事实基础），其余编排与
// FinalizeObservationContext 逐行同构——同步演化由 TestFinalizeSkipDOMParityLock
// 锁定；原函数零改动（AGENTS §12 现路径是权威输入）。
func FinalizeObservationContextSkipDOM(obs *ObservationPacket, req Request, now string) {
	if obs == nil {
		return
	}
	if strings.TrimSpace(now) == "" {
		now = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if observationWantsBandStereoProjection(req.Args) {
		applyBandStereoProjection(obs, req)
	}
	finalizeMOMProjection(obs, req)
	noteObservationFinalize(finalizeKindMOM)
	finalizeTIMProjection(obs, req)
	noteObservationFinalize(finalizeKindTIM)
	finalizeFXMProjection(obs, req)
	noteObservationFinalize(finalizeKindFXM)
	finalizeCOMProjection(obs, req)
	noteObservationFinalize(finalizeKindCOM)
	obs.Digest = BuildDigest(*obs, req)
	obs.Catalog = BuildCatalog(*obs, req, now)
	if observationWantsBandStereoProjection(req.Args) {
		filterBandStereoProjectionCatalog(obs)
		stripObservationRawKeys(obs)
	}
}
