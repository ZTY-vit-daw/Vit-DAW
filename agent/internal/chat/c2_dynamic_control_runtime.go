package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/dynamiccontrol"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
)

var c2DiscoveryViews = []any{"track.basic_energy", "track.time_dynamics", "track.peak_structure", "track.activity_structure", "track.frequency_time_events", "track.transient_structure", "track.band_dynamics"}

// handleDynamicControlRuntime is the C2 orchestration shell. It owns the
// independent session and project-cut boundary, then delegates the selected
// leaf to COM-7 or semantic_dynamic_workflow. It never treats C1 as a gate.
// handleLegacyC2SingleTargetRuntime remains only to deserialize and reject
// historical C2 sessions created before the project-batch contract.  New C2
// entry always goes through handleC2ProjectBatchRuntime.
func (s *Server) handleLegacyC2SingleTargetRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
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
	referral, referralErr := c2ReferralFromContext(req.Context)
	if referralErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "invalid_referral", referralErr)
	}
	dependencies := []string{"vsp.snapshot:" + state.SnapshotHash}
	if referral != nil {
		dependencies = append(dependencies, "peer_referral:"+referral.SourcePlanID+":"+referral.ProjectRevision)
	}
	buildRequest := projectcut.BuildRequest{State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel), DependencyFingerprints: dependencies,
		ContractVersions: []string{"capability:" + dynamicControlCapabilityID, "referral:" + dynamiccontrol.ReferralSchema}}
	cut, err := projectcut.Build(buildRequest)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "project_cut_unavailable", err)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && session.ActiveProposal != nil && session.FrozenPlan != nil && len(session.FrozenPlan.ActionSet.Actions) == 1 && session.FrozenPlan.ActionSet.Actions[0].Command == c2DynamicPluginLoadCommand {
		if !expectedProposalMatches(req.Context, session.ActiveProposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "C2 插件加载确认已过期；没有加载或修改参数。")
		}
		if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok && decision.Kind == orchestration.ApprovalApprove {
			return s.authorizeC2DynamicPluginLoad(ctx, conversationID, req, goal, session, state)
		}
		return c2PluginLoadProposalResponse(conversationID, goal, session)
	}
	if !exists {
		if _, err = s.orchestrationRuntime.StartC2ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context)); err != nil {
			return c2BoundaryResponse(conversationID, goal, sessionID, "session_create_failed", err)
		}
	}
	trackID := firstStringFromMap(req.Context, "c2_target_track_id", "selected_plugin_track_id", "selected_track_id", "track_id")
	pluginID := firstStringFromMap(req.Context, "c2_target_plugin_id", "selected_plugin_id")
	family := c2FamilyFromContext(req.Context)
	hypothesis, discoveryData, resumeErr := s.c2TrustedHypothesisFromSession(session, req.Context)
	var discoveryErr error
	if resumeErr == nil {
		// A plugin-load transaction does not alter source dynamics. Its next
		// session uses a fresh ProjectCut and the already audited C2 treatment
		// decision, rather than asking a second model turn to rediscover it.
		trackID, family = hypothesis.TrackID, hypothesis.Intent.Family
	} else {
		hypothesis, discoveryData, discoveryErr = s.c2DiscoverTarget(ctx, conversationID, req.Message, req.Context, state, referral, trackID, family)
	}
	if discoveryErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "target_discovery_failed", discoveryErr)
	}
	if hypothesis.Status == dynamiccontrol.HypothesisNoAction {
		response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
			Reply: "C2 已完成固定的全工程动态观察；当前没有需要动态处理的目标，没有修改工程。", Workflow: "capability_runtime_v1",
			GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{"session_id": sessionID, "capability_id": dynamicControlCapabilityID, "stage": "no_action_needed", "hypothesis": hypothesis, "discovery": discoveryData, "mutation_performed": false}}
		cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
		attachMixboardDecisionProjection(&response, cancelled)
		return response
	}
	if hypothesis.Status != dynamiccontrol.HypothesisTargeted {
		return c2BoundaryResponse(conversationID, goal, sessionID, "c2_observation_incomplete", fmt.Errorf("fixed C2 observation did not resolve to a treatment or no-action outcome"))
	}
	trackID = hypothesis.TrackID
	family = hypothesis.Intent.Family
	registry, registryErr := processorregistry.Default()
	if registryErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_registry_unavailable", registryErr)
	}
	pcaCoverage, pcaErr := registry.PCARequiredCoverage(family, hypothesis.Intent.RequiredCoverage)
	if pcaErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_coverage_unproven", pcaErr)
	}
	if pluginID == "" {
		options := s.c2PCAEligibleCandidates(ctx, s.c2LoadedCandidates(ctx, state, trackID, family), family, pcaCoverage)
		if len(options) == 0 {
			return decorateC2DelegatedResponse(s.c2DynamicPluginLoadSelection(ctx, conversationID, req.Message, req.Context, goal, sessionID, hypothesis, discoveryData, family, pcaCoverage), sessionID, cut.Hash, referral)
		}
		if len(options) != 1 {
			return decorateC2DelegatedResponse(s.c2PluginSelectionResponse(conversationID, goal, sessionID, cut, hypothesis, options, discoveryData), sessionID, cut.Hash, referral)
		}
		pluginID = options[0].PluginID
	} else if _, pcaErr := s.semanticLoadedInstancePCAAdmission(ctx, trackID, pluginID, semanticTreatmentPCAInput{Family: family, RequiredCoverage: pcaCoverage}); pcaErr != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "pca_admission_rejected", pcaErr)
	}
	treatment := dynamiccontrol.TreatmentPlan{SchemaVersion: dynamiccontrol.TreatmentPlanSchema, PlanID: "c2_plan_" + sanitizeCanaryID(sessionID), ProjectCutHash: cut.Hash, ObservationID: c2DiscoveryObservationID(discoveryData, trackID), Referral: referral, Targets: []dynamiccontrol.TreatmentTarget{{TrackID: trackID, PluginID: pluginID, ProcessorFamily: family, ListeningGoal: hypothesis.ListeningGoal, RequiredCoverage: hypothesis.Intent.RequiredCoverage, EvidenceRefs: hypothesis.EvidenceRefs}}}
	if err := treatment.Validate(); err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "treatment_plan_invalid", err)
	}
	requestContext := mergeContext(req.Context, map[string]any{
		"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "capability_session_id": sessionID,
		"c2_source_only_reversible": true,
		"c2_project_cut_hash":       cut.Hash, "c2_referral": referral, "selected_plugin_track_id": trackID, "selected_plugin_id": pluginID,
		"c2_target_track_id": trackID, "c2_processor_family": family, "c2_semantic_processor_intent": mapFromJSONStruct(hypothesis.Intent),
		"c2_treatment_plan": mapFromJSONStruct(treatment),
	})
	cfg, _, cfgErr := config.Load()
	if cfgErr != nil || !cfg.Complete() {
		return c2BoundaryResponse(conversationID, goal, sessionID, "ai_configuration_incomplete", firstNonNilError(cfgErr, fmt.Errorf("AI configuration is incomplete")))
	}
	var response ChatResponse
	if family == processorintent.FamilyBroadbandCompressor {
		response = s.planBoundSemanticCompressor(ctx, conversationID, hypothesis.ListeningGoal, requestContext, cfg)
	} else {
		observation := s.c2TargetObservation(ctx, conversationID, trackID, req.Message)
		response = s.planBoundSemanticDynamic(ctx, conversationID, hypothesis.ListeningGoal, requestContext, family, semanticDynamicObservationFromRecent(observation), cfg)
	}
	response = decorateC2DelegatedResponse(response, sessionID, cut.Hash, referral)
	if response.NeedsConfirmation {
		if err := s.attachC2FrozenPlan(sessionID, cut, treatment); err != nil {
			return c2BoundaryResponse(conversationID, goal, sessionID, "frozen_plan_persist_failed", err)
		}
	}
	return response
}

