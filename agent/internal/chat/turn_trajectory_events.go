package chat

import (
	"strings"

	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// AGENT-F1: plain chat turns accompany their turn.started/turn.completed
// transport events with an observable-trajectory turn node so the webui
// conversation stream can render a live TraceBlock during the wait and a
// single-line receipt afterwards. The trace node identity is stable
// ("turn:{runID}"), so the webui reducer upserts by trace_node_id and repeat
// emissions stay idempotent instead of duplicating nodes.

func chatTurnTrajectoryNodeID(turnID string) string {
	return "turn:" + strings.TrimSpace(turnID)
}

func chatTurnTrajectoryTurnID(goalID, runID string) string {
	return strings.TrimSpace(firstNonEmpty(runID, goalID))
}

// emitChatTurnTrajectory routes a turn transport event to its accompanied
// trajectory projection. Unknown event types emit nothing.
func (s *Server) emitChatTurnTrajectory(conversationID, eventType string, resp ChatResponse, goalID, runID string) {
	if s == nil {
		return
	}
	switch eventType {
	case "turn.started":
		s.emitChatTurnTrajectoryStarted(conversationID, goalID, runID)
	case "turn.completed", "turn.failed", "turn.stopped":
		s.emitChatTurnTrajectoryTerminal(conversationID, goalID, runID, chatTurnTrajectoryStatus(eventType, resp), chatTurnTrajectorySummary(resp))
	}
}

func (s *Server) emitChatTurnTrajectoryStarted(conversationID, goalID, runID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	turnID := chatTurnTrajectoryTurnID(goalID, runID)
	if turnID == "" {
		return
	}
	// 防重复：自由态实验路径自带 turn:{loopID} 键的完整轨迹投影，会话内
	// 活跃循环期间伴发 runID 键的 turn 节点会让对话流出现双轨迹块。
	if s.hasActiveFreeStateReasoningLoop(conversationID) {
		return
	}
	if s.hasChatTurnTrajectoryNodeEvent(conversationID, trajectory.EventTurnStarted, turnID) {
		return
	}
	// Title stays empty and Body carries the live status line: the webui
	// activity factory renders the event body as the thinking-line content
	// while the turn is live.
	event := trajectory.Event{
		Type: trajectory.EventTurnStarted, ConversationID: conversationID,
		GoalID: goalID, RunID: runID, ItemID: chatTurnTrajectoryNodeID(turnID),
		Body: "正在处理",
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TurnID:        turnID, TraceNodeID: chatTurnTrajectoryNodeID(turnID),
			NodeKind: trajectory.NodeTurn, Phase: "framing", Status: trajectory.StatusRunning,
		},
	}
	if _, err := s.emitTrajectoryEvent(conversationID, event); err != nil && s.logger != nil {
		s.logger.Warn("[chat.turn] trajectory started event rejected: %v", err)
	}
}

func (s *Server) emitChatTurnTrajectoryTerminal(conversationID, goalID, runID string, status trajectory.Status, summary string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	turnID := chatTurnTrajectoryTurnID(goalID, runID)
	if turnID == "" {
		return
	}
	// Only close a node this path opened: free-state-suppressed turns never
	// emitted a started node, and a stale live block must not be resurrected.
	if !s.hasChatTurnTrajectoryNodeEvent(conversationID, trajectory.EventTurnStarted, turnID) {
		return
	}
	for _, terminal := range []trajectory.EventType{trajectory.EventTurnCompleted, trajectory.EventTurnFailed, trajectory.EventTurnStopped} {
		if s.hasChatTurnTrajectoryNodeEvent(conversationID, terminal, turnID) {
			return
		}
	}
	eventType := trajectory.EventTurnCompleted
	phase := "completed"
	switch status {
	case trajectory.StatusFailed:
		eventType = trajectory.EventTurnFailed
		phase = "failed"
	case trajectory.StatusStopped:
		eventType = trajectory.EventTurnStopped
		phase = "stopped"
	}
	event := trajectory.Event{
		Type: eventType, ConversationID: conversationID,
		GoalID: goalID, RunID: runID, ItemID: chatTurnTrajectoryNodeID(turnID),
		Title: "Turn " + phase,
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TurnID:        turnID, TraceNodeID: chatTurnTrajectoryNodeID(turnID),
			NodeKind: trajectory.NodeTurn, Phase: phase, Status: status,
			Summary: strings.TrimSpace(summary),
		},
	}
	if _, err := s.emitTrajectoryEvent(conversationID, event); err != nil && s.logger != nil {
		s.logger.Warn("[chat.turn] trajectory terminal event rejected: %v", err)
	}
}

// chatTurnTrajectoryStatus maps the turn transport outcome onto the
// trajectory status vocabulary. A response that leaves the goal running in
// the background still closes its message-scope node as completed: any
// continuing work is projected by its own trajectory (free-state experiment),
// and an open running node here would leave the TraceBlock live forever.
func chatTurnTrajectoryStatus(eventType string, resp ChatResponse) trajectory.Status {
	switch {
	case eventType == "turn.failed" || strings.TrimSpace(resp.Error) != "" || strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusFailed)):
		return trajectory.StatusFailed
	case eventType == "turn.stopped" || strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusStopped)) || strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusCancelled)):
		return trajectory.StatusStopped
	case resp.NeedsConfirmation || strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusWaitingConfirmation)) ||
		strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusWaitingClarification)) || strings.EqualFold(resp.GoalStatus, string(agentruntime.StatusWaitingContinue)):
		return trajectory.StatusWaiting
	default:
		return trajectory.StatusCompleted
	}
}

func chatTurnTrajectorySummary(resp ChatResponse) string {
	return firstNonEmpty(strings.TrimSpace(resp.Error), strings.TrimSpace(resp.StopReason))
}

func (s *Server) hasChatTurnTrajectoryNodeEvent(conversationID string, eventType trajectory.EventType, turnID string) bool {
	nodeID := chatTurnTrajectoryNodeID(turnID)
	for _, event := range s.agentEventsSinceForControl(conversationID) {
		if event.Type == string(eventType) && event.ItemID == nodeID {
			return true
		}
	}
	return false
}
