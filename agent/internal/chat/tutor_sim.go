package chat

// TUTOR-SIM-1: server-side three-step tutor experiment loop.
//
// This file is intentionally self-contained: the experiment schema/state
// machine lives in internal/tutorsim, while this file owns the three HTTP
// surfaces and the LLM prompt construction. A tutor conversation is handled
// before handleChat creates a goal, pending plan, semantic entry or harness
// node, so it never falls into the ordinary agent loop. Non-tutor
// conversations are untouched: the only added work on that path is a map
// lookup that misses.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/tutorsim"
)

const (
	tutorSimWorkflow = "tutor_sim"

	tutorSimStopRetry            = "tutor_sim_retry"
	tutorSimStopAdvanced         = "tutor_sim_step_advanced"
	tutorSimStopCompleted        = "tutor_sim_completed"
	tutorSimStopAlreadyCompleted = "tutor_sim_already_completed"
	tutorSimStopLLMTransient     = "tutor_sim_llm_transient"
	tutorSimStopLLMError         = "tutor_sim_llm_error"
	tutorSimStopLLMEmptyReply    = "tutor_sim_llm_empty_reply"
	tutorSimStopLLMUnavailable   = "tutor_sim_llm_unavailable"

	tutorSimFailureTransient = "transient"
	tutorSimFailurePermanent = "permanent"
	tutorSimFailureEmpty     = "empty_reply"
	tutorSimFailureConfig    = "config"
)

var errTutorSimEmptyReply = errors.New("tutor_sim: empty LLM reply")

// tutorSimCompleter is the narrow LLM seam. *llm.Client satisfies it; tests
// inject a scripted fake so the state machine and HTTP contract can be checked
// without a real model.
type tutorSimCompleter interface {
	CompleteRequest(context.Context, config.EngineConfig, llm.Request) (llm.Response, error)
}

type tutorSimRuntime struct {
	store      *tutorsim.Store
	completer  tutorSimCompleter
	loadConfig func() (config.EngineConfig, string, error)
}

func newTutorSimRuntime() *tutorSimRuntime {
	return &tutorSimRuntime{
		store:      tutorsim.NewStore(),
		loadConfig: config.Load,
	}
}

func (rt *tutorSimRuntime) config() (config.EngineConfig, string, error) {
	if rt == nil || rt.loadConfig == nil {
		return config.EngineConfig{}, "", errors.New("tutor_sim: config loader unavailable")
	}
	return rt.loadConfig()
}

func (rt *tutorSimRuntime) complete(ctx context.Context, s *Server, cfg config.EngineConfig, req llm.Request) (llm.Response, error) {
	if rt == nil {
		return llm.Response{}, errors.New("tutor_sim: runtime unavailable")
	}
	completer := rt.completer
	if completer == nil {
		if s == nil || s.llm == nil {
			return llm.Response{}, errors.New("tutor_sim: llm client unavailable")
		}
		completer = s.llm
	}
	return completer.CompleteRequest(ctx, cfg, req)
}

type tutorSimStartRequest struct {
	ConversationID string `json:"conversation_id"`
	ProjectPath    string `json:"project_path"`
}

