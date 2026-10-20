package chat

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FULLACCESS-SEMANTIC-CHAIN-1 (card 147): under an explicit full_project_access
// turn the semantic processing chain — recommendation, plugin load, typed
// controller execution — runs autonomously end to end. The user's ruling
// (2026-09-13, quoted verbatim in agent/internal/agentloop/full_access_autonomy.go):
// 完全访问权限应该是它能够自主进行所有执行，我只负责AB试听. The three confirmation
// faces this file bridges used to park interaction cards even in that mode
// (M2 experience session 1, 2026-10-20: plugin_selection_required, the load
// confirmation, and semantic_dynamic_confirmation_required all reached the
// user), which the autonomy contract calls a behavioral defect, not a courtesy.
//
// Fork posture, mirrored on the 2026-09-11 mix_tick precedent
// (improvement_proposal_workflow.go: full access applies the admitted proposal
// immediately instead of issuing a confirmation card):
//
//   - every fork requires the explicit pair the harness gate reads
//     (explicitFullAccessRequestContext: authority_mode=full_project_access +
//     authority_mode_explicit) — a bare mode string is never a grant;
//   - the fork only decides WHO answers the card; it never lowers a boundary.
//     The load still passes pendingPlanExecutionContext (the same
//     non-forgeable PCA authorization a user confirm provides) and the harness
//     PCA load gate; the typed controller still re-runs PCA admission, the
//     topology-generation and control-ref staleness checks, and the atomic
//     readback validation;
//   - a pending user stop is honored before each mutating leg (FS-STOP-APPLY-1
//     Part A: this chain runs after the message loop returned, outside the
//     runner's before/after-tool checkpoints, so the stop must be re-read
//     here);
//   - the A/B audition remains the only user face, per the same ruling.
//
// manual_confirmation turns never enter these helpers: their three faces keep
// byte-identical behavior (parked card + waiting status), pinned by
// full_access_semantic_chain_test.go.

// fullAccessSemanticChainContext reports whether this request context is both
// an explicit full-access turn and the plugin-recommendation semantic chain the
// card covers. The chain key matters on the typed-controller face: C2
// capability sessions and the B4 semantic-treatment faces call the same
// planner with a chat context that may inherit the authority keys; they own
// their confirmation surfaces, so only the chain the user's M2 session
// actually traversed (selection → load → typed controller) forks here.
func fullAccessSemanticChainContext(requestContext map[string]any) bool {
	return explicitFullAccessRequestContext(requestContext) && boolValue(requestContext["semantic_plugin_recommendation_selection"])
}

// fullAccessStopPendingSkippedResponse is the honest skip for a chain leg whose
// user stop arrived mid-chain: already-applied work keeps its bounded
// reversible semantics, but no new intervention fires after the stop.
func fullAccessStopPendingSkippedResponse(conversationID, goalID, runID, workflow, stage string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply:               "已停止当前 Turn：停止请求之后到达的自主执行腿没有执行，本轮没有新的干预；已应用的调整保持可回滚。",
		Workflow:            workflow,
		WorkflowData:        map[string]any{"full_access_auto_authorized": true, "stop_turn": true, "stage": stage, "intervention_skipped": true, "skip_reason": "user_stop_pending", "mutation_performed": false},
		GoalStatus:          string(agentruntime.StatusStopped),
		StopReason:          stopPendingProposalSkippedReason,
		NeedsConfirmation:   false,
		InteractionRequests: nil,
	}
}

