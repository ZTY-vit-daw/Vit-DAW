package chat

import (
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
)

const orchestrationOwnerContextKey = "orchestration_controller_owner"

func (s *Server) hasActiveOrchestrationController(conversationID string) bool {
	if s == nil || s.controllerOwners == nil {
		return false
	}
	_, ok := s.controllerOwners.Active(conversationID)
	return ok
}

func (s *Server) bindActiveOrchestrationController(conversationID string, requestContext map[string]any) map[string]any {
	if s == nil || s.controllerOwners == nil {
		return requestContext
	}
	owner, ok := s.controllerOwners.Active(conversationID)
	if !ok {
		return requestContext
	}
	decision := orchestrationDecisionForOwner(owner)
	if routed, ok := orchestrationControllerDecisionFromContext(requestContext); ok {
		decision = routed
	}
	out := mergeContext(requestContext, map[string]any{
		orchestrationDecisionContextKey: orchestrationControllerDecisionMap(decision),
		orchestrationOwnerContextKey:    orchestrationControllerOwnerMap(owner),
	})
	// Recovered owner state is host-trusted. Reconstruct semantic entry after
	// untrusted transport fields were removed so continuation does not depend
	// on the client echoing a previous response envelope.
	entry := semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Controller: string(owner.Controller), TargetScope: owner.TargetScope,
		Confidence: 1, Reason: "restored orchestration controller owner",
	}
	switch owner.Controller {
	case orchestrationcontroller.MinimalAudioClosure:
		entry.Route, entry.ControlMode, entry.UserAuthorization = semanticEntryRouteOpenSemantic, semanticEntryControlSemanticLoop, semanticEntryAuthorizationAction
		if s.audioClosures != nil {
			if closure, exists := s.audioClosures.ActiveForConversation(conversationID); exists && closure.Mode == "diagnostic" {
				entry.Route, entry.ControlMode, entry.UserAuthorization = semanticEntryRouteObservation, semanticEntryControlObserveOnly, semanticEntryAuthorizationObserve
				decision.Authorization, decision.SourceRoute = semanticEntryAuthorizationObserve, semanticEntryRouteObservation
				out[orchestrationDecisionContextKey] = orchestrationControllerDecisionMap(decision)
			}
		}
	case orchestrationcontroller.ProjectMixWorkflow:
		entry.Route, entry.ControlMode, entry.UserAuthorization = semanticEntryRouteOpenSemantic, semanticEntryControlSemanticLoop, semanticEntryAuthorizationAction
	default:
		return out
	}
	if route := s.previousCapabilityRoute("", conversationID); route.SchemaVersion == capabilityRouteSchema {
		semantic := route.SemanticEntry
		entry.Route = firstNonEmpty(firstStringFromMap(semantic, "route"), entry.Route)
		entry.TargetScope = firstNonEmpty(firstStringFromMap(semantic, "target_scope"), entry.TargetScope)
		entry.ControlMode = firstNonEmpty(firstStringFromMap(semantic, "control_mode"), entry.ControlMode)
		entry.UserAuthorization = firstNonEmpty(firstStringFromMap(semantic, "user_authorization"), entry.UserAuthorization)
		entry.Reason = firstNonEmpty(firstStringFromMap(semantic, "reason"), entry.Reason)
		decision.SourceRoute, decision.Authorization = entry.Route, entry.UserAuthorization
		out[orchestrationDecisionContextKey] = orchestrationControllerDecisionMap(decision)
		out = contextWithCapabilityRoute(out, route)
	}
	out[semanticEntryVerifiedContextKey] = true
	out[semanticEntryDecisionContextKey] = semanticEntryDecisionMap(entry)
	return out
}

func (s *Server) prepareProjectMixController(conversationID string, requestContext map[string]any) (map[string]any, orchestrationcontroller.Owner, bool, error) {
	decision, ok := orchestrationControllerDecisionFromContext(requestContext)
	if !ok || decision.Controller != orchestrationcontroller.ProjectMixWorkflow {
		return requestContext, orchestrationcontroller.Owner{}, false, nil
	}
	if s.controllerOwners == nil {
		s.controllerOwners = orchestrationcontroller.NewRegistry()
	}
	if owner, active := s.controllerOwners.Active(conversationID); active {
		if owner.Controller != orchestrationcontroller.ProjectMixWorkflow {
			return requestContext, owner, false, fmt.Errorf("conversation is already owned by %s", owner.Controller)
		}
		return bindProjectMixControllerContext(requestContext, decision, owner), owner, true, nil
	}
	owner, _, err := s.controllerOwners.Acquire(conversationID, "project_mix_"+randomID(), decision, time.Now().UTC())
	if err != nil {
		return requestContext, orchestrationcontroller.Owner{}, false, err
	}
	s.persistCurrentProjectWorkspace()
	return bindProjectMixControllerContext(requestContext, decision, owner), owner, true, nil
}

