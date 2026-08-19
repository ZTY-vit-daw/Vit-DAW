package mixboard

import (
	"encoding/json"
	"math"
	"strings"

	"vit-daw-agent/internal/dom"
)

func finalizeDOMProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	input := domInputFromObservation(*obs, req)
	projection := dom.Build(input)
	obs.DOMProjection = &projection
}

func domInputFromObservation(obs ObservationPacket, req Request) dom.Input {
	mode := strings.ToLower(strings.TrimSpace(cleanAnyString(req.Args["dom_mode"])))
	if mode == "" {
		mode = dom.ModeSourceOnly
	}
	snapshot := mapValue(obs.GlobalSummary["feature_snapshot"])
	waveform := mapValue(snapshot["waveform_envelope"])
	metrics := mapValue(obs.MixPackage["current_metrics"])
	if len(waveform) == 0 {
		waveform = mapValue(metrics["waveform"])
	}
	bandSummary := mapValue(snapshot["band_energy_summary"])
	if len(bandSummary) == 0 {
		bandSummary = mapValue(metrics["band_energy"])
	}
	if targetNeedsMatchingRow(obs.TargetRef) {
		if !featureRowMatchesTarget(waveform, obs.TargetRef) {
			waveform = featureRowForTarget(snapshot["track_waveform_envelopes"], obs.TargetRef)
		}
		if !featureRowMatchesTarget(bandSummary, obs.TargetRef) {
			bandSummary = featureRowForTarget(snapshot["band_energy_summaries"], obs.TargetRef)
		}
	}
	if len(waveform) == 0 {
		waveform = mapValue(metrics["waveform"])
	}
	if len(bandSummary) == 0 {
		bandSummary = mapValue(metrics["band_energy"])
	}

	segments := waveformTimeSegments(waveform)
	domSegments := make([]dom.TimeSegment, 0, len(segments))
	for _, segment := range segments {
		domSegments = append(domSegments, dom.TimeSegment{
			StartSeconds: numberFromMap(segment, "start_seconds"),
			EndSeconds:   numberFromMap(segment, "end_seconds"),
			RMSDBFS:      comNumberPointer(segment, "rms_dbfs"),
			PeakDBFS:     comNumberPointer(segment, "peak_dbfs"),
			CrestDB:      comNumberPointer(segment, "crest_db"),
			EnergyState:  cleanAnyString(segment["energy_state"]),
		})
	}
	domBands := []dom.BandEvidence{}
	bands := mapValue(bandSummary["bands"])
	for _, id := range []string{"sub", "bass", "low_mid", "mid", "presence", "air"} {
		row := mapValue(bands[id])
		if len(row) == 0 {
			continue
		}
		domBands = append(domBands, dom.BandEvidence{
			ID:         id,
			Status:     firstNonEmpty(cleanAnyString(row["status"]), featureStatus(bandSummary)),
			MinHz:      numberFromMap(row, "min_hz"),
			MaxHz:      numberFromMap(row, "max_hz"),
			UnitEnergy: optionalNumberPointer(row, "unit_energy"),
			EnergyDB:   optionalNumberPointer(row, "energy_db"),
		})
	}

	status := featureStatus(waveform)
	if (status == "missing" || status == "") && (featureStatus(bandSummary) == "ready" || featureStatus(bandSummary) == "partial") {
		status = dom.StatusPartial
	}
	coverage := numberFromMap(waveform, "coverage_ratio")
	if coverage <= 0 {
		coverage = numberFromMap(bandSummary, "coverage_ratio")
	}
	if coverage <= 0 && status == dom.StatusReady {
		coverage = 1
	}
	duration := numberFromMap(waveform, "duration_seconds")
	if duration <= 0 {
		duration = numberFromMap(bandSummary, "duration_seconds")
	}
	if duration <= 0 {
		duration = obs.TimeRuler.DurationSeconds
	}
	sampleRate := numberFromMap(waveform, "sample_rate")
	if sampleRate <= 0 {
		sampleRate = numberFromMap(bandSummary, "sample_rate")
	}
	if sampleRate <= 0 {
		sampleRate = firstPositiveFloat(req.Args, "sample_rate")
	}
	channelCount := int(numberFromMap(waveform, "channel_count"))
	if channelCount <= 0 {
		channelCount = int(numberFromMap(bandSummary, "channel_count"))
	}
	if channelCount <= 0 {
		channelCount = int(firstPositiveFloat(req.Args, "channel_count"))
	}
	startSample := int64(0)
	endSample := int64(numberFromMap(waveform, "analyzed_sample_count"))
	if endSample <= 0 {
		endSample = int64(numberFromMap(bandSummary, "analyzed_sample_count"))
	}
	startSeconds, endSeconds := 0.0, 0.0
	analyzedRange := mapValue(waveform["analyzed_range"])
	if len(analyzedRange) == 0 {
		analyzedRange = mapValue(bandSummary["analyzed_range"])
	}
	if len(analyzedRange) > 0 {
		startSample = int64(numberFromMap(analyzedRange, "start_sample"))
		endSample = int64(numberFromMap(analyzedRange, "end_sample"))
		startSeconds = numberFromMap(analyzedRange, "start_seconds")
		endSeconds = numberFromMap(analyzedRange, "end_seconds")
	}
	if len(analyzedRange) == 0 && duration > 0 {
		startSeconds = 0
		endSeconds = duration
	}
	if endSample <= startSample && sampleRate > 0 && duration > 0 {
		endSample = startSample + int64(math.Round(sampleRate*duration))
	}
	if endSeconds <= startSeconds && sampleRate > 0 && endSample > startSample {
		startSeconds = float64(startSample) / positiveOrOne(sampleRate)
		endSeconds = float64(endSample) / positiveOrOne(sampleRate)
	}
	if endSeconds <= startSeconds && duration > 0 {
		startSeconds = 0
		endSeconds = duration
	}
	windowMS := numberFromMap(waveform, "window_ms")
	if windowMS <= 0 {
		windowMS = numberFromMap(bandSummary, "window_ms")
	}
	if windowMS <= 0 {
		frame := numberFromMap(bandSummary, "frame_duration_seconds")
		if frame <= 0 {
			frame = numberFromMap(waveform, "frame_duration_seconds")
		}
		if frame > 0 {
			windowMS = frame * 1000
		}
	}
	hopMS := numberFromMap(waveform, "hop_ms")
	if hopMS <= 0 {
		hopMS = numberFromMap(bandSummary, "hop_ms")
	}
	evidenceRefs := []string{}
	for _, source := range []map[string]any{waveform, bandSummary} {
		if ref := cleanAnyString(source["evidence_ref"]); ref != "" {
			evidenceRefs = append(evidenceRefs, ref)
		}
	}
	noiseFloor := domNoiseFloorEvidence(waveform, bandSummary, evidenceRefs)
	frequencyEvidence := domFrequencyEvidence(waveform, bandSummary, evidenceRefs)
	transientEvidence := domTransientEvidence(waveform, bandSummary, evidenceRefs)
	bandDynamicsEvidence := domBandDynamicsEvidence(waveform, bandSummary)
	sourceFeatures := []string{}
	if len(waveform) > 0 {
		sourceFeatures = append(sourceFeatures, "waveform_envelope")
	}
	if len(domSegments) > 0 {
		sourceFeatures = append(sourceFeatures, "time_energy_segments")
	}
	if len(domBands) > 0 {
		sourceFeatures = append(sourceFeatures, "whole_window_band_energy")
	}
	if noiseFloor != nil {
		sourceFeatures = append(sourceFeatures, "noise_floor_percentiles")
	}
	if frequencyEvidence != nil {
		sourceFeatures = append(sourceFeatures, "frequency_time_events")
	}
	if transientEvidence != nil {
		sourceFeatures = append(sourceFeatures, "transient_events")
	}
	if len(bandDynamicsEvidence) > 0 {
		sourceFeatures = append(sourceFeatures, "band_dynamics_distributions")
	}
	conditions := dom.Conditions{
		ProjectRevision: firstNonEmpty(cleanAnyString(obs.ProjectPackage["project_revision"]), cleanAnyString(obs.ProjectPackage["project_uuid"])),
		SourceRevision:  firstNonEmpty(cleanAnyString(waveform["source_revision"]), cleanAnyString(bandSummary["source_revision"])),
		ClipRevision:    firstNonEmpty(cleanAnyString(waveform["clip_revision"]), cleanAnyString(bandSummary["clip_revision"])),
		StartSample:     startSample, EndSample: endSample,
		StartSeconds: startSeconds, EndSeconds: endSeconds,
		SampleRate: sampleRate, ChannelCount: channelCount,
		AnalyzerVersion: firstNonEmpty(cleanAnyString(waveform["analyzer_version"]), cleanAnyString(waveform["analyzer_revision"]),
			cleanAnyString(bandSummary["analyzer_version"]), cleanAnyString(bandSummary["analyzer_revision"])),
		TapPoint:       domTapPoint(waveform, bandSummary),
		WindowMS:       windowMS,
		HopMS:          hopMS,
		RenderMode:     firstNonEmpty(cleanAnyString(waveform["render_mode"]), cleanAnyString(bandSummary["render_mode"])),
		MeasurementKey: "",
	}
	conditions.MeasurementKey = conditions.ComparabilityKey()
	input := dom.Input{
		Mode:          mode,
		ObservationID: obs.ObservationID,
		MixSessionID:  obs.MixSessionID,
		CreatedAt:     obs.CreatedAt,
		TargetRef:     map[string]any{"kind": obs.TargetRef.Kind, "id": obs.TargetRef.ID, "label": obs.TargetRef.Label},
		Conditions:    conditions,
		Source: dom.SourceEvidence{
			SchemaVersion: "dad.dynamic_source_evidence.v1", Status: status, Freshness: domSourceFreshness(waveform, bandSummary), Duration: duration,
			RMSDBFS: comNumberPointer(waveform, "rms_dbfs"), PeakDBFS: comNumberPointer(waveform, "peak_dbfs"),
			HeadroomDB: comNumberPointer(waveform, "headroom_db"), CrestDB: comNumberPointer(waveform, "crest_db"),
			TimeSegments: domSegments, Bands: domBands,
			NoiseFloor: noiseFloor, Frequency: frequencyEvidence, Transient: transientEvidence,
			BandDynamicsEvidence: bandDynamicsEvidence,
			Quality: dom.Quality{Status: firstNonEmpty(cleanAnyString(waveform["quality_status"]), status),
				Nonzero:  numberFromMap(waveform, "nonzero_count") > 0 || numberFromMap(waveform, "sum_abs") > 0 || numberFromMap(waveform, "peak_abs") > 0,
				Coverage: coverage, NaNInfCount: int(numberFromMap(waveform, "nan_count") + numberFromMap(waveform, "inf_count"))},
			EvidenceRefs: evidenceRefs, SourceFeatures: sourceFeatures,
		},
	}
	if raw := req.Args["dom_processor_scope"]; raw != nil && mode != dom.ModeSourceOnly {
		data, err := json.Marshal(raw)
		if err == nil {
			var scope dom.ProcessorScope
			if json.Unmarshal(data, &scope) == nil {
				input.ProcessorScope = &scope
			}
		}
	}
	return input
}

