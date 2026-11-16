package genapi

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// YuE ABC dialect score conversion (GEN-YUE-WIRE-1, audit H6 item 1).
//
// The yue_local dialect's primary input face is the native YuE2 ABC dialect
// score (dual voices + in-bar quoted chords + % section comments — see the
// stimuli of coord/runs/YUE2-ABC-ADHERENCE-1). Naive ABC loses the melody
// (corr 0.16-0.20 vs dialect 0.99), so a student melody must be converted by
// a deterministic program, never paraphrased through an LLM.
//
// This file implements exactly that: kernel MIDI clip notes (pitch +
// quarter-note beats) -> YuE2 ABC dialect score, and a parser that reads the
// melody voice back out of such a score so round-trip equality can be
// unit-tested. Known boundaries, disclosed rather than hidden:
//   - Chord annotations are optional input (YueScoreInput.Chords); without
//     them the converter emits bare bars and the acoustic effect of a bare
//     dialect line is a real-stack question (smoke leg), not a promise here.
//   - Note edges are quantized to the 1/32 grid (L:1/32, 8 units per
//     quarter note). Edges off the grid move to the nearest unit; the
//     round-trip test pins the tolerance.

// yueUnitPerQuarter is the number of L:1/32 units per quarter note.
const yueUnitPerQuarter = 8

// YueNote is one melodic note of the student score, matching the kernel's
// get_midi_clip_notes truth (pitch 0-127; start/length in quarter-note
// beats, clip-relative).
type YueNote struct {
	Pitch  int     `json:"pitch"`
	Start  float64 `json:"start"`
	Length float64 `json:"length"`
}

// YueChord annotates one bar with an in-bar quoted chord label.
type YueChord struct {
	Bar  int    `json:"bar"`  // 0-based bar index within the score
	Name string `json:"name"` // e.g. "Dm7"; empty entries are skipped
}

// YueSection splits the score into % section comments.
type YueSection struct {
	Name string `json:"name"` // e.g. "intro", "verse"
	Bars int    `json:"bars"` // bars in this section (>0)
}

// YueScoreInput is everything the deterministic conversion needs.
type YueScoreInput struct {
	Notes       []YueNote     `json:"notes"`
	Numerator   int           `json:"numerator"`   // time signature numerator (M: line), default 4
	Denominator int           `json:"denominator"` // time signature denominator, default 4
	TempoBPM    float64       `json:"tempo_bpm"`   // Q:1/4=<bpm>; 0 = omit the Q: line
	Key         string        `json:"key"`         // K: line value, e.g. "C", "G", "Am"; default "C"
	Title       string        `json:"title"`
	Sections    []YueSection  `json:"sections,omitempty"`
	Chords      []YueChord    `json:"chords,omitempty"`
	MelodyVoice string        `json:"melody_voice,omitempty"` // "Ins" (default) or "Vocal"
	Flats       bool          `json:"flats,omitempty"`        // spell chromatic notes with flats (flat keys)
}

