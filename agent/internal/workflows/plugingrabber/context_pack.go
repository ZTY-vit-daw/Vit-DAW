package plugingrabber

import (
	"fmt"
	"sort"
	"strings"
)

func BuildContextPack(digest ParameterDigest) map[string]any {
	paramIndex := map[string]ParameterInfo{}
	for _, param := range digest.Parameters {
		paramIndex[param.ID] = param
	}

	quickRows := make([]map[string]any, 0, len(digest.QuickControls))
	quickIDs := map[string]bool{}
	for _, quick := range digest.QuickControls {
		param := paramIndex[quick.ParamID]
		if param.ID == "" {
			param.ID = quick.ParamID
		}
		label := strings.TrimSpace(quick.Label)
		if label == "" {
			label = parameterDisplayName(param)
		}
		group := firstNonEmptyString(quick.DisplayGroup, param.DisplayGroup, "Other")
		role := firstNonEmptyString(quick.NormalizedRole, param.NormalizedRole, "other")
		semanticHint := pluginSemanticHint(param, role, group)
		quickIDs[quick.ParamID] = true
		quickRows = append(quickRows, map[string]any{
			"param_id":                     quick.ParamID,
			"label":                        label,
			"widget":                       quick.Widget,
			"display_group":                group,
			"normalized_role":              role,
			"value":                        param.Value,
			"normalized_value":             param.NormalizedValue,
			"value_text":                   param.ValueText,
			"min":                          param.Min,
			"max":                          param.Max,
			"is_boolean":                   param.IsBoolean,
			"is_discrete":                  param.IsDiscrete,
			"host_controllable":            param.HostControllable,
			"control_relevance":            param.ControlRelevance,
			"semantic_hint":                semanticHint,
			"explanation_hint":             semanticHint["likely_meaning"],
			"full_parameter_ref":           quick.ParamID,
			"recommended_source":           digest.ProfileSource,
			"still_part_of_all_parameters": true,
		})
		if param.DisplayDomainCandidate != nil {
			quickRows[len(quickRows)-1]["display_domain_candidate"] = param.DisplayDomainCandidate
		}
		if param.DisplayProbe != nil {
			quickRows[len(quickRows)-1]["display_probe"] = param.DisplayProbe
		}
	}

	groups := buildPluginContextGroups(digest, quickIDs)
	roles := buildPluginRoleSummary(digest)
	macroCandidates := buildPluginMacroCandidates(quickRows)
	allParameters := buildPluginAllParameterRows(digest)
	runtimeProfile := buildPluginRuntimeProfile(digest)
	pluginName := digest.PluginName
	if pluginName == "" {
		pluginName = digest.PluginID
	}
	source := digest.ProfileSource
	if source == "" {
		source = "heuristic"
	}

	return map[string]any{
		"status":                  "ok",
		"schema_version":          "plugin_grabber_context_pack.v1",
		"context_strategy":        "summary_first_full_parameters_available",
		"track_id":                digest.TrackID,
		"plugin_id":               digest.PluginID,
		"plugin_name":             pluginName,
		"plugin_identity":         digest.PluginIdentity,
		"template_role":           digest.TemplateRole,
		"profile_source":               source,
		"profile_applied":              digest.ProfileApplied,
		"profile_stale_param_ids":      digest.ProfileStaleParamIDs,
		"global_profile_applied":       digest.GlobalProfileApplied,
		"global_profile_source":        digest.GlobalProfileSource,
		"current_param_signature_hash": digest.CurrentParamSignatureHash,
		"profile_param_signature_hash": digest.ProfileParamSignatureHash,
		"plugin_class":            digest.PluginClass,
		"parameters_retained":     true,
		"parameter_count":         digest.ParameterCount,
		"display_probe_summary":   DisplayProbeSummary(digest),
		"quick_control_count":     len(digest.QuickControls),
		"recommended_group_count": len(groups),
		"semantic_hint_policy": map[string]any{
			"kind":   "weak_local_heuristic",
			"source": "local_semantic_hint_rule",
			"caveat": "Hints are prior clues for AI/user reasoning, not verified facts about the plugin DSP. Keep raw param_id/name and verify by listening or opening the plugin UI when precision matters.",
		},
		"quick_controls":       quickRows,
		"all_parameters":       allParameters,
		"all_parameter_count":  len(allParameters),
		"all_parameters_note":  "This is the complete controllable surface. quick_controls is a small UI convenience subset chosen by local heuristics, NOT the limit of what can be controlled. Always pick param_id from all_parameters when writing a parameter.",
		"macro_candidates":     macroCandidates,
		"groups":           groups,
		"role_summary":     roles,
		"runtime_profile":  runtimeProfile,
		"full_parameter_access": map[string]any{
			"command":   "get_plugin_parameters",
			"track_id":  digest.TrackID,
			"plugin_id": digest.PluginID,
			"args":      map[string]any{"include_parameters": true},
			"note":      "all_parameters above already carries the complete controllable surface. Call this only to re-read live values after a write, or to page through a very large plugin via offset/limit.",
			"all_view":  true,
		},
	}
}

