package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/mixcontrolsurface"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	mixSessionEntryWorkflow = "mix_session_entry"

	mixModeAuto = "auto_mix"
	mixModeCo   = "co_mix"

	mixStateWaitingConfirmation    = "waiting_mix_confirmation"
	mixStatePreparing              = "preparing"
	mixStateReadyObservation       = "ready_for_observation"
	mixStateObservationReady       = "observation_ready"
	mixStateObservationPartial     = "observation_partial"
	mixStateObservationUnavailable = "observation_unavailable"
	mixStateTuningRunning          = "tuning_running"
	mixStateTuningStopped          = "tuning_stopped"
	mixStateTuningPaused           = "tuning_paused"
	mixStateWaitingReview          = "waiting_review"
	mixStateTuningComplete         = "tuning_complete"
	mixStateRollbackAvailable      = "rollback_available"
	mixStateCancelled              = "cancelled"
	mixStateFailed                 = "failed"
)

const (
	mixAutoTuneMaxRounds       = 3
	mixAutoTuneStepDB          = 0.75
	mixFallbackExecutorType    = "fallback_executor"
	mixFallbackExecutorVersion = "mix_auto_tune_fallback.v0.2"

	mixReviewWaiting    = "waiting_review"
	mixReviewEffective  = "effective"
	mixReviewNotObvious = "not_obvious"
	mixReviewReviewed   = "reviewed"
	mixReviewFailed     = "failed"
	mixReviewRolledBack = "rolled_back"

	mixStopReasonMaxRounds              = "max_rounds"
	mixStopReasonUserIntervention       = "user_intervention"
	mixStopReasonNoSignificantChange    = "no_significant_change"
	mixStopReasonObservationUnavailable = "observation_unavailable"
	mixStopReasonRollbackRequested      = "rollback_requested"
)

const (
	mixInteractionPlanningChat            = "planning_chat"
	mixInteractionPlanDrafting            = "plan_drafting"
	mixInteractionControlSurfacePublished = "control_surface_published"
	mixInteractionReadyForTick            = "ready_for_tick"
	mixInteractionFastTickRunning         = "fast_tick_running"
	mixInteractionWaitingPlannerReview    = "waiting_planner_review"
	mixInteractionLearningRequired        = "learning_required"

	mixBoardVisibilityHidden    = "hidden"
	mixBoardVisibilityCollapsed = "collapsed"
	mixBoardVisibilityPublished = "published"
	mixBoardVisibilityExecution = "execution"

	mixPlannerPolicyAutoAssist    = "auto_assist"
	mixPlannerPolicyCoMixDirector = "comix_director"
)

const (
	mixControlPlanSchemaVersion       = "mix_control_plan.v1"
	mixMacroControlPanelSchemaVersion = "mix_macro_control_panel.v1"
	mixPlannerPrepSchemaVersion       = "mix_planner_prep.v1"
	mixPlanningWorkspaceSchemaVersion = "mix_planning_workspace.v1"
	mixBuiltInVolumeMacroRole         = "gain_staging"
	mixBuiltInVolumeMacroControl      = "track.volume"

	mixTickPacketSchemaVersion   = "mix_tick_packet.v1"
	mixTickDecisionSchemaVersion = "mix_tick_decision.v1"
	mixSingleTickExecutorType    = "single_tick_executor"
	mixSingleTickExecutorVersion = "mix_single_tick.v0.1"
)

type MixSession struct {
	MixSessionID       string           `json:"mix_session_id"`
	ConversationID     string           `json:"conversation_id,omitempty"`
	Mode               string           `json:"mode"`
	State              string           `json:"state"`
	TargetRef          MixTargetRef     `json:"target_ref"`
	GoalText           string           `json:"goal_text"`
	UserNote           string           `json:"user_note,omitempty"`
	ApprovedPlugins    []string         `json:"approved_plugins"`
	ControlSurfaceID   string           `json:"control_surface_id"`
	ExecutorType       string           `json:"executor_type,omitempty"`
	ExecutorVersion    string           `json:"executor_version,omitempty"`
	ReviewStatus       string           `json:"review_status,omitempty"`
	StopReason         string           `json:"stop_reason,omitempty"`
	InteractionPhase   string           `json:"interaction_phase,omitempty"`
	MixBoardVisibility string           `json:"mixboard_visibility,omitempty"`
	PlannerPolicy      string           `json:"planner_policy,omitempty"`
	FastModelProfile   string           `json:"fast_model_profile,omitempty"`
	RoundCount         int              `json:"round_count"`
	MaxRounds          int              `json:"max_rounds"`
	JournalRefs        []string         `json:"journal_refs"`
	BlockingPoint      string           `json:"blocking_point,omitempty"`
	Preparation        []map[string]any `json:"preparation,omitempty"`
	PlannerPrep        map[string]any   `json:"mix_planner_prep,omitempty"`
	CreatedAt          string           `json:"created_at,omitempty"`
	UpdatedAt          string           `json:"updated_at,omitempty"`
}

type MixTargetRef struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Label      string `json:"label"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
}

func (s *Server) runMixSessionEntryChat(ctx context.Context, conversationID string, req ChatRequest, agentMode string) (ChatResponse, bool) {
	if !legacyMixSessionWorkflowEnabled(req.Context) {
		return ChatResponse{}, false
	}
	mode := mixModeFromRequest(req.Message, req.Context)
	if mode == "" {
		return ChatResponse{}, false
	}
	goalID, runID := goalIDsFromContext(req.Context)
	projectPath := projectPathFromChatContext(req.Context)
	target := resolveMixTargetRef(req.Message, req.Context)
	goalText := firstNonEmpty(cleanContextText(req.Context["goal_text"]), cleanContextText(req.Context["mix_goal_text"]), req.Message)
	if agentModeFromString(agentMode) == agentModePlan {
		reply := mixPlanModeReply(mode, target, goalText)
		projectHistory := s.harness.RecordConversationNodeForProject(ctx, projectPath, "vit", reply, goalID, runID)
		if len(projectHistory) == 0 {
			projectHistory = s.harness.ProjectHistorySummaryForProject(ctx, goalID, projectPath)
		}
		return ChatResponse{
			ConversationID: conversationID,
			GoalID:         goalID,
			RunID:          runID,
			AgentMode:      agentMode,
			Reply:          reply,
			GoalStatus:     string(agentruntime.StatusCompleted),
			ProjectHistory: projectHistory,
		}, true
	}
	session := pendingMixSession(mode, target, goalText)
	session.ConversationID = conversationID
	data := mixSessionWorkflowData(conversationID, goalID, runID, req.Context, session)
	reqCard := s.mixSessionInteractionRequest(conversationID, goalID, runID, data, target)
	reply := mixSessionEntryReply(session, target)
	resp := ChatResponse{
		ConversationID:      conversationID,
		GoalID:              goalID,
		RunID:               runID,
		AgentMode:           agentMode,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{reqCard},
		GoalStatus:          string(agentruntime.StatusWaitingConfirmation),
		CurrentStep:         "mix_session_entry",
	}
	return resp, true
}

func (s *Server) invokeMixSessionEntryWorkflow(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !legacyMixSessionWorkflowEnabled(mergeContext(req.Context, req.Args)) {
		if !isMixSessionEntryInvoke(req) {
			return harness.InvokeResponse{}, false
		}
		return disabledMixSessionEntryInvokeResponse(), true
	}
	mode := mixModeFromRequest("", mergeContext(req.Context, req.Args))
	if mode == "" {
		return harness.InvokeResponse{}, false
	}
	goalID := firstNonEmpty(strings.TrimSpace(req.GoalID), cleanContextText(req.Context["goal_id"]))
	runID := firstNonEmpty(strings.TrimSpace(req.RunID), cleanContextText(req.Context["run_id"]))
	conversationID := cleanContextText(req.Context["conversation_id"])
	target := resolveMixTargetRef(cleanContextText(req.Args["goal_text"]), mergeContext(req.Context, req.Args))
	goalText := firstNonEmpty(cleanContextText(req.Args["goal_text"]), cleanContextText(req.Context["goal_text"]), cleanContextText(req.Context["user_message"]), mixModeLabel(mode))
	if agentModeFromContext(req.Context) == agentModePlan {
		return harness.InvokeResponse{
			Status:      "ok",
			Tool:        "mix.session_entry",
			CommandName: mixSessionEntryWorkflow,
			Result: map[string]any{
				"reply":       mixPlanModeReply(mode, target, goalText),
				"workflow":    mixSessionEntryWorkflow,
				"goal_status": string(agentruntime.StatusCompleted),
			},
		}, true
	}
	session := pendingMixSession(mode, target, goalText)
	session.ConversationID = conversationID
	data := mixSessionWorkflowData(conversationID, goalID, runID, req.Context, session)
	card := s.mixSessionInteractionRequest(conversationID, goalID, runID, data, target)
	response := map[string]any{
		"reply":                mixSessionEntryReply(session, target),
		"workflow":             mixSessionEntryWorkflow,
		"workflow_data":        data,
		"mix_session":          mixSessionMap(session),
		"interaction_requests": []AgentInteractionRequest{card},
		"goal_status":          string(agentruntime.StatusWaitingConfirmation),
	}
	return harness.InvokeResponse{
		Status:               "needs_confirmation",
		Tool:                 "mix.session_entry",
		CommandName:          mixSessionEntryWorkflow,
		RequiresConfirmation: true,
		Result:               response,
	}, true
}

func (s *Server) continueMixSessionInteraction(ctx context.Context, interaction PendingInteraction, payload map[string]any, decision string) ChatResponse {
	data := copyStringAnyMap(interaction.Payload)
	if len(data) == 0 {
		data = copyStringAnyMap(interaction.Data)
	}
	session := mixSessionFromMap(mapValue(data["mix_session"]))
	if session.Mode == "" {
		session = pendingMixSession(cleanContextText(data["mode"]), mixTargetFromMap(mapValue(data["target_ref"])), cleanContextText(data["goal_text"]))
	}
	if !legacyMixSessionWorkflowEnabled(mergeContext(interaction.RequestContext, payload)) && !isCurrentConversationalMixAction(data, payload, session) {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if strings.EqualFold(decision, "cancel_mix_session") || strings.EqualFold(decision, "cancel") {
		session.State = mixStateCancelled
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		observation := mapValue(data["mix_observation"])
		updateMixBoardRuntimeState(observation, session, nil)
		if len(observation) > 0 {
			data["mix_observation"] = observation
		}
		return ChatResponse{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Reply:          "已取消自动混音任务设计。",
			Workflow:       mixSessionEntryWorkflow,
			WorkflowData:   data,
			MixSession:     mixSessionMap(session),
			GoalStatus:     string(agentruntime.StatusCancelled),
		}
	}
	if strings.EqualFold(decision, "edit_mix_target") || strings.EqualFold(decision, "modify_target") {
		targetText := cleanContextText(payload["target_text"])
		target := mixManualTargetRef(targetText)
		if target.ID == "" {
			target = resolveMixTargetRef(targetText, mergeContext(interaction.RequestContext, payload))
		}
		if target.ID != "" {
			session.TargetRef = target
		}
		session.State = mixStateWaitingConfirmation
		data["mix_session"] = mixSessionMap(session)
		reqCard := s.mixSessionInteractionRequest(interaction.ConversationID, interaction.GoalID, interaction.RunID, data, session.TargetRef)
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               "请确认更新后的混音目标。",
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{reqCard},
			GoalStatus:          string(agentruntime.StatusWaitingConfirmation),
		}
	}
	if strings.EqualFold(decision, "revise_mixboard") || strings.EqualFold(decision, "refresh_observation") || strings.EqualFold(decision, "update_mixboard") {
		return s.reviseMixBoardInteraction(ctx, interaction, data, session, payload, decision)
	}
	if isLegacyMixSessionApprovalDecision(decision) {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if isDeprecatedMixPlannerDecision(decision) {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if strings.EqualFold(decision, "publish_mixboard") {
		return s.publishMixBoardInteraction(ctx, interaction, data, session, payload)
	}
	if strings.EqualFold(decision, "confirm_mix_planner_plan") {
		if payload == nil {
			payload = map[string]any{}
		}
		payload["publish_mixboard_confirmed"] = true
		return s.advanceMixPlannerInteraction(ctx, interaction, data, session, payload)
	}
	if strings.EqualFold(decision, "advance_mix_planner") || strings.EqualFold(decision, "submit_mix_planner_answers") || strings.EqualFold(decision, "continue_mix_planning") {
		return s.advanceMixPlannerInteraction(ctx, interaction, data, session, payload)
	}
	if strings.EqualFold(decision, "confirm_control_surface") {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if strings.EqualFold(decision, "execute_single_mix_tick") || strings.EqualFold(decision, "start_mix_tuning") || strings.EqualFold(decision, "auto_tune_mix") {
		if !isCurrentConversationalMixAction(data, payload, session) {
			return staleMixActionResponse(interaction, data, session)
		}
		return s.runSingleMixTickInteraction(ctx, interaction, data, session, payload)
	}
	if strings.EqualFold(decision, "stop_mix_tuning") {
		return s.stopMixTuningInteraction(interaction, data, session)
	}
	if strings.EqualFold(decision, "rollback_last_mix_tick") || strings.EqualFold(decision, "rollback_last_mix_turn") {
		return s.rollbackLastMixTurnInteraction(ctx, interaction, data, session)
	}
	if strings.EqualFold(decision, "enter_discussion") {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if strings.EqualFold(decision, "submit_mixboard_intervention") {
		return deprecatedMixPlannerInteractionResponse(interaction, data, session)
	}
	if session.TargetRef.ID == "" || strings.EqualFold(session.TargetRef.Confidence, "low") {
		reqCard := s.mixSessionInteractionRequest(interaction.ConversationID, interaction.GoalID, interaction.RunID, data, session.TargetRef)
		reqCard.Body = "还没有明确的混音目标。请先选择轨道、片段或输入目标。"
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               reqCard.Body,
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{reqCard},
			GoalStatus:          string(agentruntime.StatusWaitingClarification),
		}
	}
	session.State = mixStatePreparing
	session.ConversationID = firstNonEmpty(session.ConversationID, cleanContextText(data["conversation_id"]), interaction.ConversationID)
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
	session.BlockingPoint = ""
	session.Preparation = mixPreparationRows(session.TargetRef)
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	prep := initialMixPlannerPrep(session, interaction.RequestContext)
	session.PlannerPrep = prep
	s.storeMixSession(session)
	applyMixPlannerPrepToWorkflowData(data, prep)
	data["mix_session"] = mixSessionMap(session)
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               mixPlanningDiscussionReply(session),
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixPlanningDiscussionInteraction(interaction, session, data)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         mixInteractionPlanningChat,
	}
}

func isDeprecatedMixPlannerDecision(decision string) bool {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "publish_mixboard",
		"confirm_mix_planner_plan",
		"advance_mix_planner",
		"submit_mix_planner_answers",
		"continue_mix_planning",
		"enter_discussion",
		"submit_mixboard_intervention",
		"start_mix_tuning",
		"auto_tune_mix":
		return true
	default:
		return false
	}
}

func isLegacyMixSessionApprovalDecision(decision string) bool {
	switch strings.ToLower(strings.TrimSpace(decision)) {
	case "approve", "allow", "confirm", "yes", "submit", "done":
		return true
	default:
		return false
	}
}

func deprecatedMixPlannerInteractionResponse(interaction PendingInteraction, data map[string]any, session MixSession) ChatResponse {
	if session.MixSessionID != "" {
		session.State = mixStateCancelled
		session.InteractionPhase = ""
		session.MixBoardVisibility = mixBoardVisibilityHidden
		session.BlockingPoint = "deprecated_mix_planner_action"
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		data["mix_session"] = mixSessionMap(session)
	}
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          "这个混音规划入口已停用。请直接在聊天里描述你要调整的声音目标；本次没有执行 DAW 操作。",
		Workflow:       mixSessionEntryWorkflow,
		WorkflowData:   data,
		MixSession:     mixSessionMap(session),
		GoalStatus:     string(agentruntime.StatusCompleted),
		CurrentStep:    "deprecated_mix_planner_action",
	}
}

func isCurrentConversationalMixAction(data map[string]any, payload map[string]any, session MixSession) bool {
	if boolValue(data["conversational_mix"]) || boolValue(payload["conversational_mix"]) ||
		boolValue(data["ask_vit_mix_action"]) || boolValue(payload["ask_vit_mix_action"]) {
		return true
	}
	for _, ctx := range []map[string]any{
		data,
		payload,
		mapValue(data["request_context"]),
		mapValue(payload["request_context"]),
		mapValue(data["mix_action_context"]),
		mapValue(payload["mix_action_context"]),
	} {
		source := strings.ToLower(strings.TrimSpace(firstNonEmpty(
			cleanContextText(ctx["mix_action_context"]),
			cleanContextText(ctx["source"]),
			cleanContextText(ctx["route"]),
		)))
		if source == "ask_vit" || source == "ask_vit_conversational" || source == "conversational_mix" {
			return true
		}
	}
	phase := strings.ToLower(strings.TrimSpace(mixInteractionPhase(session)))
	switch phase {
	case mixInteractionPlanningChat, mixInteractionPlanDrafting, mixInteractionControlSurfacePublished, mixInteractionWaitingPlannerReview, "planner_draft_review":
		return false
	}
	return false
}

func staleMixActionResponse(interaction PendingInteraction, data map[string]any, session MixSession) ChatResponse {
	session.BlockingPoint = "stale_mix_action_context"
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	data["mix_session"] = mixSessionMap(session)
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          "这张旧 MixBoard 动作已经失效。请直接在聊天里说出下一步混音目标；本次没有执行 DAW 操作。",
		Workflow:       mixSessionEntryWorkflow,
		WorkflowData:   data,
		MixSession:     mixSessionMap(session),
		GoalStatus:     string(agentruntime.StatusCompleted),
		CurrentStep:    "stale_mix_action_context",
	}
}

func legacyMixSessionWorkflowEnabled(ctx map[string]any) bool {
	if contextBool(ctx, "legacy_mix_session_workflow") || contextBool(ctx, "allow_legacy_mix_session") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("VIT_ENABLE_LEGACY_MIX_SESSION_WORKFLOW")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("VIT_ENABLE_LEGACY_MIX_SESSION_WORKFLOW")), "true")
}

func isMixSessionEntryInvoke(req harness.InvokeRequest) bool {
	tool := strings.ToLower(strings.TrimSpace(firstNonEmpty(req.Tool, cleanContextText(req.Command["cmd"]), cleanContextText(req.Command["command"]))))
	if tool == "mix.session_entry" || tool == "mix_session_entry" {
		return true
	}
	ctx := mergeContext(req.Context, req.Args)
	return contextBool(ctx, "mix_requested") || cleanContextText(ctx["mix_entry_source"]) != "" || normalizeMixMode(firstNonEmpty(cleanContextText(ctx["mix_mode"]), cleanContextText(ctx["mode"]))) != ""
}

func disabledMixSessionEntryInvokeResponse() harness.InvokeResponse {
	return harness.InvokeResponse{
		Status:      "ok",
		Tool:        "mix.session_entry",
		CommandName: mixSessionEntryWorkflow,
		Result: map[string]any{
			"reply":       "旧的 Auto Mix / Co-Mix 入口已经停用。请直接在 Ask Vit 聊天里描述要调整的声音目标；本次没有执行 DAW 操作。",
			"workflow":    mixSessionEntryWorkflow,
			"disabled":    true,
			"goal_status": string(agentruntime.StatusCompleted),
		},
	}
}

func (s *Server) reviseMixBoardInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any, decision string) ChatResponse {
	if len(payload) == 0 {
		payload = map[string]any{}
	}
	if session.MixSessionID == "" {
		session = mixSessionFromMap(mapValue(payload["mix_session"]))
	}
	if session.Mode == "" {
		session.Mode = firstNonEmpty(cleanContextText(data["mode"]), mixModeAuto)
	}
	goalText := cleanContextText(payload["goal_text"])
	if goalText != "" {
		session.GoalText = goalText
	}
	userNote := cleanContextText(payload["user_note"])
	if userNote == "" {
		userNote = cleanContextText(mapValue(payload["fields"])["user_note"])
	}
	if userNote != "" {
		data["user_note"] = userNote
		session.UserNote = userNote
	}
	requestContext := mergeContext(interaction.RequestContext, mapValue(data["request_context"]))
	requestContext = mergeContext(requestContext, mapValue(payload["request_context"]))
	observationOverrides := mixObservationOverridesFromRevision(payload, session.TargetRef, requestContext)
	if len(observationOverrides) > 0 {
		requestContext = mergeContext(requestContext, observationOverrides)
		data["mix_revision"] = observationOverrides
	}
	if userNote != "" {
		requestContext["user_note"] = userNote
	}
	session.RoundCount++
	if session.RoundCount <= 0 {
		session.RoundCount = 1
	}
	session.State = mixStateReadyObservation
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	nextInteraction := interaction
	nextInteraction.RequestContext = requestContext
	observationResult, observationErr := s.requestMixObservationRound(ctx, nextInteraction, session, session.RoundCount, observationOverrides)
	session.State = mixStateFromObservationResult(observationResult, observationErr)
	session.BlockingPoint = mixObservationBlockingPoint(observationResult, observationErr)
	session.Preparation = mixPreparationRowsForObservation(session.TargetRef, session.State, session.BlockingPoint)
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	data["request_context"] = requestContext
	if observationResult != nil {
		if userNote != "" {
			board := mapValue(observationResult["mixboard"])
			board["user_note"] = userNote
			board["current_judgement"] = "用户已修订听感，下一轮以该听感为准。"
			board["current_action"] = firstNonEmpty(cleanContextText(payload["action_note"]), "按新的混音对象 / 试听范围重新观察。")
			board["next_step"] = "根据修订后的 MixBoard 继续调控。"
			observationResult["mixboard"] = board
		}
		s.attachGoalControlSurface(ctx, nextInteraction, session, observationResult)
		updateMixBoardRuntimeState(observationResult, session, nil)
		data["goal_control_surface"] = mixGoalControlSurfaceFromObservation(observationResult)
		data["mix_observation"] = observationResult
	}
	reply := "MixBoard 已更新。"
	if strings.EqualFold(decision, "refresh_observation") {
		reply = "MixBoard 已按当前工程状态重新观察。"
	}
	if observationErr != nil {
		reply = mixObservationReply(session, observationErr)
	}
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(nextInteraction, session, observationResult)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func (s *Server) publishMixBoardInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	prep, observation, prepErr := s.ensureMixPlannerPrepReady(ctx, interaction, data, session, payload)
	applyMixPlannerPrepToWorkflowData(data, prep)
	session.PlannerPrep = prep
	if prepErr != nil || !boolValue(prep["ready_to_publish_mixboard"]) {
		applyMixPlannerPrepSessionState(&session, prep)
		session.BlockingPoint = firstNonEmpty(cleanContextText(prep["next_question"]), cleanContextText(prep["blocking_point"]), fmt.Sprint(prepErr))
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		if len(observation) > 0 {
			data["mix_observation"] = observation
		}
		reply := firstNonEmpty(session.BlockingPoint, "规划准备还没有完成，暂时不能发布 MixBoard。")
		if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
			return s.mixSessionStatusResponse(interaction, data, session, observation, reply)
		}
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               reply,
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixPlanningDiscussionInteraction(interaction, session, data)},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         mixInteractionPlanningChat,
		}
	}
	if len(observation) == 0 {
		var err error
		observation, err = s.requestInitialMixObservation(ctx, interaction, session)
		session.State = mixStateFromObservationResult(observation, err)
		session.BlockingPoint = mixObservationBlockingPoint(observation, err)
		if err != nil {
			return s.mixSessionStatusResponse(interaction, data, session, observation, err.Error())
		}
	}
	s.attachGoalControlSurface(ctx, interaction, session, observation)
	applyMixPlannerPrep(observation, prep)
	session = mixSessionAfterControlSurface(session, observation, false)
	session.PlannerPrep = prep
	packet := buildMixTickPacket(session, observation, mixTuningUserNote(payload, interaction.RequestContext, data, session))
	applyMixTickPacket(observation, packet)
	updateMixBoardRuntimeState(observation, session, nil)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	applyMixPlannerPrepToWorkflowData(data, prep)
	data["goal_control_surface"] = mixGoalControlSurfaceFromObservation(observation)
	data["mix_tick_packet"] = packet
	data["mix_observation"] = observation
	return s.mixSessionStatusResponse(interaction, data, session, observation, "MixBoard 已发布，请确认控制面后再执行。")
}

func (s *Server) advanceMixPlannerInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	prep, observation, prepErr := s.ensureMixPlannerPrepReady(ctx, interaction, data, session, payload)
	applyMixPlannerPrepSessionState(&session, prep)
	session.PlannerPrep = prep
	session.State = mixStatePreparing
	if prepErr != nil {
		session.BlockingPoint = prepErr.Error()
	} else {
		session.BlockingPoint = cleanContextText(prep["blocking_point"])
	}
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	applyMixPlannerPrepToWorkflowData(data, prep)
	if len(observation) > 0 {
		if boolValue(prep["ready_to_publish_mixboard"]) || cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
			data["mix_observation"] = observation
		} else {
			delete(data, "mix_observation")
		}
	}
	reply := mixPlannerPrepReply(prep, prepErr)
	if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
		return s.mixSessionStatusResponse(interaction, data, session, observation, reply)
	}
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixPlanningDiscussionInteraction(interaction, session, data)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         mixInteractionPlanningChat,
	}
}

func (s *Server) continueActiveMixPlannerChat(ctx context.Context, conversationID string, req ChatRequest) (ChatResponse, bool) {
	session, ok := s.activeMixPlannerSession(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	goalID, runID := goalIDsFromContext(req.Context)
	data := mixSessionWorkflowData(conversationID, goalID, runID, req.Context, session)
	if len(session.PlannerPrep) > 0 {
		applyMixPlannerPrepToWorkflowData(data, session.PlannerPrep)
	}
	payload := map[string]any{
		"message":         req.Message,
		"request_context": req.Context,
	}
	llmPatch, llmErr := s.mixPlannerLLMPatch(ctx, conversationID, goalID, req.Message, req.Context, session, mapValue(data["mix_planner_prep"]))
	if len(llmPatch) > 0 {
		payload["planner_llm_patch"] = llmPatch
		for key, value := range llmPatch {
			if value != nil {
				payload[key] = value
			}
		}
	}
	interaction := PendingInteraction{
		ID:             "interaction_" + randomID(),
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Type:           "mix_planning_discussion",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		RequestContext: req.Context,
		Payload:        data,
		Data:           data,
	}
	resp := s.advanceMixPlannerInteraction(ctx, interaction, data, session, payload)
	writeMixBoardDiag("mix_planner_chat", map[string]any{
		"conversation_id":  conversationID,
		"mix_session_id":   session.MixSessionID,
		"user_text":        req.Message,
		"planner_llm_used": len(llmPatch) > 0,
		"planner_llm_error": func() string {
			if llmErr != nil {
				return llmErr.Error()
			}
			return ""
		}(),
		"planner_stage":    cleanContextText(mapValue(resp.WorkflowData["mix_planner_prep"])["stage"]),
		"missing_inputs":   contextStringSlice(mapValue(resp.WorkflowData["mix_planner_prep"])["missing_inputs"]),
		"ready_to_publish": boolValue(mapValue(resp.WorkflowData["mix_planner_prep"])["ready_to_publish_mixboard"]),
	})
	return resp, true
}

func (s *Server) mixPlannerLLMPatch(ctx context.Context, conversationID, goalID, userText string, requestContext map[string]any, session MixSession, prep map[string]any) (map[string]any, error) {
	if s == nil || s.llm == nil || strings.TrimSpace(userText) == "" {
		return nil, nil
	}
	cfg, _, err := config.Load()
	if err != nil {
		return nil, err
	}
	if !cfg.Complete() {
		return nil, nil
	}
	state := map[string]any{
		"mix_session":            mixSessionMap(session),
		"mix_planner_prep":       prep,
		"mix_planning_workspace": mapValue(prep["mix_planning_workspace"]),
		"request_context":        requestContext,
		"hard_workflow_slots": []string{
			"mix_goal",
			"plugin_types",
			"local_plugin_candidates",
			"plugin_chain_order",
			"skill_profile_status",
			"macro_panel",
			"fast_tick_packet",
		},
	}
	stateJSON, _ := json.MarshalIndent(state, "", "  ")
	system := `你是 Vit Auto Mix 的思考模式规划器。你只负责理解当前自然对话并更新规划工作区字段，不执行 DAW 操作，不发布 MixBoard，不加载插件。

