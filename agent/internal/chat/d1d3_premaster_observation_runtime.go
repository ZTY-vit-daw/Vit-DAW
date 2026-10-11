package chat

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/rlm"
	agentruntime "vit-daw-agent/internal/runtime"
)

// MIXLAYER-M1-4：D1-D3 母带前观察·只读能力 Session（pre_master.observation.v0）。
// master_plan §6.1 母带前段三面的观察载体（全部既有只读面）：
//  1. D1 全局响度与峰值 —— mix.observe digest 面（peak/rms/crest + waveform
//     headroom + loudness/peak/headroom rankings + missing_metrics）+ RLM
//     reference_level_model.projection.v0（现役，B1 同款 Build 入参）；
//  2. D2 翻译检查 —— mix.observe digest 全局 stereo_relation（correlation/
//     balance）+ mix.read 逐轨 track.<id>.slow.stereo.summary 的低相关/负相位
//     mono 风险观察（阈值与 project_package phase_risk_candidate 同款）；
//  3. D3 参考曲对比 —— RLM 投影 user intent + RenderBindings fail-closed 面
//     （render→delivery 绑定经 rlm.Build 披露，unresolvable 行原样可见）。
//
// 边界：无写控制面。参考曲导入/对比命令缺口如实登记不动：audition.* 十命令
// （prepare/status/ready/stale/failed/select/position/stop/inspect_candidate/
// apply_candidate）在内核 CommandDispatcher，agent harness 工具面未编目（内部
// auditionCommandClient 仅四命令、只服务 A/B 试听回路）；D3 的参考曲对比本体
// 缺命令载体，本 pack 只收口 RLM 投影侧可观察面。D/E 母带实现后置维持
// （2026-10-04 裁定 2）：本能力不做任何母带处理。
const premasterObservationCapabilityID = "pre_master.observation.v0"

const d1d3ObservationSchema = "d1d3.premaster.observation.v0"

// d1d3ReadOnlyTools 是本 runtime 的 harness 调用白名单：所有调用必须经
// d1d3InvokeReadOnly 走这张表，表外工具 fail-closed。写命令（rack_add_node /
// set_plugin_param / mix_apply_* / render_profile_bind 等）一律不在表内
// ——单测以写命令黑名单交叉断言（TestD1D3ReadOnlyToolWhitelistHasNoWriteCommands）。
// render_profile_bind 是 RenderBindings 的写端（绑定 render→profile），不在
// 白名单：D3 面只读披露现状绑定，不产生新绑定。
var d1d3ReadOnlyTools = map[string]bool{
	"mix.observe": true, // D1 digest 面 + D2 全局 stereo 面
	"mix.read":    true, // D2 逐轨 stereo summary（track.<id>.slow.stereo.summary）
}

// d1d3TrackStereoReadLimit 限制单次观察的逐轨 stereo 读取数：逐轨 mix.read 是
// 逐次工具往返，上限保护大工程；超出部分计入报告覆盖说明，不静默丢弃。
const d1d3TrackStereoReadLimit = 8

// D2 mono/相位风险阈值（与 mixboard project_package phaseRiskCandidates 同款）：
// correlation < 0.25 或 phase_negative_ratio > 0.1 记为风险行。阈值口径与
// 投影层保持一致，本 pack 不另立标准。
const (
	d1d3CorrelationRiskFloor = 0.25
	d1d3NegativePhaseCeiling = 0.1
)

// d1d3ReferenceTrackGap 登记 D3 参考曲命令缺口（如实登记不动，见文件头注释）。
const d1d3ReferenceTrackGap = "参考曲导入/对比命令缺口：audition.* 十命令在内核 dispatcher 而不在 agent catalog，D3 参考曲对比本体缺命令载体——如实登记，本卡不补"

