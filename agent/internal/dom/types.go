// Package dom implements the Dynamics Observation Model. DOM is a peer
// projection beside MOM/TIM/TOM/FXM/COM. It describes neutral source dynamics
// and, in later phases, measured behavior of governed dynamics processors.
package dom

import (
	"fmt"
	"strings"
)

const (
	Version       = "v1"
	SchemaVersion = "dom.projection.v1"
)

const (
	ModeSourceOnly  = "source_only"
	ModePairedIO    = "paired_io"
	ModeChangeDelta = "change_delta"
)

const (
	StatusReady       = "ready"
	StatusPartial     = "partial"
	StatusMissing     = "missing"
	StatusStale       = "stale"
	StatusSuspect     = "suspect"
	StatusUnsupported = "unsupported"
)

const (
	FamilyLimiter      = "limiter"
	FamilyGateExpander = "gate_expander"
	FamilyDeEsser      = "de_esser"
	FamilyTransient    = "transient_shaper"
	FamilyMultiband    = "multiband_dynamics"
)

const (
	PairedEvidenceSchema = "dad.dynamics_processor_dual_tap_evidence.v1"
	BehaviorChangeSchema = "dom.behavior_change.v1"
)

type Input struct {
	Mode           string
	ObservationID  string
	MixSessionID   string
	CreatedAt      string
	TargetRef      map[string]any
	Conditions     Conditions
	Source         SourceEvidence
	ProcessorScope *ProcessorScope
	Paired         *PairedEvidence
	Change         *ChangeInput
}

type Conditions struct {
	ProjectRevision string  `json:"project_revision,omitempty"`
	SourceRevision  string  `json:"source_revision,omitempty"`
	ClipRevision    string  `json:"clip_revision,omitempty"`
	StartSample     int64   `json:"start_sample,omitempty"`
	EndSample       int64   `json:"end_sample,omitempty"`
	StartSeconds    float64 `json:"start_seconds,omitempty"`
	EndSeconds      float64 `json:"end_seconds,omitempty"`
	SampleRate      float64 `json:"sample_rate,omitempty"`
	ChannelCount    int     `json:"channel_count,omitempty"`
	AnalyzerVersion string  `json:"analyzer_version,omitempty"`
	TapPoint        string  `json:"tap_point,omitempty"`
	WindowMS        float64 `json:"window_ms,omitempty"`
	HopMS           float64 `json:"hop_ms,omitempty"`
	RenderMode      string  `json:"render_mode,omitempty"`
	MeasurementKey  string  `json:"measurement_key,omitempty"`
}

// ComparabilityKey is a stable, deterministic identity for the measurement
// conditions that produced one source-only DOM projection. Cross-track MOM
// relationships may compare these keys before treating values as comparable.
func (c Conditions) ComparabilityKey() string {
	parts := []string{
		"tap=" + c.TapPoint,
		"project=" + c.ProjectRevision,
		"source=" + c.SourceRevision,
		"clip=" + c.ClipRevision,
		fmt.Sprintf("sample_rate=%.6g", c.SampleRate),
		fmt.Sprintf("channel_count=%d", c.ChannelCount),
		fmt.Sprintf("range=%.6g:%.6g", c.StartSeconds, c.EndSeconds),
		fmt.Sprintf("window_ms=%.6g", c.WindowMS),
		fmt.Sprintf("hop_ms=%.6g", c.HopMS),
		"analyzer=" + c.AnalyzerVersion,
		"render=" + c.RenderMode,
	}
	return strings.Join(parts, "|")
}

