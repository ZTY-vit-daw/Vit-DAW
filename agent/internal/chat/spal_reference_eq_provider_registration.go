package chat

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/spallab"
)

var (
	spalReferenceEQProviderPluginPattern = regexp.MustCompile(`(?i)(?:plugin(?:[_ ]?id)?|插件(?:[_ ]?id)?)\s*[:=：]\s*([^\s,，；;]+)`)
	spalReferenceEQProviderBandPattern   = regexp.MustCompile(`(?i)(?:band|频段)\s*[:=：]?\s*(band[1-4])\b`)
)

// spalReferenceEQProviderRegistrationRequest contains only the explicit
// control-plane choices required to turn an already observed plug-in instance
// into a Provider record.  It deliberately contains no raw parameter name or
// value: Plugin Learning and the conformance adapter keep that implementation
// detail behind SPAL.
type spalReferenceEQProviderRegistrationRequest struct {
	TargetRef           string
	PluginID            string
	BandSlot            string
	StaticBellConfirmed bool
	Missing             []string
}

type spalReferenceEQProviderCandidate struct {
	TrackID    string
	TrackName  string
	TargetRef  string
	PluginID   string
	PluginName string
	PluginPath string
	Format     string
}

func (s *Server) handleSPALReferenceEQProviderRegistration(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ Provider 注册需要可用的 Project-aware Runtime 与 Project Kernel。")
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, spalReferenceEQProviderRegistrationCapabilityID)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Provider 注册 Proposal 的确认绑定已过期；没有写入注册表或修改工程。")
	}
	if exists && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
		return s.recoverSPALReferenceEQProviderRegistration(ctx, conversationID, goal, session)
	}
	if exists && session.ActiveProposal != nil {
		if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok {
			switch decision.Kind {
			case orchestration.ApprovalApprove:
				state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
				if err != nil || state == nil || !state.OK() {
					return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程状态，SPAL Provider 注册 Proposal 未获授权。")
				}
				return s.handleSPALReferenceEQProviderRegistrationAuthorization(ctx, conversationID, req, goal, session, state)
			case orchestration.ApprovalRevise, orchestration.ApprovalNarrow:
				return capabilityProposalWaitingResponse(conversationID, goal, session, "Provider 注册 Proposal 不会在冻结后隐式改写目标、插件实例、频段或静态 Bell 证明。请取消后重新发起注册。", "provider_registration_revision_requires_new_request")
			}
		}
		if session.Status == orchestration.StatusAuthorized {
			return authorizedCapabilityWaitingResponse(conversationID, goal, session)
		}
		return proposalAmbiguousResponse(conversationID, goal, session)
	}

	registration := parseSPALReferenceEQProviderRegistrationRequest(req)
	if len(registration.Missing) > 0 {
		return spalReferenceEQProviderRegistrationClarificationResponse(conversationID, goal, registration.Missing)
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程 snapshot；没有发现或登记任何 Provider 实例。")
	}
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Kernel 未提供 command.base_revision_cas；SPAL Provider 注册不会在缺少强 Project Cut 时创建 Proposal。")
	}
	projectUUID := spalReferenceEQProjectUUID(state)
	if projectUUID == "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Project Cut 缺少 project_uuid；没有登记 Provider 实例。")
	}
	candidate, candidates, selection := selectObservedSPALReferenceEQProviderCandidate(state, registration)
	if selection != "" {
		return spalReferenceEQProviderCandidateResponse(conversationID, goal, registration, candidates, selection)
	}

	readback, err := s.readSPALReferenceEQProviderParameters(ctx, candidate)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前已观察插件的参数；没有登记 Provider 实例。"+err.Error())
	}
	attestationRef := "chat_attestation:spal_reference_eq_static_bell:" + firstNonEmpty(goal.RunID, sessionID, conversationID)
	staticBellEvidence := append([]string{attestationRef}, req.ArtifactRefs...)
	record, err := spallab.ConformTDRNovaFromPluginReply(spallab.TDRNovaConformanceRequest{
		ProjectUUID: projectUUID,
		TargetRef:   candidate.TargetRef,
		TrackID:     candidate.TrackID,
		PluginID:    candidate.PluginID,
		BandSlot:    registration.BandSlot,
		// The parser accepts this only from an explicit user statement/context;
		// the adapter then checks the observed invariant instead of guessing a
		// vendor enum value.
		StaticBellConfirmed: registration.StaticBellConfirmed,
		StaticBellEvidence:  staticBellEvidence,
	}, readback)
	if err != nil {
		return spalReferenceEQProviderConformanceResponse(conversationID, goal, candidate, err)
	}
	cut, err := s.buildSPALReferenceEQProviderRegistrationCut(state, record, req)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 SPAL Provider 注册 Project Cut："+err.Error())
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartSPALReferenceEQProviderRegistrationChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context))
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 SPAL Provider 注册 Planning Session："+err.Error())
		}
	}
	bundle := orchestration.ContextBundle{
		ID:             "spal_provider_registration:" + record.ID + ":" + cut.Hash[:16],
		CapabilityID:   spalReferenceEQProviderRegistrationCapabilityID,
		ProjectCutHash: cut.Hash,
		ArtifactRefs:   appendUniqueSPALRefs(nil, append(req.ArtifactRefs, "spal.provider_record:"+record.ID)...),
		EvidenceRefs:   appendUniqueSPALRefs(nil, record.ConformanceEvidence...),
		Disclosure:     "The frozen registration stores one observed, conformed Reference EQ Provider instance only; it does not load or choose a fallback plug-in.",
	}
	envelope, envelopeErr := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		sessionID, bundle, capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if envelopeErr != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Provider 注册 Context Envelope 未通过预算准入："+envelopeErr.Error())
	}
	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return ChatResponse{
			ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
			Reply:    "SPAL Provider 注册已完成只读一致性检查：当前实例、Plugin Learning 记录和静态 Bell 证明匹配。此次为 inspect，不会写入 Provider 注册表。",
			Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted),
			WorkflowData: map[string]any{
				"session_id": sessionID, "capability_id": spalReferenceEQProviderRegistrationCapabilityID, "canary_stage": "analysis",
				"candidate": candidate, "provider_record_id": record.ID, "project_cut_hash": cut.Hash,
				"context_budget": capabilityCanaryContextBudget(envelope), "user_acceptance": "unknown",
			},
		}
	}
	revision := int64(1)
	if session.ActiveProposal != nil {
		revision = session.ActiveProposal.Revision + 1
	}
	proposal, actionSet, err := spallab.FreezeProviderRegistrationProposal(record, cut, revision)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 SPAL Provider 注册 Proposal："+err.Error())
	}
	proposal.Presentation = spalReferenceEQProviderRegistrationPresentation(proposal, candidate, record)
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: bundle.ID, FrozenAt: time.Now().UTC(),
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 SPAL Provider 注册 Frozen Proposal："+err.Error())
	}
	return spalReferenceEQProviderRegistrationProposalResponse(conversationID, goal, updated, bundle, envelope, candidate, record)
}

