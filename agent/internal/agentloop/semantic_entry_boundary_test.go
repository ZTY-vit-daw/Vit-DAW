package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

func semanticEntryState(route, authorization string) *runState {
	return &runState{input: Input{UserText: "make it clearer", Context: map[string]any{
		"semantic_entry_verified": true,
		"semantic_entry_decision": map[string]any{
			"schema_version": "semantic_entry_decision.v1",
			"route":          route, "control_mode": "ordinary_agent",
			"user_authorization": authorization,
		},
	}}}
}

func TestSemanticEntryRouteOwnsMutationBarrier(t *testing.T) {
	observation := semanticEntryState("observation", "observe_only")
	if !messageLoopMutationBarrierActive(observation) {
		t.Fatal("model observation route did not activate mutation barrier")
	}
	open := semanticEntryState("open_semantic", "action_requested")
	open.input.UserText = "why does this sound muddy"
	if messageLoopMutationBarrierActive(open) {
		t.Fatal("model open-semantic route was overridden by lexical discussion wording")
	}
	discussion := semanticEntryState("discussion", "none")
	if !messageLoopMutationBarrierActive(discussion) {
		t.Fatal("model discussion route did not activate mutation barrier")
	}
}

func TestSemanticEntryRouteDisablesDirectFamilySemanticAction(t *testing.T) {
	for _, route := range []string{"discussion", "observation", "explicit_control", "open_semantic", "other"} {
		state := semanticEntryState(route, "action_requested")
		if messageLoopSemanticEffectProposalAllowed(state) {
			t.Fatalf("route %s retained direct semantic_action authority", route)
		}
		if messageLoopOrdinarySemanticEQRequest(state) {
			t.Fatalf("route %s retained lexical EQ family gate", route)
		}
	}
}

func TestSemanticEntryExplicitTypedControlDoesNotRequireFreeStateObservation(t *testing.T) {
	state := semanticEntryState("explicit_control", "action_requested")
	decision := messageLoopMapValue(state.input.Context["semantic_entry_decision"])
	decision["control_mode"] = "typed_control"
	if messageLoopFreeStateActive(state) {
		t.Fatal("explicit typed control unexpectedly started the abstract free-state loop")
	}
	if issue := messageLoopFreeStateOutputIssue(state, messageLoopOutput{Final: true, Reply: "typed control handoff"}); issue != "" {
		t.Fatalf("explicit typed control was subjected to the CCB first-action gate: %s", issue)
	}
}

func TestSemanticEntryUnavailableDeniesMutationWithoutFamilyFallback(t *testing.T) {
	state := &runState{input: Input{UserText: "change the selected effect", Context: map[string]any{"semantic_entry_unavailable": true}}}
	for _, call := range []planner.ToolCall{
		{Tool: "plugin.load_to_rack", Args: map[string]any{"track_id": "track-1"}},
		{Tool: "track.add", Args: map[string]any{"name": "new track"}},
	} {
		issue := messageLoopToolGuardIssue(state, call, false)
		if !strings.Contains(issue, "semantic entry classification was unavailable") {
			t.Fatalf("mutation %s was not denied by unavailable entry: %q", call.Tool, issue)
		}
	}
}
