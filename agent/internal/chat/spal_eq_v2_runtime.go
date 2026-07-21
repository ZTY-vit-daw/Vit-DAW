package chat

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spal"
)

var (
	eqV2BandPattern       = regexp.MustCompile(`(?i)(?:band|频段)\s*(1|2|3|4|i{1,3}|iv)\b`)
	eqV2FrequencyPattern  = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*(k?hz)\b`)
	eqV2GainPattern       = regexp.MustCompile(`(?i)([+-]?[0-9]+(?:\.[0-9]+)?)\s*db\b`)
	eqV2QPattern          = regexp.MustCompile(`(?i)\bq\s*[:=]?\s*([0-9]+(?:\.[0-9]+)?)`)
	eqV2SlopePattern      = regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)\s*db\s*/?\s*(?:oct|octave|倍频程)`)
	eqV2ThresholdPattern  = regexp.MustCompile(`(?i)(?:threshold|阈值)\s*[:=]?\s*([+-]?[0-9]+(?:\.[0-9]+)?)\s*db?`)
	eqV2RatioPattern      = regexp.MustCompile(`(?i)(?:ratio|压缩比)\s*[:=]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?::\s*1)?`)
	eqV2AttackPattern     = regexp.MustCompile(`(?i)(?:attack|启动|起音)\s*[:=]?\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
	eqV2ReleasePattern    = regexp.MustCompile(`(?i)(?:release|释放)\s*[:=]?\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
	eqV2DryMixPattern     = regexp.MustCompile(`(?i)(?:dry\s*mix|干湿比|干声混合)\s*[:=]?\s*([0-9]+(?:\.[0-9]+)?)\s*%?`)
	eqV2OutputGainPattern = regexp.MustCompile(`(?i)(?:out(?:put)?\s*gain|输出增益)\s*[:=]?\s*([+-]?[0-9]+(?:\.[0-9]+)?)\s*db`)
)

type spalEQV2Request struct {
	TargetRef            string
	PluginID             string
	ProviderCredentialID string
	Instruction          spal.Instruction
	Missing              []string
}

const vpsForgeControlledAcceptanceMode = "vpsforge.controlled_acceptance.v1"

// vpsForgeControlledAcceptanceRequested is intentionally narrow: this is an
// opt-in Forge acceptance run, not an alternate production execution mode.
// Its mutation port always restores the frozen preimage before returning.
func vpsForgeControlledAcceptanceRequested(req ChatRequest) bool {
	return strings.EqualFold(strings.TrimSpace(firstNonEmpty(
		cleanContextText(req.Context["vpsforge_execution_mode"]),
		cleanContextText(mapValue(req.Context["vpsforge"])["execution_mode"]),
	)), vpsForgeControlledAcceptanceMode)
}

func isSPALEQV2ControlIntent(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	pluginOrEQ := strings.Contains(lower, "tdr nova") || strings.Contains(lower, "nova") || strings.Contains(lower, " eq") || strings.HasPrefix(lower, "eq") || strings.Contains(message, "均衡")
	if !pluginOrEQ {
		return false
	}
	return eqV2BandPattern.MatchString(message) || containsAny(lower, "highpass", "high-pass", "lowpass", "low-pass", "output gain", "dry mix", "bypass", "dynamic") || containsAny(message, "高通", "低通", "输出增益", "干湿比", "旁通", "动态")
}

