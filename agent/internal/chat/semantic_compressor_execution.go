package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/mom"
	"vit-daw-agent/internal/semanticeffect"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	semanticCompressorExecutionWorkflow = "semantic_compressor_execution"
	semanticCompressorExecutionSchema   = "semantic_effect.compressor_execution_ticket.v1"
	semanticCompressorReceiptSchema     = "semantic_effect.compressor_execution_receipt.v1"
	semanticCompressorExecutionTTL      = 15 * time.Minute
)

type compressorExecutionTicket struct {
	SchemaVersion       string                                      `json:"schema_version"`
	TicketID            string                                      `json:"ticket_id"`
	ConversationID      string                                      `json:"conversation_id,omitempty"`
	GoalID              string                                      `json:"goal_id,omitempty"`
	RunID               string                                      `json:"run_id,omitempty"`
	TrackID             string                                      `json:"track_id"`
	PluginID            string                                      `json:"plugin_id"`
	PluginName          string                                      `json:"plugin_name,omitempty"`
	TopologyClass       string                                      `json:"topology_class"`
	TopologyGeneration  string                                      `json:"topology_generation"`
	ParameterCount      int                                         `json:"parameter_count"`
	BaselineFingerprint string                                      `json:"baseline_fingerprint"`
	PlanFingerprint     string                                      `json:"plan_fingerprint"`
	Controls            []compressorExecutionControl                `json:"controls"`
	EvaluationContract  semanticeffect.CompressorEvaluationContract `json:"evaluation_contract"`
	Evidence            semanticeffect.CompressorPlanEvidence       `json:"evidence"`
	ObservationContext  map[string]any                              `json:"observation_context,omitempty"`
	CreatedAt           string                                      `json:"created_at"`
}

type compressorExecutionControl struct {
	ProposalID   string                                 `json:"proposal_id"`
	Axis         string                                 `json:"axis"`
	PathKey      string                                 `json:"path_key"`
	Role         string                                 `json:"role"`
	ControlRef   string                                 `json:"control_ref"`
	Target       semanticeffect.CompressorControlTarget `json:"target"`
	CurrentText  string                                 `json:"current_text,omitempty"`
	PhysicalUnit string                                 `json:"physical_unit,omitempty"`
	Purpose      string                                 `json:"purpose"`
	Confidence   string                                 `json:"confidence"`
}

