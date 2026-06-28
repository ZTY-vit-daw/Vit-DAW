package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
)

const (
	pluginPrepWorkerWorkflow               = "plugin_prep_worker"
	pluginPrepParameterTreatmentType       = "plugin_parameter_treatment"
	pluginPrepWorkerCandidateStopReason    = "plugin_prep_parameter_candidate_pending"
	pluginPrepWorkerAppliedStopReason      = "plugin_prep_parameter_treatment_applied_reobserved"
	pluginPrepWorkerTerminalNoMapping      = "plugin_prep_worker_terminal_insufficient_parameter_mapping"
	pluginPrepWorkerTerminalAmbiguousTrack = "plugin_prep_worker_waiting_target_clarification"
)

type pluginPrepWorkerControl struct {
	ParamID string
	Label   string
	Role    string
	Band    string
	Raw     map[string]any
}

func (s *Server) pluginPrepWorkerFromDigest(plan PendingPlan, trackID, pluginID, pluginName string, params map[string]any, message string, baseEvents []map[string]any) pluginPrepContinuation {
	conversationID := firstNonEmpty(cleanContextText(plan.Context["conversation_id"]), cleanContextText(plan.WorkflowData["conversation_id"]))
	goalID := cleanContextText(plan.Context["goal_id"])
	runID := cleanContextText(plan.Context["run_id"])
	intent := firstNonEmpty(cleanContextText(plan.WorkflowData["intent"]), cleanContextText(plan.Context["intent"]), cleanContextText(plan.Context["user_message"]), message)
	targetRef := pluginPrepWorkerTargetRef(trackID, plan.WorkflowData, plan.Context)
	if targetRef == "" {
		event := pluginPrepWorkerUserInputEvent(plan, "target_track", "需要先明确目标轨道，才能准备插件参数候选。确认前不会写入任何插件参数。")
		return pluginPrepContinuation{
			WorkflowData: pluginPrepWorkerBaseWorkflowData(plan, trackID, pluginID, pluginName, params, nil),
			TypedEvents:  append(baseEvents, event),
			GoalStatus:   string(agentruntime.StatusWaitingClarification),
			StopReason:   pluginPrepWorkerTerminalAmbiguousTrack,
		}
	}
	if trackID == "" {
		trackID = strings.TrimPrefix(targetRef, "track:")
	}

	selected, controlsOK := pluginPrepWorkerSelectEQControl(params)
	if !controlsOK {
		terminal := typedPluginPrepTerminalEvent(plan, trackID, pluginID, pluginName, "insufficient_parameter_mapping", "已取得插件参数摘要，但没有找到可靠的 EQ 增益控制映射，因此没有写入任何插件参数。")
		return pluginPrepContinuation{
			WorkflowData: pluginPrepWorkerBaseWorkflowData(plan, trackID, pluginID, pluginName, params, nil),
			TypedEvents:  append(baseEvents, terminal),
			GoalStatus:   string(agentruntime.StatusCompleted),
			StopReason:   pluginPrepWorkerTerminalNoMapping,
		}
	}

	digestHash := pluginPrepWorkerDigestHash(params, targetRef, pluginID, selected.ParamID)
	pendingID := agentprotocol.NormalizeID("pending", pluginPrepWorkerWorkflow, conversationID, targetRef, pluginID, digestHash, "candidate")
	if existing, ok := s.pluginPrepWorkerExistingPending(pendingID); ok {
		workflowData := pluginPrepWorkerBaseWorkflowData(plan, trackID, pluginID, pluginName, params, existing.CandidateAction)
		workflowData["pending_candidate"] = agentprotocol.ToMap(existing)
		req := s.pluginPrepWorkerConfirmationRequest(existing, workflowData, conversationID, goalID, runID)
		resp := ChatResponse{
			ConversationID:      conversationID,
			GoalID:              goalID,
			RunID:               runID,
			Reply:               message,
			Workflow:            pluginPrepWorkerWorkflow,
			WorkflowData:        workflowData,
			PluginLearning:      workflowData,
			InteractionRequests: []AgentInteractionRequest{req},
			TypedEvents:         append(baseEvents, agentprotocol.ToMap(agentprotocol.NewEvent(existing, existing.Source))),
			GoalStatus:          string(agentruntime.StatusWaitingConfirmation),
			StopReason:          pluginPrepWorkerCandidateStopReason,
		}
		s.storePendingInteraction(req, workflowData)
		s.attachInteractionRequests(&resp)
		return pluginPrepContinuation{
			WorkflowData:        workflowData,
			InteractionRequests: resp.InteractionRequests,
			TypedEvents:         resp.TypedEvents,
			GoalStatus:          resp.GoalStatus,
			StopReason:          resp.StopReason,
		}
	}

	action := pluginPrepWorkerCandidateAction(trackID, pluginID, pluginName, targetRef, selected, params, plan.Context, plan.WorkflowData, intent, digestHash)
	candidate := agentprotocol.PendingCandidate{
		ID:              pendingID,
		Kind:            agentprotocol.KindPendingCandidate,
		Domain:          "plugin_prep",
		CandidateType:   pluginPrepParameterTreatmentType,
		TargetRef:       targetRef,
		Summary:         "已根据插件参数摘要生成一个保守的 EQ 参数候选。",
		Rationale:       "用户请求清理低频/低中频浑浊；当前只准备小幅 EQ 增益削减，并在用户确认前保持不写入。",
		CandidateAction: action,
		Risk:            "undoable_commit",
		RequiredPermissionDomains: []string{
			"plugin.param_write",
		},
		Status:    agentprotocol.PendingStatusWaitingUser,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Source: agentprotocol.Source{
			ConversationID: conversationID,
			GoalID:         goalID,
			RunID:          runID,
			CallID:         strings.TrimSpace(plan.ID),
			LegacyKind:     pluginPrepParameterTreatmentType,
			Metadata: map[string]any{
				"candidate_source":      pluginPrepWorkerWorkflow,
				"parameter_digest_hash": digestHash,
				"idempotency_key":       pendingID,
				"plugin_id":             pluginID,
				"plugin_name":           pluginName,
			},
		},
	}
	s.upsertPendingCandidate(candidate)

	workflowData := pluginPrepWorkerBaseWorkflowData(plan, trackID, pluginID, pluginName, params, action)
	workflowData["pending_candidate"] = agentprotocol.ToMap(candidate)
	req := s.pluginPrepWorkerConfirmationRequest(candidate, workflowData, conversationID, goalID, runID)
	s.storePendingInteraction(req, workflowData)
	candidateEvent := agentprotocol.ToMap(agentprotocol.NewEvent(candidate, candidate.Source))
	approvalEvent := pluginPrepWorkerApprovalEvent(candidate)
	resp := ChatResponse{
		ConversationID:      conversationID,
		GoalID:              goalID,
		RunID:               runID,
		Reply:               message,
		Workflow:            pluginPrepWorkerWorkflow,
		WorkflowData:        workflowData,
		PluginLearning:      workflowData,
		InteractionRequests: []AgentInteractionRequest{req},
		TypedEvents:         append(baseEvents, candidateEvent, approvalEvent),
		GoalStatus:          string(agentruntime.StatusWaitingConfirmation),
		StopReason:          pluginPrepWorkerCandidateStopReason,
	}
	s.attachInteractionRequests(&resp)
	return pluginPrepContinuation{
		WorkflowData:        workflowData,
		InteractionRequests: resp.InteractionRequests,
		TypedEvents:         resp.TypedEvents,
		GoalStatus:          resp.GoalStatus,
		StopReason:          resp.StopReason,
	}
}

