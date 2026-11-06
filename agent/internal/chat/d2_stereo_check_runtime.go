package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/mixboard"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-STEREO-THIN-1：D2 翻译检查（stereo→mono translation）·薄 runtime。观察载体
// 复用 pre_master.observation.v0 pack 已建的 D2 面（MIXLAYER-M1-4：mix.observe 全局
// stereo_relation + mix.read 逐轨 track.<id>.slow.stereo.summary，mono 风险阈值
// corr<0.25||neg>0.1 已在 M1-4 入库）；本文件按 D1 settle 模式（d1_loudness_settle_
// runtime.go）只补四件事：
//
//  1. 显式检查分支（d2StereoCheckIntent 触发）：在观察报告之上跑判据纯函数，
//     verdict 三态（risk_found/no_risk/unmeasurable，LW verdict 模式——行缺失/
//     无生产者=unmeasurable 一等公民）；
//  2. verdict 落 store：stereo_check_verdicts.v0（LW verdict store 同款纪律：
//     外来 schema fail-closed、dedupe、上限裁剪），与 loudness_watch_verdicts.json
//     同目录存放；
//  3. 动作面 v1=纯披露：确认链 v1 操作族（track_gain_adjust/track_pan_adjust/
//     track_pan_set/static_eq_band_adjust/broadband_threshold_adjust）无一能改变
//     声道间相关性——gain 对 mono 抵消比例不变、pan 收窄不修复相位抵消，真实修复
//     （立体声宽度/相位处理）属母带 E3 面未建。按「不发明动作」红线（loudnesswatch/
//     action.go 同款：unaddressable 目标如实披露、绝不发明），risk_found 不挂写卡，
//     回执申报由主管裁定；
//  4. D2 点亮：queue view 的 D2 节点 not_started→in_progress（annotation 面；
//     completed 留给账本证据推导，M1-2 红线不变）。
//
// 判断权冻结令：本文件无 LLM 参与；入口是显式检查意图词（词法命中在 semantic_entry
// 之前，判据腿零 LLM），阈值判断归 d2AssessStereoRisk 纯函数，消费 M1-4 已定的
// MonoRisk 标志与同款阈值常量，不另立标准。
const d2StereoCheckCapabilityID = "pre_master.stereo_check.v0"

// d2StereoCheckTriggerKind marks explicit D2 checks on verdict records.
const d2StereoCheckTriggerKind = "d2_stereo_check"

// Verdict states (three-state, LW verdict pattern).
const (
	d2StereoVerdictRiskFound    = "risk_found"
	d2StereoVerdictNoRisk       = "no_risk"
	d2StereoVerdictUnmeasurable = "unmeasurable"
)

// d2StereoNoV1ActionReason pins the v1 action-face boundary (disclosure only):
// no existing mix-tick operation honestly addresses mono translation risk.
const d2StereoNoV1ActionReason = "v1 确认链操作（gain/pan/eq_band/broadband）无一能改变声道间相关性：gain 对 mono 抵消比例不变，pan 收窄不修复相位抵消；真实修复（立体声宽度/相位处理）属母带 E3 面未建——按「不发明动作」红线，risk_found 仅披露不挂写卡"

// d2StereoVerdictStoreSchemaVersion is the only accepted stereo-check verdict
// store schema.
const d2StereoVerdictStoreSchemaVersion = "stereo_check_verdicts.v0"

// d2StereoMaxKeptVerdicts bounds the verdict history (LW store 同款).
const d2StereoMaxKeptVerdicts = 50

// d2StereoCheckIntent reports whether the message asks for the explicit D2
// translation-check face. Explicit D markers already route into the pack through
// the D1-D3 observation entry; these check-request phrases (same family shape
// as D1's 全局响度检查/峰值检查族) select the deep face inside the handler.
// 裸话题词（「立体声」「mono」「单声道」「相位」单独出现）不是检查请求，不在此列
// ——裸话术入口按卡面走 semantic_entry 实测（263 教训），能过则不建词表。
func d2StereoCheckIntent(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return textHasAny(text,
		"翻译检查", "立体声检查", "单声道检查", "单声道兼容", "mono 检查", "mono 兼容",
		"mono 风险", "mono check", "stereo check", "translation check", "mono compatibility",
	)
}