func buildPluginRuntimeProfile(digest ParameterDigest) map[string]any {
	out := map[string]any{
		"available": digest.GlobalProfileApplied || len(digest.PluginSkill) > 0 || digest.PluginClass != "" || len(digest.PluginGroups) > 0 || len(digest.VirtualControls) > 0,
	}
	if digest.GlobalProfileSource != "" {
		out["source"] = digest.GlobalProfileSource
	} else if digest.GlobalProfileApplied {
		out["source"] = "global_profile"
	}
	if digest.PluginClass != "" {
		out["class"] = digest.PluginClass
	}
	if len(digest.PluginGroups) > 0 {
		out["semantic_group_count"] = len(digest.PluginGroups)
		out["semantic_groups"] = compactProfileRows(digest.PluginGroups, 12, []string{"id", "role", "label", "name"})
	}
	if len(digest.VirtualControls) > 0 {
		out["virtual_control_count"] = len(digest.VirtualControls)
		out["virtual_controls"] = compactProfileRows(digest.VirtualControls, 12, []string{"name", "component_id", "component", "resolver"})
	}
	if safety := SanitizeProfileSafety(digest.SafetyLimits); len(safety) > 0 {
		out["safety_limits"] = safety
	}
	if len(digest.PluginSkill) > 0 {
		components := mapRowsValue(digest.PluginSkill["components"])
		operations := mapRowsValue(digest.PluginSkill["operations"])
		out["plugin_skill_available"] = true
		out["plugin_skill_schema_version"] = digest.PluginSkill["schema_version"]
		out["component_count"] = len(components)
		out["operation_count"] = len(operations)
		if capabilities := mapValue(digest.PluginSkill["capabilities"]); len(capabilities) > 0 {
			out["capabilities"] = capabilities
		}
		out["components"] = compactPluginSkillComponents(components, 12)
		out["operations"] = compactPluginSkillOperations(operations, 12)
	}
	return out
}

func compactProfileRows(rows []map[string]any, limit int, keys []string) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{}
		for _, key := range keys {
			if text := firstNonEmptyText(row, key); text != "" {
				compact[key] = text
			}
		}
		if params := compactPluginSkillParams(row["params"], 8); len(params) > 0 {
			compact["params"] = params
		}
		out = append(out, compact)
	}
	return out
}

func compactPluginSkillComponents(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{
			"id":    firstNonEmptyText(row, "id"),
			"role":  firstNonEmptyText(row, "role"),
			"label": firstNonEmptyText(row, "label"),
		}
		if params := compactPluginSkillParams(row["params"], 12); len(params) > 0 {
			compact["params"] = params
		}
		out = append(out, compact)
	}
	return out
}

func compactPluginSkillOperations(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		compact := map[string]any{
			"name":         firstNonEmptyText(row, "name"),
			"resolver":     firstNonEmptyText(row, "resolver"),
			"component_id": firstNonEmptyText(row, "component_id"),
			"inputs":       stringSliceValue(row["inputs"]),
		}
		if params := compactPluginSkillParams(row["params"], 12); len(params) > 0 {
			compact["params"] = params
		}
		out = append(out, compact)
	}
	return out
}

