package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
)

const c2DynamicPluginLoadCommand = "c2.dynamic_plugin_load.governed"

// continueC2LoadedPluginSelectionInteraction only transports an opaque
// selection into the C2 runtime. The runtime restores the target hypothesis
// from its durable checkpoint and rechecks the live instance PCA admission;
// neither the interaction payload nor request context is evidence authority.
func (s *Server) continueC2LoadedPluginSelectionInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	payload := interaction.Payload
	if len(payload) == 0 {
		payload = interaction.Data
	}
	goal := agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID}
	sessionID := firstStringFromMap(payload, "session_id")
	if strings.EqualFold(decision, "cancel") || strings.Contains(strings.ToLower(decision), "cancel") {
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
		response := ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "已取消 C2 已加载动态实例选择；没有修改工程。", Workflow: "c2_loaded_plugin_selection", GoalStatus: string(agentruntime.StatusCancelled), WorkflowData: map[string]any{"status": "cancelled", "mutation_performed": false}}
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	pluginID := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	if pluginID == "" {
		return c2BoundaryResponse(interaction.ConversationID, goal, sessionID, "invalid_loaded_dynamic_plugin_selection", fmt.Errorf("selected C2 plugin id is missing"))
	}
	found := false
	for _, row := range mapRowsValue(payload["pca_candidates"]) {
		if firstStringFromMap(row, "plugin_id") == pluginID {
			found = true
			break
		}
	}
	if !found {
		return c2BoundaryResponse(interaction.ConversationID, goal, sessionID, "invalid_loaded_dynamic_plugin_selection", fmt.Errorf("selected plugin is not in the PCA-admitted interaction candidates"))
	}
	if s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil {
		return c2BoundaryResponse(interaction.ConversationID, goal, sessionID, "c2_runtime_unavailable", fmt.Errorf("C2 orchestration store is unavailable"))
	}
	checkpoint, ok := s.orchestrationRuntime.Store.Load(sessionID)
	if !ok || checkpoint.Invocation.CapabilityID != dynamicControlCapabilityID || checkpoint.Invocation.ConversationID != interaction.ConversationID || checkpoint.FrozenPlan == nil || len(checkpoint.FrozenPlan.ActionSet.Actions) != 1 || checkpoint.FrozenPlan.ActionSet.Actions[0].Command != c2LoadedInstanceSelectionCheckpointCommand {
		return c2BoundaryResponse(interaction.ConversationID, goal, sessionID, "loaded_instance_checkpoint_unavailable", fmt.Errorf("C2 loaded-instance selection checkpoint is missing or stale"))
	}
	return s.handleDynamicControlRuntime(ctx, interaction.ConversationID, ChatRequest{ConversationID: interaction.ConversationID, Message: checkpoint.Goal, Context: mergeContext(interaction.RequestContext, map[string]any{
		"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "capability_session_id": sessionID,
		"c2_resume_session_id": sessionID, "c2_target_plugin_id": pluginID,
	})}, goal)
}

func (s *Server) continueC2DynamicPluginSelectionInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	payload := interaction.Payload
	if len(payload) == 0 {
		payload = interaction.Data
	}
	goal := agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID}
	if strings.EqualFold(decision, "cancel") || strings.Contains(strings.ToLower(decision), "cancel") {
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(firstStringFromMap(payload, "session_id"))
		response := ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "已取消 C2 动态插件选择；没有加载插件或修改参数。", Workflow: "c2_dynamic_plugin_selection", GoalStatus: string(agentruntime.StatusCancelled), WorkflowData: map[string]any{"status": "cancelled", "mutation_performed": false}}
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	key := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	var candidate pluginRecommendationCandidate
	for _, row := range mapRowsValue(payload["recommendations"]) {
		if firstStringFromMap(row, "candidate_key") == key {
			_ = decodeAnyJSON(row, &candidate)
			candidate.Key = key
			break
		}
	}
	if candidate.Key == "" || candidate.PluginPath == "" {
		return c2BoundaryResponse(interaction.ConversationID, goal, firstStringFromMap(payload, "session_id"), "invalid_dynamic_plugin_candidate", fmt.Errorf("selected C2 plugin candidate is missing or stale"))
	}
	family := firstStringFromMap(payload, "processor_family")
	// Recheck the selected binary immediately before the load Proposal. A
	// persisted UI choice is not PCA authority.
	current, err := currentPCAAdmittedPluginRecommendationCandidate(mapFromJSONStruct(candidate), family)
	if err != nil {
		return c2BoundaryResponse(interaction.ConversationID, goal, firstStringFromMap(payload, "session_id"), "pca_candidate_stale", err)
	}
	current.Key = candidate.Key
	var hypothesis dynamiccontrol.TargetHypothesis
	if err := decodeAnyJSON(payload["hypothesis"], &hypothesis); err != nil {
		return c2BoundaryResponse(interaction.ConversationID, goal, firstStringFromMap(payload, "session_id"), "frozen_hypothesis_missing", err)
	}
	return s.createC2DynamicPluginLoadProposal(ctx, interaction.ConversationID, goal, firstStringFromMap(payload, "session_id"), hypothesis, firstMapFromAny(payload["discovery"]), current)
}