type SourceEvidence struct {
	SchemaVersion        string                  `json:"schema_version,omitempty"`
	Status               string                  `json:"status"`
	Freshness            string                  `json:"freshness,omitempty"`
	Duration             float64                 `json:"duration_seconds,omitempty"`
	RMSDBFS              *float64                `json:"rms_dbfs,omitempty"`
	PeakDBFS             *float64                `json:"peak_dbfs,omitempty"`
	HeadroomDB           *float64                `json:"headroom_db,omitempty"`
	CrestDB              *float64                `json:"crest_db,omitempty"`
	TimeSegments         []TimeSegment           `json:"time_segments,omitempty"`
	Bands                []BandEvidence          `json:"bands,omitempty"`
	NoiseFloor           *NoiseFloorEvidence     `json:"noise_floor,omitempty"`
	Frequency            *FrequencyEventEvidence `json:"frequency_events,omitempty"`
	Transient            *TransientEventEvidence `json:"transient_events,omitempty"`
	BandDynamicsEvidence []BandDynamicsEvidence  `json:"band_dynamics_evidence,omitempty"`
	Quality              Quality                 `json:"quality"`
	EvidenceRefs         []string                `json:"evidence_refs,omitempty"`
	SourceFeatures       []string                `json:"source_features,omitempty"`
}

// These structures carry bounded analyzer facts only. They deliberately do
// not contain samples, FFT bins, processor identity, or control semantics.
type NoiseFloorEvidence struct {
	Status       string   `json:"status"`
	EstimateDBFS *float64 `json:"estimate_dbfs,omitempty"`
	P10DBFS      *float64 `json:"p10_dbfs,omitempty"`
	P50DBFS      *float64 `json:"p50_dbfs,omitempty"`
	Method       string   `json:"method,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
	WindowCount  int      `json:"window_count,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type FrequencyEventEvidence struct {
	Status              string           `json:"status"`
	Events              []FrequencyEvent `json:"events,omitempty"`
	Coverage            float64          `json:"coverage,omitempty"`
	EventCountAvailable bool             `json:"event_count_available"`
	EvidenceRefs        []string         `json:"evidence_refs,omitempty"`
}

type FrequencyEvent struct {
	StartSeconds float64  `json:"start_seconds"`
	EndSeconds   float64  `json:"end_seconds"`
	BandID       string   `json:"band_id"`
	MinHz        float64  `json:"min_hz,omitempty"`
	MaxHz        float64  `json:"max_hz,omitempty"`
	LevelDBFS    *float64 `json:"level_dbfs,omitempty"`
	ContrastDB   *float64 `json:"contrast_db,omitempty"`
}

type TransientEventEvidence struct {
	Status       string           `json:"status"`
	Events       []TransientEvent `json:"events,omitempty"`
	Coverage     float64          `json:"coverage,omitempty"`
	EvidenceRefs []string         `json:"evidence_refs,omitempty"`
}

type TransientEvent struct {
	OnsetSeconds         float64  `json:"onset_seconds"`
	BodyEndSeconds       float64  `json:"body_end_seconds,omitempty"`
	SustainEndSeconds    float64  `json:"sustain_end_seconds,omitempty"`
	OnsetDBFS            *float64 `json:"onset_dbfs,omitempty"`
	BodyDBFS             *float64 `json:"body_dbfs,omitempty"`
	SustainDBFS          *float64 `json:"sustain_dbfs,omitempty"`
	AttackBodyContrastDB *float64 `json:"attack_body_contrast_db,omitempty"`
	SustainDecayDB       *float64 `json:"sustain_decay_db,omitempty"`
}

type BandDynamicsEvidence struct {
	ID                string        `json:"id"`
	Status            string        `json:"status,omitempty"`
	TimeDistribution  *Distribution `json:"time_distribution,omitempty"`
	CrestDistribution *Distribution `json:"crest_distribution,omitempty"`
	EvidenceRefs      []string      `json:"evidence_refs,omitempty"`
}

type TimeSegment struct {
	StartSeconds float64  `json:"start_seconds"`
	EndSeconds   float64  `json:"end_seconds"`
	RMSDBFS      *float64 `json:"rms_dbfs,omitempty"`
	PeakDBFS     *float64 `json:"peak_dbfs,omitempty"`
	CrestDB      *float64 `json:"crest_db,omitempty"`
	EnergyState  string   `json:"energy_state,omitempty"`
}