func parseSPALEQV2Request(req ChatRequest) spalEQV2Request {
	values := mapValue(req.Context["spal_eq_v2"])
	target := firstNonEmpty(spalReferenceEQContextText(values, "target_ref"), spalReferenceEQContextText(req.Context, "spal_target_ref"), spalReferenceEQContextText(req.Context, "selected_track_id"), spalReferenceEQContextText(req.Context, "primary_selected_track_id"))
	pluginID := firstNonEmpty(spalReferenceEQContextText(values, "plugin_id"), spalReferenceEQContextText(req.Context, "selected_plugin_id"), spalReferenceEQContextText(req.Context, "primary_selected_plugin_id"))
	credentialID := firstNonEmpty(spalReferenceEQContextText(values, "provider_credential_id"), spalReferenceEQContextText(req.Context, "spal_provider_credential_id"))
	if target == "" {
		if match := spalReferenceEQTrackRefPattern.FindString(req.Message); match != "" {
			target = match
		}
	}
	result := spalEQV2Request{TargetRef: target, PluginID: pluginID, ProviderCredentialID: credentialID}
	lower := strings.ToLower(req.Message)

	schema := ""
	switch {
	case containsAny(lower, "highpass", "high-pass", "lowpass", "low-pass") || containsAny(req.Message, "高通", "低通"):
		schema = spal.EQPassFilterPatchControlID
	case strings.Contains(lower, "dynamic") || strings.Contains(req.Message, "动态"):
		schema = spal.EQDynamicBandPatchControlID
	case containsAny(lower, "output gain", "out gain", "dry mix", "bypass") || containsAny(req.Message, "输出增益", "干湿比", "干声混合", "旁通"):
		schema = spal.EQOutputPatchControlID
	case eqV2BandPattern.MatchString(req.Message):
		schema = spal.EQBandPatchControlID
	}
	if schema == "" {
		result.Missing = append(result.Missing, "明确的 EQ 动作（Band、HP/LP、动态 EQ 或输出控制）")
		return result
	}
	instruction := spal.Instruction{SchemaID: schema, TargetRef: target, Parameters: map[string]float64{}, StringParameters: map[string]string{}}

	switch schema {
	case spal.EQBandPatchControlID:
		band := parseEQV2BandRef(req.Message)
		if band == "" {
			result.Missing = append(result.Missing, "Band 编号")
		} else {
			instruction.StringParameters["band_ref"] = band
		}
		shape := parseEQV2Shape(req.Message)
		if shape == "" {
			result.Missing = append(result.Missing, "响应形状（Bell/Low Shelf/High Shelf）")
		} else {
			instruction.StringParameters["response_shape"] = shape
		}
		if value, ok := parseEQV2Frequency(req.Message); ok {
			instruction.Parameters["frequency_hz"] = value
		} else {
			result.Missing = append(result.Missing, "频率")
		}
		if value, ok := regexFloat(eqV2GainPattern, req.Message); ok {
			instruction.Parameters["gain_db"] = value
		} else {
			result.Missing = append(result.Missing, "增益")
		}
		if value, ok := regexFloat(eqV2QPattern, req.Message); ok {
			instruction.Parameters["q"] = value
		} else {
			result.Missing = append(result.Missing, "Q")
		}
		instruction.Parameters["enabled"] = parseEQV2Enabled(req.Message, 1)
		maxGain := 18.0
		instruction.SafetyBounds.MaxAbsoluteGainDB = &maxGain
	case spal.EQPassFilterPatchControlID:
		kind := "highpass"
		if containsAny(lower, "lowpass", "low-pass") || strings.Contains(req.Message, "低通") {
			kind = "lowpass"
		}
		instruction.StringParameters["filter_kind"] = kind
		instruction.Parameters["enabled"] = parseEQV2Enabled(req.Message, 1)
		if value, ok := parseEQV2Frequency(req.Message); ok {
			instruction.Parameters["cutoff_frequency_hz"] = value
		} else {
			result.Missing = append(result.Missing, "截止频率")
		}
		if value, ok := regexFloat(eqV2SlopePattern, req.Message); ok {
			instruction.Parameters["slope_db_per_octave"] = value
		} else {
			result.Missing = append(result.Missing, "斜率（dB/oct）")
		}
	case spal.EQDynamicBandPatchControlID:
		band := parseEQV2BandRef(req.Message)
		if band == "" {
			result.Missing = append(result.Missing, "Band 编号")
		} else {
			instruction.StringParameters["band_ref"] = band
		}
		mode := "normal"
		if isEQV2Off(req.Message) {
			mode = "off"
		}
		instruction.StringParameters["dynamics_mode"] = mode
		instruction.StringParameters["routing_scope"] = "independent"
		if value, ok := regexFloat(eqV2ThresholdPattern, req.Message); ok {
			instruction.Parameters["threshold_db"] = value
		} else {
			result.Missing = append(result.Missing, "Threshold")
		}
		for key, pattern := range map[string]*regexp.Regexp{"ratio": eqV2RatioPattern, "attack_ms": eqV2AttackPattern, "release_ms": eqV2ReleasePattern} {
			if value, ok := regexFloat(pattern, req.Message); ok {
				instruction.Parameters[key] = value
			}
		}
	case spal.EQOutputPatchControlID:
		if strings.Contains(lower, "bypass") || strings.Contains(req.Message, "旁通") {
			instruction.Parameters["bypass"] = parseEQV2Enabled(req.Message, 1)
		}
		if value, ok := regexFloat(eqV2DryMixPattern, req.Message); ok {
			instruction.Parameters["dry_mix_percent"] = value
		}
		if value, ok := regexFloat(eqV2OutputGainPattern, req.Message); ok {
			instruction.Parameters["output_gain_db"] = value
		}
		if len(instruction.Parameters) == 0 {
			result.Missing = append(result.Missing, "至少一个输出参数")
		}
	}
	if target == "" {
		result.Missing = append(result.Missing, "目标轨道（可直接选中轨道）")
	}
	if len(result.Missing) == 0 {
		if err := instruction.Validate(); err != nil {
			result.Missing = append(result.Missing, err.Error())
		}
	}
	result.Instruction = instruction
	return result
}

