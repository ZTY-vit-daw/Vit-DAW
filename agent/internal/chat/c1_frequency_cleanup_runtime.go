package chat

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/frequencycleanup"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
)

const (
	c1LoadBatchCommand = "c1.eq_plugin_load_batch.governed"
	c1EQBatchCommand   = "c1.semantic_eq_batch.governed"
)

var c1BatchSpec = projectEQBatchMutationSpec{CapabilityID: frequencyCleanupCapabilityID, LoadCommand: c1LoadBatchCommand, EQCommand: c1EQBatchCommand, SourcePrefix: "c1"}

type c1TargetResolution struct {
	Resolved  []c1EQTargetContext
	Missing   []frequencycleanup.TreatmentItem
	Ambiguous map[string][]semanticTreatmentInstance
}

func c1MutationRequested(text string, requestContext map[string]any) bool {
	if boolValue(requestContext["c1_execute"]) || boolValue(requestContext["execute"]) || strings.EqualFold(firstStringFromMap(requestContext, "c1_mode"), "apply") {
		return true
	}
	value := strings.ToLower(strings.TrimSpace(text))
	for _, token := range []string{"analysis only", "analyze only", "inspect only", "只分析", "仅分析", "查看", "解释"} {
		if strings.Contains(value, token) {
			return false
		}
	}
	for _, token := range []string{"apply", "execute", "treat", "clean up", "cleanup", "run c1", "complete c1", "执行", "应用", "处理", "清理", "完成c1", "做c1"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func (s *Server) handleFrequencyCleanupRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) (response ChatResponse) {
	runtimeStarted := time.Now()
	timing := map[string]any{}
	defer func() {
		timing["total_ms"] = time.Since(runtimeStarted).Milliseconds()
		attachC1Timing(&response, timing)
	}()
	if s == nil || s.kernel == nil || s.harness == nil || s.orchestrationRuntime == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 Project-aware Runtime is unavailable.")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 could not obtain the current project snapshot: "+firstNonEmpty(errorText(err), "snapshot unavailable"))
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, frequencyCleanupCapabilityID)
	}
	if session, exists := s.orchestrationRuntime.Store.Load(sessionID); exists && session.ActiveProposal != nil && session.FrozenPlan != nil {
		if !expectedProposalMatches(req.Context, session.ActiveProposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "C1 confirmation is bound to an expired Proposal; no mutation was authorized.")
		}
		if session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying {
			return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
		}
		if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok && decision.Kind == orchestration.ApprovalApprove {
			return s.authorizeC1Batch(ctx, conversationID, req, goal, session, state)
		}
		return c1ProposalResponse(conversationID, goal, session)
	}

	assemblyStarted := time.Now()
	input, dependencies, err := s.acquireFrequencyCleanupContext(ctx, sessionID, req, state, "")
	timing["existing_evidence_assembly_ms"] = time.Since(assemblyStarted).Milliseconds()
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 read-only context acquisition failed: "+err.Error())
	}
	decisionRefs, decisionRefsErr := mixboardDecisionContextForState(state.LegacyState, frequencyCleanupCapabilityID)
	buildCut := projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: dependencies, ArtifactRefs: mixboardDecisionArtifactRefs(decisionRefs), ContractVersions: []string{"capability:" + frequencyCleanupCapabilityID, "context:" + capabilitycontext.FrequencyCleanupContextPackSchema, "observation:frequency_relationship.v1"}}
	cut, err := projectcut.Build(buildCut)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 could not build ProjectCut: "+err.Error())
	}
	if _, exists := s.orchestrationRuntime.Store.Load(sessionID); !exists {
		if _, err = s.orchestrationRuntime.StartC1ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context)); err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "C1 PlanningSession creation failed: "+err.Error())
		}
	}
	ccbStarted := time.Now()
	planned, _, err := s.orchestrationRuntime.ShadowC1FromVSP(sessionID, state, buildCut, input)
	timing["ccb_ms"] = time.Since(ccbStarted).Milliseconds()
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 analysis failed: "+err.Error())
	}
	// C1 readiness only assembles already-materialized observation evidence.
	// Missing evidence is reported as a blocker and never triggers a render.
	timing["diagnosis_fill_requested_track_count"] = 0
	timing["diagnosis_fill_render_count"] = 0
	attachMixboardDecisionContext(&planned.Bundle, decisionRefs, decisionRefsErr)
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(sessionID, planned.Bundle, capabilityCanaryToolSchemas(s), []orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}}, orchestration.DefaultContextWindowBudget())
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 Context Envelope admission failed: "+err.Error())
	}
	if planned.Outcome.Kind == orchestration.OutcomeBlocked {
		reason := strings.Join(planned.Outcome.Blockers, ", ")
		return c1ReadinessBlockedResponse(conversationID, goal, sessionID, planned.Pack, reason)
	}
	if c1MutationRequested(req.Message, req.Context) {
		return s.buildC1ActionableResponse(ctx, conversationID, req, goal, sessionID, planned.Pack.Model(), state.Revision, timing)
	}
	cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
	response = ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: fmt.Sprintf("C1 analyzed all %d project tracks. This was read-only; no Proposal or mutation was created.", planned.Pack.AnalyzedTrackCount), Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": frequencyCleanupCapabilityID, "canary_stage": "analysis", "coverage": planned.Pack.Model().Coverage, "diagnosis_candidates": planned.Pack.Candidates, "readiness": planned.Pack.Readiness, "context_budget": capabilityCanaryContextBudget(envelope), "mutation_performed": false}}
	attachMixboardDecisionProjection(&response, cancelled)
	return response
}

