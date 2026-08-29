package chat

import (
	"context"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// D2-2-S3d: the settle-report race window. The round's intervention executed
// but its deterministic post-action observation booking had not landed when
// the settle turn returned, so recordFreeStateDecision refuses the settle
// report. In that same envelope the model replayed the round's proposal as a
// pending mix tick; the spent-mutation guard read the loop projection that
// still showed the round pre-action (no interventions booked, no fresh
// post-action evidence, no decision) and none of its predicates fired, so the
// tick was stored and its continuation parked at waiting_interaction — a
// leftover the driver cannot answer and the restart-idempotency check fails
// on (2026-08-29 17:00:50 smoke trace: refused settle, tick stored, and the
// waiting_interaction bind all in the same second).

// settleRaceTurn builds the refused-settle turn's result envelope: settle
// fields riding a needs_experiment decision that still carries the round's
// proposal, the proposal's pending mix tick in execution memory, and a
// waiting-confirmation continuation.
func settleRaceTurn(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-race",
		SliceID: "slice-race", TurnID: "turn-race",
		Status:     agentruntime.StatusWaitingConfirmation,
		StopReason: "improvement_proposal_native_tool_confirmation_required",
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:            agentloop.FreeStateNeedsExperiment,
			Summary:           "settle report for the applied round",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable,
				Attempt: 1, EvidenceRefs: []string{"obs-before"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTickCandidate: &agentloop.PendingMixTickCandidate{
				Operation: "track_gain_adjust", TrackID: "vocal", DeltaDB: -1.0,
				ObservationID: "obs-before", Status: "pending_confirmation",
			},
		},
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-race",
			SliceID: "slice-race", TurnID: "turn-race",
			OriginalIntent: loop.OriginalIntent,
			Context:        map[string]any{"free_state_reasoning_loop": map[string]any{"status": "observing"}},
		},
	}
}

// driveSettleRaceRefusal stores the pre-race loop projection (the round's
// intervention executed, its booking not yet landed: no interventions, no
// post-action observation, no decision) and drives the handler sequence the
// settle turn takes — recordFreeStateDecision first, then the copy-back and
// chatResponseFromAgentLoopResult, mirroring goalrunner_chat.go.
func driveSettleRaceRefusal(t *testing.T, s *Server, loop freeStateReasoningLoop) (agentloop.Result, ChatResponse) {
	t.Helper()
	loop.RequiresPostActionObservation = true
	s.storeFreeStateLoop(loop)
	res := settleRaceTurn(loop)
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

func durableContinuationsForGoal(s *Server, goalID string) []DurableContinuation {
	out := []DurableContinuation{}
	for _, item := range s.durableContinuations {
		if item.GoalID == goalID {
			out = append(out, item)
		}
	}
	return out
}

// The reproduction: a refused settle report's turn must not store its pending
// mix tick and must not leave a waiting_interaction continuation behind. The
// settle chain retries on its own budget once the deterministic post-action
// observation booking lands.
func TestRefusedSettleReportRaceRefusesTickAndContinuationPark(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	res, resp := driveSettleRaceRefusal(t, s, loop)

	if _, ok := s.pendingMixTickForConversation(loop.ConversationID); ok {
		t.Fatal("pending mix tick was stored during the refused-settle race window")
	}
	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(continuations))
	}
	if continuations[0].Status == ContinuationWaitingInteraction {
		t.Fatalf("refused-settle continuation parked at waiting_interaction: %+v", continuations[0].PendingInteraction)
	}
	if continuations[0].Status != ContinuationPending {
		t.Fatalf("refused-settle continuation must stay schedulable, got status=%q", continuations[0].Status)
	}
	if resp.NeedsConfirmation {
		t.Fatalf("refused-settle turn surfaced a confirmation: stop=%q workflow=%q", resp.StopReason, resp.Workflow)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("refused-settle turn answered %q, want waiting_continue", resp.GoalStatus)
	}
}

