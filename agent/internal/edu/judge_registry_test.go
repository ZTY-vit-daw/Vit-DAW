package edu

// 判定器三件单测（乐局 3a）：注册表闭集 + 三判定器正反例 + 与 279 定稿
// 夹具的互测基线（改错轨 judge 差分=附录 A 基线：恰 1 处 mismatch idx 33）。

import (
	"strings"
	"testing"
)

func mkNotes(pitches []int, start float64, step, length float64) []Note {
	notes := make([]Note, len(pitches))
	for k, p := range pitches {
		notes[k] = Note{ID: string(rune('a'+k)) + itoa(k), Pitch: p, Start: start + float64(k)*step, Length: length}
	}
	return notes
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func TestJudgeRegistryClosedSet(t *testing.T) {
	want := []string{JudgeNoteSequenceMatch, JudgePitchSetMatch, JudgeRhythmMatch}
	got := RegisteredJudges()
	if len(got) != 3 {
		t.Fatalf("registry size = %d, want 3（首版闭集）: %v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("registry = %v, want %v", got, want)
		}
	}
	if _, ok := LookupJudge("chord_match"); ok {
		t.Fatal("LookupJudge(unknown) = ok, want false（fail-closed）")
	}
	for _, name := range want {
		if _, ok := LookupJudge(name); !ok {
			t.Fatalf("LookupJudge(%s) missing", name)
		}
	}
}

func TestJudgeNoteSequenceMatchTranspositionContract(t *testing.T) {
	proto := mkNotes([]int{60, 62, 64}, 0, 1, 1)
	exact := mkNotes([]int{60, 62, 64}, 8, 1, 1)
	transposed := mkNotes([]int{62, 64, 66}, 8, 1, 1)
	wrong := mkNotes([]int{60, 63, 64}, 8, 1, 1)

	if v := judgeNoteSequenceMatch(JudgeInput{TargetNotes: exact, ReferenceNotes: proto}); !v.Passed {
		t.Fatalf("exact = %+v, want pass", v)
	}
	// accept_transposed=false（默认）：transposed_match 不算过（276 语义说明
	// 预留的乐局卡裁决落点）。
	v := judgeNoteSequenceMatch(JudgeInput{TargetNotes: transposed, ReferenceNotes: proto})
	if v.Passed {
		t.Fatal("transposed passed with default accept_transposed, want fail")
	}
	j := v.Detail["judgment"].(Judgment)
	if j.PitchStatus != StatusTransposedMatch || j.Transposition != 2 {
		t.Fatalf("judgment = %+v, want transposed_match/+2", j)
	}
	// accept_transposed=true：模进练习口径算过。
	tacc := true
	if v := judgeNoteSequenceMatch(JudgeInput{TargetNotes: transposed, ReferenceNotes: proto, AcceptTransposed: tacc}); !v.Passed {
		t.Fatalf("transposed with accept=true = %+v, want pass", v)
	}
	if v := judgeNoteSequenceMatch(JudgeInput{TargetNotes: wrong, ReferenceNotes: proto}); v.Passed {
		t.Fatal("wrong note passed, want fail")
	} else if j := v.Detail["judgment"].(Judgment); len(j.Mismatches) != 1 || j.Mismatches[0].AttemptIndex != 1 {
		t.Fatalf("mismatches = %+v, want single idx 1", j.Mismatches)
	}
	// 音数不等 → length_mismatch（仍 fail）。
	short := mkNotes([]int{60, 62}, 8, 1, 1)
	if v := judgeNoteSequenceMatch(JudgeInput{TargetNotes: short, ReferenceNotes: proto}); v.Passed {
		t.Fatal("length mismatch passed, want fail")
	}
}

func TestJudgePitchSetMatch(t *testing.T) {
	// 默认 pitch_class：八度错位容认。
	a := mkNotes([]int{60, 64, 67}, 0, 1, 1)
	octave := mkNotes([]int{72, 76, 79}, 0, 1, 1)
	if v := judgePitchSetMatch(JudgeInput{TargetNotes: octave, ReferenceNotes: a}); !v.Passed {
		t.Fatalf("pitch_class default should tolerate octave displacement: %+v", v)
	}
	// absolute：八度错位 fail，missing/extra 对称披露。
	v := judgePitchSetMatch(JudgeInput{TargetNotes: octave, ReferenceNotes: a, SetMode: "absolute"})
	if v.Passed {
		t.Fatal("absolute mode passed octave displacement, want fail")
	}
	if len(v.Detail["missing"].([]int)) != 3 || len(v.Detail["extra"].([]int)) != 3 {
		t.Fatalf("missing/extra = %v/%v", v.Detail["missing"], v.Detail["extra"])
	}
	// 多重集：重复音计数保留。
	dupRef := mkNotes([]int{60, 60, 64}, 0, 1, 1)
	dupTgt := mkNotes([]int{60, 64, 64}, 0, 1, 1)
	if v := judgePitchSetMatch(JudgeInput{TargetNotes: dupTgt, ReferenceNotes: dupRef}); v.Passed {
		t.Fatal("multiset violation passed, want fail（重复音计数保留）")
	}
	// 空侧 fail-closed（空对空不构成证据）。
	if v := judgePitchSetMatch(JudgeInput{TargetNotes: nil, ReferenceNotes: nil}); v.Passed {
		t.Fatal("empty passed, want fail")
	}
	// 节奏/顺序无关：乱序同集过、节奏不同不干预集合口径。
	shuffled := mkNotes([]int{67, 60, 64}, 0, 0.5, 0.25)
	if v := judgePitchSetMatch(JudgeInput{TargetNotes: shuffled, ReferenceNotes: a}); !v.Passed {
		t.Fatalf("order/rhythm-independent set compare failed: %+v", v)
	}
}

func TestJudgeRhythmMatch(t *testing.T) {
	ref := mkNotes([]int{60, 62, 64}, 0, 1, 1)
	sameRhythm := mkNotes([]int{72, 74, 76}, 5, 1, 1) // 音高无关，节奏一致
	if v := judgeRhythmMatch(JudgeInput{TargetNotes: sameRhythm, ReferenceNotes: ref}); !v.Passed {
		t.Fatalf("same rhythm = %+v, want pass", v)
	}
	diffLen := mkNotes([]int{72, 74, 76}, 5, 1, 2) // 逐音长度不同
	if v := judgeRhythmMatch(JudgeInput{TargetNotes: diffLen, ReferenceNotes: ref}); v.Passed {
		t.Fatal("different note lengths passed, want fail")
	}
	diffIOI := mkNotes([]int{72, 74, 76}, 5, 0.5, 1) // IOI 不同
	if v := judgeRhythmMatch(JudgeInput{TargetNotes: diffIOI, ReferenceNotes: ref}); v.Passed {
		t.Fatal("different IOI passed, want fail")
	}
	fewer := mkNotes([]int{72, 74}, 5, 1, 1)
	if v := judgeRhythmMatch(JudgeInput{TargetNotes: fewer, ReferenceNotes: ref}); v.Passed {
		t.Fatal("unequal counts passed, want fail（音数不等即 fail）")
	}
}

// 互测基线（279 附录 A）：完整 theme vs 改错轨（@37→68）恰 1 处 mismatch，
// idx 33、actual 68 / expected 69 / prototype 69，rhythm_matched=true。
func TestJudgeAgainstFixtureCorruptedBaseline(t *testing.T) {
	card := loadFixtureLevel(t)
	theme, ok := card.AssetNotes("theme")
	if !ok {
		t.Fatal("theme notes unresolvable")
	}
	corrupted, ok := card.AssetNotes("theme_corrupted")
	if !ok {
		t.Fatal("corrupted notes unresolvable")
	}
	in := JudgeInput{
		JudgeName:        JudgeNoteSequenceMatch,
		TargetNotes:      windowNotes(corrupted, 0, 48),
		ReferenceNotes:   windowNotes(theme, 0, 48),
		AcceptTransposed: false,
	}
	v := judgeNoteSequenceMatch(in)
	if v.Passed {
		t.Fatal("corrupted target passed, want fail")
	}
	j, ok := v.Detail["judgment"].(Judgment)
	if !ok {
		t.Fatalf("detail.judgment type %T", v.Detail["judgment"])
	}
	if j.PitchStatus != StatusPitchMismatch || len(j.Mismatches) != 1 {
		t.Fatalf("judgment = %+v, want pitch_mismatch with exactly 1 mismatch", j)
	}
	m := j.Mismatches[0]
	if m.AttemptIndex != 33 || m.ActualPitch != 68 || m.ExpectedPitch != 69 || m.PrototypePitch != 69 {
		t.Fatalf("mismatch = %+v, want idx 33 68/69/69（附录 A 基线）", m)
	}
	if !j.RhythmMatched {
		t.Fatal("rhythm_matched = false, want true（单音高差不破坏节奏）")
	}
	if !strings.Contains(v.Summary, "pitch_status=pitch_mismatch") {
		t.Fatalf("summary = %q", v.Summary)
	}
}