func targetNeedsMatchingRow(target TargetRef) bool {
	switch strings.ToLower(strings.TrimSpace(target.Kind)) {
	case "track", "clip":
		return true
	default:
		return false
	}
}

func featureRowMatchesTarget(row map[string]any, target TargetRef) bool {
	if len(row) == 0 {
		return false
	}
	if !targetNeedsMatchingRow(target) {
		return true
	}
	id := strings.TrimSpace(cleanAnyString(target.ID))
	if id == "" {
		return true
	}
	return cleanAnyString(row["track_id"]) == id || cleanAnyString(row["id"]) == id || cleanAnyString(row["clip_id"]) == id
}

func featureRowForTarget(value any, target TargetRef) map[string]any {
	rows := mapRowsAny(value)
	id := strings.TrimSpace(cleanAnyString(target.ID))
	if id == "" {
		return nil
	}
	for _, row := range rows {
		if cleanAnyString(row["track_id"]) == id || cleanAnyString(row["id"]) == id || cleanAnyString(row["clip_id"]) == id {
			return row
		}
	}
	return nil
}

func domTapPoint(sources ...map[string]any) string {
	for _, row := range sources {
		if tap := strings.ToLower(strings.TrimSpace(cleanAnyString(row["tap_point"]))); tap != "" {
			return tap
		}
	}
	for _, row := range sources {
		source := strings.ToLower(firstNonEmpty(cleanAnyString(row["source"]), cleanAnyString(row["source_kind"]), cleanAnyString(row["layer"])))
		if strings.Contains(source, "l2_render_probe") || strings.Contains(source, "render_probe") || strings.Contains(source, "offline_probe") {
			return "unknown_post_chain"
		}
		if strings.Contains(source, "kernel_l3_offline_analyzer") || strings.Contains(source, "spectral_tile_derived") || strings.Contains(source, "l3_deep") || strings.Contains(source, "spectral_field") {
			return "source_file_pre_fx"
		}
	}
	return ""
}

