package com

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
)

const silenceFloorDBFS = -90.0

// Build constructs a COM projection. source_only consumes macro evidence;
// paired_io consumes one validated COM-2 artifact without retaining its raw
// traces in the returned projection.
func Build(input Input) Projection {
	input = normalizeInput(input)
	mode := normalizeMode(input.Mode)
	if mode == ModePairedIO {
		return buildPairedProjection(input)
	}
	if mode == ModeChangeDelta {
		return buildChangeDeltaProjection(input)
	}
	source := buildSourceDynamics(input)
	identifiability := buildSourceOnlyIdentifiability(source)
	trust := buildSourceOnlyTrust(input, source, mode)
	generatedAt := strings.TrimSpace(input.CreatedAt)
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	projection := Projection{
		SchemaVersion:  SchemaVersion,
		COMVersion:     Version,
		Mode:           mode,
		Status:         trust.OverallStatus,
		ObservationID:  strings.TrimSpace(input.ObservationID),
		MixSessionID:   strings.TrimSpace(input.MixSessionID),
		TargetRef:      cloneMap(input.TargetRef),
		ProcessorScope: input.ProcessorScope,
		Conditions:     input.Conditions,
		EvidenceInputs: EvidenceInputs{
			SourceEvidenceID: strings.TrimSpace(input.Source.ID),
			SourceSchema:     strings.TrimSpace(input.Source.SchemaVersion),
			SourceStatus:     normalizeStatus(input.Source.Status),
			EvidenceRefs:     append([]string(nil), input.Source.EvidenceRefs...),
		},
		SourceDynamics:  &source,
		Identifiability: identifiability,
		TrustQuality:    trust,
		EvidenceRefs:    append([]string(nil), input.Source.EvidenceRefs...),
		Limitations:     uniqueSorted(append(append([]string{}, source.Limitations...), trust.Limitations...)),
		GeneratedAt:     generatedAt,
	}
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func normalizeInput(input Input) Input {
	input.TargetRef = cloneMap(input.TargetRef)
	input.Source.EvidenceRefs = uniqueSorted(input.Source.EvidenceRefs)
	input.Source.TimeSegments = normalizeSegments(input.Source.TimeSegments)
	input.Conditions.AnalysisResolutions = normalizeResolutions(input.Conditions.AnalysisResolutions)
	if input.Paired != nil {
		paired := *input.Paired
		paired.AlignedEnvelopeFrames = normalizePairedFrames(paired.AlignedEnvelopeFrames)
		paired.InputEventCandidates = normalizePairedEvents(paired.InputEventCandidates)
		input.Paired = &paired
	}
	return input
}

func normalizeSegments(in []TimeSegment) []TimeSegment {
	out := append([]TimeSegment(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartSeconds != out[j].StartSeconds {
			return out[i].StartSeconds < out[j].StartSeconds
		}
		return out[i].EndSeconds < out[j].EndSeconds
	})
	return out
}

func normalizeResolutions(in []AnalysisResolution) []AnalysisResolution {
	out := append([]AnalysisResolution(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Scale != out[j].Scale {
			return out[i].Scale < out[j].Scale
		}
		if out[i].WindowMS != out[j].WindowMS {
			return out[i].WindowMS < out[j].WindowMS
		}
		return out[i].HopMS < out[j].HopMS
	})
	return out
}

func buildSourceDynamics(input Input) SourceDynamics {
	source := input.Source
	levels := SourceLevels{
		RMSDBFS:       finiteCopy(source.RMSDBFS),
		ActiveRMSDBFS: finiteCopy(source.ActiveRMSDBFS),
		PeakDBFS:      finiteCopy(source.PeakDBFS),
		CrestDB:       finiteCopy(source.CrestDB),
	}
	activity, rmsValues, peakValues, crestValues, durations := analyzeSegments(source.TimeSegments, source.DurationSeconds)
	macro := MacroDynamics{
		RMSDistribution:   distribution(rmsValues),
		PeakDistribution:  distribution(peakValues),
		CrestDistribution: distribution(crestValues),
	}
	if len(rmsValues) > 1 {
		rmsRange := round3(maximum(rmsValues) - minimum(rmsValues))
		macro.RMSRangeDB = &rmsRange
		rmsStdDev := round3(stddev(rmsValues))
		macro.RMSStdDevDB = &rmsStdDev
	}
	windowMS, hopMS := segmentResolution(source.TimeSegments, durations)
	timeScales := []TimeScaleCoverage{
		{Scale: ScaleMacroProgram, Status: timeScaleStatus(activity.ValidSegmentCount > 0), WindowMS: windowMS, HopMS: hopMS, SegmentCount: activity.ValidSegmentCount, Reason: missingReason(activity.ValidSegmentCount > 0, "macro_time_segments_missing")},
		{Scale: ScaleMicroTransient, Status: StatusMissing, Reason: "source_only_macro_evidence_cannot_resolve_transients"},
		{Scale: ScaleShortGainMotion, Status: StatusMissing, Reason: "paired_fine_envelope_required"},
		{Scale: ScaleEventRecovery, Status: StatusMissing, Reason: "paired_event_evidence_required"},
	}
	limitations := []string{
		"source_only_cannot_observe_compressor_behavior",
		"event_statistics_not_identifiable_from_macro_segments",
		"numeric_compressor_parameters_not_inferable_from_audio",
	}
	if activity.ValidSegmentCount == 0 {
		limitations = append(limitations, "macro_time_segments_missing")
	}
	if input.Conditions.EndSample <= input.Conditions.StartSample {
		limitations = append(limitations, "sample_index_window_missing")
	}
	if len(source.EvidenceRefs) == 0 {
		limitations = append(limitations, "source_evidence_refs_missing")
	}
	if !source.Quality.Nonzero || activity.ActiveSegmentCount == 0 {
		limitations = append(limitations, "source_has_no_confirmed_active_signal")
	}
	status := sourceDynamicsStatus(input, levels, activity)
	duration := source.DurationSeconds
	if duration <= 0 && input.Conditions.EndSeconds > input.Conditions.StartSeconds {
		duration = input.Conditions.EndSeconds - input.Conditions.StartSeconds
	}
	if duration <= 0 {
		duration = activity.AnalyzedSeconds
	}
	return SourceDynamics{
		SchemaVersion:     "com.source_dynamics.v1",
		Status:            status,
		DeclaredScale:     declaredScale(activity.ValidSegmentCount > 0),
		AnalyzedDuration:  round3(duration),
		Levels:            levels,
		Activity:          activity,
		MacroDynamics:     macro,
		TimeScaleCoverage: timeScales,
		EvidenceRefs:      append([]string(nil), source.EvidenceRefs...),
		Limitations:       uniqueSorted(limitations),
	}
}

func analyzeSegments(segments []TimeSegment, sourceDuration float64) (ActivityCoverage, []float64, []float64, []float64, []float64) {
	activity := ActivityCoverage{SegmentCount: len(segments)}
	rmsValues := []float64{}
	peakValues := []float64{}
	crestValues := []float64{}
	durations := []float64{}
	for _, segment := range segments {
		duration := segment.EndSeconds - segment.StartSeconds
		if !finite(duration) || duration <= 0 {
			activity.UnknownSegmentCount++
			continue
		}
		durations = append(durations, duration)
		hasRMS, rms := finiteValue(segment.RMSDBFS)
		hasPeak, peak := finiteValue(segment.PeakDBFS)
		hasCrest, crest := finiteValue(segment.CrestDB)
		state := strings.ToLower(strings.TrimSpace(segment.EnergyState))
		known := hasRMS || hasPeak || state != ""
		if !known {
			activity.UnknownSegmentCount++
			activity.UnknownSeconds += duration
			continue
		}
		activity.ValidSegmentCount++
		activity.AnalyzedSeconds += duration
		active := state != "silent" && ((hasRMS && rms > silenceFloorDBFS) || (hasPeak && peak > silenceFloorDBFS) || (state != "" && state != "silent"))
		if active {
			activity.ActiveSegmentCount++
			activity.ActiveSeconds += duration
			if hasRMS {
				rmsValues = append(rmsValues, rms)
			}
			if hasPeak {
				peakValues = append(peakValues, peak)
			}
			if hasCrest {
				crestValues = append(crestValues, crest)
			}
		} else {
			activity.SilentSegmentCount++
			activity.SilentSeconds += duration
		}
	}
	denominator := sourceDuration
	if denominator <= 0 {
		denominator = activity.AnalyzedSeconds + activity.UnknownSeconds
	}
	if denominator > 0 {
		activity.CoverageRatio = round3(clamp(activity.AnalyzedSeconds/denominator, 0, 1))
	}
	if activity.AnalyzedSeconds > 0 {
		activity.ActiveRatio = round3(clamp(activity.ActiveSeconds/activity.AnalyzedSeconds, 0, 1))
	}
	activity.AnalyzedSeconds = round3(activity.AnalyzedSeconds)
	activity.ActiveSeconds = round3(activity.ActiveSeconds)
	activity.SilentSeconds = round3(activity.SilentSeconds)
	activity.UnknownSeconds = round3(activity.UnknownSeconds)
	return activity, rmsValues, peakValues, crestValues, durations
}

func sourceDynamicsStatus(input Input, levels SourceLevels, activity ActivityCoverage) string {
	sourceStatus := normalizeStatus(input.Source.Status)
	if strings.EqualFold(strings.TrimSpace(input.Source.Freshness), StatusStale) || sourceStatus == StatusStale {
		return StatusStale
	}
	qualityStatus := strings.ToLower(strings.TrimSpace(input.Source.Quality.QualityStatus))
	if sourceStatus == StatusSuspect || input.Source.Quality.NaNInfCount > 0 || hasNonFiniteSource(input.Source) || qualityStatus == StatusSuspect || qualityStatus == "failed" {
		return StatusSuspect
	}
	usable := countLevels(levels) > 0 || activity.ValidSegmentCount > 0
	if !usable || sourceStatus == StatusMissing {
		return StatusMissing
	}
	if sourceStatus == StatusApproximate {
		return StatusApproximate
	}
	ready := sourceStatus == StatusReady &&
		strings.TrimSpace(input.Conditions.SourceRevision) != "" &&
		input.Conditions.EndSample > input.Conditions.StartSample &&
		input.Conditions.SampleRate > 0 && input.Conditions.ChannelCount > 0 &&
		len(input.Source.EvidenceRefs) > 0 &&
		input.Source.Quality.Nonzero &&
		activity.ValidSegmentCount > 0 &&
		activity.ActiveSegmentCount > 0
	if ready {
		return StatusReady
	}
	return StatusPartial
}

func buildSourceOnlyTrust(input Input, source SourceDynamics, mode string) TrustQuality {
	supportedMode := mode == ModeSourceOnly
	hasUsable := source.Status != StatusMissing
	gates := []QualityGate{
		gate("mode_source_only", true, supportedMode, unsupportedModeReason(supportedMode)),
		gate("source_evidence_present", true, hasUsable, "source_evidence_missing"),
		gateStatus("source_evidence_status", true, source.Status, "source_evidence_not_ready"),
		gate("source_revision", true, strings.TrimSpace(input.Conditions.SourceRevision) != "", "source_revision_missing"),
		gate("exact_sample_window", true, input.Conditions.EndSample > input.Conditions.StartSample, "sample_index_window_missing"),
		gate("evidence_refs", true, len(input.Source.EvidenceRefs) > 0, "source_evidence_refs_missing"),
		gate("sample_format", true, input.Conditions.SampleRate > 0 && input.Conditions.ChannelCount > 0, "sample_format_missing"),
		gate("finite_metrics", true, input.Source.Quality.NaNInfCount == 0 && !hasNonFiniteSource(input.Source), "source_contains_nan_or_inf"),
		gate("active_signal", true, input.Source.Quality.Nonzero && source.Activity.ActiveSegmentCount > 0, "active_signal_not_confirmed"),
		gate("macro_time_coverage", true, source.Activity.ValidSegmentCount > 0, "macro_time_segments_missing"),
	}
	blocked, approximate, suspect, stale, missing := classifyGates(gates)
	limitations := []string{
		"comparison_fields_not_applicable_source_only",
		"source_only_never_supports_behavior_observation",
		"source_only_never_supports_post_action_evaluation",
	}
	if !supportedMode {
		limitations = append(limitations, "com_1_supports_source_only_mode")
	}
	status := source.Status
	if !supportedMode {
		status = StatusMissing
	}
	canDescribe := supportedMode && (status == StatusReady || status == StatusPartial || status == StatusApproximate)
	return TrustQuality{
		OverallStatus: status,
		ModeGates:     gates,
		Coverage: map[string]any{
			"declared_scale":      source.DeclaredScale,
			"segment_count":       source.Activity.SegmentCount,
			"valid_segment_count": source.Activity.ValidSegmentCount,
			"coverage_ratio":      source.Activity.CoverageRatio,
			"active_ratio":        source.Activity.ActiveRatio,
		},
		SameSource:                     strings.TrimSpace(input.Conditions.SourceRevision) != "",
		SameWindow:                     input.Conditions.EndSample > input.Conditions.StartSample,
		SameFormat:                     input.Conditions.SampleRate > 0 && input.Conditions.ChannelCount > 0,
		CanSupportSourceDescription:    canDescribe,
		CanSupportBehaviorObservation:  false,
		CanSupportSemanticPlanning:     false,
		CanSupportPostActionEvaluation: false,
		BlockedReasons:                 blocked,
		ApproximateFields:              approximate,
		SuspectFields:                  suspect,
		StaleFields:                    stale,
		MissingFields:                  missing,
		Limitations:                    uniqueSorted(limitations),
		EvidenceRefs:                   append([]string(nil), input.Source.EvidenceRefs...),
	}
}

func buildSourceOnlyIdentifiability(source SourceDynamics) Identifiability {
	windowMS, hopMS := macroResolution(source.TimeScaleCoverage)
	sourceStatus := NotIdentifiable
	confidence := 0.0
	limitations := []string{}
	if source.Status == StatusReady {
		sourceStatus = Identified
		confidence = sourceConfidence(source)
	} else if source.Status == StatusPartial || source.Status == StatusApproximate {
		sourceStatus = Bounded
		confidence = sourceConfidence(source) * 0.75
		limitations = append(limitations, "source_dynamics_only_bounded_by_available_macro_evidence")
	} else {
		limitations = append(limitations, "source_evidence_not_usable_for_identification")
	}
	dimensions := []IdentifiabilityDimension{
		{
			Dimension: DimensionSourceDynamics, Status: sourceStatus, Basis: ModeSourceOnly,
			Confidence: round3(clamp(confidence, 0, 1)), Resolution: ResolutionEvidence{WindowMS: windowMS, HopMS: hopMS, SegmentCount: source.Activity.ValidSegmentCount},
			Supports: []string{"source_axis:macro_dynamics"}, DoesNotSupport: []string{"compressor_behavior", "parameter_values"}, Limitations: limitations,
		},
		notIdentifiableDimension(DimensionGainAction, "paired_input_output_evidence_required", "semantic_axis:activation_intensity", "parameter_value:threshold_db", "parameter_value:ratio"),
		notIdentifiableDimension(DimensionTransientResponse, "paired_fine_envelope_required", "semantic_axis:transient_timing", "parameter_value:attack_ms", "parameter_value:lookahead_ms"),
		notIdentifiableDimension(DimensionRecoveryMotion, "paired_event_recovery_evidence_required", "semantic_axis:recovery_sustain", "parameter_value:release_ms", "parameter_value:auto_release"),
		notIdentifiableDimension(DimensionLevelEffect, "paired_level_evidence_required", "semantic_axis:output_normalization", "parameter_value:makeup_gain_db"),
		notIdentifiableDimension(DimensionStereoBehavior, "paired_channel_evidence_required", "semantic_axis:channel_link", "parameter_value:channel_link"),
		notIdentifiableDimension(DimensionTriggerRelation, "paired_time_aligned_band_evidence_required", "semantic_axis:detector_focus", "parameter_value:sidechain_filter_hz"),
	}
	return Identifiability{Dimensions: dimensions}
}

func notIdentifiableDimension(dimension, reason, unsupportedAxis string, unsupported ...string) IdentifiabilityDimension {
	return IdentifiabilityDimension{
		Dimension:      dimension,
		Status:         NotIdentifiable,
		Basis:          ModeSourceOnly,
		Confidence:     0,
		Resolution:     ResolutionEvidence{},
		DoesNotSupport: append([]string{unsupportedAxis}, unsupported...),
		Limitations:    []string{reason, "numeric_parameter_inference_forbidden"},
	}
}

func sourceConfidence(source SourceDynamics) float64 {
	coverage := source.Activity.CoverageRatio
	if coverage <= 0 {
		coverage = 0.25
	}
	segmentFactor := math.Min(float64(source.Activity.ValidSegmentCount)/4, 1)
	return 0.45 + 0.35*coverage + 0.2*segmentFactor
}

func stableProjectionID(value Projection) string {
	copy := value
	copy.ProjectionID = ""
	copy.LLMContext = LLMContext{}
	copy.GeneratedAt = ""
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return "com_" + hex.EncodeToString(sum[:])[:20]
}

func distribution(values []float64) *Distribution {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return &Distribution{Count: len(sorted), Min: round3(sorted[0]), P50: round3(quantile(sorted, 0.5)), P90: round3(quantile(sorted, 0.9)), Max: round3(sorted[len(sorted)-1])}
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := clamp(q, 0, 1) * float64(len(sorted)-1)
	lo := int(math.Floor(position))
	hi := int(math.Ceil(position))
	if lo == hi {
		return sorted[lo]
	}
	fraction := position - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*fraction
}

func segmentResolution(segments []TimeSegment, durations []float64) (float64, float64) {
	if len(durations) == 0 {
		return 0, 0
	}
	sortedDurations := append([]float64(nil), durations...)
	sort.Float64s(sortedDurations)
	windowMS := quantile(sortedDurations, 0.5) * 1000
	hops := []float64{}
	for i := 1; i < len(segments); i++ {
		hop := segments[i].StartSeconds - segments[i-1].StartSeconds
		if finite(hop) && hop > 0 {
			hops = append(hops, hop)
		}
	}
	sort.Float64s(hops)
	hopMS := 0.0
	if len(hops) > 0 {
		hopMS = quantile(hops, 0.5) * 1000
	}
	return round3(windowMS), round3(hopMS)
}

func macroResolution(scales []TimeScaleCoverage) (float64, float64) {
	for _, scale := range scales {
		if scale.Scale == ScaleMacroProgram {
			return scale.WindowMS, scale.HopMS
		}
	}
	return 0, 0
}

func gate(id string, required, pass bool, reason string) QualityGate {
	status := StatusReady
	if !pass {
		status = StatusMissing
	}
	return QualityGate{ID: id, Required: required, Status: status, Reason: missingReason(pass, reason)}
}

func gateStatus(id string, required bool, status, reason string) QualityGate {
	status = normalizeStatus(status)
	return QualityGate{ID: id, Required: required, Status: status, Reason: missingReason(status == StatusReady, reason)}
}

func classifyGates(gates []QualityGate) (blocked, approximate, suspect, stale, missing []string) {
	for _, gate := range gates {
		if gate.Status == StatusReady {
			continue
		}
		field := gate.ID
		switch gate.Status {
		case StatusApproximate:
			approximate = append(approximate, field)
		case StatusSuspect:
			suspect = append(suspect, field)
		case StatusStale:
			stale = append(stale, field)
		default:
			missing = append(missing, field)
		}
		if gate.Required {
			blocked = append(blocked, firstNonEmpty(gate.Reason, gate.ID+"_not_ready"))
		}
	}
	return uniqueSorted(blocked), uniqueSorted(approximate), uniqueSorted(suspect), uniqueSorted(stale), uniqueSorted(missing)
}

func normalizeMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", ModeSourceOnly:
		return ModeSourceOnly
	case ModePairedIO:
		return ModePairedIO
	case ModeChangeDelta:
		return ModeChangeDelta
	default:
		return normalized
	}
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusReady:
		return StatusReady
	case StatusPartial:
		return StatusPartial
	case StatusStale:
		return StatusStale
	case StatusSuspect:
		return StatusSuspect
	case StatusApproximate, "approx":
		return StatusApproximate
	case StatusNotApplicable:
		return StatusNotApplicable
	default:
		return StatusMissing
	}
}

