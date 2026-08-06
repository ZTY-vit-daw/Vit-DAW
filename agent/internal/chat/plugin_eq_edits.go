package chat

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberApplyEQEditsCommand = "plugin_grabber_apply_eq_edits"
	pluginGrabberApplyEQEditsTool    = "plugin_grabber.apply_eq_edits"
	eqOperationJournalCapacity       = 256
)

type eqEditRequest struct {
	Action        string
	Shape         string
	FrequencyHz   *float64
	GainDB        *float64
	Q             *float64
	SlopeDBPerOct *float64
	ControlRef    string
	OperationRef  string
}

type eqPlannedEdit struct {
	Index      int
	Request    eqEditRequest
	Section    map[string]any
	SectionKey string
	Shape      string
	Writes     []eqWriteStep
	Selection  map[string]any
	Quantized  bool
	ControlRef string
	Requested  map[string]any
}

type eqOperationRecord struct {
	OperationRef       string
	TrackID            string
	PluginID           string
	TopologyGeneration string
	Preimage           []eqPreimageValue
	Postimage          map[string]float64
	ControlRefs        []string
	CreatedAt          time.Time
	Undone             bool
}

type eqControlReference struct {
	Version            string `json:"v"`
	TrackID            string `json:"track"`
	PluginID           string `json:"plugin"`
	TopologyGeneration string `json:"generation"`
	Section            string `json:"section"`
	Shape              string `json:"shape"`
}

type eqControlFailure struct {
	Code    string
	Message string
}

func (e *eqControlFailure) Error() string { return e.Code + ": " + e.Message }

func rejectEQControl(code, format string, args ...any) error {
	return &eqControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}

func eqControlFailureCode(err error) string {
	if typed, ok := err.(*eqControlFailure); ok {
		return typed.Code
	}
	return "eq_control_failed"
}

func pluginGrabberApplyEQEditsInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName == pluginGrabberApplyEQEditsTool || toolName == pluginGrabberApplyEQEditsCommand {
		cmd := map[string]any{"cmd": pluginGrabberApplyEQEditsCommand}
		for key, value := range req.Args {
			cmd[key] = value
		}
		return cmd, true
	}
	args := workflowCommandArgs(req.Command)
	name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(args["command"]))
	}
	return args, name == pluginGrabberApplyEQEditsCommand
}

