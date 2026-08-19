package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	c2DynamicPluginLoadBatchCommand = "c2.dynamic_plugin_load_batch.governed"
	c2DynamicParameterBatchWorkflow = "c2_dynamic_parameter_batch"
	c2LeafSurfaceRetryDelay         = 750 * time.Millisecond
	c2LeafSurfaceMaxAttempts        = 4
)

type c2ResolvedTarget struct {
	Hypothesis          dynamiccontrol.TargetHypothesis `json:"hypothesis"`
	PluginID            string                          `json:"plugin_id"`
	PluginName          string                          `json:"plugin_name,omitempty"`
	PCAAdmissionReceipt map[string]any                  `json:"pca_admission_receipt,omitempty"`
}

type c2BatchLeaf struct {
	Family     string         `json:"family"`
	TrackID    string         `json:"track_id"`
	PluginID   string         `json:"plugin_id"`
	Ticket     map[string]any `json:"ticket"`
	RequestCtx map[string]any `json:"request_context"`
}

// handleDynamicControlRuntime is intentionally a project-batch entry point.
// The legacy single-target runtime is retained only for old persisted sessions
// and is never selected for a new C2 capability invocation.
func (s *Server) handleDynamicControlRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	return s.handleC2ProjectBatchRuntime(ctx, conversationID, req, goal)
}

// handleC2ProjectBatchRuntime is the project-capability entry.  C2 owns the
// whole treatment batch; it never delegates a selected target into an
// independent ordinary-agent confirmation.
func (s *Server) handleC2ProjectBatchRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C2 Project-aware Runtime is unavailable.")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法取得当前工程 snapshot，C2 未启动："+canaryErrorText(err))
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, dynamicControlCapabilityID)
	}
	if current, exists := s.orchestrationRuntime.Store.Load(sessionID); exists && current.ActiveProposal != nil && current.FrozenPlan != nil {
		if !expectedProposalMatches(req.Context, current.ActiveProposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "C2 confirmation is bound to an expired Proposal; no project mutation was authorized.")
		}
		if len(current.FrozenPlan.ActionSet.Actions) == 1 && current.FrozenPlan.ActionSet.Actions[0].Command == c2DynamicPluginLoadBatchCommand {
			if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok && decision.Kind == orchestration.ApprovalApprove {
				return s.authorizeC2DynamicPluginLoadBatch(ctx, conversationID, req, goal, current, state)
			}
			return c2BatchProposalResponse(conversationID, goal, current)
		}
	}
	referral, referralErr := c2ReferralFromContext(req.Context)
	if referralErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "invalid_referral", referralErr)
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: []string{"vsp.snapshot:" + state.SnapshotHash}, ContractVersions: []string{"capability:" + dynamicControlCapabilityID, "treatment:" + dynamiccontrol.ProjectPlanSchema}})
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "project_cut_unavailable", err)
	}
	if _, exists := s.orchestrationRuntime.Store.Load(sessionID); !exists {
		if _, err := s.orchestrationRuntime.StartC2ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context)); err != nil {
			return c2BoundaryResponse(conversationID, goal, sessionID, "session_create_failed", err)
		}
	}
	// C2 is a project-level capability.  A normal UI selection is presentation
	// state, not an execution scope.  Only an explicit internal C2 target may
	// narrow observation.
	treatment, discovery, err := s.c2DiscoverProjectTreatment(ctx, conversationID, req.Message, state, c2ExplicitProjectTargetID(req.Context), c2FamilyFromContext(req.Context))
	if err != nil {
		var pending *c2ObservationPendingError
		if errors.As(err, &pending) {
			return c2ObservationPendingResponse(conversationID, goal, sessionID, pending)
		}
		return c2BoundaryResponse(conversationID, goal, sessionID, "project_treatment_discovery_failed", err)
	}
	if treatment.Status == dynamiccontrol.HypothesisNoAction {
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
		response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C2 已完成全工程动态观察；没有发现需要处理的动态目标，没有修改工程。", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"capability_id": dynamicControlCapabilityID, "stage": "no_action_needed", "project_treatment": treatment, "discovery": discovery, "mutation_performed": false}}
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	resolved, missing, err := s.c2ResolveProjectTargets(ctx, state, treatment)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_resolution_failed", err)
	}
	if len(missing) > 0 {
		return s.c2CreateDynamicLoadBatchProposal(ctx, conversationID, req, goal, sessionID, cut, treatment, discovery, resolved, missing, referral)
	}
	return s.c2CreateParameterBatchInteraction(ctx, conversationID, req, goal, sessionID, cut, treatment, discovery, resolved, referral)
}

func c2ExplicitProjectTargetID(requestContext map[string]any) string {
	// selected_track_id and selected_plugin_track_id are UI presentation state;
	// C2 must not let either one narrow its fixed project-level observation.
	return firstStringFromMap(requestContext, "c2_target_track_id")
}

func (s *Server) c2ResolveProjectTargets(ctx context.Context, state *kernel.VSPStateResult, treatment dynamiccontrol.ProjectTreatment) ([]c2ResolvedTarget, []dynamiccontrol.TargetHypothesis, error) {
	registry, err := processorregistry.Default()
	if err != nil {
		return nil, nil, err
	}
	resolved, missing := []c2ResolvedTarget{}, []dynamiccontrol.TargetHypothesis{}
	for _, target := range treatment.Targets {
		coverage, err := registry.PCARequiredCoverage(target.Intent.Family, target.Intent.RequiredCoverage)
		if err != nil {
			return nil, nil, err
		}
		options := s.c2PCAEligibleCandidates(ctx, s.c2LoadedCandidates(ctx, state, target.TrackID, target.Intent.Family), target.Intent.Family, coverage)
		if len(options) == 0 {
			missing = append(missing, target)
			continue
		}
		sort.Slice(options, func(i, j int) bool { return options[i].PluginID < options[j].PluginID })
		// Multiple PCA-admitted instances are equivalent only at the resource
		// boundary.  Keep the deterministic first exact instance in the frozen
		// batch; acoustic target selection has already happened independently.
		resolved = append(resolved, c2ResolvedTarget{Hypothesis: target, PluginID: options[0].PluginID, PluginName: options[0].PluginName})
	}
	return resolved, missing, nil
}

