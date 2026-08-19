package dom

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func Build(input Input) Projection {
	input = normalizeInput(input)
	if strings.TrimSpace(input.Conditions.MeasurementKey) == "" {
		input.Conditions.MeasurementKey = input.Conditions.ComparabilityKey()
	}
	mode := strings.ToLower(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = ModeSourceOnly
	}
	if mode != ModeSourceOnly {
		return buildReservedModeProjection(input, mode)
	}

	peak := buildPeakStructure(input.Source)
	activity := buildActivityStructure(input.Conditions, input.Source)
	frequency := buildFrequencyTimeEvents(input.Conditions, input.Source)
	transient := buildTransientStructure(input.Conditions, input.Source)
	bandDynamics := buildBandDynamics(input.Conditions, input.Source)
	readiness := []DimensionReadiness{
		readinessFor("peak_structure", peak.Status, []string{"sample_peak_context", "crest_context", "headroom_context"}, peak.Limitations),
		readinessFor("activity_structure", activity.Status, []string{"declared_activity_context", "low_energy_interval_context"}, activity.Limitations),
		readinessFor("frequency_time_events", frequency.Status, []string{"whole_window_frequency_context"}, frequency.Limitations),
		readinessFor("transient_structure", transient.Status, []string{"macro_crest_context"}, transient.Limitations),
		readinessFor("band_dynamics", bandDynamics.Status, []string{"whole_window_band_context"}, bandDynamics.Limitations),
	}
	status := sourceProjectionStatus(input.Source, readiness)
	limitations := []string{
		"source_only_does_not_identify_or_recommend_a_processing_category",
		"source_only_does_not_observe_processor_behavior",
		"numeric_parameter_inference_forbidden",
	}
	for _, dimension := range readiness {
		limitations = append(limitations, dimension.Limitations...)
	}
	limitations = uniqueStrings(limitations)
	usableDimensions := 0
	for _, dimension := range readiness {
		if dimension.Status == StatusReady || dimension.Status == StatusPartial {
			usableDimensions++
		}
	}
	trust := TrustQuality{
		OverallStatus:                  status,
		CanSupportSourceDescription:    usableDimensions > 0 && status != StatusStale && status != StatusSuspect,
		CanSupportFamilySelection:      usableDimensions > 0 && status != StatusStale && status != StatusSuspect,
		CanSupportBehaviorObservation:  false,
		CanSupportPostActionEvaluation: false,
		Coverage: map[string]any{
			"usable_dimension_count":   lenReadyDimensions(readiness),
			"declared_dimension_count": len(readiness),
			"source_coverage_ratio":    input.Source.Quality.Coverage,
			"time_segment_count":       len(input.Source.TimeSegments),
			"whole_window_band_count":  len(input.Source.Bands),
		},
		Limitations: limitations,
	}
	if status == StatusMissing || status == StatusStale || status == StatusSuspect {
		trust.BlockedReasons = []string{"source_evidence_" + status}
	}
	projection := Projection{
		SchemaVersion:       SchemaVersion,
		DOMVersion:          Version,
		Mode:                ModeSourceOnly,
		Status:              status,
		ObservationID:       strings.TrimSpace(input.ObservationID),
		MixSessionID:        strings.TrimSpace(input.MixSessionID),
		TargetRef:           cloneMap(input.TargetRef),
		Conditions:          input.Conditions,
		PeakStructure:       &peak,
		ActivityStructure:   &activity,
		FrequencyTimeEvents: &frequency,
		TransientStructure:  &transient,
		BandDynamics:        &bandDynamics,
		DimensionReadiness:  readiness,
		TrustQuality:        trust,
		EvidenceRefs:        append([]string(nil), input.Source.EvidenceRefs...),
		Limitations:         limitations,
		GeneratedAt:         generatedAt(input.CreatedAt),
	}
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func buildReservedModeProjection(input Input, mode string) Projection {
	status := StatusMissing
	limitations := []string{"dom_v1_paired_behavior_materialization_not_implemented", "numeric_parameter_inference_forbidden"}
	blocked := []string{"paired_behavior_evidence_unavailable"}
	if mode != ModePairedIO && mode != ModeChangeDelta {
		status = StatusUnsupported
		limitations = append(limitations, "unsupported_dom_projection_mode")
		blocked = []string{"unsupported_dom_projection_mode"}
	} else if input.ProcessorScope == nil || strings.TrimSpace(input.ProcessorScope.Family) == "" {
		limitations = append(limitations, "bound_processor_family_required_after_family_selection")
		blocked = []string{"processor_scope_missing"}
	} else if !SupportedProcessorFamily(strings.ToLower(strings.TrimSpace(input.ProcessorScope.Family))) {
		status = StatusUnsupported
		limitations = append(limitations, "processor_family_outside_dom_v1_boundary")
		blocked = []string{"unsupported_processor_family"}
	}
	projection := Projection{
		SchemaVersion:  SchemaVersion,
		DOMVersion:     Version,
		Mode:           mode,
		Status:         status,
		ObservationID:  strings.TrimSpace(input.ObservationID),
		MixSessionID:   strings.TrimSpace(input.MixSessionID),
		TargetRef:      cloneMap(input.TargetRef),
		Conditions:     input.Conditions,
		ProcessorScope: cloneProcessorScope(input.ProcessorScope),
		DimensionReadiness: []DimensionReadiness{
			{Dimension: "processor_behavior", Status: status, Limitations: append([]string(nil), limitations...)},
			{Dimension: "behavior_change", Status: status, Limitations: append([]string(nil), limitations...)},
		},
		TrustQuality: TrustQuality{
			OverallStatus:  status,
			Coverage:       map[string]any{"implemented_modes": []string{ModeSourceOnly}, "reserved_modes": []string{ModePairedIO, ModeChangeDelta}},
			BlockedReasons: blocked,
			Limitations:    uniqueStrings(limitations),
		},
		EvidenceRefs: append([]string(nil), input.Source.EvidenceRefs...),
		Limitations:  uniqueStrings(limitations),
		GeneratedAt:  generatedAt(input.CreatedAt),
	}
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func buildPeakStructure(source SourceEvidence) PeakStructure {
	peaks := []float64{}
	crests := []float64{}
	atOrAbove := 0
	for _, segment := range source.TimeSegments {
		if value, ok := finitePointer(segment.PeakDBFS); ok {
			peaks = append(peaks, value)
			if value >= 0 {
				atOrAbove++
			}
		}
		if value, ok := finitePointer(segment.CrestDB); ok {
			crests = append(crests, value)
		}
	}
	status := StatusMissing
	if source.PeakDBFS != nil || source.HeadroomDB != nil || source.CrestDB != nil || len(peaks) > 0 || len(crests) > 0 {
		status = StatusReady
	}
	return PeakStructure{
		Status:                     status,
		PeakDBFS:                   finiteCopy(source.PeakDBFS),
		HeadroomDB:                 finiteCopy(source.HeadroomDB),
		CrestDB:                    finiteCopy(source.CrestDB),
		SegmentPeakDistribution:    distribution(peaks),
		SegmentCrestDistribution:   distribution(crests),
		AtOrAboveFullScaleSegments: atOrAbove,
		TruePeakStatus:             StatusMissing,
		EvidenceRefs:               append([]string(nil), source.EvidenceRefs...),
		Limitations:                []string{"sample_peak_does_not_establish_true_peak", "clip_events_are_not_inferred_from_peak_alone"},
	}
}

func buildActivityStructure(conditions Conditions, source SourceEvidence) ActivityStructure {
	noiseFloorStatus := StatusMissing
	if source.NoiseFloor != nil {
		noiseFloorStatus = normalizeEvidenceStatus(source.NoiseFloor.Status)
		if noiseFloorStatus == StatusReady && !validNoiseFloorEvidence(*source.NoiseFloor) {
			noiseFloorStatus = StatusPartial
		}
	}
	activity := ActivityStructure{SegmentCount: len(source.TimeSegments), NoiseFloorStatus: noiseFloorStatus, EvidenceRefs: append([]string(nil), source.EvidenceRefs...)}
	if source.NoiseFloor != nil {
		activity.EvidenceRefs = uniqueStrings(append(activity.EvidenceRefs, source.NoiseFloor.EvidenceRefs...))
	}
	validSeconds := 0.0
	activeSeconds := 0.0
	lowSeconds := 0.0
	currentLowRun := 0.0
	lowRuns := []float64{}
	flushLowRun := func() {
		if currentLowRun > 0 {
			lowRuns = append(lowRuns, currentLowRun)
			currentLowRun = 0
		}
	}
	for _, segment := range source.TimeSegments {
		duration := segment.EndSeconds - segment.StartSeconds
		if !finite(duration) || duration <= 0 {
			activity.UnknownSegmentCount++
			flushLowRun()
			continue
		}
		state := strings.ToLower(strings.TrimSpace(segment.EnergyState))
		switch state {
		case "active", "medium", "high":
			activity.ValidSegmentCount++
			activity.ActiveSegmentCount++
			validSeconds += duration
			activeSeconds += duration
			flushLowRun()
		case "low", "quiet":
			activity.ValidSegmentCount++
			activity.LowEnergyCount++
			validSeconds += duration
			lowSeconds += duration
			currentLowRun += duration
		case "silent", "silence":
			activity.ValidSegmentCount++
			activity.SilentSegmentCount++
			validSeconds += duration
			lowSeconds += duration
			currentLowRun += duration
		default:
			activity.UnknownSegmentCount++
			flushLowRun()
		}
	}
	flushLowRun()
	if source.Duration > 0 {
		activity.CoverageRatio = round(validSeconds / source.Duration)
	} else if len(source.TimeSegments) > 0 {
		activity.CoverageRatio = round(float64(activity.ValidSegmentCount) / float64(len(source.TimeSegments)))
	}
	if validSeconds > 0 {
		activity.ActiveRatio = round(activeSeconds / validSeconds)
		activity.LowOrSilentRatio = round(lowSeconds / validSeconds)
	}
	activity.LowRunSeconds = distribution(lowRuns)
	if activity.ValidSegmentCount > 0 && activity.CoverageRatio > 0 && noiseFloorStatus == StatusReady {
		activity.Status = StatusReady
	} else if activity.ValidSegmentCount > 0 {
		activity.Status = StatusPartial
	} else {
		activity.Status = StatusMissing
	}
	activity.Limitations = []string{"control_threshold_or_range_cannot_be_inferred"}
	if noiseFloorStatus != StatusReady {
		activity.Limitations = append([]string{"noise_floor_not_identifiable_from_declared_energy_states"}, activity.Limitations...)
	}
	return activity
}

func buildFrequencyTimeEvents(conditions Conditions, source SourceEvidence) FrequencyTimeEvents {
	status := StatusMissing
	limitations := []string{"time_localized_frequency_events_missing", "narrowband_high_frequency_events_not_identifiable_from_whole_window_bands"}
	if len(source.Bands) > 0 {
		status = StatusPartial
	}
	evidenceRefs := append([]string(nil), source.EvidenceRefs...)
	timeLocalized := false
	eventCountAvailable := false
	if source.Frequency != nil {
		status = normalizeEvidenceStatus(source.Frequency.Status)
		timeLocalized = len(source.Frequency.Events) > 0
		eventCountAvailable = source.Frequency.EventCountAvailable
		evidenceRefs = uniqueStrings(append(evidenceRefs, source.Frequency.EvidenceRefs...))
		if status == StatusReady && (source.Frequency.Coverage < 0.5 || !eventCountAvailable || len(source.Frequency.Events) == 0 || !frequencyEventsWithinRange(source.Frequency.Events, conditions, source.Duration)) {
			status = StatusPartial
		}
		if status == StatusReady {
			limitations = []string{}
		} else if status == StatusPartial {
			limitations = []string{"time_localized_frequency_events_partial"}
		}
	}
	return FrequencyTimeEvents{Status: status, WholeWindowBands: append([]BandEvidence(nil), source.Bands...),
		TimeLocalized: timeLocalized, EventCountAvailable: eventCountAvailable, EvidenceRefs: evidenceRefs, Limitations: limitations}
}

func frequencyEventsWithinRange(events []FrequencyEvent, conditions Conditions, duration float64) bool {
	start, end, known := measurementTimeRange(conditions, duration)
	if !known {
		return false
	}
	for _, event := range events {
		if !finite(event.StartSeconds) || !finite(event.EndSeconds) || event.StartSeconds < start-0.01 || event.EndSeconds > end+0.01 {
			return false
		}
	}
	return true
}

func measurementTimeRange(conditions Conditions, duration float64) (float64, float64, bool) {
	if conditions.EndSeconds > conditions.StartSeconds {
		return conditions.StartSeconds, conditions.EndSeconds, true
	}
	if duration > 0 {
		return 0, duration, true
	}
	return 0, 0, false
}

func buildTransientStructure(conditions Conditions, source SourceEvidence) TransientStructure {
	crests := []float64{}
	for _, segment := range source.TimeSegments {
		if value, ok := finitePointer(segment.CrestDB); ok {
			crests = append(crests, value)
		}
	}
	status := StatusMissing
	if len(crests) > 0 || source.CrestDB != nil {
		status = StatusPartial
	}
	onsetReady, attackBodyReady, sustainDecayReady := false, false, false
	evidenceRefs := append([]string(nil), source.EvidenceRefs...)
	if source.Transient != nil {
		status = normalizeEvidenceStatus(source.Transient.Status)
		evidenceRefs = uniqueStrings(append(evidenceRefs, source.Transient.EvidenceRefs...))
		completeCount := 0
		for _, event := range source.Transient.Events {
			if completeTransientEvent(event) && transientEventWithinRange(event, conditions, source.Duration) {
				completeCount++
			}
		}
		onsetReady = completeCount >= 1
		attackBodyReady = onsetReady
		sustainDecayReady = onsetReady
		if status == StatusReady && (!onsetReady || !attackBodyReady || !sustainDecayReady) {
			status = StatusPartial
		}
	}
	limitations := []string{}
	if !onsetReady {
		limitations = append(limitations, "macro_crest_does_not_identify_onset_events")
	}
	if !attackBodyReady || !sustainDecayReady {
		limitations = append(limitations, "attack_body_and_sustain_decay_evidence_missing")
	}
	return TransientStructure{Status: status, SegmentCrest: distribution(crests), OnsetEventsReady: onsetReady,
		AttackBodyReady: attackBodyReady, SustainDecayReady: sustainDecayReady, EvidenceRefs: evidenceRefs,
		Limitations: limitations}
}

func completeTransientEvent(event TransientEvent) bool {
	return finite(event.OnsetSeconds) && event.BodyEndSeconds > 0 && event.SustainEndSeconds > 0 &&
		event.OnsetDBFS != nil && event.BodyDBFS != nil && event.SustainDBFS != nil &&
		event.AttackBodyContrastDB != nil && event.SustainDecayDB != nil
}

func transientEventWithinRange(event TransientEvent, conditions Conditions, duration float64) bool {
	start, end, known := measurementTimeRange(conditions, duration)
	if !known {
		return false
	}
	return finite(event.OnsetSeconds) && finite(event.BodyEndSeconds) && finite(event.SustainEndSeconds) &&
		event.OnsetSeconds >= start-0.01 && event.SustainEndSeconds <= end+0.01
}

func validNoiseFloorEvidence(evidence NoiseFloorEvidence) bool {
	return evidence.EstimateDBFS != nil && evidence.P10DBFS != nil && evidence.P50DBFS != nil && evidence.WindowCount > 0 && strings.TrimSpace(evidence.Method) != ""
}

func buildBandDynamics(conditions Conditions, source SourceEvidence) BandDynamics {
	status := StatusMissing
	if len(source.Bands) > 0 {
		status = StatusPartial
	}
	timeReady, crestReady, crossBandReady := false, false, false
	evidenceRefs := append([]string(nil), source.EvidenceRefs...)
	byID := map[string]BandDynamicsEvidence{}
	for _, row := range source.BandDynamicsEvidence {
		evidenceRefs = append(evidenceRefs, row.EvidenceRefs...)
		if row.ID != "" {
			byID[row.ID] = row
		}
	}
	required := map[string]bool{}
	for _, band := range source.Bands {
		if band.ID != "" {
			required[band.ID] = true
		}
	}
	if len(required) == 0 {
		for id := range byID {
			required[id] = true
		}
	}
	readyCount := 0
	allReady := len(required) > 0
	timeReady = len(required) > 0
	crestReady = len(required) > 0
	for id := range required {
		row, ok := byID[id]
		if !ok {
			allReady = false
			timeReady = false
			crestReady = false
			continue
		}
		if row.TimeDistribution == nil {
			timeReady = false
		}
		if row.CrestDistribution == nil {
			crestReady = false
		}
		if row.TimeDistribution != nil && row.CrestDistribution != nil && normalizeEvidenceStatus(row.Status) == StatusReady {
			readyCount++
		} else {
			allReady = false
		}
	}
	if allReady && timeReady && crestReady && readyCount >= 2 {
		status = StatusReady
		crossBandReady = true
	} else if len(required) > 0 {
		status = StatusPartial
	}
	limitations := []string{}
	if !timeReady {
		limitations = append(limitations, "whole_window_band_energy_has_no_per_band_time_distribution")
	}
	if !crestReady {
		limitations = append(limitations, "per_band_crest_and_cross_band_motion_missing")
	} else if !crossBandReady {
		limitations = append(limitations, "cross_band_relation_evidence_missing")
	}
	_ = conditions
	return BandDynamics{Status: status, Bands: append([]BandEvidence(nil), source.Bands...), TimeVaryingReady: timeReady,
		PerBandCrestReady: crestReady, CrossBandReady: crossBandReady, EvidenceRefs: uniqueStrings(evidenceRefs), Limitations: limitations}
}

func normalizeEvidenceStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case StatusReady, StatusPartial, StatusMissing, StatusStale, StatusSuspect, StatusUnsupported:
		return status
	default:
		return StatusMissing
	}
}

