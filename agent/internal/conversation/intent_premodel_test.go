package conversation

import (
	"reflect"
	"testing"
)

// TestSynthesizeClipRangeSplitCommandsMatchesDispatch locks the invariant the
// pre-model takeover relies on: whenever SynthesizeClipRangeSplitCommands
// returns a plan, SynthesizeLocalDAWCommands must return exactly the same plan
// at its existing fallback position. A divergence here would mean the pre-LLM
// takeover re-routes an intent form that today reaches the model turn.
func TestSynthesizeClipRangeSplitCommandsMatchesDispatch(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		context map[string]any
	}{
		{"boxed range extraction", "把这段拆出来", rangeContextRows(boxedRangeRow())},
		{"boxed range split phrasing", "把框选的范围拆开", rangeContextRows(boxedRangeRow())},
		{"range touching clip start", "把这段拆出来", rangeContextRows(map[string]any{
			"range_id": "range_1", "clip_id": "clip_a", "track_id": "track_a",
			"start_seconds": 1.0, "end_seconds": 3.5,
			"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
		})},
		{"spoken seconds keep existing path", "在2.5秒切开这段", rangeContextRows(boxedRangeRow())},
		{"playhead keeps existing path", "在播放头这里切开这段", func() map[string]any {
			ctx := rangeContextRows(boxedRangeRow())
			ctx["playhead_seconds"] = "4.0"
			return ctx
		}()},
		{"no ranges falls through", "把这段拆出来", map[string]any{}},
		{"delete outranks split", "删除并把这段拆出来", rangeContextRows(boxedRangeRow())},
		// Documented compound-text behaviour: expandLocalIntentText appends the
		// "clip" alias for "这段", which disarms the add-track gate's clip
		// exclusion, so today's dispatch (and the mirror) land on the split plan.
		{"compound track add with 这段 falls to split", "新建一条轨道然后把这段拆出来", rangeContextRows(boxedRangeRow())},
		{"mute outranks split", "静音然后把这段拆出来", rangeContextRows(boxedRangeRow())},
		{"solo outranks split", "solo然后把这段拆出来", rangeContextRows(boxedRangeRow())},
		{"audio import outranks split", "把这段拆出来并导入音频到当前轨道", func() map[string]any {
			ctx := rangeContextRows(boxedRangeRow())
			ctx["selected_library_file_path"] = `D:\Samples\Loop.wav`
			return ctx
		}()},
		{"attachment import outranks split", "把这段拆出来", func() map[string]any {
			ctx := rangeContextRows(boxedRangeRow())
			ctx["attachment_import_kind"] = "audio"
			ctx["attachment_import_file_path"] = `D:\Samples\Loop.mp3`
			return ctx
		}()},
		{"midi import outranks split", "把这段拆出来", func() map[string]any {
			ctx := rangeContextRows(boxedRangeRow())
			ctx["attachment_import_kind"] = "midi"
			ctx["attachment_import_file_path"] = `D:\Samples\Part.mid`
			return ctx
		}()},
		{"malformed range fails open", "把这段拆出来", rangeContextRows(map[string]any{
			"clip_id": "clip_a", "track_id": "track_a",
			"start_seconds": "abc", "end_seconds": 3.5,
			"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
		})},
		{"whole clip range yields no cuts", "把这段拆出来", rangeContextRows(map[string]any{
			"range_id": "range_1", "clip_id": "clip_a", "track_id": "track_a",
			"start_seconds": 1.0, "end_seconds": 5.0,
			"clip_start_seconds": 1.0, "clip_end_seconds": 5.0,
		})},
	}
	for _, tc := range cases {
		preModel := SynthesizeClipRangeSplitCommands(tc.text, tc.context)
		dispatched := SynthesizeLocalDAWCommands(tc.text, tc.context)
		if len(preModel) == 0 {
			continue
		}
		if !reflect.DeepEqual(preModel, dispatched) {
			t.Fatalf("%s: pre-model plan %+v diverges from dispatch result %+v", tc.name, preModel, dispatched)
		}
	}
}

func TestSynthesizeClipRangeSplitCommandsHit(t *testing.T) {
	cmds := SynthesizeClipRangeSplitCommands("把这段拆出来", rangeContextRows(boxedRangeRow()))
	if len(cmds) != 2 {
		t.Fatalf("expected the two-cut plan, got %+v", cmds)
	}
	wantTimes := []float64{3.5, 2.0}
	for i, cmd := range cmds {
		if cmd["tool"] != "clip.split" {
			t.Fatalf("command %d tool = %v", i, cmd["tool"])
		}
		args := splitArgsOf(t, cmd)
		if args["split_time"] != wantTimes[i] {
			t.Fatalf("command %d split_time = %v, want %v (end cut must come first)", i, args["split_time"], wantTimes[i])
		}
	}
}

func TestSynthesizeClipRangeSplitCommandsBoundariesStayNil(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		context map[string]any
	}{
		{"no ranges", "把这段拆出来", map[string]any{}},
		{"spoken seconds", "在2.5秒切开这段", rangeContextRows(boxedRangeRow())},
		{"playhead mention", "在这里把这段拆出来", rangeContextRows(boxedRangeRow())},
		{"no split verb", "把这段放大", rangeContextRows(boxedRangeRow())},
		{"no range phrase", "切开这个片段", map[string]any{"selected_clip_ids": []any{"clip_c1"}}},
		{"delete precedence", "删除并把这段拆出来", rangeContextRows(boxedRangeRow())},
		// "框选" phrasing does not trigger the "这段/这个" clip alias in
		// expandLocalIntentText, so the add-track gate stays intact and keeps
		// its dispatch precedence over the split branch.
		{"track add precedence", "新建轨道把框选范围拆开", rangeContextRows(boxedRangeRow())},
		{"attachment import precedence", "把这段拆出来", func() map[string]any {
			ctx := rangeContextRows(boxedRangeRow())
			ctx["attachment_import_kind"] = "audio"
			ctx["attachment_import_file_path"] = `D:\Samples\Loop.mp3`
			return ctx
		}()},
		{"empty text", "  ", rangeContextRows(boxedRangeRow())},
	}
	for _, tc := range cases {
		if cmds := SynthesizeClipRangeSplitCommands(tc.text, tc.context); len(cmds) != 0 {
			t.Fatalf("%s: expected no pre-model takeover, got %+v", tc.name, cmds)
		}
	}
}
