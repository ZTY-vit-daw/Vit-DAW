package contextruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
)

func fixedOptions() Options {
	return Options{
		RecentTurns:            4,
		RecentTraceEvents:      4,
		MaxTextRunes:           80,
		MaxListItems:           5,
		SkipPluginSemanticLoad: true,
		Now: func() time.Time {
			return time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
		},
	}
}

func TestBuildCompactsLongConversationAndKeepsRecentTurns(t *testing.T) {
	var history []llm.Message
	for i := 0; i < 12; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, llm.Message{Role: role, Content: strings.Repeat("turn", 30) + string(rune('A'+i))})
	}
	snap := Build(Input{ConversationID: "chat_1", Conversation: history}, fixedOptions())
	if snap.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %q", snap.SchemaVersion)
	}
	if len(snap.RecentTurns) != 4 {
		t.Fatalf("recent turns = %d", len(snap.RecentTurns))
	}
	if snap.ConversationSummary["message_count"] != 12 || snap.ConversationSummary["omitted_message_count"] != 8 {
		t.Fatalf("summary = %+v", snap.ConversationSummary)
	}
	if !strings.Contains(snap.RecentTurns[0].Content, "<truncated>") {
		t.Fatalf("recent content should be compacted: %q", snap.RecentTurns[0].Content)
	}
}

func TestBuildSummarizesToolResultsWithoutDroppingIDs(t *testing.T) {
	trace := []planner.TraceEvent{
		{
			Kind: "tool_result",
			ToolResult: &planner.ToolResult{
				ToolCallID: "tool_1",
				Tool:       "workspace.read_file",
				Status:     "error",
				Error:      "permission denied",
				Result: map[string]any{
					"track_id":  "track_7",
					"plugin_id": "plugin_a",
					"file_path": "D:/Vit_DAW/huge.txt",
					"content":   strings.Repeat("x", 400),
				},
			},
		},
	}
	snap := Build(Input{GoalTrace: trace}, fixedOptions())
	if len(snap.ToolResultSummary) != 1 {
		t.Fatalf("tool summaries = %+v", snap.ToolResultSummary)
	}
	row := snap.ToolResultSummary[0]
	for key, want := range map[string]any{"tool_call_id": "tool_1", "tool": "workspace.read_file", "status": "error", "error": "permission denied"} {
		if row[key] != want {
			t.Fatalf("%s = %v want %v in %+v", key, row[key], want, row)
		}
	}
	important, ok := row["important_fields"].(map[string]any)
	if !ok || important["track_id"] != "track_7" || important["plugin_id"] != "plugin_a" {
		t.Fatalf("important fields = %+v", row["important_fields"])
	}
	if snap.GoalTraceSummary["error_count"] != 1 {
		t.Fatalf("trace summary = %+v", snap.GoalTraceSummary)
	}
	if snap.DAWSemanticSummary["recent_execution_result"] == nil {
		t.Fatalf("daw semantic summary missing recent execution result: %+v", snap.DAWSemanticSummary)
	}
}

func TestTraceSummaryIncludesPlanItemsAndVerification(t *testing.T) {
	trace := []planner.TraceEvent{
		{
			Kind: "tool_call",
			ToolCall: &planner.ToolCall{
				ID:         "call_1",
				Tool:       "plugin.load_to_rack",
				PlanItemID: "load_eq",
				Args:       map[string]any{"track_id": "track_1", "plugin_path": `C:\TDR Nova.vst3`},
			},
		},
		{
			Kind: "verification",
			Verification: &planner.VerificationResult{
				ToolCallID:    "call_1",
				PlanItemID:    "load_eq",
				Tool:          "plugin.load_to_rack",
				CommandName:   "rack_add_node",
				Status:        "verified",
				Postcondition: "rack_node_present",
				Message:       "observed rack node",
				Evidence:      map[string]any{"observed_node_id": "node_1"},
			},
		},
		{
			Kind:      "plan_update",
			PlanItems: []planner.PlanItem{{ID: "load_eq", Description: "Load EQ", Status: "completed", Evidence: "observed rack node"}},
		},
	}
	snap := Build(Input{GoalTrace: trace}, fixedOptions())
	events, ok := snap.GoalTraceSummary["recent_events"].([]map[string]any)
	if !ok || len(events) != 3 {
		t.Fatalf("recent events = %+v", snap.GoalTraceSummary["recent_events"])
	}
	call := events[0]["tool_call"].(map[string]any)
	if call["plan_item_id"] != "load_eq" {
		t.Fatalf("tool call missing plan item id: %+v", call)
	}
	verification := events[1]["verification"].(map[string]any)
	if verification["status"] != "verified" || verification["plan_item_id"] != "load_eq" {
		t.Fatalf("verification missing fields: %+v", verification)
	}
	items := events[2]["plan_items"].([]map[string]any)
	if len(items) != 1 || items[0]["status"] != "completed" {
		t.Fatalf("plan items missing fields: %+v", items)
	}
	if snap.RecentGoalContext["recent_verification"] == nil || snap.RecentGoalContext["plan_steps"] == nil {
		t.Fatalf("recent goal context missing plan/verification: %+v", snap.RecentGoalContext)
	}
}

