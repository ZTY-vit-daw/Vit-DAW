// Package coursetemplate loads and validates the fixed-healthy-source course
// templates (SRC-TEMPLATE-1, pool 373) and prepares per-lesson project copies.
//
// The template package lives with the lesson design (e.g.
// coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/) and is
// described by a vit.course_template.v1 manifest. The template file itself is
// a clear-XML Tracktion EDIT: opening, saving and reopening all take the
// kernel's normal clear-XML branches, and the manifest pins the plugin state
// blobs (program pinning) plus the W01 timing seam (see docs/
// LEGAME_DESIGN_CONTRACT_V0.md workspace.timing; the creation-side binding is
// W01-C / G0-LEGAME-BIND-1).
//
// This package is deliberately zero-stack: everything here is pure file and
// XML work, unit-testable without the kernel, agent or Godot frontend.
package coursetemplate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// ManifestFile is the fixed manifest filename inside a template directory.
const ManifestFile = "template_manifest.json"

// SchemaVersion is the manifest schema this package consumes.
const SchemaVersion = "vit.course_template.v1"

// Manifest mirrors template_manifest.json (vit.course_template.v1).
type Manifest struct {
	SchemaVersion   string           `json:"schema_version"`
	ID              string           `json:"id"`
	Card            string           `json:"card"`
	LessonDesign    LessonDesignRef  `json:"lesson_design"`
	TemplateFile    string           `json:"template_file"`
	TemplateSHA256  string           `json:"template_sha256"`
	TemplateUUID    string           `json:"template_project_uuid"`
	Format          FormatBlock      `json:"format"`
	Sources         map[string]Voice `json:"sources"`
	Grooves         []GrooveSpec     `json:"grooves"`
	Clips           []ClipSpec       `json:"clips"`
	WorkspaceTiming Timing           `json:"workspace_timing"`
	W01Seam         SeamBlock        `json:"w01_seam"`
	Exclusions      ExclusionsBlock  `json:"exclusions"`
	Gate            GateBlock        `json:"gate"`
	BuiltBy         string           `json:"built_by"`
	BuiltAt         string           `json:"built_at"`
}

// LessonDesignRef points at the lesson design source the template serves.
type LessonDesignRef struct {
	Source   string `json:"source"`
	Schema   string `json:"schema"`
	DesignID string `json:"design_id"`
	Revision int    `json:"revision"`
}

// ExclusionsBlock records the honest non-adoption declarations.
type ExclusionsBlock struct {
	WavesFamily         string `json:"waves_family"`
	FakePresetSwitching string `json:"fake_preset_switching"`
}

// GateBlock records the acceptance gate the template is subject to.
type GateBlock struct {
	ID     string `json:"id"`
	Spec   string `json:"spec"`
	Device string `json:"device"`
}

// FormatBlock documents the template file format and portability contract.
type FormatBlock struct {
	Kind        string `json:"kind"`
	Note        string `json:"note"`
	Portability string `json:"portability"`
}

// Voice is one instrument voice declaration (melody/theme/groove).
type Voice struct {
	Track        string      `json:"track"`
	TrackName    string      `json:"track_name"`
	Plugin       *PluginDecl `json:"plugin,omitempty"`
	PluginRef    string      `json:"plugin_ref,omitempty"`
	MIDIChannel  int         `json:"midi_channel"`
	GMMapping    *GMMapping  `json:"gm_mapping,omitempty"`
	GMMappingRef string      `json:"gm_mapping_ref,omitempty"`
}

// PluginDecl pins the plugin identity and state blob ("program 固化").
type PluginDecl struct {
	Name            string `json:"name"`
	Identifier      string `json:"identifier"`
	UniqueID        string `json:"unique_id"`
	UID             string `json:"uid"`
	Manufacturer    string `json:"manufacturer"`
	StateSHA256     string `json:"state_sha256"`
	StateProvenance string `json:"state_provenance"`
}

