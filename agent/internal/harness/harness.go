package harness

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/acousticpackage"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/browsercapture"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/macrocontrols"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/preview"
	"vit-daw-agent/internal/projectworkspace"
	"vit-daw-agent/internal/resourceintake"
	"vit-daw-agent/internal/rollback"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/shelltools"
	"vit-daw-agent/internal/tim"
	"vit-daw-agent/internal/toolpolicy"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/webtools"
	"vit-daw-agent/internal/workflows/plugingrabber"
	"vit-daw-agent/internal/workspace"
)

type Harness struct {
	kernel        KernelSender
	shadow        *shadow.Project
	catalog       *tools.Catalog
	journal       *journal.Journal
	runtime       *agentruntime.Runtime
	mixTicks      *mixTickStore
	logger        *logx.Logger
	snapshotCache *PluginSnapshotCache
	featureMu     sync.Mutex
	waveforms     map[string]*waveformFeatureCollector
	spectrals     map[string]*spectralFeatureCollector
}

type KernelSender interface {
	SendCommand(context.Context, map[string]any) (map[string]any, string, error)
}

type VSPKernelSender interface {
	SendVSPCommand(context.Context, string, map[string]any) (*kernel.VSPCommandResult, error)
	SendVSPLegacyCommand(context.Context, map[string]any) (*kernel.VSPCommandResult, error)
	VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error)
	VSPStateDelta(context.Context, int64, string) (*kernel.VSPStateResult, error)
	VSPStateResync(context.Context, string) (*kernel.VSPStateResult, error)
}

type kernelExecutionResult struct {
	reply        map[string]any
	raw          string
	vsp          map[string]any
	usedVSP      bool
	fallbackSafe bool
	err          error
}

const mixboardFeatureReadyWaitDefault = 1500 * time.Millisecond
const mixboardFeatureSubscriberWarmupDefault = 200 * time.Millisecond
const mixboardFeatureSubURL = "tcp://127.0.0.1:5556"

var mixboardObservationFeatureTypes = []string{"waveform_envelope", "spectral_field", "l3_acoustic_summary"}

var mixboardFeatureSnapshotWriteMu sync.Mutex

type InvokeRequest struct {
	Tool       string         `json:"tool"`
	Args       map[string]any `json:"args"`
	Command    map[string]any `json:"command"`
	Context    map[string]any `json:"context,omitempty"`
	Source     string         `json:"source"`
	Confirmed  bool           `json:"confirmed"`
	RunID      string         `json:"run_id,omitempty"`
	GoalID     string         `json:"goal_id,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type InvokeResponse struct {
	Status               string          `json:"status"`
	AgentActionID        string          `json:"agent_action_id,omitempty"`
	Tool                 string          `json:"tool,omitempty"`
	CommandName          string          `json:"command_name,omitempty"`
	RiskLevel            tools.RiskLevel `json:"risk_level,omitempty"`
	RequiresConfirmation bool            `json:"requires_confirmation"`
	Preview              string          `json:"preview,omitempty"`
	UndoLabel            string          `json:"undo_label,omitempty"`
	Result               map[string]any  `json:"result,omitempty"`
	ProjectHistory       map[string]any  `json:"project_history,omitempty"`
	Error                string          `json:"error,omitempty"`
}

func New(kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Harness {
	if kernelClient == nil {
		return newWithSender(nil, shadowProject, logger)
	}
	return newWithSender(kernelClient, shadowProject, logger)
}

func NewWithSender(sender KernelSender, shadowProject *shadow.Project, logger *logx.Logger) *Harness {
	return newWithSender(sender, shadowProject, logger)
}

func newWithSender(sender KernelSender, shadowProject *shadow.Project, logger *logx.Logger) *Harness {
	j, err := journal.NewPersistent(500, defaultJournalPath())
	if err != nil {
		j = journal.New(500)
	}
	return &Harness{
		kernel:        sender,
		shadow:        shadowProject,
		catalog:       tools.DefaultCatalog(),
		journal:       j,
		runtime:       agentruntime.New(),
		mixTicks:      newMixTickStore(),
		snapshotCache: NewPluginSnapshotCache(),
		logger:        logger,
		waveforms:     map[string]*waveformFeatureCollector{},
		spectrals:     map[string]*spectralFeatureCollector{},
	}
}

func defaultJournalPath() string {
	if envPath := strings.TrimSpace(os.Getenv("VIT_AGENT_JOURNAL_PATH")); envPath != "" {
		if strings.EqualFold(envPath, "off") || strings.EqualFold(envPath, "memory") || strings.EqualFold(envPath, "disabled") {
			return ""
		}
		return envPath
	}
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		if filepath.Base(dir) == "agent" {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return filepath.Join(filepath.Dir(dir), "VitApp", "Workspace", "Logs", "agent_journal.json")
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return filepath.Join(wd, "VitApp", "Workspace", "Logs", "agent_journal.json")
}

func (h *Harness) Tools() []tools.Tool {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.Tools()
}

func (h *Harness) Commands() []tools.CommandSpec {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.Commands()
}

func (h *Harness) Actions(limit int) []journal.Action {
	if h == nil || h.journal == nil {
		return nil
	}
	return h.journal.Recent(limit)
}

func (h *Harness) Runtime() *agentruntime.Runtime {
	if h == nil {
		return nil
	}
	return h.runtime
}

func (h *Harness) EnsureGoal(goalID, runID, summary string) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.Ensure(goalID, runID, summary)
}

func (h *Harness) ContinueGoal(goalID, summary string) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.Continue(goalID, summary)
}
func (h *Harness) ClearGoalInterjections(goalID string) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.ClearInterjections(goalID)
}
func (h *Harness) RuntimeStatus(goalID string) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.Status(goalID)
}

func (h *Harness) RuntimeSnapshot() agentruntime.Snapshot {
	if h == nil || h.runtime == nil {
		return agentruntime.Snapshot{}
	}
	return h.runtime.Snapshot()
}

func (h *Harness) RestoreRuntime(snapshot agentruntime.Snapshot) {
	if h == nil || h.runtime == nil {
		return
	}
	h.runtime.Restore(snapshot)
}

func (h *Harness) BeginGoal(summary string) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.Create(summary)
}

func (h *Harness) SetGoalStatus(goalID string, status agentruntime.GoalStatus, err error) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.SetStatus(goalID, status, err)
}

func (h *Harness) CompleteGoal(goalID string, err error) agentruntime.Goal {
	if h == nil || h.runtime == nil {
		return agentruntime.Goal{Status: agentruntime.StatusIdle}
	}
	return h.runtime.Complete(goalID, err)
}

func (h *Harness) JournalGet(targetID string) (journal.Action, bool) {
	if h == nil || h.journal == nil {
		return journal.Action{}, false
	}
	return h.journal.Get(targetID)
}

func (h *Harness) JournalMarkRollback(targetActionID, rollbackActionID, state string) {
	if h == nil || h.journal == nil {
		return
	}
	h.journal.MarkRollback(targetActionID, rollbackActionID, state)
}

func (h *Harness) KernelSendCommand(ctx context.Context, command map[string]any) (map[string]any, string, error) {
	if h == nil || h.kernel == nil {
		return nil, "", fmt.Errorf("kernel client is nil")
	}
	return h.kernel.SendCommand(ctx, command)
}

func (h *Harness) vspKernel() (VSPKernelSender, bool) {
	if h == nil || h.kernel == nil {
		return nil, false
	}
	vsp, ok := h.kernel.(VSPKernelSender)
	return vsp, ok
}

func (h *Harness) WorkspaceContext(cmd map[string]any) workspace.Context {
	return h.workspaceContext(cmd)
}

func (h *Harness) RefreshShadow(ctx context.Context, reason string) {
	h.refreshShadow(ctx, reason)
}

func (h *Harness) DirectCommandNames() []string {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.DirectCommandNames()
}

func (h *Harness) StateSummary(ctx context.Context) map[string]any {
	if h == nil || h.shadow == nil {
		return map[string]any{"initialized": false}
	}
	if !h.shadow.Initialized() {
		h.refreshShadow(ctx, "state_summary")
	}
	return h.shadow.Summary()
}

func (h *Harness) UserStateSummary(ctx context.Context) map[string]any {
	return userVisibleState(h.StateSummary(ctx))
}

func (h *Harness) CurrentProjectIdentity(ctx context.Context) (string, string) {
	if h == nil || h.shadow == nil {
		return "", ""
	}
	state := userVisibleState(h.shadow.Summary())
	projectPath := projectPathFromState(state)
	projectUUID := firstString(state, "project_uuid", "project_id")
	if projectUUID != "" {
		projectPath = history.BindProjectIdentity(projectPath, projectUUID)
	}
	_ = ctx
	return projectPath, projectUUID
}

func (h *Harness) CurrentProjectParentUUID() string {
	if h == nil || h.shadow == nil {
		return ""
	}
	return firstString(userVisibleState(h.shadow.Summary()), "parent_project_uuid")
}

func (h *Harness) ProjectHistorySummary(ctx context.Context, goalID string) map[string]any {
	return h.ProjectHistorySummaryForProject(ctx, goalID, "")
}

func (h *Harness) ProjectHistorySummaryForProject(ctx context.Context, goalID, projectPath string) map[string]any {
	currentPath, projectUUID := h.CurrentProjectIdentity(ctx)
	if projectUUID != "" && (strings.TrimSpace(projectPath) == "" || history.IsDraftProjectPath(projectPath) || samePath(projectPath, currentPath)) {
		projectPath = currentPath
	}
	args := map[string]any{}
	if h != nil {
		args = h.historyArgs(map[string]any{"project_path": strings.TrimSpace(projectPath)})
	}
	out := map[string]any{"available": false}
	status, err := history.Status(args)
	if err != nil {
		out["warnings"] = []string{err.Error()}
		return out
	}
	out = cloneAnyMap(status)
	out["available"] = true
	if worktrees, ok := out["worktrees"].([]map[string]any); ok {
		out["worktree_count"] = len(worktrees)
	}
	if h != nil && h.runtime != nil {
		goal := h.runtime.Status(goalID)
		if goal.ProjectHistory != nil {
			out["baseline_commit"] = goal.ProjectHistory.BaselineCommitID
			out["baseline_created"] = goal.ProjectHistory.BaselineCreated
			if goal.ProjectHistory.ActiveBranch != "" {
				out["goal_active_branch"] = goal.ProjectHistory.ActiveBranch
			}
			out["goal_detached"] = goal.ProjectHistory.Detached
		}
	}
	list, err := history.List(args)
	if err == nil {
		out["recent_checkpoints"] = compactHistoryCommits(list["commits"], 6)
	}
	return out
}

func (h *Harness) RecordConversationNode(ctx context.Context, kind, textPreview, goalID, runID string) map[string]any {
	return h.RecordConversationNodeForProject(ctx, "", kind, textPreview, goalID, runID)
}

func (h *Harness) RecordConversationNodeForProject(ctx context.Context, projectPath, kind, textPreview, goalID, runID string) map[string]any {
	return h.RecordConversationNodeForProjectWithData(ctx, projectPath, kind, textPreview, goalID, runID, nil)
}

func (h *Harness) RecordConversationNodeForProjectWithData(ctx context.Context, projectPath, kind, textPreview, goalID, runID string, data map[string]any) map[string]any {
	if h == nil {
		return nil
	}
	kind = strings.TrimSpace(kind)
	args := h.historyArgs(map[string]any{
		"project_path": strings.TrimSpace(projectPath),
		"kind":         kind,
		"text_preview": textPreview,
		"goal_id":      goalID,
		"run_id":       runID,
	})
	for key, value := range data {
		args[key] = value
	}
	if strings.EqualFold(kind, "vit") {
		message := "Conversation vit"
		if strings.TrimSpace(textPreview) != "" {
			message = "Vit: " + strings.TrimSpace(textPreview)
		}
		checkpointArgs := h.enrichHistoryCheckpointArgs(map[string]any{
			"project_path":    args["project_path"],
			"message":         message,
			"goal_id":         goalID,
			"run_id":          runID,
			"source":          "conversation_graph",
			"checkpoint_kind": "manual",
		})
		if checkpoint, checkpointErr := history.Checkpoint(checkpointArgs); checkpointErr == nil {
			if checkpointID := firstString(checkpoint, "commit_id"); checkpointID != "" {
				args["commit_id"] = checkpointID
			}
		}
	}
	result, err := history.AppendConversationNode(args)
	if err != nil && strings.Contains(err.Error(), "commit_id or current HEAD is required") {
		checkpointArgs := h.enrichHistoryCheckpointArgs(map[string]any{
			"project_path":    args["project_path"],
			"message":         "Conversation " + firstNonEmpty(strings.TrimSpace(kind), "node"),
			"goal_id":         goalID,
			"run_id":          runID,
			"source":          "conversation_graph",
			"checkpoint_kind": "manual",
		})
		if checkpoint, checkpointErr := history.Checkpoint(checkpointArgs); checkpointErr == nil {
			checkpointID := firstString(checkpoint, "commit_id")
			args["commit_id"] = checkpointID
			if h.runtime != nil && strings.TrimSpace(goalID) != "" && checkpointID != "" {
				meta := projectHistoryMetaFromResult(checkpoint)
				meta.BaselineCommitID = checkpointID
				meta.BaselineCreated = true
				h.runtime.SetProjectHistory(goalID, meta)
			}
			result, err = history.AppendConversationNode(args)
		} else {
			err = checkpointErr
		}
	}
	if err != nil {
		out := h.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		if len(out) > 0 {
			out["warnings"] = appendStringAny(out["warnings"], "conversation graph update failed: "+err.Error())
		}
		return out
	}
	out := cloneAnyMap(result)
	out["available"] = true
	return out
}

func (h *Harness) ModelCatalogSummary() string {
	if h == nil || h.catalog == nil {
		return ""
	}
	return h.catalog.ModelSummary()
}

func (h *Harness) ModelCatalogSummaryForTools(toolNames []string) string {
	if h == nil || h.catalog == nil {
		return ""
	}
	return h.catalog.ModelSummaryForTools(toolNames)
}

func (h *Harness) ModelCatalogSummaryForCapabilityPacks(packNames, toolNames []string) string {
	if h == nil || h.catalog == nil {
		return ""
	}
	return h.catalog.ModelSummaryForCapabilityPacks(packNames, toolNames)
}

func (h *Harness) Invoke(ctx context.Context, req InvokeRequest) (resp InvokeResponse, err error) {
	started := time.Now()
	defer func() {
		if h != nil && h.logger != nil {
			h.logger.Info("[timing] harness.invoke total_ms=%d tool=%s command=%s status=%s confirmed=%t err=%t",
				time.Since(started).Milliseconds(),
				firstNonEmpty(resp.Tool, req.Tool),
				firstNonEmpty(resp.CommandName, tools.CommandName(req.Command)),
				resp.Status,
				req.Confirmed,
				err != nil,
			)
		}
	}()
	if h == nil {
		return InvokeResponse{Status: "error", Error: "harness is nil"}, fmt.Errorf("harness is nil")
	}
	cmd, spec, err := h.resolveCommand(req)
	if err != nil {
		resp := InvokeResponse{Status: "error", Error: err.Error()}
		h.logPreJournalInvokeFailure("resolve_command", req, tools.CommandSpec{}, nil, err)
		return resp, err
	}
	if err := broadMixObserveFirstWriteGuard(req.Context, spec, cmd); err != nil {
		resp := InvokeResponse{
			Status:      "error",
			Tool:        spec.ToolName,
			CommandName: spec.CommandName,
			RiskLevel:   spec.RiskLevel,
			Error:       err.Error(),
		}
		h.logPreJournalInvokeFailure("broad_mix_observe_first_write_guard", req, spec, cmd, err)
		return resp, err
	}
	if err := h.resolveImplicitTargets(ctx, spec, cmd, req.Context); err != nil {
		resp := InvokeResponse{
			Status:      "error",
			Tool:        spec.ToolName,
			CommandName: spec.CommandName,
			RiskLevel:   spec.RiskLevel,
			Error:       err.Error(),
		}
		h.logPreJournalInvokeFailure("resolve_implicit_targets", req, spec, cmd, err)
		return resp, err
	}
	if err := validateRequiredTargetIDs(spec, cmd); err != nil {
		resp := InvokeResponse{
			Status:      "error",
			Tool:        spec.ToolName,
			CommandName: spec.CommandName,
			RiskLevel:   spec.RiskLevel,
			Error:       err.Error(),
		}
		h.logPreJournalInvokeFailure("validate_required_target_ids", req, spec, cmd, err)
		return resp, err
	}
	if spec.CommandName == "set_plugin_param" {
		if err := h.validateSetPluginParam(cmd); err != nil {
			resp := InvokeResponse{
				Status:      "error",
				Tool:        spec.ToolName,
				CommandName: spec.CommandName,
				RiskLevel:   spec.RiskLevel,
				Error:       err.Error(),
			}
			h.logPreJournalInvokeFailure("validate_set_plugin_param", req, spec, cmd, err)
			return resp, err
		}
	}

	actionID := "act_" + randomID()
	runID, goalID, toolCallID := h.executionIDs(req)
	undoLabel := buildUndoLabel(spec, cmd)
	cmd["agent_action_id"] = actionID
	if runID != "" {
		cmd["run_id"] = runID
	}
	if goalID != "" {
		cmd["goal_id"] = goalID
	}
	if toolCallID != "" {
		cmd["tool_call_id"] = toolCallID
	}
	if undoLabel != "" {
		cmd["undo_label"] = undoLabel
	}
	if req.Confirmed {
		cmd["confirmation"] = true
		cmd["confirmed"] = true
	}

	needsConfirmation := spec.RequiresConfirmation && !req.Confirmed
	action := journal.Action{
		AgentActionID:        actionID,
		RunID:                runID,
		GoalID:               goalID,
		ToolCallID:           toolCallID,
		Domain:               spec.Domain,
		Source:               fallback(req.Source, "agent"),
		Summary:              spec.Description,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		Command:              cmd,
		RiskLevel:            string(spec.RiskLevel),
		RequiresConfirmation: spec.RequiresConfirmation,
		ConfirmationStatus:   confirmationStatus(needsConfirmation, req.Confirmed),
		Status:               journal.StatusRunning,
		UndoLabel:            undoLabel,
	}
	if needsConfirmation {
		action.Status = journal.StatusPendingConfirmation
		h.journal.Record(action)
		preview := PreviewCommand(spec, cmd)
		if shouldAutoGoalBaseline(spec) && goalID != "" {
			preview = "\u786e\u8ba4\u540e\uff0cVitAgent \u4f1a\u5148\u521b\u5efa\u4e00\u4e2a\u9879\u76ee\u5386\u53f2\u5b89\u5168\u68c0\u67e5\u70b9\u3002\n" + preview
		}
		resp := InvokeResponse{
			Status:               "needs_confirmation",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: true,
			Preview:              preview,
			UndoLabel:            undoLabel,
			ProjectHistory:       h.ProjectHistorySummary(ctx, goalID),
		}
		return resp, nil
	}

	if baselineID, err := h.ensureProjectHistoryBaseline(ctx, spec, cmd, goalID, runID); err != nil {
		action.Status = journal.StatusFailed
		action.Error = err.Error()
		h.journal.Record(action)
		resp := InvokeResponse{
			Status:               "error",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: false,
			UndoLabel:            undoLabel,
			ProjectHistory:       h.ProjectHistorySummary(ctx, goalID),
			Error:                err.Error(),
		}
		return resp, err
	} else if baselineID != "" {
		action.VersionCommitID = baselineID
	}

	h.journal.Record(action)
	if result, ok := h.invokeLocal(ctx, spec, cmd, req.Context); ok {
		if strings.EqualFold(strings.TrimSpace(fmt.Sprint(result["status"])), "error") {
			err := fmt.Errorf("%s", firstNonEmpty(fmt.Sprint(result["error"]), "local tool failed"))
			h.journal.MarkResult(actionID, journal.StatusFailed, result, err)
			resp := InvokeResponse{
				Status:               "error",
				AgentActionID:        actionID,
				Tool:                 spec.ToolName,
				CommandName:          spec.CommandName,
				RiskLevel:            spec.RiskLevel,
				RequiresConfirmation: false,
				Result:               result,
				UndoLabel:            undoLabel,
				ProjectHistory:       h.ProjectHistorySummary(ctx, goalID),
				Error:                err.Error(),
			}
			return resp, err
		}
		h.journal.MarkResult(actionID, journal.StatusSucceeded, result, nil)
		resp := InvokeResponse{
			Status:               "ok",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: false,
			Result:               result,
			UndoLabel:            undoLabel,
			ProjectHistory:       h.ProjectHistorySummary(ctx, goalID),
		}
		return resp, nil
	}
	if h.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:         "error",
			AgentActionID:  actionID,
			Tool:           spec.ToolName,
			CommandName:    spec.CommandName,
			RiskLevel:      spec.RiskLevel,
			UndoLabel:      undoLabel,
			ProjectHistory: h.ProjectHistorySummary(ctx, goalID),
			Error:          err.Error(),
		}
		return resp, err
	}

	if err := h.prepareProjectLifecycleCommand(spec, cmd); err != nil {
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		return InvokeResponse{
			Status: "error", AgentActionID: actionID, Tool: spec.ToolName,
			CommandName: spec.CommandName, RiskLevel: spec.RiskLevel,
			ProjectHistory: h.ProjectHistorySummary(ctx, goalID), Error: err.Error(),
		}, err
	}
	translateCommandForKernel(spec, cmd)
	kernelStarted := time.Now()
	execution := h.executeKernelCommand(ctx, spec, cmd)
	reply := execution.reply
	err = execution.err
	if h.logger != nil {
		h.logger.Info("[timing] kernel.send_command ms=%d command=%s tool=%s confirmed=%t err=%t",
			time.Since(kernelStarted).Milliseconds(), spec.CommandName, spec.ToolName, req.Confirmed, err != nil)
	}
	if err != nil {
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:         "error",
			AgentActionID:  actionID,
			Tool:           spec.ToolName,
			CommandName:    spec.CommandName,
			RiskLevel:      spec.RiskLevel,
			UndoLabel:      undoLabel,
			ProjectHistory: h.ProjectHistorySummary(ctx, goalID),
			Error:          err.Error(),
		}
		return resp, err
	}
	if spec.CommandName == "project.audio_analysis_status" && strings.EqualFold(firstString(reply, "status"), "error") {
		if recovered, recoverErr := h.recoverAudioAnalysisManifest(ctx); recoverErr == nil {
			reply = recovered
			execution.reply = recovered
		}
	}

	status := "ok"
	journalStatus := journal.StatusSucceeded
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		status = "kernel_error"
		journalStatus = journal.StatusFailed
		err = fmt.Errorf("%s", firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel command failed"))
	}
	h.journal.MarkResult(actionID, journalStatus, reply, err)
	result := h.publicResult(spec, cmd, reply)
	attachVSPExecutionResult(result, execution.vsp)
	if status == "ok" {
		if workspace, workspaceErr := h.applyProjectLifecycle(ctx, spec, cmd, result); workspaceErr != nil {
			result["project_workspace_warning"] = workspaceErr.Error()
		} else if len(workspace) > 0 {
			result["project_workspace"] = workspace
		}
	}
	resp = InvokeResponse{
		Status:               status,
		AgentActionID:        actionID,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		RiskLevel:            spec.RiskLevel,
		RequiresConfirmation: false,
		Result:               result,
		UndoLabel:            undoLabel,
		ProjectHistory:       h.ProjectHistorySummary(ctx, goalID),
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, err
}

func (h *Harness) prepareProjectLifecycleCommand(spec tools.CommandSpec, cmd map[string]any) error {
	if h == nil || cmd == nil {
		return nil
	}
	switch spec.CommandName {
	case "save_as_project", "save_project":
		projectPath, projectUUID := h.CurrentProjectIdentity(context.Background())
		if projectPath == "" || projectUUID == "" {
			return nil
		}
		if projectPath != "" {
			cmd["source_project_path"] = projectPath
		}
		if projectUUID != "" {
			cmd["source_project_uuid"] = projectUUID
		}
		saveKind := "save"
		if spec.CommandName == "save_as_project" {
			saveKind = "save_as"
		}
		prepared, err := history.PrepareWorkingSessionSave(projectPath, projectUUID, saveKind)
		if err != nil {
			return err
		}
		cmd["history_prepare_id"] = firstString(prepared, "prepare_id")
		cmd["agent_history_generation"] = firstString(prepared, "agent_history_generation", "generation_id")
	}
	return nil
}

func (h *Harness) applyProjectLifecycle(ctx context.Context, spec tools.CommandSpec, cmd, result map[string]any) (map[string]any, error) {
	lifecycle := firstString(result, "project_lifecycle")
	if lifecycle == "" {
		switch spec.CommandName {
		case "open_project":
			lifecycle = "open"
		case "new_project":
			lifecycle = "new"
		case "save_as_project":
			lifecycle = "save_as"
		case "save_project":
			lifecycle = "save"
		default:
			return nil, nil
		}
	}
	projectPath := firstNonEmpty(firstString(result, "project_path", "current_project_path"), firstString(cmd, "file_path", "project_path"))
	projectUUID := firstString(result, "project_uuid", "project_id")
	if projectUUID == "" {
		_, projectUUID = h.CurrentProjectIdentity(ctx)
	}
	switch lifecycle {
	case "new":
		created, err := history.ProjectNew(map[string]any{})
		if err != nil {
			return nil, err
		}
		projectPath = history.BindProjectIdentity(firstString(created, "project_path", "draft_project_path"), projectUUID)
		if _, err := history.EnsureWorkingSession(projectPath, projectUUID); err != nil {
			return nil, err
		}
		return history.Status(map[string]any{"project_path": projectPath})
	case "open":
		// The command path is the authoritative host input. Some legacy kernel
		// transports can damage non-ASCII path text in the echoed lifecycle reply.
		projectPath = firstNonEmpty(firstString(cmd, "file_path", "project_path"), projectPath)
		if projectPath == "" || projectUUID == "" {
			return nil, errors.New("project lifecycle reply omitted project path or UUID")
		}
		parentUUID := firstString(result, "parent_project_uuid")
		generationID := firstString(result, "agent_history_generation", "history_generation")
		recovered, err := history.RecoverPreparedSaveOnOpen(
			projectPath, projectUUID, firstString(result, "source_project_path"), parentUUID,
			generationID, firstString(result, "history_prepare_id"),
		)
		if err != nil {
			return nil, err
		}
		if parentUUID != "" && parentUUID != projectUUID && !boolValueDefault(recovered["recovered"], false) {
			if _, err := history.RecoverMissingProjectFork("", parentUUID, projectPath, projectUUID); err != nil {
				return nil, err
			}
		}
		history.BindProjectIdentity(projectPath, projectUUID)
		if _, err := history.OpenWorkingSessionAtGeneration(projectPath, projectUUID, generationID); err != nil {
			return nil, err
		}
		return history.Status(map[string]any{"project_path": projectPath})
	case "save":
		if projectPath == "" || projectUUID == "" {
			return nil, errors.New("project lifecycle reply omitted project path or UUID")
		}
		return history.CommitPreparedWorkingSession(
			projectPath, projectUUID, projectPath, projectUUID,
			firstNonEmpty(firstString(result, "history_prepare_id"), firstString(cmd, "history_prepare_id")),
			firstNonEmpty(firstString(result, "agent_history_generation", "history_generation"), firstString(cmd, "agent_history_generation")),
			"save",
		)
	case "save_as":
		sourcePath := firstString(cmd, "source_project_path")
		sourceUUID := firstNonEmpty(firstString(result, "source_project_uuid"), firstString(cmd, "source_project_uuid"))
		if sourcePath == "" || sourceUUID == "" || projectPath == "" || projectUUID == "" {
			return nil, errors.New("save as lifecycle omitted source/target project identity")
		}
		workspace, err := history.CommitPreparedWorkingSession(
			sourcePath, sourceUUID, projectPath, projectUUID,
			firstNonEmpty(firstString(result, "history_prepare_id"), firstString(cmd, "history_prepare_id")),
			firstNonEmpty(firstString(result, "agent_history_generation", "history_generation"), firstString(cmd, "agent_history_generation")),
			"save_as",
		)
		if err != nil {
			return nil, err
		}
		derivedDir, derivedErr := projectworkspace.ForkDerived(sourcePath, sourceUUID, projectPath, projectUUID)
		if derivedErr != nil {
			workspace["derived_fork_warning"] = derivedErr.Error()
		} else if derivedDir != "" {
			workspace["derived_dir"] = derivedDir
			workspace["forked_derived"] = true
		}
		return workspace, nil
	default:
		return nil, nil
	}
}

func (h *Harness) logPreJournalInvokeFailure(stage string, req InvokeRequest, spec tools.CommandSpec, cmd map[string]any, err error) {
	if h == nil || h.logger == nil || err == nil {
		return
	}
	toolName := firstNonEmpty(spec.ToolName, req.Tool)
	commandName := firstNonEmpty(spec.CommandName, tools.CommandName(req.Command))
	keys := invokeCommandKeys(cmd)
	if len(keys) == 0 {
		keys = invokeCommandKeys(req.Command)
	}
	h.logger.Warn("[harness] invoke pre_journal_validation_failed stage=%s tool=%s command=%s confirmed=%t arg_keys=%s error=%s",
		strings.TrimSpace(stage),
		toolName,
		commandName,
		req.Confirmed,
		strings.Join(keys, ","),
		err.Error(),
	)
}

func invokeCommandKeys(cmd map[string]any) []string {
	if len(cmd) == 0 {
		return nil
	}
	keys := make([]string, 0, len(cmd))
	for key := range cmd {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (h *Harness) executionIDs(req InvokeRequest) (string, string, string) {
	runID := strings.TrimSpace(req.RunID)
	goalID := strings.TrimSpace(req.GoalID)
	toolCallID := strings.TrimSpace(req.ToolCallID)
	if req.Context != nil {
		if runID == "" {
			runID = firstString(req.Context, "run_id")
		}
		if goalID == "" {
			goalID = firstString(req.Context, "goal_id")
		}
		if toolCallID == "" {
			toolCallID = firstString(req.Context, "tool_call_id")
		}
	}
	if toolCallID == "" {
		toolCallID = agentruntime.NewToolCallID()
	}
	if h != nil && h.runtime != nil && goalID != "" {
		goal := h.runtime.Ensure(goalID, runID, "")
		if runID == "" {
			runID = goal.RunID
		}
	}
	if runID == "" && goalID != "" {
		runID = "run_" + randomID()
	}
	return runID, goalID, toolCallID
}

func translateCommandForKernel(spec tools.CommandSpec, cmd map[string]any) {
	// These are registered in the Agent catalog for tool calling, but the kernel
	// compatibility handler still exposes the n_* command names.
	switch spec.CommandName {
	case "plugin_grabber_explain_controls":
		cmd["cmd"] = "get_plugin_parameters"
	case "plugin_grabber_get_project_profiles":
		cmd["cmd"] = "n_get_project_profiles"
	case "plugin_grabber_upsert_project_profile":
		cmd["cmd"] = "n_project_profile"
	case "plugin_grabber_remove_project_profile":
		cmd["cmd"] = "n_remove_project_profile"
	case "plugin_grabber_apply_control":
		cmd["cmd"] = "n_apply_control"
	case "apply_midi_note_patch":
		translateInsertNotePatchToLegacyCommand(cmd)
	}
}

func (h *Harness) executeKernelCommand(ctx context.Context, spec tools.CommandSpec, cmd map[string]any) kernelExecutionResult {
	if vsp, ok := h.vspKernel(); ok {
		execution := h.executeKernelCommandVSP(ctx, vsp, spec, cmd)
		if execution.usedVSP || !execution.fallbackSafe {
			return execution
		}
		if h.logger != nil && execution.err != nil {
			h.logger.Warn("[harness] VSP unavailable for %s, falling back to legacy command path: %v", spec.CommandName, execution.err)
		}
	}

	reply, raw, err := h.kernel.SendCommand(ctx, cmd)
	if err == nil {
		refreshStarted := time.Now()
		h.afterKernelReply(ctx, spec, reply)
		if h.logger != nil {
			h.logger.Info("[timing] harness.after_kernel_reply ms=%d command=%s status=%s",
				time.Since(refreshStarted).Milliseconds(), spec.CommandName, strings.TrimSpace(fmt.Sprint(reply["status"])))
		}
	}
	return kernelExecutionResult{reply: reply, raw: raw, err: err}
}

func (h *Harness) executeKernelCommandVSP(ctx context.Context, vsp VSPKernelSender, spec tools.CommandSpec, cmd map[string]any) kernelExecutionResult {
	if spec.CommandName == "get_project_state" {
		observe, err := vsp.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil {
			return kernelExecutionResult{fallbackSafe: true, err: err}
		}
		if observe == nil {
			return kernelExecutionResult{fallbackSafe: true, err: fmt.Errorf("vsp state snapshot returned nil")}
		}
		reply := cloneAnyMap(observe.LegacyState)
		h.afterKernelReplyVSP(ctx, spec, reply, observe)
		return kernelExecutionResult{
			reply:   reply,
			raw:     observe.Raw,
			vsp:     vspExecutionSummary(nil, observe, nil, nil, nil),
			usedVSP: true,
		}
	}

	writeLike := spec.MutatesProject || spec.RefreshAfter
	var before *kernel.VSPStateResult
	if writeLike {
		var err error
		before, err = vsp.VSPStateSnapshot(ctx, "project.timeline")
		if err != nil {
			return kernelExecutionResult{fallbackSafe: true, err: err}
		}
		if before == nil {
			return kernelExecutionResult{fallbackSafe: true, err: fmt.Errorf("vsp state snapshot returned nil")}
		}
		if before.OK() && h.shadow != nil {
			h.shadow.Initialize(before.LegacyState)
		}
	}

	var commandResult *kernel.VSPCommandResult
	var err error
	if commandName := vspCanonicalCommandForSpec(spec.CommandName); commandName != "" {
		commandResult, err = vsp.SendVSPCommand(ctx, commandName, vspArgsFromCommand(cmd))
	} else {
		commandResult, err = vsp.SendVSPLegacyCommand(ctx, cmd)
	}
	if err != nil {
		return kernelExecutionResult{usedVSP: true, err: err}
	}
	if commandResult == nil {
		return kernelExecutionResult{usedVSP: true, err: fmt.Errorf("vsp command returned nil")}
	}

	reply := commandResult.LegacyLikeReply()
	var delta *kernel.VSPStateResult
	var resync *kernel.VSPStateResult
	var warnings []string
	if kernelReplySucceeded(reply) && writeLike && before != nil && before.Revision > 0 {
		delta, err = vsp.VSPStateDelta(ctx, before.Revision, "project.timeline")
		if err != nil {
			warnings = append(warnings, "vsp state delta failed: "+err.Error())
		} else if delta.ResyncHint || !delta.OK() {
			resync, err = vsp.VSPStateResync(ctx, "project.timeline")
			if err != nil {
				warnings = append(warnings, "vsp state resync failed: "+err.Error())
			}
		} else if len(delta.Ops) > 0 {
			resync, err = vsp.VSPStateResync(ctx, "project.timeline")
			if err != nil {
				warnings = append(warnings, "vsp state resync after delta failed: "+err.Error())
			}
		}
	}

	observed := delta
	if resync != nil {
		observed = resync
	}
	h.afterKernelReplyVSP(ctx, spec, reply, observed)
	summary := vspExecutionSummary(commandResult, nil, delta, resync, warnings)
	return kernelExecutionResult{
		reply:   reply,
		raw:     commandResult.Raw,
		vsp:     summary,
		usedVSP: true,
	}
}

func (h *Harness) afterKernelReplyVSP(ctx context.Context, spec tools.CommandSpec, reply map[string]any, observed *kernel.VSPStateResult) {
	_ = ctx
	if !kernelReplySucceeded(reply) {
		return
	}
	if spec.CommandName == "get_plugin_parameters" || spec.CommandName == "plugin_grabber_explain_controls" {
		h.ObservePluginParametersReply(reply)
	}
	if h.shadow != nil && observed != nil && observed.OK() && len(observed.LegacyState) > 0 {
		h.shadow.Initialize(observed.LegacyState)
		return
	}
	if spec.CommandName == "get_project_state" {
		if h.shadow != nil {
			h.shadow.Initialize(reply)
		}
		return
	}
	h.applyKernelReplyShadowDelta(spec, reply)
}

func vspCanonicalCommandForSpec(commandName string) string {
	switch commandName {
	case "ping":
		return "kernel.ping"
	case "play":
		return "transport.play"
	case "stop":
		return "transport.stop"
	case "list_tracks":
		return "project.tracks.list"
	case "project.import_audio_files":
		return "project.import_audio_files"
	case "add_track":
		return "track.create"
	case "delete_track":
		return "track.delete"
	case "move_clip":
		return "clip.move"
	case "resize_clip":
		return "clip.resize"
	case "split_clip":
		return "clip.split"
	case "remove_clips":
		return "clip.remove"
	case "clip.fade.set":
		return "clip.fade.set"
	case "clip.fade.read":
		return "clip.fade.read"
	case "clip.gain.set":
		return "clip.gain.set"
	case "clip.gain.read":
		return "clip.gain.read"
	case "track.group.list":
		return "track.group.list"
	case "track.group.create":
		return "track.group.create"
	case "track.group.update":
		return "track.group.update"
	case "track.group.set_members":
		return "track.group.set_members"
	case "track.group.delete":
		return "track.group.delete"
	case "track.group.apply_control":
		return "track.group.apply_control"
	case "undo":
		return "project.undo"
	case "redo":
		return "project.redo"
	case "get_plugin_parameters":
		return "plugin.parameters.get"
	case "start_render":
		return "render.start"
	case "cancel_render":
		return "render.cancel"
	default:
		return ""
	}
}

func vspArgsFromCommand(cmd map[string]any) map[string]any {
	out := tools.CloneCommand(cmd)
	delete(out, "cmd")
	return out
}

func vspExecutionSummary(commandResult *kernel.VSPCommandResult, observe, delta, resync *kernel.VSPStateResult, warnings []string) map[string]any {
	out := map[string]any{
		"transport": "vsp",
		"version":   "1.0",
	}
	if commandResult != nil {
		out["command_ack"] = vspCommandSummary(commandResult)
	}
	if observe != nil {
		out["project_observe"] = vspStateSummary("vsp.state.snapshot", observe, true)
	}
	if delta != nil {
		out["state_delta"] = vspStateSummary("vsp.state.delta", delta, false)
	}
	if resync != nil {
		out["state_resync"] = vspStateSummary("vsp.state.resync", resync, true)
	}
	if len(warnings) > 0 {
		out["warnings"] = append([]string(nil), warnings...)
	}
	return out
}

func vspCommandSummary(result *kernel.VSPCommandResult) map[string]any {
	if result == nil {
		return nil
	}
	out := map[string]any{
		"source":         "vsp.command.response",
		"command":        result.Command,
		"legacy_command": result.LegacyCommand,
		"transaction_id": result.TransactionID,
		"revision":       result.Revision,
		"resync_hint":    result.ResyncHint,
	}
	if ack := mapAnyFromAny(result.Response["ack"]); len(ack) > 0 {
		out["ack"] = ack
		out["stage"] = firstString(ack, "stage")
	}
	if errorObj := mapAnyFromAny(result.Response["error"]); len(errorObj) > 0 {
		out["error"] = errorObj
	}
	return out
}

func vspStateSummary(source string, result *kernel.VSPStateResult, includeSnapshot bool) map[string]any {
	if result == nil {
		return nil
	}
	out := map[string]any{
		"source":         source,
		"type":           firstString(result.Response, "type"),
		"revision":       result.Revision,
		"base_revision":  result.BaseRevision,
		"project_epoch":  result.ProjectEpoch,
		"snapshot_hash":  result.SnapshotHash,
		"scope":          result.Scope,
		"resync":         result.Resync,
		"resync_hint":    result.ResyncHint,
		"ops_count":      len(result.Ops),
		"changed_tracks": result.ChangedTracks,
		"changed_clips":  result.ChangedClips,
	}
	if ack := mapAnyFromAny(result.Response["ack"]); len(ack) > 0 {
		out["ack"] = ack
		out["stage"] = firstString(ack, "stage")
	}
	if len(result.Ops) > 0 && len(result.Ops) <= 64 {
		out["ops"] = result.Ops
	}
	if includeSnapshot {
		if project := mapAnyFromAny(result.Payload["project"]); len(project) > 0 {
			out["project"] = project
		}
		if tracks, ok := result.Payload["tracks"]; ok {
			out["tracks"] = tracks
		}
	}
	return out
}

func attachVSPExecutionResult(result map[string]any, vsp map[string]any) {
	if len(vsp) == 0 || result == nil {
		return
	}
	result["vsp"] = vsp
	if commandAck := mapAnyFromAny(vsp["command_ack"]); len(commandAck) > 0 {
		result["command_ack"] = commandAck
	}
	if delta := mapAnyFromAny(vsp["state_delta"]); len(delta) > 0 {
		result["state_delta"] = delta
	}
	if resync := mapAnyFromAny(vsp["state_resync"]); len(resync) > 0 {
		result["state_resync"] = resync
	}
	if observe := mapAnyFromAny(vsp["project_observe"]); len(observe) > 0 {
		result["project_observe"] = observe
	}
}

func broadMixObserveFirstWriteGuard(requestContext map[string]any, spec tools.CommandSpec, cmd map[string]any) error {
	if !broadMixWriteCommand(spec, cmd) {
		return nil
	}
	userText := broadMixGuardUserText(requestContext)
	knownPluginNames := toolpolicy.CollectPluginNames(requestContext, cmd)
	if !broadMixNaturalRequest(userText) || toolpolicy.ExplicitPluginRequest(userText, knownPluginNames) {
		return nil
	}
	return fmt.Errorf("ordinary acoustic mixing requests must run mix.request_observation and receive a concrete observation before loading plugins, learning plugin profiles, changing volume, applying controls, or writing parameters")
}

func broadMixWriteCommand(spec tools.CommandSpec, cmd map[string]any) bool {
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(spec.CommandName, tools.CommandName(cmd), fmt.Sprint(cmd["tool"]))))
	switch name {
	case "rack_add_node", "rack.add_node", "plugin.load_to_rack", "instantiate_plugin", "plugin.instantiate",
		"plugin_grabber_load_and_get_params", "plugin_grabber.load_and_get_params",
		"plugin_grabber_learn_project_profile", "plugin_grabber.learn_project_profile", "plugin.learn_project_profile",
		"plugin_grabber_apply_control", "plugin_grabber.apply_control", "plugin_grabber.apply",
		"set_plugin_param", "plugin.set_parameter", "plugin_set_parameter",
		"set_volume", "track.volume",
		"track.group.apply_control", "track_group.apply_control", "track_group_apply_control",
		"set_pan", "track.pan",
		"control_add_macro", "control.add_macro", "rack.add_macro", "control_add_binding", "control.add_binding":
		return true
	default:
		return false
	}
}

func broadMixGuardUserText(requestContext map[string]any) string {
	for _, key := range []string{"user_message", "user_text", "goal_summary", "goal", "intent", "user_intent", "original_user_message"} {
		if text := strings.TrimSpace(fmt.Sprint(requestContext[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func broadMixNaturalRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return broadMixTextHasAny(text,
		"\u6df7\u97f3", "\u6df7\u4e00\u4e0b", "\u5e2e\u6211\u6df7", "\u7f29\u6df7", "\u58f0\u97f3\u5904\u7406", "\u8c03\u4e00\u4e0b", "\u5904\u7406\u4e00\u4e0b",
		"\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal",
		"\u9760\u524d", "\u5f80\u524d", "\u63d0\u5347\u54cd\u5ea6", "\u54cd\u5ea6", "\u66f4\u4eae", "\u660e\u4eae", "\u6d51\u6d4a", "\u523a\u8033",
		"\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u7a7a\u95f4\u611f", "\u52a0\u4e00\u70b9\u7a7a\u95f4", "\u52a8\u6001", "\u538b\u7f29",
		"mix", "mixing", "loudness", "louder", "forward", "mud", "muddy", "harsh", "bright", "space", "reverb", "dynamic",
	)
}

func broadMixTextHasAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func (h *Harness) PreviewResolvedCommand(ctx context.Context, command map[string]any, requestContext map[string]any) (string, error) {
	cmd, spec, err := h.resolveCommand(InvokeRequest{Command: command})
	if err != nil {
		return "", err
	}
	if err := h.resolveImplicitTargets(ctx, spec, cmd, requestContext); err != nil {
		return "", err
	}
	return PreviewCommand(spec, cmd), nil
}

func (h *Harness) resolveCommand(req InvokeRequest) (map[string]any, tools.CommandSpec, error) {
	if h == nil || h.catalog == nil {
		return nil, tools.CommandSpec{}, fmt.Errorf("command catalog is unavailable")
	}
	if len(req.Command) > 0 {
		cmd := tools.CloneCommand(req.Command)
		flattenCommandParams(cmd)
		name := tools.CommandName(cmd)
		if name == "" {
			if toolName := firstString(cmd, "tool"); toolName != "" {
				return h.resolveToolCall(toolName, commandArgs(cmd))
			}
			return nil, tools.CommandSpec{}, fmt.Errorf("command is missing cmd/action/command or tool")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
			if spec, toolOK := h.catalog.LookupTool(name); toolOK {
				cmd["cmd"] = spec.CommandName
				return cmd, spec, nil
			}
			return nil, tools.CommandSpec{}, fmt.Errorf("unknown or unregistered DAW command: %s", name)
		}
		return cmd, spec, nil
	}

	toolName := strings.TrimSpace(req.Tool)
	if toolName == "" {
		return nil, tools.CommandSpec{}, fmt.Errorf("tool or command is required")
	}
	if toolName == "daw.invoke" {
		cmd := tools.CloneCommand(req.Args)
		flattenCommandParams(cmd)
		name := tools.CommandName(cmd)
		if name == "" {
			return nil, tools.CommandSpec{}, fmt.Errorf("daw.invoke args must include cmd/action/command")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
			if spec, toolOK := h.catalog.LookupTool(name); toolOK {
				cmd["cmd"] = spec.CommandName
				return cmd, spec, nil
			}
			return nil, tools.CommandSpec{}, fmt.Errorf("unknown or unregistered DAW command: %s", name)
		}
		return cmd, spec, nil
	}

	spec, ok := h.catalog.LookupTool(toolName)
	if !ok {
		if commandSpec, commandOK := h.catalog.LookupCommand(toolName); commandOK {
			cmd := tools.BuildCommand(commandSpec.CommandName, req.Args)
			flattenCommandParams(cmd)
			return cmd, commandSpec, nil
		}
		return nil, tools.CommandSpec{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	cmd := tools.BuildCommand(spec.CommandName, req.Args)
	flattenCommandParams(cmd)
	return cmd, spec, nil
}

func (h *Harness) resolveToolCall(toolName string, args map[string]any) (map[string]any, tools.CommandSpec, error) {
	toolName = strings.TrimSpace(toolName)
	spec, ok := h.catalog.LookupTool(toolName)
	if !ok {
		if commandSpec, commandOK := h.catalog.LookupCommand(toolName); commandOK {
			cmd := tools.BuildCommand(commandSpec.CommandName, args)
			flattenCommandParams(cmd)
			return cmd, commandSpec, nil
		}
		return nil, tools.CommandSpec{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	cmd := tools.BuildCommand(spec.CommandName, args)
	flattenCommandParams(cmd)
	return cmd, spec, nil
}

func commandArgs(cmd map[string]any) map[string]any {
	if args, ok := cmd["args"].(map[string]any); ok {
		return tools.CloneCommand(args)
	}
	out := tools.CloneCommand(cmd)
	delete(out, "tool")
	delete(out, "args")
	return out
}

func (h *Harness) invokeLocal(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (map[string]any, bool) {
	if spec.CommandName == "project.audio_analysis_status" && boolValueDefault(cmd["ensure_ready"], false) {
		result, err := h.ensureProjectAudioAnalysis(ctx, cmd)
		return resultWithErr(result, err), true
	}
	switch spec.CommandName {
	case "select_track":
		return map[string]any{
			"status":     "ok",
			"ui_action":  "focus_track",
			"track_id":   firstString(cmd, "track_id", "target_track_id"),
			"track_name": firstString(cmd, "track_name", "name"),
		}, true
	case "select_clip":
		return map[string]any{
			"status":        "ok",
			"ui_action":     "select_clip",
			"track_id":      firstString(cmd, "track_id"),
			"track_name":    firstString(cmd, "track_name"),
			"clip_id":       firstString(cmd, "clip_id"),
			"clip_name":     firstString(cmd, "clip_name", "name"),
			"start_seconds": cmd["start_seconds"],
			"start_beats":   cmd["start_beats"],
		}, true
	case "select_plugin":
		return map[string]any{
			"status":      "ok",
			"ui_action":   "focus_plugin",
			"track_id":    firstString(cmd, "track_id", "selected_plugin_track_id"),
			"plugin_id":   firstString(cmd, "plugin_id", "plugin_item_id", "node_id"),
			"plugin_name": firstString(cmd, "plugin_name", "name"),
		}, true
	case "goal_status":
		goal := h.runtime.Status(firstString(cmd, "goal_id"))
		return map[string]any{"goal": goal}, true
	case "goal_cancel":
		goal := h.runtime.Cancel(firstString(cmd, "goal_id"), firstString(cmd, "reason", "message"))
		return map[string]any{"goal": goal}, true
	case "goal_tick":
		goal := h.runtime.Tick(firstString(cmd, "goal_id"), firstString(cmd, "checkpoint"))
		return map[string]any{"goal": goal}, true
	case "project_snapshot_export":
		return h.projectSnapshotExportCompat(cmd), true
	case "mix_observe", "mix_request_observation":
		result, err := h.requestMixObservation(ctx, cmd)
		return resultWithErr(result, err), true
	case "mix_read":
		result, err := h.readMixObservation(cmd)
		return resultWithErr(result, err), true
	case "mix_derive":
		result, err := h.deriveMixObservation(cmd)
		return resultWithErr(result, err), true
	case "mix_propose_tick":
		result, err := h.proposeMixTick(ctx, cmd)
		return resultWithErr(result, err), true
	case "mix_apply_tick":
		result, err := h.applyMixTick(ctx, cmd)
		return resultWithErr(result, err), true
	case "mix_apply_static_balance_batch":
		result, err := h.applyStaticBalanceBatch(ctx, cmd)
		return resultWithErr(result, err), true
	case "mix_apply_pan_layout_batch":
		result, err := h.applyPanLayoutBatch(ctx, cmd)
		return resultWithErr(result, err), true
	case "mix_rollback_tick":
		result, err := h.rollbackMixTick(ctx, cmd)
		return resultWithErr(result, err), true
	case "clip.strip_silence.suggest":
		result, err := h.suggestStripSilence(ctx, cmd, requestContext)
		return resultWithErr(result, err), true
	case "clip.strip_silence.apply_batch":
		result, err := h.applyStripSilenceBatch(ctx, cmd)
		return resultWithErr(result, err), true
	case "clip.gain.set_batch":
		result, err := h.applyClipGainBatch(ctx, cmd)
		return resultWithErr(result, err), true
	case "control_add_macro", "plugin_map_macro_to_params":
		return h.upsertMacroControlResult(cmd), true
	case "control_rename_macro", "control.rename_macro":
		return h.renameMacroControlResult(cmd), true
	case "control_add_binding", "control.add_binding":
		return h.addMacroBindingResult(cmd), true
	case "control_set_macro_values", "control.set_macro_values":
		return h.setMacroValuesResult(ctx, cmd), true
	case "workspace_glob":
		result, err := workspace.Glob(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_grep":
		result, err := workspace.Grep(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_read_file":
		result, err := workspace.ReadFile(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_file_info":
		result, err := workspace.FileInfo(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "logs_read":
		logCmd := tools.CloneCommand(cmd)
		if isEmptyValue(logCmd["root_id"]) {
			logCmd["root_id"] = "logs"
		}
		if isEmptyValue(logCmd["path"]) {
			logCmd["path"] = "agent_last.log"
		}
		result, err := workspace.ReadFile(h.workspaceContext(cmd), logCmd)
		return resultWithErr(result, err), true
	case "logs_search":
		logCmd := tools.CloneCommand(cmd)
		if isEmptyValue(logCmd["root_id"]) {
			logCmd["root_id"] = "logs"
		}
		if isEmptyValue(logCmd["pattern"]) {
			logCmd["pattern"] = "*.log"
		}
		result, err := workspace.Grep(h.workspaceContext(cmd), logCmd)
		return resultWithErr(result, err), true
	case "workspace_edit_preview":
		result, err := workspace.EditPreview(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_apply_edit":
		result, err := workspace.ApplyEdit(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_write_file":
		result, err := workspace.WriteFile(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_blob_info":
		result, err := workspace.BlobInfo(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "workspace_blob_copy":
		result, err := workspace.BlobCopy(h.workspaceContext(cmd), cmd)
		return resultWithErr(result, err), true
	case "web_search":
		result, err := webtools.Search(ctx, cmd)
		return resultWithErr(result, err), true
	case "web_fetch":
		result, err := webtools.Fetch(ctx, cmd)
		return resultWithErr(result, err), true
	case "artifact_list":
		result, err := artifacts.ListCommand(h.artifactStore(), h.enrichArtifactScopeArgs(ctx, cmd))
		return resultWithErr(result, err), true
	case "artifact_read":
		result, err := artifacts.ReadCommand(h.artifactStore(), cmd)
		return resultWithErr(result, err), true
	case "artifact_extract":
		result, err := artifacts.ExtractCommand(h.artifactStore(), h.enrichArtifactScopeArgs(ctx, cmd))
		return resultWithErr(result, err), true
	case "media_register_assets":
		result, err := resourceintake.RegisterAssets(h.artifactStore(), h.enrichArtifactScopeArgs(ctx, cmd))
		return resultWithErr(result, err), true
	case "media_index_authorized_folder":
		result, err := resourceintake.IndexAuthorizedFolder(h.artifactStore(), h.enrichArtifactScopeArgs(ctx, cmd))
		return resultWithErr(result, err), true
	case "browser_fetch":
		args := h.enrichArtifactScopeArgs(ctx, cmd)
		args["include_body"] = true
		result, err := webtools.Fetch(ctx, args)
		if err == nil {
			result, err = browsercapture.CaptureFetchResult(h.artifactStore(), args, result)
		}
		return resultWithErr(result, err), true
	case "browser_search":
		args := h.enrichArtifactScopeArgs(ctx, cmd)
		result, err := webtools.Search(ctx, args)
		if err == nil {
			result, err = browsercapture.CaptureSearchResult(h.artifactStore(), args, result)
		}
		return resultWithErr(result, err), true
	case "shell_run":
		result, err := shelltools.Run(ctx, cmd)
		return resultWithErr(result, err), true
	case "plugin_semantic_build_index":
		result, err := h.buildPluginSemanticIndex(cmd)
		return resultWithErr(result, err), true
	case "plugin_list_available":
		result, err := h.listAvailablePlugins(context.Background(), cmd)
		return resultWithErr(result, err), true
	case "plugin_search":
		result, err := h.searchAvailablePlugins(context.Background(), cmd)
		return resultWithErr(result, err), true
	case "plugin_semantic_search":
		result, err := h.searchPluginSemanticIndex(cmd)
		return resultWithErr(result, err), true
	case "plugin_semantic_get":
		result, err := h.getPluginSemanticEntry(cmd)
		return resultWithErr(result, err), true
	case "agent_rollback_action":
		result, err := rollback.Action(context.Background(), h, cmd)
		return resultWithErr(result, err), true
	case "version_status":
		result, err := history.Status(h.historyArgs(cmd))
		return resultWithErr(result, err), true
	case "version_checkpoint":
		args := h.enrichHistoryCheckpointArgs(cmd)
		result, err := history.Checkpoint(args)
		return resultWithErr(result, err), true
	case "version_list":
		result, err := history.List(h.historyArgs(cmd))
		return resultWithErr(result, err), true
	case "version_show":
		result, err := history.Show(h.historyArgs(cmd))
		return resultWithErr(result, err), true
	case "version_diff":
		result, err := history.Diff(h.enrichHistorySnapshotArgs(cmd))
		return resultWithErr(result, err), true
	case "version_restore_preview":
		result, err := history.RestorePreview(h.enrichHistorySnapshotArgs(cmd))
		return resultWithErr(result, err), true
	case "version_restore":
		result, err := history.Restore(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_restore", result)
		return resultWithErr(result, err), true
	case "version_branch_create":
		result, err := history.BranchCreate(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_branch_create", result)
		return resultWithErr(result, err), true
	case "version_node_checkout":
		result, err := history.NodeCheckout(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_node_checkout", result)
		return resultWithErr(result, err), true
	case "version_node_delete":
		result, err := history.NodeDelete(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_node_delete", result)
		return resultWithErr(result, err), true
	case "version_worktree_create":
		args := h.worktreeCreateArgs(ctx, cmd)
		result, err := history.WorktreeCreate(args)
		if result != nil {
			if id := firstString(args, "source_checkpoint_id"); id != "" {
				result["source_checkpoint_id"] = id
			}
			if warning := firstString(args, "source_checkpoint_warning"); warning != "" {
				result["warnings"] = appendStringAny(result["warnings"], warning)
			}
		}
		return resultWithErr(result, err), true
	case "version_worktree_checkout":
		args := h.worktreeCheckoutArgs(ctx, cmd)
		result, err := history.WorktreeCheckout(args)
		if result != nil {
			if id := firstString(args, "switch_autosave_commit_id"); id != "" {
				result["switch_autosave_commit_id"] = id
			}
			if warning := firstString(args, "switch_autosave_warning"); warning != "" {
				result["warnings"] = appendStringAny(result["warnings"], warning)
			}
		}
		result = h.withProjectReload(ctx, "version_worktree_checkout", result)
		return resultWithErr(result, err), true
	case "version_worktree_list":
		result, err := history.WorktreeList(h.historyArgs(cmd))
		return resultWithErr(result, err), true
	case "version_project_new":
		result, err := h.applyExternalProjectNew(ctx, cmd)
		return resultWithErr(result, err), true
	case "version_project_opened":
		result, err := h.applyExternalProjectOpened(ctx, cmd)
		return resultWithErr(result, err), true
	case "version_project_save_prepare":
		projectPath := firstString(cmd, "project_path", "current_project_path")
		projectUUID := firstString(cmd, "project_uuid", "project_id")
		if projectPath == "" || projectUUID == "" {
			projectPath, projectUUID = h.CurrentProjectIdentity(ctx)
		}
		result, err := history.PrepareWorkingSessionSave(projectPath, projectUUID, firstString(cmd, "save_kind"))
		return resultWithErr(result, err), true
	case "version_project_saved":
		result, err := h.applyExternalProjectSaved(ctx, cmd)
		return resultWithErr(result, err), true
	case "version_checkout":
		result, err := history.Checkout(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_checkout", result)
		return resultWithErr(result, err), true
	default:
		return nil, false
	}
}

func (h *Harness) applyExternalProjectNew(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	result, err := history.ProjectNew(cmd)
	if err != nil {
		return result, err
	}
	projectUUID := firstString(cmd, "project_uuid", "project_id")
	projectPath := firstString(result, "project_path", "draft_project_path")
	if projectUUID != "" {
		projectPath = history.BindProjectIdentity(projectPath, projectUUID)
		if _, err := history.EnsureWorkingSession(projectPath, projectUUID); err != nil {
			return result, err
		}
		result, err = history.Status(map[string]any{"project_path": projectPath})
		if err != nil {
			return result, err
		}
		result["status"] = "ok"
	}
	result["refresh"] = h.refreshShadowWithStatus(ctx, "version_project_new")
	return result, nil
}

func (h *Harness) applyExternalProjectOpened(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	projectPath := firstString(cmd, "project_path", "current_project_path")
	projectUUID := firstString(cmd, "project_uuid", "project_id")
	if projectPath == "" || projectUUID == "" {
		return nil, errors.New("external project open notification omitted project path or UUID")
	}
	parentUUID := firstString(cmd, "parent_project_uuid")
	generationID := firstString(cmd, "agent_history_generation", "history_generation")
	recovered, err := history.RecoverPreparedSaveOnOpen(
		projectPath, projectUUID, firstString(cmd, "source_project_path"), parentUUID,
		generationID, firstString(cmd, "history_prepare_id"),
	)
	if err != nil {
		return nil, err
	}
	if parentUUID != "" && parentUUID != projectUUID && !boolValueDefault(recovered["recovered"], false) {
		if _, err := history.RecoverMissingProjectFork("", parentUUID, projectPath, projectUUID); err != nil {
			return nil, err
		}
	}
	history.BindProjectIdentity(projectPath, projectUUID)
	if _, err := history.OpenWorkingSessionAtGeneration(projectPath, projectUUID, generationID); err != nil {
		return nil, err
	}
	result, err := history.Status(map[string]any{"project_path": projectPath})
	if err != nil {
		return nil, err
	}
	result["status"] = "ok"
	if boolValueDefault(recovered["recovered"], false) {
		result["prepared_save_recovery"] = recovered
	}
	result["refresh"] = h.refreshShadowWithStatus(ctx, "version_project_opened")
	return result, nil
}

func (h *Harness) applyExternalProjectSaved(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	saveKind := strings.ToLower(firstString(cmd, "save_kind", "project_lifecycle"))
	targetPath := firstString(cmd, "project_path", "current_project_path")
	targetUUID := firstString(cmd, "project_uuid", "project_id")
	if targetPath == "" || targetUUID == "" {
		return nil, errors.New("external project save notification omitted target path or UUID")
	}
	var result map[string]any
	var err error
	prepareID := firstString(cmd, "history_prepare_id", "prepare_id")
	generationID := firstString(cmd, "agent_history_generation", "history_generation", "generation_id")
	if prepareID == "" || generationID == "" {
		return nil, errors.New("external project save notification omitted prepared Agent history identity")
	}
	if saveKind == "save_as" {
		sourcePath := firstString(cmd, "source_project_path")
		sourceUUID := firstString(cmd, "source_project_uuid")
		if sourcePath == "" || sourceUUID == "" {
			return nil, errors.New("external Save As notification omitted source project identity")
		}
		result, err = history.CommitPreparedWorkingSession(sourcePath, sourceUUID, targetPath, targetUUID, prepareID, generationID, "save_as")
		if err == nil {
			if derivedDir, derivedErr := projectworkspace.ForkDerived(sourcePath, sourceUUID, targetPath, targetUUID); derivedErr != nil {
				result["derived_fork_warning"] = derivedErr.Error()
			} else if derivedDir != "" {
				result["derived_dir"] = derivedDir
				result["forked_derived"] = true
			}
		}
	} else {
		result, err = history.CommitPreparedWorkingSession(targetPath, targetUUID, targetPath, targetUUID, prepareID, generationID, "save")
	}
	if err != nil {
		return result, err
	}
	refresh := h.refreshShadowWithStatus(ctx, "version_project_saved_"+firstNonEmpty(saveKind, "save"))
	result["refresh"] = refresh
	if warning := firstString(refresh, "warning"); warning != "" {
		result["warnings"] = appendStringAny(result["warnings"], warning)
	}
	return h.retagArtifactsForSavedProject(ctx, cmd, result), nil
}

func (h *Harness) upsertMacroControlResult(cmd map[string]any) map[string]any {
	if boolFromAnyDefault(cmd["_use_existing_macro"], false) {
		existing := macrocontrols.NormalizeControl(mapAnyFromAny(cmd["_existing_macro"]))
		macroID := firstString(existing, "macro_id", "id", "control_id")
		if macroID != "" {
			return map[string]any{
				"status":                  "ok",
				"ui_action":               "rack_macro_selected",
				"kind":                    "rack_macro_selected",
				"macro_id":                macroID,
				"macro":                   existing,
				"selected_existing_macro": true,
			}
		}
	}
	trackID := firstString(cmd, "track_id")
	if trackID == "" {
		return map[string]any{"status": "error", "error": "track_id is required"}
	}
	macroID := firstString(cmd, "macro_id", "id")
	if macroID == "" {
		macroID = "macro_" + randomID()
	}
	macro := macrocontrols.NormalizeControl(mapAnyFromAny(cmd["macro"]))
	macro["macro_id"] = macroID
	macro["id"] = macroID
	macro["track_id"] = trackID
	if pluginID := firstString(cmd, "plugin_id"); pluginID != "" {
		macro["plugin_id"] = pluginID
	}
	if name := firstString(cmd, "name", "label", "title", "macro_name"); name != "" {
		macro["name"] = name
	}
	if controlType := firstString(cmd, "control_type", "type"); controlType != "" {
		macro["control_type"] = controlType
	}
	if value, ok := numberValueFromMap(cmd, "value", "default"); ok {
		macro["value"] = clampFloat(value, 0, 1)
	}
	if bindings := macroBindingRows(cmd["bindings"]); len(bindings) > 0 {
		macro["bindings"] = bindings
	}
	return map[string]any{
		"status":        "ok",
		"ui_action":     "rack_macro_upserted",
		"kind":          "rack_macro_upserted",
		"macro_id":      macroID,
		"macro":         macro,
		"binding_count": len(macroBindingRows(macro["bindings"])),
	}
}

func (h *Harness) renameMacroControlResult(cmd map[string]any) map[string]any {
	macroID := firstString(cmd, "macro_id", "id", "control_id", "source_node_id", "source_macro_id")
	if macroID == "" {
		return map[string]any{"status": "error", "error": "macro_id is required"}
	}
	newName := firstString(cmd, "new_name", "name", "label", "title")
	if newName == "" {
		return map[string]any{"status": "error", "error": "name is required", "macro_id": macroID}
	}
	macro := macrocontrols.NormalizeControl(mapAnyFromAny(cmd["_existing_macro"]))
	if firstString(macro, "macro_id", "id") == "" {
		macro = macrocontrols.NormalizeControl(mapAnyFromAny(cmd["macro"]))
	}
	if firstString(macro, "macro_id", "id") == "" {
		macro = macrocontrols.NormalizeControl(cmd)
	}
	oldName := firstString(macro, "name", "label", "title", "macro_name")
	macro["macro_id"] = macroID
	macro["id"] = macroID
	macro["name"] = newName
	return map[string]any{
		"status":    "ok",
		"ui_action": "rack_macro_renamed",
		"kind":      "rack_macro_renamed",
		"macro_id":  macroID,
		"name":      newName,
		"new_name":  newName,
		"old_name":  oldName,
		"macro":     macro,
	}
}

func (h *Harness) addMacroBindingResult(cmd map[string]any) map[string]any {
	macroID := firstString(cmd, "macro_id", "id", "control_id", "source_node_id", "source_macro_id")
	if macroID == "" {
		return map[string]any{"status": "error", "error": "macro_id is required"}
	}
	binding := mapAnyFromAny(cmd["binding"])
	if len(binding) == 0 {
		binding = tools.CloneCommand(cmd)
		delete(binding, "cmd")
		delete(binding, "command")
		delete(binding, "macro_id")
	}
	if firstString(binding, "track_id") == "" && firstString(cmd, "track_id") != "" {
		binding["track_id"] = firstString(cmd, "track_id")
	}
	if firstString(binding, "track_id") == "" && firstString(cmd, "target_track_id") != "" {
		binding["track_id"] = firstString(cmd, "target_track_id")
	}
	if firstString(binding, "plugin_id") == "" && firstString(cmd, "plugin_id") != "" {
		binding["plugin_id"] = firstString(cmd, "plugin_id")
	}
	if firstString(binding, "plugin_id") == "" {
		if pluginID := firstString(cmd, "target_plugin_id", "plugin_item_id", "target_node_id"); pluginID != "" {
			binding["plugin_id"] = pluginID
		}
	}
	if firstString(binding, "param_id") == "" {
		if paramID := firstString(cmd, "param_id", "target_param_id", "parameter_id", "target_input", "key"); paramID != "" {
			binding["param_id"] = paramID
		}
	}
	if firstString(binding, "param_name") == "" {
		if paramName := firstString(cmd, "param_name", "target_param_name", "parameter_name", "label", "name"); paramName != "" {
			binding["param_name"] = paramName
		}
	}
	if firstString(binding, "param_id") == "" {
		return map[string]any{"status": "error", "error": "param_id is required", "macro_id": macroID}
	}
	return map[string]any{
		"status":    "ok",
		"ui_action": "rack_macro_binding_added",
		"kind":      "rack_macro_binding_added",
		"macro_id":  macroID,
		"binding":   binding,
	}
}

func (h *Harness) setMacroValuesResult(ctx context.Context, cmd map[string]any) map[string]any {
	macro := macrocontrols.NormalizeControl(mapAnyFromAny(cmd["macro"]))
	if firstString(macro, "macro_id", "id") == "" {
		macro = macrocontrols.NormalizeControl(cmd)
	}
	macroID := firstNonEmpty(firstString(cmd, "macro_id", "id"), firstString(macro, "macro_id", "id"))
	if macroID == "" {
		return map[string]any{"status": "error", "error": "macro_id is required"}
	}
	value, ok := numberValueFromMap(cmd, "value")
	if !ok {
		return map[string]any{"status": "error", "error": "value is required", "macro_id": macroID}
	}
	minValue, _ := numberValueFromMap(macro, "min")
	maxValue, ok := numberValueFromMap(macro, "max")
	if !ok || maxValue <= minValue {
		minValue = 0
		maxValue = 1
	}
	value = clampFloat(value, minValue, maxValue)
	macro["macro_id"] = macroID
	macro["id"] = macroID
	macro["value"] = value
	bindings := macroBindingRows(macro["bindings"])
	if len(bindings) == 0 {
		return map[string]any{"status": "error", "error": "macro has no parameter bindings", "macro_id": macroID, "value": value, "macro": macro}
	}
	applied := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		if !boolFromAnyDefault(binding["enabled"], true) {
			continue
		}
		trackID := firstNonEmpty(firstString(binding, "track_id"), firstString(macro, "track_id"))
		pluginID := firstString(binding, "plugin_id")
		paramID := firstString(binding, "param_id")
		control := firstString(binding, "control")
		if control == "track.volume" || paramID == "track.volume" {
			if trackID == "" {
				continue
			}
			targetMin := numberFromAnyWithDefault(binding["target_min"], -60)
			targetMax := numberFromAnyWithDefault(binding["target_max"], 12)
			if targetMin == 0 && targetMax == 1 && minValue < 0 && maxValue > 1 {
				targetMin = minValue
				targetMax = maxValue
			}
			targetValue := clampFloat(value, targetMin, targetMax)
			if h == nil || h.kernel == nil {
				return map[string]any{"status": "error", "error": "kernel client is required for track.volume macro binding", "macro_id": macroID, "value": value, "macro": macro}
			}
			kernelCmd := map[string]any{
				"cmd":      "set_volume",
				"track_id": trackID,
				"db":       targetValue,
			}
			volumeSpec := tools.CommandSpec{CommandName: "set_volume", MutatesProject: true, RefreshAfter: true}
			if h.catalog != nil {
				if spec, ok := h.catalog.LookupCommand("set_volume"); ok {
					volumeSpec = spec
				}
			}
			execution := h.executeKernelCommand(ctx, volumeSpec, kernelCmd)
			reply, err := execution.reply, execution.err
			if err != nil || !kernelReplySucceeded(reply) {
				return map[string]any{
					"status":   "error",
					"error":    firstNonEmpty(firstString(reply, "message"), firstString(reply, "error"), fmt.Sprint(err), "track.volume macro binding failed"),
					"macro_id": macroID,
					"value":    value,
					"macro":    macro,
				}
			}
			appliedRow := map[string]any{
				"track_id":     trackID,
				"control":      "track.volume",
				"param_id":     "track.volume",
				"param_name":   firstNonEmpty(firstString(binding, "param_name"), "\u8f68\u9053\u97f3\u91cf"),
				"target_value": targetValue,
				"unit":         firstString(binding, "unit"),
			}
			if len(execution.vsp) > 0 {
				appliedRow["vsp"] = execution.vsp
			}
			applied = append(applied, appliedRow)
			continue
		}
		if trackID == "" || pluginID == "" || paramID == "" {
			continue
		}
		targetValue := macroBindingTargetValue(binding, value)
		applied = append(applied, map[string]any{
			"track_id":     trackID,
			"plugin_id":    pluginID,
			"param_id":     paramID,
			"param_name":   firstString(binding, "param_name"),
			"target_value": targetValue,
		})
	}
	if len(applied) == 0 {
		return map[string]any{"status": "error", "error": "macro has no enabled parameter bindings", "macro_id": macroID, "value": value, "macro": macro}
	}
	return map[string]any{
		"status":             "ok",
		"ui_action":          "rack_macro_value_changed",
		"kind":               "rack_macro_value_changed",
		"macro_id":           macroID,
		"value":              value,
		"commit":             boolFromAnyDefault(cmd["commit"], true),
		"macro":              macro,
		"applied_count":      len(applied),
		"applied_parameters": applied,
	}
}

func macroBindingRows(raw any) []map[string]any {
	switch rows := raw.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, item := range rows {
			if row := mapAnyFromAny(item); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func macroBindingTargetValue(binding map[string]any, macroValue float64) float64 {
	sourceMin := clampFloat(numberFromAnyWithDefault(binding["source_min"], 0), 0, 1)
	sourceMax := clampFloat(numberFromAnyWithDefault(binding["source_max"], 1), 0, 1)
	t := 0.0
	if absFloat(sourceMax-sourceMin) < 0.0001 {
		if macroValue >= sourceMax {
			t = 1
		}
	} else {
		t = clampFloat((macroValue-sourceMin)/(sourceMax-sourceMin), 0, 1)
	}
	targetMin := numberFromAnyWithDefault(binding["target_min"], 0)
	targetMax := numberFromAnyWithDefault(binding["target_max"], 1)
	return targetMin + (targetMax-targetMin)*t
}

func numberFromAnyWithDefault(value any, fallback float64) float64 {
	if isEmptyValue(value) {
		return fallback
	}
	switch x := value.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case json.Number:
		if n, err := x.Float64(); err == nil {
			return n
		}
	default:
		if n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64); err == nil {
			return n
		}
	}
	return fallback
}

func clampFloat(value, minValue, maxValue float64) float64 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func boolFromAnyDefault(value any, fallback bool) bool {
	if isEmptyValue(value) {
		return fallback
	}
	switch x := value.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "enabled":
			return true
		case "false", "0", "no", "off", "disabled":
			return false
		}
	}
	return fallback
}

func resultWithErr(result map[string]any, err error) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	if err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
	} else if _, ok := result["status"]; !ok {
		result["status"] = "ok"
	}
	return result
}

func (h *Harness) projectSnapshotExportCompat(cmd map[string]any) map[string]any {
	if h != nil && h.kernel != nil && !boolValueDefault(cmd["compat_only"], false) {
		for _, commandName := range []string{"project.snapshot_export", "project_snapshot_export"} {
			reply, _, err := h.kernel.SendCommand(context.Background(), map[string]any{"cmd": commandName})
			if err == nil && !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
				return reply
			}
		}
	}
	state := map[string]any{}
	if h != nil {
		state = h.UserStateSummary(context.Background())
	}
	out := map[string]any{
		"status": "ok",
		"source": "agent_shadow_snapshot_compat",
	}
	if path := projectPathFromState(state); path != "" {
		out["project_path"] = path
	}
	if len(state) > 0 {
		out["project_state"] = state
		if data, err := json.Marshal(state); err == nil {
			out["snapshot_json"] = string(data)
		}
	}
	return out
}

func (h *Harness) workspaceContext(cmd map[string]any) workspace.Context {
	state := map[string]any{}
	if h != nil {
		state = h.UserStateSummary(context.Background())
	}
	return workspace.NewContext(cmd, state)
}

func (h *Harness) artifactStore() artifacts.Store {
	return artifacts.NewStore("")
}

func (h *Harness) requestMixObservation(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	cmd = cloneAnyMap(cmd)
	state := map[string]any{}
	if h != nil {
		state = h.UserStateSummary(ctx)
		if mixObservationScopeNeedsFreshProjectState(cmd) {
			refresh := h.refreshShadowWithStatus(ctx, "mix_request_observation_project_scope_state")
			if boolValueDefault(refresh["shadow_refreshed"], false) {
				state = h.UserStateSummary(ctx)
			}
		}
	}
	explicitSourceIdentityArgs := map[string]any(nil)
	if mixObservationHasExplicitSourceIdentity(cmd) {
		explicitSourceIdentityArgs = cloneAnyMap(cmd)
	}
	target := mixTargetFromCommand(cmd)
	if target.ID == "" {
		target.ID = firstString(cmd, "track_id", "clip_id", "target_id")
	}
	if target.Kind == "" {
		target.Kind = firstNonEmpty(firstString(cmd, "target_kind", "kind"), "selection")
	}
	resolvedContext := resolveMixObservationTargetContext(cmd, state, target)
	if normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope")) == "full_project_with_focus_track" && firstString(resolvedContext, "track_id") == "" {
		if focusResolved := resolveMixObservationFocusHint(cmd, state); len(focusResolved) > 0 {
			resolvedContext = focusResolved
			target.Kind = "track"
			target.ID = firstString(focusResolved, "track_id")
			target.Label = firstString(focusResolved, "track_name")
			target.Source = firstNonEmpty(firstString(focusResolved, "source"), "focus_hint")
			target.Confidence = firstNonEmpty(firstString(focusResolved, "confidence"), "medium")
		}
	}
	if mixObservationResolutionNeedsRefreshForCommand(cmd, resolvedContext) && h != nil {
		refresh := h.refreshShadowWithStatus(ctx, "mix_request_observation_target_resolution")
		if boolValueDefault(refresh["shadow_refreshed"], false) {
			state = h.UserStateSummary(ctx)
			resolvedContext = resolveMixObservationTargetContext(cmd, state, target)
			if normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope")) == "full_project_with_focus_track" && firstString(resolvedContext, "track_id") == "" {
				if focusResolved := resolveMixObservationFocusHint(cmd, state); len(focusResolved) > 0 {
					resolvedContext = focusResolved
					target.Kind = "track"
					target.ID = firstString(focusResolved, "track_id")
					target.Label = firstString(focusResolved, "track_name")
					target.Source = firstNonEmpty(firstString(focusResolved, "source"), "focus_hint")
					target.Confidence = firstNonEmpty(firstString(focusResolved, "confidence"), "medium")
				}
			}
		}
	}
	if len(explicitSourceIdentityArgs) > 0 {
		resolvedContext = mixObservationResolvedWithExplicitSourceIdentity(resolvedContext, explicitSourceIdentityArgs)
	}
	if resolvedTrackID := firstString(resolvedContext, "track_id"); resolvedTrackID != "" {
		cmd["track_id"] = resolvedTrackID
		if strings.EqualFold(target.Kind, "track") || strings.EqualFold(target.Kind, "selection") || target.Kind == "" {
			target.Kind = "track"
			target.ID = resolvedTrackID
		}
	}
	if resolvedClipID := firstString(resolvedContext, "clip_id"); resolvedClipID != "" {
		cmd["clip_id"] = resolvedClipID
		if strings.EqualFold(target.Kind, "clip") {
			target.ID = resolvedClipID
		}
	}
	if resolvedName := firstString(resolvedContext, "track_name"); resolvedName != "" && strings.EqualFold(target.Kind, "track") {
		target.Label = resolvedName
	}
	if path := firstString(resolvedContext, "file_path"); path != "" {
		cmd["file_path"] = path
	}
	if duration := firstPositiveNumber(resolvedContext, "duration_seconds", "length_seconds", "duration"); duration > 0 {
		cmd["duration_seconds"] = duration
	}
	if label := firstString(resolvedContext, "track_name", "clip_name"); target.Label == "" && label != "" {
		target.Label = label
	}
	if source := firstString(resolvedContext, "source"); source != "" {
		cmd["target_resolution_source"] = source
	}
	cmd = canonicalizeMixObservationCommand(cmd, target, resolvedContext)
	target = mixTargetFromCommand(cmd)
	l2ProbeRequest := h.requestMixObservationL2RenderProbe(ctx, cmd, state, target, resolvedContext)
	acousticStatus, acousticStorePath, featureRequest := h.prepareMixObservationAcousticPackage(ctx, cmd, state, target, resolvedContext)
	intent := mom.ResolveIntent(cmd, firstString(cmd, "mom_intent", "intent", "workflow_intent"))
	if gateStatus, gateStorePath, gateRequest, blocked := h.ensureMixObservationReadyGate(ctx, cmd, state, target, resolvedContext, intent, acousticStatus, acousticStorePath); len(blocked) > 0 {
		if len(l2ProbeRequest) > 0 {
			blocked["l2_render_probe_request"] = l2ProbeRequest
		}
		return blocked, nil
	} else {
		if len(gateStatus) > 0 {
			acousticStatus = gateStatus
		}
		if gateStorePath != "" {
			acousticStorePath = gateStorePath
		}
		if len(gateRequest) > 0 {
			featureRequest = gateRequest
		}
	}
	observationArgs := cloneAnyMap(cmd)
	if firstString(observationArgs, "feature_snapshot_path") == "" {
		observationArgs["feature_snapshot_path"] = mixboard.FeatureSnapshotPath(cmd)
	}
	if len(acousticStatus) > 0 {
		observationArgs["acoustic_package_status"] = acousticStatus
		observationArgs["acoustic_package_status_path"] = acousticStorePath
	}
	result, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: firstString(cmd, "mix_session_id", "session_id"),
		Round:        int(numberFromAny(cmd["round"])),
		GoalText:     firstString(cmd, "goal_text", "goal"),
		TargetRef:    target,
		MixObjects:   mixObjectsFromCommand(cmd),
		ListenScope:  mixListenScopeFromCommand(cmd),
		ProjectState: state,
		Args:         observationArgs,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"status":            result.Status,
		"mix_session_id":    result.Observation.MixSessionID,
		"observation_id":    result.Observation.ObservationID,
		"board_path":        result.BoardPath,
		"observation_path":  result.ObservationPath,
		"context_pack_path": result.ContextPackPath,
		"observation":       result.Observation,
		"mom_projection":    result.Observation.MOMProjection,
		"digest":            result.Observation.Digest,
		"catalog":           result.Observation.Catalog,
		"mixboard":          result.Board,
		"context_pack":      result.ContextPack,
	}
	if len(resolvedContext) > 0 {
		out["resolved_target"] = resolvedContext
	}
	if digest := mixObservationAcousticDigest(result.Observation, featureRequest, resolvedContext); len(digest) > 0 {
		out["acoustic_digest"] = digest
	}
	if len(acousticStatus) > 0 {
		out["acoustic_package_status"] = acousticStatus
		out["acoustic_package_status_path"] = acousticStorePath
		if event := acousticPackageStatusTypedEvent(acousticStatus, acousticStorePath); len(event) > 0 {
			out["typed_events"] = []map[string]any{event}
		}
	}
	if len(featureRequest) > 0 {
		out["feature_request"] = featureRequest
	}
	if len(l2ProbeRequest) > 0 {
		out["l2_render_probe_request"] = l2ProbeRequest
	}
	return out, nil
}

func (h *Harness) requestMixObservationL2RenderProbe(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef, resolvedContext map[string]any) map[string]any {
	if h == nil || h.kernel == nil || !mixObservationShouldRequestL2RenderProbe(cmd) {
		return nil
	}
	resolved := cloneAnyMap(resolvedContext)
	if len(resolved) == 0 {
		resolved = resolveMixboardFeatureTarget(cmd, state, target)
	}
	trackID := firstNonEmpty(firstString(resolved, "track_id"), func() string {
		if strings.EqualFold(target.Kind, "track") {
			return target.ID
		}
		return ""
	}())
	clipID := firstString(resolved, "clip_id", "id")
	if trackID == "" {
		return map[string]any{"status": "blocked", "reason": "track_id_required_for_l2_render_probe"}
	}
	requestID := firstNonEmpty(firstString(cmd, "l2_render_probe_request_id"), "mixboard_l2_render_probe_"+safeRequestIDPart(trackID)+"_"+time.Now().UTC().Format("20060102T150405.000000000"))
	packet := newMixboardFeatureRequestPacket(cmd, target)
	packet["request_id"] = requestID
	packet["status"] = "requested"
	packet["lifecycle"] = "l2_render_probe_ab"
	packet["trigger"] = "mix.observe_l2_render_probe"
	packet["resolved_target"] = resolved
	packet["requested_features"] = []any{map[string]any{
		"feature_type": "l2_render_probe",
		"request_id":   requestID,
		"track_id":     trackID,
		"clip_id":      clipID,
		"tap_point":    firstNonEmpty(firstString(cmd, "tap_point"), "track_post_fader"),
		"render_mode":  "offline_probe",
	}}
	kernelCmd := map[string]any{
		"cmd":         "l2_render_probe",
		"request_id":  requestID,
		"track_id":    trackID,
		"tap_point":   firstNonEmpty(firstString(cmd, "tap_point"), "track_post_fader"),
		"render_mode": "offline_probe",
	}
	if clipID != "" {
		kernelCmd["clip_id"] = clipID
	}
	for _, key := range []string{"source_revision", "clip_revision"} {
		if value := firstNonEmpty(firstString(cmd, key), firstString(resolved, key)); value != "" {
			kernelCmd[key] = value
		}
	}
	row, reply, err := h.collectMixObservationL2RenderProbe(ctx, kernelCmd, requestID, trackID, clipID)
	if err != nil {
		packet["status"] = "requested"
		packet["reason"] = err.Error()
	} else if len(row) > 0 {
		row = stampMixboardFeatureRowIdentity(row, packet, resolved)
		writeMixboardReadyL2RenderProbeSnapshot(cmd, packet, row)
		packet["status"] = firstNonEmpty(firstString(row, "status"), "ready")
		packet["render_revision"] = firstString(row, "render_revision")
		packet["evidence_ref"] = firstString(row, "evidence_ref")
	} else {
		packet["status"] = "requested"
		packet["reason"] = "l2_render_probe_ready_not_observed_before_timeout"
	}
	if len(reply) > 0 {
		packet["kernel_reply"] = compactSelectedAny(reply, []string{"status", "cmd", "feature_type", "tap_point", "render_mode", "track_id", "clip_id", "render_revision", "evidence_ref", "message", "error"})
	}
	packet["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	return packet
}

func mixObservationShouldRequestL2RenderProbe(cmd map[string]any) bool {
	if firstString(cmd, "previous_observation", "previous_observation_id") != "" {
		return true
	}
	if boolValueDefault(cmd["observation_only"], false) && firstString(cmd, "track_id") != "" {
		return true
	}
	return strings.EqualFold(firstString(cmd, "mom_intent", "intent"), "action_preflight_observation")
}

func (h *Harness) collectMixObservationL2RenderProbe(ctx context.Context, kernelCmd map[string]any, requestID, trackID, clipID string) (map[string]any, map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, nil, fmt.Errorf("kernel_client_unavailable")
	}
	timeout := mixboardL2RenderProbeCollectWait()
	var reply map[string]any
	var sendErr error
	send := func() error {
		sendCtx, cancel := context.WithTimeout(context.Background(), mixboardFeatureBackgroundSendWait())
		defer cancel()
		reply, _, sendErr = h.kernel.SendCommand(sendCtx, kernelCmd)
		if sendErr != nil {
			return sendErr
		}
		if !kernelReplySucceeded(reply) {
			return fmt.Errorf("%s", firstNonEmpty(firstString(reply, "message", "error"), "l2_render_probe_request_failed"))
		}
		return nil
	}
	row, err := collectL2RenderProbeEvent(ctx, mixboardFeatureSubURL, trackID, clipID, requestID, timeout, send)
	if err != nil {
		return row, reply, err
	}
	return row, reply, nil
}

func (h *Harness) prepareMixObservationAcousticPackage(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef, resolvedContext map[string]any) (map[string]any, string, map[string]any) {
	storePath := acousticpackage.DefaultStorePath(cmd)
	store := acousticpackage.NewStore(storePath)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	identity := acousticpackage.IdentityFromMaps(state, resolvedContext, cmd)
	featureSnapshot := readMixboardFeatureSnapshotForAcousticPackage(mixboard.FeatureSnapshotPath(cmd))
	status := acousticpackage.BuildStatus(identity, featureSnapshot, now, "mix.observe_read_first")
	if stored, ok, err := store.Find(identity); err == nil && ok {
		status = acousticpackage.MergeStatus(stored, status)
	}
	if _, err := store.Upsert(status); err != nil && h != nil && h.logger != nil {
		h.logger.Warn("[acoustic_package] store upsert failed path=%s err=%v", storePath, err)
	}
	var featureRequest map[string]any
	if h != nil && h.kernel != nil && mixObservationProjectBackgroundFillNeeded(cmd, featureSnapshot, status) {
		featureRequest = h.requestMixObservationFeaturesBackground(ctx, cmd, state, target)
	}
	return acousticpackage.ToMap(status), storePath, featureRequest
}

func readMixboardFeatureSnapshotForAcousticPackage(path string) map[string]any {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	featureSnapshot := readMixboardFeatureSnapshotFile(path)
	featureSnapshot = sanitizeMixboardSnapshotForAcousticPackageReadFirst(featureSnapshot)
	latest, _ := featureSnapshot["latest_request"].(map[string]any)
	if len(featureSnapshot) > 0 && len(latest) > 0 {
		normalizeMixboardFeatureSnapshotToLatestRequest(featureSnapshot)
		writeMixboardFeatureSnapshotFile(path, featureSnapshot, latest)
	}
	return featureSnapshot
}

func normalizeMixboardFeatureSnapshotToLatestRequest(snapshot map[string]any) {
	if len(snapshot) == 0 {
		return
	}
	latest, _ := snapshot["latest_request"].(map[string]any)
	if len(latest) == 0 {
		return
	}
	promoteMixboardBridgeRowsForPacket(snapshot, latest)
	normalizeMixboardBridgeRowsForPacket(snapshot, latest)
}

func (h *Harness) ensureMixObservationReadyGate(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef, resolvedContext map[string]any, intent string, acousticStatus map[string]any, acousticStorePath string) (map[string]any, string, map[string]any, map[string]any) {
	required := mixObservationReadyGateRequiredFeatures(intent)
	if len(required) == 0 || !mixObservationReadyGateApplies(cmd, intent) {
		return acousticStatus, acousticStorePath, nil, nil
	}
	missing := mixObservationReadyGateMissingFeatures(acousticStatus, required)
	if len(missing) == 0 {
		return acousticStatus, acousticStorePath, nil, nil
	}
	if mixObservationReadyGateHasBuildingFeatures(acousticStatus, required) {
		return acousticStatus, acousticStorePath, nil, nil
	}
	if h == nil || h.kernel == nil {
		return acousticStatus, acousticStorePath, nil, nil
	}
	featureRequest := h.requestMixObservationFeatures(ctx, cmd, state, target)
	acousticStatus, acousticStorePath, _ = h.prepareMixObservationAcousticPackage(ctx, cmd, state, target, resolvedContext)
	missing = mixObservationReadyGateMissingFeatures(acousticStatus, required)
	if len(missing) == 0 {
		return acousticStatus, acousticStorePath, featureRequest, nil
	}
	blocked := map[string]any{
		"status":                       "blocked",
		"reason":                       "observation_ready_gate_timeout",
		"intent":                       intent,
		"required_layers":              mixObservationReadyGateLayerNames(required),
		"missing_required_features":    missing,
		"blocked_reasons":              []any{"required_dad_layers_not_ready_before_llm_response"},
		"acoustic_package_status":      acousticStatus,
		"acoustic_package_status_path": acousticStorePath,
	}
	if len(featureRequest) > 0 {
		blocked["feature_request"] = featureRequest
	}
	if len(resolvedContext) > 0 {
		blocked["resolved_target"] = resolvedContext
	}
	if event := acousticPackageStatusTypedEvent(acousticStatus, acousticStorePath); len(event) > 0 {
		blocked["typed_events"] = []map[string]any{event}
	}
	return acousticStatus, acousticStorePath, featureRequest, blocked
}

func mixObservationReadyGateApplies(cmd map[string]any, intent string) bool {
	if boolValueDefault(cmd["observation_ready_gate"], false) || boolValueDefault(cmd["ready_gate"], false) {
		return true
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VIT_OBSERVATION_READY_GATE")))
	if env == "1" || env == "true" || env == "yes" {
		return true
	}
	return intent == mom.IntentActionPreflightObservation
}

func mixObservationReadyGateRequiredFeatures(intent string) []string {
	switch intent {
	case mom.IntentABResultObservation, mom.IntentProjectMultitrackObservation:
		return nil
	default:
		return []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"}
	}
}

func mixObservationReadyGateLayerNames(features []string) []any {
	out := make([]any, 0, len(features))
	for _, feature := range features {
		switch feature {
		case "band_energy_summary":
			out = append(out, "timbre_frequency")
		case "stereo_relation_summary":
			out = append(out, "space_stereo")
		case "loudness_summary":
			out = append(out, "loudness_summary")
		default:
			out = append(out, feature)
		}
	}
	return out
}

func mixObservationReadyGateMissingFeatures(status map[string]any, required []string) []string {
	missing := []string{}
	for _, featureName := range required {
		feature := acousticStatusL3Feature(status, featureName)
		if !mixObservationReadyGateFeatureReady(featureName, feature) {
			missing = append(missing, featureName)
		}
	}
	return missing
}

func mixObservationReadyGateHasBuildingFeatures(status map[string]any, required []string) bool {
	for _, featureName := range required {
		feature := acousticStatusL3Feature(status, featureName)
		switch strings.ToLower(firstString(feature, "status")) {
		case acousticpackage.StatusBuilding, "requested", "pending":
			return true
		}
	}
	return false
}

func mixObservationReadyGateFeatureReady(featureName string, feature map[string]any) bool {
	if len(feature) == 0 {
		return false
	}
	switch mom.StatusFromSource(firstString(feature, "status")) {
	case mom.StatusReady:
		return true
	case mom.StatusApprox:
		return featureName == "loudness_summary"
	default:
		ref := mapAnyFromAny(feature["ref"])
		if strings.EqualFold(firstString(ref, "quality_status"), "ready") && firstString(ref, "source_revision", "clip_revision") != "" {
			return true
		}
		return false
	}
}

func acousticStatusL3Feature(status map[string]any, featureName string) map[string]any {
	layers := mapAnyFromAny(status["package_layers"])
	l3 := mapAnyFromAny(layers["l3_deep"])
	features := mapAnyFromAny(l3["features"])
	return mapAnyFromAny(features[featureName])
}

func sanitizeMixboardSnapshotForAcousticPackageReadFirst(snapshot map[string]any) map[string]any {
	if len(snapshot) == 0 {
		return snapshot
	}
	out := cloneAnyMap(snapshot)
	for _, key := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		row := mapAnyFromAny(out[key])
		if len(row) == 0 {
			continue
		}
		switch strings.ToLower(firstString(row, "status")) {
		case "ready", "partial":
		default:
			continue
		}
		if firstString(row, "request_id") != "" || firstString(row, "source_revision") != "" || firstString(row, "source_hash") != "" {
			continue
		}
		out[key] = map[string]any{
			"status":       "missing",
			"feature_type": firstNonEmpty(firstString(row, "feature_type"), key),
			"track_id":     firstString(row, "track_id"),
			"clip_id":      firstString(row, "clip_id"),
			"reason":       "stale_feature_snapshot_without_request_or_source_revision",
			"updated_at":   time.Now().UTC().Format(time.RFC3339Nano),
		}
	}
	return out
}

func (h *Harness) IngestKernelTelemetry(event map[string]any) {
	if h == nil || len(event) == 0 {
		return
	}
	command := firstString(event, "command", "cmd")
	featureType := firstString(event, "feature_type")
	switch {
	case strings.EqualFold(command, "audio_feature_data_ready") && strings.EqualFold(featureType, "waveform_envelope"):
		h.ingestKernelWaveformTelemetry(event)
	case strings.EqualFold(command, "tile_ready") && strings.EqualFold(firstNonEmpty(featureType, "spectral_field"), "spectral_field"):
		h.ingestKernelSpectralTelemetry(event)
	case strings.EqualFold(command, "audio_feature_data_ready") && isL3AcousticSummaryFeature(featureType):
		h.ingestKernelL3AcousticTelemetry(event)
	}
}

func (h *Harness) ingestKernelWaveformTelemetry(event map[string]any) {
	trackID := firstString(event, "track_id", "source_track_id")
	clipID := firstString(event, "clip_id")
	if trackID == "" && clipID == "" {
		return
	}
	key := kernelFeatureMaterializerKey(trackID, clipID, firstString(event, "file_path"))
	h.featureMu.Lock()
	collector := h.waveforms[key]
	if collector == nil {
		collector = &waveformFeatureCollector{TrackID: trackID, ClipID: clipID}
		h.waveforms[key] = collector
	}
	collector.AddEvent(event)
	row := collector.SnapshotRow(kernelFeatureMaterializerRequestID(event))
	h.featureMu.Unlock()
	if len(row) == 0 {
		return
	}
	target := kernelFeatureMaterializerTarget(event, trackID, clipID)
	packet := kernelFeatureMaterializerPacket(event, target)
	row["source"] = "kernel_prepared_telemetry"
	writeMixboardReadyProjectTrackWaveformSnapshot(nil, packet, row)
}

func (h *Harness) ingestKernelSpectralTelemetry(event map[string]any) {
	trackID := firstString(event, "track_id", "source_track_id")
	clipID := firstString(event, "clip_id")
	if trackID == "" && clipID == "" {
		return
	}
	key := kernelFeatureMaterializerKey(trackID, clipID, firstString(event, "file_path"))
	h.featureMu.Lock()
	collector := h.spectrals[key]
	if collector == nil {
		collector = &spectralFeatureCollector{TrackID: trackID, ClipID: clipID, FeatureType: "spectral_field", LastTileIndex: -1}
		h.spectrals[key] = collector
	}
	collector.AddEvent(event)
	requestID := kernelFeatureMaterializerRequestID(event)
	row := collector.SnapshotRow(requestID)
	h.featureMu.Unlock()
	if len(row) == 0 {
		return
	}
	target := kernelFeatureMaterializerTarget(event, trackID, clipID)
	packet := kernelFeatureMaterializerPacket(event, target)
	row["source"] = firstNonEmpty(firstString(row, "source"), "kernel_tile_ready_direct_collector")
	row["materialized_by"] = "kernel_prepared_telemetry"
	writeMixboardReadySpectralSnapshot(nil, packet, row)
}

func isL3AcousticSummaryFeature(featureType string) bool {
	switch strings.ToLower(strings.TrimSpace(featureType)) {
	case "band_energy_summary", "stereo_relation_summary", "loudness_summary":
		return true
	default:
		return false
	}
}

func (h *Harness) ingestKernelL3AcousticTelemetry(event map[string]any) {
	trackID := firstString(event, "track_id", "source_track_id")
	clipID := firstString(event, "clip_id")
	if trackID == "" && clipID == "" {
		return
	}
	row := cloneAnyMap(event)
	if len(row) == 0 {
		return
	}
	featureType := firstString(row, "feature_type")
	row["status"] = firstNonEmpty(firstString(row, "status"), firstString(row, "quality_status"), "suspect")
	row["source"] = firstNonEmpty(firstString(row, "source"), "kernel_l3_offline_analyzer")
	row["materialized_by"] = "kernel_prepared_l3_telemetry"
	target := kernelFeatureMaterializerTarget(event, trackID, clipID)
	packet := kernelFeatureMaterializerPacket(event, target)
	if requestID := firstString(event, "request_id"); requestID != "" {
		packet["request_id"] = requestID
	}
	writeMixboardReadyL3SummarySnapshot(nil, packet, row)
	if h != nil && h.logger != nil {
		h.logger.Info("[mixboard] ingested L3 summary feature_type=%s track_id=%s clip_id=%s status=%s", featureType, trackID, clipID, firstString(row, "status"))
	}
}

func kernelFeatureMaterializerKey(trackID, clipID, filePath string) string {
	return strings.Join([]string{strings.TrimSpace(trackID), strings.TrimSpace(clipID), strings.ToLower(filepath.Clean(strings.TrimSpace(filePath)))}, "|")
}

func kernelFeatureMaterializerTarget(event map[string]any, trackID, clipID string) map[string]any {
	target := map[string]any{
		"track_id": trackID,
		"clip_id":  clipID,
	}
	if path := firstString(event, "file_path", "source_path"); path != "" {
		target["file_path"] = path
		target["source_path"] = path
	}
	if duration := firstPositiveNumber(event, "total_duration", "duration_seconds", "length_seconds", "duration"); duration > 0 {
		target["duration_seconds"] = round3(duration)
	}
	if sourceHash := firstString(event, "source_hash"); sourceHash != "" {
		target["source_hash"] = sourceHash
	}
	return target
}

func kernelFeatureMaterializerPacket(event map[string]any, target map[string]any) map[string]any {
	requestID := kernelFeatureMaterializerRequestID(event)
	return map[string]any{
		"schema_version":  "mixboard_feature_request.v1",
		"status":          "materialized",
		"lifecycle":       "kernel_prepared_materializer",
		"source_kind":     "kernel_prepared_telemetry",
		"request_id":      requestID,
		"project_id":      "current",
		"resolved_target": target,
		"requested_features": kernelFeatureMaterializerRequestedFeatures(
			requestID,
			target,
		),
		"updated_at":       time.Now().UTC().Format(time.RFC3339Nano),
		"kernel_event_cmd": firstString(event, "command", "cmd"),
	}
}

func kernelFeatureMaterializerRequestedFeatures(requestID string, target map[string]any) []any {
	if strings.TrimSpace(requestID) == "" {
		return nil
	}
	out := make([]any, 0, 2)
	for _, featureType := range []string{"waveform_envelope", "spectral_field", "l3_acoustic_summary"} {
		row := map[string]any{
			"feature_type": featureType,
			"request_id":   requestID,
		}
		if trackID := firstString(target, "track_id"); trackID != "" {
			row["track_id"] = trackID
		}
		if clipID := firstString(target, "clip_id"); clipID != "" {
			row["clip_id"] = clipID
		}
		out = append(out, row)
	}
	return out
}

func kernelFeatureMaterializerRequestID(event map[string]any) string {
	trackID := safeRequestIDPart(firstString(event, "track_id", "source_track_id"))
	clipID := safeRequestIDPart(firstString(event, "clip_id"))
	featureType := safeRequestIDPart(firstNonEmpty(firstString(event, "feature_type"), "feature"))
	if clipID == "" {
		clipID = trackID
	}
	parts := []string{"kernel_prepared"}
	if featureType != "" {
		parts = append(parts, featureType)
	}
	if clipID != "" {
		parts = append(parts, clipID)
	}
	return strings.Join(parts, "_")
}

func (h *Harness) requestMixObservationFeaturesBackground(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef) map[string]any {
	packet := newMixboardFeatureRequestPacket(cmd, target)
	packet["lifecycle"] = "acoustic_package_background_fill"
	packet["priority"] = "background_warm"
	packet["trigger"] = "mix.observe_read_first_missing_or_stale"
	resolved := resolveMixboardFeatureTarget(cmd, state, target)
	packet["resolved_target"] = resolved
	if mixObservationWantsProjectAcoustics(cmd) {
		targets := visibleAudioTrackFeatureTargets(state)
		packet["scope"] = "full_project"
		packet["track_feature_targets"] = targets
		return h.requestMixObservationFeatureBackgroundTargets(ctx, cmd, packet, targets)
	}
	return h.requestMixObservationFeatureBackgroundTargets(ctx, cmd, packet, []map[string]any{resolved})
}

func (h *Harness) requestMixObservationFeatureBackgroundTargets(ctx context.Context, cmd map[string]any, packet map[string]any, targets []map[string]any) map[string]any {
	validTargets := make([]map[string]any, 0, len(targets))
	skipped := make([]any, 0)
	for _, targetRow := range targets {
		trackID := firstString(targetRow, "track_id")
		clipID := firstString(targetRow, "clip_id")
		if trackID == "" || clipID == "" {
			for _, featureType := range mixboardObservationFeatureTypes {
				skipped = append(skipped, map[string]any{
					"feature_type": featureType,
					"track_id":     trackID,
					"clip_id":      clipID,
					"reason":       "clip_source_required_for_current_feature_bakers",
				})
			}
			continue
		}
		validTargets = append(validTargets, targetRow)
	}
	if len(validTargets) == 0 {
		packet["status"] = "blocked"
		packet["reason"] = "clip_source_required_for_current_feature_bakers"
		packet["skipped_features"] = skipped
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	if h == nil || h.kernel == nil {
		packet["status"] = "blocked"
		packet["reason"] = "kernel_client_unavailable"
		packet["skipped_features"] = skipped
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	requested := make([]any, 0, len(validTargets)*len(mixboardObservationFeatureTypes))
	for _, targetRow := range validTargets {
		trackID := firstString(targetRow, "track_id")
		clipID := firstString(targetRow, "clip_id")
		for _, featureType := range mixboardObservationFeatureTypes {
			requestID := fmt.Sprintf("%s_%s_%s", firstString(packet, "request_id"), featureType, safeRequestIDPart(firstNonEmpty(clipID, trackID)))
			requestRow := map[string]any{
				"feature_type": featureType,
				"request_id":   requestID,
				"track_id":     trackID,
				"clip_id":      clipID,
				"priority":     "background_warm",
			}
			requestRow = stampMixboardFeatureRowIdentity(requestRow, packet, targetRow)
			requested = append(requested, requestRow)
			packet["status"] = "requested"
			packet["requested_features"] = requested
			packet["skipped_features"] = skipped
			writeMixboardFeatureRequestSnapshot(cmd, packet)
			h.startMixObservationFeatureBackgroundCollector(cmd, packet, targetRow, featureType)
		}
	}
	packet["requested_features"] = requested
	packet["skipped_features"] = skipped
	if len(requested) > len(skipped) {
		packet["status"] = "requested"
	} else {
		packet["status"] = "blocked"
		packet["reason"] = "all_feature_requests_skipped"
	}
	writeMixboardFeatureRequestSnapshot(cmd, packet)
	return packet
}

func (h *Harness) startMixObservationFeatureBackgroundCollector(cmd map[string]any, packet map[string]any, targetRow map[string]any, featureType string) {
	if h == nil || h.kernel == nil {
		return
	}
	cmdCopy := cloneAnyMap(cmd)
	packetCopy := cloneAnyMap(packet)
	targetCopy := cloneAnyMap(targetRow)
	featureType = strings.TrimSpace(featureType)
	collectorReady := make(chan struct{})
	go h.collectMixObservationFeatureBackground(cmdCopy, packetCopy, targetCopy, featureType, collectorReady)
	select {
	case <-collectorReady:
	case <-time.After(mixboardFeatureBackgroundSubscribeWait()):
	}
	kernelCmd := mixObservationBackgroundFeatureKernelCommand(packetCopy, targetCopy, featureType)
	sendCtx, cancel := context.WithTimeout(context.Background(), mixboardFeatureBackgroundSendWait())
	defer cancel()
	reply, _, err := h.kernel.SendCommand(sendCtx, kernelCmd)
	if (err != nil || !kernelReplySucceeded(reply)) && h.logger != nil {
		reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), fmt.Sprint(err), "kernel_feature_request_failed")
		h.logger.Warn("[mixboard] background feature request failed track_id=%s clip_id=%s feature_type=%s err=%s", firstString(targetCopy, "track_id"), firstString(targetCopy, "clip_id"), featureType, reason)
	}
}

func mixObservationBackgroundFeatureKernelCommand(packet map[string]any, targetRow map[string]any, featureType string) map[string]any {
	trackID := firstString(targetRow, "track_id")
	clipID := firstString(targetRow, "clip_id")
	kernelCmd := map[string]any{
		"cmd":                 "warm_waveform_bake",
		"command":             "mixboard_acoustic_package_background_fill",
		"track_id":            trackID,
		"clip_id":             clipID,
		"feature_type":        featureType,
		"mixboard_request_id": firstString(packet, "request_id"),
		"priority":            "background_warm",
		"source_kind":         "mix_observe_background_fill",
	}
	if filePath := firstString(targetRow, "file_path", "source_path", "current_source_path"); filePath != "" {
		kernelCmd["file_path"] = filePath
	}
	return kernelCmd
}

func (h *Harness) collectMixObservationFeatureBackground(cmd map[string]any, packet map[string]any, targetRow map[string]any, featureType string, collectorReady chan<- struct{}) {
	trackID := firstString(targetRow, "track_id")
	clipID := firstString(targetRow, "clip_id")
	readyClosed := false
	closeCollectorReady := func() {
		if collectorReady != nil && !readyClosed {
			close(collectorReady)
			readyClosed = true
		}
	}
	defer closeCollectorReady()
	if h == nil || h.kernel == nil || trackID == "" || clipID == "" || featureType == "" {
		return
	}
	timeout := mixboardFeatureBackgroundCollectWait()
	opCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	markReady := func() error {
		closeCollectorReady()
		return nil
	}
	switch featureType {
	case "waveform_envelope":
		collector, err := collectWaveformFeatureTiles(opCtx, mixboardFeatureSubURL, trackID, clipID, timeout, markReady)
		if err != nil && h.logger != nil {
			h.logger.Warn("[mixboard] background waveform collector failed track_id=%s clip_id=%s err=%v", trackID, clipID, err)
		}
		if collector != nil && collector.TileCount() > 0 {
			if row := collector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
				row = stampMixboardFeatureRowIdentity(row, packet, targetRow)
				writeMixboardReadyProjectTrackWaveformSnapshot(cmd, packet, row)
			}
		}
	case "spectral_field":
		collector, err := collectSpectralFeatureTiles(opCtx, mixboardFeatureSubURL, trackID, clipID, timeout, markReady)
		if err != nil && h.logger != nil {
			h.logger.Warn("[mixboard] background spectral collector failed track_id=%s clip_id=%s err=%v", trackID, clipID, err)
		}
		if collector != nil {
			recordSpectralCollectorStatus(packet, collector, trackID, clipID)
			if collector.TileCount() > 0 {
				if row := collector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
					row = stampMixboardFeatureRowIdentity(row, packet, targetRow)
					writeMixboardReadySpectralSnapshot(cmd, packet, row)
				}
			}
		}
	default:
		return
	}
}

func acousticPackageStatusTypedEvent(status map[string]any, artifactPath string) map[string]any {
	if len(status) == 0 {
		return nil
	}
	typed := agentprotocol.AcousticPackageStatus{
		ID:             agentprotocol.NormalizeID("acoustic_package_status", firstString(status, "project_id"), firstString(status, "track_id"), firstString(status, "clip_id"), firstString(status, "source_revision")),
		Kind:           agentprotocol.KindAcousticPackageStatus,
		SchemaVersion:  firstNonEmpty(firstString(status, "schema_version"), acousticpackage.SchemaVersion),
		Status:         firstString(status, "status"),
		ProjectID:      firstString(status, "project_id"),
		TrackID:        firstString(status, "track_id"),
		ClipID:         firstString(status, "clip_id"),
		SourceHash:     firstString(status, "source_hash"),
		SourceRevision: firstString(status, "source_revision"),
		ArtifactPath:   artifactPath,
		PackageLayers:  mapAnyFromAny(status["package_layers"]),
		CreatedAt:      firstString(status, "updated_at"),
		Source: agentprotocol.Source{
			LegacySchema: acousticpackage.SchemaVersion,
			LegacyKind:   "mix.observe",
		},
	}
	return agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source))
}

func (h *Harness) readMixObservation(cmd map[string]any) (map[string]any, error) {
	req := mixboard.ReadRequest{
		ObservationID: firstString(cmd, "observation_id"),
		MixSessionID:  firstString(cmd, "mix_session_id", "session_id"),
		Keys:          observationReadKeys(cmd),
		Detail:        firstString(cmd, "detail"),
		MaxItems:      int(numberFromAny(cmd["max_items"])),
	}
	req.RangeStart, req.RangeEnd = observationReadRange(cmd)
	return mixboard.NewStore("").Read(req)
}

func (h *Harness) deriveMixObservation(cmd map[string]any) (map[string]any, error) {
	req := mixboard.DeriveRequest{
		ObservationID: firstString(cmd, "observation_id"),
		MixSessionID:  firstString(cmd, "mix_session_id", "session_id"),
		Type:          firstString(cmd, "type", "derive_type", "relationship_type"),
		A:             mapAnyFromAny(cmd["a"]),
		B:             mapAnyFromAny(cmd["b"]),
		Focus:         mapAnyFromAny(cmd["focus"]),
		Dimensions:    stringSliceFromAny(cmd["dimensions"]),
		MaxItems:      int(numberFromAny(cmd["max_items"])),
	}
	return mixboard.NewStore("").Derive(req)
}

func observationReadKeys(cmd map[string]any) []string {
	keys := stringSliceFromAny(cmd["keys"])
	if len(keys) == 0 {
		if key := firstString(cmd, "key"); key != "" {
			keys = []string{key}
		}
	}
	return keys
}

func observationReadRange(cmd map[string]any) (float64, float64) {
	if rows, ok := cmd["range_sec"].([]any); ok && len(rows) >= 2 {
		return numberFromAny(rows[0]), numberFromAny(rows[1])
	}
	if rows, ok := cmd["range_seconds"].([]any); ok && len(rows) >= 2 {
		return numberFromAny(rows[0]), numberFromAny(rows[1])
	}
	rangeMap := mapAnyFromAny(cmd["range_sec"])
	if len(rangeMap) == 0 {
		rangeMap = mapAnyFromAny(cmd["range_seconds"])
	}
	start := firstNonZeroNumber(
		numberFromAny(rangeMap["start"]),
		numberFromAny(rangeMap["start_seconds"]),
		numberFromAny(cmd["start_seconds"]),
		numberFromAny(cmd["range_start"]),
	)
	end := firstNonZeroNumber(
		numberFromAny(rangeMap["end"]),
		numberFromAny(rangeMap["end_seconds"]),
		numberFromAny(cmd["end_seconds"]),
		numberFromAny(cmd["range_end"]),
	)
	return start, end
}

func firstNonZeroNumber(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func mixObservationResolutionNeedsRefresh(resolved map[string]any) bool {
	trackID := firstString(resolved, "track_id")
	clipID := firstString(resolved, "clip_id")
	if trackID == "" || clipID == "" {
		return true
	}
	return looksSyntheticTrackAlias(trackID)
}

func mixObservationResolutionNeedsRefreshForCommand(cmd map[string]any, resolved map[string]any) bool {
	switch normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope")) {
	case "full_project", "track_group":
		return false
	default:
		return mixObservationResolutionNeedsRefresh(resolved)
	}
}

func mixObservationScopeNeedsFreshProjectState(cmd map[string]any) bool {
	switch normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope")) {
	case "full_project", "full_project_with_focus_track", "track_group":
		return true
	default:
		return false
	}
}

func canonicalizeMixObservationCommand(cmd map[string]any, target mixboard.TargetRef, resolved map[string]any) map[string]any {
	cmd = cloneAnyMap(cmd)
	if mixObservationHasExplicitSourceIdentity(cmd) {
		resolved = mixObservationResolvedWithExplicitSourceIdentity(resolved, cmd)
	}
	scope := normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope"))
	if scope == "" {
		scope = "selected_track"
	}
	cmd["scope"] = scope
	trackID := firstString(resolved, "track_id")
	clipID := firstString(resolved, "clip_id")
	if scope == "full_project" || scope == "track_group" {
		cmd["listen_scope"] = map[string]any{
			"time": map[string]any{
				"mode":   "full_song",
				"source": "scope_" + scope,
			},
			"source": map[string]any{
				"mode":   scope,
				"source": "mix_observation_scope",
			},
		}
		if scope == "full_project" {
			cmd["target_ref"] = map[string]any{
				"kind":       "project",
				"id":         "current",
				"label":      "Current project",
				"source":     "mix_observation_scope",
				"confidence": "high",
			}
			cmd["mix_objects"] = []any{map[string]any{
				"mode":   "scope",
				"kind":   "project",
				"id":     "current",
				"label":  "Current project",
				"source": "mix_observation_scope",
			}}
		}
		return cmd
	}
	if trackID != "" {
		cmd["track_id"] = trackID
	}
	if clipID != "" {
		cmd["clip_id"] = clipID
	}
	if path := firstString(resolved, "file_path"); path != "" {
		cmd["file_path"] = path
	}
	if duration := firstPositiveNumber(resolved, "duration_seconds", "length_seconds", "duration"); duration > 0 {
		cmd["duration_seconds"] = duration
	}
	if trackID != "" && !looksSyntheticTrackAlias(trackID) {
		label := firstNonEmpty(firstString(resolved, "track_name"), target.Label, trackID)
		sourceMode := "full_mix_context"
		if scope == "selected_track" || scope == "named_track" {
			sourceMode = scope + "_with_project_context"
		}
		if scope == "full_project_with_focus_track" {
			sourceMode = "full_project_with_focus_track"
		}
		cmd["target_ref"] = map[string]any{
			"kind":       "track",
			"id":         trackID,
			"label":      label,
			"source":     firstNonEmpty(firstString(resolved, "source"), target.Source, "resolved_observation_target"),
			"confidence": firstNonEmpty(target.Confidence, "high"),
		}
		cmd["mix_objects"] = []any{map[string]any{
			"mode":         "target_ref",
			"kind":         "track",
			"id":           trackID,
			"label":        label,
			"effect_scope": "track_rack",
			"source":       "resolved_observation_target",
		}}
		cmd["listen_scope"] = map[string]any{
			"time": map[string]any{
				"mode":   "full_song",
				"source": "default",
			},
			"source": map[string]any{
				"mode":      sourceMode,
				"focus_ids": []any{trackID},
			},
		}
	}
	return cmd
}

func mixObservationHasExplicitSourceIdentity(cmd map[string]any) bool {
	return firstNonEmpty(
		firstString(cmd, "file_path"),
		firstString(cmd, "source_path"),
		firstString(cmd, "source_revision"),
		firstString(cmd, "source_fingerprint"),
		firstString(cmd, "source_hash"),
	) != ""
}

func mixObservationResolvedWithExplicitSourceIdentity(resolved, cmd map[string]any) map[string]any {
	out := cloneAnyMap(resolved)
	if trackID := firstString(cmd, "track_id"); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := firstString(cmd, "clip_id", "selected_clip_id"); clipID != "" {
		out["clip_id"] = clipID
	}
	if path := firstNonEmpty(firstString(cmd, "file_path"), firstString(cmd, "source_path")); path != "" {
		out["file_path"] = path
		out["source_path"] = path
		out["current_source_path"] = path
	} else {
		delete(out, "file_path")
		delete(out, "source_path")
		delete(out, "current_source_path")
	}
	if duration := firstPositiveNumber(cmd, "duration_seconds", "length_seconds", "duration"); duration > 0 {
		out["duration_seconds"] = duration
	} else {
		delete(out, "duration_seconds")
		delete(out, "length_seconds")
		delete(out, "duration")
	}
	for _, key := range []string{"source_revision", "source_fingerprint", "source_hash", "clip_revision", "render_revision", "analyzer_revision"} {
		if value := firstString(cmd, key); value != "" {
			out[key] = value
		}
	}
	out["source"] = firstNonEmpty(firstString(out, "source"), "explicit_source_identity")
	return out
}

func normalizeMixObservationScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "selected_clip", "clip":
		return "selected_clip"
	case "selected_track", "current_track", "track":
		return "selected_track"
	case "named_track":
		return "named_track"
	case "track_group", "group":
		return "track_group"
	case "full_project", "project", "whole_project":
		return "full_project"
	case "full_project_with_focus_track", "project_with_focus", "focus_track":
		return "full_project_with_focus_track"
	default:
		return ""
	}
}

func resolveMixObservationFocusHint(cmd map[string]any, state map[string]any) map[string]any {
	hint := mapAnyFromAny(cmd["focus_hint"])
	if len(hint) == 0 {
		return nil
	}
	role := strings.ToLower(strings.TrimSpace(firstString(hint, "role", "target_role")))
	nameHint := strings.ToLower(strings.TrimSpace(firstString(hint, "name", "track_name", "query")))
	userTrackIndex, hasUserTrackIndex := firstPositiveInt(hint, "user_track_index", "track_index", "track_number", "target_track_index", "target_user_track_index")
	if role == "" && nameHint == "" && !hasUserTrackIndex {
		return nil
	}
	rows := visibleTrackRows(state)
	if hasUserTrackIndex {
		for _, row := range rows {
			if index, ok := firstPositiveInt(row, "user_track_index", "track_index", "index"); ok && index == userTrackIndex {
				return mixObservationFocusHintResolvedTrack(row, 6)
			}
		}
	}
	var best map[string]any
	bestScore := 0
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		score := mixObservationFocusHintScore(row, role, nameHint)
		if score > bestScore {
			best = row
			bestScore = score
		}
	}
	if bestScore < 3 || len(best) == 0 {
		return nil
	}
	return mixObservationFocusHintResolvedTrack(best, bestScore)
}

func mixObservationFocusHintResolvedTrack(best map[string]any, bestScore int) map[string]any {
	trackID := firstNonEmpty(firstString(best, "track_id"), firstString(best, "id"))
	if trackID == "" || looksSyntheticTrackAlias(trackID) {
		return nil
	}
	name := firstNonEmpty(firstString(best, "track_name"), firstString(best, "name"), trackID)
	out := map[string]any{
		"track_id":   trackID,
		"track_name": name,
		"source":     "focus_hint",
		"confidence": "medium",
	}
	if bestScore >= 5 {
		out["confidence"] = "high"
	}
	if clips := mapRowsFromAny(firstPresentValue(best, "clips", "clip_summaries")); len(clips) > 0 {
		if clip := clips[0]; len(clip) > 0 {
			if clipID := firstNonEmpty(firstString(clip, "clip_id"), firstString(clip, "id")); clipID != "" {
				out["clip_id"] = clipID
			}
			if path := firstString(clip, "file_path", "source_file", "audio_file"); path != "" {
				out["file_path"] = path
			}
			if duration := firstPositiveNumber(clip, "duration_seconds", "length_seconds", "duration"); duration > 0 {
				out["duration_seconds"] = duration
			}
		}
	}
	return out
}

func firstPresentValue(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			return value
		}
	}
	return nil
}

func mixObservationFocusHintScore(row map[string]any, role, nameHint string) int {
	name := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		firstString(row, "track_name"),
		firstString(row, "name"),
		firstString(row, "track_type"),
	}, " ")))
	guessedRole := mixObservationTrackRoleGuess(name)
	score := 0
	if role != "" {
		switch role {
		case "vocal", "voice", "lead_vocal", "lead vocal":
			if guessedRole == "vocal" {
				score += 5
			}
		case "bass", "kick":
			if guessedRole == role {
				score += 5
			}
		default:
			if strings.Contains(name, role) || guessedRole == role {
				score += 4
			}
		}
	}
	if nameHint != "" && strings.Contains(name, nameHint) {
		score += 3
	}
	if boolValueDefault(row["selected"], false) {
		score++
	}
	if strings.Contains(name, "lead") || strings.Contains(name, "\u4e3b\u5531") {
		score++
	}
	return score
}

func mixObservationTrackRoleGuess(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(text, "vocal") || strings.Contains(text, "vox") || strings.Contains(text, "voice") || strings.Contains(text, "lead vocal") || strings.Contains(text, "lead vox") || strings.Contains(text, "\u4e3b\u5531") || strings.Contains(text, "\u4eba\u58f0") || strings.Contains(text, "\u6b4c\u58f0") || strings.Contains(text, "\u58f0\u4e50"):
		return "vocal"
	case strings.Contains(text, "kick") || strings.Contains(text, "bass drum") || strings.Contains(text, "\u5e95\u9f13") || strings.Contains(text, "\u5927\u9f13"):
		return "kick"
	case strings.Contains(text, "bass") || strings.Contains(text, "sub") || strings.Contains(text, "808") || strings.Contains(text, "\u8d1d\u65af") || strings.Contains(text, "\u4f4e\u97f3") || strings.Contains(text, "\u4f4e\u9891"):
		return "bass"
	default:
		return ""
	}
}

func (h *Harness) requestMixObservationFeatures(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef) map[string]any {
	packet := newMixboardFeatureRequestPacket(cmd, target)
	resolved := resolveMixboardFeatureTarget(cmd, state, target)
	packet["resolved_target"] = resolved
	if mixObservationWantsProjectAcoustics(cmd) {
		return h.requestFullProjectMixObservationFeatures(ctx, cmd, state, target, packet)
	}
	trackID := firstString(resolved, "track_id")
	clipID := firstString(resolved, "clip_id")
	if trackID == "" || clipID == "" {
		packet["status"] = "blocked"
		packet["reason"] = "clip_source_required_for_current_feature_bakers"
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	if h == nil || h.kernel == nil {
		packet["status"] = "blocked"
		packet["reason"] = "kernel_client_unavailable"
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	requested := make([]any, 0, len(mixboardObservationFeatureTypes))
	skipped := make([]any, 0)
	for _, featureType := range mixboardObservationFeatureTypes {
		requestID := fmt.Sprintf("%s_%s", firstString(packet, "request_id"), featureType)
		kernelCmd := map[string]any{
			"cmd":                 "warm_waveform_bake",
			"command":             "mixboard_request_observation_features",
			"track_id":            trackID,
			"clip_id":             clipID,
			"feature_type":        featureType,
			"mixboard_request_id": firstString(packet, "request_id"),
		}
		requestRow := map[string]any{
			"feature_type": featureType,
			"request_id":   requestID,
			"track_id":     trackID,
			"clip_id":      clipID,
		}
		packet["status"] = "requested"
		packet["requested_features"] = append(append([]any{}, requested...), requestRow)
		packet["skipped_features"] = skipped
		writeMixboardFeatureRequestSnapshot(cmd, packet)

		var reply map[string]any
		sent := false
		sendBake := func() error {
			if sent {
				return nil
			}
			sent = true
			var sendErr error
			reply, _, sendErr = h.kernel.SendCommand(ctx, kernelCmd)
			if sendErr != nil || !kernelReplySucceeded(reply) {
				reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), fmt.Sprint(sendErr), "kernel_feature_request_failed")
				return fmt.Errorf("%s", reason)
			}
			return nil
		}
		var collector *waveformFeatureCollector
		var spectralCollector *spectralFeatureCollector
		if featureType == "waveform_envelope" {
			collector, _ = collectWaveformFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
		} else if featureType == "spectral_field" {
			spectralCollector, _ = collectSpectralFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
		} else {
			_ = sendBake()
		}
		if !sent {
			_ = sendBake()
		}
		if !kernelReplySucceeded(reply) {
			reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel_feature_request_failed")
			skipped = append(skipped, map[string]any{"feature_type": featureType, "reason": reason})
			continue
		}
		requested = append(requested, requestRow)
		if collector != nil && collector.TileCount() > 0 {
			if row := collector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
				writeMixboardReadyProjectTrackWaveformSnapshot(cmd, packet, row)
			}
		}
		if spectralCollector != nil {
			recordSpectralCollectorStatus(packet, spectralCollector, trackID, clipID)
			if spectralCollector.TileCount() > 0 {
				if row := spectralCollector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
					writeMixboardReadySpectralSnapshot(cmd, packet, row)
				}
			}
		}
	}
	packet["requested_features"] = requested
	packet["skipped_features"] = skipped
	if len(requested) > 0 {
		packet["status"] = "requested"
	} else {
		packet["status"] = "blocked"
		packet["reason"] = "all_feature_requests_skipped"
	}
	if len(requested) == 0 || !mixboardFeatureRowsReadyForTarget(readMixboardFeatureSnapshotFile(mixboard.FeatureSnapshotPath(cmd)), trackID, clipID) {
		writeMixboardFeatureRequestSnapshot(cmd, packet)
	}
	waitForMixboardFeatureSnapshotReady(ctx, cmd, trackID, clipID, mixboardFeatureReadyWait())
	waitForMixboardAcousticBridgeSnapshot(ctx, cmd, packet, mixboardFeatureReadyWait())
	finalizeMixboardFeatureSnapshotAfterWait(cmd, packet, []map[string]any{{"track_id": trackID, "clip_id": clipID}})
	return packet
}

func (h *Harness) requestFullProjectMixObservationFeatures(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef, packet map[string]any) map[string]any {
	targets := visibleAudioTrackFeatureTargets(state)
	packet["scope"] = "full_project"
	packet["track_feature_targets"] = targets
	if len(targets) == 0 {
		packet["status"] = "blocked"
		packet["reason"] = "visible_audio_tracks_required_for_project_acoustic_observation"
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	if h == nil || h.kernel == nil {
		packet["status"] = "blocked"
		packet["reason"] = "kernel_client_unavailable"
		writeMixboardFeatureRequestSnapshot(cmd, packet)
		return packet
	}
	requested := make([]any, 0, len(targets)*len(mixboardObservationFeatureTypes))
	skipped := make([]any, 0)
	for _, targetRow := range targets {
		trackID := firstString(targetRow, "track_id")
		clipID := firstString(targetRow, "clip_id")
		if trackID == "" || clipID == "" {
			for _, featureType := range mixboardObservationFeatureTypes {
				skipped = append(skipped, map[string]any{
					"feature_type": featureType,
					"track_id":     trackID,
					"clip_id":      clipID,
					"reason":       "clip_source_required_for_current_feature_bakers",
				})
			}
			continue
		}
		for _, featureType := range mixboardObservationFeatureTypes {
			requestID := fmt.Sprintf("%s_%s_%s", firstString(packet, "request_id"), featureType, safeRequestIDPart(trackID))
			requestRow := map[string]any{
				"feature_type": featureType,
				"request_id":   requestID,
				"track_id":     trackID,
				"clip_id":      clipID,
			}
			packet["status"] = "requested"
			packet["requested_features"] = append(append([]any{}, requested...), requestRow)
			packet["skipped_features"] = skipped
			writeMixboardFeatureRequestSnapshot(cmd, packet)

			kernelCmd := map[string]any{
				"cmd":                 "warm_waveform_bake",
				"command":             "mixboard_request_observation_features",
				"track_id":            trackID,
				"clip_id":             clipID,
				"feature_type":        featureType,
				"mixboard_request_id": firstString(packet, "request_id"),
			}
			var reply map[string]any
			sent := false
			sendBake := func() error {
				if sent {
					return nil
				}
				sent = true
				var sendErr error
				reply, _, sendErr = h.kernel.SendCommand(ctx, kernelCmd)
				if sendErr != nil || !kernelReplySucceeded(reply) {
					reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), fmt.Sprint(sendErr), "kernel_feature_request_failed")
					return fmt.Errorf("%s", reason)
				}
				return nil
			}
			var collector *waveformFeatureCollector
			var spectralCollector *spectralFeatureCollector
			if featureType == "waveform_envelope" {
				collector, _ = collectWaveformFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
			} else if featureType == "spectral_field" {
				spectralCollector, _ = collectSpectralFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
			} else {
				_ = sendBake()
			}
			if !sent {
				_ = sendBake()
			}
			if !kernelReplySucceeded(reply) {
				reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel_feature_request_failed")
				skipped = append(skipped, map[string]any{"feature_type": featureType, "track_id": trackID, "clip_id": clipID, "reason": reason})
				continue
			}
			requested = append(requested, requestRow)
			if collector != nil && collector.TileCount() > 0 {
				if row := collector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
					writeMixboardReadyProjectTrackWaveformSnapshot(cmd, packet, row)
				}
			}
			if spectralCollector != nil {
				recordSpectralCollectorStatus(packet, spectralCollector, trackID, clipID)
				if spectralCollector.TileCount() > 0 {
					if row := spectralCollector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
						writeMixboardReadySpectralSnapshot(cmd, packet, row)
					}
				}
			}
		}
	}
	packet["requested_features"] = requested
	packet["skipped_features"] = skipped
	if len(requested) > 0 {
		packet["status"] = "requested"
	} else {
		packet["status"] = "blocked"
		packet["reason"] = "all_feature_requests_skipped"
	}
	if len(requested) == 0 || !mixboardProjectFeatureRowsReady(readMixboardFeatureSnapshotFile(mixboard.FeatureSnapshotPath(cmd)), targets) {
		writeMixboardFeatureRequestSnapshot(cmd, packet)
	}
	writeMixboardProjectSpectralSnapshotFromCollectors(cmd, packet, targets)
	waitForMixboardProjectFeatureSnapshotReady(ctx, cmd, targets, mixboardFeatureReadyWait())
	waitForMixboardAcousticBridgeSnapshot(ctx, cmd, packet, mixboardFeatureReadyWait())
	finalizeMixboardFeatureSnapshotAfterWait(cmd, packet, targets)
	return packet
}

func collectMixboardWaveformSnapshot(ctx context.Context, cmd map[string]any, packet map[string]any, trackID, clipID string, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	collector, err := collectWaveformFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, timeout, nil)
	if err != nil || collector == nil || collector.TileCount() == 0 {
		return
	}
	row := collector.SnapshotRow(firstString(packet, "request_id"))
	if len(row) == 0 {
		return
	}
	writeMixboardReadyWaveformSnapshot(cmd, packet, row)
}

type waveformFeatureCollector struct {
	TrackID          string
	ClipID           string
	FilePath         string
	SourceRevision   string
	ClipRevision     string
	RenderRevision   string
	AnalyzerRevision string
	TotalDuration    float64
	ExpectedTiles    int
	FrameCount       int
	FeatureStride    int
	FloatCount       int
	TilesSeen        int
	TileIndexes      map[int]bool
	RawTileEvents    int
	QualityStatus    string
	QualityReason    string
	NonzeroCount     int64
	SumAbs           float64
	MaxAbs           float64
	NanInfCount      int64
	PeakAbs          float64
	SumSquares       float64
	SampleFrames     int64
	TimeSegments     []map[string]any
	FirstReceivedAt  string
	LastReceivedAt   string
}

func (c *waveformFeatureCollector) AddEvent(event map[string]any) {
	if c == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if c.FirstReceivedAt == "" {
		c.FirstReceivedAt = now
	}
	c.LastReceivedAt = now
	c.TrackID = firstNonEmpty(c.TrackID, firstString(event, "track_id", "source_track_id"))
	c.ClipID = firstNonEmpty(c.ClipID, firstString(event, "clip_id"))
	c.FilePath = firstNonEmpty(c.FilePath, firstString(event, "file_path"))
	c.SourceRevision = firstNonEmpty(c.SourceRevision, firstString(event, "source_revision", "source_fingerprint"))
	c.ClipRevision = firstNonEmpty(c.ClipRevision, firstString(event, "clip_revision"))
	c.RenderRevision = firstNonEmpty(c.RenderRevision, firstString(event, "render_revision"))
	c.AnalyzerRevision = firstNonEmpty(c.AnalyzerRevision, firstString(event, "analyzer_revision", "analysis_version"))
	c.QualityStatus = combineKernelQualityStatus(c.QualityStatus, firstString(event, "quality_status"))
	c.QualityReason = combineKernelQualityReason(c.QualityReason, firstString(event, "quality_reason"))
	if duration := numberFromAny(event["total_duration"]); duration > 0 {
		c.TotalDuration = duration
	}
	frameCount := int(numberFromAny(event["resolution_frame_count"]))
	stride := int(numberFromAny(event["feature_stride"]))
	floatCount := int(numberFromAny(event["float_count"]))
	if frameCount <= 0 || stride <= 0 || floatCount <= 0 {
		return
	}
	c.FrameCount = frameCount
	c.FeatureStride = stride
	if c.ExpectedTiles <= 0 {
		c.ExpectedTiles = expectedWaveformTileCount(c.TotalDuration)
	}
	tileIndexValue := firstPresentValue(event, "tile_index")
	tileIndex := int(numberFromAny(tileIndexValue))
	hasTileIndex := tileIndexValue != nil
	if hasTileIndex {
		if c.TileIndexes == nil {
			c.TileIndexes = map[int]bool{}
		}
		if c.TileIndexes[tileIndex] {
			c.RawTileEvents++
			return
		}
	}
	floats, err := readSharedFloat32Array(firstString(event, "shared_memory"), floatCount)
	if err != nil || len(floats) < stride {
		return
	}
	tileStart := numberFromAny(event["tile_content_start_seconds"])
	tileDuration := numberFromAny(event["tile_duration"])
	frameDuration := numberFromAny(event["frame_duration_seconds"])
	var tilePeak float64
	var tileSumSquares float64
	var tileFrames int64
	for frame := 0; frame < frameCount; frame++ {
		base := frame * stride
		if base+5 >= len(floats) {
			break
		}
		minL := math.Abs(float64(floats[base+0]))
		maxL := math.Abs(float64(floats[base+1]))
		minR := math.Abs(float64(floats[base+2]))
		maxR := math.Abs(float64(floats[base+3]))
		lRMS := float64(floats[base+4])
		rRMS := float64(floats[base+5])
		peak := math.Max(math.Max(minL, maxL), math.Max(minR, maxR))
		rms := math.Sqrt((lRMS*lRMS + rRMS*rRMS) / 2)
		if peak > tilePeak {
			tilePeak = peak
		}
		if peak > c.PeakAbs {
			c.PeakAbs = peak
		}
		tileSumSquares += rms * rms
		tileFrames++
	}
	if tileFrames <= 0 {
		return
	}
	c.RawTileEvents++
	if hasTileIndex {
		c.TileIndexes[tileIndex] = true
		c.TilesSeen = len(c.TileIndexes)
	} else {
		c.TilesSeen++
	}
	c.FloatCount += floatCount
	c.NonzeroCount += int64(numberFromAny(event["nonzero_count"]))
	c.SumAbs += numberFromAny(event["sum_abs"])
	if maxAbs := numberFromAny(event["max_abs"]); maxAbs > c.MaxAbs {
		c.MaxAbs = maxAbs
	}
	c.NanInfCount += int64(numberFromAny(event["nan_inf_count"]))
	c.SumSquares += tileSumSquares
	c.SampleFrames += tileFrames
	tileRMS := math.Sqrt(tileSumSquares / float64(tileFrames))
	if tileDuration <= 0 && frameDuration > 0 {
		tileDuration = frameDuration * float64(tileFrames)
	}
	c.TimeSegments = append(c.TimeSegments, map[string]any{
		"start_seconds": tileStart,
		"end_seconds":   round3(tileStart + tileDuration),
		"rms":           round6(tileRMS),
		"rms_dbfs":      dbfsValue(tileRMS),
		"peak_abs":      round6(tilePeak),
		"peak_dbfs":     dbfsValue(tilePeak),
		"crest_db":      crestDBValue(tilePeak, tileRMS),
		"energy_state":  waveformEnergyState(tileRMS),
	})
}

func (c *waveformFeatureCollector) TileCount() int {
	if c == nil {
		return 0
	}
	if len(c.TileIndexes) > 0 {
		return len(c.TileIndexes)
	}
	return c.TilesSeen
}

func (c *waveformFeatureCollector) Complete() bool {
	if c == nil || c.TileCount() <= 0 {
		return false
	}
	return c.ExpectedTiles > 0 && c.TileCount() >= c.ExpectedTiles
}

func (c *waveformFeatureCollector) SnapshotRow(requestID string) map[string]any {
	if c == nil || c.TilesSeen <= 0 || c.SampleFrames <= 0 {
		return nil
	}
	rms := math.Sqrt(c.SumSquares / float64(c.SampleFrames))
	coverageStatus := "partial"
	if c.Complete() {
		coverageStatus = "ready"
	}
	qualityStatus, qualityReason := c.EffectiveQuality()
	status := statusFromKernelQuality(coverageStatus, qualityStatus)
	row := map[string]any{
		"status":              status,
		"track_id":            c.TrackID,
		"clip_id":             c.ClipID,
		"file_path":           c.FilePath,
		"request_id":          requestID,
		"source":              "kernel_audio_feature_data_ready",
		"total_duration":      round3(c.TotalDuration),
		"rms":                 round6(rms),
		"peak_abs":            round6(c.PeakAbs),
		"rms_dbfs":            dbfsValue(rms),
		"peak_dbfs":           dbfsValue(c.PeakAbs),
		"headroom_db":         headroomDBValue(c.PeakAbs),
		"crest_db":            crestDBValue(c.PeakAbs, rms),
		"float_count":         c.FloatCount,
		"tile_count_seen":     c.TileCount(),
		"tile_count_expected": c.ExpectedTiles,
		"time_segments":       c.TimeSegments,
		"updated_at":          firstNonEmpty(c.LastReceivedAt, time.Now().UTC().Format(time.RFC3339Nano)),
	}
	if c.RawTileEvents > c.TileCount() {
		row["tile_event_count"] = c.RawTileEvents
	}
	if qualityStatus != strings.TrimSpace(c.QualityStatus) && strings.TrimSpace(c.QualityStatus) != "" {
		row["tile_quality_status"] = strings.TrimSpace(c.QualityStatus)
	}
	if qualityReason != strings.TrimSpace(c.QualityReason) && strings.TrimSpace(c.QualityReason) != "" {
		row["tile_quality_reason"] = strings.TrimSpace(c.QualityReason)
	}
	stampKernelEventIdentityAndQuality(row, c.SourceRevision, c.ClipRevision, c.RenderRevision, c.AnalyzerRevision, qualityStatus, qualityReason, c.NonzeroCount, c.SumAbs, c.MaxAbs, c.NanInfCount)
	if c.FirstReceivedAt != "" {
		row["first_received_at"] = c.FirstReceivedAt
	}
	return row
}

func (c *waveformFeatureCollector) EffectiveQuality() (string, string) {
	if c == nil {
		return "", ""
	}
	status := strings.ToLower(strings.TrimSpace(c.QualityStatus))
	reason := strings.TrimSpace(c.QualityReason)
	if (status == "suspect" || status == "partial") &&
		c.NanInfCount == 0 &&
		(c.NonzeroCount > 0 || c.SumAbs > 0 || c.MaxAbs > 0 || c.PeakAbs > 0) &&
		waveformQualityReasonsOnlySilentTiles(reason) {
		return "ready", "ok_with_silent_tiles"
	}
	return status, reason
}

func waveformQualityReasonsOnlySilentTiles(reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return false
	}
	seenSilentTile := false
	for _, part := range strings.Split(reason, ";") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		switch part {
		case "ok":
			continue
		case "input_all_zero":
			seenSilentTile = true
		default:
			return false
		}
	}
	return seenSilentTile
}

func collectWaveformFeatureTiles(ctx context.Context, subURL, trackID, clipID string, timeout time.Duration, afterSubscribe func() error) (*waveformFeatureCollector, error) {
	if strings.TrimSpace(subURL) == "" {
		subURL = mixboardFeatureSubURL
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return nil, err
	}
	if err := sub.Dial(subURL); err != nil {
		return nil, err
	}
	collector := &waveformFeatureCollector{TrackID: trackID, ClipID: clipID}
	if afterSubscribe != nil {
		warmMixboardFeatureSubscriber(opCtx)
		if err := afterSubscribe(); err != nil {
			return collector, err
		}
	}
	for {
		select {
		case <-opCtx.Done():
			return collector, nil
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return collector, nil
			}
			return collector, err
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(zmqMsgPayload(msg)), &event); err != nil {
			continue
		}
		if !strings.EqualFold(firstString(event, "command"), "audio_feature_data_ready") {
			continue
		}
		if !strings.EqualFold(firstString(event, "feature_type"), "waveform_envelope") {
			continue
		}
		if trackID != "" && firstString(event, "track_id", "source_track_id") != trackID {
			continue
		}
		if clipID != "" && firstString(event, "clip_id") != clipID {
			continue
		}
		collector.AddEvent(event)
		if collector.Complete() {
			return collector, nil
		}
	}
}

func collectL2RenderProbeEvent(ctx context.Context, subURL, trackID, clipID, requestID string, timeout time.Duration, afterSubscribe func() error) (map[string]any, error) {
	if strings.TrimSpace(subURL) == "" {
		subURL = mixboardFeatureSubURL
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return nil, err
	}
	if err := sub.Dial(subURL); err != nil {
		if afterSubscribe != nil {
			_ = afterSubscribe()
		}
		return nil, err
	}
	if afterSubscribe != nil {
		warmMixboardFeatureSubscriber(opCtx)
		if err := afterSubscribe(); err != nil {
			return nil, err
		}
	}
	for {
		select {
		case <-opCtx.Done():
			return nil, nil
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return nil, nil
			}
			return nil, err
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(zmqMsgPayload(msg)), &event); err != nil {
			continue
		}
		if !strings.EqualFold(firstString(event, "command"), "l2_render_probe_ready") {
			continue
		}
		if !strings.EqualFold(firstString(event, "feature_type"), "l2_render_probe") {
			continue
		}
		if requestID != "" && firstString(event, "request_id") != requestID {
			continue
		}
		if trackID != "" && firstString(event, "track_id") != trackID {
			continue
		}
		if clipID != "" && firstString(event, "clip_id") != clipID {
			continue
		}
		return event, nil
	}
}

type spectralFeatureCollector struct {
	TrackID           string
	ClipID            string
	FilePath          string
	SourceRevision    string
	ClipRevision      string
	RenderRevision    string
	AnalyzerRevision  string
	FeatureType       string
	ChannelsSemantics string
	ExpectedTiles     int
	TilesSeen         int
	TotalDuration     float64
	TileDuration      float64
	FrameDuration     float64
	LastTileIndex     int
	TileContentStart  float64
	FrameWidth        int
	FrequencyBins     int
	SharedMemory      string
	FloatCount        int
	ShmBytes          int
	QualityStatus     string
	QualityReason     string
	NonzeroCount      int64
	SumAbs            float64
	MaxAbs            float64
	NanInfCount       int64
	QualityFailures   []string
	FirstReceivedAt   string
	LastReceivedAt    string
	tileIndexes       map[int]bool
	tileDescriptors   map[int]spectralTileDescriptor
	unindexedTiles    []spectralTileDescriptor
}

type spectralTileDescriptor struct {
	TrackID           string
	ClipID            string
	FilePath          string
	QualityStatus     string
	QualityReason     string
	TileIndex         int
	ExpectedTiles     int
	TileDuration      float64
	TileContentStart  float64
	TotalDuration     float64
	FrameDuration     float64
	FrameWidth        int
	FrequencyBins     int
	SharedMemory      string
	FloatCount        int
	ShmBytes          int
	ChannelsSemantics string
}

func (c *spectralFeatureCollector) AddEvent(event map[string]any) {
	if c == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if c.FirstReceivedAt == "" {
		c.FirstReceivedAt = now
	}
	c.LastReceivedAt = now
	c.TrackID = firstNonEmpty(c.TrackID, firstString(event, "track_id", "source_track_id"))
	c.ClipID = firstNonEmpty(c.ClipID, firstString(event, "clip_id"))
	c.FilePath = firstNonEmpty(c.FilePath, firstString(event, "file_path"))
	c.SourceRevision = firstNonEmpty(c.SourceRevision, firstString(event, "source_revision", "source_fingerprint"))
	c.ClipRevision = firstNonEmpty(c.ClipRevision, firstString(event, "clip_revision"))
	c.RenderRevision = firstNonEmpty(c.RenderRevision, firstString(event, "render_revision"))
	c.AnalyzerRevision = firstNonEmpty(c.AnalyzerRevision, firstString(event, "analyzer_revision", "analysis_version"))
	c.FeatureType = firstNonEmpty(c.FeatureType, firstString(event, "feature_type"), "spectral_field")
	c.ChannelsSemantics = firstNonEmpty(firstString(event, "channels_semantics"), c.ChannelsSemantics, spectralTileChannelsSemantics)
	c.SharedMemory = firstNonEmpty(firstString(event, "shared_memory"), c.SharedMemory)
	eventQuality := firstString(event, "quality_status")
	eventReason := firstString(event, "quality_reason")
	c.QualityStatus = combineKernelQualityStatus(c.QualityStatus, eventQuality)
	c.QualityReason = combineKernelQualityReason(c.QualityReason, eventReason)
	if !kernelQualityReadyOrUnknown(eventQuality) {
		c.QualityFailures = append(c.QualityFailures, firstNonEmpty(eventReason, eventQuality))
	}
	c.NonzeroCount += int64(numberFromAny(event["nonzero_count"]))
	c.SumAbs += numberFromAny(event["sum_abs"])
	if maxAbs := numberFromAny(event["max_abs"]); maxAbs > c.MaxAbs {
		c.MaxAbs = maxAbs
	}
	c.NanInfCount += int64(numberFromAny(event["nan_inf_count"]))
	if expected := int(numberFromAny(event["tile_count"])); expected > 0 {
		c.ExpectedTiles = expected
	}
	if duration := numberFromAny(event["total_duration"]); duration > 0 {
		c.TotalDuration = duration
	}
	if tileDuration := numberFromAny(event["tile_duration"]); tileDuration > 0 {
		c.TileDuration = tileDuration
	}
	if frameDuration := numberFromAny(event["frame_duration_seconds"]); frameDuration > 0 {
		c.FrameDuration = frameDuration
	}
	if tileStart := numberFromAny(event["tile_content_start_seconds"]); tileStart >= 0 {
		c.TileContentStart = tileStart
	}
	if frameWidth := int(numberFromAny(firstPresentValue(event, "resolution_frame_width", "resolution_frame_count", "frame_count"))); frameWidth > 0 {
		c.FrameWidth = frameWidth
	}
	if bins := int(numberFromAny(firstPresentValue(event, "resolution_frequency_bins", "frequency_bins", "bin_count"))); bins > 0 {
		c.FrequencyBins = bins
	}
	if floatCount := int(numberFromAny(firstPresentValue(event, "float_count"))); floatCount > 0 {
		c.FloatCount = floatCount
	}
	if shmBytes := int(numberFromAny(firstPresentValue(event, "shm_bytes", "byte_count"))); shmBytes > 0 {
		c.ShmBytes = shmBytes
		if c.FloatCount <= 0 && shmBytes%4 == 0 {
			c.FloatCount = shmBytes / 4
		}
	}
	tileIndex := int(numberFromAny(event["tile_index"]))
	if c.tileIndexes == nil {
		c.tileIndexes = map[int]bool{}
	}
	c.recordTileDescriptor(event, tileIndex)
	if tileIndex >= 0 {
		c.LastTileIndex = tileIndex
		if !c.tileIndexes[tileIndex] {
			c.tileIndexes[tileIndex] = true
			c.TilesSeen++
		}
		return
	}
	c.TilesSeen++
}

const (
	spectralTileChannelStride     = 4
	spectralTileMinHz             = 20.0
	spectralTileMaxHz             = 20000.0
	spectralTileDefaultFrameSecs  = 0.01
	spectralTileChannelsSemantics = "r=left_energy,g=right_energy,b=cross_spectrum_phase_delta,a=phase_display_weight"
)

func combineKernelQualityStatus(current, next string) string {
	current = strings.ToLower(strings.TrimSpace(current))
	next = strings.ToLower(strings.TrimSpace(next))
	if next == "" {
		return current
	}
	if current == "" || kernelQualityRank(next) > kernelQualityRank(current) {
		return next
	}
	return current
}

func combineKernelQualityReason(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	switch {
	case current == "":
		return next
	case next == "":
		return current
	case strings.Contains(current, next):
		return current
	default:
		return current + "; " + next
	}
}

func kernelQualityRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "invalid":
		return 3
	case "suspect", "partial":
		return 2
	case "ready", "ok":
		return 1
	default:
		return 0
	}
}

func kernelQualityReadyOrUnknown(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "ready", "ok":
		return true
	default:
		return false
	}
}

func statusFromKernelQuality(coverageStatus, qualityStatus string) string {
	switch strings.ToLower(strings.TrimSpace(qualityStatus)) {
	case "", "ready", "ok":
		return coverageStatus
	case "failed", "error", "invalid":
		return "failed"
	default:
		return "partial"
	}
}

func stampKernelEventIdentityAndQuality(row map[string]any, sourceRevision, clipRevision, renderRevision, analyzerRevision, qualityStatus, qualityReason string, nonzeroCount int64, sumAbs, maxAbs float64, nanInfCount int64) {
	if len(row) == 0 {
		return
	}
	if strings.TrimSpace(sourceRevision) != "" {
		row["source_revision"] = strings.TrimSpace(sourceRevision)
		row["source_fingerprint"] = strings.TrimSpace(sourceRevision)
	}
	if strings.TrimSpace(clipRevision) != "" {
		row["clip_revision"] = strings.TrimSpace(clipRevision)
	}
	if strings.TrimSpace(renderRevision) != "" {
		row["render_revision"] = strings.TrimSpace(renderRevision)
	}
	if strings.TrimSpace(analyzerRevision) != "" {
		row["analyzer_revision"] = strings.TrimSpace(analyzerRevision)
	}
	if strings.TrimSpace(qualityStatus) != "" {
		row["quality_status"] = strings.TrimSpace(qualityStatus)
	}
	if strings.TrimSpace(qualityReason) != "" {
		row["quality_reason"] = strings.TrimSpace(qualityReason)
	}
	if nonzeroCount > 0 {
		row["nonzero_count"] = nonzeroCount
	}
	if sumAbs > 0 {
		row["sum_abs"] = round6(sumAbs)
	}
	if maxAbs > 0 {
		row["max_abs"] = round6(maxAbs)
	}
	if nanInfCount > 0 {
		row["nan_inf_count"] = nanInfCount
	}
}

func (c *spectralFeatureCollector) recordTileDescriptor(event map[string]any, tileIndex int) {
	if c == nil {
		return
	}
	desc := spectralTileDescriptor{
		TrackID:           firstNonEmpty(firstString(event, "track_id", "source_track_id"), c.TrackID),
		ClipID:            firstNonEmpty(firstString(event, "clip_id"), c.ClipID),
		FilePath:          firstNonEmpty(firstString(event, "file_path"), c.FilePath),
		QualityStatus:     firstString(event, "quality_status"),
		QualityReason:     firstString(event, "quality_reason"),
		TileIndex:         tileIndex,
		ExpectedTiles:     firstPositiveIntValue(int(numberFromAny(event["tile_count"])), c.ExpectedTiles),
		TileDuration:      firstPositiveFloatValue(numberFromAny(event["tile_duration"]), c.TileDuration),
		TileContentStart:  firstNonNegativeFloatValue(numberFromAny(event["tile_content_start_seconds"]), c.TileContentStart),
		TotalDuration:     firstPositiveFloatValue(numberFromAny(event["total_duration"]), c.TotalDuration),
		FrameDuration:     firstPositiveFloatValue(numberFromAny(event["frame_duration_seconds"]), c.FrameDuration, spectralTileDefaultFrameSecs),
		FrameWidth:        firstPositiveIntValue(int(numberFromAny(firstPresentValue(event, "resolution_frame_width", "resolution_frame_count", "frame_count"))), c.FrameWidth),
		FrequencyBins:     firstPositiveIntValue(int(numberFromAny(firstPresentValue(event, "resolution_frequency_bins", "frequency_bins", "bin_count"))), c.FrequencyBins),
		SharedMemory:      firstNonEmpty(firstString(event, "shared_memory"), c.SharedMemory),
		FloatCount:        firstPositiveIntValue(int(numberFromAny(firstPresentValue(event, "float_count"))), c.FloatCount),
		ShmBytes:          firstPositiveIntValue(int(numberFromAny(firstPresentValue(event, "shm_bytes", "byte_count"))), c.ShmBytes),
		ChannelsSemantics: firstNonEmpty(firstString(event, "channels_semantics"), c.ChannelsSemantics, spectralTileChannelsSemantics),
	}
	if desc.FloatCount <= 0 && desc.ShmBytes > 0 && desc.ShmBytes%4 == 0 {
		desc.FloatCount = desc.ShmBytes / 4
	}
	if desc.FloatCount <= 0 && desc.FrameWidth > 0 && desc.FrequencyBins > 0 {
		desc.FloatCount = desc.FrameWidth * desc.FrequencyBins * spectralTileChannelStride
	}
	if desc.FrameWidth <= 0 && desc.FrequencyBins <= 0 && desc.SharedMemory == "" {
		return
	}
	if tileIndex >= 0 {
		if c.tileDescriptors == nil {
			c.tileDescriptors = map[int]spectralTileDescriptor{}
		}
		c.tileDescriptors[tileIndex] = desc
		return
	}
	c.unindexedTiles = append(c.unindexedTiles, desc)
}

func (c *spectralFeatureCollector) TileDescriptors() []spectralTileDescriptor {
	if c == nil {
		return nil
	}
	out := make([]spectralTileDescriptor, 0, len(c.tileDescriptors)+len(c.unindexedTiles))
	for _, desc := range c.tileDescriptors {
		out = append(out, desc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TileIndex < out[j].TileIndex
	})
	out = append(out, c.unindexedTiles...)
	if len(out) == 0 && c.SharedMemory != "" {
		out = append(out, spectralTileDescriptor{
			TrackID:           c.TrackID,
			ClipID:            c.ClipID,
			FilePath:          c.FilePath,
			TileIndex:         c.LastTileIndex,
			ExpectedTiles:     c.ExpectedTiles,
			TileDuration:      c.TileDuration,
			TileContentStart:  c.TileContentStart,
			TotalDuration:     c.TotalDuration,
			FrameDuration:     firstPositiveFloatValue(c.FrameDuration, spectralTileDefaultFrameSecs),
			FrameWidth:        c.FrameWidth,
			FrequencyBins:     c.FrequencyBins,
			SharedMemory:      c.SharedMemory,
			QualityStatus:     c.QualityStatus,
			QualityReason:     c.QualityReason,
			FloatCount:        firstPositiveIntValue(c.FloatCount, c.FrameWidth*c.FrequencyBins*spectralTileChannelStride),
			ShmBytes:          c.ShmBytes,
			ChannelsSemantics: firstNonEmpty(c.ChannelsSemantics, spectralTileChannelsSemantics),
		})
	}
	return out
}

func firstPositiveIntValue(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveFloatValue(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonNegativeFloatValue(values ...float64) float64 {
	for _, value := range values {
		if value >= 0 {
			return value
		}
	}
	return 0
}

func (c *spectralFeatureCollector) TileCount() int {
	if c == nil {
		return 0
	}
	return c.TilesSeen
}

func (c *spectralFeatureCollector) Complete() bool {
	if c == nil || c.TilesSeen <= 0 {
		return false
	}
	return c.ExpectedTiles > 0 && c.TilesSeen >= c.ExpectedTiles
}

func (c *spectralFeatureCollector) SnapshotRow(requestID string) map[string]any {
	if c == nil || c.TilesSeen <= 0 {
		return nil
	}
	status := "partial"
	if c.ExpectedTiles > 0 && c.TilesSeen >= c.ExpectedTiles {
		status = "ready"
	}
	status = statusFromKernelQuality(status, c.QualityStatus)
	target := map[string]any{
		"track_id": c.TrackID,
		"clip_id":  c.ClipID,
	}
	row := map[string]any{
		"status":                     status,
		"feature_type":               firstNonEmpty(c.FeatureType, "spectral_field"),
		"track_id":                   c.TrackID,
		"clip_id":                    c.ClipID,
		"target":                     target,
		"file_path":                  c.FilePath,
		"request_id":                 requestID,
		"source":                     "kernel_tile_ready_direct_collector",
		"tile_count_seen":            c.TilesSeen,
		"tile_count_expected":        c.ExpectedTiles,
		"last_tile_index":            c.LastTileIndex,
		"tile_duration":              round3(c.TileDuration),
		"tile_content_start_seconds": round3(c.TileContentStart),
		"total_duration":             round3(c.TotalDuration),
		"resolution_frame_width":     c.FrameWidth,
		"resolution_frequency_bins":  c.FrequencyBins,
		"channels_semantics":         firstNonEmpty(c.ChannelsSemantics, spectralTileChannelsSemantics),
		"updated_at":                 firstNonEmpty(c.LastReceivedAt, time.Now().UTC().Format(time.RFC3339Nano)),
	}
	stampKernelEventIdentityAndQuality(row, c.SourceRevision, c.ClipRevision, c.RenderRevision, c.AnalyzerRevision, c.QualityStatus, c.QualityReason, c.NonzeroCount, c.SumAbs, c.MaxAbs, c.NanInfCount)
	if len(c.QualityFailures) > 0 {
		row["quality_failures"] = c.QualityFailures
	}
	if status != "ready" && c.QualityReason != "" {
		row["reason"] = c.QualityReason
	}
	if c.SharedMemory != "" {
		row["shared_memory"] = c.SharedMemory
	}
	if c.FloatCount > 0 {
		row["float_count"] = c.FloatCount
	}
	if c.ShmBytes > 0 {
		row["shm_bytes"] = c.ShmBytes
	}
	if c.FrameDuration > 0 {
		row["frame_duration_seconds"] = c.FrameDuration
	}
	if c.FirstReceivedAt != "" {
		row["first_received_at"] = c.FirstReceivedAt
	}
	return row
}

func (c *spectralFeatureCollector) StatusRow(trackID, clipID string) map[string]any {
	row := map[string]any{
		"feature_type": "spectral_field",
		"track_id":     firstNonEmpty(c.TrackID, trackID),
		"clip_id":      firstNonEmpty(c.ClipID, clipID),
		"tile_count":   0,
		"status":       "no_tile_ready_received",
		"reason":       "no_spectral_tile_ready_received",
	}
	if c != nil {
		row["tile_count"] = c.TileCount()
		if c.TileCount() > 0 {
			row["status"] = "tile_ready_observed"
			delete(row, "reason")
		}
		if c.ExpectedTiles > 0 {
			row["tile_count_expected"] = c.ExpectedTiles
		}
		if c.FrameWidth > 0 {
			row["resolution_frame_width"] = c.FrameWidth
		}
		if c.FrequencyBins > 0 {
			row["resolution_frequency_bins"] = c.FrequencyBins
		}
		if c.TileDuration > 0 {
			row["tile_duration"] = round3(c.TileDuration)
		}
		if c.FirstReceivedAt != "" {
			row["first_received_at"] = c.FirstReceivedAt
		}
		if c.LastReceivedAt != "" {
			row["last_received_at"] = c.LastReceivedAt
		}
	}
	return row
}

func collectSpectralFeatureTiles(ctx context.Context, subURL, trackID, clipID string, timeout time.Duration, afterSubscribe func() error) (*spectralFeatureCollector, error) {
	if strings.TrimSpace(subURL) == "" {
		subURL = mixboardFeatureSubURL
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return nil, err
	}
	if err := sub.Dial(subURL); err != nil {
		return nil, err
	}
	collector := &spectralFeatureCollector{TrackID: trackID, ClipID: clipID, FeatureType: "spectral_field", LastTileIndex: -1}
	if afterSubscribe != nil {
		warmMixboardFeatureSubscriber(opCtx)
		if err := afterSubscribe(); err != nil {
			return collector, err
		}
	}
	for {
		select {
		case <-opCtx.Done():
			return collector, nil
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return collector, nil
			}
			return collector, err
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(zmqMsgPayload(msg)), &event); err != nil {
			continue
		}
		if !strings.EqualFold(firstString(event, "command"), "tile_ready") {
			continue
		}
		if !strings.EqualFold(firstNonEmpty(firstString(event, "feature_type"), "spectral_field"), "spectral_field") {
			continue
		}
		if trackID != "" && firstString(event, "track_id", "source_track_id") != trackID {
			continue
		}
		if clipID != "" && firstString(event, "clip_id") != "" && firstString(event, "clip_id") != clipID {
			continue
		}
		collector.AddEvent(event)
		if collector.Complete() {
			return collector, nil
		}
	}
}

func recordSpectralCollectorStatus(packet map[string]any, collector *spectralFeatureCollector, trackID, clipID string) {
	if packet == nil {
		return
	}
	status := collector.StatusRow(trackID, clipID)
	packet["spectral_direct_collector"] = status
	rows := mapRowsFromAny(packet["spectral_direct_collectors"])
	rows = append(rows, status)
	packet["spectral_direct_collectors"] = rows
	packet["spectral_tile_ready_seen"] = boolValueDefault(packet["spectral_tile_ready_seen"], false) || (collector != nil && collector.TileCount() > 0)
	if collector != nil && collector.TileCount() > 0 {
		packet["spectral_tile_summary_source"] = "visual_tile_only_no_l3_derivation"
	}
}

func zmqMsgPayload(msg zmq4.Msg) string {
	if len(msg.Frames) == 0 {
		return ""
	}
	return string(msg.Frames[len(msg.Frames)-1])
}

func expectedWaveformTileCount(duration float64) int {
	if duration <= 0 {
		return 0
	}
	return int(math.Ceil(duration / 5.0))
}

func dbfsValue(value float64) any {
	if value <= 0 {
		return nil
	}
	return round3(20 * math.Log10(value))
}

func headroomDBValue(peak float64) any {
	if peak <= 0 {
		return nil
	}
	return round3(-20 * math.Log10(peak))
}

func crestDBValue(peak, rms float64) any {
	if peak <= 0 || rms <= 0 {
		return nil
	}
	return round3(20 * math.Log10(peak/rms))
}

func waveformEnergyState(rms float64) string {
	switch {
	case rms <= 0:
		return "silent"
	case rms < 0.02:
		return "low"
	case rms < 0.12:
		return "medium"
	default:
		return "high"
	}
}

func round6(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}

var (
	modkernel32       = syscall.NewLazyDLL("kernel32.dll")
	procOpenFileMapA  = modkernel32.NewProc("OpenFileMappingA")
	procMapViewOfFile = modkernel32.NewProc("MapViewOfFile")
	procUnmapView     = modkernel32.NewProc("UnmapViewOfFile")
	procCloseHandle   = modkernel32.NewProc("CloseHandle")
)

const fileMapRead = 0x0004

func readSharedFloat32Array(memoryName string, floatCount int) ([]float32, error) {
	memoryName = strings.TrimSpace(memoryName)
	if memoryName == "" || floatCount <= 0 {
		return nil, fmt.Errorf("shared memory name and float_count are required")
	}
	namePtr, err := syscall.BytePtrFromString(memoryName)
	if err != nil {
		return nil, err
	}
	handle, _, err := procOpenFileMapA.Call(uintptr(fileMapRead), uintptr(0), uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return nil, firstSyscallError(err, "OpenFileMappingA failed")
	}
	defer procCloseHandle.Call(handle)
	byteCount := uintptr(floatCount * 4)
	view, _, err := procMapViewOfFile.Call(handle, uintptr(fileMapRead), 0, 0, byteCount)
	if view == 0 {
		return nil, firstSyscallError(err, "MapViewOfFile failed")
	}
	defer procUnmapView.Call(view)
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(view)), int(byteCount))
	out := make([]float32, floatCount)
	for i := range out {
		bits := binary.LittleEndian.Uint32(bytes[i*4 : i*4+4])
		out[i] = math.Float32frombits(bits)
	}
	return out, nil
}

func firstSyscallError(err error, fallback string) error {
	if err != nil && !errors.Is(err, syscall.Errno(0)) {
		return err
	}
	return fmt.Errorf("%s", fallback)
}

func writeMixboardReadyWaveformSnapshot(cmd map[string]any, packet map[string]any, waveform map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	waveform = stampMixboardFeatureRowIdentity(waveform, packet, nil)
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	existing["waveform_envelope"] = waveform
	existing["track_waveform_envelopes"] = mergeTrackWaveformRows(existing["track_waveform_envelopes"], []map[string]any{waveform})
	if _, ok := existing["spectrogram_tiles"]; !ok {
		existing["spectrogram_tiles"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["band_energy_summary"]; !ok {
		existing["band_energy_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["stereo_relation_summary"]; !ok {
		existing["stereo_relation_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["loudness_summary"]; !ok {
		existing["loudness_summary"] = map[string]any{"status": "missing"}
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func writeMixboardReadyProjectTrackWaveformSnapshot(cmd map[string]any, packet map[string]any, waveform map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	waveform = stampMixboardFeatureRowIdentity(waveform, packet, nil)
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	existing["track_waveform_envelopes"] = mergeTrackWaveformRows(existing["track_waveform_envelopes"], []map[string]any{waveform})
	if projectWaveformMatchesResolvedTarget(packet, waveform) {
		existing["waveform_envelope"] = waveform
	} else if _, ok := existing["waveform_envelope"]; !ok {
		existing["waveform_envelope"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["spectrogram_tiles"]; !ok {
		existing["spectrogram_tiles"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["band_energy_summary"]; !ok {
		existing["band_energy_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["stereo_relation_summary"]; !ok {
		existing["stereo_relation_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["loudness_summary"]; !ok {
		existing["loudness_summary"] = map[string]any{"status": "missing"}
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func writeMixboardReadySpectralSnapshot(cmd map[string]any, packet map[string]any, spectral map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	spectral = stampMixboardFeatureRowIdentity(spectral, packet, nil)
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	existing["spectrogram_tiles"] = spectral
	existing["spectrogram_tile_rows"] = mergeMixboardFeatureRows(existing["spectrogram_tile_rows"], []map[string]any{spectral})
	if _, ok := existing["waveform_envelope"]; !ok {
		existing["waveform_envelope"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["track_waveform_envelopes"]; !ok {
		existing["track_waveform_envelopes"] = []any{}
	}
	if _, ok := existing["band_energy_summary"]; !ok {
		existing["band_energy_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["stereo_relation_summary"]; !ok {
		existing["stereo_relation_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["loudness_summary"]; !ok {
		existing["loudness_summary"] = map[string]any{"status": "missing"}
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func writeMixboardReadyL3SummarySnapshot(cmd map[string]any, packet map[string]any, row map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	row = stampMixboardFeatureRowIdentity(row, packet, nil)
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	switch strings.ToLower(firstString(row, "feature_type")) {
	case "band_energy_summary":
		if shouldUseL3BridgeSummary(existing["band_energy_summary"], packet, row) {
			existing["band_energy_summary"] = row
		}
		existing["band_energy_summaries"] = mergeMixboardFeatureRows(existing["band_energy_summaries"], []map[string]any{row})
	case "stereo_relation_summary":
		if shouldUseL3BridgeSummary(existing["stereo_relation_summary"], packet, row) {
			existing["stereo_relation_summary"] = row
		}
		existing["stereo_relation_summaries"] = mergeMixboardFeatureRows(existing["stereo_relation_summaries"], []map[string]any{row})
	case "loudness_summary":
		if shouldUseL3BridgeSummary(existing["loudness_summary"], packet, row) {
			existing["loudness_summary"] = row
		}
		existing["loudness_summaries"] = mergeMixboardFeatureRows(existing["loudness_summaries"], []map[string]any{row})
	}
	if _, ok := existing["waveform_envelope"]; !ok {
		existing["waveform_envelope"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["track_waveform_envelopes"]; !ok {
		existing["track_waveform_envelopes"] = []any{}
	}
	if _, ok := existing["spectrogram_tiles"]; !ok {
		existing["spectrogram_tiles"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["band_energy_summary"]; !ok {
		existing["band_energy_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["stereo_relation_summary"]; !ok {
		existing["stereo_relation_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["loudness_summary"]; !ok {
		existing["loudness_summary"] = map[string]any{"status": "missing"}
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func writeMixboardReadyL2RenderProbeSnapshot(cmd map[string]any, packet map[string]any, row map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	row = stampMixboardFeatureRowIdentity(row, packet, nil)
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	existing["l2_render_probe"] = row
	existing["l2_render_probes"] = mergeMixboardFeatureRows(existing["l2_render_probes"], []map[string]any{row})
	if _, ok := existing["waveform_envelope"]; !ok {
		existing["waveform_envelope"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["spectrogram_tiles"]; !ok {
		existing["spectrogram_tiles"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["band_energy_summary"]; !ok {
		existing["band_energy_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["stereo_relation_summary"]; !ok {
		existing["stereo_relation_summary"] = map[string]any{"status": "missing"}
	}
	if _, ok := existing["loudness_summary"]; !ok {
		existing["loudness_summary"] = map[string]any{"status": "missing"}
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func shouldUseL3BridgeSummary(existing any, packet map[string]any, row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	if mixboardFeatureRowNeedsMissingReasonForPacket(existing, packet) {
		return true
	}
	existingRow, _ := existing.(map[string]any)
	if len(existingRow) == 0 {
		return true
	}
	if !mixboardBridgeFeatureRowFreshForPacket(existingRow, packet) && mixboardBridgeFeatureRowFreshForPacket(row, packet) {
		return true
	}
	rowSource := strings.ToLower(firstString(row, "source"))
	existingSource := strings.ToLower(firstString(existingRow, "source"))
	if strings.Contains(rowSource, "kernel_l3_offline_analyzer") && !strings.Contains(existingSource, "kernel_l3_offline_analyzer") {
		return true
	}
	if featureRowPriority(row) > featureRowPriority(existingRow) {
		return true
	}
	return l3BridgeSummaryScore(row) > l3BridgeSummaryScore(existingRow)+0.001
}

func l3BridgeSummaryScore(row map[string]any) float64 {
	if len(row) == 0 {
		return 0
	}
	seen := firstPositiveNumber(row, "analyzed_sample_count", "sample_count", "frame_count")
	expected := firstPositiveNumber(row, "expected_sample_count")
	coverage := firstPositiveNumber(row, "coverage_ratio", "coverage_seconds", "duration_seconds", "total_duration")
	score := coverage
	if expected > 0 && seen >= expected {
		score += 10000
	} else if expected > 0 && seen > 0 {
		score += (seen / expected) * 1000
	}
	score += seen * 10
	if len(mapAnyFromAny(row["bands"])) > 0 {
		score += 100
	}
	if row["correlation_estimate"] != nil || row["balance_db"] != nil || row["left_level_db"] != nil || row["right_level_db"] != nil {
		score += 100
	}
	if row["integrated_lufs"] != nil || row["approximate_lufs"] != nil || row["rms_dbfs"] != nil || row["peak_dbfs"] != nil {
		score += 100
	}
	if strings.Contains(strings.ToLower(firstString(row, "source")), "kernel_l3_offline_analyzer") {
		score += 50
	}
	if strings.EqualFold(firstString(row, "quality_status"), "ready") || strings.EqualFold(firstString(row, "quality_reason"), "ok") {
		score += 10
	}
	if strings.EqualFold(firstString(row, "status"), "ready") {
		score += 5
	}
	return score
}

func writeMixboardProjectSpectralSnapshotFromCollectors(cmd map[string]any, packet map[string]any, targets []map[string]any) {
	rows := mapRowsFromAny(packet["spectral_direct_collectors"])
	if len(rows) == 0 {
		return
	}
	seen := 0
	expected := 0
	var frameWidth int
	var frequencyBins int
	var tileDuration float64
	var firstReceivedAt string
	var lastReceivedAt string
	for _, row := range rows {
		tileSeen := int(numberFromAny(firstPresentValue(row, "tile_count_seen", "tile_count")))
		if tileSeen <= 0 {
			continue
		}
		seen += tileSeen
		if count := int(numberFromAny(row["tile_count_expected"])); count > 0 {
			expected += count
		}
		if frameWidth == 0 {
			frameWidth = int(numberFromAny(row["resolution_frame_width"]))
		}
		if frequencyBins == 0 {
			frequencyBins = int(numberFromAny(row["resolution_frequency_bins"]))
		}
		if tileDuration == 0 {
			tileDuration = numberFromAny(row["tile_duration"])
		}
		if receivedAt := firstString(row, "first_received_at"); receivedAt != "" && (firstReceivedAt == "" || receivedAt < firstReceivedAt) {
			firstReceivedAt = receivedAt
		}
		if receivedAt := firstString(row, "last_received_at"); receivedAt != "" && receivedAt > lastReceivedAt {
			lastReceivedAt = receivedAt
		}
	}
	if seen <= 0 {
		return
	}
	status := "partial"
	if expected > 0 && seen >= expected {
		status = "ready"
	}
	trackIDs := make([]string, 0, len(targets))
	seenTrackIDs := map[string]bool{}
	for _, target := range targets {
		trackID := firstString(target, "track_id")
		if trackID == "" || seenTrackIDs[trackID] {
			continue
		}
		seenTrackIDs[trackID] = true
		trackIDs = append(trackIDs, trackID)
	}
	row := map[string]any{
		"status":              status,
		"feature_type":        "spectral_field",
		"request_id":          firstString(packet, "request_id"),
		"source":              "kernel_tile_ready_direct_collector",
		"scope":               "full_project",
		"target_count":        len(targets),
		"tile_count_seen":     seen,
		"tile_count_expected": expected,
		"updated_at":          time.Now().UTC().Format(time.RFC3339Nano),
	}
	if len(trackIDs) > 0 {
		row["track_ids"] = trackIDs
	}
	if frameWidth > 0 {
		row["resolution_frame_width"] = frameWidth
	}
	if frequencyBins > 0 {
		row["resolution_frequency_bins"] = frequencyBins
	}
	if tileDuration > 0 {
		row["tile_duration"] = round3(tileDuration)
	}
	if firstReceivedAt != "" {
		row["first_received_at"] = firstReceivedAt
	}
	if lastReceivedAt != "" {
		row["last_received_at"] = lastReceivedAt
	}
	writeMixboardReadySpectralSnapshot(cmd, packet, row)
}

func projectWaveformMatchesResolvedTarget(packet map[string]any, waveform map[string]any) bool {
	target, _ := packet["resolved_target"].(map[string]any)
	trackID := firstString(target, "track_id")
	clipID := firstString(target, "clip_id")
	if trackID == "" || clipID == "" {
		return false
	}
	return firstString(waveform, "track_id") == trackID && firstString(waveform, "clip_id") == clipID
}

func resolveMixObservationTargetContext(cmd map[string]any, state map[string]any, target mixboard.TargetRef) map[string]any {
	resolved := map[string]any{}
	trackID := strings.TrimSpace(firstString(cmd, "track_id", "target_track_id", "selected_track_id"))
	clipID := strings.TrimSpace(firstString(cmd, "clip_id", "selected_clip_id", "source_clip_id", "target_clip_id"))
	targetID := strings.TrimSpace(target.ID)
	targetKind := strings.ToLower(strings.TrimSpace(target.Kind))
	if trackID == "" {
		trackID = strings.TrimSpace(firstString(targetToMap(target), "track_id"))
	}
	if clipID == "" {
		clipID = strings.TrimSpace(firstString(targetToMap(target), "clip_id"))
	}
	if trackID == "" && targetID != "" && (targetKind == "track" || targetKind == "selection" || targetKind == "") {
		trackID = targetID
	}
	if clipID == "" && targetID != "" && targetKind == "clip" {
		clipID = targetID
	}
	tracks := visibleTrackRows(state)
	if trackID != "" {
		if canonical, ok := resolveVisibleTrackAlias(trackID, tracks); ok {
			trackID = canonical
		}
	}
	if trackID != "" {
		for _, row := range tracks {
			if visibleTrackID(row) != trackID {
				continue
			}
			resolved["track_id"] = trackID
			if name := visibleTrackName(row); name != "" {
				resolved["track_name"] = name
			}
			clips := mapRowsFromAny(row["clips"])
			if clipID == "" && len(clips) > 0 {
				if selected := strings.TrimSpace(firstString(cmd, "selected_clip_id")); selected != "" {
					clipID = selected
				}
				if clipID == "" {
					clipID = firstString(clips[0], "clip_id", "id", "item_id")
				}
			}
			if clipID != "" {
				for _, clip := range clips {
					if firstString(clip, "clip_id", "id", "item_id") == clipID {
						resolved["clip_id"] = clipID
						if name := firstString(clip, "name", "clip_name"); name != "" {
							resolved["clip_name"] = name
						}
						if path := firstNonEmpty(firstString(clip, "file_path", "source_path", "current_source_path"), firstString(row, "file_path", "source_path", "current_source_path")); path != "" {
							resolved["file_path"] = path
						}
						if length := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration"); length > 0 {
							resolved["duration_seconds"] = length
						}
						resolved["source"] = "selected_track_first_clip"
						return resolved
					}
				}
			}
			if len(clips) == 1 {
				clip := clips[0]
				if found := firstString(clip, "clip_id", "id", "item_id"); found != "" {
					resolved["clip_id"] = found
				}
				if name := firstString(clip, "name", "clip_name"); name != "" {
					resolved["clip_name"] = name
				}
				if path := firstNonEmpty(firstString(clip, "file_path", "source_path", "current_source_path"), firstString(row, "file_path", "source_path", "current_source_path")); path != "" {
					resolved["file_path"] = path
				}
				if length := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration"); length > 0 {
					resolved["duration_seconds"] = length
				}
				resolved["source"] = "selected_track_only_clip"
				return resolved
			}
			if path := firstString(row, "file_path", "source_path", "current_source_path"); path != "" {
				resolved["file_path"] = path
				resolved["source"] = "track_file_path"
			}
			return resolved
		}
	}
	if clipID != "" {
		for _, row := range tracks {
			for _, clip := range mapRowsFromAny(row["clips"]) {
				if firstString(clip, "clip_id", "id", "item_id") != clipID {
					continue
				}
				resolved["track_id"] = visibleTrackID(row)
				resolved["track_name"] = visibleTrackName(row)
				resolved["clip_id"] = clipID
				if name := firstString(clip, "name", "clip_name"); name != "" {
					resolved["clip_name"] = name
				}
				if path := firstNonEmpty(firstString(clip, "file_path", "source_path", "current_source_path"), firstString(row, "file_path", "source_path", "current_source_path")); path != "" {
					resolved["file_path"] = path
				}
				if length := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration"); length > 0 {
					resolved["duration_seconds"] = length
				}
				resolved["source"] = "clip_lookup"
				return resolved
			}
		}
	}
	if len(tracks) == 1 {
		row := tracks[0]
		resolved["track_id"] = visibleTrackID(row)
		resolved["track_name"] = visibleTrackName(row)
		clips := mapRowsFromAny(row["clips"])
		if len(clips) == 1 {
			clip := clips[0]
			if found := firstString(clip, "clip_id", "id", "item_id"); found != "" {
				resolved["clip_id"] = found
			}
			if name := firstString(clip, "name", "clip_name"); name != "" {
				resolved["clip_name"] = name
			}
			if path := firstNonEmpty(firstString(clip, "file_path", "source_path", "current_source_path"), firstString(row, "file_path", "source_path", "current_source_path")); path != "" {
				resolved["file_path"] = path
			}
			if length := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration"); length > 0 {
				resolved["duration_seconds"] = length
			}
			resolved["source"] = "single_visible_track_single_clip"
			return resolved
		}
	}
	return resolved
}

func looksSyntheticTrackAlias(ref string) bool {
	ref = strings.ToLower(strings.TrimSpace(ref))
	if ref == "" {
		return false
	}
	if regexp.MustCompile(`^track[_ -]?\d+$`).MatchString(ref) {
		return true
	}
	return false
}

func resolveVisibleTrackAlias(ref string, tracks []map[string]any) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	for _, row := range tracks {
		if id := visibleTrackID(row); id != "" && id == ref {
			return id, true
		}
	}
	for _, row := range tracks {
		id := visibleTrackID(row)
		if id == "" {
			continue
		}
		if strings.EqualFold(visibleTrackName(row), ref) {
			return id, true
		}
		if aliases := visibleTrackAliases(row); stringSliceContainsFold(aliases, ref) {
			return id, true
		}
	}
	return "", false
}

func visibleTrackAliases(row map[string]any) []string {
	id := visibleTrackID(row)
	name := visibleTrackName(row)
	out := []string{id, name}
	if idx, ok := firstPositiveInt(row, "user_track_index", "track_index", "index"); ok {
		out = append(out,
			fmt.Sprintf("track_%d", idx),
			fmt.Sprintf("Track %d", idx),
			fmt.Sprintf("track %d", idx),
			fmt.Sprintf("%d", idx),
		)
	}
	if n, ok := trailingPositiveInt(name); ok {
		out = append(out,
			fmt.Sprintf("track_%d", n),
			fmt.Sprintf("Track %d", n),
			fmt.Sprintf("track %d", n),
		)
	}
	return out
}

func trailingPositiveInt(text string) (int, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, false
	}
	matches := regexp.MustCompile(`(\d+)\s*$`).FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(matches[1])
	return n, err == nil && n > 0
}

func stringSliceContainsFold(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}

func targetToMap(target mixboard.TargetRef) map[string]any {
	return map[string]any{
		"kind":  target.Kind,
		"id":    target.ID,
		"label": target.Label,
	}
}

func mixObservationAcousticDigest(obs mixboard.ObservationPacket, featureRequest map[string]any, resolvedContext map[string]any) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(obs.Status) != "" {
		out["status"] = obs.Status
	}
	if target := obs.TargetRef; strings.TrimSpace(target.ID) != "" {
		out["target"] = map[string]any{
			"kind":  target.Kind,
			"id":    target.ID,
			"label": target.Label,
		}
	}
	if trackName := firstString(resolvedContext, "track_name"); trackName != "" {
		out["track_name"] = trackName
		out["user_label"] = trackName
		if target, _ := out["target"].(map[string]any); len(target) > 0 {
			target["track_name"] = trackName
			target["user_label"] = trackName
		}
	}
	if clipID := firstString(resolvedContext, "clip_id"); clipID != "" {
		out["clip_id"] = clipID
	}
	if clipName := firstString(resolvedContext, "clip_name"); clipName != "" {
		out["clip_name"] = clipName
	}
	if path := firstString(resolvedContext, "file_path"); path != "" {
		out["file_path"] = path
	}
	if duration := numberFromAny(resolvedContext["duration_seconds"]); duration > 0 {
		out["duration_seconds"] = round3(duration)
	}
	if global := obs.GlobalSummary; len(global) > 0 {
		for _, key := range []string{"peak_dbfs", "rms_dbfs", "crest_db", "dominant_problem_tags"} {
			if value, ok := global[key]; ok && !isEmptyValue(value) {
				out[key] = value
			}
		}
	}
	if mixPkg, ok := obs.MixPackage["current_metrics"].(map[string]any); ok && len(mixPkg) > 0 {
		if waveform, ok := mixPkg["waveform"].(map[string]any); ok {
			out["waveform"] = map[string]any{
				"status":      waveform["status"],
				"rms":         waveform["rms"],
				"peak_abs":    waveform["peak_abs"],
				"peak_dbfs":   waveform["peak_dbfs"],
				"rms_dbfs":    waveform["rms_dbfs"],
				"crest_db":    waveform["crest_db"],
				"headroom_db": waveform["headroom_db"],
			}
		}
		if timeEnergy, ok := mixPkg["time_energy"].([]map[string]any); ok && len(timeEnergy) > 0 {
			out["time_energy"] = capHarnessRows(timeEnergy, 8)
		} else if rows := mapRowsFromAny(mixPkg["time_energy"]); len(rows) > 0 {
			out["time_energy"] = capHarnessRows(rows, 8)
		}
		if bandEnergy, ok := mixPkg["band_energy"].(map[string]any); ok {
			out["band_energy_status"] = bandEnergy["status"]
			if bands, ok := bandEnergy["bands"].(map[string]any); ok && len(bands) > 0 {
				out["band_energy"] = map[string]any{
					"low_mid":  numberOrText(bands["low_mid"], "energy_db"),
					"mid":      numberOrText(bands["mid"], "energy_db"),
					"presence": numberOrText(bands["presence"], "energy_db"),
				}
			}
		}
		if stereo, ok := mixPkg["stereo_relation"].(map[string]any); ok {
			out["stereo_relation_status"] = stereo["status"]
			for _, key := range []string{"balance_db", "correlation_estimate", "balance_state", "correlation_state"} {
				if value, ok := stereo[key]; ok && !isEmptyValue(value) {
					out[key] = value
				}
			}
		}
	}
	if featureRequestStatus := firstString(featureRequest, "status"); featureRequestStatus != "" {
		out["feature_request_status"] = featureRequestStatus
	}
	if reason := firstString(featureRequest, "reason"); reason != "" {
		out["feature_request_reason"] = reason
	}
	if missing := stringSliceFromAny(obs.MixPackage["missing_metrics"]); len(missing) > 0 {
		out["missing_metrics"] = missing
	}
	if notes := obs.Notes; len(notes) > 0 {
		out["notes"] = notes
	}
	if len(obs.AcousticPackageStatus) > 0 {
		out["acoustic_package_status"] = compactAcousticPackageStatusForDigest(obs.AcousticPackageStatus)
	}
	return out
}

func compactAcousticPackageStatusForDigest(status map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"schema_version", "status", "project_id", "track_id", "clip_id", "source_revision", "updated_at"} {
		if value, ok := status[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	layers := mapAnyFromAny(status["package_layers"])
	layerOut := map[string]any{}
	for _, layerName := range []string{"l1_static", "l2_realtime", "l3_deep"} {
		layer := mapAnyFromAny(layers[layerName])
		if len(layer) == 0 {
			continue
		}
		compactLayer := map[string]any{"status": firstString(layer, "status")}
		features := mapAnyFromAny(layer["features"])
		featureOut := map[string]any{}
		for featureName, raw := range features {
			feature := mapAnyFromAny(raw)
			if len(feature) == 0 {
				continue
			}
			row := map[string]any{"status": firstString(feature, "status")}
			if reason := firstString(feature, "reason"); reason != "" {
				row["reason"] = reason
			}
			if progress := mapAnyFromAny(feature["progress"]); len(progress) > 0 {
				row["progress"] = compactSelectedAny(progress, []string{"tile_count_seen", "tile_count_expected", "coverage_seconds", "coverage_ratio", "updated_at", "reason"})
			}
			featureOut[featureName] = row
		}
		compactLayer["features"] = featureOut
		layerOut[layerName] = compactLayer
	}
	if len(layerOut) > 0 {
		out["package_layers"] = layerOut
	}
	return out
}

func compactSelectedAny(row map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	return out
}

func numberOrText(value any, key string) any {
	row, _ := value.(map[string]any)
	if len(row) == 0 {
		return nil
	}
	if n, ok := numberValueFromMap(row, key); ok {
		return round3(n)
	}
	if text := firstString(row, key); text != "" {
		return text
	}
	return nil
}

func capHarnessRows(rows []map[string]any, max int) []map[string]any {
	if max <= 0 || len(rows) <= max {
		return rows
	}
	return rows[:max]
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func mixboardFeatureReadyWait() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_FEATURE_READY_WAIT_MS"))
	if raw == "" {
		return mixboardFeatureReadyWaitDefault
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return mixboardFeatureReadyWaitDefault
	}
	return time.Duration(ms) * time.Millisecond
}

func mixboardFeatureCollectWait() time.Duration {
	wait := mixboardFeatureReadyWait()
	if wait < 120*time.Millisecond {
		return 120 * time.Millisecond
	}
	return wait
}

func mixboardFeatureBackgroundCollectWait() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_BACKGROUND_FEATURE_WAIT_MS"))
	if raw != "" {
		ms, err := strconv.Atoi(raw)
		if err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	wait := mixboardFeatureCollectWait()
	if wait < 30*time.Second {
		return 30 * time.Second
	}
	return wait
}

func mixboardL2RenderProbeCollectWait() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VIT_L2_RENDER_PROBE_WAIT_MS"))
	if raw != "" {
		ms, err := strconv.Atoi(raw)
		if err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 12 * time.Second
}

func mixboardFeatureBackgroundSubscribeWait() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_BACKGROUND_SUBSCRIBE_WAIT_MS"))
	if raw != "" {
		ms, err := strconv.Atoi(raw)
		if err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	wait := mixboardFeatureSubscriberWarmupDefault + 150*time.Millisecond
	if collectWait := mixboardFeatureCollectWait(); collectWait > 0 && collectWait < wait {
		return collectWait
	}
	return wait
}

func mixboardFeatureBackgroundSendWait() time.Duration {
	raw := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_BACKGROUND_SEND_WAIT_MS"))
	if raw != "" {
		ms, err := strconv.Atoi(raw)
		if err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 2 * time.Second
}

func warmMixboardFeatureSubscriber(ctx context.Context) {
	warmup := mixboardFeatureSubscriberWarmupDefault
	if wait := mixboardFeatureCollectWait(); wait > 0 && wait < warmup {
		warmup = wait
	}
	if warmup <= 0 {
		return
	}
	timer := time.NewTimer(warmup)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func newMixboardFeatureRequestPacket(cmd map[string]any, target mixboard.TargetRef) map[string]any {
	requestID := firstNonEmpty(firstString(cmd, "mixboard_request_id", "request_id"), "mixboard_"+time.Now().UTC().Format("20060102T150405.000000000"))
	return map[string]any{
		"schema_version":     "mixboard_feature_request.v1",
		"request_id":         requestID,
		"project_id":         firstNonEmpty(firstString(cmd, "project_id"), "current"),
		"session_id":         firstString(cmd, "mix_session_id", "session_id"),
		"status":             "blocked",
		"target_ref":         map[string]any{"kind": target.Kind, "id": target.ID, "label": target.Label, "source": target.Source, "confidence": target.Confidence},
		"resolved_target":    map[string]any{},
		"requested_features": []any{},
		"skipped_features":   []any{},
		"created_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
}

func mixObservationWantsProjectAcoustics(cmd map[string]any) bool {
	switch normalizeMixObservationScope(firstString(cmd, "scope", "observation_scope")) {
	case "full_project", "full_project_with_focus_track", "track_group":
		return true
	default:
		return false
	}
}

func mixObservationProjectBackgroundFillNeeded(cmd map[string]any, featureSnapshot map[string]any, status acousticpackage.Status) bool {
	if !mixObservationWantsProjectAcoustics(cmd) {
		return false
	}
	if acousticPackageL3CoreReady(status) {
		return false
	}
	if acousticPackageL3CoreBuilding(status) {
		return false
	}
	latest, _ := featureSnapshot["latest_request"].(map[string]any)
	if len(latest) == 0 {
		return true
	}
	if !requestedFeatureIncludesFeature(latest, "waveform_envelope") || !requestedFeatureIncludesFeature(latest, "spectral_field") || !requestedFeatureIncludesFeature(latest, "l3_acoustic_summary") {
		return true
	}
	if !strings.EqualFold(firstString(latest, "status"), "requested") {
		return true
	}
	currentRequestID := firstNonEmpty(firstString(cmd, "mixboard_request_id", "request_id"), firstString(cmd, "request_id"))
	latestRequestID := firstString(latest, "request_id")
	return currentRequestID != "" && latestRequestID != currentRequestID
}

func acousticPackageL3CoreReady(status acousticpackage.Status) bool {
	layer := status.PackageLayers["l3_deep"]
	if len(layer.Features) == 0 {
		return false
	}
	for _, featureName := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		if !strings.EqualFold(layer.Features[featureName].Status, acousticpackage.StatusReady) {
			return false
		}
	}
	return true
}

func acousticPackageL3CoreBuilding(status acousticpackage.Status) bool {
	layer := status.PackageLayers["l3_deep"]
	if len(layer.Features) == 0 {
		return false
	}
	for _, featureName := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		switch strings.ToLower(layer.Features[featureName].Status) {
		case acousticpackage.StatusBuilding, "requested", "pending":
			return true
		}
	}
	return false
}

func visibleAudioTrackFeatureTargets(state map[string]any) []map[string]any {
	rows := visibleTrackRows(state)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 || !trackLooksAudio(row) {
			continue
		}
		trackID := visibleTrackID(row)
		if trackID == "" {
			continue
		}
		clip := primaryVisibleAudioClip(row)
		target := map[string]any{
			"track_id":   trackID,
			"track_name": visibleTrackName(row),
		}
		if len(clip) > 0 {
			if clipID := firstString(clip, "clip_id", "id", "item_id"); clipID != "" {
				target["clip_id"] = clipID
			}
			if name := firstString(clip, "name", "clip_name"); name != "" {
				target["clip_name"] = name
			}
			if path := firstNonEmpty(firstString(clip, "file_path", "source_path", "current_source_path"), firstString(row, "file_path", "source_path", "current_source_path")); path != "" {
				target["file_path"] = path
			}
			if duration := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration"); duration > 0 {
				target["duration_seconds"] = round3(duration)
			}
			target["source"] = "visible_audio_track_primary_clip"
		} else {
			target["source"] = "visible_audio_track_without_clip"
		}
		out = append(out, target)
	}
	return out
}

func trackLooksAudio(row map[string]any) bool {
	if value, ok := boolValue(row["is_audio_track"]); ok {
		return value
	}
	trackType := strings.ToLower(firstString(row, "track_type", "type", "kind"))
	if strings.Contains(trackType, "midi") {
		return false
	}
	if strings.Contains(trackType, "audio") || strings.Contains(trackType, "hybrid") {
		return true
	}
	return len(mapRowsFromAny(firstPresentValue(row, "clips", "clip_summaries"))) > 0
}

func primaryVisibleAudioClip(row map[string]any) map[string]any {
	clips := mapRowsFromAny(firstPresentValue(row, "clips", "clip_summaries"))
	var best map[string]any
	bestLength := -1.0
	for _, clip := range clips {
		if len(clip) == 0 {
			continue
		}
		length := firstPositiveNumber(clip, "length_seconds", "duration_seconds", "duration")
		if best == nil || length > bestLength {
			best = clip
			bestLength = length
		}
	}
	return best
}

func safeRequestIDPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "target"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "target"
	}
	if len(out) > 48 {
		return out[:48]
	}
	return out
}

func resolveMixboardFeatureTarget(cmd map[string]any, state map[string]any, target mixboard.TargetRef) map[string]any {
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	id := strings.TrimSpace(target.ID)
	trackID := ""
	clipID := ""
	if kind == "clip" && id != "" {
		clipID = id
	}
	if kind == "track" && id != "" {
		trackID = id
	}
	if clipID == "" {
		clipID = firstString(cmd, "selected_clip_id", "clip_id")
	}
	if clipID == "" {
		if ids := stringSliceFromAny(cmd["selected_clip_ids"]); len(ids) > 0 {
			clipID = ids[0]
		}
	}
	if trackID == "" {
		trackID = firstString(cmd, "selected_clip_track_id", "track_id", "selected_track_id")
	}
	if trackID == "" && id != "" && (kind == "track" || kind == "selection" || kind == "") {
		trackID = id
	}
	if clipID == "" && id != "" && kind == "clip" {
		clipID = id
	}
	refs := visibleClipRefs(state)
	if trackID != "" {
		if canonical, ok := resolveVisibleTrackAlias(trackID, visibleTrackRows(state)); ok {
			trackID = canonical
			if kind == "track" {
				id = canonical
			}
		}
	}
	if clipID != "" {
		for _, ref := range refs {
			if ref.ID == clipID {
				if trackID == "" {
					trackID = ref.TrackID
				}
				return map[string]any{"kind": "clip", "id": clipID, "track_id": trackID, "clip_id": clipID, "source": "mixboard_context"}
			}
		}
	}
	if trackID != "" {
		for _, ref := range refs {
			if ref.TrackID == trackID {
				return map[string]any{"kind": "track", "id": trackID, "track_id": trackID, "clip_id": ref.ID, "source": "first_audio_clip_on_track"}
			}
		}
	}
	return map[string]any{"kind": kind, "id": id, "track_id": trackID, "clip_id": clipID, "source": "mixboard_context"}
}

func writeMixboardFeatureRequestSnapshot(cmd map[string]any, packet map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	targetRows := trackWaveformRowsForPacket(packet, existing)
	snapshot := map[string]any{
		"schema_version":            "mixboard_feature_snapshot.v1",
		"updated_at":                time.Now().UTC().Format(time.RFC3339Nano),
		"latest_request":            packet,
		"waveform_envelope":         pendingMixboardFeatureRow("waveform_envelope", packet, existing),
		"track_waveform_envelopes":  targetRows,
		"spectrogram_tiles":         pendingMixboardFeatureRow("spectral_field", packet, existing),
		"spectrogram_tile_rows":     mergeMixboardFeatureRows(existing["spectrogram_tile_rows"], []map[string]any{pendingMixboardFeatureRow("spectral_field", packet, existing)}),
		"band_energy_summary":       reusableMixboardBridgeFeatureRow("band_energy_summary", existing, packet, map[string]any{"status": "missing"}),
		"band_energy_summaries":     reusableMixboardBridgeFeatureRows("band_energy_summaries", existing, packet),
		"stereo_relation_summary":   reusableMixboardBridgeFeatureRow("stereo_relation_summary", existing, packet, map[string]any{"status": "missing"}),
		"stereo_relation_summaries": reusableMixboardBridgeFeatureRows("stereo_relation_summaries", existing, packet),
		"loudness_summary":          reusableMixboardBridgeFeatureRow("loudness_summary", existing, packet, map[string]any{"status": "missing"}),
		"loudness_summaries":        reusableMixboardBridgeFeatureRows("loudness_summaries", existing, packet),
	}
	writeMixboardFeatureSnapshotFile(path, snapshot, packet)
}

func finalizeMixboardFeatureSnapshotAfterWait(cmd map[string]any, packet map[string]any, targets []map[string]any) {
	mixboardFeatureSnapshotWriteMu.Lock()
	defer mixboardFeatureSnapshotWriteMu.Unlock()
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	if len(existing) == 0 {
		existing = map[string]any{"schema_version": "mixboard_feature_snapshot.v1"}
	}
	existing["schema_version"] = "mixboard_feature_snapshot.v1"
	existing["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	existing["latest_request"] = packet
	if mixboardFeatureRowNeedsMissingReasonForPacket(existing["spectrogram_tiles"], packet) {
		existing["spectrogram_tiles"] = missingMixboardAcousticRow("spectral_field", packet, targets, acousticBridgeMissingReason(packet, "spectral_field", "spectral_field_not_received_before_observation_timeout"))
	}
	if mixboardFeatureRowNeedsMissingReasonForPacket(existing["band_energy_summary"], packet) {
		existing["band_energy_summary"] = missingMixboardAcousticRow("band_energy_summary", packet, targets, acousticBridgeMissingReason(packet, "band_energy_summary", "band_energy_summary_not_received_before_observation_timeout"))
	}
	if mixboardFeatureRowNeedsMissingReasonForPacket(existing["stereo_relation_summary"], packet) {
		existing["stereo_relation_summary"] = missingMixboardAcousticRow("stereo_relation_summary", packet, targets, acousticBridgeMissingReason(packet, "stereo_relation_summary", "stereo_relation_summary_not_received_before_observation_timeout"))
	}
	if mixboardFeatureRowNeedsMissingReasonForPacket(existing["loudness_summary"], packet) {
		existing["loudness_summary"] = missingMixboardAcousticRow("loudness_summary", packet, targets, acousticBridgeMissingReason(packet, "loudness_summary", "loudness_summary_not_received_before_observation_timeout"))
	}
	writeMixboardFeatureSnapshotFile(path, existing, packet)
}

func writeMixboardFeatureSnapshotFile(path string, snapshot map[string]any, packet map[string]any) {
	promoteMixboardBridgeRowsForPacket(snapshot, packet)
	normalizeMixboardBridgeRowsForPacket(snapshot, packet)
	data, err := json.MarshalIndent(snapshot, "", "\t")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}

func normalizeMixboardBridgeRowsForPacket(snapshot map[string]any, packet map[string]any) {
	if len(snapshot) == 0 || len(packet) == 0 {
		return
	}
	for key, featureType := range map[string]string{
		"spectrogram_tiles":       "spectral_field",
		"band_energy_summary":     "band_energy_summary",
		"stereo_relation_summary": "stereo_relation_summary",
		"loudness_summary":        "loudness_summary",
	} {
		row, _ := snapshot[key].(map[string]any)
		if len(row) == 0 {
			continue
		}
		snapshot[key] = normalizeMixboardBridgeRowForPacket(row, packet, featureType)
	}
}

func normalizeMixboardBridgeRowForPacket(row map[string]any, packet map[string]any, featureType string) map[string]any {
	requestID := firstString(packet, "request_id")
	if requestID == "" || len(row) == 0 {
		return row
	}
	target := mapAnyFromAny(packet["resolved_target"])
	status := strings.ToLower(strings.TrimSpace(firstString(row, "status")))
	if status == "" {
		status = "missing"
	}
	if status == "missing" &&
		firstString(row, "reason") == "" &&
		firstString(row, "request_id") == "" &&
		firstString(row, "track_id") == "" &&
		firstString(row, "clip_id") == "" {
		out := cloneAnyMap(row)
		out["status"] = status
		out["feature_type"] = featureType
		return out
	}
	if (status == "ready" || status == "partial") && mixboardFeatureRowHasMaterialIdentity(row) && sameFeatureTarget(row, target) {
		out := cloneAnyMap(row)
		out["status"] = status
		out["feature_type"] = featureType
		rowRequestID := firstString(out, "request_id")
		if rowRequestID == "" || materialBridgeRequestIDBelongsToLatest(rowRequestID, requestID, target) {
			out["request_id"] = requestID
		}
		stampBridgeRowTarget(out, target)
		return out
	}
	if mixboardBridgeFeatureRowFreshForPacket(row, packet) || sameFeatureTarget(row, target) || (len(target) == 0 && len(row) > 0) {
		out := cloneAnyMap(row)
		out["status"] = status
		out["feature_type"] = featureType
		out["request_id"] = requestID
		stampBridgeRowTarget(out, target)
		if status == "missing" && firstString(out, "reason") == "" {
			out["reason"] = "stale_feature_snapshot_for_current_request"
		}
		return out
	}
	return missingMixboardAcousticRow(featureType, packet, nil, "stale_feature_snapshot_for_current_request")
}

func stampBridgeRowTarget(row map[string]any, target map[string]any) {
	if len(row) == 0 || len(target) == 0 {
		return
	}
	if trackID := firstString(target, "track_id"); trackID != "" {
		row["track_id"] = trackID
	}
	if clipID := firstString(target, "clip_id"); clipID != "" {
		row["clip_id"] = clipID
	}
}

func materialBridgeRequestIDBelongsToLatest(rowRequestID string, latestRequestID string, target map[string]any) bool {
	rowRequestID = strings.TrimSpace(rowRequestID)
	latestRequestID = strings.TrimSpace(latestRequestID)
	if rowRequestID == "" || latestRequestID == "" || rowRequestID == latestRequestID {
		return false
	}
	if !strings.HasPrefix(rowRequestID, "kernel_prepared_") || !strings.HasPrefix(latestRequestID, "kernel_prepared_") {
		return false
	}
	suffix := safeKernelPreparedRequestIDSuffix(target)
	return suffix != "" && strings.HasSuffix(rowRequestID, "_"+suffix) && strings.HasSuffix(latestRequestID, "_"+suffix)
}

func safeKernelPreparedRequestIDSuffix(target map[string]any) string {
	for _, value := range []string{firstString(target, "clip_id"), firstString(target, "track_id")} {
		value = safeRequestIDPart(value)
		if value != "" && value != "target" {
			return value
		}
	}
	return ""
}

func mixboardFeatureRowNeedsMissingReason(value any) bool {
	row, _ := value.(map[string]any)
	if len(row) == 0 {
		return true
	}
	switch strings.ToLower(firstString(row, "status")) {
	case "ready", "partial", "blocked", "unavailable", "invalid":
		return false
	case "missing":
		return firstString(row, "reason") == ""
	default:
		return true
	}
}

func mixboardFeatureRowNeedsMissingReasonForPacket(value any, packet map[string]any) bool {
	row, _ := value.(map[string]any)
	if len(row) == 0 {
		return true
	}
	switch strings.ToLower(firstString(row, "status")) {
	case "ready", "partial":
		return !mixboardBridgeFeatureRowFreshForPacket(row, packet)
	case "blocked", "unavailable", "invalid":
		return false
	case "missing":
		return firstString(row, "reason") == "" || !mixboardBridgeFeatureRowFreshForPacket(row, packet)
	default:
		return true
	}
}

func missingMixboardAcousticRow(featureType string, packet map[string]any, targets []map[string]any, reason string) map[string]any {
	out := map[string]any{
		"status":       "missing",
		"feature_type": featureType,
		"request_id":   firstString(packet, "request_id"),
		"reason":       reason,
		"updated_at":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	target, _ := packet["resolved_target"].(map[string]any)
	if len(target) == 0 && len(targets) == 1 {
		target = targets[0]
	}
	if trackID := firstString(target, "track_id"); trackID != "" {
		out["track_id"] = trackID
	}
	if clipID := firstString(target, "clip_id"); clipID != "" {
		out["clip_id"] = clipID
	}
	if len(targets) > 1 {
		out["scope"] = "full_project"
		out["target_count"] = len(targets)
		trackIDs := make([]string, 0, len(targets))
		for _, target := range targets {
			if trackID := firstString(target, "track_id"); trackID != "" {
				trackIDs = append(trackIDs, trackID)
			}
		}
		if len(trackIDs) > 0 {
			out["track_ids"] = trackIDs
		}
	}
	return out
}

func acousticBridgeMissingReason(packet map[string]any, featureType, fallback string) string {
	if reason := skippedFeatureReasonForFeature(packet, featureType, "", ""); reason != "" {
		return reason
	}
	if featureType == "spectral_field" {
		if !boolValueDefault(packet["spectral_tile_ready_seen"], false) && requestedFeatureIncludesFeature(packet, "spectral_field") {
			return "no_spectral_tile_ready_received"
		}
		return fallback
	}
	if isL3AcousticSummaryFeature(featureType) {
		if reason := skippedFeatureReasonForFeature(packet, "l3_acoustic_summary", "", ""); reason != "" {
			return "l3_acoustic_summary_request_failed: " + reason
		}
		if !requestedFeatureIncludesFeature(packet, "l3_acoustic_summary") {
			return "l3_acoustic_summary_not_requested_for_downstream_summary"
		}
		return strings.TrimSuffix(featureType, "_summary") + "_summary_not_received_before_observation_timeout"
	}
	return fallback
}

func readMixboardFeatureSnapshotFile(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func waitForMixboardFeatureSnapshotReady(ctx context.Context, cmd map[string]any, trackID, clipID string, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	path := mixboard.FeatureSnapshotPath(cmd)
	deadline := time.Now().Add(timeout)
	for {
		if mixboardFeatureRowsReadyForTarget(readMixboardFeatureSnapshotFile(path), trackID, clipID) {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(120 * time.Millisecond):
		}
	}
}

func waitForMixboardAcousticBridgeSnapshot(ctx context.Context, cmd map[string]any, packet map[string]any, timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	path := mixboard.FeatureSnapshotPath(cmd)
	deadline := time.Now().Add(timeout)
	for {
		if mixboardAcousticBridgeSnapshotReady(readMixboardFeatureSnapshotFile(path), packet) {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(120 * time.Millisecond):
		}
	}
}

func mixboardAcousticBridgeSnapshotReady(snapshot map[string]any, packet map[string]any) bool {
	if len(snapshot) == 0 {
		return false
	}
	for _, key := range []string{"spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		row, _ := snapshot[key].(map[string]any)
		switch strings.ToLower(firstString(row, "status")) {
		case "ready", "partial":
			if mixboardBridgeFeatureRowFreshForPacket(row, packet) {
				return true
			}
		}
	}
	return false
}

func mixboardFeatureRowsReadyForTarget(snapshot map[string]any, trackID, clipID string) bool {
	if len(snapshot) == 0 {
		return false
	}
	if mixboardFeatureRowReadyForTarget(snapshot["waveform_envelope"], trackID, clipID) {
		return true
	}
	for _, row := range mapRowsFromAny(snapshot["track_waveform_envelopes"]) {
		if mixboardFeatureRowReadyForTarget(row, trackID, clipID) {
			return true
		}
	}
	return false
}

func mixboardFeatureRowReadyForTarget(value any, trackID, clipID string) bool {
	row, _ := value.(map[string]any)
	if len(row) == 0 || !strings.EqualFold(firstString(row, "status"), "ready") {
		return false
	}
	if trackID != "" && firstString(row, "track_id") != "" && firstString(row, "track_id") != trackID {
		return false
	}
	if clipID != "" && firstString(row, "clip_id") != "" && firstString(row, "clip_id") != clipID {
		return false
	}
	return true
}

func pendingMixboardFeatureRow(featureType string, packet map[string]any, existing map[string]any) map[string]any {
	target, _ := packet["resolved_target"].(map[string]any)
	if row := reusableReadyMixboardFeatureRow(featureType, target, existing); len(row) > 0 {
		return row
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(packet["status"])))
	rowStatus := "requested"
	if status == "blocked" {
		rowStatus = "blocked"
	}
	found := false
	for _, raw := range anyListFromAny(packet["requested_features"]) {
		item, _ := raw.(map[string]any)
		if strings.EqualFold(firstString(item, "feature_type"), featureType) || (featureType == "spectral_field" && strings.EqualFold(firstString(item, "feature_type"), "spectral_field")) {
			found = true
			break
		}
	}
	if status != "blocked" && !found {
		rowStatus = "missing"
	}
	out := map[string]any{
		"status":     rowStatus,
		"track_id":   firstString(target, "track_id"),
		"clip_id":    firstString(target, "clip_id"),
		"request_id": firstString(packet, "request_id"),
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if reason := firstString(packet, "reason"); reason != "" {
		out["reason"] = reason
	}
	return out
}

func reusableReadyMixboardFeatureRow(featureType string, target map[string]any, existing map[string]any) map[string]any {
	if len(existing) == 0 {
		return nil
	}
	key := "waveform_envelope"
	if featureType == "spectral_field" {
		key = "spectrogram_tiles"
	}
	row, _ := existing[key].(map[string]any)
	if len(row) == 0 || !strings.EqualFold(firstString(row, "status"), "ready") {
		return nil
	}
	if featureType == "spectral_field" {
		return nil
	}
	targetTrack := firstString(target, "track_id")
	targetClip := firstString(target, "clip_id")
	if targetTrack != "" && firstString(row, "track_id") != "" && firstString(row, "track_id") != targetTrack {
		return nil
	}
	if targetClip != "" && firstString(row, "clip_id") != "" && firstString(row, "clip_id") != targetClip {
		return nil
	}
	return cloneAnyMap(row)
}

func waitForMixboardProjectFeatureSnapshotReady(ctx context.Context, cmd map[string]any, targets []map[string]any, timeout time.Duration) {
	if timeout <= 0 || len(targets) == 0 {
		return
	}
	path := mixboard.FeatureSnapshotPath(cmd)
	deadline := time.Now().Add(timeout)
	for {
		if mixboardProjectFeatureRowsReady(readMixboardFeatureSnapshotFile(path), targets) {
			return
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(120 * time.Millisecond):
		}
	}
}

func mixboardProjectFeatureRowsReady(snapshot map[string]any, targets []map[string]any) bool {
	if len(snapshot) == 0 || len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		trackID := firstString(target, "track_id")
		clipID := firstString(target, "clip_id")
		if trackID == "" || clipID == "" {
			continue
		}
		if !mixboardFeatureRowsReadyForTarget(snapshot, trackID, clipID) {
			return false
		}
	}
	return true
}

func trackWaveformRowsForPacket(packet map[string]any, existing map[string]any) []map[string]any {
	targets := mapRowsFromAny(packet["track_feature_targets"])
	if len(targets) == 0 {
		return reusableTrackWaveformRows(existing)
	}
	rows := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, pendingTrackWaveformRow(packet, target, existing))
	}
	return mergeTrackWaveformRows(existing["track_waveform_envelopes"], rows)
}

func pendingTrackWaveformRow(packet, target, existing map[string]any) map[string]any {
	if row := reusableReadyTrackWaveformRow(target, existing); len(row) > 0 {
		return row
	}
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(packet["status"])))
	rowStatus := "requested"
	if status == "blocked" {
		rowStatus = "blocked"
	}
	trackID := firstString(target, "track_id")
	clipID := firstString(target, "clip_id")
	if status != "blocked" && !requestedFeatureIncludesTarget(packet, trackID, clipID) {
		rowStatus = "missing"
	}
	if clipID == "" || skippedFeatureReason(packet, trackID, clipID) != "" {
		rowStatus = "blocked"
	}
	row := map[string]any{
		"status":     rowStatus,
		"track_id":   trackID,
		"track_name": firstString(target, "track_name"),
		"clip_id":    clipID,
		"request_id": firstString(packet, "request_id"),
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
	}
	if clipName := firstString(target, "clip_name"); clipName != "" {
		row["clip_name"] = clipName
	}
	if filePath := firstString(target, "file_path"); filePath != "" {
		row["file_path"] = filePath
	}
	row = stampMixboardFeatureRowIdentity(row, packet, target)
	if reason := firstString(packet, "reason"); reason != "" {
		row["reason"] = reason
	} else if reason := skippedFeatureReason(packet, trackID, clipID); reason != "" {
		row["reason"] = reason
	} else if clipID == "" {
		row["reason"] = "primary_audio_clip_missing"
	}
	return row
}

func reusableReadyTrackWaveformRow(target map[string]any, existing map[string]any) map[string]any {
	for _, row := range mapRowsFromAny(existing["track_waveform_envelopes"]) {
		if !strings.EqualFold(firstString(row, "status"), "ready") {
			continue
		}
		if sameFeatureTarget(row, target) {
			return cloneAnyMap(row)
		}
	}
	if row, _ := existing["waveform_envelope"].(map[string]any); len(row) > 0 && strings.EqualFold(firstString(row, "status"), "ready") && sameFeatureTarget(row, target) {
		return cloneAnyMap(row)
	}
	return nil
}

func reusableTrackWaveformRows(existing map[string]any) []map[string]any {
	rows := mapRowsFromAny(existing["track_waveform_envelopes"])
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneAnyMap(row))
	}
	return out
}

func mergeTrackWaveformRows(existing any, updates []map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	seen := map[string]int{}
	for _, row := range mapRowsFromAny(existing) {
		if len(row) == 0 {
			continue
		}
		key := featureTargetKey(row)
		if key == "" {
			key = fmt.Sprintf("existing_%d", len(rows))
		}
		seen[key] = len(rows)
		rows = append(rows, cloneAnyMap(row))
	}
	for _, update := range updates {
		if len(update) == 0 {
			continue
		}
		key := featureTargetKey(update)
		if key == "" {
			key = fmt.Sprintf("update_%d", len(rows))
		}
		if idx, ok := seen[key]; ok {
			if sameFeatureRequest(rows[idx], update) || featureRowPriority(update) >= featureRowPriority(rows[idx]) {
				rows[idx] = cloneAnyMap(update)
			}
			continue
		}
		seen[key] = len(rows)
		rows = append(rows, cloneAnyMap(update))
	}
	return rows
}

func mergeMixboardFeatureRows(existing any, updates []map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	seen := map[string]int{}
	for _, row := range mapRowsFromAny(existing) {
		if len(row) == 0 {
			continue
		}
		key := featureTargetKey(row)
		if key == "" {
			key = fmt.Sprintf("existing_%d", len(rows))
		}
		seen[key] = len(rows)
		rows = append(rows, cloneAnyMap(row))
	}
	for _, update := range updates {
		if len(update) == 0 {
			continue
		}
		key := featureTargetKey(update)
		if key == "" {
			key = fmt.Sprintf("update_%d", len(rows))
		}
		if idx, ok := seen[key]; ok {
			if sameFeatureRequest(rows[idx], update) || featureRowPriority(update) >= featureRowPriority(rows[idx]) {
				rows[idx] = cloneAnyMap(update)
			}
			continue
		}
		seen[key] = len(rows)
		rows = append(rows, cloneAnyMap(update))
	}
	return rows
}

func sameFeatureRequest(a, b map[string]any) bool {
	requestA := firstString(a, "request_id")
	requestB := firstString(b, "request_id")
	return requestA != "" && requestA == requestB
}

func requestedFeatureIncludesTarget(packet map[string]any, trackID, clipID string) bool {
	for _, raw := range anyListFromAny(packet["requested_features"]) {
		item, _ := raw.(map[string]any)
		if !strings.EqualFold(firstString(item, "feature_type"), "waveform_envelope") {
			continue
		}
		if firstString(item, "track_id") == trackID && firstString(item, "clip_id") == clipID {
			return true
		}
	}
	return false
}

func requestedFeatureIncludesFeature(packet map[string]any, featureType string) bool {
	for _, raw := range anyListFromAny(packet["requested_features"]) {
		item, _ := raw.(map[string]any)
		if strings.EqualFold(firstString(item, "feature_type"), featureType) {
			return true
		}
	}
	return false
}

func skippedFeatureReason(packet map[string]any, trackID, clipID string) string {
	return skippedFeatureReasonForFeature(packet, "waveform_envelope", trackID, clipID)
}

func skippedFeatureReasonForFeature(packet map[string]any, featureType, trackID, clipID string) string {
	for _, raw := range anyListFromAny(packet["skipped_features"]) {
		item, _ := raw.(map[string]any)
		if !strings.EqualFold(firstString(item, "feature_type"), featureType) {
			continue
		}
		if trackID != "" && firstString(item, "track_id") != trackID {
			continue
		}
		if clipID != "" && firstString(item, "clip_id") != clipID {
			continue
		}
		if trackID != "" || clipID != "" || firstString(item, "reason") != "" {
			return firstString(item, "reason")
		}
	}
	return ""
}

func sameFeatureTarget(row, target map[string]any) bool {
	trackID := firstString(target, "track_id")
	clipID := firstString(target, "clip_id")
	if trackID != "" && firstString(row, "track_id") != "" && firstString(row, "track_id") != trackID {
		return false
	}
	if clipID != "" && firstString(row, "clip_id") != "" && firstString(row, "clip_id") != clipID {
		return false
	}
	if !featureSourceIdentityMatchesTarget(row, target) {
		return false
	}
	return trackID != "" || clipID != ""
}

func featureTargetKey(row map[string]any) string {
	trackID := firstString(row, "track_id")
	clipID := firstString(row, "clip_id")
	if trackID == "" && clipID == "" {
		return ""
	}
	parts := []string{
		firstString(row, "project_id"),
		trackID,
		clipID,
		firstString(row, "clip_revision"),
		firstString(row, "source_revision"),
		firstString(row, "source_fingerprint"),
		firstString(row, "source_hash"),
		strings.ToLower(filepath.Clean(firstNonEmpty(firstString(row, "file_path", "source_path", "current_source_path")))),
	}
	return strings.Join(parts, "::")
}

func featureRowPriority(row map[string]any) int {
	switch strings.ToLower(firstString(row, "status")) {
	case "ready":
		return 4
	case "partial":
		return 3
	case "requested":
		return 2
	case "blocked", "unavailable":
		return 1
	default:
		return 0
	}
}

func reusableMixboardFeatureRow(key string, existing map[string]any, fallback map[string]any) map[string]any {
	if len(existing) > 0 {
		if row, _ := existing[key].(map[string]any); len(row) > 0 {
			return cloneAnyMap(row)
		}
	}
	return cloneAnyMap(fallback)
}

func reusableMixboardBridgeFeatureRow(key string, existing map[string]any, packet map[string]any, fallback map[string]any) map[string]any {
	if row := bestReusableMixboardBridgeFeatureRow(key, existing, packet); len(row) > 0 {
		return row
	}
	out := cloneAnyMap(fallback)
	if len(out) == 0 {
		out = map[string]any{"status": "missing"}
	}
	if requestID := firstString(packet, "request_id"); requestID != "" {
		out["request_id"] = requestID
	}
	return out
}

func promoteMixboardBridgeRowsForPacket(snapshot map[string]any, packet map[string]any) {
	if len(snapshot) == 0 || len(packet) == 0 {
		return
	}
	for _, key := range []string{"band_energy_summary", "stereo_relation_summary", "loudness_summary"} {
		if row := bestReusableMixboardBridgeFeatureRow(key, snapshot, packet); len(row) > 0 {
			snapshot[key] = row
		}
	}
}

func bestReusableMixboardBridgeFeatureRow(key string, existing map[string]any, packet map[string]any) map[string]any {
	if len(existing) == 0 || len(packet) == 0 {
		return nil
	}
	candidates := make([]map[string]any, 0, 4)
	if row, _ := existing[key].(map[string]any); len(row) > 0 {
		candidates = append(candidates, row)
	}
	if historyKey := mixboardBridgeFeatureRowsKey(key); historyKey != "" {
		candidates = append(candidates, mapRowsFromAny(existing[historyKey])...)
	}
	var best map[string]any
	for _, row := range candidates {
		if len(row) == 0 || !mixboardBridgeFeatureRowFreshForPacket(row, packet) {
			continue
		}
		if len(best) == 0 || bridgeFeatureRowBeats(row, best) {
			best = row
		}
	}
	if len(best) == 0 {
		return nil
	}
	return cloneAnyMap(best)
}

func mixboardBridgeFeatureRowsKey(key string) string {
	switch key {
	case "band_energy_summary":
		return "band_energy_summaries"
	case "stereo_relation_summary":
		return "stereo_relation_summaries"
	case "loudness_summary":
		return "loudness_summaries"
	default:
		return ""
	}
}

func bridgeFeatureRowBeats(candidate, current map[string]any) bool {
	candidatePriority := featureRowPriority(candidate)
	currentPriority := featureRowPriority(current)
	if candidatePriority != currentPriority {
		return candidatePriority > currentPriority
	}
	return l3BridgeSummaryScore(candidate) > l3BridgeSummaryScore(current)+0.001
}

func reusableMixboardBridgeFeatureRows(key string, existing map[string]any, packet map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, row := range mapRowsFromAny(existing[key]) {
		if mixboardBridgeFeatureRowFreshForPacket(row, packet) {
			rows = append(rows, cloneAnyMap(row))
		}
	}
	return rows
}

func stampMixboardFeatureRowIdentity(row map[string]any, packet map[string]any, target map[string]any) map[string]any {
	if len(row) == 0 {
		return row
	}
	out := cloneAnyMap(row)
	if len(target) == 0 {
		target = mapAnyFromAny(packet["resolved_target"])
	}
	projectID := firstNonEmpty(firstString(out, "project_id"), firstString(packet, "project_id"), "current")
	sessionID := firstNonEmpty(firstString(out, "session_id"), firstString(packet, "session_id", "mix_session_id"))
	trackID := firstNonEmpty(firstString(out, "track_id"), firstString(target, "track_id"))
	clipID := firstNonEmpty(firstString(out, "clip_id"), firstString(target, "clip_id"))
	sourcePath := firstNonEmpty(firstString(out, "file_path", "source_path", "current_source_path"), firstString(target, "file_path", "source_path", "current_source_path"))
	sourceHash := firstNonEmpty(firstString(out, "source_hash"), firstString(target, "source_hash"))
	duration := firstPositiveNumber(out, "duration_seconds", "length_seconds", "duration")
	if duration <= 0 {
		duration = firstPositiveNumber(target, "duration_seconds", "length_seconds", "duration")
	}
	out["project_id"] = projectID
	if sessionID != "" {
		out["session_id"] = sessionID
	}
	if trackID != "" {
		out["track_id"] = trackID
	}
	if clipID != "" {
		out["clip_id"] = clipID
	}
	if sourcePath != "" {
		out["file_path"] = sourcePath
		out["source_path"] = sourcePath
	}
	if sourceHash != "" {
		out["source_hash"] = sourceHash
	}
	if duration > 0 {
		out["duration_seconds"] = round3(duration)
	}
	if firstString(out, "source_revision") == "" {
		sourceRevision := acousticpackage.ComputeSourceRevision(acousticpackage.Identity{
			SourcePath:  sourcePath,
			SourceHash:  sourceHash,
			DurationSec: duration,
		})
		out["source_revision"] = sourceRevision
	}
	if firstString(out, "source_fingerprint") == "" {
		out["source_fingerprint"] = firstString(out, "source_revision")
	}
	if firstString(out, "clip_revision") == "" {
		out["clip_revision"] = acousticpackage.ComputeClipRevision(acousticpackage.Identity{
			ProjectID:         projectID,
			TrackID:           trackID,
			ClipID:            clipID,
			SourcePath:        sourcePath,
			SourceHash:        sourceHash,
			SourceRevision:    firstString(out, "source_revision"),
			SourceFingerprint: firstString(out, "source_fingerprint"),
			DurationSec:       duration,
		})
	}
	if firstString(out, "analyzer_revision") == "" {
		out["analyzer_revision"] = "agent_lightweight_o1"
	}
	return out
}

func featureSourceIdentityMatchesTarget(row, target map[string]any) bool {
	targetRevision := firstString(target, "source_revision", "source_fingerprint")
	rowRevision := firstString(row, "source_revision", "source_fingerprint")
	if targetRevision != "" && rowRevision != "" && rowRevision != targetRevision {
		return false
	}
	targetClipRevision := firstString(target, "clip_revision")
	rowClipRevision := firstString(row, "clip_revision")
	if targetClipRevision != "" && rowClipRevision != "" && rowClipRevision != targetClipRevision {
		return false
	}
	targetRenderRevision := firstString(target, "render_revision")
	rowRenderRevision := firstString(row, "render_revision")
	if harnessBridgeFeatureUsesRenderRevision(row) && targetRenderRevision != "" && rowRenderRevision != "" && rowRenderRevision != targetRenderRevision {
		return false
	}
	targetPath := firstString(target, "file_path", "source_path", "current_source_path")
	targetHash := firstString(target, "source_hash")
	targetDuration := firstPositiveNumber(target, "duration_seconds", "length_seconds", "duration")
	if targetPath == "" && targetHash == "" && targetDuration <= 0 && targetRevision == "" && targetClipRevision == "" {
		return true
	}
	rowPath := firstString(row, "file_path", "source_path", "current_source_path")
	rowHash := firstString(row, "source_hash")
	rowDuration := firstPositiveNumber(row, "duration_seconds", "length_seconds", "duration")
	if targetPath != "" {
		if rowPath == "" || strings.ToLower(filepath.Clean(rowPath)) != strings.ToLower(filepath.Clean(targetPath)) {
			return false
		}
	}
	if targetHash != "" {
		if rowHash == "" || rowHash != targetHash {
			return false
		}
	}
	if targetDuration > 0 {
		if rowDuration <= 0 || math.Abs(rowDuration-targetDuration) > 0.25 {
			return false
		}
	}
	return true
}

func harnessBridgeFeatureUsesRenderRevision(row map[string]any) bool {
	featureType := strings.ToLower(strings.TrimSpace(firstString(row, "feature_type", "type")))
	switch featureType {
	case "spectral_field", "spectrogram_tiles", "band_energy_summary", "stereo_relation_summary", "loudness_summary", "l3_acoustic_summary":
		return false
	}
	source := strings.ToLower(firstString(row, "source"))
	if strings.Contains(source, "kernel_l3_offline_analyzer") || strings.Contains(source, "spectral_tile_derived") {
		return false
	}
	return true
}

func mixboardBridgeFeatureRowFreshForPacket(row map[string]any, packet map[string]any) bool {
	if len(row) == 0 || len(packet) == 0 {
		return false
	}
	target, _ := packet["resolved_target"].(map[string]any)
	if mixboardFeatureRowHasMaterialIdentity(row) && sameFeatureTarget(row, target) {
		return true
	}
	requestID := firstString(packet, "request_id")
	rowRequestID := firstString(row, "request_id")
	if requestID != "" && rowRequestID != "" && rowRequestID != requestID {
		return false
	}
	if requestID != "" && rowRequestID == "" {
		return false
	}
	if strings.EqualFold(firstString(row, "scope"), "full_project") {
		return true
	}
	if len(target) == 0 || (firstString(target, "track_id") == "" && firstString(target, "clip_id") == "") {
		return true
	}
	if !sameFeatureTarget(row, target) {
		return false
	}
	return true
}

func mixboardFeatureRowHasMaterialIdentity(row map[string]any) bool {
	return firstString(row, "source_revision") != "" ||
		firstString(row, "source_fingerprint") != "" ||
		firstString(row, "source_hash") != "" ||
		firstString(row, "clip_revision") != "" ||
		firstString(row, "source_path", "file_path", "current_source_path") != ""
}

func anyListFromAny(value any) []any {
	switch x := value.(type) {
	case []any:
		return x
	case []map[string]any:
		out := make([]any, 0, len(x))
		for _, row := range x {
			out = append(out, row)
		}
		return out
	default:
		return nil
	}
}

func mixTargetFromCommand(cmd map[string]any) mixboard.TargetRef {
	if row, ok := cmd["target_ref"].(map[string]any); ok {
		return mixboard.TargetRef{
			Kind:       firstNonEmpty(firstString(row, "kind"), firstString(cmd, "target_kind", "kind")),
			ID:         firstNonEmpty(firstString(row, "id", "track_id", "clip_id"), firstString(cmd, "target_id", "track_id", "clip_id")),
			Label:      firstNonEmpty(firstString(row, "label", "name"), firstString(cmd, "target_label", "track_name", "clip_name")),
			Source:     firstString(row, "source"),
			Confidence: firstString(row, "confidence"),
		}
	}
	return mixboard.TargetRef{
		Kind:       firstNonEmpty(firstString(cmd, "target_kind", "kind"), "selection"),
		ID:         firstString(cmd, "target_id", "track_id", "clip_id"),
		Label:      firstString(cmd, "target_label", "track_name", "clip_name"),
		Source:     firstString(cmd, "target_source", "source"),
		Confidence: firstString(cmd, "target_confidence", "confidence"),
	}
}

func mixObjectsFromCommand(cmd map[string]any) []mixboard.MixObject {
	rows, ok := cmd["mix_objects"].([]any)
	if !ok {
		return nil
	}
	out := make([]mixboard.MixObject, 0, len(rows))
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		obj := mixboard.MixObject{
			Mode:        firstString(row, "mode"),
			Kind:        firstString(row, "kind", "type"),
			ID:          firstString(row, "id"),
			Label:       firstString(row, "label", "name"),
			EffectScope: firstString(row, "effect_scope"),
			Source:      firstString(row, "source"),
		}
		if obj.Kind != "" || obj.ID != "" || obj.Label != "" {
			out = append(out, obj)
		}
	}
	return out
}

func mixListenScopeFromCommand(cmd map[string]any) mixboard.ListenScope {
	row, ok := cmd["listen_scope"].(map[string]any)
	if !ok {
		return mixboard.ListenScope{}
	}
	timeRow, _ := row["time"].(map[string]any)
	sourceRow, _ := row["source"].(map[string]any)
	return mixboard.ListenScope{
		Time: mixboard.ListenTimeScope{
			Mode:         firstString(timeRow, "mode"),
			StartSeconds: firstPositiveNumber(timeRow, "start_seconds", "start"),
			EndSeconds:   firstPositiveNumber(timeRow, "end_seconds", "end"),
			Locked:       boolFromAny(timeRow["locked"]),
			Source:       firstString(timeRow, "source"),
		},
		Source: mixboard.ListenSourceScope{
			Mode:       firstString(sourceRow, "mode"),
			FocusIDs:   stringSliceFromAny(sourceRow["focus_ids"]),
			ContextIDs: stringSliceFromAny(sourceRow["context_ids"]),
		},
	}
}

func firstPositiveNumber(row map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			if n := numberFromAny(value); n > 0 {
				return n
			}
		}
	}
	return 0
}

func (h *Harness) enrichArtifactScopeArgs(ctx context.Context, cmd map[string]any) map[string]any {
	out := tools.CloneCommand(cmd)
	if scope := artifacts.ScopeFromMap(out); !scope.Empty() {
		return withArtifactScope(out, scope)
	}
	projectPath := firstString(out, "project_path", "current_project_path")
	if projectPath == "" && h != nil {
		state := h.UserStateSummary(ctx)
		projectPath = projectPathFromState(state)
	}
	scope := h.artifactScopeForProject(ctx, projectPath)
	return withArtifactScope(out, scope)
}

func (h *Harness) artifactScopeForProject(ctx context.Context, projectPath string) artifacts.Scope {
	if h == nil {
		return artifacts.Scope{}
	}
	state := h.UserStateSummary(ctx)
	historySummary := h.ProjectHistorySummaryForProject(ctx, "", projectPath)
	scope := map[string]any{
		"project_path":      firstNonEmpty(firstString(historySummary, "project_path", "current_project_path"), projectPathFromState(state)),
		"root_project_path": firstString(historySummary, "root_project_path"),
		"active_worktree":   firstString(historySummary, "active_worktree"),
		"active_branch":     firstString(historySummary, "active_branch"),
		"active_node_id":    firstString(historySummary, "active_node_id"),
	}
	scope["history_scope_key"] = artifactHistoryScopeKey(historySummary, state)
	return artifacts.ScopeFromMap(scope)
}

func (h *Harness) retagArtifactsForSavedProject(ctx context.Context, cmd map[string]any, result map[string]any) map[string]any {
	if result == nil || !boolValueDefault(result["adopted_draft"], false) {
		return result
	}
	draftPath := firstString(result, "draft_project_path")
	projectPath := firstNonEmpty(firstString(result, "project_path"), firstString(cmd, "project_path", "current_project_path"))
	if draftPath == "" || projectPath == "" {
		return result
	}
	from := h.artifactScopeForProject(ctx, draftPath)
	to := h.artifactScopeForProject(ctx, projectPath)
	count, err := artifacts.RetagScope(h.artifactStore(), from, to)
	if err != nil {
		result["warnings"] = appendStringAny(result["warnings"], "artifact scope adoption failed: "+err.Error())
		return result
	}
	result["adopted_artifact_count"] = count
	return result
}

func withArtifactScope(cmd map[string]any, scope artifacts.Scope) map[string]any {
	if cmd == nil {
		cmd = map[string]any{}
	}
	if scope.Empty() {
		return cmd
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
			cmd[key] = value
		}
	}
	return cmd
}

func artifactHistoryScopeKey(projectHistory map[string]any, state map[string]any) string {
	projectPath := firstNonEmpty(firstString(projectHistory, "project_path", "current_project_path"), projectPathFromState(state))
	rootProjectPath := firstString(projectHistory, "root_project_path")
	projectIdentity := firstNonEmpty(projectPath, rootProjectPath, "unsaved")
	return strings.Join([]string{
		projectIdentity,
		firstNonEmpty(rootProjectPath, "root"),
		firstNonEmpty(firstString(projectHistory, "active_worktree"), "main"),
		firstNonEmpty(firstString(projectHistory, "active_branch"), "main"),
		firstNonEmpty(firstString(projectHistory, "active_node_id"), firstString(projectHistory, "head"), "no-node"),
		firstNonEmpty(firstString(projectHistory, "draft", "unsaved"), "saved"),
		firstNonEmpty(firstString(projectHistory, "initialized"), "unknown"),
	}, "::")
}

func (h *Harness) historyArgs(cmd map[string]any) map[string]any {
	out := tools.CloneCommand(cmd)
	currentPath, projectUUID := h.CurrentProjectIdentity(context.Background())
	if projectUUID != "" {
		out["project_uuid"] = projectUUID
	}
	if !isEmptyValue(out["project_path"]) {
		if samePath(firstString(out, "project_path"), currentPath) {
			history.BindProjectIdentity(firstString(out, "project_path"), projectUUID)
		}
		return out
	}
	if currentPath != "" {
		out["project_path"] = currentPath
	}
	return out
}

func (h *Harness) enrichHistoryCheckpointArgs(cmd map[string]any) map[string]any {
	out := h.historyArgs(cmd)
	if !isEmptyValue(out["project_snapshot_xml"]) || h == nil {
		return out
	}
	preferKernelProjectPath := boolValueDefault(out["prefer_kernel_project_path"], false)
	delete(out, "prefer_kernel_project_path")
	reply := h.projectSnapshotExportCompat(nil)
	if snapshot := firstString(reply, "snapshot_xml"); snapshot != "" {
		out["project_snapshot_xml"] = snapshot
	}
	if projectPath := firstString(reply, "project_path"); projectPath != "" {
		out["kernel_project_path"] = projectPath
		if isEmptyValue(out["project_path"]) || preferKernelProjectPath {
			out["project_path"] = projectPath
		}
	}
	return out
}

func (h *Harness) enrichHistorySnapshotArgs(cmd map[string]any) map[string]any {
	out := h.historyArgs(cmd)
	if !isEmptyValue(out["project_snapshot_xml"]) || h == nil {
		return out
	}
	reply := h.projectSnapshotExportCompat(nil)
	if snapshot := firstString(reply, "snapshot_xml"); snapshot != "" {
		out["project_snapshot_xml"] = snapshot
	}
	if isEmptyValue(out["project_path"]) {
		if projectPath := firstString(reply, "project_path"); projectPath != "" {
			out["project_path"] = projectPath
		}
	}
	return out
}

func (h *Harness) worktreeCreateArgs(ctx context.Context, cmd map[string]any) map[string]any {
	out := h.historyArgs(cmd)
	if firstString(out, "commit_id", "branch") != "" {
		return out
	}
	checkpointArgs := h.enrichHistoryCheckpointArgs(map[string]any{
		"project_path":               firstString(out, "project_path", "path"),
		"message":                    "Worktree source autosave",
		"source":                     "worktree_create",
		"checkpoint_kind":            "auto_worktree_source",
		"prefer_kernel_project_path": true,
	})
	if isEmptyValue(checkpointArgs["project_path"]) {
		out["source_checkpoint_warning"] = "current project path is unavailable; worktree source autosave skipped"
		return out
	}
	result, err := history.Checkpoint(checkpointArgs)
	if err != nil {
		out["source_checkpoint_warning"] = err.Error()
		return out
	}
	if id := firstString(result, "commit_id"); id != "" {
		out["commit_id"] = id
		out["source_checkpoint_id"] = id
	}
	if projectPath := firstString(checkpointArgs, "project_path"); projectPath != "" {
		out["project_path"] = projectPath
	}
	_ = ctx
	return out
}

func (h *Harness) worktreeCheckoutArgs(ctx context.Context, cmd map[string]any) map[string]any {
	out := h.historyArgs(cmd)
	if h == nil || h.kernel == nil {
		return out
	}
	checkpointArgs := h.enrichHistoryCheckpointArgs(map[string]any{
		"project_path":               "",
		"message":                    "Worktree switch autosave",
		"source":                     "worktree_checkout",
		"checkpoint_kind":            "auto_worktree_switch",
		"prefer_kernel_project_path": true,
	})
	if targetPath := firstString(out, "project_file_path"); targetPath != "" {
		if sourcePath := firstString(checkpointArgs, "project_path"); samePath(sourcePath, targetPath) {
			return out
		}
	}
	if isEmptyValue(checkpointArgs["project_path"]) {
		out["switch_autosave_warning"] = "current project path is unavailable; worktree switch autosave skipped"
		return out
	}
	result, err := history.Checkpoint(checkpointArgs)
	if err != nil {
		out["switch_autosave_warning"] = err.Error()
		return out
	}
	if id := firstString(result, "commit_id"); id != "" {
		out["switch_autosave_commit_id"] = id
	}
	_ = ctx
	return out
}

func (h *Harness) ensureProjectHistoryBaseline(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, goalID, runID string) (string, error) {
	if h == nil || h.runtime == nil || strings.TrimSpace(goalID) == "" || !shouldAutoGoalBaseline(spec) {
		return "", nil
	}
	goal := h.runtime.Ensure(goalID, runID, "")
	if goal.ProjectHistory != nil && goal.ProjectHistory.BaselineCreated && strings.TrimSpace(goal.ProjectHistory.BaselineCommitID) != "" {
		return goal.ProjectHistory.BaselineCommitID, nil
	}
	args := h.enrichHistoryCheckpointArgs(map[string]any{
		"project_path":    firstString(cmd, "project_path", "path"),
		"message":         "Auto baseline before " + spec.CommandName,
		"goal_id":         goalID,
		"run_id":          runID,
		"source":          "runtime_guard",
		"checkpoint_kind": "auto_goal_baseline",
	})
	result, err := history.Checkpoint(args)
	if err != nil {
		return "", fmt.Errorf("project history baseline failed: %w", err)
	}
	commitID := firstString(result, "commit_id")
	if commitID == "" {
		if commit, ok := result["commit"].(history.Commit); ok {
			commitID = commit.ID
		}
	}
	meta := projectHistoryMetaFromResult(result)
	meta.BaselineCommitID = commitID
	meta.BaselineCreated = commitID != ""
	if commitID == "" {
		meta.Warnings = append(meta.Warnings, "baseline checkpoint did not return a commit_id")
	}
	h.runtime.SetProjectHistory(goalID, meta)
	_ = ctx
	return commitID, nil
}

// EnsureCapabilityExecutionBaseline exposes the existing Project History
// guard to the v1 Execution Coordinator without routing the capability
// mutation back through the legacy pending/tool authority.
func (h *Harness) EnsureCapabilityExecutionBaseline(ctx context.Context, goalID, runID, capabilityID string) (string, error) {
	capabilityID = strings.TrimSpace(capabilityID)
	if capabilityID == "" {
		return "", fmt.Errorf("capability id is required")
	}
	return h.ensureProjectHistoryBaseline(ctx, tools.CommandSpec{
		CommandName:    "capability.execute." + capabilityID,
		Category:       "capability_runtime_v1",
		MutatesProject: true,
	}, map[string]any{"capability_id": capabilityID}, goalID, runID)
}

func shouldAutoGoalBaseline(spec tools.CommandSpec) bool {
	if !spec.MutatesProject {
		return false
	}
	switch spec.CommandName {
	case "version_checkpoint", "version_restore", "version_branch_create", "version_worktree_create", "version_checkout":
		return false
	default:
		return spec.Category != "project_history"
	}
}

func projectHistoryMetaFromResult(result map[string]any) agentruntime.ProjectHistoryMeta {
	meta := agentruntime.ProjectHistoryMeta{
		ActiveBranch: firstString(result, "active_branch"),
		Detached:     boolValueDefault(result["detached"], false),
		Head:         firstString(result, "head"),
	}
	meta.Warnings = stringSliceFromAny(result["warnings"])
	return meta
}

func (h *Harness) buildPluginSemanticIndex(cmd map[string]any) (map[string]any, error) {
	inventory, err := h.listAvailablePlugins(context.Background(), cmd)
	if err != nil {
		return nil, err
	}
	rows := mapRowsFromAny(inventory["plugins"])
	idx := pluginsemantics.Build(rows, time.Now().UTC())
	idx.Source = "scan_plugins"
	path, err := pluginsemantics.Save(firstString(cmd, "index_path", "path"), idx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"index_path":     path,
		"schema_version": idx.SchemaVersion,
		"built_at":       idx.BuiltAt.Format(time.RFC3339),
		"plugin_count":   len(idx.Entries),
		"summary":        idx.Summary,
		"warnings":       idx.Warnings,
		"source":         idx.Source,
		"scanned_paths":  inventory["scanned_paths"],
	}, nil
}

func (h *Harness) listAvailablePlugins(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	rows, scannedPaths, err := h.scanAvailablePluginRows(ctx, cmd)
	if err != nil {
		return nil, err
	}
	limit := len(rows)
	if n, ok := firstPositiveInt(cmd, "limit", "max_plugins", "max_results"); ok && n < limit {
		limit = n
	}
	if limit < 0 {
		limit = 0
	}
	out := append([]map[string]any(nil), rows...)
	if len(out) > limit {
		out = out[:limit]
	}
	return map[string]any{
		"status":        "ok",
		"source":        "scan_plugins",
		"scanned_paths": scannedPaths,
		"plugin_count":  len(rows),
		"plugins":       out,
		"entries":       out,
	}, nil
}

func (h *Harness) searchAvailablePlugins(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	query := firstString(cmd, "query", "search", "plugin_query", "name")
	rows, scannedPaths, err := h.scanAvailablePluginRows(ctx, cmd)
	if err != nil {
		return nil, err
	}
	limit := 12
	if n, ok := firstPositiveInt(cmd, "limit", "max_results"); ok {
		limit = n
	}
	idx := pluginsemantics.Build(rows, time.Now().UTC())
	results := pluginsemantics.Search(idx, pluginsemantics.SearchOptions{
		Query:              query,
		Type:               firstString(cmd, "type", "semantic_type", "role", "effect_type"),
		Manufacturer:       firstString(cmd, "manufacturer", "maker", "vendor"),
		Format:             firstString(cmd, "format"),
		Limit:              limit,
		IncludeInstruments: true,
	})
	out := pluginSemanticEntriesToRows(results)
	return map[string]any{
		"status":        "ok",
		"source":        "scan_plugins",
		"query":         query,
		"scanned_paths": scannedPaths,
		"plugin_count":  len(rows),
		"match_count":   len(out),
		"plugins":       out,
		"entries":       out,
	}, nil
}

func (h *Harness) scanAvailablePluginRows(ctx context.Context, cmd map[string]any) ([]map[string]any, []string, error) {
	if h == nil || h.kernel == nil {
		return nil, nil, fmt.Errorf("plugin inventory requires a kernel client")
	}
	scanCmd := map[string]any{"cmd": "scan_plugins"}
	paths := stringSliceFromAny(cmd["paths"])
	if len(paths) == 0 {
		if path := firstString(cmd, "path", "plugin_path", "folder", "directory"); path != "" {
			paths = []string{path}
		}
	}
	if len(paths) == 0 {
		paths = defaultPluginScanPaths()
	}
	scanCmd["paths"] = paths
	reply, _, err := h.kernel.SendCommand(ctx, scanCmd)
	if err != nil {
		return nil, paths, err
	}
	if pluginKernelErrored(reply) {
		return nil, paths, fmt.Errorf("%s", firstNonEmpty(firstString(reply, "message"), firstString(reply, "error"), "scan_plugins failed"))
	}
	rows := normalizePluginInventoryRows(mapRowsFromAny(reply["plugins"]))
	return rows, paths, nil
}

func defaultPluginScanPaths() []string {
	out := []string{`C:\Program Files\Common Files\VST3`}
	if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
		out = append(out, filepath.Join(programFiles, "Steinberg", "VSTPlugins"))
	}
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		out = append(out, filepath.Join(localAppData, "Programs", "Common", "CLAP"))
	}
	return out
}

func normalizePluginInventoryRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		next := cloneAnyMap(row)
		name := firstString(next, "name", "descriptive_name", "display_name")
		if name == "" {
			continue
		}
		if firstString(next, "plugin_path", "path", "file_path", "file_or_identifier") == "" {
			if identifier := firstString(next, "identifier"); identifier != "" {
				next["file_or_identifier"] = identifier
			}
		}
		if firstString(next, "plugin_path") == "" {
			if path := firstString(next, "path", "file_path", "file_or_identifier"); path != "" {
				next["plugin_path"] = path
			}
		}
		if firstString(next, "format") == "" {
			next["format"] = "VST3"
		}
		out = append(out, next)
	}
	return out
}

func pluginSemanticEntriesToRows(entries []pluginsemantics.Entry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, map[string]any{
			"id":                 entry.ID,
			"name":               entry.Name,
			"descriptive_name":   entry.DescriptiveName,
			"manufacturer":       entry.Manufacturer,
			"format":             entry.Format,
			"category":           entry.Category,
			"identifier":         entry.Identifier,
			"plugin_path":        entry.PluginPath,
			"file_or_identifier": firstNonEmpty(entry.PluginPath, entry.Identifier),
			"is_instrument":      entry.IsInstrument,
			"primary_type":       entry.PrimaryType,
			"semantic_types":     entry.SemanticTypes,
			"confidence":         entry.Confidence,
			"match_score":        entry.SearchScore,
		})
	}
	return out
}

func (h *Harness) searchPluginSemanticIndex(cmd map[string]any) (map[string]any, error) {
	path := firstString(cmd, "index_path", "path")
	idx, err := pluginsemantics.Load(path)
	indexExists := true
	transient := false
	query := firstString(cmd, "query", "search", "plugin_query")
	if err != nil {
		if !pluginsemantics.IsNotExist(err) {
			return nil, err
		}
		if h != nil && h.kernel != nil {
			limit := 2000
			if n, ok := firstPositiveInt(cmd, "source_limit", "max_plugins"); ok {
				limit = n
			}
			reply, listErr := h.listAvailablePlugins(context.Background(), map[string]any{"limit": limit})
			if listErr != nil {
				return nil, listErr
			}
			idx = pluginsemantics.Build(mapRowsFromAny(reply["plugins"]), time.Now().UTC())
			indexExists = false
			transient = true
		}
		if len(idx.Entries) == 0 {
			defaultPath, _ := pluginsemantics.DefaultPath()
			return map[string]any{
				"index_exists": false,
				"transient":    transient,
				"index_path":   firstNonEmpty(path, defaultPath),
				"plugins":      []pluginsemantics.Entry{},
				"entries":      []pluginsemantics.Entry{},
				"message":      "plugin semantic index has not been built yet",
			}, nil
		}
	}
	liveSearchCount := 0
	liveSearchWarning := ""
	if h != nil && h.kernel != nil && strings.TrimSpace(query) != "" {
		limit := 24
		if n, ok := firstPositiveInt(cmd, "live_limit"); ok {
			limit = n
		}
		reply, liveErr := h.searchAvailablePlugins(context.Background(), map[string]any{
			"query": query,
			"limit": limit,
			"type":  firstString(cmd, "type", "semantic_type", "role", "effect_type"),
		})
		if liveErr != nil {
			liveSearchWarning = liveErr.Error()
		} else {
			liveIdx := pluginsemantics.Build(mapRowsFromAny(reply["plugins"]), time.Now().UTC())
			liveSearchCount = len(liveIdx.Entries)
			idx.Entries = mergePluginSemanticEntries(idx.Entries, liveIdx.Entries)
			idx.Summary = pluginsemantics.BuildSummary(idx.Entries)
			transient = transient || liveSearchCount > 0
		}
	}
	limit := 12
	if n, ok := firstPositiveInt(cmd, "limit", "max_results"); ok {
		limit = n
	}
	includeInstruments, _ := boolValue(cmd["include_instruments"])
	results := pluginsemantics.Search(idx, pluginsemantics.SearchOptions{
		Query:              query,
		Type:               firstString(cmd, "type", "semantic_type", "role", "effect_type"),
		Manufacturer:       firstString(cmd, "manufacturer", "maker", "vendor"),
		Format:             firstString(cmd, "format"),
		Limit:              limit,
		IncludeInstruments: includeInstruments,
	})
	defaultPath, _ := pluginsemantics.DefaultPath()
	return map[string]any{
		"index_exists": indexExists,
		"transient":    transient,
		"index_path":   firstNonEmpty(path, defaultPath),
		"built_at":     idx.BuiltAt.Format(time.RFC3339),
		"summary":      idx.Summary,
		"plugins":      results,
		"entries":      results,
		"live_search": map[string]any{
			"queried": strings.TrimSpace(query) != "",
			"count":   liveSearchCount,
			"warning": liveSearchWarning,
		},
	}, nil
}

func mergePluginSemanticEntries(primary []pluginsemantics.Entry, secondary []pluginsemantics.Entry) []pluginsemantics.Entry {
	if len(primary) == 0 {
		return append([]pluginsemantics.Entry(nil), secondary...)
	}
	if len(secondary) == 0 {
		return append([]pluginsemantics.Entry(nil), primary...)
	}
	merged := make([]pluginsemantics.Entry, 0, len(primary)+len(secondary))
	seen := map[string]bool{}
	add := func(entry pluginsemantics.Entry) {
		key := strings.ToLower(strings.TrimSpace(firstNonEmpty(entry.ID, entry.PluginPath, entry.Identifier, entry.Name)))
		if key != "" && seen[key] {
			return
		}
		if key != "" {
			seen[key] = true
		}
		merged = append(merged, entry)
	}
	for _, entry := range primary {
		add(entry)
	}
	for _, entry := range secondary {
		add(entry)
	}
	return merged
}

func (h *Harness) getPluginSemanticEntry(cmd map[string]any) (map[string]any, error) {
	path := firstString(cmd, "index_path", "path")
	idx, err := pluginsemantics.Load(path)
	if err != nil {
		return nil, err
	}
	needle := firstNonEmpty(firstString(cmd, "id", "plugin_id", "plugin_path", "identifier"), firstString(cmd, "query", "search", "name"))
	if strings.TrimSpace(needle) == "" {
		return nil, fmt.Errorf("id or query is required")
	}
	entry, ok := pluginsemantics.Get(idx, needle)
	if !ok {
		return map[string]any{
			"found": false,
			"entry": nil,
		}, nil
	}
	return map[string]any{
		"found": true,
		"entry": entry,
	}, nil
}

func pluginKernelErrored(reply map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(firstString(reply, "status")))
	return status == "error" || status == "failed"
}

func flattenCommandParams(cmd map[string]any) {
	for _, nestedKey := range []string{"params", "args"} {
		params, ok := cmd[nestedKey].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range params {
			if isEmptyValue(cmd[k]) && !isEmptyValue(v) {
				cmd[k] = v
			}
		}
		delete(cmd, nestedKey)
	}
}

func (h *Harness) resolveImplicitTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	normalizeCommandArgs(spec, cmd, requestContext)
	h.resolveRackAddNodeZone(ctx, spec, cmd)

	if err := h.resolveMacroTargets(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve macro target: %w", spec.ToolName, err)
	}

	if err := h.resolveImportAudioSource(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve import source: %w", spec.ToolName, err)
	}
	if err := h.resolveImportMidiSource(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve import source: %w", spec.ToolName, err)
	}

	if err := h.resolveClipTargets(ctx, spec, cmd, requestContext); err != nil {
		return fmt.Errorf("%s could not resolve clip target: %w", spec.ToolName, err)
	}

	for _, field := range spec.RequiredTargetIDs {
		if !isEmptyValue(cmd[field]) {
			continue
		}
		switch field {
		case "track_id":
			trackID, err := h.resolveTrackID(ctx, spec, cmd, requestContext)
			if err != nil {
				return fmt.Errorf("%s could not resolve track target: %w", spec.ToolName, err)
			}
			if trackID != "" {
				cmd[field] = trackID
			}
		case "plugin_id":
			pluginID, err := h.resolvePluginID(ctx, spec, cmd, requestContext)
			if err != nil {
				return fmt.Errorf("%s could not resolve plugin target: %w", spec.ToolName, err)
			}
			if pluginID != "" {
				cmd[field] = pluginID
			}
		}
	}
	return nil
}

func normalizeCommandArgs(spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) {
	switch spec.CommandName {
	case "rename_track":
		copyFirstNonEmpty(cmd, "name", "new_name", "track_name")
	case "set_mute":
		copyBoolAlias(cmd, "mute", "muted", "enabled", "value")
		inferBoolFromUserMessage(cmd, "mute", requestContext,
			[]string{"unmute", "\u53d6\u6d88\u9759\u97f3", "\u89e3\u9664\u9759\u97f3", "\u53d6\u6d88mute", "\u53d6\u6d88 mute", "\u5173\u95ed\u9759\u97f3"},
			[]string{"mute", "\u9759\u97f3"})
	case "set_solo":
		copyBoolAlias(cmd, "solo", "is_solo", "enabled", "value")
		inferBoolFromUserMessage(cmd, "solo", requestContext,
			[]string{"unsolo", "\u53d6\u6d88\u72ec\u594f", "\u89e3\u9664\u72ec\u594f", "\u53d6\u6d88solo", "\u53d6\u6d88 solo", "\u5173\u95ed\u72ec\u594f"},
			[]string{"solo", "\u72ec\u594f"})
	case "arm_track":
		copyBoolAlias(cmd, "is_armed", "arm", "armed", "enabled", "value")
		inferBoolFromUserMessage(cmd, "is_armed", requestContext,
			[]string{"disarm", "鍙栨秷褰曢煶鍑嗗", "瑙ｉ櫎褰曢煶鍑嗗", "鍙栨秷arm", "鍙栨秷 arm"},
			[]string{"arm", "褰曢煶鍑嗗"})
	case "set_click":
		copyBoolAlias(cmd, "enabled", "click", "value")
	case "set_tempo":
		copyFirstNonEmpty(cmd, "bpm", "tempo", "value")
	case "seek":
		copyFirstNonEmpty(cmd, "time", "position_seconds", "seconds", "value")
	case "move_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds", "time")
		inferTimeUnit(cmd)
	case "resize_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds")
		copyFirstNonEmpty(cmd, "new_length", "new_length_seconds", "new_length_beats", "new_length_beat", "length", "length_seconds", "length_beats", "duration", "duration_seconds", "duration_beats")
		copyFirstNonEmpty(cmd, "offset_in_source", "offset_in_source_seconds", "offset_in_source_beats", "offset_in_source_beat")
		inferTimeUnit(cmd)
	case "split_clip":
		copyFirstNonEmpty(cmd, "clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "split_time", "split_time_seconds", "split_time_beats", "split_time_beat", "position_seconds", "time", "at", "start", "start_seconds", "start_beats")
		inferTimeUnit(cmd)
	case "clone_clip":
		copyFirstNonEmpty(cmd, "source_clip_id", "clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "target_track_id", "track_id")
		copyFirstNonEmpty(cmd, "new_start", "new_start_seconds", "new_start_beats", "new_start_beat", "start", "start_seconds", "start_beats", "position_seconds", "time")
		inferTimeUnit(cmd)
	case "remove_clips":
		normalizeClipIDsArray(cmd)
	case "clip.fade.set":
		copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "fade_in_seconds", "fade_in", "fadeInSeconds", "fadeIn", "in_seconds")
		copyFirstNonEmpty(cmd, "fade_out_seconds", "fade_out", "fadeOutSeconds", "fadeOut", "out_seconds")
		copyFirstNonEmpty(cmd, "fade_in_curve", "fadeInCurve", "in_curve")
		copyFirstNonEmpty(cmd, "fade_out_curve", "fadeOutCurve", "out_curve")
		copyFirstNonEmpty(cmd, "auto_crossfade", "autoCrossfade")
	case "clip.fade.read", "clip.gain.read":
		copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
	case "clip.gain.set":
		copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "gain_db", "clip_gain_db", "db", "value_db", "value")
	case "get_midi_clip_notes", "get_midi_clip_data":
		copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
		copyFirstNonEmpty(cmd, "track_id", "selected_clip_track_id", "focused_track_id", "selected_track_id")
	case "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes":
		normalizeLegacyMidiNoteArgs(cmd)
	case "apply_midi_note_patch":
		normalizeMidiPatchArgs(spec, cmd)
	case "import_audio":
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "audio_path", "source_path", "source_file", "selected_library_file_path", "library_file_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id")
		copyFirstNonEmpty(cmd, "offset_time", "start_time", "start", "start_seconds", "position_seconds", "time")
	case "project.import_folder_as_stems", "project.import_audio_files":
		copyFirstNonEmpty(cmd, "folder_path", "folder", "directory", "asset_location", "asset_folder")
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "audio_path", "source_path", "source_file")
		copyFirstNonEmpty(cmd, "start_time_seconds", "start_time", "offset_time", "start", "start_seconds", "position_seconds", "time")
		if isEmptyValue(cmd["target_policy"]) {
			cmd["target_policy"] = "create_tracks"
		}
		if isEmptyValue(cmd["command_timeout_ms"]) {
			cmd["command_timeout_ms"] = 120000
		}
	case "project.audio_analysis_start", "project.audio_analysis_status", "project.audio_analysis_cancel":
		copyFirstNonEmpty(cmd, "analysis_job_id", "job_id", "audio_analysis_job_id")
	case "plugin_search":
		copyFirstNonEmpty(cmd, "query", "plugin_query", "plugin_name", "name", "search")
	case "plugin_semantic_search":
		copyFirstNonEmpty(cmd, "query", "plugin_query", "plugin_name", "name", "search")
		copyFirstNonEmpty(cmd, "type", "semantic_type", "role", "effect_type")
	case "plugin_semantic_get":
		copyFirstNonEmpty(cmd, "id", "plugin_id", "plugin_path", "path", "identifier")
		copyFirstNonEmpty(cmd, "query", "plugin_query", "plugin_name", "name", "search")
	case "scan_plugins":
		normalizePluginScanArgs(cmd)
	case "rack_add_node":
		copyFirstNonEmpty(cmd, "plugin_path", "path", "file_path", "plugin_file", "source_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id", "selected_plugin_track_id")
		copyFirstNonEmpty(cmd, "zone_id", "zone", "rack_zone", "rack_zone_id")
		if isEmptyValue(cmd["x"]) {
			cmd["x"] = 360.0
		}
		if isEmptyValue(cmd["y"]) {
			cmd["y"] = 260.0
		}
	case "import_media_to_track":
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "audio_path", "source_path", "source_file", "selected_library_file_path", "library_file_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id")
		copyFirstNonEmpty(cmd, "start_time", "offset_time", "start", "start_seconds", "position_seconds", "time")
		if isEmptyValue(cmd["media_type"]) {
			cmd["media_type"] = "audio"
		}
		if isEmptyValue(cmd["mode"]) {
			cmd["mode"] = "non_destructive"
		}
	case "import_midi_to_track":
		copyFirstNonEmpty(cmd, "file_path", "path", "absolute_path", "midi_path", "source_path", "source_file", "selected_library_file_path", "library_file_path", "selected_attachment_file_path", "attachment_import_file_path")
		copyFirstNonEmpty(cmd, "track_id", "target_track_id", "selected_track_id")
		copyFirstNonEmpty(cmd, "start_time_beats", "start_beats", "start_beat", "start_time", "start", "position_beats", "time")
		if isEmptyValue(cmd["mode"]) {
			cmd["mode"] = "merge_tracks"
		}
	}
}

func (h *Harness) resolveRackAddNodeZone(ctx context.Context, spec tools.CommandSpec, cmd map[string]any) {
	if spec.CommandName != "rack_add_node" {
		return
	}
	if instrument, ok := rackPluginMetadataIsInstrument(cmd); ok {
		applyRackPluginZone(cmd, instrument)
		return
	}
	if entry, ok := h.rackPluginSemanticEntry(ctx, firstString(cmd, "plugin_path")); ok {
		applyRackPluginZone(cmd, pluginSemanticEntryIsInstrument(entry))
		return
	}
	if isEmptyValue(cmd["zone_id"]) {
		cmd["zone_id"] = "Z3"
	}
}

func applyRackPluginZone(cmd map[string]any, instrument bool) {
	if instrument {
		if !strings.EqualFold(strings.TrimSpace(firstString(cmd, "zone_id")), "Z2") {
			cmd["zone_id"] = "Z2"
		}
		return
	}
	if isEmptyValue(cmd["zone_id"]) {
		cmd["zone_id"] = "Z3"
	}
}

func (h *Harness) rackPluginSemanticEntry(ctx context.Context, pluginPath string) (pluginsemantics.Entry, bool) {
	pluginPath = strings.TrimSpace(pluginPath)
	if pluginPath == "" {
		return pluginsemantics.Entry{}, false
	}
	if idx, err := pluginsemantics.Load(""); err == nil {
		if entry, ok := pluginsemantics.Get(idx, pluginPath); ok {
			return entry, true
		}
	}
	if h == nil || h.kernel == nil {
		return pluginsemantics.Entry{}, false
	}
	reply, err := h.listAvailablePlugins(ctx, map[string]any{"limit": 2000})
	if err != nil {
		return pluginsemantics.Entry{}, false
	}
	rows := mapRowsFromAny(reply["plugins"])
	if len(rows) == 0 {
		rows = mapRowsFromAny(reply["entries"])
	}
	if len(rows) == 0 {
		return pluginsemantics.Entry{}, false
	}
	idx := pluginsemantics.Build(rows, time.Now().UTC())
	entry, ok := pluginsemantics.Get(idx, pluginPath)
	return entry, ok
}

func rackPluginMetadataIsInstrument(row map[string]any) (bool, bool) {
	if row == nil {
		return false, false
	}
	for _, key := range []string{"is_instrument", "isInstrument", "plugin_is_instrument", "instrument"} {
		if value, ok := boolValue(row[key]); ok {
			return value, true
		}
	}
	if categoryIndicatesInstrument(firstString(row, "category", "plugin_category")) {
		return true, true
	}
	if semanticTagsIndicateInstrument(row["semantic_types"]) {
		return true, true
	}
	for _, key := range []string{"plugin", "plugin_entry", "entry", "candidate", "resolved_plugin"} {
		if nested, ok := row[key].(map[string]any); ok {
			if value, found := rackPluginMetadataIsInstrument(nested); found {
				return value, true
			}
		}
	}
	return false, false
}

func pluginSemanticEntryIsInstrument(entry pluginsemantics.Entry) bool {
	if entry.IsInstrument || categoryIndicatesInstrument(entry.Category) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(entry.PrimaryType), "instrument") {
		return true
	}
	for _, tag := range entry.SemanticTypes {
		if strings.EqualFold(strings.TrimSpace(tag.Type), "instrument") {
			return true
		}
	}
	return false
}

func categoryIndicatesInstrument(category string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(category)), "instrument")
}

func semanticTagsIndicateInstrument(value any) bool {
	switch tags := value.(type) {
	case []string:
		for _, tag := range tags {
			if strings.EqualFold(strings.TrimSpace(tag), "instrument") {
				return true
			}
		}
	case []any:
		for _, tag := range tags {
			switch typed := tag.(type) {
			case string:
				if strings.EqualFold(strings.TrimSpace(typed), "instrument") {
					return true
				}
			case map[string]any:
				if strings.EqualFold(firstString(typed, "type", "name", "value"), "instrument") {
					return true
				}
			}
		}
	}
	return false
}

func normalizePluginScanArgs(cmd map[string]any) {
	if len(stringSliceFromAny(cmd["paths"])) == 0 {
		if path := firstString(cmd, "path", "plugin_path", "folder", "directory"); path != "" {
			cmd["paths"] = []string{path}
		}
	}
}

func normalizeLegacyMidiNoteArgs(cmd map[string]any) {
	copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
	copyFirstNonEmpty(cmd, "track_id", "selected_clip_track_id", "focused_track_id", "selected_track_id")
	if isEmptyValue(cmd["time_unit"]) {
		cmd["time_unit"] = "beats"
	}
	commandName := firstString(cmd, "cmd", "command", "action")
	if commandName == "delete_midi_notes" {
		normalizeLegacyDeleteNoteIDs(cmd)
		return
	}
	if len(operationRowsFromAny(cmd["notes"])) == 0 {
		switch commandName {
		case "add_midi_notes", "add_midi_notes_bulk":
			if note := legacyMidiNoteFromTopLevel(cmd, false); note != nil {
				cmd["notes"] = []map[string]any{note}
			}
		case "mutate_midi_notes":
			if note := legacyMidiNoteFromTopLevel(cmd, true); note != nil {
				cmd["notes"] = []map[string]any{note}
			}
		}
	}
	if notes := operationRowsFromAny(cmd["notes"]); len(notes) > 0 {
		for _, note := range notes {
			copyFirstNonEmpty(note, "length", "duration", "length_beats", "duration_beats")
			copyFirstNonEmpty(note, "start", "start_beat", "start_beats")
			if isEmptyValue(note["id"]) && !isEmptyValue(note["note_id"]) {
				note["id"] = note["note_id"]
			}
		}
		cmd["notes"] = notes
	}
}

func normalizeLegacyDeleteNoteIDs(cmd map[string]any) {
	if ids := stringSliceFromAny(cmd["note_ids"]); len(ids) > 0 {
		cmd["note_ids"] = ids
		return
	}
	if id := firstString(cmd, "note_id", "id"); id != "" {
		cmd["note_ids"] = []string{id}
		return
	}
	notes := operationRowsFromAny(cmd["notes"])
	ids := make([]string, 0, len(notes))
	for _, note := range notes {
		if id := firstString(note, "note_id", "id"); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		cmd["note_ids"] = ids
	}
}

func legacyMidiNoteFromTopLevel(cmd map[string]any, requireID bool) map[string]any {
	if requireID && firstString(cmd, "note_id", "id") == "" {
		return nil
	}
	if !requireID && isEmptyValue(cmd["pitch"]) {
		return nil
	}
	note := map[string]any{}
	for _, key := range []string{"id", "note_id", "pitch", "start", "start_beat", "start_beats", "length", "length_beats", "duration", "duration_beats", "velocity", "channel"} {
		if !isEmptyValue(cmd[key]) {
			note[key] = cmd[key]
		}
	}
	if len(note) == 0 {
		return nil
	}
	return note
}

func normalizeMidiPatchArgs(spec tools.CommandSpec, cmd map[string]any) {
	copyFirstNonEmpty(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id")
	copyFirstNonEmpty(cmd, "track_id", "selected_clip_track_id", "focused_track_id", "selected_track_id")
	if isEmptyValue(cmd["time_unit"]) {
		cmd["time_unit"] = "beats"
	}
	if ops := operationRowsFromAny(cmd["operations"]); len(ops) > 0 {
		cmd["operations"] = normalizeMidiPatchOperations(ops)
		return
	}
	if op := firstString(cmd, "op", "operation"); op != "" {
		row := cloneAnyMap(cmd)
		row["op"] = op
		delete(row, "cmd")
		delete(row, "clip_id")
		delete(row, "track_id")
		delete(row, "time_unit")
		delete(row, "undo_label")
		delete(row, "agent_action_id")
		normalizeMidiPatchOperation(row)
		cmd["operations"] = []map[string]any{row}
		return
	}
	if notes := insertNoteOperations(cmd); len(notes) > 0 {
		cmd["operations"] = notes
		return
	}
	if looksLikeInsertNoteOperation(cmd) {
		row := midiOperationFromArgs("insert_note", cmd)
		cmd["operations"] = []map[string]any{row}
		return
	}
	switch spec.ToolName {
	case "midi.insert_notes":
		cmd["operations"] = insertNoteOperations(cmd)
	case "midi.delete_notes":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("delete_note", cmd)}
	case "midi.move_notes":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("move_note", cmd)}
	case "midi.resize_notes":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("resize_note", cmd)}
	case "midi.quantize":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("quantize_region", cmd)}
	case "midi.transpose":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("transpose_note", cmd)}
	case "midi.set_velocity":
		cmd["operations"] = []map[string]any{midiOperationFromArgs("set_velocity", cmd)}
	case "midi.replace_region":
		row := midiOperationFromArgs("replace_region", cmd)
		normalizeMidiPatchOperation(row)
		cmd["operations"] = []map[string]any{row}
	case "midi.write_clip_notes":
		if ops := insertNoteOperations(cmd); len(ops) > 0 {
			cmd["operations"] = ops
		}
	}
}

func normalizeMidiPatchOperations(ops []map[string]any) []map[string]any {
	for _, op := range ops {
		normalizeMidiPatchOperation(op)
	}
	return ops
}

func normalizeMidiPatchOperation(op map[string]any) {
	if isEmptyValue(op["op"]) {
		copyFirstNonEmpty(op, "op", "operation", "type", "action", "kind")
	}
	if opName := normalizeMidiOperationName(firstString(op, "op")); opName != "" {
		op["op"] = opName
	}
	if firstString(op, "op") != "replace_region" {
		flattenNestedMidiNoteOperation(op)
	}
	if firstString(op, "op") == "replace_region" {
		copyFirstNonEmpty(op, "start", "region_start", "region_start_beat", "region_start_beats")
		copyFirstNonEmpty(op, "length", "region_length", "region_length_beats", "region_duration", "region_duration_beats")
	}
	copyFirstNonEmpty(op, "length", "duration", "length_beats", "duration_beats")
	copyFirstNonEmpty(op, "start", "start_beat", "start_beats")
	if isEmptyValue(op["id"]) && !isEmptyValue(op["note_id"]) {
		op["id"] = op["note_id"]
	}
	if isEmptyValue(op["op"]) && looksLikeInsertNoteOperation(op) {
		op["op"] = "insert_note"
	}
	normalizeReplaceRegionNotes(op)
	if notes := operationRowsFromAny(op["notes"]); len(notes) > 0 {
		op["notes"] = normalizeMidiPatchNoteRows(notes)
	}
}

func normalizeMidiPatchNoteRows(notes []map[string]any) []map[string]any {
	for _, note := range notes {
		copyFirstNonEmpty(note, "length", "duration", "length_beats", "duration_beats")
		copyFirstNonEmpty(note, "start", "start_beat", "start_beats", "relative_start", "relative_start_beats")
		if isEmptyValue(note["id"]) && !isEmptyValue(note["note_id"]) {
			note["id"] = note["note_id"]
		}
	}
	return notes
}

func translateInsertNotePatchToLegacyCommand(cmd map[string]any) bool {
	ops := operationRowsFromAny(cmd["operations"])
	if len(ops) == 0 {
		return false
	}
	notes := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		normalized := cloneAnyMap(op)
		normalizeMidiPatchOperation(normalized)
		if firstString(normalized, "op") != "insert_note" {
			return false
		}
		note := map[string]any{}
		for _, key := range []string{"id", "note_id", "pitch", "start", "start_beat", "start_beats", "length", "length_beats", "duration", "duration_beats", "velocity", "channel"} {
			if !isEmptyValue(normalized[key]) {
				note[key] = normalized[key]
			}
		}
		copyFirstNonEmpty(note, "length", "duration", "length_beats", "duration_beats")
		copyFirstNonEmpty(note, "start", "start_beat", "start_beats")
		if len(note) == 0 {
			return false
		}
		notes = append(notes, note)
	}
	cmd["cmd"] = "add_midi_notes"
	cmd["notes"] = notes
	delete(cmd, "operations")
	return true
}

func normalizeReplaceRegionNotes(op map[string]any) {
	if firstString(op, "op") != "replace_region" || len(operationRowsFromAny(op["notes"])) > 0 {
		return
	}
	for _, key := range []string{"replacement_notes", "new_notes"} {
		if notes := operationRowsFromAny(op[key]); len(notes) > 0 {
			op["notes"] = normalizeMidiPatchNoteRows(notes)
			return
		}
	}
	for _, key := range []string{"replacement_note", "new_note", "note"} {
		if note, ok := op[key].(map[string]any); ok {
			op["notes"] = normalizeMidiPatchNoteRows([]map[string]any{cloneAnyMap(note)})
			return
		}
	}
	if isEmptyValue(op["pitch"]) {
		return
	}
	note := map[string]any{"pitch": op["pitch"]}
	copyFirstNonEmptyFrom(note, "velocity", op, "note_velocity", "replacement_velocity", "velocity")
	copyFirstNonEmptyFrom(note, "channel", op, "note_channel", "replacement_channel", "channel")
	copyFirstNonEmptyFrom(note, "id", op, "note_id", "replacement_note_id", "replacement_id")
	copyFirstNonEmptyFrom(note, "start", op, "note_start", "note_start_beat", "note_start_beats", "relative_start", "relative_start_beats", "replacement_start", "replacement_start_beat", "replacement_start_beats")
	copyFirstNonEmptyFrom(note, "length", op, "note_length", "note_length_beats", "note_duration", "note_duration_beats", "replacement_length", "replacement_length_beats", "replacement_duration", "replacement_duration_beats")
	if isEmptyValue(note["start"]) {
		note["start"] = 0.0
	}
	op["notes"] = normalizeMidiPatchNoteRows([]map[string]any{note})
}

func flattenNestedMidiNoteOperation(op map[string]any) {
	note, ok := op["note"].(map[string]any)
	if !ok {
		return
	}
	for _, key := range []string{"id", "note_id", "pitch", "start", "start_beat", "start_beats", "length", "length_beats", "duration", "duration_beats", "velocity", "channel"} {
		if isEmptyValue(op[key]) && !isEmptyValue(note[key]) {
			op[key] = note[key]
		}
	}
}

func normalizeMidiOperationName(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "insert", "add", "add_note", "create_note", "insert_midi_note":
		return "insert_note"
	case "delete", "remove", "remove_note":
		return "delete_note"
	case "move":
		return "move_note"
	case "resize", "length", "set_length":
		return "resize_note"
	case "transpose", "transpose_notes":
		return "transpose_note"
	case "velocity", "set_note_velocity":
		return "set_velocity"
	case "quantize", "quantize_notes":
		return "quantize_region"
	default:
		return strings.TrimSpace(raw)
	}
}

func looksLikeInsertNoteOperation(op map[string]any) bool {
	return !isEmptyValue(op["pitch"]) &&
		!isEmptyValue(op["start"]) &&
		!isEmptyValue(op["length"])
}

func operationRowsFromAny(v any) []map[string]any {
	switch rows := v.(type) {
	case map[string]any:
		return []map[string]any{rows}
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

func insertNoteOperations(cmd map[string]any) []map[string]any {
	notes := operationRowsFromAny(cmd["notes"])
	out := make([]map[string]any, 0, len(notes))
	for _, note := range notes {
		row := cloneAnyMap(note)
		row["op"] = "insert_note"
		out = append(out, row)
	}
	return out
}

func midiOperationFromArgs(op string, cmd map[string]any) map[string]any {
	row := cloneAnyMap(cmd)
	row["op"] = op
	delete(row, "cmd")
	delete(row, "clip_id")
	delete(row, "track_id")
	delete(row, "time_unit")
	delete(row, "undo_label")
	delete(row, "agent_action_id")
	delete(row, "operations")
	return row
}

func inferTimeUnit(cmd map[string]any) {
	if !isEmptyValue(cmd["time_unit"]) {
		return
	}
	for _, key := range []string{"new_start_beats", "new_start_beat", "start_beats", "new_length_beats", "new_length_beat", "length_beats", "duration_beats", "offset_in_source_beats", "offset_in_source_beat", "split_time_beats", "split_time_beat"} {
		if !isEmptyValue(cmd[key]) {
			cmd["time_unit"] = "beats"
			return
		}
	}
	for _, key := range []string{"new_start", "new_start_seconds", "start", "start_seconds", "position_seconds", "time", "new_length", "new_length_seconds", "length", "length_seconds", "duration", "duration_seconds", "offset_in_source", "offset_in_source_seconds", "split_time", "split_time_seconds"} {
		if !isEmptyValue(cmd[key]) {
			cmd["time_unit"] = "seconds"
			return
		}
	}
}

func normalizeClipIDsArray(cmd map[string]any) {
	if ids := stringSliceFromAny(cmd["clip_ids"]); len(ids) > 0 {
		cmd["clip_ids"] = ids
		return
	}
	if id := firstString(cmd, "clip_id", "source_clip_id", "target_clip_id"); id != "" {
		cmd["clip_ids"] = []string{id}
	}
}

var (
	audioPathPattern       = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))`)
	quotedAudioPathPattern = regexp.MustCompile(`(?i)["'\x{201c}\x{201d}\x{2018}\x{2019}]([^"'\x{201c}\x{201d}\x{2018}\x{2019}]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))["'\x{201c}\x{201d}\x{2018}\x{2019}]?`)
	midiPathPattern        = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:mid|midi))`)
	errAudioSearchLimit    = errors.New("audio search limit reached")
)

var audioExtensions = map[string]bool{
	".wav":  true,
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".oga":  true,
	".aif":  true,
	".aiff": true,
	".m4a":  true,
	".wma":  true,
}

var midiExtensions = map[string]bool{
	".mid":  true,
	".midi": true,
}

func (h *Harness) resolveImportAudioSource(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	_ = ctx
	if spec.CommandName != "import_audio" && spec.CommandName != "import_media_to_track" {
		return nil
	}

	if isEmptyValue(cmd["file_path"]) {
		if fp := selectedLibraryFilePathFromContext(requestContext); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		if fp := firstAudioPathInText(firstString(requestContext, "user_message", "message", "prompt", "utterance")); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		query := importSearchQuery(cmd, requestContext)
		if query != "" {
			fp, err := findAudioFileInRoots(query, importSearchRoots(cmd, requestContext))
			if err != nil {
				return err
			}
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		return fmt.Errorf("missing audio file path; provide an absolute path, select an audio file in the library, or include a library search query")
	}

	resolved, err := resolveExistingAudioPath(firstString(cmd, "file_path"), cmd, requestContext)
	if err != nil {
		return err
	}
	cmd["file_path"] = resolved

	if spec.CommandName == "import_media_to_track" {
		if isEmptyValue(cmd["media_type"]) {
			cmd["media_type"] = "audio"
		}
		if isEmptyValue(cmd["mode"]) {
			cmd["mode"] = "non_destructive"
		}
		if isEmptyValue(cmd["start_time"]) {
			cmd["start_time"] = 0.0
		}
	}
	return nil
}

func (h *Harness) resolveImportMidiSource(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	_ = ctx
	if spec.CommandName != "import_midi_to_track" {
		return nil
	}
	if isEmptyValue(cmd["file_path"]) {
		if fp := selectedMidiAttachmentPathFromContext(requestContext); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		if fp := selectedMidiLibraryFilePathFromContext(requestContext); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		if fp := firstMidiPathInText(firstString(requestContext, "user_message", "message", "prompt", "utterance")); fp != "" {
			cmd["file_path"] = fp
		}
	}
	if isEmptyValue(cmd["file_path"]) {
		return fmt.Errorf("missing MIDI file path; attach a MIDI file, select a MIDI file in the library, or provide an absolute .mid/.midi path")
	}
	resolved, err := resolveExistingMidiPath(firstString(cmd, "file_path"))
	if err != nil {
		return err
	}
	cmd["file_path"] = resolved
	if isEmptyValue(cmd["start_time_beats"]) {
		cmd["start_time_beats"] = 0.0
	}
	if isEmptyValue(cmd["mode"]) {
		cmd["mode"] = "merge_tracks"
	}
	return nil
}

func selectedMidiAttachmentPathFromContext(requestContext map[string]any) string {
	if strings.EqualFold(firstString(requestContext, "attachment_import_kind", "selected_attachment_kind"), "midi") {
		if fp := firstString(requestContext, "attachment_import_file_path", "selected_attachment_file_path"); fp != "" {
			return fp
		}
	}
	for _, row := range mapRowsFromAny(requestContext["attachments"]) {
		if strings.EqualFold(firstString(row, "kind"), "midi") {
			if fp := firstString(row, "path", "file_path"); fp != "" {
				return fp
			}
		}
	}
	return ""
}

func selectedMidiLibraryFilePathFromContext(requestContext map[string]any) string {
	if strings.EqualFold(firstString(requestContext, "selected_library_kind"), "midi") {
		if fp := firstString(requestContext, "selected_library_file_path", "library_file_path"); fp != "" {
			return fp
		}
	}
	if fp := firstString(requestContext, "selected_library_file_path", "library_file_path"); fp != "" && isSupportedMidiPath(fp) {
		return fp
	}
	return ""
}

func selectedLibraryFilePathFromContext(requestContext map[string]any) string {
	if fp := firstString(requestContext, "selected_library_file_path", "library_file_path"); fp != "" {
		return fp
	}
	for _, key := range []string{"selected_library_item", "selected_library_resource", "library_item"} {
		item, ok := requestContext[key].(map[string]any)
		if !ok {
			continue
		}
		if fp := firstString(item, "file_path", "path"); fp != "" {
			return fp
		}
	}
	return ""
}

func resolveExistingAudioPath(raw string, cmd map[string]any, requestContext map[string]any) (string, error) {
	path := normalizeAudioPath(raw)
	if path == "" {
		return "", fmt.Errorf("audio file path is empty")
	}
	if !isSupportedAudioPath(path) {
		return "", fmt.Errorf("unsupported audio file extension: %s", path)
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		abs, absErr := filepath.Abs(path)
		if absErr == nil {
			return filepath.Clean(abs), nil
		}
		return filepath.Clean(path), nil
	}
	if !filepath.IsAbs(path) {
		if found, err := findAudioFileInRoots(path, importSearchRoots(cmd, requestContext)); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("audio file does not exist: %s", path)
}

func normalizeAudioPath(path string) string {
	path = trimPathPunctuation(path)
	if path == "" {
		return ""
	}
	path = filepath.FromSlash(path)
	return filepath.Clean(path)
}

func trimPathPunctuation(path string) string {
	return strings.Trim(path, " \t\r\n\"'`\u201c\u201d\u2018\u2019.,\uff0c\u3002;\uff1b:\uff1a)\uff09]\u3011")
}

func isSupportedAudioPath(path string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(path))]
}

func isSupportedMidiPath(path string) bool {
	return midiExtensions[strings.ToLower(filepath.Ext(path))]
}

func resolveExistingMidiPath(raw string) (string, error) {
	path := normalizeAudioPath(raw)
	if path == "" {
		return "", fmt.Errorf("MIDI file path is empty")
	}
	if !isSupportedMidiPath(path) {
		return "", fmt.Errorf("unsupported MIDI file extension: %s", path)
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		abs, absErr := filepath.Abs(path)
		if absErr == nil {
			return filepath.Clean(abs), nil
		}
		return filepath.Clean(path), nil
	}
	return "", fmt.Errorf("MIDI file does not exist: %s", path)
}

func firstAudioPathInText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, pattern := range []*regexp.Regexp{quotedAudioPathPattern, audioPathPattern} {
		match := pattern.FindStringSubmatch(text)
		if len(match) > 1 {
			return trimPathPunctuation(match[1])
		}
	}
	return ""
}

func firstMidiPathInText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	match := midiPathPattern.FindStringSubmatch(text)
	if len(match) > 1 {
		return trimPathPunctuation(match[1])
	}
	return ""
}

func importSearchQuery(cmd map[string]any, requestContext map[string]any) string {
	if q := firstString(cmd, "asset_query", "search_query", "file_query", "query", "search", "file_name"); q != "" {
		return q
	}
	return inferImportSearchQuery(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
}

func inferImportSearchQuery(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || firstAudioPathInText(text) != "" {
		return ""
	}
	if !containsTextAnyFold(text, "\u641c\u7d22", "\u67e5\u627e", "\u627e", "search", "find") {
		return ""
	}
	cleaned := text
	for _, phrase := range []string{
		"\u641c\u7d22", "\u67e5\u627e", "\u627e\u4e00\u4e0b", "\u627e\u5230", "\u627e", "\u5e76\u5bfc\u5165", "\u7136\u540e\u5bfc\u5165", "\u5bfc\u5165", "\u653e\u5230", "\u653e\u8fdb", "\u62d6\u5165",
		"\u8d44\u6599\u5e93", "\u7d20\u6750\u5e93", "\u5f53\u524d\u8f68\u9053", "\u9009\u4e2d\u8f68\u9053", "\u8fd9\u4e2a\u8f68\u9053", "\u8fd9\u6761\u8f68\u9053", "\u97f3\u9891", "\u7d20\u6750",
		"search", "find", "import", "add", "audio", "sample", "current track", "selected track", "track", "and", "to",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
		cleaned = strings.ReplaceAll(cleaned, strings.Title(phrase), " ")
	}
	replacer := strings.NewReplacer("\uff0c", " ", "\u3002", " ", ",", " ", ".", " ", "\uff1b", " ", ";", " ", "\uff1a", " ", ":", " ", "\uff08", " ", "\uff09", " ", "(", " ", ")", " ", "\u5e76", " ", "\u5230", " ", "\u7ed9", " ")
	cleaned = replacer.Replace(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

func importSearchRoots(cmd map[string]any, requestContext map[string]any) []string {
	var roots []string
	for _, key := range []string{"search_roots", "library_places", "library_roots", "places"} {
		roots = append(roots, stringSliceFromAny(cmd[key])...)
		roots = append(roots, stringSliceFromAny(requestContext[key])...)
	}
	for _, key := range []string{"selected_library_root", "library_root"} {
		if root := firstString(cmd, key); root != "" {
			roots = append(roots, root)
		}
		if root := firstString(requestContext, key); root != "" {
			roots = append(roots, root)
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(roots))
	for _, root := range roots {
		root = normalizeAudioPath(root)
		if root == "" || seen[strings.ToLower(root)] {
			continue
		}
		seen[strings.ToLower(root)] = true
		out = append(out, root)
	}
	return out
}

type audioFileMatch struct {
	path  string
	score int
}

func findAudioFileInRoots(query string, roots []string) (string, error) {
	terms := importSearchTerms(query)
	if len(terms) == 0 {
		return "", fmt.Errorf("empty audio search query")
	}
	if len(roots) == 0 {
		return "", fmt.Errorf("library search needs at least one Places root")
	}

	matches := make([]audioFileMatch, 0)
	const maxScannedFiles = 6000
	scanned := 0
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		root = filepath.Clean(root)
		scanErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(filepath.Base(path), ".") {
					return filepath.SkipDir
				}
				if directoryDepth(root, path) > 8 {
					return filepath.SkipDir
				}
				return nil
			}
			scanned++
			if scanned > maxScannedFiles {
				return errAudioSearchLimit
			}
			if !isSupportedAudioPath(path) {
				return nil
			}
			if score := audioSearchScore(filepath.Base(path), terms); score > 0 {
				matches = append(matches, audioFileMatch{path: filepath.Clean(path), score: score})
			}
			return nil
		})
		if scanErr != nil && !errors.Is(scanErr, errAudioSearchLimit) {
			return "", scanErr
		}
		if errors.Is(scanErr, errAudioSearchLimit) {
			break
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no audio file matched %q in library Places", query)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return len(matches[i].path) < len(matches[j].path)
	})
	if len(matches) > 1 && matches[0].score == matches[1].score {
		return "", fmt.Errorf("multiple audio files matched %q; specify one path, e.g. %s or %s", query, matches[0].path, matches[1].path)
	}
	return matches[0].path, nil
}

func importSearchTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	query = strings.TrimSuffix(query, strings.ToLower(filepath.Ext(query)))
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ", ",", " ", "\uff0c", " ", "\u3002", " ", "/", " ", "\\", " ")
	query = replacer.Replace(query)
	fields := strings.Fields(query)
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func audioSearchScore(fileName string, terms []string) int {
	name := strings.ToLower(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
	score := 0
	for _, term := range terms {
		if !strings.Contains(name, term) {
			return 0
		}
		score += 10 + len(term)
		if name == term {
			score += 50
		}
	}
	joined := strings.Join(terms, " ")
	if name == joined {
		score += 100
	} else if strings.HasPrefix(name, joined) {
		score += 30
	}
	return score
}

func directoryDepth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}
	return len(strings.Split(rel, string(os.PathSeparator)))
}

func inferBoolFromUserMessage(cmd map[string]any, target string, requestContext map[string]any, negativePhrases, positivePhrases []string) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if text != "" {
		if value, ok := inferBoolFromText(text, negativePhrases, positivePhrases); ok {
			cmd[target] = value
			return
		}
	}
	if value, ok := boolValue(cmd[target]); ok {
		cmd[target] = value
		return
	}
}

func inferBoolFromText(text string, negativePhrases, positivePhrases []string) (bool, bool) {
	lower := strings.ToLower(text)
	stripped := lower
	hasNegative := false
	for _, phrase := range negativePhrases {
		p := strings.ToLower(strings.TrimSpace(phrase))
		if p == "" {
			continue
		}
		if strings.Contains(lower, p) {
			hasNegative = true
			stripped = strings.ReplaceAll(stripped, p, " ")
		}
	}
	hasPositive := false
	for _, phrase := range positivePhrases {
		p := strings.ToLower(strings.TrimSpace(phrase))
		if p != "" && strings.Contains(stripped, p) {
			hasPositive = true
			break
		}
	}
	if hasPositive && hasNegative {
		return false, false
	}
	if hasPositive {
		return true, true
	}
	if hasNegative {
		return false, true
	}
	return false, false
}

func copyFirstNonEmpty(cmd map[string]any, target string, aliases ...string) {
	if !isEmptyValue(cmd[target]) {
		return
	}
	for _, key := range aliases {
		if !isEmptyValue(cmd[key]) {
			cmd[target] = cmd[key]
			return
		}
	}
}

func copyFirstNonEmptyFrom(dst map[string]any, target string, src map[string]any, aliases ...string) {
	if !isEmptyValue(dst[target]) {
		return
	}
	for _, key := range aliases {
		if !isEmptyValue(src[key]) {
			dst[target] = src[key]
			return
		}
	}
}

func copyBoolAlias(cmd map[string]any, target string, aliases ...string) {
	if value, ok := boolValue(cmd[target]); ok {
		cmd[target] = value
		return
	}
	for _, key := range aliases {
		if value, ok := boolValue(cmd[key]); ok {
			cmd[target] = value
			return
		}
	}
}

func boolValue(v any) (bool, bool) {
	if isEmptyValue(v) {
		return false, false
	}
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "enable", "enabled":
			return true, true
		case "false", "0", "no", "off", "disable", "disabled":
			return false, true
		}
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	case float64:
		return x != 0, true
	case json.Number:
		n, err := strconv.Atoi(x.String())
		if err == nil {
			return n != 0, true
		}
	}
	return false, false
}

type clipRef struct {
	ID             string
	Name           string
	TrackID        string
	TrackName      string
	UserTrackIndex int
	Row            map[string]any
}

func (h *Harness) resolveClipTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	switch spec.CommandName {
	case "move_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["new_start"]) {
			if mentionsPlayheadTarget(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
				if seconds, ok := firstPlayheadSeconds(cmd, requestContext); ok {
					cmd["new_start"] = seconds
					if isEmptyValue(cmd["time_unit"]) {
						cmd["time_unit"] = "seconds"
					}
				}
			}
		}
		if isEmptyValue(cmd["new_start"]) {
			if seconds, ok := inferMoveStartSeconds(ref, requestContext); ok {
				cmd["new_start"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["source_track_id"]) {
			cmd["source_track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["target_track_id"]) {
			targetTrackID, err := h.resolveTargetTrackID(ctx, spec, cmd, requestContext)
			if err != nil {
				return err
			}
			if targetTrackID == "" {
				targetTrackID = ref.TrackID
			}
			cmd["target_track_id"] = targetTrackID
		}
	case "resize_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["new_length"]) {
			if seconds, ok := inferLengthSeconds(requestContext); ok {
				cmd["new_length"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
	case "split_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if mentionsPlayheadSplit(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
			if seconds, ok := firstPlayheadSeconds(cmd, requestContext); ok {
				cmd["split_time"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["split_time"]) {
			if seconds, ok := inferSplitTimeSeconds(ref, requestContext); ok {
				cmd["split_time"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["split_time"]) {
			return fmt.Errorf("split_clip requires split_time; say a time like 鍦?2 绉掑鍒囧紑, or move the playhead and say 鍦ㄦ挱鏀惧ご鍒囧紑")
		}
		if isEmptyValue(cmd["time_unit"]) {
			cmd["time_unit"] = "seconds"
		}
	case "clone_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "source_clip_id")
		if err != nil {
			return err
		}
		cmd["source_clip_id"] = ref.ID
		if isEmptyValue(cmd["new_start"]) {
			if mentionsPlayheadTarget(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
				if seconds, ok := firstPlayheadSeconds(cmd, requestContext); ok {
					cmd["new_start"] = seconds
					if isEmptyValue(cmd["time_unit"]) {
						cmd["time_unit"] = "seconds"
					}
				}
			}
		}
		if isEmptyValue(cmd["new_start"]) {
			if seconds, ok := inferCloneStartSeconds(ref, requestContext); ok {
				cmd["new_start"] = seconds
				if isEmptyValue(cmd["time_unit"]) {
					cmd["time_unit"] = "seconds"
				}
			}
		}
		if isEmptyValue(cmd["target_track_id"]) {
			targetTrackID, err := h.resolveTargetTrackID(ctx, spec, cmd, requestContext)
			if err != nil {
				return err
			}
			if targetTrackID == "" {
				targetTrackID = ref.TrackID
			}
			cmd["target_track_id"] = targetTrackID
		}
		if isEmptyValue(cmd["new_start"]) {
			return fmt.Errorf("clone_clip requires new_start; say where to place the copy, e.g. duplicate after 20 seconds")
		}
		if isEmptyValue(cmd["time_unit"]) {
			cmd["time_unit"] = "seconds"
		}
	case "remove_clips":
		if ids := stringSliceFromAny(cmd["clip_ids"]); len(ids) > 0 {
			cmd["clip_ids"] = ids
			return nil
		}
		if ids := selectedClipIDsFromContext(requestContext); len(ids) > 0 {
			cmd["clip_ids"] = ids
			return nil
		}
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_ids"] = []string{ref.ID}
	case "select_clip":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
		if isEmptyValue(cmd["clip_name"]) && ref.Name != "" {
			cmd["clip_name"] = ref.Name
		}
		if isEmptyValue(cmd["track_name"]) && ref.TrackName != "" {
			cmd["track_name"] = ref.TrackName
		}
	case "clip.fade.set", "clip.fade.read", "clip.gain.set", "clip.gain.read":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
	case "get_midi_clip_notes", "get_midi_clip_data", "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes", "apply_midi_note_patch":
		ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
		if err != nil {
			return err
		}
		cmd["clip_id"] = ref.ID
		if isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
			cmd["track_id"] = ref.TrackID
		}
		if isLegacyMidiWriteCommand(spec.CommandName) && isEmptyValue(cmd["time_unit"]) {
			cmd["time_unit"] = "beats"
		}
		if spec.CommandName == "apply_midi_note_patch" {
			if isEmptyValue(cmd["time_unit"]) {
				cmd["time_unit"] = "beats"
			}
			if len(operationRowsFromAny(cmd["operations"])) == 0 {
				return fmt.Errorf("apply_midi_note_patch requires operations")
			}
		}
	default:
		if specRequiresTarget(spec, "clip_id") && isEmptyValue(cmd["clip_id"]) {
			ref, err := h.resolveSingleClipRef(ctx, spec, cmd, requestContext, "clip_id")
			if err != nil {
				return err
			}
			cmd["clip_id"] = ref.ID
			if specRequiresTarget(spec, "track_id") && isEmptyValue(cmd["track_id"]) && ref.TrackID != "" {
				cmd["track_id"] = ref.TrackID
			}
		}
	}
	return nil
}

func (h *Harness) resolveTargetTrackID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if targetTrackID := firstString(cmd, "target_track_id"); targetTrackID != "" {
		return targetTrackID, nil
	}
	if trackID := firstString(cmd, "track_id"); trackID != "" {
		return trackID, nil
	}
	if hasTargetTrackHint(cmd) {
		return h.resolveTrackID(ctx, spec, cmd, requestContext)
	}
	return "", nil
}

func hasTargetTrackHint(cmd map[string]any) bool {
	if firstString(cmd, "target_track_name", "target_track") != "" {
		return true
	}
	_, ok := firstPositiveInt(cmd, "target_track_index", "target_track_number", "target_user_track_index")
	return ok
}

func (h *Harness) resolveSingleClipRef(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any, field string) (clipRef, error) {
	state := h.UserStateSummary(ctx)
	refs := visibleClipRefs(state)
	if id := firstString(cmd, field, "clip_id", "source_clip_id", "target_clip_id"); id != "" {
		return clipRefByIDOrContext(id, refs, requestContext)
	}

	if name := firstString(cmd, "clip_name", "target_clip_name", "source_clip_name", "clip"); name != "" {
		scope, _ := explicitScopedClipRefs(refs, cmd)
		matches := filterClipRefsByName(scope, name)
		switch len(matches) {
		case 1:
			return matches[0], nil
		case 0:
			return clipRef{}, fmt.Errorf("no user-visible clip named %q", name)
		default:
			return clipRef{}, fmt.Errorf("multiple user-visible clips named %q; specify track or clip_index", name)
		}
	}

	if index, ok := firstPositiveInt(cmd, "clip_index", "clip_number", "user_clip_index"); ok {
		scope, hasScope := explicitScopedClipRefs(refs, cmd)
		if !hasScope {
			scope, hasScope = contextScopedClipRefs(refs, requestContext)
		}
		if len(scope) == 0 && !hasScope {
			scope = refs
		}
		if index < 1 || index > len(scope) {
			return clipRef{}, fmt.Errorf("clip_index %d is out of range", index)
		}
		return scope[index-1], nil
	}

	if scope, hasScope := explicitScopedClipRefs(refs, cmd); hasScope {
		if len(scope) == 1 {
			return scope[0], nil
		}
		if len(scope) > 1 {
			return clipRef{}, fmt.Errorf("multiple clips exist on the selected track; select a clip or specify clip_index")
		}
		return clipRef{}, fmt.Errorf("there are no clips on the selected track")
	}

	selected := selectedClipIDsFromContext(requestContext)
	if len(selected) == 1 {
		return clipRefByIDOrContext(selected[0], refs, requestContext)
	}
	if len(selected) > 1 {
		return clipRef{}, fmt.Errorf("multiple clips are selected; specify which clip or use a multi-clip command")
	}

	if scope, hasScope := contextScopedClipRefs(refs, requestContext); hasScope {
		if len(scope) == 1 {
			return scope[0], nil
		}
		if len(scope) > 1 {
			return clipRef{}, fmt.Errorf("multiple clips exist on the selected track; select a clip or specify clip_index")
		}
		return clipRef{}, fmt.Errorf("there are no clips on the selected track")
	}

	switch len(refs) {
	case 0:
		return clipRef{}, fmt.Errorf("there are no user-visible clips")
	case 1:
		return refs[0], nil
	default:
		return clipRef{}, fmt.Errorf("multiple user-visible clips exist; select a clip or specify clip name/clip_index")
	}
}

func clipRefByIDOrContext(id string, refs []clipRef, requestContext map[string]any) (clipRef, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return clipRef{}, fmt.Errorf("clip_id is empty")
	}
	for _, ref := range refs {
		if ref.ID == id {
			return ref, nil
		}
	}
	for _, selectedID := range selectedClipIDsFromContext(requestContext) {
		if selectedID == id {
			return clipRef{
				ID:      id,
				TrackID: firstString(requestContext, "selected_clip_track_id"),
			}, nil
		}
	}
	return clipRef{}, fmt.Errorf("clip_id %q is not present in the current user-visible project state", id)
}

func explicitClipScopeTrackID(cmd map[string]any) string {
	if trackID := firstString(cmd, "track_id", "source_track_id"); trackID != "" {
		return trackID
	}
	return ""
}

func contextClipScopeTrackID(requestContext map[string]any) string {
	return firstString(requestContext, "selected_clip_track_id", "focused_track_id", "selected_track_id", "track_id")
}

func explicitScopedClipRefs(refs []clipRef, cmd map[string]any) ([]clipRef, bool) {
	if trackID := explicitClipScopeTrackID(cmd); trackID != "" {
		return filterClipRefsByTrack(refs, trackID), true
	}
	if index, ok := explicitClipScopeUserTrackIndex(cmd); ok {
		return filterClipRefsByUserTrackIndex(refs, index), true
	}
	return refs, false
}

func contextScopedClipRefs(refs []clipRef, requestContext map[string]any) ([]clipRef, bool) {
	if trackID := contextClipScopeTrackID(requestContext); trackID != "" {
		return filterClipRefsByTrack(refs, trackID), true
	}
	return refs, false
}

func explicitClipScopeUserTrackIndex(cmd map[string]any) (int, bool) {
	return firstPositiveInt(cmd, "user_track_index", "source_track_index", "source_user_track_index", "track_index", "track_number")
}

func visibleClipRefs(state map[string]any) []clipRef {
	tracks := visibleTrackRows(state)
	out := make([]clipRef, 0)
	for _, track := range tracks {
		trackID := visibleTrackID(track)
		trackName := visibleTrackName(track)
		userIndex, _ := firstPositiveInt(track, "user_track_index")
		for _, clip := range mapRowsFromAny(track["clips"]) {
			id := firstString(clip, "clip_id", "id", "item_id")
			if id == "" {
				continue
			}
			out = append(out, clipRef{
				ID:             id,
				Name:           firstString(clip, "name", "clip_name"),
				TrackID:        trackID,
				TrackName:      trackName,
				UserTrackIndex: userIndex,
				Row:            clip,
			})
		}
	}
	return out
}

func filterClipRefsByName(refs []clipRef, name string) []clipRef {
	name = strings.TrimSpace(name)
	out := make([]clipRef, 0, len(refs))
	for _, ref := range refs {
		if strings.EqualFold(ref.Name, name) {
			out = append(out, ref)
		}
	}
	return out
}

func filterClipRefsByTrack(refs []clipRef, trackID string) []clipRef {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return refs
	}
	out := make([]clipRef, 0, len(refs))
	for _, ref := range refs {
		if ref.TrackID == trackID {
			out = append(out, ref)
		}
	}
	return out
}

func filterClipRefsByUserTrackIndex(refs []clipRef, index int) []clipRef {
	if index <= 0 {
		return refs
	}
	out := make([]clipRef, 0, len(refs))
	for _, ref := range refs {
		if ref.UserTrackIndex == index {
			out = append(out, ref)
		}
	}
	return out
}

func selectedClipIDsFromContext(requestContext map[string]any) []string {
	if len(requestContext) == 0 {
		return nil
	}
	if ids := stringSliceFromAny(requestContext["selected_clip_ids"]); len(ids) > 0 {
		return ids
	}
	if id := firstString(requestContext, "selected_clip_id", "primary_selected_clip_id", "clip_id"); id != "" {
		return []string{id}
	}
	return nil
}

func isLegacyMidiWriteCommand(commandName string) bool {
	switch commandName {
	case "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes":
		return true
	default:
		return false
	}
}

func specRequiresTarget(spec tools.CommandSpec, field string) bool {
	for _, id := range spec.RequiredTargetIDs {
		if id == field {
			return true
		}
	}
	return false
}

var secondsInTextPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|\x{79d2})`)

func inferMoveStartSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	seconds, ok := firstSecondsInText(text)
	if !ok {
		return 0, false
	}
	lower := strings.ToLower(text)
	current := numberFromAny(ref.Row["start_seconds"])
	switch {
	case strings.Contains(text, "\u5411\u524d") || strings.Contains(text, "\u524d\u79fb") || strings.Contains(text, "\u63d0\u524d") || strings.Contains(lower, "earlier") || strings.Contains(lower, "left") || strings.Contains(lower, "backward"):
		next := current - seconds
		if next < 0 {
			next = 0
		}
		return next, true
	case strings.Contains(text, "\u5411\u540e") || strings.Contains(text, "\u540e\u79fb") || strings.Contains(text, "\u63a8\u540e") || strings.Contains(lower, "later") || strings.Contains(lower, "right") || strings.Contains(lower, "forward"):
		return current + seconds, true
	default:
		return seconds, true
	}
}

func inferCloneStartSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	if seconds, ok := inferMoveStartSeconds(ref, requestContext); ok {
		return seconds, true
	}
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if strings.TrimSpace(text) == "" || isCloneAfterText(text) || containsTextAnyFold(text, "\u590d\u5236", "\u514b\u9686", "\u62f7\u8d1d", "duplicate", "clone", "copy") {
		start := numberFromAny(ref.Row["start_seconds"])
		length := numberFromAny(ref.Row["length_seconds"])
		if length <= 0 {
			end := numberFromAny(ref.Row["end_seconds"])
			if end > start {
				length = end - start
			}
		}
		if length > 0 {
			return start + length, true
		}
	}
	return 0, false
}

func inferSplitTimeSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if seconds, ok := firstSecondsInText(text); ok {
		return splitTimeFromTextSeconds(ref, seconds, text), true
	}
	if mentionsPlayheadSplit(text) {
		if seconds, ok := firstPlayheadSeconds(nil, requestContext); ok {
			return seconds, true
		}
	}
	return 0, false
}

func firstPlayheadSeconds(cmd map[string]any, requestContext map[string]any) (float64, bool) {
	keys := []string{"playhead_seconds", "current_playhead_seconds", "transport_position_seconds", "position_seconds"}
	if seconds, ok := numberValueFromMap(cmd, keys...); ok {
		return seconds, true
	}
	return numberValueFromMap(requestContext, keys...)
}

func splitTimeFromTextSeconds(ref clipRef, seconds float64, text string) float64 {
	lower := strings.ToLower(text)
	if containsTextAnyFold(text, "\u5185\u90e8", "\u91cc\u9762", "\u4ece\u5f00\u5934", "\u4ece\u8d77\u70b9", "\u76f8\u5bf9", "offset", "into clip", "from start") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if containsTextAnyFold(text, "\u540e", "\u4e4b\u540e", "later", "after") && !containsTextAnyFold(text, "\u79d2\u5904", "s mark", "at") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if strings.Contains(lower, "relative") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	return seconds
}

func mentionsPlayheadSplit(text string) bool {
	return containsTextAnyFold(text, "\u64ad\u653e\u5934", "\u5f53\u524d\u4f4d\u7f6e", "\u8fd9\u91cc", "\u6b64\u5904", "\u5f53\u524d\u65f6\u95f4", "playhead", "cursor", "current position", "here")
}

func mentionsPlayheadTarget(text string) bool {
	return containsTextAnyFold(text, "\u64ad\u653e\u5934", "\u5f53\u524d\u4f4d\u7f6e", "\u8fd9\u91cc", "\u6b64\u5904", "\u5f53\u524d\u65f6\u95f4", "playhead", "cursor", "current position", "here")
}

func isCloneAfterText(text string) bool {
	return containsTextAnyFold(text, "\u540e\u9762", "\u540e\u8fb9", "\u540e\u65b9", "\u540e\u9762\u4e00\u4efd", "\u540e\u7eed", "\u7d27\u63a5", "\u540e", "after", "next", "right after")
}

func inferLengthSeconds(requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if !isResizeLengthText(text) {
		return 0, false
	}
	return firstSecondsInText(text)
}

func isResizeLengthText(text string) bool {
	if containsTextAnyFold(text, "\u4f4d\u7f6e", "\u8d77\u70b9", "\u5f00\u59cb\u4f4d\u7f6e", "\u79fb\u52a8", "\u79fb\u5230", "\u632a\u5230", "\u62d6\u5230", "move", "position", "start") {
		return false
	}
	return containsTextAnyFold(text,
		"\u957f\u5ea6", "\u65f6\u957f", "\u6301\u7eed", "\u88c1\u5230", "\u88c1\u6210", "\u526a\u5230", "\u526a\u6210", "\u526a\u77ed",
		"\u7f29\u5230", "\u7f29\u77ed", "\u62c9\u957f", "\u62c9\u5230", "\u4f38\u5230", "\u6539\u6210", "\u6539\u4e3a", "\u53d8\u6210",
		"\u8c03\u6574\u5230", "\u8c03\u6210", "resize", "length", "duration", "trim", "shorten")
}

func containsTextAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func (h *Harness) applyStaticBalanceBatch(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("mix.apply_static_balance_batch requires a connected kernel")
	}
	rows := mapRowsFromAny(cmd["actions"])
	if len(rows) == 0 {
		return nil, fmt.Errorf("mix.apply_static_balance_batch requires actions")
	}
	seen := map[string]bool{}
	kernelActions := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		args := mapFromAny(row["args"])
		if len(args) == 0 {
			args = row
		}
		trackID := firstNonEmpty(firstString(args, "track_id"), firstString(row, "track_id"))
		targetDB, ok := numberValueFromMap(args, "target_db", "db", "volume_db")
		if !ok {
			targetDB, ok = numberValueFromMap(row, "target_db", "db", "volume_db")
		}
		if trackID == "" || !ok || math.IsNaN(targetDB) || math.IsInf(targetDB, 0) {
			return nil, fmt.Errorf("mix.apply_static_balance_batch action requires track_id and finite target_db")
		}
		if seen[trackID] {
			return nil, fmt.Errorf("mix.apply_static_balance_batch contains duplicate track_id: %s", trackID)
		}
		seen[trackID] = true
		kernelActions = append(kernelActions, map[string]any{
			"track_id":  trackID,
			"target_db": targetDB,
		})
	}

	batchSpec, ok := h.catalog.LookupTool("track.volume")
	if !ok {
		return nil, fmt.Errorf("track.volume is not cataloged")
	}
	batchSpec.CommandName = "track.volume.set_batch"
	batchSpec.ToolName = "mix.apply_static_balance_batch"
	batchSpec.RefreshAfter = false
	batchCmd := map[string]any{
		"cmd":               "track.volume.set_batch",
		"actions":           kernelActions,
		"observation_id":    firstString(cmd, "observation_id"),
		"candidate_plan_id": firstString(cmd, "candidate_plan_id"),
	}
	execution := h.executeKernelCommand(ctx, batchSpec, batchCmd)
	if execution.err != nil {
		return nil, execution.err
	}
	if !kernelReplySucceeded(execution.reply) {
		return nil, fmt.Errorf("%s", firstNonEmpty(firstString(execution.reply, "message"), firstString(execution.reply, "error"), "track.volume.set_batch failed"))
	}
	h.refreshShadow(ctx, "mix.apply_static_balance_batch")

	out := cloneAnyMap(execution.reply)
	out["ui_action"] = "static_balance_batch_applied"
	out["observation_id"] = firstString(cmd, "observation_id")
	out["candidate_plan_id"] = firstString(cmd, "candidate_plan_id")
	out["internal_apply_tool"] = "track.volume.set_batch"
	out["internal_apply_count"] = len(kernelActions)
	return out, nil
}

func (h *Harness) applyPanLayoutBatch(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("mix.apply_pan_layout_batch requires a connected kernel")
	}
	rows := mapRowsFromAny(cmd["actions"])
	if len(rows) == 0 {
		return nil, fmt.Errorf("mix.apply_pan_layout_batch requires actions")
	}
	seen := map[string]bool{}
	kernelActions := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		args := mapFromAny(row["args"])
		if len(args) == 0 {
			args = row
		}
		trackID := firstNonEmpty(firstString(args, "track_id"), firstString(row, "track_id"))
		target, ok := numberValueFromMap(args, "target_pan", "pan", "pan_value")
		if !ok {
			target, ok = numberValueFromMap(row, "target_pan", "pan", "pan_value")
		}
		if trackID == "" || !ok || math.IsNaN(target) || math.IsInf(target, 0) || target < -1 || target > 1 {
			return nil, fmt.Errorf("mix.apply_pan_layout_batch action requires track_id and target_pan in [-1,1]")
		}
		if seen[trackID] {
			return nil, fmt.Errorf("mix.apply_pan_layout_batch contains duplicate track_id: %s", trackID)
		}
		seen[trackID] = true
		kernelActions = append(kernelActions, map[string]any{"track_id": trackID, "target_pan": target})
	}
	batchSpec, ok := h.catalog.LookupTool("track.pan")
	if !ok {
		return nil, fmt.Errorf("track.pan is not cataloged")
	}
	batchSpec.CommandName = "track.pan.set_batch"
	batchSpec.ToolName = "mix.apply_pan_layout_batch"
	batchSpec.RefreshAfter = false
	batchCmd := map[string]any{"cmd": "track.pan.set_batch", "actions": kernelActions, "observation_id": firstString(cmd, "observation_id"), "candidate_plan_id": firstString(cmd, "candidate_plan_id"), "style_hash": firstString(cmd, "style_hash")}
	execution := h.executeKernelCommand(ctx, batchSpec, batchCmd)
	if execution.err != nil {
		return nil, execution.err
	}
	if !kernelReplySucceeded(execution.reply) {
		return nil, fmt.Errorf("%s", firstNonEmpty(firstString(execution.reply, "message"), firstString(execution.reply, "error"), "track.pan.set_batch failed"))
	}
	h.refreshShadow(ctx, "mix.apply_pan_layout_batch")
	out := cloneAnyMap(execution.reply)
	out["ui_action"] = "pan_layout_batch_applied"
	out["observation_id"] = firstString(cmd, "observation_id")
	out["candidate_plan_id"] = firstString(cmd, "candidate_plan_id")
	out["style_hash"] = firstString(cmd, "style_hash")
	out["internal_apply_tool"] = "track.pan.set_batch"
	out["internal_apply_count"] = len(kernelActions)
	return out, nil
}

func firstSecondsInText(text string) (float64, bool) {
	match := secondsInTextPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = -value
	}
	return value, true
}

func (h *Harness) applyClipGainBatch(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("clip.gain.set_batch requires a connected kernel")
	}
	applyCommands := clipGainBatchApplyCommands(cmd)
	if len(applyCommands) == 0 {
		return nil, fmt.Errorf("clip.gain.set_batch requires pending_actions with clip_id and gain_db")
	}
	applySpec, ok := h.catalog.LookupTool("clip.gain.set_batch")
	if !ok {
		return nil, fmt.Errorf("clip.gain.set_batch is not cataloged")
	}
	applySpec.RefreshAfter = false
	batchCmd := tools.CloneCommand(cmd)
	batchCmd["cmd"] = "clip.gain.set_batch"
	batchCmd["pending_actions"] = applyCommands
	execution := h.executeKernelCommand(ctx, applySpec, batchCmd)
	if execution.err != nil {
		return nil, execution.err
	}
	if !kernelReplySucceeded(execution.reply) {
		return nil, fmt.Errorf("%s", firstNonEmpty(firstString(execution.reply, "message"), firstString(execution.reply, "error"), "clip.gain.set_batch failed"))
	}
	h.refreshShadow(ctx, "clip.gain.set_batch")

	out := cloneAnyMap(execution.reply)
	out["ui_action"] = "clip_gain_batch_applied"
	out["internal_apply_tool"] = "clip.gain.set_batch"
	out["internal_apply_count"] = len(applyCommands)
	out["response_granularity"] = "summary"
	return out, nil
}

func clipGainBatchApplyCommands(cmd map[string]any) []map[string]any {
	rows := mapRowsFromAny(cmd["pending_actions"])
	if len(rows) == 0 {
		rows = mapRowsFromAny(cmd["actions"])
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		args := mapFromAny(row["args"])
		if len(args) == 0 {
			args = row
		}
		clipID := firstString(args, "clip_id")
		if clipID == "" {
			clipID = firstString(row, "clip_id")
		}
		gainDB, ok := numberValueFromMap(args, "gain_db", "clip_gain_db", "db")
		if !ok {
			gainDB, ok = numberValueFromMap(row, "gain_db", "clip_gain_db", "target_gain_db")
		}
		if clipID == "" || !ok {
			continue
		}
		applyCmd := tools.CloneCommand(args)
		applyCmd["cmd"] = "clip.gain.set"
		applyCmd["clip_id"] = clipID
		applyCmd["gain_db"] = gainDB
		if trackID := firstNonEmpty(firstString(applyCmd, "track_id"), firstString(row, "track_id")); trackID != "" {
			applyCmd["track_id"] = trackID
		}
		out = append(out, applyCmd)
	}
	return out
}

func numberFromAny(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case json.Number:
		n, _ := x.Float64()
		return n
	default:
		n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
		return n
	}
}

func numberValueFromMap(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || isEmptyValue(v) {
			continue
		}
		switch x := v.(type) {
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		case float64:
			return x, true
		case json.Number:
			n, err := x.Float64()
			return n, err == nil
		default:
			n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
			return n, err == nil
		}
	}
	return 0, false
}

func (h *Harness) resolveTrackID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if h == nil || h.shadow == nil {
		return firstString(requestContext, "selected_plugin_track_id", "selected_track_id", "track_id"), nil
	}
	state := h.UserStateSummary(ctx)
	tracks := visibleTrackRows(state)
	if index, ok := firstPositiveInt(cmd, "user_track_index", "track_index", "track_number", "target_track_index", "target_track_number", "target_user_track_index", "index"); ok {
		if index < 1 || index > len(tracks) {
			return "", fmt.Errorf("user_track_index %d is out of range", index)
		}
		return visibleTrackID(tracks[index-1]), nil
	}

	nameKeys := []string{"target_track_name", "target_track", "track"}
	if spec.CommandName != "rename_track" {
		nameKeys = append(nameKeys, "track_name")
	}
	if name := firstString(cmd, nameKeys...); name != "" {
		for _, row := range tracks {
			if strings.EqualFold(visibleTrackName(row), name) {
				return visibleTrackID(row), nil
			}
		}
		return "", fmt.Errorf("no user-visible track named %q", name)
	}
	if trackID, ok := trackIDFromContext(tracks, requestContext); ok {
		return trackID, nil
	}

	switch len(tracks) {
	case 0:
		return "", fmt.Errorf("there are no user-visible tracks")
	case 1:
		return visibleTrackID(tracks[0]), nil
	default:
		return "", fmt.Errorf("multiple user-visible tracks exist; specify track name or user_track_index")
	}
}

func trackIDFromContext(tracks []map[string]any, requestContext map[string]any) (string, bool) {
	if len(tracks) == 0 || len(requestContext) == 0 {
		return "", false
	}
	if selectedID := firstString(requestContext, "selected_plugin_track_id", "selected_track_id", "track_id"); selectedID != "" {
		for _, row := range tracks {
			if visibleTrackID(row) == selectedID {
				return selectedID, true
			}
		}
	}
	if selectedName := firstString(requestContext, "selected_track_name", "track_name", "name"); selectedName != "" {
		for _, row := range tracks {
			if strings.EqualFold(visibleTrackName(row), selectedName) {
				return visibleTrackID(row), true
			}
		}
	}
	return "", false
}

func (h *Harness) resolvePluginID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if id := firstString(cmd, "plugin_id", "plugin_item_id", "item_id"); id != "" {
		return id, nil
	}
	if id := firstString(requestContext, "selected_plugin_id", "current_plugin_id", "plugin_id"); id != "" {
		return id, nil
	}
	if h == nil || h.shadow == nil {
		return "", nil
	}

	state := h.UserStateSummary(ctx)
	trackScope := firstNonEmpty(
		firstString(cmd, "track_id", "selected_plugin_track_id"),
		firstString(requestContext, "selected_plugin_track_id", "selected_track_id", "track_id"),
	)
	refs := visiblePluginRefs(state, trackScope)
	if name := firstString(cmd, "plugin_name", "target_plugin_name", "plugin"); name != "" {
		matches := make([]pluginRef, 0, 1)
		for _, ref := range refs {
			if strings.EqualFold(ref.Name, name) {
				matches = append(matches, ref)
			}
		}
		switch len(matches) {
		case 1:
			return matches[0].ID, nil
		case 0:
			return "", fmt.Errorf("no visible plugin named %q", name)
		default:
			return "", fmt.Errorf("multiple visible plugins named %q; specify plugin_id", name)
		}
	}
	switch len(refs) {
	case 0:
		return "", fmt.Errorf("no visible plugin target is selected")
	case 1:
		return refs[0].ID, nil
	default:
		if spec.CommandName == "plugin_grabber_upsert_project_profile" || spec.CommandName == "plugin_grabber_remove_project_profile" || spec.CommandName == "plugin_grabber_apply_control" || spec.CommandName == "get_plugin_parameters" {
			return "", fmt.Errorf("multiple visible plugins match; select one plugin or specify plugin_id")
		}
	}
	return "", nil
}

func (h *Harness) resolveMacroTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	switch spec.CommandName {
	case "control_add_macro":
		if !looksLikeExistingMacroBindingIntent(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
			return nil
		}
		macro, ok := h.resolveMacroControl(ctx, cmd, requestContext)
		if !ok {
			return nil
		}
		applyResolvedMacroToCommand(cmd, macro)
		cmd["_use_existing_macro"] = true
		cmd["_existing_macro"] = macro
	case "control_add_binding", "control_set_macro_values":
		macro, ok := h.resolveMacroControl(ctx, cmd, requestContext)
		if !ok {
			return nil
		}
		applyResolvedMacroToCommand(cmd, macro)
	case "control_rename_macro":
		macro, ok := h.resolveMacroControlForRename(ctx, cmd, requestContext)
		if !ok {
			return nil
		}
		applyResolvedMacroToCommand(cmd, macro)
		cmd["_existing_macro"] = macro
	}
	return nil
}

func (h *Harness) resolveMacroControl(ctx context.Context, cmd map[string]any, requestContext map[string]any) (map[string]any, bool) {
	macros := h.macroControlsForResolution(ctx, requestContext)
	if len(macros) == 0 {
		return nil, false
	}

	for _, ref := range macroReferenceCandidates(cmd) {
		if macro, ok := findMacroControlByReference(macros, ref); ok {
			return macro, true
		}
	}
	if ref := macroReferenceFromUserText(firstString(requestContext, "user_message", "message", "prompt", "utterance")); ref != "" {
		if macro, ok := findMacroControlByReference(macros, ref); ok {
			return macro, true
		}
	}
	if refs := macrocontrols.NormalizeList(requestContext["macro_refs"]); len(refs) == 1 && referencesCurrentMacro(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
		return refs[0], true
	}
	return nil, false
}

func (h *Harness) resolveMacroControlForRename(ctx context.Context, cmd map[string]any, requestContext map[string]any) (map[string]any, bool) {
	macros := h.macroControlsForResolution(ctx, requestContext)
	if len(macros) == 0 {
		return nil, false
	}
	for _, ref := range renameMacroReferenceCandidates(cmd) {
		if macro, ok := findMacroControlByReference(macros, ref); ok {
			return macro, true
		}
	}
	if ref := macroReferenceFromUserText(firstString(requestContext, "user_message", "message", "prompt", "utterance")); ref != "" {
		if macro, ok := findMacroControlByReference(macros, ref); ok {
			return macro, true
		}
	}
	if refs := macrocontrols.NormalizeList(requestContext["macro_refs"]); len(refs) == 1 && referencesCurrentMacro(firstString(requestContext, "user_message", "message", "prompt", "utterance")) {
		return refs[0], true
	}
	return nil, false
}

func (h *Harness) macroControlsForResolution(ctx context.Context, requestContext map[string]any) []map[string]any {
	out := make([]map[string]any, 0)
	seen := map[string]bool{}
	appendRows := func(rows []map[string]any) {
		for _, row := range rows {
			macroID := firstString(row, "macro_id", "id", "control_id")
			if macroID == "" || seen[macroID] {
				continue
			}
			seen[macroID] = true
			out = append(out, row)
		}
	}
	for _, key := range []string{"macro_refs", "available_macro_controls", "active_macro_controls", "macro_controls", "rack_control_macros", "control_macros", "macros"} {
		appendRows(macrocontrols.NormalizeList(requestContext[key]))
	}
	if h != nil && h.shadow != nil {
		state := h.UserStateSummary(ctx)
		for _, key := range []string{"macro_controls", "rack_control_macros", "control_macros", "macros"} {
			appendRows(macrocontrols.NormalizeList(state[key]))
		}
	}
	return out
}

func macroReferenceCandidates(cmd map[string]any) []string {
	candidates := make([]string, 0, 8)
	for _, key := range []string{"macro_id", "id", "control_id", "source_node_id", "source_macro_id", "macro_name", "macro", "macro_label", "source_name", "source_label"} {
		if value := firstString(cmd, key); value != "" {
			candidates = append(candidates, value)
		}
	}
	if name := firstString(cmd, "name", "label", "title"); name != "" && !isGenericMacroReferenceName(name) {
		candidates = append(candidates, name)
	}
	return candidates
}

func renameMacroReferenceCandidates(cmd map[string]any) []string {
	candidates := make([]string, 0, 10)
	for _, key := range []string{"target_macro_id", "target_control_id", "macro_id", "id", "control_id", "source_node_id", "source_macro_id", "macro_name", "macro", "macro_label", "source_name", "source_label", "old_name", "old_label"} {
		if value := firstString(cmd, key); value != "" {
			candidates = append(candidates, value)
		}
	}
	return candidates
}

func applyResolvedMacroToCommand(cmd map[string]any, macro map[string]any) {
	macroID := firstString(macro, "macro_id", "id", "control_id")
	if macroID == "" {
		return
	}
	cmd["macro_id"] = macroID
	cmd["id"] = macroID
	cmd["source_node_id"] = macroID
	if trackID := firstString(macro, "track_id"); trackID != "" && firstString(cmd, "track_id", "target_track_id") == "" {
		cmd["track_id"] = trackID
	}
	if name := firstString(macro, "name", "label", "title"); name != "" && firstString(cmd, "macro_name") == "" {
		cmd["macro_name"] = name
	}
}

func findMacroControlByReference(macros []map[string]any, ref string) (map[string]any, bool) {
	clean := strings.TrimSpace(ref)
	if clean == "" {
		return nil, false
	}
	for _, macro := range macros {
		if strings.EqualFold(firstString(macro, "macro_id", "id", "control_id"), clean) {
			return macro, true
		}
	}
	normalizedRef := normalizeMacroReferenceText(clean)
	if normalizedRef != "" {
		for _, macro := range macros {
			for _, value := range []string{firstString(macro, "name", "label", "title"), firstString(macro, "macro_id", "id", "control_id")} {
				if normalizeMacroReferenceText(value) == normalizedRef {
					return macro, true
				}
			}
		}
	}
	if ordinal := macroOrdinalFromText(clean); ordinal > 0 {
		for _, macro := range macros {
			name := normalizeMacroReferenceText(firstString(macro, "name", "label", "title"))
			if name == "macro"+strconv.Itoa(ordinal) || name == "\u5b8f\u63a7\u4ef6"+strconv.Itoa(ordinal) || name == "\u5b8f\u63a7\u5236"+strconv.Itoa(ordinal) {
				return macro, true
			}
		}
		if ordinal <= len(macros) {
			return macros[ordinal-1], true
		}
	}
	return nil, false
}

func normalizeMacroReferenceText(text string) string {
	out := strings.ToLower(strings.TrimSpace(text))
	out = strings.ReplaceAll(out, "marco", "macro")
	for _, part := range []string{" ", "\t", "\r", "\n", "_", "-", "#", "\uff1a", "\u5230", "\u7ed9"} {
		out = strings.ReplaceAll(out, part, "")
	}
	return out
}

var macroReferencePattern = regexp.MustCompile(`(?i)(?:macro|marco|\x{5b8f}\x{63a7}(?:\x{4ef6}|\x{5236})?|\x{5b8f}\x{6ed1}\x{5757}|\x{5b8f}\x{6ed1}\x{687f}|\x{5b8f})\s*#?\s*([0-9]+)`)

func macroReferenceFromUserText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if match := macroReferencePattern.FindStringSubmatch(text); len(match) >= 2 {
		return "Macro " + match[1]
	}
	return ""
}

func macroOrdinalFromText(text string) int {
	if match := macroReferencePattern.FindStringSubmatch(text); len(match) >= 2 {
		if n, err := strconv.Atoi(match[1]); err == nil {
			return n
		}
	}
	return 0
}

func looksLikeExistingMacroBindingIntent(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	hasBind := strings.Contains(lower, "bind") || strings.Contains(lower, "map") || strings.Contains(text, "\u7ed1\u5b9a") || strings.Contains(text, "\u7d81\u5b9a") || strings.Contains(text, "\u6346\u7ed1") || strings.Contains(text, "\u6620\u5c04") || strings.Contains(text, "\u7ed1\u5230") || strings.Contains(text, "\u7d81\u5230")
	hasMacroRef := macroReferenceFromUserText(text) != "" || strings.Contains(lower, "macro") || strings.Contains(lower, "marco") || strings.Contains(text, "\u5b8f\u63a7")
	return hasBind && hasMacroRef
}

func referencesCurrentMacro(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "this macro") || strings.Contains(lower, "current macro") || strings.Contains(text, "\u8fd9\u4e2a\u5b8f") || strings.Contains(text, "\u9019\u500b\u5b8f") || strings.Contains(text, "\u5f53\u524d\u5b8f") || strings.Contains(text, "\u76ee\u524d\u5b8f")
}

func isGenericMacroReferenceName(name string) bool {
	normalized := normalizeMacroReferenceText(name)
	return normalized == "" || normalized == "macro" || normalized == "agentmacro" || normalized == "\u5b8f\u63a7\u4ef6" || normalized == "\u5b8f\u63a7\u5236" || normalized == "\u901a\u7528\u5b8f\u63a7\u4ef6"
}

type pluginRef struct {
	ID      string
	Name    string
	TrackID string
}

func visiblePluginRefs(state map[string]any, trackScope string) []pluginRef {
	tracks := visibleTrackRows(state)
	out := make([]pluginRef, 0)
	seen := map[string]bool{}
	for _, track := range tracks {
		trackID := visibleTrackID(track)
		if strings.TrimSpace(trackScope) != "" && trackID != strings.TrimSpace(trackScope) {
			continue
		}
		appendPluginRows := func(rows []map[string]any) {
			for _, plugin := range rows {
				id := firstString(plugin, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
				if id == "" {
					continue
				}
				key := trackID + "::" + id
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, pluginRef{
					ID:      id,
					Name:    firstNonEmpty(firstString(plugin, "plugin_name", "name", "display_name"), id),
					TrackID: trackID,
				})
			}
		}

		appendPluginRows(mapRowsFromAny(track["plugins"]))
		appendPluginRows(mapRowsFromAny(track["rack_nodes"]))
		if rack := mapFromAny(track["rack"]); len(rack) > 0 {
			appendPluginRows(mapRowsFromAny(rack["nodes"]))
		}
	}
	return out
}

func (h *Harness) afterKernelReply(ctx context.Context, spec tools.CommandSpec, reply map[string]any) {
	if !kernelReplySucceeded(reply) {
		return
	}
	if spec.CommandName == "get_plugin_parameters" || spec.CommandName == "plugin_grabber_explain_controls" {
		h.ObservePluginParametersReply(reply)
	}
	if spec.CommandName == "get_project_state" {
		if h.shadow != nil {
			h.shadow.Initialize(reply)
		}
		return
	}
	if spec.CommandName == "project.audio_analysis_status" || spec.CommandName == "project.audio_analysis_start" {
		h.persistAudioAnalysisManifest(ctx, reply)
	}
	if spec.RefreshAfter {
		h.refreshShadow(ctx, spec.CommandName)
	}
	h.applyKernelReplyShadowDelta(spec, reply)
}

func (h *Harness) persistAudioAnalysisManifest(ctx context.Context, reply map[string]any) {
	job := mapFromAny(reply["analysis_job"])
	status := firstNonEmpty(firstString(reply, "dad_fact_status"), firstString(job, "dad_fact_status"))
	if status != "ready" {
		return
	}
	projectPath, projectUUID := h.CurrentProjectIdentity(ctx)
	if projectPath == "" || projectUUID == "" {
		return
	}
	manifest, path, err := projectworkspace.SaveAnalysisManifest(projectPath, projectUUID, reply)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("[workspace] analysis manifest save failed project=%s uuid=%s error=%v", projectPath, projectUUID, err)
		}
		return
	}
	reply["analysis_manifest_path"] = path
	reply["analysis_manifest_row_count"] = len(manifest.Rows)
}

func (h *Harness) recoverAudioAnalysisManifest(ctx context.Context) (map[string]any, error) {
	projectPath, projectUUID := h.CurrentProjectIdentity(ctx)
	manifest, path, err := projectworkspace.LoadAnalysisManifest(projectPath, projectUUID)
	if err == nil {
		return manifest.AudioAnalysisStatus(path), nil
	}
	if h == nil || h.shadow == nil {
		return nil, err
	}
	embedded := mapFromAny(h.shadow.Summary()["analysis_manifest"])
	if len(embedded) == 0 {
		return nil, err
	}
	status := map[string]any{
		"dad_fact_status":          embedded["status"],
		"track_waveform_envelopes": embedded["l1_waveform_rows"],
	}
	manifest, path, saveErr := projectworkspace.SaveAnalysisManifest(projectPath, projectUUID, status)
	if saveErr != nil {
		return nil, saveErr
	}
	return manifest.AudioAnalysisStatus(path), nil
}

func (h *Harness) ensureProjectAudioAnalysis(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if recovered, err := h.recoverAudioAnalysisManifest(ctx); err == nil && audioAnalysisFactsReady(recovered) {
		return recovered, nil
	}
	if h == nil || h.kernel == nil {
		return nil, errors.New("project audio analysis ensure requires a connected kernel")
	}
	timeoutMS := int(numberFromAny(cmd["timeout_ms"]))
	if timeoutMS <= 0 {
		timeoutMS = 240000
	}
	pollMS := int(numberFromAny(cmd["poll_interval_ms"]))
	if pollMS <= 0 {
		pollMS = 250
	}
	timeout := time.Duration(timeoutMS) * time.Millisecond
	pollInterval := time.Duration(pollMS) * time.Millisecond
	if timeout < time.Second {
		timeout = time.Second
	}
	if pollInterval < 50*time.Millisecond {
		pollInterval = 50 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	statusCommand := map[string]any{"cmd": "project.audio_analysis_status", "latest": true}
	status, _, statusErr := h.kernel.SendCommand(ctx, statusCommand)
	if statusErr == nil && kernelReplySucceeded(status) && audioAnalysisFactsReady(status) {
		h.persistAudioAnalysisManifest(ctx, status)
		return status, nil
	}
	start := map[string]any{
		"cmd":                  "project.audio_analysis_start",
		"retry_missing":        true,
		"rebuild_from_project": true,
		"interval_ms":          50,
	}
	started, _, err := h.kernel.SendCommand(ctx, start)
	if err != nil {
		return nil, err
	}
	if !kernelReplySucceeded(started) {
		return nil, fmt.Errorf("%s", firstNonEmpty(firstString(started, "message", "error"), "project audio analysis rebuild failed"))
	}
	jobID := firstString(started, "analysis_job_id", "job_id")
	if jobID != "" {
		statusCommand["analysis_job_id"] = jobID
		statusCommand["job_id"] = jobID
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("project audio analysis ensure timed out after %s", timeout)
		}
		status, _, err = h.kernel.SendCommand(ctx, statusCommand)
		if err == nil && kernelReplySucceeded(status) {
			if audioAnalysisFactsReady(status) {
				h.persistAudioAnalysisManifest(ctx, status)
				status["analysis_ensure_rebuilt"] = true
				return status, nil
			}
			if firstString(status, "dad_fact_status") == "failed" {
				return nil, errors.New("project audio analysis reported failed DAD facts")
			}
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func audioAnalysisFactsReady(status map[string]any) bool {
	job := mapFromAny(status["analysis_job"])
	readyStatus := firstNonEmpty(firstString(status, "dad_fact_status"), firstString(job, "dad_fact_status"))
	ready := int(numberFromAny(firstNonNil(status["dad_fact_ready_count"], job["dad_fact_ready_count"])))
	total := int(numberFromAny(firstNonNil(status["dad_fact_total_count"], job["dad_fact_total_count"], status["total_clips"], job["total_clips"])))
	return readyStatus == "ready" && total > 0 && ready >= total
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func kernelReplySucceeded(reply map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
	return status == "ok" || status == "success"
}

func (h *Harness) applyKernelReplyShadowDelta(spec tools.CommandSpec, reply map[string]any) {
	if h == nil || h.shadow == nil {
		return
	}
	switch spec.CommandName {
	case "set_pan":
		trackID := firstString(reply, "track_id")
		pan, ok := numberValueFromMap(reply, "pan", "pan_value")
		if trackID == "" || !ok {
			return
		}
		h.shadow.ApplyDelta(map[string]any{
			"type":       "delta_update",
			"target_uid": trackID,
			"action":     "property_changed:pan",
			"value":      pan,
		})
	case "set_volume":
		trackID := firstString(reply, "track_id")
		db, ok := numberValueFromMap(reply, "volume_db", "fader_db", "gain_db", "db")
		if trackID == "" || !ok {
			return
		}
		h.shadow.ApplyDelta(map[string]any{
			"type":       "delta_update",
			"target_uid": trackID,
			"action":     "property_changed:volume_db",
			"value":      db,
		})
	case "track.group.apply_control":
		for _, member := range mapRowsFromAny(reply["members"]) {
			trackID := firstString(member, "track_id", "id")
			db, ok := numberValueFromMap(member, "after_db", "target_db", "volume_db", "fader_db", "gain_db", "db")
			if trackID == "" || !ok {
				continue
			}
			h.shadow.ApplyDelta(map[string]any{
				"type":       "delta_update",
				"target_uid": trackID,
				"action":     "property_changed:volume_db",
				"value":      db,
			})
		}
	}
}

func (h *Harness) publicResult(spec tools.CommandSpec, cmd map[string]any, reply map[string]any) map[string]any {
	switch spec.CommandName {
	case "get_project_state":
		if h.shadow != nil {
			return withOKStatus(userVisibleState(h.shadow.Summary()))
		}
	case "project.get_audio_settings", "project.set_audio_settings", "project.validate_audio_settings_change":
		return publicProjectAudioSettingsResult(reply)
	case "project.import_preflight", "media.inspect_files":
		return publicAudioPreflightResult(spec.CommandName, reply)
	case "project.import_folder_as_stems", "project.import_audio_files":
		return publicStemsImportResult(spec.CommandName, cmd, reply)
	case "project.audio_analysis_start", "project.audio_analysis_status", "project.audio_analysis_cancel":
		return publicAudioAnalysisResult(spec.CommandName, cmd, reply)
	case "get_plugin_parameters":
		return publicPluginParametersResult(cmd, reply)
	case "list_tracks":
		if h.shadow != nil {
			summary := h.shadow.Summary()
			return map[string]any{
				"status":           "ok",
				"track_count":      summary["track_count"],
				"user_track_count": summary["user_track_count"],
				"tracks":           summary["tracks"],
			}
		}
	case "track.group.list":
		out := cloneAnyMap(reply)
		groupValue := reply["track_groups"]
		if groupValue == nil {
			groupValue = reply["groups"]
		}
		groups := mapRowsFromAny(groupValue)
		out["track_groups"] = groups
		out["groups"] = groups
		out["group_count"] = len(groups)
		return out
	case "track.group.create", "track.group.update", "track.group.set_members":
		out := cloneAnyMap(reply)
		out["ui_action"] = "track_groups_changed"
		if groupID := firstNonEmpty(firstString(reply, "group_id", "id"), firstString(cmd, "group_id", "id")); groupID != "" {
			out["group_id"] = groupID
		}
		return out
	case "track.group.delete":
		out := cloneAnyMap(reply)
		out["ui_action"] = "track_groups_changed"
		out["deleted_group_id"] = firstNonEmpty(firstString(reply, "group_id", "id"), firstString(cmd, "group_id", "id"))
		return out
	case "track.group.apply_control":
		out := cloneAnyMap(reply)
		out["ui_action"] = "track_group_control_applied"
		out["group_id"] = firstNonEmpty(firstString(reply, "group_id", "id"), firstString(cmd, "group_id", "id"))
		affected := make([]string, 0)
		for _, member := range mapRowsFromAny(reply["members"]) {
			if trackID := firstString(member, "track_id", "id"); trackID != "" {
				affected = append(affected, trackID)
			}
		}
		out["affected_track_ids"] = affected
		return out
	case "remove_clips":
		out := cloneAnyMap(reply)
		requested := stringSliceFromAny(cmd["clip_ids"])
		missing := stringSet(stringSliceFromAny(reply["missing_ids"]))
		removed := make([]string, 0, len(requested))
		for _, id := range requested {
			if !missing[id] {
				removed = append(removed, id)
			}
		}
		out["ui_action"] = "remove_clips"
		out["requested_clip_ids"] = requested
		out["removed_clip_ids"] = removed
		return out
	case "clone_clip":
		out := cloneAnyMap(reply)
		newClipID := firstString(reply, "new_clip_id", "clip_id")
		targetTrackID := firstNonEmpty(
			firstString(reply, "target_track_id", "track_id", "affected_track_id"),
			firstString(cmd, "target_track_id", "track_id"),
		)
		if newClipID != "" {
			out["ui_action"] = "select_clip"
			out["clip_id"] = newClipID
			out["new_clip_id"] = newClipID
			out["created_clip_ids"] = []string{newClipID}
		}
		if targetTrackID != "" {
			out["track_id"] = targetTrackID
			out["target_track_id"] = targetTrackID
		}
		if sourceClipID := firstString(cmd, "source_clip_id", "clip_id"); sourceClipID != "" {
			out["source_clip_id"] = sourceClipID
		}
		return out
	case "clip.fade.set", "clip.fade.read", "clip.gain.set", "clip.gain.read":
		out := cloneAnyMap(reply)
		out["ui_action"] = "clip_state_changed"
		out["clip_id"] = firstNonEmpty(firstString(reply, "clip_id"), firstString(cmd, "clip_id"))
		out["track_id"] = firstNonEmpty(firstString(reply, "track_id"), firstString(cmd, "track_id"))
		return out
	case "insert_midi_clip", "create_midi_clip", "import_midi_to_track":
		out := cloneAnyMap(reply)
		newClipID := firstString(reply, "new_clip_id", "clip_id", "created_clip_id")
		targetTrackID := firstNonEmpty(
			firstString(reply, "target_track_id", "track_id", "affected_track_id"),
			firstString(cmd, "target_track_id", "track_id"),
		)
		if newClipID != "" {
			out["ui_action"] = "select_clip"
			out["clip_id"] = newClipID
			out["new_clip_id"] = newClipID
			out["created_clip_ids"] = []string{newClipID}
		}
		if targetTrackID != "" {
			out["track_id"] = targetTrackID
			out["target_track_id"] = targetTrackID
		}
		return out
	case "apply_midi_note_patch":
		out := cloneAnyMap(reply)
		out["ui_action"] = "midi_note_patch"
		out["clip_id"] = firstNonEmpty(firstString(reply, "clip_id"), firstString(cmd, "clip_id"))
		if trackID := firstNonEmpty(firstString(reply, "track_id"), firstString(cmd, "track_id")); trackID != "" {
			out["track_id"] = trackID
		}
		return out
	case "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes":
		out := cloneAnyMap(reply)
		out["ui_action"] = "midi_note_patch"
		out["clip_id"] = firstNonEmpty(firstString(reply, "clip_id"), firstString(cmd, "clip_id"))
		if trackID := firstNonEmpty(firstString(reply, "track_id"), firstString(cmd, "track_id")); trackID != "" {
			out["track_id"] = trackID
		}
		return out
	}
	return reply
}

func publicProjectAudioSettingsResult(reply map[string]any) map[string]any {
	out := map[string]any{
		"status":  firstNonEmpty(firstString(reply, "status"), "ok"),
		"command": firstString(reply, "command"),
	}
	if message := firstString(reply, "message"); message != "" {
		out["message"] = message
	}
	settings := mapFromAny(reply["audio_settings"])
	if len(settings) > 0 {
		compact := map[string]any{}
		for _, key := range []string{
			"schema_version", "sample_rate_hz", "record_bit_depth", "record_file_type", "pcm_format",
			"internal_processing_format", "import_sample_rate_policy", "import_bit_depth_policy",
			"media_copy_policy", "channel_import_policy", "render_default_sample_rate_hz",
			"render_default_bit_depth", "render_default_file_type", "dither_policy",
			"metadata_state", "defaulted_this_call", "migration_state", "defaulted_origin",
		} {
			if value, ok := settings[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if fieldStatus := mapFromAny(settings["field_status"]); len(fieldStatus) > 0 {
			compact["field_status"] = fieldStatus
		}
		if capabilities := mapFromAny(settings["capabilities"]); len(capabilities) > 0 {
			compact["capabilities"] = capabilities
		}
		if presets := compactProjectAudioPresets(mapRowsFromAny(settings["recommended_presets"]), 8); len(presets) > 0 {
			compact["recommended_presets"] = presets
		}
		out["audio_settings"] = compact
	}
	if warnings := compactAudioWarnings(reply["warnings"], 8); len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if safe, ok := reply["safe_to_apply"]; ok {
		out["safe_to_apply"] = safe
	}
	if count, ok := reply["audio_clip_count"]; ok {
		out["audio_clip_count"] = count
	}
	if changed, ok := reply["changes_audio_device_sample_rate"]; ok {
		out["changes_audio_device_sample_rate"] = changed
	}
	return out
}

func publicAudioPreflightResult(commandName string, reply map[string]any) map[string]any {
	out := map[string]any{
		"status":  firstNonEmpty(firstString(reply, "status"), "ok"),
		"command": firstNonEmpty(firstString(reply, "command"), commandName),
	}
	if summary := mapFromAny(reply["summary"]); len(summary) > 0 {
		out["summary"] = summary
	}
	if settings := mapFromAny(reply["audio_settings_snapshot"]); len(settings) > 0 {
		out["audio_settings_snapshot"] = settings
	}
	if decision := mapFromAny(reply["sample_rate_decision"]); len(decision) > 0 {
		out["sample_rate_decision"] = decision
	}
	if patch := mapFromAny(reply["project_audio_settings_patch"]); len(patch) > 0 {
		out["project_audio_settings_patch"] = patch
	}
	if plan := mapFromAny(reply["import_plan"]); len(plan) > 0 {
		compactPlan := map[string]any{}
		for _, key := range []string{"intended_mode", "target_policy", "start_time_seconds", "tracks_to_create", "copy_reference_strategy", "requires_user_confirmation"} {
			if value, ok := plan[key]; ok && !isEmptyValue(value) {
				compactPlan[key] = value
			}
		}
		if rows := compactPreflightRows(mapRowsFromAny(plan["track_plan"]), 8); len(rows) > 0 {
			compactPlan["track_plan_preview"] = rows
		}
		if rows := compactPreflightRows(mapRowsFromAny(plan["sample_rate_mismatches"]), 8); len(rows) > 0 {
			compactPlan["sample_rate_mismatch_examples"] = rows
		}
		if rows := compactPreflightRows(mapRowsFromAny(plan["bit_depth_or_format_mismatches"]), 8); len(rows) > 0 {
			compactPlan["bit_depth_or_format_mismatch_examples"] = rows
		}
		if unreadable := firstStrings(stringSliceFromAny(plan["unreadable_files"]), 8); len(unreadable) > 0 {
			compactPlan["unreadable_file_examples"] = unreadable
		}
		if decision := mapFromAny(plan["sample_rate_decision"]); len(decision) > 0 {
			compactPlan["sample_rate_decision"] = decision
		}
		if patch := mapFromAny(plan["project_audio_settings_patch"]); len(patch) > 0 {
			compactPlan["project_audio_settings_patch"] = patch
		}
		out["import_plan"] = compactPlan
	}
	if files := compactPreflightRows(mapRowsFromAny(reply["files"]), 8); len(files) > 0 {
		out["file_preview"] = files
		out["file_preview_count"] = len(files)
	}
	return out
}

func publicStemsImportResult(commandName string, cmd map[string]any, reply map[string]any) map[string]any {
	out := map[string]any{
		"status":  firstNonEmpty(firstString(reply, "status"), "ok"),
		"command": firstNonEmpty(firstString(reply, "command"), commandName),
	}
	for _, key := range []string{
		"message", "action", "baking_status", "analysis_deferred", "analysis_job_id",
		"analysis_queue_status", "analysis_jobs_created", "analysis_jobs_queued",
		"analysis_total_clips", "analysis_total_feature_jobs",
		"last_created_track_id", "last_created_clip_id",
	} {
		if value, ok := reply[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	attachMixboardFeatureSnapshotPath(out, cmd, reply)
	if summary := mapFromAny(reply["summary"]); len(summary) > 0 {
		out["summary"] = summary
	}
	if settings := mapFromAny(reply["audio_settings_snapshot"]); len(settings) > 0 {
		out["audio_settings_snapshot"] = settings
	}
	if decision := mapFromAny(reply["sample_rate_decision"]); len(decision) > 0 {
		out["sample_rate_decision"] = decision
	}
	if patch := mapFromAny(reply["project_audio_settings_patch"]); len(patch) > 0 {
		out["project_audio_settings_patch"] = patch
	}
	if ids := firstStrings(stringSliceFromAny(reply["created_track_ids"]), 16); len(ids) > 0 {
		out["created_track_ids_preview"] = ids
		out["created_track_count_reported"] = len(stringSliceFromAny(reply["created_track_ids"]))
	}
	if ids := firstStrings(stringSliceFromAny(reply["created_clip_ids"]), 16); len(ids) > 0 {
		out["created_clip_ids_preview"] = ids
		out["created_clip_count_reported"] = len(stringSliceFromAny(reply["created_clip_ids"]))
	}
	if rows := compactImportedStemRows(mapRowsFromAny(reply["imported_tracks"]), 12); len(rows) > 0 {
		out["imported_tracks_preview"] = rows
		out["imported_tracks_preview_count"] = len(rows)
	}
	if refs := compactImportedStemRefs(mapRowsFromAny(reply["imported_tracks"])); len(refs) > 0 {
		out["imported_track_refs"] = refs
		out["imported_track_ref_count"] = len(refs)
	}
	if rows := compactImportedStemRows(mapRowsFromAny(reply["imported_tracks"]), 0); len(rows) > 0 {
		proj := tim.BuildFromImportRows(tim.ImportInput{
			Summary:       mapFromAny(reply["summary"]),
			Rows:          rows,
			AudioSettings: mapFromAny(reply["audio_settings_snapshot"]),
		})
		out["tim_projection"] = tim.ContextProjectionMap(proj)
	}
	if rows := compactPreflightRows(mapRowsFromAny(reply["sample_rate_mismatches"]), 8); len(rows) > 0 {
		out["sample_rate_mismatch_examples"] = rows
	}
	if rows := compactPreflightRows(mapRowsFromAny(reply["bit_depth_or_format_mismatches"]), 8); len(rows) > 0 {
		out["bit_depth_or_format_mismatch_examples"] = rows
	}
	if unreadable := compactPreflightRows(mapRowsFromAny(reply["unreadable_files"]), 8); len(unreadable) > 0 {
		out["unreadable_file_examples"] = unreadable
	}
	if copied := compactImportedStemRows(mapRowsFromAny(reply["copied_media"]), 8); len(copied) > 0 {
		out["copied_media_preview"] = copied
	}
	if warnings := compactAudioWarnings(reply["warnings"], 8); len(warnings) > 0 {
		out["warnings"] = warnings
	}
	return out
}

func publicAudioAnalysisResult(commandName string, cmd map[string]any, reply map[string]any) map[string]any {
	out := map[string]any{
		"status":  firstNonEmpty(firstString(reply, "status"), "ok"),
		"command": firstNonEmpty(firstString(reply, "command"), commandName),
	}
	for _, key := range []string{
		"message", "analysis_job_id", "job_id", "analysis_queue_status",
		"total_clips", "submitted_clips", "total_feature_jobs", "submitted_feature_jobs",
		"dad_fact_status", "dad_fact_ready_count", "dad_fact_total_count", "dad_fact_pending_count", "dad_fact_failed_count",
		"dad_fact_completion_scope",
		"feature_snapshot_path", "mixboard_feature_snapshot_path",
		"analysis_manifest_path", "analysis_manifest_row_count", "analysis_manifest_recovered", "project_uuid",
	} {
		if value, ok := reply[key]; ok && !isEmptyValue(value) {
			out[key] = value
		}
	}
	if rows := compactAudioAnalysisWaveformRows(mapRowsFromAny(reply["track_waveform_envelopes"])); len(rows) > 0 {
		out["track_waveform_envelopes"] = rows
	}
	attachMixboardFeatureSnapshotPath(out, cmd, reply)
	if proj := mapFromAny(reply["tim_projection"]); len(proj) > 0 {
		out["tim_projection"] = proj
	}
	if job := compactAudioAnalysisJob(mapFromAny(reply["analysis_job"])); len(job) > 0 {
		out["analysis_job"] = job
	}
	return out
}

func attachMixboardFeatureSnapshotPath(out map[string]any, cmd map[string]any, reply map[string]any) {
	if len(out) == 0 {
		return
	}
	path := firstNonEmpty(
		firstString(reply, "feature_snapshot_path", "mixboard_feature_snapshot_path"),
		firstString(cmd, "feature_snapshot_path", "mixboard_feature_snapshot_path"),
		mixboard.FeatureSnapshotPath(cmd),
	)
	if path == "" {
		return
	}
	out["feature_snapshot_path"] = path
	out["mixboard_feature_snapshot_path"] = path
}

func compactAudioAnalysisJob(job map[string]any) map[string]any {
	if len(job) == 0 {
		return nil
	}
	compact := map[string]any{}
	for _, key := range []string{
		"analysis_job_id", "job_id", "status", "analysis_queue_status",
		"total_clips", "submitted_clips", "pending_clips",
		"total_feature_jobs", "submitted_feature_jobs", "pending_feature_jobs",
		"canceled_feature_jobs", "progress", "progress_percent",
		"dad_fact_status", "dad_fact_ready_count", "dad_fact_total_count", "dad_fact_pending_count", "dad_fact_failed_count",
		"dad_fact_completion_scope",
		"interval_ms", "cancel_scope", "completion_scope",
		"feature_snapshot_path", "mixboard_feature_snapshot_path",
	} {
		if value, ok := job[key]; ok && !isEmptyValue(value) {
			compact[key] = value
		}
	}
	if rows := compactAudioAnalysisWaveformRows(mapRowsFromAny(job["track_waveform_envelopes"])); len(rows) > 0 {
		compact["track_waveform_envelopes"] = rows
	}
	return compact
}

func compactAudioAnalysisWaveformRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		compact := map[string]any{}
		for _, key := range []string{
			"status", "reason", "feature_type", "source", "source_kind",
			"track_id", "clip_id", "source_track_id",
			"source_path", "file_path", "source_id", "source_revision", "source_fingerprint",
			"clip_revision", "render_revision", "analyzer_revision",
			"duration_seconds", "total_duration", "sample_rate",
			"channel_count", "channels", "rms", "peak", "peak_abs", "rms_dbfs", "peak_dbfs",
			"headroom_db", "crest_db", "balance_db", "balance_state", "correlation_estimate",
			"correlation_state",
			"tile_count_seen", "tile_count_expected", "completed_tiles", "total_tiles",
			"handle_count", "frame_count", "feature_stride", "updated_at",
			"analysis_job_id", "job_id",
		} {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func compactImportedStemRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{}
		for _, key := range []string{
			"track_id", "track_name", "clip_id", "clip_name", "source_file_path",
			"imported_file_path", "copied_file_path", "start_time_seconds", "duration_seconds",
			"sample_rate_hz", "bit_depth", "pcm_format", "channel_count",
			"needs_sample_rate_conversion", "bit_depth_or_format_differs",
			"sample_rate_policy", "bit_depth_policy", "media_copy_policy",
			"channel_import_policy", "baking_status", "analysis_queue_status",
		} {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func compactImportedStemRefs(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		compact := map[string]any{}
		for _, key := range []string{
			"track_id", "track_name", "clip_id", "clip_name",
			"source_file_path", "current_source_path", "source_path", "file_path",
			"imported_file_path", "copied_file_path",
			"source_revision", "source_fingerprint", "source_hash", "clip_revision",
			"start_time_seconds", "duration_seconds", "length_seconds",
			"sample_rate_hz", "sample_rate", "bit_depth", "bits_per_sample", "pcm_format",
			"channel_count", "channels", "media_copy_policy",
		} {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func compactProjectAudioPresets(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{}
		for _, key := range []string{"preset_id", "name", "role", "sample_rate_hz", "bit_depth", "file_type", "default_project_working_spec"} {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func compactAudioWarnings(value any, limit int) []any {
	items := anySliceFromAny(value)
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]any, 0, limit)
	for i := 0; i < limit; i++ {
		item := items[i]
		if row := mapFromAny(item); len(row) > 0 {
			compact := map[string]any{}
			for _, key := range []string{"code", "message", "project_sample_rate_hz", "audio_device_sample_rate_hz", "audio_clip_count", "old_sample_rate_hz", "new_sample_rate_hz", "old_record_bit_depth", "new_record_bit_depth"} {
				if value, ok := row[key]; ok && !isEmptyValue(value) {
					compact[key] = value
				}
			}
			out = append(out, compact)
			continue
		}
		if !isEmptyValue(item) {
			out = append(out, item)
		}
	}
	return out
}

func compactPreflightRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{}
		for _, key := range []string{
			"file_path", "file_name", "file_type", "readable", "duration_seconds", "sample_rate_hz",
			"bit_depth", "pcm_format", "channel_count", "compressed_bitrate_kbps", "suggested_track_name",
			"start_time_seconds", "needs_sample_rate_conversion", "sample_rate_policy",
			"bit_depth_or_format_differs", "bit_depth_policy", "media_copy_policy",
			"source_sample_rate_hz", "project_sample_rate_hz", "source_bit_depth",
			"source_pcm_format", "project_record_bit_depth", "policy", "warnings", "errors",
		} {
			if value, ok := row[key]; ok && !isEmptyValue(value) {
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			out = append(out, compact)
		}
	}
	return out
}

func firstStrings(values []string, limit int) []string {
	if limit <= 0 || limit > len(values) {
		limit = len(values)
	}
	if limit <= 0 {
		return nil
	}
	return append([]string(nil), values[:limit]...)
}

func publicPluginParametersResult(cmd map[string]any, reply map[string]any) map[string]any {
	params := mapRowsFromAny(reply["parameters"])
	quick := mapRowsFromAny(reply["quick_controls"])
	groups := mapRowsFromAny(reply["recommended_groups"])
	pluginGroups := mapRowsFromAny(reply["plugin_groups"])
	virtualControls := mapRowsFromAny(reply["virtual_controls"])
	globalProfile := mapAnyFromAny(reply["global_profile"])
	pluginSkill := mapAnyFromAny(reply["plugin_skill"])
	if pluginSkill == nil && globalProfile != nil {
		pluginSkill = mapAnyFromAny(globalProfile["plugin_skill"])
	}
	digest := plugingrabber.BuildParameterDigest(reply)
	out := map[string]any{
		"status":                  firstNonEmpty(firstString(reply, "status"), "ok"),
		"track_id":                reply["track_id"],
		"plugin_id":               reply["plugin_id"],
		"plugin_item_id":          reply["plugin_item_id"],
		"plugin_identity":         reply["plugin_identity"],
		"template_role":           reply["template_role"],
		"supports_param_grabber":  reply["supports_param_grabber"],
		"profile_applied":         reply["profile_applied"],
		"profile_source":          reply["profile_source"],
		"profile_stale_param_ids": reply["profile_stale_param_ids"],
		"global_profile_applied":  reply["global_profile_applied"],
		"global_profile_source":   reply["global_profile_source"],
		"plugin_class":            reply["plugin_class"],
		"parameter_count":         len(params),
		"quick_control_count":     len(quick),
		"quick_controls":          compactQuickControls(quick, 16),
		"recommended_group_count": len(groups),
		"recommended_groups":      compactRecommendedGroups(groups, 16),
		"plugin_group_count":      len(pluginGroups),
		"plugin_groups":           compactRuntimeProfileRows(pluginGroups, 12, []string{"id", "role", "label", "name"}),
		"virtual_control_count":   len(virtualControls),
		"virtual_controls":        compactRuntimeProfileRows(virtualControls, 12, []string{"name", "component_id", "component", "resolver"}),
		"safety_limits":           reply["safety_limits"],
		"capability_manifest":     reply["capability_manifest"],
		"display_probe_summary":   plugingrabber.DisplayProbeSummary(digest),
	}
	includeVPSV3Surface, _ := boolValue(cmd["include_vps_v3_surface"])
	includeParameters, _ := boolValue(cmd["include_parameters"])
	if !includeParameters {
		includeParameters, _ = boolValue(cmd["include_full_parameters"])
	}
	if !includeParameters {
		includeParameters, _ = boolValue(cmd["include_parameter_snapshot"])
	}
	if includeVPSV3Surface {
		// This is an internal, user-authorized conformance read. VPS v3 needs
		// the complete host surface (type/range/enum/display probe), rather
		// than the normal UI-sized snapshot, to construct a fail-closed
		// fingerprint. It is never enabled for ordinary chat responses.
		fullParameters := make([]map[string]any, 0, len(params))
		for _, parameter := range params {
			fullParameters = append(fullParameters, cloneAnyMap(parameter))
		}
		out["parameters"] = fullParameters
		out["vps_v3_parameter_surface"] = true
	} else if includeParameters {
		out["parameters"] = compactPluginParameterSnapshotRows(params, digest, 512)
	}
	if skill := compactPublicPluginSkill(pluginSkill); len(skill) > 0 {
		out["plugin_skill"] = skill
	}
	return out
}

func compactQuickControls(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		out = append(out, map[string]any{
			"param_id":          firstString(row, "param_id", "id"),
			"label":             firstString(row, "label", "name"),
			"widget":            firstString(row, "widget"),
			"normalized_role":   firstString(row, "normalized_role"),
			"display_group":     firstString(row, "display_group"),
			"control_relevance": row["control_relevance"],
		})
	}
	return out
}

func compactPluginParameterSnapshotRows(rows []map[string]any, digest plugingrabber.ParameterDigest, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	byID := map[string]plugingrabber.ParameterInfo{}
	for _, param := range digest.Parameters {
		if strings.TrimSpace(param.ID) != "" {
			byID[param.ID] = param
		}
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		paramID := firstString(row, "id", "param_id")
		compact := map[string]any{
			"param_id":         firstString(row, "id", "param_id"),
			"name":             firstNonEmpty(firstString(row, "name"), firstString(row, "raw_param_name"), firstString(row, "alias")),
			"raw_param_name":   firstString(row, "raw_param_name"),
			"normalized_value": row["normalized_value"],
			"value":            row["value"],
			"value_text":       firstString(row, "value_text"),
			"display_group":    firstString(row, "display_group"),
			"normalized_role":  firstString(row, "normalized_role"),
		}
		if param := byID[paramID]; strings.TrimSpace(param.ID) != "" {
			if param.DisplayProbe != nil {
				compact["display_probe"] = param.DisplayProbe
			}
			if param.DisplayDomainCandidate != nil {
				compact["display_domain_candidate"] = param.DisplayDomainCandidate
			}
		}
		out = append(out, compact)
	}
	return out
}

func compactRecommendedGroups(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		ids := stringSliceFromAny(row["parameter_ids"])
		out = append(out, map[string]any{
			"name":            firstString(row, "name", "group"),
			"parameter_count": len(ids),
			"sample_ids":      firstNStrings(ids, 8),
		})
	}
	return out
}

func compactRuntimeProfileRows(rows []map[string]any, limit int, keys []string) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{}
		for _, key := range keys {
			if value := firstString(row, key); value != "" {
				compact[key] = value
			}
		}
		if params := compactPublicPluginSkillParams(row["params"], 8); len(params) > 0 {
			compact["params"] = params
		}
		out = append(out, compact)
	}
	return out
}

func compactPublicPluginSkill(skill map[string]any) map[string]any {
	if len(skill) == 0 {
		return nil
	}
	components := mapRowsFromAny(skill["components"])
	operations := mapRowsFromAny(skill["operations"])
	out := map[string]any{
		"schema_version":  skill["schema_version"],
		"component_count": len(components),
		"operation_count": len(operations),
	}
	if capabilities := mapAnyFromAny(skill["capabilities"]); len(capabilities) > 0 {
		out["capabilities"] = capabilities
	}
	out["components"] = compactRuntimeProfileRows(components, 12, []string{"id", "role", "label"})
	out["operations"] = compactRuntimeProfileRows(operations, 12, []string{"name", "component_id", "resolver"})
	return out
}

func compactPublicPluginSkillParams(value any, limit int) []map[string]any {
	params := mapAnyFromAny(value)
	if len(params) == 0 {
		return nil
	}
	slots := make([]string, 0, len(params))
	for slot := range params {
		slots = append(slots, slot)
	}
	sort.Strings(slots)
	if limit > 0 && len(slots) > limit {
		slots = slots[:limit]
	}
	out := make([]map[string]any, 0, len(slots))
	for _, slot := range slots {
		row := map[string]any{"slot": slot}
		switch mapped := params[slot].(type) {
		case map[string]any:
			if paramID := firstString(mapped, "param_id", "id"); paramID != "" {
				row["param_id"] = paramID
			}
			if label := firstString(mapped, "label", "name"); label != "" {
				row["label"] = label
			}
			if confidence := mapped["confidence"]; confidence != nil {
				row["confidence"] = confidence
			}
		default:
			if text := strings.TrimSpace(fmt.Sprint(mapped)); text != "" && text != "<nil>" {
				row["param_id"] = text
			}
		}
		if row["param_id"] != nil {
			out = append(out, row)
		}
	}
	return out
}

func mapAnyFromAny(value any) map[string]any {
	switch x := value.(type) {
	case map[string]any:
		return x
	case map[string]string:
		out := make(map[string]any, len(x))
		for key, value := range x {
			out[key] = value
		}
		return out
	default:
		return nil
	}
}

func firstNStrings(values []string, limit int) []string {
	if limit < 0 {
		limit = 0
	}
	if limit > len(values) {
		limit = len(values)
	}
	out := make([]string, limit)
	copy(out, values[:limit])
	return out
}

func compactHistoryCommits(value any, limit int) []map[string]any {
	if limit <= 0 {
		limit = 6
	}
	var commits []history.Commit
	switch rows := value.(type) {
	case []history.Commit:
		commits = rows
	case []any:
		for _, item := range rows {
			if commit, ok := item.(history.Commit); ok {
				commits = append(commits, commit)
			}
		}
	default:
		return nil
	}
	if len(commits) > limit {
		commits = commits[:limit]
	}
	out := make([]map[string]any, 0, len(commits))
	for _, commit := range commits {
		row := map[string]any{
			"id":              commit.ID,
			"message":         commit.Message,
			"created_at":      commit.CreatedAt,
			"branch":          commit.Branch,
			"checkpoint_kind": commit.CheckpointKind,
			"goal_id":         commit.GoalID,
		}
		for key, value := range row {
			if isEmptyValue(value) {
				delete(row, key)
			}
		}
		out = append(out, row)
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func stringSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func appendStringAny(value any, item string) []string {
	out := stringSliceFromAny(value)
	if strings.TrimSpace(item) != "" {
		out = append(out, strings.TrimSpace(item))
	}
	return out
}

func boolValueDefault(value any, fallback bool) bool {
	if b, ok := boolValue(value); ok {
		return b
	}
	return fallback
}

func userVisibleState(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch k {
		case "engine_track_count", "internal_track_count":
			continue
		default:
			out[k] = v
		}
	}
	return out
}

func projectPathFromState(state map[string]any) string {
	if path := firstString(state, "project_path", "current_project_path"); path != "" {
		return path
	}
	for _, key := range []string{"shadow", "project", "state"} {
		nested, ok := state[key].(map[string]any)
		if !ok {
			continue
		}
		if path := firstString(nested, "project_path", "current_project_path"); path != "" {
			return path
		}
	}
	return ""
}

func samePath(a, b string) bool {
	left := strings.TrimSpace(a)
	right := strings.TrimSpace(b)
	if left == "" || right == "" {
		return false
	}
	if abs, err := filepath.Abs(left); err == nil {
		left = abs
	}
	if abs, err := filepath.Abs(right); err == nil {
		right = abs
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func withOKStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	out["status"] = "ok"
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (h *Harness) refreshShadow(ctx context.Context, reason string) {
	_ = h.refreshShadowWithStatus(ctx, reason)
}

func (h *Harness) refreshShadowWithStatus(ctx context.Context, reason string) map[string]any {
	started := time.Now()
	refreshStatus := "skipped"
	refreshErr := false
	defer func() {
		if h != nil && h.logger != nil {
			h.logger.Info("[timing] harness.refresh_shadow ms=%d reason=%s status=%s err=%t",
				time.Since(started).Milliseconds(), reason, refreshStatus, refreshErr)
		}
	}()
	if h == nil {
		refreshErr = true
		return map[string]any{
			"shadow_refreshed": false,
			"warning":          "harness unavailable; agent shadow could not be refreshed",
		}
	}
	if h.kernel == nil || h.shadow == nil {
		return map[string]any{
			"shadow_refreshed": false,
			"warning":          "kernel reload unavailable; agent shadow could not be refreshed",
		}
	}
	if vsp, ok := h.vspKernel(); ok {
		kernelStarted := time.Now()
		observe, err := vsp.VSPStateSnapshot(ctx, "project.timeline")
		if h.logger != nil {
			h.logger.Info("[timing] kernel.vsp_state_snapshot ms=%d reason=%s err=%t",
				time.Since(kernelStarted).Milliseconds(), reason, err != nil)
		}
		if err == nil && observe != nil && observe.OK() {
			h.shadow.Initialize(observe.LegacyState)
			refreshStatus = "ok"
			return map[string]any{
				"shadow_refreshed": true,
				"reason":           reason,
				"source":           "vsp.state.snapshot",
				"revision":         observe.Revision,
				"project_epoch":    observe.ProjectEpoch,
				"snapshot_hash":    observe.SnapshotHash,
			}
		}
		if err != nil && h.logger != nil {
			h.logger.Warn("[harness] VSP shadow refresh after %s failed, falling back to legacy state: %v", reason, err)
		}
	}
	kernelStarted := time.Now()
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "get_project_state"})
	if h.logger != nil {
		h.logger.Info("[timing] kernel.send_command ms=%d command=get_project_state reason=%s err=%t",
			time.Since(kernelStarted).Milliseconds(), reason, err != nil)
	}
	if err != nil {
		refreshStatus = "kernel_error"
		refreshErr = true
		if h.logger != nil {
			h.logger.Warn("[harness] shadow refresh after %s failed: %v", reason, err)
		}
		return map[string]any{
			"shadow_refreshed": false,
			"warning":          err.Error(),
		}
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		h.shadow.Initialize(reply)
		refreshStatus = "ok"
		return map[string]any{
			"shadow_refreshed": true,
			"reason":           reason,
		}
	}
	refreshStatus = strings.TrimSpace(fmt.Sprint(reply["status"]))
	if refreshStatus == "" {
		refreshStatus = "unexpected"
	}
	refreshErr = true
	return map[string]any{
		"shadow_refreshed": false,
		"warning":          firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "get_project_state did not return ok"),
	}
}

func (h *Harness) withProjectRefresh(ctx context.Context, reason string, result map[string]any) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	refresh := h.refreshShadowWithStatus(ctx, reason)
	result["refresh"] = refresh
	if warning := firstString(refresh, "warning"); warning != "" {
		result["warnings"] = appendStringAny(result["warnings"], warning)
	}
	return result
}

func (h *Harness) withProjectReload(ctx context.Context, reason string, result map[string]any) map[string]any {
	if result == nil {
		result = map[string]any{}
	}
	refresh := map[string]any{
		"reason":            reason,
		"kernel_reloaded":   false,
		"shadow_refreshed":  false,
		"project_file_path": "",
	}
	projectFilePath := firstString(result, "project_file_path", "project_path")
	refresh["project_file_path"] = projectFilePath
	if h == nil || h.kernel == nil {
		warning := "kernel reload unavailable; project files were materialized but the open project could not be reloaded"
		refresh["warning"] = warning
		result["warnings"] = appendStringAny(result["warnings"], warning)
		result["refresh"] = refresh
		return result
	}
	if projectFilePath == "" {
		warning := "project reload path is unavailable"
		refresh["warning"] = warning
		result["warnings"] = appendStringAny(result["warnings"], warning)
	} else {
		reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "open_project", "file_path": projectFilePath})
		if err != nil {
			warning := err.Error()
			refresh["warning"] = warning
			result["warnings"] = appendStringAny(result["warnings"], warning)
			if h.logger != nil {
				h.logger.Warn("[harness] project reload after %s failed: %v", reason, err)
			}
		} else if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
			warning := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "open_project did not return ok")
			refresh["warning"] = warning
			result["warnings"] = appendStringAny(result["warnings"], warning)
		} else {
			refresh["kernel_reloaded"] = true
			refresh["kernel_status"] = firstNonEmpty(fmt.Sprint(reply["status"]), "ok")
		}
	}
	shadowRefresh := h.refreshShadowWithStatus(ctx, reason)
	refresh["shadow_refreshed"] = boolValueDefault(shadowRefresh["shadow_refreshed"], false)
	refresh["shadow_refresh"] = shadowRefresh
	if warning := firstString(shadowRefresh, "warning"); warning != "" {
		result["warnings"] = appendStringAny(result["warnings"], warning)
	}
	result["refresh"] = refresh
	return result
}

func PreviewCommand(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.Command(spec, cmd)
}

func PreviewRackAddNode(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.RackAddNode(spec, cmd)
}

func PreviewPluginGrabberProfileUpsert(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.PluginGrabberProfileUpsert(spec, cmd)
}

func PreviewPluginGrabberProfileRemove(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.PluginGrabberProfileRemove(spec, cmd)
}

func PreviewLegacyMidiNotes(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.LegacyMidiNotes(spec, cmd)
}

func PreviewMidiPatch(spec tools.CommandSpec, cmd map[string]any) string {
	return preview.MidiPatch(spec, cmd)
}

func validateRequiredTargetIDs(spec tools.CommandSpec, cmd map[string]any) error {
	if spec.CommandName == "track.group.apply_control" {
		return validateTrackGroupApplyControlTarget(spec, cmd)
	}
	for _, field := range spec.RequiredTargetIDs {
		if strings.TrimSpace(field) == "" {
			continue
		}
		v, ok := cmd[field]
		if !ok || isEmptyValue(v) {
			return fmt.Errorf("%s requires non-empty %s; use stable IDs from /agent/state user-visible tracks", spec.ToolName, field)
		}
	}
	return nil
}

func validateTrackGroupApplyControlTarget(spec tools.CommandSpec, cmd map[string]any) error {
	if firstString(cmd, "group_id", "id") != "" {
		return nil
	}
	trackIDs := stringSliceFromAny(firstPresentValue(cmd, "track_ids", "member_track_ids", "members"))
	if len(trackIDs) == 0 {
		return fmt.Errorf("%s requires non-empty group_id or explicit track_ids with create_group_if_missing", spec.ToolName)
	}
	createGroup, _ := boolValue(cmd["create_group_if_missing"])
	if createGroup {
		return nil
	}
	if createGroup, _ = boolValue(cmd["ensure_group"]); createGroup {
		return nil
	}
	if createGroup, _ = boolValue(cmd["create_group"]); createGroup {
		return nil
	}
	return fmt.Errorf("%s with track_ids requires create_group_if_missing=true", spec.ToolName)
}

func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}

func visibleTrackRows(state map[string]any) []map[string]any {
	for _, value := range []any{
		state["tracks"],
		mapFromAny(state["shadow"])["tracks"],
		mapFromAny(state["project_state"])["tracks"],
		mapFromAny(mapFromAny(state["project_state"])["shadow"])["tracks"],
	} {
		if rows := usableVisibleTrackRows(mapRowsFromAny(value)); len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func usableVisibleTrackRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		if firstString(row, "track_id", "id", "track_name", "name") == "" {
			continue
		}
		out = append(out, row)
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
		items := anySliceFromAny(v)
		if len(items) == 0 {
			return nil
		}
		out := make([]map[string]any, 0, len(items))
		for _, it := range items {
			if row := mapFromAny(it); len(row) > 0 {
				out = append(out, row)
			}
		}
		return out
	}
}

func mapFromAny(v any) map[string]any {
	if row, ok := v.(map[string]any); ok {
		return row
	}
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return nil
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return nil
	}
	return row
}

func anySliceFromAny(v any) []any {
	switch rows := v.(type) {
	case []any:
		return rows
	case []map[string]any:
		out := make([]any, 0, len(rows))
		for _, row := range rows {
			out = append(out, row)
		}
		return out
	default:
		data, err := json.Marshal(v)
		if err != nil || len(data) == 0 || string(data) == "null" {
			return nil
		}
		var decodedRows []any
		if err := json.Unmarshal(data, &decodedRows); err != nil {
			return nil
		}
		return decodedRows
	}
}

func visibleTrackID(row map[string]any) string {
	if id := firstString(row, "track_id", "id"); id != "" {
		return id
	}
	return ""
}

func visibleTrackName(row map[string]any) string {
	if name := firstString(row, "name", "track_name"); name != "" {
		return name
	}
	return visibleTrackID(row)
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok && !isEmptyValue(v) {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func stringSliceFromAny(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case []string:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(it)
			if s != "" {
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
	case map[string]any:
		out := make([]string, 0, len(x))
		for key, enabled := range x {
			if value, ok := boolValue(enabled); ok && !value {
				continue
			}
			s := strings.TrimSpace(key)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}

func mapStringStringFromAny(v any) map[string]string {
	switch x := v.(type) {
	case map[string]string:
		out := make(map[string]string, len(x))
		for k, v := range x {
			if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
				out[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(x))
		for k, v := range x {
			key := strings.TrimSpace(k)
			value := strings.TrimSpace(fmt.Sprint(v))
			if key != "" && value != "" && value != "<nil>" {
				out[key] = value
			}
		}
		return out
	default:
		return nil
	}
}

func firstPositiveInt(row map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok || isEmptyValue(v) {
			continue
		}
		switch x := v.(type) {
		case int:
			return x, x > 0
		case int64:
			return int(x), x > 0
		case float64:
			n := int(x)
			return n, n > 0
		case json.Number:
			n, err := strconv.Atoi(x.String())
			return n, err == nil && n > 0
		default:
			n, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
			return n, err == nil && n > 0
		}
	}
	return 0, false
}

func buildUndoLabel(spec tools.CommandSpec, cmd map[string]any) string {
	if !spec.SupportsUndo {
		return ""
	}
	if v := strings.TrimSpace(fmt.Sprint(cmd["undo_label"])); v != "" && v != "<nil>" {
		return v
	}
	switch {
	case spec.CommandName == "apply_midi_note_patch":
		return "Agent: MIDI note patch"
	case spec.MutatesProject:
		return "Agent: " + spec.CommandName
	default:
		return ""
	}
}

func confirmationStatus(needsConfirmation, confirmed bool) string {
	switch {
	case needsConfirmation:
		return "pending"
	case confirmed:
		return "confirmed"
	default:
		return "not_required"
	}
}

func fallback(v, fb string) string {
	if strings.TrimSpace(v) == "" {
		return fb
	}
	return strings.TrimSpace(v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && v != "<nil>" {
			return v
		}
	}
	return ""
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