func pluginPrepWorkerBaseWorkflowData(plan PendingPlan, trackID, pluginID, pluginName string, params map[string]any, action map[string]any) map[string]any {
	workflowData := copyStringAnyMap(plan.WorkflowData)
	workflowData["stage"] = "pending_confirmation"
	workflowData["type"] = pluginPrepParameterTreatmentType
	workflowData["workflow"] = pluginPrepWorkerWorkflow
	workflowData["candidate_source"] = pluginPrepWorkerWorkflow
	workflowData["track_id"] = strings.TrimSpace(trackID)
	workflowData["plugin_id"] = strings.TrimSpace(pluginID)
	workflowData["plugin_name"] = strings.TrimSpace(pluginName)
	workflowData["request_context"] = cloneContext(plan.Context)
	workflowData["parameter_digest"] = params
	workflowData["parameter_count"] = params["parameter_count"]
	workflowData["quick_control_count"] = params["quick_control_count"]
	if action != nil {
		workflowData["plugin_prep_worker"] = action
		workflowData["plugin_parameter_treatment_candidate"] = action
		workflowData["display_summary"] = action["display_summary"]
		workflowData["evidence_status"] = action["evidence_status"]
		workflowData["evidence_summary"] = action["evidence_summary"]
		workflowData["diagnosis_context_id"] = action["diagnosis_context_id"]
		workflowData["diagnosis_context"] = action["diagnosis_context"]
		workflowData["parameter_change_summary"] = action["parameter_change_summary"]
		workflowData["treatment_type"] = action["treatment_type"]
		workflowData["parameter_changes"] = action["parameter_changes"]
		workflowData["verify_plan"] = action["verify_plan"]
		workflowData["pending_id"] = action["pending_id"]
		if beforeObservationID := cleanContextText(action["before_observation_id"]); beforeObservationID != "" {
			workflowData["before_observation_id"] = beforeObservationID
			workflowData["previous_observation"] = beforeObservationID
		}
		if mixSessionID := cleanContextText(action["mix_session_id"]); mixSessionID != "" {
			workflowData["mix_session_id"] = mixSessionID
		}
	}
	return workflowData
}

func pluginPrepWorkerTargetRef(trackID string, workflowData map[string]any, context map[string]any) string {
	if trackID = strings.TrimSpace(trackID); trackID != "" {
		return "track:" + trackID
	}
	for _, row := range []map[string]any{workflowData, context} {
		targetRef := cleanContextText(row["target_ref"])
		if strings.HasPrefix(strings.ToLower(targetRef), "track:") {
			return targetRef
		}
		if id := firstNonEmpty(cleanContextText(row["track_id"]), cleanContextText(row["target_track_id"]), cleanContextText(row["selected_track_id"])); id != "" {
			return "track:" + id
		}
	}
	return ""
}

func pluginPrepWorkerSelectEQControl(params map[string]any) (pluginPrepWorkerControl, bool) {
	controls := pluginPrepWorkerControls(params)
	if len(controls) == 0 {
		return pluginPrepWorkerControl{}, false
	}
	sort.SliceStable(controls, func(i, j int) bool {
		return pluginPrepWorkerControlScore(controls[i]) > pluginPrepWorkerControlScore(controls[j])
	})
	for _, control := range controls {
		if control.ParamID == "" {
			continue
		}
		role := strings.ToLower(control.Role)
		label := strings.ToLower(control.Label)
		if strings.Contains(role, "eq_gain") || strings.Contains(label, "gain") {
			return control, true
		}
	}
	return pluginPrepWorkerControl{}, false
}

func pluginPrepWorkerControls(params map[string]any) []pluginPrepWorkerControl {
	rows := mapRowsFromAny(params["quick_controls"])
	if len(rows) == 0 {
		rows = mapRowsFromAny(params["controls"])
	}
	out := make([]pluginPrepWorkerControl, 0, len(rows))
	for _, row := range rows {
		paramID := firstNonEmpty(cleanContextText(row["param_id"]), cleanContextText(row["id"]), cleanContextText(row["raw_param_id"]))
		if paramID == "" {
			continue
		}
		label := firstNonEmpty(cleanContextText(row["label"]), cleanContextText(row["name"]), cleanContextText(row["raw_name"]), paramID)
		role := firstNonEmpty(cleanContextText(row["normalized_role"]), cleanContextText(row["role"]), cleanContextText(row["control_role"]))
		out = append(out, pluginPrepWorkerControl{
			ParamID: paramID,
			Label:   label,
			Role:    role,
			Band:    pluginPrepWorkerBandKey(label),
			Raw:     copyStringAnyMap(row),
		})
	}
	return out
}

func pluginPrepWorkerControlScore(control pluginPrepWorkerControl) int {
	text := strings.ToLower(strings.TrimSpace(control.Label + " " + control.Role + " " + control.Band))
	score := 0
	if strings.Contains(text, "eq_gain") || strings.Contains(text, "gain") {
		score += 100
	}
	if strings.Contains(text, "b1") || strings.Contains(text, "band 1") || strings.Contains(text, "band_1") {
		score += 20
	}
	if strings.Contains(text, "b2") || strings.Contains(text, "band 2") || strings.Contains(text, "band_2") {
		score += 10
	}
	if strings.Contains(text, "dry") || strings.Contains(text, "mix") {
		score -= 50
	}
	return score
}

func pluginPrepWorkerBandKey(label string) string {
	text := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(text, "b1"), strings.Contains(text, "band 1"), strings.Contains(text, "band_1"):
		return "B1"
	case strings.Contains(text, "b2"), strings.Contains(text, "band 2"), strings.Contains(text, "band_2"):
		return "B2"
	case strings.Contains(text, "b3"), strings.Contains(text, "band 3"), strings.Contains(text, "band_3"):
		return "B3"
	default:
		return ""
	}
}

type pluginPrepWorkerEvidenceSnapshot struct {
	Status             map[string]any
	Summary            string
	DisplaySummary     string
	BandEnergy         map[string]any
	BandReady          bool
	DiagnosisContextID string
	DiagnosisContext   map[string]any
	EvidenceRefs       []string
}

