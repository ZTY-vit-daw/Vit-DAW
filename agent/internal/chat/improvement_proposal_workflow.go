package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/processorintent"
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
	// An experiment whose budget is spent and whose current round has acted has
	// no admissible round for another proposal: the settle phase owes its
	// report. Parking a fresh confirmation here would leave an interaction no
	// driver can answer (the probe filters later proposal confirmations) and a
	// non-terminal continuation behind it (2026-08-29 S3c smoke: the refused
	// round-2 settle report fell back into a proposal whose waiting checkpoint
	// failed the restart-idempotency check). Answer waiting_continue so the
	// settle chain keeps its own budget.
	if loop, loopOK := s.freeStateLoop(conversationID); loopOK && loop.Experiment != nil &&
		strings.EqualFold(strings.TrimSpace(string(loop.Experiment.Status)), string(experiment.StatusRunning)) &&
		freeStateExperimentBudgetExhausted(loop.Experiment) && !freeStateLoopRoundOwesIntervention(loop) {
		resp.Reply = "实验预算已用完；本轮改动正在等待基于改动后证据的结算报告，不会再提出新的改动。"
		resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
		resp.StopReason = "experiment_budget_awaits_settlement"
		resp.NeedsConfirmation = false
		resp.Workflow = improvementProposalWorkflow
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
			"experiment_budget_exhausted": true, "mutation_performed": false,
			"free_state_reasoning_loop":   freeStateLoopMap(loop),
		})
		resp.InteractionRequests = nil
		return resp
	}
	// The refused-settle race window: the round's settle report was refused
	// because its fresh post-action observation had not landed. The experiment
	// projection in that window can still show the round pre-action, which
	// makes the replayed proposal look like the owed round's first mutation —
	// parking it behind a confirmation (or routing it straight to a mix tick)
	// leaves an interaction the frozen driver cannot answer and a non-terminal
	// continuation behind the still-owed settle retry (2026-08-29 17:00:50
	// smoke: refused settle and the replayed tick bind in the same second).
	// Answer waiting_continue: the settle chain retries on its own budget once
	// the deterministic booking lands.
	// S3h2 split: on a round that never acted (the 205921 misdirected settle
	// on a freshly opened recalibration round) no observation can ever land,
	// so the copy points at the round's actual debt — one bounded intervention
	// proposal against the named fresh base — and a bare proposal (no settle
	// report fields) is that owed work itself: it must fall through to the
	// recalibration routing below instead of being swallowed here.
	if loop, loopOK := s.freeStateLoop(conversationID); loopOK && freeStateLoopRoundSettleRefused(loop) {
		neverActedRound := freeStateLoopRoundNeverActed(loop)
		settleReplayCarried := res.FreeStateDecision != nil &&
			(res.FreeStateDecision.ExperimentMateriality != nil || res.FreeStateDecision.ExperimentTargetResponse != nil ||
				strings.TrimSpace(res.FreeStateDecision.ExperimentRoundDecision) != "")
		if !neverActedRound || settleReplayCarried {
			if neverActedRound {
				baseRef := freeStateOwedRoundBaseReference(loop)
				resp.Reply = "本轮欠一次有界干预提案；新鲜基准为 " + baseRef + "，请引用它提出本轮提案。"
				if baseRef == "" {
					resp.Reply = "本轮欠一次有界干预提案；请引用当前新鲜基准提出本轮提案。"
				}
				resp.StopReason = "round_owes_intervention_proposal"
			} else {
				resp.Reply = "本轮改动后的观察证据尚未入账，结算报告暂缓重试；观察入账后自动继续结算，本轮不会再提出新的改动。"
				resp.StopReason = "settle_report_refused_awaiting_observation"
			}
			resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
			resp.NeedsConfirmation = false
			resp.Workflow = improvementProposalWorkflow
			resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
				"settle_report_refused": true, "mutation_performed": false,
				"round_owes_intervention": neverActedRound,
				"free_state_reasoning_loop": freeStateLoopMap(loop),
			})
			if baseRef := freeStateOwedRoundBaseReference(loop); baseRef != "" {
				resp.WorkflowData["round_base_reference"] = baseRef
			}
			resp.InteractionRequests = nil
			return resp
		}
	}
	// A recalibrating D2-2 round already carries its admitted experiment
	// direction, and the frozen probe driver cannot approve a second
	// improvement_proposal_confirmation (its experiment_proposal_approved
	// filter is per-run). Parking the owed round behind that hop would stall
	// it forever, so route the accepted proposal straight to the bounded
	// mix-tick confirmation — the per-round user boundary and every dose bound
	// stay intact (2026-08-29 S3c smoke: round 2's proposal confirmation was
	// unreachable for the driver).
	if loop, loopOK := s.freeStateLoop(conversationID); loopOK && freeStateLoopRoundOwesIntervention(loop) {
		if admittedDomain := firstStringFromMap(loop.Experiment.Admission.TypedAction, "action_domain"); admittedDomain != "" &&
			!strings.EqualFold(strings.TrimSpace(proposal.ActionDomain), admittedDomain) {
			// The recalibration reuses the existing admission, so a proposal
			// that drifts to another action domain has no admissible execution
			// path. Refuse it and keep the round live: the refusal lands in the
			// conversation the next continuation reads, and the owed round
			// re-proposes within the admitted domain (2026-08-29 S3c smoke:
			// round 2 proposed track_gain under a static_eq admission and its
			// confirmed execution died at the plan builder).
			loop.LastError = "recalibration proposal must stay within the admitted action domain " + admittedDomain
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			resp.Reply = "再校准轮沿用已准入的实验方向（" + admittedDomain + "）；这条提案切换了动作域，未执行。请在已准入域内提出本轮的单一小步调整。"
			resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
			resp.StopReason = "recalibration_domain_mismatch"
			resp.NeedsConfirmation = false
			resp.InteractionRequests = nil
			resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
				"recalibration_domain_mismatch": true, "admitted_action_domain": admittedDomain,
				"proposed_action_domain": strings.TrimSpace(proposal.ActionDomain), "mutation_performed": false,
			})
			return resp
		}
		interactionRequest := improvementProposalInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, candidate, requestContext)
		interaction := PendingInteraction{ID: interactionRequest.ID, Kind: interactionRequest.Kind, Type: interactionRequest.Type, Source: interactionRequest.Source, Workflow: interactionRequest.Workflow, ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID, RequestContext: cloneContext(requestContext), Payload: cloneContext(interactionRequest.Payload), Data: cloneContext(interactionRequest.Data)}
		s.transitionActivePendingCandidate(conversationID, "improvement_proposal", agentprotocol.PendingStatusAccepted, "recalibration round proposal routed to its bounded tick confirmation")
		if response, routed := s.routeAcceptedImprovementProposal(context.Background(), interaction); routed {
			response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{"recalibration_proposal_routed": true})
			return response
		}
	}
	// FIX-F3-G4-SEMANTICS 方案乙: a terminal-adjudication park is a gate-refused
	// proposal surfaced for the user's explicit ruling. The full-access
	// auto-authorization policy (B6 ③) covers admitted proposals only, so the
	// confirmation card is always presented for a parked one.
	parkedForAdjudication := false
	var adjudicationDisclosure map[string]any
	if adjudicationLoop, loopOK := s.freeStateLoop(conversationID); loopOK && len(adjudicationLoop.TerminalAdjudication) > 0 {
		parkedForAdjudication = true
		adjudicationDisclosure = cloneContext(adjudicationLoop.TerminalAdjudication)
	}
	if authorityModeFromContext(requestContext) == experiment.AuthorityFull && !parkedForAdjudication {
		interactionRequest := improvementProposalInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, candidate, requestContext)
		interaction := PendingInteraction{ID: interactionRequest.ID, Kind: interactionRequest.Kind, Type: interactionRequest.Type, Source: interactionRequest.Source, Workflow: interactionRequest.Workflow, ConversationID: conversationID, GoalID: res.GoalID, RunID: res.RunID, RequestContext: cloneContext(requestContext), Payload: cloneContext(interactionRequest.Payload), Data: cloneContext(interactionRequest.Data)}
		s.transitionActivePendingCandidate(conversationID, "improvement_proposal", agentprotocol.PendingStatusAccepted, "full_project_access")
		if response, routed := s.routeAcceptedImprovementProposal(context.Background(), interaction); routed {
			response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{"authority_mode": authorityModeFull, "full_access_auto_authorized": true})
			return response
		}
	}
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
	if parkedForAdjudication {
		// 方案乙: the dimensions-not-closed disclosure rides the confirmation
		// face (workflow_data.open_dimensions + terminal_adjudication) so the
		// user adjudicates the evidence-completeness gap in view.
		resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
			"open_dimensions":       adjudicationDisclosure["open_dimensions"],
			"terminal_adjudication": cloneContext(adjudicationDisclosure),
		})
		failedIDs := adjudicationGateIDText(adjudicationDisclosure)
		suffix := "证据完备类门未通过，维度未闭合；确认后按受控执行域推进，取消则不修改工程。"
		if len(failedIDs) > 0 {
			suffix = "证据完备类门（" + failedIDs + "）未通过，维度未闭合；确认后按受控执行域推进，取消则不修改工程。"
		}
		resp.Reply = strings.TrimSpace(resp.Reply) + "（维度未闭合披露：" + suffix + "）"
	}
	resp.TypedEvents = append(resp.TypedEvents, agentprotocol.ToMap(agentprotocol.NewEvent(candidate, candidate.Source)))
	req := improvementProposalInteractionRequest(conversationID, res.GoalID, res.RunID, resp.Reply, candidate, requestContext)
	resp.InteractionRequests = []AgentInteractionRequest{req}
	s.storePendingInteraction(req, req.Payload)
	if err := s.updateTaskExperimentPendingInteraction(conversationID, res.GoalID, req.ID, req.Kind, "improvement proposal confirmation required"); err != nil {
		_, _ = s.takePendingInteraction(req.ID)
		resp.InteractionRequests = nil
		resp.NeedsConfirmation = false
		resp.GoalStatus = string(agentruntime.StatusFailed)
		resp.StopReason = "task_pending_interaction_projection_failed"
		resp.Error = err.Error()
	}
	return resp
}

