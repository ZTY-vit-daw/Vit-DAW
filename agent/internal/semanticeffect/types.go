package semanticeffect

import (
	"fmt"
	"math"
	"strings"
)

const (
	ActionSchema = "semantic_effect_action.v1"
	BatchSchema  = "semantic_effect_batch.v1"
	EQPlanSchema = "semantic_effect.eq_plan.v1"
	ActionEQEdit = "eq_edit"
)

// Batch is a project-level carrier for several exact generic-EQ leaves.  It
// does not add a second EQ language: every leaf is the same Action used by the
// ordinary Agent.  B4 uses the wrapper so one compact LLM decision and one
// project confirmation can cover all relationship targets.
type Batch struct {
	SchemaVersion string   `json:"schema_version"`
	ProjectGoal   string   `json:"project_goal"`
	Atomic        bool     `json:"atomic"`
	Actions       []Action `json:"actions"`
}

func (b Batch) Validate() error {
	if strings.TrimSpace(b.SchemaVersion) != BatchSchema {
		return fmt.Errorf("schema_version must be %s", BatchSchema)
	}
	if strings.TrimSpace(b.ProjectGoal) == "" {
		return fmt.Errorf("project_goal is required")
	}
	if !b.Atomic {
		return fmt.Errorf("batch.atomic must be true")
	}
	if len(b.Actions) == 0 {
		return fmt.Errorf("batch actions are required")
	}
	seen := map[string]bool{}
	for index, action := range b.Actions {
		if err := action.Validate(); err != nil {
			return fmt.Errorf("action %d: %w", index+1, err)
		}
		key := strings.TrimSpace(action.Target.TrackID) + "\x00" + strings.TrimSpace(action.Target.PluginID)
		if seen[key] {
			return fmt.Errorf("action %d duplicates exact target", index+1)
		}
		seen[key] = true
	}
	return nil
}

// Action is the horizontal ordinary-Agent carrier for a concrete semantic
// effect proposal. It is not a B-stage capability and has no mutation
// authority. Chat materializes it into a governed FrozenPlan only after this
// value has passed domain validation.
type Action struct {
	SchemaVersion string           `json:"schema_version"`
	ActionType    string           `json:"action_type"`
	PayloadSchema string           `json:"payload_schema"`
	Target        Target           `json:"target"`
	UserGoal      string           `json:"user_goal"`
	Constraints   []string         `json:"negative_constraints,omitempty"`
	Evidence      EvidenceDecision `json:"evidence_decision"`
	Limitations   []string         `json:"limitations,omitempty"`
	EQPlan        *EQPlan          `json:"eq_plan,omitempty"`
}

type Target struct {
	TrackID    string `json:"track_id"`
	PluginID   string `json:"plugin_id"`
	TrackName  string `json:"track_name,omitempty"`
	PluginName string `json:"plugin_name,omitempty"`
}

