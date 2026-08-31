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

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberApplyDeEsserCommand = "plugin_grabber_apply_de_esser_controls"
	pluginGrabberApplyDeEsserTool    = "plugin_grabber.apply_de_esser_controls"
	deEsserRestoreRefKind            = "de_esser_restore_ref.v1"
)

type deEsserControlFailure struct{ Code, Message string }

func (f *deEsserControlFailure) Error() string { return f.Code + ": " + f.Message }
func rejectDeEsserControl(code, format string, args ...any) error {
	return &deEsserControlFailure{Code: code, Message: fmt.Sprintf(format, args...)}
}
func deEsserControlFailureCode(err error) string {
	if f, ok := err.(*deEsserControlFailure); ok {
		return f.Code
	}
	return "de_esser_control_failed"
}

type deEsserControlRequest struct {
	ControlRef      string
	Value           *float64
	Unit, EnumLabel string
	Requested       map[string]any
}
type deEsserRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role,omitempty"`
}
type deEsserRestoreReference struct {
	Kind               string                `json:"kind"`
	TrackID            string                `json:"track_id"`
	PluginID           string                `json:"plugin_id"`
	TopologyGeneration string                `json:"generation"`
	Values             []deEsserRestoreValue `json:"values"`
}

func pluginGrabberApplyDeEsserInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberApplyDeEsserTool && name != pluginGrabberApplyDeEsserCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberApplyDeEsserCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberApplyDeEsserCommand}
	for k, v := range req.Args {
		cmd[k] = v
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberApplyDeEsserWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberApplyDeEsserTool, CommandName: pluginGrabberApplyDeEsserCommand, RiskLevel: tools.RiskUndoable}
	result, err := s.applyPluginGrabberDeEsserControls(ctx, cmd, req.Context)
	out.Result = result
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		if out.Result == nil {
			out.Result = map[string]any{}
		}
		out.Result["status"] = "rejected"
		out.Result["rejection_code"] = deEsserControlFailureCode(err)
		out.Result["message"] = err.Error()
		return out, err
	}
	return out, nil
}

