package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/audioclosure"
)

// D2-NEUTRAL3 fair-presentation contract: the improvement convergence
// guidance must not name a fast-path view sequence, must not define
// cross-dimension touring as a failure mode, and must teach catalog
// discovery as the first observation step. Steering text only: these
// assertions pin the prompt layer, never a gate or runtime criterion.

func TestNeutral3ConvergenceGuidancePresentsViewsFairly(t *testing.T) {
	guidance := freeStateImprovementConvergenceGuidance()
	if !strings.Contains(guidance, "ccb.observation_catalog") {
		t.Fatal("convergence guidance must teach ccb.observation_catalog discovery")
	}
	for _, fragment := range []string{
		"first observation step",
		"no default view sequence",
		"legal and is often necessary",
	} {
		if !strings.Contains(guidance, fragment) {
			t.Fatalf("fair-presentation guidance missing %q", fragment)
		}
	}
	for _, banned := range []string{
		"mix.multitrack_relationship", "mix.frequency_relationship",
		"project.structure", "timbre", "static_eq", "track_gain",
		"Do NOT tour", "Breadth across dimensions",
	} {
		if strings.Contains(guidance, banned) {
			t.Fatalf("convergence guidance still names a view/domain or keeps anti-breadth wording: %q", banned)
		}
	}
}

func TestNeutral3BudgetDirectiveOmitsNamedViewsAndAntiBreadth(t *testing.T) {
	directives := map[string]string{
		"warning": messageLoopFreeStateContinuationBudgetDirective(budgetState(3, 6)),
		"critical": messageLoopFreeStateContinuationBudgetDirective(budgetState(5, 6)),
	}
	for name, directive := range directives {
		if directive == "" {
			t.Fatalf("%s budget directive must still fire", name)
		}
		for _, banned := range []string{
			"mix.multitrack_relationship or mix.frequency_relationship",
			"do not open a new diagnostic dimension",
			"Do not spend this turn on other dimensions",
		} {
			if strings.Contains(directive, banned) {
				t.Fatalf("%s budget directive still carries named fast path or anti-breadth wording %q", name, banned)
			}
		}
	}
}

func TestNeutral3FrontierDirectiveDropsCatalogLockout(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"minimal_audio_closure": map[string]any{"hypothesis_frontier": map[string]any{
			"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}},
		}},
	}}}
	directive := messageLoopCandidateFrontierDirective(state)
	if directive == "" {
		t.Fatal("frontier directive must still fire with candidates present")
	}
	if strings.Contains(directive, "Do not request a catalog, project.*, or mix.* view") {
		t.Fatal("frontier directive still carries the catalog/project/mix lockout")
	}
	if !strings.Contains(directive, "track.*") {
		t.Fatal("frontier directive must keep gate alignment (target-level track.* resolution)")
	}
}

func TestNeutral3ImprovementPromptCarriesFairPresentationAndAuthorizationSemantics(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
	}}}
	prompt := messageLoopNeutralFamilySystemPrompt(state)
	for _, fragment := range []string{
		"ccb.observation_catalog",
		"no default view sequence",
		"legal and is often necessary",
		"still authorizes bounded reversible improvement experiments",
		"not statements about your task authorization",
		"must not be used alone as a capability_blocked authorization basis",
	} {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("improvement prompt missing fair-presentation/authorization fragment %q", fragment)
		}
	}
	for _, banned := range []string{
		"Breadth across dimensions is the most common way these tasks fail",
		"Do NOT tour the other diagnostic dimensions",
		"Do not request a catalog, project.*, or mix.* view",
		"mix.multitrack_relationship or mix.frequency_relationship",
	} {
		if strings.Contains(prompt, banned) {
			t.Fatalf("improvement prompt still carries steering fragment %q", banned)
		}
	}
}

// D2-NEUTRAL3 disclosure contract: the compact free-state ledger projection
// must disclose the closure host's priority-queue dimension state (open /
// closed / skipped) so "dimensions still unobserved" is a model-visible
// fact. Disclosure of the existing record only: no readiness upgrade.
func neutral3QueueFixture() map[string]any {
	entries := []any{}
	for i, dim := range audioclosure.DefaultDimensionOrder {
		entry := map[string]any{"dimension": string(dim), "priority_reason": "default_order"}
		if i < 2 {
			entry["status"] = "closed"
		} else {
			entry["status"] = "open"
		}
		entries = append(entries, entry)
	}
	return map[string]any{"schema_version": audioclosure.PriorityQueueSchema, "entries": entries}
}

func TestNeutral3PromptContextDisclosesPriorityQueueOpenDimensions(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":      "free_state_reasoning_loop.v1",
			"continuation_budget": 6,
			"continuation_used":   2,
			"priority_queue":      neutral3QueueFixture(),
		},
	}}}
	ctx := messageLoopFreeStatePromptContext(state)
	summary := messageLoopMapValue(ctx["priority_queue"])
	if len(summary) == 0 {
		t.Fatalf("compact ledger must disclose the priority queue, got %#v", ctx)
	}
	open := messageLoopStringList(summary["open_dimensions"])
	if strings.Join(open, ",") != "dynamics,stereo_space,transient_event" {
		t.Fatalf("open dimensions not disclosed: %#v", summary["open_dimensions"])
	}
	closed := messageLoopStringList(summary["closed_dimensions"])
	if strings.Join(closed, ",") != "level_headroom,frequency_occupancy" {
		t.Fatalf("closed dimensions not disclosed: %#v", summary["closed_dimensions"])
	}
}

func TestNeutral3PromptContextDisclosesPriorityQueueFromClosureFallback(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":      "free_state_reasoning_loop.v1",
			"continuation_budget": 6,
			"continuation_used":   2,
		},
		"minimal_audio_closure": map[string]any{
			"priority_queue": neutral3QueueFixture(),
		},
	}}}
	ctx := messageLoopFreeStatePromptContext(state)
	summary := messageLoopMapValue(ctx["priority_queue"])
	if len(summary) == 0 || len(messageLoopStringList(summary["open_dimensions"])) != 3 {
		t.Fatalf("closure-fallback queue not disclosed: %#v", ctx["priority_queue"])
	}
}

func TestNeutral3PromptContextWithoutQueueStaysUndisclosed(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version":      "free_state_reasoning_loop.v1",
			"continuation_budget": 6,
			"continuation_used":   2,
		},
	}}}
	ctx := messageLoopFreeStatePromptContext(state)
	if _, ok := ctx["priority_queue"]; ok {
		t.Fatalf("a loop with no queue record must not get an invented disclosure: %#v", ctx["priority_queue"])
	}
	invalid := state.input.Context["free_state_reasoning_loop"].(map[string]any)
	invalid["priority_queue"] = map[string]any{"schema_version": "not-a-queue", "entries": "garbage"}
	if disclosed := messageLoopMapValue(messageLoopFreeStatePromptContext(state)["priority_queue"]); len(disclosed) > 0 {
		t.Fatalf("an unparseable queue must not be disclosed as dimension facts: %#v", disclosed)
	}
}