func compactPluginSkillParams(value any, limit int) []map[string]any {
	params := mapValue(value)
	if len(params) == 0 {
		return nil
	}
	slots := make([]string, 0, len(params))
	for slot := range params {
		slots = append(slots, slot)
	}
	sort.Strings(slots)
	if limit > 0 && len(slots) > limit {
		slots = slots[:limit]
	}
	out := make([]map[string]any, 0, len(slots))
	for _, slot := range slots {
		row := map[string]any{"slot": slot}
		switch mapped := params[slot].(type) {
		case map[string]any:
			if paramID := firstNonEmptyText(mapped, "param_id", "id"); paramID != "" {
				row["param_id"] = paramID
			}
			if label := firstNonEmptyText(mapped, "label", "name"); label != "" {
				row["label"] = label
			}
			if confidence := mapped["confidence"]; confidence != nil {
				row["confidence"] = confidence
			}
		default:
			if text := strings.TrimSpace(fmt.Sprint(mapped)); text != "" && text != "<nil>" {
				row["param_id"] = text
			}
		}
		if row["param_id"] != nil {
			out = append(out, row)
		}
	}
	return out
}

// buildPluginAllParameterRows emits the complete controllable surface so the LLM
// can pick any param_id, not just the heuristic quick-control subset. Each row
// carries the inferred display domain (unit/min/max/scale) when available; the
// raw 5-point probe samples are attached only when that inference is missing or
// low confidence, so a large plugin stays a few KB rather than hundreds of rows
// of sample text.
func buildPluginAllParameterRows(digest ParameterDigest) []map[string]any {
	out := make([]map[string]any, 0, len(digest.Parameters))
	for _, param := range digest.Parameters {
		row := map[string]any{
			"param_id":         param.ID,
			"name":             parameterDisplayName(param),
			"normalized_value": param.NormalizedValue,
			"value_text":       param.ValueText,
		}
		if group := strings.TrimSpace(param.DisplayGroup); group != "" {
			row["display_group"] = group
		}
		if role := strings.TrimSpace(param.NormalizedRole); role != "" && role != "other" {
			row["normalized_role"] = role
		}
		if param.IsBoolean {
			row["is_boolean"] = true
		}
		if param.IsDiscrete {
			row["is_discrete"] = true
		}
		if !param.HostControllable {
			row["host_controllable"] = false
		}
		needProbe := true
		if param.DisplayDomainCandidate != nil {
			row["display_domain_candidate"] = param.DisplayDomainCandidate
			needProbe = param.DisplayDomainCandidate.Confidence < 0.80
		}
		if needProbe && param.DisplayProbe != nil {
			row["display_probe"] = param.DisplayProbe
		}
		out = append(out, row)
	}
	return out
}

func buildPluginContextGroups(digest ParameterDigest, quickIDs map[string]bool) []map[string]any {
	type groupInfo struct {
		Name     string
		IDs      []string
		QuickIDs []string
	}
	groupsByName := map[string]*groupInfo{}
	order := []string{}
	for _, param := range digest.Parameters {
		group := strings.TrimSpace(param.DisplayGroup)
		if group == "" {
			group = "Other"
		}
		info := groupsByName[group]
		if info == nil {
			info = &groupInfo{Name: group}
			groupsByName[group] = info
			order = append(order, group)
		}
		info.IDs = append(info.IDs, param.ID)
		if quickIDs[param.ID] {
			info.QuickIDs = append(info.QuickIDs, param.ID)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return groupSortKey(order[i]) < groupSortKey(order[j])
	})
	out := make([]map[string]any, 0, len(order))
	for _, name := range order {
		info := groupsByName[name]
		out = append(out, map[string]any{
			"name":              info.Name,
			"parameter_count":   len(info.IDs),
			"quick_control_ids": firstStringLimit(info.QuickIDs, 12),
			"sample_param_ids":  firstStringLimit(info.IDs, 8),
		})
	}
	return out
}

