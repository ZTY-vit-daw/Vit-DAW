package chat

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// D2-2 multi-round chat wiring (GLM rulings 2026-08-26): the tier is injected
// server-side only, the single-round default path is untouched, no audible
// difference recalibrates while true ambiguity is terminal on every tier, and
// the human-judgment boundary persists at experiment scope.

func d2MultiRoundServerLoopForTest(t *testing.T, budget int) (*Server, freeStateReasoningLoop) {
	t.Helper()
	t.Setenv(agentloop.FreeStateD2MultiRoundBudgetEnv, strconv.Itoa(budget))
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-d2", ConversationID: "conversation-d2",
		GoalID: "goal-d2", RunID: "run-d2", Status: "awaiting_experiment", OriginalIntent: "improve the mix",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: now, UpdatedAt: now,
	}
	s := New(nil, nil, nil)
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	return s, loop
}

func d2ApplyAndObserveForTest(t *testing.T, turn *experiment.Turn, actionID, revision string) {
	t.Helper()
	now := time.Now().UTC()
	if _, err := turn.ApplyIntervention(experiment.Intervention{
		ID: actionID, Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true,
		AchievedDelta: map[string]any{"delta_db": -1.0},
		Receipt:       map[string]any{"action_id": actionID, "status": "applied", "after_revision": revision, "transaction_id": "tx-" + actionID, "idempotency_key": "key-" + actionID, "readback_verified": true},
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

func TestD2MultiRoundAdmissionTierWiringInjectsBudgetAndFingerprint(t *testing.T) {
	loop := freeStateReasoningLoop{LoopID: "loop-tier", LatestObservation: d1FreshObservationForTest("7")}

	// No injection: the sealed single-round default, byte-identical path.
	admission, err := freeStateExperimentAdmission(loop, experimentTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	if admission.ExperimentBudget != 1 || admission.IsD2MultiRound() || len(admission.BaselineFingerprint) != 0 || !freeStateAdmissionRunsSingleRound(admission) {
		t.Fatalf("default admission is not single-round: %+v", admission)
	}

	// Injected tier: multi-round admission with the server-derived baseline
	// fingerprint anchoring cumulative dose accounting.
	t.Setenv(agentloop.FreeStateD2MultiRoundBudgetEnv, "3")
	if resolved := agentloop.ResolveD2MultiRoundBudget(); resolved != 3 {
		t.Fatalf("resolved tier=%d", resolved)
	}
	multi, err := freeStateExperimentAdmissionWithTier(loop, experimentTestProposal(), agentloop.ResolveD2MultiRoundBudget())
	if err != nil {
		t.Fatal(err)
	}
	if multi.ExperimentBudget != 3 || !multi.IsD2MultiRound() || !multi.IsD1S1() || freeStateAdmissionRunsSingleRound(multi) {
		t.Fatalf("tier admission=%+v", multi)
	}
	if err := multi.ValidateD2MultiRound(); err != nil {
		t.Fatalf("tier admission rejected by ValidateD2MultiRound: %v", err)
	}
	if firstStringFromMap(multi.BaselineFingerprint, "revision") != "7" {
		t.Fatalf("baseline fingerprint=%+v", multi.BaselineFingerprint)
	}

	_, started := d2MultiRoundServerLoopForTest(t, 2)
	if started.Experiment == nil || started.Experiment.Admission.ExperimentBudget != 2 {
		t.Fatalf("started admission=%+v", started.Experiment.Admission)
	}
}

func TestD2MultiRoundAdmissionTierFailsClosedAndBarsModelBudgetUpgrade(t *testing.T) {
	for _, raw := range []string{"5", "0", "junk", "-2"} {
		t.Setenv(agentloop.FreeStateD2MultiRoundBudgetEnv, raw)
		if got := agentloop.ResolveD2MultiRoundBudget(); got != 1 {
			t.Fatalf("injection %q resolved to budget %d, want fail-closed 1", raw, got)
		}
	}
	_, loop := d2MultiRoundServerLoopForTest(t, 5) // resolves to 1
	if loop.Experiment.Admission.ExperimentBudget != 1 || !freeStateAdmissionRunsSingleRound(loop.Experiment.Admission) {
		t.Fatalf("out-of-range injection widened the tier: %+v", loop.Experiment.Admission)
	}

	// Without an injection the model cannot upgrade the budget itself: a
	// model-supplied admission with budget 3 stays rejected.
	upgraded := loop.Experiment.Admission
	upgraded.ExperimentBudget = 3
	fresh := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-upgrade", ConversationID: "conversation-upgrade", OriginalIntent: "improve the mix", LatestObservation: d1FreshObservationForTest("7")}
	s := New(nil, nil, nil)
	err := s.startFreeStateExperiment(&fresh, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal(), ExperimentAdmission: &upgraded}, "goal", "run")
	if err == nil || !strings.Contains(err.Error(), "experiment_budget must be 1") {
		t.Fatalf("model-supplied budget upgrade admitted: err=%v", err)
	}

	// With an injection the tier is authoritative even when the model supplied
	// a different budget.
	t.Setenv(agentloop.FreeStateD2MultiRoundBudgetEnv, "3")
	narrowed := loop.Experiment.Admission
	narrowed.ExperimentBudget = 2
	fresh = freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-narrowed", ConversationID: "conversation-narrowed", OriginalIntent: "improve the mix", LatestObservation: d1FreshObservationForTest("7")}
	if err := s.startFreeStateExperiment(&fresh, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal(), ExperimentAdmission: &narrowed}, "goal", "run"); err != nil {
		t.Fatal(err)
	}
	if fresh.Experiment.Admission.ExperimentBudget != 3 {
		t.Fatalf("injected tier was narrowed by the model: %+v", fresh.Experiment.Admission)
	}
}

func TestD2MultiRoundInsufficientDoseOpensCalibrationRound(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}},
	})
	if len(loop.Experiment.Rounds) != 2 {
		t.Fatalf("insufficient dose did not open the calibration round: rounds=%d", len(loop.Experiment.Rounds))
	}
	first := loop.Experiment.Rounds[0]
	if first.Decision != experiment.DecisionNextRound || first.DecisionSummary != "insufficient dose; calibrate in next round" {
		t.Fatalf("calibration decision=%q summary=%q", first.Decision, first.DecisionSummary)
	}
}

