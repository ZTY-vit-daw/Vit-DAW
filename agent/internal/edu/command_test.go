package edu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeKernel 伪造 analyze_midi_transformations 依赖的三条内核只读命令。
type fakeKernel struct {
	state    map[string]any
	clip     map[string]any
	notes    map[string]any
	stateErr error
	clipErr  error
	notesErr error
}

func (f fakeKernel) send(_ context.Context, cmd map[string]any) (map[string]any, error) {
	switch cmd["cmd"] {
	case "get_project_state":
		return f.state, f.stateErr
	case "get_midi_clip_data":
		return f.clip, f.clipErr
	case "get_midi_clip_notes":
		return f.notes, f.notesErr
	default:
		return nil, fmt.Errorf("unexpected kernel command: %v", cmd["cmd"])
	}
}

// twinkleNotesReply 把 42 音小星星 fixture 组装成 get_midi_clip_notes 回复。
func twinkleNotesReply() map[string]any {
	notes := twinkleNotes()
	rows := make([]any, len(notes))
	for i, n := range notes {
		rows[i] = map[string]any{
			"id": n.ID, "pitch": float64(n.Pitch),
			"start": n.Start, "length": n.Length,
			"velocity": 80.0, "channel": 0.0,
		}
	}
	return map[string]any{
		"status": "ok", "clip_id": "clipA", "track_id": "track1",
		"time_unit": "beats", "notes": rows,
	}
}

func stdMusicContext() map[string]any {
	return map[string]any{"status": "ok", "music_context": map[string]any{
		"tempo_bpm": 100.0, "time_signature": "4/4",
		"time_signature_numerator": 4.0, "time_signature_denominator": 4.0,
		"tempo_setting_count": 1.0, "time_sig_setting_count": 1.0,
		"key_signature": map[string]any{"root_pitch": 60.0, "root_name": "C", "scale": "major"},
	}}
}

func stdClipData() map[string]any {
	return map[string]any{"status": "ok", "clip_id": "clipA", "track_id": "track1",
		"start_seconds": 0.0, "length_seconds": 28.8}
}

func stdFakeKernel() fakeKernel {
	return fakeKernel{state: stdMusicContext(), clip: stdClipData(), notes: twinkleNotesReply()}
}

func runAnalyze(t *testing.T, fk fakeKernel, args map[string]any) map[string]any {
	t.Helper()
	out, err := AnalyzeTransformationsCommand(context.Background(), fk.send, args)
	if err != nil {
		t.Fatalf("AnalyzeTransformationsCommand error: %v", err)
	}
	return out
}

func findingsList(t *testing.T, out map[string]any) []map[string]any {
	t.Helper()
	raw, ok := out["findings"].([]any)
	if !ok {
		t.Fatalf("findings is not a list: %T", out["findings"])
	}
	fs := make([]map[string]any, len(raw))
	for i, r := range raw {
		fs[i] = r.(map[string]any)
	}
	return fs
}

func segmentOf(t *testing.T, f map[string]any, role string) map[string]any {
	t.Helper()
	segs := f["segments"].([]any)
	for _, s := range segs {
		seg := s.(map[string]any)
		if seg["role"] == role {
			return seg
		}
	}
	t.Fatalf("segment role %q not found", role)
	return nil
}

func number(t *testing.T, v any) float64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("expected number, got %T (%v)", v, v)
	}
	return f
}

