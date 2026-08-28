package chat

// D2-2-S4 chat execution-bridge multi-round wiring tests: the plan builders
// tier-branch their admission validation (single-round stays byte-for-byte,
// the D2-2 tier runs ValidateD2MultiRound), durable identities are unique per
// round from round 2 on, the cumulative-dose pre-check refuses an overshoot
// before the kernel mutation, and the D2 intervention booking carries the
// achieved delta the experiment runtime's cumulative accounting requires.

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/trajectory"
)

// d2ExecutionSingleRoundLoopForTest starts the sealed single-round admission
// (no tier injection) for the byte-for-byte comparisons.
func d2ExecutionSingleRoundLoopForTest(t *testing.T) freeStateReasoningLoop {
	t.Helper()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d2-single", ConversationID: "conversation-d2-single",
		GoalID: "goal-d2-single", RunID: "run-d2-single", Status: "awaiting_experiment", OriginalIntent: "improve the mix",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

// d2ExecutionMultiRoundLoopWithDeltaForTest starts a D2-2 multi-round
// admission with an explicit admitted delta so cumulative-bound scenarios can
// vary the dose.
func d2ExecutionMultiRoundLoopWithDeltaForTest(t *testing.T, budget int, deltaDB float64) freeStateReasoningLoop {
	t.Helper()
	t.Setenv(agentloop.FreeStateD2MultiRoundBudgetEnv, strconv.Itoa(budget))
	proposal := experimentTestProposal()
	proposal.ParameterBounds["delta_db"] = deltaDB
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d2-exec", ConversationID: "conversation-d2-exec",
		GoalID: "goal-d2-exec", RunID: "run-d2-exec", Status: "awaiting_experiment", OriginalIntent: "improve the mix",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: proposal}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return loop
}

// d2ExecutionOpenCalibrationRoundForTest applies the round-1 mutation, books
// its post-action observation, and lands the insufficient-dose settle that
// opens the calibration round.
func d2ExecutionOpenCalibrationRoundForTest(t *testing.T, s *Server, loop *freeStateReasoningLoop, deltaDB float64) {
	t.Helper()
	d2ExecutionApplyForTest(t, loop.Experiment, "d2-exec-action-1", "8", deltaDB)
	loop.LatestProjectChange = map[string]any{"project_revision": "8"}
	s.recordFreeStateExperimentDecision(context.Background(), loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-exec-action-1"}},
	})
	if len(loop.Experiment.Rounds) != 2 {
		t.Fatalf("calibration round did not open: rounds=%d", len(loop.Experiment.Rounds))
	}
}

func d2ExecutionApplyForTest(t *testing.T, turn *experiment.Turn, actionID, revision string, deltaDB float64) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := turn.ApplyIntervention(experiment.Intervention{
		ID: actionID, Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true,
		AchievedDelta: map[string]any{"delta_db": deltaDB},
		Receipt:       map[string]any{"action_id": actionID, "status": "applied", "after_revision": revision, "transaction_id": "tx-" + actionID},
	}, now); err != nil {
		t.Fatal(err)
	}
	observation := experiment.Observation{
		ID: "obs-" + actionID, RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"},
		ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: revision, EvidenceRefs: []string{"obs-" + actionID},
	}
	if _, err := turn.RecordObservation(observation, true, now); err != nil {
		t.Fatal(err)
	}
}

