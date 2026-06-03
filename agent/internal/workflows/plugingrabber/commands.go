package plugingrabber

import (
	"fmt"
	"strings"
)

const (
	pluginGrabberLearnCommand = "plugin_grabber_learn_project_profile"
	pluginGrabberLearnTool    = "plugin_grabber.learn_project_profile"
	pluginGrabberLoadCommand  = "plugin_grabber_load_and_get_params"
	pluginGrabberLoadTool     = "plugin_grabber.load_and_get_params"
)

func PluginIntentKind(userText string) string {
	lower := strings.ToLower(strings.TrimSpace(userText))
	return pluginIntentKindFromText(userText, lower)
}

func pluginLoadVerbText(userText, lower string) bool {
	return strings.Contains(lower, "load") ||
		strings.Contains(lower, "insert") ||
		strings.Contains(lower, "add ") ||
		strings.HasPrefix(lower, "add") ||
		strings.Contains(userText, "\u52a0\u8f7d") ||
		strings.Contains(userText, "\u63d2\u5165") ||
		strings.Contains(userText, "\u52a0\u4e00\u4e2a") ||
		strings.Contains(userText, "\u6dfb\u52a0") ||
		strings.Contains(userText, "加载") ||
		strings.Contains(userText, "插入") ||
		strings.Contains(userText, "加一个")
}

func pluginNegativeLoadText(userText, lower string) bool {
	return strings.Contains(lower, "do not load") ||
		strings.Contains(lower, "don't load") ||
		strings.Contains(lower, "not load") ||
		strings.Contains(userText, "\u4e0d\u52a0\u8f7d") ||
		strings.Contains(userText, "\u4e0d\u8981\u52a0\u8f7d")
}

func pluginIntentKindFromText(userText, lower string) string {
	if hasInstrumentPluginSubject(userText, lower) {
		return "instrument"
	}
	if hasEffectPluginSubject(userText, lower) {
		return "effect"
	}
	return ""
}

func hasAnyPluginSubject(userText, lower string) bool {
	return strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "vst") ||
		hasEffectPluginSubject(userText, lower) ||
		hasInstrumentPluginSubject(userText, lower)
}

func hasInstrumentPluginSubject(userText, lower string) bool {
	return strings.Contains(lower, "synth") ||
		strings.Contains(lower, "synthesizer") ||
		strings.Contains(lower, "instrument") ||
		strings.Contains(lower, "vsti") ||
		strings.Contains(lower, "midi instrument") ||
		strings.Contains(lower, "sampler") ||
		strings.Contains(userText, "\u5408\u6210\u5668") ||
		strings.Contains(userText, "\u4e50\u5668") ||
		strings.Contains(userText, "\u97f3\u6e90") ||
		strings.Contains(userText, "MIDI\u4e50\u5668") ||
		strings.Contains(userText, "MIDI \u4e50\u5668")
}

func hasEffectPluginSubject(userText, lower string) bool {
	return strings.Contains(lower, "effect") ||
		strings.Contains(lower, "fx") ||
		strings.Contains(lower, "eq") ||
		strings.Contains(lower, "equalizer") ||
		strings.Contains(lower, "compressor") ||
		strings.Contains(lower, "reverb") ||
		strings.Contains(lower, "delay") ||
		strings.Contains(lower, "limiter") ||
		strings.Contains(lower, "analyzer") ||
		strings.Contains(userText, "\u6548\u679c\u5668") ||
		strings.Contains(userText, "\u5747\u8861") ||
		strings.Contains(userText, "\u6df7\u54cd") ||
		strings.Contains(userText, "\u538b\u7f29") ||
		strings.Contains(userText, "\u5ef6\u8fdf") ||
		strings.Contains(userText, "\u9650\u5236")
}

