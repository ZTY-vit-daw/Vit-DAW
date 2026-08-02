package rlm

const (
	SchemaVersion = "reference_level_model.projection.v0"
	Version       = "rlm.v0"

	StatusReady   = "ready"
	StatusPartial = "partial"
	StatusMissing = "missing"

	ModeStrict = "strict"
	ModeCoarse = "coarse"
)

type Input struct {
	UserIntent          string
	ProjectState        map[string]any
	MixObservation      map[string]any
	AudioAnalysisStatus map[string]any
	GeneratedAt         string
}

type Projection struct {
	SchemaVersion  string              `json:"schema_version"`
	RLMVersion     string              `json:"rlm_version"`
	ProjectionID   string              `json:"projection_id"`
	Status         string              `json:"status"`
	Mode           string              `json:"mode,omitempty"`
	SelectedMetric string              `json:"selected_metric,omitempty"`
	SelectedUnit   string              `json:"selected_unit,omitempty"`
	ReferenceLevel *float64            `json:"reference_level,omitempty"`
	Actionable     bool                `json:"actionable"`
	Summary        Summary             `json:"summary"`
	MetricProfiles []MetricProfile     `json:"metric_profiles,omitempty"`
	Rows           []ReferenceLevelRow `json:"rows,omitempty"`
	Calibration    []CalibrationRow    `json:"calibration_rows,omitempty"`
	EvidenceRefs   []string            `json:"evidence_refs,omitempty"`
	Limitations    []string            `json:"limitations,omitempty"`
	GeneratedAt    string              `json:"generated_at,omitempty"`
}

type Summary struct {
	TrackCount            int     `json:"track_count"`
	EligibleTrackCount    int     `json:"eligible_track_count"`
	CalibrationReadyCount int     `json:"calibration_ready_count"`
	CandidateCount        int     `json:"candidate_count"`
	ActionCount           int     `json:"action_count"`
	MissingMetricCount    int     `json:"missing_metric_count"`
	NotCalibratableCount  int     `json:"not_calibratable_count"`
	ActiveRMSCount        int     `json:"active_rms_count"`
	RMSCount              int     `json:"rms_count"`
	LUFSCount             int     `json:"lufs_count"`
	SelectedMetric        string  `json:"selected_metric,omitempty"`
	SelectedUnit          string  `json:"selected_unit,omitempty"`
	Mode                  string  `json:"mode,omitempty"`
	ReferenceBasis        string  `json:"reference_basis,omitempty"`
	MinActionDeltaDB      float64 `json:"min_action_delta_db,omitempty"`
}

type MetricProfile struct {
	Metric                string   `json:"metric"`
	Unit                  string   `json:"unit,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
	Approximate           bool     `json:"approximate,omitempty"`
	EligibleTrackCount    int      `json:"eligible_track_count"`
	KnownTrackCount       int      `json:"known_track_count"`
	CalibrationReadyCount int      `json:"calibration_ready_count"`
	CandidateCount        int      `json:"candidate_count"`
	MissingMetricCount    int      `json:"missing_metric_count"`
	NotCalibratableCount  int      `json:"not_calibratable_count"`
	MissingTracks         []string `json:"missing_tracks,omitempty"`
	NotCalibratableTracks []string `json:"not_calibratable_tracks,omitempty"`
}

type ReferenceLevelRow struct {
	TrackID            string   `json:"track_id,omitempty"`
	TrackName          string   `json:"track_name,omitempty"`
	TrackType          string   `json:"track_type,omitempty"`
	Muted              bool     `json:"muted,omitempty"`
	PrimaryClipID      string   `json:"primary_clip_id,omitempty"`
	PrimaryClipName    string   `json:"primary_clip_name,omitempty"`
	TrackFaderDB       *float64 `json:"track_fader_db,omitempty"`
	ClipGainDB         *float64 `json:"clip_gain_db,omitempty"`
	PeakDBFS           *float64 `json:"peak_dbfs,omitempty"`
	RMSDBFS            *float64 `json:"rms_dbfs,omitempty"`
	ActiveRMSDBFS      *float64 `json:"active_rms_dbfs,omitempty"`
	IntegratedLUFS     *float64 `json:"integrated_lufs,omitempty"`
	ApproximateLUFS    *float64 `json:"approximate_lufs,omitempty"`
	HeadroomDB         *float64 `json:"headroom_db,omitempty"`
	CrestDB            *float64 `json:"crest_db,omitempty"`
	SilentSource       bool     `json:"silent_source,omitempty"`
	Eligible           bool     `json:"eligible"`
	CalibrationReady   bool     `json:"calibration_ready"`
	EligibilityReason  string   `json:"eligibility_reason,omitempty"`
	CalibrationBlocker string   `json:"calibration_blocker,omitempty"`
	SelectedMetric     string   `json:"selected_metric,omitempty"`
	ObservedLevel      *float64 `json:"observed_level,omitempty"`
	ReferenceLevel     *float64 `json:"reference_level,omitempty"`
	DeltaDB            *float64 `json:"delta_db,omitempty"`
	TargetClipGainDB   *float64 `json:"target_clip_gain_db,omitempty"`
	TargetClipped      bool     `json:"target_clipped_to_bound,omitempty"`
	RequiresAction     bool     `json:"requires_action,omitempty"`
	EvidenceRefs       []string `json:"evidence_refs,omitempty"`
}

type CalibrationRow struct {
	TrackID            string   `json:"track_id,omitempty"`
	TrackName          string   `json:"track_name,omitempty"`
	ClipID             string   `json:"clip_id,omitempty"`
	ClipName           string   `json:"clip_name,omitempty"`
	Metric             string   `json:"metric"`
	Unit               string   `json:"unit,omitempty"`
	Mode               string   `json:"mode,omitempty"`
	ObservedLevel      float64  `json:"observed_level"`
	ReferenceLevel     float64  `json:"reference_level"`
	CurrentClipGainDB  float64  `json:"current_clip_gain_db"`
	TargetClipGainDB   float64  `json:"target_clip_gain_db"`
	RequestedDeltaDB   float64  `json:"requested_delta_db"`
	AppliedDeltaDB     float64  `json:"applied_delta_db"`
	TargetClipped      bool     `json:"target_clipped_to_bound,omitempty"`
	PeakSafetyClipped  bool     `json:"target_clipped_to_peak_safety,omitempty"`
	SourcePeakDBFS     *float64 `json:"observed_source_peak_dbfs,omitempty"`
	ProjectedPeakDBFS  *float64 `json:"projected_static_peak_dbfs,omitempty"`
	PeakSafetyAchieved *bool    `json:"peak_safety_achieved,omitempty"`
	Risk               string   `json:"risk,omitempty"`
	EvidenceRefs       []string `json:"evidence_refs,omitempty"`
}
