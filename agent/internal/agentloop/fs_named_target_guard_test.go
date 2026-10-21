package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/planner"
)

// FS-NAMED-TARGET-1 guard faces (test-only addition; the guard mechanism and
// its derivation are unchanged): a user_intent synthetic candidate carries
// track_ids like any machine candidate, so the progression guard's
// allowed-track set — derived as the union of candidate track_ids — must
// admit a track.* observation on the user-named track and keep bouncing
// off-set targets. Fixture ids are synthetic (AGENTS §10).

func TestFSNamedTargetGuardAdmitsUserNamedTrack(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"minimal_audio_closure": map[string]any{"hypothesis_frontier": map[string]any{
			"candidates": []any{map[string]any{
				"id": "user_named:7001", "source": "user_intent", "track_ids": []any{"7001"},
			}},
		}},
	}}}
	onNamed := []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids":   []string{"track.timbre_frequency"},
		"target_ref": map[string]any{"kind": "track", "id": "7001"},
	}}}
	if issue := messageLoopFreeStateCandidateProgressionIssue(state, FreeStateNeedsObservation, onNamed); issue != "" {
		t.Fatalf("user_intent candidate track was not admitted by the progression guard: %q", issue)
	}
	offSet := []planner.ToolCall{{Tool: "ccb.observation_request", Args: map[string]any{
		"view_ids":   []string{"track.timbre_frequency"},
		"target_ref": map[string]any{"kind": "track", "id": "7999"},
	}}}
	if issue := messageLoopFreeStateCandidateProgressionIssue(state, FreeStateNeedsObservation, offSet); !strings.Contains(issue, "an id from the candidate track set") {
		t.Fatalf("off-set target bounce signature changed: %q", issue)
	}
}

// TestFSNamedTargetGatePathG5Wording pins the C5 wording sync on the GATE
// PATH G5 row: the disclosure names user_intent candidates as a second
// frontier source. Disclosure only — no evaluator reads this row.
func TestFSNamedTargetGatePathG5Wording(t *testing.T) {
	state := &runState{input: Input{Context: map[string]any{
		"task_contract": map[string]any{"kind": "improvement"},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "observing", "original_intent": "inspect the project",
		},
		"minimal_audio_closure": map[string]any{"hypothesis_frontier": map[string]any{"candidates": []any{}}},
	}}}
	directive := messageLoopFreeStateGatePathDirective(state)
	if !strings.Contains(directive, "G5_frontier_established") {
		t.Fatalf("GATE PATH directive did not render the G5 row: %q", directive)
	}
	if !strings.Contains(directive, "user_intent") {
		t.Fatalf("G5 row missed the C5 user_intent wording: %q", directive)
	}
}
