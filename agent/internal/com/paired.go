package com

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	pairedArtifactSchema = "dad.compressor_dual_tap_evidence.v1"
	pairedTraceBasis     = "paired_trace"
	pairedEventsBasis    = "paired_events"
	aggregateOnlyBasis   = "aggregate_only"
	actionThresholdDB    = 0.5
)

type derivedFrame struct {
	startSample   int64
	inputPeak     float64
	inputRMS      float64
	outputPeak    float64
	outputRMS     float64
	gainDB        float64
	actionDB      float64
	channelGain   []float64
	channelAct    []float64
	channelActive []bool
	active        bool
}

type transientObservation struct {
	sample          int64
	contrastDeltaDB float64
	peakActionDB    float64
}

type recoveryObservation struct {
	eventSample int64
	band        string
	complete    bool
	initialDB   float64
}

func DecodePairedEvidenceArtifact(raw []byte) (PairedEvidenceArtifact, error) {
	var artifact PairedEvidenceArtifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return PairedEvidenceArtifact{}, fmt.Errorf("decode compressor dual-tap artifact: %w", err)
	}
	return artifact, nil
}

func normalizePairedFrames(in []PairedEnvelopeFrame) []PairedEnvelopeFrame {
	out := append([]PairedEnvelopeFrame(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartSample != out[j].StartSample {
			return out[i].StartSample < out[j].StartSample
		}
		return out[i].EndSample < out[j].EndSample
	})
	return out
}

func normalizePairedEvents(in []PairedInputEventCandidate) []PairedInputEventCandidate {
	out := append([]PairedInputEventCandidate(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Sample != out[j].Sample {
			return out[i].Sample < out[j].Sample
		}
		return out[i].InputPeakDBFS > out[j].InputPeakDBFS
	})
	return out
}

