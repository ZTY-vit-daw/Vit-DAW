package chat

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

const agentEventBufferLimit = 500

type AgentEvent struct {
	Seq            int64          `json:"seq"`
	Type           string         `json:"type"`
	ConversationID string         `json:"conversation_id,omitempty"`
	GoalID         string         `json:"goal_id,omitempty"`
	RunID          string         `json:"run_id,omitempty"`
	ItemID         string         `json:"item_id,omitempty"`
	ItemType       string         `json:"item_type,omitempty"`
	Status         string         `json:"status,omitempty"`
	Title          string         `json:"title,omitempty"`
	Body           string         `json:"body,omitempty"`
	Payload        map[string]any `json:"payload,omitempty"`
	CreatedAt      string         `json:"created_at,omitempty"`
}

type AgentEventsResponse struct {
	Status string       `json:"status"`
	Events []AgentEvent `json:"events"`
	NextSeq int64        `json:"next_seq"`
}

func (s *Server) handleAgentEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
		return
	}
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	if conversationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "conversation_id is required"})
		return
	}
	since, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("since")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if limit <= 0 || limit > agentEventBufferLimit {
		limit = 120
	}
	events, nextSeq := s.agentEventsSince(conversationID, since, limit)
	writeJSON(w, http.StatusOK, AgentEventsResponse{Status: "ok", Events: events, NextSeq: nextSeq})
}