func (s *Server) applyPluginGrabberDeEsserControls(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	args := workflowCommandArgs(cmd)
	if atomic, ok := args["atomic"].(bool); ok && !atomic {
		return nil, rejectDeEsserControl("atomic_required", "De-esser controls only support atomic=true")
	}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, requestContext, "")
	if err != nil {
		return nil, rejectDeEsserControl("target_unavailable", "%v", err)
	}
	digest, summary, err := s.readLiveDeEsserControlSurface(ctx, target.TrackID, target.PluginID)
	if err != nil {
		return nil, err
	}
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return nil, rejectDeEsserControl("topology_generation_unavailable", "recognizer did not publish a topology generation")
	}
	if restore := firstNonEmptyText(args, "restore_ref"); restore != "" {
		if len(mapRowsValue(args["controls"])) > 0 {
			return nil, rejectDeEsserControl("ambiguous_restore", "restore_ref cannot be combined with controls")
		}
		return s.restorePluginGrabberDeEsserControls(ctx, target.TrackID, target.PluginID, generation, digest, restore)
	}
	requests, err := parseDeEsserControlRequests(args)
	if err != nil {
		return nil, err
	}
	writes := make([]eqWriteStep, 0, len(requests))
	results := make([]map[string]any, 0, len(requests))
	seen := map[string]bool{}
	for i, request := range requests {
		write, plan, planErr := planDeEsserControl(summary, request, target.TrackID, target.PluginID, generation)
		if planErr != nil {
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": deEsserControlFailureCode(planErr)}, planErr
		}
		if seen[write.ParamID] {
			e := rejectDeEsserControl("duplicate_control", "parameter %s appears more than once in one atomic request", write.ParamID)
			return map[string]any{"status": "rejected", "failed_control_index": i, "rejection_code": deEsserControlFailureCode(e)}, e
		}
		seen[write.ParamID] = true
		writes = append(writes, write)
		results = append(results, plan)
	}
	writes, err = combineAtomicEQWrites([]eqPlannedEdit{{Writes: writes}})
	if err != nil {
		return nil, rejectDeEsserControl("parameter_conflict", "%v", err)
	}
	preimage, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectDeEsserControl("preimage_unavailable", "%v", err)
	}
	restoreRef, err := encodeDeEsserRestoreRef(target.TrackID, target.PluginID, generation, preimage)
	if err != nil {
		return nil, rejectDeEsserControl("restore_ref_unavailable", "%v", err)
	}
	snapshot := eqParameterSnapshot(digest)
	if err = s.resolveCompressorTransactionalProbeDirections(ctx, target.TrackID, target.PluginID, digest, writes, preimage, snapshot); err != nil {
		return nil, rejectDeEsserControl("physical_probe_failed", "%v", err)
	}
	requestID := firstNonEmptyText(args, "request_id")
	executed, actual, accounting, err := s.executeEQTransactionAccounted(ctx, target.TrackID, target.PluginID, writes, preimage, snapshot, requestID)
	if err != nil {
		code := "atomic_execution_failed"
		if strings.Contains(err.Error(), "unplanned_parameter_change") {
			code = "unplanned_parameter_change"
		}
		return nil, rejectDeEsserControl(code, "%v", err)
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

func parseDeEsserControlRequests(args map[string]any) ([]deEsserControlRequest, error) {
	rows := mapRowsValue(args["controls"])
	if len(rows) == 0 {
		return nil, rejectDeEsserControl("controls_required", "controls must contain at least one explicit De-esser control")
	}
	out := make([]deEsserControlRequest, 0, len(rows))
	for i, row := range rows {
		req := deEsserControlRequest{ControlRef: firstNonEmptyText(row, "control_ref"), Requested: cloneStringAnyMap(row)}
		if req.ControlRef == "" {
			return nil, rejectDeEsserControl("control_ref_required", "control %d omitted control_ref from inspect_de_esser", i)
		}
		for _, f := range []struct{ Key, Unit string }{{"value_db", "dB"}, {"value_ms", "ms"}, {"frequency_hz", "Hz"}, {"percent", "%"}, {"display_value", "display"}} {
			if v, ok := numericAny(row[f.Key]); ok {
				if req.Value != nil || req.EnumLabel != "" {
					return nil, rejectDeEsserControl("ambiguous_value", "control %d specifies more than one target value", i)
				}
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return nil, rejectDeEsserControl("invalid_value", "control %d contains a non-finite target", i)
				}
				req.Value, req.Unit = &v, f.Unit
			}
		}
		if label := firstNonEmptyText(row, "enum_label"); label != "" {
			if req.Value != nil {
				return nil, rejectDeEsserControl("ambiguous_value", "control %d mixes numeric and enum targets", i)
			}
			req.EnumLabel = label
		}
		if req.Value == nil && req.EnumLabel == "" {
			return nil, rejectDeEsserControl("value_required", "control %d has no physical or enum target", i)
		}
		out = append(out, req)
	}
	return out, nil
}

func (s *Server) readLiveDeEsserControlSurface(ctx context.Context, trackID, pluginID string) (plugingrabber.ParameterDigest, map[string]any, error) {
	var digest plugingrabber.ParameterDigest
	if s.eqKernelClient() == nil {
		return digest, nil, rejectDeEsserControl("kernel_unavailable", "kernel client is nil")
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": trackID, "plugin_id": pluginID, "include_parameters": true})
	if err != nil {
		return digest, nil, rejectDeEsserControl("parameter_read_failed", "%v", err)
	}
	if !kernelReplyOK(reply) {
		return digest, nil, rejectDeEsserControl("parameter_read_failed", "%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
	}
	s.observePluginParametersReply(reply)
	digest = plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildDeEsserSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_de_esser"
		}
		return digest, nil, rejectDeEsserControl(boundary, "no provable supported De-esser stage/control-path topology")
	}
	if err = attachDeEsserControlRefs(summary, trackID, pluginID); err != nil {
		return digest, nil, rejectDeEsserControl("binding_ref_failed", "%v", err)
	}
	return digest, summary, nil
}

