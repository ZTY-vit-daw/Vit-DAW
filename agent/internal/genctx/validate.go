package genctx

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

// scaleSteps is the agent-side semitone step table for the 11 tracktion
// ScaleType names (tracktion_Musicality.h:77-90 alignment; getSteps shape
// :125). Kept as a hard table on purpose — no kernel dependency, and the
// validation is deterministic set membership only (01 §2).
var scaleSteps = map[string][]int{
	"major":         {0, 2, 4, 5, 7, 9, 11},
	"minor":         {0, 2, 3, 5, 7, 8, 10},
	"ionian":        {0, 2, 4, 5, 7, 9, 11},
	"dorian":        {0, 2, 3, 5, 7, 9, 10},
	"phrygian":      {0, 1, 3, 5, 7, 8, 10},
	"lydian":        {0, 2, 4, 6, 7, 9, 11},
	"mixolydian":    {0, 2, 4, 5, 7, 9, 10},
	"aeolian":       {0, 2, 3, 5, 7, 8, 10},
	"locrian":       {0, 1, 3, 5, 6, 8, 10},
	"melodicMinor":  {0, 2, 3, 5, 7, 9, 11},
	"harmonicMinor": {0, 2, 3, 5, 7, 8, 11},
}

// ScaleSteps returns the semitone steps for a scale name (fail-closed on
// unknown names).
func ScaleSteps(scale string) ([]int, error) {
	steps, ok := scaleSteps[scale]
	if !ok {
		return nil, fmt.Errorf("genctx: unknown scale %q", scale)
	}
	out := make([]int, len(steps))
	copy(out, steps)
	return out, nil
}

