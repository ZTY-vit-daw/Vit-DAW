package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	agentruntime "vit-daw-agent/internal/runtime"
)

// F5 pin (1): a slice whose execution fails must still deliver the chain's
// terminal. Before F5 the executionErr branch returned before the delivery gate
// (continuation_scheduler.go:957-967), so a dying chain produced zero
// turn.failed deliveries and zero conversation-graph nodes — the same
// "server holds a terminal, the user gets nothing" shape B1 diagnosed for the
// success path (F2 fixed that half). The failure terminal reuses the F2
// delivery shape: scheduler_chain marker, failure body/stop_reason, and a plain
// assistant vit node that the refresh hydration recovers.
func TestFailedChainEndDeliversTurnFailedAndLandsNode(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f5", "run-f5", "improve the mix")
	s.mu.Lock()
	s.durableContinuations["cont_f5_failed"] = DurableContinuation{
		ContinuationID: "cont_f5_failed", GoalID: "goal-f5", RunID: "run-f5",
		ConversationID: "conversation-f5", OriginalIntent: "improve the mix",
		ProjectPath: projectPath, Status: ContinuationPending,
	}
	s.mu.Unlock()

	// The live conversation opened a turn node when the slice started; the
	// scheduler's terminal event closes that same node (F2's success-path shape).
	s.emitChatTurnTrajectoryStarted("conversation-f5", "goal-f5", "run-f5")

	failureDetail := "模型传输链路中断，本片未能完成"
	s.continuationExecutor = func(context.Context, DurableContinuation) error {
		return errors.New(failureDetail)
	}
	if err := s.runContinuationSchedulerOnce(context.Background()); err == nil {
		t.Fatal("a failed slice must surface its execution error to the scheduler caller")
	}
	// The test drives the scheduler synchronously; keep the injected executor
	// from outliving the drive (see the STAB1 note in continuation_scheduler_test).
	s.continuationExecutor = nil

	events, _ := s.agentEventsSince("conversation-f5", 0, 64)
	var delivered *AgentEvent
	for index := range events {
		event := events[index]
		if event.Type != "turn.failed" || event.ItemID != "chain_result" {
			continue
		}
		delivered = &event
		break
	}
	if delivered == nil {
		t.Fatalf("a failed chain end must deliver a turn.failed terminal: %+v", events)
	}
	if marked, _ := delivered.Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("failed chain terminal must carry the scheduler_chain marker: %+v", delivered.Payload)
	}
	if !strings.Contains(delivered.Body, failureDetail) {
		t.Fatalf("failed chain terminal body must carry the failure detail: %q", delivered.Body)
	}
	if strings.TrimSpace(delivered.Status) != string(agentruntime.StatusFailed) {
		t.Fatalf("failed chain terminal must annotate the failed goal status, got %q", delivered.Status)
	}
	if stopReason, _ := delivered.Payload["stop_reason"].(string); strings.TrimSpace(stopReason) == "" {
		t.Fatalf("failed chain terminal must annotate a stop reason: %+v", delivered.Payload)
	}
	if payloadError, _ := delivered.Payload["error"].(string); !strings.Contains(payloadError, failureDetail) {
		t.Fatalf("failed chain terminal payload must carry the failure error: %+v", delivered.Payload)
	}
	if goalStatus, _ := delivered.Payload["goal_status"].(string); goalStatus != string(agentruntime.StatusFailed) {
		t.Fatalf("failed chain terminal payload must annotate the failed goal status: %+v", delivered.Payload)
	}
	if types := chainEventTypes(s, "conversation-f5"); types["trajectory.turn.failed"] == 0 {
		t.Fatalf("failed chain end must close the turn trajectory node as failed: %+v", types)
	}

	// Node landing: the failure terminal must survive a refresh through the
	// conversation graph, exactly like the F2 success terminal.
	if !schedulerTerminalHydrationContains(t, s, projectPath, failureDetail) {
		t.Fatalf("failed chain terminal must land in the conversation graph for refresh recovery: %+v", schedulerTerminalHydratedMessages(t, s, projectPath))
	}
	for _, message := range schedulerTerminalHydratedMessages(t, s, projectPath) {
		if !strings.Contains(message.Content, failureDetail) {
			continue
		}
		if message.MessageKind == "proposal" || message.MessageData != nil {
			t.Fatalf("failed terminal node must hydrate as a plain report, not an interaction card: kind=%s data=%+v", message.MessageKind, message.MessageData)
		}
	}
	if status := s.harness.RuntimeStatus("goal-f5").Status; status != agentruntime.StatusFailed {
		t.Fatalf("a failed chain must settle its goal as failed, got %s", status)
	}
}

