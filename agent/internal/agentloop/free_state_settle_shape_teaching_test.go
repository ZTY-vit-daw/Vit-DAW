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

// FIX-FS-SETTLE-DEADCHAIN-1 leg E: run9 whack-a-mole — with short-circuit
// rejection the model fixed the one named field per bounce and dropped the
// others (state=""↔attempt=0 alternating; evidence_refs also missing on
// retries two and four) until the budget ran out. These tests pin that one
// rejection lists every violated field at once and carries the complete
// zero-truth materiality skeleton.

func TestSettleRejectionListsAllViolatedFieldsAtOnce(t *testing.T) {
	state := gateTestState(nil)
	// run9's composite retry shape: illegal state, zero attempt, and the
	// evidence_refs the retries kept dropping are all missing together.
	out := settleShapeTestOutput(func(m *experiment.MaterialityEvaluation) {
		m.State = ""
		m.Attempt = 0
		m.EvidenceRefs = nil
	})
	issue := messageLoopFreeStateOutputIssue(state, out)
	for _, want := range []string{
		"unsupported materiality state",
		"materiality attempt must be a positive integer (attempt >= 1), got 0",
		"materiality evidence_refs are required",
	} {
		if !strings.Contains(issue, want) {
			t.Fatalf("combined rejection is missing a violated field: %q (want %q)", issue, want)
		}
	}
}

func TestSettleRejectionCarriesCompleteMaterialitySkeleton(t *testing.T) {
	state := gateTestState(nil)
	out := settleShapeTestOutput(func(m *experiment.MaterialityEvaluation) { m.State = "" })
	issue := messageLoopFreeStateOutputIssue(state, out)
	if !strings.Contains(issue, "complete object shape:") {
		t.Fatalf("rejection does not carry the complete object skeleton: %q", issue)
	}
	// All four required fields must appear in the skeleton, zero truth values:
	// enum unions and placeholders only, no fixture or track identifiers.
	for _, want := range []string{
		`"state":"material|subthreshold|none"`,
		`"evaluation":"not_ready|insufficient_dose|agent_evaluable|ambiguous"`,
		`"evidence_refs":["<fresh post-action observation id>"]`,
		`"attempt":1`,
	} {
		if !strings.Contains(issue, want) {
			t.Fatalf("rejection skeleton is missing a required field shape: %q (want %q)", issue, want)
		}
	}
}
