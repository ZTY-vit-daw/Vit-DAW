package genctx

import (
	"fmt"
	"strings"
)

// RenderProfile is the consumer take-up face (03 §2): consumers declare who
// they are and may only TRIM which fields/blocks they consume. There is no
// custom-template entry point — template evolution happens in this engine
// only, which is what "生成插件壳/外部适配器/VitTape/编曲共用同一契约"
// means in verifiable form.
type RenderProfile struct {
	// ConsumerID is one of gen_api | gen_plugin | arrange | vitreel (free
	// string in v1, recorded for audit).
	ConsumerID string
	// IncludeFields trims field-level consumption. Nil = all seven fields
	// (tempo, time_signature, key_signature, style_note, sections,
	// harmonic_note, energy_note). Unknown names fail closed at render.
	IncludeFields []string
	// IncludeBlocks trims block-level consumption. Nil = all four blocks
	// (music_context, constraints, guidance, user).
	IncludeBlocks []string
}

var allFields = []string{"tempo", "time_signature", "key_signature", "style_note", "sections", "harmonic_note", "energy_note"}

var allBlocks = []string{"music_context", "constraints", "guidance", "user"}

func (p RenderProfile) fieldOn(f string) (bool, error) {
	if p.IncludeFields == nil {
		return true, nil
	}
	for _, x := range p.IncludeFields {
		if x == f {
			return true, nil
		}
	}
	return false, nil
}

func (p RenderProfile) blockOn(b string) bool {
	if p.IncludeBlocks == nil {
		return true
	}
	for _, x := range p.IncludeBlocks {
		if x == b {
			return true
		}
	}
	return false
}

// RenderGenerationPrompt is the single template engine (03 §1.1). Consumers
// pass a snapshot, their free user text, and a trim-only profile; they cannot
// inject custom template text.
//
// Exempt scenes (free_creation / experiment_control) omit every auto block —
// the prompt is the user block alone (04 §1: exemption is "zero automatic
// context", and consumers must not smuggle project fields back in).
func RenderGenerationPrompt(snap Snapshot, userFreeText string, profile RenderProfile) (string, error) {
	if snap.SchemaVersion != SchemaVersion {
		return "", fmt.Errorf("genctx: refusing to render snapshot schema %q (engine knows %q)", snap.SchemaVersion, SchemaVersion)
	}
	for _, f := range profile.IncludeFields {
		known := false
		for _, x := range allFields {
			if x == f {
				known = true
				break
			}
		}
		if !known {
			return "", fmt.Errorf("genctx: profile includes unknown field %q", f)
		}
	}
	for _, b := range profile.IncludeBlocks {
		known := false
		for _, x := range allBlocks {
			if x == b {
				known = true
				break
			}
		}
		if !known {
			return "", fmt.Errorf("genctx: profile includes unknown block %q", b)
		}
	}

	var b strings.Builder
	exempt := snap.Scene != SceneInProject

	if !exempt && profile.blockOn("music_context") {
		writeMusicContextBlock(&b, snap, profile)
	}
	if !exempt && profile.blockOn("constraints") {
		writeConstraintsBlock(&b, snap, profile)
	}
	if !exempt && profile.blockOn("guidance") {
		writeGuidanceBlock(&b, snap, profile)
	}
	// User block: verbatim, trimmed of outer whitespace only, always last.
	if profile.blockOn("user") {
		if b.Len() > 0 {
			b.WriteString("---\n")
		}
		b.WriteString(strings.Trim(userFreeText, " \t\n\r"))
		b.WriteString("\n")
	}
	return b.String(), nil
}

