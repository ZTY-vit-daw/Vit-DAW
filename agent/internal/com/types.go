// Package com implements the Compression Observation Model. COM is a peer
// projection beside MOM/TIM/TOM/FXM. It describes source dynamics and, in
// later phases, measured broadband-compressor behavior without selecting or
// changing compressor controls.
package com

const (
	Version       = "v1"
	SchemaVersion = "com.projection.v1"
)

const (
	ModeSourceOnly  = "source_only"
	ModePairedIO    = "paired_io"
	ModeChangeDelta = "change_delta"
)

const (
	StatusReady         = "ready"
	StatusPartial       = "partial"
	StatusMissing       = "missing"
	StatusStale         = "stale"
	StatusSuspect       = "suspect"
	StatusApproximate   = "approximate"
	StatusNotApplicable = "not_applicable"
)

const (
	Identified      = "identified"
	Bounded         = "bounded"
	NotIdentifiable = "not_identifiable"
	NotApplicable   = "not_applicable"
)

const (
	DimensionSourceDynamics    = "source_dynamics"
	DimensionGainAction        = "gain_action"
	DimensionTransientResponse = "transient_response"
	DimensionRecoveryMotion    = "recovery_motion"
	DimensionLevelEffect       = "level_effect"
	DimensionStereoBehavior    = "stereo_behavior"
	DimensionTriggerRelation   = "trigger_relation"
	DimensionBehaviorChange    = "behavior_change"
)

const (
	ScaleWholeWindow     = "whole_window"
	ScaleMacroProgram    = "macro_program"
	ScaleMicroTransient  = "micro_transient"
	ScaleShortGainMotion = "short_gain_motion"
	ScaleEventRecovery   = "event_recovery"
)

type Input struct {
	Mode           string
	ObservationID  string
	MixSessionID   string
	CreatedAt      string
	TargetRef      map[string]any
	ProcessorScope ProcessorScope
	Conditions     Conditions
	Source         SourceEvidence
	Paired         *PairedEvidenceArtifact
	Derivation     PairedDerivationConstraints
	Change         *ChangeDeltaInput
}

type ProcessorScope struct {
	TrackID            string `json:"track_id,omitempty"`
	ChannelID          string `json:"channel_id,omitempty"`
	PluginInstanceID   string `json:"plugin_instance_id,omitempty"`
	PluginPosition     string `json:"plugin_position,omitempty"`
	TopologyClass      string `json:"topology_class,omitempty"`
	TopologyGeneration string `json:"topology_generation,omitempty"`
	ChainHash          string `json:"chain_hash,omitempty"`
	ProcessorStateHash string `json:"processor_state_hash,omitempty"`
	ScopeRevision      string `json:"scope_revision,omitempty"`
	SupportClass       string `json:"support_class,omitempty"`
}

type Conditions struct {
	ProjectCutRef       string               `json:"project_cut_ref,omitempty"`
	SourceRevision      string               `json:"source_revision,omitempty"`
	ClipRevision        string               `json:"clip_revision,omitempty"`
	RenderRevision      string               `json:"render_revision,omitempty"`
	MaterialRef         string               `json:"material_ref,omitempty"`
	StartSample         int64                `json:"start_sample,omitempty"`
	EndSample           int64                `json:"end_sample,omitempty"`
	StartSeconds        float64              `json:"start_seconds,omitempty"`
	EndSeconds          float64              `json:"end_seconds,omitempty"`
	SampleRate          float64              `json:"sample_rate,omitempty"`
	ChannelCount        int                  `json:"channel_count,omitempty"`
	ChannelLayout       string               `json:"channel_layout,omitempty"`
	RenderMode          string               `json:"render_mode,omitempty"`
	Deterministic       bool                 `json:"deterministic,omitempty"`
	InputTap            string               `json:"input_tap,omitempty"`
	OutputTap           string               `json:"output_tap,omitempty"`
	LatencySamples      int                  `json:"latency_samples,omitempty"`
	LatencyMethod       string               `json:"latency_compensation_method,omitempty"`
	TailPolicy          string               `json:"tail_policy,omitempty"`
	AnalyzerVersion     string               `json:"analyzer_version,omitempty"`
	AnalysisResolutions []AnalysisResolution `json:"analysis_resolutions,omitempty"`
}

type AnalysisResolution struct {
	Scale    string  `json:"scale"`
	WindowMS float64 `json:"window_ms,omitempty"`
	HopMS    float64 `json:"hop_ms,omitempty"`
}

