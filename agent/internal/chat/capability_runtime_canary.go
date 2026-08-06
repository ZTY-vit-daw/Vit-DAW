package chat

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"vit-daw-agent/internal/actionworkflow"
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

const staticBalanceCapabilityID = "static_mix.static_balance.v0"
const panLayoutCapabilityID = "static_mix.pan_layout.v0"
const lowEndRelationCapabilityID = "static_mix.low_end_relation.v0"
const frequencyCleanupCapabilityID = "fine_mix.frequency_cleanup.v1"
const retiredFocusPositionCapabilityID = "static_mix.focus_position.v0"

// handleCapabilityRuntimeCanary is the permanent B2/B3 abstraction seam.
// The historical name is retained for API/source compatibility, but every
// newly resolved B2/B3 Session is owned by runtime v1.
func (s *Server) handleCapabilityRuntimeCanary(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) (ChatResponse, bool) {
	if s == nil || s.orchestrationRuntime == nil {
		return ChatResponse{}, false
	}
	resolution := s.resolveCapabilityOwner(conversationID, req)
	if resolution.Ambiguous {
		message := "当前有多个 v1 能力 Session 等待处理，请明确说明要确认 B2 静态平衡还是 B3 声像布局。"
		return ChatResponse{ConversationID: conversationID, Reply: message, Workflow: "capability_runtime_v1", GoalStatus: "waiting_clarification"}, true
	}
	if resolution.CapabilityID == "" || resolution.Decision.Owner != orchestration.EngineV1 {
		return ChatResponse{}, false
	}
	req.Context = cloneContext(req.Context)
	if req.Context == nil {
		req.Context = map[string]any{}
	}
	req.Context["capability_runtime_v1"] = true
	req.Context["capability_id"] = resolution.CapabilityID
	req.Context["capability_session_id"] = resolution.SessionID
	req.Context["engine_owner_source"] = string(resolution.Decision.Source)
	capabilityID := resolution.CapabilityID
	if resolution.SessionID != "" && actionworkflow.ClassifyConfirmation(req.Message, true).Kind == actionworkflow.DecisionReject {
		// A phrase about rolling back a retired effect path can be a request to
		// construct a new rollback Proposal, not an instruction to cancel a
		// Session that has not been created yet.  Only cancel an existing v1
		// Session; the capability-specific handler owns fresh rollback routing.
		if _, exists := s.orchestrationRuntime.Store.Load(resolution.SessionID); exists {
			cancelled, err := s.orchestrationRuntime.CancelPlanningSession(resolution.SessionID)
			if err != nil {
				return capabilityCanaryBlockedResponse(conversationID, goal, "v1 Session 取消失败："+err.Error()), true
			}
			response := ChatResponse{
				ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
				Reply: "已取消当前 Proposal/PlanningSession；没有执行新的工程修改。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCancelled),
				WorkflowData: map[string]any{"session_id": cancelled.ID, "capability_id": capabilityID, "engine_owner": cancelled.EngineOwner, "canary_stage": "cancelled"},
			}
			attachMixboardDecisionProjection(&response, cancelled)
			return response, true
		}
	}
	if resolution.SessionID != "" {
		if pendingSession, exists := s.orchestrationRuntime.Store.Load(resolution.SessionID); exists && pendingSession.ActiveProposal != nil && pendingSession.FrozenPlan != nil && (pendingSession.Status == orchestration.StatusWaiting || pendingSession.Status == orchestration.StatusReady) {
			if !expectedProposalMatches(req.Context, pendingSession.ActiveProposal) {
				return capabilityCanaryBlockedResponse(conversationID, goal, "确认交互绑定的 Proposal revision 已过期；没有授权或修改工程。"), true
			}
			decision := resolvePendingCapabilityTurn(req, goal, pendingSession)
			req.Context = contextWithCapabilityApprovalDecision(req.Context, decision)
			switch decision.Kind {
			case orchestration.ApprovalReject:
				cancelled, err := s.orchestrationRuntime.CancelPlanningSession(resolution.SessionID)
				if err != nil {
					return capabilityCanaryBlockedResponse(conversationID, goal, "v1 Session 取消失败："+err.Error()), true
				}
				response := ChatResponse{
					ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
					Reply: "已取消当前 Proposal；没有执行新的工程修改。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCancelled),
					WorkflowData: map[string]any{"session_id": cancelled.ID, "capability_id": capabilityID, "engine_owner": cancelled.EngineOwner, "canary_stage": "cancelled", "approval_decision": decision},
				}
				attachMixboardDecisionProjection(&response, cancelled)
				return response, true
			case orchestration.ApprovalQuestion:
				return proposalQuestionResponse(conversationID, goal, pendingSession), true
			case orchestration.ApprovalRevise, orchestration.ApprovalNarrow:
				if capabilityID == agentSemanticEQCapabilityID {
					if _, err := s.orchestrationRuntime.CancelPlanningSession(resolution.SessionID); err != nil {
						return capabilityCanaryBlockedResponse(conversationID, goal, "无法取消旧 EQ Proposal 以重新规划："+err.Error()), true
					}
					// Let the ordinary Agent interpret the revised acoustic goal and
					// produce a brand-new semantic action. No frozen action is edited.
					return ChatResponse{}, false
				}
			case orchestration.ApprovalAmbiguous, orchestration.ApprovalNoDecision:
				return proposalAmbiguousResponse(conversationID, goal, pendingSession), true
			}
		}
	}
	if capabilityID == panLayoutCapabilityID {
		return s.handlePanLayoutRuntimeCanary(ctx, conversationID, req, goal), true
	}
	if capabilityID == lowEndRelationCapabilityID {
		return s.handleLowEndRelationRuntimeCanary(ctx, conversationID, req, goal), true
	}
	if capabilityID == frequencyCleanupCapabilityID {
		return s.handleFrequencyCleanupRuntime(ctx, conversationID, req, goal), true
	}
	if capabilityID == agentSemanticEQCapabilityID {
		return s.handleSemanticEQRuntime(ctx, conversationID, req, goal), true
	}
	if capabilityID != staticBalanceCapabilityID {
		return ChatResponse{}, false
	}
	if s.kernel == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前没有可用的 Project Kernel，B2 只读 Shadow 未启动。"), true
	}

	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		message := "无法取得当前工程 snapshot，B2 只读 Shadow 未启动。"
		if err != nil {
			message += " " + err.Error()
		}
		return capabilityCanaryBlockedResponse(conversationID, goal, message), true
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, staticBalanceCapabilityID)
	}
	session, ok := s.orchestrationRuntime.Store.Load(sessionID)
	if ok && !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "确认交互绑定的 Proposal 已过期；没有授权或修改工程。"), true
	}
	if ok && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
		return s.recoverCapabilityExecution(ctx, conversationID, goal, session), true
	}
	decision, hasDecision := capabilityApprovalDecisionFromContext(req.Context)
	if ok && session.ActiveProposal != nil && hasDecision {
		switch decision.Kind {
		case orchestration.ApprovalApprove:
			return s.handleCapabilityCanaryAuthorization(ctx, conversationID, req, goal, session, state), true
		case orchestration.ApprovalRevise, orchestration.ApprovalNarrow:
			if !proposalDecisionRequiresReplan(decision) {
				baseRevision, _ := strconv.ParseInt(session.FrozenPlan.ProjectCut.BaseProjectRevision, 10, 64)
				if state.ProjectEpoch != session.FrozenPlan.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
					return capabilityCanaryBlockedResponse(conversationID, goal, "工程已变化，当前 Proposal 不能继续修订；请重新发起 B2 分析。"), true
				}
				revised, reviseErr := applyConversationalProposalRevision(session, decision)
				if reviseErr != nil {
					return proposalRevisionClarificationResponse(conversationID, goal, session, reviseErr), true
				}
				updated, saveErr := s.orchestrationRuntime.AttachFrozenPlan(session.ID, revised)
				if saveErr != nil {
					return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 B2 Proposal revision："+saveErr.Error()), true
				}
				return proposalRevisionResponse(conversationID, goal, updated), true
			}
		}
	}
	if ok && session.Status == orchestration.StatusAuthorized {
		return authorizedCapabilityWaitingResponse(conversationID, goal, session), true
	}

	input, dependencies, err := s.acquireCapabilityCanaryB2Context(ctx, sessionID, req, state)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B2 CCB 只读上下文获取失败："+err.Error()), true
	}
	cutGuarantee := capabilityCanaryCutGuarantee(ctx, s.kernel)
	decisionRefs, decisionRefsErr := mixboardDecisionContextForState(state.LegacyState, staticBalanceCapabilityID)
	buildCut := projectcut.BuildRequest{
		State:                  state,
		Guarantee:              cutGuarantee,
		DependencyFingerprints: dependencies,
		TargetFingerprints:     canaryTrackFingerprints(state.LegacyState),
		ArtifactRefs:           appendUniqueStrings(canaryStringSlice(req.Context["artifact_refs"]), mixboardDecisionArtifactRefs(decisionRefs)...),
		ContractVersions:       []string{"capability:static_mix.static_balance.v0", "context:static_mix.static_balance.context_pack.v1"},
	}
	cut, err := projectcut.Build(buildCut)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 Project Cut："+err.Error()), true
	}
	if !ok {
		mode := canaryInteractionMode(req.Context)
		session, err = s.orchestrationRuntime.StartB2ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, mode)
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 v1 Planning Session："+err.Error()), true
		}
	}

	planned, _, err := s.orchestrationRuntime.ShadowB2FromVSP(sessionID, state, buildCut, input)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B2 Shadow 失败："+err.Error()), true
	}
	attachMixboardDecisionContext(&planned.Bundle, decisionRefs, decisionRefsErr)
	envelope, err := s.orchestrationRuntime.BuildB2ContextEnvelope(
		sessionID,
		planned.Bundle,
		capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{
			ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message,
			Reference: "chat-history:" + conversationID, Priority: 100,
		}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B2 Context Envelope 未通过预算准入："+err.Error()), true
	}

	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return capabilityCanaryAnalysisResponse(conversationID, goal, sessionID, planned, envelope), true
	}
	if planned.Outcome.Kind == orchestration.OutcomeBlocked || len(planned.CandidateIDs) == 0 {
		response := capabilityCanaryBlockedResponse(conversationID, goal, "B2 当前不能形成可执行候选："+strings.Join(planned.Outcome.Blockers, ", "))
		response.WorkflowData = map[string]any{
			"session_id": sessionID, "capability_id": staticBalanceCapabilityID,
			"canary_stage": "readiness_blocked", "blockers": append([]string(nil), planned.Outcome.Blockers...),
			"readiness": planned.Pack.Readiness, "evidence_status": planned.Pack.EvidenceStatus,
			"context_bundle_id": planned.Bundle.ID,
		}
		return response, true
	}

	selected := canaryCandidateIndex(req.Message, len(planned.CandidateIDs))
	if selected < 0 {
		selected = recommendedCanaryCandidate(planned.Pack)
	}
	if selected >= len(planned.CandidateIDs) {
		selected = 0
	}

	frozen, err := capabilityadaptersFreezeStaticBalance(planned, cut, selected, session)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 B2 Proposal："+err.Error()), true
	}
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, frozen)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 B2 Proposal："+err.Error()), true
	}
	return capabilityCanaryProposalResponse(conversationID, goal, updated, planned, envelope), true
}