func (s *Server) createC2DynamicPluginLoadProposal(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, hypothesis dynamiccontrol.TargetHypothesis, discovery map[string]any, candidate pluginRecommendationCandidate) ChatResponse {
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return c2BoundaryResponse(conversationID, goal, sessionID, "plugin_load_project_snapshot_unavailable", err)
	}
	registry, err := processorregistry.Default()
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_registry_unavailable", err)
	}
	coverage, err := registry.PCARequiredCoverage(hypothesis.Intent.Family, hypothesis.Intent.RequiredCoverage)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_coverage_unproven", err)
	}
	instances, _ := s.semanticTreatmentInstances(ctx, hypothesis.TrackID)
	before := []string{}
	for _, instance := range instances {
		before = append(before, instance.PluginID)
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: []string{"c2-hypothesis:" + hypothesis.Rationale, "pca-candidate:" + candidate.BinaryFingerprint}, TargetFingerprints: []string{"track:" + hypothesis.TrackID}, ContractVersions: []string{"capability:" + dynamicControlCapabilityID, "plugin-load:rack_add_node", "pca:processor_control_attestation"}})
	if err != nil || !cut.IsExecutable() {
		return c2BoundaryResponse(conversationID, goal, sessionID, "plugin_load_project_cut_unavailable", firstNonNilError(err, fmt.Errorf("C2 plugin load requires an executable project cut")))
	}
	args := map[string]any{"track_id": hypothesis.TrackID, "candidate": mapFromJSONStruct(candidate), "processor_family": hypothesis.Intent.Family, "required_coverage": coverage, "preexisting_plugin_ids": before, "hypothesis": mapFromJSONStruct(hypothesis), "discovery": discovery}
	action := orchestration.Action{ID: "c2_dynamic_plugin_load", Command: c2DynamicPluginLoadCommand, TargetRef: "track:" + hypothesis.TrackID, BeforeFingerprint: "c2-load:" + candidate.BinaryFingerprint, Args: args, Compensatable: true, IdempotencyClass: "c2_dynamic_plugin_load_or_rollback"}
	actionSet := orchestration.ActionSet{ID: "c2_dynamic_plugin_load_" + sanitizeCanaryID(sessionID), CapabilityID: dynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "c2_load_" + sanitizeCanaryID(sessionID), Revision: 1, CapabilityID: dynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: candidate.Key, ActionSetHash: actionSet.Hash, TargetScope: []string{"track:" + hypothesis.TrackID}, Risk: "bounded_reversible", VerificationRef: "fine_mix.dynamic_control.plugin_load.verification.v1", Summary: "Load PCA-admitted dynamic plugin " + candidate.Name + " for C2 target", CreatedAt: time.Now().UTC()}
	proposal.Presentation = &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: dynamicControlCapabilityID, Title: "C2 动态插件加载方案", Conclusion: fmt.Sprintf("在目标轨道加载 PCA 认证的动态插件 %s。", candidate.Name), Recommendation: proposal.Summary, ActionCount: 1, Risk: proposal.Risk, Reversible: true, Actions: []orchestration.ProposalActionPreview{{ActionID: action.ID, TrackID: hypothesis.TrackID, Operation: "load_dynamic_plugin", Reason: hypothesis.Rationale}}, Limitations: []string{"本次确认只授权插件加载与真实实例 PCA 验证，不会写入动态参数。", "加载或 PCA 验证失败将删除刚新增的插件实例。"}, ApprovalPrompt: "是否确认加载此 C2 动态插件？"}
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_c2_load_" + sanitizeCanaryID(sessionID), PreviousObservationID: c2DiscoveryObservationID(discovery, hypothesis.TrackID), FrozenAt: time.Now().UTC()})
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "plugin_load_plan_persist_failed", err)
	}
	return c2PluginLoadProposalResponse(conversationID, goal, session)
}

func c2PluginLoadProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil || session.FrozenPlan == nil || session.ActiveProposal.Presentation == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C2 Session 缺少动态插件加载 Proposal。")
	}
	proposal := session.ActiveProposal
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: proposal.Presentation.Conclusion + "\n\n" + proposal.Presentation.ApprovalPrompt, NeedsConfirmation: true, PlanID: proposal.ID, Preview: proposal.Presentation.Conclusion, ProposalPresentation: proposal.Presentation, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: map[string]any{"session_id": session.ID, "capability_id": dynamicControlCapabilityID, "proposal_id": proposal.ID, "proposal_revision": proposal.Revision, "action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash, "stage": "plugin_load_confirmation", "writes_performed": 0, "proposal_presentation": proposal.Presentation}}
}

