package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	"vit-daw-agent/internal/protocolvalue"
	agentruntime "vit-daw-agent/internal/runtime"
)

func (s *Server) handlePanLayoutRuntimeCanary(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.kernel == nil || s.orchestrationRuntime == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前没有可用的 B3 Project-aware Runtime。")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法取得当前工程 snapshot，B3 Shadow 未启动："+canaryErrorText(err))
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, panLayoutCapabilityID)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 确认交互绑定的 Proposal 已过期；没有授权或修改工程。")
	}
	if exists && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
		return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
	}
	decision, hasDecision := capabilityApprovalDecisionFromContext(req.Context)
	if exists && session.ActiveProposal != nil && hasDecision {
		switch decision.Kind {
		case orchestration.ApprovalApprove:
			return s.handlePanLayoutCanaryAuthorization(ctx, conversationID, req, goal, session, state)
		case orchestration.ApprovalRevise, orchestration.ApprovalNarrow:
			if !proposalDecisionRequiresReplan(decision) {
				baseRevision, _ := strconv.ParseInt(session.FrozenPlan.ProjectCut.BaseProjectRevision, 10, 64)
				if state.ProjectEpoch != session.FrozenPlan.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
					return capabilityCanaryBlockedResponse(conversationID, goal, "工程已变化，当前 Proposal 不能继续修订；请重新发起 B3 分析。")
				}
				revised, reviseErr := applyConversationalProposalRevision(session, decision)
				if reviseErr != nil {
					return proposalRevisionClarificationResponse(conversationID, goal, session, reviseErr)
				}
				updated, saveErr := s.orchestrationRuntime.AttachFrozenPlan(session.ID, revised)
				if saveErr != nil {
					return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 B3 Proposal revision："+saveErr.Error())
				}
				return proposalRevisionResponse(conversationID, goal, updated)
			}
		}
	}
	if exists && session.Status == orchestration.StatusAuthorized {
		return authorizedCapabilityWaitingResponse(conversationID, goal, session)
	}
	input, dependencies, err := s.acquirePanLayoutCanaryContext(ctx, sessionID, req, state)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 CCB 只读上下文获取失败："+err.Error())
	}
	buildCut := projectcut.BuildRequest{
		State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel),
		DependencyFingerprints: dependencies, TargetFingerprints: canaryPanFingerprints(state.LegacyState),
		ArtifactRefs:     canaryStringSlice(req.Context["artifact_refs"]),
		ContractVersions: []string{"capability:static_mix.pan_layout.v0", "context:static_mix.pan_layout.context_pack.v1"},
	}
	cut, err := projectcut.Build(buildCut)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 B3 Project Cut："+err.Error())
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartB3ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context))
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 B3 v1 Planning Session："+err.Error())
		}
	}
	planned, _, err := s.orchestrationRuntime.ShadowB3FromVSP(sessionID, state, buildCut, input)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 Shadow 失败："+err.Error())
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		sessionID, planned.Bundle, capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 Context Envelope 未通过预算准入："+err.Error())
	}
	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return panLayoutCanaryAnalysisResponse(conversationID, goal, sessionID, planned, envelope)
	}
	if planned.Outcome.Kind == orchestration.OutcomeBlocked || len(planned.CandidateIDs) == 0 {
		response := capabilityCanaryBlockedResponse(conversationID, goal, "B3 当前不能形成可执行候选："+strings.Join(planned.Outcome.Blockers, ", "))
		response.WorkflowData = map[string]any{
			"session_id": sessionID, "capability_id": panLayoutCapabilityID,
			"canary_stage": "readiness_blocked", "blockers": append([]string(nil), planned.Outcome.Blockers...),
			"readiness": planned.Pack.Readiness, "evidence_status": planned.Pack.EvidenceStatus,
			"context_bundle_id": planned.Bundle.ID, "mom_summary": planned.Pack.MOMSummary,
			"layout_summary": planned.Pack.LayoutSummary,
		}
		return response
	}
	selected := canaryCandidateIndex(req.Message, len(planned.CandidateIDs))
	if selected < 0 {
		selected = recommendedPanLayoutCandidate(planned.Pack)
	}
	if selected >= len(planned.CandidateIDs) {
		selected = 0
	}
	frozen, err := freezePanLayoutCanary(planned, cut, selected, session)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 B3 Proposal："+err.Error())
	}
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, frozen)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 B3 Proposal："+err.Error())
	}
	return panLayoutCanaryProposalResponse(conversationID, goal, updated, planned, envelope)
}

