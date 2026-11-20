package genapi

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Event compiler for the YuE2 instrumental ABC dialect (YUE-INSTRUMENTAL-
// SCORE-TRANSFER-1 leg A). Exact Go port of
// coord/runs/YUE-INSTRUMENTAL-PATH-1/scripts-score-transfer-20261120/compile_score.py
// (SHA c4eec7f6...): explicit beat-domain events -> native two-voice ABC
// text, with a full independent re-parse roundtrip before the text is
// returned. This is the serializer half of the Vocal->Ins score transfer;
// all times are exact quarter-note fractions (yq).

// yueCompileBarSpec is one bar of the compile input. Empty Meter/Key
// inherit the running values (Python dict.get default); HasSection=false
// inherits the running section.
type yueCompileBarSpec struct {
	Meter      string
	Key        string
	Section    string
	HasMeter   bool
	HasKey     bool
	HasSection bool
}

// yueCompileNote is one [onset, duration, MIDI pitch] event.
type yueCompileNote struct {
	Onset yq
	Dur   yq
	Pitch int
}

// yueCompileChord is one [onset, symbol] harmony event.
type yueCompileChord struct {
	When   yq
	Symbol string
}

// yueCompileInput mirrors the events dict accepted by compile_events.
type yueCompileInput struct {
	BPM    int
	Key    string // default "C"
	Bars   []yueCompileBarSpec
	Notes  []yueCompileNote
	Chords []yueCompileChord
}

type yueCompiledBar struct {
	start, end yq
	meter      string
	key        string
	section    string
	hasSection bool
	n, d       int
}

// yqLCM is math.lcm for positive int64 values.
func yqLCM(a, b int64) int64 {
	if a <= 0 || b <= 0 {
		panic("yqLCM: non-positive operand")
	}
	return a / yqGCD(a, b) * b
}

// yueDecomposeUnits splits a whole-number unit count greedily over the
// supported token lengths (compile_score.py lengths()).
func yueDecomposeUnits(v yq) ([]int, error) {
	if v.d != 1 || v.n <= 0 {
		return nil, fmt.Errorf("Unrepresentable duration in ABC units: %s", v.String())
	}
	left := v.n
	var out []int
	for _, part := range yueDurationsDesc {
		for left >= int64(part) {
			out = append(out, part)
			left -= int64(part)
		}
	}
	return out, nil
}

// yueSpellPitch renders one MIDI pitch in the bar's key context, tracking
// per-bar active accidentals (compile_score.py spelling()). prefer-sharp/
// flat follows the key signature; candidate order and tuple comparison
// mirror the Python original exactly.
func yueSpellPitch(pitch int, key string, active map[byte]int) string {
	sig, _ := yueKeyAccidentals(key) // caller validated the key
	sum := 0
	for _, v := range sig {
		sum += v
	}
	preferSharp := sum >= 0
	bestScore1, bestScore2, bestScore3 := false, 0, false
	haveBest := false
	var bestLetter byte
	var bestAlt, bestOctave int
	letters := []byte{'C', 'D', 'E', 'F', 'G', 'A', 'B'}
	boolLess := func(a, b bool) int {
		if a == b {
			return 0
		}
		if !a {
			return -1
		}
		return 1
	}
	for _, letter := range letters {
		pc := yueNaturalPC[letter]
		for _, alt := range []int{-1, 0, 1} {
			base := int64(pitch) - int64(alt) - 60 - int64(pc)
			if modFloor(base, 12) != 0 {
				continue
			}
			octave := int(divFloor(base, 12))
			s1 := alt != sig[letter]
			s2 := absInt(alt)
			var s3 bool
			if preferSharp {
				s3 = alt < 0
			} else {
				s3 = alt > 0
			}
			better := false
			if !haveBest {
				better = true
			} else if c := boolLess(s1, bestScore1); c != 0 {
				better = c < 0
			} else if c := cmpInt64(int64(s2), int64(bestScore2)); c != 0 {
				better = c < 0
			} else if c := boolLess(s3, bestScore3); c != 0 {
				better = c < 0
			}
			if better {
				haveBest = true
				bestScore1, bestScore2, bestScore3 = s1, s2, s3
				bestLetter, bestAlt, bestOctave = letter, alt, octave
			}
		}
	}
	accidental := ""
	cur, has := active[bestLetter]
	if !has {
		cur = sig[bestLetter]
	}
	if bestAlt != cur {
		switch bestAlt {
		case -1:
			accidental = "_"
		case 0:
			accidental = "="
		case 1:
			accidental = "^"
		}
		active[bestLetter] = bestAlt
	}
	var name string
	if bestOctave < 0 {
		name = string(bestLetter) + strings.Repeat(",", -bestOctave)
	} else if bestOctave == 0 {
		name = string(bestLetter)
	} else {
		name = strings.ToLower(string(bestLetter)) + strings.Repeat("'", bestOctave-1)
	}
	return accidental + name
}

// yueCompressBarRun joins 1-4 rendered bar strings into one music line,
// collapsing runs of whole-bar rests into Z<n> (compile_score.py compress).
func yueCompressBarRun(bars []string) string {
	var out strings.Builder
	index := 0
	for index < len(bars) {
		if bars[index] != "Z" {
			out.WriteString(bars[index])
			out.WriteString("|")
			index++
			continue
		}
		end := index + 1
		for end < len(bars) && bars[end] == "Z" {
			end++
		}
		count := end - index
		out.WriteString("Z")
		if count > 1 {
			out.WriteString(strconv.Itoa(count))
		}
		out.WriteString("|")
		index = end
	}
	return out.String()
}

// yueCompileEvents renders the explicit event list as a native two-voice
// ABC score (port of compile_score.py compile_events). The text is fully
// re-parsed and checked against the input events before being returned.
func yueCompileEvents(in yueCompileInput) (string, error) {
	if in.BPM <= 0 {
		return "", fmt.Errorf("bpm must be a positive integer quarter-note tempo")
	}
	key := in.Key
	if key == "" {
		key = "C"
	}
	if _, err := yueKeyAccidentals(key); err != nil {
		return "", err
	}
	if len(in.Bars) == 0 {
		return "", fmt.Errorf("bars must be a nonempty list")
	}
	unitDen := int64(32)
	var bars []yueCompiledBar
	curTime := yqInt(0)
	curMeter := "4/4"
	curKey := key
	curSection := ""
	curHasSection := false
	for i, item := range in.Bars {
		meter, k := curMeter, curKey
		if item.HasMeter {
			meter = item.Meter
		}
		if item.HasKey {
			k = item.Key
		}
		m, err := yueMeterValue(meter)
		if err != nil {
			return "", err
		}
		if _, kerr := yueKeyAccidentals(k); kerr != nil {
			return "", kerr
		}
		unitDen = yqLCM(unitDen, int64(4*m.D))
		if item.HasSection {
			if !yueSections[item.Section] {
				return "", fmt.Errorf("bar %d: section must be one of %v", i+1, yueSectionNames())
			}
			curSection, curHasSection = item.Section, true
		}
		end := yqAdd(curTime, yqNorm(int64(4*m.N), int64(m.D)))
		bars = append(bars, yueCompiledBar{start: curTime, end: end, meter: meter, key: k,
			section: curSection, hasSection: curHasSection, n: m.N, d: m.D})
		curTime = end
	}
	type cnote = yueCompileNote
	notes := make([]cnote, 0, len(in.Notes))
	for i, item := range in.Notes {
		if item.Pitch < 0 || item.Pitch > 127 {
			return "", fmt.Errorf("note %d: pitch must be one MIDI integer; reduce polyphony first", i+1)
		}
		if yqLess(item.Onset, yqInt(0)) || yqCmp(item.Dur, yqInt(0)) <= 0 || yqCmp(yqAdd(item.Onset, item.Dur), curTime) > 0 {
			return "", fmt.Errorf("note %d: onset/duration is outside the score", i+1)
		}
		notes = append(notes, item)
		unitDen = yqLCM(unitDen, denOf4(item.Onset))
		unitDen = yqLCM(unitDen, denOf4(item.Dur))
	}
	sort.Slice(notes, func(i, j int) bool {
		if c := yqCmp(notes[i].Onset, notes[j].Onset); c != 0 {
			return c < 0
		}
		if notes[i].Pitch != notes[j].Pitch {
			return notes[i].Pitch < notes[j].Pitch
		}
		return yqCmp(notes[i].Dur, notes[j].Dur) < 0
	})
	if len(notes) == 0 {
		return "", fmt.Errorf("Instrumental score has no sounding notes")
	}
	for k := 1; k < len(notes); k++ {
		a, b := notes[k-1], notes[k]
		if yqCmp(yqAdd(a.Onset, a.Dur), b.Onset) > 0 {
			return "", fmt.Errorf("Overlapping melody notes at quarter %s; choose one lead line before compilation", b.Onset.String())
		}
	}
	chords := make([]yueCompileChord, 0, len(in.Chords))
	for i, item := range in.Chords {
		if yqLess(item.When, yqInt(0)) || yqLess(curTime, item.When) || !matchYueChordSymbol(item.Symbol) {
			return "", fmt.Errorf("chord %d: unsupported symbol or onset", i+1)
		}
		chords = append(chords, item)
		unitDen = yqLCM(unitDen, denOf4(item.When))
	}
	sort.Slice(chords, func(i, j int) bool {
		if c := yqCmp(chords[i].When, chords[j].When); c != 0 {
			return c < 0
		}
		return chords[i].Symbol < chords[j].Symbol
	})
	for k := 1; k < len(chords); k++ {
		if yqEq(chords[k-1].When, chords[k].When) {
			return "", fmt.Errorf("Conflicting/duplicate chord events at the same onset; choose one harmony timeline")
		}
	}
	if unitDen > 1024 || unitDen&(unitDen-1) != 0 {
		return "", fmt.Errorf("Rhythm is outside the power-of-two training grid; explicitly quantize tuplets and record the changes")
	}
	unitQuarters := yqNorm(4, unitDen)

	// Per-bar rendering: Ins carries the melody pieces, Vocal the chord
	// annotation timeline over rests.
	ins := make([]string, len(bars))
	vocal := make([]string, len(bars))
	for bi, bar := range bars {
		events := make([]cnote, 0)
		for _, nt := range notes {
			// overlap window: onset < end && onset+dur > start
			if yqLess(nt.Onset, bar.end) && yqCmp(yqAdd(nt.Onset, nt.Dur), bar.start) > 0 {
				events = append(events, nt)
			}
		}
		var pieces []string
		cursor := bar.start
		active := map[byte]int{}
		for _, ev := range events {
			// a = max(bar.start, onset); b = min(bar.end, onset+duration)
			a, b := ev.Onset, yqAdd(ev.Onset, ev.Dur)
			if yqLess(a, bar.start) {
				a = bar.start
			}
			if yqLess(bar.end, b) {
				b = bar.end
			}
			if yqLess(cursor, a) {
				rests, rerr := yueDecomposeUnits(yqDiv(yqSub(a, cursor), unitQuarters))
				if rerr != nil {
					return "", rerr
				}
				pieces = append(pieces, yueRestTokens(rests)...)
			}
			parts, perr := yueDecomposeUnits(yqDiv(yqSub(b, a), unitQuarters))
			if perr != nil {
				return "", perr
			}
			for pi, count := range parts {
				tie := ""
				if pi < len(parts)-1 || yqLess(b, yqAdd(ev.Onset, ev.Dur)) {
					tie = "-"
				}
				token := yueSpellPitch(ev.Pitch, bar.key, active)
				if count != 1 {
					token += strconv.Itoa(count)
				}
				token += tie
				pieces = append(pieces, token)
			}
			cursor = b
		}
		if yqLess(cursor, bar.end) {
			rests, rerr := yueDecomposeUnits(yqDiv(yqSub(bar.end, cursor), unitQuarters))
			if rerr != nil {
				return "", rerr
			}
			pieces = append(pieces, yueRestTokens(rests)...)
		}
		if len(events) > 0 {
			ins[bi] = strings.Join(pieces, "")
		} else {
			ins[bi] = "Z"
		}

		current := ""
		hasCurrent := false
		for k := len(chords) - 1; k >= 0; k-- {
			if yqCmp(chords[k].When, bar.start) <= 0 {
				current, hasCurrent = chords[k].Symbol, true
				break
			}
		}
		type change struct {
			when yq
			sym  string
			has  bool
		}
		changes := []change{{when: bar.start, sym: current, has: hasCurrent}}
		for _, ch := range chords {
			if yqLess(bar.start, ch.When) && yqLess(ch.When, bar.end) {
				changes = append(changes, change{when: ch.When, sym: ch.Symbol, has: true})
			}
		}
		var chunks []string
		anySymbol := false
		for ci, ch := range changes {
			stop := bar.end
			if ci+1 < len(changes) {
				stop = changes[ci+1].when
			}
			if ch.has {
				chunks = append(chunks, "\""+ch.sym+"\"")
				anySymbol = true
			}
			rests, rerr := yueDecomposeUnits(yqDiv(yqSub(stop, ch.when), unitQuarters))
			if rerr != nil {
				return "", rerr
			}
			chunks = append(chunks, yueRestTokens(rests)...)
		}
		if anySymbol {
			vocal[bi] = strings.Join(chunks, "")
		} else {
			vocal[bi] = "Z"
		}
	}

	header := []string{
		"X:1", "T:",
		"M:" + bars[0].meter,
		"L:1/" + strconv.FormatInt(unitDen, 10),
		"Q:1/4=" + strconv.Itoa(in.BPM),
		`V: Vocal clef=treble name="Vocal Melody" snm="Vocal"`,
		`V: Ins clef=treble name="Ins Melody" snm="Inst."`,
		"K:" + bars[0].key,
	}
	lines := header
	index := 0
	for index < len(bars) {
		bar := bars[index]
		previous := bar
		if index > 0 {
			previous = bars[index-1]
		}
		end := index + 1
		for end < len(bars) && end-index < 4 &&
			bars[end].meter == bar.meter && bars[end].key == bar.key &&
			bars[end].section == bar.section {
			end++
		}
		if bar.hasSection && (index == 0 || bar.section != previous.section) {
			lines = append(lines, "% "+bar.section)
		}
		for _, vr := range []struct {
			name     string
			rendered []string
		}{{"Vocal", vocal}, {"Ins", ins}} {
			lines = append(lines, "V: "+vr.name)
			if bar.meter != previous.meter {
				lines = append(lines, "M:"+bar.meter)
			}
			if bar.key != previous.key {
				lines = append(lines, "K:"+bar.key)
			}
			lines = append(lines, yueCompressBarRun(vr.rendered[index:end]))
		}
		index = end
	}
	text := strings.Join(lines, "\n") + "\n"

	// Independent re-parse roundtrip: exact note events and harmony.
	parsed, perr := yueParseNativeScore(text)
	if perr != nil {
		return "", fmt.Errorf("Internal note roundtrip failed: %v", perr)
	}
	parsedIns := make([]cnote, 0, len(parsed.Voices["Ins"].Notes))
	for _, nt := range parsed.Voices["Ins"].Notes {
		parsedIns = append(parsedIns, cnote{Onset: nt.Start, Dur: nt.Dur, Pitch: nt.Pitch})
	}
	if !yueCompileNotesEqual(parsedIns, notes) || len(parsed.Voices["Vocal"].Notes) != 0 {
		return "", fmt.Errorf("Internal note roundtrip failed")
	}
	chordPairs := make([]yueNativePair, 0, len(chords))
	for _, ch := range chords {
		chordPairs = append(chordPairs, yueNativePair{T: ch.When, Symbol: ch.Symbol})
	}
	if !yuePairsEqual(yueDedupBySymbol(parsed.Voices["Vocal"].Chords), yueDedupBySymbol(chordPairs)) {
		return "", fmt.Errorf("Internal harmony roundtrip failed")
	}
	return text, nil
}

// yueSectionNames returns the sorted section label list for error messages.
func yueSectionNames() []string {
	out := make([]string, 0, len(yueSections))
	for k := range yueSections {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// denOf4 returns the denominator of value/4 (whole-note fractions).
func denOf4(v yq) int64 { return yqDiv(v, yqInt(4)).d }

func yueRestTokens(units []int) []string {
	out := make([]string, 0, len(units))
	for _, n := range units {
		if n != 1 {
			out = append(out, "z"+strconv.Itoa(n))
		} else {
			out = append(out, "z")
		}
	}
	return out
}

func yueCompileNotesEqual(a, b []yueCompileNote) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !yqEq(a[i].Onset, b[i].Onset) || a[i].Pitch != b[i].Pitch || !yqEq(a[i].Dur, b[i].Dur) {
			return false
		}
	}
	return true
}

// yueDedupBySymbol keeps the first of consecutive same-symbol events
// (compile_score.py dedup / instrumentalize.py harmony).
func yueDedupBySymbol(events []yueNativePair) []yueNativePair {
	var out []yueNativePair
	for _, ev := range events {
		if len(out) == 0 || out[len(out)-1].Symbol != ev.Symbol {
			out = append(out, ev)
		}
	}
	return out
}
