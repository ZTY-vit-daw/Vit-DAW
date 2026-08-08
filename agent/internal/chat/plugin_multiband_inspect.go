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
	pluginGrabberInspectMultibandCommand = "plugin_grabber_inspect_multiband"
	pluginGrabberInspectMultibandTool    = "plugin_grabber.inspect_multiband"
	multibandControlRefKind              = "multiband_control_ref.v1"
	multibandRestoreRefKind              = "multiband_restore_ref.v1"
)

type multibandControlReference struct {
	Kind               string `json:"kind"`
	TrackID            string `json:"track_id"`
	PluginID           string `json:"plugin_id"`
	TopologyGeneration string `json:"generation"`
	Section            string `json:"section"`
	BandKey            string `json:"band_key,omitempty"`
	Role               string `json:"role"`
	ParamID            string `json:"param_id"`
}

type multibandRestoreValue struct {
	ParamID string  `json:"param_id"`
	Role    string  `json:"role"`
	Value   float64 `json:"normalized"`
}
type multibandRestoreReference struct {
	Kind               string                  `json:"kind"`
	TrackID            string                  `json:"track_id"`
	PluginID           string                  `json:"plugin_id"`
	TopologyGeneration string                  `json:"generation"`
	Values             []multibandRestoreValue `json:"values"`
}

func pluginGrabberInspectMultibandInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectMultibandTool && name != pluginGrabberInspectMultibandCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectMultibandCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectMultibandCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectMultibandWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectMultibandTool, CommandName: pluginGrabberInspectMultibandCommand, RiskLevel: tools.RiskDirect}
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
	summary, boundary := plugingrabber.BuildMultibandSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_multiband_dynamics"
		}
		err = fmt.Errorf("%s: no provable supported multiband dynamics topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	if err = attachMultibandControlRefs(summary, target.TrackID, target.PluginID); err != nil {
		out.Status, out.Error = "error", err.Error()
		return out, err
	}
	summary["status"], summary["track_id"], summary["plugin_id"] = "ok", target.TrackID, target.PluginID
	out.Result = summary
	return out, nil
}

func attachMultibandControlRefs(summary map[string]any, trackID, pluginID string) error {
	generation := firstNonEmptyText(mapValue(summary["control_topology"]), "generation")
	if generation == "" {
		return fmt.Errorf("multiband topology omitted generation")
	}
	for _, row := range mapRowsValue(mapValue(summary["filterbank"])["crossovers"]) {
		if err := attachMultibandBindingRef(row, multibandControlReference{Kind: multibandControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, Section: "filterbank", BandKey: "shared"}); err != nil {
			return err
		}
	}
	for _, band := range mapRowsValue(summary["band_cells"]) {
		key := firstNonEmptyText(band, "band_key")
		if key == "" {
			return fmt.Errorf("multiband topology omitted band key")
		}
		for _, section := range []string{"operating_point", "transfer", "timing", "gain_action", "mode"} {
			for _, row := range mapRowsValue(band[section]) {
				if err := attachMultibandBindingRef(row, multibandControlReference{Kind: multibandControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, Section: section, BandKey: key}); err != nil {
					return err
				}
			}
		}
	}
	for _, row := range mapRowsValue(summary["shared_controls"]) {
		if err := attachMultibandBindingRef(row, multibandControlReference{Kind: multibandControlRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation, Section: "shared", BandKey: "shared"}); err != nil {
			return err
		}
	}
	return nil
}

func attachMultibandBindingRef(row map[string]any, base multibandControlReference) error {
	base.Role, base.ParamID = firstNonEmptyText(row, "role"), firstNonEmptyText(row, "param_id")
	if base.Role == "" || base.ParamID == "" {
		return fmt.Errorf("incomplete multiband binding")
	}
	encoded, err := encodeMultibandControlRef(base)
	if err != nil {
		return err
	}
	row["control_ref"] = encoded
	return nil
}

func encodeMultibandControlRef(ref multibandControlReference) (string, error) {
	if ref.Kind != multibandControlRefKind || ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || ref.Section == "" || ref.Role == "" || ref.ParamID == "" {
		return "", fmt.Errorf("incomplete multiband control reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "mb1cr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}

func decodeMultibandControlRef(value string) (multibandControlReference, error) {
	var ref multibandControlReference
	if !strings.HasPrefix(strings.TrimSpace(value), "mb1cr1_") {
		return ref, fmt.Errorf("invalid_control_ref: multiband control_ref prefix is invalid")
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "mb1cr1_"), ".")
	if len(parts) != 2 {
		return ref, fmt.Errorf("invalid_control_ref: multiband control_ref framing is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ref, fmt.Errorf("invalid_control_ref: multiband control_ref payload is invalid")
	}
	sum := sha256.Sum256(data)
	if parts[1] != hex.EncodeToString(sum[:8]) || json.Unmarshal(data, &ref) != nil || ref.Kind != multibandControlRefKind {
		return multibandControlReference{}, fmt.Errorf("invalid_control_ref: multiband control_ref checksum or payload is invalid")
	}
	return ref, nil
}

func encodeMultibandRestoreRef(trackID, pluginID, generation string, preimage []eqPreimageValue) (string, error) {
	ref := multibandRestoreReference{Kind: multibandRestoreRefKind, TrackID: trackID, PluginID: pluginID, TopologyGeneration: generation}
	for _, value := range preimage {
		if value.ParamID == "" || math.IsNaN(value.Normalized) || math.IsInf(value.Normalized, 0) {
			return "", fmt.Errorf("invalid multiband restore preimage")
		}
		ref.Values = append(ref.Values, multibandRestoreValue{ParamID: value.ParamID, Value: value.Normalized, Role: value.Role})
	}
	if ref.TrackID == "" || ref.PluginID == "" || ref.TopologyGeneration == "" || len(ref.Values) == 0 {
		return "", fmt.Errorf("incomplete multiband restore reference")
	}
	data, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "mb1rr1_" + base64.RawURLEncoding.EncodeToString(data) + "." + hex.EncodeToString(sum[:8]), nil
}
