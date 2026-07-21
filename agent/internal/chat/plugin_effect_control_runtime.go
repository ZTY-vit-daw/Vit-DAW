package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/tools"
)

// runPluginEffectControlRuntime keeps the B4 collector independently owned
// while allowing invocation-boundary tests to prove that raw apply calls do
// not fall through to the Harness mutation path.
func (s *Server) runPluginEffectControlRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s != nil && s.pluginEffectControlRuntimeOverride != nil {
		return s.pluginEffectControlRuntimeOverride(ctx, conversationID, req, goal)
	}
	return s.handlePluginEffectControlRuntime(ctx, conversationID, req, goal)
}

// handlePluginEffectControlRuntime is the deliberately independent B4
// collector/owner.  It does not share or generalise the proven B2/B3 CCBs.
func (s *Server) handlePluginEffectControlRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制治理路径缺少 Runtime、Kernel 或 Harness；没有修改工程。")
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, pluginEffectControlCapabilityID)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "确认交互绑定的插件控制 Proposal 已过期；没有授权或修改工程。")
	}
	if exists && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
		recovered := s.recoverCapabilityExecution(ctx, conversationID, goal, session)
		return recovered
	}

	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法取得当前工程强快照；插件控制 Proposal 未创建。")
	}
	decision, hasDecision := capabilityApprovalDecisionFromContext(req.Context)
	if exists && session.ActiveProposal != nil && hasDecision && decision.Kind == orchestration.ApprovalApprove {
		return s.authorizePluginEffectControl(ctx, conversationID, req, goal, session, state)
	}
	if exists && session.Status == orchestration.StatusAuthorized {
		return authorizedCapabilityWaitingResponse(conversationID, goal, session)
	}
	if exists && session.ActiveProposal != nil {
		// A pending exact proposal is handled by the common interaction seam.
		return pluginEffectControlProposalResponse(conversationID, goal, session)
	}

	applyArgs := s.pluginEffectApplyArgsWithVerifiedVPS(req.Context)
	if len(applyArgs) == 0 {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制请求缺少冻结的 plugin_grabber.apply_control 参数；没有修改工程。")
	}
	trackID := firstStringFromMap(applyArgs, "track_id")
	pluginID := firstStringFromMap(applyArgs, "plugin_id")
	control := firstStringFromMap(applyArgs, "control", "operation", "name")
	if trackID == "" || pluginID == "" || control == "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "plugin effect control 需要 track_id、plugin_id 和 control；没有修改工程。")
	}

	before, err := s.collectPluginEffectBeforeEvidence(ctx, sessionID, req, trackID, pluginID, applyArgs)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 CCB 前证据获取失败："+err.Error())
	}
	previewArgs := cloneContext(applyArgs)
	previewArgs["resolve_only"] = true
	previewArgs["preview"] = true
	resolved, err := s.kernel.SendVSPCommandWithIDs(ctx, "plugin.apply_control", previewArgs, "ccb:"+sessionID+":resolve", "ccb:"+sessionID)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制 resolve-only 失败："+err.Error())
	}
	preview := pluginEffectReply(resolved)
	if failure := pluginEffectFailure(preview); failure != "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制 resolve-only 被拒绝："+failure)
	}
	parameters, err := pluginEffectResolvedParameters(preview["resolved_parameters"])
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制预映像无效："+err.Error())
	}
	expectedIDs := make([]string, 0, len(parameters))
	targetFingerprints := []string{"plugin:" + trackID + ":" + pluginID + ":control:" + control}
	for _, parameter := range parameters {
		expectedIDs = append(expectedIDs, parameter.ParameterID)
		targetFingerprints = append(targetFingerprints, fmt.Sprintf("plugin-param:%s:%0.9f", parameter.ParameterID, parameter.OldNormalizedValue))
	}
	sort.Strings(targetFingerprints)
	profileSignature := firstStringFromMap(preview, "profile_param_signature_hash", "current_param_signature_hash")
	dependencies := []string{"observation:" + before.ObservationID, "profile-source:" + firstStringFromMap(preview, "profile_source")}
	if profileSignature != "" {
		dependencies = append(dependencies, "profile-signature:"+profileSignature)
	}
	buildCut := projectcut.BuildRequest{
		State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel),
		DependencyFingerprints: dependencies, TargetFingerprints: targetFingerprints,
		ArtifactRefs:     pluginEffectEvidenceRefs(before.Observation),
		ContractVersions: []string{"capability:plugin.effect_control.v0", "resolver:plugin.apply_control.resolve_only.v1"},
	}
	cut, err := projectcut.Build(buildCut)
	if err != nil || !cut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立可执行 Project Cut；插件控制 Proposal 未创建。")
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartPluginEffectControlChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, orchestration.InteractionPropose)
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建插件控制 PlanningSession："+err.Error())
		}
	}
	revision := int64(1)
	if session.ActiveProposal != nil {
		revision = session.ActiveProposal.Revision + 1
	}
	delete(applyArgs, "resolve_only")
	delete(applyArgs, "preview")
	proposal, actionSet, err := capabilityadapters.FreezePluginEffectControl(capabilityadapters.PluginEffectControlPlan{
		TrackID: trackID, PluginID: pluginID, Control: control, ApplyArgs: applyArgs,
		ResolvedParameters: parameters, ExpectedParameterIDs: expectedIDs,
		ProfileSource: firstStringFromMap(preview, "profile_source"), ProfileSignature: profileSignature,
		Resolution: firstMapFromAny(preview["resolution"]), PreviousObservation: before.ObservationID,
		BeforeEvidence: map[string]any{"observation": before.Observation, "tom_projection": before.TOMProjection},
	}, cut, revision)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结插件控制 Proposal："+err.Error())
	}
	proposal.Presentation = pluginEffectControlPresentation(proposal, actionSet, trackID, pluginID, control, parameters, before.ObservationID)
	frozen := orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut,
		ContextBundleID:       "bundle_plugin_effect_" + sanitizeCanaryID(sessionID),
		PreviousObservationID: before.ObservationID, FrozenAt: time.Now().UTC(),
	}
	session, err = s.orchestrationRuntime.AttachFrozenPlan(sessionID, frozen)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法持久化插件控制 Proposal："+err.Error())
	}
	return pluginEffectControlProposalResponse(conversationID, goal, session)
}