// ConvertMidiToYueABC renders the student score as a YuE2 ABC dialect score.
// The melody rides the MelodyVoice (Ins by default — the student melody is
// the instrumental lead); the other voice rests for the full length, which
// is the request-side instrumental conditioning (Vocal part rested, audit
// H6 item 3). The YuE2 engine itself has no instrumental switch: a rested
// Vocal score does not guarantee a vocal-free render (YUE-VOCAL-FORENSICS-1
// forensics, run_20261116_000009).
func ConvertMidiToYueABC(in YueScoreInput) (string, error) {
	num, den := in.Numerator, in.Denominator
	if num <= 0 {
		num = 4
	}
	if den <= 0 || den == 0 {
		den = 4
	}
	unitsPerBar := num * 32 / den
	if unitsPerBar <= 0 {
		return "", fmt.Errorf("yue abc: time signature %d/%d has no 1/32 representation", num, den)
	}
	if len(in.Notes) == 0 {
		return "", fmt.Errorf("yue abc: score has no notes")
	}
	for i, n := range in.Notes {
		if n.Pitch < 0 || n.Pitch > 127 {
			return "", fmt.Errorf("yue abc: notes[%d] pitch %d outside 0-127", i, n.Pitch)
		}
		if n.Start < 0 || n.Length <= 0 {
			return "", fmt.Errorf("yue abc: notes[%d] needs start>=0 and length>0 (start=%v length=%v)", i, n.Start, n.Length)
		}
	}

	// Quantize to the 1/32 grid and split across bars.
	type seg struct {
		bar      int
		startU   int // unit offset inside the bar
		lengthU  int
		pitch    int
		tieStart bool // continuation of a note split across a bar line
		tieEnd   bool // continues into the next bar
	}
	var segs []seg
	maxUnit := 0
	for _, n := range in.Notes {
		s := int(math.Round(n.Start * yueUnitPerQuarter))
		e := int(math.Round((n.Start + n.Length) * yueUnitPerQuarter))
		if e <= s {
			e = s + 1
		}
		if e > maxUnit {
			maxUnit = e
		}
		for b := s / unitsPerBar; b < (e-1)/unitsPerBar+1; b++ {
			bs, be := b*unitsPerBar, (b+1)*unitsPerBar
			ls, le := s, e
			if ls < bs {
				ls = bs
			}
			if le > be {
				le = be
			}
			if le <= ls {
				continue
			}
			segs = append(segs, seg{bar: b, startU: ls - bs, lengthU: le - ls, pitch: n.Pitch, tieStart: ls > s, tieEnd: le < e})
		}
	}
	totalBars := (maxUnit + unitsPerBar - 1) / unitsPerBar
	if totalBars <= 0 {
		return "", fmt.Errorf("yue abc: score resolves to zero bars")
	}

	// Bar -> segments (sorted by start).
	byBar := make([][]seg, totalBars)
	for _, s := range segs {
		byBar[s.bar] = append(byBar[s.bar], s)
	}
	for _, list := range byBar {
		sort.Slice(list, func(i, j int) bool { return list[i].startU < list[j].startU })
	}
	chordAt := map[int]string{}
	for _, c := range in.Chords {
		if c.Bar >= 0 && c.Bar < totalBars && strings.TrimSpace(c.Name) != "" {
			chordAt[c.Bar] = strings.TrimSpace(c.Name)
		}
	}

	// Section layout: explicit sections when provided (bar counts validated),
	// otherwise one implicit block covering all bars without % comments.
	type block struct {
		name   string
		named  bool
		first  int
		bars   int
	}
	var blocks []block
	if len(in.Sections) > 0 {
		pos := 0
		for i, s := range in.Sections {
			name := strings.TrimSpace(s.Name)
			if name == "" {
				name = fmt.Sprintf("section%d", i+1)
			}
			if s.Bars <= 0 {
				return "", fmt.Errorf("yue abc: section %q needs bars>0", name)
			}
			blocks = append(blocks, block{name: name, named: true, first: pos, bars: s.Bars})
			pos += s.Bars
		}
		if pos < totalBars {
			blocks = append(blocks, block{first: pos, bars: totalBars - pos})
		}
		if pos > totalBars {
			// Section layout longer than the notes: extend with rest bars so
			// the structure the caller asked for is honored deterministically.
			totalBars = pos
			byBar = append(byBar, make([][]seg, pos-len(byBar))...)
		}
	} else {
		blocks = []block{{first: 0, bars: totalBars}}
	}

	melodyVoice := strings.TrimSpace(in.MelodyVoice)
	if melodyVoice == "" {
		melodyVoice = "Ins"
	}
	if melodyVoice != "Ins" && melodyVoice != "Vocal" {
		return "", fmt.Errorf("yue abc: melody_voice must be Ins or Vocal, got %q", melodyVoice)
	}
	restVoice := "Vocal"
	if melodyVoice == "Vocal" {
		restVoice = "Ins"
	}
	key := strings.TrimSpace(in.Key)
	if key == "" {
		key = "C"
	}

	var b strings.Builder
	b.WriteString("X:1\n")
	if t := strings.TrimSpace(in.Title); t != "" {
		b.WriteString("T:" + t + "\n")
	} else {
		b.WriteString("T:\n")
	}
	fmt.Fprintf(&b, "M:%d/%d\n", num, den)
	b.WriteString("L:1/32\n")
	if in.TempoBPM > 0 {
		fmt.Fprintf(&b, "Q:1/4=%s\n", strconv.FormatFloat(in.TempoBPM, 'f', -1, 64))
	}
	b.WriteString("V: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n")
	b.WriteString("V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\n")
	b.WriteString("K:" + key + "\n")

	for _, blk := range blocks {
		if blk.named {
			b.WriteString("% " + blk.name + "\n")
		}
		// Lines of at most 4 bars inside the block.
		for line := blk.first; line < blk.first+blk.bars; line += 4 {
			lineEnd := line + 4
			if lineEnd > blk.first+blk.bars {
				lineEnd = blk.first+blk.bars
			}
			// Rest voice line: the whole line is silent — one Z<n>| covering
			// the line's bars (the dialect's multi-bar rest form).
			b.WriteString("V: " + restVoice + "\n")
			fmt.Fprintf(&b, "Z%d|\n", lineEnd-line)
			// Melody voice line.
			b.WriteString("V: " + melodyVoice + "\n")
			for bar := line; bar < lineEnd; bar++ {
				if ch, ok := chordAt[bar]; ok {
					b.WriteString("\"" + ch + "\"")
				}
				if len(byBar[bar]) == 0 {
					b.WriteString("Z|")
					continue
				}
				cursor := 0
				tiedOver := false
				for _, s := range byBar[bar] {
					if s.startU > cursor {
						fmt.Fprintf(&b, "z%d", s.startU-cursor)
					}
					cursor = s.startU + s.lengthU
					tie := ""
					if s.tieEnd {
						tie = "-"
						tiedOver = true
					}
					fmt.Fprintf(&b, "%s%d%s", YuePitchSpelling(s.pitch, in.Flats), s.lengthU, tie)
				}
				if cursor < unitsPerBar && !tiedOver {
					// No trailing rest when the last segment ties over the
					// bar line — the note itself fills the gap.
					fmt.Fprintf(&b, "z%d", unitsPerBar-cursor)
				}
				b.WriteString("|")
			}
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

var yueNaturalPC = map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}

// YuePitchSpelling renders a MIDI pitch as an ABC note token relative to
// middle C (c = MIDI 60). Chromatic notes default to sharps; flat keys pass
// flats=true to spell them as flats.
func YuePitchSpelling(midi int, flats bool) string {
	if midi < 0 || midi > 127 {
		return ""
	}
	d := midi - 60
	oct := 0
	if d < 0 {
		oct = -(((-d) + 11) / 12)
	} else {
		oct = d / 12
	}
	step := d - oct*12 // 0..11
	var letter byte
	accidental := ""
	switch step {
	case 0:
		letter = 'C'
	case 1:
		if flats {
			letter, accidental = 'D', "_"
		} else {
			letter, accidental = 'C', "^"
		}
	case 2:
		letter = 'D'
	case 3:
		if flats {
			letter, accidental = 'E', "_"
		} else {
			letter, accidental = 'D', "^"
		}
	case 4:
		letter = 'E'
	case 5:
		letter = 'F'
	case 6:
		if flats {
			letter, accidental = 'G', "_"
		} else {
			letter, accidental = 'F', "^"
		}
	case 7:
		letter = 'G'
	case 8:
		if flats {
			letter, accidental = 'A', "_"
		} else {
			letter, accidental = 'G', "^"
		}
	case 9:
		letter = 'A'
	case 10:
		if flats {
			letter, accidental = 'B', "_"
		} else {
			letter, accidental = 'A', "^"
		}
	case 11:
		letter = 'B'
	}
	out := accidental
	if oct >= 0 {
		out += strings.ToLower(string(letter)) + strings.Repeat("'", oct)
	} else {
		out += strings.ToUpper(string(letter)) + strings.Repeat(",", -oct-1)
	}
	return out
}

// yueSpelledNote is one parsed melody note (already merged across ties).
type yueSpelledNote struct {
	Pitch  int
	Start  float64 // quarter-note beats from the score start
	Length float64
}

// yueVoiceState is the per-voice parse cursor. Voices in the dialect score
// run in parallel: every voice's music lines advance that voice's own bar
// cursor, independent of the other voice's lines.
type yueVoiceState struct {
	notes   []yueSpelledNote
	barUnit int // unit cursor within the voice's current bar
	barIdx  int // 0-based bar cursor of this voice
	lastTie *yueSpelledNote
}

// ParseYueABC reads a YuE2 ABC dialect score and extracts one voice's notes
// as MIDI pitches on the quarter-note beat grid. Quoted chord annotations
// are skipped; whole-bar rests (Z<n>| with its closing bar line) and
// explicit rests (z) advance the cursor; ties merge; each voice keeps its
// own bar cursor. This is the read-back half of the round-trip contract and
// the substrate the instrumental Vocal-resting rewrite operates on.
func ParseYueABC(abc, voice string) ([]yueSpelledNote, error) {
	voice = strings.TrimSpace(voice)
	if voice == "" {
		voice = "Ins"
	}
	lines := strings.Split(strings.ReplaceAll(abc, "\r\n", "\n"), "\n")
	var (
		num, den      = 4, 4
		unitsPerBar   = 32
		voices        = map[string]*yueVoiceState{}
		currentVoice  = ""
	)
	state := func(v string) *yueVoiceState {
		s, ok := voices[v]
		if !ok {
			s = &yueVoiceState{}
			voices[v] = s
		}
		return s
	}
	for ln, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "w:") {
			continue
		}
		if strings.HasPrefix(line, "V:") {
			fields := strings.Fields(strings.TrimPrefix(line, "V:"))
			if len(fields) == 0 {
				return nil, fmt.Errorf("yue abc parse: line %d empty V:", ln+1)
			}
			currentVoice = fields[0]
			continue
		}
		if len(line) > 1 && line[1] == ':' {
			if line[0] == 'M' {
				parts := strings.SplitN(strings.TrimPrefix(line, "M:"), "/", 2)
				if len(parts) == 2 {
					if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
						num = n
					}
					if d, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && d > 0 {
						den = d
					}
					if num > 0 && den > 0 {
						unitsPerBar = num * 32 / den
					}
				}
			}
			continue
		}
		vs := state(currentVoice)
		i := 0
		zBarTail := false // the | right after Z<n> closes the multi-bar rest
		for i < len(line) {
			c := line[i]
			switch {
			case c == '"':
				j := strings.IndexByte(line[i+1:], '"')
				if j < 0 {
					return nil, fmt.Errorf("yue abc parse: line %d unterminated chord", ln+1)
				}
				i += j + 2
				zBarTail = false
			case c == '|':
				if !zBarTail {
					vs.barUnit = 0
					vs.barIdx++
				}
				zBarTail = false
				i++
			case c == 'Z':
				i++
				n := 1
				if digits, adv := readDigits(line, i); adv > 0 {
					n = digits
					i += adv
				}
				vs.barUnit = 0
				vs.barIdx += n
				zBarTail = true
			case c == 'z':
				i++
				n := 1
				if digits, adv := readDigits(line, i); adv > 0 {
					n = digits
					i += adv
				}
				vs.barUnit += n
				zBarTail = false
			case c == '^' || c == '_' || c == '=' ||
				(c >= 'a' && c <= 'g') || (c >= 'A' && c <= 'G'):
				acc := 0
				if c == '^' || c == '_' {
					sign := 1
					if c == '_' {
						sign = -1
					}
					acc = sign
					i++
					if i < len(line) && (line[i] == '^' || line[i] == '_') {
						acc *= 2
						i++
					}
				}
				if i >= len(line) || !isABCLetter(line[i]) {
					return nil, fmt.Errorf("yue abc parse: line %d accidental without note at %d", ln+1, i)
				}
				letter := line[i]
				i++
				octShift := 0
				for i < len(line) && line[i] == '\'' {
					octShift++
					i++
				}
				for i < len(line) && line[i] == ',' {
					octShift--
					i++
				}
				lengthU := 1
				if digits, adv := readDigits(line, i); adv > 0 {
					lengthU = digits
					i += adv
				}
				tie := false
				if i < len(line) && line[i] == '-' {
					tie = true
					i++
				}
				base, ok := yueNaturalPC[upperByte(letter)]
				if !ok {
					return nil, fmt.Errorf("yue abc parse: line %d unknown note letter %q", ln+1, letter)
				}
				midi := base + acc + 60
				if !(letter >= 'a' && letter <= 'g') {
					midi -= 12
				}
				midi += 12 * octShift
				if midi < 0 || midi > 127 {
					return nil, fmt.Errorf("yue abc parse: line %d pitch out of range (%d)", ln+1, midi)
				}
				startBeats := (float64(vs.barIdx)*float64(unitsPerBar) + float64(vs.barUnit)) / yueUnitPerQuarter
				lengthBeats := float64(lengthU) / yueUnitPerQuarter
				note := yueSpelledNote{Pitch: midi, Start: startBeats, Length: lengthBeats}
				if vs.lastTie != nil && vs.lastTie.Pitch == midi {
					vs.lastTie.Length += lengthBeats
					if !tie {
						vs.lastTie = nil
					}
					vs.barUnit += lengthU
					zBarTail = false
					continue
				}
				vs.notes = append(vs.notes, note)
				if tie {
					vs.lastTie = &vs.notes[len(vs.notes)-1]
				} else {
					vs.lastTie = nil
				}
				vs.barUnit += lengthU
				zBarTail = false
			case c == ' ' || c == '\t':
				i++
			default:
				return nil, fmt.Errorf("yue abc parse: line %d unexpected byte %q at %d", ln+1, c, i)
			}
		}
	}
	return state(voice).notes, nil
}