func synthesizePluginGrabberLearningCommands(userText string, requestContext map[string]any) []map[string]any {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return nil
	}
	hasLearnIntent := strings.Contains(lower, "learn") ||
		strings.Contains(lower, "profile") ||
		strings.Contains(lower, "quick control") ||
		strings.Contains(lower, "quick") ||
		strings.Contains(userText, "学习") ||
		strings.Contains(userText, "保存常用") ||
		strings.Contains(userText, "常用控制") ||
		strings.Contains(userText, "参数分类") ||
		strings.Contains(userText, "插件抓手")
	hasPluginSubject := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "param") ||
		strings.Contains(lower, "control") ||
		strings.Contains(userText, "插件") ||
		strings.Contains(userText, "参数") ||
		strings.Contains(userText, "控制")
	if !hasLearnIntent || !hasPluginSubject {
		return nil
	}
	cmd := map[string]any{
		"cmd":    pluginGrabberLearnCommand,
		"intent": userText,
	}
	for _, key := range []string{"selected_plugin_id", "selected_plugin_name", "selected_plugin_track_id", "selected_track_id"} {
		if value := strings.TrimSpace(fmt.Sprint(requestContext[key])); value != "" && value != "<nil>" {
			cmd[key] = value
		}
	}
	return []map[string]any{cmd}
}

func synthesizePluginGrabberLoadCommands(userText string, requestContext map[string]any) []map[string]any {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return nil
	}
	if looksLikePluginLoadInquiry(userText, lower) {
		return nil
	}
	hasLoadIntent := strings.Contains(lower, "load") ||
		strings.Contains(lower, "insert") ||
		strings.Contains(lower, "add ") ||
		strings.HasPrefix(lower, "add") ||
		strings.Contains(lower, "add plugin") ||
		strings.Contains(userText, "\u52a0\u8f7d") ||
		strings.Contains(userText, "\u63d2\u5165") ||
		strings.Contains(userText, "\u52a0\u4e00\u4e2a") ||
		strings.Contains(userText, "\u6dfb\u52a0") ||
		strings.Contains(userText, "加载") ||
		strings.Contains(userText, "插入") ||
		strings.Contains(userText, "加一个")
	hasGrabIntent := strings.Contains(lower, "grab") ||
		strings.Contains(lower, "parameter") ||
		strings.Contains(lower, "quick control") ||
		strings.Contains(lower, "control") ||
		strings.Contains(userText, "抓") ||
		strings.Contains(userText, "参数") ||
		strings.Contains(userText, "控制")
	hasPluginSubject := hasAnyPluginSubject(userText, lower) ||
		strings.Contains(userText, "\u63d2\u4ef6") ||
		strings.Contains(userText, "插件")
	if !hasLoadIntent || (!hasPluginSubject && !hasGrabIntent) {
		return nil
	}
	query := extractPluginLoadQuery(userText)
	if query == "" {
		return nil
	}
	cmd := map[string]any{
		"cmd":          pluginGrabberLoadCommand,
		"plugin_query": query,
		"intent":       userText,
	}
	if kind := pluginIntentKindFromText(userText, lower); kind != "" {
		cmd["plugin_intent_kind"] = kind
		if kind == "instrument" {
			cmd["include_instruments"] = true
			cmd["plugin_type"] = "synth"
		}
	}
	for _, key := range []string{"selected_track_id", "selected_track_name", "selected_plugin_track_id"} {
		if value := strings.TrimSpace(fmt.Sprint(requestContext[key])); value != "" && value != "<nil>" {
			cmd[key] = value
		}
	}
	return []map[string]any{cmd}
}

func extractPluginLoadQuery(text string) string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return ""
	}
	candidates := []string{clean}
	for _, marker := range []string{"add a ", "add an ", "add ", "load", "Load", "LOAD", "insert", "\u52a0\u8f7d", "\u63d2\u5165", "\u52a0\u4e00\u4e2a", "\u6dfb\u52a0", "加载", "插入", "加一个"} {
		if idx := strings.Index(clean, marker); idx >= 0 {
			candidates = append([]string{strings.TrimSpace(clean[idx+len(marker):])}, candidates...)
			break
		}
	}
	query := candidates[0]
	for _, cut := range []string{" then ", " and ", " to ", " into ", "然后", "并", "到", "，", ",", "。"} {
		if idx := strings.Index(query, cut); idx > 0 {
			query = strings.TrimSpace(query[:idx])
		}
	}
	replacer := strings.NewReplacer(
		"plugin", "", "Plugin", "", "VST3", "", "vst3", "", "VST", "", "vst", "",
		"current track", "", "selected track", "", "track", "", "rack", "",
		"a ", "", "an ", "",
		"\u5f53\u524d\u8f68\u9053", "", "\u9009\u4e2d\u8f68\u9053", "", "\u73b0\u5728\u9009\u4e2d\u7684\u8f68\u9053", "",
		"\u63d2\u4ef6", "", "\u6548\u679c\u5668", "", "\u4e00\u4e2a", "", "\u7ed9", "",
		"插件", "", "效果器", "", "一个", "", "一個", "",
		"帮我", "", "請", "", "请", "",
	)
	query = strings.Trim(query, " \t\r\n\"'“”‘’")
	query = strings.TrimSpace(replacer.Replace(query))
	query = strings.Trim(query, " \t\r\n\"'“”‘’")
	return query
}