func (s *Server) authorizeC2DynamicPluginLoad(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, current any) ChatResponse {
	state, ok := current.(*kernel.VSPStateResult)
	if !ok || session.FrozenPlan == nil || session.ActiveProposal == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C2 插件加载 Session 缺少冻结计划。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision || !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C2 插件加载 Proposal 的工程状态或持久化条件已失效；没有加载。")
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此 C2 插件加载 Proposal 的明确授权。")
	}
	if _, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision, ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash, Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision}); err != nil {
		return c2BoundaryResponse(conversationID, goal, session.ID, "plugin_load_authorization_failed", err)
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, session.ID, frozen.ActionSet, frozen.ProjectCut, &c2DynamicPluginLoadPort{server: s}, c2DynamicPluginLoadVerifier{}, executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID})
	response := c2DynamicPluginLoadExecutionResponse(conversationID, goal, executed, executeErr)
	attachMixboardDecisionProjection(&response, executed)
	if executeErr == nil && executed.Execution != nil && len(executed.Execution.Receipts) == 1 && executed.Execution.Receipts[0].Status == "applied" {
		pluginID := firstStringFromMap(executed.Execution.Receipts[0].Details, "plugin_id")
		var hypothesis dynamiccontrol.TargetHypothesis
		_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["hypothesis"], &hypothesis)
		if pluginID != "" && hypothesis.Status == dynamiccontrol.HypothesisTargeted {
			nextID := s.nextCapabilitySessionID(conversationID, dynamicControlCapabilityID)
			if _, startErr := s.orchestrationRuntime.StartC2ChatSession(nextID, conversationID, frozen.ProjectCut.ProjectUUID, session.Goal, orchestration.InteractionPropose); startErr == nil {
				nextContext := mergeContext(req.Context, map[string]any{
					"capability_session_id": nextID, "c2_target_track_id": hypothesis.TrackID, "c2_target_plugin_id": pluginID,
					"c2_processor_family": hypothesis.Intent.Family, "c2_resume_session_id": session.ID,
				})
				next := s.handleDynamicControlRuntime(ctx, conversationID, ChatRequest{ConversationID: conversationID, Message: session.Goal, Context: nextContext}, goal)
				next.Reply = "C2 动态插件已加载并通过真实实例 PCA 验证。\n\n" + next.Reply
				return next
			}
		}
	}
	return response
}

type c2DynamicPluginLoadPort struct{ server *Server }

func (p *c2DynamicPluginLoadPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.server == nil || p.server.kernel == nil || actionSet.CapabilityID != dynamicControlCapabilityID || len(actionSet.Actions) != 1 || actionSet.Actions[0].Command != c2DynamicPluginLoadCommand || !cut.IsExecutable() || actionSet.Hash != actionSet.ComputeHash() {
		return fmt.Errorf("invalid frozen C2 dynamic plugin-load action set")
	}
	state, err := p.server.kernel.VSPStateSnapshot(ctx, "project.timeline")
	base, parseErr := strconv.ParseInt(cut.BaseProjectRevision, 10, 64)
	if err != nil || parseErr != nil || state == nil || !state.OK() || state.ProjectEpoch != cut.ProjectEpoch || state.Revision != base {
		return fmt.Errorf("stale_project_cut")
	}
	candidate := firstMapFromAny(actionSet.Actions[0].Args["candidate"])
	if firstStringFromMap(actionSet.Actions[0].Args, "track_id") == "" || firstStringFromMap(candidate, "plugin_path") == "" {
		return fmt.Errorf("C2 dynamic plugin load lacks exact track or candidate path")
	}
	_, err = currentPCAAdmittedPluginRecommendationCandidate(candidate, firstStringFromMap(actionSet.Actions[0].Args, "processor_family"))
	return err
}

