package experiment

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

func testD2MultiRoundAdmission() Admission {
	a := testD1Admission(AuthorityFull)
	a.ExperimentBudget = 3
	a.BaselineFingerprint = map[string]any{"revision": "rev-base", "parameter": "fader_db", "before_value": -6.0}
	return a
}

func testD2Intervention(attempt int, id string, delta float64) Intervention {
	i := testIntervention(attempt, id)
	i.RequestedDelta = map[string]any{"delta_db": delta}
	i.AchievedDelta = map[string]any{"delta_db": delta}
	i.Receipt = map[string]any{"status": "readback_ok", "action_id": id, "after_revision": fmt.Sprintf("rev-after-%d", attempt)}
	return i
}

// advanceD2Round drives one calibrating round: one bounded forward mutation
// that stays subthreshold, then the next_round decision (GLM ruling 1: an
// insufficient-dose signal may continue into the next round).
func advanceD2Round(t *testing.T, turn *Turn, attempt int, id string, delta float64) {
	t.Helper()
	if _, err := turn.ApplyIntervention(testD2Intervention(attempt, id, delta), time.Now().UTC()); err != nil {
		t.Fatalf("round intervention %s: %v", id, err)
	}
	if _, err := turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: attempt, EvidenceRefs: []string{id + "-effect"}}, time.Now().UTC()); err != nil {
		t.Fatalf("round materiality %s: %v", id, err)
	}
	if _, err := turn.DecideRound(DecisionNextRound, "insufficient dose; calibrate in next round", time.Now().UTC()); err != nil {
		t.Fatalf("round decision %s: %v", id, err)
	}
}

// The tier trap, sealed: IsD1S1 stays a pure domain-membership test that
// never consults the budget, ValidateD1S1 keeps rejecting budget>1, and only
// the explicit D2-2 predicate/validator admit the multi-round tier.
func TestD2MultiRoundAdmissionTierSplit(t *testing.T) {
	a := testD2MultiRoundAdmission()
	if !a.IsD1S1() {
		t.Fatal("IsD1S1 must remain a domain-membership test: a D2-2 budget>1 admission still hits the domain guards")
	}
	if !a.IsD2MultiRound() {
		t.Fatal("D2-2 admission not recognized by the explicit tier predicate")
	}
	if a.isD1S1SingleRound() {
		t.Fatal("D2-2 admission must not run under the single-round guard set")
	}
	if err := a.ValidateD2MultiRound(); err != nil {
		t.Fatalf("valid D2-2 admission rejected: %v", err)
	}
	if err := a.ValidateD1S1(); err == nil || !strings.Contains(err.Error(), "budget must be 1") {
		t.Fatalf("ValidateD1S1 must keep rejecting budget>1, got %v", err)
	}
	single := a
	single.ExperimentBudget = 1
	if single.IsD2MultiRound() {
		t.Fatal("budget 1 must not opt into the multi-round tier")
	}
	if err := single.ValidateD2MultiRound(); err == nil {
		t.Fatal("ValidateD2MultiRound must reject budget 1")
	}
	if !single.isD1S1SingleRound() {
		t.Fatal("budget 1 domain member must keep the single-round guards")
	}
	noBaseline := a
	noBaseline.BaselineFingerprint = nil
	if err := noBaseline.ValidateD2MultiRound(); err == nil || !strings.Contains(err.Error(), "baseline fingerprint") {
		t.Fatalf("D2-2 admission without a baseline fingerprint must be rejected, got %v", err)
	}
}

func TestD2MultiRoundAdmissionBudgetBounds(t *testing.T) {
	atBound := testD2MultiRoundAdmission()
	atBound.ExperimentBudget = MaxD2MultiRoundBudget
	if err := atBound.ValidateD2MultiRound(); err != nil {
		t.Fatalf("budget at the sealed upper bound rejected: %v", err)
	}
	aboveBound := testD2MultiRoundAdmission()
	aboveBound.ExperimentBudget = MaxD2MultiRoundBudget + 1
	if aboveBound.IsD2MultiRound() {
		t.Fatal("budget above the sealed upper bound must not opt into the multi-round tier")
	}
	if err := aboveBound.ValidateD2MultiRound(); err == nil {
		t.Fatal("ValidateD2MultiRound must reject a budget above the sealed upper bound")
	}
	// An unknown tier fails closed: it keeps the D1-S1 single-round guards,
	// whose validator still rejects the out-of-tier budget.
	if !aboveBound.isD1S1SingleRound() {
		t.Fatal("an out-of-tier budget must fall back to the single-round guard set")
	}
}

