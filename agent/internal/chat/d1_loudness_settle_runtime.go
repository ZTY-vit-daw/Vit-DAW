package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/loudnesswatch"
	agentruntime "vit-daw-agent/internal/runtime"
)

// AF-D1-LOUDNESS-SETTLE-1：D1 全局响度与峰值·settle 面。观察载体复用
// pre_master.observation.v0 pack（mix.observe digest 的全局 peak/rms/crest +
// loudness_summaries rows），判断底座是 loudnesswatch 包；本文件只补三件事：
//
//  1. 显式检查（d1_settle 触发）：按 watch 配置（未配置时按内置流媒体目标
//     一次性判断，不写配置面）跑一次 Compare，verdict 落 loudness_watch
//     verdict store（dedupe 同规）；
//  2. 待确认动作面：breach 且能定位最越界载体时，生成一条有界推子建议并挂
//     既有 mix_tick 确认链（mix.propose_tick → 用户确认 → mix.apply_tick）。
//     建议不等于写入：确认前零 mutation；完全访问模式按 improvement 先例
//     （2026-09-11 用户改判）直接执行同一治理链；
//  3. D1 点亮：queue view 的 D1 节点从 not_started 起步——检查跑过即
//     in_progress，挂了确认卡即 needs_confirmation（annotation 面；completed
//     留给账本证据推导，annotation 不命令完成）。
//
// 判断权冻结令：本文件无 LLM 参与；固定文案只做识别/执行/兜底，阈值判断归
// loudnesswatch.Compare 的纯函数口径。
const d1LoudnessSettleCapabilityID = "pre_master.loudness_settle.v0"

// d1SettleDefaultProfile 是 watch 未配置时一次性判断用的内置目标（Spotify
// 流媒体口径：integrated ≤ -14 LUFS，true peak ≤ -1 dBTP）。
const d1SettleDefaultProfile = "builtin:spotify"

// d1LoudnessSettleIntent reports whether the message asks for the explicit D1
// loudness/peak check face. Explicit D markers already route here through the
// D1-D3 observation entry; the natural phrases cover the same request said
// plainly.
func d1LoudnessSettleIntent(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return textHasAny(text,
		"全局响度", "响度检查", "响度峰值", "峰值检查", "母带前响度", "母带前检查",
		"loudness check", "loudness settle", "global loudness",
	)
}

// handleD1LoudnessSettle runs the D1 settle face on top of the already-built
// D1-D3 observation report. read_only for the project: the only state this
// face writes is the loudness watch verdict store (observation bookkeeping)
// and, on a breach, a pending-confirmation candidate through the governed
// mix-tick chain.
func (s *Server) handleD1LoudnessSettle(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, state *kernel.VSPStateResult, report d1d3PreMasterReport) ChatResponse {
	rows := readLoudnessSummariesFromFeatureSnapshot()
	settle := s.d1SettleEvaluate(ctx, state, sessionID, rows, report)

	workflowData := map[string]any{
		"session_id":          sessionID,
		"capability_id":       d1LoudnessSettleCapabilityID,
		"canary_stage":        "d1_settle",
		"read_only":           true,
		"mutation_performed":  false,
		"schema":              d1d3ObservationSchema,
		"observation":         report,
		"d1_settle":           settle.payload(),
		"observation_carrier": premasterObservationCapabilityID,
	}

	response := ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          settle.reply(report),
		Workflow:       "capability_runtime_v1",
		WorkflowData:   workflowData,
		GoalStatus:     string(agentruntime.StatusCompleted),
		StopReason:     "d1_loudness_settled",
	}

	// 动作面：breach + 可定位载体 → 一条有界推子建议挂既有确认链。
	if settle.suggestion != nil {
		candidate := d1SettlePendingCandidate(settle, report, conversationID)
		autoApply := authorityModeFromContext(req.Context) == experiment.AuthorityFull
		s.storePendingMixTickCandidateForMode(conversationID, goal.GoalID, goal.RunID, candidate, autoApply)
		if autoApply {
			executed := s.executePendingMixTickCandidate(ctx, conversationID, req, agentModeFromContext(req.Context), candidate)
			executed.WorkflowData = mergeContext(executed.WorkflowData, map[string]any{
				"capability_id": d1LoudnessSettleCapabilityID, "full_access_auto_authorized": true,
			})
			s.writeD1LoudnessSettleQueueNode(conversationID, settle, "in_progress")
			return executed
		}
		request := mixTickInteractionRequest(conversationID, goal.GoalID, goal.RunID, candidate)
		s.storePendingInteraction(request, request.Payload)
		response.InteractionRequests = append(response.InteractionRequests, request)
		response.NeedsConfirmation = true
		workflowData["pending_action"] = settle.suggestion
		workflowData["mutation_performed"] = false
	}

	// 点亮：检查已跑 = in_progress；挂了卡 = needs_confirmation（上方手动分支
	// 已在挂卡路径写 needs_confirmation，此处覆盖 auto-apply 与无卡路径）。
	nodeState := "in_progress"
	if settle.suggestion != nil && authorityModeFromContext(req.Context) != experiment.AuthorityFull {
		nodeState = "needs_confirmation"
	}
	s.writeD1LoudnessSettleQueueNode(conversationID, settle, nodeState)
	return response
}