func capabilityCanaryCutGuarantee(ctx context.Context, client *kernel.Client) projectcut.SourceGuarantee {
	if client == nil {
		return projectcut.GuaranteeAdapterSnapshot
	}
	enabled, err := client.VSPFeature(ctx, "command.base_revision_cas")
	if err == nil && enabled {
		return projectcut.GuaranteeKernelBarrier
	}
	return projectcut.GuaranteeAdapterSnapshot
}

func (s *Server) acquireCapabilityCanaryB2Context(ctx context.Context, sessionID string, req ChatRequest, state *kernel.VSPStateResult) (capabilitycontext.StaticBalanceInput, []string, error) {
	if s == nil || s.harness == nil || state == nil {
		return capabilitycontext.StaticBalanceInput{}, nil, fmt.Errorf("harness and VSP state are required")
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
		return capabilitycontext.StaticBalanceInput{}, nil, fmt.Errorf("mix.observe: %s", firstNonEmpty(observe.Error, canaryErrorText(err), observe.Status))
	}
	observationID := firstStringFromMap(observe.Result, "observation_id")
	if observationID == "" {
		return capabilitycontext.StaticBalanceInput{}, nil, fmt.Errorf("mix.observe omitted observation_id")
	}
	tomProjection := capabilitycontext.BuildProjectTOMProjection(state.LegacyState, observationID, sessionID)
	dependencies := []string{
		"vsp.snapshot:" + state.SnapshotHash,
		"MOM:" + observationID,
	}
	if len(tomProjection) > 0 {
		dependencies = append(dependencies, "TOM:"+firstNonEmpty(firstStringFromMap(tomProjection, "tom_version"), "project-state-derived")+":"+observationID)
	}
	projectState := cloneContext(state.LegacyState)
	projectState["snapshot_hash"] = state.SnapshotHash
	projectState["project_revision"] = state.Revision
	projectState["project_epoch"] = state.ProjectEpoch
	return capabilitycontext.StaticBalanceInput{
		UserIntent: req.Message, ProjectState: projectState,
		MixObservation: cloneContext(observe.Result), MOMProjection: protocolvalue.Object(observe.Result["mom_projection"]),
		TOMProjection: tomProjection, RequestContext: cloneContext(req.Context),
		ExecutionMemory: firstMapFromAny(req.Context["execution_memory"]),
	}, dependencies, nil
}