type SourceEvidence struct {
	ID              string          `json:"id,omitempty"`
	SchemaVersion   string          `json:"schema_version,omitempty"`
	Status          string          `json:"status,omitempty"`
	Freshness       string          `json:"freshness,omitempty"`
	DurationSeconds float64         `json:"duration_seconds,omitempty"`
	RMSDBFS         *float64        `json:"rms_dbfs,omitempty"`
	ActiveRMSDBFS   *float64        `json:"active_rms_dbfs,omitempty"`
	PeakDBFS        *float64        `json:"peak_dbfs,omitempty"`
	CrestDB         *float64        `json:"crest_db,omitempty"`
	TimeSegments    []TimeSegment   `json:"time_segments,omitempty"`
	Quality         QualityEvidence `json:"quality"`
	EvidenceRefs    []string        `json:"evidence_refs,omitempty"`
}

type TimeSegment struct {
	StartSeconds float64  `json:"start_seconds"`
	EndSeconds   float64  `json:"end_seconds"`
	RMSDBFS      *float64 `json:"rms_dbfs,omitempty"`
	PeakDBFS     *float64 `json:"peak_dbfs,omitempty"`
	CrestDB      *float64 `json:"crest_db,omitempty"`
	EnergyState  string   `json:"energy_state,omitempty"`
}

type QualityEvidence struct {
	QualityStatus string  `json:"quality_status,omitempty"`
	Nonzero       bool    `json:"nonzero"`
	Coverage      float64 `json:"coverage,omitempty"`
	NaNInfCount   int     `json:"nan_inf_count,omitempty"`
}

type Projection struct {
	SchemaVersion     string                    `json:"schema_version"`
	COMVersion        string                    `json:"com_version"`
	ProjectionID      string                    `json:"projection_id"`
	Mode              string                    `json:"mode"`
	Status            string                    `json:"status"`
	ObservationID     string                    `json:"observation_id,omitempty"`
	MixSessionID      string                    `json:"mix_session_id,omitempty"`
	TargetRef         map[string]any            `json:"target_ref,omitempty"`
	ProcessorScope    ProcessorScope            `json:"processor_scope"`
	Conditions        Conditions                `json:"conditions"`
	EvidenceInputs    EvidenceInputs            `json:"evidence_inputs"`
	SourceDynamics    *SourceDynamics           `json:"source_dynamics,omitempty"`
	GainAction        *BehaviorProjection       `json:"gain_action,omitempty"`
	TransientResponse *BehaviorProjection       `json:"transient_response,omitempty"`
	RecoveryMotion    *BehaviorProjection       `json:"recovery_motion,omitempty"`
	LevelEffect       *BehaviorProjection       `json:"level_effect,omitempty"`
	StereoBehavior    *BehaviorProjection       `json:"stereo_behavior,omitempty"`
	TriggerRelation   *BehaviorProjection       `json:"trigger_relation,omitempty"`
	BehaviorChange    *BehaviorChangeProjection `json:"behavior_change,omitempty"`
	Identifiability   Identifiability           `json:"identifiability"`
	TrustQuality      TrustQuality              `json:"trust_quality"`
	EvidenceRefs      []string                  `json:"evidence_refs,omitempty"`
	Limitations       []string                  `json:"limitations,omitempty"`
	LLMContext        LLMContext                `json:"llm_context"`
	GeneratedAt       string                    `json:"generated_at,omitempty"`
}

type EvidenceInputs struct {
	SourceEvidenceID   string             `json:"source_evidence_id,omitempty"`
	SourceSchema       string             `json:"source_schema,omitempty"`
	SourceStatus       string             `json:"source_status"`
	EvidenceRefs       []string           `json:"evidence_refs,omitempty"`
	InputTrace         InputTraceIdentity `json:"input_trace_identity,omitempty"`
	BeforeProjectionID string             `json:"before_projection_id,omitempty"`
	AfterProjectionID  string             `json:"after_projection_id,omitempty"`
}

type InputTraceIdentity struct {
	Method         string  `json:"method,omitempty"`
	SHA256         string  `json:"sha256,omitempty"`
	FrameCount     int     `json:"frame_count,omitempty"`
	ChannelCount   int     `json:"channel_count,omitempty"`
	QuantizationDB float64 `json:"quantization_db,omitempty"`
}

type SourceDynamics struct {
	SchemaVersion     string              `json:"schema_version"`
	Status            string              `json:"status"`
	DeclaredScale     string              `json:"declared_scale"`
	AnalyzedDuration  float64             `json:"analyzed_duration_seconds,omitempty"`
	Levels            SourceLevels        `json:"levels"`
	Activity          ActivityCoverage    `json:"activity"`
	MacroDynamics     MacroDynamics       `json:"macro_dynamics"`
	Events            SourceEventSummary  `json:"events"`
	TimeScaleCoverage []TimeScaleCoverage `json:"time_scale_coverage"`
	EvidenceRefs      []string            `json:"evidence_refs,omitempty"`
	Limitations       []string            `json:"limitations,omitempty"`
}

type SourceEventSummary struct {
	EventCount            int           `json:"event_count"`
	EventDensityPerSecond *float64      `json:"event_density_per_second,omitempty"`
	InterEventIntervalMS  *Distribution `json:"inter_event_interval_ms_distribution,omitempty"`
	TransientContrastDB   *Distribution `json:"transient_contrast_db_distribution,omitempty"`
}