type c2Candidate struct {
	TrackID    string
	PluginID   string
	PluginName string
	Families   []string
}

// c2DiscoverTarget builds a fixed, project-wide observation pack before any
// processor inventory or PCA decision is considered. A loaded plugin is an
// execution resource, never a condition for C2 to observe an audio track.
func (s *Server) c2DiscoverTarget(ctx context.Context, conversationID, goalText string, requestContext map[string]any, state *kernel.VSPStateResult, referral *dynamiccontrol.Referral, requestedTrack, requestedFamily string) (dynamiccontrol.TargetHypothesis, map[string]any, error) {
	if state == nil {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("project snapshot is unavailable")
	}
	trackIDs := c2ProjectTrackIDs(state.LegacyState, requestedTrack)
	if len(trackIDs) == 0 {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("project has no observable audio tracks")
	}
	projectObserve, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "mix.observe", Args: map[string]any{
		"scope": "full_project", "project_context": true, "observation_only": true,
		"disclosure": "digest_catalog", "mix_session_id": "c2_" + sanitizeCanaryID(conversationID), "goal_text": goalText,
	}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "c2_fixed_observation": true, "observation_only": true}, Source: "c2_project_observation", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":mix.observe"})
	if err != nil || !strings.EqualFold(projectObserve.Status, "ok") {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("full-project mix.observe: %s", firstNonEmpty(errorText(err), projectObserve.Error, projectObserve.Status))
	}
	projectDigest := c2ProjectObservationDigest(projectObserve.Result)
	// The full-project MOM observation is the fixed all-track decision surface.
	// CCB track views are intentionally requested only for the single candidate
	// it selects: CCB is a deep target proof, not a scalable project index.
	trackRows := c2ProjectTrackRows(state.LegacyState, trackIDs)
	candidate, candidateErr := s.c2ProjectCandidate(ctx, conversationID, goalText, projectDigest, trackRows, requestedFamily)
	if candidateErr != nil {
		return dynamiccontrol.TargetHypothesis{}, nil, candidateErr
	}
	baseDiscovery := map[string]any{"schema_version": "fine_mix.dynamic_control.context_pack.v1", "project_observation": projectDigest, "analyzed_track_count": len(trackIDs), "candidate_inventory_deferred_until_after_treatment": true, "project_candidate": candidate}
	if candidate.Status == "no_action" {
		return dynamiccontrol.TargetHypothesis{SchemaVersion: dynamiccontrol.HypothesisSchema, Status: dynamiccontrol.HypothesisNoAction, Rationale: candidate.Rationale}, baseDiscovery, nil
	}
	if candidate.Status != "candidate" {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("fixed C2 project observation could not identify a confirmable target: %s", candidate.Rationale)
	}
	response, invokeErr := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "ccb.observation_request", Args: map[string]any{"request_id": "c2-discovery-" + sanitizeCanaryID(conversationID) + "-" + sanitizeCanaryID(candidate.TrackID), "mix_session_id": "c2_" + sanitizeCanaryID(conversationID), "view_ids": c2DiscoveryViews, "target_kind": "track", "target_id": candidate.TrackID, "max_disclosure_bytes": 8192, "goal_text": goalText}, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "c2_discovery": true, "observation_only": true}, Source: "c2_target_confirmation", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":ccb:" + sanitizeCanaryID(candidate.TrackID)})
	if invokeErr != nil || !strings.EqualFold(response.Status, "ok") && !strings.EqualFold(response.Status, "partial") {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("CCB confirmation for C2 candidate %s failed: %s", candidate.TrackID, firstNonEmpty(errorText(invokeErr), response.Error, response.Status))
	}
	bundle := c2ObservationBundle(response.Result["bundle"])
	if len(bundle) == 0 {
		bundle = c2ObservationBundle(response.Result)
	}
	observations := []map[string]any{{"track_id": candidate.TrackID, "allowed_families": []string{candidate.Family}, "bundle": bundle}}
	input := map[string]any{"schema_version": "fine_mix.dynamic_control.context_pack.v1", "user_request": goalText, "project_observation": projectDigest, "tracks": observations, "optional_peer_referral": referral, "requested_track": requestedTrack, "requested_family": candidate.Family, "analyzed_track_count": len(trackIDs), "candidate_inventory_deferred_until_after_treatment": true}
	encoded, _ := json.Marshal(input)
	if s.llm == nil {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("C2 discovery planner is unavailable")
	}
	system := `You are the fixed C2 project-level dynamic-control decision stage. Return ONLY fine_mix.dynamic_control.hypothesis.v1 JSON. This is not a free-state improvement loop. First decide whether the supplied fixed full-project C2 context contains a justified dynamic treatment target. Return status=no_action when no dynamic treatment is currently justified; this is a successful terminal C2 outcome. Return targeted only for one exact supplied track and one exact family from that track's allowed_families. Do not use plugin availability, plugin identity, vendor names, or parameter names to decide whether treatment is needed: PCA and plugin selection occur after this decision. Evidence refs are an allow-list: copy them verbatim from the target CCB bundle observation_id, evidence_refs, or audit_receipt.receipt_id. The semantic_processor_intent must contain the smallest evidence-supported required_coverage for the selected family, scope=current_track, control_mode=semantic_loop, and exact observation evidence refs. Never infer parameter values or promise acoustic certainty. If the fixed context cannot establish either no_action or a treatment target, return unresolved. Shape: {"schema_version":"fine_mix.dynamic_control.hypothesis.v1","status":"targeted|no_action|unresolved","track_id":"exact supplied id","listening_goal":"open audible improvement","semantic_processor_intent":{"schema_version":"semantic_processor_intent.v1","status":"resolved","family":"exact supplied family","intent":"open acoustic intent","required_coverage":["exact family vocabulary axis"],"scope":"current_track","control_mode":"semantic_loop","confidence":0.0,"evidence_refs":["exact observation id/evidence ref"]},"evidence_refs":["exact observation id/evidence ref"],"rationale":"evidence-grounded reason"}.`
	cfg, _, cfgErr := config.Load()
	if cfgErr != nil || !cfg.Complete() {
		return dynamiccontrol.TargetHypothesis{}, nil, firstNonNilError(cfgErr, fmt.Errorf("AI configuration is incomplete"))
	}
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(encoded)}}, Metadata: llm.RequestMetadata{Source: "c2_project_target_planner", ConversationID: conversationID}, PreferJSON: true}
	modelResponse, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return dynamiccontrol.TargetHypothesis{}, nil, err
	}
	familyMap := map[string]map[string]bool{}
	for _, trackID := range trackIDs {
		familyMap[trackID] = map[string]bool{}
		for _, f := range c2SupportedFamilies(candidate.Family) {
			familyMap[trackID][f] = true
		}
	}
	validate := func(text string) (dynamiccontrol.TargetHypothesis, error) {
		var candidate dynamiccontrol.TargetHypothesis
		if err := decodeJSONObject(text, &candidate); err != nil {
			return candidate, err
		}
		if err := candidate.Validate(familyMap); err != nil {
			return candidate, err
		}
		if err := c2ValidateHypothesisEvidence(candidate, observations); err != nil {
			return candidate, err
		}
		return candidate, nil
	}
	hypothesis, validationErr := validate(modelResponse.Text)
	if validationErr != nil {
		request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: modelResponse.Text}, llm.Message{Role: "user", Content: "Your hypothesis was rejected by the deterministic C2 boundary: " + validationErr.Error() + ". Return one corrected hypothesis for the same observations. Evidence refs must be copied verbatim from the target CCB bundle; if evidence is insufficient, return status=no_action with no target or intent fields."})
		repaired, repairErr := s.llm.CompleteRequest(ctx, cfg, request)
		if repairErr != nil {
			return dynamiccontrol.TargetHypothesis{}, nil, repairErr
		}
		hypothesis, validationErr = validate(repaired.Text)
	}
	if validationErr != nil {
		return dynamiccontrol.TargetHypothesis{}, nil, validationErr
	}
	baseDiscovery["observation_id"] = firstStringFromMap(bundle, "observation_id")
	baseDiscovery["tracks"] = observations
	baseDiscovery["disclosed_track_count"] = 1
	return hypothesis, baseDiscovery, nil
}

