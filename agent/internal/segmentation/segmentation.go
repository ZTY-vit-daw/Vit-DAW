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

import (
	"encoding/json"
	"fmt"
	"math"
)

// Exposed detection constants (确定性范式：参数全部显式透出，不发明自适应).
const (
	// DensityScalePerSecond and NoveltyScaleLinear are the ABSOLUTE
	// reference scales: an onset density of DensityScalePerSecond onsets/s
	// (resp. a linear-RMS novelty of NoveltyScaleLinear) contributes 1.0 to
	// the combined signal. Absolute references keep the threshold meaning
	// stable across sources and runs — no input-relative normalisation.
	DensityScalePerSecond = 4.0
	NoveltyScaleLinear    = 0.05
	// BoundaryDensityWeight and BoundaryNoveltyWeight are the linear
	// combination weights over the scaled signals.
	BoundaryDensityWeight     = 1.0
	BoundaryNoveltyWeight     = 1.0
	BoundaryCombinedThreshold = 0.65
	// MinSectionSeconds is the anti-fragmentation floor: boundaries closer
	// than this merge (the stronger candidate wins), and boundaries that
	// would leave a shorter first/last section are dropped.
	MinSectionSeconds       = 8.0
	ConfidenceHighThreshold = 0.8
	ConfidenceMediumFloor   = 0.7
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
	Status      string         `json:"status"`
	MaxNovelty  float64        `json:"max_novelty"`
	MeanNovelty float64        `json:"mean_novelty"`
	Frames      []noveltyFrame `json:"frames"`
}

// Primitives is the agent-side view of the kernel segmentation_primitives
// payload (SEG-1). The boundary picker consumes only the density and
// novelty curves; onset events ride along for future consumers.
type Primitives struct {
	FeatureType   string             `json:"feature_type"`
	TrackID       string             `json:"track_id,omitempty"`
	ClipID        string             `json:"clip_id,omitempty"`
	WindowMs      float64            `json:"window_ms"`
	HopMs         float64            `json:"hop_ms"`
	FrameCount    int64              `json:"frame_count"`
	OnsetEvents   onsetEventsBlock   `json:"onset_events"`
	OnsetDensity  onsetDensityBlock  `json:"onset_density"`
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
	var p Primitives
	if err := json.Unmarshal(payload, &p); err != nil {
		return Primitives{}, fmt.Errorf("segmentation: parse primitives: %w", err)
	}
	return p, nil
}