func buildPairedProjection(input Input) Projection {
	generatedAt := strings.TrimSpace(input.CreatedAt)
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if input.Paired == nil {
		return missingPairedProjection(input, generatedAt, "paired_evidence_missing")
	}
	artifact := *input.Paired
	validation := validatePairedArtifact(artifact, input)
	processor := processorScopeFromPaired(artifact.ProcessorScope)
	conditions := conditionsFromPaired(artifact)
	frames, baseline, validFrames := deriveFrames(artifact)
	events := selectPrimaryEvents(artifact.InputEventCandidates, artifact.Conditions.SampleRate)
	source := pairedSourceDynamics(artifact, frames, events)
	trust := pairedTrust(artifact, validation, frames, events, input.Derivation)

	projection := Projection{
		SchemaVersion: SchemaVersion, COMVersion: Version, Mode: ModePairedIO,
		Status: trust.OverallStatus, ObservationID: strings.TrimSpace(input.ObservationID),
		MixSessionID: strings.TrimSpace(input.MixSessionID), TargetRef: cloneMap(input.TargetRef),
		ProcessorScope: processor, Conditions: conditions, SourceDynamics: &source,
		EvidenceInputs: EvidenceInputs{SourceEvidenceID: artifact.PairID, SourceSchema: artifact.SchemaVersion,
			SourceStatus: trust.OverallStatus, EvidenceRefs: []string{artifact.EvidenceRef}, InputTrace: inputTraceIdentity(artifact)},
		TrustQuality: trust, EvidenceRefs: []string{artifact.EvidenceRef}, GeneratedAt: generatedAt,
	}

	projection.LevelEffect = deriveLevelEffect(artifact)
	projection.GainAction = deriveGainAction(frames, events, artifact, baseline, validFrames, input.Derivation)
	projection.TransientResponse = deriveTransientResponse(frames, events, artifact, baseline, input.Derivation)
	projection.RecoveryMotion = deriveRecoveryMotion(frames, events, artifact, input.Derivation)
	projection.StereoBehavior = deriveStereoBehavior(frames, artifact, input.Derivation)
	projection.TriggerRelation = unavailableBehavior("aligned_band_energy_evidence_missing", artifact.EvidenceRef)
	applyPairedTrustBoundary(&projection, trust.OverallStatus)
	projection.Identifiability = pairedIdentifiability(projection, artifact, frames, events, input.Derivation)
	finalizePairedReadiness(&projection)
	projection.Limitations = pairedLimitations(projection, validation, input.Derivation)
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func inputTraceIdentity(a PairedEvidenceArtifact) InputTraceIdentity {
	const quantum = .001
	hash := sha256.New()
	for _, frame := range a.AlignedEnvelopeFrames {
		_, _ = fmt.Fprintf(hash, "%d|%d", frame.StartSample, frame.EndSample)
		for _, values := range [][]float64{frame.InputPeakDBFS, frame.InputRMSDBFS} {
			for _, value := range values {
				_, _ = fmt.Fprintf(hash, "|%.3f", math.Round(value/quantum)*quantum)
			}
		}
		_, _ = fmt.Fprint(hash, "\n")
	}
	return InputTraceIdentity{Method: "quantized_input_peak_rms_sha256_v1", SHA256: hex.EncodeToString(hash.Sum(nil)),
		FrameCount: len(a.AlignedEnvelopeFrames), ChannelCount: a.Conditions.ChannelCount, QuantizationDB: quantum}
}

func missingPairedProjection(input Input, generatedAt, reason string) Projection {
	source := buildSourceDynamics(input)
	trust := TrustQuality{
		OverallStatus:               StatusMissing,
		ModeGates:                   []QualityGate{{ID: "paired_evidence", Required: true, Status: StatusMissing, Reason: reason}},
		CanSupportSourceDescription: source.Status != StatusMissing,
		BlockedReasons:              []string{reason}, MissingFields: []string{"paired_evidence"},
		Limitations: []string{"paired_io_requires_com2_artifact", "numeric_parameter_inference_forbidden"},
	}
	p := Projection{SchemaVersion: SchemaVersion, COMVersion: Version, Mode: ModePairedIO,
		Status: StatusMissing, ObservationID: input.ObservationID, MixSessionID: input.MixSessionID,
		TargetRef: cloneMap(input.TargetRef), ProcessorScope: input.ProcessorScope,
		Conditions: input.Conditions, SourceDynamics: &source, TrustQuality: trust,
		Identifiability: buildSourceOnlyIdentifiability(source), GeneratedAt: generatedAt,
		Limitations: []string{reason, "numeric_parameter_inference_forbidden"}}
	p.ProjectionID = stableProjectionID(p)
	p.LLMContext = buildLLMContext(p)
	return p
}

func validatePairedArtifact(a PairedEvidenceArtifact, input Input) PairedEvidenceValidation {
	reasons := []string{}
	require := func(ok bool, reason string) {
		if !ok {
			reasons = append(reasons, reason)
		}
	}
	require(a.SchemaVersion == pairedArtifactSchema, "paired_schema_mismatch")
	require(strings.TrimSpace(a.PairID) != "" && a.EvidenceRef == "dad.compressor_dual_tap:"+a.PairID, "pair_identity_invalid")
	require(strings.TrimSpace(a.AnalyzerVersion) != "", "analyzer_version_missing")
	require(a.ProcessorScope.SupportClass == "single_band_broadband", "unsupported_processor_scope")
	require(strings.TrimSpace(a.ProcessorScope.TrackID) != "" && strings.TrimSpace(a.ProcessorScope.PluginInstanceID) != "", "processor_target_missing")
	require(strings.TrimSpace(a.ProcessorScope.ChainHash) != "" && strings.TrimSpace(a.ProcessorScope.ProcessorStateHash) != "" && strings.TrimSpace(a.ProcessorScope.ScopeRevision) != "", "processor_scope_identity_missing")
	c := a.Conditions
	require(strings.TrimSpace(c.SourceRevision) != "" && strings.TrimSpace(c.ClipRevision) != "" && strings.TrimSpace(c.RenderRevision) != "", "revision_identity_missing")
	require(c.StartSample >= 0 && c.EndSample > c.StartSample, "sample_window_invalid")
	require(finite(c.SampleRate) && c.SampleRate > 7000 && c.ChannelCount > 0 && strings.TrimSpace(c.ChannelLayout) != "", "sample_format_invalid")
	require(c.RenderMode == "offline_probe" && c.Deterministic, "render_not_deterministic")
	require(c.InputTap == "compressor_input" && c.OutputTap == "compressor_output", "tap_order_invalid")
	require(c.TailPolicy == "exact_window_no_tail", "tail_policy_invalid")
	require(c.FrameSizeSamples > 0 && c.HopSizeSamples > 0, "analysis_resolution_missing")
	require(a.DeterminismProof.Status == StatusReady && a.DeterminismProof.Correlation >= .999 && math.Abs(a.DeterminismProof.PeakDBDelta) <= .10 && math.Abs(a.DeterminismProof.RMSDBDelta) <= .10, "determinism_proof_failed")
	require(a.LatencyAlignment.Status == StatusReady && a.LatencyAlignment.AppliedOffsetSamples == a.LatencyAlignment.MeasuredOffsetSamples && absInt(a.LatencyAlignment.ResidualErrorSamples) <= 1 && a.LatencyAlignment.Correlation >= .25, "latency_alignment_failed")
	require(a.QualityEvidence.Input.SampleFrames >= c.EndSample-c.StartSample && a.QualityEvidence.Output.SampleFrames >= c.EndSample-c.StartSample, "window_coverage_failed")
	require(a.QualityEvidence.Input.NonzeroSamples > 0 && a.QualityEvidence.Output.NonzeroSamples > 0, "active_signal_missing")
	require(a.QualityEvidence.Input.NaNInfSamples == 0 && a.QualityEvidence.Output.NaNInfSamples == 0, "nan_inf_detected")
	require(a.QualityEvidence.Input.Coverage >= .999 && a.QualityEvidence.Output.Coverage >= .999, "quality_coverage_failed")
	require(len(a.AlignedEnvelopeFrames) > 0, "aligned_envelope_missing")
	previous := int64(-1)
	for index, frame := range a.AlignedEnvelopeFrames {
		valid := frame.StartSample >= c.StartSample && frame.EndSample > frame.StartSample && frame.EndSample <= c.EndSample && frame.StartSample > previous
		valid = valid && frame.EndSample-frame.StartSample == int64(c.FrameSizeSamples)
		if index > 0 {
			valid = valid && frame.StartSample-previous == int64(c.HopSizeSamples)
		}
		valid = valid && len(frame.InputPeakDBFS) == c.ChannelCount && len(frame.InputRMSDBFS) == c.ChannelCount && len(frame.OutputPeakDBFS) == c.ChannelCount && len(frame.OutputRMSDBFS) == c.ChannelCount
		for _, values := range [][]float64{frame.InputPeakDBFS, frame.InputRMSDBFS, frame.OutputPeakDBFS, frame.OutputRMSDBFS} {
			for _, value := range values {
				valid = valid && finite(value)
			}
		}
		if !valid {
			reasons = append(reasons, "aligned_envelope_invalid")
			break
		}
		previous = frame.StartSample
	}
	previousEvent := int64(-1)
	for _, event := range a.InputEventCandidates {
		valid := event.Sample >= c.StartSample && event.Sample < c.EndSample && event.Sample > previousEvent
		valid = valid && finite(event.InputPeakDBFS) && event.Kind == "input_onset_candidate"
		if !valid {
			reasons = append(reasons, "input_event_candidates_invalid")
			break
		}
		previousEvent = event.Sample
	}
	compareExpectedIdentity(&reasons, a, input)
	return PairedEvidenceValidation{Ready: len(reasons) == 0, Reasons: uniqueSorted(reasons)}
}

func compareExpectedIdentity(reasons *[]string, a PairedEvidenceArtifact, input Input) {
	compare := func(actual, expected, reason string) {
		if strings.TrimSpace(expected) != "" && actual != expected {
			*reasons = append(*reasons, reason)
		}
	}
	compare(a.ProcessorScope.TrackID, input.ProcessorScope.TrackID, "track_identity_mismatch")
	compare(a.ProcessorScope.PluginInstanceID, input.ProcessorScope.PluginInstanceID, "plugin_identity_mismatch")
	compare(a.ProcessorScope.TopologyGeneration, input.ProcessorScope.TopologyGeneration, "topology_generation_mismatch")
	compare(a.ProcessorScope.PluginPosition, input.ProcessorScope.PluginPosition, "plugin_position_mismatch")
	compare(a.ProcessorScope.ChainHash, input.ProcessorScope.ChainHash, "chain_hash_mismatch")
	compare(a.ProcessorScope.ProcessorStateHash, input.ProcessorScope.ProcessorStateHash, "processor_state_mismatch")
	compare(a.ProcessorScope.ScopeRevision, input.ProcessorScope.ScopeRevision, "scope_revision_mismatch")
	compare(a.ProcessorScope.SupportClass, input.ProcessorScope.SupportClass, "support_class_mismatch")
	compare(a.Conditions.SourceRevision, input.Conditions.SourceRevision, "source_revision_mismatch")
	compare(a.Conditions.ClipRevision, input.Conditions.ClipRevision, "clip_revision_mismatch")
	compare(a.Conditions.RenderRevision, input.Conditions.RenderRevision, "render_revision_mismatch")
	compare(a.Conditions.ChannelLayout, input.Conditions.ChannelLayout, "channel_layout_mismatch")
	compare(a.Conditions.RenderMode, input.Conditions.RenderMode, "render_mode_mismatch")
	compare(a.AnalyzerVersion, input.Conditions.AnalyzerVersion, "analyzer_version_mismatch")
	if input.Conditions.EndSample > input.Conditions.StartSample && (a.Conditions.StartSample != input.Conditions.StartSample || a.Conditions.EndSample != input.Conditions.EndSample) {
		*reasons = append(*reasons, "sample_window_mismatch")
	}
	if input.Conditions.SampleRate > 0 && math.Abs(a.Conditions.SampleRate-input.Conditions.SampleRate) >= .01 {
		*reasons = append(*reasons, "sample_rate_mismatch")
	}
	if input.Conditions.ChannelCount > 0 && a.Conditions.ChannelCount != input.Conditions.ChannelCount {
		*reasons = append(*reasons, "channel_count_mismatch")
	}
}

func processorScopeFromPaired(s PairedProcessorScope) ProcessorScope {
	return ProcessorScope{TrackID: s.TrackID, PluginInstanceID: s.PluginInstanceID,
		PluginPosition: s.PluginPosition, TopologyClass: s.TopologyClass,
		TopologyGeneration: s.TopologyGeneration, ChainHash: s.ChainHash,
		ProcessorStateHash: s.ProcessorStateHash, ScopeRevision: s.ScopeRevision, SupportClass: s.SupportClass}
}

func conditionsFromPaired(a PairedEvidenceArtifact) Conditions {
	c := a.Conditions
	return Conditions{SourceRevision: c.SourceRevision, ClipRevision: c.ClipRevision, RenderRevision: c.RenderRevision,
		StartSample: c.StartSample, EndSample: c.EndSample,
		StartSeconds: float64(c.StartSample) / c.SampleRate, EndSeconds: float64(c.EndSample) / c.SampleRate,
		SampleRate: c.SampleRate, ChannelCount: c.ChannelCount, ChannelLayout: c.ChannelLayout,
		RenderMode: c.RenderMode, Deterministic: c.Deterministic, InputTap: c.InputTap, OutputTap: c.OutputTap,
		LatencySamples: a.LatencyAlignment.AppliedOffsetSamples, LatencyMethod: a.LatencyAlignment.Method,
		TailPolicy: c.TailPolicy, AnalyzerVersion: a.AnalyzerVersion,
		AnalysisResolutions: []AnalysisResolution{
			{Scale: ScaleMicroTransient, WindowMS: samplesMS(c.FrameSizeSamples, c.SampleRate), HopMS: samplesMS(c.HopSizeSamples, c.SampleRate)},
			{Scale: ScaleShortGainMotion, WindowMS: samplesMS(c.FrameSizeSamples, c.SampleRate), HopMS: samplesMS(c.HopSizeSamples, c.SampleRate)},
			{Scale: ScaleEventRecovery, WindowMS: samplesMS(c.FrameSizeSamples, c.SampleRate), HopMS: samplesMS(c.HopSizeSamples, c.SampleRate)},
		}}
}

func deriveFrames(a PairedEvidenceArtifact) ([]derivedFrame, float64, int) {
	frames := make([]derivedFrame, len(a.AlignedEnvelopeFrames))
	gains := []float64{}
	inputActivityFloor := math.Max(-90, a.QualityEvidence.Input.RMSDBFS-30)
	for i, raw := range a.AlignedEnvelopeFrames {
		frame := derivedFrame{startSample: raw.StartSample,
			inputPeak: combineDB(raw.InputPeakDBFS, true), inputRMS: combineDB(raw.InputRMSDBFS, false),
			outputPeak: combineDB(raw.OutputPeakDBFS, true), outputRMS: combineDB(raw.OutputRMSDBFS, false)}
		frame.channelGain = make([]float64, a.Conditions.ChannelCount)
		frame.channelActive = make([]bool, a.Conditions.ChannelCount)
		frame.active = frame.inputRMS > inputActivityFloor && frame.outputRMS > -140
		if frame.active {
			frame.gainDB = frame.outputRMS - frame.inputRMS
			gains = append(gains, frame.gainDB)
		}
		for channel := 0; channel < a.Conditions.ChannelCount; channel++ {
			frame.channelGain[channel] = raw.OutputRMSDBFS[channel] - raw.InputRMSDBFS[channel]
			frame.channelActive[channel] = raw.InputRMSDBFS[channel] > inputActivityFloor && raw.OutputRMSDBFS[channel] > -140
		}
		frames[i] = frame
	}
	sort.Float64s(gains)
	baseline := 0.0
	if len(gains) > 0 {
		baseline = quantile(gains, .90)
	}
	channelBaselines := make([]float64, a.Conditions.ChannelCount)
	for channel := 0; channel < a.Conditions.ChannelCount; channel++ {
		values := []float64{}
		for _, frame := range frames {
			if frame.channelActive[channel] {
				values = append(values, frame.channelGain[channel])
			}
		}
		sort.Float64s(values)
		if len(values) > 0 {
			channelBaselines[channel] = quantile(values, .90)
		}
	}
	for i := range frames {
		if frames[i].active {
			frames[i].actionDB = math.Max(0, baseline-frames[i].gainDB)
		}
		frames[i].channelAct = make([]float64, a.Conditions.ChannelCount)
		for channel := 0; channel < a.Conditions.ChannelCount; channel++ {
			if frames[i].channelActive[channel] {
				frames[i].channelAct[channel] = math.Max(0, channelBaselines[channel]-frames[i].channelGain[channel])
			}
		}
	}
	return frames, baseline, len(gains)
}

func pairedSourceDynamics(a PairedEvidenceArtifact, frames []derivedFrame, events []PairedInputEventCandidate) SourceDynamics {
	duration := float64(a.Conditions.EndSample-a.Conditions.StartSample) / a.Conditions.SampleRate
	inputQ := a.QualityEvidence.Input
	rms, peak := inputQ.RMSDBFS, inputQ.PeakDBFS
	crest := peak - rms
	activeFrames := 0
	for _, f := range frames {
		if f.active {
			activeFrames++
		}
	}
	contrasts := sourceEventContrasts(frames, events, a.Conditions.SampleRate)
	activity := ActivityCoverage{SegmentCount: len(frames), ValidSegmentCount: len(frames), ActiveSegmentCount: activeFrames,
		SilentSegmentCount: len(frames) - activeFrames, AnalyzedSeconds: round3(duration), CoverageRatio: 1}
	if len(frames) > 0 {
		activity.ActiveRatio = round3(float64(activeFrames) / float64(len(frames)))
		activity.ActiveSeconds = round3(duration * activity.ActiveRatio)
		activity.SilentSeconds = round3(duration - activity.ActiveSeconds)
	}
	intervals := eventIntervalsMS(events, a.Conditions.SampleRate)
	density := 0.0
	if duration > 0 {
		density = round3(float64(len(events)) / duration)
	}
	return SourceDynamics{SchemaVersion: "com.source_dynamics.v1", Status: StatusReady,
		DeclaredScale: ScaleMicroTransient, AnalyzedDuration: round3(duration),
		Levels:   SourceLevels{RMSDBFS: &rms, PeakDBFS: &peak, CrestDB: &crest},
		Activity: activity,
		Events: SourceEventSummary{EventCount: len(events), EventDensityPerSecond: &density,
			InterEventIntervalMS: distribution(intervals), TransientContrastDB: distribution(contrasts)},
		TimeScaleCoverage: []TimeScaleCoverage{
			{Scale: ScaleMicroTransient, Status: StatusReady, WindowMS: samplesMS(a.Conditions.FrameSizeSamples, a.Conditions.SampleRate), HopMS: samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate), SegmentCount: len(frames)},
			{Scale: ScaleShortGainMotion, Status: StatusReady, WindowMS: samplesMS(a.Conditions.FrameSizeSamples, a.Conditions.SampleRate), HopMS: samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate), SegmentCount: len(frames)},
			{Scale: ScaleEventRecovery, Status: timeScaleStatus(len(events) >= 2), WindowMS: samplesMS(a.Conditions.FrameSizeSamples, a.Conditions.SampleRate), HopMS: samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate), SegmentCount: len(events), Reason: missingReason(len(events) >= 2, "event_coverage_insufficient")},
			{Scale: ScaleMacroProgram, Status: StatusReady, SegmentCount: len(frames)},
		}, EvidenceRefs: []string{a.EvidenceRef},
		Limitations: []string{"source_event_candidates_are_observed_not_semantic_hits"}}
}