func (s *Server) invokePluginGrabberApplyEQEditsWorkflow(ctx context.Context, req harness.InvokeRequest,
	workflowCmd map[string]any) (harness.InvokeResponse, error) {
	requestContext := req.Context
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	result, err := s.applyPluginGrabberEQEdits(ctx, workflowCmd, requestContext)
	out := harness.InvokeResponse{
		Status: "ok", Tool: pluginGrabberApplyEQEditsTool, CommandName: pluginGrabberApplyEQEditsCommand,
		RiskLevel: tools.RiskUndoable, RequiresConfirmation: false, UndoLabel: "Undo EQ edits", Result: result,
	}
	if err != nil {
		out.Status = "error"
		out.Error = err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = eqControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberEQEdits(ctx context.Context, workflowCmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(workflowCmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectEQControl("atomic_required", "generic EQ edits only support atomic=true")
	}
	edits, err := parseEQEditRequests(args)
	if err != nil {
		return nil, err
	}
	target, err := s.resolvePluginObservationTarget(ctx, workflowCmd, requestContext, "")
	if err != nil {
		return nil, rejectEQControl("target_unavailable", "%v", err)
	}
	if s.eqKernelClient() == nil {
		return nil, rejectEQControl("kernel_unavailable", "kernel client is nil")
	}
	if len(edits) == 1 && edits[0].Action == "undo" {
		return s.undoEQOperation(ctx, target.TrackID, target.PluginID, edits[0].OperationRef)
	}
	for _, edit := range edits {
		if edit.Action == "undo" {
			return nil, rejectEQControl("undo_must_be_standalone", "undo cannot be mixed with forward edits")
		}
	}

	digest, summary, err := s.readLiveEQControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := eqTopologyGenerationFromSummary(summary)
	if generation == "" {
		return nil, rejectEQControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}

	reserved := map[string]bool{}
	plans := make([]eqPlannedEdit, 0, len(edits))
	for index, edit := range edits {
		plan, planErr := planEQEdit(summary, edit, target.TrackID, target.PluginID, generation, reserved)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_edit_index": index,
				"rejection_code": eqControlFailureCode(planErr)}, planErr
		}
		plan.Index = index
		reserved[plan.SectionKey] = true
		plans = append(plans, plan)
	}
	writes, err := combineAtomicEQWrites(plans)
	if err != nil {
		return nil, err
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectEQControl("preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, target.TrackID, target.PluginID, writes, preimage,
		eqParameterSnapshot(digest))
	if err != nil {
		if _, ok := err.(*eqControlFailure); ok {
			return nil, err
		}
		return nil, rejectEQControl("atomic_execution_failed", "%v", err)
	}

	operationRef := "eqop_" + randomID()
	postimage := map[string]float64{}
	for _, row := range actual {
		if value, ok := firstNumericAny(row, "normalized"); ok {
			postimage[firstNonEmptyText(row, "param_id")] = value
		}
	}
	controlRefs := make([]string, 0, len(plans))
	editResults := make([]map[string]any, 0, len(plans))
	overall := "exact"
	for _, plan := range plans {
		if plan.Quantized {
			overall = "quantized"
		}
		controlRefs = append(controlRefs, plan.ControlRef)
		result := map[string]any{
			"index": plan.Index, "action": plan.Request.Action, "shape": plan.Shape,
			"status":      map[bool]string{true: "quantized", false: "exact"}[plan.Quantized],
			"control_ref": plan.ControlRef, "requested": plan.Requested, "selection": plan.Selection,
			"actual_readback": eqActualRowsForWrites(actual, plan.Writes),
		}
		editResults = append(editResults, result)
	}
	s.storeEQOperation(eqOperationRecord{OperationRef: operationRef, TrackID: target.TrackID, PluginID: target.PluginID,
		TopologyGeneration: generation, Preimage: preimage, Postimage: postimage, ControlRefs: controlRefs, CreatedAt: time.Now()})
	return map[string]any{
		"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID,
		"topology_generation": generation, "operation_ref": operationRef, "edits": editResults,
		"writes": executed, "rollback": map[string]any{"available": true, "operation_ref": operationRef,
			"lifetime": "process", "expires_on_restart": true, "journal_capacity": eqOperationJournalCapacity},
	}, nil
}

func parseEQEditRequests(args map[string]any) ([]eqEditRequest, error) {
	rows := mapRowsValue(args["edits"])
	if len(rows) == 0 {
		return nil, rejectEQControl("edits_required", "edits must contain at least one EQ edit")
	}
	out := make([]eqEditRequest, 0, len(rows))
	for index, row := range rows {
		action := strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "action")))
		switch action {
		case "upsert", "modify", "disable", "remove", "undo":
		default:
			return nil, rejectEQControl("invalid_action", "edit %d has unsupported action %q", index, action)
		}
		shape := ""
		if rawShape := firstNonEmptyText(row, "shape", "filter_type"); rawShape != "" {
			canonical, err := normalizeRequestedEQShape(rawShape)
			if err != nil {
				return nil, rejectEQControl("unsupported_shape", "edit %d: %v", index, err)
			}
			shape = canonical
		}
		edit := eqEditRequest{Action: action, Shape: shape,
			ControlRef: firstNonEmptyText(row, "control_ref"), OperationRef: firstNonEmptyText(row, "operation_ref")}
		var err error
		edit.FrequencyHz, err = parseOptionalEQEditNumber(row, "frequency_hz", "freq_hz")
		if err == nil {
			edit.GainDB, err = parseOptionalEQEditNumber(row, "gain_db")
		}
		if err == nil {
			edit.Q, err = parseOptionalEQEditNumber(row, "q")
		}
		if err == nil {
			edit.SlopeDBPerOct, err = parseOptionalEQEditNumber(row, "slope_db_per_oct")
		}
		if err != nil {
			return nil, rejectEQControl("invalid_field", "edit %d: %v", index, err)
		}
		if edit.FrequencyHz != nil && *edit.FrequencyHz <= 0 || edit.Q != nil && *edit.Q <= 0 ||
			edit.SlopeDBPerOct != nil && *edit.SlopeDBPerOct <= 0 {
			return nil, rejectEQControl("invalid_field", "edit %d has a non-positive frequency/Q/slope", index)
		}
		if action == "upsert" && (shape == "" || edit.FrequencyHz == nil) {
			return nil, rejectEQControl("required_field_missing", "edit %d upsert requires shape and frequency_hz", index)
		}
		if (action == "modify" || action == "disable" || action == "remove") && edit.ControlRef == "" {
			return nil, rejectEQControl("control_ref_required", "edit %d action %s requires control_ref", index, action)
		}
		if action == "undo" && edit.OperationRef == "" {
			return nil, rejectEQControl("operation_ref_required", "edit %d undo requires operation_ref", index)
		}
		if action == "disable" || action == "remove" || action == "undo" {
			if shape != "" || edit.FrequencyHz != nil || edit.GainDB != nil || edit.Q != nil || edit.SlopeDBPerOct != nil {
				return nil, rejectEQControl("invalid_field_for_action", "edit %d action %s accepts only its reference", index, action)
			}
		}
		out = append(out, edit)
	}
	return out, nil
}