func attachC1Timing(response *ChatResponse, timing map[string]any) {
	if response == nil || len(timing) == 0 {
		return
	}
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	copy := map[string]any{}
	for key, value := range timing {
		copy[key] = value
	}
	response.WorkflowData["timing"] = copy
}

func (s *Server) acquireFrequencyCleanupContext(ctx context.Context, sessionID string, req ChatRequest, state *kernel.VSPStateResult, previousObservation string) (capabilitycontext.FrequencyCleanupInput, []string, error) {
	_ = ctx
	_ = previousObservation
	assembled := mixboard.AssembleFrequencyContext(state.LegacyState, sessionID, req.Message, firstStringFromMap(req.Context, "feature_snapshot_path"))
	observationID := strings.TrimSpace(assembled.ObservationID)
	if observationID == "" {
		return capabilitycontext.FrequencyCleanupInput{}, nil, fmt.Errorf("existing-evidence CCB assembly omitted observation_id")
	}
	observe := map[string]any{"observation_id": observationID, "status": assembled.Status, "mom_projection": assembled.MOMProjection, "digest": assembled.Digest, "catalog": assembled.Catalog, "assembly": assembled.Assembly}
	tom := capabilitycontext.BuildProjectTOMProjection(state.LegacyState, observationID, sessionID)
	deps := []string{"vsp.snapshot:" + state.SnapshotHash, "MOM.frequency_relationship:" + observationID}
	if len(tom) > 0 {
		deps = append(deps, "TOM:project-state-derived:"+observationID)
	}
	return capabilitycontext.FrequencyCleanupInput{UserIntent: req.Message, ProjectState: cloneContext(state.LegacyState), MixObservation: cloneContext(observe), MOMProjection: cloneContext(assembled.MOMProjection), TOMProjection: tom, ContextSnapshot: map[string]any{"existing_evidence_assembly": cloneContext(assembled.Assembly)}, RequestContext: cloneContext(req.Context), ExecutionMemory: firstMapFromAny(req.Context["execution_memory"])}, deps, nil
}

func (s *Server) collectC1TargetPostFXBaseline(ctx context.Context, sessionID, goalText, phase string, state *kernel.VSPStateResult, treatment frequencycleanup.TreatmentPlan, model frequencycleanup.Model, forceFresh bool) (frequencycleanup.TargetPostFXBaseline, error) {
	trackIDs := frequencycleanup.TargetPreflightTrackIDs(treatment, model)
	if len(trackIDs) == 0 {
		return frequencycleanup.TargetPostFXBaseline{}, fmt.Errorf("C1 target preflight has no static-EQ target or relationship peer")
	}
	collected, err := s.harness.CollectL2RenderProbeBatch(ctx, harness.L2RenderProbeBatchRequest{
		SessionID: sessionID, GoalText: goalText, TrackIDs: trackIDs, TapPoint: "track_post_fader", ForceFresh: forceFresh,
	})
	projectID := firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid"), firstStringFromMap(firstMapFromAny(state.LegacyState["project"]), "project_uuid", "uuid", "id"), "current")
	baseline := frequencycleanup.BuildTargetPostFXBaseline(sessionID, phase, projectID, trackIDs, collected.Rows, time.Now().UTC())
	baseline.Collection = frequencycleanup.TargetBaselineCollection{ElapsedMS: collected.ElapsedMS, RenderedTrackIDs: append([]string(nil), collected.RenderedTrackIDs...), CacheHitTrackIDs: append([]string(nil), collected.CacheHitTrackIDs...)}
	if err != nil {
		return baseline, err
	}
	if !baseline.Readiness.CanProceed {
		return baseline, fmt.Errorf("target post-FX readiness blocked: %s", strings.Join(baseline.Readiness.BlockedBy, ", "))
	}
	return baseline, nil
}

