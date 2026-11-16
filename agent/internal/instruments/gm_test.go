package instruments

import "testing"

func TestGMTableComplete(t *testing.T) {
	programs := GMPrograms()
	if len(programs) != 128 {
		t.Fatalf("GM table has %d entries, want 128", len(programs))
	}
	for i, program := range programs {
		if program.Number != i+1 {
			t.Fatalf("GM entry %d has number %d, want %d", i, program.Number, i+1)
		}
		if program.Name == "" || program.Family == "" {
			t.Fatalf("GM entry %d has empty name or family: %+v", i, program)
		}
	}
}

func TestGMLookupNumber(t *testing.T) {
	if program, ok := LookupGMProgram(82); !ok || program.Name != "Lead 2 (sawtooth)" {
		t.Fatalf("GM 82 = %+v ok=%v, want Lead 2 (sawtooth)", program, ok)
	}
	if program, ok := LookupGMProgram(1); !ok || program.Name != "Acoustic Grand Piano" {
		t.Fatalf("GM 1 = %+v ok=%v, want Acoustic Grand Piano", program, ok)
	}
	if _, ok := LookupGMProgram(0); ok {
		t.Fatalf("GM 0 should not resolve")
	}
	if _, ok := LookupGMProgram(129); ok {
		t.Fatalf("GM 129 should not resolve")
	}
}

// TestGMManifestCompat pins the numbering convention against the
// melody-drum-v1 template manifest (gm_program 82 = Lead 2 (sawtooth)).
func TestGMManifestCompat(t *testing.T) {
	program, ok := LookupGMProgram(82)
	if !ok || program.Name != "Lead 2 (sawtooth)" || program.ProgramChange() != 81 {
		t.Fatalf("manifest pairing broken: %+v ok=%v", program, ok)
	}
}

func TestGMLookupByName(t *testing.T) {
	cases := []struct {
		query  string
		number int
	}{
		{"Lead 2 (sawtooth)", 82},
		{"acoustic grand piano", 1},
		{"这曲用 Acoustic Grand Piano", 1},
		{"Electric Grand Piano", 3},
		{"electric piano 1", 5},
		{"Slap Bass 1", 37},
	}
	for _, tc := range cases {
		program, ok := LookupGMProgramByName(tc.query)
		if !ok || program.Number != tc.number {
			t.Fatalf("LookupGMProgramByName(%q) = %+v ok=%v, want number %d", tc.query, program, ok, tc.number)
		}
	}
	if _, ok := LookupGMProgramByName("水果沙拉"); ok {
		t.Fatalf("non-instrument text should not resolve to a GM program")
	}
}

func TestGMPercussionKeys(t *testing.T) {
	cases := []struct {
		note int
		name string
	}{
		{36, "Electric Bass Drum"}, // template percussion key "kick"
		{38, "Acoustic Snare"},     // template percussion key "snare"
		{42, "Closed Hi-Hat"},      // template percussion key "closed_hat"
		{35, "Acoustic Bass Drum"},
		{49, "Crash Cymbal 1"},
		{81, "Open Triangle"},
	}
	for _, tc := range cases {
		name, ok := GMPercussionKey(tc.note)
		if !ok || name != tc.name {
			t.Fatalf("GMPercussionKey(%d) = %q ok=%v, want %q", tc.note, name, ok, tc.name)
		}
	}
	if _, ok := GMPercussionKey(34); ok {
		t.Fatalf("note 34 is below the GM percussion map")
	}
	if _, ok := GMPercussionKey(82); ok {
		t.Fatalf("note 82 is above the GM percussion map")
	}
	notes := GMPercussionNotes()
	if len(notes) != 47 {
		t.Fatalf("GM percussion map has %d notes, want 47", len(notes))
	}
	for i := 1; i < len(notes); i++ {
		if notes[i] <= notes[i-1] {
			t.Fatalf("percussion notes not sorted: %v", notes)
		}
	}
}
