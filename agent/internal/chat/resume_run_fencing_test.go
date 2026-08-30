package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-2-CLEAN1-2: read-side run tenure for the conversation/goal-keyed
// continuation reads. The write side already fences completions by run (the
// S3h8 completion bridge matches conversation+goal+run+interaction_id), but
// the scans behind the bare-continue answers matched on the conversation key
// alone: after the same conversation opens a new goal run, an earlier run's
// non-terminal durable — an unanswered waiting_confirmation park, or a parked
// auto-continue checkpoint — still answered the new run's nudges and resumed
// in their place (CLEAN1 audit F8/L2: "旧轮残留吞新轮驱动"). The restore
// normalizer had the same gap one layer down: without a TaskContract its
// RunID rebind silently re-attached an earlier run's record to the goal's
// current run.

const (
	runFenceConversation = "conversation-run-fence"
	runFenceGoal         = "goal-run-fence"
	runFenceCurrentRun   = "run-fence-current"
	runFenceStaleRun     = "run-fence-stale"
)

// runFencedServer wires the conversation to a live harness goal whose current
// run is runFenceCurrentRun — the tenure every scan must read.
func runFencedServer() *Server {
	s := New(nil, nil, nil)
	s.harness.EnsureGoal(runFenceGoal, runFenceCurrentRun, "current task")
	s.mu.Lock()
	s.conversationGoals[runFenceConversation] = runFenceGoal
	s.mu.Unlock()
	return s
}

func storeRunFencePark(s *Server, durable DurableContinuation, interaction *PendingInteraction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.durableContinuations[durable.ContinuationID] = durable
	if interaction != nil {
		s.interactions[interaction.ID] = *interaction
	}
}

func runFencePendingPark(continuationID, runID string, updatedAt time.Time) DurableContinuation {
	return DurableContinuation{
		ContinuationID: continuationID, TaskID: "task-run-fence", GoalID: runFenceGoal, RunID: runID,
		ConversationID: runFenceConversation, OriginalIntent: "inspect the project",
		Continuation: agentloop.Continuation{GoalID: runFenceGoal, RunID: runID, TaskID: "task-run-fence",
			SliceID: "slice-" + continuationID, TurnID: "turn-" + continuationID, OriginalIntent: "inspect the project"},
		Status: ContinuationPending, CreatedAt: updatedAt, UpdatedAt: updatedAt,
	}
}

func runFenceConfirmationPark(continuationID, runID string, updatedAt time.Time) DurableContinuation {
	park := runFencePendingPark(continuationID, runID, updatedAt)
	park.Status = ContinuationWaitingInteraction
	park.PendingInteraction = map[string]any{
		"status": "waiting_confirmation", "interaction_id": "interaction-run-fence", "continuation_id": continuationID,
		"requests": []any{map[string]any{"request_id": "interaction-run-fence", "kind": "confirmation", "prompt": "Confirm this bounded action"}},
	}
	return park
}

func runFenceStoredInteraction() PendingInteraction {
	return PendingInteraction{
		ID: "interaction-run-fence", Kind: "mix_tick_confirmation", Stage: "pending_confirmation",
		ConversationID: runFenceConversation, GoalID: runFenceGoal, RunID: runFenceCurrentRun,
		Payload: map[string]any{"display": map[string]any{"summary": "确认应用这条增益调整"}},
	}
}

func runFenceNudge(t *testing.T, s *Server) ChatResponse {
	t.Helper()
	resp, handled := s.runAgentLoopChat(context.Background(), runFenceConversation, ChatRequest{
		ConversationID: runFenceConversation, Message: "继续",
	}, config.EngineConfig{})
	if !handled {
		t.Fatal("bare continue nudge was not handled")
	}
	return resp
}