// d1SettleOutcome carries one D1 settle evaluation's results for the response
// assembly.
type d1SettleOutcome struct {
	verdict     *loudnesswatch.Verdict
	configBasis string
	checked     bool
	suggestion  *loudnesswatch.ActionSuggestion
	storeKey    string
}

// d1SettleEvaluate runs the explicit check: effective config (watch config or
// the builtin streaming default), one Compare through the shared runtime
// (verdict lands in the store under the d1_settle trigger), then the bounded
// action suggestion for a breach.
func (s *Server) d1SettleEvaluate(ctx context.Context, state *kernel.VSPStateResult, sessionID string, rows []map[string]any, report d1d3PreMasterReport) d1SettleOutcome {
	outcome := d1SettleOutcome{}
	config, basis, usable := s.d1SettleEffectiveConfig()
	outcome.configBasis = basis
	if !usable {
		// Realtime config: stored but never active — disclose the gap, no verdict.
		return outcome
	}
	adapter, ok := s.loudnessWatchHook().(*loudnessWatcherAdapter)
	if !ok || adapter == nil {
		return outcome
	}
	projectUUID, projectRevision := "", ""
	if state != nil && state.Revision > 0 {
		projectRevision = fmt.Sprintf("%d", state.Revision)
	}
	if roots := s.d1SettleProjectIdentity(ctx); roots != "" {
		projectUUID = roots
	}
	verdict, checked := adapter.runtime.CheckWithConfig(loudnesswatch.TriggerContext{
		ProjectUUID:     projectUUID,
		ProjectRevision: projectRevision,
		TriggerKind:     loudnesswatch.TriggerKindD1Settle,
		TriggerRef:      sessionID,
		AllowBake:       true,
	}, config)
	outcome.checked = checked
	if !checked {
		return outcome
	}
	verdictCopy := verdict
	outcome.verdict = &verdictCopy
	outcome.storeKey = loudnesswatch.DedupeKey(verdict.ProjectUUID, verdict.ProjectRevision, verdict.Verdict)
	outcome.suggestion = loudnesswatch.SuggestGainAction(verdict, rows, experiment.D1S1MaxAbsDeltaDB)
	return outcome
}