func c1TargetPreflightBlockedResponse(conversationID string, goal agentruntime.Goal, sessionID string, treatment frequencycleanup.TreatmentPlan, baseline frequencycleanup.TargetPostFXBaseline, err error) ChatResponse {
	reason := firstNonEmpty(errorText(err), strings.Join(baseline.Readiness.BlockedBy, ", "), "target post-FX evidence unavailable")
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C1 classified the project, but exact target post-FX preflight is blocked; no Proposal or mutation was created: " + reason, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingContinue), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": frequencyCleanupCapabilityID, "canary_stage": "target_preflight_blocked", "treatment_plan": treatment, "target_post_fx_baseline": baseline, "mutation_performed": false}}
}

func c1ReadinessBlockedResponse(conversationID string, goal agentruntime.Goal, sessionID string, pack capabilitycontext.FrequencyCleanupPack, reason string) ChatResponse {
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C1 readiness is blocked; no plug-in was loaded and no parameter was changed: " + reason, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingContinue), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": frequencyCleanupCapabilityID, "canary_stage": "readiness_blocked", "diagnosis_readiness": pack.Readiness.Diagnosis, "mutation_readiness": pack.Readiness.Mutation, "coverage": pack.Model().Coverage, "mutation_performed": false}}
}

func (s *Server) buildC1ActionableResponse(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, model frequencycleanup.Model, projectRevision int64, timing map[string]any) (response ChatResponse) {
	if timing == nil {
		timing = map[string]any{}
	}
	defer func() { attachC1Timing(&response, timing) }()
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 project planning requires an available AI configuration; no Proposal was created: "+firstNonEmpty(errorText(err), "configuration incomplete"))
	}
	classificationStarted := time.Now()
	treatment, err := s.planC1Treatment(ctx, conversationID, req.Message, model, cfg)
	timing["llm_classification_ms"] = time.Since(classificationStarted).Milliseconds()
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 project treatment classification failed; no Proposal was created: "+err.Error())
	}
	staticItems := frequencycleanup.StaticEQItems(treatment)
	if len(staticItems) == 0 {
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
		response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C1 classified every project track and found no justified static-EQ action. Deferred and no-change items were reported without mutation.", Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": frequencyCleanupCapabilityID, "canary_stage": "no_static_eq_required", "coverage": model.Coverage, "treatment_plan": treatment, "mutation_performed": false}}
		attachC1DynamicReferral(&response, treatment, projectRevision)
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	resolutionStarted := time.Now()
	resolution := s.resolveC1EQTargets(ctx, staticItems, req.Context, nil)
	timing["target_resolution_ms"] = time.Since(resolutionStarted).Milliseconds()
	if len(resolution.Ambiguous) > 0 {
		response = c1PluginSelectionRequiredResponse(conversationID, goal, sessionID, treatment, resolution)
		attachC1DynamicReferral(&response, treatment, projectRevision)
		return response
	}
	if len(resolution.Missing) > 0 {
		response = s.createC1LoadProposal(ctx, conversationID, goal, sessionID, treatment, model, resolution.Missing, cfg)
		attachC1DynamicReferral(&response, treatment, projectRevision)
		return response
	}
	state, stateErr := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if stateErr != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 target preflight could not obtain the current project snapshot; no Proposal was created.")
	}
	preflightStarted := time.Now()
	baseline, baselineErr := s.collectC1TargetPostFXBaseline(ctx, sessionID, req.Message, "pre_parameter", state, treatment, model, false)
	timing["target_preflight_ms"] = time.Since(preflightStarted).Milliseconds()
	timing["target_preflight_render_count"] = len(baseline.Collection.RenderedTrackIDs)
	timing["target_preflight_cache_hit_count"] = len(baseline.Collection.CacheHitTrackIDs)
	if baselineErr != nil {
		return c1TargetPreflightBlockedResponse(conversationID, goal, sessionID, treatment, baseline, baselineErr)
	}
	proposalStarted := time.Now()
	response = s.createC1EQProposal(ctx, conversationID, goal, sessionID, treatment, model, baseline, resolution.Resolved, cfg)
	attachC1DynamicReferral(&response, treatment, projectRevision)
	timing["proposal_ms"] = time.Since(proposalStarted).Milliseconds()
	return response
}

func c1DynamicControlReferral(plan frequencycleanup.TreatmentPlan, projectRevision int64) *dynamiccontrol.Referral {
	if projectRevision <= 0 {
		return nil
	}
	referral := &dynamiccontrol.Referral{SchemaVersion: dynamiccontrol.ReferralSchema, SourceCapabilityID: frequencyCleanupCapabilityID, SourcePlanID: plan.PlanID, ProjectRevision: strconv.FormatInt(projectRevision, 10)}
	for _, item := range plan.Items {
		if item.Classification != frequencycleanup.TreatmentDeferredDynamic {
			continue
		}
		referral.Targets = append(referral.Targets, dynamiccontrol.ReferralTarget{TrackID: item.TrackID, Rationale: item.Rationale, Constraints: item.Constraints, EvidenceRefs: item.EvidenceRefs})
	}
	if len(referral.Targets) == 0 {
		return nil
	}
	referral.EvidenceRefs = append(referral.EvidenceRefs, plan.EvidenceRefs...)
	if err := referral.Validate(); err != nil {
		return nil
	}
	return referral
}