type d1d3TrackStereoRow struct {
	TrackID             string   `json:"track_id"`
	Status              string   `json:"status"` // read | missing | error
	CorrelationEstimate *float64 `json:"correlation_estimate,omitempty"`
	CorrelationState    string   `json:"correlation_state,omitempty"`
	BalanceDB           *float64 `json:"balance_db,omitempty"`
	PhaseNegativeRatio  *float64 `json:"phase_negative_ratio,omitempty"`
	MonoRisk            bool     `json:"mono_risk"`
	Error               string   `json:"error,omitempty"`
}

type d1d3PreMasterReport struct {
	SchemaVersion string `json:"schema_version"`
	ObservationID string `json:"observation_id,omitempty"`
	// D1 面
	LoudnessPeakStatus string           `json:"loudness_peak_status"` // present | missing
	GlobalPeakDBFS     *float64         `json:"global_peak_dbfs,omitempty"`
	GlobalRMSDBFS      *float64         `json:"global_rms_dbfs,omitempty"`
	GlobalCrestDB      *float64         `json:"global_crest_db,omitempty"`
	Waveform           map[string]any   `json:"waveform,omitempty"`
	LoudnessRanking    []map[string]any `json:"loudness_ranking_rows,omitempty"`
	HeadroomRisk       []map[string]any `json:"headroom_risk_rows,omitempty"`
	PeakRisk           []map[string]any `json:"peak_risk_rows,omitempty"`
	MissingMetrics     []string         `json:"missing_metrics,omitempty"`
	RLMStatus          string           `json:"rlm_status"`
	RLMMode            string           `json:"rlm_mode,omitempty"`
	RLMSelectedMetric  string           `json:"rlm_selected_metric,omitempty"`
	RLMReferenceLevel  *float64         `json:"rlm_reference_level,omitempty"`
	RLMSummary         map[string]any   `json:"rlm_summary,omitempty"`
	RLMLimitations     []string         `json:"rlm_limitations,omitempty"`
	// D2 面
	StereoStatus        string               `json:"stereo_status"` // present | missing
	CorrelationEstimate *float64             `json:"correlation_estimate,omitempty"`
	CorrelationState    string               `json:"correlation_state,omitempty"`
	BalanceDB           *float64             `json:"balance_db,omitempty"`
	BalanceState        string               `json:"balance_state,omitempty"`
	StereoSourceStatus  string               `json:"stereo_source_status,omitempty"`
	TrackStereo         []d1d3TrackStereoRow `json:"track_stereo,omitempty"`
	TracksConsidered    int                  `json:"tracks_considered"`
	TracksSkipped       int                  `json:"tracks_skipped"`
	MonoRiskTracks      int                  `json:"mono_risk_tracks"`
	// D3 面
	UserIntent         string           `json:"user_intent"`
	RenderBindingCount int              `json:"render_binding_count"`
	RenderBindings     []map[string]any `json:"render_bindings,omitempty"`
	ReferenceTrackGap  string           `json:"reference_track_gap"`
	Limitations        []string         `json:"limitations,omitempty"`
	ObserveError       string           `json:"observe_error,omitempty"`
}