func flattenCandidateFamilies(candidates []c2Candidate) []string {
	out := []string{}
	for _, c := range candidates {
		out = append(out, c.Families...)
	}
	return out
}

func c2SupportedFamilies(requested string) []string {
	all := []string{
		processorintent.FamilyBroadbandCompressor, processorintent.FamilyLimiter,
		processorintent.FamilyGateExpander, processorintent.FamilyDeEsser,
		processorintent.FamilyTransientShaper, processorintent.FamilyMultibandDynamics,
	}
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return all
	}
	for _, family := range all {
		if family == requested {
			return []string{family}
		}
	}
	return nil
}

type c2ProjectCandidate struct {
	SchemaVersion string `json:"schema_version"`
	Status        string `json:"status"`
	TrackID       string `json:"track_id,omitempty"`
	Family        string `json:"family,omitempty"`
	Rationale     string `json:"rationale"`
}

// c2ProjectCandidate turns the full-project MOM digest into a single optional
// C2 target. It deliberately has no coverage or parameter fields: the target
// CCB bundle is still required before a processor intent may be frozen.
func (s *Server) c2ProjectCandidate(ctx context.Context, conversationID, goalText string, projectObservation map[string]any, tracks []map[string]any, requestedFamily string) (c2ProjectCandidate, error) {
	if s == nil || s.llm == nil {
		return c2ProjectCandidate{}, fmt.Errorf("C2 project candidate planner is unavailable")
	}
	rows := make([]map[string]any, 0, len(tracks))
	for _, track := range tracks {
		rows = append(rows, map[string]any{"track_id": firstStringFromMap(track, "track_id", "id"), "track_name": firstStringFromMap(track, "track_name", "name"), "allowed_families": c2SupportedFamilies(requestedFamily)})
	}
	input, _ := json.Marshal(map[string]any{"schema_version": "fine_mix.dynamic_control.project_candidate.v1", "user_request": goalText, "project_observation": projectObservation, "tracks": rows})
	system := `You are the first, fixed C2 full-project dynamic-control stage. Return ONLY JSON. This is not free-state reasoning. Read the supplied full-project MOM observation and decide whether a dynamic treatment candidate needs deep confirmation. Return no_action only when the full-project observation supports no dynamic treatment. Return candidate only for one exact supplied track and one allowed family when its project-level dynamics justify target CCB confirmation. Do not use plugins, vendor names, parameters, or PCA. Do not infer control values or coverage. Shape: {"schema_version":"fine_mix.dynamic_control.project_candidate.v1","status":"candidate|no_action|unresolved","track_id":"exact supplied id when candidate","family":"exact allowed family when candidate","rationale":"short evidence-grounded reason"}.`
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return c2ProjectCandidate{}, firstNonNilError(err, fmt.Errorf("AI configuration is incomplete"))
	}
	request := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(input)}}, Metadata: llm.RequestMetadata{Source: "c2_project_candidate_planner", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, request)
	if err != nil {
		return c2ProjectCandidate{}, err
	}
	decode := func(text string) (c2ProjectCandidate, error) {
		var value c2ProjectCandidate
		if err := decodeJSONObject(text, &value); err != nil {
			return value, err
		}
		if value.SchemaVersion != "fine_mix.dynamic_control.project_candidate.v1" || (value.Status != "candidate" && value.Status != "no_action" && value.Status != "unresolved") || strings.TrimSpace(value.Rationale) == "" {
			return value, fmt.Errorf("invalid C2 project candidate")
		}
		if value.Status == "candidate" {
			found := false
			for _, track := range rows {
				if value.TrackID == firstStringFromMap(track, "track_id") {
					found = true
					break
				}
			}
			if !found || len(c2SupportedFamilies(value.Family)) != 1 {
				return value, fmt.Errorf("C2 candidate selected an unavailable track or family")
			}
		}
		return value, nil
	}
	candidate, decodeErr := decode(response.Text)
	if decodeErr == nil {
		return candidate, nil
	}
	request.Messages = append(request.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: "Your response was rejected by the deterministic C2 candidate boundary: " + decodeErr.Error() + ". Return one corrected candidate, no_action, or unresolved JSON for the same project observation."})
	repaired, repairErr := s.llm.CompleteRequest(ctx, cfg, request)
	if repairErr != nil {
		return c2ProjectCandidate{}, repairErr
	}
	return decode(repaired.Text)
}