func attachC1DynamicReferral(response *ChatResponse, plan frequencycleanup.TreatmentPlan, projectRevision int64) {
	if response == nil {
		return
	}
	referral := c1DynamicControlReferral(plan, projectRevision)
	if referral == nil {
		return
	}
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["c2_referral"] = referral
}

func (s *Server) resolveC1EQTargets(ctx context.Context, items []frequencycleanup.TreatmentItem, requestContext map[string]any, preferred map[string]string) c1TargetResolution {
	result := c1TargetResolution{Ambiguous: map[string][]semanticTreatmentInstance{}}
	selections := map[string]string{}
	for key, value := range firstMapFromAny(requestContext["c1_eq_instance_selection"]) {
		selections[strings.TrimSpace(key)] = strings.TrimSpace(fmt.Sprint(value))
	}
	for _, item := range items {
		instances, _ := s.semanticTreatmentInstances(ctx, item.TrackID)
		qualified := semanticTreatmentQualifiedEQ(instances)
		selectedID := firstNonEmpty(preferred[item.TrackID], selections[item.TrackID])
		if selectedID != "" {
			if selected, ok := semanticTreatmentFindPlugin(qualified, selectedID); ok {
				result.Resolved = append(result.Resolved, c1EQTargetContext{Treatment: item, Instance: selected})
				continue
			}
		}
		switch len(qualified) {
		case 0:
			result.Missing = append(result.Missing, item)
		case 1:
			result.Resolved = append(result.Resolved, c1EQTargetContext{Treatment: item, Instance: qualified[0]})
		default:
			result.Ambiguous[item.TrackID] = qualified
		}
	}
	sort.SliceStable(result.Resolved, func(i, j int) bool { return result.Resolved[i].Treatment.Order < result.Resolved[j].Treatment.Order })
	return result
}

func c1PluginSelectionRequiredResponse(conversationID string, goal agentruntime.Goal, sessionID string, treatment frequencycleanup.TreatmentPlan, resolution c1TargetResolution) ChatResponse {
	rows := []map[string]any{}
	for trackID, instances := range resolution.Ambiguous {
		options := []map[string]any{}
		for _, instance := range instances {
			options = append(options, map[string]any{"plugin_id": instance.PluginID, "plugin_name": instance.PluginName, "qualification_status": instance.QualificationStatus})
		}
		rows = append(rows, map[string]any{"track_id": trackID, "instances": options})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return firstStringFromMap(rows[i], "track_id") < firstStringFromMap(rows[j], "track_id")
	})
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C1 found multiple qualified static-EQ instances on one or more tracks. Supply one exact plugin_id per listed track in c1_eq_instance_selection; no mutation has occurred.", Workflow: "plugin_selection_required", GoalStatus: string(agentruntime.StatusWaitingClarification), WorkflowData: map[string]any{"schema_version": "plugin_selection_required.v1", "capability_id": frequencyCleanupCapabilityID, "session_id": sessionID, "treatment_plan": treatment, "ambiguous_loaded_eq": rows, "missing_track_count": len(resolution.Missing), "mutation_performed": false}}
}

