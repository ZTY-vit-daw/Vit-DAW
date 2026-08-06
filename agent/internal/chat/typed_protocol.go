package chat

import (
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/policy"
)

func typedApprovalFromDecision(planID string, decision policy.Decision, conversationID, goalID, runID, reason string) agentprotocol.ApprovalRequest {
	name := strings.TrimSpace(decision.Name)
	if name == "" {
		name = firstNonEmpty(commandNameForEvent(decision.Command), cleanContextText(decision.Command["tool"]))
	}
	resourceDomain, operation := typedResourceDomainForCommand(name, decision.Command)
	return agentprotocol.ApprovalRequest{
		ID:                 agentprotocol.NormalizeID("approval", planID, goalID, runID, name),
		Kind:               agentprotocol.KindApprovalRequest,
		ResourceDomain:     resourceDomain,
		Operation:          operation,
		Scope:              "single_call",
		TargetRef:          typedTargetRefFromCommand(decision.Command),
		Reason:             firstNonEmpty(strings.TrimSpace(reason), strings.TrimSpace(decision.Reason)),
		Status:             agentprotocol.ApprovalStatusWaitingUser,
		AvailableDecisions: []string{"approve_once", "deny"},
		Source: agentprotocol.Source{
			ConversationID: strings.TrimSpace(conversationID),
			GoalID:         strings.TrimSpace(goalID),
			RunID:          strings.TrimSpace(runID),
			CallID:         strings.TrimSpace(planID),
			LegacyKind:     "confirmation_plan",
		},
	}
}

func typedApprovalDecisionEvents(planID string, plan PendingPlan, conversationID, goalID, runID, decisionStatus, reason string) []map[string]any {
	decision, ok := firstApprovalDecision(plan.Decisions)
	if !ok {
		return nil
	}
	approval := typedApprovalFromDecision(planID, decision, conversationID, goalID, runID, reason)
	approval.Status = agentprotocol.LegacyApprovalStatus(decisionStatus)
	return []map[string]any{agentprotocol.ToMap(agentprotocol.NewEvent(approval, approval.Source))}
}

func typedApprovalFromToolResult(in executorpkg.Input, result executorpkg.Result, err error) agentprotocol.ApprovalRequest {
	call := in.ToolCall
	cmd := cloneStringAnyMap(call.Command)
	if len(cmd) == 0 {
		cmd = cloneStringAnyMap(call.Args)
	}
	if cmd == nil {
		cmd = map[string]any{}
	}
	name := firstNonEmpty(strings.TrimSpace(result.CommandName), commandNameForEvent(cmd), strings.TrimSpace(result.Tool), strings.TrimSpace(call.Tool))
	resourceDomain, operation := typedResourceDomainForCommand(name, cmd)
	return agentprotocol.ApprovalRequest{
		ID:                 agentprotocol.NormalizeID("approval", result.ToolCallID, in.GoalID, in.RunID, name),
		Kind:               agentprotocol.KindApprovalRequest,
		ResourceDomain:     resourceDomain,
		Operation:          operation,
		Scope:              "single_call",
		TargetRef:          typedTargetRefFromCommand(cmd),
		Reason:             firstNonEmpty(strings.TrimSpace(result.Preview), strings.TrimSpace(call.Reason), errorText(err)),
		Status:             agentprotocol.LegacyApprovalStatus(result.Status),
		AvailableDecisions: []string{"approve_once", "deny"},
		Source: agentprotocol.Source{
			ConversationID: eventConversationIDFromContext(in.Context),
			GoalID:         strings.TrimSpace(in.GoalID),
			RunID:          strings.TrimSpace(in.RunID),
			ToolCallID:     firstNonEmpty(strings.TrimSpace(result.ToolCallID), strings.TrimSpace(call.ID)),
			AgentActionID:  strings.TrimSpace(result.AgentActionID),
			LegacyKind:     "tool_requires_confirmation",
		},
	}
}