func pluginPrepWorkerCandidateAction(trackID, pluginID, pluginName, targetRef string, selected pluginPrepWorkerControl, params, requestContext, workflowData map[string]any, intent, digestHash string) map[string]any {
	evidence := pluginPrepWorkerEvidenceSnapshotFromContext(params, requestContext, workflowData)
	treatmentType := "EQ 低频/低中频清理"
	currentValue := pluginPrepWorkerCurrentValueText(selected, params)
	frequencyNote := pluginPrepWorkerFrequencyNote(selected, params, evidence)
	parameterSummary := map[string]any{
		"param_id":        selected.ParamID,
		"label":           selected.Label,
		"role":            firstNonEmpty(selected.Role, "eq_gain"),
		"target":          "-1.5 dB",
		"treatment_type":  treatmentType,
		"frequency_note":  frequencyNote,
		"evidence_status": evidence.Status,
	}
	if currentValue != "" {
		parameterSummary["current"] = currentValue
	}
	if selected.Band != "" {
		parameterSummary["band"] = selected.Band
	}
	change := map[string]any{
		"param_id":          selected.ParamID,
		"label":             selected.Label,
		"role":              firstNonEmpty(selected.Role, "eq_gain"),
		"target_value_text": "-1.5 dB",
		"value_text":        "-1.5 dB",
		"reason":            "为低频/低中频浑浊清理准备的小幅 EQ 增益削减。",
		"confidence":        "medium",
		"frequency_note":    frequencyNote,
	}
	if currentValue != "" {
		change["current_value_text"] = currentValue
	}
	if selected.Band != "" {
		change["band"] = selected.Band
	}
	command := map[string]any{
		"cmd":        "set_plugin_param",
		"track_id":   trackID,
		"plugin_id":  pluginID,
		"param_id":   selected.ParamID,
		"value_text": "-1.5 dB",
		"reason":     "用户已确认 Plugin Prep Worker 参数候选。",
	}
	action := map[string]any{
		"schema_version":        "plugin_prep_worker_candidate.v0",
		"action_kind":           pluginPrepParameterTreatmentType,
		"processor_type":        "eq",
		"treatment_type":        treatmentType,
		"display_summary":       evidence.DisplaySummary,
		"target_ref":            targetRef,
		"plugin_ref":            "plugin:" + pluginID,
		"track_id":              trackID,
		"plugin_id":             pluginID,
		"plugin_name":           pluginName,
		"candidate_source":      pluginPrepWorkerWorkflow,
		"parameter_digest_hash": digestHash,
		"intent":                strings.TrimSpace(intent),
		"confidence":            "medium",
		"evidence_status":       evidence.Status,
		"evidence_summary":      evidence.Summary,
		"diagnosis_context_id":  evidence.DiagnosisContextID,
		"diagnosis_context":     cloneContext(evidence.DiagnosisContext),
		"parameter_change_summary": []map[string]any{
			parameterSummary,
		},
		"parameter_changes": []map[string]any{change},
		"write_commands":    []map[string]any{command},
		"verify_plan": map[string]any{
			"after_write": []string{"mix.observe"},
			"target_ref":  targetRef,
			"expected":    "写入保守 EQ 参数后重新观测工程。",
		},
		"evidence_refs": pluginPrepWorkerEvidenceRefs(evidence, params, requestContext, workflowData),
	}
	if len(evidence.BandEnergy) > 0 {
		action["band_energy_summary"] = evidence.BandEnergy
	}
	if beforeObservationID := pluginPrepWorkerFindObservationID(action, requestContext, workflowData, params); beforeObservationID != "" {
		action["before_observation_id"] = beforeObservationID
		action["previous_observation"] = beforeObservationID
	}
	if mixSessionID := pluginPrepWorkerFindMixSessionID(action, requestContext, workflowData, params); mixSessionID != "" {
		action["mix_session_id"] = mixSessionID
	}
	removeEmptyTreatmentValues(action)
	return action
}

func pluginPrepWorkerEvidenceSnapshotFromContext(params, requestContext, workflowData map[string]any) pluginPrepWorkerEvidenceSnapshot {
	if diagnosis := pluginPrepWorkerFindDiagnosisContext(params, requestContext, workflowData); len(diagnosis) > 0 {
		if snapshot, ok := pluginPrepWorkerEvidenceSnapshotFromDiagnosis(diagnosis); ok {
			return snapshot
		}
	}
	bandEnergy := pluginPrepWorkerFindBandEnergy(params, requestContext, workflowData)
	status := strings.ToLower(firstNonEmpty(
		cleanContextText(bandEnergy["status"]),
		pluginPrepWorkerFindTextByKey(params, "band_energy_status", "spectral_status"),
		pluginPrepWorkerFindTextByKey(requestContext, "band_energy_status", "spectral_status"),
		pluginPrepWorkerFindTextByKey(workflowData, "band_energy_status", "spectral_status"),
	))
	if status == "" && len(bandEnergy) > 0 {
		status = "ready"
	}
	ready := status == "ready" || status == "available" || status == "ok"
	evidenceStatus := map[string]any{
		"peak_headroom":      "not_evaluated",
		"band_energy":        "missing",
		"strategy":           "conservative_probe",
		"confidence_ceiling": "medium",
		"explanation":        "当前未取得可靠的频段观测，因此该候选只作为小幅保守试探处理。",
	}
	if ready {
		evidenceStatus["band_energy"] = "ready"
		evidenceStatus["strategy"] = "band_observed_conservative"
		evidenceStatus["explanation"] = "已取得频段能量观测，仍采用小幅保守 EQ 处理并等待用户确认。"
		summary := "频段能量观测可用"
		if detail := pluginPrepWorkerBandEnergyText(bandEnergy); detail != "" {
			summary += "：" + detail
		}
		return pluginPrepWorkerEvidenceSnapshot{
			Status:         evidenceStatus,
			Summary:        summary,
			DisplaySummary: "已根据频段能量观测和插件参数生成一个保守的 EQ 参数候选。确认前不会写入插件参数。",
			BandEnergy:     bandEnergy,
			BandReady:      true,
		}
	}
	return pluginPrepWorkerEvidenceSnapshot{
		Status:         evidenceStatus,
		Summary:        "当前未取得可靠的频段观测；该候选只作为保守试探处理，置信度不超过 medium。",
		DisplaySummary: "已生成一个保守的 EQ 参数候选。确认前不会写入插件参数。当前未取得可靠的频段观测，因此该候选只作为小幅保守试探处理。",
		BandEnergy:     bandEnergy,
		BandReady:      false,
	}
}

func pluginPrepWorkerFindDiagnosisContext(rows ...map[string]any) map[string]any {
	for _, row := range rows {
		if strings.EqualFold(cleanContextText(row["schema_version"]), "mix_diagnosis_context.v0") {
			return row
		}
		if found := pluginPrepWorkerFindMapByKey(row, 0, "diagnosis_context", "mix_diagnosis_context"); len(found) > 0 {
			return found
		}
	}
	return nil
}

func pluginPrepWorkerEvidenceSnapshotFromDiagnosis(diagnosis map[string]any) (pluginPrepWorkerEvidenceSnapshot, bool) {
	if !strings.EqualFold(cleanContextText(diagnosis["schema_version"]), "mix_diagnosis_context.v0") {
		return pluginPrepWorkerEvidenceSnapshot{}, false
	}
	id := firstNonEmpty(cleanContextText(diagnosis["id"]), cleanContextText(diagnosis["diagnosis_context_id"]))
	status := mapValue(diagnosis["evidence_status"])
	recommendation := mapValue(diagnosis["recommendation"])
	strategy := firstNonEmpty(cleanContextText(recommendation["strategy"]), cleanContextText(status["strategy"]), "conservative_probe")
	confidence := firstNonEmpty(cleanContextText(recommendation["confidence"]), cleanContextText(status["confidence_ceiling"]), "medium")
	bandEnergy := pluginPrepWorkerBandEnergyFromDiagnosis(diagnosis)
	bandReady := pluginPrepWorkerEvidenceReady(firstNonEmpty(cleanContextText(status["band_energy"]), cleanContextText(bandEnergy["status"]))) || len(bandEnergy) > 0 && cleanContextText(bandEnergy["status"]) == ""
	missing := pluginPrepWorkerDiagnosisMissingKeys(diagnosis)
	evidenceStatus := map[string]any{
		"peak_headroom":        "not_evaluated",
		"band_energy":          "missing",
		"strategy":             strategy,
		"confidence_ceiling":   confidence,
		"diagnosis_context_id": id,
		"problem_kind":         cleanContextText(diagnosis["problem_kind"]),
		"missing_evidence":     missing,
		"explanation":          "来自 Mix Diagnosis Context Pack v0；候选参数必须继续等待用户确认。",
	}
	if bandReady {
		evidenceStatus["band_energy"] = "ready"
		if strategy == "" || strategy == "conservative_probe" {
			evidenceStatus["strategy"] = "band_observed_conservative"
		}
		summary := "诊断上下文已引用频段能量观测"
		if detail := pluginPrepWorkerBandEnergyText(bandEnergy); detail != "" {
			summary += "：" + detail
		}
		return pluginPrepWorkerEvidenceSnapshot{
			Status:             evidenceStatus,
			Summary:            summary,
			DisplaySummary:     "已根据诊断上下文中的频段能量观测和插件参数生成一个保守的 EQ 参数候选。确认前不会写入插件参数。",
			BandEnergy:         bandEnergy,
			BandReady:          true,
			DiagnosisContextID: id,
			DiagnosisContext:   cloneContext(diagnosis),
			EvidenceRefs:       pluginPrepWorkerDiagnosisEvidenceRefs(diagnosis),
		}, true
	}
	if len(missing) == 0 {
		missing = []string{"band_energy_summary"}
		evidenceStatus["missing_evidence"] = missing
	}
	return pluginPrepWorkerEvidenceSnapshot{
		Status:             evidenceStatus,
		Summary:            "诊断上下文标记频段/频谱证据缺失；该候选只作为保守试探处理，置信度不超过 " + confidence + "。",
		DisplaySummary:     "已根据诊断上下文生成一个保守的 EQ 参数候选。确认前不会写入插件参数。当前未取得可靠的频段观测，因此该候选只作为小幅保守试探处理。",
		BandEnergy:         bandEnergy,
		BandReady:          false,
		DiagnosisContextID: id,
		DiagnosisContext:   cloneContext(diagnosis),
		EvidenceRefs:       pluginPrepWorkerDiagnosisEvidenceRefs(diagnosis),
	}, true
}