func declaredScale(hasSegments bool) string {
	if hasSegments {
		return ScaleMacroProgram
	}
	return ScaleWholeWindow
}

func timeScaleStatus(available bool) string {
	if available {
		return StatusReady
	}
	return StatusMissing
}

func unsupportedModeReason(supported bool) string {
	if supported {
		return ""
	}
	return "com_1_supports_source_only_mode"
}

func missingReason(pass bool, reason string) string {
	if pass {
		return ""
	}
	return reason
}

func countLevels(levels SourceLevels) int {
	count := 0
	for _, value := range []*float64{levels.RMSDBFS, levels.ActiveRMSDBFS, levels.PeakDBFS, levels.CrestDB} {
		if value != nil {
			count++
		}
	}
	return count
}

func finiteCopy(value *float64) *float64 {
	if value == nil || !finite(*value) {
		return nil
	}
	out := round3(*value)
	return &out
}

func finiteValue(value *float64) (bool, float64) {
	if value == nil || !finite(*value) {
		return false, 0
	}
	return true, *value
}

func hasNonFiniteSource(source SourceEvidence) bool {
	for _, value := range []*float64{source.RMSDBFS, source.ActiveRMSDBFS, source.PeakDBFS, source.CrestDB} {
		if value != nil && !finite(*value) {
			return true
		}
	}
	for _, segment := range source.TimeSegments {
		if !finite(segment.StartSeconds) || !finite(segment.EndSeconds) {
			return true
		}
		for _, value := range []*float64{segment.RMSDBFS, segment.PeakDBFS, segment.CrestDB} {
			if value != nil && !finite(*value) {
				return true
			}
		}
	}
	return false
}

func maximum(values []float64) float64 {
	out := values[0]
	for _, value := range values[1:] {
		if value > out {
			out = value
		}
	}
	return out
}

func minimum(values []float64) float64 {
	out := values[0]
	for _, value := range values[1:] {
		if value < out {
			out = value
		}
	}
	return out
}

func stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := 0.0
	for _, value := range values {
		mean += value
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, value := range values {
		delta := value - mean
		variance += delta * delta
	}
	return math.Sqrt(variance / float64(len(values)))
}

func round3(value float64) float64           { return math.Round(value*1000) / 1000 }
func finite(value float64) bool              { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func clamp(value, low, high float64) float64 { return math.Max(low, math.Min(high, value)) }

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	data, _ := json.Marshal(value)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}
