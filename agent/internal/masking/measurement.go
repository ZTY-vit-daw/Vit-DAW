package masking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"
)

const (
	MeasurementSchema = "dad.masking_measurement.v1"
	FrameSchema       = "dad.l2_masking_frames.v1"
	ModelVersion      = "vit_relative_energetic_masking_risk.v1"
)

type BandDefinition struct {
	ID    string  `json:"id"`
	MinHz float64 `json:"min_hz"`
	MaxHz float64 `json:"max_hz"`
}

type Frame struct {
	StartSeconds float64            `json:"start_seconds"`
	EndSeconds   float64            `json:"end_seconds"`
	LevelsDBFS   map[string]float64 `json:"levels_dbfs"`
}

type TrackEvidence struct {
	TrackID          string           `json:"track_id"`
	TrackName        string           `json:"track_name,omitempty"`
	TapPoint         string           `json:"tap_point"`
	RenderRevision   string           `json:"render_revision"`
	AnalyzerRevision string           `json:"analyzer_revision"`
	EvidenceRef      string           `json:"evidence_ref"`
	SampleRate       float64          `json:"sample_rate"`
	TailSeconds      float64          `json:"tail_seconds"`
	RangeStart       float64          `json:"range_start_seconds"`
	RangeEnd         float64          `json:"range_end_seconds"`
	Bands            []BandDefinition `json:"bands"`
	Frames           []Frame          `json:"frames"`
}

type BuildInput struct {
	ProjectUUID      string
	ProjectRevision  string
	ProjectStateHash string
	CreatedAt        string
	Tracks           []TrackEvidence
}

type Measurement struct {
	SchemaVersion  string            `json:"schema_version"`
	MeasurementID  string            `json:"measurement_id"`
	Status         string            `json:"status"`
	Freshness      string            `json:"freshness"`
	Model          map[string]any    `json:"model"`
	ProjectBinding map[string]string `json:"project_binding"`
	Conditions     map[string]any    `json:"conditions"`
	Coverage       map[string]any    `json:"coverage"`
	PairBands      []PairBand        `json:"pair_bands,omitempty"`
	EvidenceRefs   []string          `json:"evidence_refs,omitempty"`
	Limitations    []string          `json:"limitations,omitempty"`
}

type PairBand struct {
	MaskerTrackID   string  `json:"masker_track_id"`
	MaskerTrackName string  `json:"masker_track_name,omitempty"`
	TargetTrackID   string  `json:"target_track_id"`
	TargetTrackName string  `json:"target_track_name,omitempty"`
	BandID          string  `json:"band_id"`
	MinHz           float64 `json:"min_hz"`
	MaxHz           float64 `json:"max_hz"`
	ActiveFrames    int     `json:"active_frame_count"`
	RiskFrames      int     `json:"risk_frame_count"`
	CoverageRatio   float64 `json:"risk_coverage_ratio"`
	MedianMarginDB  float64 `json:"median_margin_db"`
	P90MarginDB     float64 `json:"p90_margin_db"`
	MaxMarginDB     float64 `json:"max_margin_db"`
	Status          string  `json:"status"`
}

