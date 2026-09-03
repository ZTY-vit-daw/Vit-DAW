package chat

import (
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