func (s *Server) handleSPALReferenceEQProviderRegistrationAuthorization(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 SPAL Provider 注册 Proposal 缺少冻结的 ActionSet，不能授权。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || state == nil || !state.OK() || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision || spalReferenceEQProjectUUID(state) != frozen.ProjectCut.ProjectUUID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision、epoch 或 project_uuid 已变化；SPAL Provider 注册 Proposal 未获授权，请重新生成。")
	}
	if !s.orchestrationRuntime.HasDurableStore() || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Provider 注册需要持久化 Session Store 与 Project History；当前不会写入注册表。")
	}
	record, err := spalReferenceEQProviderRegistrationRecord(frozen)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "冻结的 SPAL Provider 注册记录无效："+err.Error())
	}
	if err := record.ProductReadyFor(frozen.ProjectCut.ProjectUUID); err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "冻结的 SPAL Provider 注册记录不属于当前工程："+err.Error())
	}
	if err := s.validateSPALReferenceEQRecord(ctx, record); err != nil {
		return spalReferenceEQProviderValidationResponse(conversationID, goal, record.Instance.TargetRef, err)
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: []string{"spal.provider_record:" + record.ID}},
		capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "authorization_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Provider 注册授权 Context Envelope 无效："+err.Error())
	}
	authorized := session
	if session.Status != orchestration.StatusAuthorized {
		proposal := frozen.Proposal
		decision, ok := capabilityApprovalDecisionFromContext(req.Context)
		if !ok || !decision.ExactApprovalFor(proposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定该 Proposal revision 的明确 Provider 注册授权。")
		}
		authorized, err = s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
			ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: proposal.ActionSetHash,
			ProjectCutHash: proposal.ProjectCutHash, Scope: append([]string(nil), proposal.TargetScope...),
			SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision,
		})
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法持久化 SPAL Provider 注册授权："+err.Error())
		}
	}
	store, err := s.spalReferenceEQProviderStore()
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法打开 SPAL Provider 注册表："+err.Error())
	}
	port := spallab.ProviderRegistrationPort{
		Store: store,
		ValidateRecord: func(checkCtx context.Context, candidate spallab.ProviderRecord) error {
			return s.validateSPALReferenceEQRecord(checkCtx, candidate)
		},
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut, port,
		executionverifiers.SPALProviderRegistration{},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	response := capabilityCanaryExecutionResponse(conversationID, goal, executed, envelope, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	return spalReferenceEQProviderRegistrationExecutionResponse(response, record)
}

func (s *Server) recoverSPALReferenceEQProviderRegistration(ctx context.Context, conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Provider 注册执行恢复缺少 Frozen Plan。")
	}
	record, err := spalReferenceEQProviderRegistrationRecord(*session.FrozenPlan)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取待恢复的 SPAL Provider 注册记录："+err.Error())
	}
	store, err := s.spalReferenceEQProviderStore()
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法打开 SPAL Provider 注册表以恢复执行："+err.Error())
	}
	port := spallab.ProviderRegistrationPort{Store: store}
	recovered, reconcileErr := s.orchestrationRuntime.ReconcileActionSet(ctx, session.ID, session.FrozenPlan.ActionSet, port, executionverifiers.SPALProviderRegistration{})
	envelope, _ := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: session.FrozenPlan.ContextBundleID, CapabilityID: session.FrozenPlan.ActionSet.CapabilityID, ProjectCutHash: session.FrozenPlan.ProjectCut.Hash, ArtifactRefs: []string{"spal.provider_record:" + record.ID}},
		capabilityCanaryToolSchemas(s), nil, orchestration.DefaultContextWindowBudget(),
	)
	response := capabilityCanaryExecutionResponse(conversationID, goal, recovered, envelope, reconcileErr)
	response.WorkflowData["recovered"] = true
	return spalReferenceEQProviderRegistrationExecutionResponse(response, record)
}

