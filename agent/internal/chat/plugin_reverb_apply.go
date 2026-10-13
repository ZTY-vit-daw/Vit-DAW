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

// M2-TIMEFX-1 reverb apply face: atomically writes explicit reverb controls
// addressed by generation-scoped control_ref through the same preimage /
// transaction / readback machine every family apply uses, or restores a
// successful apply with its exact normalized restore_ref.

const (
	pluginGrabberApplyReverbCommand = "plugin_grabber_apply_reverb_controls"
	pluginGrabberApplyReverbTool    = "plugin_grabber.apply_reverb_controls"
)

type reverbControlFailure struct{ Code, Message string }

func (f *reverbControlFailure) Error() string { return f.Code + ": " + f.Message }

func rejectReverbControl(code, format string, args ...any) error {
	return &reverbControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}

func reverbControlFailureCode(err error) string {
	if f, ok := err.(*reverbControlFailure); ok {
		return f.Code
	}
	return "reverb_control_failed"
}

type reverbControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyReverbInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyReverbTool && name != pluginGrabberApplyReverbCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyReverbCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyReverbCommand}
	for k, v := range req.Args {
		cmd[k] = v
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyReverbWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyReverbTool, CommandName: pluginGrabberApplyReverbCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberReverbControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = reverbControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberReverbControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectReverbControl("atomic_required", "reverb controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectReverbControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveReverbControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectReverbControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectReverbControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		restored, restoreErr := s.restorePluginGrabberReverbControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
		if restoreErr == nil {
			s.attachTimefxFXMProbe(ctx, restored, target.TrackID, target.PluginID, "reverb",
				firstNonEmptyText(summary, "classification"), generation)
		}
		return restored, restoreErr
	}
	requests, err := parseReverbControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planReverbControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": reverbControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectReverbControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": reverbControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectReverbControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectReverbControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeReverbRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectReverbControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectReverbControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectReverbControl(code, "%v", err)
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
	s.attachTimefxFXMProbe(ctx, result, target.TrackID, target.PluginID, "reverb",
		firstNonEmptyText(summary, "classification"), generation)
	return result, nil
}

func parseReverbControlRequests(args map[string]any) ([]reverbControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectReverbControl("controls_required", "controls must contain at least one explicit reverb control")
	}
	out := make([]reverbControlRequest, 0, len(rows))
	for i, row := range rows {
		req := reverbControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectReverbControl("control_ref_required", "control %d omitted control_ref from inspect_reverb", i)
		}
		for _, f := range []struct{ Key, Unit string }{{"value_s", "s"}, {"value_ms", "ms"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[f.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectReverbControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectReverbControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value = &v
				req.Unit = f.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectReverbControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectReverbControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveReverbControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectReverbControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectReverbControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectReverbControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildReverbSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_reverb"
		}
		return digest, nil, rejectReverbControl(boundary, "no provable supported reverb stage/control-path topology")
	}
	if err := attachReverbControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectReverbControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planReverbControl(summary map[string]any, req reverbControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeReverbControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectReverbControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectReverbControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectReverbControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findReverbSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectReverbControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectReverbControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateReverbRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		cn, nok := firstNumericAny(binding, "current_normalized")
		cp, pok := firstNumericAny(binding, "current_physical")
		if !nok || !pok {
			return eqWriteStep{}, nil, rejectReverbControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
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
		return eqWriteStep{}, nil, translateReverbError(err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing = normalized, &actual, quantized, decreasing
	write.DiscretePhysical = quantized
	return write, plan, nil
}

func translateReverbError(err error) error {
	if err == nil {
		return nil
	}
	if f, ok := err.(*compressorControlFailure); ok {
		return rejectReverbControl(f.Code, "%s", f.Message)
	}
	return rejectReverbControl("physical_target_failed", "%v", err)
}

func validateReverbRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	if !reverbUnitCompatible(role, unit, domain) {
		return rejectReverbControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func reverbUnitCompatible(role, unit, domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	switch unit {
	case "s":
		return reverbRoleIn(role, "decay", "size") && (domain == "s" || domain == "ms" || domain == "")
	case "ms":
		return reverbRoleIn(role, "pre_delay") && (domain == "ms" || domain == "s" || domain == "")
	case "%":
		// decay/size ship as percent on the measured Pro-R surface (M2-TIMEFX-1
		// decay_pct redesign); the domain-row dose writes them directly in %,
		// so percent is a first-class decay/size unit alongside seconds.
		return reverbRoleIn(role, "mix", "wet", "dry", "decay", "size") && (domain == "%" || domain == "")
	case "display":
		return domain != "enum" && domain != "toggle"
	}
	return false
}

func findReverbSummaryBinding(summary map[string]any, ref reverbControlReference) map[string]any {
	stage := mapValue(summary["reverb_stage"])
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

func (s *Server) restorePluginGrabberReverbControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeReverbRestoreRef(encoded)
	if err != nil {
		return nil, rejectReverbControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectReverbControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectReverbControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, v := range ref.Values {
		if v.Normalized < 0 || v.Normalized > 1 {
			return nil, rejectReverbControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", v.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: v.ParamID, NormalizedValue: v.Normalized, Role: v.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectReverbControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectReverbControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual, "rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}

func reverbRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}
