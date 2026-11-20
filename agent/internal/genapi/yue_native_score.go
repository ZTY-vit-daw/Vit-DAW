package genapi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Native YuE2 ABC dialect parser (YUE-INSTRUMENTAL-SCORE-TRANSFER-1 leg A).
//
// Exact Go port of the reference tools' parser in
// coord/runs/YUE-INSTRUMENTAL-PATH-1/scripts-score-transfer-20261120/abc_tools.py
// (SHA 2c146608...; gold-pair original.abc -> instrumental.abc pinned by
// yue_transfer_test.go). The dialect is the limited two-voice form the YuE2
// engine consumes: dual Vocal/Ins voices, in-bar quoted chord annotations,
// % section comments, L:1/2^k unit grid, Q:1/4=<bpm> tempo.
//
// Like the Python original this parser fails closed on unsupported tokens
// and structural invariants — it resolves sounding notes, not token counts.
// All times are exact fractions of a quarter note (yq below), never floats.

// yueDurations are the supported ABC unit lengths per token (abc_tools.py
// DURATIONS).
var yueDurations = map[int]bool{1: true, 2: true, 3: true, 4: true, 6: true, 8: true, 12: true, 16: true, 24: true, 32: true, 48: true}

// yueDurationsDesc is the greedy decomposition order for unit lengths
// (compile_score.py lengths()).
var yueDurationsDesc = []int{48, 32, 24, 16, 12, 8, 6, 4, 3, 2, 1}

// yueChordQualities is the supported chord-symbol quality list in the exact
// regex alternation order of abc_tools.py CHORD (backtracking order matters:
// "m7b5" must be tried after "m7").
var yueChordQualities = []string{"", "m", "dim", "aug", "7", "maj7", "m7", "dim7", "m7b5",
	"sus4", "sus2", "6", "m6", "7sus4", "m(maj7)"}

// yueKeys maps standard major/minor K: spellings to sharp counts -7..7
// (abc_tools.py KEYS).
var yueKeys = map[string]int{
	"Cb": -7, "Gb": -6, "Db": -5, "Ab": -4, "Eb": -3, "Bb": -2, "F": -1,
	"C": 0, "G": 1, "D": 2, "A": 3, "E": 4, "B": 5, "F#": 6, "C#": 7,
	"Abm": -7, "Ebm": -6, "Bbm": -5, "Fm": -4, "Cm": -3, "Gm": -2, "Dm": -1,
	"Am": 0, "Em": 1, "Bm": 2, "F#m": 3, "C#m": 4, "G#m": 5, "D#m": 6, "A#m": 7,
}

// yueSections is the supported % section label set (compile_score.py
// SECTIONS).
var yueSections = map[string]bool{
	"intro": true, "verse": true, "pre-chorus": true, "chorus": true,
	"bridge": true, "interlude": true, "outro": true, "instrumental": true,
}

// yq is an exact rational (normalized: d>0, gcd(|n|,d)=1). It mirrors
// fractions.Fraction for the reference tools' quarter-note time arithmetic.
type yq struct {
	n, d int64
}