func parseSPALReferenceEQProviderRegistrationRequest(req ChatRequest) spalReferenceEQProviderRegistrationRequest {
	values := mapValue(req.Context["spal_reference_eq_provider"])
	result := spalReferenceEQProviderRegistrationRequest{
		TargetRef: firstNonEmpty(
			spalReferenceEQContextText(values, "target_ref"),
			spalReferenceEQContextText(req.Context, "spal_target_ref"),
			spalReferenceEQContextText(req.Context, "selected_plugin_track_id"),
			spalReferenceEQContextText(req.Context, "selected_track_id"),
			spalReferenceEQContextText(req.Context, "primary_selected_track_id"),
		),
		PluginID: firstNonEmpty(
			spalReferenceEQContextText(values, "plugin_id"),
			spalReferenceEQContextText(req.Context, "spal_provider_plugin_id"),
			spalReferenceEQContextText(req.Context, "selected_plugin_id"),
		),
		BandSlot: strings.ToLower(firstNonEmpty(
			spalReferenceEQContextText(values, "band_slot"),
			spalReferenceEQContextText(req.Context, "spal_band_slot"),
		)),
		StaticBellConfirmed: spalReferenceEQProviderContextBool(values, "static_bell_confirmed") ||
			spalReferenceEQProviderContextBool(req.Context, "spal_static_bell_confirmed"),
	}
	if result.TargetRef == "" {
		if match := spalReferenceEQTrackRefPattern.FindString(req.Message); match != "" {
			result.TargetRef = strings.TrimSpace(match)
		} else if match := spalReferenceEQTargetPattern.FindStringSubmatch(req.Message); len(match) == 2 {
			result.TargetRef = trimSPALReferenceEQTarget(match[1])
		}
	}
	if result.PluginID == "" {
		if match := spalReferenceEQProviderPluginPattern.FindStringSubmatch(req.Message); len(match) == 2 {
			result.PluginID = strings.TrimSpace(match[1])
		}
	}
	if result.BandSlot == "" {
		if match := spalReferenceEQProviderBandPattern.FindStringSubmatch(req.Message); len(match) == 2 {
			result.BandSlot = strings.ToLower(strings.TrimSpace(match[1]))
		}
	}
	lower := strings.ToLower(req.Message)
	if strings.Contains(lower, "static bell") || (strings.Contains(req.Message, "静态") && strings.Contains(lower, "bell")) {
		result.StaticBellConfirmed = true
	}
	if result.TargetRef == "" && result.PluginID == "" {
		result.Missing = append(result.Missing, "目标轨道（target_ref）或当前选中的插件实例")
	}
	if result.BandSlot == "" {
		result.Missing = append(result.Missing, "用于本次 Reference EQ 的频段（band1 至 band4）")
	}
	if !result.StaticBellConfirmed {
		result.Missing = append(result.Missing, "对所选频段已是静态 Bell 的明确确认")
	}
	return result
}

