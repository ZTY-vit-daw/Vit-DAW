package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/panlayout"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/staticbalance"
	"vit-daw-agent/internal/toolpolicy"
	"vit-daw-agent/internal/tools"
)

const agentLoopConfirmationWorkflow = "agentloop_confirmation"

func (s *Server) beginChatGoal(conversationID, message string, requestContext map[string]any) agentruntime.Goal {
	if s == nil || s.harness == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	if isContinueMessage(message) || s.shouldResumeConversationGoal(conversationID) {
		if goalID := s.conversationGoalID(conversationID); goalID != "" {
			goal := s.harness.RuntimeStatus(goalID)
			if goal.GoalID != "" {
				return goal
			}
		}
	}
	goalID, runID := goalIDsFromContext(requestContext)
	if strings.TrimSpace(goalID) != "" {
		existing := s.harness.RuntimeStatus(goalID)
		if existing.GoalID != "" && isTerminalGoalStatus(existing.Status) {
			return s.harness.BeginGoal(message)
		}
		return s.harness.EnsureGoal(goalID, runID, message)
	}
	return s.harness.BeginGoal(message)
}

func (s *Server) shouldResumeConversationGoal(conversationID string) bool {
	if s == nil || s.harness == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}
	goalID := s.conversationGoalID(conversationID)
	if strings.TrimSpace(goalID) == "" {
		return false
	}
	return shouldResumeGoalFromStatus(s.harness.RuntimeStatus(goalID).Status)
}

func shouldUseAgentLoop(message string, requestContext map[string]any) bool {
	msg := strings.TrimSpace(message)
	if contextBool(requestContext, "use_legacy_chat") || contextBool(requestContext, "disable_agent_loop") || contextBool(requestContext, "disable_goal_runner") {
		return false
	}
	lower := strings.ToLower(msg)
	if strings.HasPrefix(lower, "/goal") {
		return true
	}
	if strings.HasPrefix(msg, "/") {
		return false
	}
	return true
}
func isTerminalGoalStatus(status agentruntime.GoalStatus) bool {
	switch status {
	case agentruntime.StatusCompleted, agentruntime.StatusFailed, agentruntime.StatusCancelled, agentruntime.StatusStopped, agentruntime.StatusStable:
		return true
	default:
		return false
	}
}

func isMultiPluginLoadIntent(message string) bool {
	if !looksLikePluginGrabberLoadIntent(message) {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(message))
	hasPluginSubject := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "effect") ||
		strings.Contains(message, "\u63d2\u4ef6") ||
		strings.Contains(message, "\u6548\u679c\u5668")
	hasPluralReference := strings.Contains(message, "\u8fd9\u4e24\u4e2a") ||
		strings.Contains(message, "\u9019\u5169\u500b") ||
		strings.Contains(message, "\u4e24\u4e2a") ||
		strings.Contains(message, "\u5169\u500b") ||
		strings.Contains(message, "\u90fd") ||
		strings.Contains(lower, "both") ||
		strings.Contains(lower, "all")
	if hasPluginSubject && hasPluralReference {
		return true
	}
	types := 0
	for _, needles := range [][]string{
		{"eq", "equalizer", "\u5747\u8861"},
		{"reverb", "\u6df7\u54cd"},
		{"compressor", "\u538b\u7f29"},
		{"delay", "\u5ef6\u8fdf"},
		{"analyzer", "\u5206\u6790"},
		{"limiter", "\u9650\u5236"},
	} {
		for _, needle := range needles {
			if strings.Contains(lower, strings.ToLower(needle)) {
				types++
				break
			}
		}
	}
	return types >= 2 || (types >= 1 && (strings.Contains(message, "\u4e24\u4e2a") || strings.Contains(message, "\u5169\u500b") || strings.Contains(lower, "both")))
}
func (s *Server) runAgentLoopChat(ctx context.Context, conversationID string, req ChatRequest, cfg config.EngineConfig) (resp ChatResponse, handled bool) {
	started := time.Now()
	defer func() {
		if s != nil && s.logger != nil && handled {
			s.logger.Info("[timing] agent_loop_chat total_ms=%d conversation=%s goal=%s mode=%s status=%s stop=%s completed_steps=%d executed=%d",
				time.Since(started).Milliseconds(), conversationID, resp.GoalID, resp.AgentMode, resp.GoalStatus, resp.StopReason, resp.CompletedSteps, len(resp.ExecutedKernelReply))
		}
	}()
	if s == nil || s.harness == nil {
		return ChatResponse{ConversationID: conversationID, Reply: "Agent 运行器暂时不可用。", Error: "agentloop_unavailable"}, true
	}
	userText := agentLoopUserText(req.Message)
	durableInvocationID := durableContinuationInvocationID(ctx)
	requestContext := contextWithoutUntrustedSemanticEntry(req.Context)
	if durableInvocationID != "" {
		// This context was loaded from the durable project snapshot by the
		// scheduler, not supplied by an HTTP caller. Preserve its authoritative
		// semantic/controller bindings and address the exact checkpoint.
		requestContext = cloneContext(req.Context)
		requestContext["durable_continuation_id"] = durableInvocationID
	}
	chatContext := contextWithUserMessage(requestContext, userText)
	chatContext = s.bindActiveOrchestrationController(conversationID, chatContext)
	mode := agentModeFromContext(chatContext)
	if durableInvocationID != "" {
		if _, ok := s.resumeContinuationForChat(conversationID, chatContext); !ok {
			return ChatResponse{
				ConversationID: conversationID,
				Reply:          "Durable continuation checkpoint is unavailable; no new task was started.",
				GoalStatus:     string(agentruntime.StatusFailed),
				StopReason:     "durable_continuation_checkpoint_unavailable",
				Error:          "durable continuation checkpoint is unavailable or not claimed by this scheduler",
			}, true
		}
	}
	if durableInvocationID == "" && isContinueMessage(req.Message) {
		if pending, ok := s.automaticContinuationForConversation(conversationID); ok {
			return ChatResponse{
				ConversationID: conversationID,
				TaskID:         pending.TaskID, GoalID: pending.GoalID, RunID: pending.RunID,
				SliceID: pending.CurrentSliceID, TurnID: pending.CurrentTurnID, OriginalIntent: pending.OriginalIntent,
				Reply:      "任务已在自动续跑队列中；这条消息不会创建新任务或重复执行。",
				GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "automatic_continuation_already_scheduled",
				Workflow: "durable_continuation", WorkflowData: map[string]any{
					"continuation_id": pending.ContinuationID, "status": pending.Status, "attempt": pending.Attempt,
				},
			}, true
		}
		if pending, ok := s.interactionContinuationForConversation(conversationID); ok {
			goalStatus := s.harness.RuntimeStatus(pending.GoalID).Status
			if goalStatus == "" || goalStatus == agentruntime.StatusIdle {
				goalStatus = agentruntime.StatusWaitingContinue
			}
			return ChatResponse{
				ConversationID: conversationID,
				TaskID:         pending.TaskID, GoalID: pending.GoalID, RunID: pending.RunID,
				SliceID: pending.CurrentSliceID, TurnID: pending.CurrentTurnID, OriginalIntent: pending.OriginalIntent,
				Reply:      "当前任务正在等待明确的确认、澄清或人工判断；单独说‘继续’不会越过这个交互边界。",
				GoalStatus: string(goalStatus), StopReason: "pending_interaction_requires_response",
				Workflow: "durable_continuation", WorkflowData: map[string]any{
					"continuation_id": pending.ContinuationID, "status": pending.Status,
					"pending_interaction": cloneContext(pending.PendingInteraction),
				},
			}, true
		}
	}
	classificationRequired := mode != agentModePlan && durableInvocationID == "" && !isContinueMessage(req.Message) &&
		!contextBool(chatContext, "free_state_diagnostic_only") &&
		!s.hasActiveFreeStateReasoningLoop(conversationID) && !s.hasActiveOrchestrationController(conversationID)
	if classificationRequired {
		if _, continuing := s.resumeContinuationForChat(conversationID, chatContext); continuing {
			classificationRequired = false
		}
	}
	if classificationRequired {
		decision, err := s.planSemanticEntry(ctx, conversationID, userText, chatContext, cfg)
		if err != nil {
			// The frozen blind smoke must not silently fall back to legacy
			// mix.observe after the model-owned semantic entry failed. That
			// fallback cannot produce the required CCB observation receipt.
			if blindProjectSmokeContext(chatContext) {
				return semanticEntryServiceFailureResponse(conversationID, mode, err), true
			}
			chatContext = mergeContext(chatContext, map[string]any{"semantic_entry_unavailable": true})
		}
		if err == nil && decision.Route == semanticEntryRouteUnresolved {
			return semanticEntryUnresolvedResponse(conversationID, mode, &decision, nil), true
		}
		if err == nil {
			var routingIdentity CapabilityRouteRecord
			chatContext, routingIdentity = s.ensureCapabilityRoutingTask(userText, chatContext)
			decision, routingIdentity, err = s.planObservationFirstCapabilityRoute(ctx, conversationID, userText, chatContext, decision, routingIdentity)
			if err != nil {
				return semanticEntryUnresolvedResponse(conversationID, mode, &decision, err), true
			}
			chatContext = contextWithSemanticEntryDecision(chatContext, decision)
			chatContext = contextWithCapabilityRoute(chatContext, routingIdentity)
		}
	}
	if !classificationRequired {
		var refreshErr error
		chatContext, refreshErr = s.refreshCapabilityRouteForRevision(ctx, conversationID, chatContext)
		if refreshErr != nil {
			return audioClosureControllerErrorResponse(conversationID, mode, refreshErr), true
		}
	}
	chatContext, projectMixOwner, projectMixActive, projectMixErr := s.prepareProjectMixController(conversationID, chatContext)
	if projectMixErr != nil {
		return audioClosureControllerErrorResponse(conversationID, mode, projectMixErr), true
	}
	if projectMixActive && !projectMixWorkflowV1Available(chatContext) {
		return s.projectMixUnavailableResponse(conversationID, mode, projectMixOwner, chatContext), true
	}
	chatContext, audioClosure, audioClosureActive, closureErr := s.prepareAudioClosureContext(conversationID, userText, chatContext)
	if closureErr != nil {
		return audioClosureControllerErrorResponse(conversationID, mode, closureErr), true
	}
	chatContext, freeStateActive := s.prepareFreeStateReasoningContext(conversationID, userText, chatContext)
	mode = agentModeFromContext(chatContext)
	// An internal free-state continuation must never resurrect a completed or
	// blocked reasoning loop. In particular, native-tool confirmations used to
	// settle the parent closure early; the next continuation then re-entered
	// semantic entry and started a fresh closure indefinitely. Return the
	// durable terminal decision directly so the caller can start a new task
	// explicitly if desired.
	if contextBool(chatContext, "free_state_internal_resume") {
		if loop, ok := freeStateLoopFromAny(chatContext["free_state_reasoning_loop"]); ok && !freeStateLoopActive(loop) {
			if audioClosureActive && s.audioClosures != nil {
				if current, tracked := s.audioClosures.ActiveForConversation(conversationID); tracked && !current.Terminal() {
					reason := audioclosure.StopRoundLimit
					if strings.EqualFold(strings.TrimSpace(loop.Status), "cancelled") {
						reason = audioclosure.StopCancelled
					}
					if settled, settleErr := (audioclosure.Driver{}).Settle(current, current.Revision, reason,
						firstNonEmpty(loop.LastError, "free-state reasoning loop is no longer active"), false, time.Now().UTC()); settleErr == nil {
						if s.audioClosures.Save(settled, current.Revision) == nil {
							s.settleAudioClosureOwner(settled)
							s.persistCurrentProjectWorkspace()
							return s.audioClosureResponse(conversationID, mode, settled, agentloop.Result{GoalID: settled.GoalID, RunID: settled.RunID}), true
						}
					}
				}
			}
			return ChatResponse{ConversationID: conversationID, GoalID: loop.GoalID, RunID: loop.RunID,
				Reply:      firstNonEmpty(loop.LastError, "free-state reasoning loop has ended; no further automatic action was started."),
				GoalStatus: string(agentruntime.StatusCompleted), Workflow: "free_state_reasoning_loop",
				StopReason:   firstNonEmpty(loop.LastError, "free_state_loop_inactive"),
				WorkflowData: map[string]any{"status": loop.Status, "free_state_reasoning_loop": freeStateLoopMap(loop), "mutation_performed": false},
			}, true
		}
	}
	messageLoop := s.newAgentMessageLoop(cfg, mode)
	legacyRunner := s.newAgentLoopRunner(cfg, mode)
	loopBudget := func(context map[string]any) agentloop.Budget {
		budget := agentLoopBudgetForContext(mode, context)
		if audioClosureActive {
			budget = audioClosureMessageLoopBudget(budget, context)
		}
		return budget
	}
	if freeStateActive && isContinueMessage(req.Message) {
		if response, resumed := s.resumeFreeStateMaterialization(ctx, conversationID, mode, chatContext, cfg); resumed {
			return s.bindFreeStateContextToResponse(response, chatContext), true
		}
	}
	if audioClosureActive {
		if audioClosure.Terminal() {
			return s.audioClosureResponse(conversationID, mode, audioClosure, agentloop.Result{GoalID: audioClosure.GoalID, RunID: audioClosure.RunID}), true
		}
		var admitted bool
		audioClosure, admitted, closureErr = s.admitAudioClosureRound(audioClosure)
		chatContext = bindAudioClosureContext(chatContext, audioClosure)
		if closureErr != nil {
			return audioClosureControllerErrorResponse(conversationID, mode, closureErr), true
		}
		if !admitted || audioClosure.Terminal() {
			return s.audioClosureResponse(conversationID, mode, audioClosure, agentloop.Result{GoalID: audioClosure.GoalID, RunID: audioClosure.RunID}), true
		}
	}

	var res agentloop.Result
	if isContinueMessage(req.Message) {
		cont, ok := s.goalContinuationForConversation(conversationID)
		if !ok {
			goalID, runID := goalIDsFromContext(req.Context)
			return ChatResponse{
				ConversationID: conversationID,
				AgentMode:      mode,
				GoalID:         goalID,
				RunID:          runID,
				Reply:          "当前没有可继续的暂停任务。",
				GoalStatus:     string(s.harness.RuntimeStatus("").Status),
				StopReason:     "no_continuation",
			}, true
		}
		cont.Context = mergeContext(cont.Context, chatContext)
		mode = agentModeFromContext(cont.Context)
		projectPath := projectPathFromChatContext(cont.Context)
		cont.State = s.harness.UserStateSummary(ctx)
		cont.ProjectHistory = s.harness.ProjectHistorySummaryForProject(ctx, cont.GoalID, projectPath)
		if len(cont.ProjectHistory) > 0 {
			cont.State = cloneContext(cont.State)
			cont.State["project_history"] = cont.ProjectHistory
		}
		if useLegacyPlannerLoop() || len(cont.Conversation) == 0 {
			cont.Conversation = s.recentConversationMessages(conversationID, 8, cont.ProjectHistory)
		}
		toolContext := s.agentLoopToolContext(mode, cont.UserText, cont.Context)
		cont.CatalogSummary = toolContext.CatalogSummary
		cont.AllowedTools = toolContext.AllowedTools
		cont.Budget = loopBudget(cont.Context)
		if useLegacyPlannerLoop() && !freeStateActive {
			res = legacyRunner.Continue(ctx, cont)
		} else {
			res = messageLoop.Continue(ctx, cont)
		}
	} else if cont, ok := s.resumeContinuationForChat(conversationID, chatContext); ok && shouldResumeGoalFromStatus(s.harness.RuntimeStatus(cont.GoalID).Status) {
		cont.UserText = agentLoopContinuationUserText(cont, userText)
		cont.Context = mergeContext(cont.Context, chatContext)
		mode = agentModeFromContext(cont.Context)
		projectPath := projectPathFromChatContext(cont.Context)
		cont.State = s.harness.UserStateSummary(ctx)
		cont.ProjectHistory = s.harness.ProjectHistorySummaryForProject(ctx, cont.GoalID, projectPath)
		if len(cont.ProjectHistory) > 0 {
			cont.State = cloneContext(cont.State)
			cont.State["project_history"] = cont.ProjectHistory
		}
		if useLegacyPlannerLoop() || len(cont.Conversation) == 0 {
			cont.Conversation = s.recentConversationMessages(conversationID, 8, cont.ProjectHistory)
		}
		toolContext := s.agentLoopToolContext(mode, cont.UserText, cont.Context)
		cont.CatalogSummary = toolContext.CatalogSummary
		cont.AllowedTools = toolContext.AllowedTools
		cont.Budget = loopBudget(cont.Context)
		if useLegacyPlannerLoop() && !freeStateActive {
			res = legacyRunner.Continue(ctx, cont)
		} else {
			res = messageLoop.Continue(ctx, cont)
		}
	} else {
		goalID, runID := s.freshAgentLoopGoalIDs(chatContext)
		projectPath := projectPathFromChatContext(chatContext)
		projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		state := s.harness.UserStateSummary(ctx)
		if len(projectHistory) > 0 {
			state = cloneContext(state)
			state["project_history"] = projectHistory
		}
		toolContext := s.agentLoopToolContext(mode, userText, chatContext)
		executionMemory := s.agentLoopExecutionMemoryForConversation(conversationID)
		if useLegacyPlannerLoop() && !freeStateActive {
			res = legacyRunner.Start(ctx, agentloop.Input{
				GoalID:          goalID,
				RunID:           runID,
				UserText:        userText,
				Summary:         userText,
				Context:         chatContext,
				ProjectHistory:  projectHistory,
				Conversation:    s.freshAgentLoopConversation(conversationID, userText, projectHistory),
				State:           state,
				CatalogSummary:  toolContext.CatalogSummary,
				AllowedTools:    toolContext.AllowedTools,
				Budget:          loopBudget(chatContext),
				ExecutionMemory: executionMemory,
			})
		} else {
			res = messageLoop.Start(ctx, agentloop.Input{
				GoalID:          goalID,
				RunID:           runID,
				UserText:        userText,
				Summary:         userText,
				Context:         chatContext,
				ProjectHistory:  projectHistory,
				Conversation:    s.freshAgentLoopConversation(conversationID, userText, projectHistory),
				State:           state,
				CatalogSummary:  toolContext.CatalogSummary,
				AllowedTools:    toolContext.AllowedTools,
				Budget:          loopBudget(chatContext),
				ExecutionMemory: executionMemory,
			})
		}
	}
	if res.Status == agentruntime.StatusStopped {
		goal, checkpoint := s.finalizeStoppedTurn(ctx, conversationID, res.GoalID, res.RunID, "user_stop")
		resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
		resp.GoalStatus = string(agentruntime.StatusStopped)
		resp.StopReason = "stop_turn_completed"
		resp.Reply = "已停止当前 Turn；不会再启动新的实验动作。"
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{"stop_turn": true, "checkpoint_ref": checkpoint, "goal_status": goal.Status})
		return s.bindFreeStateContextToResponse(resp, chatContext), true
	}
	if freeStateActive {
		if observation := freeStateCCBObservation(res); observation != nil {
			res.RecentObservation = observation
		}
		loop, _ := s.recordFreeStateDecision(conversationID, res)
		if loop.LatestDecision != nil {
			res.FreeStateDecision = loop.LatestDecision
		}
		res.RecentObservation = loop.LatestObservation
		chatContext = mergeContext(chatContext, map[string]any{"free_state_reasoning_loop": freeStateLoopMap(loop)})
		chatContext = s.bindFreeStateAuthoritativeTrack(conversationID, chatContext)
		awaitingAction := strings.EqualFold(loop.Status, "awaiting_action") && res.FreeStateDecision != nil &&
			strings.EqualFold(res.FreeStateDecision.Status, agentloop.FreeStateNeedsAction)
		awaitingExperiment := strings.EqualFold(loop.Status, "awaiting_experiment") && res.FreeStateDecision != nil &&
			strings.EqualFold(res.FreeStateDecision.Status, agentloop.FreeStateNeedsExperiment)
		if !awaitingAction && !awaitingExperiment {
			if audioClosureActive {
				audioClosure, closureErr = s.recordAudioClosureRound(audioClosure, res, chatContext)
				if closureErr != nil {
					return audioClosureControllerErrorResponse(conversationID, mode, closureErr), true
				}
				chatContext = bindAudioClosureContext(chatContext, audioClosure)
				return s.bindFreeStateContextToResponse(s.audioClosureResponse(conversationID, mode, audioClosure, res), chatContext), true
			}
			return s.bindFreeStateContextToResponse(s.chatResponseFromAgentLoopResult(conversationID, mode, res), chatContext), true
		}
		if awaitingExperiment {
			if audioClosureActive {
				audioClosure, closureErr = s.recordAudioClosureRound(audioClosure, res, chatContext)
				if closureErr != nil {
					return audioClosureControllerErrorResponse(conversationID, mode, closureErr), true
				}
				chatContext = bindAudioClosureContext(chatContext, audioClosure)
			}
			response := s.improvementProposalResponse(conversationID, mode, res, chatContext)
			if audioClosureActive {
				response, audioClosure = s.bindAudioClosureCapabilityHandoff(response, audioClosure)
			}
			return s.bindFreeStateContextToResponse(response, chatContext), true
		}
		userText = strings.TrimSpace(res.FreeStateDecision.RemainingIntent)
		chatContext = mergeContext(chatContext, map[string]any{
			"free_state_route_authorized": true,
			"free_state_processor_type":   strings.ToLower(strings.TrimSpace(res.FreeStateDecision.ProcessorType)),
			"goal_id":                     res.GoalID,
			"run_id":                      res.RunID,
		})
		if intent := freeStateProcessorIntentMap(res.FreeStateDecision); len(intent) > 0 {
			chatContext["free_state_semantic_processor_intent"] = intent
		}
	}
	if audioClosureActive {
		audioClosure, closureErr = s.recordAudioClosureRound(audioClosure, res, chatContext)
		if closureErr != nil {
			return audioClosureControllerErrorResponse(conversationID, mode, closureErr), true
		}
		chatContext = bindAudioClosureContext(chatContext, audioClosure)
		if audioClosure.Terminal() {
			return s.bindFreeStateContextToResponse(s.audioClosureResponse(conversationID, mode, audioClosure, res), chatContext), true
		}
	}
	if projectMixActive {
		response := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
		return s.projectMixControllerResponse(response, projectMixOwner, res), true
	}
	if freeStateRouteAuthorized(chatContext) {
		response, routed := s.routeOrdinaryAgentTreatmentStrategy(ctx, conversationID, mode, userText, chatContext, res, cfg)
		if !routed {
			base := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
			if audioClosureActive {
				base = bindAudioClosureToResponse(base, audioClosure)
			}
			return s.bindFreeStateContextToResponse(base, chatContext), true
		}
		response = s.makeFreeStateMaterializationResumable(conversationID, chatContext, res, response)
		if audioClosureActive {
			response, audioClosure = s.bindAudioClosureCapabilityHandoff(response, audioClosure)
		}
		return s.bindFreeStateContextToResponse(response, chatContext), true
	}
	entryDecision, entryVerified := semanticEntryDecisionFromContext(chatContext)
	entryUnavailable := contextBool(chatContext, "semantic_entry_unavailable")
	if res.SemanticAction == nil && !entryUnavailable && (!entryVerified || entryDecision.Route == semanticEntryRouteOther) && ordinaryAgentPluginRecommendationIntent(userText, chatContext) {
		return s.bindFreeStateContextToResponse(s.ordinaryAgentPluginRecommendationResponse(ctx, conversationID, mode, userText, chatContext, res, cfg), chatContext), true
	}
	if res.SemanticAction != nil {
		if entryVerified || entryUnavailable {
			res.SemanticAction = nil
			res.Error = "semantic_entry forbids direct semantic_action materialization outside the governed free-state handoff"
			res.StopReason = "semantic_entry_direct_family_materialization_denied"
			res.Status = agentruntime.StatusFailed
			return s.chatResponseFromAgentLoopResult(conversationID, mode, res), true
		}
		req.Context = chatContext
		return s.bindFreeStateContextToResponse(s.materializeAgentSemanticEQAction(ctx, conversationID, req, mode, res), chatContext), true
	}
	response := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	if audioClosureActive {
		response = bindAudioClosureToResponse(response, audioClosure)
	}
	return s.bindFreeStateContextToResponse(response, chatContext), true
}

