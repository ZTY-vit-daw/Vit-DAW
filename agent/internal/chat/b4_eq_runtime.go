package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
)

const (
	b4LoadBatchCommand = "b4.eq_plugin_load_batch.governed"
	b4EQBatchCommand   = "b4.semantic_eq_batch.governed"
)

type b4TargetResolution struct {
	Resolved  []b4EQTargetContext
	Missing   []lowendrelation.TreatmentTarget
	Ambiguous map[string][]semanticTreatmentInstance
}

func b4MutationRequested(text string, requestContext map[string]any) bool {
	if boolValue(requestContext["b4_execute"]) || boolValue(requestContext["execute"]) || strings.EqualFold(firstStringFromMap(requestContext, "b4_mode"), "apply") {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(text))
	if value == "" {
		return false
	}
	for _, readOnly := range []string{"只分析", "仅分析", "看看", "查看", "解释", "为什么", "分析一下", "analysis only", "analyze", "analyse", "inspect", "explain", "why"} {
		if strings.Contains(value, readOnly) {
			return false
		}
	}
	for _, action := range []string{"调整", "处理", "修复", "改善", "执行", "应用", "完成 b4", "做b4", "做 b4", "adjust", "fix", "apply", "execute", "treat", "complete b4", "run b4"} {
		if strings.Contains(value, action) {
			return true
		}
	}
	return false
}

func b4InstanceSelections(requestContext map[string]any) map[string]string {
	out := map[string]string{}
	for key, value := range firstMapFromAny(requestContext["b4_eq_instance_selection"]) {
		if pluginID := strings.TrimSpace(fmt.Sprint(value)); key != "" && pluginID != "" && pluginID != "<nil>" {
			out[strings.TrimSpace(key)] = pluginID
		}
	}
	for _, row := range mapRowsValue(requestContext["b4_eq_instance_selections"]) {
		if trackID, pluginID := firstStringFromMap(row, "track_id"), firstStringFromMap(row, "plugin_id"); trackID != "" && pluginID != "" {
			out[trackID] = pluginID
		}
	}
	return out
}

func (s *Server) resolveB4EQTargets(ctx context.Context, treatment lowendrelation.TreatmentPlan, requestContext map[string]any, preferred map[string]string) b4TargetResolution {
	result := b4TargetResolution{Ambiguous: map[string][]semanticTreatmentInstance{}}
	selections := b4InstanceSelections(requestContext)
	for _, target := range treatment.Targets {
		instances, _ := s.semanticTreatmentInstances(ctx, target.TrackID)
		qualified := semanticTreatmentQualifiedEQ(instances)
		selectedID := firstNonEmpty(preferred[target.TrackID], selections[target.TrackID])
		if selectedID != "" {
			if selected, ok := semanticTreatmentFindPlugin(qualified, selectedID); ok {
				result.Resolved = append(result.Resolved, b4EQTargetContext{Treatment: target, Instance: selected})
				continue
			}
		}
		switch len(qualified) {
		case 0:
			result.Missing = append(result.Missing, target)
		case 1:
			result.Resolved = append(result.Resolved, b4EQTargetContext{Treatment: target, Instance: qualified[0]})
		default:
			result.Ambiguous[target.TrackID] = qualified
		}
	}
	sort.SliceStable(result.Resolved, func(i, j int) bool { return result.Resolved[i].Treatment.Order < result.Resolved[j].Treatment.Order })
	return result
}

func b4PluginSelectionRequiredResponse(conversationID string, goal agentruntime.Goal, sessionID string, treatment lowendrelation.TreatmentPlan, resolution b4TargetResolution) ChatResponse {
	ambiguous := make([]map[string]any, 0, len(resolution.Ambiguous))
	for trackID, instances := range resolution.Ambiguous {
		rows := make([]map[string]any, 0, len(instances))
		for _, instance := range instances {
			rows = append(rows, map[string]any{"track_id": instance.TrackID, "plugin_id": instance.PluginID, "plugin_name": instance.PluginName, "qualification_status": instance.QualificationStatus})
		}
		ambiguous = append(ambiguous, map[string]any{"track_id": trackID, "instances": rows})
	}
	sort.SliceStable(ambiguous, func(i, j int) bool {
		return firstStringFromMap(ambiguous[i], "track_id") < firstStringFromMap(ambiguous[j], "track_id")
	})
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "B4 已完成全工程关系判断，但部分轨道有多个已资格化 EQ。请为每个列出的轨道选择一个精确 plugin_id；尚未加载的轨道会在选择完成后合并为一次插件加载确认。",
		Workflow: "plugin_selection_required", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{"schema_version": "plugin_selection_required.v1", "capability_id": lowEndRelationCapabilityID, "session_id": sessionID, "treatment_plan": treatment, "ambiguous_loaded_eq": ambiguous, "missing_track_count": len(resolution.Missing), "mutation_performed": false},
	}
}

