package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/semanticeffect"
)

const agentSemanticEQCapabilityID = "agent.effect.eq_control.v0"

type semanticEQMaterialization struct {
	DigestGeneration string
	Edits            []map[string]any
	PlannedEdits     []map[string]any
	PlannedWrites    []map[string]any
	Preimage         []map[string]any
	Snapshot         []map[string]any
	PreviewResults   []map[string]any
}

func semanticEQTopologyPromptSummary(trackID, pluginID string, summary map[string]any) map[string]any {
	out := map[string]any{
		"schema_version":        "generic_eq_topology.prompt.v1",
		"track_id":              strings.TrimSpace(trackID),
		"plugin_id":             strings.TrimSpace(pluginID),
		"topology_generation":   eqTopologyGenerationFromSummary(summary),
		"structural_only":       true,
		"llm_selects_acoustics": true,
	}
	sections := []map[string]any{}
	for _, section := range mapRowsValue(summary["sections"]) {
		row := map[string]any{
			"section": firstStringFromMap(section, "section"), "active": section["active"],
			"addressing":         firstStringFromMap(section, "addressing"),
			"dedicated_kind":     firstStringFromMap(section, "dedicated_kind"),
			"reachable_kinds":    eqStringSlice(section["reachable_kinds"]),
			"frequency_writable": len(mapRowsValue(section["frequency_bindings"])) > 0,
			"gain_writable":      len(mapRowsValue(section["gain_bindings"])) > 0,
			"q_writable":         len(mapRowsValue(section["q_bindings"])) > 0,
			"slope_writable":     len(mapRowsValue(section["slope_bindings"])) > 0,
		}
		if value, ok := eqBandFloat(section, "fixed_freq_hz"); ok {
			row["fixed_frequency_hz"] = value
		}
		capabilities := []map[string]any{}
		for _, capability := range mapRowsValue(section["shape_capabilities"]) {
			shape := firstStringFromMap(capability, "shape")
			if shape == "" {
				continue
			}
			capabilities = append(capabilities, map[string]any{
				"shape": shape, "actions": cloneContext(firstMapFromAny(capability["actions"])),
				"rejection_codes": cloneContext(firstMapFromAny(capability["rejection_codes"])),
			})
		}
		row["shape_capabilities"] = capabilities
		sections = append(sections, row)
	}
	out["sections"] = sections
	out["supported_filter_kinds"] = eqStringSlice(summary["supported_filter_kinds"])
	return out
}

// materializeAgentSemanticEQAction is the ordinary-Agent -> governance seam.
// It is deliberately read-only: the existing EQ recognizer/planner is reused
// to freeze a proposal, but no mutation authority enters this function.
func (s *Server) materializeAgentSemanticEQAction(ctx context.Context, conversationID string, req ChatRequest, mode string, res agentloop.Result) ChatResponse {
	goal := agentruntime.Goal{GoalID: res.GoalID, RunID: res.RunID, Status: agentruntime.StatusRunning}
	if res.SemanticAction == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "普通 Agent 没有提供可物化的语义 EQ 动作；没有创建方案或修改工程。")
	}
	action := *res.SemanticAction
	if err := action.Validate(); err != nil {
		return semanticEQRejectedResponse(conversationID, goal, "semantic_action_invalid", err.Error(), nil)
	}
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil || s.harness == nil {
		return semanticEQRejectedResponse(conversationID, goal, "runtime_unavailable", "治理 Runtime、Kernel 或 Harness 不可用", nil)
	}
	if mismatch := semanticEQSelectedTargetMismatch(req.Context, action.Target); mismatch != "" {
		return semanticEQRejectedResponse(conversationID, goal, "selected_target_mismatch", mismatch, nil)
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return semanticEQRejectedResponse(conversationID, goal, "project_snapshot_unavailable", firstNonEmpty(errorText(err), "无法取得强工程快照"), nil)
	}
	materialized, err := s.planSemanticEQReadOnly(ctx, action)
	if err != nil {
		return semanticEQRejectedResponse(conversationID, goal, eqControlFailureCode(err), err.Error(), map[string]any{
			"track_id": action.Target.TrackID, "plugin_id": action.Target.PluginID,
		})
	}
	evidenceSummary := semanticEQRecentObservationSummary(res.RecentObservation)
	evidenceRefs := semanticEQEvidenceRefs(action)
	previousObservationID := strings.TrimSpace(action.Evidence.ObservationID)
	acousticMode := "unavailable"
	if semanticEQEvidenceSupportsSameTapPostFX(action, evidenceSummary) {
		acousticMode = "eligible"
	}
	dependencies := []string{
		"semantic-action:" + semanticEQHash(action),
		"evidence-decision:" + strings.TrimSpace(action.Evidence.Choice) + ":" + strings.TrimSpace(action.Evidence.Basis),
	}
	for _, ref := range evidenceRefs {
		dependencies = append(dependencies, "evidence:"+ref)
	}
	targetFingerprints := []string{
		"plugin:" + action.Target.TrackID + ":" + action.Target.PluginID,
		"eq-topology:" + materialized.DigestGeneration,
	}
	for _, row := range materialized.Preimage {
		targetFingerprints = append(targetFingerprints, fmt.Sprintf("eq-param:%s:%0.9f", firstStringFromMap(row, "param_id"), semanticEQNumber(row["normalized"])))
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: state, Guarantee: capabilityCanaryCutGuarantee(ctx, s.kernel),
		DependencyFingerprints: dependencies,
		TargetFingerprints:     targetFingerprints,
		ArtifactRefs:           evidenceRefs,
		ContractVersions: []string{
			"capability:" + agentSemanticEQCapabilityID,
			"action:" + semanticeffect.ActionSchema,
			"payload:" + semanticeffect.EQPlanSchema,
			"executor:plugin_grabber.apply_eq_edits",
		},
	})
	if err != nil || !cut.IsExecutable() {
		return semanticEQRejectedResponse(conversationID, goal, "strong_project_cut_unavailable", firstNonEmpty(errorText(err), "Kernel 未提供可执行的强 ProjectCut"), nil)
	}
	sessionID := s.nextCapabilitySessionID(conversationID, agentSemanticEQCapabilityID)
	session, err := s.orchestrationRuntime.StartAgentSemanticEQChatSession(sessionID, conversationID, cut.ProjectUUID, action.UserGoal, orchestration.InteractionPropose)
	if err != nil {
		return semanticEQRejectedResponse(conversationID, goal, "session_create_failed", err.Error(), nil)
	}
	proposal, actionSet, err := capabilityadapters.FreezeSemanticEQ(capabilityadapters.SemanticEQPlan{
		Action: action, TopologyGeneration: materialized.DigestGeneration,
		Edits: materialized.Edits, PlannedEdits: materialized.PlannedEdits,
		PlannedWrites: materialized.PlannedWrites, ParameterPreimage: materialized.Preimage,
		ParameterSnapshot: materialized.Snapshot, PreviewResults: materialized.PreviewResults,
		EvidenceSummary: evidenceSummary, EvidenceRefs: evidenceRefs,
		PreviousObservation: previousObservationID, AcousticVerification: acousticMode,
	}, cut, 1)
	if err != nil {
		_, _ = s.orchestrationRuntime.CancelPlanningSession(session.ID)
		return semanticEQRejectedResponse(conversationID, goal, "freeze_failed", err.Error(), nil)
	}
	proposal.Presentation = semanticEQProposalPresentation(proposal, actionSet, action, materialized, acousticMode)
	session, err = s.orchestrationRuntime.AttachFrozenPlan(session.ID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut,
		ContextBundleID:       "bundle_semantic_eq_" + sanitizeCanaryID(session.ID),
		PreviousObservationID: previousObservationID,
		FrozenAt:              time.Now().UTC(),
	})
	if err != nil {
		_, _ = s.orchestrationRuntime.CancelPlanningSession(session.ID)
		return semanticEQRejectedResponse(conversationID, goal, "proposal_persist_failed", err.Error(), nil)
	}
	response := semanticEQProposalResponse(conversationID, goal, session)
	response.AgentMode = mode
	response.GoalSummary = action.UserGoal
	response.CompletedSteps = res.CompletedSteps
	return response
}