// adjudicationGateIDText renders the parking disclosure's failed gate ids as
// display text, whatever slice shape the durable JSON round-trip left.
func adjudicationGateIDText(disclosure map[string]any) string {
	values, _ := disclosure["failed_gate_ids"].([]any)
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			parts = append(parts, strings.TrimSpace(text))
		}
	}
	return strings.Join(parts, "、")
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
	s.completePendingInteractionContinuation(interaction)
	_ = s.updateTaskExperimentPendingInteraction(interaction.ConversationID, interaction.GoalID, "", "", "improvement proposal interaction resolved")
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
			if len(response.InteractionRequests) > 0 {
				next := response.InteractionRequests[0]
				_ = s.updateTaskExperimentPendingInteraction(interaction.ConversationID, interaction.GoalID, next.ID, firstNonEmpty(next.Kind, next.Type), "exact experiment action confirmation required")
			}
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
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	requestContext = s.bindFreeStateAuthoritativeTrack(interaction.ConversationID, requestContext)
	// Restore the host-owned capability route before authorization is checked.
	// The interaction payload may contain a pre-promotion observation snapshot;
	// the validated Task route is the authority for this already-admitted
	// continuation. This is an in-place recovery, never a new semantic entry.
	if route := s.previousCapabilityRoute("", interaction.ConversationID); route.SchemaVersion == capabilityRouteSchema &&
		route.GoalID == interaction.GoalID && route.RunID == interaction.RunID {
		if semantic, ok := capabilityRouteSemanticEntry(route); ok {
			requestContext = contextWithSemanticEntryDecision(requestContext, semantic)
			requestContext = contextWithCapabilityRoute(requestContext, route)
			if route.SemanticEntry == nil || firstStringFromMap(route.SemanticEntry, "route") != semantic.Route ||
				firstStringFromMap(route.SemanticEntry, "control_mode") != semantic.ControlMode ||
				firstStringFromMap(route.SemanticEntry, "user_authorization") != semantic.UserAuthorization {
				// Persist a narrow compatibility migration for snapshots created
				// before the post-capacity semantic entry was stored.
				route.SemanticEntry = semanticEntryDecisionMap(semantic)
				route.UpdatedAt = time.Now().UTC()
				s.storeCapabilityRoute(route)
				requestContext = contextWithCapabilityRoute(requestContext, route)
			}
		}
	}
	// Proposal approval is the continuation boundary for the already-admitted
	// free-state task. Rebuild the internal route authorization here instead of
	// treating the approval as a new semantic entry. The original semantic
	// entry decision remains in the persisted context and is still validated by
	// freeStateRouteAuthorized.
	requestContext["free_state_route_authorized"] = true
	requestContext["free_state_internal_resume"] = true
	loop, ok := s.freeStateLoop(interaction.ConversationID)
	if !ok {
		if recovered, recoveredOK := freeStateLoopFromAny(requestContext["free_state_reasoning_loop"]); recoveredOK {
			loop, ok = recovered, true
		}
	}
	if !ok || !freeStateLoopActive(loop) || loop.LatestObservation == nil {
		return improvementProposalBoundaryResponse(interaction, "已确认，但缺少可恢复的原始观察上下文；没有修改工程。", "improvement_proposal_observation_context_missing"), true
	}
	if response, routed := s.routeAcceptedImprovementProposalNativeDomain(ctx, interaction, proposal, requestContext); routed {
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
	// D2-SEMINT1: an admitted experiment whose domain row carries a semantic
	// intent anchor rides the free_state_semantic_processor_intent channel
	// with that server-derived axis, so the dynamic execution face checks the
	// PCA receipt against the domain's certified semantic axis. The generic
	// dynamic path (no free-state intent) keeps its own LLM-frozen axis
	// behavior unchanged.
	if semanticIntent, intentErr := freeStateAdmittedSemanticProcessorIntent(loop); intentErr != nil {
		return improvementProposalBoundaryResponse(interaction,
			"已确认，但已准入实验的语义 intent 轴无法从域表行派生，未进入执行；没有修改工程。",
			"improvement_proposal_semantic_intent_invalid"), true
	} else if semanticIntent != nil {
		requestContext["free_state_semantic_processor_intent"] = semanticIntent
	}
	res := agentloop.Result{
		GoalID:            firstNonEmpty(interaction.GoalID, loop.GoalID),
		RunID:             firstNonEmpty(interaction.RunID, loop.RunID),
		GoalSummary:       firstNonEmpty(proposal.ImprovementIntent, loop.OriginalIntent),
		RecentObservation: freeStateObservationWithAuthoritativeAudit(loop, loop.LatestObservation),
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
//
// AGENT-1 A1 (2026-09-05): the plugin-bound domains static_eq and
// broadband_compression consult the per-domain switch (VIT_AGENT_DOMAIN_ROUTES,
// domain_routing.go) before the legacy interception. Default legacy_native
// keeps this function byte-equivalent with the pre-switch behavior; an
// explicitly enabled processor_selection route declines here, freezes the
// routing marker, and lets the accepted proposal fall through to the governed
// semantic treatment router (existing instance planner / plugin
// recommendation / load confirmation / post-load qualification). track_gain
// and pan are true native control domains and never leave this router.
func (s *Server) routeAcceptedImprovementProposalNativeDomain(ctx context.Context, interaction PendingInteraction, proposal agentprotocol.ImprovementProposal, requestContext map[string]any) (ChatResponse, bool) {
	domain := strings.ToLower(strings.TrimSpace(proposal.ActionDomain))
	if domain != agentprotocol.ImprovementActionDomainTrackGain && domain != agentprotocol.ImprovementActionDomainPan &&
		domain != agentprotocol.ImprovementActionDomainStaticEQ && domain != d1BroadbandCompressionDomain {
		return ChatResponse{}, false
	}
	if domain == agentprotocol.ImprovementActionDomainStaticEQ || domain == d1BroadbandCompressionDomain {
		if s.domainRouteFor(domain) == DomainRouteProcessorSelection {
			markProcessorSelectionRoute(requestContext, interaction, proposal, domain)
			return ChatResponse{}, false
		}
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
	case agentprotocol.ImprovementActionDomainStaticEQ:
		// D2-1: the admitted typed action (frequency/band/Q) stays
		// authoritative on the experiment admission; this route only carries
		// the bounded band gain so the mix tick mirrors the track_gain gate.
		gainDB, ok := treatmentNumber(proposal.ParameterBounds, "gain_db")
		if !ok || gainDB == 0 || math.Abs(gainDB) > 2 {
			return improvementProposalBoundaryResponse(interaction, "已确认改善方向，但静态 EQ 工具需要一个非零且不超过 +/-2 dB 的明确 gain_db；没有修改工程。", "improvement_proposal_static_eq_bounds_missing"), true
		}
		candidate.Operation = d1StaticEQKind
	case d1BroadbandCompressionDomain:
		// D2-1.5: same mirror for the bounded broadband threshold move; the
		// admitted typed action stays authoritative on the experiment admission.
		thresholdDB, ok := treatmentNumber(proposal.ParameterBounds, "threshold_db")
		if !ok || thresholdDB == 0 || math.Abs(thresholdDB) > 2 {
			return improvementProposalBoundaryResponse(interaction, "已确认改善方向，但宽带压缩工具需要一个非零且不超过 +/-2 dB 的明确 threshold_db；没有修改工程。", "improvement_proposal_broadband_compression_bounds_missing"), true
		}
		candidate.Operation = d1BroadbandCompressionKind
	}

	// B6 缺陷③：完全访问（自动应用）分支下，pending 表面的 display 不得承诺
	// 「等待你确认」——存储与随后的直接执行在同一个调用链里，确认承诺与行为
	// 不一致（2026-09-11 用户改判定性：无确认卡直接应用=正确行为，文案是缺陷）。
	autoApply := authorityModeFromContext(requestContext) == experiment.AuthorityFull
	s.storePendingMixTickCandidateForMode(interaction.ConversationID, interaction.GoalID, interaction.RunID, candidate, autoApply)
	if autoApply {
		response := s.executePendingMixTickCandidate(ctx, interaction.ConversationID, ChatRequest{ConversationID: interaction.ConversationID, Message: "full project access", Context: requestContext, AuthorityMode: authorityModeFull}, agentModeFromContext(requestContext), candidate)
		response.NeedsConfirmation = false
		response.WorkflowData = mergeContext(response.WorkflowData, map[string]any{"authority_mode": authorityModeFull, "full_access_auto_authorized": true, "proposal_confirmation": "policy_authorized"})
		return response, true
	}
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

// freeStateAdmittedSemanticProcessorIntent derives the structured processor
// intent that rides the free-state channel when an admitted bounded
// experiment drives semantic dynamic execution (D2-SEMINT1). The admitted
// domain row is the axis authority: family and required coverage come only
// from the server-validated experiment admission, never from the proposal
// text, so the execution face checks the receipt against the domain's
// certified semantic axis instead of an accidentally LLM-frozen
// parameter-centric one. Admissions without a semantic anchor (native
// mix-tick domains, or no experiment) return nil and their continuation is
// unchanged.
func freeStateAdmittedSemanticProcessorIntent(loop freeStateReasoningLoop) (map[string]any, error) {
	if loop.Experiment == nil {
		return nil, nil
	}
	spec, ok := experiment.D1S1SemanticIntentSpecFor(loop.Experiment.Admission)
	if !ok {
		return nil, nil
	}
	refs := make([]string, 0, len(loop.Experiment.Admission.EvidenceRefs))
	seenRef := map[string]bool{}
	for _, ref := range loop.Experiment.Admission.EvidenceRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seenRef[ref] {
			continue
		}
		seenRef[ref] = true
		refs = append(refs, ref)
	}
	intent := map[string]any{
		"schema_version": processorintent.SchemaVersion,
		"status":         processorintent.StatusResolved,
		"family":         spec.SemanticIntentFamily,
		"intent": fmt.Sprintf("admitted bounded experiment %s on axis %s", spec.ActionKind,
			strings.Join(spec.SemanticIntentCoverage, "+")),
		"required_coverage": append([]string(nil), spec.SemanticIntentCoverage...),
		"scope":            processorintent.ScopeCurrentTrack,
		"control_mode":     processorintent.ControlModeSemantic,
		"confidence":       1.0,
		"evidence_refs":    refs,
	}
	// Fail closed: a domain row that cannot yield a valid intent must stop
	// this continuation instead of silently falling back to the accidental
	// LLM-frozen parameter axis.
	encoded, err := json.Marshal(intent)
	if err != nil {
		return nil, fmt.Errorf("admitted domain semantic intent is invalid: %w", err)
	}
	if _, err := processorintent.Decode(string(encoded)); err != nil {
		return nil, fmt.Errorf("admitted domain semantic intent is invalid: %w", err)
	}
	return intent, nil
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
	// AGENT-1 A1: the two D1 plugin-bound domains carry the same processor
	// families the governed semantic router already knows. These mappings only
	// matter after the native-domain interceptor declined (processor_selection
	// route); the legacy path never reaches this function for these domains.
	case agentprotocol.ImprovementActionDomainStaticEQ:
		return "eq"
	case agentprotocol.ImprovementActionDomainBroadbandCompression:
		return "compressor"
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