func (s *Server) b4PluginSelectionResponse(conversationID string, goal agentruntime.Goal, sessionID string, treatment lowendrelation.TreatmentPlan, model lowendrelation.Model, missing []lowendrelation.TreatmentTarget, recommendation pluginRecommendationPlan, candidates []pluginRecommendationCandidate) ChatResponse {
	candidateByKey := map[string]pluginRecommendationCandidate{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	rows := []map[string]any{}
	actions := []AgentInteractionAction{}
	for index, choice := range recommendation.Choices {
		candidate, ok := candidateByKey[choice.CandidateKey]
		if !ok {
			continue
		}
		row := pluginRecommendationChoiceRow(choice, candidate)
		rows = append(rows, row)
		actions = append(actions, AgentInteractionAction{ID: "select_" + choice.CandidateKey, Label: "选择 " + candidate.Name, Style: map[bool]string{true: "primary", false: "secondary"}[index == 0], Description: choice.Reason, Recommended: index == 0, Value: map[string]any{"candidate_key": choice.CandidateKey}})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不加载", Style: "secondary"})
	payload := map[string]any{"schema_version": "plugin_selection_required.v1", "status": "awaiting_selection", "capability_id": lowEndRelationCapabilityID, "session_id": sessionID, "conversation_id": conversationID, "goal_id": goal.GoalID, "run_id": goal.RunID, "treatment_plan": treatment, "diagnosis_model": model, "missing_targets": missing, "recommendation": recommendation, "recommendations": rows, "mutation_performed": false}
	req := AgentInteractionRequest{ID: "interaction_" + randomID(), Kind: "b4_plugin_selection", Type: "b4_plugin_selection", Source: "b4_plugin_selection", Workflow: "plugin_selection_required", Stage: "awaiting_selection", Title: "B4 全工程 EQ 插件选择", Body: recommendation.Summary, Status: "waiting_for_user", ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Payload: payload, Data: payload, Actions: actions}
	s.storePendingInteraction(req, payload)
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: firstNonEmpty(recommendation.Summary, "B4 的部分目标没有已加载 EQ。") + "\n请选择一个本地 EQ；选择后只会生成一次项目级插件加载 Proposal，不会立即加载或修改参数。", Workflow: "plugin_selection_required", WorkflowData: payload, GoalStatus: string(agentruntime.StatusWaitingClarification), MessageKind: "proposal", InteractionRequests: []AgentInteractionRequest{req}}
}

func recoverB4PluginSelectionInteraction(interactionID string, payload map[string]any) (PendingInteraction, bool) {
	if firstStringFromMap(payload, "schema_version") != "plugin_selection_required.v1" || firstStringFromMap(payload, "capability_id") != lowEndRelationCapabilityID || firstStringFromMap(payload, "session_id") == "" || len(mapRowsValue(payload["missing_targets"])) == 0 {
		return PendingInteraction{}, false
	}
	return PendingInteraction{ID: interactionID, Kind: "b4_plugin_selection", Type: "b4_plugin_selection", Source: "b4_plugin_selection", Workflow: "plugin_selection_required", Stage: "awaiting_selection", ConversationID: firstStringFromMap(payload, "conversation_id"), GoalID: firstStringFromMap(payload, "goal_id"), RunID: firstStringFromMap(payload, "run_id"), Payload: cloneContext(payload), Data: cloneContext(payload)}, true
}

func (s *Server) continueB4PluginSelectionInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	payload := interaction.Payload
	if len(payload) == 0 {
		payload = interaction.Data
	}
	goal := agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID}
	if strings.EqualFold(decision, "cancel") || strings.Contains(strings.ToLower(decision), "cancel") {
		var cancelled orchestration.PlanningSession
		if sessionID := firstStringFromMap(payload, "session_id"); sessionID != "" {
			cancelled, _ = s.orchestrationRuntime.CancelPlanningSession(sessionID)
		}
		response := ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "已取消 B4 插件选择；没有加载插件或修改参数。", Workflow: "plugin_selection_required", GoalStatus: string(agentruntime.StatusCancelled), WorkflowData: map[string]any{"schema_version": "plugin_selection_required.v1", "status": "cancelled", "mutation_performed": false}}
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	key := strings.TrimPrefix(strings.TrimSpace(decision), "select_")
	var candidate pluginRecommendationCandidate
	found := false
	for _, row := range mapRowsValue(payload["recommendations"]) {
		if firstStringFromMap(row, "candidate_key") == key {
			_ = decodeAnyJSON(row, &candidate)
			candidate.Key = key
			found = candidate.PluginPath != ""
			break
		}
	}
	if !found {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "所选 B4 EQ 候选已过期；没有加载或写入。", Workflow: "plugin_selection_required", GoalStatus: string(agentruntime.StatusWaitingClarification), Error: "invalid B4 plugin candidate"}
	}
	var treatment lowendrelation.TreatmentPlan
	var model lowendrelation.Model
	var recommendation pluginRecommendationPlan
	var missing []lowendrelation.TreatmentTarget
	_ = decodeAnyJSON(payload["treatment_plan"], &treatment)
	_ = decodeAnyJSON(payload["diagnosis_model"], &model)
	_ = decodeAnyJSON(payload["recommendation"], &recommendation)
	_ = decodeAnyJSON(payload["missing_targets"], &missing)
	return s.createB4LoadProposal(ctx, interaction.ConversationID, goal, firstStringFromMap(payload, "session_id"), treatment, model, missing, candidate, recommendation)
}

func (s *Server) buildB4ActionableResponse(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, model lowendrelation.Model) ChatResponse {
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 项目级声学规划需要可用的 AI 配置；没有创建 Proposal："+firstNonEmpty(errorText(err), "configuration incomplete"))
	}
	treatment, err := s.planB4Treatment(ctx, conversationID, req.Message, model, cfg)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 全工程处理判断失败；没有创建 Proposal："+err.Error())
	}
	if len(treatment.Targets) == 0 {
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
		response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "B4 的全工程关系判断没有发现足够依据要求 EQ 调整；没有加载插件或修改参数。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"capability_id": lowEndRelationCapabilityID, "canary_stage": "no_treatment_required", "treatment_plan": treatment, "mutation_performed": false}}
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	resolution := s.resolveB4EQTargets(ctx, treatment, req.Context, nil)
	if len(resolution.Ambiguous) > 0 {
		selected, selectionErr := s.planB4InstanceSelections(ctx, conversationID, treatment, resolution.Ambiguous, cfg)
		if selectionErr != nil {
			return b4PluginSelectionRequiredResponse(conversationID, goal, sessionID, treatment, resolution)
		}
		resolution = s.resolveB4EQTargets(ctx, treatment, req.Context, selected)
		if len(resolution.Ambiguous) > 0 {
			return b4PluginSelectionRequiredResponse(conversationID, goal, sessionID, treatment, resolution)
		}
	}
	if len(resolution.Missing) > 0 {
		candidates, searchErr := s.localPluginRecommendationCandidates(ctx, "eq")
		candidates = genericStaticEQRecommendationCandidates(candidates)
		if searchErr != nil || len(candidates) == 0 {
			return capabilityCanaryBlockedResponse(conversationID, goal, "B4 需要为部分目标加载 EQ，但本地 EQ 候选不可用；没有加载或写入："+firstNonEmpty(errorText(searchErr), "no local EQ candidates"))
		}
		recommendation, recommendErr := s.planPluginRecommendation(ctx, conversationID, treatment.Summary, "eq", map[string]any{"b4_full_project": true}, nil, candidates, cfg)
		if recommendErr != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "B4 本地 EQ 推荐失败；没有加载或写入："+recommendErr.Error())
		}
		candidateByKey := map[string]pluginRecommendationCandidate{}
		for _, candidate := range candidates {
			candidateByKey[candidate.Key] = candidate
		}
		_, ok := candidateByKey[recommendation.Choices[0].CandidateKey]
		if !ok {
			return capabilityCanaryBlockedResponse(conversationID, goal, "B4 EQ 推荐没有绑定本地精确候选；没有加载或写入。")
		}
		return s.b4PluginSelectionResponse(conversationID, goal, sessionID, treatment, model, resolution.Missing, recommendation, candidates)
	}
	return s.createB4EQProposal(ctx, conversationID, goal, sessionID, treatment, model, resolution.Resolved, cfg)
}

