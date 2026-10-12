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

// M2-TIMEFX-1 delay inspect face: reads the live plugin parameter surface
// through the same generic snapshot command every family uses, then projects
// the family's control topology with generation-scoped control_ref values for
// the typed apply surface and the local certification line.

const (
	pluginGrabberInspectDelayCommand = "plugin_grabber_inspect_delay"
	pluginGrabberInspectDelayTool    = "plugin_grabber.inspect_delay"
	delayControlRefKind              = "delay_control_ref.v1"
	delayRestoreRefKind              = "delay_restore_ref.v1"
)

type delayControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type delayRestoreValue struct {
	ParamID    string  `json:"param_id"`
	Normalized float64 `json:"normalized"`
	Role       string  `json:"role"`
}

type delayRestoreReference struct {
	Kind               string              `json:"kind"`
	TrackID            string              `json:"track_id"`
	PluginID           string              `json:"plugin_id"`
	TopologyGeneration string              `json:"generation"`
	Values             []delayRestoreValue `json:"values"`
}

func pluginGrabberInspectDelayInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectDelayTool && name != pluginGrabberInspectDelayCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectDelayCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectDelayCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectDelayWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectDelayTool, CommandName: pluginGrabberInspectDelayCommand, RiskLevel: tools.RiskDirect}
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
	summary, boundary := plugingrabber.BuildDelaySummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_delay"
		}
		err = fmt.Errorf("%s: no provable supported delay stage/control-path topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachDelayControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"] = "ok"
	summary["track_id"] = target.TrackID
	summary["plugin_id"] = target.PluginID
	out.Result = summary
	return out, nil
}

func attachDelayControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return fmt.Errorf("delay topology omitted generation")
	}
	stage := mapValue(summary["delay_stage"])
	stageKey := firstNonEmptyText(stage, "stage_key")
	if stageKey == "" {
		return fmt.Errorf("delay topology omitted stage key")
	}
	for _, section := range []string{"timing", "feedback", "character", "output"} {
		for _, row := range mapRowsValue(stage[section]) {
			if err := attachDelayBindingRef(row, delayControlReference{Kind: delayControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, StageKey: stageKey, Section: section}); err != nil {
				return err
			}
		}
	}
	return nil
}

func attachDelayBindingRef(row map[string]any, base delayControlReference) error {
	base.Role = firstNonEmptyText(row, "role")
	base.ParamID = firstNonEmptyText(row, "param_id")
	if base.Role == "" || base.ParamID == "" {
		return fmt.Errorf("incomplete delay binding")
	}
	encoded, err := encodeDelayControlRef(base)
	if err != nil {
		return err
	}
	row["control_ref"] = encoded
	return nil
}

func encodeDelayControlRef(ref delayControlReference) (string, error) {
	if ref.Kind != delayControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || ref.StageKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete delay control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "dlcr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeDelayControlRef(value string) (delayControlReference, error) {
	var ref delayControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "dlcr1_") {
		return ref, fmt.Errorf("invalid_control_ref: delay control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "dlcr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: delay control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: delay control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != delayControlRefKind {
		return delayControlReference{}, fmt.Errorf("invalid_control_ref: delay control_ref checksum or payload is invalid")
	}
	return ref, nil
}

func encodeDelayRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := delayRestoreReference{Kind: delayRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid delay restore preimage")
		}
		ref.Values = append(ref.Values, delayRestoreValue{ParamID: value.ParamID, Normalized: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete delay restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "dlrr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeDelayRestoreRef(value string) (delayRestoreReference, error) {
	var ref delayRestoreReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "dlrr1_") {
		return ref, fmt.Errorf("invalid_restore_ref: delay restore_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "dlrr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_restore_ref: delay restore_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_restore_ref: delay restore_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != delayRestoreRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return delayRestoreReference{}, fmt.Errorf("invalid_restore_ref: delay restore_ref checksum or payload is invalid")
	}
	for _, item := range ref.Values {
		if item.ParamID == "" || math.IsNaN(item.Normalized) || math.IsInf(item.Normalized, 0) {
			return delayRestoreReference{}, fmt.Errorf("invalid_restore_ref: delay restore_ref contains an invalid value")
		}
	}
	return ref, nil
}