func (s *Server) handlePreMasterObservationRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.kernel == nil || s.harness == nil {
		response := capabilityCanaryBlockedResponse(conversationID, goal, "D1-D3 母带前观察需要可用的 Project Kernel 与 Harness；当前未启动。")
		response.WorkflowData = map[string]any{
			"capability_id": premasterObservationCapabilityID, "canary_stage": "observation_blocked",
			"read_only": true, "mutation_performed": false,
		}
		return response
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		message := "无法取得当前工程 snapshot，D1-D3 母带前观察未启动。"
		if err != nil {
			message += " " + err.Error()
		}
		response := capabilityCanaryBlockedResponse(conversationID, goal, message)
		response.WorkflowData = map[string]any{
			"capability_id": premasterObservationCapabilityID, "canary_stage": "observation_blocked",
			"read_only": true, "mutation_performed": false,
		}
		return response
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, premasterObservationCapabilityID)
	}

	// 载体 1+2：mix.observe 一次取 D1 digest（peak/rms/crest+waveform+rankings）
	// 与 D2 全局 stereo_relation 面。
	observe, observeErr := s.d1d3InvokeReadOnly(ctx, sessionID, "mix.observe", d1d3ObserveArgs(sessionID, req.Message))

	// 载体 2 深面：逐轨 stereo summary（mix.read，工程态 track 面）。
	trackIDs := d1d3TrackIDsFromState(state.LegacyState)
	trackRows := make([]d1d3TrackStereoRow, 0, len(trackIDs))
	for index, trackID := range trackIDs {
		if index >= d1d3TrackStereoReadLimit {
			break
		}
		trackRows = append(trackRows, s.d1d3ReadTrackStereo(ctx, sessionID, trackID))
	}

	// 载体 1 RLM 面 + 载体 3：RenderBindings 只读快照经 rlm.Build 披露
	//（fail-closed：unresolvable 绑定行原样可见）。
	rlmProjection := s.d1d3BuildRLMProjection(req, state, observe)

	report := d1d3AssemblePreMasterReport(observe, observeErr, len(trackIDs), trackRows, rlmProjection, req.Message)
	workflowData := map[string]any{
		"session_id":         sessionID,
		"capability_id":      premasterObservationCapabilityID,
		"canary_stage":       "observation",
		"read_only":          true,
		"mutation_performed": false,
		"schema":             d1d3ObservationSchema,
		"observation":        report,
	}
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          d1d3RenderPreMasterReply(report),
		Workflow:       "capability_runtime_v1",
		WorkflowData:   workflowData,
		GoalStatus:     string(agentruntime.StatusCompleted),
	}
}

// d1d3ObserveArgs 构造 mix.observe 参数（B2 只读上下文同款读面）。不带
// mom_intent：D1-D3 观察不消费 MOM static_level_relationship（那是 B2/C3 载体），
// digest 的 D1/D2 面不依赖 MOM 投影。
func d1d3ObserveArgs(sessionID, message string) map[string]any {
	return map[string]any{
		"scope": "full_project", "project_context": true, "observation_only": true,
		"disclosure":     "digest_catalog",
		"mix_session_id": sessionID, "goal_text": message,
	}
}

// d1d3InvokeReadOnly 是 D1-D3 的唯一 harness 出口：白名单外的工具名直接报错，
// 保证观察 Session 不出现任何写命令调用点。
func (s *Server) d1d3InvokeReadOnly(ctx context.Context, sessionID, tool string, args map[string]any) (map[string]any, error) {
	if !d1d3ReadOnlyTools[tool] {
		return nil, fmt.Errorf("D1-D3 只读白名单拒绝工具 %q（写控制不在 %s）", tool, premasterObservationCapabilityID)
	}
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: tool, Args: args,
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true, "capability_id": premasterObservationCapabilityID},
		Source:  "capability_runtime_v1_d1d3", Confirmed: true, ToolCallID: "d1d3:" + sessionID + ":" + tool,
	})
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(resp.Status, "ok") {
		return nil, fmt.Errorf("%s: %s", tool, firstNonEmpty(resp.Error, resp.Status))
	}
	if resp.Result == nil {
		return map[string]any{}, nil
	}
	return resp.Result, nil
}

// d1d3TrackIDsFromState 从工程态提取 track id 序（kernel get_project_state
// shape，tracks[].track_id），用于 D2 逐轨 stereo 读取的目标面。排序后去重，
// 保证逐轨读取次序确定。
func d1d3TrackIDsFromState(state map[string]any) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, row := range mapRowsFromAny(state["tracks"]) {
		trackID := firstStringFromMap(row, "track_id", "id")
		if trackID == "" || seen[trackID] {
			continue
		}
		seen[trackID] = true
		out = append(out, trackID)
	}
	sort.Strings(out)
	return out
}

