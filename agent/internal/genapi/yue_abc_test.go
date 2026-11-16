package genapi

import (
	"strings"
	"testing"
)

// GEN-YUE-WIRE-1 item 1: deterministic student-score -> YuE2 ABC dialect
// conversion with a round-trip proof (convert -> parse back -> the melody
// voice's notes equal the input within the 1/32 quantization grid).

func TestYueABCRoundTripSimpleMelody(t *testing.T) {
	// 4/4, one bar: quarter notes C E G c (MIDI 60 64 67 72) + trailing rest.
	in := YueScoreInput{
		Notes: []YueNote{
			{Pitch: 60, Start: 0, Length: 1},
			{Pitch: 64, Start: 1, Length: 1},
			{Pitch: 67, Start: 2, Length: 1},
			{Pitch: 72, Start: 3, Length: 1},
		},
		Numerator:   4,
		Denominator: 4,
		TempoBPM:    96,
		Key:         "C",
	}
	abc, err := ConvertMidiToYueABC(in)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	for _, want := range []string{"X:1", "M:4/4", "L:1/32", "Q:1/4=96", "K:C", "V: Vocal", "V: Ins"} {
		if !strings.Contains(abc, want) {
			t.Fatalf("header missing %q in:\n%s", want, abc)
		}
	}
	// Melody rides Ins; Vocal rests.
	if !strings.Contains(abc, "V: Vocal\nZ1|") {
		t.Fatalf("vocal voice must rest for the single bar, got:\n%s", abc)
	}
	if !strings.Contains(abc, "c8e8g8c'8|") {
		t.Fatalf("melody line not as expected, got:\n%s", abc)
	}
	back, err := ParseYueABC(abc, "Ins")
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(back) != 4 {
		t.Fatalf("round trip lost notes: %#v", back)
	}
	for i, want := range []struct {
		pitch  int
		start  float64
		length float64
	}{{60, 0, 1}, {64, 1, 1}, {67, 2, 1}, {72, 3, 1}} {
		got := back[i]
		if got.Pitch != want.pitch || got.Start != want.start || got.Length != want.length {
			t.Fatalf("note %d round trip mismatch: got (%d %v %v) want (%d %v %v)", i, got.Pitch, got.Start, got.Length, want.pitch, want.start, want.length)
		}
	}
	// The resting voice parses back silent.
	vocal, err := ParseYueABC(abc, "Vocal")
	if err != nil {
		t.Fatalf("parse vocal: %v", err)
	}
	if len(vocal) != 0 {
		t.Fatalf("vocal voice must parse back silent, got %#v", vocal)
	}
}

func TestYueABCRoundTripTiesAndOffbeat(t *testing.T) {
	// 2/4 (course time signature), a note tied across the bar line and an
	// off-beat eighth (0.5 beats) entry.
	in := YueScoreInput{
		Notes: []YueNote{
			{Pitch: 62, Start: 1, Length: 1.5}, // [8,20) units crosses the 2/4 bar line
			{Pitch: 65, Start: 2.5, Length: 0.5},
			{Pitch: 67, Start: 3, Length: 1},
		},
		Numerator:   2,
		Denominator: 4,
		Key:         "C",
	}
	abc, err := ConvertMidiToYueABC(in)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(abc, "M:2/4") {
		t.Fatalf("2/4 not written: %s", abc)
	}
	if !strings.Contains(abc, "z8d8-|d4") {
		t.Fatalf("tie across the bar line missing, got:\n%s", abc)
	}
	back, err := ParseYueABC(abc, "Ins")
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	want := []struct {
		pitch  int
		start  float64
		length float64
	}{{62, 1, 1.5}, {65, 2.5, 0.5}, {67, 3, 1}}
	if len(back) != len(want) {
		t.Fatalf("round trip note count: got %d want %d (%#v)", len(back), len(want), back)
	}
	for i, w := range want {
		if back[i].Pitch != w.pitch || back[i].Start != w.start || back[i].Length != w.length {
			t.Fatalf("note %d mismatch: got (%d %v %v) want (%d %v %v)", i, back[i].Pitch, back[i].Start, back[i].Length, w.pitch, w.start, w.length)
		}
	}
}

func TestYueABCRoundTripChromaticFlatsSectionsChords(t *testing.T) {
	in := YueScoreInput{
		Notes: []YueNote{
			{Pitch: 61, Start: 0, Length: 1}, // C#/Db
			{Pitch: 70, Start: 1, Length: 1}, // Bb in flat spelling
			{Pitch: 65, Start: 2, Length: 2},
			{Pitch: 65, Start: 4, Length: 4},
		},
		Numerator:   4,
		Denominator: 4,
		Key:         "F",
		Flats:       true,
		Sections:    []YueSection{{Name: "verse", Bars: 2}},
		Chords:      []YueChord{{Bar: 0, Name: "Dm7"}},
	}
	abc, err := ConvertMidiToYueABC(in)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(abc, "% verse") {
		t.Fatalf("section comment missing: %s", abc)
	}
	if !strings.Contains(abc, "\"Dm7\"") {
		t.Fatalf("in-bar chord missing: %s", abc)
	}
	if !strings.Contains(abc, "_d8") || !strings.Contains(abc, "_b8") {
		t.Fatalf("flat spelling missing: %s", abc)
	}
	back, err := ParseYueABC(abc, "Ins")
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(back) != 4 {
		t.Fatalf("round trip count: %#v", back)
	}
	if back[0].Pitch != 61 || back[1].Pitch != 70 {
		t.Fatalf("chromatic pitches lost: %#v", back)
	}
	if back[2].Start != 2 || back[2].Length != 2 {
		t.Fatalf("sustained note mismatch: %#v", back[2])
	}
}