func pairedTrust(a PairedEvidenceArtifact, validation PairedEvidenceValidation, frames []derivedFrame, events []PairedInputEventCandidate, constraints PairedDerivationConstraints) TrustQuality {
	hopMS := samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate)
	validationStatus := StatusReady
	if !validation.Ready {
		validationStatus = pairedValidationStatus(validation.Reasons)
	}
	gates := []QualityGate{
		{ID: "paired_artifact_valid", Required: true, Status: validationStatus, Reason: missingReason(validation.Ready, strings.Join(validation.Reasons, ","))},
		gate("supported_processor_scope", true, a.ProcessorScope.SupportClass == "single_band_broadband", "unsupported_processor_scope"),
		gate("same_source", true, strings.TrimSpace(a.Conditions.SourceRevision) != "", "source_revision_missing"),
		gate("exact_sample_window", true, a.Conditions.EndSample > a.Conditions.StartSample, "sample_window_invalid"),
		gate("sample_format", true, a.Conditions.SampleRate > 0 && a.Conditions.ChannelCount > 0, "sample_format_invalid"),
		gate("tap_order", true, a.Conditions.InputTap == "compressor_input" && a.Conditions.OutputTap == "compressor_output", "tap_order_invalid"),
		gate("determinism", true, a.DeterminismProof.Status == StatusReady, "determinism_proof_failed"),
		gate("latency_alignment", true, a.LatencyAlignment.Status == StatusReady && absInt(a.LatencyAlignment.ResidualErrorSamples) <= 1, "latency_alignment_failed"),
		gate("active_coverage", true, countActiveFrames(frames) >= 16, "active_frame_coverage_insufficient"),
		gate("fine_trace_resolution", true, hopMS > 0 && hopMS <= 5, "fine_trace_resolution_insufficient"),
		gate("event_coverage", false, len(events) >= 3, "event_coverage_insufficient"),
		gate("unconfounded_parallel_path", true, !constraints.ParallelBlendActive, "parallel_blend_confounds_gain_action"),
	}
	blocked, approximate, suspect, stale, missing := classifyGates(gates)
	status := StatusReady
	if !validation.Ready {
		status = pairedValidationStatus(validation.Reasons)
	} else if constraints.ParallelBlendActive || countActiveFrames(frames) < 16 || hopMS > 5 {
		status = StatusPartial
	} else if constraints.GainStageAmbiguous {
		status = StatusPartial
	} else if len(events) < 3 {
		status = StatusPartial
	}
	limitations := []string{"effective_behavior_not_plugin_meter_data", "numeric_parameter_inference_forbidden", "trigger_relation_requires_aligned_band_evidence"}
	if !constraints.ParallelBlendKnown {
		limitations = append(limitations, "internal_wet_dry_state_not_independently_observed")
	}
	if constraints.GainStageAmbiguous {
		limitations = append(limitations, "gain_stage_interpretation_ambiguous")
	}
	sameSource := !hasAnyReason(validation.Reasons, "source_revision_mismatch", "source_revision_missing", "revision_identity_missing")
	sameWindow := !hasAnyReason(validation.Reasons, "sample_window_mismatch", "sample_window_invalid", "window_coverage_failed")
	sameFormat := !hasAnyReason(validation.Reasons, "sample_rate_mismatch", "channel_count_mismatch", "channel_layout_mismatch", "sample_format_invalid")
	tapOrderValid := !hasAnyReason(validation.Reasons, "tap_order_invalid")
	return TrustQuality{OverallStatus: status, ModeGates: gates,
		Coverage:   map[string]any{"envelope_frame_count": len(frames), "active_frame_count": countActiveFrames(frames), "event_count": len(events), "window_ms": samplesMS(a.Conditions.FrameSizeSamples, a.Conditions.SampleRate), "hop_ms": hopMS},
		SameSource: sameSource, SameWindow: sameWindow, SameFormat: sameFormat, TapOrderValid: tapOrderValid,
		Deterministic: a.DeterminismProof.Status == StatusReady, LatencyAligned: a.LatencyAlignment.Status == StatusReady,
		InputEquivalent: true, CanSupportSourceDescription: validation.Ready,
		CanSupportBehaviorObservation:  status == StatusReady || status == StatusPartial,
		CanSupportSemanticPlanning:     status == StatusReady || status == StatusPartial,
		CanSupportPostActionEvaluation: false, BlockedReasons: blocked,
		ApproximateFields: approximate, SuspectFields: suspect, StaleFields: stale, MissingFields: missing,
		Limitations: uniqueSorted(limitations), EvidenceRefs: []string{a.EvidenceRef}}
}

