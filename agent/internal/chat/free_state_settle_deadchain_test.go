package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

// FIX-FS-SETTLE-DEADCHAIN-1 (2026-10-13 runs 20261013_231930/232817): the
// hallucinated-settle dead chain. deepseek's final proposal turn carried
// experiment_materiality={state:none, evaluation:not_ready} together with
// final=true (the model narrated the application as done; the project held
// zero plugins). The runtime read field presence as a settle attempt,
// refused it, and the refusal marker swallowed the admitted proposal's
// response path — the loop parked at waiting_continue with no interaction
// and no continuation, the owed withhold armed nothing, interventions stayed
// 0 and the D1 gate failed with "exactly one forward mutation". The
// predicate refinement (B-i): the pure empty-state pair on a round that
// never acted is schema noise, not a settlement claim — see
// freeStateVacuousSettleClaimOnUnactedRound. These tests pin both halves of
// the contract: the dead chain stays dead no longer (the proposal surface
// survives), and every genuine settle-attempt shape keeps its refusal.

// vacuousSettleTurn builds the hallucinated turn's envelope: the preserved
// proposal riding a needs_experiment decision, the empty-state materiality
// as the only settle signal, and the final=true completion the runner
// delivered with no continuation.
func vacuousSettleTurn(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-deadchain",
		SliceID: "slice-deadchain", TurnID: "turn-deadchain",
		Status:     agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:              agentloop.FreeStateNeedsExperiment,
			Summary:             "admitted proposal with a hallucinated settle payload",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialityNone, Evaluation: trajectory.EvaluationNotReady,
			},
		},
	}
}

// The counterexample chain must not die: the vacuous materiality arms no
// refusal marker, the hallucinated settle never lands on the round, and the
// proposal still reaches its confirmation surface — the driver-answerable
// path that carries the admitted experiment to its intervention.
func TestVacuousSettleClaimKeepsAdmittedChainAlive(t *testing.T) {
	s := New(nil, nil, nil)
	loop := d1LoopForTest(t, "7")
	s.storeFreeStateLoop(loop)

	res := vacuousSettleTurn(loop)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("hallucinated-settle turn was not retained by recordFreeStateDecision")
	}
	if stored.SettleRefusedRoundID != "" {
		t.Fatalf("vacuous settle claim armed the refusal marker: %q", stored.SettleRefusedRoundID)
	}
	if freeStateLoopRoundSettleRefused(stored) {
		t.Fatal("vacuous settle claim must not open the settle-refused race window")
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Materiality != nil {
		t.Fatalf("hallucinated settle landed on the round: %+v", round.Materiality)
	}
	if stored.Status != "awaiting_experiment" {
		t.Fatalf("admitted loop left the experiment boundary: %q", stored.Status)
	}

	if stored.LatestDecision != nil {
		res.FreeStateDecision = stored.LatestDecision
	}
	resp := s.improvementProposalResponse(loop.ConversationID, "default", res, map[string]any{})
	if !resp.NeedsConfirmation || len(resp.InteractionRequests) == 0 {
		t.Fatalf("admitted proposal lost its confirmation surface: needs=%v requests=%d status=%q stop=%q",
			resp.NeedsConfirmation, len(resp.InteractionRequests), resp.GoalStatus, resp.StopReason)
	}
	kind := ""
	if len(resp.InteractionRequests) > 0 {
		kind = resp.InteractionRequests[0].Kind
	}
	if kind != "improvement_proposal_confirmation" {
		t.Fatalf("unexpected confirmation kind: %q", kind)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("proposal surface answered %q, want waiting_confirmation", resp.GoalStatus)
	}
}

// The exemption must not leak past the unacted round: once the round carries
// its intervention, the empty-state payload again counts as a settle attempt
// and the report is refused until the fresh post-action observation records
// — the observation precondition is the evidence gate EvaluateMateriality
// itself never checks.
func TestVacuousSettleClaimStillCountsAfterRoundActed(t *testing.T) {
	s := New(nil, nil, nil)
	loop := d1LoopForTest(t, "7")
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{
		ID: "d1-action-1", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied,
		UserConfirmed: true, Receipt: map[string]any{"after_revision": "8"}, EvidenceRefs: []string{"obs-before"},
	}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(loop)

	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialityNone, Evaluation: trajectory.EvaluationNotReady,
		},
	})
	if loop.SettleRefusedRoundID != round.ID {
		t.Fatalf("acted round's vacuous settle was not refused: marker=%q round=%s", loop.SettleRefusedRoundID, round.ID)
	}
	if round, err = loop.Experiment.CurrentRound(); err == nil && round.Materiality != nil {
		t.Fatalf("vacuous settle landed on the acted round: %+v", round.Materiality)
	}
}