func c2ProjectTrackRows(state map[string]any, trackIDs []string) []map[string]any {
	byID := map[string]map[string]any{}
	for _, track := range mapRowsValue(state["tracks"]) {
		if trackID := firstStringFromMap(track, "track_id", "id"); trackID != "" {
			byID[trackID] = track
		}
	}
	rows := make([]map[string]any, 0, len(trackIDs))
	for _, trackID := range trackIDs {
		track := byID[trackID]
		rows = append(rows, map[string]any{"track_id": trackID, "track_name": firstStringFromMap(track, "track_name", "name")})
	}
	return rows
}

const c2ProjectDigestMaxBytes = 6_000

// c2ProjectObservationDigest is the model-facing projection of the immutable
// full-project observation. The raw response can contain every clip and media
// descriptor in a large project; those are available by receipt but are not
// dynamic-control evidence and must not consume the model context window.
func c2ProjectObservationDigest(raw map[string]any) map[string]any {
	budget := c2ProjectDigestMaxBytes
	omitted := false
	projection := c2DigestValue(raw, "", 0, &budget, &omitted)
	project, _ := projection.(map[string]any)
	if project == nil {
		project = map[string]any{}
	}
	return map[string]any{"schema_version": "fine_mix.dynamic_control.project_digest.v1", "projection": project, "raw_observation_retained_by_receipt": true, "omitted_for_context_budget": omitted, "max_bytes": c2ProjectDigestMaxBytes}
}

func c2DigestValue(value any, key string, depth int, budget *int, omitted *bool) any {
	if budget == nil || *budget <= 0 || depth > 8 {
		if omitted != nil {
			*omitted = true
		}
		return nil
	}
	if c2DigestOmitKey(key) {
		if omitted != nil {
			*omitted = true
		}
		return nil
	}
	consume := func(count int) bool {
		*budget -= count
		if *budget < 0 {
			if omitted != nil {
				*omitted = true
			}
			return false
		}
		return true
	}
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for name := range typed {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		out := map[string]any{}
		limit := 128
		if key == "tracks" {
			limit = 96
		}
		for index, name := range keys {
			if index >= limit || !consume(len(name)+4) {
				if omitted != nil {
					*omitted = true
				}
				break
			}
			if compact := c2DigestValue(typed[name], name, depth+1, budget, omitted); compact != nil {
				out[name] = compact
			}
		}
		return out
	case []any:
		limit := 24
		if key == "tracks" || key == "track_summaries" || key == "track_facts" {
			limit = 96
		}
		capacity := limit
		if len(typed) < capacity {
			capacity = len(typed)
		}
		out := make([]any, 0, capacity)
		for index, item := range typed {
			if index >= limit || !consume(2) {
				if omitted != nil {
					*omitted = true
				}
				break
			}
			if compact := c2DigestValue(item, key, depth+1, budget, omitted); compact != nil {
				out = append(out, compact)
			}
		}
		return out
	case string:
		if len(typed) > 512 {
			typed = typed[:512]
			if omitted != nil {
				*omitted = true
			}
		}
		if !consume(len(typed) + 2) {
			return nil
		}
		return typed
	case float64, float32, int, int64, int32, bool:
		if !consume(24) {
			return nil
		}
		return typed
	default:
		return nil
	}
}

func c2DigestOmitKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "clips" || key == "items" || key == "samples" || key == "waveform" || key == "waveforms" || key == "spectrogram" || key == "parameters" || key == "plugin_parameters" || key == "raw_audio" || key == "audio_data" || key == "media" {
		return true
	}
	return strings.Contains(key, "sample_buffer") || strings.Contains(key, "waveform_") || strings.Contains(key, "spectrogram_")
}

const c2LoadedInstanceSelectionCheckpointCommand = "c2.loaded_instance_selection_checkpoint.v1"

// c2TrustedHypothesisFromSession restores a treatment decision only from a
// durable C2 checkpoint or a completed C2 load receipt. Request context is
// deliberately limited to an opaque session reference; accepting a raw
// hypothesis here would let a caller skip the required project observation.
func (s *Server) c2TrustedHypothesisFromSession(current orchestration.PlanningSession, context map[string]any) (dynamiccontrol.TargetHypothesis, map[string]any, error) {
	resumeID := firstStringFromMap(context, "c2_resume_session_id")
	if resumeID == "" || s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("no trusted C2 resume session")
	}
	proof, ok := s.orchestrationRuntime.Store.Load(resumeID)
	if !ok || proof.Invocation.CapabilityID != dynamicControlCapabilityID || proof.Invocation.ConversationID != current.Invocation.ConversationID || proof.ProjectUUID != current.ProjectUUID || proof.FrozenPlan == nil || len(proof.FrozenPlan.ActionSet.Actions) != 1 {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("trusted C2 resume session is unavailable or mismatched")
	}
	action := proof.FrozenPlan.ActionSet.Actions[0]
	if action.Command != c2LoadedInstanceSelectionCheckpointCommand && action.Command != c2DynamicPluginLoadCommand {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("trusted C2 resume action is not eligible")
	}
	var hypothesis dynamiccontrol.TargetHypothesis
	if err := decodeAnyJSON(action.Args["hypothesis"], &hypothesis); err != nil {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("decode trusted C2 hypothesis: %w", err)
	}
	families := c2SupportedFamilies(hypothesis.Intent.Family)
	if hypothesis.Status != dynamiccontrol.HypothesisTargeted || hypothesis.TrackID == "" || len(families) != 1 {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("trusted C2 hypothesis is invalid")
	}
	pluginID := firstStringFromMap(context, "c2_target_plugin_id", "selected_plugin_id")
	if pluginID == "" {
		return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("trusted C2 resume is missing the selected plugin")
	}
	switch action.Command {
	case c2LoadedInstanceSelectionCheckpointCommand:
		found := false
		for _, row := range mapRowsValue(action.Args["pca_candidates"]) {
			if firstStringFromMap(row, "plugin_id") == pluginID && firstStringFromMap(row, "track_id") == hypothesis.TrackID {
				found = true
				break
			}
		}
		if !found {
			return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("selected plugin is not in the durable C2 PCA candidate set")
		}
	case c2DynamicPluginLoadCommand:
		if proof.Execution == nil || len(proof.Execution.Receipts) != 1 || proof.Execution.Receipts[0].Status != "applied" || firstStringFromMap(proof.Execution.Receipts[0].Details, "plugin_id") != pluginID {
			return dynamiccontrol.TargetHypothesis{}, nil, fmt.Errorf("C2 load receipt does not prove the selected plugin")
		}
	}
	return hypothesis, firstMapFromAny(action.Args["discovery"]), nil
}