// pluginRecommendationFullAccessSelectionResponse replaces the parked
// plugin-selection card with an autonomous choice: the plan's primary
// recommendation (the model-ranked 主推荐, which is also the single candidate
// when the admitted set has exactly one). The choice then continues through
// the same governed continuation a user selection takes
// (continuePluginRecommendationInteraction), so PCA recency re-verification,
// target binding, and the AGENT-1 A0 selection records behave identically.
func (s *Server) pluginRecommendationFullAccessSelectionResponse(conversationID, mode string, requestContext map[string]any, res agentloop.Result, plan pluginRecommendationPlan, candidates []pluginRecommendationCandidate) ChatResponse {
	if len(plan.Choices) == 0 {
		// Defensive: a validated plan always carries choices; if not, keep the
		// ordinary waiting face rather than inventing an autonomy move.
		return s.pluginRecommendationSelectionResponse(conversationID, mode, requestContext, res, plan, candidates)
	}
	candidateByKey := map[string]pluginRecommendationCandidate{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	primary := plan.Choices[0]
	candidate := candidateByKey[primary.CandidateKey]
	selected := pluginRecommendationChoiceRow(primary, candidate)
	// The synthetic interaction mirrors exactly what the parked card would
	// carry, so the continuation cannot tell an autonomous selection from a
	// user's; nothing is stored and no interaction request is emitted.
	interaction := PendingInteraction{
		ID: "interaction_fullaccess_" + randomID(), Kind: "plugin_recommendation_question", Type: "plugin_recommendation_selection",
		Source: "plugin_recommendation", Workflow: pluginRecommendationWorkflow, Stage: "awaiting_selection",
		ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID,
		RequestContext: cloneContext(requestContext),
		Payload: map[string]any{
			"schema_version": pluginRecommendationSchema, "status": "awaiting_selection",
			"processor_type": plan.ProcessorType, "listening_goal": plan.UserGoal, "summary": plan.Summary,
			"target_ref": map[string]any{
				"kind":  "track",
				"id":    firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id"),
				"label": firstStringFromMap(requestContext, "selected_track_name"),
			},
			"recommendations": []map[string]any{selected},
			"request_context": cloneContext(requestContext),
		},
	}
	for key, value := range map[string]any{
		"post_load_goal":                      firstNonEmpty(firstStringFromMap(requestContext, "semantic_post_load_goal", "semantic_eq_post_load_goal", "semantic_compressor_post_load_goal"), plan.UserGoal),
		"post_load_observation_context":       semanticTreatmentObservationPayload(res.RecentObservation),
		"post_load_semantic_processor_intent": cloneContext(firstMapFromAny(requestContext["free_state_semantic_processor_intent"])),
	} {
		if value != nil {
			interaction.Payload[key] = value
		}
	}
	if family, planner, ok := semanticPostLoadAdapterForProcessorType(plan.ProcessorType); ok {
		interaction.Payload["post_load_family"] = family
		interaction.Payload["post_load_planner"] = planner
	}
	if boolValue(requestContext["semantic_eq_post_load_handoff"]) && plan.ProcessorType == "eq" {
		interaction.Payload["post_load_planner"] = "semantic_eq"
	} else if boolValue(requestContext["semantic_compressor_post_load_handoff"]) && plan.ProcessorType == "compressor" {
		interaction.Payload["post_load_planner"] = "semantic_compressor"
	}
	goalID, runID := goalIDsFromContext(requestContext)
	if goalID == "" {
		goalID = res.GoalID
	}
	if runID == "" {
		runID = res.RunID
	}
	if s.goalStopPending(goalID) {
		return fullAccessStopPendingSkippedResponse(conversationID, goalID, runID, pluginRecommendationWorkflow, "plugin_selection")
	}
	// context.Background() matches the accepted full-access inline-continuation
	// precedent (routeAcceptedImprovementProposal): the chain is synchronous
	// server work the turn already owns, not a request-bound read.
	resp := s.continuePluginRecommendationInteraction(context.Background(), interaction, "select_"+primary.CandidateKey)
	resp.NeedsConfirmation = false
	resp.InteractionRequests = nil
	resp.Reply = fmt.Sprintf("完全访问权限下我自主选择了主推荐：%s——%s。",
		firstNonEmpty(candidate.Name, primary.CandidateKey), strings.TrimSpace(firstNonEmpty(primary.Reason, plan.Summary))) +
		pluginRecommendationAdmissionBoundaryReplySuffix(requestContext) + "\n" + resp.Reply
	resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
		"authority_mode": authorityModeFull, "full_access_auto_authorized": true,
		"recommendation_schema_version": pluginRecommendationSchema,
		"selected_candidate":            selected,
		"selection_performed":           true,
		"selection_owner":               "full_project_access",
		"interaction_card_issued":       false,
	})
	return resp
}

