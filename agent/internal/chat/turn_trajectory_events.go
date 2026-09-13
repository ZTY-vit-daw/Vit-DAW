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
		// AGENT-F6：切片边界不是回合终局。goal=waiting_continue 且仍有活
		// 续跑链（pending/claimed/running）时，turn:{runID} 节点必须保持
		// running——UI 常转 spinner、回执不提前收成"等待你的判断"（2026-09-04
		// 手测铁证：trajectory.turn.completed 带 waiting_for_user +
		// limit_reached，而背景链还在跑）。链终局由调度侧终片的 turn 事件闭块。
		if eventType == "turn.completed" && strings.EqualFold(strings.TrimSpace(resp.GoalStatus), string(agentruntime.StatusWaitingContinue)) {
			if s.goalHasLiveContinuationOwner(goalID) {
				return
			}
			// 无活链时传输层的 waiting_continue 未必是真相（预算耗尽路径在
			// recordGoalResult 内直接把 goal 落成 completed）——以运行时
			// goal 状态为准再映射。
			if s.harness != nil {
				if goal := s.harness.RuntimeStatus(goalID); goal.GoalID != "" {
					adjusted := resp
					adjusted.GoalStatus = string(goal.Status)
					resp = adjusted
				}
			}
		}
		s.emitChatTurnTrajectoryTerminal(conversationID, goalID, runID, chatTurnTrajectoryStatus(eventType, resp), chatTurnTrajectorySummary(resp))
	}
}

// goalHasLiveContinuationOwner reports whether the goal still owns a next
// automatic slice (pending/claimed/running). A waiting_interaction park is
// deliberately excluded: that is a genuine user-facing wait, not ongoing
// background execution.
func (s *Server) goalHasLiveContinuationOwner(goalID string) bool {
	if s == nil || strings.TrimSpace(goalID) == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, armed := s.goalContinuations[goalID]; armed {
		return true
	}
	for _, item := range s.durableContinuations {
		if item.GoalID != goalID {
			continue
		}
		switch item.Status {
		case ContinuationPending, ContinuationClaimed, ContinuationRunning:
			return true
		}
	}
	return false
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
	// TRAJ-DUAL-1：自由态实验回合壳与本壳同点、同闸门收口（见下方注释）。
	s.closeFreeStateExperimentTrajectoryShells(conversationID, turnID, goalID, runID, status, summary)
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
	s.emitChatTurnTrajectoryNodeTerminal(conversationID, turnID, goalID, runID, chatTurnTrajectoryNodeID(turnID), status, summary)
}

// chatTurnTrajectoryFreeStateTurnPrefix is the free-state experiment turn
// namespace. experiment.NewTurn is called with TurnID "turn:"+loop.LoopID
// (free_state_experiment_runtime.go) and every loop id is minted as
// "free_state_"+randomID (free_state_reasoning_loop.go), so the experiment turn
// shell always lands under this prefix.
const chatTurnTrajectoryFreeStateTurnPrefix = "turn:free_state_"

// TRAJ-DUAL-1（2026-09-13）：自由态实验回合壳的终局闭合。
//
// 取证（artifacts/b12_1_blind/forensics/events.json，run_5ef8d61bc9b78e20）：
// 实验回合壳由 experiment.Turn.StartEvents 在准入时开启（trajectory.turn.started，
// node_kind=turn，payload.turn_id=turn:{loopID}），但它的终局路线只有 Stop() 的
// trajectory.turn.stopped——正常收口路径 Settle() 只发 trajectory.settled
// （node_kind=settlement），而消费面（webui trajectory.ts B9 统一面）的轮次终态
// **只由 trajectory.turn.completed/failed/stopped 收口**。于是闸门生效的那一半
// 分支（循环已存在 ⇒ 消息域壳被 :94 抑制）下，整个会话回合在轨迹流里拿不到任何
// turn 家族终局——轨迹块永不闭合（前端悬置 spinner 的服务端根）。
//
// 收口点刻意选在消息域壳的同一处、同一闸门之后：transport turn 终局本身已受
// AGENT-F6 的「waiting_continue + 活续跑链」抑制约束，因此实验壳只在整轮真正
// 收尾（含调度链终片）时闭合，不新增任何时序面。
func (s *Server) closeFreeStateExperimentTrajectoryShells(conversationID, currentTurnID, goalID, runID string, status trajectory.Status, summary string) {
	for _, shellTurnID := range s.openChatFreeStateExperimentTurnIDs(conversationID, currentTurnID) {
		s.emitChatTurnTrajectoryNodeTerminal(conversationID, shellTurnID, goalID, runID, shellTurnID, status, summary)
	}
}

