package chat

// This file is the deliberately narrow product-path bridge for SPAL v0.  It
// is not B4: it accepts an explicitly stated static-bell request and proves
// that one semantic action can travel through Ask Vit, Proposal confirmation,
// VSP, Receipt, verification and a separately confirmable rollback.

import (
	"context"
	"fmt"
	"math"
	"os"
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
	"vit-daw-agent/internal/spallab"
)

const (
	spalReferenceEQSchemaID     = spal.StaticBellControlID
	spalReferenceEQMaxAbsGainDB = 6.0
)

var (
	spalReferenceEQTrackRefPattern  = regexp.MustCompile(`(?i)\btrack:[a-z0-9._-]+`)
	spalReferenceEQTargetPattern    = regexp.MustCompile(`(?i)(?:目标(?:轨道)?|target(?:[_ ]?(?:ref|track))?)\s*[:=：]\s*([^,，;；\n]+)`)
	spalReferenceEQFrequencyPattern = regexp.MustCompile(`(?i)(?:频率|frequency|freq)\s*[:=：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:hz|赫兹)?`)
	spalReferenceEQGainPattern      = regexp.MustCompile(`(?i)(?:增益|gain)\s*[:=：]?\s*([+-]?[0-9]+(?:\.[0-9]+)?)\s*(?:db)?`)
	// Bare unit-qualified values cover the natural form users actually type,
	// such as "92Hz、-2.5dB、Q 1.2", without treating arbitrary numbers as EQ
	// controls.  Labelled forms above remain preferred when both are present.
	spalReferenceEQBareFrequencyPattern = regexp.MustCompile(`(?i)(?:^|[\s,，;；、(（\[【])([0-9]+(?:\.[0-9]+)?)\s*(?:hz|赫兹)\b`)
	spalReferenceEQBareGainPattern      = regexp.MustCompile(`(?i)(?:^|[\s,，;；、(（\[【])([+-]?[0-9]+(?:\.[0-9]+)?)\s*db\b`)
	spalReferenceEQQPattern             = regexp.MustCompile(`(?i)(?:\bq\b|q值|q value)\s*[:=：]?\s*([0-9]+(?:\.[0-9]+)?)`)
	spalReferenceEQClipPattern          = regexp.MustCompile(`(?i)(?:片段(?:id)?|clip(?:[_ ]?id)?)\s*[:=：]\s*([^\s,，;；]+)`)
	spalReferenceEQRangePattern         = regexp.MustCompile(`(?i)(?:范围|时间范围|range|time(?: range)?)\s*[:=：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:秒|s)?\s*(?:-|~|到|to)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:秒|s)?`)
	spalReferenceEQTailPattern          = regexp.MustCompile(`(?i)(?:尾音|尾部|tail)\s*[:=：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:秒|s)?`)
	spalReferenceEQBandPattern          = regexp.MustCompile(`(?i)(?:检测频段|signal band|band)\s*[:=：]?\s*([0-9]+(?:\.[0-9]+)?)\s*(?:hz)?\s*(?:-|~|到|to)\s*([0-9]+(?:\.[0-9]+)?)\s*(?:hz)?`)
	spalReferenceEQProviderPattern      = regexp.MustCompile(`(?i)(?:provider(?:[_ ]?record)?|provider_record)\s*[:=：]\s*([^\s,，;；]+)`)
)

type spalReferenceEQRequest struct {
	TargetRef            string
	PluginID             string
	ProviderRecordID     string // Retained only to decode historical requests; v3 dispatch ignores it.
	ProviderCredentialID string
	FrequencyHz          float64
	GainDB               float64
	Q                    float64
	BandLowHz            float64
	BandHighHz           float64
	Scope                spal.SignalProbeScope
	Missing              []string
}

func (s *Server) handleSPALReferenceEQRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ 测试需要可用的 Project-aware Runtime 与 Project Kernel。")
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, spalReferenceEQTestCapabilityID)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ Proposal 的确认绑定已过期；没有授权或修改工程。")
	}
	if exists && (session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying) {
		return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
	}
	if exists && session.ActiveProposal != nil {
		if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok {
			switch decision.Kind {
			case orchestration.ApprovalApprove:
				state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
				if err != nil || state == nil || !state.OK() {
					return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程状态，SPAL Reference EQ Proposal 未获授权。")
				}
				return s.handleSPALReferenceEQAuthorization(ctx, conversationID, req, goal, session, state)
			case orchestration.ApprovalRevise, orchestration.ApprovalNarrow:
				return spalReferenceEQRevisionResponse(conversationID, goal, session)
			}
		}
		if session.Status == orchestration.StatusAuthorized {
			return authorizedCapabilityWaitingResponse(conversationID, goal, session)
		}
		return proposalAmbiguousResponse(conversationID, goal, session)
	}

	if spalReferenceEQRollbackRequested(req) {
		return s.planSPALReferenceEQRollback(ctx, conversationID, req, goal, sessionID)
	}

	request := parseSPALReferenceEQRequest(req)
	if len(request.Missing) > 0 {
		return spalReferenceEQClarificationResponse(conversationID, goal, request.Missing)
	}
	providerState, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || providerState == nil || !providerState.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程 snapshot；SPAL Reference EQ 测试未启动。")
	}
	providerProjectUUID := spalReferenceEQProjectUUID(providerState)
	if providerProjectUUID == "" {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Project Cut 缺少 project_uuid；SPAL Reference EQ 测试未启动。")
	}
	resolved, candidates, selectionErr := s.resolveVPSStaticEQProvider(ctx, providerState, request)
	if selectionErr != nil {
		return vpsSPALProviderResponse(conversationID, goal, request.TargetRef, selectionErr, candidates)
	}
	request.TargetRef = resolved.Instance.TargetRef
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取当前工程 snapshot，SPAL Reference EQ 测试未启动。")
	}
	if currentProjectUUID := spalReferenceEQProjectUUID(state); currentProjectUUID == "" || currentProjectUUID != providerProjectUUID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "Provider 选择期间工程已切换；SPAL Reference EQ 测试不会跨项目复用注册实例。")
	}
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 Kernel 未提供 command.base_revision_cas；SPAL Reference EQ 测试不会在缺少强 Project Cut 的情况下生成 Proposal。")
	}
	cut, err := s.buildVPSStaticEQCut(state, resolved, req)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 SPAL Reference EQ Project Cut："+err.Error())
	}
	if !exists {
		session, err = s.orchestrationRuntime.StartSPALReferenceEQTestChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context))
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 SPAL Reference EQ Planning Session："+err.Error())
		}
	}
	instruction, err := request.instructionForProvider("vps.provider_credential:"+resolved.Credential.ID, resolved.EvidenceRefs)
	if err != nil {
		return spalReferenceEQClarificationResponse(conversationID, goal, []string{err.Error()})
	}
	if err := validateSPALReferenceEQSignalScope(state, spallab.ProviderRecord{Instance: resolved.Instance}, instruction.SignalProbeScope); err != nil {
		return spalReferenceEQClarificationResponse(conversationID, goal, []string{err.Error()})
	}
	planned, err := capabilityadapters.PlanSPAL(capabilityadapters.SPALPlanRequest{
		Context: ctx, SessionID: sessionID, Goal: req.Message, Mode: canaryInteractionMode(req.Context),
		CapabilityID: spalReferenceEQTestCapabilityID, CapabilityVersion: "v0", ProjectCut: cut,
		Instruction: instruction, Registry: resolved.Registry, Instances: []spal.ProviderInstance{resolved.Instance},
		PreimageReader: executionports.SPALVSPInspector{Client: s.kernel},
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ 只读准备失败："+err.Error())
	}
	l2Ready, l2Note := s.spalReferenceEQL2Availability(ctx)
	planned.Bundle.ArtifactRefs = appendUniqueSPALRefs(planned.Bundle.ArtifactRefs, "vps.provider_credential:"+resolved.Credential.ID, "vps.document:"+resolved.Document.ID)
	planned.Bundle.EvidenceRefs = appendUniqueSPALRefs(planned.Bundle.EvidenceRefs, resolved.EvidenceRefs...)
	if !l2Ready && l2Note != "" {
		planned.Bundle.OmissionReasons = appendUniqueSPALRefs(planned.Bundle.OmissionReasons, "signal_probe_unavailable:"+l2Note)
	}
	envelope, envelopeErr := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		sessionID, planned.Bundle, capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if envelopeErr != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ Context Envelope 未通过预算准入："+envelopeErr.Error())
	}
	if canaryInteractionMode(req.Context) == orchestration.InteractionInspect {
		return spalReferenceEQAnalysisResponse(conversationID, goal, sessionID, request, planned, envelope, l2Ready, l2Note)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || planned.Preparation.Manifest == nil {
		return spalReferenceEQPlanOutcomeResponse(conversationID, goal, request.TargetRef, planned)
	}
	revision := int64(1)
	if session.ActiveProposal != nil {
		revision = session.ActiveProposal.Revision + 1
	}
	proposal, actionSet, err := capabilityadapters.FreezeSPALProposal(planned, spalReferenceEQTestCapabilityID, "v0", cut, revision)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 SPAL Reference EQ Proposal："+err.Error())
	}
	proposal.Presentation = spalReferenceEQProposalPresentation(proposal, actionSet, instruction, l2Ready, l2Note)
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: planned.Bundle.ID,
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 SPAL Reference EQ Frozen Proposal："+err.Error())
	}
	return spalReferenceEQProposalResponse(conversationID, goal, updated, planned, envelope, resolved, l2Ready, l2Note)
}