func (s *Server) createB4LoadProposal(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, treatment lowendrelation.TreatmentPlan, model lowendrelation.Model, missing []lowendrelation.TreatmentTarget, candidate pluginRecommendationCandidate, recommendation pluginRecommendationPlan) ChatResponse {
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 插件加载方案无法取得强工程快照；没有加载。")
	}
	loads := make([]map[string]any, 0, len(missing))
	targetScope := make([]string, 0, len(missing))
	for _, target := range missing {
		instances, _ := s.semanticTreatmentInstances(ctx, target.TrackID)
		before := make([]string, 0, len(instances))
		for _, instance := range instances {
			before = append(before, instance.PluginID)
		}
		loads = append(loads, map[string]any{"track_id": target.TrackID, "track_name": target.TrackName, "plugin_path": candidate.PluginPath, "plugin_identifier": candidate.Identifier, "plugin_name": candidate.Name, "preexisting_plugin_ids": before})
		targetScope = append(targetScope, "track:"+target.TrackID)
	}
	decisionRefs, _ := mixboardDecisionContextForState(state.LegacyState, lowEndRelationCapabilityID)
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: []string{"b4-treatment:" + treatment.PlanID, "plugin-candidate:" + candidate.PluginPath}, TargetFingerprints: targetScope, ArtifactRefs: mixboardDecisionArtifactRefs(decisionRefs), ContractVersions: []string{"capability:" + lowEndRelationCapabilityID, "plugin-load:rack_add_node", "rollback:delete_plugin"}})
	if err != nil || !cut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 插件加载方案无法建立强 ProjectCut；没有加载。")
	}
	action := orchestration.Action{ID: "b4_eq_plugin_load_batch", Command: b4LoadBatchCommand, TargetRef: "project:b4_missing_eq", BeforeFingerprint: "b4-load:" + semanticEQHash(loads), Args: map[string]any{"loads": loads, "treatment_plan": treatment, "diagnosis_model": model, "recommendation": recommendation, "candidate": candidate}, Compensatable: true, IdempotencyClass: "b4_plugin_load_all_or_rollback"}
	actionSet := orchestration.ActionSet{ID: "actionset_b4_eq_load_" + sanitizeCanaryID(treatment.PlanID), CapabilityID: lowEndRelationCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "proposal_" + actionSet.Hash[:16], Revision: 1, CapabilityID: lowEndRelationCapabilityID, CapabilityVer: "v0", ProjectCutHash: cut.Hash, CandidateID: "b4_load_" + actionSet.Hash[:16], ActionSetHash: actionSet.Hash, TargetScope: targetScope, Risk: "bounded_reversible", VerificationRef: "static_mix.low_end_relation.plugin_load.verification.v1", Summary: fmt.Sprintf("Load recommended local EQ %s on %d B4 target tracks", candidate.Name, len(loads)), CreatedAt: time.Now().UTC()}
	proposal.Presentation = b4LoadProposalPresentation(proposal, loads, candidate, recommendation)
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_b4_load_" + sanitizeCanaryID(treatment.PlanID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 插件加载 Proposal 持久化失败；没有加载："+err.Error())
	}
	return b4ProposalResponse(conversationID, goal, session)
}

func b4LoadProposalPresentation(proposal orchestration.Proposal, loads []map[string]any, candidate pluginRecommendationCandidate, recommendation pluginRecommendationPlan) *orchestration.ProposalPresentation {
	actions := make([]orchestration.ProposalActionPreview, 0, len(loads))
	for index, load := range loads {
		actions = append(actions, orchestration.ProposalActionPreview{ActionID: fmt.Sprintf("load_%d", index+1), TrackID: firstStringFromMap(load, "track_id"), TrackName: firstStringFromMap(load, "track_name"), Operation: "load_eq_plugin", Reason: "B4 target currently has no qualified generic EQ"})
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: "B4 全工程 EQ 插件加载方案", Conclusion: fmt.Sprintf("推荐 %s，并在 %d 个缺失目标上批量加载。", candidate.Name, len(loads)), AnalysisSummary: []string{recommendation.Summary}, Recommendation: proposal.Summary, ActionCount: len(loads), Risk: proposal.Risk, Reversible: true, Actions: actions, Limitations: []string{"本次确认只授权插件加载；不会写入 EQ 参数。", "任一加载或资格确认失败将删除本批已新增的全部实例。"}, ApprovalPrompt: "是否确认本次项目级插件加载？参数调整将在加载资格确认后另行生成 Proposal。"}
}

func (s *Server) createB4EQProposal(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, treatment lowendrelation.TreatmentPlan, model lowendrelation.Model, targets []b4EQTargetContext, cfg config.EngineConfig) ChatResponse {
	batch, err := s.planB4EQBatch(ctx, conversationID, treatment, targets, cfg, "")
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 项目级 EQ 声学规划失败；没有写入："+err.Error())
	}
	leaves, materializeErr := s.materializeB4EQLeaves(ctx, treatment, batch)
	if materializeErr != nil {
		batch, err = s.planB4EQBatch(ctx, conversationID, treatment, targets, cfg, materializeErr.Error())
		if err == nil {
			leaves, materializeErr = s.materializeB4EQLeaves(ctx, treatment, batch)
		}
	}
	if err != nil || materializeErr != nil {
		return b4EQRejectedResponse(conversationID, goal, treatment, "materialization_rejected", firstNonEmpty(errorText(materializeErr), errorText(err)))
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 EQ Proposal 无法取得强工程快照；没有写入。")
	}
	baseline, baselineErr := s.collectB4TargetPostFXBaseline(ctx, sessionID, goal.Summary, "pre_parameter", state, treatment, model, false)
	if baselineErr != nil {
		return b4TargetPreflightBlockedResponse(conversationID, goal, sessionID, treatment, baseline, baselineErr)
	}
	dependencies := []string{"b4-treatment:" + treatment.PlanID, "b4-target-post-fx:" + baseline.BaselineID, "b4-semantic-batch:" + semanticEQHash(batch)}
	targetFingerprints := []string{}
	for _, leaf := range leaves {
		targetFingerprints = append(targetFingerprints, "plugin:"+leaf.Action.Target.TrackID+":"+leaf.Action.Target.PluginID, "eq-topology:"+leaf.TopologyGeneration)
		for _, row := range leaf.ParameterPreimage {
			targetFingerprints = append(targetFingerprints, fmt.Sprintf("eq-param:%s:%0.9f", firstStringFromMap(row, "param_id"), semanticEQNumber(row["normalized"])))
		}
	}
	decisionRefs, _ := mixboardDecisionContextForState(state.LegacyState, lowEndRelationCapabilityID)
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: dependencies, TargetFingerprints: targetFingerprints, ArtifactRefs: appendUniqueStrings(append(append([]string(nil), treatment.EvidenceRefs...), baseline.EvidenceRefs...), mixboardDecisionArtifactRefs(decisionRefs)...), ContractVersions: []string{"capability:" + lowEndRelationCapabilityID, "treatment:" + lowendrelation.TreatmentPlanSchema, "baseline:" + lowendrelation.TargetPostFXBaselineSchema, "batch:" + semanticeffect.BatchSchema, "action:" + semanticeffect.ActionSchema, "executor:plugin_grabber.apply_eq_edits"}})
	if err != nil || !cut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 EQ Proposal 无法建立强 ProjectCut；没有写入。")
	}
	proposal, actionSet, err := capabilityadapters.FreezeSemanticEQBatch(capabilityadapters.SemanticEQBatchPlan{Treatment: treatment, Diagnosis: model, VerificationContext: baseline, Batch: batch, Leaves: leaves}, cut, 1)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 EQ 批冻结失败；没有写入："+err.Error())
	}
	proposal.Presentation = b4EQProposalPresentation(proposal, batch, leaves)
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_b4_eq_" + sanitizeCanaryID(treatment.PlanID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 EQ Proposal 持久化失败；没有写入："+err.Error())
	}
	return b4ProposalResponse(conversationID, goal, session)
}

