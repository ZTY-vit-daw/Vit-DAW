// materialize_onpath.go — MAT-E on 态读端切换（docs/MATERIALIZATION_V1_DESIGN.md
// §7.1 on 行 / §2.2 observe 读端触发表；QUERY_ENGINE §2.6 响应字段）。
//
// 三件（全部旁路，FinalizeObservationContext 原路径零改动）：
//
//	consult   requestMixObservation 装配前经 Request.MaterializedDOM 注入——
//	          白名单 kind（dom）命中 current 行集→evidence CAS 回读全量投影
//	          直接装配（recomputed=0）；miss/stale/无句柄→miss（原路径现算）；
//	backfill  miss 轮收尾：现算产物全量 PutEvidence（kind=materialized.dom，
//	          §2.1 内容层）+ 行挂 evidence:// 句柄 Upsert——命中装配的内容源；
//	          measurement-carried 行（MixPackage 测量回退轴激活）不回填（F9
//	          非 precomputable，与影子闸门排除口径一致）；
//	response  observe 响应字段 actual_cost_class（index/compile）+recomputed
//	          （0/1）——仅 on 态可见（off/shadow 响应面不变；T11 与 CCB-PARAM
//	          汇合时再统一常驻）。
//
// on_round 日志行（真栈烟测断言面）：
//
//	[materialize] on_round observation_id=... dom=hit|miss recomputed=0|1 cost_class=index|compile hits=N misses=N backfilled=0|1
package harness

import (
	"encoding/json"
	"strings"

	"vit-daw-agent/internal/dom"
	"vit-daw-agent/internal/materialize"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/projectstore"
)

// materializeEvidenceKindDom 是物化内容层的 evidence kind（§2.1：materialized.<kind>）。
const materializeEvidenceKindDom = "materialized.dom"

// materializeDOMConsult 是 on 态读端 consult（Request.MaterializedDOM 的实现；
// per-request 构造——asked/hit 是本轮结果态）。
type materializeDOMConsult struct {
	h     *Harness
	asked bool
	hit   bool
}

// materializeDOMSource 装配 on 态读端 consult；nil=不消费（off/shadow/未接线）。
func (h *Harness) materializeDOMSource() *materializeDOMConsult {
	if h == nil || h.materializeStore == nil || h.materializeMode != materialize.ModeOn {
		return nil
	}
	return &materializeDOMConsult{h: h}
}

// MaterializedDOM 实现 mixboard.MaterializedDOMSource：命中返回 CAS 全量投影。
func (c *materializeDOMConsult) MaterializedDOM(target mixboard.TargetRef, domMode string) (dom.Projection, bool) {
	if c == nil || c.h == nil || c.h.materializeStore == nil {
		return dom.Projection{}, false
	}
	c.asked = true
	c.hit = false
	miss := func() (dom.Projection, bool) {
		c.h.materializeStore.NoteObserveConsult("dom", false)
		return dom.Projection{}, false
	}
	if domMode != dom.ModeSourceOnly {
		return miss() // F9：paired/change_delta 请求携带测量，不预计算
	}
	trackID := strings.TrimSpace(target.ID)
	if !strings.EqualFold(strings.TrimSpace(target.Kind), "track") || trackID == "" {
		return miss() // 物化行坐标是 track 域（非轨目标结构性 miss）
	}
	row, ok := c.h.materializeStore.CurrentPrecomputableRow("dom", "track", trackID)
	if !ok {
		return miss() // miss/stale 如实（§2.3 读侧零推断）
	}
	if !strings.HasPrefix(row.Handle, "evidence://") {
		return miss() // 影子级行（无内容句柄）不供读端装配
	}
	roots, active := projectstore.Current()
	if !active {
		return miss()
	}
	blob, err := projectstore.GetEvidence(roots, row.Handle)
	if err != nil || blob.Kind != materializeEvidenceKindDom {
		if c.h.logger != nil {
			// 取证面：句柄失效（预算清理/身份不符）——降级 miss 不静默。
			c.h.logger.Warn("[materialize] on_read_handle_unresolvable track=%s handle=%s err=%v (降级 miss，走原路径)", trackID, row.Handle, err)
		}
		return miss()
	}
	data, err := json.Marshal(blob.Content)
	if err != nil {
		return miss()
	}
	var projection dom.Projection
	if err := json.Unmarshal(data, &projection); err != nil {
		return miss()
	}
	if strings.TrimSpace(projection.SchemaVersion) == "" && strings.TrimSpace(projection.Status) == "" {
		return miss() // 空投影不装配（与登记口径同判定）
	}
	c.hit = true
	c.h.materializeStore.NoteObserveConsult("dom", true)
	return projection, true
}