// GMMapping declares the GM stand-in semantics (never a compatibility claim).
type GMMapping struct {
	Kind           string            `json:"kind"`
	GMProgram      int               `json:"gm_program,omitempty"`
	GMName         string            `json:"gm_name,omitempty"`
	PercussionKeys map[string]string `json:"percussion_keys,omitempty"`
	Note           string            `json:"note"`
}

// GrooveSpec describes one auxiliary groove preset (template-library form).
type GrooveSpec struct {
	ID             string `json:"id"`
	ClipAlias      string `json:"clip_alias"`
	Track          string `json:"track"`
	TimeSignature  string `json:"time_signature"`
	Bars           int    `json:"bars"`
	Beats          int    `json:"beats"`
	PatternSummary string `json:"pattern_summary"`
	NoteCount      int    `json:"note_count"`
}

// ClipSpec describes one student scaffold clip (placeholder pitch etc.).
type ClipSpec struct {
	Alias            string `json:"alias"`
	Track            string `json:"track"`
	TimeSignature    string `json:"time_signature"`
	Bars             int    `json:"bars"`
	Beats            int    `json:"beats"`
	Kind             string `json:"kind"`
	PlaceholderPitch int    `json:"placeholder_pitch,omitempty"`
	Note             string `json:"note"`
}

// Timing mirrors design workspace.timing (W01 contract field-for-field).
type Timing struct {
	TimeSignature   string  `json:"time_signature"`
	KeySignature    string  `json:"key_signature"`
	TempoBPM        float64 `json:"tempo_bpm"`
	PickupBeats     int     `json:"pickup_beats"`
	TempoProvenance string  `json:"tempo_provenance,omitempty"`
}

// SeamBlock documents the W01-C creation-side binding seam.
type SeamBlock struct {
	Contract      string      `json:"contract"`
	BindingTarget string      `json:"binding_target"`
	FieldMap      []SeamField `json:"field_map"`
	TrackMap      []SeamTrack `json:"track_map"`
	OutOfScope    string      `json:"out_of_scope"`
}

// SeamField is one design-field -> template-pin -> creation-command seam row.
type SeamField struct {
	DesignField     string `json:"design_field"`
	TemplatePin     string `json:"template_pin"`
	ApplyAtCreation string `json:"apply_at_creation"`
}

// SeamTrack maps a design track id onto the template track name.
type SeamTrack struct {
	DesignTrack string `json:"design_track"`
	TrackName   string `json:"track_name"`
	Instrument  bool   `json:"instrument"`
}

// Loaded is a manifest + parsed template pair with verified file hash.
type Loaded struct {
	Dir          string
	Manifest     Manifest
	TemplatePath string
	TemplateSHA  string
	Edit         EditDoc
}

// EditDoc is the parsed clear-XML EDIT (only the pins this package asserts).
type EditDoc struct {
	ProjectUUID  string
	KeySignature string
	Numerator    int
	Denominator  int
	TempoBPM     float64
	Tracks       []TrackDoc
}

// TrackDoc is one TRACK with its VST instrument and MIDI clips.
type TrackDoc struct {
	Name     string
	ID       string
	HasVST   bool
	VSTName  string
	VSTUID   string
	VSTState string
	StateSHA string
	Clips    []ClipDoc
}

// ClipDoc is one MIDICLIP with its notes.
type ClipDoc struct {
	Name          string
	LengthBeats   float64
	ChannelNumber int
	Notes         []NoteDoc
}

// NoteDoc is one NOTE.
type NoteDoc struct {
	Pitch      int
	StartBeat  float64
	LengthBeat float64
	Velocity   int
	Channel    int
	NoteID     string
}

type xmlEdit struct {
	XMLName       xml.Name `xml:"EDIT"`
	ProjectUUID   string   `xml:"vit_project_uuid,attr"`
	KeySignature  string   `xml:"vit_music_key_signature,attr"`
	TempoSequence struct {
		Tempo struct {
			BPM string `xml:"bpm,attr"`
		} `xml:"TEMPO"`
		TimeSig struct {
			Numerator   string `xml:"numerator,attr"`
			Denominator string `xml:"denominator,attr"`
		} `xml:"TIMESIG"`
	} `xml:"TEMPOSEQUENCE"`
	Tracks []xmlTrack `xml:"TRACK"`
}

