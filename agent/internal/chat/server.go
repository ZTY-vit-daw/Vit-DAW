package chat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/shadow"
)

type Server struct {
	kernel  *kernel.Client
	shadow  *shadow.Project
	llm     *llm.Client
	logger  *logx.Logger
	harness *harness.Harness

	mu            sync.Mutex
	conversations map[string][]llm.Message
	pending       map[string]PendingPlan
}

type PendingPlan struct {
	ID        string            `json:"id"`
	CreatedAt time.Time         `json:"created_at"`
	Decisions []policy.Decision `json:"decisions"`
	Preview   string            `json:"preview"`
}

type ChatRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

type ChatResponse struct {
	ConversationID      string            `json:"conversation_id"`
	Reply               string            `json:"reply"`
	NeedsConfirmation   bool              `json:"needs_confirmation"`
	PlanID              string            `json:"plan_id,omitempty"`
	Preview             string            `json:"preview,omitempty"`
	Commands            []policy.Decision `json:"commands,omitempty"`
	ExecutedKernelReply []map[string]any  `json:"executed_kernel_reply,omitempty"`
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

func New(kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Server {
	return &Server{
		kernel:        kernelClient,
		shadow:        shadowProject,
		llm:           &llm.Client{},
		logger:        logger,
		harness:       harness.New(kernelClient, shadowProject, logger),
		conversations: map[string][]llm.Message{},
		pending:       map[string]PendingPlan{},
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
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"shadow":          s.shadow.Summary(),
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
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = "chat_" + randomID()
	}

	cfg, cfgPath, err := config.Load()
	if err != nil {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          "VitAgent 已启动，但读取 AI 配置失败：" + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	if !cfg.Complete() {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          fmt.Sprintf("VitAgent 已启动，但还没有完整 AI 配置。请在 %s 写入 baseUrl、apiKey、defaultModel，或设置 VIT_AGENT_LLM_* 环境变量。", cfgPath),
			Error:          "llm_config_incomplete",
		})
		return
	}

	messages := s.buildMessages(conversationID, req.Message)
	raw, err := s.llm.Complete(r.Context(), cfg, messages)
	if err != nil {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          "Ask Vit 调用 AI 失败：" + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	env := parseModelEnvelope(raw)
	if strings.TrimSpace(env.Reply) == "" {
		env.Reply = strings.TrimSpace(raw)
	}

	decisions := policy.Analyze(env.Commands)
	s.remember(conversationID, req.Message, env.Reply)

	if len(decisions) == 0 {
		writeJSON(w, http.StatusOK, ChatResponse{ConversationID: conversationID, Reply: env.Reply})
		return
	}
	if policy.NeedsConfirmation(decisions) {
		plan := PendingPlan{
			ID:        "plan_" + randomID(),
			CreatedAt: time.Now(),
			Decisions: decisions,
			Preview:   policy.Preview(decisions),
		}
		s.mu.Lock()
		s.pending[plan.ID] = plan
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID:    conversationID,
			Reply:             env.Reply + "\n\n我准备执行下面这些 DAW 操作，请确认后再继续。",
			NeedsConfirmation: true,
			PlanID:            plan.ID,
			Preview:           plan.Preview,
			Commands:          decisions,
		})
		return
	}

	replies, execErr := s.executeDecisions(r.Context(), decisions, false)
	resp := ChatResponse{
		ConversationID:      conversationID,
		Reply:               env.Reply,
		Commands:            decisions,
		ExecutedKernelReply: replies,
	}
	if execErr != nil {
		resp.Error = execErr.Error()
		resp.Reply += "\n\n低风险命令执行时出错：" + execErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
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
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if decision != "approve" && decision != "allow" && decision != "confirm" && decision != "yes" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "plan cancelled", "plan_id": planID})
		return
	}
	replies, err := s.executeDecisions(r.Context(), plan.Decisions, true)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": err.Error(),
			"plan_id": planID,
			"replies": replies,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": "plan executed",
		"plan_id": planID,
		"replies": replies,
	})
}

func (s *Server) buildMessages(conversationID, userText string) []llm.Message {
	state, _ := json.MarshalIndent(s.shadow.Summary(), "", "  ")
	catalog := s.harness.ModelCatalogSummary()
	system := fmt.Sprintf(`You are Ask Vit, the DAW assistant inside Vit-DAW.
Return ONLY JSON with this shape:
{"reply":"short user-facing answer","commands":[{"cmd":"get_project_state"}]}

Use commands only when they are clearly useful. Unknown commands are rejected by the agent harness.
Commands marked confirm require user preview/confirmation. Commands marked undoable can run directly when the target is unambiguous.
Do not invent track_id or clip_id. Use IDs from the DAW state below.

Available DAW command catalog:
%s

Current DAW state:
%s`, catalog, string(state))

	s.mu.Lock()
	history := append([]llm.Message(nil), s.conversations[conversationID]...)
	s.mu.Unlock()

	out := []llm.Message{{Role: "system", Content: system}}
	if len(history) > 8 {
		history = history[len(history)-8:]
	}
	out = append(out, history...)
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

func (s *Server) executeDecisions(ctx context.Context, decisions []policy.Decision, confirmed bool) ([]map[string]any, error) {
	replies := make([]map[string]any, 0, len(decisions))
	for _, d := range decisions {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Command:   d.Command,
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
