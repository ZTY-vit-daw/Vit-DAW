package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/trajectory"
)

// FIX-FS-SETTLE-DEADCHAIN-1 leg D: rerun2 showed the model reaching the settle
// boundary with semantically correct reports whose experiment_materiality shape
// was illegal (state="", attempt<=0), and the final-gate rejection fed back
// only the bare error — no legal shape — so the retries kept guessing blind
// until the continuation budget ran out. These tests pin that the rejection
// feedback carries the violated field's legal shape (bounded schema teaching:
// enum members and the positive-integer bound, nothing else).

// settleShapeTestOutput mirrors the run6 settle attempt: a final turn whose
// decision books the settle report on the preserved proposal, with only the
// materiality shape made illegal per case.
func settleShapeTestOutput(mutate func(*experiment.MaterialityEvaluation)) messageLoopOutput {
	materiality := &experiment.MaterialityEvaluation{
		Evaluation:   trajectory.EvaluationAmbiguous,
		EvidenceRefs: []string{"obs-target"},
		Summary:      "dose cannot be judged from the fresh post-action evidence",
		Attempt:      1,
	}
	if mutate != nil {
		mutate(materiality)
	}
	out := gateTestProposal(nil)
	out.Final = true
	out.FreeStateDecision.ExperimentMateriality = materiality
	out.FreeStateDecision.ExperimentTargetResponse = &experiment.TargetEvaluation{
		Response:     experiment.TargetAmbiguous,
		Outcome:      trajectory.EvaluationHumanAuditionReady,
		EvidenceRefs: []string{"obs-target"},
	}
	out.FreeStateDecision.ExperimentRoundDecision = string(experiment.DecisionUserJudgment)
	return out
}

func TestSettleRejectionTeachesMaterialityStateEnum(t *testing.T) {
	state := gateTestState(nil)
	out := settleShapeTestOutput(func(m *experiment.MaterialityEvaluation) { m.State = "" })
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "invalid free_state decision: experiment_materiality: unsupported materiality state") {
		t.Fatalf("state rejection did not surface through the final-gate wrap: %q", issue)
	}
	if !strings.Contains(issue, "state must be one of: none, subthreshold, material") {
		t.Fatalf("state rejection does not teach the legal state enum: %q", issue)
	}
}

func TestSettleRejectionTeachesMaterialityEvaluationEnum(t *testing.T) {
	state := gateTestState(nil)
	out := settleShapeTestOutput(func(m *experiment.MaterialityEvaluation) {
		m.State = experiment.MaterialitySubthreshold
		m.Evaluation = "definitely"
	})
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "evaluation must be one of: not_ready, insufficient_dose, agent_evaluable, ambiguous") {
		t.Fatalf("evaluation rejection does not teach the legal evaluation enum: %q", issue)
	}
}

func TestSettleRejectionTeachesMaterialityAttemptShape(t *testing.T) {
	state := gateTestState(nil)
	out := settleShapeTestOutput(func(m *experiment.MaterialityEvaluation) {
		m.State = experiment.MaterialitySubthreshold
		m.Attempt = 0
	})
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "materiality attempt must be a positive integer (attempt >= 1)") {
		t.Fatalf("attempt rejection does not teach the positive-integer shape: %q", issue)
	}
	// Scope control: the attempt teaching must stay bounded to the violated
	// field and not pad the message with the other fields' enums.
	if strings.Contains(issue, "state must be one of:") {
		t.Fatalf("attempt rejection leaked the state enum teaching: %q", issue)
	}
}
