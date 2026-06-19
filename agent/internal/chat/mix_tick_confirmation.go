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
	case messageKeepsMixTickDiscussion(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] discussion continued without execution conversation=%s message=%q %s", conversationID, req.Message, pendingMixTickLogSummary(candidate))
		}
		return ChatResponse{}, false
	case messageClearlyShiftsMixTickContext(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] expired on context shift conversation=%s message=%q %s", conversationID, req.Message, pendingMixTickLogSummary(candidate))
		}
		s.expirePendingMixTick(conversationID)
		return ChatResponse{}, false
	case messageExplicitMixTickApply(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] explicit confirmation routed conversation=%s %s observation=%s", conversationID, pendingMixTickLogSummary(candidate), candidate.ObservationID)
		}
		if s != nil {
			s.emitAgentEvent(conversationID, AgentEvent{
				Type:     "mix_tick.confirmation.routed",
				ItemType: "mix_tick",
				Status:   "running",
				Title:    "Applying pending mix tick",
				Body:     "Routing explicit confirmation through mix.propose_tick -> mix.apply_tick for " + pendingMixTickHumanSummary(candidate) + ".",
				Payload:  pendingMixTickEventPayload(candidate, candidate.ObservationID),
			})
		}
		return s.executePendingMixTickCandidate(ctx, conversationID, req, mode, candidate), true
	case messageAmbiguousMixTickApproval(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] ambiguous confirmation held conversation=%s message=%q %s", conversationID, req.Message, pendingMixTickLogSummary(candidate))
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          fmt.Sprintf("我还没有执行。刚才待确认的是：%s。要执行请明确说“可以执行”；如果只是继续讨论，我会保持不动。", pendingMixTickHumanSummary(candidate)),
			GoalStatus:     "completed",
			StopReason:     "ambiguous_mix_tick_confirmation",
		}, true
	default:
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.tick.pending] expired on context shift conversation=%s message=%q %s", conversationID, req.Message, pendingMixTickLogSummary(candidate))
		}
		s.expirePendingMixTick(conversationID)
		return ChatResponse{}, false
	}
}

