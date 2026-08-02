package frequencycleanup

import (
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mom"
)

const (
	CapabilityID        = "fine_mix.frequency_cleanup.v1"
	CapabilityVersion   = "v1"
	ModelSchemaVersion  = "frequency_cleanup.model.v1"
	ResultSchemaVersion = "frequency_cleanup.result.v1"
	TreatmentPlanSchema = "frequency_cleanup.treatment_plan.v1"
	VerificationSchema  = "frequency_cleanup.verification.v1"
)

const (
	TreatmentStaticEQ           = "static_eq"
	TreatmentDeferredDynamic    = "defer_dynamic_processing"
	TreatmentDeferredSpace      = "defer_space_processing"
	TreatmentDeferredAutomation = "defer_automation"
	TreatmentArrangementSource  = "arrangement_or_source"
	TreatmentNoChange           = "no_change"
)

// Input contains CCB-owned evidence only. C1 does not read a plug-in topology
// or execute anything while building this model.
type Input struct {
	UserIntent      string
	ProjectState    map[string]any
	MixObservation  map[string]any
	MOMProjection   map[string]any
	TOMProjection   map[string]any
	ContextSnapshot map[string]any
	RequestContext  map[string]any
	ExecutionMemory map[string]any
	GeneratedAt     time.Time
}

type Readiness struct {
	Diagnosis capabilityruntime.Readiness `json:"diagnosis"`
	Planning  capabilityruntime.Readiness `json:"planning"`
	Mutation  capabilityruntime.Readiness `json:"mutation"`
}

type Coverage struct {
	ProjectTrackCount       int     `json:"project_track_count"`
	ProfileCount            int     `json:"profile_count"`
	EligibleTrackCount      int     `json:"eligible_track_count"`
	MissingTrackCount       int     `json:"missing_track_count"`
	EligibleTrackRatio      float64 `json:"eligible_track_ratio"`
	ConflictCandidateCount  int     `json:"conflict_candidate_count"`
	TonalTendencyCount      int     `json:"tonal_tendency_count"`
	TapPoint                string  `json:"tap_point"`
	FullProjectCoverage     bool    `json:"full_project_coverage"`
	SupportsStaticDiagnosis bool    `json:"supports_static_diagnosis"`
	SupportsPostFXCompare   bool    `json:"supports_post_fx_compare"`
}

type TrackProfile struct {
	TrackID          string                    `json:"track_id"`
	TrackName        string                    `json:"track_name,omitempty"`
	Role             string                    `json:"role,omitempty"`
	Status           string                    `json:"status"`
	Freshness        string                    `json:"freshness,omitempty"`
	TapPoint         string                    `json:"tap_point"`
	Bands            map[string]map[string]any `json:"bands,omitempty"`
	EvidenceRef      string                    `json:"evidence_ref,omitempty"`
	SilenceConfirmed bool                      `json:"silence_confirmed,omitempty"`
	SilenceReason    string                    `json:"silence_reason,omitempty"`
}

// DiagnosisCandidate is observable evidence that the specialist planner may
// classify. It is deliberately not an assertion of masking or an EQ target.
type DiagnosisCandidate struct {
	ID           string         `json:"candidate_id"`
	Kind         string         `json:"kind"`
	TrackIDs     []string       `json:"track_ids,omitempty"`
	Region       string         `json:"region,omitempty"`
	Confidence   string         `json:"confidence,omitempty"`
	Summary      string         `json:"summary"`
	Evidence     map[string]any `json:"evidence,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
}

type Model struct {
	SchemaVersion         string                    `json:"schema_version"`
	ModelID               string                    `json:"model_id"`
	CapabilityID          string                    `json:"capability_id"`
	GeneratedAt           string                    `json:"generated_at"`
	UserIntent            string                    `json:"user_intent,omitempty"`
	ObservationID         string                    `json:"observation_id"`
	ProjectCutRef         string                    `json:"project_cut_ref"`
	FrequencyRelationship mom.FrequencyRelationship `json:"frequency_relationship"`
	Tracks                []TrackProfile            `json:"tracks"`
	Candidates            []DiagnosisCandidate      `json:"diagnosis_candidates,omitempty"`
	Coverage              Coverage                  `json:"coverage"`
	Readiness             Readiness                 `json:"readiness"`
	EvidenceRefs          []string                  `json:"evidence_refs,omitempty"`
	Limitations           []string                  `json:"limitations,omitempty"`
}

type Result struct {
	SchemaVersion string               `json:"schema_version"`
	ResultID      string               `json:"result_id"`
	ModelID       string               `json:"model_id"`
	CapabilityID  string               `json:"capability_id"`
	ObservationID string               `json:"observation_id"`
	Coverage      Coverage             `json:"coverage"`
	Readiness     Readiness            `json:"readiness"`
	Candidates    []DiagnosisCandidate `json:"diagnosis_candidates,omitempty"`
	EvidenceRefs  []string             `json:"evidence_refs,omitempty"`
	Limitations   []string             `json:"limitations,omitempty"`
}

// TreatmentItem covers exactly one project track. Requiring an item for every
// supplied track makes no-change and deferral decisions explicit instead of
// silently treating omitted tracks as already clean.
type TreatmentItem struct {
	Order          int      `json:"order"`
	ItemID         string   `json:"item_id"`
	TrackID        string   `json:"track_id"`
	TrackName      string   `json:"track_name,omitempty"`
	Classification string   `json:"classification"`
	DiagnosisRefs  []string `json:"diagnosis_refs,omitempty"`
	ListeningGoal  string   `json:"listening_goal,omitempty"`
	Rationale      string   `json:"rationale"`
	Constraints    []string `json:"constraints,omitempty"`
	EvidenceRefs   []string `json:"evidence_refs,omitempty"`
}

type TreatmentPlan struct {
	SchemaVersion     string          `json:"schema_version"`
	PlanID            string          `json:"plan_id"`
	DiagnosisID       string          `json:"diagnosis_id"`
	ObservationID     string          `json:"observation_id"`
	ObservationScope  string          `json:"observation_scope"`
	Summary           string          `json:"summary"`
	Items             []TreatmentItem `json:"items"`
	GlobalConstraints []string        `json:"global_constraints,omitempty"`
	EvidenceRefs      []string        `json:"evidence_refs,omitempty"`
	Limitations       []string        `json:"limitations,omitempty"`
}

type Verification struct {
	SchemaVersion     string   `json:"schema_version"`
	Status            string   `json:"status"`
	Comparable        bool     `json:"comparable"`
	PostFXComparable  bool     `json:"post_fx_comparable"`
	BeforeObservation string   `json:"before_observation_id,omitempty"`
	AfterObservation  string   `json:"after_observation_id,omitempty"`
	TapPoint          string   `json:"tap_point,omitempty"`
	TrackIDs          []string `json:"track_ids,omitempty"`
	ChangedDimensions []string `json:"changed_dimensions,omitempty"`
	Reasons           []string `json:"reasons,omitempty"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	UserAcceptance    string   `json:"user_acceptance"`
}