func (s *Server) projectMixControllerResponse(resp ChatResponse, owner orchestrationcontroller.Owner, res agentloop.Result) ChatResponse {
	if s == nil {
		return resp
	}
	terminal := res.Status == agentruntime.StatusCompleted || res.Status == agentruntime.StatusFailed || res.Status == agentruntime.StatusCancelled || res.Continuation == nil
	if terminal && s.controllerOwners != nil {
		if current, ok := s.controllerOwners.Active(owner.ConversationID); ok && current.ControllerID == owner.ControllerID {
			reason := firstNonEmpty(res.StopReason, string(res.Status), "settled")
			if settled, err := s.controllerOwners.Settle(owner.ConversationID, owner.ControllerID, current.Revision, reason, time.Now().UTC()); err == nil {
				owner = settled
				s.persistCurrentProjectWorkspace()
			}
		}
	}
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData[orchestrationOwnerContextKey] = orchestrationControllerOwnerMap(owner)
	resp.WorkflowData[orchestrationDecisionContextKey] = orchestrationControllerDecisionMap(orchestrationDecisionForOwner(owner))
	if strings.TrimSpace(resp.Workflow) == "" {
		resp.Workflow = "project_mix_workflow"
	}
	return resp
}

// ProjectMixWorkflow v1 must be registered as a governed controller before it
// can receive continuation. The disabled legacy MixSession is deliberately not
// treated as that registration and ordinary MessageLoop is never a fallback.
func projectMixWorkflowV1Available(_ map[string]any) bool {
	return false
}

func (s *Server) projectMixUnavailableResponse(conversationID, mode string, owner orchestrationcontroller.Owner, requestContext map[string]any) ChatResponse {
	if s != nil && s.controllerOwners != nil {
		if current, ok := s.controllerOwners.Active(conversationID); ok && current.ControllerID == owner.ControllerID {
			if settled, err := s.controllerOwners.Settle(conversationID, owner.ControllerID, current.Revision, "capability_unavailable", time.Now().UTC()); err == nil {
				owner = settled
				s.persistCurrentProjectWorkspace()
			}
		}
	}
	decision := orchestrationDecisionForOwner(owner)
	if routed, ok := orchestrationControllerDecisionFromContext(requestContext); ok {
		decision = routed
	}
	entryPlan := firstMapFromAny(requestContext[capabilityEntryPlanContextKey])
	stage := firstNonEmpty(firstStringFromMap(entryPlan, "stage"), "A2")
	capabilityID := firstNonEmpty(firstStringFromMap(entryPlan, "capability_id"), "project_prep.technical_integrity.v0")
	goalID := firstStringFromMap(requestContext, "goal_id")
	if s != nil && s.harness != nil && goalID != "" {
		s.harness.SetGoalStatus(goalID, agentruntime.StatusFailed, fmt.Errorf("project mix capability runtime is unavailable"))
		s.persistCurrentProjectWorkspace()
	}
	return ChatResponse{
		ConversationID: conversationID, AgentMode: mode,
		TaskID: firstStringFromMap(requestContext, "task_id"), GoalID: goalID, RunID: firstStringFromMap(requestContext, "run_id"),
		OriginalIntent: firstStringFromMap(requestContext, "original_intent"),
		Reply:          fmt.Sprintf("工程结构容量评估已选择全工程固定混音能力层，入口为 %s（%s）；当前运行时尚未注册可取得执行权的 ProjectMixWorkflow v1，因此停在能力边界，没有降级到自由态，也没有修改工程。", stage, capabilityID),
		GoalStatus:     string(agentruntime.StatusFailed), StopReason: "capability_unavailable",
		Workflow: "project_mix_workflow", WorkflowData: map[string]any{
			"status": "capability_unavailable", "mutation_performed": false,
			capacityAssessmentContextKey: requestContext[capacityAssessmentContextKey], capabilityEntryPlanContextKey: requestContext[capabilityEntryPlanContextKey],
			capabilityRouteContextKey:    requestContext[capabilityRouteContextKey],
			orchestrationOwnerContextKey: orchestrationControllerOwnerMap(owner), orchestrationDecisionContextKey: orchestrationControllerDecisionMap(decision),
		},
	}
}

func bindProjectMixControllerContext(requestContext map[string]any, decision orchestrationcontroller.Decision, owner orchestrationcontroller.Owner) map[string]any {
	return mergeContext(requestContext, map[string]any{
		orchestrationDecisionContextKey: orchestrationControllerDecisionMap(decision),
		orchestrationOwnerContextKey:    orchestrationControllerOwnerMap(owner),
		"project_mix_workflow_selected": true,
	})
}

func orchestrationDecisionForOwner(owner orchestrationcontroller.Owner) orchestrationcontroller.Decision {
	decision := orchestrationcontroller.Decision{
		SchemaVersion: orchestrationcontroller.DecisionSchema, Controller: owner.Controller,
		TargetScope: owner.TargetScope, Reason: "persistent controller owner",
	}
	switch owner.Controller {
	case orchestrationcontroller.ProjectMixWorkflow, orchestrationcontroller.MinimalAudioClosure:
		decision.Authorization, decision.SourceRoute = semanticEntryAuthorizationAction, semanticEntryRouteOpenSemantic
	default:
		decision.Authorization, decision.SourceRoute = semanticEntryAuthorizationNone, semanticEntryRouteOther
	}
	return decision
}

func orchestrationControllerOwnerMap(owner orchestrationcontroller.Owner) map[string]any {
	return map[string]any{
		"conversation_id": owner.ConversationID, "controller_id": owner.ControllerID, "controller": string(owner.Controller),
		"target_scope": owner.TargetScope, "status": string(owner.Status), "revision": owner.Revision,
		"settlement_reason": owner.SettlementReason, "created_at": owner.CreatedAt, "updated_at": owner.UpdatedAt,
	}
}