// materializeOnPathBackfill 把 miss 轮现算产物回填物化层（§2.2 读端触发表：
// upsert+计 miss 的写侧半边）：全量投影 PutEvidence 进内容层（命中装配的
// 内容源）+ 行挂句柄 Upsert。命中轮与未 consult 轮（paired）零回填；
// measurement-carried 行不回填（DOMMeasurementFallbackActive——与影子闸门
// 排除同口径，F9 非 precomputable 不得进读端内容层）。
func (h *Harness) materializeOnPathBackfill(obs *mixboard.ObservationPacket) {
	if h == nil || h.materializeStore == nil || obs == nil || obs.DOMProjection == nil {
		return
	}
	trackID := strings.TrimSpace(obs.TargetRef.ID)
	if !strings.EqualFold(strings.TrimSpace(obs.TargetRef.Kind), "track") || trackID == "" {
		return
	}
	if mixboard.DOMMeasurementFallbackActive(*obs) {
		if h.logger != nil {
			h.logger.Info("[materialize] on_backfill_skipped kind=dom track=%s observation_id=%s (measurement_carried：MixPackage 测量回退轴激活，非 precomputable 不回填)", trackID, obs.ObservationID)
		}
		return
	}
	row, ok := materialize.DOMRowFromProjection(trackID, *obs.DOMProjection)
	if !ok {
		return
	}
	roots, active := projectstore.Current()
	if !active {
		if h.logger != nil {
			h.logger.Info("[materialize] on_backfill_skipped kind=dom track=%s observation_id=%s (projectstore 未激活：无内容层，行降级影子级)", trackID, obs.ObservationID)
		}
		return
	}
	handle, _, err := projectstore.PutEvidence(roots, materializeEvidenceKindDom, *obs.DOMProjection, "obs:"+obs.ObservationID)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("[materialize] on_backfill_evidence_failed kind=dom track=%s observation_id=%s err=%v (行降级影子级，读端将持续 miss)", trackID, obs.ObservationID, err)
		}
		return
	}
	row.Handle = handle
	if err := h.materializeStore.Upsert([]materialize.Row{row}); err != nil && h.logger != nil {
		h.logger.Warn("materialize: 读端回填 upsert 失败: %s", err)
	}
}

// materializeOnPathTail 是 on 态读端收尾（requestMixObservation 尾挂，影子轮
// 之后）：miss 回填 + on_round 日志行 + 响应字段值。返回 visible=false 表示
// 读端未切换（consult 未装配——off/shadow，响应面不变）。
func (h *Harness) materializeOnPathTail(obs *mixboard.ObservationPacket, consult *materializeDOMConsult) (costClass string, recomputed int, visible bool) {
	if h == nil || consult == nil || h.materializeStore == nil {
		return "", 0, false
	}
	backfilled := 0
	if consult.hit {
		costClass, recomputed = "index", 0
	} else {
		costClass, recomputed = "compile", 1
		if consult.asked {
			h.materializeOnPathBackfill(obs)
			backfilled = 1
		}
	}
	if h.logger != nil {
		domState := "miss"
		if consult.hit {
			domState = "hit"
		}
		metrics := h.materializeStore.Metrics()
		observationID := ""
		if obs != nil {
			observationID = obs.ObservationID
		}
		h.logger.Info("[materialize] on_round observation_id=%s dom=%s recomputed=%d cost_class=%s hits=%d misses=%d backfilled=%d",
			observationID, domState, recomputed, costClass, metrics.PerKind["dom"].Hits, metrics.PerKind["dom"].Misses, backfilled)
	}
	return costClass, recomputed, true
}

// materializeOnResponseFields 返回 observe 响应字段值（actual_cost_class/
// recomputed；仅 on 态 visible——QUERY_ENGINE §2.6，T11 汇合点预留）。
func (h *Harness) materializeOnResponseFields(consult *materializeDOMConsult) (costClass string, recomputed int, visible bool) {
	if h == nil || consult == nil || h.materializeStore == nil || h.materializeMode != materialize.ModeOn {
		return "", 0, false
	}
	if consult.hit {
		return "index", 0, true
	}
	return "compile", 1, true
}