// 黄金用例（命令面）：小星星 42 音 + C 大调 key，检出与检测核完全一致的
// findings，bar 锚定正确（乐句2 beats 8-16 = 第 3-4 小节，乐句3 beats
// 16-24 = 第 5-6 小节），diatonic sequence 带 sequence_quality/label。
func TestAnalyzeCommandGoldenWiringAndBars(t *testing.T) {
	out := runAnalyze(t, stdFakeKernel(), map[string]any{
		"clip_id": "clipA", "key": map[string]any{"tonic": 0.0, "mode": "major"},
	})

	if out["status"] != "ok" {
		t.Fatalf("status = %v", out["status"])
	}
	if out["schema_version"] != AnalyzeTransformationsSchemaVersion {
		t.Fatalf("schema_version = %v", out["schema_version"])
	}
	if out["time_unit"] != "beats" || out["clip_id"] != "clipA" || out["track_id"] != "track1" {
		t.Fatalf("echo fields wrong: %v / %v / %v", out["time_unit"], out["clip_id"], out["track_id"])
	}
	if number(t, out["analyzed_note_count"]) != 42 {
		t.Fatalf("analyzed_note_count = %v", out["analyzed_note_count"])
	}

	// 与检测核接线：findings 与 DetectTransformations 直接调用逐条对齐。
	core, err := DetectTransformations(twinkleNotes(), DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("core detect: %v", err)
	}
	cmdFindings := findingsList(t, out)
	if len(cmdFindings) != len(core) {
		t.Fatalf("command findings %d != core findings %d", len(cmdFindings), len(core))
	}
	for i, cf := range cmdFindings {
		kf := core[i]
		if cf["kind"] != string(kf.Kind) {
			t.Fatalf("finding %d kind %v != %v", i, cf["kind"], kf.Kind)
		}
		if number(t, cf["shift_semitones"]) != float64(kf.ShiftSemitones) {
			t.Fatalf("finding %d shift %v != %d", i, cf["shift_semitones"], kf.ShiftSemitones)
		}
		proto, match := segmentOf(t, cf, "prototype"), segmentOf(t, cf, "match")
		if number(t, proto["start_beats"]) != kf.Prototype.StartBeats || number(t, match["start_beats"]) != kf.Match.StartBeats {
			t.Fatalf("finding %d segment beats misaligned: %v/%v vs %v/%v",
				i, proto["start_beats"], match["start_beats"], kf.Prototype.StartBeats, kf.Match.StartBeats)
		}
		protoIDs := joinIDs(proto["note_ids"].([]any))
		if protoIDs != strings.Join(kf.Prototype.NoteIDs, ",") {
			t.Fatalf("finding %d note_ids %v != %v", i, protoIDs, kf.Prototype.NoteIDs)
		}
	}

	// 黄金定位：乐句2↔乐句3 调内模进（proto beats 8-16 → match beats 16-24，
	// shift 2 / diatonic / bar 3-4 → 5-6）。用 proto+match 双起点钉住，避免
	// 撞上其他同样从 beat 8 出发的短匹配（如乐句2头↔乐句6中段的 steps=-1）。
	var diatonic map[string]any
	for _, f := range cmdFindings {
		if f["kind"] == "sequence" && f["sequence_quality"] == "diatonic" &&
			number(t, segmentOf(t, f, "prototype")["start_beats"]) == 8.0 &&
			number(t, segmentOf(t, f, "match")["start_beats"]) == 16.0 {
			diatonic = f
		}
	}
	if diatonic == nil {
		t.Fatal("diatonic sequence phrase2->phrase3 not found")
	}
	if number(t, diatonic["shift_semitones"]) != 2 || number(t, diatonic["diatonic_steps"]) != 1 {
		t.Fatalf("diatonic shift/steps = %v/%v", diatonic["shift_semitones"], diatonic["diatonic_steps"])
	}
	if diatonic["label"] != "模进（调内，上移 2 半音）" {
		t.Fatalf("diatonic label = %v", diatonic["label"])
	}
	proto, match := segmentOf(t, diatonic, "prototype"), segmentOf(t, diatonic, "match")
	if number(t, proto["bar_start"]) != 3 || number(t, proto["bar_end"]) != 4 {
		t.Fatalf("diatonic proto bars = %v-%v, want 3-4", proto["bar_start"], proto["bar_end"])
	}
	if number(t, match["bar_start"]) != 5 || number(t, match["bar_end"]) != 6 {
		t.Fatalf("diatonic match bars = %v-%v, want 5-6", match["bar_start"], match["bar_end"])
	}

	// 乐句3↔乐句4 原样重复严格区分（proto beats 16-24 → match beats 24-32，
	// bar 5-6 → 7-8，shift 0）。
	var repeat map[string]any
	for _, f := range cmdFindings {
		if f["kind"] == "repeat" &&
			number(t, segmentOf(t, f, "prototype")["start_beats"]) == 16.0 &&
			number(t, segmentOf(t, f, "match")["start_beats"]) == 24.0 {
			repeat = f
		}
	}
	if repeat == nil {
		t.Fatal("repeat phrase3->phrase4 not found")
	}
	if _, has := repeat["sequence_quality"]; has {
		t.Fatalf("repeat must not carry sequence_quality: %v", repeat)
	}
	if repeat["label"] != "原样重复" {
		t.Fatalf("repeat label = %v", repeat["label"])
	}
	proto, match = segmentOf(t, repeat, "prototype"), segmentOf(t, repeat, "match")
	if number(t, proto["bar_start"]) != 5 || number(t, proto["bar_end"]) != 6 ||
		number(t, match["bar_start"]) != 7 || number(t, match["bar_end"]) != 8 {
		t.Fatalf("repeat bars = %v-%v -> %v-%v, want 5-6 -> 7-8",
			proto["bar_start"], proto["bar_end"], match["bar_start"], match["bar_end"])
	}

	// EPM 惯例字段：confidence 双字段 + requires_confirmation 恒 false + basis。
	if diatonic["confidence"] != "high" || number(t, diatonic["confidence_score"]) != 0.99 {
		t.Fatalf("confidence = %v/%v", diatonic["confidence"], diatonic["confidence_score"])
	}
	if diatonic["requires_confirmation"] != false {
		t.Fatalf("requires_confirmation = %v", diatonic["requires_confirmation"])
	}
	basis := joinIDs(diatonic["basis"].([]any))
	for _, want := range []string{"interval_pattern_exact_match", "rhythm_exact_match", "diatonic_scale_degree_match"} {
		if !strings.Contains(basis, want) {
			t.Fatalf("diatonic basis missing %q in %v", want, basis)
		}
	}

	// music_context_echo + clip_origin 回显换算依据。
	echo := out["music_context_echo"].(map[string]any)
	if number(t, echo["tempo_bpm"]) != 100 || number(t, echo["tempo_setting_count"]) != 1 ||
		echo["time_signature"] != "4/4" || number(t, echo["time_signature_numerator"]) != 4 {
		t.Fatalf("music_context_echo = %v", echo)
	}
	if number(t, out["clip_origin"].(map[string]any)["start_beats_global"]) != 0 {
		t.Fatalf("clip_origin = %v", out["clip_origin"])
	}
	// 顶层 limitations 恒含单声部假设披露。
	lims := joinIDs(out["limitations"].([]any))
	if !strings.Contains(lims, "单声部假设") {
		t.Fatalf("limitations missing single-voice assumption: %v", lims)
	}
}