type pluginEffectBeforeEvidence struct {
	ObservationID string
	Observation   map[string]any
	TOMProjection map[string]any
}

func (s *Server) collectPluginEffectBeforeEvidence(ctx context.Context, sessionID string, req ChatRequest, trackID, pluginID string, applyArgs map[string]any) (pluginEffectBeforeEvidence, error) {
	observed, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.observe", Args: map[string]any{
			"scope": "full_project", "project_context": true, "observation_only": true,
			"disclosure": "digest_catalog", "mom_intent": "project_multitrack_relation_observation",
			"mix_session_id": sessionID, "goal_text": req.Message,
			"target_ref": map[string]any{"track_id": trackID, "plugin_id": pluginID},
		},
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true, "plugin_effect_control_ccb": true},
		Source:  "capability_runtime_v1_plugin_effect_ccb", Confirmed: true,
		ToolCallID: "ccb:" + sessionID + ":mix.observe",
	})
	if err != nil || !strings.EqualFold(observed.Status, "ok") {
		return pluginEffectBeforeEvidence{}, fmt.Errorf("mix.observe: %s", firstNonEmpty(observed.Error, canaryErrorText(err), observed.Status))
	}
	observationID := firstStringFromMap(observed.Result, "observation_id")
	if observationID == "" {
		return pluginEffectBeforeEvidence{}, fmt.Errorf("mix.observe omitted observation_id")
	}
	tom := firstMapFromAny(req.Context["tom_projection"])
	if len(tom) == 0 {
		tom = firstMapFromAny(req.Context["track_organization_model"])
	}
	if len(tom) == 0 {
		tom = map[string]any{"status": "snapshot_only", "track_id": trackID, "plugin_id": pluginID, "apply_target": cloneContext(applyArgs)}
	}
	return pluginEffectBeforeEvidence{ObservationID: observationID, Observation: cloneContext(observed.Result), TOMProjection: tom}, nil
}

func (s *Server) authorizePluginEffectControl(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil || session.ActiveProposal == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制 Session 缺少 frozen plan；没有执行。")
	}
	frozen := *session.FrozenPlan
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "Kernel 未提供强 CAS 保证；插件控制未授权。")
	}
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if state == nil || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 已变化；旧插件控制 Proposal 未授权。")
	}
	if !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "v1 Session Store 非持久化；插件控制未授权。")
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此插件控制 Proposal revision 的明确授权。")
	}
	authorized, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
		ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision,
		ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash,
		Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID,
		Sequence: session.Revision + 1, Decision: &decision,
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制授权绑定失败："+err.Error())
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
		&executionports.PluginGrabberPort{Client: s.kernel},
		executionverifiers.PluginEffectControl{Invoker: s.harness, PreviousObservationID: frozen.PreviousObservationID, MixSessionID: session.ID, GoalText: session.Goal},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	response := capabilityCanaryExecutionResponse(conversationID, goal, executed, orchestration.ContextEnvelope{}, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	if executed.Status == orchestration.StatusCompleted {
		response.Reply = "插件控制已通过 Proposal → Authorization → 冻结预映像执行 → fresh 参数回读 → 定量与方向证据验证。用户听感验收仍为 unknown。"
	}
	return response
}

