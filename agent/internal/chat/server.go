package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/policy"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

type Server struct {
	kernel  *kernel.Client
	shadow  *shadow.Project
	llm     *llm.Client
	logger  *logx.Logger
	harness *harness.Harness

	mu                sync.Mutex
	conversations     map[string][]llm.Message
	pending           map[string]PendingPlan
	goalContinuations map[string]agentloop.Continuation
	conversationGoals map[string]string
}

type PendingPlan struct {
	ID               string                  `json:"id"`
	CreatedAt        time.Time               `json:"created_at"`
	Decisions        []policy.Decision       `json:"decisions"`
	Context          map[string]any          `json:"context,omitempty"`
	Preview          string                  `json:"preview"`
	Workflow         string                  `json:"workflow,omitempty"`
	WorkflowData     map[string]any          `json:"workflow_data,omitempty"`
	GoalContinuation *agentloop.Continuation `json:"-"`
}

type ChatRequest struct {
	ConversationID string         `json:"conversation_id"`
	Message        string         `json:"message"`
	Context        map[string]any `json:"context,omitempty"`
	Attachments    []Attachment   `json:"attachments,omitempty"`
}

type Attachment struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	Kind      string `json:"kind,omitempty"`
	MIME      string `json:"mime,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Exists    bool   `json:"exists,omitempty"`
}

type ChatResponse struct {
	ConversationID      string            `json:"conversation_id"`
	GoalID              string            `json:"goal_id,omitempty"`
	RunID               string            `json:"run_id,omitempty"`
	Reply               string            `json:"reply"`
	AgentMode           string            `json:"agent_mode,omitempty"`
	AgentPlan           *AgentPlan        `json:"agent_plan,omitempty"`
	NeedsConfirmation   bool              `json:"needs_confirmation"`
	PlanID              string            `json:"plan_id,omitempty"`
	Preview             string            `json:"preview,omitempty"`
	Commands            []policy.Decision `json:"commands,omitempty"`
	ExecutedKernelReply []map[string]any  `json:"executed_kernel_reply,omitempty"`
	GoalStatus          string            `json:"goal_status,omitempty"`
	GoalSummary         string            `json:"goal_summary,omitempty"`
	CurrentStep         string            `json:"current_step,omitempty"`
	CompletedSteps      int               `json:"completed_steps,omitempty"`
	StopReason          string            `json:"stop_reason,omitempty"`
	LimitType           string            `json:"limit_type,omitempty"`
	ProjectHistory      map[string]any    `json:"project_history,omitempty"`
	Error               string            `json:"error,omitempty"`
}

type ConfirmRequest struct {
	PlanID   string `json:"plan_id"`
	Decision string `json:"decision"`
}

type ConfigResponse struct {
	Status    string              `json:"status"`
	Path      string              `json:"path"`
	Config    config.EngineConfig `json:"config"`
	HasAPIKey bool                `json:"hasApiKey"`
}

type modelEnvelope struct {
	Reply    string           `json:"reply"`
	Commands []map[string]any `json:"commands"`
}

const (
	agentModeDefault = "default"
	agentModePlan    = "plan"
	agentModeGoal    = "goal"
)

var devToolSmokeNames = []string{
	"project.undo_state",
	"project.snapshot_export",
	"version.checkpoint",
	"workspace.grep",
	"plugin.semantic_search",
	"web.fetch",
	"shell.run",
}

func New(kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Server {
	return &Server{
		kernel:            kernelClient,
		shadow:            shadowProject,
		llm:               &llm.Client{},
		logger:            logger,
		harness:           harness.New(kernelClient, shadowProject, logger),
		conversations:     map[string][]llm.Message{},
		pending:           map[string]PendingPlan{},
		goalContinuations: map[string]agentloop.Continuation{},
		conversationGoals: map[string]string{},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/agent/state", s.handleState)
	mux.HandleFunc("/agent/chat", s.handleChat)
	mux.HandleFunc("/agent/confirm", s.handleConfirm)
	mux.HandleFunc("/agent/config", s.handleConfig)
	mux.HandleFunc("/agent/tools", s.handleTools)
	mux.HandleFunc("/agent/actions", s.handleActions)
	mux.HandleFunc("/agent/invoke", s.handleInvoke)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "VitAgent"})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	goal := s.harness.RuntimeStatus("")
	projectHistory := s.harness.ProjectHistorySummary(r.Context(), goal.GoalID)
	agentPlan := s.activeGoalPlan(goal, projectHistory)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"shadow": s.harness.StateSummary(r.Context()),
		"goal":   goal,
		"active_goal": map[string]any{
			"goal":            goal,
			"agent_plan":      agentPlan,
			"project_history": projectHistory,
		},
		"project_history": projectHistory,
		"direct_commands": s.harness.DirectCommandNames(),
		"tool_count":      len(s.harness.Tools()),
	})
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"tools":           s.harness.Tools(),
		"command_catalog": s.harness.Commands(),
	})
}

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"actions": s.harness.Actions(limit),
	})
}

func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var req harness.InvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		req.Source = "http"
	}
	if workflowCmd, ok := pluginGrabberExplainInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberExplainWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	resp, err := s.harness.Invoke(r.Context(), req)
	status := http.StatusOK
	if err != nil && resp.Status == "error" {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, resp)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, path, err := config.Load()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		hasAPIKey := strings.TrimSpace(cfg.APIKey) != ""
		cfg.APIKey = ""
		writeJSON(w, http.StatusOK, ConfigResponse{Status: "ok", Path: path, Config: cfg, HasAPIKey: hasAPIKey})
	case http.MethodPost:
		var cfg config.EngineConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
			return
		}
		cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
		cfg.APIKey = strings.TrimSpace(cfg.APIKey)
		cfg.DefaultModel = strings.TrimSpace(cfg.DefaultModel)
		if cfg.APIKey == "" {
			current, _, err := config.Load()
			if err == nil {
				cfg.APIKey = strings.TrimSpace(current.APIKey)
			}
		}
		path, err := config.Save(cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		hasAPIKey := strings.TrimSpace(cfg.APIKey) != ""
		cfg.APIKey = ""
		writeJSON(w, http.StatusOK, ConfigResponse{Status: "ok", Path: path, Config: cfg, HasAPIKey: hasAPIKey})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET or POST required"})
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "message is required"})
		return
	}
	req.Context = contextWithAttachments(req.Context, req.Attachments)
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = "chat_" + randomID()
	}
	projectPath := projectPathFromChatContext(req.Context)
	agentMode := agentModeFromContext(req.Context)
	goal := s.beginChatGoal(conversationID, req.Message, req.Context)
	req.Context = contextWithGoal(req.Context, goal.GoalID, goal.RunID)
	s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "ask", req.Message, goal.GoalID, goal.RunID)
	writeChat := func(status int, resp ChatResponse) {
		if resp.AgentMode == "" {
			resp.AgentMode = agentMode
		}
		if resp.GoalID == "" {
			resp.GoalID = goal.GoalID
		}
		if resp.RunID == "" {
			resp.RunID = goal.RunID
		}
		goalID := firstNonEmpty(resp.GoalID, goal.GoalID)
		if strings.TrimSpace(resp.Reply) != "" {
			if history := s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "vit", resp.Reply, goalID, resp.RunID); len(history) > 0 {
				resp.ProjectHistory = history
			}
		}
		if len(resp.ProjectHistory) == 0 {
			resp.ProjectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		}
		if resp.GoalStatus == "" {
			resp.GoalStatus = inferredGoalStatus(resp)
		}
		if resp.GoalSummary == "" {
			resp.GoalSummary = firstNonEmpty(goal.Summary, req.Message)
		}
		responseMode := agentModeFromString(resp.AgentMode)
		if resp.AgentPlan == nil && shouldExposeAgentPlan(responseMode, resp) {
			resp.AgentPlan = simpleAgentPlan(resp.GoalID, resp.RunID, agentruntime.GoalStatus(resp.GoalStatus), resp.GoalSummary, resp.CurrentStep, resp.Error, resp.ProjectHistory)
		}
		if resp.AgentPlan != nil {
			syncAgentPlanProjectHistory(&resp)
		}
		switch {
		case resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingClarification):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingClarification, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingContinue):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
		case resp.GoalStatus == string(agentruntime.StatusCancelled):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		case resp.GoalStatus == string(agentruntime.StatusRunning):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusRunning, nil)
		case strings.TrimSpace(resp.Error) != "":
			s.harness.CompleteGoal(goalID, fmt.Errorf("%s", resp.Error))
		default:
			s.harness.CompleteGoal(goalID, nil)
		}
		writeJSON(w, status, resp)
	}

	if !strings.HasPrefix(req.Message, "/") {
		if plan, ok := s.pendingPlanForChat(conversationID, req.Context); ok {
			projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goal.GoalID, projectPath)
			pendingAgentMode := agentModeFromContext(plan.Context)
			resp := ChatResponse{
				ConversationID:    conversationID,
				AgentMode:         pendingAgentMode,
				Reply:             "Current operation is still waiting for confirmation. Please confirm or cancel it first.",
				AgentPlan:         agentPlanForMode(pendingAgentMode, agentPlanFromPendingPlan(plan, agentruntime.StatusWaitingConfirmation, projectHistory)),
				NeedsConfirmation: true,
				PlanID:            plan.ID,
				Preview:           plan.Preview,
				Commands:          plan.Decisions,
				GoalStatus:        string(agentruntime.StatusWaitingConfirmation),
				ProjectHistory:    projectHistory,
			}
			s.remember(conversationID, req.Message, resp.Reply)
			writeChat(http.StatusOK, resp)
			return
		}
	}

	if isPendingConfirmationStatusQuestion(req.Message) {
		if plan, ok := s.pendingPlanForChat(conversationID, req.Context); ok {
			pendingAgentMode := agentModeFromContext(plan.Context)
			resp := ChatResponse{
				ConversationID:    conversationID,
				AgentMode:         pendingAgentMode,
				AgentPlan:         agentPlanForMode(pendingAgentMode, agentPlanFromPendingPlan(plan, agentruntime.StatusWaitingConfirmation, nil)),
				Reply:             "Not yet. The previous operation is still waiting for confirmation, so it has not executed.",
				NeedsConfirmation: true,
				PlanID:            plan.ID,
				Preview:           plan.Preview,
				Commands:          plan.Decisions,
			}
			s.remember(conversationID, req.Message, resp.Reply)
			writeChat(http.StatusOK, resp)
			return
		}
	}

	if resp, handled := s.handleDevChatCommand(r.Context(), conversationID, req); handled {
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}

	chatContext := contextWithUserMessage(req.Context, req.Message)
	cfg, cfgPath, err := config.Load()
	if err != nil {
		writeChat(http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          "VitAgent started, but reading AI config failed: " + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	if !cfg.Complete() {
		writeChat(http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          fmt.Sprintf("VitAgent started, but AI config is incomplete. Please configure %s or VIT_AGENT_LLM_* environment variables.", cfgPath),
			Error:          "llm_config_incomplete",
		})
		return
	}

	if shouldUseAgentLoop(req.Message, chatContext) {
		resp, handled := s.runAgentLoopChat(r.Context(), conversationID, req, cfg)
		if handled {
			s.remember(conversationID, req.Message, resp.Reply)
			writeChat(http.StatusOK, resp)
			return
		}
	}

	messages := s.buildMessages(r.Context(), conversationID, req.Message, req.Context)
	raw, err := s.llm.Complete(r.Context(), cfg, messages)
	if err != nil {
		writeChat(http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          "Ask Vit AI call failed: " + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	env := parseModelEnvelope(raw)
	if strings.TrimSpace(env.Reply) == "" {
		env.Reply = strings.TrimSpace(raw)
	}

	if libraryCommands := synthesizePluginLibraryCommands(req.Message); len(libraryCommands) > 0 && !looksLikePluginGrabberLoadIntent(req.Message) {
		env.Commands = libraryCommands
	}
	if len(env.Commands) == 0 {
		env.Commands = synthesizeLocalDAWCommands(req.Message, chatContext)
	}
	if len(env.Commands) == 0 {
		env.Commands = synthesizePluginGrabberLoadCommands(req.Message, chatContext)
	}
	if workflowCmd, ok := coercePluginGrabberLoadCommand(env.Commands, req.Message, chatContext); ok {
		resp := s.runPluginGrabberLoadWorkflow(r.Context(), conversationID, req.Message, chatContext, workflowCmd)
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}
	if workflowCmd, ok := coercePluginGrabberExplainCommand(env.Commands, req.Message, chatContext); ok {
		resp := s.runPluginGrabberExplainWorkflow(r.Context(), conversationID, req.Message, chatContext, workflowCmd)
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}
	if len(env.Commands) == 0 {
		env.Commands = synthesizePluginGrabberLearningCommands(req.Message, chatContext)
	}
	if workflowCmd, ok := firstPluginGrabberLearningCommand(env.Commands); ok {
		resp := s.runPluginGrabberLearningWorkflow(r.Context(), conversationID, req.Message, chatContext, cfg, workflowCmd)
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}
	if resp, handled := s.chatResponseForCommands(r.Context(), conversationID, req.Message, env.Reply, env.Commands, chatContext); handled {
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}
	if len(env.Commands) == 0 {
		reply := sanitizeUserReply(env.Reply, s.harness.UserStateSummary(r.Context()), req.Message)
		s.remember(conversationID, req.Message, reply)
		writeChat(http.StatusOK, ChatResponse{ConversationID: conversationID, Reply: reply})
		return
	}
}

func (s *Server) planModeBlockedResponse(ctx context.Context, conversationID, userMessage string, decisions []policy.Decision, chatContext map[string]any) (ChatResponse, bool) {
	if agentModeFromContext(chatContext) != agentModePlan {
		return ChatResponse{}, false
	}
	blocked := planModeBlockedDecisions(decisions)
	if len(blocked) == 0 {
		return ChatResponse{}, false
	}
	names := decisionNames(blocked)
	reply := "Plan mode blocks DAW project mutations/high-risk writes. Blocked: " + strings.Join(names, ", ") + ". Switch to Default or Goal mode to execute."
	steps := make([]AgentPlanStep, 0, len(decisions))
	for _, decision := range decisions {
		status := "pending"
		if planModeDecisionBlocked(decision) {
			status = "cancelled"
		}
		steps = append(steps, AgentPlanStep{
			ID:          decisionDisplayName(decision),
			Description: decisionDisplayName(decision),
			Status:      status,
			Metadata:    map[string]any{"risk": string(decision.Risk), "reason": decision.Reason},
		})
	}
	projectPath := projectPathFromChatContext(chatContext)
	goalID, runID := goalIDsFromContext(chatContext)
	projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          reply,
		AgentMode:      agentModePlan,
		Commands:       decisions,
		GoalStatus:     string(agentruntime.StatusCompleted),
		GoalSummary:    firstNonEmpty(strings.TrimSpace(userMessage), "Plan mode"),
		ProjectHistory: projectHistory,
		AgentPlan: &AgentPlan{
			GoalID:         goalID,
			RunID:          runID,
			Status:         string(agentruntime.StatusCompleted),
			Summary:        "Plan mode: project changes not executed",
			Steps:          steps,
			FailureReason:  "Plan mode blocks DAW project mutations and high-risk writes.",
			ProjectHistory: projectHistory,
		},
	}, true
}

func planModeBlockedDecisions(decisions []policy.Decision) []policy.Decision {
	blocked := make([]policy.Decision, 0, len(decisions))
	for _, decision := range decisions {
		if planModeDecisionBlocked(decision) {
			blocked = append(blocked, decision)
		}
	}
	return blocked
}

func planModeDecisionBlocked(decision policy.Decision) bool {
	spec, ok := commandSpecForDecision(decision)
	if !ok {
		return decision.Risk != policy.RiskDirect
	}
	if spec.MutatesProject {
		return true
	}
	if spec.Category == "web" {
		return false
	}
	if spec.Category == "shell" {
		return true
	}
	if spec.Category == "workspace" && spec.RiskLevel != tools.RiskDirect {
		return true
	}
	if spec.Category == "project_history" && spec.RiskLevel != tools.RiskDirect {
		return true
	}
	return spec.RiskLevel == tools.RiskConfirm
}

func commandSpecForDecision(decision policy.Decision) (tools.CommandSpec, bool) {
	catalog := tools.DefaultCatalog()
	if name := strings.TrimSpace(decision.Name); name != "" {
		if spec, ok := catalog.LookupCommand(name); ok {
			return spec, true
		}
	}
	if cmdName := policy.CommandName(decision.Command); strings.TrimSpace(cmdName) != "" {
		if spec, ok := catalog.LookupCommand(cmdName); ok {
			return spec, true
		}
	}
	if toolName := strings.TrimSpace(fmt.Sprint(decision.Command["tool"])); toolName != "" && toolName != "<nil>" {
		if spec, ok := catalog.LookupTool(toolName); ok {
			return spec, true
		}
	}
	return tools.CommandSpec{}, false
}

func decisionNames(decisions []policy.Decision) []string {
	out := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		out = append(out, decisionDisplayName(decision))
	}
	return out
}

func decisionDisplayName(decision policy.Decision) string {
	if name := strings.TrimSpace(decision.Name); name != "" {
		return name
	}
	if name := policy.CommandName(decision.Command); strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	if toolName := strings.TrimSpace(fmt.Sprint(decision.Command["tool"])); toolName != "" && toolName != "<nil>" {
		return toolName
	}
	return "<unknown>"
}

func shouldExposeAgentPlan(mode string, resp ChatResponse) bool {
	if resp.AgentPlan != nil {
		return true
	}
	switch agentModeFromString(mode) {
	case agentModePlan, agentModeGoal:
		return true
	default:
		return false
	}
}

func agentPlanForMode(mode string, plan *AgentPlan) *AgentPlan {
	switch agentModeFromString(mode) {
	case agentModePlan, agentModeGoal:
		return plan
	default:
		return nil
	}
}

func (s *Server) chatResponseForCommands(ctx context.Context, conversationID, userMessage, fallbackReply string, commands []map[string]any, chatContext map[string]any) (ChatResponse, bool) {
	decisions := policy.Analyze(commands)
	if len(decisions) == 0 {
		return ChatResponse{}, false
	}
	if resp, blocked := s.planModeBlockedResponse(ctx, conversationID, userMessage, decisions, chatContext); blocked {
		return resp, true
	}
	if policy.NeedsConfirmation(decisions) {
		preview, previewErr := s.confirmationPreview(ctx, decisions, chatContext)
		if previewErr != nil {
			reply := friendlyExecutionError(previewErr)
			return ChatResponse{
				ConversationID: conversationID,
				Reply:          reply,
				Commands:       decisions,
				Error:          previewErr.Error(),
			}, true
		}
		plan := PendingPlan{
			ID:        "plan_" + randomID(),
			CreatedAt: time.Now(),
			Decisions: decisions,
			Context:   chatContext,
			Preview:   preview,
		}
		s.mu.Lock()
		s.pending[plan.ID] = plan
		s.mu.Unlock()
		reply := confirmationReply(decisions)
		return ChatResponse{
			ConversationID:    conversationID,
			Reply:             reply,
			NeedsConfirmation: true,
			PlanID:            plan.ID,
			Preview:           plan.Preview,
			Commands:          decisions,
		}, true
	}

	beforeState := s.harness.UserStateSummary(ctx)
	replies, execErr := s.executeDecisions(ctx, decisions, false, chatContext)
	reply := executedReply(beforeState, s.harness.UserStateSummary(ctx), decisions, replies)
	if strings.TrimSpace(reply) == "" {
		reply = strings.TrimSpace(fallbackReply)
	}
	if strings.TrimSpace(reply) == "" {
		reply = "Done."
	}
	resp := ChatResponse{
		ConversationID:      conversationID,
		Reply:               reply,
		Commands:            decisions,
		ExecutedKernelReply: replies,
	}
	if execErr != nil {
		resp.Error = execErr.Error()
		resp.Reply = friendlyExecutionError(execErr)
	}
	return resp, true
}

func (s *Server) pendingPlanForChat(conversationID string, chatContext map[string]any) (PendingPlan, bool) {
	goalID, _ := goalIDsFromContext(chatContext)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, plan := range s.pending {
		if strings.TrimSpace(fmt.Sprint(plan.WorkflowData["conversation_id"])) == conversationID {
			return plan, true
		}
		if goalID != "" && strings.TrimSpace(fmt.Sprint(plan.Context["goal_id"])) == goalID {
			return plan, true
		}
	}
	if len(s.pending) == 1 {
		for _, plan := range s.pending {
			return plan, true
		}
	}
	return PendingPlan{}, false
}

func isPendingConfirmationStatusQuestion(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return strings.Contains(message, "\u52a0\u8f7d\u4e86\u5417") ||
		strings.Contains(message, "\u6267\u884c\u4e86\u5417") ||
		strings.Contains(message, "\u5b8c\u6210\u4e86\u5417") ||
		strings.Contains(message, "\u6210\u529f\u4e86\u5417") ||
		strings.Contains(text, "did you load") ||
		strings.Contains(text, "did it load") ||
		strings.Contains(text, "loaded yet") ||
		strings.Contains(text, "was it loaded")
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	planID := strings.TrimSpace(req.PlanID)
	s.mu.Lock()
	plan, ok := s.pending[planID]
	if ok {
		delete(s.pending, planID)
	}
	s.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "message": "plan not found"})
		return
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	projectPath := projectPathFromChatContext(plan.Context)
	agentMode := agentModeFromContext(plan.Context)
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if plan.Workflow == agentLoopConfirmationWorkflow {
		s.handleAgentLoopConfirm(w, r, planID, plan, decision)
		return
	}
	if decision != "approve" && decision != "allow" && decision != "confirm" && decision != "yes" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		projectHistory := s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "vit", "cancelled", goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		}
		response := map[string]any{"status": "ok", "message": "cancelled", "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": agentMode, "goal_status": string(agentruntime.StatusCancelled), "project_history": projectHistory}
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	beforeState := s.harness.UserStateSummary(r.Context())
	replies, err := s.executeDecisions(r.Context(), plan.Decisions, true, plan.Context)
	if err != nil {
		s.harness.CompleteGoal(goalID, err)
		projectHistory := s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "vit", err.Error(), goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
		}
		response := map[string]any{
			"status":          "error",
			"message":         err.Error(),
			"plan_id":         planID,
			"goal_id":         goalID,
			"run_id":          runID,
			"agent_mode":      agentMode,
			"replies":         replies,
			"project_history": projectHistory,
		}
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", err.Error(), projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(r.Context()), plan.Decisions, replies)
	if plan.Workflow == pluginGrabberLoadCommand {
		message, replies = s.finishPluginGrabberLoadWorkflow(r.Context(), plan, replies, message)
	}
	if plan.Workflow == "goal_ui_smoke" {
		message = "done"
	}
	if strings.TrimSpace(message) == "" {
		message = "done"
	}
	s.harness.CompleteGoal(goalID, nil)
	projectHistory := s.harness.RecordConversationNodeForProject(r.Context(), projectPath, "vit", message, goalID, runID)
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
	}
	response := map[string]any{
		"status":          "ok",
		"message":         message,
		"plan_id":         planID,
		"goal_id":         goalID,
		"run_id":          runID,
		"agent_mode":      agentMode,
		"replies":         replies,
		"project_history": projectHistory,
		"goal_status":     string(agentruntime.StatusCompleted),
	}
	if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCompleted, "", "", "", projectHistory)); plan != nil {
		response["agent_plan"] = plan
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleDevChatCommand(ctx context.Context, conversationID string, req ChatRequest) (ChatResponse, bool) {
	msg := strings.TrimSpace(req.Message)
	if msg == "/tools" || msg == "/agent/tools" {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          s.devToolSmokeReply(),
		}, true
	}
	if msg == "/actions" || strings.HasPrefix(msg, "/actions ") {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          s.devActionsReply(msg),
		}, true
	}
	if msg == "/smoke rollback" {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          s.runRollbackSmoke(ctx, req.Context),
		}, true
	}
	if msg == "/smoke workspace" {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          s.runWorkspaceSmoke(ctx),
		}, true
	}
	if msg == "/smoke plugin_semantics" || msg == "/smoke plugins" {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          s.runPluginSemanticSmoke(ctx),
		}, true
	}
	if msg == "/smoke goal" {
		return s.startGoalUISmoke(conversationID, req.Context), true
	}
	tool, args, confirmed, ok, err := parseChatToolCommand(msg)
	if !ok {
		return ChatResponse{}, false
	}
	if err != nil {
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          "Tool command parse error: " + err.Error(),
			Error:          err.Error(),
		}, true
	}
	resp, invokeErr := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      tool,
		Args:      args,
		Context:   req.Context,
		Source:    "chat_dev_tool",
		Confirmed: confirmed,
	})
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          renderChatToolResult(resp),
		Error:          errorString(invokeErr),
	}, true
}

func (s *Server) devToolSmokeReply() string {
	seen := map[string]bool{}
	for _, tool := range s.harness.Tools() {
		seen[tool.Name] = true
	}
	var lines []string
	lines = append(lines, "Agent tool smoke list:")
	for _, name := range devToolSmokeNames {
		mark := "missing"
		if seen[name] {
			mark = "ok"
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, mark))
	}
	return strings.Join(lines, "\n")
}

func (s *Server) startGoalUISmoke(conversationID string, chatContext map[string]any) ChatResponse {
	planID := "plan_" + randomID()
	preview := "Goal UI smoke: this does not change the project. Use Cancel to verify cancelled state, or Confirm to verify completed state."
	s.mu.Lock()
	s.pending[planID] = PendingPlan{
		ID:        planID,
		CreatedAt: time.Now(),
		Context:   cloneContext(chatContext),
		Preview:   preview,
		Workflow:  "goal_ui_smoke",
	}
	s.mu.Unlock()
	return ChatResponse{
		ConversationID:    conversationID,
		Reply:             "Goal UI smoke is waiting for confirmation.",
		NeedsConfirmation: true,
		PlanID:            planID,
		Preview:           preview,
	}
}

func (s *Server) runRollbackSmoke(ctx context.Context, chatContext map[string]any) string {
	var lines []string
	lines = append(lines, "Rollback smoke:")
	workspaceOK, workspaceDetail := s.runWorkspaceRollbackSmoke(ctx)
	lines = append(lines, "- workspace reverse patch: "+smokeStatus(workspaceOK)+" "+workspaceDetail)
	dawOK, dawDetail := s.runDAWRollbackSmoke(ctx, chatContext)
	lines = append(lines, "- DAW undo rollback: "+smokeStatus(dawOK)+" "+dawDetail)
	if workspaceOK && dawOK {
		lines = append(lines, "Result: ok")
	} else {
		lines = append(lines, "Result: check details")
	}
	return strings.Join(lines, "\n")
}

func (s *Server) runWorkspaceSmoke(ctx context.Context) string {
	type check struct {
		name string
		run  func() (bool, string)
	}
	checks := []check{
		{name: "write/read text", run: func() (bool, string) {
			resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.write_file",
				Args:      map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.txt", "content": "alpha vit", "overwrite": true},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err != nil || resp.Status != "ok" {
				return false, firstNonEmpty(errorString(err), resp.Error, "write failed")
			}
			read, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:   "workspace.read_file",
				Args:   map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.txt"},
				Source: "chat_smoke",
			})
			if err != nil || read.Status != "ok" {
				return false, firstNonEmpty(errorString(err), read.Error, "read failed")
			}
			if strings.TrimSpace(fmt.Sprint(read.Result["content"])) != "alpha vit" {
				return false, "content mismatch"
			}
			return true, "text file roundtrip"
		}},
		{name: "preview/apply text edit", run: func() (bool, string) {
			preview, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:   "workspace.edit_preview",
				Args:   map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.txt", "old_text": "vit", "new_text": "history"},
				Source: "chat_smoke",
			})
			if err != nil || preview.Status != "ok" {
				return false, firstNonEmpty(errorString(err), preview.Error, "preview failed")
			}
			before, _ := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:   "workspace.read_file",
				Args:   map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.txt"},
				Source: "chat_smoke",
			})
			if strings.TrimSpace(fmt.Sprint(before.Result["content"])) != "alpha vit" {
				return false, "preview mutated file"
			}
			apply, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.apply_edit",
				Args:      map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.txt", "old_text": "vit", "new_text": "history"},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err != nil || apply.Status != "ok" {
				return false, firstNonEmpty(errorString(err), apply.Error, "apply failed")
			}
			return true, "preview was dry, apply changed file"
		}},
		{name: "root escape rejected", run: func() (bool, string) {
			resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.write_file",
				Args:      map[string]any{"root_id": "agent", "path": "../workspace_escape_smoke.txt", "content": "bad", "overwrite": true},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err == nil || resp.Status == "ok" {
				return false, "escape write unexpectedly succeeded"
			}
			return true, "escape blocked"
		}},
		{name: "live .vit write rejected", run: func() (bool, string) {
			resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.write_file",
				Args:      map[string]any{"root_id": "agent", "path": "tmp/workspace_smoke.vit", "content": "bad", "overwrite": true},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err == nil || resp.Status == "ok" {
				return false, ".vit write unexpectedly succeeded"
			}
			return true, ".vit blocked"
		}},
		{name: "blob info/copy", run: func() (bool, string) {
			write, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.write_file",
				Args:      map[string]any{"root_id": "agent", "path": "tmp/blob_source.txt", "content": "blob-ish payload", "overwrite": true},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err != nil || write.Status != "ok" {
				return false, firstNonEmpty(errorString(err), write.Error, "source write failed")
			}
			info, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:   "workspace.blob_info",
				Args:   map[string]any{"root_id": "agent", "path": "tmp/blob_source.txt"},
				Source: "chat_smoke",
			})
			if err != nil || info.Status != "ok" || strings.TrimSpace(fmt.Sprint(info.Result["sha256"])) == "" {
				return false, firstNonEmpty(errorString(err), info.Error, "blob info failed")
			}
			copyResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool:      "workspace.blob_copy",
				Args:      map[string]any{"source_root_id": "agent", "source_path": "tmp/blob_source.txt", "target_root_id": "agent", "target_path": "tmp/blob_copy.txt", "overwrite": true},
				Confirmed: true,
				Source:    "chat_smoke",
			})
			if err != nil || copyResp.Status != "ok" {
				return false, firstNonEmpty(errorString(err), copyResp.Error, "blob copy failed")
			}
			return true, "blob hash and copy ok"
		}},
	}
	lines := []string{"Workspace smoke:"}
	allOK := true
	for _, check := range checks {
		ok, detail := check.run()
		if !ok {
			allOK = false
		}
		lines = append(lines, "- "+check.name+": "+smokeStatus(ok)+" "+detail)
	}
	if allOK {
		lines = append(lines, "Result: ok")
	} else {
		lines = append(lines, "Result: check details")
	}
	return strings.Join(lines, "\n")
}

func (s *Server) runPluginSemanticSmoke(ctx context.Context) string {
	var lines []string
	lines = append(lines, "Plugin semantic smoke:")
	cmds := synthesizePluginLibraryCommands("recommend an eq plugin")
	routeOK := len(cmds) == 1 && strings.TrimSpace(fmt.Sprint(cmds[0]["cmd"])) == "plugin_semantic_search"
	if routeOK {
		lines = append(lines, "- natural language route: ok recommend eq -> plugin_semantic_search")
	} else {
		lines = append(lines, fmt.Sprintf("- natural language route: failed commands=%+v", cmds))
	}
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:   "plugin.semantic_search",
		Args:   map[string]any{"query": "eq", "type": "eq", "limit": 5},
		Source: "chat_smoke",
	})
	searchOK := err == nil && resp.Status == "ok"
	if !searchOK {
		lines = append(lines, "- semantic search: failed "+firstNonEmpty(errorString(err), resp.Error, "unknown error"))
		lines = append(lines, "Result: check details")
		return strings.Join(lines, "\n")
	}
	rows := mapRowsValue(resp.Result["entries"])
	if len(rows) == 0 {
		rows = mapRowsValue(resp.Result["plugins"])
	}
	if len(rows) == 0 {
		detail := strings.TrimSpace(fmt.Sprint(resp.Result["message"]))
		if detail == "" || detail == "<nil>" {
			detail = "no candidates; scan plugins or build the semantic library first"
		}
		lines = append(lines, "- semantic search: ok, no candidates ("+detail+")")
	} else {
		names := make([]string, 0, len(rows))
		for i, row := range rows {
			if i >= 5 {
				break
			}
			name := firstNonEmptyText(row, "name", "descriptive_name", "plugin_path")
			typ := firstNonEmptyText(row, "primary_type")
			if typ != "" {
				name += " [" + typ + "]"
			}
			names = append(names, name)
		}
		lines = append(lines, "- semantic search: ok candidates="+strings.Join(names, ", "))
	}
	if routeOK && searchOK {
		lines = append(lines, "Result: ok")
	} else {
		lines = append(lines, "Result: check details")
	}
	return strings.Join(lines, "\n")
}

func (s *Server) runWorkspaceRollbackSmoke(ctx context.Context) (bool, string) {
	writeResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "workspace.write_file",
		Args:      map[string]any{"root_id": "agent", "path": "tmp/rollback_smoke.txt", "content": "hello vit", "overwrite": true},
		Confirmed: true,
		Source:    "chat_smoke",
	})
	if err != nil || writeResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), writeResp.Error, "write_file failed")
	}
	applyResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "workspace.apply_edit",
		Args:      map[string]any{"root_id": "agent", "path": "tmp/rollback_smoke.txt", "old_text": "vit", "new_text": "history"},
		Confirmed: true,
		Source:    "chat_smoke",
	})
	if err != nil || applyResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), applyResp.Error, "apply_edit failed")
	}
	rollbackResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": applyResp.AgentActionID},
		Confirmed: true,
		Source:    "chat_smoke",
	})
	if err != nil || rollbackResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), rollbackResp.Error, "rollback failed")
	}
	readResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:   "workspace.read_file",
		Args:   map[string]any{"root_id": "agent", "path": "tmp/rollback_smoke.txt"},
		Source: "chat_smoke",
	})
	if err != nil || readResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), readResp.Error, "read_file failed")
	}
	content := strings.TrimSpace(fmt.Sprint(readResp.Result["content"]))
	if content != "hello vit" {
		return false, "expected content 'hello vit', got " + strconv.Quote(content)
	}
	return true, "temp file restored"
}

func (s *Server) runDAWRollbackSmoke(ctx context.Context, chatContext map[string]any) (bool, string) {
	trackID, currentMute, ok := s.rollbackSmokeTrack(ctx, chatContext)
	if !ok {
		return true, "skipped; select a track to include DAW undo"
	}
	targetMute := !currentMute
	muteResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "track.mute",
		Args:      map[string]any{"track_id": trackID, "mute": targetMute},
		Context:   chatContext,
		Confirmed: true,
		Source:    "chat_smoke",
	})
	if err != nil || muteResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), muteResp.Error, "track.mute failed")
	}
	rollbackResp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "agent.rollback_action",
		Args:      map[string]any{"target_action_id": muteResp.AgentActionID},
		Context:   chatContext,
		Confirmed: true,
		Source:    "chat_smoke",
	})
	if err != nil || rollbackResp.Status != "ok" {
		return false, firstNonEmpty(errorString(err), rollbackResp.Error, "rollback failed")
	}
	return true, "track " + trackID + " changed then undone"
}

func (s *Server) rollbackSmokeTrack(ctx context.Context, chatContext map[string]any) (string, bool, bool) {
	state := s.harness.UserStateSummary(ctx)
	trackID := strings.TrimSpace(fmt.Sprint(chatContext["selected_track_id"]))
	if trackID == "" {
		trackID = strings.TrimSpace(fmt.Sprint(chatContext["selected_clip_track_id"]))
	}
	if trackID == "" {
		trackID = strings.TrimSpace(fmt.Sprint(chatContext["selected_plugin_track_id"]))
	}
	tracks := mapRowsValue(state["tracks"])
	if trackID == "" && len(tracks) > 0 {
		trackID = strings.TrimSpace(fmt.Sprint(firstPresentValue(tracks[0], "track_id", "id")))
	}
	if trackID == "" {
		return "", false, false
	}
	for _, track := range tracks {
		id := strings.TrimSpace(fmt.Sprint(firstPresentValue(track, "track_id", "id")))
		if id != trackID {
			continue
		}
		if muted, ok := firstBool(track, "mute", "muted", "is_muted"); ok {
			return trackID, muted, true
		}
		return trackID, false, true
	}
	return trackID, false, true
}

func smokeStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

func firstPresentValue(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return ""
}

func (s *Server) devActionsReply(message string) string {
	limit := 10
	fields := strings.Fields(message)
	if len(fields) > 1 {
		if n, err := strconv.Atoi(fields[1]); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	actions := s.harness.Actions(limit)
	if len(actions) == 0 {
		return "No journal actions yet."
	}
	var lines []string
	lines = append(lines, "Recent agent actions:")
	for _, action := range actions {
		rollback := ""
		if strings.TrimSpace(action.RollbackState) != "" {
			rollback = " rollback=" + action.RollbackState
		}
		lines = append(lines, fmt.Sprintf(
			"- %s tool=%s domain=%s status=%s%s",
			action.AgentActionID,
			action.Tool,
			action.Domain,
			action.Status,
			rollback,
		))
	}
	return strings.Join(lines, "\n")
}

func parseChatToolCommand(message string) (string, map[string]any, bool, bool, error) {
	raw := strings.TrimSpace(message)
	confirmed := false
	var rest string
	switch {
	case strings.HasPrefix(raw, "/tool!"):
		confirmed = true
		rest = strings.TrimSpace(strings.TrimPrefix(raw, "/tool!"))
	case raw == "/tool" || strings.HasPrefix(raw, "/tool "):
		rest = strings.TrimSpace(strings.TrimPrefix(raw, "/tool"))
	default:
		return "", nil, false, false, nil
	}
	if rest == "" {
		return "", nil, confirmed, true, fmt.Errorf("usage: /tool[!] tool.name {optional_json_args}")
	}
	tool := rest
	argsText := ""
	if idx := strings.IndexAny(rest, " \t\r\n"); idx >= 0 {
		tool = strings.TrimSpace(rest[:idx])
		argsText = strings.TrimSpace(rest[idx:])
	}
	if tool == "" {
		return "", nil, confirmed, true, fmt.Errorf("tool name is required")
	}
	args := map[string]any{}
	if argsText != "" {
		if err := json.Unmarshal([]byte(argsText), &args); err != nil {
			return "", nil, confirmed, true, fmt.Errorf("args must be a JSON object: %w", err)
		}
	}
	return tool, args, confirmed, true, nil
}

func renderChatToolResult(resp harness.InvokeResponse) string {
	if resp.Status == "needs_confirmation" {
		return fmt.Sprintf(
			"Tool `%s` requires confirmation.\nPreview:\n%s\nRun again with `/tool! %s { ... }` to execute.",
			resp.Tool,
			resp.Preview,
			resp.Tool,
		)
	}
	payload := map[string]any{
		"status":       resp.Status,
		"tool":         resp.Tool,
		"command_name": resp.CommandName,
	}
	if resp.Error != "" {
		payload["error"] = resp.Error
	}
	if resp.Preview != "" {
		payload["preview"] = resp.Preview
	}
	if resp.Result != nil {
		payload["result"] = compactChatToolValue(resp.Result)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return fmt.Sprintf("Tool result: %s", resp.Status)
	}
	out := strings.TrimRight(buf.String(), "\n")
	if len([]rune(out)) > 3000 {
		out = string([]rune(out)[:3000]) + "\n...<truncated>"
	}
	return "Tool result:\n```json\n" + out + "\n```"
}

func compactChatToolValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			if strings.EqualFold(key, "snapshot_xml") {
				out[key] = fmt.Sprintf("<omitted %d chars>", len(fmt.Sprint(child)))
				continue
			}
			out[key] = compactChatToolValue(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		limit := len(v)
		if limit > 20 {
			limit = 20
		}
		for i := 0; i < limit; i++ {
			out = append(out, compactChatToolValue(v[i]))
		}
		if len(v) > limit {
			out = append(out, fmt.Sprintf("<omitted %d items>", len(v)-limit))
		}
		return out
	case string:
		runes := []rune(v)
		if len(runes) > 800 {
			return string(runes[:800]) + "...<truncated>"
		}
		return v
	default:
		return v
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Server) buildMessages(ctx context.Context, conversationID, userText string, requestContext map[string]any) []llm.Message {
	stateSummary := s.harness.UserStateSummary(ctx)
	catalog := s.harness.ModelCatalogSummary()
	goalID, runID := goalIDsFromContext(requestContext)
	projectPath := projectPathFromChatContext(requestContext)
	projectHistory := s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
	history := s.conversationHistory(ctx, conversationID, 12, projectHistory)
	snapshot := contextruntime.Build(contextruntime.Input{
		ConversationID:        conversationID,
		GoalID:                goalID,
		RunID:                 runID,
		UserText:              userText,
		Conversation:          history,
		Context:               requestContext,
		State:                 stateSummary,
		ProjectHistorySummary: projectHistory,
	}, contextruntime.Options{})
	_ = contextruntime.AppendDefault(snapshot)
	modeInstruction := agentModeSystemInstruction(agentModeFromContext(requestContext))
	system := fmt.Sprintf(`You are Ask Vit, the DAW assistant inside Vit-DAW.