func synthesizePluginLibraryCommands(userText string) []map[string]any {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return nil
	}
	if wantsPluginScan(userText, lower) {
		cmd := map[string]any{"cmd": "scan_plugins"}
		if path := extractPluginScanPath(userText); path != "" {
			cmd["paths"] = []string{path}
		}
		return []map[string]any{cmd}
	}
	if wantsPluginSemanticSearch(userText, lower) {
		if types := semanticPluginTypesFromText(userText, lower); len(types) > 1 {
			cmds := make([]map[string]any, 0, len(types))
			for _, typ := range types {
				cmds = append(cmds, pluginSemanticSearchCommand(typ, typ))
			}
			return cmds
		}
		query := semanticPluginTypeFromText(userText, lower)
		if query == "" {
			query = extractPluginSearchQuery(userText)
		}
		if query == "" {
			return nil
		}
		cmd := pluginSemanticSearchCommand(query, semanticPluginTypeFromText(userText, lower))
		return []map[string]any{cmd}
	}
	if wantsPluginList(userText, lower) {
		cmd := map[string]any{"cmd": "plugin_list_available", "limit": 10}
		if limit := extractSmallPositiveNumber(userText); limit > 0 {
			cmd["limit"] = limit
		}
		return []map[string]any{cmd}
	}
	if wantsPluginSearch(userText, lower) {
		query := extractPluginSearchQuery(textWithoutFiller(userText))
		if query == "" {
			return nil
		}
		return []map[string]any{{
			"cmd":   "plugin_search",
			"query": query,
			"limit": 10,
		}}
	}
	return nil
}

func pluginSemanticSearchCommand(query, typ string) map[string]any {
	cmd := map[string]any{
		"cmd":   "plugin_semantic_search",
		"query": query,
		"limit": 8,
	}
	if typ != "" {
		cmd["type"] = typ
	}
	if typ == "synth" || typ == "instrument" {
		cmd["include_instruments"] = true
	}
	return cmd
}

func wantsPluginScan(userText, lower string) bool {
	hasScan := strings.Contains(lower, "scan") || strings.Contains(lower, "rescan") || strings.Contains(userText, "扫描")
	hasPlugin := strings.Contains(lower, "plugin") || strings.Contains(lower, "vst") || strings.Contains(userText, "插件")
	return hasScan && hasPlugin
}

func wantsPluginList(userText, lower string) bool {
	if strings.Contains(userText, "候选") || strings.Contains(lower, "candidate") {
		return false
	}
	hasSearch := strings.Contains(lower, "search") ||
		strings.Contains(lower, "find") ||
		strings.Contains(userText, "搜索") ||
		strings.Contains(userText, "查找") ||
		strings.Contains(userText, "找")
	hasInventoryQuestion := strings.Contains(userText, "有什么") ||
		strings.Contains(userText, "有哪些") ||
		strings.Contains(userText, "都有什么") ||
		strings.Contains(lower, "what plugins") ||
		strings.Contains(lower, "which plugins")
	hasList := strings.Contains(lower, "list") ||
		strings.Contains(lower, "show") ||
		strings.Contains(userText, "列出") ||
		strings.Contains(userText, "显示")
	hasPlugin := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "vst") ||
		strings.Contains(userText, "插件") ||
		strings.Contains(userText, "插件库")
	return (hasInventoryQuestion || (hasList && !hasSearch)) && hasPlugin
}

