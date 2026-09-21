package chat

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// schedulerTerminalServerForTest builds a kernel-free server whose harness
// hydrates conversation history from a temp-dir project workspace. The seed
// checkpoint and the shadow revision intentionally match so every vit node
// append takes the checkpoint gate's skip path (bind to HEAD, zero kernel).
func schedulerTerminalServerForTest(t *testing.T) (*Server, string) {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "terminal.vit")
	const projectUUID = "vitproj_f2_terminal_test"
	history.BindProjectIdentity(projectPath, projectUUID)
	if _, err := history.EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Checkpoint(map[string]any{
		"project_path":         projectPath,
		"message":              "seed",
		"source":               "test",
		"project_snapshot_xml": "<project/>",
		"project_revision":     "rev-7",
	}); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"project_path":     projectPath,
		"project_uuid":     projectUUID,
		"project_revision": "rev-7",
		"tracks":           []any{},
	})
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, shadowProject, nil)
	return s, projectPath
}

func schedulerTerminalHydratedMessages(t *testing.T, s *Server, projectPath string) []history.ConversationMessage {
	t.Helper()
	summary := s.harness.ProjectHistorySummaryForProject(context.Background(), "goal-f2", projectPath)
	messages, ok := summary["conversation_messages"].([]history.ConversationMessage)
	if !ok {
		t.Fatalf("hydration did not return conversation messages: %+v", summary)
	}
	return messages
}

func schedulerTerminalHydrationContains(t *testing.T, s *Server, projectPath, needle string) bool {
	t.Helper()
	for _, message := range schedulerTerminalHydratedMessages(t, s, projectPath) {
		if strings.Contains(message.Content, needle) {
			return true
		}
	}
	return false
}

const schedulerTerminalReply = "实验调整已应用并完成观测评估，等待用户试听确认。"

// F2 pin (1): a chain parked at the human-judgment boundary
// (waiting_interaction record, goal waiting_confirmation) must deliver its
// terminal reply through the scheduler_chain transport channel. B1-DIAG Q1:
// the delivery gate's terminal-only vocabulary silently dropped the waiting
// park, so "等待试听确认" never reached any user-facing surface. The reply body
// falls back to the loop's latest decision summary when the park slice's own
// reply is empty (the repro shape), and the payload must annotate the waiting
// status so the terminal is distinguishable from a completed settle.
func TestWaitingParkChainEndDeliversSchedulerChainTerminal(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "improve the mix")
	s.harness.SetGoalStatus("goal-f2", agentruntime.StatusWaitingConfirmation, nil)
	now := time.Now().UTC()
	s.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free_state_f2_park", ConversationID: "conversation-f2",
		GoalID: "goal-f2", RunID: "run-f2", Status: "blocked", OriginalIntent: "improve the mix",
		LatestDecision: &agentloop.FreeStateDecision{Summary: schedulerTerminalReply},
		CreatedAt:      now, UpdatedAt: now,
	})

	park := DurableContinuation{
		ContinuationID: "cont_f2_park", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationWaitingInteraction,
	}
	s.settleAndDeliverContinuationChainEnd(context.Background(), park, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusWaitingConfirmation), StopReason: "human_judgment_required",
	}, true)

	events, _ := s.agentEventsSince("conversation-f2", 0, 64)
	var delivered *AgentEvent
	for index := range events {
		event := events[index]
		if event.Type != "turn.completed" || event.ItemID != "chain_result" {
			continue
		}
		delivered = &event
		break
	}
	if delivered == nil {
		t.Fatalf("waiting-park chain end must deliver a scheduler_chain terminal event: %+v", events)
	}
	if marked, _ := delivered.Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("waiting-park terminal must carry the scheduler_chain marker: %+v", delivered.Payload)
	}
	if delivered.Body != schedulerTerminalReply {
		t.Fatalf("waiting-park terminal body must carry the terminal reply (decision-summary fallback): %q", delivered.Body)
	}
	if delivered.Status != string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("waiting-park terminal must annotate the waiting status, got %q", delivered.Status)
	}
	if status, _ := delivered.Payload["goal_status"].(string); status != string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("waiting-park terminal payload must annotate the waiting goal status: %+v", delivered.Payload)
	}
	if stopReason, _ := delivered.Payload["stop_reason"].(string); stopReason != "human_judgment_required" {
		t.Fatalf("waiting-park terminal payload must keep the stop reason: %+v", delivered.Payload)
	}
	if !schedulerTerminalHydrationContains(t, s, projectPath, schedulerTerminalReply) {
		t.Fatalf("waiting-park terminal reply must land in the conversation graph for refresh recovery: %+v", schedulerTerminalHydratedMessages(t, s, projectPath))
	}
	// The persisted node is a terminal report, not an interaction offer: a
	// proposal-kind node re-rendered as a consumed interaction card on refresh
	// and its text merged away the scheduler_chain bubble (run 20260910_202854).
	for _, message := range schedulerTerminalHydratedMessages(t, s, projectPath) {
		if strings.Contains(message.Content, schedulerTerminalReply) {
			if message.MessageKind == "proposal" || message.MessageData != nil {
				t.Fatalf("terminal node must hydrate as a plain report, not an interaction card: kind=%s data=%+v", message.MessageKind, message.MessageData)
			}
		}
	}
	if status := s.harness.RuntimeStatus("goal-f2").Status; status != agentruntime.StatusWaitingConfirmation {
		t.Fatalf("waiting park must stay answerable after delivery, got %s", status)
	}

	// Second park form (20260910_212325 real-stack): the audition judgment park
	// whose slice acked stop=done — record completed, response goal completed —
	// while the loop parked the goal at the judgment boundary inside the
	// slice. The waiting branch keys on the answerable goal status, so this
	// shape must deliver too; the event annotates the waiting truth, not the
	// slice's completed ack.
	completedPark := DurableContinuation{
		ContinuationID: "cont_f2_park_done", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationCompleted,
	}
	s.settleAndDeliverContinuationChainEnd(context.Background(), completedPark, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusCompleted), StopReason: "done", Reply: "观测评估已完成，等待你的试听判定。",
	}, true)

	events, _ = s.agentEventsSince("conversation-f2", 0, 64)
	delivered = nil
	for index := range events {
		event := events[index]
		if event.Type != "turn.completed" || event.ItemID != "chain_result" || event.Body != "观测评估已完成，等待你的试听判定。" {
			continue
		}
		if event.Status != string(agentruntime.StatusWaitingConfirmation) {
			t.Fatalf("judgment park behind a done-acked slice must annotate the waiting goal status, got %q", event.Status)
		}
		delivered = &event
		break
	}
	if delivered == nil {
		t.Fatalf("judgment park behind a done-acked slice must deliver the terminal event: %+v", events)
	}
}

