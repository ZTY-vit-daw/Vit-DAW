package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/browsercapture"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/macrocontrols"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/resourceintake"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/workflows/plugingrabber"
)

type Server struct {
	kernel       *kernel.Client
	shadow       *shadow.Project
	llm          *llm.Client
	logger       *logx.Logger
	harness      *harness.Harness
	artifactRoot string
	webUIRoot    string
	startedAt    time.Time

	mu                sync.Mutex
	conversations     map[string][]llm.Message
	pending           map[string]PendingPlan
	interactions      map[string]PendingInteraction
	mixSessions       map[string]MixSession
	goalContinuations map[string]agentloop.Continuation
	conversationGoals map[string]string
	uiContext         map[string]any
	events            map[string][]AgentEvent
	eventSeq          map[string]int64
	webUILogged       bool
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
	ArtifactRefs   []string       `json:"artifact_refs,omitempty"`
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
	ConversationID      string                    `json:"conversation_id"`
	GoalID              string                    `json:"goal_id,omitempty"`
	RunID               string                    `json:"run_id,omitempty"`
	Reply               string                    `json:"reply"`
	AgentMode           string                    `json:"agent_mode,omitempty"`
	AgentPlan           *AgentPlan                `json:"agent_plan,omitempty"`
	NeedsConfirmation   bool                      `json:"needs_confirmation"`
	PlanID              string                    `json:"plan_id,omitempty"`
	Preview             string                    `json:"preview,omitempty"`
	Workflow            string                    `json:"workflow,omitempty"`
	WorkflowData        map[string]any            `json:"workflow_data,omitempty"`
	PluginLearning      map[string]any            `json:"plugin_learning,omitempty"`
	MixSession          map[string]any            `json:"mix_session,omitempty"`
	InteractionRequests []AgentInteractionRequest `json:"interaction_requests,omitempty"`
	Commands            []policy.Decision         `json:"commands,omitempty"`
	ExecutedKernelReply []map[string]any          `json:"executed_kernel_reply,omitempty"`
	ProjectResultCards  []map[string]any          `json:"project_result_cards,omitempty"`
	GoalStatus          string                    `json:"goal_status,omitempty"`
	GoalSummary         string                    `json:"goal_summary,omitempty"`
	CurrentStep         string                    `json:"current_step,omitempty"`
	CompletedSteps      int                       `json:"completed_steps,omitempty"`
	StopReason          string                    `json:"stop_reason,omitempty"`
	LimitType           string                    `json:"limit_type,omitempty"`
	ProjectHistory      map[string]any            `json:"project_history,omitempty"`
	Artifacts           []artifacts.Summary       `json:"artifacts,omitempty"`
	SidePanelRequest    *SidePanelRequest         `json:"side_panel_request,omitempty"`
	Error               string                    `json:"error,omitempty"`
}

type SidePanelRequest struct {
	View         string `json:"view,omitempty"`
	Tab          string `json:"tab,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	MacroPanelID string `json:"macro_panel_id,omitempty"`
}

type ConfirmRequest struct {
	PlanID   string `json:"plan_id"`
	Decision string `json:"decision"`
}

type AgentInteractionRequest struct {
	ID             string                   `json:"id"`
	Kind           string                   `json:"kind"`
	Source         string                   `json:"source,omitempty"`
	Workflow       string                   `json:"workflow,omitempty"`
	Stage          string                   `json:"stage,omitempty"`
	Title          string                   `json:"title,omitempty"`
	Body           string                   `json:"body,omitempty"`
	Status         string                   `json:"status,omitempty"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	GoalID         string                   `json:"goal_id,omitempty"`
	RunID          string                   `json:"run_id,omitempty"`
	Questions      []AgentInteractionField  `json:"questions,omitempty"`
	Fields         []AgentInteractionField  `json:"fields,omitempty"`
	ReviewItems    []AgentInteractionReview `json:"review_items,omitempty"`
	Payload        map[string]any           `json:"payload,omitempty"`
	Actions        []AgentInteractionAction `json:"actions,omitempty"`
	PlanID         string                   `json:"plan_id,omitempty"`
	Type           string                   `json:"type,omitempty"`
	Data           map[string]any           `json:"data,omitempty"`
}

type AgentInteractionAction struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Style       string         `json:"style,omitempty"`
	Description string         `json:"description,omitempty"`
	Value       map[string]any `json:"value,omitempty"`
	Recommended bool           `json:"recommended,omitempty"`
}

type AgentInteractionField struct {
	ID          string           `json:"id"`
	Label       string           `json:"label,omitempty"`
	Kind        string           `json:"kind,omitempty"`
	Required    bool             `json:"required,omitempty"`
	Value       any              `json:"value,omitempty"`
	Options     []map[string]any `json:"options,omitempty"`
	Description string           `json:"description,omitempty"`
	Placeholder string           `json:"placeholder,omitempty"`
	Payload     map[string]any   `json:"payload,omitempty"`
}

type AgentInteractionReview struct {
	ID      string         `json:"id"`
	Title   string         `json:"title,omitempty"`
	Body    string         `json:"body,omitempty"`
	Status  string         `json:"status,omitempty"`
	Payload map[string]any `json:"payload,omitempty"`
}

type PendingInteraction struct {
	ID             string         `json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	Kind           string         `json:"kind"`
	Source         string         `json:"source,omitempty"`
	Workflow       string         `json:"workflow,omitempty"`
	Stage          string         `json:"stage,omitempty"`
	PlanID         string         `json:"plan_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	GoalID         string         `json:"goal_id,omitempty"`
	RunID          string         `json:"run_id,omitempty"`
	RequestContext map[string]any `json:"request_context,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	Type           string         `json:"type,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
}

