package genctx

import (
	"strings"
	"testing"
)

// Golden synthetic-integration cases (card acceptance): mock get_project_state
// → assemble → render → validators, covering the three spec cases:
//  1. key=C major with a black-key note → constraint_violation, write refused;
//  2. 4/4 grid, 1/32-beat offset beyond tolerance → violation, write refused;
//  3. legal sequence → zero violations, prompt shape golden.
func TestGoldenSyntheticIntegration(t *testing.T) {
	cases := []struct {
		name     string
		notes    []Note
		wantKind string // "" = must pass
	}{
		{"black-key-reject", []Note{{Pitch: 61, Start: 0, Length: 1, Velocity: 90}}, "key_membership"},
		{"grid-offset-reject", []Note{{Pitch: 60, Start: 1.0 + 1.0/32.0, Length: 1, Velocity: 90}}, "grid_alignment"},
		{"legal-pass", []Note{
			{Pitch: 60, Start: 0, Length: 0.5, Velocity: 90},
			{Pitch: 64, Start: 0.5, Length: 0.5, Velocity: 80},
			{Pitch: 67, Start: 1.0, Length: 0.5, Velocity: 70},
		}, ""},
	}
	for _, c := range cases {
		state := testState(120, 4, 4, cMajorKey(), "暖色钢琴主导，克制", []any{
			map[string]any{"name": "verse", "start_seconds": 0.0, "end_seconds": 32.0},
		})
		snap, err := AssembleSnapshot(state, SceneInProject, CallParams{HarmonicNote: "I-vi-IV-V"})
		if err != nil {
			t.Fatalf("%s: assemble: %v", c.name, err)
		}
		prompt, err := RenderGenerationPrompt(snap, "write four bars of drums", RenderProfile{ConsumerID: "gen_plugin"})
		if err != nil {
			t.Fatalf("%s: render: %v", c.name, err)
		}
		if !strings.Contains(prompt, "== MUSIC CONTEXT (project, music.context.v1, revision ab12cd) ==") {
			t.Errorf("%s: context header missing:\n%s", c.name, prompt)
		}
		if !strings.Contains(prompt, "---\nwrite four bars of drums") {
			t.Errorf("%s: user block must be last:\n%s", c.name, prompt)
		}
		v := ValidateNotes(snap, c.notes, DefaultLimits)
		if c.wantKind == "" {
			if len(v) != 0 {
				t.Errorf("%s: legal sequence must have zero violations, got %v", c.name, v)
			}
			continue
		}
		if len(v) == 0 {
			t.Errorf("%s: expected %s violation, got none", c.name, c.wantKind)
			continue
		}
		if v[0].Kind != c.wantKind {
			t.Errorf("%s: first violation kind = %s, want %s (all: %v)", c.name, v[0].Kind, c.wantKind, v)
		}
	}
}

// The legal-pass prompt pinned as a golden string (template-engine shape
// lock — template evolution is engine-version evolution, 03 §1.1).
func TestGoldenPromptShape(t *testing.T) {
	state := testState(120, 4, 4, cMajorKey(), "暖色钢琴主导，克制", nil)
	snap, _ := AssembleSnapshot(state, SceneInProject, CallParams{})
	snap.CapturedAt = "2026-10-12T02:00:00Z" // pin for the golden
	prompt, err := RenderGenerationPrompt(snap, "drums please", RenderProfile{ConsumerID: "gen_plugin"})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(prompt), "\n")
	want := []string{
		"== MUSIC CONTEXT (project, music.context.v1, revision ab12cd) ==",
		"tempo: 120.0 BPM (initial; automation: none)",
		"time signature: 4/4",
		"key: C major (set by user)",
		"== CONSTRAINTS (machine-checked before write) ==",
		"MUST 1: note times are in beats (clip-relative, beat 0 = clip start), time_unit:\"beats\"",
		"MUST 2: rhythm positions land on the 4/4 grid (16th-note resolution; tolerance ±1/64 beat)",
		"MUST 3: all MIDI pitches are in the C major pitch set {C,D,E,F,G,A,B} + octaves",
		"MUST 4: velocity 1-127, length > 0, pitch within 24-96",
		"== GUIDANCE ==",
		"style note: 暖色钢琴主导，克制",
		"---",
		"drums please",
	}
	if len(lines) != len(want) {
		t.Fatalf("golden shape: %d lines, want %d:\n%s", len(lines), len(want), prompt)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("golden line %d:\nwant %q\ngot  %q", i, want[i], lines[i])
		}
	}
}
