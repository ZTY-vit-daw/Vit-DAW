package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/config"
	agentruntime "vit-daw-agent/internal/runtime"
)

const improvementProposalWorkflow = "improvement_proposal"

func (s *Server) improvementProposalResponse(conversationID, mode string, res agentloop.Result, requestContext map[string]any) ChatResponse {
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	proposal := res.FreeStateDecision.ImprovementProposal
	if proposal == nil {
		return resp
	}
	candidate := proposal.ToPendingCandidate(conversationID, res.GoalID, res.RunID, "")
	s.upsertPendingCandidate(candidate)
	resp.Reply = firstNonEmpty(resp.Reply, "已形成一条基于证据的改善性提案，等待确认后进入对应执行域。")
	resp.NeedsConfirmation = true
	resp.GoalStatus = string(agentruntime.StatusWaitingConfirmation)
	resp.StopReason = "improvement_proposal_confirmation_required"
	resp.Workflow = improvementProposalWorkflow
	resp.WorkflowData = typedPendingPayload(map[string]any{
		"schema_version":     agentprotocol.ImprovementProposalSchema,
		"status":             "waiting_user",
		"mutation_performed": false,
		"proposal":           agentprotocol.ToMap(proposal),
		"request_context":    requestContext,
	}, candidate)
	resp.TypedEvents = append(resp.TypedEvents, agentprotocol.ToMap(agentprotocol.NewEvent(candidate, candidate.Source)))
	req := improvementProposalInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, candidate, requestContext)
	resp.InteractionRequests = []AgentInteractionRequest{req}
	s.storePendingInteraction(req, req.Payload)
	return resp
}

func improvementProposalInteractionRequest(conversationID, goalID, runID, reply string, candidate agentprotocol.PendingCandidate, requestContext map[string]any) AgentInteractionRequest {
	payload := typedPendingPayload(map[string]any{
		"schema_version":  agentprotocol.ImprovementProposalSchema,
		"status":          "waiting_user",
		"proposal":        candidate.CandidateAction,
		"request_context": cloneContext(requestContext),
	}, candidate)
	payload["conversation_id"] = conversationID
	return AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "improvement_proposal_confirmation",
		Type:           "improvement_proposal_confirmation",
		Source:         "vit_agent",
		Workflow:       improvementProposalWorkflow,
		Stage:          "pending_confirmation",
		Title:          "改善性提案待确认",
		Body:           firstNonEmpty(strings.TrimSpace(reply), "这是一条可逆、受边界约束的改善性提案。确认后才会进入对应执行域。"),
		Status:         "waiting_for_user",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "确认提案", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
}

func (s *Server) continueImprovementProposalInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	confirmation := actionworkflow.ClassifyConfirmation(decision, true)
	status := agentprotocol.PendingStatusAccepted
	reply := "提案已确认，当前尚未修改工程；正在把它交给对应 action domain 的受控入口。"
	goalStatus := string(agentruntime.StatusWaitingContinue)
	stopReason := "improvement_proposal_accepted"
	if confirmation.Kind == actionworkflow.DecisionReject || strings.EqualFold(strings.TrimSpace(decision), "cancel") {
		status = agentprotocol.PendingStatusRejected
		reply = "已取消这条改善性提案，工程没有被修改。"
		goalStatus = string(agentruntime.StatusCancelled)
		stopReason = "improvement_proposal_rejected"
	}
	s.transitionActivePendingCandidate(interaction.ConversationID, "improvement_proposal", status, stopReason)
	if status == agentprotocol.PendingStatusAccepted {
		if response, routed := s.routeAcceptedImprovementProposal(ctx, interaction); routed {
			return response
		}
	}
	return ChatResponse{
		ConversationID:    interaction.ConversationID,
		GoalID:            interaction.GoalID,
		RunID:             interaction.RunID,
		Reply:             reply,
		NeedsConfirmation: false,
		Workflow:          improvementProposalWorkflow,
		GoalStatus:        goalStatus,
		StopReason:        stopReason,
		WorkflowData: map[string]any{
			"schema_version":     agentprotocol.ImprovementProposalSchema,
			"status":             status,
			"mutation_performed": false,
			"next_stage":         "action_domain_router",
			"pending_candidate":  interaction.Payload,
		},
	}
}

