package genapi

import (
	"sort"
	"strings"
	"testing"
)

// Gold-pair reconciliation for the Vocal->Ins score transfer
// (YUE-INSTRUMENTAL-SCORE-TRANSFER-1 leg A). The two scores below are the
// byte-exact gold pair from the user+Codex success round of 2026-11-20:
// coord/runs/YUE-INSTRUMENTAL-PATH-1/scripts-score-transfer-20261120/
// {original.abc -> instrumental.abc} (conversion.json: original_abc_sha256
// f96d5568..., conversion_script_sha256 instrumentalize.py a7691116...).
// The assertions pin, against the reference receipt: vocal 197 -> 0, Ins
// 78 -> 275, per-note pitch/onset/duration preservation, and chord/meter/
// tempo preservation. Byte-exact output is the transfer target; failing
// this pair is a stop-the-card condition (no assertion relaxation).

const goldOriginalABC = `X:1
T:
M:3/4
L:1/32
Q:1/4=87
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"
V: Ins clef=treble name="Ins Melody" snm="Inst."
K:D
% intro
V: Vocal
z16z4"D"z4|"D"z24|"D"z24|"D"z24|
V: Ins
Z|D6A,4A,2D6A,4A,2|D,6A,4A,2E2F2E2D2C2D2|D,6A,4A,2D6A,4A,2|
V: Vocal
"D"z24|"Bm7"z24|"Bm7"z24|"D"z24|
V: Ins
D,6A,4A,2D4E4F4|A16z4F4|E16D4C4|D24-|
V: Vocal
"D"z24|"Bm7"z24|"Bm7"z24|"D"z24|
V: Ins
D8z4D4E4F4|A16z4F4|E16D4C4|D24-|
V: Vocal
"D"z8d4d6a6|
V: Ins
D8z16|
% verse
V: Vocal
"D"a8f2f2f6e6|"D"e8d4d6a6|"Bm7"a8f4f2e8z2|"Bm7"d8z2d2d6a6|
V: Ins
Z4|
V: Vocal
"G"a8f4f4e2e6|"G"d4z4d4d6f6|"A"f8e4e2d8z2|"A"e8z16|
V: Ins
Z4|
V: Vocal
"A"z12D4E2F6|"G"A8z2F2F2E8z2|"G"D8z4D4E2F6|"Bm7"A8F2F2F4E8|
V: Ins
Z4|
V: Vocal
"Bm7"D8z4D4E2F6|"G"A8F4F4E2D6|"G"D8z4D4E2F6|"A"F4E8E8D4|
V: Ins
Z4|
V: Vocal
"A"E8z16|"A"z8d4d6a6|
V: Ins
Z2|
% chorus
V: Vocal
"D"a8f4f6e6|"D"d8d4d6a6|"Bm7"a8f4f6e6|"Bm7"d8z4d4e2f6|
V: Ins
Z4|
V: Vocal
"G"a8f4f6e6|"G"d8z4d4e2f6|"A"f8e4e2d8z2|"A"e16z8|
V: Ins
Z4|
V: Vocal
"A"z24|
V: Ins
z12D4E4F4|
% interlude
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z4D4E4F4|
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z8d4d6a6|
V: Ins
A16z4F4|E16D4C4|D24-|D8z16|
% verse
V: Vocal
"D"a8f4f6e6|"D"e8d4d6a6|"Bm7"a8f4f4e8|"Bm7"d4z4d4d6a6|
V: Ins
Z4|
V: Vocal
"G"a8f4f4e8|"G"d4z4d4d6f6|"A"f8e4e4d8|"A"e8z16|
V: Ins
Z4|
V: Vocal
"A"z8D4D6A6|"D"A8F4F6E6|"D"D8D4D6A6|"Bm7"A8F4F4E8|
V: Ins
Z4|
V: Vocal
"Bm7"D8z4D4E2F6|"G"A8F4F4E2D6|"G"D8z4D4E2F6|"A"F8E4E4D8|
V: Ins
Z4|
V: Vocal
"A"E8z16|"A"z8d4d6a6|
V: Ins
Z2|
% chorus
V: Vocal
"D"a8f4f6e6|"D"d8d4d6a6|"Bm7"a8f4f6e6|"Bm7"d8z4d4e2f6|
V: Ins
Z4|
V: Vocal
"G"a8f4f6e6|"G"d8z4d4e2f6|"A"f8e4e2d8z2|"A"e16z8|
V: Ins
Z4|
V: Vocal
"A"z24|
V: Ins
z12D4E4F4|
% outro
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z4D4E4F4|
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z16|
V: Vocal
"D"z24|
V: Ins
Z|
`