func (s *Server) c2CreateDynamicLoadBatchProposal(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, cut orchestration.ProjectCut, treatment dynamiccontrol.ProjectTreatment, discovery map[string]any, resolved []c2ResolvedTarget, missing []dynamiccontrol.TargetHypothesis, referral *dynamiccontrol.Referral) ChatResponse {
	loads := make([]map[string]any, 0, len(missing))
	unavailable := make([]map[string]any, 0)
	executableTargets := make([]dynamiccontrol.TargetHypothesis, 0, len(treatment.Targets))
	missingByTrack := map[string]dynamiccontrol.TargetHypothesis{}
	for _, target := range missing {
		missingByTrack[target.TrackID] = target
	}
	for _, target := range treatment.Targets {
		if _, needsLoad := missingByTrack[target.TrackID]; !needsLoad {
			executableTargets = append(executableTargets, target)
		}
	}
	for _, target := range missing {
		processorType := c2ProcessorTypeForFamily(target.Intent.Family)
		if processorType == "" {
			return c2BoundaryResponse(conversationID, goal, sessionID, "unsupported_dynamic_family", fmt.Errorf("C2 family %s has no load adapter", target.Intent.Family))
		}
		candidates, catalogErr := s.localPluginRecommendationCandidates(ctx, processorType)
		if catalogErr != nil {
			return c2BoundaryResponse(conversationID, goal, sessionID, "dynamic_plugin_catalog_unavailable", catalogErr)
		}
		registry, registryErr := processorregistry.Default()
		coverage := []processorattestation.Coverage(nil)
		coverageErr := registryErr
		if coverageErr == nil {
			coverage, coverageErr = registry.PCARequiredCoverage(target.Intent.Family, target.Intent.RequiredCoverage)
		}
		if coverageErr == nil {
			candidates, catalogErr = c2CoverageAttestedPluginRecommendationCandidates(candidates, processorType, coverage)
		}
		candidateErr := firstNonNilError(coverageErr, catalogErr)
		if candidateErr != nil || len(candidates) == 0 {
			unavailable = append(unavailable, map[string]any{"track_id": target.TrackID, "processor_family": target.Intent.Family, "required_coverage": target.Intent.RequiredCoverage, "reason": firstNonEmpty(errorText(candidateErr), "no exact PCA-covered local candidate")})
			continue
		}
		candidate, ok := c2StablePCAAdmittedCandidate(candidates)
		if !ok {
			return c2BoundaryResponse(conversationID, goal, sessionID, "no_pca_admitted_dynamic_plugin_candidate", fmt.Errorf("no exact PCA-admitted %s candidate", target.Intent.Family))
		}
		executableTargets = append(executableTargets, target)
		loads = append(loads, map[string]any{"track_id": target.TrackID, "processor_family": target.Intent.Family, "required_coverage": target.Intent.RequiredCoverage, "plugin_path": candidate.PluginPath, "plugin_identifier": candidate.Identifier, "plugin_name": candidate.Name, "candidate": candidate, "pca_authorization": map[string]any{"subject_key": candidate.SubjectKey, "binary_fingerprint": candidate.BinaryFingerprint, "attestation_id": candidate.AttestationID}, "hypothesis": target})
	}
	if len(unavailable) > 0 {
		discovery = cloneContext(discovery)
		discovery["unavailable_targets"] = unavailable
		treatment.Targets = executableTargets
	}
	if len(treatment.Targets) == 0 {
		return c2BoundaryResponse(conversationID, goal, sessionID, "no_pca_coverage_for_project_treatment", fmt.Errorf("no C2 target has a locally PCA-covered exact processor candidate"))
	}
	if len(loads) == 0 {
		// An unavailable load target must not block already PCA-qualified live
		// instances from receiving their independently frozen C2 treatment.
		return s.c2CreateParameterBatchInteraction(ctx, conversationID, req, goal, sessionID, cut, treatment, discovery, resolved, referral)
	}
	action := orchestration.Action{ID: "c2_dynamic_plugin_load_batch", Command: c2DynamicPluginLoadBatchCommand, TargetRef: "project:c2_dynamic_targets", BeforeFingerprint: "c2-load-batch:" + semanticEQHash(loads), Args: map[string]any{"loads": loads, "project_treatment": treatment, "discovery": discovery, "resolved": resolved, "referral": referral}, Compensatable: true, IdempotencyClass: "c2_dynamic_plugin_load_all_or_rollback"}
	actionSet := orchestration.ActionSet{ID: "actionset_c2_load_" + sanitizeCanaryID(sessionID), CapabilityID: dynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	actionSet.Hash = actionSet.ComputeHash()
	targetScope := []string{}
	for _, load := range loads {
		targetScope = append(targetScope, "track:"+firstStringFromMap(load, "track_id"))
	}
	proposal := orchestration.Proposal{ID: "proposal_" + actionSet.Hash[:16], Revision: 1, CapabilityID: dynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: "c2_load_" + actionSet.Hash[:16], ActionSetHash: actionSet.Hash, TargetScope: targetScope, Risk: "bounded_reversible", VerificationRef: "fine_mix.dynamic_control.plugin_load_batch.v1", Summary: fmt.Sprintf("Load PCA-admitted dynamic processors on %d C2 target tracks", len(loads)), CreatedAt: time.Now().UTC()}
	proposal.Presentation = c2DynamicLoadBatchPresentation(proposal, loads)
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_c2_load_" + sanitizeCanaryID(sessionID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "plugin_load_batch_persist_failed", err)
	}
	return c2BatchProposalResponse(conversationID, goal, session)
}

// c2StablePCAAdmittedCandidate binds an exact binary only after PCA has
// admitted it to the requested processor family. The C2 acoustic planner has
// already selected the target, so this must not become a second LLM-driven
// product recommendation step.
func c2StablePCAAdmittedCandidate(candidates []pluginRecommendationCandidate) (pluginRecommendationCandidate, bool) {
	if len(candidates) == 0 {
		return pluginRecommendationCandidate{}, false
	}
	ordered := append([]pluginRecommendationCandidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(ordered[i].Identifier + "\x00" + ordered[i].PluginPath))
		right := strings.ToLower(strings.TrimSpace(ordered[j].Identifier + "\x00" + ordered[j].PluginPath))
		return left < right
	})
	return ordered[0], true
}