func blindProjectSmokeContext(requestContext map[string]any) bool {
	return contextBool(requestContext, "blind_experiment") &&
		strings.EqualFold(firstStringFromMap(requestContext, "interaction_path"), "blind_project_smoke")
}

func semanticEntryServiceFailureResponse(conversationID, mode string, err error) ChatResponse {
	message := "semantic entry model request failed"
	if err != nil {
		message += ": " + err.Error()
	}
	return ChatResponse{
		ConversationID: conversationID,
		AgentMode:      mode,
		Reply:          "开放语义处理因模型服务暂时不可用而暂停；未执行观察或工程修改。",
		Error:          message,
		GoalStatus:     string(agentruntime.StatusWaitingContinue),
		StopReason:     "transient_llm_error",
		Workflow:       "semantic_entry",
		WorkflowData: map[string]any{
			"status":             "transient_llm_error",
			"mutation_performed": false,
		},
	}
}

func (s *Server) semanticEQPluginSelectionRequiredResponse(conversationID, mode, userText string, requestContext map[string]any, res agentloop.Result) ChatResponse {
	trackID := firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")
	trackName := firstStringFromMap(requestContext, "selected_track_name")
	res.ExecutionMemory.PendingMixTreatment = nil
	res.Status = agentruntime.StatusCompleted
	res.StopReason = ""
	res.Error = ""
	res.Continuation = nil
	res.Reply = "当前选中轨道尚未选择可执行的 EQ。已保留听感目标和观察证据，但没有创建旧式 profile/learn 方案，也没有加载插件或写入参数；下一步需要进入效果器推荐与选择。"
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.NeedsConfirmation = false
	resp.Workflow = "plugin_selection_required"
	resp.WorkflowData = map[string]any{
		"schema_version":      "plugin_selection_required.v1",
		"status":              "required",
		"processor_type":      "eq",
		"listening_goal":      strings.TrimSpace(userText),
		"target_ref":          map[string]any{"kind": "track", "id": trackID, "label": trackName},
		"selection_performed": false,
		"mutation_performed":  false,
		"next_stage":          "plugin_recommendation_and_selection",
	}
	if res.RecentObservation != nil {
		texts := semanticEQRecursiveText(res.RecentObservation.Summary)
		if observationID := semanticEQFirstRecursive(texts, "observation_id"); observationID != "" {
			resp.WorkflowData["observation_id"] = observationID
			resp.WorkflowData["evidence_refs"] = []string{"mix.observe:" + observationID}
		}
	}
	return resp
}

// agentLoopContextWithGenericEQTopology supplies deterministic structural
// constraints only after an upstream family-owned caller has authorized the
// exact EQ topology read. It is intentionally inert for ordinary entry turns;
// the semantic entry arbiter must not receive a pre-family EQ hint.
func (s *Server) agentLoopContextWithGenericEQTopology(ctx context.Context, userText string, requestContext map[string]any) map[string]any {
	if !contextBool(requestContext, "semantic_eq_topology_authorized") {
		return requestContext
	}
	if s == nil || !agentLoopNeedsGenericEQTopology(userText, requestContext) {
		return requestContext
	}
	trackID := firstStringFromMap(requestContext, "selected_plugin_track_id", "selected_track_id")
	pluginID := firstStringFromMap(requestContext, "selected_plugin_id")
	if trackID == "" || pluginID == "" {
		return requestContext
	}
	_, summary, err := s.readLiveEQControlSurface(ctx, trackID, pluginID)
	if err != nil || len(summary) == 0 {
		return requestContext
	}
	out := cloneContext(requestContext)
	if out == nil {
		out = map[string]any{}
	}
	out["generic_eq_topology"] = semanticEQTopologyPromptSummary(trackID, pluginID, summary)
	return out
}

func agentLoopNeedsGenericEQTopology(userText string, requestContext map[string]any) bool {
	if !contextHasAnyValue(requestContext, "selected_plugin_id") {
		return false
	}
	return ordinaryAgentSemanticEQTextTopic(userText)
}
func (s *Server) recentConversationMessages(conversationID string, limit int, projectHistory map[string]any) []llm.Message {
	if s == nil || limit == 0 {
		return nil
	}
	return s.conversationHistory(context.Background(), conversationID, limit, projectHistory)
}

func (s *Server) freshAgentLoopConversation(conversationID, userText string, projectHistory map[string]any) []llm.Message {
	if !freshAgentLoopNeedsRecentConversation(userText) {
		return nil
	}
	return s.recentConversationMessages(conversationID, 8, projectHistory)
}

func (s *Server) freshAgentLoopGoalIDs(chatContext map[string]any) (string, string) {
	goalID, runID := goalIDsFromContext(chatContext)
	if s == nil || s.harness == nil || strings.TrimSpace(goalID) == "" {
		return goalID, runID
	}
	if isTerminalGoalStatus(s.harness.RuntimeStatus(goalID).Status) {
		return "", ""
	}
	return goalID, runID
}

