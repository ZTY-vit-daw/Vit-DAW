package chat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/browsercapture"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/macrocontrols"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/orchestrationruntime"
	"vit-daw-agent/internal/pendingmanager"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/resourceintake"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

type Server struct {
	kernel                  *kernel.Client
	auditionKernel          auditionCommandClient
	auditionCandidateDriver auditionCandidateProjectDriver
	eqKernelOverride        eqKernelTransport
	shadow                  *shadow.Project
	llm                     *llm.Client
	logger                  *logx.Logger
	harness                 *harness.Harness
	artifactRoot            string
	webUIRoot               string
	startedAt               time.Time

	mu                                 sync.Mutex
	conversations                      map[string][]llm.Message
	pending                            map[string]PendingPlan
	interactions                       map[string]PendingInteraction
	mixSessions                        map[string]MixSession
	goalContinuations                  map[string]agentloop.Continuation
	durableContinuations               map[string]DurableContinuation
	capabilityRoutes                   map[string]CapabilityRouteRecord
	schedulerCtx                       context.Context
	schedulerCancel                    context.CancelFunc
	schedulerOnce                      sync.Once
	schedulerWake                      chan struct{}
	schedulerDone                      chan struct{}
	schedulerStarted                   bool
	schedulerOwner                     string
	continuationLease                  time.Duration
	continuationExecutor               func(context.Context, DurableContinuation) error
	continuationPersist                func() error
	schedulerExecutionMu               sync.Mutex
	activeRuntimeInvocations           int
	lastWorkspaceRecoveryAttempt       time.Time
	conversationGoals                  map[string]string
	conversationMemory                 map[string]agentloop.ExecutionMemory
	pendingMixTicks                    map[string]agentloop.PendingMixTickCandidate
	pendingTreatments                  map[string]agentloop.MixTreatmentPending
	freeStateLoops                     map[string]freeStateReasoningLoop
	audioClosures                      *audioclosure.MemoryStore
	controllerOwners                   *orchestrationcontroller.Registry
	pendingManager                     *pendingmanager.MemoryManager
	orchestrationRuntime               *orchestrationruntime.Runtime
	pluginEffectControlRuntimeOverride func(context.Context, string, ChatRequest, agentruntime.Goal) ChatResponse
	uiContext                          map[string]any
	events                             map[string][]AgentEvent
	eventSeq                           map[string]int64
	eqOperations                       map[string]eqOperationRecord
	processorCertificationMu           sync.Mutex
	processorCertificationJobs         map[string]processorCertificationJob
	processorCertificationLoadTokens   map[string]processorCertificationLoadAuthorization
	webUILogged                        bool
	workspaceMu                        sync.Mutex
	activeWorkspacePath                string
	activeWorkspaceUUID                string
	activeWorkspaceSessionID           string
	authorityMode                      string
}

type PendingPlan struct {
	ID               string                  `json:"id"`
	CreatedAt        time.Time               `json:"created_at"`
	Decisions        []policy.Decision       `json:"decisions"`
	Context          map[string]any          `json:"context,omitempty"`
	Preview          string                  `json:"preview"`
	Workflow         string                  `json:"workflow,omitempty"`
	WorkflowData     map[string]any          `json:"workflow_data,omitempty"`
	GoalContinuation *agentloop.Continuation `json:"goal_continuation,omitempty"`
}

type projectAgentRuntimeState struct {
	SchemaVersion        string                                       `json:"schema_version"`
	ProjectPath          string                                       `json:"project_path"`
	ProjectUUID          string                                       `json:"project_uuid"`
	SavedAt              time.Time                                    `json:"saved_at"`
	Conversations        map[string][]llm.Message                     `json:"conversations,omitempty"`
	Pending              map[string]PendingPlan                       `json:"pending,omitempty"`
	Interactions         map[string]PendingInteraction                `json:"interactions,omitempty"`
	MixSessions          map[string]MixSession                        `json:"mix_sessions,omitempty"`
	GoalContinuations    map[string]agentloop.Continuation            `json:"goal_continuations,omitempty"`
	DurableContinuations map[string]DurableContinuation               `json:"durable_continuations,omitempty"`
	CapabilityRoutes     map[string]CapabilityRouteRecord             `json:"capability_routes,omitempty"`
	ConversationGoals    map[string]string                            `json:"conversation_goals,omitempty"`
	ConversationMemory   map[string]agentloop.ExecutionMemory         `json:"conversation_memory,omitempty"`
	PendingMixTicks      map[string]agentloop.PendingMixTickCandidate `json:"pending_mix_ticks,omitempty"`
	// Decode-only migration fields. Runtime v1 never writes or executes these
	// legacy B2/B3 pending records; restore retires them as rejected audit data.
	PendingStaticBalancePlans map[string]agentloop.PendingStaticBalancePlan `json:"pending_static_balance_plans,omitempty"`
	PendingPanLayoutPlans     map[string]agentloop.PendingPanLayoutPlan     `json:"pending_pan_layout_plans,omitempty"`
	PendingTreatments         map[string]agentloop.MixTreatmentPending      `json:"pending_treatments,omitempty"`
	FreeStateLoops            map[string]freeStateReasoningLoop             `json:"free_state_reasoning_loops,omitempty"`
	AudioClosures             map[string]audioclosure.State                 `json:"minimal_audio_closures,omitempty"`
	ControllerOwners          map[string]orchestrationcontroller.Owner      `json:"orchestration_controller_owners,omitempty"`
	PendingCandidates         []agentprotocol.PendingCandidate              `json:"pending_candidates,omitempty"`
	GoalRuntime               agentruntime.Snapshot                         `json:"goal_runtime,omitempty"`
	AuthorityMode             string                                        `json:"authority_mode,omitempty"`
}

type ChatRequest struct {
	ConversationID string         `json:"conversation_id"`
	Message        string         `json:"message"`
	Context        map[string]any `json:"context,omitempty"`
	Attachments    []Attachment   `json:"attachments,omitempty"`
	ArtifactRefs   []string       `json:"artifact_refs,omitempty"`
	AuthorityMode  string         `json:"authority_mode,omitempty"`
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
	ConversationID            string                              `json:"conversation_id"`
	TaskID                    string                              `json:"task_id,omitempty"`
	GoalID                    string                              `json:"goal_id,omitempty"`
	RunID                     string                              `json:"run_id,omitempty"`
	SliceID                   string                              `json:"slice_id,omitempty"`
	OriginalIntent            string                              `json:"original_intent,omitempty"`
	Reply                     string                              `json:"reply"`
	AgentMode                 string                              `json:"agent_mode,omitempty"`
	AgentPlan                 *AgentPlan                          `json:"agent_plan,omitempty"`
	NeedsConfirmation         bool                                `json:"needs_confirmation"`
	PlanID                    string                              `json:"plan_id,omitempty"`
	Preview                   string                              `json:"preview,omitempty"`
	ProposalPresentation      *orchestration.ProposalPresentation `json:"proposal_presentation,omitempty"`
	Workflow                  string                              `json:"workflow,omitempty"`
	WorkflowData              map[string]any                      `json:"workflow_data,omitempty"`
	MixSession                map[string]any                      `json:"mix_session,omitempty"`
	InteractionRequests       []AgentInteractionRequest           `json:"interaction_requests,omitempty"`
	TypedEvents               []map[string]any                    `json:"typed_events,omitempty"`
	AcousticPackageStatus     map[string]any                      `json:"acoustic_package_status,omitempty"`
	AcousticPackageStatusPath string                              `json:"acoustic_package_status_path,omitempty"`
	Commands                  []policy.Decision                   `json:"commands,omitempty"`
	ExecutedKernelReply       []map[string]any                    `json:"executed_kernel_reply,omitempty"`
	ProjectResultCards        []map[string]any                    `json:"project_result_cards,omitempty"`
	GoalStatus                string                              `json:"goal_status,omitempty"`
	GoalSummary               string                              `json:"goal_summary,omitempty"`
	CurrentStep               string                              `json:"current_step,omitempty"`
	CompletedSteps            int                                 `json:"completed_steps,omitempty"`
	StopReason                string                              `json:"stop_reason,omitempty"`
	LimitType                 string                              `json:"limit_type,omitempty"`
	ProjectHistory            map[string]any                      `json:"project_history,omitempty"`
	Artifacts                 []artifacts.Summary                 `json:"artifacts,omitempty"`
	SidePanelRequest          *SidePanelRequest                   `json:"side_panel_request,omitempty"`
	Error                     string                              `json:"error,omitempty"`
	Lifecycle                 string                              `json:"lifecycle,omitempty"`
	Persistence               string                              `json:"persistence,omitempty"`
	MessageKind               string                              `json:"message_kind,omitempty"`
	TurnID                    string                              `json:"turn_id,omitempty"`
	LogicalMessageID          string                              `json:"logical_message_id,omitempty"`
	Supersedes                []string                            `json:"supersedes,omitempty"`
}

type SidePanelRequest struct {
	View         string `json:"view,omitempty"`
	Tab          string `json:"tab,omitempty"`
	ArtifactID   string `json:"artifact_id,omitempty"`
	MacroPanelID string `json:"macro_panel_id,omitempty"`
}

func ensureChatResponseMessageProtocol(resp *ChatResponse) {
	if resp == nil {
		return
	}
	if strings.TrimSpace(resp.Lifecycle) == "" {
		resp.Lifecycle = "durable"
	}
	if strings.TrimSpace(resp.Persistence) == "" {
		resp.Persistence = "project_history"
	}
	if strings.TrimSpace(resp.MessageKind) == "" {
		resp.MessageKind = chatResponseMessageKind(*resp)
	}
	if strings.TrimSpace(resp.TurnID) == "" {
		resp.TurnID = firstNonEmpty(strings.TrimSpace(resp.RunID), strings.TrimSpace(resp.GoalID))
	}
	proposalID := ""
	if resp.ProposalPresentation != nil {
		proposalID = strings.TrimSpace(resp.ProposalPresentation.ProposalID)
	}
	if proposalID == "" {
		proposalID = firstNonEmpty(
			cleanContextText(resp.WorkflowData["proposal_id"]),
			cleanContextText(resp.WorkflowData["plan_id"]),
		)
	}
	if resp.MessageKind == "proposal" && strings.TrimSpace(resp.LogicalMessageID) == "" && proposalID != "" {
		resp.LogicalMessageID = proposalID
	}
	if resp.MessageKind != "proposal" && proposalID != "" && !containsString(resp.Supersedes, proposalID) {
		resp.Supersedes = append(resp.Supersedes, proposalID)
	}
}

func chatResponseMessageKind(resp ChatResponse) string {
	status := strings.ToLower(strings.TrimSpace(resp.GoalStatus))
	workflowStage := strings.ToLower(cleanContextText(resp.WorkflowData["canary_stage"]))
	hasVerification := cleanContextText(resp.WorkflowData["verification"]) != "" || len(mapValue(resp.WorkflowData["verification_result"])) > 0 || strings.HasPrefix(workflowStage, "executed_")
	hasExecutionReceipt := cleanContextText(resp.WorkflowData["execution_id"]) != "" || cleanContextText(resp.WorkflowData["execution_status"]) != "" || chatIntValue(resp.WorkflowData["receipt_count"]) > 0
	switch {
	case strings.TrimSpace(resp.Error) != "" || status == "error" || status == "failed":
		return "error"
	case resp.NeedsConfirmation || resp.ProposalPresentation != nil || strings.Contains(status, "confirmation"):
		return "proposal"
	case hasVerification:
		return "verification"
	case hasExecutionReceipt || len(resp.ExecutedKernelReply) > 0 || len(resp.ProjectResultCards) > 0:
		return "execution_receipt"
	case strings.Contains(strings.ToLower(strings.TrimSpace(resp.CurrentStep)), "verif"):
		return "verification"
	default:
		return "assistant"
	}
}

func chatResponseHistoryData(resp ChatResponse, data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	data["lifecycle"] = firstNonEmpty(resp.Lifecycle, "durable")
	data["persistence"] = firstNonEmpty(resp.Persistence, "project_history")
	data["message_kind"] = firstNonEmpty(resp.MessageKind, chatResponseMessageKind(resp))
	data["turn_id"] = firstNonEmpty(resp.TurnID, resp.RunID, resp.GoalID)
	if strings.TrimSpace(resp.LogicalMessageID) != "" {
		data["logical_message_id"] = strings.TrimSpace(resp.LogicalMessageID)
	}
	if len(resp.Supersedes) > 0 {
		data["supersedes"] = append([]string(nil), resp.Supersedes...)
	}
	if messageData := chatResponseMessageData(resp); len(messageData) > 0 {
		data["message_data"] = messageData
	}
	return data
}

func chatResponseMessageData(resp ChatResponse) map[string]any {
	if resp.MessageKind != "proposal" && !resp.NeedsConfirmation && len(resp.InteractionRequests) == 0 {
		return nil
	}
	data := map[string]any{
		"schema_version":     "vit.message_data.v1",
		"needs_confirmation": resp.NeedsConfirmation,
		"plan_id":            strings.TrimSpace(resp.PlanID),
		"preview":            strings.TrimSpace(resp.Preview),
		"workflow":           strings.TrimSpace(resp.Workflow),
	}
	if resp.ProposalPresentation != nil {
		data["proposal_presentation"] = resp.ProposalPresentation
	}
	if len(resp.WorkflowData) > 0 {
		data["workflow_data"] = resp.WorkflowData
	}
	if len(resp.InteractionRequests) > 0 {
		data["interaction_requests"] = resp.InteractionRequests
	}
	return data
}

func syncChatResponseMessageIdentityFromHistory(resp *ChatResponse) {
	if resp == nil || strings.TrimSpace(resp.LogicalMessageID) != "" {
		return
	}
	rows := dictionaryRowsFromAny(resp.ProjectHistory["conversation_messages"])
	expected := strings.TrimSpace(resp.Reply)
	for index := len(rows) - 1; index >= 0; index-- {
		row := rows[index]
		role := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(row["role"]), cleanContextText(row["kind"]))))
		if role != "assistant" && role != "vit" {
			continue
		}
		content := strings.TrimSpace(firstNonEmpty(cleanContextText(row["content"]), cleanContextText(row["text"]), cleanContextText(row["text_preview"])))
		if expected != "" && content != expected {
			continue
		}
		resp.LogicalMessageID = firstNonEmpty(cleanContextText(row["logical_message_id"]), cleanContextText(row["node_id"]), cleanContextText(row["id"]))
		return
	}
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
	orchestrationRuntime := orchestrationruntime.New()
	if storePath := orchestration.DefaultFileStorePath(); storePath != "" {
		if store, err := orchestration.NewFileStore(storePath); err == nil {
			orchestrationRuntime = orchestrationruntime.NewWithStore(store)
		} else if logger != nil {
			logger.Warn("[orchestration] persistent store unavailable path=%s error=%v; using memory store", storePath, err)
		}
	}
	server := &Server{
		kernel:                           kernelClient,
		auditionKernel:                   kernelClient,
		shadow:                           shadowProject,
		llm:                              &llm.Client{},
		logger:                           logger,
		harness:                          harness.New(kernelClient, shadowProject, logger),
		startedAt:                        time.Now(),
		conversations:                    map[string][]llm.Message{},
		pending:                          map[string]PendingPlan{},
		interactions:                     map[string]PendingInteraction{},
		mixSessions:                      map[string]MixSession{},
		goalContinuations:                map[string]agentloop.Continuation{},
		durableContinuations:             map[string]DurableContinuation{},
		capabilityRoutes:                 map[string]CapabilityRouteRecord{},
		schedulerWake:                    make(chan struct{}, 1),
		schedulerDone:                    make(chan struct{}),
		schedulerOwner:                   "scheduler_" + randomID(),
		continuationLease:                continuationLeaseDuration,
		conversationGoals:                map[string]string{},
		conversationMemory:               map[string]agentloop.ExecutionMemory{},
		pendingMixTicks:                  map[string]agentloop.PendingMixTickCandidate{},
		pendingTreatments:                map[string]agentloop.MixTreatmentPending{},
		freeStateLoops:                   map[string]freeStateReasoningLoop{},
		authorityMode:                    authorityModeManual,
		audioClosures:                    audioclosure.NewMemoryStore(),
		controllerOwners:                 orchestrationcontroller.NewRegistry(),
		pendingManager:                   pendingmanager.NewMemoryManager(),
		orchestrationRuntime:             orchestrationRuntime,
		uiContext:                        map[string]any{},
		events:                           map[string][]AgentEvent{},
		eventSeq:                         map[string]int64{},
		eqOperations:                     map[string]eqOperationRecord{},
		processorCertificationJobs:       map[string]processorCertificationJob{},
		processorCertificationLoadTokens: map[string]processorCertificationLoadAuthorization{},
	}
	server.schedulerCtx, server.schedulerCancel = context.WithCancel(context.Background())
	server.auditionCandidateDriver = &serverAuditionCandidateProjectDriver{server: server}
	return server
}

