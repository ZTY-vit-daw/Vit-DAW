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
	pluginGrabberApplyLimiterCommand = "plugin_grabber_apply_limiter_controls"
	pluginGrabberApplyLimiterTool    = "plugin_grabber.apply_limiter_controls"
)

type limiterControlFailure struct{ Code, Message string }

func (f *limiterControlFailure) Error() string { return f.Code + ": " + f.Message }
func rejectLimiterControl(code, format string, args ...any) error {
	return &limiterControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}
func limiterControlFailureCode(err error) string {
	if f, ok := err.(*limiterControlFailure); ok {
		return f.Code
	}
	return "limiter_control_failed"
}

type limiterControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyLimiterInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyLimiterTool && name != pluginGrabberApplyLimiterCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyLimiterCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyLimiterCommand}
	for k, v := range req.Args {
		cmd[k] = v
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyLimiterWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyLimiterTool, CommandName: pluginGrabberApplyLimiterCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberLimiterControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = limiterControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberLimiterControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectLimiterControl("atomic_required", "limiter controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectLimiterControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveLimiterControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectLimiterControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectLimiterControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberLimiterControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseLimiterControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planLimiterControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": limiterControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectLimiterControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": limiterControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectLimiterControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectLimiterControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeLimiterRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectLimiterControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectLimiterControl("physical_probe_failed", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectLimiterControl(code, "%v", err)
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
	return map[string]any{"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID, "topology_generation": generation, "controls": results, "writes": executed, "restore_ref": restoreRef, "rollback": map[string]any{"on_failure": "full_preimage", "verified": true}}, nil
}

func parseLimiterControlRequests(args map[string]any) ([]limiterControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectLimiterControl("controls_required", "controls must contain at least one explicit limiter control")
	}
	out := make([]limiterControlRequest, 0, len(rows))
	for i, row := range rows {
		req := limiterControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectLimiterControl("control_ref_required", "control %d omitted control_ref from inspect_limiter", i)
		}
		for _, f := range []struct{ Key, Unit string }{{"value_db", "dB"}, {"value_ms", "ms"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[f.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectLimiterControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectLimiterControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value = &v
				req.Unit = f.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectLimiterControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectLimiterControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveLimiterControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectLimiterControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectLimiterControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectLimiterControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildLimiterSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_limiter"
		}
		return digest, nil, rejectLimiterControl(boundary, "no provable supported limiter stage/control-path topology")
	}
	if err := attachLimiterControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectLimiterControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planLimiterControl(summary map[string]any, req limiterControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeLimiterControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectLimiterControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectLimiterControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectLimiterControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findLimiterSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectLimiterControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectLimiterControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateLimiterRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		cn, nok := firstNumericAny(binding, "current_normalized")
		cp, pok := firstNumericAny(binding, "current_physical")
		if !nok || !pok {
			return eqWriteStep{}, nil, rejectLimiterControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
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
		return eqWriteStep{}, nil, translateLimiterError(err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing = normalized, &actual, quantized, decreasing
	write.DiscretePhysical = quantized
	return write, plan, nil
}

func translateLimiterError(err error) error {
	if err == nil {
		return nil
	}
	if f, ok := err.(*compressorControlFailure); ok {
		return rejectLimiterControl(f.Code, "%s", f.Message)
	}
	return rejectLimiterControl("physical_target_failed", "%v", err)
}
func validateLimiterRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	if !limiterUnitCompatible(role, unit, domain) {
		return rejectLimiterControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}
func limiterUnitCompatible(role, unit, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	switch unit {
	case "dB":
		return compressorRoleIn(role, "threshold", "input_drive", "ceiling", "output_gain") == true && (domain == "db" || domain == "")
	case "ms":
		return compressorRoleIn(role, "release", "lookahead", "attack", "hold") && (domain == "ms" || domain == "s" || domain == "")
	case "%":
		return compressorRoleIn(role, "channel_link", "mix") && (domain == "%" || domain == "")
	case "display":
		return domain != "enum" && domain != "toggle"
	}
	return false
}

func findLimiterSummaryBinding(summary map[string]any, ref limiterControlReference) map[string]any {
	if ref.Section == "shared" {
		for _, row := range mapRowsValue(summary["shared_controls"]) {
			if firstNonEmptyText(row, "param_id") == ref.ParamID && firstNonEmptyText(row, "role") == ref.Role {
				return row
			}
		}
		return nil
	}
	for _, stage := range mapRowsValue(summary["limiter_stages"]) {
		if firstNonEmptyText(stage, "stage_key") != ref.StageKey {
			continue
		}
		for _, row := range mapRowsValue(stage[ref.Section]) {
			if firstNonEmptyText(row, "param_id") == ref.ParamID && firstNonEmptyText(row, "role") == ref.Role {
				return row
			}
		}
	}
	return nil
}

func (s *Server) restorePluginGrabberLimiterControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeLimiterRestoreRef(encoded)
	if err != nil {
		return nil, rejectLimiterControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectLimiterControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectLimiterControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, v := range ref.Values {
		if v.Normalized < 0 || v.Normalized > 1 {
			return nil, rejectLimiterControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", v.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: v.ParamID, NormalizedValue: v.Normalized, Role: v.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectLimiterControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectLimiterControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual, "rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}