func (s *Server) createC1LoadProposal(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, treatment frequencycleanup.TreatmentPlan, model frequencycleanup.Model, missing []frequencycleanup.TreatmentItem, cfg config.EngineConfig) ChatResponse {
	candidates, err := s.localPluginRecommendationCandidates(ctx, "eq")
	candidates = genericStaticEQRecommendationCandidates(candidates)
	if err != nil || len(candidates) == 0 {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 needs a static EQ on some tracks, but no qualified local EQ candidate is available: "+firstNonEmpty(errorText(err), "no local EQ candidates"))
	}
	recommendation, err := s.planPluginRecommendation(ctx, conversationID, treatment.Summary, "eq", map[string]any{"c1_full_project": true}, nil, candidates, cfg)
	if err != nil || len(recommendation.Choices) == 0 {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 local EQ recommendation failed; no plug-in was loaded: "+firstNonEmpty(errorText(err), "empty recommendation"))
	}
	candidateByKey := map[string]pluginRecommendationCandidate{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	candidate, ok := candidateByKey[recommendation.Choices[0].CandidateKey]
	if !ok {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 recommendation did not bind an exact local candidate; no plug-in was loaded.")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 plug-in load Proposal could not obtain a strong project snapshot.")
	}
	loads := []map[string]any{}
	scope := []string{}
	for _, item := range missing {
		instances, _ := s.semanticTreatmentInstances(ctx, item.TrackID)
		before := []string{}
		for _, instance := range instances {
			before = append(before, instance.PluginID)
		}
		loads = append(loads, map[string]any{"track_id": item.TrackID, "track_name": item.TrackName, "plugin_path": candidate.PluginPath, "plugin_identifier": candidate.Identifier, "plugin_name": candidate.Name, "preexisting_plugin_ids": before})
		scope = append(scope, "track:"+item.TrackID)
	}
	decisionRefs, _ := mixboardDecisionContextForState(state.LegacyState, frequencyCleanupCapabilityID)
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: []string{"c1-treatment:" + treatment.PlanID, "plugin-candidate:" + candidate.PluginPath}, TargetFingerprints: scope, ArtifactRefs: appendUniqueStrings(treatment.EvidenceRefs, mixboardDecisionArtifactRefs(decisionRefs)...), ContractVersions: []string{"capability:" + frequencyCleanupCapabilityID, "plugin-load:rack_add_node", "rollback:delete_plugin"}})
	if err != nil || !cut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 plug-in load Proposal could not build a strong ProjectCut.")
	}
	action := orchestration.Action{ID: "c1_eq_plugin_load_batch", Command: c1LoadBatchCommand, TargetRef: "project:c1_missing_eq", BeforeFingerprint: "c1-load:" + semanticEQHash(loads), Args: map[string]any{"loads": loads, "treatment_plan": treatment, "diagnosis_model": model, "recommendation": recommendation, "candidate": candidate}, Compensatable: true, IdempotencyClass: "c1_plugin_load_all_or_rollback"}
	actionSet := orchestration.ActionSet{ID: "actionset_c1_eq_load_" + sanitizeCanaryID(treatment.PlanID), CapabilityID: frequencyCleanupCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "proposal_" + actionSet.Hash[:16], Revision: 1, CapabilityID: frequencyCleanupCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: "c1_load_" + actionSet.Hash[:16], ActionSetHash: actionSet.Hash, TargetScope: scope, Risk: "bounded_reversible", VerificationRef: "frequency_cleanup.plugin_load.verification.v1", Summary: fmt.Sprintf("Load recommended local EQ %s on %d C1 static-EQ target tracks", candidate.Name, len(loads)), CreatedAt: time.Now().UTC()}
	proposal.Presentation = c1LoadProposalPresentation(proposal, loads, candidate, recommendation)
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_c1_load_" + sanitizeCanaryID(treatment.PlanID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 plug-in load Proposal persistence failed: "+err.Error())
	}
	return c1ProposalResponse(conversationID, goal, session)
}

func c1LoadProposalPresentation(proposal orchestration.Proposal, loads []map[string]any, candidate pluginRecommendationCandidate, recommendation pluginRecommendationPlan) *orchestration.ProposalPresentation {
	actions := []orchestration.ProposalActionPreview{}
	for index, load := range loads {
		actions = append(actions, orchestration.ProposalActionPreview{ActionID: fmt.Sprintf("load_%d", index+1), TrackID: firstStringFromMap(load, "track_id"), TrackName: firstStringFromMap(load, "track_name"), Operation: "load_eq_plugin", Reason: "C1 static_eq target has no qualified generic EQ"})
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: "C1 full-project EQ plug-in load Proposal", Conclusion: fmt.Sprintf("Load %s on %d tracks that require static EQ.", candidate.Name, len(loads)), AnalysisSummary: []string{recommendation.Summary}, Recommendation: proposal.Summary, ActionCount: len(loads), Risk: proposal.Risk, Reversible: true, Actions: actions, Limitations: []string{"This confirmation authorizes plug-in loading only, not EQ parameter writes.", "Any load or qualification failure removes every instance added by this batch."}, ApprovalPrompt: "Confirm this exact project-level plug-in load batch? EQ parameters will require a separate Proposal."}
}