// D2-2 opens exactly the loop boundary: a second round and cross-round forward
// mutations run, while each round still admits one mutation and the budget
// stays the cross-round cap (InterventionCount semantics confirmed).
func TestD2MultiRoundAdmitsSecondRoundAndCrossRoundMutations(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d2-multi"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	advanceD2Round(t, &turn, 1, "d2-forward-1", 1.5)
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
		t.Fatalf("second round rejected under the D2-2 tier: %v", err)
	}
	if _, err = turn.ApplyIntervention(testD2Intervention(1, "d2-forward-2", 0.4), time.Now().UTC()); err != nil {
		t.Fatalf("cross-round forward mutation rejected under the D2-2 tier: %v", err)
	}
	if turn.InterventionCount() != 2 || len(turn.Rounds) != 2 {
		t.Fatalf("InterventionCount=%d rounds=%d, want the budget to accumulate across rounds", turn.InterventionCount(), len(turn.Rounds))
	}
	if _, err = turn.ApplyIntervention(testD2Intervention(2, "d2-same-round", 0.1), time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "D2-2 permits one forward mutation per round") {
		t.Fatalf("second same-round mutation error=%v", err)
	}
	if turn.InterventionCount() != 2 {
		t.Fatalf("rejected mutation changed the count to %d", turn.InterventionCount())
	}
}

func TestD2MultiRoundBudgetExhaustedRejectsWithoutEvents(t *testing.T) {
	admission := testD2MultiRoundAdmission()
	admission.ExperimentBudget = 2
	turn, err := NewTurn(Identity{ConversationID: "conversation-d2-budget"}, "bounded multi-round experiment", admission, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	advanceD2Round(t, &turn, 1, "d2-budget-1", 0.5)
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	advanceD2Round(t, &turn, 1, "d2-budget-2", 0.5)
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-3", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	events, err := turn.ApplyIntervention(testD2Intervention(1, "d2-over-budget", 0.5), time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "budget exhausted") {
		t.Fatalf("over-budget mutation error=%v", err)
	}
	if len(events) != 0 {
		t.Fatalf("over-budget mutation emitted events: %+v", events)
	}
	if turn.InterventionCount() != 2 {
		t.Fatalf("over-budget mutation changed the count to %d", turn.InterventionCount())
	}
}