func readDigits(s string, i int) (n, adv int) {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	if j == i {
		return 0, 0
	}
	v, err := strconv.Atoi(s[i:j])
	if err != nil {
		return 0, 0
	}
	return v, j - i
}

func isABCLetter(c byte) bool {
	return (c >= 'a' && c <= 'g') || (c >= 'A' && c <= 'G')
}

func upperByte(c byte) byte {
	if c >= 'a' && c <= 'g' {
		return c - 32
	}
	return c
}

// RestYueVocalPart returns the score with the Vocal voice's sounding notes
// replaced by equal-length rests, preserving bar lines and in-bar chord
// annotations (the instrumental conditioning path: student melody rides Ins,
// Vocal rests — audit H6 item 3). Non-Vocal lines are untouched. The bool
// reports whether any sounding note was rested.
func RestYueVocalPart(abc string) (string, bool, error) {
	lines := strings.Split(strings.ReplaceAll(abc, "\r\n", "\n"), "\n")
	rested := false
	inVocal := false
	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "V:") {
			fields := strings.Fields(strings.TrimPrefix(line, "V:"))
			inVocal = len(fields) > 0 && fields[0] == "Vocal"
			continue
		}
		if !inVocal || len(line) < 2 || line[1] != ':' {
			if inVocal && !strings.HasPrefix(line, "%") {
				// music line for Vocal: rewrite note/rest runs per bar to z<sum>
				newLine, changed, err := restBarLine(line)
				if err != nil {
					return "", false, err
				}
				lines[idx] = newLine
				rested = rested || changed
			}
			continue
		}
	}
	// Re-join preserving the original line endings shape (simple \n form).
	return strings.Join(lines, "\n"), rested, nil
}

