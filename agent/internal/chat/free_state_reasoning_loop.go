package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	freeStateReasoningLoopSchema           = "free_state_reasoning_loop.v1"
	freeStateDefaultMaxCycles              = 6
	freeStatePhaseProcessorSelection       = "processor_selection"
	freeStatePhaseProcessorMaterialization = "processor_materialization"
	freeStatePhasePostActionEvaluation     = "post_action_evaluation"
)

type freeStateActionRecord struct {
	Cycle         int            `json:"cycle"`
	ProcessorType string         `json:"processor_type"`
	Workflow      string         `json:"workflow"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary,omitempty"`
	Receipt       map[string]any `json:"receipt,omitempty"`
	RecordedAt    time.Time      `json:"recorded_at"`
}

// freeStateReasoningLoop is orchestration memory, not mutation authority. It
// keeps one user intent intact while governed processor workflows pause for
// confirmation and return their receipts on later HTTP turns.
type freeStateReasoningLoop struct {
	SchemaVersion                 string                       `json:"schema_version"`
	LoopID                        string                       `json:"loop_id"`
	ConversationID                string                       `json:"conversation_id"`
	GoalID                        string                       `json:"goal_id,omitempty"`
	RunID                         string                       `json:"run_id,omitempty"`
	Status                        string                       `json:"status"`
	DecisionPhase                 string                       `json:"decision_phase"`
	OriginalIntent                string                       `json:"original_intent"`
	ActiveIntent                  string                       `json:"active_intent"`
	TargetRef                     map[string]any               `json:"target_ref,omitempty"`
	Cycle                         int                          `json:"cycle"`
	MaxCycles                     int                          `json:"max_cycles"`
	ObservationIDs                []string                     `json:"observation_ids,omitempty"`
	LatestObservation             *agentloop.RecentObservation `json:"latest_observation,omitempty"`
	Actions                       []freeStateActionRecord      `json:"actions,omitempty"`
	LatestDecision                *agentloop.FreeStateDecision `json:"latest_decision,omitempty"`
	RequiresPostActionObservation bool                         `json:"requires_post_action_observation"`
	LastError                     string                       `json:"last_error,omitempty"`
	CreatedAt                     time.Time                    `json:"created_at"`
	UpdatedAt                     time.Time                    `json:"updated_at"`
}

func freeStateLoopActive(loop freeStateReasoningLoop) bool {
	if loop.SchemaVersion != freeStateReasoningLoopSchema || strings.TrimSpace(loop.OriginalIntent) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(loop.Status)) {
	case "completed", "cancelled", "blocked":
		return false
	default:
		return true
	}
}

func shouldStartFreeStateReasoningLoop(userText string, requestContext map[string]any) bool {
	if useLegacyPlannerLoop() || agentModeFromContext(requestContext) == agentModePlan ||
		contextBool(requestContext, "disable_free_state_reasoning") || strings.HasPrefix(strings.TrimSpace(userText), "/") {
		return false
	}
	if ordinaryAgentPluginRecommendationIntent(userText, requestContext) || compressorExactParameterRequest.MatchString(userText) {
		return false
	}
	return ordinaryAgentSemanticEQMutationRequest(userText, requestContext) ||
		ordinaryAgentTreatmentStrategyIntent(userText, requestContext) ||
		ordinaryAgentSemanticCompressorPlanningRequest(userText, requestContext)
}