func parseOptionalEQEditNumber(row map[string]any, keys ...string) (*float64, error) {
	for _, key := range keys {
		if _, exists := row[key]; !exists {
			continue
		}
		value, _, err := parseOptionalFloat64Arg(row, key)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("%s must be finite", key)
		}
		return &value, nil
	}
	return nil, nil
}

func (s *Server) readLiveEQControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	client := s.eqKernelClient()
	if client == nil {
		return plugingrabber.ParameterDigest{}, nil, rejectEQControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID})
	if err != nil {
		return plugingrabber.ParameterDigest{}, nil, rejectEQControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return plugingrabber.ParameterDigest{}, nil, rejectEQControl("parameter_read_failed", "%s",
			firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest := buildPluginParameterDigest(reply)
	summary := plugingrabber.BuildEQBandSummary(digest)
	if summary == nil {
		return digest, nil, rejectEQControl("not_static_eq", "the selected plugin has no provable static EQ control topology")
	}
	return digest, summary, nil
}

func eqTopologyGenerationFromSummary(summary map[string]any) string {
	return firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
}

func planEQEdit(summary map[string]any, edit eqEditRequest, trackID, pluginID, generation string,
	reserved map[string]bool) (eqPlannedEdit, error) {
	switch edit.Action {
	case "upsert":
		return planEQUpsert(summary, edit, trackID, pluginID, generation, reserved)
	case "modify", "disable", "remove":
		return planEQReferencedEdit(summary, edit, trackID, pluginID, generation, reserved)
	default:
		return eqPlannedEdit{}, rejectEQControl("invalid_action", "unsupported forward action %s", edit.Action)
	}
}

func planEQUpsert(summary map[string]any, edit eqEditRequest, trackID, pluginID, generation string,
	reserved map[string]bool) (eqPlannedEdit, error) {
	if (edit.Shape == "bell" || edit.Shape == "low_shelf" || edit.Shape == "high_shelf") && edit.GainDB == nil {
		return eqPlannedEdit{}, rejectEQControl("required_field_missing", "gain_db is required for %s", edit.Shape)
	}
	if (edit.Shape == "low_cut" || edit.Shape == "high_cut") && edit.GainDB != nil {
		return eqPlannedEdit{}, rejectEQControl("invalid_field_for_shape", "gain_db is not applicable to %s", edit.Shape)
	}
	filtered := []map[string]any{}
	for _, section := range mapRowsValue(summary["sections"]) {
		if reserved[firstNonEmptyText(section, "section")] {
			continue
		}
		filtered = append(filtered, section)
	}
	local := make(map[string]any, len(summary))
	for key, value := range summary {
		local[key] = value
	}
	local["sections"] = filtered
	request := eqTypedPointRequest{FreqHz: *edit.FrequencyHz, GainDB: edit.GainDB, Q: edit.Q,
		Shape: edit.Shape, SlopeDBPerOct: edit.SlopeDBPerOct}
	writes, selection, err := eqTypedSectionWritePlan(local, request)
	if err != nil {
		return eqPlannedEdit{}, normalizeEQPlanningError(err, edit.Shape, "upsert")
	}
	sectionKey := firstNonEmptyText(selection, "section")
	section := findEQSummarySection(summary, sectionKey)
	if section == nil {
		return eqPlannedEdit{}, rejectEQControl("section_resolution_failed", "selected section %q disappeared", sectionKey)
	}
	ref, err := encodeEQControlRef(eqControlReference{Version: "eq-control-ref/v1", TrackID: trackID, PluginID: pluginID,
		TopologyGeneration: generation, Section: sectionKey, Shape: edit.Shape})
	if err != nil {
		return eqPlannedEdit{}, rejectEQControl("control_ref_failed", "%v", err)
	}
	quantized := eqWritesAreQuantized(writes)
	if value, ok := selection["frequency_quantized"].(bool); ok {
		quantized = quantized || value
	}
	return eqPlannedEdit{Request: edit, Section: section, SectionKey: sectionKey, Shape: edit.Shape,
		Writes: writes, Selection: selection, Quantized: quantized, ControlRef: ref, Requested: eqEditRequestedMap(edit)}, nil
}

func planEQReferencedEdit(summary map[string]any, edit eqEditRequest, trackID, pluginID, generation string,
	reserved map[string]bool) (eqPlannedEdit, error) {
	ref, err := decodeEQControlRef(edit.ControlRef)
	if err != nil {
		return eqPlannedEdit{}, rejectEQControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqPlannedEdit{}, rejectEQControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqPlannedEdit{}, rejectEQControl("stale_control_ref", "control_ref generation %s does not match current %s",
			ref.TopologyGeneration, generation)
	}
	if reserved[ref.Section] {
		return eqPlannedEdit{}, rejectEQControl("section_conflict", "section %s is targeted more than once in one atomic batch", ref.Section)
	}
	section := findEQSummarySection(summary, ref.Section)
	if section == nil {
		return eqPlannedEdit{}, rejectEQControl("stale_control_ref", "referenced section %s is absent", ref.Section)
	}
	shape := ref.Shape
	if edit.Shape != "" {
		shape = edit.Shape
	}
	if !eqSectionActionSupported(section, shape, edit.Action) {
		return eqPlannedEdit{}, sectionCapabilityFailure(section, shape, edit.Action)
	}
	var writes []eqWriteStep
	selection := map[string]any{"section": ref.Section, "shape": shape, "addressing": section["addressing"]}
	quantized := false
	if edit.Action == "modify" {
		writes, quantized, err = planEQModifyWrites(section, edit, shape)
	} else {
		writes, err = planEQDeactivateWrites(section, edit.Action == "remove")
	}
	if err != nil {
		return eqPlannedEdit{}, err
	}
	newRef, err := encodeEQControlRef(eqControlReference{Version: "eq-control-ref/v1", TrackID: trackID, PluginID: pluginID,
		TopologyGeneration: generation, Section: ref.Section, Shape: shape})
	if err != nil {
		return eqPlannedEdit{}, rejectEQControl("control_ref_failed", "%v", err)
	}
	return eqPlannedEdit{Request: edit, Section: section, SectionKey: ref.Section, Shape: shape,
		Writes: writes, Selection: selection, Quantized: quantized || eqWritesAreQuantized(writes),
		ControlRef: newRef, Requested: eqEditRequestedMap(edit)}, nil
}

func planEQModifyWrites(section map[string]any, edit eqEditRequest, shape string) ([]eqWriteStep, bool, error) {
	if edit.FrequencyHz == nil && edit.GainDB == nil && edit.Q == nil && edit.SlopeDBPerOct == nil && edit.Shape == "" {
		return nil, false, rejectEQControl("no_changes_requested", "modify requires at least one acoustic field")
	}
	if (shape == "low_cut" || shape == "high_cut") && edit.GainDB != nil {
		return nil, false, rejectEQControl("invalid_field_for_shape", "gain_db is not applicable to %s", shape)
	}
	writes := []eqWriteStep{}
	if edit.Shape != "" {
		shapeWrites, err := eqFilterKindWrites(section, shape)
		if err != nil {
			return nil, false, normalizeEQPlanningError(err, shape, "modify")
		}
		writes = append(writes, shapeWrites...)
	}
	quantized := false
	if edit.FrequencyHz != nil {
		if len(mapRowsValue(section["frequency_bindings"])) == 0 {
			fixed, ok := eqBandFloat(section, "fixed_freq_hz")
			if !ok {
				return nil, false, rejectEQControl("frequency_binding_unavailable", "referenced section has no frequency control")
			}
			quantized = math.Abs(fixed-*edit.FrequencyHz) > 1e-9
		} else {
			rows, frequencyQuantized, err := eqFrequencyWritesForSection(section, *edit.FrequencyHz)
			if err != nil {
				return nil, false, rejectEQControl("frequency_binding_unavailable", "%v", err)
			}
			writes = append(writes, rows...)
			quantized = quantized || frequencyQuantized
		}
	}
	for _, field := range []struct {
		value           *float64
		role, writeRole string
		min, max        float64
		scale           string
		code            string
	}{
		{edit.GainDB, "gain", "gain", -24, 24, "linear", "gain_binding_unavailable"},
		{edit.Q, "q", "q", 0.1, 6, "log", "explicit_q_unavailable"},
		{edit.SlopeDBPerOct, "slope", "slope", 6, 96, "linear", "explicit_slope_unavailable"},
	} {
		if field.value == nil {
			continue
		}
		rows, err := eqWritesForRole(section, field.role, field.writeRole, *field.value, field.min, field.max, field.scale)
		if err != nil {
			return nil, false, rejectEQControl(field.code, "%v", err)
		}
		writes = append(writes, rows...)
	}
	if len(writes) == 0 && !quantized {
		return nil, false, rejectEQControl("no_writable_changes", "modify produced no parameter writes")
	}
	return writes, quantized || eqWritesAreQuantized(writes), nil
}

func planEQDeactivateWrites(section map[string]any, remove bool) ([]eqWriteStep, error) {
	if remove && !eqBoolValue(section["deallocatable"]) {
		return nil, rejectEQControl("section_not_deallocatable", "section %s is resident/anchored and can only be disabled or undone",
			firstNonEmptyText(section, "section"))
	}
	bindingGroups := [][]map[string]any{mapRowsValue(section["activation_bindings"])}
	if firstNonEmptyText(section, "activation_strategy") == "implicit_in_domain" {
		bindingGroups = append(bindingGroups, mapRowsValue(section["frequency_bindings"]))
	} else if firstNonEmptyText(section, "activation_strategy") == "property_sentinel" {
		bindingGroups = append(bindingGroups, mapRowsValue(section["slope_bindings"]))
	}
	writes := []eqWriteStep{}
	for _, bindings := range bindingGroups {
		for _, binding := range bindings {
			if remove && firstNonEmptyText(binding, "activation_kind") != "used" {
				continue
			}
			normalized, label, ok := eqInactiveBindingTarget(binding, remove)
			if !ok {
				continue
			}
			role := "disabled"
			if remove {
				role = "removed"
			}
			writes = append(writes, eqWriteStep{ParamID: firstNonEmptyText(binding, "param_id"), Role: role,
				Channel: firstNonEmptyText(binding, "channel"), NormalizedValue: normalized, Quantized: true,
				RequestedLabel: role, ExpectedLabel: label})
		}
		if len(writes) > 0 {
			break
		}
	}
	if len(writes) == 0 {
		code := "activation_binding_unavailable"
		if remove {
			code = "section_not_deallocatable"
		}
		return nil, rejectEQControl(code, "section %s has no provable inactive value", firstNonEmptyText(section, "section"))
	}
	return writes, nil
}

func eqInactiveBindingTarget(binding map[string]any, remove bool) (float64, string, bool) {
	preferred := []string{"off", "out", "disabled", "inactive", "bypass", "bypassed", "unused"}
	if remove {
		preferred = []string{"unused", "inactive", "off"}
	}
	for _, want := range preferred {
		for _, row := range mapRowsValue(binding["reachable_values"]) {
			label := strings.ToLower(strings.TrimSpace(firstNonEmptyText(row, "label")))
			if label != want && !strings.HasPrefix(label, want+" ") {
				continue
			}
			value, ok := eqBandFloatAny(row, "normalized")
			return value, firstNonEmptyText(row, "label"), ok
		}
	}
	return 0, "", false
}

func sectionCapabilityFailure(section map[string]any, shape, action string) error {
	for _, capability := range mapRowsValue(section["shape_capabilities"]) {
		if !strings.EqualFold(firstNonEmptyText(capability, "shape"), shape) {
			continue
		}
		codes := eqStringSlice(mapValue(capability["rejection_codes"])[action])
		if len(codes) > 0 {
			return rejectEQControl(codes[0], "%s %s rejected by section %s (%s)", action, shape,
				firstNonEmptyText(section, "section"), strings.Join(codes, ","))
		}
	}
	return rejectEQControl("shape_not_provably_reachable", "%s is not controllable by section %s", shape,
		firstNonEmptyText(section, "section"))
}

func normalizeEQPlanningError(err error, shape, action string) error {
	text := err.Error()
	code := "planning_failed"
	switch {
	case strings.Contains(text, "explicit_field_unavailable") || strings.Contains(text, "requested Q"):
		code = "explicit_field_unavailable"
	case strings.Contains(text, "invalid_field_for_shape"):
		code = "invalid_field_for_shape"
	case strings.Contains(text, "not provably reachable"):
		code = "shape_not_provably_reachable"
	case strings.Contains(text, "no complete") || strings.Contains(text, "no typed sections"):
		code = "no_available_section"
	}
	return rejectEQControl(code, "%s %s: %v", action, shape, err)
}

func combineAtomicEQWrites(plans []eqPlannedEdit) ([]eqWriteStep, error) {
	byID := map[string]eqWriteStep{}
	order := []string{}
	for _, plan := range plans {
		for _, write := range plan.Writes {
			if previous, exists := byID[write.ParamID]; exists {
				if math.Abs(previous.NormalizedValue-write.NormalizedValue) > 1e-9 {
					return nil, rejectEQControl("parameter_conflict", "atomic edits request different values for parameter %s", write.ParamID)
				}
				continue
			}
			byID[write.ParamID] = write
			order = append(order, write.ParamID)
		}
	}
	out := make([]eqWriteStep, 0, len(order))
	activation := make([]eqWriteStep, 0)
	for _, paramID := range order {
		write := byID[paramID]
		if write.ActivationLast || write.Role == "used" || write.Role == "disabled" || write.Role == "removed" {
			activation = append(activation, write)
		} else {
			out = append(out, write)
		}
	}
	out = append(out, activation...)
	if len(out) == 0 {
		return nil, rejectEQControl("no_writes_planned", "atomic edit batch produced no writes")
	}
	return out, nil
}

func eqWritesAreQuantized(writes []eqWriteStep) bool {
	for _, write := range writes {
		if write.Quantized && write.RequestedPhysical != nil {
			return true
		}
	}
	return false
}

func eqEditRequestedMap(edit eqEditRequest) map[string]any {
	out := map[string]any{"action": edit.Action}
	if edit.Shape != "" {
		out["shape"] = edit.Shape
	}
	if edit.FrequencyHz != nil {
		out["frequency_hz"] = *edit.FrequencyHz
	}
	if edit.GainDB != nil {
		out["gain_db"] = *edit.GainDB
	}
	if edit.Q != nil {
		out["q"] = *edit.Q
	}
	if edit.SlopeDBPerOct != nil {
		out["slope_db_per_oct"] = *edit.SlopeDBPerOct
	}
	return out
}

func findEQSummarySection(summary map[string]any, key string) map[string]any {
	for _, section := range mapRowsValue(summary["sections"]) {
		if firstNonEmptyText(section, "section") == key {
			return section
		}
	}
	return nil
}

func eqActualRowsForWrites(actual []map[string]any, writes []eqWriteStep) []map[string]any {
	wanted := map[string]bool{}
	for _, write := range writes {
		wanted[write.ParamID] = true
	}
	out := []map[string]any{}
	for _, row := range actual {
		if wanted[firstNonEmptyText(row, "param_id")] {
			out = append(out, row)
		}
	}
	return out
}

func encodeEQControlRef(ref eqControlReference) (string, error) {
	payload, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "eqc1_" + base64.RawURLEncoding.EncodeToString(payload) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeEQControlRef(value string) (eqControlReference, error) {
	var ref eqControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "eqc1_") {
		return ref, fmt.Errorf("unknown control_ref version")
	}
	parts := strings.Split(strings.TrimPrefix(value, "eqc1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("malformed control_ref")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("malformed control_ref payload")
	}
	sum := sha256.Sum256(payload)
	if parts[1] != hex.EncodeToString(sum[:8]) {
		return ref, fmt.Errorf("control_ref checksum mismatch")
	}
	if err := json.Unmarshal(payload, &ref); err != nil {
		return ref, fmt.Errorf("malformed control_ref object")
	}
	if ref.Version != "eq-control-ref/v1" || ref.TrackID == "" || ref.PluginID == "" || ref.Section == "" || ref.Shape == "" {
		return ref, fmt.Errorf("incomplete control_ref")
	}
	return ref, nil
}

func (s *Server) storeEQOperation(record eqOperationRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eqOperations == nil {
		s.eqOperations = map[string]eqOperationRecord{}
	}
	// Bound the ephemeral transaction journal. Refs intentionally expire on
	// process restart because fresh plug-in state cannot then be assumed.
	if len(s.eqOperations) >= eqOperationJournalCapacity {
		oldestKey := ""
		var oldestTime time.Time
		for key, existing := range s.eqOperations {
			if oldestKey == "" || existing.CreatedAt.Before(oldestTime) ||
				(existing.CreatedAt.Equal(oldestTime) && key < oldestKey) {
				oldestKey, oldestTime = key, existing.CreatedAt
			}
		}
		delete(s.eqOperations, oldestKey)
	}
	s.eqOperations[record.OperationRef] = record
}

func (s *Server) undoEQOperation(ctx context.Context, trackID, pluginID, operationRef string) (map[string]any, error) {
	s.mu.Lock()
	record, ok := s.eqOperations[operationRef]
	s.mu.Unlock()
	if !ok {
		return nil, rejectEQControl("unknown_operation_ref", "operation_ref is absent or expired")
	}
	if record.Undone {
		return nil, rejectEQControl("operation_already_undone", "operation %s was already undone", operationRef)
	}
	if record.TrackID != trackID || record.PluginID != pluginID {
		return nil, rejectEQControl("operation_ref_target_mismatch", "operation_ref belongs to another plugin instance")
	}
	digest, summary, err := s.readLiveEQControlSurface(ctx, trackID, pluginID)
	if err != nil {
		return nil, err
	}
	if current := eqTopologyGenerationFromSummary(summary); current != record.TopologyGeneration {
		return nil, rejectEQControl("stale_operation_ref", "operation topology generation no longer matches")
	}
	currentByID := map[string]float64{}
	for _, param := range digest.Parameters {
		if value, ok := numericAny(param.NormalizedValue); ok {
			currentByID[strings.TrimSpace(param.ID)] = value
		}
	}
	for paramID, expected := range record.Postimage {
		if actual, ok := currentByID[paramID]; !ok || math.Abs(actual-expected) > 1e-4 {
			return nil, rejectEQControl("operation_state_conflict", "parameter %s changed after operation; refusing destructive out-of-order undo", paramID)
		}
	}
	if err := s.rollbackEQTransaction(ctx, trackID, pluginID, record.Preimage); err != nil {
		return nil, rejectEQControl("undo_failed", "%v", err)
	}
	s.mu.Lock()
	record.Undone = true
	s.eqOperations[operationRef] = record
	s.mu.Unlock()
	return map[string]any{"status": "exact", "atomic": true, "action": "undo", "operation_ref": operationRef,
		"restored_parameter_count": len(record.Preimage), "rollback": map[string]any{"verified": true}}, nil
}

func eqBoolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}
