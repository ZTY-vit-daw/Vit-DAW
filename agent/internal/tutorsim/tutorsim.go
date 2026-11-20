// Package tutorsim implements the TUTOR-SIM-1 server-side experiment loop:
// an independent vit.tutor_sim.v0 YAML experiment, in-memory conversation
// sessions, and a trim/exact-match step state machine.
//
// This package intentionally does not depend on the frozen production design
// schema (vit.design.v1) or on the edu LevelCard schema. The experiment file
// lives next to the project at .vit_agent/tutor_sim.yaml.
package tutorsim

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// SchemaVersion is the v0 experiment schema. v0 documents stay first-class
	// inputs: they parse exactly as before (brief/materials default empty, the
	// UI renders no brief/material cards for them).
	SchemaVersion = "vit.tutor_sim.v0"
	// SchemaVersionV1 (TUTOR-SIM-4, 池序 438) extends every step with an
	// optional `brief` (teaching brief for the step) and `materials` list.
	SchemaVersionV1 = "vit.tutor_sim.v1"
	// SidecarRelPath is the experiment file location relative to the project
	// directory (the directory that contains the .vit project file).
	SidecarRelPath = ".vit_agent/tutor_sim.yaml"
)

// ErrSidecarMissing marks the "no experiment configured for this project" case.
// It is not a parse failure: callers report it as status=unavailable.
var ErrSidecarMissing = errors.New("tutor_sim: experiment sidecar missing")

