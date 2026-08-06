package semanticeffect

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	AudioProcessorIdentityCardSchema = "audio_processor.semantic_identity_card.v1"
	CompressorIntentPlanSchema       = "semantic_effect.compressor_intent_plan.v1"
	CompressorPlanSchema             = "semantic_effect.compressor_plan.v1"
	CompressorRevisionSchema         = "semantic_effect.compressor_revision.v1"
	CompressorRejectionSchema        = "semantic_effect.compressor_materialization_rejection.v1"

	AudioProcessorIdentityCardMaxBytes = 2048
	AudioProcessorSignatureControlMax  = 5
	AudioProcessorHardBoundaryMax      = 8
)

var compressorAxes = map[string]bool{
	"activation_intensity": true,
	"transfer_severity":    true,
	"transient_timing":     true,
	"recovery_motion":      true,
	"detector_focus":       true,
	"output_normalization": true,
	"parallel_balance":     true,
	"character":            true,
}

// AudioProcessorIdentityCard is compact LLM-facing identity and boundary
// context. It deliberately contains neither parameter identities nor an
// executable mapping.
type AudioProcessorIdentityCard struct {
	SchemaVersion     string                    `json:"schema_version"`
	CardID            string                    `json:"card_id"`
	Identity          ProcessorIdentity         `json:"identity"`
	Archetype         ProcessorArchetype        `json:"archetype"`
	SignatureControls []string                  `json:"signature_controls"`
	HardBoundaries    []string                  `json:"hard_boundaries"`
	IdentityStatus    string                    `json:"identity_status"`
	TopologyEvidence  ProcessorTopologyEvidence `json:"topology_evidence"`
}

type ProcessorIdentity struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer,omitempty"`
}

type ProcessorArchetype struct {
	EffectFamily     string `json:"effect_family"`
	InteractionStyle string `json:"interaction_style"`
}

type ProcessorTopologyEvidence struct {
	Classification string  `json:"classification"`
	Confidence     float64 `json:"confidence"`
	Generation     string  `json:"generation"`
	Source         string  `json:"source"`
}

func (c AudioProcessorIdentityCard) Validate() error {
	if c.SchemaVersion != AudioProcessorIdentityCardSchema {
		return fmt.Errorf("identity card schema_version must be %s", AudioProcessorIdentityCardSchema)
	}
	if strings.TrimSpace(c.CardID) == "" || strings.TrimSpace(c.Identity.Name) == "" {
		return fmt.Errorf("identity card card_id and identity.name are required")
	}
	if c.Archetype.EffectFamily != "broadband_compressor" {
		return fmt.Errorf("identity card effect_family must be broadband_compressor")
	}
	if strings.TrimSpace(c.Archetype.InteractionStyle) == "" {
		return fmt.Errorf("identity card interaction_style is required")
	}
	if c.IdentityStatus != "identified" && c.IdentityStatus != "name_only" && c.IdentityStatus != "unknown" {
		return fmt.Errorf("identity card identity_status is invalid")
	}
	if len(c.SignatureControls) == 0 || len(c.SignatureControls) > AudioProcessorSignatureControlMax {
		return fmt.Errorf("identity card requires 1 to %d signature_controls", AudioProcessorSignatureControlMax)
	}
	if len(c.HardBoundaries) > AudioProcessorHardBoundaryMax {
		return fmt.Errorf("identity card exceeds hard boundary budget")
	}
	if strings.TrimSpace(c.TopologyEvidence.Generation) == "" || c.TopologyEvidence.Source != "live_generic_structural" {
		return fmt.Errorf("identity card requires live generic structural topology evidence")
	}
	if c.TopologyEvidence.Confidence < 0 || c.TopologyEvidence.Confidence > 1 || math.IsNaN(c.TopologyEvidence.Confidence) {
		return fmt.Errorf("identity card topology confidence must be finite in [0,1]")
	}
	encoded, _ := json.Marshal(c)
	if len(encoded) > AudioProcessorIdentityCardMaxBytes {
		return fmt.Errorf("identity card exceeds %d-byte context budget", AudioProcessorIdentityCardMaxBytes)
	}
	for _, forbidden := range []string{"control_ref", "param_id", "normalized", "curve", "gui", "preset"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			return fmt.Errorf("identity card contains forbidden execution detail %q", forbidden)
		}
	}
	return nil
}

