package chat

import (
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/policy"
)

func TestExecutedReplyListsTrackNamesOnly(t *testing.T) {
	after := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(nil, after, []policy.Decision{
		{Name: "list_tracks", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "list_tracks"}},
	}, nil)

	if reply != "当前有 1 条轨道：Track 1。" {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") || strings.Contains(reply, "即将") {
		t.Fatalf("reply leaked internal wording/id: %q", reply)
	}
}

func TestExecutedReplyFormatsEmptyMidiRead(t *testing.T) {
	after := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}
	reply := executedReply(nil, after, []policy.Decision{
		{
			Name: "get_midi_clip_notes",
			Risk: policy.RiskDirect,
			Command: map[string]any{
				"tool": "midi.read_clip_notes",
				"args": map[string]any{"clip_id": "clip_a"},
			},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":  "ok",
				"clip_id": "clip_a",
				"notes":   []any{},
			},
		},
	})
	if reply != "当前 MIDI clip 有 0 个 notes。" {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "Track 1") || strings.Contains(reply, "当前有 1 条轨道") {
		t.Fatalf("midi read fell back to track list: %q", reply)
	}
}

func TestExecutedReplyFormatsNonEmptyMidiRead(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "get_midi_clip_notes",
			Risk:    policy.RiskDirect,
			Command: map[string]any{"cmd": "get_midi_clip_notes", "clip_id": "clip_a"},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":  "ok",
				"clip_id": "clip_a",
				"notes": []any{
					map[string]any{"id": "note_a", "pitch": 60, "start": 0.0, "length": 1.0, "velocity": 100},
				},
			},
		},
	})
	for _, want := range []string{"当前 MIDI clip 有 1 个 notes", "id=note_a", "pitch=60", "start=0", "length=1", "velocity=100"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
}

func TestExecutedReplyFormatsMidiClipCreate(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "create_midi_clip",
			Risk:    policy.RiskConfirm,
			Command: map[string]any{"cmd": "create_midi_clip", "track_id": "1007"},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":      "ok",
				"clip_id":     "clip_new",
				"new_clip_id": "clip_new",
				"track_id":    "1007",
			},
		},
	})
	for _, want := range []string{"MIDI clip", "id=clip_new"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
}

func TestExecutedReplyFormatsPluginSearch(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{Name: "plugin_search", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "plugin_search", "query": "TDR Nova"}},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status": "ok",
				"query":  "TDR Nova",
				"plugins": []any{
					map[string]any{"name": "TDR Nova", "format": "VST3", "plugin_path": "C:/Program Files/Common Files/VST3/TDR Nova.vst3"},
				},
			},
		},
	})
	for _, want := range []string{"找到 1 个插件候选", "TDR Nova", "plugin_path"} {
		if want == "plugin_path" {
			if strings.Contains(reply, want) {
				t.Fatalf("reply leaked raw key name: %q", reply)
			}
			continue
		}
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
}

func TestExecutedReplyFormatsLargePluginListAsSummary(t *testing.T) {
	plugins := []any{}
	for i := 1; i <= 12; i++ {
		plugins = append(plugins, map[string]any{
			"name":         fmt.Sprintf("Plugin %02d", i),
			"format":       "VST3",
			"category":     "Fx|Dynamics",
			"manufacturer": "Test",
		})
	}
	reply := executedReply(nil, nil, []policy.Decision{
		{Name: "plugin_list_available", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "plugin_list_available"}},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":       "ok",
				"plugin_count": 120,
				"plugins":      plugins,
			},
		},
	})
	for _, want := range []string{"当前插件库约有 120 个可用插件", "显示前 8 个", "另有 112 个未显示"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
	if strings.Contains(reply, "Plugin 09") {
		t.Fatalf("reply should not list beyond summary limit: %q", reply)
	}
}

func TestExecutedReplyFormatsPluginSemanticSearch(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{Name: "plugin_semantic_search", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "plugin_semantic_search", "query": "eq", "type": "eq"}},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":    "ok",
				"transient": true,
				"entries": []any{
					map[string]any{
						"name":          "TDR Nova",
						"format":        "VST3",
						"manufacturer":  "Tokyo Dawn Labs",
						"plugin_path":   "C:/Program Files/Common Files/VST3/TDR Nova.vst3",
						"primary_type":  "eq",
						"confidence":    100,
						"search_score":  1260,
						"is_instrument": false,
					},
				},
			},
		},
	})
	for _, want := range []string{"TDR Nova", "eq", "Tokyo Dawn Labs"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
}