func c2DynamicLoadBatchPresentation(proposal orchestration.Proposal, loads []map[string]any) *orchestration.ProposalPresentation {
	actions := make([]orchestration.ProposalActionPreview, 0, len(loads))
	for index, load := range loads {
		actions = append(actions, orchestration.ProposalActionPreview{ActionID: fmt.Sprintf("load_%d", index+1), TrackID: firstStringFromMap(load, "track_id"), Operation: "load_dynamic_plugin", Reason: "C2 target has no PCA-admitted live instance"})
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: "C2 全工程动态插件加载方案", Conclusion: fmt.Sprintf("将在 %d 个 C2 动态目标上批量加载 PCA 合格实例。", len(loads)), Recommendation: proposal.Summary, ActionCount: len(loads), Risk: proposal.Risk, Reversible: true, Actions: actions, Limitations: []string{"本次确认只授权批量加载与每实例 PCA 验证。", "任一加载或验证失败将删除本批所有新增实例。", "动态参数会在实例全部合格后组成单独的项目级 Proposal。"}, ApprovalPrompt: "是否确认本次 C2 项目级动态插件批量加载？"}
}

func c2BatchProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return c2BoundaryResponse(conversationID, goal, session.ID, "proposal_missing", fmt.Errorf("C2 batch proposal is unavailable"))
	}
	proposal := session.ActiveProposal
	presentation := proposal.Presentation
	c2 := map[string]any{
		"schema_version":               dynamiccontrol.ProjectPlanSchema,
		"session_id":                   session.ID,
		"project_cut_hash":             proposal.ProjectCutHash,
		"independent":                  true,
		"c1_referral_present":          c2FrozenPlanReferralPresent(session),
		"post_action_refresh_required": true,
	}
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: firstNonEmpty(presentation.Conclusion, proposal.Summary) + "\n\n" + presentation.ApprovalPrompt, NeedsConfirmation: true, PlanID: proposal.ID, Preview: presentation.Conclusion, ProposalPresentation: presentation, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: map[string]any{"session_id": session.ID, "capability_id": dynamicControlCapabilityID, "stage": "plugin_load_batch_confirmation", "proposal_id": proposal.ID, "proposal_revision": proposal.Revision, "action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash, "mutation_performed": false, "proposal_presentation": presentation, "c2": c2}}
}

func c2FrozenPlanReferralPresent(session orchestration.PlanningSession) bool {
	if session.FrozenPlan == nil || len(session.FrozenPlan.ActionSet.Actions) == 0 {
		return false
	}
	raw, ok := session.FrozenPlan.ActionSet.Actions[0].Args["referral"]
	if !ok || raw == nil {
		return false
	}
	encoded, err := json.Marshal(raw)
	if err != nil || string(encoded) == "null" {
		return false
	}
	var referral *dynamiccontrol.Referral
	return json.Unmarshal(encoded, &referral) == nil && referral != nil
}

func (s *Server) authorizeC2DynamicPluginLoadBatch(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, current *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil || session.ActiveProposal == nil || current == nil {
		return c2BoundaryResponse(conversationID, goal, session.ID, "load_batch_missing", fmt.Errorf("C2 load batch is unavailable"))
	}
	frozen := *session.FrozenPlan
	base, parseErr := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if parseErr != nil || capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || current.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || current.Revision != base || !s.orchestrationRuntime.HasDurableStore() {
		return c2BoundaryResponse(conversationID, goal, session.ID, "stale_project_cut", fmt.Errorf("C2 load batch project state changed"))
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return c2BoundaryResponse(conversationID, goal, session.ID, "load_batch_approval_missing", fmt.Errorf("explicit approval is required"))
	}
	if _, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision, ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash, Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision}); err != nil {
		return c2BoundaryResponse(conversationID, goal, session.ID, "load_batch_authorization_failed", err)
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, session.ID, frozen.ActionSet, frozen.ProjectCut, &c2DynamicPluginLoadBatchPort{server: s}, c2DynamicPluginLoadBatchVerifier{}, executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID})
	if executeErr != nil || executed.Execution == nil || len(executed.Execution.Receipts) != 1 || executed.Execution.Receipts[0].Status != "applied" {
		return c2BoundaryResponse(conversationID, goal, session.ID, "load_batch_execution_failed", firstNonNilError(executeErr, fmt.Errorf("C2 dynamic load batch failed")))
	}
	var treatment dynamiccontrol.ProjectTreatment
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["project_treatment"], &treatment)
	var discovery map[string]any
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["discovery"], &discovery)
	var resolved []c2ResolvedTarget
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["resolved"], &resolved)
	for _, row := range mapRowsValue(executed.Execution.Receipts[0].Details["loaded"]) {
		var hypothesis dynamiccontrol.TargetHypothesis
		_ = decodeAnyJSON(row["hypothesis"], &hypothesis)
		resolved = append(resolved, c2ResolvedTarget{
			Hypothesis:          hypothesis,
			PluginID:            firstStringFromMap(row, "plugin_id"),
			PluginName:          firstStringFromMap(row, "plugin_name"),
			PCAAdmissionReceipt: cloneContext(firstMapFromAny(row["pca_admission_receipt"])),
		})
	}
	newSessionID := s.nextCapabilitySessionID(conversationID, dynamicControlCapabilityID)
	if _, err := s.orchestrationRuntime.StartC2ChatSession(newSessionID, conversationID, frozen.ProjectCut.ProjectUUID, session.Goal, orchestration.InteractionPropose); err != nil {
		return c2BoundaryResponse(conversationID, goal, session.ID, "post_load_session_create_failed", err)
	}
	response := s.c2CreateParameterBatchInteraction(ctx, conversationID, req, goal, newSessionID, frozen.ProjectCut, treatment, discovery, resolved, nil)
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["plugin_load_mutation_performed"] = true
	response.WorkflowData["mutation_performed"] = true
	response.Reply = "C2 批量加载与每实例 PCA 验证已通过。\n\n" + response.Reply
	return response
}

