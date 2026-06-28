package agentloop

import "strings"

const mixWorkflowDecisionSchemaVersion = "mix_workflow_decision.v0"

type MixWorkflowDecision struct {
	SchemaVersion      string `json:"schema_version"`
	Intent             string `json:"intent"`
	Reason             string `json:"reason,omitempty"`
	ObservationID      string `json:"observation_id,omitempty"`
	PendingCandidateID string `json:"pending_candidate_id,omitempty"`
}

func MixWorkflowDecisionGate(userText string, observation map[string]any, actionKind string) MixWorkflowDecision {
	intent := "observation_only"
	reason := "observation request has no explicit executable action"
	normalizedAction := strings.ToLower(strings.TrimSpace(actionKind))
	switch {
	case messageLoopReadOnlyObservationRequest(userText):
		intent = "observation_only"
		reason = "user explicitly requested read-only acoustic observation"
	case mixWorkflowNeedsClarification(userText, normalizedAction):
		intent = "needs_clarification"
		reason = "target or exact move is unresolved"
	case mixWorkflowExplicitPendingAction(userText, normalizedAction):
		intent = "propose_pending"
		reason = "user requested an executable mix action after observation"
	case strings.EqualFold(strings.TrimSpace(firstMapText(observation, "status")), "terminal"):
		intent = "terminal"
		reason = "observation workflow reached terminal status"
	}
	return MixWorkflowDecision{
		SchemaVersion: mixWorkflowDecisionSchemaVersion,
		Intent:        intent,
		Reason:        reason,
		ObservationID: firstMapText(observation, "observation_id"),
	}
}

func mixWorkflowPendingActionKindBlocked(actionKind string) bool {
	switch strings.ToLower(strings.TrimSpace(actionKind)) {
	case "observation_only", "ask_clarification", "needs_clarification", "":
		return true
	default:
		return false
	}
}

func mixWorkflowExplicitPendingAction(userText, actionKind string) bool {
	switch strings.ToLower(strings.TrimSpace(actionKind)) {
	case "plugin_treatment", "gain_balance", "pan_balance":
		return true
	}
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	return messageLoopTextHasAny(text,
		"apply", "execute", "adjust", "move", "pan left", "pan right", "lower", "raise", "reduce", "boost", "fix", "process",
		"\u6267\u884c", "\u5e94\u7528", "\u8c03\u6574", "\u5904\u7406", "\u4fee", "\u6539", "\u79fb", "\u5f80\u5de6", "\u5f80\u53f3", "\u538b\u4f4e", "\u964d\u4f4e", "\u63d0\u9ad8",
	) && !messageLoopReadOnlyObservationRequest(text)
}

func mixWorkflowNeedsClarification(userText, actionKind string) bool {
	text := strings.ToLower(strings.TrimSpace(userText + " " + actionKind))
	if text == "" {
		return false
	}
	if strings.Contains(text, "clarification") || strings.Contains(text, "\u6f84\u6e05") || strings.Contains(text, "\u786e\u8ba4\u76ee\u6807") {
		return true
	}
	if messageLoopTextHasAny(text, "which track", "what track", "\u54ea\u4e00\u8f68", "\u54ea\u4e2a\u8f68\u9053") {
		return true
	}
	return false
}