func (s *Server) handleSPALEQV2Runtime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	return s.handleSPALEQV2RuntimeRequest(ctx, conversationID, req, goal, nil)
}

// handleSPALEQV2StructuredRuntime is the Agent-facing capability seam. The
// Agent has already translated natural language into a task-level action and
// the capability tool has validated its finite semantic fields; SPAL never
// reparses the user's wording on this path.
func (s *Server) handleSPALEQV2StructuredRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, parsed spalEQV2Request) ChatResponse {
	return s.handleSPALEQV2RuntimeRequest(ctx, conversationID, req, goal, &parsed)
}

func (s *Server) handleSPALEQV2RuntimeRequest(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, structured *spalEQV2Request) ChatResponse {
	if s == nil || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "EQ 运行时需要可用的命令执行器。")
	}
	if spalEQV2RollbackRequested(req) {
		sessionID := firstStringFromMap(req.Context, "capability_session_id")
		if sessionID == "" {
			sessionID = s.nextCapabilitySessionID(conversationID, spalEQV2CapabilityID)
		}
		return s.planSPALEQV2Rollback(ctx, conversationID, req, goal, sessionID)
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	var session orchestration.PlanningSession
	exists := false
	if s.orchestrationRuntime != nil && s.kernel != nil {
		if sessionID == "" {
			sessionID = s.nextCapabilitySessionID(conversationID, spalEQV2CapabilityID)
		}
		session, exists = s.orchestrationRuntime.Store.Load(sessionID)
		if exists && !expectedProposalMatches(req.Context, session.ActiveProposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 Proposal 的确认绑定已过期；没有授权或修改工程。")
		}
		if exists && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
			return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
		}
		if exists && session.ActiveProposal != nil {
			if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok && decision.Kind == orchestration.ApprovalApprove {
				state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
				if err != nil || state == nil || !state.OK() {
					return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程状态；SPAL EQ v2 Proposal 未获授权。")
				}
				return s.handleSPALEQV2Authorization(ctx, conversationID, req, goal, session, state)
			}
			if session.Status == orchestration.StatusAuthorized {
				return authorizedCapabilityWaitingResponse(conversationID, goal, session)
			}
			return proposalAmbiguousResponse(conversationID, goal, session)
		}
	}

	// An approval for an existing frozen Proposal is intentionally handled
	// above. Only a request that has no pending Proposal may be interpreted as
	// new EQ semantics; otherwise a generic confirmation such as "可以执行"
	// would incorrectly be treated as a fresh incomplete command.
	parsed := parseSPALEQV2Request(req)
	if structured != nil {
		parsed = *structured
	}
	if len(parsed.Missing) > 0 {
		return spalEQV2ClarificationResponse(conversationID, goal, parsed.Missing)
	}
	// An explicitly configured VPS Forge staging bridge is checked before the
	// verified SPAL Provider lookup. The bridge can never create a Credential,
	// Catalog entry, or SPAL route; it is only a narrow local transport test.
	if response, handled := s.handleVPSForgeStagingEQ(ctx, conversationID, req, goal, parsed); handled {
		return response
	}
	if s.orchestrationRuntime == nil || s.kernel == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 需要可用的 Project-aware Runtime、Kernel 与命令执行器。")
	}
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, spalEQV2CapabilityID)
	}

	providerState, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || providerState == nil || !providerState.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程 snapshot；SPAL EQ v2 未启动。")
	}
	projectUUID := spalReferenceEQProjectUUID(providerState)
	if projectUUID == "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Project Cut 缺少 project_uuid；SPAL EQ v2 未启动。")
	}
	resolved, candidates, err := s.resolveVPSEQV2Provider(ctx, providerState, vpsEQV2ResolveRequest{TargetRef: parsed.TargetRef, PluginID: parsed.PluginID, ProviderCredentialID: parsed.ProviderCredentialID, Instruction: parsed.Instruction})
	if err != nil {
		return vpsEQV2ProviderResponse(conversationID, goal, parsed.TargetRef, err, candidates)
	}
	parsed.Instruction.TargetRef = resolved.Instance.TargetRef
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() || spalReferenceEQProjectUUID(state) != projectUUID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "Provider 解析期间工程发生变化；请重新发起 SPAL EQ v2 请求。")
	}
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Kernel 未提供 command.base_revision_cas；SPAL EQ v2 不会生成可执行 Proposal。")
	}
	cut, err := s.buildVPSEQV2Cut(state, resolved, req)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 SPAL EQ v2 Project Cut："+err.Error())
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartSPALEQV2ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context))
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 SPAL EQ v2 Session："+err.Error())
		}
	}
	parsed.Instruction.EvidenceRefs = appendUniqueSPALRefs([]string{"vps.provider_credential:" + resolved.Credential.ID}, resolved.EvidenceRefs...)
	planned, err := capabilityadapters.PlanSPAL(capabilityadapters.SPALPlanRequest{Context: ctx, SessionID: sessionID, Goal: req.Message, Mode: canaryInteractionMode(req.Context), CapabilityID: spalEQV2CapabilityID, CapabilityVersion: "v2", ProjectCut: cut, Instruction: parsed.Instruction, Registry: resolved.Registry, Instances: []spal.ProviderInstance{resolved.Instance}, PreimageReader: executionports.SPALVSPInspector{Client: s.kernel}})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 只读准备失败："+err.Error())
	}
	planned.Bundle.ArtifactRefs = appendUniqueSPALRefs(planned.Bundle.ArtifactRefs, "vps.provider_credential:"+resolved.Credential.ID, "vps.document:"+resolved.Document.ID)
	planned.Bundle.EvidenceRefs = appendUniqueSPALRefs(planned.Bundle.EvidenceRefs, resolved.EvidenceRefs...)
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(sessionID, planned.Bundle, capabilityCanaryToolSchemas(s), []orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}}, orchestration.DefaultContextWindowBudget())
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 Context Envelope 未通过："+err.Error())
	}
	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return spalEQV2AnalysisResponse(conversationID, goal, sessionID, parsed, planned, envelope, resolved)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || planned.Preparation.Manifest == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 未形成可执行 Proposal："+planned.Outcome.Summary)
	}
	proposal, actionSet, err := capabilityadapters.FreezeSPALProposal(planned, spalEQV2CapabilityID, "v2", cut, 1)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 SPAL EQ v2 Proposal："+err.Error())
	}
	proposal.Presentation = spalEQV2ProposalPresentation(proposal, parsed.Instruction, spal.OperationApply)
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: planned.Bundle.ID})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 SPAL EQ v2 Proposal："+err.Error())
	}
	return spalEQV2ProposalResponse(conversationID, goal, updated, planned, envelope, resolved)
}