func textWithoutFiller(text string) string {
	replacer := strings.NewReplacer("一下", "", "帮我", "", "请", "", "請", "")
	return strings.TrimSpace(replacer.Replace(text))
}

func wantsPluginSearch(userText, lower string) bool {
	hasSearch := strings.Contains(lower, "search") ||
		strings.Contains(lower, "find") ||
		strings.Contains(userText, "搜索") ||
		strings.Contains(userText, "查找") ||
		strings.Contains(userText, "找")
	hasPlugin := hasAnyPluginSubject(userText, lower) ||
		strings.Contains(userText, "\u63d2\u4ef6") ||
		strings.Contains(userText, "插件")
	return hasSearch && hasPlugin && !looksLikePluginGrabberLoadIntent(userText)
}

func wantsPluginSemanticSearch(userText, lower string) bool {
	hasRecommend := strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "suggest") ||
		strings.Contains(lower, "best") ||
		strings.Contains(userText, "\u63a8\u8350")
	hasFind := strings.Contains(lower, "search") ||
		strings.Contains(lower, "find") ||
		strings.Contains(userText, "\u627e") ||
		strings.Contains(userText, "\u641c\u7d22") ||
		strings.Contains(userText, "\u67e5\u627e")
	hasInspect := strings.Contains(userText, "\u68c0\u67e5") ||
		strings.Contains(userText, "\u770b\u770b") ||
		strings.Contains(userText, "\u770b\u4e00\u4e0b") ||
		strings.Contains(userText, "\u54ea\u4e2a") ||
		strings.Contains(userText, "\u54ea\u4e9b") ||
		strings.Contains(userText, "\u6709\u6ca1\u6709") ||
		strings.Contains(lower, "check") ||
		strings.Contains(lower, "inspect") ||
		strings.Contains(lower, "which")
	hasSubject := hasAnyPluginSubject(userText, lower) ||
		strings.Contains(userText, "\u63d2\u4ef6") ||
		strings.Contains(userText, "插件")
	hasType := semanticPluginTypeFromText(userText, lower) != ""
	return (hasRecommend || hasFind || hasInspect) && (hasSubject || hasType) && hasType && !looksLikePluginGrabberLoadIntent(userText)
}

func semanticPluginTypeFromText(userText, lower string) string {
	switch {
	case strings.Contains(lower, "equalizer") || strings.Contains(lower, "eq") || strings.Contains(userText, "\u5747\u8861"):
		return "eq"
	case strings.Contains(lower, "reverb") || strings.Contains(lower, "verb") || strings.Contains(userText, "\u6df7\u54cd"):
		return "reverb"
	case strings.Contains(lower, "compressor") || strings.Contains(lower, "compression") || strings.Contains(lower, " comp") || strings.Contains(lower, "comp ") || strings.Contains(userText, "\u538b\u7f29"):
		return "compressor"
	case strings.Contains(lower, "delay") || strings.Contains(lower, "echo") || strings.Contains(userText, "\u5ef6\u8fdf"):
		return "delay"
	case strings.Contains(lower, "limiter") || strings.Contains(userText, "\u9650\u5236"):
		return "limiter"
	case strings.Contains(lower, "analyzer") || strings.Contains(lower, "spectrum") || strings.Contains(lower, "meter") || strings.Contains(userText, "\u5206\u6790"):
		return "analyzer"
	case hasInstrumentPluginSubject(userText, lower):
		return "synth"
	default:
		return ""
	}
}

