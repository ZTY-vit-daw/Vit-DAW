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

	"vit-daw-agent/internal/harness"
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
	mixControlPlanSchemaVersion       = "mix_control_plan.v1"
	mixMacroControlPanelSchemaVersion = "mix_macro_control_panel.v1"
	mixBuiltInVolumeMacroRole         = "gain_staging"
	mixBuiltInVolumeMacroControl      = "track.volume"
)

type MixSession struct {
	MixSessionID     string           `json:"mix_session_id"`
	Mode             string           `json:"mode"`
	State            string           `json:"state"`
	TargetRef        MixTargetRef     `json:"target_ref"`
	GoalText         string           `json:"goal_text"`
	UserNote         string           `json:"user_note,omitempty"`
	ApprovedPlugins  []string         `json:"approved_plugins"`
	ControlSurfaceID string           `json:"control_surface_id"`
	ExecutorType     string           `json:"executor_type,omitempty"`
	ExecutorVersion  string           `json:"executor_version,omitempty"`
	ReviewStatus     string           `json:"review_status,omitempty"`
	StopReason       string           `json:"stop_reason,omitempty"`
	RoundCount       int              `json:"round_count"`
	MaxRounds        int              `json:"max_rounds"`
	JournalRefs      []string         `json:"journal_refs"`
	BlockingPoint    string           `json:"blocking_point,omitempty"`
	Preparation      []map[string]any `json:"preparation,omitempty"`
	CreatedAt        string           `json:"created_at,omitempty"`
	UpdatedAt        string           `json:"updated_at,omitempty"`
}

type MixTargetRef struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Label      string `json:"label"`
	Source     string `json:"source"`
	Confidence string `json:"confidence"`
}