func (s *Server) handleSPALReferenceEQAuthorization(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, state *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前 SPAL Reference EQ Proposal 缺少冻结的 ActionSet，不能授权。")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || state == nil || !state.OK() || state.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || state.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 或强 Project Cut 已变化；该 SPAL Reference EQ Proposal 未获授权，请重新生成。")
	}
	if !s.orchestrationRuntime.HasDurableStore() || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ 执行需要持久化 Session Store 和 Project History；当前不会修改工程。")
	}
	if err := s.validateSPALReferenceEQFrozenProvider(ctx, frozen); err != nil {
		return vpsSPALProviderResponse(conversationID, goal, "", err, nil)
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		session.ID,
		orchestration.ContextBundle{ID: frozen.ContextBundleID, CapabilityID: frozen.ActionSet.CapabilityID, ProjectCutHash: frozen.ProjectCut.Hash, ArtifactRefs: []string{"spal.manifest:" + frozen.Proposal.CandidateID}},
		capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "authorization_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL Reference EQ 授权 Context Envelope 无效："+err.Error())
	}
	authorized := session
	if session.Status != orchestration.StatusAuthorized {
		proposal := frozen.Proposal
		decision, ok := capabilityApprovalDecisionFromContext(req.Context)
		if !ok || !decision.ExactApprovalFor(proposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定该 Proposal revision 的明确授权。")
		}
		authorized, err = s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
			ProposalID: proposal.ID, ProposalRevision: proposal.Revision, ActionSetHash: proposal.ActionSetHash,
			ProjectCutHash: proposal.ProjectCutHash, Scope: append([]string(nil), proposal.TargetScope...),
			SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision,
		})
		if err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法持久化 SPAL Reference EQ 授权："+err.Error())
		}
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
		s.spalReferenceEQMutationPort(ctx),
		executionverifiers.SPAL{Signal: executionverifiers.ReceiptSPALSignalProbe{}},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	response := spalReferenceEQExecutionResponse(conversationID, goal, executed, envelope, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	return response
}

func (s *Server) loadSPALReferenceEQProviderRecords() ([]spallab.ProviderRecord, error) {
	store, err := s.spalReferenceEQProviderStore()
	if err != nil {
		return nil, err
	}
	return store.List()
}

func (s *Server) spalReferenceEQRegistry() (*spal.Registry, error) {
	store, err := spallab.NewProviderStore(spalReferenceEQProviderStorePath())
	if err != nil {
		return nil, err
	}
	return store.Registry()
}

// The product fixture reads the existing, explicitly conformed reference
// records.  This is intentionally not a general plug-in catalog and it never
// provisions or loads a plug-in.  A later Provider Registry can replace this
// bridge without changing the semantic instruction or capability lifecycle.
func spalReferenceEQProviderStorePath() string {
	if path := strings.TrimSpace(os.Getenv("VIT_SPAL_REFERENCE_EQ_PROVIDER_STORE")); path != "" {
		return path
	}
	return spallab.DefaultReferenceEQProviderStorePath()
}

func selectSPALReferenceEQProvider(records []spallab.ProviderRecord, projectUUID, targetRef, requestedRecordID string) (spallab.ProviderRecord, string, error) {
	projectUUID = strings.TrimSpace(projectUUID)
	targetRef = strings.TrimSpace(targetRef)
	requestedRecordID = strings.TrimSpace(requestedRecordID)
	matching := make([]spallab.ProviderRecord, 0, 1)
	for _, record := range records {
		if !strings.EqualFold(strings.TrimSpace(record.ProjectUUID), projectUUID) || !strings.EqualFold(strings.TrimSpace(record.Instance.Metadata["project_uuid"]), projectUUID) {
			continue
		}
		if requestedRecordID != "" && !strings.EqualFold(record.ID, requestedRecordID) {
			continue
		}
		if !spalReferenceEQTargetMatches(record, targetRef) {
			continue
		}
		matching = append(matching, record)
	}
	sort.Slice(matching, func(i, j int) bool { return matching[i].ID < matching[j].ID })
	switch len(matching) {
	case 0:
		return spallab.ProviderRecord{}, "", fmt.Errorf("no_verified_provider")
	case 1:
		return matching[0], matching[0].Instance.TargetRef, nil
	default:
		return spallab.ProviderRecord{}, "", fmt.Errorf("provider_selection_required")
	}
}