func (s *Server) planSemanticEQReadOnly(ctx context.Context, action semanticeffect.Action) (semanticEQMaterialization, error) {
	edits := semanticEQEditRows(action)
	requests, err := parseEQEditRequests(map[string]any{"edits": edits})
	if err != nil {
		return semanticEQMaterialization{}, err
	}
	digest, summary, err := s.readLiveEQControlSurface(ctx, action.Target.TrackID, action.Target.PluginID)
	if err != nil {
		return semanticEQMaterialization{}, err
	}
	generation := eqTopologyGenerationFromSummary(summary)
	if generation == "" {
		return semanticEQMaterialization{}, rejectEQControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	reserved := map[string]bool{}
	plans := make([]eqPlannedEdit, 0, len(requests))
	for index, request := range requests {
		planned, planErr := planEQEdit(summary, request, action.Target.TrackID, action.Target.PluginID, generation, reserved)
		if planErr != nil {
			return semanticEQMaterialization{}, planErr
		}
		planned.Index = index
		reserved[planned.SectionKey] = true
		plans = append(plans, planned)
	}
	writes, err := combineAtomicEQWrites(plans)
	if err != nil {
		return semanticEQMaterialization{}, err
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return semanticEQMaterialization{}, rejectEQControl("preimage_unavailable", "%v", err)
	}
	atoms := action.EQPlan.Atoms
	plannedRows := make([]map[string]any, 0, len(plans))
	preview := make([]map[string]any, 0, len(plans))
	for index, plan := range plans {
		atomID := ""
		if index < len(atoms) {
			atomID = atoms[index].AtomID
		}
		plannedRows = append(plannedRows, semanticEQPlannedEditMap(plan, atomID))
		preview = append(preview, map[string]any{
			"index": index, "atom_id": atomID, "status": map[bool]string{true: "quantized", false: "exact"}[plan.Quantized],
			"shape": plan.Shape, "section": plan.SectionKey, "requested": cloneContext(plan.Requested),
			"selection": cloneContext(plan.Selection), "control_ref": plan.ControlRef,
		})
	}
	return semanticEQMaterialization{
		DigestGeneration: generation,
		Edits:            edits,
		PlannedEdits:     plannedRows,
		PlannedWrites:    semanticEQWriteRows(writes),
		Preimage:         semanticEQPreimageRows(preimage),
		Snapshot:         semanticEQPreimageRows(eqParameterSnapshot(digest)),
		PreviewResults:   preview,
	}, nil
}

func semanticEQEditRows(action semanticeffect.Action) []map[string]any {
	if action.EQPlan == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(action.EQPlan.Atoms))
	for _, atom := range action.EQPlan.Atoms {
		row := map[string]any{"action": atom.Action}
		if atom.Shape != "" {
			row["shape"] = atom.Shape
		}
		if atom.FrequencyHz != nil {
			row["frequency_hz"] = *atom.FrequencyHz
		}
		if atom.GainDB != nil {
			row["gain_db"] = *atom.GainDB
		}
		if atom.Q != nil {
			row["q"] = *atom.Q
		}
		if atom.SlopeDBPerOct != nil {
			row["slope_db_per_oct"] = *atom.SlopeDBPerOct
		}
		if atom.ControlRef != "" {
			row["control_ref"] = atom.ControlRef
		}
		if atom.OperationRef != "" {
			row["operation_ref"] = atom.OperationRef
		}
		out = append(out, row)
	}
	return out
}

func semanticEQPlannedEditMap(plan eqPlannedEdit, atomID string) map[string]any {
	return map[string]any{
		"index": plan.Index, "atom_id": atomID, "section": plan.SectionKey, "shape": plan.Shape,
		"quantized": plan.Quantized, "control_ref": plan.ControlRef,
		"requested": cloneContext(plan.Requested), "selection": cloneContext(plan.Selection),
		"writes": semanticEQWriteRows(plan.Writes),
	}
}