// Scales lists the supported scale names.
func Scales() []string {
	out := make([]string, 0, len(scaleSteps))
	for k := range scaleSteps {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var pitchClassNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

// PitchClassName maps a MIDI pitch to its note name (C..B).
func PitchClassName(pitch int) string {
	if pitch < 0 {
		return ""
	}
	return pitchClassNames[pitch%12]
}

// PitchClassSet expands a key signature into its pitch-class set (01 §2
// deterministic definition: root pitch-class + scale step table).
func PitchClassSet(key *KeySignature) map[int]bool {
	if key == nil {
		return nil
	}
	steps := scaleSteps[key.Scale]
	root := ((key.RootPitch % 12) + 12) % 12
	set := make(map[int]bool, len(steps))
	for _, s := range steps {
		set[(root+s)%12] = true
	}
	return set
}

// PitchSetNames renders the pitch-class set as ascending note names (for the
// CONSTRAINTS MUST line).
func PitchSetNames(key *KeySignature) string {
	set := PitchClassSet(key)
	classes := make([]int, 0, len(set))
	for c := range set {
		classes = append(classes, c)
	}
	sort.Ints(classes)
	// rotate so the root names the set
	root := ((key.RootPitch % 12) + 12) % 12
	names := make([]string, 0, len(classes))
	for _, c := range classes {
		if c >= root {
			names = append(names, pitchClassNames[c])
		}
	}
	for _, c := range classes {
		if c < root {
			names = append(names, pitchClassNames[c])
		}
	}
	return strings.Join(names, ",")
}

// Violation is one machine-check failure (03 §3 form A row): any violation
// means the orchestrator must not dispatch the write command at all, and the
// violations land in the run receipt for failure classification (AGENTS §8).
type Violation struct {
	Kind      string `json:"kind"`
	Field     string `json:"field"`
	Expected  string `json:"expected"`
	Actual    string `json:"actual"`
	NoteIndex int    `json:"note_index"`
}

func (v Violation) String() string {
	return fmt.Sprintf("constraint_violation{%s: %s: expected %s, actual %s (note %d)}", v.Kind, v.Field, v.Expected, v.Actual, v.NoteIndex)
}

// Limits are the write-front bounds (03 §3 item ③ — the drum-gen card's
// "range / density / length caps reject with an error" semantics unified
// into this gate). Values are v1 defaults; consumer cards may tighten.
type Limits struct {
	MinPitch       int     // design range floor (inside the kernel's 0-127 edge)
	MaxPitch       int     // design range ceiling
	MaxNotes       int     // density cap per clip
	MaxLengthBeats float64 // per-note length cap
}

// DefaultLimits are the v1 default bounds.
var DefaultLimits = Limits{MinPitch: 24, MaxPitch: 96, MaxNotes: 512, MaxLengthBeats: 64}

// Grid resolution and tolerance (03 §3 form A row ②: 16th-note resolution,
// tolerance ±1/64 beat — constants in v1).
const (
	gridBeat      = 1.0 / 16.0
	gridTolerance = 1.0 / 64.0
	floatEpsilon  = 1e-9
)

// Note is the neutral note shape the orchestrator produces before the write
// command (times in clip-relative beats — MUST 1).
type Note struct {
	Pitch    int     `json:"pitch"`
	Start    float64 `json:"start"`
	Length   float64 `json:"length"`
	Velocity int     `json:"velocity"`
}

// ValidateNotes is the write-front gate for form-A products (generated note
// sequences). Returns all violations; empty = safe to dispatch the write.
//
// The kernel cannot fail-closed here (its MIDI write path clamps pitch /
// velocity via jlimit — MidiService 8 sites), so the gate must run before
// any write command is sent (03 §3 core rationale).
func ValidateNotes(snap Snapshot, notes []Note, limits Limits) []Violation {
	if limits == (Limits{}) {
		limits = DefaultLimits
	}
	var out []Violation
	add := func(kind, field, expected, actual string, idx int) {
		out = append(out, Violation{Kind: kind, Field: field, Expected: expected, Actual: actual, NoteIndex: idx})
	}
	keySet := PitchClassSet(snap.KeySignature)

	if len(notes) > limits.MaxNotes {
		add("density", "note_count", fmt.Sprintf("≤%d", limits.MaxNotes), fmt.Sprintf("%d", len(notes)), -1)
	}
	for i, n := range notes {
		// structural legality — reject before the kernel would clamp
		if n.Pitch < 0 || n.Pitch > 127 {
			add("pitch_range_kernel", "pitch", "0-127", fmt.Sprintf("%d", n.Pitch), i)
		} else if n.Pitch < limits.MinPitch || n.Pitch > limits.MaxPitch {
			add("pitch_range", "pitch", fmt.Sprintf("%d-%d", limits.MinPitch, limits.MaxPitch), fmt.Sprintf("%d", n.Pitch), i)
		}
		if n.Velocity < 1 || n.Velocity > 127 {
			add("velocity_range", "velocity", "1-127", fmt.Sprintf("%d", n.Velocity), i)
		}
		if n.Length <= 0 {
			add("length_positive", "length", ">0", fmt.Sprintf("%g", n.Length), i)
		} else if n.Length > limits.MaxLengthBeats {
			add("length_cap", "length", fmt.Sprintf("≤%g beats", limits.MaxLengthBeats), fmt.Sprintf("%g", n.Length), i)
		}
		// grid alignment (tempo compliance degenerates to the grid check in
		// the beats coordinate system — 03 §3 clarification)
		if dist := math.Abs(n.Start/gridBeat-math.Round(n.Start/gridBeat)) * gridBeat; dist > gridTolerance+floatEpsilon {
			add("grid_alignment", "start", fmt.Sprintf("on 1/16 grid (±1/64 beat, dist %.6f)", dist), fmt.Sprintf("%g", n.Start), i)
		}
		// key membership (hard-when-set: no key, no constraint)
		if keySet != nil && !keySet[((n.Pitch%12)+12)%12] {
			add("key_membership", "pitch", fmt.Sprintf("in %s %s set", snap.KeySignature.RootName, snap.KeySignature.Scale), fmt.Sprintf("%d (%s)", n.Pitch, PitchClassName(n.Pitch)), i)
		}
	}
	return out
}

// SMFInfo is what the agent-side pre-parse extracts from a Standard MIDI
// File before an import command (03 §5: the parser must not lean on kernel
// error messages — the SMF chunk-id silent-tolerance note).
type SMFInfo struct {
	TicksPerQuarter int
	HasTempoMeta    bool
	TempoUSPerQN    int64 // µs per quarter note from the first tempo meta (SMF default 500000 = 120 BPM)
	HasTimeSigMeta  bool
	TimeSignature   string      // "nn/dd" from the first time-signature meta
	NotePitches     map[int]int // pitch -> occurrence count
}

// ParseSMF is a minimal read-only SMF parser (metrical division only; SMPTE
// timing rejected). It collects tempo / time-signature metas and note
// on/off pairs including running status, and skips everything else.
func ParseSMF(data []byte) (*SMFInfo, error) {
	if len(data) < 14 || string(data[0:4]) != "MThd" {
		return nil, fmt.Errorf("genctx: not an SMF file (missing MThd)")
	}
	headerLen := int(binary.BigEndian.Uint32(data[4:8]))
	if headerLen < 6 || len(data) < 8+headerLen {
		return nil, fmt.Errorf("genctx: bad MThd length %d", headerLen)
	}
	format := binary.BigEndian.Uint16(data[8:10])
	if format > 2 {
		return nil, fmt.Errorf("genctx: unsupported SMF format %d", format)
	}
	division := int(int16(binary.BigEndian.Uint16(data[12:14])))
	if division <= 0 {
		return nil, fmt.Errorf("genctx: SMPTE-timed SMF (division %d) unsupported in v1", division)
	}
	info := &SMFInfo{TicksPerQuarter: division, NotePitches: map[int]int{}}
	info.TempoUSPerQN = 500000 // SMF default until a tempo meta says otherwise

	pos := 8 + headerLen
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkLen := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		bodyStart := pos + 8
		bodyEnd := bodyStart + chunkLen
		if bodyEnd > len(data) {
			return nil, fmt.Errorf("genctx: truncated chunk %q", chunkID)
		}
		if chunkID == "MTrk" {
			if err := parseTrack(data[bodyStart:bodyEnd], info); err != nil {
				return nil, err
			}
		}
		// unknown chunks are skipped (JUCE tolerates them — and the GEN-MAC
		// run1 probe story showed a wrong-case id being silently ignored, so
		// we surface nothing here either; import-time checks catch emptiness)
		pos = bodyEnd
	}
	return info, nil
}

func parseTrack(track []byte, info *SMFInfo) error {
	var status byte
	pos := 0
	readVar := func() (int64, error) {
		var v int64
		for {
			if pos >= len(track) {
				return 0, fmt.Errorf("genctx: truncated variable-length quantity")
			}
			b := track[pos]
			pos++
			v = v<<7 | int64(b&0x7f)
			if b&0x80 == 0 {
				return v, nil
			}
		}
	}
	for pos < len(track) {
		if _, err := readVar(); err != nil { // delta time
			return err
		}
		if pos >= len(track) {
			return fmt.Errorf("genctx: truncated track event")
		}
		b := track[pos]
		if b&0x80 == 0 {
			// running status: reuse last status byte
			if status == 0 {
				return fmt.Errorf("genctx: running status without preceding status byte")
			}
		} else {
			status = b
			pos++
		}
		switch {
		case status == 0xFF:
			if pos >= len(track) {
				return fmt.Errorf("genctx: truncated meta event")
			}
			metaType := track[pos]
			pos++
			metaLen, err := readVar()
			if err != nil || pos+int(metaLen) > len(track) {
				return fmt.Errorf("genctx: truncated meta event body")
			}
			body := track[pos : pos+int(metaLen)]
			pos += int(metaLen)
			switch metaType {
			case 0x51: // tempo
				if !info.HasTempoMeta && len(body) >= 3 {
					info.HasTempoMeta = true
					info.TempoUSPerQN = int64(body[0])<<16 | int64(body[1])<<8 | int64(body[2])
				}
			case 0x58: // time signature
				if !info.HasTimeSigMeta && len(body) >= 2 {
					info.HasTimeSigMeta = true
					info.TimeSignature = fmt.Sprintf("%d/%d", body[0], 1<<body[1])
				}
			}
		case status == 0xF0 || status == 0xF7:
			sysLen, err := readVar()
			if err != nil || pos+int(sysLen) > len(track) {
				return fmt.Errorf("genctx: truncated sysex event")
			}
			pos += int(sysLen)
		case status&0xF0 == 0x90:
			if pos+2 > len(track) {
				return fmt.Errorf("genctx: truncated note-on")
			}
			pitch, vel := int(track[pos]), int(track[pos+1])
			pos += 2
			if vel > 0 {
				info.NotePitches[pitch]++
			}
		case status&0xF0 == 0x80:
			if pos+2 > len(track) {
				return fmt.Errorf("genctx: truncated note-off")
			}
			pos += 2
		default:
			// channel messages with two data bytes we don't track
			if pos+2 > len(track) {
				return fmt.Errorf("genctx: truncated channel event")
			}
			pos += 2
		}
	}
	return nil
}

// ValidateMidiFile is the write-front gate for form-B products (external MIDI
// files before import_midi_to_track). It combines the agent-side SMF
// pre-parse with the project truth:
//  1. tempo: SMF tempo meta (or the SMF default 120 when absent) vs project
//     tempo_bpm within ±0.5 BPM (03 §3 leaves the exact window to the
//     implementation card; v1 pins 0.5);
//  2. time signature: meta must equal the project's — mismatch rejects with
//     a regenerate hint (v1 offers no transpose option: uniformity first);
//  3. key membership (when set): every note pitch must be in the pitch set.
func ValidateMidiFile(snap Snapshot, smf *SMFInfo) []Violation {
	var out []Violation
	add := func(kind, field, expected, actual string) {
		out = append(out, Violation{Kind: kind, Field: field, Expected: expected, Actual: actual, NoteIndex: -1})
	}
	smfBPM := 60_000_000.0 / float64(smf.TempoUSPerQN)
	if math.Abs(smfBPM-snap.TempoBPM) > 0.5+floatEpsilon {
		add("tempo_mismatch", "tempo_bpm", fmt.Sprintf("%.1f (project)", snap.TempoBPM), fmt.Sprintf("%.1f%s", smfBPM, metaLabel(smf.HasTempoMeta)))
	}
	if smf.HasTimeSigMeta && smf.TimeSignature != snap.TimeSignature {
		add("timesig_mismatch", "time_signature", fmt.Sprintf("%s (project; regenerate with project time signature — no transpose in v1)", snap.TimeSignature), smf.TimeSignature)
	}
	if keySet := PitchClassSet(snap.KeySignature); keySet != nil {
		for pitch, count := range smf.NotePitches {
			if !keySet[((pitch%12)+12)%12] {
				add("key_membership", "pitch", fmt.Sprintf("in %s %s set", snap.KeySignature.RootName, snap.KeySignature.Scale), fmt.Sprintf("%d ×%d (%s)", pitch, count, PitchClassName(pitch)))
			}
		}
	}
	return out
}

func metaLabel(has bool) string {
	if has {
		return " (SMF tempo meta)"
	}
	return " (SMF default, no tempo meta)"
}
