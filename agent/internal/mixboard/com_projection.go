package mixboard

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"vit-daw-agent/internal/com"
)

const maxCOMArtifactBytes = 32 * 1024 * 1024

func finalizeCOMProjection(obs *ObservationPacket, req Request) {
	if obs == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(cleanAnyString(req.Args["com_mode"])))
	if mode == "" {
		return
	}
	input := com.Input{
		Mode:          mode,
		ObservationID: obs.ObservationID,
		MixSessionID:  obs.MixSessionID,
		CreatedAt:     obs.CreatedAt,
		TargetRef: map[string]any{
			"kind": obs.TargetRef.Kind, "id": obs.TargetRef.ID, "label": obs.TargetRef.Label,
		},
	}
	switch mode {
	case com.ModePairedIO:
		if artifact, ok := readCOMPairedArtifact(cleanAnyString(req.Args["com_artifact_path"])); ok {
			input.Paired = &artifact
		}
	case com.ModeChangeDelta:
		before, beforeOK := readCOMPairedArtifact(cleanAnyString(req.Args["com_before_artifact_path"]))
		after, afterOK := readCOMPairedArtifact(cleanAnyString(req.Args["com_after_artifact_path"]))
		if beforeOK && afterOK {
			beforeProjection := com.Build(com.Input{Mode: com.ModePairedIO, ObservationID: obs.ObservationID,
				MixSessionID: obs.MixSessionID, CreatedAt: obs.CreatedAt, TargetRef: input.TargetRef, Paired: &before})
			afterProjection := com.Build(com.Input{Mode: com.ModePairedIO, ObservationID: obs.ObservationID,
				MixSessionID: obs.MixSessionID, CreatedAt: obs.CreatedAt, TargetRef: input.TargetRef, Paired: &after})
			input.Change = &com.ChangeDeltaInput{Before: &beforeProjection, After: &afterProjection}
		}
	case com.ModeSourceOnly:
		input.Source, input.Conditions = comSourceInput(*obs, req)
	}
	projection := com.Build(input)
	obs.COMProjection = &projection
}

