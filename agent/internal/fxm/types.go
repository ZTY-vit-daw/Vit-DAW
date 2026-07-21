// Package fxm implements the Effects Transformation Model. FXM is a peer
// projection beside MOM/TIM/TOM/EPM: it describes the measured transformation
// caused by a plug-in chain, without turning that transformation into a
// musical judgement.
package fxm

const (
	Version       = "v0"
	SchemaVersion = "fxm.projection.v0"
)

const (
	StatusReady   = "ready"
	StatusPartial = "partial"
	StatusMissing = "missing"
	StatusStale   = "stale"
)

type Input struct {
	ObservationID string            `json:"observation_id,omitempty"`
	MixSessionID  string            `json:"mix_session_id,omitempty"`
	CreatedAt     string            `json:"created_at,omitempty"`
	TargetRef     map[string]any    `json:"target_ref,omitempty"`
	Chain         ChainIdentity     `json:"chain"`
	Conditions    MeasurementWindow `json:"conditions"`
	Baseline      Measurement       `json:"baseline"`
	Processed     Measurement       `json:"processed"`
	VPSRefs       []string          `json:"vps_refs,omitempty"`
}

type ChainIdentity struct {
	ChainHash       string           `json:"chain_hash,omitempty"`
	ProjectRevision string           `json:"project_revision,omitempty"`
	RoutingHash     string           `json:"routing_hash,omitempty"`
	Plugins         []PluginIdentity `json:"plugins,omitempty"`
}

type PluginIdentity struct {
	Order        int    `json:"order"`
	PluginID     string `json:"plugin_id,omitempty"`
	Name         string `json:"name,omitempty"`
	Format       string `json:"format,omitempty"`
	VPSID        string `json:"vps_id,omitempty"`
	CredentialID string `json:"credential_id,omitempty"`
	StateHash    string `json:"state_hash,omitempty"`
}

type MeasurementWindow struct {
	SourceRevision string   `json:"source_revision,omitempty"`
	ClipRevision   string   `json:"clip_revision,omitempty"`
	StartSeconds   float64  `json:"start_seconds,omitempty"`
	EndSeconds     float64  `json:"end_seconds,omitempty"`
	SampleRate     float64  `json:"sample_rate,omitempty"`
	ChannelCount   int      `json:"channel_count,omitempty"`
	InputLevelDBFS *float64 `json:"input_level_dbfs,omitempty"`
	MaterialRef    string   `json:"material_ref,omitempty"`
}

type Measurement struct {
	ID             string            `json:"id,omitempty"`
	Stage          string            `json:"stage,omitempty"`
	Status         string            `json:"status,omitempty"`
	SourceRevision string            `json:"source_revision,omitempty"`
	ClipRevision   string            `json:"clip_revision,omitempty"`
	RenderRevision string            `json:"render_revision,omitempty"`
	ChainStateHash string            `json:"chain_state_hash,omitempty"`
	Window         MeasurementWindow `json:"window"`
	RMSDBFS        *float64          `json:"rms_dbfs,omitempty"`
	LUFS           *float64          `json:"lufs,omitempty"`
	PeakDBFS       *float64          `json:"peak_dbfs,omitempty"`
	CrestDB        *float64          `json:"crest_db,omitempty"`
	LatencySamples *int              `json:"latency_samples,omitempty"`
	Bands          []BandMetric      `json:"bands,omitempty"`
	Quality        QualityEvidence   `json:"quality"`
	EvidenceRefs   []string          `json:"evidence_refs,omitempty"`
}

type QualityEvidence struct {
	Deterministic      bool    `json:"deterministic"`
	LatencyCompensated bool    `json:"latency_compensated"`
	Nonzero            bool    `json:"nonzero"`
	Coverage           float64 `json:"coverage,omitempty"`
	NaNInfCount        int     `json:"nan_inf_count,omitempty"`
	TailCaptured       bool    `json:"tail_captured,omitempty"`
}

type BandMetric struct {
	ID       string  `json:"id"`
	MinHz    float64 `json:"min_hz"`
	MaxHz    float64 `json:"max_hz"`
	EnergyDB float64 `json:"energy_db"`
}

type Projection struct {
	SchemaVersion string            `json:"schema_version"`
	FXMVersion    string            `json:"fxm_version"`
	ProjectionID  string            `json:"projection_id"`
	ObservationID string            `json:"observation_id,omitempty"`
	MixSessionID  string            `json:"mix_session_id,omitempty"`
	Status        string            `json:"status"`
	TargetRef     map[string]any    `json:"target_ref,omitempty"`
	Chain         ChainIdentity     `json:"chain"`
	Conditions    MeasurementWindow `json:"conditions"`
	BaselineRef   MeasurementRef    `json:"baseline_ref"`
	ProcessedRef  MeasurementRef    `json:"processed_ref"`
	EffectDelta   EffectDelta       `json:"effect_delta"`
	TrustQuality  TrustQuality      `json:"trust_quality"`
	EvidenceRefs  []string          `json:"evidence_refs,omitempty"`
	Limitations   []string          `json:"limitations,omitempty"`
	LLMContext    LLMContext        `json:"llm_context"`
	GeneratedAt   string            `json:"generated_at,omitempty"`
}

type MeasurementRef struct {
	ID             string `json:"id,omitempty"`
	Stage          string `json:"stage,omitempty"`
	RenderRevision string `json:"render_revision,omitempty"`
	ChainStateHash string `json:"chain_state_hash,omitempty"`
}

type EffectDelta struct {
	RMSDB   *float64    `json:"rms_delta_db,omitempty"`
	LUFSDB  *float64    `json:"lufs_delta_db,omitempty"`
	PeakDB  *float64    `json:"peak_delta_db,omitempty"`
	CrestDB *float64    `json:"crest_delta_db,omitempty"`
	Latency *int        `json:"latency_delta_samples,omitempty"`
	Bands   []BandDelta `json:"spectral_delta,omitempty"`
}

type BandDelta struct {
	ID      string  `json:"id"`
	MinHz   float64 `json:"min_hz"`
	MaxHz   float64 `json:"max_hz"`
	DeltaDB float64 `json:"delta_db"`
}

type TrustQuality struct {
	OverallStatus             string   `json:"overall_status"`
	SameSource                bool     `json:"same_source"`
	SameWindow                bool     `json:"same_window"`
	SameFormat                bool     `json:"same_format"`
	Deterministic             bool     `json:"deterministic"`
	LatencyCompensated        bool     `json:"latency_compensated"`
	CanSupportObservation     bool     `json:"can_support_observation"`
	CanSupportSuggestion      bool     `json:"can_support_suggestion"`
	CanSupportActionPreflight bool     `json:"can_support_action_preflight"`
	BlockedReasons            []string `json:"blocked_reasons,omitempty"`
	Limitations               []string `json:"limitations,omitempty"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts,omitempty"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}