// restBarLine rewrites one Vocal music line: inside each bar (or multi-bar
// rest), every note/rest run becomes a single z<units> of equal total
// length. Chord annotations and bar lines stay verbatim, so the resting
// voice keeps the score's structure and the harmonic context while carrying
// no sounding notes.
func restBarLine(line string) (string, bool, error) {
	var out strings.Builder
	i := 0
	changed := false
	pendingChord := ""
	flushRest := func(units int) {
		if units <= 0 {
			return
		}
		out.WriteString(pendingChord)
		pendingChord = ""
		fmt.Fprintf(&out, "z%d", units)
	}
	for i < len(line) {
		c := line[i]
		switch {
		case c == '"':
			j := strings.IndexByte(line[i+1:], '"')
			if j < 0 {
				return "", false, fmt.Errorf("yue rest: unterminated chord in %q", line)
			}
			pendingChord = line[i : i+j+2]
			i += j + 2
		case c == '|':
			out.WriteByte(c)
			i++
		case c == 'Z':
			// Whole-bar rests are already silent; keep them verbatim (the
			// closing | after Z<n> belongs to the rest, not a new bar).
			out.WriteString(pendingChord)
			pendingChord = ""
			out.WriteByte(c)
			i++
			if digits, adv := readDigits(line, i); adv > 0 {
				fmt.Fprintf(&out, "%d", digits)
				i += adv
			}
			if i < len(line) && line[i] == '|' {
				out.WriteByte('|')
				i++
			}
		case c == ' ' || c == '\t':
			out.WriteByte(c)
			i++
		default:
			// A note/rest run inside this bar: sum every unit and emit one
			// rest of the same length.
			rest := 0
			for i < len(line) {
				n := line[i]
				if n == '|' || n == '"' || n == ' ' || n == '\t' || n == 'Z' {
					break
				}
				if n == 'z' {
					i++
					if digits, adv := readDigits(line, i); adv > 0 {
						rest += digits
						i += adv
					} else {
						rest++
					}
					continue
				}
				if n == '^' || n == '_' || n == '=' || isABCLetter(n) {
					u, adv, err := readNoteUnits(line, i)
					if err != nil {
						return "", false, err
					}
					rest += u
					changed = true
					i += adv
					continue
				}
				return "", false, fmt.Errorf("yue rest: unexpected byte %q in %q", n, line)
			}
			flushRest(rest)
		}
	}
	return out.String(), changed, nil
}