type BandEvidence struct {
	ID         string   `json:"id"`
	Status     string   `json:"status,omitempty"`
	MinHz      float64  `json:"min_hz,omitempty"`
	MaxHz      float64  `json:"max_hz,omitempty"`
	UnitEnergy *float64 `json:"unit_energy,omitempty"`
	EnergyDB   *float64 `json:"energy_db,omitempty"`
}

type Quality struct {
	Status      string  `json:"status,omitempty"`
	Nonzero     bool    `json:"nonzero"`
	Coverage    float64 `json:"coverage,omitempty"`
	NaNInfCount int     `json:"nan_inf_count,omitempty"`
}

type ProcessorScope struct {
	Family             string `json:"processor_family"`
	TrackID            string `json:"track_id,omitempty"`
	PluginInstanceID   string `json:"plugin_instance_id,omitempty"`
	TopologyGeneration string `json:"topology_generation,omitempty"`
	ProcessorStateHash string `json:"processor_state_hash,omitempty"`
	ScopeRevision      string `json:"scope_revision,omitempty"`
}

// PairedEvidence freezes the identity and evidence boundary for a future
// paired_io implementation. Raw samples, frames, and event rows stay in DAD.
type PairedEvidence struct {
	SchemaVersion string   `json:"schema_version"`
	EvidenceID    string   `json:"evidence_id"`
	EvidenceRef   string   `json:"evidence_ref"`
	Status        string   `json:"status"`
	EvidenceRefs  []string `json:"evidence_refs,omitempty"`
}

type ChangeInput struct {
	Before *Projection
	After  *Projection
}

type Projection struct {
	SchemaVersion       string               `json:"schema_version"`
	DOMVersion          string               `json:"dom_version"`
	ProjectionID        string               `json:"projection_id"`
	Mode                string               `json:"mode"`
	Status              string               `json:"status"`
	ObservationID       string               `json:"observation_id,omitempty"`
	MixSessionID        string               `json:"mix_session_id,omitempty"`
	TargetRef           map[string]any       `json:"target_ref,omitempty"`
	Conditions          Conditions           `json:"conditions"`
	ProcessorScope      *ProcessorScope      `json:"processor_scope,omitempty"`
	PeakStructure       *PeakStructure       `json:"peak_structure,omitempty"`
	ActivityStructure   *ActivityStructure   `json:"activity_structure,omitempty"`
	FrequencyTimeEvents *FrequencyTimeEvents `json:"frequency_time_events,omitempty"`
	TransientStructure  *TransientStructure  `json:"transient_structure,omitempty"`
	BandDynamics        *BandDynamics        `json:"band_dynamics,omitempty"`
	BehaviorChange      *BehaviorChange      `json:"behavior_change,omitempty"`
	DimensionReadiness  []DimensionReadiness `json:"dimension_readiness"`
	TrustQuality        TrustQuality         `json:"trust_quality"`
	EvidenceRefs        []string             `json:"evidence_refs,omitempty"`
	Limitations         []string             `json:"limitations,omitempty"`
	LLMContext          LLMContext           `json:"llm_context"`
	GeneratedAt         string               `json:"generated_at,omitempty"`
}

type Distribution struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
	Max   float64 `json:"max"`
}

type PeakStructure struct {
	Status                     string        `json:"status"`
	PeakDBFS                   *float64      `json:"peak_dbfs,omitempty"`
	HeadroomDB                 *float64      `json:"headroom_db,omitempty"`
	CrestDB                    *float64      `json:"crest_db,omitempty"`
	SegmentPeakDistribution    *Distribution `json:"segment_peak_dbfs_distribution,omitempty"`
	SegmentCrestDistribution   *Distribution `json:"segment_crest_db_distribution,omitempty"`
	AtOrAboveFullScaleSegments int           `json:"at_or_above_full_scale_segment_count"`
	TruePeakStatus             string        `json:"true_peak_status"`
	EvidenceRefs               []string      `json:"evidence_refs,omitempty"`
	Limitations                []string      `json:"limitations,omitempty"`
}