func materializeSemanticCompressorPlan(plan semanticeffect.CompressorPlan, digest plugingrabber.ParameterDigest,
	summary, comContext map[string]any, conversationID, goalID, runID, pluginName string, requestContext map[string]any) (compressorExecutionTicket, string, error) {
	if err := plan.Validate(); err != nil {
		return compressorExecutionTicket{}, "", fmt.Errorf("invalid compressor plan: %w", err)
	}
	actualMode := firstNonEmptyText(comContext, "mode")
	actualStatus := firstNonEmptyText(comContext, "status")
	actualProjectionID := firstNonEmptyText(comContext, "projection_id")
	if (actualMode != com.ModePairedIO || actualStatus != com.StatusReady) &&
		!c2ReversibleCompressorEvidence(requestContext, actualMode, actualStatus) {
		return compressorExecutionTicket{}, "", fmt.Errorf("semantic execution requires paired_io / ready COM evidence or C2 reversible partial evidence (actual mode=%s status=%s reason=%s)",
			firstNonEmpty(actualMode, "missing"), firstNonEmpty(actualStatus, "missing"), firstNonEmptyText(comContext, "reason"))
	}
	if plan.Evidence.COMMode != actualMode || plan.Evidence.COMStatus != actualStatus ||
		(plan.Evidence.COMProjectionID != "" && plan.Evidence.COMProjectionID != actualProjectionID) {
		return compressorExecutionTicket{}, "", fmt.Errorf("compressor plan evidence does not match the deterministic COM projection")
	}
	trackID, pluginID := plan.Target.TrackID, plan.Target.PluginID
	if digest.TrackID != "" && digest.TrackID != trackID || digest.PluginID != "" && digest.PluginID != pluginID {
		return compressorExecutionTicket{}, "", fmt.Errorf("compressor plan target does not match the live target")
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" || generation != plan.TopologyGeneration {
		return compressorExecutionTicket{}, "", fmt.Errorf("topology generation is stale or unavailable")
	}
	if err := attachCompressorControlRefs(summary, trackID, pluginID); err != nil {
		return compressorExecutionTicket{}, "", fmt.Errorf("control reference materialization failed: %w", err)
	}
	controls := make([]compressorExecutionControl, 0, len(plan.Controls))
	seenControlRefs := map[string]bool{}
	for _, proposal := range plan.Controls {
		binding, bindingErr := semanticCompressorBinding(summary, proposal.PathKey, proposal.Role)
		if bindingErr != nil {
			return compressorExecutionTicket{}, "", fmt.Errorf("proposal %s cannot be bound: %w", proposal.ProposalID, bindingErr)
		}
		controlRef := firstNonEmptyText(binding, "control_ref")
		if controlRef == "" {
			return compressorExecutionTicket{}, "", fmt.Errorf("proposal %s has no generation-scoped control reference", proposal.ProposalID)
		}
		request, _ := compressorRequestForSemanticTarget(controlRef, proposal.Target)
		if _, _, err := planCompressorControl(summary, request, trackID, pluginID, generation); err != nil {
			return compressorExecutionTicket{}, "", fmt.Errorf("proposal %s is unreachable: %w", proposal.ProposalID, err)
		}
		if seenControlRefs[controlRef] {
			return compressorExecutionTicket{}, "", fmt.Errorf("multiple proposals target the same live compressor control")
		}
		seenControlRefs[controlRef] = true
		controls = append(controls, compressorExecutionControl{ProposalID: proposal.ProposalID, Axis: proposal.Axis,
			PathKey: proposal.PathKey, Role: proposal.Role, ControlRef: controlRef, Target: proposal.Target,
			CurrentText: firstNonEmptyText(binding, "current_text", "value_text"), PhysicalUnit: firstNonEmptyText(binding, "physical_unit", "unit"),
			Purpose: proposal.Purpose, Confidence: proposal.Confidence})
	}
	ticket := compressorExecutionTicket{SchemaVersion: semanticCompressorExecutionSchema, TicketID: "semexec_" + randomID(),
		ConversationID: conversationID, GoalID: goalID, RunID: runID, TrackID: trackID, PluginID: pluginID, PluginName: pluginName,
		TopologyClass: firstNonEmptyText(summary, "classification"), TopologyGeneration: generation,
		ParameterCount: digest.ParameterCount, BaselineFingerprint: compressorParameterBaselineFingerprint(digest),
		PlanFingerprint: compressorSemanticFingerprint(plan), Controls: controls, EvaluationContract: plan.EvaluationContract,
		Evidence: plan.Evidence, ObservationContext: compressorExecutionObservationContext(requestContext),
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	return ticket, compressorExecutionPreview(ticket), nil
}

func c2ReversibleCompressorEvidence(requestContext map[string]any, mode, status string) bool {
	if !contextBool(requestContext, "c2_source_only_reversible") {
		return false
	}
	if mode == com.ModeSourceOnly {
		return status == com.StatusReady || status == com.StatusPartial
	}
	// A paired partial projection contains at least the evidence available to
	// source-only planning. C2 may use it only under its bounded reversible
	// contract; post-action CCB remains mandatory and terminal status is review.
	return mode == com.ModePairedIO && status == com.StatusPartial
}

func semanticCompressorBinding(summary map[string]any, pathKey, role string) (map[string]any, error) {
	stage := mapValue(summary["compressor_stage"])
	matches := []map[string]any{}
	if pathKey == "stage_output" {
		for _, row := range mapRowsValue(stage["output"]) {
			if firstNonEmptyText(row, "role") == role {
				matches = append(matches, row)
			}
		}
	} else {
		for _, path := range mapRowsValue(stage["control_paths"]) {
			if firstNonEmptyText(path, "path_key") != pathKey {
				continue
			}
			for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
				for _, row := range mapRowsValue(path[section]) {
					if firstNonEmptyText(row, "role") == role {
						matches = append(matches, row)
					}
				}
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("role %s is absent from path %s in the live topology", role, pathKey)
	}
	if len(matches) != 1 {
		// Keep planning and execution bound to the same deterministic semantic
		// choice. A compressor may expose a binary input pad and a continuous
		// input gain under the structural input_drive role; activation intensity
		// can safely use the unique continuous binding. Other duplicate roles
		// remain an execution boundary.
		if role == "input_drive" {
			continuous := make([]map[string]any, 0, len(matches))
			for _, match := range matches {
				if compressorSummaryBindingIsContinuous(match) {
					continuous = append(continuous, match)
				}
			}
			if len(continuous) == 1 {
				return continuous[0], nil
			}
		}
		if preferred := preferredEquivalentCompressorSummaryBinding(matches); preferred != nil {
			return preferred, nil
		}
		return nil, fmt.Errorf("role %s on path %s has %d live bindings; exactly one is required", role, pathKey, len(matches))
	}
	return matches[0], nil
}

func preferredEquivalentCompressorSummaryBinding(matches []map[string]any) map[string]any {
	var preferred map[string]any
	signature := ""
	for _, match := range matches {
		if !compressorSummaryBindingIsContinuous(match) {
			return nil
		}
		surface := map[string]any{
			"physical_unit":    match["physical_unit"],
			"domain":           match["domain"],
			"curve":            match["curve"],
			"reachable_values": match["reachable_values"],
		}
		encoded, _ := json.Marshal(surface)
		if preferred == nil {
			preferred, signature = match, string(encoded)
			continue
		}
		if signature != string(encoded) {
			return nil
		}
	}
	return preferred
}

func compressorSummaryBindingIsContinuous(binding map[string]any) bool {
	if len(mapRowsValue(binding["reachable_values"])) > 0 {
		return false
	}
	domain := mapValue(binding["domain"])
	if domain["min"] != nil && domain["max"] != nil {
		return true
	}
	return binding["curve"] != nil
}

func compressorRequestForSemanticTarget(controlRef string, target semanticeffect.CompressorControlTarget) (compressorControlRequest, map[string]any) {
	request := compressorControlRequest{ControlRef: controlRef, Requested: map[string]any{"control_ref": controlRef}}
	requested := request.Requested
	switch {
	case target.ValueDB != nil:
		request.Value, request.Unit, requested["value_db"] = target.ValueDB, "dB", *target.ValueDB
	case target.Ratio != nil:
		request.Value, request.Unit, requested["ratio"] = target.Ratio, "ratio", *target.Ratio
	case target.ValueMS != nil:
		request.Value, request.Unit, requested["value_ms"] = target.ValueMS, "ms", *target.ValueMS
	case target.Percent != nil:
		request.Value, request.Unit, requested["percent"] = target.Percent, "%", *target.Percent
	case target.DisplayValue != nil:
		request.Value, request.Unit, requested["display_value"] = target.DisplayValue, "display", *target.DisplayValue
	default:
		request.EnumLabel, requested["enum_label"] = target.EnumLabel, target.EnumLabel
	}
	return request, requested
}

func compressorExecutionObservationContext(requestContext map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"selected_clip_id", "clip_id", "start_sample", "end_sample", "sample_rate", "channel_count"} {
		if value, ok := requestContext[key]; ok {
			out[key] = value
		}
	}
	return out
}

func compressorParameterBaselineFingerprint(digest plugingrabber.ParameterDigest) string {
	rows := make([]string, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		value := "text:" + strings.TrimSpace(param.ValueText)
		if normalized, ok := numericAny(param.NormalizedValue); ok {
			value = fmt.Sprintf("normalized:%.17g", normalized)
		}
		rows = append(rows, param.ID+"="+value)
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return "cpb1_" + hex.EncodeToString(sum[:16])
}

func compressorSemanticFingerprint(plan semanticeffect.CompressorPlan) string {
	data, _ := json.Marshal(plan)
	sum := sha256.Sum256(data)
	return "csp1_" + hex.EncodeToString(sum[:16])
}

func compressorExecutionPreview(ticket compressorExecutionTicket) string {
	lines := []string{fmt.Sprintf("将对 %s 执行单段宽带压缩器语义方案：", firstNonEmpty(ticket.PluginName, "当前压缩器"))}
	for _, control := range ticket.Controls {
		lines = append(lines, fmt.Sprintf("- [%s] %s (%s): %s -> %s", control.Axis, control.Role, control.PathKey,
			firstNonEmpty(control.CurrentText, "当前值"), compressorTargetText(control.Target)))
	}
	lines = append(lines, "确认后才会写入；执行后将回读参数并生成一次只读 COM change_delta 评价。")
	return strings.Join(lines, "\n")
}

func compressorTargetText(target semanticeffect.CompressorControlTarget) string {
	switch {
	case target.ValueDB != nil:
		return fmt.Sprintf("%.3g dB", *target.ValueDB)
	case target.Ratio != nil:
		return fmt.Sprintf("%.3g:1", *target.Ratio)
	case target.ValueMS != nil:
		return fmt.Sprintf("%.3g ms", *target.ValueMS)
	case target.Percent != nil:
		return fmt.Sprintf("%.3g%%", *target.Percent)
	case target.DisplayValue != nil:
		return fmt.Sprintf("%.3g", *target.DisplayValue)
	default:
		return target.EnumLabel
	}
}

func compressorExecutionTicketMap(ticket compressorExecutionTicket) map[string]any {
	data, _ := json.Marshal(ticket)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return out
}

func compressorExecutionTicketFromMap(value any) (compressorExecutionTicket, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return compressorExecutionTicket{}, err
	}
	var ticket compressorExecutionTicket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return ticket, err
	}
	if ticket.SchemaVersion != semanticCompressorExecutionSchema || ticket.TicketID == "" || ticket.TrackID == "" || ticket.PluginID == "" || len(ticket.Controls) == 0 || ticket.CreatedAt == "" {
		return ticket, fmt.Errorf("invalid compressor execution ticket")
	}
	return ticket, nil
}

