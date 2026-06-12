package chat

import "testing"

func TestProjectResultCardsUseExecutionTarget(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":       "ok",
			"tool":         "midi.apply_note_patch",
			"command_name": "apply_midi_note_patch",
			"result": map[string]any{
				"status":    "ok",
				"ui_action": "midi_note_patch",
				"track_id":  "1007",
				"clip_id":   "clip_a",
			},
		},
	})
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	target, ok := cards[0]["target"].(map[string]any)
	if !ok {
		t.Fatalf("target missing: %#v", cards[0])
	}
	if target["track_id"] != "1007" || target["clip_id"] != "clip_a" || target["can_audition"] != true {
		t.Fatalf("target = %#v", target)
	}
}

func TestProjectResultCardsUseVerificationEvidence(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":       "ok",
			"tool":         "midi.create_clip",
			"command_name": "create_midi_clip",
			"result": map[string]any{
				"status": "ok",
			},
			"verification": map[string]any{
				"status":        "verified",
				"postcondition": "clip_present",
				"evidence": map[string]any{
					"observed_track_id": "1007",
					"observed_clip_id":  "clip_new",
				},
			},
		},
	})
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	target, ok := cards[0]["target"].(map[string]any)
	if !ok {
		t.Fatalf("target missing: %#v", cards[0])
	}
	if target["track_id"] != "1007" || target["clip_id"] != "clip_new" || target["can_audition"] != true {
		t.Fatalf("target = %#v", target)
	}
}

func TestProjectResultCardsSkipReadOnly(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":       "ok",
			"tool":         "midi.read_notes",
			"command_name": "get_midi_clip_notes",
			"result": map[string]any{
				"status":   "ok",
				"track_id": "1007",
				"clip_id":  "clip_a",
			},
		},
	})
	if len(cards) != 0 {
		t.Fatalf("cards = %#v, want none", cards)
	}
}

func TestProjectResultStableIDPrefersAgentActionID(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":          "ok",
			"tool_call_id":    "tool_step_1",
			"agent_action_id": "act_unique",
			"tool":            "track.add",
			"command_name":    "add_track",
			"result": map[string]any{
				"status":   "ok",
				"track_id": "1007",
			},
		},
	})
	if len(cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(cards))
	}
	if cards[0]["id"] != "project_result_act_unique" {
		t.Fatalf("id = %#v", cards[0]["id"])
	}
}