// F2 pin (2): a scheduler chain that truly settles (terminal slice, goal
// completed) must write its terminal reply as a conversation-graph vit node
// so the cold-read hydration recovers it. B1-DIAG Q3: vit nodes existed only
// on the HTTP finalize boundary, so scheduled chains vanished on refresh.
func TestTerminalSettleSliceWritesVitNodeAndHydrates(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "improve the mix")
	s.harness.SetGoalStatus("goal-f2", agentruntime.StatusCompleted, nil)

	settled := DurableContinuation{
		ContinuationID: "cont_f2_settled", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationCompleted,
	}
	reply := "混音观察链已完成：候选已应用并通过评估。"
	s.settleAndDeliverContinuationChainEnd(context.Background(), settled, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusCompleted), Reply: reply,
	}, true)

	events, _ := s.agentEventsSince("conversation-f2", 0, 64)
	var delivered bool
	for _, event := range events {
		if event.Type == "turn.completed" && event.ItemID == "chain_result" && event.Body == reply {
			if marked, _ := event.Payload["scheduler_chain"].(bool); marked {
				delivered = true
			}
		}
	}
	if !delivered {
		t.Fatalf("terminal settle must keep delivering the scheduler_chain event: %+v", events)
	}
	if !schedulerTerminalHydrationContains(t, s, projectPath, reply) {
		t.Fatalf("terminal settle reply must land in the conversation graph (B1-DIAG Q3): %+v", schedulerTerminalHydratedMessages(t, s, projectPath))
	}
}

