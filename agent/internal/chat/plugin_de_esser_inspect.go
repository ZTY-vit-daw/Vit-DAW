package chat

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberInspectDeEsserCommand = "plugin_grabber_inspect_de_esser"
	pluginGrabberInspectDeEsserTool    = "plugin_grabber.inspect_de_esser"
	deEsserControlRefKind              = "de_esser_control_ref.v1"
)

type deEsserControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	StageKey           string `json:"stage_key"`
	Section            string `json:"section"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

func pluginGrabberInspectDeEsserInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectDeEsserTool && name != pluginGrabberInspectDeEsserCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectDeEsserCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectDeEsserCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectDeEsserWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectDeEsserTool,
		CommandName: pluginGrabberInspectDeEsserCommand, RiskLevel: tools.RiskDirect}
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
	reply, _, err := s.eqKernelClient().SendCommand(ctx, map[string]any{
		"cmd": "get_plugin_parameters", "track_id": target.TrackID, "plugin_id": target.PluginID, "include_parameters": true,
	})
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
	summary, boundary := plugingrabber.BuildDeEsserSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_de_esser"
		}
		err = fmt.Errorf("%s: no provable supported De-esser stage/control-path topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachDeEsserControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"] = "ok"
	summary["track_id"] = target.TrackID
	summary["plugin_id"] = target.PluginID
	out.Result = summary
	return out, nil
}

func attachDeEsserControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return fmt.Errorf("De-esser topology omitted generation")
	}
	for _, stage := range mapRowsValue(summary["de_esser_stages"]) {
		stageKey := firstNonEmptyText(stage, "stage_key")
		if stageKey == "" {
			return fmt.Errorf("De-esser topology omitted stage key")
		}
		for _, section := range []string{"operating_point", "frequency_selectivity", "timing", "mode", "output"} {
			for _, row := range mapRowsValue(stage[section]) {
				role := firstNonEmptyText(row, "role")
				paramID := firstNonEmptyText(row, "param_id")
				if role == "" || paramID == "" {
					return fmt.Errorf("incomplete De-esser binding")
				}
				ref, err := encodeDeEsserControlRef(deEsserControlReference{
					Kind: deEsserControlRefKind, TrackID: trackID, PluginID: pluginID,
					TopologyGeneration: generation, StageKey: stageKey, Section: section,
					Role: role, ParamID: paramID,
				})
				if err != nil {
					return err
				}
				row["control_ref"] = ref
			}
		}
	}
	return nil
}

func encodeDeEsserControlRef(ref deEsserControlReference) (string, error) {
	if ref.Kind != deEsserControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" ||
		ref.StageKey == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete De-esser control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "d1cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeDeEsserControlRef(value string) (deEsserControlReference, error) {
	var ref deEsserControlReference
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "d1cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: De-esser control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(value, "d1cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: De-esser control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: De-esser control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != deEsserControlRefKind {
		return deEsserControlReference{}, fmt.Errorf("invalid_control_ref: De-esser control_ref checksum or payload is invalid")
	}
	return ref, nil
}