func (s *Server) collectB4TargetPostFXBaseline(ctx context.Context, sessionID, goalText, phase string, state *kernel.VSPStateResult, treatment lowendrelation.TreatmentPlan, model lowendrelation.Model, forceFresh bool) (lowendrelation.TargetPostFXBaseline, error) {
	trackIDs := lowendrelation.TargetPostFXTrackIDs(treatment, model)
	if len(trackIDs) == 0 {
		return lowendrelation.TargetPostFXBaseline{}, fmt.Errorf("B4 target preflight has no treatment target or referenced relationship peer")
	}
	if s == nil || s.harness == nil {
		return lowendrelation.TargetPostFXBaseline{}, fmt.Errorf("B4 target post-FX collector is unavailable")
	}
	collected, err := s.harness.CollectL2RenderProbeBatch(ctx, harness.L2RenderProbeBatchRequest{SessionID: sessionID, GoalText: goalText, TrackIDs: trackIDs, TapPoint: "track_post_fader", ForceFresh: forceFresh})
	projectID := firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid"), firstStringFromMap(firstMapFromAny(state.LegacyState["project"]), "project_uuid", "uuid", "id"), "current")
	baseline := lowendrelation.BuildTargetPostFXBaseline(sessionID, phase, projectID, trackIDs, collected.Rows, time.Now().UTC())
	baseline.Collection = lowendrelation.TargetBaselineCollection{ElapsedMS: collected.ElapsedMS, RenderedTrackIDs: append([]string(nil), collected.RenderedTrackIDs...), CacheHitTrackIDs: append([]string(nil), collected.CacheHitTrackIDs...)}
	if err != nil {
		return baseline, err
	}
	if !baseline.Readiness.CanProceed {
		return baseline, fmt.Errorf("B4 target post-FX readiness blocked: %s", strings.Join(baseline.Readiness.BlockedBy, ", "))
	}
	return baseline, nil
}

func b4TargetPreflightBlockedResponse(conversationID string, goal agentruntime.Goal, sessionID string, treatment lowendrelation.TreatmentPlan, baseline lowendrelation.TargetPostFXBaseline, err error) ChatResponse {
	reason := firstNonEmpty(errorText(err), strings.Join(baseline.Readiness.BlockedBy, ", "), "target post-FX evidence unavailable")
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "B4 completed relationship planning, but exact target/peer post-FX preflight is blocked; no EQ parameter Proposal or write was created: " + reason, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingContinue), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": lowEndRelationCapabilityID, "canary_stage": "target_preflight_blocked", "treatment_plan": treatment, "target_post_fx_baseline": baseline, "mutation_performed": false}}
}

func b4EQRejectedResponse(conversationID string, goal agentruntime.Goal, treatment lowendrelation.TreatmentPlan, code, reason string) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "B4 项目级 EQ 方案被确定性边界拒绝；没有加载或写入参数：" + reason, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusFailed), Error: reason, WorkflowData: map[string]any{"capability_id": lowEndRelationCapabilityID, "stage": "eq_plan_rejected", "status": "rejected", "rejection_code": code, "rejection_reason": reason, "treatment_plan_id": treatment.PlanID, "requested_target_count": len(treatment.Targets), "writes_performed": 0, "mutation_performed": false}}
}

func (s *Server) materializeB4EQLeaves(ctx context.Context, treatment lowendrelation.TreatmentPlan, batch semanticeffect.Batch) ([]capabilityadapters.SemanticEQPlan, error) {
	return s.materializeProjectSemanticEQLeaves(ctx, batch, treatment.EvidenceRefs, treatment.ObservationID, "full_project_b4")
}

func (s *Server) materializeProjectSemanticEQLeaves(ctx context.Context, batch semanticeffect.Batch, evidenceRefs []string, observationID, verificationScope string) ([]capabilityadapters.SemanticEQPlan, error) {
	out := make([]capabilityadapters.SemanticEQPlan, 0, len(batch.Actions))
	for index, action := range batch.Actions {
		materialized, err := s.planSemanticEQReadOnly(ctx, action)
		if err != nil {
			return nil, fmt.Errorf("target %d %s/%s: %w", index+1, action.Target.TrackID, action.Target.PluginID, err)
		}
		out = append(out, capabilityadapters.SemanticEQPlan{Action: action, TopologyGeneration: materialized.DigestGeneration, Edits: materialized.Edits, PlannedEdits: materialized.PlannedEdits, PlannedWrites: materialized.PlannedWrites, ParameterPreimage: materialized.Preimage, ParameterSnapshot: materialized.Snapshot, PreviewResults: materialized.PreviewResults, EvidenceRefs: append([]string(nil), evidenceRefs...), PreviousObservation: observationID, AcousticVerification: verificationScope})
	}
	return out, nil
}

func b4EQProposalPresentation(proposal orchestration.Proposal, batch semanticeffect.Batch, leaves []capabilityadapters.SemanticEQPlan) *orchestration.ProposalPresentation {
	actions := []orchestration.ProposalActionPreview{}
	analysis := []string{}
	limitations := []string{"本次确认仅授权 EQ 参数批；插件加载已独立确认。", "任一目标失败会按冻结预像逆序恢复本批所有已执行目标。"}
	for index, leaf := range leaves {
		analysis = append(analysis, fmt.Sprintf("%d. %s / %s：%s", index+1, leaf.Action.Target.TrackName, leaf.Action.Target.PluginName, leaf.Action.UserGoal))
		for atomIndex, atom := range leaf.Action.EQPlan.Atoms {
			status := "exact"
			if atomIndex < len(leaf.PreviewResults) {
				status = firstStringFromMap(leaf.PreviewResults[atomIndex], "status")
			}
			frequency, gain := 0.0, 0.0
			if atom.FrequencyHz != nil {
				frequency = *atom.FrequencyHz
			}
			if atom.GainDB != nil {
				gain = *atom.GainDB
			}
			actions = append(actions, orchestration.ProposalActionPreview{ActionID: atom.AtomID, TrackID: leaf.Action.Target.TrackID, TrackName: leaf.Action.Target.TrackName, Operation: atom.Action + ":" + atom.Shape, Target: frequency, Delta: gain, Unit: "Hz/dB", Reason: atom.Purpose + "; preview=" + status})
		}
		limitations = append(limitations, leaf.Action.Limitations...)
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: "B4 全工程抽象 EQ 方案", Conclusion: fmt.Sprintf("对 %d 个精确轨道/EQ 实例执行一个项目级原子批。", len(batch.Actions)), AnalysisSummary: analysis, Recommendation: proposal.Summary, ActionCount: len(actions), Risk: proposal.Risk, Reversible: true, Actions: actions, Limitations: semanticEQUniqueStrings(limitations), ApprovalPrompt: "是否确认执行这个冻结的项目级 B4 EQ 参数批？"}
}