func (s *Server) handleCapabilityCanaryAuthorization(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Proposal 缺少持久化 frozen ActionSet，不能授权；请重新生成方案。")
	}
	frozen := *session.FrozenPlan
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Kernel 未协商 command.base_revision_cas，旧 Proposal 未获授权。")
	}
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if state == nil || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 已变化，旧 Proposal 未获授权；请重新生成 Proposal。")
	}
	if strings.TrimSpace(frozen.PreviousObservationID) == "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Proposal 缺少前序 observation identity，无法执行 fresh acoustic verification。")
	}
	if !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 v1 Session Store 不是持久化 Store；为保证崩溃恢复，本次没有授权或修改工程。")
	}
	if s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Harness 不可用，无法创建 Project History baseline。")
	}
	envelope, envelopeErr := s.orchestrationRuntime.BuildB2ContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: appendUniqueStrings([]string{"capability-pack:" + frozen.ContextBundleID}, frozen.ProjectCut.ArtifactRefs...)},
		capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "authorization_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if envelopeErr != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "授权上下文未通过预算准入："+envelopeErr.Error())
	}
	authorized := session
	if session.Status != orchestration.StatusAuthorized {
		proposal := frozen.Proposal
		decision, ok := capabilityApprovalDecisionFromContext(req.Context)
		if !ok || !decision.ExactApprovalFor(proposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此 Proposal revision 的明确授权。")
		}
		var authErr error
		authorized, authErr = s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
			ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: proposal.ActionSetHash,
			ProjectCutHash: proposal.ProjectCutHash, Scope: append([]string(nil), proposal.TargetScope...),
			SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision,
		})
		if authErr != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "授权未能绑定当前 Proposal："+authErr.Error())
		}
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
		&executionports.StaticBalanceVSPPort{Client: s.kernel},
		executionverifiers.StaticBalance{
			State: s.kernel,
			Acoustic: executionverifiers.HarnessAcoustic{
				Invoker: s.harness, PreviousObservationID: frozen.PreviousObservationID,
				MixSessionID: session.ID, GoalText: session.Goal,
			},
		},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	response := capabilityCanaryExecutionResponse(conversationID, goal, executed, envelope, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	attachMixboardDecisionProjection(&response, executed)
	return response
}

