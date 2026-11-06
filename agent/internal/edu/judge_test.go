package edu

import (
	"fmt"
	"testing"
)

// protoNotes 构造原型片段 [62,64,66,67]（D E F# G，四分音符）。
func protoNotes() []Note {
	return notesFromPitches([]int{62, 64, 66, 67})
}

func notesFromPitches(pitches []int) []Note {
	notes := make([]Note, len(pitches))
	for i, p := range pitches {
		notes[i] = Note{ID: fmt.Sprintf("j%d", i), Pitch: p, Start: float64(i), Length: 1}
	}
	return notes
}

// 原样一致 → match，无错音，节奏一致。
func TestJudgeExactMatch(t *testing.T) {
	j := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{62, 64, 66, 67}))
	if j.PitchStatus != StatusMatch {
		t.Fatalf("status = %s, want match", j.PitchStatus)
	}
	if j.Transposition != 0 {
		t.Errorf("transposition = %d, want 0", j.Transposition)
	}
	if len(j.Mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none", j.Mismatches)
	}
	if !j.RhythmMatched {
		t.Error("RhythmMatched = false, want true")
	}
	if j.PrototypeNoteCount != 4 || j.AttemptNoteCount != 4 {
		t.Errorf("note counts = %d/%d, want 4/4", j.PrototypeNoteCount, j.AttemptNoteCount)
	}
}

// 整体移调 +2（音程序列全等，首音差 2）→ transposed_match。
func TestJudgeTransposedMatch(t *testing.T) {
	j := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{64, 66, 68, 69}))
	if j.PitchStatus != StatusTransposedMatch {
		t.Fatalf("status = %s, want transposed_match", j.PitchStatus)
	}
	if j.Transposition != 2 {
		t.Errorf("transposition = %d, want 2", j.Transposition)
	}
	if len(j.Mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none", j.Mismatches)
	}
	if !j.RhythmMatched {
		t.Error("RhythmMatched = false, want true")
	}
}

// 错音定位：学生 [62,64,65,67] 第 3 音（idx2）应为 66；恢复点（idx3）不报。
func TestJudgePitchMismatchLocalization(t *testing.T) {
	j := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{62, 64, 65, 67}))
	if j.PitchStatus != StatusPitchMismatch {
		t.Fatalf("status = %s, want pitch_mismatch", j.PitchStatus)
	}
	if len(j.Mismatches) != 1 {
		t.Fatalf("mismatches = %+v, want exactly 1 (recovery point must be suppressed)", j.Mismatches)
	}
	m := j.Mismatches[0]
	if m.AttemptIndex != 2 || m.NoteID != "j2" {
		t.Errorf("mismatch located at (%d,%s), want (2,j2)", m.AttemptIndex, m.NoteID)
	}
	if m.ActualPitch != 65 || m.ExpectedPitch != 66 || m.PrototypePitch != 66 {
		t.Errorf("pitches actual=%d expected=%d prototype=%d, want 65/66/66", m.ActualPitch, m.ExpectedPitch, m.PrototypePitch)
	}
}

// 移调语境下的错音：学生整体 +2 后第 4 音走错 → 期望值按移位假设换算。
func TestJudgeMismatchUnderTransposition(t *testing.T) {
	j := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{64, 66, 68, 67}))
	if j.PitchStatus != StatusPitchMismatch {
		t.Fatalf("status = %s, want pitch_mismatch", j.PitchStatus)
	}
	if j.Transposition != 2 {
		t.Errorf("transposition = %d, want 2 (first-note anchor)", j.Transposition)
	}
	if len(j.Mismatches) != 1 {
		t.Fatalf("mismatches = %+v, want exactly 1", j.Mismatches)
	}
	m := j.Mismatches[0]
	if m.AttemptIndex != 3 {
		t.Errorf("mismatch index = %d, want 3", m.AttemptIndex)
	}
	if m.ActualPitch != 67 || m.ExpectedPitch != 69 || m.PrototypePitch != 67 {
		t.Errorf("pitches actual=%d expected=%d prototype=%d, want 67/69/67", m.ActualPitch, m.ExpectedPitch, m.PrototypePitch)
	}
}

// 音数不等 → length_mismatch；公共区间仍做逐音 diff。
func TestJudgeLengthMismatch(t *testing.T) {
	// 学生少弹末音：intervals 公共区间全等，无错音，仅数量差。
	j := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{62, 64, 66}))
	if j.PitchStatus != StatusLengthMismatch {
		t.Fatalf("status = %s, want length_mismatch", j.PitchStatus)
	}
	if j.PrototypeNoteCount != 4 || j.AttemptNoteCount != 3 {
		t.Errorf("counts = %d/%d, want 4/3", j.PrototypeNoteCount, j.AttemptNoteCount)
	}
	if len(j.Mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none (prefix aligned)", j.Mismatches)
	}
	if j.RhythmMatched {
		t.Error("RhythmMatched = true, want false")
	}

	// 学生多弹且中间走音：数量差 + 定位到错音。
	j2 := JudgeAgainstPrototype(protoNotes(), notesFromPitches([]int{62, 65, 66, 67, 69}))
	if j2.PitchStatus != StatusLengthMismatch {
		t.Fatalf("status = %s, want length_mismatch", j2.PitchStatus)
	}
	if len(j2.Mismatches) != 1 || j2.Mismatches[0].AttemptIndex != 1 {
		t.Errorf("mismatches = %+v, want single mismatch at index 1", j2.Mismatches)
	}
}

// 音高全对但节奏不同 → pitch=match 而 RhythmMatched=false。
func TestJudgeRhythmMismatch(t *testing.T) {
	attempt := notesFromPitches([]int{62, 64, 66, 67})
	attempt[2].Length = 3 // 拉长第三个音
	j := JudgeAgainstPrototype(protoNotes(), attempt)
	if j.PitchStatus != StatusMatch {
		t.Fatalf("status = %s, want match (pitch domain)", j.PitchStatus)
	}
	if j.RhythmMatched {
		t.Error("RhythmMatched = true, want false")
	}
}

// 输入顺序无关 + 空输入不 panic。
func TestJudgeInputOrderAndEmpty(t *testing.T) {
	proto := protoNotes()
	attempt := notesFromPitches([]int{62, 64, 65, 67})
	a := JudgeAgainstPrototype(proto, attempt)
	// 反转两侧输入顺序（函数内部排序归一）。
	for i, k := 0, len(proto)-1; i < k; i, k = i+1, k-1 {
		proto[i], proto[k] = proto[k], proto[i]
	}
	for i, k := 0, len(attempt)-1; i < k; i, k = i+1, k-1 {
		attempt[i], attempt[k] = attempt[k], attempt[i]
	}
	b := JudgeAgainstPrototype(proto, attempt)
	if a.PitchStatus != b.PitchStatus || len(a.Mismatches) != len(b.Mismatches) ||
		a.Mismatches[0].AttemptIndex != b.Mismatches[0].AttemptIndex {
		t.Errorf("input order affected judgment: %+v vs %+v", a, b)
	}

	empty := JudgeAgainstPrototype(nil, attempt)
	if empty.PitchStatus != StatusLengthMismatch || len(empty.Mismatches) != 0 {
		t.Errorf("empty prototype = %+v, want length_mismatch with no mismatches", empty)
	}
}