func semanticEQWriteRows(writes []eqWriteStep) []map[string]any {
	out := make([]map[string]any, 0, len(writes))
	for _, write := range writes {
		row := map[string]any{
			"param_id": write.ParamID, "normalized_value": write.NormalizedValue,
			"role": write.Role, "channel": write.Channel, "quantized": write.Quantized,
			"correction_low": write.CorrectionLow, "correction_high": write.CorrectionHigh,
			"physical_decreasing": write.PhysicalDecreasing, "requested_label": write.RequestedLabel,
			"expected_label": write.ExpectedLabel, "activation_last": write.ActivationLast,
			"physical_multiplier": write.PhysicalMultiplier, "discrete_physical": write.DiscretePhysical,
		}
		if write.RequestedPhysical != nil {
			row["requested_physical"] = *write.RequestedPhysical
		}
		out = append(out, row)
	}
	return out
}

func semanticEQPreimageRows(values []eqPreimageValue) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{"param_id": value.ParamID, "normalized": value.Normalized, "role": value.Role})
	}
	return out
}

func semanticEQPreimageValues(value any) ([]eqPreimageValue, error) {
	rows := mapRowsValue(value)
	out := make([]eqPreimageValue, 0, len(rows))
	for _, row := range rows {
		id := firstStringFromMap(row, "param_id")
		number, ok := firstNumericAny(row, "normalized")
		if id == "" || !ok {
			return nil, fmt.Errorf("invalid frozen EQ preimage")
		}
		out = append(out, eqPreimageValue{ParamID: id, Normalized: number, Role: firstStringFromMap(row, "role")})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("frozen EQ preimage is empty")
	}
	return out, nil
}

func semanticEQSelectedTargetMismatch(ctx map[string]any, target semanticeffect.Target) string {
	selectedTrack := firstStringFromMap(ctx, "selected_track_id")
	selectedPluginTrack := firstStringFromMap(ctx, "selected_plugin_track_id")
	selectedPlugin := firstStringFromMap(ctx, "selected_plugin_id")
	if selectedTrack != "" && selectedTrack != strings.TrimSpace(target.TrackID) {
		return fmt.Sprintf("LLM 目标轨道 %s 与当前选择 %s 不一致", target.TrackID, selectedTrack)
	}
	if selectedPluginTrack != "" && selectedPluginTrack != strings.TrimSpace(target.TrackID) {
		return fmt.Sprintf("LLM 目标轨道 %s 与所选插件所在轨道 %s 不一致", target.TrackID, selectedPluginTrack)
	}
	if selectedPlugin != "" && selectedPlugin != strings.TrimSpace(target.PluginID) {
		return fmt.Sprintf("LLM 目标插件 %s 与当前选择 %s 不一致", target.PluginID, selectedPlugin)
	}
	return ""
}

func semanticEQRecentObservationSummary(observation *agentloop.RecentObservation) map[string]any {
	if observation == nil {
		return nil
	}
	return map[string]any{
		"tool_call_id": observation.ToolCallID, "tool": observation.Tool,
		"command_name": observation.CommandName, "status": observation.Status,
		"summary": cloneContext(observation.Summary), "mutation_barrier": observation.MutationBarrier,
	}
}

func semanticEQEvidenceRefs(action semanticeffect.Action) []string {
	refs := append([]string(nil), action.Evidence.EvidenceRefs...)
	if id := strings.TrimSpace(action.Evidence.ObservationID); id != "" {
		refs = append(refs, "mix.observe:"+id)
	}
	if action.EQPlan != nil {
		for _, atom := range action.EQPlan.Atoms {
			refs = append(refs, atom.EvidenceRefs...)
		}
	}
	return semanticEQUniqueStrings(refs)
}

func semanticEQEvidenceSupportsSameTapPostFX(action semanticeffect.Action, summary map[string]any) bool {
	if action.Evidence.Basis != "observation" && action.Evidence.Basis != "both" {
		return false
	}
	texts := semanticEQRecursiveText(summary)
	hasTap, hasRevision := false, false
	for key, values := range texts {
		for _, value := range values {
			lower := strings.ToLower(value)
			if key == "tap_point" && (strings.Contains(lower, "post_fader") || strings.Contains(lower, "post_fx") || strings.Contains(lower, "master_out")) {
				hasTap = true
			}
			if (key == "render_revision" || key == "source_revision") && strings.TrimSpace(value) != "" {
				hasRevision = true
			}
		}
	}
	return hasTap && hasRevision && strings.TrimSpace(action.Evidence.ObservationID) != ""
}