func domEvidenceSource(primary, secondary map[string]any, key string) map[string]any {
	if value := mapValue(primary[key]); len(value) > 0 {
		return value
	}
	return mapValue(secondary[key])
}

func domEvidenceSourceAny(primary, secondary map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		if row := domEvidenceSource(primary, secondary, key); len(row) > 0 {
			return row
		}
	}
	return nil
}

func domNoiseFloorEvidence(primary, secondary map[string]any, refs []string) *dom.NoiseFloorEvidence {
	row := domEvidenceSourceAny(primary, secondary, "noise_floor_evidence", "noise_floor")
	if len(row) == 0 {
		return nil
	}
	return &dom.NoiseFloorEvidence{Status: firstNonEmpty(cleanAnyString(row["status"]), featureStatus(row)),
		EstimateDBFS: optionalNumberPointer(row, "estimate_dbfs"), P10DBFS: optionalNumberPointer(row, "p10_dbfs"),
		P50DBFS: optionalNumberPointer(row, "p50_dbfs"), Method: cleanAnyString(row["method"]),
		Confidence: cleanAnyString(row["confidence"]), WindowCount: int(numberFromMap(row, "window_count")), EvidenceRefs: uniqueSortedStrings(append(refs, stringRows(row["evidence_refs"])...))}
}

