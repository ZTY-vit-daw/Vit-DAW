package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// D2-2-S3g: the refused-settle retry turn's completed-status residue and the
// goal-terminal reload fold. The retry turn's settle report can finish the
// turn cleanly (final:true, no replayed tick), so its result envelope carries
// Status=completed past the refusal: the refusal downgrade rewrote only the
// HTTP response, durableContinuationFromResult has no branch for a completed
// result and parks the slice checkpoint at waiting_interaction with an empty
// shell, and the budget enqueue skips it. The same completed envelope flips
// the harness goal terminal inside the race window, so the next scheduler
// reload folds the retry checkpoint to completed at its birth-millisecond
// stamp (attempt=0) and goal_continuations is left empty — every later nudge
// answers no_continuation and the round's owed work is never proposed again
// (2026-08-29 192048 trace: cont_5a13 born pending/interaction-shell, folded
// on reload, goal completed, nudges at 19:26:41/19:28:42 both no_continuation).

// refusedSettleCompletedTurn builds the retry turn's result envelope in the
// 192048 trace shape: the settle report (boundary pair riding the preserved
// proposal) finished the turn, and the completed envelope still carries the
// slice's checkpoint.
func refusedSettleCompletedTurn(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3g",
		SliceID: "slice-s3g", TurnID: "turn-s3g",
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:            agentloop.FreeStateNeedsExperiment,
			Summary:           "settle report finished the retry turn cleanly",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAmbiguous,
				Attempt: 1, EvidenceRefs: []string{"obs-stale"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
				Summary: "boundary pair of the refused report", EvidenceRefs: []string{"obs-stale"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3g",
			SliceID: "slice-s3g", TurnID: "turn-s3g",
			OriginalIntent: loop.OriginalIntent,
			Context:        map[string]any{"free_state_reasoning_loop": map[string]any{"status": "observing"}},
		},
	}
}

// driveRefusedSettleCompleted mirrors goalrunner_chat.go's retry-turn
// sequence: recordFreeStateDecision (which refuses the report and books the
// settle-refused marker on the round), the loop copy-back onto the result
// envelope, then the durable checkpoint recording.
func driveRefusedSettleCompleted(t *testing.T, s *Server, loop freeStateReasoningLoop) (agentloop.Result, ChatResponse) {
	t.Helper()
	s.storeFreeStateLoop(loop)
	res := refusedSettleCompletedTurn(loop)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("retry turn was not retained by recordFreeStateDecision")
	}
	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	resp := s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)
	return res, resp
}

// flipGoalTerminalForReload reproduces the 192048 disk precondition: the
// harness goal already flipped terminal while the loop stayed active with the
// round owing its settle retry.
func flipGoalTerminalForReload(t *testing.T, s *Server, goalID, runID string) {
	t.Helper()
	s.harness.EnsureGoal(goalID, runID, "refused-settle retry goal")
	s.harness.SetGoalStatus(goalID, agentruntime.StatusCompleted, nil)
}

// reloadRuntimeState drives the scheduler's durable reload: snapshot the
// in-memory runtime state and restore it, exactly like
// runContinuationSchedulerOnce's reloadActiveRuntimeState round trip.
func reloadRuntimeState(t *testing.T, s *Server) {
	t.Helper()
	s.mu.Lock()
	state := s.projectAgentRuntimeStateLocked()
	s.restoreProjectAgentRuntimeStateLocked(state)
	s.mu.Unlock()
}