func (s *Server) HandleKernelTelemetry(event map[string]any) {
	if s == nil {
		return
	}
	if s.harness != nil {
		s.harness.IngestKernelTelemetry(event)
	}
	s.ingestAuditionTelemetry(event)
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/app", s.handleApp)
	mux.HandleFunc("/app/", s.handleApp)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/agent/runtime/status", s.handleRuntimeStatus)
	mux.HandleFunc("/agent/authority", s.handleAuthorityMode)
	mux.HandleFunc("/agent/turn/stop", s.handleTurnStop)
	mux.HandleFunc("/agent/events", s.handleAgentEvents)
	mux.HandleFunc("/agent/audition/status", s.handleAuditionStatus)
	mux.HandleFunc("/agent/audition/select", s.handleAuditionSelect)
	mux.HandleFunc("/agent/audition/stop", s.handleAuditionStop)
	mux.HandleFunc("/agent/audition/judgment", s.handleAuditionJudgment)
	mux.HandleFunc("/agent/audition/inspect_candidate", s.handleAuditionInspectCandidate)
	mux.HandleFunc("/agent/audition/apply_candidate", s.handleAuditionApplyCandidate)
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
	mux.HandleFunc("/agent/processor-certification/candidates", s.handleProcessorCertificationCandidates)
	mux.HandleFunc("/agent/processor-certification/start", s.handleProcessorCertificationStart)
	mux.HandleFunc("/agent/processor-certification/status", s.handleProcessorCertificationStatus)
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
	s.logger.Info("[webui] serving root=%s index_mod=%s assets=%s build_mark=project-aware-mondrian-workbench-v1-20260713", root, indexMod, assetSummary)
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
	goal := s.harness.RuntimeStatus("")
	_, checkoutBlocked := s.harness.CheckoutBlocked()
	response := map[string]any{
		"status":            "ok",
		"service":           "VitAgent",
		"pid":               os.Getpid(),
		"checked_at":        time.Now().Format(time.RFC3339Nano),
		"kernel":            kernelStatus,
		"shadow":            runtimeShadowStatus(s.harness.StateSummary(shadowCtx)),
		"goal":              goal,
		"continuations":     s.continuationRuntimeProjection(),
		"capability_routes": s.capabilityRouteProjection(),
		"authority_mode":    s.authorityModeSnapshot(),
		"checkout_blocked":  checkoutBlocked,
	}
	// Keep the legacy goal status for compatibility, but expose the canonical
	// Task semantic projection as a first-class runtime status surface.
	if goal.Task != nil {
		response["task"] = goal.Task
		response["task_trajectory"] = s.taskRuntimeTrajectoryProjection(goal)
		if goal.Task.Contract != nil {
			response["task_contract"] = goal.Task.Contract
		}
		if goal.Task.SemanticState != nil {
			response["task_state"] = goal.Task.SemanticState.State
			response["task_state_revision"] = goal.Task.SemanticState.Revision
			response["task_semantic_state"] = goal.Task.SemanticState
		}
	}
	writeJSON(w, http.StatusOK, response)
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
	stateHistory := compactAgentStateProjectHistory(projectHistory)
	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("detail")), "full") {
		stateHistory = projectHistory
	}
	agentPlan := s.activeGoalPlan(goal, stateHistory)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"shadow": shadowState,
		"goal":   goal,
		"active_goal": map[string]any{
			"goal":            goal,
			"agent_plan":      agentPlan,
			"project_history": stateHistory,
		},
		"project_history": stateHistory,
		"direct_commands": s.harness.DirectCommandNames(),
		"tool_count":      len(s.harness.Tools()),
	})
}

// compactAgentStateProjectHistory keeps /agent/state suitable for frequent GUI
// polling. Full conversation nodes can carry large proposal/readback payloads and
// are available through the history tools (or /agent/state?detail=full), but
// including them several times in this response exceeds Godot's 16 MiB HTTP
// chunk limit on real projects.
func compactAgentStateProjectHistory(history map[string]any) map[string]any {
	if len(history) == 0 {
		return map[string]any{}
	}
	keys := []string{
		"available",
		"project_path",
		"current_project_path",
		"project_uuid",
		"root_project_path",
		"initialized",
		"draft",
		"unsaved",
		"project_label",
		"active_branch",
		"active_node_id",
		"active_worktree",
		"detached",
		"head",
		"commit_count",
		"worktree_count",
		"baseline_commit",
		"baseline_created",
		"goal_active_branch",
		"goal_detached",
		"branches",
		"refs",
		"worktrees",
		"recent_checkpoints",
		"warnings",
	}
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := history[key]; ok {
			out[key] = value
		}
	}
	return out
}

// compactChatResponseProjectHistory keeps the small conversation tail needed
// by clients to reconcile an interaction response with the durable project
// history. The full graph remains available through the history APIs, while
// omitting it here avoids Godot's 16 MiB HTTP chunk limit on large projects.
func compactChatResponseProjectHistory(history map[string]any) map[string]any {
	out := compactAgentStateProjectHistory(history)
	rows := dictionaryRowsFromAny(history["conversation_messages"])
	if len(rows) == 0 {
		return out
	}
	start := len(rows) - 2
	if start < 0 {
		start = 0
	}
	messages := make([]map[string]any, 0, len(rows)-start)
	for _, row := range rows[start:] {
		message := map[string]any{}
		for _, key := range []string{
			"role", "content", "node_id", "message_kind", "logical_message_id",
		} {
			if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
				message[key] = value
			}
		}
		if len(message) > 0 {
			messages = append(messages, message)
		}
	}
	if len(messages) > 0 {
		out["conversation_messages"] = messages
	}
	return out
}

func compactHistoryListProjectHistory(historyMap map[string]any) map[string]any {
	out := compactAgentStateProjectHistory(historyMap)
	for _, key := range []string{"history_dir", "state_dir"} {
		if value, ok := historyMap[key]; ok {
			out[key] = value
		}
	}
	rows := dictionaryRowsFromAny(historyMap["conversation_messages"])
	const messageLimit = 200
	if len(rows) > messageLimit {
		rows = rows[len(rows)-messageLimit:]
	}
	messages := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		message := map[string]any{}
		for _, key := range []string{
			"role", "content", "node_id", "commit_id", "branch", "message_kind",
			"logical_message_id", "created_at",
		} {
			if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
				message[key] = value
			}
		}
		if len(message) > 0 {
			messages = append(messages, message)
		}
	}
	if len(messages) > 0 {
		out["conversation_messages"] = messages
	}
	if graph := compactHistoryConversationGraph(historyMap["conversation_graph"], 500); len(graph) > 0 {
		out["conversation_graph"] = graph
	}
	return out
}

func compactHistoryConversationGraph(value any, limit int) map[string]any {
	graph := map[string]any{}
	nodes := make([]map[string]any, 0)
	switch typed := value.(type) {
	case history.ConversationGraph:
		graph["project_path"] = typed.ProjectPath
		graph["project_uuid"] = typed.ProjectUUID
		graph["active_node_id"] = typed.ActiveNodeID
		graph["active_branch"] = typed.ActiveBranch
		graph["active_worktree"] = typed.ActiveWorktree
		start := 0
		if limit > 0 && len(typed.Nodes) > limit {
			start = len(typed.Nodes) - limit
		}
		for _, node := range typed.Nodes[start:] {
			preview := strings.TrimSpace(node.TextPreview)
			if preview == "" {
				preview = compactHistoryTextPreview(node.Text, 240)
			}
			nodes = append(nodes, map[string]any{
				"id": node.ID, "kind": node.Kind, "commit_id": node.CommitID,
				"parent_node_id": node.ParentNodeID, "branch": node.Branch,
				"text_preview": preview, "message_kind": node.MessageKind,
				"logical_message_id": node.LogicalMessageID, "created_at": node.CreatedAt,
			})
		}
	case map[string]any:
		for _, key := range []string{"project_path", "project_uuid", "active_node_id", "active_branch", "active_worktree"} {
			if item, ok := typed[key]; ok {
				graph[key] = item
			}
		}
		rows := mapRowsFromAny(typed["nodes"])
		if limit > 0 && len(rows) > limit {
			rows = rows[len(rows)-limit:]
		}
		for _, row := range rows {
			node := map[string]any{}
			for _, key := range []string{
				"id", "node_id", "kind", "commit_id", "parent_node_id", "branch",
				"text_preview", "message_kind", "logical_message_id", "created_at",
			} {
				if item, ok := row[key]; ok && !stripSilenceCompactEmpty(item) {
					node[key] = item
				}
			}
			if _, ok := node["text_preview"]; !ok {
				node["text_preview"] = compactHistoryTextPreview(cleanContextText(row["text"]), 240)
			}
			nodes = append(nodes, node)
		}
	}
	if len(nodes) > 0 {
		graph["nodes"] = nodes
	}
	return graph
}

func compactHistoryTextPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func (s *Server) handleUIState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	s.activateCurrentProjectWorkspace(r.Context())
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
		"status":           "ok",
		"project":          uiProjectState(state),
		"transport":        uiTransportState(state),
		"tracks":           mapRowsFromAny(state["tracks"]),
		"selected_track":   uiSelectedTrack(state),
		"selected_plugin":  uiSelectedPlugin(state),
		"plugin_rack":      uiPluginRack(state),
		"macro_controls":   macroControls,
		"ui_context":       uiContext,
		"goal":             goal,
		"authority_mode":   s.authorityModeSnapshot(),
		"checkout_blocked": agentruntime.IsActiveStatus(goal.Status),
		"agent_plan":       s.activeGoalPlan(goal, projectHistory),
		"artifacts":        artifacts.Summaries(items),
		"project_history":  projectHistory,
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
	for _, key := range []string{"kind", "source", "artifact_schema"} {
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
	for _, key := range []string{"track_id", "plugin_id", "plugin_name"} {
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
	tool := strings.ToLower(strings.TrimSpace(req.Tool))
	hostLifecycleNotification := tool == "version.project_saved" || tool == "version.project_opened" || tool == "version.project_new"
	if hostLifecycleNotification {
		// The Kernel has already switched identity. Preserve the old in-memory
		// workspace until the lifecycle notification establishes the new one.
		s.persistCurrentProjectWorkspace()
	} else {
		s.activateCurrentProjectWorkspace(r.Context())
		s.persistCurrentProjectWorkspace()
	}
	defer s.syncCurrentProjectWorkspace(r.Context())
	if strings.TrimSpace(req.Source) == "" {
		req.Source = "http"
	}
	if err := s.validateInvokeAuthority(req.Context); err != nil {
		writeJSON(w, http.StatusConflict, harness.InvokeResponse{Status: "error", Tool: req.Tool, Error: "authority_mode_not_active: " + err.Error(), Result: map[string]any{"error_code": "authority_mode_not_active"}})
		return
	}
	if guard, blocked := s.checkoutGuard(req); blocked {
		writeJSON(w, http.StatusConflict, harness.InvokeResponse{Status: "error", Tool: req.Tool, CommandName: firstNonEmpty(fmt.Sprint(req.Command["cmd"]), req.Tool), Error: "checkout_blocked_while_agent_running", Result: guard})
		return
	}
	if reason := freeStateInvokeMutationReason(req); reason != "" {
		writeJSON(w, http.StatusBadRequest, harness.InvokeResponse{
			Status: "error", Tool: req.Tool, Error: reason,
			Result: map[string]any{"status": "rejected", "rejection_code": "free_state_mutation_forbidden", "mutation_performed": false},
		})
		return
	}
	if response, needsConfirmation := typedPluginApplyConfirmationResponse(req); needsConfirmation {
		writeJSON(w, http.StatusOK, response)
		return
	}
	if strings.TrimSpace(req.AuthorizationToken) != "" {
		var authErr error
		req, authErr = s.consumeProcessorCertificationLoadAuthorization(req)
		if authErr != nil {
			writeJSON(w, http.StatusForbidden, harness.InvokeResponse{Status: "error", Tool: req.Tool, Error: authErr.Error()})
			return
		}
	}
	if isVersionProjectNewInvoke(req) {
		actionID := "act_" + randomID()
		bgReq := req
		go s.invokeVersionProjectNewInBackground(bgReq, actionID)
		writeJSON(w, http.StatusOK, harness.InvokeResponse{
			Status:               "ok",
			AgentActionID:        actionID,
			Tool:                 "version.project_new",
			CommandName:          "version_project_new",
			RiskLevel:            tools.RiskDirect,
			RequiresConfirmation: false,
			Result: map[string]any{
				"status":   "ok",
				"accepted": true,
				"async":    true,
				"message":  "Project new notification accepted",
			},
		})
		return
	}
	if resp, ok := s.invokeMixSessionEntryWorkflow(r.Context(), req); ok {
		writeJSON(w, http.StatusOK, compactStripSilenceInvokeResponseForTransport(resp))
		return
	}
	if workflowCmd, ok := pluginGrabberInspectGateExpanderInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectGateExpanderWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyGateExpanderInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyGateExpanderWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectTransientShaperInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectTransientShaperWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyTransientShaperInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyTransientShaperWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectMultibandInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectMultibandWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyMultibandInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyMultibandWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectLimiterInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectLimiterWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectDeEsserInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectDeEsserWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyDeEsserInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyDeEsserWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectSpectralDynamicsInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectSpectralDynamicsWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyLimiterInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyLimiterWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberInspectCompressorInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberInspectCompressorWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberApplyCompressorInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyCompressorWorkflow(r.Context(), req, workflowCmd)
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
	if workflowCmd, ok := pluginGrabberApplyEQEditsInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberApplyEQEditsWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if workflowCmd, ok := pluginGrabberSetEQPointInvokeCommand(req); ok {
		resp, err := s.invokePluginGrabberSetEQPointWorkflow(r.Context(), req, workflowCmd)
		status := http.StatusOK
		if err != nil && resp.Status == "error" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, resp)
		return
	}
	if resp, blocked := s.guardEQOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardCompressorOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardLimiterOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardGateExpanderOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardDeEsserOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardTransientShaperOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	if resp, blocked := s.guardMultibandOwnedGenericParameterWrite(r.Context(), req); blocked {
		writeJSON(w, http.StatusBadRequest, resp)
		return
	}
	resp, err := s.harness.Invoke(r.Context(), req)
	status := http.StatusOK
	if err != nil && resp.Status == "error" {
		status = http.StatusBadRequest
	}
	if isVersionProjectSavePrepareInvoke(req) {
		resp = compactProjectSavePrepareInvokeResponseForTransport(resp)
	} else if isProjectSaveAsFolderInvoke(req) {
		resp = compactSaveAsFolderInvokeResponseForTransport(resp)
	} else if hostLifecycleNotification {
		resp = compactHostLifecycleInvokeResponseForTransport(resp)
	}
	// ProjectHistory is auxiliary context on invoke responses. The actual tool
	// result remains authoritative, while full history is available from the
	// history APIs. Returning the full conversation graph after every tool call
	// makes large action sets (for example B1) exceed Godot's 16 MiB chunk
	// parser limit and turns successful executions into transport failures.
	if isVersionListInvoke(req) {
		resp.Result = compactHistoryListProjectHistory(resp.Result)
		resp.ProjectHistory = nil
	} else {
		resp.ProjectHistory = compactAgentStateProjectHistory(resp.ProjectHistory)
	}
	writeJSON(w, status, compactStripSilenceInvokeResponseForTransport(resp))
}

// A project-save prepare may freeze a very large Agent workspace, but the host
// only needs the immutable prepare identity before asking the Kernel to write
// the .vit file. Returning ProjectHistory here can exceed Godot's 16 MiB HTTP
// chunk limit and strand a valid prepared package before the Kernel save.
func compactProjectSavePrepareInvokeResponseForTransport(resp harness.InvokeResponse) harness.InvokeResponse {
	result := map[string]any{}
	for _, key := range []string{
		"status", "prepared", "prepare_id", "history_prepare_id",
		"generation_id", "agent_history_generation", "project_path", "current_project_path",
		"project_uuid", "project_id", "save_kind", "working_session_id",
		"project_package_prepared",
	} {
		if value, ok := resp.Result[key]; ok {
			result[key] = value
		}
	}
	resp.Result = result
	resp.ProjectHistory = nil
	return resp
}

func compactHostLifecycleInvokeResponseForTransport(resp harness.InvokeResponse) harness.InvokeResponse {
	result := map[string]any{}
	for _, key := range []string{
		"status", "project_path", "current_project_path", "project_uuid", "project_id",
		"parent_project_uuid", "source_project_uuid", "history_prepare_id", "agent_history_generation",
		"project_package_committed", "project_package_path",
		"prepared_save_recovery", "project_workspace_warning", "refresh",
	} {
		if value, ok := resp.Result[key]; ok {
			result[key] = value
		}
	}
	resp.Result = result
	resp.ProjectHistory = compactAgentStateProjectHistory(resp.ProjectHistory)
	return resp
}

