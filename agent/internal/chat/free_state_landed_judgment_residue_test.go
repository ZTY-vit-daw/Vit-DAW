package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// D2-2-S3h5: the landed-judgment boundary residue. While the round-1 settle
// report parks at the human judgment boundary, recordFreeStateDecision stores
// its decision — experiment_round_decision=user_judgment_pending together with
// experiment_target_response.outcome=human_audition_ready — as
// loop.LatestDecision. The judgment POST then lands (DecideRound(next_round),
// round 2 opens, the loop returns to active), but nothing rewrites
// LatestDecision: the settled round-1 boundary pair rides on. Round 2's first
// turn stops at the audio-closure MaxTurns=1 slice limit with no new decision;
// runAgentLoopChat copies loop.LatestDecision onto the result envelope, and
// continuationRequiresUserInteraction's decision double-check classifies the
// limit stop as an interaction boundary — the checkpoint parks at
// waiting_interaction with an empty-shell pending payload (no requests, no
// interaction id). The scheduler never claims it and every "继续" nudge is
// swallowed by the bare-continue gate, so round 2's owed intervention is never
// proposed (2026-08-29 223957 trace: round_two_interventions=0,
// round_two_nudges=3).

// landedJudgmentLoopForTest drives a D2-2 loop to the moment the human judgment
// lands: round 1 applied and observed, settle report accepted at the judgment
// boundary (LatestDecision still carrying the parked boundary signal pair),
// judgment evidence recorded, judgment outcome not yet applied.
func landedJudgmentLoopForTest(t *testing.T, budget int) (*Server, freeStateReasoningLoop, experiment.UserJudgmentEvidence) {
	t.Helper()
	s, loop := d2MultiRoundServerLoopForTest(t, budget)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	loop.LatestProjectChange = map[string]any{"project_revision": "8"}
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", "session-d2", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// The settle turn's stored decision: the full boundary signal pair exactly
	// as recordFreeStateDecision persisted it while the round was parked.
	loop.LatestDecision = &agentloop.FreeStateDecision{
		Status:                  agentloop.FreeStateNeedsExperiment,
		Summary:                 "round 1 settle parked at the judgment boundary",
		ImprovementProposal:     experimentTestProposal(),
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		ExperimentTargetResponse: &experiment.TargetEvaluation{
			Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
			EvidenceRefs: []string{"obs-d2-action-1"},
		},
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	evidence := experiment.UserJudgmentEvidence{
		SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ConversationID: loop.ConversationID,
		TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: "session-d2",
		CandidateARef: "before.wav", CandidateBRef: "after.wav",
		HeardDifference: experiment.HeardDifferenceNo, Preference: experiment.PreferenceUnsure, CreatedAt: time.Now().UTC(),
	}
	if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return s, loop, evidence
}

// The judgment landing opens round 2; the settled round-1 boundary pair on
// LatestDecision is factually stale at that instant (round 1 decided
// next_round, the loop is active, the boundary is released) and must not
// survive as classification input for the new round's checkpoints.
func TestLandedJudgmentRecalibrationNeutralizesBoundaryResidue(t *testing.T) {
	s, loop, evidence := landedJudgmentLoopForTest(t, 2)
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
		t.Fatal(err)
	}
	if len(loop.Experiment.Rounds) != 2 {
		t.Fatalf("judgment landing did not open round 2: rounds=%d", len(loop.Experiment.Rounds))
	}
	if loop.LatestDecision == nil {
		t.Fatal("neutralization dropped the latest decision entirely")
	}
	if decision := loop.LatestDecision.ExperimentRoundDecision; decision == string(experiment.DecisionUserJudgment) {
		t.Fatal("landed judgment boundary residue survived the recalibration round opening")
	}
	if target := loop.LatestDecision.ExperimentTargetResponse; target != nil &&
		strings.EqualFold(strings.TrimSpace(string(target.Outcome)), string(trajectory.EvaluationHumanAuditionReady)) {
		t.Fatalf("landed judgment target response survived the recalibration round opening: %+v", target)
	}
	if continuationRequiresUserInteraction(agentloop.Result{FreeStateDecision: loop.LatestDecision}) {
		t.Fatal("neutralized latest decision still classifies a result as user-interaction-bound")
	}
}

// The reproduction: round 2's limit stop (the owed round's automatic-resume
// semantics) must stay schedulable. With the residue neutralized at the round
// boundary, the loop copy-back onto the limit-stop envelope no longer parks
// the checkpoint at an empty-shell waiting_interaction.
func TestOwedRoundLimitStopNotParkedByLandedJudgmentResidue(t *testing.T) {
	s, loop, evidence := landedJudgmentLoopForTest(t, 2)
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(loop)

	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3h5",
		SliceID: "slice-s3h5", TurnID: "turn-s3h5",
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		LimitType:  "max_turns",
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-s3h5",
			SliceID: "slice-s3h5", TurnID: "turn-s3h5",
			OriginalIntent: loop.OriginalIntent,
			Context:        map[string]any{"free_state_reasoning_loop": map[string]any{"status": "active"}},
		},
	}
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("limit stop turn was not retained by recordFreeStateDecision")
	}
	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	_ = s.chatResponseFromAgentLoopResult(loop.ConversationID, "default", res)

	continuations := durableContinuationsForGoal(s, res.GoalID)
	if len(continuations) != 1 {
		t.Fatalf("expected exactly one durable continuation, got %d", len(continuations))
	}
	if continuations[0].Status != ContinuationPending {
		t.Fatalf("landed-judgment residue parked the owed round's limit stop: status=%q pending=%+v",
			continuations[0].Status, continuations[0].PendingInteraction)
	}
	if item, ok := s.interactionContinuationForConversation(loop.ConversationID); ok {
		t.Fatalf("limit stop left an interaction park for the bare-continue gate: status=%q pending=%+v",
			item.Status, item.PendingInteraction)
	}
}

// Red line (control): while the judgment is genuinely pending — requested but
// not landed — the boundary signal pair on LatestDecision remains the legal
// park input. The landed-round neutralization must not widen to the pending
// window.
func TestPendingJudgmentResidueStillClassifiesAsInteraction(t *testing.T) {
	_, loop, _ := landedJudgmentLoopForTest(t, 2)
	if loop.LatestDecision == nil {
		t.Fatal("setup lost the settle decision residue")
	}
	if !continuationRequiresUserInteraction(agentloop.Result{FreeStateDecision: loop.LatestDecision}) {
		t.Fatal("pending judgment boundary lost its interaction-bound classification")
	}
}