func spalReferenceEQTargetMatches(record spallab.ProviderRecord, requested string) bool {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return false
	}
	if strings.EqualFold(record.Instance.TargetRef, requested) || strings.EqualFold(record.Instance.TrackID, requested) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(record.Instance.TargetRef), "track:") && strings.EqualFold(strings.TrimPrefix(record.Instance.TargetRef, "track:"), requested) {
		return true
	}
	return false
}

func (s *Server) validateSPALReferenceEQRecord(ctx context.Context, record spallab.ProviderRecord) error {
	if s == nil || s.kernel == nil {
		return fmt.Errorf("Project Kernel is unavailable")
	}
	reply, err := s.kernel.SendVSPLegacyCommandWithIDs(ctx, map[string]any{
		"cmd": "get_plugin_parameters", "track_id": record.Instance.TrackID, "plugin_id": record.Instance.PluginID,
	}, "spal:reference-eq:validate:"+record.ID, "")
	if err != nil {
		return fmt.Errorf("read current Reference EQ parameters: %w", err)
	}
	return spallab.ValidateProviderRecordCurrent(record, reply.LegacyLikeReply())
}

func (s *Server) validateSPALReferenceEQFrozenProvider(ctx context.Context, frozen orchestration.FrozenPlan) error {
	if len(frozen.ActionSet.Actions) != 1 {
		return fmt.Errorf("SPAL Reference EQ requires exactly one frozen semantic action")
	}
	manifest, err := spal.ManifestFromAction(frozen.ActionSet.Actions[0])
	if err != nil {
		return err
	}
	resolved, err := s.resolveVPSStaticEQProviderForFrozen(ctx, manifest.Binding.Instance, frozen.ProjectCut.ProjectUUID)
	if err != nil {
		return err
	}
	if resolved.Instance.ProviderID != manifest.Binding.Provider.ID || resolved.Instance.TargetRef != manifest.Instruction.TargetRef {
		return fmt.Errorf("verified VPS Provider no longer matches the frozen SPAL binding")
	}
	return nil
}

func (s *Server) spalReferenceEQProviderRecordForInstance(instanceID, projectUUID string) (spallab.ProviderRecord, error) {
	records, err := s.loadSPALReferenceEQProviderRecords()
	if err != nil {
		return spallab.ProviderRecord{}, err
	}
	for _, record := range records {
		if record.Instance.ID == strings.TrimSpace(instanceID) && record.ProductReadyFor(projectUUID) == nil {
			return record, nil
		}
	}
	return spallab.ProviderRecord{}, fmt.Errorf("frozen SPAL Provider instance is no longer registered")
}

func (s *Server) buildSPALReferenceEQCut(state *kernel.VSPStateResult, record spallab.ProviderRecord, req ChatRequest) (orchestration.ProjectCut, error) {
	artifacts := append([]string(nil), req.ArtifactRefs...)
	artifacts = append(artifacts, canaryStringSlice(req.Context["artifact_refs"])...)
	return projectcut.Build(projectcut.BuildRequest{
		// Both callers have just negotiated command.base_revision_cas against
		// the same live Kernel.  Do not repeat that round trip with a detached
		// background Context while constructing the immutable Cut.
		State: state, Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: []string{
			"vsp.snapshot:" + state.SnapshotHash,
			"spal.provider_record:" + record.ID + ":" + record.CurrentParameterSignature,
			"spal.plugin_skill:" + record.PluginSkillSignature,
		},
		TargetFingerprints: []string{
			"track:" + record.Instance.TrackID,
			"plugin:" + record.Instance.TrackID + ":" + record.Instance.PluginID,
			"spal.binding:" + record.Instance.ID,
		},
		ArtifactRefs: artifacts,
		ContractVersions: []string{
			"capability:" + spalReferenceEQTestCapabilityID,
			"spal:" + spal.SchemaVersion,
			"spallab:" + spallab.SchemaVersion,
		},
	})
}

func (s *Server) spalReferenceEQL2Availability(ctx context.Context) (bool, string) {
	if s == nil || s.kernel == nil {
		return false, "Project Kernel unavailable"
	}
	enabled, err := s.kernel.VSPFeature(ctx, "audio.l2_render_probe")
	if err != nil {
		return false, err.Error()
	}
	if !enabled {
		return false, "audio.l2_render_probe is not negotiated"
	}
	return true, ""
}

func (s *Server) spalReferenceEQMutationPort(ctx context.Context) *executionports.SPALSignalCapturePort {
	port := &executionports.SPALSignalCapturePort{Mutation: &executionports.SPALVSPPort{Client: s.kernel}}
	if enabled, _ := s.spalReferenceEQL2Availability(ctx); enabled {
		port.Signal = executionports.SPALL2RenderProbe{Client: s.kernel}
	}
	return port
}

// A frozen clip scope must belong to the same track as the Provider binding.
// This prevents a selected UI clip on another track from silently becoming the
// evidence target for the EQ action.  Explicit time ranges are validated by
// the SPAL schema and remain valid without a clip lookup.
func validateSPALReferenceEQSignalScope(state *kernel.VSPStateResult, record spallab.ProviderRecord, scope spal.SignalProbeScope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(scope.ClipID) == "" {
		return nil
	}
	for _, track := range mapRowsFromAny(state.LegacyState["tracks"]) {
		trackID := firstStringFromMap(track, "track_id", "id")
		if trackID != record.Instance.TrackID {
			continue
		}
		for _, clip := range mapRowsFromAny(track["clips"]) {
			if firstStringFromMap(clip, "clip_id", "id", "item_id") == scope.ClipID {
				return nil
			}
		}
		return fmt.Errorf("信号验证片段 %s 不属于已验证 Provider 所在轨道", scope.ClipID)
	}
	return fmt.Errorf("已验证 Provider 轨道 %s 不在当前 Project Cut snapshot 中", record.Instance.TrackID)
}