func c2ProjectTrackIDs(state map[string]any, requestedTrack string) []string {
	requestedTrack = strings.TrimSpace(requestedTrack)
	if requestedTrack != "" {
		return []string{requestedTrack}
	}
	ids := []string{}
	for _, track := range mapRowsValue(state["tracks"]) {
		if trackID := firstStringFromMap(track, "track_id", "id"); trackID != "" {
			ids = appendUniqueStrings(ids, trackID)
		}
	}
	return ids
}

// c2LoadedCandidates is deliberately called only after the fixed C2 decision.
// It turns current project instances into execution candidates without
// feeding their availability back into project-level acoustic observation.
func (s *Server) c2LoadedCandidates(ctx context.Context, state *kernel.VSPStateResult, trackID, requestedFamily string) []c2Candidate {
	if state == nil || strings.TrimSpace(trackID) == "" {
		return nil
	}
	out := []c2Candidate{}
	for _, ref := range chatVisiblePluginRefs(state.LegacyState, trackID) {
		instances, _ := s.semanticTreatmentInstances(ctx, ref.TrackID)
		for _, instance := range instances {
			if instance.PluginID != ref.ID {
				continue
			}
			families := []string{}
			for _, surface := range instance.QualifiedSurfaces {
				if surface.InspectOnly || surface.Family == processorintent.FamilyStaticEQ || !processorintent.IsSupportedFamily(surface.Family) {
					continue
				}
				if requestedFamily == "" || surface.Family == requestedFamily {
					families = appendUniqueStrings(families, surface.Family)
				}
			}
			if len(families) > 0 {
				out = append(out, c2Candidate{TrackID: ref.TrackID, PluginID: ref.ID, PluginName: ref.Name, Families: families})
			}
		}
	}
	return out
}

func c2ProcessorTypeForFamily(family string) string {
	switch strings.TrimSpace(family) {
	case processorintent.FamilyBroadbandCompressor:
		return "compressor"
	case processorintent.FamilyLimiter, processorintent.FamilyGateExpander, processorintent.FamilyDeEsser, processorintent.FamilyTransientShaper, processorintent.FamilyMultibandDynamics:
		return family
	default:
		return ""
	}
}

// c2DynamicPluginLoadSelection exposes only currently PCA-admitted local
// dynamic candidates. The choice is separate from the later load confirmation.
func (s *Server) c2DynamicPluginLoadSelection(ctx context.Context, conversationID, goalText string, requestContext map[string]any, goal agentruntime.Goal, sessionID string, hypothesis dynamiccontrol.TargetHypothesis, discovery map[string]any, family string, coverage []processorattestation.Coverage) ChatResponse {
	processorType := c2ProcessorTypeForFamily(family)
	if processorType == "" {
		return c2BoundaryResponse(conversationID, goal, sessionID, "unsupported_dynamic_family", fmt.Errorf("C2 family %s has no load adapter", family))
	}
	candidates, err := s.localPluginRecommendationCandidates(ctx, processorType)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "dynamic_plugin_catalog_unavailable", err)
	}
	candidates, err = c2CoverageAttestedPluginRecommendationCandidates(candidates, processorType, coverage)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "dynamic_plugin_pca_catalog_unavailable", err)
	}
	if len(candidates) == 0 {
		return c2BoundaryResponse(conversationID, goal, sessionID, "no_pca_admitted_dynamic_plugin_candidate", fmt.Errorf("no locally loadable PCA-admitted %s candidate", family))
	}
	cfg, _, err := config.Load()
	if err != nil || !cfg.Complete() {
		return c2BoundaryResponse(conversationID, goal, sessionID, "ai_configuration_incomplete", firstNonNilError(err, fmt.Errorf("AI configuration is incomplete")))
	}
	recommendation, err := s.planPluginRecommendation(ctx, conversationID, hypothesis.ListeningGoal, processorType, mergeContext(requestContext, map[string]any{"selected_track_id": hypothesis.TrackID}), nil, candidates, cfg)
	if err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "dynamic_plugin_recommendation_failed", err)
	}
	candidateByKey := map[string]pluginRecommendationCandidate{}
	for _, candidate := range candidates {
		candidateByKey[candidate.Key] = candidate
	}
	rows, actions := []map[string]any{}, []AgentInteractionAction{}
	for index, choice := range recommendation.Choices {
		candidate, ok := candidateByKey[choice.CandidateKey]
		if !ok {
			continue
		}
		rows = append(rows, pluginRecommendationChoiceRow(choice, candidate))
		actions = append(actions, AgentInteractionAction{ID: "select_" + choice.CandidateKey, Label: "选择 " + candidate.Name, Style: map[bool]string{true: "primary", false: "secondary"}[index == 0], Description: choice.Reason, Recommended: index == 0})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不加载", Style: "secondary"})
	payload := map[string]any{"schema_version": "fine_mix.dynamic_control.plugin_selection.v1", "status": "awaiting_selection", "capability_id": dynamicControlCapabilityID, "session_id": sessionID, "conversation_id": conversationID, "goal_id": goal.GoalID, "run_id": goal.RunID, "hypothesis": hypothesis, "discovery": discovery, "processor_family": family, "required_coverage": coverage, "recommendation": recommendation, "recommendations": rows, "request_context": cloneContext(requestContext), "mutation_performed": false}
	interaction := AgentInteractionRequest{ID: "interaction_" + randomID(), Kind: "c2_dynamic_plugin_selection", Type: "c2_dynamic_plugin_selection", Source: "c2_dynamic_plugin_selection", Workflow: "c2_dynamic_plugin_selection", Stage: "awaiting_selection", Title: "C2 动态插件选择", Body: recommendation.Summary, Status: "waiting_for_user", ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Payload: payload, Data: payload, Actions: actions}
	s.storePendingInteraction(interaction, payload)
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: firstNonEmpty(recommendation.Summary, "C2 已发现动态处理目标，但该轨没有已加载的 PCA 合格实例。") + "\n请选择一个 PCA 认证的本地动态插件；选择后只会生成插件加载确认，不会写入任何动态参数。", Workflow: "c2_dynamic_plugin_selection", WorkflowData: payload, GoalStatus: string(agentruntime.StatusWaitingClarification), MessageKind: "proposal", InteractionRequests: []AgentInteractionRequest{interaction}}
}

