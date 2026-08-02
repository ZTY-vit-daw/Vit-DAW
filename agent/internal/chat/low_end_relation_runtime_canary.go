package chat

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	"vit-daw-agent/internal/protocolvalue"
	agentruntime "vit-daw-agent/internal/runtime"
)

func (s *Server) handleLowEndRelationRuntimeCanary(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.kernel == nil || s.orchestrationRuntime == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前没有可用的 B4 Project-aware Runtime。")
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法取得当前工程 snapshot，B4 分析未启动："+canaryErrorText(err))
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	if sessionID == "" {
		sessionID = s.nextCapabilitySessionID(conversationID, lowEndRelationCapabilityID)
	}
	session, exists := s.orchestrationRuntime.Store.Load(sessionID)
	if exists && session.ActiveProposal != nil && session.FrozenPlan != nil {
		if !expectedProposalMatches(req.Context, session.ActiveProposal) {
			return capabilityCanaryBlockedResponse(conversationID, goal, "B4 confirmation is bound to an expired Proposal; no project mutation was authorized.")
		}
		if session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying {
			return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
		}
		if decision, ok := capabilityApprovalDecisionFromContext(req.Context); ok && decision.Kind == orchestration.ApprovalApprove {
			return s.authorizeB4Batch(ctx, conversationID, req, goal, session, state)
		}
		return b4ProposalResponse(conversationID, goal, session)
	}

	input, dependencies, err := s.acquireLowEndRelationCanaryContext(ctx, sessionID, req, state)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 CCB 只读上下文获取失败："+err.Error())
	}
	decisionRefs, decisionRefsErr := mixboardDecisionContextForState(state.LegacyState, lowEndRelationCapabilityID)
	buildCut := projectcut.BuildRequest{
		State:                  state,
		Guarantee:              capabilityCanaryCutGuarantee(ctx, s.kernel),
		DependencyFingerprints: dependencies,
		ArtifactRefs:           mixboardDecisionArtifactRefs(decisionRefs),
		ContractVersions: []string{
			"capability:static_mix.low_end_relation.v0",
			"context:static_mix.low_end_relation.context_pack.v0",
		},
	}
	cut, err := projectcut.Build(buildCut)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法建立 B4 Project Cut："+err.Error())
	}
	if !exists {
		if _, err = s.orchestrationRuntime.StartB4ChatSession(sessionID, conversationID, cut.ProjectUUID, req.Message, canaryInteractionMode(req.Context)); err != nil {
			return capabilityCanaryBlockedResponse(conversationID, goal, "无法创建 B4 v1 Planning Session："+err.Error())
		}
	}
	planned, _, err := s.orchestrationRuntime.ShadowB4FromVSP(sessionID, state, buildCut, input)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 分析失败："+err.Error())
	}
	attachMixboardDecisionContext(&planned.Bundle, decisionRefs, decisionRefsErr)
	envelope, err := s.orchestrationRuntime.BuildCapabilityContextEnvelope(
		sessionID, planned.Bundle, capabilityCanaryToolSchemas(s),
		[]orchestration.ContextEntry{{ID: firstNonEmpty(goal.RunID, "current_turn"), Content: req.Message, Reference: "chat-history:" + conversationID, Priority: 100}},
		orchestration.DefaultContextWindowBudget(),
	)
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "B4 Context Envelope 未通过预算准入："+err.Error())
	}
	if planned.Outcome.Kind == orchestration.OutcomeBlocked {
		if lowEndRelationBlockersNeedBandAnalysis(planned.Outcome.Blockers) {
			return s.lowEndRelationTriggerBandAnalysisResponse(ctx, conversationID, goal, sessionID, planned)
		}
		response := capabilityCanaryBlockedResponse(conversationID, goal, "B4 当前没有低频频段占用数据："+strings.Join(planned.Outcome.Blockers, ", "))
		response.WorkflowData = map[string]any{
			"session_id": sessionID, "capability_id": lowEndRelationCapabilityID,
			"canary_stage": "readiness_blocked", "blockers": append([]string(nil), planned.Outcome.Blockers...),
			"readiness": planned.Pack.Readiness, "evidence_status": planned.Pack.EvidenceStatus,
			"context_bundle_id": planned.Bundle.ID,
		}
		return response
	}
	if b4MutationRequested(req.Message, req.Context) {
		return s.buildB4ActionableResponse(ctx, conversationID, req, goal, sessionID, planned.Pack.Model())
	}
	// Read-only B4 analysis is terminal. Keeping it active would make B4 the
	// sticky owner of unrelated future ordinary-Agent EQ requests.
	cancelled, _ := s.orchestrationRuntime.CancelPlanningSession(sessionID)
	response := lowEndRelationCanaryAnalysisResponse(conversationID, goal, sessionID, planned, envelope)
	attachMixboardDecisionProjection(&response, cancelled)
	return response
}