type SourceLevels struct {
	RMSDBFS       *float64 `json:"rms_dbfs,omitempty"`
	ActiveRMSDBFS *float64 `json:"active_rms_dbfs,omitempty"`
	PeakDBFS      *float64 `json:"peak_dbfs,omitempty"`
	CrestDB       *float64 `json:"crest_db,omitempty"`
}

type ActivityCoverage struct {
	SegmentCount        int     `json:"segment_count"`
	ValidSegmentCount   int     `json:"valid_segment_count"`
	ActiveSegmentCount  int     `json:"active_segment_count"`
	SilentSegmentCount  int     `json:"silent_segment_count"`
	UnknownSegmentCount int     `json:"unknown_segment_count"`
	AnalyzedSeconds     float64 `json:"analyzed_seconds,omitempty"`
	ActiveSeconds       float64 `json:"active_seconds,omitempty"`
	SilentSeconds       float64 `json:"silent_seconds,omitempty"`
	UnknownSeconds      float64 `json:"unknown_seconds,omitempty"`
	CoverageRatio       float64 `json:"coverage_ratio,omitempty"`
	ActiveRatio         float64 `json:"active_ratio,omitempty"`
}

type MacroDynamics struct {
	RMSDistribution   *Distribution `json:"rms_dbfs_distribution,omitempty"`
	PeakDistribution  *Distribution `json:"peak_dbfs_distribution,omitempty"`
	CrestDistribution *Distribution `json:"crest_db_distribution,omitempty"`
	RMSRangeDB        *float64      `json:"rms_range_db,omitempty"`
	RMSStdDevDB       *float64      `json:"rms_stddev_db,omitempty"`
}

type Distribution struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
	Max   float64 `json:"max"`
}

type TimeScaleCoverage struct {
	Scale        string  `json:"scale"`
	Status       string  `json:"status"`
	WindowMS     float64 `json:"window_ms,omitempty"`
	HopMS        float64 `json:"hop_ms,omitempty"`
	SegmentCount int     `json:"segment_count,omitempty"`
	Reason       string  `json:"reason,omitempty"`
}

// BehaviorProjection contains only compact derived facts. Fine envelope rows
// and event lists remain in the external evidence artifact.
type BehaviorProjection struct {
	Status       string         `json:"status"`
	Facts        map[string]any `json:"facts,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Limitations  []string       `json:"limitations,omitempty"`
}

type Identifiability struct {
	Dimensions []IdentifiabilityDimension `json:"dimensions"`
}

type IdentifiabilityDimension struct {
	Dimension      string             `json:"dimension"`
	Status         string             `json:"status"`
	Basis          string             `json:"basis"`
	Confidence     float64            `json:"confidence"`
	Resolution     ResolutionEvidence `json:"resolution"`
	Supports       []string           `json:"supports,omitempty"`
	DoesNotSupport []string           `json:"does_not_support,omitempty"`
	Limitations    []string           `json:"limitations,omitempty"`
}

type ResolutionEvidence struct {
	WindowMS     float64 `json:"window_ms,omitempty"`
	HopMS        float64 `json:"hop_ms,omitempty"`
	EventCount   int     `json:"event_count"`
	SegmentCount int     `json:"segment_count,omitempty"`
}

type TrustQuality struct {
	OverallStatus                  string         `json:"overall_status"`
	ModeGates                      []QualityGate  `json:"mode_gates"`
	Coverage                       map[string]any `json:"coverage"`
	SameSource                     bool           `json:"same_source"`
	SameWindow                     bool           `json:"same_window"`
	SameFormat                     bool           `json:"same_format"`
	TapOrderValid                  bool           `json:"tap_order_valid"`
	Deterministic                  bool           `json:"deterministic"`
	LatencyAligned                 bool           `json:"latency_aligned"`
	InputEquivalent                bool           `json:"input_equivalent"`
	CanSupportSourceDescription    bool           `json:"can_support_source_description"`
	CanSupportBehaviorObservation  bool           `json:"can_support_behavior_observation"`
	CanSupportSemanticPlanning     bool           `json:"can_support_semantic_planning"`
	CanSupportPostActionEvaluation bool           `json:"can_support_post_action_evaluation"`
	BlockedReasons                 []string       `json:"blocked_reasons,omitempty"`
	ApproximateFields              []string       `json:"approximate_fields,omitempty"`
	SuspectFields                  []string       `json:"suspect_fields,omitempty"`
	StaleFields                    []string       `json:"stale_fields,omitempty"`
	MissingFields                  []string       `json:"missing_fields,omitempty"`
	Limitations                    []string       `json:"limitations,omitempty"`
	EvidenceRefs                   []string       `json:"evidence_refs,omitempty"`
}

type QualityGate struct {
	ID       string `json:"id"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	QualitySummary         map[string]any   `json:"quality_summary,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}