// d1d3ReadTrackStereo 读单轨 stereo summary（mix.read catalog key
// track.<id>.slow.stereo.summary）并按投影层同款阈值分类 mono 风险。
func (s *Server) d1d3ReadTrackStereo(ctx context.Context, sessionID, trackID string) d1d3TrackStereoRow {
	row := d1d3TrackStereoRow{TrackID: trackID, Status: "error"}
	result, err := s.d1d3InvokeReadOnly(ctx, sessionID, "mix.read", map[string]any{
		"mix_session_id": sessionID,
		"keys":           []string{"track." + trackID + ".slow.stereo.summary"},
	})
	if err != nil {
		row.Error = err.Error()
		return row
	}
	item := mapValue(mapValue(result["items"])["track."+trackID+".slow.stereo.summary"])
	if len(item) == 0 || firstStringFromMap(item, "reason") == "catalog_key_not_found" {
		row.Status = "missing"
		return row
	}
	row.Status = "read"
	if value, ok := d1d3Number(item["correlation_estimate"]); ok {
		row.CorrelationEstimate = &value
	}
	row.CorrelationState = firstStringFromMap(item, "correlation_state")
	if value, ok := d1d3Number(item["balance_db"]); ok {
		row.BalanceDB = &value
	}
	if value, ok := d1d3Number(item["phase_negative_ratio"]); ok {
		row.PhaseNegativeRatio = &value
	}
	row.MonoRisk = d1d3MonoRisk(row.CorrelationEstimate, row.PhaseNegativeRatio)
	return row
}

// d1d3MonoRisk 与 project_package phaseRiskCandidates 同款阈值：低相关或负相位
// 占比超限记风险。任一指标缺失时按已有指标判定（与投影层一致：无相关值且无
// 负相位值的不记风险）。
func d1d3MonoRisk(correlation, negativePhase *float64) bool {
	if correlation != nil && *correlation < d1d3CorrelationRiskFloor {
		return true
	}
	if negativePhase != nil && *negativePhase > d1d3NegativePhaseCeiling {
		return true
	}
	return false
}

// d1d3BuildRLMProjection 构造 RLM 参考电平投影（B1 gain staging 同款 Build
// 入参）：user intent（D3 面）+ 工程态 + mix.observe 结果 + RenderBindings
// 只读快照（harness 内存索引，零工具调用；render_profile_list 的披露形态由
// rlm.Build 自产，不重复取）。
func (s *Server) d1d3BuildRLMProjection(req ChatRequest, state *kernel.VSPStateResult, observe map[string]any) rlm.Projection {
	projectState := cloneContext(state.LegacyState)
	projectState["snapshot_hash"] = state.SnapshotHash
	projectState["project_revision"] = state.Revision
	projectState["project_epoch"] = state.ProjectEpoch
	return rlm.Build(rlm.Input{
		UserIntent:     req.Message,
		ProjectState:   projectState,
		MixObservation: observe,
		RenderBindings: s.harness.RenderProfileBindingsSnapshot(),
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339Nano),
	})
}