// pluginGrabberLoadFullAccessResponse replaces the parked load confirmation
// with the immediate execution a user confirm would have performed: the same
// pendingPlanExecutionContext authorization, the same executeDecisions
// confirmed dispatch (the harness PCA load gate still runs unchanged), the
// same parameter grab and post-load semantic handoff as handleConfirm.
func (s *Server) pluginGrabberLoadFullAccessResponse(ctx context.Context, conversationID string, plan PendingPlan) ChatResponse {
	goalID, runID := goalIDsFromContext(plan.Context)
	if s.goalStopPending(goalID) {
		return fullAccessStopPendingSkippedResponse(conversationID, goalID, runID, pluginGrabberLoadCommand, "plugin_load")
	}
	executionContext, err := pendingPlanExecutionContext(plan)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply: "完全访问装载被 PCA 授权核对拒绝，因此没有加载插件或修改工程。", Error: err.Error(),
			Workflow: pluginGrabberLoadCommand, WorkflowData: mergeContext(plan.WorkflowData, map[string]any{"full_access_auto_authorized": true, "mutation_performed": false}),
			GoalStatus: string(agentruntime.StatusFailed), StopReason: "full_access_load_authorization_failed", NeedsConfirmation: false,
		}
	}
	beforeState := s.harness.UserStateSummary(ctx)
	replies, err := s.executeDecisions(ctx, plan.Decisions, true, executionContext)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
			Reply: firstNonEmpty(friendlyExecutionError(err), err.Error()), Error: err.Error(),
			Workflow: pluginGrabberLoadCommand, WorkflowData: mergeContext(plan.WorkflowData, map[string]any{"full_access_auto_authorized": true, "mutation_performed": len(replies) > 0}),
			GoalStatus: string(agentruntime.StatusFailed), StopReason: "full_access_load_execution_failed",
			ExecutedKernelReply: compactAgentLoopExecutedForResponse(replies), NeedsConfirmation: false,
		}
	}
	message := executedReply(beforeState, s.harness.UserStateSummary(ctx), plan.Decisions, replies)
	message, replies = s.finishPluginGrabberLoadWorkflow(ctx, plan, replies, message)
	if handoff, ok := s.semanticProcessorPostLoadHandoff(ctx, plan, replies); ok {
		// attachInteractionRequests is deliberately NOT called here: under full
		// access this chain issues no confirmation card, and that helper
		// synthesizes one from any waiting-confirmation shape.
		handoff.Reply = "完全访问权限下装载已自主执行（无确认卡）。\n" + handoff.Reply
		handoff.WorkflowData = mergeContext(handoff.WorkflowData, map[string]any{
			"full_access_load_executed": true,
			"load_plugin_name":          firstStringFromMap(plan.WorkflowData, "plugin_name"),
			"load_plugin_path":          firstStringFromMap(plan.WorkflowData, "plugin_path"),
			"load_track_id":             firstStringFromMap(plan.WorkflowData, "track_id"),
		})
		if len(handoff.ExecutedKernelReply) == 0 {
			handoff.ExecutedKernelReply = compactAgentLoopExecutedForResponse(replies)
		}
		return handoff
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply:    "完全访问权限下装载已自主执行（无确认卡）。\n" + message,
		Workflow: pluginGrabberLoadCommand,
		WorkflowData: mergeContext(plan.WorkflowData, map[string]any{
			"authority_mode": authorityModeFull, "full_access_auto_authorized": true,
			"mutation_performed": true, "interaction_card_issued": false,
		}),
		GoalStatus:          string(agentruntime.StatusCompleted),
		ExecutedKernelReply: compactAgentLoopExecutedForResponse(replies),
		NeedsConfirmation:   false,
		InteractionRequests: nil,
	}
}

// semanticDynamicFullAccessExecution replaces the parked typed-controller
// confirmation with the immediate execution an approve would have performed.
// It replays the approve-path progressive-disclosure confirm step and then
// calls the same executor, so PCA admission, topology staleness, control-ref
// binding, atomic readback, and the settlement bracket are all unchanged.
func (s *Server) semanticDynamicFullAccessExecution(ctx context.Context, conversationID string, requestContext map[string]any, ticket semanticDynamicTicket) ChatResponse {
	goalID, runID := goalIDsFromContext(requestContext)
	if s.goalStopPending(goalID) {
		return fullAccessStopPendingSkippedResponse(conversationID, goalID, runID, semanticDynamicWorkflow, "typed_controller")
	}
	if _, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); ok {
		if err := semanticProgressiveDisclosureConfirm(requestContext); err != nil {
			return semanticDynamicFailure(conversationID, requestContext, "progressive_disclosure_confirmation_invalid", err)
		}
	}
	interaction := PendingInteraction{
		ConversationID: conversationID, GoalID: goalID, RunID: runID,
		RequestContext: cloneContext(requestContext),
	}
	resp := s.executeSemanticDynamicTicket(ctx, interaction, ticket)
	if _, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); ok {
		if receipt := firstMapFromAny(resp.WorkflowData["execution_receipt"]); len(receipt) > 0 {
			if err := semanticProgressiveDisclosureReceipt(requestContext, receipt); err != nil {
				return semanticDynamicFailure(conversationID, requestContext, "progressive_disclosure_receipt_invalid", err)
			}
			resp.WorkflowData["semantic_progressive_disclosure"] = requestContext["semantic_progressive_disclosure"]
			resp.WorkflowData["request_context"] = cloneContext(requestContext)
		}
	}
	resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
		"authority_mode": authorityModeFull, "full_access_auto_authorized": true,
		"confirmation_bypassed": "full_project_access", "interaction_card_issued": false,
	})
	resp.NeedsConfirmation = false
	resp.InteractionRequests = nil
	return resp
}
