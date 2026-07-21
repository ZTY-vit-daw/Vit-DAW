package agentloop

import (
	"strings"

	"vit-daw-agent/internal/actionworkflow"
)

func ActionWorkflowPendingSpecFromMixTreatment(treatment MixTreatmentPending) actionworkflow.PendingSpec {
	return actionworkflow.PendingSpec{
		TargetRef:       strings.TrimSpace(treatment.TargetRef),
		ActionKind:      strings.TrimSpace(treatment.ActionKind),
		ProcessorType:   strings.TrimSpace(treatment.ProcessorType),
		DeltaDB:         treatment.DeltaDB,
		DeltaPan:        treatment.DeltaPan,
		TargetPan:       cloneFloat64Pointer(treatment.TargetPan),
		PluginID:        strings.TrimSpace(treatment.PluginID),
		PluginName:      strings.TrimSpace(treatment.PluginName),
		Control:         strings.TrimSpace(treatment.Control),
		Target:          cloneMap(treatment.Target),
		Reasoning:       strings.TrimSpace(treatment.ReasoningSummary),
		Confidence:      strings.TrimSpace(treatment.Confidence),
		EvidenceRefs:    append([]string(nil), treatment.EvidenceRefs...),
		NeedsResolution: append([]string(nil), treatment.NeedsResolution...),
		ObservationID:   strings.TrimSpace(treatment.ObservationID),
		Intent:          strings.TrimSpace(treatment.Intent),
	}
}

func ActionWorkflowDisplayFromMixTreatment(treatment MixTreatmentPending) actionworkflow.DisplayModel {
	return actionworkflow.RenderPending(
		ActionWorkflowPendingSpecFromMixTreatment(treatment),
		actionworkflow.PreflightGate{CanCreateExecutablePending: true, CanSuggest: true},
	)
}

func messageLoopActionWorkflowSpecFromTreatment(treatment MixTreatmentPending) actionworkflow.PendingSpec {
	return ActionWorkflowPendingSpecFromMixTreatment(treatment)
}

func messageLoopActionWorkflowPreflightGate(state *runState) (actionworkflow.PreflightGate, bool) {
	var fallback actionworkflow.PreflightGate
	for _, proj := range messageLoopRecentMOMProjections(state) {
		trust := messageLoopMapValue(proj["trust_quality"])
		if len(trust) == 0 {
			if len(fallback.EvidenceRefs) == 0 {
				fallback.EvidenceRefs = messageLoopStringSlice(proj["evidence_refs"])
			}
			continue
		}
		if messageLoopActionWorkflowTrustBlocksActionPreflight(trust) {
			gate := actionworkflow.GateFromMOMProjection(proj)
			return gate, true
		}
		evidenceRefs := messageLoopStringSlice(trust["evidence_refs"])
		if len(evidenceRefs) == 0 {
			evidenceRefs = messageLoopStringSlice(trust["source_refs"])
		}
		if len(evidenceRefs) == 0 {
			evidenceRefs = messageLoopStringSlice(proj["evidence_refs"])
		}
		if len(fallback.EvidenceRefs) == 0 {
			fallback = actionworkflow.PreflightGate{
				CanCreateExecutablePending: true,
				CanSuggest:                 true,
				Status:                     firstMapText(trust, "overall_status", "status"),
				EvidenceRefs:               evidenceRefs,
			}
		}
	}
	if fallback.CanCreateExecutablePending || fallback.CanSuggest || len(fallback.EvidenceRefs) > 0 {
		return fallback, false
	}
	return actionworkflow.PreflightGate{CanCreateExecutablePending: true, CanSuggest: true}, false
}

func messageLoopActionWorkflowTrustBlocksActionPreflight(trust map[string]any) bool {
	if len(trust) == 0 {
		return false
	}
	if value, ok := trust["can_support_action_preflight"]; ok && !messageLoopBool(value) {
		return true
	}
	for _, reason := range messageLoopStringSlice(trust["blocked_reasons"]) {
		if strings.Contains(strings.ToLower(strings.TrimSpace(reason)), "action_preflight") {
			return true
		}
	}
	return messageLoopHasActionRelevantTrustField(messageLoopStringSlice(trust["suspect_fields"])) ||
		messageLoopHasActionRelevantTrustField(messageLoopStringSlice(trust["stale_fields"]))
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