func validateSemanticCompressorExecutionTicket(ticket compressorExecutionTicket, interaction PendingInteraction, now time.Time) error {
	createdAt, err := time.Parse(time.RFC3339Nano, ticket.CreatedAt)
	if err != nil {
		return fmt.Errorf("compressor execution ticket has an invalid creation time")
	}
	if now.Before(createdAt.Add(-time.Minute)) || !now.Before(createdAt.Add(semanticCompressorExecutionTTL)) {
		return fmt.Errorf("compressor execution ticket expired")
	}
	if ticket.ConversationID != interaction.ConversationID || ticket.GoalID != interaction.GoalID || ticket.RunID != interaction.RunID || ticket.TicketID != interaction.PlanID {
		return fmt.Errorf("compressor execution ticket does not belong to this interaction")
	}
	return nil
}

func compressorExecutionControlRows(ticket compressorExecutionTicket) []map[string]any {
	rows := make([]map[string]any, 0, len(ticket.Controls))
	for _, control := range ticket.Controls {
		_, requested := compressorRequestForSemanticTarget(control.ControlRef, control.Target)
		rows = append(rows, requested)
	}
	return rows
}

func semanticCompressorExecutionRequest(ticket compressorExecutionTicket) map[string]any {
	return map[string]any{"cmd": pluginGrabberApplyCompressorCommand, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID,
		"atomic": true, "controls": compressorExecutionControlRows(ticket), "semantic_execution_ticket": ticket.TicketID,
		"topology_generation": ticket.TopologyGeneration, "baseline_fingerprint": ticket.BaselineFingerprint}
}