// F2 pin (3): anti-regression on the delivery gate. A slice that ends while
// the chain still owns a next automatic slice must not deliver anything
// (mid-chain acks are not terminals), and a terminal slice whose loop parked
// waiting_continue must not be re-vocabularied into a waiting interaction —
// the F2 waiting branch is bound to the answerable interaction parks only.
func TestSchedulerChainDeliveryGateBoundariesHold(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "improve the mix")

	midChain := DurableContinuation{
		ContinuationID: "cont_f2_mid", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationCompleted,
	}
	s.settleAndDeliverContinuationChainEnd(context.Background(), midChain, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "limit_reached",
		Reply: "第一片完成，后台继续。",
	}, false)

	events, _ := s.agentEventsSince("conversation-f2", 0, 64)
	for _, event := range events {
		if event.ItemID == "chain_result" {
			t.Fatalf("a mid-chain slice must never deliver a chain result: %+v", event)
		}
	}
	if schedulerTerminalHydrationContains(t, s, projectPath, "第一片完成，后台继续。") {
		t.Fatal("a mid-chain slice reply must not be persisted as a terminal node")
	}

	// The gate drives delivery off the runtime goal status. A waiting-continue
	// goal behind a waiting_interaction record (legacy unanswerable shell) must
	// stay undelivered — only waiting_confirmation/waiting_clarification are
	// answerable parks.
	legacy := DurableContinuation{
		ContinuationID: "cont_f2_legacy", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationWaitingInteraction,
	}
	s.harness.SetGoalStatus("goal-f2", agentruntime.StatusWaitingContinue, nil)
	s.settleAndDeliverContinuationChainEnd(context.Background(), legacy, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "legacy checkpoint has no authoritative stop reason",
	}, true)
	events, _ = s.agentEventsSince("conversation-f2", 0, 64)
	for _, event := range events {
		if event.ItemID == "chain_result" {
			t.Fatalf("an unanswerable waiting_continue park must not deliver a chain result: %+v", event)
		}
	}
}

// F2 pin (4): old sessions whose conversation graph carries only the HTTP
// finalize nodes (ask + plain vit, no scheduler terminal node) must keep
// hydrating cleanly, and a scheduler chain ending afterwards must append the
// terminal node alongside them without a format conflict.
func TestColdReadHydrationCompatibleWithoutTerminalNode(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "improve the mix")

	seeded := schedulerTerminalHydratedMessages(t, s, projectPath)
	if len(seeded) != 0 {
		t.Fatalf("setup: fresh workspace must hydrate empty: %+v", seeded)
	}

	legacy := DurableContinuation{
		ContinuationID: "cont_f2_legacy_graph", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "improve the mix",
		Status: ContinuationCompleted,
	}
	legacyReply := "旧会话的 HTTP 终局回复。"
	s.settleAndDeliverContinuationChainEnd(context.Background(), legacy, ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusCompleted), Reply: legacyReply,
	}, true)

	messages := schedulerTerminalHydratedMessages(t, s, projectPath)
	if !schedulerTerminalHydrationContains(t, s, projectPath, legacyReply) {
		t.Fatalf("hydration must surface the terminal node appended to the legacy graph: %+v", messages)
	}
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			t.Fatalf("hydration must not produce empty-message rows: %+v", messages)
		}
	}
}

// The deterministic clarify question from the ⑤ R1 durable evidence
// (goal_continuations/goal_b702…/trace[21]): the model asked exactly this and
// the settle synthesis never saw it.
const clarifyParkQuestion = "需要先确认哪条是主唱轨。请告诉我主唱是 Track 几，或把主唱轨重命名为 vocal / 主唱后让我重新观察。"