func TrackEvidenceFromProbeRow(row map[string]any, trackName string) (TrackEvidence, bool) {
	framesBlock := mapValue(row["masking_frames"])
	if textValue(framesBlock["schema_version"]) != FrameSchema || !strings.EqualFold(textValue(framesBlock["status"]), "ready") {
		return TrackEvidence{}, false
	}
	rangeRow := mapValue(row["analyzed_range"])
	out := TrackEvidence{
		TrackID:          textValue(row["track_id"]),
		TrackName:        strings.TrimSpace(trackName),
		TapPoint:         textValue(row["tap_point"]),
		RenderRevision:   textValue(row["render_revision"]),
		AnalyzerRevision: textValue(framesBlock["analyzer_revision"]),
		EvidenceRef:      textValue(row["evidence_ref"]),
		SampleRate:       numberValue(framesBlock["sample_rate"]),
		TailSeconds:      numberValue(row["tail_seconds"]),
		RangeStart:       numberValue(rangeRow["start_seconds"]),
		RangeEnd:         numberValue(rangeRow["end_seconds"]),
	}
	for _, bandRow := range mapRows(framesBlock["bands"]) {
		band := BandDefinition{ID: textValue(bandRow["id"]), MinHz: numberValue(bandRow["min_hz"]), MaxHz: numberValue(bandRow["max_hz"])}
		if band.ID != "" && band.MinHz > 0 && band.MaxHz > band.MinHz {
			out.Bands = append(out.Bands, band)
		}
	}
	for _, frameRow := range mapRows(framesBlock["frames"]) {
		levelsRow := mapValue(frameRow["levels_dbfs"])
		frame := Frame{StartSeconds: numberValue(frameRow["start_seconds"]), EndSeconds: numberValue(frameRow["end_seconds"]), LevelsDBFS: map[string]float64{}}
		for _, band := range out.Bands {
			if value, ok := levelsRow[band.ID]; ok {
				frame.LevelsDBFS[band.ID] = numberValue(value)
			}
		}
		if frame.EndSeconds > frame.StartSeconds && len(frame.LevelsDBFS) > 0 {
			out.Frames = append(out.Frames, frame)
		}
	}
	return out, out.TrackID != "" && out.TapPoint != "" && out.RenderRevision != "" && out.AnalyzerRevision != "" && out.SampleRate > 0 && out.RangeEnd > out.RangeStart && len(out.Bands) > 0 && len(out.Frames) > 0
}

// Build derives a directional relative masking-risk measurement from exact
// same-window, same-tap render-probe frames. It deliberately does not claim a
// calibrated audibility threshold or a subjective mixing defect.
func Build(input BuildInput) Measurement {
	out := Measurement{
		SchemaVersion: MeasurementSchema,
		Status:        "suspect",
		Freshness:     "suspect",
		Model: map[string]any{
			"model_version":                 ModelVersion,
			"threshold_domain":              "relative_dbfs",
			"masker_offset_db":              6.0,
			"upward_spread_db_per_octave":   12.0,
			"downward_spread_db_per_octave": 24.0,
			"candidate_only":                true,
		},
		ProjectBinding: map[string]string{
			"project_uuid":       strings.TrimSpace(input.ProjectUUID),
			"project_revision":   strings.TrimSpace(input.ProjectRevision),
			"project_state_hash": strings.TrimSpace(input.ProjectStateHash),
		},
		Limitations: []string{
			"relative_dbfs_model_is_not_calibrated_spl",
			"broad_frequency_bands_do_not_establish_a_universal_psychoacoustic_threshold",
			"masking_risk_candidates_are_improvement_evidence_not_deterministic_mix_defects",
		},
	}
	tracks, reason := comparableTracks(input.Tracks)
	if reason != "" {
		out.Limitations = append(out.Limitations, reason)
		out.Coverage = map[string]any{"track_count": len(input.Tracks), "eligible_track_count": len(tracks)}
		out.MeasurementID = measurementID(input, tracks)
		return out
	}
	for _, track := range tracks {
		out.EvidenceRefs = appendUnique(out.EvidenceRefs, track.EvidenceRef)
	}
	for maskerIndex := range tracks {
		for targetIndex := range tracks {
			if maskerIndex == targetIndex {
				continue
			}
			out.PairBands = append(out.PairBands, compareTrackPair(tracks[maskerIndex], tracks[targetIndex])...)
		}
	}
	sort.Slice(out.PairBands, func(i, j int) bool {
		left, right := out.PairBands[i], out.PairBands[j]
		if left.Status != right.Status {
			return left.Status == "candidate"
		}
		if left.CoverageRatio != right.CoverageRatio {
			return left.CoverageRatio > right.CoverageRatio
		}
		if left.P90MarginDB != right.P90MarginDB {
			return left.P90MarginDB > right.P90MarginDB
		}
		return left.MaskerTrackID+left.TargetTrackID+left.BandID < right.MaskerTrackID+right.TargetTrackID+right.BandID
	})
	candidates := 0
	for _, row := range out.PairBands {
		if row.Status == "candidate" {
			candidates++
		}
	}
	first := tracks[0]
	out.Status = "ready"
	out.Freshness = "fresh"
	out.Conditions = map[string]any{
		"tap_point":             first.TapPoint,
		"range_start_seconds":   round3(first.RangeStart),
		"range_end_seconds":     round3(first.RangeEnd),
		"sample_rate":           first.SampleRate,
		"tail_seconds":          round3(first.TailSeconds),
		"analyzer_revision":     first.AnalyzerRevision,
		"synchronized":          true,
		"frame_count_per_track": len(first.Frames),
	}
	out.Coverage = map[string]any{
		"track_count":             len(input.Tracks),
		"eligible_track_count":    len(tracks),
		"directional_pair_count":  len(tracks) * (len(tracks) - 1),
		"pair_band_count":         len(out.PairBands),
		"candidate_count":         candidates,
		"complete_track_coverage": len(tracks) == len(input.Tracks),
	}
	out.MeasurementID = measurementID(input, tracks)
	out.EvidenceRefs = appendUnique(out.EvidenceRefs, "dad.masking_measurement:"+out.MeasurementID)
	return out
}