func semanticEQRecursiveText(value any) map[string][]string {
	out := map[string][]string{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, item := range typed {
				if text, ok := item.(string); ok {
					out[strings.ToLower(key)] = append(out[strings.ToLower(key)], text)
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case []map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

func semanticEQProposalPresentation(proposal orchestration.Proposal, actionSet orchestration.ActionSet, action semanticeffect.Action, planned semanticEQMaterialization, acousticMode string) *orchestration.ProposalPresentation {
	analysis := []string{fmt.Sprintf("观察选择：%s（%s）— %s", action.Evidence.Choice, action.Evidence.Basis, action.Evidence.Reason)}
	actions := make([]orchestration.ProposalActionPreview, 0, len(action.EQPlan.Atoms))
	for index, atom := range action.EQPlan.Atoms {
		status := "exact"
		if index < len(planned.PreviewResults) {
			status = firstStringFromMap(planned.PreviewResults[index], "status")
		}
		line := semanticEQAtomText(atom, status, nil)
		analysis = append(analysis, line)
		frequency := 0.0
		if atom.FrequencyHz != nil {
			frequency = *atom.FrequencyHz
		}
		delta := 0.0
		if atom.GainDB != nil {
			delta = *atom.GainDB
		}
		actions = append(actions, orchestration.ProposalActionPreview{ActionID: atom.AtomID, TrackID: action.Target.TrackID, TrackName: action.Target.TrackName, Operation: atom.Action + ":" + atom.Shape, Target: frequency, Delta: delta, Unit: "Hz/dB", Reason: atom.Purpose + "；preview=" + status})
	}
	limitations := append([]string(nil), action.Limitations...)
	if acousticMode != "eligible" {
		limitations = append(limitations, "当前证据不是可信的同 tap post-FX 证据；执行后只保证结构读回，声学验证将报告 unavailable。")
	}
	return &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: proposal.ID, ProposalRevision: proposal.Revision,
		CapabilityID: proposal.CapabilityID, Title: "普通 Agent 抽象 EQ 方案",
		Conclusion:      fmt.Sprintf("目标：%s。拟对轨道 %s 的插件 %s 执行 %d 个共同授权的原子 EQ 动作。", action.UserGoal, action.Target.TrackID, action.Target.PluginID, len(action.EQPlan.Atoms)),
		AnalysisSummary: analysis, Recommendation: proposal.Summary, ActionCount: len(actionSet.Actions), Risk: proposal.Risk, Reversible: true,
		Readiness: []orchestration.ProposalMetric{
			{ID: "topology", Label: "EQ topology", Value: planned.DigestGeneration, Status: "frozen"},
			{ID: "preimage", Label: "参数预映像", Value: fmt.Sprintf("%d parameters", len(planned.Preimage)), Status: "frozen"},
			{ID: "acoustic_verification", Label: "声学后验证", Value: acousticMode, Status: acousticMode},
		},
		Actions: actions, Limitations: semanticEQUniqueStrings(limitations), EvidenceRefs: semanticEQEvidenceRefs(action),
		ApprovalPrompt: "是否授权执行这个冻结的 EQ 方案？确认前不会写入插件。",
	}
}

func semanticEQProposalResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession) ChatResponse {
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "语义 EQ Session 缺少冻结 Proposal。")
	}
	proposal := session.ActiveProposal
	action, _ := semanticEQActionFromAny(session.FrozenPlan.ActionSet.Actions[0].Args["semantic_action"])
	preview := mapRowsValue(session.FrozenPlan.ActionSet.Actions[0].Args["preview_results"])
	reply := semanticEQProposalText(*proposal, action, preview)
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply: reply, NeedsConfirmation: true, PlanID: proposal.ID,
		Preview: proposal.Presentation.Conclusion, ProposalPresentation: proposal.Presentation,
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusWaitingConfirmation),
		WorkflowData: map[string]any{
			"session_id": session.ID, "capability_id": session.Invocation.CapabilityID,
			"proposal_id": proposal.ID, "proposal_revision": proposal.Revision,
			"action_set_hash": proposal.ActionSetHash, "project_cut_hash": proposal.ProjectCutHash,
			"stage": "proposal", "semantic_action": action, "preview_results": preview,
			"writes_performed": 0, "approval_mode": "conversational", "proposal_presentation": proposal.Presentation,
		},
	}
}

func semanticEQProposalText(proposal orchestration.Proposal, action semanticeffect.Action, preview []map[string]any) string {
	lines := []string{"已生成普通 Agent 抽象 EQ 方案；目前没有写入插件。", "", fmt.Sprintf("目标：%s", action.UserGoal)}
	if len(action.Constraints) > 0 {
		lines = append(lines, "共同约束："+strings.Join(action.Constraints, "；"))
	}
	for index, atom := range action.EQPlan.Atoms {
		status := "exact"
		if index < len(preview) {
			status = firstStringFromMap(preview[index], "status")
		}
		lines = append(lines, "- "+semanticEQAtomText(atom, status, nil))
	}
	lines = append(lines, "", fmt.Sprintf("Proposal %s · revision %d。确认后才会执行；执行时会重新检查 topology 和完整参数预映像。", proposal.ID, proposal.Revision))
	return strings.Join(lines, "\n")
}

func semanticEQRejectedResponse(conversationID string, goal agentruntime.Goal, code, message string, details map[string]any) ChatResponse {
	data := cloneContext(details)
	if data == nil {
		data = map[string]any{}
	}
	data["capability_id"] = agentSemanticEQCapabilityID
	data["stage"] = "preflight_rejected"
	data["status"] = "rejected"
	data["rejection_code"] = code
	data["writes_performed"] = 0
	return ChatResponse{
		ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID,
		Reply:    fmt.Sprintf("EQ 方案在写入前被拒绝（%s）：%s。没有修改任何插件参数。", code, message),
		Workflow: "capability_runtime_v1", WorkflowData: data,
		GoalStatus: string(agentruntime.StatusFailed), Error: message,
	}
}

// semanticEQMutationPort bridges the frozen governance contract to the
// existing generic applyPluginGrabberEQEdits executor. Preflight recomputes
// the deterministic plan and rejects any topology/preimage drift before the
// generic executor is called.
type semanticEQStateReader interface {
	VSPStateSnapshot(context.Context, string) (*kernel.VSPStateResult, error)
}

type semanticEQMutationPort struct {
	server      *Server
	stateReader semanticEQStateReader
}

func (p *semanticEQMutationPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.server == nil {
		return fmt.Errorf("semantic EQ server is required")
	}
	reader := p.stateReader
	if reader == nil {
		reader = p.server.kernel
	}
	if reader == nil {
		return fmt.Errorf("semantic EQ state reader is required")
	}
	if actionSet.CapabilityID != agentSemanticEQCapabilityID || actionSet.Hash == "" || actionSet.Hash != actionSet.ComputeHash() || actionSet.ProjectCutHash != cut.Hash || !cut.IsExecutable() {
		return fmt.Errorf("invalid frozen semantic EQ action set/project cut")
	}
	if len(actionSet.Actions) != 1 || actionSet.Actions[0].Command != "plugin_grabber.apply_eq_edits.governed" || !actionSet.Actions[0].Compensatable {
		return fmt.Errorf("unsupported semantic EQ action")
	}
	baseRevision, err := strconv.ParseInt(strings.TrimSpace(cut.BaseProjectRevision), 10, 64)
	if err != nil || baseRevision <= 0 {
		return fmt.Errorf("invalid frozen base revision")
	}
	state, err := reader.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return fmt.Errorf("read current VSP snapshot: %w", err)
	}
	if state.ProjectEpoch != cut.ProjectEpoch || state.Revision != baseRevision {
		return fmt.Errorf("stale_project_cut: current epoch/revision does not match proposal")
	}
	action, err := semanticEQActionFromAny(actionSet.Actions[0].Args["semantic_action"])
	if err != nil {
		return err
	}
	current, err := p.server.planSemanticEQReadOnly(ctx, action)
	if err != nil {
		return err
	}
	args := actionSet.Actions[0].Args
	if current.DigestGeneration != firstStringFromMap(args, "topology_generation") {
		return fmt.Errorf("stale_eq_topology: generation changed")
	}
	checks := []struct {
		name    string
		frozen  any
		current any
	}{
		{"edits", args["edits"], current.Edits}, {"planned_edits", args["planned_edits"], current.PlannedEdits},
		{"planned_writes", args["planned_writes"], current.PlannedWrites}, {"parameter_preimage", args["parameter_preimage"], current.Preimage},
	}
	for _, check := range checks {
		if semanticEQHash(check.frozen) != semanticEQHash(check.current) {
			return fmt.Errorf("stale_eq_%s: frozen plan no longer matches live control surface", check.name)
		}
	}
	return nil
}