func freshAgentLoopNeedsRecentConversation(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	for _, needle := range []string{
		"继续", "接着", "刚才", "上次", "上一", "前面", "之前", "同样", "一样", "再来", "照着", "基于",
		"continue", "resume", "again", "same as", "as before", "previous", "last time",
	} {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func (s *Server) newAgentLoopRunner(cfg config.EngineConfig, mode string) agentloop.Runner {
	return agentloop.Runner{
		Runtime:  s.harness.Runtime(),
		Planner:  planner.LLMPlanner{Client: s.llm, Config: cfg},
		Executor: s.newAgentLoopExecutor(cfg),
		Budget:   agentLoopBudgetForMode(mode),
	}
}

func (s *Server) newAgentMessageLoop(cfg config.EngineConfig, mode string) *agentloop.MessageLoop {
	return &agentloop.MessageLoop{
		Runtime:  s.harness.Runtime(),
		Client:   s.llm,
		Config:   cfg,
		Executor: s.newAgentLoopExecutor(cfg),
		Budget:   agentLoopBudgetForMode(mode),
		Logger:   s.logger,
	}
}

func agentLoopBudgetForContext(mode string, requestContext map[string]any) agentloop.Budget {
	if contextBool(requestContext, "free_state_diagnostic_only") {
		// Catalog discovery, optional structure discovery, one rejected exact
		// view set, one usable diagnostic bundle, and the terminal conclusion
		// can require five model turns. The observation window gate prevents
		// this extra turn from becoming another observation cycle.
		return agentloop.Budget{MaxTurns: 5, MaxToolCalls: 6, Timeout: 210 * time.Second}
	}
	budget := agentLoopBudgetForMode(mode)
	if loop := firstMapFromAny(requestContext["free_state_reasoning_loop"]); strings.EqualFold(strings.TrimSpace(firstStringFromMap(loop, "decision_phase")), freeStatePhasePostActionEvaluation) {
		// The post-apply settle turn often needs the final-gate feedback cycle
		// (bare terminals are refused until the structured settle report is
		// emitted). Pre-apply reasoning has already consumed most of the run's
		// turn budget, so without this headroom the gate feedback can never
		// retry and the round strands without materiality (2026-08-28 12:50
		// smoke: settle turns 5-6 ended limit_reached after one call each).
		budget.MaxTurns += 4
		if budget.Timeout > 0 {
			budget.Timeout += 120 * time.Second
		}
	}
	return budget
}

func (s *Server) newAgentLoopExecutor(cfg config.EngineConfig) agentloop.ToolExecutor {
	return pluginGrabberWorkflowExecutor{
		server: s,
		base:   executorpkg.New(s.harness),
		cfg:    cfg,
	}
}

type pluginGrabberWorkflowExecutor struct {
	server *Server
	base   agentloop.ToolExecutor
	cfg    config.EngineConfig
}

func (e pluginGrabberWorkflowExecutor) RunToolCall(ctx context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	toolCallID := strings.TrimSpace(in.ToolCall.ID)
	if toolCallID == "" {
		toolCallID = "tool_goal_step"
	}
	authorityConfirmed := in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull
	if reason := freeStateInvokeMutationReason(harness.InvokeRequest{
		Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context), Confirmed: authorityConfirmed,
	}); reason != "" {
		out := executorpkg.Result{ToolCallID: toolCallID, Tool: in.ToolCall.Tool, CommandName: "free_state_mutation_forbidden", Status: "error", Error: reason,
			Result: map[string]any{"status": "rejected", "rejection_code": "free_state_mutation_forbidden", "mutation_performed": false}}
		if e.server != nil {
			e.server.emitToolItemCompleted(in, out, nil)
		}
		return out, nil
	}
	if response, needsConfirmation := typedPluginApplyConfirmationResponse(harness.InvokeRequest{
		Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context), Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull,
	}); needsConfirmation {
		out := executorpkg.Result{
			ToolCallID:           toolCallID,
			Tool:                 response.Tool,
			CommandName:          response.CommandName,
			Status:               response.Status,
			RequiresConfirmation: true,
			Preview:              response.Preview,
			Result:               response.Result,
			Response:             response,
		}
		if e.server != nil {
			e.server.emitToolItemCompleted(in, out, nil)
		}
		return out, nil
	}
	if e.server != nil {
		e.server.emitToolItemStarted(in, toolCallID)
	}
	if e.server != nil && isPluginGrabberApplyEQEditsToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyEQEdits(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberSetEQPointToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberSetEQPoint(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectGateExpanderToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectGateExpander(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyGateExpanderToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyGateExpander(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectTransientShaperToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectTransientShaper(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyTransientShaperToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyTransientShaper(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectMultibandToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectMultiband(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyMultibandToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyMultiband(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectLimiterToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectLimiter(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectDeEsserToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectDeEsser(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyDeEsserToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyDeEsser(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectSpectralDynamicsToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectSpectralDynamics(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyLimiterToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyLimiter(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberInspectCompressorToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberInspectCompressor(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if e.server != nil && isPluginGrabberApplyCompressorToolCall(in.ToolCall) {
		out, err := e.invokePluginGrabberApplyCompressor(ctx, in, toolCallID)
		e.server.emitToolItemCompleted(in, out, err)
		return out, err
	}
	if toolpolicy.IsRetiredPluginControlTool(firstNonEmpty(strings.TrimSpace(in.ToolCall.Tool), cleanContextText(in.ToolCall.Command["cmd"]), cleanContextText(in.ToolCall.Command["command"]), cleanContextText(in.ToolCall.Command["tool"]))) {
		out := executorpkg.Result{
			ToolCallID:  toolCallID,
			Tool:        strings.TrimSpace(in.ToolCall.Tool),
			CommandName: "plugin_control_retired",
			Status:      "error",
			Error:       "retired plugin control surfaces are unavailable; inspect live parameters or use a deterministic typed tool",
		}
		if e.server != nil {
			e.server.emitToolItemCompleted(in, out, nil)
		}
		return out, nil
	}
	if e.server != nil && agentLoopSelectedPluginEQProviderFallbackLoadBlocked(in) {
		out := executorpkg.Result{
			ToolCallID:  toolCallID,
			Tool:        firstNonEmpty(strings.TrimSpace(in.ToolCall.Tool), "plugin.load_to_rack"),
			CommandName: "plugin.load_to_rack",
			Status:      "ok",
			Result: map[string]any{
				"status":                "blocked",
				"blocker":               "selected_plugin_provider_fallback_forbidden",
				"message":               "当前已选择一个插件实例；普通 EQ 控制不能因为该实例没有 verified Provider 而自动加载或替换为另一款插件。请使用当前实例的 staging/verified 路径，或明确要求加载指定插件。",
				"required_control_tool": "plugin_grabber.apply_eq_edits",
				"plugin_loading":        "requires_explicit_user_request",
			},
		}
		e.server.emitToolItemCompleted(in, out, nil)
		return out, nil
	}
	if e.server != nil && isPluginSetParameterToolCall(in.ToolCall) {
		if out, blocked := e.blockEQOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockMultibandOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockTransientShaperOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockGateExpanderOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockLimiterOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockDeEsserOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
		if out, blocked := e.blockCompressorOwnedGenericParameterWrite(ctx, in, toolCallID); blocked {
			e.server.emitToolItemCompleted(in, out, nil)
			return out, nil
		}
	}
	out, err := e.base.RunToolCall(ctx, in)
	if e.server != nil {
		if strings.TrimSpace(out.ToolCallID) == "" {
			out.ToolCallID = toolCallID
		}
		e.server.emitToolItemCompleted(in, out, err)
	}
	return out, err
}

func (e pluginGrabberWorkflowExecutor) blockEQOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input, toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardEQOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName, Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (e pluginGrabberWorkflowExecutor) blockMultibandOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input, toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardMultibandOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName, Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func isPluginGrabberInspectGateExpanderToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectGateExpanderTool || name == pluginGrabberInspectGateExpanderCommand
}

func isPluginGrabberApplyGateExpanderToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyGateExpanderTool || name == pluginGrabberApplyGateExpanderCommand
}

func isPluginGrabberInspectTransientShaperToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectTransientShaperTool || name == pluginGrabberInspectTransientShaperCommand
}

func isPluginGrabberApplyTransientShaperToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyTransientShaperTool || name == pluginGrabberApplyTransientShaperCommand
}

func isPluginGrabberInspectMultibandToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectMultibandTool || name == pluginGrabberInspectMultibandCommand
}

func isPluginGrabberApplyMultibandToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyMultibandTool || name == pluginGrabberApplyMultibandCommand
}

func isPluginGrabberInspectLimiterToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectLimiterTool || name == pluginGrabberInspectLimiterCommand
}

func isPluginGrabberInspectDeEsserToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectDeEsserTool || name == pluginGrabberInspectDeEsserCommand
}

func isPluginGrabberApplyDeEsserToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyDeEsserTool || name == pluginGrabberApplyDeEsserCommand
}

func isPluginGrabberInspectSpectralDynamicsToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectSpectralDynamicsTool || name == pluginGrabberInspectSpectralDynamicsCommand
}

func isPluginGrabberApplyLimiterToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyLimiterTool || name == pluginGrabberApplyLimiterCommand
}

func isPluginGrabberInspectCompressorToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberInspectCompressorTool || name == pluginGrabberInspectCompressorCommand
}

func isPluginGrabberApplyCompressorToolCall(call planner.ToolCall) bool {
	name := agentLoopPluginToolCallName(call)
	return name == pluginGrabberApplyCompressorTool || name == pluginGrabberApplyCompressorCommand
}

func isPluginSetParameterToolCall(call planner.ToolCall) bool {
	switch agentLoopPluginToolCallName(call) {
	case "plugin.set_parameter", "plugin_set_parameter", "set_plugin_param":
		return true
	default:
		return false
	}
}

func agentLoopPluginToolCallName(call planner.ToolCall) string {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name != "" && name != "daw.invoke" && name != "daw_invoke" {
		return name
	}
	for _, row := range []map[string]any{workflowCommandArgs(call.Command), workflowCommandArgs(call.Args)} {
		if name = strings.ToLower(firstNonEmpty(cleanContextText(row["cmd"]), cleanContextText(row["command"]), cleanContextText(row["tool"]))); name != "" {
			return name
		}
	}
	return strings.ToLower(strings.TrimSpace(call.Tool))
}

func pluginToolCallArgs(call planner.ToolCall) map[string]any {
	out := workflowCommandArgs(call.Command)
	for key, value := range workflowCommandArgs(call.Args) {
		out[key] = value
	}
	return out
}

func (e pluginGrabberWorkflowExecutor) blockCompressorOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardCompressorOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName,
		Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (e pluginGrabberWorkflowExecutor) blockLimiterOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardLimiterOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName,
		Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (e pluginGrabberWorkflowExecutor) blockGateExpanderOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardGateExpanderOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName,
		Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (e pluginGrabberWorkflowExecutor) blockTransientShaperOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardTransientShaperOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName, Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (s *Server) guardTransientShaperOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveTransientShaperControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch transientShaperControlFailureCode(err) {
		case "not_transient_shaper", "unsupported_broadband_compressor", "unsupported_limiter", "unsupported_multiband_dynamics", "unsupported_gate_expander", "unsupported_de_esser", "unsupported_spectral_dynamics", "unsupported_clipper":
			return harness.InvokeResponse{}, false
		default:
			return transientShaperGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_transient_shaper_surface_unavailable", "cannot prove this parameter is outside the typed transient-shaper surface: "+err.Error()), true
		}
	}
	if !transientShaperSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return transientShaperGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_transient_shaper_control_required", "parameter is owned by the live transient-shaper topology; use inspect_transient_shaper and apply_transient_shaper_controls"), true
}

func transientShaperGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID, "required_tools": []string{pluginGrabberInspectTransientShaperTool, pluginGrabberApplyTransientShaperTool}, "parameters_changed": false}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func transientShaperSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	stage := mapValue(summary["transient_shaper_stage"])
	for _, section := range []string{"envelope_action", "detector", "timing", "shape", "mode", "output"} {
		for _, binding := range mapRowsValue(stage[section]) {
			if firstNonEmptyText(binding, "param_id") == paramID {
				return true
			}
		}
	}
	return false
}

func (s *Server) guardGateExpanderOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveGateExpanderControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch gateExpanderControlFailureCode(err) {
		case "not_gate_expander", "unsupported_broadband_compressor", "unsupported_limiter", "unsupported_multiband_dynamics", "unsupported_spectral_dynamics", "unsupported_de_esser", "unsupported_transient_shaper", "unsupported_clipper":
			return harness.InvokeResponse{}, false
		default:
			return gateExpanderGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_gate_expander_surface_unavailable", "cannot prove this parameter is outside the typed gate/expander surface: "+err.Error()), true
		}
	}
	if !gateExpanderSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return gateExpanderGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_gate_expander_control_required", "parameter is owned by the live gate/expander topology; use inspect_gate_expander and apply_gate_expander_controls"), true
}

func gateExpanderGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID,
		"required_tools": []string{pluginGrabberInspectGateExpanderTool, pluginGrabberApplyGateExpanderTool}, "parameters_changed": false}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func gateExpanderSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	stage := mapValue(summary["gate_expander_stage"])
	for _, section := range []string{"detector", "operating_point", "gain_action", "direction_control", "timing", "mode", "output"} {
		for _, binding := range mapRowsValue(stage[section]) {
			if firstNonEmptyText(binding, "param_id") == paramID {
				return true
			}
		}
	}
	return false
}

func (s *Server) guardLimiterOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveLimiterControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch limiterControlFailureCode(err) {
		case "not_limiter", "unsupported_broadband_compressor", "unsupported_clipper", "unsupported_multiband_dynamics", "unsupported_gate_expander":
			return harness.InvokeResponse{}, false
		default:
			return limiterGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_limiter_surface_unavailable", "cannot prove this parameter is outside the typed limiter surface: "+err.Error()), true
		}
	}
	if !limiterSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return limiterGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_limiter_control_required", "parameter is owned by the live limiter topology; use inspect_limiter and apply_limiter_controls"), true
}

func limiterGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID,
		"required_tools": []string{pluginGrabberInspectLimiterTool, pluginGrabberApplyLimiterTool}, "parameters_changed": false}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func limiterSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	for _, stage := range mapRowsValue(summary["limiter_stages"]) {
		for _, section := range []string{"operating_point", "safety", "timing", "detector", "mode", "output"} {
			for _, binding := range mapRowsValue(stage[section]) {
				if firstNonEmptyText(binding, "param_id") == paramID {
					return true
				}
			}
		}
	}
	for _, binding := range mapRowsValue(summary["shared_controls"]) {
		if firstNonEmptyText(binding, "param_id") == paramID {
			return true
		}
	}
	return false
}

func (e pluginGrabberWorkflowExecutor) blockDeEsserOwnedGenericParameterWrite(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, bool) {
	req := harness.InvokeRequest{Tool: in.ToolCall.Tool, Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command), Context: cloneStringAnyMap(in.Context)}
	resp, blocked := e.server.guardDeEsserOwnedGenericParameterWrite(ctx, req)
	if !blocked {
		return executorpkg.Result{}, false
	}
	return executorpkg.Result{ToolCallID: toolCallID, Tool: resp.Tool, CommandName: resp.CommandName, Status: resp.Status, Error: resp.Error, Result: resp.Result, Response: resp}, true
}

func (s *Server) guardDeEsserOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveDeEsserControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch deEsserControlFailureCode(err) {
		case "not_de_esser", "unsupported_broadband_compressor", "unsupported_limiter", "unsupported_multiband_dynamics", "unsupported_gate_expander", "unsupported_spectral_dynamics", "unsupported_clipper":
			return harness.InvokeResponse{}, false
		default:
			return deEsserGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_de_esser_surface_unavailable", "cannot prove this parameter is outside the typed De-esser surface: "+err.Error()), true
		}
	}
	if !deEsserSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return deEsserGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_de_esser_control_required", "parameter is owned by the live De-esser topology; use inspect_de_esser and apply_de_esser_controls"), true
}

func deEsserGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID, "required_tools": []string{pluginGrabberInspectDeEsserTool, pluginGrabberApplyDeEsserTool}, "parameters_changed": false}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func deEsserSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	for _, stage := range mapRowsValue(summary["de_esser_stages"]) {
		for _, section := range []string{"operating_point", "frequency_selectivity", "timing", "mode", "output"} {
			for _, binding := range mapRowsValue(stage[section]) {
				if firstNonEmptyText(binding, "param_id") == paramID {
					return true
				}
			}
		}
	}
	return false
}

func (s *Server) guardCompressorOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveCompressorControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch compressorControlFailureCode(err) {
		case "not_compressor", "unsupported_limiter", "unsupported_multiband_compressor", "unsupported_gate_expander":
			return harness.InvokeResponse{}, false
		}
		return compressorGenericWriteBlockedResponse(trackID, pluginID, paramID,
			"typed_compressor_surface_unavailable", "cannot prove this parameter is outside the typed compressor surface: "+err.Error()), true
	}
	if !compressorSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return compressorGenericWriteBlockedResponse(trackID, pluginID, paramID,
		"typed_compressor_control_required", "parameter is owned by the live broadband-compressor topology; use inspect_compressor and apply_compressor_controls"), true
}

func (s *Server) guardEQOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveEQControlSurface(ctx, trackID, pluginID)
	if err != nil {
		if eqControlFailureCode(err) == "not_static_eq" {
			return harness.InvokeResponse{}, false
		}
		return eqGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_eq_surface_unavailable", "cannot prove this parameter is outside the live static-EQ topology: "+err.Error()), true
	}
	owned := semanticSurfaceOwnedParameterSet(summary)
	if len(owned) == 0 {
		return eqGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_eq_surface_unavailable", "the live static-EQ topology exposes no proven parameter ownership"), true
	}
	if !owned[paramID] {
		return harness.InvokeResponse{}, false
	}
	return eqGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_eq_control_required", "parameter is owned by the live static-EQ topology; use plugin_grabber.explain_controls and plugin_grabber.apply_eq_edits"), true
}

func eqGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID,
		"required_tools": []string{"plugin_grabber.explain_controls", "plugin_grabber.apply_eq_edits"}, "parameters_changed": false}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func semanticSurfaceOwnedParameterSet(topology map[string]any) map[string]bool {
	set := map[string]bool{}
	for _, parameterID := range semanticSurfaceOwnedParameterIDs(topology) {
		set[parameterID] = true
	}
	return set
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectLimiter(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberInspectLimiterInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectLimiterCommand
	}
	resp, err := e.server.invokePluginGrabberInspectLimiterWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectDeEsser(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberInspectDeEsserInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectDeEsserCommand
	}
	resp, err := e.server.invokePluginGrabberInspectDeEsserWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyDeEsser(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberApplyDeEsserInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyDeEsserCommand
	}
	resp, err := e.server.invokePluginGrabberApplyDeEsserWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectSpectralDynamics(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberInspectSpectralDynamicsInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectSpectralDynamicsCommand
	}
	resp, err := e.server.invokePluginGrabberInspectSpectralDynamicsWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectGateExpander(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberInspectGateExpanderInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectGateExpanderCommand
	}
	resp, err := e.server.invokePluginGrabberInspectGateExpanderWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyGateExpander(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberApplyGateExpanderInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyGateExpanderCommand
	}
	resp, err := e.server.invokePluginGrabberApplyGateExpanderWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectTransientShaper(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberInspectTransientShaperInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectTransientShaperCommand
	}
	resp, err := e.server.invokePluginGrabberInspectTransientShaperWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyTransientShaper(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberApplyTransientShaperInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyTransientShaperCommand
	}
	resp, err := e.server.invokePluginGrabberApplyTransientShaperWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectMultiband(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberInspectMultibandInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectMultibandCommand
	}
	resp, err := e.server.invokePluginGrabberInspectMultibandWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyMultiband(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID}
	workflowCmd, ok := pluginGrabberApplyMultibandInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyMultibandCommand
	}
	resp, err := e.server.invokePluginGrabberApplyMultibandWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyLimiter(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberApplyLimiterInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyLimiterCommand
	}
	resp, err := e.server.invokePluginGrabberApplyLimiterWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func pluginSetParameterInvokeRequest(req harness.InvokeRequest) bool {
	name := strings.ToLower(strings.TrimSpace(req.Tool))
	if name == "plugin.set_parameter" || name == "plugin_set_parameter" || name == "set_plugin_param" {
		return true
	}
	for _, row := range []map[string]any{workflowCommandArgs(req.Command), workflowCommandArgs(req.Args)} {
		name = strings.ToLower(firstNonEmptyText(row, "cmd", "command", "tool"))
		if name == "plugin.set_parameter" || name == "plugin_set_parameter" || name == "set_plugin_param" {
			return true
		}
	}
	return false
}

func compressorGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	result := map[string]any{
		"status": "rejected", "rejection_code": code, "message": full,
		"track_id": trackID, "plugin_id": pluginID, "param_id": paramID,
		"required_tools":     []string{pluginGrabberInspectCompressorTool, pluginGrabberApplyCompressorTool},
		"parameters_changed": false,
	}
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param",
		RiskLevel: tools.RiskUndoable, Error: full, Result: result}
}

func compressorSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	stage := mapValue(summary["compressor_stage"])
	for _, path := range mapRowsValue(stage["control_paths"]) {
		for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
			for _, binding := range mapRowsValue(path[section]) {
				if firstNonEmptyText(binding, "param_id") == paramID {
					return true
				}
			}
		}
	}
	for _, binding := range mapRowsValue(stage["output"]) {
		if firstNonEmptyText(binding, "param_id") == paramID {
			return true
		}
	}
	return false
}

func isPluginGrabberSetEQPointToolCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name == "plugin_grabber.set_eq_point" || name == "plugin_grabber_set_eq_point" {
		return true
	}
	cmd := workflowCommandArgs(call.Command)
	name = strings.ToLower(firstNonEmpty(cleanContextText(cmd["cmd"]), cleanContextText(cmd["command"]), cleanContextText(cmd["tool"])))
	return name == "plugin_grabber.set_eq_point" || name == "plugin_grabber_set_eq_point"
}

func isPluginGrabberApplyEQEditsToolCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name == pluginGrabberApplyEQEditsTool || name == pluginGrabberApplyEQEditsCommand {
		return true
	}
	cmd := workflowCommandArgs(call.Command)
	name = strings.ToLower(firstNonEmpty(cleanContextText(cmd["cmd"]), cleanContextText(cmd["command"]), cleanContextText(cmd["tool"])))
	return name == pluginGrabberApplyEQEditsTool || name == pluginGrabberApplyEQEditsCommand
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyEQEdits(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args),
		Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID),
		Source:  firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"), Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull,
		GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberApplyEQEditsInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyEQEditsCommand
	}
	resp, err := e.server.invokePluginGrabberApplyEQEditsWorkflow(ctx, req, workflowCmd)
	out := executorpkg.Result{
		ToolCallID: toolCallID, Tool: firstNonEmpty(resp.Tool, in.ToolCall.Tool, pluginGrabberApplyEQEditsTool),
		CommandName: resp.CommandName, AgentActionID: resp.AgentActionID, Status: resp.Status,
		RequiresConfirmation: resp.RequiresConfirmation || resp.Status == "needs_confirmation",
		Preview:              resp.Preview, UndoLabel: resp.UndoLabel, Result: resp.Result,
		ProjectHistory: resp.ProjectHistory, Error: resp.Error, Response: resp,
	}
	if err != nil && out.Error == "" {
		out.Error = err.Error()
	}
	if !out.RequiresConfirmation && err == nil && out.Status != "error" && e.server.harness != nil {
		out.ObservedState = e.server.harness.UserStateSummary(ctx)
	}
	return out, err
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberSetEQPoint(ctx context.Context, in executorpkg.Input, toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool:       strings.TrimSpace(in.ToolCall.Tool),
		Args:       cloneStringAnyMap(in.ToolCall.Args),
		Command:    cloneStringAnyMap(in.ToolCall.Command),
		Context:    contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID),
		Source:     firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed:  in.Confirmed,
		GoalID:     in.GoalID,
		RunID:      in.RunID,
		ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberSetEQPointInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberSetEQPointCommand
	}
	resp, err := e.server.invokePluginGrabberSetEQPointWorkflow(ctx, req, workflowCmd)
	out := executorpkg.Result{
		ToolCallID:           toolCallID,
		Tool:                 firstNonEmpty(resp.Tool, in.ToolCall.Tool, pluginGrabberSetEQPointTool),
		CommandName:          resp.CommandName,
		AgentActionID:        resp.AgentActionID,
		Status:               resp.Status,
		RequiresConfirmation: resp.RequiresConfirmation || resp.Status == "needs_confirmation",
		Preview:              resp.Preview,
		UndoLabel:            resp.UndoLabel,
		Result:               resp.Result,
		ProjectHistory:       resp.ProjectHistory,
		Error:                resp.Error,
		Response:             resp,
	}
	if err != nil && out.Error == "" {
		out.Error = err.Error()
	}
	if !out.RequiresConfirmation && err == nil && out.Status != "error" && e.server.harness != nil {
		out.ObservedState = e.server.harness.UserStateSummary(ctx)
	}
	return out, err
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberInspectCompressor(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberInspectCompressorInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberInspectCompressorCommand
	}
	resp, err := e.server.invokePluginGrabberInspectCompressorWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) invokePluginGrabberApplyCompressor(ctx context.Context, in executorpkg.Input,
	toolCallID string) (executorpkg.Result, error) {
	req := harness.InvokeRequest{
		Tool: strings.TrimSpace(in.ToolCall.Tool), Args: cloneStringAnyMap(in.ToolCall.Args), Command: cloneStringAnyMap(in.ToolCall.Command),
		Context: contextWithAgentLoopIDs(in.Context, in.GoalID, in.RunID, toolCallID), Source: firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
		Confirmed: in.Confirmed || authorityModeFromContext(in.Context) == experiment.AuthorityFull, GoalID: in.GoalID, RunID: in.RunID, ToolCallID: toolCallID,
	}
	workflowCmd, ok := pluginGrabberApplyCompressorInvokeCommand(req)
	if !ok {
		workflowCmd = cloneStringAnyMap(in.ToolCall.Args)
		workflowCmd["cmd"] = pluginGrabberApplyCompressorCommand
	}
	resp, err := e.server.invokePluginGrabberApplyCompressorWorkflow(ctx, req, workflowCmd)
	return e.compressorWorkflowResult(ctx, in, toolCallID, resp, err)
}

func (e pluginGrabberWorkflowExecutor) compressorWorkflowResult(ctx context.Context, in executorpkg.Input, toolCallID string,
	resp harness.InvokeResponse, err error) (executorpkg.Result, error) {
	out := executorpkg.Result{
		ToolCallID: toolCallID, Tool: firstNonEmpty(resp.Tool, in.ToolCall.Tool), CommandName: resp.CommandName,
		AgentActionID: resp.AgentActionID, Status: resp.Status, RequiresConfirmation: resp.RequiresConfirmation || resp.Status == "needs_confirmation",
		Preview: resp.Preview, UndoLabel: resp.UndoLabel, Result: resp.Result, ProjectHistory: resp.ProjectHistory, Error: resp.Error, Response: resp,
	}
	if err != nil && out.Error == "" {
		out.Error = err.Error()
	}
	if !out.RequiresConfirmation && err == nil && out.Status != "error" && e.server.harness != nil {
		out.ObservedState = e.server.harness.UserStateSummary(ctx)
	}
	return out, err
}

func agentLoopSelectedPluginEQProviderFallbackLoadBlocked(in executorpkg.Input) bool {
	if !agentLoopPluginLoadToolCall(in.ToolCall) {
		return false
	}
	if !contextHasAnyValue(in.Context, "selected_plugin_id", "primary_selected_plugin_id") {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(cleanContextText(in.Context["user_message"])))
	if !agentLoopTextHasAny(text,
		"均衡", "低切", "高切", "高通", "低通", "eq", "equalizer", "highpass", "high-pass", "lowpass", "low-pass",
	) {
		return false
	}
	return !agentLoopExplicitPluginLoadRequest(text)
}

func agentLoopPluginLoadToolCall(call planner.ToolCall) bool {
	name := strings.ToLower(strings.TrimSpace(call.Tool))
	if name == "" {
		command := workflowCommandArgs(call.Command)
		name = strings.ToLower(firstNonEmpty(cleanContextText(command["cmd"]), cleanContextText(command["command"]), cleanContextText(command["tool"])))
	}
	switch name {
	case "plugin.load_to_rack", "rack.add_node", "rack_add_node", "instantiate_plugin", "plugin.instantiate":
		return true
	default:
		return false
	}
}

func agentLoopExplicitPluginLoadRequest(text string) bool {
	return agentLoopTextHasAny(text,
		"加载一个", "加载插件", "添加插件", "插入插件", "挂载插件", "新建插件", "加载 tdr", "加载 nova", "加载 pro-q",
		"load a plugin", "load plugin", "add plugin", "insert plugin", "instantiate plugin", "load tdr", "load nova", "load pro-q",
	)
}

func contextWithAgentLoopIDs(ctx map[string]any, goalID, runID, toolCallID string) map[string]any {
	out := cloneStringAnyMap(ctx)
	if out == nil {
		out = map[string]any{}
	}
	if strings.TrimSpace(goalID) != "" {
		out["goal_id"] = goalID
	}
	if strings.TrimSpace(runID) != "" {
		out["run_id"] = runID
	}
	if strings.TrimSpace(toolCallID) != "" {
		out["tool_call_id"] = toolCallID
	}
	return out
}

func cloneStringAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func useLegacyPlannerLoop() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("VIT_AGENT_USE_LEGACY_PLANNER_LOOP")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

// Keep one /agent/chat invocation inside the frozen 420-second transport
// boundary even if the next LLM request consumes all three 60-second retries.
// Longer goals resume from the continuation checkpoint.
const agentMessageLoopInvocationTimeout = 3 * time.Minute

func agentLoopBudgetForMode(mode string) agentloop.Budget {
	switch agentModeFromString(mode) {
	case agentModeGoal:
		return agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16, Timeout: agentMessageLoopInvocationTimeout}
	case agentModePlan:
		return agentloop.Budget{MaxTurns: 8, MaxToolCalls: 12, Timeout: agentMessageLoopInvocationTimeout}
	default:
		return agentloop.Budget{MaxTurns: 8, MaxToolCalls: 12, Timeout: agentMessageLoopInvocationTimeout}
	}
}

func agentLoopBudgetForModeAfter(mode string, elapsed time.Duration) agentloop.Budget {
	budget := agentLoopBudgetForMode(mode)
	if budget.Timeout <= 0 || elapsed <= 0 {
		return budget
	}
	budget.Timeout -= elapsed
	if budget.Timeout <= 0 {
		budget.Timeout = time.Nanosecond
	}
	return budget
}
func (s *Server) chatResponseFromAgentLoopResult(conversationID, mode string, res agentloop.Result) ChatResponse {
	res = s.applyLegacyCapabilityCreationGate(res)
	if err := s.recordGoalResult(conversationID, res); err != nil {
		res.Status = agentruntime.StatusFailed
		res.StopReason = "durable_checkpoint_persist_failed"
		res.Error = err.Error()
		res.Continuation = nil
	}
	if res.Status == agentruntime.StatusWaitingContinue && res.Continuation == nil {
		res.Status = agentruntime.StatusFailed
		res.StopReason = "durable_continuation_missing"
		res.Error = "waiting_continue requires a durable continuation checkpoint"
	}
	reply := strings.TrimSpace(res.Reply)
	if reply == "" {
		switch res.Status {
		case agentruntime.StatusWaitingConfirmation:
			reply = "\u9700\u8981\u4f60\u786e\u8ba4\u540e\u518d\u7ee7\u7eed\u3002"
		case agentruntime.StatusWaitingClarification:
			reply = firstNonEmpty(res.ClarificationQuestion, "\u9700\u8981\u4f60\u5148\u8865\u5145\u4e00\u4e2a\u7f16\u8f91\u76ee\u6807\u3002")
		case agentruntime.StatusWaitingContinue:
			if res.StopReason == agentloop.StopReasonInterjection {
				reply = "\u672c\u8f6e\u5df2\u6682\u505c\uff0c\u7b49\u5f85\u5904\u7406\u65b0\u7684\u7528\u6237\u8f93\u5165\u3002"
			} else {
				reply = "\u5f53\u524d\u6267\u884c\u5207\u7247\u5df2\u7ed3\u675f\uff0c\u4efb\u52a1\u4f1a\u81ea\u52a8\u7ee7\u7eed\u3002"
			}
		case agentruntime.StatusCompleted:
			reply = "\u5df2\u5b8c\u6210\u3002"
		case agentruntime.StatusCancelled:
			reply = "\u5df2\u53d6\u6d88\u3002"
		case agentruntime.StatusFailed:
			reply = firstNonEmpty(res.Error, "\u6267\u884c\u5931\u8d25\u3002")
		}
	}
	if res.Status == agentruntime.StatusFailed && len(res.Executed) > 0 {
		reply = "\u524d\u9762\u7684\u5de5\u5177\u64cd\u4f5c\u5df2\u5b8c\u6210\uff0c\u4f46\u6700\u7ec8\u56de\u590d\u751f\u6210\u5931\u8d25\uff1a" + firstNonEmpty(res.Error, reply)
	}
	if res.StopReason == agentloop.StopReasonLimitReached && !strings.Contains(reply, "\u81ea\u52a8\u7ee7\u7eed") {
		reply += " \u4efb\u52a1\u4f1a\u4ece\u5df2\u4fdd\u5b58\u7684\u68c0\u67e5\u70b9\u81ea\u52a8\u7ee7\u7eed\u3002"
	}
	if chatResponseLooksMixRelated(res) {
		reply = localizedDisplayTextFallback(reply)
	}
	visibleExecuted := compactAgentLoopExecutedForResponse(res.Executed)
	resp := ChatResponse{
		ConversationID:      conversationID,
		TaskID:              res.TaskID,
		GoalID:              res.GoalID,
		RunID:               res.RunID,
		SliceID:             res.SliceID,
		TurnID:              res.TurnID,
		OriginalIntent:      res.OriginalIntent,
		Reply:               reply,
		AgentMode:           mode,
		NeedsConfirmation:   res.Status == agentruntime.StatusWaitingConfirmation,
		Preview:             res.Preview,
		ExecutedKernelReply: visibleExecuted,
		InteractionRequests: agentLoopInteractionRequests(res.Executed),
		ProjectResultCards:  nil,
		Artifacts:           artifactSummariesFromExecuted(visibleExecuted),
		GoalStatus:          string(res.Status),
		GoalSummary:         res.GoalSummary,
		CurrentStep:         res.CurrentStep,
		CompletedSteps:      res.CompletedSteps,
		StopReason:          res.StopReason,
		LimitType:           res.LimitType,
		ProjectHistory:      res.ProjectHistory,
		Error:               res.Error,
		AgentPlan:           agentPlanForMode(mode, agentPlanFromAgentLoopResult(res)),
	}
	if res.Status == agentruntime.StatusWaitingContinue || res.StopReason == "durable_continuation_missing" {
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
			"continuation_id":       continuationIDForResult(res),
			"checkpoint_status":     string(res.Status),
			"checkpoint_persisted":  res.Status == agentruntime.StatusWaitingContinue && res.Continuation != nil,
			"scheduler_eligible":    res.Status == agentruntime.StatusWaitingContinue && res.Continuation != nil,
		})
	}
	if !chatResponseTurnFailed(resp) {
		resp.ProjectResultCards = projectResultCardsFromExecuted(visibleExecuted)
	}
	attachAcousticPackageStatusFromAgentLoopResult(&resp, res.Executed)
	if res.Status == agentruntime.StatusWaitingConfirmation && res.Continuation != nil {
		decisions := agentLoopPendingDecisions(res.Continuation)
		responseDecisions := compactAgentLoopDecisionsForResponse(decisions)
		plan := PendingPlan{
			ID:               "plan_" + randomID(),
			CreatedAt:        time.Now(),
			Decisions:        decisions,
			Context:          contextWithGoal(res.Continuation.Context, res.GoalID, res.RunID),
			Preview:          res.Preview,
			Workflow:         agentLoopConfirmationWorkflow,
			WorkflowData:     map[string]any{"conversation_id": conversationID},
			GoalContinuation: res.Continuation,
		}
		s.mu.Lock()
		s.pending[plan.ID] = plan
		s.mu.Unlock()
		resp.PlanID = plan.ID
		resp.NeedsConfirmation = true
		resp.Commands = responseDecisions
		if event := typedApprovalEventFromPendingTool(plan.ID, res.Continuation, responseDecisions, conversationID); event != nil {
			resp.TypedEvents = append(resp.TypedEvents, event)
		}
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil && strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
		typed := candidate.ToPendingCandidate(conversationID, res.GoalID, res.RunID, "")
		s.upsertPendingCandidate(typed)
		resp.NeedsConfirmation = true
		resp.GoalStatus = string(agentruntime.StatusWaitingConfirmation)
		resp.StopReason = agentloop.StopReasonNeedsConfirmation
		resp.Workflow = "mix_tick"
		resp.WorkflowData = typedPendingPayload(pendingMixTickEventPayload(*candidate, candidate.ObservationID), typed)
		resp.TypedEvents = append(resp.TypedEvents, agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source)))
		req := mixTickInteractionRequest(conversationID, res.GoalID, res.RunID, *candidate)
		resp.InteractionRequests = []AgentInteractionRequest{req}
		s.storePendingInteraction(req, req.Payload)
		if resp.AgentPlan != nil {
			resp.AgentPlan.Status = string(agentruntime.StatusWaitingConfirmation)
		}
	}
	if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil && strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
		if localized := localizedMixTreatmentUserReply(resp.Reply, *treatment); localized != "" {
			resp.Reply = localized
		}
		typed := treatment.ToPendingCandidate(conversationID, res.GoalID, res.RunID, "")
		s.upsertPendingCandidate(typed)
		resp.NeedsConfirmation = true
		resp.GoalStatus = string(agentruntime.StatusWaitingConfirmation)
		resp.StopReason = agentloop.StopReasonNeedsConfirmation
		resp.Workflow = "mix_treatment"
		resp.WorkflowData = mixTreatmentInteractionPayload(*treatment)
		resp.WorkflowData = typedPendingPayload(resp.WorkflowData, typed)
		resp.TypedEvents = append(resp.TypedEvents, agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source)))
		req := mixTreatmentInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, *treatment)
		resp.InteractionRequests = []AgentInteractionRequest{req}
		s.storePendingInteraction(req, req.Payload)
		if resp.AgentPlan != nil {
			resp.AgentPlan.Status = string(agentruntime.StatusWaitingConfirmation)
		}
	}
	if len(resp.WorkflowData) == 0 && strings.TrimSpace(res.ExecutionMemory.MixDiagnosisContextID) != "" {
		resp.WorkflowData = mixDiagnosisContextPayload(res.ExecutionMemory)
	}
	return resp
}

// Agent-loop tools keep their original response under executed[].result. A
// formal interaction there is still the card the UI must render; promoting it
// prevents a generic confirmation card from replacing a workflow-specific
// choice supplied by a read-only observation workflow.
func agentLoopInteractionRequests(executed []map[string]any) []AgentInteractionRequest {
	requests := make([]AgentInteractionRequest, 0)
	seen := map[string]bool{}
	for _, row := range executed {
		result := mapValue(row["result"])
		for _, request := range interactionRequestsFromAny(result["interaction_requests"]) {
			id := strings.TrimSpace(request.ID)
			if id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			requests = append(requests, request)
		}
	}
	return requests
}

func chatResponseLooksMixRelated(res agentloop.Result) bool {
	if res.ExecutionMemory.PendingMixTreatment != nil || res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingStaticBalancePlan != nil || res.ExecutionMemory.PendingPanLayoutPlan != nil {
		return true
	}
	for _, executed := range res.Executed {
		tool := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(executed["tool"]), cleanContextText(executed["command_name"]))))
		if tool == "" {
			if result := mapValue(executed["result"]); len(result) > 0 && (result["acoustic_package_status"] != nil || result["mom_projection"] != nil) {
				return true
			}
			continue
		}
		if strings.Contains(tool, "mix.observe") || strings.Contains(tool, "mix.read") || strings.Contains(tool, "mix.request_observation") || strings.Contains(tool, "mix.derive") {
			return true
		}
		if result := mapValue(executed["result"]); len(result) > 0 && (result["acoustic_package_status"] != nil || result["mom_projection"] != nil) {
			return true
		}
	}
	return false
}