// Folder export has a two-step protocol: a small media preflight followed by
// the actual snapshot publication. Preserve every field the Godot dialog and
// verification path consume, but never echo the source project's full history
// graph as part of either response.
func compactSaveAsFolderInvokeResponseForTransport(resp harness.InvokeResponse) harness.InvokeResponse {
	result := map[string]any{}
	for _, key := range []string{
		"status", "message", "error", "project_lifecycle",
		"project_path", "current_project_path", "project_uuid", "project_id",
		"source_project_path", "source_project_uuid", "directory_path", "media_policy",
		"referenced_audio_count", "external_audio_count", "missing_audio_count", "estimated_copy_bytes",
		"audio_self_contained", "package_status", "package_manifest_path", "active_project_unchanged",
	} {
		if value, ok := resp.Result[key]; ok {
			result[key] = value
		}
	}
	resp.Result = result
	resp.ProjectHistory = nil
	return resp
}

func isProjectSaveAsFolderInvoke(req harness.InvokeRequest) bool {
	for _, candidate := range []string{
		req.Tool,
		tools.CommandName(req.Command),
		tools.CommandName(req.Args),
		firstStringFromMap(req.Command, "tool"),
		firstStringFromMap(req.Args, "tool"),
	} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		normalized = strings.ReplaceAll(normalized, ".", "_")
		if normalized == "project_save_as_folder" || normalized == "save_as_folder" {
			return true
		}
	}
	return false
}

func isVersionProjectSavePrepareInvoke(req harness.InvokeRequest) bool {
	for _, candidate := range []string{
		req.Tool,
		tools.CommandName(req.Command),
		tools.CommandName(req.Args),
		firstStringFromMap(req.Command, "tool"),
		firstStringFromMap(req.Args, "tool"),
	} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		normalized = strings.ReplaceAll(normalized, ".", "_")
		if normalized == "version_project_save_prepare" {
			return true
		}
	}
	return false
}

func isVersionListInvoke(req harness.InvokeRequest) bool {
	for _, candidate := range []string{
		req.Tool,
		tools.CommandName(req.Command),
		tools.CommandName(req.Args),
		firstStringFromMap(req.Command, "tool"),
		firstStringFromMap(req.Args, "tool"),
	} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		normalized = strings.ReplaceAll(normalized, ".", "_")
		if normalized == "version_list" {
			return true
		}
	}
	return false
}

func isVersionProjectNewInvoke(req harness.InvokeRequest) bool {
	for _, candidate := range []string{
		req.Tool,
		tools.CommandName(req.Command),
		tools.CommandName(req.Args),
		firstStringFromMap(req.Command, "tool"),
		firstStringFromMap(req.Args, "tool"),
	} {
		normalized := strings.ToLower(strings.TrimSpace(candidate))
		normalized = strings.ReplaceAll(normalized, ".", "_")
		if normalized == "version_project_new" {
			return true
		}
	}
	return false
}