func pluginGrabberApplyInvokeRequest(req harness.InvokeRequest) bool {
	return isPluginGrabberApplyToolCall(planner.ToolCall{
		Tool: strings.TrimSpace(req.Tool), Args: cloneStringAnyMap(req.Args), Command: cloneStringAnyMap(req.Command),
	})
}

func pluginGrabberApplyInvokeArgs(req harness.InvokeRequest) map[string]any {
	if len(req.Args) > 0 {
		return cloneStringAnyMap(req.Args)
	}
	command := workflowCommandArgs(req.Command)
	if nested := mapValue(command["args"]); len(nested) > 0 {
		return cloneStringAnyMap(nested)
	}
	delete(command, "cmd")
	delete(command, "command")
	delete(command, "action")
	delete(command, "tool")
	return command
}

func pluginEffectControlInvocationContext(base, applyArgs map[string]any, sourceCallID string) map[string]any {
	requestContext := cloneStringAnyMap(base)
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	requestContext["capability_runtime_v1"] = true
	requestContext["capability_id"] = pluginEffectControlCapabilityID
	requestContext["plugin_effect_apply_args"] = cloneStringAnyMap(applyArgs)
	requestContext["source_tool_call_id"] = strings.TrimSpace(sourceCallID)
	return requestContext
}

func pluginEffectControlResult(response ChatResponse, status string) map[string]any {
	result := cloneStringAnyMap(response.WorkflowData)
	if result == nil {
		result = map[string]any{}
	}
	result["status"] = status
	result["requires_confirmation"] = response.NeedsConfirmation
	result["reply"] = response.Reply
	result["proposal_id"] = response.PlanID
	result["preview"] = response.Preview
	if response.ProposalPresentation != nil {
		result["proposal_presentation"] = response.ProposalPresentation
	}
	return result
}

// invokePluginEffectControlHTTP converts direct /agent/invoke raw grabber
// requests into the same B4 Proposal seam used by AgentLoop. req.Confirmed is
// intentionally not forwarded: raw tool confirmation is not v1 Authorization.
func (s *Server) invokePluginEffectControlHTTP(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, error) {
	callID := strings.TrimSpace(req.ToolCallID)
	if callID == "" {
		callID = "http_plugin_effect_" + randomID()
	}
	conversationID := firstNonEmpty(firstStringFromMap(req.Context, "conversation_id", "chat_conversation_id"), "http_plugin_effect_"+sanitizeCanaryID(req.GoalID))
	message := firstNonEmpty(firstStringFromMap(req.Context, "user_message", "goal_summary"), "执行已请求的插件语义控制")
	requestContext := pluginEffectControlInvocationContext(req.Context, pluginGrabberApplyInvokeArgs(req), callID)
	if req.Confirmed {
		requestContext["raw_tool_confirmation_ignored"] = true
	}
	goal := agentruntime.Goal{GoalID: strings.TrimSpace(req.GoalID), RunID: strings.TrimSpace(req.RunID), Summary: message, Status: agentruntime.StatusRunning}
	response := s.runPluginEffectControlRuntime(ctx, conversationID, ChatRequest{ConversationID: conversationID, Message: message, Context: requestContext}, goal)
	status := "needs_confirmation"
	if response.Error != "" {
		status = "error"
	}
	result := pluginEffectControlResult(response, status)
	if req.Confirmed {
		result["raw_tool_confirmation_ignored"] = true
	}
	out := harness.InvokeResponse{
		Status: status, Tool: "plugin_grabber.apply_control", CommandName: pluginEffectControlCapabilityID,
		RiskLevel: tools.RiskUndoable, RequiresConfirmation: response.NeedsConfirmation,
		Preview: response.Preview, UndoLabel: "Apply governed plugin control", Result: result,
		ProjectHistory: response.ProjectHistory, Error: response.Error,
	}
	if response.Error != "" {
		return out, fmt.Errorf("%s", response.Error)
	}
	return out, nil
}

func (s *Server) pluginEffectApplyArgsWithVerifiedVPS(ctx map[string]any) map[string]any {
	applyArgs := pluginEffectApplyArgs(ctx)
	if len(applyArgs) == 0 || s == nil || s.pluginVPS == nil {
		return applyArgs
	}
	if profiles := s.pluginVPS.RuntimeProfiles(); len(profiles) > 0 {
		applyArgs["verified_vps_profiles"] = profiles
	}
	return applyArgs
}

