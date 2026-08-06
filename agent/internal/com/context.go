package com

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const maxCompactFacts = 24

var windowsAbsolutePath = regexp.MustCompile(`(?i)(^|[\s"'])(([a-z]:[\\/])|(\\\\))`)

func buildLLMContext(projection Projection) LLMContext {
	facts := []map[string]any{
		{
			"layer": "trust_quality", "status": projection.TrustQuality.OverallStatus,
			"can_support_source_description":     projection.TrustQuality.CanSupportSourceDescription,
			"can_support_behavior_observation":   projection.TrustQuality.CanSupportBehaviorObservation,
			"can_support_semantic_planning":      projection.TrustQuality.CanSupportSemanticPlanning,
			"can_support_post_action_evaluation": projection.TrustQuality.CanSupportPostActionEvaluation,
		},
	}
	for _, dimension := range projection.Identifiability.Dimensions {
		facts = append(facts, map[string]any{
			"layer": "identifiability", "dimension": dimension.Dimension, "status": dimension.Status,
			"basis": dimension.Basis, "confidence": dimension.Confidence,
			"does_not_support": append([]string(nil), dimension.DoesNotSupport...),
		})
	}
	if source := projection.SourceDynamics; source != nil {
		facts = append(facts, sourceContextFacts(*source)...)
	}
	if projection.Mode == ModePairedIO {
		for _, item := range []struct {
			name  string
			value *BehaviorProjection
		}{
			{DimensionGainAction, projection.GainAction},
			{DimensionTransientResponse, projection.TransientResponse},
			{DimensionRecoveryMotion, projection.RecoveryMotion},
			{DimensionLevelEffect, projection.LevelEffect},
			{DimensionStereoBehavior, projection.StereoBehavior},
			{DimensionTriggerRelation, projection.TriggerRelation},
		} {
			facts = append(facts, behaviorContextFacts(item.name, item.value)...)
		}
	}
	if projection.Mode == ModeChangeDelta && projection.BehaviorChange != nil {
		facts = append(facts, map[string]any{
			"layer": DimensionBehaviorChange, "status": projection.BehaviorChange.Status,
			"before_projection_id": projection.BehaviorChange.BeforeProjectionID,
			"after_projection_id":  projection.BehaviorChange.AfterProjectionID,
		})
		for _, change := range projection.BehaviorChange.Dimensions {
			facts = append(facts, map[string]any{
				"layer": DimensionBehaviorChange, "dimension": change.Dimension,
				"status": change.Status, "classification": change.Classification,
				"basis": change.Basis, "confidence": change.Confidence, "metrics": change.Metrics,
			})
		}
	}
	facts = sanitizeContextFacts(facts)
	if len(facts) > maxCompactFacts {
		facts = facts[:maxCompactFacts]
	}
	declaredScale := ScaleWholeWindow
	if projection.SourceDynamics != nil {
		declaredScale = projection.SourceDynamics.DeclaredScale
	}
	summary := fmt.Sprintf("COM %s %s projection describes source dynamics at %s scale; compressor behavior is not identifiable from source-only evidence.", projection.Status, projection.Mode, declaredScale)
	next := "Refresh source evidence before using this projection."
	if projection.Mode == ModePairedIO {
		summary = fmt.Sprintf("COM %s paired_io projection reports bounded observed broadband-compressor behavior from aligned input/output evidence; it does not infer control values.", projection.Status)
		next = "Use compact behavior and identifiability facts for future read-only planning; consult the deterministic control surface for parameter values."
	} else if projection.Mode == ModeChangeDelta {
		summary = fmt.Sprintf("COM %s change_delta projection attributes bounded observed compressor-behavior changes between two strictly comparable paired_io children; it does not infer control changes.", projection.Status)
		next = "Use typed behavior deltas for post-action evaluation only; retain both paired_io children as the evidence of record."
	} else if projection.TrustQuality.CanSupportSourceDescription {
		next = "Use these compact source-dynamics facts as context only; paired input/output evidence is required before compressor behavior or parameter planning."
	}
	return LLMContext{
		SummaryMD:              summary,
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           sanitizeEvidenceRefs(projection.EvidenceRefs),
		QualitySummary: map[string]any{
			"overall_status":                     projection.TrustQuality.OverallStatus,
			"blocked_reasons":                    projection.TrustQuality.BlockedReasons,
			"missing_fields":                     projection.TrustQuality.MissingFields,
			"suspect_fields":                     projection.TrustQuality.SuspectFields,
			"stale_fields":                       projection.TrustQuality.StaleFields,
			"can_support_source_description":     projection.TrustQuality.CanSupportSourceDescription,
			"can_support_behavior_observation":   projection.TrustQuality.CanSupportBehaviorObservation,
			"can_support_semantic_planning":      projection.TrustQuality.CanSupportSemanticPlanning,
			"can_support_post_action_evaluation": projection.TrustQuality.CanSupportPostActionEvaluation,
		},
		LimitationNotes:   append([]string(nil), projection.Limitations...),
		SuggestedNextStep: next,
	}
}