func (s *Server) executePendingMixTickCandidate(ctx context.Context, conversationID string, req ChatRequest, mode string, candidate agentloop.PendingMixTickCandidate) ChatResponse {
	if err := s.validatePendingMixTickCandidate(ctx, candidate); err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] candidate validation failed conversation=%s %s err=%v", conversationID, pendingMixTickLogSummary(candidate), err)
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
		"operation":      candidate.Operation,
		"track_id":       candidate.TrackID,
		"observation_id": candidate.ObservationID,
		"evidence":       candidate.Evidence,
	}
	if strings.EqualFold(candidate.Operation, "track_pan_set") {
		if candidate.TargetPan != nil {
			proposeArgs["pan"] = *candidate.TargetPan
		}
	} else if strings.EqualFold(candidate.Operation, "track_pan_adjust") {
		proposeArgs["delta_pan"] = candidate.DeltaPan
	} else {
		proposeArgs["delta_db"] = candidate.DeltaDB
	}
	propose, err := runTool(planner.ToolCall{
		ID:     "propose_pending_mix_tick",
		Tool:   "mix.propose_tick",
		Args:   proposeArgs,
		Reason: "propose the pending confirmed mix tick",
	}, false)
	if err != nil || resultFailed(propose) {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] propose failed conversation=%s %s err=%v result_error=%s", conversationID, pendingMixTickLogSummary(candidate), err, propose.Error)
		}
		return pendingMixTickErrorResponse(conversationID, mode, "生成待执行混音 tick 失败", propose, err, executed)
	}
	tickID := cleanContextText(propose.Result["tick_id"])
	if tickID == "" {
		if s != nil && s.logger != nil {
			s.logger.Warn("[mix.tick.pending] propose returned no tick_id conversation=%s %s", conversationID, pendingMixTickLogSummary(candidate))
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
			s.logger.Warn("[mix.tick.pending] apply failed conversation=%s %s tick=%s err=%v result_error=%s", conversationID, pendingMixTickLogSummary(candidate), tickID, err, apply.Error)
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
			s.logger.Warn("[mix.tick.pending] reobserve failed conversation=%s %s tick=%s err=%v result_error=%s", conversationID, pendingMixTickLogSummary(candidate), tickID, err, observe.Error)
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
	nextCandidate, hasNext := nextPendingMixTickCandidateFromReobserve(conversationID, goal.GoalID, goal.RunID, candidate, observe)
	nextSuffix := ""
	if hasNext {
		s.storePendingMixTickCandidate(conversationID, goal.GoalID, goal.RunID, nextCandidate)
		nextSuffix = pendingMixTickNextCandidateSuffix(nextCandidate)
	}
	if s != nil && s.logger != nil {
		s.logger.Info("[mix.tick.pending] applied and reobserved conversation=%s %s tick=%s", conversationID, pendingMixTickLogSummary(candidate), tickID)
	}
	return ChatResponse{
		ConversationID:      conversationID,
		AgentMode:           mode,
		GoalID:              goal.GoalID,
		RunID:               goal.RunID,
		Reply:               pendingMixTickReport(candidate, propose, apply, observe, nextSuffix),
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

func (s *Server) storePendingMixTickCandidate(conversationID, goalID, runID string, candidate agentloop.PendingMixTickCandidate) {
	if s == nil || strings.TrimSpace(conversationID) == "" || !strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
		return
	}
	s.mu.Lock()
	if s.pendingMixTicks == nil {
		s.pendingMixTicks = map[string]agentloop.PendingMixTickCandidate{}
	}
	s.pendingMixTicks[conversationID] = candidate
	s.mu.Unlock()
	if s.logger != nil {
		s.logger.Info("[mix.tick.pending] stored conversation=%s goal=%s run=%s %s observation=%s",
			conversationID, goalID, runID, pendingMixTickLogSummary(candidate), candidate.ObservationID)
	}
	s.emitAgentEvent(conversationID, AgentEvent{
		Type:     "mix_tick.pending",
		GoalID:   goalID,
		RunID:    runID,
		ItemType: "mix_tick",
		Status:   "pending_confirmation",
		Title:    "Mix tick pending confirmation",
		Body:     pendingMixTickEventBody(candidate),
		Payload:  pendingMixTickEventPayload(candidate, candidate.ObservationID),
	})
}

func (s *Server) validatePendingMixTickCandidate(ctx context.Context, candidate agentloop.PendingMixTickCandidate) error {
	switch strings.TrimSpace(candidate.Operation) {
	case "track_gain_adjust", "track_pan_adjust", "track_pan_set":
	default:
		return fmt.Errorf("v1 只支持 track_gain_adjust, track_pan_adjust, track_pan_set")
	}
	if strings.TrimSpace(candidate.TrackID) == "" {
		return fmt.Errorf("缺少 track_id")
	}
	switch strings.TrimSpace(candidate.Operation) {
	case "track_gain_adjust":
		if candidate.DeltaDB == 0 || math.Abs(candidate.DeltaDB) > 2 {
			return fmt.Errorf("delta_db 超出 +/-2 dB 范围")
		}
	case "track_pan_adjust":
		if candidate.DeltaPan == 0 || math.Abs(candidate.DeltaPan) > 0.15 {
			return fmt.Errorf("delta_pan 超出 +/-0.15 范围")
		}
	case "track_pan_set":
		if candidate.TargetPan == nil || *candidate.TargetPan < -1 || *candidate.TargetPan > 1 {
			return fmt.Errorf("target_pan 超出 -1 到 +1 范围")
		}
	}
	if candidate.DeltaDB == 0 && strings.EqualFold(strings.TrimSpace(candidate.Operation), "track_gain_adjust") {
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
			if wantPan, ok := pendingMixTickFingerprintFloat(candidate.Fingerprint, "track_pan"); ok {
				if currentPan, ok := pendingMixTickTrackPan(row); ok && math.Abs(currentPan-wantPan) > 0.001 {
					return fmt.Errorf("目标轨道声像已经变化")
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

func pendingMixTickTrackPan(row map[string]any) (float64, bool) {
	if row == nil {
		return 0, false
	}
	for _, key := range []string{"pan", "pan_value", "balance"} {
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

func pendingMixTickLogSummary(candidate agentloop.PendingMixTickCandidate) string {
	return pendingMixTickHumanSummary(candidate)
}

func pendingMixTickHumanSummary(candidate agentloop.PendingMixTickCandidate) string {
	switch strings.TrimSpace(candidate.Operation) {
	case "track_pan_adjust":
		return fmt.Sprintf("track %s pan %+0.2f", candidate.TrackID, candidate.DeltaPan)
	case "track_pan_set":
		if candidate.TargetPan != nil {
			return fmt.Sprintf("track %s pan set to %+0.2f", candidate.TrackID, *candidate.TargetPan)
		}
		return fmt.Sprintf("track %s pan set", candidate.TrackID)
	default:
		return fmt.Sprintf("track %s %+0.2f dB", candidate.TrackID, candidate.DeltaDB)
	}
}

func pendingMixTickEventBody(candidate agentloop.PendingMixTickCandidate) string {
	return "Track " + candidate.TrackID + " " + pendingMixTickEventActionText(candidate) + " is waiting for explicit confirmation."
}

func pendingMixTickEventActionText(candidate agentloop.PendingMixTickCandidate) string {
	switch strings.TrimSpace(candidate.Operation) {
	case "track_pan_adjust":
		return fmt.Sprintf("pan %+0.2f", candidate.DeltaPan)
	case "track_pan_set":
		if candidate.TargetPan != nil {
			return fmt.Sprintf("pan set to %+0.2f", *candidate.TargetPan)
		}
		return "pan set"
	default:
		return fmt.Sprintf("%+0.2f dB", candidate.DeltaDB)
	}
}

func pendingMixTickEventPayload(candidate agentloop.PendingMixTickCandidate, observationID string) map[string]any {
	payload := map[string]any{
		"operation":      candidate.Operation,
		"track_id":       candidate.TrackID,
		"observation_id": observationID,
	}
	switch strings.TrimSpace(candidate.Operation) {
	case "track_pan_adjust":
		payload["delta_pan"] = candidate.DeltaPan
	case "track_pan_set":
		if candidate.TargetPan != nil {
			payload["target_pan"] = *candidate.TargetPan
		}
	default:
		payload["delta_db"] = candidate.DeltaDB
	}
	return payload
}

func messageExplicitMixTickApply(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	switch text {
	case "可以执行", "确认", "确认执行", "继续", "可以继续", "可以，继续", "可以, 继续", "执行", "应用", "apply", "do it", "continue":
		return true
	}
	return textHasAny(text,
		"可以执行", "可以继续", "可以，继续", "可以, 继续", "确认执行", "执行吧", "执行这个", "应用这个", "应用调整", "按这个调", "按你说的调", "就按这个调", "开始执行",
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
		"cancel", "don't apply", "do not apply", "overall mix", "full project", "track ", "vocal", "compress", "reverb",
	)
}

func messageAmbiguousMixTickApproval(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	switch text {
	case "可以", "好", "行", "ok", "okay", "yes", "需要":
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

func nextPendingMixTickCandidateFromReobserve(conversationID, goalID, runID string, previous agentloop.PendingMixTickCandidate, observe executor.Result) (agentloop.PendingMixTickCandidate, bool) {
	if resultFailed(observe) {
		return agentloop.PendingMixTickCandidate{}, false
	}
	row, ok := nextPendingMixTickRiskRow(observe, previous.TrackID)
	if !ok {
		return agentloop.PendingMixTickCandidate{}, false
	}
	trackID := firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["id"]), cleanContextText(row["target_track_id"]))
	if trackID == "" {
		return agentloop.PendingMixTickCandidate{}, false
	}
	observationID := firstNonEmpty(
		cleanContextText(observe.Result["observation_id"]),
		cleanContextText(mapValue(observe.Result["observation"])["observation_id"]),
		cleanContextText(mapValue(observe.Result["digest"])["observation_id"]),
		previous.ObservationID,
	)
	fingerprint := map[string]any{
		"conversation_id":   conversationID,
		"goal_id":           goalID,
		"run_id":            runID,
		"target_track_id":   trackID,
		"observation_id":    observationID,
		"target_scope":      "selected_track",
		"track_count":       nextPendingMixTickTrackCount(observe, previous),
		"mix_session_id":    firstNonEmpty(cleanContextText(mapValue(observe.Result["observation"])["mix_session_id"]), cleanContextText(observe.Result["mix_session_id"]), cleanContextText(previous.Fingerprint["mix_session_id"])),
		"previous_track_id": previous.TrackID,
	}
	if len(row) > 0 {
		fingerprint["before_track"] = row
		for _, key := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"} {
			if value, ok := row[key]; ok && value != nil {
				fingerprint[key] = value
			}
		}
	}
	return agentloop.PendingMixTickCandidate{
		Operation:                 "track_gain_adjust",
		TrackID:                   trackID,
		DeltaDB:                   -1,
		ObservationID:             observationID,
		Evidence:                  nextPendingMixTickEvidence(row, previous),
		CreatedFromReply:          "",
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               fingerprint,
	}, true
}

func nextPendingMixTickRiskRow(observe executor.Result, previousTrackID string) (map[string]any, bool) {
	previousTrackID = strings.TrimSpace(previousTrackID)
	for _, row := range nextPendingMixTickRiskRows(observe) {
		trackID := firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["id"]), cleanContextText(row["target_track_id"]))
		if trackID == "" {
			continue
		}
		risk, ok := nextPendingMixTickHeadroomRisk(row)
		if !ok || pendingMixTickRiskRank(risk) < pendingMixTickRiskRank("high") {
			continue
		}
		if previousTrackID == "" || trackID != previousTrackID {
			return row, true
		}
	}
	return nil, false
}

func nextPendingMixTickRiskRows(observe executor.Result) []map[string]any {
	var rows []map[string]any
	addRows := func(values ...any) {
		for _, value := range values {
			for _, row := range mapRowsFromAny(value) {
				if len(row) == 0 {
					continue
				}
				rows = append(rows, row)
			}
		}
	}
	observation := mapValue(observe.Result["observation"])
	projectPackage := mapValue(observation["project_package"])
	addRows(
		projectPackage["headroom_risk"],
		mapValue(observe.Result["project_package"])["headroom_risk"],
		mapValue(observe.Result["digest"])["project_headroom_risk_excerpt"],
		mapValue(observe.Result["acoustic_digest"])["project_headroom_risk_excerpt"],
	)
	addRows(
		projectPackage["tracks"],
		mapValue(observe.Result["project_package"])["tracks"],
		observation["tracks"],
		observe.Result["tracks"],
	)
	return rows
}

func nextPendingMixTickHeadroomRisk(row map[string]any) (string, bool) {
	if risk := strings.ToLower(strings.TrimSpace(cleanContextText(row["risk"]))); risk != "" {
		switch risk {
		case "critical", "high", "medium", "low":
			return risk, true
		}
	}
	headroomText := firstNonEmpty(cleanContextText(row["headroom_db"]), cleanContextText(row["headroom"]))
	peakText := firstNonEmpty(cleanContextText(row["peak_dbfs"]), cleanContextText(row["peak_db"]))
	return pendingMixTickPeakRisk(peakText, headroomText)
}

func nextPendingMixTickTrackCount(observe executor.Result, previous agentloop.PendingMixTickCandidate) int {
	if count := intNumber(mapValue(mapValue(observe.Result["observation"])["project_package"])["active_acoustic_track_count"]); count > 0 {
		return count
	}
	if count := intNumber(mapValue(observe.Result["project_package"])["active_acoustic_track_count"]); count > 0 {
		return count
	}
	if count := intNumber(previous.Fingerprint["track_count"]); count > 0 {
		return count
	}
	seen := map[string]bool{}
	for _, row := range nextPendingMixTickRiskRows(observe) {
		trackID := firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["id"]), cleanContextText(row["target_track_id"]))
		if trackID != "" {
			seen[trackID] = true
		}
	}
	return len(seen)
}

func nextPendingMixTickEvidence(row map[string]any, previous agentloop.PendingMixTickCandidate) map[string]any {
	out := map[string]any{
		"source":            "reobserve_after_mix_tick",
		"reason":            "headroom_risk",
		"previous_track_id": previous.TrackID,
		"previous_delta_db": previous.DeltaDB,
	}
	if risk, ok := nextPendingMixTickHeadroomRisk(row); ok {
		out["risk"] = risk
	}
	for _, key := range []string{"headroom_db", "peak_dbfs", "rms_dbfs", "crest_db", "name", "track_name", "role_guess", "rank"} {
		if value, ok := row[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
			out[key] = value
		}
	}
	return out
}

func pendingMixTickNextCandidateSuffix(candidate agentloop.PendingMixTickCandidate) string {
	return fmt.Sprintf("Next small-step suggestion is pending only: track %s %+0.2f dB. It will not run until you explicitly confirm it.",
		candidate.TrackID, candidate.DeltaDB)
}

func pendingMixTickReport(candidate agentloop.PendingMixTickCandidate, propose, apply, observe executor.Result, suffix string) string {
	if strings.HasPrefix(strings.TrimSpace(candidate.Operation), "track_pan_") {
		return pendingMixTickPanReport(candidate, propose, apply, suffix)
	}
	before := firstNonEmpty(cleanContextText(apply.Result["before_db"]), cleanContextText(propose.Result["before_db"]))
	after := firstNonEmpty(cleanContextText(apply.Result["after_db"]), cleanContextText(propose.Result["after_db"]))
	beforeTrack := pendingMixTickReportTrackFromCandidate(candidate)
	afterTrack := pendingMixTickReportTrackFromObserve(candidate.TrackID, observe)
	beforePeak, beforeRMS, beforeHeadroom := pendingMixTickReportMetrics(beforeTrack, false)
	afterPeak, afterRMS, afterHeadroom := pendingMixTickReportMetrics(afterTrack, true)
	headroom := firstNonEmpty(afterHeadroom, cleanContextText(mapValue(observe.Result["acoustic_digest"])["headroom_db"]), cleanContextText(propose.Result["project_headroom_db"]))
	peakRisk := pendingMixTickPeakRiskText(beforePeak, afterPeak, beforeHeadroom, afterHeadroom)
	lines := []string{
		fmt.Sprintf("已执行：轨道 %s 音量 %+0.2f dB。", candidate.TrackID, candidate.DeltaDB),
	}
	if before != "" || after != "" {
		lines = append(lines, fmt.Sprintf("执行前后音量：%s dB -> %s dB。", firstNonEmpty(before, "未知"), firstNonEmpty(after, "未知")))
	}
	if beforePeak != "" || afterPeak != "" || beforeRMS != "" || afterRMS != "" || beforeHeadroom != "" || afterHeadroom != "" {
		lines = append(lines, fmt.Sprintf("声学 before/after：peak %s -> %s dBFS，RMS %s -> %s dBFS，headroom %s -> %s dB。",
			firstNonEmpty(beforePeak, "未知"),
			firstNonEmpty(afterPeak, "未知"),
			firstNonEmpty(beforeRMS, "未知"),
			firstNonEmpty(afterRMS, "未知"),
			firstNonEmpty(beforeHeadroom, "未知"),
			firstNonEmpty(afterHeadroom, "未知"),
		))
	} else if headroom != "" {
		lines = append(lines, "重新观察后 headroom 参考值："+headroom+" dB。")
	}
	if peakRisk != "" {
		lines = append(lines, peakRisk)
	}
	lines = append(lines, "rollback 可用：如果听感不对，可以用项目撤销或 mix.rollback_tick 回滚这一步。")
	if strings.TrimSpace(suffix) != "" {
		lines = append(lines, suffix)
	}
	return strings.Join(lines, "\n")
}

func pendingMixTickPanReport(candidate agentloop.PendingMixTickCandidate, propose, apply executor.Result, suffix string) string {
	before := firstNonEmpty(cleanContextText(apply.Result["before_pan"]), cleanContextText(propose.Result["before_pan"]))
	after := firstNonEmpty(cleanContextText(apply.Result["after_pan"]), cleanContextText(propose.Result["after_pan"]))
	lines := []string{
		fmt.Sprintf("已执行：轨道 %s 声像调整。", candidate.TrackID),
	}
	if before != "" || after != "" {
		lines = append(lines, fmt.Sprintf("执行前后声像：%s -> %s。", firstNonEmpty(before, "未知"), firstNonEmpty(after, "未知")))
	}
	lines = append(lines, "rollback 可用：如果听感不对，可以用项目撤销或 mix.rollback_tick 回滚这一步。")
	if strings.TrimSpace(suffix) != "" {
		lines = append(lines, suffix)
	}
	return strings.Join(lines, "\n")
}

func pendingMixTickReportTrackFromCandidate(candidate agentloop.PendingMixTickCandidate) map[string]any {
	if candidate.Fingerprint == nil {
		return nil
	}
	for _, key := range []string{"before_track", "track", "target_track"} {
		if row := mapValue(candidate.Fingerprint[key]); len(row) > 0 {
			return row
		}
	}
	row := map[string]any{}
	copyIfPresent := func(outKey string, keys ...string) {
		for _, key := range keys {
			if value, ok := candidate.Fingerprint[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
				row[outKey] = value
				return
			}
		}
	}
	copyIfPresent("track_id", "target_track_id", "track_id")
	copyIfPresent("peak_dbfs", "peak_dbfs", "track_peak_dbfs")
	copyIfPresent("rms_dbfs", "rms_dbfs", "track_rms_dbfs")
	copyIfPresent("headroom_db", "headroom_db", "track_headroom_db")
	if len(row) == 0 {
		return nil
	}
	return row
}

func pendingMixTickReportTrackFromObserve(trackID string, observe executor.Result) map[string]any {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return nil
	}
	row := map[string]any{}
	for _, value := range []any{
		observe.Result["tracks"],
		mapValue(observe.Result["observation"])["tracks"],
		mapValue(mapValue(observe.Result["observation"])["project_package"])["tracks"],
		mapValue(observe.Result["project_package"])["tracks"],
		mapValue(observe.Result["digest"])["project_loudness_ranking_excerpt"],
		mapValue(observe.Result["digest"])["project_peak_ranking_excerpt"],
		mapValue(observe.Result["digest"])["project_headroom_risk_excerpt"],
		mapValue(observe.Result["acoustic_digest"])["project_loudness_ranking_excerpt"],
		mapValue(observe.Result["acoustic_digest"])["project_peak_ranking_excerpt"],
		mapValue(observe.Result["acoustic_digest"])["project_headroom_risk_excerpt"],
	} {
		for _, candidateRow := range mapRowsFromAny(value) {
			if pendingMixTickReportRowTrackID(candidateRow) == trackID {
				pendingMixTickMergeReportTrackRow(row, candidateRow)
			}
		}
	}
	pendingMixTickMergeReportTrackRow(row, pendingMixTickReportSelectedAcousticRow(trackID, observe))
	if len(row) == 0 {
		return nil
	}
	if _, ok := row["track_id"]; !ok {
		row["track_id"] = trackID
	}
	return row
}

func pendingMixTickReportSelectedAcousticRow(trackID string, observe executor.Result) map[string]any {
	target := mapValue(observe.Result["resolved_target"])
	acoustic := mapValue(observe.Result["acoustic_digest"])
	if len(target) == 0 && len(acoustic) == 0 {
		return nil
	}
	targetID := firstNonEmpty(
		cleanContextText(target["track_id"]),
		cleanContextText(acoustic["track_id"]),
		cleanContextText(mapValue(acoustic["target"])["track_id"]),
	)
	if targetID != trackID {
		return nil
	}
	row := map[string]any{"track_id": trackID}
	copyReportValue := func(key string, sources ...map[string]any) {
		if _, ok := row[key]; ok {
			return
		}
		for _, source := range sources {
			if value, ok := source[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
				row[key] = value
				return
			}
		}
	}
	waveform := mapValue(acoustic["waveform"])
	targetInfo := mapValue(acoustic["target"])
	for _, key := range []string{"name", "track_name", "user_label", "role_guess"} {
		copyReportValue(key, target, acoustic, targetInfo)
	}
	for _, key := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db"} {
		copyReportValue(key, acoustic, waveform)
	}
	return row
}

func pendingMixTickReportRowTrackID(row map[string]any) string {
	return firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["id"]), cleanContextText(row["target_track_id"]))
}

