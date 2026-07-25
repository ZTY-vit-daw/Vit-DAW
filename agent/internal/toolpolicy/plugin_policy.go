package toolpolicy

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type Verdict string

const (
	Abstain Verdict = "abstain"
	Allow   Verdict = "allow"
	Deny    Verdict = "deny"
)

type TurnContext struct {
	UserText             string
	KnownPluginNames     []string
	MutationBarrier      bool
	HasUsableObservation bool
	ObservedBeforeTurn   bool
}

type ToolCall struct {
	Name string
	Args map[string]any
}

type Decision struct {
	Verdict Verdict
	Rule    string
	Reason  string
}

type rule struct {
	name  string
	match func(TurnContext, ToolCall) bool
	apply func(TurnContext, ToolCall) Decision
}

var pluginMutationRules = []rule{
	{
		name: "mutation_barrier",
		match: func(ctx TurnContext, call ToolCall) bool {
			return ctx.MutationBarrier && IsPluginMutationTool(call.Name)
		},
		apply: func(TurnContext, ToolCall) Decision {
			return Decision{Verdict: Deny, Rule: "mutation_barrier", Reason: "read-only observation mutation barrier is active"}
		},
	},
	{
		name: "grabber_apply_requires_live_target_and_control",
		match: func(_ TurnContext, call ToolCall) bool {
			return IsPluginGrabberApplyTool(call.Name) && !CompleteGrabberApply(call.Args)
		},
		apply: func(TurnContext, ToolCall) Decision {
			return Decision{Verdict: Deny, Rule: "grabber_apply_requires_live_target_and_control", Reason: "plugin_grabber.apply_control requires track_id, plugin_id, and a learned control name"}
		},
	},
	{
		name: "observed_explicit_plugin_followup",
		match: func(ctx TurnContext, call ToolCall) bool {
			return IsPluginMutationTool(call.Name) && (ctx.HasUsableObservation || ctx.ObservedBeforeTurn) && ExplicitPluginRequest(ctx.UserText, ctx.KnownPluginNames)
		},
		apply: func(TurnContext, ToolCall) Decision {
			return Decision{Verdict: Allow, Rule: "observed_explicit_plugin_followup", Reason: "usable observation exists and this turn explicitly requests a plug-in action"}
		},
	},
	{
		name: "explicit_named_plugin_action",
		match: func(ctx TurnContext, call ToolCall) bool {
			return IsPluginMutationTool(call.Name) && ExplicitPluginRequest(ctx.UserText, ctx.KnownPluginNames)
		},
		apply: func(TurnContext, ToolCall) Decision {
			return Decision{Verdict: Allow, Rule: "explicit_named_plugin_action", Reason: "the user explicitly named or selected a plug-in action"}
		},
	},
}

func Decide(ctx TurnContext, call ToolCall) Decision {
	call.Name = normalizeToolName(call.Name)
	for _, candidate := range pluginMutationRules {
		if candidate.match(ctx, call) {
			return candidate.apply(ctx, call)
		}
	}
	return Decision{Verdict: Abstain, Rule: "no_plugin_policy_match"}
}

func ExplicitPluginRequest(userText string, knownPluginNames []string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasRawParameter := containsAny(text, "param_id", "parameter id", "参数 id", "参数id", "归一化", "normalized")
	if hasRawParameter {
		return true
	}
	hasVerb := containsAny(text,
		"加载", "挂载", "打开", "学习", "抓手", "插入", "新增", "设置参数", "写参数", "改参数", "调参数", "调整",
		"用", "使用", "通过", "切掉", "削掉", "提升", "降低", "应用", "执行",
		"load", "insert", "open", "learn", "grabber", "set parameter", "write parameter", "use", "apply", "through",
	)
	return hasVerb && MentionsPlugin(userText, knownPluginNames)
}

func MentionsPlugin(userText string, knownPluginNames []string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if containsAny(text,
		"插件", "效果器", "均衡器", "均衡", "压缩器", "混响", "延迟",
		"plugin", "vst", "eq", "compressor", "reverb", "delay", "tdr", "nova", "zl",
	) {
		return true
	}
	for _, name := range knownPluginNames {
		for _, alias := range pluginNameAliases(name) {
			if alias != "" && strings.Contains(normalizePluginText(text), alias) {
				return true
			}
		}
	}
	return false
}