// d1d3AssemblePreMasterReport 是纯组装函数（单测直接喂面数据断言）：三面
// 状态归一 + limitations 汇总。缺省态为 missing，只有实际数据在场才升级；
// 缺失面如实记录，不升级不虚构。
func d1d3AssemblePreMasterReport(observe map[string]any, observeErr error, trackCandidates int, trackRows []d1d3TrackStereoRow, rlmProjection rlm.Projection, userIntent string) d1d3PreMasterReport {
	report := d1d3PreMasterReport{
		SchemaVersion:      d1d3ObservationSchema,
		ReferenceTrackGap:  d1d3ReferenceTrackGap,
		UserIntent:         userIntent,
		LoudnessPeakStatus: "missing",
		StereoStatus:       "missing",
		RLMStatus:          rlmProjection.Status,
		RLMMode:            rlmProjection.Mode,
		RLMSelectedMetric:  rlmProjection.SelectedMetric,
		RLMReferenceLevel:  rlmProjection.ReferenceLevel,
		RLMSummary:         rlmSummaryMap(rlmProjection),
		RLMLimitations:     append([]string(nil), rlmProjection.Limitations...),
		TrackStereo:        trackRows,
		TracksConsidered:   len(trackRows),
	}
	if observeErr != nil {
		report.ObserveError = observeErr.Error()
		report.Limitations = append(report.Limitations, "mix.observe 未成功返回，D1 digest/D2 全局 stereo 载体缺席（观察未完成，RLM 投影按已有输入降级）")
	} else if len(observe) == 0 {
		report.Limitations = append(report.Limitations, "mix.observe 返回空 digest（D1/D2 载体缺席，缺失如实保留）")
	}
	if len(observe) > 0 {
		report.ObservationID = firstStringFromMap(observe, "observation_id")
		if value, ok := d1d3Number(observe["peak_dbfs"]); ok {
			report.GlobalPeakDBFS = &value
		}
		if value, ok := d1d3Number(observe["rms_dbfs"]); ok {
			report.GlobalRMSDBFS = &value
		}
		if value, ok := d1d3Number(observe["crest_db"]); ok {
			report.GlobalCrestDB = &value
		}
		report.Waveform = mapValue(observe["waveform"])
		report.LoudnessRanking = mapRowsFromAny(observe["project_loudness_ranking_excerpt"])
		report.HeadroomRisk = mapRowsFromAny(observe["project_headroom_risk_excerpt"])
		report.PeakRisk = mapRowsFromAny(observe["project_peak_risk_ranking_excerpt"])
		report.MissingMetrics = stringRowsFromAny(observe["missing_metrics"])
		report.LoudnessPeakStatus = d1d3LoudnessFaceStatus(report)
		// D2 全局 stereo 面
		if observe["stereo_status"] != nil || observe["correlation_estimate"] != nil || observe["balance_db"] != nil {
			report.StereoStatus = "present"
			if value, ok := d1d3Number(observe["correlation_estimate"]); ok {
				report.CorrelationEstimate = &value
			}
			report.CorrelationState = firstStringFromMap(observe, "correlation_state")
			if value, ok := d1d3Number(observe["balance_db"]); ok {
				report.BalanceDB = &value
			}
			report.BalanceState = firstStringFromMap(observe, "balance_state")
		} else {
			report.StereoStatus = "missing"
			report.Limitations = append(report.Limitations, "全局 stereo_relation 缺席（依赖声学证据，缺失原样保留不升级）")
		}
		report.StereoSourceStatus = firstStringFromMap(mapValue(observe["available_detail"]), "stereo_relation")
	}
	// D2 逐轨面覆盖
	if trackCandidates > d1d3TrackStereoReadLimit {
		report.TracksSkipped = trackCandidates - d1d3TrackStereoReadLimit
		report.Limitations = append(report.Limitations, fmt.Sprintf("工程轨数 %d 超出单次逐轨 stereo 读取上限 %d，剩余 %d 轨未读 stereo summary", trackCandidates, d1d3TrackStereoReadLimit, trackCandidates-d1d3TrackStereoReadLimit))
	}
	for _, row := range trackRows {
		if row.MonoRisk {
			report.MonoRiskTracks++
		}
		if row.Status == "error" {
			report.Limitations = append(report.Limitations, fmt.Sprintf("轨 %s stereo summary 读取失败：%s", row.TrackID, row.Error))
		}
	}
	// D3 面
	report.RenderBindingCount = len(rlmProjection.RenderBindings)
	for _, binding := range rlmProjection.RenderBindings {
		row := map[string]any{"render_id": binding.RenderID, "profile_id": binding.ProfileID, "status": binding.Status}
		report.RenderBindings = append(report.RenderBindings, row)
	}
	if report.RenderBindingCount == 0 {
		report.Limitations = append(report.Limitations, "当前无 render→delivery 绑定（render_profile_bind 写端不在本只读 pack，绑定面如实为空）")
	}
	report.Limitations = append(report.Limitations, d1d3ReferenceTrackGap)
	if rlmProjection.Status == rlm.StatusMissing {
		report.Limitations = append(report.Limitations, "RLM 无可比参考电平指标（rlm_no_comparable_reference_level_metric，缺失如实保留）")
	}
	return report
}

