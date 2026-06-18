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
	"syscall"
	"time"
	"unsafe"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/artifacts"
	"vit-daw-agent/internal/browsercapture"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/macrocontrols"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/preview"
	"vit-daw-agent/internal/resourceintake"
	"vit-daw-agent/internal/rollback"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/shelltools"
	"vit-daw-agent/internal/tools"
	"vit-daw-agent/internal/webtools"
	"vit-daw-agent/internal/workflows/plugingrabber"
	"vit-daw-agent/internal/workspace"
)

type Harness struct {
	kernel        kernelSender
	shadow        *shadow.Project
	catalog       *tools.Catalog
	journal       *journal.Journal
	runtime       *agentruntime.Runtime
	mixTicks      *mixTickStore
	logger        *logx.Logger
	snapshotCache *PluginSnapshotCache
}

type kernelSender interface {
	SendCommand(context.Context, map[string]any) (map[string]any, string, error)
}

const mixboardFeatureReadyWaitDefault = 1500 * time.Millisecond
const mixboardFeatureSubURL = "tcp://127.0.0.1:5556"

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
	j, err := journal.NewPersistent(500, defaultJournalPath())
	if err != nil {
		j = journal.New(500)
	}
	return &Harness{
		kernel:        kernelClient,
		shadow:        shadowProject,
		catalog:       tools.DefaultCatalog(),
		journal:       j,
		runtime:       agentruntime.New(),
		mixTicks:      newMixTickStore(),
		snapshotCache: NewPluginSnapshotCache(),
		logger:        logger,
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

func (h *Harness) ProjectHistorySummary(ctx context.Context, goalID string) map[string]any {
	return h.ProjectHistorySummaryForProject(ctx, goalID, "")
}

func (h *Harness) ProjectHistorySummaryForProject(ctx context.Context, goalID, projectPath string) map[string]any {
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
	if result, ok := h.invokeLocal(ctx, spec, cmd); ok {
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

	// Translate plugin_grabber_* commands to kernel-known commands.
	// These are registered in the catalog for LLM tool calling, but the Godot
	// kernel only knows the n_* (or get_plugin_parameters) variants.
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
	kernelStarted := time.Now()
	reply, _, err := h.kernel.SendCommand(ctx, cmd)
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

	refreshStarted := time.Now()
	h.afterKernelReply(ctx, spec, reply)
	if h.logger != nil {
		h.logger.Info("[timing] harness.after_kernel_reply ms=%d command=%s status=%s",
			time.Since(refreshStarted).Milliseconds(), spec.CommandName, strings.TrimSpace(fmt.Sprint(reply["status"])))
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

func broadMixObserveFirstWriteGuard(requestContext map[string]any, spec tools.CommandSpec, cmd map[string]any) error {
	if !broadMixWriteCommand(spec, cmd) {
		return nil
	}
	userText := broadMixGuardUserText(requestContext)
	if !broadMixNaturalRequest(userText) || broadMixExplicitPluginOrRawRequest(userText) {
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

func broadMixExplicitPluginOrRawRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasExplicitVerb := broadMixTextHasAny(text,
		"\u52a0\u8f7d", "\u6302\u8f7d", "\u6253\u5f00", "\u5b66\u4e60", "\u6293\u624b", "\u63d2\u5165", "\u65b0\u589e",
		"\u8bbe\u7f6e\u53c2\u6570", "\u5199\u53c2\u6570", "\u6539\u53c2\u6570", "\u8c03\u53c2\u6570",
		"load", "insert", "open", "learn", "grabber", "set parameter", "write parameter",
	)
	hasPluginObject := broadMixTextHasAny(text,
		"\u63d2\u4ef6", "\u6548\u679c\u5668", "\u5747\u8861\u5668", "\u538b\u7f29\u5668", "\u6df7\u54cd", "\u5ef6\u8fdf",
		"plugin", "vst", "eq", "compressor", "reverb", "delay", "tdr", "nova", "zl",
	)
	hasRawParam := broadMixTextHasAny(text, "param_id", "parameter id", "\u53c2\u6570 id", "\u5f52\u4e00\u5316", "normalized")
	return hasRawParam || (hasExplicitVerb && hasPluginObject)
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

func (h *Harness) invokeLocal(ctx context.Context, spec tools.CommandSpec, cmd map[string]any) (map[string]any, bool) {
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
	case "mix_rollback_tick":
		result, err := h.rollbackMixTick(ctx, cmd)
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
		result, err := history.ProjectNew(cmd)
		return resultWithErr(result, err), true
	case "version_project_saved":
		result, err := history.ProjectSaved(h.historyArgs(cmd))
		if err == nil {
			result = h.retagArtifactsForSavedProject(ctx, cmd, result)
		}
		return resultWithErr(result, err), true
	case "version_checkout":
		result, err := history.Checkout(h.historyArgs(cmd))
		result = h.withProjectReload(ctx, "version_checkout", result)
		return resultWithErr(result, err), true
	default:
		return nil, false
	}
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
			reply, _, err := h.kernel.SendCommand(ctx, map[string]any{
				"cmd":      "set_volume",
				"track_id": trackID,
				"db":       targetValue,
			})
			if err != nil || !kernelReplySucceeded(reply) {
				return map[string]any{
					"status":   "error",
					"error":    firstNonEmpty(firstString(reply, "message"), firstString(reply, "error"), fmt.Sprint(err), "track.volume macro binding failed"),
					"macro_id": macroID,
					"value":    value,
					"macro":    macro,
				}
			}
			if h.catalog != nil {
				if volumeSpec, ok := h.catalog.LookupCommand("set_volume"); ok {
					h.afterKernelReply(ctx, volumeSpec, reply)
				}
			}
			applied = append(applied, map[string]any{
				"track_id":     trackID,
				"control":      "track.volume",
				"param_id":     "track.volume",
				"param_name":   firstNonEmpty(firstString(binding, "param_name"), "\u8f68\u9053\u97f3\u91cf"),
				"target_value": targetValue,
				"unit":         firstString(binding, "unit"),
			})
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
		reply, _, err := h.kernel.SendCommand(context.Background(), map[string]any{"cmd": "project_snapshot_export"})
		if err == nil && !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
			return reply
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
	state := map[string]any{}
	if h != nil {
		state = h.UserStateSummary(ctx)
	}
	cmd = cloneAnyMap(cmd)
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
	if mixObservationResolutionNeedsRefresh(resolvedContext) && h != nil {
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
	featureRequest := h.requestMixObservationFeatures(ctx, cmd, state, target)
	result, err := mixboard.NewStore("").RequestObservation(mixboard.Request{
		MixSessionID: firstString(cmd, "mix_session_id", "session_id"),
		Round:        int(numberFromAny(cmd["round"])),
		GoalText:     firstString(cmd, "goal_text", "goal"),
		TargetRef:    target,
		MixObjects:   mixObjectsFromCommand(cmd),
		ListenScope:  mixListenScopeFromCommand(cmd),
		ProjectState: state,
		Args:         cloneAnyMap(cmd),
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
	if len(featureRequest) > 0 {
		out["feature_request"] = featureRequest
	}
	return out, nil
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

func canonicalizeMixObservationCommand(cmd map[string]any, target mixboard.TargetRef, resolved map[string]any) map[string]any {
	cmd = cloneAnyMap(cmd)
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
	requested := make([]any, 0, 1)
	skipped := make([]any, 0)
	for _, featureType := range []string{"waveform_envelope"} {
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
		collector, _ := collectWaveformFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
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
	requested := make([]any, 0, len(targets))
	skipped := make([]any, 0)
	for _, targetRow := range targets {
		trackID := firstString(targetRow, "track_id")
		clipID := firstString(targetRow, "clip_id")
		if trackID == "" || clipID == "" {
			skipped = append(skipped, map[string]any{
				"feature_type": "waveform_envelope",
				"track_id":     trackID,
				"clip_id":      clipID,
				"reason":       "clip_source_required_for_current_feature_bakers",
			})
			continue
		}
		requestID := fmt.Sprintf("%s_waveform_envelope_%s", firstString(packet, "request_id"), safeRequestIDPart(trackID))
		requestRow := map[string]any{
			"feature_type": "waveform_envelope",
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
			"feature_type":        "waveform_envelope",
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
		collector, _ := collectWaveformFeatureTiles(ctx, mixboardFeatureSubURL, trackID, clipID, mixboardFeatureCollectWait(), sendBake)
		if !sent {
			_ = sendBake()
		}
		if !kernelReplySucceeded(reply) {
			reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel_feature_request_failed")
			skipped = append(skipped, map[string]any{"feature_type": "waveform_envelope", "track_id": trackID, "clip_id": clipID, "reason": reason})
			continue
		}
		requested = append(requested, requestRow)
		if collector != nil && collector.TileCount() > 0 {
			if row := collector.SnapshotRow(firstString(packet, "request_id")); len(row) > 0 {
				writeMixboardReadyWaveformSnapshot(cmd, packet, row)
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
	waitForMixboardProjectFeatureSnapshotReady(ctx, cmd, targets, mixboardFeatureReadyWait())
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
	TrackID         string
	ClipID          string
	FilePath        string
	TotalDuration   float64
	ExpectedTiles   int
	FrameCount      int
	FeatureStride   int
	FloatCount      int
	TilesSeen       int
	PeakAbs         float64
	SumSquares      float64
	SampleFrames    int64
	TimeSegments    []map[string]any
	FirstReceivedAt string
	LastReceivedAt  string
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
	c.FloatCount += floatCount
	if c.ExpectedTiles <= 0 {
		c.ExpectedTiles = expectedWaveformTileCount(c.TotalDuration)
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
	c.TilesSeen++
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
	return c.TilesSeen
}

func (c *waveformFeatureCollector) Complete() bool {
	if c == nil || c.TilesSeen <= 0 {
		return false
	}
	return c.ExpectedTiles > 0 && c.TilesSeen >= c.ExpectedTiles
}

func (c *waveformFeatureCollector) SnapshotRow(requestID string) map[string]any {
	if c == nil || c.TilesSeen <= 0 || c.SampleFrames <= 0 {
		return nil
	}
	rms := math.Sqrt(c.SumSquares / float64(c.SampleFrames))
	row := map[string]any{
		"status":              "ready",
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
		"tile_count_seen":     c.TilesSeen,
		"tile_count_expected": c.ExpectedTiles,
		"time_segments":       c.TimeSegments,
		"updated_at":          firstNonEmpty(c.LastReceivedAt, time.Now().UTC().Format(time.RFC3339Nano)),
	}
	if c.FirstReceivedAt != "" {
		row["first_received_at"] = c.FirstReceivedAt
	}
	return row
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
	data, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeMixboardReadyProjectTrackWaveformSnapshot(cmd map[string]any, packet map[string]any, waveform map[string]any) {
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
	data, err := json.MarshalIndent(existing, "", "\t")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
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

func newMixboardFeatureRequestPacket(cmd map[string]any, target mixboard.TargetRef) map[string]any {
	requestID := firstNonEmpty(firstString(cmd, "mixboard_request_id", "request_id"), "mixboard_"+time.Now().UTC().Format("20060102T150405.000000000"))
	return map[string]any{
		"schema_version":     "mixboard_feature_request.v1",
		"request_id":         requestID,
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
	path := mixboard.FeatureSnapshotPath(cmd)
	existing := readMixboardFeatureSnapshotFile(path)
	targetRows := trackWaveformRowsForPacket(packet, existing)
	snapshot := map[string]any{
		"schema_version":           "mixboard_feature_snapshot.v1",
		"updated_at":               time.Now().UTC().Format(time.RFC3339Nano),
		"latest_request":           packet,
		"waveform_envelope":        pendingMixboardFeatureRow("waveform_envelope", packet, existing),
		"track_waveform_envelopes": targetRows,
		"spectrogram_tiles":        pendingMixboardFeatureRow("spectral_field", packet, existing),
		"band_energy_summary":      reusableMixboardFeatureRow("band_energy_summary", existing, map[string]any{"status": "missing"}),
		"stereo_relation_summary":  reusableMixboardFeatureRow("stereo_relation_summary", existing, map[string]any{"status": "missing"}),
	}
	data, err := json.MarshalIndent(snapshot, "", "\t")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
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

func skippedFeatureReason(packet map[string]any, trackID, clipID string) string {
	for _, raw := range anyListFromAny(packet["skipped_features"]) {
		item, _ := raw.(map[string]any)
		if !strings.EqualFold(firstString(item, "feature_type"), "waveform_envelope") {
			continue
		}
		if firstString(item, "track_id") == trackID && firstString(item, "clip_id") == clipID {
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
	return trackID != "" || clipID != ""
}

func featureTargetKey(row map[string]any) string {
	trackID := firstString(row, "track_id")
	clipID := firstString(row, "clip_id")
	if trackID == "" && clipID == "" {
		return ""
	}
	return trackID + "::" + clipID
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
	if !isEmptyValue(out["project_path"]) {
		return out
	}
	if h != nil {
		state := h.UserStateSummary(context.Background())
		if path := projectPathFromState(state); path != "" {
			out["project_path"] = path
		}
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
	for _, track := range tracks {
		trackID := visibleTrackID(track)
		if strings.TrimSpace(trackScope) != "" && trackID != strings.TrimSpace(trackScope) {
			continue
		}
		for _, plugin := range mapRowsFromAny(track["plugins"]) {
			id := firstString(plugin, "plugin_id", "item_id", "id")
			if id == "" {
				continue
			}
			out = append(out, pluginRef{
				ID:      id,
				Name:    firstNonEmpty(firstString(plugin, "plugin_name", "name"), id),
				TrackID: trackID,
			})
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
	if spec.RefreshAfter {
		h.refreshShadow(ctx, spec.CommandName)
	}
}

func kernelReplySucceeded(reply map[string]any) bool {
	status := strings.ToLower(strings.TrimSpace(fmt.Sprint(reply["status"])))
	return status == "ok" || status == "success"
}

func (h *Harness) publicResult(spec tools.CommandSpec, cmd map[string]any, reply map[string]any) map[string]any {
	switch spec.CommandName {
	case "get_project_state":
		if h.shadow != nil {
			return withOKStatus(userVisibleState(h.shadow.Summary()))
		}
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
	includeParameters, _ := boolValue(cmd["include_parameters"])
	if !includeParameters {
		includeParameters, _ = boolValue(cmd["include_full_parameters"])
	}
	if !includeParameters {
		includeParameters, _ = boolValue(cmd["include_parameter_snapshot"])
	}
	if includeParameters {
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
		if rows := mapRowsFromAny(value); len(rows) > 0 {
			return rows
		}
	}
	return nil
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
