package agentloop

import (
	"fmt"
	"strings"
	"testing"
)

// BOUNDARY-1 v2 Phase 2: prompt-level information and pricing. Every number
// in the new closed templates is filled from the authoritative loop/closure
// counters (single source, §2.4); the frozen sentences are pinned verbatim.

func boundaryPromptState(loop map[string]any, mutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"task_contract":             map[string]any{"kind": "improvement"},
		"free_state_phase":          "fs6_target_confirmed",
		"free_state_reasoning_loop": loop,
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7", "phase": "fs6_target_confirmed",
			"rounds_started": 4,
			"policy":         map[string]any{"max_closure_rounds": 6},
			"hypothesis_frontier": map[string]any{
				"candidate_id": "candidate-1",
				"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}},
			},
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

func boundaryLoopContext(mutate func(loop map[string]any)) map[string]any {
	loop := map[string]any{
		"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
		"continuation_budget": 6, "continuation_used": 2,
	}
	if mutate != nil {
		mutate(loop)
	}
	return loop
}

// §2.4 single-source pin: the price sentence's numbers follow the
// authoritative counters — mutate the counters and the rendered digits move
// with them; no hardcoded constants.
func TestWindowBudgetSentenceFollowsAuthoritativeCounters(t *testing.T) {
	state := boundaryPromptState(boundaryLoopContext(nil), nil)
	sentence := messageLoopFreeStateWindowBudgetSentence(state)
	want := "Further observations consume window budget (closure rounds remaining: 2/6; continuations remaining: 4/6). The evidence you hold is sufficient for a bounded proposal."
	if sentence != want {
		t.Fatalf("price sentence = %q, want %q", sentence, want)
	}
	// Mutate both counters: the digits must follow.
	state = boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["continuation_budget"], loop["continuation_used"] = 9, 8
	}), func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["rounds_started"] = 1
		messageLoopMapValue(messageLoopMapValue(ctx["minimal_audio_closure"])["policy"])["max_closure_rounds"] = 3
	})
	want = "Further observations consume window budget (closure rounds remaining: 2/3; continuations remaining: 1/9). The evidence you hold is sufficient for a bounded proposal."
	if got := messageLoopFreeStateWindowBudgetSentence(state); got != want {
		t.Fatalf("mutated counters did not move the rendered digits: %q", got)
	}
	// Without the closure disclosure the sentence stays silent rather than
	// inventing numbers.
	state = boundaryPromptState(boundaryLoopContext(nil), func(ctx map[string]any) {
		delete(ctx, "minimal_audio_closure")
	})
	if got := messageLoopFreeStateWindowBudgetSentence(state); got != "" {
		t.Fatalf("sentence must not render without authoritative counters, got %q", got)
	}
}

// The terminal-turn directive carries the frozen final-turn sentence verbatim
// with the window counters in front of it.
func TestTerminalTurnDirectiveRendersFrozenSentence(t *testing.T) {
	state := boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["terminal_turn_locked"] = true
	}), nil)
	directive := messageLoopFreeStateTerminalTurnDirective(state)
	if !strings.Contains(directive, freeStateTerminalTurnSentence) {
		t.Fatalf("terminal directive missing the frozen sentence: %q", directive)
	}
	if !strings.Contains(directive, "closure rounds remaining: 2/6") || !strings.Contains(directive, "continuations remaining: 4/6") {
		t.Fatalf("terminal directive missing the authoritative counters: %q", directive)
	}
	// Unlocked loops never see it.
	if got := messageLoopFreeStateTerminalTurnDirective(boundaryPromptState(boundaryLoopContext(nil), nil)); got != "" {
		t.Fatalf("unlocked loop rendered the terminal directive: %q", got)
	}
	// A locked loop also silences the ordinary budget-pressure directive.
	if got := messageLoopFreeStateContinuationBudgetDirective(state); got != "" {
		t.Fatalf("locked loop must not carry the ordinary budget directive: %q", got)
	}
}

// TIMING-1: the gate-open signal is retired with the phase admission gate
// (needs_experiment is admitted in every FS phase; there is no later gate
// opening to signal). A loop context still carrying the retired pending flag
// renders nothing.