func buildPluginRoleSummary(digest ParameterDigest) []map[string]any {
	counts := map[string]int{}
	for _, param := range digest.Parameters {
		role := strings.TrimSpace(param.NormalizedRole)
		if role == "" {
			role = "other"
		}
		counts[role]++
	}
	roles := make([]string, 0, len(counts))
	for role := range counts {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool {
		if counts[roles[i]] == counts[roles[j]] {
			return roles[i] < roles[j]
		}
		return counts[roles[i]] > counts[roles[j]]
	})
	if len(roles) > 16 {
		roles = roles[:16]
	}
	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		out = append(out, map[string]any{"normalized_role": role, "parameter_count": counts[role]})
	}
	return out
}

func buildPluginMacroCandidates(quickRows []map[string]any) []map[string]any {
	type candidate struct {
		Score int
		Row   map[string]any
	}
	candidates := make([]candidate, 0, len(quickRows))
	for _, row := range quickRows {
		role := strings.ToLower(firstNonEmptyText(row, "normalized_role"))
		if boolValue(row["is_boolean"]) || role == "common_bypass" {
			continue
		}
		score, reason := macroCandidateScoreAndReason(role, firstNonEmptyText(row, "display_group"))
		if score <= 0 {
			continue
		}
		hint, _ := row["semantic_hint"].(map[string]any)
		confidence := "low"
		if hint != nil {
			confidence = firstNonEmptyText(hint, "confidence")
		}
		candidates = append(candidates, candidate{
			Score: score,
			Row: map[string]any{
				"param_id":             row["param_id"],
				"label":                row["label"],
				"display_group":        row["display_group"],
				"normalized_role":      row["normalized_role"],
				"confidence":           confidence,
				"source":               "local_semantic_hint_rule",
				"candidate_reason":     reason,
				"requires_user_review": true,
			},
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > 8 {
		candidates = candidates[:8]
	}
	out := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.Row)
	}
	return out
}

func macroCandidateScoreAndReason(role, group string) (int, string) {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "common_mix", "bus_mix":
		return 100, "混合/干湿比通常适合做宏控，因为它能提供一个直观的效果量控制。"
	case "eq_gain", "tone_drive", "tone_filter", "mod_depth":
		return 90, "这个控制可能会明显改变听感强度，适合作为演奏型宏控候选。"
	case "comp_threshold", "comp_ratio":
		return 85, "这个控制可能影响动态处理强度，适合考虑做压缩/动态量宏控。"
	case "eq_frequency", "eq_cutoff", "eq_freq_low", "eq_freq_mid", "eq_freq_high", "eq_q":
		return 75, "这个控制可能影响音色焦点，和同频段的增益/Q 联动时适合做音色宏控。"
	case "common_gain", "comp_makeup_gain", "common_width", "common_pan":
		return 65, "这个控制适合做平衡或空间类宏控，但绑定前需要谨慎限制范围。"
	case "mod_rate":
		return 60, "这个控制适合调制类宏控，但节奏/速度关系需要靠听感确认。"
	}
	if group == "Tone" || group == "Dynamics" || group == "Mix" {
		return 40, "这个控制位于音乐相关分组，但真实行为需要人工确认后再绑定宏控。"
	}
	return 0, ""
}