type ActivityStructure struct {
	Status              string        `json:"status"`
	SegmentCount        int           `json:"segment_count"`
	ValidSegmentCount   int           `json:"valid_segment_count"`
	ActiveSegmentCount  int           `json:"active_segment_count"`
	LowEnergyCount      int           `json:"low_energy_segment_count"`
	SilentSegmentCount  int           `json:"silent_segment_count"`
	UnknownSegmentCount int           `json:"unknown_segment_count"`
	CoverageRatio       float64       `json:"coverage_ratio,omitempty"`
	ActiveRatio         float64       `json:"active_ratio,omitempty"`
	LowOrSilentRatio    float64       `json:"low_or_silent_ratio,omitempty"`
	LowRunSeconds       *Distribution `json:"low_or_silent_run_seconds_distribution,omitempty"`
	NoiseFloorStatus    string        `json:"noise_floor_status"`
	EvidenceRefs        []string      `json:"evidence_refs,omitempty"`
	Limitations         []string      `json:"limitations,omitempty"`
}

type FrequencyTimeEvents struct {
	Status              string         `json:"status"`
	WholeWindowBands    []BandEvidence `json:"whole_window_bands,omitempty"`
	TimeLocalized       bool           `json:"time_localized"`
	EventCountAvailable bool           `json:"event_count_available"`
	EvidenceRefs        []string       `json:"evidence_refs,omitempty"`
	Limitations         []string       `json:"limitations,omitempty"`
}

type TransientStructure struct {
	Status            string        `json:"status"`
	SegmentCrest      *Distribution `json:"segment_crest_db_distribution,omitempty"`
	OnsetEventsReady  bool          `json:"onset_events_ready"`
	AttackBodyReady   bool          `json:"attack_body_contrast_ready"`
	SustainDecayReady bool          `json:"sustain_decay_ready"`
	EvidenceRefs      []string      `json:"evidence_refs,omitempty"`
	Limitations       []string      `json:"limitations,omitempty"`
}

type BandDynamics struct {
	Status            string         `json:"status"`
	Bands             []BandEvidence `json:"bands,omitempty"`
	TimeVaryingReady  bool           `json:"time_varying_ready"`
	PerBandCrestReady bool           `json:"per_band_crest_ready"`
	CrossBandReady    bool           `json:"cross_band_relation_ready"`
	EvidenceRefs      []string       `json:"evidence_refs,omitempty"`
	Limitations       []string       `json:"limitations,omitempty"`
}

type BehaviorChange struct {
	SchemaVersion      string         `json:"schema_version"`
	Status             string         `json:"status"`
	BeforeProjectionID string         `json:"before_projection_id,omitempty"`
	AfterProjectionID  string         `json:"after_projection_id,omitempty"`
	Dimensions         map[string]any `json:"dimensions,omitempty"`
	Limitations        []string       `json:"limitations,omitempty"`
}

type DimensionReadiness struct {
	Dimension   string   `json:"dimension"`
	Status      string   `json:"status"`
	Supports    []string `json:"supports,omitempty"`
	Limitations []string `json:"limitations,omitempty"`
}

type TrustQuality struct {
	OverallStatus                  string         `json:"overall_status"`
	CanSupportSourceDescription    bool           `json:"can_support_source_description"`
	CanSupportFamilySelection      bool           `json:"can_support_family_selection"`
	CanSupportBehaviorObservation  bool           `json:"can_support_behavior_observation"`
	CanSupportPostActionEvaluation bool           `json:"can_support_post_action_evaluation"`
	Coverage                       map[string]any `json:"coverage"`
	BlockedReasons                 []string       `json:"blocked_reasons,omitempty"`
	Limitations                    []string       `json:"limitations,omitempty"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}

func SupportedProcessorFamily(family string) bool {
	switch family {
	case FamilyLimiter, FamilyGateExpander, FamilyDeEsser, FamilyTransient, FamilyMultiband:
		return true
	default:
		return false
	}
}