func semanticPluginTypesFromText(userText, lower string) []string {
	candidates := []struct {
		typ     string
		needles []string
	}{
		{"eq", []string{"equalizer", "eq", "\u5747\u8861"}},
		{"reverb", []string{"reverb", "verb", "\u6df7\u54cd"}},
		{"compressor", []string{"compressor", "compression", " comp", "comp ", "\u538b\u7f29"}},
		{"delay", []string{"delay", "echo", "\u5ef6\u8fdf"}},
		{"limiter", []string{"limiter", "\u9650\u5236"}},
		{"analyzer", []string{"analyzer", "spectrum", "meter", "\u5206\u6790"}},
		{"synth", []string{"synth", "synthesizer", "instrument", "vsti", "midi instrument", "\u5408\u6210\u5668", "\u4e50\u5668", "\u97f3\u6e90", "MIDI\u4e50\u5668", "MIDI \u4e50\u5668"}},
	}
	var out []string
	for _, candidate := range candidates {
		for _, needle := range candidate.needles {
			if strings.Contains(lower, strings.ToLower(needle)) || strings.Contains(userText, needle) {
				out = append(out, candidate.typ)
				break
			}
		}
	}
	return out
}

func extractPluginSearchQuery(text string) string {
	query := strings.TrimSpace(text)
	for _, marker := range []string{"\u641c\u7d22", "\u67e5\u627e", "\u627e", "search", "Search", "find", "Find"} {
		if idx := strings.Index(query, marker); idx >= 0 {
			query = strings.TrimSpace(query[idx+len(marker):])
			break
		}
	}
	for _, cut := range []string{
		"\u63d2\u4ef6",
		"\u6548\u679c\u5668",
		"\u53ea\u5217\u51fa",
		"\u4e0d\u52a0\u8f7d",
		"\u5019\u9009",
		"plugin candidates",
		"candidate plugins",
		"candidates only",
		"candidate only",
		"candidates",
		"candidate",
		"do not load",
		"only do not load",
		"do not insert",
		"do not instantiate",
		",",
		"\uff0c",
		"\u3002",
	} {
		if idx := strings.Index(strings.ToLower(query), strings.ToLower(cut)); idx > 0 {
			query = strings.TrimSpace(query[:idx])
		}
	}
	query = strings.Trim(query, " \t\r\n\"'")
	replacer := strings.NewReplacer("plugin", "", "Plugin", "", "\u63d2\u4ef6", "", "\u6548\u679c\u5668", "")
	query = strings.TrimSpace(replacer.Replace(query))
	return strings.Trim(query, " \t\r\n\"'")
}
func extractPluginScanPath(text string) string {
	lower := strings.ToLower(text)
	for _, marker := range []string{"扫描", "scan"} {
		idx := strings.Index(lower, strings.ToLower(marker))
		if idx < 0 {
			continue
		}
		tail := strings.TrimSpace(text[idx+len(marker):])
		for _, marker2 := range []string{"目录", "路径", "folder", "path"} {
			tail = strings.ReplaceAll(tail, marker2, "")
		}
		tail = strings.TrimSpace(tail)
		if strings.Contains(tail, ":/") || strings.Contains(tail, ":\\") {
			for _, cut := range []string{" 的插件", " 插件", "，", ",", "。"} {
				if cutIdx := strings.Index(tail, cut); cutIdx > 0 {
					tail = strings.TrimSpace(tail[:cutIdx])
				}
			}
			return strings.Trim(tail, " \t\r\n\"'“”‘’")
		}
	}
	return ""
}

func extractSmallPositiveNumber(text string) int {
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if token == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(token, "%d", &n); err == nil && n > 0 && n <= 100 {
			return n
		}
	}
	return 0
}

func firstPluginGrabberLearningCommand(commands []map[string]any) (map[string]any, bool) {
	for _, cmd := range commands {
		name := strings.TrimSpace(fmt.Sprint(cmd["cmd"]))
		if name == "" || name == "<nil>" {
			name = strings.TrimSpace(fmt.Sprint(cmd["command"]))
		}
		toolName := strings.TrimSpace(fmt.Sprint(cmd["tool"]))
		if name == pluginGrabberLearnCommand || toolName == pluginGrabberLearnTool {
			return cmd, true
		}
	}
	return nil, false
}

func firstPluginGrabberLoadCommand(commands []map[string]any) (map[string]any, bool) {
	for _, cmd := range commands {
		name := strings.TrimSpace(fmt.Sprint(cmd["cmd"]))
		if name == "" || name == "<nil>" {
			name = strings.TrimSpace(fmt.Sprint(cmd["command"]))
		}
		toolName := strings.TrimSpace(fmt.Sprint(cmd["tool"]))
		if name == pluginGrabberLoadCommand || toolName == pluginGrabberLoadTool {
			return cmd, true
		}
	}
	return nil, false
}

func coercePluginGrabberLoadCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if looksLikePluginLoadInquiry(userText, lower) {
		return nil, false
	}
	if cmd, ok := firstPluginGrabberLoadCommand(commands); ok {
		applyPluginIntentFields(cmd, userText, lower)
		return cmd, true
	}
	if !looksLikePluginGrabberLoadIntent(userText) {
		return nil, false
	}
	for _, cmd := range commands {
		args := workflowCommandArgs(cmd)
		name := strings.TrimSpace(fmt.Sprint(args["cmd"]))
		if name == "" || name == "<nil>" {
			name = strings.TrimSpace(fmt.Sprint(args["command"]))
		}
		toolName := strings.TrimSpace(fmt.Sprint(args["tool"]))
		if name != "instantiate_plugin" && name != "rack_add_node" && toolName != "plugin.instantiate" && toolName != "plugin.load_to_rack" && toolName != "rack.add_node" {
			continue
		}
		workflow := map[string]any{
			"cmd":    pluginGrabberLoadCommand,
			"intent": userText,
		}
		if kind := pluginIntentKindFromText(userText, lower); kind != "" {
			workflow["plugin_intent_kind"] = kind
			if kind == "instrument" {
				workflow["include_instruments"] = true
				workflow["plugin_type"] = "synth"
			}
		}
		copyWorkflowField(workflow, args, "track_id", "track_id", "selected_track_id", "selected_plugin_track_id")
		copyWorkflowField(workflow, args, "plugin_path", "plugin_path", "path", "file_path")
		copyWorkflowField(workflow, args, "plugin_query", "plugin_query", "query", "plugin_name", "name", "plugin")
		if firstNonEmptyText(workflow, "track_id") == "" {
			copyWorkflowField(workflow, requestContext, "track_id", "selected_track_id", "selected_plugin_track_id", "track_id")
		}
		if firstNonEmptyText(workflow, "plugin_query") == "" && firstNonEmptyText(workflow, "plugin_path") == "" {
			if query := extractPluginLoadQuery(userText); query != "" {
				workflow["plugin_query"] = query
			}
		}
		return workflow, true
	}
	return nil, false
}

func applyPluginIntentFields(cmd map[string]any, userText, lower string) {
	if cmd == nil {
		return
	}
	if firstNonEmptyText(cmd, "plugin_intent_kind") != "" {
		return
	}
	if kind := pluginIntentKindFromText(userText, lower); kind != "" {
		cmd["plugin_intent_kind"] = kind
		if kind == "instrument" {
			cmd["include_instruments"] = true
			cmd["plugin_type"] = "synth"
		}
	}
}

func looksLikePluginGrabberLoadIntent(userText string) bool {
	lower := strings.ToLower(strings.TrimSpace(userText))
	if lower == "" {
		return false
	}
	if pluginNegativeLoadText(userText, lower) {
		return false
	}
	if looksLikePluginLoadInquiry(userText, lower) {
		return false
	}
	if pluginLoadVerbText(userText, lower) && hasAnyPluginSubject(userText, lower) {
		return true
	}
	if strings.Contains(lower, "do not load") ||
		strings.Contains(lower, "don't load") ||
		strings.Contains(lower, "not load") ||
		strings.Contains(userText, "不加载") ||
		strings.Contains(userText, "不要加载") {
		return false
	}
	hasLoadIntent := strings.Contains(lower, "load") ||
		strings.Contains(lower, "insert") ||
		strings.Contains(lower, "add plugin") ||
		strings.Contains(userText, "加载") ||
		strings.Contains(userText, "插入") ||
		strings.Contains(userText, "加一个")
	hasGrabIntent := strings.Contains(lower, "grab") ||
		strings.Contains(lower, "parameter") ||
		strings.Contains(lower, "quick control") ||
		strings.Contains(lower, "control") ||
		strings.Contains(userText, "抓") ||
		strings.Contains(userText, "参数") ||
		strings.Contains(userText, "控制")
	hasPluginSubject := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "vst") ||
		strings.Contains(lower, "eq") ||
		strings.Contains(lower, "compressor") ||
		strings.Contains(lower, "reverb") ||
		strings.Contains(userText, "均衡") ||
		strings.Contains(userText, "混响") ||
		strings.Contains(userText, "压缩") ||
		strings.Contains(userText, "延迟") ||
		strings.Contains(userText, "限制") ||
		strings.Contains(userText, "插件") ||
		strings.Contains(userText, "效果器")
	return hasLoadIntent && (hasGrabIntent || hasPluginSubject)
}