// c2CreateParameterBatchInteraction materializes every already-bound leaf
// before showing a confirmation.  The existing family planners remain the
// source of truth for controller binding; their one-leaf interactions are
// consumed internally so C2 can expose one project-level confirmation.
func (s *Server) c2CreateParameterBatchInteraction(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, cut orchestration.ProjectCut, treatment dynamiccontrol.ProjectTreatment, discovery map[string]any, targets []c2ResolvedTarget, referral *dynamiccontrol.Referral) ChatResponse {
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return c2BoundaryResponse(conversationID, goal, sessionID, "ai_configuration_incomplete", firstNonNilError(err, fmt.Errorf("AI configuration is incomplete")))
	}
	if len(targets) != len(treatment.Targets) {
		return c2BoundaryResponse(conversationID, goal, sessionID, "unresolved_project_targets", fmt.Errorf("C2 has %d resolved instances for %d treatment targets", len(targets), len(treatment.Targets)))
	}
	leaves := make([]c2BatchLeaf, 0, len(targets))
	for _, target := range targets {
		requestContext := mergeContext(req.Context, map[string]any{
			"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "capability_session_id": sessionID,
			"c2_source_only_reversible": true, "c2_project_cut_hash": cut.Hash,
			"selected_plugin_track_id": target.Hypothesis.TrackID, "selected_plugin_id": target.PluginID,
			"c2_target_track_id": target.Hypothesis.TrackID, "c2_processor_family": target.Hypothesis.Intent.Family,
			"c2_semantic_processor_intent": mapFromJSONStruct(target.Hypothesis.Intent),
			"goal_id":                      goal.GoalID, "run_id": goal.RunID,
		})
		if len(target.PCAAdmissionReceipt) > 0 {
			// The rack node ID is only a live topology locator. Preserve the
			// server-issued binary receipt for both leaf planning and execution.
			requestContext["pca_admission_receipt"] = cloneContext(target.PCAAdmissionReceipt)
		}
		planned := s.c2PlanParameterLeaf(ctx, conversationID, req.Message, target, requestContext, cfg)
		if !planned.NeedsConfirmation || len(planned.InteractionRequests) != 1 {
			return c2BoundaryResponse(conversationID, goal, sessionID, "parameter_leaf_planning_failed", fmt.Errorf("C2 target %s/%s was not materialized: %s", target.Hypothesis.TrackID, target.PluginID, c2LeafPlanningFailureReason(planned)))
		}
		pending, ok := s.takePendingInteraction(planned.InteractionRequests[0].ID)
		if !ok {
			return c2BoundaryResponse(conversationID, goal, sessionID, "parameter_leaf_ticket_missing", fmt.Errorf("C2 target %s did not retain its materialized ticket", target.Hypothesis.TrackID))
		}
		leaf := c2BatchLeaf{Family: target.Hypothesis.Intent.Family, TrackID: target.Hypothesis.TrackID, PluginID: target.PluginID, RequestCtx: requestContext}
		leaf.Ticket = c2MaterializedLeafTicket(pending)
		if len(leaf.Ticket) == 0 {
			return c2BoundaryResponse(conversationID, goal, sessionID, "parameter_leaf_ticket_missing", fmt.Errorf("C2 target %s has an empty materialized ticket", target.Hypothesis.TrackID))
		}
		leaves = append(leaves, leaf)
	}
	if err := s.attachC2BatchFrozenPlan(sessionID, cut, treatment, leaves); err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "parameter_batch_persist_failed", err)
	}
	preview := make([]map[string]any, 0, len(leaves))
	for _, leaf := range leaves {
		preview = append(preview, map[string]any{"track_id": leaf.TrackID, "plugin_id": leaf.PluginID, "family": leaf.Family, "controls": leaf.Ticket["controls"]})
	}
	interaction := AgentInteractionRequest{ID: "interaction_" + randomID(), Kind: "confirmation", Type: c2DynamicParameterBatchWorkflow, Source: c2DynamicParameterBatchWorkflow, Workflow: c2DynamicParameterBatchWorkflow, Stage: "waiting_for_user", Title: "确认执行 C2 全工程动态处理", Body: fmt.Sprintf("将对 %d 个动态处理目标执行已冻结的 Typed Executor 控制。", len(leaves)), Status: "waiting_for_user", ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, PlanID: "c2_batch_" + sanitizeCanaryID(sessionID), Payload: map[string]any{"preview": preview}, Data: map[string]any{"session_id": sessionID, "project_treatment": treatment, "leaves": leaves, "request_context": cloneContext(req.Context), "referral": referral}, Actions: []AgentInteractionAction{{ID: "approve", Label: "确认执行全部", Style: "primary", Recommended: true}, {ID: "cancel", Label: "取消", Style: "secondary"}}}
	s.storePendingInteraction(interaction, interaction.Data)
	data := map[string]any{"capability_id": dynamicControlCapabilityID, "session_id": sessionID, "stage": "parameter_batch_confirmation", "project_treatment": treatment, "target_count": len(leaves), "mutation_performed": false, "c2": map[string]any{"schema_version": dynamiccontrol.ProjectPlanSchema, "session_id": sessionID, "project_cut_hash": cut.Hash, "independent": true, "c1_referral_present": referral != nil, "post_action_refresh_required": true}}
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: interaction.Body, NeedsConfirmation: true, PlanID: interaction.PlanID, Preview: interaction.Body, Workflow: c2DynamicParameterBatchWorkflow, WorkflowData: data, InteractionRequests: []AgentInteractionRequest{interaction}, GoalStatus: string(agentruntime.StatusWaitingConfirmation)}
}

func c2LeafPlanningFailureReason(response ChatResponse) string {
	detail := firstNonEmptyText(mapValue(response.WorkflowData["execution"]), "message")
	if detail == "" {
		detail = firstNonEmpty(response.Error, response.StopReason)
	}
	if response.StopReason != "" && detail != response.StopReason {
		return response.StopReason + ": " + detail
	}
	return firstNonEmpty(detail, "parameter leaf did not return a materialized confirmation")
}