func (p *semanticEQMutationPort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	if p == nil || p.server == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("semantic EQ server is required")
	}
	trackID := firstStringFromMap(action.Args, "track_id")
	pluginID := firstStringFromMap(action.Args, "plugin_id")
	command := map[string]any{"cmd": pluginGrabberApplyEQEditsCommand, "track_id": trackID, "plugin_id": pluginID, "atomic": true, "edits": cloneContextRows(mapRowsValue(action.Args["edits"]))}
	result, applyErr := p.server.applyPluginGrabberEQEdits(ctx, command, map[string]any{
		"selected_track_id": trackID, "selected_plugin_track_id": trackID, "selected_plugin_id": pluginID,
		"capability_runtime_v1": true, "capability_id": agentSemanticEQCapabilityID,
	})
	details := map[string]any{
		"track_id": trackID, "plugin_id": pluginID, "idempotency_key": idempotencyKey,
		"result": cloneContext(result), "structural_readback": "fail", "writes_performed": 0,
	}
	actionPlan, _ := semanticEQActionFromAny(action.Args["semantic_action"])
	if result != nil {
		details["atom_results"] = semanticEQAtomResultRows(actionPlan, mapRowsValue(result["edits"]))
	}
	if applyErr != nil {
		details["status"] = "rejected"
		details["rejection_code"] = eqControlFailureCode(applyErr)
		details["failure"] = applyErr.Error()
		rollback := map[string]any{"attempted": false, "status": "not_needed_no_write"}
		if strings.Contains(strings.ToLower(applyErr.Error()), "preimage restored") {
			rollback = map[string]any{"attempted": true, "status": "restored"}
		}
		details["rollback"] = rollback
		receipt := orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: applyErr.Error(), Details: details, EvidenceRefs: []string{"plugin_grabber.apply_eq_edits:" + idempotencyKey}}
		return receipt, applyErr
	}
	details["status"] = firstStringFromMap(result, "status")
	details["structural_readback"] = "pass"
	details["writes_performed"] = len(mapRowsValue(result["writes"]))
	details["operation_ref"] = firstStringFromMap(result, "operation_ref")
	details["rollback"] = cloneContext(firstMapFromAny(result["rollback"]))
	appliedRevision := ""
	if p.server.kernel != nil {
		if state, err := p.server.kernel.VSPStateSnapshot(ctx, "project.timeline"); err == nil && state != nil && state.OK() {
			appliedRevision = strconv.FormatInt(state.Revision, 10)
		}
	}
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", AppliedRevision: appliedRevision, EffectivelyOnce: true,
		EvidenceRefs: []string{"plugin_grabber.apply_eq_edits:" + idempotencyKey, "plugin.parameters.readback:" + idempotencyKey}, Details: details,
	}, nil
}

func (p *semanticEQMutationPort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, _ orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	if p == nil || p.server == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("semantic EQ server is required")
	}
	trackID, pluginID := firstStringFromMap(action.Args, "track_id"), firstStringFromMap(action.Args, "plugin_id")
	rows, readErr := p.server.readEQParameters(ctx, trackID, pluginID)
	index := eqParameterRowIndex(rows)
	writes := mapRowsValue(action.Args["planned_writes"])
	matching := readErr == nil && len(writes) > 0
	for _, write := range writes {
		id := firstStringFromMap(write, "param_id")
		expected, expectedOK := firstNumericAny(write, "normalized_value")
		actual, actualOK := firstNumericAny(index[id], "normalized_value", "normalised_value", "current_normalized_value")
		if id == "" || !expectedOK || !actualOK || math.Abs(expected-actual) > 1e-4 {
			matching = false
			break
		}
	}
	details := map[string]any{"fresh_readback": rows, "structural_readback": map[bool]string{true: "pass", false: "fail"}[matching], "recovery": true}
	if matching {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "applied", EffectivelyOnce: true, Details: details, EvidenceRefs: []string{"plugin.parameters.reconcile:" + idempotencyKey}}, nil
	}
	preimage, decodeErr := semanticEQPreimageValues(action.Args["parameter_preimage"])
	if decodeErr != nil {
		details["rollback"] = map[string]any{"status": "failed", "error": decodeErr.Error()}
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed_closed", Error: decodeErr.Error(), Details: details}, decodeErr
	}
	rollbackErr := p.server.rollbackEQTransaction(ctx, trackID, pluginID, preimage)
	if rollbackErr != nil {
		details["rollback"] = map[string]any{"status": "failed", "error": rollbackErr.Error()}
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed_closed", Error: rollbackErr.Error(), Details: details}, rollbackErr
	}
	details["rollback"] = map[string]any{"status": "restored"}
	err := fmt.Errorf("ambiguous semantic EQ outcome; frozen preimage restored")
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: err.Error(), Details: details}, err
}

type semanticEQVerifier struct {
	server                                     *Server
	previousObservationID, sessionID, goalText string
}

