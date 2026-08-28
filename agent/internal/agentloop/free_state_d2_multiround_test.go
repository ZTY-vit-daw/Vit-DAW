package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
)

// D2-2 multi-round agentloop wiring: the model prompt keeps the single-round
// prohibitions byte-identical on the default tier and only swaps in the
// next-round calibration wording under an explicitly admitted multi-round
// budget (continue_once and the per-round single-mutation prohibition stay on
// every tier); the final-gate judgment boundary persists at experiment scope.

func multiRoundExperimentLoopContext(budget int, rounds []any) map[string]any {
	loop := map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
	}
	if budget > 0 {
		loop["experiment"] = map[string]any{
			"status": "running",
			"admission": map[string]any{
				"experiment_budget": budget,
			},
			"rounds": rounds,
		}
	}
	return loop
}

func TestPromptTierConditionalSingleRoundDefault(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		"D1-S1 permits one experiment round and one forward mutation. Never return next_round, continue_once, or a second treatment after the action has been applied",
		"it MUST NOT request next_round or another mutation",
		`"verification_plan":{"experiment_budget":1,"max_action_attempts":1}`,
		"In every case include verification_plan with experiment_budget=1 and max_action_attempts=1.",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("single-round prompt lost its sealed prohibition: %q", fragment)
		}
	}
	for _, fragment := range []string{
		"multi-round experiment permits exactly one forward mutation per round",
		"next calibration round",
		`(runtime-controlled)`,
	} {
		if strings.Contains(prompt, fragment) {
			t.Fatalf("multi-round wording leaked into the default single-round prompt: %q", fragment)
		}
	}
}

func TestPromptTierConditionalMultiRoundByInjection(t *testing.T) {
	t.Setenv(FreeStateD2MultiRoundBudgetEnv, "3")
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		"This multi-round experiment permits exactly one forward mutation per round within the runtime-controlled experiment budget",
		"never return continue_once",
		"never apply or request a second treatment within the same round",
		"An ambiguous human judgment (a difference was heard but neither candidate is preferred) is terminal",
		"the runtime may then open one next calibration round",
		`"verification_plan":{"experiment_budget":3,"max_action_attempts":1}`,
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("multi-round prompt missing %q", fragment)
		}
	}
	if strings.Contains(prompt, "it MUST NOT request next_round or another mutation") {
		t.Fatal("single-round subthreshold prohibition leaked into the multi-round prompt")
	}
}

func TestPromptTierFollowsLiveAdmissionBudget(t *testing.T) {
	// A live experiment's own admission budget is authoritative even without
	// an injection.
	multi := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": multiRoundExperimentLoopContext(3, nil),
	}}}
	if !messageLoopFreeStateMultiRoundTier(multi) {
		t.Fatal("live admission budget 3 was not detected as multi-round")
	}
	prompt := messageLoopNeutralFamilySystemPrompt(multi)
	if !strings.Contains(prompt, "one next calibration round") {
		t.Fatal("live multi-round admission did not select the multi-round wording")
	}
	single := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": multiRoundExperimentLoopContext(1, nil),
	}}}
	if messageLoopFreeStateMultiRoundTier(single) {
		t.Fatal("live admission budget 1 was detected as multi-round")
	}
	// Budgets above the sealed bound fall back to the single-round wording.
	over := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": multiRoundExperimentLoopContext(int(experiment.MaxD2MultiRoundBudget) + 1, nil),
	}}}
	if messageLoopFreeStateMultiRoundTier(over) {
		t.Fatal("an over-bound admission budget was detected as multi-round")
	}
}

// Sealed (survey §4.2-6): the final-gate judgment boundary persists at
// experiment scope. An earlier round whose judgment request never landed keeps
// every revival refused (observation / action / new admission / next round)
// while the settle family stays admitted; a landed judgment on an earlier
// round does not park the later calibration rounds.
func TestJudgmentBoundarySpansRoundsAtFinalGate(t *testing.T) {
	pendingEarlier := gateTestState(func(ctx map[string]any) {
		ctx["free_state_reasoning_loop"] = multiRoundExperimentLoopContext(3, []any{
			map[string]any{"round_id": "round-1", "number": 1, "user_judgment_requested": true},
			map[string]any{"round_id": "round-2", "number": 2},
		})
	})
	if !messageLoopFreeStateJudgmentBoundary(pendingEarlier) {
		t.Fatal("final gate missed the earlier round's pending judgment")
	}
	revival := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "observe again", RequestedViewIDs: []string{"track.basic_energy"},
	}}
	if issue := messageLoopFreeStateOutputIssue(pendingEarlier, revival); !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("needs_observation revived the parked experiment: %q", issue)
	}
	nextRound := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "calibrate again",
		ExperimentRoundDecision: string(experiment.DecisionNextRound),
	}}
	if issue := messageLoopFreeStateOutputIssue(pendingEarlier, nextRound); !strings.Contains(issue, "human-judgment boundary") {
		t.Fatalf("next_round crossed the parked experiment: %q", issue)
	}
	settle := messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment,
		EvidenceStatus: "plausible", Summary: "stop here",
		ExperimentRoundDecision: string(experiment.DecisionStopped),
	}}
	if issue := messageLoopFreeStateOutputIssue(pendingEarlier, settle); issue != "" {
		t.Fatalf("settle decision was refused at the boundary: %q", issue)
	}

	landedEarlier := gateTestState(func(ctx map[string]any) {
		ctx["free_state_reasoning_loop"] = multiRoundExperimentLoopContext(3, []any{
			map[string]any{"round_id": "round-1", "number": 1, "user_judgment_requested": true,
				"user_judgment_evidence": []any{map[string]any{"id": "judgment-1", "heard_difference": "no"}}},
			map[string]any{"round_id": "round-2", "number": 2},
		})
	})
	if messageLoopFreeStateJudgmentBoundary(landedEarlier) {
		t.Fatal("a landed earlier-round judgment parked the later calibration rounds")
	}
	if issue := messageLoopFreeStateJudgmentBoundaryIssue(landedEarlier, revival.FreeStateDecision); issue != "" {
		t.Fatalf("recalibration round refused by the judgment boundary after a landed judgment: %q", issue)
	}
}