// d1SettleEffectiveConfig resolves the check target: the project's watch
// config when one exists, otherwise the builtin streaming default for one
// honest on-demand check (the config face stays untouched). usable=false
// marks the realtime capability gap.
func (s *Server) d1SettleEffectiveConfig() (config loudnesswatch.Config, basis string, usable bool) {
	s.mu.Lock()
	stored := s.loudnessWatchConfigCopyLocked()
	s.mu.Unlock()
	if stored != nil {
		config = *stored
		if config.TriggerMode == loudnesswatch.TriggerRealtime {
			return config, "watch 配置为 realtime（无内核 producer，仅存储）", false
		}
		return config, "工程 loudness watch 配置（" + config.ProfileSource + "）", true
	}
	config = loudnesswatch.Normalize(loudnesswatch.Config{
		Enabled:       true,
		TriggerMode:   loudnesswatch.TriggerAfterProcessing,
		ProfileSource: d1SettleDefaultProfile,
	})
	return config, "内置流媒体目标（" + d1SettleDefaultProfile + "，watch 未配置，一次性判断不写配置面）", true
}

// d1SettleProjectIdentity resolves the current project uuid through the same
// seams the conclusion bridge uses.
func (s *Server) d1SettleProjectIdentity(ctx context.Context) string {
	if s != nil && s.harness != nil {
		if _, projectUUID := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(projectUUID) != "" {
			return strings.TrimSpace(projectUUID)
		}
	}
	if bridge := s.freeStateConclusionBridgeWithHarness(ctx, nil, ""); bridge.ProjectUUID != "" {
		return bridge.ProjectUUID
	}
	return ""
}

// d1SettlePendingCandidate builds the governed pending-confirmation candidate
// from the breach suggestion (v1 domain: bounded track_gain_adjust).
func d1SettlePendingCandidate(outcome d1SettleOutcome, report d1d3PreMasterReport, conversationID string) agentloop.PendingMixTickCandidate {
	suggestion := outcome.suggestion
	evidence := map[string]any{
		"source":          "d1_loudness_settle",
		"capability_id":   d1LoudnessSettleCapabilityID,
		"verdict_kind":    outcome.verdictKind(),
		"metric":          suggestion.Metric,
		"measured":        suggestion.Measured,
		"threshold":       suggestion.Threshold,
		"breach_delta_db": suggestion.Measured - suggestion.Threshold,
		"config_basis":    outcome.configBasis,
	}
	if outcome.verdict != nil {
		evidence["verdict_summary"] = outcome.verdict.Summary
		evidence["clip_id"] = suggestion.ClipID
	}
	fingerprint := map[string]any{
		"conversation_id": conversationID,
		"target_scope":    "selected_track",
		"mix_session_id":  "d1_settle_" + report.ObservationID,
		"config_basis":    outcome.configBasis,
	}
	return agentloop.PendingMixTickCandidate{
		Operation:                 loudnesswatch.ActionOperationGainAdjust,
		TrackID:                   suggestion.TrackID,
		DeltaDB:                   suggestion.DeltaDB,
		ObservationID:             report.ObservationID,
		Evidence:                  evidence,
		CreatedFromReply:          "D1 全局响度与峰值 settle 建议",
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               fingerprint,
	}
}

// writeD1LoudnessSettleQueueNode advances the workflow queue's D1 node out of
// not_started. Annotation-face discipline: in_progress / needs_confirmation
// only — completion stays ledger-derived (M1-2 red line). A write failure
// WARNs and never blocks the settle response.
func (s *Server) writeD1LoudnessSettleQueueNode(conversationID string, outcome d1SettleOutcome, nodeState string) {
	reason := "D1 全局响度与峰值检查已执行（" + outcome.verdictKind() + "）"
	evidence := []string{"d1_settle:" + d1LoudnessSettleCapabilityID}
	if outcome.storeKey != "" {
		evidence = append(evidence, "loudness_watch_verdict:"+outcome.storeKey)
	}
	nodeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.WriteMixWorkflowQueueNodeState(nodeCtx, "D1", nodeState, reason, evidence, conversationID); err != nil {
		if s.logger != nil {
			s.logger.Warn("[d1.loudness_settle] queue node annotation failed conversation=%s err=%v", conversationID, err)
		}
	} else if s.logger != nil {
		s.logger.Info("[d1.loudness_settle] D1 node advanced conversation=%s state=%s verdict=%s", conversationID, nodeState, outcome.verdictKind())
	}
}