func (s *Server) runMixSessionEntryChat(ctx context.Context, conversationID string, req ChatRequest, agentMode string) (ChatResponse, bool) {
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
	if strings.EqualFold(decision, "start_mix_tuning") || strings.EqualFold(decision, "auto_tune_mix") {
		return s.runMixTuningInteraction(ctx, interaction, data, session, payload)
	}
	if strings.EqualFold(decision, "stop_mix_tuning") {
		return s.stopMixTuningInteraction(interaction, data, session)
	}
	if strings.EqualFold(decision, "rollback_last_mix_turn") {
		return s.rollbackLastMixTurnInteraction(ctx, interaction, data, session)
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
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	session.Preparation = mixPreparationRows(session.TargetRef)
	session.State = mixStateReadyObservation
	session.BlockingPoint = ""
	observationResult, observationErr := s.requestInitialMixObservation(ctx, interaction, session)
	session.State = mixStateFromObservationResult(observationResult, observationErr)
	session.BlockingPoint = mixObservationBlockingPoint(observationResult, observationErr)
	session.RoundCount = 1
	session.Preparation = mixPreparationRowsForObservation(session.TargetRef, session.State, session.BlockingPoint)
	session.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	s.storeMixSession(session)
	data["mix_session"] = mixSessionMap(session)
	if observationResult != nil {
		s.attachGoalControlSurface(ctx, interaction, session, observationResult)
		updateMixBoardRuntimeState(observationResult, session, nil)
		data["goal_control_surface"] = mixGoalControlSurfaceFromObservation(observationResult)
		data["mix_observation"] = observationResult
	}
	executedReplies := mixMacroUpsertExecutions(mixVolumeMacroFromObservation(observationResult, session))
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               mixObservationReply(session, observationErr),
		Workflow:            mixSessionEntryWorkflow,
		WorkflowData:        data,
		MixSession:          mixSessionMap(session),
		InteractionRequests: []AgentInteractionRequest{s.mixBoardStatusInteraction(interaction, session, observationResult)},
		ExecutedKernelReply: executedReplies,
		GoalStatus:          string(agentruntime.StatusWaitingContinue),
		CurrentStep:         session.State,
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
		"session_state":      session.State,
		"executor_type":      executorType,
		"executor_version":   executorVersion,
		"review_status":      reviewStatus,
		"stop_reason":        session.StopReason,
		"round_count":        session.RoundCount,
		"max_rounds":         session.MaxRounds,
		"rollback_available": len(session.JournalRefs) > 0,
		"updated_at":         now,
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
			if action.ID == "start_mix_tuning" || action.ID == "auto_tune_mix" {
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
		{ID: "start_mix_tuning", Label: "开始调控", Style: "primary", Recommended: true},
		{ID: "revise_mixboard", Label: "更新判断", Style: "secondary"},
		{ID: "refresh_observation", Label: "重新观察", Style: "secondary"},
	}
	if len(session.JournalRefs) > 0 {
		actions = append(actions, AgentInteractionAction{ID: "rollback_last_mix_turn", Label: "撤回上次调控", Style: "secondary"})
	}
	if session.State == mixStateTuningRunning {
		actions = append(actions, AgentInteractionAction{ID: "stop_mix_tuning", Label: "停止调控", Style: "secondary"})
	}
	return actions
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
	if submitted.GoalText != "" {
		out.GoalText = submitted.GoalText
	}
	if submitted.TargetRef.ID != "" || submitted.TargetRef.Label != "" {
		out.TargetRef = submitted.TargetRef
	}
	if submitted.UserNote != "" {
		out.UserNote = submitted.UserNote
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
	return MixSession{
		MixSessionID:     "mix_" + randomID(),
		Mode:             mode,
		State:            mixStateWaitingConfirmation,
		TargetRef:        target,
		GoalText:         strings.TrimSpace(goalText),
		ApprovedPlugins:  []string{},
		ControlSurfaceID: "",
		ExecutorType:     mixFallbackExecutorType,
		ExecutorVersion:  mixFallbackExecutorVersion,
		ReviewStatus:     "",
		StopReason:       "",
		RoundCount:       0,
		MaxRounds:        maxRounds,
		JournalRefs:      []string{},
		Preparation:      mixPreparationRows(target),
		CreatedAt:        now,
		UpdatedAt:        now,
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
	return map[string]any{
		"workflow":        mixSessionEntryWorkflow,
		"conversation_id": conversationID,
		"goal_id":         goalID,
		"run_id":          runID,
		"mode":            session.Mode,
		"target_ref":      mixTargetMap(session.TargetRef),
		"goal_text":       session.GoalText,
		"mix_session":     mixSessionMap(session),
		"request_context": requestContext,
	}
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
		MixSessionID:     cleanContextText(row["mix_session_id"]),
		Mode:             cleanContextText(row["mode"]),
		State:            cleanContextText(row["state"]),
		TargetRef:        mixTargetFromMap(mapValue(row["target_ref"])),
		GoalText:         cleanContextText(row["goal_text"]),
		UserNote:         cleanContextText(row["user_note"]),
		ApprovedPlugins:  contextStringSlice(row["approved_plugins"]),
		ControlSurfaceID: cleanContextText(row["control_surface_id"]),
		ExecutorType:     firstNonEmpty(cleanContextText(row["executor_type"]), mixFallbackExecutorType),
		ExecutorVersion:  firstNonEmpty(cleanContextText(row["executor_version"]), mixFallbackExecutorVersion),
		ReviewStatus:     cleanContextText(row["review_status"]),
		StopReason:       cleanContextText(row["stop_reason"]),
		RoundCount:       intNumber(row["round_count"]),
		MaxRounds:        intNumber(row["max_rounds"]),
		JournalRefs:      contextStringSlice(row["journal_refs"]),
		BlockingPoint:    cleanContextText(row["blocking_point"]),
		Preparation:      mapRowsValue(row["preparation"]),
		CreatedAt:        cleanContextText(row["created_at"]),
		UpdatedAt:        cleanContextText(row["updated_at"]),
	}
}

func mixSessionMap(session MixSession) map[string]any {
	return map[string]any{
		"mix_session_id":     session.MixSessionID,
		"mode":               session.Mode,
		"state":              session.State,
		"target_ref":         mixTargetMap(session.TargetRef),
		"goal_text":          session.GoalText,
		"user_note":          session.UserNote,
		"approved_plugins":   session.ApprovedPlugins,
		"control_surface_id": session.ControlSurfaceID,
		"executor_type":      firstNonEmpty(session.ExecutorType, mixFallbackExecutorType),
		"executor_version":   firstNonEmpty(session.ExecutorVersion, mixFallbackExecutorVersion),
		"review_status":      session.ReviewStatus,
		"stop_reason":        session.StopReason,
		"round_count":        session.RoundCount,
		"max_rounds":         session.MaxRounds,
		"journal_refs":       session.JournalRefs,
		"blocking_point":     session.BlockingPoint,
		"preparation":        session.Preparation,
		"created_at":         session.CreatedAt,
		"updated_at":         session.UpdatedAt,
	}
}

func mixSessionHTTPStatus(resp ChatResponse) int {
	if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
		return http.StatusBadRequest
	}
	return http.StatusOK
}