type CompressorIntentPlan struct {
	SchemaVersion       string                    `json:"schema_version"`
	UserGoal            string                    `json:"user_goal"`
	NegativeConstraints []string                  `json:"negative_constraints,omitempty"`
	SelectedAxes        []string                  `json:"selected_axes"`
	EvidenceRequest     CompressorEvidenceRequest `json:"evidence_request"`
	Reason              string                    `json:"reason"`
	NeedsControlBrief   bool                      `json:"needs_control_brief"`
}

type CompressorEvidenceRequest struct {
	PreferredMode string   `json:"preferred_mode"`
	FallbackMode  string   `json:"fallback_mode,omitempty"`
	Dimensions    []string `json:"dimensions"`
	Reason        string   `json:"reason"`
}

func (p CompressorIntentPlan) Validate() error {
	if p.SchemaVersion != CompressorIntentPlanSchema || strings.TrimSpace(p.UserGoal) == "" || strings.TrimSpace(p.Reason) == "" {
		return fmt.Errorf("compressor intent plan requires schema, user_goal, and reason")
	}
	if len(p.SelectedAxes) == 0 || len(p.SelectedAxes) > 4 {
		return fmt.Errorf("compressor intent plan requires 1 to 4 selected_axes")
	}
	if err := validateCompressorAxes(p.SelectedAxes); err != nil {
		return err
	}
	switch p.EvidenceRequest.PreferredMode {
	case "source_only", "paired_io", "change_delta", "not_needed":
	default:
		return fmt.Errorf("invalid preferred COM mode %q", p.EvidenceRequest.PreferredMode)
	}
	if p.EvidenceRequest.FallbackMode != "" && p.EvidenceRequest.FallbackMode != "source_only" && p.EvidenceRequest.FallbackMode != "not_needed" {
		return fmt.Errorf("invalid fallback COM mode %q", p.EvidenceRequest.FallbackMode)
	}
	if strings.TrimSpace(p.EvidenceRequest.Reason) == "" || len(p.EvidenceRequest.Dimensions) == 0 {
		return fmt.Errorf("compressor evidence request requires dimensions and reason")
	}
	if !p.NeedsControlBrief {
		return fmt.Errorf("compressor intent plan must request progressive control disclosure")
	}
	return nil
}

type CompressorPlan struct {
	SchemaVersion       string                       `json:"schema_version"`
	PlanningOnly        bool                         `json:"planning_only"`
	MutationAuthorized  bool                         `json:"mutation_authorized"`
	Target              Target                       `json:"target"`
	UserGoal            string                       `json:"user_goal"`
	NegativeConstraints []string                     `json:"negative_constraints,omitempty"`
	IdentityCardID      string                       `json:"identity_card_id"`
	TopologyGeneration  string                       `json:"topology_generation"`
	SelectedAxes        []string                     `json:"selected_axes"`
	Evidence            CompressorPlanEvidence       `json:"evidence"`
	Controls            []CompressorControlProposal  `json:"controls"`
	EvaluationContract  CompressorEvaluationContract `json:"evaluation_contract"`
	Limitations         []string                     `json:"limitations,omitempty"`
	Revision            CompressorRevision           `json:"revision"`
}

type CompressorPlanEvidence struct {
	COMProjectionID string   `json:"com_projection_id,omitempty"`
	COMMode         string   `json:"com_mode"`
	COMStatus       string   `json:"com_status"`
	EvidenceRefs    []string `json:"evidence_refs,omitempty"`
	Summary         string   `json:"summary"`
}

type CompressorControlProposal struct {
	ProposalID   string                  `json:"proposal_id"`
	Axis         string                  `json:"axis"`
	PathKey      string                  `json:"path_key"`
	Role         string                  `json:"role"`
	Target       CompressorControlTarget `json:"target"`
	Purpose      string                  `json:"purpose"`
	EvidenceRefs []string                `json:"evidence_refs,omitempty"`
	Confidence   string                  `json:"confidence"`
}

