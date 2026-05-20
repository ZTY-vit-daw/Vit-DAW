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
	Context   map[string]any    `json:"context,omitempty"`
	Preview   string            `json:"preview"`
}

type ChatRequest struct {
	ConversationID string         `json:"conversation_id"`
	Message        string         `json:"message"`
	Context        map[string]any `json:"context,omitempty"`
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
		"shadow":          s.harness.StateSummary(r.Context()),
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

	messages := s.buildMessages(r.Context(), conversationID, req.Message, req.Context)
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

	chatContext := contextWithUserMessage(req.Context, req.Message)
	if len(env.Commands) == 0 {
		env.Commands = synthesizeLocalDAWCommands(req.Message, chatContext)
	}
	decisions := policy.Analyze(env.Commands)

	if len(decisions) == 0 {
		reply := sanitizeUserReply(env.Reply, s.harness.UserStateSummary(r.Context()), req.Message)
		s.remember(conversationID, req.Message, reply)
		writeJSON(w, http.StatusOK, ChatResponse{ConversationID: conversationID, Reply: reply})
		return
	}
	if policy.NeedsConfirmation(decisions) {
		plan := PendingPlan{
			ID:        "plan_" + randomID(),
			CreatedAt: time.Now(),
			Decisions: decisions,
			Context:   chatContext,
			Preview:   policy.Preview(decisions),
		}
		s.mu.Lock()
		s.pending[plan.ID] = plan
		s.mu.Unlock()
		reply := confirmationReply(decisions)
		s.remember(conversationID, req.Message, reply)
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID:    conversationID,
			Reply:             reply,
			NeedsConfirmation: true,
			PlanID:            plan.ID,
			Preview:           plan.Preview,
			Commands:          decisions,
		})
		return
	}

	beforeState := s.harness.UserStateSummary(r.Context())
	replies, execErr := s.executeDecisions(r.Context(), decisions, false, chatContext)
	reply := executedReply(beforeState, s.harness.UserStateSummary(r.Context()), decisions, replies)
	if strings.TrimSpace(reply) == "" {
		reply = env.Reply
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
	s.remember(conversationID, req.Message, resp.Reply)
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
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "已取消。", "plan_id": planID})
		return
	}
	beforeState := s.harness.UserStateSummary(r.Context())
	replies, err := s.executeDecisions(r.Context(), plan.Decisions, true, plan.Context)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": err.Error(),
			"plan_id": planID,
			"replies": replies,
		})
		return
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(r.Context()), plan.Decisions, replies)
	if strings.TrimSpace(message) == "" {
		message = "已执行。"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": message,
		"plan_id": planID,
		"replies": replies,
	})
}

func (s *Server) buildMessages(ctx context.Context, conversationID, userText string, requestContext map[string]any) []llm.Message {
	state, _ := json.MarshalIndent(s.harness.UserStateSummary(ctx), "", "  ")
	catalog := s.harness.ModelCatalogSummary()
	selectedContext := agentContextForPrompt(requestContext)
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
For clone_clip / clip.clone you MUST include source_clip_id, target_track_id, time_unit, and new_start. If the user asks to duplicate a clip without naming a time, place the copy immediately after the source clip.
For importing audio, prefer clip.import_media_to_track with file_path, track_id, start_time, media_type:"audio", mode:"non_destructive". If the user says "this audio", "selected library item", or "资料库选中的音频", use selected_library_file_path from Selected DAW context. If the user gives an absolute path, copy it exactly into file_path. If the user asks to search the library, use asset_query and the selected/current track; the agent will search only the library Places roots.
Commands marked confirm require user preview/confirmation. Commands marked undoable can run directly when the target is unambiguous.
Do not invent track_id or clip_id. Use IDs from the DAW state below.
Use stable IDs only inside commands. User-facing replies should use track names, clip names, or plain musical descriptions; do not show track_id, clip_id, plugin_id, or agent_action_id unless the user explicitly asks for technical details.
The DAW state below intentionally hides internal Tracktion tracks such as arranger/chord/marker/tempo/master. Treat tracks[] as the user-visible editable track list.
When the user says "first track" or "第一条轨道", use tracks[0].track_id / user_track_index=1, not the lowest engine track ID.
Never use or mention hidden engine/internal track IDs that are not present in Current DAW state tracks[].
get_project_state and list_tracks results are sanitized for Ask Vit; they are for user-visible DAW work, not raw engine inspection.
Selected DAW context comes from the Godot UI. When the user says "this track", "current track", or "selected track", use selected_track_id if it is present and it appears in Current DAW state tracks[].
When the user says "this clip", "current clip", or "selected clip", use selected_clip_id or selected_clip_ids from Selected DAW context. If no clip is selected and multiple clips exist, ask the user to select or name one instead of guessing.
When importing audio and no target track is named, use selected_track_id as the target. If selected_track_id is absent and multiple tracks exist, ask the user which track to import into.
If commands is non-empty, keep reply as a short internal intent summary. VitAgent will replace it with the final user-facing result after execution, so do not rely on "about to" / "即将" wording as the final answer.

Available DAW command catalog:
%s

Selected DAW context:
%s

Current DAW state:
%s`, catalog, selectedContext, string(state))

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
	for _, key := range []string{"selected_track_id", "selected_track_name", "selected_scene_track_id", "selected_clip_id", "selected_clip_track_id", "selected_library_file_path", "selected_library_item_name", "selected_library_kind", "library_search_query"} {
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
