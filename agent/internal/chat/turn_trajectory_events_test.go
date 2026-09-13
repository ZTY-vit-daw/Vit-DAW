package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
)

func chatTurnTrajectoryEvents(t *testing.T, s *Server, conversationID string) []AgentEvent {
	t.Helper()
	events, _ := s.agentEventsSince(conversationID, 0, 200)
	return events
}

func newChatTurnTrajectoryServer(t *testing.T) *Server {
	t.Helper()
	return New(nil, shadow.New(nil), nil)
}

func countChatTurnTrajectory(events []AgentEvent, eventType trajectory.EventType, itemID string) int {
	count := 0
	for _, event := range events {
		if event.Type == string(eventType) && event.ItemID == itemID {
			count++
		}
	}
	return count
}

func TestEmitTurnEventAccompaniesChatTurnTrajectoryLifecycle(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	s.emitTurnEvent("conv-chat", "turn.started", ChatResponse{GoalStatus: "running"}, "goal-1", "run-1")
	events := chatTurnTrajectoryEvents(t, s, "conv-chat")
	if got := countChatTurnTrajectory(events, trajectory.EventTurnStarted, "turn:run-1"); got != 1 {
		t.Fatalf("started events=%d want 1 (%+v)", got, events)
	}
	var started AgentEvent
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnStarted) {
			started = event
		}
	}
	if started.ItemType != "trajectory" || started.TurnID != "run-1" {
		t.Fatalf("started anchoring identity: %+v", started)
	}
	if started.Body != "正在处理" {
		t.Fatalf("started body feeds the webui thinking line, got %q", started.Body)
	}
	payload := started.Payload
	if payload["schema_version"] != trajectory.SchemaVersion || payload["turn_id"] != "run-1" ||
		payload["trace_node_id"] != "turn:run-1" || payload["node_kind"] != string(trajectory.NodeTurn) ||
		payload["status"] != string(trajectory.StatusRunning) || payload["phase"] != "framing" {
		t.Fatalf("started payload=%+v", payload)
	}

	s.emitTurnEvent("conv-chat", "turn.completed", ChatResponse{Reply: "done"}, "goal-1", "run-1")
	events = chatTurnTrajectoryEvents(t, s, "conv-chat")
	if got := countChatTurnTrajectory(events, trajectory.EventTurnCompleted, "turn:run-1"); got != 1 {
		t.Fatalf("terminal events=%d want 1 (%+v)", got, events)
	}
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnCompleted) {
			if event.Payload["status"] != string(trajectory.StatusCompleted) || event.Payload["turn_id"] != "run-1" {
				t.Fatalf("terminal payload=%+v", event.Payload)
			}
		}
	}
}

func TestChatTurnTrajectoryTerminalRequiresStartedAndStaysSingleShot(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	// A terminal without an accompanied started node (free-state suppressed
	// turn, or a foreign run) must not fabricate a trajectory node.
	s.emitTurnEvent("conv-dedup", "turn.completed", ChatResponse{}, "goal-2", "run-2")
	if events := chatTurnTrajectoryEvents(t, s, "conv-dedup"); len(events) != 1 {
		t.Fatalf("expected only the plain turn event, got %+v", events)
	}
	s.emitTurnEvent("conv-dedup", "turn.started", ChatResponse{}, "goal-2", "run-2")
	s.emitTurnEvent("conv-dedup", "turn.failed", ChatResponse{Error: "boom"}, "goal-2", "run-2")
	// A second terminal for the same run (late finalizeInteractionChatResponse
	// replay) must not add another terminal node.
	s.emitTurnEvent("conv-dedup", "turn.completed", ChatResponse{}, "goal-2", "run-2")
	events := chatTurnTrajectoryEvents(t, s, "conv-dedup")
	if got := countChatTurnTrajectory(events, trajectory.EventTurnFailed, "turn:run-2"); got != 1 {
		t.Fatalf("failed terminals=%d want 1", got)
	}
	if got := countChatTurnTrajectory(events, trajectory.EventTurnCompleted, "turn:run-2"); got != 0 {
		t.Fatalf("completed terminals=%d want 0 (already failed)", got)
	}
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnFailed) {
			if event.Payload["status"] != string(trajectory.StatusFailed) {
				t.Fatalf("failed payload=%+v", event.Payload)
			}
		}
	}
}

func TestChatTurnTrajectorySkippedWhileFreeStateLoopActive(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "conv-free"}, "bounded", gapTestAdmission(experiment.AuthorityOrdinary), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "conv-free", LoopID: "loop-1", OriginalIntent: "bounded", Status: "reasoning", Experiment: &turn})
	s.emitTurnEvent("conv-free", "turn.started", ChatResponse{}, "goal-3", "run-3")
	s.emitTurnEvent("conv-free", "turn.completed", ChatResponse{}, "goal-3", "run-3")
	for _, event := range chatTurnTrajectoryEvents(t, s, "conv-free") {
		if event.ItemType == "trajectory" {
			t.Fatalf("active free-state loop must not get accompanied trajectory events: %+v", event)
		}
	}
}