func pendingMixTickMergeReportTrackRow(out map[string]any, row map[string]any) {
	if len(out) == 0 && out == nil || len(row) == 0 {
		return
	}
	copyIfMissing := func(key string, value any) {
		if value == nil {
			return
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return
		}
		if existing, ok := out[key]; ok && strings.TrimSpace(fmt.Sprint(existing)) != "" && strings.TrimSpace(fmt.Sprint(existing)) != "<nil>" {
			return
		}
		out[key] = value
	}
	for _, key := range []string{"track_id", "id", "target_track_id", "name", "track_name", "user_label", "role_guess", "peak_dbfs", "peak_db", "rms_dbfs", "rms_db", "level_db", "headroom_db", "headroom", "crest_db"} {
		copyIfMissing(key, row[key])
	}
	metric := strings.ToLower(strings.TrimSpace(cleanContextText(row["metric"])))
	if value, ok := row["value"]; ok {
		switch metric {
		case "peak_dbfs", "peak_db":
			copyIfMissing("peak_dbfs", value)
		case "rms_dbfs", "rms_db", "level_db":
			copyIfMissing("rms_dbfs", value)
		case "headroom_db", "headroom":
			copyIfMissing("headroom_db", value)
		}
	}
}

func pendingMixTickReportMetrics(row map[string]any, rankingFallback bool) (string, string, string) {
	if len(row) == 0 {
		return "", "", ""
	}
	peak := firstNonEmpty(cleanContextText(row["peak_dbfs"]), cleanContextText(row["peak_db"]))
	rms := firstNonEmpty(cleanContextText(row["rms_dbfs"]), cleanContextText(row["rms_db"]), cleanContextText(row["level_db"]))
	headroom := firstNonEmpty(cleanContextText(row["headroom_db"]), cleanContextText(row["headroom"]))
	if rankingFallback {
		metric := strings.ToLower(strings.TrimSpace(cleanContextText(row["metric"])))
		if value := cleanContextText(row["value"]); value != "" {
			switch metric {
			case "peak_dbfs", "peak_db":
				peak = firstNonEmpty(peak, value)
			case "rms_dbfs", "rms_db", "level_db":
				rms = firstNonEmpty(rms, value)
			case "headroom_db", "headroom":
				headroom = firstNonEmpty(headroom, value)
			}
		}
	}
	return peak, rms, headroom
}