func spalReferenceEQProviderContextBool(values map[string]any, key string) bool {
	value, ok := values[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
		case "1", "true", "yes", "on", "confirmed":
			return true
		}
	}
	return false
}

func selectObservedSPALReferenceEQProviderCandidate(state *kernel.VSPStateResult, request spalReferenceEQProviderRegistrationRequest) (spalReferenceEQProviderCandidate, []spalReferenceEQProviderCandidate, string) {
	candidates := observedSPALReferenceEQProviderCandidates(state)
	matching := make([]spalReferenceEQProviderCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if request.PluginID != "" && !strings.EqualFold(candidate.PluginID, request.PluginID) {
			continue
		}
		if request.TargetRef != "" && !spalReferenceEQProviderTargetMatches(candidate, request.TargetRef) {
			continue
		}
		matching = append(matching, candidate)
	}
	if len(matching) == 0 {
		return spalReferenceEQProviderCandidate{}, candidates, "no_observed_reference_eq_candidate"
	}
	if len(matching) > 1 {
		return spalReferenceEQProviderCandidate{}, matching, "provider_instance_selection_required"
	}
	return matching[0], matching, ""
}

func observedSPALReferenceEQProviderCandidates(state *kernel.VSPStateResult) []spalReferenceEQProviderCandidate {
	all := observedVPSProviderCandidates(state)
	out := make([]spalReferenceEQProviderCandidate, 0, len(all))
	for _, candidate := range all {
		if isObservedTDRNovaReferenceEQ(candidate.PluginName, candidate.PluginPath, candidate.Format) {
			out = append(out, candidate)
		}
	}
	return out
}

// observedVPSProviderCandidates enumerates currently loaded VST3 instances
// without assigning them a task badge.  The general VPS/Catalog resolver must
// see every observed VST3 and later match the exact Credential fingerprint;
// the older TDR-only Reference EQ experiment applies its own narrow filter
// above and must never become a hidden global provider filter.
func observedVPSProviderCandidates(state *kernel.VSPStateResult) []spalReferenceEQProviderCandidate {
	if state == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]spalReferenceEQProviderCandidate, 0)
	for _, track := range mapRowsFromAny(state.LegacyState["tracks"]) {
		trackID := firstStringFromMap(track, "track_id", "id", "item_id")
		if trackID == "" {
			continue
		}
		trackName := firstStringFromMap(track, "track_name", "name", "display_name")
		appendRows := func(rows []map[string]any) {
			for _, row := range rows {
				pluginID := firstStringFromMap(row, "plugin_id", "plugin_item_id", "node_id", "item_id", "id")
				if pluginID == "" {
					continue
				}
				pluginName := firstStringFromMap(row, "plugin_name", "name", "descriptive_name", "display_name")
				pluginPath := firstStringFromMap(row, "plugin_path", "path", "file_path", "source_path")
				format := firstStringFromMap(row, "format", "plugin_format")
				if !isObservedVST3Plugin(format) {
					continue
				}
				key := trackID + "::" + pluginID
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, spalReferenceEQProviderCandidate{
					TrackID: trackID, TrackName: trackName, TargetRef: "track:" + trackID,
					PluginID: pluginID, PluginName: pluginName, PluginPath: pluginPath, Format: format,
				})
			}
		}
		appendRows(mapRowsFromAny(track["plugins"]))
		appendRows(mapRowsFromAny(track["rack_nodes"]))
		if rack := mapValue(track["rack"]); len(rack) > 0 {
			appendRows(mapRowsFromAny(rack["nodes"]))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TrackID == out[j].TrackID {
			return out[i].PluginID < out[j].PluginID
		}
		return out[i].TrackID < out[j].TrackID
	})
	return out
}

