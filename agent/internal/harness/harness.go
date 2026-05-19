package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
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
	Context   map[string]any `json:"context,omitempty"`
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
	result := h.publicResult(spec, reply)
	resp := InvokeResponse{
		Status:               status,
		AgentActionID:        actionID,
		Tool:                 spec.ToolName,
		CommandName:          spec.CommandName,
		RiskLevel:            spec.RiskLevel,
		RequiresConfirmation: false,
		Result:               result,
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
		flattenCommandParams(cmd)
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
		flattenCommandParams(cmd)
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
	cmd := tools.BuildCommand(spec.CommandName, req.Args)
	flattenCommandParams(cmd)
	return cmd, spec, nil
}

func flattenCommandParams(cmd map[string]any) {
	params, ok := cmd["params"].(map[string]any)
	if !ok {
		return
	}
	for k, v := range params {
		if isEmptyValue(cmd[k]) && !isEmptyValue(v) {
			cmd[k] = v
		}
	}
	delete(cmd, "params")
}

func (h *Harness) resolveImplicitTargets(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) error {
	normalizeCommandArgs(spec, cmd)

	for _, field := range spec.RequiredTargetIDs {
		if field != "track_id" || !isEmptyValue(cmd[field]) {
			continue
		}
		trackID, err := h.resolveTrackID(ctx, spec, cmd, requestContext)
		if err != nil {
			return fmt.Errorf("%s could not resolve track target: %w", spec.ToolName, err)
		}
		if trackID != "" {
			cmd[field] = trackID
		}
	}
	return nil
}

func normalizeCommandArgs(spec tools.CommandSpec, cmd map[string]any) {
	if spec.CommandName != "rename_track" {
		return
	}
	if isEmptyValue(cmd["name"]) && !isEmptyValue(cmd["new_name"]) {
		cmd["name"] = cmd["new_name"]
	}
}

func (h *Harness) resolveTrackID(ctx context.Context, spec tools.CommandSpec, cmd map[string]any, requestContext map[string]any) (string, error) {
	if h == nil || h.shadow == nil {
		return "", nil
	}
	state := h.UserStateSummary(ctx)
	tracks := visibleTrackRows(state)
	if index, ok := firstPositiveInt(cmd, "user_track_index", "track_index", "track_number", "index"); ok {
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
	if selectedID := firstString(requestContext, "selected_track_id", "track_id"); selectedID != "" {
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

func (h *Harness) publicResult(spec tools.CommandSpec, reply map[string]any) map[string]any {
	switch spec.CommandName {
	case "get_project_state":
		if h.shadow != nil {
			return withOKStatus(userVisibleState(h.shadow.Summary()))
		}
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
	}
	return reply
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

func withOKStatus(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	out["status"] = "ok"
	for k, v := range in {
		out[k] = v
	}
	return out
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