// The reproduction: the new run's bare continue must not be intercepted by the
// earlier run's unanswered waiting_confirmation park. The gate's scan fences
// by the conversation goal's current run, so the stale park neither gates the
// nudge nor feeds the drive fallback — the honest no_continuation answer.
func TestCrossRunWaitingConfirmationParkDoesNotInterceptNewRunContinue(t *testing.T) {
	s := runFencedServer()
	interaction := runFenceStoredInteraction()
	storeRunFencePark(s, runFenceConfirmationPark("cont-run-fence-stale", runFenceStaleRun, time.Now().UTC()), &interaction)
	if item, ok := s.interactionContinuationForConversation(runFenceConversation); ok {
		t.Fatalf("cross-run park surfaced to the interception gate: %+v", item)
	}
	resp := runFenceNudge(t, s)
	if resp.StopReason == "pending_interaction_requires_response" {
		t.Fatalf("stale-run confirmation park intercepted the new run's nudge: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.StopReason != "no_continuation" {
		t.Fatalf("new run's nudge did not fall through to the honest continue path: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
}

// The automatic-continue queue answer ("已在自动续跑队列中") must reflect the
// current run's queue only: an earlier run's parked checkpoint must not answer
// the new run's nudge as if it were still scheduled for this conversation.
func TestCrossRunAutomaticQueueDoesNotAnswerNewRunContinue(t *testing.T) {
	s := runFencedServer()
	storeRunFencePark(s, runFencePendingPark("cont-run-fence-queue", runFenceStaleRun, time.Now().UTC()), nil)
	if item, ok := s.automaticContinuationForConversation(runFenceConversation); ok {
		t.Fatalf("cross-run checkpoint surfaced as the conversation's automatic queue: %+v", item)
	}
	resp := runFenceNudge(t, s)
	if resp.StopReason == "automatic_continuation_already_scheduled" {
		t.Fatalf("stale-run checkpoint answered the new run's nudge as scheduled: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.StopReason != "no_continuation" {
		t.Fatalf("new run's nudge did not fall through to the honest continue path: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
}

// The resume chain's two scan sources must skip the earlier run's checkpoint
// while the current run's own checkpoint stays resumable.
func TestCrossRunDurableDoesNotFeedNewRunResumeLookups(t *testing.T) {
	s := runFencedServer()
	now := time.Now().UTC()
	storeRunFencePark(s, runFencePendingPark("cont-run-fence-stale", runFenceStaleRun, now), nil)
	if cont, ok := s.goalContinuationForCurrentGoal(map[string]any{"goal_id": runFenceGoal}); ok {
		t.Fatalf("cross-run checkpoint fed the goal-keyed resume lookup: %+v", cont)
	}
	if cont, ok := s.goalContinuationForConversation(runFenceConversation); ok {
		t.Fatalf("cross-run checkpoint fed the conversation-keyed resume lookup: %+v", cont)
	}
	storeRunFencePark(s, runFencePendingPark("cont-run-fence-current", runFenceCurrentRun, now.Add(time.Second)), nil)
	if _, ok := s.goalContinuationForCurrentGoal(map[string]any{"goal_id": runFenceGoal}); !ok {
		t.Fatal("current run's checkpoint was lost from the goal-keyed resume lookup")
	}
	if _, ok := s.goalContinuationForConversation(runFenceConversation); !ok {
		t.Fatal("current run's checkpoint was lost from the conversation-keyed resume lookup")
	}
}

// Red line (control): a same-run drivable waiting_confirmation park keeps the
// exact interception semantics under the fenced scans, and a same-run pending
// checkpoint keeps answering as the conversation's automatic queue.
func TestRunFenceKeepsCurrentRunBehaviorUnchanged(t *testing.T) {
	s := runFencedServer()
	interaction := runFenceStoredInteraction()
	now := time.Now().UTC()
	storeRunFencePark(s, runFenceConfirmationPark("cont-run-fence-current", runFenceCurrentRun, now), &interaction)
	resp := runFenceNudge(t, s)
	if resp.StopReason != "pending_interaction_requires_response" {
		t.Fatalf("same-run confirmation park lost its interception: stop=%q reply=%q", resp.StopReason, resp.Reply)
	}
	if resp.Reply != "当前任务正在等待明确的确认、澄清或人工判断；单独说‘继续’不会越过这个交互边界。" {
		t.Fatalf("interception reply text changed: %q", resp.Reply)
	}
	if resp.WorkflowData["continuation_id"] != "cont-run-fence-current" || len(resp.InteractionRequests) != 1 || resp.InteractionRequests[0].ID != "interaction-run-fence" {
		t.Fatalf("same-run interception envelope changed: data=%+v requests=%+v", resp.WorkflowData, resp.InteractionRequests)
	}

	queueOnly := runFencedServer()
	storeRunFencePark(queueOnly, runFencePendingPark("cont-run-fence-current", runFenceCurrentRun, now), nil)
	if item, ok := queueOnly.automaticContinuationForConversation(runFenceConversation); !ok || item.ContinuationID != "cont-run-fence-current" {
		t.Fatalf("current run's pending checkpoint lost its automatic queue answer: %+v ok=%v", item, ok)
	}
}

// The restore normalizer must not silently re-attach an earlier run's
// non-terminal record to the goal's current run when no TaskContract guards
// the identity: the record keeps its true run identity and is quarantined
// (recovery_validation_required — the same shape the contract identity check
// uses), while same-run records pass through untouched and identity-corrupt
// records keep the task-snapshot identity repair.
func TestRestoreWithoutTaskContractQuarantinesStaleRunDurable(t *testing.T) {
	now := time.Now().UTC()
	state := projectAgentRuntimeState{
		ConversationGoals: map[string]string{runFenceConversation: runFenceGoal},
		GoalRuntime: agentruntime.Snapshot{Goals: []agentruntime.Goal{{
			GoalID: runFenceGoal, RunID: runFenceCurrentRun, Status: agentruntime.StatusRunning,
			Task: &agentruntime.Task{TaskID: "task-run-fence-current", GoalID: runFenceGoal, OriginalIntent: "current task"},
		}}},
	}
	restoredPark := func(runID string) DurableContinuation {
		park := runFencePendingPark("cont-run-fence-"+runID, runID, now)
		park.ConversationID = runFenceConversation
		return park
	}

	_, stale := normalizeRestoredDurableContinuation("cont-run-fence-restore", restoredPark(runFenceStaleRun), state, now)
	if stale.RunID != runFenceStaleRun {
		t.Fatalf("restore silently re-bound the stale record to the current run: %+v", stale)
	}
	if stale.Status != ContinuationWaitingInteraction || firstStringFromMap(stale.PendingInteraction, "status") != "recovery_validation_required" {
		t.Fatalf("stale-run record was not quarantined on restore: %+v", stale)
	}

	_, current := normalizeRestoredDurableContinuation("cont-run-fence-current", restoredPark(runFenceCurrentRun), state, now)
	if current.Status != ContinuationPending || firstStringFromMap(current.PendingInteraction, "status") != "" {
		t.Fatalf("same-run record did not pass through restore untouched: %+v", current)
	}

	_, corrupt := normalizeRestoredDurableContinuation("cont-run-fence-corrupt", func() DurableContinuation {
		park := restoredPark("run-corrupt")
		park.Status = "unknown_status"
		return park
	}(), state, now)
	if corrupt.RunID != runFenceCurrentRun {
		t.Fatalf("task-snapshot identity repair was lost for the corrupt record: %+v", corrupt)
	}
	if corrupt.Status != ContinuationWaitingInteraction || firstStringFromMap(corrupt.PendingInteraction, "status") != "recovery_validation_required" {
		t.Fatalf("corrupt record did not fail closed: %+v", corrupt)
	}
}