func behaviorContextFacts(dimension string, behavior *BehaviorProjection) []map[string]any {
	if behavior == nil {
		return nil
	}
	fact := map[string]any{"layer": dimension, "status": behavior.Status}
	keys := make([]string, 0, len(behavior.Facts))
	for key := range behavior.Facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fact[key] = behavior.Facts[key]
	}
	return []map[string]any{fact}
}

func sourceContextFacts(source SourceDynamics) []map[string]any {
	facts := []map[string]any{
		{"layer": DimensionSourceDynamics, "status": source.Status, "declared_scale": source.DeclaredScale, "analyzed_duration_seconds": source.AnalyzedDuration},
	}
	levels := map[string]any{"layer": DimensionSourceDynamics, "fact": "whole_window_levels"}
	addPointer(levels, "rms_dbfs", source.Levels.RMSDBFS)
	addPointer(levels, "active_rms_dbfs", source.Levels.ActiveRMSDBFS)
	addPointer(levels, "peak_dbfs", source.Levels.PeakDBFS)
	addPointer(levels, "crest_db", source.Levels.CrestDB)
	if len(levels) > 2 {
		facts = append(facts, levels)
	}
	facts = append(facts, map[string]any{
		"layer": DimensionSourceDynamics, "fact": "macro_activity",
		"segment_count": source.Activity.SegmentCount, "valid_segment_count": source.Activity.ValidSegmentCount,
		"active_segment_count": source.Activity.ActiveSegmentCount, "active_ratio": source.Activity.ActiveRatio,
		"coverage_ratio": source.Activity.CoverageRatio,
	})
	macro := map[string]any{"layer": DimensionSourceDynamics, "fact": "macro_variability"}
	addPointer(macro, "rms_range_db", source.MacroDynamics.RMSRangeDB)
	addPointer(macro, "rms_stddev_db", source.MacroDynamics.RMSStdDevDB)
	if source.MacroDynamics.RMSDistribution != nil {
		macro["rms_dbfs_distribution"] = source.MacroDynamics.RMSDistribution
	}
	if len(macro) > 2 {
		facts = append(facts, macro)
	}
	for _, scale := range source.TimeScaleCoverage {
		facts = append(facts, map[string]any{
			"layer": DimensionSourceDynamics, "fact": "time_scale_coverage", "scale": scale.Scale,
			"status": scale.Status, "window_ms": scale.WindowMS, "hop_ms": scale.HopMS,
			"segment_count": scale.SegmentCount, "reason": scale.Reason,
		})
		if len(facts) >= 6 {
			break
		}
	}
	return facts
}