// d2StereoRiskTrack is one mono-risk row in the verdict, carrying the metric
// that tripped the pinned threshold.
type d2StereoRiskTrack struct {
	TrackID             string   `json:"track_id"`
	CorrelationEstimate *float64 `json:"correlation_estimate,omitempty"`
	PhaseNegativeRatio  *float64 `json:"phase_negative_ratio,omitempty"`
	Basis               string   `json:"basis"`
}

// d2StereoVerdict is the stereo_check.v0 record. Pure data: the assessor
// leaves identity/trigger/时间戳 empty; the runtime fills them.
type d2StereoVerdict struct {
	SchemaVersion      string              `json:"schema_version"`
	Verdict            string              `json:"verdict"`
	GlobalCorrelation  *float64            `json:"global_correlation,omitempty"`
	GlobalBalanceDB    *float64            `json:"global_balance_db,omitempty"`
	RiskTracks         []d2StereoRiskTrack `json:"risk_tracks,omitempty"`
	TracksMeasured     int                 `json:"tracks_measured"`
	TracksSkipped      int                 `json:"tracks_skipped"`
	UnmeasurableReason string              `json:"unmeasurable_reason,omitempty"`
	Summary            string              `json:"summary"`
}

// d2AssessStereoRisk is the pure criterion (zero I/O, no clock): the M1-4
// observation report in, one three-state verdict out. 阈值复用 M1-4——判据消费
// 报告行上已按 d1d3MonoRisk（corr<0.25||neg>0.1，与 project_package
// phaseRiskCandidates 同款）计算好的 MonoRisk 标志，basis 行按同款常量推导，
// 不另立标准。全局 stereo_relation 的 correlation 只披露不判风险（宽立体声混音
// 全局相关合法偏低，mono 风险判据面=逐轨，与投影层语义一致）。
func d2AssessStereoRisk(report d1d3PreMasterReport) d2StereoVerdict {
	verdict := d2StereoVerdict{SchemaVersion: d2StereoVerdictStoreSchemaVersion}
	verdict.GlobalCorrelation = report.CorrelationEstimate
	verdict.GlobalBalanceDB = report.BalanceDB
	verdict.TracksSkipped = report.TracksSkipped

	measured, riskRows := 0, []d2StereoRiskTrack{}
	rowsSeen := false
	for _, row := range report.TrackStereo {
		if row.Status != "read" {
			continue // missing/error 行不计测量（缺失如实保留，见 limitations 面）
		}
		rowsSeen = true
		if row.CorrelationEstimate == nil && row.PhaseNegativeRatio == nil {
			continue // read 但双指标缺席：不是测量证据
		}
		measured++
		if !row.MonoRisk {
			continue
		}
		risk := d2StereoRiskTrack{TrackID: row.TrackID, Basis: d2StereoRiskBasis(row.CorrelationEstimate, row.PhaseNegativeRatio)}
		if row.CorrelationEstimate != nil {
			value := *row.CorrelationEstimate
			risk.CorrelationEstimate = &value
		}
		if row.PhaseNegativeRatio != nil {
			value := *row.PhaseNegativeRatio
			risk.PhaseNegativeRatio = &value
		}
		riskRows = append(riskRows, risk)
	}

	switch {
	case len(riskRows) > 0:
		verdict.Verdict = d2StereoVerdictRiskFound
		verdict.RiskTracks = riskRows
		verdict.TracksMeasured = measured
		verdict.Summary = d2StereoSummary(verdict)
	case measured > 0:
		verdict.Verdict = d2StereoVerdictNoRisk
		verdict.TracksMeasured = measured
		verdict.Summary = d2StereoSummary(verdict)
	default:
		// 逐轨判据面一条可测行都没有：unmeasurable 一等公民（LW 模式）。全局
		// correlation/balance 已在 verdict 上披露，但不构成逐轨风险扫描结论。
		verdict.Verdict = d2StereoVerdictUnmeasurable
		verdict.UnmeasurableReason = d2StereoUnmeasurableReason(report, rowsSeen)
		verdict.Summary = "无法测量：" + verdict.UnmeasurableReason + "。"
	}
	return verdict
}