type EvidenceDecision struct {
	Choice        string   `json:"choice"`
	Basis         string   `json:"basis"`
	Reason        string   `json:"reason"`
	ObservationID string   `json:"observation_id,omitempty"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
}

type EQPlan struct {
	SchemaVersion string   `json:"schema_version"`
	Atomic        bool     `json:"atomic"`
	Atoms         []EQAtom `json:"atoms"`
}

type EQAtom struct {
	AtomID        string            `json:"atom_id"`
	Action        string            `json:"action"`
	Shape         string            `json:"shape,omitempty"`
	FrequencyHz   *float64          `json:"frequency_hz,omitempty"`
	GainDB        *float64          `json:"gain_db,omitempty"`
	Q             *float64          `json:"q,omitempty"`
	SlopeDBPerOct *float64          `json:"slope_db_per_oct,omitempty"`
	ControlRef    string            `json:"control_ref,omitempty"`
	OperationRef  string            `json:"operation_ref,omitempty"`
	Purpose       string            `json:"purpose"`
	FieldOrigins  map[string]string `json:"field_origins,omitempty"`
	EvidenceRefs  []string          `json:"evidence_refs,omitempty"`
	Confidence    string            `json:"confidence"`
}

func (a Action) Validate() error {
	if strings.TrimSpace(a.SchemaVersion) != ActionSchema {
		return fmt.Errorf("schema_version must be %s", ActionSchema)
	}
	if strings.TrimSpace(a.ActionType) != ActionEQEdit {
		return fmt.Errorf("unsupported action_type %q", a.ActionType)
	}
	if strings.TrimSpace(a.PayloadSchema) != EQPlanSchema {
		return fmt.Errorf("payload_schema must be %s", EQPlanSchema)
	}
	if strings.TrimSpace(a.Target.TrackID) == "" || strings.TrimSpace(a.Target.PluginID) == "" {
		return fmt.Errorf("target track_id and plugin_id are required")
	}
	if strings.TrimSpace(a.UserGoal) == "" {
		return fmt.Errorf("user_goal is required")
	}
	if err := a.Evidence.validate(); err != nil {
		return err
	}
	if a.EQPlan == nil {
		return fmt.Errorf("eq_plan is required")
	}
	return a.EQPlan.Validate()
}

func (e EvidenceDecision) validate() error {
	switch strings.TrimSpace(e.Choice) {
	case "reuse", "read", "derive", "observe", "not_needed":
	default:
		return fmt.Errorf("invalid evidence_decision.choice %q", e.Choice)
	}
	switch strings.TrimSpace(e.Basis) {
	case "user_report", "observation", "both":
	default:
		return fmt.Errorf("invalid evidence_decision.basis %q", e.Basis)
	}
	if strings.TrimSpace(e.Reason) == "" {
		return fmt.Errorf("evidence_decision.reason is required")
	}
	if (e.Basis == "observation" || e.Basis == "both") &&
		strings.TrimSpace(e.ObservationID) == "" && len(cleanStrings(e.EvidenceRefs)) == 0 {
		return fmt.Errorf("observation evidence basis requires observation_id or evidence_refs")
	}
	return nil
}

func (p EQPlan) Validate() error {
	if strings.TrimSpace(p.SchemaVersion) != EQPlanSchema {
		return fmt.Errorf("eq_plan.schema_version must be %s", EQPlanSchema)
	}
	if !p.Atomic {
		return fmt.Errorf("eq_plan.atomic must be true")
	}
	if len(p.Atoms) == 0 || len(p.Atoms) > 3 {
		return fmt.Errorf("eq_plan must contain 1 to 3 atoms")
	}
	seen := map[string]bool{}
	for index, atom := range p.Atoms {
		if err := atom.validate(index); err != nil {
			return err
		}
		id := strings.TrimSpace(atom.AtomID)
		if seen[id] {
			return fmt.Errorf("atom %d duplicates atom_id %q", index, id)
		}
		seen[id] = true
	}
	return nil
}

func (a EQAtom) validate(index int) error {
	if strings.TrimSpace(a.AtomID) == "" {
		return fmt.Errorf("atom %d atom_id is required", index)
	}
	if strings.TrimSpace(a.Purpose) == "" {
		return fmt.Errorf("atom %d purpose is required", index)
	}
	switch strings.TrimSpace(a.Confidence) {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("atom %d confidence must be low, medium, or high", index)
	}
	action := strings.TrimSpace(a.Action)
	switch action {
	case "upsert", "modify", "disable", "remove", "undo":
	default:
		return fmt.Errorf("atom %d has unsupported action %q", index, action)
	}
	shape := strings.TrimSpace(a.Shape)
	if shape != "" {
		switch shape {
		case "bell", "low_shelf", "high_shelf", "low_cut", "high_cut":
		default:
			return fmt.Errorf("atom %d has unsupported shape %q", index, shape)
		}
	}
	if action == "upsert" && (shape == "" || a.FrequencyHz == nil) {
		return fmt.Errorf("atom %d upsert requires shape and frequency_hz", index)
	}
	if (action == "modify" || action == "disable" || action == "remove") && strings.TrimSpace(a.ControlRef) == "" {
		return fmt.Errorf("atom %d %s requires control_ref", index, action)
	}
	if action == "undo" && strings.TrimSpace(a.OperationRef) == "" {
		return fmt.Errorf("atom %d undo requires operation_ref", index)
	}
	if action == "disable" || action == "remove" || action == "undo" {
		if shape != "" || a.FrequencyHz != nil || a.GainDB != nil || a.Q != nil || a.SlopeDBPerOct != nil {
			return fmt.Errorf("atom %d %s accepts only its reference", index, action)
		}
	}
	if (shape == "bell" || shape == "low_shelf" || shape == "high_shelf") && action == "upsert" && a.GainDB == nil {
		return fmt.Errorf("atom %d %s upsert requires gain_db", index, shape)
	}
	if (shape == "low_cut" || shape == "high_cut") && a.GainDB != nil {
		return fmt.Errorf("atom %d %s forbids gain_db", index, shape)
	}
	for field, value := range map[string]*float64{
		"frequency_hz": a.FrequencyHz, "gain_db": a.GainDB, "q": a.Q, "slope_db_per_oct": a.SlopeDBPerOct,
	} {
		if value == nil {
			continue
		}
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			return fmt.Errorf("atom %d %s must be finite", index, field)
		}
		if field != "gain_db" && *value <= 0 {
			return fmt.Errorf("atom %d %s must be positive", index, field)
		}
		origin := strings.TrimSpace(a.FieldOrigins[field])
		switch origin {
		case "user_fixed", "llm_selected", "context_inherited":
		default:
			return fmt.Errorf("atom %d %s requires a valid field origin", index, field)
		}
	}
	return nil
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
