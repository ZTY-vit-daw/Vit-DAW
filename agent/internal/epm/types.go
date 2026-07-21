package epm

const (
	Version       = "v0"
	SchemaVersion = "epm.projection.v0"
)

type ImportInput struct {
	ObservationID   string
	MixSessionID    string
	CreatedAt       string
	Summary         map[string]any
	Rows            []map[string]any
	TIMProjection   map[string]any
	TOMProjection   map[string]any
	DADWaveformRows []map[string]any
}

type Projection struct {
	SchemaVersion  string                `json:"schema_version"`
	EPMVersion     string                `json:"epm_version"`
	ObservationID  string                `json:"observation_id,omitempty"`
	MixSessionID   string                `json:"mix_session_id,omitempty"`
	Status         string                `json:"status"`
	ClipCleanup    ClipCleanupProjection `json:"clip_cleanup"`
	SectionMap     SectionMapProjection  `json:"section_map"`
	DeepReasonPack DeepReasonPack        `json:"deep_reason_pack,omitempty"`
	EvidenceRefs   []string              `json:"evidence_refs,omitempty"`
	Limitations    []string              `json:"limitations,omitempty"`
	LLMContext     LLMContext            `json:"llm_context"`
	GeneratedAt    string                `json:"generated_at,omitempty"`
}

type ClipCleanupProjection struct {
	Status                  string           `json:"status"`
	TrackCount              int              `json:"track_count"`
	ClipCount               int              `json:"clip_count"`
	TrackWithTimingCount    int              `json:"track_with_timing_count"`
	ClipWithTimingCount     int              `json:"clip_with_timing_count"`
	MaxDurationSeconds      float64          `json:"max_duration_seconds,omitempty"`
	FullLengthStemDetected  bool             `json:"full_length_stem_detected"`
	StemAlignmentConfidence string           `json:"stem_alignment_confidence"`
	PreserveStemAlignment   bool             `json:"preserve_stem_alignment"`
	AlignedStartCount       int              `json:"aligned_start_count"`
	NearFullLengthCount     int              `json:"near_full_length_count"`
	AlignmentCoverage       float64          `json:"alignment_coverage,omitempty"`
	Recommendation          string           `json:"recommendation"`
	TrimRecommendation      string           `json:"trim_recommendation"`
	FadeRecommendation      string           `json:"fade_recommendation"`
	CrossfadeRecommendation string           `json:"crossfade_recommendation"`
	AutoEditAllowed         bool             `json:"auto_edit_allowed"`
	CandidateSummary        CandidateSummary `json:"candidate_summary"`
	Candidates              []EditCandidate  `json:"candidates,omitempty"`
	TechniqueNotes          []string         `json:"technique_notes,omitempty"`
}

type CandidateSummary struct {
	TrimCandidateCount      int `json:"trim_candidate_count"`
	FadeCandidateCount      int `json:"fade_candidate_count"`
	CrossfadeCandidateCount int `json:"crossfade_candidate_count"`
	ShortClipReviewCount    int `json:"short_clip_review_count"`
	SilenceReviewCount      int `json:"silence_review_count"`
	HotPeakReviewCount      int `json:"hot_peak_review_count"`
}

type EditCandidate struct {
	TrackID         string   `json:"track_id,omitempty"`
	TrackName       string   `json:"track_name,omitempty"`
	ClipID          string   `json:"clip_id,omitempty"`
	ClipName        string   `json:"clip_name,omitempty"`
	Kind            string   `json:"kind"`
	Severity        string   `json:"severity"`
	Recommendation  string   `json:"recommendation"`
	Reason          string   `json:"reason"`
	Evidence        []string `json:"evidence,omitempty"`
	RequiresConfirm bool     `json:"requires_confirmation"`
	PendingAction   *PendingActionPlan `json:"pending_action,omitempty"`
}

type PendingActionPlan struct {
	ToolName     string         `json:"tool_name"`
	TargetIDs    map[string]any `json:"target_ids,omitempty"`
	Args         map[string]any `json:"args"`
	Before       map[string]any `json:"before,omitempty"`
	After        map[string]any `json:"after,omitempty"`
	TimeRange    map[string]any `json:"time_range,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	Risk         string         `json:"risk"`
	RollbackHint string         `json:"rollback_hint,omitempty"`
}

type SectionMapProjection struct {
	Status              string             `json:"status"`
	TrackCount          int                `json:"track_count"`
	DurationSeconds     float64            `json:"duration_seconds,omitempty"`
	ReferenceStrategy   string             `json:"reference_strategy"`
	Confidence          string             `json:"confidence"`
	CandidateCount      int                `json:"candidate_count"`
	CoverageSeconds     float64            `json:"coverage_seconds,omitempty"`
	CoverageRatio       float64            `json:"coverage_ratio,omitempty"`
	EvidenceSummary     SectionEvidence    `json:"evidence_summary"`
	RecommendedInputs   []string           `json:"recommended_inputs,omitempty"`
	SupportedStrategies []SectionStrategy  `json:"supported_strategies,omitempty"`
	Sections            []SectionCandidate `json:"sections,omitempty"`
	MarkerWriteSupport  string             `json:"marker_write_support"`
	LLMReasoningMode    string             `json:"llm_reasoning_mode"`
	Notes               []string           `json:"notes,omitempty"`
}

type SectionStrategy struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	AppliesWhen string `json:"applies_when"`
	Status      string `json:"status"`
}

type SectionEvidence struct {
	TimingTrackCount        int      `json:"timing_track_count"`
	TimeSegmentTrackCount   int      `json:"time_segment_track_count"`
	TimeSegmentCount        int      `json:"time_segment_count"`
	BoundaryCandidateCount  int      `json:"boundary_candidate_count"`
	BoundarySource          string   `json:"boundary_source"`
	ActivityChangeThreshold float64  `json:"activity_change_threshold,omitempty"`
	EnergyChangeThreshold   float64  `json:"energy_change_threshold,omitempty"`
	Limitations             []string `json:"limitations,omitempty"`
}

type SectionCandidate struct {
	SectionID       string   `json:"section_id"`
	Label           string   `json:"label"`
	LabelHint       string   `json:"label_hint"`
	StartSeconds    float64  `json:"start_seconds"`
	EndSeconds      float64  `json:"end_seconds"`
	DurationSeconds float64  `json:"duration_seconds"`
	Confidence      string   `json:"confidence"`
	ConfidenceScore float64  `json:"confidence_score"`
	Basis           []string `json:"basis,omitempty"`
	RequiresConfirm bool     `json:"requires_confirmation"`
}

type DeepReasonPack struct {
	Available          bool     `json:"available"`
	DefaultMode        string   `json:"default_mode"`
	ExpandableLayers   []string `json:"expandable_layers,omitempty"`
	RequiresUserIntent bool     `json:"requires_user_intent"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}