func TestD2MultiRoundPlanBuildersAcceptMultiRoundAdmission(t *testing.T) {
	// The single-round tier first (before any tier injection): identical
	// historical action identity and the sealed admission guard.
	single := d2ExecutionSingleRoundLoopForTest(t)
	if err := validateD1TierAdmission(single.Experiment.Admission); err != nil {
		t.Fatalf("single-round admission rejected: %v", err)
	}
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	singlePlan, err := d1TrackGainPlan(single, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "vocal", DeltaDB: -1}, 7, "project-1", "epoch-1", "snapshot-7", state)
	if err != nil {
		t.Fatal(err)
	}
	singleAction := singlePlan.ActionSet.Actions[0]
	if singleAction.ID != "d1_"+sanitizeCanaryID(single.Experiment.ID)+"_gain" {
		t.Fatalf("single-round action id drifted from the historical composition: %q", singleAction.ID)
	}

	// Regression for the S3 real-stack gap: a budget-2 admission died at the
	// plan builder with "D1-S1 experiment_budget must be 1" before any
	// intervention could execute. Round 1 keeps the same composition.
	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	plan, err := d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "vocal", DeltaDB: -1}, 7, "project-1", "epoch-1", "snapshot-7", state)
	if err != nil {
		t.Fatalf("multi-round track_gain plan rejected: %v", err)
	}
	action := plan.ActionSet.Actions[0]
	if action.ID != "d1_"+sanitizeCanaryID(loop.Experiment.ID)+"_gain" || strings.Contains(action.ID, "_round_") {
		t.Fatalf("round-1 action id lost its historical shape: %q", action.ID)
	}

	// The PluginBound table builder accepts the multi-round admission on its
	// historical stub path too.
	eqLoop := d1StaticEQLoopForTest(t, "7")
	eqState := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -2.0}}}
	eqPlan, err := d1PluginParamPlanWithBinding(eqLoop, agentloop.PendingMixTickCandidate{Operation: d1StaticEQKind, TrackID: "vocal"}, 7, "project-1", "epoch-1", "snapshot-7", eqState, nil)
	if err != nil {
		t.Fatalf("multi-round static_eq plan rejected: %v", err)
	}
	if !strings.HasSuffix(eqPlan.ActionSet.Actions[0].ID, "_eq") {
		t.Fatalf("multi-round static_eq action id: %q", eqPlan.ActionSet.Actions[0].ID)
	}

	// Tier fail-closed: a domain member above the D2-2 ceiling falls back to
	// the single-round guards and is rejected.
	aboveCeiling := loop.Experiment.Admission
	aboveCeiling.ExperimentBudget = experiment.MaxD2MultiRoundBudget + 1
	if err := validateD1TierAdmission(aboveCeiling); err == nil || !strings.Contains(err.Error(), "experiment_budget must be 1") {
		t.Fatalf("above-ceiling tier admitted: err=%v", err)
	}
}

func TestD2MultiRoundRoundScopedIdentities(t *testing.T) {
	// The single-round tier never grows a suffix (built before the tier
	// injection so its admission is genuinely budget 1).
	single := d2ExecutionSingleRoundLoopForTest(t)
	if got := d1MultiRoundRoundScopeSuffix(single); got != "" {
		t.Fatalf("single-round suffix=%q", got)
	}

	s := New(nil, nil, nil)
	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	if got := d1MultiRoundRoundScopeSuffix(loop); got != "" {
		t.Fatalf("round 1 must keep the historical identity, got suffix %q", got)
	}
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.0)

	// Round 2 owns a distinct durable session id and a distinct plan action
	// id: reusing round 1's completed session would re-project round 1's
	// receipt as this round's intervention and duplicate its transaction
	// identity (probe-fatal).
	if got := d1MultiRoundRoundScopeSuffix(loop); got != "_round_2" {
		t.Fatalf("round-2 suffix=%q", got)
	}
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -3.0}}}
	plan, err := d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "vocal", DeltaDB: -1}, 8, "project-1", "epoch-1", "snapshot-8", state)
	if err != nil {
		t.Fatalf("round-2 plan rejected: %v", err)
	}
	if !strings.Contains(plan.ActionSet.Actions[0].ID, "_round_2_gain") {
		t.Fatalf("round-2 action id is not round-scoped: %q", plan.ActionSet.Actions[0].ID)
	}
}

func TestD2MultiRoundCalibrationRoundGrantsContinuation(t *testing.T) {
	s := New(nil, nil, nil)
	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	loop.ContinuationUsed = 5
	loop.ContinuationBudget = 6
	loop.PostActionObservationReserved = true
	loop.PostApplyBudgetReserved = true
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.0)
	if loop.ContinuationBudget < loop.ContinuationUsed+freeStateD2RoundContinuationAllowance {
		t.Fatalf("calibration round continuation not granted: budget=%d used=%d", loop.ContinuationBudget, loop.ContinuationUsed)
	}
	if loop.PostActionObservationReserved || loop.PostApplyBudgetReserved {
		t.Fatal("per-round post-apply reserves were not re-armed for the calibration round")
	}
}

