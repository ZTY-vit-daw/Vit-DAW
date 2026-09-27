package harness

// materialize_shadow.go — MAT-C 三态 flag 装配与影子轮（旁路，FinalizeObservation
// Context 原路径零改动；docs/MATERIALIZATION_V1_DESIGN.md §7.1）。
//
// 装配（initMaterialization，构造期一次）：
//
//	off    零接线（现状逐字节一致，off 态锁定测试见证）；
//	shadow 全接线——物化库+默认适配器（白名单 kind）+MAT-B Notifier 三挂点；
//	on     同 shadow（读端命中装配/miss 回填归 MAT-E，本卡不消费读端）。
//
// 影子轮（materializeObserveRound，requestMixObservation 尾挂）：
//  1. RecomputeLazy——白名单 precomputable kind 从输入域 lazy 重算回填；
//  2. fxm/com 观察产物登记（F9：随观察落盘，不预计算）；
//  3. ReconcileShadow——观察路径现算的 dom 产物 vs 物化行逐行 hash 对账，
//     分歧入 Metrics.ShadowDivergences（§7.2 切换闸门仪表）。MAT-D2 登记型：
//     经 MixPackage 测量回退轴激活的 dom 行（DOMRowFromObservation 判定）标记
//     non_precomputable+measurement_carried，闸门排除并单列计数
//     ShadowMeasurementCarriedExcluded（与物化侧不同源——DepInputs 不携带测量）。
//
// 只写物化库（h.materializeStore）与只读 harness 状态——既有可观察面零触碰。

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/materialize"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/shadow"
)

// initMaterialization 读三态 flag 并装配物化层（构造期单次、不与事件流并发）。
// 非法 flag 值：log 后回退 off（旁路层宁可不开不可开错）。
func (h *Harness) initMaterialization() {
	cfg, err := materialize.ConfigFromEnv(os.Getenv)
	if err != nil && h.logger != nil {
		h.logger.Warn("%s", err)
	}
	if cfg.Mode == materialize.ModeOff {
		return
	}
	store := materialize.NewStore()
	if err := materialize.RegisterDefaultAdapters(store, cfg.Kinds); err != nil {
		if h.logger != nil {
			h.logger.Warn("materialize: 适配器注册失败，物化层保持未接线: %s", err)
		}
		return
	}
	h.materializeStore = store
	h.SetMaterializeNotifier(h.loggingMaterializeNotifier(store.Notifier()))
	if h.logger != nil {
		// MAT-D 烟测断言①：shadow 第三态在真栈可活的启动证据（off 态零行）。
		h.logger.Info("[materialize] mode=%s kinds=%s store=memory", cfg.Mode, strings.Join(cfg.Kinds, ","))
	}
}

// loggingMaterializeNotifier 在转发通知的同时打事件级日志（MAT-D 烟测断言③：
// 变更事件→脏传播在真栈事件流上的可观测证据）。遥测分支高频不打（arrive 证据
// 由影子轮 metrics 行的 Arrivals 计数承载）；off 态不构造本包装（零行为）。
type loggingMaterializeNotifier struct {
	h    *Harness
	next materialize.Notifier
}

func (h *Harness) loggingMaterializeNotifier(next materialize.Notifier) materialize.Notifier {
	return loggingMaterializeNotifier{h: h, next: next}
}

func (n loggingMaterializeNotifier) NotifyShadowChange(receipt shadow.ChangeReceipt) {
	n.next.NotifyShadowChange(receipt)
	if n.h.logger != nil {
		n.h.logger.Info("[materialize] receipt change_id=%s scopes=%s entities=%d",
			receipt.ChangeID, strings.Join(receipt.AffectedScopes, ","), len(receipt.ChangedEntities))
	}
}

func (n loggingMaterializeNotifier) NotifyTelemetry(event map[string]any) {
	n.next.NotifyTelemetry(event)
}

func (n loggingMaterializeNotifier) NotifyRenderJob(jobID, status, filePath string) {
	n.next.NotifyRenderJob(jobID, status, filePath)
	if n.h.logger != nil {
		n.h.logger.Info("[materialize] render_job job_id=%s status=%s file=%s", jobID, status, filePath)
	}
}