func semanticCompressorProjectionTyped(value any) *com.Projection {
	switch projection := value.(type) {
	case *com.Projection:
		return projection
	case com.Projection:
		copy := projection
		return &copy
	case map[string]any:
		data, err := json.Marshal(projection)
		if err == nil {
			var out com.Projection
			if json.Unmarshal(data, &out) == nil {
				return &out
			}
		}
	}
	return nil
}

func captureSemanticCompressorPairedProjection(ctx context.Context, s *Server, ticket compressorExecutionTicket) (*com.Projection, error) {
	if s == nil || s.harness == nil {
		return nil, fmt.Errorf("harness unavailable for COM paired observation")
	}
	args := map[string]any{"scope": "selected_track", "project_context": true, "observation_only": true,
		"track_id": ticket.TrackID, "plugin_id": ticket.PluginID, "com_mode": com.ModePairedIO,
		"mom_intent":     mom.IntentGeneralBandStereoObservation,
		"topology_class": ticket.TopologyClass, "topology_generation": ticket.TopologyGeneration,
		"mix_session_id": "compressor_execution_" + sanitizeCanaryID(ticket.TicketID) + "_" + fmt.Sprint(time.Now().UnixNano()),
		"goal_text":      "semantic compressor execution evidence", "target_ref": map[string]any{"kind": "plugin", "id": ticket.PluginID, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID}}
	for key, value := range ticket.ObservationContext {
		args[key] = value
	}
	if _, ok := args["start_sample"]; !ok {
		return nil, fmt.Errorf("paired COM observation requires an exact sample window")
	}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{Tool: "mix.observe", Args: args,
		Context: map[string]any{"observation_only": true, "semantic_compressor_execution": true}, Source: "semantic_compressor_execution", Confirmed: true,
		ToolCallID: "compressor_execution:" + sanitizeCanaryID(ticket.TicketID) + ":mix.observe"})
	if err != nil || response.Status != "ok" {
		return nil, fmt.Errorf("paired COM observation failed: %s", firstNonEmpty(errorText(err), response.Error, response.Status))
	}
	projection := semanticCompressorProjectionTyped(response.Result["com_projection"])
	if projection == nil || projection.Mode != com.ModePairedIO || (projection.Status != com.StatusReady && projection.Status != com.StatusPartial) {
		return nil, fmt.Errorf("paired COM observation omitted a usable projection")
	}
	return projection, nil
}

func compressorExecutionParameterAudit(before, after plugingrabber.ParameterDigest, ticket compressorExecutionTicket) (map[string]any, error) {
	if before.ParameterCount != after.ParameterCount || before.ParameterCount != ticket.ParameterCount {
		return nil, fmt.Errorf("parameter count changed during compressor execution")
	}
	beforeRows := map[string]plugingrabber.ParameterInfo{}
	afterRows := map[string]plugingrabber.ParameterInfo{}
	for _, row := range before.Parameters {
		beforeRows[row.ID] = row
	}
	for _, row := range after.Parameters {
		afterRows[row.ID] = row
	}
	if len(beforeRows) != len(afterRows) {
		return nil, fmt.Errorf("parameter identity set changed during compressor execution")
	}
	targetIDs := map[string]bool{}
	for _, control := range ticket.Controls {
		ref, err := decodeCompressorControlRef(control.ControlRef)
		if err != nil {
			return nil, err
		}
		targetIDs[ref.ParamID] = true
	}
	for id, beforeRow := range beforeRows {
		afterRow, ok := afterRows[id]
		if !ok {
			return nil, fmt.Errorf("parameter %s disappeared during execution", id)
		}
		if targetIDs[id] {
			continue
		}
		if beforeValue, beforeOK := numericAny(beforeRow.NormalizedValue); beforeOK {
			afterValue, afterOK := numericAny(afterRow.NormalizedValue)
			if !afterOK || beforeValue != afterValue {
				return nil, fmt.Errorf("non-target compressor parameter %s changed during execution", id)
			}
		} else if beforeRow.ValueText != afterRow.ValueText {
			return nil, fmt.Errorf("non-target compressor parameter %s changed during execution", id)
		}
	}
	return map[string]any{"status": "pass", "parameter_count": after.ParameterCount,
		"before_fingerprint": compressorParameterBaselineFingerprint(before), "after_fingerprint": compressorParameterBaselineFingerprint(after),
		"target_parameter_count": len(targetIDs), "non_target_parameters_unchanged": true}, nil
}