func (s *Server) acquirePanLayoutCanaryContext(ctx context.Context, sessionID string, req ChatRequest, state *kernel.VSPStateResult) (capabilitycontext.PanLayoutInput, []string, error) {
	if s == nil || s.harness == nil || state == nil {
		return capabilitycontext.PanLayoutInput{}, nil, fmt.Errorf("harness and VSP state are required")
	}
	observe, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.observe",
		Args: map[string]any{
			"scope": "full_project", "project_context": true, "observation_only": true,
			"disclosure": "digest_catalog", "mom_intent": "project_multitrack_relation_observation",
			"mix_session_id": sessionID, "goal_text": req.Message,
		},
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true},
		Source:  "capability_runtime_v1_ccb", Confirmed: true, ToolCallID: "ccb:" + sessionID + ":mix.observe",
	})
	if err != nil || !strings.EqualFold(observe.Status, "ok") {
		return capabilitycontext.PanLayoutInput{}, nil, fmt.Errorf("mix.observe: %s", firstNonEmpty(observe.Error, canaryErrorText(err), observe.Status))
	}
	observationID := firstStringFromMap(observe.Result, "observation_id")
	if observationID == "" {
		return capabilitycontext.PanLayoutInput{}, nil, fmt.Errorf("mix.observe omitted observation_id")
	}
	tomProjection := capabilitycontext.BuildProjectTOMProjection(state.LegacyState, observationID, sessionID)
	dependencies := []string{"vsp.snapshot:" + state.SnapshotHash, "MOM:" + observationID}
	if len(tomProjection) > 0 {
		dependencies = append(dependencies, "TOM:"+firstNonEmpty(firstStringFromMap(tomProjection, "tom_version"), "project-state-derived")+":"+observationID)
	}
	return capabilitycontext.PanLayoutInput{
		UserIntent: req.Message, ProjectState: cloneContext(state.LegacyState),
		MixObservation: cloneContext(observe.Result), MOMProjection: protocolvalue.Object(observe.Result["mom_projection"]),
		TOMProjection: tomProjection, RequestContext: cloneContext(req.Context), ExecutionMemory: firstMapFromAny(req.Context["execution_memory"]),
	}, dependencies, nil
}

func freezePanLayoutCanary(planned capabilityadapters.PanLayoutPlanResult, cut orchestration.ProjectCut, index int, session orchestration.PlanningSession) (orchestration.FrozenPlan, error) {
	if index < 0 || index >= len(planned.CandidateIDs) {
		return orchestration.FrozenPlan{}, fmt.Errorf("candidate index %d is out of range", index)
	}
	revision := int64(1)
	if session.ActiveProposal != nil {
		revision = session.ActiveProposal.Revision + 1
	}
	candidateID := planned.CandidateIDs[index]
	proposal, err := capabilityadapters.FreezePanLayoutProposal(planned.Pack, cut, candidateID, revision)
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	actionSet, err := capabilityadapters.PanLayoutActionSet(planned.Pack, cut, candidateID)
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	proposal.Presentation = panLayoutProposalPresentation(proposal, planned.Pack, candidateID, actionSet)
	return orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut,
		ContextBundleID: planned.Bundle.ID, PreviousObservationID: planned.Pack.Result().ObservationID,
	}, nil
}