func parseSPALReferenceEQRequest(req ChatRequest) spalReferenceEQRequest {
	text := normalizeSPALReferenceEQText(req.Message)
	contextValues := mapValue(req.Context["spal_reference_eq"])
	result := spalReferenceEQRequest{
		// The normal Godot path already carries the selected track/clip IDs in
		// request context.  They are safe explicit references, unlike choosing
		// an arbitrary first track or clip, so the user can test a selected item
		// without having to type opaque engine IDs.
		TargetRef: firstNonEmpty(
			spalReferenceEQContextText(contextValues, "target_ref"),
			spalReferenceEQContextText(req.Context, "spal_target_ref"),
			spalReferenceEQContextText(req.Context, "selected_track_id"),
			spalReferenceEQContextText(req.Context, "primary_selected_track_id"),
		),
		PluginID: firstNonEmpty(
			spalReferenceEQContextText(contextValues, "plugin_id"),
			spalReferenceEQContextText(req.Context, "selected_plugin_id"),
			spalReferenceEQContextText(req.Context, "primary_selected_plugin_id"),
		),
		ProviderRecordID: firstNonEmpty(spalReferenceEQContextText(contextValues, "provider_record_id"), spalReferenceEQContextText(req.Context, "spal_provider_record_id")),
		ProviderCredentialID: firstNonEmpty(
			spalReferenceEQContextText(contextValues, "provider_credential_id"),
			spalReferenceEQContextText(req.Context, "spal_provider_credential_id"),
		),
	}
	if result.TargetRef == "" {
		if match := spalReferenceEQTrackRefPattern.FindString(text); match != "" {
			result.TargetRef = strings.TrimSpace(match)
		} else if match := spalReferenceEQTargetPattern.FindStringSubmatch(text); len(match) == 2 {
			result.TargetRef = trimSPALReferenceEQTarget(match[1])
		}
	}
	if result.ProviderRecordID == "" {
		if match := spalReferenceEQProviderPattern.FindStringSubmatch(text); len(match) == 2 {
			result.ProviderRecordID = strings.TrimSpace(match[1])
		}
	}
	result.FrequencyHz, _ = spalReferenceEQContextNumber(contextValues, "frequency_hz")
	if result.FrequencyHz == 0 {
		result.FrequencyHz, _ = spalReferenceEQContextNumber(req.Context, "spal_frequency_hz")
	}
	if result.FrequencyHz == 0 {
		result.FrequencyHz, _ = spalReferenceEQFrequencyFromText(text)
	}
	result.GainDB, _ = spalReferenceEQContextNumber(contextValues, "gain_db")
	if result.GainDB == 0 {
		if value, ok := spalReferenceEQContextNumber(req.Context, "spal_gain_db"); ok {
			result.GainDB = value
		} else if value, ok := spalReferenceEQGainFromText(text); ok {
			result.GainDB = value
		}
	}
	result.Q, _ = spalReferenceEQContextNumber(contextValues, "q")
	if result.Q == 0 {
		result.Q, _ = spalReferenceEQContextNumber(req.Context, "spal_q")
	}
	if result.Q == 0 {
		result.Q, _ = spalReferenceEQRegexNumber(spalReferenceEQQPattern, text, 1)
	}
	result.BandLowHz, _ = spalReferenceEQContextNumber(contextValues, "band_low_hz")
	result.BandHighHz, _ = spalReferenceEQContextNumber(contextValues, "band_high_hz")
	if result.BandLowHz == 0 || result.BandHighHz == 0 {
		if low, ok := spalReferenceEQRegexNumber(spalReferenceEQBandPattern, text, 1); ok {
			if high, highOK := spalReferenceEQRegexNumber(spalReferenceEQBandPattern, text, 2); highOK {
				result.BandLowHz, result.BandHighHz = low, high
			}
		}
	}
	result.Scope = spalReferenceEQScopeFromRequest(contextValues, req.Context, text)
	if result.TargetRef == "" {
		result.Missing = append(result.Missing, "目标（target_ref）")
	}
	if result.FrequencyHz == 0 {
		result.Missing = append(result.Missing, "频率（frequency_hz）")
	}
	if result.GainDB == 0 {
		result.Missing = append(result.Missing, "增益（gain_db，且不能为 0）")
	}
	if result.Q == 0 {
		result.Missing = append(result.Missing, "Q")
	}
	if !result.Scope.IsConfigured() {
		result.Missing = append(result.Missing, "用于信号验证的片段（clip_id）或时间范围")
	}
	return result
}

func trimSPALReferenceEQTarget(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	for _, marker := range []string{" 频率", " frequency", " freq", " 增益", " gain", " q", " 片段", " clip", " 范围", " range"} {
		if index := strings.Index(lower, marker); index >= 0 {
			value = value[:index]
			break
		}
	}
	return strings.Trim(strings.TrimSpace(value), "\"'")
}

func spalReferenceEQContextText(values map[string]any, key string) string {
	return strings.TrimSpace(cleanContextText(values[key]))
}