// routeAcceptedImprovementProposal is deliberately an adapter, not a second
// executor. It restores the durable free-state context and hands the accepted
// hypothesis to the existing semantic treatment router. That router remains
// responsible for PCA/topology qualification, parameter planning, a fresh
// execution confirmation, Typed Controller mutation, readback, and rollback.
func (s *Server) routeAcceptedImprovementProposal(ctx context.Context, interaction PendingInteraction) (ChatResponse, bool) {
	proposal, err := improvementProposalFromInteraction(interaction)
	if err != nil {
		return improvementProposalBoundaryResponse(interaction, "已确认，但提案载荷无法通过协议校验；没有修改工程。", "improvement_proposal_payload_invalid"), true
	}
	requestContext := cloneContext(interaction.RequestContext)
	if len(requestContext) == 0 {
		requestContext = cloneContext(firstMapFromAny(interaction.Payload["request_context"]))
	}
	requestContext = s.bindFreeStateAuthoritativeTrack(interaction.ConversationID, requestContext)
	loop, ok := s.freeStateLoop(interaction.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
		}
	}
	if !ok || !freeStateLoopActive(loop) || loop.LatestObservation == nil {
		return improvementProposalBoundaryResponse(interaction, "已确认，但缺少可恢复的原始观察上下文；没有修改工程。", "improvement_proposal_observation_context_missing"), true
	}
	if response, routed := s.routeAcceptedImprovementProposalNativeDomain(interaction, proposal, requestContext); routed {
		return response, true
	}
	processorType := improvementProposalProcessorType(proposal)
	if processorType == "" {
		return improvementProposalBoundaryResponse(interaction, "已确认，但该 action domain 尚未携带可治理的 processor family；没有修改工程。", "improvement_proposal_processor_family_missing"), true
	}
	requestContext = mergeContext(requestContext, map[string]any{
		"free_state_route_authorized": true,
		"free_state_internal_resume":  true,
		"free_state_processor_type":   processorType,
		// These are planning notes, not a second user-confirmation gate. The
		// governed router owns qualification and parameter resolution after
		// the proposal has been accepted.
		"improvement_proposal_needs_resolution": append([]string(nil), proposal.NeedsResolution...),
		"goal_id":                               firstNonEmpty(interaction.GoalID, loop.GoalID),
		"run_id":                                firstNonEmpty(interaction.RunID, loop.RunID),
	})
	res := agentloop.Result{
		GoalID:            firstNonEmpty(interaction.GoalID, loop.GoalID),
		RunID:             firstNonEmpty(interaction.RunID, loop.RunID),
		GoalSummary:       firstNonEmpty(proposal.ImprovementIntent, loop.OriginalIntent),
		RecentObservation: loop.LatestObservation,
	}
	cfg, _, cfgErr := config.Load()
	if cfgErr != nil || !cfg.Complete() {
		return improvementProposalBoundaryResponse(interaction, "已确认，但当前执行规划配置不可用；没有修改工程。", "improvement_proposal_config_unavailable"), true
	}
	response, routed := s.routeOrdinaryAgentTreatmentStrategy(ctx, interaction.ConversationID, agentModeFromContext(requestContext), proposal.ImprovementIntent, requestContext, res, cfg)
	if !routed {
		return improvementProposalBoundaryResponse(interaction, "已确认，但 action domain 没有可用的受治理入口；没有修改工程。", "improvement_proposal_router_unavailable"), true
	}
	response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{
		"improvement_proposal":         agentprotocol.ToMap(proposal),
		"action_domain_router":         true,
		"proposal_confirmation":        "accepted",
		"mutation_performed":           boolValue(response.WorkflowData["mutation_performed"]),
		"proposal_execution_authority": "existing_governed_router",
	})
	return response, true
}

