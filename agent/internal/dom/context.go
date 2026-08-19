package dom

import "strings"

// ContextProjection returns the bounded DOM surface intended for Agent/LLM
// consumption. Source time rows, raw evidence, plug-in identity, topology
// generations, and processor-state hashes remain server-side.
func ContextProjection(projection Projection) map[string]any {
	out := map[string]any{
		"schema_version":        projection.SchemaVersion,
		"dom_version":           projection.DOMVersion,
		"projection_id":         projection.ProjectionID,
		"mode":                  projection.Mode,
		"status":                projection.Status,
		"freshness":             projectionFreshness(projection.Status),
		"conditions":            projection.Conditions,
		"target_ref":            projection.TargetRef,
		"peak_structure":        projection.PeakStructure,
		"activity_structure":    projection.ActivityStructure,
		"frequency_time_events": projection.FrequencyTimeEvents,
		"transient_structure":   projection.TransientStructure,
		"band_dynamics":         projection.BandDynamics,
		"behavior_change":       projection.BehaviorChange,
		"dimension_readiness":   projection.DimensionReadiness,
		"trust_quality":         projection.TrustQuality,
		"llm_context":           projection.LLMContext,
		"evidence_refs":         append([]string(nil), projection.EvidenceRefs...),
		"limitations":           append([]string(nil), projection.Limitations...),
	}
	if projection.Mode != ModeSourceOnly && projection.ProcessorScope != nil {
		out["processor_scope"] = map[string]any{"processor_family": projection.ProcessorScope.Family}
	}
	return out
}

func projectionFreshness(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusReady:
		return "fresh"
	case StatusPartial:
		return "partial"
	case StatusStale:
		return "stale"
	case StatusSuspect:
		return "suspect"
	default:
		return "missing"
	}
}
