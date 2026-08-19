// Package dynamiccontrol owns the project-level C2 handoff contract.
// It deliberately contains no observer, processor recognizer, or executor.
package dynamiccontrol

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/processorintent"
)

const (
	CapabilityID        = "fine_mix.dynamic_control.v1"
	CapabilityVersion   = "v1"
	ReferralSchema      = "fine_mix.dynamic_control.referral.v1"
	HypothesisSchema    = "fine_mix.dynamic_control.hypothesis.v1"
	ProjectPlanSchema   = "fine_mix.dynamic_control.project_treatment.v1"
	TreatmentPlanSchema = "fine_mix.dynamic_control.treatment_plan.v1"
)

const (
	HypothesisTargeted   = "targeted"
	HypothesisNoAction   = "no_action"
	HypothesisUnresolved = "unresolved"
)

// TargetHypothesis is the model-owned C2 discovery handoff. It deliberately
// has no processor identity: the host resolves a PCA-admitted loaded instance
// only after this hypothesis has been validated.
type TargetHypothesis struct {
	SchemaVersion string                 `json:"schema_version"`
	Status        string                 `json:"status"`
	TrackID       string                 `json:"track_id,omitempty"`
	ListeningGoal string                 `json:"listening_goal,omitempty"`
	Intent        processorintent.Intent `json:"semantic_processor_intent,omitempty"`
	EvidenceRefs  []string               `json:"evidence_refs,omitempty"`
	Rationale     string                 `json:"rationale,omitempty"`
}

// ProjectTreatment is the C2 project-stage decision.  Unlike TreatmentPlan,
// it intentionally has no plugin identity: acoustic treatment is decided for
// the whole project before PCA instance resolution or plugin loading begins.
// This is the same diagnosis-to-execution separation used by B4.
type ProjectTreatment struct {
	SchemaVersion string             `json:"schema_version"`
	Status        string             `json:"status"`
	Summary       string             `json:"summary"`
	ObservationID string             `json:"observation_id"`
	Targets       []TargetHypothesis `json:"targets,omitempty"`
	EvidenceRefs  []string           `json:"evidence_refs,omitempty"`
}

func (p *ProjectTreatment) Validate(candidateFamilies map[string]map[string]bool) error {
	if p == nil || strings.TrimSpace(p.SchemaVersion) != ProjectPlanSchema {
		return fmt.Errorf("schema_version must be %s", ProjectPlanSchema)
	}
	p.Status = strings.ToLower(strings.TrimSpace(p.Status))
	p.Summary = strings.TrimSpace(p.Summary)
	p.ObservationID = strings.TrimSpace(p.ObservationID)
	if p.Summary == "" || p.ObservationID == "" {
		return fmt.Errorf("project treatment requires summary and observation_id")
	}
	switch p.Status {
	case HypothesisNoAction:
		if len(p.Targets) != 0 {
			return fmt.Errorf("no_action project treatment must not contain targets")
		}
	case HypothesisTargeted:
		if len(p.Targets) == 0 {
			return fmt.Errorf("targeted project treatment requires targets")
		}
		seen := map[string]bool{}
		for index := range p.Targets {
			target := &p.Targets[index]
			if err := target.Validate(candidateFamilies); err != nil || target.Status != HypothesisTargeted {
				if err != nil {
					return fmt.Errorf("target %d: %w", index+1, err)
				}
				return fmt.Errorf("target %d is not targeted", index+1)
			}
			key := target.TrackID + "\x00" + target.Intent.Family
			if seen[key] {
				return fmt.Errorf("duplicate dynamic target %s", key)
			}
			seen[key] = true
		}
	default:
		return fmt.Errorf("project treatment status must be targeted or no_action")
	}
	p.EvidenceRefs = uniqueSorted(p.EvidenceRefs)
	return nil
}

func (h *TargetHypothesis) Validate(candidateFamilies map[string]map[string]bool) error {
	if h == nil {
		return fmt.Errorf("hypothesis is nil")
	}
	if strings.TrimSpace(h.SchemaVersion) != HypothesisSchema {
		return fmt.Errorf("schema_version must be %s", HypothesisSchema)
	}
	h.Status = strings.ToLower(strings.TrimSpace(h.Status))
	switch h.Status {
	case HypothesisNoAction, HypothesisUnresolved:
		if h.TrackID != "" || h.ListeningGoal != "" || h.Intent.SchemaVersion != "" {
			return fmt.Errorf("%s hypothesis must not contain a target or processor intent", h.Status)
		}
		h.EvidenceRefs = uniqueSorted(h.EvidenceRefs)
		h.Rationale = strings.TrimSpace(h.Rationale)
		return nil
	case HypothesisTargeted:
		// Continue below.
	default:
		return fmt.Errorf("hypothesis status must be targeted, no_action, or unresolved")
	}
	h.TrackID = strings.TrimSpace(h.TrackID)
	h.ListeningGoal = strings.TrimSpace(h.ListeningGoal)
	h.Rationale = strings.TrimSpace(h.Rationale)
	if h.TrackID == "" || h.ListeningGoal == "" || h.Rationale == "" {
		return fmt.Errorf("targeted hypothesis requires track_id, listening_goal, and rationale")
	}
	if err := h.Intent.Validate(); err != nil || h.Intent.Status != processorintent.StatusResolved ||
		h.Intent.ControlMode != processorintent.ControlModeSemantic {
		if err != nil {
			return fmt.Errorf("semantic_processor_intent: %w", err)
		}
		return fmt.Errorf("targeted hypothesis requires a resolved semantic_loop processor intent")
	}
	families := candidateFamilies[h.TrackID]
	if !families[h.Intent.Family] {
		return fmt.Errorf("track %s has no available PCA candidate surface for family %s", h.TrackID, h.Intent.Family)
	}
	h.EvidenceRefs = uniqueSorted(h.EvidenceRefs)
	if len(h.EvidenceRefs) == 0 {
		return fmt.Errorf("targeted hypothesis requires observation evidence_refs")
	}
	return nil
}

