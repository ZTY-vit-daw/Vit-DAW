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
	pluginGrabberApplyCompressorCommand = "plugin_grabber_apply_compressor_controls"
	pluginGrabberApplyCompressorTool    = "plugin_grabber.apply_compressor_controls"
)

type compressorControlFailure struct {
	Code    string
	Message string
}

func (failure *compressorControlFailure) Error() string { return failure.Code + ": " + failure.Message }

func rejectCompressorControl(code, format string, args ...any) error {
	return &compressorControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}

func compressorControlFailureCode(err error) string {
	if failure, ok := err.(*compressorControlFailure); ok {
		return failure.Code
	}
	return "compressor_control_failed"
}

type compressorControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyCompressorInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName != pluginGrabberApplyCompressorTool && toolName != pluginGrabberApplyCompressorCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyCompressorCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyCompressorCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyCompressorWorkflow(ctx context.Context, req harness.InvokeRequest,
	workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyCompressorTool,
		CommandName: pluginGrabberApplyCompressorCommand, RiskLevel: tools.RiskUndoable, RequiresConfirmation: false}
	result, err := s.applyPluginGrabberCompressorControls(ctx, workflowCmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = compressorControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberCompressorControls(ctx context.Context, workflowCmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(workflowCmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectCompressorControl("atomic_required", "compressor controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, workflowCmd, requestContext, "")
	if err != nil {
		return nil, rejectCompressorControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveCompressorControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectCompressorControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restoreRef := firstNonEmptyText(args, "restore_ref"); restoreRef != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectCompressorControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberCompressorControls(ctx, target.TrackID, target.PluginID, generation, digest, restoreRef)
	}
	requests, err := parseCompressorControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seenParameters := map[string]bool{}
	for index, request := range requests {
		write, plan, planErr := planCompressorControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": index,
				"rejection_code": compressorControlFailureCode(planErr)}, planErr
		}
		if seenParameters[write.ParamID] {
			return map[string]any{"status": "rejected", "failed_control_index": index,
				"rejection_code": "duplicate_control"}, rejectCompressorControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
		}
		seenParameters[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectCompressorControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectCompressorControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeCompressorRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectCompressorControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectCompressorControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectCompressorControl(code, "%v", err)
	}
	overall := "exact"
	for index := range results {
		results[index]["actual_readback"] = eqActualRowsForWrites(actual, []eqWriteStep{writes[index]})
		if writes[index].Quantized {
			results[index]["status"] = "quantized"
			overall = "quantized"
		} else {
			results[index]["status"] = "exact"
		}
	}
	result := map[string]any{"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID,
		"topology_generation": generation, "controls": results, "writes": executed,
		"restore_ref": restoreRef,
		"rollback":    map[string]any{"on_failure": "full_preimage", "verified": true}}
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

func (s *Server) resolveCompressorTransactionalProbeDirections(ctx context.Context, trackID, pluginID string,
	digest plugingrabber.ParameterDigest, writes []eqWriteStep, preimage, snapshot []eqPreimageValue) error {
	params := map[string]plugingrabber.ParameterInfo{}
	for _, param := range digest.Parameters {
		params[param.ID] = param
	}
	preimageByID := map[string]eqPreimageValue{}
	for _, value := range preimage {
		preimageByID[value.ParamID] = value
	}
	for index := range writes {
		write := &writes[index]
		if !write.TransactionalProbe {
			continue
		}
		before, ok := preimageByID[write.ParamID]
		if !ok {
			return fmt.Errorf("transactional probe omitted preimage for %s", write.ParamID)
		}
		param, ok := params[write.ParamID]
		if !ok {
			return fmt.Errorf("transactional probe omitted parameter %s", write.ParamID)
		}
		currentPhysical, ok := plugingrabber.ParseCompressorPhysical(write.Role, param.ValueText)
		if !ok {
			return fmt.Errorf("cannot parse %s transactional probe seed %q", write.Role, param.ValueText)
		}
		foundDirection := false
		resolvedAtProbe := false
		seen := map[float64]bool{before.Normalized: true}
		for _, delta := range []float64{0.125, -0.125, 0.25, -0.25, 0.5, -0.5, 1, -1} {
			probeNormalized := math.Max(0, math.Min(1, before.Normalized+delta))
			if seen[probeNormalized] {
				continue
			}
			seen[probeNormalized] = true
			result, probeErr := s.eqKernelClient().SendVSPCommand(ctx, "plugin.set_params_batch", map[string]any{
				"track_id": trackID, "plugin_id": pluginID,
				"parameters": []map[string]any{{"parameter_id": write.ParamID, "normalized_value": probeNormalized}}, "readback": true,
			})
			if probeErr != nil || eqVSPFailure(result) != "" {
				failure := firstNonEmpty(errorText(probeErr), eqVSPFailure(result), "transactional direction probe failed")
				return s.eqTransactionFailure(ctx, trackID, pluginID, preimage, failure)
			}
			rows, readErr := s.readEQParameters(ctx, trackID, pluginID)
			if readErr != nil {
				return s.eqTransactionFailure(ctx, trackID, pluginID, preimage, fmt.Sprintf("transactional direction readback failed: %v", readErr))
			}
			rowIndex := eqParameterRowIndex(rows)
			if unexpected := unexpectedEQParameterChanges(snapshot, writes, rowIndex); len(unexpected) > 0 {
				return s.eqTransactionFailure(ctx, trackID, pluginID, append(preimage, unexpected...),
					fmt.Sprintf("transactional direction probe changed unplanned parameters: %s", eqPreimageParamIDs(unexpected)))
			}
			actual, parseOK := physicalReadbackForRole(write.Role, rowIndex[write.ParamID])
			if parseOK && !nearlyEqualCompressor(actual, currentPhysical) {
				write.PhysicalDecreasing = (actual-currentPhysical)/(probeNormalized-before.Normalized) < 0
				if write.RequestedPhysical != nil && math.Abs(actual-*write.RequestedPhysical) <= eqPhysicalTolerance(write.Role, *write.RequestedPhysical) {
					write.NormalizedValue = probeNormalized
					resolvedAtProbe = true
				}
				if write.RequestedPhysical != nil && compressorTargetBetween(*write.RequestedPhysical, currentPhysical, actual) {
					write.CorrectionLow = math.Min(before.Normalized, probeNormalized)
					write.CorrectionHigh = math.Max(before.Normalized, probeNormalized)
				}
				foundDirection = true
			}
			if restoreErr := s.rollbackEQTransaction(ctx, trackID, pluginID, preimage); restoreErr != nil {
				return fmt.Errorf("transactional direction probe restore failed: %w", restoreErr)
			}
			if foundDirection {
				break
			}
		}
		if !foundDirection {
			return fmt.Errorf("%s did not expose a reversible physical direction inside normalized range", write.ParamID)
		}
		if !resolvedAtProbe {
			write.NormalizedValue = before.Normalized
		}
	}
	return nil
}

func compressorTargetBetween(target, one, two float64) bool {
	return target >= math.Min(one, two) && target <= math.Max(one, two)
}

func (s *Server) restorePluginGrabberCompressorControls(ctx context.Context, trackID, pluginID, generation string,
	digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeCompressorRestoreRef(encoded)
	if err != nil {
		return nil, rejectCompressorControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectCompressorControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectCompressorControl("stale_restore_ref", "restore_ref generation %s does not match current %s",
			ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, value := range ref.Values {
		if value.Normalized < 0 || value.Normalized > 1 {
			return nil, rejectCompressorControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", value.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: value.ParamID, NormalizedValue: value.Normalized, Role: value.Role,
			CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectCompressorControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectCompressorControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID,
		"topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual,
		"rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}

func parseCompressorControlRequests(args map[string]any) ([]compressorControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectCompressorControl("controls_required", "controls must contain at least one explicit compressor control")
	}
	out := make([]compressorControlRequest, 0, len(rows))
	for index, row := range rows {
		request := compressorControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if request.ControlRef == "" {
			return nil, rejectCompressorControl("control_ref_required", "control %d omitted control_ref from inspect_compressor", index)
		}
		fields := []struct {
			Key, Unit string
		}{{"value_db", "dB"}, {"ratio", "ratio"}, {"value_ms", "ms"}, {"percent", "%"}, {"display_value", "display"}}
		for _, field := range fields {
			if value, ok := numericAny(row[field.Key]); ok {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return nil, rejectCompressorControl("invalid_value", "control %d contains a non-finite target", index)
				}
				if request.Value != nil || request.EnumLabel != "" {
					return nil, rejectCompressorControl("ambiguous_value", "control %d specifies more than one target value", index)
				}
				request.Value, request.Unit = &value, field.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if request.Value != nil {
				return nil, rejectCompressorControl("ambiguous_value", "control %d mixes numeric and enum targets", index)
			}
			request.EnumLabel = label
		}
		if request.Value == nil && request.EnumLabel == "" {
			return nil, rejectCompressorControl("value_required", "control %d has no physical or enum target", index)
		}
		out = append(out, request)
	}
	return out, nil
}

func (s *Server) readLiveCompressorControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectCompressorControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID,
		"plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectCompressorControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectCompressorControl("parameter_read_failed", "%s",
			firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildCompressorSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_compressor"
		}
		return digest, nil, rejectCompressorControl(boundary, "no provable supported broadband compressor stage/control-path topology")
	}
	return digest, summary, nil
}

func planCompressorControl(summary map[string]any, request compressorControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeCompressorControlRef(request.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectCompressorControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectCompressorControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectCompressorControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findCompressorSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectCompressorControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"),
		CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": request.ControlRef, "role": ref.Role, "path_key": ref.PathKey, "requested": request.Requested}
	if request.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, request.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectCompressorControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, request.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, request.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(request.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateCompressorRequestUnit(ref.Role, request.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *request.Value
	if fallback, _ := binding["transactional_probe_required"].(bool); fallback {
		currentNormalized, normalizedOK := firstNumericAny(binding, "current_normalized")
		currentPhysical, physicalOK := firstNumericAny(binding, "current_physical")
		if !normalizedOK || !physicalOK {
			return eqWriteStep{}, nil, rejectCompressorControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
		}
		write.NormalizedValue = currentNormalized
		write.RequestedPhysical = &target
		if nearlyEqualCompressor(currentPhysical, target) {
			return write, plan, nil
		}
		write.TransactionalProbe = true
		return write, plan, nil
	}
	normalized, actualTarget, quantized, decreasing, err := compressorPhysicalTarget(binding, target, request.Unit)
	if err != nil {
		return eqWriteStep{}, nil, err
	}
	write.NormalizedValue = normalized
	write.RequestedPhysical = &actualTarget
	write.Quantized = quantized
	write.PhysicalDecreasing = decreasing
	if quantized {
		write.DiscretePhysical = true
	}
	return write, plan, nil
}

func findCompressorSummaryBinding(summary map[string]any, ref compressorControlReference) map[string]any {
	stage := mapValue(summary["compressor_stage"])
	if firstNonEmptyText(stage, "stage_key") != ref.StageKey {
		return nil
	}
	if ref.Section == "output" && ref.PathKey == "stage_output" {
		return matchingCompressorBinding(mapRowsValue(stage["output"]), ref)
	}
	for _, path := range mapRowsValue(stage["control_paths"]) {
		if firstNonEmptyText(path, "path_key") == ref.PathKey {
			return matchingCompressorBinding(mapRowsValue(path[ref.Section]), ref)
		}
	}
	return nil
}

func matchingCompressorBinding(rows []map[string]any, ref compressorControlReference) map[string]any {
	for _, row := range rows {
		if firstNonEmptyText(row, "param_id") == ref.ParamID && firstNonEmptyText(row, "role") == ref.Role {
			return row
		}
	}
	return nil
}

func validateCompressorRequestUnit(role, unit string, binding map[string]any) error {
	domainUnit := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"),
		firstNonEmptyText(binding, "physical_unit")))
	if !compressorUnitCompatible(role, unit, domainUnit) {
		return rejectCompressorControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domainUnit)
	}
	return nil
}

func compressorUnitCompatible(role, unit, domainUnit string) bool {
	domainUnit = strings.ToLower(strings.TrimSpace(domainUnit))
	switch unit {
	case "dB":
		return compressorRoleIn(role, "threshold", "input_drive", "reduction_amount", "low_level_amount", "high_level_amount", "knee", "reduction_range", "makeup_gain", "output_gain", "wet_gain", "dry_gain") && domainUnit == "db"
	case "ratio":
		return compressorRoleIn(role, "ratio", "direction_curve")
	case "ms":
		return compressorRoleIn(role, "attack", "release", "recovery", "time_constant", "pdr_time", "lookahead", "hold") && (domainUnit == "ms" || domainUnit == "s" || domainUnit == "")
	case "%":
		return domainUnit == "%" || (role == "mix" && domainUnit == "")
	case "display":
		return domainUnit != "enum" && domainUnit != "toggle"
	default:
		return false
	}
}

func compressorPhysicalTarget(binding map[string]any, requested float64, unit string) (float64, float64, bool, bool, error) {
	curve := compressorCurveFromAny(binding["curve"])
	if len(curve) >= 3 {
		lo, hi := curve[0][1], curve[len(curve)-1][1]
		minValue, maxValue := math.Min(lo, hi), math.Max(lo, hi)
		if requested < minValue || requested > maxValue {
			return 0, 0, false, false, rejectCompressorControl("value_out_of_range", "requested %g is outside observed range %g..%g", requested, minValue, maxValue)
		}
		normalized, ok := eqNormalizedFromCurve(requested, curve)
		if !ok {
			return 0, 0, false, false, rejectCompressorControl("display_curve_unusable", "observed display curve is not invertible")
		}
		return normalized, requested, false, hi < lo, nil
	}
	reachable := mapRowsValue(binding["reachable_values"])
	if len(reachable) > 0 {
		bestDistance := math.Inf(1)
		var best map[string]any
		for _, row := range reachable {
			physical, ok := firstNumericAny(row, "physical")
			if !ok {
				continue
			}
			if distance := math.Abs(physical - requested); distance < bestDistance {
				bestDistance, best = distance, row
			}
		}
		if best != nil {
			normalized, _ := firstNumericAny(best, "normalized")
			physical, _ := firstNumericAny(best, "physical")
			return normalized, physical, !nearlyEqualCompressor(physical, requested), false, nil
		}
	}
	domain := mapValue(binding["domain"])
	minValue, minOK := firstNumericAny(domain, "min")
	maxValue, maxOK := firstNumericAny(domain, "max")
	confidence, _ := firstNumericAny(domain, "confidence")
	if !minOK || !maxOK || confidence < 0.80 {
		return 0, 0, false, false, rejectCompressorControl("physical_domain_unavailable", "binding has no sufficiently reliable measured physical domain")
	}
	if unit == "ms" && strings.EqualFold(firstNonEmptyText(domain, "unit"), "s") {
		minValue *= 1000
		maxValue *= 1000
	}
	if requested < math.Min(minValue, maxValue) || requested > math.Max(minValue, maxValue) {
		return 0, 0, false, false, rejectCompressorControl("value_out_of_range", "requested %g is outside observed range %g..%g", requested, minValue, maxValue)
	}
	scale := firstNonEmpty(firstNonEmptyText(domain, "scale"), "linear")
	return normalizeEQValue(requested, minValue, maxValue, scale), requested, false, maxValue < minValue, nil
}

func compressorEnumTarget(binding map[string]any, requested string) (float64, string, bool) {
	for _, row := range mapRowsValue(binding["reachable_values"]) {
		label := firstNonEmptyText(row, "label")
		if strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(requested)) {
			normalized, ok := firstNumericAny(row, "normalized")
			return normalized, label, ok
		}
	}
	return 0, "", false
}

func compressorCurveFromAny(value any) [][2]float64 {
	switch typed := value.(type) {
	case [][2]float64:
		return typed
	case []any:
		out := make([][2]float64, 0, len(typed))
		for _, item := range typed {
			pair, ok := item.([]any)
			if !ok || len(pair) != 2 {
				return nil
			}
			normalized, nOK := numericAny(pair[0])
			physical, pOK := numericAny(pair[1])
			if !nOK || !pOK {
				return nil
			}
			out = append(out, [2]float64{normalized, physical})
		}
		return out
	default:
		return nil
	}
}

func compressorRoleIn(role string, values ...string) bool {
	for _, value := range values {
		if role == value {
			return true
		}
	}
	return false
}

func nearlyEqualCompressor(one, two float64) bool {
	return math.Abs(one-two) <= math.Max(1e-6, math.Abs(two)*1e-6)
}