func TestExecutedReplyFormatsWebSearch(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{Name: "web_search", Risk: policy.RiskDirect, Command: map[string]any{"cmd": "web_search", "query": "牧童短笛 作者"}},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status": "ok",
				"query":  "牧童短笛 作者",
				"source": "bing_html",
				"results": []any{
					map[string]any{
						"title":   "牧童短笛",
						"url":     "https://example.com/mutong-duandi",
						"snippet": "《牧童短笛》由贺绿汀创作。",
					},
				},
			},
		},
	})
	for _, want := range []string{"Bing", "牧童短笛", "贺绿汀", "https://example.com/mutong-duandi"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q in %q", want, reply)
		}
	}
	if reply == "已完成。" {
		t.Fatalf("web search reply fell back to generic completion: %q", reply)
	}
}

func TestExecutedReplyFormatsLegacyMidiWrite(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "add_midi_notes_bulk",
			Risk:    policy.RiskConfirm,
			Command: map[string]any{"cmd": "add_midi_notes_bulk", "clip_id": "clip_a"},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"status":      "ok",
				"clip_id":     "clip_a",
				"added_count": 1,
			},
		},
	})
	if reply != "Updated MIDI clip: inserted 1." {
		t.Fatalf("reply = %q", reply)
	}
}

func TestExecutedReplyRenamesTrackAfterExecution(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}
	after := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Lead Vocal"},
		},
	}

	reply := executedReply(before, after, []policy.Decision{
		{
			Name:    "rename_track",
			Risk:    policy.RiskUndoable,
			Command: map[string]any{"cmd": "rename_track", "track_id": "1007", "name": "Lead Vocal"},
		},
	}, nil)

	if reply != "已把 Track 1 重命名为 Lead Vocal，可撤销。" {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") || strings.Contains(reply, "即将") {
		t.Fatalf("reply leaked internal wording/id: %q", reply)
	}
}

func TestExecutedReplyRenamesTrackFromKernelReply(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "rename_track",
			Risk:    policy.RiskUndoable,
			Command: map[string]any{"cmd": "rename_track", "track_id": "1007", "name": "A"},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"old_name":   "Track 1",
				"track_name": "A",
			},
		},
	})

	if reply != "已把 Track 1 重命名为 A，可撤销。" {
		t.Fatalf("reply = %q", reply)
	}
}

func TestExecutedReplyUsesKernelMuteStateForCancel(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(before, before, []policy.Decision{
		{
			Name: "set_mute",
			Risk: policy.RiskUndoable,
			Command: map[string]any{
				"tool": "track.mute",
				"args": map[string]any{
					"track_id": "1007",
					"enabled":  true,
				},
			},
		},
	}, []map[string]any{
		{
			"result": map[string]any{
				"track_id": "1007",
				"mute":     false,
			},
		},
	})

	if !strings.Contains(reply, "已取消 Track 1 的静音") {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") {
		t.Fatalf("reply leaked track id: %q", reply)
	}
}

func TestExecutedReplyDescribesClipMove(t *testing.T) {
	reply := executedReply(nil, nil, []policy.Decision{
		{
			Name:    "move_clip",
			Risk:    policy.RiskConfirm,
			Command: map[string]any{"cmd": "move_clip", "clip_id": "1011", "new_start": 20},
		},
	}, nil)

	if !strings.Contains(reply, "移动") || !strings.Contains(reply, "clip") {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1011") {
		t.Fatalf("reply leaked clip id: %q", reply)
	}
}

func TestExecutedReplyExpandsToolArgsForSoloCancel(t *testing.T) {
	before := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Track 1"},
		},
	}

	reply := executedReply(before, before, []policy.Decision{
		{
			Name: "set_solo",
			Risk: policy.RiskUndoable,
			Command: map[string]any{
				"tool": "track.solo",
				"args": map[string]any{
					"track_id": "1007",
					"enabled":  false,
				},
			},
		},
	}, nil)

	if !strings.Contains(reply, "已取消 Track 1 的独奏") {
		t.Fatalf("reply = %q", reply)
	}
	if strings.Contains(reply, "1007") {
		t.Fatalf("reply leaked track id: %q", reply)
	}
}

func TestSanitizeUserReplyReplacesTrackIDsUnlessAsked(t *testing.T) {
	state := map[string]any{
		"tracks": []map[string]any{
			{"track_id": "1007", "name": "Lead Vocal"},
		},
	}

	reply := sanitizeUserReply("当前轨道是 1007。", state, "列出当前轨道")
	if reply != "当前轨道是 Lead Vocal。" {
		t.Fatalf("reply = %q", reply)
	}

	technical := sanitizeUserReply("当前轨道 ID 是 1007。", state, "轨道 ID 是多少")
	if technical != "当前轨道 ID 是 1007。" {
		t.Fatalf("technical reply = %q", technical)
	}
}
