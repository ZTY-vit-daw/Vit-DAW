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
	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
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
	case agentruntime.StatusCompleted, agentruntime.StatusFailed, agentruntime.StatusCancelled:
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
	chatContext := contextWithUserMessage(req.Context, userText)
	mode := agentModeFromContext(chatContext)
	messageLoop := s.newAgentMessageLoop(cfg, mode)
	legacyRunner := s.newAgentLoopRunner(cfg, mode)

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
		cont.Budget = agentLoopBudgetForMode(mode)
		if useLegacyPlannerLoop() {
			res = legacyRunner.Continue(ctx, cont)
		} else {
			res = messageLoop.Continue(ctx, cont)
		}
	} else if cont, ok := s.resumeContinuationForChat(conversationID, req.Context); ok && shouldResumeGoalFromStatus(s.harness.RuntimeStatus(cont.GoalID).Status) {
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
		cont.Budget = agentLoopBudgetForMode(mode)
		if useLegacyPlannerLoop() {
			res = legacyRunner.Continue(ctx, cont)
		} else {
			res = messageLoop.Continue(ctx, cont)
		}
	} else {
		goalID, runID := s.freshAgentLoopGoalIDs(req.Context)
		projectPath := projectPathFromChatContext(chatContext)
		projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		state := s.harness.UserStateSummary(ctx)
		if len(projectHistory) > 0 {
			state = cloneContext(state)
			state["project_history"] = projectHistory
		}
		toolContext := s.agentLoopToolContext(mode, userText, chatContext)
		if useLegacyPlannerLoop() {
			res = legacyRunner.Start(ctx, agentloop.Input{
				GoalID:         goalID,
				RunID:          runID,
				UserText:       userText,
				Summary:        userText,
				Context:        chatContext,
				ProjectHistory: projectHistory,
				Conversation:   s.freshAgentLoopConversation(conversationID, userText, projectHistory),
				State:          state,
				CatalogSummary: toolContext.CatalogSummary,
				AllowedTools:   toolContext.AllowedTools,
				Budget:         agentLoopBudgetForMode(mode),
			})
		} else {
			res = messageLoop.Start(ctx, agentloop.Input{
				GoalID:         goalID,
				RunID:          runID,
				UserText:       userText,
				Summary:        userText,
				Context:        chatContext,
				ProjectHistory: projectHistory,
				Conversation:   s.freshAgentLoopConversation(conversationID, userText, projectHistory),
				State:          state,
				CatalogSummary: toolContext.CatalogSummary,
				AllowedTools:   toolContext.AllowedTools,
				Budget:         agentLoopBudgetForMode(mode),
			})
		}
	}
	return s.chatResponseFromAgentLoopResult(conversationID, mode, res), true
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
	if e.server != nil {
		e.server.emitToolItemStarted(in, toolCallID)
	}
	if e.server == nil || !isPluginGrabberLearnToolCall(in.ToolCall) {
		out, err := e.base.RunToolCall(ctx, in)
		if e.server != nil {
			if strings.TrimSpace(out.ToolCallID) == "" {
				out.ToolCallID = toolCallID
			}
			e.server.emitToolItemCompleted(in, out, err)
		}
		return out, err
	}
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
	workflowCmd, _ := pluginGrabberLearningInvokeCommand(req)
	resp, err := e.server.invokePluginGrabberLearningWorkflow(ctx, req, workflowCmd, e.cfg)
	out := executorpkg.Result{
		ToolCallID:           toolCallID,
		Tool:                 firstNonEmpty(resp.Tool, in.ToolCall.Tool),
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
	if e.server != nil {
		e.server.emitToolItemCompleted(in, out, err)
	}
	return out, err
}