func looksLikePluginLoadInquiry(userText, lower string) bool {
	hasPluginSubject := strings.Contains(lower, "plugin") ||
		strings.Contains(lower, "vst") ||
		strings.Contains(lower, "rack") ||
		strings.Contains(userText, "插件") ||
		strings.Contains(userText, "效果器") ||
		strings.Contains(userText, "机架") ||
		strings.Contains(userText, "轨道")
	hasCountQuestion := strings.Contains(userText, "几个") ||
		strings.Contains(userText, "多少") ||
		strings.Contains(lower, "how many") ||
		strings.Contains(lower, "count")
	hasLoadedQuestion := strings.Contains(userText, "加载了") ||
		strings.Contains(userText, "已加载") ||
		strings.Contains(userText, "加载过") ||
		strings.Contains(lower, "loaded") ||
		strings.Contains(lower, "did you load") ||
		strings.Contains(lower, "was it loaded")
	hasQuestionMarker := strings.Contains(userText, "？") ||
		strings.Contains(userText, "?") ||
		strings.Contains(userText, "吗") ||
		strings.Contains(userText, "是否") ||
		strings.Contains(userText, "有没有") ||
		strings.Contains(userText, "有没") ||
		strings.Contains(lower, "what") ||
		strings.Contains(lower, "which") ||
		strings.Contains(lower, "how many")
	return hasPluginSubject && ((hasCountQuestion && (hasLoadedQuestion || hasQuestionMarker)) || (hasLoadedQuestion && hasQuestionMarker))
}

func copyWorkflowField(dst map[string]any, src map[string]any, target string, keys ...string) {
	if firstNonEmptyText(dst, target) != "" {
		return
	}
	for _, key := range keys {
		if value := firstNonEmptyText(src, key); value != "" {
			dst[target] = value
			return
		}
	}
}

func SynthesizeLearningCommands(userText string, requestContext map[string]any) []map[string]any {
	return synthesizePluginGrabberLearningCommands(userText, requestContext)
}

func SynthesizeLoadCommands(userText string, requestContext map[string]any) []map[string]any {
	return synthesizePluginGrabberLoadCommands(userText, requestContext)
}

func ExtractLoadQuery(text string) string {
	return extractPluginLoadQuery(text)
}

func SynthesizeLibraryCommands(userText string) []map[string]any {
	return synthesizePluginLibraryCommands(userText)
}

func FirstLearningCommand(commands []map[string]any) (map[string]any, bool) {
	return firstPluginGrabberLearningCommand(commands)
}

func FirstLoadCommand(commands []map[string]any) (map[string]any, bool) {
	return firstPluginGrabberLoadCommand(commands)
}

func CoerceLoadCommand(commands []map[string]any, userText string, requestContext map[string]any) (map[string]any, bool) {
	return coercePluginGrabberLoadCommand(commands, userText, requestContext)
}

func LooksLikeLoadIntent(userText string) bool {
	return looksLikePluginGrabberLoadIntent(userText)
}

func CopyWorkflowField(dst map[string]any, src map[string]any, target string, keys ...string) {
	copyWorkflowField(dst, src, target, keys...)
}

func workflowCommandArgs(cmd map[string]any) map[string]any {
	out := map[string]any{}
	if args, ok := cmd["args"].(map[string]any); ok {
		for k, v := range args {
			out[k] = v
		}
	}
	for k, v := range cmd {
		if k == "args" {
			continue
		}
		out[k] = v
	}
	return out
}