func TestToolImportantFieldsAreDeterministic(t *testing.T) {
	opts := fixedOptions()
	opts.MaxListItems = 2
	result := planner.ToolResult{
		ToolCallID: "tool_1",
		Tool:       "workspace.read_file",
		Status:     "ok",
		Result: map[string]any{
			"z_track_id":  "track_z",
			"a_clip_id":   "clip_a",
			"m_plugin_id": "plugin_m",
			"content":     strings.Repeat("x", 400),
		},
	}
	var first string
	for i := 0; i < 25; i++ {
		snap := Build(Input{ToolResults: []planner.ToolResult{result}}, opts)
		important := snap.ToolResultSummary[0]["important_fields"]
		b, err := json.Marshal(important)
		if err != nil {
			t.Fatalf("marshal important fields: %v", err)
		}
		if first == "" {
			first = string(b)
			continue
		}
		if string(b) != first {
			t.Fatalf("important fields changed: first=%s next=%s", first, string(b))
		}
	}
	if first != `{"a_clip_id":"clip_a","m_plugin_id":"plugin_m"}` {
		t.Fatalf("important fields = %s", first)
	}
}

func TestToolResultPreviewBudgetSummarizesLargeNestedPayload(t *testing.T) {
	opts := fixedOptions()
	opts.MaxPreviewBytes = 1200
	rows := make([]any, 0, 8)
	for i := 0; i < 8; i++ {
		row := map[string]any{"id": fmt.Sprintf("param_%d", i)}
		for j := 0; j < 40; j++ {
			row[fmt.Sprintf("field_%02d", j)] = "verbose parameter field " + strings.Repeat("x", 80)
		}
		rows = append(rows, row)
	}
	snap := Build(Input{ToolResults: []planner.ToolResult{{
		ToolCallID: "tool_1",
		Tool:       "plugin.get_parameters",
		Status:     "ok",
		Result: map[string]any{
			"track_id":         "track_7",
			"plugin_id":        "plugin_a",
			"plugin_name":      "TDR Nova",
			"parameter_count":  len(rows),
			"parameter_values": rows,
		},
	}}}, opts)
	row := snap.ToolResultSummary[0]
	preview, ok := row["result_preview"].(map[string]any)
	if !ok {
		t.Fatalf("result preview = %+v", row["result_preview"])
	}
	if preview["preview_omitted"] == nil || preview["track_id"] != "track_7" || preview["plugin_id"] != "plugin_a" {
		t.Fatalf("large preview summary lost key fields: %+v", preview)
	}
	raw, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 2500 || strings.Contains(string(raw), "verbose parameter field") {
		t.Fatalf("large tool result leaked into summary len=%d text=%s", len(raw), string(raw))
	}
}

func TestPluginContextPackKeepsCountsAndDropsFullParameterList(t *testing.T) {
	pack := map[string]any{
		"schema_version":      "plugin_grabber_context_pack.v1",
		"context_strategy":    "summary_first_full_parameters_available",
		"track_id":            "track_7",
		"plugin_id":           "plugin_a",
		"plugin_name":         "TDR Nova",
		"parameters_retained": true,
		"parameter_count":     42,
		"quick_controls": []any{
			map[string]any{"param_id": "threshold", "label": "Threshold", "display_group": "Dynamics", "normalized_role": "comp_threshold"},
		},
		"groups": []any{
			map[string]any{"name": "Dynamics", "parameter_count": 12, "sample_param_ids": []any{"threshold"}},
		},
		"role_summary": []any{
			map[string]any{"normalized_role": "comp_threshold", "parameter_count": 3},
		},
		"parameters": []any{
			map[string]any{"id": "threshold"},
			map[string]any{"id": "ratio"},
		},
	}
	snap := Build(Input{PluginContextPack: pack}, fixedOptions())
	if snap.PluginContextSummary["parameter_count"] != 42 || snap.PluginContextSummary["parameters_retained"] != true {
		t.Fatalf("plugin context summary = %+v", snap.PluginContextSummary)
	}
	if _, ok := snap.PluginContextSummary["parameters"]; ok {
		t.Fatalf("full parameters leaked into snapshot: %+v", snap.PluginContextSummary)
	}
	if len(snap.Warnings) == 0 || !strings.Contains(snap.Warnings[0], "full parameters") {
		t.Fatalf("expected full parameter warning, got %+v", snap.Warnings)
	}
}

