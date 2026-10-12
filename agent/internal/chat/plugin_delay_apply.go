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

// M2-TIMEFX-1 delay apply face: atomically writes explicit delay controls
// addressed by generation-scoped control_ref through the same preimage /
// transaction / readback machine every family apply uses, or restores a
// successful apply with its exact normalized restore_ref.

const (
	pluginGrabberApplyDelayCommand = "plugin_grabber_apply_delay_controls"
	pluginGrabberApplyDelayTool    = "plugin_grabber.apply_delay_controls"
)

type delayControlFailure struct{ Code, Message string }

func (f *delayControlFailure) Error() string { return f.Code + ": " + f.Message }

func rejectDelayControl(code, format string, args ...any) error {
	return &delayControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}

func delayControlFailureCode(err error) string {
	if f, ok := err.(*delayControlFailure); ok {
		return f.Code
	}
	return "delay_control_failed"
}

type delayControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyDelayInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyDelayTool && name != pluginGrabberApplyDelayCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyDelayCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyDelayCommand}
	for k, v := range req.Args {
		cmd[k] = v
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyDelayWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyDelayTool, CommandName: pluginGrabberApplyDelayCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberDelayControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = delayControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberDelayControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectDelayControl("atomic_required", "delay controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectDelayControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveDelayControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectDelayControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectDelayControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberDelayControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseDelayControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planDelayControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": delayControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectDelayControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": delayControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectDelayControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectDelayControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeDelayRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectDelayControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectDelayControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectDelayControl(code, "%v", err)
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
		result["idempotency_key"] = accounting.RequestID
		if accounting.TransactionID != "" {
			result["transaction_id"] = accounting.TransactionID
		}
	}
	return result, nil
}

func parseDelayControlRequests(args map[string]any) ([]delayControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectDelayControl("controls_required", "controls must contain at least one explicit delay control")
	}
	out := make([]delayControlRequest, 0, len(rows))
	for i, row := range rows {
		req := delayControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectDelayControl("control_ref_required", "control %d omitted control_ref from inspect_delay", i)
		}
		for _, f := range []struct{ Key, Unit string }{{"value_ms", "ms"}, {"value_s", "s"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[f.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectDelayControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectDelayControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value = &v
				req.Unit = f.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectDelayControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectDelayControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveDelayControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectDelayControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectDelayControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectDelayControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildDelaySummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_delay"
		}
		return digest, nil, rejectDelayControl(boundary, "no provable supported delay stage/control-path topology")
	}
	if err := attachDelayControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectDelayControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planDelayControl(summary map[string]any, req delayControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeDelayControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectDelayControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectDelayControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectDelayControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findDelaySummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectDelayControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectDelayControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateDelayRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		cn, nok := firstNumericAny(binding, "current_normalized")
		cp, pok := firstNumericAny(binding, "current_physical")
		if !nok || !pok {
			return eqWriteStep{}, nil, rejectDelayControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
		}
		write.NormalizedValue = cn
		write.RequestedPhysical = &target
		if nearlyEqualCompressor(cp, target) {
			return write, plan, nil
		}
		write.TransactionalProbe = true
		return write, plan, nil
	}
	normalized, actual, quantized, decreasing, err := compressorPhysicalTarget(binding, target, req.Unit)
	if err != nil {
		return eqWriteStep{}, nil, translateDelayError(err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing = normalized, &actual, quantized, decreasing
	write.DiscretePhysical = quantized
	return write, plan, nil
}

func translateDelayError(err error) error {
	if err == nil {
		return nil
	}
	if f, ok := err.(*compressorControlFailure); ok {
		return rejectDelayControl(f.Code, "%s", f.Message)
	}
	return rejectDelayControl("physical_target_failed", "%v", err)
}

func validateDelayRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	if !delayUnitCompatible(role, unit, domain) {
		return rejectDelayControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func delayUnitCompatible(role, unit, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	switch unit {
	case "ms":
		return delayRoleIn(role, "time", "sync") && (domain == "ms" || domain == "s" || domain == "")
	case "s":
		return delayRoleIn(role, "time") && (domain == "s" || domain == "ms" || domain == "")
	case "%":
		return delayRoleIn(role, "feedback", "mix", "wet", "dry") && (domain == "%" || domain == "")
	case "display":
		return domain != "enum" && domain != "toggle"
	}
	return false
}

func findDelaySummaryBinding(summary map[string]any, ref delayControlReference) map[string]any {
	stage := mapValue(summary["delay_stage"])
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

func (s *Server) restorePluginGrabberDelayControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeDelayRestoreRef(encoded)
	if err != nil {
		return nil, rejectDelayControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectDelayControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectDelayControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, v := range ref.Values {
		if v.Normalized < 0 || v.Normalized > 1 {
			return nil, rejectDelayControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", v.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: v.ParamID, NormalizedValue: v.Normalized, Role: v.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectDelayControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectDelayControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual, "rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}

func delayRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}