func (p *c2DynamicPluginLoadPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	if action.Command != c2DynamicPluginLoadCommand {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("unsupported C2 action %s", action.Command)
	}
	candidate := firstMapFromAny(action.Args["candidate"])
	trackID, family := firstStringFromMap(action.Args, "track_id"), firstStringFromMap(action.Args, "processor_family")
	response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "rack.add_node", Args: map[string]any{"track_id": trackID, "plugin_path": firstStringFromMap(candidate, "plugin_path"), "plugin_name": firstStringFromMap(candidate, "name"), "plugin_identifier": firstStringFromMap(candidate, "identifier"), "plugin_kind": "Effect", "zone_id": "Z3", "x": 360.0, "y": 260.0}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID}, Source: "c2_dynamic_plugin_load", Confirmed: true, ToolCallID: idempotencyKey + ":load"})
	pluginID := firstNonEmpty(firstStringFromMap(response.Result, "plugin_id", "plugin_item_id", "item_id"), semanticEQFirstRecursive(semanticEQRecursiveText(response.Result), "plugin_id"))
	if err != nil || !strings.EqualFold(response.Status, "ok") || pluginID == "" {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("C2 dynamic plugin load failed: %s", firstNonEmpty(errorText(err), response.Error, "plugin identity missing"))
	}
	registry, regErr := processorregistry.Default()
	coverage, covErr := registry.PCARequiredCoverage(family, contextStringSlice(action.Args["required_coverage"]))
	if regErr != nil || covErr != nil {
		p.rollback(ctx, trackID, pluginID, idempotencyKey)
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Details: map[string]any{"plugin_id": pluginID, "rollback": "attempted"}}, firstNonNilError(regErr, covErr)
	}
	if _, admissionErr := p.server.semanticLoadedInstancePCAAdmission(ctx, trackID, pluginID, semanticTreatmentPCAInput{Family: family, RequiredCoverage: coverage}); admissionErr != nil {
		rollback, rollbackErr := p.rollback(ctx, trackID, pluginID, idempotencyKey)
		if rollbackErr != nil {
			admissionErr = fmt.Errorf("%w; rollback failed: %v", admissionErr, rollbackErr)
		}
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: admissionErr.Error(), Details: map[string]any{"track_id": trackID, "plugin_id": pluginID, "pca_admission": "failed", "rollback": rollback}}, admissionErr
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"track_id": trackID, "plugin_id": pluginID, "plugin_name": firstStringFromMap(candidate, "name"), "pca_admission": "pass", "structural_readback": "pass", "rollback": map[string]any{"status": "available"}}, EvidenceRefs: []string{"c2.dynamic_plugin_load:" + idempotencyKey}}, nil
}

func (p *c2DynamicPluginLoadPort) rollback(ctx context.Context, trackID, pluginID, idempotencyKey string) (map[string]any, error) {
	response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "plugin.delete", Args: map[string]any{"track_id": trackID, "plugin_item_id": pluginID}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID}, Source: "c2_dynamic_plugin_load_rollback", Confirmed: true, ToolCallID: idempotencyKey + ":rollback"})
	if err != nil || !strings.EqualFold(response.Status, "ok") {
		return map[string]any{"status": "failed", "plugin_id": pluginID}, fmt.Errorf("delete loaded C2 plugin: %s", firstNonEmpty(errorText(err), response.Error, response.Status))
	}
	return map[string]any{"status": "restored", "plugin_id": pluginID}, nil
}

type c2DynamicPluginLoadVerifier struct{}

func (c2DynamicPluginLoadVerifier) Verify(_ context.Context, _ orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	if len(receipts) != 1 || receipts[0].Status != "applied" || firstStringFromMap(receipts[0].Details, "pca_admission") != "pass" {
		return orchestration.VerificationResult{Status: "fail", Structural: "fail", Acoustic: "not_run", UserAcceptance: "unknown"}, fmt.Errorf("C2 loaded instance did not pass PCA")
	}
	return orchestration.VerificationResult{Status: "pass", Structural: "pass", Acoustic: "not_run", UserAcceptance: "unknown", Summary: "C2 dynamic plugin loaded and its exact live instance passed PCA admission."}, nil
}

func c2DynamicPluginLoadExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, executeErr error) ChatResponse {
	data := map[string]any{"session_id": session.ID, "capability_id": dynamicControlCapabilityID, "stage": "plugin_load_execution", "mutation_performed": false}
	if executeErr != nil {
		data["error"] = executeErr.Error()
	}
	if session.Execution != nil {
		data["execution_id"], data["execution_status"], data["receipts"], data["verification"] = session.Execution.ID, session.Execution.Status, session.Execution.Receipts, session.Execution.VerificationResult
	}
	if executeErr != nil || session.Execution == nil || len(session.Execution.Receipts) != 1 || session.Execution.Receipts[0].Status != "applied" {
		return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C2 动态插件加载或实例 PCA 验证未能完成；已按事务结果停止。", Error: errorText(executeErr), Workflow: "capability_runtime_v1", WorkflowData: data, GoalStatus: string(agentruntime.StatusFailed)}
	}
	data["mutation_performed"] = true
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C2 已加载 PCA 合格的动态插件实例；正在进入已冻结处理目标的动态参数规划。", Workflow: "capability_runtime_v1", WorkflowData: data, GoalStatus: string(agentruntime.StatusCompleted)}
}