func deriveGainAction(frames []derivedFrame, events []PairedInputEventCandidate, a PairedEvidenceArtifact, baseline float64, validFrames int, constraints PairedDerivationConstraints) *BehaviorProjection {
	ref := a.EvidenceRef
	actions := []float64{}
	activeCount := 0
	for _, frame := range frames {
		if frame.active {
			actions = append(actions, frame.actionDB)
			if frame.actionDB >= actionThresholdDB {
				activeCount++
			}
		}
	}
	if validFrames < 16 {
		return unavailableBehavior("active_frame_coverage_insufficient", ref)
	}
	sort.Float64s(actions)
	duty := float64(activeCount) / float64(len(actions))
	label := "no_time_varying_action_observed"
	if quantile(actions, .90) >= actionThresholdDB {
		label = "time_varying_gain_action_observed"
	}
	status := StatusReady
	limitations := []string{"steady_gain_baseline_is_observed_not_makeup_parameter", "not_a_plugin_gain_reduction_meter"}
	if constraints.ParallelBlendActive || constraints.GainStageAmbiguous {
		status = StatusPartial
		limitations = append(limitations, "effective_action_confounded_by_known_signal_blend_or_gain_stage")
	}
	eventDepths := eventActionDepths(frames, events, a.Conditions.SampleRate)
	eventConsistency := "not_identifiable"
	eventSpread := 0.0
	if len(eventDepths) >= 3 {
		eventSpread = stddev(eventDepths)
		eventConsistency = "variable"
		if eventSpread <= .75 {
			eventConsistency = "consistent"
		} else if eventSpread <= 2.0 {
			eventConsistency = "moderately_variable"
		}
	}
	return &BehaviorProjection{Status: status, Facts: map[string]any{
		"classification": label, "steady_gain_baseline_db": round3(baseline),
		"active_action_coverage": round3(duty), "gain_action_duty_cycle": round3(duty),
		"reduction_depth_db_distribution": compactDistribution(actions),
		"active_frame_count":              len(actions), "action_detection_tolerance_db": actionThresholdDB,
		"event_action_depth_db": compactDistribution(eventDepths), "event_action_consistency": eventConsistency,
		"event_action_depth_stddev_db": round3(eventSpread),
	}, EvidenceRefs: []string{ref}, Limitations: uniqueSorted(limitations)}
}