// The reproduction, stage one: the refused settle's completed residue must not
// park the retry checkpoint. The refusal semantics say the round still owes
// work, so the envelope's status has to be neutralized to waiting_continue
// with a schedulable checkpoint and its own budget slot — the residue instead
// fell into durableContinuationFromResult's default waiting_interaction shell
// and the enqueue skipped it (192048: used stayed 7/8 across the two turns).
func TestRefusedSettleCompletedResidueKeepsRetryCheckpointSchedulable(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	before, _ := s.freeStateLoop(loop.ConversationID)
	res, resp := driveRefusedSettleCompleted(t, s, loop)

	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(continuations))
	}
	if continuations[0].Status != ContinuationPending {
		t.Fatalf("completed residue parked the refused-settle retry checkpoint: status=%q pending=%+v",
			continuations[0].Status, continuations[0].PendingInteraction)
	}
	after, _ := s.freeStateLoop(loop.ConversationID)
	if after.ContinuationUsed != before.ContinuationUsed+1 {
		t.Fatalf("owed retry was not enqueued on the loop budget: used=%d before=%d",
			after.ContinuationUsed, before.ContinuationUsed)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) ||
		resp.StopReason != "round_owes_intervention_proposal" {
		t.Fatalf("response semantics diverged from the demoted envelope: status=%q stop=%q",
			resp.GoalStatus, resp.StopReason)
	}
	// S3h2 reclassification: this fixture's round never acted (fresh round-1,
	// zero interventions, no post-action debt bit), so the demotion keeps the
	// S3g mechanism (waiting_continue + schedulable checkpoint + budget slot
	// below) but the refusal classification is the owed-intervention guidance,
	// not awaiting-observation.
}

// The reproduction, stage two: the goal-terminal reload fold must spare the
// active loop's owing-round checkpoint. After the flip the reload folded the
// retry checkpoint to completed at its birth stamp (attempt=0) and left
// goal_continuations empty, which is exactly the lookup the bare-continue
// no_continuation branch reads — the armed round chain starved.
func TestRefusedSettleGoalTerminalFoldSparesOwingRoundContinuation(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res, _ := driveRefusedSettleCompleted(t, s, loop)

	birth := durableContinuationsForGoal(s, res.GoalID)
	if len(birth) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(birth))
	}
	if birth[0].Status == ContinuationCompleted || birth[0].Status == ContinuationCancelled || birth[0].Status == ContinuationFailed {
		t.Fatalf("retry checkpoint was already terminal before the reload: %q", birth[0].Status)
	}
	flipGoalTerminalForReload(t, s, res.GoalID, res.RunID)
	reloadRuntimeState(t, s)

	folded := durableContinuationsForGoal(s, res.GoalID)
	if len(folded) != 1 {
		t.Fatalf("expected exactly one durable continuation after reload, got %d", len(folded))
	}
	if folded[0].Status != birth[0].Status || folded[0].Attempt != birth[0].Attempt {
		t.Fatalf("goal-terminal fold retired the owing round's checkpoint: status=%q attempt=%d (birth status=%q attempt=%d)",
			folded[0].Status, folded[0].Attempt, birth[0].Status, birth[0].Attempt)
	}
	// goalrunner_chat.go's no_continuation branch answers from exactly this
	// lookup; an empty result is the starved round-2 chain from the trace.
	if _, ok := s.goalContinuationForConversation(loop.ConversationID); !ok {
		t.Fatal("goal-terminal fold starved the bare-continue gate: later nudges answer no_continuation")
	}
}

// Red line (control): when the loop itself is terminal the goal-terminal fold
// keeps its authority — a really completed free-state loop still retires its
// stale checkpoint on reload. The S3g exemption is scoped to the active-loop
// owing-round race window only.
func TestRefusedSettleGoalTerminalFoldStillRetiresInactiveLoopContinuation(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res, _ := driveRefusedSettleCompleted(t, s, loop)

	stored, _ := s.freeStateLoop(loop.ConversationID)
	stored.Status = "completed"
	s.storeFreeStateLoop(stored)
	flipGoalTerminalForReload(t, s, res.GoalID, res.RunID)
	reloadRuntimeState(t, s)

	folded := durableContinuationsForGoal(s, res.GoalID)
	if len(folded) != 1 {
		t.Fatalf("expected exactly one durable continuation after reload, got %d", len(folded))
	}
	if folded[0].Status != ContinuationCompleted {
		t.Fatalf("inactive loop's checkpoint escaped the goal-terminal fold: status=%q attempt=%d",
			folded[0].Status, folded[0].Attempt)
	}
}
