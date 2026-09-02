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
	pluginGrabberApplyGateExpanderCommand = "plugin_grabber_apply_gate_expander_controls"
	pluginGrabberApplyGateExpanderTool    = "plugin_grabber.apply_gate_expander_controls"
)

type gateExpanderControlFailure struct{ Code, Message string }

func (f *gateExpanderControlFailure) Error() string { return f.Code + ": " + f.Message }

func rejectGateExpanderControl(code, format string, args ...any) error {
	return &gateExpanderControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}

func gateExpanderControlFailureCode(err error) string {
	if f, ok := err.(*gateExpanderControlFailure); ok {
		return f.Code
	}
	return "gate_expander_control_failed"
}

type gateExpanderControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyGateExpanderInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyGateExpanderTool && name != pluginGrabberApplyGateExpanderCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyGateExpanderCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyGateExpanderCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyGateExpanderWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyGateExpanderTool,
		CommandName: pluginGrabberApplyGateExpanderCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberGateExpanderControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = gateExpanderControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberGateExpanderControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectGateExpanderControl("atomic_required", "gate/expander controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectGateExpanderControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveGateExpanderControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectGateExpanderControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectGateExpanderControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberGateExpanderControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseGateExpanderControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planGateExpanderControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i,
				"rejection_code": gateExpanderControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectGateExpanderControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i,
				"rejection_code": gateExpanderControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectGateExpanderControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectGateExpanderControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeGateExpanderRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectGateExpanderControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectGateExpanderControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectGateExpanderControl(code, "%v", err)
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
	result := map[string]any{"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID,
		"topology_generation": generation, "controls": results, "writes": executed, "restore_ref": restoreRef,
		"rollback": map[string]any{"on_failure": "full_preimage", "verified": true}}
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

func parseGateExpanderControlRequests(args map[string]any) ([]gateExpanderControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectGateExpanderControl("controls_required", "controls must contain at least one explicit gate/expander control")
	}
	out := make([]gateExpanderControlRequest, 0, len(rows))
	for i, row := range rows {
		req := gateExpanderControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectGateExpanderControl("control_ref_required", "control %d omitted control_ref from inspect_gate_expander", i)
		}
		for _, field := range []struct{ Key, Unit string }{{"value_db", "dB"}, {"ratio", "ratio"}, {"value_ms", "ms"}, {"frequency_hz", "Hz"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[field.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectGateExpanderControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectGateExpanderControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value, req.Unit = &v, field.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectGateExpanderControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectGateExpanderControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveGateExpanderControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectGateExpanderControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectGateExpanderControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectGateExpanderControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildGateExpanderSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_gate_expander"
		}
		return digest, nil, rejectGateExpanderControl(boundary, "no provable supported hard-gate or downward-expander stage/control-path topology")
	}
	if err := attachGateExpanderControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectGateExpanderControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planGateExpanderControl(summary map[string]any, req gateExpanderControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeGateExpanderControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectGateExpanderControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectGateExpanderControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectGateExpanderControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findGateExpanderSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectGateExpanderControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		if ref.Role == "direction_mode" && !gateExpanderDirectionLabelAllowed(req.EnumLabel) {
			return eqWriteStep{}, nil, rejectGateExpanderControl("direction_value_unavailable", "direction label %q is not Gate or downward Expander evidence", req.EnumLabel)
		}
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectGateExpanderControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateGateExpanderRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		cn, nok := firstNumericAny(binding, "current_normalized")
		cp, pok := firstNumericAny(binding, "current_physical")
		if !nok || !pok {
			return eqWriteStep{}, nil, rejectGateExpanderControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
		}
		write.NormalizedValue, write.RequestedPhysical = cn, &target
		if nearlyEqualCompressor(cp, target) {
			return write, plan, nil
		}
		write.TransactionalProbe = true
		return write, plan, nil
	}
	normalized, actual, quantized, decreasing, err := compressorPhysicalTarget(binding, target, req.Unit)
	if err != nil {
		return eqWriteStep{}, nil, translateGateExpanderError(err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing = normalized, &actual, quantized, decreasing
	write.DiscretePhysical = quantized
	return write, plan, nil
}

func translateGateExpanderError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*gateExpanderControlFailure); ok {
		return err
	}
	if f, ok := err.(*compressorControlFailure); ok {
		return rejectGateExpanderControl(f.Code, "%s", f.Message)
	}
	return rejectGateExpanderControl("physical_target_failed", "%v", err)
}

func validateGateExpanderRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	if !gateExpanderUnitCompatible(role, unit, domain) {
		return rejectGateExpanderControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func gateExpanderUnitCompatible(role, unit, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	switch unit {
	case "dB":
		return compressorRoleIn(role, "threshold", "hysteresis", "range", "detector_gain", "output_gain") && (domain == "db" || domain == "")
	case "ratio":
		return role == "expansion_ratio" && (domain == "ratio" || domain == "" || domain == "enum")
	case "ms":
		return compressorRoleIn(role, "attack", "hold", "release", "lookahead", "cycle_delay") && (domain == "ms" || domain == "s" || domain == "")
	case "Hz":
		return compressorRoleIn(role, "sidechain_highpass", "sidechain_lowpass") && (domain == "hz" || domain == "")
	case "%":
		return role == "mix" && (domain == "%" || domain == "")
	case "display":
		return domain != "enum" && domain != "toggle"
	}
	return false
}

func gateExpanderDirectionLabelAllowed(label string) bool {
	text := strings.ToLower(strings.TrimSpace(label))
	if strings.Contains(text, "upward") || strings.Contains(text, "compressor") || strings.Contains(text, "compression") {
		return false
	}
	return strings.Contains(text, "gate") || strings.Contains(text, "expander") || strings.Contains(text, "expansion") || strings.Contains(text, "downward")
}

func findGateExpanderSummaryBinding(summary map[string]any, ref gateExpanderControlReference) map[string]any {
	stage := mapValue(summary["gate_expander_stage"])
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

func (s *Server) restorePluginGrabberGateExpanderControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeGateExpanderRestoreRef(encoded)
	if err != nil {
		return nil, rejectGateExpanderControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectGateExpanderControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectGateExpanderControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, value := range ref.Values {
		if value.Normalized < 0 || value.Normalized > 1 || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return nil, rejectGateExpanderControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", value.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: value.ParamID, NormalizedValue: value.Normalized, Role: value.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectGateExpanderControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectGateExpanderControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID,
		"topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual,
		"rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}