// GLM ruling 1, part A: no audible difference is an insufficiency signal, not
// ambiguity — the D2-2 tier recalibrates in a next round under the existing
// admission (never raising a dose by itself).
func TestD2MultiRoundNoDifferenceJudgmentRecalibrates(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
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
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
		t.Fatal(err)
	}
	if len(loop.Experiment.Rounds) != 2 || loop.Experiment.Status == experiment.StatusSettled {
		t.Fatalf("no-difference judgment did not recalibrate: rounds=%d status=%s", len(loop.Experiment.Rounds), loop.Experiment.Status)
	}
	first := loop.Experiment.Rounds[0]
	if first.Decision != experiment.DecisionNextRound || first.DecisionSummary != "no audible difference; recalibrate" {
		t.Fatalf("recalibration decision=%q summary=%q", first.Decision, first.DecisionSummary)
	}
	// The landed judgment does not park the experiment (ruling 3 releases on
	// landing): the fresh calibration round keeps the loop live.
	if freeStateJudgmentBoundary(loop) {
		t.Fatal("landed judgment parked the recalibration round")
	}
}

// GLM ruling 1, part B (sealed, survey §4.2-4): a heard difference without
// preference is true ambiguity — terminal on every tier, and no automatic
// next round or dose change may follow it.
func TestD2MultiRoundAmbiguousJudgmentIsTerminalAndBlocksFurtherMutation(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 3)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", "session-d2", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	evidence := experiment.UserJudgmentEvidence{
		SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ConversationID: loop.ConversationID,
		TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: "session-d2",
		CandidateARef: "before.wav", CandidateBRef: "after.wav",
		HeardDifference: experiment.HeardDifferenceYes, Preference: experiment.PreferenceNeither, CreatedAt: time.Now().UTC(),
	}
	if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
		t.Fatal(err)
	}
	turn := loop.Experiment
	if turn.Status != experiment.StatusSettled || turn.Outcome != experiment.OutcomeNeedsJudgment || len(turn.Rounds) != 1 {
		t.Fatalf("ambiguous judgment was not terminal: status=%s outcome=%s rounds=%d", turn.Status, turn.Outcome, len(turn.Rounds))
	}
	if round, _ := turn.CurrentRound(); round.Decision != experiment.DecisionStopped {
		t.Fatalf("ambiguous round decision=%q", round.Decision)
	}
	// Sealed: after the ambiguous settlement, every automatic continuation and
	// dose channel is refused (no new round, no mutation). The runtime's
	// DecideRound carries no ensureLive and can still overwrite the settled
	// round's decision field — recorded as a residual in the execution notes;
	// it opens no round, mutation, or dose channel by itself.
	if _, err := turn.ApplyIntervention(experiment.Intervention{ID: "d2-action-2", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true}, time.Now().UTC()); err == nil {
		t.Fatal("mutation admitted after ambiguous settlement")
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, turn.Admission.CheckpointRef, "8", time.Now().UTC()); err == nil {
		t.Fatal("new round admitted after ambiguous settlement")
	}
}