func pendingMixTickPeakRiskText(beforePeak, afterPeak, beforeHeadroom, afterHeadroom string) string {
	beforeRisk, beforeOK := pendingMixTickPeakRisk(beforePeak, beforeHeadroom)
	afterRisk, afterOK := pendingMixTickPeakRisk(afterPeak, afterHeadroom)
	if !beforeOK && !afterOK {
		return ""
	}
	if beforeOK && afterOK {
		if pendingMixTickRiskRank(afterRisk) < pendingMixTickRiskRank(beforeRisk) {
			return "peak risk 已改善：" + beforeRisk + " -> " + afterRisk + "。"
		}
		if pendingMixTickRiskRank(afterRisk) > pendingMixTickRiskRank(beforeRisk) {
			return "peak risk 变高：" + beforeRisk + " -> " + afterRisk + "，建议听感确认后必要时回滚。"
		}
		return "peak risk 维持：" + afterRisk + "。"
	}
	if afterOK {
		return "重新观察后的 peak risk：" + afterRisk + "。"
	}
	return ""
}

func pendingMixTickPeakRisk(peakText, headroomText string) (string, bool) {
	headroom, ok := parseOptionalFloat(headroomText)
	if !ok {
		if peak, peakOK := parseOptionalFloat(peakText); peakOK {
			headroom = -peak
			ok = true
		}
	}
	if !ok {
		return "", false
	}
	switch {
	case headroom <= 1:
		return "high", true
	case headroom <= 3:
		return "medium", true
	default:
		return "low", true
	}
}

func pendingMixTickRiskRank(risk string) int {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "high", "critical":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func parseOptionalFloat(text string) (float64, bool) {
	text = strings.TrimSpace(text)
	if text == "" || text == "<nil>" || text == "未知" {
		return 0, false
	}
	var value float64
	if _, err := fmt.Sscanf(text, "%f", &value); err != nil {
		return 0, false
	}
	return value, true
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