func canaryTrackFingerprints(state map[string]any) []string {
	rows := mapRowsFromAny(state["tracks"])
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		trackID := firstStringFromMap(row, "track_id", "id")
		if trackID == "" {
			continue
		}
		out = append(out, fmt.Sprintf("track:%s:fader:%v", trackID, row[firstPresentCanaryKey(row, "volume_db", "fader_db", "gain_db", "db")]))
	}
	return out
}

func firstPresentCanaryKey(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if _, ok := row[key]; ok {
			return key
		}
	}
	return "volume_db"
}

func canaryErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func capabilityCanaryToolSchemas(s *Server) []orchestration.ContextEntry {
	names := []string{"project.state", "mix.observe", "mix.read", "mix.derive"}
	entries := make([]orchestration.ContextEntry, 0, len(names))
	for index, name := range names {
		content := ""
		if s != nil && s.harness != nil {
			content = s.harness.ModelCatalogSummaryForTools([]string{name})
		}
		entries = append(entries, orchestration.ContextEntry{
			ID: name, Content: content, Reference: "tool-catalog:" + name, Priority: 100 - index,
		})
	}
	return entries
}

func capabilityadaptersFreezeStaticBalance(planned capabilityadapters.StaticBalancePlanResult, cut orchestration.ProjectCut, index int, session orchestration.PlanningSession) (orchestration.FrozenPlan, error) {
	if index < 0 || index >= len(planned.CandidateIDs) {
		return orchestration.FrozenPlan{}, fmt.Errorf("candidate index %d is out of range", index)
	}
	revision := int64(1)
	if session.ActiveProposal != nil {
		revision = session.ActiveProposal.Revision + 1
	}
	proposal, err := capabilityadapters.FreezeStaticBalanceProposal(planned.Pack, cut, planned.CandidateIDs[index], revision)
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	actionSet, err := capabilityadapters.StaticBalanceActionSet(planned.Pack, cut, planned.CandidateIDs[index])
	if err != nil {
		return orchestration.FrozenPlan{}, err
	}
	proposal.Presentation = staticBalanceProposalPresentation(proposal, planned.Pack, planned.CandidateIDs[index], actionSet)
	return orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut,
		ContextBundleID: planned.Bundle.ID, PreviousObservationID: planned.Pack.Result().ObservationID,
	}, nil
}

