package experiment

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/taskstate"
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

func testD1Admission(mode AuthorityMode) Admission {
	a := testAdmission(mode)
	a.TargetRef = map[string]any{"kind": "track", "id": "track-1", "source": "fresh_g1_g7_observation"}
	a.TypedAction = map[string]any{
		"action_domain": "track_gain",
		"action_kind":   "track_gain_adjust",
		"target_db":     -1.0,
	}
	a.DiagnosticDoseBounds = map[string]any{"delta_db": -1.0, "max_action_attempts": 1}
	a.RetainedDoseBounds = map[string]any{"delta_db": -1.0, "max_action_attempts": 1}
	a.ExperimentBudget = 1
	return a
}

func TestD1S1AdmissionRequiresSingleBoundedTrackGainAction(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Admission)
	}{
		{"wrong domain", func(a *Admission) { a.TypedAction["action_domain"] = "eq" }},
		{"wrong action", func(a *Admission) { a.TypedAction["action_kind"] = "track_pan_adjust" }},
		{"budget above one", func(a *Admission) { a.ExperimentBudget = 2 }},
		{"attempts above one", func(a *Admission) { a.DiagnosticDoseBounds["max_action_attempts"] = 2 }},
		{"unbounded delta", func(a *Admission) { a.DiagnosticDoseBounds["delta_db"] = -3.0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := testD1Admission(AuthorityFull)
			test.mutate(&a)
			if err := a.ValidateD1S1(); err == nil {
				t.Fatal("invalid D1-S1 admission accepted")
			}
		})
	}
	if err := testD1Admission(AuthorityFull).ValidateD1S1(); err != nil {
		t.Fatalf("valid D1-S1 admission rejected: %v", err)
	}
}

