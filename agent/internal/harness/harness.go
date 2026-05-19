package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/tools"
)

type Harness struct {
	kernel  *kernel.Client
	shadow  *shadow.Project
	catalog *tools.Catalog
	journal *journal.Journal
	logger  *logx.Logger
}

type InvokeRequest struct {
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
	Command   map[string]any `json:"command"`
	Source    string         `json:"source"`
	Confirmed bool           `json:"confirmed"`
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
	Error                string          `json:"error,omitempty"`
}

func New(kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Harness {
	return &Harness{
		kernel:  kernelClient,
		shadow:  shadowProject,
		catalog: tools.DefaultCatalog(),
		journal: journal.New(300),
		logger:  logger,
	}
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

func (h *Harness) DirectCommandNames() []string {
	if h == nil || h.catalog == nil {
		return nil
	}
	return h.catalog.DirectCommandNames()
}

func (h *Harness) ModelCatalogSummary() string {
	if h == nil || h.catalog == nil {
		return ""
	}
	return h.catalog.ModelSummary()
}

func (h *Harness) Invoke(ctx context.Context, req InvokeRequest) (InvokeResponse, error) {
	if h == nil {
		return InvokeResponse{Status: "error", Error: "harness is nil"}, fmt.Errorf("harness is nil")
	}
	cmd, spec, err := h.resolveCommand(req)
	if err != nil {
		resp := InvokeResponse{Status: "error", Error: err.Error()}
		return resp, err
	}

	actionID := "act_" + randomID()
	undoLabel := buildUndoLabel(spec, cmd)
	cmd["agent_action_id"] = actionID
	if undoLabel != "" {
		cmd["undo_label"] = undoLabel
	}

	needsConfirmation := spec.RequiresConfirmation && !req.Confirmed
	action := journal.Action{
		AgentActionID:        actionID,
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
		resp := InvokeResponse{
			Status:               "needs_confirmation",
			AgentActionID:        actionID,
			Tool:                 spec.ToolName,
			CommandName:          spec.CommandName,
			RiskLevel:            spec.RiskLevel,
			RequiresConfirmation: true,
			Preview:              PreviewCommand(spec, cmd),
			UndoLabel:            undoLabel,
		}
		return resp, nil
	}

	h.journal.Record(action)
	if h.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:        "error",
			AgentActionID: actionID,
			Tool:          spec.ToolName,
			CommandName:   spec.CommandName,
			RiskLevel:     spec.RiskLevel,
			UndoLabel:     undoLabel,
			Error:         err.Error(),
		}
		return resp, err
	}
	reply, _, err := h.kernel.SendCommand(ctx, cmd)
	if err != nil {
		h.journal.MarkResult(actionID, journal.StatusFailed, nil, err)
		resp := InvokeResponse{
			Status:        "error",
			AgentActionID: actionID,
			Tool:          spec.ToolName,
			CommandName:   spec.CommandName,
			RiskLevel:     spec.RiskLevel,
			UndoLabel:     undoLabel,
			Error:         err.Error(),
		}
		return resp, err
	}

	h.afterKernelReply(ctx, spec, reply)
	status := "ok"
	journalStatus := journal.StatusSucceeded
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		status = "kernel_error"
		journalStatus = journal.StatusFailed
		err = fmt.Errorf("%s", firstNonEmpty(fmt.Sprint(reply["message"]), fmt.Sprint(reply["error"]), "kernel command failed"))
	}
	h.journal.MarkResult(actionID, journalStatus, reply, err)
	resp := InvokeResponse{
		Status:               status,
		AgentActionID:        actionID,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		RiskLevel:            spec.RiskLevel,
		RequiresConfirmation: false,
		Result:               reply,
		UndoLabel:            undoLabel,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, err
}

func (h *Harness) resolveCommand(req InvokeRequest) (map[string]any, tools.CommandSpec, error) {
	if h == nil || h.catalog == nil {
		return nil, tools.CommandSpec{}, fmt.Errorf("command catalog is unavailable")
	}
	if len(req.Command) > 0 {
		cmd := tools.CloneCommand(req.Command)
		name := tools.CommandName(cmd)
		if name == "" {
			return nil, tools.CommandSpec{}, fmt.Errorf("command is missing cmd/action/command")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
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
		name := tools.CommandName(cmd)
		if name == "" {
			return nil, tools.CommandSpec{}, fmt.Errorf("daw.invoke args must include cmd/action/command")
		}
		spec, ok := h.catalog.LookupCommand(name)
		if !ok {
			return nil, tools.CommandSpec{}, fmt.Errorf("unknown or unregistered DAW command: %s", name)
		}
		return cmd, spec, nil
	}

	spec, ok := h.catalog.LookupTool(toolName)
	if !ok {
		return nil, tools.CommandSpec{}, fmt.Errorf("unknown tool: %s", toolName)
	}
	return tools.BuildCommand(spec.CommandName, req.Args), spec, nil
}

func (h *Harness) afterKernelReply(ctx context.Context, spec tools.CommandSpec, reply map[string]any) {
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		return
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

func (h *Harness) refreshShadow(ctx context.Context, reason string) {
	if h.kernel == nil || h.shadow == nil {
		return
	}
	reply, _, err := h.kernel.SendCommand(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("[harness] shadow refresh after %s failed: %v", reason, err)
		}
		return
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
		h.shadow.Initialize(reply)
	}
}

func PreviewCommand(spec tools.CommandSpec, cmd map[string]any) string {
	raw, _ := json.Marshal(cmd)
	return fmt.Sprintf("%s [%s]: %s\n%s", spec.CommandName, spec.RiskLevel, spec.Description, string(raw))
}

func buildUndoLabel(spec tools.CommandSpec, cmd map[string]any) string {
	if !spec.SupportsUndo {
		return ""
	}
	if v := strings.TrimSpace(fmt.Sprint(cmd["undo_label"])); v != "" && v != "<nil>" {
		return v
	}
	switch {
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