func (s *Server) createC1EQProposal(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, treatment frequencycleanup.TreatmentPlan, model frequencycleanup.Model, baseline frequencycleanup.TargetPostFXBaseline, targets []c1EQTargetContext, cfg config.EngineConfig) ChatResponse {
	if !baseline.Readiness.CanProceed || baseline.SchemaVersion != frequencycleanup.TargetPostFXBaselineSchema {
		return c1TargetPreflightBlockedResponse(conversationID, goal, sessionID, treatment, baseline, fmt.Errorf("target post-FX baseline is not ready"))
	}
	batch, err := s.planC1EQBatch(ctx, conversationID, treatment, baseline, targets, cfg, "")
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 static-EQ planning failed; no writes occurred: "+err.Error())
	}
	leaves, materializeErr := s.materializeProjectSemanticEQLeaves(ctx, batch, treatment.EvidenceRefs, treatment.ObservationID, "full_project_c1_frequency_cleanup")
	if materializeErr != nil {
		batch, err = s.planC1EQBatch(ctx, conversationID, treatment, baseline, targets, cfg, materializeErr.Error())
		if err == nil {
			leaves, materializeErr = s.materializeProjectSemanticEQLeaves(ctx, batch, treatment.EvidenceRefs, treatment.ObservationID, "full_project_c1_frequency_cleanup")
		}
	}
	if err != nil || materializeErr != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 deterministic EQ materialization rejected the batch; no writes occurred: "+firstNonEmpty(errorText(materializeErr), errorText(err)))
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 EQ Proposal could not obtain a strong project snapshot.")
	}
	deps := []string{"c1-treatment:" + treatment.PlanID, "c1-target-post-fx:" + baseline.BaselineID, "c1-semantic-batch:" + semanticEQHash(batch)}
	fingerprints := []string{}
	for _, leaf := range leaves {
		fingerprints = append(fingerprints, "plugin:"+leaf.Action.Target.TrackID+":"+leaf.Action.Target.PluginID, "eq-topology:"+leaf.TopologyGeneration)
		for _, row := range leaf.ParameterPreimage {
			fingerprints = append(fingerprints, fmt.Sprintf("eq-param:%s:%0.9f", firstStringFromMap(row, "param_id"), semanticEQNumber(row["normalized"])))
		}
	}
	decisionRefs, _ := mixboardDecisionContextForState(state.LegacyState, frequencyCleanupCapabilityID)
	cut, err := projectcut.Build(projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: deps, TargetFingerprints: fingerprints, ArtifactRefs: appendUniqueStrings(append(append([]string(nil), treatment.EvidenceRefs...), baseline.EvidenceRefs...), mixboardDecisionArtifactRefs(decisionRefs)...), ContractVersions: []string{"capability:" + frequencyCleanupCapabilityID, "treatment:" + frequencycleanup.TreatmentPlanSchema, "baseline:" + frequencycleanup.TargetPostFXBaselineSchema, "batch:" + semanticeffect.BatchSchema, "action:" + semanticeffect.ActionSchema, "executor:plugin_grabber.apply_eq_edits"}})
	if err != nil || !cut.IsExecutable() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 EQ Proposal could not build a strong ProjectCut.")
	}
	proposal, actionSet, err := capabilityadapters.FreezeProjectSemanticEQBatch(capabilityadapters.ProjectSemanticEQBatchSpec{CapabilityID: frequencyCleanupCapabilityID, CapabilityVersion: "v1", PlanID: treatment.PlanID, ActionID: "c1_semantic_eq_batch", ActionSetPrefix: "actionset_c1_eq_", Command: c1EQBatchCommand, TargetRef: "project:frequency_cleanup", CandidatePrefix: "c1_eq_", VerificationRef: frequencycleanup.VerificationSchema, Summary: fmt.Sprintf("Apply %d C1 static-EQ targets as one project all-or-rollback batch", len(leaves)), IdempotencyClass: "c1_project_eq_all_or_rollback", Treatment: treatment, Diagnosis: model, VerificationContext: baseline, Batch: batch, Leaves: leaves, ExpectedTargetCount: len(targets)}, cut, 1)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 EQ batch freeze failed; no writes occurred: "+err.Error())
	}
	proposal.Presentation = c1EQProposalPresentation(proposal, batch, leaves)
	session, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: "bundle_c1_eq_" + sanitizeCanaryID(treatment.PlanID), PreviousObservationID: treatment.ObservationID, FrozenAt: time.Now().UTC()})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 EQ Proposal persistence failed; no writes occurred: "+err.Error())
	}
	return c1ProposalResponse(conversationID, goal, session)
}