func (s *Server) acquireLowEndRelationCanaryContext(ctx context.Context, sessionID string, req ChatRequest, state *kernel.VSPStateResult) (capabilitycontext.LowEndRelationInput, []string, error) {
	if s == nil || s.harness == nil || state == nil {
		return capabilitycontext.LowEndRelationInput{}, nil, fmt.Errorf("harness and VSP state are required")
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
		return capabilitycontext.LowEndRelationInput{}, nil, fmt.Errorf("mix.observe: %s", firstNonEmpty(observe.Error, canaryErrorText(err), observe.Status))
	}
	observationID := firstStringFromMap(observe.Result, "observation_id")
	if observationID == "" {
		return capabilitycontext.LowEndRelationInput{}, nil, fmt.Errorf("mix.observe omitted observation_id")
	}
	tomProjection := capabilitycontext.BuildProjectTOMProjection(state.LegacyState, observationID, sessionID)
	dependencies := []string{"vsp.snapshot:" + state.SnapshotHash, "MOM:" + observationID}
	if len(tomProjection) > 0 {
		dependencies = append(dependencies, "TOM:project-state-derived:"+observationID)
	}
	return capabilitycontext.LowEndRelationInput{
		UserIntent:      req.Message,
		ProjectState:    cloneContext(state.LegacyState),
		MixObservation:  cloneContext(observe.Result),
		MOMProjection:   protocolvalue.Object(observe.Result["mom_projection"]),
		TOMProjection:   tomProjection,
		RequestContext:  cloneContext(req.Context),
		ExecutionMemory: firstMapFromAny(req.Context["execution_memory"]),
	}, dependencies, nil
}

// lowEndRelationBlockersNeedBandAnalysis reports whether B4's readiness block
// is caused by missing MOM sub/bass band occupancy data (see
// lowendrelation.assessReadiness condition "mom_low_end_band_occupancy").
// That data only exists once L3 band-energy analysis has run for the
// project's tracks — there is no automatic trigger for it (project load and
// import do not request it), so B4 must kick it off itself instead of
// reporting a dead-end block every time.
func lowEndRelationBlockersNeedBandAnalysis(blockers []string) bool {
	for _, blocker := range blockers {
		if blocker == "mom_low_end_band_occupancy" {
			return true
		}
	}
	return false
}

// lowEndRelationTriggerBandAnalysisResponse starts the deferred project
// audio-analysis queue directly through the harness (bypassing the agent
// tool-call guard that blocks the LLM from starting/canceling DAD analysis)
// and tells the user analysis is running instead of returning a static
// "blocked" reply with no path forward.
func (s *Server) lowEndRelationTriggerBandAnalysisResponse(ctx context.Context, conversationID string, goal agentruntime.Goal, sessionID string, planned capabilityadapters.LowEndRelationPlanResult) ChatResponse {
	started, err := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "project.audio_analysis_start",
		Args: map[string]any{
			"retry_missing":        true,
			"rebuild_from_project": true,
		},
		Context:    map[string]any{"capability_runtime_v1": true},
		Source:     "capability_runtime_v1_ccb_band_analysis_kickoff",
		Confirmed:  true,
		ToolCallID: "ccb:" + sessionID + ":project.audio_analysis_start",
	})
	reply := "B4 需要的低频/低音频段能量数据尚未生成，已在后台启动工程音频分析，请稍后重新提问以获取低频关系分析。"
	stage := "band_analysis_triggered"
	if err != nil || !strings.EqualFold(started.Status, "ok") {
		reply = "B4 需要的低频/低音频段能量数据尚未生成，尝试启动后台音频分析也失败了：" + firstNonEmpty(started.Error, canaryErrorText(err), started.Status)
		stage = "band_analysis_trigger_failed"
	}
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:      reply,
		Workflow:   "capability_runtime_v1",
		GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: map[string]any{
			"session_id": sessionID, "capability_id": lowEndRelationCapabilityID,
			"canary_stage": stage, "blockers": append([]string(nil), planned.Outcome.Blockers...),
			"readiness": planned.Pack.Readiness, "evidence_status": planned.Pack.EvidenceStatus,
			"context_bundle_id": planned.Bundle.ID,
		},
	}
}