func readCOMPairedArtifact(path string) (com.PairedEvidenceArtifact, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || !comArtifactPathAllowed(path) {
		return com.PairedEvidenceArtifact{}, false
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxCOMArtifactBytes {
		return com.PairedEvidenceArtifact{}, false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return com.PairedEvidenceArtifact{}, false
	}
	artifact, err := com.DecodePairedEvidenceArtifact(raw)
	return artifact, err == nil
}

func comArtifactPathAllowed(path string) bool {
	roots := []string{}
	if root := strings.TrimSpace(os.Getenv("VIT_MIXBOARD_ROOT")); root != "" {
		roots = append(roots, root)
	}
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT"} {
		if root := strings.TrimSpace(os.Getenv(key)); root != "" {
			roots = append(roots, filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence"))
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		absRoot, rootErr := filepath.Abs(filepath.Clean(root))
		if rootErr != nil {
			continue
		}
		rel, relErr := filepath.Rel(absRoot, absPath)
		if relErr == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

func comSourceInput(obs ObservationPacket, req Request) (com.SourceEvidence, com.Conditions) {
	snapshot := mapValue(obs.GlobalSummary["feature_snapshot"])
	waveform := mapValue(snapshot["waveform_envelope"])
	if len(waveform) == 0 {
		waveform = mapValue(mapValue(obs.MixPackage["current_metrics"])["waveform"])
	}
	segments := waveformTimeSegments(waveform)
	sourceSegments := make([]com.TimeSegment, 0, len(segments))
	for _, row := range segments {
		sourceSegments = append(sourceSegments, com.TimeSegment{
			StartSeconds: numberFromMap(row, "start_seconds"), EndSeconds: numberFromMap(row, "end_seconds"),
			RMSDBFS: comNumberPointer(row, "rms_dbfs"), PeakDBFS: comNumberPointer(row, "peak_dbfs"),
			CrestDB: comNumberPointer(row, "crest_db"), EnergyState: cleanAnyString(row["energy_state"]),
		})
	}
	sampleRate := numberFromMap(waveform, "sample_rate")
	if sampleRate <= 0 {
		sampleRate = firstPositiveFloat(req.Args, "sample_rate")
	}
	channelCount := int(numberFromMap(waveform, "channel_count"))
	if channelCount <= 0 {
		channelCount = int(firstPositiveFloat(req.Args, "channel_count"))
	}
	duration := numberFromMap(waveform, "duration_seconds")
	if duration <= 0 {
		duration = obs.TimeRuler.DurationSeconds
	}
	startSample := int64(0)
	endSample := int64(numberFromMap(waveform, "analyzed_sample_count"))
	if analyzedRange := mapValue(waveform["analyzed_range"]); len(analyzedRange) > 0 {
		startSample = int64(numberFromMap(analyzedRange, "start_sample"))
		endSample = int64(numberFromMap(analyzedRange, "end_sample"))
	}
	if endSample <= startSample && sampleRate > 0 && duration > 0 {
		endSample = startSample + int64(math.Round(sampleRate*duration))
	}
	ref := cleanAnyString(waveform["evidence_ref"])
	refs := []string{}
	if ref != "" {
		refs = append(refs, ref)
	}
	status := featureStatus(waveform)
	nonzero := numberFromMap(waveform, "nonzero_count") > 0 || numberFromMap(waveform, "sum_abs") > 0 || numberFromMap(waveform, "peak_abs") > 0
	coverage := numberFromMap(waveform, "coverage_ratio")
	if coverage <= 0 && status == com.StatusReady {
		coverage = 1
	}
	source := com.SourceEvidence{
		ID: firstNonEmpty(ref, cleanAnyString(waveform["request_id"])), SchemaVersion: cleanAnyString(waveform["schema_version"]),
		Status: status, Freshness: comSourceFreshness(waveform), DurationSeconds: duration,
		RMSDBFS: comNumberPointer(waveform, "rms_dbfs"), PeakDBFS: comNumberPointer(waveform, "peak_dbfs"),
		CrestDB: comNumberPointer(waveform, "crest_db"), TimeSegments: sourceSegments,
		TransientEvents: comTransientEvidence(snapshot),
		Quality: com.QualityEvidence{QualityStatus: firstNonEmpty(cleanAnyString(waveform["quality_status"]), status),
			Nonzero: nonzero, Coverage: coverage, NaNInfCount: int(numberFromMap(waveform, "nan_count") + numberFromMap(waveform, "inf_count"))},
		EvidenceRefs: refs,
	}
	conditions := com.Conditions{
		SourceRevision: firstNonEmpty(cleanAnyString(waveform["source_revision"]), cleanAnyString(waveform["source_fingerprint"]), cleanAnyString(waveform["source_hash"])),
		ClipRevision:   cleanAnyString(waveform["clip_revision"]), StartSample: startSample, EndSample: endSample,
		StartSeconds: float64(startSample) / positiveOrOne(sampleRate), EndSeconds: float64(endSample) / positiveOrOne(sampleRate),
		SampleRate: sampleRate, ChannelCount: channelCount, ChannelLayout: cleanAnyString(waveform["channel_layout"]),
		AnalyzerVersion: firstNonEmpty(cleanAnyString(waveform["analyzer_version"]), cleanAnyString(waveform["analyzer_revision"])),
	}
	return source, conditions
}

func comNumberPointer(row map[string]any, key string) *float64 {
	value, ok := row[key]
	if !ok || value == nil {
		return nil
	}
	number := numberFromMap(row, key)
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil
	}
	return &number
}

// comTransientEvidence maps the DAD L3 frame-level transient_events block
// (status/events/coverage/window_ms/hop_ms/evidence_refs) into the compact COM
// source input. Missing evidence yields nil so source-only macro dynamics stay
// unchanged; the frame window/hop travel with the evidence so micro-transient
// readiness can declare its real analyzer resolution.
func comTransientEvidence(snapshot map[string]any) *com.TransientEventEvidence {
	if len(snapshot) == 0 {
		return nil
	}
	transient := mapValue(snapshot["transient_events"])
	if len(transient) == 0 {
		return nil
	}
	events := make([]com.TransientEvent, 0)
	for _, row := range mapRowsAny(transient["events"]) {
		events = append(events, com.TransientEvent{
			OnsetSeconds:         numberFromMap(row, "onset_seconds"),
			BodyEndSeconds:       numberFromMap(row, "body_end_seconds"),
			SustainEndSeconds:    numberFromMap(row, "sustain_end_seconds"),
			OnsetDBFS:            comNumberPointer(row, "onset_dbfs"),
			BodyDBFS:             comNumberPointer(row, "body_dbfs"),
			SustainDBFS:          comNumberPointer(row, "sustain_dbfs"),
			AttackBodyContrastDB: comNumberPointer(row, "attack_body_contrast_db"),
			SustainDecayDB:       comNumberPointer(row, "sustain_decay_db"),
		})
	}
	return &com.TransientEventEvidence{
		Status:       cleanAnyString(transient["status"]),
		Events:       events,
		Coverage:     numberFromMap(transient, "coverage"),
		WindowMS:     numberFromMap(transient, "window_ms"),
		HopMS:        numberFromMap(transient, "hop_ms"),
		EvidenceRefs: comEvidenceRefs(transient["evidence_refs"]),
	}
}

func comEvidenceRefs(value any) []string {
	refs := []string{}
	switch typed := value.(type) {
	case []string:
		for _, ref := range typed {
			if ref = strings.TrimSpace(ref); ref != "" {
				refs = append(refs, ref)
			}
		}
	case []any:
		for _, raw := range typed {
			if ref := strings.TrimSpace(cleanAnyString(raw)); ref != "" {
				refs = append(refs, ref)
			}
		}
	}
	return refs
}

func comSourceFreshness(row map[string]any) string {
	if status := strings.ToLower(cleanAnyString(row["freshness"])); status != "" {
		return status
	}
	if featureStatus(row) == com.StatusStale {
		return com.StatusStale
	}
	return "fresh"
}

func positiveOrOne(value float64) float64 {
	if value > 0 {
		return value
	}
	return 1
}