// The refusal must leave a durable same-round marker the recordGoalResult
// guard sees (the pre-fix guard read the pre-race projection, where neither
// pendingSettlement nor spentMutation held), and the marker must retire once
// the settle report lands after the deterministic booking catches up.
func TestSettleRefusalMarkerLifecycle(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}

	// Refused: no fresh post-action observation on the round yet.
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable,
			Attempt: 1, EvidenceRefs: []string{"obs-before"},
		},
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	})
	if loop.SettleRefusedRoundID != round.ID {
		t.Fatalf("refused settle report did not mark the round: marker=%q round=%s", loop.SettleRefusedRoundID, round.ID)
	}
	if !freeStateLoopRoundSettleRefused(loop) {
		t.Fatal("round with a refused settle report must report the race window")
	}
	if loop.LatestDecision != nil && loop.LatestDecision.ExperimentRoundDecision != "" {
		t.Fatalf("refused settle report left its park marker: %q", loop.LatestDecision.ExperimentRoundDecision)
	}

	// The deterministic booking catches up (intervention + fresh post-action
	// observation), the settle report retries and lands: the marker retires.
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	loop.LatestProjectChange = map[string]any{"project_revision": "8"}
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable,
			Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"},
		},
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	})
	if loop.SettleRefusedRoundID != "" {
		t.Fatalf("accepted settle report left the refusal marker: %q", loop.SettleRefusedRoundID)
	}
	if freeStateLoopRoundSettleRefused(loop) {
		t.Fatal("accepted settle report must close the race window")
	}

	// A marker from an older round is inert once the round advanced.
	stale := loop
	stale.SettleRefusedRoundID = round.ID
	if freeStateLoopRoundSettleRefused(stale) {
		t.Fatal("refusal marker survived a round advance")
	}
}

// The proposal surface itself must not park an unanswerable confirmation in
// the race window: improvementProposalResponse answers waiting_continue so the
// settle chain keeps its own budget (the frozen driver cannot approve a
// replayed proposal confirmation). S3h2: the fixture carries the applied
// boundary's debt bit (RequiresPostActionObservation) — the round executed and
// its booking lagged — so it stays on the acted-race side of the S3h2 copy
// split; a zero-intervention round without the debt bit is the never-acted
// recalibration round, whose refusal copy points at the owed proposal instead.
func TestImprovementProposalSuppressedDuringSettleRaceWindow(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	loop.SettleRefusedRoundID = round.ID
	loop.RequiresPostActionObservation = true
	s.storeFreeStateLoop(loop)
	res := settleRaceTurn(loop)
	resp := s.improvementProposalResponse(loop.ConversationID, "default", res, map[string]any{})
	if resp.NeedsConfirmation || len(resp.InteractionRequests) != 0 {
		t.Fatalf("proposal surfaced a confirmation during the race window: needs=%v requests=%d", resp.NeedsConfirmation, len(resp.InteractionRequests))
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) || resp.StopReason != "settle_report_refused_awaiting_observation" {
		t.Fatalf("unexpected race-window proposal answer: status=%q stop=%q", resp.GoalStatus, resp.StopReason)
	}
}

// Control (default path): a proposal turn for a round that still owes its
// intervention stores its pending tick and parks the confirmation as before —
// the race-window refusal must not widen into the ordinary proposal path.
func TestOrdinaryProposalTurnStillStoresPendingTick(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	loop.RequiresPostActionObservation = false
	s.storeFreeStateLoop(loop)
	res := settleRaceTurn(loop)
	res.FreeStateDecision.ExperimentMateriality = nil
	res.FreeStateDecision.ExperimentRoundDecision = ""
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("ordinary proposal turn was not retained")
	}
	if stored.Experiment == nil || !stored.Experiment.Admission.IsD2MultiRound() {
		t.Fatal("ordinary proposal turn lost its experiment admission")
	}
	resp := s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)
	if _, ok := s.pendingMixTickForConversation(loop.ConversationID); !ok {
		t.Fatal("ordinary proposal turn's pending tick was refused")
	}
	if !resp.NeedsConfirmation {
		t.Fatalf("ordinary proposal turn lost its confirmation surface: status=%q stop=%q", resp.GoalStatus, resp.StopReason)
	}
}