func spalReferenceEQContextNumber(values map[string]any, key string) (float64, bool) {
	value, ok := values[key]
	if !ok || value == nil {
		return 0, false
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func spalReferenceEQRegexNumber(pattern *regexp.Regexp, text string, group int) (float64, bool) {
	match := pattern.FindStringSubmatch(text)
	if len(match) <= group {
		return 0, false
	}
	number, err := strconv.ParseFloat(match[group], 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func normalizeSPALReferenceEQText(text string) string {
	return strings.NewReplacer("−", "-", "＋", "+").Replace(text)
}

func spalReferenceEQFrequencyFromText(text string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{spalReferenceEQFrequencyPattern, spalReferenceEQBareFrequencyPattern} {
		if value, ok := spalReferenceEQRegexNumber(pattern, text, 1); ok {
			return value, true
		}
	}
	return 0, false
}

func spalReferenceEQGainFromText(text string) (float64, bool) {
	for _, pattern := range []*regexp.Regexp{spalReferenceEQGainPattern, spalReferenceEQBareGainPattern} {
		if value, ok := spalReferenceEQRegexNumber(pattern, text, 1); ok {
			return value, true
		}
	}
	return 0, false
}

// isVPSStaticBellControlIntent deliberately recognizes only the complete
// user-facing static-bell tuple.  It makes the verified VPS v3 route usable
// through natural language without turning broad EQ or mix requests into this
// narrow, confirmation-gated product path.
func isVPSStaticBellControlIntent(message string) bool {
	text := normalizeSPALReferenceEQText(message)
	lower := strings.ToLower(strings.TrimSpace(text))
	hasExplicitStaticBell := strings.Contains(lower, "static bell") || strings.Contains(lower, "static-bell")
	hasChineseStaticBell := strings.Contains(text, "静态") && strings.Contains(lower, "bell") && (strings.Contains(lower, "eq") || strings.Contains(text, "均衡"))
	if !hasExplicitStaticBell && !hasChineseStaticBell {
		return false
	}
	frequency, frequencyOK := spalReferenceEQFrequencyFromText(text)
	gain, gainOK := spalReferenceEQGainFromText(text)
	q, qOK := spalReferenceEQRegexNumber(spalReferenceEQQPattern, text, 1)
	return frequencyOK && frequency > 0 && gainOK && math.Abs(gain) > .0001 && qOK && q > 0
}

func spalReferenceEQScopeFromRequest(values, root map[string]any, text string) spal.SignalProbeScope {
	clipID := firstNonEmpty(
		spalReferenceEQContextText(values, "clip_id"),
		spalReferenceEQContextText(root, "spal_clip_id"),
		spalReferenceEQContextText(root, "selected_clip_id"),
		spalReferenceEQContextText(root, "primary_selected_clip_id"),
	)
	if clipID == "" {
		if match := spalReferenceEQClipPattern.FindStringSubmatch(text); len(match) == 2 {
			clipID = strings.TrimSpace(match[1])
		}
	}
	scope := spal.SignalProbeScope{TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: clipID}
	if clipID != "" {
		if tail, ok := spalReferenceEQScopeNumber(values, root, "tail_seconds", "spal_tail_seconds"); ok {
			scope.TailSeconds = &tail
		} else if tail, ok := spalReferenceEQRegexNumber(spalReferenceEQTailPattern, text, 1); ok {
			scope.TailSeconds = &tail
		}
		return scope
	}
	start, startOK := spalReferenceEQScopeNumber(values, root, "start_seconds", "spal_start_seconds")
	end, endOK := spalReferenceEQScopeNumber(values, root, "end_seconds", "spal_end_seconds")
	if !startOK || !endOK {
		if parsedStart, ok := spalReferenceEQRegexNumber(spalReferenceEQRangePattern, text, 1); ok {
			if parsedEnd, endParsed := spalReferenceEQRegexNumber(spalReferenceEQRangePattern, text, 2); endParsed {
				start, end, startOK, endOK = parsedStart, parsedEnd, true, true
			}
		}
	}
	if startOK && endOK {
		scope.StartSeconds, scope.EndSeconds = &start, &end
		if tail, ok := spalReferenceEQScopeNumber(values, root, "tail_seconds", "spal_tail_seconds"); ok {
			scope.TailSeconds = &tail
		} else if tail, ok := spalReferenceEQRegexNumber(spalReferenceEQTailPattern, text, 1); ok {
			scope.TailSeconds = &tail
		}
		return scope
	}
	return spal.SignalProbeScope{}
}

func spalReferenceEQScopeNumber(values, root map[string]any, key, rootKey string) (float64, bool) {
	if value, ok := spalReferenceEQContextNumber(values, key); ok {
		return value, true
	}
	if value, ok := spalReferenceEQContextNumber(root, rootKey); ok {
		return value, true
	}
	// Godot's selected-item context uses unprefixed timing fields.  They are
	// still explicit user/UI scope, so accept them without requiring an
	// internal SPAL-prefixed duplicate.
	return spalReferenceEQContextNumber(root, key)
}

func (r spalReferenceEQRequest) instruction(record spallab.ProviderRecord) (spal.Instruction, error) {
	return r.instructionForProvider("spal.reference_eq_test:provider_record:"+record.ID, record.ConformanceEvidence)
}

func (r spalReferenceEQRequest) instructionForProvider(providerEvidenceRef string, conformanceEvidence []string) (spal.Instruction, error) {
	if r.TargetRef == "" || r.FrequencyHz <= 0 || r.Q <= 0 {
		return spal.Instruction{}, fmt.Errorf("目标、正数频率和正数 Q 是必需的")
	}
	if math.Abs(r.GainDB) < .0001 {
		return spal.Instruction{}, fmt.Errorf("gain_db 不能为 0；Reference EQ 测试必须有明确的信号变化方向")
	}
	if math.Abs(r.GainDB) > spalReferenceEQMaxAbsGainDB {
		return spal.Instruction{}, fmt.Errorf("gain_db 必须在 ±%.1f dB 的 Reference EQ 测试安全范围内", spalReferenceEQMaxAbsGainDB)
	}
	low, high := r.BandLowHz, r.BandHighHz
	if low == 0 && high == 0 {
		span := math.Max(5, r.FrequencyHz/(2*r.Q))
		low = math.Max(10, r.FrequencyHz-span)
		high = math.Min(40000, r.FrequencyHz+span)
	}
	if low <= 0 || high <= low {
		return spal.Instruction{}, fmt.Errorf("检测频段必须满足低频率小于高频率")
	}
	limit := spalReferenceEQMaxAbsGainDB
	direction := "increase"
	if r.GainDB < 0 {
		direction = "decrease"
	}
	evidence := append([]string{providerEvidenceRef}, conformanceEvidence...)
	return spal.Instruction{
		SchemaID: spalReferenceEQSchemaID, TargetRef: r.TargetRef,
		Parameters:           map[string]float64{"center_frequency_hz": r.FrequencyHz, "gain_db": r.GainDB, "q": r.Q},
		SafetyBounds:         spal.SafetyBounds{MaxAbsoluteGainDB: &limit},
		EvidenceRefs:         appendUniqueSPALRefs(nil, evidence...),
		ExpectedSignalChange: spal.SignalExpectation{BandLowHz: low, BandHighHz: high, Direction: direction},
		SignalProbeScope:     r.Scope,
	}, nil
}

func appendUniqueSPALRefs(base []string, refs ...string) []string {
	seen := make(map[string]bool, len(base)+len(refs))
	out := make([]string, 0, len(base)+len(refs))
	for _, value := range append(append([]string(nil), base...), refs...) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func spalReferenceEQClarificationResponse(conversationID string, goal agentruntime.Goal, missing []string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "要启动 SPAL Reference EQ 测试，请补充：" + strings.Join(missing, "、") + "。\n\n示例：SPAL Reference EQ 测试：目标: track:bass，频率: 92Hz，增益: -2.5dB，Q: 1.2，片段: clip-001。\n\n这是一条显式参数测试路径，不是 B4 低频诊断；确认前不会加载插件或修改工程。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQTestCapabilityID, "canary_stage": "parameter_clarification",
			"missing_fields": append([]string(nil), missing...), "semantic_schema": spalReferenceEQSchemaID,
		},
	}
}

func spalReferenceEQProviderResponse(conversationID string, goal agentruntime.Goal, targetRef string, cause error, records []spallab.ProviderRecord, projectUUID string) ChatResponse {
	targets := make([]string, 0, len(records))
	recordIDs := make([]string, 0, len(records))
	for _, record := range records {
		if err := record.ProductReadyFor(projectUUID); err != nil {
			continue
		}
		targets = append(targets, record.Instance.TargetRef)
		recordIDs = append(recordIDs, record.ID)
	}
	targets = appendUniqueSPALRefs(nil, targets...)
	recordIDs = appendUniqueSPALRefs(nil, recordIDs...)
	stage := "no_verified_provider"
	reply := "目标 " + firstNonEmpty(targetRef, "（未指定）") + " 没有可用的已验证 SPAL Provider 实例。Vit 不会自动选择 TDR Nova、加载替代 EQ，或猜测插件参数。请先通过 Plugin Learning 生成并完成该实例的验证记录，然后重新发起这条测试。"
	if cause != nil && strings.Contains(cause.Error(), "no_verified_provider") {
		reply = "目标 " + firstNonEmpty(targetRef, "（未指定）") + " 没有当前项目可用的已验证 SPAL Provider 实例。Vit 不会自动选择 TDR Nova、加载替代 EQ，或猜测插件参数。请先对当前已加载实例完成 Plugin Learning，然后发起“注册当前 TDR Nova 为 SPAL Reference EQ Provider：频段: band1，已确认静态 Bell”。"
	}
	if cause != nil && strings.Contains(cause.Error(), "provider_selection_required") {
		stage = "provider_selection_required"
		reply = "该目标存在多个已验证的 SPAL Provider 实例。为了避免隐式选择，请在请求中加入 provider_record: <记录 ID> 后重新发起测试。"
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: reply,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQTestCapabilityID, "canary_stage": stage,
			"blockers": []string{"no_verified_provider", "generate_plugin_skill"}, "target_ref": targetRef,
			"available_verified_targets": targets, "available_provider_record_ids": recordIDs,
			"plugin_learning_required": true,
		},
	}
}

func spalReferenceEQProviderValidationResponse(conversationID string, goal agentruntime.Goal, targetRef string, err error) ChatResponse {
	reason := "已验证 Provider 的当前实例状态与其一致性记录不匹配。"
	if err != nil {
		reason += " " + err.Error()
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    reason + " Vit 不会把旧的 Plugin Skill 或参数映射继续用于执行；请重新运行 Plugin Learning/一致性验证后再试。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQTestCapabilityID, "canary_stage": "provider_validation_blocked",
			"blockers": []string{"provider_conformance_stale", "generate_plugin_skill"}, "target_ref": targetRef,
			"plugin_learning_required": true,
		},
	}
}

func spalReferenceEQPlanOutcomeResponse(conversationID string, goal agentruntime.Goal, targetRef string, planned capabilityadapters.SPALPlanResult) ChatResponse {
	stage := "planning_blocked"
	status := string(agentruntime.StatusFailed)
	if planned.Outcome.Kind == orchestration.OutcomeBlocked || planned.Outcome.Kind == orchestration.OutcomeNeedUserInput {
		stage = "no_verified_provider"
		status = string(agentruntime.StatusWaitingClarification)
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    firstNonEmpty(planned.Outcome.Summary, "SPAL Reference EQ 当前不能形成可执行 Proposal。") + " 当前没有执行或加载任何插件。",
		Workflow: "capability_runtime_v1", GoalStatus: status,
		WorkflowData: map[string]any{
			"capability_id": spalReferenceEQTestCapabilityID, "canary_stage": stage, "target_ref": targetRef,
			"blockers": append([]string(nil), planned.Outcome.Blockers...), "plugin_learning_required": planned.Outcome.Kind == orchestration.OutcomeBlocked,
		},
	}
}

func spalReferenceEQProposalPresentation(proposal orchestration.Proposal, actionSet orchestration.ActionSet, instruction spal.Instruction, l2Ready bool, l2Note string) *orchestration.ProposalPresentation {
	values, _ := instruction.StaticBellValues()
	operation := spal.OperationApply
	if len(actionSet.Actions) == 1 {
		if manifest, err := spal.ManifestFromAction(actionSet.Actions[0]); err == nil {
			operation = manifest.Operation
		}
	}
	isRollback := strings.EqualFold(operation, spal.OperationRollback)
	title := "SPAL Reference EQ 测试方案"
	conclusion := fmt.Sprintf("将对 %s 应用一个有界、可回滚的静态 Bell 语义控制：%.1f Hz，%+.2f dB，Q %.2f。", instruction.TargetRef, values.CenterFrequencyHz, values.GainDB, values.Q)
	recommendation := "确认后由 SPAL 绑定的已验证 Provider 执行；能力层不会接触插件名称或原始参数 ID。"
	if isRollback {
		title = "SPAL Reference EQ 测试回滚方案"
		conclusion = fmt.Sprintf("将把 %s 恢复到原始冻结物理预像。回滚本身也需要这一次明确确认。", instruction.TargetRef)
		recommendation = "SPAL 会先检查当前参数仍等于原操作后的状态；若有人工或并发修改，将拒绝回滚。"
	}
	limitations := []string{
		"结构验证只证明插件实例存在、参数写入且回读一致，不等于混音效果成功。",
		"即使信号方向通过，音乐性/用户试听结论仍保持 unknown。",
	}
	signalStatus := "ready"
	signalValue := "L2 同 tap 目标频段方向验证将在执行后运行"
	if !l2Ready {
		signalStatus = "limited"
		signalValue = "当前 Kernel 未协商 L2 render probe；执行后信号验证会明确显示 inconclusive"
		limitations = append(limitations, signalValue+"。"+l2Note)
	}
	return &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema,
		ProposalID:    proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID,
		Title: title, Conclusion: conclusion,
		AnalysisSummary: []string{
			fmt.Sprintf("语义 schema：%s；预期目标频段 %.1f–%.1f Hz 信号%s。", instruction.SchemaID, instruction.ExpectedSignalChange.BandLowHz, instruction.ExpectedSignalChange.BandHighHz, signalDirectionLabel(instruction.ExpectedSignalChange.Direction)),
			"Provider 仅在当前项目存在一条已验证实例且其 Plugin Skill/滤波器状态仍匹配时才会绑定。",
		},
		Recommendation: recommendation, AnalyzedTracks: 1, ActionCount: len(actionSet.Actions), Risk: proposal.Risk, Reversible: true,
		Readiness: []orchestration.ProposalMetric{
			{ID: "verified_provider", Label: "已验证 Provider 实例", Value: "已绑定", Status: "ready"},
			{ID: "structural_readback", Label: "参数结构回读", Value: "执行后验证", Status: "ready"},
			{ID: "signal_probe", Label: "目标频段信号验证", Value: signalValue, Status: signalStatus},
		},
		ChangeGroups: []orchestration.ProposalChangeGroup{{
			ID: "spal.static_bell", Label: "静态 Bell 语义控制", Function: "spectral_control",
			TrackCount: 1, MoveCount: 1, MinValue: values.GainDB, MaxValue: values.GainDB, Unit: "dB",
		}},
		Limitations: limitations, EvidenceRefs: firstProposalRefs(instruction.EvidenceRefs, 12),
		ApprovalPrompt: "你可以回复“执行这个方案”或点击确认；也可以取消。确认仅授权当前 Proposal revision。",
	}
}