func FormatContextPackReply(pack map[string]any) string {
	if pack == nil {
		return ""
	}
	pluginName := firstNonEmptyText(pack, "plugin_name", "plugin_id")
	paramCount := firstNonEmptyText(pack, "parameter_count")
	source := profileSourceDisplayName(firstNonEmptyText(pack, "profile_source"))
	quickRows := mapRowsValue(pack["quick_controls"])
	quickLabels := make([]string, 0, len(quickRows))
	quickHints := make([]string, 0, len(quickRows))
	for _, row := range quickRows {
		if label := firstNonEmptyText(row, "label", "param_id"); label != "" {
			quickLabels = append(quickLabels, label)
			if hint, ok := row["semantic_hint"].(map[string]any); ok && len(quickHints) < 4 {
				meaning := firstNonEmptyText(hint, "likely_meaning")
				confidence := firstNonEmptyText(hint, "confidence")
				if meaning != "" {
					if confidence != "" {
						meaning += " [" + confidenceDisplayName(confidence) + "]"
					}
					quickHints = append(quickHints, label+": "+meaning)
				}
			}
		}
		if len(quickLabels) >= 10 {
			break
		}
	}
	groupRows := mapRowsValue(pack["groups"])
	groupSummaries := make([]string, 0, len(groupRows))
	for _, row := range groupRows {
		name := firstNonEmptyText(row, "name")
		count := firstNonEmptyText(row, "parameter_count")
		if name != "" && count != "" {
			groupSummaries = append(groupSummaries, fmt.Sprintf("%s(%s)", name, count))
		}
		if len(groupSummaries) >= 8 {
			break
		}
	}
	runtimeProfile := mapValue(pack["runtime_profile"])
	runtimeSummary := ""
	if boolValue(runtimeProfile["available"]) {
		runtimeBits := []string{}
		if cls := firstNonEmptyText(runtimeProfile, "class"); cls != "" {
			runtimeBits = append(runtimeBits, "class="+cls)
		}
		if count := firstNonEmptyText(runtimeProfile, "component_count"); count != "" {
			runtimeBits = append(runtimeBits, "components="+count)
		}
		if count := firstNonEmptyText(runtimeProfile, "operation_count"); count != "" {
			runtimeBits = append(runtimeBits, "operations="+count)
		}
		if len(runtimeBits) == 0 {
			runtimeBits = append(runtimeBits, "available")
		}
		runtimeSummary = "Runtime profile: " + strings.Join(runtimeBits, ", ") + "."
	}
	macroRows := mapRowsValue(pack["macro_candidates"])
	macroLabels := make([]string, 0, len(macroRows))
	for _, row := range macroRows {
		if label := firstNonEmptyText(row, "label", "param_id"); label != "" {
			macroLabels = append(macroLabels, label)
		}
		if len(macroLabels) >= 6 {
			break
		}
	}
	parts := []string{fmt.Sprintf("%s 插件抓手上下文：保留 %s 个完整参数，来源=%s。", pluginName, paramCount, source)}
	if len(quickLabels) > 0 {
		parts = append(parts, "优先控制："+strings.Join(quickLabels, "、")+"。")
	}
	if len(quickHints) > 0 {
		parts = append(parts, "语义提示："+strings.Join(quickHints, "；")+"。")
	}
	if len(groupSummaries) > 0 {
		parts = append(parts, "分组："+strings.Join(groupSummaries, "、")+"。")
	}
	if runtimeSummary != "" {
		parts = append(parts, runtimeSummary)
	}
	if len(macroLabels) > 0 {
		parts = append(parts, "宏控候选："+strings.Join(macroLabels, "、")+"；绑定前需要复核范围。")
	}
	parts = append(parts, "语义提示只是本地启发式弱提示，不是已验证的插件事实。")
	parts = append(parts, "完整参数仍可通过 get_plugin_parameters 获取；这个上下文包只是摘要/索引。")
	return strings.Join(parts, "\n")
}

func profileSourceDisplayName(source string) string {
	switch strings.TrimSpace(source) {
	case "project_profile":
		return "工程记录"
	case "heuristic", "":
		return "本地启发式"
	default:
		return source
	}
}

func confidenceDisplayName(confidence string) string {
	switch strings.TrimSpace(confidence) {
	case "high":
		return "高置信度"
	case "medium":
		return "中等置信度"
	case "low":
		return "低置信度"
	default:
		return confidence
	}
}

func parameterDisplayName(param ParameterInfo) string {
	return firstNonEmptyString(param.Alias, param.Name, param.RawName, param.ID)
}

