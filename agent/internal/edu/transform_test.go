package edu

import (
	"fmt"
	"reflect"
	"testing"
)

// twinkleNotes 构造《小星星》主题 42 音 fixture（C 大调，4/4，每乐句
// 6 个四分音符 + 1 个二分音符 = 8 beats）。六乐句结构 a a' b b a a'：
//
//	乐句1 a  = C C G G A A G（一闪一闪亮晶晶）   idx 0-6   beats 0-8
//	乐句2 a' = F F E E D D C（满天都是小星星）   idx 7-13  beats 8-16
//	乐句3 b  = G G F F E E D（挂在天上放光明）   idx 14-20 beats 16-24
//	乐句4 b  = G G F F E E D（好像许多小眼睛）   idx 21-27 beats 24-32
//	乐句5 a  = C C G G A A G（一闪一闪亮晶晶）   idx 28-34 beats 32-40
//	乐句6 a' = F F E E D D C（满天都是小星星）   idx 35-41 beats 40-48
//
// 黄金结构：乐句2 ↔ 乐句3 为调内模进（音级 +1，首音 +2 半音）；
// 乐句3 ↔ 乐句4 为原样重复；乐句1+2 ↔ 乐句5+6 为 A 段整体再现。
// fixture 为测试内联资产，不触碰样例工程（AGENTS §10）。
func twinkleNotes() []Note {
	phrases := [][]int{
		{60, 60, 67, 67, 69, 69, 67},
		{65, 65, 64, 64, 62, 62, 60},
		{67, 67, 65, 65, 64, 64, 62},
		{67, 67, 65, 65, 64, 64, 62},
		{60, 60, 67, 67, 69, 69, 67},
		{65, 65, 64, 64, 62, 62, 60},
	}
	notes := make([]Note, 0, 42)
	idx := 0
	for pi, ph := range phrases {
		for ni, p := range ph {
			length := 1.0
			if ni == len(ph)-1 {
				length = 2.0
			}
			notes = append(notes, Note{
				ID:     fmt.Sprintf("tw%02d", idx),
				Pitch:  p,
				Start:  float64(pi*8 + ni),
				Length: length,
			})
			idx++
		}
	}
	return notes
}

func cmajor() *Key { return &Key{Tonic: 0, Mode: "major"} }

func pitchesEqual(a, b []int) bool { return reflect.DeepEqual(a, b) }

func findByPitches(fs []Finding, kind Kind, quality SequenceQuality, proto, match []int) *Finding {
	for i := range fs {
		f := &fs[i]
		if f.Kind != kind || (quality != "" && f.SequenceQuality != quality) {
			continue
		}
		if pitchesEqual(f.Prototype.Pitches, proto) && pitchesEqual(f.Match.Pitches, match) {
			return f
		}
	}
	return nil
}