// ContextProjection returns the bounded projection intended for agent/LLM
// consumption. Detailed source evidence and all raw time rows remain external.
func ContextProjection(projection Projection) map[string]any {
	out := map[string]any{
		"schema_version":     projection.SchemaVersion,
		"com_version":        projection.COMVersion,
		"projection_id":      projection.ProjectionID,
		"mode":               projection.Mode,
		"status":             projection.Status,
		"target_ref":         projection.TargetRef,
		"source_dynamics":    projection.SourceDynamics,
		"gain_action":        projection.GainAction,
		"transient_response": projection.TransientResponse,
		"recovery_motion":    projection.RecoveryMotion,
		"level_effect":       projection.LevelEffect,
		"stereo_behavior":    projection.StereoBehavior,
		"trigger_relation":   projection.TriggerRelation,
		"behavior_change":    projection.BehaviorChange,
		"identifiability":    projection.Identifiability,
		"trust_quality":      projection.TrustQuality,
		"llm_context":        projection.LLMContext,
		"evidence_refs":      sanitizeEvidenceRefs(projection.EvidenceRefs),
		"limitations":        projection.Limitations,
	}
	sanitized, ok := sanitizeContextValue(out)
	if !ok {
		return map[string]any{}
	}
	result, _ := sanitized.(map[string]any)
	return result
}

func sanitizeContextFacts(facts []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		if value, ok := sanitizeContextValue(fact); ok {
			if row, ok := value.(map[string]any); ok && len(row) > 0 {
				out = append(out, row)
			}
		}
	}
	return out
}

func sanitizeContextValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case map[string]any:
		out := map[string]any{}
		for key, child := range typed {
			if contextForbiddenKey(key) {
				continue
			}
			if sanitized, ok := sanitizeContextValue(child); ok {
				out[key] = sanitized
			}
		}
		return out, true
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			if sanitized, ok := sanitizeContextValue(row); ok {
				if mapped, ok := sanitized.(map[string]any); ok && len(mapped) > 0 {
					out = append(out, mapped)
				}
			}
		}
		return out, true
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if sanitized, ok := sanitizeContextString(item); ok {
				out = append(out, sanitized)
			}
		}
		return out, len(out) > 0
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized, ok := sanitizeContextValue(item); ok {
				out = append(out, sanitized)
			}
		}
		return out, len(out) > 0
	case string:
		return sanitizeContextString(typed)
	default:
		data, err := json.Marshal(typed)
		if err == nil && (strings.HasPrefix(string(data), "{") || strings.HasPrefix(string(data), "[")) {
			var generic any
			if json.Unmarshal(data, &generic) == nil {
				return sanitizeContextValue(generic)
			}
		}
		return value, true
	}
}

func contextForbiddenKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, forbidden := range []string{
		"raw_package", "raw_waveform", "raw_samples", "samples", "waveform_arrays",
		"time_segments", "time_energy_rows", "frame_trace", "gain_trace", "event_list",
		"spectrogram_tiles", "spectrogram_tile_rows", "tile_payload", "render_file_path",
		"render_path", "file_path", "source_path", "shared_memory", "full_parameter",
		"full_artifact_history",
	} {
		if key == forbidden || strings.Contains(key, forbidden) {
			return true
		}
	}
	return false
}

func sanitizeContextString(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	if trimmed == "" {
		return "", false
	}
	if windowsAbsolutePath.MatchString(trimmed) || strings.HasPrefix(trimmed, "/") || strings.HasPrefix(trimmed, "~/") {
		return "", false
	}
	for _, extension := range []string{".wav", ".wave", ".aiff", ".aif", ".flac", ".mp3", ".ogg"} {
		if strings.Contains(lower, extension) {
			return "", false
		}
	}
	for _, forbidden := range []string{"raw_samples", "raw_waveform", "spectrogram_tiles", "tile_payload", "shared_memory"} {
		if strings.Contains(lower, forbidden) {
			return "", false
		}
	}
	return trimmed, true
}

func sanitizeEvidenceRefs(refs []string) []string {
	out := []string{}
	for _, ref := range refs {
		if sanitized, ok := sanitizeContextString(ref); ok {
			out = append(out, sanitized)
		}
	}
	return uniqueSorted(out)
}

func addPointer(target map[string]any, key string, value *float64) {
	if value != nil {
		target[key] = *value
	}
}