func (s *Server) handleSPALEQV2Authorization(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 Proposal 缺少冻结 ActionSet。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || state == nil || !state.OK() || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 已变化；请重新生成 SPAL EQ v2 Proposal。")
	}
	manifest, err := spal.ManifestFromAction(frozen.ActionSet.Actions[0])
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "冻结的 SPAL EQ v2 Manifest 无效："+err.Error())
	}
	if _, err := s.resolveVPSEQV2ProviderForFrozen(ctx, manifest.Binding.Instance, frozen.ProjectCut.ProjectUUID, manifest.Instruction); err != nil {
		return vpsEQV2ProviderResponse(conversationID, goal, manifest.Instruction.TargetRef, err, nil)
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定该 Proposal revision 的明确授权。")
	}
	authorized, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision, ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash, Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法持久化 SPAL EQ v2 授权："+err.Error())
	}
	envelope, _ := s.orchestrationRuntime.BuildCapabilityContextEnvelope(session.ID, orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: []string{"spal.manifest:" + frozen.Proposal.CandidateID}}, capabilityCanaryToolSchemas(s), nil, orchestration.DefaultContextWindowBudget())
	controlledAcceptance := vpsForgeControlledAcceptanceRequested(req)
	persistence := executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID}
	var executed orchestration.PlanningSession
	var executeErr error
	if controlledAcceptance {
		executed, executeErr = s.orchestrationRuntime.ExecuteActionSetWithPersistence(
			ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
			&executionports.SPALVSPControlledRoundtripPort{Client: s.kernel},
			executionverifiers.SPAL{RequireControlledRoundtrip: true}, persistence,
		)
	} else {
		executed, executeErr = s.orchestrationRuntime.ExecuteActionSetWithPersistence(
			ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
			s.spalReferenceEQMutationPort(ctx), executionverifiers.SPAL{}, persistence,
		)
	}
	if executed.ID == "" {
		executed = authorized
	}
	response := capabilityCanaryExecutionResponse(conversationID, goal, executed, envelope, executeErr)
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["semantic_action"] = spalEQV2SemanticAction(frozen.ActionSet)
	response.WorkflowData["provider_source"] = "vps_v3_catalog"
	response.WorkflowData["rollback_available"] = !controlledAcceptance && spalEQV2RollbackEligible(executed)
	if controlledAcceptance {
		response.WorkflowData["execution_mode"] = vpsForgeControlledAcceptanceMode
		response.WorkflowData["controlled_roundtrip"] = true
		response.WorkflowData["project_parameter_state"] = "restored_to_frozen_preimage"
	}
	if executed.Status == orchestration.StatusCompleted || executed.Status == orchestration.StatusNeedsReview {
		operation := spal.OperationApply
		if executed.FrozenPlan != nil && len(executed.FrozenPlan.ActionSet.Actions) == 1 {
			if manifest, manifestErr := spal.ManifestFromAction(executed.FrozenPlan.ActionSet.Actions[0]); manifestErr == nil {
				operation = firstNonEmpty(manifest.Operation, spal.OperationApply)
			}
		}
		if controlledAcceptance {
			response.Reply = "Forge 受控验收已完成：已按冻结方案写入、完成 fresh readback，并已在同一轮恢复完整物理 preimage 后再次 readback。此验收不会留下插件参数改动；音乐审美结论仍为 unknown。"
		} else if operation == spal.OperationRollback {
			response.Reply = "SPAL EQ v2 已恢复冻结的完整物理 preimage，并完成 fresh readback。"
		} else {
			response.Reply = "SPAL EQ v2 已写入已认证 Provider 的 conformed 控件并完成 fresh readback。结构验证结果已记录；音乐性结论仍由用户判断。"
		}
	}
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	return response
}