func deriveTransientResponse(frames []derivedFrame, events []PairedInputEventCandidate, a PairedEvidenceArtifact, baseline float64, constraints PairedDerivationConstraints) *BehaviorProjection {
	hopMS := samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate)
	if hopMS <= 0 || hopMS > 5 {
		return unavailableBehavior("micro_transient_resolution_insufficient", a.EvidenceRef)
	}
	observations := transientObservations(frames, events, a.Conditions.SampleRate, baseline)
	if len(observations) < 3 {
		return unavailableBehavior("matched_event_coverage_insufficient", a.EvidenceRef)
	}
	deltas, peakActions := []float64{}, []float64{}
	positive, negative := 0, 0
	for _, observation := range observations {
		deltas = append(deltas, observation.contrastDeltaDB)
		peakActions = append(peakActions, observation.peakActionDB)
		if observation.contrastDeltaDB >= .75 {
			positive++
		}
		if observation.contrastDeltaDB <= -.75 {
			negative++
		}
	}
	sort.Float64s(deltas)
	sort.Float64s(peakActions)
	median := quantile(deltas, .5)
	classification := "unchanged_within_tolerance"
	if positive > 0 && negative > 0 && float64(positive+negative)/float64(len(deltas)) >= .35 {
		classification = "mixed"
	} else if median <= -.75 {
		classification = "more_rounded"
	} else if median >= .75 {
		classification = "more_preserved"
	}
	status := StatusReady
	limits := []string{"classification_is_observed_leading_edge_behavior_not_attack_parameter", "event_candidates_are_not_semantic_hit_labels"}
	if constraints.ParallelBlendActive {
		status = StatusPartial
		limits = append(limits, "parallel_blend_confounds_leading_edge_attribution")
	}
	return &BehaviorProjection{Status: status, Facts: map[string]any{
		"classification": classification, "matched_event_count": len(observations),
		"transient_to_body_contrast_change_db": compactDistribution(deltas),
		"leading_edge_action_db":               compactDistribution(peakActions), "classification_tolerance_db": .75,
	}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: limits}
}

func deriveRecoveryMotion(frames []derivedFrame, events []PairedInputEventCandidate, a PairedEvidenceArtifact, constraints PairedDerivationConstraints) *BehaviorProjection {
	hopMS := samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate)
	if hopMS <= 0 || hopMS > 10 || len(events) < 2 {
		return unavailableBehavior("event_recovery_resolution_or_coverage_insufficient", a.EvidenceRef)
	}
	observations, unobservable := recoveryObservations(frames, events, a.Conditions.SampleRate)
	if len(observations) == 0 {
		if unobservable > 0 {
			return unavailableBehavior("recovery_not_identifiable_over_silent_inter_event_window", a.EvidenceRef)
		}
		return &BehaviorProjection{Status: StatusReady, Facts: map[string]any{
			"classification": "no_action_to_recover", "evaluated_event_count": len(events),
		}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: []string{"no_numeric_release_or_auto_release_inference"}}
	}
	bandCounts := map[string]int{}
	complete := 0
	initial := []float64{}
	for _, observation := range observations {
		bandCounts[observation.band]++
		initial = append(initial, observation.initialDB)
		if observation.complete {
			complete++
		}
	}
	classification := "recovery_observed"
	if complete == 0 {
		classification = "incomplete_before_next_event"
	}
	if complete > 0 && complete < len(observations) {
		classification = "mixed_recovery"
	}
	status := StatusReady
	limits := []string{"return_time_is_a_behavior_band_not_release_parameter", "no_auto_release_state_inference", "pumping_not_diagnosed_from_recovery_alone"}
	if constraints.ParallelBlendActive {
		status = StatusPartial
		limits = append(limits, "parallel_blend_confounds_recovery_depth")
	}
	if unobservable > 0 {
		limits = append(limits, "some_recovery_windows_unobservable_because_input_fell_below_floor")
	}
	coverage := recoveryObservationCoverage(len(observations), unobservable)
	if len(observations) < 3 || coverage < .8 {
		status = StatusPartial
	}
	periodicity := periodicRecoveryCandidate(observations, events, a.Conditions.SampleRate)
	return &BehaviorProjection{Status: status, Facts: map[string]any{
		"classification": classification, "evaluated_action_event_count": len(observations),
		"complete_recovery_ratio":  round3(float64(complete) / float64(len(observations))),
		"unobservable_event_count": unobservable, "observable_recovery_coverage": round3(coverage),
		"return_time_band_counts": bandCounts, "initial_action_db": compactDistribution(initial),
		"periodic_modulation_candidate": periodicity,
	}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: limits}
}

func recoveryObservationCoverage(observed, unobservable int) float64 {
	total := observed + unobservable
	if total <= 0 {
		return 0
	}
	return float64(observed) / float64(total)
}

