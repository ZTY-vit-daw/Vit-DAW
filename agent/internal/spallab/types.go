// Package spallab owns the narrow Reference EQ conformance fixture used by
// SPAL v0.  The TDR Nova adapter remains experimental, but a product-facing
// registration flow may persist one explicitly conformed instance for the
// current project.  It remains separate from B4 diagnosis and never exposes
// raw plug-in controls to a capability layer.
package spallab

import (
	"context"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	SchemaVersion        = "vit.spal_lab.v0"
	StaticBellCapability = "static_mix.low_end_relation.v0"
	StaticBellVersion    = "v0"
)

// ProviderRecord is a lab-only, explicitly conformed Provider binding. It is
// never a global default and contains enough provenance to reject a stale or
// hand-edited plug-in profile on the next lab run.
type ProviderRecord struct {
	SchemaVersion string `json:"schema_version"`
	ID            string `json:"id"`
	// ProjectUUID scopes a product registration to the exact project that
	// supplied the observed track and plug-in instance.  Empty is retained
	// only for legacy developer-lab records; product resolution rejects it.
	ProjectUUID               string                `json:"project_uuid,omitempty"`
	LabOnly                   bool                  `json:"lab_only"`
	ProviderID                string                `json:"provider_id"`
	Instance                  spal.ProviderInstance `json:"instance"`
	PluginSkillSignature      string                `json:"plugin_skill_signature"`
	CurrentParameterSignature string                `json:"current_parameter_signature"`
	PluginProfileKey          string                `json:"plugin_profile_key"`
	PluginVersion             string                `json:"plugin_version"`
	StaticBellInvariant       StaticBellInvariant   `json:"static_bell_invariant"`
	StaticBellEvidence        []string              `json:"static_bell_evidence"`
	ConformanceEvidence       []string              `json:"conformance_evidence"`
	ObservedParameterIDs      []string              `json:"observed_parameter_ids"`
	CreatedAt                 time.Time             `json:"created_at"`
	UpdatedAt                 time.Time             `json:"updated_at"`
}

// StaticBellInvariant records the verified live filter-type state without
// inventing a vendor enum. The value is captured only after an operator has
// placed the Band in Bell mode and asserted that fact during conformance.
type StaticBellInvariant struct {
	ParameterID     string  `json:"parameter_id"`
	Value           float64 `json:"value"`
	NormalizedValue float64 `json:"normalized_value"`
	ValueText       string  `json:"value_text,omitempty"`
}

func (i StaticBellInvariant) Valid() bool {
	return strings.TrimSpace(i.ParameterID) != "" && !math.IsNaN(i.Value) && !math.IsInf(i.Value, 0) && !math.IsNaN(i.NormalizedValue) && !math.IsInf(i.NormalizedValue, 0)
}

// TDRNovaConformanceRequest is deliberately strict. Plugin Learning supplies
// the learned PluginSkillDocument, while the lab operator supplies the narrow
// assertion that a selected band is already a static Bell. SPAL never guesses
// a vendor enum value for that assertion.
type TDRNovaConformanceRequest struct {
	ProjectUUID         string                            `json:"project_uuid,omitempty"`
	TargetRef           string                            `json:"target_ref"`
	TrackID             string                            `json:"track_id"`
	PluginID            string                            `json:"plugin_id"`
	BandSlot            string                            `json:"band_slot"`
	PluginSkill         plugingrabber.PluginSkillDocument `json:"plugin_skill"`
	ObservedParamIDs    []string                          `json:"observed_param_ids"`
	ObservedParamHash   string                            `json:"observed_param_signature,omitempty"`
	StaticBellInvariant StaticBellInvariant               `json:"static_bell_invariant"`
	StaticBellConfirmed bool                              `json:"static_bell_confirmed"`
	StaticBellEvidence  []string                          `json:"static_bell_evidence"`
}

// PlanRequest is the laboratory control-plane input. The semantic instruction
// has the same vendor-neutral shape used by future capability code.
type PlanRequest struct {
	SessionID      string                                      `json:"session_id"`
	Goal           string                                      `json:"goal"`
	ProjectCut     orchestration.ProjectCut                    `json:"project_cut"`
	Instruction    spal.Instruction                            `json:"instruction"`
	PreimageReader spal.PreimageReader                         `json:"-"`
	ValidateRecord func(context.Context, ProviderRecord) error `json:"-"`
}

// RollbackPlanRequest creates a new, separately confirmable SPAL operation.
// It restores a previous frozen preimage only when the live state still equals
// the original postimage.
type RollbackPlanRequest struct {
	SessionID         string                                      `json:"session_id"`
	OriginalSessionID string                                      `json:"original_session_id"`
	Goal              string                                      `json:"goal"`
	ProjectCut        orchestration.ProjectCut                    `json:"project_cut"`
	PreimageReader    spal.PreimageReader                         `json:"-"`
	ValidateRecord    func(context.Context, ProviderRecord) error `json:"-"`
}

// PlanResult is safe to show to a human before authorization. A nil Proposal
// represents an explicit blocker such as no matching verified Provider.
type PlanResult struct {
	Outcome   orchestration.CapabilityOutcome `json:"outcome"`
	Bundle    orchestration.ContextBundle     `json:"bundle"`
	Proposal  *orchestration.Proposal         `json:"proposal,omitempty"`
	ActionSet *orchestration.ActionSet        `json:"action_set,omitempty"`
	Session   *orchestration.PlanningSession  `json:"session,omitempty"`
}

// ExecutionJournalEntry records the durable pre-execution persistence point
// required by the Coordinator. Final receipts remain in the orchestration
// store, where their complete SPAL details are persisted by the Coordinator.
type ExecutionJournalEntry struct {
	SchemaVersion  string    `json:"schema_version"`
	SessionID      string    `json:"session_id"`
	ExecutionID    string    `json:"execution_id"`
	ActionSetHash  string    `json:"action_set_hash"`
	ProjectCutHash string    `json:"project_cut_hash"`
	PreparedAt     time.Time `json:"prepared_at"`
}
