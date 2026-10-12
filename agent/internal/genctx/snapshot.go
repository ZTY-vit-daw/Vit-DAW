// Package genctx implements the unified project music-context layer
// (GEN-UNIFY-CTX-DESIGN-1, card GEN-CTX-INJECT-1): snapshot assembly from
// get_project_state, the single generation-prompt renderer, write-front
// machine validation, and post-write read-back verification.
//
// Design sources (authoritative, in coord/runs/GEN-UNIFY-CTX-DESIGN-1/):
// 01-context-inventory.md (grading contract, pitch-class sets, snapshot model),
// 03-injection-contract.md (template, profile, validation matrix, read-back),
// 04-scene-boundary.md (scene enum + write-command sentinel).
//
// Red line: this package must not be imported by capabilitycontext /
// contextruntime / promptruntime, and it imports none of them (chat assembly
// and generation rendering stay separate, 03 §1.2).
package genctx

import (
	"fmt"
	"strings"
	"time"
)

// SchemaVersion is the snapshot schema identifier. Consumers must
// fail-closed on unknown versions (v2 analysis fields will bump it).
const SchemaVersion = "music.context.v1"

// Scene classifies one generation call (04-scene-boundary.md §1). It is a
// per-call explicit parameter, defaulting to in_project; there is deliberately
// no global switch and no project-level persisted field.
type Scene string

const (
	// SceneInProject is the default: production into the project (write to
	// track / media pool / render chain) — injection and machine checks both
	// mandatory.
	SceneInProject Scene = "in_project"
	// SceneFreeCreation is exempt: autonomous creation outside the project;
	// zero auto-context blocks, no machine gate.
	SceneFreeCreation Scene = "free_creation"
	// SceneExperimentControl is the in-project experiment "off" arm for the
	// thesis compliance-rate experiment; zero auto-context, no gate, products
	// tagged by consumers for exclusion from production statistics.
	SceneExperimentControl Scene = "experiment_control"
)

// ParseScene validates a scene string; unknown values fail closed.
func ParseScene(s string) (Scene, error) {
	switch Scene(s) {
	case SceneInProject, SceneFreeCreation, SceneExperimentControl:
		return Scene(s), nil
	default:
		return "", fmt.Errorf("genctx: unknown scene %q (want one of in_project|free_creation|experiment_control)", s)
	}
}

// KeySignature is the project key field (hard-when-set). Nil on the snapshot
// means "not set" — no constraint is fabricated for an unset key.
type KeySignature struct {
	RootPitch int    `json:"root_pitch"`
	RootName  string `json:"root_name"`
	Scale     string `json:"scale"`
	SetBy     string `json:"set_by,omitempty"`
	SetAt     string `json:"set_at,omitempty"`
}

// Section is a project section derived from get_project_state markers
// (inject-only in v1; marker name becomes the section name, marker seconds
// are converted to beats at the current constant tempo).
type Section struct {
	Name      string  `json:"name"`
	StartBeat float64 `json:"start_beat"`
	EndBeat   float64 `json:"end_beat"`
}

// Snapshot is the single authoritative intermediate form every consumer
// renders from (01 §4). Consumers must not bypass it straight to kernel
// fields.
type Snapshot struct {
	SchemaVersion       string        `json:"schema_version"`
	CapturedAt          string        `json:"captured_at"`
	ProjectRevision     string        `json:"project_revision"`
	Scene               Scene         `json:"scene"`
	TempoBPM            float64       `json:"tempo_bpm"`
	TempoSettingCount   int           `json:"tempo_setting_count"`
	TimeSignature       string        `json:"time_signature"`
	TimeSigNumerator    int           `json:"time_signature_numerator"`
	TimeSigDenominator  int           `json:"time_signature_denominator"`
	TimeSigSettingCount int           `json:"time_sig_setting_count"`
	KeySignature        *KeySignature `json:"key_signature"`
	StyleNote           string        `json:"style_note"`
	HarmonicNote        string        `json:"harmonic_note"`
	Sections            []Section     `json:"sections"`
	EnergyNote          string        `json:"energy_note"`
}

// CallParams carries the per-call free-text inputs that are not persisted on
// the project (harmonic / energy notes, v1 provenance = call parameters).
type CallParams struct {
	HarmonicNote string
	EnergyNote   string
	// Now lets tests pin captured_at; defaults to time.Now().
	Now func() time.Time
}