func validateSemanticCompressorExecutionFreshness(ticket compressorExecutionTicket, digest plugingrabber.ParameterDigest,
	summary map[string]any) error {
	if digest.TrackID != "" && digest.TrackID != ticket.TrackID || digest.PluginID != "" && digest.PluginID != ticket.PluginID {
		return fmt.Errorf("compressor target changed after materialization")
	}
	if firstNonEmptyText(summary, "classification") != ticket.TopologyClass ||
		firstNonEmptyText(mapValue(summary["control_topology"]), "generation") != ticket.TopologyGeneration {
		return fmt.Errorf("compressor topology changed after materialization")
	}
	if digest.ParameterCount != ticket.ParameterCount || compressorParameterBaselineFingerprint(digest) != ticket.BaselineFingerprint {
		return fmt.Errorf("compressor parameter baseline changed after materialization")
	}
	return nil
}

func compressorExecutionVisibleControls(ticket compressorExecutionTicket) []map[string]any {
	rows := make([]map[string]any, 0, len(ticket.Controls))
	for _, control := range ticket.Controls {
		rows = append(rows, map[string]any{"proposal_id": control.ProposalID, "axis": control.Axis, "path_key": control.PathKey,
			"role": control.Role, "current_text": control.CurrentText, "physical_unit": control.PhysicalUnit,
			"target": control.Target, "purpose": control.Purpose, "confidence": control.Confidence})
	}
	return rows
}

func (s *Server) semanticCompressorWaitingResponse(conversationID string, requestContext map[string]any,
	card semanticeffect.AudioProcessorIdentityCard, intent semanticeffect.CompressorIntentPlan, comContext map[string]any,
	brief plugingrabber.CompressorControlBrief, plan semanticeffect.CompressorPlan, ticket compressorExecutionTicket, preview string) ChatResponse {
	if _, hasProgressiveState := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); hasProgressiveState {
		refs := make([]string, 0, len(ticket.Controls))
		for _, control := range ticket.Controls {
			refs = append(refs, control.ControlRef)
		}
		if err := semanticProgressiveDisclosureAdvanceToConfirmation(requestContext, refs); err != nil {
			return semanticCompressorExecutionMaterializationFailure(conversationID, requestContext, card, intent,
				comContext, brief, plan, fmt.Errorf("progressive disclosure boundary: %w", err))
		}
	}
	workflowData := map[string]any{"schema_version": "semantic_compressor.workflow.v2", "status": "waiting_confirmation",
		"planning_only": true, "mutation_authorized": false, "mutation_performed": false,
		"processor_identity_card": card, "intent_plan": intent, "com_observation": comContext, "control_brief": brief,
		"compressor_plan": plan, "execution": map[string]any{"schema_version": semanticCompressorExecutionSchema,
			"status": "waiting_confirmation", "ticket_id": ticket.TicketID, "preview": preview,
			"controls": compressorExecutionVisibleControls(ticket)},
		"progressive_disclosure": []string{"processor_identity_card", "com_observation", "candidate_control_brief", "execution_preview"}}
	goalID, runID := goalIDsFromContext(requestContext)
	interactionID := "interaction_" + randomID()
	interaction := AgentInteractionRequest{ID: interactionID, Kind: "confirmation", Type: "confirmation", Source: "vit_agent",
		Title: "确认执行压缩器语义方案", Body: preview, Status: "waiting_for_user", Workflow: semanticCompressorExecutionWorkflow,
		Stage: "waiting_for_user", PlanID: ticket.TicketID, ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Payload: map[string]any{"ticket_id": ticket.TicketID, "preview": preview, "workflow": semanticCompressorExecutionWorkflow},
		Actions: []AgentInteractionAction{{ID: "approve", Label: "确认执行", Style: "primary", Recommended: true},
			{ID: "cancel", Label: "取消", Style: "secondary"}}}
	workflowData["interaction_id"] = interactionID
	workflowData["execution_context"] = map[string]any{"track_id": ticket.TrackID, "plugin_id": ticket.PluginID,
		"topology_generation": ticket.TopologyGeneration}
	workflowData = semanticCompressorWorkflowData(requestContext, workflowData)
	s.storeSemanticCompressorExecutionInteraction(interaction, requestContext,
		map[string]any{"workflow_data": workflowData, "ticket": compressorExecutionTicketMap(ticket)})
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply: "方案已完成确定性绑定，等待你的确认。确认后才会写入压缩器参数。", NeedsConfirmation: true,
		PlanID: ticket.TicketID, Preview: preview, Workflow: semanticCompressorExecutionWorkflow, WorkflowData: workflowData,
		InteractionRequests: []AgentInteractionRequest{interaction}, GoalStatus: "waiting_confirmation",
		StopReason: "compressor_execution_confirmation_required"}
}

