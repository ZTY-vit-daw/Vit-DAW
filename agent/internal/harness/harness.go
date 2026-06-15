package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
	logger        *logx.Logger
	snapshotCache *PluginSnapshotCache
}

type kernelSender interface {
	SendCommand(context.Context, map[string]any) (map[string]any, string, error)
}

const mixboardFeatureReadyWaitDefault = 1500 * time.Millisecond

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
		snapshotCache: NewPluginSnapshotCache(),
		logger:        logger,
	}
}

func defaultJournalPath() string {
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
			preview = "确认后，VitAgent 会先创建一个项目历史安全检查点。\n" + preview
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
	case "mix_request_observation":
		result, err := h.requestMixObservation(ctx, cmd)
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
				"param_name":   firstNonEmpty(firstString(binding, "param_name"), "轨道音量"),
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
	target := mixTargetFromCommand(cmd)
	if target.ID == "" {
		target.ID = firstString(cmd, "track_id", "clip_id", "target_id")
	}
	if target.Kind == "" {
		target.Kind = firstNonEmpty(firstString(cmd, "target_kind", "kind"), "selection")
	}
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
		"mixboard":          result.Board,
		"context_pack":      result.ContextPack,
	}
	if len(featureRequest) > 0 {
		out["feature_request"] = featureRequest
	}
	return out, nil
}

