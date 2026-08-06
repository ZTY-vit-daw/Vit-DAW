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

func TestProjectResultCardsPreservePluginLoadName(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":       "ok",
			"tool":         "plugin.load_to_rack",
			"command_name": "rack_add_node",
			"result": map[string]any{
				"status":      "ok",
				"track_id":    "1007",
				"plugin_id":   "plugin_1",
				"plugin_name": "TDR Nova",
			},
			"verification": map[string]any{
				"status":        "verified",
				"postcondition": "rack_node_present",
				"evidence": map[string]any{
					"observed_plugin_name": "TDR Nova",
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
	if target["track_id"] != "1007" || target["plugin_id"] != "plugin_1" || target["plugin_name"] != "TDR Nova" {
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

func TestProjectResultCardsSkipInnerErrorResult(t *testing.T) {
	cards := projectResultCardsFromExecuted([]map[string]any{
		{
			"status":       "ok",
			"tool":         "mix.apply_tick",
			"command_name": "mix_apply_tick",
			"result": map[string]any{
				"status":   "error",
				"error":    "confirmation is required",
				"track_id": "1007",
			},
		},
	})
	if len(cards) != 0 {
		t.Fatalf("cards = %#v, want none", cards)
	}
}

func TestProjectResultCardsWithABSkipWhenMutationFailed(t *testing.T) {
	executed := []map[string]any{
		{
			"status":       "completed",
			"command_name": "plugin.effect_control.v0",
			"result": map[string]any{
				"status":    "error",
				"message":   "live parameter surface does not match the current plugin",
				"track_id":  "1007",
				"plugin_id": "1015",
			},
		},
	}
	observe := executorResultForMixTickReport("mix.observe", map[string]any{
		"observation": map[string]any{
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"ab_result": map[string]any{"status": "ready"},
				},
			},
		},
	})

	if cards := projectResultCardsFromExecutedWithAB(executed, observe); len(cards) != 0 {
		t.Fatalf("cards = %#v, want none for failed mutation", cards)
	}
}

func TestProjectResultCardsSkipReadOnlyPluginGrabberQueries(t *testing.T) {
	executed := []map[string]any{
		{
			"status":       "ok",
			"command_name": "plugin_grabber.explain_controls",
			"result": map[string]any{
				"status":    "ok",
				"track_id":  "1007",
				"plugin_id": "1013",
			},
		},
	}
	if cards := projectResultCardsFromExecuted(executed); len(cards) != 0 {
		t.Fatalf("cards = %#v, want none for read-only plugin grabber queries", cards)
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
