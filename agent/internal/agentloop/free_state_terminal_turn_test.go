package agentloop

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/planner"
)

// BOUNDARY-1 v2 Phase 1: the terminal-turn reservation. The output gate on a
// locked loop admits only (a) needs_experiment with a complete proposal or
// (b) the TerminalDecisions family; everything else bounces with the frozen
// final-turn sentence, so the reserved checkpoint can never be consumed by an
// observation or continuation request.

func terminalLockedState(mutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"task_contract": map[string]any{"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-7"},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
			"terminal_turn_locked": true, "terminal_turn_reason": "budget_critical",
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

func terminalUnlockedState() *runState {
	return terminalLockedState(func(ctx map[string]any) {
		delete(messageLoopMapValue(ctx["free_state_reasoning_loop"]), "terminal_turn_locked")
	})
}

func terminalObservationOutput() messageLoopOutput {
	return messageLoopOutput{
		FreeStateDecision: &FreeStateDecision{
			SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "need one more view",
		},
		ToolCalls: []planner.ToolCall{{Tool: "ccb.observation_catalog", Args: map[string]any{}, Reason: "discover"}},
	}
}

func TestFreeStateTerminalTurnRejectionMatrix(t *testing.T) {
	proposal := &agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "1007"}, EvidenceRefs: []string{"obs-target"},
		ImprovementIntent: "make the bass relationship feel clearer", Hypothesis: "a small bounded change may improve separation",
		ExpectedEffect: "the relationship should be easier to compare", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
		ActionKind: "bounded_gain_adjustment", ParameterBounds: map[string]any{"delta_db": -0.5}, Confidence: 0.55,
	}
	cases := []struct {
		name      string
		out       messageLoopOutput
		lockedEra string // "" = admitted through the terminal gate (later gates may still reject), non-empty = must bounce with the frozen sentence
	}{
		{name: "needs_observation_with_tool", out: terminalObservationOutput(), lockedEra: "sentence"},
		{name: "needs_observation_no_tool", out: messageLoopOutput{FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "x"}}, lockedEra: "sentence"},
		{name: "needs_action", out: messageLoopOutput{FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsAction, EvidenceStatus: "sufficient", Summary: "x", ProcessorType: "eq"}}, lockedEra: "sentence"},
		{name: "needs_experiment_without_proposal", out: messageLoopOutput{FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible", Summary: "x"}}, lockedEra: "sentence"},
		{name: "needs_clarification", out: messageLoopOutput{NeedsClarification: true, ClarificationQuestion: "which track?", Reply: "which track?"}, lockedEra: "sentence"},
		{name: "nil_decision", out: messageLoopOutput{Reply: "thinking out loud"}, lockedEra: "sentence"},
		{name: "needs_experiment_with_proposal", out: messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible", Summary: "x", ImprovementProposal: proposal}}},
		{name: "terminal_satisfied", out: messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateSatisfied, EvidenceStatus: "sufficient", Summary: "x"}}},
		{name: "terminal_no_candidate", out: messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNoCandidateFound, EvidenceStatus: "sufficient", Summary: "x"}}},
		{name: "terminal_capability_blocked", out: messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateCapabilityBlocked, EvidenceStatus: "insufficient", Summary: "x"}}},
		{name: "terminal_blocked", out: messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{SchemaVersion: FreeStateDecisionSchema, Status: FreeStateBlocked, EvidenceStatus: "insufficient", Summary: "x"}}},
	}
	for _, tc := range cases {
		issue := messageLoopFreeStateTerminalTurnIssue(terminalLockedState(nil), tc.out)
		if tc.lockedEra == "sentence" {
			if issue != freeStateTerminalTurnSentence {
				t.Fatalf("%s: locked turn must bounce with the frozen final-turn sentence, got %q", tc.name, issue)
			}
		} else if issue != "" {
			t.Fatalf("%s: admissible final family must pass the terminal gate, got %q", tc.name, issue)
		}
	}
	// The unlocked loop keeps today's semantics: no terminal sentence is ever
	// returned by the terminal check.
	for _, tc := range cases {
		if issue := messageLoopFreeStateTerminalTurnIssue(terminalUnlockedState(), tc.out); issue != "" {
			t.Fatalf("%s: unlocked turn must not receive the final-turn sentence, got %q", tc.name, issue)
		}
	}
}

// The reservation is structural: on a locked turn even a well-formed
// observation request with tool calls bounces before any tool could run.
func TestFreeStateTerminalReservationNotConsumedByObservation(t *testing.T) {
	state := terminalLockedState(nil)
	issue := messageLoopFreeStateOutputIssue(state, terminalObservationOutput())
	if issue != freeStateTerminalTurnSentence {
		t.Fatalf("observation on the reserved checkpoint must bounce with the frozen sentence, got %q", issue)
	}
}

// The retry counter increments at the rejection boundary and rides the loop
// context (SCAFFOLD-1 marker pattern).
func TestFreeStateTerminalRetryNoteIncrementsLoopContext(t *testing.T) {
	state := terminalLockedState(nil)
	if got := messageLoopFreeStateTerminalRetryCount(state); got != 0 {
		t.Fatalf("retry count starts at 0, got %d", got)
	}
	messageLoopFreeStateNoteTerminalRetry(state)
	if got := messageLoopFreeStateTerminalRetryCount(state); got != 1 {
		t.Fatalf("retry count after one note = %d, want 1", got)
	}
	messageLoopFreeStateNoteTerminalRetry(state)
	if got := messageLoopFreeStateTerminalRetryCount(state); got != 2 {
		t.Fatalf("retry counter must be monotonic, got %d", got)
	}
}

// Full-loop fallback chain: a locked slice that keeps returning
// needs_observation gets exactly one strengthened retry inside the slice, then
// finishes with the dedicated fallback stop reason — never a success exit and
// never another model turn.
func TestMessageLoopTerminalFallbackAfterOneStrengthenedRetry(t *testing.T) {
	observation := `{"final":false,"reply":"need one more view","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"x"},"tool_calls":[{"tool":"ccb.observation_catalog","args":{},"reason":"discover"}]}`
	client := &fakeMessageCompleter{responses: []string{observation, observation}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 2, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	res := loop.Start(context.Background(), Input{
		UserText:     "improve the mix",
		AllowedTools: []string{"ccb.observation_catalog"},
		Context: map[string]any{
			"free_state_reasoning_loop": map[string]any{
				"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
				"terminal_turn_locked": true, "terminal_turn_reason": "budget_critical",
			},
		},
	})
	if len(client.calls) != 2 {
		t.Fatalf("model calls = %d, want exactly attempt + one strengthened retry", len(client.calls))
	}
	if res.Status != "failed" || res.StopReason != FreeStateTerminalFallbackStopReason {
		t.Fatalf("fallback result = status=%q stop=%q, want failed/%s", res.Status, res.StopReason, FreeStateTerminalFallbackStopReason)
	}
	if res.FreeStateDecision != nil {
		t.Fatalf("fallback must not fabricate a model decision, got %+v", res.FreeStateDecision)
	}
	// The retry the model actually received carries the frozen retry directive.
	retrySeen := false
	for _, message := range client.calls[1] {
		if strings.Contains(message.Content, freeStateTerminalTurnRetryDirective) {
			retrySeen = true
		}
	}
	if !retrySeen {
		t.Fatal("the strengthened retry prompt must contain the frozen retry directive")
	}
}