// c2PlanParameterLeaf permits a bounded re-read only while a just-loaded
// processor's topology/control surface is still settling. It never retries a
// model decision or a write, and retains the frozen C2 intent on every read.
func (s *Server) c2PlanParameterLeaf(ctx context.Context, conversationID, requestText string, target c2ResolvedTarget, requestContext map[string]any, cfg config.EngineConfig) ChatResponse {
	var planned ChatResponse
	transportRetryUsed := false
	for attempt := 0; attempt < c2LeafSurfaceMaxAttempts; attempt++ {
		if target.Hypothesis.Intent.Family == "broadband_compressor" {
			planned = s.planBoundSemanticCompressor(ctx, conversationID, target.Hypothesis.ListeningGoal, requestContext, cfg)
		} else {
			observation := s.c2TargetObservation(ctx, conversationID, target.Hypothesis.TrackID, requestText)
			planned = s.planBoundSemanticDynamic(ctx, conversationID, target.Hypothesis.ListeningGoal, requestContext, target.Hypothesis.Intent.Family, semanticDynamicObservationFromRecent(observation), cfg)
		}
		retry := c2LeafSurfaceMayBeUnready(planned)
		if c2LeafPlanningTransportMayRetry(planned) && !transportRetryUsed {
			// No model decision was returned. Retry this read-only leaf plan once;
			// completed model decisions and every write remain single-shot.
			transportRetryUsed = true
			retry = true
		}
		if planned.NeedsConfirmation || !retry || attempt+1 == c2LeafSurfaceMaxAttempts {
			return planned
		}
		timer := time.NewTimer(c2LeafSurfaceRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ChatResponse{Error: ctx.Err().Error(), StopReason: "parameter_leaf_surface_wait_cancelled"}
		case <-timer.C:
		}
	}
	return planned
}

func c2LeafPlanningTransportMayRetry(response ChatResponse) bool {
	if strings.TrimSpace(response.Error) == "" {
		return false
	}
	return c2TransientProjectTreatmentError(errors.New(response.Error))
}

func c2LeafSurfaceMayBeUnready(response ChatResponse) bool {
	text := strings.ToLower(firstNonEmpty(response.Error, response.StopReason))
	return strings.Contains(text, "selected_axes_unreachable") ||
		strings.Contains(text, "required_coverage_unreachable") ||
		strings.Contains(text, "topology_generation_unavailable")
}

// Leaf planners keep their tickets server-side. The C2 batch consumes those
// tickets immediately, but durable workspace restoration can decode a ticket
// as either a map or a concrete value. Normalize through JSON at this boundary
// instead of relying on a single in-memory map representation.
func c2MaterializedLeafTicket(interaction PendingInteraction) map[string]any {
	for _, container := range []map[string]any{interaction.Data, interaction.Payload} {
		if ticket := firstMapFromAny(container["ticket"]); len(ticket) > 0 {
			return ticket
		}
		if ticket := mapFromJSONStruct(container["ticket"]); len(ticket) > 0 {
			return ticket
		}
	}
	return nil
}

func (s *Server) attachC2BatchFrozenPlan(sessionID string, cut orchestration.ProjectCut, treatment dynamiccontrol.ProjectTreatment, leaves []c2BatchLeaf) error {
	if s == nil || s.orchestrationRuntime == nil || len(leaves) == 0 {
		return fmt.Errorf("C2 parameter batch is unavailable")
	}
	targetScope := make([]string, 0, len(leaves))
	for _, leaf := range leaves {
		targetScope = append(targetScope, "plugin:"+leaf.TrackID+":"+leaf.PluginID)
	}
	action := orchestration.Action{ID: "c2_dynamic_parameter_batch", Command: "c2.dynamic_parameter_batch.governed", TargetRef: "project:c2_dynamic_targets", Args: map[string]any{"project_treatment": treatment, "leaves": leaves}, Compensatable: true, IdempotencyClass: "c2_dynamic_parameter_all_or_rollback"}
	set := orchestration.ActionSet{ID: "actionset_c2_dynamic_" + sanitizeCanaryID(sessionID), CapabilityID: dynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	set.Hash = set.ComputeHash()
	proposal := orchestration.Proposal{ID: "proposal_" + set.Hash[:16], Revision: 1, CapabilityID: dynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: "c2_dynamic_" + set.Hash[:16], ActionSetHash: set.Hash, TargetScope: targetScope, Risk: "bounded_reversible", VerificationRef: "fine_mix.dynamic_control.parameter_batch.v1", Summary: fmt.Sprintf("Apply %d C2 dynamic targets as one all-or-rollback batch", len(leaves)), CreatedAt: time.Now().UTC()}
	_, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: set, ProjectCut: cut, ContextBundleID: "bundle_c2_dynamic_" + sanitizeCanaryID(sessionID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	return err
}

func (s *Server) continueC2DynamicParameterBatchInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	clean := strings.ToLower(strings.TrimSpace(decision))
	if clean == "cancel" || clean == "deny" || clean == "reject" {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "已取消 C2 全工程动态处理；没有写入参数。", Workflow: c2DynamicParameterBatchWorkflow, WorkflowData: map[string]any{"capability_id": dynamicControlCapabilityID, "stage": "parameter_batch_cancelled", "mutation_performed": false}, GoalStatus: string(agentruntime.StatusCancelled)}
	}
	if !isApprovalDecision(clean) {
		s.restorePendingInteraction(interaction)
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "请明确确认或取消整批 C2 动态处理。", Workflow: c2DynamicParameterBatchWorkflow, Error: "explicit approval or cancellation required", GoalStatus: string(agentruntime.StatusFailed)}
	}
	var leaves []c2BatchLeaf
	data, _ := json.Marshal(interaction.Data["leaves"])
	if err := json.Unmarshal(data, &leaves); err != nil || len(leaves) == 0 {
		return c2BoundaryResponse(interaction.ConversationID, agentruntime.Goal{GoalID: interaction.GoalID, RunID: interaction.RunID}, firstStringFromMap(interaction.Data, "session_id"), "invalid_parameter_batch", fmt.Errorf("C2 parameter batch tickets are unavailable"))
	}
	applied := make([]c2BatchLeaf, 0, len(leaves))
	leafReceipts := make([]map[string]any, 0, len(leaves))
	for _, leaf := range leaves {
		leafInteraction := PendingInteraction{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, RequestContext: leaf.RequestCtx}
		var response ChatResponse
		if leaf.Family == "broadband_compressor" {
			ticket, err := compressorExecutionTicketFromMap(leaf.Ticket)
			if err != nil {
				return s.c2ParameterBatchFailure(ctx, interaction, applied, leafReceipts, "invalid_compressor_ticket", err)
			}
			response = s.executeSemanticCompressorTicket(ctx, leafInteraction, ticket)
		} else {
			var ticket semanticDynamicTicket
			ticketData, _ := json.Marshal(leaf.Ticket)
			if err := json.Unmarshal(ticketData, &ticket); err != nil || ticket.SchemaVersion != semanticDynamicSchema {
				return s.c2ParameterBatchFailure(ctx, interaction, applied, leafReceipts, "invalid_dynamic_ticket", fmt.Errorf("C2 dynamic ticket is invalid"))
			}
			response = s.executeSemanticDynamicTicket(ctx, leafInteraction, ticket)
		}
		if !boolValue(response.WorkflowData["mutation_performed"]) {
			return s.c2ParameterBatchFailure(ctx, interaction, applied, leafReceipts, "typed_leaf_failed", fmt.Errorf("typed executor failed for %s/%s: %s", leaf.TrackID, leaf.PluginID, firstNonEmpty(response.Error, response.StopReason)))
		}
		receipt := firstMapFromAny(response.WorkflowData["execution_receipt"])
		if len(receipt) == 0 {
			return s.c2ParameterBatchFailure(ctx, interaction, applied, leafReceipts, "typed_leaf_receipt_missing", fmt.Errorf("typed executor omitted a receipt for %s/%s", leaf.TrackID, leaf.PluginID))
		}
		leafReceipts = append(leafReceipts, receipt)
		applied = append(applied, leaf)
	}
	if err := validateC2BatchLeafReceipts(leaves, leafReceipts); err != nil {
		return s.c2ParameterBatchFailure(ctx, interaction, applied, leafReceipts, "typed_leaf_receipt_invalid", err)
	}
	return s.completeC2DynamicParameterBatch(ctx, interaction, leaves, leafReceipts)
}

