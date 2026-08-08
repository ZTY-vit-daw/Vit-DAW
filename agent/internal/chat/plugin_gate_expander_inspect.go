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
	pluginGrabberInspectGateExpanderCommand = "plugin_grabber_inspect_gate_expander"
	pluginGrabberInspectGateExpanderTool    = "plugin_grabber.inspect_gate_expander"
	gateExpanderControlRefKind              = "gate_expander_control_ref.v1"
	gateExpanderRestoreRefKind              = "gate_expander_restore_ref.v1"
)

type gateExpanderControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type gateExpanderRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role"`
}

type gateExpanderRestoreReference struct {
	Kind               string                     `json:"kind"`
	TrackID            string                     `json:"track_id"`
	PluginID           string                     `json:"plugin_id"`
	TopologyGeneration string                     `json:"generation"`
	Values             []gateExpanderRestoreValue `json:"values"`
}

func pluginGrabberInspectGateExpanderInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectGateExpanderTool && name != pluginGrabberInspectGateExpanderCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectGateExpanderCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectGateExpanderCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectGateExpanderWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectGateExpanderTool,
		CommandName: pluginGrabberInspectGateExpanderCommand, RiskLevel: tools.RiskDirect}
	target, err := s.resolvePluginObservationTarget(ctx, workflowCmd, req.Context, firstNonEmptyText(workflowCmd, "intent"))
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	if s.eqKernelClient() == nil {
		err = fmt.Errorf("kernel client is nil")
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": target.TrackID,
		"plugin_id": target.PluginID, "include_parameters": true})
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	if !kernelReplyOK(reply) {
		err = fmt.Errorf("%s", firstNonEmpty(firstNonEmptyText(reply, "message", "error"), "get_plugin_parameters failed"))
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	s.observePluginParametersReply(reply)
	digest := plugingrabber.BuildParameterDigest(reply)
	summary, boundary := plugingrabber.BuildGateExpanderSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_gate_expander"
		}
		err = fmt.Errorf("%s: no provable supported hard-gate or downward-expander stage topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachGateExpanderControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"] = "ok"
	summary["track_id"] = target.TrackID
	summary["plugin_id"] = target.PluginID
	out.Result = summary
	return out, nil
}

func attachGateExpanderControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	stage := mapValue(summary["gate_expander_stage"])
	stageKey := firstNonEmptyText(stage, "stage_key")
	if generation == "" || stageKey == "" {
		return fmt.Errorf("gate/expander topology omitted generation or stage key")
	}
	for _, section := range []string{"detector", "operating_point", "gain_action", "direction_control", "timing", "mode", "output"} {
		for _, row := range mapRowsValue(stage[section]) {
			if err := attachGateExpanderBindingRef(row, gateExpanderControlReference{Kind: gateExpanderControlRefKind,
				TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, Section: section}); err != nil {
				return err
			}
		}
	}
	return nil
}

func attachGateExpanderBindingRef(row map[string]any, base gateExpanderControlReference) error {
	base.Role = firstNonEmptyText(row, "role")
	base.ParamID = firstNonEmptyText(row, "param_id")
	if base.Role == "" || base.ParamID == "" {
		return fmt.Errorf("incomplete gate/expander binding")
	}
	encoded, err := encodeGateExpanderControlRef(base)
	if err != nil {
		return err
	}
	row["control_ref"] = encoded
	return nil
}

func encodeGateExpanderControlRef(ref gateExpanderControlReference) (string, error) {
	if ref.Kind != gateExpanderControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" ||
		ref.StageKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete gate/expander control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "g1cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeGateExpanderControlRef(value string) (gateExpanderControlReference, error) {
	var ref gateExpanderControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "g1cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: gate/expander control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "g1cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: gate/expander control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: gate/expander control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != gateExpanderControlRefKind {
		return gateExpanderControlReference{}, fmt.Errorf("invalid_control_ref: gate/expander control_ref checksum or payload is invalid")
	}
	return ref, nil
}

func encodeGateExpanderRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := gateExpanderRestoreReference{Kind: gateExpanderRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid gate/expander restore preimage")
		}
		ref.Values = append(ref.Values, gateExpanderRestoreValue{ParamID: value.ParamID, Normalized: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete gate/expander restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "g1rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeGateExpanderRestoreRef(value string) (gateExpanderRestoreReference, error) {
	var ref gateExpanderRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "g1rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: gate/expander restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "g1rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: gate/expander restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: gate/expander restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != gateExpanderRestoreRefKind ||
		ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return gateExpanderRestoreReference{}, fmt.Errorf("invalid_restore_ref: gate/expander restore_ref checksum or payload is invalid")
	}
	for _, item := range ref.Values {
		if item.ParamID == "" || math.IsNaN(item.Normalized) || math.IsInf(item.Normalized, 0) {
			return gateExpanderRestoreReference{}, fmt.Errorf("invalid_restore_ref: gate/expander restore_ref contains an invalid value")
		}
	}
	return ref, nil
}