func spalEQV2ProposalPresentation(proposal orchestration.Proposal, instruction spal.Instruction, operation string) *orchestration.ProposalPresentation {
	title := "SPAL EQ v2 参数方案"
	conclusion := describeSPALEQV2Instruction(instruction)
	recommendation := "确认后执行这一个参数事务。"
	approval := "确认执行此 SPAL EQ v2 方案，或取消。"
	analysis := []string{
		"Provider 来自 VPS v3 Catalog 中的 verified EQ v2 Credential。",
		"只会写入该 Credential 能力矩阵内已 conformed 的控件。",
		"执行前已冻结完整 preimage，写入后进行 fresh readback。",
	}
	if strings.EqualFold(operation, spal.OperationRollback) {
		title = "SPAL EQ v2 回滚方案"
		conclusion = "将恢复此前已执行 SPAL EQ v2 事务的冻结完整物理 preimage。"
		recommendation = "确认后先验证当前参数仍等于原事务的 postimage；只有匹配时才恢复 preimage。"
		approval = "确认执行这一个 SPAL EQ v2 回滚方案，或取消。"
		analysis = append(analysis, "回滚不会覆盖人工或并发修改；若当前 postimage 不再匹配，将拒绝写入。")
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: title, Conclusion: conclusion, AnalysisSummary: analysis, Recommendation: recommendation, AnalyzedTracks: 1, ActionCount: 1, Risk: proposal.Risk, Reversible: true, Limitations: []string{"Sticky、Split/W-Band 与厂商预设不属于本次通用 EQ 调度。", "参数结构验证不等于音乐性评价。"}, EvidenceRefs: append([]string(nil), instruction.EvidenceRefs...), ApprovalPrompt: approval}
}