// d2StereoRiskBasis names the threshold(s) a risk row tripped, using the same
// pinned constants as d1d3MonoRisk.
func d2StereoRiskBasis(correlation, negativePhase *float64) string {
	parts := []string{}
	if correlation != nil && *correlation < d1d3CorrelationRiskFloor {
		parts = append(parts, fmt.Sprintf("correlation %.2f < %.2f", *correlation, d1d3CorrelationRiskFloor))
	}
	if negativePhase != nil && *negativePhase > d1d3NegativePhaseCeiling {
		parts = append(parts, fmt.Sprintf("phase_negative_ratio %.2f > %.2f", *negativePhase, d1d3NegativePhaseCeiling))
	}
	if len(parts) == 0 {
		return "mono_risk 标志来自观察面（M1-4 同款阈值）"
	}
	return strings.Join(parts, "；")
}

// d2StereoUnmeasurableReason distinguishes the honest gap shapes (LW 模式：
// 行缺失/无生产者分开记录).
func d2StereoUnmeasurableReason(report d1d3PreMasterReport, rowsSeen bool) string {
	switch {
	case report.ObserveError != "":
		return "mix.observe 未成功返回，全局 stereo 载体缺席：" + report.ObserveError
	case report.StereoStatus != "present" && !rowsSeen:
		return "no_stereo_evidence: 全局 stereo_relation 缺席且无任何可读的逐轨 stereo summary 行"
	case report.StereoStatus != "present":
		return "no_stereo_producer: 逐轨 stereo summary 行在场但均未携带 correlation/phase_negative_ratio 值（无生产者）"
	default:
		return "no_track_stereo_producer: 全局 stereo 面在场但逐轨 stereo summary 无一携带可测值（无生产者）"
	}
}

func d2StereoSummary(verdict d2StereoVerdict) string {
	thresholdNote := fmt.Sprintf("阈值与投影层同款：correlation<%.2f 或 phase_negative_ratio>%.2f；近似口径结论，不构成翻译合规判定", d1d3CorrelationRiskFloor, d1d3NegativePhaseCeiling)
	switch verdict.Verdict {
	case d2StereoVerdictRiskFound:
		text := fmt.Sprintf("检出 %d 条 mono 风险轨（已测 %d 轨；%s）。", len(verdict.RiskTracks), verdict.TracksMeasured, thresholdNote)
		if verdict.TracksSkipped > 0 {
			text += fmt.Sprintf("另有 %d 轨超出读取上限未读，结论仅覆盖已读轨。", verdict.TracksSkipped)
		}
		return text
	default:
		text := fmt.Sprintf("已测 %d 轨未见 mono 风险（%s）。", verdict.TracksMeasured, thresholdNote)
		if verdict.TracksSkipped > 0 {
			text += fmt.Sprintf("另有 %d 轨超出读取上限未读，结论仅覆盖已读轨。", verdict.TracksSkipped)
		}
		return text
	}
}

