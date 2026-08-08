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
	pluginGrabberInspectLimiterCommand = "plugin_grabber_inspect_limiter"
	pluginGrabberInspectLimiterTool    = "plugin_grabber.inspect_limiter"
	limiterControlRefKind              = "limiter_control_ref.v1"
	limiterRestoreRefKind              = "limiter_restore_ref.v1"
)

type limiterControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type limiterRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role"`
}
type limiterRestoreReference struct {
	Kind               string                `json:"kind"`
	TrackID            string                `json:"track_id"`
	PluginID           string                `json:"plugin_id"`
	TopologyGeneration string                `json:"generation"`
	Values             []limiterRestoreValue `json:"values"`
}

func pluginGrabberInspectLimiterInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectLimiterTool && name != pluginGrabberInspectLimiterCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectLimiterCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectLimiterCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectLimiterWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectLimiterTool, CommandName: pluginGrabberInspectLimiterCommand, RiskLevel: tools.RiskDirect}
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
	summary, boundary := plugingrabber.BuildLimiterSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_limiter"
		}
		err = fmt.Errorf("%s: no provable supported limiter stage/control-path topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachLimiterControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"] = "ok"
	summary["track_id"] = target.TrackID
	summary["plugin_id"] = target.PluginID
	out.Result = summary
	return out, nil
}

func attachLimiterControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return fmt.Errorf("limiter topology omitted generation")
	}
	for _, stage := range mapRowsValue(summary["limiter_stages"]) {
		stageKey := firstNonEmptyText(stage, "stage_key")
		if stageKey == "" {
			return fmt.Errorf("limiter topology omitted stage key")
		}
		for _, section := range []string{"operating_point", "safety", "timing", "detector", "mode", "output"} {
			for _, row := range mapRowsValue(stage[section]) {
				if err := attachLimiterBindingRef(row, limiterControlReference{Kind: limiterControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, Section: section}); err != nil {
					return err
				}
			}
		}
	}
	for _, row := range mapRowsValue(summary["shared_controls"]) {
		if err := attachLimiterBindingRef(row, limiterControlReference{Kind: limiterControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: "shared", Section: "shared"}); err != nil {
			return err
		}
	}
	return nil
}

func attachLimiterBindingRef(row map[string]any, base limiterControlReference) error {
	base.Role = firstNonEmptyText(row, "role")
	base.ParamID = firstNonEmptyText(row, "param_id")
	if base.Role == "" || base.ParamID == "" {
		return fmt.Errorf("incomplete limiter binding")
	}
	encoded, err := encodeLimiterControlRef(base)
	if err != nil {
		return err
	}
	row["control_ref"] = encoded
	return nil
}

func encodeLimiterControlRef(ref limiterControlReference) (string, error) {
	if ref.Kind != limiterControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || ref.StageKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete limiter control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "l1cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeLimiterControlRef(value string) (limiterControlReference, error) {
	var ref limiterControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "l1cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: limiter control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "l1cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: limiter control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: limiter control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != limiterControlRefKind {
		return limiterControlReference{}, fmt.Errorf("invalid_control_ref: limiter control_ref checksum or payload is invalid")
	}
	return ref, nil
}

func encodeLimiterRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := limiterRestoreReference{Kind: limiterRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid limiter restore preimage")
		}
		ref.Values = append(ref.Values, limiterRestoreValue{ParamID: value.ParamID, Normalized: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete limiter restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "l1rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeLimiterRestoreRef(value string) (limiterRestoreReference, error) {
	var ref limiterRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "l1rr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: limiter restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "l1rr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: limiter restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: limiter restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != limiterRestoreRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return limiterRestoreReference{}, fmt.Errorf("invalid_restore_ref: limiter restore_ref checksum or payload is invalid")
	}
	for _, item := range ref.Values {
		if item.ParamID == "" || math.IsNaN(item.Normalized) || math.IsInf(item.Normalized, 0) {
			return limiterRestoreReference{}, fmt.Errorf("invalid_restore_ref: limiter restore_ref contains an invalid value")
		}
	}
	return ref, nil
}