func TestSelectionDAWAndSemanticSummaryAreStable(t *testing.T) {
	semantic := map[string]any{
		"index_exists": true,
		"index_path":   "D:/index.json",
		"type_counts":  map[string]any{"eq": 2},
	}
	snap := Build(Input{
		Context: map[string]any{
			"selected_track_id":  "track_7",
			"selected_clip_ids":  []any{"clip_a", "clip_b"},
			"selected_plugin_id": "plugin_a",
			"playhead_seconds":   12.5,
		},
		State: map[string]any{
			"initialized":  true,
			"project_path": "D:/song.vit",
			"tracks": []any{
				map[string]any{
					"track_id":   "track_7",
					"track_name": "Lead",
					"clips":      []any{map[string]any{"id": "clip_a", "name": "MIDI"}},
					"plugins":    []any{map[string]any{"plugin_id": "plugin_a", "plugin_name": "TDR Nova"}},
				},
			},
		},
		PluginSemanticSummary: semantic,
	}, fixedOptions())
	if snap.CurrentSelection["selected_track_id"] != "track_7" || len(snap.CurrentSelection["selected_clip_ids"].([]string)) != 2 {
		t.Fatalf("selection = %+v", snap.CurrentSelection)
	}
	if snap.DAWStateSummary["project_path"] != "D:/song.vit" {
		t.Fatalf("daw summary = %+v", snap.DAWStateSummary)
	}
	if snap.DAWSemanticSummary["current_selection"] == nil || snap.DAWSemanticSummary["visible_tracks"] == nil || snap.DAWSemanticSummary["clip_time_ranges"] == nil {
		t.Fatalf("daw semantic summary = %+v", snap.DAWSemanticSummary)
	}
	loaded, ok := snap.DAWStateSummary["loaded_plugins"].([]map[string]any)
	if !ok || len(loaded) != 1 || loaded[0]["plugin_id"] != "plugin_a" {
		t.Fatalf("loaded plugins = %+v", snap.DAWStateSummary["loaded_plugins"])
	}
	lib, ok := snap.PluginContextSummary["semantic_library"].(map[string]any)
	if !ok || lib["index_exists"] != true {
		t.Fatalf("semantic library = %+v", snap.PluginContextSummary)
	}
}

func TestDAWSummaryIncludesRackNodesAsLoadedPlugins(t *testing.T) {
	snap := Build(Input{
		State: map[string]any{
			"initialized": true,
			"tracks": []any{
				map[string]any{
					"track_id":   "track_7",
					"track_name": "Lead",
					"plugins":    []any{map[string]any{"plugin_item_id": "rack_1", "name": "New Rack"}},
					"rack": map[string]any{
						"rack_item_id": "rack_1",
						"nodes": []any{
							map[string]any{"node_id": "nova_1", "name": "TDR Nova", "template_role": "eq", "zone_id": "Z3"},
							map[string]any{"node_id": "verb_1", "name": "ValhallaSupermassive", "template_role": "reverb", "zone_id": "Z3"},
						},
					},
				},
			},
		},
	}, fixedOptions())
	tracks := snap.DAWStateSummary["tracks"].([]map[string]any)
	if tracks[0]["rack_node_count"] != 2 {
		t.Fatalf("track summary did not include rack nodes: %+v", tracks[0])
	}
	loaded := snap.DAWStateSummary["loaded_plugins"].([]map[string]any)
	var sawNova, sawVerb bool
	for _, row := range loaded {
		if row["plugin_name"] == "TDR Nova" && row["source"] == "rack_node" {
			sawNova = true
		}
		if row["plugin_name"] == "ValhallaSupermassive" && row["source"] == "rack_node" {
			sawVerb = true
		}
	}
	if !sawNova || !sawVerb {
		t.Fatalf("loaded plugins should include rack nodes: %+v", loaded)
	}
}

func TestSnapshotJSONLPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent_context_snapshots.jsonl")
	snap := Build(Input{ConversationID: "chat_1", UserText: "hello"}, fixedOptions())
	if _, err := json.Marshal(snap.Map()); err != nil {
		t.Fatalf("snapshot map must be JSON serializable: %v", err)
	}
	if err := AppendJSONL(path, snap); err != nil {
		t.Fatalf("append snapshot: %v", err)
	}
	next := Build(Input{ConversationID: "chat_2", UserText: "world"}, fixedOptions())
	if err := AppendJSONL(path, next); err != nil {
		t.Fatalf("append second snapshot: %v", err)
	}
	all, err := ReadJSONL(path, 0)
	if err != nil {
		t.Fatalf("read snapshots: %v", err)
	}
	if len(all) != 2 || all[0].ConversationID != "chat_1" || all[1].ConversationID != "chat_2" {
		t.Fatalf("snapshots = %+v", all)
	}
	limited, err := ReadJSONL(path, 1)
	if err != nil {
		t.Fatalf("read limited snapshots: %v", err)
	}
	if len(limited) != 1 || limited[0].ConversationID != "chat_2" {
		t.Fatalf("limited snapshots = %+v", limited)
	}
}

func TestDefaultSnapshotPathFindsVitAppFromNestedWorkdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "agent"), 0o755); err != nil {
		t.Fatalf("mkdir agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "agent", "go.mod"), []byte("module test\n"), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nested := filepath.Join(root, "VitApp", "Workspace")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir nested: %v", err)
	}
	want := filepath.Join(root, "VitApp", "Workspace", "Logs", "agent_context_snapshots.jsonl")
	if got := DefaultSnapshotPath(); got != want {
		t.Fatalf("DefaultSnapshotPath() = %q want %q", got, want)
	}
}
