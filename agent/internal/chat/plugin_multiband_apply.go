package chat

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberApplyMultibandCommand = "plugin_grabber_apply_multiband_controls"
	pluginGrabberApplyMultibandTool    = "plugin_grabber.apply_multiband_controls"
)

type multibandControlFailure struct{ Code, Message string }

func (f *multibandControlFailure) Error() string { return f.Code + ": " + f.Message }
func rejectMultibandControl(code, format string, args ...any) error {
	return &multibandControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}
func multibandControlFailureCode(err error) string {
	if f, ok := err.(*multibandControlFailure); ok {
		return f.Code
	}
	return "multiband_control_failed"
}

type multibandControlRequest struct {
	ControlRef string
	Value      *float64
	Unit       string
	EnumLabel  string
	Requested  map[string]any
}

func pluginGrabberApplyMultibandInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyMultibandTool && name != pluginGrabberApplyMultibandCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyMultibandCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyMultibandCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyMultibandWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyMultibandTool, CommandName: pluginGrabberApplyMultibandCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberMultibandControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"], out.Result["rejection_code"], out.Result["message"] = "rejected", multibandControlFailureCode(err), err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberMultibandControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectMultibandControl("atomic_required", "multiband controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectMultibandControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveMultibandControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectMultibandControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectMultibandControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberMultibandControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseMultibandControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for index, request := range requests {
		write, result, planErr := planMultibandControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": index, "rejection_code": multibandControlFailureCode(planErr), "parameters_changed": false}, planErr
		}
		if seen[write.ParamID] {
			e := rejectMultibandControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": index, "parameters_changed": false}, e
		}
		seen[write.ParamID] = true
		writes, results = append(writes, write), append(results, result)
	}
	if err = validateAtomicCrossoverOrder(summary, writes); err != nil {
		return map[string]any{"status": "rejected", "rejection_code": multibandControlFailureCode(err), "parameters_changed": false}, err
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectMultibandControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectMultibandControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeMultibandRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectMultibandControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectMultibandControl("physical_probe_failed", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectMultibandControl(code, "%v", err)
	}
	overall := "exact"
	for index := range results {
		results[index]["actual_readback"] = eqActualRowsForWrites(actual, []eqWriteStep{writes[index]})
		if writes[index].Quantized {
			results[index]["status"], overall = "quantized", "quantized"
		} else {
			results[index]["status"] = "exact"
		}
	}
	return map[string]any{"status": overall, "atomic": true, "track_id": target.TrackID, "plugin_id": target.PluginID, "topology_generation": generation, "controls": results, "writes": executed, "restore_ref": restoreRef, "rollback": map[string]any{"on_failure": "full_preimage", "verified": true}}, nil
}

func parseMultibandControlRequests(args map[string]any) ([]multibandControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectMultibandControl("controls_required", "controls must contain at least one explicit multiband control")
	}
	out := make([]multibandControlRequest, 0, len(rows))
	for index, row := range rows {
		req := multibandControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectMultibandControl("control_ref_required", "control %d omitted control_ref from inspect_multiband", index)
		}
		for _, field := range []struct{ Key, Unit string }{{"value_db", "dB"}, {"value_ms", "ms"}, {"value_hz", "Hz"}, {"ratio", "ratio"}, {"percent", "%"}, {"display_value", "display"}} {
			if value, ok := numericAny(row[field.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectMultibandControl("ambiguous_value", "control %d specifies more than one target value", index)
				}
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return nil, rejectMultibandControl("invalid_value", "control %d contains a non-finite target", index)
				}
				req.Value, req.Unit = &value, field.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectMultibandControl("ambiguous_value", "control %d mixes numeric and enum targets", index)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectMultibandControl("value_required", "control %d has no physical or enum target", index)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveMultibandControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectMultibandControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectMultibandControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectMultibandControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildMultibandSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_multiband_dynamics"
		}
		return digest, nil, rejectMultibandControl(boundary, "no provable supported multiband dynamics topology")
	}
	if err := attachMultibandControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectMultibandControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planMultibandControl(summary map[string]any, req multibandControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeMultibandControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectMultibandControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectMultibandControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectMultibandControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findMultibandSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectMultibandControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: ref.BandKey, CorrectionLow: 0, CorrectionHigh: 1}
	result := map[string]any{"control_ref": req.ControlRef, "section": ref.Section, "band_key": ref.BandKey, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectMultibandControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, result, nil
	}
	if err := validateMultibandRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		currentNormalized, normalizedOK := firstNumericAny(binding, "current_normalized")
		currentPhysical, physicalOK := firstNumericAny(binding, "current_physical")
		if !normalizedOK || !physicalOK {
			return eqWriteStep{}, nil, rejectMultibandControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
		}
		write.NormalizedValue, write.RequestedPhysical = currentNormalized, &target
		if !nearlyEqualCompressor(currentPhysical, target) {
			write.TransactionalProbe = true
		}
		return write, result, nil
	}
	normalized, actual, quantized, decreasing, targetErr := compressorPhysicalTarget(binding, target, req.Unit)
	if targetErr != nil {
		if f, ok := targetErr.(*compressorControlFailure); ok {
			return eqWriteStep{}, nil, rejectMultibandControl(f.Code, "%s", f.Message)
		}
		return eqWriteStep{}, nil, rejectMultibandControl("physical_target_failed", "%v", targetErr)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing = normalized, &actual, quantized, decreasing
	write.DiscretePhysical = quantized
	return write, result, nil
}

func validateMultibandRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	ok := false
	switch unit {
	case "Hz":
		ok = role == "crossover" && (domain == "hz" || domain == "khz")
	case "dB":
		ok = compressorRoleIn(role, "threshold", "reduction_amount", "range", "gain", "makeup_gain") && (domain == "db" || domain == "")
	case "ratio":
		ok = role == "ratio"
	case "ms":
		ok = compressorRoleIn(role, "attack", "release", "hold", "lookahead") && (domain == "ms" || domain == "s" || domain == "")
	case "%":
		ok = domain == "%"
	case "display":
		ok = domain != "enum" && domain != "toggle"
	}
	if !ok {
		return rejectMultibandControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func findMultibandSummaryBinding(summary map[string]any, ref multibandControlReference) map[string]any {
	if ref.Section == "filterbank" {
		return matchingMultibandBinding(mapRowsValue(mapValue(summary["filterbank"])["crossovers"]), ref)
	}
	if ref.Section == "shared" {
		return matchingMultibandBinding(mapRowsValue(summary["shared_controls"]), ref)
	}
	for _, band := range mapRowsValue(summary["band_cells"]) {
		if firstNonEmptyText(band, "band_key") == ref.BandKey {
			return matchingMultibandBinding(mapRowsValue(band[ref.Section]), ref)
		}
	}
	return nil
}
func matchingMultibandBinding(rows []map[string]any, ref multibandControlReference) map[string]any {
	for _, row := range rows {
		if firstNonEmptyText(row, "param_id") == ref.ParamID && firstNonEmptyText(row, "role") == ref.Role {
			return row
		}
	}
	return nil
}

func (s *Server) guardMultibandOwnedGenericParameterWrite(ctx context.Context, req harness.InvokeRequest) (harness.InvokeResponse, bool) {
	if !pluginSetParameterInvokeRequest(req) {
		return harness.InvokeResponse{}, false
	}
	args := workflowCommandArgs(req.Command)
	for key, value := range workflowCommandArgs(req.Args) {
		args[key] = value
	}
	trackID := firstNonEmptyText(args, "track_id", "selected_plugin_track_id", "selected_track_id")
	pluginID := firstNonEmptyText(args, "plugin_id", "selected_plugin_id", "plugin_item_id")
	paramID := firstNonEmptyText(args, "param_id", "parameter_id")
	if trackID == "" {
		trackID = firstNonEmptyText(req.Context, "selected_plugin_track_id", "selected_track_id", "track_id")
	}
	if pluginID == "" {
		pluginID = firstNonEmptyText(req.Context, "selected_plugin_id", "plugin_id")
	}
	if trackID == "" || pluginID == "" || paramID == "" {
		return harness.InvokeResponse{}, false
	}
	_, summary, err := s.readLiveMultibandControlSurface(ctx, trackID, pluginID)
	if err != nil {
		switch multibandControlFailureCode(err) {
		case "not_multiband_dynamics", "unsupported_dynamic_eq", "unsupported_de_esser", "unsupported_spectral_dynamics", "unsupported_clipper", "unsupported_multiband_maximizer", "unresolved_multiband_cells", "unresolved_shared_modifiers", "unresolved_crossover_order", "unresolved_band_local_crossovers":
			return harness.InvokeResponse{}, false
		}
		return multibandGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_multiband_surface_unavailable", "cannot prove this parameter is outside the typed multiband surface: "+err.Error()), true
	}
	if !multibandSummaryOwnsParameter(summary, paramID) {
		return harness.InvokeResponse{}, false
	}
	return multibandGenericWriteBlockedResponse(trackID, pluginID, paramID, "typed_multiband_control_required", "parameter is owned by the live multiband topology; use inspect_multiband and apply_multiband_controls"), true
}

func multibandSummaryOwnsParameter(summary map[string]any, paramID string) bool {
	for _, row := range mapRowsValue(mapValue(summary["filterbank"])["crossovers"]) {
		if firstNonEmptyText(row, "param_id") == paramID {
			return true
		}
	}
	for _, band := range mapRowsValue(summary["band_cells"]) {
		for _, section := range []string{"operating_point", "transfer", "timing", "gain_action", "mode"} {
			for _, row := range mapRowsValue(band[section]) {
				if firstNonEmptyText(row, "param_id") == paramID {
					return true
				}
			}
		}
	}
	for _, row := range mapRowsValue(summary["shared_controls"]) {
		if firstNonEmptyText(row, "param_id") == paramID {
			return true
		}
	}
	return false
}

func multibandGenericWriteBlockedResponse(trackID, pluginID, paramID, code, message string) harness.InvokeResponse {
	full := code + ": " + message
	return harness.InvokeResponse{Status: "error", Tool: "plugin.set_parameter", CommandName: "set_plugin_param", RiskLevel: tools.RiskUndoable, Error: full,
		Result: map[string]any{"status": "rejected", "rejection_code": code, "message": full, "track_id": trackID, "plugin_id": pluginID, "param_id": paramID, "required_tools": []string{pluginGrabberInspectMultibandTool, pluginGrabberApplyMultibandTool}, "parameters_changed": false}}
}

// validateAtomicCrossoverOrder runs after every control has been planned and
// before a preimage is read or any parameter write is attempted.
func validateAtomicCrossoverOrder(summary map[string]any, writes []eqWriteStep) error {
	targets := map[string]float64{}
	for _, write := range writes {
		if write.Role == "crossover" && write.RequestedPhysical != nil {
			targets[write.ParamID] = *write.RequestedPhysical
		}
	}
	values := []float64{}
	for _, row := range mapRowsValue(mapValue(summary["filterbank"])["crossovers"]) {
		id := firstNonEmptyText(row, "param_id")
		value, ok := targets[id]
		if !ok {
			value, ok = firstNumericAny(row, "current_physical")
		}
		if !ok {
			value, ok = plugingrabber.ParseCompressorPhysical("frequency", firstNonEmptyText(row, "current_text"))
		}
		if !ok {
			return rejectMultibandControl("crossover_order_unverifiable", "crossover %s has no physical frequency evidence", id)
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return rejectMultibandControl("crossover_order_unverifiable", "topology published no crossover controls")
	}
	for index := 1; index < len(values); index++ {
		if values[index] <= values[index-1] {
			return rejectMultibandControl("invalid_crossover_order", "requested crossover frequencies must remain strictly ordered")
		}
	}
	return nil
}

func decodeMultibandRestoreRef(value string) (multibandRestoreReference, error) {
	var ref multibandRestoreReference
	if !strings.HasPrefix(strings.TrimSpace(value), "mb1rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: multiband restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "mb1rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: multiband restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: multiband restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != multibandRestoreRefKind || len(ref.Values) == 0 {
		return multibandRestoreReference{}, fmt.Errorf("invalid_restore_ref: multiband restore_ref checksum or payload is invalid")
	}
	return ref, nil
}

func (s *Server) restorePluginGrabberMultibandControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeMultibandRestoreRef(encoded)
	if err != nil {
		return nil, rejectMultibandControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectMultibandControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectMultibandControl("stale_restore_ref", "restore_ref generation does not match current topology")
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, value := range ref.Values {
		if value.ParamID == "" || value.Value < 0 || value.Value > 1 {
			return nil, rejectMultibandControl("invalid_restore_ref", "restore_ref contains an invalid normalized value")
		}
		writes = append(writes, eqWriteStep{ParamID: value.ParamID, Role: value.Role, NormalizedValue: value.Value, CorrectionLow: 0, CorrectionHigh: 1})
	}
	sort.SliceStable(writes, func(i, j int) bool { return writes[i].ParamID < writes[j].ParamID })
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectMultibandControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectMultibandControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual}, nil
}
