package lowendrelation

import (
	"time"

	"vit-daw-agent/internal/capabilityruntime"
)

const (
	CapabilityID        = "static_mix.low_end_relation.v0"
	ModelSchemaVersion  = "low_end_relation.model.v0"
	ResultSchemaVersion = "low_end_relation.result.v0"
	TreatmentPlanSchema = "low_end_relation.treatment_plan.v1"
)

// Input feeds the B4 model from CCB-assembled evidence.
// AudioAnalysisStatus and MixStyle are intentionally absent: the deterministic
// B4 diagnosis operates on MOM/TOM evidence. A later chat orchestration phase
// may turn that diagnosis into a separately governed generic-EQ treatment.
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

// LowEndTrack represents one track identified as a low-end contributor.
// Tracks are included when they appear in MOM sub or bass band occupancy.
type LowEndTrack struct {
	TrackID        string   `json:"track_id"`
	TrackName      string   `json:"track_name,omitempty"`
	Role           string   `json:"role,omitempty"`
	RoleSource     string   `json:"role_source,omitempty"`
	RoleConfidence float64  `json:"role_confidence,omitempty"`
	SubUnitEnergy  *float64 `json:"sub_unit_energy,omitempty"`
	SubEnergyDB    *float64 `json:"sub_energy_db,omitempty"`
	SubRank        int      `json:"sub_rank,omitempty"`
	BassUnitEnergy *float64 `json:"bass_unit_energy,omitempty"`
	BassEnergyDB   *float64 `json:"bass_energy_db,omitempty"`
	BassRank       int      `json:"bass_rank,omitempty"`
	FaderDB        *float64 `json:"fader_db,omitempty"`
	ConflictBands  []string `json:"conflict_bands,omitempty"`
}

// LowEndConflict is a masking-conflict candidate from MOM band_conflict_candidates
// narrowed to the sub and bass bands.
type LowEndConflict struct {
	ID         string           `json:"conflict_id"`
	Band       string           `json:"band"`
	Tracks     []map[string]any `json:"tracks,omitempty"`
	Confidence string           `json:"confidence,omitempty"`
	Reason     string           `json:"reason,omitempty"`
}

// TreatmentTarget is one leaf in a project-wide B4 treatment plan. It binds
// the specialist relationship decision to an exact track, but deliberately
// contains no plug-in identity, topology, or EQ parameter. Those remain the
// responsibility of the reusable generic-EQ flow after instance resolution.
type TreatmentTarget struct {
	TargetID         string   `json:"target_id"`
	Order            int      `json:"order"`
	TrackID          string   `json:"track_id"`
	TrackName        string   `json:"track_name,omitempty"`
	RelationshipRefs []string `json:"relationship_refs"`
	ListeningGoal    string   `json:"listening_goal"`
	Constraints      []string `json:"constraints,omitempty"`
	EvidenceRefs     []string `json:"evidence_refs,omitempty"`
}

// TreatmentPlan is the bounded project-level handoff from B4 diagnosis to
// generic EQ. The observation scope remains the whole project while Targets
// enumerate every exact leaf that the project-level plan requires.
type TreatmentPlan struct {
	SchemaVersion     string            `json:"schema_version"`
	PlanID            string            `json:"plan_id"`
	DiagnosisID       string            `json:"diagnosis_id"`
	ObservationID     string            `json:"observation_id"`
	ObservationScope  string            `json:"observation_scope"`
	Summary           string            `json:"summary"`
	Targets           []TreatmentTarget `json:"targets"`
	GlobalConstraints []string          `json:"global_constraints,omitempty"`
	EvidenceRefs      []string          `json:"evidence_refs,omitempty"`
	Limitations       []string          `json:"limitations,omitempty"`
}

// LowEndSummary is a compact project-level overview of the low-end landscape.
type LowEndSummary struct {
	LowEndTendency      string `json:"low_end_tendency"`
	DominantSubTrackID  string `json:"dominant_sub_track_id,omitempty"`
	DominantBassTrackID string `json:"dominant_bass_track_id,omitempty"`
	ConflictCount       int    `json:"conflict_count"`
	SubBandTrackCount   int    `json:"sub_band_track_count"`
	BassBandTrackCount  int    `json:"bass_band_track_count"`
}

// Coverage records evidence availability for B4 readiness evaluation.
type Coverage struct {
	AnalyzedTrackCount      int     `json:"analyzed_track_count"`
	BandEvidenceKnownCount  int     `json:"band_evidence_known_count"`
	SubBandLeaderCount      int     `json:"sub_band_leader_count"`
	BassBandLeaderCount     int     `json:"bass_band_leader_count"`
	ConflictCandidateCount  int     `json:"conflict_candidate_count"`
	LowEndTendencyAvailable bool    `json:"low_end_tendency_available"`
	RoleKnownCount          int     `json:"role_known_count"`
	BandEvidenceCoverage    float64 `json:"band_evidence_coverage"`
}

// Observation is one structured low-end finding surfaced to the LLM layer.
type Observation struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Summary    string         `json:"summary"`
	TrackRefs  []string       `json:"track_refs,omitempty"`
	Risk       string         `json:"risk,omitempty"`
	Confidence string         `json:"confidence,omitempty"`
	Evidence   map[string]any `json:"evidence,omitempty"`
}

// Model is the full internal B4 representation before disclosure budgeting.
type Model struct {
	SchemaVersion string                      `json:"schema_version"`
	ModelID       string                      `json:"model_id"`
	CapabilityID  string                      `json:"capability_id"`
	GeneratedAt   string                      `json:"generated_at"`
	UserIntent    string                      `json:"user_intent,omitempty"`
	ObservationID string                      `json:"observation_id,omitempty"`
	Tracks        []LowEndTrack               `json:"tracks,omitempty"`
	Conflicts     []LowEndConflict            `json:"conflicts,omitempty"`
	Summary       LowEndSummary               `json:"summary"`
	Coverage      Coverage                    `json:"coverage"`
	Readiness     capabilityruntime.Readiness `json:"readiness"`
	Observations  []Observation               `json:"observations,omitempty"`
	EvidenceRefs  []string                    `json:"evidence_refs,omitempty"`
	Limitations   []string                    `json:"limitations,omitempty"`
	MOMSummary    map[string]any              `json:"mom_summary,omitempty"`
	TOMSummary    map[string]any              `json:"tom_summary,omitempty"`
}

// Result is the output of Analyze; it mirrors the structure callers need
// when building the Context Pack and Capability Outcome.
type Result struct {
	SchemaVersion      string                      `json:"schema_version"`
	ResultID           string                      `json:"result_id"`
	ModelID            string                      `json:"model_id"`
	CapabilityID       string                      `json:"capability_id"`
	GeneratedAt        string                      `json:"generated_at"`
	ObservationID      string                      `json:"observation_id,omitempty"`
	Readiness          capabilityruntime.Readiness `json:"readiness"`
	Observations       []Observation               `json:"observations,omitempty"`
	Conflicts          []LowEndConflict            `json:"conflicts,omitempty"`
	Summary            LowEndSummary               `json:"summary"`
	AnalyzedTrackCount int                         `json:"analyzed_track_count"`
	EvidenceRefs       []string                    `json:"evidence_refs,omitempty"`
	Limitations        []string                    `json:"limitations,omitempty"`
}