func deriveLevelEffect(a PairedEvidenceArtifact) *BehaviorProjection {
	in, out := a.QualityEvidence.Input, a.QualityEvidence.Output
	inCrest, outCrest := in.PeakDBFS-in.RMSDBFS, out.PeakDBFS-out.RMSDBFS
	return &BehaviorProjection{Status: StatusReady, Facts: map[string]any{
		"input_rms_dbfs": round3(in.RMSDBFS), "output_rms_dbfs": round3(out.RMSDBFS), "rms_delta_db": round3(out.RMSDBFS - in.RMSDBFS),
		"input_peak_dbfs": round3(in.PeakDBFS), "output_peak_dbfs": round3(out.PeakDBFS), "peak_delta_db": round3(out.PeakDBFS - in.PeakDBFS),
		"input_crest_db": round3(inCrest), "output_crest_db": round3(outCrest), "crest_delta_db": round3(outCrest - inCrest),
	}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: []string{"scalar_level_effect_does_not_identify_dynamic_action", "makeup_or_output_gain_may_dominate_scalar_delta"}}
}

func deriveStereoBehavior(frames []derivedFrame, a PairedEvidenceArtifact, constraints PairedDerivationConstraints) *BehaviorProjection {
	if a.Conditions.ChannelCount == 1 || strings.EqualFold(a.Conditions.ChannelLayout, "mono") {
		return &BehaviorProjection{Status: StatusNotApplicable, Facts: map[string]any{"classification": "mono_not_applicable"}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: []string{"channel_link_not_applicable_to_mono_evidence"}}
	}
	if a.Conditions.ChannelCount != 2 {
		return unavailableBehavior("stereo_behavior_v1_requires_two_channels", a.EvidenceRef)
	}
	left, right, differences := []float64{}, []float64{}, []float64{}
	for _, frame := range frames {
		if !frame.active || len(frame.channelAct) < 2 || len(frame.channelActive) < 2 || !frame.channelActive[0] || !frame.channelActive[1] {
			continue
		}
		left = append(left, frame.channelAct[0])
		right = append(right, frame.channelAct[1])
		differences = append(differences, math.Abs(frame.channelAct[0]-frame.channelAct[1]))
	}
	if len(left) < 16 {
		return unavailableBehavior("stereo_active_coverage_insufficient", a.EvidenceRef)
	}
	sort.Float64s(differences)
	corr := pearson(left, right)
	p90Diff := quantile(differences, .9)
	classification := "mixed_channel_action"
	if corr >= .90 && p90Diff <= .75 {
		classification = "closely_coherent_action"
	}
	if corr < .5 || p90Diff >= 2 {
		classification = "asymmetric_action"
	}
	status := StatusReady
	limits := []string{"observed_channel_relation_does_not_identify_channel_link_control", "image_or_phase_correlation_change_not_identifiable_from_envelope_only_evidence"}
	if constraints.ParallelBlendActive {
		status = StatusPartial
		limits = append(limits, "parallel_blend_confounds_channel_action")
	}
	return &BehaviorProjection{Status: status, Facts: map[string]any{
		"classification": classification, "left_right_action_correlation": round3(corr),
		"absolute_action_difference_db": compactDistribution(differences), "action_difference_p90_db": round3(p90Diff), "active_frame_count": len(left),
	}, EvidenceRefs: []string{a.EvidenceRef}, Limitations: limits}
}

func pairedIdentifiability(p Projection, a PairedEvidenceArtifact, frames []derivedFrame, events []PairedInputEventCandidate, constraints PairedDerivationConstraints) Identifiability {
	resolution := ResolutionEvidence{WindowMS: samplesMS(a.Conditions.FrameSizeSamples, a.Conditions.SampleRate), HopMS: samplesMS(a.Conditions.HopSizeSamples, a.Conditions.SampleRate), EventCount: len(events), SegmentCount: len(frames)}
	dim := []IdentifiabilityDimension{
		{Dimension: DimensionSourceDynamics, Status: Identified, Basis: pairedTraceBasis, Confidence: pairedConfidence(p, 1), Resolution: resolution, Supports: []string{"source_axis:macro_dynamics", "source_axis:event_density"}, DoesNotSupport: []string{"parameter_values"}},
		behaviorDimension(DimensionGainAction, p.GainAction, pairedTraceBasis, resolution, "semantic_axis:activation_intensity", []string{"parameter_value:threshold_db", "parameter_value:ratio", "plugin_meter:gain_reduction_db"}, pairedConfidence(p, .98)),
		behaviorDimension(DimensionTransientResponse, p.TransientResponse, pairedEventsBasis, resolution, "semantic_axis:transient_timing", []string{"parameter_value:attack_ms", "parameter_value:lookahead_ms"}, pairedConfidence(p, .92)),
		behaviorDimension(DimensionRecoveryMotion, p.RecoveryMotion, pairedEventsBasis, resolution, "semantic_axis:recovery_sustain", []string{"parameter_value:release_ms", "parameter_value:auto_release"}, pairedConfidence(p, .88)),
		behaviorDimension(DimensionLevelEffect, p.LevelEffect, aggregateOnlyBasis, resolution, "semantic_axis:output_normalization", []string{"parameter_value:makeup_gain_db"}, pairedConfidence(p, .98)),
		behaviorDimension(DimensionStereoBehavior, p.StereoBehavior, pairedTraceBasis, resolution, "semantic_axis:channel_link", []string{"parameter_value:channel_link"}, pairedConfidence(p, .85)),
		notIdentifiablePairedDimension(DimensionTriggerRelation, "aligned_band_energy_evidence_missing", "semantic_axis:detector_focus", "parameter_value:sidechain_filter_hz"),
	}
	if constraints.ParallelBlendActive {
		for i := range dim {
			if dim[i].Dimension == DimensionGainAction || dim[i].Dimension == DimensionTransientResponse || dim[i].Dimension == DimensionRecoveryMotion || dim[i].Dimension == DimensionStereoBehavior {
				dim[i].Status = Bounded
				dim[i].Confidence = round3(dim[i].Confidence * .6)
				dim[i].Limitations = append(dim[i].Limitations, "parallel_blend_prevents_unique_compressor_action_attribution")
			}
		}
	}
	return Identifiability{Dimensions: dim}
}

func behaviorDimension(name string, behavior *BehaviorProjection, basis string, resolution ResolutionEvidence, support string, unsupported []string, confidence float64) IdentifiabilityDimension {
	d := IdentifiabilityDimension{Dimension: name, Basis: basis, Confidence: round3(clamp(confidence, 0, 1)), Resolution: resolution,
		Supports: []string{support}, DoesNotSupport: append(unsupported, "numeric_parameter_inference"), Limitations: []string{"numeric_parameter_inference_forbidden"}}
	if behavior == nil || behavior.Status == StatusMissing || behavior.Status == StatusSuspect || behavior.Status == StatusStale {
		d.Status = NotIdentifiable
		d.Confidence = 0
		d.Supports = nil
		if behavior != nil {
			d.Limitations = append(d.Limitations, behavior.Limitations...)
		}
	} else if behavior.Status == StatusNotApplicable {
		d.Status = NotApplicable
		d.Confidence = 1
		d.Supports = nil
	} else if behavior.Status == StatusPartial || behavior.Status == StatusApproximate {
		d.Status = Bounded
		d.Limitations = append(d.Limitations, behavior.Limitations...)
	} else {
		d.Status = Identified
	}
	return d
}