func mixTreatmentInteractionPayload(treatment agentloop.MixTreatmentPending) map[string]any {
	display := localizedMixTreatmentDisplayPayload(treatment)
	payload := map[string]any{
		"schema_version":       treatment.SchemaVersion,
		"status":               treatment.Status,
		"intent":               treatment.Intent,
		"target_ref":           treatment.TargetRef,
		"action_kind":          treatment.ActionKind,
		"processor_type":       treatment.ProcessorType,
		"delta_db":             treatment.DeltaDB,
		"delta_pan":            treatment.DeltaPan,
		"plugin_id":            treatment.PluginID,
		"plugin_name":          treatment.PluginName,
		"control":              treatment.Control,
		"target":               cloneContext(treatment.Target),
		"confidence":           treatment.Confidence,
		"reasoning_summary":    treatment.ReasoningSummary,
		"evidence_refs":        append([]string(nil), treatment.EvidenceRefs...),
		"diagnosis_context_id": treatment.DiagnosisContextID,
		"diagnosis_context":    cloneContext(treatment.DiagnosisContext),
		"needs_resolution":     append([]string(nil), treatment.NeedsResolution...),
		"observation_id":       treatment.ObservationID,
		"display":              display,
	}
	if value := cleanContextText(display["confidence"]); value != "" {
		payload["display_confidence"] = value
	}
	if value := cleanContextText(display["reasoning_summary"]); value != "" {
		payload["display_reasoning_summary"] = value
	}
	if value := cleanContextText(display["card_body"]); value != "" {
		payload["display_card_body"] = value
	}
	if value := contextStringSlice(display["needs_resolution"]); len(value) > 0 {
		payload["display_needs_resolution"] = value
	}
	if treatment.TargetPan != nil {
		payload["target_pan"] = *treatment.TargetPan
	}
	typed := treatment.ToPendingCandidate(treatment.ConversationID, "", "", "")
	payload = typedPendingPayload(payload, typed)
	removeEmptyTreatmentValues(payload)
	return payload
}

func mixDiagnosisContextPayload(memory agentloop.ExecutionMemory) map[string]any {
	payload := map[string]any{
		"schema_version":        "mix_diagnosis_context_payload.v0",
		"diagnosis_context_id":  strings.TrimSpace(memory.MixDiagnosisContextID),
		"diagnosis_context":     cloneContext(memory.MixDiagnosisContext),
		"mix_diagnosis_context": cloneContext(memory.MixDiagnosisContext),
	}
	removeEmptyTreatmentValues(payload)
	return payload
}

func typedPendingPayload(payload map[string]any, pending agentprotocol.PendingCandidate) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["typed_state"] = agentprotocol.ToMap(pending)
	payload["typed_event"] = agentprotocol.ToMap(agentprotocol.NewEvent(pending, pending.Source))
	return payload
}

func mixTreatmentInteractionRequest(conversationID, goalID, runID, reply string, treatment agentloop.MixTreatmentPending) AgentInteractionRequest {
	payload := mixTreatmentInteractionPayload(treatment)
	typed := treatment.ToPendingCandidate(conversationID, goalID, runID, "")
	payload = typedPendingPayload(payload, typed)
	payload["conversation_id"] = conversationID
	payload["request_context"] = contextWithGoal(map[string]any{"conversation_id": conversationID}, goalID, runID)
	body := firstNonEmpty(cleanContextText(payload["display_card_body"]), cleanContextText(payload["display_reasoning_summary"]), localizedMixTreatmentReasoning(treatment), strings.TrimSpace(reply), "这个混音动作正在等待确认。")
	return AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "mix_treatment_confirmation",
		Type:           "mix_treatment_confirmation",
		Source:         "vit_agent",
		Workflow:       "mix_treatment",
		Title:          "混音建议待确认",
		Body:           body,
		Status:         "waiting_for_user",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "确认执行", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
}

func mixTickInteractionRequest(conversationID, goalID, runID string, candidate agentloop.PendingMixTickCandidate) AgentInteractionRequest {
	typed := candidate.ToPendingCandidate(conversationID, goalID, runID, "")
	payload := typedPendingPayload(pendingMixTickEventPayload(candidate, candidate.ObservationID), typed)
	payload["conversation_id"] = conversationID
	payload["request_context"] = contextWithGoal(map[string]any{"conversation_id": conversationID}, goalID, runID)
	return AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "mix_tick_confirmation",
		Type:           "mix_tick_confirmation",
		Source:         "vit_agent",
		Workflow:       "mix_tick",
		Stage:          "pending_confirmation",
		Title:          "混音单步待确认",
		Body:           pendingMixTickEventBody(candidate),
		Status:         "waiting_for_user",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "确认执行", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
}

func agentLoopPendingDecisions(cont *agentloop.Continuation) []policy.Decision {
	if cont == nil || cont.PendingToolCall == nil {
		return nil
	}
	call := cont.PendingToolCall
	cmd := cloneStringAnyMap(call.Command)
	if len(cmd) == 0 {
		cmd = cloneStringAnyMap(call.Args)
	}
	if cmd == nil {
		cmd = map[string]any{}
	}
	if firstNonEmpty(fmt.Sprint(cmd["tool"])) == "" && strings.TrimSpace(call.Tool) != "" {
		cmd["tool"] = strings.TrimSpace(call.Tool)
	}
	if firstNonEmpty(fmt.Sprint(cmd["cmd"])) == "" && strings.TrimSpace(call.Tool) != "" {
		if spec, ok := tools.DefaultCatalog().LookupTool(strings.TrimSpace(call.Tool)); ok {
			cmd["cmd"] = spec.CommandName
		}
	}
	decision := policy.Classify(cmd)
	if strings.TrimSpace(decision.Reason) == "" {
		decision.Reason = strings.TrimSpace(call.Reason)
	}
	return []policy.Decision{decision}
}

func (s *Server) handleAgentLoopConfirm(w http.ResponseWriter, r *http.Request, planID string, plan PendingPlan, decision string) {
	status, response := s.resolveAgentLoopConfirm(r.Context(), planID, plan, decision)
	writeJSON(w, status, response)
}

func (s *Server) resolveAgentLoopConfirm(ctx context.Context, planID string, plan PendingPlan, decision string) (statusCode int, response map[string]any) {
	started := time.Now()
	defer func() {
		if s != nil && s.logger != nil {
			s.logger.Info("[timing] agent_loop_confirm total_ms=%d plan=%s status_code=%d response_status=%s goal=%s next_plan=%s needs_confirmation=%t err=%t",
				time.Since(started).Milliseconds(),
				planID,
				statusCode,
				strings.TrimSpace(fmt.Sprint(response["status"])),
				strings.TrimSpace(fmt.Sprint(response["goal_id"])),
				strings.TrimSpace(fmt.Sprint(response["next_plan_id"])),
				boolValue(response["needs_confirmation"]),
				strings.TrimSpace(fmt.Sprint(response["error"])) != "",
			)
		}
	}()
	goalID, runID := goalIDsFromContext(plan.Context)
	projectPath := projectPathFromChatContext(plan.Context)
	mode := agentModeFromContext(plan.Context)
	if projectPath == "" && plan.GoalContinuation != nil {
		projectPath = projectPathFromChatContext(plan.GoalContinuation.Context)
		mode = agentModeFromContext(plan.GoalContinuation.Context)
	}
	conversationID := strings.TrimSpace(fmt.Sprint(plan.WorkflowData["conversation_id"]))
	if !isApprovalDecision(decision) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		s.clearGoalContinuation(goalID)
		projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		return http.StatusOK, map[string]any{"status": "ok", "message": "cancelled", "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusCancelled), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory))}
	}
	if plan.GoalContinuation == nil {
		err := "goal confirmation continuation is missing"
		s.harness.CompleteGoal(goalID, errors.New(err))
		projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		return http.StatusOK, map[string]any{"status": "error", "message": err, "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusFailed), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", err, projectHistory))}
	}
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		msg := "AI 配置不可用，无法继续当前 Agent 任务。"
		if err != nil {
			msg += " " + err.Error()
		}
		s.harness.CompleteGoal(goalID, errors.New(msg))
		projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		return http.StatusOK, map[string]any{"status": "error", "message": msg, "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusFailed), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", msg, projectHistory))}
	}
	cont := *plan.GoalContinuation
	cont.State = s.harness.UserStateSummary(ctx)
	cont.ProjectHistory = s.harness.ProjectHistorySummaryForProject(ctx, cont.GoalID, projectPath)
	if len(cont.ProjectHistory) > 0 {
		cont.State = cloneContext(cont.State)
		cont.State["project_history"] = cont.ProjectHistory
	}
	toolContext := s.agentLoopToolContext(mode, firstNonEmpty(cont.UserText, cont.Summary), cont.Context)
	cont.CatalogSummary = toolContext.CatalogSummary
	cont.AllowedTools = toolContext.AllowedTools
	var res agentloop.Result
	resumeStarted := time.Now()
	if useLegacyPlannerLoop() {
		runner := s.newAgentLoopRunner(cfg, mode)
		res = runner.ResumeAfterConfirmation(ctx, cont)
	} else {
		messageLoop := s.newAgentMessageLoop(cfg, mode)
		res = messageLoop.ResumeAfterConfirmation(ctx, cont)
	}
	if s.logger != nil {
		s.logger.Info("[timing] agent_loop_confirm.resume ms=%d plan=%s goal=%s mode=%s status=%s stop=%s executed=%d",
			time.Since(resumeStarted).Milliseconds(), planID, res.GoalID, mode, res.Status, res.StopReason, len(res.Executed))
	}
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	ensureChatResponseMessageProtocol(&resp)
	status := "ok"
	if strings.TrimSpace(resp.Error) != "" {
		status = "error"
	}
	responsePlanID := AgentLoopConfirmResponsePlanID(planID, resp)
	projectHistory := resp.ProjectHistory
	if strings.TrimSpace(resp.Reply) != "" {
		historyData := chatResponseHistoryData(resp, map[string]any{})
		if len(resp.Artifacts) > 0 {
			historyData["artifacts"] = artifactSummaryRows(resp.Artifacts)
		}
		if len(resp.ProjectResultCards) > 0 {
			historyData["project_result_cards"] = resp.ProjectResultCards
		}
		if updatedHistory := s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, "vit", resp.Reply, resp.GoalID, resp.RunID, historyData); len(updatedHistory) > 0 {
			projectHistory = updatedHistory
		}
	}
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, resp.GoalID, projectPath)
	}
	resp.ProjectHistory = projectHistory
	syncChatResponseMessageIdentityFromHistory(&resp)
	syncAgentPlanProjectHistory(&resp)
	s.attachInteractionRequests(&resp)
	return http.StatusOK, map[string]any{
		"status":                       status,
		"message":                      resp.Reply,
		"reply":                        resp.Reply,
		"plan_id":                      responsePlanID,
		"confirmed_plan_id":            planID,
		"next_plan_id":                 resp.PlanID,
		"goal_id":                      resp.GoalID,
		"run_id":                       resp.RunID,
		"agent_mode":                   mode,
		"goal_status":                  resp.GoalStatus,
		"goal_summary":                 resp.GoalSummary,
		"current_step":                 resp.CurrentStep,
		"completed_steps":              resp.CompletedSteps,
		"stop_reason":                  resp.StopReason,
		"limit_type":                   resp.LimitType,
		"needs_confirmation":           resp.NeedsConfirmation,
		"preview":                      resp.Preview,
		"replies":                      resp.ExecutedKernelReply,
		"executed_kernel_reply":        resp.ExecutedKernelReply,
		"project_result_cards":         resp.ProjectResultCards,
		"artifacts":                    resp.Artifacts,
		"project_history":              projectHistory,
		"agent_plan":                   resp.AgentPlan,
		"interaction_requests":         resp.InteractionRequests,
		"typed_events":                 resp.TypedEvents,
		"acoustic_package_status":      resp.AcousticPackageStatus,
		"acoustic_package_status_path": resp.AcousticPackageStatusPath,
		"error":                        resp.Error,
		"lifecycle":                    resp.Lifecycle,
		"persistence":                  resp.Persistence,
		"message_kind":                 resp.MessageKind,
		"turn_id":                      resp.TurnID,
		"logical_message_id":           resp.LogicalMessageID,
		"supersedes":                   resp.Supersedes,
	}
}

func attachAcousticPackageStatusFromAgentLoopResult(resp *ChatResponse, executed []map[string]any) {
	if resp == nil {
		return
	}
	for i := len(executed) - 1; i >= 0; i-- {
		result := mapValue(executed[i]["result"])
		if len(result) == 0 {
			continue
		}
		status := mapValue(result["acoustic_package_status"])
		if len(status) == 0 {
			continue
		}
		if len(resp.AcousticPackageStatus) == 0 {
			resp.AcousticPackageStatus = status
			resp.AcousticPackageStatusPath = cleanContextText(result["acoustic_package_status_path"])
		}
		for _, event := range mapRowsFromAny(result["typed_events"]) {
			if !chatResponseHasTypedEvent(resp.TypedEvents, event) {
				resp.TypedEvents = append(resp.TypedEvents, event)
			}
		}
		return
	}
}