// 黄金用例：小星星主题检出调内模进（乐句2↔乐句3）与原样重复（乐句3↔乐句4），
// 区分 repeat/sequence，shift 与音级步差正确。
func TestTwinkleGoldenDetectsDiatonicSequenceAndRepeat(t *testing.T) {
	fs, err := DetectTransformations(twinkleNotes(), DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}

	// 关系一：乐句2 ↔ 乐句3 调内模进（音级 +1，首音 +2 半音）。
	seq := findByPitches(fs, KindSequence, SeqDiatonic,
		[]int{65, 65, 64, 64, 62, 62, 60}, []int{67, 67, 65, 65, 64, 64, 62})
	if seq == nil {
		t.Fatalf("diatonic sequence (phrase2 -> phrase3) not found; got %d findings", len(fs))
	}
	if seq.DiatonicSteps != 1 {
		t.Errorf("DiatonicSteps = %d, want 1", seq.DiatonicSteps)
	}
	if seq.ShiftSemitones != 2 {
		t.Errorf("ShiftSemitones = %d, want 2 (G-F)", seq.ShiftSemitones)
	}
	if !pitchesEqual(seq.IntervalPattern, []int{0, -1, 0, -2, 0, -2}) {
		t.Errorf("IntervalPattern = %v, want [0 -1 0 -2 0 -2]", seq.IntervalPattern)
	}
	if seq.Prototype.NoteCount != 7 || seq.Match.NoteCount != 7 {
		t.Errorf("segment note counts = %d/%d, want 7/7", seq.Prototype.NoteCount, seq.Match.NoteCount)
	}
	if seq.Prototype.StartBeats != 8 || seq.Match.StartBeats != 16 {
		t.Errorf("segment beats = %v/%v, want 8/16", seq.Prototype.StartBeats, seq.Match.StartBeats)
	}
	if seq.Prototype.NoteIDs[0] != "tw07" || seq.Match.NoteIDs[0] != "tw14" {
		t.Errorf("note ids = %s/%s, want tw07/tw14", seq.Prototype.NoteIDs[0], seq.Match.NoteIDs[0])
	}

	// 关系二：乐句3 ↔ 乐句4 原样重复（B 段两遍），必须被判为 repeat 而非 sequence。
	phraseB := []int{67, 67, 65, 65, 64, 64, 62}
	rep := findByPitches(fs, KindRepeat, "", phraseB, phraseB)
	if rep == nil {
		t.Fatalf("repeat (phrase3 -> phrase4) not found; got %d findings", len(fs))
	}
	if rep.ShiftSemitones != 0 {
		t.Errorf("repeat ShiftSemitones = %d, want 0", rep.ShiftSemitones)
	}
	if rep.SequenceQuality != "" {
		t.Errorf("repeat finding carries quality %q, want empty", rep.SequenceQuality)
	}
	if rep.DiatonicSteps != 0 {
		t.Errorf("repeat DiatonicSteps = %d, want 0", rep.DiatonicSteps)
	}

	// 附带断言：A 段整体再现（乐句1+2 ↔ 乐句5+6，14 音）也被检出。
	if findByPitches(fs, KindRepeat, "",
		[]int{60, 60, 67, 67, 69, 69, 67, 65, 65, 64, 64, 62, 62, 60},
		[]int{60, 60, 67, 67, 69, 69, 67, 65, 65, 64, 64, 62, 62, 60}) == nil {
		t.Errorf("long-form repeat (phrases 1+2 -> 5+6) not found")
	}

	// 不变式：repeat 恒 shift=0；sequence 恒 shift!=0（两关系严格区分）。
	for _, f := range fs {
		switch f.Kind {
		case KindRepeat:
			if f.ShiftSemitones != 0 {
				t.Errorf("repeat finding with shift %d", f.ShiftSemitones)
			}
		case KindSequence:
			if f.ShiftSemitones == 0 {
				t.Errorf("sequence finding with shift 0")
			}
		}
	}
}

// 负例：无变换关系的序列（同音程不重复、无倒影/逆行/模进子段对）→ 空 findings。
func TestNoRelationYieldsNoFindings(t *testing.T) {
	notes := []Note{
		{ID: "n0", Pitch: 60, Start: 0, Length: 1},
		{ID: "n1", Pitch: 66, Start: 1, Length: 1},
		{ID: "n2", Pitch: 61, Start: 2, Length: 1},
		{ID: "n3", Pitch: 70, Start: 3, Length: 1},
		{ID: "n4", Pitch: 62, Start: 4, Length: 1},
		{ID: "n5", Pitch: 71, Start: 5, Length: 1},
	}
	fs, err := DetectTransformations(notes, DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected no findings, got %d: %+v", len(fs), fs)
	}
}

// 边界例：同节奏、异轮廓（全部落在 C 大调内）→ 不误报任何变换关系。
// 序列 C G B D | E C F A：全部 6 个段对 (0,3)(0,4)(0,5)(1,4)(1,5)(2,5)
// 已逐对人工核对（identity/inversion/retrograde/diatonic 四条件均不满足；
// 前一版后段 [69,65,62] 曾与 [60,64,67] 构成轴 129 的真倒影，属 fixture
// 缺陷而非误报，已换轮廓）。
func TestSameRhythmDifferentContourNoFalsePositive(t *testing.T) {
	pitches := []int{60, 67, 71, 62, 64, 60, 65, 69}
	notes := make([]Note, len(pitches))
	for i, p := range pitches {
		notes[i] = Note{ID: fmt.Sprintf("b%d", i), Pitch: p, Start: float64(i), Length: 1}
	}
	fs, err := DetectTransformations(notes, DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("expected no findings for same-rhythm different-contour input, got %d: %+v", len(fs), fs)
	}
}