func pairedValidationStatus(reasons []string) string {
	for _, reason := range reasons {
		if strings.Contains(reason, "_mismatch") {
			return StatusStale
		}
	}
	return StatusSuspect
}

func hasAnyReason(reasons []string, wanted ...string) bool {
	for _, reason := range reasons {
		for _, target := range wanted {
			if reason == target {
				return true
			}
		}
	}
	return false
}

func notIdentifiablePairedDimension(name, reason, support string, unsupported ...string) IdentifiabilityDimension {
	return IdentifiabilityDimension{Dimension: name, Status: NotIdentifiable, Basis: pairedTraceBasis,
		DoesNotSupport: append([]string{support}, unsupported...), Limitations: []string{reason, "numeric_parameter_inference_forbidden"}}
}

func pairedConfidence(p Projection, factor float64) float64 {
	base := .95
	if p.TrustQuality.OverallStatus == StatusPartial {
		base = .7
	}
	if p.TrustQuality.OverallStatus == StatusSuspect {
		base = .3
	}
	return base * factor
}

func pairedLimitations(p Projection, validation PairedEvidenceValidation, constraints PairedDerivationConstraints) []string {
	limits := append([]string{}, p.TrustQuality.Limitations...)
	limits = append(limits, validation.Reasons...)
	limits = append(limits, "observed_behavior_does_not_uniquely_identify_compressor_controls", "no_mutation_authority")
	if constraints.ParallelBlendActive {
		limits = append(limits, "known_parallel_blend_confounds_unique_action")
	}
	return uniqueSorted(limits)
}

func finalizePairedReadiness(p *Projection) {
	if p == nil || p.Status == StatusMissing || p.Status == StatusStale || p.Status == StatusSuspect {
		return
	}
	required := []string{DimensionGainAction, DimensionTransientResponse, DimensionRecoveryMotion, DimensionLevelEffect}
	if p.Conditions.ChannelCount == 2 {
		required = append(required, DimensionStereoBehavior)
	}
	statusByDimension := map[string]string{}
	for _, dimension := range p.Identifiability.Dimensions {
		statusByDimension[dimension.Dimension] = dimension.Status
	}
	for _, dimension := range required {
		if statusByDimension[dimension] != Identified {
			p.Status = StatusPartial
			p.TrustQuality.OverallStatus = StatusPartial
			p.TrustQuality.CanSupportBehaviorObservation = true
			p.TrustQuality.CanSupportSemanticPlanning = true
			p.TrustQuality.Limitations = uniqueSorted(append(p.TrustQuality.Limitations, dimension+"_not_fully_identified"))
		}
	}
}

func applyPairedTrustBoundary(p *Projection, trustStatus string) {
	if p == nil || (trustStatus != StatusSuspect && trustStatus != StatusStale && trustStatus != StatusMissing) {
		return
	}
	for _, behavior := range []*BehaviorProjection{p.GainAction, p.TransientResponse, p.RecoveryMotion, p.LevelEffect, p.StereoBehavior, p.TriggerRelation} {
		if behavior == nil || behavior.Status == StatusNotApplicable {
			continue
		}
		behavior.Status = trustStatus
		behavior.Limitations = uniqueSorted(append(behavior.Limitations, "paired_attribution_blocked_by_trust_gate"))
	}
}

func unavailableBehavior(reason, ref string) *BehaviorProjection {
	return &BehaviorProjection{Status: StatusMissing, EvidenceRefs: []string{ref}, Limitations: []string{reason, "numeric_parameter_inference_forbidden"}}
}

func selectPrimaryEvents(events []PairedInputEventCandidate, sampleRate float64) []PairedInputEventCandidate {
	if len(events) == 0 || sampleRate <= 0 {
		return nil
	}
	minimumGap := int64(.150 * sampleRate)
	ranked := normalizePairedEvents(events)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].InputPeakDBFS != ranked[j].InputPeakDBFS {
			return ranked[i].InputPeakDBFS > ranked[j].InputPeakDBFS
		}
		return ranked[i].Sample < ranked[j].Sample
	})
	selected := make([]PairedInputEventCandidate, 0, len(ranked))
	for _, candidate := range ranked {
		keep := true
		for _, primary := range selected {
			distance := candidate.Sample - primary.Sample
			if distance < 0 {
				distance = -distance
			}
			if distance < minimumGap {
				keep = false
				break
			}
		}
		if keep {
			selected = append(selected, candidate)
		}
	}
	return normalizePairedEvents(selected)
}

func transientObservations(frames []derivedFrame, events []PairedInputEventCandidate, sampleRate, baseline float64) []transientObservation {
	out := []transientObservation{}
	for _, event := range events {
		leadEnd := event.Sample + int64(.010*sampleRate)
		bodyStart := event.Sample + int64(.010*sampleRate)
		bodyEnd := event.Sample + int64(.060*sampleRate)
		inLead, outLead, inBody, outBody := []float64{}, []float64{}, []float64{}, []float64{}
		for _, frame := range frames {
			if frame.startSample >= event.Sample && frame.startSample < leadEnd {
				inLead = append(inLead, frame.inputPeak)
				outLead = append(outLead, frame.outputPeak)
			}
			if frame.startSample >= bodyStart && frame.startSample < bodyEnd && frame.active {
				inBody = append(inBody, frame.inputRMS)
				outBody = append(outBody, frame.outputRMS)
			}
		}
		if len(inLead) == 0 || len(inBody) < 2 {
			continue
		}
		sort.Float64s(inBody)
		sort.Float64s(outBody)
		inContrast := maximum(inLead) - quantile(inBody, .5)
		outContrast := maximum(outLead) - quantile(outBody, .5)
		peakAction := math.Max(0, baseline-(maximum(outLead)-maximum(inLead)))
		out = append(out, transientObservation{sample: event.Sample, contrastDeltaDB: round3(outContrast - inContrast), peakActionDB: round3(peakAction)})
	}
	return out
}