func signalDirectionLabel(direction string) string {
	if strings.EqualFold(direction, "decrease") {
		return "下降"
	}
	return "上升"
}

func spalReferenceEQProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, planned capabilityadapters.SPALPlanResult, envelope orchestration.ContextEnvelope, provider vpsStaticEQResolvedProvider, l2Ready bool, l2Note string) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: session.ActiveProposal.ID,
		Preview: firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
			"proposal_id": session.ActiveProposal.ID, "proposal_revision": session.ActiveProposal.Revision,
			"action_set_hash": session.ActiveProposal.ActionSetHash, "project_cut_hash": session.ActiveProposal.ProjectCutHash,
			"context_bundle_id": planned.Bundle.ID, "canary_stage": "proposal", "approval_mode": "conversational",
			"proposal_presentation": presentation, "context_budget": capabilityCanaryContextBudget(envelope),
			"semantic_action":        spalReferenceEQSemanticAction(session.FrozenPlan.ActionSet),
			"signal_probe_available": l2Ready, "signal_probe_note": l2Note,
			"provider_source":        "vps_v3_catalog",
			"vps_id":                 provider.Document.ID,
			"provider_credential_id": provider.Credential.ID,
			"provider_instance_id":   provider.Instance.ID,
		},
	}
}

func spalReferenceEQAnalysisResponse(conversationID string, goal agentruntime.Goal, sessionID string, request spalReferenceEQRequest, planned capabilityadapters.SPALPlanResult, envelope orchestration.ContextEnvelope, l2Ready bool, l2Note string) ChatResponse {
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    "SPAL Reference EQ 已完成只读可用性检查：已解析显式语义参数并验证了当前 Provider 实例。此次为 inspect，不会创建 Proposal、授权或修改工程。",
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: map[string]any{
			"session_id": sessionID, "capability_id": spalReferenceEQTestCapabilityID, "canary_stage": "analysis",
			"context_bundle_id": planned.Bundle.ID, "project_cut_hash": planned.Bundle.ProjectCutHash,
			"semantic_request":       map[string]any{"target_ref": request.TargetRef, "frequency_hz": request.FrequencyHz, "gain_db": request.GainDB, "q": request.Q},
			"signal_probe_available": l2Ready, "signal_probe_note": l2Note, "context_budget": capabilityCanaryContextBudget(envelope),
		},
	}
}

func spalReferenceEQRevisionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	return capabilityProposalWaitingResponse(conversationID, goal, session,
		"SPAL Reference EQ v0 不会在已冻结 Proposal 内隐式重写频率、增益、Q 或检测范围。请取消当前 Proposal 后，用完整的显式参数重新发起测试；当前工程没有被修改。", "revision_requires_new_reference_eq_request")
}

func spalReferenceEQSemanticAction(actionSet orchestration.ActionSet) map[string]any {
	if len(actionSet.Actions) != 1 {
		return nil
	}
	manifest, err := spal.ManifestFromAction(actionSet.Actions[0])
	if err != nil {
		return nil
	}
	values, err := manifest.Instruction.StaticBellValues()
	if err != nil {
		return nil
	}
	return map[string]any{
		"schema_id": manifest.Instruction.SchemaID, "target_ref": manifest.Instruction.TargetRef,
		"frequency_hz": values.CenterFrequencyHz, "gain_db": values.GainDB, "q": values.Q,
		"expected_signal_change": manifest.Instruction.ExpectedSignalChange, "signal_probe_scope": manifest.Instruction.SignalProbeScope,
		"operation": firstNonEmpty(manifest.Operation, spal.OperationApply), "rollback_contract": "frozen_preimage_with_current_postimage_guard",
	}
}

func spalReferenceEQExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, envelope orchestration.ContextEnvelope, executeErr error) ChatResponse {
	response := capabilityCanaryExecutionResponse(conversationID, goal, session, envelope, executeErr)
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["semantic_action"] = spalReferenceEQSemanticActionFromSession(session)
	response.WorkflowData["user_acceptance"] = "unknown"
	response.WorkflowData["rollback_available"] = spalReferenceEQRollbackEligible(session)
	if session.Execution == nil {
		return response
	}
	verification := session.Execution.VerificationResult
	structural, signal := "inconclusive", "not_run"
	if verification != nil {
		structural, signal = verification.Structural, verification.Acoustic
	}
	response.WorkflowData["structural_verification"] = structural
	response.WorkflowData["signal_verification"] = signal
	switch session.Status {
	case orchestration.StatusCompleted:
		response.Reply = "SPAL Reference EQ 参数已执行并完成结构/信号验证。它只说明当前冻结的语义操作按预期落地；音乐性和用户试听结论仍为 unknown。"
		response.GoalStatus = string(agentruntime.StatusCompleted)
	case orchestration.StatusNeedsReview:
		response.GoalStatus = string(agentruntime.StatusWaitingContinue)
		switch strings.ToLower(signal) {
		case "pass":
			response.Reply = "SPAL Reference EQ 参数已写入并通过结构回读，目标频段信号方向也符合预期。系统不把这称为“混音成功”；音乐性/用户试听仍为 unknown，已保留 Receipt 与可确认回滚路径。"
		case "mismatch", "fail":
			response.Reply = "SPAL Reference EQ 参数已写入并通过结构回读，但目标频段的信号变化方向与预期不符。参数执行不是失败；结果被标记为 needs_review，不会自动重试或宣称混音成功。"
		default:
			response.Reply = "SPAL Reference EQ 参数已写入并通过结构回读，但目标频段信号验证尚无结论。参数执行不是失败；音乐性/用户试听仍为 unknown，已保留 Receipt 与可确认回滚路径。"
		}
	case orchestration.StatusFailed:
		response.Reply = "SPAL Reference EQ 执行未形成可接受的结构结果；请查看 Receipt 中的补偿/回滚状态。系统不会把它描述为混音效果成功。"
	}
	return response
}

func spalReferenceEQSemanticActionFromSession(session orchestration.PlanningSession) map[string]any {
	if session.FrozenPlan == nil {
		return nil
	}
	return spalReferenceEQSemanticAction(session.FrozenPlan.ActionSet)
}

