package plugingrabber

import (
	"fmt"
	"strings"
)

const (
	pluginGrabberExplainCommand = "plugin_grabber_explain_controls"
	pluginGrabberExplainTool    = "plugin_grabber.explain_controls"
)

func synthesizePluginGrabberExplainCommands(userText string, requestContext map[string]any) []map[string]any {
	if !looksLikePluginGrabberExplainIntent(userText) {
		return nil
	}
	cmd := map[string]any{
		"cmd":    pluginGrabberExplainCommand,
		"intent": userText,
	}
	for _, key := range []string{"selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_track_id", "track_id", "plugin_id"} {
		if value := strings.TrimSpace(fmt.Sprint(requestContext[key])); value != "" && value != "<nil>" {
			cmd[key] = value
		}
	}
	return []map[string]any{cmd}
}

func coercePluginGrabberExplainCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	if cmd, ok := firstPluginGrabberExplainCommand(commands); ok {
		return cmd, true
	}
	if !looksLikePluginGrabberExplainIntent(userText) {
		return nil, false
	}
	for _, cmd := range commands {
		args := workflowCommandArgs(cmd)
		name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
		if name == "" || name == "<nil>" {
			name = strings.TrimSpace(fmt.Sprint(args["command"]))
		}
		toolName := strings.TrimSpace(fmt.Sprint(args["tool"]))
		if name != "get_plugin_parameters" && toolName != "plugin.get_parameters" && toolName != "plugin_get_parameters" {
			continue
		}
		workflow := map[string]any{
			"cmd":    pluginGrabberExplainCommand,
			"intent": userText,
		}
		copyWorkflowField(workflow, args, "track_id", "track_id", "selected_plugin_track_id", "selected_track_id")
		copyWorkflowField(workflow, args, "plugin_id", "plugin_id", "selected_plugin_id", "plugin_item_id", "item_id")
		copyWorkflowField(workflow, args, "plugin_name", "plugin_name", "selected_plugin_name", "plugin", "name")
		copyWorkflowField(workflow, requestContext, "track_id", "selected_plugin_track_id", "selected_track_id", "track_id")
		copyWorkflowField(workflow, requestContext, "plugin_id", "selected_plugin_id", "plugin_id")
		copyWorkflowField(workflow, requestContext, "plugin_name", "selected_plugin_name", "plugin_name")
		return workflow, true
	}
	if len(commands) == 0 {
		cmds := synthesizePluginGrabberExplainCommands(userText, requestContext)
		if len(cmds) > 0 {
			return cmds[0], true
		}
	}
	return nil, false
}

func firstPluginGrabberExplainCommand(commands []map[string]any) (map[string]any, bool) {
	for _, cmd := range commands {
		name := strings.TrimSpace(fmt.Sprint(cmd["cmd"]))
		if name == "" || name == "<nil>" {
			name = strings.TrimSpace(fmt.Sprint(cmd["command"]))
		}
		toolName := strings.TrimSpace(fmt.Sprint(cmd["tool"]))
		if name == pluginGrabberExplainCommand || toolName == pluginGrabberExplainTool {
			return cmd, true
		}
	}
	return nil, false
}

func looksLikePluginGrabberExplainIntent(userText string) bool {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return false
	}
	hasExplain := strings.Contains(lower, "explain") ||
		strings.Contains(lower, "describe") ||
		strings.Contains(lower, "summary") ||
		strings.Contains(lower, "summarize") ||
		strings.Contains(lower, "context pack") ||
		strings.Contains(lower, "context") ||
		strings.Contains(userText, "解释") ||
		strings.Contains(userText, "说明") ||
		strings.Contains(userText, "总结") ||
		strings.Contains(userText, "摘要") ||
		strings.Contains(userText, "上下文")
	hasPluginSubject := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "vst") ||
		strings.Contains(lower, "parameter") ||
		strings.Contains(lower, "param") ||
		strings.Contains(lower, "control") ||
		strings.Contains(lower, "grabber") ||
		strings.Contains(userText, "插件") ||
		strings.Contains(userText, "参数") ||
		strings.Contains(userText, "控制") ||
		strings.Contains(userText, "抓手")
	return hasExplain && hasPluginSubject
}

func SynthesizeExplainCommands(userText string, requestContext map[string]any) []map[string]any {
	return synthesizePluginGrabberExplainCommands(userText, requestContext)
}

func CoerceExplainCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	return coercePluginGrabberExplainCommand(commands, userText, requestContext)
}

func FirstExplainCommand(commands []map[string]any) (map[string]any, bool) {
	return firstPluginGrabberExplainCommand(commands)
}

func LooksLikeExplainIntent(userText string) bool {
	return looksLikePluginGrabberExplainIntent(userText)
}