const goldInstrumentalABC = `X:1
T:
M:3/4
L:1/32
Q:1/4=87
V: Vocal clef=treble name="Vocal Melody" snm="Vocal"
V: Ins clef=treble name="Ins Melody" snm="Inst."
K:D
% intro
V: Vocal
z16z4"D"z4|"D"z24|"D"z24|"D"z24|
V: Ins
Z|D6A,4A,2D6A,4A,2|D,6A,4A,2E2F2E2D2C2D2|D,6A,4A,2D6A,4A,2|
V: Vocal
"D"z24|"Bm7"z24|"Bm7"z24|"D"z24|
V: Ins
D,6A,4A,2D4E4F4|A16z4F4|E16D4C4|D24-|
V: Vocal
"D"z24|"Bm7"z24|"Bm7"z24|"D"z24|
V: Ins
D8z4D4E4F4|A16z4F4|E16D4C4|D24-|
V: Vocal
"D"z24|
V: Ins
D8d4d6a6|
% verse
V: Vocal
"D"z24|"D"z24|"Bm7"z24|"Bm7"z24|
V: Ins
a8f2f2f6e6|e8d4d6a6|a8f4f2e8z2|d8z2d2d6a6|
V: Vocal
"G"z24|"G"z24|"A"z24|"A"z24|
V: Ins
a8f4f4e2e6|d4z4d4d6f6|f8e4e2d8z2|e8z16|
V: Vocal
"A"z24|"G"z24|"G"z24|"Bm7"z24|
V: Ins
z12D4E2F6|A8z2F2F2E8z2|D8z4D4E2F6|A8F2F2F4E8|
V: Vocal
"Bm7"z24|"G"z24|"G"z24|"A"z24|
V: Ins
D8z4D4E2F6|A8F4F4E2D6|D8z4D4E2F6|F4E8E8D4|
V: Vocal
"A"z24|"A"z24|
V: Ins
E8z16|z8d4d6a6|
% chorus
V: Vocal
"D"z24|"D"z24|"Bm7"z24|"Bm7"z24|
V: Ins
a8f4f6e6|d8d4d6a6|a8f4f6e6|d8z4d4e2f6|
V: Vocal
"G"z24|"G"z24|"A"z24|"A"z24|
V: Ins
a8f4f6e6|d8z4d4e2f6|f8e4e2d8z2|e16z8|
V: Vocal
"A"z24|
V: Ins
z12D4E4F4|
% interlude
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z4D4E4F4|
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8d4d6a6|
% verse
V: Vocal
"D"z24|"D"z24|"Bm7"z24|"Bm7"z24|
V: Ins
a8f4f6e6|e8d4d6a6|a8f4f4e8|d4z4d4d6a6|
V: Vocal
"G"z24|"G"z24|"A"z24|"A"z24|
V: Ins
a8f4f4e8|d4z4d4d6f6|f8e4e4d8|e8z16|
V: Vocal
"A"z24|"D"z24|"D"z24|"Bm7"z24|
V: Ins
z8D4D6A6|A8F4F6E6|D8D4D6A6|A8F4F4E8|
V: Vocal
"Bm7"z24|"G"z24|"G"z24|"A"z24|
V: Ins
D8z4D4E2F6|A8F4F4E2D6|D8z4D4E2F6|F8E4E4D8|
V: Vocal
"A"z24|"A"z24|
V: Ins
E8z16|z8d4d6a6|
% chorus
V: Vocal
"D"z24|"D"z24|"Bm7"z24|"Bm7"z24|
V: Ins
a8f4f6e6|d8d4d6a6|a8f4f6e6|d8z4d4e2f6|
V: Vocal
"G"z24|"G"z24|"A"z24|"A"z24|
V: Ins
a8f4f6e6|d8z4d4e2f6|f8e4e2d8z2|e16z8|
V: Vocal
"A"z24|
V: Ins
z12D4E4F4|
% outro
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z4D4E4F4|
V: Vocal
"Bm7"z24|"Bm7"z24|"D"z24|"D"z24|
V: Ins
A16z4F4|E16D4C4|D24-|D8z16|
V: Vocal
"D"z24|
V: Ins
Z|
`