func (h *Harness) requestMixObservationFeatures(ctx context.Context, cmd map[string]any, state map[string]any, target mixboard.TargetRef) map[string]any {
	packet := newMixboardFeatureRequestPacket(cmd, target)
	resolved := resolveMixboardFeatureTarget(cmd, state, target)
	packet["resolved_target"] = resolved
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
		reply, _, err := h.kernel.SendCommand(ctx, kernelCmd)
		if err != nil || !kernelReplySucceeded(reply) {
			reason := firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), fmt.Sprint(err), "kernel_feature_request_failed")
			skipped = append(skipped, map[string]any{"feature_type": featureType, "reason": reason})
			continue
		}
		requested = append(requested, map[string]any{
			"feature_type": featureType,
			"request_id":   requestID,
			"track_id":     trackID,
			"clip_id":      clipID,
		})
	}
	packet["requested_features"] = requested
	packet["skipped_features"] = skipped
	if len(requested) > 0 {
		packet["status"] = "requested"
	} else {
		packet["status"] = "blocked"
		packet["reason"] = "all_feature_requests_skipped"
	}
	writeMixboardFeatureRequestSnapshot(cmd, packet)
	waitForMixboardFeatureSnapshotReady(ctx, cmd, trackID, clipID, mixboardFeatureReadyWait())
	return packet
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
	refs := visibleClipRefs(state)
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
	snapshot := map[string]any{
		"schema_version":          "mixboard_feature_snapshot.v1",
		"updated_at":              time.Now().UTC().Format(time.RFC3339Nano),
		"latest_request":          packet,
		"waveform_envelope":       pendingMixboardFeatureRow("waveform_envelope", packet, existing),
		"spectrogram_tiles":       pendingMixboardFeatureRow("spectral_field", packet, existing),
		"band_energy_summary":     reusableMixboardFeatureRow("band_energy_summary", existing, map[string]any{"status": "missing"}),
		"stereo_relation_summary": reusableMixboardFeatureRow("stereo_relation_summary", existing, map[string]any{"status": "missing"}),
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
	if !mixboardFeatureRowReadyForTarget(snapshot["waveform_envelope"], trackID, clipID) {
		return false
	}
	if mixboardFeatureRowReadyForTarget(snapshot["band_energy_summary"], trackID, "") &&
		mixboardFeatureRowReadyForTarget(snapshot["stereo_relation_summary"], trackID, "") {
		return true
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
	if !isEmptyValue(out["project_snapshot_xml"]) || h == nil || h.kernel == nil {
		return out
	}
	preferKernelProjectPath := boolValueDefault(out["prefer_kernel_project_path"], false)
	delete(out, "prefer_kernel_project_path")
	reply, _, err := h.kernel.SendCommand(context.Background(), map[string]any{"cmd": "project_snapshot_export"})
	if err != nil {
		out["snapshot_export_error"] = err.Error()
		return out
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		out["snapshot_export_error"] = firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "project_snapshot_export failed")
		return out
	}
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
	if !isEmptyValue(out["project_snapshot_xml"]) || h == nil || h.kernel == nil {
		return out
	}
	reply, _, err := h.kernel.SendCommand(context.Background(), map[string]any{"cmd": "project_snapshot_export"})
	if err != nil {
		out["snapshot_export_error"] = err.Error()
		return out
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		out["snapshot_export_error"] = firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "project_snapshot_export failed")
		return out
	}
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
			[]string{"unmute", "取消静音", "解除静音", "取消mute", "取消 mute", "关闭静音"},
			[]string{"mute", "静音"})
	case "set_solo":
		copyBoolAlias(cmd, "solo", "is_solo", "enabled", "value")
		inferBoolFromUserMessage(cmd, "solo", requestContext,
			[]string{"unsolo", "取消独奏", "解除独奏", "取消solo", "取消 solo", "关闭独奏"},
			[]string{"solo", "独奏"})
	case "arm_track":
		copyBoolAlias(cmd, "is_armed", "arm", "armed", "enabled", "value")
		inferBoolFromUserMessage(cmd, "is_armed", requestContext,
			[]string{"disarm", "取消录音准备", "解除录音准备", "取消arm", "取消 arm"},
			[]string{"arm", "录音准备"})
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
	quotedAudioPathPattern = regexp.MustCompile(`(?i)["'“”‘’]([^"'“”‘’]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))["'“”‘’]?`)
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
	return strings.Trim(path, " \t\r\n\"'`“”‘’.,，。;；:：)）]】")
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
	if !containsTextAnyFold(text, "搜索", "查找", "找", "search", "find") {
		return ""
	}
	cleaned := text
	for _, phrase := range []string{
		"搜索", "查找", "找一下", "找到", "找", "并导入", "然后导入", "导入", "放到", "放进", "拖入",
		"资料库", "素材库", "当前轨道", "选中轨道", "这个轨道", "这条轨道", "音频", "素材",
		"search", "find", "import", "add", "audio", "sample", "current track", "selected track", "track", "and", "to",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
		cleaned = strings.ReplaceAll(cleaned, strings.Title(phrase), " ")
	}
	replacer := strings.NewReplacer("，", " ", "。", " ", ",", " ", ".", " ", "；", " ", ";", " ", "：", " ", ":", " ", "（", " ", "）", " ", "(", " ", ")", " ", "并", " ", "到", " ", "给", " ")
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
	replacer := strings.NewReplacer("_", " ", "-", " ", ".", " ", ",", " ", "，", " ", "。", " ", "/", " ", "\\", " ")
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
			return fmt.Errorf("split_clip requires split_time; say a time like 在 2 秒处切开, or move the playhead and say 在播放头切开")
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
			return fmt.Errorf("clone_clip requires new_start; say where to place the copy, e.g. 复制到 20 秒")
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

var secondsInTextPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|秒)`)

func inferMoveStartSeconds(ref clipRef, requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	seconds, ok := firstSecondsInText(text)
	if !ok {
		return 0, false
	}
	lower := strings.ToLower(text)
	current := numberFromAny(ref.Row["start_seconds"])
	switch {
	case strings.Contains(text, "向前") || strings.Contains(text, "前移") || strings.Contains(text, "提前") || strings.Contains(lower, "earlier") || strings.Contains(lower, "left") || strings.Contains(lower, "backward"):
		next := current - seconds
		if next < 0 {
			next = 0
		}
		return next, true
	case strings.Contains(text, "向后") || strings.Contains(text, "后移") || strings.Contains(text, "推后") || strings.Contains(lower, "later") || strings.Contains(lower, "right") || strings.Contains(lower, "forward"):
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
	if strings.TrimSpace(text) == "" || isCloneAfterText(text) || containsTextAnyFold(text, "复制", "克隆", "拷贝", "duplicate", "clone", "copy") {
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
	if containsTextAnyFold(text, "内部", "里面", "从开头", "从起点", "相对", "offset", "into clip", "from start") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if containsTextAnyFold(text, "后", "之后", "later", "after") && !containsTextAnyFold(text, "秒处", "s mark", "at") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	if strings.Contains(lower, "relative") {
		return numberFromAny(ref.Row["start_seconds"]) + seconds
	}
	return seconds
}

func mentionsPlayheadSplit(text string) bool {
	return containsTextAnyFold(text, "播放头", "当前位置", "这里", "此处", "当前时间", "playhead", "cursor", "current position", "here")
}

func mentionsPlayheadTarget(text string) bool {
	return containsTextAnyFold(text, "播放头", "当前位置", "这里", "此处", "当前时间", "playhead", "cursor", "current position", "here")
}

func isCloneAfterText(text string) bool {
	return containsTextAnyFold(text, "后面", "后边", "后方", "后面一份", "后续", "紧接", "后", "after", "next", "right after")
}

func inferLengthSeconds(requestContext map[string]any) (float64, bool) {
	text := strings.TrimSpace(firstString(requestContext, "user_message", "message", "prompt", "utterance"))
	if !isResizeLengthText(text) {
		return 0, false
	}
	return firstSecondsInText(text)
}

func isResizeLengthText(text string) bool {
	if containsTextAnyFold(text, "位置", "起点", "开始位置", "移动", "移到", "挪到", "拖到", "move", "position", "start") {
		return false
	}
	return containsTextAnyFold(text,
		"长度", "时长", "持续", "裁到", "裁成", "剪到", "剪成", "剪短",
		"缩到", "缩短", "拉长", "拉到", "伸到", "改成", "改为", "变成",
		"调整到", "调成", "resize", "length", "duration", "trim", "shorten")
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
			if name == "macro"+strconv.Itoa(ordinal) || name == "宏控件"+strconv.Itoa(ordinal) || name == "宏控制"+strconv.Itoa(ordinal) {
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
	for _, part := range []string{" ", "\t", "\r", "\n", "_", "-", "#", "＃", "号", "號"} {
		out = strings.ReplaceAll(out, part, "")
	}
	return out
}

var macroReferencePattern = regexp.MustCompile(`(?i)(?:macro|marco|宏控(?:件|制)?|宏滑块|宏滑桿|宏)\s*#?\s*([0-9]+)`)

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
	hasBind := strings.Contains(lower, "bind") || strings.Contains(lower, "map") || strings.Contains(text, "绑定") || strings.Contains(text, "綁定") || strings.Contains(text, "捆绑") || strings.Contains(text, "映射") || strings.Contains(text, "绑到") || strings.Contains(text, "綁到")
	hasMacroRef := macroReferenceFromUserText(text) != "" || strings.Contains(lower, "macro") || strings.Contains(lower, "marco") || strings.Contains(text, "宏控")
	return hasBind && hasMacroRef
}

func referencesCurrentMacro(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(lower, "this macro") || strings.Contains(lower, "current macro") || strings.Contains(text, "这个宏") || strings.Contains(text, "這個宏") || strings.Contains(text, "当前宏") || strings.Contains(text, "目前宏")
}

func isGenericMacroReferenceName(name string) bool {
	normalized := normalizeMacroReferenceText(name)
	return normalized == "" || normalized == "macro" || normalized == "agentmacro" || normalized == "宏控件" || normalized == "宏控制" || normalized == "通用宏控件"
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
	switch rows := state["tracks"].(type) {
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