func (s *Server) invokeVersionProjectNewInBackground(req harness.InvokeRequest, actionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	started := time.Now()
	resp, err := s.harness.Invoke(ctx, req)
	if s.logger != nil {
		s.logger.Info("[timing] async version_project_new total_ms=%d ack_action=%s status=%s err=%t",
			time.Since(started).Milliseconds(), actionID, resp.Status, err != nil)
		if err != nil {
			s.logger.Warn("async version_project_new failed ack_action=%s error=%v", actionID, err)
		}
	}
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
		"selected_clip_ranges",
		"selected_clip_range_count",
		"selected_clip_range",
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
	defer s.beginContinuationSensitiveInvocation()()
	req.Context = contextWithConversationID(req.Context, conversationID)
	req.Context = contextWithCachedUIContext(req.Context, s.uiContextSnapshot())
	req.Context = s.contextWithCurrentProjectWorkspace(r.Context(), req.Context)
	s.activateCurrentProjectWorkspace(r.Context())
	defer s.syncCurrentProjectWorkspace(r.Context())
	var authorityErr error
	req.Context, authorityErr = s.bindChatAuthorityMode(req.AuthorityMode, req.Context)
	if authorityErr != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error_code": "authority_mode_change_blocked", "error": authorityErr.Error()})
		return
	}
	projectPath := projectPathFromChatContext(req.Context)
	agentMode := agentModeFromContext(req.Context)
	// Bind a plain-language proposal confirmation to the persisted interaction
	// before beginChatGoal can perform semantic entry. The confirmation is a
	// continuation of the existing Task, never a new natural-language task.
	if pending, ok := s.pendingImprovementProposalForConversation(conversationID); ok {
		decision := actionworkflow.ClassifyConfirmation(req.Message, true)
		if decision.Kind == actionworkflow.DecisionAccept || decision.Kind == actionworkflow.DecisionReject || isImprovementProposalConfirmationText(req.Message) {
			req.Context = contextWithGoal(req.Context, pending.GoalID, pending.RunID)
		}
	}
	goal := s.beginChatGoal(conversationID, req.Message, req.Context)
	s.mu.Lock()
	if s.conversationGoals == nil {
		s.conversationGoals = map[string]string{}
	}
	s.conversationGoals[conversationID] = goal.GoalID
	s.mu.Unlock()
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
		s.bindTaskIdentityToChatResponse(&resp)
		compactStripSilenceChatResponseForTransport(&resp)
		// Never synthesise a project-result card for a turn that failed. A
		// failed capability turn (e.g. B4 blocked at resolve-only) still leaves
		// read-only kernel replies in ExecutedKernelReply, and building cards
		// from those reports a successful project mutation that never happened.
		if len(resp.ProjectResultCards) == 0 && !chatResponseTurnFailed(resp) {
			resp.ProjectResultCards = projectResultCardsFromExecuted(resp.ExecutedKernelReply)
		}
		goalID := firstNonEmpty(resp.GoalID, goal.GoalID)
		ensureChatResponseMessageProtocol(&resp)
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, artifactSummariesFromExecuted(resp.ExecutedKernelReply))
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, s.artifactsFromDialogueMediaReferences(req.Message, resp.Reply, conversationID, goalID, firstNonEmpty(resp.RunID, goal.RunID), attachmentScope))
		resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, requestArtifacts)
		s.attachInteractionRequests(&resp)
		if strings.TrimSpace(resp.Reply) != "" {
			historyData := chatResponseHistoryData(resp, map[string]any{
				"artifacts": artifactSummaryRows(resp.Artifacts),
			})
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
		syncChatResponseMessageIdentityFromHistory(&resp)
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
		compactStripSilenceChatResponseForTransport(&resp)
		switch {
		case resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingClarification):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingClarification, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingContinue):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
		case resp.GoalStatus == string(agentruntime.StatusStopped):
			s.harness.MarkGoalStopped(goalID, firstStringFromMap(resp.ProjectHistory, "head", "commit_id"))
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
		if resp.GoalStatus == string(agentruntime.StatusStopped) {
			eventType = "turn.stopped"
		} else if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			eventType = "turn.failed"
		}
		s.emitTurnEvent(conversationID, eventType, resp, goal.GoalID, goal.RunID)
		writeJSON(w, status, resp)
	}

	chatContext := contextWithUserMessage(req.Context, req.Message)
	s.clearPendingMixForClipFadeGainRequest(conversationID, req.Message)

	// A natural-language confirmation is a response to the durable interaction,
	// not a new semantic request. Resolve it before any capability or semantic
	// entry router can reinterpret the text and create a second Task/Goal/Run.
	if interaction, ok := s.takePendingImprovementProposalForChat(conversationID, req.Message); ok {
		resp := s.continueImprovementProposalInteraction(r.Context(), interaction, req.Message)
		resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, req.Message)
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}

	// The v1 capability runtime is an opt-in canary. Its Session owner is fixed
	// before any planning state is created, so legacy pending cannot consume the
	// new Proposal or Authorization.
	if resp, handled := s.handleCapabilityRuntimeCanary(r.Context(), conversationID, ChatRequest{
		ConversationID: conversationID,
		Message:        req.Message,
		Context:        chatContext,
		Attachments:    req.Attachments,
		ArtifactRefs:   req.ArtifactRefs,
	}, goal); handled {
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
	}

	if !strings.HasPrefix(req.Message, "/") {
		if plan, ok := s.pendingPlanForChat(conversationID, req.Context); ok {
			if pendingPlanPlainApproval(req.Message) {
				status, response := s.resolvePendingPlanDecision(r.Context(), plan.ID, "approve")
				resp := chatResponseFromPendingPlanDecision(conversationID, response, agentModeFromContext(plan.Context))
				s.remember(conversationID, req.Message, resp.Reply)
				writeChat(status, resp)
				return
			}
			if pendingPlanCanBeRevisedByMixRequest(plan, req.Message) {
				s.expirePendingPlan(plan.ID)
				if goalID, _ := goalIDsFromContext(plan.Context); goalID != "" {
					s.clearGoalContinuation(goalID)
				}
				if s != nil && s.logger != nil {
					s.logger.Info("[chat.pending] expired mix confirmation for revised request conversation=%s plan=%s message=%q", conversationID, plan.ID, req.Message)
				}
			} else {
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
					Commands:          compactAgentLoopDecisionsForResponse(plan.Decisions),
					GoalStatus:        string(agentruntime.StatusWaitingConfirmation),
					ProjectHistory:    projectHistory,
				}
				s.attachInteractionRequests(&resp)
				s.remember(conversationID, req.Message, resp.Reply)
				writeChat(http.StatusOK, resp)
				return
			}
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
				Commands:          compactAgentLoopDecisionsForResponse(plan.Decisions),
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

	if resp, handled := s.handlePendingPluginParameterTreatmentChat(r.Context(), conversationID, ChatRequest{
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

	if resp, handled := s.handlePendingMixTreatmentChat(r.Context(), conversationID, ChatRequest{
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

	if resp, handled := s.handlePendingMixTickChat(r.Context(), conversationID, ChatRequest{
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

	if messagePlainMixApproval(req.Message) && !s.hasPendingConfirmationForChat(conversationID, chatContext) &&
		!s.hasActiveFreeStateReasoningLoop(conversationID) {
		resp := ChatResponse{
			ConversationID: conversationID,
			AgentMode:      agentMode,
			Reply:          "我没有找到正在等待确认的混音动作，所以这句确认不会执行任何操作。请先告诉我要调整什么，或重新提出具体的混音修改。",
			GoalStatus:     string(agentruntime.StatusCompleted),
			StopReason:     "plain_approval_without_pending_confirmation",
		}
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
	if len(env.Commands) == 0 && !legacyChatBroadMixRequestNeedsObservation(req.Message) {
		env.Commands = synthesizePluginGrabberLoadCommands(req.Message, chatContext)
	}
	if resp, blocked := s.legacyChatBroadMixWriteBlockedResponse(r.Context(), conversationID, req.Message, env.Commands, chatContext); blocked {
		s.remember(conversationID, req.Message, resp.Reply)
		writeChat(http.StatusOK, resp)
		return
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

func (s *Server) legacyChatBroadMixWriteBlockedResponse(ctx context.Context, conversationID, userMessage string, commands []map[string]any, chatContext map[string]any) (ChatResponse, bool) {
	decisions := policy.Analyze(commands)
	return s.legacyChatBroadMixDecisionBlockedResponse(ctx, conversationID, userMessage, decisions, chatContext)
}

func (s *Server) legacyChatBroadMixDecisionBlockedResponse(ctx context.Context, conversationID, userMessage string, decisions []policy.Decision, chatContext map[string]any) (ChatResponse, bool) {
	if len(decisions) == 0 {
		return ChatResponse{}, false
	}
	if !legacyChatBroadMixRequestNeedsObservation(userMessage) {
		return ChatResponse{}, false
	}
	if legacyChatExplicitPluginOrRawRequest(userMessage) {
		return ChatResponse{}, false
	}
	blocked := legacyChatBroadMixBlockedDecisions(decisions)
	if len(blocked) == 0 {
		return ChatResponse{}, false
	}
	names := decisionNames(blocked)
	reply := "我不会直接给宽泛的混音目标加载效果器或写参数。先做一次 mix.request_observation，基于当前轨道/音频的实际观察给出建议；你确认一个具体小动作后，我再执行可撤回的单步调整。已拦截本次旧流程动作：" + strings.Join(names, ", ") + "。"
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          reply,
		Commands:       decisions,
		GoalStatus:     string(agentruntime.StatusCompleted),
		ProjectHistory: s.harness.ProjectHistorySummaryForProject(ctx, "", projectPathFromChatContext(chatContext)),
	}, true
}

func legacyChatBroadMixBlockedDecisions(decisions []policy.Decision) []policy.Decision {
	blocked := make([]policy.Decision, 0, len(decisions))
	for _, decision := range decisions {
		if legacyChatBroadMixDecisionBlocked(decision) {
			blocked = append(blocked, decision)
		}
	}
	return blocked
}

func legacyChatBroadMixDecisionBlocked(decision policy.Decision) bool {
	name := legacyChatDecisionName(decision)
	switch name {
	case "rack_add_node", "rack.add_node", "plugin.load_to_rack", "instantiate_plugin", "plugin.instantiate",
		"plugin_grabber_load_and_get_params", "plugin_grabber.load_and_get_params",
		"plugin_grabber_apply_eq_edits", "plugin_grabber.apply_eq_edits",
		"plugin_grabber_apply_compressor_controls", "plugin_grabber.apply_compressor_controls",
		"plugin_grabber_apply_limiter_controls", "plugin_grabber.apply_limiter_controls",
		"plugin_grabber_apply_gate_expander_controls", "plugin_grabber.apply_gate_expander_controls",
		"plugin_grabber_apply_transient_shaper_controls", "plugin_grabber.apply_transient_shaper_controls",
		"plugin_grabber_apply_multiband_controls", "plugin_grabber.apply_multiband_controls",
		"plugin_grabber_apply_de_esser_controls", "plugin_grabber.apply_de_esser_controls",
		"set_plugin_param", "plugin.set_parameter", "plugin_set_parameter",
		"set_volume", "track.volume",
		"control_add_macro", "control.add_macro", "rack.add_macro", "control_add_binding", "control.add_binding":
		return true
	default:
		return false
	}
}

func legacyChatDecisionName(decision policy.Decision) string {
	for _, value := range []string{
		decision.Name,
		policy.CommandName(decision.Command),
		cleanContextText(decision.Command["cmd"]),
		cleanContextText(decision.Command["command"]),
		cleanContextText(decision.Command["action"]),
		cleanContextText(decision.Command["tool"]),
	} {
		if text := strings.ToLower(strings.TrimSpace(value)); text != "" {
			return text
		}
	}
	return ""
}

func legacyChatBroadMixRequestNeedsObservation(userText string) bool {
	if legacyChatVolumeMixRequestNeedsObservation(userText) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return legacyChatTextHasAny(text,
		"混音", "缩混", "声音处理", "调一下", "处理一下",
		"主唱", "人声", "vocal", "lead vocal",
		"靠前", "往前", "提升响度", "响度", "更亮", "明亮", "浑浊", "刺耳",
		"低频", "低中频", "空间感", "加一点空间", "动态", "压缩",
		"mix", "mixing", "loudness", "louder", "forward", "mud", "muddy", "harsh", "bright", "space", "reverb", "dynamic",
	)
}

func legacyChatVolumeMixRequestNeedsObservation(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return legacyChatTextHasAny(text,
		"\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e",
		"\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca",
		"too loud", "too quiet", "volume", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase",
	)
}

func legacyChatExplicitPluginOrRawRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasExplicitVerb := legacyChatTextHasAny(text,
		"加载", "挂载", "打开", "学习", "抓手", "插入", "新增",
		"设置参数", "写参数", "改参数", "调参数",
		"load", "insert", "open", "learn", "grabber", "set parameter", "write parameter",
	)
	hasPluginObject := legacyChatTextHasAny(text,
		"插件", "效果器", "均衡器", "压缩器", "混响", "延迟",
		"plugin", "vst", "eq", "compressor", "reverb", "delay", "tdr", "nova", "zl",
	)
	hasRawParam := legacyChatTextHasAny(text, "param_id", "parameter id", "参数 id", "归一化", "normalized")
	return hasRawParam || (hasExplicitVerb && hasPluginObject)
}

func legacyChatTextHasAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func pendingPlanPlainApproval(message string) bool {
	return messagePlainMixApproval(message)
}

func chatResponseFromPendingPlanDecision(conversationID string, response map[string]any, fallbackMode string) ChatResponse {
	if len(response) == 0 {
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      fallbackMode,
			Reply:          "确认已处理。",
			GoalStatus:     string(agentruntime.StatusCompleted),
		}
	}
	mode := firstNonEmpty(cleanContextText(response["agent_mode"]), fallbackMode)
	reply := firstNonEmpty(cleanContextText(response["reply"]), cleanContextText(response["message"]))
	if strings.TrimSpace(reply) == "" {
		reply = "确认已处理。"
	}
	resp := ChatResponse{
		ConversationID:            conversationID,
		GoalID:                    cleanContextText(response["goal_id"]),
		RunID:                     cleanContextText(response["run_id"]),
		Reply:                     reply,
		AgentMode:                 mode,
		NeedsConfirmation:         boolValue(response["needs_confirmation"]),
		PlanID:                    firstNonEmpty(cleanContextText(response["next_plan_id"]), cleanContextText(response["plan_id"])),
		Preview:                   cleanContextText(response["preview"]),
		Workflow:                  cleanContextText(response["workflow"]),
		WorkflowData:              mapValue(response["workflow_data"]),
		ExecutedKernelReply:       compactAgentLoopExecutedForResponse(mapRowsFromAny(response["executed_kernel_reply"])),
		ProjectResultCards:        mapRowsFromAny(response["project_result_cards"]),
		GoalStatus:                cleanContextText(response["goal_status"]),
		GoalSummary:               cleanContextText(response["goal_summary"]),
		CurrentStep:               cleanContextText(response["current_step"]),
		CompletedSteps:            chatIntValue(response["completed_steps"]),
		StopReason:                cleanContextText(response["stop_reason"]),
		LimitType:                 cleanContextText(response["limit_type"]),
		ProjectHistory:            mapValue(response["project_history"]),
		TypedEvents:               mapRowsFromAny(response["typed_events"]),
		AcousticPackageStatus:     mapValue(response["acoustic_package_status"]),
		AcousticPackageStatusPath: cleanContextText(response["acoustic_package_status_path"]),
		Error:                     cleanContextText(response["error"]),
	}
	resp.Commands = compactAgentLoopDecisionsForResponse(resp.Commands)
	if resp.GoalStatus == "" {
		if boolValue(response["blocked"]) || strings.EqualFold(cleanContextText(response["status"]), "error") {
			resp.GoalStatus = string(agentruntime.StatusFailed)
		} else if resp.NeedsConfirmation {
			resp.GoalStatus = string(agentruntime.StatusWaitingConfirmation)
		} else {
			resp.GoalStatus = string(agentruntime.StatusCompleted)
		}
	}
	if resp.NeedsConfirmation {
		if nextPlanID := cleanContextText(response["next_plan_id"]); nextPlanID != "" {
			resp.PlanID = nextPlanID
		}
	} else if !strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) {
		resp.PlanID = ""
	}
	if agentPlan := chatAgentPlanFromAny(response["agent_plan"]); agentPlan != nil {
		resp.AgentPlan = agentPlanForMode(mode, agentPlan)
	}
	if artifactsValue, ok := response["artifacts"]; ok {
		appendArtifactSummariesFromValue(&resp.Artifacts, artifactsValue)
		resp.Artifacts = mergeArtifactSummaries(nil, resp.Artifacts)
	}
	return resp
}

func chatIntValue(value any) int {
	switch n := value.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case int32:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	case json.Number:
		i, _ := strconv.Atoi(n.String())
		return i
	default:
		i, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(value)))
		return i
	}
}

func chatAgentPlanFromAny(value any) *AgentPlan {
	switch plan := value.(type) {
	case *AgentPlan:
		return plan
	case AgentPlan:
		out := plan
		return &out
	case map[string]any:
		data, err := json.Marshal(plan)
		if err != nil {
			return nil
		}
		var out AgentPlan
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return &out
	default:
		return nil
	}
}

func (s *Server) legacyPendingPlanBroadMixBlockedConfirmResponse(ctx context.Context, planID string, plan PendingPlan, goalID, runID, agentMode, projectPath string) (map[string]any, bool) {
	if pendingPlanIsMixTreatmentPreparation(plan) || pendingPlanHasQualifiedSemanticPluginSelection(plan) {
		return nil, false
	}
	userMessage := legacyPendingPlanUserMessage(plan)
	if !legacyChatBroadMixRequestNeedsObservation(userMessage) || legacyChatExplicitPluginOrRawRequest(userMessage) {
		return nil, false
	}
	blocked := legacyChatBroadMixBlockedDecisions(plan.Decisions)
	if len(blocked) == 0 {
		return nil, false
	}
	names := decisionNames(blocked)
	message := "已拦截：这个确认计划来自宽泛混音请求，不能直接加载效果器、学习插件或写参数。请先让 Ask Vit 执行 mix.request_observation，听感/工程观察明确后，再确认一个具体的小步调整。被拦截动作：" + strings.Join(names, ", ") + "。"
	if s != nil && s.harness != nil {
		s.harness.CompleteGoal(goalID, nil)
	}
	projectHistory := map[string]any{}
	if s != nil && s.harness != nil {
		projectHistory = s.harness.RecordConversationNodeForProject(ctx, projectPath, "vit", message, goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		}
	}
	response := map[string]any{
		"status":          "ok",
		"message":         message,
		"plan_id":         planID,
		"goal_id":         goalID,
		"run_id":          runID,
		"agent_mode":      agentMode,
		"goal_status":     string(agentruntime.StatusCompleted),
		"blocked":         true,
		"blocked_actions": names,
	}
	if len(projectHistory) > 0 {
		response["project_history"] = projectHistory
	}
	return response, true
}

// A target-3 selection is already a bounded user decision over exact local
// catalog facts. Let that one exact rack_add_node reach its normal confirmation
// without weakening the guard for arbitrary broad-mix loading plans.
func pendingPlanHasQualifiedSemanticPluginSelection(plan PendingPlan) bool {
	if plan.Workflow != pluginGrabberLoadCommand || !boolValue(plan.Context["semantic_plugin_recommendation_selection"]) {
		return false
	}
	candidate := firstMapFromAny(plan.Context["semantic_plugin_recommendation_candidate"])
	wantTrack := firstStringFromMap(plan.Context, "selected_track_id", "selected_plugin_track_id")
	wantPath := firstStringFromMap(candidate, "plugin_path")
	wantIdentifier := firstStringFromMap(candidate, "identifier")
	if wantTrack == "" || wantPath == "" || len(plan.Decisions) != 1 {
		return false
	}
	command := workflowCommandArgs(plan.Decisions[0].Command)
	if firstStringFromMap(command, "cmd", "command") != "rack_add_node" ||
		firstStringFromMap(command, "track_id") != wantTrack ||
		!strings.EqualFold(firstStringFromMap(command, "plugin_path"), wantPath) {
		return false
	}
	if wantIdentifier != "" && !strings.EqualFold(firstStringFromMap(command, "plugin_identifier"), wantIdentifier) {
		return false
	}
	return true
}

// pendingPlanExecutionContext converts a server-verified target-3 choice into
// the narrow, in-process harness authorization needed for exactly one plug-in
// load. Keeping this token out of the persisted/client payload prevents a
// caller from asserting that an arbitrary broad-mix load was selected.
func pendingPlanExecutionContext(plan PendingPlan) (map[string]any, error) {
	if !pendingPlanHasQualifiedSemanticPluginSelection(plan) {
		return plan.Context, nil
	}
	receipt, found, err := semanticValidatePCAAdmissionReceipt(plan, "", "")
	if err != nil {
		return nil, err
	}
	if !found {
		return plan.Context, nil
	}
	command := workflowCommandArgs(plan.Decisions[0].Command)
	return harness.AuthorizeProcessorSelectionLoad(
		plan.Context,
		firstStringFromMap(command, "track_id"),
		firstStringFromMap(command, "plugin_path"),
		receipt.Identifier,
		processorattestation.EligibilityRequirement{ProcessorFamily: receipt.ProcessorFamily},
		receipt.SubjectKey, receipt.BinaryFingerprint, receipt.AttestationID,
	), nil
}

func pendingPlanIsMixTreatmentPreparation(plan PendingPlan) bool {
	if boolValue(plan.Context["mix_treatment_preparation"]) || boolValue(plan.WorkflowData["mix_treatment_preparation"]) {
		return true
	}
	for _, row := range []map[string]any{
		mapValue(plan.Context["mix_treatment_preparation_plan"]),
		mapValue(plan.WorkflowData["mix_treatment_preparation_plan"]),
	} {
		if cleanContextText(row["schema_version"]) == "mix_treatment_preparation.v0" {
			return true
		}
	}
	return false
}

func legacyPendingPlanUserMessage(plan PendingPlan) string {
	if plan.GoalContinuation != nil {
		for _, value := range []string{
			cleanContextText(plan.GoalContinuation.UserText),
			cleanContextText(plan.GoalContinuation.Summary),
			cleanContextText(plan.GoalContinuation.Context["user_message"]),
		} {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	for _, value := range []string{
		cleanContextText(plan.Context["user_message"]),
		cleanContextText(plan.WorkflowData["user_message"]),
		cleanContextText(plan.WorkflowData["intent"]),
		cleanContextText(plan.WorkflowData["user_intent"]),
		cleanContextText(plan.WorkflowData["goal"]),
	} {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
	if resp, blocked := s.legacyChatBroadMixDecisionBlockedResponse(ctx, conversationID, userMessage, decisions, chatContext); blocked {
		return resp, true
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

func (s *Server) hasPendingConfirmationForChat(conversationID string, chatContext map[string]any) bool {
	if s == nil {
		return false
	}
	if _, ok := s.pendingMixTreatmentForConversation(conversationID); ok {
		return true
	}
	if _, ok := s.pendingMixTickForConversation(conversationID); ok {
		return true
	}
	if _, ok := s.pendingPlanForChat(conversationID, chatContext); ok {
		return true
	}
	return false
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
	return PendingPlan{}, false
}

func (s *Server) attachInteractionRequests(resp *ChatResponse) {
	if resp == nil || len(resp.InteractionRequests) > 0 {
		attachTypedInteractionRequests(resp)
		return
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
	attachTypedInteractionRequests(resp)
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
		resp.Commands = compactAgentLoopDecisionsForResponse(plan.Decisions)
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

func (s *Server) expirePendingPlan(planID string) {
	if s == nil || strings.TrimSpace(planID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletePendingAliasesForPlanIDLocked(planID)
}

func pendingPlanCanBeRevisedByMixRequest(plan PendingPlan, message string) bool {
	if !pendingPlanIsMixConfirmation(plan) {
		return false
	}
	return messageRevisesPendingMixTreatment(message) || messageLooksLikePanRevisionRequest(message) || messageLooksLikeNewMixRequest(message)
}

func pendingPlanIsMixConfirmation(plan PendingPlan) bool {
	if pendingPlanHasMixWriteAction(plan) {
		return true
	}
	return false
}

func pendingPlanHasMixWriteAction(plan PendingPlan) bool {
	if plan.GoalContinuation != nil && pendingPlanToolIsMixConfirmation(plan.GoalContinuation.PendingToolCall) {
		return true
	}
	for _, decision := range plan.Decisions {
		if decisionIsMixConfirmation(decision) {
			return true
		}
	}
	return false
}

func pendingPlanToolIsMixConfirmation(call *planner.ToolCall) bool {
	if call == nil {
		return false
	}
	return textHasAny(firstNonEmpty(
		strings.TrimSpace(call.Tool),
		cleanContextText(call.Command["tool"]),
		cleanContextText(call.Args["tool"]),
		cleanContextText(call.Command["cmd"]),
		cleanContextText(call.Args["cmd"]),
	), "mix.apply_tick", "mix_apply_tick", "mix.propose_tick", "mix_propose_tick", "track.pan", "track_pan", "set_pan", "track.volume", "set_volume")
}

func decisionIsMixConfirmation(decision policy.Decision) bool {
	name := strings.ToLower(strings.TrimSpace(decision.Name))
	if textHasAny(name, "mix_apply_tick", "mix.apply_tick", "mix_propose_tick", "mix.propose_tick") {
		return true
	}
	cmd := decision.Command
	return textHasAny(firstNonEmpty(
		cleanContextText(cmd["tool"]),
		cleanContextText(cmd["cmd"]),
		cleanContextText(cmd["command"]),
		cleanContextText(cmd["action"]),
	), "mix.apply_tick", "mix_apply_tick", "mix.propose_tick", "mix_propose_tick", "track.pan", "track_pan", "set_pan", "track.volume", "set_volume")
}

func pendingPlanToolIsClipFadeGainWrite(call *planner.ToolCall) bool {
	if call == nil {
		return false
	}
	return commandNameIsClipFadeGainWrite(firstNonEmpty(
		strings.TrimSpace(call.Tool),
		cleanContextText(call.Command["tool"]),
		cleanContextText(call.Args["tool"]),
		cleanContextText(call.Command["cmd"]),
		cleanContextText(call.Args["cmd"]),
	))
}

func decisionIsClipFadeGainWrite(decision policy.Decision) bool {
	if commandNameIsClipFadeGainWrite(decision.Name) {
		return true
	}
	cmd := decision.Command
	return commandNameIsClipFadeGainWrite(firstNonEmpty(
		cleanContextText(cmd["tool"]),
		cleanContextText(cmd["cmd"]),
		cleanContextText(cmd["command"]),
		cleanContextText(cmd["action"]),
	))
}

func commandNameIsClipFadeGainWrite(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "clip.fade.set", "clip_fade_set", "clip.gain.set", "clip_gain_set":
		return true
	default:
		return false
	}
}

func messageLooksLikeNewMixRequest(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	if textHasAny(text, "为什么", "为啥", "原因", "解释", "why", "explain") {
		return false
	}
	if textHasAny(text, "确认执行", "可以执行", "执行这个", "应用这个", "apply it", "execute it", "confirm and apply") {
		return false
	}
	if textHasAny(text, "我想", "希望", "能不能", "可不可以", "帮我", "让", "把", "换", "改", "调到", "设置为", "instead", "change", "switch", "make it", "set to") &&
		textHasAny(text, "混音", "声像", "声相", "轨道", "音量", "响度", "增益", "左", "右", "中间", "居中", "靠前", "压缩", "eq", "reverb", "pan", "panning", "volume", "gain", "loudness", "track", "left", "right", "center") {
		return true
	}
	return textHasAny(text, "左 70", "右 70", "70% left", "70% right", "全左", "全右", "最左", "最右", "hard left", "hard right")
}

func (s *Server) confirmationInteractionRequest(resp ChatResponse) AgentInteractionRequest {
	planID := strings.TrimSpace(resp.PlanID)
	if planID == "" {
		return AgentInteractionRequest{}
	}
	commands := compactAgentLoopDecisionsForResponse(resp.Commands)
	payload := map[string]any{
		"plan_id":  planID,
		"preview":  resp.Preview,
		"commands": commands,
	}
	if requestContext := firstMapFromAny(resp.WorkflowData["request_context"]); len(requestContext) > 0 {
		payload["request_context"] = cloneContext(requestContext)
	}
	presentationMap := mapValue(resp.WorkflowData["proposal_presentation"])
	isCapabilityProposal := strings.EqualFold(resp.Workflow, "capability_runtime_v1") &&
		(resp.ProposalPresentation != nil || cleanContextText(presentationMap["proposal_id"]) != "")
	if isCapabilityProposal {
		for _, key := range []string{"session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash", "approval_mode", "proposal_presentation"} {
			if value, ok := resp.WorkflowData[key]; ok {
				payload[key] = value
			}
		}
		if _, ok := payload["proposal_presentation"]; !ok && resp.ProposalPresentation != nil {
			payload["proposal_presentation"] = resp.ProposalPresentation
		}
		if s.orchestrationRuntime != nil && s.orchestrationRuntime.Store != nil {
			if session, ok := s.orchestrationRuntime.Store.Load(cleanContextText(resp.WorkflowData["session_id"])); ok && session.ActiveProposal != nil {
				payload["action_set_hash"] = session.ActiveProposal.ActionSetHash
			}
		}
	}
	if decision, ok := firstApprovalDecision(commands); ok {
		approval := typedApprovalFromDecision(planID, decision, resp.ConversationID, resp.GoalID, resp.RunID, resp.Reply)
		payload["typed_state"] = agentprotocol.ToMap(approval)
		payload["typed_event"] = agentprotocol.ToMap(agentprotocol.NewEvent(approval, approval.Source))
	}
	kind := "confirmation"
	title := "需要确认"
	body := firstNonEmpty(strings.TrimSpace(resp.Reply), "Vit 需要你确认后再继续。")
	stage := "waiting_for_user"
	if isCapabilityProposal {
		kind = "proposal_approval"
		stage = "proposal_review"
		presentation := mapValue(payload["proposal_presentation"])
		presentationTitle := cleanContextText(presentation["title"])
		presentationConclusion := cleanContextText(presentation["conclusion"])
		if resp.ProposalPresentation != nil {
			presentationTitle = firstNonEmpty(resp.ProposalPresentation.Title, presentationTitle)
			presentationConclusion = firstNonEmpty(resp.ProposalPresentation.Conclusion, presentationConclusion)
		}
		title = firstNonEmpty(presentationTitle, "方案等待确认")
		body = firstNonEmpty(presentationConclusion, strings.TrimSpace(resp.Preview), "方案已完成分析并等待你的授权。")
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           kind,
		Type:           kind,
		Source:         "vit_agent",
		Title:          title,
		Body:           body,
		Status:         "waiting_for_user",
		Workflow:       resp.Workflow,
		Stage:          stage,
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
	s.bindPendingInteractionContinuationLocked(req)
	s.mu.Unlock()
	if s.harness != nil && strings.TrimSpace(req.GoalID) != "" {
		s.harness.SetGoalStatus(req.GoalID, agentruntime.StatusWaitingConfirmation, nil)
	}
	s.persistCurrentProjectWorkspace()
}

func (s *Server) pendingImprovementProposalForConversation(conversationID string) (PendingInteraction, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return PendingInteraction{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var newest PendingInteraction
	found := false
	for _, interaction := range s.interactions {
		if interaction.ConversationID != conversationID ||
			(!strings.EqualFold(interaction.Kind, "improvement_proposal_confirmation") &&
				!strings.EqualFold(interaction.Type, "improvement_proposal_confirmation") &&
				!strings.EqualFold(interaction.Workflow, improvementProposalWorkflow)) {
			continue
		}
		if !found || interaction.CreatedAt.After(newest.CreatedAt) {
			newest, found = interaction, true
		}
	}
	return newest, found
}

func (s *Server) takePendingImprovementProposalForChat(conversationID, message string) (PendingInteraction, bool) {
	decision := actionworkflow.ClassifyConfirmation(message, true)
	if decision.Kind != actionworkflow.DecisionAccept && decision.Kind != actionworkflow.DecisionReject && !isImprovementProposalConfirmationText(message) {
		return PendingInteraction{}, false
	}
	interaction, ok := s.pendingImprovementProposalForConversation(conversationID)
	if !ok {
		return PendingInteraction{}, false
	}
	return s.takePendingInteraction(interaction.ID)
}

func isImprovementProposalConfirmationText(message string) bool {
	text := strings.TrimSpace(strings.ToLower(message))
	return strings.Contains(text, "确认") && (strings.Contains(text, "提案") || strings.Contains(text, "建议") || strings.Contains(text, "这个"))
}

func (s *Server) bindTaskIdentityToChatResponse(resp *ChatResponse) {
	if s == nil || s.harness == nil || resp == nil || strings.TrimSpace(resp.GoalID) == "" {
		return
	}
	goal := s.harness.RuntimeStatus(resp.GoalID)
	if goal.Task == nil {
		return
	}
	resp.TaskID = firstNonEmpty(resp.TaskID, goal.Task.TaskID)
	resp.OriginalIntent = firstNonEmpty(resp.OriginalIntent, goal.Task.OriginalIntent)
	resp.SliceID = firstNonEmpty(resp.SliceID, goal.Task.Run.CurrentSliceID)
	resp.TurnID = firstNonEmpty(resp.TurnID, goal.Task.Run.CurrentTurnID)
}

func (s *Server) bindPendingInteractionContinuationLocked(req AgentInteractionRequest) {
	var latestID string
	var latest DurableContinuation
	for id, item := range s.durableContinuations {
		if item.ConversationID != req.ConversationID || item.GoalID != req.GoalID || item.RunID != req.RunID {
			continue
		}
		if latestID == "" || item.UpdatedAt.After(latest.UpdatedAt) {
			latestID, latest = id, item
		}
	}
	if latestID == "" {
		return
	}
	latest.Status = ContinuationWaitingInteraction
	latest.LeaseOwner = ""
	latest.LeaseExpiresAt = time.Time{}
	latest.PendingInteraction = map[string]any{
		"status": string(agentruntime.StatusWaitingConfirmation), "interaction_id": req.ID,
		"kind": firstNonEmpty(req.Kind, req.Type), "goal_id": req.GoalID, "run_id": req.RunID,
		"task_id": latest.TaskID, "continuation_id": latest.ContinuationID,
		"requests": []any{structMap(req)},
	}
	latest.UpdatedAt = time.Now().UTC()
	if s.harness != nil {
		goal := s.harness.RuntimeStatus(req.GoalID)
		if goal.Task != nil {
			latest.TaskSemanticState = cloneTaskSemanticState(goal.Task.SemanticState)
		}
	}
	s.durableContinuations[latestID] = cloneDurableContinuation(latest)
}

func (s *Server) completePendingInteractionContinuation(interaction PendingInteraction) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for id, item := range s.durableContinuations {
		if item.ConversationID != interaction.ConversationID || item.GoalID != interaction.GoalID || item.RunID != interaction.RunID {
			continue
		}
		if firstStringFromMap(item.PendingInteraction, "interaction_id") != interaction.ID {
			continue
		}
		item.Status = ContinuationCompleted
		item.PendingInteraction = nil
		item.UpdatedAt = time.Now().UTC()
		s.durableContinuations[id] = cloneDurableContinuation(item)
	}
	s.mu.Unlock()
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

func (s *Server) restorePendingInteraction(interaction PendingInteraction) {
	if s == nil || strings.TrimSpace(interaction.ID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interactions[interaction.ID] = interaction
}

func capabilityInteractionConfirmationKind(decision string) string {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approve", "approve_once", "approved":
		return actionworkflow.DecisionAccept
	case "cancel", "reject", "deny", "decline", "stop", "no":
		return actionworkflow.DecisionReject
	default:
		return actionworkflow.ClassifyConfirmation(decision, true).Kind
	}
}

func (s *Server) recoverMixBoardInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if len(payload) == 0 {
		return PendingInteraction{}, false
	}
	requestContext := mapValue(payload["request_context"])
	if !legacyMixSessionWorkflowEnabled(mergeContext(requestContext, payload)) &&
		!boolValue(payload["conversational_mix"]) &&
		!boolValue(payload["ask_vit_mix_action"]) {
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

func (s *Server) recoverCapabilityRuntimeInteractionFromPayload(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil || len(payload) == 0 {
		return PendingInteraction{}, false
	}
	if !strings.EqualFold(cleanContextText(payload["workflow"]), "capability_runtime_v1") ||
		!strings.EqualFold(firstNonEmpty(cleanContextText(payload["approval_mode"]), "conversational"), "conversational") {
		return PendingInteraction{}, false
	}
	sessionID := cleanContextText(payload["session_id"])
	capabilityID := cleanContextText(payload["capability_id"])
	proposalID := cleanContextText(payload["proposal_id"])
	proposalRevision := int64(chatIntValue(payload["proposal_revision"]))
	actionSetHash := cleanContextText(payload["action_set_hash"])
	projectCutHash := cleanContextText(payload["project_cut_hash"])
	if sessionID == "" || capabilityID == "" || proposalID == "" || proposalRevision < 1 || actionSetHash == "" || projectCutHash == "" {
		return PendingInteraction{}, false
	}
	session, ok := s.orchestrationRuntime.Store.Load(sessionID)
	if !ok || session.ActiveProposal == nil || session.FrozenPlan == nil ||
		(session.Status != orchestration.StatusWaiting && session.Status != orchestration.StatusReady) ||
		session.Invocation.CapabilityID != capabilityID || session.ActiveProposal.ID != proposalID ||
		session.ActiveProposal.Revision != proposalRevision || session.ActiveProposal.ActionSetHash != actionSetHash ||
		session.ActiveProposal.ProjectCutHash != projectCutHash {
		return PendingInteraction{}, false
	}
	data := copyStringAnyMap(payload)
	requestContext := mapValue(payload["request_context"])
	conversationID := firstNonEmpty(cleanContextText(payload["conversation_id"]), session.Invocation.ConversationID, interactionID)
	return PendingInteraction{
		ID: interactionID, CreatedAt: time.Now(), Kind: "proposal_approval", Type: "proposal_approval",
		Source: "vit_agent", Workflow: "capability_runtime_v1", Stage: "proposal_review", PlanID: proposalID,
		ConversationID: conversationID, GoalID: cleanContextText(payload["goal_id"]), RunID: cleanContextText(payload["run_id"]),
		RequestContext: requestContext, Payload: data, Data: data,
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
	defer s.beginContinuationSensitiveInvocation()()
	s.activateCurrentProjectWorkspace(r.Context())
	defer s.syncCurrentProjectWorkspace(r.Context())
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
		interaction, ok = s.recoverCapabilityRuntimeInteractionFromPayload(interactionID, req.Payload)
		if ok && s != nil && s.logger != nil {
			s.logger.Info("[capability.interaction] recovered expired interaction=%s session=%s proposal=%s revision=%v", interactionID, cleanContextText(req.Payload["session_id"]), cleanContextText(req.Payload["proposal_id"]), req.Payload["proposal_revision"])
		}
	}
	if !ok {
		interaction, ok = recoverPluginRecommendationInteractionFromPayload(interactionID, req.Payload)
		if ok && s != nil && s.logger != nil {
			s.logger.Info("[plugin.recommendation] recovered workspace-transition interaction=%s decision=%s", interactionID, decision)
		}
	}
	if !ok {
		interaction, ok = recoverSemanticTreatmentInteractionFromPayload(interactionID, req.Payload)
		if ok && s != nil && s.logger != nil {
			s.logger.Info("[semantic.treatment] recovered workspace-transition interaction=%s decision=%s", interactionID, decision)
		}
	}
	if !ok {
		interaction, ok = recoverB4PluginSelectionInteraction(interactionID, req.Payload)
		if ok && s != nil && s.logger != nil {
			s.logger.Info("[b4.plugin-selection] recovered interaction=%s decision=%s", interactionID, decision)
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
	if goal := s.harness.RuntimeStatus(interaction.GoalID); goal.Status == agentruntime.StatusStopped || goal.StopRequested {
		writeJSON(w, http.StatusConflict, ChatResponse{ConversationID: interaction.ConversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "当前 Turn 已停止，不能继续执行待处理动作。", GoalStatus: string(agentruntime.StatusStopped), StopReason: "stop_turn_prevents_new_action"})
		return
	}
	kind := firstNonEmpty(interaction.Kind, interaction.Type)
	isMixTickInteraction := strings.EqualFold(interaction.Kind, "mix_tick_confirmation") || strings.EqualFold(interaction.Type, "mix_tick_confirmation") || strings.EqualFold(interaction.Workflow, "mix_tick")
	isMixTreatmentInteraction := strings.EqualFold(interaction.Kind, "mix_treatment_confirmation") || strings.EqualFold(interaction.Type, "mix_treatment_confirmation") || strings.EqualFold(interaction.Workflow, "mix_treatment")
	isImprovementProposalInteraction := strings.EqualFold(interaction.Kind, "improvement_proposal_confirmation") || strings.EqualFold(interaction.Type, "improvement_proposal_confirmation") || strings.EqualFold(interaction.Workflow, improvementProposalWorkflow)
	isCapabilityRuntimeInteraction := strings.EqualFold(interaction.Workflow, "capability_runtime_v1") && firstNonEmpty(cleanContextText(interaction.Data["session_id"]), cleanContextText(interaction.Payload["session_id"])) != ""
	if isCapabilityRuntimeInteraction {
		capabilityID := firstNonEmpty(cleanContextText(interaction.Data["capability_id"]), cleanContextText(interaction.Payload["capability_id"]))
		sessionID := firstNonEmpty(cleanContextText(interaction.Data["session_id"]), cleanContextText(interaction.Payload["session_id"]))
		message := ""
		switch capabilityInteractionConfirmationKind(decision) {
		case actionworkflow.DecisionAccept:
			message = "可以执行"
		case actionworkflow.DecisionReject:
			message = "取消"
		default:
			s.restorePendingInteraction(interaction)
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status": "error", "message": "capability proposal requires an explicit approve or cancel decision",
				"interaction_id": interactionID,
			})
			return
		}
		requestContext := mergeContext(interaction.RequestContext, map[string]any{
			"capability_runtime_v1": true, "capability_id": capabilityID,
			"capability_session_id": sessionID, "expected_proposal_id": interaction.PlanID,
			"expected_proposal_revision": firstNonEmpty(cleanContextText(interaction.Data["proposal_revision"]), cleanContextText(interaction.Payload["proposal_revision"])),
			"expected_action_set_hash":   firstNonEmpty(cleanContextText(interaction.Data["action_set_hash"]), cleanContextText(interaction.Payload["action_set_hash"])),
			"expected_project_cut_hash":  firstNonEmpty(cleanContextText(interaction.Data["project_cut_hash"]), cleanContextText(interaction.Payload["project_cut_hash"])),
			"conversation_id":            interaction.ConversationID,
		})
		resp, handled := s.handleCapabilityRuntimeCanary(r.Context(), interaction.ConversationID, ChatRequest{
			ConversationID: interaction.ConversationID, Message: message, Context: requestContext,
		}, agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID})
		if !handled {
			resp = capabilityCanaryBlockedResponse(interaction.ConversationID, agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID}, "v1 capability confirmation 已过期或 owner 不匹配。")
		}
		if firstStringFromMap(interaction.RequestContext, "capability_id") != dynamicControlCapabilityID {
			resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		}
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, message)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Workflow, semanticCompressorExecutionWorkflow) {
		resp := s.continueSemanticCompressorExecutionInteraction(r.Context(), interaction, decision)
		resp = s.completeC2PostAction(r.Context(), interaction, resp)
		if firstStringFromMap(interaction.RequestContext, "capability_id") != dynamicControlCapabilityID {
			resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		}
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Workflow, semanticDynamicWorkflow) {
		resp := s.continueSemanticDynamicExecutionInteraction(r.Context(), interaction, decision)
		resp = s.completeC2PostAction(r.Context(), interaction, resp)
		if firstStringFromMap(interaction.RequestContext, "capability_id") != dynamicControlCapabilityID {
			resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		}
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if isImprovementProposalInteraction {
		resp := s.continueImprovementProposalInteraction(r.Context(), interaction, decision)
		resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	// C2 carries a PlanID for presentation/history, but its parameter batch is
	// executed by the C2 all-or-rollback handler rather than the legacy pending
	// plan registry. Route it before the generic confirmation fallback.
	if strings.EqualFold(interaction.Source, c2DynamicParameterBatchWorkflow) || strings.EqualFold(interaction.Type, c2DynamicParameterBatchWorkflow) {
		resp := s.continueC2DynamicParameterBatchInteraction(r.Context(), interaction, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.TrimSpace(interaction.PlanID) != "" && (kind == "confirmation" || strings.Contains(interaction.Type, "final_review") || strings.Contains(interaction.Type, "teach_review")) {
		status, response := s.resolvePendingPlanDecision(r.Context(), interaction.PlanID, decision)
		response["interaction_id"] = interaction.ID
		s.attachInteractionsToResponseMap(&response, interaction)
		writeJSON(w, status, response)
		return
	}
	if strings.EqualFold(interaction.Source, pluginPrepWorkerWorkflow) || strings.EqualFold(interaction.Type, pluginPrepParameterTreatmentType) || strings.EqualFold(interaction.Workflow, pluginPrepWorkerWorkflow) {
		resp := s.continuePluginPrepWorkerCandidateInteraction(r.Context(), interaction, req.Payload, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_recommendation") || strings.EqualFold(interaction.Workflow, pluginRecommendationWorkflow) || strings.EqualFold(interaction.Type, "plugin_recommendation_selection") {
		resp := s.continuePluginRecommendationInteraction(r.Context(), interaction, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "semantic_treatment") || strings.EqualFold(interaction.Workflow, semanticTreatmentWorkflow) || strings.EqualFold(interaction.Type, "semantic_treatment_selection") {
		resp := s.continueSemanticTreatmentInteraction(r.Context(), interaction, decision)
		resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "b4_plugin_selection") || strings.EqualFold(interaction.Type, "b4_plugin_selection") {
		resp := s.continueB4PluginSelectionInteraction(r.Context(), interaction, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "c2_dynamic_plugin_selection") || strings.EqualFold(interaction.Type, "c2_dynamic_plugin_selection") {
		resp := s.continueC2DynamicPluginSelectionInteraction(r.Context(), interaction, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "c2_loaded_plugin_selection") || strings.EqualFold(interaction.Type, "c2_loaded_plugin_selection") {
		resp := s.continueC2LoadedPluginSelectionInteraction(r.Context(), interaction, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if isMixTickInteraction {
		if strings.EqualFold(decision, "cancel") || strings.EqualFold(decision, "cancel_mix_tick") {
			s.expirePendingMixTick(interaction.ConversationID)
			s.transitionActivePendingCandidate(interaction.ConversationID, "mix_tick", agentprotocol.PendingStatusRejected, "user cancelled pending mix tick")
			resp := ChatResponse{
				ConversationID: interaction.ConversationID,
				GoalID:         interaction.GoalID,
				RunID:          interaction.RunID,
				Reply:          "已取消这次混音单步建议，工程没有被修改。",
				Workflow:       interaction.Workflow,
				WorkflowData:   interaction.Payload,
				GoalStatus:     string(agentruntime.StatusCancelled),
			}
			s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, decision)
			writeJSON(w, http.StatusOK, resp)
			return
		}
		approvalText := "可以执行"
		if strings.TrimSpace(decision) != "" && !strings.EqualFold(decision, "approve") && !strings.EqualFold(decision, "confirm") && !strings.EqualFold(decision, "execute") {
			approvalText = decision
		}
		chatReq := ChatRequest{
			ConversationID: interaction.ConversationID,
			Message:        approvalText,
			Context:        contextWithGoal(mergeContext(interaction.RequestContext, map[string]any{"conversation_id": interaction.ConversationID}), interaction.GoalID, interaction.RunID),
		}
		resp, handled := s.handlePendingMixTickChat(r.Context(), interaction.ConversationID, chatReq, agentModeDefault)
		if !handled {
			resp = ChatResponse{
				ConversationID: interaction.ConversationID,
				GoalID:         interaction.GoalID,
				RunID:          interaction.RunID,
				Reply:          "这个混音单步确认已处理或已过期。",
				Workflow:       interaction.Workflow,
				WorkflowData:   interaction.Payload,
				GoalStatus:     string(agentruntime.StatusCompleted),
			}
		}
		// A confirmed native mix tick is still part of the active free-state
		// treatment loop. Feed its governed execution result through the same
		// post-action bridge as processor/capability workflows so the loop can
		// request fresh CCB evidence before any terminal conclusion.
		resp = s.maybeContinueFreeStateAfterInteraction(r.Context(), interaction, resp, decision)
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, approvalText)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(decision, "cancel") && !isMixTreatmentInteraction {
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
	if isMixTreatmentInteraction {
		approvalText := "可以执行"
		if strings.EqualFold(decision, "approve") || strings.EqualFold(decision, "confirm") || strings.EqualFold(decision, "execute") {
			approvalText = "可以执行"
		} else if strings.TrimSpace(decision) != "" {
			approvalText = decision
		}
		chatReq := ChatRequest{
			ConversationID: interaction.ConversationID,
			Message:        approvalText,
			Context:        contextWithGoal(mergeContext(interaction.RequestContext, map[string]any{"conversation_id": interaction.ConversationID}), interaction.GoalID, interaction.RunID),
		}
		resp, handled := s.handlePendingMixTreatmentChat(r.Context(), interaction.ConversationID, chatReq, agentModeDefault)
		if !handled {
			resp = ChatResponse{
				ConversationID: interaction.ConversationID,
				GoalID:         interaction.GoalID,
				RunID:          interaction.RunID,
				Reply:          "这个混音确认已处理或已过期。",
				Workflow:       interaction.Workflow,
				WorkflowData:   interaction.Payload,
				GoalStatus:     string(agentruntime.StatusCompleted),
			}
		}
		s.attachInteractionRequests(&resp)
		s.finalizeInteractionChatResponse(r.Context(), &resp, interaction, approvalText)
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if strings.EqualFold(interaction.Source, "plugin_grabber") && strings.EqualFold(interaction.Type, "plugin_prep_continuation") {
		resp := s.continuePluginPrepContinuationInteraction(r.Context(), interaction, decision)
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

func (s *Server) finalizeInteractionChatResponse(ctx context.Context, resp *ChatResponse, interaction PendingInteraction, userText string) {
	if s == nil || resp == nil {
		return
	}
	ctxGoalID, ctxRunID := goalIDsFromContext(interaction.RequestContext)
	if resp.ConversationID == "" {
		resp.ConversationID = interaction.ConversationID
	}
	if resp.GoalID == "" {
		resp.GoalID = firstNonEmpty(interaction.GoalID, ctxGoalID)
	}
	if resp.RunID == "" {
		resp.RunID = firstNonEmpty(interaction.RunID, ctxRunID)
	}
	if resp.AgentMode == "" {
		resp.AgentMode = agentModeFromContext(interaction.RequestContext)
	}
	if resp.Workflow == "" {
		resp.Workflow = interaction.Workflow
	}
	if len(resp.WorkflowData) == 0 && len(interaction.Payload) > 0 {
		resp.WorkflowData = interaction.Payload
	}
	compactStripSilenceChatResponseForTransport(resp)
	if len(resp.ProjectResultCards) == 0 {
		resp.ProjectResultCards = projectResultCardsFromExecuted(resp.ExecutedKernelReply)
	}
	resp.Artifacts = mergeArtifactSummaries(resp.Artifacts, artifactSummariesFromExecuted(resp.ExecutedKernelReply))
	ensureChatResponseMessageProtocol(resp)
	projectPath := projectPathFromChatContext(interaction.RequestContext)
	if s.harness != nil {
		if currentPath, _ := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(currentPath) != "" {
			projectPath = currentPath
		}
		goalID := firstNonEmpty(resp.GoalID, interaction.GoalID, ctxGoalID)
		runID := firstNonEmpty(resp.RunID, interaction.RunID, ctxRunID)
		if strings.TrimSpace(userText) != "" {
			s.harness.RecordConversationNodeForProject(ctx, projectPath, "ask", userText, goalID, runID)
		}
		if strings.TrimSpace(resp.Reply) != "" {
			historyData := chatResponseHistoryData(*resp, map[string]any{"artifacts": artifactSummaryRows(resp.Artifacts)})
			if len(resp.ProjectResultCards) > 0 {
				historyData["project_result_cards"] = resp.ProjectResultCards
			}
			if projectHistory := s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, "vit", resp.Reply, goalID, runID, historyData); len(projectHistory) > 0 {
				resp.ProjectHistory = projectHistory
			}
		}
		if len(resp.ProjectHistory) == 0 {
			resp.ProjectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		}
	}
	syncChatResponseMessageIdentityFromHistory(resp)
	if strings.TrimSpace(userText) != "" || strings.TrimSpace(resp.Reply) != "" {
		s.remember(resp.ConversationID, userText, resp.Reply)
	}
	if strings.TrimSpace(resp.GoalStatus) == "" {
		resp.GoalStatus = inferredGoalStatus(*resp)
	}
	if resp.AgentPlan == nil && shouldExposeAgentPlan(resp.AgentMode, *resp) {
		resp.AgentPlan = simpleAgentPlan(resp.GoalID, resp.RunID, agentruntime.GoalStatus(resp.GoalStatus), resp.GoalSummary, resp.CurrentStep, resp.Error, resp.ProjectHistory)
	}
	if resp.AgentPlan != nil {
		syncAgentPlanProjectHistory(resp)
	}
	goalID := firstNonEmpty(resp.GoalID, interaction.GoalID, ctxGoalID)
	if goalID != "" {
		switch {
		case resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingClarification):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingClarification, nil)
		case resp.GoalStatus == string(agentruntime.StatusWaitingContinue):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
		case resp.GoalStatus == string(agentruntime.StatusStopped):
			s.harness.MarkGoalStopped(goalID, firstStringFromMap(resp.ProjectHistory, "head", "commit_id"))
		case resp.GoalStatus == string(agentruntime.StatusCancelled):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		case resp.GoalStatus == string(agentruntime.StatusRunning):
			s.harness.SetGoalStatus(goalID, agentruntime.StatusRunning, nil)
		case strings.TrimSpace(resp.Error) != "":
			s.harness.CompleteGoal(goalID, fmt.Errorf("%s", resp.Error))
		default:
			s.harness.CompleteGoal(goalID, nil)
		}
	}
	eventType := "turn.completed"
	if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
		eventType = "turn.failed"
	}
	if resp.ConversationID != "" {
		s.emitTurnEvent(resp.ConversationID, eventType, *resp, firstNonEmpty(interaction.GoalID, ctxGoalID), firstNonEmpty(interaction.RunID, ctxRunID))
	}
	compactStripSilenceChatResponseForTransport(resp)
}

func (s *Server) attachInteractionsToResponseMap(response *map[string]any, interaction PendingInteraction) {
	if response == nil || *response == nil {
		return
	}
	data := mapValue((*response)["workflow_data"])
	goalStatus := strings.TrimSpace(fmt.Sprint((*response)["goal_status"]))
	needsConfirmation := boolValue((*response)["needs_confirmation"]) || strings.EqualFold(goalStatus, string(agentruntime.StatusWaitingConfirmation))
	resp := ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["goal_id"])), interaction.GoalID),
		RunID:               firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["run_id"])), interaction.RunID),
		Reply:               firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["message"])), strings.TrimSpace(fmt.Sprint((*response)["reply"]))),
		Workflow:            firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["workflow"])), interaction.Workflow),
		WorkflowData:        data,
		NeedsConfirmation:   needsConfirmation,
		PlanID:              firstNonEmpty(strings.TrimSpace(fmt.Sprint((*response)["plan_id"])), strings.TrimSpace(fmt.Sprint((*response)["next_plan_id"]))),
		Preview:             strings.TrimSpace(fmt.Sprint((*response)["preview"])),
		GoalStatus:          goalStatus,
		InteractionRequests: interactionRequestsFromAny((*response)["interaction_requests"]),
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

func interactionRequestsFromAny(value any) []AgentInteractionRequest {
	switch typed := value.(type) {
	case []AgentInteractionRequest:
		return append([]AgentInteractionRequest(nil), typed...)
	case []any:
		out := make([]AgentInteractionRequest, 0, len(typed))
		for _, item := range typed {
			if req, ok := agentInteractionRequestFromAny(item); ok {
				out = append(out, req)
			}
		}
		return out
	default:
		if req, ok := agentInteractionRequestFromAny(value); ok {
			return []AgentInteractionRequest{req}
		}
		return nil
	}
}

func agentInteractionRequestFromAny(value any) (AgentInteractionRequest, bool) {
	switch typed := value.(type) {
	case AgentInteractionRequest:
		return typed, true
	case map[string]any:
		raw, err := json.Marshal(typed)
		if err != nil {
			return AgentInteractionRequest{}, false
		}
		var req AgentInteractionRequest
		if err := json.Unmarshal(raw, &req); err != nil || req.ID == "" {
			return AgentInteractionRequest{}, false
		}
		return req, true
	default:
		return AgentInteractionRequest{}, false
	}
}

func (s *Server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
		return
	}
	defer s.beginContinuationSensitiveInvocation()()
	s.activateCurrentProjectWorkspace(r.Context())
	defer s.syncCurrentProjectWorkspace(r.Context())
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
	if goal := s.harness.RuntimeStatus(goalID); goal.Status == agentruntime.StatusStopped || goal.StopRequested {
		writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error_code": "stop_turn_prevents_new_action", "message": "当前 Turn 已停止，不能继续执行待确认计划。", "goal_id": goal.GoalID, "goal_status": goal.Status})
		return
	}
	projectPath := projectPathFromChatContext(plan.Context)
	agentMode := agentModeFromContext(plan.Context)
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	if plan.Workflow == agentLoopConfirmationWorkflow {
		if isApprovalDecision(decision) {
			if response, blocked := s.legacyPendingPlanBroadMixBlockedConfirmResponse(r.Context(), planID, plan, goalID, runID, agentMode, projectPath); blocked {
				writeJSON(w, http.StatusOK, response)
				return
			}
		}
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
		response["typed_events"] = typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "denied", "user cancelled confirmation")
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	if response, blocked := s.legacyPendingPlanBroadMixBlockedConfirmResponse(r.Context(), planID, plan, goalID, runID, agentMode, projectPath); blocked {
		writeJSON(w, http.StatusOK, response)
		return
	}
	executionContext, err := pendingPlanExecutionContext(plan)
	beforeState := map[string]any{}
	var replies []map[string]any
	if err == nil {
		beforeState = s.harness.UserStateSummary(r.Context())
		replies, err = s.executeDecisions(r.Context(), plan.Decisions, true, executionContext)
	}
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
			"replies":         compactAgentLoopExecutedForResponse(replies),
			"project_history": projectHistory,
			"typed_events":    typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "denied", err.Error()),
		}
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", err.Error(), projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(r.Context()), plan.Decisions, replies)
	var pluginPrep pluginPrepContinuation
	var semanticProcessorHandoff *ChatResponse
	if plan.Workflow == pluginGrabberLoadCommand {
		message, replies = s.finishPluginGrabberLoadWorkflow(r.Context(), plan, replies, message)
		pluginPrep = s.pluginPrepContinuationFromReplies(plan, replies, message)
		if handoff, ok := s.semanticProcessorPostLoadHandoff(r.Context(), plan, replies); ok {
			s.attachInteractionRequests(&handoff)
			semanticProcessorHandoff = &handoff
			message = handoff.Reply
		}
	}
	if plan.Workflow == "goal_ui_smoke" {
		message = "done"
	}
	if strings.TrimSpace(message) == "" {
		message = "done"
	}
	if semanticProcessorHandoff != nil && semanticProcessorHandoff.NeedsConfirmation {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
	} else if semanticProcessorHandoff != nil && strings.EqualFold(semanticProcessorHandoff.GoalStatus, string(agentruntime.StatusWaitingContinue)) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingContinue)) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
	} else {
		s.harness.CompleteGoal(goalID, nil)
	}
	visibleReplies := compactAgentLoopExecutedForResponse(replies)
	projectResultCards := projectResultCardsFromExecuted(visibleReplies)
	historyData := map[string]any{}
	if len(projectResultCards) > 0 {
		historyData["project_result_cards"] = projectResultCards
	}
	if semanticProcessorHandoff != nil {
		if messageData := chatResponseMessageData(*semanticProcessorHandoff); len(messageData) > 0 {
			historyData["message_data"] = messageData
		}
		historyData["message_kind"] = chatResponseMessageKind(*semanticProcessorHandoff)
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
		"replies":               visibleReplies,
		"executed_kernel_reply": visibleReplies,
		"project_result_cards":  projectResultCards,
		"project_history":       projectHistory,
		"goal_status":           string(agentruntime.StatusCompleted),
		"typed_events":          typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "consumed", message),
	}
	if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCompleted, "", "", "", projectHistory)); plan != nil {
		response["agent_plan"] = plan
	}
	response = applyPluginPrepContinuationResponse(response, pluginPrep)
	if semanticProcessorHandoff != nil {
		response = applySemanticPostLoadHandoffResponse(response, *semanticProcessorHandoff)
		if semanticProcessorHandoff.NeedsConfirmation {
			if agentPlan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingConfirmation, semanticProcessorHandoff.GoalSummary, "", "", projectHistory)); agentPlan != nil {
				response["agent_plan"] = agentPlan
			}
		}
	}
	if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) {
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingConfirmation, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingContinue)) {
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingContinue, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
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
	if goal := s.harness.RuntimeStatus(goalID); goal.Status == agentruntime.StatusStopped || goal.StopRequested {
		return http.StatusConflict, map[string]any{"status": "error", "error_code": "stop_turn_prevents_new_action", "message": "当前 Turn 已停止，不能继续执行待确认计划。", "goal_id": goal.GoalID, "goal_status": goal.Status}
	}
	projectPath := projectPathFromChatContext(plan.Context)
	agentMode := agentModeFromContext(plan.Context)
	cleanDecision := strings.ToLower(strings.TrimSpace(decision))
	if plan.Workflow == agentLoopConfirmationWorkflow {
		if isApprovalDecision(cleanDecision) {
			if response, blocked := s.legacyPendingPlanBroadMixBlockedConfirmResponse(ctx, planID, plan, goalID, runID, agentMode, projectPath); blocked {
				return http.StatusOK, response
			}
		}
		return s.resolveAgentLoopConfirm(ctx, planID, plan, cleanDecision)
	}
	if cleanDecision != "approve" && cleanDecision != "allow" && cleanDecision != "confirm" && cleanDecision != "yes" && cleanDecision != "submit" && cleanDecision != "save" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
		projectHistory := s.harness.RecordConversationNodeForProject(ctx, projectPath, "vit", "cancelled", goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		}
		response := map[string]any{"status": "ok", "message": "cancelled", "plan_id": planID, "goal_id": goalID, "run_id": runID, "agent_mode": agentMode, "goal_status": string(agentruntime.StatusCancelled), "project_history": projectHistory}
		response["typed_events"] = typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "denied", "user cancelled confirmation")
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCancelled, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		return http.StatusOK, response
	}
	if response, blocked := s.legacyPendingPlanBroadMixBlockedConfirmResponse(ctx, planID, plan, goalID, runID, agentMode, projectPath); blocked {
		return http.StatusOK, response
	}
	executionContext, err := pendingPlanExecutionContext(plan)
	beforeState := map[string]any{}
	var replies []map[string]any
	if err == nil {
		beforeState = s.harness.UserStateSummary(ctx)
		replies, err = s.executeDecisions(ctx, plan.Decisions, true, executionContext)
	}
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
			"replies":         compactAgentLoopExecutedForResponse(replies),
			"project_history": projectHistory,
			"typed_events":    typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "denied", err.Error()),
		}
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusFailed, "", "", err.Error(), projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
		return http.StatusOK, response
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(ctx), plan.Decisions, replies)
	var pluginPrep pluginPrepContinuation
	var semanticProcessorHandoff *ChatResponse
	if plan.Workflow == pluginGrabberLoadCommand {
		message, replies = s.finishPluginGrabberLoadWorkflow(ctx, plan, replies, message)
		pluginPrep = s.pluginPrepContinuationFromReplies(plan, replies, message)
		if handoff, ok := s.semanticProcessorPostLoadHandoff(ctx, plan, replies); ok {
			s.attachInteractionRequests(&handoff)
			semanticProcessorHandoff = &handoff
			message = handoff.Reply
		}
	}
	if strings.TrimSpace(message) == "" {
		message = "done"
	}
	if semanticProcessorHandoff != nil && semanticProcessorHandoff.NeedsConfirmation {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingConfirmation, nil)
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingContinue)) {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
	} else {
		s.harness.CompleteGoal(goalID, nil)
	}
	visibleReplies := compactAgentLoopExecutedForResponse(replies)
	projectResultCards := projectResultCardsFromExecuted(visibleReplies)
	historyData := map[string]any{}
	if len(projectResultCards) > 0 {
		historyData["project_result_cards"] = projectResultCards
	}
	if semanticProcessorHandoff != nil {
		if messageData := chatResponseMessageData(*semanticProcessorHandoff); len(messageData) > 0 {
			historyData["message_data"] = messageData
		}
		historyData["message_kind"] = chatResponseMessageKind(*semanticProcessorHandoff)
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
		"replies":               visibleReplies,
		"executed_kernel_reply": visibleReplies,
		"project_result_cards":  projectResultCards,
		"project_history":       projectHistory,
		"goal_status":           string(agentruntime.StatusCompleted),
		"typed_events":          typedApprovalDecisionEvents(planID, plan, cleanContextText(plan.WorkflowData["conversation_id"]), goalID, runID, "consumed", message),
	}
	if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusCompleted, "", "", "", projectHistory)); plan != nil {
		response["agent_plan"] = plan
	}
	response = applyPluginPrepContinuationResponse(response, pluginPrep)
	if semanticProcessorHandoff != nil {
		response = applySemanticPostLoadHandoffResponse(response, *semanticProcessorHandoff)
		if semanticProcessorHandoff.NeedsConfirmation {
			if agentPlan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingConfirmation, semanticProcessorHandoff.GoalSummary, "", "", projectHistory)); agentPlan != nil {
				response["agent_plan"] = agentPlan
			}
		}
	}
	if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) {
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingConfirmation, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
	} else if strings.EqualFold(pluginPrep.GoalStatus, string(agentruntime.StatusWaitingContinue)) {
		if plan := agentPlanForMode(agentMode, simpleAgentPlan(goalID, runID, agentruntime.StatusWaitingContinue, "", "", "", projectHistory)); plan != nil {
			response["agent_plan"] = plan
		}
	}
	return http.StatusOK, response
}