func c2DiscoveryObservationID(discovery map[string]any, trackID string) string {
	for _, row := range mapRowsValue(discovery["tracks"]) {
		if firstStringFromMap(row, "track_id") == trackID {
			return firstStringFromMap(firstMapFromAny(row["bundle"]), "observation_id")
		}
	}
	return firstStringFromMap(discovery, "observation_id")
}

// c2ObservationBundle normalizes both the typed CCB bundle returned by the
// in-process harness and the map-shaped bundle returned across the HTTP
// boundary. Evidence validation must see the same structured fields in both
// paths, without accepting arbitrary string values as evidence.
func c2ObservationBundle(value any) map[string]any {
	if bundle := firstMapFromAny(value); len(bundle) > 0 {
		return bundle
	}
	if bundle := mapFromJSONStruct(value); len(bundle) > 0 {
		return bundle
	}
	return nil
}

func c2ValidateHypothesisEvidence(hypothesis dynamiccontrol.TargetHypothesis, observations []map[string]any) error {
	if hypothesis.Status != dynamiccontrol.HypothesisTargeted {
		return nil
	}
	allowed := map[string]bool{}
	for _, row := range observations {
		if firstStringFromMap(row, "track_id") != hypothesis.TrackID {
			continue
		}
		bundle := firstMapFromAny(row["bundle"])
		for _, ref := range c2BundleEvidenceRefs(bundle) {
			if ref = strings.TrimSpace(ref); ref != "" {
				allowed[ref] = true
			}
		}
	}
	for _, ref := range append(append([]string(nil), hypothesis.EvidenceRefs...), hypothesis.Intent.EvidenceRefs...) {
		if !allowed[strings.TrimSpace(ref)] {
			return fmt.Errorf("hypothesis cites evidence %q that was not returned for track %s", ref, hypothesis.TrackID)
		}
	}
	return nil
}

// c2BundleEvidenceRefs extracts only evidence-bearing fields from the CCB
// structure, including nested view facts. It deliberately ignores arbitrary
// strings such as user text, labels, and rationale prose.
func c2BundleEvidenceRefs(bundle map[string]any) []string {
	refs := []string{}
	var walk func(any, string)
	walk = func(value any, key string) {
		switch typed := value.(type) {
		case map[string]any:
			for name, nested := range typed {
				lower := strings.ToLower(strings.TrimSpace(name))
				if lower == "evidence_ref" || lower == "observation_id" || lower == "receipt_id" {
					if text := strings.TrimSpace(fmt.Sprint(nested)); text != "" && text != "<nil>" {
						refs = appendUniqueStrings(refs, text)
					}
				}
				if lower == "evidence_refs" {
					for _, ref := range contextStringSlice(nested) {
						refs = appendUniqueStrings(refs, ref)
					}
				}
				walk(nested, lower)
			}
		case []any:
			for _, nested := range typed {
				walk(nested, key)
			}
		}
	}
	walk(bundle, "")
	return refs
}

func (s *Server) c2PCAEligibleCandidates(ctx context.Context, candidates []c2Candidate, family string, coverage []processorattestation.Coverage) []c2Candidate {
	out := []c2Candidate{}
	for _, c := range candidates {
		for _, f := range c.Families {
			if f == family {
				if _, err := s.semanticLoadedInstancePCAAdmission(ctx, c.TrackID, c.PluginID, semanticTreatmentPCAInput{Family: family, RequiredCoverage: coverage}); err == nil {
					out = append(out, c)
				}
				break
			}
		}
	}
	return out
}

func (s *Server) c2PluginSelectionResponse(conversationID string, goal agentruntime.Goal, sessionID string, cut orchestration.ProjectCut, hypothesis dynamiccontrol.TargetHypothesis, options []c2Candidate, discovery map[string]any) ChatResponse {
	rows := []map[string]any{}
	actions := []AgentInteractionAction{}
	for _, option := range options {
		rows = append(rows, map[string]any{"track_id": option.TrackID, "plugin_id": option.PluginID, "plugin_name": option.PluginName, "families": option.Families})
		actions = append(actions, AgentInteractionAction{ID: "select_" + option.PluginID, Label: "选择 " + option.PluginName, Style: "secondary", Value: map[string]any{"plugin_id": option.PluginID}})
	}
	actions = append(actions, AgentInteractionAction{ID: "cancel", Label: "暂不处理", Style: "secondary"})
	checkpointAction := orchestration.Action{ID: "c2_loaded_instance_checkpoint_" + sanitizeCanaryID(sessionID), Command: c2LoadedInstanceSelectionCheckpointCommand, TargetRef: "track:" + hypothesis.TrackID, Args: map[string]any{"hypothesis": mapFromJSONStruct(hypothesis), "discovery": cloneContext(discovery), "pca_candidates": rows}, Compensatable: true, IdempotencyClass: "c2_loaded_instance_selection"}
	checkpointSet := orchestration.ActionSet{ID: "c2_loaded_instance_checkpoint_" + sanitizeCanaryID(sessionID), CapabilityID: dynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{checkpointAction}}
	checkpointSet.Hash = checkpointSet.ComputeHash()
	checkpointProposal := orchestration.Proposal{ID: "c2_loaded_instance_selection_" + sanitizeCanaryID(sessionID), Revision: 1, CapabilityID: dynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: hypothesis.TrackID, ActionSetHash: checkpointSet.Hash, TargetScope: []string{"track:" + hypothesis.TrackID}, Risk: "read_only_selection", VerificationRef: "fine_mix.dynamic_control.instance_selection.v1", Summary: "Select an already-loaded PCA-admitted C2 dynamic instance", CreatedAt: time.Now().UTC()}
	if _, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: checkpointProposal, ActionSet: checkpointSet, ProjectCut: cut, ContextBundleID: "bundle_c2_loaded_instance_" + sanitizeCanaryID(sessionID), PreviousObservationID: c2DiscoveryObservationID(discovery, hypothesis.TrackID), FrozenAt: time.Now().UTC()}); err != nil {
		return c2BoundaryResponse(conversationID, goal, sessionID, "loaded_instance_checkpoint_persist_failed", err)
	}
	payload := map[string]any{"schema_version": "fine_mix.dynamic_control.loaded_instance_selection.v1", "session_id": sessionID, "capability_id": dynamicControlCapabilityID, "conversation_id": conversationID, "goal_id": goal.GoalID, "run_id": goal.RunID, "pca_candidates": rows, "mutation_performed": false}
	interaction := AgentInteractionRequest{ID: "interaction_" + randomID(), Kind: "c2_loaded_plugin_selection", Type: "c2_loaded_plugin_selection", Source: "c2_loaded_plugin_selection", Workflow: "c2_loaded_plugin_selection", Stage: "awaiting_selection", Title: "C2 已加载动态实例选择", Body: "请选择一个已通过 PCA 的精确动态插件实例。", Status: "waiting_for_user", ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Payload: payload, Data: payload, Actions: actions}
	s.storePendingInteraction(interaction, payload)
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C2 已完成项目级观察和 PCA 候选筛选，但当前目标存在多个合格动态处理实例；请选择一个精确插件实例。没有修改工程。", Workflow: "c2_loaded_plugin_selection", GoalStatus: string(agentruntime.StatusWaitingClarification), WorkflowData: payload, InteractionRequests: []AgentInteractionRequest{interaction}}
}