func (v semanticEQVerifier) Verify(ctx context.Context, actionSet orchestration.ActionSet, receipts []orchestration.ActionReceipt) (orchestration.VerificationResult, error) {
	result := orchestration.VerificationResult{Status: "inconclusive", Structural: "inconclusive", Acoustic: "not_run", UserAcceptance: "unknown"}
	if len(actionSet.Actions) != 1 || len(receipts) != 1 || !strings.EqualFold(receipts[0].Status, "applied") || !strings.EqualFold(firstStringFromMap(receipts[0].Details, "structural_readback"), "pass") {
		result.Status, result.Structural = "fail", "fail"
		return result, fmt.Errorf("semantic EQ structural receipt/readback failed")
	}
	result.Structural = "pass"
	result.EvidenceRefs = append(result.EvidenceRefs, receipts[0].EvidenceRefs...)
	mode := firstStringFromMap(actionSet.Actions[0].Args, "acoustic_verification")
	if mode != "eligible" {
		result.Status, result.Acoustic = "pass", "unavailable"
		result.Summary = "结构参数读回通过；前序证据不是可信的同 tap post-FX 证据，因此没有声称声学目标已验证；用户听感验收仍为 unknown。"
		return result, nil
	}
	if v.server == nil || v.server.harness == nil || strings.TrimSpace(v.previousObservationID) == "" {
		result.Acoustic = "unavailable"
		result.Summary = "结构参数读回通过，但缺少可执行的同 tap 声学验证上下文。"
		return result, nil
	}
	trackID := firstStringFromMap(actionSet.Actions[0].Args, "track_id")
	pluginID := firstStringFromMap(actionSet.Actions[0].Args, "plugin_id")
	response, err := v.server.harness.Invoke(ctx, harness.InvokeRequest{
		Tool: "mix.observe", Args: map[string]any{
			"scope": "full_project_with_focus_track", "track_id": trackID, "project_context": true, "observation_only": true,
			"disclosure": "digest_catalog", "mom_intent": mom.IntentProjectMultitrackObservation,
			"previous_observation": v.previousObservationID, "mix_session_id": v.sessionID,
			"goal_text":  v.goalText,
			"focus_hint": map[string]any{"track_id": trackID, "source": "frozen_semantic_eq_target"},
			"target_ref": map[string]any{"kind": "track", "id": trackID, "track_id": trackID, "plugin_id": pluginID},
		},
		Context: map[string]any{"capability_runtime_v1": true, "observation_only": true, "ordinary_agent_semantic_eq_verifier": true},
		Source:  "capability_runtime_v1_semantic_eq_verifier", Confirmed: true,
		ToolCallID: "verify:" + actionSet.ID + ":mix.observe",
	})
	if err != nil || !strings.EqualFold(response.Status, "ok") {
		result.Acoustic = "inconclusive"
		result.Summary = "结构参数读回通过，但 fresh 同 tap mix.observe 不可用：" + firstNonEmpty(errorText(err), response.Error, response.Status)
		return result, nil
	}
	afterID := firstStringFromMap(response.Result, "observation_id")
	beforeTexts := semanticEQRecursiveText(actionSet.Actions[0].Args["evidence_summary"])
	afterTexts := semanticEQRecursiveText(response.Result)
	beforeTap, afterTap := semanticEQFirstRecursive(beforeTexts, "tap_point"), semanticEQFirstRecursive(afterTexts, "tap_point")
	beforeRevision, afterRevision := semanticEQFirstRecursive(beforeTexts, "render_revision"), semanticEQFirstRecursive(afterTexts, "render_revision")
	result.EvidenceRefs = append(result.EvidenceRefs, "mix.observe:"+afterID)
	if afterID == "" || afterID == v.previousObservationID || beforeTap == "" || !strings.EqualFold(beforeTap, afterTap) || beforeRevision == "" || afterRevision == "" || beforeRevision == afterRevision {
		result.Acoustic = "inconclusive"
		result.Summary = "结构参数读回通过，但 post-execution 观察未证明 fresh same-tap post-FX 身份；用户听感验收仍为 unknown。"
		return result, nil
	}
	semanticEQMarkEvidenceRefreshPass(&result, afterID, afterTap)
	return result, nil
}

func semanticEQMarkEvidenceRefreshPass(result *orchestration.VerificationResult, observationID, tap string) {
	if result == nil {
		return
	}
	result.Status = "inconclusive"
	result.Acoustic = "inconclusive"
	result.Summary = fmt.Sprintf("结构参数读回通过；evidence_refresh=pass，fresh observation %s 保持 tap %s 且 render revision 已更新。这里只证明执行后证据身份有效，尚未证明听感目标得到改善；用户听感验收仍为 unknown。", observationID, tap)
}

func (s *Server) handleSemanticEQRuntime(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal) ChatResponse {
	if s == nil || s.orchestrationRuntime == nil || s.kernel == nil || s.harness == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "语义 EQ 治理 Runtime 不可用；没有修改工程。")
	}
	sessionID := firstStringFromMap(req.Context, "capability_session_id")
	session, ok := s.orchestrationRuntime.Store.Load(sessionID)
	if !ok || session.Invocation.CapabilityID != agentSemanticEQCapabilityID {
		return capabilityCanaryBlockedResponse(conversationID, goal, "找不到绑定的普通 Agent EQ Proposal；没有修改工程。")
	}
	if !expectedProposalMatches(req.Context, session.ActiveProposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "确认交互绑定的 EQ Proposal 已过期；没有授权或修改工程。")
	}
	if session.Status == orchestration.StatusExecuting || session.Status == orchestration.StatusVerifying {
		return s.recoverCapabilityExecution(ctx, conversationID, goal, session)
	}
	state, err := s.kernel.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil || state == nil || !state.OK() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "无法取得当前工程快照；EQ Proposal 未获授权。")
	}
	decision, hasDecision := capabilityApprovalDecisionFromContext(req.Context)
	if session.ActiveProposal != nil && hasDecision && decision.Kind == orchestration.ApprovalApprove {
		return s.authorizeSemanticEQ(ctx, conversationID, req, goal, session, state)
	}
	if session.Status == orchestration.StatusAuthorized {
		return authorizedCapabilityWaitingResponse(conversationID, goal, session)
	}
	return semanticEQProposalResponse(conversationID, goal, session)
}

