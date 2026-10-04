package conversation

import (
	"reflect"
	"testing"
)

func boxedRangeRow() map[string]any {
	return map[string]any{
		"range_id":                 "range_1",
		"clip_id":                  "clip_a",
		"track_id":                 "track_a",
		"start_seconds":            2.0,
		"end_seconds":              3.5,
		"duration_seconds":         1.5,
		"clip_start_seconds":       1.0,
		"clip_end_seconds":         5.0,
		"clip_local_start_seconds": 1.0,
		"clip_local_end_seconds":   2.5,
		"source":                   "range_tool",
	}
}

func rangeContextRows(rows ...map[string]any) map[string]any {
	list := make([]any, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return map[string]any{"selected_clip_ranges": list}
}

func splitArgsOf(t *testing.T, cmd map[string]any) map[string]any {
	t.Helper()
	args, ok := cmd["args"].(map[string]any)
	if !ok {
		t.Fatalf("command args = %T (%+v)", cmd["args"], cmd)
	}
	return args
}

func TestClipRangeSplitCutsAtRangeBoundaries(t *testing.T) {
	cmds := SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(boxedRangeRow()))
	if len(cmds) != 2 {
		t.Fatalf("expected two split commands, got %+v", cmds)
	}
	// ClipService keeps the original clip_id on the left half after a split,
	// so the end cut must run before the start cut.
	wantTimes := []float64{3.5, 2.0}
	for i, cmd := range cmds {
		if cmd["tool"] != "clip.split" {
			t.Fatalf("command %d tool = %v", i, cmd["tool"])
		}
		args := splitArgsOf(t, cmd)
		if args["clip_id"] != "clip_a" || args["track_id"] != "track_a" {
			t.Fatalf("command %d clip/track = %v/%v", i, args["clip_id"], args["track_id"])
		}
		if args["time_unit"] != "seconds" {
			t.Fatalf("command %d time_unit = %v", i, args["time_unit"])
		}
		if args["split_time"] != wantTimes[i] {
			t.Fatalf("command %d split_time = %v, want %v", i, args["split_time"], wantTimes[i])
		}
	}
}

func TestClipRangeSplitWithoutRangesUnchanged(t *testing.T) {
	// Without ranges the extraction phrase keeps today's routing: expandLocalIntentText
	// appends the "this clip selected clip" alias, so it lands on clip.select, never split.
	cmds := SynthesizeLocalDAWCommands("把这段拆出来", map[string]any{})
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.select" {
		t.Fatalf("range extraction text without ranges must keep existing select routing, got %+v", cmds)
	}
	if args := splitArgsOf(t, cmds[0]); len(args) != 0 {
		t.Fatalf("select args must stay empty without ranges, got %+v", args)
	}
	cmds = SynthesizeLocalDAWCommands("切开这个片段", map[string]any{"selected_clip_ids": []any{"clip_c1"}})
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.split" {
		t.Fatalf("expected one clip.split command, got %+v", cmds)
	}
	args := splitArgsOf(t, cmds[0])
	if args["clip_id"] != "clip_c1" {
		t.Fatalf("clip selection unchanged expected, got %+v", args)
	}
	for _, key := range []string{"split_time", "time_unit", "selected_clip_range_count", "selected_clip_range"} {
		if _, ok := args[key]; ok {
			t.Fatalf("split without ranges/time must not carry %s: %+v", key, args)
		}
	}
}

func TestClipRangeSplitMalformedRangesIgnored(t *testing.T) {
	cases := map[string]map[string]any{
		"missing clip_id":     {"track_id": "track_a", "start_seconds": 2.0, "end_seconds": 3.5, "clip_start_seconds": 1.0, "clip_end_seconds": 5.0},
		"missing track_id":    {"clip_id": "clip_a", "start_seconds": 2.0, "end_seconds": 3.5, "clip_start_seconds": 1.0, "clip_end_seconds": 5.0},
		"non-numeric start":   {"clip_id": "clip_a", "track_id": "track_a", "start_seconds": "abc", "end_seconds": 3.5, "clip_start_seconds": 1.0, "clip_end_seconds": 5.0},
		"start after end":     {"clip_id": "clip_a", "track_id": "track_a", "start_seconds": 3.5, "end_seconds": 2.0, "clip_start_seconds": 1.0, "clip_end_seconds": 5.0},
		"missing clip bounds": {"clip_id": "clip_a", "track_id": "track_a", "start_seconds": 2.0, "end_seconds": 3.5},
		"range outside clip":  {"clip_id": "clip_a", "track_id": "track_a", "start_seconds": 6.0, "end_seconds": 7.0, "clip_start_seconds": 1.0, "clip_end_seconds": 5.0},
	}
	for name, row := range cases {
		cmds := SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(row))
		// Fail-open: the malformed range never yields a split; the phrase falls
		// through to today's select routing (alias-driven).
		if len(cmds) != 1 || cmds[0]["tool"] != "clip.select" {
			t.Fatalf("%s: malformed range must fail open to existing routing, got %+v", name, cmds)
		}
	}
}