func (s *Server) attachC2FrozenPlan(sessionID string, cut orchestration.ProjectCut, treatment dynamiccontrol.TreatmentPlan) error {
	if s == nil || s.orchestrationRuntime == nil {
		return fmt.Errorf("C2 orchestration runtime is unavailable")
	}
	target := treatment.Targets[0]
	actionSet := orchestration.ActionSet{ID: "c2_action_" + sanitizeCanaryID(sessionID), CapabilityID: dynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{{ID: "c2_leaf_" + sanitizeCanaryID(sessionID), Command: "semantic_dynamic_control.pending", TargetRef: "plugin:" + target.TrackID + ":" + target.PluginID, Args: map[string]any{"treatment_plan": mapFromJSONStruct(treatment)}, Compensatable: true}}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "c2_proposal_" + sanitizeCanaryID(sessionID), Revision: 1, CapabilityID: dynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, CandidateID: target.TrackID + ":" + target.PluginID, ActionSetHash: actionSet.Hash, TargetScope: []string{"track:" + target.TrackID, "plugin:" + target.PluginID}, Risk: "bounded_reversible", VerificationRef: "fine_mix.dynamic_control.verification.v1", Summary: target.ListeningGoal, CreatedAt: time.Now().UTC()}
	_, err := s.orchestrationRuntime.AttachFrozenPlan(sessionID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, PreviousObservationID: treatment.ObservationID})
	return err
}

func c2ReferralFromContext(context map[string]any) (*dynamiccontrol.Referral, error) {
	raw := firstMapFromAny(context["c2_referral"])
	if len(raw) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var referral dynamiccontrol.Referral
	if err := json.Unmarshal(data, &referral); err != nil {
		return nil, err
	}
	if err := referral.Validate(); err != nil {
		return nil, err
	}
	return &referral, nil
}

func c2FamilyFromContext(context map[string]any) string {
	if intent := firstMapFromAny(context["c2_semantic_processor_intent"]); len(intent) > 0 {
		if family := strings.TrimSpace(firstStringFromMap(intent, "family")); family != "" {
			if canonical := processorFamilyForTreatmentProcessorType(family); canonical != "" {
				return canonical
			}
			return family
		}
	}
	family := strings.TrimSpace(firstStringFromMap(context, "c2_processor_family", "processor_family"))
	if canonical := processorFamilyForTreatmentProcessorType(family); canonical != "" {
		return canonical
	}
	return family
}

func (s *Server) c2TargetObservation(ctx context.Context, conversationID, trackID, goalText string) *agentloop.RecentObservation {
	if s == nil || s.harness == nil || strings.TrimSpace(trackID) == "" {
		return nil
	}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "mix.observe", Args: map[string]any{
		"scope": "selected_track", "project_context": true, "observation_only": true, "track_id": trackID,
		"mix_session_id": "c2_" + sanitizeCanaryID(conversationID), "goal_text": goalText,
	}, Context: map[string]any{"capability_runtime_v1": true, "c2_observation": true, "observation_only": true}, Source: "c2_dynamic_control", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":mix.observe"})
	if err != nil || !strings.EqualFold(response.Status, "ok") {
		return &agentloop.RecentObservation{Tool: "mix.observe", Status: firstNonEmpty(response.Status, "error"), Error: firstNonEmpty(errorText(err), response.Error)}
	}
	return &agentloop.RecentObservation{ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":mix.observe", Tool: "mix.observe", Status: response.Status, Summary: cloneContext(response.Result)}
}

func decorateC2DelegatedResponse(response ChatResponse, sessionID, projectCutHash string, referral *dynamiccontrol.Referral) ChatResponse {
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	response.WorkflowData["c2"] = map[string]any{"schema_version": dynamiccontrol.TreatmentPlanSchema, "session_id": sessionID, "project_cut_hash": projectCutHash, "independent": true, "c1_referral_present": referral != nil, "post_action_refresh_required": true}
	response.WorkflowData["capability_id"] = dynamicControlCapabilityID
	response.WorkflowData["session_id"] = sessionID
	return response
}