func isObservedVST3Plugin(format string) bool {
	format = strings.TrimSpace(format)
	return format == "" || strings.EqualFold(format, "VST3")
}

func isObservedTDRNovaReferenceEQ(pluginName, pluginPath, format string) bool {
	identity := strings.ToLower(strings.TrimSpace(pluginName + " " + pluginPath))
	if !strings.Contains(identity, "tdr nova") {
		return false
	}
	format = strings.TrimSpace(format)
	return format == "" || strings.EqualFold(format, "VST3")
}

func spalReferenceEQProviderTargetMatches(candidate spalReferenceEQProviderCandidate, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return false
	}
	if strings.EqualFold(candidate.TargetRef, requested) || strings.EqualFold(candidate.TrackID, requested) || strings.EqualFold(candidate.TrackName, requested) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(requested), "track:") {
		value := strings.TrimSpace(strings.TrimPrefix(requested, "track:"))
		return strings.EqualFold(candidate.TrackID, value) || strings.EqualFold(candidate.TrackName, value)
	}
	return false
}

// spalReferenceEQProviderTargetMatchesAny reports whether candidate matches
// any of the requested refs. Callers may have several candidate refs of
// differing reliability (an explicit target_ref the model supplied, a
// track_id, a selected_track_id from context); rather than pick just the
// first non-empty one and let an unresolved literal like "current_selection"
// shadow a good track_id, any match should count.
func spalReferenceEQProviderTargetMatchesAny(candidate spalReferenceEQProviderCandidate, requested []string) bool {
	for _, ref := range requested {
		if spalReferenceEQProviderTargetMatches(candidate, ref) {
			return true
		}
	}
	return false
}

func (s *Server) readSPALReferenceEQProviderParameters(ctx context.Context, candidate spalReferenceEQProviderCandidate) (map[string]any, error) {
	reply, err := s.kernel.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd": "get_plugin_parameters", "track_id": candidate.TrackID, "plugin_id": candidate.PluginID,
	}, "spal:reference-eq-provider:conform:"+candidate.TrackID+":"+candidate.PluginID, "")
	if err != nil {
		return nil, err
	}
	legacy := reply.LegacyLikeReply()
	if strings.EqualFold(firstStringFromMap(legacy, "status"), "error") {
		return nil, fmt.Errorf("get_plugin_parameters failed: %s", firstStringFromMap(legacy, "message", "error"))
	}
	if trackID := firstStringFromMap(legacy, "track_id"); trackID != "" && trackID != candidate.TrackID {
		return nil, fmt.Errorf("current parameter readback track does not match the observed candidate")
	}
	if pluginID := firstStringFromMap(legacy, "plugin_id", "plugin_item_id"); pluginID != "" && pluginID != candidate.PluginID {
		return nil, fmt.Errorf("current parameter readback plug-in does not match the observed candidate")
	}
	return legacy, nil
}

