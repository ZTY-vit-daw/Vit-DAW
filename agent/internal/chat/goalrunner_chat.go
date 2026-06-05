package chat

import (
	"context"
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
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/tools"
)

const agentLoopConfirmationWorkflow = "agentloop_confirmation"

func (s *Server) beginChatGoal(conversationID, message string, requestContext map[string]any) agentruntime.Goal {
	if s == nil || s.harness == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	if isContinueMessage(message) {
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
func (s *Server) runAgentLoopChat(ctx context.Context, conversationID string, req ChatRequest, cfg config.EngineConfig) (ChatResponse, bool) {
	if s == nil || s.harness == nil {
		return ChatResponse{ConversationID: conversationID, Reply: "AgentLoop unavailable.", Error: "agentloop_unavailable"}, true
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
				Reply:          "No paused agent work is available to continue.",
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
		cont.CatalogSummary = s.harness.ModelCatalogSummary()
		cont.AllowedTools = toolNamesForAgentLoop(s.harness, mode)
		cont.Budget = agentLoopBudgetForMode(mode)
		if useLegacyPlannerLoop() {
			res = legacyRunner.Continue(ctx, cont)
		} else {
			res = messageLoop.Continue(ctx, cont)
		}
	} else if cont, ok := s.goalContinuationForCurrentGoal(req.Context); ok && shouldResumeGoalFromStatus(s.harness.RuntimeStatus(cont.GoalID).Status) {
		cont.UserText = userText
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
		cont.CatalogSummary = s.harness.ModelCatalogSummary()
		cont.AllowedTools = toolNamesForAgentLoop(s.harness, mode)
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
				CatalogSummary: s.harness.ModelCatalogSummary(),
				AllowedTools:   toolNamesForAgentLoop(s.harness, mode),
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
				CatalogSummary: s.harness.ModelCatalogSummary(),
				AllowedTools:   toolNamesForAgentLoop(s.harness, mode),
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
	if e.server == nil || !isPluginGrabberLearnToolCall(in.ToolCall) {
		return e.base.RunToolCall(ctx, in)
	}
	toolCallID := strings.TrimSpace(in.ToolCall.ID)
	if toolCallID == "" {
		toolCallID = "tool_goal_step"
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
		plan := PendingPlan{
			ID:               "plan_" + randomID(),
			CreatedAt:        time.Now(),
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
	}
	return resp
}

func (s *Server) handleAgentLoopConfirm(w http.ResponseWriter, r *http.Request, planID string, plan PendingPlan, decision string) {
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
		projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "cancelled", "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusCancelled), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory))})
		return
	}
	if plan.GoalContinuation == nil {
		err := "goal confirmation continuation is missing"
		s.harness.CompleteGoal(goalID, fmt.Errorf(err))
		projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": err, "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusFailed), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", err, projectHistory))})
		return
	}
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		msg := "AI config is unavailable; cannot resume agentloop."
		if err != nil {
			msg += " " + err.Error()
		}
		s.harness.CompleteGoal(goalID, fmt.Errorf(msg))
		projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": msg, "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": mode, "goal_status": string(agentruntime.StatusFailed), "project_history": projectHistory, "agent_plan": agentPlanForMode(mode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", msg, projectHistory))})
		return
	}
	cont := *plan.GoalContinuation
	cont.State = s.harness.UserStateSummary(r.Context())
	cont.ProjectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), cont.GoalID, projectPath)
	if len(cont.ProjectHistory) > 0 {
		cont.State = cloneContext(cont.State)
		cont.State["project_history"] = cont.ProjectHistory
	}
	cont.CatalogSummary = s.harness.ModelCatalogSummary()
	cont.AllowedTools = toolNamesForAgentLoop(s.harness, mode)
	var res agentloop.Result
	if useLegacyPlannerLoop() {
		runner := s.newAgentLoopRunner(cfg, mode)
		res = runner.ResumeAfterConfirmation(r.Context(), cont)
	} else {
		messageLoop := s.newAgentMessageLoop(cfg, mode)
		res = messageLoop.ResumeAfterConfirmation(r.Context(), cont)
	}
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	status := "ok"
	if strings.TrimSpace(resp.Error) != "" {
		status = "error"
	}
	responsePlanID := AgentLoopConfirmResponsePlanID(planID, resp)
	projectHistory := resp.ProjectHistory
	if strings.TrimSpace(resp.Reply) != "" {
		if updatedHistory := s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "vit", resp.Reply, resp.GoalID, resp.RunID); len(updatedHistory) > 0 {
			projectHistory = updatedHistory
		}
	}
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), resp.GoalID, projectPath)
	}
	resp.ProjectHistory = projectHistory
	syncAgentPlanProjectHistory(&resp)
	writeJSON(w, http.StatusOK, map[string]any{
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
		"project_history":       projectHistory,
		"agent_plan":            resp.AgentPlan,
		"error":                 resp.Error,
	})
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(conversationID) != "" {
		s.conversationGoals[conversationID] = res.GoalID
	}
	if res.Continuation != nil {
		s.goalContinuations[res.GoalID] = *res.Continuation
	} else {
		delete(s.goalContinuations, res.GoalID)
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
	case "project", "track", "clip", "midi", "plugin", "plugin_grabber", "workspace", "project_history", "runtime":
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
