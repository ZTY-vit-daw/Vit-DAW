package agentloop

import (
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

func TestVerifyMIDINotesPresentRequiresRefreshedStateNotes(t *testing.T) {
	call := planner.ToolCall{
		ID:   "write_notes",
		Tool: "midi.apply_note_patch",
		Args: map[string]any{
			"clip_id": "clip_1",
			"operations": []map[string]any{
				{"op": "insert_note", "pitch": 60, "start": 0, "length": 1},
				{"op": "insert_note", "pitch": 64, "start": 1, "length": 1},
			},
		},
	}
	result := executorpkg.Result{
		ToolCallID:  "write_notes",
		Tool:        "midi.apply_note_patch",
		CommandName: "apply_midi_note_patch",
		Status:      "ok",
		Result: map[string]any{
			"clip_id":           "clip_1",
			"inserted_count":    2,
			"inserted_note_ids": []string{"note_1", "note_2"},
		},
		ObservedState: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
			"clips": []map[string]any{{
				"id":    "clip_1",
				"notes": []map[string]any{},
			}},
		}}},
	}

	ver := verifyToolExecution(call, result)
	if ver.Status != verificationUnverified || ver.Postcondition != postconditionMIDINotesPresent {
		t.Fatalf("verification = status=%q postcondition=%q message=%q", ver.Status, ver.Postcondition, ver.Message)
	}
}

func TestVerifyMIDINotesPresentMatchesInsertedIDs(t *testing.T) {
	call := planner.ToolCall{
		ID:   "write_notes",
		Tool: "midi.apply_note_patch",
		Args: map[string]any{"clip_id": "clip_1"},
	}
	result := executorpkg.Result{
		ToolCallID:  "write_notes",
		Tool:        "midi.apply_note_patch",
		CommandName: "apply_midi_note_patch",
		Status:      "ok",
		Result: map[string]any{
			"clip_id":           "clip_1",
			"inserted_count":    2,
			"inserted_note_ids": []string{"note_1", "note_2"},
		},
		ObservedState: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
			"clips": []map[string]any{{
				"id": "clip_1",
				"notes": []map[string]any{
					{"id": "note_1", "pitch": 60},
					{"id": "note_2", "pitch": 64},
				},
			}},
		}}},
	}

	ver := verifyToolExecution(call, result)
	if ver.Status != verificationVerified || ver.Postcondition != postconditionMIDINotesPresent {
		t.Fatalf("verification = status=%q postcondition=%q message=%q", ver.Status, ver.Postcondition, ver.Message)
	}
}