type tutorSimStartResponse struct {
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
	Error          string `json:"error,omitempty"`
	FailureClass   string `json:"failure_class,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	ExperimentID   string `json:"experiment_id,omitempty"`
	SchemaVersion  string `json:"schema_version,omitempty"`
	ProjectPath    string `json:"project_path,omitempty"`
	SidecarPath    string `json:"sidecar_path,omitempty"`
	StepIndex      int    `json:"step_index,omitempty"`
	StepTotal      int    `json:"step_total,omitempty"`
	StepID         string `json:"step_id,omitempty"`
	Reply          string `json:"reply,omitempty"`
}

type tutorSimStatusResponse struct {
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	ExperimentID   string `json:"experiment_id,omitempty"`
	ProjectPath    string `json:"project_path,omitempty"`
	StepIndex      int    `json:"step_index,omitempty"`
	StepTotal      int    `json:"step_total,omitempty"`
	StepID         string `json:"step_id,omitempty"`
	SessionStatus  string `json:"session_status,omitempty"`
	Completed      bool   `json:"completed"`
	Attempts       int    `json:"attempts,omitempty"`
	Failures       int    `json:"failures,omitempty"`
}

// handleTutorStart implements POST /agent/tutor/start.
func (s *Server) handleTutorStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var req tutorSimStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	projectPath := normalizeTutorSimProjectPath(req.ProjectPath)
	if projectPath == "" {
		projectPath = s.currentTutorSimProjectPath(r.Context())
	}
	if projectPath == "" {
		writeJSON(w, http.StatusOK, tutorSimStartResponse{
			Status: "unavailable",
			Reason: "no_project",
		})
		return
	}
	exp, sidecarPath, err := tutorsim.LoadProject(projectPath)
	if err != nil {
		if errors.Is(err, tutorsim.ErrSidecarMissing) {
			writeJSON(w, http.StatusOK, tutorSimStartResponse{
				Status:      "unavailable",
				Reason:      "sidecar_missing",
				ProjectPath: projectPath,
				SidecarPath: sidecarPath,
			})
			return
		}
		s.logTutorSim("[tutor_sim] start invalid_experiment project=%s sidecar=%s error=%v", projectPath, sidecarPath, err)
		writeJSON(w, http.StatusUnprocessableEntity, tutorSimStartResponse{
			Status:      "error",
			Error:       err.Error(),
			ProjectPath: projectPath,
			SidecarPath: sidecarPath,
		})
		return
	}

	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = "tutor_" + randomID()
	}
	rt := s.tutorSim
	if rt == nil {
		writeJSON(w, http.StatusInternalServerError, tutorSimStartResponse{
			Status: "error",
			Error:  "tutor_sim_runtime_unavailable",
		})
		return
	}
	cfg, cfgPath, cfgErr := rt.config()
	if cfgErr != nil || !cfg.Complete() {
		errText := "llm_config_incomplete"
		if cfgErr != nil {
			errText = cfgErr.Error()
		}
		s.logTutorSim("[tutor_sim] start llm_unavailable conversation=%s config=%s error=%s", conversationID, cfgPath, errText)
		writeJSON(w, http.StatusServiceUnavailable, tutorSimStartResponse{
			Status:         "error",
			Error:          errText,
			FailureClass:   tutorSimFailureConfig,
			ConversationID: conversationID,
			ProjectPath:    projectPath,
			SidecarPath:    sidecarPath,
		})
		return
	}

	llmResp, llmErr := rt.complete(r.Context(), s, cfg, llm.Request{
		Messages:        tutorSimOpenerMessages(exp),
		MaxOutputTokens: 512,
		Metadata: llm.RequestMetadata{
			Source:         "tutor_sim",
			ConversationID: conversationID,
		},
	})
	if llmErr != nil {
		s.writeTutorSimLLMFailure(w, http.StatusBadGateway, conversationID, projectPath, sidecarPath, exp, llmErr)
		return
	}
	reply := strings.TrimSpace(llmResp.Text)
	if reply == "" {
		s.writeTutorSimLLMFailure(w, http.StatusBadGateway, conversationID, projectPath, sidecarPath, exp, errTutorSimEmptyReply)
		return
	}
	snapshot, err := rt.store.Start(conversationID, projectPath, exp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, tutorSimStartResponse{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}
	s.logTutorSim("[tutor_sim] start conversation=%s experiment=%s project=%s step=%d/%d", conversationID, exp.ID, projectPath, snapshot.StepIndex, snapshot.StepTotal)
	writeJSON(w, http.StatusOK, tutorSimStartResponse{
		Status:         "ok",
		ConversationID: conversationID,
		ExperimentID:   snapshot.ExperimentID,
		SchemaVersion:  tutorsim.SchemaVersion,
		ProjectPath:    projectPath,
		SidecarPath:    sidecarPath,
		StepIndex:      snapshot.StepIndex,
		StepTotal:      snapshot.StepTotal,
		StepID:         snapshot.StepID,
		Reply:          reply,
	})
}

// handleTutorStatus implements GET /agent/tutor/status.
func (s *Server) handleTutorStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "conversation_id is required"})
		return
	}
	rt := s.tutorSim
	if rt == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": "tutor_sim_runtime_unavailable"})
		return
	}
	snapshot, ok := rt.store.Status(conversationID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "unknown_tutor_session"})
		return
	}
	writeJSON(w, http.StatusOK, tutorSimStatusResponse{
		Status:         "ok",
		ConversationID: snapshot.ConversationID,
		ExperimentID:   snapshot.ExperimentID,
		ProjectPath:    snapshot.ProjectPath,
		StepIndex:      snapshot.StepIndex,
		StepTotal:      snapshot.StepTotal,
		StepID:         snapshot.StepID,
		SessionStatus:  string(snapshot.Status),
		Completed:      snapshot.Completed,
		Attempts:       snapshot.Attempts,
		Failures:       snapshot.Failures,
	})
}

// handleTutorSimChatTurn intercepts POST /agent/chat for a conversation that
// belongs to an in-memory tutor session. It returns true when the tutor loop
// handled the request (and wrote the response); false leaves the ordinary chat
// path completely intact.
func (s *Server) handleTutorSimChatTurn(w http.ResponseWriter, r *http.Request, conversationID, message string) bool {
	if s == nil {
		return false
	}
	rt := s.tutorSim
	if rt == nil {
		return false
	}
	verdict, exp, ok := rt.store.Submit(conversationID, message)
	if !ok {
		return false
	}
	resp := ChatResponse{
		ConversationID: conversationID,
		Workflow:       tutorSimWorkflow,
	}
	data := tutorSimVerdictData(exp, verdict)
	if verdict.AlreadyCompleted {
		resp.Reply = "本实验已完成，不再有新的步骤。如需重做，请重新开始实验。"
		resp.WorkflowData = data
		resp.StopReason = tutorSimStopAlreadyCompleted
		resp.GoalStatus = string(agentruntime.StatusCompleted)
		writeJSON(w, http.StatusOK, resp)
		return true
	}

	cfg, cfgPath, cfgErr := rt.config()
	if cfgErr != nil || !cfg.Complete() {
		errText := "llm_config_incomplete"
		if cfgErr != nil {
			errText = cfgErr.Error()
		}
		resp = tutorSimFailureResponse(resp, data, tutorSimFailureConfig, tutorSimStopLLMUnavailable, errText, verdict)
		resp.Error = errText
		s.logTutorSim("[tutor_sim] turn llm_unavailable conversation=%s config=%s error=%s", conversationID, cfgPath, errText)
		writeJSON(w, http.StatusOK, resp)
		return true
	}

	llmResp, llmErr := rt.complete(r.Context(), s, cfg, llm.Request{
		Messages:        tutorSimTurnMessages(exp, verdict, message),
		MaxOutputTokens: 512,
		Metadata: llm.RequestMetadata{
			Source:         "tutor_sim",
			ConversationID: conversationID,
		},
	})
	if llmErr != nil {
		class := tutorSimFailureClass(llmErr)
		resp = tutorSimFailureResponse(resp, data, class, tutorSimStopReasonForFailure(class), llmErr.Error(), verdict)
		resp.Error = llmErr.Error()
		s.logTutorSim("[tutor_sim] turn llm_failure conversation=%s class=%s step=%d/%d completed=%t error=%v", conversationID, class, verdict.StepIndex, verdict.StepTotal, verdict.Completed, llmErr)
		writeJSON(w, http.StatusOK, resp)
		return true
	}
	reply := strings.TrimSpace(llmResp.Text)
	if reply == "" {
		class := tutorSimFailureEmpty
		resp = tutorSimFailureResponse(resp, data, class, tutorSimStopReasonForFailure(class), errTutorSimEmptyReply.Error(), verdict)
		resp.Error = errTutorSimEmptyReply.Error()
		s.logTutorSim("[tutor_sim] turn empty_reply conversation=%s step=%d/%d", conversationID, verdict.StepIndex, verdict.StepTotal)
		writeJSON(w, http.StatusOK, resp)
		return true
	}
	resp.Reply = reply
	resp.WorkflowData = data
	resp.StopReason = tutorSimStopReasonForVerdict(verdict)
	if verdict.Completed {
		resp.GoalStatus = string(agentruntime.StatusCompleted)
	}
	s.logTutorSim("[tutor_sim] turn conversation=%s step=%d/%d passed=%t completed=%t stop=%s", conversationID, verdict.StepIndex, verdict.StepTotal, verdict.Passed, verdict.Completed, resp.StopReason)
	writeJSON(w, http.StatusOK, resp)
	return true
}

func (s *Server) writeTutorSimLLMFailure(w http.ResponseWriter, status int, conversationID, projectPath, sidecarPath string, exp *tutorsim.Experiment, err error) {
	class := tutorSimFailureClass(err)
	s.logTutorSim("[tutor_sim] start llm_failure conversation=%s class=%s experiment=%s error=%v", conversationID, class, exp.ID, err)
	writeJSON(w, status, tutorSimStartResponse{
		Status:         "error",
		Error:          err.Error(),
		FailureClass:   class,
		ConversationID: conversationID,
		ExperimentID:   exp.ID,
		SchemaVersion:  tutorsim.SchemaVersion,
		ProjectPath:    projectPath,
		SidecarPath:    sidecarPath,
	})
}

func tutorSimFailureClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errTutorSimEmptyReply):
		return tutorSimFailureEmpty
	// Reuse the package-wide transient-service classifier so tutor failures use
	// the same marker set as the free-state loop.
	case freeStateTransientServiceError(err.Error()):
		return tutorSimFailureTransient
	default:
		return tutorSimFailurePermanent
	}
}

func tutorSimStopReasonForFailure(class string) string {
	switch class {
	case tutorSimFailureTransient:
		return tutorSimStopLLMTransient
	case tutorSimFailureEmpty:
		return tutorSimStopLLMEmptyReply
	default:
		return tutorSimStopLLMError
	}
}

func tutorSimStopReasonForVerdict(verdict tutorsim.Verdict) string {
	switch {
	case verdict.Completed:
		return tutorSimStopCompleted
	case verdict.Passed:
		return tutorSimStopAdvanced
	default:
		return tutorSimStopRetry
	}
}

func tutorSimVerdictData(exp *tutorsim.Experiment, verdict tutorsim.Verdict) map[string]any {
	return map[string]any{
		"schema_version": tutorsim.SchemaVersion,
		"experiment_id":  exp.ID,
		"step_index":     verdict.StepIndex,
		"step_total":     verdict.StepTotal,
		"step_id":        verdict.StepID,
		"passed":         verdict.Passed,
		"completed":      verdict.Completed,
		"attempts":       verdict.Attempts,
		"failures":       verdict.Failures,
	}
}

func tutorSimFailureResponse(base ChatResponse, data map[string]any, class, stopReason, errText string, verdict tutorsim.Verdict) ChatResponse {
	data["failure_class"] = class
	data["failure"] = errText
	base.Reply = tutorSimFailureReply(verdict, class)
	base.WorkflowData = data
	base.StopReason = stopReason
	if verdict.Completed {
		base.GoalStatus = string(agentruntime.StatusCompleted)
	}
	return base
}

// tutorSimFailureReply is an explicit failure disclosure, never a fabricated
// model reply. The server-side verdict recorded in workflow_data stays
// authoritative.
func tutorSimFailureReply(verdict tutorsim.Verdict, class string) string {
	state := fmt.Sprintf("第 %d/%d 步", verdict.StepIndex, verdict.StepTotal)
	switch {
	case verdict.Completed:
		return fmt.Sprintf("导师回复生成失败（%s）；服务器已记录实验完成。", class)
	case verdict.Passed:
		return fmt.Sprintf("导师回复生成失败（%s）；服务器已记录本步通过，当前%s。", class, state)
	default:
		return fmt.Sprintf("导师回复生成失败（%s）；本次输入未通过，仍停留在%s。", class, state)
	}
}

func tutorSimSystemPrompt(exp *tutorsim.Experiment) string {
	return strings.TrimSpace(fmt.Sprintf(`你是 Vit-DAW 导师实验里的导师。
人设：
%s

规则：
- 只围绕当前步骤引导，不替用户回答，不透露任何步骤的答案。
- 服务器独立校验用户输入；只有事件明确给出“通过/未通过”时，你才可以复述该判定。
- 回复简短（1-3 句），使用中文，不输出命令、JSON 或代码块。`, exp.Persona))
}

func tutorSimOpenerMessages(exp *tutorsim.Experiment) []llm.Message {
	total := len(exp.Steps)
	userPrompt := fmt.Sprintf(`实验 %s 开始，共 %d 步。
第 1 步任务：%s
请用你的人设开场，说明当前是第 1/%d 步，并邀请用户完成这一步。不要替用户回答，不要透露答案，不要提及服务器校验。`,
		exp.ID, total, exp.Steps[0].Ask, total)
	return []llm.Message{
		{Role: "system", Content: tutorSimSystemPrompt(exp)},
		{Role: "user", Content: userPrompt},
	}
}

func tutorSimTurnMessages(exp *tutorsim.Experiment, verdict tutorsim.Verdict, userMessage string) []llm.Message {
	var userPrompt string
	userText := strings.TrimSpace(userMessage)
	switch {
	case verdict.Completed:
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：通过（已完成第 %d/%d 步），实验完成。
通过反馈素材：%s
请简短收尾并祝贺用户；不要再给出任务或答案。`,
			userText, verdict.AnsweredStepIndex, verdict.StepTotal, verdict.AnsweredStep.Pass)
	case verdict.Passed:
		next := exp.Steps[verdict.StepIndex-1]
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：通过（已完成第 %d/%d 步）。
通过反馈素材：%s
下一步任务（第 %d/%d 步）：%s
请简短确认通过，并给出下一步任务；不要透露任何答案，不要提前展开更后面的步骤。`,
			userText, verdict.AnsweredStepIndex, verdict.StepTotal, verdict.AnsweredStep.Pass,
			verdict.StepIndex, verdict.StepTotal, next.Ask)
	default:
		userPrompt = fmt.Sprintf(`用户输入：%s
服务器判定：未通过，停留在第 %d/%d 步。
当前步骤任务：%s
重试引导素材：%s
请按重试引导回应用户；不要透露正确答案，不要进入下一步。`,
			userText, verdict.StepIndex, verdict.StepTotal, verdict.AnsweredStep.Ask, verdict.AnsweredStep.Retry)
	}
	return []llm.Message{
		{Role: "system", Content: tutorSimSystemPrompt(exp)},
		{Role: "user", Content: userPrompt},
	}
}

func normalizeTutorSimProjectPath(projectPath string) string {
	path := strings.TrimSpace(projectPath)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func (s *Server) currentTutorSimProjectPath(ctx context.Context) string {
	if s == nil {
		return ""
	}
	if s.harness != nil {
		if path, _ := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(path) != "" {
			return normalizeTutorSimProjectPath(path)
		}
	}
	return normalizeTutorSimProjectPath(s.activeWorkspacePathSnapshotForUI())
}

func (s *Server) logTutorSim(format string, args ...any) {
	if s == nil || s.logger == nil {
		return
	}
	s.logger.Info(format, args...)
}
