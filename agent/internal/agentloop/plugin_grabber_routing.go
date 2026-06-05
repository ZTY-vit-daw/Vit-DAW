package agentloop

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"vit-daw-agent/internal/planner"
)

const pluginGrabberLearnToolName = "plugin_grabber.learn_project_profile"
const pluginGrabberApplyToolName = "plugin_grabber.apply_control"

var (
	frequencyPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(k?hz)\b`)
	gainPattern      = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*dB\b`)
	percentPattern   = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:%|percent|pct)\b?`)
	timePattern      = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(ms|msec|milliseconds?|s|sec|seconds?)\b`)
	qPattern         = regexp.MustCompile(`(?i)\bq\s*=?\s*(\d+(?:\.\d+)?)\b`)
)

func coercePluginGrabberLearningToolCall(userText string, call planner.ToolCall) planner.ToolCall {
	if !looksLikePluginGrabberLearningGoal(userText) || !isPluginParameterReadTool(call) {
		return call
	}
	args := cloneToolArgs(call.Args)
	for key, value := range call.Command {
		if _, exists := args[key]; !exists {
			args[key] = value
		}
	}
	if strings.TrimSpace(fmt.Sprint(args["intent"])) == "" || strings.TrimSpace(fmt.Sprint(args["intent"])) == "<nil>" {
		args["intent"] = strings.TrimSpace(userText)
	}
	call.Tool = pluginGrabberLearnToolName
	call.Args = args
	call.Command = nil
	if strings.TrimSpace(call.Reason) == "" {
		call.Reason = "learn plugin grabber controls instead of only reading raw parameters"
	}
	return call
}

func coercePluginGrabberRuntimeToolCall(userText string, call planner.ToolCall) planner.ToolCall {
	if isPluginGrabberApplyToolCall(call) {
		return normalizePluginGrabberApplyControlToolCall(userText, call)
	}
	if !looksLikePluginGrabberRuntimeGoal(userText) || !isPluginParameterWriteTool(call) {
		return call
	}

	args := cloneToolArgs(call.Args)
	for key, value := range call.Command {
		if _, exists := args[key]; !exists {
			args[key] = value
		}
	}
	if strings.TrimSpace(fmt.Sprint(args["track_id"])) == "" || strings.TrimSpace(fmt.Sprint(args["plugin_id"])) == "" {
		return call
	}
	if strings.TrimSpace(fmt.Sprint(args["param_id"])) != "" {
		if !looksLikeAcousticParamSpeculation(userText) {
			return call
		}
	}

	target := map[string]any{}
	if freq, ok := inferRuntimeFrequencyHz(userText); ok {
		target["freq_hz"] = freq
	}
	if gain, ok := inferRuntimeGainDb(userText); ok {
		target["gain_db"] = gain
	}
	if q, ok := inferRuntimeQ(userText); ok {
		target["q"] = q
	}
	if displayAmount, ok := inferRuntimeDisplayAmount(userText); ok {
		target["amount"] = displayAmount
	}
	if amount := inferRuntimeAmount(userText); amount != "" && emptyArgValue(target["amount"]) {
		target["amount"] = amount
	}
	if len(target) == 0 {
		return call
	}

	outArgs := map[string]any{
		"track_id":  args["track_id"],
		"plugin_id": args["plugin_id"],
		"control":   inferRuntimeControlName(userText),
		"target":    target,
	}
	if isRelativeDisplayDeltaRequest(userText) && !strings.HasPrefix(fmt.Sprint(outArgs["control"]), "eq.") {
		outArgs["value_mode"] = "relative_delta"
	}
	call.Tool = pluginGrabberApplyToolName
	call.Args = outArgs
	call.Command = nil
	if strings.TrimSpace(call.Reason) == "" {
		call.Reason = "apply a learned plugin grabber runtime control from the acoustic target instead of writing a speculative raw param_id"
	}
	return call
}