func joinIDs(raw []any) string {
	parts := make([]string, len(raw))
	for i, v := range raw {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ",")
}

// bar 守门三态 + 两个扩展降级态：bar 字段只在换算依据齐全时出现，其余
// null + limitations 披露原因；findings 本身不受守门影响。
func TestAnalyzeCommandBarGateStates(t *testing.T) {
	args := map[string]any{"clip_id": "clipA", "key": map[string]any{"tonic": 0.0}}

	t.Run("usable single tempo", func(t *testing.T) {
		out := runAnalyze(t, stdFakeKernel(), args)
		for _, f := range findingsList(t, out) {
			for _, role := range []string{"prototype", "match"} {
				seg := segmentOf(t, f, role)
				if seg["bar_start"] == nil || seg["bar_end"] == nil {
					t.Fatalf("segment %s bars nil: %v", role, seg)
				}
			}
		}
	})

	t.Run("multi tempo null bars", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.state = map[string]any{"status": "ok", "music_context": map[string]any{
			"tempo_bpm": 100.0, "tempo_setting_count": 3.0,
			"time_signature": "4/4", "time_signature_numerator": 4.0, "time_signature_denominator": 4.0,
		}}
		out := runAnalyze(t, fk, args)
		if len(findingsList(t, out)) == 0 {
			t.Fatal("findings must survive bar gating")
		}
		for _, f := range findingsList(t, out) {
			for _, role := range []string{"prototype", "match"} {
				seg := segmentOf(t, f, role)
				if seg["bar_start"] != nil || seg["bar_end"] != nil {
					t.Fatalf("segment %s bars must be null: %v", role, seg)
				}
			}
		}
		if out["clip_origin"].(map[string]any)["start_beats_global"] != nil {
			t.Fatalf("clip_origin must be null under multi tempo: %v", out["clip_origin"])
		}
		lims := joinIDs(out["limitations"].([]any))
		if !strings.Contains(lims, "tempo_setting_count=3") {
			t.Fatalf("limitations missing multi-tempo reason: %v", lims)
		}
	})

	t.Run("missing music context null bars", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.state = map[string]any{"status": "ok"}
		out := runAnalyze(t, fk, args)
		for _, f := range findingsList(t, out) {
			if segmentOf(t, f, "prototype")["bar_start"] != nil {
				t.Fatal("bars must be null without music_context")
			}
		}
		echo := out["music_context_echo"].(map[string]any)
		if echo["tempo_bpm"] != nil || echo["tempo_setting_count"] != nil {
			t.Fatalf("echo should be null without music_context: %v", echo)
		}
		lims := joinIDs(out["limitations"].([]any))
		if !strings.Contains(lims, "music_context") {
			t.Fatalf("limitations missing music_context reason: %v", lims)
		}
	})

	t.Run("multi time signature null bars", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.state = map[string]any{"status": "ok", "music_context": map[string]any{
			"tempo_bpm": 100.0, "tempo_setting_count": 1.0,
			"time_signature": "4/4", "time_signature_numerator": 4.0, "time_signature_denominator": 4.0,
			"time_sig_setting_count": 2.0,
		}}
		out := runAnalyze(t, fk, args)
		if segmentOf(t, findingsList(t, out)[0], "prototype")["bar_start"] != nil {
			t.Fatal("bars must be null under multi time signature")
		}
		lims := joinIDs(out["limitations"].([]any))
		if !strings.Contains(lims, "time_sig_setting_count=2") {
			t.Fatalf("limitations missing multi-time-sig reason: %v", lims)
		}
	})

	t.Run("clip data failure null bars", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.clipErr = fmt.Errorf("socket closed")
		out := runAnalyze(t, fk, args)
		if segmentOf(t, findingsList(t, out)[0], "prototype")["bar_start"] != nil {
			t.Fatal("bars must be null without clip origin")
		}
		lims := joinIDs(out["limitations"].([]any))
		if !strings.Contains(lims, "clip 全局起点不可得") {
			t.Fatalf("limitations missing clip-origin reason: %v", lims)
		}
	})
}