type CompressorControlTarget struct {
	ValueDB      *float64 `json:"value_db,omitempty"`
	Ratio        *float64 `json:"ratio,omitempty"`
	ValueMS      *float64 `json:"value_ms,omitempty"`
	Percent      *float64 `json:"percent,omitempty"`
	DisplayValue *float64 `json:"display_value,omitempty"`
	EnumLabel    string   `json:"enum_label,omitempty"`
}

type CompressorEvaluationContract struct {
	SchemaVersion      string                     `json:"schema_version"`
	FrozenUserGoal     string                     `json:"frozen_user_goal"`
	Axes               []CompressorAxisEvaluation `json:"axes"`
	LevelMatchRequired bool                       `json:"level_match_required"`
	SuccessPolicy      string                     `json:"success_policy"`
	NoAutoIteration    bool                       `json:"no_auto_iteration"`
}

type CompressorAxisEvaluation struct {
	Axis                string   `json:"axis"`
	DesiredDirection    string   `json:"desired_direction"`
	COMDimensions       []string `json:"com_dimensions"`
	AcceptanceCondition string   `json:"acceptance_condition"`
}

type CompressorRevision struct {
	SchemaVersion string `json:"schema_version"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"max_attempts"`
	RejectionID   string `json:"rejection_id,omitempty"`
}

func (p CompressorPlan) Validate() error {
	if p.SchemaVersion != CompressorPlanSchema || !p.PlanningOnly || p.MutationAuthorized {
		return fmt.Errorf("compressor plan must be planning_only with mutation_authorized=false")
	}
	if strings.TrimSpace(p.Target.TrackID) == "" || strings.TrimSpace(p.Target.PluginID) == "" || strings.TrimSpace(p.UserGoal) == "" {
		return fmt.Errorf("compressor plan requires exact target and user_goal")
	}
	if strings.TrimSpace(p.IdentityCardID) == "" || strings.TrimSpace(p.TopologyGeneration) == "" {
		return fmt.Errorf("compressor plan requires identity card and topology generation")
	}
	if err := validateCompressorAxes(p.SelectedAxes); err != nil {
		return err
	}
	if len(p.Controls) == 0 || len(p.Controls) > 6 {
		return fmt.Errorf("compressor plan requires 1 to 6 controls")
	}
	seen := map[string]bool{}
	selected := stringSet(p.SelectedAxes)
	for index, control := range p.Controls {
		if strings.TrimSpace(control.ProposalID) == "" || seen[control.ProposalID] {
			return fmt.Errorf("control %d requires unique proposal_id", index)
		}
		seen[control.ProposalID] = true
		if !selected[control.Axis] || strings.TrimSpace(control.PathKey) == "" || strings.TrimSpace(control.Role) == "" || strings.TrimSpace(control.Purpose) == "" {
			return fmt.Errorf("control %d requires selected axis, path_key, role, and purpose", index)
		}
		if control.Confidence != "low" && control.Confidence != "medium" && control.Confidence != "high" {
			return fmt.Errorf("control %d confidence is invalid", index)
		}
		if err := control.Target.validate(); err != nil {
			return fmt.Errorf("control %d: %w", index, err)
		}
		if (control.Role == "output_gain" || control.Role == "makeup_gain" || control.Role == "auto_makeup") && control.Axis != "output_normalization" {
			return fmt.Errorf("control %d output/makeup role may only serve output_normalization", index)
		}
		if (control.Role == "mix" || control.Role == "wet_gain" || control.Role == "dry_gain") && control.Axis != "parallel_balance" {
			return fmt.Errorf("control %d mix role may only serve parallel_balance", index)
		}
	}
	if err := p.EvaluationContract.validate(p.UserGoal, p.SelectedAxes); err != nil {
		return err
	}
	if p.Revision.SchemaVersion != CompressorRevisionSchema || p.Revision.MaxAttempts != 1 || p.Revision.Attempt < 0 || p.Revision.Attempt > 1 {
		return fmt.Errorf("compressor revision permits exactly one constrained revision")
	}
	if p.Revision.Attempt == 1 && strings.TrimSpace(p.Revision.RejectionID) == "" {
		return fmt.Errorf("revised compressor plan requires rejection_id")
	}
	return nil
}

