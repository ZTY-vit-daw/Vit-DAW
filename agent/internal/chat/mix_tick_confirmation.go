package chat

import (
	"context"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func (s *Server) handlePendingMixTickChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	candidate, ok := s.pendingMixTickForConversation(conversationID)
	if !ok {
		if messageExplicitMixTickApply(req.Message) {
			if s != nil && s.logger != nil {
				s.logger.Info("[mix.tick.pending] explicit confirmation had no candidate conversation=%s message=%q", conversationID, req.Message)
			}
			if s != nil {
				s.emitAgentEvent(conversationID, AgentEvent{
					Type:     "mix_tick.confirmation.no_candidate",
					ItemType: "mix_tick",
					Status:   "missing",
					Title:    "No pending mix tick",
					Body:     "Explicit confirmation was received, but no pending mix tick candidate was available for this conversation.",
				})
			}
			return ChatResponse{
				ConversationID: conversationID,
				AgentMode:      mode,
				Reply:          "现在没有可执行的待确认混音动作。你可以让我重新观察，或者明确说要调整哪条轨道多少 dB。",
				GoalStatus:     "completed",
				StopReason:     "no_pending_mix_tick_candidate",
			}, true
		}
		return ChatResponse{}, false
	}
	switch {
	case messageExplicitMixTickApply(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] explicit confirmation routed conversation=%s track=%s delta=%+.2f observation=%s", conversationID, candidate.TrackID, candidate.DeltaDB, candidate.ObservationID)
		}
		if s != nil {
			s.emitAgentEvent(conversationID, AgentEvent{
				Type:     "mix_tick.confirmation.routed",
				ItemType: "mix_tick",
				Status:   "running",
				Title:    "Applying pending mix tick",
				Body:     fmt.Sprintf("Routing explicit confirmation through mix.propose_tick -> mix.apply_tick for track %s %+0.2f dB.", candidate.TrackID, candidate.DeltaDB),
				Payload: map[string]any{
					"operation":      candidate.Operation,
					"track_id":       candidate.TrackID,
					"delta_db":       candidate.DeltaDB,
					"observation_id": candidate.ObservationID,
				},
			})
		}
		return s.executePendingMixTickCandidate(ctx, conversationID, req, mode, candidate), true
	case messageAmbiguousMixTickApproval(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] ambiguous confirmation held conversation=%s message=%q track=%s delta=%+.2f", conversationID, req.Message, candidate.TrackID, candidate.DeltaDB)
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          fmt.Sprintf("我还没有执行。刚才待确认的是：轨道 %s %+0.2f dB。要执行请明确说“可以执行”；如果只是继续讨论，我会保持不动。", candidate.TrackID, candidate.DeltaDB),
			GoalStatus:     "completed",
			StopReason:     "ambiguous_mix_tick_confirmation",
		}, true
	case messageKeepsMixTickDiscussion(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] discussion continued without execution conversation=%s message=%q track=%s delta=%+.2f", conversationID, req.Message, candidate.TrackID, candidate.DeltaDB)
		}
		return ChatResponse{}, false
	default:
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] expired on context shift conversation=%s message=%q track=%s delta=%+.2f", conversationID, req.Message, candidate.TrackID, candidate.DeltaDB)
		}
		s.expirePendingMixTick(conversationID)
		return ChatResponse{}, false
	}
}