// GLM ruling 2, sealed with the adversarial case: every round stays inside
// the single-action bound yet the experiment-lifetime cumulative displacement
// must not exceed the domain's absolute bound.
func TestD2MultiRoundCumulativeDisplacementBound(t *testing.T) {
	t.Run("each round legal but cumulative over the bound", func(t *testing.T) {
		turn, err := NewTurn(Identity{ConversationID: "conversation-d2-cumulative"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		advanceD2Round(t, &turn, 1, "d2-cum-1", 1.5)
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		events, err := turn.ApplyIntervention(testD2Intervention(1, "d2-cum-2", 1.0), time.Now().UTC())
		if err == nil || !strings.Contains(err.Error(), "cumulative") {
			t.Fatalf("cumulative overrun error=%v", err)
		}
		if len(events) != 0 {
			t.Fatalf("cumulative overrun emitted events: %+v", events)
		}
		if turn.InterventionCount() != 1 {
			t.Fatalf("rejected cumulative overrun changed the count to %d", turn.InterventionCount())
		}
	})
	t.Run("signed accumulation lets reciprocal moves cancel", func(t *testing.T) {
		turn, err := NewTurn(Identity{ConversationID: "conversation-d2-reciprocal"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		advanceD2Round(t, &turn, 1, "d2-up", 1.5)
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.ApplyIntervention(testD2Intervention(1, "d2-down", -1.5), time.Now().UTC()); err != nil {
			t.Fatalf("reciprocal move rejected: %v", err)
		}
	})
	t.Run("boundary value stays legal and the next step is rejected", func(t *testing.T) {
		turn, err := NewTurn(Identity{ConversationID: "conversation-d2-boundary"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		advanceD2Round(t, &turn, 1, "d2-boundary-1", 1.0)
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		advanceD2Round(t, &turn, 1, "d2-boundary-2", 1.0)
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-3", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.ApplyIntervention(testD2Intervention(1, "d2-boundary-3", 0.1), time.Now().UTC()); err == nil || !strings.Contains(err.Error(), "cumulative") {
			t.Fatalf("displacement past the lifetime bound error=%v", err)
		}
	})
	t.Run("failed attempts spend budget but record no displacement", func(t *testing.T) {
		turn, err := NewTurn(Identity{ConversationID: "conversation-d2-failed"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		advanceD2Round(t, &turn, 1, "d2-failed-base", 1.5)
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		failed := testIntervention(1, "d2-failed-attempt")
		failed.TechnicalApplication = TechnicalFailed
		failed.AchievedDelta = nil
		if _, err = turn.ApplyIntervention(failed, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"d2-failed-effect"}}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.DecideRound(DecisionNextRound, "failed application; recalibrate", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-3", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		if _, err = turn.ApplyIntervention(testD2Intervention(1, "d2-after-failed", 0.5), time.Now().UTC()); err != nil {
			t.Fatalf("applied move after a failed attempt rejected: %v", err)
		}
	})
	t.Run("applied intervention without a displacement record is rejected", func(t *testing.T) {
		turn, err := NewTurn(Identity{ConversationID: "conversation-d2-nodelta"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		bare := testIntervention(1, "d2-nodelta")
		bare.AchievedDelta = nil
		events, err := turn.ApplyIntervention(bare, time.Now().UTC())
		if err == nil || !strings.Contains(err.Error(), "cumulative dose accounting") {
			t.Fatalf("applied intervention without displacement error=%v", err)
		}
		if len(events) != 0 {
			t.Fatalf("rejected intervention emitted events: %+v", events)
		}
	})
}

// The cumulative bound equals the domain-table single-action absolute bound by
// ruling; this seals the equality against the live table so the two can never
// drift apart silently.
func TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound(t *testing.T) {
	for _, domain := range D1S1DomainSpecs() {
		t.Run(domain.ActionDomain, func(t *testing.T) {
			key := domain.AdmissionValueKey
			for _, delta := range []float64{-d2MultiRoundCumulativeDeltaBoundDB, d2MultiRoundCumulativeDeltaBoundDB} {
				if err := domain.ValidateDoseBounds("cumulative-probe", map[string]any{key: delta}); err != nil {
					t.Fatalf("domain absolute bound no longer matches the cumulative bound %.1g dB: %v", delta, err)
				}
			}
			for _, delta := range []float64{0, -d2MultiRoundCumulativeDeltaBoundDB - 0.1, d2MultiRoundCumulativeDeltaBoundDB + 0.1} {
				if err := domain.ValidateDoseBounds("cumulative-probe", map[string]any{key: delta}); err == nil {
					t.Fatalf("domain absolute bound accepts %.1g dB; the sealed equality is broken", delta)
				}
			}
		})
	}
}

// GLM ruling 3, runtime layer: a requested-but-unlanded user judgment parks
// the whole experiment. Rounds, forward mutations, and non-settle decisions
// are rejected with the durable error; the settle family stays reachable and
// a landed judgment reopens the loop.
func TestD2MultiRoundJudgmentBoundaryPersistsAcrossRounds(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d2-judgment"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before := testObservation("judgment-before", false)
	before.ProjectRevision = "rev-1"
	if _, err = turn.RecordObservation(before, false, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	forward := testD2Intervention(1, "d2-judgment-1", 0.5)
	if _, err = turn.ApplyIntervention(forward, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.EvaluateMateriality(MaterialityEvaluation{State: MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"d2-judgment-effect"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	after := testObservation("judgment-after", true)
	after.ProjectRevision = "rev-after-1"
	if _, err = turn.RecordObservation(after, true, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordTargetResponse(TargetEvaluation{Response: TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"judgment-after-evidence"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RequestUserJudgmentForSession("human A/B judgment required", "audition-d2-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if !turn.experimentJudgmentPending() {
		t.Fatal("a requested-but-unlanded judgment must park the experiment")
	}
	if _, err := turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err == nil || !errors.Is(err, ErrJudgmentPending) {
		t.Fatalf("round after pending judgment error=%v", err)
	}
	// In the natural flow the requesting round already spent its single
	// mutation, so the per-round guard fires before the boundary check; both
	// reject, i.e. no mutation path exists while the judgment pends.
	if _, err := turn.ApplyIntervention(testD2Intervention(2, "d2-judgment-2", 0.5), time.Now().UTC()); err == nil {
		t.Fatal("mutation after pending judgment accepted")
	}
	for _, decision := range []RoundDecision{DecisionNextRound, DecisionUserJudgment, DecisionPlateau, DecisionBlockedObservation, DecisionBlockedCapability} {
		if _, err := turn.DecideRound(decision, "revival attempt", time.Now().UTC()); err == nil || !errors.Is(err, ErrJudgmentPending) {
			t.Fatalf("decision %s after pending judgment error=%v", decision, err)
		}
	}
	round, err := turn.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	landed := UserJudgmentEvidence{
		SchemaVersion: UserJudgmentEvidenceSchemaVersion, ID: "judgment-d2-1",
		ConversationID: turn.ConversationID, TurnID: turn.ID, RoundID: round.ID,
		AuditionSessionID: "audition-d2-1",
		CandidateARef:     "candidate-a", CandidateBRef: "candidate-b",
		HeardDifference: HeardDifferenceNo, Preference: PreferenceEqual,
		CreatedAt: time.Now().UTC(),
	}
	if _, err = turn.RecordUserJudgmentEvidence(landed, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// The judgment landed (no audible difference = an insufficient-effect
	// signal, ruling 1), so the loop may calibrate into the next round.
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-2", time.Now().UTC()); err != nil {
		t.Fatalf("round after a landed judgment rejected: %v", err)
	}
	if _, err = turn.ApplyIntervention(testD2Intervention(1, "d2-judgment-round2", 0.3), time.Now().UTC()); err != nil {
		t.Fatalf("mutation after a landed judgment rejected: %v", err)
	}
	if turn.InterventionCount() != 2 {
		t.Fatalf("InterventionCount=%d after the reopened round", turn.InterventionCount())
	}
}

// Budget-exhausted settlement contract: a turn whose budget ran dry settles
// only onto the canonically capability-blocked task projection.
func TestBudgetExhaustedSettlementRequiresCapabilityBlocked(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d2-settle"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = turn.BindTaskState("contract-d2", taskstate.StateSettled, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	advanceD2Round(t, &turn, 1, "d2-settle-1", 1.5)
	if err := turn.BindTaskState("contract-d2", taskstate.StateCapabilityBlocked, 2); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.Settle(OutcomeBudgetExhausted, "budget spent without a sufficient response", time.Now().UTC()); err != nil {
		t.Fatalf("budget-exhausted settlement rejected: %v", err)
	}
	if turn.Outcome != OutcomeBudgetExhausted || turn.Status != StatusSettled {
		t.Fatalf("settlement state=%+v", turn)
	}
}

func TestBudgetExhaustedSettlementRejectsNonBlockedCanonicalState(t *testing.T) {
	turn, err := NewTurn(Identity{ConversationID: "conversation-d2-settle-mismatch"}, "bounded multi-round experiment", testD2MultiRoundAdmission(), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = turn.BindTaskState("contract-d2-mismatch", taskstate.StateSettled, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"mix.multitrack_relationship"}, "checkpoint-d2", "rev-1", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	advanceD2Round(t, &turn, 1, "d2-settle-mismatch", 0.5)
	if _, err = turn.Settle(OutcomeBudgetExhausted, "budget spent", time.Now().UTC()); err == nil || !strings.Contains(err.Error(), string(taskstate.StateCapabilityBlocked)) {
		t.Fatalf("budget-exhausted settlement without the canonical capability_blocked state error=%v", err)
	}
}