func (s *Server) prepareFreeStateReasoningContext(conversationID, userText string, requestContext map[string]any) (map[string]any, bool) {
	if s == nil {
		return requestContext, false
	}
	loop, ok := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"])
	if !ok {
		s.mu.Lock()
		loop, ok = s.freeStateLoops[conversationID]
		s.mu.Unlock()
	}
	if !freeStateLoopActive(loop) {
		if contextBool(requestContext, "free_state_internal_resume") || !shouldStartFreeStateReasoningLoop(userText, requestContext) {
			return requestContext, false
		}
		now := time.Now().UTC()
		goalID, runID := goalIDsFromContext(requestContext)
		loop = freeStateReasoningLoop{
			SchemaVersion:  freeStateReasoningLoopSchema,
			LoopID:         "free_state_" + randomID(),
			ConversationID: conversationID,
			GoalID:         goalID,
			RunID:          runID,
			Status:         "reasoning",
			DecisionPhase:  freeStatePhaseProcessorSelection,
			OriginalIntent: strings.TrimSpace(userText),
			ActiveIntent:   strings.TrimSpace(userText),
			TargetRef:      freeStateTargetRef(requestContext),
			MaxCycles:      freeStateDefaultMaxCycles,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		s.storeFreeStateLoop(loop)
	}
	goalID, runID := goalIDsFromContext(requestContext)
	if goalID != "" {
		loop.GoalID = goalID
	}
	if runID != "" {
		loop.RunID = runID
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return mergeContext(requestContext, map[string]any{"free_state_reasoning_loop": freeStateLoopMap(loop)}), true
}

func freeStateTargetRef(ctx map[string]any) map[string]any {
	trackID := firstStringFromMap(ctx, "selected_track_id", "selected_plugin_track_id")
	pluginID := firstStringFromMap(ctx, "selected_plugin_id")
	if trackID == "" && pluginID == "" {
		return nil
	}
	return map[string]any{
		"kind":        map[bool]string{true: "plugin", false: "track"}[pluginID != ""],
		"id":          firstNonEmpty(pluginID, trackID),
		"label":       firstNonEmpty(firstStringFromMap(ctx, "selected_plugin_name"), firstStringFromMap(ctx, "selected_track_name")),
		"track_id":    trackID,
		"track_name":  firstStringFromMap(ctx, "selected_track_name"),
		"plugin_id":   pluginID,
		"plugin_name": firstStringFromMap(ctx, "selected_plugin_name"),
	}
}

func (s *Server) storeFreeStateLoop(loop freeStateReasoningLoop) {
	if s == nil || strings.TrimSpace(loop.ConversationID) == "" {
		return
	}
	loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
	loop = cloneFreeStateLoop(loop)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.freeStateLoops == nil {
		s.freeStateLoops = map[string]freeStateReasoningLoop{}
	}
	s.freeStateLoops[loop.ConversationID] = loop
}

func (s *Server) freeStateLoop(conversationID string) (freeStateReasoningLoop, bool) {
	if s == nil {
		return freeStateReasoningLoop{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	loop, ok := s.freeStateLoops[conversationID]
	return cloneFreeStateLoop(loop), ok
}

func (s *Server) hasActiveFreeStateReasoningLoop(conversationID string) bool {
	loop, ok := s.freeStateLoop(conversationID)
	return ok && freeStateLoopActive(loop)
}

func (s *Server) recordFreeStateDecision(conversationID string, res agentloop.Result) (freeStateReasoningLoop, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) || res.FreeStateDecision == nil {
		return loop, ok
	}
	decision := *res.FreeStateDecision
	decision.RequestedViewIDs = append([]string(nil), res.FreeStateDecision.RequestedViewIDs...)
	decision.Limitations = append([]string(nil), res.FreeStateDecision.Limitations...)
	loop.LatestDecision = &decision
	if res.GoalID != "" {
		loop.GoalID = res.GoalID
	}
	if res.RunID != "" {
		loop.RunID = res.RunID
	}
	if decision.ObservationID != "" && !freeStateContainsString(loop.ObservationIDs, decision.ObservationID) {
		loop.ObservationIDs = append(loop.ObservationIDs, decision.ObservationID)
	}
	observation := freeStateCCBObservation(res)
	if observation != nil {
		loop.LatestObservation = observation
		if target := freeStateObservationTrackTarget(observation); len(target) > 0 {
			loop.TargetRef = target
		}
		if observationID := firstStringFromMap(observation.Summary, "observation_id"); observationID != "" &&
			!freeStateContainsString(loop.ObservationIDs, observationID) {
			loop.ObservationIDs = append(loop.ObservationIDs, observationID)
		}
	}
	switch strings.ToLower(strings.TrimSpace(decision.Status)) {
	case agentloop.FreeStateNeedsAction:
		loop.Status = "awaiting_action"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.ActiveIntent = strings.TrimSpace(decision.RemainingIntent)
		if observation != nil {
			loop.RequiresPostActionObservation = false
		}
	case agentloop.FreeStateNeedsObservation:
		loop.Status = "observing"
		loop.DecisionPhase = resolvedFreeStateDecisionPhase(loop)
	case agentloop.FreeStateSatisfied:
		loop.Status = "completed"
		loop.ActiveIntent = ""
		loop.RequiresPostActionObservation = false
	case agentloop.FreeStateBlocked:
		loop.Status = "blocked"
		loop.LastError = firstNonEmpty(decision.StopReason, strings.Join(decision.Limitations, "; "), decision.Summary)
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	return loop, true
}

func freeStateObservationTrackTarget(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	target := firstMapFromAny(observation.Summary["target_ref"])
	kind := strings.ToLower(firstStringFromMap(target, "kind", "target_kind"))
	trackID := firstStringFromMap(target, "track_id")
	if kind == "track" {
		trackID = firstNonEmpty(trackID, firstStringFromMap(target, "id", "target_id"))
	}
	if trackID == "" {
		return nil
	}
	trackName := firstStringFromMap(target, "track_name", "label", "name")
	return map[string]any{
		"kind":       "track",
		"id":         trackID,
		"label":      trackName,
		"track_id":   trackID,
		"track_name": trackName,
		"source":     "ccb_observation_binding",
	}
}

func (s *Server) bindFreeStateAuthoritativeTrack(conversationID string, requestContext map[string]any) map[string]any {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK && freeStateLoopActive(recovered) {
			loop, ok = recovered, true
		}
	}
	if !ok || !freeStateLoopActive(loop) {
		return requestContext
	}
	trackID := firstStringFromMap(loop.TargetRef, "track_id")
	if strings.EqualFold(firstStringFromMap(loop.TargetRef, "kind"), "track") {
		trackID = firstNonEmpty(trackID, firstStringFromMap(loop.TargetRef, "id"))
	}
	if trackID == "" {
		return requestContext
	}
	trackName := firstStringFromMap(loop.TargetRef, "track_name", "label", "name")
	out := mergeContext(requestContext, map[string]any{
		"selected_track_id":   trackID,
		"selected_track_name": trackName,
	})
	if pluginTrackID := firstStringFromMap(out, "selected_plugin_track_id"); pluginTrackID != "" && pluginTrackID != trackID {
		delete(out, "selected_plugin_id")
		delete(out, "selected_plugin_name")
		delete(out, "selected_plugin_track_id")
	}
	return out
}

func freeStateMaterializationRetry(loop freeStateReasoningLoop) (string, agentloop.Result, bool) {
	if !freeStateLoopActive(loop) || !strings.EqualFold(loop.Status, "awaiting_action") ||
		loop.LatestDecision == nil || !strings.EqualFold(loop.LatestDecision.Status, agentloop.FreeStateNeedsAction) {
		return "", agentloop.Result{}, false
	}
	decision := *loop.LatestDecision
	decision.RequestedViewIDs = append([]string(nil), loop.LatestDecision.RequestedViewIDs...)
	decision.Limitations = append([]string(nil), loop.LatestDecision.Limitations...)
	return firstNonEmpty(decision.RemainingIntent, loop.ActiveIntent, loop.OriginalIntent), agentloop.Result{
		GoalID:            loop.GoalID,
		RunID:             loop.RunID,
		Status:            agentruntime.StatusWaitingContinue,
		StopReason:        agentloop.StopReasonTransientLLMError,
		GoalSummary:       loop.OriginalIntent,
		RecentObservation: loop.LatestObservation,
		FreeStateDecision: &decision,
	}, true
}

func freeStateTransientServiceError(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"context deadline exceeded", "timeout", "timed out", "awaiting headers",
		"502", "503", "504", "temporary", "stream returned no output", "connection reset",
		"runtime configuration state unavailable", "\u8fd0\u884c\u65f6\u914d\u7f6e\u72b6\u6001\u4e0d\u53ef\u7528",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (s *Server) makeFreeStateMaterializationResumable(conversationID string, requestContext map[string]any,
	res agentloop.Result, response ChatResponse) ChatResponse {
	if !freeStateRouteAuthorized(requestContext) || response.GoalStatus != string(agentruntime.StatusFailed) ||
		!freeStateTransientServiceError(response.Error) {
		return response
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !freeStateLoopActive(loop) || !strings.EqualFold(loop.Status, "awaiting_action") {
		return response
	}
	originalError := response.Error
	response.GoalStatus = string(agentruntime.StatusWaitingContinue)
	response.StopReason = agentloop.StopReasonTransientLLMError
	response.Error = ""
	response.Reply = "处理器选择或物化阶段遇到临时模型服务错误。原始意图、CCB 观察和待处理动作均已保留；任务尚未完成，可以继续重试这一阶段。"
	response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{
		"status": "paused_transient", "resumable": true, "transient_error": originalError, "mutation_performed": false,
	})
	goalID := firstNonEmpty(response.GoalID, res.GoalID, loop.GoalID)
	if s != nil && s.harness != nil && goalID != "" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusWaitingContinue, nil)
	}
	if s != nil && conversationID != "" && goalID != "" {
		s.mu.Lock()
		s.conversationGoals[conversationID] = goalID
		s.mu.Unlock()
	}
	return response
}

func (s *Server) resumeFreeStateMaterialization(ctx context.Context, conversationID, mode string,
	requestContext map[string]any, cfg config.EngineConfig) (ChatResponse, bool) {
	loop, ok := s.freeStateLoop(conversationID)
	userText, res, resumable := freeStateMaterializationRetry(loop)
	if !ok || !resumable {
		return ChatResponse{}, false
	}
	bound := mergeContext(requestContext, map[string]any{
		"free_state_route_authorized": true,
		"free_state_internal_resume":  true,
		"free_state_processor_type":   strings.ToLower(strings.TrimSpace(loop.LatestDecision.ProcessorType)),
		"free_state_reasoning_loop":   freeStateLoopMap(loop),
		"goal_id":                     loop.GoalID,
		"run_id":                      loop.RunID,
	})
	response, routed := s.routeOrdinaryAgentTreatmentStrategy(ctx, conversationID, mode, userText, bound, res, cfg)
	if !routed {
		return ChatResponse{}, false
	}
	return s.makeFreeStateMaterializationResumable(conversationID, bound, res, response), true
}

func freeStateContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func freeStateRouteAuthorized(ctx map[string]any) bool {
	return contextBool(ctx, "free_state_route_authorized") && freeStateLoopActiveContext(ctx)
}

func freeStateLoopActiveContext(ctx map[string]any) bool {
	loop, ok := freeStateLoopFromAny(ctx["free_state_reasoning_loop"])
	return ok && freeStateLoopActive(loop)
}

func (s *Server) bindFreeStateContextToResponse(resp ChatResponse, requestContext map[string]any) ChatResponse {
	loop, ok := s.freeStateLoop(resp.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
			s.storeFreeStateLoop(loop)
		}
	}
	if !ok {
		return resp
	}
	if reason, blocked := freeStateMaterializationBlockReason(resp); blocked && freeStateLoopActive(loop) {
		loop.Status = "blocked"
		loop.LastError = reason
		loop.RequiresPostActionObservation = false
		loop.LatestDecision = &agentloop.FreeStateDecision{
			SchemaVersion:  agentloop.FreeStateDecisionSchema,
			Status:         agentloop.FreeStateBlocked,
			EvidenceStatus: "insufficient",
			Summary:        reason,
			StopReason:     firstNonEmpty(resp.StopReason, "semantic_treatment_capability_boundary"),
			Limitations:    []string{reason},
		}
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
	}
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	ctx := cloneContext(firstMapFromAny(resp.WorkflowData["request_context"]))
	ctx = mergeContext(ctx, requestContext)
	ctx["free_state_reasoning_loop"] = freeStateLoopMap(loop)
	resp.WorkflowData["request_context"] = ctx
	resp.WorkflowData["free_state_reasoning_loop"] = freeStateLoopMap(loop)
	return resp
}

func freeStateEvidenceRefreshReason(resp ChatResponse) (string, bool) {
	if !strings.EqualFold(strings.TrimSpace(resp.Workflow), semanticCompressorExecutionWorkflow) ||
		!strings.EqualFold(firstStringFromMap(resp.WorkflowData, "status"), "materialization_rejected") {
		return "", false
	}
	execution := firstMapFromAny(resp.WorkflowData["execution"])
	detail := firstStringFromMap(execution, "message", "failure_code")
	text := strings.ToLower(firstNonEmpty(resp.Error, detail, resp.StopReason))
	if !strings.Contains(text, "com evidence") && !strings.Contains(text, "paired_io") {
		return "", false
	}
	return firstNonEmpty(detail, resp.Reply, resp.StopReason, "fresh paired COM evidence is required"), true
}

func freeStateMaterializationBlockReason(resp ChatResponse) (string, bool) {
	workflow := strings.ToLower(strings.TrimSpace(resp.Workflow))
	status := strings.ToLower(firstStringFromMap(resp.WorkflowData, "status"))
	blocked := false
	switch workflow {
	case semanticTreatmentWorkflow:
		blocked = status == "capability_boundary" || status == "qualification_failed"
	case semanticCompressorExecutionWorkflow:
		if _, recoverable := freeStateEvidenceRefreshReason(resp); recoverable {
			return "", false
		}
		blocked = status == "materialization_rejected"
	case "capability_runtime_v1":
		blocked = status == "rejected" && strings.EqualFold(firstStringFromMap(resp.WorkflowData, "stage"), "preflight_rejected")
	}
	if !blocked {
		return "", false
	}
	detail := firstStringFromMap(firstMapFromAny(resp.WorkflowData["execution"]), "message", "failure_code")
	reason := firstNonEmpty(resp.Error, detail, resp.Reply, resp.StopReason, "the selected processor path reached a governed materialization boundary")
	return reason, true
}

func (s *Server) maybeContinueFreeStateAfterInteraction(ctx context.Context, interaction PendingInteraction, resp ChatResponse, decision string) ChatResponse {
	loop, ok := s.freeStateLoop(interaction.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(interaction.RequestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
			s.storeFreeStateLoop(loop)
		}
	}
	if !ok || !freeStateLoopActive(loop) {
		return resp
	}
	if isFreeStateCancellation(decision, resp) {
		loop.Status = "cancelled"
		loop.LastError = "user cancelled the pending processor action"
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	if reason, recoverable := freeStateEvidenceRefreshReason(resp); recoverable {
		loop.Status = "awaiting_action"
		loop.DecisionPhase = freeStatePhaseProcessorMaterialization
		loop.LastError = reason
		loop.RequiresPostActionObservation = false
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
		resp.StopReason = "free_state_com_evidence_refresh_required"
		resp.Error = ""
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
			"resumable": true, "free_state_status": "awaiting_evidence_refresh", "mutation_performed": false,
		})
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	processorType, actionStatus, receipt, attempted := freeStateAcousticActionOutcome(interaction, resp)
	if !attempted || resp.NeedsConfirmation || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) {
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	loop.Cycle++
	loop.Actions = append(loop.Actions, freeStateActionRecord{
		Cycle:         loop.Cycle,
		ProcessorType: processorType,
		Workflow:      resp.Workflow,
		Status:        actionStatus,
		Summary:       firstNonEmpty(resp.StopReason, resp.Reply),
		Receipt:       receipt,
		RecordedAt:    time.Now().UTC(),
	})
	loop.RequiresPostActionObservation = actionStatus == "applied"
	loop.Status = "re_evaluating"
	loop.DecisionPhase = freeStatePhaseProcessorSelection
	if loop.RequiresPostActionObservation {
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
	}
	loop.LastError = ""
	if actionStatus != "applied" {
		loop.LastError = firstNonEmpty(resp.Error, resp.StopReason, "processor action failed")
	}
	if loop.MaxCycles <= 0 {
		loop.MaxCycles = freeStateDefaultMaxCycles
	}
	if loop.Cycle >= loop.MaxCycles {
		loop.Status = "blocked"
		loop.LastError = fmt.Sprintf("free-state reasoning reached its %d-action safety limit", loop.MaxCycles)
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
		resp.StopReason = "free_state_cycle_limit"
		resp.Error = ""
		resp.Reply = "The processor reasoning loop reached its bounded action limit. The original intent remains recorded, but no further automatic action was started."
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		if err == nil {
			err = fmt.Errorf("AI configuration is incomplete")
		}
		loop.Status = "blocked"
		loop.LastError = err.Error()
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
		return s.bindFreeStateContextToResponse(resp, interaction.RequestContext)
	}
	resumeContext := mergeContext(interaction.RequestContext, map[string]any{
		"conversation_id":                   interaction.ConversationID,
		"goal_id":                           firstNonEmpty(interaction.GoalID, loop.GoalID),
		"run_id":                            firstNonEmpty(interaction.RunID, loop.RunID),
		"free_state_internal_resume":        true,
		"free_state_reasoning_loop":         freeStateLoopMap(loop),
		"free_state_latest_action_evidence": freeStateActionMap(loop.Actions[len(loop.Actions)-1]),
		"requires_post_action_observation":  loop.RequiresPostActionObservation,
	})
	resumed, handled := s.runAgentLoopChat(ctx, interaction.ConversationID, ChatRequest{
		ConversationID: interaction.ConversationID,
		Message:        loop.OriginalIntent,
		Context:        resumeContext,
	}, cfg)
	if !handled {
		return s.bindFreeStateContextToResponse(resp, resumeContext)
	}
	return resumed
}

func resolvedFreeStateDecisionPhase(loop freeStateReasoningLoop) string {
	if loop.RequiresPostActionObservation {
		return freeStatePhasePostActionEvaluation
	}
	if strings.EqualFold(strings.TrimSpace(loop.Status), "awaiting_action") {
		return freeStatePhaseProcessorMaterialization
	}
	return freeStatePhaseProcessorSelection
}

func freeStateCCBObservation(res agentloop.Result) *agentloop.RecentObservation {
	for index := len(res.Executed) - 1; index >= 0; index-- {
		record := res.Executed[index]
		name := strings.ToLower(firstNonEmpty(firstStringFromMap(record, "tool"), firstStringFromMap(record, "command_name")))
		if name != "ccb.observation_request" && name != "ccb_observation_request" {
			continue
		}
		result := firstMapFromAny(record["result"])
		bundle := firstMapFromAny(result["bundle"])
		if len(bundle) == 0 {
			continue
		}
		status := firstNonEmpty(firstStringFromMap(record, "status"), firstStringFromMap(result, "status"), firstStringFromMap(bundle, "status"))
		return &agentloop.RecentObservation{
			ToolCallID:  firstStringFromMap(record, "tool_call_id"),
			Tool:        firstNonEmpty(firstStringFromMap(record, "tool"), "ccb.observation_request"),
			CommandName: firstStringFromMap(record, "command_name"),
			Status:      status,
			Error:       firstStringFromMap(record, "error"),
			Summary:     cloneContext(bundle),
		}
	}
	return nil
}

func isFreeStateCancellation(decision string, resp ChatResponse) bool {
	clean := strings.ToLower(strings.TrimSpace(decision))
	return clean == "cancel" || clean == "deny" || clean == "reject" ||
		resp.GoalStatus == string(agentruntime.StatusCancelled)
}

func freeStateAcousticActionOutcome(interaction PendingInteraction, resp ChatResponse) (string, string, map[string]any, bool) {
	workflow := firstNonEmpty(resp.Workflow, interaction.Workflow)
	switch {
	case strings.EqualFold(workflow, semanticCompressorExecutionWorkflow):
		status := firstStringFromMap(resp.WorkflowData, "status")
		if status == "executed" {
			return "compressor", "applied", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
		if status == "failed" || strings.TrimSpace(resp.Error) != "" {
			return "compressor", "failed", cloneContext(firstMapFromAny(resp.WorkflowData["execution_receipt"])), true
		}
	case strings.EqualFold(workflow, "capability_runtime_v1") &&
		firstStringFromMap(resp.WorkflowData, "capability_id") == agentSemanticEQCapabilityID:
		for _, receipt := range freeStateMapRows(resp.WorkflowData["receipts"]) {
			if strings.EqualFold(firstStringFromMap(receipt, "status"), "applied") {
				out := cloneContext(receipt)
				if verification := resp.WorkflowData["verification"]; verification != nil {
					out["verification"] = verification
				}
				return "eq", "applied", out, true
			}
		}
		if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
			return "eq", "failed", cloneContext(resp.WorkflowData), true
		}
	}
	return "", "", nil, false
}

func freeStateLoopFromAny(value any) (freeStateReasoningLoop, bool) {
	if typed, ok := value.(freeStateReasoningLoop); ok {
		return cloneFreeStateLoop(typed), typed.SchemaVersion == freeStateReasoningLoopSchema
	}
	data, err := json.Marshal(value)
	if err != nil || string(data) == "null" {
		return freeStateReasoningLoop{}, false
	}
	loop := freeStateReasoningLoop{}
	if err := json.Unmarshal(data, &loop); err != nil || loop.SchemaVersion != freeStateReasoningLoopSchema {
		return freeStateReasoningLoop{}, false
	}
	return loop, true
}

func freeStateLoopMap(loop freeStateReasoningLoop) map[string]any {
	data, _ := json.Marshal(loop)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func freeStateActionMap(action freeStateActionRecord) map[string]any {
	data, _ := json.Marshal(action)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func freeStateMapRows(value any) []map[string]any {
	data, _ := json.Marshal(value)
	rows := []map[string]any{}
	_ = json.Unmarshal(data, &rows)
	return rows
}

func cloneFreeStateLoop(loop freeStateReasoningLoop) freeStateReasoningLoop {
	data, _ := json.Marshal(loop)
	out := freeStateReasoningLoop{}
	_ = json.Unmarshal(data, &out)
	return out
}