func (s *Server) buildSPALReferenceEQProviderRegistrationCut(state *kernel.VSPStateResult, record spallab.ProviderRecord, req ChatRequest) (orchestration.ProjectCut, error) {
	artifacts := append([]string(nil), req.ArtifactRefs...)
	artifacts = append(artifacts, canaryStringSlice(req.Context["artifact_refs"])...)
	return projectcut.Build(projectcut.BuildRequest{
		State: state, Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: []string{
			"vsp.snapshot:" + state.SnapshotHash,
			"spal.provider_candidate:" + record.Instance.TrackID + ":" + record.Instance.PluginID,
			"spal.plugin_skill:" + record.PluginSkillSignature,
			"spal.current_parameter_signature:" + record.CurrentParameterSignature,
		},
		TargetFingerprints: []string{
			"track:" + record.Instance.TrackID,
			"plugin:" + record.Instance.TrackID + ":" + record.Instance.PluginID,
			"spal.provider_registration:" + record.ID,
		},
		ArtifactRefs: artifacts,
		ContractVersions: []string{
			"capability:" + spalReferenceEQProviderRegistrationCapabilityID,
			"spal:reference_eq_provider_registration.v0",
			"spallab:" + spallab.SchemaVersion,
		},
	})
}

func spalReferenceEQProjectUUID(state *kernel.VSPStateResult) string {
	if state == nil {
		return ""
	}
	if value := firstStringFromMap(state.LegacyState, "project_uuid", "id"); value != "" {
		return value
	}
	return firstStringFromMap(mapValue(state.LegacyState["project"]), "project_uuid", "id")
}

func (s *Server) spalReferenceEQProviderStore() (*spallab.ProviderStore, error) {
	return spallab.NewProviderStore(spalReferenceEQProviderStorePath())
}

func spalReferenceEQProviderRegistrationRecord(frozen orchestration.FrozenPlan) (spallab.ProviderRecord, error) {
	if frozen.ActionSet.CapabilityID != spalReferenceEQProviderRegistrationCapabilityID || len(frozen.ActionSet.Actions) != 1 {
		return spallab.ProviderRecord{}, fmt.Errorf("frozen plan is not a single SPAL Provider registration action")
	}
	return spallab.ProviderRecordFromRegistrationAction(frozen.ActionSet.Actions[0])
}

func spalReferenceEQProviderRegistrationPresentation(proposal orchestration.Proposal, candidate spalReferenceEQProviderCandidate, record spallab.ProviderRecord) *orchestration.ProposalPresentation {
	name := firstNonEmpty(candidate.PluginName, "当前已观察到的 Reference EQ")
	return &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema,
		ProposalID:    proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID,
		Title:      "SPAL Reference EQ Provider 注册方案",
		Conclusion: fmt.Sprintf("将把当前工程中已观察到的 %s 实例登记为 %s 的已验证 Reference EQ Provider。此操作不加载插件，也不会修改任何音频参数。", name, record.Instance.TargetRef),
		AnalysisSummary: []string{
			"已读取当前插件参数并验证 Plugin Learning 映射与参数签名。",
			fmt.Sprintf("用户已明确确认 %s 为静态 Bell；SPAL 将冻结这一当前状态而非猜测滤波器枚举。", record.Instance.Metadata["band_slot"]),
			"注册仅绑定当前 project_uuid、轨道和插件实例；其他项目或实例不能复用该绑定。",
		},
		Recommendation: "确认后仅写入这条冻结的 Provider 注册记录。随后仍需单独发起并确认 SPAL Reference EQ 测试。",
		AnalyzedTracks: 1, ActionCount: 1, Risk: proposal.Risk, Reversible: false,
		Readiness: []orchestration.ProposalMetric{
			{ID: "observed_instance", Label: "当前已观察实例", Value: "已匹配", Status: "ready"},
			{ID: "plugin_learning", Label: "Plugin Learning 一致性", Value: "已验证", Status: "ready"},
			{ID: "static_bell_attestation", Label: "静态 Bell 明确确认", Value: "已冻结", Status: "ready"},
			{ID: "registry_readback", Label: "注册表结构回读", Value: "执行后验证", Status: "ready"},
		},
		Limitations: []string{
			"此 Proposal 只登记已验证实例，不执行 EQ，也不表示混音效果改善。",
			"Reference EQ 的参数写入、信号验证、Receipt 与回滚仍是后续独立 Proposal。",
		},
		EvidenceRefs:   appendUniqueSPALRefs(nil, record.ConformanceEvidence...),
		ApprovalPrompt: "你可以回复“执行这个方案”或点击确认，登记当前冻结实例；也可以取消。",
	}
}

func spalReferenceEQProviderRegistrationProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, bundle orchestration.ContextBundle, envelope orchestration.ContextEnvelope, candidate spalReferenceEQProviderCandidate, record spallab.ProviderRecord) ChatResponse {
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
			"context_bundle_id": bundle.ID, "canary_stage": "provider_registration_proposal", "approval_mode": "conversational",
			"proposal_presentation": presentation, "context_budget": capabilityCanaryContextBudget(envelope),
			"candidate": candidate, "provider_record_id": record.ID, "provider_project_uuid": record.ProjectUUID,
		},
	}
}

func spalReferenceEQProviderRegistrationExecutionResponse(response ChatResponse, record spallab.ProviderRecord) ChatResponse {
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["provider_record_id"] = record.ID
	response.WorkflowData["provider_project_uuid"] = record.ProjectUUID
	response.WorkflowData["provider_instance_id"] = record.Instance.ID
	response.WorkflowData["user_acceptance"] = "unknown"
	if response.WorkflowData["verification_result"] != nil {
		response.WorkflowData["structural_verification"] = "pass"
	}
	if response.GoalStatus == string(agentruntime.StatusCompleted) {
		response.Reply = "SPAL Reference EQ Provider 已登记并完成注册表结构回读。它只说明当前已观察实例可被后续 SPAL 测试绑定；没有执行 EQ，也不表示混音效果成功。现在可单独发起 Reference EQ 测试 Proposal。"
	}
	return response
}

func spalReferenceEQProviderRegistrationClarificationResponse(conversationID string, goal agentruntime.Goal, missing []string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "要注册 SPAL Reference EQ Provider，请补充：" + strings.Join(missing, "、") + "。\n\n示例：注册当前 TDR Nova 为 SPAL Reference EQ Provider：目标: track:1007，插件: 1013，频段: band1，已确认静态 Bell。\n\nVit 只会检查当前已加载、已观察到的实例；不会自动加载、替换或猜测插件。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQProviderRegistrationCapabilityID, "canary_stage": "registration_clarification", "missing_fields": append([]string(nil), missing...),
		},
	}
}

func spalReferenceEQProviderCandidateResponse(conversationID string, goal agentruntime.Goal, request spalReferenceEQProviderRegistrationRequest, candidates []spalReferenceEQProviderCandidate, reason string) ChatResponse {
	reply := "当前目标没有可登记的已观察 TDR Nova Reference EQ 实例。Vit 不会加载替代插件或把其他 EQ 当作 Provider。"
	if reason == "provider_instance_selection_required" {
		reply = "当前目标存在多个已观察 TDR Nova 实例。请通过选中具体插件，或在请求中加入“插件: <plugin_id>”后重新发起注册；Vit 不会隐式选择其中一个。"
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: reply, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQProviderRegistrationCapabilityID, "canary_stage": reason,
			"target_ref": request.TargetRef, "plugin_id": request.PluginID, "observed_candidates": candidates,
		},
	}
}

func spalReferenceEQProviderConformanceResponse(conversationID string, goal agentruntime.Goal, candidate spalReferenceEQProviderCandidate, cause error) ChatResponse {
	message := "当前实例无法完成 SPAL Reference EQ Provider 一致性验证；没有写入注册表。"
	stage := "provider_conformance_blocked"
	if cause != nil && strings.Contains(strings.ToLower(cause.Error()), "plugin skill") {
		stage = "plugin_learning_required"
		message = "当前实例尚无可用的 Plugin Learning 记录，或记录与当前参数签名不一致。请先对这个已选实例完成 Plugin Learning，再重新发起注册；Vit 不会猜测参数映射。"
	}
	if cause != nil && stage != "plugin_learning_required" {
		message += " " + cause.Error()
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: message, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQProviderRegistrationCapabilityID, "canary_stage": stage,
			"candidate": candidate, "plugin_learning_required": stage == "plugin_learning_required",
		},
	}
}