func (s *Server) handlePanLayoutCanaryAuthorization(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 B3 Proposal 缺少 frozen ActionSet。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || state == nil || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 Cut 或 Kernel CAS 已变化，旧 B3 Proposal 未获授权；请重新生成。")
	}
	if frozen.PreviousObservationID == "" || !s.orchestrationRuntime.HasDurableStore() || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 执行缺少 fresh observation identity、durable Store 或 Project History Harness；没有修改工程。")
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: []string{"capability-pack:" + frozen.ContextBundleID}},
		capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "authorization_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B3 授权上下文未通过预算准入："+err.Error())
	}
	authorized := session
	if session.Status != orchestration.StatusAuthorized {
		proposal := frozen.Proposal
		decision, ok := capabilityApprovalDecisionFromContext(req.Context)
		if !ok || !decision.ExactApprovalFor(proposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此 B3 Proposal revision 的明确授权。")
		}
		authorized, err = s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
			ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: proposal.ActionSetHash,
			ProjectCutHash: proposal.ProjectCutHash, Scope: append([]string(nil), proposal.TargetScope...),
			SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision,
		})
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "B3 授权未绑定当前 Proposal："+err.Error())
		}
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
		&executionports.PanLayoutVSPPort{Client: s.kernel},
		executionverifiers.PanLayout{State: s.kernel, Acoustic: executionverifiers.HarnessAcoustic{
			Invoker: s.harness, PreviousObservationID: frozen.PreviousObservationID, MixSessionID: session.ID, GoalText: session.Goal,
		}},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	response := panLayoutCanaryExecutionResponse(conversationID, goal, executed, envelope, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	return response
}

func canaryPanFingerprints(state map[string]any) []string {
	rows := mapRowsFromAny(state["tracks"])
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		trackID := firstStringFromMap(row, "track_id", "id")
		if trackID != "" {
			out = append(out, fmt.Sprintf("track:%s:pan:%v", trackID, row[firstPresentCanaryKey(row, "pan", "pan_value", "balance")]))
		}
	}
	return out
}

func recommendedPanLayoutCandidate(pack capabilitycontext.PanLayoutPack) int {
	for index, candidate := range pack.Result().Candidates {
		if candidate.Recommended {
			return index
		}
	}
	return 0
}

func panLayoutCanaryProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, planned capabilityadapters.PanLayoutPlanResult, envelope orchestration.ContextEnvelope) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:             renderProposalConversation(presentation),
		NeedsConfirmation: true, PlanID: session.ActiveProposal.ID, Preview: firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID, "proposal_id": session.ActiveProposal.ID, "proposal_revision": session.ActiveProposal.Revision,
			"action_set_hash": session.ActiveProposal.ActionSetHash,
			"candidate_ids":   planned.CandidateIDs, "context_bundle_id": planned.Bundle.ID,
			"project_cut_hash": session.ActiveProposal.ProjectCutHash, "canary_stage": "proposal",
			"approval_mode": "conversational", "proposal_presentation": presentation,
			"context_budget": capabilityCanaryContextBudget(envelope),
		},
	}
}

func panLayoutCanaryAnalysisResponse(conversationID string, goal agentruntime.Goal, sessionID string, planned capabilityadapters.PanLayoutPlanResult, envelope orchestration.ContextEnvelope) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    planned.Outcome.Summary + "。这是 B3 只读分析，没有创建 Proposal 或授权。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: map[string]any{
			"session_id": sessionID, "capability_id": planned.Pack.CapabilityID, "candidate_ids": planned.CandidateIDs, "context_bundle_id": planned.Bundle.ID,
			"project_cut_hash": planned.Bundle.ProjectCutHash, "canary_stage": "analysis",
			"context_budget": capabilityCanaryContextBudget(envelope),
		},
	}
}

func panLayoutCanaryExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, envelope orchestration.ContextEnvelope, executeErr error) ChatResponse {
	response := capabilityCanaryExecutionResponse(conversationID, goal, session, envelope, executeErr)
	if session.Status == orchestration.StatusCompleted {
		response.Reply = "B3 frozen ActionSet 已执行；pan 结构回读与 fresh mix.observe/MOM 验证通过。用户审美接受仍为 unknown。"
	} else if session.Status == orchestration.StatusNeedsReview {
		response.Reply = "B3 pan 动作与持久化 Receipt 已完成，工程执行没有失败；当前 stereo verification 为 inconclusive，结果已标记为待复核且不会自动重试。用户审美接受仍为 unknown。"
	} else if session.Execution != nil {
		response.Reply = "B3 已产生持久化 Receipt，但验证未形成完整 pass；不会宣称声像布局成功，请复查或回滚。"
	}
	return response
}