func (o d1SettleOutcome) verdictKind() string {
	if o.verdict == nil || strings.TrimSpace(o.verdict.Verdict) == "" {
		return "unmeasurable"
	}
	return o.verdict.Verdict
}

// payload is the structured settle disclosure for WorkflowData.
func (o d1SettleOutcome) payload() map[string]any {
	payload := map[string]any{
		"capability_id": d1LoudnessSettleCapabilityID,
		"config_basis":  o.configBasis,
		"checked":       o.checked,
	}
	if o.verdict != nil {
		payload["verdict"] = o.verdict.Verdict
		payload["verdict_summary"] = o.verdict.Summary
		payload["peak_check"] = o.verdict.PeakCheck
		payload["measured_clips"] = len(o.verdict.Measured)
		if len(o.verdict.BreachDetails) > 0 {
			payload["breach_count"] = len(o.verdict.BreachDetails)
		}
		if o.verdict.UnmeasurableReason != "" {
			payload["unmeasurable_reason"] = o.verdict.UnmeasurableReason
		}
	}
	if o.suggestion != nil {
		payload["suggestion"] = map[string]any{
			"operation": o.suggestion.Operation, "track_id": o.suggestion.TrackID,
			"delta_db": o.suggestion.DeltaDB, "metric": o.suggestion.Metric, "basis": o.suggestion.Basis,
		}
	}
	return payload
}

// reply renders the user-visible settle report: observation globals, verdict
// summary (with the approximate qualifier the comparator pinned), and the
// confirmation boundary when a suggestion is parked.
func (o d1SettleOutcome) reply(report d1d3PreMasterReport) string {
	lines := []string{"D1 全局响度与峰值检查（loudness settle，schema " + d1d3ObservationSchema + "）。"}
	if report.GlobalPeakDBFS != nil || report.GlobalRMSDBFS != nil || report.GlobalCrestDB != nil {
		globals := "全局观察（mix.observe digest）："
		parts := []string{}
		if report.GlobalPeakDBFS != nil {
			parts = append(parts, fmt.Sprintf("peak %.1f dBFS", *report.GlobalPeakDBFS))
		}
		if report.GlobalRMSDBFS != nil {
			parts = append(parts, fmt.Sprintf("rms %.1f dBFS", *report.GlobalRMSDBFS))
		}
		if report.GlobalCrestDB != nil {
			parts = append(parts, fmt.Sprintf("crest %.1f dB", *report.GlobalCrestDB))
		}
		lines = append(lines, globals+strings.Join(parts, " / ")+"。")
	} else {
		lines = append(lines, "全局观察（mix.observe digest）：峰值/响度域值缺席（缺失如实保留）。")
	}
	lines = append(lines, "判断口径："+o.configBasis+"。")
	if !o.checked {
		lines = append(lines, "本次未执行判断：watch 配置为 realtime，无内核实时 producer（能力缺口如实披露；可改用 after_processing 或 periodic 模式）。")
	} else if o.verdict != nil {
		lines = append(lines, "判断结果："+o.verdict.Summary)
	}
	if o.suggestion != nil {
		lines = append(lines, "修复建议（待确认，未改工程）："+o.suggestion.Basis+"。")
		lines = append(lines, "确认后我会通过 mix.propose_tick → mix.apply_tick 执行这一步；确认前不会修改工程。")
	} else if o.checked {
		lines = append(lines, "本次检查没有生成修复建议（无越界或无法定位载体）。")
	}
	lines = append(lines, "检查结果已记入 loudness watch verdict 记录；D1 节点状态可在队列视图查看。")
	return strings.Join(lines, "\n")
}
