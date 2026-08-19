package experiment

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/trajectory"
)

func testAdmission(mode AuthorityMode) Admission {
	return Admission{
		SchemaVersion:        SchemaVersion,
		TargetRef:            map[string]any{"kind": "track", "id": "vocal"},
		EvidenceRefs:         []string{"obs-before"},
		Hypothesis:           "a bounded treatment will make the vocal more forward",
		TypedAction:          map[string]any{"domain": "eq", "kind": "bounded"},
		DiagnosticDoseBounds: map[string]any{"max_attempts": 2},
		RetainedDoseBounds:   map[string]any{"max_db": 2},
		ExperimentBudget:     3,
		ExpectedEffect:       "forwardness without transient regression",
		ProtectedDimensions:  []string{"transient", "peak"},
		VerificationPlan:     map[string]any{"views": []string{"track.timbre_frequency"}},
		CheckpointRef:        "checkpoint-before",
		RollbackPlan:         map[string]any{"kind": "restore_checkpoint"},
		AuthorityMode:        mode,
	}
}

func testObservation(id string, postAction bool) Observation {
	return Observation{
		ID: id, ReceiptID: "receipt-" + id,
		RequestedViewIDs: []string{"track.timbre_frequency", "track.transient"},
		ExecutedViewIDs:  []string{"track.transient", "track.timbre_frequency"},
		ViewSetMatches:   true, Fresh: true, PostAction: postAction,
		EvidenceRefs: []string{id + "-evidence"}, RecordedAt: time.Now().UTC(),
	}
}

func testIntervention(attempt int, id string) Intervention {
	return Intervention{ID: id, Attempt: attempt, TechnicalApplication: TechnicalApplied, PolicyAuthorized: true,
		RequestedDelta: map[string]any{"gain_db": 0.5}, AchievedDelta: map[string]any{"gain_db": 0.5},
		Receipt: map[string]any{"status": "readback_ok", "action_id": id}, AppliedAt: time.Now().UTC()}
}

func TestTurnRoundLifecycleEmitsV1Trajectory(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1", TurnID: "turn-1"}, "make the vocal more forward", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	events, err := turn.StartRound([]string{"track.timbre_frequency", "track.transient"}, "checkpoint-before", "rev-1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != trajectory.EventRoundStarted {
		t.Fatalf("round events=%+v", events)
	}
	if events[0].Payload.SchemaVersion != trajectory.SchemaVersion || events[0].Payload.TurnID != "turn-1" || events[0].Payload.RoundID == "" {
		t.Fatalf("bad trajectory payload=%+v", events[0].Payload)
	}
	if err := events[0].Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("before", false), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "action-1"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"action-1-effect"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("after", true), true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordTargetResponse(TargetEvaluation{Response: TargetSufficient, Outcome: trajectory.EvaluationAgentEvaluable, EvidenceRefs: []string{"after-evidence"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.DecideRound(DecisionRetain, "target response sufficient", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.Settle(OutcomeImproved, "retained", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if turn.Status != StatusSettled || turn.Outcome != OutcomeImproved || turn.LastTraceNodeID == "" {
		t.Fatalf("turn not settled=%+v", turn)
	}
}

func TestInsufficientDoseDoesNotDisproveHypothesis(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-dose"}, "stabilize the vocal", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.time_dynamics"}, "checkpoint-dose", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("before-dose", false), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "dose-1"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"dose-1-effect"}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Payload.Materiality != trajectory.EvaluationInsufficientDose {
		t.Fatalf("materiality event=%+v", events)
	}
	round, _ := turn.currentRound()
	if round.Status != RoundDoseCalibrating || round.Decision == DecisionRollback {
		t.Fatalf("insufficient dose ended hypothesis=%+v", round)
	}
	if _, err := turn.DecideRound(DecisionNextRound, "not enough dose", time.Now().UTC()); err == nil {
		t.Fatal("next round should require further dose calibration")
	}
}

func TestTargetResponseRequiresFreshPostActionObservation(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-target"}, "improve clarity", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-target", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("before-target", false), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "action-target"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"material-target"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordTargetResponse(TargetEvaluation{Response: TargetSufficient, Outcome: trajectory.EvaluationAgentEvaluable, EvidenceRefs: []string{"before-target"}}, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "fresh post-action") {
		t.Fatalf("target response error=%v", err)
	}
}

func TestRollbackRestoresRoundStateAndEmitsEvent(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-rollback"}, "reduce harshness", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-rollback", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("before-rollback", false), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "action-rollback"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events, err := turn.MarkRollback(time.Now().UTC(), map[string]any{"status": "restored", "checkpoint_ref": "checkpoint-rollback"}, []string{"before-rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != trajectory.EventRollbackCompleted {
		t.Fatalf("rollback events=%+v", events)
	}
	round, _ := turn.currentRound()
	if round.Status != RoundRolledBack || round.Decision != DecisionRollback {
		t.Fatalf("round=%+v", round)
	}
	if err := events[0].Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCCBViewSetMismatchRejected(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-ccb"}, "inspect", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-ccb", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	observation := testObservation("bad-ccb", false)
	observation.ExecutedViewIDs = []string{"track.peak_structure"}
	observation.ViewSetMatches = false
	if _, err := turn.RecordObservation(observation, false, time.Now().UTC()); err == nil {
		t.Fatal("mismatched CCB view set accepted")
	}
}

func TestMultiRoundExperimentCalibratesDoseThenAdvances(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-multi"}, "improve vocal stability", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.time_dynamics"}, "checkpoint-multi", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("multi-before", false), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "multi-dose-1"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"multi-dose-1-effect"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(2, "multi-dose-2"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 2, EvidenceRefs: []string{"multi-dose-2-effect"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(testObservation("multi-after", true), true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordTargetResponse(TargetEvaluation{Response: TargetDirectional, Outcome: trajectory.EvaluationAgentEvaluable, EvidenceRefs: []string{"multi-after-evidence"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.DecideRound(DecisionNextRound, "directional but not sufficient", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.time_dynamics", "track.transient"}, "checkpoint-round-2", "rev-2", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if len(turn.Rounds) != 2 || turn.Rounds[0].Decision != DecisionNextRound || turn.Rounds[1].Number != 2 {
		t.Fatalf("multi-round state=%+v", turn.Rounds)
	}
}

func TestExperimentBudgetBoundsDoseAttempts(t *testing.T) {
	admission := testAdmission(AuthorityFull)
	admission.ExperimentBudget = 1
	turn, err := NewTurn(Identity{ConversationID: "conversation-budget"}, "bounded experiment", admission, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.time_dynamics"}, "checkpoint-budget", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(1, "budget-1"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.ApplyIntervention(testIntervention(2, "budget-2"), time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Fatalf("budget error=%v", err)
	}
}

func TestOrdinaryAuthorityRequiresConfirmedIntervention(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-authority"}, "bounded change", testAdmission(AuthorityOrdinary), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.time_dynamics"}, "checkpoint-authority", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	intervention := testIntervention(1, "authority-action")
	intervention.PolicyAuthorized = false
	intervention.UserConfirmed = false
	if _, err := turn.ApplyIntervention(intervention, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "confirmation or bounded policy") {
		t.Fatalf("authority error=%v", err)
	}
	intervention.UserConfirmed = true
	if _, err := turn.ApplyIntervention(intervention, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}