func typedUserInputFromInteraction(req AgentInteractionRequest) agentprotocol.UserInputRequest {
	return agentprotocol.UserInputRequest{
		ID:           agentprotocol.NormalizeID("input", req.ID),
		Kind:         agentprotocol.KindUserInputRequest,
		QuestionType: typedQuestionTypeFromInteraction(req),
		Prompt:       firstNonEmpty(strings.TrimSpace(req.Body), strings.TrimSpace(req.Title)),
		Options:      typedOptionsFromInteraction(req),
		Status:       agentprotocol.LegacyUserInputStatus(req.Status),
		Source: agentprotocol.Source{
			ConversationID: strings.TrimSpace(req.ConversationID),
			GoalID:         strings.TrimSpace(req.GoalID),
			RunID:          strings.TrimSpace(req.RunID),
			CallID:         strings.TrimSpace(req.ID),
			LegacyKind:     strings.TrimSpace(req.Kind),
			Metadata: map[string]any{
				"workflow": strings.TrimSpace(req.Workflow),
				"stage":    strings.TrimSpace(req.Stage),
				"type":     strings.TrimSpace(req.Type),
			},
		},
	}
}

func attachTypedUserInput(req *AgentInteractionRequest) {
	if req == nil {
		return
	}
	if !interactionIsUserInput(*req) {
		return
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}
	typed := typedUserInputFromInteraction(*req)
	req.Payload["typed_state"] = agentprotocol.ToMap(typed)
	req.Payload["typed_event"] = agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source))
	if req.Data == nil {
		req.Data = req.Payload
	}
}

func attachTypedInteractionRequests(resp *ChatResponse) {
	if resp == nil {
		return
	}
	seen := map[string]struct{}{}
	for _, event := range resp.TypedEvents {
		if key := typedEventIdentity(event); key != "" {
			seen[key] = struct{}{}
		}
	}
	for i := range resp.InteractionRequests {
		attachTypedUserInput(&resp.InteractionRequests[i])
		if event := typedInteractionEvent(resp.InteractionRequests[i]); event != nil {
			if key := typedEventIdentity(event); key != "" {
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
			}
			resp.TypedEvents = append(resp.TypedEvents, event)
		}
	}
}

func interactionIsUserInput(req AgentInteractionRequest) bool {
	kind := strings.ToLower(strings.TrimSpace(firstNonEmpty(req.Kind, req.Type)))
	if kind == "" || kind == "confirmation" || kind == "mix_treatment_confirmation" {
		return false
	}
	context := strings.ToLower(strings.TrimSpace(req.Source + " " + req.Workflow + " " + req.Stage + " " + req.Type))
	if strings.Contains(context, "plugin_grabber") {
		return true
	}
	return len(req.Fields) > 0 || len(req.Questions) > 0 || len(req.ReviewItems) > 0 || kind == "form" || strings.Contains(kind, "question")
}

func typedQuestionTypeFromInteraction(req AgentInteractionRequest) string {
	text := strings.ToLower(strings.TrimSpace(req.Workflow + " " + req.Stage + " " + req.Kind + " " + req.Type))
	switch {
	case strings.Contains(text, "display_domain"), strings.Contains(text, "experiment"):
		return "workflow_detail"
	case strings.Contains(text, "mix"):
		return "aesthetic_choice"
	default:
		return "missing_info"
	}
}

func typedOptionsFromInteraction(req AgentInteractionRequest) []string {
	if len(req.Actions) == 0 {
		return nil
	}
	out := make([]string, 0, len(req.Actions))
	for _, action := range req.Actions {
		label := firstNonEmpty(strings.TrimSpace(action.ID), strings.TrimSpace(action.Label))
		if label != "" {
			out = append(out, label)
		}
	}
	return out
}

