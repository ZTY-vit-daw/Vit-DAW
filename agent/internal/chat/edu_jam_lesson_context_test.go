package chat

// W02-A1b：有界课程上下文组装面单测（纯内存，不依赖内核/夹具卡）。
// 覆盖契约字段、上界与 truncated 语义、选中 clip 优先级/缺省，以及
// 设计源/notes/文件路径永不进入投影。

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/edu"
)

func eduJamLessonTestCard() *edu.LevelCard {
	assets := make([]edu.AssetSpec, 0, 10)
	for i := 0; i < 10; i++ {
		assets = append(assets, edu.AssetSpec{
			ID: fmt.Sprintf("asset_%d", i), Kind: "midi_reference",
			File: "assets/theme_twinkle_v0.mid", SHA256: strings.Repeat("a", 64),
		})
	}
	allow := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		allow = append(allow, fmt.Sprintf("write_cmd_%d", i))
	}
	return &edu.LevelCard{
		ID:            "test-level",
		Title:         strings.Repeat("长", contextruntime.LessonContextTitleMaxRunes+40),
		Assets:        assets,
		WriteBoundary: edu.WriteBoundaryBlock{Allow: allow},
		Nodes: []edu.LevelNode{
			{
				ID:    "n0_long",
				Title: strings.Repeat("节", contextruntime.LessonContextTitleMaxRunes+40),
				Goal:  strings.Repeat("目", contextruntime.LessonContextGoalMaxRunes+80),
			},
			{ID: "n1_short", Title: "第二关", Goal: "短目标"},
		},
	}
}

func eduJamLessonActiveSession(nodeID string) eduJamSession {
	return eduJamSession{Jam: edu.JamState{Nodes: []edu.JamNodeState{{NodeID: nodeID, State: edu.JamNodeActive}}}}
}

func TestEduJamLessonContextBoundsAndContractFields(t *testing.T) {
	card := eduJamLessonTestCard()
	lesson := eduJamLessonContext(eduJamLessonActiveSession("n0_long"), card, map[string]any{})
	if lesson == nil {
		t.Fatalf("lesson context is nil")
	}
	if lesson["schema_version"] != contextruntime.LessonContextSchemaVersion {
		t.Fatalf("schema_version = %v", lesson["schema_version"])
	}
	// Exact contract surface: nothing from the card (story/notes/file) leaks.
	wantKeys := map[string]bool{
		"schema_version": true, "card": true, "node": true, "assets": true,
		"allowed": true, "truncated": true,
	}
	if len(lesson) != len(wantKeys) {
		t.Fatalf("lesson keys = %v", lesson)
	}
	for key := range lesson {
		if !wantKeys[key] {
			t.Fatalf("unexpected lesson key %q: %v", key, lesson)
		}
	}
	cardRow := lesson["card"].(map[string]any)
	if cardRow["id"] != "test-level" {
		t.Fatalf("card = %v", cardRow)
	}
	if got := len([]rune(cardRow["title"].(string))); got != contextruntime.LessonContextTitleMaxRunes {
		t.Fatalf("card title runes = %d", got)
	}
	node := lesson["node"].(map[string]any)
	if node["id"] != "n0_long" || len([]rune(node["title"].(string))) != contextruntime.LessonContextTitleMaxRunes {
		t.Fatalf("node = %v", node)
	}
	if got := len([]rune(node["goal"].(string))); got != contextruntime.LessonContextGoalMaxRunes {
		t.Fatalf("node goal runes = %d", got)
	}
	assets := lesson["assets"].([]map[string]any)
	if len(assets) != contextruntime.LessonContextMaxItems {
		t.Fatalf("assets = %d", len(assets))
	}
	for _, row := range assets {
		if len(row) != 2 || row["id"] == nil || row["kind"] != "midi_reference" {
			t.Fatalf("asset row leaked non-contract fields: %v", row)
		}
	}
	allowed := lesson["allowed"].([]string)
	if len(allowed) != contextruntime.LessonContextMaxItems || allowed[0] != "write_cmd_0" {
		t.Fatalf("allowed = %v", allowed)
	}
	if lesson["truncated"] != true {
		t.Fatalf("truncated = %v", lesson["truncated"])
	}
	raw, err := json.Marshal(lesson)
	if err != nil {
		t.Fatalf("marshal lesson: %v", err)
	}
	for _, forbidden := range []string{"theme_twinkle_v0.mid", "sha256", "story", "\"notes\""} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("lesson leaked %q: %s", forbidden, raw)
		}
	}

	// Within bounds: no truncation, node follows the active node, and the
	// selected-clip block stays absent without a selection.
	shortCard := &edu.LevelCard{
		ID: "short-level", Title: "短标题",
		Assets: []edu.AssetSpec{
			{ID: "a1", Kind: "midi_reference", File: "assets/a1.mid"},
			{ID: "a2", Kind: "audio", File: "assets/a2.wav"},
		},
		WriteBoundary: edu.WriteBoundaryBlock{Allow: []string{"cmd_a", "cmd_b"}},
		Nodes:         []edu.LevelNode{{ID: "n1_short", Title: "第二关", Goal: "短目标"}},
	}
	short := eduJamLessonContext(eduJamLessonActiveSession("n1_short"), shortCard, map[string]any{})
	if short["truncated"] != false {
		t.Fatalf("short card truncated = %v", short["truncated"])
	}
	if shortNode := short["node"].(map[string]any); shortNode["id"] != "n1_short" || shortNode["title"] != "第二关" || shortNode["goal"] != "短目标" {
		t.Fatalf("short node = %v", shortNode)
	}
	if got := len(short["assets"].([]map[string]any)); got != 2 {
		t.Fatalf("short assets = %d", got)
	}
	if got := len(short["allowed"].([]string)); got != 2 {
		t.Fatalf("short allowed = %d", got)
	}
	if _, ok := short["selected_clip"]; ok {
		t.Fatalf("selected_clip present without selection: %v", short)
	}
}