// readNoteUnits consumes one note token (accidental+letter+octaves+digits)
// and returns its 1/32 unit length.
func readNoteUnits(s string, i int) (units, adv int, err error) {
	j := i
	for j < len(s) && (s[j] == '^' || s[j] == '_' || s[j] == '=') {
		j++
	}
	if j >= len(s) || !isABCLetter(s[j]) {
		return 0, 0, fmt.Errorf("yue rest: accidental without note at %d in %q", j, s)
	}
	j++
	for j < len(s) && (s[j] == '\'' || s[j] == ',') {
		j++
	}
	length := 1
	if digits, adv := readDigits(s, j); adv > 0 {
		length = digits
		j += adv
	}
	if j < len(s) && s[j] == '-' {
		j++ // tie marker is timing-neutral at line scope
	}
	return length, j - i, nil
}

// YueSectionTagLyrics extracts the score's % section comments and renders
// them as the official instrumental lyric form (YuE repo
// skills/yue2-music/instrumental lyric_tags): one "[Title Case]" tag per
// section, blank-line separated, with a trailing newline. Bool=false when
// the score carries no section comments (the official equivalent there is
// empty lyrics). This is a lyric-tags channel only — no sung words are ever
// generated here.
func YueSectionTagLyrics(abc string) (string, bool) {
	var tags []string
	for _, raw := range strings.Split(strings.ReplaceAll(abc, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "% ") {
			tags = append(tags, "["+yueTitleCase(strings.TrimSpace(line[2:]))+"]")
		}
	}
	if len(tags) == 0 {
		return "", false
	}
	return strings.Join(tags, "\n\n") + "\n", true
}