func yqGCD(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func yqNorm(n, d int64) yq {
	if d == 0 {
		panic("yq: zero denominator")
	}
	if d < 0 {
		n, d = -n, -d
	}
	if n == 0 {
		return yq{0, 1}
	}
	g := yqGCD(n, d)
	if g > 1 {
		n, d = n/g, d/g
	}
	return yq{n, d}
}

func yqInt(v int64) yq { return yq{v, 1} }

func yqAdd(a, b yq) yq { return yqNorm(a.n*b.d+b.n*a.d, a.d*b.d) }
func yqSub(a, b yq) yq { return yqNorm(a.n*b.d-b.n*a.d, a.d*b.d) }
func yqMul(a, b yq) yq { return yqNorm(a.n*b.n, a.d*b.d) }
func yqDiv(a, b yq) yq { return yqNorm(a.n*b.d, a.d*b.n) }

func yqCmp(a, b yq) int  { return cmpInt64(a.n*b.d, b.n*a.d) }
func yqLess(a, b yq) bool { return a.n*b.d < b.n*a.d }
func yqEq(a, b yq) bool   { return a.n == b.n && a.d == b.d }

// yqString renders Python str(Fraction): "3" or "3/4" (normalized form).
func (f yq) String() string {
	if f.d == 1 {
		return strconv.FormatInt(f.n, 10)
	}
	return strconv.FormatInt(f.n, 10) + "/" + strconv.FormatInt(f.d, 10)
}

// yqFloat converts exactly like Python float(Fraction): one correctly
// rounded division of the exact integers.
func (f yq) Float() float64 { return float64(f.n) / float64(f.d) }

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// divFloor mirrors Python floor division (base // 12 with negative base).
func divFloor(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// modFloor mirrors Python modulo semantics (result has the divisor's sign).
func modFloor(a, b int64) int64 {
	m := a % b
	if m != 0 && ((m < 0) != (b < 0)) {
		m += b
	}
	return m
}

// yueVoiceMeter is one parsed M: meter.
type yueVoiceMeter struct{ N, D int }

// yueNativeNote is one sounding note: exact onset (quarters), MIDI pitch and
// exact duration (quarters), ties already merged.
type yueNativeNote struct {
	Start yq
	Pitch int
	Dur   yq
}

// yueNativeBar is one parsed measure: absolute start, length and meter.
type yueNativeBar struct {
	Start yq
	Len   yq
	Meter yueVoiceMeter
}

// yueNativePair is one (time, symbol) event (chords and key changes).
type yueNativePair struct {
	T      yq
	Symbol string
}

// yueNativeVoice is one parsed voice (abc_tools.py Voice).
type yueNativeVoice struct {
	Meter  yueVoiceMeter
	Key    string
	Time   yq
	Notes  []yueNativeNote
	Bars   []yueNativeBar
	Chords []yueNativePair
	Keys   []yueNativePair
	// tie continuation state: nil when no tie is open.
	tiePitch   int
	tieWritten int
	tieOpen    bool
}

// yueNativeScore is one parsed two-voice score (abc_tools.py Score).
type yueNativeScore struct {
	Text       string
	Unit       yq
	BPM        int
	Voices     map[string]*yueNativeVoice
	MusicLines map[int]string
}

// yueSplitLines mirrors text.splitlines() for \n/\r\n text: the trailing
// empty element after a final newline is dropped.
func yueSplitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// yueKeyAccidentals returns the letter->alteration map for a standard major
// or minor key (abc_tools.py key_accidentals). Missing keys fail closed.
func yueKeyAccidentals(key string) (map[byte]int, error) {
	count, ok := yueKeys[key]
	if !ok {
		return nil, fmt.Errorf("Unsupported key %q; use a standard major or minor K: field", key)
	}
	result := map[byte]int{}
	for letter := range yueNaturalPC {
		result[letter] = 0
	}
	order := "FCGDAEB"
	alter := 1
	if count < 0 {
		order = "BEADGCF"
		alter = -1
	}
	for i := 0; i < absInt(count); i++ {
		result[order[i]] = alter
	}
	return result, nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// yueMeterValue parses an explicit fraction meter (abc_tools.py
// meter_value): denominator a power of two <= 1024.
func yueMeterValue(text string) (yueVoiceMeter, error) {
	m := yueMeterRE.FindStringSubmatch(text)
	if m == nil {
		return yueVoiceMeter{}, fmt.Errorf("Unsupported meter %q; write an explicit fraction", text)
	}
	n, _ := strconv.Atoi(m[1])
	d, _ := strconv.Atoi(m[2])
	if d > 1024 || d&(d-1) != 0 {
		return yueVoiceMeter{}, fmt.Errorf("Unsupported meter denominator %d", d)
	}
	return yueVoiceMeter{N: n, D: d}, nil
}

// matchYueChordSymbol reports whether s is a fully supported chord symbol:
// root + optional accidental + quality + optional /bass (abc_tools.py CHORD
// fullmatch with its exact backtracking order).
func matchYueChordSymbol(s string) bool {
	if len(s) == 0 || s[0] < 'A' || s[0] > 'G' {
		return false
	}
	rest := s[1:]
	for _, acc := range []string{"bb", "##", "b", "#", ""} {
		if !strings.HasPrefix(rest, acc) {
			continue
		}
		if matchYueChordQualityTail(rest[len(acc):]) {
			return true
		}
	}
	return false
}

func matchYueChordQualityTail(s string) bool {
	for _, q := range yueChordQualities {
		if !strings.HasPrefix(s, q) {
			continue
		}
		tail := s[len(q):]
		if tail == "" {
			return true
		}
		if strings.HasPrefix(tail, "/") && matchYueChordBass(tail[1:]) {
			return true
		}
	}
	return false
}

func matchYueChordBass(s string) bool {
	if len(s) == 0 || s[0] < 'A' || s[0] > 'G' {
		return false
	}
	rest := s[1:]
	for _, acc := range []string{"bb", "##", "b", "#", ""} {
		if len(rest) == len(acc) && strings.HasPrefix(rest, acc) {
			return true
		}
	}
	return false
}

// yueToken is one matched music-line token (abc_tools.py TOKEN).
type yueToken struct {
	chord    string // quoted chord annotation (may be empty string)
	hasChord bool
	key      string // inline [K:...] value
	hasKey   bool
	acc      string // "", "^", "^^", "_", "__", "="
	note     byte   // A-G a-g z
	hasNote  bool
	octave   string // ',' and "'" marks
	digits   string // duration digits ("" = 1)
	tie      bool
	end      int // index after the token
}

// matchYueToken mirrors the TOKEN regex alternation anchored at i: quoted
// chord, then inline [K:], then accidental+note+octaves+digits+tie.
func matchYueToken(s string, i int) (yueToken, bool) {
	if s[i] == '"' {
		j := strings.IndexByte(s[i+1:], '"')
		if j < 0 {
			return yueToken{}, false
		}
		return yueToken{chord: s[i+1 : i+1+j], hasChord: true, end: i + j + 2}, true
	}
	if strings.HasPrefix(s[i:], "[K:") {
		j := strings.IndexByte(s[i+3:], ']')
		if j < 0 || j == 0 {
			return yueToken{}, false
		}
		return yueToken{key: s[i+3 : i+3+j], hasKey: true, end: i + j + 4}, true
	}
	t := yueToken{}
	j := i
	switch {
	case strings.HasPrefix(s[j:], "^^"):
		t.acc, j = "^^", j+2
	case strings.HasPrefix(s[j:], "__"):
		t.acc, j = "__", j+2
	case strings.HasPrefix(s[j:], "^"):
		t.acc, j = "^", j+1
	case strings.HasPrefix(s[j:], "_"):
		t.acc, j = "_", j+1
	case strings.HasPrefix(s[j:], "="):
		t.acc, j = "=", j+1
	}
	if j >= len(s) {
		return yueToken{}, false
	}
	c := s[j]
	if !(c >= 'A' && c <= 'G') && !(c >= 'a' && c <= 'g') && c != 'z' {
		return yueToken{}, false
	}
	t.note, t.hasNote, j = c, true, j+1
	for j < len(s) && (s[j] == ',' || s[j] == '\'') {
		j++
	}
	t.octave = s[i+len(t.acc)+1 : j]
	k := j
	for k < len(s) && s[k] >= '0' && s[k] <= '9' {
		k++
	}
	t.digits = s[j:k]
	j = k
	if j < len(s) && s[j] == '-' {
		t.tie, j = true, j+1
	}
	t.end = j
	return t, true
}

var yueZRestCount = func(s string) (int, bool) {
	// fullmatch Z([2-4])?
	if len(s) == 0 || s[0] != 'Z' {
		return 0, false
	}
	if len(s) == 1 {
		return 1, true
	}
	if len(s) == 2 && s[1] >= '2' && s[1] <= '4' {
		return int(s[1] - '0'), true
	}
	return 0, false
}

// yueParseNativeScore parses the native two-voice dialect score (port of
// abc_tools.py parse). Fails closed on unsupported notation.
func yueParseNativeScore(text string) (*yueNativeScore, error) {
	fail := func(cond bool, msg string) error {
		if cond {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
	lines := yueSplitLines(text)
	if err := fail(len(lines) < 12, "Incomplete native two-voice ABC"); err != nil {
		return nil, err
	}
	if err := fail(len(lines) < 2 || lines[0] != "X:1" || lines[1] != "T:", "Expected native X:1 and blank T: header"); err != nil {
		return nil, err
	}
	if err := fail(!strings.HasPrefix(lines[2], "M:"), "Missing header M:"); err != nil {
		return nil, err
	}
	meter, err := yueMeterValue(lines[2][2:])
	if err != nil {
		return nil, err
	}
	unit := yq{}
	if m := yueL1DenRE.FindStringSubmatch(lines[3]); m != nil {
		den, _ := strconv.Atoi(m[1])
		if den > 1024 || den&(den-1) != 0 {
			return nil, fmt.Errorf("Unsupported L: denominator")
		}
		unit = yqNorm(1, int64(den))
	} else {
		return nil, fmt.Errorf("Expected L:1/<power of two>, usually L:1/32")
	}
	bpm := 0
	if m := yueQTempoRE.FindStringSubmatch(lines[4]); m != nil {
		bpm, _ = strconv.Atoi(m[1])
	} else {
		return nil, fmt.Errorf("Expected integer quarter-note tempo Q:1/4=<BPM>")
	}
	expectedVoices := []string{
		`V: Vocal clef=treble name="Vocal Melody" snm="Vocal"`,
		`V: Ins clef=treble name="Ins Melody" snm="Inst."`,
	}
	for k, want := range expectedVoices {
		if err := fail(lines[5+k] != want, "Preserve native Vocal and Ins voice definitions"); err != nil {
			return nil, err
		}
	}
	if err := fail(!strings.HasPrefix(lines[7], "K:"), "Missing header K:"); err != nil {
		return nil, err
	}
	headerKey := lines[7][2:]
	if _, kerr := yueKeyAccidentals(headerKey); kerr != nil {
		return nil, kerr
	}
	voices := map[string]*yueNativeVoice{}
	for _, name := range []string{"Vocal", "Ins"} {
		voices[name] = &yueNativeVoice{Meter: meter, Key: headerKey, Time: yqInt(0), Keys: []yueNativePair{{T: yqInt(0), Symbol: headerKey}}}
	}
	musicLines := map[int]string{}
	cursor := 8
	group := 0
	for cursor < len(lines) {
		for cursor < len(lines) && strings.HasPrefix(lines[cursor], "% ") {
			cursor++
		}
		if err := fail(cursor == len(lines), "Dangling section comment without music"); err != nil {
			return nil, err
		}
		group++
		var counts []int
		for _, name := range []string{"Vocal", "Ins"} {
			context := fmt.Sprintf("group %d, %s", group, name)
			if err := fail(cursor >= len(lines) || lines[cursor] != "V: "+name, context+": expected V: "+name); err != nil {
				return nil, err
			}
			cursor++
			voice := voices[name]
			fields := map[string]bool{}
			for cursor < len(lines) && (strings.HasPrefix(lines[cursor], "M:") || strings.HasPrefix(lines[cursor], "K:")) {
				idx := strings.Index(lines[cursor], ":")
				fieldName, value := lines[cursor][:idx], lines[cursor][idx+1:]
				if err := fail(fields[fieldName], context+": duplicate "+fieldName+": field"); err != nil {
					return nil, err
				}
				fields[fieldName] = true
				if fieldName == "M" {
					m, merr := yueMeterValue(value)
					if merr != nil {
						return nil, fmt.Errorf("%s: %v", context, merr)
					}
					voice.Meter = m
				} else {
					if _, kerr := yueKeyAccidentals(value); kerr != nil {
						return nil, fmt.Errorf("%s: %v", context, kerr)
					}
					voice.Key = value
					voice.Keys = append(voice.Keys, yueNativePair{T: voice.Time, Symbol: value})
				}
				cursor++
			}
			if err := fail(cursor >= len(lines), context+": missing music line"); err != nil {
				return nil, err
			}
			line := lines[cursor]
			if err := fail(!strings.HasSuffix(line, "|"), context+": music line must end with a plain barline"); err != nil {
				return nil, err
			}
			musicLines[cursor] = name
			cursor++
			var bars []string
			for _, bar := range strings.Split(line[:len(line)-1], "|") {
				bar = strings.TrimSpace(bar)
				if err := fail(bar == "", context+": empty measure or unsupported double/repeat barline"); err != nil {
					return nil, err
				}
				if n, ok := yueZRestCount(bar); ok {
					for i := 0; i < n; i++ {
						bars = append(bars, "Z")
					}
				} else {
					bars = append(bars, bar)
				}
			}
			if err := fail(len(bars) < 1 || len(bars) > 4, context+": expected 1–4 measures after expanding Z rests"); err != nil {
				return nil, err
			}
			counts = append(counts, len(bars))
			for _, bar := range bars {
				barContext := fmt.Sprintf("%s, bar %d", context, len(voice.Bars)+1)
				if perr := yueParseBar(bar, voice, unit, barContext); perr != nil {
					return nil, perr
				}
			}
		}
		if err := fail(counts[0] != counts[1], fmt.Sprintf("group %d: voices have different measure counts", group)); err != nil {
			return nil, err
		}
	}
	for _, name := range []string{"Vocal", "Ins"} {
		if err := fail(voices[name].tieOpen, name+": unresolved tie at end of score"); err != nil {
			return nil, err
		}
	}
	if err := fail(len(voices["Ins"].Chords) != 0, "Native chord symbols belong in Vocal, not Ins"); err != nil {
		return nil, err
	}
	if err := fail(!yueBarsEqual(voices["Vocal"].Bars, voices["Ins"].Bars), "Voice meter/time grids differ"); err != nil {
		return nil, err
	}
	if err := fail(!yuePairsEqual(voices["Vocal"].Keys, voices["Ins"].Keys), "Voice key-change timelines differ"); err != nil {
		return nil, err
	}
	return &yueNativeScore{Text: text, Unit: unit, BPM: bpm, Voices: voices, MusicLines: musicLines}, nil
}

var (
	yueL1DenRE  = regexp.MustCompile(`^L:1/([1-9][0-9]*)$`)
	yueQTempoRE = regexp.MustCompile(`^Q:1/4=([1-9][0-9]*)$`)
	yueMeterRE  = regexp.MustCompile(`^([1-9][0-9]*)/([1-9][0-9]*)$`)
)

// yueParseBar parses one measure body into the voice state (port of
// abc_tools.py parse_bar).
func yueParseBar(body string, voice *yueNativeVoice, unit yq, context string) error {
	failf := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", context, fmt.Sprintf(format, args...))
	}
	n, d := voice.Meter.N, voice.Meter.D
	length := yqNorm(int64(4*n), int64(d))
	start := voice.Time
	offset := yqInt(0)
	local := map[byte]int{} // accidentals propagate by letter, across octaves
	if body == "Z" {
		if voice.tieOpen {
			return failf("tie enters a full-measure rest")
		}
		offset = length
	} else {
		i := 0
		for i < len(body) {
			if body[i] == ' ' || body[i] == '\t' {
				i++
				continue
			}
			tok, ok := matchYueToken(body, i)
			if !ok {
				return failf("unsupported token at %q", body[i:minInt(i+24, len(body))])
			}
			i = tok.end
			if yqCmp(offset, length) >= 0 {
				return failf("event after the measure end")
			}
			if tok.hasChord {
				if !matchYueChordSymbol(tok.chord) {
					return failf("unsupported chord %q", tok.chord)
				}
				voice.Chords = append(voice.Chords, yueNativePair{T: yqAdd(start, offset), Symbol: tok.chord})
				continue
			}
			if tok.hasKey {
				if _, kerr := yueKeyAccidentals(tok.key); kerr != nil {
					return kerr
				}
				voice.Key = tok.key
				voice.Keys = append(voice.Keys, yueNativePair{T: yqAdd(start, offset), Symbol: tok.key})
				local = map[byte]int{}
				continue
			}
			units := 1
			if tok.digits != "" {
				v, err := strconv.Atoi(tok.digits)
				if err != nil {
					return failf("unsupported duration %q", tok.digits)
				}
				units = v
			}
			if !yueDurations[units] {
				return failf("unsupported duration %d; split it into tied supported lengths", units)
			}
			dur := yqMul(yqInt(int64(units)), yqMul(unit, yqInt(4)))
			if yqLess(length, yqAdd(offset, dur)) {
				return failf("note/rest exceeds meter duration")
			}
			if strings.Contains(tok.octave, ",") && strings.Contains(tok.octave, "'") {
				return failf("mixed octave marks")
			}
			if tok.note == 'z' {
				if tok.acc != "" || tok.octave != "" || tok.tie {
					return failf("a rest cannot have accidentals, octave marks or ties")
				}
				if voice.tieOpen {
					return failf("tie enters a rest")
				}
			} else {
				isLower := tok.note >= 'a' && tok.note <= 'g'
				letter := upperByte(tok.note)
				written := 60 + yueNaturalPC[letter]
				if isLower {
					written += 12
				}
				written += 12 * (strings.Count(tok.octave, "'") - strings.Count(tok.octave, ","))
				sig, kerr := yueKeyAccidentals(voice.Key)
				if kerr != nil {
					return kerr
				}
				alteration, hasLocal := local[letter]
				if !hasLocal {
					alteration = sig[letter]
				}
				if tok.acc != "" {
					switch tok.acc {
					case "=":
						alteration = 0
					case "_":
						alteration = -1
					case "__":
						alteration = -2
					case "^":
						alteration = 1
					case "^^":
						alteration = 2
					}
					local[letter] = alteration
				}
				pitch := written + alteration
				if voice.tieOpen {
					if tok.acc == "" && written == voice.tieWritten {
						pitch = voice.tiePitch
					}
					if pitch != voice.tiePitch {
						return failf("tie changes pitch from %d to %d", voice.tiePitch, pitch)
					}
					last := &voice.Notes[len(voice.Notes)-1]
					last.Dur = yqAdd(last.Dur, dur)
				} else {
					if pitch < 0 || pitch > 127 {
						return failf("pitch %d is outside MIDI range", pitch)
					}
					voice.Notes = append(voice.Notes, yueNativeNote{Start: yqAdd(start, offset), Pitch: pitch, Dur: dur})
				}
				if tok.tie {
					voice.tiePitch, voice.tieWritten, voice.tieOpen = pitch, written, true
				} else {
					voice.tieOpen = false
				}
			}
			offset = yqAdd(offset, dur)
		}
	}
	if !yqEq(offset, length) {
		return failf("duration %s quarter notes != meter duration %s", offset.String(), length.String())
	}
	voice.Bars = append(voice.Bars, yueNativeBar{Start: start, Len: length, Meter: voice.Meter})
	voice.Time = yqAdd(voice.Time, length)
	return nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func yueBarsEqual(a, b []yueNativeBar) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !yqEq(a[i].Start, b[i].Start) || !yqEq(a[i].Len, b[i].Len) || a[i].Meter != b[i].Meter {
			return false
		}
	}
	return true
}

func yuePairsEqual(a, b []yueNativePair) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !yqEq(a[i].T, b[i].T) || a[i].Symbol != b[i].Symbol {
			return false
		}
	}
	return true
}

func yueNotesEqual(a, b []yueNativeNote) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !yqEq(a[i].Start, b[i].Start) || a[i].Pitch != b[i].Pitch || !yqEq(a[i].Dur, b[i].Dur) {
			return false
		}
	}
	return true
}