func b4ProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 Session 缺少冻结 Proposal。")
	}
	proposal := session.ActiveProposal
	stage := "eq_parameter_confirmation"
	if len(session.FrozenPlan.ActionSet.Actions) == 1 && session.FrozenPlan.ActionSet.Actions[0].Command == b4LoadBatchCommand {
		stage = "plugin_load_confirmation"
	}
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: proposal.Presentation.Conclusion + "\n\n" + proposal.Presentation.ApprovalPrompt, NeedsConfirmation: true, PlanID: proposal.ID, Preview: proposal.Presentation.Conclusion, ProposalPresentation: proposal.Presentation, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: map[string]any{"session_id": session.ID, "capability_id": lowEndRelationCapabilityID, "proposal_id": proposal.ID, "proposal_revision": proposal.Revision, "action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash, "stage": stage, "writes_performed": 0, "proposal_presentation": proposal.Presentation}}
}

type projectEQBatchMutationSpec struct {
	CapabilityID string
	LoadCommand  string
	EQCommand    string
	SourcePrefix string
}

type projectEQBatchMutationPort struct {
	server *Server
	spec   projectEQBatchMutationSpec
}

// Keep the historical B4 name as an alias so persisted recovery and existing
// callers reuse the same implementation while C1 supplies its own identities.
type b4BatchMutationPort = projectEQBatchMutationPort

func (p *projectEQBatchMutationPort) normalizedSpec() projectEQBatchMutationSpec {
	spec := projectEQBatchMutationSpec{}
	if p != nil {
		spec = p.spec
	}
	if spec.CapabilityID == "" {
		spec.CapabilityID = lowEndRelationCapabilityID
	}
	if spec.LoadCommand == "" {
		spec.LoadCommand = b4LoadBatchCommand
	}
	if spec.EQCommand == "" {
		spec.EQCommand = b4EQBatchCommand
	}
	if spec.SourcePrefix == "" {
		spec.SourcePrefix = "b4"
	}
	return spec
}

func (p *projectEQBatchMutationPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	spec := p.normalizedSpec()
	if p == nil || p.server == nil || p.server.kernel == nil || actionSet.CapabilityID != spec.CapabilityID || len(actionSet.Actions) != 1 || actionSet.Hash != actionSet.ComputeHash() || actionSet.ProjectCutHash != cut.Hash || !cut.IsExecutable() {
		return fmt.Errorf("invalid frozen project EQ action set/project cut")
	}
	frozenAction := actionSet.Actions[0]
	if !frozenAction.Compensatable || (frozenAction.Command != spec.LoadCommand && frozenAction.Command != spec.EQCommand) {
		return fmt.Errorf("unsupported or non-compensatable project EQ batch action")
	}
	baseRevision, err := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if err != nil || baseRevision <= 0 {
		return fmt.Errorf("invalid frozen base revision")
	}
	state, err := p.server.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() || state.ProjectEpoch != cut.ProjectEpoch || state.Revision != baseRevision {
		return fmt.Errorf("stale_project_cut: current epoch/revision changed")
	}
	if frozenAction.Command == spec.LoadCommand {
		loads := mapRowsValue(frozenAction.Args["loads"])
		if len(loads) == 0 {
			return fmt.Errorf("project EQ load batch is empty")
		}
		for index, load := range loads {
			if firstStringFromMap(load, "track_id") == "" || firstStringFromMap(load, "plugin_path") == "" {
				return fmt.Errorf("project EQ load %d lacks exact track or local plugin path", index+1)
			}
		}
	}
	if frozenAction.Command == spec.EQCommand {
		leaves := mapRowsValue(frozenAction.Args["leaves"])
		if len(leaves) == 0 {
			return fmt.Errorf("project EQ batch is empty")
		}
		for index, row := range leaves {
			action, err := semanticEQActionFromAny(row["semantic_action"])
			if err != nil {
				return err
			}
			current, err := p.server.planSemanticEQReadOnly(ctx, action)
			if err != nil {
				return err
			}
			if current.DigestGeneration != firstStringFromMap(row, "topology_generation") || semanticEQHash(current.Edits) != semanticEQHash(row["edits"]) || semanticEQHash(current.PlannedWrites) != semanticEQHash(row["planned_writes"]) || semanticEQHash(current.Preimage) != semanticEQHash(row["parameter_preimage"]) {
				return fmt.Errorf("stale_eq_leaf_%d: topology or preimage changed", index+1)
			}
		}
	}
	return nil
}

func (p *projectEQBatchMutationPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	spec := p.normalizedSpec()
	switch action.Command {
	case spec.LoadCommand:
		return p.applyLoads(ctx, action, idempotencyKey)
	case spec.EQCommand:
		return p.applyEQ(ctx, action, idempotencyKey)
	default:
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("unsupported project EQ command %s", action.Command)
	}
}

