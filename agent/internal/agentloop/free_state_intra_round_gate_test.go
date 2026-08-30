package agentloop

import (
	"strings"
	"testing"
)

// D2-2-S3h6: a proposal-only needs_experiment on a running D2-2 multi-round
// experiment is an intra-round proposal of an already-admitted experiment (the
// owed recalibration round's bounded intervention), not a new admission. G1's
// contract-vs-closure revision equality is structurally unsatisfiable there:
// the contract revision is frozen at the original admission while the closure
// revision legitimately advances with each applied intervention
// (20260830_085624 trace: contract rev 2 vs closure rev 4 after round 1, every
// round-2 proposal refused at G1_project_binding, and the refusal's
// needs_observation direction collided with the anti-duplicate observation
// gate and the owed-round settle refusal until the continuation budget burnt
// out with zero round-2 interventions). The gate boundary mirrors the
// settle-report exemption; the round-level boundaries stay enforced.

// intraRoundGateTestState mirrors the forensic scene: the task contract frozen
// at the admission revision (2), the closure advanced to the round-1 after
// revision (4), a running multi-round experiment whose round 1 decided
// next_round and whose freshly opened round 2 carries its fresh non-post-action
// base observation and owes its single bounded intervention.
func intraRoundGateTestState(mutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "proj-1", "project_revision": "2",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version":      "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_phase": "fs8_experiment_verification",
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
			"experiment": map[string]any{
				"schema_version": "experiment.turn.v1", "status": "running",
				"admission": map[string]any{"experiment_budget": float64(2)},
				"rounds": []any{
					map[string]any{
						"round_number": float64(1), "decision": "next_round",
						"interventions": []any{map[string]any{"id": "act-round-1"}},
						"observations": []any{map[string]any{
							"observation_id": "obs-round-1-after", "post_action": true, "fresh": true, "project_revision": "4",
						}},
					},
					map[string]any{
						"round_number": float64(2),
						"observations": []any{map[string]any{
							"observation_id": "obs-round-2-base", "post_action": false, "fresh": true, "project_revision": "4",
						}},
					},
				},
			},
		},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "4", "phase": "fs8_experiment_verification",
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

// The owed round's intra-round proposal passes the final gate: G1 is
// structurally false (contract 2 vs closure 4) yet the proposal is not a new
// admission, so the full gate must not run on it.
func TestOwedRoundProposalSkipsFullAdmissionGate(t *testing.T) {
	state := intraRoundGateTestState(nil)
	if gateG1(state) {
		t.Fatal("fixture no longer reproduces the structural G1 failure (contract revision == closure revision)")
	}
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); issue != "" {
		t.Fatalf("owed-round proposal was refused by the full admission gate: %q", issue)
	}
}

// A real new admission (no experiment on the loop) in the same
// mismatched-revision context keeps the full gate: G1 must still refuse it.
func TestOwedRoundGateExemptionDoesNotCoverNewAdmissions(t *testing.T) {
	state := intraRoundGateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		delete(loop, "experiment")
	})
	issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil))
	if !strings.Contains(issue, "G1_project_binding") || !strings.Contains(issue, "needs_observation") {
		t.Fatalf("new admission escaped the G1 revision binding: %q", issue)
	}
}

// The sealed single-round tier keeps the full gate on every proposal-only
// needs_experiment while its experiment runs (default path zero change).
func TestOwedRoundGateExemptionExcludesSingleRoundTier(t *testing.T) {
	state := intraRoundGateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		experiment := loop["experiment"].(map[string]any)
		experiment["admission"] = map[string]any{"experiment_budget": float64(1)}
	})
	issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil))
	if !strings.Contains(issue, "G1_project_binding") {
		t.Fatalf("single-round running experiment skipped the admission gate: %q", issue)
	}
}

// A settled experiment is not an intra-round proposal surface: the full gate
// runs again and G1 refuses the post-settlement proposal.
func TestOwedRoundGateExemptionExcludesSettledExperiments(t *testing.T) {
	state := intraRoundGateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		experiment := loop["experiment"].(map[string]any)
		experiment["status"] = "settled"
	})
	issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil))
	if !strings.Contains(issue, "G1_project_binding") {
		t.Fatalf("settled experiment skipped the admission gate: %q", issue)
	}
}

// The exemption must not open the round boundary itself: a bare proposal on a
// round that already applied and carries fresh post-action evidence stays the
// illegal second admission mid-round (the settle report is the only legal
// exit), exactly as before.
func TestOwedRoundGateExemptionKeepsPendingSettlementRefusal(t *testing.T) {
	state := intraRoundGateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		experiment := loop["experiment"].(map[string]any)
		rounds := experiment["rounds"].([]any)
		current := rounds[len(rounds)-1].(map[string]any)
		current["observations"] = []any{
			map[string]any{"observation_id": "obs-round-2-base", "post_action": false, "fresh": true, "project_revision": "4"},
			map[string]any{"observation_id": "obs-round-2-after", "post_action": true, "fresh": true, "project_revision": "4"},
		}
	})
	issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil))
	if !strings.Contains(issue, "pending settlement") || !strings.Contains(issue, "illegal second admission") {
		t.Fatalf("pending-settlement round boundary was relaxed: %q", issue)
	}
}

// The exemption must not revive the human-judgment boundary: a bare proposal
// while an earlier round's judgment has not landed stays refused.
func TestOwedRoundGateExemptionKeepsJudgmentBoundaryRefusal(t *testing.T) {
	state := intraRoundGateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		experiment := loop["experiment"].(map[string]any)
		rounds := experiment["rounds"].([]any)
		rounds[0].(map[string]any)["user_judgment_requested"] = true
	})
	issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil))
	if !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("judgment boundary was revived by a bare proposal: %q", issue)
	}
}