// routeAcceptedImprovementProposalNativeDomain assigns non-plug-in domains to
// their existing typed tools. The proposal confirmation approves the bounded
// experiment direction; a concrete mix tick remains separately confirmable
// before it can write project state.
func (s *Server) routeAcceptedImprovementProposalNativeDomain(interaction PendingInteraction, proposal agentprotocol.ImprovementProposal, requestContext map[string]any) (ChatResponse, bool) {
	domain := strings.ToLower(strings.TrimSpace(proposal.ActionDomain))
	if domain != agentprotocol.ImprovementActionDomainTrackGain && domain != agentprotocol.ImprovementActionDomainPan {
		return ChatResponse{}, false
	}
	trackID := improvementProposalTrackID(proposal)
	if trackID == "" {
		return improvementProposalBoundaryResponse(interaction, "已确认，但原生控制域缺少明确的轨道目标；没有修改工程。", "improvement_proposal_native_target_missing"), true
	}
	candidate := agentloop.PendingMixTickCandidate{
		TrackID:                   trackID,
		ObservationID:             firstNonEmpty(proposal.EvidenceRefs...),
		Evidence:                  map[string]any{"source": "improvement_proposal", "action_domain": domain, "action_kind": proposal.ActionKind, "evidence_refs": append([]string(nil), proposal.EvidenceRefs...)},
		Fingerprint:               map[string]any{"source": "improvement_proposal", "target_scope": "selected_track"},
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
	}
	switch domain {
	case agentprotocol.ImprovementActionDomainTrackGain:
		deltaDB, ok := treatmentNumber(proposal.ParameterBounds, "delta_db", "db_delta", "gain_delta_db")
		if !ok || deltaDB == 0 || math.Abs(deltaDB) > 2 {
			return improvementProposalBoundaryResponse(interaction, "已确认改善方向，但轨道电平工具需要一个非零且不超过 +/-2 dB 的明确 delta_db；没有修改工程。", "improvement_proposal_track_gain_bounds_missing"), true
		}
		candidate.Operation = "track_gain_adjust"
		candidate.DeltaDB = deltaDB
	case agentprotocol.ImprovementActionDomainPan:
		if targetPan, ok := treatmentNumber(proposal.ParameterBounds, "target_pan", "pan"); ok {
			if targetPan < -1 || targetPan > 1 {
				return improvementProposalBoundaryResponse(interaction, "已确认改善方向，但声像工具要求 target_pan 位于 -1 到 +1；没有修改工程。", "improvement_proposal_pan_bounds_invalid"), true
			}
			candidate.Operation = "track_pan_set"
			candidate.TargetPan = &targetPan
		} else if deltaPan, ok := treatmentNumber(proposal.ParameterBounds, "delta_pan", "pan_delta"); ok && deltaPan != 0 && math.Abs(deltaPan) <= 0.15 {
			candidate.Operation = "track_pan_adjust"
			candidate.DeltaPan = deltaPan
		} else {
			return improvementProposalBoundaryResponse(interaction, "已确认改善方向，但声像工具需要一个非零且不超过 +/-0.15 的 delta_pan，或 -1 到 +1 的 target_pan；没有修改工程。", "improvement_proposal_pan_bounds_missing"), true
		}
	}

	s.storePendingMixTickCandidate(interaction.ConversationID, interaction.GoalID, interaction.RunID, candidate)
	typed := candidate.ToPendingCandidate(interaction.ConversationID, interaction.GoalID, interaction.RunID, "")
	request := mixTickInteractionRequest(interaction.ConversationID, interaction.GoalID, interaction.RunID, candidate)
	s.storePendingInteraction(request, request.Payload)
	return ChatResponse{
		ConversationID:    interaction.ConversationID,
		GoalID:            interaction.GoalID,
		RunID:             interaction.RunID,
		Reply:             "改善性提案已进入原生控制工具链。精确混音单步仍需确认，确认前不会修改工程。",
		NeedsConfirmation: true,
		Workflow:          "mix_tick",
		GoalStatus:        string(agentruntime.StatusWaitingConfirmation),
		StopReason:        "improvement_proposal_native_tool_confirmation_required",
		WorkflowData: mergeContext(typedPendingPayload(pendingMixTickEventPayload(candidate, candidate.ObservationID), typed), map[string]any{
			"improvement_proposal":         agentprotocol.ToMap(proposal),
			"action_domain_router":         true,
			"proposal_confirmation":        "accepted",
			"proposal_execution_authority": "mix_tick_tools",
			"mutation_performed":           false,
		}),
		TypedEvents:         []map[string]any{agentprotocol.ToMap(agentprotocol.NewEvent(typed, typed.Source))},
		InteractionRequests: []AgentInteractionRequest{request},
	}, true
}