func (s *Server) authorizeSemanticEQ(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, session orchestration.PlanningSession, current *kernel.VSPStateResult) ChatResponse {
	if session.FrozenPlan == nil || session.ActiveProposal == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "EQ Session 缺少 FrozenPlan；没有执行。")
	}
	frozen := *session.FrozenPlan
	if current == nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "EQ 授权缺少 VSP 状态；没有执行。")
	}
	baseRevision, _ := strconv.ParseInt(frozen.ProjectCut.BaseProjectRevision, 10, 64)
	if capabilityCanaryCutGuarantee(ctx, s.kernel) != projectcut.GuaranteeKernelBarrier || !frozen.ProjectCut.IsExecutable() || current.ProjectEpoch != frozen.ProjectCut.ProjectEpoch || current.Revision != baseRevision {
		return capabilityCanaryBlockedResponse(conversationID, goal, "工程 revision/epoch 或 Kernel CAS 已变化；旧 EQ Proposal 未获授权，也没有写入。")
	}
	if !s.orchestrationRuntime.HasDurableStore() {
		return capabilityCanaryBlockedResponse(conversationID, goal, "v1 Session Store 非持久化；EQ Proposal 未获授权。")
	}
	decision, ok := capabilityApprovalDecisionFromContext(req.Context)
	if !ok || !decision.ExactApprovalFor(frozen.Proposal) {
		return capabilityCanaryBlockedResponse(conversationID, goal, "当前消息没有形成绑定此 EQ Proposal revision 的明确授权。")
	}
	authorized, err := s.orchestrationRuntime.AuthorizeProposal(session.ID, orchestration.Authorization{
		ProposalID: frozen.Proposal.ID, ProposalRevision: frozen.Proposal.Revision,
		ActionSetHash: frozen.Proposal.ActionSetHash, ProjectCutHash: frozen.Proposal.ProjectCutHash,
		Scope: append([]string(nil), frozen.Proposal.TargetScope...), SourceTurnID: decision.SourceTurnID,
		Sequence: session.Revision + 1, Decision: &decision,
	})
	if err != nil {
		return capabilityCanaryBlockedResponse(conversationID, goal, "EQ 授权绑定失败："+err.Error())
	}
	executed, executeErr := s.orchestrationRuntime.ExecuteActionSetWithPersistence(
		ctx, session.ID, frozen.ActionSet, frozen.ProjectCut,
		&semanticEQMutationPort{server: s},
		semanticEQVerifier{server: s, previousObservationID: frozen.PreviousObservationID, sessionID: session.ID, goalText: session.Goal},
		executionports.ProjectHistory{Harness: s.harness, GoalID: firstNonEmpty(goal.GoalID, session.ID), RunID: goal.RunID},
	)
	if executed.ID == "" {
		executed = authorized
	}
	if executeErr != nil && executed.Status == orchestration.StatusAuthorized {
		if cancelled, cancelErr := s.orchestrationRuntime.CancelPlanningSession(session.ID); cancelErr == nil {
			executed = cancelled
		}
	}
	response := semanticEQExecutionResponse(conversationID, goal, executed, executeErr)
	response.ProjectHistory = s.harness.ProjectHistorySummary(ctx, firstNonEmpty(goal.GoalID, session.ID))
	return response
}

func semanticEQExecutionResponse(conversationID string, goal agentruntime.Goal, session orchestration.PlanningSession, executeErr error) ChatResponse {
	data := map[string]any{"session_id": session.ID, "capability_id": agentSemanticEQCapabilityID, "user_acceptance": "unknown"}
	if session.ActiveProposal != nil {
		data["proposal_id"] = session.ActiveProposal.ID
		data["proposal_revision"] = session.ActiveProposal.Revision
		data["action_set_hash"] = session.ActiveProposal.ActionSetHash
		data["project_cut_hash"] = session.ActiveProposal.ProjectCutHash
	}
	status := string(agentruntime.StatusFailed)
	reply := "EQ 执行在写入前被安全门拒绝；没有参数写入。"
	if executeErr != nil {
		data["error"] = executeErr.Error()
	}
	if session.Execution != nil {
		data["execution_id"] = session.Execution.ID
		data["execution_status"] = session.Execution.Status
		data["receipts"] = session.Execution.Receipts
		data["verification"] = session.Execution.VerificationResult
		if len(session.Execution.Receipts) > 0 {
			receipt := session.Execution.Receipts[0]
			data["result"] = receipt.Details
			reply = semanticEQReceiptText(receipt, session.Execution.VerificationResult)
			if strings.EqualFold(receipt.Status, "applied") {
				status = string(agentruntime.StatusCompleted)
				if session.Status == orchestration.StatusNeedsReview {
					status = string(agentruntime.StatusWaitingContinue)
				}
			}
		}
	}
	response := ChatResponse{ConversationID: conversationID, GoalID: goal.GoalID, RunID: goal.RunID, Reply: reply, Workflow: "capability_runtime_v1", WorkflowData: data, GoalStatus: status}
	if executeErr != nil && session.Execution != nil && len(session.Execution.Receipts) > 0 && !strings.EqualFold(session.Execution.Receipts[0].Status, "applied") {
		response.Error = executeErr.Error()
	}
	return response
}

