package mom

const Version = "v1.4"

const (
	IntentGeneralBandStereoObservation  = "general_band_stereo_observation"
	IntentRealtimeBandStereoObservation = "realtime_band_stereo_observation"
	IntentActionPreflightObservation    = "action_preflight_observation"
	IntentProjectMultitrackObservation  = "project_multitrack_relation_observation"
	IntentABResultObservation           = "ab_result_observation"
)

const (
	StatusReady    = "ready"
	StatusPartial  = "partial"
	StatusDeferred = "deferred"
	StatusStale    = "stale"
	StatusSuspect  = "suspect"
	StatusMissing  = "missing"
	StatusApprox   = "approximate"
)

type Input struct {
	ObservationID         string
	MixSessionID          string
	Intent                string
	CreatedAt             string
	Args                  map[string]any
	TargetRef             map[string]any
	ListenScope           map[string]any
	TimeRuler             map[string]any
	GlobalSummary         map[string]any
	EnvironmentPackage    map[string]any
	ProjectPackage        map[string]any
	MixPackage            map[string]any
	DeepPackage           map[string]any
	AcousticPackageStatus map[string]any
	SourceCapabilities    map[string]any
}

type Projection struct {
	MOMVersion         string             `json:"mom_version"`
	Intent             string             `json:"intent"`
	IntentPolicy       IntentPolicy       `json:"intent_policy"`
	ObservationID      string             `json:"observation_id,omitempty"`
	MixSessionID       string             `json:"mix_session_id,omitempty"`
	ProjectStructure   ProjectStructure   `json:"project_structure"`
	ProjectMixProfile  ProjectMixProfile  `json:"project_mix_profile"`
	MultitrackRelation MultitrackRelation `json:"multitrack_relation"`
	Layers             Layers             `json:"layers"`
	TrustQuality       TrustQuality       `json:"trust_quality"`
	LLMContext         LLMContext         `json:"llm_context"`
}

type IntentPolicy struct {
	Name           string   `json:"name"`
	RequiredLayers []string `json:"required_layers,omitempty"`
	OptionalLayers []string `json:"optional_layers,omitempty"`
	OutputContract []string `json:"output_contract,omitempty"`
}

type ProjectMixProfile struct {
	Status         string           `json:"status"`
	Freshness      string           `json:"freshness"`
	TrackCount     int              `json:"track_count"`
	ComparedTracks []map[string]any `json:"compared_tracks,omitempty"`
	LevelOverview  map[string]any   `json:"level_overview,omitempty"`
	DominantBands  []map[string]any `json:"dominant_bands,omitempty"`
	BandTendency   map[string]any   `json:"band_tendency,omitempty"`
	StereoOverview map[string]any   `json:"stereo_overview,omitempty"`
	RiskTags       []string         `json:"risk_tags,omitempty"`
	Limitations    []string         `json:"limitations,omitempty"`
	EvidenceRefs   []string         `json:"evidence_refs,omitempty"`
}

type MultitrackRelation struct {
	Status                 string           `json:"status"`
	Freshness              string           `json:"freshness"`
	TrackCount             int              `json:"track_count"`
	ComparedTracks         []map[string]any `json:"compared_tracks,omitempty"`
	MissingTracks          []map[string]any `json:"missing_tracks,omitempty"`
	LevelDistribution      map[string]any   `json:"level_distribution,omitempty"`
	BandOccupancy          []map[string]any `json:"band_occupancy,omitempty"`
	BandConflictCandidates []map[string]any `json:"band_conflict_candidates,omitempty"`
	StereoDistribution     map[string]any   `json:"stereo_distribution,omitempty"`
	PhaseRiskTracks        []map[string]any `json:"phase_risk_tracks,omitempty"`
	Limitations            []string         `json:"limitations,omitempty"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
}

type ProjectStructure struct {
	Status         string         `json:"status"`
	Freshness      string         `json:"freshness"`
	Source         string         `json:"source,omitempty"`
	ProjectID      string         `json:"project_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	TrackID        string         `json:"track_id,omitempty"`
	ClipID         string         `json:"clip_id,omitempty"`
	GUIID          string         `json:"gui_id,omitempty"`
	SourcePath     string         `json:"source_path,omitempty"`
	SourceRevision string         `json:"source_revision,omitempty"`
	ClipRevision   string         `json:"clip_revision,omitempty"`
	RenderRevision string         `json:"render_revision,omitempty"`
	DurationSec    float64        `json:"duration_seconds,omitempty"`
	SampleRate     float64        `json:"sample_rate,omitempty"`
	ChannelCount   int            `json:"channel_count,omitempty"`
	TargetRef      map[string]any `json:"target_ref,omitempty"`
	ListenScope    map[string]any `json:"listen_scope,omitempty"`
	TimeRuler      map[string]any `json:"time_ruler,omitempty"`
	EvidenceRefs   []string       `json:"evidence_refs,omitempty"`
	Limitations    []string       `json:"limitations,omitempty"`
}