// completeC2PostAction is intentionally separate from leaf execution. A
// successful typed write proves controller behavior, not that C2's acoustic
// goal is satisfied. C2 therefore waits for an authoritative project refresh
// and records a new CCB receipt for the relevant verification view.
func (s *Server) completeC2PostAction(ctx context.Context, interaction PendingInteraction, response ChatResponse) ChatResponse {
	if firstStringFromMap(interaction.RequestContext, "capability_id") != dynamicControlCapabilityID || !boolValue(response.WorkflowData["mutation_performed"]) {
		return response
	}
	if response.WorkflowData == nil {
		response.WorkflowData = map[string]any{}
	}
	meta := firstMapFromAny(response.WorkflowData["c2"])
	if len(meta) == 0 {
		meta = map[string]any{"schema_version": dynamiccontrol.TreatmentPlanSchema, "session_id": firstStringFromMap(interaction.RequestContext, "capability_session_id"), "independent": true}
	}
	change, authoritative := s.postActionProjectChange(ctx, "c2_post_action_refresh")
	meta["project_change"] = cloneContext(change)
	if !authoritative {
		meta["post_action_verification"] = map[string]any{"status": "pending_authoritative_refresh", "requires_fresh_observation": true}
		response.WorkflowData["c2"] = meta
		response.GoalStatus = string(agentruntime.StatusWaitingContinue)
		response.StopReason = "c2_project_refresh_pending"
		response.Reply = "C2 的 Typed Executor 已完成，但工程状态尚未完成权威刷新；不会使用旧观察进行验收。"
		return response
	}
	family := c2FamilyFromContext(interaction.RequestContext)
	trackID := firstStringFromMap(interaction.RequestContext, "c2_target_track_id", "selected_plugin_track_id", "selected_track_id", "track_id")
	if family == "" || trackID == "" || s == nil || s.harness == nil {
		meta["post_action_verification"] = map[string]any{"status": "inconclusive", "reason": "verification_target_or_family_unavailable", "requires_fresh_observation": true}
		response.WorkflowData["c2"] = meta
		return response
	}
	receipt := s.c2PostActionCCBObservation(ctx, interaction.ConversationID, firstStringFromMap(interaction.RequestContext, "capability_session_id"), trackID, family)
	meta["post_action_verification"] = receipt
	if sessionID := firstStringFromMap(interaction.RequestContext, "capability_session_id"); sessionID != "" {
		leafReceipt := firstMapFromAny(response.WorkflowData["execution_receipt"])
		if len(leafReceipt) > 0 {
			verification := orchestration.VerificationResult{Status: "inconclusive", Structural: "pass", Acoustic: "inconclusive", UserAcceptance: "unknown", Summary: "Typed leaf execution and fresh CCB observation are recorded; acoustic improvement remains a model/user review decision."}
			evidence := []string{}
			if observationID := firstStringFromMap(receipt, "observation_id"); observationID != "" {
				evidence = append(evidence, "ccb-observation:"+observationID)
			}
			if auditID := firstStringFromMap(firstMapFromAny(receipt["audit_receipt"]), "receipt_id"); auditID != "" {
				evidence = append(evidence, "ccb-receipt:"+auditID)
			}
			verification.EvidenceRefs = uniqueStrings(append(evidence, contextStringSlice(firstMapFromAny(leafReceipt["controller_result"])["evidence_refs"])...))
			orchestrationReceipt := orchestration.ActionReceipt{ActionID: firstStringFromMap(leafReceipt, "ticket_id", "execution_id"), Status: "applied", EffectivelyOnce: true, Details: leafReceipt, EvidenceRefs: verification.EvidenceRefs}
			if firstStringFromMap(leafReceipt, "status") != "executed" {
				orchestrationReceipt.Status = "failed"
				orchestrationReceipt.EffectivelyOnce = false
				verification.Structural = "failed"
				verification.Status = "failed"
			}
			if session, finalizeErr := s.orchestrationRuntime.FinalizeExternalLeaf(sessionID, []orchestration.ActionReceipt{orchestrationReceipt}, verification); finalizeErr == nil {
				meta["terminal_receipt"] = map[string]any{"session_id": session.ID, "status": session.Status, "execution_status": session.Execution.Status, "verification": session.Execution.VerificationResult}
				attachMixboardDecisionProjection(&response, session)
			} else {
				meta["terminal_receipt"] = map[string]any{"status": "persistence_failed", "error": finalizeErr.Error()}
			}
		}
	}
	response.WorkflowData["c2"] = meta
	return response
}

func (s *Server) c2PostActionCCBObservation(ctx context.Context, conversationID, sessionID, trackID, family string) map[string]any {
	views := c2VerificationViews(family)
	if len(views) == 0 || s == nil || s.harness == nil {
		return map[string]any{"status": "inconclusive", "reason": "no_registered_verification_view", "requires_fresh_observation": true}
	}
	args := map[string]any{"request_id": "c2-post-" + sanitizeCanaryID(conversationID), "mix_session_id": firstNonEmpty(sessionID, "c2_"+sanitizeCanaryID(conversationID)), "view_ids": views, "target_kind": "track", "target_id": trackID, "max_disclosure_bytes": 8192}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "ccb.observation_request", Args: args, Context: map[string]any{"capability_runtime_v1": true, "capability_id": dynamicControlCapabilityID, "c2_post_action": true, "observation_only": true}, Source: "c2_post_action_verification", Confirmed: true, ToolCallID: "c2:" + sanitizeCanaryID(conversationID) + ":ccb.post"})
	if err != nil || !strings.EqualFold(response.Status, "ok") {
		return map[string]any{"status": "inconclusive", "reason": firstNonEmpty(errorText(err), response.Error, response.Status), "requires_fresh_observation": true}
	}
	bundle := c2ObservationBundle(response.Result["bundle"])
	if len(bundle) == 0 {
		bundle = c2ObservationBundle(response.Result)
	}
	receipt := firstMapFromAny(bundle["audit_receipt"])
	if len(receipt) == 0 {
		return map[string]any{"status": "inconclusive", "reason": "ccb_receipt_missing", "requires_fresh_observation": true}
	}
	return map[string]any{"status": map[bool]string{true: "observed", false: "inconclusive"}[boolValue(receipt["view_set_matches"])], "observation_id": firstStringFromMap(bundle, "observation_id"), "audit_receipt": receipt, "requires_fresh_observation": false, "user_acceptance": "unknown"}
}

func c2VerificationViews(family string) []any {
	switch strings.TrimSpace(family) {
	case processorintent.FamilyBroadbandCompressor:
		return []any{"track.time_dynamics"}
	case processorintent.FamilyLimiter:
		return []any{"track.peak_structure"}
	case processorintent.FamilyGateExpander:
		return []any{"track.activity_structure", "track.time_dynamics"}
	case processorintent.FamilyDeEsser:
		return []any{"track.frequency_time_events"}
	case processorintent.FamilyTransientShaper:
		return []any{"track.transient_structure"}
	case processorintent.FamilyMultibandDynamics:
		return []any{"track.band_dynamics"}
	default:
		return nil
	}
}

func c2BoundaryResponse(conversationID string, goal agentruntime.Goal, sessionID, code string, err error) ChatResponse {
	message := firstNonEmpty(errorText(err), code)
	return ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: "C2 在受治理边界停止：" + message + "；没有修改工程。", Error: message, Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusFailed), StopReason: code, WorkflowData: map[string]any{"session_id": sessionID, "capability_id": dynamicControlCapabilityID, "stage": code, "mutation_performed": false}}
}

func c2ObservationPendingResponse(conversationID string, goal agentruntime.Goal, sessionID string, pending *c2ObservationPendingError) ChatResponse {
	reason := "observation data is still preparing"
	if pending != nil && strings.TrimSpace(pending.Reason) != "" {
		reason = strings.TrimSpace(pending.Reason)
	}
	return ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          "C2 的工程观察数据仍在准备中，暂未进入动态处理；没有修改工程。原因：" + reason + "。可以在数据就绪后续轮。",
		Workflow:       "capability_runtime_v1",
		GoalStatus:     string(agentruntime.StatusWaitingContinue),
		StopReason:     "c2_observation_pending",
		WorkflowData: map[string]any{
			"session_id":         sessionID,
			"capability_id":      dynamicControlCapabilityID,
			"stage":              "observation_pending",
			"resumable":          true,
			"mutation_performed": false,
			"observation_reason": reason,
		},
	}
}

func firstNonNilError(primary error, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