func (s *Server) storeSemanticCompressorExecutionInteraction(req AgentInteractionRequest, requestContext, data map[string]any) {
	if s == nil || req.ID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interactions[req.ID] = PendingInteraction{ID: req.ID, CreatedAt: time.Now(), Kind: req.Kind, Source: req.Source,
		Workflow: req.Workflow, Stage: req.Stage, PlanID: req.PlanID, ConversationID: req.ConversationID, GoalID: req.GoalID,
		RunID: req.RunID, RequestContext: cloneContext(requestContext), Payload: cloneStringAnyMap(req.Payload), Type: req.Type,
		Data: cloneStringAnyMap(data)}
}

func (s *Server) continueSemanticCompressorExecutionInteraction(ctx context.Context, interaction PendingInteraction, decision string) ChatResponse {
	ticket, err := compressorExecutionTicketFromMap(interaction.Data["ticket"])
	if err != nil {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply: "压缩器执行票据无效，已停止且没有修改参数。", Workflow: semanticCompressorExecutionWorkflow,
			GoalStatus: "failed", Error: err.Error(), StopReason: "invalid_compressor_execution_ticket"}
	}
	clean := strings.ToLower(strings.TrimSpace(decision))
	if clean == "cancel" || clean == "deny" || clean == "reject" {
		data := mapValue(interaction.Data["workflow_data"])
		data["status"], data["mutation_performed"] = "cancelled", false
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply: "已取消压缩器语义方案，插件参数没有修改。", Workflow: semanticCompressorExecutionWorkflow,
			WorkflowData: data, GoalStatus: "cancelled", StopReason: "compressor_execution_cancelled"}
	}
	if !isApprovalDecision(clean) {
		return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
			Reply: "请明确选择确认执行或取消。", Workflow: semanticCompressorExecutionWorkflow,
			GoalStatus: "failed", Error: "explicit approval or cancellation required", StopReason: "invalid_confirmation_decision"}
	}
	if err := validateSemanticCompressorExecutionTicket(ticket, interaction, time.Now().UTC()); err != nil {
		return compressorExecutionFailureResponse(interaction, ticket, "expired_or_mismatched_compressor_execution_ticket", err, nil, nil)
	}
	requestContext := interaction.RequestContext
	if _, hasProgressiveState := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); hasProgressiveState {
		if err := semanticProgressiveDisclosureConfirm(requestContext); err != nil {
			return compressorExecutionFailureResponse(interaction, ticket, "progressive_disclosure_confirmation_invalid", err, nil, nil)
		}
	}
	response := s.executeSemanticCompressorTicket(ctx, interaction, ticket)
	if _, hasProgressiveState := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); hasProgressiveState {
		if receipt := firstMapFromAny(response.WorkflowData["execution_receipt"]); len(receipt) > 0 {
			if err := semanticProgressiveDisclosureReceipt(requestContext, receipt); err != nil {
				return compressorExecutionFailureResponse(interaction, ticket, "progressive_disclosure_receipt_invalid", err, nil, receipt)
			}
			if response.WorkflowData == nil {
				response.WorkflowData = map[string]any{}
			}
			response.WorkflowData["semantic_progressive_disclosure"] = requestContext["semantic_progressive_disclosure"]
			response.WorkflowData["request_context"] = cloneContext(requestContext)
		}
	}
	return response
}