func comparableTracks(in []TrackEvidence) ([]TrackEvidence, string) {
	tracks := make([]TrackEvidence, 0, len(in))
	for _, track := range in {
		if strings.TrimSpace(track.TrackID) == "" || strings.TrimSpace(track.TapPoint) == "" || strings.TrimSpace(track.RenderRevision) == "" || len(track.Bands) == 0 || len(track.Frames) == 0 {
			continue
		}
		tracks = append(tracks, track)
	}
	if len(tracks) < 2 {
		return tracks, "at_least_two_ready_track_render_probes_are_required"
	}
	base := tracks[0]
	for _, track := range tracks[1:] {
		if !strings.EqualFold(base.TapPoint, track.TapPoint) {
			return tracks, "mixed_tap_points_are_not_comparable"
		}
		if math.Abs(base.RangeStart-track.RangeStart) > 0.001 || math.Abs(base.RangeEnd-track.RangeEnd) > 0.001 {
			return tracks, "analyzed_ranges_are_not_synchronized"
		}
		if math.Abs(base.SampleRate-track.SampleRate) > 0.5 || base.AnalyzerRevision != track.AnalyzerRevision {
			return tracks, "measurement_conditions_are_not_comparable"
		}
		if math.Abs(base.TailSeconds-track.TailSeconds) > 0.001 {
			return tracks, "mixed_tail_windows_are_not_comparable"
		}
		if len(base.Frames) != len(track.Frames) || !sameBands(base.Bands, track.Bands) {
			return tracks, "frame_or_band_layout_is_not_comparable"
		}
		for index := range base.Frames {
			if math.Abs(base.Frames[index].StartSeconds-track.Frames[index].StartSeconds) > 0.001 || math.Abs(base.Frames[index].EndSeconds-track.Frames[index].EndSeconds) > 0.001 {
				return tracks, "frame_timing_is_not_synchronized"
			}
		}
	}
	return tracks, ""
}