func TestYueInstrumentalizeGoldPairBytes(t *testing.T) {
	got, receipt, err := YueInstrumentalizeTransfer(goldOriginalABC)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if got != goldInstrumentalABC {
		t.Fatalf("transferred score is not byte-identical to the gold instrumental.abc (len got=%d want=%d)", len(got), len(goldInstrumentalABC))
	}
	if receipt.VocalNotesBefore != 197 || receipt.VocalNotesAfter != 0 {
		t.Fatalf("vocal conservation broken: %d -> %d (want 197 -> 0)", receipt.VocalNotesBefore, receipt.VocalNotesAfter)
	}
	if receipt.OriginalInsNotes != 78 || receipt.OutputInsNotes != 275 {
		t.Fatalf("ins conservation broken: %d -> %d (want 78 -> 275)", receipt.OriginalInsNotes, receipt.OutputInsNotes)
	}
	if receipt.UnalteredInsNotes != 78 {
		t.Fatalf("unaltered ins notes = %d, want 78 (no overlap in the gold pair)", receipt.UnalteredInsNotes)
	}
	if len(receipt.AffectedInsNotes) != 0 {
		t.Fatalf("affected ins notes must be empty for the gold pair: %#v", receipt.AffectedInsNotes)
	}
	if !receipt.VocalPitchOnsetDurationPreserved || !receipt.MeterAndTempoPreserved || receipt.Chords != "preserved" || receipt.OverlapPolicy != "vocal" {
		t.Fatalf("receipt flags wrong: %#v", receipt)
	}
	if receipt.BPM != 87 || receipt.Bars != 84 {
		t.Fatalf("receipt bpm/bars = %d/%d, want 87/84", receipt.BPM, receipt.Bars)
	}
	// 84 bars x 3 quarters x 60/87 s — exact rational then one rounding.
	wantNominal := yqDiv(yqMul(yqInt(252), yqInt(60)), yqInt(87)).Float()
	if receipt.NominalSeconds != wantNominal {
		t.Fatalf("nominal seconds = %v, want %v", receipt.NominalSeconds, wantNominal)
	}
}

func TestYueInstrumentalizeGoldPairPerNote(t *testing.T) {
	source, err := yueParseNativeScore(goldOriginalABC)
	if err != nil {
		t.Fatalf("parse original: %v", err)
	}
	target, err := yueParseNativeScore(goldInstrumentalABC)
	if err != nil {
		t.Fatalf("parse instrumental: %v", err)
	}
	tv, ti := target.Voices["Vocal"], target.Voices["Ins"]
	sv, si := source.Voices["Vocal"], source.Voices["Ins"]
	if len(tv.Notes) != 0 {
		t.Fatalf("transferred Vocal voice must be silent, got %d notes", len(tv.Notes))
	}
	if len(sv.Notes) != 197 || len(si.Notes) != 78 || len(ti.Notes) != 275 {
		t.Fatalf("note counts %d/%d/%d, want 197/78/275", len(sv.Notes), len(si.Notes), len(ti.Notes))
	}
	// Per-note pitch/onset/duration: the transferred Ins list is exactly the
	// sorted union of the source Vocal and Ins note lists (no overlap in the
	// gold pair).
	union := append(append([]yueNativeNote{}, sv.Notes...), si.Notes...)
	sort.Slice(union, func(i, j int) bool {
		if c := yqCmp(union[i].Start, union[j].Start); c != 0 {
			return c < 0
		}
		if union[i].Pitch != union[j].Pitch {
			return union[i].Pitch < union[j].Pitch
		}
		return yqCmp(union[i].Dur, union[j].Dur) < 0
	})
	if !yueNotesEqual(ti.Notes, union) {
		for k := range union {
			if k >= len(ti.Notes) || !yqEq(ti.Notes[k].Start, union[k].Start) || ti.Notes[k].Pitch != union[k].Pitch || !yqEq(ti.Notes[k].Dur, union[k].Dur) {
				t.Fatalf("note %d differs: got %v want %v", k, ti.Notes[k], union[k])
			}
		}
		t.Fatal("transferred Ins notes differ from the vocal+ins union")
	}
	// Chords preserved: same active-harmony timeline on the (rested) Vocal.
	if !yuePairsEqual(yueDedupBySymbol(tv.Chords), yueDedupBySymbol(sv.Chords)) {
		t.Fatalf("harmony changed: %v vs %v", tv.Chords, sv.Chords)
	}
	if len(tv.Chords) == 0 {
		t.Fatal("gold pair must carry chord annotations on the Vocal voice")
	}
	// Meter/tempo preserved: identical bar grids and bpm.
	if !yueBarsEqual(ti.Bars, si.Bars) || target.BPM != source.BPM {
		t.Fatal("meter/tempo changed")
	}
	if len(ti.Bars) != 84 {
		t.Fatalf("bar count %d, want 84", len(ti.Bars))
	}
	// Section comments preserved through the transfer.
	if got, _ := YueSectionTagLyrics(goldInstrumentalABC); !strings.Contains(got, "[Intro]") || !strings.Contains(got, "[Outro]") {
		t.Fatalf("section tags lost in the transferred score: %q", got)
	}
}

