package agentloop

import (
	"fmt"
	"strings"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

const FreeStateDecisionSchema = "free_state_decision.v1"

const (
	FreeStateNeedsObservation = "needs_observation"
	FreeStateNeedsAction      = "needs_action"
	FreeStateSatisfied        = "satisfied"
	FreeStateBlocked          = "blocked"
)

// FreeStateDecision is the model-owned control state for an ordinary-Agent
// observation/action loop. It carries no execution authority.
type FreeStateDecision struct {
	SchemaVersion    string   `json:"schema_version"`
	Status           string   `json:"status"`
	EvidenceStatus   string   `json:"evidence_status"`
	Summary          string   `json:"summary"`
	RemainingIntent  string   `json:"remaining_intent,omitempty"`
	ProcessorType    string   `json:"processor_type,omitempty"`
	RequestedViewIDs []string `json:"requested_view_ids,omitempty"`
	ObservationID    string   `json:"observation_id,omitempty"`
	Limitations      []string `json:"limitations,omitempty"`
	StopReason       string   `json:"stop_reason,omitempty"`
}

func (d FreeStateDecision) Validate() error {
	if strings.TrimSpace(d.SchemaVersion) != FreeStateDecisionSchema {
		return fmt.Errorf("schema_version must be %s", FreeStateDecisionSchema)
	}
	status := strings.ToLower(strings.TrimSpace(d.Status))
	switch status {
	case FreeStateNeedsObservation:
		if len(nonEmptyFreeStateStrings(d.RequestedViewIDs)) == 0 {
			return fmt.Errorf("needs_observation requires requested_view_ids")
		}
	case FreeStateNeedsAction:
		if strings.TrimSpace(d.RemainingIntent) == "" {
			return fmt.Errorf("needs_action requires remaining_intent")
		}
		switch strings.ToLower(strings.TrimSpace(d.ProcessorType)) {
		case "eq", "compressor", "reverb", "delay", "distortion", "limiter", "native":
		default:
			return fmt.Errorf("needs_action requires processor_type=eq, compressor, reverb, delay, distortion, limiter, or native")
		}
	case FreeStateSatisfied:
		if strings.ToLower(strings.TrimSpace(d.EvidenceStatus)) != "sufficient" {
			return fmt.Errorf("satisfied requires evidence_status=sufficient")
		}
	case FreeStateBlocked:
		// Summary is the canonical human-readable boundary. stop_reason and
		// limitations add structure when available but are not redundant gates.
	default:
		return fmt.Errorf("status must be needs_observation, needs_action, satisfied, or blocked")
	}
	if strings.TrimSpace(d.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

func cloneFreeStateDecision(in *FreeStateDecision) *FreeStateDecision {
	if in == nil {
		return nil
	}
	out := *in
	out.RequestedViewIDs = append([]string(nil), in.RequestedViewIDs...)
	out.Limitations = append([]string(nil), in.Limitations...)
	return &out
}

func nonEmptyFreeStateStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func messageLoopFreeStateContext(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	return messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
}

func messageLoopFreeStatePromptContext(state *runState) map[string]any {
	source := messageLoopFreeStateContext(state)
	if len(source) == 0 {
		return nil
	}
	out := compactSelectedKeys(source, []string{
		"schema_version", "loop_id", "status", "decision_phase", "original_intent", "active_intent",
		"target_ref", "cycle", "max_cycles", "observation_ids", "latest_decision",
		"requires_post_action_observation", "last_error",
	})
	actions := messageLoopMapRows(source["actions"])
	if len(actions) > 0 {
		start := len(actions) - 3
		if start < 0 {
			start = 0
		}
		rows := make([]map[string]any, 0, len(actions)-start)
		for _, action := range actions[start:] {
			row := compactSelectedKeys(action, []string{
				"cycle", "processor_type", "workflow", "status", "summary", "recorded_at",
			})
			if receipt := messageLoopMapValue(action["receipt"]); len(receipt) > 0 {
				row["receipt_summary"] = messageLoopFreeStateReceiptPromptSummary(receipt)
			}
			rows = append(rows, row)
		}
		out["actions"] = rows
	}
	return out
}

func messageLoopFreeStateReceiptPromptSummary(receipt map[string]any) map[string]any {
	out := compactSelectedKeys(receipt, []string{
		"schema_version", "status", "ticket_id", "track_id", "plugin_id", "topology_generation",
	})
	if controller := messageLoopMapValue(receipt["controller_result"]); len(controller) > 0 {
		out["controller_result"] = compactSelectedKeys(controller, []string{"status", "atomic", "control_count"})
	}
	if audit := messageLoopMapValue(receipt["parameter_audit"]); len(audit) > 0 {
		out["parameter_audit"] = compactSelectedKeys(audit, []string{"status", "target_parameter_count", "non_target_parameters_unchanged"})
	}
	if evaluation := messageLoopMapValue(receipt["com_evaluation"]); len(evaluation) > 0 {
		out["com_evaluation"] = messageLoopFreeStateCOMEvaluationPromptSummary(evaluation)
	}
	if verification := messageLoopMapValue(receipt["verification"]); len(verification) > 0 {
		out["verification"] = compactSelectedKeys(verification, []string{
			"status", "structural", "acoustic", "user_acceptance", "summary",
		})
	}
	return out
}

func messageLoopFreeStateCOMEvaluationPromptSummary(evaluation map[string]any) map[string]any {
	out := compactSelectedKeys(evaluation, []string{"com_version", "status", "mode", "projection_id"})
	change := messageLoopMapValue(evaluation["behavior_change"])
	if len(change) == 0 {
		return out
	}
	compactChange := compactSelectedKeys(change, []string{"status"})
	dimensions := messageLoopMapRows(change["dimensions"])
	if len(dimensions) > 0 {
		rows := make([]map[string]any, 0, len(dimensions))
		for _, dimension := range dimensions {
			row := compactSelectedKeys(dimension, []string{
				"dimension", "status", "classification", "confidence", "basis", "metrics",
			})
			rows = append(rows, row)
		}
		compactChange["dimensions"] = rows
	}
	out["behavior_change"] = compactChange
	return out
}

func messageLoopFreeStateActive(state *runState) bool {
	row := messageLoopFreeStateContext(state)
	if len(row) == 0 {
		return false
	}
	status := strings.ToLower(firstMapText(row, "status"))
	return status != "completed" && status != "cancelled" && status != "blocked"
}

func messageLoopFreeStateOutputIssue(state *runState, out messageLoopOutput) string {
	active := messageLoopFreeStateActive(state)
	if !active {
		if out.FreeStateDecision != nil {
			return "free_state is only valid while a free_state_reasoning_loop context is active"
		}
		return ""
	}
	if out.NeedsClarification || strings.TrimSpace(out.FailureReason) != "" {
		return ""
	}
	if out.FreeStateDecision == nil {
		return "an active free-state reasoning loop requires one free_state_decision.v1 on every reasoning turn; preserve the original intent and state whether observation, action, satisfaction, or a blocker comes next"
	}
	if err := out.FreeStateDecision.Validate(); err != nil {
		return "invalid free_state decision: " + err.Error()
	}
	status := strings.ToLower(strings.TrimSpace(out.FreeStateDecision.Status))
	switch status {
	case FreeStateNeedsObservation:
		if out.Final || len(out.ToolCalls) == 0 {
			return "needs_observation must be non-final and call ccb.observation_catalog or ccb.observation_request"
		}
		for _, call := range out.ToolCalls {
			if !messageLoopIsCCBObservationTool(call) {
				return "needs_observation may call only CCB observation catalog/request tools in the free-state loop"
			}
		}
	case FreeStateNeedsAction:
		if len(out.ToolCalls) != 0 {
			return "needs_action must contain no direct mutation tool calls; the local governed processor router owns materialization and confirmation"
		}
		if !messageLoopFreeStateProcessorGoverned(out.FreeStateDecision.ProcessorType) {
			return "the requested processor_type has no governed open-semantic execution path in this release; only eq and compressor are executable here, so return blocked for the unsupported family instead of substituting, recommending, or loading another processor"
		}
		ctx := messageLoopFreeStateContext(state)
		if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
			return "cannot select another processor action after an action until a fresh CCB observation_request has returned in this reasoning cycle"
		}
		if messageLoopFreeStateProcessorAlreadyApplied(ctx, out.FreeStateDecision.ProcessorType) {
			return "cannot automatically select the same processor_type again after it was already applied in this free-state loop; use its action receipt and fresh CCB evidence to evaluate other unresolved acoustic clauses, or return blocked for user review instead of auto-iterating"
		}
	case FreeStateSatisfied:
		if len(out.ToolCalls) != 0 {
			return "satisfied must contain no tool calls"
		}
		ctx := messageLoopFreeStateContext(state)
		if freeStateBool(ctx["requires_post_action_observation"]) && !messageLoopHasSuccessfulCCBObservationRequest(state) {
			return "cannot mark the original intent satisfied after an action until a fresh CCB observation_request has returned in this reasoning cycle"
		}
	case FreeStateBlocked:
		if len(out.ToolCalls) != 0 {
			return "blocked must contain no tool calls"
		}
		if messageLoopFreeStateClaimsUnavailableObservation(state, *out.FreeStateDecision) {
			return "cannot claim that observation is unavailable: ccb.observation_catalog and ccb.observation_request are available in this free-state turn; use the CCB catalog/request protocol, then decide the treatment family from the returned bounded evidence and limitations"
		}
	}
	return ""
}

func messageLoopFreeStateProcessorGoverned(processorType string) bool {
	switch strings.ToLower(strings.TrimSpace(processorType)) {
	case "eq", "compressor":
		return true
	default:
		return false
	}
}

func messageLoopFreeStateProcessorAlreadyApplied(ctx map[string]any, processorType string) bool {
	processorType = strings.ToLower(strings.TrimSpace(processorType))
	if processorType == "" {
		return false
	}
	for _, action := range messageLoopMapRows(ctx["actions"]) {
		if strings.EqualFold(firstMapText(action, "status"), "applied") &&
			strings.EqualFold(firstMapText(action, "processor_type"), processorType) {
			return true
		}
	}
	return false
}

func messageLoopFreeStateClaimsUnavailableObservation(state *runState, decision FreeStateDecision) bool {
	if state == nil || !allowedTool("ccb.observation_request", state.input.AllowedTools) || messageLoopHasSuccessfulCCBObservationRequest(state) {
		return false
	}
	ctx := messageLoopFreeStateContext(state)
	if phase := strings.ToLower(firstMapText(ctx, "decision_phase")); phase != "" && phase != "processor_selection" {
		return false
	}
	text := strings.ToLower(strings.Join([]string{
		decision.Summary,
		decision.StopReason,
		strings.Join(decision.Limitations, " "),
	}, " "))
	mentionsObservation := strings.Contains(text, "observation") || strings.Contains(text, "observe") ||
		strings.Contains(text, "mix.observe") || strings.Contains(text, "\u89c2\u5bdf")
	mentionsInterface := strings.Contains(text, "interface") || strings.Contains(text, "tool") ||
		strings.Contains(text, "\u63a5\u53e3") || strings.Contains(text, "\u5de5\u5177")
	claimsUnavailable := strings.Contains(text, "unavailable") || strings.Contains(text, "not available") ||
		strings.Contains(text, "cannot access") || strings.Contains(text, "\u4e0d\u53ef\u7528") ||
		strings.Contains(text, "\u65e0\u6cd5\u4f7f\u7528")
	return mentionsObservation && mentionsInterface && claimsUnavailable
}

func messageLoopFreeStateTerminalReply(out messageLoopOutput) (string, string, bool) {
	if out.FreeStateDecision == nil || len(out.ToolCalls) != 0 {
		return "", "", false
	}
	status := strings.ToLower(strings.TrimSpace(out.FreeStateDecision.Status))
	if status != FreeStateSatisfied && status != FreeStateBlocked {
		return "", "", false
	}
	reply := strings.TrimSpace(out.Reply)
	if reply == "" {
		reply = strings.TrimSpace(out.FreeStateDecision.Summary)
	}
	if reply == "" {
		reply = "The free-state reasoning loop reached a terminal decision."
	}
	return reply, status, true
}

func coerceMessageLoopFreeStateObservationOutput(state *runState, out messageLoopOutput) messageLoopOutput {
	if !messageLoopFreeStateActive(state) || out.FreeStateDecision == nil ||
		!strings.EqualFold(strings.TrimSpace(out.FreeStateDecision.Status), FreeStateNeedsObservation) ||
		len(nonEmptyFreeStateStrings(out.FreeStateDecision.RequestedViewIDs)) == 0 {
		return out
	}
	if len(out.ToolCalls) > 0 {
		for _, call := range out.ToolCalls {
			name := strings.ToLower(strings.TrimSpace(normalizedActionName(call, executorpkg.Result{})))
			if messageLoopIsCCBObservationName(name) {
				return out
			}
			if name != "mix.observe" && name != "mix_observe" {
				return out
			}
		}
	}
	call := planner.ToolCall{
		Tool:   "ccb.observation_request",
		Args:   map[string]any{"view_ids": nonEmptyFreeStateStrings(out.FreeStateDecision.RequestedViewIDs)},
		Reason: firstNonEmpty(out.FreeStateDecision.Summary, "fulfill the model's structured CCB observation request"),
	}
	if len(out.ToolCalls) > 0 {
		call.ID = out.ToolCalls[0].ID
		call.PlanItemID = out.ToolCalls[0].PlanItemID
		call.Reason = firstNonEmpty(out.ToolCalls[0].Reason, call.Reason)
	}
	out.ToolCalls = []planner.ToolCall{call}
	out.Final = false
	return out
}

func freeStateBool(value any) bool {
	if typed, ok := value.(bool); ok {
		return typed
	}
	switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

func messageLoopIsCCBObservationTool(call planner.ToolCall) bool {
	return messageLoopIsCCBObservationName(normalizedActionName(call, executorpkg.Result{}))
}

func messageLoopIsCCBObservationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ccb.observation_catalog", "ccb_observation_catalog", "ccb.observation_request", "ccb_observation_request":
		return true
	default:
		return false
	}
}

func messageLoopIsCCBObservationRequestName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ccb.observation_request", "ccb_observation_request":
		return true
	default:
		return false
	}
}

func messageLoopHasSuccessfulCCBObservationRequest(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		name := firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))
		if messageLoopIsCCBObservationRequestName(name) && messageLoopExecutionSucceeded(record) && messageLoopCCBObservationResultUsable(messageLoopMapValue(record["result"])) {
			return true
		}
	}
	observation := state.recentObservation
	if observation != nil && observation.Error == "" && !toolStatusFailed(observation.Status) &&
		messageLoopIsCCBObservationRequestName(firstNonEmpty(observation.Tool, observation.CommandName)) &&
		messageLoopCCBObservationBundleUsable(observation.Summary) {
		return true
	}
	return false
}

func messageLoopCCBObservationResultUsable(result map[string]any) bool {
	if len(result) == 0 {
		return false
	}
	return messageLoopCCBObservationBundleUsable(messageLoopMapValue(result["bundle"]))
}

func messageLoopCCBObservationBundleUsable(bundle map[string]any) bool {
	if len(bundle) == 0 || bundle["read_only"] != true || bundle["mutation_authority"] != false {
		return false
	}
	status := strings.ToLower(firstMapText(bundle, "status"))
	if status != "ready" && status != "partial" {
		return false
	}
	return len(messageLoopMapValue(bundle["views"])) > 0
}