func domFrequencyEvidence(primary, secondary map[string]any, refs []string) *dom.FrequencyEventEvidence {
	row := domEvidenceSourceAny(primary, secondary, "frequency_time_events", "frequency_events")
	if len(row) == 0 {
		return nil
	}
	evidence := &dom.FrequencyEventEvidence{Status: firstNonEmpty(cleanAnyString(row["status"]), featureStatus(row)), Coverage: numberFromMap(row, "coverage"), EventCountAvailable: boolFromAny(row["event_count_available"]) || len(mapRowsAny(row["events"])) > 0, EvidenceRefs: uniqueSortedStrings(append(refs, stringRows(row["evidence_refs"])...))}
	for _, raw := range mapRowsAny(row["events"]) {
		event := dom.FrequencyEvent{StartSeconds: numberFromMap(raw, "start_seconds"), EndSeconds: numberFromMap(raw, "end_seconds"), BandID: cleanAnyString(raw["band_id"]), MinHz: numberFromMap(raw, "min_hz"), MaxHz: numberFromMap(raw, "max_hz"), LevelDBFS: optionalNumberPointer(raw, "level_dbfs"), ContrastDB: optionalNumberPointer(raw, "contrast_db")}
		evidence.Events = append(evidence.Events, event)
	}
	return evidence
}