func (s *Server) executeSemanticCompressorTicket(ctx context.Context, interaction PendingInteraction, ticket compressorExecutionTicket) ChatResponse {
	digest, summary, err := s.readLiveCompressorControlSurface(ctx, ticket.TrackID, ticket.PluginID)
	if err == nil {
		err = validateSemanticCompressorExecutionFreshness(ticket, digest, summary)
	}
	if err != nil {
		return compressorExecutionFailureResponse(interaction, ticket, "stale_or_unavailable_target", err, nil, nil)
	}
	if err := attachCompressorControlRefs(summary, ticket.TrackID, ticket.PluginID); err != nil {
		return compressorExecutionFailureResponse(interaction, ticket, "materialization_invalidated", err, nil, nil)
	}
	for _, control := range ticket.Controls {
		binding, bindingErr := semanticCompressorBinding(summary, control.PathKey, control.Role)
		if bindingErr != nil || firstNonEmptyText(binding, "control_ref") != control.ControlRef {
			if bindingErr == nil {
				bindingErr = fmt.Errorf("control reference changed after materialization")
			}
			return compressorExecutionFailureResponse(interaction, ticket, "materialization_invalidated", bindingErr, nil, nil)
		}
		request, _ := compressorRequestForSemanticTarget(control.ControlRef, control.Target)
		if _, _, planErr := planCompressorControl(summary, request, ticket.TrackID, ticket.PluginID, ticket.TopologyGeneration); planErr != nil {
			return compressorExecutionFailureResponse(interaction, ticket, "materialization_invalidated", planErr, nil, nil)
		}
	}
	sourceOnlyReversible := contextBool(interaction.RequestContext, "c2_source_only_reversible")
	var beforeProjection *com.Projection
	if !sourceOnlyReversible {
		beforeProjection, err = captureSemanticCompressorPairedProjection(ctx, s, ticket)
		if err != nil {
			return compressorExecutionFailureResponse(interaction, ticket, "before_com_observation_failed", err, nil, nil)
		}
	}
	applyResult, err := s.applyPluginGrabberCompressorControls(ctx, semanticCompressorExecutionRequest(ticket), interaction.RequestContext)
	if err != nil {
		return compressorExecutionFailureResponse(interaction, ticket, "compressor_atomic_execution_failed", err, applyResult, nil)
	}
	if resultErr := validateCompressorExecutionResult(applyResult, ticket); resultErr != nil {
		return s.rollbackSemanticCompressorFailure(ctx, interaction, ticket, digest, applyResult, "controller_result_invalid", resultErr)
	}
	afterDigest, _, readErr := s.readLiveCompressorControlSurface(ctx, ticket.TrackID, ticket.PluginID)
	if readErr != nil {
		return s.rollbackSemanticCompressorFailure(ctx, interaction, ticket, digest, applyResult, "post_execution_readback_failed", readErr)
	}
	parameterAudit, auditErr := compressorExecutionParameterAudit(digest, afterDigest, ticket)
	if auditErr != nil {
		return s.rollbackSemanticCompressorFailure(ctx, interaction, ticket, digest, applyResult, "parameter_audit_failed", auditErr)
	}
	var afterProjection *com.Projection
	var afterCOMErr error
	if !sourceOnlyReversible {
		afterProjection, afterCOMErr = captureSemanticCompressorPairedProjection(ctx, s, ticket)
	} else {
		afterCOMErr = fmt.Errorf("source_only reversible C2 execution; paired COM change_delta deferred to post-action observation")
	}
	receipt := map[string]any{"schema_version": semanticCompressorReceiptSchema, "status": "executed", "ticket_id": ticket.TicketID,
		"track_id": ticket.TrackID, "plugin_id": ticket.PluginID, "topology_generation": ticket.TopologyGeneration,
		"parameter_audit": parameterAudit, "controller_result": compressorExecutionResultSummary(applyResult),
		"rollback": map[string]any{"status": "not_needed", "verified": true}}
	if afterCOMErr != nil {
		receipt["com_evaluation"] = map[string]any{"status": "unavailable", "reason": afterCOMErr.Error()}
	} else {
		delta := com.Build(com.Input{Mode: com.ModeChangeDelta, ObservationID: ticket.TicketID,
			MixSessionID: "compressor_execution_" + ticket.TicketID,
			TargetRef:    map[string]any{"kind": "plugin", "id": ticket.PluginID, "track_id": ticket.TrackID, "plugin_id": ticket.PluginID},
			Change:       &com.ChangeDeltaInput{Before: beforeProjection, After: afterProjection}})
		receipt["com_evaluation"] = com.ContextProjection(delta)
	}
	return compressorExecutionSuccessResponse(interaction, ticket, receipt)
}

func compressorExecutionResultSummary(result map[string]any) map[string]any {
	if len(result) == 0 {
		return map[string]any{"status": "missing"}
	}
	return map[string]any{"status": result["status"], "atomic": result["atomic"],
		"control_count": len(mapRowsValue(result["controls"])), "restore_ref": firstNonEmptyText(result, "restore_ref"), "rollback": result["rollback"]}
}

func validateCompressorExecutionResult(result map[string]any, ticket compressorExecutionTicket) error {
	status := firstNonEmptyText(result, "status")
	if status != "exact" && status != "quantized" {
		return fmt.Errorf("compressor controller returned invalid status %q", status)
	}
	if !boolValue(result["atomic"]) {
		return fmt.Errorf("compressor controller did not attest atomic execution")
	}
	if firstNonEmptyText(result, "track_id") != ticket.TrackID || firstNonEmptyText(result, "plugin_id") != ticket.PluginID ||
		firstNonEmptyText(result, "topology_generation") != ticket.TopologyGeneration {
		return fmt.Errorf("compressor controller result target or topology does not match the ticket")
	}
	if firstNonEmptyText(result, "restore_ref") == "" {
		return fmt.Errorf("compressor controller omitted its restore reference")
	}
	controls := mapRowsValue(result["controls"])
	if len(controls) != len(ticket.Controls) {
		return fmt.Errorf("compressor controller returned %d controls for %d ticket controls", len(controls), len(ticket.Controls))
	}
	for index, control := range controls {
		controlStatus := firstNonEmptyText(control, "status")
		if controlStatus != "exact" && controlStatus != "quantized" {
			return fmt.Errorf("compressor controller control %d has invalid status %q", index, controlStatus)
		}
		if len(mapRowsValue(control["actual_readback"])) == 0 {
			return fmt.Errorf("compressor controller control %d omitted actual readback", index)
		}
	}
	return nil
}

