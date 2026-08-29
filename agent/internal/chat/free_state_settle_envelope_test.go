package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// D2-2-S3f: the refused-settle degrade's result-envelope residue. The settle
// report the prompt contract mandates carries BOTH the judgment-boundary
// signals (experiment_target_response.outcome=human_audition_ready together
// with experiment_round_decision=user_judgment_pending). When the report is
// refused the S3c neutralization cleared only the round decision from the
// loop projection; the surviving target response still flowed through
// runAgentLoopChat's copy-back into the result envelope, where
// continuationRequiresUserInteraction's decision double-check read it and
// parked the turn's continuation at waiting_interaction with an empty-shell
// pending payload (no interaction_id, no requests). The bare-continue gate
// then absorbed every later nudge behind that unanswerable park, so the armed
// round-2 continuation stayed unreachable and the owed intervention was never
// proposed (2026-08-29 183540 trace: nudges #2/#3 stopped with
// pending_interaction_requires_response, interaction_id null).

// refusedSettleEnvelopeTurn builds the refused-settle turn's result envelope
// in its full prompt-contract shape: settle fields riding a needs_experiment
// decision, the target response's human_audition_ready boundary signal, and
// the turn-exhausted waiting_continue checkpoint the 183540 trace recorded.
func refusedSettleEnvelopeTurn(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3f",
		SliceID: "slice-s3f", TurnID: "turn-s3f",
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		LimitType:  "max_turns",
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:            agentloop.FreeStateNeedsExperiment,
			Summary:           "settle report replayed for the round that owes its intervention",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAmbiguous,
				Attempt: 1, EvidenceRefs: []string{"obs-stale"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
				Summary: "replayed boundary signal from the refused report", EvidenceRefs: []string{"obs-stale"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3f",
			SliceID: "slice-s3f", TurnID: "turn-s3f",
			OriginalIntent: loop.OriginalIntent,
			Context:        map[string]any{"free_state_reasoning_loop": map[string]any{"status": "observing"}},
		},
	}
}

// driveRefusedSettleEnvelope mirrors goalrunner_chat.go's settle-turn
// sequence: recordFreeStateDecision (which refuses the report), the loop
// copy-back onto the result envelope, then the durable checkpoint recording.
func driveRefusedSettleEnvelope(t *testing.T, s *Server, loop freeStateReasoningLoop) (agentloop.Result, ChatResponse) {
	t.Helper()
	s.storeFreeStateLoop(loop)
	res := refusedSettleEnvelopeTurn(loop)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("settle turn was not retained by recordFreeStateDecision")
	}
	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	resp := s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)
	return res, resp
}

// The reproduction: a refused settle report's envelope must not park the
// turn's continuation at an unanswerable waiting_interaction. After the
// refusal the round owes its settle retry on its own budget — the residual
// human_audition_ready from the refused report is not a judgment boundary and
// must not classify the continuation as user-interaction-bound.
func TestRefusedSettleEnvelopeResidualDoesNotParkContinuation(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res, _ := driveRefusedSettleEnvelope(t, s, loop)

	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(continuations))
	}
	if continuations[0].Status != ContinuationPending {
		t.Fatalf("refused-settle envelope parked the continuation: status=%q pending=%+v",
			continuations[0].Status, continuations[0].PendingInteraction)
	}
	// The bare-continue gate reads exactly this lookup; an entry here is the
	// nudge-swallowing park from the 183540 trace.
	if item, ok := s.interactionContinuationForConversation(loop.ConversationID); ok {
		t.Fatalf("refused-settle envelope left an interaction park for the bare-continue gate: status=%q pending=%+v",
			item.Status, item.PendingInteraction)
	}
}

// The refusal must neutralize the full judgment-boundary signal on the loop
// projection the envelope copy-back uses: neither the round decision nor the
// human_audition_ready target response of the refused report may survive as
// classification input.
func TestRefusedSettleNeutralizesEntireBoundarySignal(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res, _ := driveRefusedSettleEnvelope(t, s, loop)
	if res.FreeStateDecision == nil {
		t.Fatal("settle turn lost its decision envelope")
	}
	if decision := res.FreeStateDecision.ExperimentRoundDecision; decision != "" {
		t.Fatalf("refused report's round decision survived: %q", decision)
	}
	if target := res.FreeStateDecision.ExperimentTargetResponse; target != nil {
		t.Fatalf("refused report's target response survived: %+v", target)
	}
}

// Red line (control): when the settle report is genuine — the round booked its
// fresh post-action observation and the report is accepted — the real
// human_audition_ready judgment boundary must still park the continuation.
// The refusal-scoped neutralization must not widen past the race window.
func TestGenuineSettleJudgmentBoundaryStillParks(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	loop.LatestProjectChange = map[string]any{"project_revision": "8"}

	s.storeFreeStateLoop(loop)
	res := refusedSettleEnvelopeTurn(loop)
	res.FreeStateDecision.ExperimentMateriality.EvidenceRefs = []string{"obs-d2-action-1"}
	res.FreeStateDecision.ExperimentTargetResponse.EvidenceRefs = []string{"obs-d2-action-1"}
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("genuine settle turn was not retained by recordFreeStateDecision")
	}
	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	_ = s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)

	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(continuations))
	}
	if continuations[0].Status == ContinuationPending {
		t.Fatalf("genuine judgment boundary left the continuation schedulable: %+v", continuations[0].PendingInteraction)
	}
	if marker := firstStringFromMap(continuations[0].PendingInteraction, "status"); marker != "completed_at_human_judgment_boundary" {
		t.Fatalf("genuine judgment boundary did not retire the continuation at the boundary: status=%q pending=%+v",
			continuations[0].Status, continuations[0].PendingInteraction)
	}
}