func typedResourceDomainForCommand(name string, cmd map[string]any) (string, string) {
	text := strings.ToLower(strings.TrimSpace(name + " " + commandNameForEvent(cmd) + " " + cleanContextText(cmd["tool"])))
	switch {
	case strings.Contains(text, "plugin.set_parameter"), strings.Contains(text, "set_plugin_param"):
		return "plugin.param_write", "commit"
	case strings.Contains(text, "plugin.load"), strings.Contains(text, "load_to_rack"), strings.Contains(text, "instantiate"), strings.Contains(text, "rack.add_node"):
		return "plugin.load", "commit"
	case strings.Contains(text, "mix.apply_tick"), strings.Contains(text, "mix_apply_tick"), strings.Contains(text, "track.volume"), strings.Contains(text, "track.pan"), strings.Contains(text, "set_volume"), strings.Contains(text, "set_pan"):
		return "mix.commit", "commit"
	case strings.Contains(text, "version.restore"), strings.Contains(text, "track.delete"), strings.Contains(text, "clip.remove"):
		return "project.commit", "destructive_commit"
	case strings.Contains(text, "render"), strings.Contains(text, "bounce"):
		return "render.audio", "preview"
	case strings.Contains(text, "web."), strings.Contains(text, "browser."), strings.Contains(text, "aigc"):
		return "aigc.network", "external_network"
	case strings.Contains(text, "artifact"):
		return "artifact.read_write", "prepare"
	default:
		return "project.commit", "commit"
	}
}

func typedTargetRefFromCommand(cmd map[string]any) string {
	if len(cmd) == 0 {
		return ""
	}
	if trackID := firstNonEmpty(cleanContextText(cmd["track_id"]), cleanContextText(cmd["target_track_id"]), cleanContextText(cmd["selected_track_id"])); trackID != "" {
		return "track:" + trackID
	}
	if pluginID := firstNonEmpty(cleanContextText(cmd["plugin_id"]), cleanContextText(cmd["plugin_item_id"]), cleanContextText(cmd["selected_plugin_id"])); pluginID != "" {
		return "plugin:" + pluginID
	}
	if clipID := firstNonEmpty(cleanContextText(cmd["clip_id"]), cleanContextText(cmd["selected_clip_id"])); clipID != "" {
		return "clip:" + clipID
	}
	return ""
}

func firstApprovalDecision(decisions []policy.Decision) (policy.Decision, bool) {
	for _, decision := range decisions {
		if decision.Risk == policy.RiskConfirm {
			return decision, true
		}
	}
	if len(decisions) > 0 {
		return decisions[0], true
	}
	return policy.Decision{}, false
}

func typedApprovalEventFromPendingTool(planID string, cont *agentloop.Continuation, decisions []policy.Decision, conversationID string) map[string]any {
	if cont == nil {
		return nil
	}
	decision, ok := firstApprovalDecision(decisions)
	if !ok {
		return nil
	}
	reason := ""
	if cont.PendingToolCall != nil {
		reason = cont.PendingToolCall.Reason
	}
	typed := typedApprovalFromDecision(planID, decision, conversationID, cont.GoalID, cont.RunID, reason)
	return agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source))
}

func typedInteractionEvent(req AgentInteractionRequest) map[string]any {
	if !interactionIsUserInput(req) {
		return nil
	}
	typed := typedUserInputFromInteraction(req)
	return agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source))
}

func typedEventState(row map[string]any) map[string]any {
	if len(row) == 0 {
		return nil
	}
	return mapValue(row["state"])
}

func typedEventIdentity(row map[string]any) string {
	if len(row) == 0 {
		return ""
	}
	eventType := cleanContextText(row["event_type"])
	stateID := cleanContextText(row["state_id"])
	status := cleanContextText(row["status"])
	state := mapValue(row["state"])
	if eventType == "" {
		eventType = cleanContextText(state["kind"])
	}
	if stateID == "" {
		stateID = cleanContextText(state["id"])
	}
	if status == "" {
		status = cleanContextText(state["status"])
	}
	if eventType == "" || stateID == "" {
		return ""
	}
	return eventType + "|" + stateID + "|" + status
}