// The compiler half gets its own edge cases beyond the gold path: ties
// across barlines, sharps needing accidentals, chord symbols, mixed rests.
func TestYueCompileEventsRoundtripSmall(t *testing.T) {
	text, err := yueCompileEvents(yueCompileInput{
		BPM: 96,
		Key: "C",
		Bars: []yueCompileBarSpec{
			{Meter: "4/4", Key: "C", HasMeter: true, HasKey: true},
			{Meter: "4/4", Key: "C", HasMeter: true, HasKey: true},
			{Meter: "4/4", Key: "C", HasMeter: true, HasKey: true, Section: "chorus", HasSection: true},
		},
		Notes: []yueCompileNote{
			{Onset: yqInt(0), Dur: yqNorm(1, 1), Pitch: 60},
			{Onset: yqInt(1), Dur: yqNorm(1, 2), Pitch: 61},
			{Onset: yqNorm(3, 2), Dur: yqNorm(1, 2), Pitch: 62},
			{Onset: yqInt(2), Dur: yqInt(6), Pitch: 64}, // ties across the barline
			{Onset: yqInt(9), Dur: yqInt(2), Pitch: 67},
		},
		Chords: []yueCompileChord{
			{When: yqInt(0), Symbol: "C"},
			{When: yqInt(8), Symbol: "G7"},
		},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	parsed, perr := yueParseNativeScore(text)
	if perr != nil {
		t.Fatalf("reparse: %v\n%s", perr, text)
	}
	ins := parsed.Voices["Ins"]
	if len(ins.Notes) != 5 {
		t.Fatalf("notes %d, want 5: %#v", len(ins.Notes), ins.Notes)
	}
	if !yqEq(ins.Notes[3].Start, yqInt(2)) || ins.Notes[3].Pitch != 64 || !yqEq(ins.Notes[3].Dur, yqInt(6)) {
		t.Fatalf("cross-barline tie note wrong: %#v", ins.Notes[3])
	}
	if ins.Notes[1].Pitch != 61 || ins.Notes[2].Pitch != 62 {
		t.Fatalf("accidental spelling lost: %#v", ins.Notes)
	}
	if !yuePairsEqual(yueDedupBySymbol(parsed.Voices["Vocal"].Chords), []yueNativePair{{yqInt(0), "C"}, {yqInt(8), "G7"}}) {
		t.Fatalf("chords wrong: %#v", parsed.Voices["Vocal"].Chords)
	}
	if !strings.Contains(text, "% chorus") {
		t.Fatalf("section comment lost:\n%s", text)
	}
}

// Fail-closed edges (no silent acceptance of off-dialect input).
func TestYueInstrumentalizeFailClosed(t *testing.T) {
	if _, _, err := YueInstrumentalizeTransfer("   "); err == nil || !strings.Contains(err.Error(), "nonempty native score") {
		t.Fatalf("empty input must fail: %v", err)
	}
	// Unsupported key fails closed.
	bad := strings.Replace(goldOriginalABC, "K:D\n", "K:Q\n", 1)
	if _, _, err := YueInstrumentalizeTransfer(bad); err == nil || !strings.Contains(err.Error(), "Unsupported key") {
		t.Fatalf("bad key must fail: %v", err)
	}
	// A score with rests only must fail.
	restsOnly := "X:1\nT:\nM:4/4\nL:1/32\nQ:1/4=90\nV: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\nV: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\nK:C\n% intro\nV: Vocal\n\"C\"z32|\nV: Ins\nZ|\n"
	if _, _, err := YueInstrumentalizeTransfer(restsOnly); err == nil || !strings.Contains(err.Error(), "no sounding notes") {
		t.Fatalf("rests-only score must fail: %v", err)
	}
}
