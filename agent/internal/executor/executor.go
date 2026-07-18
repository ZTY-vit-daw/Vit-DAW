package executor

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/planner"
)

type Executor struct {
	Harness *harness.Harness
}

type Input struct {
	GoalID    string
	RunID     string
	ToolCall  planner.ToolCall
	Context   map[string]any
	Confirmed bool
	Source    string
}

type Result struct {
	ToolCallID           string
	Tool                 string
	CommandName          string
	AgentActionID        string
	Status               string
	RequiresConfirmation bool
	Preview              string
	UndoLabel            string
	Result               map[string]any
	ProjectHistory       map[string]any
	ObservedState        map[string]any
	Error                string
	Response             harness.InvokeResponse
}

func New(h *harness.Harness) *Executor {
	return &Executor{Harness: h}
}

func (e *Executor) RunToolCall(ctx context.Context, in Input) (Result, error) {
	call := coercePluginInstantiateCall(in.ToolCall)
	toolCallID := strings.TrimSpace(call.ID)
	if toolCallID == "" {
		// Harness will create one too, but keeping one here makes the trace stable.
		toolCallID = "tool_goal_step"
	}
	if e == nil || e.Harness == nil {
		return Result{ToolCallID: toolCallID, Tool: call.Tool, Status: "error", Error: "executor harness is nil"}, fmt.Errorf("executor harness is nil")
	}
	req := harness.InvokeRequest{
		Tool:       strings.TrimSpace(call.Tool),
		Args:       cloneMap(call.Args),
		Command:    cloneMap(call.Command),
		Context:    contextWithIDs(in.Context, in.GoalID, in.RunID, toolCallID),
		Source:     firstNonEmpty(in.Source, "agentloop"),
		Confirmed:  in.Confirmed,
		GoalID:     in.GoalID,
		RunID:      in.RunID,
		ToolCallID: toolCallID,
	}
	resp, err := e.Harness.Invoke(ctx, req)
	out := Result{
		ToolCallID:           toolCallID,
		Tool:                 firstNonEmpty(resp.Tool, call.Tool),
		CommandName:          resp.CommandName,
		AgentActionID:        resp.AgentActionID,
		Status:               resp.Status,
		RequiresConfirmation: resp.RequiresConfirmation || resp.Status == "needs_confirmation" || resultRequiresConfirmation(resp.Result),
		Preview:              resp.Preview,
		UndoLabel:            resp.UndoLabel,
		Result:               resp.Result,
		ProjectHistory:       resp.ProjectHistory,
		Error:                resp.Error,
		Response:             resp,
	}
	if err != nil && out.Error == "" {
		out.Error = err.Error()
	}
	if !out.RequiresConfirmation && err == nil && !isErrorStatus(out.Status) {
		out.ObservedState = e.Harness.UserStateSummary(ctx)
	}
	return out, nil
}

func resultRequiresConfirmation(result map[string]any) bool {
	if result == nil {
		return false
	}
	value, ok := result["requires_confirmation"]
	if ok && truthyResultValue(value) {
		return true
	}
	return resultHasExecutablePendingAction(result)
}

func resultHasExecutablePendingAction(result map[string]any) bool {
	for _, action := range resultPendingActionRows(result) {
		tool := strings.TrimSpace(firstNonEmpty(firstMapText(action, "tool_name", "tool"), commandName(resultMapValue(action["args"]))))
		if strings.EqualFold(tool, "clip.strip_silence.apply") {
			return true
		}
	}
	return false
}

func resultPendingActionRows(result map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	if row := resultMapValue(result["pending_action"]); len(row) > 0 {
		rows = append(rows, row)
	}
	switch typed := result["pending_actions"].(type) {
	case []map[string]any:
		rows = append(rows, typed...)
	case []any:
		for _, item := range typed {
			if row := resultMapValue(item); len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func resultMapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func truthyResultValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		return text == "true" || text == "yes" || text == "1"
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}

func coercePluginInstantiateCall(call planner.ToolCall) planner.ToolCall {
	tool := strings.TrimSpace(call.Tool)
	cmdName := commandName(call.Command)
	if cmdName == "" {
		cmdName = commandName(call.Args)
	}
	if !strings.EqualFold(tool, "plugin.instantiate") && !strings.EqualFold(cmdName, "instantiate_plugin") {
		return call
	}

	merged := cloneMap(call.Command)
	if merged == nil {
		merged = map[string]any{}
	}
	for k, v := range call.Args {
		if isEmpty(merged[k]) {
			merged[k] = v
		}
	}

	if path := firstMapText(merged, "plugin_path", "path", "file_path", "plugin_file", "source_path"); path != "" {
		args := map[string]any{"plugin_path": path}
		copyTextArg(args, merged, "track_id", "track_id", "target_track_id", "selected_track_id", "selected_plugin_track_id")
		copyAnyArg(args, merged, "x", "x")
		copyAnyArg(args, merged, "y", "y")
		copyTextArg(args, merged, "zone_id", "zone_id")
		call.Tool = "plugin.load_to_rack"
		call.Args = args
		call.Command = nil
		return call
	}

	if query := firstMapText(merged, "plugin_query", "query", "plugin_name", "name", "plugin"); query != "" {
		call.Tool = "plugin.semantic_search"
		call.Args = map[string]any{"query": query, "limit": 8}
		call.Command = nil
		return call
	}

	return call
}

func contextWithIDs(in map[string]any, goalID, runID, toolCallID string) map[string]any {
	out := cloneMap(in)
	if out == nil {
		out = map[string]any{}
	}
	if strings.TrimSpace(goalID) != "" {
		out["goal_id"] = strings.TrimSpace(goalID)
	}
	if strings.TrimSpace(runID) != "" {
		out["run_id"] = strings.TrimSpace(runID)
	}
	if strings.TrimSpace(toolCallID) != "" {
		out["tool_call_id"] = strings.TrimSpace(toolCallID)
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func commandName(cmd map[string]any) string {
	for _, key := range []string{"cmd", "command", "action"} {
		if value := firstMapText(cmd, key); value != "" {
			return value
		}
	}
	return ""
}

func firstMapText(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if m == nil {
			return ""
		}
		if value, ok := m[key]; ok && !isEmpty(value) {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func copyTextArg(dst, src map[string]any, target string, keys ...string) {
	if value := firstMapText(src, keys...); value != "" {
		dst[target] = value
	}
}

func copyAnyArg(dst, src map[string]any, target string, keys ...string) {
	for _, key := range keys {
		if value, ok := src[key]; ok && !isEmpty(value) {
			dst[target] = value
			return
		}
	}
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func isErrorStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "error", "kernel_error", "failed":
		return true
	default:
		return false
	}
}