// validateC2BatchLeafReceipts keeps the project batch bound to the exact
// target order that was confirmed. A receipt merely being present is not
// enough: a stale or cross-target receipt would make the all-or-rollback
// evidence ambiguous even when the controller reported success.
func validateC2BatchLeafReceipts(leaves []c2BatchLeaf, receipts []map[string]any) error {
	if len(leaves) == 0 || len(leaves) != len(receipts) {
		return fmt.Errorf("C2 batch receipt count %d does not match leaf count %d", len(receipts), len(leaves))
	}
	for index, leaf := range leaves {
		receipt := receipts[index]
		if strings.ToLower(firstStringFromMap(receipt, "status")) != "executed" {
			return fmt.Errorf("C2 leaf %d receipt is not executed", index+1)
		}
		if got := firstStringFromMap(receipt, "track_id"); got != leaf.TrackID {
			return fmt.Errorf("C2 leaf %d receipt track %q does not match %q", index+1, got, leaf.TrackID)
		}
		if got := firstStringFromMap(receipt, "plugin_id"); got != leaf.PluginID {
			return fmt.Errorf("C2 leaf %d receipt plugin %q does not match %q", index+1, got, leaf.PluginID)
		}
		controller := firstMapFromAny(receipt["controller_result"])
		if strings.ToLower(firstStringFromMap(controller, "status")) != "exact" || firstStringFromMap(controller, "restore_ref") == "" {
			return fmt.Errorf("C2 leaf %d lacks exact controller readback and restore_ref", index+1)
		}
		if audit := firstMapFromAny(receipt["parameter_audit"]); len(audit) > 0 && strings.ToLower(firstStringFromMap(audit, "status")) != "pass" {
			return fmt.Errorf("C2 leaf %d parameter audit did not pass", index+1)
		}
	}
	return nil
}

func (s *Server) c2ParameterBatchFailure(ctx context.Context, interaction PendingInteraction, applied []c2BatchLeaf, receipts []map[string]any, code string, cause error) ChatResponse {
	rollback, rollbackErr := s.c2RollbackParameterLeaves(ctx, applied, receipts)
	data := map[string]any{"capability_id": dynamicControlCapabilityID, "stage": "parameter_batch_failed", "mutation_performed": false, "leaf_receipts": receipts, "rollback": rollback, "error": cause.Error()}
	if rollbackErr != nil {
		data["rollback_error"] = rollbackErr.Error()
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "C2 参数批执行失败，且跨叶回滚未完全成功；已停止。", Workflow: c2DynamicParameterBatchWorkflow, WorkflowData: data, Error: fmt.Sprintf("%v; rollback: %v", cause, rollbackErr), GoalStatus: string(agentruntime.StatusFailed), StopReason: "c2_parameter_batch_failed_closed"}
	}
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "C2 参数批执行失败；已恢复此前已写入的动态控制。", Workflow: c2DynamicParameterBatchWorkflow, WorkflowData: data, Error: cause.Error(), GoalStatus: string(agentruntime.StatusFailed), StopReason: code}
}