// chromatic sequence 正例：严格移调 +2 半音（含调外音 F#，不受 Key 影响）。
func TestChromaticSequenceDetection(t *testing.T) {
	notes := []Note{
		{ID: "c0", Pitch: 60, Start: 0, Length: 1},
		{ID: "c1", Pitch: 62, Start: 1, Length: 1},
		{ID: "c2", Pitch: 64, Start: 2, Length: 1},
		{ID: "c3", Pitch: 65, Start: 3, Length: 1},
		{ID: "c4", Pitch: 62, Start: 4, Length: 1},
		{ID: "c5", Pitch: 64, Start: 5, Length: 1},
		{ID: "c6", Pitch: 66, Start: 6, Length: 1},
		{ID: "c7", Pitch: 67, Start: 7, Length: 1},
	}
	fs, err := DetectTransformations(notes, DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Kind != KindSequence || f.SequenceQuality != SeqChromatic {
		t.Fatalf("kind/quality = %s/%s, want sequence/chromatic", f.Kind, f.SequenceQuality)
	}
	if f.ShiftSemitones != 2 {
		t.Errorf("ShiftSemitones = %d, want 2", f.ShiftSemitones)
	}
	if f.DiatonicSteps != 0 {
		t.Errorf("chromatic finding carries DiatonicSteps = %d, want 0", f.DiatonicSteps)
	}
	if !pitchesEqual(f.Prototype.Pitches, []int{60, 62, 64, 65}) ||
		!pitchesEqual(f.Match.Pitches, []int{62, 64, 66, 67}) {
		t.Errorf("segments = %v -> %v", f.Prototype.Pitches, f.Match.Pitches)
	}
}

// inversion 正例：B[t] = 120 - A[t]（轴=120）。
func TestInversionDetection(t *testing.T) {
	notes := []Note{
		{ID: "i0", Pitch: 60, Start: 0, Length: 1},
		{ID: "i1", Pitch: 64, Start: 1, Length: 1},
		{ID: "i2", Pitch: 67, Start: 2, Length: 1},
		{ID: "i3", Pitch: 60, Start: 3, Length: 1},
		{ID: "i4", Pitch: 56, Start: 4, Length: 1},
		{ID: "i5", Pitch: 53, Start: 5, Length: 1},
	}
	fs, err := DetectTransformations(notes, DetectOptions{})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Kind != KindInversion {
		t.Fatalf("kind = %s, want inversion", f.Kind)
	}
	if f.InversionAxisSemitones != 120 {
		t.Errorf("axis = %d, want 120", f.InversionAxisSemitones)
	}
	if !pitchesEqual(f.IntervalPattern, []int{4, 3}) {
		t.Errorf("IntervalPattern = %v, want [4 3]", f.IntervalPattern)
	}
}

// retrograde 正例：匹配段音高=原型逆序，节奏同步镜像（IOI [1,1.5] ↔ [1.5,1]，
// 长度 [1,1,2] ↔ [2,1,1]）；节奏不镜像的对照组不报。
func TestRetrogradeDetection(t *testing.T) {
	makeNotes := func(bStarts []float64, bLens []float64) []Note {
		return []Note{
			{ID: "r0", Pitch: 60, Start: 0, Length: 1},
			{ID: "r1", Pitch: 64, Start: 1, Length: 1},
			{ID: "r2", Pitch: 65, Start: 2.5, Length: 2},
			{ID: "r3", Pitch: 65, Start: bStarts[0], Length: bLens[0]},
			{ID: "r4", Pitch: 64, Start: bStarts[1], Length: bLens[1]},
			{ID: "r5", Pitch: 60, Start: bStarts[2], Length: bLens[2]},
		}
	}
	fs, err := DetectTransformations(makeNotes([]float64{4, 5.5, 6.5}, []float64{2, 1, 1}), DetectOptions{})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected exactly 1 retrograde finding, got %d: %+v", len(fs), fs)
	}
	if fs[0].Kind != KindRetrograde {
		t.Fatalf("kind = %s, want retrograde", fs[0].Kind)
	}
	if !pitchesEqual(fs[0].Prototype.Pitches, []int{60, 64, 65}) ||
		!pitchesEqual(fs[0].Match.Pitches, []int{65, 64, 60}) {
		t.Errorf("segments = %v -> %v", fs[0].Prototype.Pitches, fs[0].Match.Pitches)
	}

	// 对照组：音高镜像但节奏未镜像（IOI [1,1] 而非 [1.5,1]）→ 不报。
	fsNeg, err := DetectTransformations(makeNotes([]float64{4, 5, 6}, []float64{2, 1, 1}), DetectOptions{})
	if err != nil {
		t.Fatalf("DetectTransformations(neg): %v", err)
	}
	if len(fsNeg) != 0 {
		t.Fatalf("non-mirrored rhythm must not be reported, got %d: %+v", len(fsNeg), fsNeg)
	}
}

