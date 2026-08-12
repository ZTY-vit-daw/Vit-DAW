package tim

const (
	Version       = "v0"
	SchemaVersion = "tim.projection.v0"
)

type Input struct {
	ObservationID         string
	MixSessionID          string
	CreatedAt             string
	Args                  map[string]any
	ProjectPackage        map[string]any
	AcousticPackageStatus map[string]any
	SourceCapabilities    map[string]any
	// AuthoritativeState is an internal, read-only summary of the current
	// project binding/topology and DAD completion state. It is never copied to
	// the model projection; TIM may use it only after the binding is proven.
	AuthoritativeState map[string]any
}

type Projection struct {
	SchemaVersion    string            `json:"schema_version"`
	TIMVersion       string            `json:"tim_version"`
	ObservationID    string            `json:"observation_id,omitempty"`
	MixSessionID     string            `json:"mix_session_id,omitempty"`
	Status           string            `json:"status"`
	TechnicalSummary TechnicalSummary  `json:"technical_summary"`
	Coverage         TechnicalCoverage `json:"coverage"`
	RiskSummary      RiskSummary       `json:"risk_summary"`
	Issues           []Issue           `json:"issues,omitempty"`
	TrackFacts       []TrackFact       `json:"track_facts,omitempty"`
	EvidenceRefs     []string          `json:"evidence_refs,omitempty"`
	Limitations      []string          `json:"limitations,omitempty"`
	LLMContext       LLMContext        `json:"llm_context"`
	GeneratedAt      string            `json:"generated_at,omitempty"`
}

type TechnicalSummary struct {
	TrackCount              int            `json:"track_count"`
	ClipCount               int            `json:"clip_count"`
	EmptyTrackCount         int            `json:"empty_track_count,omitempty"`
	SourcePresentCount      int            `json:"source_present_count"`
	SourceMissingCount      int            `json:"source_missing_count,omitempty"`
	SourceUnknownCount      int            `json:"source_unknown_count,omitempty"`
	PlaybackValidCount      int            `json:"playback_valid_count,omitempty"`
	PlaybackInvalidCount    int            `json:"playback_invalid_count,omitempty"`
	PlaybackUnknownCount    int            `json:"playback_unknown_count,omitempty"`
	AcousticReadyTrackCount int            `json:"acoustic_ready_track_count,omitempty"`
	AcousticMissingCount    int            `json:"acoustic_missing_count,omitempty"`
	PCMSourceCount          int            `json:"pcm_source_count,omitempty"`
	CompressedSourceCount   int            `json:"compressed_source_count,omitempty"`
	UnknownFormatCount      int            `json:"unknown_format_count,omitempty"`
	FormatFamilyCounts      map[string]int `json:"format_family_counts,omitempty"`
	SampleRateCounts        map[string]int `json:"sample_rate_counts,omitempty"`
	BitDepthCounts          map[string]int `json:"bit_depth_counts,omitempty"`
	ChannelCountCounts      map[string]int `json:"channel_count_counts,omitempty"`
}

type TechnicalCoverage struct {
	SourcePath       CoverageItem `json:"source_path"`
	PlaybackValidity CoverageItem `json:"playback_validity"`
	FormatFamily     CoverageItem `json:"format_family"`
	SampleRate       CoverageItem `json:"sample_rate"`
	BitDepth         CoverageItem `json:"bit_depth"`
	ChannelCount     CoverageItem `json:"channel_count"`
	AcousticPackage  CoverageItem `json:"acoustic_package"`
}

type CoverageItem struct {
	Status        string `json:"status"`
	KnownCount    int    `json:"known_count"`
	TotalCount    int    `json:"total_count"`
	MissingCount  int    `json:"missing_count,omitempty"`
	UnknownCount  int    `json:"unknown_count,omitempty"`
	InvalidCount  int    `json:"invalid_count,omitempty"`
	NotApplicable int    `json:"not_applicable_count,omitempty"`
}

type RiskSummary struct {
	OverallRisk  string         `json:"overall_risk"`
	IssueCount   int            `json:"issue_count"`
	BySeverity   map[string]int `json:"by_severity,omitempty"`
	PrimaryCodes []string       `json:"primary_codes,omitempty"`
}

type Issue struct {
	Code         string   `json:"code"`
	Severity     string   `json:"severity"`
	TrackID      string   `json:"track_id,omitempty"`
	TrackName    string   `json:"track_name,omitempty"`
	ClipID       string   `json:"clip_id,omitempty"`
	ClipName     string   `json:"clip_name,omitempty"`
	Detail       string   `json:"detail"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type TrackFact struct {
	TrackID              string   `json:"track_id,omitempty"`
	TrackName            string   `json:"track_name,omitempty"`
	ClipID               string   `json:"clip_id,omitempty"`
	ClipName             string   `json:"clip_name,omitempty"`
	ClipCount            int      `json:"clip_count"`
	LengthSeconds        float64  `json:"length_seconds,omitempty"`
	SourcePresent        bool     `json:"source_present"`
	SourceStatus         string   `json:"source_status"`
	PlaybackSourceStatus string   `json:"playback_source_status"`
	SourceExtension      string   `json:"source_extension,omitempty"`
	FormatFamily         string   `json:"format_family,omitempty"`
	SampleRateHz         float64  `json:"sample_rate_hz,omitempty"`
	BitDepth             int      `json:"bit_depth,omitempty"`
	ChannelCount         int      `json:"channel_count,omitempty"`
	AcousticStatus       string   `json:"acoustic_status,omitempty"`
	RMSDBFS              *float64 `json:"rms_dbfs,omitempty"`
	PeakDBFS             *float64 `json:"peak_dbfs,omitempty"`
	HeadroomDB           *float64 `json:"headroom_db,omitempty"`
	RiskCodes            []string `json:"risk_codes,omitempty"`
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
