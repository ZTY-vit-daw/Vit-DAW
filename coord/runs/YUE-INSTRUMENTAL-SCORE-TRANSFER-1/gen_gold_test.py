#!/usr/bin/env python3
"""Emit yue_transfer_test.go with the gold ABC pair embedded verbatim."""
import io

GOLD_DIR = r"D:/Vit_DAW/coord/runs/YUE-INSTRUMENTAL-PATH-1/scripts-score-transfer-20261120"
OUT = r"D:/Vit_DAW_wt_pc440/agent/internal/genapi/yue_transfer_test.go"

def read(p):
    with io.open(p, "r", encoding="utf-8", newline="") as fh:
        return fh.read()

original = read(GOLD_DIR + "/original.abc")
instrumental = read(GOLD_DIR + "/instrumental.abc")

# Go raw string literal escaping: backticks cannot appear in these scores,
# but a trailing backtick guard is still applied defensively.
def go_raw(s):
    assert "`" not in s, "gold sample contains a backtick; needs stitching"
    return "`" + s + "`"

TEMPLATE = '''package genapi

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

const goldOriginalABC = ''' + go_raw(original) + '''

const goldInstrumentalABC = ''' + go_raw(instrumental) + '''

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
		t.Fatalf("reparse: %v\\n%s", perr, text)
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
		t.Fatalf("section comment lost:\\n%s", text)
	}
}

// Fail-closed edges (no silent acceptance of off-dialect input).
func TestYueInstrumentalizeFailClosed(t *testing.T) {
	if _, _, err := YueInstrumentalizeTransfer("   "); err == nil || !strings.Contains(err.Error(), "nonempty native score") {
		t.Fatalf("empty input must fail: %v", err)
	}
	// Unsupported key fails closed.
	bad := strings.Replace(goldOriginalABC, "K:D\\n", "K:Q\\n", 1)
	if _, _, err := YueInstrumentalizeTransfer(bad); err == nil || !strings.Contains(err.Error(), "Unsupported key") {
		t.Fatalf("bad key must fail: %v", err)
	}
	// A score with rests only must fail.
	restsOnly := "X:1\\nT:\\nM:4/4\\nL:1/32\\nQ:1/4=90\\nV: Vocal clef=treble name=\\"Vocal Melody\\" snm=\\"Vocal\\"\\nV: Ins clef=treble name=\\"Ins Melody\\" snm=\\"Inst.\\"\\nK:C\\n% intro\\nV: Vocal\\n\\"C\\"z32|\\nV: Ins\\nZ|\\n"
	if _, _, err := YueInstrumentalizeTransfer(restsOnly); err == nil || !strings.Contains(err.Error(), "no sounding notes") {
		t.Fatalf("rests-only score must fail: %v", err)
	}
}
'''

with io.open(OUT, "w", encoding="utf-8", newline="\n") as fh:
    fh.write(TEMPLATE)

# Post-process: python string escapes above (\\n, \\") must become real Go escapes.
with io.open(OUT, "r", encoding="utf-8") as fh:
    content = fh.read()
# Only fix the fail-closed test body which used python-escaped sequences.
content = content.replace('strings.Replace(goldOriginalABC, "K:D\\n", "K:Q\\n", 1)',
                          'strings.Replace(goldOriginalABC, "K:D\\n", "K:Q\\n", 1)')
with io.open(OUT, "w", encoding="utf-8", newline="\n") as fh:
    fh.write(content)
print("written", OUT)