// d2StereoVerdictRecord is one stored verdict plus trigger telemetry (LW
// VerdictRecord 同款形态).
type d2StereoVerdictRecord struct {
	Key             string          `json:"key"`
	ProjectUUID     string          `json:"project_uuid,omitempty"`
	ProjectRevision string          `json:"project_revision,omitempty"`
	VerdictKind     string          `json:"verdict_kind"`
	Verdict         d2StereoVerdict `json:"verdict"`
	TriggerKind     string          `json:"trigger_kind"`
	TriggerRef      string          `json:"trigger_ref,omitempty"`
	RecordedAt      string          `json:"recorded_at"`
	LastRecheckedAt string          `json:"last_rechecked_at,omitempty"`
	RecheckCount    int             `json:"recheck_count,omitempty"`
	RiskTrackCount  int             `json:"risk_track_count"`
	TracksMeasured  int             `json:"tracks_measured"`
	TracksSkipped   int             `json:"tracks_skipped"`
}

type d2StereoVerdictStoreFile struct {
	SchemaVersion string                  `json:"schema_version"`
	UpdatedAt     string                  `json:"updated_at"`
	ChecksTotal   int64                   `json:"checks_total"`
	Verdicts      []d2StereoVerdictRecord `json:"verdicts"`
}

// d2StereoVerdictStoreMu serializes store file access in-process (explicit
// checks are rare; the mutex keeps concurrent conversations from interleaving
// read-modify-write cycles on the same file).
var d2StereoVerdictStoreMu sync.Mutex

// d2StereoDedupeKey mirrors the LW pinned dedupe key: project uuid + project
// revision + verdict kind.
func d2StereoDedupeKey(projectUUID, projectRevision, verdictKind string) string {
	return projectUUID + "|" + projectRevision + "|" + verdictKind
}

// d2StereoVerdictStorePath colocates the stereo-check verdict history with the
// mixboard feature snapshot of the current project binding (loudness_watch_
// verdicts.json 同目录).
func d2StereoVerdictStorePath() string {
	return filepath.Join(filepath.Dir(mixboard.FeatureSnapshotPath(map[string]any{})), "stereo_check_verdicts.json")
}

// recordD2StereoVerdict persists one verdict with the LW store discipline:
// load-once-per-call, foreign schema fail-closed to an empty history (AGENTS
// §11), dedupe by pinned key (rechecks bump the audit counters, never
// duplicate), max-kept trim, write-through. Failures WARN through the logger
// and never fail the check response (observation parasite, LW 同款); the
// dedupe key is returned regardless so the queue-node evidence stays stable.
func recordD2StereoVerdict(path string, record d2StereoVerdictRecord, now time.Time, warn func(format string, args ...any)) string {
	d2StereoVerdictStoreMu.Lock()
	defer d2StereoVerdictStoreMu.Unlock()
	store := d2StereoVerdictStoreFile{SchemaVersion: d2StereoVerdictStoreSchemaVersion}
	if data, err := os.ReadFile(path); err == nil {
		loaded := d2StereoVerdictStoreFile{}
		if err := json.Unmarshal(data, &loaded); err != nil {
			warn("stereo check verdict store unmarshal failed path=%s err=%v (starting an empty history)", path, err)
		} else if loaded.SchemaVersion != "" && loaded.SchemaVersion != d2StereoVerdictStoreSchemaVersion {
			warn("stereo check verdict store: schema version %q is not %q (fail closed to empty history)", loaded.SchemaVersion, d2StereoVerdictStoreSchemaVersion)
		} else {
			if loaded.SchemaVersion == "" {
				loaded.SchemaVersion = d2StereoVerdictStoreSchemaVersion
			}
			store = loaded
		}
	}
	key := d2StereoDedupeKey(record.ProjectUUID, record.ProjectRevision, record.VerdictKind)
	record.Key = key
	stamp := now.UTC().Format(time.RFC3339Nano)
	store.SchemaVersion = d2StereoVerdictStoreSchemaVersion
	store.UpdatedAt = stamp
	store.ChecksTotal++
	deduped := false
	for index := range store.Verdicts {
		if store.Verdicts[index].Key == key {
			store.Verdicts[index].LastRecheckedAt = stamp
			store.Verdicts[index].RecheckCount++
			store.Verdicts[index].TriggerKind = record.TriggerKind
			store.Verdicts[index].TriggerRef = record.TriggerRef
			store.Verdicts[index].RiskTrackCount = record.RiskTrackCount
			store.Verdicts[index].TracksMeasured = record.TracksMeasured
			store.Verdicts[index].TracksSkipped = record.TracksSkipped
			deduped = true
			break
		}
	}
	if !deduped {
		record.RecordedAt = stamp
		store.Verdicts = append(store.Verdicts, record)
	}
	if len(store.Verdicts) > d2StereoMaxKeptVerdicts {
		store.Verdicts = store.Verdicts[len(store.Verdicts)-d2StereoMaxKeptVerdicts:]
	}
	if path == "" {
		return key
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		warn("stereo check verdict store mkdir failed path=%s err=%v", path, err)
		return key
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		warn("stereo check verdict store marshal failed err=%v", err)
		return key
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		warn("stereo check verdict store write failed path=%s err=%v", path, err)
	}
	return key
}