// F2-SURFACE-REPLY pin (1): a scheduler slice that parks at the clarification
// boundary must deliver the clarify question as the turn-level terminal body.
// ⑤ R1 real-stack shape (conv product_path_vocal_clarify_20260921_201033): the
// slice paused with Result.Reply = the question (durable trace[21]),
// recordGoalResult parked the child checkpoint as waiting_interaction while
// goalContinuations stayed armed for answerability, and the delivery gate's
// live-owner predicate then reported the chain alive — the question died
// without any turn-level event and the settle synthesis fell back to the last
// tool step title ("已完成 ccb_observation_request") as the reply.
func TestClarifyParkSliceEndDeliversQuestionAsTurnEventBody(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "让主唱更靠前")
	s.harness.SetGoalStatus("goal-f2", agentruntime.StatusWaitingClarification, nil)

	// The slice's tool step already delivered its item body (the step title the
	// settle synthesis wrongly captured); the turn-level terminal under test
	// must not be that text.
	s.emitAgentEvent("conversation-f2", AgentEvent{
		Type: "item.completed", GoalID: "goal-f2", RunID: "run-f2", ItemID: "tool_step_1",
		ItemType: "daw_action", Status: "ok", Title: "观察请求", Body: "已完成 ccb_observation_request",
	})

	// recordGoalResult's park shape: the child checkpoint is waiting_interaction
	// (its pending payload carries no question — pendingInteractionFromResult
	// only extracts executed interaction_requests) and goalContinuations stays
	// armed so the user's answer can resume the checkpoint.
	child := DurableContinuation{
		ContinuationID: "cont_f2_clarify_child", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "让主唱更靠前",
		Status: ContinuationWaitingInteraction,
		PendingInteraction: map[string]any{
			"status": "waiting_clarification", "stop_reason": "needs_clarification", "limit_type": "",
		},
	}
	child.Continuation.ContinuationID = child.ContinuationID
	claimed := DurableContinuation{
		ContinuationID: "cont_f2_clarify_slice", GoalID: "goal-f2", RunID: "run-f2",
		ConversationID: "conversation-f2", OriginalIntent: "让主唱更靠前",
		Status: ContinuationCompleted,
	}
	s.mu.Lock()
	s.durableContinuations[child.ContinuationID] = cloneDurableContinuation(child)
	s.durableContinuations[claimed.ContinuationID] = cloneDurableContinuation(claimed)
	s.goalContinuations["goal-f2"] = child.Continuation
	s.mu.Unlock()

	// The scheduler caller's exact bookkeeping: chainEnded from the live-owner
	// predicate, then the delivery gate. chainResp carries the slice's own
	// reply — the deterministic clarify question.
	chainResp := ChatResponse{
		ConversationID: "conversation-f2", GoalID: "goal-f2", RunID: "run-f2",
		GoalStatus: string(agentruntime.StatusWaitingClarification),
		StopReason: agentloop.StopReasonNeedsClarification, Reply: clarifyParkQuestion,
	}
	chainEnded := !s.goalHasLiveContinuationOwner("goal-f2")
	s.settleAndDeliverContinuationChainEnd(context.Background(), claimed, chainResp, chainEnded)

	events, _ := s.agentEventsSince("conversation-f2", 0, 64)
	var delivered *AgentEvent
	for index := range events {
		event := events[index]
		if event.Type != "turn.completed" || event.ItemID != "chain_result" {
			continue
		}
		delivered = &event
		break
	}
	if delivered == nil {
		t.Fatalf("clarify park must deliver a turn-level terminal carrying the question: %+v", events)
	}
	if delivered.Body != clarifyParkQuestion {
		t.Fatalf("clarify-park terminal body must be the user-visible question, not the tool step title: %q", delivered.Body)
	}
	if marked, _ := delivered.Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("clarify-park terminal must carry the scheduler_chain marker: %+v", delivered.Payload)
	}
	if delivered.Status != string(agentruntime.StatusWaitingClarification) {
		t.Fatalf("clarify-park terminal must annotate the waiting status, got %q", delivered.Status)
	}
	if !schedulerTerminalHydrationContains(t, s, projectPath, clarifyParkQuestion) {
		t.Fatalf("clarify-park question must land in the conversation graph: %+v", schedulerTerminalHydratedMessages(t, s, projectPath))
	}
	// The park stays answerable: delivery must not retire the armed checkpoint.
	s.mu.Lock()
	_, armed := s.goalContinuations["goal-f2"]
	s.mu.Unlock()
	if !armed {
		t.Fatal("clarify park must stay resumable after delivery (goalContinuations armed)")
	}
	if status := s.harness.RuntimeStatus("goal-f2").Status; status != agentruntime.StatusWaitingClarification {
		t.Fatalf("clarify park must stay answerable after delivery, got %s", status)
	}
}

// F2-SURFACE-REPLY pin (2): the parked checkpoint's durable pending payload
// must carry the park's user-visible reply (the clarify question) so
// /agent/runtime status consumers — including the smoke settle fallback — can
// read it. The ⑤ R1 durable record carried only {status, stop_reason,
// limit_type} with no question.
func TestClarifyParkPendingInteractionCarriesQuestion(t *testing.T) {
	s, _ := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f2", "run-f2", "让主唱更靠前")
	res := agentloop.Result{
		GoalID: "goal-f2", RunID: "run-f2", TaskID: "task-f2", SliceID: "slice-f2", TurnID: "turn-f2",
		OriginalIntent: "让主唱更靠前", Status: agentruntime.StatusWaitingClarification,
		StopReason: agentloop.StopReasonNeedsClarification,
		Reply:               clarifyParkQuestion,
		ClarificationQuestion: clarifyParkQuestion,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-f2", RunID: "run-f2", TaskID: "task-f2", SliceID: "slice-f2", TurnID: "turn-f2",
			OriginalIntent: "让主唱更靠前", UserText: "让主唱更靠前", Summary: "让主唱更靠前",
		},
	}
	if err := s.recordGoalResult("conversation-f2", res); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	var parked *DurableContinuation
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationWaitingInteraction {
			clone := cloneDurableContinuation(item)
			parked = &clone
		}
	}
	s.mu.Unlock()
	if parked == nil {
		t.Fatal("clarify park result must store a waiting_interaction checkpoint")
	}
	if reply, _ := parked.PendingInteraction["reply"].(string); reply != clarifyParkQuestion {
		t.Fatalf("parked pending payload must carry the clarify question for the runtime status surface: %+v", parked.PendingInteraction)
	}
}