type xmlTrack struct {
	Name    string      `xml:"name,attr"`
	ID      string      `xml:"id,attr"`
	Plugins []xmlPlugin `xml:"PLUGIN"`
	Clips   []xmlClip   `xml:"MIDICLIP"`
}

type xmlPlugin struct {
	Type     string `xml:"type,attr"`
	Name     string `xml:"name,attr"`
	UniqueID string `xml:"uniqueId,attr"`
	UID      string `xml:"uid,attr"`
	State    string `xml:"state,attr"`
}

type xmlClip struct {
	Name   string `xml:"name,attr"`
	Length string `xml:"length,attr"`
	Seq    struct {
		ChannelNumber int       `xml:"channelNumber,attr"`
		Notes         []xmlNote `xml:"NOTE"`
	} `xml:"SEQUENCE"`
}

type xmlNote struct {
	Pitch    string `xml:"p,attr"`
	Beat     string `xml:"b,attr"`
	Length   string `xml:"l,attr"`
	Velocity string `xml:"v,attr"`
	Channel  string `xml:"c,attr"`
	NoteID   string `xml:"vit_note_id,attr"`
}

// Load reads the manifest + template from dir and verifies the file hash.
func Load(dir string) (*Loaded, error) {
	manifestPath := filepath.Join(dir, ManifestFile)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("coursetemplate: read manifest: %w", err)
	}
	var m Manifest
	if err := jsonUnmarshalStrict(raw, &m); err != nil {
		return nil, fmt.Errorf("coursetemplate: parse manifest %s: %w", manifestPath, err)
	}
	if m.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("coursetemplate: manifest schema %q != supported %q (file %s)", m.SchemaVersion, SchemaVersion, manifestPath)
	}
	templatePath := filepath.Join(dir, m.TemplateFile)
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("coursetemplate: read template: %w", err)
	}
	// Canonical-form hash: normalize CRLF->LF so the pinned hash is stable
	// across git autocrlf checkout modes (generator emits LF; a Windows
	// checkout may rewrite line endings on disk).
	sum := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	got := hex.EncodeToString(sum[:])
	if m.TemplateSHA256 != "" && got != m.TemplateSHA256 {
		return nil, fmt.Errorf("coursetemplate: template sha256 mismatch: manifest %s != file %s (%s)", m.TemplateSHA256, got, templatePath)
	}
	edit, err := parseEdit(data)
	if err != nil {
		return nil, fmt.Errorf("coursetemplate: parse template %s: %w", templatePath, err)
	}
	return &Loaded{Dir: dir, Manifest: m, TemplatePath: templatePath, TemplateSHA: got, Edit: edit}, nil
}