type InteractionRespondRequest struct {
	InteractionID string         `json:"interaction_id"`
	Decision      string         `json:"decision,omitempty"`
	ActionID      string         `json:"action_id,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
}

type ConfigResponse struct {
	Status       string              `json:"status"`
	Path         string              `json:"path"`
	Config       config.EngineConfig `json:"config"`
	HasAPIKey    bool                `json:"hasApiKey"`
	RouteAPIKeys map[string]bool     `json:"routeApiKeys,omitempty"`
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
		startedAt:         time.Now(),
		conversations:     map[string][]llm.Message{},
		pending:           map[string]PendingPlan{},
		interactions:      map[string]PendingInteraction{},
		mixSessions:       map[string]MixSession{},
		goalContinuations: map[string]agentloop.Continuation{},
		conversationGoals: map[string]string{},
		uiContext:         map[string]any{},
		events:            map[string][]AgentEvent{},
		eventSeq:          map[string]int64{},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/app", s.handleApp)
	mux.HandleFunc("/app/", s.handleApp)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/agent/runtime/status", s.handleRuntimeStatus)
	mux.HandleFunc("/agent/events", s.handleAgentEvents)
	mux.HandleFunc("/agent/state", s.handleState)
	mux.HandleFunc("/agent/ui/state", s.handleUIState)
	mux.HandleFunc("/agent/ui/context", s.handleUIContext)
	mux.HandleFunc("/agent/chat", s.handleChat)
	mux.HandleFunc("/agent/confirm", s.handleConfirm)
	mux.HandleFunc("/agent/interaction/respond", s.handleInteractionRespond)
	mux.HandleFunc("/agent/config", s.handleConfig)
	mux.HandleFunc("/agent/tools", s.handleTools)
	mux.HandleFunc("/agent/actions", s.handleActions)
	mux.HandleFunc("/agent/invoke", s.handleInvoke)
	mux.HandleFunc("/agent/debug/confirmation", s.handleConfirmationDebug)
	mux.HandleFunc("/agent/artifacts/upload", s.handleArtifactUpload)
	mux.HandleFunc("/agent/artifacts", s.handleArtifacts)
	mux.HandleFunc("/agent/artifact", s.handleArtifact)
	mux.HandleFunc("/agent/artifact/reveal", s.handleArtifactReveal)
	mux.HandleFunc("/agent/artifact/file", s.handleArtifactFile)
	mux.HandleFunc("/agent/artifact/extract", s.handleArtifactExtract)
	mux.HandleFunc("/agent/browser/capture", s.handleBrowserCapture)
	mux.HandleFunc("/agent/resource/url", s.handleResourceURL)
	mux.HandleFunc("/agent/downloads/scan", s.handleDownloadsScan)
	mux.HandleFunc("/agent/downloads/watch", s.handleDownloadsWatch)
	return mux
}

func (s *Server) handleApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path == "/app" {
		http.Redirect(w, r, "/app/", http.StatusMovedPermanently)
		return
	}
	root := s.webUIRootPath()
	if root == "" {
		http.Error(w, "Ask Vit WebUI has not been built yet", http.StatusNotFound)
		return
	}
	s.logWebUIRootOnce(root)
	clean := strings.TrimPrefix(r.URL.Path, "/app/")
	if clean == "" {
		clean = "index.html"
	}
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	path := filepath.Join(root, filepath.Clean(clean))
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		http.Error(w, "invalid app path", http.StatusBadRequest)
		return
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		path = filepath.Join(root, "index.html")
	}
	http.ServeFile(w, r, path)
}

func (s *Server) logWebUIRootOnce(root string) {
	if s == nil || s.logger == nil {
		return
	}
	s.mu.Lock()
	if s.webUILogged {
		s.mu.Unlock()
		return
	}
	s.webUILogged = true
	s.mu.Unlock()
	indexPath := filepath.Join(root, "index.html")
	assetSummary := ""
	if entries, err := os.ReadDir(filepath.Join(root, "assets")); err == nil {
		var names []string
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		assetSummary = strings.Join(names, ",")
	}
	indexMod := ""
	if st, err := os.Stat(indexPath); err == nil {
		indexMod = st.ModTime().Format(time.RFC3339)
	}
	s.logger.Info("[webui] serving root=%s index_mod=%s assets=%s build_mark=mixboard-macro-hold-v18-20260614", root, indexMod, assetSummary)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "VitAgent"})
}

func (s *Server) handleConfirmationDebug(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	payload["server_received_at"] = time.Now().Format(time.RFC3339Nano)
	path := confirmationDebugLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "path": path})
}

func confirmationDebugLogPath() string {
	if envPath := strings.TrimSpace(os.Getenv("VIT_CONFIRM_DEBUG_LOG_PATH")); envPath != "" {
		return envPath
	}
	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		if strings.EqualFold(filepath.Base(dir), "bin") {
			dir = filepath.Dir(dir)
		}
		return filepath.Join(dir, "tmp", "confirmation_debug.jsonl")
	}
	return filepath.Join(os.TempDir(), "vit_confirmation_debug.jsonl")
}

func (s *Server) handleRuntimeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	statusCtx, cancel := context.WithTimeout(r.Context(), 900*time.Millisecond)
	defer cancel()
	kernelStatus := s.runtimeKernelStatus(statusCtx)
	shadowCtx, shadowCancel := context.WithTimeout(r.Context(), 900*time.Millisecond)
	defer shadowCancel()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"service":    "VitAgent",
		"pid":        os.Getpid(),
		"checked_at": time.Now().Format(time.RFC3339Nano),
		"kernel":     kernelStatus,
		"shadow":     runtimeShadowStatus(s.harness.StateSummary(shadowCtx)),
	})
}

func (s *Server) runtimeKernelStatus(ctx context.Context) map[string]any {
	out := map[string]any{
		"connected": false,
		"status":    "offline",
	}
	if s.kernel != nil {
		out["endpoint"] = s.kernel.Endpoint
	}
	if s.harness == nil || s.kernel == nil {
		out["error"] = "kernel client is nil"
		return out
	}
	started := time.Now()
	reply, _, err := s.harness.KernelSendCommand(ctx, map[string]any{"cmd": "ping"})
	out["latency_ms"] = time.Since(started).Milliseconds()
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	replyStatus := strings.TrimSpace(fmt.Sprint(reply["status"]))
	if replyStatus == "" {
		replyStatus = "ok"
	}
	out["reply_status"] = replyStatus
	if message := strings.TrimSpace(fmt.Sprint(reply["message"])); message != "" {
		out["message"] = message
	}
	connected := !strings.EqualFold(replyStatus, "error")
	out["connected"] = connected
	if connected {
		out["status"] = "online"
	} else {
		out["status"] = "error"
		out["error"] = firstNonEmpty(strings.TrimSpace(fmt.Sprint(reply["error"])), strings.TrimSpace(fmt.Sprint(reply["message"])), "kernel ping returned error")
	}
	return out
}

func runtimeShadowStatus(state map[string]any) map[string]any {
	if state == nil {
		return map[string]any{"initialized": false}
	}
	projectPath := strings.TrimSpace(fmt.Sprint(state["project_path"]))
	if projectPath == "<nil>" {
		projectPath = ""
	}
	return map[string]any{
		"initialized":      state["initialized"],
		"project_path":     projectPath,
		"track_count":      state["track_count"],
		"user_track_count": state["user_track_count"],
		"graph_revision":   state["graph_revision"],
		"last_delta_seq":   state["last_delta_seq"],
		"delta_seq_gaps":   state["delta_seq_gaps"],
	}
}

func (s *Server) handleBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	payload = mapWithArtifactScope(payload, s.artifactScopeFromArgs(r.Context(), payload))
	result, err := browsercapture.CapturePage(s.artifactStore(), payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	if artifact, ok := result["artifact"].(artifacts.Artifact); ok {
		result["artifacts"] = []artifacts.Summary{artifact.CompactSummary()}
		result["side_panel_request"] = SidePanelRequest{View: "artifact", Tab: "browser", ArtifactID: artifact.ID}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleResourceURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	payload = mapWithArtifactScope(payload, s.artifactScopeFromArgs(r.Context(), payload))
	result, err := resourceintake.RegisterURL(s.artifactStore(), payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDownloadsScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	payload = mapWithArtifactScope(payload, s.artifactScopeFromArgs(r.Context(), payload))
	result, err := resourceintake.ScanDownloads(s.artifactStore(), payload, resourceintake.ScanOptions{MinModifiedAt: s.startedAt})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDownloadsWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	result, err := resourceintake.WatchDownloads(r.Context(), payload, resourceintake.ScanOptions{MinModifiedAt: s.startedAt})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	goal := s.harness.RuntimeStatus("")
	shadowState := s.harness.StateSummary(r.Context())
	projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goal.GoalID, firstStringFromMap(shadowState, "project_path", "current_project_path"))
	agentPlan := s.activeGoalPlan(goal, projectHistory)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"shadow": shadowState,
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

func (s *Server) handleUIState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	goal := s.harness.RuntimeStatus("")
	uiContext := s.uiContextSnapshot()
	state := mergeUIContext(s.harness.UserStateSummary(r.Context()), uiContext)
	macroControls := uiMacroControls(state)
	projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goal.GoalID, firstStringFromMap(state, "project_path", "current_project_path"))
	scope := currentArtifactScope(projectHistory, state)
	items, err := s.artifactStore().List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	items = artifacts.FilterByScope(items, scope)
	items = artifacts.FilterUserVisible(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"project":         uiProjectState(state),
		"transport":       uiTransportState(state),
		"tracks":          mapRowsFromAny(state["tracks"]),
		"selected_track":  uiSelectedTrack(state),
		"selected_plugin": uiSelectedPlugin(state),
		"plugin_rack":     uiPluginRack(state),
		"macro_controls":  macroControls,
		"ui_context":      uiContext,
		"goal":            goal,
		"agent_plan":      s.activeGoalPlan(goal, projectHistory),
		"artifacts":       artifacts.Summaries(items),
		"project_history": projectHistory,
		"capabilities": map[string]any{
			"tools":            len(s.harness.Tools()),
			"direct_commands":  s.harness.DirectCommandNames(),
			"macro_controls":   macroControls,
			"macro_control_ui": map[string]any{"source": "godot_rack", "supports_reference": true, "supports_live_cards": true},
		},
	})
}

func currentArtifactScope(projectHistory map[string]any, state map[string]any) artifacts.Scope {
	scope := map[string]any{
		"project_path":      firstNonEmpty(firstStringFromMap(projectHistory, "project_path", "current_project_path"), firstStringFromMap(state, "project_path", "current_project_path")),
		"root_project_path": firstStringFromMap(projectHistory, "root_project_path"),
		"active_worktree":   firstStringFromMap(projectHistory, "active_worktree"),
		"active_branch":     firstStringFromMap(projectHistory, "active_branch"),
		"active_node_id":    firstStringFromMap(projectHistory, "active_node_id"),
	}
	scope["history_scope_key"] = artifactHistoryScopeKey(projectHistory, state)
	return artifacts.ScopeFromMap(scope)
}

func (s *Server) artifactScopeFromArgs(ctx context.Context, args map[string]any) artifacts.Scope {
	if scope := artifacts.ScopeFromMap(args); !scope.Empty() {
		return scope
	}
	projectPath := firstNonEmpty(
		cleanContextString(args["project_path"]),
		cleanContextString(args["current_project_path"]),
	)
	projectHistory := map[string]any{}
	state := map[string]any{}
	if s != nil && s.harness != nil {
		projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, "", projectPath)
		uiContext := s.uiContextSnapshot()
		state = mergeUIContext(s.harness.UserStateSummary(ctx), uiContext)
	}
	return currentArtifactScope(projectHistory, state)
}

func mapWithArtifactScope(in map[string]any, scope artifacts.Scope) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if scope.Empty() {
		return out
	}
	for key, value := range map[string]string{
		"project_path":      scope.ProjectPath,
		"root_project_path": scope.RootProjectPath,
		"active_worktree":   scope.ActiveWorktree,
		"active_branch":     scope.ActiveBranch,
		"active_node_id":    scope.ActiveNodeID,
		"history_scope_key": scope.HistoryScopeKey,
		"media_scope_key":   scope.StableKey(),
	} {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	return out
}

func artifactHistoryScopeKey(projectHistory map[string]any, state map[string]any) string {
	projectPath := firstNonEmpty(firstStringFromMap(projectHistory, "project_path", "current_project_path"), firstStringFromMap(state, "project_path", "current_project_path"))
	rootProjectPath := firstStringFromMap(projectHistory, "root_project_path")
	projectIdentity := firstNonEmpty(projectPath, rootProjectPath, "unsaved")
	return strings.Join([]string{
		projectIdentity,
		firstNonEmpty(rootProjectPath, "root"),
		firstNonEmpty(firstStringFromMap(projectHistory, "active_worktree"), "main"),
		firstNonEmpty(firstStringFromMap(projectHistory, "active_branch"), "main"),
		firstNonEmpty(firstStringFromMap(projectHistory, "active_node_id"), firstStringFromMap(projectHistory, "head"), "no-node"),
		firstNonEmpty(firstStringFromMap(projectHistory, "draft", "unsaved"), "saved"),
		firstNonEmpty(firstStringFromMap(projectHistory, "initialized"), "unknown"),
	}, "::")
}

func (s *Server) handleUIContext(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "context": s.uiContextSnapshot()})
	case http.MethodPost:
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
			return
		}
		contextPayload := sanitizeUIContext(payload)
		s.mu.Lock()
		s.uiContext = contextPayload
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "context": contextPayload})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET or POST required"})
	}
}

func (s *Server) handleArtifacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	args := map[string]any{}
	if conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id")); conversationID != "" {
		args["conversation_id"] = conversationID
	}
	for _, key := range []string{"kind", "source", "plugin_learning_session_id", "plugin_learning_stage", "artifact_schema"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			args[key] = value
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("include_internal")); raw != "" {
		args["include_internal"] = raw
	}
	for _, key := range []string{"project_path", "current_project_path", "root_project_path", "active_worktree", "active_branch", "active_node_id", "history_scope_key", "media_scope_key"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			args[key] = value
		}
	}
	args = mapWithArtifactScope(args, s.artifactScopeFromArgs(r.Context(), args))
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			args["limit"] = n
		}
	}
	result, err := artifacts.ListCommand(s.artifactStore(), args)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		result, err := artifacts.ReadCommand(s.artifactStore(), map[string]any{"id": r.URL.Query().Get("id")})
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPatch:
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
			return
		}
		if rawID := strings.TrimSpace(fmt.Sprint(payload["id"])); rawID == "" || rawID == "<nil>" {
			payload["id"] = r.URL.Query().Get("id")
		}
		result, err := artifacts.RenameCommand(s.artifactStore(), payload)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodDelete:
		result, err := artifacts.DeleteCommand(s.artifactStore(), map[string]any{"id": r.URL.Query().Get("id")})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET, PATCH, or DELETE required"})
	}
}

func (s *Server) handleArtifactExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	result, err := artifacts.ExtractCommand(s.artifactStore(), payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleArtifactReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			id = strings.TrimSpace(fmt.Sprint(payload["id"]))
		}
	}
	a, err := s.artifactStore().Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	path := strings.TrimSpace(a.Path)
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "artifact has no local file path"})
		return
	}
	if err := revealLocalPath(path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "path": path})
}

func (s *Server) handleArtifactFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	a, err := s.artifactStore().Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	path := strings.TrimSpace(a.Path)
	if path == "" {
		http.Error(w, "artifact has no file", http.StatusNotFound)
		return
	}
	if st, err := os.Stat(path); err != nil || st.IsDir() {
		http.Error(w, "artifact file is not available", http.StatusNotFound)
		return
	}
	if mt := firstNonEmpty(a.MIME, mime.TypeByExtension(filepath.Ext(path))); mt != "" {
		w.Header().Set("Content-Type", mt)
	}
	http.ServeFile(w, r, path)
}

func (s *Server) handleArtifactUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid multipart upload: " + err.Error()})
		return
	}
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "file is required"})
		return
	}
	store := s.artifactStore()
	uploadDir := filepath.Join(store.Root, "files")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
		return
	}
	conversationID := strings.TrimSpace(r.FormValue("conversation_id"))
	goalID := strings.TrimSpace(r.FormValue("goal_id"))
	runID := strings.TrimSpace(r.FormValue("run_id"))
	uploadMetadata := map[string]any{}
	for _, key := range []string{"plugin_learning_session_id", "plugin_learning_purpose", "track_id", "plugin_id", "plugin_name"} {
		if value := strings.TrimSpace(r.FormValue(key)); value != "" {
			uploadMetadata[key] = value
		}
	}
	scope := s.artifactScopeFromArgs(r.Context(), map[string]any{
		"project_path":         r.FormValue("project_path"),
		"current_project_path": r.FormValue("current_project_path"),
		"root_project_path":    r.FormValue("root_project_path"),
		"active_worktree":      r.FormValue("active_worktree"),
		"active_branch":        r.FormValue("active_branch"),
		"active_node_id":       r.FormValue("active_node_id"),
		"history_scope_key":    r.FormValue("history_scope_key"),
		"media_scope_key":      r.FormValue("media_scope_key"),
	})
	var out []artifacts.Summary
	for _, header := range files {
		summary, err := s.storeUploadedArtifact(store, uploadDir, header, conversationID, goalID, runID, scope, uploadMetadata)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		out = append(out, summary)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "artifacts": out, "root": store.Root})
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
	if resp, ok := s.invokeMixSessionEntryWorkflow(r.Context(), req); ok {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberLearningInvokeCommand(req); ok {
		cfg, _, err := config.Load()
		if err != nil {
			resp := harness.InvokeResponse{
				Status:      "error",
				Tool:        pluginGrabberLearnTool,
				CommandName: pluginGrabberLearnCommand,
				RiskLevel:   tools.RiskConfirm,
				Error:       "reading AI config failed: " + err.Error(),
			}
			writeJSON(w, http.StatusBadRequest, resp)
			return
		}
		resp, err := s.invokePluginGrabberLearningWorkflow(r.Context(), req, workflowCmd, cfg)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
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

func uiProjectState(state map[string]any) map[string]any {
	return map[string]any{
		"initialized":      state["initialized"],
		"project_path":     state["project_path"],
		"track_count":      state["track_count"],
		"user_track_count": state["user_track_count"],
		"graph_revision":   state["graph_revision"],
		"project_health":   state["project_health"],
		"observability":    state["observability"],
		"pending_job_data": state["pending_job_data"],
		"last_delta_seq":   state["last_delta_seq"],
		"delta_seq_gaps":   state["delta_seq_gaps"],
	}
}

func uiTransportState(state map[string]any) map[string]any {
	if transport := firstMapFromAny(state["transport"]); transport != nil {
		return transport
	}
	if transport := firstMapFromAny(state["transport_state"]); transport != nil {
		return transport
	}
	out := map[string]any{}
	for _, key := range []string{"playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "bpm", "tempo", "sample_rate", "is_playing", "playing", "recording"} {
		if !isEmptyContextValue(state, key) {
			out[key] = state[key]
		}
	}
	return out
}

func uiSelectedTrack(state map[string]any) map[string]any {
	tracks := mapRowsFromAny(state["tracks"])
	if len(tracks) == 0 {
		return nil
	}
	selectedID := firstStringFromMap(state, "selected_track_id", "selected_plugin_track_id", "track_id")
	selectedName := firstStringFromMap(state, "selected_track_name", "track_name")
	for _, track := range tracks {
		if selectedID != "" && firstStringFromMap(track, "track_id", "id") == selectedID {
			return track
		}
		if selectedName != "" && strings.EqualFold(firstStringFromMap(track, "track_name", "name"), selectedName) {
			return track
		}
	}
	return nil
}

func uiSelectedPlugin(state map[string]any) map[string]any {
	pluginID := firstStringFromMap(state, "selected_plugin_id", "plugin_id")
	pluginName := firstStringFromMap(state, "selected_plugin_name", "plugin_name")
	trackID := firstStringFromMap(state, "selected_plugin_track_id", "selected_track_id", "track_id")
	source := firstStringFromMap(state, "selected_plugin_source", "plugin_source")
	if pluginID == "" && pluginName == "" {
		return nil
	}
	return map[string]any{
		"track_id":    trackID,
		"plugin_id":   pluginID,
		"plugin_name": pluginName,
		"source":      source,
	}
}

func uiPluginRack(state map[string]any) map[string]any {
	selected := uiSelectedTrack(state)
	if len(selected) == 0 {
		return map[string]any{"track": nil, "plugins": []map[string]any{}, "rack": nil}
	}
	selectedPluginID := firstStringFromMap(state, "selected_plugin_id", "plugin_id")
	plugins := mapRowsFromAny(selected["plugins"])
	markedPlugins := make([]map[string]any, 0, len(plugins))
	for _, plugin := range plugins {
		row := cloneContext(plugin)
		if selectedPluginID != "" && firstStringFromMap(row, "plugin_id", "id", "plugin_item_id", "item_id") == selectedPluginID {
			row["selected"] = true
		}
		markedPlugins = append(markedPlugins, row)
	}
	return map[string]any{
		"track": map[string]any{
			"track_id":   selected["track_id"],
			"track_name": selected["track_name"],
		},
		"plugins": markedPlugins,
		"rack":    selected["rack"],
	}
}

func uiMacroControls(state map[string]any) []map[string]any {
	byID := map[string]map[string]any{}
	order := []string{}
	for _, key := range []string{"macro_controls", "active_macro_controls", "rack_control_macros", "control_macros", "macros"} {
		for _, macro := range macrocontrols.NormalizeList(state[key]) {
			macro = enrichUIMacroControlFromState(macro, state)
			macroID := firstStringFromMap(macro, "macro_id", "id", "control_id")
			if macroID == "" {
				continue
			}
			if existing, ok := byID[macroID]; ok {
				byID[macroID] = mergeUIMacroControl(existing, macro)
				continue
			}
			byID[macroID] = macro
			order = append(order, macroID)
		}
	}
	out := make([]map[string]any, 0, len(order))
	for _, macroID := range order {
		if macro := byID[macroID]; len(macro) > 0 {
			row := cloneContext(macro)
			delete(row, "_track_volume_synced")
			out = append(out, row)
		}
	}
	return out
}

func enrichUIMacroControlFromState(macro map[string]any, state map[string]any) map[string]any {
	if !uiMacroIsTrackVolume(macro) {
		return macro
	}
	out := cloneContext(macro)
	trackID := firstStringFromMap(out, "track_id", "track")
	out["control"] = "track.volume"
	if uiMacroHasDefaultRange(out) {
		out["min"] = -60.0
		out["max"] = 12.0
		if firstStringFromMap(out, "unit") == "" {
			out["unit"] = "dB"
		}
	}
	if trackID != "" {
		if volumeDB, ok := uiTrackVolumeDB(state, trackID); ok {
			out["value"] = volumeDB
			out["_track_volume_synced"] = true
		}
	}
	if len(mapRowsFromAny(out["bindings"])) == 0 && trackID != "" {
		macroID := firstStringFromMap(out, "macro_id", "id", "control_id")
		out["bindings"] = []map[string]any{{
			"binding_id": "binding_" + macroID + "_track_volume",
			"control":    "track.volume",
			"track_id":   trackID,
			"param_id":   "track.volume",
			"param_name": "Track volume",
			"target_min": out["min"],
			"target_max": out["max"],
			"unit":       out["unit"],
			"enabled":    true,
		}}
		out["binding_count"] = 1
	}
	return macrocontrols.NormalizeControl(out)
}

func uiTrackVolumeDB(state map[string]any, trackID string) (float64, bool) {
	if strings.TrimSpace(trackID) == "" {
		return 0, false
	}
	for _, track := range mapRowsFromAny(state["tracks"]) {
		if firstStringFromMap(track, "track_id", "id") != trackID {
			continue
		}
		for _, key := range []string{"volume_db", "gain_db", "fader_db", "db"} {
			if _, exists := track[key]; exists {
				return mixFloatNumber(track[key]), true
			}
		}
	}
	return 0, false
}

func uiMacroIsTrackVolume(macro map[string]any) bool {
	if firstStringFromMap(macro, "control") == "track.volume" {
		return true
	}
	probe := strings.ToLower(strings.Join([]string{
		firstStringFromMap(macro, "macro_id", "id", "control_id"),
		firstStringFromMap(macro, "role"),
		firstStringFromMap(macro, "param_id"),
	}, " "))
	return strings.Contains(probe, "track_volume") || strings.Contains(probe, "track.volume") || strings.Contains(probe, "gain_staging")
}

func mergeUIMacroControl(existing, next map[string]any) map[string]any {
	out := cloneContext(existing)
	for key, value := range next {
		out[key] = value
	}
	existingBindings := mapRowsFromAny(existing["bindings"])
	nextBindings := mapRowsFromAny(next["bindings"])
	nextHasSyncedTrackVolume := uiMacroIsTrackVolume(next) && boolValue(next["_track_volume_synced"])
	if len(existingBindings) > 0 && len(nextBindings) == 0 {
		out["bindings"] = existingBindings
		out["binding_count"] = len(existingBindings)
		if firstStringFromMap(existing, "control") != "" && firstStringFromMap(out, "control") == "" {
			out["control"] = existing["control"]
		}
		if firstStringFromMap(existing, "role") != "" && firstStringFromMap(out, "role") == "" {
			out["role"] = existing["role"]
		}
		if uiMacroHasDefaultRange(next) && !uiMacroHasDefaultRange(existing) {
			for _, key := range []string{"min", "max", "unit", "value"} {
				if value, ok := existing[key]; ok {
					out[key] = value
				}
			}
		}
	}
	if len(existingBindings) > 0 && uiMacroIsTrackVolume(existing) && uiMacroIsTrackVolume(next) && !nextHasSyncedTrackVolume {
		if value, ok := existing["value"]; ok {
			out["value"] = value
		}
	}
	return out
}

func uiMacroHasDefaultRange(macro map[string]any) bool {
	return mixFloatNumber(macro["min"]) == 0 && mixFloatNumber(macro["max"]) == 1
}

func (s *Server) uiContextSnapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneContext(s.uiContext)
}

func mergeUIContext(state map[string]any, uiContext map[string]any) map[string]any {
	if len(uiContext) == 0 {
		return state
	}
	out := cloneContext(state)
	if out == nil {
		out = map[string]any{}
	}
	for key, value := range uiContext {
		if !isEmptyContextValue(uiContext, key) {
			out[key] = value
		}
	}
	return out
}

func sanitizeUIContext(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"selected_track_id",
		"selected_track_name",
		"selected_scene_track_id",
		"selected_clip_id",
		"selected_clip_ids",
		"selected_clip_track_id",
		"selected_clip_name",
		"piano_roll_focus_clip_id",
		"piano_roll_focus_track_id",
		"selected_plugin_id",
		"selected_plugin_name",
		"selected_plugin_track_id",
		"selected_plugin_source",
		"rack_control_macros",
		"macro_controls",
		"active_macro_controls",
		"macro_control_summary",
		"active_workflow_mode",
		"active_mode_status",
		"playhead_seconds",
		"current_playhead_seconds",
		"transport_position_seconds",
	} {
		value, ok := in[key]
		if !ok {
			continue
		}
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			out[key] = value
		}
	}
	return out
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
		redacted, routeKeys := config.RedactSecrets(cfg)
		writeJSON(w, http.StatusOK, ConfigResponse{Status: "ok", Path: path, Config: redacted, HasAPIKey: hasAPIKey, RouteAPIKeys: routeKeys})
	case http.MethodPost:
		var cfg config.EngineConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
			return
		}
		cfg.Normalize()
		current, _, loadErr := config.Load()
		if loadErr == nil {
			cfg = config.PreserveBlankSecrets(cfg, current)
		}
		path, err := config.Save(cfg)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		hasAPIKey := strings.TrimSpace(cfg.APIKey) != ""
		redacted, routeKeys := config.RedactSecrets(cfg)
		writeJSON(w, http.StatusOK, ConfigResponse{Status: "ok", Path: path, Config: redacted, HasAPIKey: hasAPIKey, RouteAPIKeys: routeKeys})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET or POST required"})
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	logConversationID := ""
	messageLen := 0
	defer func() {
		if s != nil && s.logger != nil {
			s.logger.Info("[timing] http.chat total_ms=%d conversation=%s message_len=%d",
				time.Since(started).Milliseconds(), logConversationID, messageLen)
		}
	}()
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
	messageLen = len(req.Message)
	conversationID := strings.TrimSpace(req.ConversationID)
	if conversationID == "" {
		conversationID = "chat_" + randomID()
	}
	logConversationID = conversationID
	req.Context = contextWithConversationID(req.Context, conversationID)
	projectPath := projectPathFromChatContext(req.Context)
	agentMode := agentModeFromContext(req.Context)
	goal := s.beginChatGoal(conversationID, req.Message, req.Context)
	req.Context = contextWithGoal(req.Context, goal.GoalID, goal.RunID)
	s.emitTurnEvent(conversationID, "turn.started", ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		GoalStatus:     string(agentruntime.StatusRunning),
		Reply:          req.Message,
	}, goal.GoalID, goal.RunID)
	req.Context = contextWithAttachments(req.Context, req.Attachments)
	attachmentScope := artifacts.ScopeFromMap(req.Context)
	requestArtifacts := mergeArtifactSummaries(
		s.artifactsFromAttachments(req.Attachments, conversationID, goal.GoalID, goal.RunID, attachmentScope),
		s.artifactsFromRefs(req.ArtifactRefs),
	)
	req.Context = contextWithArtifactSummaries(req.Context, requestArtifacts)
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
		if len(resp.ProjectResultCards) == 0 {
			resp.ProjectResultCards = projectResultCardsFromExecuted(resp.ExecutedKernelReply)
		}
		goalID := firstNonEmpty(resp.GoalID, goal.GoalID)
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, artifactSummariesFromExecuted(resp.ExecutedKernelReply))
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, s.artifactsFromDialogueMediaReferences(req.Message, resp.Reply, conversationID, goalID, firstNonEmpty(resp.RunID, goal.RunID), attachmentScope))
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, requestArtifacts)
		if strings.TrimSpace(resp.Reply) != "" {
			historyData := map[string]any{
				"artifacts": artifactSummaryRows(resp.Artifacts),
			}
			if len(resp.ProjectResultCards) > 0 {
				historyData["project_result_cards"] = resp.ProjectResultCards
			}
			if history := s.harness.RecordConversationNodeForProjectWithData(r.Context(), projectPath, "vit", resp.Reply, goalID, resp.RunID, historyData); len(history) > 0 {
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
		if s.logger != nil {
			s.logger.Info("[chat.artifacts] conversation=%s response_artifacts=%d request_artifacts=%d executed=%d reply_len=%d",
				conversationID, len(resp.Artifacts), len(requestArtifacts), len(resp.ExecutedKernelReply), len(resp.Reply))
		}
		responseMode := agentModeFromString(resp.AgentMode)
		if resp.AgentPlan == nil && shouldExposeAgentPlan(responseMode, resp) {
			resp.AgentPlan = simpleAgentPlan(resp.GoalID, resp.RunID, agentruntime.GoalStatus(resp.GoalStatus), resp.GoalSummary, resp.CurrentStep, resp.Error, resp.ProjectHistory)
		}
		if resp.AgentPlan != nil {
			syncAgentPlanProjectHistory(&resp)
		}
		s.attachInteractionRequests(&resp)
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
		eventType := "turn.completed"
		if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			eventType = "turn.failed"
		}
		s.emitTurnEvent(conversationID, eventType, resp, goal.GoalID, goal.RunID)
		writeJSON(w, status, resp)
	}

	if !strings.HasPrefix(req.Message, "/") {
		if plan, ok := s.pendingPlanForChat(conversationID, req.Context); ok {
			projectHistory := s.harness.ProjectHistorySummaryForProject(r.Context(), goal.GoalID, projectPath)
			pendingAgentMode := agentModeFromContext(plan.Context)
			resp := ChatResponse{
				ConversationID:    conversationID,
				AgentMode:         pendingAgentMode,
				Reply:             "当前操作仍在等待确认，请先确认执行或取消。",
				AgentPlan:         agentPlanForMode(pendingAgentMode, agentPlanFromPendingPlan(plan, agentruntime.StatusWaitingConfirmation, projectHistory)),
				NeedsConfirmation: true,
				PlanID:            plan.ID,
				Preview:           plan.Preview,
				Workflow:          plan.Workflow,
				WorkflowData:      plan.WorkflowData,
				Commands:          plan.Decisions,
				GoalStatus:        string(agentruntime.StatusWaitingConfirmation),
				ProjectHistory:    projectHistory,
			}
			if isPluginGrabberLearningPlan(plan) {
				resp.PluginLearning = plan.WorkflowData
			}
			s.attachInteractionRequests(&resp)
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
				Reply:             "还没有执行。上一项操作仍在等待确认，请先确认执行或取消。",
				NeedsConfirmation: true,
				PlanID:            plan.ID,
				Preview:           plan.Preview,
				Workflow:          plan.Workflow,
				WorkflowData:      plan.WorkflowData,
				Commands:          plan.Decisions,
			}
			if isPluginGrabberLearningPlan(plan) {
				resp.PluginLearning = plan.WorkflowData
			}
			s.attachInteractionRequests(&resp)
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
	if resp, handled := s.runMixSessionEntryChat(r.Context(), conversationID, ChatRequest{
		ConversationID: conversationID,
		Message:        req.Message,
		Context:        chatContext,
		Attachments:    req.Attachments,
		ArtifactRefs:   req.ArtifactRefs,
	}, agentMode); handled {
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}

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

	assembly := s.buildAssembly(r.Context(), conversationID, req.Message, req.Context)
	goalID, _ := goalIDsFromContext(req.Context)
	resp, err := s.llm.CompleteRequest(r.Context(), cfg, llm.Request{
		Messages: assembly.Messages,
		Metadata: llm.RequestMetadata{
			Source:            "chat",
			ConversationID:    conversationID,
			GoalID:            goalID,
			PromptFingerprint: assembly.Fingerprint,
			PromptStats:       assembly.Stats.Map(),
		},
	})
	if err != nil {
		writeChat(http.StatusOK, ChatResponse{
			ConversationID: conversationID,
			Reply:          "Ask Vit AI call failed: " + err.Error(),
			Error:          err.Error(),
		})
		return
	}
	raw := resp.Text
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
	// HARDCODED: Generate Learn command if user text contains learning keyword
	if strings.Contains(req.Message, "\u5b66\u4e60") {
		env.Commands = append(env.Commands, map[string]any{"cmd": "plugin_grabber_learn_project_profile", "intent": req.Message})
	}
	if workflowCmd, ok := firstPluginGrabberLearningCommand(env.Commands); ok {
		resp := s.runPluginGrabberLearningWorkflow(r.Context(), conversationID, req.Message, chatContext, cfg, workflowCmd)
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
	reply := "计划模式不会执行工程修改或高风险写入。本次已阻止：" + strings.Join(names, ", ") + "。切换到默认模式或目标模式后才能执行。"
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
			FailureReason:  "计划模式不会执行工程修改或高风险写入。",
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
		if spec, ok := catalog.LookupTool(name); ok {
			return spec, true
		}
	}
	if cmdName := policy.CommandName(decision.Command); strings.TrimSpace(cmdName) != "" {
		if spec, ok := catalog.LookupCommand(cmdName); ok {
			return spec, true
		}
		if spec, ok := catalog.LookupTool(cmdName); ok {
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
		reply = "已完成。"
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
	for key, plan := range s.pending {
		if s.pendingPlanAliasIsStaleLocked(key, plan) {
			delete(s.pending, key)
			continue
		}
		if strings.TrimSpace(fmt.Sprint(plan.WorkflowData["conversation_id"])) == conversationID {
			return plan, true
		}
		if goalID != "" && strings.TrimSpace(fmt.Sprint(plan.Context["goal_id"])) == goalID {
			return plan, true
		}
	}
	if len(s.pending) == 1 {
		for key, plan := range s.pending {
			if s.pendingPlanAliasIsStaleLocked(key, plan) {
				delete(s.pending, key)
				continue
			}
			return plan, true
		}
	}
	return PendingPlan{}, false
}

func (s *Server) attachInteractionRequests(resp *ChatResponse) {
	if resp == nil || len(resp.InteractionRequests) > 0 {
		return
	}
	if len(resp.PluginLearning) > 0 {
		req := s.pluginLearningInteractionRequest(*resp)
		if req.ID != "" {
			resp.InteractionRequests = append(resp.InteractionRequests, req)
			if len(req.Actions) > 0 || !responseNeedsConfirmationInteraction(*resp) {
				return
			}
		}
	}
	if responseNeedsConfirmationInteraction(*resp) {
		s.hydrateConfirmationResponse(resp)
	}
	if responseNeedsConfirmationInteraction(*resp) && strings.TrimSpace(resp.PlanID) != "" {
		req := s.confirmationInteractionRequest(*resp)
		if req.ID != "" {
			resp.InteractionRequests = append(resp.InteractionRequests, req)
		}
	}
}

func responseNeedsConfirmationInteraction(resp ChatResponse) bool {
	status := strings.ToLower(strings.TrimSpace(resp.GoalStatus))
	return resp.NeedsConfirmation ||
		status == string(agentruntime.StatusWaitingConfirmation) ||
		status == "needs_confirmation" ||
		strings.Contains(status, "waiting_confirmation") ||
		strings.Contains(status, "needs_confirmation")
}

func (s *Server) hydrateConfirmationResponse(resp *ChatResponse) {
	if resp == nil {
		return
	}
	resp.NeedsConfirmation = true
	if planID := firstNonEmpty(
		strings.TrimSpace(resp.PlanID),
		mapPlanID(resp.WorkflowData),
		mapPlanID(resp.PluginLearning),
	); planID != "" {
		resp.PlanID = planID
	}
	if strings.TrimSpace(resp.PlanID) != "" {
		return
	}
	plan, ok := s.pendingPlanForResponse(*resp)
	if !ok {
		return
	}
	resp.PlanID = strings.TrimSpace(plan.ID)
	if strings.TrimSpace(resp.Preview) == "" {
		resp.Preview = plan.Preview
	}
	if strings.TrimSpace(resp.Workflow) == "" {
		resp.Workflow = plan.Workflow
	}
	if len(resp.WorkflowData) == 0 && len(plan.WorkflowData) > 0 {
		resp.WorkflowData = copyStringAnyMap(plan.WorkflowData)
	}
	if len(resp.Commands) == 0 && len(plan.Decisions) > 0 {
		resp.Commands = plan.Decisions
	}
}

func mapPlanID(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	return cleanContextText(data["plan_id"])
}

func (s *Server) pendingPlanForResponse(resp ChatResponse) (PendingPlan, bool) {
	conversationID := strings.TrimSpace(resp.ConversationID)
	goalID := strings.TrimSpace(resp.GoalID)
	runID := strings.TrimSpace(resp.RunID)
	workflow := strings.TrimSpace(resp.Workflow)
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, plan := range s.pending {
		if s.pendingPlanAliasIsStaleLocked(key, plan) {
			delete(s.pending, key)
			continue
		}
		if pendingPlanMatchesResponse(plan, conversationID, goalID, runID, workflow) {
			return plan, true
		}
	}
	if len(s.pending) == 1 {
		for key, plan := range s.pending {
			if s.pendingPlanAliasIsStaleLocked(key, plan) {
				delete(s.pending, key)
				continue
			}
			return plan, true
		}
	}
	return PendingPlan{}, false
}

func pendingPlanMatchesResponse(plan PendingPlan, conversationID, goalID, runID, workflow string) bool {
	if conversationID != "" {
		if cleanContextText(plan.Context["conversation_id"]) == conversationID || cleanContextText(plan.WorkflowData["conversation_id"]) == conversationID {
			return true
		}
	}
	if goalID != "" && cleanContextText(plan.Context["goal_id"]) == goalID {
		return true
	}
	if runID != "" && cleanContextText(plan.Context["run_id"]) == runID {
		return true
	}
	if workflow != "" && strings.TrimSpace(plan.Workflow) == workflow {
		return true
	}
	return false
}

func (s *Server) confirmationInteractionRequest(resp ChatResponse) AgentInteractionRequest {
	planID := strings.TrimSpace(resp.PlanID)
	if planID == "" {
		return AgentInteractionRequest{}
	}
	payload := map[string]any{
		"plan_id":  planID,
		"preview":  resp.Preview,
		"commands": resp.Commands,
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "confirmation",
		Type:           "confirmation",
		Source:         "vit_agent",
		Title:          "需要确认",
		Body:           firstNonEmpty(strings.TrimSpace(resp.Reply), "Vit 需要你确认后再继续。"),
		Status:         "waiting_for_user",
		Workflow:       resp.Workflow,
		PlanID:         planID,
		ConversationID: resp.ConversationID,
		GoalID:         resp.GoalID,
		RunID:          resp.RunID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "确认执行", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
	s.storePendingInteraction(req, resp.WorkflowData)
	return req
}

func (s *Server) pluginLearningInteractionRequest(resp ChatResponse) AgentInteractionRequest {
	data := map[string]any{}
	for key, value := range resp.PluginLearning {
		data[key] = value
	}
	if len(data) == 0 {
		return AgentInteractionRequest{}
	}
	planID := firstNonEmpty(strings.TrimSpace(resp.PlanID), strings.TrimSpace(fmt.Sprint(data["plan_id"])))
	stage := strings.TrimSpace(fmt.Sprint(data["stage"]))
	mode := strings.TrimSpace(fmt.Sprint(data["mode"]))
	kind := "review"
	typ := "plugin_learning_review"
	title := "复核 Plugin Grabber 学习结果"
	body := strings.TrimSpace(resp.Reply)
	if body == "" {
		body = "请复核本次 Plugin Grabber 学习结果。"
	}
	actions := []AgentInteractionAction{}
	reviewItems := pluginLearningReviewItems(data)
	fields := []AgentInteractionField{}
	questions := []AgentInteractionField{}
	switch {
	case stage == pluginLearningUIReferenceStage || strings.EqualFold(strings.TrimSpace(fmt.Sprint(data["type"])), pluginLearningUIReferenceType):
		kind = "form"
		typ = pluginLearningUIReferenceType
		title = "提供插件界面图样"
		body = firstNonEmpty(body, "可以提供一份本次学习专用的插件界面图样，也可以跳过直接学习。只有当前卡片上传并绑定到本次学习会话的图样会进入学习。")
		actions = []AgentInteractionAction{
			{ID: "continue_with_ui_reference", Label: "使用图样继续", Style: "primary", Recommended: true},
			{ID: "skip_ui_reference", Label: "跳过图样，直接学习", Style: "secondary"},
			{ID: "cancel_plugin_learning", Label: "取消学习", Style: "secondary"},
		}
	case boolValue(data["learning_completed"]):
		kind = "mode_boundary"
		typ = "plugin_learning_completion"
		title = "Plugin Grabber 已完成"
		actions = []AgentInteractionAction{{ID: "done", Label: "完成", Style: "primary", Recommended: true}}
	case stage == "candidate_review" || boolValue(data["needs_user_review"]):
		kind = "review"
		typ = "plugin_learning_candidate_review"
		title = "复核自动学习候选"
		body = firstNonEmpty(body, "我已生成插件控制候选，请复核后继续生成保存草图。")
		primaryLabel := "确认候选草图"
		if len(mapRowsValue(data["experiments"])) == 0 {
			primaryLabel = "生成保存草图"
		}
		actions = []AgentInteractionAction{
			{ID: "submit", Label: primaryLabel, Style: "primary", Recommended: true},
			{ID: "skip_experiments", Label: "跳过抽样，生成保存草图", Style: "secondary"},
			{ID: "rerun", Label: "重新扫描", Style: "secondary"},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		}
	case mode == string(plugingrabber.LearningModeTeach):
		kind = "review"
		typ = "plugin_learning_teach_review"
		title = "复核教学模式结果"
		actions = []AgentInteractionAction{
			{ID: "approve", Label: "保存", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		}
	case planID != "":
		kind = "review"
		typ = "plugin_learning_final_review"
		title = "确认保存 Plugin Skill"
		actions = []AgentInteractionAction{
			{ID: "approve", Label: "保存", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
			{ID: "revise", Label: "继续修改", Style: "secondary"},
		}
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           kind,
		Type:           typ,
		Title:          title,
		Body:           body,
		Status:         "waiting_for_user",
		Source:         "plugin_grabber",
		Workflow:       firstNonEmpty(resp.Workflow, strings.TrimSpace(fmt.Sprint(data["workflow"]))),
		Stage:          firstNonEmpty(stage, typ),
		PlanID:         planID,
		ConversationID: resp.ConversationID,
		GoalID:         resp.GoalID,
		RunID:          resp.RunID,
		Questions:      questions,
		Fields:         fields,
		ReviewItems:    reviewItems,
		Payload:        data,
		Data:           data,
		Actions:        actions,
	}
	data["interaction_id"] = req.ID
	if planID != "" {
		data["plan_id"] = planID
	}
	s.storePendingInteraction(req, data)
	return req
}

func pluginLearningReviewItems(data map[string]any) []AgentInteractionReview {
	if len(data) == 0 {
		return nil
	}
	items := []AgentInteractionReview{}
	if pluginName := strings.TrimSpace(fmt.Sprint(data["plugin_name"])); pluginName != "" && pluginName != "<nil>" {
		items = append(items, AgentInteractionReview{
			ID:     "plugin",
			Title:  "插件",
			Body:   pluginName,
			Status: "info",
		})
	}
	if count := intNumber(data["parameter_count"]); count > 0 {
		items = append(items, AgentInteractionReview{
			ID:     "parameter_count",
			Title:  "扫描统计",
			Body:   fmt.Sprintf("共抓到 %d 个参数。", count),
			Status: "info",
		})
	}
	if count := intNumber(data["component_count"]); count > 0 {
		items = append(items, AgentInteractionReview{
			ID:     "component_count",
			Title:  "候选组件",
			Body:   fmt.Sprintf("生成 %d 个候选组件。", count),
			Status: "info",
		})
	}
	if count := intNumber(data["operation_count"]); count > 0 {
		items = append(items, AgentInteractionReview{
			ID:     "operation_count",
			Title:  "候选控制",
			Body:   fmt.Sprintf("生成 %d 个自然语言控制。", count),
			Status: "info",
		})
	}
	if summary, ok := data["display_domain_summary"].(map[string]int); ok && len(summary) > 0 {
		items = append(items, AgentInteractionReview{
			ID:     "display_domain_summary",
			Title:  "显示域",
			Body:   fmt.Sprintf("已确认 %d，推断 %d，待确认 %d，未知 %d。", summary["confirmed"], summary["inferred"], summary["needs_confirmation"], summary["unknown"]),
			Status: "info",
			Payload: map[string]any{
				"summary": summary,
			},
		})
	}
	if warnings := stringListValue(data["validation_warnings"]); len(warnings) > 0 {
		items = append(items, AgentInteractionReview{
			ID:     "validation_warnings",
			Title:  "提醒",
			Body:   strings.Join(firstStringLimit(warnings, 3), "\n"),
			Status: "warning",
		})
	}
	return items
}

func (s *Server) storePendingInteraction(req AgentInteractionRequest, data map[string]any) {
	if strings.TrimSpace(req.ID) == "" {
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = strings.TrimSpace(req.Type)
	}
	payload := req.Payload
	if len(payload) == 0 {
		payload = data
	}
	requestContext := map[string]any{}
	if ctx, ok := payload["request_context"].(map[string]any); ok {
		requestContext = ctx
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interactions[req.ID] = PendingInteraction{
		ID:             req.ID,
		CreatedAt:      time.Now(),
		Kind:           kind,
		Source:         req.Source,
		Workflow:       req.Workflow,
		Stage:          req.Stage,
		PlanID:         req.PlanID,
		ConversationID: req.ConversationID,
		GoalID:         req.GoalID,
		RunID:          req.RunID,
		RequestContext: requestContext,
		Payload:        payload,
		Type:           req.Type,
		Data:           data,
	}
}

func (s *Server) takePendingInteraction(interactionID string) (PendingInteraction, bool) {
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return PendingInteraction{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	interaction, ok := s.interactions[interactionID]
	if ok {
		delete(s.interactions, interactionID)
	}
	return interaction, ok
}

func (s *Server) recoverMixBoardInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if len(payload) == 0 {
		return PendingInteraction{}, false
	}
	session := mixSessionFromMap(mapValue(payload["mix_session"]))
	sessionID := firstNonEmpty(session.MixSessionID, cleanContextText(payload["mix_session_id"]))
	if sessionID != "" {
		if stored, ok := s.lookupMixSession(sessionID); ok {
			session = mergeRecoveredMixSession(stored, session)
		}
	}
	if session.MixSessionID == "" {
		return PendingInteraction{}, false
	}
	data := copyStringAnyMap(payload)
	data["mix_session"] = mixSessionMap(session)
	requestContext := mapValue(payload["request_context"])
	conversationID := firstNonEmpty(cleanContextText(payload["conversation_id"]), cleanContextText(data["conversation_id"]), interactionID)
	return PendingInteraction{
		ID:             interactionID,
		CreatedAt:      time.Now(),
		Kind:           "mode_boundary",
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Stage:          session.State,
		ConversationID: conversationID,
		GoalID:         cleanContextText(payload["goal_id"]),
		RunID:          cleanContextText(payload["run_id"]),
		RequestContext: requestContext,
		Payload:        data,
		Type:           "mixboard_status",
		Data:           data,
	}, true
}

func writeMixBoardDiag(event string, fields map[string]any) {
	logDir := `D:\Vit_DAW\VitApp\Workspace\Logs`
	if st, err := os.Stat(logDir); err != nil || !st.IsDir() {
		logDir = os.TempDir()
	}
	row := map[string]any{
		"event":      event,
		"created_at": time.Now().Format(time.RFC3339Nano),
	}
	for key, value := range fields {
		row[key] = value
	}
	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	_ = os.MkdirAll(logDir, 0o755)
	f, err := os.OpenFile(filepath.Join(logDir, "mixboard_interaction_diag.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(data, '\n'))
}

func mapKeysForDiag(row map[string]any) []string {
	if len(row) == 0 {
		return nil
	}
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	return keys
}

func (s *Server) takePendingPlan(planKey string) (PendingPlan, bool, bool) {
	planKey = strings.TrimSpace(planKey)
	if planKey == "" {
		return PendingPlan{}, false, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, ok := s.pending[planKey]
	if !ok {
		stale := s.deletePendingAliasesForPlanIDLocked(planKey) > 0
		return PendingPlan{}, false, stale
	}
	planID := strings.TrimSpace(plan.ID)
	if planID == "" {
		delete(s.pending, planKey)
		return plan, true, false
	}
	if planKey != planID {
		if _, hasCanonical := s.pending[planID]; !hasCanonical {
			s.deletePendingAliasesForPlanIDLocked(planID)
			return PendingPlan{}, false, true
		}
	}
	s.deletePendingAliasesForPlanIDLocked(planID)
	return plan, true, false
}

func (s *Server) pendingPlanAliasIsStaleLocked(key string, plan PendingPlan) bool {
	planID := strings.TrimSpace(plan.ID)
	if planID == "" || key == planID {
		return false
	}
	_, hasCanonical := s.pending[planID]
	return !hasCanonical
}

func (s *Server) deletePendingAliasesForPlanIDLocked(planID string) int {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return 0
	}
	removed := 0
	for key, plan := range s.pending {
		if key == planID || strings.TrimSpace(plan.ID) == planID {
			delete(s.pending, key)
			removed++
		}
	}
	return removed
}

func isPluginGrabberLearningPlan(plan PendingPlan) bool {
	return pluginGrabberLearningMode(plan) != ""
}

func pluginGrabberLearningMode(plan PendingPlan) string {
	mode := strings.TrimSpace(strings.ToLower(fmt.Sprint(plan.WorkflowData["mode"])))
	if mode == string(plugingrabber.LearningModeAutoLearn) || mode == string(plugingrabber.LearningModeTeach) {
		return mode
	}
	workflow := strings.ToLower(strings.TrimSpace(plan.Workflow))
	if strings.Contains(workflow, "auto_learn") {
		return string(plugingrabber.LearningModeAutoLearn)
	}
	if strings.Contains(workflow, "teach") {
		return string(plugingrabber.LearningModeTeach)
	}
	return ""
}

func pluginGrabberLearningCompletionData(plan PendingPlan, saved bool) map[string]any {
	mode := pluginGrabberLearningMode(plan)
	if mode == "" {
		return nil
	}
	data := map[string]any{}
	for key, value := range plan.WorkflowData {
		data[key] = value
	}
	data["mode"] = mode
	data["state"] = "cancelled"
	if saved {
		data["state"] = "saved"
	}
	data["plan_id"] = plan.ID
	data["needs_confirmation"] = false
	data["learning_completed"] = true
	data["next_actions"] = pluginGrabberLearningNextActions(mode, saved)
	return data
}

func attachPluginGrabberCompletionAssets(response map[string]any, data map[string]any) {
	if response == nil || len(data) == 0 {
		return
	}
	if artifacts := firstPresentAny(data, "artifacts"); artifacts != nil {
		response["artifacts"] = artifacts
	}
	if sidePanelRequest := firstPresentAny(data, "side_panel_request"); sidePanelRequest != nil {
		response["side_panel_request"] = sidePanelRequest
	}
}

func pluginGrabberLearningNextActions(mode string, saved bool) []map[string]any {
	if !saved {
		return []map[string]any{{
			"id":    "done",
			"label": "完成",
		}}
	}
	continueLabel := "继续自动学习"
	continueMode := string(plugingrabber.LearningModeAutoLearn)
	if mode == string(plugingrabber.LearningModeTeach) {
		continueLabel = "继续教学"
		continueMode = string(plugingrabber.LearningModeTeach)
	}
	return []map[string]any{
		{
			"id":     "explain_controls",
			"label":  "解释控制",
			"prompt": "解释这个插件刚保存的 Plugin Skill 控制。",
		},
		{
			"id":     "try_controls",
			"label":  "试用控制",
			"prompt": "试用刚保存的 Plugin Skill 控制。",
		},
		{
			"id":            "continue_learning",
			"label":         continueLabel,
			"learning_mode": continueMode,
		},
		{
			"id":    "done",
			"label": "完成",
		},
	}
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

func (s *Server) handleInteractionRespond(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	logInteractionID := ""
	logActionID := ""
	defer func() {
		if s != nil && s.logger != nil {
			s.logger.Info("[timing] http.interaction_respond total_ms=%d interaction=%s action=%s",
				time.Since(started).Milliseconds(), logInteractionID, logActionID)
		}
	}()
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	var req InteractionRespondRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	interactionID := strings.TrimSpace(req.InteractionID)
	logInteractionID = interactionID
	if interactionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "interaction_id is required"})
		return
	}
	decision := firstNonEmpty(strings.TrimSpace(req.Decision), strings.TrimSpace(req.ActionID))
	if decision == "" {
		decision = "submit"
	}
	logActionID = decision
	payloadFields := mapValue(req.Payload["fields"])
	payloadSession := mapValue(req.Payload["mix_session"])
	writeMixBoardDiag("interaction_respond_received", map[string]any{
		"interaction_id":    interactionID,
		"decision":          decision,
		"action_id":         strings.TrimSpace(req.ActionID),
		"payload_keys":      mapKeysForDiag(req.Payload),
		"payload_user_note": cleanContextText(req.Payload["user_note"]),
		"field_user_note":   cleanContextText(payloadFields["user_note"]),
		"has_mix_session":   len(payloadSession) > 0,
		"mix_session_id":    cleanContextText(payloadSession["mix_session_id"]),
		"client_build":      cleanContextText(req.Payload["_mixboard_client_build"]),
	})
	interaction, ok := s.takePendingInteraction(interactionID)
	if !ok {
		interaction, ok = s.recoverMixBoardInteractionFromPayload(interactionID, req.Payload)
		if ok && s != nil && s.logger != nil {
			s.logger.Info("[mixboard.interaction] recovered expired interaction=%s decision=%s session=%s client_build=%s", interactionID, decision, cleanContextText(mapValue(req.Payload["mix_session"])["mix_session_id"]), cleanContextText(req.Payload["_mixboard_client_build"]))
		}
	}
	if !ok {
		writeJSON(w, http.StatusOK, ChatResponse{
			ConversationID: interactionID,
			Reply:          "这个交互已处理或已过期。",
			GoalStatus:     string(agentruntime.StatusCompleted),
		})
		return
	}
	kind := firstNonEmpty(interaction.Kind, interaction.Type)
	if strings.TrimSpace(interaction.PlanID) != "" && (kind == "confirmation" || strings.Contains(interaction.Type, "final_review") || strings.Contains(interaction.Type, "teach_review")) {
		status, response := s.resolvePendingPlanDecision(r.Context(), interaction.PlanID, decision)
		response["interaction_id"] = interaction.ID
		s.attachInteractionsToResponseMap(&response, interaction)
		writeJSON(w, status, response)
		return
	}
	if strings.EqualFold(decision, "cancel") || strings.EqualFold(decision, "cancel_plugin_learning") {
		resp := ChatResponse{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Reply:          "已取消。",
			Workflow:       interaction.Workflow,
			WorkflowData:   interaction.Payload,
			GoalStatus:     string(agentruntime.StatusCancelled),
			InteractionRequests: []AgentInteractionRequest{{
				ID:             "interaction_" + randomID(),
				Kind:           "mode_boundary",
				Source:         interaction.Source,
				Workflow:       interaction.Workflow,
				Stage:          "cancelled",
				Title:          "已取消",
				Body:           "已退出当前交互。",
				Status:         "completed",
				ConversationID: interaction.ConversationID,
				GoalID:         interaction.GoalID,
				RunID:          interaction.RunID,
				Payload:        interaction.Payload,
				Actions:        []AgentInteractionAction{{ID: "done", Label: "完成", Style: "primary", Recommended: true}},
			}},
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "mix_session") || strings.EqualFold(interaction.Workflow, mixSessionEntryWorkflow) || strings.EqualFold(interaction.Type, "mix_session_entry") {
		if s != nil && s.logger != nil {
			fields := mapValue(req.Payload["fields"])
			s.logger.Info("[mixboard.interaction] decision=%s interaction=%s payload_user_note=%q field_user_note=%q", decision, interactionID, cleanContextText(req.Payload["user_note"]), cleanContextText(fields["user_note"]))
		}
		resp := s.continueMixSessionInteraction(r.Context(), interaction, req.Payload, decision)
		writeJSON(w, mixSessionHTTPStatus(resp), resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.Contains(interaction.Type, "ui_reference_request") {
		resp, err := s.continuePluginGrabberUIReferenceInteraction(r.Context(), interaction, req.Payload, decision)
		if err != nil {
			writeJSON(w, http.StatusOK, pluginGrabberInteractionErrorResponse(interaction, err))
			return
		}
		s.attachInteractionRequests(&resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.Contains(interaction.Type, "candidate_review") && strings.EqualFold(decision, "start_experiments") {
		next := interaction
		next.Payload = pluginGrabberPayloadWithSubmittedReviews(interaction.Payload, req.Payload)
		next.Data = next.Payload
		resp, err := s.beginPluginGrabberExperimentInteraction(r.Context(), next, 0)
		if err != nil {
			writeJSON(w, http.StatusOK, pluginGrabberInteractionErrorResponse(interaction, err))
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.Contains(interaction.Type, "experiment") {
		resp, err := s.completePluginGrabberExperimentInteraction(r.Context(), interaction, req.Payload)
		if err != nil {
			writeJSON(w, http.StatusOK, pluginGrabberInteractionErrorResponse(interaction, err))
			return
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.Contains(interaction.Type, "display_domain_form") {
		resp, err := s.finalizePluginGrabberDisplayDomainInteraction(r.Context(), interaction, req.Payload)
		if err != nil {
			writeJSON(w, http.StatusOK, pluginGrabberInteractionErrorResponse(interaction, err))
			return
		}
		s.attachInteractionRequests(&resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.Contains(interaction.Type, "candidate_review") {
		resp, err := s.continuePluginGrabberCandidateInteraction(r.Context(), interaction, req.Payload, decision)
		if err != nil {
			writeJSON(w, http.StatusOK, pluginGrabberInteractionErrorResponse(interaction, err))
			return
		}
		s.attachInteractionRequests(&resp)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	writeJSON(w, http.StatusOK, ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          "已收到。",
		Workflow:       interaction.Workflow,
		WorkflowData:   interaction.Payload,
	})
}

func (s *Server) attachInteractionsToResponseMap(response *map[string]any, interaction PendingInteraction) {
	if response == nil || *response == nil {
		return
	}
	data := mapValue((*response)["plugin_learning"])
	if len(data) == 0 {
		data = mapValue((*response)["workflow_data"])
	}
	goalStatus := strings.TrimSpace(fmt.Sprint((*response)["goal_status"]))
	needsConfirmation := boolValue((*response)["needs_confirmation"]) || strings.EqualFold(goalStatus, string(agentruntime.StatusWaitingConfirmation))
	resp := ChatResponse{
		ConversationID:    interaction.ConversationID,
		GoalID:            firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["goal_id"])), interaction.GoalID),
		RunID:             firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["run_id"])), interaction.RunID),
		Reply:             firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["message"])), strings.TrimSpace(fmt.Sprint((*response)["reply"]))),
		Workflow:          firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["workflow"])), interaction.Workflow),
		WorkflowData:      data,
		PluginLearning:    data,
		NeedsConfirmation: needsConfirmation,
		PlanID:            firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["plan_id"])), strings.TrimSpace(fmt.Sprint((*response)["next_plan_id"]))),
		Preview:           strings.TrimSpace(fmt.Sprint((*response)["preview"])),
		GoalStatus:        goalStatus,
	}
	if len(data) == 0 && !responseNeedsConfirmationInteraction(resp) {
		return
	}
	s.attachInteractionRequests(&resp)
	if len(resp.InteractionRequests) > 0 {
		(*response)["interaction_requests"] = resp.InteractionRequests
	}
	if strings.TrimSpace(resp.Reply) != "" {
		(*response)["reply"] = resp.Reply
	}
	if resp.ConversationID != "" {
		(*response)["conversation_id"] = resp.ConversationID
	}
}

func (s *Server) continuePluginGrabberUIReferenceInteraction(ctx context.Context, interaction PendingInteraction, payload map[string]any, decision string) (ChatResponse, error) {
	base := copyStringAnyMap(interaction.Payload)
	submitted := copyStringAnyMap(payload)
	target := mapValue(base["target"])
	if len(target) == 0 {
		target = map[string]any{
			"track_id":    base["track_id"],
			"plugin_id":   base["plugin_id"],
			"plugin_name": base["plugin_name"],
		}
	}
	sessionID := firstNonEmpty(firstNonEmptyText(submitted, "plugin_learning_session_id"), firstNonEmptyText(base, "plugin_learning_session_id"))
	startedAt := firstNonEmpty(firstNonEmptyText(submitted, "plugin_learning_session_started_at"), firstNonEmptyText(base, "plugin_learning_session_started_at"))
	webReferenceDecision := strings.TrimSpace(strings.ToLower(firstNonEmpty(
		firstNonEmptyText(submitted, "web_reference_decision"),
		firstNonEmptyText(base, "web_reference_decision"),
	)))
	if webReferenceDecision == "" {
		if boolValue(firstPresentAny(submitted, "web_reference_enabled")) || boolValue(firstPresentAny(base, "web_reference_enabled")) {
			webReferenceDecision = "enabled"
		} else {
			webReferenceDecision = "skipped"
		}
	}
	args := map[string]any{
		"mode":                               string(plugingrabber.LearningModeAutoLearn),
		"track_id":                           firstNonEmptyText(target, "track_id"),
		"plugin_id":                          firstNonEmptyText(target, "plugin_id"),
		"plugin_name":                        firstNonEmptyText(target, "plugin_name"),
		"plugin_learning_session_id":         sessionID,
		"plugin_learning_session_started_at": startedAt,
		"web_reference_decision":             webReferenceDecision,
	}
	if args["track_id"] == "" {
		args["track_id"] = firstNonEmptyText(base, "track_id")
	}
	if args["plugin_id"] == "" {
		args["plugin_id"] = firstNonEmptyText(base, "plugin_id")
	}
	if args["plugin_name"] == "" {
		args["plugin_name"] = firstNonEmptyText(base, "plugin_name")
	}
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "skip_ui_reference", "skip", "skipped":
		args["ui_reference_decision"] = "skipped"
	default:
		args["ui_reference_decision"] = "provided"
		ids := stringListValue(firstPresentAny(submitted, "ui_reference_artifact_ids", "artifact_ids"))
		args["ui_reference_artifact_ids"] = ids
	}
	cfg, _, err := config.Load()
	if err != nil {
		return ChatResponse{}, err
	}
	resp := s.runPluginGrabberLearningWorkflow(ctx, interaction.ConversationID, "", interaction.RequestContext, cfg, args)
	resp.GoalID = firstNonEmpty(resp.GoalID, interaction.GoalID)
	resp.RunID = firstNonEmpty(resp.RunID, interaction.RunID)
	if resp.Error != "" && strings.Contains(resp.Error, errPluginUIReferenceNoValidImage.Error()) {
		return pluginGrabberUIReferenceRecoverableResponse(interaction, base), nil
	}
	return resp, nil
}

func pluginGrabberUIReferenceRecoverableResponse(interaction PendingInteraction, base map[string]any) ChatResponse {
	target := pluginLearningTarget{
		TrackID:    firstNonEmptyText(mapValue(base["target"]), "track_id"),
		PluginID:   firstNonEmptyText(mapValue(base["target"]), "plugin_id"),
		PluginName: firstNonEmptyText(mapValue(base["target"]), "plugin_name"),
	}
	if target.TrackID == "" {
		target.TrackID = firstNonEmptyText(base, "track_id")
	}
	if target.PluginID == "" {
		target.PluginID = firstNonEmptyText(base, "plugin_id")
	}
	if target.PluginName == "" {
		target.PluginName = firstNonEmptyText(base, "plugin_name")
	}
	resp := pluginGrabberUIReferenceRequestResponse(
		interaction.ConversationID,
		firstNonEmptyText(base, "intent"),
		interaction.RequestContext,
		target,
		firstNonEmptyText(base, "plugin_learning_session_id"),
		firstNonEmptyText(base, "plugin_learning_session_started_at"),
	)
	resp.GoalID = interaction.GoalID
	resp.RunID = interaction.RunID
	resp.Reply = "还没有收到绑定到本次学习会话的有效插件界面图样。请在这张卡片里上传图样后继续，或选择跳过图样直接学习。之前发送过的图片不会被使用。"
	return resp
}

func (s *Server) continuePluginGrabberCandidateInteraction(ctx context.Context, interaction PendingInteraction, payload map[string]any, decision string) (ChatResponse, error) {
	if strings.EqualFold(decision, "rerun") {
		return s.runPluginGrabberInteractionWorkflow(ctx, interaction, payload, string(plugingrabber.LearningModeAutoLearn))
	}
	nextPayload := pluginGrabberPayloadWithSubmittedReviews(interaction.Payload, payload)
	if len(mapRowsValue(nextPayload["experiments"])) > 0 && !strings.EqualFold(decision, "skip_experiments") {
		next := interaction
		next.Payload = nextPayload
		next.Data = nextPayload
		return s.beginPluginGrabberExperimentInteraction(ctx, next, 0)
	}
	if len(pluginGrabberDisplayDomainFields(nextPayload)) > 0 {
		return s.showPluginGrabberDisplayDomainInteraction(ctx, interaction, nextPayload), nil
	}
	return s.runPluginGrabberInteractionWorkflow(ctx, interaction, nextPayload, "auto_learn_reviewed")
}

func (s *Server) beginPluginGrabberExperimentInteraction(ctx context.Context, interaction PendingInteraction, index int) (ChatResponse, error) {
	payload := copyStringAnyMap(interaction.Payload)
	experiments := mapRowsValue(payload["experiments"])
	if index < 0 || index >= len(experiments) {
		return s.showPluginGrabberDisplayDomainInteraction(ctx, interaction, payload), nil
	}
	experiment := experiments[index]
	target := mapValue(payload["target"])
	paramID := firstNonEmptyText(experiment, "param_id")
	after := floatNumber(experiment["after_normalized"])
	if err := s.setPluginParamNormalized(ctx, target, paramID, after); err != nil {
		return ChatResponse{}, err
	}
	payload["experiment_index"] = index
	payload["active_experiment"] = experiment
	title := fmt.Sprintf("实验 %d/%d", index+1, len(experiments))
	label := firstNonEmptyText(experiment, "label", "param_id")
	body := fmt.Sprintf("我临时调整了 %s。请观察插件界面或声音变化，告诉我它对应什么显示范围或单位。提交后我会先恢复原值。", label)
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "form",
		Type:           "plugin_learning_experiment",
		Source:         "plugin_grabber",
		Workflow:       interaction.Workflow,
		Stage:          "experiment_question",
		Title:          title,
		Body:           body,
		Status:         "waiting_for_user",
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Fields:         pluginGrabberExperimentFields(experiment),
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "submit", Label: "提交并恢复", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
	s.storePendingInteraction(req, payload)
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               body,
		Workflow:            interaction.Workflow,
		WorkflowData:        payload,
		PluginLearning:      payload,
		InteractionRequests: []AgentInteractionRequest{req},
	}, nil
}

func (s *Server) completePluginGrabberExperimentInteraction(ctx context.Context, interaction PendingInteraction, submitPayload map[string]any) (ChatResponse, error) {
	payload := copyStringAnyMap(interaction.Payload)
	active := mapValue(payload["active_experiment"])
	target := mapValue(payload["target"])
	paramID := firstNonEmptyText(active, "param_id")
	before := floatNumber(active["before_normalized"])
	if err := s.setPluginParamNormalized(ctx, target, paramID, before); err != nil {
		return ChatResponse{}, fmt.Errorf("恢复实验参数失败：%w", err)
	}
	fields := mapValue(submitPayload["fields"])
	observationNotes := strings.TrimSpace(fmt.Sprint(fields["observation"]))
	displayDomainText := strings.TrimSpace(fmt.Sprint(fields["display_domain_text"]))
	customDisplayDomainText := strings.TrimSpace(fmt.Sprint(fields["custom_display_domain_text"]))
	if strings.EqualFold(displayDomainText, "custom") {
		displayDomainText = customDisplayDomainText
	}
	if strings.EqualFold(displayDomainText, "not_sure") {
		displayDomainText = ""
	}
	answer := map[string]any{
		"experiment":          active,
		"observation_kind":    "",
		"observation":         observationNotes,
		"observation_notes":   observationNotes,
		"display_domain_text": displayDomainText,
	}
	answers := mapRowsValue(payload["experiment_answers"])
	answers = append(answers, answer)
	payload["experiment_answers"] = answers
	delete(payload, "active_experiment")
	index := intNumber(payload["experiment_index"]) + 1
	next := interaction
	next.Payload = payload
	next.Data = payload
	return s.beginPluginGrabberExperimentInteraction(ctx, next, index)
}

func pluginGrabberExperimentFields(experiment map[string]any) []AgentInteractionField {
	return []AgentInteractionField{
		{
			ID:          "display_domain_text",
			Label:       "这个参数的显示域更像哪一个？",
			Kind:        "choice",
			Required:    true,
			Value:       pluginGrabberDefaultDisplayDomainChoice(experiment),
			Options:     pluginGrabberDisplayDomainOptions(experiment),
			Description: "优先选择插件界面显示的单位和范围；如果都不对，选择手动填写。",
		},
		{
			ID:          "observation",
			Label:       "补充说明",
			Kind:        "text",
			Required:    false,
			Placeholder: "例如：Feedback 显示为 68%，单位是百分比",
		},
		{
			ID:          "custom_display_domain_text",
			Label:       "手动填写显示域",
			Kind:        "text",
			Placeholder: "如果上面的选项都不对，填写：20~20000 Hz / -18~18 dB / 0~100 %",
		},
	}
}

func pluginGrabberDefaultDisplayDomainChoice(experiment map[string]any) string {
	if text := firstNonEmptyText(experiment, "inferred_display_domain_text", "display_domain_text"); text != "" {
		return text
	}
	return "not_sure"
}

func pluginGrabberDisplayDomainOptions(experiment map[string]any) []map[string]any {
	inferred := firstNonEmptyText(experiment, "inferred_display_domain_text", "display_domain_text")
	candidates := []string{}
	add := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, text) {
				return
			}
		}
		candidates = append(candidates, text)
	}
	add(inferred)
	combined := strings.ToLower(strings.Join([]string{
		firstNonEmptyText(experiment, "slot"),
		firstNonEmptyText(experiment, "label"),
		firstNonEmptyText(experiment, "param_id"),
		firstNonEmptyText(experiment, "normalized_role"),
		firstNonEmptyText(experiment, "value_text"),
	}, " "))
	switch {
	case strings.Contains(combined, "delay") || strings.Contains(combined, "time") || strings.Contains(combined, "ms"):
		add("0~2000 ms")
		add("0.01~10 Hz 对数")
		add("0~100 %")
	case strings.Contains(combined, "freq") || strings.Contains(combined, "hz"):
		add("20~20000 Hz 对数")
		add("10~2000 Hz 对数")
		add("200~20000 Hz 对数")
	case strings.Contains(combined, "gain") || strings.Contains(combined, "level") || strings.Contains(combined, "db"):
		add("-18~18 dB")
		add("-12~0 dB")
		add("0~100 %")
	case strings.Contains(combined, "mix") || strings.Contains(combined, "wet") || strings.Contains(combined, "dry") || strings.Contains(combined, "feedback") || strings.Contains(combined, "depth") || strings.Contains(combined, "%"):
		add("0~100 %")
		add("-12~0 dB")
		add("0~2000 ms")
	default:
		add("0~100 %")
		add("-18~18 dB")
		add("20~20000 Hz 对数")
	}
	options := make([]map[string]any, 0, len(candidates)+2)
	for i, candidate := range candidates {
		label := candidate
		if inferred != "" && strings.EqualFold(candidate, inferred) && i == 0 {
			label = "推测：" + candidate
		}
		options = append(options, map[string]any{"value": candidate, "label": label})
	}
	options = append(options,
		map[string]any{"value": "custom", "label": "手动填写"},
		map[string]any{"value": "not_sure", "label": "不确定"},
	)
	return options
}

func pluginGrabberExperimentObservationSummary(kind, notes string) string {
	kind = strings.TrimSpace(kind)
	notes = strings.TrimSpace(notes)
	labels := map[string]string{
		"display_value_increased": "显示值变大",
		"display_value_decreased": "显示值变小",
		"sound_changed":           "声音变化",
		"no_visible_change":       "没有明显变化",
		"not_sure":                "不确定",
	}
	label := labels[kind]
	if label == "" {
		label = kind
	}
	if label == "" {
		return notes
	}
	if notes == "" {
		return label
	}
	return label + "；" + notes
}

func (s *Server) showPluginGrabberDisplayDomainInteraction(ctx context.Context, interaction PendingInteraction, payload map[string]any) ChatResponse {
	fields := pluginGrabberDisplayDomainFields(payload)
	if len(fields) == 0 {
		nextPayload := map[string]any{
			"reviewed_profile_patch": payload["profile_patch"],
		}
		if reviews := firstPresentAny(payload, "display_domain_reviews", "display_domains"); reviews != nil {
			nextPayload["display_domain_reviews"] = reviews
		}
		resp, err := s.runPluginGrabberInteractionWorkflow(ctx, interaction, nextPayload, "auto_learn_reviewed")
		if err == nil {
			s.attachInteractionRequests(&resp)
			return resp
		}
		return pluginGrabberInteractionErrorResponse(interaction, err)
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "form",
		Type:           "plugin_learning_display_domain_form",
		Source:         "plugin_grabber",
		Workflow:       interaction.Workflow,
		Stage:          "display_domain_form",
		Title:          "补充显示域",
		Body:           "请补充或确认这些参数在插件界面上的显示范围与单位。",
		Status:         "waiting_for_user",
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Fields:         fields,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "submit", Label: "生成保存草图", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
	s.storePendingInteraction(req, payload)
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               "请补充显示域后继续。",
		Workflow:            interaction.Workflow,
		WorkflowData:        payload,
		PluginLearning:      payload,
		InteractionRequests: []AgentInteractionRequest{req},
	}
}

func (s *Server) finalizePluginGrabberDisplayDomainInteraction(ctx context.Context, interaction PendingInteraction, submitPayload map[string]any) (ChatResponse, error) {
	payload := copyStringAnyMap(interaction.Payload)
	fields := mapValue(submitPayload["fields"])
	reviews := mapRowsValue(firstPresentAny(payload, "display_domain_reviews", "display_domains"))
	reviews = append(reviews, pluginGrabberDisplayDomainReviews(payload, fields)...)
	nextPayload := map[string]any{
		"reviewed_profile_patch": payload["profile_patch"],
		"display_domain_reviews": reviews,
	}
	return s.runPluginGrabberInteractionWorkflow(ctx, interaction, nextPayload, "auto_learn_reviewed")
}

func (s *Server) runPluginGrabberInteractionWorkflow(ctx context.Context, interaction PendingInteraction, payload map[string]any, mode string) (ChatResponse, error) {
	cfg, _, err := config.Load()
	if err != nil {
		return ChatResponse{}, err
	}
	base := mapValue(interaction.Payload)
	target := mapValue(base["target"])
	args := map[string]any{
		"mode":        mode,
		"track_id":    firstNonEmptyText(target, "track_id"),
		"plugin_id":   firstNonEmptyText(target, "plugin_id"),
		"plugin_name": firstNonEmptyText(target, "plugin_name"),
	}
	if args["track_id"] == "" {
		args["track_id"] = firstNonEmptyText(base, "track_id")
	}
	if args["plugin_id"] == "" {
		args["plugin_id"] = firstNonEmptyText(base, "plugin_id")
	}
	if args["plugin_name"] == "" {
		args["plugin_name"] = firstNonEmptyText(base, "plugin_name")
	}
	if uiReference := firstPresentAny(base, "ui_reference"); uiReference != nil {
		args["ui_reference"] = uiReference
	}
	if uiReferenceArtifacts := firstPresentAny(base, "ui_reference_artifacts"); uiReferenceArtifacts != nil {
		args["ui_reference_artifacts"] = uiReferenceArtifacts
	}
	if mode == "auto_learn_reviewed" {
		if patch := firstPresentAny(payload, "reviewed_profile_patch", "profile_patch"); patch != nil {
			args["reviewed_profile_patch"] = patch
		} else if patch := base["profile_patch"]; patch != nil {
			args["reviewed_profile_patch"] = patch
		}
		if reviews := firstPresentAny(payload, "display_domain_reviews", "display_domains"); reviews != nil {
			args["display_domain_reviews"] = reviews
		}
	}
	resp := s.runPluginGrabberLearningWorkflow(ctx, interaction.ConversationID, "", interaction.RequestContext, cfg, args)
	resp.GoalID = firstNonEmpty(resp.GoalID, interaction.GoalID)
	resp.RunID = firstNonEmpty(resp.RunID, interaction.RunID)
	return resp, nil
}

func (s *Server) setPluginParamNormalized(ctx context.Context, target map[string]any, paramID string, value float64) error {
	if s.kernel == nil {
		return fmt.Errorf("kernel client is nil")
	}
	trackID := firstNonEmptyText(target, "track_id")
	pluginID := firstNonEmptyText(target, "plugin_id")
	paramID = strings.TrimSpace(paramID)
	if trackID == "" || pluginID == "" || paramID == "" {
		return fmt.Errorf("实验参数目标不完整")
	}
	reply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":       "set_plugin_param",
		"track_id":  trackID,
		"plugin_id": pluginID,
		"param_id":  paramID,
		"value":     value,
	})
	if err != nil {
		return err
	}
	if !kernelReplyOK(reply) {
		message := firstNonEmptyText(reply, "message", "error")
		if message == "" {
			message = "set_plugin_param failed"
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func pluginGrabberDisplayDomainFields(payload map[string]any) []AgentInteractionField {
	fields := []AgentInteractionField{}
	seen := map[string]bool{}
	for i, answer := range mapRowsValue(payload["experiment_answers"]) {
		experiment := mapValue(answer["experiment"])
		paramID := firstNonEmptyText(experiment, "param_id")
		if paramID == "" {
			continue
		}
		key := pluginGrabberDisplayReviewKey(firstNonEmptyText(experiment, "component_id"), firstNonEmptyText(experiment, "slot"), paramID)
		seen[key] = true
		value := firstNonEmptyText(answer, "display_domain_text")
		if value == "" {
			value = firstNonEmptyText(experiment, "inferred_display_domain_text", "display_domain_text")
		}
		fields = append(fields, AgentInteractionField{
			ID:          fmt.Sprintf("display_domain_%d", i),
			Label:       firstNonEmptyText(experiment, "label", "param_id"),
			Kind:        "text",
			Required:    false,
			Value:       value,
			Placeholder: "例如：20~20000 Hz / -18~18 dB / 0~100 %",
			Payload: map[string]any{
				"component_id":    experiment["component_id"],
				"operation_name":  experiment["operation_name"],
				"slot":            experiment["slot"],
				"param_id":        paramID,
				"value_text":      experiment["value_text"],
				"observation":     answer["observation"],
				"provenance_kind": "active_roundtrip_probe",
			},
		})
	}
	fields = appendPluginGrabberUnresolvedDisplayDomainFields(fields, payload, seen)
	return fields
}

func appendPluginGrabberUnresolvedDisplayDomainFields(fields []AgentInteractionField, payload map[string]any, seen map[string]bool) []AgentInteractionField {
	patch := mapValue(payload["profile_patch"])
	if len(patch) == 0 && payload["profile_patch"] != nil {
		patch = mapFromJSONStruct(payload["profile_patch"])
	}
	for _, group := range mapRowsValue(patch["groups"]) {
		componentID := firstNonEmptyText(group, "id", "component_id")
		componentLabel := firstNonEmptyText(group, "label", "name", "id")
		params := mapValue(group["params"])
		for rawSlot, rawMapping := range params {
			slot := strings.TrimSpace(fmt.Sprint(rawSlot))
			mapping := mapValue(rawMapping)
			paramID := firstNonEmptyText(mapping, "param_id", "id")
			if paramID == "" {
				paramID = strings.TrimSpace(fmt.Sprint(rawMapping))
			}
			if paramID == "" || paramID == "<nil>" || !pluginGrabberMappingNeedsDisplayReview(mapping) {
				continue
			}
			key := pluginGrabberDisplayReviewKey(componentID, slot, paramID)
			if seen[key] {
				continue
			}
			seen[key] = true
			label := strings.TrimSpace(strings.Join([]string{componentLabel, slot, firstNonEmptyText(mapping, "label", "name", "param_id")}, " / "))
			fields = append(fields, AgentInteractionField{
				ID:          fmt.Sprintf("display_domain_unresolved_%d", len(fields)),
				Label:       label,
				Kind:        "text",
				Required:    false,
				Value:       pluginGrabberMappingDomainText(mapping),
				Placeholder: "例如：20~20000 Hz / -18~18 dB / 0~100 %",
				Payload: map[string]any{
					"component_id":    componentID,
					"slot":            slot,
					"param_id":        paramID,
					"provenance_kind": "user_review",
				},
			})
		}
	}
	return fields
}

func pluginGrabberPayloadWithSubmittedReviews(base, submitted map[string]any) map[string]any {
	payload := copyStringAnyMap(base)
	for key, value := range submitted {
		payload[key] = value
	}
	reviews := firstPresentAny(payload, "display_domain_reviews", "display_domains")
	if reviews == nil {
		return payload
	}
	patch, err := reviewedProfilePatchFromArgs(map[string]any{"reviewed_profile_patch": payload["profile_patch"]})
	if err == nil {
		applyReviewedDisplayDomains(&patch, reviews)
		payload["profile_patch"] = patch
	}
	return payload
}

func pluginGrabberMappingNeedsDisplayReview(mapping map[string]any) bool {
	if len(mapping) == 0 || boolValue(mapping["confirmed"]) {
		return false
	}
	domain := mapValue(mapping["display_domain"])
	if len(domain) == 0 {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(firstNonEmptyText(domain, "status")))
	if status == "unknown" || status == "needs_confirmation" {
		return true
	}
	confidence := floatNumber(domain["confidence"])
	return confidence > 0 && confidence < 0.80
}

func pluginGrabberMappingDomainText(mapping map[string]any) string {
	if text := firstNonEmptyText(mapping, "display_domain_text", "display_range", "display_unit", "unit", "range"); text != "" {
		return text
	}
	domain := mapValue(mapping["display_domain"])
	if text := firstNonEmptyText(domain, "text"); text != "" {
		return text
	}
	unit := firstNonEmptyText(domain, "unit")
	minText := strings.TrimSpace(fmt.Sprint(domain["min"]))
	maxText := strings.TrimSpace(fmt.Sprint(domain["max"]))
	if minText != "" && minText != "<nil>" && maxText != "" && maxText != "<nil>" {
		return strings.TrimSpace(minText + "~" + maxText + " " + unit)
	}
	return unit
}

func pluginGrabberDisplayReviewKey(componentID, slot, paramID string) string {
	return strings.TrimSpace(componentID) + "|" + strings.TrimSpace(slot) + "|" + strings.TrimSpace(paramID)
}

func pluginGrabberDisplayDomainReviews(payload map[string]any, fields map[string]any) []map[string]any {
	reviews := []map[string]any{}
	formFields := pluginGrabberDisplayDomainFields(payload)
	for _, field := range formFields {
		text := strings.TrimSpace(fmt.Sprint(fields[field.ID]))
		if text == "" || text == "<nil>" {
			continue
		}
		review := copyStringAnyMap(field.Payload)
		review["display_domain_text"] = text
		reviews = append(reviews, review)
	}
	return reviews
}

func pluginGrabberInteractionErrorResponse(interaction PendingInteraction, err error) ChatResponse {
	message := friendlyExecutionError(err)
	if err != nil && strings.Contains(err.Error(), "恢复实验参数失败") {
		message = "实验参数恢复失败，已停止学习流程。请手动检查插件参数。"
	}
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          message,
		Workflow:       interaction.Workflow,
		WorkflowData:   interaction.Payload,
		Error:          errorString(err),
		InteractionRequests: []AgentInteractionRequest{{
			ID:             "interaction_" + randomID(),
			Kind:           "mode_boundary",
			Source:         interaction.Source,
			Workflow:       interaction.Workflow,
			Stage:          "error",
			Title:          "学习已停止",
			Body:           message,
			Status:         "error",
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Payload:        interaction.Payload,
			Actions:        []AgentInteractionAction{{ID: "done", Label: "完成", Style: "primary", Recommended: true}},
		}},
	}
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
	plan, ok, stale := s.takePendingPlan(planID)
	if !ok {
		if stale {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":      "ok",
				"message":     "这个确认已处理或已过期。",
				"plan_id":     planID,
				"goal_status": string(agentruntime.StatusCompleted),
			})
			return
		}
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
		if data := pluginGrabberLearningCompletionData(plan, false); data != nil {
			response["workflow"] = plan.Workflow
			response["workflow_data"] = data
			response["plugin_learning"] = data
			response["message"] = "已取消 Plugin Grabber 学习保存。"
			attachPluginGrabberCompletionAssets(response, data)
		}
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
	projectResultCards := projectResultCardsFromExecuted(replies)
	historyData := map[string]any{}
	if len(projectResultCards) > 0 {
		historyData["project_result_cards"] = projectResultCards
	}
	projectHistory := s.harness.RecordConversationNodeForProjectWithData(r.Context(), projectPath, "vit", message, goalID, runID, historyData)
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(r.Context(), goalID, projectPath)
	}
	response := map[string]any{
		"status":                "ok",
		"message":               message,
		"plan_id":               planID,
		"goal_id":               goalID,
		"run_id":                runID,
		"agent_mode":            agentMode,
		"replies":               replies,
		"executed_kernel_reply": replies,
		"project_result_cards":  projectResultCards,
		"project_history":       projectHistory,
		"goal_status":           string(agentruntime.StatusCompleted),
	}
	if data := pluginGrabberLearningCompletionData(plan, true); data != nil {
		response["workflow"] = plan.Workflow
		response["workflow_data"] = data
		response["plugin_learning"] = data
		attachPluginGrabberCompletionAssets(response, data)
	}
	if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCompleted, "", "", "", projectHistory)); plan != nil {
		response["agent_plan"] = plan
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) resolvePendingPlanDecision(ctx context.Context, planID, decision string) (int, map[string]any) {
	planID = strings.TrimSpace(planID)
	plan, ok, stale := s.takePendingPlan(planID)
	if !ok {
		if stale {
			return http.StatusOK, map[string]any{
				"status":      "ok",
				"message":     "这个交互已处理或已过期。",
				"plan_id":     planID,
				"goal_status": string(agentruntime.StatusCompleted),
			}
		}
		return http.StatusNotFound, map[string]any{"status": "error", "message": "plan not found"}
	}
	goalID, runID := goalIDsFromContext(plan.Context)
	projectPath := projectPathFromChatContext(plan.Context)
	agentMode := agentModeFromContext(plan.Context)
	cleanDecision := strings.ToLower(strings.TrimSpace(decision))
	if plan.Workflow == agentLoopConfirmationWorkflow {
		return s.resolveAgentLoopConfirm(ctx, planID, plan, cleanDecision)
	}
	if cleanDecision != "approve" && cleanDecision != "allow" && cleanDecision != "confirm" && cleanDecision != "yes" && cleanDecision != "submit" && cleanDecision != "save" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		projectHistory := s.harness.RecordConversationNodeForProject(ctx, projectPath, "vit", "cancelled", goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		}
		response := map[string]any{"status": "ok", "message": "cancelled", "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": agentMode, "goal_status": string(agentruntime.StatusCancelled), "project_history": projectHistory}
		if data := pluginGrabberLearningCompletionData(plan, false); data != nil {
			response["workflow"] = plan.Workflow
			response["workflow_data"] = data
			response["plugin_learning"] = data
			response["message"] = "已取消 Plugin Grabber 学习保存。"
			attachPluginGrabberCompletionAssets(response, data)
		}
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		return http.StatusOK, response
	}
	beforeState := s.harness.UserStateSummary(ctx)
	replies, err := s.executeDecisions(ctx, plan.Decisions, true, plan.Context)
	if err != nil {
		s.harness.CompleteGoal(goalID, err)
		projectHistory := s.harness.RecordConversationNodeForProject(ctx, projectPath, "vit", err.Error(), goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
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
		return http.StatusOK, response
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(ctx), plan.Decisions, replies)
	if plan.Workflow == pluginGrabberLoadCommand {
		message, replies = s.finishPluginGrabberLoadWorkflow(ctx, plan, replies, message)
	}
	if strings.TrimSpace(message) == "" {
		message = "done"
	}
	s.harness.CompleteGoal(goalID, nil)
	projectResultCards := projectResultCardsFromExecuted(replies)
	historyData := map[string]any{}
	if len(projectResultCards) > 0 {
		historyData["project_result_cards"] = projectResultCards
	}
	projectHistory := s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, "vit", message, goalID, runID, historyData)
	if len(projectHistory) == 0 {
		projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
	}
	response := map[string]any{
		"status":                "ok",
		"message":               message,
		"plan_id":               planID,
		"goal_id":               goalID,
		"run_id":                runID,
		"agent_mode":            agentMode,
		"replies":               replies,
		"executed_kernel_reply": replies,
		"project_result_cards":  projectResultCards,
		"project_history":       projectHistory,
		"goal_status":           string(agentruntime.StatusCompleted),
	}
	if data := pluginGrabberLearningCompletionData(plan, true); data != nil {
		response["workflow"] = plan.Workflow
		response["workflow_data"] = data
		response["plugin_learning"] = data
		attachPluginGrabberCompletionAssets(response, data)
	}
	if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCompleted, "", "", "", projectHistory)); plan != nil {
		response["agent_plan"] = plan
	}
	return http.StatusOK, response
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
			Reply:          "工具命令解析失败：" + err.Error(),
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
	if invokeErr == nil && resp.Status == "needs_confirmation" {
		if confirmResp, ok := s.chatToolConfirmationResponse(conversationID, req, tool, args, resp); ok {
			return confirmResp, true
		}
	}
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          renderChatToolResult(resp),
		Error:          errorString(invokeErr),
	}, true
}

func (s *Server) chatToolConfirmationResponse(conversationID string, req ChatRequest, tool string, args map[string]any, invokeResp harness.InvokeResponse) (ChatResponse, bool) {
	command, ok := commandFromChatTool(tool, args)
	if !ok {
		return ChatResponse{}, false
	}
	decisions := policy.Analyze([]map[string]any{command})
	if len(decisions) == 0 || !policy.NeedsConfirmation(decisions) {
		return ChatResponse{}, false
	}
	preview := strings.TrimSpace(invokeResp.Preview)
	plan := PendingPlan{
		ID:        "plan_" + randomID(),
		CreatedAt: time.Now(),
		Decisions: decisions,
		Context:   req.Context,
		Preview:   preview,
	}
	s.mu.Lock()
	s.pending[plan.ID] = plan
	s.mu.Unlock()
	reply := confirmationReply(decisions)
	if strings.TrimSpace(reply) == "" {
		reply = fmt.Sprintf("工具 `%s` 需要你确认后才会执行。", firstNonEmpty(invokeResp.Tool, tool))
	}
	return ChatResponse{
		ConversationID:    conversationID,
		Reply:             reply,
		NeedsConfirmation: true,
		PlanID:            plan.ID,
		Preview:           preview,
		Commands:          decisions,
		ProjectHistory:    invokeResp.ProjectHistory,
	}, true
}

func commandFromChatTool(tool string, args map[string]any) (map[string]any, bool) {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return nil, false
	}
	if args == nil {
		args = map[string]any{}
	}
	catalog := tools.DefaultCatalog()
	if tool == "daw.invoke" {
		cmd := tools.CloneCommand(args)
		name := policy.CommandName(cmd)
		if name == "" {
			return nil, false
		}
		if spec, ok := catalog.LookupCommand(name); ok {
			cmd["cmd"] = spec.CommandName
			return cmd, true
		}
		if spec, ok := catalog.LookupTool(name); ok {
			cmd["cmd"] = spec.CommandName
			return cmd, true
		}
		return cmd, true
	}
	if spec, ok := catalog.LookupTool(tool); ok {
		return tools.BuildCommand(spec.CommandName, args), true
	}
	if spec, ok := catalog.LookupCommand(tool); ok {
		return tools.BuildCommand(spec.CommandName, args), true
	}
	return nil, false
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
	resp := ChatResponse{
		ConversationID:    conversationID,
		Reply:             "Goal UI smoke is waiting for confirmation.",
		NeedsConfirmation: true,
		PlanID:            planID,
		Preview:           preview,
	}
	s.attachInteractionRequests(&resp)
	return resp
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

func firstPresentAny(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func mapValue(value any) map[string]any {
	switch x := value.(type) {
	case map[string]any:
		return x
	case map[string]string:
		out := make(map[string]any, len(x))
		for k, v := range x {
			out[k] = v
		}
		return out
	default:
		return map[string]any{}
	}
}

func copyStringAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringListValue(value any) []string {
	switch x := value.(type) {
	case []string:
		return append([]string{}, x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	case []map[string]any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			text := firstNonEmptyText(item, "message", "label", "name", "id")
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" || text == "<nil>" {
			return nil
		}
		return []string{text}
	}
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
			"工具 `%s` 需要确认。\n预览：\n%s\n请在确认卡片中选择“确认执行”或“取消”。",
			resp.Tool,
			resp.Preview,
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
		return fmt.Sprintf("工具结果：%s", resp.Status)
	}
	out := strings.TrimRight(buf.String(), "\n")
	if len([]rune(out)) > 3000 {
		out = string([]rune(out)[:3000]) + "\n...<truncated>"
	}
	return "工具结果：\n```json\n" + out + "\n```"
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
	return s.buildAssembly(ctx, conversationID, userText, requestContext).Messages
}

func (s *Server) buildAssembly(ctx context.Context, conversationID, userText string, requestContext map[string]any) promptruntime.Assembly {
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
Attachments appear in current_selection.attachments, and their artifact refs appear in current_selection.artifacts. Use artifact.read or artifact.extract when the user asks about attachment/web artifact contents. Documents may include extracted text in the artifact digest; answer from that extracted text when available. Do not claim to inspect raw images, audio, or video beyond the available artifact digest/metadata. Video attachments are local media previews only unless a future explicit video-analysis digest is available. For an audio attachment the user wants placed in a track, use clip.import_media_to_track. For a MIDI attachment the user wants imported, use midi.import_file with file_path, track_id, start_time_beats:0, mode:"merge_tracks". Browser WebView pages are only readable after the user explicitly captures the page into a web_page artifact.
When the user gives a local asset file or folder and asks what media is available, use media.register_assets or media.index_authorized_folder so the response can include clickable media pool artifact cards. Do not replace artifact cards with long raw path lists.
For MIDI reading, use midi.read_clip_notes with clip_id from piano_roll_focus_clip_id first, then selected_clip_id, when the user asks to inspect or describe the current MIDI clip.
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
When the user says "this clip", "current clip", or "selected clip", use piano_roll_focus_clip_id first for MIDI editor tasks, then selected_clip_id or selected_clip_ids from current_selection. If no clip is selected and multiple clips exist, ask the user to select or name one instead of guessing.
When the user says "at the playhead", use playhead_seconds from current_selection.
When importing audio and no target track is named, use selected_track_id as the target. If selected_track_id is absent and multiple tracks exist, ask the user which track to import into.
If commands is non-empty, keep reply as a short internal intent summary. VitAgent will replace it with the final user-facing result after execution, so do not rely on "about to" wording as the final answer.

For plugin loading/grabber setup requests such as loading TDR Nova, finding an EQ/compressor, or loading a plugin and grabbing useful controls, use the special chat workflow command {"cmd":"plugin_grabber_load_and_get_params","track_id":"...","plugin_query":"TDR Nova","intent":"short user intent"}. This workflow searches indexed plugins, asks for confirmation before loading a rack node, then reads parameters after the load succeeds. Do not use instantiate_plugin for these requests; instantiate_plugin requires an exact plugin_path and bypasses the rack grabber workflow.
For project-scoped plugin grabber learning requests such as learning a plugin, saving quick controls, grouping plugin parameters, or improving plugin control names on an already loaded/selected plugin, use the special chat workflow command {"cmd":"plugin_grabber_learn_project_profile","track_id":"...","plugin_id":"...","intent":"short user intent"}. This workflow is agent-side: it first reads full parameters, asks AI for a profile patch, validates parameter IDs, then asks the user to confirm before saving. Do not use it for ordinary parameter value changes.
For plugin grabber explanation, summary, context pack, or "explain controls" requests on an already loaded/selected plugin, use the special read-only workflow command {"cmd":"plugin_grabber_explain_controls","track_id":"...","plugin_id":"...","intent":"short user intent"}. This workflow reads full parameters, then returns a compact context pack with quick controls, groups, roles, and full-parameter access hints. It does not filter or save parameters.
For basic macro-control creation requests such as creating a generic macro knob/slider, use {"cmd":"control_add_macro","track_id":"...","name":"Macro","control_type":"slider","value":0.5,"bindings":[]}. Do not use rack.add_macro. Semantic macro generation from plugin skills should be proposed for confirmation before writing bindings.
For macro-control rename requests, use {"cmd":"control_rename_macro","macro_id":"...","name":"New Macro Name"}. If the user names the macro by visible label, resolve it from macro_refs or available_macro_controls; do not create a new macro to rename one.
When the user asks to bind/map a plugin parameter to an existing macro control, such as "bind B1 Gain to Macro 1", "bind it to this macro", or "绑定到已有宏控件", do not call control_add_macro first. Use the existing macro_id from macro_refs or available_macro_controls and call {"cmd":"control_add_binding","macro_id":"...","track_id":"...","plugin_id":"...","param_id":"...","param_name":"...","target_min":...,"target_max":...}. If the named macro is ambiguous or absent, ask which macro to use instead of creating a new one.
For runtime acoustic plugin adjustments on an already learned plugin, such as "cut 500Hz mud", "boost presence", "reduce harshness", or similar mixing targets, use {"cmd":"plugin_grabber_apply_control","track_id":"...","plugin_id":"...","control":"eq.cut_region|eq.boost_region|eq.set_region","target":{"freq_hz":500,"gain_db":-2.5,"q":1.1}}. Do not use set_plugin_param/plugin.set_parameter for these acoustic targets unless the user explicitly gives an exact param_id and raw value. If the profile is missing or stale, the command will report that Get Param/Learn is needed.
For explicit one-parameter plugin control where the user gives a concrete param_id and a display value/unit, and get_plugin_parameters display_probe evidence is high confidence, set_plugin_param/plugin.set_parameter may use value_text such as "1000 ms" or "28 percent" without a saved profile. Do not use this for semantic mixing, multi-parameter control, or automatic mixing.
For plugin_grabber_apply_control results, treat applied_parameters[].new_value_text, applied_value, and confirmed display_domain data as the evidence. Do not infer a control's min/max from the current value_text snapshot or advisory safety notes.

Mode instruction:
%s

Available DAW command catalog:
%s`, modeInstruction, catalog)

	return promptruntime.Build(promptruntime.AssemblyInput{
		SystemSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionStatic, "chat_system", "", system, true),
			promptruntime.TextSection(promptruntime.SectionRuntime, "chat_context_snapshot", "Context snapshot JSON", snapshot.JSON(), false),
		},
		UserSections: []promptruntime.Section{
			promptruntime.TextSection(promptruntime.SectionCurrentUser, "chat_current_user", "", userText, false),
		},
	})
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
				"role":                 item.Role,
				"content":              item.Content,
				"node_id":              item.NodeID,
				"commit_id":            item.CommitID,
				"branch":               item.Branch,
				"artifacts":            item.Artifacts,
				"project_result_cards": item.ProjectCards,
				"created_at":           item.CreatedAt,
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