func normalizeInput(input Input) Input {
	input.TargetRef = cloneMap(input.TargetRef)
	input.Source.EvidenceRefs = uniqueStrings(input.Source.EvidenceRefs)
	sort.SliceStable(input.Source.TimeSegments, func(i, j int) bool {
		return input.Source.TimeSegments[i].StartSeconds < input.Source.TimeSegments[j].StartSeconds
	})
	sort.SliceStable(input.Source.Bands, func(i, j int) bool {
		if input.Source.Bands[i].MinHz == input.Source.Bands[j].MinHz {
			return input.Source.Bands[i].ID < input.Source.Bands[j].ID
		}
		return input.Source.Bands[i].MinHz < input.Source.Bands[j].MinHz
	})
	if input.ProcessorScope != nil {
		input.ProcessorScope.Family = strings.ToLower(strings.TrimSpace(input.ProcessorScope.Family))
	}
	return input
}

func sourceProjectionStatus(source SourceEvidence, dimensions []DimensionReadiness) string {
	status := strings.ToLower(strings.TrimSpace(source.Status))
	if strings.EqualFold(source.Freshness, StatusStale) || status == StatusStale {
		return StatusStale
	}
	if status == StatusSuspect || source.Quality.NaNInfCount > 0 {
		return StatusSuspect
	}
	if lenReadyDimensions(dimensions) == 0 || status == StatusMissing {
		return StatusMissing
	}
	for _, dimension := range dimensions {
		if dimension.Status != StatusReady {
			return StatusPartial
		}
	}
	if status == StatusReady {
		return StatusReady
	}
	return StatusPartial
}