func improvementProposalTrackID(proposal agentprotocol.ImprovementProposal) string {
	if !strings.EqualFold(strings.TrimSpace(fmt.Sprint(proposal.Target["kind"])), "track") {
		return ""
	}
	return firstStringFromMap(proposal.Target, "id", "track_id")
}

func improvementProposalFromInteraction(interaction PendingInteraction) (agentprotocol.ImprovementProposal, error) {
	row := firstMapFromAny(interaction.Payload["proposal"])
	if len(row) == 0 {
		state := firstMapFromAny(interaction.Payload["typed_state"])
		candidate := firstMapFromAny(state["candidate_action"])
		row = candidate
	}
	if len(row) == 0 {
		return agentprotocol.ImprovementProposal{}, fmt.Errorf("proposal payload missing")
	}
	raw, err := json.Marshal(row)
	if err != nil {
		return agentprotocol.ImprovementProposal{}, err
	}
	var proposal agentprotocol.ImprovementProposal
	if err := json.Unmarshal(raw, &proposal); err != nil {
		return agentprotocol.ImprovementProposal{}, err
	}
	if err := proposal.Validate(); err != nil {
		return agentprotocol.ImprovementProposal{}, err
	}
	return proposal, nil
}

func improvementProposalProcessorType(proposal agentprotocol.ImprovementProposal) string {
	if processor := strings.ToLower(strings.TrimSpace(proposal.ProcessorType)); processor != "" {
		return processor
	}
	switch strings.ToLower(strings.TrimSpace(proposal.ActionDomain)) {
	case agentprotocol.ImprovementActionDomainEQ:
		return "eq"
	case agentprotocol.ImprovementActionDomainCompressor:
		return "compressor"
	case agentprotocol.ImprovementActionDomainLimiter:
		return "limiter"
	case agentprotocol.ImprovementActionDomainGateExpander:
		return "gate_expander"
	case agentprotocol.ImprovementActionDomainDeEsser:
		return "de_esser"
	case agentprotocol.ImprovementActionDomainTransientShaper:
		return "transient_shaper"
	case agentprotocol.ImprovementActionDomainMultibandDynamics:
		return "multiband_dynamics"
	default:
		return ""
	}
}

func improvementProposalResolutionResponse(interaction PendingInteraction, proposal agentprotocol.ImprovementProposal) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply:    "提案已确认，但仍有需要澄清的边界条件；在补齐这些信息前不会进入参数执行。",
		Workflow: improvementProposalWorkflow, WorkflowData: map[string]any{
			"schema_version": agentprotocol.ImprovementProposalSchema, "status": "resolution_required",
			"mutation_performed": false, "action_domain_router": false,
			"needs_resolution": proposal.NeedsResolution, "proposal": agentprotocol.ToMap(proposal),
		}, GoalStatus: string(agentruntime.StatusWaitingClarification), StopReason: "improvement_proposal_resolution_required"}
}

func improvementProposalBoundaryResponse(interaction PendingInteraction, reply, reason string) ChatResponse {
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: reply, Workflow: improvementProposalWorkflow, WorkflowData: map[string]any{
			"schema_version": agentprotocol.ImprovementProposalSchema, "status": "capability_boundary",
			"mutation_performed": false, "action_domain_router": false,
		}, GoalStatus: string(agentruntime.StatusCompleted), StopReason: reason}
}