func spalEQV2ProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, planned capabilityadapters.SPALPlanResult, envelope orchestration.ContextEnvelope, provider vpsEQV2ResolvedProvider) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: session.ActiveProposal.ID, Preview: presentationConclusion(presentation), ProposalPresentation: presentation, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: map[string]any{"session_id": session.ID, "capability_id": spalEQV2CapabilityID, "proposal_id": session.ActiveProposal.ID, "proposal_revision": session.ActiveProposal.Revision, "action_set_hash": session.ActiveProposal.ActionSetHash, "project_cut_hash": session.ActiveProposal.ProjectCutHash, "context_bundle_id": planned.Bundle.ID, "canary_stage": "proposal", "semantic_action": spalEQV2SemanticAction(session.FrozenPlan.ActionSet), "provider_source": "vps_v3_catalog", "vps_id": provider.Document.ID, "provider_credential_id": provider.Credential.ID, "provider_instance_id": provider.Instance.ID, "special_capabilities": provider.CatalogEntry.SpecialCapabilities, "special_invocation_policy": "user_initiated_only", "context_budget": capabilityCanaryContextBudget(envelope)}}
}

func spalEQV2AnalysisResponse(conversationID string, goal agentruntime.Goal, sessionID string, request spalEQV2Request, planned capabilityadapters.SPALPlanResult, envelope orchestration.ContextEnvelope, provider vpsEQV2ResolvedProvider) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "SPAL EQ v2 已完成只读解析和 Provider/Credential 匹配；inspect 模式未创建授权或写入插件。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": spalEQV2CapabilityID, "semantic_request": instructionMap(request.Instruction), "vps_id": provider.Document.ID, "provider_credential_id": provider.Credential.ID, "context_bundle_id": planned.Bundle.ID, "context_budget": capabilityCanaryContextBudget(envelope)}}
}

func spalEQV2ClarificationResponse(conversationID string, goal agentruntime.Goal, missing []string) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "要执行 SPAL EQ v2，请补充：" + strings.Join(missing, "、") + "。\n\n示例：用 TDR Nova 把 Band 2 设为 High Shelf，632 Hz，+5.4 dB，Q 0.95。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification), WorkflowData: map[string]any{"capability_id": spalEQV2CapabilityID, "missing_fields": missing}}
}

func vpsEQV2ProviderResponse(conversationID string, goal agentruntime.Goal, target string, cause error, candidates []spalReferenceEQProviderCandidate) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "当前目标没有与实际插件指纹匹配、且覆盖此 EQ 动作的 verified VPS EQ v2 Credential；没有执行参数写入。" + errorSuffix(cause), Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification), WorkflowData: map[string]any{"capability_id": spalEQV2CapabilityID, "canary_stage": "no_verified_eq_v2_provider", "target_ref": target, "cause": errorText(cause), "observed_candidates": candidates}}
}

func spalEQV2SemanticAction(actionSet orchestration.ActionSet) map[string]any {
	if len(actionSet.Actions) != 1 {
		return nil
	}
	manifest, err := spal.ManifestFromAction(actionSet.Actions[0])
	if err != nil {
		return nil
	}
	result := instructionMap(manifest.Instruction)
	result["operation"] = firstNonEmpty(manifest.Operation, spal.OperationApply)
	result["rollback_contract"] = "frozen_preimage_with_current_postimage_guard"
	return result
}

// spalEQV2RollbackRequested is deliberately explicit.  Generic undo language
// must not become a plug-in mutation; callers either set the typed operation
// or name SPAL EQ v2 with a rollback request.
func spalEQV2RollbackRequested(req ChatRequest) bool {
	operation := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		cleanContextText(req.Context["spal_operation"]),
		cleanContextText(mapValue(req.Context["spal_eq_v2"])["operation"]),
	)))
	if operation == spal.OperationRollback {
		return true
	}
	text := strings.ToLower(req.Message)
	return strings.Contains(text, "rollback") &&
		(strings.Contains(text, "spal eq") || strings.EqualFold(cleanContextText(req.Context["capability_id"]), spalEQV2CapabilityID))
}

