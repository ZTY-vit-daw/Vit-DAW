package genctx

import (
	"fmt"
	"math"
)

// ReadbackNote is the neutral shape of a note as read back through
// get_midi_clip_notes (read commands are RiskDirect — zero-interaction
// verification, 03 §4).
type ReadbackNote struct {
	Pitch    int     `json:"pitch"`
	Start    float64 `json:"start"`
	Length   float64 `json:"length"`
	Velocity int     `json:"velocity"`
}

// ReadbackMismatch is one second-gate discrepancy. A mismatch means a write
// ACCIDENT (not a generation violation): report immediately and mark the clip
// for human handling — never auto-delete (project data already landed;
// deletion is destructive and needs explicit confirmation).
type ReadbackMismatch struct {
	Field     string `json:"field"`
	NoteIndex int    `json:"note_index"`
	Expected  string `json:"expected"`
	Actual    string `json:"actual"`
}

func (m ReadbackMismatch) String() string {
	return fmt.Sprintf("readback_mismatch{%s note %d: expected %s, actual %s}", m.Field, m.NoteIndex, m.Expected, m.Actual)
}

// beatTolerance is the float slack for beat comparisons (kernel round-trips
// beats exactly per GEN-MAC read-back evidence; the epsilon only guards
// JSON float formatting).
const beatTolerance = 1e-6

// VerifyReadback is the second gate after a successful write command
// (03 §4): the read-back must match the dispatched sequence on
// count/pitch/start/length/velocity, and — when a key is set — the pitch
// collection must stay inside the pitch set (catching id-mapping /
// coordinate-swap accidents even when every value individually survived).
func VerifyReadback(snap Snapshot, sent []Note, got []ReadbackNote) []ReadbackMismatch {
	var out []ReadbackMismatch
	if len(sent) != len(got) {
		out = append(out, ReadbackMismatch{Field: "note_count", NoteIndex: -1,
			Expected: fmt.Sprintf("%d", len(sent)), Actual: fmt.Sprintf("%d", len(got))})
	}
	keySet := PitchClassSet(snap.KeySignature)
	n := len(sent)
	if len(got) < n {
		n = len(got)
	}
	for i := 0; i < n; i++ {
		s, g := sent[i], got[i]
		if s.Pitch != g.Pitch {
			out = append(out, ReadbackMismatch{Field: "pitch", NoteIndex: i,
				Expected: fmt.Sprintf("%d", s.Pitch), Actual: fmt.Sprintf("%d", g.Pitch)})
		}
		if math.Abs(s.Start-g.Start) > beatTolerance {
			out = append(out, ReadbackMismatch{Field: "start", NoteIndex: i,
				Expected: fmt.Sprintf("%g", s.Start), Actual: fmt.Sprintf("%g", g.Start)})
		}
		if math.Abs(s.Length-g.Length) > beatTolerance {
			out = append(out, ReadbackMismatch{Field: "length", NoteIndex: i,
				Expected: fmt.Sprintf("%g", s.Length), Actual: fmt.Sprintf("%g", g.Length)})
		}
		if s.Velocity != g.Velocity {
			out = append(out, ReadbackMismatch{Field: "velocity", NoteIndex: i,
				Expected: fmt.Sprintf("%d", s.Velocity), Actual: fmt.Sprintf("%d", g.Velocity)})
		}
		if keySet != nil && !keySet[((g.Pitch%12)+12)%12] {
			out = append(out, ReadbackMismatch{Field: "pitch_set_subset", NoteIndex: i,
				Expected: fmt.Sprintf("in %s %s set", snap.KeySignature.RootName, snap.KeySignature.Scale),
				Actual:   fmt.Sprintf("%d (%s)", g.Pitch, PitchClassName(g.Pitch))})
		}
	}
	return out
}