必须把用户消息分类到一个 intent，并只在用户确实表达混音目标/方向时更新 goal_detail。
权限句例如“允许你加载”“可以加载推荐插件”只能更新 allow_plugin_loads=true，不能写成混音目标。
插件推荐问题例如“有什么插件推荐”只能作为讨论/推荐意图，不能覆盖已有目标。
“由你决定/你来定”表示 planner_autonomy=agent_decides，并可视上下文允许规划器选择插件，但不代表发布 MixBoard。
只有用户在系统已经询问是否生成 MixBoard 后明确同意，才使用 confirm_publish_mixboard。

返回严格 JSON，不要 Markdown，不要解释。字段：
{
  "intent": "provide_goal|allow_plugin_loads|deny_plugin_loads|ask_plugin_recommendation|delegate_to_agent|confirm_publish_mixboard|reject_publish_mixboard|other",
  "goal_detail": null|string,
  "priority_focus": null|string,
  "tone_reference": null|string,
  "allow_plugin_loads": null|boolean,
  "planner_autonomy": null|"agent_decides",
  "planner_policy": null|"auto_assist"|"comix_director",
  "discussion_summary": null|string,
  "confidence": 0.0
}`
	messages := []llm.Message{{Role: "system", Content: system}}
	if history := s.conversationHistory(ctx, conversationID, 6, nil); len(history) > 0 {
		messages = append(messages, history...)
	}
	messages = append(messages,
		llm.Message{Role: "user", Content: "当前规划状态 JSON：\n" + string(stateJSON)},
		llm.Message{Role: "user", Content: "用户最新消息：\n" + userText},
	)
	resp, err := s.llm.CompleteRequest(ctx, cfg, llm.Request{
		Messages:   messages,
		PreferJSON: true,
		Timeout:    30 * time.Second,
		Metadata: llm.RequestMetadata{
			Source:            "mix_planner_chat",
			ConversationID:    conversationID,
			GoalID:            goalID,
			PromptFingerprint: "mix_planner_chat_intake.v1",
			PromptStats: map[string]any{
				"state_bytes": len(stateJSON),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	patch, err := parseFirstPluginUIReferenceJSONObject(resp.Text)
	if err != nil {
		return nil, err
	}
	return sanitizeMixPlannerLLMPatch(patch), nil
}

func sanitizeMixPlannerLLMPatch(patch map[string]any) map[string]any {
	out := map[string]any{}
	intent := cleanContextText(patch["intent"])
	switch intent {
	case "provide_goal", "allow_plugin_loads", "deny_plugin_loads", "ask_plugin_recommendation", "delegate_to_agent", "confirm_publish_mixboard", "reject_publish_mixboard", "other":
		out["intent"] = intent
	}
	for _, key := range []string{"goal_detail", "priority_focus", "tone_reference", "planner_autonomy", "planner_policy", "discussion_summary"} {
		if value := cleanContextText(patch[key]); value != "" && !strings.EqualFold(value, "null") {
			out[key] = value
		}
	}
	if value, ok := patch["allow_plugin_loads"]; ok {
		out["allow_plugin_loads"] = boolValue(value)
	}
	if confidence := floatNumber(patch["confidence"]); confidence > 0 {
		out["planner_llm_confidence"] = confidence
	}
	return out
}

func (s *Server) activeMixPlannerSession(conversationID string) (MixSession, bool) {
	if s == nil {
		return MixSession{}, false
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return MixSession{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var best MixSession
	for _, session := range s.mixSessions {
		if strings.TrimSpace(session.ConversationID) != conversationID {
			continue
		}
		if mixInteractionPhase(session) != mixInteractionPlanningChat {
			continue
		}
		if session.State == mixStateCancelled || session.State == mixStateTuningComplete {
			continue
		}
		if best.MixSessionID == "" || session.UpdatedAt > best.UpdatedAt {
			best = session
		}
	}
	return best, best.MixSessionID != ""
}

func (s *Server) confirmControlSurfaceInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	observation := mapValue(data["mix_observation"])
	if len(mixGoalControlSurfaceFromObservation(observation)) == 0 {
		s.attachGoalControlSurface(ctx, interaction, session, observation)
	}
	userNote := mixTuningUserNote(payload, interaction.RequestContext, data, session)
	if userNote != "" {
		session.UserNote = userNote
	}
	packet := buildMixTickPacket(session, observation, userNote)
	applyMixTickPacket(observation, packet)
	status := cleanContextText(packet["status"])
	if status == mixInteractionLearningRequired {
		session.InteractionPhase = mixInteractionLearningRequired
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "执行前需要先完成 Plugin Grabber 学习。"
	} else if status == "ready" {
		session.InteractionPhase = mixInteractionReadyForTick
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = ""
	} else {
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "Control surface is not executable yet: " + status
	}
	session.State = mixStateObservationReady
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	updateMixBoardRuntimeState(observation, session, nil)
	data["mix_session"] = mixSessionMap(session)
	data["mix_tick_packet"] = packet
	data["mix_observation"] = observation
	reply := "Control surface checked; no plugin was loaded from this legacy control-surface action."
	if session.BlockingPoint != "" {
		reply = session.BlockingPoint
	}
	return s.mixSessionStatusResponse(interaction, data, session, observation, reply)
}

func (s *Server) enterMixDiscussionInteraction(interaction PendingInteraction, data map[string]any, session MixSession) ChatResponse {
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
	session.State = mixStateTuningPaused
	session.ReviewStatus = mixReviewWaiting
	session.StopReason = mixStopReasonUserIntervention
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	observation := mapValue(data["mix_observation"])
	board := mapValue(observation["mixboard"])
	board["current_action"] = "MixBoard collapsed for planner discussion."
	board["next_step"] = "Continue the mix direction discussion in chat, then publish MixBoard again before execution."
	observation["mixboard"] = board
	updateMixBoardRuntimeState(observation, session, nil)
	data["mix_session"] = mixSessionMap(session)
	data["mix_observation"] = observation
	return s.mixSessionStatusResponse(interaction, data, session, observation, "MixBoard 已收起，已进入规划讨论。")
}

func (s *Server) submitMixBoardIntervention(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	userNote := mixTuningUserNote(payload, interaction.RequestContext, data, session)
	if userNote == "" {
		userNote = cleanContextText(payload["message"])
	}
	if userNote != "" {
		session.UserNote = userNote
		data["user_note"] = userNote
	}
	observation := mapValue(data["mix_observation"])
	packet := buildMixTickPacket(session, observation, userNote)
	applyMixTickPacket(observation, packet)
	session = mixSessionAfterControlSurface(session, observation, cleanContextText(packet["status"]) == "ready")
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	updateMixBoardRuntimeState(observation, session, nil)
	data["mix_session"] = mixSessionMap(session)
	data["mix_tick_packet"] = packet
	data["mix_observation"] = observation
	return s.mixSessionStatusResponse(interaction, data, session, observation, "MixBoard intervention captured for the next single tick.")
}

func (s *Server) loadConfirmedControlSurfacePlugins(ctx context.Context, interaction PendingInteraction, session MixSession, observation map[string]any) ([]map[string]any, error) {
	if s == nil || s.harness == nil {
		return nil, nil
	}
	surface := mixGoalControlSurfaceFromObservation(observation)
	if len(surface) == 0 {
		return nil, nil
	}
	results := []map[string]any{}
	attemptedLoadKeys := map[string]bool{}
	for _, row := range mapRowsValue(surface["selected_chain"]) {
		if cleanContextText(row["instance_status"]) != mixcontrolsurface.InstanceNeedsLoad {
			continue
		}
		profileStatus := cleanContextText(row["profile_status"])
		if profileStatus != mixcontrolsurface.ProfileReady && profileStatus != mixcontrolsurface.ProfileNotRequired {
			continue
		}
		plugin := mapValue(row["selected_plugin"])
		pluginPath := firstNonEmpty(cleanContextText(plugin["plugin_path"]), cleanContextText(plugin["path"]), cleanContextText(plugin["file_path"]))
		trackID := firstNonEmpty(session.TargetRef.ID, cleanContextText(mapValue(row["target"])["id"]))
		loadKey := confirmedControlSurfacePluginLoadKey(trackID, plugin)
		result := map[string]any{
			"role":        cleanContextText(row["role"]),
			"type":        cleanContextText(row["type"]),
			"plugin_name": firstNonEmpty(cleanContextText(plugin["name"]), cleanContextText(plugin["descriptive_name"]), pluginPath),
			"plugin_path": pluginPath,
			"track_id":    trackID,
		}
		if attemptedLoadKeys[loadKey] {
			result["status"] = "skipped_duplicate_plugin"
			result["reason"] = "同一个插件已在本次控制面确认中加载，复用该实例承载多个处理角色。"
			results = append(results, result)
			continue
		}
		attemptedLoadKeys[loadKey] = true
		if pluginPath == "" || trackID == "" {
			result["status"] = "blocked"
			result["error"] = "selected plugin has no loadable plugin_path or track_id"
			results = append(results, result)
			return results, fmt.Errorf("%s", result["error"])
		}
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool: "rack.add_node",
			Args: map[string]any{
				"track_id":    trackID,
				"plugin_path": pluginPath,
			},
			Context:   interaction.RequestContext,
			Source:    "mixboard_control_surface_confirm",
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
		result["status"] = resp.Status
		result["agent_action_id"] = resp.AgentActionID
		if resp.Error != "" || err != nil {
			result["error"] = firstNonEmpty(resp.Error, fmt.Sprint(err))
			results = append(results, result)
			return results, fmt.Errorf("%s", result["error"])
		}
		if len(resp.Result) > 0 {
			result["result"] = resp.Result
		}
		results = append(results, result)
	}
	return results, nil
}

func confirmedControlSurfacePluginLoadKey(trackID string, plugin map[string]any) string {
	pluginPath := firstNonEmpty(cleanContextText(plugin["plugin_path"]), cleanContextText(plugin["path"]), cleanContextText(plugin["file_path"]))
	pluginID := firstNonEmpty(cleanContextText(plugin["profile_id"]), cleanContextText(plugin["id"]), cleanContextText(plugin["identifier"]), pluginPath, cleanContextText(plugin["name"]))
	return strings.ToLower(strings.Join([]string{strings.TrimSpace(trackID), strings.TrimSpace(pluginID)}, "|"))
}

func (s *Server) runSingleMixTickInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	if session.MixSessionID == "" {
		session = mixSessionFromMap(mapValue(payload["mix_session"]))
	}
	if session.MixSessionID == "" {
		session = mixSessionFromMap(mapValue(data["mix_session"]))
	}
	if session.Mode == "" {
		session.Mode = firstNonEmpty(cleanContextText(data["mode"]), mixModeAuto)
	}
	if session.TargetRef.ID == "" {
		session.State = mixStateFailed
		session.BlockingPoint = "mix target is not resolved"
		return s.mixSessionStatusResponse(interaction, data, session, mapValue(data["mix_observation"]), session.BlockingPoint)
	}
	if !strings.EqualFold(session.TargetRef.Kind, "track") {
		session.State = mixStateFailed
		session.BlockingPoint = "自动调参 v0 只支持当前轨道音量。"
		return s.mixSessionStatusResponse(interaction, data, session, mapValue(data["mix_observation"]), session.BlockingPoint)
	}
	if s == nil || s.harness == nil {
		session.State = mixStateFailed
		session.BlockingPoint = "harness unavailable for single tick execution"
		return s.mixSessionStatusResponse(interaction, data, session, mapValue(data["mix_observation"]), session.BlockingPoint)
	}
	observationSeed := mapValue(data["mix_observation"])
	userNote := mixTuningUserNote(payload, interaction.RequestContext, data, session)
	if userNote != "" {
		session.UserNote = userNote
	}
	packet := mixTickPacketFromObservation(observationSeed)
	if len(packet) == 0 || userNote != "" {
		packet = buildMixTickPacket(session, observationSeed, userNote)
		applyMixTickPacket(observationSeed, packet)
	}
	status := cleanContextText(packet["status"])
	if status == mixInteractionLearningRequired || status == "blocked" || status == "no_allowed_controls" {
		session.InteractionPhase = mixInteractionLearningRequired
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = firstNonEmpty(cleanContextText(packet["status"]), "single tick is blocked")
		if request := mapValue(packet["plugin_learning_request"]); len(request) > 0 {
			session.BlockingPoint = "需要先完成 Plugin Grabber 学习：" + firstNonEmpty(cleanContextText(request["plugin_name"]), cleanContextText(request["type"]))
		}
		return s.mixSessionStatusResponse(interaction, data, session, observationSeed, session.BlockingPoint)
	}
	if status == "needs_confirmation" && mixInteractionPhase(session) != mixInteractionReadyForTick {
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "Control surface needs confirmation before execution."
		return s.mixSessionStatusResponse(interaction, data, session, observationSeed, session.BlockingPoint)
	}
	if mixInteractionPhase(session) != mixInteractionReadyForTick && !boolValue(payload["confirmed"]) && !boolValue(payload["control_surface_confirmed"]) {
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "请先确认控制面，再执行 single tick。"
		return s.mixSessionStatusResponse(interaction, data, session, observationSeed, session.BlockingPoint)
	}
	selected, selectErr := selectMixTickControl(packet, payload)
	if selectErr != nil {
		session.BlockingPoint = selectErr.Error()
		return s.mixSessionStatusResponse(interaction, data, session, observationSeed, session.BlockingPoint)
	}
	session.State = mixStateTuningRunning
	session.InteractionPhase = mixInteractionFastTickRunning
	session.MixBoardVisibility = mixBoardVisibilityExecution
	session.ExecutorType = mixSingleTickExecutorType
	session.ExecutorVersion = mixSingleTickExecutorVersion
	session.ReviewStatus = mixReviewWaiting
	session.StopReason = ""
	session.BlockingPoint = ""
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	requestContext := mergeContext(interaction.RequestContext, mapValue(data["request_context"]))
	requestContext = mergeContext(requestContext, mapValue(payload["request_context"]))
	session.FastModelProfile = mixFastModelProfile(session, mergeContext(requestContext, payload))
	stepDB := mixTuningStepDB(session.GoalText, payload)
	turn, executedReplies, applyErr := s.applySelectedMixTickControl(ctx, interaction, requestContext, session, packet, selected, stepDB, userNote)
	if applyErr != nil {
		session.State = mixStateFailed
		session.ReviewStatus = mixReviewFailed
		session.BlockingPoint = applyErr.Error()
		turn["error"] = applyErr.Error()
	} else {
		session.JournalRefs = append(session.JournalRefs, cleanContextText(turn["agent_action_id"]))
		session.RoundCount++
	}
	var observationResult map[string]any
	var observationErr error
	if session.State != mixStateFailed {
		observationResult, observationErr = s.requestMixObservationRound(ctx, interaction, session, session.RoundCount, map[string]any{
			"last_mix_action": turn,
		})
		if observationErr != nil {
			session.State = mixStateObservationUnavailable
			session.StopReason = mixStopReasonObservationUnavailable
			session.BlockingPoint = observationErr.Error()
		} else {
			turn["observation_status"] = firstNonEmpty(cleanContextText(observationResult["status"]), "unavailable")
			turn["delta_status"] = mixBeforeAfterDeltaStatus(observationResult)
			turn["decision"] = mixTuneDecisionFromObservation(observationResult, stepDB)
			turn["review_status"] = mixTuneReviewStatus(cleanContextText(turn["decision"]), cleanContextText(turn["delta_status"]))
			session.State = mixStateWaitingReview
			session.InteractionPhase = mixInteractionWaitingPlannerReview
			session.MixBoardVisibility = mixBoardVisibilityExecution
			session.ReviewStatus = mixTuneReviewStatus(cleanContextText(turn["decision"]), cleanContextText(turn["delta_status"]))
		}
	}
	if observationResult == nil {
		observationResult = observationSeed
	}
	turns := append(mixTuneTurnsFromObservation(observationResult), turn)
	packet["latest_tick"] = turn
	packet["recent_ticks"] = recentMixTuneTurns(turns, 3)
	applyMixTickPacket(observationResult, packet)
	board := mapValue(observationResult["mixboard"])
	board["current_action"] = mixTickCurrentAction(turn)
	board["current_judgement"] = firstNonEmpty(cleanContextText(turn["reason"]), mixTuneJudgement([]map[string]any{turn}))
	board["next_step"] = "Review this single tick, then continue, rollback, or enter discussion."
	board["latest_mix_tick"] = turn
	board["auto_tune_turns"] = turns
	board["single_tick_turns"] = turns
	board["review_status"] = session.ReviewStatus
	board["rollback_available"] = len(session.JournalRefs) > 0
	observationResult["mixboard"] = board
	writeMixTuneActionRecord(observationResult, turn)
	updateMixBoardRuntimeState(observationResult, session, turns)
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	data["request_context"] = requestContext
	data["mix_tick_packet"] = packet
	data["mix_observation"] = observationResult
	reply := "Single mix tick executed."
	if observationErr != nil {
		reply = "Single mix tick executed, but readback is unavailable: " + observationErr.Error()
	}
	if session.BlockingPoint != "" {
		reply = session.BlockingPoint
	}
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observationResult)},
		ExecutedKernelReply: executedReplies,
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func (s *Server) mixSessionStatusResponse(interaction PendingInteraction, data map[string]any, session MixSession, observation map[string]any, reply string) ChatResponse {
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	if s != nil {
		s.storeMixSession(session)
	}
	updateMixBoardRuntimeState(observation, session, nil)
	data["mix_session"] = mixSessionMap(session)
	if len(observation) > 0 {
		data["mix_observation"] = observation
	}
	if reply == "" {
		reply = "Mix session state updated."
	}
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func initialMixPlannerPrep(session MixSession, requestContext map[string]any) map[string]any {
	now := time.Now().Format(time.RFC3339Nano)
	prep := map[string]any{
		"schema_version":            mixPlannerPrepSchemaVersion,
		"mix_session_id":            session.MixSessionID,
		"stage":                     "planner_intake",
		"planner_policy":            mixPlannerPolicy(session),
		"model_strategy":            mixPlannerModelStrategy(),
		"goal_summary":              strings.TrimSpace(session.GoalText),
		"target_ref":                mixTargetMap(session.TargetRef),
		"mixboard_visibility":       mixBoardVisibilityCollapsed,
		"ready_to_publish_mixboard": false,
		"created_at":                now,
		"updated_at":                now,
	}
	if detail := firstNonEmpty(cleanContextText(requestContext["mix_goal_detail"]), cleanContextText(requestContext["goal_detail"]), cleanContextText(requestContext["user_note"])); detail != "" {
		prep["goal_detail"] = detail
	}
	updateMixPlannerPrepDerived(prep, session)
	syncMixPlanningWorkspace(prep, session)
	return prep
}

func mixPlannerPrepFromData(data map[string]any, session MixSession, requestContext map[string]any) map[string]any {
	prep := mapValue(data["mix_planner_prep"])
	if len(prep) == 0 {
		prep = initialMixPlannerPrep(session, requestContext)
	}
	if cleanContextText(prep["schema_version"]) == "" {
		prep["schema_version"] = mixPlannerPrepSchemaVersion
	}
	if cleanContextText(prep["mix_session_id"]) == "" {
		prep["mix_session_id"] = session.MixSessionID
	}
	if len(mapValue(prep["target_ref"])) == 0 {
		prep["target_ref"] = mixTargetMap(session.TargetRef)
	}
	if cleanContextText(prep["goal_summary"]) == "" {
		prep["goal_summary"] = strings.TrimSpace(session.GoalText)
	}
	syncMixPlanningWorkspace(prep, session)
	return prep
}

func absorbMixPlannerAnswers(prep map[string]any, session *MixSession, payload map[string]any) {
	fields := mapValue(payload["fields"])
	message := cleanContextText(payload["message"])
	lowerMessage := strings.ToLower(message)
	priorWorkspaceMissing := mixPlannerWorkspaceMissingSlots(prep)
	payloadIntent := cleanContextText(payload["intent"])
	intent := firstNonEmpty(payloadIntent, mixPlannerMessageIntent(message))
	if payloadIntent == "" && mixPlannerGenericAffirmative(message) {
		if boolValue(prep["publish_mixboard_prompted"]) && mixPlannerWorkspaceComplete(prep) {
			intent = "confirm_publish_mixboard"
		} else if containsString(mixPlannerBaseMissingInputs(prep), "allow_plugin_loads") {
			intent = "allow_plugin_loads"
		}
	}
	if intent != "" {
		prep["last_user_intent"] = intent
	}
	if confidence := floatNumber(payload["planner_llm_confidence"]); confidence > 0 {
		prep["planner_llm_confidence"] = confidence
	}
	if summary := cleanContextText(payload["discussion_summary"]); summary != "" {
		prep["last_discussion_summary"] = summary
	}
	if boolValue(prep["planner_revision_requested"]) && mixPlannerRevisionCanResume(intent, message) {
		prep["planner_revision_requested"] = false
		prep["publish_mixboard_prompted"] = false
		prep["publish_mixboard_confirmed"] = false
	}
	pick := func(keys ...string) string {
		for _, key := range keys {
			if value := cleanContextText(payload[key]); value != "" {
				return value
			}
			if value := cleanContextText(fields[key]); value != "" {
				return value
			}
		}
		return ""
	}
	if goal := pick("goal_detail", "mix_goal_detail", "mix_direction", "direction"); goal != "" {
		mixPlannerSetGoalDetail(prep, session, goal)
	} else if note := pick("user_note"); note != "" && !mixPlannerMetaMessage(note) {
		mixPlannerSetGoalDetail(prep, session, note)
	} else if message != "" && mixPlannerMessageCanUpdateGoal(intent, message) {
		mixPlannerSetGoalDetail(prep, session, message)
	}
	if priority := pick("priority_focus", "mix_priority", "focus"); priority != "" {
		prep["priority_focus"] = priority
	}
	if tone := pick("tone_reference", "reference", "style_reference"); tone != "" {
		prep["tone_reference"] = tone
	}
	if allowRaw, ok := payload["allow_plugin_loads"]; ok {
		prep["allow_plugin_loads"] = boolValue(allowRaw)
	} else if allowRaw, ok := fields["allow_plugin_loads"]; ok {
		prep["allow_plugin_loads"] = boolValue(allowRaw)
	} else if intent == "allow_plugin_loads" || mixPlannerUserDelegates(message) || mixTextContainsAny(lowerMessage, "可以加载", "能加载", "同意加载") {
		prep["allow_plugin_loads"] = true
	} else if intent == "deny_plugin_loads" || mixTextContainsAny(lowerMessage, "不加载", "不要加载", "只用已有") {
		prep["allow_plugin_loads"] = false
	}
	if mode := pick("planner_policy", "mix_policy"); mode != "" {
		session.PlannerPolicy = mode
		prep["planner_policy"] = mode
	}
	if intent == "delegate_to_agent" || mixPlannerUserDelegates(message) {
		prep["planner_autonomy"] = "agent_decides"
		prep["planner_auto_accept_drafts"] = true
	}
	if confirmedRaw, ok := payload["publish_mixboard_confirmed"]; ok {
		prep["publish_mixboard_confirmed"] = boolValue(confirmedRaw)
	} else if confirmedRaw, ok := payload["planner_plan_confirmed"]; ok {
		prep["publish_mixboard_confirmed"] = boolValue(confirmedRaw)
	} else if confirmedRaw, ok := fields["planner_plan_confirmed"]; ok {
		prep["publish_mixboard_confirmed"] = boolValue(confirmedRaw)
	} else if intent == "confirm_publish_mixboard" && boolValue(prep["publish_mixboard_prompted"]) {
		prep["publish_mixboard_confirmed"] = true
	} else if intent == "reject_publish_mixboard" {
		prep["publish_mixboard_confirmed"] = false
		prep["publish_mixboard_prompted"] = false
		prep["planner_revision_requested"] = true
		prep["planner_revision_note"] = message
	}
	if mixPlannerShouldAcceptCurrentDraft(intent, message, prep) {
		mixPlannerAcceptFirstMissingDraftSlot(prep, priorWorkspaceMissing)
	}
	prep["updated_at"] = time.Now().Format(time.RFC3339Nano)
}

func mixPlannerSetGoalDetail(prep map[string]any, session *MixSession, goal string) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	if cleanContextText(prep["goal_detail"]) != goal {
		mixPlannerClearAcceptedSlots(prep, "plugin_types", "local_plugin_candidates", "plugin_chain_order", "skill_profile_status", "macro_panel", "fast_tick_packet")
		prep["publish_mixboard_prompted"] = false
		prep["publish_mixboard_confirmed"] = false
	}
	prep["goal_detail"] = goal
	prep["accepted_planning_slots"] = uniqueStrings(append(contextStringSlice(prep["accepted_planning_slots"]), "mix_goal"))
	if session != nil {
		session.UserNote = goal
	}
}

func mixPlannerShouldAcceptCurrentDraft(intent, message string, prep map[string]any) bool {
	if boolValue(prep["publish_mixboard_prompted"]) {
		return false
	}
	if len(mixPlannerBaseMissingInputs(prep)) > 0 {
		return false
	}
	if intent == "delegate_to_agent" {
		prep["planner_auto_accept_drafts"] = true
		return true
	}
	if intent == "confirm_publish_mixboard" || mixPlannerGenericAffirmative(message) {
		return true
	}
	return false
}

func mixPlannerMessageIntent(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	if mixTextContainsAny(text, "有什么插件推荐", "插件推荐", "推荐什么插件", "用什么插件", "哪些插件", "候选插件") {
		return "ask_plugin_recommendation"
	}
	if mixTextContainsAny(text, "不行", "还不行", "先别", "不要生成", "别生成", "不发布", "别发布") {
		return "reject_publish_mixboard"
	}
	if mixTextContainsAny(text, "允许加载", "允许你加载", "允许你加", "可以加载", "你可以加载", "能加载", "同意加载") || text == "允许" {
		return "allow_plugin_loads"
	}
	if mixTextContainsAny(text, "不加载", "不要加载", "只用已有") {
		return "deny_plugin_loads"
	}
	if mixPlannerUserDelegates(text) {
		return "delegate_to_agent"
	}
	if mixPlannerUserConfirmsPlan(text) {
		return "confirm_publish_mixboard"
	}
	return "provide_goal"
}

func mixPlannerGenericAffirmative(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return text == "可以" || text == "确认" || text == "好" || text == "ok" || text == "yes"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mixPlannerMessageCanUpdateGoal(intent, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	switch intent {
	case "provide_goal":
		return true
	default:
		return false
	}
}

func mixPlannerRevisionCanResume(intent, text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	switch intent {
	case "provide_goal", "ask_plugin_recommendation", "delegate_to_agent", "allow_plugin_loads", "deny_plugin_loads":
		return true
	default:
		return false
	}
}

func mixPlannerUserDelegates(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return mixTextContainsAny(text, "由你决定", "你决定", "你来定", "你看着办", "交给你", "按你的判断", "agent decides", "you decide")
}

func mixPlannerUserConfirmsPlan(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	return mixTextContainsAny(text, "确认规划", "按这个", "按这个来", "可以发布", "发布mixboard", "发布 mixboard", "开始生成mixboard", "开始生成 mixboard", "确认控制面", "ok", "yes")
}

func mixPlannerMetaMessage(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	if mixPlannerUserDelegates(text) || mixPlannerUserConfirmsPlan(text) {
		return true
	}
	switch mixPlannerMessageIntent(text) {
	case "ask_plugin_recommendation", "allow_plugin_loads", "deny_plugin_loads", "reject_publish_mixboard", "confirm_publish_mixboard", "delegate_to_agent":
		return true
	default:
		return false
	}
}

func mixTextContainsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func updateMixPlannerPrepDerived(prep map[string]any, session MixSession) {
	goalParts := []string{}
	for _, part := range []string{
		cleanContextText(prep["goal_summary"]),
		cleanContextText(prep["goal_detail"]),
		cleanContextText(prep["priority_focus"]),
	} {
		if strings.TrimSpace(part) != "" {
			goalParts = append(goalParts, part)
		}
	}
	goalText := strings.TrimSpace(strings.Join(goalParts, " "))
	if goalText == "" {
		goalText = session.GoalText
	}
	roleTypes := mixcontrolsurface.RequiredRoleTypes(goalText)
	if len(roleTypes) == 0 {
		roleTypes = []string{"eq", "dynamics"}
	}
	roleRows := []map[string]any{}
	pluginTypes := []map[string]any{}
	for i, roleType := range roleTypes {
		roleRows = append(roleRows, map[string]any{
			"slot":     i + 1,
			"type":     roleType,
			"required": i < 2,
			"source":   "planner_goal_analysis",
		})
		pluginTypes = append(pluginTypes, map[string]any{
			"slot":          i + 1,
			"required_type": roleType,
			"reason":        "Needed for the current mix goal.",
		})
	}
	prep["required_roles"] = roleRows
	prep["plugin_type_plan"] = pluginTypes
	prep["plugin_chain_order"] = mixPlannerChainOrder(roleTypes)
	if boolValue(prep["planner_auto_accept_drafts"]) {
		mixPlannerAcceptAllDraftSlots(prep)
	}
	baseMissing := mixPlannerBaseMissingInputs(prep)
	if len(baseMissing) > 0 {
		prep["missing_inputs"] = baseMissing
		prep["open_questions"] = mixPlannerQuestions(baseMissing)
		prep["next_question"] = mixPlannerNextQuestion(baseMissing)
		prep["stage"] = "planner_intake"
		prep["ready_to_publish_mixboard"] = false
		return
	}
	if cleanContextText(prep["blocking_point"]) != "" || cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
		prep["missing_inputs"] = []string{"system_preflight_blocked"}
		prep["open_questions"] = []map[string]any{}
		prep["next_question"] = firstNonEmpty(cleanContextText(prep["blocking_point"]), "系统预检仍有阻断，不能生成 MixBoard。")
		prep["ready_to_publish_mixboard"] = false
		return
	}
	workspaceMissing := mixPlannerWorkspaceMissingSlots(prep)
	if len(workspaceMissing) > 0 {
		prep["missing_inputs"] = workspaceMissing
		prep["open_questions"] = mixPlannerQuestions(workspaceMissing)
		prep["next_question"] = mixPlannerNextQuestion(workspaceMissing)
		prep["stage"] = "plan_drafting"
		prep["ready_to_publish_mixboard"] = false
		return
	}
	if boolValue(prep["planner_revision_requested"]) {
		prep["missing_inputs"] = []string{"planner_revision_note"}
		prep["open_questions"] = mixPlannerQuestions([]string{"planner_revision_note"})
		prep["next_question"] = mixPlannerNextQuestion([]string{"planner_revision_note"})
		prep["stage"] = "plan_drafting"
		prep["ready_to_publish_mixboard"] = false
		return
	}
	if !boolValue(prep["publish_mixboard_prompted"]) {
		prep["publish_mixboard_prompted"] = true
		prep["missing_inputs"] = []string{"publish_mixboard_confirmed"}
		prep["open_questions"] = mixPlannerQuestions([]string{"publish_mixboard_confirmed"})
		prep["next_question"] = mixPlannerNextQuestion([]string{"publish_mixboard_confirmed"})
		prep["stage"] = "planner_draft_review"
		prep["ready_to_publish_mixboard"] = false
		return
	}
	if !boolValue(prep["publish_mixboard_confirmed"]) {
		prep["missing_inputs"] = []string{"publish_mixboard_confirmed"}
		prep["open_questions"] = mixPlannerQuestions([]string{"publish_mixboard_confirmed"})
		prep["next_question"] = mixPlannerNextQuestion([]string{"publish_mixboard_confirmed"})
		prep["stage"] = "planner_draft_review"
		prep["ready_to_publish_mixboard"] = false
		return
	}
	prep["missing_inputs"] = []string{}
	prep["open_questions"] = []map[string]any{}
	prep["next_question"] = ""
	prep["stage"] = "planner_preflight_complete"
	prep["ready_to_publish_mixboard"] = true
}

func syncMixPlanningWorkspace(prep map[string]any, session MixSession) map[string]any {
	if len(prep) == 0 {
		return nil
	}
	workspace := mixPlanningWorkspaceFromPrep(prep, session)
	prep["mix_planning_workspace"] = workspace
	return workspace
}

func mixPlanningWorkspaceFromPrep(prep map[string]any, session MixSession) map[string]any {
	missing := mixPlannerMissingInputs(prep)
	if len(missing) == 0 {
		missing = contextStringSlice(prep["missing_inputs"])
	}
	semanticReady := len(mixPlannerBaseMissingInputs(prep)) == 0
	systemReady := boolValue(prep["ready_to_publish_mixboard"])
	systemBlockers := []string{}
	if blocker := cleanContextText(prep["blocking_point"]); blocker != "" {
		systemBlockers = append(systemBlockers, blocker)
	}
	if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
		systemReady = false
		if len(systemBlockers) == 0 {
			systemBlockers = append(systemBlockers, "plugin learning required")
		}
	}
	workspace := map[string]any{
		"schema_version":      mixPlanningWorkspaceSchemaVersion,
		"mix_session_id":      session.MixSessionID,
		"conversation_id":     session.ConversationID,
		"mode":                session.Mode,
		"planner_policy":      mixPlannerPolicy(session),
		"interaction_phase":   mixInteractionPlanningChat,
		"mixboard_visibility": mixBoardVisibilityCollapsed,
		"stage":               cleanContextText(prep["stage"]),
		"target_ref":          mixTargetMap(session.TargetRef),
		"goal": map[string]any{
			"summary":        cleanContextText(prep["goal_summary"]),
			"detail":         cleanContextText(prep["goal_detail"]),
			"priority_focus": cleanContextText(prep["priority_focus"]),
			"tone_reference": cleanContextText(prep["tone_reference"]),
			"autonomy":       cleanContextText(prep["planner_autonomy"]),
		},
		"workflow_slots": []map[string]any{
			mixWorkspaceSlot("mix_goal", "混音目标", mixPlannerWorkspaceSlotComplete(prep, "mix_goal"), cleanContextText(prep["goal_detail"])),
			mixWorkspaceSlot("plugin_types", "所需插件类型", mixPlannerWorkspaceSlotComplete(prep, "plugin_types"), strings.Join(mixPluginTypeNames(mapRowsValue(prep["plugin_type_plan"])), " / ")),
			mixWorkspaceSlot("local_plugin_candidates", "本地插件候选", mixPlannerWorkspaceSlotComplete(prep, "local_plugin_candidates"), mixCandidateSummary(mapRowsValue(prep["plugin_candidates"]))),
			mixWorkspaceSlot("plugin_chain_order", "插件链顺序", mixPlannerWorkspaceSlotComplete(prep, "plugin_chain_order"), mixChainSummary(mapRowsValue(prep["plugin_chain_order"]))),
			mixWorkspaceSlot("skill_profile_status", "Skill/Profile 检查", mixPlannerWorkspaceSlotComplete(prep, "skill_profile_status"), mixSkillProfileSummary(prep)),
			mixWorkspaceSlot("macro_panel", "宏面板草案", mixPlannerWorkspaceSlotComplete(prep, "macro_panel"), mixMacroPanelSummary(mapValue(prep["macro_panel_draft"]))),
			mixWorkspaceSlot("fast_tick_packet", "快速模式上下文包", mixPlannerWorkspaceSlotComplete(prep, "fast_tick_packet"), mixFastPacketSummary(prep)),
		},
		"required_plugin_types":   mapRowsValue(prep["plugin_type_plan"]),
		"plugin_chain_order":      mapRowsValue(prep["plugin_chain_order"]),
		"local_plugin_candidates": mapRowsValue(prep["plugin_candidates"]),
		"selected_chain_draft":    mapRowsValue(prep["selected_chain_draft"]),
		"skill_profile_status":    mapValue(prep["skill_profile_status"]),
		"macro_panel_draft":       mapValue(prep["macro_panel_draft"]),
		"fast_tick_context": map[string]any{
			"status":            mixFastPacketSummary(prep),
			"proposed_controls": mapRowsValue(prep["proposed_controls"]),
			"model_strategy":    mixFastModelStrategyPreview(session),
		},
		"planner_verdict": map[string]any{
			"semantic_ready":  semanticReady,
			"system_ready":    systemReady,
			"publishable":     boolValue(prep["ready_to_publish_mixboard"]),
			"missing":         missing,
			"system_blockers": systemBlockers,
			"next_question":   cleanContextText(prep["next_question"]),
			"source":          "workflow_conditions",
		},
		"open_questions": mapRowsValue(prep["open_questions"]),
		"next_question":  cleanContextText(prep["next_question"]),
		"updated_at":     time.Now().Format(time.RFC3339Nano),
	}
	if createdAt := cleanContextText(prep["created_at"]); createdAt != "" {
		workspace["created_at"] = createdAt
	}
	return workspace
}

func mixPlannerWorkspaceMissingSlots(prep map[string]any) []string {
	missing := []string{}
	for _, slotID := range mixPlannerWorkspaceSlotIDs() {
		if !mixPlannerWorkspaceSlotComplete(prep, slotID) {
			missing = append(missing, "workspace_"+slotID)
		}
	}
	return missing
}

func mixPlannerWorkspaceComplete(prep map[string]any) bool {
	return len(mixPlannerWorkspaceMissingSlots(prep)) == 0
}

func mixPlannerWorkspaceSlotComplete(prep map[string]any, slotID string) bool {
	if !mixPlannerWorkspaceSlotHasDraftValue(prep, slotID) {
		return false
	}
	if slotID == "mix_goal" {
		return true
	}
	return mixPlannerSlotAccepted(prep, slotID)
}

func mixPlannerWorkspaceSlotHasDraftValue(prep map[string]any, slotID string) bool {
	switch slotID {
	case "mix_goal":
		return cleanContextText(prep["goal_detail"]) != ""
	case "plugin_types":
		return len(mapRowsValue(prep["plugin_type_plan"])) > 0
	case "local_plugin_candidates":
		return len(mapRowsValue(prep["plugin_candidates"])) > 0
	case "plugin_chain_order":
		return len(mapRowsValue(prep["plugin_chain_order"])) > 0
	case "skill_profile_status":
		return len(mapValue(prep["skill_profile_status"])) > 0
	case "macro_panel":
		return len(mapRowsValue(mapValue(prep["macro_panel_draft"])["controls"])) > 0
	case "fast_tick_packet":
		return len(mapRowsValue(prep["proposed_controls"])) > 0
	default:
		return false
	}
}

func mixPlannerWorkspaceSlotIDs() []string {
	return []string{"mix_goal", "plugin_types", "local_plugin_candidates", "plugin_chain_order", "skill_profile_status", "macro_panel", "fast_tick_packet"}
}

func mixPlannerSlotAccepted(prep map[string]any, slotID string) bool {
	return containsString(contextStringSlice(prep["accepted_planning_slots"]), slotID)
}

func mixPlannerAcceptSlot(prep map[string]any, slotID string) {
	if slotID == "" {
		return
	}
	prep["accepted_planning_slots"] = uniqueStrings(append(contextStringSlice(prep["accepted_planning_slots"]), slotID))
}

func mixPlannerClearAcceptedSlots(prep map[string]any, slotIDs ...string) {
	if len(slotIDs) == 0 {
		return
	}
	remove := map[string]bool{}
	for _, slotID := range slotIDs {
		remove[slotID] = true
	}
	kept := []string{}
	for _, slotID := range contextStringSlice(prep["accepted_planning_slots"]) {
		if !remove[slotID] {
			kept = append(kept, slotID)
		}
	}
	prep["accepted_planning_slots"] = kept
}

func mixPlannerAcceptFirstMissingDraftSlot(prep map[string]any, missing []string) {
	if len(missing) == 0 {
		missing = mixPlannerWorkspaceMissingSlots(prep)
	}
	for _, missingID := range missing {
		slotID := strings.TrimPrefix(missingID, "workspace_")
		if slotID == "mix_goal" || !containsString(mixPlannerWorkspaceSlotIDs(), slotID) {
			continue
		}
		if mixPlannerWorkspaceSlotHasDraftValue(prep, slotID) {
			mixPlannerAcceptSlot(prep, slotID)
			return
		}
	}
}

func mixPlannerAcceptAllDraftSlots(prep map[string]any) {
	for _, slotID := range mixPlannerWorkspaceSlotIDs() {
		if mixPlannerWorkspaceSlotHasDraftValue(prep, slotID) {
			mixPlannerAcceptSlot(prep, slotID)
		}
	}
}

func mixWorkspaceSlot(id, label string, complete bool, summary string) map[string]any {
	status := "missing"
	if complete {
		status = "complete"
	} else if strings.TrimSpace(summary) != "" {
		status = "draft"
	}
	return map[string]any{
		"id":       id,
		"label":    label,
		"status":   status,
		"summary":  firstNonEmpty(summary, "-"),
		"complete": complete,
	}
}

func mixPluginTypeNames(rows []map[string]any) []string {
	out := []string{}
	for _, row := range rows {
		out = append(out, cleanContextText(row["required_type"]))
	}
	return compactNonEmptyStrings(out)
}

func mixCandidateSummary(rows []map[string]any) string {
	names := []string{}
	for _, row := range rows {
		name := firstNonEmpty(cleanContextText(row["name"]), cleanContextText(row["plugin_name"]), cleanContextText(row["descriptive_name"]))
		if name != "" {
			names = append(names, name)
		}
	}
	names = compactNonEmptyStrings(names)
	if len(names) == 0 {
		return ""
	}
	if len(names) > 4 {
		return strings.Join(names[:4], " / ") + " ..."
	}
	return strings.Join(names, " / ")
}

func mixChainSummary(rows []map[string]any) string {
	parts := []string{}
	for _, row := range rows {
		role := firstNonEmpty(cleanContextText(row["role"]), cleanContextText(row["type"]))
		if role != "" {
			parts = append(parts, role)
		}
	}
	return strings.Join(compactNonEmptyStrings(parts), " -> ")
}

func mixSkillProfileSummary(prep map[string]any) string {
	status := mapValue(prep["skill_profile_status"])
	profileStatus := cleanContextText(status["profile_status"])
	if profileStatus == "" {
		profileStatus = cleanContextText(prep["profile_status"])
	}
	if profileStatus == "" {
		return ""
	}
	next := cleanContextText(status["next_action"])
	if next != "" {
		return profileStatus + " / " + next
	}
	return profileStatus
}

func mixMacroPanelSummary(panel map[string]any) string {
	controls := mapRowsValue(panel["controls"])
	if len(controls) == 0 {
		return ""
	}
	names := []string{}
	for _, control := range controls {
		names = append(names, firstNonEmpty(cleanContextText(control["name"]), cleanContextText(control["label"]), cleanContextText(control["control"])))
	}
	return strings.Join(compactNonEmptyStrings(names), " / ")
}

func mixFastPacketSummary(prep map[string]any) string {
	if boolValue(prep["ready_to_publish_mixboard"]) {
		return "ready_to_package"
	}
	if len(mapRowsValue(prep["proposed_controls"])) > 0 {
		return "control_surface_draft_ready"
	}
	return ""
}

func mixFastModelStrategyPreview(session MixSession) map[string]any {
	return map[string]any{
		"route":              "mix_tick",
		"fast_model_profile": session.FastModelProfile,
		"reasoning_effort":   "low",
		"chain_of_thought":   "disabled",
	}
}

func compactNonEmptyStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
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

func mixPlannerMissingInputs(prep map[string]any) []string {
	missing := mixPlannerBaseMissingInputs(prep)
	if len(missing) > 0 {
		return missing
	}
	if workspaceMissing := mixPlannerWorkspaceMissingSlots(prep); len(workspaceMissing) > 0 {
		return workspaceMissing
	}
	if boolValue(prep["planner_revision_requested"]) {
		missing = append(missing, "planner_revision_note")
		return missing
	}
	if !boolValue(prep["publish_mixboard_prompted"]) || !boolValue(prep["publish_mixboard_confirmed"]) {
		missing = append(missing, "publish_mixboard_confirmed")
	}
	return missing
}

func mixPlannerBaseMissingInputs(prep map[string]any) []string {
	missing := []string{}
	if strings.TrimSpace(cleanContextText(prep["goal_detail"])) == "" {
		missing = append(missing, "goal_detail")
	}
	if _, ok := prep["allow_plugin_loads"]; !ok {
		missing = append(missing, "allow_plugin_loads")
	}
	return missing
}

func mixPlannerQuestions(missing []string) []map[string]any {
	out := []map[string]any{}
	for _, id := range missing {
		switch id {
		case "goal_detail":
			out = append(out, map[string]any{
				"id":       id,
				"question": "这次缩混要优先解决当前目标的什么问题？",
				"examples": []string{"清理低中频并轻微稳定动态", "让人声更靠前但不要刺耳"},
			})
		case "allow_plugin_loads":
			out = append(out, map[string]any{
				"id":       id,
				"question": "确认插件链后，是否允许规划器加载推荐的本地插件？",
				"examples": []string{"可以，允许加载推荐插件", "不可以，只使用当前已有实例"},
			})
		case "planner_plan_confirmed":
			out = append(out, map[string]any{
				"id":       id,
				"question": "请先复核规划草案；确认后我再发布 MixBoard。",
				"examples": []string{"确认规划，按这个来", "先别发布，我想调整插件方向"},
			})
		case "workspace_plugin_types":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我已经根据目标草拟了所需处理类型，需要先和你确认这个方向。",
				"examples": []string{"可以，继续推荐插件", "别用动态处理，先只做空间和电平"},
			})
		case "workspace_local_plugin_candidates":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我需要先列出本地候选插件并说明推荐理由，再继续规划。",
				"examples": []string{"请推荐本地可用插件", "优先使用已有 profile 的插件"},
			})
		case "workspace_plugin_chain_order":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我已经草拟了插件链顺序，需要先确认处理先后关系。",
				"examples": []string{"可以，按这个顺序", "先混响再压缩不合适，换一版"},
			})
		case "workspace_skill_profile_status":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我需要先检查候选插件是否具备 Plugin Grabber skill/profile。",
				"examples": []string{"如果缺 skill 就先停下", "优先使用 skill ready 的插件"},
			})
		case "workspace_macro_panel":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我需要先生成可执行的宏面板草案。",
				"examples": []string{"先给出宏控制草案", "只允许 virtual controls"},
			})
		case "workspace_fast_tick_packet":
			out = append(out, map[string]any{
				"id":       id,
				"question": "我需要先打包快速模式执行上下文。",
				"examples": []string{"执行阶段只使用确认过的控件", "不要让快速模式直接碰 raw 参数"},
			})
		case "planner_revision_note":
			out = append(out, map[string]any{
				"id":       id,
				"question": "先不生成 MixBoard。你想继续调整目标、插件选择、链路顺序，还是宏面板？",
				"examples": []string{"换一个更温和的动态处理方向", "先别用这个插件链，再推荐一版"},
			})
		case "publish_mixboard_confirmed":
			out = append(out, map[string]any{
				"id":       id,
				"question": "规划工作区已经完整。是否生成 MixBoard？",
				"examples": []string{"可以，生成 MixBoard", "还不行，我想继续调整"},
			})
		}
	}
	return out
}

func mixPlannerNextQuestion(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	switch missing[0] {
	case "goal_detail":
		return "在我生成控制面之前，请先告诉我这次缩混的方向。"
	case "allow_plugin_loads":
		return "请确认：如果选中的插件链需要插件实例，是否允许我加载推荐的本地插件？"
	case "planner_plan_confirmed":
		return "我会先整理混音目标、插件类型、候选插件、链路顺序和宏控制草案；请确认规划后再发布 MixBoard。"
	case "workspace_plugin_types":
		return "我已经根据目标草拟了所需处理类型。你可以确认继续，或告诉我要避开的处理方向。"
	case "workspace_local_plugin_candidates":
		return "我需要先列出本地候选插件并说明推荐理由。"
	case "workspace_plugin_chain_order":
		return "我已经草拟了插件链顺序。请确认这个处理先后关系，或告诉我想调整哪里。"
	case "workspace_skill_profile_status":
		return "我需要先检查候选插件的 skill/profile 状态，缺 skill 时必须停下学习。"
	case "workspace_macro_panel":
		return "我需要先生成宏面板草案，只包含 macro、virtual controls 或确认过的 binding。"
	case "workspace_fast_tick_packet":
		return "我需要先打包快速模式上下文，执行阶段只能消费确认后的控件。"
	case "planner_revision_note":
		return "先不生成 MixBoard。你想继续调整目标、插件选择、链路顺序，还是宏面板？"
	case "publish_mixboard_confirmed":
		return "规划工作区已经完整。是否生成 MixBoard？"
	default:
		return "规划器还需要一个补充答案，之后才能发布 MixBoard。"
	}
}

func mixPlannerChainOrder(roleTypes []string) []map[string]any {
	order := []map[string]any{{
		"slot":   0,
		"type":   "track_gain",
		"role":   "gain_staging",
		"reason": "先建立可回退的音量宏控制，再进入插件调控。",
	}}
	for i, roleType := range roleTypes {
		order = append(order, map[string]any{
			"slot":   i + 1,
			"type":   roleType,
			"role":   mixPlannerRoleName(roleType),
			"reason": "根据当前混音目标选择的处理角色。",
		})
	}
	return order
}

func mixPlannerRoleName(roleType string) string {
	switch roleType {
	case "eq":
		return "tone_balance"
	case "dynamics":
		return "dynamic_stability"
	case "de_ess":
		return "sibilance_control"
	case "reverb":
		return "depth_space"
	default:
		return roleType
	}
}

func mixPlannerModelStrategy() map[string]any {
	return map[string]any{
		"route":            "mix_strategy",
		"phase":            mixInteractionPlanningChat,
		"reasoning_effort": "normal",
		"chain_of_thought": "planner_private",
		"must_collect":     []string{"goal_detail", "plugin_type_plan", "plugin_candidates", "plugin_chain_order", "skill_profile_status", "macro_panel_draft"},
		"publish_gate":     "ready_to_publish_mixboard",
	}
}

func (s *Server) ensureMixPlannerPrepReady(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) (map[string]any, map[string]any, error) {
	requestContext := mergeContext(interaction.RequestContext, mapValue(data["request_context"]))
	prep := mixPlannerPrepFromData(data, session, requestContext)
	defer func() { syncMixPlanningWorkspace(prep, session) }()
	absorbMixPlannerAnswers(prep, &session, payload)
	updateMixPlannerPrepDerived(prep, session)
	if len(mixPlannerBaseMissingInputs(prep)) > 0 {
		return prep, mapValue(data["mix_observation"]), nil
	}
	observation := mapValue(data["mix_observation"])
	if len(observation) == 0 || cleanContextText(prep["stage"]) != "planner_preflight_complete" {
		var err error
		session.State = mixStateReadyObservation
		observation, err = s.requestInitialMixObservation(ctx, interaction, session)
		session.State = mixStateFromObservationResult(observation, err)
		session.BlockingPoint = mixObservationBlockingPoint(observation, err)
		if err != nil {
			prep["stage"] = "planner_observation_blocked"
			prep["blocking_point"] = err.Error()
			prep["ready_to_publish_mixboard"] = false
			return prep, observation, err
		}
		s.attachGoalControlSurface(ctx, interaction, session, observation)
		fillMixPlannerPrepFromObservation(prep, observation, session)
		updateMixPlannerPrepDerived(prep, session)
		applyMixPlannerPrep(observation, prep)
		if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
			packet := buildMixTickPacket(session, observation, "")
			applyMixTickPacket(observation, packet)
		}
	}
	if !boolValue(prep["ready_to_publish_mixboard"]) {
		prep["updated_at"] = time.Now().Format(time.RFC3339Nano)
		return prep, observation, nil
	}
	prep["updated_at"] = time.Now().Format(time.RFC3339Nano)
	return prep, observation, nil
}

func fillMixPlannerPrepFromObservation(prep map[string]any, observation map[string]any, session MixSession) {
	surface := mixGoalControlSurfaceFromObservation(observation)
	if len(surface) > 0 {
		prep["goal_control_surface_draft"] = surface
		prep["plugin_candidates"] = mapRowsValue(surface["plugin_candidates"])
		prep["selected_chain_draft"] = mapRowsValue(surface["selected_chain"])
		prep["proposed_controls"] = mapRowsValue(surface["proposed_controls"])
		prep["instance_status"] = cleanContextText(surface["instance_status"])
		prep["profile_status"] = cleanContextText(surface["profile_status"])
		prep["skill_profile_status"] = map[string]any{
			"profile_status": cleanContextText(surface["profile_status"]),
			"blockers":       contextStringSlice(surface["blockers"]),
			"next_action":    cleanContextText(surface["next_required_action"]),
		}
		if learningRequest := mixPluginLearningRequestFromSurface(surface); len(learningRequest) > 0 {
			prep["stage"] = mixInteractionLearningRequired
			prep["ready_to_publish_mixboard"] = false
			prep["plugin_learning_request"] = learningRequest
			prep["blocking_point"] = "需要先完成 Plugin Grabber 学习：" + firstNonEmpty(cleanContextText(learningRequest["plugin_name"]), cleanContextText(learningRequest["type"]))
		} else if cleanContextText(surface["readiness"]) == mixcontrolsurface.ReadinessBlocked {
			prep["blocking_point"] = strings.Join(contextStringSlice(surface["blockers"]), "; ")
		} else {
			delete(prep, "blocking_point")
		}
	}
	if plan := ensureMixControlPlan(observation, session); len(plan) > 0 {
		prep["macro_panel_draft"] = mapValue(plan["macro_control_panel"])
	}
}

func applyMixPlannerPrep(observation map[string]any, prep map[string]any) {
	if len(observation) == 0 || len(prep) == 0 {
		return
	}
	workspace := mapValue(prep["mix_planning_workspace"])
	observation["mix_planner_prep"] = prep
	if len(workspace) > 0 {
		observation["mix_planning_workspace"] = workspace
	}
	board := mapValue(observation["mixboard"])
	board["mix_planner_prep"] = prep
	if len(workspace) > 0 {
		board["mix_planning_workspace"] = workspace
	}
	board["planner_stage"] = cleanContextText(prep["stage"])
	board["planner_ready_to_publish"] = boolValue(prep["ready_to_publish_mixboard"])
	observation["mixboard"] = board
	if contextPack := mapValue(observation["context_pack"]); len(contextPack) > 0 {
		contextPack["mix_planner_prep"] = prep
		if len(workspace) > 0 {
			contextPack["mix_planning_workspace"] = workspace
		}
		observation["context_pack"] = contextPack
	}
}

func applyMixPlannerPrepToWorkflowData(data map[string]any, prep map[string]any) {
	if len(data) == 0 || len(prep) == 0 {
		return
	}
	workspace := mapValue(prep["mix_planning_workspace"])
	data["mix_planner_prep"] = prep
	if len(workspace) > 0 {
		data["mix_planning_workspace"] = workspace
	}
}

func applyMixPlannerPrepSessionState(session *MixSession, prep map[string]any) {
	if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
		session.InteractionPhase = mixInteractionLearningRequired
		session.MixBoardVisibility = mixBoardVisibilityPublished
		return
	}
	session.InteractionPhase = mixInteractionPlanningChat
	session.MixBoardVisibility = mixBoardVisibilityCollapsed
}

func mixPlannerPrepReply(prep map[string]any, err error) string {
	workspace := mapValue(prep["mix_planning_workspace"])
	if err != nil {
		return "规划预检被阻断：" + err.Error()
	}
	if cleanContextText(prep["stage"]) == mixInteractionLearningRequired {
		return firstNonEmpty(cleanContextText(prep["blocking_point"]), "发布可执行 MixBoard 前需要先完成 Plugin Grabber 学习。")
	}
	if boolValue(prep["ready_to_publish_mixboard"]) {
		return mixPlanningConversationalReply(workspace, "规划信息已经足够。我已经整理好混音目标、插件角色、本地候选、链路顺序、skill/profile 状态和宏面板草案；确认后可以发布 MixBoard。")
	}
	if question := cleanContextText(prep["next_question"]); question != "" {
		return mixPlanningConversationalReply(workspace, question)
	}
	return mixPlanningConversationalReply(workspace, "发布 MixBoard 前还需要继续补齐规划信息。")
}

func mixPlanningConversationalReply(workspace map[string]any, next string) string {
	lines := []string{}
	goal := mapValue(workspace["goal"])
	if detail := cleanContextText(goal["detail"]); detail != "" {
		lines = append(lines, "我先把当前方向记为："+detail)
	} else if summary := cleanContextText(goal["summary"]); summary != "" {
		lines = append(lines, "我已经为这次任务建立了混音规划工作区："+summary)
	} else {
		lines = append(lines, "我已经进入混音思考模式，正在后台整理这次任务的规划工作区。")
	}
	if slotLine := mixPlanningIncompleteSlotLine(mapRowsValue(workspace["workflow_slots"])); slotLine != "" {
		lines = append(lines, slotLine)
	}
	if strings.TrimSpace(next) != "" {
		lines = append(lines, "接下来我需要确认："+next)
	}
	return strings.Join(lines, "\n")
}

func mixPlanningIncompleteSlotLine(slots []map[string]any) string {
	missing := []string{}
	done := []string{}
	for _, slot := range slots {
		label := cleanContextText(slot["label"])
		if label == "" {
			continue
		}
		if boolValue(slot["complete"]) {
			done = append(done, label)
		} else {
			missing = append(missing, label)
		}
	}
	if len(missing) == 0 && len(done) == 0 {
		return ""
	}
	if len(missing) == 0 {
		return "当前规划工作区已经补齐，可以进入发布前确认。"
	}
	if len(missing) > 3 {
		missing = missing[:3]
	}
	return "当前还在补齐：" + strings.Join(missing, "、")
}

func mixPlannerInteractionBody(prep map[string]any) string {
	parts := []string{
		"阶段：" + mixPlannerStageLabel(firstNonEmpty(cleanContextText(prep["stage"]), "planner_intake")),
		"MixBoard：已收起",
		"目标：" + firstNonEmpty(cleanContextText(prep["goal_summary"]), "-"),
	}
	if detail := cleanContextText(prep["goal_detail"]); detail != "" {
		parts = append(parts, "混音方向："+detail)
	}
	if question := cleanContextText(prep["next_question"]); question != "" {
		parts = append(parts, "下一步："+question)
	}
	parts = append(parts, mixPlannerDraftSummary(prep)...)
	if boolValue(prep["ready_to_publish_mixboard"]) {
		parts = append(parts, "状态：可以发布 MixBoard")
	}
	return strings.Join(parts, "\n")
}

func mixPlannerDraftSummary(prep map[string]any) []string {
	out := []string{}
	if rows := mapRowsValue(prep["plugin_type_plan"]); len(rows) > 0 {
		types := []string{}
		for _, row := range rows {
			if typ := cleanContextText(row["required_type"]); typ != "" {
				types = append(types, typ)
			}
		}
		if len(types) > 0 {
			out = append(out, "插件类型："+strings.Join(types, " / "))
		}
	}
	if rows := mapRowsValue(prep["plugin_chain_order"]); len(rows) > 0 {
		chain := []string{}
		for _, row := range rows {
			if role := cleanContextText(row["role"]); role != "" {
				chain = append(chain, role)
			}
		}
		if len(chain) > 0 {
			out = append(out, "链路顺序："+strings.Join(chain, " -> "))
		}
	}
	if rows := mapRowsValue(prep["selected_chain_draft"]); len(rows) > 0 {
		selected := []string{}
		for _, row := range rows {
			plugin := mapValue(row["selected_plugin"])
			name := firstNonEmpty(cleanContextText(plugin["name"]), cleanContextText(row["plugin_name"]))
			if name != "" {
				selected = append(selected, cleanContextText(row["role"])+":"+name)
			}
		}
		if len(selected) > 0 {
			out = append(out, "插件草案："+strings.Join(selected, "；"))
		}
	}
	if panel := mapValue(prep["macro_panel_draft"]); len(panel) > 0 {
		out = append(out, "宏面板：已生成草案")
	}
	return out
}

func mixPlannerStageLabel(stage string) string {
	switch stage {
	case "planner_intake":
		return "补充规划信息"
	case "planner_preflight_ready":
		return "准备预检"
	case "planner_draft_review":
		return "复核规划草案"
	case "planner_preflight_complete":
		return "规划预检完成"
	case "planner_observation_blocked":
		return "观察预检受阻"
	case mixInteractionLearningRequired:
		return "需要插件学习"
	default:
		return firstNonEmpty(stage, "规划中")
	}
}

func mixPlannerInteractionFields(prep map[string]any) []AgentInteractionField {
	missing := mixPlannerMissingInputs(prep)
	fields := []AgentInteractionField{}
	for _, id := range missing {
		switch id {
		case "goal_detail":
			fields = append(fields, AgentInteractionField{
				ID:          "goal_detail",
				Label:       "混音方向",
				Kind:        "textarea",
				Required:    true,
				Placeholder: "例如：清理低中频，让人声靠前，轻微稳定动态",
			})
		case "allow_plugin_loads":
			fields = append(fields, AgentInteractionField{
				ID:       "allow_plugin_loads",
				Label:    "允许加载插件",
				Kind:     "boolean",
				Required: true,
				Value:    false,
			})
		case "planner_plan_confirmed":
			fields = append(fields, AgentInteractionField{
				ID:       "planner_plan_confirmed",
				Label:    "确认规划草案",
				Kind:     "boolean",
				Required: true,
				Value:    false,
			})
		}
	}
	return fields
}

func mixPlannerInteractionActions(prep map[string]any) []AgentInteractionAction {
	if boolValue(prep["ready_to_publish_mixboard"]) {
		return []AgentInteractionAction{
			{ID: "publish_mixboard", Label: "发布 MixBoard", Style: "primary", Recommended: true},
			{ID: "advance_mix_planner", Label: "刷新规划", Style: "secondary"},
			{ID: "enter_discussion", Label: "继续讨论", Style: "secondary"},
			{ID: "cancel_mix_session", Label: "取消", Style: "secondary"},
		}
	}
	if cleanContextText(prep["stage"]) == "planner_draft_review" && boolValue(prep["publish_mixboard_prompted"]) && mixPlannerWorkspaceComplete(prep) {
		return []AgentInteractionAction{
			{ID: "confirm_mix_planner_plan", Label: "确认生成 MixBoard", Style: "primary", Recommended: true},
			{ID: "advance_mix_planner", Label: "继续调整规划", Style: "secondary"},
			{ID: "enter_discussion", Label: "继续讨论", Style: "secondary"},
			{ID: "cancel_mix_session", Label: "取消", Style: "secondary"},
		}
	}
	return []AgentInteractionAction{
		{ID: "advance_mix_planner", Label: "继续规划", Style: "primary", Recommended: true},
		{ID: "enter_discussion", Label: "继续讨论", Style: "secondary"},
		{ID: "cancel_mix_session", Label: "取消", Style: "secondary"},
	}
}

func mixPlanningDiscussionReply(session MixSession) string {
	workspace := mapValue(session.PlannerPrep["mix_planning_workspace"])
	if len(workspace) > 0 {
		return mixPlanningConversationalReply(workspace, cleanContextText(workspace["next_question"]))
	}
	return fmt.Sprintf("%s 已进入思考讨论模式。我会在后台建立混音规划工作区，先和你确认目标、插件类型、候选插件、链路顺序、skill/profile、宏面板和快速模式上下文；MixBoard 会先保持收起。", mixModeLabel(session.Mode))
}

func (s *Server) mixPlanningDiscussionInteraction(interaction PendingInteraction, session MixSession, data map[string]any) AgentInteractionRequest {
	prep := mixPlannerPrepFromData(data, session, interaction.RequestContext)
	workspace := syncMixPlanningWorkspace(prep, session)
	payload := map[string]any{
		"mix_session":            mixSessionMap(session),
		"mix_planner_prep":       prep,
		"mix_planning_workspace": workspace,
		"request_context":        interaction.RequestContext,
		"interaction_phase":      mixInteractionPlanningChat,
		"mixboard_visibility":    mixBoardVisibilityCollapsed,
		"planner_policy":         mixPlannerPolicy(session),
	}
	for key, value := range data {
		if _, exists := payload[key]; !exists {
			payload[key] = value
		}
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "mode_boundary",
		Type:           "mix_planning_discussion",
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Stage:          mixInteractionPlanningChat,
		Title:          "混音规划",
		Body:           mixPlannerInteractionBody(prep),
		Status:         "waiting",
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		ReviewItems:    mixPlanningWorkspaceReviewItems(workspace),
		Payload:        payload,
		Data:           payload,
		Actions:        mixPlannerInteractionActions(prep),
	}
	if s != nil {
		s.storePendingInteraction(req, payload)
	}
	return req
}

func mixPlanningWorkspaceReviewItems(workspace map[string]any) []AgentInteractionReview {
	items := []AgentInteractionReview{}
	for _, slot := range mapRowsValue(workspace["workflow_slots"]) {
		status := "pending"
		if boolValue(slot["complete"]) {
			status = "complete"
		}
		items = append(items, AgentInteractionReview{
			ID:     cleanContextText(slot["id"]),
			Title:  cleanContextText(slot["label"]),
			Body:   cleanContextText(slot["summary"]),
			Status: status,
			Payload: map[string]any{
				"complete": boolValue(slot["complete"]),
			},
		})
	}
	return items
}

func mixSessionAfterControlSurface(session MixSession, observation map[string]any, confirmed bool) MixSession {
	surface := mixGoalControlSurfaceFromObservation(observation)
	packet := mixTickPacketFromObservation(observation)
	status := cleanContextText(packet["status"])
	if status == "" {
		status = cleanContextText(surface["readiness"])
	}
	switch {
	case len(mixPluginLearningRequestFromObservation(observation)) > 0 || status == mixInteractionLearningRequired:
		session.InteractionPhase = mixInteractionLearningRequired
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "执行前需要先完成 Plugin Grabber 学习。"
	case confirmed && status == "ready":
		session.InteractionPhase = mixInteractionReadyForTick
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = ""
	case status == mixcontrolsurface.ReadinessPlanReady || status == "ready":
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = ""
	case status == mixcontrolsurface.ReadinessNeedsConfirmation || status == "needs_confirmation":
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
		session.BlockingPoint = "Control surface needs confirmation before execution."
	default:
		session.InteractionPhase = mixInteractionControlSurfacePublished
		session.MixBoardVisibility = mixBoardVisibilityPublished
	}
	return session
}

func selectMixTickControl(packet map[string]any, payload map[string]any) (map[string]any, error) {
	fields := mapValue(payload["fields"])
	if strings.EqualFold(cleanContextText(payload["tool"]), "set_plugin_param") ||
		strings.EqualFold(cleanContextText(payload["command_name"]), "set_plugin_param") ||
		strings.EqualFold(cleanContextText(payload["control_kind"]), "raw_param") {
		return nil, fmt.Errorf("raw plugin parameter writes are not allowed in single tick execution")
	}
	requestedID := firstNonEmpty(cleanContextText(payload["control_id"]), cleanContextText(fields["control_id"]))
	requestedName := firstNonEmpty(cleanContextText(payload["control_name"]), cleanContextText(payload["control"]), cleanContextText(fields["control_name"]), cleanContextText(fields["control"]))
	allowed := mapRowsValue(packet["allowed_controls"])
	if len(allowed) == 0 {
		return nil, fmt.Errorf("MixTickPacket has no allowed controls")
	}
	for _, control := range allowed {
		if requestedID != "" && cleanContextText(control["control_id"]) == requestedID {
			return control, nil
		}
		if requestedName != "" && strings.EqualFold(cleanContextText(control["control_name"]), requestedName) {
			return control, nil
		}
	}
	if requestedID != "" || requestedName != "" {
		return nil, fmt.Errorf("requested control is outside MixTickPacket.allowed_controls")
	}
	for _, control := range allowed {
		if cleanContextText(control["control_kind"]) == "plugin_virtual_control" {
			return control, nil
		}
	}
	return allowed[0], nil
}

func (s *Server) applySelectedMixTickControl(ctx context.Context, interaction PendingInteraction, requestContext map[string]any, session MixSession, packet, control map[string]any, stepDB float64, userNote string) (map[string]any, []map[string]any, error) {
	tickID := "mix_tick_" + randomID()
	decision := map[string]any{
		"schema_version":          mixTickDecisionSchemaVersion,
		"decision":                "apply",
		"control_id":              cleanContextText(control["control_id"]),
		"control_name":            cleanContextText(control["control_name"]),
		"small_step":              true,
		"reason":                  "Apply one allowed control from MixTickPacket.",
		"expected_effect":         session.GoalText,
		"confidence":              "medium",
		"requires_planner_review": true,
	}
	turn := map[string]any{
		"tick_id":                 tickID,
		"tuning_batch_id":         tickID,
		"turn":                    1,
		"batch_turn":              1,
		"batch_max_turns":         1,
		"session_round":           session.RoundCount + 1,
		"executor_type":           mixSingleTickExecutorType,
		"executor_version":        mixSingleTickExecutorVersion,
		"mix_tick_packet_id":      cleanContextText(packet["mix_session_id"]) + ":" + cleanContextText(packet["created_at"]),
		"mix_tick_decision":       decision,
		"control_id":              cleanContextText(control["control_id"]),
		"control_name":            cleanContextText(control["control_name"]),
		"control_kind":            cleanContextText(control["control_kind"]),
		"track_id":                firstNonEmpty(cleanContextText(control["track_id"]), session.TargetRef.ID),
		"target_label":            firstNonEmpty(session.TargetRef.Label, session.TargetRef.ID),
		"user_note":               userNote,
		"status":                  "pending",
		"review_status":           mixReviewWaiting,
		"rollback_available":      true,
		"raw_param_write":         false,
		"planner_policy":          mixPlannerPolicy(session),
		"fast_model_profile":      mixFastModelProfile(session, nil),
		"model_strategy":          mixFastModelStrategy(mixFastModelProfile(session, nil)),
		"requires_planner_review": true,
	}
	switch cleanContextText(control["control_kind"]) {
	case "macro":
		if !strings.EqualFold(session.TargetRef.Kind, "track") {
			return turn, nil, fmt.Errorf("macro %s requires a track target", cleanContextText(control["control_name"]))
		}
		currentDB := mixTrackVolumeDB(s.harness.UserStateSummary(ctx), session.TargetRef.ID)
		nextDB := clampMixVolumeDB(currentDB + stepDB)
		macro := mixVolumeMacroFromObservation(mapValue(packet["observation"]), session)
		if len(macro) == 0 || cleanContextText(macro["macro_id"]) == "" {
			macro = mixBuiltInVolumeMacro(session, currentDB)
		}
		macro["macro_id"] = strings.TrimPrefix(cleanContextText(control["control_id"]), "macro:")
		macro["control"] = cleanContextText(control["control_name"])
		macro["track_id"] = firstNonEmpty(cleanContextText(control["track_id"]), session.TargetRef.ID)
		resp, err := s.applyMixMacroControl(ctx, interaction, requestContext, macro, nextDB)
		turn["before_db"] = currentDB
		turn["target_db"] = nextDB
		turn["step_db"] = roundMixFloat(nextDB - currentDB)
		turn["direction"] = mixTuningDirectionLabel(stepDB)
		turn["direction_source"] = mixTuningDirectionSource(session.GoalText, map[string]any{"user_note": userNote, "step_db": stepDB})
		turn["agent_action_id"] = resp.AgentActionID
		turn["status"] = resp.Status
		if err != nil || resp.Status != "ok" {
			return turn, mixMacroValueExecution(macro, nextDB, resp), fmt.Errorf("%s", firstNonEmpty(resp.Error, fmt.Sprint(err), "macro single tick failed"))
		}
		return turn, mixMacroValueExecution(macro, nextDB, resp), nil
	case "plugin_virtual_control":
		if cleanContextText(control["plugin_id"]) == "" || cleanContextText(control["track_id"]) == "" {
			return turn, nil, fmt.Errorf("plugin virtual control requires ready track_id and plugin_id")
		}
		target := map[string]any{
			"intent":     firstNonEmpty(userNote, session.GoalText),
			"amount":     "small",
			"small_step": true,
		}
		if componentID := cleanContextText(control["component_id"]); componentID != "" {
			target["component_id"] = componentID
		}
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool: "plugin_grabber.apply_control",
			Args: map[string]any{
				"track_id":  cleanContextText(control["track_id"]),
				"plugin_id": cleanContextText(control["plugin_id"]),
				"control":   cleanContextText(control["control_name"]),
				"target":    target,
			},
			Context:   requestContext,
			Source:    "mixboard_single_tick",
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
		turn["plugin_id"] = cleanContextText(control["plugin_id"])
		turn["plugin_name"] = cleanContextText(control["plugin_name"])
		turn["component_id"] = cleanContextText(control["component_id"])
		turn["target"] = target
		turn["agent_action_id"] = resp.AgentActionID
		turn["status"] = resp.Status
		if applied := mapRowsValue(resp.Result["applied_parameters"]); len(applied) > 0 {
			turn["applied_parameters"] = applied
		}
		if err != nil || resp.Status != "ok" {
			return turn, nil, fmt.Errorf("%s", firstNonEmpty(resp.Error, fmt.Sprint(err), "plugin virtual control single tick failed"))
		}
		return turn, nil, nil
	default:
		return turn, nil, fmt.Errorf("unsupported single tick control kind %q", cleanContextText(control["control_kind"]))
	}
}

func mixTickCurrentAction(turn map[string]any) string {
	switch cleanContextText(turn["control_kind"]) {
	case "plugin_virtual_control":
		return fmt.Sprintf("Single tick: %s via %s.", cleanContextText(turn["control_name"]), cleanContextText(turn["plugin_name"]))
	case "macro":
		return fmt.Sprintf("Single tick: %s %.2f dB -> %.2f dB.", cleanContextText(turn["control_name"]), mixFloatNumber(turn["before_db"]), mixFloatNumber(turn["target_db"]))
	default:
		return "Single mix tick completed."
	}
}

func (s *Server) runMixTuningInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession, payload map[string]any) ChatResponse {
	if session.MixSessionID == "" {
		session = mixSessionFromMap(mapValue(payload["mix_session"]))
	}
	if session.MixSessionID == "" {
		session = mixSessionFromMap(mapValue(data["mix_session"]))
	}
	if session.Mode == "" {
		session.Mode = firstNonEmpty(cleanContextText(data["mode"]), mixModeAuto)
	}
	if blocker := mixGoalControlSurfaceTuningBlocker(mapValue(data["mix_observation"])); blocker != "" {
		session.BlockingPoint = blocker
		session.Preparation = mixPreparationRowsForObservation(session.TargetRef, mixStateObservationReady, blocker)
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		observation := mapValue(data["mix_observation"])
		updateMixBoardRuntimeState(observation, session, nil)
		if len(observation) > 0 {
			data["mix_observation"] = observation
		}
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               blocker,
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         session.State,
		}
	}
	if session.TargetRef.ID == "" || !strings.EqualFold(session.TargetRef.Kind, "track") {
		session.State = mixStateFailed
		session.BlockingPoint = "自动调参 v0 只支持当前轨道音量。"
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		observation := mapValue(data["mix_observation"])
		updateMixBoardRuntimeState(observation, session, nil)
		if len(observation) > 0 {
			data["mix_observation"] = observation
		}
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               session.BlockingPoint,
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         session.State,
		}
	}
	if s == nil || s.harness == nil {
		session.State = mixStateFailed
		session.BlockingPoint = "harness 不可用，暂不能执行自动调参。"
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		return ChatResponse{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			Reply:          session.BlockingPoint,
			Workflow:       mixSessionEntryWorkflow,
			WorkflowData:   data,
			MixSession:     mixSessionMap(session),
			GoalStatus:     string(agentruntime.StatusWaitingContinue),
			CurrentStep:    session.State,
		}
	}
	session.State = mixStateTuningRunning
	session.BlockingPoint = ""
	session.ExecutorType = mixFallbackExecutorType
	session.ExecutorVersion = mixFallbackExecutorVersion
	session.ReviewStatus = mixReviewWaiting
	session.StopReason = ""
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	turns := []map[string]any{}
	observationSeed := mapValue(data["mix_observation"])
	updateMixBoardRuntimeState(observationSeed, session, nil)
	data["mix_observation"] = observationSeed
	volumeMacro := mixVolumeMacroFromObservation(observationSeed, session)
	executedReplies := mixMacroUpsertExecutions(volumeMacro)
	requestContext := mergeContext(interaction.RequestContext, mapValue(data["request_context"]))
	requestContext = mergeContext(requestContext, mapValue(payload["request_context"]))
	userNote := mixTuningUserNote(payload, requestContext, data, session)
	if userNote != "" {
		requestContext["user_note"] = userNote
		data["user_note"] = userNote
		session.UserNote = userNote
		payload["user_note"] = userNote
	}
	stepDB := mixTuningStepDB(session.GoalText, payload)
	directionLabel := mixTuningDirectionLabel(stepDB)
	directionSource := mixTuningDirectionSource(session.GoalText, payload)
	batchID := "mix_tune_" + randomID()
	writeMixBoardDiag("tuning_start", map[string]any{
		"mix_session_id":   session.MixSessionID,
		"tuning_batch_id":  batchID,
		"target_id":        session.TargetRef.ID,
		"goal_text":        session.GoalText,
		"user_note":        userNote,
		"step_db":          stepDB,
		"direction":        directionLabel,
		"direction_source": directionSource,
		"payload_keys":     mapKeysForDiag(payload),
	})
	if s != nil && s.logger != nil {
		s.logger.Info("[mixboard.tuning] start session=%s target=%s step_db=%.3f direction=%s source=%s user_note=%q goal=%q", session.MixSessionID, session.TargetRef.ID, stepDB, directionLabel, directionSource, userNote, session.GoalText)
	}
	if mixTuningShouldHoldVolume(userNote, stepDB) {
		session.State = mixStateTuningPaused
		session.ReviewStatus = mixReviewWaiting
		session.StopReason = mixStopReasonUserIntervention
		session.BlockingPoint = "用户干预要求暂停当前音量方向，本轮未继续推子调节。"
		session.Preparation = mixPreparationRowsForObservation(session.TargetRef, mixStateObservationReady, session.BlockingPoint)
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		data["request_context"] = requestContext
		observationResult := mapValue(data["mix_observation"])
		board := mapValue(observationResult["mixboard"])
		board["user_note"] = userNote
		board["current_action"] = "用户干预要求暂停当前音量方向，本轮未继续推子调节。"
		board["current_judgement"] = mixTuningUserHoldJudgement(userNote)
		board["next_step"] = mixTuningUserHoldNextStep(userNote)
		board["review_status"] = session.ReviewStatus
		board["auto_tune_turns"] = turns
		board["executor_type"] = session.ExecutorType
		board["executor_version"] = session.ExecutorVersion
		board["mix_tuning_direction"] = directionLabel
		board["mix_tuning_direction_source"] = directionSource
		board["mix_tuning_step_db"] = stepDB
		board["mix_tuning_user_note"] = userNote
		board["session_state"] = session.State
		board["stop_reason"] = session.StopReason
		observationResult["mixboard"] = board
		updateMixBoardRuntimeState(observationResult, session, turns)
		data["mix_observation"] = observationResult
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               board["current_judgement"].(string),
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observationResult)},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         session.State,
		}
	}
	if session.MaxRounds <= 0 {
		session.MaxRounds = pendingMixSession(session.Mode, session.TargetRef, session.GoalText).MaxRounds
	}
	remainingTurns := session.MaxRounds - session.RoundCount
	if remainingTurns <= 0 {
		session.State = mixStateTuningComplete
		session.StopReason = mixStopReasonMaxRounds
		session.BlockingPoint = "mix_session_max_rounds_reached"
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		observation := mapValue(data["mix_observation"])
		updateMixBoardRuntimeState(observation, session, nil)
		if len(observation) > 0 {
			data["mix_observation"] = observation
		}
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               "自动调参已达到本次 MixSession 的轮次上限。",
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         session.State,
		}
	}
	maxTurns := mixAutoTuneMaxRounds
	if remainingTurns < maxTurns {
		maxTurns = remainingTurns
	}
	if requested := intNumber(payload["max_turns"]); requested > 0 && requested < maxTurns {
		maxTurns = requested
	}
	var observationResult map[string]any
	var observationErr error
	latestMacroValue := 0.0
	hasLatestMacroValue := false
	for turn := 1; turn <= maxTurns; turn++ {
		currentDB := mixTrackVolumeDB(s.harness.UserStateSummary(ctx), session.TargetRef.ID)
		nextDB := clampMixVolumeDB(currentDB + stepDB)
		volumeMacro = mixMacroWithValue(volumeMacro, currentDB)
		applyResp, applyErr := s.applyMixMacroControl(ctx, interaction, requestContext, volumeMacro, nextDB)
		executedReplies = append(executedReplies, mixMacroValueExecution(volumeMacro, nextDB, applyResp)...)
		if applyErr == nil && applyResp.Status == "ok" {
			latestMacroValue = nextDB
			hasLatestMacroValue = true
		}
		turnRow := map[string]any{
			"tuning_batch_id":  batchID,
			"turn":             turn,
			"batch_turn":       turn,
			"batch_max_turns":  maxTurns,
			"session_round":    session.RoundCount + 1,
			"executor_type":    session.ExecutorType,
			"executor_version": session.ExecutorVersion,
			"control":          "mix.macro",
			"macro_id":         cleanContextText(volumeMacro["macro_id"]),
			"macro_label":      firstNonEmpty(cleanContextText(volumeMacro["name"]), "轨道电平"),
			"macro_control":    cleanContextText(volumeMacro["control"]),
			"control_label":    "轨道音量",
			"track_id":         session.TargetRef.ID,
			"target_label":     firstNonEmpty(session.TargetRef.Label, session.TargetRef.ID, "当前轨道"),
			"before_db":        currentDB,
			"target_db":        nextDB,
			"step_db":          roundMixFloat(nextDB - currentDB),
			"direction":        directionLabel,
			"direction_source": directionSource,
			"user_note":        userNote,
			"agent_action_id":  applyResp.AgentActionID,
			"status":           applyResp.Status,
			"review_status":    "waiting_review",
		}
		if applyErr != nil || applyResp.Status != "ok" {
			turnRow["error"] = firstNonEmpty(applyResp.Error, fmt.Sprint(applyErr), "track volume adjustment failed")
			turns = append(turns, turnRow)
			session.State = mixStateFailed
			session.ReviewStatus = mixReviewFailed
			session.BlockingPoint = cleanContextText(turnRow["error"])
			break
		}
		if s != nil && s.logger != nil {
			s.logger.Info("[mixboard.tuning] turn session=%s turn=%d track=%s before_db=%.3f target_db=%.3f step_db=%.3f direction=%s user_note=%q status=%s", session.MixSessionID, turn, session.TargetRef.ID, currentDB, nextDB, roundMixFloat(nextDB-currentDB), directionLabel, userNote, applyResp.Status)
		}
		writeMixBoardDiag("tuning_turn", map[string]any{
			"mix_session_id":   session.MixSessionID,
			"tuning_batch_id":  batchID,
			"turn":             turn,
			"track_id":         session.TargetRef.ID,
			"before_db":        currentDB,
			"target_db":        nextDB,
			"step_db":          roundMixFloat(nextDB - currentDB),
			"direction":        directionLabel,
			"direction_source": directionSource,
			"user_note":        userNote,
			"status":           applyResp.Status,
			"agent_action_id":  applyResp.AgentActionID,
			"error":            firstNonEmpty(applyResp.Error, fmt.Sprint(applyErr)),
		})
		session.JournalRefs = append(session.JournalRefs, applyResp.AgentActionID)
		session.RoundCount++
		if session.RoundCount <= 0 {
			session.RoundCount = 1
		}
		observationResult, observationErr = s.requestMixObservationRound(ctx, interaction, session, session.RoundCount, map[string]any{
			"last_mix_action": turnRow,
		})
		turnRow["observation_status"] = firstNonEmpty(cleanContextText(observationResult["status"]), "unavailable")
		turnRow["delta_status"] = mixBeforeAfterDeltaStatus(observationResult)
		turnRow["decision"] = mixTuneDecisionFromObservation(observationResult, stepDB)
		turnRow["review_status"] = mixTuneReviewStatus(cleanContextText(turnRow["decision"]), cleanContextText(turnRow["delta_status"]))
		turns = append(turns, turnRow)
		writeMixTuneActionRecord(observationResult, turnRow)
		if observationErr != nil {
			session.State = mixStateObservationUnavailable
			session.StopReason = mixStopReasonObservationUnavailable
			session.BlockingPoint = observationErr.Error()
			break
		}
		if turnRow["decision"] == "stop" {
			session.StopReason = mixStopReasonNoSignificantChange
			break
		}
	}
	if session.State == mixStateTuningRunning {
		session.State = mixStateWaitingReview
	}
	session.ReviewStatus = mixTuneLatestReviewStatus(turns)
	if session.StopReason == "" && session.RoundCount >= session.MaxRounds {
		session.StopReason = mixStopReasonMaxRounds
	}
	session.Preparation = mixPreparationRowsForObservation(session.TargetRef, mixStateObservationReady, session.BlockingPoint)
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	data["request_context"] = requestContext
	if observationResult != nil {
		board := mapValue(observationResult["mixboard"])
		board["current_action"] = mixTuneCurrentAction(turns)
		board["current_judgement"] = mixTuneJudgement(turns)
		board["next_step"] = "试听本轮结果，必要时继续小步调控或撤回上一轮。"
		board["auto_tune_turns"] = turns
		board["last_action_summary"] = mixTuneCurrentAction(turns)
		board["review_status"] = session.ReviewStatus
		board["executor_type"] = session.ExecutorType
		board["executor_version"] = session.ExecutorVersion
		board["session_state"] = session.State
		board["stop_reason"] = session.StopReason
		board["round_count"] = session.RoundCount
		board["max_rounds"] = session.MaxRounds
		board["rollback_available"] = len(session.JournalRefs) > 0
		if userNote != "" {
			board["user_note"] = userNote
		}
		observationResult["mixboard"] = board
		if hasLatestMacroValue {
			setMixControlMacroValue(observationResult, cleanContextText(volumeMacro["macro_id"]), latestMacroValue)
		}
		updateMixBoardRuntimeState(observationResult, session, turns)
		data["mix_observation"] = observationResult
	}
	reply := mixTuneReply(session, turns, observationErr)
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observationResult)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func (s *Server) stopMixTuningInteraction(interaction PendingInteraction, data map[string]any, session MixSession) ChatResponse {
	session.State = mixStateTuningPaused
	session.ExecutorType = firstNonEmpty(session.ExecutorType, mixFallbackExecutorType)
	session.ExecutorVersion = firstNonEmpty(session.ExecutorVersion, mixFallbackExecutorVersion)
	session.ReviewStatus = mixReviewWaiting
	session.StopReason = mixStopReasonUserIntervention
	session.BlockingPoint = "用户已停止自动调参。"
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	observation := mapValue(data["mix_observation"])
	updateMixBoardRuntimeState(observation, session, nil)
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               "已停止自动调参。",
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func (s *Server) rollbackLastMixTurnInteraction(ctx context.Context, interaction PendingInteraction, data map[string]any, session MixSession) ChatResponse {
	if len(session.JournalRefs) == 0 {
		session.BlockingPoint = "没有可撤回的自动调参动作。"
		session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
		s.storeMixSession(session)
		data["mix_session"] = mixSessionMap(session)
		return ChatResponse{
			ConversationID:      interaction.ConversationID,
			GoalID:              interaction.GoalID,
			RunID:               interaction.RunID,
			Reply:               session.BlockingPoint,
			Workflow:            mixSessionEntryWorkflow,
			WorkflowData:        data,
			MixSession:          mixSessionMap(session),
			InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, mapValue(data["mix_observation"]))},
			GoalStatus:          string(agentruntime.StatusWaitingContinue),
			CurrentStep:         session.State,
		}
	}
	last := session.JournalRefs[len(session.JournalRefs)-1]
	observation := mapValue(data["mix_observation"])
	turns := mixTuneTurnsFromObservation(observation)
	rollbackBatch := mixLatestTuneBatch(turns)
	rollbackTurn := map[string]any{}
	if len(rollbackBatch) > 0 {
		rollbackTurn = rollbackBatch[0]
	} else if len(turns) > 0 {
		rollbackTurn = turns[len(turns)-1]
	}
	rollbackActionIDs := mixTuneBatchActionIDs(rollbackBatch)
	if len(rollbackActionIDs) == 0 && last != "" {
		rollbackActionIDs = []string{last}
	}
	rollbackTrackID := firstNonEmpty(cleanContextText(rollbackTurn["track_id"]), session.TargetRef.ID)
	rollbackDB, hasRollbackDB := mixOptionalFloat(rollbackTurn, "before_db")
	rollbackCount := len(rollbackBatch)
	if rollbackCount == 0 && len(turns) > 0 {
		rollbackCount = 1
	}
	var resp harness.InvokeResponse
	var err error
	if rollbackTrackID != "" && hasRollbackDB {
		resp, err = s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool: "track.volume",
			Args: map[string]any{
				"track_id": rollbackTrackID,
				"db":       rollbackDB,
			},
			Context:   interaction.RequestContext,
			Source:    "mixboard_auto_tune_rollback",
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
	} else {
		for i := len(rollbackActionIDs) - 1; i >= 0; i-- {
			resp, err = s.harness.Invoke(ctx, harness.InvokeRequest{
				Tool: "agent.rollback_action",
				Args: map[string]any{
					"target_action_id": rollbackActionIDs[i],
				},
				Context:   interaction.RequestContext,
				Source:    "mixboard_auto_tune_rollback",
				Confirmed: true,
				RunID:     interaction.RunID,
				GoalID:    interaction.GoalID,
			})
			if err != nil || resp.Status != "ok" {
				break
			}
		}
	}
	rollbackOK := err == nil && resp.Status == "ok"
	if rollbackOK {
		session.BlockingPoint = ""
		trimCount := len(rollbackActionIDs)
		if trimCount == 0 {
			trimCount = rollbackCount
		}
		if trimCount > len(session.JournalRefs) {
			trimCount = len(session.JournalRefs)
		}
		session.JournalRefs = session.JournalRefs[:len(session.JournalRefs)-trimCount]
		session.RoundCount -= trimCount
		if session.RoundCount < 0 {
			session.RoundCount = 0
		}
	}
	session.State = mixStateTuningPaused
	session.InteractionPhase = mixInteractionWaitingPlannerReview
	session.MixBoardVisibility = mixBoardVisibilityExecution
	session.ExecutorType = firstNonEmpty(session.ExecutorType, mixFallbackExecutorType)
	session.ExecutorVersion = firstNonEmpty(session.ExecutorVersion, mixFallbackExecutorVersion)
	session.ReviewStatus = mixReviewRolledBack
	session.StopReason = mixStopReasonRollbackRequested
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	writeMixBoardDiag("rollback_turn", map[string]any{
		"mix_session_id":  session.MixSessionID,
		"tuning_batch_id": cleanContextText(rollbackTurn["tuning_batch_id"]),
		"target_id":       session.TargetRef.ID,
		"track_id":        rollbackTrackID,
		"before_db":       rollbackDB,
		"has_before_db":   hasRollbackDB,
		"rollback_count":  rollbackCount,
		"action_ids":      rollbackActionIDs,
		"tool_status":     resp.Status,
		"tool_error":      firstNonEmpty(resp.Error, fmt.Sprint(err)),
		"agent_action":    last,
	})
	if err != nil || resp.Status != "ok" {
		session.BlockingPoint = firstNonEmpty(resp.Error, fmt.Sprint(err), "撤回上一轮失败")
	}
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	board := mapValue(observation["mixboard"])
	if rollbackTrackID != "" && hasRollbackDB && session.BlockingPoint == "" {
		board["current_action"] = fmt.Sprintf("已将 %s 音量恢复到 %.2f dB。", rollbackTrackID, rollbackDB)
	} else {
		board["current_action"] = "已请求撤回上一轮自动调参。"
	}
	board["current_judgement"] = "上一轮推子调节已撤回，等待重新试听或继续调控。"
	board["next_step"] = "重新试听当前状态；如仍需调整，可补充听感干预后再开始调控。"
	if len(turns) > 0 {
		remainingCount := len(turns) - rollbackCount
		if remainingCount < 0 || !rollbackOK {
			remainingCount = len(turns)
		}
		remainingTurns := turns[:remainingCount]
		board["auto_tune_turns"] = remainingTurns
		board["recent_auto_tune_turns"] = recentMixTuneTurns(remainingTurns, 3)
	}
	board["review_status"] = session.ReviewStatus
	board["latest_tick_status"] = session.ReviewStatus
	board["rollback_result"] = map[string]any{
		"status":      firstNonEmpty(resp.Status, "error"),
		"action_ids":  rollbackActionIDs,
		"error":       firstNonEmpty(resp.Error, fmt.Sprint(err)),
		"rolled_back": rollbackOK,
	}
	board["executor_type"] = session.ExecutorType
	board["executor_version"] = session.ExecutorVersion
	board["session_state"] = session.State
	board["stop_reason"] = session.StopReason
	board["round_count"] = session.RoundCount
	board["max_rounds"] = session.MaxRounds
	board["rollback_available"] = len(session.JournalRefs) > 0
	observation["mixboard"] = board
	updateMixBoardRuntimeState(observation, session, nil)
	data["mix_observation"] = observation
	reply := "已撤回上一轮自动调参。"
	if rollbackTrackID != "" && hasRollbackDB && session.BlockingPoint == "" {
		reply = fmt.Sprintf("已将上一轮音量调节恢复到 %.2f dB。", rollbackDB)
	}
	if session.BlockingPoint != "" {
		reply = session.BlockingPoint
	}
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observation)},
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
	}
}

func mixObservationOverridesFromRevision(payload map[string]any, target MixTargetRef, requestContext map[string]any) map[string]any {
	out := map[string]any{}
	mixObject := mapValue(payload["mix_object"])
	if len(mixObject) > 0 {
		kind := firstNonEmpty(cleanContextText(mixObject["kind"]), cleanContextText(mixObject["mode"]), target.Kind)
		id := firstNonEmpty(cleanContextText(mixObject["id"]), target.ID)
		label := firstNonEmpty(cleanContextText(mixObject["label"]), target.Label, id)
		if id != "" || label != "" {
			out["mix_objects"] = []map[string]any{{
				"mode":         firstNonEmpty(cleanContextText(mixObject["mode"]), "user_revision"),
				"kind":         kind,
				"id":           id,
				"label":        label,
				"effect_scope": mixEffectScopeForKind(kind),
				"source":       firstNonEmpty(cleanContextText(mixObject["source"]), "mixboard_revision"),
			}}
		}
	}
	listenChoice := cleanContextText(payload["listen_scope_choice"])
	if listenChoice != "" {
		out["listen_scope_choice"] = listenChoice
		switch listenChoice {
		case "full_song":
			out["listen_scope"] = map[string]any{
				"time":   map[string]any{"mode": "full_song", "source": "mixboard_card"},
				"source": map[string]any{"mode": "full_mix"},
			}
		case "time_selection":
			timeScope := map[string]any{"mode": "time_selection", "source": "mixboard_card"}
			if ts := mapValue(requestContext["time_selection"]); len(ts) > 0 {
				if start := mixFloatNumber(ts["start_seconds"]); start > 0 {
					timeScope["start_seconds"] = start
				}
				if end := mixFloatNumber(ts["end_seconds"]); end > 0 {
					timeScope["end_seconds"] = end
				}
				if locked, ok := ts["locked"]; ok {
					timeScope["locked"] = locked
				}
			}
			out["listen_scope"] = map[string]any{
				"time":   timeScope,
				"source": map[string]any{"mode": "full_mix"},
			}
		case "current_clip":
			out["listen_scope"] = map[string]any{
				"time":   map[string]any{"mode": "current_clip", "source": "mixboard_card"},
				"source": map[string]any{"mode": "selected_clips"},
			}
		case "mix_object":
			out["listen_scope"] = map[string]any{
				"time":   map[string]any{"mode": "mix_object", "source": "mixboard_card"},
				"source": map[string]any{"mode": "mix_objects"},
			}
		case "mix_objects_only":
			out["listen_scope"] = map[string]any{
				"time":   map[string]any{"mode": "mix_object", "source": "mixboard_card"},
				"source": map[string]any{"mode": "mix_objects"},
			}
		}
	}
	return out
}

func mixEffectScopeForKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "clip", "current_clip":
		return "clip_scope"
	case "bus":
		return "bus_rack"
	default:
		return "track_rack"
	}
}

func (s *Server) requestInitialMixObservation(ctx context.Context, interaction PendingInteraction, session MixSession) (map[string]any, error) {
	return s.requestMixObservationRound(ctx, interaction, session, 1, nil)
}

func (s *Server) requestMixObservationRound(ctx context.Context, interaction PendingInteraction, session MixSession, round int, overrides map[string]any) (map[string]any, error) {
	if s == nil || s.harness == nil {
		return nil, fmt.Errorf("harness is nil")
	}
	if round <= 0 {
		round = 1
	}
	observationArgs := map[string]any{
		"mix_session_id": session.MixSessionID,
		"round":          round,
		"goal_text":      session.GoalText,
		"target_ref":     mixTargetMap(session.TargetRef),
	}
	for _, key := range []string{
		"time_selection",
		"listen_time_start_seconds",
		"listen_time_end_seconds",
		"listen_time_source",
		"selected_track_id",
		"selected_track_name",
		"selected_clip_id",
		"selected_clip_ids",
		"selected_clip_track_id",
	} {
		if value, ok := interaction.RequestContext[key]; ok {
			observationArgs[key] = value
		}
	}
	for key, value := range overrides {
		observationArgs[key] = value
	}
	resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.request_observation",
		Args: observationArgs,
		Context: mergeContext(interaction.RequestContext, map[string]any{
			"conversation_id": interaction.ConversationID,
			"goal_id":         interaction.GoalID,
			"run_id":          interaction.RunID,
		}),
		Source:    "mix_session",
		Confirmed: true,
		RunID:     interaction.RunID,
		GoalID:    interaction.GoalID,
	})
	if err != nil {
		return resp.Result, err
	}
	if resp.Status != "ok" {
		return resp.Result, fmt.Errorf("%s", firstNonEmpty(resp.Error, "mix observation failed"))
	}
	return resp.Result, nil
}

func mixStateFromObservationResult(result map[string]any, err error) string {
	if err != nil {
		return mixStateObservationUnavailable
	}
	status := strings.ToLower(strings.TrimSpace(cleanContextText(result["status"])))
	switch status {
	case "ready", "ok":
		return mixStateObservationReady
	case "partial":
		return mixStateObservationPartial
	default:
		return mixStateObservationUnavailable
	}
}

func mixObservationBlockingPoint(result map[string]any, err error) string {
	if err != nil {
		return err.Error()
	}
	status := strings.ToLower(strings.TrimSpace(cleanContextText(result["status"])))
	if status == "partial" {
		obs := mapValue(result["observation"])
		caps := mapValue(obs["source_capabilities"])
		waveform := strings.ToLower(cleanContextText(caps["waveform_envelope"]))
		spectrum := strings.ToLower(cleanContextText(caps["spectrogram_tiles"]))
		if waveform == "ready" {
			return ""
		}
		if waveform == "requested" {
			return "audio_feature_request_pending"
		}
		if waveform == "blocked" {
			return "audio_feature_request_blocked"
		}
		if waveform != "ready" && spectrum == "ready" {
			return "spectrum ready; waveform envelope observation pending"
		}
		return "lightweight acoustic feature reader partially connected; MixBoard observation is partial"
	}
	if status == "unavailable" {
		return "audio feature data unavailable"
	}
	return ""
}

func (s *Server) attachGoalControlSurface(ctx context.Context, interaction PendingInteraction, session MixSession, observation map[string]any) map[string]any {
	if len(observation) == 0 || session.MixSessionID == "" {
		return nil
	}
	roles := mixcontrolsurface.RequiredRoleTypes(session.GoalText)
	candidates, warnings := s.mixControlSurfacePluginCandidates(ctx, interaction, roles)
	profiles, profileWarnings := s.mixControlSurfaceProjectProfiles(ctx, interaction)
	warnings = append(warnings, profileWarnings...)
	rackPlugins := []map[string]any{}
	if s != nil && s.harness != nil {
		rackPlugins = mixControlSurfaceRackPlugins(s.harness.UserStateSummary(ctx))
	}
	rackPlugins = append(rackPlugins, mixControlSurfaceRackPlugins(interaction.RequestContext)...)
	rackPlugins = append(rackPlugins, mixControlSurfaceRackPlugins(observation)...)
	surface := mixcontrolsurface.Build(mixcontrolsurface.Request{
		MixSessionID:     session.MixSessionID,
		Mode:             session.Mode,
		Goal:             session.GoalText,
		Target:           mixSurfaceTarget(session.TargetRef),
		Observation:      observation,
		PluginCandidates: candidates,
		ProjectProfiles:  profiles,
		RackPlugins:      rackPlugins,
		Warnings:         warnings,
	})
	applyGoalControlSurface(observation, surface)
	return surface
}

func (s *Server) mixControlSurfacePluginCandidates(ctx context.Context, interaction PendingInteraction, roles []string) ([]map[string]any, []string) {
	if s == nil || s.harness == nil {
		return nil, []string{"harness unavailable for plugin semantic search"}
	}
	out := []map[string]any{}
	warnings := []string{}
	if len(roles) == 0 {
		roles = []string{"eq", "dynamics"}
	}
	for _, role := range roles {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool: "plugin.semantic_search",
			Args: map[string]any{
				"query": role,
				"type":  role,
				"limit": 6,
			},
			Context: mergeContext(interaction.RequestContext, map[string]any{
				"conversation_id": interaction.ConversationID,
				"goal_id":         interaction.GoalID,
				"run_id":          interaction.RunID,
			}),
			Source:    "mix_goal_control_surface",
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
		if err != nil || resp.Status != "ok" {
			warnings = append(warnings, fmt.Sprintf("plugin semantic search for %s unavailable: %s", role, firstNonEmpty(resp.Error, fmt.Sprint(err))))
			continue
		}
		out = append(out, mapRowsValue(resp.Result["plugins"])...)
		out = append(out, mapRowsValue(resp.Result["entries"])...)
	}
	return out, warnings
}

func (s *Server) mixControlSurfaceProjectProfiles(ctx context.Context, interaction PendingInteraction) ([]map[string]any, []string) {
	warnings := []string{}
	if s != nil && s.harness != nil {
		resp, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Tool:      "plugin_grabber.get_project_profiles",
			Args:      map[string]any{},
			Context:   interaction.RequestContext,
			Source:    "mix_goal_control_surface",
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
		if err == nil && resp.Status == "ok" {
			profiles := mapRowsValue(resp.Result["plugin_grabber_profiles"])
			if len(profiles) == 0 {
				profiles = mapRowsValue(resp.Result["profiles"])
			}
			if len(profiles) > 0 {
				return profiles, nil
			}
		} else {
			warnings = append(warnings, "project plugin profiles unavailable from runtime: "+firstNonEmpty(resp.Error, fmt.Sprint(err)))
		}
	}
	localProfiles, localWarnings := mixControlSurfaceLocalProfiles()
	warnings = append(warnings, localWarnings...)
	return localProfiles, warnings
}

func mixControlSurfaceLocalProfiles() ([]map[string]any, []string) {
	root := mixControlSurfaceProfileRoot()
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, []string{"local plugin profile directory unavailable: " + err.Error()}
	}
	out := []map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			continue
		}
		var row map[string]any
		if json.Unmarshal(data, &row) == nil && len(row) > 0 {
			out = append(out, row)
		}
	}
	return out, nil
}

func mixControlSurfaceProfileRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; dir != ""; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "VitApp", "Workspace", "plugin_grabber_profiles")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return ""
}

func mixControlSurfaceRackPlugins(sources ...map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, source := range sources {
		collectRackPlugins(&out, source)
	}
	return dedupeRackPlugins(out)
}

func collectRackPlugins(out *[]map[string]any, source map[string]any) {
	if len(source) == 0 {
		return
	}
	for _, row := range mapRowsValue(source["plugins"]) {
		*out = append(*out, row)
	}
	for _, key := range []string{"plugin_rack", "rack", "selected_plugin"} {
		rack := mapValue(source[key])
		if len(rack) == 0 {
			continue
		}
		for _, nested := range []string{"plugins", "items", "chain", "rack"} {
			for _, row := range mapRowsValue(rack[nested]) {
				*out = append(*out, row)
			}
		}
		if cleanContextText(rack["plugin_id"]) != "" || cleanContextText(rack["plugin_name"]) != "" || cleanContextText(rack["name"]) != "" {
			*out = append(*out, rack)
		}
	}
	for _, track := range mapRowsValue(source["tracks"]) {
		trackID := firstNonEmpty(cleanContextText(track["track_id"]), cleanContextText(track["id"]))
		for _, row := range mapRowsValue(track["plugins"]) {
			if cleanContextText(row["track_id"]) == "" && trackID != "" {
				row["track_id"] = trackID
			}
			*out = append(*out, row)
		}
	}
}

func dedupeRackPlugins(rows []map[string]any) []map[string]any {
	out := []map[string]any{}
	seen := map[string]bool{}
	for _, row := range rows {
		key := strings.ToLower(strings.Join([]string{
			cleanContextText(row["track_id"]),
			firstNonEmpty(cleanContextText(row["plugin_id"]), cleanContextText(row["plugin_item_id"]), cleanContextText(row["id"])),
			firstNonEmpty(cleanContextText(row["plugin_path"]), cleanContextText(row["path"])),
			firstNonEmpty(cleanContextText(row["plugin_name"]), cleanContextText(row["name"])),
		}, "|"))
		if key == "|||" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, row)
	}
	return out
}

func mixSurfaceTarget(target MixTargetRef) mixcontrolsurface.Target {
	return mixcontrolsurface.Target{
		Kind:       target.Kind,
		ID:         target.ID,
		Label:      target.Label,
		Source:     target.Source,
		Confidence: target.Confidence,
	}
}

func applyGoalControlSurface(observation map[string]any, surface map[string]any) {
	if len(observation) == 0 || len(surface) == 0 {
		return
	}
	observation["goal_control_surface"] = surface
	board := mapValue(observation["mixboard"])
	board["goal_control_surface"] = surface
	board["control_surface_status"] = cleanContextText(surface["readiness"])
	board["control_surface_next_required_action"] = cleanContextText(surface["next_required_action"])
	if cleanContextText(surface["readiness"]) == mixcontrolsurface.ReadinessBlocked {
		blockers := contextStringSlice(surface["blockers"])
		if len(blockers) > 0 {
			board["current_action"] = "Goal Control Surface planning is blocked."
			board["next_step"] = blockers[0]
			board["open_blockers"] = blockers
		}
	}
	observation["mixboard"] = board
	contextPack := mapValue(observation["context_pack"])
	if len(contextPack) > 0 {
		contextPack["goal_control_surface"] = surface
		header := mapValue(contextPack["session_header"])
		header["goal_control_surface"] = surface
		contextPack["session_header"] = header
		observation["context_pack"] = contextPack
	}
}

func mixGoalControlSurfaceFromObservation(observation map[string]any) map[string]any {
	if len(observation) == 0 {
		return nil
	}
	if surface := mapValue(observation["goal_control_surface"]); len(surface) > 0 {
		return surface
	}
	if surface := mapValue(mapValue(observation["mixboard"])["goal_control_surface"]); len(surface) > 0 {
		return surface
	}
	if surface := mapValue(mapValue(observation["context_pack"])["goal_control_surface"]); len(surface) > 0 {
		return surface
	}
	return nil
}

func mixGoalControlSurfaceTuningBlocker(observation map[string]any) string {
	surface := mixGoalControlSurfaceFromObservation(observation)
	if len(surface) == 0 || cleanContextText(surface["readiness"]) != mixcontrolsurface.ReadinessBlocked {
		return ""
	}
	blockers := contextStringSlice(surface["blockers"])
	if len(blockers) == 0 {
		return "Goal Control Surface is blocked; resolve plugin/profile readiness before tuning."
	}
	return "Goal Control Surface is blocked: " + strings.Join(blockers, "; ")
}

func buildMixTickPacket(session MixSession, observation map[string]any, userIntervention string) map[string]any {
	now := time.Now().Format(time.RFC3339Nano)
	fastProfile := mixFastModelProfile(session, nil)
	allowed := []map[string]any{}
	blocked := []map[string]any{}
	for _, macro := range mixMacroControlsFromPanel(mapValue(ensureMixControlPlan(observation, session)["macro_control_panel"])) {
		if enabled, ok := macro["enabled"].(bool); ok && !enabled {
			continue
		}
		controlName := cleanContextText(macro["control"])
		if controlName == "" {
			continue
		}
		allowed = append(allowed, map[string]any{
			"control_id":      "macro:" + cleanContextText(macro["macro_id"]),
			"control_name":    controlName,
			"control_kind":    "macro",
			"tool":            "track.volume",
			"role":            firstNonEmpty(cleanContextText(macro["role"]), mixBuiltInVolumeMacroRole),
			"track_id":        firstNonEmpty(cleanContextText(macro["track_id"]), session.TargetRef.ID),
			"unit":            cleanContextText(macro["unit"]),
			"safe_step":       firstNonEmpty(fmt.Sprint(macro["safe_step"]), fmt.Sprintf("%.2f", mixAutoTuneStepDB)),
			"confirmed":       true,
			"source":          "confirmed_macro",
			"raw_param_write": false,
		})
	}
	surface := mixGoalControlSurfaceFromObservation(observation)
	learningRequest := mixPluginLearningRequestFromSurface(surface)
	for _, row := range mapRowsValue(surface["selected_chain"]) {
		role := cleanContextText(row["role"])
		roleType := cleanContextText(row["type"])
		profileStatus := cleanContextText(row["profile_status"])
		instanceStatus := cleanContextText(row["instance_status"])
		instance := mapValue(row["instance"])
		trackID := firstNonEmpty(cleanContextText(instance["track_id"]), session.TargetRef.ID)
		pluginID := firstNonEmpty(cleanContextText(instance["plugin_id"]), cleanContextText(instance["plugin_item_id"]), cleanContextText(instance["id"]))
		pluginName := firstNonEmpty(cleanContextText(mapValue(row["selected_plugin"])["name"]), cleanContextText(instance["plugin_name"]), cleanContextText(instance["name"]))
		if profileStatus == mixcontrolsurface.ProfileMissing || profileStatus == mixcontrolsurface.ProfileStale {
			blocked = append(blocked, map[string]any{
				"role":           role,
				"type":           roleType,
				"plugin_name":    pluginName,
				"profile_status": profileStatus,
				"reason":         "plugin_grabber_profile_" + profileStatus,
				"next_action":    "learn_plugin_profile",
			})
			continue
		}
		if profileStatus != mixcontrolsurface.ProfileReady && profileStatus != mixcontrolsurface.ProfileNotRequired {
			blocked = append(blocked, map[string]any{
				"role":           role,
				"type":           roleType,
				"plugin_name":    pluginName,
				"profile_status": firstNonEmpty(profileStatus, "unknown"),
				"reason":         "profile_not_ready",
			})
			continue
		}
		if instanceStatus != mixcontrolsurface.InstanceExisting || trackID == "" || pluginID == "" {
			blocked = append(blocked, map[string]any{
				"role":            role,
				"type":            roleType,
				"plugin_name":     pluginName,
				"instance_status": firstNonEmpty(instanceStatus, mixcontrolsurface.InstanceUnknown),
				"reason":          "plugin_instance_not_ready",
				"next_action":     "confirm_control_surface",
			})
			continue
		}
		for _, control := range mapRowsValue(row["proposed_controls"]) {
			controlName := firstNonEmpty(cleanContextText(control["name"]), cleanContextText(control["control"]), cleanContextText(control["id"]))
			if controlName == "" {
				continue
			}
			controlID := "virtual:" + sanitizeMixID(pluginID) + ":" + sanitizeMixID(controlName)
			allowed = append(allowed, map[string]any{
				"control_id":      controlID,
				"control_name":    controlName,
				"control_kind":    "plugin_virtual_control",
				"tool":            "plugin_grabber.apply_control",
				"role":            role,
				"type":            roleType,
				"track_id":        trackID,
				"plugin_id":       pluginID,
				"plugin_name":     pluginName,
				"component_id":    cleanContextText(control["component_id"]),
				"inputs":          control["inputs"],
				"confirmed":       true,
				"source":          "goal_control_surface",
				"raw_param_write": false,
			})
		}
	}
	readiness := cleanContextText(surface["readiness"])
	status := "ready"
	switch {
	case len(learningRequest) > 0:
		status = mixInteractionLearningRequired
	case readiness == mixcontrolsurface.ReadinessBlocked:
		status = "blocked"
	case readiness == mixcontrolsurface.ReadinessNeedsConfirmation:
		status = "needs_confirmation"
	case len(allowed) == 0:
		status = "no_allowed_controls"
	}
	packet := map[string]any{
		"schema_version":        mixTickPacketSchemaVersion,
		"mix_session_id":        session.MixSessionID,
		"mode":                  firstNonEmpty(session.Mode, mixModeAuto),
		"planner_policy":        mixPlannerPolicy(session),
		"fast_model_profile":    fastProfile,
		"model_strategy":        mixFastModelStrategy(fastProfile),
		"goal_summary":          session.GoalText,
		"target_ref":            mixTargetMap(session.TargetRef),
		"observation_digest":    mixObservationDigest(observation),
		"allowed_controls":      allowed,
		"blocked_controls":      blocked,
		"recent_ticks":          recentMixTuneTurns(mixTuneTurnsFromObservation(observation), 3),
		"user_intervention":     userIntervention,
		"safety":                mixTickSafetyPolicy(session),
		"stop_conditions":       mixTickStopConditions(session),
		"rollback_policy":       mixTickRollbackPolicy(),
		"status":                status,
		"fast_execution_prompt": mixFastExecutionPrompt(session, userIntervention),
		"created_at":            now,
		"updated_at":            now,
	}
	if len(surface) > 0 {
		packet["control_surface_id"] = firstNonEmpty(cleanContextText(surface["control_surface_id"]), cleanContextText(surface["mix_session_id"]), session.ControlSurfaceID)
		packet["control_surface_status"] = readiness
	}
	if len(learningRequest) > 0 {
		packet["plugin_learning_request"] = learningRequest
	}
	return packet
}

func applyMixTickPacket(observation map[string]any, packet map[string]any) {
	if len(observation) == 0 || len(packet) == 0 {
		return
	}
	observation["mix_tick_packet"] = packet
	board := mapValue(observation["mixboard"])
	board["mix_tick_packet"] = packet
	board["mix_tick_packet_status"] = cleanContextText(packet["status"])
	if request := mapValue(packet["plugin_learning_request"]); len(request) > 0 {
		board["learning_required"] = true
		board["plugin_learning_request"] = request
	}
	observation["mixboard"] = board
	contextPack := mapValue(observation["context_pack"])
	if len(contextPack) > 0 {
		contextPack["mix_tick_packet"] = packet
		contextPack["fast_execution_prompt"] = packet["fast_execution_prompt"]
		observation["context_pack"] = contextPack
	}
}

func mixTickPacketFromObservation(observation map[string]any) map[string]any {
	if len(observation) == 0 {
		return nil
	}
	if packet := mapValue(observation["mix_tick_packet"]); len(packet) > 0 {
		return packet
	}
	if packet := mapValue(mapValue(observation["mixboard"])["mix_tick_packet"]); len(packet) > 0 {
		return packet
	}
	if packet := mapValue(mapValue(observation["context_pack"])["mix_tick_packet"]); len(packet) > 0 {
		return packet
	}
	return nil
}

func mixObservationDigest(observation map[string]any) map[string]any {
	obs := mapValue(observation["observation"])
	mixboard := mapValue(observation["mixboard"])
	sourceCaps := mapValue(obs["source_capabilities"])
	if len(sourceCaps) == 0 {
		sourceCaps = mapValue(mapValue(obs["mix_package"])["source_capabilities"])
	}
	return map[string]any{
		"status":             cleanContextText(observation["status"]),
		"observation_id":     firstNonEmpty(cleanContextText(obs["observation_id"]), cleanContextText(observation["observation_id"])),
		"current_judgement":  cleanContextText(mixboard["current_judgement"]),
		"current_action":     cleanContextText(mixboard["current_action"]),
		"waveform_status":    cleanContextText(sourceCaps["waveform_envelope"]),
		"spectrum_status":    cleanContextText(sourceCaps["spectrogram_tiles"]),
		"before_after_delta": cleanContextText(sourceCaps["before_after_delta"]),
	}
}

func mixTickSafetyPolicy(session MixSession) map[string]any {
	maxStep := mixAutoTuneStepDB
	if session.Mode == mixModeCo {
		maxStep = 0.5
	}
	return map[string]any{
		"max_macro_step_db":        maxStep,
		"max_plugin_control_step":  "small",
		"require_allowed_control":  true,
		"require_ready_profile":    true,
		"forbid_raw_param_write":   true,
		"forbid_plugin_chain_edit": true,
		"single_tick_only":         true,
	}
}

func mixTickStopConditions(session MixSession) []map[string]any {
	return []map[string]any{
		{"id": "goal_met", "description": "pause when the observation and review satisfy the goal"},
		{"id": "needs_planner_review", "description": "return to planner after each tick"},
		{"id": "missing_skill_or_profile", "description": "enter learning_required before execution"},
		{"id": "max_rounds", "limit": session.MaxRounds},
	}
}

func mixTickRollbackPolicy() map[string]any {
	return map[string]any{
		"rollback_action":     "rollback_last_mix_tick",
		"macro_before_db":     true,
		"plugin_action_undo":  true,
		"keep_failed_records": true,
	}
}

func mixFastExecutionPrompt(session MixSession, userIntervention string) string {
	parts := []string{
		"Use MixTickPacket only.",
		"Choose exactly one allowed control and make one small reversible change.",
		"Do not choose plugins, edit chains, learn profiles, or write raw plugin params.",
		"Return MixTickDecision with schema_version " + mixTickDecisionSchemaVersion + ".",
		"Goal: " + session.GoalText,
	}
	if userIntervention != "" {
		parts = append(parts, "User intervention: "+userIntervention)
	}
	return strings.Join(parts, "\n")
}

func mixPluginLearningRequestFromObservation(observation map[string]any) map[string]any {
	if packet := mixTickPacketFromObservation(observation); len(packet) > 0 {
		if request := mapValue(packet["plugin_learning_request"]); len(request) > 0 {
			return request
		}
	}
	return mixPluginLearningRequestFromSurface(mixGoalControlSurfaceFromObservation(observation))
}

func mixPluginLearningRequestFromSurface(surface map[string]any) map[string]any {
	if len(surface) == 0 {
		return nil
	}
	for _, row := range mapRowsValue(surface["selected_chain"]) {
		profileStatus := cleanContextText(row["profile_status"])
		if profileStatus != mixcontrolsurface.ProfileMissing && profileStatus != mixcontrolsurface.ProfileStale {
			continue
		}
		plugin := mapValue(row["selected_plugin"])
		controls := []map[string]any{}
		for _, control := range mapRowsValue(row["proposed_controls"]) {
			controls = append(controls, map[string]any{
				"name":         firstNonEmpty(cleanContextText(control["name"]), cleanContextText(control["id"])),
				"component_id": cleanContextText(control["component_id"]),
				"role":         cleanContextText(row["role"]),
			})
		}
		return map[string]any{
			"plugin_name":          firstNonEmpty(cleanContextText(plugin["name"]), cleanContextText(row["plugin_name"]), "selected plugin"),
			"plugin_id":            firstNonEmpty(cleanContextText(plugin["id"]), cleanContextText(plugin["profile_id"])),
			"role":                 cleanContextText(row["role"]),
			"type":                 cleanContextText(row["type"]),
			"profile_status":       profileStatus,
			"reason":               "语义化混音执行前必须先完成 Plugin Grabber 学习。",
			"needed_controls":      controls,
			"next_required_action": "learn_plugin_profile",
		}
	}
	return nil
}

func mixTrackVolumeDB(state map[string]any, trackID string) float64 {
	for _, raw := range mapRowsValue(state["tracks"]) {
		if cleanContextText(raw["track_id"]) == trackID || cleanContextText(raw["id"]) == trackID {
			for _, key := range []string{"volume_db", "gain_db", "fader_db", "db"} {
				if value := mixFloatNumber(raw[key]); value != 0 {
					return value
				}
			}
		}
	}
	return 0
}

func mixTuningStepDB(goal string, payload map[string]any) float64 {
	if explicit := mixFloatNumber(payload["step_db"]); explicit != 0 {
		if explicit > mixAutoTuneStepDB {
			return mixAutoTuneStepDB
		}
		if explicit < -mixAutoTuneStepDB {
			return -mixAutoTuneStepDB
		}
		return explicit
	}
	fields := mapValue(payload["fields"])
	userText := strings.ToLower(strings.Join([]string{
		cleanContextText(payload["user_note"]),
		cleanContextText(payload["user_feedback"]),
		cleanContextText(payload["subjective_note"]),
		cleanContextText(fields["user_note"]),
		cleanContextText(fields["user_feedback"]),
		cleanContextText(fields["subjective_note"]),
	}, " "))
	if direction := mixTuningDirectionFromText(userText); direction != 0 {
		return direction * mixAutoTuneStepDB
	}
	text := strings.ToLower(strings.Join([]string{
		goal,
		cleanContextText(payload["goal_text"]),
	}, " "))
	if direction := mixTuningDirectionFromText(text); direction != 0 {
		return direction * mixAutoTuneStepDB
	}
	return -0.5
}

func mixTuningDirectionSource(goal string, payload map[string]any) string {
	if explicit := mixFloatNumber(payload["step_db"]); explicit != 0 {
		return "explicit_step_db"
	}
	fields := mapValue(payload["fields"])
	userText := strings.ToLower(strings.Join([]string{
		cleanContextText(payload["user_note"]),
		cleanContextText(payload["user_feedback"]),
		cleanContextText(payload["subjective_note"]),
		cleanContextText(fields["user_note"]),
		cleanContextText(fields["user_feedback"]),
		cleanContextText(fields["subjective_note"]),
	}, " "))
	if direction := mixTuningDirectionFromText(userText); direction != 0 {
		return "user_note"
	}
	text := strings.ToLower(strings.Join([]string{
		goal,
		cleanContextText(payload["goal_text"]),
	}, " "))
	if direction := mixTuningDirectionFromText(text); direction != 0 {
		return "goal_text"
	}
	return "fallback_default"
}

func mixTuningDirectionFromText(text string) float64 {
	switch {
	case strings.Contains(text, "提高") ||
		strings.Contains(text, "提升") ||
		strings.Contains(text, "增大") ||
		strings.Contains(text, "增加") ||
		strings.Contains(text, "加大") ||
		strings.Contains(text, "加音量") ||
		strings.Contains(text, "调大") ||
		strings.Contains(text, "推高") ||
		strings.Contains(text, "更响") ||
		strings.Contains(text, "大声") ||
		strings.Contains(text, "放大") ||
		strings.Contains(text, "响一点") ||
		strings.Contains(text, "louder") ||
		strings.Contains(text, "raise") ||
		strings.Contains(text, "increase") ||
		strings.Contains(text, "turn up") ||
		strings.Contains(text, "up"):
		return 1
	case strings.Contains(text, "降低") ||
		strings.Contains(text, "压低") ||
		strings.Contains(text, "调低") ||
		strings.Contains(text, "减小") ||
		strings.Contains(text, "减少") ||
		strings.Contains(text, "减音量") ||
		strings.Contains(text, "降低音量") ||
		strings.Contains(text, "小声") ||
		strings.Contains(text, "轻一点") ||
		strings.Contains(text, "余量") ||
		strings.Contains(text, "headroom") ||
		strings.Contains(text, "quieter") ||
		strings.Contains(text, "lower") ||
		strings.Contains(text, "decrease") ||
		strings.Contains(text, "turn down") ||
		strings.Contains(text, "down"):
		return -1
	default:
		return 0
	}
}

func mixTuningUserNote(payload, requestContext, data map[string]any, session MixSession) string {
	fields := mapValue(payload["fields"])
	return firstNonEmpty(
		cleanContextText(payload["user_note"]),
		cleanContextText(payload["user_feedback"]),
		cleanContextText(payload["subjective_note"]),
		cleanContextText(fields["user_note"]),
		cleanContextText(fields["user_feedback"]),
		cleanContextText(fields["subjective_note"]),
		cleanContextText(requestContext["user_note"]),
		cleanContextText(data["user_note"]),
		session.UserNote,
	)
}

func mixTuningDirectionLabel(stepDB float64) string {
	if stepDB > 0 {
		return "raise_volume"
	}
	if stepDB < 0 {
		return "lower_volume"
	}
	return "hold"
}

func mixTuningShouldHoldVolume(userNote string, stepDB float64) bool {
	note := strings.ToLower(strings.TrimSpace(userNote))
	if note == "" || stepDB == 0 {
		return false
	}
	if stepDB < 0 {
		return strings.Contains(note, "不要再降") ||
			strings.Contains(note, "别再降") ||
			strings.Contains(note, "别降") ||
			strings.Contains(note, "不用降") ||
			strings.Contains(note, "不要降低") ||
			strings.Contains(note, "别降低") ||
			strings.Contains(note, "停止降") ||
			strings.Contains(note, "不要再压") ||
			strings.Contains(note, "别压低") ||
			strings.Contains(note, "音量可以") ||
			strings.Contains(note, "音量够") ||
			strings.Contains(note, "太小") ||
			strings.Contains(note, "太低") ||
			strings.Contains(note, "听不见") ||
			strings.Contains(note, "do not lower") ||
			strings.Contains(note, "don't lower") ||
			strings.Contains(note, "too quiet")
	}
	return strings.Contains(note, "不要再提") ||
		strings.Contains(note, "别再提") ||
		strings.Contains(note, "别提高") ||
		strings.Contains(note, "不用提") ||
		strings.Contains(note, "不要提升") ||
		strings.Contains(note, "停止提") ||
		strings.Contains(note, "太大") ||
		strings.Contains(note, "太响") ||
		strings.Contains(note, "刺耳") ||
		strings.Contains(note, "do not raise") ||
		strings.Contains(note, "don't raise") ||
		strings.Contains(note, "too loud")
}

func mixTuningUserHoldJudgement(userNote string) string {
	if mixTuningMentionsLowMud(userNote) {
		return "已采纳用户听感：不再沿当前音量方向推进，优先转向低频浑浊 / 低频堆积的处理判断。"
	}
	return "已采纳用户听感：当前音量方向被暂停，下一轮以新的主观听感约束为准。"
}

func mixTuningUserHoldNextStep(userNote string) string {
	if mixTuningMentionsLowMud(userNote) {
		return "下一轮建议先复查低频能量与声像关系，再进入低切、低频动态或均衡候选策略。"
	}
	return "重新试听当前状态；如仍需调整，可修改干预意见后再开始小步调控。"
}

func mixTuningMentionsLowMud(userNote string) bool {
	note := strings.ToLower(strings.TrimSpace(userNote))
	return strings.Contains(note, "低频糊") ||
		strings.Contains(note, "低频浑") ||
		strings.Contains(note, "低频混") ||
		strings.Contains(note, "低频堆") ||
		strings.Contains(note, "低频太多") ||
		strings.Contains(note, "浑浊") ||
		strings.Contains(note, "糊") ||
		strings.Contains(note, "muddy") ||
		strings.Contains(note, "boomy")
}

func clampMixVolumeDB(value float64) float64 {
	if value > 12 {
		value = 12
	}
	if value < -60 {
		value = -60
	}
	return roundMixFloat(value)
}

func roundMixFloat(value float64) float64 {
	if value >= 0 {
		return float64(int(value*1000+0.5)) / 1000
	}
	return -float64(int(-value*1000+0.5)) / 1000
}

func mixBeforeAfterDeltaStatus(observation map[string]any) string {
	obs := mapValue(observation["observation"])
	mixPkg := mapValue(obs["mix_package"])
	caps := mapValue(mixPkg["source_capabilities"])
	if status := cleanContextText(caps["before_after_delta"]); status != "" {
		return status
	}
	metrics := mapValue(mixPkg["current_metrics"])
	delta := mapValue(metrics["before_after_delta"])
	return firstNonEmpty(cleanContextText(delta["status"]), "missing")
}

func mixTuneDecisionFromObservation(observation map[string]any, stepDB float64) string {
	status := mixBeforeAfterDeltaStatus(observation)
	if status != "ready" {
		return "continue"
	}
	obs := mapValue(observation["observation"])
	mixPkg := mapValue(obs["mix_package"])
	metrics := mapValue(mixPkg["current_metrics"])
	delta := mapValue(metrics["before_after_delta"])
	waveform := mapValue(delta["waveform"])
	rms := mapValue(waveform["rms_dbfs"])
	rmsDelta := mixFloatNumber(rms["delta"])
	if stepDB < 0 && rmsDelta <= -0.2 {
		return "keep"
	}
	if stepDB > 0 && rmsDelta >= 0.2 {
		return "keep"
	}
	if tags := contextStringSlice(delta["summary_tags"]); len(tags) == 1 && tags[0] == "no_significant_change" {
		return "stop"
	}
	return "continue"
}

func mixTuneCurrentAction(turns []map[string]any) string {
	if len(turns) == 0 {
		return "自动调参未执行。"
	}
	last := turns[len(turns)-1]
	return fmt.Sprintf("第 %d 轮：轨道音量 %.2f dB -> %.2f dB。", intNumber(last["turn"]), mixFloatNumber(last["before_db"]), mixFloatNumber(last["target_db"]))
}

func mixTuneJudgement(turns []map[string]any) string {
	if len(turns) == 0 {
		return "尚未产生调参结果。"
	}
	last := turns[len(turns)-1]
	if errText := cleanContextText(last["error"]); errText != "" {
		return "调参失败：" + errText
	}
	switch cleanContextText(last["decision"]) {
	case "keep":
		return "复查显示本轮音量变化符合方向，已保留。"
	case "stop":
		return "复查显示变化很小，已暂停继续调参。"
	default:
		return "已完成本轮小步调参，等待继续复查。"
	}
}

func mixTuneReviewStatus(decision, deltaStatus string) string {
	if deltaStatus != "ready" {
		return "waiting_review"
	}
	switch decision {
	case "keep":
		return "effective"
	case "stop":
		return "not_obvious"
	default:
		return "reviewed"
	}
}

func mixTuneLatestReviewStatus(turns []map[string]any) string {
	if len(turns) == 0 {
		return "waiting_review"
	}
	last := turns[len(turns)-1]
	if cleanContextText(last["error"]) != "" {
		return "failed"
	}
	return firstNonEmpty(cleanContextText(last["review_status"]), "waiting_review")
}

func mixTuneReply(session MixSession, turns []map[string]any, err error) string {
	if err != nil {
		return "自动调参已暂停：" + err.Error()
	}
	if len(turns) == 0 {
		return "自动调参未执行。"
	}
	return fmt.Sprintf("自动调参完成 %d 轮。%s", len(turns), mixTuneJudgement(turns))
}

func writeMixTuneActionRecord(observation map[string]any, turn map[string]any) {
	boardPath := cleanContextText(observation["board_path"])
	if boardPath == "" {
		return
	}
	sessionDir := filepath.Dir(boardPath)
	actionsDir := filepath.Join(sessionDir, "actions")
	if err := os.MkdirAll(actionsDir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("turn_%03d_%s.json", intNumber(turn["turn"]), cleanContextText(turn["agent_action_id"]))
	data, err := json.MarshalIndent(turn, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(actionsDir, name), append(data, '\n'), 0o644)
}

func (s *Server) applyMixMacroControl(ctx context.Context, interaction PendingInteraction, requestContext map[string]any, macro map[string]any, value float64) (harness.InvokeResponse, error) {
	control := cleanContextText(macro["control"])
	trackID := firstNonEmpty(cleanContextText(macro["track_id"]), cleanContextText(interaction.RequestContext["selected_track_id"]))
	if control != mixBuiltInVolumeMacroControl {
		err := fmt.Errorf("mix macro %s uses unsupported control %q", cleanContextText(macro["macro_id"]), control)
		return harness.InvokeResponse{Status: "error", Error: err.Error()}, err
	}
	if trackID == "" {
		err := fmt.Errorf("mix macro %s has no track_id", cleanContextText(macro["macro_id"]))
		return harness.InvokeResponse{Status: "error", Error: err.Error()}, err
	}
	return s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "track.volume",
		Args: map[string]any{
			"track_id": trackID,
			"db":       value,
		},
		Context:   requestContext,
		Source:    "mixboard_macro_tune",
		Confirmed: true,
		RunID:     interaction.RunID,
		GoalID:    interaction.GoalID,
	})
}

func mixVolumeMacroFromObservation(observation map[string]any, session MixSession) map[string]any {
	plan := ensureMixControlPlan(observation, session)
	panel := mapValue(plan["macro_control_panel"])
	for _, macro := range mixMacroControlsFromPanel(panel) {
		if cleanContextText(macro["control"]) == mixBuiltInVolumeMacroControl || cleanContextText(macro["role"]) == mixBuiltInVolumeMacroRole {
			return macro
		}
	}
	return mixBuiltInVolumeMacro(session, -6)
}

func mixMacroWithValue(macro map[string]any, value float64) map[string]any {
	out := copyStringAnyMap(macro)
	out["value"] = roundMixFloat(value)
	out["updated_at"] = time.Now().Format(time.RFC3339Nano)
	return out
}

func mixMacroUpsertExecutions(macros ...map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(macros))
	for _, macro := range macros {
		macroID := cleanContextText(macro["macro_id"])
		if macroID == "" {
			continue
		}
		out = append(out, map[string]any{
			"status":       "ok",
			"command_name": "control_add_macro",
			"tool":         "control.add_macro",
			"result": map[string]any{
				"status":        "ok",
				"ui_action":     "rack_macro_upserted",
				"kind":          "rack_macro_upserted",
				"macro_id":      macroID,
				"macro":         macro,
				"binding_count": len(mapRowsValue(macro["bindings"])),
			},
		})
	}
	return out
}

func mixMacroValueExecution(macro map[string]any, value float64, resp harness.InvokeResponse) []map[string]any {
	macroID := cleanContextText(macro["macro_id"])
	if macroID == "" || resp.Status != "ok" {
		return nil
	}
	nextMacro := mixMacroWithValue(macro, value)
	return []map[string]any{{
		"status":       "ok",
		"command_name": "control_set_macro_values",
		"tool":         "control.set_macro_values",
		"result": map[string]any{
			"status":        "ok",
			"ui_action":     "rack_macro_value_changed",
			"kind":          "rack_macro_value_changed",
			"macro_id":      macroID,
			"value":         roundMixFloat(value),
			"commit":        true,
			"macro":         nextMacro,
			"applied_count": 1,
			"applied_parameters": []map[string]any{{
				"track_id":     cleanContextText(nextMacro["track_id"]),
				"control":      cleanContextText(nextMacro["control"]),
				"param_id":     "track.volume",
				"param_name":   "轨道音量",
				"target_value": roundMixFloat(value),
				"unit":         "dB",
			}},
		},
	}}
}

func setMixControlMacroValue(observation map[string]any, macroID string, value float64) {
	if len(observation) == 0 || strings.TrimSpace(macroID) == "" {
		return
	}
	plan := mapValue(observation["mix_control_plan"])
	if len(plan) == 0 {
		plan = mapValue(mapValue(observation["mixboard"])["mix_control_plan"])
	}
	if len(plan) == 0 {
		return
	}
	panel := mapValue(plan["macro_control_panel"])
	controls := mixMacroControlsFromPanel(panel)
	for _, control := range controls {
		if cleanContextText(control["macro_id"]) == macroID {
			control["value"] = roundMixFloat(value)
			control["updated_at"] = time.Now().Format(time.RFC3339Nano)
		}
	}
	panel["controls"] = controls
	plan["macro_control_panel"] = panel
	observation["mix_control_plan"] = plan
	board := mapValue(observation["mixboard"])
	board["mix_control_plan"] = plan
	board["macro_control_panel"] = panel
	board["macro_controls"] = controls
	observation["mixboard"] = board
}

func updateMixBoardRuntimeState(observation map[string]any, session MixSession, turns []map[string]any) {
	if len(observation) == 0 {
		return
	}
	now := time.Now().Format(time.RFC3339Nano)
	controlPlan := ensureMixControlPlan(observation, session)
	executorType := firstNonEmpty(session.ExecutorType, mixFallbackExecutorType)
	executorVersion := firstNonEmpty(session.ExecutorVersion, mixFallbackExecutorVersion)
	reviewStatus := firstNonEmpty(session.ReviewStatus, mixTuneLatestReviewStatus(turns))
	runtime := map[string]any{
		"session_state":       session.State,
		"interaction_phase":   mixInteractionPhase(session),
		"mixboard_visibility": mixBoardVisibility(session),
		"planner_policy":      mixPlannerPolicy(session),
		"fast_model_profile":  session.FastModelProfile,
		"executor_type":       executorType,
		"executor_version":    executorVersion,
		"review_status":       reviewStatus,
		"stop_reason":         session.StopReason,
		"round_count":         session.RoundCount,
		"max_rounds":          session.MaxRounds,
		"rollback_available":  len(session.JournalRefs) > 0,
		"updated_at":          now,
	}
	if session.UserNote != "" {
		runtime["user_note"] = session.UserNote
	}
	if session.BlockingPoint != "" {
		runtime["blocking_point"] = session.BlockingPoint
	}
	if len(turns) > 0 {
		runtime["auto_tune_turns"] = turns
		runtime["recent_auto_tune_turns"] = recentMixTuneTurns(turns, 3)
		runtime["last_action_summary"] = mixTuneCurrentAction(turns)
	}
	if len(controlPlan) > 0 {
		runtime["mix_control_plan"] = controlPlan
		if panel := mapValue(controlPlan["macro_control_panel"]); len(panel) > 0 {
			runtime["macro_control_panel"] = panel
			runtime["macro_controls"] = mixMacroControlsFromPanel(panel)
		}
	}
	if surface := mixGoalControlSurfaceFromObservation(observation); len(surface) > 0 {
		runtime["goal_control_surface"] = surface
		runtime["control_surface_status"] = cleanContextText(surface["readiness"])
		runtime["control_surface_next_required_action"] = cleanContextText(surface["next_required_action"])
	}
	if packet := mixTickPacketFromObservation(observation); len(packet) > 0 {
		runtime["mix_tick_packet"] = packet
		runtime["mix_tick_packet_status"] = cleanContextText(packet["status"])
		if latest := mapValue(packet["latest_tick"]); len(latest) > 0 {
			runtime["latest_mix_tick"] = latest
		}
	}
	if request := mixPluginLearningRequestFromObservation(observation); len(request) > 0 {
		runtime["learning_required"] = true
		runtime["plugin_learning_request"] = request
	}
	if board := mapValue(observation["mixboard"]); len(board) > 0 {
		mergeMixRuntimeFields(board, runtime)
		observation["mixboard"] = board
	}
	if boardPath := cleanContextText(observation["board_path"]); boardPath != "" {
		updateJSONFileMap(boardPath, func(row map[string]any) {
			mergeMixRuntimeFields(row, runtime)
		})
	}
	if contextPath := cleanContextText(observation["context_pack_path"]); contextPath != "" {
		updateJSONFileMap(contextPath, func(row map[string]any) {
			header := mapValue(row["session_header"])
			mergeMixRuntimeFields(header, runtime)
			header["mix_session"] = mixSessionMap(session)
			row["session_header"] = header
			autoTune := mapValue(row["auto_tune"])
			mergeMixRuntimeFields(autoTune, runtime)
			row["auto_tune"] = autoTune
			if surface := mapValue(runtime["goal_control_surface"]); len(surface) > 0 {
				row["goal_control_surface"] = surface
			}
			row["generated_at"] = now
		})
	}
}

func mergeMixRuntimeFields(row map[string]any, fields map[string]any) {
	for key, value := range fields {
		if key == "" || value == nil {
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			continue
		}
		row[key] = value
	}
}

func ensureMixControlPlan(observation map[string]any, session MixSession) map[string]any {
	if len(observation) == 0 || strings.TrimSpace(session.TargetRef.ID) == "" {
		return nil
	}
	board := mapValue(observation["mixboard"])
	existing := mapValue(board["mix_control_plan"])
	if len(existing) == 0 {
		existing = mapValue(observation["mix_control_plan"])
	}
	plan := mixDefaultControlPlan(session, existing)
	board["mix_control_plan"] = plan
	panel := mapValue(plan["macro_control_panel"])
	if len(panel) > 0 {
		board["macro_control_panel"] = panel
		board["macro_controls"] = mixMacroControlsFromPanel(panel)
	}
	observation["mixboard"] = board
	observation["mix_control_plan"] = plan
	if contextPack := mapValue(observation["context_pack"]); len(contextPack) > 0 {
		contextPack["mix_control_plan"] = plan
		contextPack["macro_control_panel"] = panel
		observation["context_pack"] = contextPack
	}
	return plan
}

func mixDefaultControlPlan(session MixSession, existing map[string]any) map[string]any {
	now := time.Now().Format(time.RFC3339Nano)
	volumeMacro := mixBuiltInVolumeMacro(session, -6)
	if controls := mixMacroControlsFromPanel(mapValue(existing["macro_control_panel"])); len(controls) > 0 {
		for _, control := range controls {
			if cleanContextText(control["control"]) == mixBuiltInVolumeMacroControl || cleanContextText(control["role"]) == mixBuiltInVolumeMacroRole {
				volumeMacro = mergeMacroDefaults(volumeMacro, control)
				break
			}
		}
	}
	panel := map[string]any{
		"schema_version": mixMacroControlPanelSchemaVersion,
		"status":         "ready",
		"source":         "mixboard_control_plan",
		"controls":       []map[string]any{volumeMacro},
		"updated_at":     now,
	}
	plan := map[string]any{
		"schema_version": mixControlPlanSchemaVersion,
		"mix_session_id": session.MixSessionID,
		"target": map[string]any{
			"kind":       session.TargetRef.Kind,
			"id":         session.TargetRef.ID,
			"label":      firstNonEmpty(session.TargetRef.Label, session.TargetRef.ID),
			"source":     session.TargetRef.Source,
			"confidence": session.TargetRef.Confidence,
		},
		"goal": session.GoalText,
		"effect_role_plan": []map[string]any{{
			"role":     mixBuiltInVolumeMacroRole,
			"type":     "built_in_track_control",
			"purpose":  "建立可审计的轨道电平宏控制，作为 MixBoard 快速调参循环的第一条控制面。",
			"priority": 1,
			"required": true,
			"status":   "ready",
		}},
		"plugin_selection_plan": []map[string]any{{
			"role":              mixBuiltInVolumeMacroRole,
			"required_type":     "track_gain",
			"selected_plugin":   "built_in_track_volume",
			"selection_reason":  "轨道音量是内置可撤回控制，不需要插件 skill；用于验证宏控制执行底座。",
			"skill_status":      "not_required",
			"candidate_plugins": []map[string]any{},
			"status":            "ready",
		}},
		"chain_instance_plan": []map[string]any{{
			"slot":        0,
			"role":        mixBuiltInVolumeMacroRole,
			"plugin":      "built_in_track_volume",
			"instance_id": "track:" + session.TargetRef.ID,
			"status":      "ready",
			"skill":       "not_required",
		}},
		"macro_control_panel": panel,
		"readiness": map[string]any{
			"chain_ready":          true,
			"skills_ready":         true,
			"macro_panel_ready":    true,
			"can_start_fast_loop":  true,
			"blocking_point":       "",
			"next_required_action": "fast_macro_tuning",
		},
		"updated_at": now,
	}
	if createdAt := cleanContextText(existing["created_at"]); createdAt != "" {
		plan["created_at"] = createdAt
	} else {
		plan["created_at"] = now
	}
	return plan
}

func mixBuiltInVolumeMacro(session MixSession, value float64) map[string]any {
	macroID := "mix_macro_" + sanitizeMixID(firstNonEmpty(session.MixSessionID, session.TargetRef.ID, "session")) + "_track_volume"
	return map[string]any{
		"macro_id":     macroID,
		"id":           macroID,
		"name":         "轨道电平",
		"label":        "轨道电平",
		"role":         mixBuiltInVolumeMacroRole,
		"control":      mixBuiltInVolumeMacroControl,
		"track_id":     session.TargetRef.ID,
		"control_type": "slider",
		"value":        value,
		"min":          -60.0,
		"max":          12.0,
		"unit":         "dB",
		"safe_step":    mixAutoTuneStepDB,
		"source":       "mixboard_control_plan",
		"status":       "ready",
		"enabled":      true,
		"bindings": []map[string]any{{
			"binding_id": "binding_" + macroID + "_track_volume",
			"control":    mixBuiltInVolumeMacroControl,
			"track_id":   session.TargetRef.ID,
			"param_id":   "track.volume",
			"param_name": "轨道音量",
			"target_min": -60.0,
			"target_max": 12.0,
			"unit":       "dB",
			"enabled":    true,
		}},
		"binding_count": 1,
	}
}

func mergeMacroDefaults(base, override map[string]any) map[string]any {
	out := copyStringAnyMap(base)
	for key, value := range override {
		if key == "bindings" && len(mapRowsValue(value)) == 0 {
			continue
		}
		out[key] = value
	}
	if cleanContextText(out["macro_id"]) == "" {
		out["macro_id"] = base["macro_id"]
		out["id"] = base["id"]
	}
	if len(mapRowsValue(out["bindings"])) == 0 {
		out["bindings"] = base["bindings"]
		out["binding_count"] = base["binding_count"]
	}
	return out
}

func mixMacroControlsFromPanel(panel map[string]any) []map[string]any {
	rows := mapRowsValue(panel["controls"])
	if len(rows) == 0 {
		rows = mapRowsValue(panel["macro_controls"])
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if cleanContextText(row["macro_id"]) == "" && cleanContextText(row["id"]) != "" {
			row["macro_id"] = row["id"]
		}
		if cleanContextText(row["id"]) == "" && cleanContextText(row["macro_id"]) != "" {
			row["id"] = row["macro_id"]
		}
		if cleanContextText(row["macro_id"]) != "" {
			out = append(out, row)
		}
	}
	return out
}

func sanitizeMixID(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "mix"
	}
	return out
}

func updateJSONFileMap(path string, mutate func(map[string]any)) {
	path = strings.TrimSpace(path)
	if path == "" || mutate == nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		return
	}
	mutate(row)
	next, err := json.MarshalIndent(row, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(next, '\n'), 0o644)
}

func recentMixTuneTurns(turns []map[string]any, limit int) []map[string]any {
	if len(turns) == 0 || limit <= 0 {
		return nil
	}
	start := len(turns) - limit
	if start < 0 {
		start = 0
	}
	out := make([]map[string]any, 0, len(turns)-start)
	for _, turn := range turns[start:] {
		out = append(out, copyStringAnyMap(turn))
	}
	return out
}

func mixPreparationRowsForObservation(target MixTargetRef, state, blocker string) []map[string]any {
	rows := mixPreparationRows(target)
	observationStatus := "ready"
	if state == mixStateObservationPartial {
		observationStatus = "partial"
	} else if state == mixStateObservationUnavailable || strings.TrimSpace(blocker) != "" {
		observationStatus = "blocked"
	}
	for i := range rows {
		if rows[i]["id"] == "observation" {
			rows[i]["status"] = observationStatus
			if strings.TrimSpace(blocker) != "" {
				rows[i]["reason"] = blocker
			} else {
				delete(rows[i], "reason")
			}
		}
	}
	return rows
}

func mixObservationReply(session MixSession, err error) string {
	if err != nil {
		return fmt.Sprintf("%s MixBoard observation unavailable: %s", mixModeLabel(session.Mode), err.Error())
	}
	if session.State == mixStateObservationPartial {
		return fmt.Sprintf("%s MixBoard partial observation is ready. Acoustic feature bridge still needs deeper hookup before automatic parameter turns.", mixModeLabel(session.Mode))
	}
	return fmt.Sprintf("%s MixBoard observation is ready.", mixModeLabel(session.Mode))
}

func (s *Server) mixBoardStatusInteraction(interaction PendingInteraction, session MixSession, observation map[string]any) AgentInteractionRequest {
	status := mixObservationStatusLabel(cleanContextText(observation["status"]))
	body := mixBoardStatusBody(session, observation)
	payload := map[string]any{
		"mix_session":     mixSessionMap(session),
		"mix_observation": observation,
		"request_context": interaction.RequestContext,
	}
	if packet := mixTickPacketFromObservation(observation); len(packet) > 0 {
		payload["mix_tick_packet"] = packet
	}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "mode_boundary",
		Type:           "mixboard_status",
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Stage:          session.State,
		Title:          "MixBoard - " + status,
		Body:           body,
		Status:         "completed",
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "start_mix_tuning", Label: "开始调控", Style: "primary", Recommended: true},
			{ID: "revise_mixboard", Label: "更新判断", Style: "secondary"},
			{ID: "refresh_observation", Label: "重新观察", Style: "secondary"},
			{ID: "stop_mix_tuning", Label: "停止调控", Style: "secondary"},
			{ID: "done", Label: "暂时这样", Style: "secondary"},
		},
	}
	if len(session.JournalRefs) > 0 {
		req.Actions = append(req.Actions[:len(req.Actions)-1], AgentInteractionAction{ID: "rollback_last_mix_turn", Label: "撤回上一轮", Style: "secondary"}, req.Actions[len(req.Actions)-1])
	}
	if session.State == mixStateObservationUnavailable || session.State == mixStateFailed || session.TargetRef.ID == "" {
		req.Actions = []AgentInteractionAction{
			{ID: "refresh_observation", Label: "重新观察", Style: "primary", Recommended: true},
			{ID: "revise_mixboard", Label: "更新判断", Style: "secondary"},
			{ID: "done", Label: "暂时这样", Style: "secondary"},
		}
	}
	req.Actions = mixBoardStatusActions(session)
	if mixGoalControlSurfaceTuningBlocker(observation) != "" {
		filtered := make([]AgentInteractionAction, 0, len(req.Actions))
		for _, action := range req.Actions {
			if action.ID == "start_mix_tuning" || action.ID == "auto_tune_mix" || action.ID == "execute_single_mix_tick" || action.ID == "confirm_control_surface" {
				continue
			}
			filtered = append(filtered, action)
		}
		req.Actions = filtered
	}
	if s != nil {
		s.storePendingInteraction(req, payload)
	}
	return req
}

func mixBoardStatusActions(session MixSession) []AgentInteractionAction {
	if session.State == mixStateObservationUnavailable || session.State == mixStateFailed || session.TargetRef.ID == "" {
		return []AgentInteractionAction{
			{ID: "refresh_observation", Label: "重新观察", Style: "primary", Recommended: true},
			{ID: "revise_mixboard", Label: "更新判断", Style: "secondary"},
		}
	}
	actions := []AgentInteractionAction{
		{ID: "confirm_control_surface", Label: "确认控制面", Style: "primary", Recommended: true},
		{ID: "execute_single_mix_tick", Label: "执行一轮", Style: "primary"},
		{ID: "start_mix_tuning", Label: "开始调控", Style: "secondary"},
		{ID: "enter_discussion", Label: "进入讨论", Style: "secondary"},
		{ID: "submit_mixboard_intervention", Label: "提交干预", Style: "secondary"},
		{ID: "revise_mixboard", Label: "更新判断", Style: "secondary"},
		{ID: "refresh_observation", Label: "重新观察", Style: "secondary"},
	}
	if len(session.JournalRefs) > 0 {
		actions = append(actions, AgentInteractionAction{ID: "rollback_last_mix_tick", Label: "撤回本轮", Style: "secondary"})
		actions = append(actions, AgentInteractionAction{ID: "rollback_last_mix_turn", Label: "撤回上次调控", Style: "secondary"})
	}
	if session.State == mixStateTuningRunning {
		actions = append(actions, AgentInteractionAction{ID: "stop_mix_tuning", Label: "停止调控", Style: "secondary"})
	}
	filtered := make([]AgentInteractionAction, 0, len(actions))
	for _, action := range actions {
		switch action.ID {
		case "confirm_control_surface", "execute_single_mix_tick", "start_mix_tuning", "auto_tune_mix", "enter_discussion", "submit_mixboard_intervention", "stop_mix_tuning", "rollback_last_mix_turn":
			continue
		default:
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func mixBoardStatusBody(session MixSession, observation map[string]any) string {
	obs := mapValue(observation["observation"])
	mixboard := mapValue(observation["mixboard"])
	timeRuler := mapValue(obs["time_ruler"])
	sourceCaps := mapValue(obs["source_capabilities"])
	parts := []string{
		"state=" + mixSessionStateLabel(session.State),
		"target=" + firstNonEmpty(session.TargetRef.Label, session.TargetRef.ID, session.TargetRef.Kind),
		"observation=" + firstNonEmpty(cleanContextText(obs["observation_id"]), cleanContextText(observation["observation_id"]), "-"),
		"time_ruler=" + mixTimeRulerLabel(timeRuler),
		"waveform=" + mixObservationStatusLabel(cleanContextText(sourceCaps["waveform_envelope"])),
		"spectrum=" + mixObservationStatusLabel(cleanContextText(sourceCaps["spectrogram_tiles"])),
	}
	if session.BlockingPoint != "" {
		parts = append(parts, "blocker="+session.BlockingPoint)
	} else if blockers := contextStringSlice(mixboard["open_blockers"]); len(blockers) > 0 {
		parts = append(parts, "blocker="+blockers[0])
	}
	for _, key := range []string{"board_path", "observation_path", "context_pack_path"} {
		if path := cleanContextText(observation[key]); path != "" {
			parts = append(parts, key+"="+path)
		}
	}
	return strings.Join(parts, "\n")
}

func mixTimeRulerLabel(timeRuler map[string]any) string {
	duration := mixFloatNumber(timeRuler["duration_seconds"])
	segment := mixFloatNumber(timeRuler["segment_seconds"])
	if duration <= 0 {
		return "pending"
	}
	if segment <= 0 {
		return fmt.Sprintf("%.2fs", duration)
	}
	return fmt.Sprintf("%.2fs / segment %.2fs", duration, segment)
}

func mixSessionStateLabel(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case mixStateObservationReady:
		return "observation_ready"
	case mixStateObservationPartial:
		return "observation_partial"
	case mixStateObservationUnavailable:
		return "observation_unavailable"
	case mixStateReadyObservation:
		return "ready_for_observation"
	default:
		return state
	}
}

func mixObservationStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready":
		return "ready"
	case "partial":
		return "partial"
	case "unavailable":
		return "unavailable"
	case "missing":
		return "missing"
	default:
		return firstNonEmpty(status, "-")
	}
}

func mixFloatNumber(value any) float64 {
	switch x := value.(type) {
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float64:
		return x
	case float32:
		return float64(x)
	default:
		v, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(x)), 64)
		return v
	}
}

func mixTuneTurnsFromObservation(observation map[string]any) []map[string]any {
	candidates := []any{
		mapValue(observation["mixboard"])["auto_tune_turns"],
		observation["auto_tune_turns"],
		mapValue(mapValue(observation["context_pack"])["auto_tune"])["auto_tune_turns"],
		mapValue(mapValue(observation["context_pack"])["session_header"])["auto_tune_turns"],
	}
	for _, candidate := range candidates {
		rows := mixMapRows(candidate)
		if len(rows) > 0 {
			return rows
		}
	}
	return nil
}

func mixLatestTuneBatch(turns []map[string]any) []map[string]any {
	if len(turns) == 0 {
		return nil
	}
	last := turns[len(turns)-1]
	batchID := cleanContextText(last["tuning_batch_id"])
	start := len(turns) - 1
	if batchID != "" {
		for start > 0 && cleanContextText(turns[start-1]["tuning_batch_id"]) == batchID {
			start--
		}
		return turns[start:]
	}
	previousTurn := intNumber(last["turn"])
	for start > 0 {
		currentTurn := intNumber(turns[start-1]["turn"])
		if currentTurn <= 0 || previousTurn <= 0 || currentTurn >= previousTurn {
			break
		}
		start--
		previousTurn = currentTurn
	}
	return turns[start:]
}

func mixTuneBatchActionIDs(turns []map[string]any) []string {
	if len(turns) == 0 {
		return nil
	}
	ids := make([]string, 0, len(turns))
	for _, turn := range turns {
		id := cleanContextText(turn["agent_action_id"])
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func mixMapRows(value any) []map[string]any {
	switch x := value.(type) {
	case []map[string]any:
		return x
	case []any:
		rows := make([]map[string]any, 0, len(x))
		for _, item := range x {
			if row := mapValue(item); len(row) > 0 {
				rows = append(rows, row)
			}
		}
		return rows
	default:
		return nil
	}
}

func mixOptionalFloat(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch x := value.(type) {
		case int:
			return float64(x), true
		case int64:
			return float64(x), true
		case float64:
			return x, true
		case float32:
			return float64(x), true
		case json.Number:
			v, err := x.Float64()
			return v, err == nil
		default:
			text := strings.TrimSpace(fmt.Sprint(x))
			if text == "" {
				continue
			}
			v, err := strconv.ParseFloat(text, 64)
			return v, err == nil
		}
	}
	return 0, false
}

func (s *Server) mixSessionInteractionRequest(conversationID, goalID, runID string, data map[string]any, target MixTargetRef) AgentInteractionRequest {
	session := mixSessionFromMap(mapValue(data["mix_session"]))
	title := mixModeLabel(session.Mode) + "任务设计"
	body := mixSessionEntryReply(session, target)
	actions := []AgentInteractionAction{
		{ID: "confirm_mix_session", Label: "确认开始", Style: "primary", Recommended: true},
		{ID: "edit_mix_target", Label: "修改目标", Style: "secondary"},
		{ID: "cancel_mix_session", Label: "取消", Style: "secondary"},
	}
	if target.ID == "" || strings.EqualFold(target.Confidence, "low") {
		actions = []AgentInteractionAction{
			{ID: "edit_mix_target", Label: "指定目标", Style: "primary", Recommended: true},
			{ID: "cancel_mix_session", Label: "取消", Style: "secondary"},
		}
	}
	fields := []AgentInteractionField{{
		ID:          "target_text",
		Label:       "目标",
		Kind:        "text",
		Value:       target.Label,
		Placeholder: "例如：当前轨道、人声轨、选中片段、vocal bus",
	}}
	req := AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "review",
		Type:           "mix_session_entry",
		Source:         "mix_session",
		Workflow:       mixSessionEntryWorkflow,
		Stage:          session.State,
		Title:          title,
		Body:           body,
		Status:         "waiting_for_user",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Fields:         fields,
		Payload:        data,
		Data:           data,
		Actions:        actions,
	}
	s.storePendingInteraction(req, data)
	return req
}

func (s *Server) storeMixSession(session MixSession) {
	if strings.TrimSpace(session.MixSessionID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mixSessions[session.MixSessionID] = session
}

func (s *Server) lookupMixSession(sessionID string) (MixSession, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if s == nil || sessionID == "" {
		return MixSession{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.mixSessions[sessionID]
	return session, ok
}

func mergeRecoveredMixSession(stored, submitted MixSession) MixSession {
	if stored.MixSessionID == "" {
		return submitted
	}
	out := stored
	if submitted.Mode != "" {
		out.Mode = submitted.Mode
	}
	if submitted.ConversationID != "" {
		out.ConversationID = submitted.ConversationID
	}
	if submitted.GoalText != "" {
		out.GoalText = submitted.GoalText
	}
	if submitted.TargetRef.ID != "" || submitted.TargetRef.Label != "" {
		out.TargetRef = submitted.TargetRef
	}
	if submitted.UserNote != "" {
		out.UserNote = submitted.UserNote
	}
	if len(submitted.PlannerPrep) > 0 {
		out.PlannerPrep = submitted.PlannerPrep
	}
	if submitted.RoundCount > out.RoundCount {
		out.RoundCount = submitted.RoundCount
	}
	if submitted.MaxRounds > 0 {
		out.MaxRounds = submitted.MaxRounds
	}
	return out
}

func mixModeFromRequest(message string, ctx map[string]any) string {
	mode := normalizeMixMode(firstNonEmpty(cleanContextText(ctx["mix_mode"]), cleanContextText(ctx["mode"])))
	if mode != "" && (contextBool(ctx, "mix_requested") || cleanContextText(ctx["mix_entry_source"]) != "") {
		return mode
	}
	if mode != "" && looksLikeMixSessionEntryGoal(message) {
		return mode
	}
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return ""
	}
	if strings.Contains(text, "co-mix") || strings.Contains(text, "comix") || strings.Contains(text, "一起混") || strings.Contains(text, "协同混") || strings.Contains(text, "共同混") ||
		strings.Contains(text, "一起缩混") || strings.Contains(text, "协同缩混") || strings.Contains(text, "共同缩混") {
		return mixModeCo
	}
	if looksLikeMixSessionEntryGoal(message) {
		return mixModeAuto
	}
	return ""
}

func normalizeMixMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case mixModeAuto, "auto", "automix", "auto-mix", "automatic_mix", "自动混音":
		return mixModeAuto
	case mixModeCo, "comix", "co-mix", "collab_mix", "collaborative_mix", "协同混音":
		return mixModeCo
	default:
		return ""
	}
}

func looksLikeMixSessionEntryGoal(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	hasMix := strings.Contains(text, "auto mix") || strings.Contains(text, "automix") || strings.Contains(text, "co-mix") ||
		strings.Contains(text, "mix this") || strings.Contains(text, "mix the") ||
		strings.Contains(text, "自动混") || strings.Contains(text, "协同混") || strings.Contains(text, "一起混") ||
		strings.Contains(text, "帮我混") || strings.Contains(text, "混一下")
	mixVerb := strings.Contains(text, "缩混") || strings.Contains(text, "混音") || strings.Contains(text, "混当前") || strings.Contains(text, "混这")
	mixIntent := strings.Contains(text, "自动") || strings.Contains(text, "帮我") || strings.Contains(text, "能帮") || strings.Contains(text, "请") || strings.Contains(text, "可以帮") || strings.Contains(text, "一起") || strings.Contains(text, "协同")
	if mixVerb && mixIntent {
		hasMix = true
	}
	hasTarget := strings.Contains(text, "track") || strings.Contains(text, "clip") || strings.Contains(text, "bus") ||
		strings.Contains(text, "selection") || strings.Contains(text, "vocal") || strings.Contains(text, "当前") ||
		strings.Contains(text, "这条") || strings.Contains(text, "这个") || strings.Contains(text, "这轨") || strings.Contains(text, "选中") ||
		strings.Contains(text, "轨") || strings.Contains(text, "轨道") || strings.Contains(text, "片段") || strings.Contains(text, "人声")
	return hasMix && hasTarget
}

func resolveMixTargetRef(message string, ctx map[string]any) MixTargetRef {
	if target := mixTargetFromMap(mapValue(ctx["target_ref"])); target.ID != "" {
		return target
	}
	text := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(text, "selection") || strings.Contains(text, "选中") || strings.Contains(text, "当前选区") {
		if ids := contextStringSlice(ctx["selected_clip_ids"]); len(ids) > 0 {
			return MixTargetRef{Kind: "selection", ID: strings.Join(ids, ","), Label: "选中片段", Source: "current_selection", Confidence: "high"}
		}
	}
	if strings.Contains(text, "clip") || strings.Contains(text, "片段") {
		if id := firstNonEmpty(cleanContextText(ctx["selected_clip_id"]), cleanContextText(ctx["piano_roll_focus_clip_id"])); id != "" {
			return MixTargetRef{Kind: "clip", ID: id, Label: firstNonEmpty(cleanContextText(ctx["selected_clip_name"]), "当前片段"), Source: "current_selection", Confidence: "high"}
		}
	}
	if strings.Contains(text, "bus") || strings.Contains(text, "编组") || strings.Contains(text, "总线") {
		if id := firstNonEmpty(cleanContextText(ctx["selected_track_id"]), cleanContextText(ctx["selected_plugin_track_id"])); id != "" {
			return MixTargetRef{Kind: "bus", ID: id, Label: firstNonEmpty(cleanContextText(ctx["selected_track_name"]), "当前 Bus"), Source: "current_selection", Confidence: "high"}
		}
	}
	if id := firstNonEmpty(cleanContextText(ctx["selected_track_id"]), cleanContextText(ctx["selected_scene_track_id"]), cleanContextText(ctx["selected_plugin_track_id"]), cleanContextText(ctx["piano_roll_focus_track_id"])); id != "" {
		return MixTargetRef{Kind: "track", ID: id, Label: firstNonEmpty(cleanContextText(ctx["selected_track_name"]), cleanContextText(ctx["selected_plugin_name"]), "当前轨道"), Source: "current_selection", Confidence: "high"}
	}
	if id := cleanContextText(ctx["selected_clip_id"]); id != "" {
		return MixTargetRef{Kind: "clip", ID: id, Label: firstNonEmpty(cleanContextText(ctx["selected_clip_name"]), "当前片段"), Source: "current_selection", Confidence: "high"}
	}
	return MixTargetRef{Kind: "selection", Label: "未指定目标", Source: "missing", Confidence: "low"}
}

func mixManualTargetRef(text string) MixTargetRef {
	text = strings.TrimSpace(text)
	if text == "" {
		return MixTargetRef{}
	}
	kind := "manual"
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "clip"):
		kind = "clip"
	case strings.Contains(lower, "bus"):
		kind = "bus"
	case strings.Contains(lower, "selection"):
		kind = "selection"
	case strings.Contains(lower, "track") || strings.Contains(lower, "vocal"):
		kind = "track"
	}
	return MixTargetRef{Kind: kind, ID: text, Label: text, Source: "user_text", Confidence: "medium"}
}

func pendingMixSession(mode string, target MixTargetRef, goalText string) MixSession {
	now := time.Now().Format(time.RFC3339Nano)
	if mode == "" {
		mode = mixModeAuto
	}
	maxRounds := 8
	if mode == mixModeCo {
		maxRounds = 9
	}
	plannerPolicy := mixPlannerPolicyAutoAssist
	if mode == mixModeCo {
		plannerPolicy = mixPlannerPolicyCoMixDirector
	}
	return MixSession{
		MixSessionID:       "mix_" + randomID(),
		Mode:               mode,
		State:              mixStateWaitingConfirmation,
		TargetRef:          target,
		GoalText:           strings.TrimSpace(goalText),
		ApprovedPlugins:    []string{},
		ControlSurfaceID:   "",
		ExecutorType:       mixFallbackExecutorType,
		ExecutorVersion:    mixFallbackExecutorVersion,
		ReviewStatus:       "",
		StopReason:         "",
		InteractionPhase:   mixInteractionPlanningChat,
		MixBoardVisibility: mixBoardVisibilityHidden,
		PlannerPolicy:      plannerPolicy,
		FastModelProfile:   "",
		RoundCount:         0,
		MaxRounds:          maxRounds,
		JournalRefs:        []string{},
		Preparation:        mixPreparationRows(target),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func mixPreparationRows(target MixTargetRef) []map[string]any {
	targetStatus := "blocked"
	if target.ID != "" && !strings.EqualFold(target.Confidence, "low") {
		targetStatus = "ready"
	}
	return []map[string]any{
		{"id": "target", "label": "混音目标", "status": targetStatus},
		{"id": "observation", "label": "音频观察", "status": "blocked", "reason": "mix.request_observation not available yet"},
		{"id": "plugin_profiles", "label": "Plugin Grabber profile", "status": "unknown"},
		{"id": "transport", "label": "Transport 控制", "status": "disabled"},
		{"id": "automation", "label": "Automation 写入", "status": "disabled"},
	}
}

func mixSessionWorkflowData(conversationID, goalID, runID string, requestContext map[string]any, session MixSession) map[string]any {
	data := map[string]any{
		"workflow":        mixSessionEntryWorkflow,
		"conversation_id": firstNonEmpty(session.ConversationID, conversationID),
		"goal_id":         goalID,
		"run_id":          runID,
		"mode":            session.Mode,
		"target_ref":      mixTargetMap(session.TargetRef),
		"goal_text":       session.GoalText,
		"mix_session":     mixSessionMap(session),
		"request_context": requestContext,
		"planner_policy":  mixPlannerPolicy(session),
	}
	if len(session.PlannerPrep) > 0 {
		data["mix_planner_prep"] = session.PlannerPrep
		if workspace := mapValue(session.PlannerPrep["mix_planning_workspace"]); len(workspace) > 0 {
			data["mix_planning_workspace"] = workspace
		}
	}
	return data
}

func mixSessionEntryReply(session MixSession, target MixTargetRef) string {
	targetLabel := firstNonEmpty(target.Label, target.ID, "未指定目标")
	if target.ID == "" || strings.EqualFold(target.Confidence, "low") {
		return fmt.Sprintf("%s需要先明确混音目标。确认前不会创建 MixSession，也不会修改 DAW。", mixModeLabel(session.Mode))
	}
	return fmt.Sprintf("%s任务已准备确认：目标是 %s。确认前不会播放、写参数、调用 Plugin Grabber 或修改 DAW。", mixModeLabel(session.Mode), targetLabel)
}

func mixPlanModeReply(mode string, target MixTargetRef, goalText string) string {
	targetLabel := firstNonEmpty(target.Label, "未指定目标")
	return fmt.Sprintf("Plan mode 下我只会规划%s：目标=%s，需求=%s。不会创建 MixSession、不会生成控制面，也不会修改 DAW。", mixModeLabel(mode), targetLabel, firstNonEmpty(goalText, "未填写"))
}

func mixModeLabel(mode string) string {
	if mode == mixModeCo {
		return "协同混音"
	}
	return "自动混音"
}

func mixTargetFromMap(row map[string]any) MixTargetRef {
	if len(row) == 0 {
		return MixTargetRef{}
	}
	return MixTargetRef{
		Kind:       firstNonEmpty(cleanContextText(row["kind"]), "selection"),
		ID:         cleanContextText(row["id"]),
		Label:      cleanContextText(row["label"]),
		Source:     cleanContextText(row["source"]),
		Confidence: firstNonEmpty(cleanContextText(row["confidence"]), "low"),
	}
}

func mixTargetMap(target MixTargetRef) map[string]any {
	return map[string]any{
		"kind":       target.Kind,
		"id":         target.ID,
		"label":      target.Label,
		"source":     target.Source,
		"confidence": target.Confidence,
	}
}

func mixSessionFromMap(row map[string]any) MixSession {
	if len(row) == 0 {
		return MixSession{}
	}
	return MixSession{
		MixSessionID:       cleanContextText(row["mix_session_id"]),
		ConversationID:     cleanContextText(row["conversation_id"]),
		Mode:               cleanContextText(row["mode"]),
		State:              cleanContextText(row["state"]),
		TargetRef:          mixTargetFromMap(mapValue(row["target_ref"])),
		GoalText:           cleanContextText(row["goal_text"]),
		UserNote:           cleanContextText(row["user_note"]),
		ApprovedPlugins:    contextStringSlice(row["approved_plugins"]),
		ControlSurfaceID:   cleanContextText(row["control_surface_id"]),
		ExecutorType:       firstNonEmpty(cleanContextText(row["executor_type"]), mixFallbackExecutorType),
		ExecutorVersion:    firstNonEmpty(cleanContextText(row["executor_version"]), mixFallbackExecutorVersion),
		ReviewStatus:       cleanContextText(row["review_status"]),
		StopReason:         cleanContextText(row["stop_reason"]),
		InteractionPhase:   firstNonEmpty(cleanContextText(row["interaction_phase"]), mixInteractionPlanningChat),
		MixBoardVisibility: firstNonEmpty(cleanContextText(row["mixboard_visibility"]), mixBoardVisibilityHidden),
		PlannerPolicy:      firstNonEmpty(cleanContextText(row["planner_policy"]), mixPlannerPolicyForMode(cleanContextText(row["mode"]))),
		FastModelProfile:   cleanContextText(row["fast_model_profile"]),
		RoundCount:         intNumber(row["round_count"]),
		MaxRounds:          intNumber(row["max_rounds"]),
		JournalRefs:        contextStringSlice(row["journal_refs"]),
		BlockingPoint:      cleanContextText(row["blocking_point"]),
		Preparation:        mapRowsValue(row["preparation"]),
		PlannerPrep:        mapValue(row["mix_planner_prep"]),
		CreatedAt:          cleanContextText(row["created_at"]),
		UpdatedAt:          cleanContextText(row["updated_at"]),
	}
}

func mixSessionMap(session MixSession) map[string]any {
	return map[string]any{
		"mix_session_id":      session.MixSessionID,
		"conversation_id":     session.ConversationID,
		"mode":                session.Mode,
		"state":               session.State,
		"target_ref":          mixTargetMap(session.TargetRef),
		"goal_text":           session.GoalText,
		"user_note":           session.UserNote,
		"approved_plugins":    session.ApprovedPlugins,
		"control_surface_id":  session.ControlSurfaceID,
		"executor_type":       firstNonEmpty(session.ExecutorType, mixFallbackExecutorType),
		"executor_version":    firstNonEmpty(session.ExecutorVersion, mixFallbackExecutorVersion),
		"review_status":       session.ReviewStatus,
		"stop_reason":         session.StopReason,
		"interaction_phase":   mixInteractionPhase(session),
		"mixboard_visibility": mixBoardVisibility(session),
		"planner_policy":      mixPlannerPolicy(session),
		"fast_model_profile":  session.FastModelProfile,
		"round_count":         session.RoundCount,
		"max_rounds":          session.MaxRounds,
		"journal_refs":        session.JournalRefs,
		"blocking_point":      session.BlockingPoint,
		"preparation":         session.Preparation,
		"mix_planner_prep":    session.PlannerPrep,
		"created_at":          session.CreatedAt,
		"updated_at":          session.UpdatedAt,
	}
}

func mixPlannerPolicyForMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), mixModeCo) {
		return mixPlannerPolicyCoMixDirector
	}
	return mixPlannerPolicyAutoAssist
}

func mixPlannerPolicy(session MixSession) string {
	return firstNonEmpty(session.PlannerPolicy, mixPlannerPolicyForMode(session.Mode))
}

func mixFastModelProfile(session MixSession, context map[string]any) string {
	return firstNonEmpty(
		cleanContextText(context["fast_model_profile"]),
		cleanContextText(context["mix_fast_model_profile"]),
		cleanContextText(context["mix_tick_model_profile"]),
		session.FastModelProfile,
		"mix_tick",
	)
}

func mixFastModelStrategy(profile string) map[string]any {
	return map[string]any{
		"profile":                  firstNonEmpty(profile, "mix_tick"),
		"route":                    "mix_tick",
		"phase":                    mixInteractionFastTickRunning,
		"reasoning_effort":         "low",
		"chain_of_thought":         "disabled",
		"max_decisions_per_tick":   1,
		"fallback_to_default":      true,
		"prompt_style":             "short_structured",
		"requires_mix_tick_packet": true,
	}
}

func mixInteractionPhase(session MixSession) string {
	if strings.TrimSpace(session.InteractionPhase) != "" {
		return session.InteractionPhase
	}
	switch session.State {
	case mixStateTuningRunning:
		return mixInteractionFastTickRunning
	case mixStateWaitingReview, mixStateTuningPaused, mixStateRollbackAvailable:
		return mixInteractionWaitingPlannerReview
	case mixStateObservationReady, mixStateObservationPartial:
		return mixInteractionControlSurfacePublished
	default:
		return mixInteractionPlanningChat
	}
}

func mixBoardVisibility(session MixSession) string {
	if strings.TrimSpace(session.MixBoardVisibility) != "" {
		return session.MixBoardVisibility
	}
	switch mixInteractionPhase(session) {
	case mixInteractionFastTickRunning:
		return mixBoardVisibilityExecution
	case mixInteractionControlSurfacePublished, mixInteractionReadyForTick, mixInteractionWaitingPlannerReview, mixInteractionLearningRequired:
		return mixBoardVisibilityPublished
	case mixInteractionPlanningChat:
		return mixBoardVisibilityCollapsed
	default:
		return mixBoardVisibilityHidden
	}
}

func mixSessionHTTPStatus(resp ChatResponse) int {
	if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
		return http.StatusBadRequest
	}
	return http.StatusOK
}