func pluginPrepWorkerEvidenceReady(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available", "ok":
		return true
	default:
		return false
	}
}

func pluginPrepWorkerBandEnergyFromDiagnosis(diagnosis map[string]any) map[string]any {
	for _, row := range mapRowsFromAny(diagnosis["observed_facts"]) {
		if cleanContextText(row["key"]) == "band_energy_summary" {
			if data := mapValue(row["data"]); len(data) > 0 {
				return data
			}
		}
	}
	return pluginPrepWorkerFindBandEnergy(diagnosis)
}

func pluginPrepWorkerDiagnosisMissingKeys(diagnosis map[string]any) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range pluginPrepWorkerStringList(mapValue(diagnosis["evidence_status"])["missing"]) {
		add(value)
	}
	for _, row := range mapRowsFromAny(diagnosis["missing_evidence"]) {
		add(cleanContextText(row["key"]))
	}
	return out
}

func pluginPrepWorkerDiagnosisEvidenceRefs(diagnosis map[string]any) []string {
	refs := pluginPrepWorkerStringList(diagnosis["evidence_refs"])
	if id := cleanContextText(diagnosis["id"]); id != "" {
		refs = append([]string{id}, refs...)
	}
	return refs
}

func pluginPrepWorkerStringList(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := cleanContextText(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{strings.TrimSpace(v)}
	default:
		return nil
	}
}

func pluginPrepWorkerFindBandEnergy(rows ...map[string]any) map[string]any {
	for _, row := range rows {
		if found := pluginPrepWorkerFindMapByKey(row, 0, "band_energy_summary", "band_energy"); len(found) > 0 {
			return found
		}
	}
	return nil
}

func pluginPrepWorkerFindMapByKey(value any, depth int, keys ...string) map[string]any {
	if depth > 5 {
		return nil
	}
	row := mapValue(value)
	if len(row) > 0 {
		for _, key := range keys {
			if found := mapValue(row[key]); len(found) > 0 {
				return found
			}
		}
		for _, nested := range row {
			if found := pluginPrepWorkerFindMapByKey(nested, depth+1, keys...); len(found) > 0 {
				return found
			}
		}
	}
	for _, nested := range mapRowsFromAny(value) {
		if found := pluginPrepWorkerFindMapByKey(nested, depth+1, keys...); len(found) > 0 {
			return found
		}
	}
	return nil
}

func pluginPrepWorkerFindTextByKey(value any, keys ...string) string {
	return pluginPrepWorkerFindTextByKeyDepth(value, 0, keys...)
}

func pluginPrepWorkerFindTextByKeyDepth(value any, depth int, keys ...string) string {
	if depth > 5 {
		return ""
	}
	row := mapValue(value)
	if len(row) > 0 {
		for _, key := range keys {
			if text := cleanContextText(row[key]); text != "" {
				return text
			}
		}
		for _, nested := range row {
			if text := pluginPrepWorkerFindTextByKeyDepth(nested, depth+1, keys...); text != "" {
				return text
			}
		}
	}
	for _, nested := range mapRowsFromAny(value) {
		if text := pluginPrepWorkerFindTextByKeyDepth(nested, depth+1, keys...); text != "" {
			return text
		}
	}
	return ""
}

func pluginPrepWorkerBandEnergyText(bandEnergy map[string]any) string {
	if len(bandEnergy) == 0 {
		return ""
	}
	parts := []string{}
	if source := cleanContextText(bandEnergy["source"]); source != "" {
		parts = append(parts, "来源 "+source)
	}
	if target := firstNonEmpty(cleanContextText(bandEnergy["track_id"]), cleanContextText(bandEnergy["target_ref"])); target != "" {
		parts = append(parts, "目标 "+target)
	}
	bands := mapValue(bandEnergy["bands"])
	for _, key := range []string{"sub", "bass", "low_mid", "mid"} {
		if value := cleanContextText(bands[key]); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, "；")
}

func pluginPrepWorkerCurrentValueText(selected pluginPrepWorkerControl, params map[string]any) string {
	if value := firstStringFromMap(selected.Raw, "current_value_text", "value_text", "display_value", "value"); value != "" {
		return value
	}
	for _, row := range mapRowsFromAny(params["parameters"]) {
		if pluginPrepWorkerParamMatches(row, selected.ParamID) {
			if value := firstStringFromMap(row, "current_value_text", "value_text", "display_value", "value"); value != "" {
				return value
			}
		}
	}
	return ""
}

func pluginPrepWorkerFrequencyNote(selected pluginPrepWorkerControl, params map[string]any, evidence pluginPrepWorkerEvidenceSnapshot) string {
	related := pluginPrepWorkerRelatedControlNotes(selected, params)
	if len(related) > 0 {
		if evidence.BandReady {
			return "插件当前频段设置：" + strings.Join(related, "；") + "；频段能量观测可用，仍只做小幅削减。"
		}
		return "插件当前频段设置：" + strings.Join(related, "；") + "；频点未通过频段观测可靠确认，使用当前频段设置做小幅保守削减。"
	}
	if evidence.BandReady {
		return "频段能量观测可用；具体频点沿用插件当前频段设置并做小幅削减。"
	}
	return "未取得可靠频段观测，频点未可靠确认；使用插件当前频段设置做小幅保守削减。"
}

func pluginPrepWorkerRelatedControlNotes(selected pluginPrepWorkerControl, params map[string]any) []string {
	band := selected.Band
	if band == "" {
		band = pluginPrepWorkerBandKey(selected.Label)
	}
	if band == "" {
		return nil
	}
	out := []string{}
	for _, control := range pluginPrepWorkerControls(params) {
		if control.ParamID == selected.ParamID || control.Band != band {
			continue
		}
		roleText := strings.ToLower(control.Role + " " + control.Label)
		value := pluginPrepWorkerCurrentValueText(control, params)
		if value == "" {
			continue
		}
		switch {
		case strings.Contains(roleText, "freq"):
			out = append(out, "频点 "+value)
		case strings.Contains(roleText, "q"):
			out = append(out, "Q "+value)
		}
	}
	return out
}