// planSPALEQV2Rollback builds a new, separately-confirmable transaction.  It
// refuses to overwrite a manual or concurrent edit because NewRollbackManifest
// first compares the live physical postimage to the exact original writes.
func (s *Server) planSPALEQV2Rollback(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string) ChatResponse {
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 回滚需要可用的 Project-aware Runtime 与 Kernel。")
	}
	source, ok := s.latestSPALEQV2RollbackSource(conversationID, firstStringFromMap(req.Context, "rollback_session_id", "spal_rollback_session_id"))
	if !ok || source.FrozenPlan == nil {
		return ChatResponse{
			ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
			Reply:    "没有找到可回滚的 SPAL EQ v2 已执行 Receipt；不会写入插件。",
			Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
			WorkflowData: map[string]any{"capability_id": spalEQV2CapabilityID, "canary_stage": "rollback_source_missing"},
		}
	}
	if len(source.FrozenPlan.ActionSet.Actions) != 1 {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 回滚源缺少单一冻结 Action。")
	}
	original, err := spal.ManifestFromAction(source.FrozenPlan.ActionSet.Actions[0])
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "读取原始 SPAL EQ v2 Manifest 失败："+err.Error())
	}
	resolved, err := s.resolveVPSEQV2ProviderForFrozen(ctx, original.Binding.Instance, source.FrozenPlan.ProjectCut.ProjectUUID, original.Instruction)
	if err != nil {
		return vpsEQV2ProviderResponse(conversationID, goal, original.Instruction.TargetRef, err, nil)
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() || capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前工程无法建立强 Project Cut；没有创建 SPAL EQ v2 回滚 Proposal。")
	}
	if currentProjectUUID := spalReferenceEQProjectUUID(state); currentProjectUUID == "" || currentProjectUUID != source.FrozenPlan.ProjectCut.ProjectUUID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前工程与原 SPAL EQ v2 Receipt 不属于同一 project_uuid；没有创建回滚 Proposal。")
	}
	cut, err := s.buildVPSEQV2Cut(state, resolved, req)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 SPAL EQ v2 回滚 Project Cut："+err.Error())
	}
	current, err := (executionports.SPALVSPInspector{Client: s.kernel}).CaptureSPALPreimage(ctx, original.Binding, original.Preimage)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取 SPAL EQ v2 回滚前的当前参数："+err.Error())
	}
	rollback, err := spal.NewRollbackManifest(original, current)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 回滚被安全检查拒绝："+err.Error())
	}
	if strings.TrimSpace(sessionID) == "" || strings.EqualFold(sessionID, source.ID) {
		sessionID = s.nextCapabilitySessionID(conversationID, spalEQV2CapabilityID)
	}
	rollbackGoal := "Rollback SPAL EQ v2 transaction " + source.ID
	newSession, err := s.orchestrationRuntime.StartSPALEQV2ChatSession(sessionID, conversationID, cut.ProjectUUID, rollbackGoal, orchestration.InteractionPropose)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 SPAL EQ v2 回滚 Session："+err.Error())
	}
	proposal, actionSet, err := rollback.FreezeProposal(spalEQV2CapabilityID, "v2", cut, 1)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 SPAL EQ v2 回滚 Proposal："+err.Error())
	}
	proposal.Presentation = spalEQV2ProposalPresentation(proposal, rollback.Instruction, spal.OperationRollback)
	bundle := orchestration.ContextBundle{
		ID: "spal_eq_v2_rollback_context_" + sessionID, CapabilityID: spalEQV2CapabilityID, ProjectCutHash: cut.Hash,
		ArtifactRefs: []string{"spal.rollback_manifest:" + rollback.ID, "spal.original_manifest:" + original.ID},
		EvidenceRefs: appendUniqueSPALRefs(rollback.EvidenceRefs, resolved.EvidenceRefs...),
		Disclosure:   "A separately confirmable rollback to the original frozen physical preimage is ready.",
	}
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(newSession.ID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: bundle.ID,
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 SPAL EQ v2 回滚 Frozen Proposal："+err.Error())
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(newSession.ID, bundle, capabilityCanaryToolSchemas(s), []orchestration.ContextEntry{{
		ID: firstNonEmpty(goal.RunID, "rollback_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100,
	}}, orchestration.DefaultContextWindowBudget())
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL EQ v2 回滚 Context Envelope 无效："+err.Error())
	}
	return spalEQV2RollbackProposalResponse(conversationID, goal, updated, bundle, envelope, source.ID, resolved)
}

func (s *Server) latestSPALEQV2RollbackSource(conversationID, requestedID string) (orchestration.PlanningSession, bool) {
	if s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil {
		return orchestration.PlanningSession{}, false
	}
	candidates := []orchestration.PlanningSession{}
	for _, session := range s.orchestrationRuntime.Store.List() {
		if session.Invocation.CapabilityID != spalEQV2CapabilityID || !sessionBelongsToConversation(session, conversationID) {
			continue
		}
		if requestedID != "" && session.ID != requestedID {
			continue
		}
		if spalEQV2RollbackEligible(session) {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return orchestration.PlanningSession{}, false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt) })
	return candidates[0], true
}