// DetectBoundaries is the deterministic linear-threshold boundary picker
// (v1: no adaptive parameters).
//
// Pipeline: scale the onset-density and energy-novelty curves by their
// ABSOLUTE reference constants (no input-relative normalisation — the
// threshold means the same thing on every source), sum them with the
// exposed weights, keep local-maximum frames at or above the threshold,
// merge candidates closer than MinSectionSeconds (stronger wins), drop
// boundaries that would leave a first/last section shorter than
// MinSectionSeconds, then partition the timeline into neutral S1…Sn
// sections.
func DetectBoundaries(p Primitives) Result {
	constants := map[string]float64{
		"DensityScalePerSecond":     DensityScalePerSecond,
		"NoveltyScaleLinear":        NoveltyScaleLinear,
		"BoundaryDensityWeight":     BoundaryDensityWeight,
		"BoundaryNoveltyWeight":     BoundaryNoveltyWeight,
		"BoundaryCombinedThreshold": BoundaryCombinedThreshold,
		"MinSectionSeconds":         MinSectionSeconds,
	}
	ne := func(reasons ...string) Result {
		return Result{Status: StatusNotEvaluable, Reasons: reasons, Constants: constants}
	}
	// The density curve is computed from the full onset scan regardless of
	// the event-list disclosure cap, so onset_events.status is not gating.
	if p.OnsetDensity.Status != StatusReady {
		return ne("onset_density_not_ready")
	}
	if p.EnergyNovelty.Status != StatusReady {
		return ne("energy_novelty_not_ready")
	}
	density := p.OnsetDensity.Frames
	novelty := p.EnergyNovelty.Frames
	if len(density) == 0 {
		return ne("empty_density_frames")
	}
	if len(novelty) == 0 {
		return ne("empty_novelty_frames")
	}
	if len(density) != len(novelty) {
		return ne("density_novelty_grid_mismatch")
	}

	hasDensitySignal, hasNoveltySignal := false, false
	for _, frame := range density {
		if frame.OnsetsPerSecond > 0 {
			hasDensitySignal = true
		}
	}
	for _, frame := range novelty {
		if frame.Novelty > 0 {
			hasNoveltySignal = true
		}
	}
	if !hasDensitySignal && !hasNoveltySignal {
		return ne("zero_signal_range")
	}

	combined := make([]float64, len(density))
	for i := range density {
		combined[i] = BoundaryDensityWeight*(density[i].OnsetsPerSecond/DensityScalePerSecond) +
			BoundaryNoveltyWeight*(novelty[i].Novelty/NoveltyScaleLinear)
	}

	// Local-maximum candidates above the threshold. Plateau rule: strictly
	// greater than the previous frame, at least the next one.
	type candidate struct {
		at       float64
		strength float64
	}
	var candidates []candidate
	for i := range combined {
		if combined[i] < BoundaryCombinedThreshold {
			continue
		}
		if i > 0 && combined[i] < combined[i-1] {
			continue
		}
		if i+1 < len(combined) && combined[i] < combined[i+1] {
			continue
		}
		candidates = append(candidates, candidate{at: density[i].StartSeconds, strength: combined[i]})
	}

	// Min-section merge: candidates closer than MinSectionSeconds collapse
	// onto the stronger one (deterministic: replace the kept boundary when
	// a later in-window candidate is strictly stronger).
	var kept []candidate
	for _, c := range candidates {
		if len(kept) > 0 && c.at-kept[len(kept)-1].at < MinSectionSeconds {
			if c.strength > kept[len(kept)-1].strength {
				kept[len(kept)-1] = c
			}
			continue
		}
		kept = append(kept, c)
	}

	duration := density[len(density)-1].EndSeconds
	// Drop boundaries that would leave a sub-minimum first or last section.
	filtered := kept[:0]
	for _, c := range kept {
		if c.at < MinSectionSeconds || duration-c.at < MinSectionSeconds {
			continue
		}
		filtered = append(filtered, c)
	}

	boundaries := make([]Boundary, 0, len(filtered))
	for _, c := range filtered {
		boundaries = append(boundaries, Boundary{
			AtSeconds:  round3(c.at),
			Combined:   round3(c.strength),
			Confidence: confidenceBin(c.strength),
		})
	}

	// Section assembly: section k is delimited by boundary k-1 (its opener,
	// whose confidence it inherits) and boundary k (its closer). Section 1
	// opens at the project start — no discovery event — so it carries no
	// confidence; the marker omits the field (kernel writes confidence only
	// when non-empty).
	duration = round3(duration)
	sections := make([]Section, 0, len(boundaries)+1)
	start := 0.0
	for k, b := range boundaries {
		sections = append(sections, Section{
			SectionID:    fmt.Sprintf("S%d", k+1),
			StartSeconds: round3(start),
			EndSeconds:   b.AtSeconds,
		})
		start = b.AtSeconds
	}
	sections = append(sections, Section{
		SectionID:    fmt.Sprintf("S%d", len(boundaries)+1),
		StartSeconds: round3(start),
		EndSeconds:   duration,
	})
	for k := range sections {
		if k == 0 {
			continue
		}
		sections[k].Confidence = boundaries[k-1].Confidence
	}

	return Result{
		Status:          StatusReady,
		DurationSeconds: duration,
		Boundaries:      boundaries,
		Sections:        sections,
		Constants:       constants,
	}
}

func confidenceBin(strength float64) string {
	switch {
	case strength >= ConfidenceHighThreshold:
		return ConfidenceHigh
	case strength >= ConfidenceMediumFloor:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

// BuildApplySectionMarkersCommand constructs the kernel command payload that
// writes the neutral sections as markers (source-filtered replace). Returns
// nil when the result is not evaluable or carries no sections.
func BuildApplySectionMarkersCommand(r Result) map[string]any {
	if r.Status != StatusReady || len(r.Sections) == 0 {
		return nil
	}
	sections := make([]map[string]any, 0, len(r.Sections))
	for _, s := range r.Sections {
		row := map[string]any{
			"start_seconds": s.StartSeconds,
			"end_seconds":   s.EndSeconds,
			"section_id":    s.SectionID,
			"created_by":    SourceTag,
		}
		if s.Confidence != "" {
			row["confidence"] = s.Confidence
		}
		sections = append(sections, row)
	}
	return map[string]any{
		"command":          "project.markers.apply_section_markers",
		"sections":         sections,
		"source":           SourceTag,
		"replace_existing": true,
	}
}