func isPluginGrabberLearnToolCall(call planner.ToolCall) bool {
	tool := strings.TrimSpace(call.Tool)
	if tool == pluginGrabberLearnTool || tool == "plugin_grabber.learn_project_profile" || tool == "plugin.learn_project_profile" || tool == "plugin_learn_project_profile" {
		return true
	}
	cmd := workflowCommandArgs(call.Command)
	name := strings.TrimSpace(fmt.Sprint(cmd["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(cmd["command"]))
	}
	return name == pluginGrabberLearnCommand
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

func agentLoopBudgetForMode(mode string) agentloop.Budget {
	switch agentModeFromString(mode) {
	case agentModeGoal:
		return agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	case agentModePlan:
		return agentloop.Budget{MaxTurns: 8, MaxToolCalls: 12}
	default:
		return agentloop.Budget{MaxTurns: 8, MaxToolCalls: 12}
	}
}
func (s *Server) chatResponseFromAgentLoopResult(conversationID, mode string, res agentloop.Result) ChatResponse {
	s.recordGoalResult(conversationID, res)
	reply := strings.TrimSpace(res.Reply)
	if reply == "" {
		switch res.Status {
		case agentruntime.StatusWaitingConfirmation:
			reply = "\u9700\u8981\u4f60\u786e\u8ba4\u540e\u518d\u7ee7\u7eed\u3002"
		case agentruntime.StatusWaitingClarification:
			reply = firstNonEmpty(res.ClarificationQuestion, "\u9700\u8981\u4f60\u5148\u8865\u5145\u4e00\u4e2a\u7f16\u8f91\u76ee\u6807\u3002")
		case agentruntime.StatusWaitingContinue:
			reply = "\u672c\u8f6e\u5df2\u6682\u505c\u3002\u4f60\u53ef\u4ee5\u8bf4\u201c\u7ee7\u7eed\u201d\u63a5\u7740\u8dd1\u3002"
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
	if res.StopReason == agentloop.StopReasonLimitReached && !strings.Contains(reply, "\u7ee7\u7eed") {
		reply += " \u4f60\u53ef\u4ee5\u8bf4\u201c\u7ee7\u7eed\u201d\u63a5\u7740\u8dd1\u3002"
	}
	resp := ChatResponse{
		ConversationID:      conversationID,
		GoalID:              res.GoalID,
		RunID:               res.RunID,
		Reply:               reply,
		AgentMode:           mode,
		NeedsConfirmation:   res.Status == agentruntime.StatusWaitingConfirmation,
		Preview:             res.Preview,
		ExecutedKernelReply: res.Executed,
		ProjectResultCards:  projectResultCardsFromExecuted(res.Executed),
		Artifacts:           artifactSummariesFromExecuted(res.Executed),
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
	if res.Status == agentruntime.StatusWaitingConfirmation && res.Continuation != nil {
		decisions := agentLoopPendingDecisions(res.Continuation)
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
		resp.Commands = decisions
	}
	if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil && strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
		resp.NeedsConfirmation = true
		resp.GoalStatus = string(agentruntime.StatusWaitingConfirmation)
		resp.StopReason = agentloop.StopReasonNeedsConfirmation
		resp.Workflow = "mix_treatment"
		resp.WorkflowData = mixTreatmentInteractionPayload(*treatment)
		req := mixTreatmentInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, *treatment)
		resp.InteractionRequests = []AgentInteractionRequest{req}
		s.storePendingInteraction(req, req.Payload)
		if resp.AgentPlan != nil {
			resp.AgentPlan.Status = string(agentruntime.StatusWaitingConfirmation)
		}
	}
	return resp
}

func mixTreatmentInteractionPayload(treatment agentloop.MixTreatmentPending) map[string]any {
	payload := map[string]any{
		"schema_version":    treatment.SchemaVersion,
		"status":            treatment.Status,
		"intent":            treatment.Intent,
		"target_ref":        treatment.TargetRef,
		"action_kind":       treatment.ActionKind,
		"processor_type":    treatment.ProcessorType,
		"delta_db":          treatment.DeltaDB,
		"delta_pan":         treatment.DeltaPan,
		"plugin_id":         treatment.PluginID,
		"plugin_name":       treatment.PluginName,
		"control":           treatment.Control,
		"target":            cloneContext(treatment.Target),
		"confidence":        treatment.Confidence,
		"reasoning_summary": treatment.ReasoningSummary,
		"evidence_refs":     append([]string(nil), treatment.EvidenceRefs...),
		"needs_resolution":  append([]string(nil), treatment.NeedsResolution...),
		"observation_id":    treatment.ObservationID,
	}
	if treatment.TargetPan != nil {
		payload["target_pan"] = *treatment.TargetPan
	}
	removeEmptyTreatmentValues(payload)
	return payload
}

func mixTreatmentInteractionRequest(conversationID, goalID, runID, reply string, treatment agentloop.MixTreatmentPending) AgentInteractionRequest {
	payload := mixTreatmentInteractionPayload(treatment)
	payload["conversation_id"] = conversationID
	return AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "mix_treatment_confirmation",
		Type:           "mix_treatment_confirmation",
		Source:         "vit_agent",
		Workflow:       "mix_treatment",
		Title:          "需要确认混音动作",
		Body:           firstNonEmpty(strings.TrimSpace(reply), "这个混音动作正在等待确认。"),
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
	status := "ok"
	if strings.TrimSpace(resp.Error) != "" {
		status = "error"
	}
	responsePlanID := AgentLoopConfirmResponsePlanID(planID, resp)
	projectHistory := resp.ProjectHistory
	if strings.TrimSpace(resp.Reply) != "" {
		historyData := map[string]any{}
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
	syncAgentPlanProjectHistory(&resp)
	s.attachInteractionRequests(&resp)
	return http.StatusOK, map[string]any{
		"status":                status,
		"message":               resp.Reply,
		"reply":                 resp.Reply,
		"plan_id":               responsePlanID,
		"confirmed_plan_id":     planID,
		"next_plan_id":          resp.PlanID,
		"goal_id":               resp.GoalID,
		"run_id":                resp.RunID,
		"agent_mode":            mode,
		"goal_status":           resp.GoalStatus,
		"goal_summary":          resp.GoalSummary,
		"current_step":          resp.CurrentStep,
		"completed_steps":       resp.CompletedSteps,
		"stop_reason":           resp.StopReason,
		"limit_type":            resp.LimitType,
		"needs_confirmation":    resp.NeedsConfirmation,
		"preview":               resp.Preview,
		"replies":               resp.ExecutedKernelReply,
		"executed_kernel_reply": resp.ExecutedKernelReply,
		"project_result_cards":  resp.ProjectResultCards,
		"artifacts":             resp.Artifacts,
		"project_history":       projectHistory,
		"agent_plan":            resp.AgentPlan,
		"interaction_requests":  resp.InteractionRequests,
		"error":                 resp.Error,
	}
}

func AgentLoopConfirmResponsePlanID(confirmedPlanID string, resp ChatResponse) string {
	if resp.NeedsConfirmation {
		if nextPlanID := strings.TrimSpace(resp.PlanID); nextPlanID != "" {
			return nextPlanID
		}
	}
	return confirmedPlanID
}

func (s *Server) recordGoalResult(conversationID string, res agentloop.Result) {
	if s == nil || res.GoalID == "" {
		return
	}
	var pendingEvents []AgentEvent
	s.mu.Lock()
	if strings.TrimSpace(conversationID) != "" {
		s.conversationGoals[conversationID] = res.GoalID
		if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil && strings.EqualFold(strings.TrimSpace(candidate.Status), "pending_confirmation") {
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
				Title:    "Mix tick pending confirmation",
				Body:     pendingMixTickEventBody(*candidate),
				Payload:  pendingMixTickEventPayload(*candidate, candidate.ObservationID),
			})
		}
		if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil && strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
			if s.pendingTreatments == nil {
				s.pendingTreatments = map[string]agentloop.MixTreatmentPending{}
			}
			s.pendingTreatments[conversationID] = *treatment
			if s.logger != nil {
				s.logger.Info("[mix.treatment.pending] stored conversation=%s goal=%s run=%s action=%s processor=%s target=%s observation=%s",
					conversationID, res.GoalID, res.RunID, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef, treatment.ObservationID)
			}
			treatmentPayload := map[string]any{
				"schema_version":    treatment.SchemaVersion,
				"intent":            treatment.Intent,
				"target_ref":        treatment.TargetRef,
				"action_kind":       treatment.ActionKind,
				"processor_type":    treatment.ProcessorType,
				"delta_db":          treatment.DeltaDB,
				"delta_pan":         treatment.DeltaPan,
				"plugin_id":         treatment.PluginID,
				"plugin_name":       treatment.PluginName,
				"control":           treatment.Control,
				"target":            cloneContext(treatment.Target),
				"confidence":        treatment.Confidence,
				"reasoning_summary": treatment.ReasoningSummary,
				"evidence_refs":     append([]string(nil), treatment.EvidenceRefs...),
				"needs_resolution":  treatment.NeedsResolution,
				"observation_id":    treatment.ObservationID,
			}
			removeEmptyTreatmentValues(treatmentPayload)
			if treatment.TargetPan != nil {
				treatmentPayload["target_pan"] = *treatment.TargetPan
			}
			pendingEvents = append(pendingEvents, AgentEvent{
				Type:     "mix_treatment.pending",
				GoalID:   res.GoalID,
				RunID:    res.RunID,
				ItemType: "mix_treatment",
				Status:   "pending_confirmation",
				Title:    "Mix treatment pending confirmation",
				Body:     fmt.Sprintf("%s treatment for %s is waiting for explicit confirmation.", treatment.ProcessorType, treatment.TargetRef),
				Payload:  treatmentPayload,
			})
		}
	}
	if res.Continuation != nil {
		s.goalContinuations[res.GoalID] = *res.Continuation
	} else {
		delete(s.goalContinuations, res.GoalID)
	}
	s.mu.Unlock()
	for _, pendingEvent := range pendingEvents {
		s.emitAgentEvent(conversationID, pendingEvent)
	}
}

func (s *Server) clearGoalContinuation(goalID string) {
	if s == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.goalContinuations, strings.TrimSpace(goalID))
}

func (s *Server) goalContinuationForConversation(conversationID string) (agentloop.Continuation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	goalID := strings.TrimSpace(s.conversationGoals[conversationID])
	if goalID == "" {
		return agentloop.Continuation{}, false
	}
	cont, ok := s.goalContinuations[goalID]
	return cont, ok
}

func (s *Server) goalContinuationForCurrentGoal(chatContext map[string]any) (agentloop.Continuation, bool) {
	goalID, _ := goalIDsFromContext(chatContext)
	goalID = strings.TrimSpace(goalID)
	if s == nil || goalID == "" {
		return agentloop.Continuation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cont, ok := s.goalContinuations[goalID]
	return cont, ok
}

func (s *Server) resumeContinuationForChat(conversationID string, chatContext map[string]any) (agentloop.Continuation, bool) {
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
	case agentruntime.StatusWaitingClarification, agentruntime.StatusWaitingContinue:
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
		case "track":
			add(agentLoopTrackTools()...)
		case "midi":
			add(agentLoopTrackTools()...)
			add(agentLoopMidiTools()...)
		case "clip":
			add(agentLoopTrackTools()...)
			add(agentLoopClipTools()...)
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
			add(agentLoopClipTools()...)
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
	seen := map[string]bool{}
	add := func(name string) {
		if strings.TrimSpace(name) != "" {
			seen[name] = true
		}
	}
	if agentLoopTextHasAny(text,
		"\u8f68\u9053", "\u97f3\u8f68", "\u9759\u97f3", "\u72ec\u594f", "\u97f3\u91cf", "\u5f55\u97f3", "\u51bb\u7ed3",
		"track", "mute", "solo", "volume", "arm", "freeze",
	) {
		add("track")
	}
	if agentLoopTextHasAny(text,
		"midi", "\u97f3\u7b26", "\u65cb\u5f8b", "\u548c\u5f26", "\u9f13", "\u91cf\u5316", "\u8f6c\u8c03", "\u529b\u5ea6",
		"note", "notes", "melody", "chord", "drum", "quantize", "transpose", "velocity",
	) {
		add("midi")
	}
	if agentLoopTextHasAny(text,
		"clip", "\u7247\u6bb5", "\u97f3\u9891", "\u5bfc\u5165", "\u7d20\u6750", "\u6587\u4ef6", "\u5207\u5206", "\u88c1\u526a", "\u590d\u5236\u7247\u6bb5",
		"audio", "media", "import", "split", "duplicate",
	) {
		add("clip")
	}
	if agentLoopTextHasAny(text,
		"\u63d2\u4ef6", "\u6548\u679c\u5668", "\u5747\u8861", "\u538b\u7f29", "\u6df7\u54cd", "\u5ef6\u8fdf", "\u6293\u624b", "\u53c2\u6570", "\u5b8f\u63a7", "\u5b8f\u63a7\u4ef6", "\u5b8f\u63a7\u5236",
		"plugin", "vst", "eq", "compressor", "reverb", "delay", "grabber", "param", "macro",
	) {
		add("plugin")
	}
	if agentLoopTextHasAny(text,
		"\u6df7\u97f3", "\u7f29\u6df7", "\u4e3b\u5531", "\u4eba\u58f0", "\u58f0\u97f3", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u54cd\u5ea6", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u7535\u5e73", "\u589e\u76ca", "\u52a8\u6001", "\u7a7a\u95f4\u611f", "\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u9ad8\u9891", "\u523a\u8033", "\u6d51\u6d4a", "\u9760\u524d", "\u9760\u540e", "\u5de6", "\u53f3", "\u5c45\u4e2d", "\u56de\u4e2d", "\u66f4\u4eae", "\u66f4\u6697", "\u66f4\u7a33", "\u66f4\u7d27",
		"mix", "mixing", "master", "vocal", "loudness", "too loud", "too quiet", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "presence", "mud", "muddy", "harsh", "bright", "dark", "forward", "back", "space", "depth", "dynamic", "pan", "panning", "stereo", "left", "right", "center", "centre",
	) {
		add("mix")
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
	if agentLoopTextHasAny(text,
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
	if agentLoopTextHasAny(text, "clip", "片段", "音频", "audio", "导入", "素材", "文件", "media", "import", "split", "裁剪", "复制片段", "duplicate") {
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
	if contextHasAnyValue(requestContext, "selected_track_id", "selected_clip_id", "selected_clip_ids", "selected_clip_track_id") && agentLoopTextHasAny(text,
		"\u8c03", "\u6df7", "\u9760\u524d", "\u9760\u540e", "\u54cd\u4e00\u70b9", "\u5c0f\u4e00\u70b9", "\u5927\u4e00\u70b9", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca", "\u7a7a\u95f4", "\u4f4e\u9891", "\u9ad8\u9891", "\u4eba\u58f0", "\u4e3b\u5531",
		"mix", "forward", "back", "louder", "quieter", "too loud", "too quiet", "volume", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "space", "presence", "mud", "harsh", "vocal",
	) {
		add("mix")
	}
	if contextHasAnyValue(requestContext, "selected_plugin_id", "selected_plugin_name") && agentLoopTextHasAny(text, "调", "大一点", "小一点", "亮", "暗", "浑浊", "刺耳", "mud", "harsh", "presence", "boost", "cut") {
		add("plugin")
	}
	if seen["mix"] {
		delete(seen, "track")
	}
	return sortedToolNameKeys(seen)
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
		"track.mute", "track.solo", "track.arm", "track.volume",
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
		"artifact.list", "artifact.read", "artifact.extract",
	}
}

func agentLoopPluginTools() []string {
	return []string{
		"plugin.list_available", "plugin.search", "plugin.semantic_search", "plugin.semantic_get", "plugin.semantic_build_index", "plugin.scan",
		"plugin.load_to_rack", "rack.add_node",
		"plugin.get_parameters", "plugin.set_parameter", "plugin.open", "plugin.show_editor",
		"plugin_grabber.get_project_profiles", "plugin_grabber.explain_controls", "plugin_grabber.learn_project_profile", "plugin_grabber.upsert_project_profile", "plugin_grabber.remove_project_profile", "plugin_grabber.apply_control",
		"control.add_macro", "control.rename_macro", "control.add_binding", "control.set_macro_values",
	}
}

func agentLoopMixTools() []string {
	return []string{
		"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
		"mix.propose_tick", "mix.apply_tick", "mix.rollback_tick",
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
		"version.worktree_create", "version.worktree_checkout", "version.worktree_list", "version.project_new", "version.project_saved", "version.checkout",
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