func pluginPrepWorkerParamMatches(row map[string]any, paramID string) bool {
	paramID = strings.TrimSpace(paramID)
	if paramID == "" {
		return false
	}
	for _, key := range []string{"param_id", "id", "raw_param_id"} {
		if strings.TrimSpace(cleanContextText(row[key])) == paramID {
			return true
		}
	}
	return false
}

func pluginPrepWorkerEvidenceRefs(evidence pluginPrepWorkerEvidenceSnapshot, rows ...map[string]any) []string {
	refs := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		refs = append(refs, value)
	}
	for _, row := range rows {
		for _, key := range []string{"observation_id", "context_pack_id", "artifact_id", "diagnosis_context_id"} {
			add(cleanContextText(row[key]))
		}
	}
	for _, ref := range evidence.EvidenceRefs {
		add(ref)
	}
	if evidence.DiagnosisContextID != "" {
		add(evidence.DiagnosisContextID)
	}
	if evidence.BandReady {
		source := cleanContextText(evidence.BandEnergy["source"])
		if source == "" {
			source = "available"
		}
		add("band_energy_summary:" + source)
	}
	return refs
}

func pluginPrepWorkerDigestHash(params map[string]any, targetRef, pluginID, paramID string) string {
	digest := map[string]any{
		"target_ref":      targetRef,
		"plugin_id":       pluginID,
		"param_id":        paramID,
		"parameter_count": params["parameter_count"],
		"quick_controls":  params["quick_controls"],
	}
	raw, _ := json.Marshal(digest)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Server) pluginPrepWorkerExistingPending(id string) (agentprotocol.PendingCandidate, bool) {
	if s == nil || s.pendingManager == nil || strings.TrimSpace(id) == "" {
		return agentprotocol.PendingCandidate{}, false
	}
	candidate, ok := s.pendingManager.Get(id)
	if !ok || !pluginPrepWorkerPendingActive(candidate.Status) {
		return agentprotocol.PendingCandidate{}, false
	}
	return candidate, true
}

func pluginPrepWorkerPendingActive(status string) bool {
	switch strings.TrimSpace(status) {
	case agentprotocol.PendingStatusRejected,
		agentprotocol.PendingStatusCommitted,
		agentprotocol.PendingStatusVerified,
		agentprotocol.PendingStatusVerificationFailed,
		agentprotocol.PendingStatusFailed,
		agentprotocol.PendingStatusBlocked:
		return false
	default:
		return true
	}
}

func (s *Server) pluginPrepWorkerConfirmationRequest(candidate agentprotocol.PendingCandidate, payload map[string]any, conversationID, goalID, runID string) AgentInteractionRequest {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["pending_id"] = candidate.ID
	payload["pending_candidate"] = agentprotocol.ToMap(candidate)
	return AgentInteractionRequest{
		ID:             "interaction_" + randomID(),
		Kind:           "confirmation",
		Type:           pluginPrepParameterTreatmentType,
		Source:         pluginPrepWorkerWorkflow,
		Workflow:       pluginPrepWorkerWorkflow,
		Stage:          "pending_confirmation",
		Title:          "确认插件参数处理",
		Body:           pluginPrepWorkerConfirmationBody(payload),
		Status:         "waiting_for_user",
		ConversationID: conversationID,
		GoalID:         goalID,
		RunID:          runID,
		Payload:        payload,
		Data:           payload,
		Actions: []AgentInteractionAction{
			{ID: "approve", Label: "应用候选", Style: "primary", Recommended: true},
			{ID: "revise", Label: "调整候选", Style: "secondary"},
			{ID: "cancel", Label: "取消", Style: "secondary"},
		},
	}
}

func pluginPrepWorkerConfirmationBody(payload map[string]any) string {
	action := pluginPrepWorkerDisplayAction(payload)
	lines := []string{}
	if summary := firstNonEmpty(cleanContextText(action["display_summary"]), cleanContextText(payload["display_summary"])); summary != "" {
		lines = append(lines, summary)
	} else {
		lines = append(lines, "已生成一个保守的 EQ 参数候选。确认前不会写入插件参数。")
	}
	targetRef := firstNonEmpty(cleanContextText(action["target_ref"]), cleanContextText(payload["target_ref"]))
	trackID := firstNonEmpty(cleanContextText(action["track_id"]), cleanContextText(payload["track_id"]))
	if targetRef == "" && trackID != "" {
		targetRef = "track:" + trackID
	}
	pluginName := firstNonEmpty(cleanContextText(action["plugin_name"]), cleanContextText(payload["plugin_name"]))
	if pluginName == "" {
		pluginName = firstNonEmpty(cleanContextText(action["plugin_id"]), cleanContextText(payload["plugin_id"]))
	}
	treatmentType := firstNonEmpty(cleanContextText(action["treatment_type"]), cleanContextText(payload["treatment_type"]), "EQ 低频/低中频清理")
	detailParts := []string{}
	if targetRef != "" {
		detailParts = append(detailParts, "目标："+targetRef)
	}
	if pluginName != "" {
		detailParts = append(detailParts, "插件："+pluginName)
	}
	if treatmentType != "" {
		detailParts = append(detailParts, "处理："+treatmentType)
	}
	if len(detailParts) > 0 {
		lines = append(lines, pluginPrepWorkerSentence(strings.Join(detailParts, "；")))
	}
	if line := pluginPrepWorkerParameterDisplayLine(action); line != "" {
		lines = append(lines, line)
	}
	if line := pluginPrepWorkerEvidenceDisplayLine(action); line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func pluginPrepWorkerDisplayAction(payload map[string]any) map[string]any {
	action := firstNonEmptyMap(
		mapValue(payload["plugin_prep_worker"]),
		mapValue(payload["plugin_parameter_treatment_candidate"]),
		mapValue(payload["candidate_action"]),
	)
	if len(action) == 0 {
		candidate := mapValue(payload["pending_candidate"])
		action = mapValue(candidate["candidate_action"])
	}
	if len(action) == 0 && cleanContextText(payload["action_kind"]) == pluginPrepParameterTreatmentType {
		action = payload
	}
	if action == nil {
		return map[string]any{}
	}
	return action
}

func pluginPrepWorkerParameterDisplayLine(action map[string]any) string {
	rows := mapRowsFromAny(action["parameter_change_summary"])
	if len(rows) == 0 {
		rows = mapRowsFromAny(action["parameter_changes"])
	}
	if len(rows) == 0 {
		return ""
	}
	row := rows[0]
	parts := []string{}
	if label := firstNonEmpty(cleanContextText(row["label"]), cleanContextText(row["param_id"])); label != "" {
		parts = append(parts, "参数："+label)
	}
	if current := firstNonEmpty(cleanContextText(row["current"]), cleanContextText(row["current_value_text"])); current != "" {
		parts = append(parts, "当前值："+current)
	}
	if target := firstNonEmpty(cleanContextText(row["target"]), cleanContextText(row["target_value_text"]), cleanContextText(row["value_text"])); target != "" {
		parts = append(parts, "目标值："+target)
	}
	if note := cleanContextText(row["frequency_note"]); note != "" {
		parts = append(parts, "频点/Q："+note)
	}
	if len(parts) == 0 {
		return ""
	}
	return pluginPrepWorkerSentence(strings.Join(parts, "；"))
}

func pluginPrepWorkerEvidenceDisplayLine(action map[string]any) string {
	status := mapValue(action["evidence_status"])
	if len(status) == 0 {
		return localizedDisplayTextFallback(cleanContextText(action["evidence_summary"]))
	}
	bandEnergy := pluginPrepWorkerEvidenceLabel(cleanContextText(status["band_energy"]))
	strategy := pluginPrepWorkerStrategyLabel(cleanContextText(status["strategy"]))
	confidence := pluginPrepWorkerConfidenceLabel(firstNonEmpty(cleanContextText(status["confidence_ceiling"]), cleanContextText(action["confidence"])))
	parts := []string{}
	if bandEnergy != "" {
		parts = append(parts, "频段分析："+bandEnergy)
	}
	if strategy != "" {
		parts = append(parts, "策略："+strategy)
	}
	if confidence != "" {
		parts = append(parts, "置信度上限："+confidence)
	}
	if summary := localizedDisplayTextFallback(cleanContextText(action["evidence_summary"])); summary != "" {
		parts = append(parts, "证据说明："+summary)
	}
	if len(parts) == 0 {
		return ""
	}
	return pluginPrepWorkerSentence(strings.Join(parts, "；"))
}

func pluginPrepWorkerSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.TrimRight(text, "。.!！?？") + "。"
}

func pluginPrepWorkerEvidenceLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "available", "ok":
		return "可用"
	case "missing", "":
		return "缺失"
	default:
		return status
	}
}

func pluginPrepWorkerStrategyLabel(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "conservative_probe":
		return "保守试探"
	case "band_observed_conservative":
		return "基于频段观测的保守处理"
	case "request_more_observation":
		return "需要更多观察"
	case "":
		return ""
	default:
		return localizedDisplayTextFallback(strategy)
	}
}

func pluginPrepWorkerConfidenceLabel(confidence string) string {
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	case "":
		return ""
	default:
		return localizedDisplayTextFallback(confidence)
	}
}

func pluginPrepWorkerApprovalEvent(candidate agentprotocol.PendingCandidate) map[string]any {
	approval := agentprotocol.ApprovalRequest{
		ID:                 agentprotocol.NormalizeID("approval", candidate.ID, "plugin_param_write"),
		Kind:               agentprotocol.KindApprovalRequest,
		ResourceDomain:     "plugin.param_write",
		Operation:          "commit",
		Scope:              "single_candidate",
		TargetRef:          candidate.TargetRef,
		Reason:             "写入插件参数前必须先由用户确认该候选。",
		Status:             agentprotocol.ApprovalStatusWaitingUser,
		AvailableDecisions: []string{"approve", "revise", "cancel"},
		CreatedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		Source:             candidate.Source,
	}
	return agentprotocol.ToMap(agentprotocol.NewEvent(approval, approval.Source))
}

func pluginPrepWorkerUserInputEvent(plan PendingPlan, questionType, prompt string) map[string]any {
	source := agentprotocol.Source{
		ConversationID: firstNonEmpty(cleanContextText(plan.Context["conversation_id"]), cleanContextText(plan.WorkflowData["conversation_id"])),
		GoalID:         cleanContextText(plan.Context["goal_id"]),
		RunID:          cleanContextText(plan.Context["run_id"]),
		CallID:         strings.TrimSpace(plan.ID),
		LegacyKind:     pluginPrepWorkerWorkflow,
	}
	input := agentprotocol.UserInputRequest{
		ID:           agentprotocol.NormalizeID("input", pluginPrepWorkerWorkflow, plan.ID, questionType),
		Kind:         agentprotocol.KindUserInputRequest,
		QuestionType: questionType,
		Prompt:       prompt,
		Status:       agentprotocol.UserInputStatusWaitingUser,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Source:       source,
	}
	return agentprotocol.ToMap(agentprotocol.NewEvent(input, source))
}

func (s *Server) handlePendingPluginParameterTreatmentChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	candidate, ok := s.activePluginPrepWorkerCandidate(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	switch {
	case messageRevisesPendingMixTreatment(req.Message):
		payload := pluginPrepWorkerPayloadFromCandidate(candidate, req.Context)
		interaction := pluginPrepWorkerInteractionFromCandidate(candidate, payload)
		return s.continuePluginPrepWorkerCandidateInteraction(ctx, interaction, payload, "revise"), true
	case messageExplicitMixTickApply(req.Message) || messagePlainMixApproval(req.Message):
		payload := pluginPrepWorkerPayloadFromCandidate(candidate, req.Context)
		interaction := pluginPrepWorkerInteractionFromCandidate(candidate, payload)
		resp := s.continuePluginPrepWorkerCandidateInteraction(ctx, interaction, payload, "approve")
		resp.AgentMode = mode
		return resp, true
	case messageAmbiguousMixTickApproval(req.Message):
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "当前有一个插件参数候选正在等待确认。请明确说“确认/应用”来写入，或说“取消/调整”。",
			GoalStatus:     string(agentruntime.StatusWaitingConfirmation),
			StopReason:     "ambiguous_plugin_parameter_treatment_confirmation",
		}, true
	default:
		return ChatResponse{}, false
	}
}

func (s *Server) activePluginPrepWorkerCandidate(conversationID string) (agentprotocol.PendingCandidate, bool) {
	if s == nil || s.pendingManager == nil || strings.TrimSpace(conversationID) == "" {
		return agentprotocol.PendingCandidate{}, false
	}
	for _, candidate := range s.pendingManager.ActiveForConversation(conversationID) {
		if strings.EqualFold(candidate.CandidateType, pluginPrepParameterTreatmentType) &&
			strings.EqualFold(cleanContextText(candidate.CandidateAction["candidate_source"]), pluginPrepWorkerWorkflow) {
			return candidate, true
		}
	}
	return agentprotocol.PendingCandidate{}, false
}

func pluginPrepWorkerPayloadFromCandidate(candidate agentprotocol.PendingCandidate, requestContext map[string]any) map[string]any {
	payload := copyStringAnyMap(candidate.CandidateAction)
	payload["pending_id"] = candidate.ID
	payload["pending_candidate"] = agentprotocol.ToMap(candidate)
	payload["request_context"] = cloneContext(requestContext)
	payload["track_id"] = cleanContextText(candidate.CandidateAction["track_id"])
	payload["plugin_id"] = cleanContextText(candidate.CandidateAction["plugin_id"])
	payload["plugin_name"] = cleanContextText(candidate.CandidateAction["plugin_name"])
	return payload
}

func pluginPrepWorkerInteractionFromCandidate(candidate agentprotocol.PendingCandidate, payload map[string]any) PendingInteraction {
	return PendingInteraction{
		ID:             candidate.ID,
		Kind:           "confirmation",
		Source:         pluginPrepWorkerWorkflow,
		Workflow:       pluginPrepWorkerWorkflow,
		Stage:          "pending_confirmation",
		ConversationID: candidate.Source.ConversationID,
		GoalID:         candidate.Source.GoalID,
		RunID:          candidate.Source.RunID,
		RequestContext: cloneContext(mapValue(payload["request_context"])),
		Payload:        payload,
		Type:           pluginPrepParameterTreatmentType,
		Data:           payload,
	}
}

