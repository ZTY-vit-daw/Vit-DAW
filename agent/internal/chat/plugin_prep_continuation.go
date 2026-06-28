package chat

import (
	"context"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
	agentruntime "vit-daw-agent/internal/runtime"
)

type pluginPrepContinuation struct {
	WorkflowData        map[string]any
	InteractionRequests []AgentInteractionRequest
	TypedEvents         []map[string]any
	GoalStatus          string
	StopReason          string
}

func (s *Server) pluginPrepContinuationFromReplies(plan PendingPlan, replies []map[string]any, message string) pluginPrepContinuation {
	if !boolValue(plan.WorkflowData["mix_treatment_preparation"]) {
		return pluginPrepContinuation{}
	}
	trackID, pluginID, pluginName := pluginLoadResultIDs(replies)
	params := pluginPrepParameterResult(replies)
	if pluginID == "" {
		pluginID = cleanContextText(params["plugin_id"])
	}
	if trackID == "" {
		trackID = firstNonEmpty(cleanContextText(params["track_id"]), cleanContextText(plan.WorkflowData["track_id"]))
	}
	if pluginName == "" {
		pluginName = firstNonEmpty(cleanContextText(params["plugin_name"]), cleanContextText(plan.WorkflowData["plugin_name"]))
	}
	terminalStatus := "parameters_extracted"
	terminalReason := "插件准备已完成；没有写入任何插件参数。"
	if len(params) == 0 {
		terminalStatus = "no_parameter_digest"
		terminalReason = "插件准备到达桥接阶段，但没有返回参数摘要；没有写入任何插件参数。"
	} else if pluginID == "" {
		terminalStatus = "missing_plugin_id"
		terminalReason = "插件准备到达桥接阶段，但没有可用的 plugin_id；没有写入任何插件参数。"
	}
	terminal := typedPluginPrepTerminalEvent(plan, trackID, pluginID, pluginName, terminalStatus, terminalReason)
	out := pluginPrepContinuation{TypedEvents: []map[string]any{terminal}}
	if len(params) == 0 || pluginID == "" {
		out.StopReason = "plugin_prep_terminal_no_parameter_digest"
		return out
	}
	return s.pluginPrepWorkerFromDigest(plan, trackID, pluginID, pluginName, params, message, out.TypedEvents)
}

func pluginPrepParameterResult(replies []map[string]any) map[string]any {
	for i := len(replies) - 1; i >= 0; i-- {
		if cleanContextText(replies[i]["command_name"]) != "get_plugin_parameters" {
			continue
		}
		return mapValue(replies[i]["result"])
	}
	return nil
}

func applyPluginPrepContinuationResponse(response map[string]any, prep pluginPrepContinuation) map[string]any {
	if len(response) == 0 {
		return response
	}
	if len(prep.WorkflowData) > 0 {
		response["workflow"] = pluginGrabberLoadCommand
		response["workflow_data"] = prep.WorkflowData
		response["plugin_learning"] = prep.WorkflowData
	}
	if len(prep.InteractionRequests) > 0 {
		response["interaction_requests"] = prep.InteractionRequests
	}
	if len(prep.TypedEvents) > 0 {
		response["typed_events"] = append(mapRowsFromAny(response["typed_events"]), prep.TypedEvents...)
	}
	if strings.TrimSpace(prep.GoalStatus) != "" {
		response["goal_status"] = prep.GoalStatus
	}
	if strings.TrimSpace(prep.StopReason) != "" {
		response["stop_reason"] = prep.StopReason
	}
	return response
}

func (s *Server) continuePluginPrepContinuationInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	if strings.EqualFold(decision, "done") || strings.EqualFold(decision, "stop") || strings.EqualFold(decision, "cancel") {
		event := typedPluginPrepTerminalEvent(PendingPlan{ID: interaction.ID, Context: interaction.RequestContext, WorkflowData: interaction.Payload}, cleanContextText(interaction.Payload["track_id"]), cleanContextText(interaction.Payload["plugin_id"]), cleanContextText(interaction.Payload["plugin_name"]), "stopped_by_user", "用户在插件准备后停止；没有写入任何插件参数。")
		return ChatResponse{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Reply:          "插件准备已完成。没有写入任何插件参数。",
			Workflow:       interaction.Workflow,
			WorkflowData:   interaction.Payload,
			GoalStatus:     string(agentruntime.StatusCompleted),
			StopReason:     "plugin_prep_terminal_user_stopped",
			TypedEvents:    []map[string]any{event},
		}
	}
	payload := copyStringAnyMap(interaction.Payload)
	requestContext := cloneContext(mapValue(payload["request_context"]))
	if len(requestContext) == 0 {
		requestContext = cloneContext(interaction.RequestContext)
	}
	for key, value := range payload {
		if _, exists := requestContext[key]; !exists {
			requestContext[key] = value
		}
	}
	plan := PendingPlan{
		ID:           interaction.ID,
		Workflow:     pluginPrepWorkerWorkflow,
		Context:      requestContext,
		WorkflowData: payload,
	}
	params := mapValue(payload["parameter_digest"])
	if len(params) == 0 {
		event := typedPluginPrepTerminalEvent(plan, cleanContextText(payload["track_id"]), cleanContextText(payload["plugin_id"]), cleanContextText(payload["plugin_name"]), "no_parameter_digest", "插件准备继续执行时没有参数摘要；没有写入任何插件参数。")
		return ChatResponse{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Reply:          "插件准备缺少参数摘要。没有写入任何插件参数。",
			Workflow:       pluginPrepWorkerWorkflow,
			WorkflowData:   payload,
			PluginLearning: payload,
			GoalStatus:     string(agentruntime.StatusCompleted),
			StopReason:     "plugin_prep_terminal_no_parameter_digest",
			TypedEvents:    []map[string]any{event},
		}
	}
	prep := s.pluginPrepWorkerFromDigest(
		plan,
		cleanContextText(payload["track_id"]),
		cleanContextText(payload["plugin_id"]),
		cleanContextText(payload["plugin_name"]),
		params,
		"已根据插件参数生成一个保守的处理候选。确认前不会写入任何插件参数。",
		nil,
	)
	resp := ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               "已根据插件参数生成一个保守的处理候选。确认前不会写入任何插件参数。",
		Workflow:            pluginPrepWorkerWorkflow,
		WorkflowData:        prep.WorkflowData,
		PluginLearning:      prep.WorkflowData,
		InteractionRequests: prep.InteractionRequests,
		TypedEvents:         prep.TypedEvents,
		GoalStatus:          prep.GoalStatus,
		StopReason:          prep.StopReason,
	}
	s.attachInteractionRequests(&resp)
	return resp
}

func typedPluginPrepTerminalEvent(plan PendingPlan, trackID, pluginID, pluginName, status, reason string) map[string]any {
	conversationID := firstNonEmpty(cleanContextText(plan.Context["conversation_id"]), cleanContextText(plan.WorkflowData["conversation_id"]))
	terminal := agentprotocol.TerminalResult{
		ID:      agentprotocol.NormalizeID("terminal", "plugin_prep", plan.ID, trackID, pluginID, status),
		Kind:    agentprotocol.KindTerminalResult,
		Domain:  "plugin_prep",
		Summary: "插件准备已到达桥接终态。",
		Reason:  strings.TrimSpace(reason),
		Status:  strings.TrimSpace(status),
		Source: agentprotocol.Source{
			ConversationID: conversationID,
			GoalID:         cleanContextText(plan.Context["goal_id"]),
			RunID:          cleanContextText(plan.Context["run_id"]),
			CallID:         strings.TrimSpace(plan.ID),
			LegacyKind:     "plugin_grabber_load_completion",
		},
		Metadata: map[string]any{
			"track_id":    strings.TrimSpace(trackID),
			"plugin_id":   strings.TrimSpace(pluginID),
			"plugin_name": strings.TrimSpace(pluginName),
		},
	}
	return agentprotocol.ToMap(agentprotocol.NewEvent(terminal, terminal.Source))
}