func (s *Server) executePendingMixTickCandidate(ctx context.Context, conversationID string, req ChatRequest, mode string, candidate agentloop.PendingMixTickCandidate) ChatResponse {
	if err := s.validatePendingMixTickCandidate(ctx, candidate); err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] candidate validation failed conversation=%s track=%s delta=%+.2f err=%v", conversationID, candidate.TrackID, candidate.DeltaDB, err)
		}
		s.expirePendingMixTick(conversationID)
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "刚才那条待确认混音动作已经不可执行：" + err.Error() + "。你可以让我重新观察后再生成一个新的小步建议。",
			GoalStatus:     "completed",
			StopReason:     "expired_pending_mix_tick_candidate",
			Error:          err.Error(),
		}
	}
	projectPath := projectPathFromChatContext(req.Context)
	goal := s.beginChatGoal(conversationID, req.Message, req.Context)
	toolContext := s.agentLoopToolContext(mode, req.Message, req.Context)
	exec := executor.New(s.harness)
	var executed []map[string]any
	runTool := func(call planner.ToolCall, confirmed bool) (executor.Result, error) {
		out, err := exec.RunToolCall(ctx, executor.Input{
			GoalID:    goal.GoalID,
			RunID:     goal.RunID,
			ToolCall:  call,
			Context:   req.Context,
			Confirmed: confirmed,
			Source:    "pending_mix_tick_confirmation",
		})
		executed = append(executed, agentLoopExecutionRecord(out))
		return out, err
	}
	proposeArgs := map[string]any{
		"operation":      "track_gain_adjust",
		"track_id":       candidate.TrackID,
		"delta_db":       candidate.DeltaDB,
		"observation_id": candidate.ObservationID,
		"evidence":       candidate.Evidence,
	}
	propose, err := runTool(planner.ToolCall{
		ID:     "propose_pending_mix_tick",
		Tool:   "mix.propose_tick",
		Args:   proposeArgs,
		Reason: "propose the pending confirmed mix tick",
	}, false)
	if err != nil || resultFailed(propose) {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] propose failed conversation=%s track=%s delta=%+.2f err=%v result_error=%s", conversationID, candidate.TrackID, candidate.DeltaDB, err, propose.Error)
		}
		return pendingMixTickErrorResponse(conversationID, mode, "生成待执行混音 tick 失败", propose, err, executed)
	}
	tickID := cleanContextText(propose.Result["tick_id"])
	if tickID == "" {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] propose returned no tick_id conversation=%s track=%s delta=%+.2f", conversationID, candidate.TrackID, candidate.DeltaDB)
		}
		return pendingMixTickErrorResponse(conversationID, mode, "生成待执行混音 tick 失败：没有 tick_id", propose, nil, executed)
	}
	apply, err := runTool(planner.ToolCall{
		ID:     "apply_pending_mix_tick",
		Tool:   "mix.apply_tick",
		Args:   map[string]any{"tick_id": tickID, "track_id": candidate.TrackID},
		Reason: "apply the confirmed pending mix tick",
	}, true)
	if err != nil || resultFailed(apply) {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] apply failed conversation=%s track=%s delta=%+.2f tick=%s err=%v result_error=%s", conversationID, candidate.TrackID, candidate.DeltaDB, tickID, err, apply.Error)
		}
		return pendingMixTickErrorResponse(conversationID, mode, "执行混音 tick 失败", apply, err, executed)
	}
	observeArgs := map[string]any{
		"track_id":             candidate.TrackID,
		"scope":                firstNonEmpty(cleanContextText(candidate.Fingerprint["target_scope"]), "selected_track"),
		"project_context":      true,
		"observation_only":     true,
		"disclosure":           "digest_catalog",
		"goal_text":            req.Message,
		"previous_observation": candidate.ObservationID,
		"mix_session_id":       firstNonEmpty(cleanContextText(candidate.Fingerprint["mix_session_id"]), "mix_"+goal.GoalID),
	}
	if strings.TrimSpace(observeArgs["scope"].(string)) == "" {
		observeArgs["scope"] = "selected_track"
	}
	observe, err := runTool(planner.ToolCall{
		ID:     "reobserve_after_mix_tick",
		Tool:   preferredMixObservationTool(toolContext.AllowedTools),
		Args:   observeArgs,
		Reason: "re-observe after applying the confirmed mix tick",
	}, true)
	if err != nil || resultFailed(observe) {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] reobserve failed conversation=%s track=%s delta=%+.2f tick=%s err=%v result_error=%s", conversationID, candidate.TrackID, candidate.DeltaDB, tickID, err, observe.Error)
		}
		s.expirePendingMixTick(conversationID)
		reply := pendingMixTickReport(candidate, propose, apply, observe, "已执行，但重新观察失败。你仍然可以用项目撤销或 mix.rollback_tick 回滚。")
		return ChatResponse{
			ConversationID:      conversationID,
			AgentMode:           mode,
			GoalID:              goal.GoalID,
			RunID:               goal.RunID,
			Reply:               reply,
			ExecutedKernelReply: executed,
			ProjectResultCards:  projectResultCardsFromExecuted(executed),
			GoalStatus:          "completed",
			StopReason:          "mix_tick_applied_reobserve_failed",
			ProjectHistory:      s.harness.ProjectHistorySummaryForProject(ctx, goal.GoalID, projectPath),
		}
	}
	s.expirePendingMixTick(conversationID)
	if s != nil && s.logger != nil {
		s.logger.Info("[mix.tick.pending] applied and reobserved conversation=%s track=%s delta=%+.2f tick=%s", conversationID, candidate.TrackID, candidate.DeltaDB, tickID)
	}
	return ChatResponse{
		ConversationID:      conversationID,
		AgentMode:           mode,
		GoalID:              goal.GoalID,
		RunID:               goal.RunID,
		Reply:               pendingMixTickReport(candidate, propose, apply, observe, ""),
		ExecutedKernelReply: executed,
		ProjectResultCards:  projectResultCardsFromExecuted(executed),
		Artifacts:           artifactSummariesFromExecuted(executed),
		GoalStatus:          "completed",
		GoalSummary:         req.Message,
		CompletedSteps:      3,
		StopReason:          "mix_tick_applied_reobserved",
		ProjectHistory:      s.harness.ProjectHistorySummaryForProject(ctx, goal.GoalID, projectPath),
	}
}