func (s *Server) c2RollbackParameterLeaves(ctx context.Context, leaves []c2BatchLeaf, receipts []map[string]any) ([]map[string]any, error) {
	rows, failures := []map[string]any{}, []string{}
	for index := len(leaves) - 1; index >= 0; index-- {
		leaf := leaves[index]
		receipt := receipts[index]
		controller := firstMapFromAny(receipt["controller_result"])
		restoreRef := firstStringFromMap(controller, "restore_ref")
		status := "restored"
		var err error
		if restoreRef == "" {
			err = fmt.Errorf("restore_ref missing")
		} else if leaf.Family == "broadband_compressor" {
			_, err = s.applyPluginGrabberCompressorControls(ctx, map[string]any{"track_id": leaf.TrackID, "plugin_id": leaf.PluginID, "atomic": true, "restore_ref": restoreRef}, leaf.RequestCtx)
		} else if spec, ok := semanticDynamicSpecForFamily(leaf.Family); ok {
			_, err = s.applySemanticDynamicController(ctx, spec, map[string]any{"track_id": leaf.TrackID, "plugin_id": leaf.PluginID, "atomic": true, "restore_ref": restoreRef}, leaf.RequestCtx)
		} else {
			err = fmt.Errorf("unsupported family %s", leaf.Family)
		}
		if err != nil {
			status = "failed"
			failures = append(failures, leaf.TrackID+":"+err.Error())
		}
		rows = append(rows, map[string]any{"track_id": leaf.TrackID, "plugin_id": leaf.PluginID, "status": status})
	}
	if len(failures) > 0 {
		return rows, fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return rows, nil
}

func (s *Server) completeC2DynamicParameterBatch(ctx context.Context, interaction PendingInteraction, leaves []c2BatchLeaf, receipts []map[string]any) ChatResponse {
	change, authoritative := s.postActionProjectChange(ctx, "c2_parameter_batch_post_action")
	post := []map[string]any{}
	if authoritative {
		for _, leaf := range leaves {
			post = append(post, s.c2PostActionCCBObservation(ctx, interaction.ConversationID, firstStringFromMap(interaction.Data, "session_id"), leaf.TrackID, leaf.Family))
		}
	}
	verification := orchestration.VerificationResult{Status: "inconclusive", Structural: "pass", Acoustic: "inconclusive", UserAcceptance: "unknown", Summary: "C2 dynamic typed-executor batch completed; fresh per-target CCB observations were requested for review."}
	for _, row := range post {
		if observationID := firstStringFromMap(row, "observation_id"); observationID != "" {
			verification.EvidenceRefs = appendUniqueStrings(verification.EvidenceRefs, "ccb-observation:"+observationID)
		}
	}
	batchReceipt := orchestration.ActionReceipt{ActionID: "c2_dynamic_parameter_batch", Status: "applied", EffectivelyOnce: true, Details: map[string]any{"status": "executed", "leaf_receipts": receipts, "post_action_verification": post, "project_change": change}, EvidenceRefs: verification.EvidenceRefs}
	meta := map[string]any{"schema_version": dynamiccontrol.ProjectPlanSchema, "session_id": firstStringFromMap(interaction.Data, "session_id"), "independent": true, "post_action_verification": post, "project_change": change}
	response := ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID, Reply: "C2 全工程动态批处理已完成 Typed Executor 写入，并已刷新每个目标的 CCB 观察。", Workflow: c2DynamicParameterBatchWorkflow, WorkflowData: map[string]any{"capability_id": dynamicControlCapabilityID, "stage": "parameter_batch_executed", "mutation_performed": true, "execution_receipt": batchReceipt.Details, "c2": meta}, GoalStatus: string(agentruntime.StatusCompleted)}
	if sessionID := firstStringFromMap(interaction.Data, "session_id"); sessionID != "" && s.orchestrationRuntime != nil {
		if session, err := s.orchestrationRuntime.FinalizeExternalLeaf(sessionID, []orchestration.ActionReceipt{batchReceipt}, verification); err == nil {
			meta["terminal_receipt"] = map[string]any{"session_id": session.ID, "status": session.Status, "execution_status": session.Execution.Status, "verification": session.Execution.VerificationResult}
			attachMixboardDecisionProjection(&response, session)
		} else {
			meta["terminal_receipt"] = map[string]any{"status": "persistence_failed", "error": err.Error()}
		}
	}
	return response
}

// The port is intentionally the same all-or-rollback shape as B4's loader.
// It differs only in the post-load qualification predicate: every instance
// must pass the selected dynamic-family PCA coverage.
type c2DynamicPluginLoadBatchPort struct{ server *Server }

func (p *c2DynamicPluginLoadBatchPort) Preflight(ctx context.Context, set orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.server == nil || len(set.Actions) != 1 || set.Actions[0].Command != c2DynamicPluginLoadBatchCommand || !cut.IsExecutable() {
		return fmt.Errorf("invalid C2 dynamic load batch")
	}
	return nil
}
func (p *c2DynamicPluginLoadBatchPort) Apply(ctx context.Context, action orchestration.Action, key string) (orchestration.ActionReceipt, error) {
	return p.apply(ctx, action, key)
}
func (p *c2DynamicPluginLoadBatchPort) Reconcile(ctx context.Context, action orchestration.Action, key string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed_closed"}, fmt.Errorf("C2 load batch recovery requires explicit inspection")
}
func (p *c2DynamicPluginLoadBatchPort) apply(ctx context.Context, action orchestration.Action, key string) (orchestration.ActionReceipt, error) {
	loaded := []map[string]any{}
	for index, load := range mapRowsValue(action.Args["loads"]) {
		candidate := firstMapFromAny(load["candidate"])
		family := firstStringFromMap(load, "processor_family")
		registry, regErr := processorregistry.Default()
		coverage, coverageErr := registry.PCARequiredCoverage(family, contextStringSlice(load["required_coverage"]))
		if admissionErr := firstNonNilError(regErr, coverageErr); admissionErr != nil {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: admissionErr.Error(), Details: map[string]any{"loaded": loaded}}, admissionErr
		}
		trackID, pluginPath, identifier := firstStringFromMap(load, "track_id"), firstStringFromMap(load, "plugin_path"), firstStringFromMap(load, "plugin_identifier")
		auth := firstMapFromAny(load["pca_authorization"])
		pcaReceipt, receiptErr := c2PCAAdmissionReceiptForLoad(family, coverage, candidate, auth)
		if receiptErr != nil {
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: receiptErr.Error(), Details: map[string]any{"loaded": loaded}}, receiptErr
		}
		requestContext := harness.AuthorizeProcessorSelectionLoad(map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID}, trackID, pluginPath, identifier, processorattestation.EligibilityRequirement{ProcessorFamily: family, RequiredCoverage: coverage}, firstStringFromMap(auth, "subject_key"), firstStringFromMap(auth, "binary_fingerprint"), firstStringFromMap(auth, "attestation_id"))
		response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "rack.add_node", Args: map[string]any{"track_id": trackID, "plugin_path": pluginPath, "plugin_name": firstStringFromMap(load, "plugin_name"), "plugin_identifier": identifier, "plugin_kind": "Effect", "zone_id": "Z3", "x": 360.0, "y": 260.0}, Context: requestContext, Source: "c2_dynamic_plugin_load_batch", Confirmed: true, ToolCallID: fmt.Sprintf("%s:load:%d", key, index+1)})
		pluginID := firstNonEmpty(firstStringFromMap(response.Result, "plugin_id", "plugin_item_id", "item_id"), semanticEQFirstRecursive(semanticEQRecursiveText(response.Result), "plugin_id"))
		if err != nil || !strings.EqualFold(response.Status, "ok") || pluginID == "" {
			rollback, rollbackErr := p.rollback(ctx, loaded, key)
			failure := firstNonNilError(err, fmt.Errorf("load %d failed: %s", index+1, response.Error))
			if rollbackErr != nil {
				failure = fmt.Errorf("%w; rollback: %v", failure, rollbackErr)
			}
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: failure.Error(), Details: map[string]any{"loaded": loaded, "rollback": rollback}}, failure
		}
		admissionErr := error(nil)
		if admissionErr == nil {
			_, admissionErr = p.server.semanticLoadedInstancePCAAdmissionWithReceipt(ctx, firstStringFromMap(load, "track_id"), pluginID, semanticTreatmentPCAInput{Family: family, RequiredCoverage: coverage}, &pcaReceipt)
		}
		row := map[string]any{"track_id": firstStringFromMap(load, "track_id"), "plugin_id": pluginID, "plugin_name": firstStringFromMap(load, "plugin_name"), "hypothesis": load["hypothesis"], "candidate": candidate, "pca_admission_receipt": semanticPCAAdmissionReceiptMap(pcaReceipt)}
		if admissionErr != nil {
			loaded = append(loaded, row)
			rollback, rollbackErr := p.rollback(ctx, loaded, key)
			failure := admissionErr
			if rollbackErr != nil {
				failure = fmt.Errorf("%w; rollback: %v", failure, rollbackErr)
			}
			return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: failure.Error(), Details: map[string]any{"loaded": loaded, "rollback": rollback}}, failure
		}
		loaded = append(loaded, row)
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: map[string]any{"loaded": loaded, "structural_readback": "pass", "pca_admission": "pass", "rollback": map[string]any{"status": "available"}}, EvidenceRefs: []string{"c2.dynamic_plugin_load_batch:" + key}}, nil
}