// openChatFreeStateExperimentTurnIDs returns the payload turn ids of free-state
// experiment turn shells that this conversation has opened and not yet closed.
//
// 两路取证并集，因为实验壳必须能被找到两次：
//  1. 会话持久循环当前的实验回合（生产真相；循环被清退后这条会落空）；
//  2. 事件流里已开启的 turn:free_state_* 壳（循环重启/清退后仍能收口）。
//
// 壳的身份一律取 payload.turn_id：实验壳的 trace_node_id / item_id 是
// experiment/runtime.go 的运行期随机串（newID("trace")），按 item_id 永远找不到
// 它——这正是消息域壳的 hasChatTurnTrajectoryNodeEvent 守卫看不见实验壳的原因
// （两个壳活在两个 turn_id 命名空间：裸 runID 与 turn:{loopID}）。
func (s *Server) openChatFreeStateExperimentTurnIDs(conversationID, currentTurnID string) []string {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	events := s.agentEventsSinceForControl(conversationID)
	candidates := []string{}
	if loop, ok := s.freeStateLoop(conversationID); ok && loop.Experiment != nil {
		candidates = append(candidates, chatTurnTrajectoryNodeID(loop.LoopID))
	}
	for _, event := range events {
		if event.Type != string(trajectory.EventTurnStarted) {
			continue
		}
		if strings.TrimSpace(firstStringFromMap(event.Payload, "node_kind")) != string(trajectory.NodeTurn) {
			continue
		}
		if id := strings.TrimSpace(firstStringFromMap(event.Payload, "turn_id")); strings.HasPrefix(id, chatTurnTrajectoryFreeStateTurnPrefix) {
			candidates = append(candidates, id)
		}
	}
	currentTurnID = strings.TrimSpace(currentTurnID)
	out := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		turnID := strings.TrimSpace(candidate)
		if turnID == "" || turnID == currentTurnID || seen[turnID] {
			continue
		}
		seen[turnID] = true
		if !hasChatTrajectoryPayloadNode(events, trajectory.EventTurnStarted, turnID) {
			continue
		}
		closed := false
		for _, terminal := range []trajectory.EventType{trajectory.EventTurnCompleted, trajectory.EventTurnFailed, trajectory.EventTurnStopped} {
			if hasChatTrajectoryPayloadNode(events, terminal, turnID) {
				closed = true
				break
			}
		}
		if closed {
			continue
		}
		out = append(out, turnID)
	}
	return out
}

// hasChatTrajectoryPayloadNode matches a projected trajectory event by its
// payload turn id (the shell identity), independent of the node id.
func hasChatTrajectoryPayloadNode(events []AgentEvent, eventType trajectory.EventType, turnID string) bool {
	for _, event := range events {
		if event.Type != string(eventType) {
			continue
		}
		if strings.TrimSpace(firstStringFromMap(event.Payload, "turn_id")) == strings.TrimSpace(turnID) {
			return true
		}
	}
	return false
}