// handleD2StereoCheck runs the D2 translation-check face on top of the
// already-built D1-D3 observation report. read_only for the project: the only
// state this face writes is the stereo-check verdict store (observation
// bookkeeping) and the D2 queue-node annotation; v1 parks no write card (见
// 文件头动作面冻结说明).
func (s *Server) handleD2StereoCheck(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, state *kernel.VSPStateResult, report d1d3PreMasterReport) ChatResponse {
	verdict := d2AssessStereoRisk(report)
	projectRevision := ""
	if state != nil && state.Revision > 0 {
		projectRevision = fmt.Sprintf("%d", state.Revision)
	}
	record := d2StereoVerdictRecord{
		ProjectUUID:     s.d1SettleProjectIdentity(ctx),
		ProjectRevision: projectRevision,
		VerdictKind:     verdict.Verdict,
		Verdict:         verdict,
		TriggerKind:     d2StereoCheckTriggerKind,
		TriggerRef:      sessionID,
		RiskTrackCount:  len(verdict.RiskTracks),
		TracksMeasured:  verdict.TracksMeasured,
		TracksSkipped:   verdict.TracksSkipped,
	}
	storeKey := recordD2StereoVerdict(d2StereoVerdictStorePath(), record, time.Now(), s.warnD2Stereo)

	workflowData := map[string]any{
		"session_id":          sessionID,
		"capability_id":       d2StereoCheckCapabilityID,
		"canary_stage":        "d2_stereo_check",
		"read_only":           true,
		"mutation_performed":  false,
		"schema":              d1d3ObservationSchema,
		"observation":         report,
		"d2_stereo_check":     d2StereoPayload(verdict, storeKey),
		"observation_carrier": premasterObservationCapabilityID,
	}
	response := ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          d2StereoReply(verdict, report),
		Workflow:       "capability_runtime_v1",
		WorkflowData:   workflowData,
		GoalStatus:     string(agentruntime.StatusCompleted),
		StopReason:     "d2_stereo_checked",
	}
	// v1 动作面=纯披露：无确认卡路径，不产生 InteractionRequests（AGENTS 治理红线
	// 之上的「不发明动作」边界，见文件头）。节点点亮：检查已跑=in_progress。
	s.writeD2StereoCheckQueueNode(conversationID, verdict, storeKey)
	return response
}

func (s *Server) warnD2Stereo(format string, args ...any) {
	if s != nil && s.logger != nil {
		s.logger.Warn("[d2.stereo_check] "+format, args...)
	}
}