func spalReferenceEQRollbackRequested(req ChatRequest) bool {
	operation := strings.ToLower(strings.TrimSpace(firstNonEmpty(cleanContextText(req.Context["spal_operation"]), cleanContextText(mapValue(req.Context["spal_reference_eq"])["operation"]))))
	if operation == "rollback" {
		return true
	}
	text := strings.ToLower(req.Message)
	return (strings.Contains(text, "rollback") || strings.Contains(req.Message, "回滚") || strings.Contains(req.Message, "撤销")) &&
		(strings.Contains(text, "spal reference eq") || strings.Contains(req.Message, "SPAL 参考"))
}

func (s *Server) planSPALReferenceEQRollback(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string) ChatResponse {
	source, ok := s.latestSPALReferenceEQRollbackSource(conversationID, firstStringFromMap(req.Context, "rollback_session_id", "spal_rollback_session_id"))
	if !ok || source.FrozenPlan == nil {
		return ChatResponse{
			ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
			Reply:    "没有找到可回滚的 SPAL Reference EQ 已执行 Receipt。只有已完成结构应用、且当前实例仍可验证的单一语义动作才能创建回滚 Proposal。",
			Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingClarification),
			WorkflowData: map[string]any{"capability_id": spalReferenceEQTestCapabilityID, "canary_stage": "rollback_source_missing"},
		}
	}
	originalAction := source.FrozenPlan.ActionSet.Actions[0]
	original, err := spal.ManifestFromAction(originalAction)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "读取原始 SPAL Manifest 失败："+err.Error())
	}
	resolved, err := s.resolveVPSStaticEQProviderForFrozen(ctx, original.Binding.Instance, source.FrozenPlan.ProjectCut.ProjectUUID)
	if err != nil {
		return vpsSPALProviderResponse(conversationID, goal, original.Instruction.TargetRef, err, nil)
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() || capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前工程无法建立强 Project Cut；没有创建 SPAL 回滚 Proposal。")
	}
	if currentProjectUUID := spalReferenceEQProjectUUID(state); currentProjectUUID == "" || currentProjectUUID != source.FrozenPlan.ProjectCut.ProjectUUID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前工程与原 SPAL Receipt 不属于同一 project_uuid；没有创建回滚 Proposal。")
	}
	cut, err := s.buildVPSStaticEQCut(state, resolved, req)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 SPAL 回滚 Project Cut："+err.Error())
	}
	current, err := (executionports.SPALVSPInspector{Client: s.kernel}).CaptureSPALPreimage(ctx, original.Binding, original.Preimage)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法读取回滚前的当前参数："+err.Error())
	}
	rollback, err := spal.NewRollbackManifest(original, current)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL 回滚被安全检查拒绝："+err.Error())
	}
	rollbackGoal := "Rollback SPAL Reference EQ test " + source.ID
	newSession, err := s.orchestrationRuntime.StartSPALReferenceEQTestChatSession(sessionID, conversationID, cut.ProjectUUID, rollbackGoal, orchestration.InteractionPropose)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 SPAL 回滚 Planning Session："+err.Error())
	}
	proposal, actionSet, err := rollback.FreezeProposal(spalReferenceEQTestCapabilityID, "v0", cut, 1)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法冻结 SPAL 回滚 Proposal："+err.Error())
	}
	l2Ready, l2Note := s.spalReferenceEQL2Availability(ctx)
	proposal.Presentation = spalReferenceEQProposalPresentation(proposal, actionSet, rollback.Instruction, l2Ready, l2Note)
	bundle := orchestration.ContextBundle{
		ID: "spal_rollback_context_" + sessionID, CapabilityID: spalReferenceEQTestCapabilityID, ProjectCutHash: cut.Hash,
		ArtifactRefs: []string{"spal.rollback_manifest:" + rollback.ID, "spal.original_manifest:" + original.ID},
		EvidenceRefs: appendUniqueSPALRefs(rollback.EvidenceRefs, resolved.EvidenceRefs...),
		Disclosure:   "A separately confirmable rollback to the original frozen physical preimage is ready.",
	}
	updated, err := s.orchestrationRuntime.AttachFrozenPlan(newSession.ID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: bundle.ID,
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法保存 SPAL 回滚 Frozen Proposal："+err.Error())
	}
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(newSession.ID, bundle, capabilityCanaryToolSchemas(s), []orchestration.ContextEntry{{
		ID: firstNonEmpty(goal.RunID, "rollback_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100,
	}}, orchestration.DefaultContextWindowBudget())
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "SPAL 回滚 Context Envelope 无效："+err.Error())
	}
	return spalReferenceEQRollbackProposalResponse(conversationID, goal, updated, bundle, envelope, source.ID, l2Ready, l2Note)
}

func (s *Server) latestSPALReferenceEQRollbackSource(conversationID, requestedID string) (orchestration.PlanningSession, bool) {
	if s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil {
		return orchestration.PlanningSession{}, false
	}
	var candidates []orchestration.PlanningSession
	for _, session := range s.orchestrationRuntime.Store.List() {
		if session.Invocation.CapabilityID != spalReferenceEQTestCapabilityID || !sessionBelongsToConversation(session, conversationID) {
			continue
		}
		if requestedID != "" && session.ID != requestedID {
			continue
		}
		if spalReferenceEQRollbackEligible(session) {
			candidates = append(candidates, session)
		}
	}
	if len(candidates) == 0 {
		return orchestration.PlanningSession{}, false
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt) })
	return candidates[0], true
}

func spalReferenceEQRollbackEligible(session orchestration.PlanningSession) bool {
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

func spalReferenceEQRollbackProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, bundle orchestration.ContextBundle, envelope orchestration.ContextEnvelope, sourceID string, l2Ready bool, l2Note string) ChatResponse {
	presentation := session.ActiveProposal.Presentation
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: renderProposalConversation(presentation), NeedsConfirmation: true, PlanID: session.ActiveProposal.ID,
		Preview: firstNonEmpty(presentationConclusion(presentation), session.ActiveProposal.Summary), ProposalPresentation: presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
			"proposal_id": session.ActiveProposal.ID, "proposal_revision": session.ActiveProposal.Revision,
			"action_set_hash": session.ActiveProposal.ActionSetHash, "project_cut_hash": session.ActiveProposal.ProjectCutHash,
			"context_bundle_id": bundle.ID, "canary_stage": "rollback_proposal", "approval_mode": "conversational",
			"proposal_presentation": presentation, "context_budget": capabilityCanaryContextBudget(envelope),
			"rollback_of_session_id": sourceID, "semantic_action": spalReferenceEQSemanticAction(session.FrozenPlan.ActionSet),
			"signal_probe_available": l2Ready, "signal_probe_note": l2Note,
		},
	}
}