func MentionsNamedPlugin(userText string, knownPluginNames []string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if containsAny(text, "tdr", "nova", "zl") {
		return true
	}
	normalizedText := normalizePluginText(text)
	for _, name := range knownPluginNames {
		for _, alias := range pluginNameAliases(name) {
			if alias != "" && strings.Contains(normalizedText, alias) {
				return true
			}
		}
	}
	return false
}

func LowMudNeedsObservation(userText string, knownPluginNames []string) bool {
	return lowMudIntent(userText) && !MentionsNamedPlugin(userText, knownPluginNames)
}

func CompleteGrabberApply(args map[string]any) bool {
	return argText(args, "track_id") != "" && argText(args, "plugin_id") != "" && firstArgText(args, "control", "operation", "name") != ""
}

func IsPluginGrabberApplyTool(name string) bool {
	switch normalizeToolName(name) {
	case "plugin_grabber.apply_control", "plugin_grabber_apply_control", "plugin_grabber.apply":
		return true
	default:
		return false
	}
}

func IsPluginMutationTool(name string) bool {
	switch normalizeToolName(name) {
	case "plugin.load_to_rack", "rack.add_node", "rack_add_node", "instantiate_plugin", "plugin.instantiate",
		"plugin_grabber.learn_project_profile", "plugin_grabber_learn_project_profile",
		"plugin_grabber.apply_control", "plugin_grabber_apply_control", "plugin_grabber.apply",
		"plugin.set_parameter", "plugin_set_parameter", "set_plugin_param":
		return true
	default:
		return false
	}
}

func CollectPluginNames(values ...any) []string {
	seen := map[string]string{}
	var walk func(any, string)
	walk = func(value any, parentKey string) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				cleanKey := strings.ToLower(strings.TrimSpace(key))
				if cleanKey == "plugin_name" || cleanKey == "last_loaded_plugin_name" || (cleanKey == "name" && strings.Contains(parentKey, "plugin")) {
					if text := strings.TrimSpace(fmt.Sprint(child)); text != "" && text != "<nil>" {
						seen[strings.ToLower(text)] = text
					}
				}
				walk(child, cleanKey)
			}
		case []any:
			for _, child := range typed {
				walk(child, parentKey)
			}
		case []map[string]any:
			for _, child := range typed {
				walk(child, parentKey)
			}
		}
	}
	for _, value := range values {
		walk(value, "")
	}
	out := make([]string, 0, len(seen))
	for _, name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func lowMudIntent(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasIssue := containsAny(text, "低频", "低中频", "浑浊", "糊", "low end", "low-end", "low mid", "low-mid", "mud", "muddy")
	hasEQ := containsAny(text, "eq", "均衡", "低切", "高通", "滤波", "low cut", "low-cut", "high pass", "high-pass", "filter")
	hasAction := containsAny(text, "帮我", "处理", "调整", "调一下", "收一点", "收低频", "收一下", "压低", "降低", "减少", "削减", "削一点", "切掉", "轻微", "稍微", "可以处理", "可以进行处理", "help me", "process", "treat", "adjust", "reduce", "lower", "cut", "trim", "slight", "slightly", "a little")
	hasPrep := containsAny(text, "先准备", "准备", "参数", "方案", "插件", "效果器", "prepare", "prep", "parameter", "parameters", "candidate", "plugin")
	hasEvidence := containsAny(text, "依据", "根据", "判断", "为什么", "为何", "先告诉", "先说", "evidence", "basis", "why", "before", "first tell")
	return hasIssue && (((hasEQ || hasPrep) && (hasPrep || hasAction || hasEvidence)) || hasAction)
}

func pluginNameAliases(name string) []string {
	normalized := normalizePluginText(name)
	if normalized == "" {
		return nil
	}
	aliases := []string{normalized}
	parts := strings.Fields(normalized)
	if len(parts) >= 3 {
		aliases = append(aliases, strings.Join(parts[1:], " "))
	}
	return aliases
}

func normalizePluginText(value string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeToolName(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func containsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func argText(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value := strings.TrimSpace(fmt.Sprint(args[key]))
	if value == "<nil>" {
		return ""
	}
	return value
}

func firstArgText(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := argText(args, key); value != "" {
			return value
		}
	}
	return ""
}