// clip 全局起点非零时的 bar 换算：clip start_seconds=7.2 @100bpm → 12 beats，
// 乐句2 全局 beats 20-28 → 第 6-7 小节。
func TestAnalyzeCommandClipOriginConversion(t *testing.T) {
	fk := stdFakeKernel()
	fk.clip = map[string]any{"status": "ok", "clip_id": "clipA", "track_id": "track1", "start_seconds": 7.2}
	out := runAnalyze(t, fk, map[string]any{"clip_id": "clipA", "key": map[string]any{"tonic": 0.0}})
	if got := number(t, out["clip_origin"].(map[string]any)["start_beats_global"]); got != 12.0 {
		t.Fatalf("clip origin = %v, want 12.0", got)
	}
	for _, f := range findingsList(t, out) {
		if f["kind"] == "sequence" && f["sequence_quality"] == "diatonic" && number(t, segmentOf(t, f, "prototype")["start_beats"]) == 8.0 {
			proto := segmentOf(t, f, "prototype")
			if number(t, proto["bar_start"]) != 6 || number(t, proto["bar_end"]) != 7 {
				t.Fatalf("shifted proto bars = %v-%v, want 6-7", proto["bar_start"], proto["bar_end"])
			}
			return
		}
	}
	t.Fatal("diatonic finding not found for origin conversion check")
}

