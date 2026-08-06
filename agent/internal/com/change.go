package com

import (
	"math"
	"strings"
	"time"
)

const (
	ChangeStronger      = "stronger"
	ChangeWeaker        = "weaker"
	ChangeUnchanged     = "unchanged_within_tolerance"
	ChangeMixed         = "mixed"
	ChangeNonComparable = "non_comparable"
)

type changeComparability struct {
	status          string
	gates           []QualityGate
	reasons         []string
	inputEquivalent bool
	stateChanged    bool
}

func buildChangeDeltaProjection(input Input) Projection {
	generatedAt := strings.TrimSpace(input.CreatedAt)
	if generatedAt == "" {
		generatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if input.Change == nil || input.Change.Before == nil || input.Change.After == nil {
		return missingChangeProjection(input, generatedAt, "change_children_missing")
	}
	before, after := *input.Change.Before, *input.Change.After
	comparison := compareChangeChildren(before, after)
	refs := uniqueSorted(append(append([]string{}, before.EvidenceRefs...), after.EvidenceRefs...))
	changes := deriveBehaviorChanges(before, after, comparison, refs)
	behaviorStatus := comparison.status
	if comparison.status == StatusReady {
		for _, change := range changes {
			if change.Dimension == DimensionTriggerRelation {
				continue
			}
			if change.Status != StatusReady {
				behaviorStatus = StatusPartial
				break
			}
		}
	}
	behavior := &BehaviorChangeProjection{
		Status: behaviorStatus, BeforeProjectionID: before.ProjectionID, AfterProjectionID: after.ProjectionID,
		Dimensions: changes, EvidenceRefs: refs,
		Limitations: uniqueSorted(append([]string{"behavior_delta_does_not_identify_control_values", "children_remain_evidence_of_record"}, comparison.reasons...)),
	}
	trust := changeTrust(before, after, comparison, behaviorStatus, refs)
	projection := Projection{
		SchemaVersion: SchemaVersion, COMVersion: Version, Mode: ModeChangeDelta, Status: behaviorStatus,
		ObservationID: strings.TrimSpace(input.ObservationID), MixSessionID: strings.TrimSpace(input.MixSessionID),
		TargetRef: cloneMap(input.TargetRef), ProcessorScope: after.ProcessorScope, Conditions: after.Conditions,
		EvidenceInputs: EvidenceInputs{SourceEvidenceID: before.EvidenceInputs.SourceEvidenceID + "+" + after.EvidenceInputs.SourceEvidenceID,
			SourceSchema: before.EvidenceInputs.SourceSchema, SourceStatus: behaviorStatus, EvidenceRefs: refs,
			InputTrace: after.EvidenceInputs.InputTrace, BeforeProjectionID: before.ProjectionID, AfterProjectionID: after.ProjectionID},
		SourceDynamics: cloneSourceDynamics(before.SourceDynamics), BehaviorChange: behavior,
		Identifiability: changeIdentifiability(changes), TrustQuality: trust, EvidenceRefs: refs,
		Limitations: uniqueSorted(append([]string{"no_parameter_inference_or_mutation_authority", "generic_mom_fxm_delta_not_recomputed"}, behavior.Limitations...)),
		GeneratedAt: generatedAt,
	}
	projection.ProjectionID = stableProjectionID(projection)
	projection.LLMContext = buildLLMContext(projection)
	return projection
}

func missingChangeProjection(input Input, generatedAt, reason string) Projection {
	p := Projection{SchemaVersion: SchemaVersion, COMVersion: Version, Mode: ModeChangeDelta, Status: StatusMissing,
		ObservationID: input.ObservationID, MixSessionID: input.MixSessionID, TargetRef: cloneMap(input.TargetRef),
		BehaviorChange:  &BehaviorChangeProjection{Status: StatusMissing, Limitations: []string{reason}},
		Identifiability: Identifiability{Dimensions: []IdentifiabilityDimension{changeUnavailableDimension(DimensionBehaviorChange, reason)}},
		TrustQuality: TrustQuality{OverallStatus: StatusMissing,
			ModeGates:      []QualityGate{{ID: "change_children", Required: true, Status: StatusMissing, Reason: reason}},
			BlockedReasons: []string{reason}, MissingFields: []string{"change_children"},
			Limitations: []string{reason, "no_parameter_inference_or_mutation_authority"}},
		Limitations: []string{reason, "no_parameter_inference_or_mutation_authority"}, GeneratedAt: generatedAt}
	p.ProjectionID = stableProjectionID(p)
	p.LLMContext = buildLLMContext(p)
	return p
}

func compareChangeChildren(before, after Projection) changeComparability {
	checks := []struct {
		id, reason string
		pass       bool
		stale      bool
		suspect    bool
	}{
		{"child_modes", "paired_io_children_required", before.Mode == ModePairedIO && after.Mode == ModePairedIO, false, false},
		{"child_schema", "child_schema_or_version_mismatch", before.SchemaVersion == after.SchemaVersion && before.COMVersion == after.COMVersion && before.SchemaVersion == SchemaVersion, true, false},
		{"distinct_projection_ids", "identical_projection_ids", before.ProjectionID != "" && after.ProjectionID != "" && before.ProjectionID != after.ProjectionID, true, false},
		{"child_trust", "child_projection_not_trustworthy", changeChildUsable(before) && changeChildUsable(after), false, true},
		{"processor_target", "processor_target_mismatch", sameProcessorTarget(before.ProcessorScope, after.ProcessorScope), true, false},
		{"processor_topology", "processor_topology_mismatch", sameProcessorTopology(before.ProcessorScope, after.ProcessorScope), true, false},
		{"source_identity", "source_or_clip_revision_mismatch", before.Conditions.SourceRevision != "" && before.Conditions.SourceRevision == after.Conditions.SourceRevision && before.Conditions.ClipRevision == after.Conditions.ClipRevision, true, false},
		{"sample_window", "sample_window_mismatch", before.Conditions.StartSample == after.Conditions.StartSample && before.Conditions.EndSample == after.Conditions.EndSample && before.Conditions.EndSample > before.Conditions.StartSample, true, false},
		{"sample_format", "sample_format_mismatch", sameSampleFormat(before.Conditions, after.Conditions), true, false},
		{"tap_identity", "tap_identity_mismatch", before.Conditions.InputTap == after.Conditions.InputTap && before.Conditions.OutputTap == after.Conditions.OutputTap && before.Conditions.InputTap == "compressor_input" && before.Conditions.OutputTap == "compressor_output", true, false},
		{"render_policy", "render_or_tail_policy_mismatch", before.Conditions.RenderMode == after.Conditions.RenderMode && before.Conditions.Deterministic == after.Conditions.Deterministic && before.Conditions.TailPolicy == after.Conditions.TailPolicy, true, false},
		{"latency_policy", "latency_policy_mismatch", before.Conditions.LatencyMethod == after.Conditions.LatencyMethod, true, false},
		{"analyzer", "analyzer_or_resolution_mismatch", before.Conditions.AnalyzerVersion == after.Conditions.AnalyzerVersion && sameAnalysisResolutions(before.Conditions.AnalysisResolutions, after.Conditions.AnalysisResolutions), true, false},
		{"input_trace_identity", "input_trace_not_equivalent", sameInputTrace(before.EvidenceInputs.InputTrace, after.EvidenceInputs.InputTrace), false, true},
		{"state_change", "processor_state_unchanged", before.ProcessorScope.ProcessorStateHash != "" && after.ProcessorScope.ProcessorStateHash != "" && before.ProcessorScope.ProcessorStateHash != after.ProcessorScope.ProcessorStateHash, true, false},
		{"render_revision_change", "render_revision_unchanged", before.Conditions.RenderRevision != "" && after.Conditions.RenderRevision != "" && before.Conditions.RenderRevision != after.Conditions.RenderRevision, true, false},
	}
	gates, reasons := []QualityGate{}, []string{}
	status := StatusReady
	for _, check := range checks {
		gateStatus := StatusReady
		if !check.pass {
			gateStatus = StatusMissing
			if check.stale {
				gateStatus = StatusStale
			}
			if check.suspect {
				gateStatus = StatusSuspect
			}
			reasons = append(reasons, check.reason)
			if gateStatus == StatusSuspect {
				status = StatusSuspect
			} else if status != StatusSuspect && gateStatus == StatusStale {
				status = StatusStale
			} else if status == StatusReady {
				status = StatusMissing
			}
		}
		gates = append(gates, QualityGate{ID: check.id, Required: true, Status: gateStatus, Reason: missingReason(check.pass, check.reason)})
	}
	return changeComparability{status: status, gates: gates, reasons: uniqueSorted(reasons),
		inputEquivalent: checks[13].pass, stateChanged: checks[14].pass && checks[15].pass}
}

func deriveBehaviorChanges(before, after Projection, comparison changeComparability, refs []string) []BehaviorDimensionChange {
	dimensions := []string{DimensionGainAction, DimensionTransientResponse, DimensionRecoveryMotion, DimensionLevelEffect, DimensionStereoBehavior, DimensionTriggerRelation}
	out := make([]BehaviorDimensionChange, 0, len(dimensions))
	for _, dimension := range dimensions {
		if comparison.status != StatusReady {
			out = append(out, BehaviorDimensionChange{Dimension: dimension, Status: comparison.status,
				Classification: ChangeNonComparable, BeforeStatus: behaviorStatusFor(before, dimension), AfterStatus: behaviorStatusFor(after, dimension),
				EvidenceRefs: refs, Limitations: append([]string(nil), comparison.reasons...)})
			continue
		}
		out = append(out, compareBehaviorDimension(before, after, dimension, refs))
	}
	return out
}

func compareBehaviorDimension(before, after Projection, dimension string, refs []string) BehaviorDimensionChange {
	b, a := behaviorFor(before, dimension), behaviorFor(after, dimension)
	change := BehaviorDimensionChange{Dimension: dimension, Status: StatusReady, Classification: ChangeUnchanged,
		BeforeStatus: behaviorStatus(b), AfterStatus: behaviorStatus(a), EvidenceRefs: refs, Confidence: .9}
	if b == nil || a == nil || !behaviorComparable(b) || !behaviorComparable(a) {
		change.Status, change.Classification, change.Confidence = StatusMissing, ChangeNonComparable, 0
		change.Limitations = []string{"dimension_unavailable_or_not_identifiable_in_child"}
		return change
	}
	switch dimension {
	case DimensionGainAction:
		change.Basis = "effective_gain_action_change"
		change.Classification, change.Metrics = compareGainAction(b, a)
	case DimensionTransientResponse:
		change.Basis = "leading_edge_rounding_change"
		change.Classification, change.Metrics = compareTransient(b, a)
	case DimensionRecoveryMotion:
		change.Basis = "recovery_persistence_change"
		change.Classification, change.Metrics = compareRecovery(b, a)
	case DimensionLevelEffect:
		change.Basis = "absolute_scalar_level_effect_change"
		change.Classification, change.Metrics = compareLevelEffect(b, a)
	case DimensionStereoBehavior:
		if b.Status == StatusNotApplicable && a.Status == StatusNotApplicable {
			change.Status, change.Classification, change.Confidence = StatusNotApplicable, ChangeUnchanged, 1
			change.Basis = "mono_not_applicable"
		} else {
			change.Basis = "channel_action_asymmetry_change"
			change.Classification, change.Metrics = compareStereo(b, a)
		}
	case DimensionTriggerRelation:
		change.Status, change.Classification, change.Confidence = StatusMissing, ChangeNonComparable, 0
		change.Basis = "aligned_band_relation_unavailable"
		change.Limitations = []string{"trigger_relation_not_identifiable_in_com3_children"}
	}
	if b.Status == StatusPartial || a.Status == StatusPartial {
		if change.Status == StatusReady {
			change.Status = StatusPartial
		}
		change.Confidence = round3(change.Confidence * .75)
		change.Limitations = append(change.Limitations, "one_or_both_child_dimensions_bounded")
	}
	change.Limitations = uniqueSorted(append(change.Limitations, "classification_is_behavior_change_not_control_change"))
	return change
}

func compareGainAction(b, a *BehaviorProjection) (string, map[string]any) {
	bP50, bok := distributionFact(b, "reduction_depth_db_distribution", "p50")
	aP50, aok := distributionFact(a, "reduction_depth_db_distribution", "p50")
	bP90, bok90 := distributionFact(b, "reduction_depth_db_distribution", "p90")
	aP90, aok90 := distributionFact(a, "reduction_depth_db_distribution", "p90")
	bDuty, bdok := numberFact(b, "gain_action_duty_cycle")
	aDuty, adok := numberFact(a, "gain_action_duty_cycle")
	metrics := map[string]any{"depth_p50_delta_db": round3(aP50 - bP50), "depth_p90_delta_db": round3(aP90 - bP90), "duty_cycle_delta": round3(aDuty - bDuty), "depth_tolerance_db": .5, "duty_tolerance": .03}
	if !(bok && aok && bok90 && aok90 && bdok && adok) {
		return ChangeNonComparable, metrics
	}
	return classifySigned([]signedChange{{aP50 - bP50, .5}, {aP90 - bP90, .5}, {aDuty - bDuty, .03}}), metrics
}

func compareTransient(b, a *BehaviorProjection) (string, map[string]any) {
	bContrast, bc := distributionFact(b, "transient_to_body_contrast_change_db", "p50")
	aContrast, ac := distributionFact(a, "transient_to_body_contrast_change_db", "p50")
	bLead, bl := distributionFact(b, "leading_edge_action_db", "p50")
	aLead, al := distributionFact(a, "leading_edge_action_db", "p50")
	roundingDelta := -(aContrast - bContrast)
	leadDelta := aLead - bLead
	metrics := map[string]any{"rounding_delta_db": round3(roundingDelta), "leading_edge_action_delta_db": round3(leadDelta), "tolerance_db": .5}
	if !(bc && ac && bl && al) {
		return ChangeNonComparable, metrics
	}
	return classifySigned([]signedChange{{roundingDelta, .5}, {leadDelta, .5}}), metrics
}

func compareRecovery(b, a *BehaviorProjection) (string, map[string]any) {
	bComplete, bok := numberFact(b, "complete_recovery_ratio")
	aComplete, aok := numberFact(a, "complete_recovery_ratio")
	bScore, bs := recoveryBandScore(b)
	aScore, as := recoveryBandScore(a)
	persistence := bComplete - aComplete
	bandDelta := aScore - bScore
	metrics := map[string]any{"persistence_delta": round3(persistence), "return_band_score_delta": round3(bandDelta), "ratio_tolerance": .08, "band_tolerance": .25}
	if !(bok && aok && bs && as) {
		return ChangeNonComparable, metrics
	}
	return classifySigned([]signedChange{{persistence, .08}, {bandDelta, .25}}), metrics
}

func compareLevelEffect(b, a *BehaviorProjection) (string, map[string]any) {
	bRMS, br := numberFact(b, "rms_delta_db")
	aRMS, ar := numberFact(a, "rms_delta_db")
	bPeak, bp := numberFact(b, "peak_delta_db")
	aPeak, ap := numberFact(a, "peak_delta_db")
	rmsMagnitude, peakMagnitude := math.Abs(aRMS)-math.Abs(bRMS), math.Abs(aPeak)-math.Abs(bPeak)
	metrics := map[string]any{"rms_effect_magnitude_delta_db": round3(rmsMagnitude), "peak_effect_magnitude_delta_db": round3(peakMagnitude), "tolerance_db": .25, "semantic_note": "stronger_means_larger_absolute_level_effect_not_more_compression"}
	if !(br && ar && bp && ap) {
		return ChangeNonComparable, metrics
	}
	return classifySigned([]signedChange{{rmsMagnitude, .25}, {peakMagnitude, .25}}), metrics
}

func compareStereo(b, a *BehaviorProjection) (string, map[string]any) {
	bDiff, bok := numberFact(b, "action_difference_p90_db")
	aDiff, aok := numberFact(a, "action_difference_p90_db")
	delta := aDiff - bDiff
	metrics := map[string]any{"asymmetry_p90_delta_db": round3(delta), "tolerance_db": .25, "semantic_note": "stronger_means_more_channel_action_asymmetry"}
	if !(bok && aok) {
		return ChangeNonComparable, metrics
	}
	return classifySigned([]signedChange{{delta, .25}}), metrics
}

type signedChange struct{ value, tolerance float64 }

func classifySigned(changes []signedChange) string {
	positive, negative := false, false
	for _, change := range changes {
		if change.value > change.tolerance {
			positive = true
		}
		if change.value < -change.tolerance {
			negative = true
		}
	}
	if positive && negative {
		return ChangeMixed
	}
	if positive {
		return ChangeStronger
	}
	if negative {
		return ChangeWeaker
	}
	return ChangeUnchanged
}

func changeTrust(before, after Projection, comparison changeComparability, status string, refs []string) TrustQuality {
	blocked, approximate, suspect, stale, missing := classifyGates(comparison.gates)
	ready := comparison.status == StatusReady
	return TrustQuality{OverallStatus: status, ModeGates: comparison.gates,
		Coverage:   map[string]any{"before_projection_id": before.ProjectionID, "after_projection_id": after.ProjectionID, "compared_dimension_count": 6},
		SameSource: gateReady(comparison.gates, "source_identity"), SameWindow: gateReady(comparison.gates, "sample_window"), SameFormat: gateReady(comparison.gates, "sample_format"),
		TapOrderValid: gateReady(comparison.gates, "tap_identity"), Deterministic: before.Conditions.Deterministic && after.Conditions.Deterministic,
		LatencyAligned: before.TrustQuality.LatencyAligned && after.TrustQuality.LatencyAligned, InputEquivalent: comparison.inputEquivalent,
		CanSupportSourceDescription: comparison.inputEquivalent, CanSupportBehaviorObservation: ready,
		CanSupportSemanticPlanning: ready, CanSupportPostActionEvaluation: ready,
		BlockedReasons: blocked, ApproximateFields: approximate, SuspectFields: suspect, StaleFields: stale, MissingFields: missing,
		Limitations: uniqueSorted(append([]string{"change_delta_grants_no_mutation_authority"}, comparison.reasons...)), EvidenceRefs: refs}
}

func changeIdentifiability(changes []BehaviorDimensionChange) Identifiability {
	rows := make([]IdentifiabilityDimension, 0, len(changes)+1)
	identified := true
	for _, change := range changes {
		status := Identified
		confidence := change.Confidence
		if change.Status == StatusNotApplicable {
			status = NotApplicable
		}
		if change.Status == StatusPartial {
			status = Bounded
		}
		if change.Classification == ChangeNonComparable || change.Status == StatusMissing || change.Status == StatusStale || change.Status == StatusSuspect {
			status, confidence = NotIdentifiable, 0
			identified = false
		}
		rows = append(rows, IdentifiabilityDimension{Dimension: change.Dimension, Status: status, Basis: "comparable_paired_io_children", Confidence: confidence,
			Supports: []string{"behavior_change:" + change.Dimension}, DoesNotSupport: []string{"parameter_change_inference", "causal_control_value"}, Limitations: append([]string(nil), change.Limitations...)})
	}
	rootStatus, rootConfidence := Identified, .9
	if !identified {
		rootStatus, rootConfidence = Bounded, .6
	}
	rows = append(rows, IdentifiabilityDimension{Dimension: DimensionBehaviorChange, Status: rootStatus, Basis: "strict_change_delta_comparability", Confidence: rootConfidence,
		Supports: []string{"post_action_behavior_evaluation"}, DoesNotSupport: []string{"parameter_change_inference", "mutation_authority"}})
	return Identifiability{Dimensions: rows}
}

func changeUnavailableDimension(dimension, reason string) IdentifiabilityDimension {
	return IdentifiabilityDimension{Dimension: dimension, Status: NotIdentifiable, Basis: "change_delta", DoesNotSupport: []string{"behavior_change", "parameter_change_inference"}, Limitations: []string{reason}}
}

func changeChildUsable(p Projection) bool {
	return p.Status == StatusReady || p.Status == StatusPartial
}
func sameProcessorTarget(a, b ProcessorScope) bool {
	return a.TrackID != "" && a.TrackID == b.TrackID && a.PluginInstanceID != "" && a.PluginInstanceID == b.PluginInstanceID && a.PluginPosition == b.PluginPosition && a.SupportClass == "single_band_broadband" && b.SupportClass == a.SupportClass
}
func sameProcessorTopology(a, b ProcessorScope) bool {
	return a.TopologyClass == b.TopologyClass && a.TopologyGeneration != "" && a.TopologyGeneration == b.TopologyGeneration && a.ChainHash != "" && a.ChainHash == b.ChainHash
}
func sameSampleFormat(a, b Conditions) bool {
	return math.Abs(a.SampleRate-b.SampleRate) < .01 && a.ChannelCount == b.ChannelCount && a.ChannelLayout == b.ChannelLayout && a.SampleRate > 0 && a.ChannelCount > 0
}
func sameInputTrace(a, b InputTraceIdentity) bool {
	return a.Method != "" && a.Method == b.Method && a.SHA256 != "" && a.SHA256 == b.SHA256 && a.FrameCount > 0 && a.FrameCount == b.FrameCount && a.ChannelCount == b.ChannelCount && a.QuantizationDB == b.QuantizationDB
}
func sameAnalysisResolutions(a, b []AnalysisResolution) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func cloneSourceDynamics(source *SourceDynamics) *SourceDynamics {
	if source == nil {
		return nil
	}
	copy := *source
	copy.EvidenceRefs = append([]string(nil), source.EvidenceRefs...)
	copy.Limitations = append([]string(nil), source.Limitations...)
	return &copy
}
func behaviorFor(p Projection, dimension string) *BehaviorProjection {
	switch dimension {
	case DimensionGainAction:
		return p.GainAction
	case DimensionTransientResponse:
		return p.TransientResponse
	case DimensionRecoveryMotion:
		return p.RecoveryMotion
	case DimensionLevelEffect:
		return p.LevelEffect
	case DimensionStereoBehavior:
		return p.StereoBehavior
	case DimensionTriggerRelation:
		return p.TriggerRelation
	}
	return nil
}
func behaviorStatusFor(p Projection, dimension string) string {
	return behaviorStatus(behaviorFor(p, dimension))
}
func behaviorStatus(p *BehaviorProjection) string {
	if p == nil {
		return StatusMissing
	}
	return p.Status
}
func behaviorComparable(p *BehaviorProjection) bool {
	return p != nil && (p.Status == StatusReady || p.Status == StatusPartial || p.Status == StatusNotApplicable)
}
func numberFact(p *BehaviorProjection, key string) (float64, bool) {
	if p == nil {
		return 0, false
	}
	value, ok := p.Facts[key].(float64)
	return value, ok && finite(value)
}
func distributionFact(p *BehaviorProjection, key, field string) (float64, bool) {
	if p == nil {
		return 0, false
	}
	values, ok := p.Facts[key].(map[string]any)
	if !ok {
		return 0, false
	}
	value, ok := values[field].(float64)
	return value, ok && finite(value)
}
func recoveryBandScore(p *BehaviorProjection) (float64, bool) {
	if p == nil {
		return 0, false
	}
	raw, ok := p.Facts["return_time_band_counts"].(map[string]int)
	if !ok {
		if generic, yes := p.Facts["return_time_band_counts"].(map[string]any); yes {
			raw = map[string]int{}
			for k, v := range generic {
				if n, ok := v.(int); ok {
					raw[k] = n
				}
				if f, ok := v.(float64); ok {
					raw[k] = int(f)
				}
			}
		} else {
			return 0, false
		}
	}
	weights := map[string]float64{"under_50_ms": .1, "50_to_200_ms": .5, "200_to_500_ms": 1, "500_to_1000_ms": 2, "over_1000_ms": 3, "not_returned_before_next_event": 4}
	sum, count := 0.0, 0
	for k, n := range raw {
		sum += weights[k] * float64(n)
		count += n
	}
	if count == 0 {
		return 0, false
	}
	return sum / float64(count), true
}
func gateReady(gates []QualityGate, id string) bool {
	for _, gate := range gates {
		if gate.ID == id {
			return gate.Status == StatusReady
		}
	}
	return false
}
