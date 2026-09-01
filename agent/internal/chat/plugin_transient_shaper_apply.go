package chat

import (
	"context"
	"fmt"
	"math"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberApplyTransientShaperCommand = "plugin_grabber_apply_transient_shaper_controls"
	pluginGrabberApplyTransientShaperTool    = "plugin_grabber.apply_transient_shaper_controls"
)

type transientShaperControlFailure struct{ Code, Message string }

func (f *transientShaperControlFailure) Error() string { return f.Code + ": " + f.Message }
func rejectTransientShaperControl(code, format string, args ...any) error {
	return &transientShaperControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}
func transientShaperControlFailureCode(err error) string {
	if f, ok := err.(*transientShaperControlFailure); ok {
		return f.Code
	}
	return "transient_shaper_control_failed"
}

type transientShaperControlRequest struct {
	ControlRef      string
	Value           *float64
	Unit, EnumLabel string
	Requested       map[string]any
}

func pluginGrabberApplyTransientShaperInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyTransientShaperTool && name != pluginGrabberApplyTransientShaperCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyTransientShaperCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyTransientShaperCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyTransientShaperWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyTransientShaperTool, CommandName: pluginGrabberApplyTransientShaperCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberTransientShaperControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = transientShaperControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberTransientShaperControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectTransientShaperControl("atomic_required", "transient-shaper controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectTransientShaperControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveTransientShaperControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectTransientShaperControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectTransientShaperControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberTransientShaperControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseTransientShaperControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes, results, seen := make([]eqWriteStep, 0, len(requests)), make([]map[string]any, 0, len(requests)), map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planTransientShaperControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": transientShaperControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectTransientShaperControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": transientShaperControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectTransientShaperControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectTransientShaperControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeTransientShaperRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectTransientShaperControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectTransientShaperControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectTransientShaperControl(code, "%v", err)
	}
	overall := "exact"
	for i := range results {
		results[i]["actual_readback"] = eqActualRowsForWrites(actual, []eqWriteStep{writes[i]})
		if writes[i].Quantized {
			results[i]["status"] = "quantized"
			overall = "quantized"
		} else {
			results[i]["status"] = "exact"
		}
	}
	result := map[string]any{"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID, "topology_generation": generation, "controls": results, "writes": executed, "restore_ref": restoreRef, "rollback": map[string]any{"on_failure": "full_preimage", "verified": true}}
	if accounting != nil {
		// Kernel-real transaction identity for the semantic settlement
		// bracket; callers that pin no request id (the harness tool path)
		// keep the historical result shape without these fields.
		result["idempotency_key"] = accounting.RequestID
		if accounting.TransactionID != "" {
			result["transaction_id"] = accounting.TransactionID
		}
	}
	return result, nil
}

func parseTransientShaperControlRequests(args map[string]any) ([]transientShaperControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectTransientShaperControl("controls_required", "controls must contain at least one explicit transient-shaper control")
	}
	out := make([]transientShaperControlRequest, 0, len(rows))
	for i, row := range rows {
		req := transientShaperControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectTransientShaperControl("control_ref_required", "control %d omitted control_ref from inspect_transient_shaper", i)
		}
		for _, field := range []struct{ Key, Unit string }{{"value_db", "dB"}, {"value_ms", "ms"}, {"frequency_hz", "Hz"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[field.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectTransientShaperControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectTransientShaperControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value, req.Unit = &v, field.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectTransientShaperControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectTransientShaperControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveTransientShaperControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectTransientShaperControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectTransientShaperControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectTransientShaperControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildTransientShaperSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_transient_shaper"
		}
		return digest, nil, rejectTransientShaperControl(boundary, "no provable supported transient-shaper envelope topology")
	}
	if err := attachTransientShaperControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectTransientShaperControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planTransientShaperControl(summary map[string]any, req transientShaperControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeTransientShaperControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectTransientShaperControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectTransientShaperControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectTransientShaperControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findTransientShaperSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectTransientShaperControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		if ref.Role == "processing_mode" && !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(firstNonEmptyText(binding, "current_text"))) {
			return eqWriteStep{}, nil, rejectTransientShaperControl("mode_change_requires_reinspect", "processing mode changes require a topology re-inspection and are rejected by this v1 controller")
		}
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectTransientShaperControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateTransientShaperRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	normalized, actual, quantized, decreasing, err := compressorPhysicalTarget(binding, *req.Value, req.Unit)
	if err != nil {
		return eqWriteStep{}, nil, translateTransientShaperError(err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing, write.DiscretePhysical = normalized, &actual, quantized, decreasing, quantized
	return write, plan, nil
}

func translateTransientShaperError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*transientShaperControlFailure); ok {
		return err
	}
	if f, ok := err.(*compressorControlFailure); ok {
		return rejectTransientShaperControl(f.Code, "%s", f.Message)
	}
	return rejectTransientShaperControl("physical_target_failed", "%v", err)
}

func validateTransientShaperRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	ok := false
	switch unit {
	case "dB":
		ok = transientShaperRoleIn(role, "attack_amount", "sustain_amount", "transient_range", "attack_sensitivity", "sustain_sensitivity", "detector_sensitivity", "output_gain") && (domain == "db" || domain == "")
	case "%":
		ok = transientShaperRoleIn(role, "attack_amount", "sustain_amount", "transient_range", "attack_sensitivity", "sustain_sensitivity", "detector_sensitivity", "mix") && (domain == "%" || domain == "")
	case "ms":
		ok = transientShaperRoleIn(role, "attack_duration", "sustain_duration", "duration", "release") && (domain == "ms" || domain == "s" || domain == "")
	case "Hz":
		ok = role == "focus_frequency" && (domain == "hz" || domain == "")
	case "display":
		ok = domain != "enum" && domain != "toggle"
	}
	if !ok {
		return rejectTransientShaperControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func transientShaperRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}

func findTransientShaperSummaryBinding(summary map[string]any, ref transientShaperControlReference) map[string]any {
	stage := mapValue(summary["transient_shaper_stage"])
	if firstNonEmptyText(stage, "stage_key") != ref.StageKey {
		return nil
	}
	for _, row := range mapRowsValue(stage[ref.Section]) {
		if firstNonEmptyText(row, "param_id") == ref.ParamID && firstNonEmptyText(row, "role") == ref.Role {
			return row
		}
	}
	return nil
}

func (s *Server) restorePluginGrabberTransientShaperControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeTransientShaperRestoreRef(encoded)
	if err != nil {
		return nil, rejectTransientShaperControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectTransientShaperControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectTransientShaperControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, value := range ref.Values {
		if value.Normalized < 0 || value.Normalized > 1 || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return nil, rejectTransientShaperControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", value.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: value.ParamID, NormalizedValue: value.Normalized, Role: value.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectTransientShaperControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectTransientShaperControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual, "rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}