// Experiment is the parsed vit.tutor_sim.v0/v1 document.
type Experiment struct {
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`
	ID            string `yaml:"id" json:"id"`
	Persona       string `yaml:"persona" json:"persona"`
	Steps         []Step `yaml:"steps" json:"steps"`
	// SourceDir is the directory of the experiment file (set by LoadFile /
	// LoadProject). Material paths declared relative are resolved against it.
	// Not part of the YAML contract.
	SourceDir string `yaml:"-" json:"-"`
}

// Material kinds. midi_clip is the v1 production-material form (a .mid file
// loaded into the project as track+clip via the existing kernel command
// face). audio is a schema slot only in this round: accepted by the parser,
// no shipped content, rendered by the UI's existing audio widget path.
const (
	MaterialKindMidiClip = "midi_clip"
	MaterialKindAudio    = "audio"
)

// Material is one teaching material declared on a step.
type Material struct {
	Kind  string `yaml:"kind" json:"kind"`
	Title string `yaml:"title" json:"title"`
	// Path is required for midi_clip (relative paths resolve against the
	// experiment file's directory). Optional for the audio schema slot.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	// Runtime summary (computed at load time by SummarizeMaterials; not part
	// of the YAML contract).
	AbsPath       string  `yaml:"-" json:"-"`
	NoteCount     int     `yaml:"-" json:"-"`
	DurationBeats float64 `yaml:"-" json:"-"`
	TempoBPM      float64 `yaml:"-" json:"-"`
	// SummaryError records why the material file could not be summarized
	// (unreadable/unparseable). The material stays declared but must not be
	// loaded; the UI surfaces the error verbatim.
	SummaryError string `yaml:"-" json:"-"`
}

// Step is one tutor step. Ask is shown to the learner; Expect is never sent to
// the LLM and is only used by the server-side exact-match verdict. Brief and
// Materials are v1 fields (default empty in v0 documents).
type Step struct {
	ID        string     `yaml:"id" json:"id"`
	Ask       string     `yaml:"ask" json:"ask"`
	Expect    string     `yaml:"expect" json:"expect"`
	Retry     string     `yaml:"retry" json:"retry"`
	Pass      string     `yaml:"pass" json:"pass"`
	Brief     string     `yaml:"brief,omitempty" json:"brief,omitempty"`
	Materials []Material `yaml:"materials,omitempty" json:"materials,omitempty"`
}

// Matches reports the server-side verdict for a learner input. Only surrounding
// whitespace is ignored; matching is otherwise exact and case-sensitive.
func (s Step) Matches(input string) bool {
	return strings.TrimSpace(input) == s.Expect
}

// Parse decodes and validates a vit.tutor_sim.v0 or vit.tutor_sim.v1
// document. Unknown fields and trailing YAML documents are rejected
// (fail-closed).
func Parse(data []byte) (*Experiment, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var exp Experiment
	if err := decoder.Decode(&exp); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("tutor_sim: empty experiment document")
		}
		return nil, fmt.Errorf("tutor_sim: parse: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return nil, errors.New("tutor_sim: multiple YAML documents are not supported")
	} else if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("tutor_sim: parse trailing document: %w", err)
	}
	exp.Normalize()
	if err := exp.Validate(); err != nil {
		return nil, err
	}
	return &exp, nil
}

// LoadFile reads and parses an experiment file. os.ErrNotExist is preserved in
// the error chain so callers can distinguish a missing sidecar. The returned
// experiment records the file directory as SourceDir and, for midi_clip
// materials, a deterministic note-count/duration summary read from the .mid
// file at load time (a summary failure is recorded on the material, it never
// fails the experiment load).
func LoadFile(path string) (*Experiment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("tutor_sim: read %s: %w", path, err)
	}
	exp, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%w (file %s)", err, path)
	}
	if abs, err := filepath.Abs(filepath.Dir(path)); err == nil {
		exp.SourceDir = abs
	} else {
		exp.SourceDir = filepath.Dir(path)
	}
	exp.SummarizeMaterials()
	return exp, nil
}

// LoadProject loads the sidecar experiment for projectPath. projectPath may be
// the .vit file or its enclosing directory. The returned sidecar path is
// populated even for ErrSidecarMissing.
func LoadProject(projectPath string) (*Experiment, string, error) {
	sidecarPath := SidecarPath(projectPath)
	if sidecarPath == "" {
		return nil, "", ErrSidecarMissing
	}
	exp, err := LoadFile(sidecarPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, sidecarPath, ErrSidecarMissing
		}
		return nil, sidecarPath, err
	}
	return exp, sidecarPath, nil
}

// SidecarPath returns <project dir>/.vit_agent/tutor_sim.yaml. Empty input
// yields an empty path.
func SidecarPath(projectPath string) string {
	dir := ProjectDir(projectPath)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, filepath.FromSlash(SidecarRelPath))
}

// ProjectDir resolves the directory that holds the sidecar. A directory input
// is used as-is; a file input uses its parent directory.
func ProjectDir(projectPath string) string {
	path := strings.TrimSpace(projectPath)
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if info, err := os.Stat(clean); err == nil && info.IsDir() {
		return clean
	}
	return filepath.Dir(clean)
}

// Normalize trims transport-level whitespace without changing internal prose.
func (e *Experiment) Normalize() {
	if e == nil {
		return
	}
	e.SchemaVersion = strings.TrimSpace(e.SchemaVersion)
	e.ID = strings.TrimSpace(e.ID)
	e.Persona = strings.TrimSpace(e.Persona)
	for i := range e.Steps {
		step := &e.Steps[i]
		step.ID = strings.TrimSpace(step.ID)
		step.Ask = strings.TrimSpace(step.Ask)
		step.Expect = strings.TrimSpace(step.Expect)
		step.Retry = strings.TrimSpace(step.Retry)
		step.Pass = strings.TrimSpace(step.Pass)
		step.Brief = strings.TrimSpace(step.Brief)
		for j := range step.Materials {
			material := &step.Materials[j]
			material.Kind = strings.TrimSpace(material.Kind)
			material.Title = strings.TrimSpace(material.Title)
			material.Path = strings.TrimSpace(material.Path)
		}
	}
}

// Validate enforces the v0/v1 contract. It is fail-closed on every required
// field so a malformed experiment can never create a partial session.
func (e *Experiment) Validate() error {
	if e == nil {
		return errors.New("tutor_sim: nil experiment")
	}
	if e.SchemaVersion != SchemaVersion && e.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("tutor_sim: schema_version must be %q or %q, got %q", SchemaVersion, SchemaVersionV1, e.SchemaVersion)
	}
	if e.ID == "" {
		return errors.New("tutor_sim: id is required")
	}
	if e.Persona == "" {
		return errors.New("tutor_sim: persona is required")
	}
	if len(e.Steps) == 0 {
		return errors.New("tutor_sim: steps must contain at least one step")
	}
	seen := map[string]bool{}
	for i, step := range e.Steps {
		if step.ID == "" {
			return fmt.Errorf("tutor_sim: steps[%d].id is required", i)
		}
		if seen[step.ID] {
			return fmt.Errorf("tutor_sim: duplicate step id %q", step.ID)
		}
		seen[step.ID] = true
		if step.Ask == "" {
			return fmt.Errorf("tutor_sim: steps[%d].ask is required", i)
		}
		if step.Expect == "" {
			return fmt.Errorf("tutor_sim: steps[%d].expect is required", i)
		}
		if step.Retry == "" {
			return fmt.Errorf("tutor_sim: steps[%d].retry is required", i)
		}
		if step.Pass == "" {
			return fmt.Errorf("tutor_sim: steps[%d].pass is required", i)
		}
		if err := validateMaterials(i, step.Materials); err != nil {
			return err
		}
	}
	return nil
}

// validateMaterials fail-closes the v1 material entries: known kind, non-empty
// title, and a path for midi_clip materials (the audio slot carries no
// shipped content this round, so its path stays optional).
func validateMaterials(stepIndex int, materials []Material) error {
	for j, material := range materials {
		switch material.Kind {
		case MaterialKindMidiClip, MaterialKindAudio:
		case "":
			return fmt.Errorf("tutor_sim: steps[%d].materials[%d].kind is required", stepIndex, j)
		default:
			return fmt.Errorf("tutor_sim: steps[%d].materials[%d].kind must be %q or %q, got %q", stepIndex, j, MaterialKindMidiClip, MaterialKindAudio, material.Kind)
		}
		if material.Title == "" {
			return fmt.Errorf("tutor_sim: steps[%d].materials[%d].title is required", stepIndex, j)
		}
		if material.Kind == MaterialKindMidiClip && material.Path == "" {
			return fmt.Errorf("tutor_sim: steps[%d].materials[%d].path is required for kind %q", stepIndex, j, MaterialKindMidiClip)
		}
	}
	return nil
}

// Status is a tutor session lifecycle state.
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

// SummarizeMaterials resolves material paths against SourceDir and, for
// midi_clip materials, parses the .mid file into a deterministic summary
// (note count, duration in beats, tempo). Any read/parse failure is recorded
// on the material as SummaryError and never fails the experiment.
func (e *Experiment) SummarizeMaterials() {
	if e == nil {
		return
	}
	for i := range e.Steps {
		for j := range e.Steps[i].Materials {
			material := &e.Steps[i].Materials[j]
			if material.Kind != MaterialKindMidiClip || material.Path == "" {
				continue
			}
			if e.SourceDir != "" && !filepath.IsAbs(material.Path) {
				material.AbsPath = filepath.Join(e.SourceDir, filepath.FromSlash(material.Path))
			} else {
				material.AbsPath = filepath.Clean(material.Path)
			}
			data, err := os.ReadFile(material.AbsPath)
			if err != nil {
				material.SummaryError = err.Error()
				continue
			}
			summary, err := SummarizeMidi(data)
			if err != nil {
				material.SummaryError = err.Error()
				continue
			}
			material.NoteCount = summary.NoteCount
			material.DurationBeats = summary.DurationBeats
			material.TempoBPM = summary.TempoBPM
		}
	}
}

// MaterialLoadState is the session-recorded result of loading one material
// into the project. Loaded=true means the kernel chain completed with the
// track/clip ids recorded; InstrumentAttached=false with a non-empty
// InstrumentError means the material is in place but the best-effort
// instrument attach was rejected by the kernel.
type MaterialLoadState struct {
	Loading            bool   `json:"loading"`
	Loaded             bool   `json:"loaded"`
	TrackID            string `json:"track_id,omitempty"`
	ClipID             string `json:"clip_id,omitempty"`
	PluginID           string `json:"plugin_id,omitempty"`
	InstrumentAttached bool   `json:"instrument_attached"`
	InstrumentError    string `json:"instrument_error,omitempty"`
	Error              string `json:"error,omitempty"`
}

// clone copies the load-state table so callers cannot mutate session state.
func cloneLoadStates(in map[string]MaterialLoadState) map[string]MaterialLoadState {
	out := make(map[string]MaterialLoadState, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

// Snapshot is a copy of a session's observable state.
type Snapshot struct {
	ConversationID string    `json:"conversation_id"`
	ExperimentID   string    `json:"experiment_id"`
	ProjectPath    string    `json:"project_path"`
	StepIndex      int       `json:"step_index"`
	StepTotal      int       `json:"step_total"`
	StepID         string    `json:"step_id"`
	Status         Status    `json:"session_status"`
	Completed      bool      `json:"completed"`
	Attempts       int       `json:"attempts"`
	Failures       int       `json:"failures"`
	Advances       int       `json:"advances"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Verdict is the server-side judgement of one submitted answer. Passed and
// Completed are produced only by the exact-match state machine; the LLM never
// supplies them. MaterialsLoaded mirrors the session's material load table so
// response payloads can annotate the current step's materials.
type Verdict struct {
	Passed            bool
	Completed         bool
	AlreadyCompleted  bool
	StepIndex         int
	StepTotal         int
	StepID            string
	AnsweredStepIndex int
	AnsweredStep      Step
	Attempts          int
	Failures          int
	MaterialsLoaded   map[string]MaterialLoadState
}

type session struct {
	conversationID string
	experiment     *Experiment
	projectPath    string
	stepIndex      int
	status         Status
	attempts       int
	failures       int
	advances       int
	loaded         map[string]MaterialLoadState
	createdAt      time.Time
	updatedAt      time.Time
}

// Store is an in-memory conversation_id -> session table. Sessions are
// intentionally not persisted: an agent restart resets all experiments.
type Store struct {
	mu       sync.Mutex
	sessions map[string]*session
}

// NewStore returns an empty in-memory session store.
func NewStore() *Store {
	return &Store{sessions: map[string]*session{}}
}

// Start creates (or restarts) the session for conversationID at step 1.
func (s *Store) Start(conversationID, projectPath string, exp *Experiment) (Snapshot, error) {
	if s == nil {
		return Snapshot{}, errors.New("tutor_sim: nil store")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return Snapshot{}, errors.New("tutor_sim: conversation_id is required")
	}
	if exp == nil || len(exp.Steps) == 0 {
		return Snapshot{}, errors.New("tutor_sim: experiment has no steps")
	}
	if err := exp.Validate(); err != nil {
		return Snapshot{}, err
	}
	// The store owns an immutable copy: later mutation of the loader's value can
	// never change a live session's asks or verdicts.
	expCopy := *exp
	expCopy.Steps = append([]Step(nil), exp.Steps...)
	for i := range expCopy.Steps {
		expCopy.Steps[i].Materials = append([]Material(nil), expCopy.Steps[i].Materials...)
	}
	now := time.Now().UTC()
	sess := &session{
		conversationID: conversationID,
		experiment:     &expCopy,
		projectPath:    strings.TrimSpace(projectPath),
		stepIndex:      1,
		status:         StatusActive,
		createdAt:      now,
		updatedAt:      now,
	}
	s.mu.Lock()
	if s.sessions == nil {
		s.sessions = map[string]*session{}
	}
	s.sessions[conversationID] = sess
	s.mu.Unlock()
	return snapshotOf(sess), nil
}

// Status returns a snapshot of an existing session.
func (s *Store) Status(conversationID string) (Snapshot, bool) {
	if s == nil {
		return Snapshot{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok {
		return Snapshot{}, false
	}
	return snapshotOf(sess), true
}

// Submit applies one learner answer. A wrong answer does not advance; a correct
// answer advances, and the last correct answer completes the experiment.
// Repeated input after completion is reported as AlreadyCompleted and changes
// nothing.
func (s *Store) Submit(conversationID, text string) (Verdict, *Experiment, bool) {
	if s == nil {
		return Verdict{}, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok || sess == nil {
		return Verdict{}, nil, false
	}
	exp := sess.experiment
	total := len(exp.Steps)
	current := exp.Steps[sess.stepIndex-1]
	verdict := Verdict{
		StepIndex:         sess.stepIndex,
		StepTotal:         total,
		StepID:            current.ID,
		AnsweredStepIndex: sess.stepIndex,
		AnsweredStep:      current,
		Attempts:          sess.attempts,
		Failures:          sess.failures,
		MaterialsLoaded:   cloneLoadStates(sess.loaded),
	}
	if sess.status == StatusCompleted {
		verdict.Completed = true
		verdict.AlreadyCompleted = true
		return verdict, exp, true
	}
	now := time.Now().UTC()
	sess.attempts++
	verdict.Attempts = sess.attempts
	if current.Matches(text) {
		sess.advances++
		verdict.Passed = true
		if sess.stepIndex >= total {
			sess.status = StatusCompleted
			verdict.Completed = true
		} else {
			sess.stepIndex++
		}
	} else {
		sess.failures++
	}
	sess.updatedAt = now
	verdict.StepIndex = sess.stepIndex
	verdict.StepID = exp.Steps[sess.stepIndex-1].ID
	verdict.Failures = sess.failures
	verdict.MaterialsLoaded = cloneLoadStates(sess.loaded)
	return verdict, exp, true
}

// CurrentMaterials returns the current step's materials and the session's
// load-state table. ok=false for an unknown conversation.
func (s *Store) CurrentMaterials(conversationID string) ([]Material, map[string]MaterialLoadState, bool) {
	if s == nil {
		return nil, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok || sess == nil {
		return nil, nil, false
	}
	materials := sess.experiment.Steps[sess.stepIndex-1].Materials
	return materials, cloneLoadStates(sess.loaded), true
}

// LoadedMaterials returns a copy of the session's material load table
// (empty map for an unknown conversation).
func (s *Store) LoadedMaterials(conversationID string) map[string]MaterialLoadState {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok || sess == nil {
		return map[string]MaterialLoadState{}
	}
	return cloneLoadStates(sess.loaded)
}

// ReserveMaterial atomically claims a material for loading. ok=true means the
// caller owns the load; ok=false means the material is already loaded, a load
// is in flight, or the title is not declared on the CURRENT step (state says
// which). Unknown conversations are ok=false with a zero state.
func (s *Store) ReserveMaterial(conversationID, title string) (MaterialLoadState, bool) {
	if s == nil {
		return MaterialLoadState{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok || sess == nil {
		return MaterialLoadState{}, false
	}
	declared := false
	for _, material := range sess.experiment.Steps[sess.stepIndex-1].Materials {
		if material.Title == title {
			declared = true
			break
		}
	}
	if !declared {
		return MaterialLoadState{}, false
	}
	state, exists := sess.loaded[title]
	if exists && (state.Loaded || state.Loading) {
		return state, false
	}
	if sess.loaded == nil {
		sess.loaded = map[string]MaterialLoadState{}
	}
	sess.loaded[title] = MaterialLoadState{Loading: true}
	return MaterialLoadState{}, true
}

// FinalizeMaterial records the outcome of a reserved load (success or
// failure). ok=false when no open reservation exists for the title; a failed
// load clears the reservation so the title can be reserved again.
func (s *Store) FinalizeMaterial(conversationID, title string, state MaterialLoadState) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[strings.TrimSpace(conversationID)]
	if !ok || sess == nil {
		return false
	}
	current, exists := sess.loaded[title]
	if !exists || !current.Loading {
		return false
	}
	state.Loading = false
	if !state.Loaded && state.Error == "" {
		// A failed load must say why; otherwise drop the reservation so the
		// title stays retryable without a stale phantom entry.
		delete(sess.loaded, title)
	} else {
		sess.loaded[title] = state
	}
	sess.updatedAt = time.Now().UTC()
	return true
}

func snapshotOf(sess *session) Snapshot {
	total := len(sess.experiment.Steps)
	return Snapshot{
		ConversationID: sess.conversationID,
		ExperimentID:   sess.experiment.ID,
		ProjectPath:    sess.projectPath,
		StepIndex:      sess.stepIndex,
		StepTotal:      total,
		StepID:         sess.experiment.Steps[sess.stepIndex-1].ID,
		Status:         sess.status,
		Completed:      sess.status == StatusCompleted,
		Attempts:       sess.attempts,
		Failures:       sess.failures,
		Advances:       sess.advances,
		CreatedAt:      sess.createdAt,
		UpdatedAt:      sess.updatedAt,
	}
}
