package tom

const (
	Version       = "v0.2"
	SchemaVersion = "tom.projection.v0_2"
)

type ImportInput struct {
	ObservationID   string
	MixSessionID    string
	CreatedAt       string
	Summary         map[string]any
	Rows            []map[string]any
	TIMProjection   map[string]any
	DADWaveformRows []map[string]any
}

type Projection struct {
	SchemaVersion       string                 `json:"schema_version"`
	TOMVersion          string                 `json:"tom_version"`
	ObservationID       string                 `json:"observation_id,omitempty"`
	MixSessionID        string                 `json:"mix_session_id,omitempty"`
	Status              string                 `json:"status"`
	OrganizationSummary OrganizationSummary    `json:"organization_summary"`
	DisclosurePlan      DisclosurePlan         `json:"disclosure_plan"`
	NamingSignalReport  NamingSignalReport     `json:"naming_signal_report"`
	FullManifest        FullAssignmentManifest `json:"full_assignment_manifest"`
	GroupProposals      []GroupProposal        `json:"group_proposals"`
	NeedsReviewTracks   []TrackAssignment      `json:"needs_review_tracks,omitempty"`
	EvidenceRefs        []string               `json:"evidence_refs,omitempty"`
	Limitations         []string               `json:"limitations,omitempty"`
	LLMContext          LLMContext             `json:"llm_context"`
	GeneratedAt         string                 `json:"generated_at,omitempty"`
}

type OrganizationSummary struct {
	TrackCount                  int            `json:"track_count"`
	ProposedGroupCount          int            `json:"proposed_group_count"`
	NamingMatchedTrackCount     int            `json:"naming_matched_track_count"`
	TechnicalFallbackTrackCount int            `json:"technical_fallback_track_count"`
	NeedsReviewTrackCount       int            `json:"needs_review_track_count"`
	HighConfidenceTrackCount    int            `json:"high_confidence_track_count"`
	MediumConfidenceTrackCount  int            `json:"medium_confidence_track_count"`
	LowConfidenceTrackCount     int            `json:"low_confidence_track_count"`
	GroupCounts                 map[string]int `json:"group_counts,omitempty"`
	PrimaryStrategy             string         `json:"primary_strategy"`
}

type DisclosurePlan struct {
	SelectedStage string            `json:"selected_stage"`
	Reason        string            `json:"reason,omitempty"`
	Stages        []DisclosureStage `json:"stages,omitempty"`
}

type DisclosureStage struct {
	Stage         string `json:"stage"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	TrackCount    int    `json:"track_count,omitempty"`
	CoverageCount int    `json:"coverage_count,omitempty"`
}

type NamingSignalReport struct {
	TrackCount               int                 `json:"track_count"`
	NamingMatchedTrackCount  int                 `json:"naming_matched_track_count"`
	StructuralNameTrackCount int                 `json:"structural_name_track_count"`
	NamingSignalTrackCount   int                 `json:"naming_signal_track_count"`
	NamingSignalCoverage     float64             `json:"naming_signal_coverage"`
	WeakNameTrackCount       int                 `json:"weak_name_track_count"`
	TopTokens                []TokenCluster      `json:"top_tokens,omitempty"`
	TokenClusters            []TokenCluster      `json:"token_clusters,omitempty"`
	CommonPrefixes           []TokenCluster      `json:"common_prefixes,omitempty"`
	SimilarityClusters       []SimilarityCluster `json:"similarity_clusters,omitempty"`
	IgnoredTokens            []string            `json:"ignored_tokens,omitempty"`
}

type TokenCluster struct {
	Token        string   `json:"token"`
	Count        int      `json:"count"`
	TrackIDs     []string `json:"track_ids,omitempty"`
	ExampleNames []string `json:"example_names,omitempty"`
}

type SimilarityCluster struct {
	Key          string   `json:"key"`
	Count        int      `json:"count"`
	TrackIDs     []string `json:"track_ids,omitempty"`
	ExampleNames []string `json:"example_names,omitempty"`
	Meaningful   bool     `json:"meaningful"`
}

type FullAssignmentManifest struct {
	TrackCount              int             `json:"track_count"`
	AssignmentCoverageCount int             `json:"assignment_coverage_count"`
	UniqueTrackCount        int             `json:"unique_track_count"`
	CoverageStatus          string          `json:"coverage_status"`
	Groups                  []ManifestGroup `json:"groups"`
	DuplicateTrackIDs       []string        `json:"duplicate_track_ids,omitempty"`
	MissingTrackIDs         []string        `json:"missing_track_ids,omitempty"`
}

type ManifestGroup struct {
	GroupID           string               `json:"group_id"`
	Label             string               `json:"label"`
	ProposedFolder    string               `json:"proposed_folder"`
	RoleHypothesis    string               `json:"role_hypothesis,omitempty"`
	Confidence        string               `json:"confidence"`
	ConfidenceScore   float64              `json:"confidence_score"`
	TrackCount        int                  `json:"track_count"`
	TrackIDs          []string             `json:"track_ids"`
	Assignments       []ManifestAssignment `json:"assignments,omitempty"`
	NeedsConfirmation bool                 `json:"needs_confirmation"`
	RoutingHint       string               `json:"routing_hint,omitempty"`
}

type ManifestAssignment struct {
	TrackID         string   `json:"track_id"`
	TrackName       string   `json:"track_name,omitempty"`
	ClipID          string   `json:"clip_id,omitempty"`
	GroupID         string   `json:"group_id"`
	GroupLabel      string   `json:"group_label"`
	Confidence      string   `json:"confidence"`
	ConfidenceScore float64  `json:"confidence_score"`
	ClusterKeys     []string `json:"cluster_keys,omitempty"`
}

type GroupProposal struct {
	GroupID           string            `json:"group_id"`
	Label             string            `json:"label"`
	ProposedFolder    string            `json:"proposed_folder"`
	RoleHypothesis    string            `json:"role_hypothesis,omitempty"`
	Confidence        string            `json:"confidence"`
	ConfidenceScore   float64           `json:"confidence_score"`
	TrackCount        int               `json:"track_count"`
	Assignments       []TrackAssignment `json:"assignments,omitempty"`
	Basis             []Evidence        `json:"basis,omitempty"`
	NeedsConfirmation bool              `json:"needs_confirmation"`
	RoutingHint       string            `json:"routing_hint,omitempty"`
}

type TrackAssignment struct {
	TrackID         string     `json:"track_id,omitempty"`
	TrackName       string     `json:"track_name,omitempty"`
	ClipID          string     `json:"clip_id,omitempty"`
	ClipName        string     `json:"clip_name,omitempty"`
	SourcePath      string     `json:"source_path,omitempty"`
	GroupID         string     `json:"group_id"`
	GroupLabel      string     `json:"group_label"`
	RoleHypothesis  string     `json:"role_hypothesis,omitempty"`
	Confidence      string     `json:"confidence"`
	ConfidenceScore float64    `json:"confidence_score"`
	Evidence        []Evidence `json:"evidence,omitempty"`
	ClusterKeys     []string   `json:"cluster_keys,omitempty"`
}

type Evidence struct {
	Kind   string  `json:"kind"`
	Detail string  `json:"detail"`
	Weight float64 `json:"weight,omitempty"`
}

type LLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs,omitempty"`
	LimitationNotes        []string         `json:"limitation_notes,omitempty"`
	SuggestedNextStep      string           `json:"suggested_next_step,omitempty"`
}