Return ONLY JSON with this shape:
{"reply":"short user-facing answer","commands":[{"cmd":"get_project_state"}]}
You may also use tool-form commands when it is clearer:
{"reply":"short user-facing answer","commands":[{"tool":"track.mute","args":{"mute":true}}]}

Use commands only when they are clearly useful. Unknown commands are rejected by the agent harness.
For set_mute / track.mute you MUST include mute:true for muting and mute:false for unmuting.
For set_solo / track.solo you MUST include solo:true for soloing and solo:false for unsoloing.
For arm_track / track.arm you MUST include is_armed:true or is_armed:false.
For resize_clip / clip.resize you MUST include new_length and time_unit when the user asks to trim, shorten, lengthen, or change a clip duration.
For split_clip / clip.split you MUST include clip_id, track_id, split_time, and time_unit. If the user says to split at the playhead, use playhead_seconds from current_selection as split_time.
For clone_clip / clip.clone you MUST include source_clip_id, target_track_id, time_unit, and new_start. If the user asks to duplicate a clip without naming a time, place the copy immediately after the source clip.
For importing audio, prefer clip.import_media_to_track with file_path, track_id, start_time, media_type:"audio", mode:"non_destructive". If the user says "this audio", "selected library item", or "this file", use selected_library_file_path from current_selection. If the user gives an absolute path, copy it exactly into file_path. If the user asks to search the library, use asset_query and the selected/current track; the agent will search only the library Places roots.
Attachments appear in current_selection.attachments and contain only metadata: id, name, path, kind, mime, size_bytes, exists. Do not claim to analyze image or audio content from attachments. For an audio attachment the user wants placed in a track, use clip.import_media_to_track. For a MIDI attachment the user wants imported, use midi.import_file with file_path, track_id, start_time_beats:0, mode:"merge_tracks". For image/text/unknown attachments, treat them as references unless the user asks for a supported import action.
For MIDI reading, use midi.read_clip_notes with clip_id from selected_clip_id when the user asks to inspect or describe the current MIDI clip.
For MIDI writing, prefer midi.apply_note_patch with time_unit:"beats" and operations[]. Use insert_note/delete_note/move_note/resize_note/transpose_note/set_velocity/quantize_region/replace_region. MIDI write commands require confirmation.
If the user asks to create or add a MIDI clip, use midi.create_clip or midi.insert_clip with selected_track_id and time_unit:"beats".
If the user asks to insert notes, a melody, a chord, or a drum pattern into the current MIDI clip, you MUST return a midi.apply_note_patch command; do not merely say it needs confirmation.
If no MIDI clip is selected but the user asks to write notes into a MIDI clip on the current track, use selected_track_id and let the harness resolve the only clip on that track; if no clip exists, first create one with midi.create_clip.
Commands marked confirm require user preview/confirmation. Commands marked undoable can run directly when the target is unambiguous.
Do not proactively emit version.checkpoint for ordinary writes; VitAgent creates the automatic Project History safety checkpoint before the first mutating command in a goal. Use version.* commands only for explicit history, branch, worktree, restore, checkout, or checkpoint requests.
Do not invent track_id or clip_id. Use IDs from the DAW state below.
Use stable IDs only inside commands. User-facing replies should use track names, clip names, or plain musical descriptions; do not show track_id, clip_id, plugin_id, or agent_action_id unless the user explicitly asks for technical details.
The DAW state below intentionally hides internal Tracktion tracks such as arranger/chord/marker/tempo/master. Treat tracks[] as the user-visible editable track list.
When the user says "first track", use tracks[0].track_id / user_track_index=1, not the lowest engine track ID.
Never use or mention hidden engine/internal track IDs that are not present in Current DAW state tracks[].
get_project_state and list_tracks results are sanitized for Ask Vit; they are for user-visible DAW work, not raw engine inspection.
Selected DAW context appears as current_selection in the Context snapshot and comes from the Godot UI. When the user says "this track", "current track", or "selected track", use selected_track_id if it is present and it appears in daw_state_summary.tracks[].
When the user says "this clip", "current clip", or "selected clip", use selected_clip_id or selected_clip_ids from current_selection. If no clip is selected and multiple clips exist, ask the user to select or name one instead of guessing.
When the user says "at the playhead", use playhead_seconds from current_selection.
When importing audio and no target track is named, use selected_track_id as the target. If selected_track_id is absent and multiple tracks exist, ask the user which track to import into.
If commands is non-empty, keep reply as a short internal intent summary. VitAgent will replace it with the final user-facing result after execution, so do not rely on "about to" wording as the final answer.