func TestD2MultiRoundCumulativePreCheckRefusesOvershootBeforeMutation(t *testing.T) {
	s := New(nil, nil, nil)
	// Round 1 applied -1.5 dB: a second -1.5 dB apply projects -3 dB past the
	// S1-frozen 2 dB experiment-lifetime bound and must be refused at the plan
	// boundary, before the kernel mutation.
	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.5)
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.5)
	state := map[string]any{"tracks": []any{map[string]any{"track_id": "vocal", "volume_db": -3.5}}}
	_, err := d1TrackGainPlan(loop, agentloop.PendingMixTickCandidate{Operation: experiment.D1S1ActionKind, TrackID: "vocal", DeltaDB: -1.5}, 8, "project-1", "epoch-1", "snapshot-8", state)
	if err == nil || !strings.Contains(err.Error(), "cumulative delta_db displacement") {
		t.Fatalf("overshooting round-2 plan admitted: err=%v", err)
	}
	if !strings.Contains(err.Error(), "refusing before the mutation") {
		t.Fatalf("refusal is not a pre-mutation boundary: %v", err)
	}

	// The reciprocal-move cancellation the runtime accounting defines: +1.0
	// after -1.5 projects -0.5 dB and stays admissible.
	loop = d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.5)
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.5)
	loop.Experiment.Admission.TypedAction["delta_db"] = 1.0
	if err := d2MultiRoundCheckProjectedCumulativeDelta(loop.Experiment, "delta_db", 1.0); err != nil {
		t.Fatalf("reciprocal calibration refused: %v", err)
	}

	// Boundary: -1.0 twice projects exactly -2 dB, at (not past) the bound.
	loop = d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.0)
	if err := d2MultiRoundCheckProjectedCumulativeDelta(loop.Experiment, "delta_db", -1.0); err != nil {
		t.Fatalf("boundary-dose round refused: %v", err)
	}
}

func TestD2MultiRoundInterventionBookingRecordsAchievedDelta(t *testing.T) {
	// The single-round tier first (before any tier injection): it keeps its
	// historical intervention shape — no achieved delta, no applied_delta_db.
	single := d2ExecutionSingleRoundLoopForTest(t)
	s := New(nil, nil, nil)
	singleReceipt := map[string]any{"action_id": "d1-single-1", "status": "applied", "after_revision": "8"}
	s.recordFreeStateExperimentAction(&single, "track_gain", "applied", singleReceipt)
	singleRound, _ := single.Experiment.CurrentRound()
	if len(singleRound.Interventions) != 1 {
		t.Fatalf("single-round interventions=%d", len(singleRound.Interventions))
	}
	if singleRound.Interventions[0].AchievedDelta != nil {
		t.Fatalf("single-round achieved delta drifted: %+v", singleRound.Interventions[0].AchievedDelta)
	}
	if _, present := singleRound.Interventions[0].Receipt["applied_delta_db"]; present {
		t.Fatal("single-round receipt gained applied_delta_db")
	}

	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	receipt := map[string]any{"action_id": "d2-exec-1", "status": "applied", "after_revision": "8"}
	s.recordFreeStateExperimentAction(&loop, "track_gain", "applied", receipt)
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.Interventions) != 1 {
		t.Fatalf("round interventions=%d err=%v", len(round.Interventions), err)
	}
	intervention := round.Interventions[0]
	if got, ok := intervention.AchievedDelta["delta_db"]; !ok || got != -1.0 {
		t.Fatalf("achieved delta missing or wrong: %+v", intervention.AchievedDelta)
	}
	if got, ok := intervention.Receipt["applied_delta_db"]; !ok || got != -1.0 {
		t.Fatalf("applied_delta_db missing or wrong: %+v", intervention.Receipt)
	}
	// The caller's receipt map is not mutated by the booking.
	if _, leaked := receipt["applied_delta_db"]; leaked {
		t.Fatal("booking mutated the caller's receipt map")
	}
}

func TestD2MultiRoundReceiptCountsForwardMutationsAcrossRounds(t *testing.T) {
	// The single-round tier first (before any tier injection).
	single := d2ExecutionSingleRoundLoopForTest(t)
	d2ExecutionApplyForTest(t, single.Experiment, "d1-single-action-1", "8", -1.0)
	syncD1Receipt(&single)
	if got := single.D1Receipt["forward_mutation_count"]; got != 1 {
		t.Fatalf("single-round forward_mutation_count=%v want 1", got)
	}

	s := New(nil, nil, nil)
	loop := d2ExecutionMultiRoundLoopWithDeltaForTest(t, 2, -1.0)
	d2ExecutionOpenCalibrationRoundForTest(t, s, &loop, -1.0)
	d2ExecutionApplyForTest(t, loop.Experiment, "d2-exec-action-2", "9", -1.0)
	syncD1Receipt(&loop)
	if got := loop.D1Receipt["forward_mutation_count"]; got != 2 {
		t.Fatalf("multi-round forward_mutation_count=%v want 2", got)
	}
}