func recoveryObservations(frames []derivedFrame, events []PairedInputEventCandidate, sampleRate float64) ([]recoveryObservation, int) {
	out := []recoveryObservation{}
	unobservable := 0
	for i, event := range events {
		end := event.Sample + int64(3*sampleRate)
		if i+1 < len(events) && events[i+1].Sample < end {
			end = events[i+1].Sample
		}
		attackEnd := event.Sample + int64(.100*sampleRate)
		peakAction, peakAt := 0.0, int64(0)
		peakFound := false
		for _, frame := range frames {
			if frame.startSample >= event.Sample && frame.startSample < attackEnd && frame.active && frame.actionDB > peakAction {
				peakAction, peakAt = frame.actionDB, frame.startSample
				peakFound = true
			}
		}
		if peakAction < actionThresholdDB || !peakFound {
			continue
		}
		returnThreshold := math.Max(.35, peakAction*.2)
		returnSample := int64(-1)
		consecutive := 0
		activeAfterPeak := []int64{}
		for _, frame := range frames {
			if frame.startSample <= peakAt || frame.startSample >= end || !frame.active {
				continue
			}
			activeAfterPeak = append(activeAfterPeak, frame.startSample)
			if frame.actionDB <= returnThreshold {
				consecutive++
			} else {
				consecutive = 0
			}
			if consecutive >= 3 {
				returnSample = frame.startSample
				break
			}
		}
		if returnSample < 0 {
			terminalCoverageStart := end - int64(.050*sampleRate)
			terminalActive := 0
			for _, sample := range activeAfterPeak {
				if sample >= terminalCoverageStart {
					terminalActive++
				}
			}
			if len(activeAfterPeak) < 3 || terminalActive < 3 {
				unobservable++
				continue
			}
			out = append(out, recoveryObservation{eventSample: event.Sample, band: "not_returned_before_next_event", initialDB: peakAction})
			continue
		}
		ms := float64(returnSample-peakAt) * 1000 / sampleRate
		out = append(out, recoveryObservation{eventSample: event.Sample, band: recoveryBand(ms), complete: true, initialDB: peakAction})
	}
	return out, unobservable
}

func recoveryBand(ms float64) string {
	switch {
	case ms < 50:
		return "under_50_ms"
	case ms < 200:
		return "50_to_200_ms"
	case ms < 500:
		return "200_to_500_ms"
	case ms < 1000:
		return "500_to_1000_ms"
	default:
		return "over_1000_ms"
	}
}

func eventIntervalsMS(events []PairedInputEventCandidate, sampleRate float64) []float64 {
	out := []float64{}
	for i := 1; i < len(events); i++ {
		out = append(out, round3(float64(events[i].Sample-events[i-1].Sample)*1000/sampleRate))
	}
	return out
}

func sourceEventContrasts(frames []derivedFrame, events []PairedInputEventCandidate, sampleRate float64) []float64 {
	values := []float64{}
	for _, event := range events {
		leadEnd := event.Sample + int64(.010*sampleRate)
		bodyEnd := event.Sample + int64(.060*sampleRate)
		leads, bodies := []float64{}, []float64{}
		for _, frame := range frames {
			if frame.startSample >= event.Sample && frame.startSample < leadEnd {
				leads = append(leads, frame.inputPeak)
			}
			if frame.startSample >= leadEnd && frame.startSample < bodyEnd && frame.active {
				bodies = append(bodies, frame.inputRMS)
			}
		}
		if len(leads) == 0 || len(bodies) == 0 {
			continue
		}
		sort.Float64s(bodies)
		values = append(values, round3(maximum(leads)-quantile(bodies, .5)))
	}
	return values
}

func eventActionDepths(frames []derivedFrame, events []PairedInputEventCandidate, sampleRate float64) []float64 {
	values := []float64{}
	for _, event := range events {
		end := event.Sample + int64(.100*sampleRate)
		depth := 0.0
		seen := false
		for _, frame := range frames {
			if frame.startSample >= event.Sample && frame.startSample < end && frame.active {
				depth = math.Max(depth, frame.actionDB)
				seen = true
			}
		}
		if seen {
			values = append(values, round3(depth))
		}
	}
	return values
}

func periodicRecoveryCandidate(observations []recoveryObservation, events []PairedInputEventCandidate, sampleRate float64) map[string]any {
	out := map[string]any{
		"status": "not_supported", "confidence": 0.0,
		"alternatives":   []string{"program_rhythm", "repeated_transient_pattern", "compressor_recovery_motion"},
		"does_not_prove": []string{"pumping_diagnosis", "release_parameter", "causal_detector_behavior"},
	}
	if len(observations) < 3 || len(events) < 4 || sampleRate <= 0 {
		out["reason"] = "periodic_event_coverage_insufficient"
		return out
	}
	intervals := eventIntervalsMS(events, sampleRate)
	if len(intervals) < 3 {
		out["reason"] = "periodic_event_coverage_insufficient"
		return out
	}
	mean := 0.0
	for _, value := range intervals {
		mean += value
	}
	mean /= float64(len(intervals))
	cv := 0.0
	if mean > 0 {
		cv = stddev(intervals) / mean
	}
	depths := []float64{}
	for _, observation := range observations {
		depths = append(depths, observation.initialDB)
	}
	depthMean := 0.0
	for _, value := range depths {
		depthMean += value
	}
	depthMean /= float64(len(depths))
	depthCV := 0.0
	if depthMean > 0 {
		depthCV = stddev(depths) / depthMean
	}
	confidence := clamp((1-cv)*(1-depthCV)*.75, 0, .75)
	out["event_interval_cv"] = round3(cv)
	out["action_depth_cv"] = round3(depthCV)
	out["confidence"] = round3(confidence)
	if cv <= .15 && depthCV <= .25 {
		out["status"] = "candidate"
		out["observed_relation"] = "gain_action_repeats_with_regular_input_events"
	} else {
		out["status"] = "not_supported"
		out["reason"] = "periodicity_or_action_consistency_insufficient"
	}
	return out
}

func combineDB(values []float64, peak bool) float64 {
	if len(values) == 0 {
		return -160
	}
	if peak {
		return maximum(values)
	}
	power := 0.0
	for _, value := range values {
		power += math.Pow(10, value/10)
	}
	if power <= 0 {
		return -160
	}
	return math.Max(-160, 10*math.Log10(power/float64(len(values))))
}

func compactDistribution(values []float64) map[string]any {
	if len(values) == 0 {
		return map[string]any{"count": 0}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return map[string]any{"count": len(sorted), "min": round3(sorted[0]), "p50": round3(quantile(sorted, .5)), "p90": round3(quantile(sorted, .9)), "max": round3(sorted[len(sorted)-1])}
}

func countActiveFrames(frames []derivedFrame) int {
	count := 0
	for _, frame := range frames {
		if frame.active {
			count++
		}
	}
	return count
}

func samplesMS(samples int, sampleRate float64) float64 {
	if samples <= 0 || sampleRate <= 0 {
		return 0
	}
	return round3(float64(samples) * 1000 / sampleRate)
}

func pearson(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 2 {
		return 0
	}
	meanA, meanB := 0.0, 0.0
	for i := range a {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= float64(len(a))
	meanB /= float64(len(b))
	numerator, ea, eb := 0.0, 0.0, 0.0
	for i := range a {
		da, db := a[i]-meanA, b[i]-meanB
		numerator += da * db
		ea += da * da
		eb += db * db
	}
	if ea <= 0 || eb <= 0 {
		return 1
	}
	return clamp(numerator/math.Sqrt(ea*eb), -1, 1)
}