func (t CompressorControlTarget) validate() error {
	count := 0
	for _, value := range []*float64{t.ValueDB, t.Ratio, t.ValueMS, t.Percent, t.DisplayValue} {
		if value != nil {
			count++
			if math.IsNaN(*value) || math.IsInf(*value, 0) {
				return fmt.Errorf("control target must be finite")
			}
		}
	}
	if strings.TrimSpace(t.EnumLabel) != "" {
		count++
	}
	if count != 1 {
		return fmt.Errorf("control target requires exactly one physical or enum value")
	}
	if t.Ratio != nil && *t.Ratio <= 0 || t.ValueMS != nil && *t.ValueMS < 0 || t.Percent != nil && (*t.Percent < 0 || *t.Percent > 100) {
		return fmt.Errorf("control target is outside its semantic domain")
	}
	return nil
}

func (e CompressorEvaluationContract) validate(goal string, selectedAxes []string) error {
	if e.SchemaVersion != "semantic_effect.compressor_evaluation_contract.v1" || strings.TrimSpace(e.FrozenUserGoal) != strings.TrimSpace(goal) {
		return fmt.Errorf("evaluation contract must freeze the exact user goal")
	}
	if e.SuccessPolicy != "bounded_com_change_delta_plus_user_acceptance" || !e.NoAutoIteration {
		return fmt.Errorf("evaluation contract must require bounded COM change_delta and forbid automatic iteration")
	}
	want, got := stringSet(selectedAxes), map[string]bool{}
	for index, axis := range e.Axes {
		if !want[axis.Axis] || got[axis.Axis] || strings.TrimSpace(axis.DesiredDirection) == "" || len(axis.COMDimensions) == 0 || strings.TrimSpace(axis.AcceptanceCondition) == "" {
			return fmt.Errorf("evaluation axis %d is incomplete or not selected", index)
		}
		got[axis.Axis] = true
	}
	if len(got) != len(want) {
		return fmt.Errorf("evaluation contract must cover every selected axis")
	}
	return nil
}

type CompressorMaterializationRejection struct {
	SchemaVersion         string   `json:"schema_version"`
	RejectionID           string   `json:"rejection_id"`
	Status                string   `json:"status"`
	Code                  string   `json:"code"`
	Reason                string   `json:"reason"`
	RejectedProposalIDs   []string `json:"rejected_proposal_ids"`
	ReachableAlternatives []string `json:"reachable_alternatives,omitempty"`
	RevisionAllowed       bool     `json:"revision_allowed"`
	MutationPerformed     bool     `json:"mutation_performed"`
}

func (r CompressorMaterializationRejection) Validate() error {
	if r.SchemaVersion != CompressorRejectionSchema || r.Status != "rejected" || strings.TrimSpace(r.RejectionID) == "" || strings.TrimSpace(r.Code) == "" || strings.TrimSpace(r.Reason) == "" {
		return fmt.Errorf("materialization rejection is incomplete")
	}
	if len(r.RejectedProposalIDs) == 0 || !r.RevisionAllowed || r.MutationPerformed {
		return fmt.Errorf("materialization rejection must stop mutation and permit one revision")
	}
	return nil
}

func validateCompressorAxes(axes []string) error {
	seen := map[string]bool{}
	for _, axis := range axes {
		if !compressorAxes[axis] || seen[axis] {
			return fmt.Errorf("invalid or duplicate compressor semantic axis %q", axis)
		}
		seen[axis] = true
	}
	return nil
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func SortedCompressorAxes() []string {
	out := make([]string, 0, len(compressorAxes))
	for axis := range compressorAxes {
		out = append(out, axis)
	}
	sort.Strings(out)
	return out
}