// C2 selects an installed PCA subject before creating a rack node. Preserve
// that exact admission receipt across the load boundary; a numeric rack ID is
// not itself an installed-binary identity and must never replace this proof.
func c2PCAAdmissionReceiptForLoad(family string, coverage []processorattestation.Coverage, candidate, authorization map[string]any) (semanticPCAAdmissionReceipt, error) {
	receipt, found, err := semanticPCAAdmissionReceiptFromValue(map[string]any{
		"processor_family":   family,
		"subject_key":        firstStringFromMap(authorization, "subject_key"),
		"binary_fingerprint": firstStringFromMap(authorization, "binary_fingerprint"),
		"attestation_id":     firstStringFromMap(authorization, "attestation_id"),
	}, candidate)
	if err != nil {
		return semanticPCAAdmissionReceipt{}, err
	}
	if !found {
		return semanticPCAAdmissionReceipt{}, fmt.Errorf("C2 PCA admission receipt is missing")
	}
	if err := semanticValidatePCAAdmissionReceiptForInput(receipt, semanticTreatmentPCAInput{Family: family, RequiredCoverage: coverage}); err != nil {
		return semanticPCAAdmissionReceipt{}, err
	}
	return receipt, nil
}

// C2 may only bind a candidate after its exact promoted binary covers every
// semantic axis frozen in the project treatment. Family-level PCA admission is
// intentionally too weak for a controller write.
func c2CoverageAttestedPluginRecommendationCandidates(candidates []pluginRecommendationCandidate, processorType string, coverage []processorattestation.Coverage) ([]pluginRecommendationCandidate, error) {
	family := processorAttestationFamily(processorType)
	if family == "" {
		return nil, fmt.Errorf("processor family is unavailable")
	}
	return attestedPluginRecommendationCandidates(candidates, pluginControlRequirement{SchemaVersion: pluginControlRequirementSchema, ProcessorFamily: family, Coverage: coverage, Reason: "C2 frozen semantic processor intent"})
}
func (p *c2DynamicPluginLoadBatchPort) rollback(ctx context.Context, loaded []map[string]any, key string) (map[string]any, error) {
	rows := []map[string]any{}
	failures := []string{}
	for index := len(loaded) - 1; index >= 0; index-- {
		row := loaded[index]
		response, err := p.server.harness.Invoke(ctx, harness.InvokeRequest{Tool: "plugin.delete", Args: map[string]any{"track_id": firstStringFromMap(row, "track_id"), "plugin_item_id": firstStringFromMap(row, "plugin_id")}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID}, Source: "c2_dynamic_plugin_load_batch_rollback", Confirmed: true, ToolCallID: fmt.Sprintf("%s:rollback:%d", key, index+1)})
		status := "restored"
		if err != nil || !strings.EqualFold(response.Status, "ok") {
			status = "failed"
			failures = append(failures, firstNonEmpty(errorText(err), response.Error, response.Status))
		}
		rows = append(rows, map[string]any{"track_id": firstStringFromMap(row, "track_id"), "plugin_id": firstStringFromMap(row, "plugin_id"), "status": status})
	}
	if len(failures) > 0 {
		return map[string]any{"status": "failed", "instances": rows}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return map[string]any{"status": "restored", "instances": rows}, nil
}

type c2DynamicPluginLoadBatchVerifier struct{}

func (c2DynamicPluginLoadBatchVerifier) Verify(_ context.Context, _ orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	if len(receipts) != 1 || receipts[0].Status != "applied" || firstStringFromMap(receipts[0].Details, "pca_admission") != "pass" {
		return orchestration.VerificationResult{Status: "fail", Structural: "fail", Acoustic: "not_run", UserAcceptance: "unknown"}, fmt.Errorf("C2 dynamic load batch did not pass PCA")
	}
	return orchestration.VerificationResult{Status: "pass", Structural: "pass", Acoustic: "not_run", UserAcceptance: "unknown", Summary: "C2 batch loaded every requested instance and each passed PCA admission."}, nil
}
