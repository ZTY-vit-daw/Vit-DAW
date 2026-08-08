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
	pluginGrabberInspectTransientShaperCommand = "plugin_grabber_inspect_transient_shaper"
	pluginGrabberInspectTransientShaperTool    = "plugin_grabber.inspect_transient_shaper"
	transientShaperControlRefKind              = "transient_shaper_control_ref.v1"
	transientShaperRestoreRefKind              = "transient_shaper_restore_ref.v1"
)

type transientShaperControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type transientShaperRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role"`
}

type transientShaperRestoreReference struct {
	Kind               string                        `json:"kind"`
	TrackID            string                        `json:"track_id"`
	PluginID           string                        `json:"plugin_id"`
	TopologyGeneration string                        `json:"generation"`
	Values             []transientShaperRestoreValue `json:"values"`
}

func pluginGrabberInspectTransientShaperInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectTransientShaperTool && name != pluginGrabberInspectTransientShaperCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectTransientShaperCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectTransientShaperCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectTransientShaperWorkflow(ctx context.Context, req harness.InvokeRequest, cmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectTransientShaperTool, CommandName: pluginGrabberInspectTransientShaperCommand, RiskLevel: tools.RiskDirect}
	target, err := s.resolvePluginObservationTarget(ctx, cmd, req.Context, firstNonEmptyText(cmd, "intent"))
	if err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	if s.eqKernelClient() == nil {
		err = fmt.Errorf("kernel client is nil")
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": target.TrackID, "plugin_id": target.PluginID, "include_parameters": true})
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
	summary, boundary := plugingrabber.BuildTransientShaperSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_transient_shaper"
		}
		err = fmt.Errorf("%s: no provable supported transient-shaper envelope topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachTransientShaperControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"], summary["track_id"], summary["plugin_id"] = "ok", target.TrackID, target.PluginID
	out.Result = summary
	return out, nil
}

func attachTransientShaperControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	stage := mapValue(summary["transient_shaper_stage"])
	stageKey := firstNonEmptyText(stage, "stage_key")
	if generation == "" || stageKey == "" {
		return fmt.Errorf("transient-shaper topology omitted generation or stage key")
	}
	for _, section := range []string{"envelope_action", "detector", "timing", "shape", "mode", "output"} {
		for _, row := range mapRowsValue(stage[section]) {
			if err := attachTransientShaperBindingRef(row, transientShaperControlReference{Kind: transientShaperControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, Section: section}); err != nil {
				return err
			}
		}
	}
	return nil
}

func attachTransientShaperBindingRef(row map[string]any, base transientShaperControlReference) error {
	base.Role, base.ParamID = firstNonEmptyText(row, "role"), firstNonEmptyText(row, "param_id")
	if base.Role == "" || base.ParamID == "" {
		return fmt.Errorf("incomplete transient-shaper binding")
	}
	encoded, err := encodeTransientShaperControlRef(base)
	if err != nil {
		return err
	}
	row["control_ref"] = encoded
	return nil
}

func encodeTransientShaperControlRef(ref transientShaperControlReference) (string, error) {
	if ref.Kind != transientShaperControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || ref.StageKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete transient-shaper control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "ts1cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeTransientShaperControlRef(value string) (transientShaperControlReference, error) {
	var ref transientShaperControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "ts1cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: transient-shaper control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "ts1cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: transient-shaper control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: transient-shaper control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != transientShaperControlRefKind {
		return transientShaperControlReference{}, fmt.Errorf("invalid_control_ref: transient-shaper control_ref checksum or payload is invalid")
	}
	return ref, nil
}

func encodeTransientShaperRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := transientShaperRestoreReference{Kind: transientShaperRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid transient-shaper restore preimage")
		}
		ref.Values = append(ref.Values, transientShaperRestoreValue{ParamID: value.ParamID, Normalized: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete transient-shaper restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "ts1rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeTransientShaperRestoreRef(value string) (transientShaperRestoreReference, error) {
	var ref transientShaperRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "ts1rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: transient-shaper restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "ts1rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: transient-shaper restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: transient-shaper restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != transientShaperRestoreRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return transientShaperRestoreReference{}, fmt.Errorf("invalid_restore_ref: transient-shaper restore_ref checksum or payload is invalid")
	}
	for _, value := range ref.Values {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return transientShaperRestoreReference{}, fmt.Errorf("invalid_restore_ref: transient-shaper restore_ref contains an invalid value")
		}
	}
	return ref, nil
}