func (s *Server) pendingMixTickForConversation(conversationID string) (agentloop.PendingMixTickCandidate, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return agentloop.PendingMixTickCandidate{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate, ok := s.pendingMixTicks[conversationID]
	if !ok || !strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
		return agentloop.PendingMixTickCandidate{}, false
	}
	return candidate, true
}

func (s *Server) expirePendingMixTick(conversationID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingMixTicks, conversationID)
}

func (s *Server) validatePendingMixTickCandidate(ctx context.Context, candidate agentloop.PendingMixTickCandidate) error {
	if !strings.EqualFold(strings.TrimSpace(candidate.Operation), "track_gain_adjust") {
		return fmt.Errorf("v1 只支持 track_gain_adjust")
	}
	if strings.TrimSpace(candidate.TrackID) == "" {
		return fmt.Errorf("缺少 track_id")
	}
	if candidate.DeltaDB == 0 || math.Abs(candidate.DeltaDB) > 2 {
		return fmt.Errorf("delta_db 超出 +/-2 dB 范围")
	}
	if !strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
		return fmt.Errorf("状态不是 pending_confirmation")
	}
	if s != nil && s.harness != nil {
		state := s.harness.UserStateSummary(ctx)
		rows := chatVisibleTrackRows(state)
		row, found := pendingMixTickTrackRow(rows, candidate.TrackID)
		if !found {
			return fmt.Errorf("目标轨道已经不在当前工程状态里")
		}
		if candidate.ExpiresAfterContextChange {
			if wantCount := intNumber(candidate.Fingerprint["track_count"]); wantCount > 0 && len(rows) > 0 && len(rows) != wantCount {
				return fmt.Errorf("工程轨道数量已经变化")
			}
			if wantGain, ok := pendingMixTickFingerprintFloat(candidate.Fingerprint, "track_gain_db"); ok {
				if currentGain, ok := pendingMixTickTrackVolumeDB(row); ok && math.Abs(currentGain-wantGain) > 0.01 {
					return fmt.Errorf("目标轨道音量已经变化")
				}
			}
		}
	}
	return nil
}

func pendingMixTickTrackRow(rows []map[string]any, trackID string) (map[string]any, bool) {
	for _, row := range rows {
		if firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["id"])) == trackID {
			return row, true
		}
	}
	return nil, false
}

func pendingMixTickTrackVolumeDB(row map[string]any) (float64, bool) {
	if row == nil {
		return 0, false
	}
	for _, key := range []string{"volume_db", "gain_db", "fader_db", "db"} {
		if value, ok := row[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
			return mixFloatNumber(value), true
		}
	}
	return 0, false
}

func pendingMixTickFingerprintFloat(fingerprint map[string]any, key string) (float64, bool) {
	if fingerprint == nil {
		return 0, false
	}
	value, ok := fingerprint[key]
	if !ok || strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
		return 0, false
	}
	return mixFloatNumber(value), true
}