func chatResponseHasTypedEvent(events []map[string]any, candidate map[string]any) bool {
	if len(candidate) == 0 {
		return true
	}
	candidateType := cleanContextText(firstPresent(candidate, "event_type", "state_kind", "kind"))
	candidateID := cleanContextText(firstPresent(candidate, "state_id", "id"))
	for _, event := range events {
		if len(event) == 0 {
			continue
		}
		eventType := cleanContextText(firstPresent(event, "event_type", "state_kind", "kind"))
		eventID := cleanContextText(firstPresent(event, "state_id", "id"))
		if candidateType != "" && eventType == candidateType && candidateID != "" && eventID == candidateID {
			return true
		}
	}
	return false
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func AgentLoopConfirmResponsePlanID(confirmedPlanID string, resp ChatResponse) string {
	if resp.NeedsConfirmation {
		if nextPlanID := strings.TrimSpace(resp.PlanID); nextPlanID != "" {
			return nextPlanID
		}
	}
	return confirmedPlanID
}

func (s *Server) recordGoalResult(conversationID string, res agentloop.Result) error {
	if s == nil || res.GoalID == "" {
		return nil
	}
	var pendingEvents []AgentEvent
	autoContinuation := false
	s.mu.Lock()
	if s.goalContinuations == nil {
		s.goalContinuations = map[string]agentloop.Continuation{}
	}
	if s.durableContinuations == nil {
		s.durableContinuations = map[string]DurableContinuation{}
	}
	if strings.TrimSpace(conversationID) != "" {
		s.conversationGoals[conversationID] = res.GoalID
		if hasAgentLoopExecutionMemory(res.ExecutionMemory) {
			s.conversationMemory[conversationID] = cloneAgentLoopExecutionMemory(res.ExecutionMemory)
		} else {
			delete(s.conversationMemory, conversationID)
		}
		if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil && strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
			typed := candidate.ToPendingCandidate(conversationID, res.GoalID, res.RunID, "")
			s.upsertPendingCandidate(typed)
			s.pendingMixTicks[conversationID] = *candidate
			if s.logger != nil {
				s.logger.Info("[mix.tick.pending] stored conversation=%s goal=%s run=%s %s observation=%s",
					conversationID, res.GoalID, res.RunID, pendingMixTickLogSummary(*candidate), candidate.ObservationID)
			}
			pendingEvents = append(pendingEvents, AgentEvent{
				Type:     "mix_tick.pending",
				GoalID:   res.GoalID,
				RunID:    res.RunID,
				ItemType: "mix_tick",
				Status:   "pending_confirmation",
				Title:    "混音单步待确认",
				Body:     pendingMixTickEventBody(*candidate),
				Payload:  typedPendingPayload(pendingMixTickEventPayload(*candidate, candidate.ObservationID), typed),
			})
		}
		if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil && strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
			typed := treatment.ToPendingCandidate(conversationID, res.GoalID, res.RunID, "")
			s.upsertPendingCandidate(typed)
			if s.pendingTreatments == nil {
				s.pendingTreatments = map[string]agentloop.MixTreatmentPending{}
			}
			s.pendingTreatments[conversationID] = *treatment
			if s.logger != nil {
				s.logger.Info("[mix.treatment.pending] stored conversation=%s goal=%s run=%s action=%s processor=%s target=%s observation=%s",
					conversationID, res.GoalID, res.RunID, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef, treatment.ObservationID)
			}
			treatmentPayload := mixTreatmentInteractionPayload(*treatment)
			treatmentPayload["conversation_id"] = conversationID
			treatmentPayload = typedPendingPayload(treatmentPayload, typed)
			pendingEvents = append(pendingEvents, AgentEvent{
				Type:     "mix_treatment.pending",
				GoalID:   res.GoalID,
				RunID:    res.RunID,
				ItemType: "mix_treatment",
				Status:   "pending_confirmation",
				Title:    "混音建议待确认",
				Body:     firstNonEmpty(cleanContextText(treatmentPayload["display_card_body"]), localizedMixTreatmentReasoning(*treatment)),
				Payload:  treatmentPayload,
			})
		}
	}
	autoContinuationBudgetExhausted := false
	if res.Continuation != nil {
		// The planner continuation carries the slice-start request context, while
		// recordFreeStateDecision may have imported newer CCB evidence into the
		// server-owned loop during this slice. Export that authoritative loop
		// before creating the durable child checkpoint; otherwise the next slice
		// restarts from the stale overlay and loses cross-slice observations.
		if loop, loopOK := s.freeStateLoops[conversationID]; loopOK {
			res.Continuation.Context = mergeContext(res.Continuation.Context, map[string]any{
				"free_state_reasoning_loop": freeStateLoopMap(loop),
			})
		}
		durable := durableContinuationFromResult(conversationID, res, time.Now().UTC())
		res.Continuation.ContinuationID = durable.ContinuationID
		durable.ProjectPath = s.activeWorkspacePath
		durable.ProjectUUID = s.activeWorkspaceUUID
		durable.ProjectSessionID = s.activeWorkspaceSessionID
		// Persist the canonical semantic contract and snapshot alongside the
		// continuation. This makes a request-boundary or restart recover the
		// same Task interpretation instead of reconstructing it from prompt text.
		s.bindDurableTaskSemanticState(&durable)
		if continuationWaitingForInteraction(res) {
			durable.PendingInteraction = firstNonEmptyMap(pendingInteractionFromResult(res), map[string]any{
				"status":      string(res.Status),
				"stop_reason": res.StopReason,
				"limit_type":  res.LimitType,
			})
		}
		if previous, exists := s.durableContinuations[durable.ContinuationID]; exists {
			// Re-recording the same checkpoint is idempotent. A checkpoint has
			// one lifecycle; duplicate delivery must never reopen completed work.
			durable.CreatedAt = previous.CreatedAt
			durable.Status = previous.Status
			durable.Attempt = previous.Attempt
			durable.LeaseOwner = previous.LeaseOwner
			durable.LeaseExpiresAt = previous.LeaseExpiresAt
			durable.LastError = previous.LastError
		}
		if durable.Status == ContinuationCompleted || durable.Status == ContinuationCancelled || durable.Status == ContinuationFailed {
			delete(s.goalContinuations, res.GoalID)
		} else {
			s.goalContinuations[res.GoalID] = durable.Continuation
		}
		s.durableContinuations[durable.ContinuationID] = cloneDurableContinuation(durable)
		if parentID := strings.TrimSpace(res.ResumedFromID); parentID != "" && parentID != durable.ContinuationID {
			if parent, exists := s.durableContinuations[parentID]; exists && parent.GoalID == durable.GoalID && parent.RunID == durable.RunID &&
				(parent.Status == ContinuationPending || parent.Status == ContinuationClaimed || parent.Status == ContinuationRunning || parent.Status == ContinuationWaitingInteraction) {
				parent.Status = ContinuationCompleted
				parent.LeaseOwner = ""
				parent.LeaseExpiresAt = time.Time{}
				parent.UpdatedAt = durable.UpdatedAt
				s.durableContinuations[parentID] = cloneDurableContinuation(parent)
			}
		}
		if durable.Status == ContinuationPending && strings.TrimSpace(conversationID) != "" {
			if loop, exists := s.freeStateLoops[conversationID]; exists && freeStateLoopActive(loop) {
				if loop.ContinuationBudget <= 0 {
					loop.ContinuationBudget = loop.MaxCycles
					if loop.ContinuationBudget <= 0 {
						loop.ContinuationBudget = freeStateDefaultMaxCycles
					}
				}
				// The mandatory post-Apply observation slice must also survive
				// budget pressure on the scheduler enqueue path. The reserve
				// previously fired only from the interaction respond bridge, so a
				// scheduler-driven post-apply chain could exhaust the budget
				// before the observation slice was ever scheduled (2026-08-28
				// 09:30 smoke: three enqueues 5->7 against budget 6, no reserve,
				// blocked with requires_post_action_observation still true).
				reservePostActionObservationSlice(&loop)
				loop.ContinuationUsed++
				// Allow exactly continuation_budget resumes: the n-th enqueue
				// with used == budget is still legal, the (n+1)-th is not.
				if loop.ContinuationUsed > loop.ContinuationBudget {
					// waiting_continue is an internally bounded state (ADR §10):
					// once the cross-slice budget is spent the loop must stop
					// rescheduling itself and settle with a queryable cause.
					now := time.Now().UTC()
					loop.Status = "blocked"
					loop.LastError = fmt.Sprintf("free-state reasoning exhausted its %d-continuation budget", loop.ContinuationBudget)
					loop.LatestDecision = &agentloop.FreeStateDecision{
						SchemaVersion:  agentloop.FreeStateDecisionSchema,
						Status:         agentloop.FreeStateBlocked,
						EvidenceStatus: "insufficient",
						Summary:        "The free-state reasoning loop exhausted its bounded continuation budget; the evidence gathered so far is preserved but no further automatic continuation was started.",
						StopReason:     freeStateContinuationBudgetExhaustedReason,
						Limitations:    []string{loop.LastError},
					}
					loop.UpdatedAt = now
					s.freeStateLoops[conversationID] = loop
					durable.Status = ContinuationCompleted
					durable.LeaseOwner = ""
					durable.LeaseExpiresAt = time.Time{}
					durable.UpdatedAt = now
					autoContinuationBudgetExhausted = true
					// Includes the just-stored copy of this item: the map write
					// above already captured its pending status.
					for continuationID, other := range s.durableContinuations {
						if other.GoalID != durable.GoalID ||
							(other.Status != ContinuationPending && other.Status != ContinuationClaimed && other.Status != ContinuationRunning) {
							continue
						}
						other.Status = ContinuationCompleted
						other.LeaseOwner = ""
						other.LeaseExpiresAt = time.Time{}
						other.UpdatedAt = now
						s.durableContinuations[continuationID] = cloneDurableContinuation(other)
					}
				} else {
					s.freeStateLoops[conversationID] = loop
				}
			}
		}
		autoContinuation = durable.Status == ContinuationPending
	} else {
		delete(s.goalContinuations, res.GoalID)
		terminalStatus := ContinuationCompleted
		switch res.Status {
		case agentruntime.StatusCancelled, agentruntime.StatusStopped:
			terminalStatus = ContinuationCancelled
		case agentruntime.StatusFailed:
			terminalStatus = ContinuationFailed
		}
		now := time.Now().UTC()
		for continuationID, durable := range s.durableContinuations {
			if durable.GoalID != res.GoalID || durable.Status == ContinuationCompleted || durable.Status == ContinuationCancelled || durable.Status == ContinuationFailed {
				continue
			}
			durable.Status = terminalStatus
			durable.LeaseOwner = ""
			durable.LeaseExpiresAt = time.Time{}
			durable.UpdatedAt = now
			if terminalStatus == ContinuationFailed {
				durable.LastError = firstNonEmpty(res.Error, res.FailureReason)
			}
			s.durableContinuations[continuationID] = cloneDurableContinuation(durable)
		}
	}
	s.mu.Unlock()
	// Persist before waking the worker. This is the ordering that makes a
	// request boundary harmless: a crash after this point leaves recoverable
	// pending work in the project runtime snapshot.
	persistErr := s.persistContinuationState()
	if persistErr != nil {
		s.mu.Lock()
		for continuationID, item := range s.durableContinuations {
			if item.GoalID != res.GoalID || (item.Status != ContinuationPending && item.Status != ContinuationClaimed && item.Status != ContinuationRunning) {
				continue
			}
			item.Status = ContinuationFailed
			item.LeaseOwner = ""
			item.LeaseExpiresAt = time.Time{}
			item.LastError = persistErr.Error()
			item.UpdatedAt = time.Now().UTC()
			s.durableContinuations[continuationID] = cloneDurableContinuation(item)
		}
		delete(s.goalContinuations, res.GoalID)
		s.mu.Unlock()
		if s.harness != nil {
			s.harness.SetGoalStatus(res.GoalID, agentruntime.StatusFailed, persistErr)
		}
		autoContinuation = false
	}
	if autoContinuation {
		s.wakeContinuationScheduler()
	}
	if autoContinuationBudgetExhausted && s.harness != nil {
		// The budget stop must be visible through the runtime interface: the
		// goal settles instead of staying waiting_continue forever.
		s.harness.SetGoalStatus(res.GoalID, agentruntime.StatusCompleted, nil)
	}
	for _, pendingEvent := range pendingEvents {
		s.emitAgentEvent(conversationID, pendingEvent)
	}
	return persistErr
}

func (s *Server) agentLoopExecutionMemoryForConversation(conversationID string) agentloop.ExecutionMemory {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return agentloop.ExecutionMemory{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneAgentLoopExecutionMemory(s.conversationMemory[conversationID])
}

func cloneAgentLoopExecutionMemory(in agentloop.ExecutionMemory) agentloop.ExecutionMemory {
	out := in
	if in.PendingStaticBalancePlan != nil {
		plan := *in.PendingStaticBalancePlan
		plan.Actions = append([]agentloop.PendingStaticBalanceAction(nil), in.PendingStaticBalancePlan.Actions...)
		plan.Assumptions = append([]string(nil), in.PendingStaticBalancePlan.Assumptions...)
		plan.Limitations = append([]string(nil), in.PendingStaticBalancePlan.Limitations...)
		plan.EvidenceRefs = append([]string(nil), in.PendingStaticBalancePlan.EvidenceRefs...)
		plan.FunctionSummary = append([]staticbalance.FunctionSummary(nil), in.PendingStaticBalancePlan.FunctionSummary...)
		out.PendingStaticBalancePlan = &plan
	}
	if in.PendingPanLayoutPlan != nil {
		plan := *in.PendingPanLayoutPlan
		plan.Actions = append([]agentloop.PendingPanLayoutAction(nil), in.PendingPanLayoutPlan.Actions...)
		plan.Assumptions = append([]string(nil), in.PendingPanLayoutPlan.Assumptions...)
		plan.Limitations = append([]string(nil), in.PendingPanLayoutPlan.Limitations...)
		plan.EvidenceRefs = append([]string(nil), in.PendingPanLayoutPlan.EvidenceRefs...)
		plan.GroupSummary = append([]panlayout.GroupSummary(nil), in.PendingPanLayoutPlan.GroupSummary...)
		out.PendingPanLayoutPlan = &plan
	}
	out.PendingTrackOrganization = cloneContext(in.PendingTrackOrganization)
	if len(in.Bindings) > 0 {
		out.Bindings = append([]agentloop.ExecutionBinding(nil), in.Bindings...)
	}
	return out
}

func hasAgentLoopExecutionMemory(memory agentloop.ExecutionMemory) bool {
	return strings.TrimSpace(memory.LastCreatedTrackID) != "" ||
		strings.TrimSpace(memory.LastCreatedTrackName) != "" ||
		strings.TrimSpace(memory.LastCreatedFolderTrackID) != "" ||
		strings.TrimSpace(memory.LastCreatedFolderTrackName) != "" ||
		strings.TrimSpace(memory.LastCreatedClipID) != "" ||
		strings.TrimSpace(memory.LastCreatedClipName) != "" ||
		strings.TrimSpace(memory.LastLoadedPluginID) != "" ||
		strings.TrimSpace(memory.LastLoadedPluginName) != "" ||
		strings.TrimSpace(memory.LastMixTickID) != "" ||
		strings.TrimSpace(memory.ActiveWorkTargetTrackID) != "" ||
		strings.TrimSpace(memory.ActiveWorkTargetFolderTrackID) != "" ||
		strings.TrimSpace(memory.ActiveWorkTargetClipID) != "" ||
		strings.TrimSpace(memory.ActiveWorkTargetPluginID) != "" ||
		memory.PendingMixTickCandidate != nil ||
		memory.PendingStaticBalancePlan != nil ||
		memory.PendingPanLayoutPlan != nil ||
		memory.PendingMixTreatment != nil ||
		len(memory.PendingTrackOrganization) > 0 ||
		strings.TrimSpace(memory.MixDiagnosisContextID) != "" ||
		len(memory.MixDiagnosisContext) > 0 ||
		len(memory.Bindings) > 0
}

func (s *Server) clearGoalContinuation(goalID string) {
	if s == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	s.mu.Lock()
	goalID = strings.TrimSpace(goalID)
	delete(s.goalContinuations, goalID)
	now := time.Now().UTC()
	for continuationID, item := range s.durableContinuations {
		if item.GoalID != goalID || item.Status == ContinuationCompleted || item.Status == ContinuationCancelled || item.Status == ContinuationFailed {
			continue
		}
		item.Status = ContinuationCancelled
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = now
		s.durableContinuations[continuationID] = cloneDurableContinuation(item)
	}
	s.mu.Unlock()
	s.persistCurrentProjectWorkspace()
}

func (s *Server) goalContinuationForConversation(conversationID string) (agentloop.Continuation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	goalID := strings.TrimSpace(s.conversationGoals[conversationID])
	if goalID != "" {
		if cont, ok := s.goalContinuations[goalID]; ok {
			return cont, true
		}
	}
	var newest DurableContinuation
	found := false
	for _, item := range s.durableContinuations {
		if strings.TrimSpace(item.ConversationID) != strings.TrimSpace(conversationID) {
			continue
		}
		if continuationRecoveryValidationRequired(item) {
			continue
		}
		if item.Status != ContinuationPending && item.Status != ContinuationWaitingInteraction && item.Status != ContinuationClaimed && item.Status != ContinuationRunning {
			continue
		}
		if !found || item.UpdatedAt.After(newest.UpdatedAt) {
			newest, found = item, true
		}
	}
	if found {
		return cloneDurableContinuation(newest).Continuation, true
	}
	return agentloop.Continuation{}, false
}

func (s *Server) automaticContinuationForConversation(conversationID string) (DurableContinuation, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return DurableContinuation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest DurableContinuation
	found := false
	for _, item := range s.durableContinuations {
		if item.ConversationID != conversationID ||
			(item.Status != ContinuationPending && item.Status != ContinuationClaimed && item.Status != ContinuationRunning) {
			continue
		}
		if !found || item.UpdatedAt.After(newest.UpdatedAt) {
			newest, found = item, true
		}
	}
	if !found {
		return DurableContinuation{}, false
	}
	return cloneDurableContinuation(newest), true
}

func (s *Server) interactionContinuationForConversation(conversationID string) (DurableContinuation, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return DurableContinuation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest DurableContinuation
	found := false
	for _, item := range s.durableContinuations {
		if item.ConversationID != conversationID || item.Status != ContinuationWaitingInteraction {
			continue
		}
		if !found || item.UpdatedAt.After(newest.UpdatedAt) {
			newest, found = item, true
		}
	}
	if !found {
		return DurableContinuation{}, false
	}
	return cloneDurableContinuation(newest), true
}

func (s *Server) goalContinuationForCurrentGoal(chatContext map[string]any) (agentloop.Continuation, bool) {
	goalID, _ := goalIDsFromContext(chatContext)
	goalID = strings.TrimSpace(goalID)
	if s == nil || goalID == "" {
		return agentloop.Continuation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cont, ok := s.goalContinuations[goalID]; ok {
		return cont, true
	}
	var newest DurableContinuation
	found := false
	for _, item := range s.durableContinuations {
		if item.GoalID != goalID {
			continue
		}
		if continuationRecoveryValidationRequired(item) {
			continue
		}
		if item.Status != ContinuationPending && item.Status != ContinuationWaitingInteraction && item.Status != ContinuationClaimed && item.Status != ContinuationRunning {
			continue
		}
		if !found || item.UpdatedAt.After(newest.UpdatedAt) {
			newest, found = item, true
		}
	}
	if found {
		return cloneDurableContinuation(newest).Continuation, true
	}
	return agentloop.Continuation{}, false
}

func (s *Server) resumeContinuationForChat(conversationID string, chatContext map[string]any) (agentloop.Continuation, bool) {
	if continuationID := firstStringFromMap(chatContext, "durable_continuation_id"); continuationID != "" {
		s.mu.Lock()
		item, ok := s.durableContinuations[continuationID]
		s.mu.Unlock()
		if ok && item.ConversationID == conversationID &&
			(item.Status == ContinuationClaimed || item.Status == ContinuationRunning) {
			return cloneDurableContinuation(item).Continuation, true
		}
		return agentloop.Continuation{}, false
	}
	if cont, ok := s.goalContinuationForCurrentGoal(chatContext); ok {
		return cont, true
	}
	if cont, ok := s.goalContinuationForConversation(conversationID); ok {
		return cont, true
	}
	return agentloop.Continuation{}, false
}

func agentLoopContinuationUserText(cont agentloop.Continuation, current string) string {
	current = strings.TrimSpace(current)
	original := strings.TrimSpace(firstNonEmpty(cont.Summary, cont.UserText))
	if current == "" || original == "" || strings.EqualFold(current, original) {
		return firstNonEmpty(current, original)
	}
	return original + "\n\nUser clarification: " + current
}

func (s *Server) conversationGoalID(conversationID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(s.conversationGoals[conversationID])
}

func shouldResumeGoalFromStatus(status agentruntime.GoalStatus) bool {
	switch status {
	case agentruntime.StatusWaitingConfirmation, agentruntime.StatusWaitingClarification, agentruntime.StatusWaitingContinue:
		return true
	default:
		return false
	}
}

func toolNamesFromHarness(h *harness.Harness) []string {
	return toolNamesForAgentLoop(h, agentModeGoal)
}

type agentLoopToolContext struct {
	CatalogSummary string
	AllowedTools   []string
}

func (s *Server) agentLoopToolContext(mode, userText string, requestContext map[string]any) agentLoopToolContext {
	if s == nil || s.harness == nil {
		return agentLoopToolContext{AllowedTools: toolNamesForAgentLoop(nil, mode)}
	}
	if freeStateLoopActiveContext(requestContext) {
		// The model-owned reasoning loop chooses observation views and family;
		// typed plug-in tools are disclosed only by the governed post-family
		// materializer after an exact family/instance binding exists.
		observationTools := []string{"ccb.observation_catalog", "ccb.observation_request"}
		return agentLoopToolContext{
			CatalogSummary: s.harness.ModelCatalogSummaryForTools(observationTools),
			AllowedTools:   observationTools,
		}
	}
	capabilities := agentLoopCapabilityNames(userText, requestContext)
	if len(capabilities) == 0 {
		return agentLoopToolContext{
			CatalogSummary: s.harness.ModelCatalogSummary(),
			AllowedTools:   toolNamesForAgentLoop(s.harness, mode),
		}
	}
	allowed := toolNamesForAgentLoopCapabilities(s.harness, mode, capabilities)
	if len(allowed) == 0 {
		allowed = toolNamesForAgentLoop(s.harness, mode)
	}
	summary := s.harness.ModelCatalogSummaryForCapabilityPacks(capabilities, allowed)
	if strings.TrimSpace(summary) == "" {
		summary = s.harness.ModelCatalogSummary()
	}
	return agentLoopToolContext{CatalogSummary: summary, AllowedTools: allowed}
}

func toolNamesForAgentLoop(h *harness.Harness, mode string) []string {
	seen := map[string]bool{}
	if agentModeFromString(mode) != agentModePlan {
		seen["daw.invoke"] = true
	}
	if h == nil {
		return sortedToolNameKeys(seen)
	}
	for _, tool := range h.Tools() {
		name := strings.TrimSpace(tool.Name)
		// plugin.set_parameter (set_plugin_param) is the Tier 2 direct-control
		// Write path: after a live parameter surface is observed, the model
		// reads all_parameters + display_domain_candidate from explain_controls
		// and writes a normalized value directly. Excluding it here left the
		// model with prompt instructions for a tool it could not call.
		if name == "" || name == "plugin.instantiate" {
			continue
		}
		if agentModeFromString(mode) == agentModePlan && !isPlanReadOnlyTool(tool) {
			continue
		}
		seen[name] = true
	}
	return sortedToolNameKeys(seen)
}

func toolNamesForAgentLoopCapabilities(h *harness.Harness, mode string, capabilities []string) []string {
	full := toolNamesForAgentLoop(h, mode)
	if len(capabilities) == 0 {
		return full
	}
	available := map[string]bool{}
	for _, name := range full {
		available[name] = true
	}
	seen := map[string]bool{}
	allowRawInvoke := len(capabilities) == 1 && capabilities[0] == "workspace"
	if agentModeFromString(mode) != agentModePlan && available["daw.invoke"] && allowRawInvoke {
		seen["daw.invoke"] = true
	}
	projectAudioRequested := false
	for _, capability := range capabilities {
		if capability == "project_audio" {
			projectAudioRequested = true
			break
		}
	}
	add := func(names ...string) {
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" && available[name] {
				seen[name] = true
			}
		}
	}
	add(agentLoopBaseTools()...)
	for _, capability := range capabilities {
		switch capability {
		case "static_mix_pan_layout":
			add(agentLoopStaticMixPanLayoutTools()...)
		case "static_mix_static_balance":
			add(agentLoopStaticMixStaticBalanceTools()...)
		case "static_mix_gain_staging":
			add(agentLoopStaticMixGainStagingTools()...)
		case "track":
			add(agentLoopTrackTools()...)
		case "midi":
			add(agentLoopTrackTools()...)
			add(agentLoopMidiTools()...)
		case "clip":
			if !projectAudioRequested {
				add(agentLoopTrackTools()...)
				add(agentLoopClipTools()...)
			}
		case "plugin":
			add(agentLoopTrackTools()...)
			add(agentLoopPluginTools()...)
		case "mix":
			add(agentLoopMixTools()...)
		case "transport":
			add(agentLoopTransportTools()...)
		case "version":
			add(agentLoopVersionTools()...)
		case "artifact":
			add(agentLoopArtifactTools()...)
			add(agentLoopClipTools()...)
		case "media":
			add(agentLoopMediaTools()...)
			if !projectAudioRequested {
				add(agentLoopClipTools()...)
			}
		case "project_audio":
			add(agentLoopProjectAudioTools()...)
		case "project_marker":
			add(agentLoopProjectMarkerTools()...)
		case "workspace":
			add(agentLoopWorkspaceTools()...)
		}
	}
	if len(seen) == 0 || (len(seen) == 1 && seen["daw.invoke"]) {
		return full
	}
	return sortedToolNameKeys(seen)
}