func pluginEffectApplyArgs(ctx map[string]any) map[string]any {
	for _, key := range []string{"plugin_effect_apply_args", "plugin_grabber_apply_args", "tool_args", "args"} {
		if value := firstMapFromAny(ctx[key]); len(value) > 0 {
			if nested := firstMapFromAny(value["args"]); len(nested) > 0 && firstStringFromMap(value, "tool") == "plugin_grabber.apply_control" {
				return cloneContext(nested)
			}
			return cloneContext(value)
		}
	}
	return nil
}

func pluginEffectReply(result *kernel.VSPCommandResult) map[string]any {
	if result == nil {
		return nil
	}
	return result.LegacyLikeReply()
}

func pluginEffectFailure(reply map[string]any) string {
	if len(reply) == 0 {
		return "empty kernel reply"
	}
	status := strings.ToLower(firstStringFromMap(reply, "status"))
	if status == "" || status == "ok" || status == "success" {
		return ""
	}
	return firstNonEmpty(firstStringFromMap(reply, "error", "message", "reason"), status)
}

func pluginEffectResolvedParameters(value any) ([]capabilityadapters.PluginResolvedParameter, error) {
	rows := mapRowsFromAny(value)
	if len(rows) == 0 {
		return nil, fmt.Errorf("resolve-only response omitted resolved_parameters")
	}
	out := make([]capabilityadapters.PluginResolvedParameter, 0, len(rows))
	for _, row := range rows {
		id := firstStringFromMap(row, "parameter_id", "param_id", "id")
		normalized, ok := pluginEffectNumber(row, "old_normalized_value", "old_normalised_value", "normalized_value", "normalised_value")
		if id == "" || !ok {
			return nil, fmt.Errorf("resolved parameter omitted identity or old normalized value")
		}
		old, _ := pluginEffectNumber(row, "old_value")
		out = append(out, capabilityadapters.PluginResolvedParameter{
			Slot: firstStringFromMap(row, "slot"), ParameterID: id,
			ParameterName: firstStringFromMap(row, "parameter_name", "param_name", "name"),
			OldValue:      old, OldNormalizedValue: normalized, OldValueText: firstStringFromMap(row, "old_value_text", "value_text"),
		})
	}
	return out, nil
}

func pluginEffectNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, exists := row[key]
		if !exists {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case json.Number:
			v, err := typed.Float64()
			return v, err == nil
		case string:
			v, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			return v, err == nil
		}
	}
	return 0, false
}

func pluginEffectEvidenceRefs(observation map[string]any) []string {
	projection := firstMapFromAny(observation["mom_projection"])
	relation := firstMapFromAny(projection["multitrack_relation"])
	refs := canaryStringSlice(relation["evidence_refs"])
	if id := firstStringFromMap(observation, "observation_id"); id != "" {
		refs = append(refs, "mix.observe:"+id)
	}
	sort.Strings(refs)
	return refs
}

func pluginEffectControlPresentation(proposal orchestration.Proposal, actionSet orchestration.ActionSet, trackID, pluginID, control string, parameters []capabilityadapters.PluginResolvedParameter, observationID string) *orchestration.ProposalPresentation {
	return &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision,
		CapabilityID: proposal.CapabilityID, Title: "B4 插件语义控制方案",
		Conclusion:      fmt.Sprintf("拟在轨道 %s 的插件 %s 执行 %s；%d 个参数身份和完整旧值已冻结。", trackID, pluginID, control, len(parameters)),
		AnalysisSummary: []string{"已取得 fresh mix.observe 前证据", "C++ resolve-only 已用运行时 profile 解析但未写入", "参数身份不一致或回读失败将恢复完整预映像"},
		Recommendation:  proposal.Summary, ActionCount: len(actionSet.Actions), Risk: proposal.Risk, Reversible: true,
		Readiness:    []orchestration.ProposalMetric{{ID: "profile_resolution", Label: "运行时映射", Value: "verified", Status: "ready"}, {ID: "frozen_preimage", Label: "冻结预映像", Value: fmt.Sprintf("%d parameters", len(parameters)), Status: "ready"}},
		EvidenceRefs: []string{"mix.observe:" + observationID}, ApprovalPrompt: "是否授权执行这个已冻结的插件控制方案？",
	}
}

func pluginEffectControlProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "插件控制 Session 缺少 Proposal。")
	}
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: session.ActiveProposal.ID,
		Preview: firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID, "proposal_id": session.ActiveProposal.ID,
			"proposal_revision": session.ActiveProposal.Revision, "action_set_hash": session.ActiveProposal.ActionSetHash,
			"project_cut_hash": session.ActiveProposal.ProjectCutHash, "canary_stage": "proposal", "approval_mode": "conversational",
			"proposal_presentation": presentation,
		},
	}
}
