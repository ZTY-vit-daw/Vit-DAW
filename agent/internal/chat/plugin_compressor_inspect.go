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
	pluginGrabberInspectCompressorCommand = "plugin_grabber_inspect_compressor"
	pluginGrabberInspectCompressorTool    = "plugin_grabber.inspect_compressor"
	compressorControlRefKind              = "compressor_control_ref.v1"
	compressorRestoreRefKind              = "compressor_restore_ref.v1"
)

type compressorControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	PathKey            string `json:"path_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type compressorRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role"`
}

type compressorRestoreReference struct {
	Kind               string                   `json:"kind"`
	TrackID            string                   `json:"track_id"`
	PluginID           string                   `json:"plugin_id"`
	TopologyGeneration string                   `json:"generation"`
	Values             []compressorRestoreValue `json:"values"`
}

func pluginGrabberInspectCompressorInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName != pluginGrabberInspectCompressorTool && toolName != pluginGrabberInspectCompressorCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectCompressorCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectCompressorCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectCompressorWorkflow(ctx context.Context, req harness.InvokeRequest,
	workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectCompressorTool,
		CommandName: pluginGrabberInspectCompressorCommand, RiskLevel: tools.RiskDirect, RequiresConfirmation: false}
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
	summary, boundary := plugingrabber.BuildCompressorSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_compressor"
		}
		err = fmt.Errorf("%s: no provable supported broadband compressor stage/control-path topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachCompressorControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"] = "ok"
	summary["track_id"] = target.TrackID
	summary["plugin_id"] = target.PluginID
	out.Result = summary
	return out, nil
}

func attachCompressorControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	stage := mapValue(summary["compressor_stage"])
	stageKey := firstNonEmptyText(stage, "stage_key")
	if generation == "" || stageKey == "" {
		return fmt.Errorf("compressor topology omitted generation or stage key")
	}
	for _, path := range mapRowsValue(stage["control_paths"]) {
		pathKey := firstNonEmptyText(path, "path_key")
		for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
			if err := attachCompressorBindingRefs(mapRowsValue(path[section]), compressorControlReference{Kind: compressorControlRefKind,
				TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, PathKey: pathKey, Section: section}); err != nil {
				return err
			}
		}
	}
	return attachCompressorBindingRefs(mapRowsValue(stage["output"]), compressorControlReference{Kind: compressorControlRefKind,
		TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, PathKey: "stage_output", Section: "output"})
}

func attachCompressorBindingRefs(rows []map[string]any, base compressorControlReference) error {
	for _, row := range rows {
		ref := base
		ref.Role = firstNonEmptyText(row, "role")
		ref.ParamID = firstNonEmptyText(row, "param_id")
		encoded, err := encodeCompressorControlRef(ref)
		if err != nil {
			return err
		}
		row["control_ref"] = encoded
	}
	return nil
}

func encodeCompressorControlRef(ref compressorControlReference) (string, error) {
	if ref.Kind != compressorControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" ||
		ref.StageKey == "" || ref.PathKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete compressor control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "c2cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeCompressorControlRef(value string) (compressorControlReference, error) {
	var ref compressorControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "c2cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: compressor control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "c2cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: compressor control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	sum := sha256.Sum256(data)
	if err != nil || parts[1] != hex.EncodeToString(sum[:8]) {
		return ref, fmt.Errorf("invalid_control_ref: compressor control_ref checksum is invalid")
	}
	if json.Unmarshal(data, &ref) != nil || ref.Kind != compressorControlRefKind {
		return compressorControlReference{}, fmt.Errorf("invalid_control_ref: compressor control_ref payload is invalid")
	}
	return ref, nil
}

func encodeCompressorRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := compressorRestoreReference{Kind: compressorRestoreRefKind, TrackID: trackID, PluginID: pluginID,
		TopologyGeneration: generation, Values: make([]compressorRestoreValue, 0, len(preimage))}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid compressor restore preimage")
		}
		ref.Values = append(ref.Values, compressorRestoreValue{ParamID: value.ParamID, Normalized: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete compressor restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "c2rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeCompressorRestoreRef(value string) (compressorRestoreReference, error) {
	var ref compressorRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "c2rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: compressor restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "c2rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: compressor restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: compressor restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil ||
		ref.Kind != compressorRestoreRefKind || ref.TrackID == "" || ref.PluginID == "" ||
		ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return compressorRestoreReference{}, fmt.Errorf("invalid_restore_ref: compressor restore_ref checksum or payload is invalid")
	}
	for _, item := range ref.Values {
		if item.ParamID == "" || math.IsNaN(item.Normalized) || math.IsInf(item.Normalized, 0) {
			return compressorRestoreReference{}, fmt.Errorf("invalid_restore_ref: compressor restore_ref contains an invalid value")
		}
	}
	return ref, nil
}