// Reconcile never retries a B4 mutation blindly. It either proves the whole
// top-level batch from fresh state or restores all deterministically
// identifiable partial effects to their frozen preimages.
func (p *projectEQBatchMutationPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	spec := p.normalizedSpec()
	switch action.Command {
	case spec.EQCommand:
		leaves := mapRowsValue(action.Args["leaves"])
		allMatching := len(leaves) > 0
		for _, leaf := range leaves {
			rows, err := p.server.readEQParameters(ctx, firstStringFromMap(leaf, "track_id"), firstStringFromMap(leaf, "plugin_id"))
			index := eqParameterRowIndex(rows)
			if err != nil || !b4WritesMatch(index, mapRowsValue(leaf["planned_writes"])) {
				allMatching = false
				break
			}
		}
		if allMatching {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"structural_readback": "pass", "recovery": true, "target_count": len(leaves)}, EvidenceRefs: []string{spec.SourcePrefix + ".semantic_eq_batch.reconcile:" + idempotencyKey}}, nil
		}
		rollback, rollbackErr := p.rollbackLeaves(ctx, leaves)
		details := map[string]any{"structural_readback": "fail", "recovery": true, "rollback": map[string]any{"attempted": true, "status": map[bool]string{true: "failed", false: "restored"}[rollbackErr != nil], "targets": rollback}}
		if rollbackErr != nil {
			details["rollback"].(map[string]any)["error"] = rollbackErr.Error()
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed_closed", Error: rollbackErr.Error(), Details: details}, rollbackErr
		}
		err := fmt.Errorf("partial or ambiguous project EQ outcome; every frozen leaf preimage restored")
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error(), Details: details}, err
	case spec.LoadCommand:
		loaded := []map[string]any{}
		for _, load := range mapRowsValue(action.Args["loads"]) {
			before := map[string]bool{}
			for _, id := range stringListValue(load["preexisting_plugin_ids"]) {
				before[id] = true
			}
			instances, _ := p.server.semanticTreatmentInstances(ctx, firstStringFromMap(load, "track_id"))
			newInstances := projectEQNewInstances(instances, before)
			newQualified := semanticTreatmentQualifiedEQ(newInstances)
			if len(newInstances) != 1 || len(newQualified) != 1 {
				for _, instance := range newInstances {
					loaded = append(loaded, map[string]any{"track_id": instance.TrackID, "plugin_id": instance.PluginID, "plugin_item_id": instance.PluginID, "plugin_name": instance.PluginName, "qualification_status": instance.QualificationStatus})
				}
				rollback := map[string]any{"attempted": false, "status": "not_needed"}
				var rollbackErr error
				if len(loaded) > 0 {
					rollback, rollbackErr = p.rollbackLoads(ctx, loaded, idempotencyKey)
				}
				err := fmt.Errorf("ambiguous project EQ plugin-load recovery on track %s", firstStringFromMap(load, "track_id"))
				status := "failed"
				if rollbackErr != nil {
					status = "failed_closed"
					err = fmt.Errorf("%v; recovery rollback failed: %w", err, rollbackErr)
				}
				return orchestration.ActionReceipt{ActionID: action.ID, Status: status, Error: err.Error(), Details: map[string]any{"recovery": true, "loaded": loaded, "structural_readback": "fail", "rollback": rollback}}, err
			}
			loaded = append(loaded, map[string]any{"track_id": newQualified[0].TrackID, "plugin_id": newQualified[0].PluginID, "plugin_name": newQualified[0].PluginName, "qualification_status": newQualified[0].QualificationStatus})
		}
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"recovery": true, "loaded": loaded, "structural_readback": "pass", "qualification_status": "pass"}, EvidenceRefs: []string{spec.SourcePrefix + ".plugin_load_batch.reconcile:" + idempotencyKey}}, nil
	default:
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed_closed"}, fmt.Errorf("unsupported project EQ recovery command %s", action.Command)
	}
}

func projectEQNewInstances(instances []semanticTreatmentInstance, preexisting map[string]bool) []semanticTreatmentInstance {
	out := make([]semanticTreatmentInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.PluginID != "" && !preexisting[instance.PluginID] {
			out = append(out, instance)
		}
	}
	return out
}

func b4WritesMatch(index map[string]map[string]any, writes []map[string]any) bool {
	if len(writes) == 0 {
		return false
	}
	for _, write := range writes {
		id := firstStringFromMap(write, "param_id")
		expected, expectedOK := firstNumericAny(write, "normalized_value")
		actual, actualOK := firstNumericAny(index[id], "normalized_value", "normalised_value", "current_normalized_value")
		if id == "" || !expectedOK || !actualOK || math.Abs(expected-actual) > 1e-4 {
			return false
		}
	}
	return true
}

func (p *projectEQBatchMutationPort) applyEQ(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	spec := p.normalizedSpec()
	leaves := mapRowsValue(action.Args["leaves"])
	receipts := make([]orchestration.ActionReceipt, 0, len(leaves))
	applied := []map[string]any{}
	for index, leaf := range leaves {
		leafAction := orchestration.Action{ID: fmt.Sprintf("%s_eq_leaf_%d", spec.SourcePrefix, index+1), Command: "plugin_grabber.apply_eq_edits.governed", TargetRef: "plugin:" + firstStringFromMap(leaf, "track_id") + ":" + firstStringFromMap(leaf, "plugin_id"), Args: cloneContext(leaf), Compensatable: true}
		receipt, err := (&semanticEQMutationPort{server: p.server}).Apply(ctx, leafAction, idempotencyKey+":"+leafAction.ID)
		receipts = append(receipts, receipt)
		if err != nil {
			rollbackRows, rollbackErr := p.rollbackLeaves(ctx, applied)
			details := map[string]any{"leaf_receipts": receipts, "structural_readback": "fail", "failed_leaf_index": index + 1, "rollback": map[string]any{"attempted": len(applied) > 0, "status": map[bool]string{true: "failed", false: "restored"}[rollbackErr != nil], "targets": rollbackRows}}
			if rollbackErr != nil {
				details["rollback"].(map[string]any)["error"] = rollbackErr.Error()
			}
			combined := err
			if rollbackErr != nil {
				combined = fmt.Errorf("leaf failed: %v; project rollback failed: %w", err, rollbackErr)
			}
			return orchestration.ActionReceipt{ActionID: action.ID, Status: map[bool]string{true: "failed_closed", false: "failed"}[rollbackErr != nil], Error: combined.Error(), Details: details}, combined
		}
		applied = append(applied, leaf)
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"status": "applied", "structural_readback": "pass", "leaf_receipts": receipts, "target_count": len(leaves), "rollback": map[string]any{"attempted": false, "status": "available_from_frozen_preimages"}}, EvidenceRefs: []string{spec.SourcePrefix + ".semantic_eq_batch:" + idempotencyKey}}, nil
}