func semanticEQReceiptText(receipt orchestration.ActionReceipt, verification *orchestration.VerificationResult) string {
	details := receipt.Details
	result := firstMapFromAny(details["result"])
	resultStatus := firstNonEmpty(firstStringFromMap(result, "status"), firstStringFromMap(details, "status"))
	rejectionCode := firstNonEmpty(firstStringFromMap(details, "rejection_code"), firstStringFromMap(result, "rejection_code"))
	if resultStatus == "" {
		if rejectionCode != "" {
			resultStatus = "rejected"
		} else {
			resultStatus = firstNonEmpty(strings.ToLower(strings.TrimSpace(receipt.Status)), "unknown")
		}
	}
	prefix := "EQ 执行结果"
	if strings.EqualFold(receipt.Status, "applied") {
		prefix = "EQ 已执行"
	}
	lines := []string{fmt.Sprintf("%s：%s。", prefix, resultStatus)}
	if strings.EqualFold(receipt.Status, "applied") && resultStatus != "exact" && resultStatus != "quantized" {
		lines = append(lines, "限制：执行回执没有报告 exact 或 quantized，不能推定为 exact。")
	}
	if rejectionCode != "" {
		lines = append(lines, "rejection_code："+rejectionCode)
	}
	for _, atom := range mapRowsValue(details["atom_results"]) {
		lines = append(lines, "- "+semanticEQAtomResultText(atom))
	}
	if op := firstStringFromMap(result, "operation_ref"); op != "" {
		lines = append(lines, "operation_ref："+op)
	}
	rollback := firstMapFromAny(details["rollback"])
	if len(rollback) == 0 {
		rollback = firstMapFromAny(result["rollback"])
	}
	if len(rollback) > 0 {
		parts := make([]string, 0, 6)
		if status := firstStringFromMap(rollback, "status"); status != "" {
			parts = append(parts, status)
		} else if available, ok := rollback["available"]; ok {
			parts = append(parts, "available="+fmt.Sprint(available))
			if operationRef := firstStringFromMap(rollback, "operation_ref"); operationRef != "" {
				parts = append(parts, "operation_ref="+operationRef)
			}
			if lifetime := firstStringFromMap(rollback, "lifetime"); lifetime != "" {
				parts = append(parts, "lifetime="+lifetime)
			}
			if expiresOnRestart, exists := rollback["expires_on_restart"]; exists {
				parts = append(parts, "expires_on_restart="+fmt.Sprint(expiresOnRestart))
			}
		}
		if attempted, ok := rollback["attempted"]; ok {
			parts = append(parts, "attempted="+fmt.Sprint(attempted))
		}
		if rollbackErr := firstStringFromMap(rollback, "error"); rollbackErr != "" {
			parts = append(parts, "error="+rollbackErr)
		}
		if len(parts) > 0 {
			lines = append(lines, "rollback："+strings.Join(parts, "，"))
		}
	}
	if receiptErr := strings.TrimSpace(receipt.Error); receiptErr != "" {
		lines = append(lines, "error："+receiptErr)
	}
	if verification != nil {
		lines = append(lines, fmt.Sprintf("验证：structural=%s，acoustic=%s，user_acceptance=%s。%s", verification.Structural, verification.Acoustic, verification.UserAcceptance, verification.Summary))
	}
	return strings.Join(lines, "\n")
}

func semanticEQAtomResultRows(action semanticeffect.Action, editResults []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(editResults))
	for index, edit := range editResults {
		row := cloneContext(edit)
		if index < len(action.EQPlan.Atoms) {
			row["atom_id"] = action.EQPlan.Atoms[index].AtomID
			row["purpose"] = action.EQPlan.Atoms[index].Purpose
		}
		out = append(out, row)
	}
	return out
}

func semanticEQAtomText(atom semanticeffect.EQAtom, status string, actual []map[string]any) string {
	parts := []string{atom.AtomID, atom.Action, atom.Shape}
	if atom.FrequencyHz != nil {
		parts = append(parts, fmt.Sprintf("%.1f Hz", *atom.FrequencyHz))
	}
	if atom.GainDB != nil {
		parts = append(parts, fmt.Sprintf("%+.2f dB", *atom.GainDB))
	}
	if atom.Q != nil {
		parts = append(parts, fmt.Sprintf("Q %.2f", *atom.Q))
	}
	if atom.SlopeDBPerOct != nil {
		parts = append(parts, fmt.Sprintf("%.1f dB/oct", *atom.SlopeDBPerOct))
	}
	parts = append(parts, "preview="+status, atom.Purpose)
	if len(actual) > 0 {
		parts = append(parts, fmt.Sprintf("readback=%d fields", len(actual)))
	}
	return strings.Join(parts, " · ")
}

func semanticEQAtomResultText(row map[string]any) string {
	requested := firstMapFromAny(row["requested"])
	parts := make([]string, 0, 10)
	for _, value := range []string{firstStringFromMap(row, "atom_id"), firstStringFromMap(row, "status"), firstStringFromMap(row, "shape")} {
		if value != "" {
			parts = append(parts, value)
		}
	}
	if v, ok := firstNumericAny(requested, "frequency_hz"); ok {
		parts = append(parts, fmt.Sprintf("requested %.1f Hz", v))
	}
	if v, ok := firstNumericAny(requested, "gain_db"); ok {
		parts = append(parts, fmt.Sprintf("%+.2f dB", v))
	}
	if v, ok := firstNumericAny(requested, "q"); ok {
		parts = append(parts, fmt.Sprintf("Q %.2f", v))
	}
	if v, ok := firstNumericAny(requested, "slope_db_per_oct"); ok {
		parts = append(parts, fmt.Sprintf("slope %.1f dB/oct", v))
	}
	if rejectionCode := firstStringFromMap(row, "rejection_code"); rejectionCode != "" {
		parts = append(parts, "rejection="+rejectionCode)
	}
	actual := mapRowsValue(row["actual_readback"])
	if len(actual) > 0 {
		readback := make([]string, 0, len(actual))
		for _, field := range actual {
			role := firstNonEmpty(firstStringFromMap(field, "role"), "field")
			value := firstStringFromMap(field, "value_text")
			if value == "" {
				if physical, ok := field["physical"]; ok && physical != nil {
					value = fmt.Sprint(physical)
				}
			}
			readback = append(readback, fmt.Sprintf("%s=%s", role, firstNonEmpty(value, "unknown")))
		}
		parts = append(parts, "actual "+strings.Join(readback, ", "))
	} else {
		parts = append(parts, "actual unavailable")
	}
	if purpose := firstStringFromMap(row, "purpose"); purpose != "" {
		parts = append(parts, "purpose="+purpose)
	}
	return strings.Join(parts, " · ")
}

func semanticEQActionFromAny(value any) (semanticeffect.Action, error) {
	var action semanticeffect.Action
	b, err := json.Marshal(value)
	if err == nil {
		err = json.Unmarshal(b, &action)
	}
	if err != nil {
		return action, fmt.Errorf("decode frozen semantic action: %w", err)
	}
	if err := action.Validate(); err != nil {
		return action, err
	}
	return action, nil
}

func semanticEQHash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func semanticEQNumber(value any) float64 { number, _ := numericAny(value); return number }
func semanticEQFirstRecursive(values map[string][]string, key string) string {
	if rows := values[key]; len(rows) > 0 {
		return rows[0]
	}
	return ""
}
func semanticEQUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
func cloneContextRows(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = cloneContext(row)
	}
	return out
}