func pluginSemanticHint(param ParameterInfo, role, group string) map[string]any {
	role = strings.ToLower(strings.TrimSpace(role))
	group = strings.TrimSpace(group)
	name := strings.ToLower(parameterDisplayName(param) + " " + param.RawName + " " + param.ID)
	hint := map[string]any{
		"likely_meaning":    "可控制的插件参数。",
		"typical_direction": "具体方向依插件而定；在做强判断前应查看插件 UI 或用听感确认。",
		"good_for":          []string{"手动调整", "结合完整参数上下文进行 AI 推理"},
		"confidence":        "low",
		"source":            "local_semantic_hint_rule",
		"basis":             "normalized_role/display_group/raw_parameter_name",
		"caveat":            "这是弱语义提示，不是对插件 DSP 行为的已验证结论。",
	}

	set := func(meaning, direction, confidence string, goodFor []string) {
		hint["likely_meaning"] = meaning
		hint["typical_direction"] = direction
		hint["confidence"] = confidence
		hint["good_for"] = goodFor
	}

	switch role {
	case "common_mix", "bus_mix":
		set("可能是干湿比或处理后信号混合量。", "通常更高代表更多处理后信号，更低代表更多干声。", "medium", []string{"混合平衡", "并行处理宏控"})
	case "common_gain", "comp_makeup_gain":
		set("可能是电平或补偿增益控制。", "通常更高代表电平更高；做宏控时需要限制范围，避免音量突跳。", "medium", []string{"电平匹配", "输出平衡"})
	case "common_bypass":
		set("可能是处理器旁通或启用开关。", "通常用于启用或旁通处理。", "medium", []string{"A/B 对比", "安全开关"})
	case "common_pan":
		set("可能是声像或左右平衡控制。", "通常一个方向偏左，另一个方向偏右。", "medium", []string{"空间定位", "自动化"})
	case "common_width":
		set("可能是立体声宽度控制。", "通常更高代表更宽，更低代表更窄。", "medium", []string{"立体声宽度宏控", "混音宽度控制"})
	case "comp_threshold":
		set("可能是动态阈值。", "通常更低会触发更多压缩或动态处理。", "medium", []string{"压缩量宏控", "动态控制"})
	case "comp_ratio":
		set("可能是压缩/动态比例或强度。", "通常更高代表动态处理更强。", "medium", []string{"压缩量宏控", "动态塑形"})
	case "comp_attack":
		set("可能是动态处理的 Attack 时间。", "通常更短/更快会更快响应瞬态。", "medium", []string{"瞬态塑形", "动态时间控制"})
	case "comp_release":
		set("可能是动态处理的 Release 时间。", "通常更短/更快会在处理后更快恢复。", "medium", []string{"律动塑形", "动态时间控制"})
	case "eq_frequency", "eq_cutoff", "eq_freq_low", "eq_freq_mid", "eq_freq_high":
		set("可能是 EQ/滤波器频率选择。", "通常更高代表目标频率区域更高。", "medium", []string{"音色宏控", "频率扫动", "频段定位"})
	case "eq_gain":
		set("可能是 EQ 频段增益。", "通常更高代表提升该频段，更低代表削减。", "medium", []string{"音色塑形", "频段量宏控"})
	case "eq_q":
		set("可能是 EQ 带宽/Q 或共振。", "通常更高代表更窄或更强共振，但不同插件可能不同。", "medium", []string{"频段聚焦", "与增益/频率联动的音色宏控"})
	case "tone_drive":
		set("可能是 Drive 或饱和量。", "通常更高代表更多驱动、饱和或谐波强度。", "medium", []string{"音色性格宏控", "音色强度"})
	case "tone_filter":
		set("可能是滤波或音色塑形控制。", "方向取决于滤波类型；应结合邻近的频率/增益参数判断。", "medium", []string{"音色塑形", "滤波宏控"})
	case "mod_rate":
		set("可能是调制速度/Rate。", "通常更高代表调制速度更快。", "medium", []string{"运动感宏控", "节奏调制"})
	case "mod_depth":
		set("可能是调制深度/量。", "通常更高代表调制强度更大。", "medium", []string{"运动量宏控", "效果强度"})
	}

	if hint["confidence"] == "low" {
		switch {
		case strings.Contains(name, "dry") || strings.Contains(name, "wet"):
			set("可能与干湿比或电平混合有关。", "方向取决于它控制的是 Dry Level、Wet Level 还是合并的 Mix。", "low", []string{"混合平衡", "人工复核"})
		case strings.Contains(name, "sidechain") || strings.Contains(name, " sc "):
			set("可能与侧链有关。", "方向取决于检测器/滤波设计；需要用插件 UI 确认。", "low", []string{"动态设置", "人工复核"})
		case strings.Contains(name, "delta"):
			set("可能是 Delta/差异监听。", "它可能切换监听模式，而不一定直接塑造声音。", "low", []string{"A/B 检查", "人工复核"})
		case strings.Contains(name, "band") && strings.Contains(name, "gain"):
			set("可能是频段增益。", "更高可能提升该频段，更低可能削减，但需要对照插件 UI 确认。", "low", []string{"音色塑形", "人工复核"})
		case strings.Contains(name, "band") && (strings.Contains(name, "freq") || strings.Contains(name, "frequency")):
			set("可能是频段频率选择。", "更高可能把频段移动到更高频率。", "low", []string{"音色塑形", "人工复核"})
		case group != "" && group != "Other":
			set("可能是 "+group+" 分组里的控制。", "方向依插件而定，只能当作线索。", "low", []string{"手动调整", "结合邻近参数进行 AI 推理"})
		}
	}
	return hint
}