func TestYueABCDialectSampleParse(t *testing.T) {
	// A reduced shape of the validated dialect stimulus (dual voices, quoted
	// chords, section comments, Z multi-bar rests) must parse to the exact
	// melody notes.
	abc := "X:1\nT:\nM:4/4\nL:1/32\nQ:1/4=106\n" +
		"V: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n" +
		"V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\nK:C\n" +
		"% intro\nV: Vocal\nZ1|\nV: Ins\nZ2|\n" +
		"% verse\nV: Vocal\nZ2|\nV: Ins\n\"Dm7\"c4A4z12A4c4e4-|\"G7\"e4f8d12z8|\n"
	back, err := ParseYueABC(abc, "Ins")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The Ins voice rests 2 whole bars (Z2|), so the verse starts at bar 2
	// (beat 8 in 4/4): "Dm7"c4 A4 z12 A4 c4 e4- | "G7"e4 f8 d12 z8 |
	want := []struct {
		pitch int
		start float64
	}{{60, 8}, {57, 8.5}, {57, 10.5}, {60, 11}, {64, 11.5}, {65, 12.5}, {62, 13.5}}
	if len(back) != len(want) {
		t.Fatalf("note count %d != %d: %#v", len(back), len(want), back)
	}
	for i, w := range want {
		if back[i].Pitch != w.pitch || back[i].Start != w.start {
			t.Fatalf("note %d got (%d,%v) want (%d,%v)", i, back[i].Pitch, back[i].Start, w.pitch, w.start)
		}
	}
	if back[4].Length != 1 { // e4- + e4 merged = 8 units = 1 beat
		t.Fatalf("tie merge lost: %#v", back[4])
	}
}

func TestYueABCErrors(t *testing.T) {
	if _, err := ConvertMidiToYueABC(YueScoreInput{Notes: nil}); err == nil {
		t.Fatal("empty notes must fail")
	}
	if _, err := ConvertMidiToYueABC(YueScoreInput{Notes: []YueNote{{Pitch: 200, Start: 0, Length: 1}}}); err == nil {
		t.Fatal("out-of-range pitch must fail")
	}
	if _, err := ConvertMidiToYueABC(YueScoreInput{Notes: []YueNote{{Pitch: 60, Start: 0, Length: 0}}}); err == nil {
		t.Fatal("zero-length note must fail")
	}
}

func TestRestYueVocalPart(t *testing.T) {
	abc := "X:1\nM:4/4\nL:1/32\nK:C\nV: Vocal\nV: Ins\n% verse\n" +
		"V: Vocal\n\"Dm7\"c4A4z12A4c4e4-|z32|\nV: Ins\nc8d8e8f8|g8a8b8c'8|\n"
	out, changed, err := RestYueVocalPart(abc)
	if err != nil {
		t.Fatalf("rest: %v", err)
	}
	if !changed {
		t.Fatal("sounding vocal notes must be reported as rested")
	}
	// The rested vocal line keeps chords and bar structure, all silent.
	if !strings.Contains(out, "\"Dm7\"z32|z32|") {
		t.Fatalf("vocal line not fully rested: %s", out)
	}
	// Ins line untouched.
	if !strings.Contains(out, "c8d8e8f8|g8a8b8c'8|") {
		t.Fatalf("ins line must stay verbatim: %s", out)
	}
	if _, err := ParseYueABC(out, "Vocal"); err != nil {
		t.Fatalf("rested score must still parse: %v", err)
	}
	vocal, _ := ParseYueABC(out, "Vocal")
	if len(vocal) != 0 {
		t.Fatalf("rested vocal must parse silent: %#v", vocal)
	}
}

// YUE-INSTRUMENTAL-PATH-1 S2 substrate: % section comments -> official
// section-tag lyric form (title-cased, blank-line separated, trailing \n).
func TestYueSectionTagLyrics(t *testing.T) {
	abc := "X:1\nK:G\n% verse\nV: Vocal\n\"G\"z32|\n% pre-chorus\nV: Ins\nc4|\n% OUTRO\nV: Ins\nd4|\n"
	tags, ok := YueSectionTagLyrics(abc)
	if !ok {
		t.Fatalf("sections must be found: %s", abc)
	}
	want := "[Verse]\n\n[Pre-Chorus]\n\n[Outro]\n"
	if tags != want {
		t.Fatalf("tag form mismatch:\n got %q\nwant %q", tags, want)
	}
	if _, ok := YueSectionTagLyrics("X:1\nK:C\nV: Ins\nc4|"); ok {
		t.Fatalf("sectionless score must report no tags")
	}
}

// YUE-INSTRUMENTAL-PATH-1 S4 substrate: cot signal = quoted chords on the
// Vocal voice's music lines (Ins-line chords and header fields carry no
// cot signal).
func TestYueVocalHasChords(t *testing.T) {
	official := "X:1\nM:4/4\nL:1/32\nK:G\nV: Vocal\n\"G\"z32|\"D7\"z32|\nV: Ins\nd8e6d2B8z8|F8F8=F8z8|\n"
	if !YueVocalHasChords(official) {
		t.Fatalf("chord-annotated resting Vocal must report true")
	}
	insChords := "X:1\nM:4/4\nL:1/32\nK:C\n% verse\nV: Vocal\nZ2|\nV: Ins\n\"C\"c32|\"G\"e32|\n"
	if YueVocalHasChords(insChords) {
		t.Fatalf("Ins-line chords must not select cot=full: %s", insChords)
	}
	if YueVocalHasChords("X:1\nK:C\nT:\"quoted title\"\nV: Vocal\nZ1|\n") {
		t.Fatalf("header fields must not count as Vocal chords")
	}
	if YueVocalHasChords("") {
		t.Fatalf("empty score must report false")
	}
}