func TestChatTurnTrajectoryTerminalClosesAfterLoopBecomesActive(t *testing.T) {
	// The admission-triggering message emits its started node before the loop
	// exists; the loop then activates during processing. The terminal must
	// still close that node so the TraceBlock never stays live forever.
	s := newChatTurnTrajectoryServer(t)
	s.emitTurnEvent("conv-late", "turn.started", ChatResponse{}, "goal-4", "run-4")
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: "conv-late"}, "bounded", gapTestAdmission(experiment.AuthorityOrdinary), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "conv-late", LoopID: "loop-2", OriginalIntent: "bounded", Status: "reasoning", Experiment: &turn})
	s.emitTurnEvent("conv-late", "turn.completed", ChatResponse{}, "goal-4", "run-4")
	events := chatTurnTrajectoryEvents(t, s, "conv-late")
	if got := countChatTurnTrajectory(events, trajectory.EventTurnCompleted, "turn:run-4"); got != 1 {
		t.Fatalf("terminal events=%d want 1 (%+v)", got, events)
	}
}

func TestChatTurnTrajectoryStatusMapping(t *testing.T) {
	cases := []struct {
		name      string
		eventType string
		resp      ChatResponse
		want      trajectory.Status
	}{
		{"failed event", "turn.failed", ChatResponse{}, trajectory.StatusFailed},
		{"error text", "turn.completed", ChatResponse{Error: "boom"}, trajectory.StatusFailed},
		{"failed status", "turn.completed", ChatResponse{GoalStatus: "failed"}, trajectory.StatusFailed},
		{"stopped event", "turn.stopped", ChatResponse{}, trajectory.StatusStopped},
		{"cancelled status", "turn.completed", ChatResponse{GoalStatus: "cancelled"}, trajectory.StatusStopped},
		{"needs confirmation", "turn.completed", ChatResponse{NeedsConfirmation: true}, trajectory.StatusWaiting},
		{"waiting clarification", "turn.completed", ChatResponse{GoalStatus: "waiting_clarification"}, trajectory.StatusWaiting},
		{"waiting continue", "turn.completed", ChatResponse{GoalStatus: "waiting_continue"}, trajectory.StatusWaiting},
		{"plain completion", "turn.completed", ChatResponse{GoalStatus: "completed"}, trajectory.StatusCompleted},
		{"background continuation closes the message-scope node", "turn.completed", ChatResponse{GoalStatus: "running"}, trajectory.StatusCompleted},
	}
	for _, testCase := range cases {
		if got := chatTurnTrajectoryStatus(testCase.eventType, testCase.resp); got != testCase.want {
			t.Fatalf("%s: got %q want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestChatTurnTrajectoryWaitingTurnUsesCompletedTypeWithWaitingStatus(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	s.emitTurnEvent("conv-wait", "turn.started", ChatResponse{}, "goal-5", "run-5")
	s.emitTurnEvent("conv-wait", "turn.completed", ChatResponse{GoalStatus: "waiting_confirmation", NeedsConfirmation: true}, "goal-5", "run-5")
	events := chatTurnTrajectoryEvents(t, s, "conv-wait")
	for _, event := range events {
		if event.Type == string(trajectory.EventTurnCompleted) {
			if event.Payload["status"] != string(trajectory.StatusWaiting) {
				t.Fatalf("waiting terminal payload=%+v", event.Payload)
			}
		}
	}
}

// --- TRAJ-DUAL-1（2026-09-13）：双轨迹回合壳的终局闭合 ---------------------
//
// 取证（artifacts/b12_1_blind/forensics/events.json，run_5ef8d61bc9b78e20）：
// 会话首条消息建自由态循环时，消息域壳 turn:{runID} 先在 seq2 开启（此刻循环
// 还不存在，闸门放行），实验回合壳 turn:{loopID} 在 seq10 由实验运行期开启。
// seq19 的 transport 终局只闭前者；实验壳在整条 trace 内没有任何 turn 家族
// 终局——experiment.Turn.Settle 只发 trajectory.settled（node_kind=settlement）。
//
// 消费面契约（webui trajectory.ts B9 统一面）：轮次域回合的终态**只由**
// trajectory.turn.completed/failed/stopped 收口；settlement 节点不能收口。
// 闸门生效的那一半分支（循环已存在 ⇒ 消息域壳被抑制）下，于是会话回合根本
// 拿不到任何 turn 家族终局 ⇒ 轨迹块永不闭合。
//
// 壳身份一律取 payload.turn_id（消息域=turn:{runID}，自由态实验=turn:{loopID}）：
// 实验壳的 trace_node_id 是运行期随机铸造的（experiment/runtime.go:1012
// newID("trace")），按 item_id 找不到它。

// chatTurnTrajectoryShellID 返回轨迹 turn 壳的身份；非 turn 壳返回空串。
func chatTurnTrajectoryShellID(event AgentEvent) string {
	if event.ItemType != "trajectory" || !strings.HasPrefix(event.Type, "trajectory.") {
		return ""
	}
	if firstStringFromMap(event.Payload, "node_kind") != string(trajectory.NodeTurn) {
		return ""
	}
	return firstStringFromMap(event.Payload, "turn_id")
}

func countChatTurnTrajectoryShellTerminal(events []AgentEvent, turnID string) int {
	count := 0
	for _, event := range events {
		if chatTurnTrajectoryShellID(event) != turnID {
			continue
		}
		switch event.Type {
		case string(trajectory.EventTurnCompleted), string(trajectory.EventTurnFailed), string(trajectory.EventTurnStopped):
			count++
		}
	}
	return count
}

// openChatTurnTrajectoryShells 返回「已开启且未收口」的轨迹 turn 壳——悬置块
// 的判据：回合结束后这里必须为空。
func openChatTurnTrajectoryShells(events []AgentEvent) []string {
	started, closed := map[string]bool{}, map[string]bool{}
	order := []string{}
	for _, event := range events {
		id := chatTurnTrajectoryShellID(event)
		if id == "" {
			continue
		}
		switch event.Type {
		case string(trajectory.EventTurnStarted):
			if !started[id] {
				started[id] = true
				order = append(order, id)
			}
		case string(trajectory.EventTurnCompleted), string(trajectory.EventTurnFailed), string(trajectory.EventTurnStopped):
			closed[id] = true
		}
	}
	out := []string{}
	for _, id := range order {
		if !closed[id] {
			out = append(out, id)
		}
	}
	return out
}

// freeStateShellServerForDualTest 造出「循环已存在 + 实验回合已准入并已发射
// 阶段节点」的服务器，返回实验壳身份 turn:{loopID}。
func freeStateShellServerForDualTest(t *testing.T, conversationID, loopID, goalID, runID, originalIntent string) (*Server, string) {
	t.Helper()
	s := newChatTurnTrajectoryServer(t)
	turn, err := experiment.NewTurn(
		experiment.Identity{ConversationID: conversationID, GoalID: goalID, RunID: runID, TurnID: "turn:" + loopID},
		originalIntent, gapTestAdmission(experiment.AuthorityOrdinary), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, ConversationID: conversationID, LoopID: loopID,
		GoalID: goalID, RunID: runID, OriginalIntent: originalIntent, Status: "reasoning", Experiment: &turn,
	}
	s.storeFreeStateLoop(loop)
	s.emitFreeStateExperimentEvents(turn.StartEvents(time.Now().UTC()))
	return s, "turn:" + loopID
}

// TestChatTurnTrajectoryClosesFreeStateExperimentShellAtTurnTerminal 钉住闸门
// 生效分支（循环已存在 ⇒ 消息域壳被抑制）：transport 终局必须收口自由态实验
// 回合壳，否则该会话回合在轨迹流里没有终局——块永久悬置。
func TestChatTurnTrajectoryClosesFreeStateExperimentShellAtTurnTerminal(t *testing.T) {
	s, shellID := freeStateShellServerForDualTest(t, "conv-fs-shell", "free_state_dual_live", "goal-6", "run-6", "bounded")
	// 闸门生效：循环已存在，消息域壳不得发射（既有行为，零回退）。
	s.emitTurnEvent("conv-fs-shell", "turn.started", ChatResponse{}, "goal-6", "run-6")
	if events := chatTurnTrajectoryEvents(t, s, "conv-fs-shell"); countChatTurnTrajectory(events, trajectory.EventTurnStarted, "turn:run-6") != 0 {
		t.Fatalf("active free-state loop must suppress the message-scope shell: %+v", events)
	}
	// 回合仍在跑：实验壳必须保持开启（不得提前收口）。
	if open := openChatTurnTrajectoryShells(chatTurnTrajectoryEvents(t, s, "conv-fs-shell")); len(open) != 1 || open[0] != shellID {
		t.Fatalf("live turn must keep exactly the experiment shell open, got %v", open)
	}

	s.emitTurnEvent("conv-fs-shell", "turn.completed", ChatResponse{GoalStatus: "completed"}, "goal-6", "run-6")
	events := chatTurnTrajectoryEvents(t, s, "conv-fs-shell")
	if got := countChatTurnTrajectoryShellTerminal(events, shellID); got != 1 {
		t.Fatalf("free-state experiment shell terminals=%d want 1 (events=%+v)", got, events)
	}
	if open := openChatTurnTrajectoryShells(events); len(open) != 0 {
		t.Fatalf("turn terminal must leave no suspended trajectory shell, got %v", open)
	}
	// 单发：重复终局不得叠加第二个闭块事件。
	s.emitTurnEvent("conv-fs-shell", "turn.completed", ChatResponse{GoalStatus: "completed"}, "goal-6", "run-6")
	if got := countChatTurnTrajectoryShellTerminal(chatTurnTrajectoryEvents(t, s, "conv-fs-shell"), shellID); got != 1 {
		t.Fatalf("replayed terminal must stay single-shot, got %d", got)
	}
}

// TestChatTurnTrajectoryFirstMessageLoopLeavesNoSuspendedShell 钉住铁证形态：
// 首条消息在循环存在之前就开了消息域壳，循环随后接管并开实验壳。transport
// 终局必须把**两个**壳都收口——否则实验壳永久悬置（98.7s 空窗后出现的那一块）。
func TestChatTurnTrajectoryFirstMessageLoopLeavesNoSuspendedShell(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	// seq2：循环尚不存在 ⇒ 闸门放行，消息域壳开启。
	s.emitTurnEvent("conv-first-msg", "turn.started", ChatResponse{}, "goal-7", "run-7")
	events := chatTurnTrajectoryEvents(t, s, "conv-first-msg")
	if countChatTurnTrajectory(events, trajectory.EventTurnStarted, "turn:run-7") != 1 {
		t.Fatalf("first message must open the message-scope shell: %+v", events)
	}
	// 循环在 handleChat 之后创建；实验回合随后准入并发射阶段节点。
	turn, err := experiment.NewTurn(
		experiment.Identity{ConversationID: "conv-first-msg", GoalID: "goal-7", RunID: "run-7", TurnID: "turn:free_state_first_msg"},
		"bounded", gapTestAdmission(experiment.AuthorityOrdinary), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, ConversationID: "conv-first-msg", LoopID: "free_state_first_msg",
		GoalID: "goal-7", RunID: "run-7", OriginalIntent: "bounded", Status: "reasoning", Experiment: &turn,
	})
	s.emitFreeStateExperimentEvents(turn.StartEvents(time.Now().UTC()))

	// 双壳同世形态（取证 seq2 + seq10）：两个壳都在世，这是既成事实。
	events = chatTurnTrajectoryEvents(t, s, "conv-first-msg")
	if open := openChatTurnTrajectoryShells(events); len(open) != 2 {
		t.Fatalf("dual-shell window shape changed, got %v", open)
	}

	s.emitTurnEvent("conv-first-msg", "turn.completed", ChatResponse{GoalStatus: "completed"}, "goal-7", "run-7")
	events = chatTurnTrajectoryEvents(t, s, "conv-first-msg")
	// 壳身份取 payload.turn_id：消息域壳用裸 runID，实验壳用 turn:{loopID}
	// ——两个壳本就活在两个 turn_id 命名空间里（这正是闸门看不见对方的原因）。
	for _, shell := range []string{"run-7", "turn:free_state_first_msg"} {
		if got := countChatTurnTrajectoryShellTerminal(events, shell); got != 1 {
			t.Fatalf("shell %s terminals=%d want 1 (events=%+v)", shell, got, events)
		}
	}
	if open := openChatTurnTrajectoryShells(events); len(open) != 0 {
		t.Fatalf("turn terminal must close both shells, suspended: %v", open)
	}
}

// TestChatTurnTrajectoryPlainTurnKeepsExperimentShellUntouched 防回退：没有
// 自由态实验壳的普通回合逐字不变（终局只闭自己开的消息域壳）。
func TestChatTurnTrajectoryPlainTurnKeepsExperimentShellUntouched(t *testing.T) {
	s := newChatTurnTrajectoryServer(t)
	s.emitTurnEvent("conv-plain", "turn.started", ChatResponse{}, "goal-8", "run-8")
	s.emitTurnEvent("conv-plain", "turn.completed", ChatResponse{GoalStatus: "completed"}, "goal-8", "run-8")
	events := chatTurnTrajectoryEvents(t, s, "conv-plain")
	if got := countChatTurnTrajectoryShellTerminal(events, "run-8"); got != 1 {
		t.Fatalf("plain turn terminal=%d want 1", got)
	}
	for _, event := range events {
		shellID := chatTurnTrajectoryShellID(event)
		if shellID == "" {
			continue
		}
		if shellID != "run-8" {
			t.Fatalf("plain turn must project exactly its own message-scope shell, got %q: %+v", shellID, event)
		}
	}
}
