// Package segmentation implements the L2-2-SEG-2 discovery-layer boundary
// picker: it consumes the kernel SegmentationPrimitives payload (SEG-1,
// onset density + energy novelty, both on the L3 frame grid) and derives
// deterministic section boundaries plus neutral S1…Sn section identities,
// then constructs the project.markers.apply_section_markers command.
//
// v1 is pinned to deterministic linear-threshold detection — no adaptive or
// learned parameters. Every threshold is an exported constant and rides on
// the result for run-to-run comparability (确定性范式).
//
// Naming layer is deliberately absent: sections carry only neutral IDs; any
// "sounds like a chorus" hypothesis labels belong to later cards (D10
// discovery/naming separation).
//
// Payload authority: VitApp/Source/Service/L3AcousticAnalyzer.cpp
// publishSegmentationPrimitives (feature_type segmentation_primitives,
// primitives_scope "onset_density+energy_novelty_v1_no_boundary_picking").
// Command authority: ProjectMarkerService handleApplySectionMarkers.
package segmentation

// Exposed detection constants (确定性范式：参数全部显式透出，不发明自适应).
const (
	// BoundaryDensityWeight and BoundaryNoveltyWeight are the linear
	// combination weights over the max-normalised onset-density and energy
	// novelty signals.
	BoundaryDensityWeight     = 0.5
	BoundaryNoveltyWeight     = 0.5
	BoundaryCombinedThreshold = 0.5
	// MinSectionSeconds is the anti-fragmentation floor: boundaries closer
	// than this merge (the stronger candidate wins), and boundaries that
	// would leave a shorter first/last section are dropped.
	MinSectionSeconds       = 8.0
	ConfidenceHighThreshold = 0.8
	ConfidenceMediumFloor   = 0.65
)

// SourceTag filters discovery-layer markers on replace (isolated from user
// markers by the kernel's per-source replace_existing semantics).
const SourceTag = "segmentation_primitives_v1"

// Result statuses.
const (
	StatusReady        = "ready"
	StatusNotEvaluable = "not_evaluable"
)

// Confidence bins — an explicit rule over the combined signal strength at
// the boundary frame, not a fabricated probability.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// OnsetEvent mirrors the kernel onset_events.events rows.
type OnsetEvent struct {
	OnsetSeconds float64 `json:"onset_seconds"`
	FrameIndex   int64   `json:"frame_index"`
	OnsetDBFS    float64 `json:"onset_dbfs"`
	RiseDB       float64 `json:"rise_db"`
}

type onsetEventsBlock struct {
	Status string       `json:"status"`
	Events []OnsetEvent `json:"events"`
}

type densityFrame struct {
	StartSeconds    float64 `json:"start_seconds"`
	EndSeconds      float64 `json:"end_seconds"`
	OnsetCount      int64   `json:"onset_count"`
	OnsetsPerSecond float64 `json:"onsets_per_second"`
}

type onsetDensityBlock struct {
	Status        string         `json:"status"`
	WindowSeconds float64        `json:"window_seconds"`
	WindowFrames  int64          `json:"window_frames"`
	Frames        []densityFrame `json:"frames"`
}

type noveltyFrame struct {
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Novelty      float64 `json:"novelty"`
}

type energyNoveltyBlock struct {
	Status       string          `json:"status"`
	MaxNovelty   float64         `json:"max_novelty"`
	MeanNovelty  float64         `json:"mean_novelty"`
	Frames       []noveltyFrame  `json:"frames"`
}

// Primitives is the agent-side view of the kernel segmentation_primitives
// payload (SEG-1). The boundary picker consumes only the density and
// novelty curves; onset events ride along for future consumers.
type Primitives struct {
	FeatureType  string              `json:"feature_type"`
	TrackID      string              `json:"track_id,omitempty"`
	ClipID       string              `json:"clip_id,omitempty"`
	WindowMs     float64             `json:"window_ms"`
	HopMs        float64             `json:"hop_ms"`
	FrameCount   int64               `json:"frame_count"`
	OnsetEvents  onsetEventsBlock    `json:"onset_events"`
	OnsetDensity onsetDensityBlock   `json:"onset_density"`
	EnergyNovelty energyNoveltyBlock `json:"energy_novelty"`
}

// Boundary is one detected section boundary with its explicit confidence
// bin (rule over the combined signal strength, not a probability).
type Boundary struct {
	AtSeconds  float64 `json:"at_seconds"`
	Combined   float64 `json:"combined"`
	Confidence string  `json:"confidence"`
}

// Section is one neutral-ID section between consecutive boundaries.
// Confidence inherits from the boundary that OPENS the section; the opening
// section (started by the project start, not a discovery event) carries an
// empty confidence and the marker omits the field (kernel contract: empty
// string is not written).
type Section struct {
	SectionID    string  `json:"section_id"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Confidence   string  `json:"confidence,omitempty"`
}

// Result is the deterministic output of the boundary picker.
type Result struct {
	Status          string             `json:"status"`
	Reasons         []string           `json:"reasons,omitempty"`
	DurationSeconds float64            `json:"duration_seconds"`
	Boundaries      []Boundary         `json:"boundaries,omitempty"`
	Sections        []Section          `json:"sections,omitempty"`
	Constants       map[string]float64 `json:"constants"`
}

// ParsePrimitives decodes a kernel segmentation_primitives payload JSON.
func ParsePrimitives(payload []byte) (Primitives, error) {
	_ = payload
	return Primitives{}, nil
}

// DetectBoundaries is the deterministic linear-threshold boundary picker
// (v1: no adaptive parameters).
func DetectBoundaries(p Primitives) Result {
	return Result{}
}

// BuildApplySectionMarkersCommand constructs the kernel command payload that
// writes the neutral sections as markers (source-filtered replace). Returns
// nil when the result is not evaluable or carries no sections.
func BuildApplySectionMarkersCommand(r Result) map[string]any {
	return nil
}