type Layers struct {
	BasicEnergy            Layer `json:"basic_energy"`
	TimbreFrequency        Layer `json:"timbre_frequency"`
	SpaceStereo            Layer `json:"space_stereo"`
	TimeDynamicsStructure  Layer `json:"time_dynamics_structure"`
	MultitrackRelationship Layer `json:"multitrack_relationship"`
	ABResultComparison     Layer `json:"ab_result_comparison"`
}

type Layer struct {
	Status       string         `json:"status"`
	Freshness    string         `json:"freshness"`
	Source       string         `json:"source,omitempty"`
	Summary      string         `json:"summary,omitempty"`
	Facts        map[string]any `json:"facts,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Limitations  []string       `json:"limitations,omitempty"`
}

type TrustQuality struct {
	SchemaVersion             string         `json:"schema_version"`
	OverallStatus             string         `json:"overall_status"`
	RequiredLayers            []string       `json:"required_layers,omitempty"`
	OptionalLayers            []string       `json:"optional_layers,omitempty"`
	DeferredLayers            []string       `json:"deferred_layers,omitempty"`
	Coverage                  map[string]any `json:"coverage,omitempty"`
	QualityGates              []string       `json:"quality_gates,omitempty"`
	BlockedReasons            []string       `json:"blocked_reasons,omitempty"`
	ApproximateFields         []string       `json:"approximate_fields,omitempty"`
	SuspectFields             []string       `json:"suspect_fields,omitempty"`
	StaleFields               []string       `json:"stale_fields,omitempty"`
	MissingFields             []string       `json:"missing_fields,omitempty"`
	SourceRevisionStatus      string         `json:"source_revision_status,omitempty"`
	ClipRevisionStatus        string         `json:"clip_revision_status,omitempty"`
	RenderRevisionStatus      string         `json:"render_revision_status,omitempty"`
	CanSupportObservation     bool           `json:"can_support_observation"`
	CanSupportSuggestion      bool           `json:"can_support_suggestion"`
	CanSupportActionPreflight bool           `json:"can_support_action_preflight"`
	CanSupportABResult        bool           `json:"can_support_ab_result"`
	SourceRevision            string         `json:"source_revision,omitempty"`
	ClipRevision              string         `json:"clip_revision,omitempty"`
	RenderRevision            string         `json:"render_revision,omitempty"`
	Freshness                 string         `json:"freshness"`
	Source                    string         `json:"source,omitempty"`
	L2TapPoint                string         `json:"l2_tap_point,omitempty"`
	L2Limitations             []string       `json:"l2_limitations,omitempty"`
	Limitations               []string       `json:"limitations,omitempty"`
	EvidenceRefs              []string       `json:"evidence_refs,omitempty"`
	UpdatedAt                 string         `json:"updated_at,omitempty"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	QualitySummary         map[string]any   `json:"quality_summary,omitempty"`
	TaskPolicy             map[string]any   `json:"task_policy,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SafetyGates            []string         `json:"safety_gates,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}