For plugin loading/grabber setup requests such as loading TDR Nova, finding an EQ/compressor, or loading a plugin and grabbing useful controls, use the special chat workflow command {"cmd":"plugin_grabber_load_and_get_params","track_id":"...","plugin_query":"TDR Nova","intent":"short user intent"}. This workflow searches indexed plugins, asks for confirmation before loading a rack node, then reads parameters after the load succeeds. Do not use instantiate_plugin for these requests; instantiate_plugin requires an exact plugin_path and bypasses the rack grabber workflow.
For project-scoped plugin grabber learning requests such as learning a plugin, saving quick controls, grouping plugin parameters, or improving plugin control names on an already loaded/selected plugin, use the special chat workflow command {"cmd":"plugin_grabber_learn_project_profile","track_id":"...","plugin_id":"...","intent":"short user intent"}. This workflow is agent-side: it first reads full parameters, asks AI for a profile patch, validates parameter IDs, then asks the user to confirm before saving. Do not use it for ordinary parameter value changes.
For plugin grabber explanation, summary, context pack, or "explain controls" requests on an already loaded/selected plugin, use the special read-only workflow command {"cmd":"plugin_grabber_explain_controls","track_id":"...","plugin_id":"...","intent":"short user intent"}. This workflow reads full parameters, then returns a compact context pack with quick controls, groups, roles, and full-parameter access hints. It does not filter or save parameters.