func agentLoopCapabilityNames(userText string, requestContext map[string]any) []string {
	text := strings.ToLower(strings.TrimSpace(userText))
	projectAudioImportRequested := agentLoopProjectAudioImportIntent(text, requestContext)
	trackOrganizationRequested := !projectAudioImportRequested && agentLoopTrackOrganizationIntent(text)
	route := agentLoopClassifyCapabilityRoute(userText, requestContext)
	seen := map[string]bool{}
	add := func(name string) {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	// Once the open semantic processor loop is active, CCB observation is part
	// of the runtime protocol regardless of which lexical capability pack the
	// original request happens to match (for example, Chinese "drums" can also
	// look like a MIDI request).
	if freeStateLoopActiveContext(requestContext) {
		add("mix")
	}
	if route.clipFadeGain {
		add("clip")
	}
	panLayoutRequested := agentLoopStaticMixPanLayoutIntent(text)
	staticBalanceRequested := agentLoopStaticMixStaticBalanceIntent(text)
	if panLayoutRequested {
		add("static_mix_pan_layout")
	} else if staticBalanceRequested {
		add("static_mix_static_balance")
	} else if agentLoopStaticMixGainStagingIntent(text) {
		add("static_mix_gain_staging")
	}
	if !projectAudioImportRequested && agentLoopTextHasAny(text,
		"\u8f68\u9053", "\u97f3\u8f68", "\u9759\u97f3", "\u72ec\u594f", "\u97f3\u91cf", "\u5f55\u97f3", "\u51bb\u7ed3",
		"track", "mute", "solo", "volume", "arm", "freeze",
	) {
		add("track")
	}
	if trackOrganizationRequested {
		add("track")
	}
	if agentLoopTextHasAny(text,
		"midi", "\u97f3\u7b26", "\u65cb\u5f8b", "\u548c\u5f26", "\u9f13", "\u91cf\u5316", "\u8f6c\u8c03", "\u529b\u5ea6",
		"note", "notes", "melody", "chord", "drum", "quantize", "transpose", "velocity",
	) {
		add("midi")
	}
	if !projectAudioImportRequested && agentLoopTextHasAny(text,
		"clip", "\u7247\u6bb5", "\u97f3\u9891", "\u5bfc\u5165", "\u7d20\u6750", "\u6587\u4ef6", "\u5207\u5206", "\u88c1\u526a", "\u590d\u5236\u7247\u6bb5",
		"\u7247\u6bb5\u6e05\u7406", "\u6e05\u7406\u7a7a\u767d", "\u7a7a\u767d", "\u9759\u97f3\u6e05\u7406",
		"audio", "media", "import", "split", "duplicate", "strip silence", "strip_silence", "silence", "silent",
	) {
		add("clip")
	}
	knownPluginNames := toolpolicy.CollectPluginNames(requestContext)
	if toolpolicy.MentionsPlugin(userText, knownPluginNames) || toolpolicy.ExplicitPluginRequest(userText, knownPluginNames) {
		add("plugin")
	}
	if route.allowsNaturalMixKeywords() && agentLoopTextHasAny(text,
		"\u6df7\u97f3", "\u7f29\u6df7", "\u4e3b\u5531", "\u4eba\u58f0", "\u58f0\u97f3", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u54cd\u5ea6", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u7535\u5e73", "\u589e\u76ca", "\u52a8\u6001", "\u7a7a\u95f4\u611f", "\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u9ad8\u9891", "\u523a\u8033", "\u6d51\u6d4a", "\u9760\u524d", "\u9760\u540e", "\u5de6", "\u53f3", "\u5c45\u4e2d", "\u56de\u4e2d", "\u66f4\u4eae", "\u66f4\u6697", "\u66f4\u7a33", "\u66f4\u7d27",
		"mix", "mixing", "master", "vocal", "loudness", "too loud", "too quiet", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "presence", "mud", "muddy", "harsh", "bright", "dark", "forward", "back", "space", "depth", "dynamic", "pan", "panning", "stereo", "left", "right", "center", "centre",
	) {
		add("mix")
	}
	if route.allowsNaturalMixKeywords() && agentLoopTextHasAny(text,
		"\u58f0\u5b66", "\u58f0\u5b66\u6570\u636e", "\u58f0\u5b66\u89c2\u5bdf", "\u89c2\u5bdf\u5668", "\u9891\u8c31", "\u8c31\u56fe", "\u97f3\u9891\u7279\u5f81", "\u9891\u6bb5", "\u9891\u7387", "\u6ce2\u5f62", "\u80fd\u91cf\u5206\u5e03", "\u7acb\u4f53\u58f0\u76f8\u5173", "\u58f0\u50cf\u76f8\u5173",
		"mix.observe", "mix.read", "mix.derive", "acoustic", "observation", "observe", "observer", "spectrum", "spectral", "spectrogram", "frequency", "frequencies", "band energy", "band_energy", "stereo relation", "stereo_relation",
	) {
		add("mix")
	}
	if projectAudioImportRequested || agentLoopTextHasAny(text,
		"\u5de5\u7a0b\u97f3\u9891", "\u5de5\u7a0b\u89c4\u683c", "\u91c7\u6837\u7387", "\u4f4d\u6df1", "\u5f55\u97f3\u683c\u5f0f", "\u5f55\u97f3\u6587\u4ef6\u7c7b\u578b", "\u97f3\u9891\u89c4\u683c", "\u5bfc\u5165\u9884\u68c0", "\u9884\u68c0", "\u5bfc\u5165\u8ba1\u5212",
		"project audio", "audio settings", "sample rate", "sample_rate", "bit depth", "bit_depth", "record format", "record_file_type", "import preflight", "preflight", "stems folder", "stems",
	) {
		add("project_audio")
	}
	if agentLoopTextHasAny(text,
		"marker", "markers", "section marker", "section markers", "section map", "timeline marker",
		"\u6807\u8bb0", "\u6bb5\u843d", "\u6bb5\u843d\u6807\u8bb0", "\u6bb5\u843d\u5730\u56fe", "\u5de5\u7a0b\u6807\u8bb0", "\u65f6\u95f4\u7ebf\u6807\u8bb0",
	) {
		add("project_marker")
	}
	if agentLoopTextHasAny(text,
		"\u64ad\u653e", "\u505c\u6b62", "\u6682\u505c", "\u5b9a\u4f4d", "\u8282\u62cd\u5668", "\u5f55\u97f3",
		"play", "stop", "seek", "tempo", "bpm", "click", "record",
	) {
		add("transport")
	}
	if agentLoopTextHasAny(text,
		"\u5386\u53f2", "\u7248\u672c", "\u56de\u6eda", "\u5206\u652f", "\u5de5\u4f5c\u6811",
		"checkpoint", "history", "branch", "worktree", "restore", "checkout",
	) {
		add("version")
	}
	if agentLoopTextHasAny(text,
		"\u9644\u4ef6", "\u8d44\u6599", "\u5a92\u4f53\u6c60", "\u7f51\u9875", "\u6d4f\u89c8\u5668", "\u641c\u7d22",
		"artifact", "browser", "url", "http://", "https://",
	) {
		add("artifact")
	}
	if !projectAudioImportRequested && !trackOrganizationRequested && agentLoopTextHasAny(text,
		"\u7d20\u6750", "\u5a92\u4f53\u6c60", "\u97f3\u9891", "\u89c6\u9891", "\u56fe\u7247", "\u6587\u4ef6\u5939", "\u8def\u5f84", "\u97f3\u89c6\u9891",
		"media", "asset", "assets", "folder", "path", ".wav", ".mp3", ".flac", ".mp4", ".mov", ".mid", ".midi",
	) {
		add("media")
	}
	if agentLoopTextHasAny(text,
		"\u65e5\u5fd7", "\u6e90\u7801", "\u6587\u4ef6\u5185\u5bb9", "\u641c\u7d22\u4ee3\u7801",
		"workspace", "grep", "log",
	) {
		add("workspace")
	}
	if agentLoopTextHasAny(text, "轨道", "track", "mute", "solo", "音量", "volume", "arm", "freeze", "冻结") {
		add("track")
	}
	if agentLoopTextHasAny(text, "midi", "音符", "旋律", "和弦", "鼓", "note", "notes", "melody", "chord", "drum", "quantize", "transpose", "velocity") {
		add("midi")
	}
	if agentLoopTextHasAny(text, "clip", "片段", "音频", "audio", "导入", "素材", "文件", "media", "import", "split", "裁剪", "复制片段", "duplicate", "片段清理", "清理空白", "空白", "静音清理", "strip silence", "strip_silence", "silence", "silent") {
		add("clip")
	}
	if agentLoopTextHasAny(text, "插件", "效果器", "plugin", "vst", "eq", "均衡", "compressor", "压缩", "reverb", "混响", "delay", "延迟", "grabber", "抓手", "参数", "param") {
		add("plugin")
	}
	if agentLoopTextHasAny(text, "播放", "停止", "暂停", "定位", "节拍器", "录音", "play", "stop", "seek", "tempo", "bpm", "click", "record") {
		add("transport")
	}
	if agentLoopTextHasAny(text, "历史", "版本", "回滚", "分支", "工作树", "checkpoint", "history", "branch", "worktree", "restore", "checkout") {
		add("version")
	}
	if agentLoopTextHasAny(text, "附件", "资料", "媒体池", "网页", "浏览器", "搜索", "artifact", "browser", "url", "http://", "https://") {
		add("artifact")
	}
	if agentLoopTextHasAny(text, "日志", "源码", "文件内容", "搜索代码", "workspace", "grep", "log") {
		add("workspace")
	}
	if contextHasAnyValue(requestContext, "attachments", "attachment", "artifacts", "artifact", "selected_attachment_file_path", "attachment_import_file_path") {
		add("artifact")
	}
	if route.allowsNaturalMixKeywords() && contextHasAnyValue(requestContext, "selected_track_id", "selected_clip_id", "selected_clip_ids", "selected_clip_track_id") && agentLoopTextHasAny(text,
		"\u8c03", "\u6df7", "\u9760\u524d", "\u9760\u540e", "\u54cd\u4e00\u70b9", "\u5c0f\u4e00\u70b9", "\u5927\u4e00\u70b9", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca", "\u7a7a\u95f4", "\u4f4e\u9891", "\u9ad8\u9891", "\u4eba\u58f0", "\u4e3b\u5531",
		"mix", "forward", "back", "louder", "quieter", "too loud", "too quiet", "volume", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "space", "presence", "mud", "harsh", "vocal",
	) {
		add("mix")
	}
	if contextHasAnyValue(requestContext, "selected_plugin_id", "selected_plugin_name") && agentLoopTextHasAny(text,
		"调", "调整", "大一点", "小一点", "亮", "暗", "浑浊", "刺耳", "高频", "低频", "中频", "低中频", "空气感", "清晰", "均衡",
		"阈值", "压缩比", "启动", "释放", "拐点", "补偿增益", "干湿比",
		"mud", "muddy", "harsh", "presence", "boost", "cut", "bright", "dark", "high", "treble", "air", "clear", "clarity", "eq",
		"threshold", "ratio", "attack", "release", "knee", "makeup", "mix", "input", "output",
	) {
		add("plugin")
	}
	if seen["static_mix_pan_layout"] {
		delete(seen, "static_mix_static_balance")
		delete(seen, "static_mix_gain_staging")
		delete(seen, "track")
		delete(seen, "mix")
	} else if seen["static_mix_static_balance"] {
		delete(seen, "static_mix_gain_staging")
		delete(seen, "track")
		delete(seen, "mix")
	} else if seen["static_mix_gain_staging"] {
		delete(seen, "track")
		delete(seen, "mix")
	}
	if seen["mix"] {
		delete(seen, "track")
	}
	if projectAudioImportRequested {
		delete(seen, "track")
		delete(seen, "clip")
		delete(seen, "media")
		delete(seen, "artifact")
	}
	return sortedToolNameKeys(seen)
}

type agentLoopCapabilityRoute struct {
	clipFadeGain bool
}

func agentLoopClassifyCapabilityRoute(userText string, _ map[string]any) agentLoopCapabilityRoute {
	return agentLoopCapabilityRoute{
		clipFadeGain: chatClipFadeGainRequest(userText),
	}
}

func (route agentLoopCapabilityRoute) allowsNaturalMixKeywords() bool {
	return !route.clipFadeGain
}

func agentLoopStaticMixGainStagingIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	if agentLoopTextHasAny(text, "b1", "b 1", "b1.1", "b 1.1", "b1-1", "b1_1", "b1.2", "b 1.2", "b1-2", "b 1-2", "b1_2") {
		return true
	}
	hasB1 := agentLoopTextHasAny(text, "b1", "b 1", "static_mix.gain_staging", "gain staging", "gain-staging", "gainstage")
	hasGainHealth := agentLoopTextHasAny(text,
		"gain staging", "gain structure", "gain health", "headroom", "level health", "level check", "level scan", "level audit", "input level", "source level", "peak check", "rms check", "loudness check",
		"\u589e\u76ca\u7ed3\u6784", "\u589e\u76ca\u6574\u7406", "\u589e\u76ca\u9636\u6bb5", "\u7535\u5e73\u7ed3\u6784", "\u7535\u5e73\u5065\u5eb7", "\u7535\u5e73\u68c0\u67e5", "\u7535\u5e73\u626b\u63cf", "\u7535\u5e73\u5ba1\u8ba1", "\u8f93\u5165\u7535\u5e73", "\u6e90\u7535\u5e73", "\u97f3\u91cf\u68c0\u67e5", "\u97f3\u91cf\u626b\u63cf", "\u97f3\u91cf\u5ba1\u8ba1", "\u54cd\u5ea6\u68c0\u67e5", "\u5cf0\u503c\u68c0\u67e5", "\u4f59\u91cf",
	)
	hasProjectLevelCheck := agentLoopTextHasAny(text, "full project", "whole project", "entire project", "all tracks", "\u6574\u4e2a\u5de5\u7a0b", "\u5168\u5de5\u7a0b", "\u6574\u4f53", "\u5168\u5c40", "\u6240\u6709\u8f68\u9053", "\u5168\u90e8\u8f68\u9053") &&
		agentLoopTextHasAny(text, "gain", "level", "volume", "loudness", "peak", "rms", "lufs", "headroom", "\u589e\u76ca", "\u7535\u5e73", "\u97f3\u91cf", "\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u4f59\u91cf") &&
		agentLoopTextHasAny(text, "check", "scan", "audit", "inspect", "analyze", "analyse", "\u68c0\u67e5", "\u626b\u63cf", "\u5ba1\u8ba1", "\u67e5\u770b", "\u5206\u6790")
	if hasB1 && agentLoopTextHasAny(text, "gain", "level", "volume", "loudness", "peak", "rms", "lufs", "headroom", "\u589e\u76ca", "\u7535\u5e73", "\u97f3\u91cf", "\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u4f59\u91cf") {
		return true
	}
	return hasGainHealth || hasProjectLevelCheck
}

func agentLoopStaticMixStaticBalanceIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	if agentLoopTextHasAny(text, "b2", "b 2", "static_mix.static_balance", "static balance", "静态平衡", "靜態平衡") {
		return true
	}
	projectWide := agentLoopTextHasAny(text, "全工程", "整个工程", "整個工程", "整体", "整體", "多轨", "多軌", "all tracks", "whole project", "full project", "multitrack")
	levelBalance := agentLoopTextHasAny(text, "音量平衡", "电平平衡", "電平平衡", "推子平衡", "volume balance", "level balance", "fader balance")
	return projectWide && levelBalance
}

func agentLoopStaticMixPanLayoutIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	if agentLoopTextHasAny(text,
		"b3", "b 3", "static_mix.pan_layout", "pan layout", "static pan layout",
		"声像布局", "聲像佈局", "声场布局", "聲場佈局", "静态声像", "靜態聲像",
	) {
		return true
	}
	projectWide := agentLoopTextHasAny(text,
		"全工程", "整个工程", "整個工程", "整体", "整體", "多轨", "多軌",
		"all tracks", "whole project", "full project", "multitrack",
	)
	panLayout := agentLoopTextHasAny(text,
		"声像布局", "聲像佈局", "声场布局", "聲場佈局", "声像平衡", "聲像平衡",
		"pan layout", "panning layout", "stereo placement", "stereo layout",
	)
	return projectWide && panLayout
}

func agentLoopProjectAudioImportIntent(text string, requestContext map[string]any) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasImport := agentLoopTextHasAny(text,
		"import", "importing", "add to project", "bring into project", "into the project", "create tracks",
		"\u5bfc\u5165", "\u532f\u5165", "\u8f7d\u5165", "\u52a0\u5165\u5de5\u7a0b", "\u52a0\u5230\u5de5\u7a0b", "\u5bfc\u5230\u5de5\u7a0b", "\u653e\u8fdb\u5de5\u7a0b",
	)
	if !hasImport {
		return false
	}
	if agentLoopHasSingleAudioImportObject(text, requestContext) {
		return false
	}
	hasFolderOrStems := agentLoopFolderOrStemsImportHint(text)
	hasProjectTarget := agentLoopTextHasAny(text,
		"project", "create tracks", "add to project", "bring into project",
		"\u5de5\u7a0b", "\u521b\u5efa\u8f68", "\u521b\u5efa\u8f68\u9053", "\u5efa\u8f68",
	)
	if hasFolderOrStems {
		return true
	}
	return hasProjectTarget && agentLoopHasFolderImportObject(text, requestContext)
}

func agentLoopTrackOrganizationIntent(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	hasGroupingAction := agentLoopTextHasAny(text,
		"\u6574\u7406", "\u5206\u7ec4", "\u805a\u7c7b", "\u5efa\u8bae\u5206\u7ec4", "\u6309\u5efa\u8bae",
		"organize", "organisation", "organization", "group", "grouping", "cluster", "proposal", "tom",
	)
	hasFolderTarget := agentLoopTextHasAny(text,
		"\u6587\u4ef6\u5939", "\u6587\u4ef6\u5939\u8f68", "\u8f68\u9053\u6587\u4ef6\u5939", "\u8def\u7531\u6587\u4ef6\u5939", "folder", "folder track", "track folder",
	)
	hasTrackTreeAction := agentLoopTextHasAny(text,
		"\u8f68\u9053\u6587\u4ef6\u5939", "\u6587\u4ef6\u5939\u8f68", "\u8def\u7531\u6587\u4ef6\u5939",
		"folder track", "track folder",
	)
	return hasTrackTreeAction || (hasGroupingAction && hasFolderTarget)
}

func agentLoopFolderOrStemsImportHint(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	return agentLoopTextHasAny(text,
		"stem", "stems", "stem folder", "multi-track", "multitrack", "tracks out", "folder", "directory",
		"\u5206\u8f68", "\u591a\u8f68", "\u6587\u4ef6\u5939", "\u76ee\u5f55", "\u521b\u5efa\u8f68", "\u521b\u5efa\u8f68\u9053", "\u5efa\u8f68",
	)
}

func agentLoopHasSingleAudioImportObject(text string, requestContext map[string]any) bool {
	if agentLoopTextHasAudioFileExtension(text) {
		return true
	}
	for _, row := range agentLoopContextRows(requestContext) {
		if agentLoopContextKindIsAudio(row, "selected_attachment_kind", "attachment_import_kind", "selected_library_kind", "selected_asset_kind", "media_kind", "kind") {
			return true
		}
		if agentLoopContextPathLooksAudioFile(row,
			"attachment_import_file_path", "selected_attachment_file_path", "selected_library_file_path",
			"file_path", "selected_file_path", "asset_path", "path",
		) {
			return true
		}
	}
	return false
}

func agentLoopHasFolderImportObject(text string, requestContext map[string]any) bool {
	for _, row := range agentLoopContextRows(requestContext) {
		if path := agentLoopFirstContextText(row,
			"folder_path", "folder", "directory", "asset_folder", "source_folder_path",
			"selected_folder_path", "selected_directory_path", "selected_library_folder_path", "selected_asset_folder_path",
		); path != "" && !agentLoopPathLooksAudioFile(path) {
			return true
		}
		if path := agentLoopFirstContextText(row, "asset_location", "source_root"); path != "" {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				return true
			}
		}
	}
	for _, path := range existingLocalPathsFromText(text) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func agentLoopContextRows(ctx map[string]any) []map[string]any {
	if ctx == nil {
		return nil
	}
	rows := []map[string]any{ctx}
	if current := agentLoopContextMapValue(ctx["current_selection"]); current != nil {
		rows = append(rows, current)
	}
	return rows
}

func agentLoopContextMapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func agentLoopContextKindIsAudio(row map[string]any, keys ...string) bool {
	for _, key := range keys {
		if strings.EqualFold(agentLoopFirstContextText(row, key), "audio") {
			return true
		}
	}
	return false
}

func agentLoopContextPathLooksAudioFile(row map[string]any, keys ...string) bool {
	for _, key := range keys {
		if agentLoopPathLooksAudioFile(agentLoopFirstContextText(row, key)) {
			return true
		}
	}
	return false
}

func agentLoopFirstContextText(row map[string]any, keys ...string) string {
	if row == nil {
		return ""
	}
	for _, key := range keys {
		if value := cleanContextText(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func agentLoopTextHasAudioFileExtension(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	for _, ext := range []string{".wav", ".mp3", ".flac", ".aif", ".aiff", ".ogg", ".oga", ".m4a", ".wma"} {
		if strings.Contains(text, ext) {
			return true
		}
	}
	return false
}

func agentLoopPathLooksAudioFile(path string) bool {
	return resolveAttachmentKind(strings.TrimSpace(path)) == "audio"
}

func agentLoopTextHasAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func contextHasAnyValue(ctx map[string]any, keys ...string) bool {
	if ctx == nil {
		return false
	}
	for _, key := range keys {
		if cleanContextText(ctx[key]) != "" {
			return true
		}
	}
	return false
}

func agentLoopBaseTools() []string {
	return []string{
		"project.state", "project.undo", "project.redo",
		"track.list",
		"goal.status",
	}
}

func agentLoopTrackTools() []string {
	return []string{
		"track.add", "track.add_audio", "track.rename", "track.delete",
		"track.folder.create", "track.move_to_folder", "track.folder.set_routing_bus_enabled",
		"project.apply_track_organization",
		"track.group.list", "track.group.create", "track.group.update", "track.group.set_members", "track.group.delete", "track.group.apply_control",
		"track.mute", "track.solo", "track.arm", "track.volume", "track.pan",
		"track.freeze", "track.unfreeze",
	}
}

func agentLoopMidiTools() []string {
	return []string{
		"midi.create_clip", "midi.insert_clip", "midi.import_file",
		"midi.apply_note_patch", "midi.write_clip_notes", "midi.read_notes", "midi.read_clip_notes", "midi.read_clip_data",
		"midi.insert_notes", "midi.delete_notes", "midi.move_notes", "midi.resize_notes", "midi.quantize", "midi.transpose", "midi.set_velocity", "midi.replace_region",
		"clip.select",
	}
}

func agentLoopClipTools() []string {
	return []string{
		"clip.add_audio", "clip.import_audio", "clip.import_media_to_track",
		"clip.move", "clip.resize", "clip.split", "clip.clone", "clip.remove", "clip.select", "clip.warm_waveform_bake",
		"clip.fade.set", "clip.fade.read", "clip.gain.set", "clip.gain.set_batch", "clip.gain.read",
		"clip.strip_silence.analyze", "clip.strip_silence.suggest", "clip.strip_silence.apply", "clip.strip_silence.apply_batch",
		"artifact.list", "artifact.read", "artifact.extract",
	}
}

func agentLoopPluginTools() []string {
	return []string{
		"plugin.list_available", "plugin.search", "plugin.semantic_search", "plugin.semantic_get", "plugin.semantic_build_index", "plugin.scan",
		"plugin.load_to_rack", "rack.add_node",
		"plugin.get_parameters", "plugin.open", "plugin.show_editor",
		"plugin.set_parameter", // Tier 2 direct-control path for an observed normalized value
		"plugin_grabber.explain_controls",
		"plugin_grabber.inspect_gate_expander", // identity-free gate/expander stage inspection
		"plugin_grabber.apply_gate_expander_controls",
		"plugin_grabber.inspect_transient_shaper", // identity-free signed envelope stage inspection
		"plugin_grabber.apply_transient_shaper_controls",
		"plugin_grabber.inspect_multiband",
		"plugin_grabber.apply_multiband_controls",
		"plugin_grabber.inspect_limiter",           // identity-free limiter stage inspection
		"plugin_grabber.inspect_de_esser",          // identity-free frequency-selective sibilance stage inspection
		"plugin_grabber.apply_de_esser_controls",   // typed atomic De-esser control surface
		"plugin_grabber.inspect_spectral_dynamics", // identity-free spectral field/global-law inspection
		"plugin_grabber.apply_limiter_controls",
		"plugin_grabber.inspect_compressor", // identity-free compressor stage/control-path inspection
		"plugin_grabber.apply_compressor_controls",
		"plugin_grabber.apply_eq_edits", // generic static EQ atomic planner/executor
		"plugin_grabber.set_eq_point",   // deterministic Go-side band selection; preferred over set_plugin_param for EQ
		"control.add_macro", "control.rename_macro", "control.add_binding", "control.set_macro_values",
	}
}

func agentLoopMixTools() []string {
	return []string{
		"ccb.observation_catalog", "ccb.observation_request",
		"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
		"mix.propose_tick", "mix.apply_tick", "mix.rollback_tick",
		"project.undo", "project.redo",
	}
}

func agentLoopStaticMixGainStagingTools() []string {
	return []string{
		"project.state", "project.audio_analysis_status", "project.audio_analysis_start", "track.list",
		"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
		"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
		"track.group.list", "track.group.apply_control",
		"project.undo", "project.redo",
	}
}

func agentLoopStaticMixStaticBalanceTools() []string {
	return []string{
		"project.state",
		"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
		"mix.propose_tick", "mix.apply_tick", "mix.rollback_tick",
		"project.undo", "project.redo",
	}
}

func agentLoopStaticMixPanLayoutTools() []string {
	return []string{
		"project.state", "project.audio_analysis_status",
		"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
		"mix.apply_pan_layout_batch",
		"project.undo", "project.redo",
	}
}

func agentLoopTransportTools() []string {
	return []string{
		"transport.play", "transport.stop", "transport.return_to_zero", "transport.seek",
		"transport.toggle_click", "transport.set_click", "transport.set_tempo",
		"transport.record.start", "transport.record.stop",
	}
}

func agentLoopVersionTools() []string {
	return []string{
		"version.status", "version.checkpoint", "version.list", "version.show", "version.diff",
		"version.restore_preview", "version.restore", "version.branch_create", "version.node_checkout", "version.node_delete",
		"version.worktree_create", "version.worktree_checkout", "version.worktree_list", "version.project_new", "version.project_opened", "version.project_save_prepare", "version.project_saved", "version.checkout",
	}
}

func agentLoopArtifactTools() []string {
	return []string{
		"artifact.list", "artifact.read", "artifact.extract",
		"media.register_assets", "media.index_authorized_folder",
		"browser.fetch", "browser.search",
		"web.search", "web.fetch",
	}
}

func agentLoopMediaTools() []string {
	return []string{
		"artifact.list", "artifact.read", "artifact.extract",
		"media.register_assets", "media.index_authorized_folder",
	}
}

func agentLoopProjectAudioTools() []string {
	return []string{
		"project.get_audio_settings", "project.validate_audio_settings_change", "project.set_audio_settings",
		"project.import_preflight", "project.import_folder_as_stems", "project.import_audio_files", "project.audio_analysis_status", "media.inspect_files",
	}
}

func agentLoopProjectMarkerTools() []string {
	return []string{
		"project.markers.list", "project.markers.upsert", "project.markers.apply_section_markers",
		"project.markers.rename", "project.markers.delete",
	}
}

func agentLoopWorkspaceTools() []string {
	return []string{
		"workspace.glob", "workspace.grep", "workspace.read_file", "workspace.file_info",
		"logs.read", "logs.search",
	}
}

func sortedToolNameKeys(seen map[string]bool) []string {
	names := make([]string, 0, len(seen))
	for name := range seen {
		if strings.TrimSpace(name) != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func isPlanReadOnlyTool(tool tools.Tool) bool {
	name := strings.TrimSpace(tool.Name)
	if name == "" || name == "daw.invoke" {
		return false
	}
	if tool.MutatesProject || tool.RequiresConfirmation || tool.RiskLevel != tools.RiskDirect {
		return false
	}
	switch name {
	case "transport.play", "transport.stop", "transport.seek", "transport.record.start", "transport.record.stop", "plugin.open", "plugin.show_editor":
		return false
	}
	switch tool.Namespace {
	case "project", "track", "clip", "midi", "plugin", "plugin_grabber", "workspace", "project_history", "runtime", "artifact", "browser", "media":
		return true
	default:
		return false
	}
}
func agentLoopUserText(message string) string {
	msg := strings.TrimSpace(message)
	lower := strings.ToLower(msg)
	if lower == "/goal" {
		return ""
	}
	if strings.HasPrefix(lower, "/goal ") {
		return strings.TrimSpace(msg[len("/goal "):])
	}
	return msg
}

func isContinueMessage(message string) bool {
	switch strings.ToLower(strings.TrimSpace(message)) {
	case "\u7ee7\u7eed", "\u7ee7\u7eed\u3002", "\u63a5\u7740", "\u63a5\u7740\u505a", "\u7ee7\u7eed\u6267\u884c", "continue", "go on", "resume":
		return true
	default:
		return false
	}
}

func isApprovalDecision(decision string) bool {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approve", "allow", "confirm", "yes", "y", "ok", "\u786e\u8ba4", "\u540c\u610f":
		return true
	default:
		return false
	}
}

func contextBool(ctx map[string]any, key string) bool {
	if ctx == nil {
		return false
	}
	switch value := ctx[key].(type) {
	case bool:
		return value
	case string:
		s := strings.ToLower(strings.TrimSpace(value))
		return s == "1" || s == "true" || s == "yes" || s == "on"
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}

func mergeContext(base map[string]any, overlay map[string]any) map[string]any {
	out := cloneContext(base)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