func (s *Server) handleDevChatCommand(ctx context.Context, conversationID string, req ChatRequest) (ChatResponse, bool) {
	msg := strings.TrimSpace(req.Message)
	if msg == "/capability-runtime/status" || msg == "/orchestration/status" {
		report := s.capabilityAuthorityReport()
		return ChatResponse{
			ConversationID: conversationID, Reply: capabilityAuthorityReportReply(report),
			Workflow: "capability_runtime_v1", WorkflowData: map[string]any{"authority_report": report},
		}, true
	}
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
	if msg == "/smoke range_context" {
		return smokeRangeContextResponse(conversationID, req.Context), true
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

func smokeRangeContextResponse(conversationID string, chatContext map[string]any) ChatResponse {
	checks := []string{}
	failures := []string{}
	if ranges := contextClipRangeRows(chatContext["selected_clip_ranges"]); len(ranges) > 0 {
		checks = append(checks, "top_level")
	} else {
		failures = append(failures, "missing top-level selected_clip_ranges")
	}
	current := mapValue(chatContext["current_selection"])
	if ranges := contextClipRangeRows(current["selected_clip_ranges"]); len(ranges) > 0 {
		checks = append(checks, "current_selection")
	} else {
		failures = append(failures, "missing current_selection.selected_clip_ranges")
	}
	uiContext := mapValue(chatContext["ui_context"])
	if ranges := contextClipRangeRows(uiContext["selected_clip_ranges"]); len(ranges) > 0 {
		checks = append(checks, "ui_context")
	} else {
		failures = append(failures, "missing ui_context.selected_clip_ranges")
	}
	prompt := agentContextForPrompt(chatContext)
	if strings.Contains(prompt, "selected_clip_ranges") && strings.Contains(prompt, "duration_seconds") {
		checks = append(checks, "prompt_context")
	} else {
		failures = append(failures, "missing prompt selected_clip_ranges")
	}
	if len(failures) > 0 {
		detail := strings.Join(failures, "; ")
		return ChatResponse{
			ConversationID: conversationID,
			Reply:          "range_context smoke failed: " + detail,
			StopReason:     "range_context_smoke_failed",
			Error:          detail,
		}
	}
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          "range_context smoke ok: " + strings.Join(checks, ", "),
		StopReason:     "range_context_smoke_ok",
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

// compactContextStrings trims and drops empty/"<nil>" entries, preserving
// order. Unlike firstNonEmpty this keeps every usable candidate instead of
// collapsing to one, for callers that need to try each ref rather than let
// the first non-empty one shadow the rest.
func compactContextStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "<nil>" {
			out = append(out, value)
		}
	}
	return out
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
For a single explicit audio file or selected library item that should be placed into a known track, use clip.import_media_to_track with file_path, track_id, start_time, media_type:"audio", mode:"non_destructive". If the user says "this audio", "selected library item", or "this file", use selected_library_file_path from current_selection. If the user gives an absolute path, copy it exactly into file_path. If the user asks to search the library, use asset_query and the selected/current track; the agent will search only the library Places roots. For stems, multitrack folders, folders of audio files, or explicit create-tracks-from-folder requests, use project.import_preflight first, then project.import_folder_as_stems for the write; never simulate a folder/stems import with repeated track.add_audio plus clip.import_media_to_track calls.
When the user confirms a TOM/A3 track organization proposal, use project.apply_track_organization with the proposal groups/assignments. This creates ordinary folder containers by default and moves hybrid tracks into them; do not enable routing_bus_enabled unless the user explicitly asks for folder bus/submix routing.
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
When the user says "this range", "these ranges", "selected range", "选中的范围", "这个范围", or "这些范围", use selected_clip_ranges from current_selection. Each selected clip range is local to one clip and includes clip_id, track_id, start_seconds, end_seconds, duration_seconds, and clip-local offsets.
For Strip Silence / clip cleanup recommendation requests, use clip.strip_silence.suggest first. Choose scope from intent: all_project for whole-project/all-track cleanup, selected_track for current/selected track cleanup, selected_ranges for selected range cleanup, and selected_clip for current/selected clip cleanup. It analyzes real kernel previews, returns recommended threshold/pad settings, and creates pending clip.strip_silence.apply actions without mutating the project. Use clip.strip_silence.analyze only when the user gives concrete manual parameters. After explicit user confirmation, use one clip.strip_silence.apply for one action or one clip.strip_silence.apply_batch for multiple actions; do not invent strip regions.
When the user says "at the playhead", use playhead_seconds from current_selection.
When importing audio and no target track is named, use selected_track_id as the target. If selected_track_id is absent and multiple tracks exist, ask the user which track to import into.
If commands is non-empty, keep reply as a short internal intent summary. VitAgent will replace it with the final user-facing result after execution, so do not rely on "about to" wording as the final answer.

For plugin loading/grabber setup requests such as loading TDR Nova, finding an EQ/compressor, or loading a plugin and grabbing useful controls, use the special chat workflow command {"cmd":"plugin_grabber_load_and_get_params","track_id":"...","plugin_query":"TDR Nova","intent":"short user intent"}. This workflow searches indexed plugins, asks for confirmation before loading a rack node, then reads parameters after the load succeeds. Do not use instantiate_plugin for these requests; instantiate_plugin requires an exact plugin_path and bypasses the rack grabber workflow.
For explicit plugin effect control, use the deterministic typed tool for that effect. For static EQ, call plugin_grabber.explain_controls and then plugin_grabber.apply_eq_edits(track_id, plugin_id, edits, atomic:true). For a broadband compressor, call plugin_grabber.inspect_compressor first and then plugin_grabber.apply_compressor_controls with only returned control_ref values and explicit physical or enum targets. For an independently provable limiter stage, call plugin_grabber.inspect_limiter first and then plugin_grabber.apply_limiter_controls with only returned control_ref values. For a provable De-esser stage, call plugin_grabber.inspect_de_esser first and then plugin_grabber.apply_de_esser_controls with only returned control_ref values. For a provable hard gate or downward expander, call plugin_grabber.inspect_gate_expander first and then plugin_grabber.apply_gate_expander_controls with only returned control_ref values. For a provable multiband dynamics filterbank, call plugin_grabber.inspect_multiband first and then plugin_grabber.apply_multiband_controls with only returned control_ref values; crossover targets are validated as one strictly ordered set before any write. For Spectral Dynamics, call plugin_grabber.inspect_spectral_dynamics; this v1 surface is inspect-only and returns either a proven spectral field plus global law or an explicit unresolved/adjacent boundary. Do not infer a controller from Capture, Freeze, product identity, or a scalar analyzer. De-esser controls accept value_db, value_ms, frequency_hz, percent, display_value, or enum_label only when the observed role and physical domain prove compatibility. Gate/expander controls accept value_db, ratio, value_ms, frequency_hz, percent, display_value, or enum_label; direction labels must be proved Gate or downward Expander labels. Limiter controls accept value_db, value_ms, percent, display_value, or enum_label; maximizers are supported only through a proved limiter stage, while clippers and multiband dynamics remain separate. Multiband controls accept value_hz, value_db, ratio, value_ms, percent, display_value, or enum_label. Compressor controls accept value_db, ratio, value_ms, percent, display_value, or enum_label; exactly one target field is allowed per control. Typed dynamics tools do not interpret acoustic intent such as "more punch" or "louder": decide the explicit control request before calling them. Every explicit field is a hard requirement: rejected means no parameter was touched. Report exact/quantized/rejected and actual readback exactly. Never use stored mappings or retired control mappings. Generic parameter writes are allowed only for an explicit, user-authorized control request when no typed, recognized effect surface exists; abstract semantic/free-state turns never use this fallback, and no parameter ID or mapping may be invented.
  After writing, call get_plugin_parameters (include_parameters:true) to confirm. Never pass value_text.
For plugin grabber explanation, summary, context pack, or "explain controls" requests on an already loaded/selected plugin, use the special read-only workflow command {"cmd":"plugin_grabber_explain_controls","track_id":"...","plugin_id":"...","intent":"short user intent"}. This workflow reads full parameters, then returns a compact context pack with quick controls, groups, roles, and full-parameter access hints. It does not filter or save parameters.
For basic macro-control creation requests such as creating a generic macro knob/slider, use {"cmd":"control_add_macro","track_id":"...","name":"Macro","control_type":"slider","value":0.5,"bindings":[]}. Do not use rack.add_macro. Semantic macro generation from plugin skills should be proposed for confirmation before writing bindings.
For macro-control rename requests, use {"cmd":"control_rename_macro","macro_id":"...","name":"New Macro Name"}. If the user names the macro by visible label, resolve it from macro_refs or available_macro_controls; do not create a new macro to rename one.
When the user asks to bind/map a plugin parameter to an existing macro control, such as "bind B1 Gain to Macro 1", "bind it to this macro", or "绑定到已有宏控件", do not call control_add_macro first. Use the existing macro_id from macro_refs or available_macro_controls and call {"cmd":"control_add_binding","macro_id":"...","track_id":"...","plugin_id":"...","param_id":"...","param_name":"...","target_min":...,"target_max":...}. If the named macro is ambiguous or absent, ask which macro to use instead of creating a new one.
For plugin parameter writes, use the deterministic typed tool for the recognized effect type. Where no typed tool exists, use set_plugin_param with a normalized value computed from the live display_domain_candidate and verify with get_plugin_parameters only for an explicit, user-authorized control request; abstract semantic/free-state turns never use this fallback, and a live typed topology always wins.
For selected/current clip fade/gain read/write requests, use clip.fade.read/set and clip.gain.read/set. Clip gain is static clip-level gain before track processing; do not route it to mixing, track.volume, mix.propose_tick, or mix.apply_tick.
Mixing is a native Ask Vit conversation capability, not a separate Auto Mix/Co-Mix mode. Do not create a planning card or ask the user to fill one for mixing.
For natural mixing goals such as making a vocal more forward, increasing loudness, reducing mud/harshness, tightening dynamics, or adding space, resolve the target from the user's wording and selected DAW context, then prefer mix_request_observation / mix.request_observation before choosing a write.
For B1 gain-staging fader unity reset, use track.group.apply_control with mode:absolute and db:0 after confirmation; this is an engineering state reset, not a small subjective mix move.
For B2 whole-project static balance, use the dedicated TOM/MOM/project.state CCB plus Mix Style/VMS path and one multi-track pending fader plan; do not reduce B2 to a local single-track move and never use clip gain. For local B3 or explicit small track moves, use mix.propose_tick then mix.apply_tick after confirmation; do not call track.volume directly.
If the user says to undo or roll back the last mix move, use the available undo/rollback path directly instead of returning to a mixing workflow.

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
				"message_data":         item.MessageData,
				"lifecycle":            item.Lifecycle,
				"persistence":          item.Persistence,
				"message_kind":         item.MessageKind,
				"turn_id":              item.TurnID,
				"logical_message_id":   item.LogicalMessageID,
				"supersedes":           item.Supersedes,
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
	for _, key := range []string{"selected_track_id", "selected_track_name", "selected_scene_track_id", "selected_clip_id", "selected_clip_track_id", "selected_clip_range_count", "piano_roll_focus_clip_id", "piano_roll_focus_track_id", "selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_plugin_source", "playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "selected_library_file_path", "selected_library_item_name", "selected_library_kind", "library_search_query"} {
		if value := strings.TrimSpace(fmt.Sprint(requestContext[key])); value != "" && value != "<nil>" {
			safe[key] = value
		}
	}
	if ids := contextStringSlice(requestContext["selected_clip_ids"]); len(ids) > 0 {
		safe["selected_clip_ids"] = ids
	}
	if ranges := contextClipRangeRows(requestContext["selected_clip_ranges"]); len(ranges) > 0 {
		safe["selected_clip_ranges"] = ranges
	}
	if singleRange := firstMapFromAny(requestContext["selected_clip_range"]); len(singleRange) > 0 {
		safe["selected_clip_range"] = singleRange
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

func contextClipRangeRows(v any) []map[string]any {
	rows := mapRowsFromAny(v)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		clipID := firstStringFromMap(row, "clip_id")
		if strings.TrimSpace(clipID) == "" {
			continue
		}
		item := map[string]any{}
		for _, key := range []string{
			"range_id", "clip_id", "clip_name", "track_id",
			"start_seconds", "end_seconds", "duration_seconds",
			"clip_start_seconds", "clip_end_seconds",
			"clip_local_start_seconds", "clip_local_end_seconds",
			"source", "revision",
		} {
			if value, ok := row[key]; ok && !isEmptyContextValue(row, key) {
				item[key] = value
			}
		}
		out = append(out, item)
	}
	return out
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

func contextWithCachedUIContext(in map[string]any, uiContext map[string]any) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if len(uiContext) == 0 {
		return out
	}
	current := cloneContext(mapValue(out["current_selection"]))
	if current == nil {
		current = map[string]any{}
	}
	for key, value := range uiContext {
		if isEmptyContextValue(uiContext, key) {
			continue
		}
		if isEmptyContextValue(out, key) {
			out[key] = value
		}
		if isEmptyContextValue(current, key) {
			current[key] = value
		}
	}
	if len(current) > 0 {
		out["current_selection"] = current
	}
	uiRow := cloneContext(mapValue(out["ui_context"]))
	if uiRow == nil {
		uiRow = map[string]any{}
	}
	for key, value := range uiContext {
		if !isEmptyContextValue(uiContext, key) && isEmptyContextValue(uiRow, key) {
			uiRow[key] = value
		}
	}
	if len(uiRow) > 0 {
		out["ui_context"] = uiRow
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
	if current := mapValue(out["current_selection"]); len(current) > 0 {
		for _, key := range []string{
			"selected_track_id",
			"selected_track_name",
			"selected_clip_id",
			"selected_clip_ids",
			"selected_clip_track_id",
			"selected_clip_name",
			"selected_clip_ranges",
			"selected_clip_range",
			"selected_clip_range_count",
			"piano_roll_focus_clip_id",
			"piano_roll_focus_track_id",
			"playhead_seconds",
			"current_playhead_seconds",
			"transport_position_seconds",
		} {
			if _, exists := out[key]; exists {
				continue
			}
			if value, ok := current[key]; ok && !isEmptyContextValue(current, key) {
				out[key] = value
			}
		}
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
	const promptArtifactLimit = 12
	includePath := len(summaries) == 1
	limit := len(summaries)
	if limit > promptArtifactLimit {
		limit = promptArtifactLimit
	}
	rows := make([]map[string]any, 0, len(summaries))
	ids := make([]string, 0, len(summaries))
	kindCounts := map[string]int{}
	sampleTitles := make([]string, 0, minInt(len(summaries), 8))
	for i, summary := range summaries {
		if strings.TrimSpace(summary.Kind) != "" {
			kindCounts[summary.Kind]++
		}
		if len(sampleTitles) < 8 && strings.TrimSpace(summary.Title) != "" {
			sampleTitles = append(sampleTitles, summary.Title)
		}
		if i >= limit {
			continue
		}
		row := artifactPromptSummaryRow(summary, includePath)
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
	out["artifact_count"] = len(summaries)
	if len(kindCounts) > 0 {
		out["artifact_kind_counts"] = kindCounts
	}
	if len(sampleTitles) > 0 {
		out["artifact_sample_titles"] = sampleTitles
	}
	if len(summaries) > len(rows) {
		out["artifacts_omitted_count"] = len(summaries) - len(rows)
	}
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

func artifactPromptSummaryRow(summary artifacts.Summary, includePath bool) map[string]any {
	out := map[string]any{
		"id":     summary.ID,
		"kind":   summary.Kind,
		"title":  summary.Title,
		"status": summary.Status,
	}
	for key, value := range map[string]any{
		"mime":       summary.MIME,
		"size_bytes": summary.SizeBytes,
		"summary":    summary.Summary,
	} {
		if !isEmptyContextValue(map[string]any{"v": value}, "v") {
			out[key] = value
		}
	}
	if includePath {
		for key, value := range map[string]any{
			"path": summary.Path,
			"url":  summary.URL,
		} {
			if !isEmptyContextValue(map[string]any{"v": value}, "v") {
				out[key] = value
			}
		}
	}
	for key, value := range out {
		if isEmptyContextValue(map[string]any{"v": value}, "v") {
			delete(out, key)
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
	switch x := value.(type) {
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	case map[string]string:
		return len(x) == 0
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

func (s *Server) contextWithCurrentProjectWorkspace(ctx context.Context, in map[string]any) map[string]any {
	out := cloneContext(in)
	if out == nil {
		out = map[string]any{}
	}
	if s == nil || s.harness == nil {
		return out
	}
	projectPath, projectUUID := s.harness.CurrentProjectIdentity(ctx)
	if projectUUID == "" {
		return out
	}
	out["project_path"] = projectPath
	out["current_project_path"] = projectPath
	out["project_uuid"] = projectUUID
	out["project_id"] = projectUUID
	delete(out, "project_history")
	return out
}

func (s *Server) activateCurrentProjectWorkspace(ctx context.Context) {
	if s == nil || s.harness == nil {
		return
	}
	s.schedulerExecutionMu.Lock()
	defer s.schedulerExecutionMu.Unlock()
	projectPath, projectUUID := s.harness.CurrentProjectIdentity(ctx)
	parentProjectUUID := s.harness.CurrentProjectParentUUID()
	if projectUUID == "" {
		return
	}
	// A restarted Agent can receive a project UUID before it has a stable
	// project path. Draft paths are process-local, so recover the durable
	// working session created by the previous process before reading runtime
	// state. The lookup is UUID-scoped and remains inside ProjectHistory.
	if recoveredPath := history.RecoverableProjectPathForUUID(projectPath, projectUUID); recoveredPath != "" && !sameWorkspacePath(recoveredPath, projectPath) {
		projectPath = recoveredPath
	}
	if _, err := s.harness.ActivateProjectStore(projectPath, projectUUID); err != nil {
		if s.logger != nil {
			s.logger.Warn("[workspace] v2 project store activation failed project=%s uuid=%s error=%v", projectPath, projectUUID, err)
		}
		return
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	boundSessionID := history.WorkingSessionID(projectPath)
	sameIdentity := s.activeWorkspaceUUID == projectUUID && sameWorkspacePath(s.activeWorkspacePath, projectPath)
	if sameIdentity && boundSessionID != "" && s.activeWorkspaceSessionID == boundSessionID {
		return
	}
	// Reopening the same project binds a new draft from Saved HEAD. Do not copy
	// the previous unsaved in-memory runtime into that clean working session.
	if s.activeWorkspaceUUID != "" && !(sameIdentity && boundSessionID != "" && boundSessionID != s.activeWorkspaceSessionID) {
		_ = s.persistActiveProjectWorkspaceLocked()
	}
	if parentProjectUUID != "" && parentProjectUUID != projectUUID {
		sourcePath := ""
		if parentProjectUUID == s.activeWorkspaceUUID {
			sourcePath = s.activeWorkspacePath
		}
		if _, recoverErr := history.RecoverMissingProjectFork(sourcePath, parentProjectUUID, projectPath, projectUUID); recoverErr != nil && s.logger != nil {
			s.logger.Warn("[workspace] parent history recovery failed parent=%s target=%s error=%v", parentProjectUUID, projectUUID, recoverErr)
		}
		if sourcePath == "" {
			sourcePath = history.ProjectPathForUUID(projectPath, parentProjectUUID)
		}
		if sourcePath != "" {
			if _, _, loadErr := projectworkspace.LoadAnalysisManifest(projectPath, projectUUID); os.IsNotExist(loadErr) {
				_, _ = projectworkspace.ForkDerived(sourcePath, parentProjectUUID, projectPath, projectUUID)
			}
		}
	}
	session, recovered, err := history.RecoverWorkingSession(projectPath, projectUUID)
	if err == nil && !recovered {
		session, err = history.EnsureWorkingSession(projectPath, projectUUID)
	}
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[workspace] working session activation failed project=%s uuid=%s error=%v", projectPath, projectUUID, err)
		}
		return
	}
	state := projectAgentRuntimeState{}
	if data, err := history.ReadAgentRuntimeState(projectPath, projectUUID); err == nil {
		if decodeErr := json.Unmarshal(data, &state); decodeErr != nil && s.logger != nil {
			s.logger.Warn("[workspace] runtime state decode failed project=%s uuid=%s error=%v", projectPath, projectUUID, decodeErr)
		}
	} else if !os.IsNotExist(err) && s.logger != nil {
		s.logger.Warn("[workspace] runtime state load failed project=%s uuid=%s error=%v", projectPath, projectUUID, err)
	}
	s.mu.Lock()
	s.restoreProjectAgentRuntimeStateLocked(state)
	s.activeWorkspacePath = projectPath
	s.activeWorkspaceUUID = projectUUID
	s.activeWorkspaceSessionID = session.SessionID
	s.mu.Unlock()
	s.wakeContinuationScheduler()
	s.emitRestoredAuditionProjections()
	if s.logger != nil {
		s.logger.Info("[workspace] activated project=%q uuid=%s recovered=%t conversations=%d retired_legacy_b2=%d retired_legacy_b3=%d", projectPath, projectUUID, recovered, len(state.Conversations), len(state.PendingStaticBalancePlans), len(state.PendingPanLayoutPlans))
	}
}

func (s *Server) persistCurrentProjectWorkspace() {
	_ = s.persistCurrentProjectWorkspaceChecked()
}

func (s *Server) persistCurrentProjectWorkspaceChecked() error {
	if s == nil {
		return nil
	}
	s.workspaceMu.Lock()
	defer s.workspaceMu.Unlock()
	s.mu.Lock()
	projectPath, projectUUID, owner := s.activeWorkspacePath, s.activeWorkspaceUUID, s.schedulerOwner
	s.mu.Unlock()
	if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(projectUUID) == "" {
		return s.persistActiveProjectWorkspaceLocked()
	}
	if strings.TrimSpace(owner) == "" {
		owner = "persist_" + strconv.FormatInt(int64(os.Getpid()), 10)
	}
	lease, err := history.AcquireAgentRuntimeStateLock(projectPath, projectUUID, owner, s.continuationLease)
	if err != nil {
		// The scheduler (or another process) owns the durable checkpoint. Its
		// completion write is authoritative; this request must not race it.
		if errors.Is(err, history.ErrAgentRuntimeStateLocked) {
			return nil
		}
		return err
	}
	if err := s.persistActiveProjectWorkspaceLocked(); err != nil {
		_ = lease.Release()
		return err
	}
	return lease.Release()
}

func (s *Server) syncCurrentProjectWorkspace(ctx context.Context) {
	s.activateCurrentProjectWorkspace(ctx)
	s.persistCurrentProjectWorkspace()
}

func (s *Server) persistActiveProjectWorkspaceLocked() error {
	if s.activeWorkspaceUUID == "" || s.activeWorkspacePath == "" {
		return nil
	}
	s.mu.Lock()
	state := s.projectAgentRuntimeStateLocked()
	data, err := json.MarshalIndent(state, "", "  ")
	s.mu.Unlock()
	if err == nil {
		err = history.WriteAgentRuntimeState(s.activeWorkspacePath, s.activeWorkspaceUUID, data)
	}
	if err != nil && s.logger != nil {
		s.logger.Warn("[workspace] runtime state save failed project=%s uuid=%s error=%v", s.activeWorkspacePath, s.activeWorkspaceUUID, err)
	}
	return err
}

func (s *Server) projectAgentRuntimeStateLocked() projectAgentRuntimeState {
	state := projectAgentRuntimeState{
		SchemaVersion:        "vit_project_agent_runtime.v2",
		ProjectPath:          s.activeWorkspacePath,
		ProjectUUID:          s.activeWorkspaceUUID,
		SavedAt:              time.Now().UTC(),
		Conversations:        s.conversations,
		Pending:              s.pending,
		Interactions:         s.interactions,
		MixSessions:          s.mixSessions,
		GoalContinuations:    s.goalContinuations,
		DurableContinuations: s.durableContinuations,
		CapabilityRoutes:     s.capabilityRoutes,
		ConversationGoals:    s.conversationGoals,
		ConversationMemory:   s.conversationMemory,
		PendingMixTicks:      s.pendingMixTicks,
		PendingTreatments:    s.pendingTreatments,
		FreeStateLoops:       s.freeStateLoops,
		AuthorityMode:        normalizeAuthorityModeOrDefault(s.authorityMode),
	}
	if s.audioClosures != nil {
		state.AudioClosures = s.audioClosures.Snapshot()
	}
	if s.controllerOwners != nil {
		state.ControllerOwners = s.controllerOwners.Snapshot()
	}
	if s.pendingManager != nil {
		state.PendingCandidates = s.pendingManager.Snapshot()
	}
	if s.harness != nil {
		state.GoalRuntime = s.harness.RuntimeSnapshot()
	}
	return state
}

func (s *Server) restoreProjectAgentRuntimeStateLocked(state projectAgentRuntimeState) {
	// Collect migration audit rows before clearing legacy pointers from the
	// shared ConversationMemory map.
	retiredCandidates := retireLegacyCapabilityCandidates(state)
	s.conversations = nonNilMap(state.Conversations)
	s.pending = retireLegacyCapabilityPendingPlans(nonNilMap(state.Pending))
	s.interactions = retireLegacyCapabilityInteractions(nonNilMap(state.Interactions))
	s.mixSessions = nonNilMap(state.MixSessions)
	legacyGoalContinuations := nonNilMap(state.GoalContinuations)
	s.goalContinuations = map[string]agentloop.Continuation{}
	s.durableContinuations = nonNilMap(state.DurableContinuations)
	s.capabilityRoutes = restoreCapabilityRoutes(state.CapabilityRoutes)
	s.conversationGoals = nonNilMap(state.ConversationGoals)
	if s.durableContinuations == nil {
		s.durableContinuations = map[string]DurableContinuation{}
	}
	now := time.Now().UTC()
	normalizedContinuations := make(map[string]DurableContinuation, len(s.durableContinuations))
	for continuationID, item := range s.durableContinuations {
		id, normalized := normalizeRestoredDurableContinuation(continuationID, item, state, now)
		normalizedContinuations[id] = normalized
	}
	normalizedContinuations = reconcileDurableCapabilityRoutes(normalizedContinuations, s.capabilityRoutes)
	s.durableContinuations = reconcileRestoredContinuations(normalizedContinuations)
	latestByGoal := map[string]DurableContinuation{}
	for _, normalized := range s.durableContinuations {
		if normalized.GoalID == "" || normalized.Status == ContinuationCompleted || normalized.Status == ContinuationCancelled || normalized.Status == ContinuationFailed {
			continue
		}
		if continuationRecoveryValidationRequired(normalized) {
			continue
		}
		current, exists := latestByGoal[normalized.GoalID]
		if !exists || normalized.UpdatedAt.After(current.UpdatedAt) {
			latestByGoal[normalized.GoalID] = normalized
		}
	}
	for goalID, normalized := range latestByGoal {
		s.goalContinuations[goalID] = normalized.Continuation
	}
	for planID, plan := range s.pending {
		if plan.GoalContinuation == nil {
			continue
		}
		goalID := firstNonEmpty(plan.GoalContinuation.GoalID, firstStringFromMap(plan.Context, "goal_id"))
		if normalized, exists := latestByGoal[goalID]; exists {
			continuation := normalized.Continuation
			plan.GoalContinuation = &continuation
			s.pending[planID] = plan
		}
	}
	// Migrate the pre-C in-memory-shaped continuation snapshot into an
	// explicit durable record. The legacy map remains as a compatibility read
	// index, while all new scheduling decisions use durableContinuations.
	for goalID, cont := range legacyGoalContinuations {
		if strings.TrimSpace(goalID) == "" || strings.TrimSpace(cont.GoalID) == "" {
			continue
		}
		conversationID := ""
		for candidateConversation, candidateGoal := range s.conversationGoals {
			if candidateGoal == goalID {
				conversationID = candidateConversation
				break
			}
		}
		id := continuationIDForContinuation(cont)
		if existing, exists := s.durableContinuations[id]; exists {
			if existing.Status != ContinuationCompleted && existing.Status != ContinuationCancelled && existing.Status != ContinuationFailed {
				s.goalContinuations[goalID] = existing.Continuation
			}
			continue
		}
		cont.ContinuationID = id
		s.goalContinuations[goalID] = cont
		s.durableContinuations[id] = DurableContinuation{
			SchemaVersion: continuationRuntimeSchema, ContinuationID: id,
			TaskID: cont.TaskID, GoalID: firstNonEmpty(cont.GoalID, goalID),
			ConversationID: conversationID, RunID: cont.RunID, CurrentSliceID: cont.SliceID,
			ProjectPath: state.ProjectPath, ProjectUUID: state.ProjectUUID,
			CurrentTurnID: cont.TurnID, OriginalIntent: cont.OriginalIntent,
			ProjectRevision: firstNonEmpty(firstStringFromMap(cont.ProjectHistory, "project_revision", "head", "baseline_commit"), firstStringFromMap(cont.ContextSnapshot, "project_revision")),
			ProjectHistory:  cloneContext(cont.ProjectHistory),
			Continuation:    cont, Status: ContinuationWaitingInteraction,
			PendingInteraction: map[string]any{"status": "legacy_waiting_continue", "reason": "legacy checkpoint has no authoritative stop reason"},
			CreatedAt:          now, UpdatedAt: now,
		}
	}
	s.conversationMemory = retireLegacyCapabilityExecutionMemory(nonNilMap(state.ConversationMemory))
	s.pendingMixTicks = nonNilMap(state.PendingMixTicks)
	s.pendingTreatments = nonNilMap(state.PendingTreatments)
	s.freeStateLoops = nonNilMap(state.FreeStateLoops)
	s.authorityMode = normalizeAuthorityModeOrDefault(state.AuthorityMode)
	if s.audioClosures == nil {
		s.audioClosures = audioclosure.NewMemoryStore()
	}
	if err := s.audioClosures.Restore(state.AudioClosures); err != nil {
		// A corrupt closure projection must fail closed. Keep the store empty;
		// the legacy free-state snapshot remains available for migration/audit.
		s.audioClosures = audioclosure.NewMemoryStore()
		if s.logger != nil {
			s.logger.Warn("[workspace] minimal audio closure restore rejected: %v", err)
		}
	}
	if s.controllerOwners == nil {
		s.controllerOwners = orchestrationcontroller.NewRegistry()
	}
	if err := s.controllerOwners.Restore(state.ControllerOwners); err != nil {
		s.controllerOwners = orchestrationcontroller.NewRegistry()
		if s.logger != nil {
			s.logger.Warn("[workspace] orchestration controller owner restore rejected: %v", err)
		}
	}
	if s.pendingManager == nil {
		s.pendingManager = pendingmanager.NewMemoryManager()
	}
	s.pendingManager.Restore(retiredCandidates)
	if s.harness != nil {
		s.harness.RestoreRuntime(state.GoalRuntime)
		s.reconcileRestoredTaskSemanticProjectionsLocked()
	}
}

func nonNilMap[K comparable, V any](in map[K]V) map[K]V {
	if in == nil {
		return map[K]V{}
	}
	return in
}

func sameWorkspacePath(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
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
	data, err := json.Marshal(body)
	if err != nil {
		status = http.StatusInternalServerError
		data = []byte(`{"status":"error","error":"failed to encode JSON response"}`)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