Mode instruction:
%s

Available DAW command catalog:
%s

Context snapshot JSON:
%s`, modeInstruction, catalog, snapshot.JSON())

	out := []llm.Message{{Role: "system", Content: system}}
	out = append(out, llm.Message{Role: "user", Content: userText})
	return out
}

func (s *Server) remember(conversationID, userText, assistantText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conversations[conversationID] = append(s.conversations[conversationID],
		llm.Message{Role: "user", Content: userText},
		llm.Message{Role: "assistant", Content: assistantText},
	)
}

func (s *Server) conversationHistory(ctx context.Context, conversationID string, limit int, projectHistory map[string]any) []llm.Message {
	s.mu.Lock()
	history := append([]llm.Message(nil), s.conversations[conversationID]...)
	s.mu.Unlock()
	if len(history) == 0 {
		history = llmMessagesFromProjectHistory(projectHistory)
	}
	if len(history) == 0 && s != nil && s.harness != nil {
		history = llmMessagesFromProjectHistory(s.harness.ProjectHistorySummary(ctx, ""))
	}
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}
	return history
}

func llmMessagesFromProjectHistory(projectHistory map[string]any) []llm.Message {
	if len(projectHistory) == 0 {
		return nil
	}
	rows := dictionaryRowsFromAny(projectHistory["conversation_messages"])
	out := make([]llm.Message, 0, len(rows))
	for _, row := range rows {
		role := strings.TrimSpace(fmt.Sprint(row["role"]))
		content := strings.TrimSpace(fmt.Sprint(row["content"]))
		if content == "" || content == "<nil>" {
			continue
		}
		switch role {
		case "user", "assistant":
			out = append(out, llm.Message{Role: role, Content: content})
		}
	}
	return out
}

func dictionaryRowsFromAny(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []history.ConversationMessage:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			out = append(out, map[string]any{
				"role":       item.Role,
				"content":    item.Content,
				"node_id":    item.NodeID,
				"commit_id":  item.CommitID,
				"branch":     item.Branch,
				"created_at": item.CreatedAt,
			})
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func (s *Server) confirmationPreview(ctx context.Context, decisions []policy.Decision, requestContext map[string]any) (string, error) {
	if len(decisions) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(decisions))
	for _, d := range decisions {
		if preview, err := s.harness.PreviewResolvedCommand(ctx, d.Command, requestContext); err == nil && strings.TrimSpace(preview) != "" {
			parts = append(parts, preview)
			continue
		} else if err != nil {
			return "", err
		}
		return policy.Preview(decisions), nil
	}
	if confirmationWillCreateBaseline(decisions, requestContext) {
		goalID, _ := goalIDsFromContext(requestContext)
		goal := s.harness.RuntimeStatus(goalID)
		if goal.ProjectHistory != nil && goal.ProjectHistory.BaselineCreated && strings.TrimSpace(goal.ProjectHistory.BaselineCommitID) != "" {
			return strings.Join(parts, "\n"), nil
		}
		parts = append([]string{"After confirmation, VitAgent will create a Project History safety checkpoint first."}, parts...)
	}
	return strings.Join(parts, "\n"), nil
}

func confirmationWillCreateBaseline(decisions []policy.Decision, requestContext map[string]any) bool {
	goalID, _ := goalIDsFromContext(requestContext)
	if strings.TrimSpace(goalID) == "" {
		return false
	}
	catalog := tools.DefaultCatalog()
	for _, decision := range decisions {
		name := strings.TrimSpace(decision.Name)
		if name != "" {
			if spec, ok := catalog.LookupCommand(name); ok && spec.MutatesProject && spec.Category != "project_history" {
				return true
			}
		}
		toolName := strings.TrimSpace(fmt.Sprint(decision.Command["tool"]))
		if toolName != "" && toolName != "<nil>" {
			if spec, ok := catalog.LookupTool(toolName); ok && spec.MutatesProject && spec.Category != "project_history" {
				return true
			}
		}
	}
	return false
}

func (s *Server) executeDecisions(ctx context.Context, decisions []policy.Decision, confirmed bool, requestContext map[string]any) ([]map[string]any, error) {
	replies := make([]map[string]any, 0, len(decisions))
	for _, d := range decisions {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Command:   d.Command,
			Context:   requestContext,
			Source:    "chat",
			Confirmed: confirmed,
		})
		if err != nil {
			return replies, err
		}
		if resp.Status == "needs_confirmation" {
			return replies, fmt.Errorf("command requires confirmation: %s", resp.CommandName)
		}
		replies = append(replies, map[string]any{
			"status":          resp.Status,
			"agent_action_id": resp.AgentActionID,
			"tool":            resp.Tool,
			"command_name":    resp.CommandName,
			"risk_level":      resp.RiskLevel,
			"undo_label":      resp.UndoLabel,
			"result":          resp.Result,
			"error":           resp.Error,
		})
	}
	return replies, nil
}

func agentContextForPrompt(requestContext map[string]any) string {
	safe := map[string]any{}
	for _, key := range []string{"selected_track_id", "selected_track_name", "selected_scene_track_id", "selected_clip_id", "selected_clip_track_id", "selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_plugin_source", "playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "selected_library_file_path", "selected_library_item_name", "selected_library_kind", "library_search_query"} {
		if value := strings.TrimSpace(fmt.Sprint(requestContext[key])); value != "" && value != "<nil>" {
			safe[key] = value
		}
	}
	if ids := contextStringSlice(requestContext["selected_clip_ids"]); len(ids) > 0 {
		safe["selected_clip_ids"] = ids
	}
	if places := contextStringSlice(requestContext["library_places"]); len(places) > 0 {
		safe["library_places"] = places
	}
	if attachments := contextAttachmentRows(requestContext["attachments"]); len(attachments) > 0 {
		safe["attachments"] = attachments
	}
	if len(safe) == 0 {
		return "{}"
	}
	raw, _ := json.MarshalIndent(safe, "", "  ")
	return string(raw)
}

func contextStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if s := strings.TrimSpace(it); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(fmt.Sprint(it))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func contextAttachmentRows(v any) []map[string]any {
	switch rows := v.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneContext(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func agentModeFromContext(ctx map[string]any) string {
	if ctx == nil {
		return agentModeDefault
	}
	for _, key := range []string{"agent_mode", "mode"} {
		if mode := agentModeFromString(fmt.Sprint(ctx[key])); mode != agentModeDefault {
			return mode
		}
	}
	if contextBool(ctx, "use_goal_runner") {
		return agentModeGoal
	}
	return agentModeDefault
}

func agentModeFromString(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case agentModePlan, "planner", "planning", "read_only_plan", "readonly_plan", "read-only-plan":
		return agentModePlan
	case agentModeGoal, "goals", "goal_mode", "goalmode", "autonomous", "long_goal", "long-goal":
		return agentModeGoal
	default:
		return agentModeDefault
	}
}

func agentModeSystemInstruction(mode string) string {
	switch agentModeFromString(mode) {
	case agentModePlan:
		return "Ask Vit is in Plan mode. You may inspect, analyze, explain, search, and draft steps, but you must not directly modify the DAW project file or live DAW state. Emit only read-only commands. If the user asks for a change, describe the plan and leave mutating commands empty."
	case agentModeGoal:
		return "Ask Vit is in Goal mode. Long-running autonomous DAW edits are handled by AgentLoop goal policy; preserve active worktree, branch, node, goal, and run context."
	default:
		return "Ask Vit is in default mode. Handle the current user turn with a bounded normal tool/action flow; do not treat this as a long-running autonomous goal unless explicitly requested."
	}
}

func contextWithGoal(in map[string]any, goalID, runID string) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if strings.TrimSpace(goalID) != "" {
		out["goal_id"] = strings.TrimSpace(goalID)
	}
	if strings.TrimSpace(runID) != "" {
		out["run_id"] = strings.TrimSpace(runID)
	}
	return out
}

func goalIDsFromContext(in map[string]any) (string, string) {
	if in == nil {
		return "", ""
	}
	return strings.TrimSpace(fmt.Sprint(in["goal_id"])), strings.TrimSpace(fmt.Sprint(in["run_id"]))
}

func contextWithUserMessage(in map[string]any, message string) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if strings.TrimSpace(message) != "" {
		out["user_message"] = strings.TrimSpace(message)
	}
	return out
}

func contextWithAttachments(in map[string]any, attachments []Attachment) map[string]any {
	rows := summarizeAttachments(attachments)
	if len(rows) == 0 {
		return in
	}
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	out["attachments"] = rows
	if len(rows) == 1 {
		out["attachment"] = rows[0]
	}
	for _, row := range rows {
		kind := strings.TrimSpace(fmt.Sprint(row["kind"]))
		path := strings.TrimSpace(fmt.Sprint(row["path"]))
		if path == "" {
			continue
		}
		if (kind == "audio" || kind == "midi" || kind == "plugin") && isEmptyContextValue(out, "selected_attachment_file_path") {
			out["selected_attachment_file_path"] = path
			out["selected_attachment_kind"] = kind
			out["selected_attachment_name"] = strings.TrimSpace(fmt.Sprint(row["name"]))
		}
		if (kind == "audio" || kind == "midi") && isEmptyContextValue(out, "attachment_import_file_path") {
			out["attachment_import_file_path"] = path
			out["attachment_import_kind"] = kind
			out["attachment_import_name"] = strings.TrimSpace(fmt.Sprint(row["name"]))
		}
	}
	return out
}

func summarizeAttachments(attachments []Attachment) []map[string]any {
	out := make([]map[string]any, 0, len(attachments))
	for i, att := range attachments {
		row := summarizeAttachment(att, i)
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	return out
}

func summarizeAttachment(att Attachment, index int) map[string]any {
	path := cleanAttachmentPath(att.Path)
	name := strings.TrimSpace(att.Name)
	if name == "" && path != "" {
		name = filepath.Base(path)
	}
	if path == "" && name == "" {
		return nil
	}
	kind := resolveAttachmentKind(firstNonEmpty(att.Kind, path, name))
	row := map[string]any{
		"id":     firstNonEmpty(strings.TrimSpace(att.ID), fmt.Sprintf("att_%d", index+1)),
		"name":   name,
		"path":   path,
		"kind":   kind,
		"exists": att.Exists,
	}
	if mime := strings.TrimSpace(att.MIME); mime != "" {
		row["mime"] = mime
	}
	if att.SizeBytes > 0 {
		row["size_bytes"] = att.SizeBytes
	}
	if path != "" {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			row["exists"] = true
			row["size_bytes"] = st.Size()
		} else if _, ok := row["exists"]; !ok {
			row["exists"] = false
		}
	}
	return row
}

func cleanAttachmentPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.FromSlash(path)
	return filepath.Clean(path)
}

func resolveAttachmentKind(raw string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(raw)))
	switch ext {
	case ".wav", ".mp3", ".flac", ".ogg", ".oga", ".aif", ".aiff", ".m4a", ".wma":
		return "audio"
	case ".mid", ".midi":
		return "midi"
	case ".vst3", ".clap":
		return "plugin"
	case ".png", ".jpg", ".jpeg", ".webp":
		return "image"
	case ".txt", ".md", ".markdown", ".json", ".csv", ".tsv", ".log", ".rtf":
		return "text"
	}
	kind := strings.ToLower(strings.TrimSpace(raw))
	switch kind {
	case "audio", "midi", "plugin", "image", "text", "unknown":
		return kind
	default:
		return "unknown"
	}
}

func isEmptyContextValue(ctx map[string]any, key string) bool {
	if ctx == nil {
		return true
	}
	value, ok := ctx[key]
	if !ok {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(value))
	return s == "" || s == "<nil>"
}

func projectPathFromChatContext(ctx map[string]any) string {
	if len(ctx) == 0 {
		return ""
	}
	for _, key := range []string{"project_path", "current_project_path"} {
		if value := cleanContextString(ctx[key]); value != "" {
			return value
		}
	}
	if nested, ok := ctx["project_history"].(map[string]any); ok {
		for _, key := range []string{"project_path", "current_project_path"} {
			if value := cleanContextString(nested[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func cleanContextString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func parseModelEnvelope(raw string) modelEnvelope {
	var env modelEnvelope
	text := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(text), &env); err == nil {
		return env
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		_ = json.Unmarshal([]byte(text[start:end+1]), &env)
	}
	if strings.TrimSpace(env.Reply) == "" && len(env.Commands) == 0 {
		env.Reply = raw
	}
	return env
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