func (p *projectEQBatchMutationPort) rollbackLeaves(ctx context.Context, applied []map[string]any) ([]map[string]any, error) {
	rows := []map[string]any{}
	var failures []string
	for index := len(applied) - 1; index >= 0; index-- {
		leaf := applied[index]
		preimage, decodeErr := semanticEQPreimageValues(leaf["parameter_preimage"])
		err := decodeErr
		if err == nil {
			err = p.server.rollbackEQTransaction(ctx, firstStringFromMap(leaf, "track_id"), firstStringFromMap(leaf, "plugin_id"), preimage)
		}
		status := "restored"
		if err != nil {
			status = "failed"
			failures = append(failures, err.Error())
		}
		rows = append(rows, map[string]any{"track_id": firstStringFromMap(leaf, "track_id"), "plugin_id": firstStringFromMap(leaf, "plugin_id"), "status": status, "error": errorText(err)})
	}
	if len(failures) > 0 {
		return rows, fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return rows, nil
}

func (p *projectEQBatchMutationPort) applyLoads(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	spec := p.normalizedSpec()
	loaded := []map[string]any{}
	for index, load := range mapRowsValue(action.Args["loads"]) {
		args := map[string]any{"track_id": firstStringFromMap(load, "track_id"), "plugin_path": firstStringFromMap(load, "plugin_path"), "plugin_name": firstStringFromMap(load, "plugin_name"), "plugin_kind": "Effect", "zone_id": "Z3", "x": 360.0, "y": 260.0}
		if identifier := firstStringFromMap(load, "plugin_identifier"); identifier != "" {
			args["plugin_identifier"] = identifier
		}
		response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "rack.add_node", Args: args, Context: map[string]any{"capability_runtime_v1": true, "capability_id": spec.CapabilityID}, Source: spec.SourcePrefix + "_project_eq_plugin_load", Confirmed: true, ToolCallID: fmt.Sprintf("%s:load:%d", idempotencyKey, index+1)})
		pluginID := firstNonEmpty(firstStringFromMap(response.Result, "plugin_id", "plugin_item_id", "item_id"), semanticEQFirstRecursive(semanticEQRecursiveText(response.Result), "plugin_id"))
		pluginItemID := firstNonEmpty(firstStringFromMap(response.Result, "plugin_item_id", "item_id", "plugin_id"), pluginID)
		if err != nil || !strings.EqualFold(response.Status, "ok") || pluginID == "" {
			rollback, rollbackErr := p.rollbackLoads(ctx, loaded, idempotencyKey)
			failure := fmt.Errorf("load target %s failed: %s", firstStringFromMap(load, "track_id"), firstNonEmpty(errorText(err), response.Error, response.Status, "plugin identity missing"))
			if rollbackErr != nil {
				failure = fmt.Errorf("%v; load rollback failed: %w", failure, rollbackErr)
			}
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: failure.Error(), Details: map[string]any{"loaded": loaded, "rollback": rollback, "failed_index": index + 1}}, failure
		}
		row := map[string]any{"track_id": firstStringFromMap(load, "track_id"), "plugin_id": pluginID, "plugin_item_id": pluginItemID, "plugin_name": firstStringFromMap(load, "plugin_name")}
		instance, qualificationErr := p.qualifyLoadedEQ(ctx, firstStringFromMap(load, "track_id"), pluginID)
		qualified := qualificationErr == nil
		if !qualified {
			loaded = append(loaded, row)
			rollback, rollbackErr := p.rollbackLoads(ctx, loaded, idempotencyKey)
			failure := fmt.Errorf("loaded instance %s did not qualify as generic static EQ: %v", pluginID, qualificationErr)
			if rollbackErr != nil {
				failure = fmt.Errorf("%v; load rollback failed: %w", failure, rollbackErr)
			}
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: failure.Error(), Details: map[string]any{"loaded": loaded, "rollback": rollback, "qualification_status": "failed"}}, failure
		}
		row["qualification_status"] = instance.QualificationStatus
		loaded = append(loaded, row)
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"loaded": loaded, "structural_readback": "pass", "qualification_status": "pass", "rollback": map[string]any{"attempted": false, "status": "available"}}, EvidenceRefs: []string{spec.SourcePrefix + ".plugin_load_batch:" + idempotencyKey}}, nil
}

func (p *projectEQBatchMutationPort) qualifyLoadedEQ(ctx context.Context, trackID, pluginID string) (semanticTreatmentInstance, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		instances, _ := p.server.semanticTreatmentInstances(ctx, trackID)
		instance, found := semanticTreatmentFindPlugin(instances, pluginID)
		if found && instance.QualificationStatus == "generic_static_eq_qualified" {
			return instance, nil
		}
		if !found {
			lastErr = fmt.Errorf("instance is not visible on exact track %s after load", trackID)
		} else {
			lastErr = fmt.Errorf("%s", firstNonEmpty(instance.Limitation, instance.QualificationStatus, "generic EQ topology unavailable"))
		}
		if attempt < 5 {
			timer := time.NewTimer(time.Duration(attempt) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return semanticTreatmentInstance{}, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return semanticTreatmentInstance{}, lastErr
}

func (p *projectEQBatchMutationPort) rollbackLoads(ctx context.Context, loaded []map[string]any, idempotencyKey string) (map[string]any, error) {
	spec := p.normalizedSpec()
	rows := []map[string]any{}
	var failures []string
	for index := len(loaded) - 1; index >= 0; index-- {
		row := loaded[index]
		response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "plugin.delete", Args: b4PluginDeleteArgs(row), Context: map[string]any{"capability_runtime_v1": true, "capability_id": spec.CapabilityID}, Source: spec.SourcePrefix + "_project_eq_plugin_load_rollback", Confirmed: true, ToolCallID: fmt.Sprintf("%s:rollback:%d", idempotencyKey, index+1)})
		status := "restored"
		if err != nil || !strings.EqualFold(response.Status, "ok") {
			status = "failed"
			failures = append(failures, firstNonEmpty(errorText(err), response.Error, response.Status))
		}
		rows = append(rows, map[string]any{"track_id": firstStringFromMap(row, "track_id"), "plugin_id": firstStringFromMap(row, "plugin_id"), "status": status})
	}
	result := map[string]any{"attempted": len(loaded) > 0, "status": "restored", "instances": rows}
	if len(failures) > 0 {
		result["status"] = "failed"
		result["errors"] = failures
		return result, fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return result, nil
}

func b4PluginDeleteArgs(row map[string]any) map[string]any {
	return map[string]any{
		"track_id":       firstStringFromMap(row, "track_id"),
		"plugin_item_id": firstStringFromMap(row, "plugin_item_id", "plugin_id"),
	}
}

type b4BatchVerifier struct {
	server              *Server
	treatment           lowendrelation.TreatmentPlan
	before              lowendrelation.Model
	beforePostFX        lowendrelation.TargetPostFXBaseline
	sessionID, goalText string
}

func (v b4BatchVerifier) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if len(receipts) != 1 || !strings.EqualFold(receipts[0].Status, "applied") || !strings.EqualFold(firstStringFromMap(receipts[0].Details, "structural_readback"), "pass") {
		result.Status, result.Structural = "fail", "fail"
		return result, fmt.Errorf("B4 batch structural readback failed")
	}
	result.Structural = "pass"
	result.EvidenceRefs = append(result.EvidenceRefs, receipts[0].EvidenceRefs...)
	if actionSet.Actions[0].Command == b4LoadBatchCommand {
		result.Status, result.Acoustic, result.SpecialistRelationship = "pass", "not_run", "not_run"
		result.Summary = "插件加载与每个实际实例的 generic EQ topology 资格确认通过；尚未执行 EQ 参数。"
		return result, nil
	}
	return v.verifyTargetPostFX(ctx, result)
}

