package staticbalance

import (
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
)

const (
	CapabilityID          = "static_mix.static_balance.v0"
	ModelSchemaVersion    = "static_balance.model.v1"
	ResultSchemaVersion   = "static_balance.result.v1"
	DecisionSchemaVersion = "static_balance.decision.v1"
)

type Input struct {
	UserIntent      string
	ProjectState    map[string]any
	MixObservation  map[string]any
	MOMProjection   map[string]any
	TOMProjection   map[string]any
	ContextSnapshot map[string]any
	RequestContext  map[string]any
	ExecutionMemory map[string]any
	Style           mixstyle.MixStyle
	StyleExplicit   bool
	GeneratedAt     time.Time
}

type Style struct {
	SchemaVersion string              `json:"schema_version"`
	Kind          string              `json:"kind"`
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Explicit      bool                `json:"explicit"`
	Dimensions    mixstyle.Dimensions `json:"dimensions"`
}

type Track struct {
	TrackID           string   `json:"track_id"`
	TrackName         string   `json:"track_name,omitempty"`
	TrackType         string   `json:"track_type,omitempty"`
	Role              string   `json:"role,omitempty"`
	Function          string   `json:"function,omitempty"`
	RoleSource        string   `json:"role_source,omitempty"`
	RoleConfidence    float64  `json:"role_confidence,omitempty"`
	FaderDB           *float64 `json:"fader_db,omitempty"`
	LevelDB           *float64 `json:"level_db,omitempty"`
	LevelMetric       string   `json:"level_metric,omitempty"`
	LevelSource       string   `json:"level_source,omitempty"`
	SourceLevelDB     *float64 `json:"source_level_db,omitempty"`
	PeakDBFS          *float64 `json:"peak_dbfs,omitempty"`
	HeadroomDB        *float64 `json:"headroom_db,omitempty"`
	LevelStatus       string   `json:"level_status,omitempty"`
	LevelFreshness    string   `json:"level_freshness,omitempty"`
	LevelTapPoint     string   `json:"level_tap_point,omitempty"`
	IncludedClipIDs   []string `json:"included_clip_ids,omitempty"`
	AggregationMethod string   `json:"aggregation_method,omitempty"`
	Eligible          bool     `json:"eligible"`
	Exclusion         string   `json:"exclusion,omitempty"`
}

type Coverage struct {
	AnalyzedTrackCount       int     `json:"analyzed_track_count"`
	RawRoleKnownCount        int     `json:"raw_role_known_count"`
	RoleKnownCount           int     `json:"role_known_count"`
	RoleCandidateCount       int     `json:"role_candidate_count"`
	EligibleTrackCount       int     `json:"eligible_track_count"`
	FaderKnownCount          int     `json:"fader_known_count"`
	LevelKnownCount          int     `json:"level_known_count"`
	ProjectedLevelKnownCount int     `json:"projected_level_known_count"`
	ApproximateLevelCount    int     `json:"approximate_level_count"`
	BlockedLevelCount        int     `json:"blocked_level_count"`
	ComparableCount          int     `json:"comparable_count"`
	FunctionCount            int     `json:"function_count"`
	RoleCoverage             float64 `json:"role_coverage"`
	EffectiveLevelCoverage   float64 `json:"effective_level_coverage"`
	LevelCoverage            float64 `json:"level_coverage"`
}

type Model struct {
	SchemaVersion string                      `json:"schema_version"`
	ModelID       string                      `json:"model_id"`
	CapabilityID  string                      `json:"capability_id"`
	GeneratedAt   string                      `json:"generated_at"`
	UserIntent    string                      `json:"user_intent,omitempty"`
	ObservationID string                      `json:"observation_id,omitempty"`
	Style         Style                       `json:"mix_style"`
	Tracks        []Track                     `json:"tracks"`
	Coverage      Coverage                    `json:"coverage"`
	Readiness     capabilityruntime.Readiness `json:"readiness"`
	EvidenceRefs  []string                    `json:"evidence_refs,omitempty"`
	Limitations   []string                    `json:"limitations,omitempty"`
	MOMSummary    map[string]any              `json:"mom_summary,omitempty"`
	TOMSummary    map[string]any              `json:"tom_summary,omitempty"`
}

type Action struct {
	Operation string         `json:"operation"`
	TrackID   string         `json:"track_id"`
	TrackName string         `json:"track_name,omitempty"`
	Role      string         `json:"role,omitempty"`
	Function  string         `json:"function,omitempty"`
	DeltaDB   float64        `json:"delta_db"`
	BeforeDB  float64        `json:"before_db"`
	TargetDB  float64        `json:"target_db"`
	Reason    string         `json:"reason"`
	Evidence  map[string]any `json:"evidence,omitempty"`
}

type FunctionSummary struct {
	Function    string  `json:"function"`
	TrackCount  int     `json:"track_count"`
	MoveCount   int     `json:"move_count"`
	MeanDeltaDB float64 `json:"mean_delta_db"`
	MinDeltaDB  float64 `json:"min_delta_db"`
	MaxDeltaDB  float64 `json:"max_delta_db"`
}

type CandidatePlan struct {
	CandidatePlanID string            `json:"candidate_plan_id"`
	Strategy        string            `json:"strategy"`
	Label           string            `json:"label"`
	Recommended     bool              `json:"recommended"`
	Confidence      string            `json:"confidence"`
	Strength        float64           `json:"strength"`
	MaxAbsDeltaDB   float64           `json:"max_abs_delta_db"`
	Actions         []Action          `json:"actions"`
	FunctionSummary []FunctionSummary `json:"function_summary,omitempty"`
	Assumptions     []string          `json:"assumptions,omitempty"`
	Limitations     []string          `json:"limitations,omitempty"`
}

type Result struct {
	SchemaVersion      string                      `json:"schema_version"`
	ResultID           string                      `json:"result_id"`
	ModelID            string                      `json:"model_id"`
	CapabilityID       string                      `json:"capability_id"`
	GeneratedAt        string                      `json:"generated_at"`
	ObservationID      string                      `json:"observation_id,omitempty"`
	Style              Style                       `json:"mix_style"`
	Readiness          capabilityruntime.Readiness `json:"readiness"`
	AnalyzedTrackCount int                         `json:"analyzed_track_count"`
	EligibleTrackCount int                         `json:"eligible_track_count"`
	Candidates         []CandidatePlan             `json:"candidates,omitempty"`
	EvidenceRefs       []string                    `json:"evidence_refs,omitempty"`
	Limitations        []string                    `json:"limitations,omitempty"`
}

type Decision struct {
	SchemaVersion   string `json:"schema_version,omitempty"`
	CandidatePlanID string `json:"candidate_plan_id,omitempty"`
	Disposition     string `json:"disposition,omitempty"`
	Reasoning       string `json:"reasoning,omitempty"`
}