func (s *Server) continuePluginPrepWorkerCandidateInteraction(ctx context.Context, interaction PendingInteraction, payload map[string]any, decision string) ChatResponse {
	payload = firstNonEmptyMap(payload, interaction.Payload)
	pendingID := firstNonEmpty(cleanContextText(payload["pending_id"]), cleanContextText(mapValue(payload["pending_candidate"])["id"]))
	candidate, ok := s.pluginPrepWorkerCandidateByID(pendingID)
	if !ok {
		return pluginPrepWorkerTerminalResponse(interaction, payload, "plugin_prep_worker_missing_pending_candidate", "该插件参数候选已不再有效；没有写入任何插件参数。")
	}
	decision = strings.ToLower(strings.TrimSpace(decision))
	switch decision {
	case "cancel", "reject", "deny", "stop":
		updated, _ := s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusRejected, "user cancelled plugin prep worker candidate")
		return pluginPrepWorkerNoWriteResponse(interaction, payload, updated, "已取消该插件参数候选，没有写入任何插件参数。", "plugin_prep_worker_candidate_rejected", agentruntime.StatusCompleted)
	case "revise", "edit", "change":
		updated, _ := s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusRevisionRequested, "user requested candidate revision")
		return pluginPrepWorkerNoWriteResponse(interaction, payload, updated, "没有写入任何插件参数。请描述你希望如何调整这个候选。", "plugin_prep_worker_candidate_revision_requested", agentruntime.StatusWaitingClarification)
	}
	if decision != "" && decision != "approve" && decision != "confirm" && decision != "execute" && decision != "submit" && decision != "apply" {
		return pluginPrepWorkerNoWriteResponse(interaction, payload, candidate, "没有写入任何插件参数。请确认/应用、调整或取消该候选。", "plugin_prep_worker_candidate_waiting_confirmation", agentruntime.StatusWaitingConfirmation)
	}

	accepted, _ := s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusAccepted, "user confirmed plugin prep worker candidate")
	executed := []map[string]any{}
	events := []map[string]any{agentprotocol.ToMap(agentprotocol.NewEvent(accepted, accepted.Source))}
	s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusCommitting, "plugin prep worker executing parameter writes")
	action := firstNonEmptyMap(candidate.CandidateAction, mapValue(payload["plugin_parameter_treatment_candidate"]), mapValue(payload["plugin_prep_worker"]))
	requestContext := pluginPrepWorkerWriteContext(payload, interaction)
	for _, command := range mapRowsFromAny(action["write_commands"]) {
		out, err := s.harness.Invoke(ctx, harness.InvokeRequest{
			Command:   command,
			Context:   requestContext,
			Source:    pluginPrepWorkerWorkflow,
			Confirmed: true,
			RunID:     interaction.RunID,
			GoalID:    interaction.GoalID,
		})
		executed = append(executed, pluginPrepWorkerInvokeRow(out))
		if pluginPrepWorkerInvokeFailed(out, err) {
			failed, _ := s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusFailed, firstNonEmpty(out.Error, errorText(err), "plugin parameter write failed"))
			events = append(events, agentprotocol.ToMap(agentprotocol.NewEvent(failed, failed.Source)), pluginPrepWorkerTerminalStateEvent(interaction, payload, "write_failed", firstNonEmpty(out.Error, errorText(err), "plugin parameter write failed")))
			return ChatResponse{
				ConversationID:      interaction.ConversationID,
				GoalID:              interaction.GoalID,
				RunID:               interaction.RunID,
				Reply:               "未能应用该插件参数候选；没有使用任何备用写入路径。",
				Workflow:            pluginPrepWorkerWorkflow,
				WorkflowData:        payload,
				PluginLearning:      payload,
				ExecutedKernelReply: executed,
				TypedEvents:         events,
				GoalStatus:          string(agentruntime.StatusFailed),
				StopReason:          "plugin_prep_parameter_treatment_failed",
				Error:               firstNonEmpty(out.Error, errorText(err), "plugin parameter write failed"),
			}
		}
	}
	committed, _ := s.pendingManager.Transition(candidate.ID, agentprotocol.PendingStatusCommitted, "plugin prep worker parameter writes committed")
	events = append(events, agentprotocol.ToMap(agentprotocol.NewEvent(committed, committed.Source)))

	beforeObservationID := pluginPrepWorkerFindObservationID(action, payload, requestContext, mapValue(payload["request_context"]), mapValue(payload["pending_candidate"]))
	mixSessionID := pluginPrepWorkerFindMixSessionID(action, payload, requestContext, mapValue(payload["request_context"]), mapValue(payload["pending_candidate"]))
	trackID := firstNonEmpty(cleanContextText(action["track_id"]), strings.TrimPrefix(candidate.TargetRef, "track:"), cleanContextText(payload["track_id"]), cleanContextText(requestContext["track_id"]))
	observeCommand := map[string]any{
		"cmd":              "mix_observe",
		"scope":            "full_project",
		"observation_only": true,
		"project_context":  true,
		"mom_intent":       "l2_render_probe",
		"projection":       "l2_render_probe",
		"goal_text":        "复查已确认的插件参数处理：" + candidate.TargetRef,
	}
	if trackID != "" {
		observeCommand["track_id"] = trackID
	}
	if beforeObservationID != "" {
		observeCommand["previous_observation"] = beforeObservationID
		observeCommand["previous_observation_id"] = beforeObservationID
	}
	if mixSessionID != "" {
		observeCommand["mix_session_id"] = mixSessionID
	}
	observe, observeErr := s.harness.Invoke(ctx, harness.InvokeRequest{
		Tool:      "mix.observe",
		Command:   observeCommand,
		Context:   requestContext,
		Source:    pluginPrepWorkerWorkflow,
		Confirmed: true,
		RunID:     interaction.RunID,
		GoalID:    interaction.GoalID,
	})
	executed = append(executed, pluginPrepWorkerInvokeRow(observe))
	status := agentprotocol.PendingStatusVerified
	stopReason := pluginPrepWorkerAppliedStopReason
	goalStatus := string(agentruntime.StatusCompleted)
	reply := "已应用确认的插件参数候选，并重新观测了工程。"
	if pluginPrepWorkerInvokeFailed(observe, observeErr) {
		status = agentprotocol.PendingStatusVerificationFailed
		stopReason = "plugin_prep_parameter_treatment_applied_reobserve_failed"
		goalStatus = string(agentruntime.StatusFailed)
		reply = "已应用确认的插件参数候选，但重新观测失败。"
	} else {
		replyLines := []string{reply}
		replyLines = append(replyLines, pluginPrepWorkerABResultLines(observe)...)
		reply = strings.Join(replyLines, "\n")
	}
	verified, _ := s.pendingManager.Transition(candidate.ID, status, firstNonEmpty(observe.Error, errorText(observeErr), "plugin prep worker verification completed"))
	events = append(events, agentprotocol.ToMap(agentprotocol.NewEvent(verified, verified.Source)), pluginPrepWorkerTerminalStateEvent(interaction, payload, status, reply))
	payload["executed_parameter_treatment"] = action
	payload["verify_result"] = observe.Result
	return ChatResponse{
		ConversationID:      interaction.ConversationID,
		GoalID:              interaction.GoalID,
		RunID:               interaction.RunID,
		Reply:               reply,
		Workflow:            pluginPrepWorkerWorkflow,
		WorkflowData:        payload,
		PluginLearning:      payload,
		ExecutedKernelReply: executed,
		ProjectResultCards:  projectResultCardsFromExecutedWithAB(executed, executor.Result{Result: observe.Result}),
		Artifacts:           artifactSummariesFromExecuted(executed),
		TypedEvents:         events,
		GoalStatus:          goalStatus,
		StopReason:          stopReason,
		Error:               firstNonEmpty(observe.Error, errorText(observeErr)),
	}
}