func parseEdit(data []byte) (EditDoc, error) {
	var doc xmlEdit
	if err := xml.Unmarshal(data, &doc); err != nil {
		return EditDoc{}, err
	}
	out := EditDoc{
		ProjectUUID:  doc.ProjectUUID,
		KeySignature: doc.KeySignature,
	}
	var err error
	if out.Numerator, err = strconv.Atoi(doc.TempoSequence.TimeSig.Numerator); err != nil {
		return EditDoc{}, fmt.Errorf("timesig numerator: %w", err)
	}
	if out.Denominator, err = strconv.Atoi(doc.TempoSequence.TimeSig.Denominator); err != nil {
		return EditDoc{}, fmt.Errorf("timesig denominator: %w", err)
	}
	if out.TempoBPM, err = strconv.ParseFloat(doc.TempoSequence.Tempo.BPM, 64); err != nil {
		return EditDoc{}, fmt.Errorf("tempo bpm: %w", err)
	}
	for _, tr := range doc.Tracks {
		td := TrackDoc{Name: tr.Name, ID: tr.ID}
		for _, p := range tr.Plugins {
			if p.Type == "vst" && p.State != "" {
				td.HasVST = true
				td.VSTName = p.Name
				td.VSTUID = p.UID
				td.VSTState = p.State
				s := sha256.Sum256([]byte(p.State))
				td.StateSHA = hex.EncodeToString(s[:])
			}
		}
		for _, c := range tr.Clips {
			cd := ClipDoc{Name: c.Name, ChannelNumber: c.Seq.ChannelNumber}
			// SRC-PRESET-AGENT-1 leg D (382 handed-over fix): the tracktion
			// clear-XML MIDICLIP start/length attributes are TimePosition
			// SECONDS read verbatim by the loader (tracktion_Clip.cpp
			// CachedValue<TimePosition>; 382 forensics verdict
			// template_xml_unit_mismatch). Convert to beats with the edit's
			// own tempo so LengthBeats keeps its beat semantics.
			lengthSeconds, lengthErr := strconv.ParseFloat(c.Length, 64)
			if lengthErr != nil {
				return EditDoc{}, fmt.Errorf("clip %s length: %w", c.Name, lengthErr)
			}
			if out.TempoBPM <= 0 {
				return EditDoc{}, fmt.Errorf("clip %s: tempo bpm %v cannot convert seconds to beats", c.Name, out.TempoBPM)
			}
			cd.LengthBeats = lengthSeconds * out.TempoBPM / 60.0
			for _, n := range c.Seq.Notes {
				nd := NoteDoc{NoteID: n.NoteID}
				if nd.Pitch, err = strconv.Atoi(n.Pitch); err != nil {
					return EditDoc{}, fmt.Errorf("note pitch: %w", err)
				}
				if nd.StartBeat, err = strconv.ParseFloat(n.Beat, 64); err != nil {
					return EditDoc{}, fmt.Errorf("note beat: %w", err)
				}
				if nd.LengthBeat, err = strconv.ParseFloat(n.Length, 64); err != nil {
					return EditDoc{}, fmt.Errorf("note length: %w", err)
				}
				if nd.Velocity, err = strconv.Atoi(n.Velocity); err != nil {
					return EditDoc{}, fmt.Errorf("note velocity: %w", err)
				}
				if nd.Channel, err = strconv.Atoi(n.Channel); err != nil {
					return EditDoc{}, fmt.Errorf("note channel: %w", err)
				}
				if nd.NoteID == "" {
					return EditDoc{}, fmt.Errorf("clip %s has a note without vit_note_id", c.Name)
				}
				cd.Notes = append(cd.Notes, nd)
			}
			td.Clips = append(td.Clips, cd)
		}
		out.Tracks = append(out.Tracks, td)
	}
	return out, nil
}