// 退化情形：同音反复同时满足四种变换的数学条件，只报最高优先级的 repeat。
func TestDegenerateSamePitchReportsRepeatOnly(t *testing.T) {
	notes := make([]Note, 8)
	for i := range notes {
		notes[i] = Note{ID: fmt.Sprintf("d%d", i), Pitch: 60, Start: float64(i), Length: 1}
	}
	fs, err := DetectTransformations(notes, DetectOptions{})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("expected repeat findings for same-pitch run")
	}
	for _, f := range fs {
		if f.Kind != KindRepeat {
			t.Errorf("degenerate same-pitch run reported %s, want repeat only", f.Kind)
		}
	}
}

// MinMatchNotes 过滤：阈值 8 时乐句级 7 音关系不报，仅保留 14 音 A 段再现。
func TestMinMatchNotesRaisesBar(t *testing.T) {
	fs, err := DetectTransformations(twinkleNotes(), DetectOptions{Key: cmajor(), MinMatchNotes: 8})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	for _, f := range fs {
		if f.NoteCount() < 8 {
			t.Errorf("finding with %d notes leaked below MinMatchNotes=8", f.NoteCount())
		}
	}
	var long *Finding
	for i := range fs {
		if fs[i].NoteCount() == 14 && fs[i].Kind == KindRepeat {
			long = &fs[i]
		}
	}
	if long == nil {
		t.Fatalf("14-note A-section repeat not found; got %+v", fs)
	}
}

// 确定性：同输入两次调用、乱序输入（排序归一化后）结果完全一致。
func TestDeterminismAndInputOrderIndependence(t *testing.T) {
	notes := twinkleNotes()
	a, err := DetectTransformations(notes, DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	b, err := DetectTransformations(twinkleNotes(), DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations(2nd): %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same input must yield identical findings")
	}
	shuffled := make([]Note, len(notes))
	for i := range notes {
		shuffled[i] = notes[len(notes)-1-i]
	}
	c, err := DetectTransformations(shuffled, DetectOptions{Key: cmajor()})
	if err != nil {
		t.Fatalf("DetectTransformations(shuffled): %v", err)
	}
	if !reflect.DeepEqual(a, c) {
		t.Fatal("input order must not affect findings")
	}
}

// 无 Key 时调内模进不检测（乐句2↔乐句3 只满足 diatonic 关系 → 不报）；
// chromatic sequence（如 A A G → D D C，shift=-7 的真实严格移调对）与
// repeat 不受影响。
func TestNoKeyDisablesDiatonicSequence(t *testing.T) {
	fs, err := DetectTransformations(twinkleNotes(), DetectOptions{})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	repeats := 0
	for _, f := range fs {
		if f.Kind == KindSequence && f.SequenceQuality == SeqDiatonic {
			t.Fatalf("diatonic sequence without Key, got %+v", f)
		}
		if f.Kind == KindRepeat {
			repeats++
		}
	}
	if repeats < 2 {
		t.Errorf("expected >=2 repeat findings (phrase3->4 and A-section), got %d", repeats)
	}
}

// 变换白名单：只请求 repeat 时其他变换不报。
func TestTransformationWhitelist(t *testing.T) {
	fs, err := DetectTransformations(twinkleNotes(), DetectOptions{Key: cmajor(), Transformations: []Kind{KindRepeat}})
	if err != nil {
		t.Fatalf("DetectTransformations: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("expected repeat findings")
	}
	for _, f := range fs {
		if f.Kind != KindRepeat {
			t.Errorf("whitelist violated: %s reported", f.Kind)
		}
	}
}

// 无效选项 fail-closed；空输入与过短输入返回空 findings 而非错误。
func TestInvalidOptionsAndShortInput(t *testing.T) {
	notes := twinkleNotes()
	if _, err := DetectTransformations(notes, DetectOptions{Transformations: []Kind{"bogus"}}); err == nil {
		t.Error("unknown kind must return error")
	}
	if _, err := DetectTransformations(notes, DetectOptions{Key: &Key{Tonic: 0, Mode: "minor"}}); err == nil {
		t.Error("minor mode must return error (unsupported)")
	}
	if _, err := DetectTransformations(notes, DetectOptions{Key: &Key{Tonic: 12}}); err == nil {
		t.Error("tonic 12 must return error")
	}
	fs, err := DetectTransformations(nil, DetectOptions{})
	if err != nil || len(fs) != 0 {
		t.Errorf("nil input = (%d findings, %v), want (0, nil)", len(fs), err)
	}
	short := notes[:4]
	fs, err = DetectTransformations(short, DetectOptions{})
	if err != nil || len(fs) != 0 {
		t.Errorf("short input = (%d findings, %v), want (0, nil)", len(fs), err)
	}
}