func pluginControlExplanationHint(role, group string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	group = strings.TrimSpace(group)
	switch role {
	case "common_mix", "bus_mix":
		return "Blend between dry and processed signal."
	case "common_gain", "comp_makeup_gain":
		return "Level control; use it to balance loudness before or after processing."
	case "common_bypass":
		return "Enable or bypass the processor."
	case "common_pan":
		return "Stereo position control."
	case "common_width":
		return "Stereo width control."
	case "comp_threshold":
		return "Dynamics threshold; lower values usually trigger more compression."
	case "comp_ratio":
		return "Compression ratio or strength."
	case "comp_attack":
		return "How quickly dynamics processing reacts to transients."
	case "comp_release":
		return "How quickly dynamics processing relaxes after gain reduction."
	case "eq_frequency", "eq_cutoff", "eq_freq_low", "eq_freq_mid", "eq_freq_high":
		return "Frequency selection for tonal shaping."
	case "eq_gain":
		return "Boost or cut amount for an EQ band."
	case "eq_q":
		return "EQ bandwidth or resonance."
	case "tone_drive":
		return "Drive or saturation amount."
	case "tone_filter":
		return "Tone/filter shaping control."
	case "mod_rate":
		return "Modulation speed."
	case "mod_depth":
		return "Modulation amount."
	}
	if group != "" && group != "Other" {
		return "Relevant " + group + " control; confirm exact behavior by listening or opening the plugin UI."
	}
	return "Controllable parameter; exact musical behavior may be plugin-specific."
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func groupSortKey(group string) string {
	switch group {
	case "Tone":
		return "00"
	case "Dynamics":
		return "01"
	case "Mix":
		return "02"
	case "Modulation":
		return "03"
	case "Utility":
		return "04"
	case "Routing":
		return "05"
	case "Other":
		return "99"
	default:
		return "50_" + strings.ToLower(group)
	}
}

func firstNonEmptyText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func mapRowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			if m, ok := row.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{rows}
	default:
		return nil
	}
}

func boolValue(value any) bool {
	switch x := value.(type) {
	case bool:
		return x
	case string:
		s := strings.ToLower(strings.TrimSpace(x))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	case int:
		return x != 0
	case float64:
		return x != 0
	default:
		return false
	}
}

func firstStringLimit(values []string, limit int) []string {
	if limit < 0 {
		limit = 0
	}
	if limit > len(values) {
		limit = len(values)
	}
	out := make([]string, limit)
	copy(out, values[:limit])
	return out
}