// materializeObserveRound 执行一轮影子物化（旁路尾挂；h.materializeStore nil=
// 零接线现状）。obs 只读。
func (h *Harness) materializeObserveRound(obs *mixboard.ObservationPacket) {
	if h.materializeStore == nil || obs == nil {
		return
	}
	deps := h.materializeDepInputs(obs)
	if err := h.materializeStore.RecomputeLazy(deps); err != nil && h.logger != nil {
		h.logger.Warn("materialize: 影子轮重算失败: %s", err)
	}
	var registered []materialize.Row
	registered = append(registered, materialize.FXMRowsFromObservation(obs.ObservationID, obs.FXMProjection)...)
	registered = append(registered, materialize.COMRowsFromObservation(obs.ObservationID, obs.COMProjection)...)
	if len(registered) > 0 {
		if err := h.materializeStore.Upsert(registered); err != nil && h.logger != nil {
			h.logger.Warn("materialize: 观察产物登记失败: %s", err)
		}
	}
	if obs.DOMProjection != nil {
		trackID := strings.TrimSpace(obs.TargetRef.ID)
		if strings.EqualFold(strings.TrimSpace(obs.TargetRef.Kind), "track") && trackID != "" {
			if row, ok := materialize.DOMRowFromObservation(trackID, obs); ok {
				if materialize.IsMeasurementCarried(row) && h.logger != nil {
					// MAT-D2 登记型可见性：回退轴激活行登记 non_precomputable+
					// measurement_carried 并被闸门排除——INFO 级（登记处置，
					// 不是分歧告警面；计数在 shadow_round 行单列）。
					h.logger.Info("[materialize] measurement_carried_excluded kind=dom track=%s observation_id=%s (MixPackage 测量回退轴激活：登记 non_precomputable，对账闸门排除)",
						trackID, obs.ObservationID)
				}
				reconcile := h.materializeStore.ReconcileShadowWithTiming([]materialize.Row{row}, &deps)
				if reconcile.TimingCarried > 0 && h.logger != nil {
					// MAT-D4 登记型可见性：时序两态分歧（观察投影=finalize 世代
					// vs 物化行=尾挂世代，当前输入重放==物化行而观察行异）被闸门
					// 排除并单列计数——INFO 级（登记处置，不是失效告警面；计数在
					// shadow_round 行单列；不可靠形态仍计分歧走 WARN 取证面）。
					// 两侧 hash 附带（收敛轮仍 timing 涨=停止条件，红须可解释）。
					firstFreshHash, firstStoredHash := "", ""
					if len(reconcile.TimingDetails) > 0 {
						firstFreshHash = reconcile.TimingDetails[0].FreshHash
						firstStoredHash = reconcile.TimingDetails[0].StoredHash
					}
					h.logger.Info("[materialize] timing_carried_excluded kind=dom track=%s observation_id=%s rows=%d fresh_hash=%s stored_hash=%s (输入世代差：物化行=当前输入同路径重放，观察投影=finalize 世代——对账排除并单列计数)",
						trackID, obs.ObservationID, reconcile.TimingCarried, firstFreshHash, firstStoredHash)
				}
				if reconcile.Divergences > 0 && h.logger != nil {
					// MAT-D 取证面：真栈分歧逐行 WARN（坐标+两侧 hash+两侧
					// payload）——ShadowDivergences!=0 必须可解释（G2 上交材料）。
					for _, detail := range reconcile.Details {
						freshJSON, _ := json.Marshal(detail.FreshPayload)
						storedJSON, _ := json.Marshal(detail.StoredPayload)
						freshText := string(freshJSON)
						storedText := string(storedJSON)
						if len(freshText) > 1600 {
							freshText = freshText[:1600] + "..."
						}
						if len(storedText) > 1600 {
							storedText = storedText[:1600] + "..."
						}
						h.logger.Warn("[materialize] divergence ref=%s fresh_hash=%s stored_hash=%s stored_missing=%v freshness=%s invalidated_by=%s fresh_payload=%s stored_payload=%s",
							detail.Ref, detail.FreshHash, detail.StoredHash, detail.StoredMissing, detail.Freshness, detail.InvalidatedBy, freshText, storedText)
					}
					// 回退轴占用探针：dom 现算输入 builder 的三支回退
					//（snapshot.waveform_envelope / MixPackage.current_metrics /
					// TimeRuler）恰是 DepInputs 不携带的输入——真栈分歧的
					// 根因定位证据（domInputFromObservation 消费面）。
					snapshot := mapValueAny(obs.GlobalSummary["feature_snapshot"])
					metrics := mapValueAny(obs.MixPackage["current_metrics"])
					waveEnv := mapValueAny(snapshot["waveform_envelope"])
					metricWave := mapValueAny(metrics["waveform"])
					projectionJSON, _ := json.Marshal(*obs.DOMProjection)
					projectionText := string(projectionJSON)
					if len(projectionText) > 2400 {
						projectionText = projectionText[:2400] + "..."
					}
					h.logger.Warn("[materialize] divergence_probe track=%s snapshot_waveform_envelope=%v mix_metrics_waveform=%v time_ruler_duration=%v projection=%s",
						trackID, len(waveEnv) > 0, len(metricWave) > 0, obs.TimeRuler.DurationSeconds, projectionText)
				}
			}
		}
	}
	h.logMaterializeRoundMetrics(obs)
}