// d1d3LoudnessFaceStatus 归一 D1 面 status：digest 全局值、waveform、任一
// ranking excerpt 任一在场即 present；全缺如实 missing。
func d1d3LoudnessFaceStatus(report d1d3PreMasterReport) string {
	if report.GlobalPeakDBFS != nil || report.GlobalRMSDBFS != nil || report.GlobalCrestDB != nil ||
		len(report.Waveform) > 0 || len(report.LoudnessRanking) > 0 || len(report.HeadroomRisk) > 0 || len(report.PeakRisk) > 0 {
		return "present"
	}
	return "missing"
}

func rlmSummaryMap(proj rlm.Projection) map[string]any {
	return map[string]any{
		"track_count":            proj.Summary.TrackCount,
		"eligible_track_count":   proj.Summary.EligibleTrackCount,
		"candidate_count":        proj.Summary.CandidateCount,
		"missing_metric_count":   proj.Summary.MissingMetricCount,
		"not_calibratable_count": proj.Summary.NotCalibratableCount,
		"selected_unit":          proj.Summary.SelectedUnit,
		"reference_basis":        proj.Summary.ReferenceBasis,
	}
}

func d1d3RenderPreMasterReply(report d1d3PreMasterReport) string {
	var b strings.Builder
	b.WriteString("D1-D3 母带前观察完成（schema " + d1d3ObservationSchema + "，只读）。\n")
	b.WriteString(fmt.Sprintf("D1 响度峰值面：%s", report.LoudnessPeakStatus))
	if report.GlobalPeakDBFS != nil {
		b.WriteString(fmt.Sprintf("（全局 peak %.1f dBFS", *report.GlobalPeakDBFS))
		if report.GlobalRMSDBFS != nil {
			b.WriteString(fmt.Sprintf(" / rms %.1f dBFS", *report.GlobalRMSDBFS))
		}
		if report.GlobalCrestDB != nil {
			b.WriteString(fmt.Sprintf(" / crest %.1f dB", *report.GlobalCrestDB))
		}
		b.WriteString("）")
	}
	b.WriteString(fmt.Sprintf("；loudness ranking %d 行、headroom 风险 %d 行、peak 风险 %d 行。RLM 投影 status=%s", len(report.LoudnessRanking), len(report.HeadroomRisk), len(report.PeakRisk), report.RLMStatus))
	if report.RLMSelectedMetric != "" {
		b.WriteString(fmt.Sprintf("（metric=%s", report.RLMSelectedMetric))
		if report.RLMReferenceLevel != nil {
			b.WriteString(fmt.Sprintf("，reference=%.1f", *report.RLMReferenceLevel))
		}
		b.WriteString("）")
	}
	b.WriteString("。\n")
	b.WriteString(fmt.Sprintf("D2 翻译检查面：全局 stereo %s", report.StereoStatus))
	if report.CorrelationEstimate != nil {
		b.WriteString(fmt.Sprintf("（correlation %.2f）", *report.CorrelationEstimate))
	}
	b.WriteString(fmt.Sprintf("；逐轨 stereo 读取 %d 轨，mono 风险 %d 轨。", report.TracksConsidered, report.MonoRiskTracks))
	if report.TracksSkipped > 0 {
		b.WriteString(fmt.Sprintf("另有 %d 轨超出读取上限未读。", report.TracksSkipped))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("D3 参考曲对比面：RLM user intent 已入投影，render→delivery 绑定 %d 条（fail-closed 披露）。%s。", report.RenderBindingCount, report.ReferenceTrackGap))
	b.WriteString("\n")
	b.WriteString("本次观察未创建 Proposal，未注册任何写命令，未修改工程。")
	return b.String()
}

// d1d3Number 从 any 提取有限浮点值（digest/ranking 行的数值键容错解析）。
func d1d3Number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}

func stringRowsFromAny(value any) []string {
	switch rows := value.(type) {
	case []string:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if text := strings.TrimSpace(row); text != "" {
				out = append(out, text)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if text, ok := row.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if text := strings.TrimSpace(rows); text != "" {
			return []string{text}
		}
		return nil
	default:
		return nil
	}
}