func capabilityRuntimeCanaryEnabled(ctx map[string]any) bool {
	for _, key := range []string{"capability_runtime_v1", "orchestration_v1_canary"} {
		value := strings.ToLower(strings.TrimSpace(fmt.Sprint(ctx[key])))
		if value == "true" || value == "1" || value == "yes" {
			return true
		}
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("VIT_CAPABILITY_RUNTIME_V1_CANARY")))
	return env == "1" || env == "true" || env == "yes"
}

func canaryInteractionMode(ctx map[string]any) orchestration.InteractionMode {
	switch strings.ToLower(strings.TrimSpace(firstStringFromMap(ctx, "interaction_mode", "mode"))) {
	case string(orchestration.InteractionInspect):
		return orchestration.InteractionInspect
	case string(orchestration.InteractionExecuteIntent):
		return orchestration.InteractionExecuteIntent
	default:
		return orchestration.InteractionPropose
	}
}

func canaryStringSlice(value any) []string {
	var out []string
	switch values := value.(type) {
	case []string:
		out = append(out, values...)
	case []any:
		for _, value := range values {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	case string:
		for _, value := range strings.Split(values, ",") {
			if text := strings.TrimSpace(value); text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}

func sanitizeCanaryID(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "conversation"
	}
	return b.String()
}

func canaryCandidateIndex(message string, count int) int {
	text := strings.ToLower(strings.TrimSpace(message))
	for _, token := range []string{"第3个", "第三个", "third", "3"} {
		if strings.Contains(text, token) {
			return 2
		}
	}
	for _, token := range []string{"第2个", "第二个", "second", "2"} {
		if strings.Contains(text, token) {
			return 1
		}
	}
	for _, token := range []string{"第1个", "第一个", "first", "1"} {
		if strings.Contains(text, token) {
			return 0
		}
	}
	return -1
}

func recommendedCanaryCandidate(pack capabilitycontext.StaticBalancePack) int {
	for index, candidate := range pack.Result().Candidates {
		if candidate.Recommended {
			return index
		}
	}
	return 0
}

func capabilityCanaryProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, planned capabilityadapters.StaticBalancePlanResult, envelope orchestration.ContextEnvelope) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID:       conversationID,
		GoalID:               goal.GoalID,
		RunID:                goal.RunID,
		Reply:                renderProposalConversation(presentation),
		NeedsConfirmation:    true,
		PlanID:               session.ActiveProposal.ID,
		Preview:              firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary),
		ProposalPresentation: presentation,
		Workflow:             "capability_runtime_v1",
		WorkflowData: map[string]any{
			"session_id":            session.ID,
			"capability_id":         session.Invocation.CapabilityID,
			"proposal_id":           session.ActiveProposal.ID,
			"proposal_revision":     session.ActiveProposal.Revision,
			"action_set_hash":       session.ActiveProposal.ActionSetHash,
			"candidate_ids":         planned.CandidateIDs,
			"context_bundle_id":     planned.Bundle.ID,
			"project_cut_hash":      session.ActiveProposal.ProjectCutHash,
			"canary_stage":          "proposal",
			"approval_mode":         "conversational",
			"proposal_presentation": presentation,
			"context_budget":        capabilityCanaryContextBudget(envelope),
		},
		GoalStatus: string(agentruntime.StatusWaitingConfirmation),
	}
}