// mapValueAny 是 mixboard 侧 mapValue 的只读镜像（取证探针用）。
func mapValueAny(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

// logMaterializeRoundMetrics 把累计仪表打一行日志（MAT-D 烟测断言②③：
// ShadowDivergences 经日志可见且==0、脏传播 invalidations 计数可观测——
// metrics 只读面，不消费读端，不越界验 on 态）。行格式：
//
//	[materialize] shadow_round observation_id=... kinds=dom:inv=1,arr=0,rec=2,ups=3,unch=0;tom:... shadow_divergences=0 reconcile_rows=2 dropped_changes=0 recovery_skipped_rows=0 measurement_carried_excluded=0 timing_carried_excluded=0
//
// 末段 measurement_carried_excluded 是 MAT-D2 登记型单列计数（回退轴激活行
// 被闸门排除的累计——分歧行已入排除计数，不静默）；timing_carried_excluded
// 是 MAT-D4 登记型单列计数（输入世代差分歧被闸门排除的累计——G2-D 收敛态
// 口径下变更轮登记、收敛轮不涨）。
func (h *Harness) logMaterializeRoundMetrics(obs *mixboard.ObservationPacket) {
	if h.logger == nil || h.materializeStore == nil {
		return
	}
	m := h.materializeStore.Metrics()
	kinds := make([]string, 0, len(m.PerKind))
	for kind := range m.PerKind {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		km := m.PerKind[kind]
		parts = append(parts, fmt.Sprintf("%s:inv=%d,arr=%d,rec=%d,ups=%d,unch=%d",
			kind, km.Invalidations, km.Arrivals, km.Recomputes, km.RowsUpserted, km.RowsUnchanged))
	}
	observationID := ""
	if obs != nil {
		observationID = obs.ObservationID
	}
	h.logger.Info("[materialize] shadow_round observation_id=%s kinds=%s shadow_divergences=%d reconcile_rows=%d dropped_changes=%d recovery_skipped_rows=%d measurement_carried_excluded=%d timing_carried_excluded=%d",
		observationID, strings.Join(parts, ";"), m.ShadowDivergences, m.ReconcileRows, m.DroppedChanges, m.RecoverySkippedRows, m.ShadowMeasurementCarriedExcluded, m.ShadowTimingCarriedExcluded)
}

// materializeDepInputs 从 harness 状态装配适配器输入束：shadow Summary（tom
// 输入域）+观察携带的 feature snapshot（dom 输入域——与观察路径消费的同一份
// 数据面）+acp 默认 store 快照（acp 输入域；per-observation 覆盖路径的包不
// 在此列，边界随回执申报）。
func (h *Harness) materializeDepInputs(obs *mixboard.ObservationPacket) materialize.DepInputs {
	deps := materialize.DepInputs{}
	if h.shadow != nil {
		summary := h.shadow.Summary()
		deps.ProjectState = summary
		if revision, ok := summary["project_revision"].(string); ok {
			deps.ProjectRevision = strings.TrimSpace(revision)
		}
	}
	if obs != nil {
		if snapshot, ok := obs.GlobalSummary["feature_snapshot"].(map[string]any); ok {
			deps.FeatureSnapshot = snapshot
		}
		if deps.ProjectRevision == "" {
			deps.ProjectRevision = strings.TrimSpace(summaryString(obs.ProjectPackage["project_revision"]))
		}
	}
	if snap, err := acousticpackage.NewStore("").Read(); err == nil {
		deps.AcousticPackages = snap.Packages
	}
	return deps
}

func summaryString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