// AssembleSnapshot builds a Snapshot from a get_project_state response map
// (kernel truth via the music_context disclosure of GEN-CTX-DISCLOSE-1 plus
// the pre-existing markers key) and the call parameters. scene comes in
// explicitly (default in_project) so the audit record always carries it.
func AssembleSnapshot(state map[string]any, scene Scene, call CallParams) (Snapshot, error) {
	if scene == "" {
		scene = SceneInProject
	}
	if _, err := ParseScene(string(scene)); err != nil {
		return Snapshot{}, err
	}
	now := call.Now
	if now == nil {
		now = time.Now
	}
	snap := Snapshot{
		SchemaVersion:   SchemaVersion,
		CapturedAt:      now().UTC().Format(time.RFC3339),
		ProjectRevision: revisionFromState(state),
		Scene:           scene,
	}

	if mc, ok := state["music_context"].(map[string]any); ok {
		snap.TempoBPM = floatOf(mc["tempo_bpm"])
		snap.TempoSettingCount = intOf(mc["tempo_setting_count"])
		snap.TimeSignature = stringOf(mc["time_signature"])
		snap.TimeSigNumerator = intOf(mc["time_signature_numerator"])
		snap.TimeSigDenominator = intOf(mc["time_signature_denominator"])
		snap.TimeSigSettingCount = intOf(mc["time_sig_setting_count"])
		snap.StyleNote = stringOf(mc["style_note"])
		if key, ok := mc["key_signature"].(map[string]any); ok && key != nil {
			ks := &KeySignature{
				RootPitch: intOf(key["root_pitch"]),
				RootName:  stringOf(key["root_name"]),
				Scale:     stringOf(key["scale"]),
				SetBy:     stringOf(key["set_by"]),
				SetAt:     stringOf(key["set_at"]),
			}
			if ks.RootName == "" {
				ks.RootName = PitchClassName(ks.RootPitch)
			}
			if _, err := ScaleSteps(ks.Scale); err != nil {
				return Snapshot{}, fmt.Errorf("genctx: project key_signature has unknown scale %q: %w", ks.Scale, err)
			}
			if ks.RootPitch < 0 || ks.RootPitch > 127 {
				return Snapshot{}, fmt.Errorf("genctx: project key_signature root_pitch %d out of 0-127", ks.RootPitch)
			}
			snap.KeySignature = ks
		}
	}
	// tempo/time-signature are hard fields with engine-guaranteed truth
	// (TempoSequence auto-fills defaults); a state map without them is not a
	// shape we should silently bless.
	if snap.TempoBPM <= 0 {
		return Snapshot{}, fmt.Errorf("genctx: state has no usable tempo_bpm (music_context missing?)")
	}
	if snap.TimeSigNumerator <= 0 || snap.TimeSigDenominator <= 0 {
		return Snapshot{}, fmt.Errorf("genctx: state has no usable time signature (music_context missing?)")
	}

	snap.Sections = sectionsFromMarkers(state["markers"], snap.TempoBPM)
	snap.HarmonicNote = strings.TrimSpace(call.HarmonicNote)
	snap.EnergyNote = strings.TrimSpace(call.EnergyNote)
	return snap, nil
}

// revisionFromState picks the graph revision the kernel reports alongside the
// state (appendGraphRevisionProperties semantics); tolerant to key drift.
func revisionFromState(state map[string]any) string {
	for _, k := range []string{"graph_revision", "revision"} {
		if v, ok := state[k]; ok {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return fmt.Sprintf("%d", int64(t))
			}
		}
	}
	return ""
}

// sectionsFromMarkers converts marker entries (name / start_seconds /
// end_seconds, ProjectMarkerService.cpp snapshot shape) into beat spans at a
// constant tempo. v1 note: tempo automation segments are not modelled — with
// multiple tempo settings the conversion uses tempo setting 0 only (01 §2 v2
// direction).
func sectionsFromMarkers(raw any, tempoBPM float64) []Section {
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]Section, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(stringOf(m["name"]))
		start := floatOf(m["start_seconds"])
		end := floatOf(m["end_seconds"])
		if name == "" && start == 0 && end == 0 {
			continue
		}
		out = append(out, Section{
			Name:      name,
			StartBeat: secondsToBeats(start, tempoBPM),
			EndBeat:   secondsToBeats(end, tempoBPM),
		})
	}
	return out
}

func secondsToBeats(seconds, tempoBPM float64) float64 {
	if tempoBPM <= 0 {
		return 0
	}
	return seconds * tempoBPM / 60.0
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

func floatOf(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	default:
		return 0
	}
}

func intOf(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	default:
		return 0
	}
}

// writeCommands are the project-write commands whose preceding generation
// call must be scene=in_project (04 §3: the product-landing criterion — the
// sentinel lives at the write front). Both kernel names and catalog aliases
// are listed.
var writeCommands = map[string]bool{
	"add_midi_notes":                true,
	"add_midi_notes_bulk":           true,
	"apply_midi_note_patch":         true,
	"insert_midi_clip":              true,
	"import_midi_to_track":          true,
	"bridge_ingest_generated_asset": true,
	"assets.ingest_generated_asset": true,
	"midi.write_clip_notes":         true,
	"midi.add_notes_bulk":           true,
}

// GuardWriteCommand is the write-front sentinel: writing generation products
// into the project is only allowed for in_project calls. Under an exempt
// scene (free_creation / experiment_control) dispatching any of the write
// commands is a contract violation — the orchestrator must refuse.
func GuardWriteCommand(scene Scene, command string) error {
	if scene == SceneInProject || !writeCommands[command] {
		return nil
	}
	return fmt.Errorf("genctx: scene %q forbids project-write command %q (exempt scenes produce zero project writes; re-issue with scene=in_project)", scene, command)
}