// Referral is optional diagnostic context from a peer capability such as C1.
// It cannot select a processor family, plugin instance, or parameter target;
// C2 remains independently responsible for observation and treatment choice.
type Referral struct {
	SchemaVersion      string           `json:"schema_version"`
	SourceCapabilityID string           `json:"source_capability_id"`
	SourcePlanID       string           `json:"source_plan_id"`
	ProjectRevision    string           `json:"project_revision"`
	Targets            []ReferralTarget `json:"targets"`
	EvidenceRefs       []string         `json:"evidence_refs,omitempty"`
}

type ReferralTarget struct {
	TrackID      string   `json:"track_id"`
	Rationale    string   `json:"rationale"`
	Constraints  []string `json:"constraints,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

func (r *Referral) Validate() error {
	if r == nil {
		return fmt.Errorf("referral is nil")
	}
	if r.SchemaVersion != ReferralSchema {
		return fmt.Errorf("schema_version must be %s", ReferralSchema)
	}
	if strings.TrimSpace(r.SourceCapabilityID) == "" || strings.TrimSpace(r.SourcePlanID) == "" || strings.TrimSpace(r.ProjectRevision) == "" {
		return fmt.Errorf("source capability, plan, and project revision are required")
	}
	if len(r.Targets) == 0 {
		return fmt.Errorf("at least one referral target is required")
	}
	seen := map[string]bool{}
	for index := range r.Targets {
		target := &r.Targets[index]
		target.TrackID = strings.TrimSpace(target.TrackID)
		target.Rationale = strings.TrimSpace(target.Rationale)
		if target.TrackID == "" || target.Rationale == "" || seen[target.TrackID] {
			return fmt.Errorf("referral target %d is incomplete or duplicated", index+1)
		}
		seen[target.TrackID] = true
		target.Constraints = uniqueSorted(target.Constraints)
		target.EvidenceRefs = uniqueSorted(target.EvidenceRefs)
	}
	r.EvidenceRefs = uniqueSorted(r.EvidenceRefs)
	return nil
}

// TreatmentTarget is the frozen C2-to-leaf handoff. Exact plugin identity is
// supplied only after PCA candidate resolution; the LLM never supplies it.
type TreatmentTarget struct {
	TrackID          string   `json:"track_id"`
	PluginID         string   `json:"plugin_id"`
	ProcessorFamily  string   `json:"processor_family"`
	ListeningGoal    string   `json:"listening_goal"`
	RequiredCoverage []string `json:"required_coverage"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
}

// TreatmentPlan freezes independently observed C2 decisions before leaf
// planners and Typed Executors are invoked.
type TreatmentPlan struct {
	SchemaVersion     string            `json:"schema_version"`
	PlanID            string            `json:"plan_id"`
	ProjectCutHash    string            `json:"project_cut_hash"`
	ObservationID     string            `json:"observation_id"`
	Referral          *Referral         `json:"referral,omitempty"`
	Targets           []TreatmentTarget `json:"targets"`
	GlobalConstraints []string          `json:"global_constraints,omitempty"`
}

func (p *TreatmentPlan) Validate() error {
	if p == nil {
		return fmt.Errorf("treatment plan is nil")
	}
	if p.SchemaVersion != TreatmentPlanSchema {
		return fmt.Errorf("schema_version must be %s", TreatmentPlanSchema)
	}
	if strings.TrimSpace(p.PlanID) == "" || strings.TrimSpace(p.ProjectCutHash) == "" || strings.TrimSpace(p.ObservationID) == "" {
		return fmt.Errorf("plan id, project cut hash, and observation id are required")
	}
	if p.Referral != nil {
		if err := p.Referral.Validate(); err != nil {
			return fmt.Errorf("referral: %w", err)
		}
	}
	if len(p.Targets) == 0 {
		return fmt.Errorf("at least one dynamic target is required")
	}
	seen := map[string]bool{}
	for index := range p.Targets {
		target := &p.Targets[index]
		target.TrackID = strings.TrimSpace(target.TrackID)
		target.PluginID = strings.TrimSpace(target.PluginID)
		target.ProcessorFamily = strings.TrimSpace(target.ProcessorFamily)
		target.ListeningGoal = strings.TrimSpace(target.ListeningGoal)
		key := target.TrackID + "\x00" + target.PluginID
		if target.TrackID == "" || target.PluginID == "" || target.ProcessorFamily == "" || target.ListeningGoal == "" || seen[key] {
			return fmt.Errorf("target %d is incomplete or duplicated", index+1)
		}
		seen[key] = true
		target.RequiredCoverage = uniqueSorted(target.RequiredCoverage)
		if len(target.RequiredCoverage) == 0 {
			return fmt.Errorf("target %d requires PCA coverage", index+1)
		}
		target.EvidenceRefs = uniqueSorted(target.EvidenceRefs)
	}
	p.GlobalConstraints = uniqueSorted(p.GlobalConstraints)
	return nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
