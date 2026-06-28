package agentloop

import (
	"strings"
	"time"

	"vit-daw-agent/internal/mixdiagnosis"
)

func messageLoopAttachDiagnosisToTreatment(state *runState, treatment *MixTreatmentPending) {
	if state == nil || treatment == nil {
		return
	}
	diag := messageLoopBuildDiagnosisContext(state, treatment.Intent, treatment.TargetRef, treatment.ActionKind, treatment.ProcessorType)
	if diag.ID == "" {
		return
	}
	diagMap := diag.Map()
	treatment.DiagnosisContextID = diag.ID
	treatment.DiagnosisContext = cloneMap(diagMap)
	treatment.EvidenceRefs = messageLoopAppendUniqueStrings(treatment.EvidenceRefs, diag.EvidenceRefs...)
	if treatment.Fingerprint == nil {
		treatment.Fingerprint = map[string]any{}
	}
	treatment.Fingerprint["diagnosis_context_id"] = diag.ID
	state.executionMemory.MixDiagnosisContextID = diag.ID
	state.executionMemory.MixDiagnosisContext = cloneMap(diagMap)
}

func messageLoopAttachDiagnosisToMixTick(state *runState, candidate *PendingMixTickCandidate, intent string) {
	if state == nil || candidate == nil {
		return
	}
	actionKind := "gain_balance"
	if strings.Contains(strings.ToLower(strings.TrimSpace(candidate.Operation)), "pan") {
		actionKind = "pan_balance"
	}
	targetRef := ""
	if strings.TrimSpace(candidate.TrackID) != "" {
		targetRef = "track:" + strings.TrimSpace(candidate.TrackID)
	}
	diag := messageLoopBuildDiagnosisContext(state, intent, targetRef, actionKind, "utility")
	if diag.ID == "" {
		return
	}
	diagMap := diag.Map()
	if candidate.Evidence == nil {
		candidate.Evidence = map[string]any{}
	}
	candidate.Evidence["diagnosis_context_id"] = diag.ID
	candidate.Evidence["diagnosis_context"] = cloneMap(diagMap)
	candidate.Evidence["diagnosis_evidence_refs"] = append([]string(nil), diag.EvidenceRefs...)
	if candidate.Fingerprint == nil {
		candidate.Fingerprint = map[string]any{}
	}
	candidate.Fingerprint["diagnosis_context_id"] = diag.ID
	state.executionMemory.MixDiagnosisContextID = diag.ID
	state.executionMemory.MixDiagnosisContext = cloneMap(diagMap)
}

func messageLoopBuildDiagnosisContext(state *runState, intent, targetRef, actionKind, processorType string) mixdiagnosis.Context {
	if state == nil {
		return mixdiagnosis.Context{}
	}
	createdAt := ""
	if !state.startedAt.IsZero() {
		createdAt = state.startedAt.UTC().Format(time.RFC3339Nano)
	}
	return mixdiagnosis.Build(mixdiagnosis.Input{
		ConversationID:     messageLoopConversationID(state),
		GoalID:             state.goal.GoalID,
		RunID:              state.goal.RunID,
		UserIntent:         firstNonEmpty(strings.TrimSpace(intent), strings.TrimSpace(state.input.UserText), strings.TrimSpace(state.goal.Summary)),
		TargetRef:          strings.TrimSpace(targetRef),
		ActionKind:         strings.TrimSpace(actionKind),
		ProcessorType:      strings.TrimSpace(processorType),
		ObservationSummary: messageLoopLatestMixObservationSummary(state),
		RequestContext:     messageLoopDiagnosisRequestContext(state),
		CreatedAt:          createdAt,
	})
}

func messageLoopLatestMixObservationSummary(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		return cloneMap(state.recentObservation.Summary)
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) || !messageLoopIsMixObservationName(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))) {
			continue
		}
		result := messageLoopMapValue(record["result"])
		if len(result) == 0 {
			result = record
		}
		if summary := mixObservationPromptSummary(result); len(summary) > 0 {
			return summary
		}
	}
	for i := len(state.trace) - 1; i >= 0; i-- {
		event := state.trace[i]
		if event.ToolResult == nil || toolStatusFailed(event.ToolResult.Status) || !messageLoopIsMixObservationName(event.ToolResult.Tool) {
			continue
		}
		if summary := mixObservationPromptSummary(event.ToolResult.Result); len(summary) > 0 {
			return summary
		}
	}
	return nil
}

func messageLoopDiagnosisRequestContext(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	ctx := cloneMap(state.input.Context)
	if ctx == nil {
		ctx = map[string]any{}
	}
	for _, row := range []map[string]any{state.input.State, state.contextSnapshot, state.projectHistory} {
		for key, value := range row {
			if _, exists := ctx[key]; !exists {
				ctx[key] = value
			}
		}
	}
	return ctx
}

func messageLoopAppendUniqueStrings(base []string, values ...string) []string {
	out := append([]string(nil), base...)
	seen := map[string]bool{}
	for _, value := range out {
		seen[strings.TrimSpace(value)] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