// GLM ruling 3 chat layer (sealed, survey §4.2-6): the judgment boundary
// persists at experiment scope — a requested-but-unlanded judgment on an
// earlier round refuses revival decisions and any new admission, while the
// settle family stays admitted.
func TestD2MultiRoundJudgmentBoundarySpansRoundsAndRefusesAdmission(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.DecideRound(experiment.DecisionNextRound, "insufficient dose; calibrate in next round", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.StartRound([]string{"track.timbre_frequency"}, loop.Experiment.Admission.CheckpointRef, "8", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// The second round owes (and carries) its fresh post-action observation so
	// the settle-family report passes the settle-report admission guard.
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-2", "9")
	// Simulate the inconsistent restored state the experiment-scope boundary
	// exists for: the earlier round's judgment request never landed while a
	// newer round is current.
	first := loop.Experiment.Rounds[0]
	first.UserJudgmentRequested = true
	loop.Experiment.Rounds[0] = first
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("experiment-scope boundary missed the earlier pending round")
	}
	if !freeStateExperimentJudgmentPending(loop.Experiment) {
		t.Fatal("pending predicate missed the earlier pending round")
	}

	loop.Status = "awaiting_experiment"
	s.storeFreeStateLoop(loop)
	revived, ok := s.recordFreeStateDecision(loop.ConversationID, agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "need more evidence", RequestedViewIDs: []string{"track.basic_energy"},
	}})
	if !ok {
		t.Fatal("loop was not retained")
	}
	if revived.Status != "blocked" || !strings.Contains(revived.LastError, "human judgment boundary") {
		t.Fatalf("revival crossed the experiment-scope boundary: status=%s last_error=%q", revived.Status, revived.LastError)
	}

	// A new admission is refused with the S1 runtime sentinel wording.
	err := s.startFreeStateExperiment(&revived, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, revived.GoalID, revived.RunID)
	if !errors.Is(err, experiment.ErrJudgmentPending) {
		t.Fatalf("new admission error=%v", err)
	}

	// The settle family stays admitted while the loop is still presented
	// active at the boundary (the refusal above stores status=blocked, which
	// deactivates the loop for later model turns — the audition path is the
	// production recovery from there). Re-store the parked-active state the
	// settle report arrives in, mirroring the D1 boundary contract.
	loop.Status = "awaiting_experiment"
	s.storeFreeStateLoop(loop)
	settled, ok := s.recordFreeStateDecision(loop.ConversationID, agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateCapabilityBlocked,
		EvidenceStatus: "insufficient", Summary: "stop at the boundary",
		ExperimentRoundDecision: string(experiment.DecisionStopped),
	}})
	if !ok {
		t.Fatal("settle decision was not recorded")
	}
	round, err := settled.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Decision != experiment.DecisionStopped {
		t.Fatalf("settle round decision=%q", round.Decision)
	}
}

