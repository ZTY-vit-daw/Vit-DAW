package chat

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/tools"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberInspectSpectralDynamicsCommand = "plugin_grabber_inspect_spectral_dynamics"
	pluginGrabberInspectSpectralDynamicsTool    = "plugin_grabber.inspect_spectral_dynamics"
)

func pluginGrabberInspectSpectralDynamicsInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	name := strings.TrimSpace(req.Tool)
	if name != pluginGrabberInspectSpectralDynamicsTool && name != pluginGrabberInspectSpectralDynamicsCommand {
		args := workflowCommandArgs(req.Command)
		if firstNonEmptyText(args, "cmd", "command") != pluginGrabberInspectSpectralDynamicsCommand {
			return nil, false
		}
		return args, true
	}
	cmd := map[string]any{"cmd": pluginGrabberInspectSpectralDynamicsCommand}
	for key, value := range req.Args {
		cmd[key] = value
	}
	return cmd, true
}

func (s *Server) invokePluginGrabberInspectSpectralDynamicsWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	out := harness.InvokeResponse{Status: "ok", Tool: pluginGrabberInspectSpectralDynamicsTool, CommandName: pluginGrabberInspectSpectralDynamicsCommand, RiskLevel: tools.RiskDirect}
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
	summary, boundary := plugingrabber.BuildSpectralDynamicsSummaryWithBoundary(digest)
	if summary == nil {
		if boundary == "" {
			boundary = "not_spectral_dynamics"
		}
		err = fmt.Errorf("%s: no provable supported spectral-dynamics topology", boundary)
		out.Status, out.Error = "error", err.Error()
		out.Result = map[string]any{"status": "rejected", "code": boundary, "track_id": target.TrackID, "plugin_id": target.PluginID}
		return out, err
	}
	summary["status"], summary["track_id"], summary["plugin_id"] = "ok", target.TrackID, target.PluginID
	out.Result = summary
	return out, nil
}