// yueTitleCase mirrors Python str.title() for section labels: a letter that
// follows a non-letter is uppercased, every other letter is lowercased
// ("pre-chorus" -> "Pre-Chorus", "VERSE" -> "Verse").
func yueTitleCase(s string) string {
	prevLetter := false
	return strings.Map(func(r rune) rune {
		letter := unicode.IsLetter(r)
		out := r
		if letter && !prevLetter {
			out = unicode.ToUpper(r)
		} else if letter {
			out = unicode.ToLower(r)
		}
		prevLetter = letter
		return out
	}, s)
}

// YueVocalHasChords reports whether the score's Vocal voice carries in-bar
// quoted chord annotations. This is the official instrumental recipe's cot
// derivation (YuE repo skills/yue2-music/instrumental validate_request):
// chords on the resting Vocal voice select cot=full, a bare resting Vocal
// voice (our converter's form — chords ride the Ins melody) selects
// cot=melody.
func YueVocalHasChords(abc string) bool {
	inVocal := false
	for _, raw := range strings.Split(strings.ReplaceAll(abc, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "w:") {
			continue
		}
		if strings.HasPrefix(line, "V:") {
			fields := strings.Fields(strings.TrimPrefix(line, "V:"))
			inVocal = len(fields) > 0 && fields[0] == "Vocal"
			continue
		}
		if !inVocal || (len(line) > 1 && line[1] == ':') {
			continue // other voices and header fields carry no cot signal
		}
		if strings.Contains(line, "\"") {
			return true
		}
	}
	return false
}
