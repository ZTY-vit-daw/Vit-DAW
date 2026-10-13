package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
)

func TestNeutralSemanticPromptAdmitsGenericL3ImprovementProposal(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		`"status":"needs_experiment"`,
		`"schema_version":"improvement_proposal.v1"`,
		"Do not force a unique processor family",
		"action_domain=track_gain, action_kind=track_gain_adjust",
		`"track.band_dynamics"`,
		"never prefix or namespace it with a track ID",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("semantic prompt missing L3 improvement contract %q", fragment)
		}
	}
}

func TestNeutralSemanticPromptD1DomainRuleFollowsDomainTable(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	rule := freeStateD1AdmittedDomainRule()
	if !strings.Contains(prompt, rule) {
		t.Fatal("neutral prompt must embed the table-driven D1-S1 domain rule verbatim")
	}
	specs := experiment.D1S1DomainSpecs()
	if len(specs) < 2 {
		t.Fatalf("D2-1 expects at least the track_gain and static_eq domain rows, got %d", len(specs))
	}
	for _, spec := range specs {
		for _, fragment := range []string{"action_domain=" + spec.ActionDomain, "action_kind=" + spec.ActionKind, spec.PromptParameterHint} {
			if !strings.Contains(rule, fragment) {
				t.Fatalf("domain rule omits %q for domain %s", fragment, spec.ActionDomain)
			}
		}
	}
}

func TestFreeStatePromptCarriesExperimentEvaluationContract(t *testing.T) {
	prompt := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	for _, fragment := range []string{
		"report experiment_materiality only from the fresh",
		"experiment_target_response with",
		`experiment_round_decision="user_judgment_pending"`,
		"subthreshold state MUST use evaluation=insufficient_dose",
		"Target response must never be inferred from parameter readback alone",
		"MUST NOT request next_round or another mutation",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("experiment prompt missing %q", fragment)
		}
	}
}

// FIX-FS-SETTLE-DEADCHAIN-1 C-leg: the proposal-phase settle prohibition. The
// skeleton's needs_experiment example must not teach the settle-report fields
// (deepseek copied them verbatim onto proposal turns — the placeholder trio
// skipped the admission audit and pre-filled hallucinated settle claims), and
// the discipline rules must state the prohibition explicitly.
func TestNeutralSkeletonProposalShapeOmitsSettleReportFields(t *testing.T) {
	skeleton := messageLoopNeutralFamilySystemSkeleton(&runState{input: Input{Context: map[string]any{}}})
	for _, line := range strings.Split(skeleton, "\n") {
		if !strings.Contains(line, `"status":"needs_experiment"`) || !strings.Contains(line, `"final":true`) {
			continue
		}
		for _, field := range []string{"experiment_materiality", "experiment_target_response", "experiment_round_decision"} {
			if strings.Contains(line, field) {
				t.Fatalf("needs_experiment skeleton example still teaches settle field %s", field)
			}
		}
	}
	if !strings.Contains(skeleton, "never fill experiment_materiality, experiment_target_response, or experiment_round_decision") {
		t.Fatal("neutral skeleton lacks the proposal-phase settle prohibition rule")
	}
}

func TestImprovementContractPromptCarriesConvergenceGuidance(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
	}}}
	prompt := messageLoopNeutralFamilySystemPrompt(state)
	for _, fragment := range []string{
		"Fair Observation Selection for Open Improvement Contracts",
		"continuation_budget and continuation_used",
		"rush the GATE PATH instead",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("improvement prompt missing convergence guidance %q", fragment)
		}
	}
	neutral := messageLoopNeutralFamilySystemPrompt(&runState{input: Input{Context: map[string]any{}}})
	if strings.Contains(neutral, "Continuation Budget Discipline") {
		t.Fatal("convergence guidance leaked into a non-improvement contract prompt")
	}
	if strings.Contains(neutral, "Fair Observation Selection for Open Improvement Contracts") {
		t.Fatal("convergence guidance leaked into a non-improvement contract prompt")
	}
}

func TestFreeStatePromptContextDisclosesContinuationBudget(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":      "free_state_reasoning_loop.v1",
			"continuation_budget": 6,
			"continuation_used":   2,
		},
	}}}
	ctx := messageLoopFreeStatePromptContext(state)
	if ctx["continuation_budget"] != 6 || ctx["continuation_used"] != 2 {
		t.Fatalf("prompt context must disclose continuation budget/used, got %#v", ctx)
	}
}

func budgetState(used, budget int) *runState {
	return budgetStateInPhase(used, budget, "fs4_diagnostic_round")
}

func budgetStateInPhase(used, budget int, phase string) *runState {
	return &runState{input: Input{Context: map[string]any{
		"task_contract":    map[string]any{"kind": "improvement"},
		"free_state_phase": phase,
		"free_state_reasoning_loop": map[string]any{
			"continuation_budget": budget,
			"continuation_used":   used,
		},
	}}}
}

func TestContinuationBudgetDirectiveEscalation(t *testing.T) {
	if d := messageLoopFreeStateContinuationBudgetDirective(budgetState(1, 6)); d != "" {
		t.Fatalf("early-budget turn must carry no directive, got %q", d)
	}
	if d := messageLoopFreeStateContinuationBudgetDirective(budgetState(3, 6)); d == "" || !strings.Contains(d, "Continuation budget warning") {
		t.Fatalf("half-spent turn must carry the warning directive, got %q", d)
	}
	// TIMING-1: proposals are admissible in every closure phase, so the
	// critical branch no longer splits on the host phase — fs4 and fs6 carry
	// the same descriptive honesty about admission via the evidence gate.
	for _, phase := range []string{"fs4_diagnostic_round", "fs6_target_confirmed"} {
		d := messageLoopFreeStateContinuationBudgetDirective(budgetStateInPhase(5, 6, phase))
		if d == "" || !strings.Contains(d, "CONTINUATION BUDGET CRITICAL") ||
			!strings.Contains(strings.ToLower(d), "a needs_experiment decision with one bounded improvement_proposal is admissible") {
			t.Fatalf("%s last-turn state must state the admissible proposal honestly, got %q", phase, d)
		}
		for _, retired := range []string{"does not admit needs_experiment yet", "bounced by the phase gate", "admits needs_experiment only after"} {
			if strings.Contains(d, retired) {
				t.Fatalf("%s critical directive kept the retired phase-timing claim %q: %q", phase, retired, d)
			}
		}
	}
	if strings.Contains(messageLoopFreeStateContinuationBudgetDirective(budgetState(5, 6)), "MUST") {
		t.Fatal("critical directive must be descriptive pricing, not an absolute output command")
	}
	// An admitted experiment spends continuations legitimately: no pressure.
	experiment := budgetState(5, 6)
	experiment.input.Context["free_state_reasoning_loop"].(map[string]any)["latest_decision"] = map[string]any{"status": "needs_experiment"}
	if d := messageLoopFreeStateContinuationBudgetDirective(experiment); d != "" {
		t.Fatalf("post-proposal experiment turns must not be pressured, got %q", d)
	}
}