func messageExplicitMixTickApply(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return textHasAny(text,
		"可以执行", "确认执行", "执行吧", "执行这个", "应用这个", "应用调整", "按这个调", "按你说的调", "就按这个调", "开始执行",
		"apply it", "apply this", "do it", "execute it", "confirm and apply", "go ahead and apply",
	)
}

func messageClearlyShiftsMixTickContext(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	if textHasAny(text, "为什么", "为啥", "原因", "解释", "继续说", "继续分析", "还有什么", "why", "explain", "continue discussing") {
		return false
	}
	return textHasAny(text,
		"先别", "不要执行", "别执行", "取消", "换", "整体混音", "全工程", "看整体", "鼓组", "主唱", "人声", "压缩", "eq", "均衡", "混响",
		"cancel", "don't apply", "do not apply", "overall mix", "full project", "vocal", "compress", "reverb",
	)
}

func messageAmbiguousMixTickApproval(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	switch text {
	case "可以", "好", "行", "ok", "okay", "yes", "继续", "需要":
		return true
	default:
		return false
	}
}

func messageKeepsMixTickDiscussion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return textHasAny(text,
		"为什么", "为啥", "原因", "解释", "继续说", "继续分析", "还有什么", "怎么判断", "风险", "会怎样", "听感",
		"why", "explain", "continue discussing", "tell me more", "what else", "risk",
	)
}

func preferredMixObservationTool(allowed []string) string {
	for _, tool := range allowed {
		if strings.TrimSpace(tool) == "mix.observe" {
			return "mix.observe"
		}
	}
	return "mix.request_observation"
}

func agentLoopExecutionRecord(out executor.Result) map[string]any {
	return map[string]any{
		"status":          out.Status,
		"tool_call_id":    out.ToolCallID,
		"agent_action_id": out.AgentActionID,
		"tool":            out.Tool,
		"command_name":    out.CommandName,
		"preview":         out.Preview,
		"undo_label":      out.UndoLabel,
		"result":          out.Result,
		"error":           out.Error,
	}
}

func resultFailed(out executor.Result) bool {
	status := strings.ToLower(strings.TrimSpace(out.Status))
	return out.Error != "" || status == "error" || status == "kernel_error" || status == "failed"
}

func pendingMixTickErrorResponse(conversationID, mode, prefix string, out executor.Result, err error, executed []map[string]any) ChatResponse {
	message := firstNonEmpty(out.Error, cleanContextText(out.Result["error"]))
	if message == "" && err != nil {
		message = err.Error()
	}
	return ChatResponse{
		ConversationID:      conversationID,
		AgentMode:           mode,
		Reply:               prefix + "：" + firstNonEmpty(message, "未知错误"),
		ExecutedKernelReply: executed,
		GoalStatus:          "failed",
		StopReason:          "mix_tick_confirmation_failed",
		Error:               firstNonEmpty(message, "mix tick failed"),
	}
}

func pendingMixTickReport(candidate agentloop.PendingMixTickCandidate, propose, apply, observe executor.Result, suffix string) string {
	before := firstNonEmpty(cleanContextText(apply.Result["before_db"]), cleanContextText(propose.Result["before_db"]))
	after := firstNonEmpty(cleanContextText(apply.Result["after_db"]), cleanContextText(propose.Result["after_db"]))
	headroom := firstNonEmpty(
		cleanContextText(mapValue(observe.Result["acoustic_digest"])["headroom_db"]),
		cleanContextText(propose.Result["project_headroom_db"]),
	)
	lines := []string{
		fmt.Sprintf("已执行：轨道 %s 音量 %+0.2f dB。", candidate.TrackID, candidate.DeltaDB),
	}
	if before != "" || after != "" {
		lines = append(lines, fmt.Sprintf("执行前后音量：%s dB -> %s dB。", firstNonEmpty(before, "未知"), firstNonEmpty(after, "未知")))
	}
	if headroom != "" {
		lines = append(lines, "重新观察后 headroom 参考值："+headroom+" dB。")
	}
	lines = append(lines, "如果听感不对，可以用撤销或 mix.rollback_tick 回滚这一步。")
	if strings.TrimSpace(suffix) != "" {
		lines = append(lines, suffix)
	}
	return strings.Join(lines, "\n")
}

func textHasAny(text string, needles ...string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}