// 返回 schema 序列化：顶层与嵌套键名与 256 草案 + 段 1 追认逐键核对。
func TestAnalyzeCommandSchemaFieldNames(t *testing.T) {
	out := runAnalyze(t, stdFakeKernel(), map[string]any{
		"clip_id": "clipA", "key": map[string]any{"tonic": 0.0},
	})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"status", "analysis_id", "clip_id", "track_id", "time_unit",
		"music_context_echo", "clip_origin", "findings", "llm_context", "limitations"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("top-level key %q missing", key)
		}
	}
	var echo map[string]json.RawMessage
	if err := json.Unmarshal(doc["music_context_echo"], &echo); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tempo_bpm", "time_signature", "time_signature_numerator",
		"time_signature_denominator", "tempo_setting_count"} {
		if _, ok := echo[key]; !ok {
			t.Fatalf("music_context_echo key %q missing", key)
		}
	}
	var fs []map[string]json.RawMessage
	if err := json.Unmarshal(doc["findings"], &fs); err != nil {
		t.Fatal(err)
	}
	if len(fs) == 0 {
		t.Fatal("findings empty")
	}
	for _, key := range []string{"kind", "label", "segments", "interval_pattern", "shift_semitones",
		"confidence", "confidence_score", "basis", "requires_confirmation"} {
		if _, ok := fs[0][key]; !ok {
			t.Fatalf("finding key %q missing", key)
		}
	}
	// sequence_quality（段 1 追认字段）：出现在 kind=sequence 的 finding 上。
	hasSequence := false
	for _, f := range fs {
		if strings.Trim(string(f["kind"]), `"`) == "sequence" {
			if _, ok := f["sequence_quality"]; !ok {
				t.Fatal("sequence finding must carry sequence_quality (segment-1 ratified)")
			}
			hasSequence = true
		}
	}
	if !hasSequence {
		t.Fatal("golden input must contain at least one sequence finding")
	}
	var segs []map[string]json.RawMessage
	if err := json.Unmarshal(fs[0]["segments"], &segs); err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || string(segs[0]["role"]) != `"prototype"` || string(segs[1]["role"]) != `"match"` {
		t.Fatalf("segments roles wrong: %v", segs)
	}
	for _, key := range []string{"role", "start_beats", "length_beats", "bar_start", "bar_end", "note_ids"} {
		if _, ok := segs[0][key]; !ok {
			t.Fatalf("segment key %q missing", key)
		}
	}
	var llm map[string]json.RawMessage
	if err := json.Unmarshal(doc["llm_context"], &llm); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"summary_md", "compact_facts", "do_not_include_raw_package",
		"evidence_refs", "limitation_notes", "suggested_next_step"} {
		if _, ok := llm[key]; !ok {
			t.Fatalf("llm_context key %q missing", key)
		}
	}
	if string(llm["do_not_include_raw_package"]) != "true" {
		t.Fatalf("do_not_include_raw_package = %v", llm["do_not_include_raw_package"])
	}
	if !strings.Contains(string(llm["summary_md"]), "模进") {
		t.Fatalf("summary_md missing golden mention: %v", string(llm["summary_md"]))
	}
	if !strings.Contains(string(llm["evidence_refs"]), "clip:clipA/notes:") {
		t.Fatalf("evidence_refs not clip-anchored: %v", string(llm["evidence_refs"]))
	}
}

