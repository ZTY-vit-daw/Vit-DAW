package genapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Vocal->Ins score transfer (YUE-INSTRUMENTAL-SCORE-TRANSFER-1 leg A): the
// fifth instrumental route proven by the user+Codex experiments of
// 2026-11-20 (coord/runs/YUE-INSTRUMENTAL-PATH-1/standalone_instrumental_repeat_20261120_214808,
// user-ear confirmed pure instrumental). Exact Go port of the reference
// tools' instrumentalize.py (SHA a7691116..., script trio on the same
// conversion.json receipt): every sounding Vocal note moves to the Ins
// voice (vocal-priority overlap policy), chords / meter / tempo / bar grid
// are preserved, and the compiled score is independently re-parsed and
// reconciled before it is returned.
//
// This is score-structure work, not an engine switch: the transferred
// score carries ZERO sounding Vocal notes, so the pass2 fixed-score
// re-render's vocal suppression is structural. The token-editing route was
// falsified before this port (standalone_instrument_token_edit_20261120_215640);
// do not replace this deterministic transfer with token editing.

// YueRetainedSegment is one [start, length] quarter-note pair (string
// fractions, conversion.json shape).
type YueRetainedSegment [2]string

// YueAffectedInsNote records one Ins note the vocal occupancy overlapped.
type YueAffectedInsNote struct {
	InsNote          int                   `json:"ins_note"`
	Onset            string                `json:"onset"`
	Pitch            int                   `json:"pitch"`
	Duration         string                `json:"duration"`
	RetainedSegments []YueRetainedSegment  `json:"retained_segments"`
}

// YueTransferReceipt is the conversion receipt (conversion.json shape plus
// the bpm/bars the success-run driver added). It is the job-archive face of
// the two-stage instrumental route.
type YueTransferReceipt struct {
	VocalNotesBefore                 int                  `json:"vocal_notes_before"`
	VocalNotesAfter                  int                  `json:"vocal_notes_after"`
	OriginalInsNotes                 int                  `json:"original_ins_notes"`
	UnalteredInsNotes                int                  `json:"unaltered_ins_notes"`
	OutputInsNotes                   int                  `json:"output_ins_notes"`
	VocalPitchOnsetDurationPreserved bool                 `json:"vocal_pitch_onset_duration_preserved"`
	MeterAndTempoPreserved           bool                 `json:"meter_and_tempo_preserved"`
	Chords                           string               `json:"chords"`
	OverlapPolicy                    string               `json:"overlap_policy"`
	AffectedInsNotes                 []YueAffectedInsNote `json:"affected_ins_notes"`
	NominalSeconds                   float64              `json:"nominal_seconds"`
	BPM                              int                  `json:"bpm"`
	Bars                             int                  `json:"bars"`
}

// yueSubtractIntervals subtracts every occupied interval from
// [start, start+duration) (instrumentalize.py subtract_intervals).
func yueSubtractIntervals(start, duration yq, occupied [][2]yq) [][2]yq {
	pieces := [][2]yq{{start, yqAdd(start, duration)}}
	for _, occ := range occupied {
		left, right := occ[0], occ[1]
		var remaining [][2]yq
		for _, p := range pieces {
			a, b := p[0], p[1]
			if yqCmp(right, a) <= 0 || yqCmp(left, b) >= 0 {
				remaining = append(remaining, p)
				continue
			}
			if yqLess(a, left) {
				remaining = append(remaining, [2]yq{a, left})
			}
			if yqLess(right, b) {
				remaining = append(remaining, [2]yq{right, b})
			}
		}
		pieces = remaining
	}
	return pieces
}