func isPluginGrabberApplyToolCall(call planner.ToolCall) bool {
	tool := strings.TrimSpace(call.Tool)
	if tool == pluginGrabberApplyToolName || tool == "plugin_grabber_apply_control" {
		return true
	}
	name := strings.TrimSpace(fmt.Sprint(call.Command["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(call.Command["command"]))
	}
	return name == "plugin_grabber_apply_control"
}

func normalizePluginGrabberApplyControlToolCall(userText string, call planner.ToolCall) planner.ToolCall {
	args := cloneToolArgs(call.Args)
	for key, value := range call.Command {
		if _, exists := args[key]; !exists {
			args[key] = value
		}
	}
	target := mapArgValue(args["target"])
	if target == nil {
		target = map[string]any{}
	}
	if freq, ok := inferRuntimeFrequencyHz(userText); ok && emptyArgValue(target["freq_hz"]) {
		target["freq_hz"] = freq
	}
	if gain, ok := inferRuntimeGainDb(userText); ok && emptyArgValue(target["gain_db"]) {
		target["gain_db"] = gain
	}
	if q, ok := inferRuntimeQ(userText); ok && emptyArgValue(target["q"]) {
		target["q"] = q
	}
	if displayAmount, ok := inferRuntimeDisplayAmount(userText); ok && emptyArgValue(target["amount"]) {
		target["amount"] = displayAmount
	}
	if amount := inferRuntimeAmount(userText); amount != "" && emptyArgValue(target["amount"]) {
		target["amount"] = amount
	}
	if len(target) > 0 {
		args["target"] = target
	}
	control := strings.TrimSpace(fmt.Sprint(args["control"]))
	if control == "" || control == "<nil>" {
		control = strings.TrimSpace(fmt.Sprint(args["operation"]))
	}
	if control == "" || control == "<nil>" || shouldReplaceDefaultEqControl(userText, control) {
		control = inferRuntimeControlName(userText)
		args["control"] = control
	}
	if isRelativeDisplayDeltaRequest(userText) && !strings.HasPrefix(strings.ToLower(control), "eq.") {
		args["value_mode"] = "relative_delta"
	}
	call.Tool = pluginGrabberApplyToolName
	call.Args = args
	call.Command = nil
	return call
}

func looksLikePluginGrabberLearningGoal(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if looksLikePluginGrabberProfileReadGoal(text) {
		return false
	}
	learn := strings.Contains(text, "学习") || strings.Contains(text, "learn") || strings.Contains(text, "建立配置") || strings.Contains(text, "配置文件")
	pluginControl := strings.Contains(text, "插件") || strings.Contains(text, "plugin") || strings.Contains(text, "常用控制") || strings.Contains(text, "quick control") || strings.Contains(text, "参数控制")
	return learn && pluginControl
}

func looksLikePluginGrabberProfileReadGoal(text string) bool {
	readIntent := strings.Contains(text, "有哪些") || strings.Contains(text, "有什么") || strings.Contains(text, "查询") || strings.Contains(text, "查看") || strings.Contains(text, "显示") || strings.Contains(text, "列出") || strings.Contains(text, "知道") || strings.Contains(text, "read") || strings.Contains(text, "show") || strings.Contains(text, "list")
	profileObject := strings.Contains(text, "已学习") || strings.Contains(text, "已经学习") || strings.Contains(text, "保存") || strings.Contains(text, "profile") || strings.Contains(text, "配置") || strings.Contains(text, "常用控制")
	return readIntent && profileObject
}

func isPluginParameterReadTool(call planner.ToolCall) bool {
	tool := strings.TrimSpace(call.Tool)
	if tool == "plugin.get_parameters" || tool == "plugin_get_parameters" || tool == "get_plugin_parameters" || tool == "plugin_grabber.explain_controls" || tool == "plugin_grabber_explain_controls" {
		return true
	}
	name := strings.TrimSpace(fmt.Sprint(call.Command["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(call.Command["command"]))
	}
	return name == "get_plugin_parameters" || name == "plugin_grabber_explain_controls"
}

func isPluginParameterWriteTool(call planner.ToolCall) bool {
	tool := strings.TrimSpace(call.Tool)
	if tool == "plugin.set_parameter" || tool == "plugin_set_parameter" || tool == "set_plugin_param" {
		return true
	}
	name := strings.TrimSpace(fmt.Sprint(call.Command["cmd"]))
	if name == "" || name == "<nil>" {
		name = strings.TrimSpace(fmt.Sprint(call.Command["command"]))
	}
	return name == "set_plugin_param"
}

func looksLikePluginGrabberRuntimeGoal(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if strings.Contains(text, "param_id") || strings.Contains(text, "参数 id") || strings.Contains(text, "参数id") {
		return false
	}
	hasAcousticIntent := strings.Contains(text, "hz") || strings.Contains(text, "khz") ||
		strings.Contains(text, "db") || textLooksLikeGainReduction(text) || textLooksLikeGainBoost(text) ||
		strings.Contains(text, "eq") || strings.Contains(text, "mud") || strings.Contains(text, "harsh") ||
		strings.Contains(text, "presence") || strings.Contains(text, "boomy") || strings.Contains(text, "boxy") ||
		strings.Contains(text, "delay") || strings.Contains(text, "echo") || strings.Contains(text, "reverb") ||
		strings.Contains(text, "ms") || strings.Contains(text, "msec") ||
		strings.Contains(text, "浑") || strings.Contains(text, "糊") || strings.Contains(text, "闷") ||
		strings.Contains(text, "刺") || strings.Contains(text, "削") || strings.Contains(text, "提亮") ||
		strings.Contains(text, "低频") || strings.Contains(text, "中频") || strings.Contains(text, "高频") ||
		strings.Contains(text, "延迟") || strings.Contains(text, "回声") || strings.Contains(text, "混响") || strings.Contains(text, "毫秒")
	hasPluginIntent := strings.Contains(text, "插件") || strings.Contains(text, "控制") || strings.Contains(text, "增益") ||
		strings.Contains(text, "gain") || strings.Contains(text, "plugin") ||
		strings.Contains(text, "eq") || strings.Contains(text, "混音") || strings.Contains(text, "mix") ||
		strings.Contains(text, "delay") || strings.Contains(text, "echo") || strings.Contains(text, "reverb") ||
		strings.Contains(text, "延迟") || strings.Contains(text, "回声") || strings.Contains(text, "混响")
	return hasAcousticIntent && hasPluginIntent
}

func looksLikeAcousticParamSpeculation(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	return !strings.Contains(text, "param_id") &&
		(strings.Contains(text, "hz") || strings.Contains(text, "khz") || strings.Contains(text, "mud") ||
			strings.Contains(text, "ms") || strings.Contains(text, "msec") || strings.Contains(text, "delay") || strings.Contains(text, "echo") ||
			strings.Contains(text, "浑") || strings.Contains(text, "刺") || strings.Contains(text, "presence") ||
			strings.Contains(text, "eq") || strings.Contains(text, "削") || strings.Contains(text, "提亮") ||
			strings.Contains(text, "延迟") || strings.Contains(text, "回声") || strings.Contains(text, "毫秒"))
}

func inferRuntimeFrequencyHz(userText string) (float64, bool) {
	text := strings.ToLower(userText)
	if match := frequencyPattern.FindStringSubmatch(text); len(match) == 3 {
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil && value > 0 {
			if strings.EqualFold(match[2], "khz") {
				value *= 1000
			}
			return value, true
		}
	}
	switch {
	case strings.Contains(text, "mud") || strings.Contains(text, "浑") || strings.Contains(text, "糊") || strings.Contains(text, "boxy"):
		return 500, true
	case strings.Contains(text, "boomy") || strings.Contains(text, "轰") || strings.Contains(text, "低频"):
		return 160, true
	case strings.Contains(text, "harsh") || strings.Contains(text, "刺"):
		return 3000, true
	case strings.Contains(text, "presence") || strings.Contains(text, "存在感") || strings.Contains(text, "提亮"):
		return 4000, true
	}
	return 0, false
}

func inferRuntimeGainDb(userText string) (float64, bool) {
	text := strings.ToLower(userText)
	if match := gainPattern.FindStringSubmatch(text); len(match) == 2 {
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			if textLooksLikeGainReduction(text) {
				value = -absFloat(value)
			} else if textLooksLikeGainBoost(text) {
				value = absFloat(value)
			}
			return value, true
		}
	}
	if textLooksLikeGainBoost(text) {
		return defaultGainMagnitude(text), true
	}
	if textLooksLikeGainReduction(text) {
		return -defaultGainMagnitude(text), true
	}
	if strings.Contains(text, "boost") || strings.Contains(text, "提亮") || strings.Contains(text, "增加") || strings.Contains(text, "加一点") {
		return defaultGainMagnitude(text), true
	}
	if strings.Contains(text, "cut") || strings.Contains(text, "reduce") || strings.Contains(text, "削") || strings.Contains(text, "减少") ||
		strings.Contains(text, "mud") || strings.Contains(text, "浑") || strings.Contains(text, "刺") || strings.Contains(text, "闷") {
		return -defaultGainMagnitude(text), true
	}
	return 0, false
}

func inferRuntimeQ(userText string) (float64, bool) {
	text := strings.ToLower(userText)
	if match := qPattern.FindStringSubmatch(text); len(match) == 2 {
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil && value > 0 {
			return value, true
		}
	}
	switch {
	case strings.Contains(text, "narrow") || strings.Contains(text, "窄"):
		return 2.4, true
	case strings.Contains(text, "wide") || strings.Contains(text, "宽"):
		return 0.7, true
	}
	return 0, false
}

func inferRuntimeDisplayAmount(userText string) (float64, bool) {
	text := strings.ToLower(userText)
	if match := percentPattern.FindStringSubmatch(text); len(match) == 2 {
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			return value, true
		}
	}
	if match := timePattern.FindStringSubmatch(text); len(match) == 3 {
		value, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			unit := strings.ToLower(match[2])
			if unit == "s" || unit == "sec" || strings.HasPrefix(unit, "second") {
				value *= 1000
			}
			return value, true
		}
	}
	return 0, false
}

func inferRuntimeAmount(userText string) string {
	text := strings.ToLower(userText)
	switch {
	case strings.Contains(text, "strong") || strings.Contains(text, "heavy") || strings.Contains(text, "明显") || strings.Contains(text, "重"):
		return "strong"
	case strings.Contains(text, "subtle") || strings.Contains(text, "gentle") || strings.Contains(text, "gently") || strings.Contains(text, "light") || strings.Contains(text, "轻") || strings.Contains(text, "一点"):
		return "gentle"
	case strings.Contains(text, "medium") || strings.Contains(text, "moderate") || strings.Contains(text, "适中"):
		return "medium"
	}
	return ""
}

func defaultGainMagnitude(text string) float64 {
	switch inferRuntimeAmount(text) {
	case "strong":
		return 6
	case "medium":
		return 4
	case "gentle":
		return 2.5
	default:
		return 2.5
	}
}

func inferRuntimeControlName(userText string) string {
	text := strings.ToLower(userText)
	if looksLikeDelayRuntimeText(text) {
		return "Echo Length"
	}
	if strings.Contains(text, "boost") || strings.Contains(text, "提亮") || strings.Contains(text, "增加") || strings.Contains(text, "presence") {
		return "eq.boost_region"
	}
	if strings.Contains(text, "cut") || strings.Contains(text, "reduce") || strings.Contains(text, "削") || strings.Contains(text, "减少") ||
		strings.Contains(text, "mud") || strings.Contains(text, "浑") || strings.Contains(text, "刺") || strings.Contains(text, "闷") {
		return "eq.cut_region"
	}
	return "eq.set_region"
}

func shouldReplaceDefaultEqControl(userText, control string) bool {
	clean := strings.ToLower(strings.TrimSpace(control))
	return clean == "eq.set_region" && looksLikeDelayRuntimeText(strings.ToLower(userText))
}

func looksLikeDelayRuntimeText(text string) bool {
	return strings.Contains(text, "delay") ||
		strings.Contains(text, "echo") ||
		strings.Contains(text, "reverb") ||
		strings.Contains(text, "ms") ||
		strings.Contains(text, "msec") ||
		strings.Contains(text, "延迟") ||
		strings.Contains(text, "回声") ||
		strings.Contains(text, "混响") ||
		strings.Contains(text, "毫秒")
}

func textLooksLikeGainReduction(text string) bool {
	return strings.Contains(text, "cut") || strings.Contains(text, "reduce") || strings.Contains(text, "decrease") ||
		strings.Contains(text, "lower") || strings.Contains(text, "attenuate") || strings.Contains(text, "降低") ||
		strings.Contains(text, "减少") || strings.Contains(text, "衰减") || strings.Contains(text, "调低")
}

func textLooksLikeGainBoost(text string) bool {
	return strings.Contains(text, "boost") || strings.Contains(text, "increase") || strings.Contains(text, "raise") ||
		strings.Contains(text, "提升") || strings.Contains(text, "增加") || strings.Contains(text, "调高")
}

func isRelativeDisplayDeltaRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if strings.Contains(text, "设为") || strings.Contains(text, "设置为") || strings.Contains(text, "改成") ||
		strings.Contains(text, "set to") || strings.Contains(text, "set ") {
		return false
	}
	return textLooksLikeGainReduction(text) || textLooksLikeGainBoost(text)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func emptyArgValue(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}

func mapArgValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneToolArgs(typed)
	default:
		return nil
	}
}

func cloneToolArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for key, value := range args {
		out[key] = value
	}
	return out
}