func (s *Server) confirmationPreview(ctx context.Context, decisions []policy.Decision, requestContext map[string]any) (previewText string, err error) {
	started := time.Now()
	defer func() {
		if s != nil && s.logger != nil {
			s.logger.Info("[timing] confirmation_preview ms=%d decisions=%d preview_len=%d err=%t",
				time.Since(started).Milliseconds(), len(decisions), len(previewText), err != nil)
		}
	}()
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
		parts = append([]string{"确认后，VitAgent 会先创建一个项目历史安全检查点。"}, parts...)
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

func (s *Server) executeDecisions(ctx context.Context, decisions []policy.Decision, confirmed bool, requestContext map[string]any) (replies []map[string]any, err error) {
	started := time.Now()
	defer func() {
		if s != nil && s.logger != nil {
			s.logger.Info("[timing] execute_decisions total_ms=%d decisions=%d replies=%d confirmed=%t err=%t",
				time.Since(started).Milliseconds(), len(decisions), len(replies), confirmed, err != nil)
		}
	}()
	replies = make([]map[string]any, 0, len(decisions))
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
			return replies, fmt.Errorf("命令需要确认：%s", resp.CommandName)
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
	for _, key := range []string{"selected_track_id", "selected_track_name", "selected_scene_track_id", "selected_clip_id", "selected_clip_track_id", "piano_roll_focus_clip_id", "piano_roll_focus_track_id", "selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_plugin_source", "playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "selected_library_file_path", "selected_library_item_name", "selected_library_kind", "library_search_query"} {
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

func mapRowsFromAny(v any) []map[string]any {
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

func firstMapFromAny(v any) map[string]any {
	if row, ok := v.(map[string]any); ok {
		return row
	}
	return nil
}

func firstStringFromMap(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func safeUploadName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "upload.bin"
	}
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, base)
	clean = strings.Trim(clean, "._-")
	if clean == "" {
		return "upload.bin"
	}
	return clean
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
		return "Ask Vit is in Plan mode. You may inspect, analyze, explain, search, and draft steps, but you must not directly modify the DAW project file or live DAW state. Emit only read-only commands. If the user asks for a change, describe the plan and leave mutating commands empty. If you refuse a direct execution request, say Plan mode is read-only and the change was not executed; do not say the DAW lacks that capability or that the tool does not exist."
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

func contextWithConversationID(in map[string]any, conversationID string) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if strings.TrimSpace(conversationID) != "" {
		out["conversation_id"] = strings.TrimSpace(conversationID)
	}
	return out
}

func goalIDsFromContext(in map[string]any) (string, string) {
	if in == nil {
		return "", ""
	}
	return cleanContextText(in["goal_id"]), cleanContextText(in["run_id"])
}

func cleanContextText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
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

func (s *Server) artifactStore() artifacts.Store {
	if s != nil && strings.TrimSpace(s.artifactRoot) != "" {
		return artifacts.NewStore(s.artifactRoot)
	}
	return artifacts.NewStore("")
}

func (s *Server) webUIRootPath() string {
	if s != nil && strings.TrimSpace(s.webUIRoot) != "" {
		if st, err := os.Stat(s.webUIRoot); err == nil && st.IsDir() {
			return filepath.Clean(s.webUIRoot)
		}
		return ""
	}
	for _, root := range webUISearchRoots() {
		if found := findWebUIRootFrom(root); found != "" {
			return found
		}
	}
	return ""
}

func webUISearchRoots() []string {
	var roots []string
	wd, err := os.Getwd()
	if err == nil && strings.TrimSpace(wd) != "" {
		roots = append(roots, wd)
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		roots = append(roots, filepath.Dir(exe))
	}
	return roots
}

func findWebUIRootFrom(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	for dir := filepath.Clean(root); dir != ""; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "agent" {
			candidate := filepath.Join(dir, "webui", "dist")
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func (s *Server) storeUploadedArtifact(store artifacts.Store, uploadDir string, header *multipart.FileHeader, conversationID, goalID, runID string, scope artifacts.Scope, metadata map[string]any) (artifacts.Summary, error) {
	if header == nil {
		return artifacts.Summary{}, fmt.Errorf("upload file is missing")
	}
	src, err := header.Open()
	if err != nil {
		return artifacts.Summary{}, err
	}
	defer src.Close()
	id := "art_" + randomID()
	name := safeUploadName(header.Filename)
	path := filepath.Join(uploadDir, id+"_"+name)
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return artifacts.Summary{}, err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return artifacts.Summary{}, err
	}
	if err := dst.Close(); err != nil {
		return artifacts.Summary{}, err
	}
	a := artifacts.ArtifactFromFile(path, id, conversationID, goalID, runID)
	a = artifacts.ApplyScope(a, scope)
	if strings.TrimSpace(header.Header.Get("Content-Type")) != "" {
		a.MIME = strings.TrimSpace(header.Header.Get("Content-Type"))
	}
	if len(metadata) > 0 {
		if a.Metadata == nil {
			a.Metadata = map[string]any{}
		}
		for key, value := range metadata {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				a.Metadata[key] = text
			}
		}
	}
	a = artifacts.Extract(a, artifacts.DefaultTextLimit)
	stored, err := store.Upsert(a)
	if err != nil {
		return artifacts.Summary{}, err
	}
	return stored.CompactSummary(), nil
}

func (s *Server) artifactsFromRefs(refs []string) []artifacts.Summary {
	if len(refs) == 0 {
		return nil
	}
	store := s.artifactStore()
	out := make([]artifacts.Summary, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		id := strings.TrimSpace(ref)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		a, err := store.Get(id)
		if err != nil {
			continue
		}
		out = append(out, a.CompactSummary())
	}
	return out
}

func (s *Server) artifactsFromAttachments(attachments []Attachment, conversationID, goalID, runID string, scope artifacts.Scope) []artifacts.Summary {
	rows := summarizeAttachments(attachments)
	if len(rows) == 0 {
		return nil
	}
	store := s.artifactStore()
	out := make([]artifacts.Summary, 0, len(rows))
	for _, row := range rows {
		path := strings.TrimSpace(fmt.Sprint(row["path"]))
		if path == "" {
			continue
		}
		id := artifactIDFromAttachment(row)
		a := artifacts.ArtifactFromFile(path, id, conversationID, goalID, runID)
		a = artifacts.ApplyScope(a, scope)
		if kind := strings.TrimSpace(fmt.Sprint(row["kind"])); kind != "" {
			a.Kind = artifactKindFromAttachment(kind, a.Kind)
		}
		a = artifacts.Extract(a, artifacts.DefaultTextLimit)
		stored, err := store.Upsert(a)
		if err != nil {
			continue
		}
		out = append(out, stored.CompactSummary())
	}
	return out
}

func contextWithArtifactSummaries(in map[string]any, summaries []artifacts.Summary) map[string]any {
	if len(summaries) == 0 {
		return in
	}
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	rows := make([]map[string]any, 0, len(summaries))
	ids := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		row := artifactSummaryRow(summary)
		if len(row) == 0 {
			continue
		}
		rows = append(rows, row)
		ids = append(ids, summary.ID)
	}
	if len(rows) == 0 {
		return out
	}
	out["artifacts"] = rows
	out["artifact_ids"] = ids
	if len(rows) == 1 {
		out["artifact"] = rows[0]
		out["selected_artifact_id"] = ids[0]
	}
	return out
}

func mergeArtifactSummaries(existing, extra []artifacts.Summary) []artifacts.Summary {
	if len(extra) == 0 {
		return existing
	}
	seen := map[string]bool{}
	out := make([]artifacts.Summary, 0, len(existing)+len(extra))
	for _, item := range existing {
		if strings.TrimSpace(item.ID) == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	for _, item := range extra {
		if strings.TrimSpace(item.ID) == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	return out
}

func artifactSummaryRow(summary artifacts.Summary) map[string]any {
	out := map[string]any{
		"id":     summary.ID,
		"kind":   summary.Kind,
		"title":  summary.Title,
		"source": summary.Source,
		"status": summary.Status,
	}
	for key, value := range map[string]any{
		"path":              summary.Path,
		"url":               summary.URL,
		"mime":              summary.MIME,
		"size_bytes":        summary.SizeBytes,
		"summary":           summary.Summary,
		"metadata":          summary.Metadata,
		"created_at":        summary.CreatedAt,
		"conversation_id":   summary.ConversationID,
		"goal_id":           summary.GoalID,
		"run_id":            summary.RunID,
		"project_path":      summary.ProjectPath,
		"root_project_path": summary.RootProjectPath,
		"active_worktree":   summary.ActiveWorktree,
		"active_branch":     summary.ActiveBranch,
		"active_node_id":    summary.ActiveNodeID,
		"history_scope_key": summary.HistoryScopeKey,
		"media_scope_key":   summary.MediaScopeKey,
	} {
		if !isEmptyContextValue(map[string]any{"v": value}, "v") {
			out[key] = value
		}
	}
	return out
}

func artifactSummaryRows(summaries []artifacts.Summary) []map[string]any {
	if len(summaries) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(summaries))
	for _, summary := range summaries {
		if strings.TrimSpace(summary.ID) == "" {
			continue
		}
		out = append(out, artifactSummaryRow(summary))
	}
	return out
}

func artifactIDFromAttachment(row map[string]any) string {
	id := strings.TrimSpace(fmt.Sprint(row["id"]))
	if id == "" {
		id = strings.TrimSpace(fmt.Sprint(row["path"]))
	}
	id = strings.Trim(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, id), "_")
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "art_") {
		return id
	}
	return "art_" + id
}

func artifactKindFromAttachment(kind, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "audio", "midi", "image", "text", "document", "video", "unknown":
		return strings.ToLower(strings.TrimSpace(kind))
	case "plugin":
		return "file"
	default:
		return fallback
	}
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
	case ".pdf", ".doc", ".docx":
		return "document"
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".ogv":
		return "video"
	}
	kind := strings.ToLower(strings.TrimSpace(raw))
	switch kind {
	case "audio", "midi", "plugin", "image", "text", "document", "video", "unknown":
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

func revealLocalPath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." {
		return fmt.Errorf("path is empty")
	}
	info, err := os.Stat(clean)
	if err != nil {
		return err
	}
	switch goruntime.GOOS {
	case "windows":
		if info.IsDir() {
			return exec.Command("explorer.exe", clean).Start()
		}
		return exec.Command("explorer.exe", "/select,"+clean).Start()
	case "darwin":
		return exec.Command("open", "-R", clean).Start()
	default:
		target := clean
		if !info.IsDir() {
			target = filepath.Dir(clean)
		}
		return exec.Command("xdg-open", target).Start()
	}
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