func (s *Server) rollbackSemanticCompressorFailure(ctx context.Context, interaction PendingInteraction,
	ticket compressorExecutionTicket, _ plugingrabber.ParameterDigest, applyResult map[string]any, code string, cause error) ChatResponse {
	rollbackStatus := "unavailable"
	var rollbackErr error
	if restoreRef := firstNonEmptyText(applyResult, "restore_ref"); restoreRef != "" {
		current, _, currentErr := s.readLiveCompressorControlSurface(ctx, ticket.TrackID, ticket.PluginID)
		if currentErr != nil {
			rollbackErr = currentErr
		} else {
			_, rollbackErr = s.restorePluginGrabberCompressorControls(ctx, ticket.TrackID, ticket.PluginID,
				ticket.TopologyGeneration, current, restoreRef)
		}
		if rollbackErr == nil {
			verify, _, verifyErr := s.readLiveCompressorControlSurface(ctx, ticket.TrackID, ticket.PluginID)
			if verifyErr == nil && compressorParameterBaselineFingerprint(verify) == ticket.BaselineFingerprint {
				rollbackStatus = "restored"
			}
		}
	}
	receipt := map[string]any{"schema_version": semanticCompressorReceiptSchema, "status": "rolled_back",
		"ticket_id": ticket.TicketID, "failure_code": code, "failure": cause.Error(),
		"controller_result": compressorExecutionResultSummary(applyResult),
		"rollback":          map[string]any{"status": rollbackStatus, "verified": rollbackStatus == "restored", "error": errorText(rollbackErr)}}
	return compressorExecutionFailureResponse(interaction, ticket, code, cause, applyResult, receipt)
}

func compressorExecutionFailureResponse(interaction PendingInteraction, ticket compressorExecutionTicket, code string,
	err error, applyResult, receipt map[string]any) ChatResponse {
	mutationPerformed := compressorExecutionMutationRetained(applyResult, receipt)
	data := map[string]any{"schema_version": "semantic_compressor.workflow.v2", "status": "failed", "planning_only": false,
		"mutation_authorized": true, "mutation_performed": mutationPerformed,
		"execution": map[string]any{"ticket_id": ticket.TicketID, "failure_code": code, "message": err.Error()}}
	if len(receipt) > 0 {
		data["execution_receipt"] = receipt
	}
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply: "压缩器语义执行未能安全完成；已停止并按事务状态处理，没有继续自动调参。", Error: err.Error(),
		Workflow: semanticCompressorExecutionWorkflow, WorkflowData: data, GoalStatus: "failed", StopReason: code}
}

func compressorExecutionMutationRetained(applyResult, receipt map[string]any) bool {
	status := firstNonEmptyText(applyResult, "status")
	if status != "exact" && status != "quantized" {
		return false
	}
	rollback := mapValue(receipt["rollback"])
	return firstNonEmptyText(rollback, "status") != "restored"
}

func compressorExecutionSuccessResponse(interaction PendingInteraction, ticket compressorExecutionTicket, receipt map[string]any) ChatResponse {
	data := map[string]any{"schema_version": "semantic_compressor.workflow.v2", "status": "executed", "planning_only": false,
		"mutation_authorized": true, "mutation_performed": true,
		"execution": map[string]any{"ticket_id": ticket.TicketID, "status": "executed"}, "execution_receipt": receipt}
	reply := "已按确认的压缩器语义方案完成参数调整，并通过回读审计；COM change_delta 评价已生成。"
	stopReason := "compressor_semantic_execution_completed"
	if firstNonEmptyText(mapValue(receipt["com_evaluation"]), "status") == "unavailable" {
		reply = "已按确认的压缩器语义方案完成参数调整并通过回读审计；本次 COM 后测不可用，未生成 change_delta，也没有继续自动调参。"
		stopReason = "compressor_semantic_execution_completed_without_com_evaluation"
	}
	return ChatResponse{ConversationID: interaction.ConversationID, GoalID: interaction.GoalID, RunID: interaction.RunID,
		Reply:    reply,
		Workflow: semanticCompressorExecutionWorkflow, WorkflowData: data, GoalStatus: "completed",
		StopReason: stopReason}
}

func semanticCompressorExecutionMaterializationFailure(conversationID string, requestContext map[string]any,
	card semanticeffect.AudioProcessorIdentityCard, intent semanticeffect.CompressorIntentPlan, comContext map[string]any,
	brief plugingrabber.CompressorControlBrief, plan semanticeffect.CompressorPlan, err error) ChatResponse {
	goalID, runID := goalIDsFromContext(requestContext)
	data := map[string]any{"schema_version": "semantic_compressor.workflow.v2", "status": "materialization_rejected",
		"planning_only": true, "mutation_authorized": false, "mutation_performed": false,
		"processor_identity_card": card, "intent_plan": intent, "com_observation": comContext,
		"control_brief": brief, "compressor_plan": plan,
		"execution": map[string]any{"status": "rejected", "failure_code": "execution_materialization_rejected", "message": err.Error()}}
	data = semanticCompressorWorkflowData(requestContext, data)
	return ChatResponse{ConversationID: conversationID, GoalID: goalID, RunID: runID,
		Reply: "压缩器语义方案已生成，但不满足安全执行条件；没有修改任何参数。", Workflow: semanticCompressorExecutionWorkflow,
		WorkflowData: data, GoalStatus: "completed", StopReason: "execution_materialization_rejected"}
}
