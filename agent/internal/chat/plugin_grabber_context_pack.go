package chat

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
	"vit-daw-agent/internal/tools"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

const (
	pluginGrabberExplainCommand = "plugin_grabber_explain_controls"
	pluginGrabberExplainTool    = "plugin_grabber.explain_controls"
)

func synthesizePluginGrabberExplainCommands(userText string, requestContext map[string]any) []map[string]any {
	return plugingrabber.SynthesizeExplainCommands(userText, requestContext)
}

func coercePluginGrabberExplainCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	if _, hasLearn := firstPluginGrabberLearningCommand(commands); hasLearn {
		return nil, false
	}
	return plugingrabber.CoerceExplainCommand(commands, userText, requestContext)
}

func firstPluginGrabberExplainCommand(commands []map[string]any) (map[string]any, bool) {
	return plugingrabber.FirstExplainCommand(commands)
}
func pluginGrabberExplainInvokeCommand(req harness.InvokeRequest) (map[string]any, bool) {
	toolName := strings.TrimSpace(req.Tool)
	if toolName == pluginGrabberExplainTool || toolName == pluginGrabberExplainCommand || toolName == "plugin.explain_controls" || toolName == "plugin_explain_controls" {
		cmd := map[string]any{"cmd": pluginGrabberExplainCommand}
		for key, value := range req.Args {
			cmd[key] = value
		}
		return cmd, true
	}
	args := workflowCommandArgs(req.Command)
	name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(args["command"]))
	}
	if name == pluginGrabberExplainCommand {
		return args, true
	}
	return nil, false
}

func looksLikePluginGrabberExplainIntent(userText string) bool {
	return plugingrabber.LooksLikeExplainIntent(userText)
}
func (s *Server) runPluginGrabberExplainWorkflow(ctx context.Context, conversationID, userText string, requestContext map[string]any, workflowCmd map[string]any) ChatResponse {
	target, err := s.resolvePluginLearningTarget(ctx, workflowCmd, requestContext, userText)
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if s.kernel == nil {
		err := fmt.Errorf("kernel client is nil")
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	paramsReply, _, err := s.kernel.SendCommand(ctx, map[string]any{
		"cmd":       "get_plugin_parameters",
		"track_id":  target.TrackID,
		"plugin_id": target.PluginID,
	})
	if err != nil {
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}
	if !kernelReplyOK(paramsReply) {
		message := firstNonEmptyText(paramsReply, "message", "error")
		if message == "" {
			message = "get_plugin_parameters failed"
		}
		err := fmt.Errorf("%s", message)
		return ChatResponse{ConversationID: conversationID, Reply: friendlyExecutionError(err), Error: err.Error()}
	}

	s.observePluginParametersReply(paramsReply)
	digest := buildPluginParameterDigest(paramsReply)
	pack := buildPluginGrabberContextPack(digest)
	decision := policy.Decision{
		Name:    pluginGrabberExplainCommand,
		Risk:    policy.RiskDirect,
		Command: workflowCmd,
		Reason:  "read-only plugin grabber context pack",
	}
	return ChatResponse{
		ConversationID: conversationID,
		Reply:          formatPluginGrabberContextPackReply(pack),
		Commands:       []policy.Decision{decision},
		ExecutedKernelReply: []map[string]any{
			{
				"status":       "ok",
				"command_name": pluginGrabberExplainCommand,
				"result":       pack,
			},
		},
	}
}

func (s *Server) invokePluginGrabberExplainWorkflow(ctx context.Context, req harness.InvokeRequest, workflowCmd map[string]any) (harness.InvokeResponse, error) {
	intent := firstNonEmptyText(workflowCmd, "intent", "user_intent")
	resp := s.runPluginGrabberExplainWorkflow(ctx, "invoke_"+randomID(), intent, req.Context, workflowCmd)
	out := harness.InvokeResponse{
		Status:               "ok",
		Tool:                 pluginGrabberExplainTool,
		CommandName:          pluginGrabberExplainCommand,
		RiskLevel:            tools.RiskDirect,
		RequiresConfirmation: false,
	}
	if len(resp.ExecutedKernelReply) > 0 {
		if result, ok := resp.ExecutedKernelReply[0]["result"].(map[string]any); ok {
			out.Result = result
		}
	}
	if resp.Error != "" {
		out.Status = "error"
		out.Error = resp.Error
		return out, fmt.Errorf("%s", resp.Error)
	}
	return out, nil
}

func buildPluginGrabberContextPack(digest pluginParameterDigest) map[string]any {
	return plugingrabber.BuildContextPack(digest)
}

func formatPluginGrabberContextPackReply(pack map[string]any) string {
	return plugingrabber.FormatContextPackReply(pack)
}
