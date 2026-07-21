package panlayout

import (
	"time"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/mixstyle"
)

const (
	CapabilityID          = "static_mix.pan_layout.v0"
	ModelSchemaVersion    = "pan_layout.model.v1"
	ResultSchemaVersion   = "pan_layout.result.v1"
	DecisionSchemaVersion = "pan_layout.decision.v1"
)

type Input struct {
	UserIntent          string
	ProjectState        map[string]any
	MixObservation      map[string]any
	MOMProjection       map[string]any
	AudioAnalysisStatus map[string]any
	TOMProjection       map[string]any
	ContextSnapshot     map[string]any
	RequestContext      map[string]any
	ExecutionMemory     map[string]any
	Style               mixstyle.MixStyle
	StyleExplicit       bool
	GeneratedAt         time.Time
}

type Style struct {
	SchemaVersion string                       `json:"schema_version"`
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Version       string                       `json:"version"`
	Hash          string                       `json:"hash"`
	Explicit      bool                         `json:"explicit"`
	Dimensions    mixstyle.PanLayoutDimensions `json:"dimensions"`
}

type Track struct {
	TrackID         string   `json:"track_id"`
	TrackName       string   `json:"track_name,omitempty"`
	TrackType       string   `json:"track_type,omitempty"`
	ParentTrackID   string   `json:"parent_track_id,omitempty"`
	Role            string   `json:"role,omitempty"`
	Function        string   `json:"function,omitempty"`
	RoleSource      string   `json:"role_source,omitempty"`
	RoleConfidence  float64  `json:"role_confidence,omitempty"`
	CurrentPan      *float64 `json:"current_pan,omitempty"`
	ChannelCount    int      `json:"channel_count,omitempty"`
	SourceFormat    string   `json:"source_format,omitempty"`
	StereoBalanceDB *float64 `json:"stereo_balance_db,omitempty"`
	Correlation     *float64 `json:"correlation,omitempty"`
	PairKey         string   `json:"pair_key,omitempty"`
	CenterAnchor    bool     `json:"center_anchor"`
	Eligible        bool     `json:"eligible"`
	Exclusion       string   `json:"exclusion,omitempty"`
	RiskFlags       []string `json:"risk_flags,omitempty"`
}

type Coverage struct {
	AnalyzedTrackCount       int     `json:"analyzed_track_count"`
	RoleCandidateCount       int     `json:"role_candidate_count"`
	RoleKnownCount           int     `json:"role_known_count"`
	PanKnownCount            int     `json:"pan_known_count"`
	ChannelKnownCount        int     `json:"channel_known_count"`
	StereoEvidenceKnownCount int     `json:"stereo_evidence_known_count"`
	EligibleTrackCount       int     `json:"eligible_track_count"`
	PairEligibleTrackCount   int     `json:"pair_eligible_track_count"`
	CenterAnchorCount        int     `json:"center_anchor_count"`
	FunctionCount            int     `json:"function_count"`
	RoleCoverage             float64 `json:"role_coverage"`
	PanCoverage              float64 `json:"pan_coverage"`
	ChannelCoverage          float64 `json:"channel_coverage"`
	StereoEvidenceCoverage   float64 `json:"stereo_evidence_coverage"`
}

type LayoutSummary struct {
	LeftCount       int     `json:"left_count"`
	CenterCount     int     `json:"center_count"`
	RightCount      int     `json:"right_count"`
	MeanPan         float64 `json:"mean_pan"`
	MeanAbsPan      float64 `json:"mean_abs_pan"`
	PairGroupCount  int     `json:"pair_group_count"`
	StereoRiskCount int     `json:"stereo_risk_count"`
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
	Layout        LayoutSummary               `json:"layout_summary"`
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
	BeforePan float64        `json:"before_pan"`
	TargetPan float64        `json:"target_pan"`
	DeltaPan  float64        `json:"delta_pan"`
	Reason    string         `json:"reason"`
	Evidence  map[string]any `json:"evidence,omitempty"`
}

type GroupSummary struct {
	Role       string  `json:"role"`
	TrackCount int     `json:"track_count"`
	MoveCount  int     `json:"move_count"`
	MinPan     float64 `json:"min_pan"`
	MaxPan     float64 `json:"max_pan"`
}

type CandidatePlan struct {
	CandidatePlanID string         `json:"candidate_plan_id"`
	Strategy        string         `json:"strategy"`
	Label           string         `json:"label"`
	Recommended     bool           `json:"recommended"`
	Confidence      string         `json:"confidence"`
	Strength        float64        `json:"strength"`
	MaxAbsPan       float64        `json:"max_abs_pan"`
	Actions         []Action       `json:"actions"`
	GroupSummary    []GroupSummary `json:"group_summary,omitempty"`
	Assumptions     []string       `json:"assumptions,omitempty"`
	Limitations     []string       `json:"limitations,omitempty"`
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