// Validate asserts the template pins declared by the manifest: timing pins,
// track map (names + instrument placement), clip specs, groove note keys,
// plugin state hashes, deterministic note ids, and portability (no audio
// media references inside the EDIT).
func (l *Loaded) Validate() error {
	var errs []error
	m := l.Manifest
	e := l.Edit

	// Timing pins (W01 seam: time_signature/key are template-pinned; tempo is
	// the default the W01 compiler may override per design R5).
	num, den, err := parseTimeSignature(m.WorkspaceTiming.TimeSignature)
	if err != nil {
		errs = append(errs, fmt.Errorf("manifest time_signature: %w", err))
	} else if e.Numerator != num || e.Denominator != den {
		errs = append(errs, fmt.Errorf("timesig pin mismatch: template %d/%d != manifest %d/%d", e.Numerator, e.Denominator, num, den))
	}
	if e.TempoBPM != m.WorkspaceTiming.TempoBPM {
		errs = append(errs, fmt.Errorf("tempo pin mismatch: template %v != manifest %v", e.TempoBPM, m.WorkspaceTiming.TempoBPM))
	}
	if e.KeySignature != m.WorkspaceTiming.KeySignature {
		errs = append(errs, fmt.Errorf("key pin mismatch: template %q != manifest %q", e.KeySignature, m.WorkspaceTiming.KeySignature))
	}
	if e.ProjectUUID != m.TemplateUUID {
		errs = append(errs, fmt.Errorf("project uuid mismatch: template %q != manifest %q", e.ProjectUUID, m.TemplateUUID))
	}

	// Track map: every design track resolves to one template track by name,
	// and instrument placement matches the map.
	byName := make(map[string]*TrackDoc, len(e.Tracks))
	for i := range e.Tracks {
		byName[e.Tracks[i].Name] = &e.Tracks[i]
	}
	if len(e.Tracks) != len(m.W01Seam.TrackMap) {
		errs = append(errs, fmt.Errorf("track count mismatch: template %d != track_map %d", len(e.Tracks), len(m.W01Seam.TrackMap)))
	}
	for _, st := range m.W01Seam.TrackMap {
		td, ok := byName[st.TrackName]
		if !ok {
			errs = append(errs, fmt.Errorf("track_map: template track %q (design %s) missing", st.TrackName, st.DesignTrack))
			continue
		}
		if td.HasVST != st.Instrument {
			errs = append(errs, fmt.Errorf("track %q: instrument placement mismatch (template=%v map=%v)", st.TrackName, td.HasVST, st.Instrument))
		}
	}

	// Voices: each declared voice track exists, carries a VST with the pinned
	// state hash (program pinning), and the MIDI channel matches the clip's
	// channel declaration.
	for key, voice := range m.Sources {
		td, ok := byName[voice.TrackName]
		if !ok {
			errs = append(errs, fmt.Errorf("voice %s: track %q missing", key, voice.TrackName))
			continue
		}
		if !td.HasVST {
			errs = append(errs, fmt.Errorf("voice %s: track %q has no VST instrument", key, voice.TrackName))
			continue
		}
		var decl *PluginDecl
		if voice.Plugin != nil {
			decl = voice.Plugin
		} else if ref, ok := m.Sources[voice.PluginRef]; ok && ref.Plugin != nil {
			decl = ref.Plugin
		}
		if decl == nil {
			errs = append(errs, fmt.Errorf("voice %s: no plugin declaration (direct or ref %q)", key, voice.PluginRef))
			continue
		}
		if td.StateSHA != decl.StateSHA256 {
			errs = append(errs, fmt.Errorf("voice %s: state sha mismatch (template %s != manifest %s)", key, td.StateSHA, decl.StateSHA256))
		}
		for _, clip := range td.Clips {
			if clip.ChannelNumber != voice.MIDIChannel {
				errs = append(errs, fmt.Errorf("voice %s: clip %q channelNumber %d != voice channel %d", key, clip.Name, clip.ChannelNumber, voice.MIDIChannel))
			}
			for _, n := range clip.Notes {
				if n.Channel != voice.MIDIChannel-1 {
					errs = append(errs, fmt.Errorf("voice %s: clip %q note %s channel %d != %d", key, clip.Name, n.NoteID, n.Channel, voice.MIDIChannel-1))
				}
			}
		}
	}

	// Clip specs: alias, length, placeholder pitch, note ids unique+stable.
	seenIDs := make(map[string]string)
	for _, spec := range m.Clips {
		td, ok := byName[trackNameOf(m, spec.Track)]
		if !ok {
			errs = append(errs, fmt.Errorf("clip %s: track %s missing", spec.Alias, spec.Track))
			continue
		}
		var clip *ClipDoc
		for i := range td.Clips {
			if td.Clips[i].Name == spec.Alias {
				clip = &td.Clips[i]
				break
			}
		}
		if clip == nil {
			errs = append(errs, fmt.Errorf("clip %s: not found on track %s", spec.Alias, spec.Track))
			continue
		}
		if clip.LengthBeats != float64(spec.Beats) {
			errs = append(errs, fmt.Errorf("clip %s: length %v != %d beats", spec.Alias, clip.LengthBeats, spec.Beats))
		}
		if len(clip.Notes) == 0 {
			errs = append(errs, fmt.Errorf("clip %s: no notes", spec.Alias))
		}
		if spec.Kind == "placeholder_pitch" {
			for _, n := range clip.Notes {
				if n.Pitch != spec.PlaceholderPitch {
					errs = append(errs, fmt.Errorf("clip %s: placeholder clip has non-placeholder pitch %d at %v", spec.Alias, n.Pitch, n.StartBeat))
				}
			}
		}
		for _, n := range clip.Notes {
			if prev, dup := seenIDs[n.NoteID]; dup {
				errs = append(errs, fmt.Errorf("duplicate vit_note_id %s (clips %s and %s)", n.NoteID, prev, spec.Alias))
				continue
			}
			seenIDs[n.NoteID] = spec.Alias
		}
	}

	// Groove specs: notes use the declared GM percussion keys only.
	for _, g := range m.Grooves {
		td, ok := byName[trackNameOf(m, g.Track)]
		if !ok {
			errs = append(errs, fmt.Errorf("groove %s: track %s missing", g.ID, g.Track))
			continue
		}
		var clip *ClipDoc
		for i := range td.Clips {
			if td.Clips[i].Name == g.ClipAlias {
				clip = &td.Clips[i]
				break
			}
		}
		if clip == nil {
			errs = append(errs, fmt.Errorf("groove %s: clip %s missing", g.ID, g.ClipAlias))
			continue
		}
		if len(clip.Notes) != g.NoteCount {
			errs = append(errs, fmt.Errorf("groove %s: note count %d != manifest %d", g.ID, len(clip.Notes), g.NoteCount))
		}
		voice := m.Sources["groove_voice"]
		allowed := make(map[int]bool, len(voice.GMMapping.PercussionKeys))
		for key := range voice.GMMapping.PercussionKeys {
			k, err := strconv.Atoi(key)
			if err != nil {
				errs = append(errs, fmt.Errorf("groove %s: bad percussion key %q", g.ID, key))
				continue
			}
			allowed[k] = true
		}
		for _, n := range clip.Notes {
			if !allowed[n.Pitch] {
				errs = append(errs, fmt.Errorf("groove %s: note pitch %d not in declared GM percussion keys", g.ID, n.Pitch))
			}
			if n.StartBeat < 0 || n.StartBeat+n.LengthBeat > clip.LengthBeats+1e-9 {
				errs = append(errs, fmt.Errorf("groove %s: note %s exceeds clip window", g.ID, n.NoteID))
			}
		}
		for _, n := range clip.Notes {
			if prev, dup := seenIDs[n.NoteID]; dup {
				errs = append(errs, fmt.Errorf("duplicate vit_note_id %s (clips %s and %s)", n.NoteID, prev, g.ClipAlias))
				continue
			}
			seenIDs[n.NoteID] = g.ClipAlias
		}
	}

	// Portability: the clear-XML EDIT must not reference audio media files
	// (absolute or relative). MIDI-only templates have no path surface.
	if err := assertNoMediaReferences(l.TemplatePath); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// PrepareLessonProject copies the template file to dst (place-copy contract:
// the repo asset is never opened-and-saved in place). dst parent must exist.
func (l *Loaded) PrepareLessonProject(dst string) error {
	src, err := os.Open(l.TemplatePath)
	if err != nil {
		return fmt.Errorf("coursetemplate: open template: %w", err)
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("coursetemplate: mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("coursetemplate: create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("coursetemplate: copy to %s: %w", dst, err)
	}
	return nil
}

func trackNameOf(m Manifest, trackKey string) string {
	for _, st := range m.W01Seam.TrackMap {
		if st.DesignTrack == trackKey {
			return st.TrackName
		}
	}
	return ""
}

func parseTimeSignature(sig string) (int, int, error) {
	var num, den int
	if _, err := fmt.Sscanf(sig, "%d/%d", &num, &den); err != nil {
		return 0, 0, fmt.Errorf("parse %q: %w", sig, err)
	}
	if num <= 0 || den <= 0 {
		return 0, 0, fmt.Errorf("parse %q: non-positive parts", sig)
	}
	return num, den, nil
}