func writeMusicContextBlock(b *strings.Builder, snap Snapshot, profile RenderProfile) {
	rev := snap.ProjectRevision
	if rev == "" {
		rev = "n/a"
	}
	fmt.Fprintf(b, "== MUSIC CONTEXT (project, %s, revision %s) ==\n", SchemaVersion, rev)
	if on, _ := profile.fieldOn("tempo"); on {
		automation := "none"
		if snap.TempoSettingCount > 1 {
			automation = fmt.Sprintf("%d settings — setting 0 shown", snap.TempoSettingCount)
		}
		fmt.Fprintf(b, "tempo: %.1f BPM (initial; automation: %s)\n", snap.TempoBPM, automation)
	}
	if on, _ := profile.fieldOn("time_signature"); on {
		ts := snap.TimeSignature
		if ts == "" {
			ts = fmt.Sprintf("%d/%d", snap.TimeSigNumerator, snap.TimeSigDenominator)
		}
		fmt.Fprintf(b, "time signature: %s\n", ts)
	}
	if on, _ := profile.fieldOn("key_signature"); on {
		if snap.KeySignature == nil {
			// Unset stays visible: "didn't inject" must be distinguishable
			// from "injected but unset" when auditing a generation failure.
			b.WriteString("key: (not set)\n")
		} else {
			fmt.Fprintf(b, "key: %s %s (set by %s)\n", snap.KeySignature.RootName, snap.KeySignature.Scale, setByLabel(snap.KeySignature.SetBy))
		}
	}
}

func setByLabel(setBy string) string {
	if setBy == "" {
		return "user"
	}
	return setBy
}

func writeConstraintsBlock(b *strings.Builder, snap Snapshot, profile RenderProfile) {
	b.WriteString("== CONSTRAINTS (machine-checked before write) ==\n")
	must := 0
	next := func() int { must++; return must }
	// MUST 1 is the coordinate system contract — it rides along whenever the
	// tempo field is consumed (beats ARE the project tempo).
	if on, _ := profile.fieldOn("tempo"); on {
		fmt.Fprintf(b, "MUST %d: note times are in beats (clip-relative, beat 0 = clip start), time_unit:\"beats\"\n", next())
	}
	if on, _ := profile.fieldOn("time_signature"); on {
		ts := snap.TimeSignature
		if ts == "" {
			ts = fmt.Sprintf("%d/%d", snap.TimeSigNumerator, snap.TimeSigDenominator)
		}
		fmt.Fprintf(b, "MUST %d: rhythm positions land on the %s grid (16th-note resolution; tolerance ±1/64 beat)\n", next(), ts)
	}
	if on, _ := profile.fieldOn("key_signature"); on && snap.KeySignature != nil {
		fmt.Fprintf(b, "MUST %d: all MIDI pitches are in the %s %s pitch set {%s} + octaves\n",
			next(), snap.KeySignature.RootName, snap.KeySignature.Scale, PitchSetNames(snap.KeySignature))
	}
	fmt.Fprintf(b, "MUST %d: velocity 1-127, length > 0, pitch within %d-%d\n", next(), DefaultLimits.MinPitch, DefaultLimits.MaxPitch)
}

func writeGuidanceBlock(b *strings.Builder, snap Snapshot, profile RenderProfile) {
	var lines []string
	if on, _ := profile.fieldOn("style_note"); on && strings.TrimSpace(snap.StyleNote) != "" {
		// soft grading: advisory "style note:" phrasing — never MUST wording
		// (a suggestion must not read as a constraint).
		lines = append(lines, fmt.Sprintf("style note: %s", strings.TrimSpace(snap.StyleNote)))
	}
	if on, _ := profile.fieldOn("sections"); on && len(snap.Sections) > 0 {
		for _, s := range snap.Sections {
			lines = append(lines, fmt.Sprintf("section: %s [%.0f-%.0f beats]", s.Name, s.StartBeat, s.EndBeat))
		}
	}
	if on, _ := profile.fieldOn("harmonic_note"); on && snap.HarmonicNote != "" {
		lines = append(lines, fmt.Sprintf("harmonic: %s", snap.HarmonicNote))
	}
	if on, _ := profile.fieldOn("energy_note"); on && snap.EnergyNote != "" {
		lines = append(lines, fmt.Sprintf("energy: %s", snap.EnergyNote))
	}
	if len(lines) == 0 {
		return // token economy: the guidance block only appears when it has content
	}
	b.WriteString("== GUIDANCE ==\n")
	for _, l := range lines {
		b.WriteString(l)
		b.WriteString("\n")
	}
}