// d2StereoPayload is the structured disclosure for WorkflowData.
func d2StereoPayload(verdict d2StereoVerdict, storeKey string) map[string]any {
	payload := map[string]any{
		"capability_id":      d2StereoCheckCapabilityID,
		"checked":            true,
		"verdict":            verdict.Verdict,
		"verdict_summary":    verdict.Summary,
		"risk_track_count":   len(verdict.RiskTracks),
		"tracks_measured":    verdict.TracksMeasured,
		"tracks_skipped":     verdict.TracksSkipped,
		"action_face":        "disclosure_only_v1",
		"action_face_reason": d2StereoNoV1ActionReason,
		"verdict_store_key":  storeKey,
	}
	if verdict.GlobalCorrelation != nil {
		payload["global_correlation"] = *verdict.GlobalCorrelation
	}
	if verdict.GlobalBalanceDB != nil {
		payload["global_balance_db"] = *verdict.GlobalBalanceDB
	}
	if verdict.UnmeasurableReason != "" {
		payload["unmeasurable_reason"] = verdict.UnmeasurableReason
	}
	return payload
}

// d2StereoReply renders the user-visible check report: global face, per-track
// scan, verdict summary with the pinned-threshold qualifier, and the honest
// action-face boundary.
func d2StereoReply(verdict d2StereoVerdict, report d1d3PreMasterReport) string {
	lines := []string{"D2 翻译检查（stereo→mono，schema " + d1d3ObservationSchema + " 载体，只读）。"}
	if verdict.GlobalCorrelation != nil || verdict.GlobalBalanceDB != nil {
		parts := []string{}
		if verdict.GlobalCorrelation != nil {
			parts = append(parts, fmt.Sprintf("correlation %.2f", *verdict.GlobalCorrelation))
		}
		if verdict.GlobalBalanceDB != nil {
			parts = append(parts, fmt.Sprintf("balance %+.1f dB", *verdict.GlobalBalanceDB))
		}
		lines = append(lines, "全局立体声（mix.observe digest）："+strings.Join(parts, " / ")+"。")
	} else {
		lines = append(lines, "全局立体声（mix.observe digest）：stereo_relation 缺席（缺失如实保留）。")
	}
	lines = append(lines, fmt.Sprintf("逐轨 stereo summary：读取 %d 轨，mono 风险 %d 轨。", len(report.TrackStereo), len(verdict.RiskTracks)))
	if verdict.TracksSkipped > 0 {
		lines = append(lines, fmt.Sprintf("另有 %d 轨超出单次读取上限未读，结论仅覆盖已读轨。", verdict.TracksSkipped))
	}
	lines = append(lines, "判断结果："+verdict.Summary)
	if verdict.Verdict == d2StereoVerdictRiskFound {
		for _, risk := range verdict.RiskTracks {
			lines = append(lines, fmt.Sprintf("风险轨 %s：%s。", risk.TrackID, risk.Basis))
		}
		lines = append(lines, "本次不生成写操作确认卡："+d2StereoNoV1ActionReason+"。未修改工程。")
	}
	lines = append(lines, "检查结果已记入 stereo check verdict 记录；D2 节点状态可在队列视图查看。")
	return strings.Join(lines, "\n")
}

// writeD2StereoCheckQueueNode advances the workflow queue's D2 node out of
// not_started (annotation-face discipline: in_progress only in v1 — no card is
// parked, completion stays ledger-derived). A write failure WARNs and never
// blocks the check response.
func (s *Server) writeD2StereoCheckQueueNode(conversationID string, verdict d2StereoVerdict, storeKey string) {
	reason := "D2 翻译检查已执行（" + verdict.Verdict + "）"
	evidence := []string{"d2_stereo_check:" + d2StereoCheckCapabilityID, "stereo_check_verdict:" + storeKey}
	nodeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.WriteMixWorkflowQueueNodeState(nodeCtx, "D2", MixWorkflowNodeInProgress, reason, evidence, conversationID); err != nil {
		if s.logger != nil {
			s.logger.Warn("[d2.stereo_check] queue node annotation failed conversation=%s err=%v", conversationID, err)
		}
	} else if s.logger != nil {
		s.logger.Info("[d2.stereo_check] D2 node advanced conversation=%s state=in_progress verdict=%s", conversationID, verdict.Verdict)
	}
}