func spalEQV2RollbackEligible(session orchestration.PlanningSession) bool {
	if session.FrozenPlan == nil || session.Execution == nil || len(session.FrozenPlan.ActionSet.Actions) != 1 {
		return false
	}
	manifest, err := spal.ManifestFromAction(session.FrozenPlan.ActionSet.Actions[0])
	if err != nil || !strings.EqualFold(firstNonEmpty(manifest.Operation, spal.OperationApply), spal.OperationApply) {
		return false
	}
	for _, receipt := range session.Execution.Receipts {
		if receipt.ActionID == session.FrozenPlan.ActionSet.Actions[0].ID && strings.EqualFold(receipt.Status, "applied") {
			return true
		}
	}
	return false
}

func spalEQV2RollbackProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, bundle orchestration.ContextBundle, envelope orchestration.ContextEnvelope, sourceID string, provider vpsEQV2ResolvedProvider) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: session.ActiveProposal.ID,
		Preview: presentationConclusion(presentation), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
			"proposal_id": session.ActiveProposal.ID, "proposal_revision": session.ActiveProposal.Revision,
			"action_set_hash": session.ActiveProposal.ActionSetHash, "project_cut_hash": session.ActiveProposal.ProjectCutHash,
			"context_bundle_id": bundle.ID, "canary_stage": "rollback_proposal", "approval_mode": "conversational",
			"proposal_presentation": presentation, "context_budget": capabilityCanaryContextBudget(envelope),
			"rollback_of_session_id": sourceID, "semantic_action": spalEQV2SemanticAction(session.FrozenPlan.ActionSet),
			"provider_source": "vps_v3_catalog", "vps_id": provider.Document.ID,
			"provider_credential_id": provider.Credential.ID, "provider_instance_id": provider.Instance.ID,
		},
	}
}
func instructionMap(instruction spal.Instruction) map[string]any {
	return map[string]any{"schema_id": instruction.SchemaID, "target_ref": instruction.TargetRef, "parameters": instruction.Parameters, "string_parameters": instruction.StringParameters}
}

func describeSPALEQV2Instruction(i spal.Instruction) string {
	return fmt.Sprintf("将对 %s 执行 %s：%v %v。", i.TargetRef, i.SchemaID, i.StringParameters, i.Parameters)
}
func parseEQV2BandRef(text string) string {
	match := eqV2BandPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	switch strings.ToLower(match[1]) {
	case "1", "i":
		return "b1"
	case "2", "ii":
		return "b2"
	case "3", "iii":
		return "b3"
	case "4", "iv":
		return "b4"
	}
	return ""
}
func parseEQV2Shape(text string) string {
	lower := strings.ToLower(text)
	switch {
	case containsAny(lower, "high shelf", "high-shelf", "highshelf") || containsAny(text, "高架", "高棚"):
		return "high_shelf"
	case containsAny(lower, "low shelf", "low-shelf", "lowshelf") || containsAny(text, "低架", "低棚"):
		return "low_shelf"
	case strings.Contains(lower, "bell") || strings.Contains(text, "钟形"):
		return "bell"
	}
	return ""
}
func parseEQV2Frequency(text string) (float64, bool) {
	match := eqV2FrequencyPattern.FindStringSubmatch(text)
	if len(match) < 3 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	if strings.HasPrefix(strings.ToLower(match[2]), "k") {
		value *= 1000
	}
	return value, value > 0
}
func regexFloat(pattern *regexp.Regexp, text string) (float64, bool) {
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func parseEQV2Enabled(text string, defaultValue float64) float64 {
	if isEQV2Off(text) {
		return 0
	}
	lower := strings.ToLower(text)
	if containsAny(lower, "on", "enable", "enabled", "open") || containsAny(text, "开启", "打开", "启用") {
		return 1
	}
	return defaultValue
}
func isEQV2Off(text string) bool {
	lower := strings.ToLower(text)
	return containsAny(lower, " off", "disable", "disabled", "close") || containsAny(text, "关闭", "停用")
}
func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
func errorSuffix(err error) string {
	if err == nil {
		return ""
	}
	return " 原因：" + err.Error()
}