func readinessFor(dimension, status string, supports, limitations []string) DimensionReadiness {
	if status == StatusMissing {
		supports = nil
	}
	return DimensionReadiness{Dimension: dimension, Status: status, Supports: supports, Limitations: append([]string(nil), limitations...)}
}

func lenReadyDimensions(values []DimensionReadiness) int {
	count := 0
	for _, value := range values {
		if value.Status == StatusReady || value.Status == StatusPartial {
			count++
		}
	}
	return count
}

func distribution(values []float64) *Distribution {
	clean := make([]float64, 0, len(values))
	for _, value := range values {
		if finite(value) {
			clean = append(clean, value)
		}
	}
	if len(clean) == 0 {
		return nil
	}
	sort.Float64s(clean)
	return &Distribution{Count: len(clean), Min: round(clean[0]), P50: round(quantile(clean, .5)), P90: round(quantile(clean, .9)), Max: round(clean[len(clean)-1])}
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	position := q * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	weight := position - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

func stableProjectionID(projection Projection) string {
	copy := projection
	copy.ProjectionID = ""
	copy.GeneratedAt = ""
	copy.LLMContext = LLMContext{}
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return "dom_" + hex.EncodeToString(sum[:])[:20]
}

func generatedAt(value string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func finitePointer(value *float64) (float64, bool) {
	if value == nil || !finite(*value) {
		return 0, false
	}
	return *value, true
}

func finiteCopy(value *float64) *float64 {
	if number, ok := finitePointer(value); ok {
		return &number
	}
	return nil
}

func finite(value float64) bool   { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func round(value float64) float64 { return math.Round(value*1000) / 1000 }

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneProcessorScope(input *ProcessorScope) *ProcessorScope {
	if input == nil {
		return nil
	}
	out := *input
	return &out
}

func buildLLMContext(projection Projection) LLMContext {
	facts := []map[string]any{
		{"layer": "trust_quality", "status": projection.TrustQuality.OverallStatus, "can_support_source_description": projection.TrustQuality.CanSupportSourceDescription, "can_support_family_selection": projection.TrustQuality.CanSupportFamilySelection, "can_support_behavior_observation": projection.TrustQuality.CanSupportBehaviorObservation},
	}
	for _, dimension := range projection.DimensionReadiness {
		facts = append(facts, map[string]any{"layer": "dimension_readiness", "dimension": dimension.Dimension, "status": dimension.Status})
	}
	if peak := projection.PeakStructure; peak != nil && peak.Status != StatusMissing {
		fact := map[string]any{"layer": "peak_structure", "status": peak.Status, "true_peak_status": peak.TruePeakStatus, "at_or_above_full_scale_segment_count": peak.AtOrAboveFullScaleSegments}
		addPointer(fact, "peak_dbfs", peak.PeakDBFS)
		addPointer(fact, "headroom_db", peak.HeadroomDB)
		addPointer(fact, "crest_db", peak.CrestDB)
		facts = append(facts, fact)
	}
	if activity := projection.ActivityStructure; activity != nil && activity.Status != StatusMissing {
		facts = append(facts, map[string]any{"layer": "activity_structure", "status": activity.Status, "valid_segment_count": activity.ValidSegmentCount, "active_ratio": activity.ActiveRatio, "low_or_silent_ratio": activity.LowOrSilentRatio, "noise_floor_status": activity.NoiseFloorStatus})
	}
	if frequency := projection.FrequencyTimeEvents; frequency != nil {
		facts = append(facts, map[string]any{"layer": "frequency_time_events", "status": frequency.Status, "whole_window_band_count": len(frequency.WholeWindowBands), "time_localized": frequency.TimeLocalized, "event_count_available": frequency.EventCountAvailable})
	}
	if transient := projection.TransientStructure; transient != nil {
		facts = append(facts, map[string]any{"layer": "transient_structure", "status": transient.Status, "onset_events_ready": transient.OnsetEventsReady, "attack_body_contrast_ready": transient.AttackBodyReady, "sustain_decay_ready": transient.SustainDecayReady})
	}
	if bands := projection.BandDynamics; bands != nil {
		facts = append(facts, map[string]any{"layer": "band_dynamics", "status": bands.Status, "band_count": len(bands.Bands), "time_varying_ready": bands.TimeVaryingReady, "per_band_crest_ready": bands.PerBandCrestReady, "cross_band_relation_ready": bands.CrossBandReady})
	}
	summary := fmt.Sprintf("DOM %s %s projection exposes bounded, family-neutral source evidence; it does not recommend a processor or infer controls.", projection.Status, projection.Mode)
	next := "Use only the dimensions relevant to the current acoustic question and retain every missing or partial limitation."
	if projection.Mode != ModeSourceOnly {
		summary = fmt.Sprintf("DOM %s %s projection reserves the governed dynamics-processor behavior boundary; behavior evidence is not materialized in this release.", projection.Status, projection.Mode)
		next = "Do not infer behavior or parameter values while paired evidence is unavailable."
	}
	return LLMContext{SummaryMD: summary, CompactFacts: facts, DoNotIncludeRawPackage: true,
		EvidenceRefs: append([]string(nil), projection.EvidenceRefs...), LimitationNotes: append([]string(nil), projection.Limitations...), SuggestedNextStep: next}
}

func addPointer(target map[string]any, key string, value *float64) {
	if number, ok := finitePointer(value); ok {
		target[key] = number
	}
}