func c1EQProposalPresentation(proposal orchestration.Proposal, batch semanticeffect.Batch, leaves []capabilityadapters.SemanticEQPlan) *orchestration.ProposalPresentation {
	actions := []orchestration.ProposalActionPreview{}
	analysis := []string{}
	limitations := []string{"Only static-EQ parameters in this frozen batch are authorized.", "Any leaf failure restores every previously applied leaf from its frozen preimage."}
	for index, leaf := range leaves {
		analysis = append(analysis, fmt.Sprintf("%d. %s / %s: %s", index+1, leaf.Action.Target.TrackName, leaf.Action.Target.PluginName, leaf.Action.UserGoal))
		for _, atom := range leaf.Action.EQPlan.Atoms {
			frequency, gain := 0.0, 0.0
			if atom.FrequencyHz != nil {
				frequency = *atom.FrequencyHz
			}
			if atom.GainDB != nil {
				gain = *atom.GainDB
			}
			actions = append(actions, orchestration.ProposalActionPreview{ActionID: atom.AtomID, TrackID: leaf.Action.Target.TrackID, TrackName: leaf.Action.Target.TrackName, Operation: atom.Action + ":" + atom.Shape, Target: frequency, Delta: gain, Unit: "Hz/dB", Reason: atom.Purpose})
		}
		limitations = append(limitations, leaf.Action.Limitations...)
	}
	return &orchestration.ProposalPresentation{SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID, Title: "C1 full-project static-EQ Proposal", Conclusion: fmt.Sprintf("Apply one atomic static-EQ batch to %d exact track/plugin targets.", len(batch.Actions)), AnalysisSummary: analysis, Recommendation: proposal.Summary, ActionCount: len(actions), Risk: proposal.Risk, Reversible: true, Actions: actions, Limitations: semanticEQUniqueStrings(limitations), ApprovalPrompt: "Confirm this exact frozen C1 static-EQ batch?"}
}

func c1ProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 Session is missing its frozen Proposal.")
	}
	proposal := session.ActiveProposal
	stage := "eq_parameter_confirmation"
	if session.FrozenPlan.ActionSet.Actions[0].Command == c1LoadBatchCommand {
		stage = "plugin_load_confirmation"
	}
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: proposal.Presentation.Conclusion + "\n\n" + proposal.Presentation.ApprovalPrompt, NeedsConfirmation: true, PlanID: proposal.ID, Preview: proposal.Presentation.Conclusion, ProposalPresentation: proposal.Presentation, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), WorkflowData: map[string]any{"session_id": session.ID, "capability_id": frequencyCleanupCapabilityID, "proposal_id": proposal.ID, "proposal_revision": proposal.Revision, "action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash, "stage": stage, "writes_performed": 0, "proposal_presentation": proposal.Presentation}}
}

type c1BatchVerifier struct {
	server              *Server
	before              frequencycleanup.TargetPostFXBaseline
	sessionID, goalText string
}

func (v c1BatchVerifier) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	verificationStarted := time.Now()
	defer func() {
		if v.server != nil && v.server.logger != nil {
			v.server.logger.Info("[timing] c1.target_verification total_ms=%d target_count=%d", time.Since(verificationStarted).Milliseconds(), len(v.before.RequestedTrackIDs))
		}
	}()
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if len(receipts) != 1 || !strings.EqualFold(receipts[0].Status, "applied") || !strings.EqualFold(firstStringFromMap(receipts[0].Details, "structural_readback"), "pass") {
		result.Status, result.Structural = "fail", "fail"
		return result, fmt.Errorf("C1 batch structural readback failed")
	}
	result.Structural = "pass"
	result.EvidenceRefs = append(result.EvidenceRefs, receipts[0].EvidenceRefs...)
	if actionSet.Actions[0].Command == c1LoadBatchCommand {
		result.Status, result.Acoustic, result.SpecialistRelationship = "pass", "not_run", "not_run"
		result.Summary = "Plug-in loading and exact generic-EQ qualification passed; no EQ parameter has been applied."
		return result, nil
	}
	if v.server == nil || v.server.kernel == nil || v.server.harness == nil {
		result.Acoustic = "unavailable"
		result.Summary = "Structural readback passed, but C1 post-execution observation is unavailable."
		return result, nil
	}
	state, err := v.server.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		result.Acoustic = "inconclusive"
		result.Summary = "Structural readback passed, but verification project state is unavailable."
		return result, nil
	}
	collected, err := v.server.harness.CollectL2RenderProbeBatch(ctx, harness.L2RenderProbeBatchRequest{SessionID: v.sessionID, GoalText: v.goalText, TrackIDs: v.before.RequestedTrackIDs, TapPoint: "track_post_fader", ForceFresh: true})
	if err != nil {
		result.Acoustic = "inconclusive"
		result.Summary = "Structural readback passed, but target-scoped post-FX verification collection failed: " + err.Error()
		return result, nil
	}
	projectID := firstNonEmpty(firstStringFromMap(state.LegacyState, "project_uuid"), firstStringFromMap(firstMapFromAny(state.LegacyState["project"]), "project_uuid", "uuid", "id"), "current")
	after := frequencycleanup.BuildTargetPostFXBaseline(v.sessionID, "post_execution", projectID, v.before.RequestedTrackIDs, collected.Rows, time.Now().UTC())
	verification := frequencycleanup.VerifyTargetPostFXBaselines(v.before, after)
	if verification.Status == "observed" && verification.Comparable && len(verification.ChangedDimensions) > 0 {
		result.Status, result.Acoustic, result.SpecialistRelationship = "pass", "observed", "observed_change"
	} else {
		result.Status, result.Acoustic, result.SpecialistRelationship = "inconclusive", "inconclusive", "unchanged_or_unavailable"
	}
	result.SpecialistSummary = fmt.Sprintf("C1 target-scoped same-tap post-FX comparison: comparable=%v, changed_dimensions=%s, reasons=%s", verification.Comparable, strings.Join(verification.ChangedDimensions, ","), strings.Join(verification.Reasons, ","))
	result.Summary = "All static-EQ leaves passed structural readback. " + result.SpecialistSummary + ". This records observable relationship change and does not claim user listening acceptance."
	result.EvidenceRefs = append(result.EvidenceRefs, verification.EvidenceRefs...)
	return result, nil
}

