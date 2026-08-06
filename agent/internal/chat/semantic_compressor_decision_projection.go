package chat

import (
	"sort"
	"strings"
)

const semanticCompressorDecisionEvidenceSchema = "semantic_effect.compressor_decision_evidence.v1"

// semanticCompressorDecisionProjection removes the duplicated full COM
// layers and retains only facts requested by the phase-1 intent.
func semanticCompressorDecisionProjection(comContext map[string]any, dimensions []string) map[string]any {
	requestedSet := map[string]bool{}
	requested := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		dimension = strings.TrimSpace(dimension)
		if dimension == "" || requestedSet[dimension] {
			continue
		}
		requestedSet[dimension] = true
		requested = append(requested, dimension)
	}
	sort.Strings(requested)

	llmContext := mapValue(comContext["llm_context"])
	facts := make([]map[string]any, 0)
	for _, fact := range anyMapRows(llmContext["compact_facts"]) {
		layer := firstNonEmptyText(fact, "layer")
		keep := layer == "trust_quality" || requestedSet[layer]
		if layer == "identifiability" {
			keep = requestedSet[firstNonEmptyText(fact, "dimension")]
		}
		if keep {
			facts = append(facts, cloneStringAnyMap(fact))
		}
	}

	qualitySource := mapValue(llmContext["quality_summary"])
	if len(qualitySource) == 0 {
		qualitySource = mapValue(comContext["trust_quality"])
	}
	quality := map[string]any{}
	for _, key := range []string{"overall_status", "can_support_source_description", "can_support_behavior_observation",
		"can_support_semantic_planning", "can_support_post_action_evaluation", "blocked_reasons", "missing_fields", "suspect_fields"} {
		if value, ok := qualitySource[key]; ok {
			quality[key] = value
		}
	}

	out := map[string]any{
		"schema_version":                        semanticCompressorDecisionEvidenceSchema,
		"projection_id":                         firstNonEmptyText(comContext, "projection_id"),
		"mode":                                  firstNonEmptyText(comContext, "mode"),
		"status":                                firstNonEmptyText(comContext, "status"),
		"requested_dimensions":                  requested,
		"facts":                                 facts,
		"quality":                               quality,
		"evidence_refs":                         semanticCompressorEvidenceRefs(comContext),
		"summary":                               firstNonEmptyText(llmContext, "summary_md"),
		"limitations":                           contextStringSlice(llmContext["limitation_notes"]),
		"numeric_parameter_inference_forbidden": true,
	}
	if reason := firstNonEmptyText(comContext, "reason", "fallback_reason"); reason != "" {
		out["reason"] = reason
	}
	return out
}