// Card item 7: a multi-round turn restarted through the durable continuation
// projection must not duplicate rounds or interventions, and the second
// round's post-action observation stays bound to the second mutation's
// after_revision.
func TestD2MultiRoundRestartProjectionDoesNotDuplicate(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}},
	})
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-2", "9")

	restored, ok := freeStateLoopFromAny(freeStateLoopMap(loop))
	if !ok || restored.Experiment == nil {
		t.Fatal("multi-round loop did not survive the durable projection")
	}
	// Replay the first round's action receipt and post-action observation the
	// way a restarted transport would.
	s.recordFreeStateExperimentAction(&restored, "eq", "applied", map[string]any{"action_id": "d2-action-1", "status": "applied", "after_revision": "8"})
	if _, err := restored.Experiment.RecordObservation(experiment.Observation{ID: "obs-d2-action-1", Fresh: true, PostAction: true, ProjectRevision: "8"}, true, time.Now().UTC()); err == nil {
		t.Fatal("replayed post-action observation duplicated on the round")
	}
	if len(restored.Experiment.Rounds) != 2 {
		t.Fatalf("restart changed the round count: %d", len(restored.Experiment.Rounds))
	}
	for index, round := range restored.Experiment.Rounds {
		if len(round.Interventions) != 1 {
			t.Fatalf("round %d intervention count=%d", index+1, len(round.Interventions))
		}
	}
	second := restored.Experiment.Rounds[1]
	if len(second.Observations) != 1 || second.Observations[0].ProjectRevision != "9" {
		t.Fatalf("second round post-action observation revision mismatch: %+v", second.Observations)
	}
	if firstStringFromMap(second.Interventions[0].Receipt, "after_revision") != "9" {
		t.Fatalf("second round intervention receipt=%+v", second.Interventions[0].Receipt)
	}
}

// Card item 8: the budget-exhausted settlement maps to the canonical
// capability_blocked task state (Settle enforces the pairing when a contract
// is bound).
func TestD2MultiRoundBudgetExhaustedSettlesCapabilityBlockedCanonicalTask(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	if _, err := s.ensureAudioTaskContract(loop.ConversationID, audioclosure.ModeTreatment, audioclosure.Scope{Kind: "project", ID: "project-1"}, "project-1", "8", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	proposal := taskstate.BoundedProposal{ProposalID: "proposal-d2", Summary: "bounded track gain", EvidenceRefs: []string{"before"}, RequiresExperiment: true}
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventImprovementProposed, Reason: "candidate observed", EvidenceRefs: []string{"before"}, CandidateID: "1007", Proposal: &proposal, ProjectRevision: "8"}); err != nil {
		t.Fatal(err)
	}
	if err := s.bindExperimentSemantic(&loop); err != nil {
		t.Fatal(err)
	}

	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}},
	})
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-2", "9")
	// The budget is spent: the frozen multi-round contract caps rounds at the
	// admission budget, so the second insufficient-dose settle settles the
	// experiment budget-exhausted instead of opening a third (beyond-budget)
	// calibration round. The settle is canonical (capability_blocked) and the
	// loop lands terminal.
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-2"}},
	})
	if len(loop.Experiment.Rounds) != 2 {
		t.Fatalf("beyond-budget calibration round opened: rounds=%d", len(loop.Experiment.Rounds))
	}
	if loop.Experiment.Status != experiment.StatusSettled || loop.Experiment.Outcome != experiment.OutcomeBudgetExhausted {
		t.Fatalf("budget exhaustion did not settle: status=%s outcome=%s", loop.Experiment.Status, loop.Experiment.Outcome)
	}
	if loop.Status != "completed" {
		t.Fatalf("budget-exhausted loop status=%q", loop.Status)
	}
	// After the settlement every mutation channel stays closed: a replayed
	// action cannot book onto the settled experiment.
	s.recordFreeStateExperimentAction(&loop, "eq", "applied", map[string]any{"action_id": "d2-action-3", "status": "applied", "after_revision": "10"})
	if interventions := loop.Experiment.InterventionCount(); interventions != 2 {
		t.Fatalf("budget-exhausted apply was recorded: interventions=%d", interventions)
	}

	s.settleFreeStateExperiment(&loop, experiment.OutcomeBudgetExhausted, "multi-round budget exhausted")
	turn := loop.Experiment
	if turn.Status != experiment.StatusSettled || turn.Outcome != experiment.OutcomeBudgetExhausted {
		t.Fatalf("budget exhaustion did not settle: status=%s outcome=%s", turn.Status, turn.Outcome)
	}
	if turn.TaskState != taskstate.StateCapabilityBlocked {
		t.Fatalf("experiment canonical projection state=%s", turn.TaskState)
	}
	current := s.harness.RuntimeStatus(loop.GoalID)
	if current.Task == nil || current.Task.SemanticState == nil || current.Task.SemanticState.State != taskstate.StateCapabilityBlocked || !current.Task.SemanticState.Terminal {
		t.Fatalf("canonical task semantic state=%+v", current.Task)
	}
}