func TestClipRangeSplitMultipleRangesUsesFirst(t *testing.T) {
	second := map[string]any{
		"range_id": "range_2", "clip_id": "clip_b", "track_id": "track_b",
		"start_seconds": 8.0, "end_seconds": 9.0,
		"clip_start_seconds": 7.0, "clip_end_seconds": 10.0,
	}
	cmds := SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(boxedRangeRow(), second))
	if len(cmds) != 2 {
		t.Fatalf("expected two split commands, got %+v", cmds)
	}
	args := splitArgsOf(t, cmds[0])
	if args["clip_id"] != "clip_a" || args["split_time"] != 3.5 {
		t.Fatalf("first range must win, got %+v", args)
	}
}

func TestClipRangeSplitSpokenTimeWins(t *testing.T) {
	cmds := SynthesizeLocalDAWCommands("在2.5秒切开这段", rangeContextRows(boxedRangeRow()))
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.split" {
		t.Fatalf("expected single clip.split command, got %+v", cmds)
	}
	args := splitArgsOf(t, cmds[0])
	if args["split_time"] != 2.5 || args["time_unit"] != "seconds" {
		t.Fatalf("spoken seconds must win over range bounds, got %+v", args)
	}
	if args["clip_id"] != "clip_a" || args["selected_clip_range_count"] != 1 {
		t.Fatalf("range clip id should backfill selection with count annotation, got %+v", args)
	}
}

func TestClipRangeSplitBoundaryRanges(t *testing.T) {
	rangeAtStart := map[string]any{
		"range_id": "range_1", "clip_id": "clip_a", "track_id": "track_a",
		"start_seconds": 1.0, "end_seconds": 3.5,
		"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
	}
	cmds := SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(rangeAtStart))
	if len(cmds) != 1 || splitArgsOf(t, cmds[0])["split_time"] != 3.5 {
		t.Fatalf("range touching clip start must cut once at range end, got %+v", cmds)
	}
	rangeAtEnd := map[string]any{
		"range_id": "range_1", "clip_id": "clip_a", "track_id": "track_a",
		"start_seconds": 2.0, "end_seconds": 5.0,
		"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
	}
	cmds = SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(rangeAtEnd))
	if len(cmds) != 1 || splitArgsOf(t, cmds[0])["split_time"] != 2.0 {
		t.Fatalf("range touching clip end must cut once at range start, got %+v", cmds)
	}
	wholeClip := map[string]any{
		"range_id": "range_1", "clip_id": "clip_a", "track_id": "track_a",
		"start_seconds": 1.0, "end_seconds": 5.0,
		"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
	}
	// A range covering the whole clip gives the cut plan nothing to do; the
	// phrase falls through to today's select routing instead of splitting.
	cmds = SynthesizeLocalDAWCommands("把这段拆出来", rangeContextRows(wholeClip))
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.select" {
		t.Fatalf("whole-clip range must keep existing routing without split, got %+v", cmds)
	}
}

func TestClipRangeSplitPlayheadKeepsExistingPath(t *testing.T) {
	requestContext := rangeContextRows(boxedRangeRow())
	requestContext["playhead_seconds"] = "4.0"
	cmds := SynthesizeLocalDAWCommands("在播放头切开这段", requestContext)
	if len(cmds) != 1 || cmds[0]["tool"] != "clip.split" {
		t.Fatalf("expected existing single split form, got %+v", cmds)
	}
	args := splitArgsOf(t, cmds[0])
	if args["playhead_seconds"] != "4.0" {
		t.Fatalf("playhead split form expected, got %+v", args)
	}
	if _, ok := args["split_time"]; ok {
		t.Fatalf("range cut plan must not engage for playhead-directed split: %+v", args)
	}
}

func TestSelectedClipArgsConsumesRanges(t *testing.T) {
	args := selectedClipArgs(rangeContextRows(boxedRangeRow()), false)
	if args["clip_id"] != "clip_a" || args["selected_clip_range_count"] != 1 {
		t.Fatalf("range should backfill clip selection and annotate count: %+v", args)
	}
	first, ok := args["selected_clip_range"].(map[string]any)
	if !ok || first["range_id"] != "range_1" {
		t.Fatalf("first range row expected in args: %+v", args)
	}
	args = selectedClipArgs(map[string]any{
		"selected_clip_ids":    []any{"clip_c1", "clip_c2"},
		"selected_clip_ranges": []any{boxedRangeRow(), boxedRangeRow()},
	}, true)
	if !reflect.DeepEqual(args["clip_ids"], []string{"clip_c1", "clip_c2"}) {
		t.Fatalf("explicit ids must win over ranges: %+v", args)
	}
	if args["selected_clip_range_count"] != 2 {
		t.Fatalf("range count annotation expected: %+v", args)
	}
	args = selectedClipArgs(map[string]any{"selected_clip_ids": []any{"clip_c1"}}, false)
	if _, ok := args["selected_clip_range_count"]; ok {
		t.Fatalf("no ranges must leave args unchanged: %+v", args)
	}
	if args["clip_id"] != "clip_c1" {
		t.Fatalf("single id selection unchanged: %+v", args)
	}
	malformedOnly := map[string]any{"selected_clip_ranges": []any{map[string]any{"track_id": "track_a"}}}
	args = selectedClipArgs(malformedOnly, false)
	if _, ok := args["selected_clip_range_count"]; ok {
		t.Fatalf("malformed ranges must fail open without annotation: %+v", args)
	}
}