func domTransientEvidence(primary, secondary map[string]any, refs []string) *dom.TransientEventEvidence {
	row := domEvidenceSourceAny(primary, secondary, "transient_events", "transient")
	if len(row) == 0 {
		return nil
	}
	evidence := &dom.TransientEventEvidence{Status: firstNonEmpty(cleanAnyString(row["status"]), featureStatus(row)), Coverage: numberFromMap(row, "coverage"), EvidenceRefs: uniqueSortedStrings(append(refs, stringRows(row["evidence_refs"])...))}
	for _, raw := range mapRowsAny(row["events"]) {
		evidence.Events = append(evidence.Events, dom.TransientEvent{OnsetSeconds: numberFromMap(raw, "onset_seconds"), BodyEndSeconds: numberFromMap(raw, "body_end_seconds"), SustainEndSeconds: numberFromMap(raw, "sustain_end_seconds"), OnsetDBFS: optionalNumberPointer(raw, "onset_dbfs"), BodyDBFS: optionalNumberPointer(raw, "body_dbfs"), SustainDBFS: optionalNumberPointer(raw, "sustain_dbfs"), AttackBodyContrastDB: optionalNumberPointer(raw, "attack_body_contrast_db"), SustainDecayDB: optionalNumberPointer(raw, "sustain_decay_db")})
	}
	return evidence
}

func domBandDynamicsEvidence(primary, secondary map[string]any) []dom.BandDynamicsEvidence {
	row := domEvidenceSourceAny(primary, secondary, "band_dynamics", "band_dynamics_evidence")
	if len(row) == 0 {
		return nil
	}
	rows := mapRowsAny(row["bands"])
	if len(rows) == 0 {
		for id, raw := range mapValue(row["bands"]) {
			band := mapValue(raw)
			band["id"] = id
			rows = append(rows, band)
		}
	}
	out := make([]dom.BandDynamicsEvidence, 0, len(rows))
	for _, raw := range rows {
		out = append(out, dom.BandDynamicsEvidence{ID: cleanAnyString(raw["id"]), Status: firstNonEmpty(cleanAnyString(raw["status"]), featureStatus(raw)), TimeDistribution: domDistribution(raw["time_distribution"]), CrestDistribution: domDistribution(raw["crest_distribution"]), EvidenceRefs: stringRows(raw["evidence_refs"])})
	}
	return out
}

func domDistribution(value any) *dom.Distribution {
	row := mapValue(value)
	if len(row) == 0 {
		return nil
	}
	count := int(numberFromMap(row, "count"))
	if count <= 0 {
		return nil
	}
	return &dom.Distribution{Count: count, Min: numberFromMap(row, "min"), P50: numberFromMap(row, "p50"), P90: numberFromMap(row, "p90"), Max: numberFromMap(row, "max")}
}

func stringRows(value any) []string {
	rows, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if value := cleanAnyString(row); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func optionalNumberPointer(row map[string]any, key string) *float64 {
	if _, ok := row[key]; !ok || row[key] == nil {
		return nil
	}
	value := numberFromMap(row, key)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

func domSourceFreshness(sources ...map[string]any) string {
	for _, source := range sources {
		if strings.EqualFold(cleanAnyString(source["freshness"]), dom.StatusStale) || featureStatus(source) == dom.StatusStale {
			return dom.StatusStale
		}
	}
	return "fresh"
}