func capabilityCanaryAnalysisResponse(conversationID string, goal agentruntime.Goal, sessionID string, planned capabilityadapters.StaticBalancePlanResult, envelope orchestration.ContextEnvelope) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          planned.Outcome.Summary + "。这是只读分析，当前没有创建 Proposal 或授权。",
		Workflow:       "capability_runtime_v1",
		WorkflowData: map[string]any{
			"session_id":        sessionID,
			"capability_id":     planned.Pack.CapabilityID,
			"candidate_ids":     planned.CandidateIDs,
			"context_bundle_id": planned.Bundle.ID,
			"project_cut_hash":  planned.Bundle.ProjectCutHash,
			"canary_stage":      "analysis",
			"context_budget":    capabilityCanaryContextBudget(envelope),
		},
		GoalStatus: string(agentruntime.StatusCompleted),
	}
}

func capabilityCanaryAuthorizedResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, envelope orchestration.ContextEnvelope) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          "已记录当前 B2 Proposal 的语义授权。当前 canary 仍停在 Execution Coordinator 接入前，因此没有修改工程。",
		Workflow:       "capability_runtime_v1",
		WorkflowData: map[string]any{
			"session_id":        session.ID,
			"capability_id":     session.Invocation.CapabilityID,
			"proposal_id":       session.ActiveProposal.ID,
			"proposal_revision": session.ActiveProposal.Revision,
			"project_cut_hash":  session.ActiveProposal.ProjectCutHash,
			"canary_stage":      "authorized_not_executed",
			"context_budget":    capabilityCanaryContextBudget(envelope),
		},
		GoalStatus: string(agentruntime.StatusWaitingContinue),
	}
}

func capabilityCanaryExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, envelope orchestration.ContextEnvelope, executeErr error) ChatResponse {
	stage := "authorized_execution_blocked"
	reply := "B2 Proposal 已授权，但执行在 mutation 前被安全门禁阻止。"
	goalStatus := string(agentruntime.StatusFailed)
	workflowData := map[string]any{
		"session_id": session.ID, "capability_id": session.Invocation.CapabilityID, "canary_stage": stage,
		"context_budget":  capabilityCanaryContextBudget(envelope),
		"user_acceptance": "unknown",
	}
	if session.ActiveProposal != nil {
		workflowData["proposal_id"] = session.ActiveProposal.ID
		workflowData["proposal_revision"] = session.ActiveProposal.Revision
		workflowData["action_set_hash"] = session.ActiveProposal.ActionSetHash
		workflowData["project_cut_hash"] = session.ActiveProposal.ProjectCutHash
	}
	if session.Authorization != nil {
		workflowData["authorization_source_turn"] = session.Authorization.SourceTurnID
		workflowData["authorization_scope"] = append([]string(nil), session.Authorization.Scope...)
		if session.Authorization.Decision != nil {
			workflowData["approval_decision"] = *session.Authorization.Decision
		}
	}
	if session.Execution != nil {
		workflowData["execution_id"] = session.Execution.ID
		workflowData["execution_status"] = session.Execution.Status
		workflowData["receipt_count"] = len(session.Execution.Receipts)
		workflowData["verification"] = session.Execution.Verification
		if session.Execution.VerificationResult != nil {
			workflowData["verification_result"] = *session.Execution.VerificationResult
		}
		workflowData["persistence_refs"] = append([]string(nil), session.Execution.PersistenceRefs...)
		if session.Status == orchestration.StatusCompleted {
			stage = "executed_verified"
			reply = "B2 frozen ActionSet 已通过 v1 Execution Coordinator 执行；推子结构回读与 fresh mix.observe/MOM 验证均通过。用户审美接受仍为 unknown。"
			goalStatus = string(agentruntime.StatusCompleted)
		} else if session.Status == orchestration.StatusNeedsReview {
			stage = "executed_needs_review"
			reply = "B2 动作与持久化 Receipt 已完成，工程执行没有失败；当前 verification 为 inconclusive，结果已标记为待复核且不会自动重试。用户审美接受仍为 unknown。"
			goalStatus = string(agentruntime.StatusWaitingContinue)
		} else {
			stage = "execution_failed"
			reply = "B2 执行已产生持久化 Receipt，但验证未形成完整 pass；不会宣称混音成功，请根据执行记录复查或回滚。"
		}
		workflowData["canary_stage"] = stage
	}
	response := ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: reply, Workflow: "capability_runtime_v1", WorkflowData: workflowData,
		GoalStatus: goalStatus,
	}
	if executeErr != nil && session.Status != orchestration.StatusNeedsReview {
		response.Error = executeErr.Error()
	}
	return response
}

func capabilityCanaryContextBudget(envelope orchestration.ContextEnvelope) map[string]any {
	return map[string]any{
		"valid":                  envelope.Valid,
		"estimated_input_tokens": envelope.EstimatedInputTokens,
		"max_input_tokens":       envelope.MaxInputTokens,
		"output_reserve_tokens":  envelope.OutputReserveTokens,
		"registry_entries":       len(envelope.Registry),
		"tool_schemas":           len(envelope.ToolSchemas),
		"conversation_turns":     len(envelope.ConversationTurns),
		"omissions":              envelope.Omissions,
	}
}

func capabilityCanaryBlockedResponse(conversationID string, goal agentruntime.Goal, message string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          message,
		Workflow:       "capability_runtime_v1",
		GoalStatus:     string(agentruntime.StatusFailed),
		Error:          message,
	}
}