// Genuine settle-attempt shapes keep the refusal on the unacted D1 round:
// any non-empty materiality state, a target response, or a round decision
// still counts as a settlement claim and still waits for the observation
// that cannot exist yet — the pre-fix semantics, unchanged.
func TestRealSettleReportStillRefusedOnUnactedD1Round(t *testing.T) {
	s := New(nil, nil, nil)
	material := agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable,
			Attempt: 1, EvidenceRefs: []string{"obs-before"},
		},
	}
	targetOnly := agentloop.FreeStateDecision{
		ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"obs-before"}},
	}
	decisionOnly := agentloop.FreeStateDecision{
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	}
	for name, decision := range map[string]agentloop.FreeStateDecision{
		"material-materiality": material,
		"target-response-only": targetOnly,
		"round-decision-only":  decisionOnly,
	} {
		t.Run(name, func(t *testing.T) {
			loop := d1LoopForTest(t, "7")
			round, err := loop.Experiment.CurrentRound()
			if err != nil {
				t.Fatal(err)
			}
			s.recordFreeStateExperimentDecision(context.Background(), &loop, decision)
			if loop.SettleRefusedRoundID != round.ID {
				t.Fatalf("genuine settle report was not refused on the unacted round: marker=%q round=%s", loop.SettleRefusedRoundID, round.ID)
			}
			if round, err := loop.Experiment.CurrentRound(); err == nil && round.Materiality != nil {
				t.Fatalf("refused report still landed materiality: %+v", round.Materiality)
			}
		})
	}
}

// The predicate's shape contract, pinned directly: only the pure empty-state
// pair, alone, on a round with zero booked interventions qualifies.
func TestFreeStateVacuousSettleClaimOnUnactedRoundMatrix(t *testing.T) {
	vacuous := &experiment.MaterialityEvaluation{State: experiment.MaterialityNone, Evaluation: trajectory.EvaluationNotReady}
	unacted := func(t *testing.T) *experiment.Turn {
		t.Helper()
		return d1LoopForTest(t, "7").Experiment
	}
	acted := func(t *testing.T) *experiment.Turn {
		t.Helper()
		exp := d1LoopForTest(t, "7").Experiment
		if _, err := exp.ApplyIntervention(experiment.Intervention{
			ID: "d1-action-1", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied,
			UserConfirmed: true, Receipt: map[string]any{"after_revision": "8"}, EvidenceRefs: []string{"obs-before"},
		}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		return exp
	}
	cases := []struct {
		name     string
		decision agentloop.FreeStateDecision
		exp      func(t *testing.T) *experiment.Turn
		want     bool
	}{
		{name: "pure vacuous pair on unacted round", decision: agentloop.FreeStateDecision{ExperimentMateriality: vacuous}, exp: unacted, want: true},
		{name: "vacuous pair plus round decision", decision: agentloop.FreeStateDecision{ExperimentMateriality: vacuous, ExperimentRoundDecision: string(experiment.DecisionUserJudgment)}, exp: unacted, want: false},
		{name: "vacuous pair plus target response", decision: agentloop.FreeStateDecision{ExperimentMateriality: vacuous, ExperimentTargetResponse: &experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, EvidenceRefs: []string{"obs"}}}, exp: unacted, want: false},
		{name: "material state is never vacuous", decision: agentloop.FreeStateDecision{ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"obs"}}}, exp: unacted, want: false},
		{name: "none state with a real evaluation is not vacuous", decision: agentloop.FreeStateDecision{ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialityNone, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"obs"}}}, exp: unacted, want: false},
		{name: "acted round is never exempt", decision: agentloop.FreeStateDecision{ExperimentMateriality: vacuous}, exp: acted, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := freeStateVacuousSettleClaimOnUnactedRound(test.exp(t), test.decision); got != test.want {
				t.Fatalf("freeStateVacuousSettleClaimOnUnactedRound=%v, want %v", got, test.want)
			}
		})
	}
	if freeStateVacuousSettleClaimOnUnactedRound(nil, agentloop.FreeStateDecision{ExperimentMateriality: vacuous}) {
		t.Fatal("nil experiment must not qualify for the exemption")
	}
}