func TestD1S1RoundAllowsOneForwardMutationAndNoSecondRound(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d1"}, "bounded gain experiment", testD1Admission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d1", "7", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.ApplyIntervention(testIntervention(1, "d1-forward"), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.ApplyIntervention(testIntervention(2, "d1-second"), time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "one forward mutation") {
		t.Fatalf("second forward mutation error=%v", err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d1", "8", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "one round") {
		t.Fatalf("second round error=%v", err)
	}
}

func TestD1S1PostActionObservationMustBeFreshAndMatchAfterRevision(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d1-revision"}, "bounded gain experiment", testD1Admission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d1", "7", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before := testObservation("d1-before", false)
	before.ProjectRevision = "7"
	if _, err = turn.RecordObservation(before, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	action := testIntervention(1, "d1-forward")
	action.Receipt["after_revision"] = "8"
	if _, err = turn.ApplyIntervention(action, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	after := testObservation("d1-after", true)
	after.ProjectRevision = "7"
	if _, err = turn.RecordObservation(after, true, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "match mutation after revision") {
		t.Fatalf("stale post-action observation error=%v", err)
	}
	after.ProjectRevision = "8"
	after.Fresh = false
	if _, err = turn.RecordObservation(after, true, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "fresh") {
		t.Fatalf("non-fresh post-action observation error=%v", err)
	}
	after.Fresh = true
	if _, err = turn.RecordObservation(after, true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestD1S1SubthresholdAllowsOnlyAmbiguousHumanAudition(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "d1-subthreshold"}, "compare one bounded gain move", testD1Admission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d1", "7", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before := testObservation("d1-sub-before", false)
	before.ProjectRevision = "7"
	if _, err = turn.RecordObservation(before, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	action := testIntervention(1, "d1-sub-action")
	action.Receipt["after_revision"] = "8"
	if _, err = turn.ApplyIntervention(action, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	after := testObservation("d1-sub-after", true)
	after.ProjectRevision = "8"
	if _, err = turn.RecordObservation(after, true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"subthreshold"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordTargetResponse(TargetEvaluation{Response: TargetDirectional, Outcome: trajectory.EvaluationAgentEvaluable, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err == nil {
		t.Fatal("subthreshold D1 result accepted a directional target claim")
	}
	if _, err = turn.RecordTargetResponse(TargetEvaluation{Response: TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
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
	decisionEvents, err := turn.DecideRound(DecisionNextRound, "not enough dose", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(decisionEvents) != 1 || decisionEvents[0].Type != trajectory.EventRoundDecision || decisionEvents[0].Payload.NextDecision != string(DecisionNextRound) {
		t.Fatalf("next round decision=%+v", decisionEvents)
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

// A round admits exactly one post-action CCB bundle. A second bundle can only
// arrive from a loop that revived after the human-judgment boundary
// (2026-08-25 21:09 D1 smoke: two post_action=true observations broke
// validate_d1). The runtime rejects it defensively so the caller's Warn log
// sees the duplicate instead of silently appending it.
func TestRecordObservationRejectsSecondPostActionObservation(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-dual-post"}, "evaluate the applied change", testD1Admission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-dual", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	bound := func(id string, postAction bool, revision string) Observation {
		observation := testObservation(id, postAction)
		observation.ProjectRevision = revision
		return observation
	}
	if _, err := turn.RecordObservation(bound("before-dual", false, "rev-1"), false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	intervention := testIntervention(1, "dual-dose")
	intervention.Receipt = map[string]any{"status": "readback_ok", "action_id": "dual-dose", "after_revision": "rev-2", "applied_revision": "rev-2"}
	if _, err := turn.ApplyIntervention(intervention, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(bound("after-dual-1", true, "rev-2"), true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := turn.RecordObservation(bound("after-dual-2", true, "rev-2"), true, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "post-action observation already recorded") {
		t.Fatalf("second post-action observation was accepted: %v", err)
	}
	round, err := turn.currentRound()
	if err != nil {
		t.Fatal(err)
	}
	postActionCount := 0
	for _, observation := range round.Observations {
		if observation.PostAction {
			postActionCount++
		}
	}
	if postActionCount != 1 || len(round.Observations) != 2 {
		t.Fatalf("round observations = %+v, want exactly one post-action bundle", round.Observations)
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

// A contract-bound turn may claim the needs_user_judgment outcome only after
// the canonical task terminally settled with the human evidence (the D1
// ambiguous judgment); any other canonical state must keep requesting the
// judgment instead of settling.
func TestSettleNeedsJudgmentRequiresCanonicallySettledTask(t *testing.T) {
	settled := func() *Turn {
		turn, err := NewTurn(Identity{ConversationID: "conversation-needs-judgment-settled"}, "ambiguous judgment", testAdmission(AuthorityFull), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-ambiguous", "rev-ambiguous", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if err := turn.BindTaskState("contract-ambiguous", taskstate.StateSettled, 3); err != nil {
			t.Fatal(err)
		}
		return &turn
	}()
	if _, err := settled.Settle(OutcomeNeedsJudgment, "ambiguous audition judgment", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	open := func() *Turn {
		turn, err := NewTurn(Identity{ConversationID: "conversation-needs-judgment-open"}, "ambiguous judgment", testAdmission(AuthorityFull), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-open", "rev-open", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if err := turn.BindTaskState("contract-open", taskstate.StateNeedsExperiment, 2); err != nil {
			t.Fatal(err)
		}
		return &turn
	}()
	if _, err := open.Settle(OutcomeNeedsJudgment, "premature settlement", time.Now().UTC()); err == nil {
		t.Fatal("needs_user_judgment settled without a canonically settled task")
	}
}

func TestSettlementBindsCurrentCheckpointAndProjectRevision(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-settle"}, "settle with revision", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-final", "revision-final", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events, err := turn.Settle(OutcomeStable, "stable", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Payload.CheckpointRef != "checkpoint-final" || events[0].Payload.ProjectRevision != "revision-final" {
		t.Fatalf("settlement=%+v", events)
	}
}

func TestStoppedTurnRejectsSubsequentActivity(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-stopped"}, "stop safely", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-stop", "revision-stop", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.Stop("user stopped", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-late", "revision-late", time.Now().UTC()); err == nil {
		t.Fatal("stopped turn accepted a later round")
	}
	if _, err = turn.Settle(OutcomeStable, "late settlement", time.Now().UTC()); err == nil {
		t.Fatal("stopped turn accepted a later settlement")
	}
}

func auditionReadyTurn(t *testing.T) Turn {
	t.Helper()
	now := time.Now().UTC()
	turn, err := NewTurn(Identity{ConversationID: "conversation-judgment", TurnID: "turn-judgment"}, "compare treatment", testAdmission(AuthorityFull), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-judgment", "rev-1", now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordObservation(testObservation("judgment-before", false), false, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.ApplyIntervention(testIntervention(1, "judgment-action"), now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"judgment-material"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordObservation(testObservation("judgment-after", true), true, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordTargetResponse(TargetEvaluation{Response: TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"judgment-after"}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RequestUserJudgmentForSession("compare A/B", "audition:turn-judgment:round", now); err != nil {
		t.Fatal(err)
	}
	return turn
}

func TestUserJudgmentEvidenceIsSessionBoundAndImmutable(t *testing.T) {
	now := time.Now().UTC()
	turn := auditionReadyTurn(t)
	round, _ := turn.currentRound()
	evidence := UserJudgmentEvidence{
		SchemaVersion: UserJudgmentEvidenceSchemaVersion, ID: "judgment-1",
		ConversationID: turn.ConversationID, TurnID: turn.ID, RoundID: round.ID, AuditionSessionID: round.AuditionSessionID,
		CandidateARef: "checkpoint:before", CandidateBRef: "action:treatment",
		HeardDifference: HeardDifferenceYes, Preference: PreferenceB, ReasonTags: []string{"更自然"}, CreatedAt: now,
	}
	events, err := turn.RecordUserJudgmentEvidence(evidence, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != trajectory.EventUserJudgmentRecorded || events[0].Payload.Outcome != trajectory.EvaluationHumanConfirmed {
		t.Fatalf("events=%+v", events)
	}
	stored, _ := turn.currentRound()
	if len(stored.UserJudgmentEvidence) != 1 || stored.UserJudgmentEvidence[0].ID != "judgment-1" || stored.TargetResponse.Outcome != trajectory.EvaluationHumanConfirmed {
		t.Fatalf("round=%+v", stored)
	}
	if _, err := turn.RecordUserJudgmentEvidence(evidence, now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "corrections") {
		t.Fatalf("duplicate evidence error=%v", err)
	}
	correction := evidence
	correction.ID = "judgment-2"
	correction.SupersedesID = evidence.ID
	correction.Preference = PreferenceEqual
	correction.HeardDifference = HeardDifferenceYes
	// The correction path is intentionally allowed only as a new immutable row.
	if _, err := turn.RecordUserJudgmentEvidence(correction, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	stored, _ = turn.currentRound()
	if len(stored.UserJudgmentEvidence) != 2 || stored.UserJudgmentEvidence[0].Preference != PreferenceB || stored.UserJudgmentEvidence[1].SupersedesID != "judgment-1" {
		t.Fatalf("correction history=%+v", stored.UserJudgmentEvidence)
	}
}

func TestUserJudgmentEvidenceRejectsCandidatePreferenceWithoutHeardDifference(t *testing.T) {
	turn := auditionReadyTurn(t)
	round, _ := turn.currentRound()
	evidence := UserJudgmentEvidence{SchemaVersion: UserJudgmentEvidenceSchemaVersion, ID: "invalid-judgment", ConversationID: turn.ConversationID, TurnID: turn.ID, RoundID: round.ID, AuditionSessionID: round.AuditionSessionID, CandidateARef: "a", CandidateBRef: "b", HeardDifference: HeardDifferenceNo, Preference: PreferenceA, CreatedAt: time.Now().UTC()}
	if _, err := turn.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "requires heard_difference") {
		t.Fatalf("error=%v", err)
	}
}

func TestRequestUserJudgmentRequiresHumanAuditionReadyAndExactSession(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-request", TurnID: "turn-request"}, "compare", testAdmission(AuthorityFull), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-request", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RequestUserJudgmentForSession("too early", "session-1", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "human_audition_ready") {
		t.Fatalf("error=%v", err)
	}
	if _, err = turn.RequestUserJudgment("legacy unbound", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "audition_session_id") {
		t.Fatalf("legacy error=%v", err)
	}
}