func (s *Server) agentEventsSince(conversationID string, since int64, limit int) ([]AgentEvent, int64) {
	if s == nil {
		return nil, 0
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.events == nil {
		return nil, 0
	}
	rows := s.events[conversationID]
	out := make([]AgentEvent, 0, minInt(limit, len(rows)))
	for _, event := range rows {
		if event.Seq <= since {
			continue
		}
		out = append(out, event)
		if len(out) >= limit {
			break
		}
	}
	return out, s.eventSeq[conversationID]
}

func (s *Server) emitAgentEvent(conversationID string, event AgentEvent) AgentEvent {
	if s == nil {
		return event
	}
	conversationID = firstNonEmpty(strings.TrimSpace(conversationID), strings.TrimSpace(event.ConversationID))
	if conversationID == "" {
		return event
	}
	if event.CreatedAt == "" {
		event.CreatedAt = time.Now().Format(time.RFC3339Nano)
	}
	event.ConversationID = conversationID
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.events == nil {
		s.events = map[string][]AgentEvent{}
	}
	if s.eventSeq == nil {
		s.eventSeq = map[string]int64{}
	}
	s.eventSeq[conversationID]++
	event.Seq = s.eventSeq[conversationID]
	rows := append(s.events[conversationID], event)
	if len(rows) > agentEventBufferLimit {
		rows = rows[len(rows)-agentEventBufferLimit:]
	}
	s.events[conversationID] = rows
	return event
}

func (s *Server) emitTurnEvent(conversationID, eventType string, resp ChatResponse, fallbackGoalID, fallbackRunID string) {
	if s == nil {
		return
	}
	status := strings.TrimSpace(resp.GoalStatus)
	if status == "" && eventType == "turn.started" {
		status = "running"
	}
	s.emitAgentEvent(conversationID, AgentEvent{
		Type:     eventType,
		GoalID:   firstNonEmpty(resp.GoalID, fallbackGoalID),
		RunID:    firstNonEmpty(resp.RunID, fallbackRunID),
		ItemType: "turn",
		Status:   status,
		Title:    turnEventTitle(eventType, status),
		Body:     strings.TrimSpace(resp.Reply),
		Payload: map[string]any{
			"needs_confirmation": resp.NeedsConfirmation,
			"stop_reason":        resp.StopReason,
			"completed_steps":    resp.CompletedSteps,
			"executed_count":     len(resp.ExecutedKernelReply),
			"error":              resp.Error,
		},
	})
}

func (s *Server) emitToolItemStarted(in executorpkg.Input, toolCallID string) {
	conversationID := eventConversationIDFromContext(in.Context)
	if conversationID == "" {
		return
	}
	call := in.ToolCall
	s.emitAgentEvent(conversationID, AgentEvent{
		Type:     "item.started",
		GoalID:   in.GoalID,
		RunID:    in.RunID,
		ItemID:   toolCallID,
		ItemType: "daw_action",
		Status:   "running",
		Title:    toolEventTitle(call),
		Body:     strings.TrimSpace(call.Reason),
		Payload: map[string]any{
			"tool":       strings.TrimSpace(call.Tool),
			"command":    firstNonEmpty(commandNameForEvent(call.Command), commandNameForEvent(call.Args)),
			"confirmed":  in.Confirmed,
			"plan_item":  strings.TrimSpace(call.PlanItemID),
			"source":     firstNonEmpty(strings.TrimSpace(in.Source), "agentloop"),
			"args":       compactEventMap(call.Args),
			"command_raw": compactEventMap(call.Command),
		},
	})
}

func (s *Server) emitToolItemCompleted(in executorpkg.Input, result executorpkg.Result, err error) {
	conversationID := eventConversationIDFromContext(in.Context)
	if conversationID == "" {
		return
	}
	status := normalizeEventStatus(result.Status, err, result.RequiresConfirmation)
	eventType := "item.completed"
	if result.RequiresConfirmation || strings.EqualFold(result.Status, "needs_confirmation") {
		eventType = "approval.requested"
	}
	s.emitAgentEvent(conversationID, AgentEvent{
		Type:     eventType,
		GoalID:   in.GoalID,
		RunID:    in.RunID,
		ItemID:   firstNonEmpty(strings.TrimSpace(result.ToolCallID), strings.TrimSpace(in.ToolCall.ID)),
		ItemType: "daw_action",
		Status:   status,
		Title:    toolResultTitle(in.ToolCall, result),
		Body:     toolResultBody(result, err),
		Payload: map[string]any{
			"tool":                  firstNonEmpty(strings.TrimSpace(result.Tool), strings.TrimSpace(in.ToolCall.Tool)),
			"command_name":          strings.TrimSpace(result.CommandName),
			"agent_action_id":       strings.TrimSpace(result.AgentActionID),
			"confirmed":             in.Confirmed,
			"requires_confirmation": result.RequiresConfirmation,
			"preview":               strings.TrimSpace(result.Preview),
			"undo_label":            strings.TrimSpace(result.UndoLabel),
			"error":                 firstNonEmpty(strings.TrimSpace(result.Error), errorText(err)),
			"result":                compactEventMap(result.Result),
		},
	})
}

func eventConversationIDFromContext(ctx map[string]any) string {
	if ctx == nil {
		return ""
	}
	return strings.TrimSpace(cleanContextText(ctx["conversation_id"]))
}

func turnEventTitle(eventType, status string) string {
	switch eventType {
	case "turn.started":
		return "正在处理"
	case "turn.failed":
		return "执行失败"
	case "turn.completed":
		if strings.EqualFold(status, "waiting_confirmation") {
			return "等待确认"
		}
		return "处理完成"
	default:
		return "任务状态"
	}
}

func toolEventTitle(call planner.ToolCall) string {
	if reason := strings.TrimSpace(call.Reason); reason != "" {
		return "正在执行操作"
	}
	if tool := strings.TrimSpace(call.Tool); tool != "" {
		return "正在执行 " + tool
	}
	return "正在执行工程操作"
}

func toolResultTitle(call planner.ToolCall, result executorpkg.Result) string {
	if result.RequiresConfirmation || strings.EqualFold(result.Status, "needs_confirmation") {
		return "需要确认"
	}
	if isEventErrorStatus(result.Status) || strings.TrimSpace(result.Error) != "" {
		return "执行失败"
	}
	if name := firstNonEmpty(strings.TrimSpace(result.CommandName), strings.TrimSpace(result.Tool), strings.TrimSpace(call.Tool)); name != "" {
		return "已完成 " + name
	}
	return "工程操作已完成"
}

func toolResultBody(result executorpkg.Result, err error) string {
	if text := firstNonEmpty(strings.TrimSpace(result.Error), errorText(err)); text != "" {
		return text
	}
	if result.RequiresConfirmation || strings.EqualFold(result.Status, "needs_confirmation") {
		return firstNonEmpty(strings.TrimSpace(result.Preview), "这个操作需要你确认后才会执行。")
	}
	if result.Result != nil {
		for _, key := range []string{"message", "summary", "track_name", "clip_name", "plugin_name"} {
			if text := strings.TrimSpace(cleanContextText(result.Result[key])); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeEventStatus(status string, err error, needsConfirmation bool) string {
	if needsConfirmation || strings.EqualFold(status, "needs_confirmation") {
		return "waiting_confirmation"
	}
	if err != nil || isEventErrorStatus(status) {
		return "failed"
	}
	if strings.TrimSpace(status) == "" || strings.EqualFold(status, "ok") || strings.EqualFold(status, "success") || strings.EqualFold(status, "completed") {
		return "completed"
	}
	return strings.TrimSpace(status)
}

func isEventErrorStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "error" || status == "failed" || status == "kernel_error"
}

func commandNameForEvent(row map[string]any) string {
	return firstNonEmpty(
		strings.TrimSpace(cleanContextText(row["cmd"])),
		strings.TrimSpace(cleanContextText(row["command"])),
		strings.TrimSpace(cleanContextText(row["command_name"])),
	)
}

func compactEventMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := map[string]any{}
	for key, value := range in {
		if value == nil {
			continue
		}
		text := strings.TrimSpace(cleanContextText(value))
		if text == "" || text == "<nil>" {
			continue
		}
		out[key] = value
		if len(out) >= 24 {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