// yueSectionStarts maps the score's native % section comments to their
// absolute bar-start times (instrumentalize.py section_starts).
func yueSectionStarts(text string, score *yueNativeScore) (map[yq]string, error) {
	cursor := 0
	pending := ""
	hasPending := false
	result := map[yq]string{}
	for index, line := range yueSplitLines(text) {
		if strings.HasPrefix(line, "% ") {
			pending = line[2:]
			if !yueSections[pending] {
				return nil, fmt.Errorf("Unknown section label: %s", pending)
			}
			hasPending = true
		}
		if score.MusicLines[index] != "Vocal" {
			continue
		}
		if hasPending {
			bars := score.Voices["Vocal"].Bars
			if cursor >= len(bars) {
				return nil, fmt.Errorf("section comment mapping overran the Vocal bar grid")
			}
			result[bars[cursor].Start] = pending
			hasPending = false
		}
		for _, bar := range strings.Split(line[:len(line)-1], "|") {
			if n, ok := yueZRestCount(strings.TrimSpace(bar)); ok {
				cursor += n
			} else {
				cursor++
			}
		}
	}
	return result, nil
}

// YueInstrumentalizeTransfer moves every sounding Vocal note of the native
// dialect score to the Ins voice (overlap policy "vocal", chords kept) and
// returns the transferred score text plus its reconciliation receipt. All
// of the reference implementation's post-transfer invariants are enforced:
// the transferred Ins notes equal the merged note list exactly, the Vocal
// voice is silent, the bar/meter grid and tempo are untouched, every vocal
// note survives verbatim in Ins, and the active harmony timeline is
// preserved.
func YueInstrumentalizeTransfer(text string) (string, *YueTransferReceipt, error) {
	if strings.TrimSpace(text) == "" {
		return "", nil, fmt.Errorf("Need a nonempty native score")
	}
	source, err := yueParseNativeScore(text)
	if err != nil {
		return "", nil, err
	}
	vocal := source.Voices["Vocal"]
	ins := source.Voices["Ins"]
	if len(vocal.Notes) == 0 && len(ins.Notes) == 0 {
		return "", nil, fmt.Errorf("The score contains no sounding notes")
	}
	occupied := make([][2]yq, 0, len(vocal.Notes))
	for _, nt := range vocal.Notes {
		occupied = append(occupied, [2]yq{nt.Start, yqAdd(nt.Start, nt.Dur)})
	}
	merged := make([]yueNativeNote, len(vocal.Notes), len(vocal.Notes)+len(ins.Notes))
	copy(merged, vocal.Notes)
	var affected []YueAffectedInsNote
	unchanged := 0
	for index, in := range ins.Notes {
		pieces := yueSubtractIntervals(in.Start, in.Dur, occupied)
		full := len(pieces) == 1 && yqEq(pieces[0][0], in.Start) && yqEq(pieces[0][1], yqAdd(in.Start, in.Dur))
		if full {
			unchanged++
		} else {
			segs := make([]YueRetainedSegment, 0, len(pieces))
			for _, p := range pieces {
				segs = append(segs, YueRetainedSegment{p[0].String(), yqSub(p[1], p[0]).String()})
			}
			affected = append(affected, YueAffectedInsNote{
				InsNote:          index,
				Onset:            in.Start.String(),
				Pitch:            in.Pitch,
				Duration:         in.Dur.String(),
				RetainedSegments: segs,
			})
		}
		for _, p := range pieces {
			merged = append(merged, yueNativeNote{Start: p[0], Pitch: in.Pitch, Dur: yqSub(p[1], p[0])})
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if c := yqCmp(merged[i].Start, merged[j].Start); c != 0 {
			return c < 0
		}
		if merged[i].Pitch != merged[j].Pitch {
			return merged[i].Pitch < merged[j].Pitch
		}
		return yqCmp(merged[i].Dur, merged[j].Dur) < 0
	})
	sections, serr := yueSectionStarts(text, source)
	if serr != nil {
		return "", nil, serr
	}
	starts := map[yq]bool{}
	for _, bar := range vocal.Bars {
		starts[bar.Start] = true
	}
	for _, k := range vocal.Keys {
		if !starts[k.T] {
			return "", nil, fmt.Errorf("An inline key change occurs inside a bar; normalize its spelling without changing pitches first")
		}
	}
	bars := make([]yueCompileBarSpec, 0, len(vocal.Bars))
	for _, bar := range vocal.Bars {
		key := vocal.Keys[len(vocal.Keys)-1].Symbol
		for _, k := range vocal.Keys {
			if yqCmp(k.T, bar.Start) <= 0 {
				key = k.Symbol
			}
		}
		spec := yueCompileBarSpec{Meter: fmt.Sprintf("%d/%d", bar.Meter.N, bar.Meter.D), Key: key, HasMeter: true, HasKey: true}
		if s, ok := sections[bar.Start]; ok {
			spec.Section, spec.HasSection = s, true
		}
		bars = append(bars, spec)
	}
	var chordEvents []yueCompileChord
	for _, ch := range vocal.Chords {
		if len(chordEvents) > 0 && chordEvents[len(chordEvents)-1].When.String() == ch.T.String() {
			if chordEvents[len(chordEvents)-1].Symbol != ch.Symbol {
				return "", nil, fmt.Errorf("Conflicting chord labels at the same time")
			}
			continue
		}
		chordEvents = append(chordEvents, yueCompileChord{When: ch.T, Symbol: ch.Symbol})
	}
	notes := make([]yueCompileNote, 0, len(merged))
	for _, nt := range merged {
		notes = append(notes, yueCompileNote{Onset: nt.Start, Dur: nt.Dur, Pitch: nt.Pitch})
	}
	converted, cerr := yueCompileEvents(yueCompileInput{
		BPM:    source.BPM,
		Key:    vocal.Keys[0].Symbol,
		Bars:   bars,
		Notes:  notes,
		Chords: chordEvents,
	})
	if cerr != nil {
		return "", nil, cerr
	}
	target, terr := yueParseNativeScore(converted)
	if terr != nil {
		return "", nil, fmt.Errorf("Voice transfer changed the intended note events: %v", terr)
	}
	targetIns := target.Voices["Ins"]
	if !yueNotesEqual(targetIns.Notes, merged) || len(target.Voices["Vocal"].Notes) != 0 {
		return "", nil, fmt.Errorf("Voice transfer changed the intended note events")
	}
	if !yueBarsEqual(targetIns.Bars, ins.Bars) || target.BPM != source.BPM {
		return "", nil, fmt.Errorf("Voice transfer changed meter, timing or tempo")
	}
	for _, nt := range vocal.Notes {
		found := false
		for _, m := range targetIns.Notes {
			if yqEq(m.Start, nt.Start) && m.Pitch == nt.Pitch && yqEq(m.Dur, nt.Dur) {
				found = true
				break
			}
		}
		if !found {
			return "", nil, fmt.Errorf("A vocal note was not preserved exactly")
		}
	}
	if !yuePairsEqual(yueDedupBySymbol(target.Voices["Vocal"].Chords), yueDedupBySymbol(vocal.Chords)) {
		return "", nil, fmt.Errorf("Voice transfer changed harmony")
	}
	// nominal_seconds = float(ins.time * 60 / bpm) — one exact-rational
	// rounding, matching Python float(Fraction).
	nominal := yqDiv(yqMul(ins.Time, yqInt(60)), yqInt(int64(source.BPM)))
	if affected == nil {
		affected = []YueAffectedInsNote{}
	}
	receipt := &YueTransferReceipt{
		VocalNotesBefore:                 len(vocal.Notes),
		VocalNotesAfter:                  0,
		OriginalInsNotes:                 len(ins.Notes),
		UnalteredInsNotes:                unchanged,
		OutputInsNotes:                   len(merged),
		VocalPitchOnsetDurationPreserved: true,
		MeterAndTempoPreserved:           true,
		Chords:                           "preserved",
		OverlapPolicy:                    "vocal",
		AffectedInsNotes:                 affected,
		NominalSeconds:                   nominal.Float(),
		BPM:                              source.BPM,
		Bars:                             len(vocal.Bars),
	}
	return converted, receipt, nil
}

// ReceiptMap renders the receipt as the generic map face used by job
// archives and harness disclosures (json-tag roundtrip).
func (r *YueTransferReceipt) ReceiptMap() map[string]any {
	raw, _ := json.Marshal(r)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