func TestEduJamLessonContextSelectedClipResolution(t *testing.T) {
	card := &edu.LevelCard{ID: "clip-level", Title: "clip", Nodes: []edu.LevelNode{{ID: "n0", Title: "n0", Goal: "g"}}}
	session := eduJamLessonActiveSession("n0")
	cases := []struct {
		name  string
		ctx   map[string]any
		clip  string
		track string
	}{
		{"none", map[string]any{}, "", ""},
		{"single", map[string]any{"selected_clip_id": "c_single"}, "c_single", ""},
		{"focus_wins", map[string]any{
			"selected_clip_id": "c_single", "piano_roll_focus_clip_id": "c_focus",
		}, "c_focus", ""},
		{"multi_first", map[string]any{"selected_clip_ids": []any{"c_multi", "c_other"}}, "c_multi", ""},
		// track 优先级与 conversation.intent.selectedClipTrackID 同序：
		// piano_roll_focus_track_id > selected_clip_track_id > focused_track_id >
		// selected_track_id > track_id。
		{"focus_track_wins_all_keys", map[string]any{
			"selected_clip_id":          "c1",
			"piano_roll_focus_track_id": "track_focus",
			"selected_clip_track_id":    "track_clip",
			"focused_track_id":          "track_focused",
			"selected_track_id":         "track_selected",
			"track_id":                  "track_generic",
		}, "c1", "track_focus"},
		{"clip_track_second", map[string]any{
			"selected_clip_id":       "c1",
			"selected_clip_track_id": "track_clip",
			"focused_track_id":       "track_focused",
			"selected_track_id":      "track_selected",
			"track_id":               "track_generic",
		}, "c1", "track_clip"},
		{"focused_track_third", map[string]any{
			"selected_clip_id":  "c1",
			"focused_track_id":  "track_focused",
			"selected_track_id": "track_selected",
			"track_id":          "track_generic",
		}, "c1", "track_focused"},
		{"selected_track_fourth", map[string]any{
			"selected_clip_id":  "c1",
			"selected_track_id": "track_selected",
			"track_id":          "track_generic",
		}, "c1", "track_selected"},
		{"generic_track_last", map[string]any{
			"selected_clip_id": "c1", "track_id": "track_generic",
		}, "c1", "track_generic"},
		{"no_track_keys", map[string]any{"selected_clip_id": "c1"}, "c1", ""},
		{"blank_track_values", map[string]any{
			"selected_clip_id":          "c1",
			"piano_roll_focus_track_id": " ",
			"selected_clip_track_id":    "",
			"selected_track_id":         "  ",
		}, "c1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lesson := eduJamLessonContext(session, card, tc.ctx)
			raw, ok := lesson["selected_clip"].(map[string]any)
			if tc.clip == "" {
				if ok {
					t.Fatalf("selected_clip = %v, want absent", raw)
				}
				return
			}
			if !ok || raw["clip_id"] != tc.clip {
				t.Fatalf("selected_clip = %v, want clip %q", raw, tc.clip)
			}
			if tc.track == "" {
				if _, present := raw["track_id"]; present {
					t.Fatalf("track_id present without track selection: %v", raw)
				}
				return
			}
			if raw["track_id"] != tc.track {
				t.Fatalf("track_id = %v, want %q", raw["track_id"], tc.track)
			}
		})
	}
}

func TestInjectEduJamLessonContextGuards(t *testing.T) {
	card := &edu.LevelCard{ID: "guarded", Title: "guarded", Nodes: []edu.LevelNode{{ID: "n0", Title: "n0", Goal: "g"}}}
	session := eduJamLessonActiveSession("n0")
	// nil context: no panic, no write.
	injectEduJamLessonContext(nil, session, card)
	// nil card: nothing to inject.
	ctx := map[string]any{}
	injectEduJamLessonContext(ctx, session, nil)
	if _, ok := ctx[contextruntime.LessonContextKey]; ok {
		t.Fatalf("nil card injected lesson context: %v", ctx)
	}
	// card without nodes/assets/allowed: no content to inject.
	emptyCard := &edu.LevelCard{}
	injectEduJamLessonContext(ctx, session, emptyCard)
	if _, ok := ctx[contextruntime.LessonContextKey]; ok {
		t.Fatalf("content-free card injected lesson context: %v", ctx)
	}
}