// 输入校验 fail-closed：非法参数报错，不静默纠正。
func TestAnalyzeCommandArgsFailClosed(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing clip_id", map[string]any{}, "clip_id is required"},
		{"blank clip_id", map[string]any{"clip_id": "  "}, "clip_id is required"},
		{"unknown transformation", map[string]any{"clip_id": "c", "transformations": []any{"repeat", "canon"}}, "unknown transformation"},
		{"transformations not array", map[string]any{"clip_id": "c", "transformations": "repeat"}, "must be an array"},
		{"key tonic out of range", map[string]any{"clip_id": "c", "key": map[string]any{"tonic": 12.0}}, "pitch class 0-11"},
		{"key mode unsupported", map[string]any{"clip_id": "c", "key": map[string]any{"tonic": 0.0, "mode": "minor"}}, `only supports "major"`},
		{"key not object", map[string]any{"clip_id": "c", "key": "C major"}, "key must be an object"},
		{"min_match_notes too small", map[string]any{"clip_id": "c", "min_match_notes": 1.0}, "must be >= 2"},
		{"min_match_notes fractional", map[string]any{"clip_id": "c", "min_match_notes": 2.5}, "must be an integer"},
		{"range reversed", map[string]any{"clip_id": "c", "range_beats": []any{10.0, 2.0}}, "0 <= start < end"},
		{"range zero width", map[string]any{"clip_id": "c", "range_beats": []any{4.0, 4.0}}, "0 <= start < end"},
		{"range negative", map[string]any{"clip_id": "c", "range_beats": []any{-1.0, 4.0}}, "0 <= start < end"},
		{"range one element", map[string]any{"clip_id": "c", "range_beats": []any{0.0}}, "must be [start_beats, end_beats]"},
		{"range not array", map[string]any{"clip_id": "c", "range_beats": "0-8"}, "must be [start_beats, end_beats]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnalyzeTransformationsCommand(context.Background(), stdFakeKernel().send, tc.args)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
	// min_match_notes=0 显式给出 = 默认 3（与省略同义，不报错）。
	if _, err := AnalyzeTransformationsCommand(context.Background(), stdFakeKernel().send,
		map[string]any{"clip_id": "clipA", "min_match_notes": 0.0}); err != nil {
		t.Fatalf("min_match_notes=0 should default, got %v", err)
	}
}

// 内核回复 fail-closed：notes 命令失败/畸形行报错；辅助命令失败只降级。
func TestAnalyzeCommandKernelFailClosed(t *testing.T) {
	t.Run("notes transport error", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.notesErr = fmt.Errorf("zmq timeout")
		_, err := AnalyzeTransformationsCommand(context.Background(), fk.send, map[string]any{"clip_id": "clipA"})
		if err == nil || !strings.Contains(err.Error(), "get_midi_clip_notes failed") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("notes error status", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.notes = map[string]any{"status": "error", "message": "clip not found"}
		_, err := AnalyzeTransformationsCommand(context.Background(), fk.send, map[string]any{"clip_id": "nope"})
		if err == nil || !strings.Contains(err.Error(), "clip not found") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("malformed note row", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.notes = map[string]any{"status": "ok", "notes": []any{map[string]any{"id": "n1", "pitch": 60.0, "start": 0.0}}}
		_, err := AnalyzeTransformationsCommand(context.Background(), fk.send, map[string]any{"clip_id": "clipA"})
		if err == nil || !strings.Contains(err.Error(), "missing pitch/start/length") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("pitch out of midi range", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.notes = map[string]any{"status": "ok", "notes": []any{map[string]any{"id": "n1", "pitch": 200.0, "start": 0.0, "length": 1.0}}}
		_, err := AnalyzeTransformationsCommand(context.Background(), fk.send, map[string]any{"clip_id": "clipA"})
		if err == nil || !strings.Contains(err.Error(), "outside 0-127") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("state failure degrades not fails", func(t *testing.T) {
		fk := stdFakeKernel()
		fk.stateErr = fmt.Errorf("kernel busy")
		out, err := AnalyzeTransformationsCommand(context.Background(), fk.send, map[string]any{"clip_id": "clipA"})
		if err != nil {
			t.Fatalf("state failure must degrade: %v", err)
		}
		if out["status"] != "ok" {
			t.Fatalf("status = %v", out["status"])
		}
		if segmentOf(t, findingsList(t, out)[0], "prototype")["bar_start"] != nil {
			t.Fatal("bars must be null when state failed")
		}
	})
}

// range_beats 分窗：窗口 [0,32) 覆盖乐句 1-4（28 音，beat 32 的乐句5首音
// 属下一窗口）；所有 finding 的段都不得越窗；A 段再现（匹配段 beats 32-48）
// 不报；窗口内黄金关系（模进+原样重复）保留。
func TestAnalyzeCommandRangeWindow(t *testing.T) {
	out := runAnalyze(t, stdFakeKernel(), map[string]any{
		"clip_id": "clipA", "range_beats": []any{0.0, 32.0}, "key": map[string]any{"tonic": 0.0},
	})
	if number(t, out["analyzed_note_count"]) != 28 {
		t.Fatalf("analyzed_note_count = %v, want 28", out["analyzed_note_count"])
	}
	fs := findingsList(t, out)
	if len(fs) == 0 {
		t.Fatal("windowed findings unexpectedly empty")
	}
	hasDiatonicGolden, hasRepeatGolden := false, false
	for _, f := range fs {
		for _, role := range []string{"prototype", "match"} {
			if number(t, segmentOf(t, f, role)["start_beats"]) >= 32 {
				t.Fatalf("finding segment beyond window: kind=%v %v", f["kind"], f)
			}
		}
		if f["kind"] == "sequence" && f["sequence_quality"] == "diatonic" &&
			number(t, segmentOf(t, f, "prototype")["start_beats"]) == 8.0 &&
			number(t, segmentOf(t, f, "match")["start_beats"]) == 16.0 {
			hasDiatonicGolden = true
		}
		if f["kind"] == "repeat" &&
			number(t, segmentOf(t, f, "prototype")["start_beats"]) == 0.0 &&
			number(t, segmentOf(t, f, "match")["start_beats"]) == 32.0 {
			t.Fatal("A-section reprise (match beats 32-48) must be cut by window")
		}
		if f["kind"] == "repeat" &&
			number(t, segmentOf(t, f, "prototype")["start_beats"]) == 16.0 &&
			number(t, segmentOf(t, f, "match")["start_beats"]) == 24.0 {
			hasRepeatGolden = true
		}
	}
	if !hasDiatonicGolden || !hasRepeatGolden {
		t.Fatalf("window must keep in-window golden relations: diatonic=%v repeat=%v", hasDiatonicGolden, hasRepeatGolden)
	}

	// 空窗口：ok + 空 findings + 窗口披露。
	out = runAnalyze(t, stdFakeKernel(), map[string]any{"clip_id": "clipA", "range_beats": []any{100.0, 104.0}})
	if number(t, out["analyzed_note_count"]) != 0 {
		t.Fatalf("empty window note count = %v", out["analyzed_note_count"])
	}
	if len(out["findings"].([]any)) != 0 {
		t.Fatalf("empty window findings = %v", out["findings"])
	}
	if !strings.Contains(joinIDs(out["limitations"].([]any)), "窗口内无音符") {
		t.Fatalf("empty window limitation missing: %v", out["limitations"])
	}
}

// transformations 白名单与 min_match_notes 透传到检测核；无 key 时 diatonic
// 不报且 limitations 披露未启用。
func TestAnalyzeCommandOptionsAndNoKey(t *testing.T) {
	out := runAnalyze(t, stdFakeKernel(), map[string]any{
		"clip_id": "clipA", "transformations": []any{"repeat"},
	})
	for _, f := range findingsList(t, out) {
		if f["kind"] != "repeat" {
			t.Fatalf("whitelist violated: %v", f["kind"])
		}
	}

	out = runAnalyze(t, stdFakeKernel(), map[string]any{"clip_id": "clipA"})
	for _, f := range findingsList(t, out) {
		if f["kind"] == "sequence" && f["sequence_quality"] == "diatonic" {
			t.Fatal("diatonic must be off without key")
		}
	}
	if !strings.Contains(joinIDs(out["limitations"].([]any)), "未启用") {
		t.Fatalf("no-key limitation missing: %v", out["limitations"])
	}

	out = runAnalyze(t, stdFakeKernel(), map[string]any{
		"clip_id": "clipA", "min_match_notes": 14.0, "key": map[string]any{"tonic": 0.0},
	})
	fs := findingsList(t, out)
	if len(fs) != 1 || fs[0]["kind"] != "repeat" {
		t.Fatalf("min_match_notes=14 should keep only the 14-note A-section reprise, got %d findings", len(fs))
	}
}

// 确定性：同一输入重复调用，返回（含 analysis_id）逐字节一致。
func TestAnalyzeCommandDeterministicID(t *testing.T) {
	args := map[string]any{"clip_id": "clipA", "key": map[string]any{"tonic": 0.0, "mode": "major"}}
	a, err := AnalyzeTransformationsCommand(context.Background(), stdFakeKernel().send, args)
	if err != nil {
		t.Fatal(err)
	}
	b, err := AnalyzeTransformationsCommand(context.Background(), stdFakeKernel().send, args)
	if err != nil {
		t.Fatal(err)
	}
	if a["analysis_id"] != b["analysis_id"] || a["analysis_id"] == "" {
		t.Fatalf("analysis_id unstable: %v vs %v", a["analysis_id"], b["analysis_id"])
	}
	if !strings.HasPrefix(fmt.Sprint(a["analysis_id"]), "analyze_midi_transformations_") {
		t.Fatalf("analysis_id prefix wrong: %v", a["analysis_id"])
	}
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	if string(ra) != string(rb) {
		t.Fatalf("repeat call output differs:\n%s\n%s", ra, rb)
	}
}