func planDeEsserControl(summary map[string]any, req deEsserControlRequest, trackID, pluginID, generation string) (eqWriteStep, map[string]any, error) {
	ref, err := decodeDeEsserControlRef(req.ControlRef)
	if err != nil {
		return eqWriteStep{}, nil, rejectDeEsserControl("invalid_control_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return eqWriteStep{}, nil, rejectDeEsserControl("control_ref_target_mismatch", "control_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return eqWriteStep{}, nil, rejectDeEsserControl("stale_control_ref", "control_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	binding := findDeEsserSummaryBinding(summary, ref)
	if binding == nil {
		return eqWriteStep{}, nil, rejectDeEsserControl("binding_unavailable", "control_ref binding is absent from current topology")
	}
	write := eqWriteStep{ParamID: ref.ParamID, Role: ref.Role, Channel: firstNonEmptyText(binding, "channel"), CorrectionLow: 0, CorrectionHigh: 1}
	plan := map[string]any{"control_ref": req.ControlRef, "stage_key": ref.StageKey, "section": ref.Section, "role": ref.Role, "requested": req.Requested}
	if req.EnumLabel != "" {
		normalized, label, ok := compressorEnumTarget(binding, req.EnumLabel)
		if !ok {
			return eqWriteStep{}, nil, rejectDeEsserControl("enum_value_unavailable", "%s has no reachable enum label %q", ref.Role, req.EnumLabel)
		}
		write.NormalizedValue, write.RequestedLabel, write.ExpectedLabel = normalized, req.EnumLabel, label
		write.Quantized = !strings.EqualFold(strings.TrimSpace(req.EnumLabel), strings.TrimSpace(label))
		return write, plan, nil
	}
	if err := validateDeEsserRequestUnit(ref.Role, req.Unit, binding); err != nil {
		return eqWriteStep{}, nil, err
	}
	target := *req.Value
	if probe, _ := binding["transactional_probe_required"].(bool); probe {
		currentNormalized, normalizedOK := firstNumericAny(binding, "current_normalized")
		currentPhysical, physicalOK := firstNumericAny(binding, "current_physical")
		if !normalizedOK || !physicalOK {
			return eqWriteStep{}, nil, rejectDeEsserControl("physical_probe_unavailable", "%s omitted its transactional probe seed", ref.Role)
		}
		write.NormalizedValue, write.RequestedPhysical = currentNormalized, &target
		if nearlyEqualCompressor(currentPhysical, target) {
			return write, plan, nil
		}
		write.TransactionalProbe = true
		return write, plan, nil
	}
	normalized, actual, quantized, decreasing, err := compressorPhysicalTarget(binding, target, req.Unit)
	if err != nil {
		if f, ok := err.(*compressorControlFailure); ok {
			return eqWriteStep{}, nil, rejectDeEsserControl(f.Code, "%s", f.Message)
		}
		return eqWriteStep{}, nil, rejectDeEsserControl("physical_target_failed", "%v", err)
	}
	write.NormalizedValue, write.RequestedPhysical, write.Quantized, write.PhysicalDecreasing, write.DiscretePhysical = normalized, &actual, quantized, decreasing, quantized
	return write, plan, nil
}

func validateDeEsserRequestUnit(role, unit string, binding map[string]any) error {
	domain := strings.ToLower(firstNonEmpty(firstNonEmptyText(mapValue(binding["domain"]), "unit"), firstNonEmptyText(binding, "physical_unit")))
	ok := false
	switch unit {
	case "dB":
		ok = compressorRoleIn(role, "threshold", "reduction_range", "detection_amount", "output_gain") && (domain == "db" || domain == "")
	case "ms":
		ok = compressorRoleIn(role, "attack", "release", "lookahead") && (domain == "ms" || domain == "s" || domain == "")
	case "Hz":
		ok = role == "focus_frequency" && (domain == "hz" || domain == "")
	case "%":
		ok = compressorRoleIn(role, "reduction_range", "detection_amount", "mix") && (domain == "%" || domain == "")
	case "display":
		ok = domain != "enum" && domain != "toggle"
	}
	if !ok {
		return rejectDeEsserControl("unit_role_mismatch", "%s cannot be written with %s against observed domain unit %q", role, unit, domain)
	}
	return nil
}

func findDeEsserSummaryBinding(summary map[string]any, ref deEsserControlReference) map[string]any {
	for _, stage := range mapRowsValue(summary["de_esser_stages"]) {
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

func encodeDeEsserRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := deEsserRestoreReference{Kind: deEsserRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, v := range preimage {
		if v.ParamID == "" || math.IsNaN(v.Normalized) || math.IsInf(v.Normalized, 0) {
			return "", fmt.Errorf("invalid De-esser restore preimage")
		}
		ref.Values = append(ref.Values, deEsserRestoreValue{ParamID: v.ParamID, Normalized: v.Normalized, Role: v.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete De-esser restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "d1rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}
func decodeDeEsserRestoreRef(value string) (deEsserRestoreReference, error) {
	var ref deEsserRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "d1rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: De-esser restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "d1rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: De-esser restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: De-esser restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != deEsserRestoreRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return deEsserRestoreReference{}, fmt.Errorf("invalid_restore_ref: De-esser restore_ref checksum or payload is invalid")
	}
	return ref, nil
}

func (s *Server) restorePluginGrabberDeEsserControls(ctx context.Context, trackID, pluginID, generation string, digest plugingrabber.ParameterDigest, encoded string) (map[string]any, error) {
	ref, err := decodeDeEsserRestoreRef(encoded)
	if err != nil {
		return nil, rejectDeEsserControl("invalid_restore_ref", "%v", err)
	}
	if ref.TrackID != trackID || ref.PluginID != pluginID {
		return nil, rejectDeEsserControl("restore_ref_target_mismatch", "restore_ref belongs to another plugin instance")
	}
	if ref.TopologyGeneration != generation {
		return nil, rejectDeEsserControl("stale_restore_ref", "restore_ref generation %s does not match current %s", ref.TopologyGeneration, generation)
	}
	writes := make([]eqWriteStep, 0, len(ref.Values))
	for _, v := range ref.Values {
		if v.Normalized < 0 || v.Normalized > 1 || math.IsNaN(v.Normalized) || math.IsInf(v.Normalized, 0) {
			return nil, rejectDeEsserControl("invalid_restore_ref", "restore_ref value for %s is outside normalized range", v.ParamID)
		}
		writes = append(writes, eqWriteStep{ParamID: v.ParamID, NormalizedValue: v.Normalized, Role: v.Role, CorrectionLow: 0, CorrectionHigh: 1})
	}
	current, err := eqWritePreimage(digest, writes)
	if err != nil {
		return nil, rejectDeEsserControl("restore_preimage_unavailable", "%v", err)
	}
	executed, actual, err := s.executeEQTransaction(ctx, trackID, pluginID, writes, current, eqParameterSnapshot(digest))
	if err != nil {
		return nil, rejectDeEsserControl("atomic_restore_failed", "%v", err)
	}
	return map[string]any{"status": "exact", "atomic": true, "restored": true, "track_id": trackID, "plugin_id": pluginID, "topology_generation": generation, "restore_ref": encoded, "writes": executed, "actual_readback": actual, "rollback": map[string]any{"on_failure": "restore_call_preimage", "verified": true}}, nil
}