func (s *Server) authorizeC1Batch(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, current *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil || session.ActiveProposal == nil || current == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 Session lacks a FrozenPlan; nothing executed.")
	}
	frozen := *session.FrozenPlan
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || current.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || current.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "Project revision/epoch or Kernel CAS changed; the C1 Proposal was not authorized.")
	}
	if !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "The v1 Session Store is not durable; the C1 Proposal was not authorized.")
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "This turn is not an exact authorization for the frozen C1 Proposal revision.")
	}
	_, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision, ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash, Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID, Sequence: session.Revision + 1, Decision: &decision})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "C1 authorization binding failed: "+err.Error())
	}
	var treatment frequencycleanup.TreatmentPlan
	var diagnosis frequencycleanup.Model
	var before frequencycleanup.TargetPostFXBaseline
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["treatment_plan"], &treatment)
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["diagnosis_model"], &diagnosis)
	_ = decodeAnyJSON(frozen.ActionSet.Actions[0].Args["verification_context"], &before)
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(ctx, session.ID, frozen.ActionSet, frozen.ProjectCut, &projectEQBatchMutationPort{server: s, spec: c1BatchSpec}, c1BatchVerifier{server: s, before: before, sessionID: session.ID, goalText: session.Goal}, executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID})
	response := c1ExecutionResponse(conversationID, goal, executed, executeErr)
	attachMixboardDecisionProjection(&response, executed)
	if executeErr == nil && frozen.ActionSet.Actions[0].Command == c1LoadBatchCommand && executed.Execution != nil && len(executed.Execution.Receipts) == 1 {
		preferred := map[string]string{}
		for _, row := range mapRowsValue(executed.Execution.Receipts[0].Details["loaded"]) {
			preferred[firstStringFromMap(row, "track_id")] = firstStringFromMap(row, "plugin_id")
		}
		cfg, _, cfgErr := config.Load()
		postLoadState, stateErr := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
		newSessionID := s.nextCapabilitySessionID(conversationID, frequencyCleanupCapabilityID)
		if cfgErr == nil && cfg.Complete() && stateErr == nil && postLoadState != nil && postLoadState.OK() {
			newSession, startErr := s.orchestrationRuntime.StartC1ChatSession(newSessionID, conversationID, frozen.ProjectCut.ProjectUUID, session.Goal, orchestration.InteractionPropose)
			if startErr == nil {
				items := frequencycleanup.StaticEQItems(treatment)
				resolution := s.resolveC1EQTargets(ctx, items, req.Context, preferred)
				if len(resolution.Ambiguous) > 0 {
					return c1PluginSelectionRequiredResponse(conversationID, goal, newSession.ID, treatment, resolution)
				}
				if len(resolution.Missing) == 0 {
					baseline, baselineErr := s.collectC1TargetPostFXBaseline(ctx, newSession.ID, session.Goal, "post_plugin_load", postLoadState, treatment, diagnosis, false)
					if baselineErr == nil {
						next := s.createC1EQProposal(ctx, conversationID, goal, newSession.ID, treatment, diagnosis, baseline, resolution.Resolved, cfg)
						next.Reply = "Plug-in loading and exact generic-EQ qualification passed; C1 then collected only the selected target/relationship post-FX baseline.\n\n" + next.Reply
						return next
					}
					response.WorkflowData["post_load_target_preflight_error"] = baselineErr.Error()
				}
			}
		}
		response.Reply += " The separately confirmed plug-in load completed, but C1 could not establish a fresh target-scoped post-load baseline and parameter Proposal; no EQ parameters were written."
		response.WorkflowData["post_load_handoff"] = "blocked"
	}
	return response
}

func c1ExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, executeErr error) ChatResponse {
	data := map[string]any{"session_id": session.ID, "capability_id": frequencyCleanupCapabilityID, "user_acceptance": "unknown"}
	status, reply := string(agentruntime.StatusFailed), "C1 execution was rejected before a safe write."
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
				reply = "C1 project batch completed; exact readback, rollback data, and frequency-relationship verification were persisted."
			} else {
				reply = "C1 project batch failed; the project-wide rollback policy was applied."
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