func compareTrackPair(masker, target TrackEvidence) []PairBand {
	out := make([]PairBand, 0, len(target.Bands))
	for _, targetBand := range target.Bands {
		targetLevels := make([]float64, 0, len(target.Frames))
		for _, frame := range target.Frames {
			if value, ok := frame.LevelsDBFS[targetBand.ID]; ok && isUsableLevel(value) {
				targetLevels = append(targetLevels, value)
			}
		}
		activityFloor := math.Max(-96.0, percentile(targetLevels, 0.10)-6.0)
		margins := []float64{}
		riskFrames := 0
		for frameIndex, targetFrame := range target.Frames {
			targetLevel, ok := targetFrame.LevelsDBFS[targetBand.ID]
			if !ok || !isUsableLevel(targetLevel) || targetLevel < activityFloor {
				continue
			}
			threshold := relativeMaskedThreshold(masker.Frames[frameIndex], masker.Bands, targetBand)
			if !isUsableLevel(threshold) {
				continue
			}
			margin := threshold - targetLevel
			margins = append(margins, margin)
			if margin > 0 {
				riskFrames++
			}
		}
		coverage := 0.0
		if len(margins) > 0 {
			coverage = float64(riskFrames) / float64(len(margins))
		}
		status := "clear_in_model"
		if len(margins) == 0 {
			status = "insufficient"
		} else if riskFrames >= 2 && coverage >= 0.10 && percentile(margins, 0.90) > 0 {
			status = "candidate"
		}
		out = append(out, PairBand{
			MaskerTrackID: masker.TrackID, MaskerTrackName: masker.TrackName,
			TargetTrackID: target.TrackID, TargetTrackName: target.TrackName,
			BandID: targetBand.ID, MinHz: targetBand.MinHz, MaxHz: targetBand.MaxHz,
			ActiveFrames: len(margins), RiskFrames: riskFrames, CoverageRatio: round3(coverage),
			MedianMarginDB: round3(percentile(margins, 0.50)), P90MarginDB: round3(percentile(margins, 0.90)), MaxMarginDB: round3(maxValue(margins)), Status: status,
		})
	}
	return out
}

func relativeMaskedThreshold(frame Frame, maskerBands []BandDefinition, target BandDefinition) float64 {
	targetCenter := math.Sqrt(target.MinHz * target.MaxHz)
	energy := 0.0
	for _, band := range maskerBands {
		level, ok := frame.LevelsDBFS[band.ID]
		if !ok || !isUsableLevel(level) {
			continue
		}
		maskerCenter := math.Sqrt(band.MinHz * band.MaxHz)
		octaves := math.Abs(math.Log2(targetCenter / maskerCenter))
		slope := 24.0
		if targetCenter > maskerCenter {
			slope = 12.0
		}
		contributionDB := level - 6.0 - slope*octaves
		energy += math.Pow(10.0, contributionDB/10.0)
	}
	if energy <= 0 {
		return math.Inf(-1)
	}
	return 10.0 * math.Log10(energy)
}

func sameBands(left, right []BandDefinition) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].ID != right[index].ID || math.Abs(left[index].MinHz-right[index].MinHz) > 0.01 || math.Abs(left[index].MaxHz-right[index].MaxHz) > 0.01 {
			return false
		}
	}
	return true
}

func measurementID(input BuildInput, tracks []TrackEvidence) string {
	payload := map[string]any{
		"schema_version": MeasurementSchema, "model_version": ModelVersion,
		"project_uuid": input.ProjectUUID, "project_revision": input.ProjectRevision, "project_state_hash": input.ProjectStateHash,
		"tracks": tracks,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return "mask_" + hex.EncodeToString(sum[:12])
}

func percentile(values []float64, q float64) float64 {
	clean := make([]float64, 0, len(values))
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		clean = append(clean, value)
	}
	if len(clean) == 0 {
		return 0
	}
	sort.Float64s(clean)
	position := math.Max(0, math.Min(1, q)) * float64(len(clean)-1)
	lower, upper := int(math.Floor(position)), int(math.Ceil(position))
	if lower == upper {
		return clean[lower]
	}
	weight := position - float64(lower)
	return clean[lower]*(1-weight) + clean[upper]*weight
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func isUsableLevel(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > -159.0
}

func round3(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*1000) / 1000
}

func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return map[string]any{}
}

func mapRows(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, value := range rows {
			if row, ok := value.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func textValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(stringValue(value))
}

func stringValue(value any) string {
	data, _ := json.Marshal(value)
	if len(data) >= 2 && data[0] == '"' {
		var out string
		_ = json.Unmarshal(data, &out)
		return out
	}
	return strings.Trim(string(data), "\"")
}

func numberValue(value any) float64 {
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	case int64:
		return float64(number)
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	default:
		return 0
	}
}