// F5 pin (2): the boundary the failure delivery must not cross. A slice whose
// context was cancelled is a transient lease release, not a chain terminal —
// the claim goes back to pending for a retry, so emitting turn.failed (or
// landing a terminal node) there would tell the user the chain died while the
// scheduler is about to run it again.
func TestCancelledSliceReleaseDeliversNoTerminal(t *testing.T) {
	s, projectPath := schedulerTerminalServerForTest(t)
	s.harness.EnsureGoal("goal-f5c", "run-f5c", "improve the mix")
	s.mu.Lock()
	s.durableContinuations["cont_f5_cancel"] = DurableContinuation{
		ContinuationID: "cont_f5_cancel", GoalID: "goal-f5c", RunID: "run-f5c",
		ConversationID: "conversation-f5c", OriginalIntent: "improve the mix",
		ProjectPath: projectPath, Status: ContinuationPending,
	}
	s.mu.Unlock()

	s.continuationExecutor = func(context.Context, DurableContinuation) error {
		return context.Canceled
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.runContinuationSchedulerOnce(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled slice must surface the cancellation, got %v", err)
	}
	s.continuationExecutor = nil

	events, _ := s.agentEventsSince("conversation-f5c", 0, 64)
	for _, event := range events {
		if event.ItemID == "chain_result" {
			t.Fatalf("a cancelled slice must not deliver a chain terminal: %+v", event)
		}
	}
	if len(schedulerTerminalHydratedMessages(t, s, projectPath)) != 0 {
		t.Fatalf("a cancelled slice must not land a terminal node: %+v", schedulerTerminalHydratedMessages(t, s, projectPath))
	}
	s.mu.Lock()
	released := s.durableContinuations["cont_f5_cancel"]
	s.mu.Unlock()
	if released.Status != ContinuationPending {
		t.Fatalf("a cancelled slice must release its claim for retry, got %s", released.Status)
	}
	if status := s.harness.RuntimeStatus("goal-f5c").Status; status == agentruntime.StatusFailed {
		t.Fatal("a cancelled slice is a transient release, not a goal failure")
	}
}

// F5 pin (3): the failure body is composed without a new LLM call and stays
// human-first: the failed slice's own report survives beneath the failure, the
// executor error text rides as the detail, and the B6 internal-code guardrail
// still runs over the composed text. The identity fallback is pinned here too —
// the injected executor and the pre-handling failures return an empty response,
// and an identity-less response is exactly what emitSchedulerChainResultEvent
// drops silently.
func TestSchedulerChainFailureReplyStaysHumanAndSanitized(t *testing.T) {
	body := schedulerChainFailureReply("本片已完成两轮观察。", "FS2 阶段执行超时")
	if !strings.Contains(body, "本片已完成两轮观察。") {
		t.Fatalf("the failed slice's own report must survive beneath the failure: %q", body)
	}
	if !strings.Contains(body, "执行超时") {
		t.Fatalf("the failure detail must ride the terminal body: %q", body)
	}
	if strings.Contains(body, "FS2") {
		t.Fatalf("the failure body must not leak internal phase codes: %q", body)
	}
	if strings.TrimSpace(schedulerChainFailureReply("", "")) == "" {
		t.Fatal("a failure with no detail must still produce a human terminal body")
	}

	resp := schedulerChainFailureResponse(DurableContinuation{
		ContinuationID: "cont_f5_shape", GoalID: "goal-f5", RunID: "run-f5", ConversationID: "conversation-f5",
	}, ChatResponse{}, errors.New("executor unavailable"))
	if resp.ConversationID != "conversation-f5" || resp.GoalID != "goal-f5" || resp.RunID != "run-f5" {
		t.Fatalf("failure response must inherit the durable identity: %+v", resp)
	}
	if resp.GoalStatus != string(agentruntime.StatusFailed) || strings.TrimSpace(resp.StopReason) == "" {
		t.Fatalf("failure response must annotate the failed terminal: %+v", resp)
	}
	if !strings.Contains(resp.Reply, "executor unavailable") {
		t.Fatalf("failure response must carry the executor error as the detail: %q", resp.Reply)
	}

	// Human-first detail: the executor envelope this file itself wraps around a
	// slice failure is an operator-facing prefix, so the user body keeps the
	// reason and drops the envelope. The payload error keeps the raw detail.
	enveloped := schedulerChainFailureResponse(DurableContinuation{
		ContinuationID: "cont_f5_env", GoalID: "goal-f5", RunID: "run-f5", ConversationID: "conversation-f5",
	}, ChatResponse{}, errors.New("durable continuation failed: 模型服务返回了无效的决策格式"))
	if strings.Contains(enveloped.Reply, "durable continuation failed") {
		t.Fatalf("the machine envelope must not reach the user surface: %q", enveloped.Reply)
	}
	if !strings.Contains(enveloped.Reply, "模型服务返回了无效的决策格式") {
		t.Fatalf("the failure reason must survive the envelope strip: %q", enveloped.Reply)
	}
	if !strings.Contains(enveloped.Error, "durable continuation failed") {
		t.Fatalf("the payload error must keep the raw executor detail: %q", enveloped.Error)
	}
	if strings.TrimSpace(stripSchedulerChainFailureEnvelope(schedulerChainFailureEnvelopePrefix)) == "" {
		t.Fatal("an envelope-only detail must not be emptied")
	}
}