func (s *Server) pluginPrepWorkerCandidateByID(id string) (agentprotocol.PendingCandidate, bool) {
	if s == nil || s.pendingManager == nil || strings.TrimSpace(id) == "" {
		return agentprotocol.PendingCandidate{}, false
	}
	candidate, ok := s.pendingManager.Get(id)
	if !ok || !pluginPrepWorkerPendingActive(candidate.Status) {
		return agentprotocol.PendingCandidate{}, false
	}
	return candidate, true
}

func pluginPrepWorkerWriteContext(payload map[string]any, interaction PendingInteraction) map[string]any {
	ctx := cloneContext(mapValue(payload["request_context"]))
	if len(ctx) == 0 {
		ctx = cloneContext(interaction.RequestContext)
	}
	if ctx == nil {
		ctx = map[string]any{}
	}
	for key, value := range payload {
		if _, exists := ctx[key]; !exists {
			ctx[key] = value
		}
	}
	ctx["user_message"] = "confirmed plugin parameter write using exact param_id from get_plugin_parameters"
	ctx["intent"] = firstNonEmpty(cleanContextText(payload["intent"]), cleanContextText(ctx["intent"]), "confirmed plugin parameter treatment")
	ctx["plugin_prep_worker_confirmed"] = true
	return ctx
}

func pluginPrepWorkerFindObservationID(rows ...map[string]any) string {
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		for _, key := range []string{"before_observation_id", "previous_observation", "previous_observation_id", "observation_id", "mix_observation_id", "latest_observation_id"} {
			if id := pluginPrepWorkerNormalizeObservationID(cleanContextText(row[key])); id != "" {
				return id
			}
		}
		for _, key := range []string{"observation", "latest_observation", "request_context", "diagnosis_context", "plugin_prep_worker", "plugin_parameter_treatment_candidate", "candidate_action"} {
			if id := pluginPrepWorkerFindObservationID(mapValue(row[key])); id != "" {
				return id
			}
		}
		if id := pluginPrepWorkerObservationIDFromRefs(row["evidence_refs"]); id != "" {
			return id
		}
	}
	return ""
}

func pluginPrepWorkerFindMixSessionID(rows ...map[string]any) string {
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		for _, key := range []string{"mix_session_id", "session_id"} {
			if id := strings.TrimSpace(cleanContextText(row[key])); id != "" {
				return id
			}
		}
		for _, key := range []string{"observation", "latest_observation", "request_context", "diagnosis_context", "plugin_prep_worker", "plugin_parameter_treatment_candidate", "candidate_action"} {
			if id := pluginPrepWorkerFindMixSessionID(mapValue(row[key])); id != "" {
				return id
			}
		}
	}
	return ""
}

func pluginPrepWorkerObservationIDFromRefs(value any) string {
	for _, ref := range pluginPrepWorkerStringItems(value) {
		if id := pluginPrepWorkerNormalizeObservationID(ref); id != "" {
			return id
		}
	}
	return ""
}

func pluginPrepWorkerStringItems(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string{}, v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if text := cleanContextText(item); text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	default:
		if text := cleanContextText(value); text != "" {
			return []string{text}
		}
		return nil
	}
}

func pluginPrepWorkerNormalizeObservationID(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.Contains(ref, "observation:") {
		ref = ref[strings.Index(ref, "observation:")+len("observation:"):]
	} else if strings.Contains(ref, "observation_id:") {
		ref = ref[strings.Index(ref, "observation_id:")+len("observation_id:"):]
	} else if idx := strings.Index(ref, "obs_"); idx >= 0 {
		ref = ref[idx:]
	}
	ref = strings.TrimSpace(ref)
	ref = strings.Trim(ref, "\"'` ,;")
	for i, r := range ref {
		if r == '"' || r == '\'' || r == '`' || r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			ref = ref[:i]
			break
		}
	}
	if strings.HasSuffix(strings.ToLower(ref), ".json") {
		ref = strings.TrimSuffix(ref, ".json")
	}
	if !strings.HasPrefix(ref, "obs_") {
		return ""
	}
	return ref
}

func pluginPrepWorkerABResultLines(observe harness.InvokeResponse) []string {
	return pendingMixTickABResultLines(executor.Result{Result: observe.Result})
}

func pluginPrepWorkerInvokeRow(out harness.InvokeResponse) map[string]any {
	return map[string]any{
		"status":          out.Status,
		"agent_action_id": out.AgentActionID,
		"tool":            out.Tool,
		"command_name":    out.CommandName,
		"preview":         out.Preview,
		"undo_label":      out.UndoLabel,
		"result":          out.Result,
		"error":           out.Error,
	}
}

func pluginPrepWorkerInvokeFailed(out harness.InvokeResponse, err error) bool {
	status := strings.ToLower(strings.TrimSpace(out.Status))
	return err != nil || out.Error != "" || status == "error" || status == "kernel_error" || status == "failed"
}

func pluginPrepWorkerNoWriteResponse(interaction PendingInteraction, payload map[string]any, candidate agentprotocol.PendingCandidate, reply, stopReason string, status agentruntime.GoalStatus) ChatResponse {
	event := agentprotocol.ToMap(agentprotocol.NewEvent(candidate, candidate.Source))
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          reply,
		Workflow:       pluginPrepWorkerWorkflow,
		WorkflowData:   payload,
		PluginLearning: payload,
		TypedEvents:    []map[string]any{event, pluginPrepWorkerTerminalStateEvent(interaction, payload, strings.TrimPrefix(stopReason, "plugin_prep_worker_"), reply)},
		GoalStatus:     string(status),
		StopReason:     stopReason,
	}
}

func pluginPrepWorkerTerminalResponse(interaction PendingInteraction, payload map[string]any, status, reason string) ChatResponse {
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          reason,
		Workflow:       pluginPrepWorkerWorkflow,
		WorkflowData:   payload,
		PluginLearning: payload,
		TypedEvents:    []map[string]any{pluginPrepWorkerTerminalStateEvent(interaction, payload, status, reason)},
		GoalStatus:     string(agentruntime.StatusCompleted),
		StopReason:     status,
	}
}

func pluginPrepWorkerTerminalStateEvent(interaction PendingInteraction, payload map[string]any, status, reason string) map[string]any {
	terminal := agentprotocol.TerminalResult{
		ID:      agentprotocol.NormalizeID("terminal", pluginPrepWorkerWorkflow, interaction.ID, status),
		Kind:    agentprotocol.KindTerminalResult,
		Domain:  "plugin_prep",
		Summary: "Plugin Prep Worker 已进入终态。",
		Reason:  strings.TrimSpace(reason),
		Status:  strings.TrimSpace(status),
		Source: agentprotocol.Source{
			ConversationID: interaction.ConversationID,
			GoalID:         interaction.GoalID,
			RunID:          interaction.RunID,
			CallID:         interaction.ID,
			LegacyKind:     pluginPrepWorkerWorkflow,
		},
		Metadata: map[string]any{
			"pending_id":  cleanContextText(payload["pending_id"]),
			"track_id":    cleanContextText(payload["track_id"]),
			"plugin_id":   cleanContextText(payload["plugin_id"]),
			"plugin_name": cleanContextText(payload["plugin_name"]),
		},
	}
	return agentprotocol.ToMap(agentprotocol.NewEvent(terminal, terminal.Source))
}