func (v b4BatchVerifier) verifyTargetPostFX(ctx context.Context, result orchestration.VerificationResult) (orchestration.VerificationResult, error) {
	if v.server == nil || v.server.harness == nil || v.server.kernel == nil || v.beforePostFX.SchemaVersion != lowendrelation.TargetPostFXBaselineSchema || !v.beforePostFX.Readiness.CanProceed {
		result.Acoustic = "unavailable"
		result.SpecialistRelationship = "inconclusive"
		result.Summary = "Structural readback passed, but the frozen B4 target/peer post-FX baseline is unavailable."
		return result, nil
	}
	state, stateErr := v.server.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if stateErr != nil || state == nil || !state.OK() {
		result.Acoustic = "inconclusive"
		result.SpecialistRelationship = "inconclusive"
		result.Summary = "Structural readback passed, but the B4 verification project snapshot is unavailable."
		return result, nil
	}
	after, collectErr := v.server.collectB4TargetPostFXBaseline(ctx, v.sessionID, v.goalText, "post_execution", state, v.treatment, v.before, true)
	if collectErr != nil {
		result.Acoustic = "inconclusive"
		result.SpecialistRelationship = "inconclusive"
		result.Summary = "Structural readback passed, but fresh B4 target/peer post-FX verification failed: " + collectErr.Error()
		return result, nil
	}
	verification := lowendrelation.VerifyTargetPostFXBaselines(v.beforePostFX, after)
	result.EvidenceRefs = append(result.EvidenceRefs, verification.EvidenceRefs...)
	result.SpecialistSummary = fmt.Sprintf("B4 exact target/peer same-tap post-FX comparison: comparable=%v, changed_dimensions=%s, reasons=%s", verification.Comparable, strings.Join(verification.ChangedDimensions, ","), strings.Join(verification.Reasons, ","))
	if verification.Status == "observed_change" && verification.Comparable {
		result.Status, result.Acoustic, result.SpecialistRelationship = "pass", "observed", "observed_change"
	} else {
		result.Status, result.Acoustic, result.SpecialistRelationship = "inconclusive", "inconclusive", "unchanged_or_unavailable"
	}
	result.Summary = "All B4 EQ leaves passed structural readback. " + result.SpecialistSummary + ". User listening acceptance remains unknown."
	return result, nil
}

func (s *Server) authorizeB4Batch(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, current *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil || session.ActiveProposal == nil || current == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 Session 缺少 FrozenPlan；没有执行。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || current.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || current.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 或 Kernel CAS 已变化；旧 B4 Proposal 未获授权。")
	}
	if !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "v1 Session Store 非持久化；B4 Proposal 未获授权。")
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此 B4 Proposal revision 的明确授权。")
	}
	_, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision, ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash, Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 授权绑定失败："+err.Error())
	}
	var treatment lowendrelation.TreatmentPlan
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["treatment_plan"], &treatment)
	var before lowendrelation.Model
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["diagnosis_model"], &before)
	var beforePostFX lowendrelation.TargetPostFXBaseline
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["verification_context"], &beforePostFX)
	if before.ModelID == "" && frozen.ActionSet.Actions[0].Command == b4EQBatchCommand {
		before = lowendrelation.Model{ModelID: treatment.DiagnosisID, ObservationID: treatment.ObservationID}
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, session.ID, frozen.ActionSet, frozen.ProjectCut, &b4BatchMutationPort{server: s}, b4BatchVerifier{server: s, treatment: treatment, before: before, beforePostFX: beforePostFX, sessionID: session.ID, goalText: session.Goal}, executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID})
	response := b4ExecutionResponse(conversationID, goal, executed, executeErr)
	attachMixboardDecisionProjection(&response, executed)
	if executeErr == nil && len(frozen.ActionSet.Actions) == 1 && frozen.ActionSet.Actions[0].Command == b4LoadBatchCommand && executed.Execution != nil && len(executed.Execution.Receipts) == 1 {
		preferred := map[string]string{}
		for _, row := range mapRowsValue(executed.Execution.Receipts[0].Details["loaded"]) {
			preferred[firstStringFromMap(row, "track_id")] = firstStringFromMap(row, "plugin_id")
		}
		resolution := s.resolveB4EQTargets(ctx, treatment, req.Context, preferred)
		if len(resolution.Missing) == 0 && len(resolution.Ambiguous) == 0 && len(resolution.Resolved) == len(treatment.Targets) {
			cfg, _, cfgErr := config.Load()
			if cfgErr == nil && cfg.Complete() {
				newSessionID := s.nextCapabilitySessionID(conversationID, lowEndRelationCapabilityID)
				newSession, startErr := s.orchestrationRuntime.StartB4ChatSession(newSessionID, conversationID, frozen.ProjectCut.ProjectUUID, session.Goal, orchestration.InteractionPropose)
				if startErr == nil {
					next := s.createB4EQProposal(ctx, conversationID, goal, newSession.ID, treatment, before, resolution.Resolved, cfg)
					next.Reply = "插件加载与实际实例资格确认已通过。\n\n" + next.Reply
					return next
				}
			}
		}
	}
	return response
}

func decodeAnyJSON(value any, target any) error {
	data, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(data, target)
	}
	return err
}

func b4ExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, executeErr error) ChatResponse {
	data := map[string]any{"session_id": session.ID, "capability_id": lowEndRelationCapabilityID, "user_acceptance": "unknown"}
	status, reply := string(agentruntime.StatusFailed), "B4 执行在写入前被安全门拒绝。"
	if executeErr != nil {
		data["error"] = executeErr.Error()
	}
	if session.Execution != nil {
		data["execution_id"], data["execution_status"], data["receipts"], data["verification"] = session.Execution.ID, session.Execution.Status, session.Execution.Receipts, session.Execution.VerificationResult
		if strings.EqualFold(session.Execution.Status, "verified") {
			data["canary_stage"] = "executed_verified"
		}
		if len(session.Execution.Receipts) > 0 {
			receipt := session.Execution.Receipts[0]
			data["result"] = receipt.Details
			if strings.EqualFold(receipt.Status, "applied") {
				status = string(agentruntime.StatusCompleted)
				reply = "B4 项目级批执行完成；精确实例、requested/actual readback、rollback 能力和验证报告已保存在结果中。"
			} else {
				reply = "B4 项目级批失败；已执行全工程回滚策略。"
			}
			if session.Status == orchestration.StatusNeedsReview {
				status = string(agentruntime.StatusWaitingContinue)
			}
		}
	}
	response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: reply, Workflow: "capability_runtime_v1", WorkflowData: data, GoalStatus: status}
	if executeErr != nil {
		response.Error = executeErr.Error()
	}
	return response
}