// TIMING-1: the phase bounce sentence no longer carries proposal-timing
// recovery semantics. needs_experiment is admitted at fs4 (and every FS
// phase); the bounce can still fire for the remaining status families
// (needs_action outside its phases), stating the procedural fact only.
func TestPhaseBounceSentenceConvergedAfterTimingGateRemoval(t *testing.T) {
	state := boundaryPromptState(boundaryLoopContext(nil), func(ctx map[string]any) {
		ctx["free_state_phase"] = "fs4_diagnostic_round"
		messageLoopMapValue(ctx["minimal_audio_closure"])["phase"] = "fs4_diagnostic_round"
	})
	if issue := messageLoopFreeStatePhaseDecisionIssue(state, "needs_experiment"); issue != "" {
		t.Fatalf("needs_experiment must not be phase-bounced at fs4: %q", issue)
	}
	if issue := messageLoopFreeStatePhaseDecisionIssue(state, "improvement_proposal"); issue != "" {
		t.Fatalf("improvement_proposal must not be phase-bounced at fs4: %q", issue)
	}
	issue := messageLoopFreeStatePhaseDecisionIssue(state, "needs_action")
	if issue == "" {
		t.Fatal("needs_action outside its phases must still bounce")
	}
	if !strings.Contains(issue, "fs4_diagnostic_round") || !strings.Contains(issue, "needs_action") {
		t.Fatalf("bounce sentence missing the phase/status facts: %q", issue)
	}
	if strings.Contains(issue, "gate will admit") || strings.Contains(issue, "watch the disclosed free_state_phase") {
		t.Fatalf("bounce sentence kept retired gate-open recovery semantics: %q", issue)
	}
}

// §2.3: the frontier directives are closed templates with counter slots —
// the absolute MUST / "Do not request another observation" phrasings are
// gone, the price sentence is in.
func TestFrontierDirectivesCarryWindowPricing(t *testing.T) {
	state := boundaryPromptState(boundaryLoopContext(nil), nil)
	directive := messageLoopCandidateFrontierDirective(state)
	if directive == "" {
		t.Fatal("selected-candidate frontier state must render a directive")
	}
	if !strings.Contains(directive, "Further observations consume window budget (closure rounds remaining: 2/6; continuations remaining: 4/6)") {
		t.Fatalf("frontier directive missing the priced window budget: %q", directive)
	}
	for _, banned := range []string{"You MUST", "Do not request another observation", "Return final=true now"} {
		if strings.Contains(directive, banned) {
			t.Fatalf("frontier directive kept the absolute phrasing %q: %q", banned, directive)
		}
	}
	// The numbers follow the counters here too.
	mutated := boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["continuation_budget"], loop["continuation_used"] = 5, 4
	}), func(ctx map[string]any) {
		messageLoopMapValue(ctx["minimal_audio_closure"])["rounds_started"] = 2
	})
	if got := messageLoopCandidateFrontierDirective(mutated); !strings.Contains(got, "closure rounds remaining: 4/6; continuations remaining: 1/5") {
		t.Fatalf("frontier directive digits did not follow the counters: %q", got)
	}
}

// The frozen terminal-turn sentence stays byte-identical everywhere it
// appears (prompt directive and output-gate bounce).
func TestFrozenTerminalSentenceIdentity(t *testing.T) {
	directive := messageLoopFreeStateTerminalTurnDirective(boundaryPromptState(boundaryLoopContext(func(loop map[string]any) {
		loop["terminal_turn_locked"] = true
	}), nil))
	if strings.Count(directive, freeStateTerminalTurnSentence) != 1 {
		t.Fatalf("terminal directive must embed the frozen sentence exactly once: %q", directive)
	}
	if !strings.HasSuffix(freeStateTerminalTurnSentence, "an explicit terminal statement of where you are blocked.") {
		t.Fatalf("frozen terminal sentence mutated: %q", freeStateTerminalTurnSentence)
	}
	_ = fmt.Sprint()
}