// emitChatTurnTrajectoryNodeTerminal publishes one turn-family terminal for a
// trajectory turn node. Callers own the "is this node open?" decision, so the
// free-state shell (opened by another domain, matched by payload turn id) and
// the message-scope shell (opened here, matched by item id) can both use it.
func (s *Server) emitChatTurnTrajectoryNodeTerminal(conversationID, turnID, goalID, runID, nodeID string, status trajectory.Status, summary string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
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
		GoalID: goalID, RunID: runID, ItemID: nodeID,
		Title: chatTurnPhaseTitle(phase),
		Payload: trajectory.Payload{
			SchemaVersion: trajectory.SchemaVersion,
			TurnID:        turnID, TraceNodeID: nodeID,
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

// emitSchedulerChainResultEvent delivers the final slice's outcome of a
// scheduler-driven continuation chain (AGENT-F6, 2026-09-04 手测：多轮执行后
// 没有给出任何结果——executeDurableContinuation 只查 error/交互挂起，最终
// reply 被静默丢弃，turn.completed 又只在 HTTP 路径发射)。The transport event
// carries the final reply with a scheduler_chain marker so the webui can render
// it as the assistant result message; the accompanied trajectory terminal closes
// the turn:{runID} node that the slice-boundary path deliberately left running.
func (s *Server) emitSchedulerChainResultEvent(conversationID string, resp ChatResponse, failed error) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	if failed != nil {
		resp.Error = firstNonEmpty(resp.Error, failed.Error())
		if strings.TrimSpace(resp.GoalStatus) == "" {
			resp.GoalStatus = string(agentruntime.StatusFailed)
		}
	}
	goalID := strings.TrimSpace(resp.GoalID)
	runID := strings.TrimSpace(resp.RunID)
	if goalID == "" && runID == "" {
		return
	}
	eventType := "turn.completed"
	if strings.TrimSpace(resp.Error) != "" || strings.EqualFold(strings.TrimSpace(resp.GoalStatus), string(agentruntime.StatusFailed)) {
		eventType = "turn.failed"
	} else if strings.EqualFold(strings.TrimSpace(resp.GoalStatus), string(agentruntime.StatusStopped)) ||
		strings.EqualFold(strings.TrimSpace(resp.GoalStatus), string(agentruntime.StatusCancelled)) {
		eventType = "turn.stopped"
	}
	status := strings.TrimSpace(resp.GoalStatus)
	eventBody := strings.TrimSpace(firstNonEmpty(resp.Reply, resp.Error))
	if eventType == "turn.stopped" && eventBody == "" {
		// CONTRACT-1 turn.stopped 终局补齐：stopped 终局同样要带可用终局
		// body（CONTRACT-2 已知缺口：UI 从未提取 stopped——服务端先保证
		// 事件形态完整，body 空会让终局气泡无法合成）。
		eventBody = firstNonEmpty(strings.TrimSpace(resp.StopReason), "回合已停止。")
	}
	s.emitAgentEvent(conversationID, AgentEvent{
		Type: eventType, GoalID: goalID, RunID: runID,
		ItemID: "chain_result", ItemType: "turn", Status: status,
		Title: turnEventTitle(eventType, status),
		Body:  eventBody,
		Payload: map[string]any{
			"scheduler_chain": true,
			// CONTRACT-1 settle_slice 标记（M12 定案）：调度链收尾切片的
			// 终局显式自报种类，UI 按标记隐藏"0 步"块，不再猜测。
			"turn_kind": "settle_slice",
			// B1-F2：终局自报 goal 状态——waiting park 的补投递靠它把
			// waiting_confirmation/waiting_clarification 与完成态区分开
			//（状态同时也在事件 Status 位上）。
			"goal_status":        status,
			"stop_reason":        resp.StopReason,
			"completed_steps":    resp.CompletedSteps,
			"executed_count":     len(resp.ExecutedKernelReply),
			"error":              resp.Error,
			"needs_confirmation": resp.NeedsConfirmation,
		},
		LogicalMessageID: "agent_turn:" + firstNonEmpty(runID, goalID, conversationID),
	})
	s.emitChatTurnTrajectory(conversationID, eventType, resp, goalID, runID)
}

// AGENT-W2: 轨迹终态事件标题中文化（原 "Turn completed/failed/stopped"）。
func chatTurnPhaseTitle(phase string) string {
	switch phase {
	case "completed":
		return "回合已完成"
	case "failed":
		return "回合失败"
	case "stopped":
		return "回合已停止"
	default:
		return "回合状态"
	}
}