// lowEndRelationDetailLines renders the concrete evidence B4 already computed
// (dominant tracks, conflict pairs, per-finding observations) as extra reply
// lines. Without this, the reply carries only aggregate counts and the user
// has no way to see which tracks are actually involved.
func lowEndRelationDetailLines(pack capabilitycontext.LowEndRelationPack) []string {
	trackName := func(id string) string {
		for _, t := range pack.Tracks {
			if t.TrackID == id {
				return firstNonEmpty(t.TrackName, id)
			}
		}
		return id
	}
	var lines []string
	if name := pack.Summary.DominantSubTrackID; name != "" {
		lines = append(lines, fmt.Sprintf("- Sub 频段主导: %s", trackName(name)))
	}
	if name := pack.Summary.DominantBassTrackID; name != "" {
		lines = append(lines, fmt.Sprintf("- Bass 频段主导: %s", trackName(name)))
	}
	for _, conflict := range pack.Conflicts {
		var names []string
		for _, t := range conflict.Tracks {
			if id, ok := t["track_id"].(string); ok && id != "" {
				names = append(names, trackName(id))
			}
		}
		if len(names) > 0 {
			lines = append(lines, fmt.Sprintf("- %s 频段冲突: %s（置信度：%s）", conflict.Band, strings.Join(names, " vs "), lowEndConfidenceText(conflict.Confidence)))
		}
	}
	for _, obs := range pack.Observations {
		if obs.Summary == "" || strings.HasSuffix(obs.Kind, "_masking_conflict") {
			// Masking-conflict observations restate the same pairs already
			// rendered above with concrete track names; skip to avoid duplication.
			continue
		}
		lines = append(lines, "- "+lowEndObservationText(obs))
	}
	return lines
}

// lowEndConfidenceText translates the fixed confidence labels produced by
// mom.bandConflictCandidates ("low_to_medium" is the only value emitted
// today) into Chinese for the user-facing reply.
func lowEndConfidenceText(confidence string) string {
	switch confidence {
	case "low_to_medium":
		return "低到中"
	case "":
		return "未知"
	default:
		return confidence
	}
}

// lowEndObservationText translates the fixed Observation.Kind values produced
// by lowendrelation.buildObservations into Chinese for the user-facing reply.
// obs.Summary (English) stays as-is in workflow_data for tooling/tests; an
// unrecognized future Kind falls back to that raw Summary rather than
// silently dropping the finding.
func lowEndObservationText(obs lowendrelation.Observation) string {
	switch obs.Kind {
	case "no_low_end_band_evidence":
		return "没有可用的 sub/bass 频段占用数据，无法进行低频分析"
	case "low_end_prominent":
		return "sub 和 bass 频段在整首工程中都偏突出，低频总能量相对其他频段明显偏多"
	case "no_low_end_conflict":
		return "sub/bass 频段未检测到明显的掩蔽冲突"
	default:
		return obs.Summary
	}
}

func lowEndRelationCanaryAnalysisResponse(conversationID string, goal agentruntime.Goal, sessionID string, planned capabilityadapters.LowEndRelationPlanResult, envelope orchestration.ContextEnvelope) ChatResponse {
	summary := planned.Outcome.Summary
	if summary == "" {
		summary = "B4 低频关系分析完成"
	}
	replyLines := append([]string{summary + "。这是 B4 只读分析，没有创建 Proposal 或授权。"}, lowEndRelationDetailLines(planned.Pack)...)
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:      strings.Join(replyLines, "\n"),
		Workflow:   "capability_runtime_v1",
		GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: map[string]any{
			"session_id":        sessionID,
			"capability_id":     planned.Pack.CapabilityID,
			"context_bundle_id": planned.Bundle.ID,
			"project_cut_hash":  planned.Bundle.ProjectCutHash,
			"canary_stage":      "analysis",
			"low_end_summary":   planned.Pack.Summary,
			"conflict_count":    planned.Pack.Summary.ConflictCount,
			"observation_count": len(planned.Pack.Observations),
			"context_budget":    capabilityCanaryContextBudget(envelope),
		},
	}
}
