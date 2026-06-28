package mom

import "strings"

func ResolveIntent(args map[string]any, fallback string) string {
	for _, key := range []string{"mom_intent", "projection_intent", "workflow_intent", "intent"} {
		if intent := normalizeIntent(text(args[key])); intent != "" {
			return intent
		}
	}
	if intent := normalizeIntent(fallback); intent != "" {
		return intent
	}
	if wantsRealtime(args) {
		return IntentRealtimeBandStereoObservation
	}
	goal := strings.ToLower(strings.TrimSpace(firstNonEmpty(text(args["goal_text"]), text(args["goal"]))))
	if containsAny(goal, "render probe", "offline probe", "post-fx", "post fx", "post-fader", "post fader", "master out") {
		return IntentRealtimeBandStereoObservation
	}
	if wantsABResult(args, goal) {
		return IntentABResultObservation
	}
	readOnly := observationOnlyGoal(goal)
	if !readOnly && wantsActionPreflight(goal) {
		return IntentActionPreflightObservation
	}
	if wantsProjectMultitrack(args, goal) {
		return IntentProjectMultitrackObservation
	}
	if readOnly {
		return IntentGeneralBandStereoObservation
	}
	return IntentGeneralBandStereoObservation
}

func observationOnlyGoal(goal string) bool {
	if containsAny(goal, "read only", "do not modify", "don't modify", "no edit", "不要修改", "不要处理", "不要动", "不修改", "不处理", "只观察", "仅观察", "只分析", "仅分析") {
		return true
	}
	if wantsActionPreflight(goal) {
		return false
	}
	return containsAny(goal, "observe", "observation", "analyze", "analyse", "check", "观察", "分析", "检查", "看一下", "看看")
}

func wantsActionPreflight(goal string) bool {
	if containsAny(goal,
		"adjust", "change", "fix", "lower", "raise", "widen", "compress", "mix this", "mix it", "process", "treat",
		"帮我", "处理", "调整", "调一下", "修改", "收一点", "收紧", "压低", "降低", "加宽", "削一点", "减少", "可以处理", "可以进行处理", "进行处理", "执行", "应用",
	) {
		return true
	}
	hasEQ := containsAny(goal, "eq", "均衡", "低切", "高通", "滤波")
	hasLowTarget := containsAny(goal, "低频", "低中频", "低端", "low end", "low-end", "low mid", "low-mid", "mud", "muddy")
	hasSmallMove := containsAny(goal, "轻微", "稍微", "一点", "一点点", "微调", "少许", "slight", "slightly", "a little")
	return hasEQ && (hasLowTarget || hasSmallMove)
}

func normalizeIntent(intent string) string {
	intent = strings.ToLower(strings.TrimSpace(intent))
	switch intent {
	case "", "observation_only":
		return ""
	case "general", "frequency_stereo", "band_stereo", "band_stereo_status", "frequency_stereo_status", "mix_diagnosis":
		return IntentGeneralBandStereoObservation
	case "realtime", "l2", "l2_realtime", "realtime_playback", "render_probe", "l2_render_probe", "offline_probe", "post_fx", "post_fader", "post_fx_probe", "post_fader_probe", "master_out":
		return IntentRealtimeBandStereoObservation
	case "project_multitrack", "multitrack", "project_mix", "project_relation", "mix_relationship", "project_multitrack_relation_observation":
		return IntentProjectMultitrackObservation
	case "ab", "a_b", "a/b", "ab_result", "before_after", "before_after_result", "ab_result_observation":
		return IntentABResultObservation
	case "action_preflight", "action_preflight_observation", "propose_pending":
		return IntentActionPreflightObservation
	default:
		if strings.Contains(intent, "ab_result") || strings.Contains(intent, "a/b") || strings.Contains(intent, "before_after") {
			return IntentABResultObservation
		}
		if strings.Contains(intent, "realtime") || strings.Contains(intent, "l2") || strings.Contains(intent, "render_probe") || strings.Contains(intent, "offline_probe") || strings.Contains(intent, "post_fx") || strings.Contains(intent, "post_fader") || strings.Contains(intent, "master_out") {
			return IntentRealtimeBandStereoObservation
		}
		if strings.Contains(intent, "preflight") || strings.Contains(intent, "pending") {
			return IntentActionPreflightObservation
		}
		if strings.Contains(intent, "multitrack") || strings.Contains(intent, "relationship") || strings.Contains(intent, "project_mix") {
			return IntentProjectMultitrackObservation
		}
		return intent
	}
}

func wantsABResult(args map[string]any, goal string) bool {
	for _, key := range []string{"requested_layer", "projection", "scope", "feature_keys"} {
		value := strings.ToLower(text(args[key]))
		if containsAny(value, "ab_result", "a/b", "before_after", "comparison_delta") {
			return true
		}
	}
	return containsAny(goal, "ab result", "a/b", "before after", "before/after", "after change", "compare before", "comparison delta", "ab 结果", "ab结果", "前后对比", "处理前后", "修改前后")
}

func wantsProjectMultitrack(args map[string]any, goal string) bool {
	for _, key := range []string{"requested_layer", "projection", "scope", "feature_keys"} {
		value := strings.ToLower(text(args[key]))
		if containsAny(value, "multitrack", "relationship", "project_mix", "full_project", "overall_mix", "track_conflict") {
			return true
		}
	}
	return containsAny(goal,
		"整体混音", "全工程", "多轨", "各轨", "冲突", "频段分布", "声像布局", "声像关系", "轨道关系",
		"overall mix", "full project", "multitrack", "multi-track", "track relationship", "track conflict", "band distribution", "stereo layout",
	)
}

func wantsRealtime(args map[string]any) bool {
	if boolValue(args["prefer_realtime"]) {
		return true
	}
	for _, key := range []string{"requested_layer", "capture_mode", "projection", "feature_keys"} {
		value := strings.ToLower(text(args[key]))
		if strings.Contains(value, "l2") || strings.Contains(value, "realtime") || strings.Contains(value, "playback") || strings.Contains(value, "live") || strings.Contains(value, "render_probe") || strings.Contains(value, "offline_probe") || strings.Contains(value, "post_fx") || strings.Contains(value, "post_fader") || strings.Contains(value, "master_out") {
			return true
		}
	}
	goal := strings.ToLower(strings.TrimSpace(firstNonEmpty(text(args["goal_text"]), text(args["goal"]))))
	if containsAny(goal, "l2", "realtime", "playback", "live", "meter", "spectrum", "correlation", "实时", "播放后", "当前监听", "电平", "频谱", "相关性", "后级", "处理后") {
		return true
	}
	return false
}

func IntentRequiredLayers(intent string) []string {
	switch intent {
	case IntentRealtimeBandStereoObservation:
		return []string{"project_structure", "l2_realtime.render_probe", "l2_realtime.live_meter_optional", "l2_realtime.realtime_stereo_correlation_optional", "trust_quality.l2_tap_point"}
	case IntentProjectMultitrackObservation:
		return []string{"project_structure", "project_mix_profile", "multitrack_relation", "trust_quality"}
	case IntentActionPreflightObservation:
		return []string{"project_structure", "action_relevant_mom_layer", "trust_quality", "evidence_refs"}
	case IntentABResultObservation:
		return []string{"project_structure", "ab_result_comparison", "trust_quality.same_tap_render_probe"}
	default:
		return []string{"project_structure", "timbre_frequency.l3_full_song", "space_stereo.l3_full_song", "trust_quality"}
	}
}

func IntentOptionalLayers(intent string) []string {
	switch intent {
	case IntentRealtimeBandStereoObservation:
		return []string{"l3_full_song_background_reference", "basic_energy", "time_dynamics_structure", "multitrack_relationship", "ab_result_comparison"}
	case IntentProjectMultitrackObservation:
		return []string{"basic_energy", "l3_band_stereo_target_detail", "l2_realtime_status_only", "time_dynamics_structure", "ab_result_comparison"}
	case IntentActionPreflightObservation:
		return []string{"basic_energy", "l2_realtime_status", "time_dynamics_structure", "multitrack_relationship", "ab_result_comparison"}
	case IntentABResultObservation:
		return []string{"basic_energy", "timbre_frequency", "space_stereo", "time_dynamics_structure"}
	default:
		return []string{"basic_energy", "l2_realtime_status_only", "time_dynamics_structure", "multitrack_relationship", "ab_result_comparison"}
	}
}

func StatusFromSource(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "baseline_ready", "fresh", "available", "ok", "clear", "candidate":
		return StatusReady
	case "partial", "async_partial", "limited":
		return StatusPartial
	case "approximate", "approx":
		return StatusApprox
	case "building", "requested", "pending", "deferred", "async_missing":
		return StatusDeferred
	case "stale":
		return StatusStale
	case "failed", "invalid", "error", "blocked", "unavailable", "suspect":
		return StatusSuspect
	default:
		return StatusMissing
	}
}

func FreshnessForStatus(status string) string {
	switch StatusFromSource(status) {
	case StatusReady:
		return "fresh"
	case StatusPartial:
		return "partial"
	case StatusApprox:
		return "approximate"
	case StatusDeferred:
		return "deferred"
	case StatusStale:
		return "stale"
	case StatusSuspect:
		return "suspect"
	default:
		return "missing"
	}
}

func rollupStatus(intent string, layers Layers, project ProjectStructure) string {
	critical := []string{
		StatusFromSource(project.Status),
		StatusFromSource(layers.TimbreFrequency.Status),
		StatusFromSource(layers.SpaceStereo.Status),
	}
	switch intent {
	case IntentRealtimeBandStereoObservation:
		critical = []string{
			StatusFromSource(project.Status),
			StatusFromSource(layers.TimbreFrequency.Status),
			StatusFromSource(layers.SpaceStereo.Status),
		}
	case IntentActionPreflightObservation:
		critical = []string{
			StatusFromSource(project.Status),
			StatusFromSource(layers.TimbreFrequency.Status),
			StatusFromSource(layers.SpaceStereo.Status),
		}
	}
	ready := 0
	partial := 0
	deferred := 0
	stale := 0
	suspect := 0
	missing := 0
	for _, status := range critical {
		switch status {
		case StatusReady:
			ready++
		case StatusPartial, StatusApprox:
			partial++
		case StatusDeferred:
			deferred++
		case StatusStale:
			stale++
		case StatusSuspect:
			suspect++
		default:
			missing++
		}
	}
	if suspect > 0 && ready == 0 && partial == 0 {
		return StatusSuspect
	}
	if stale > 0 && ready == 0 && partial == 0 {
		return StatusStale
	}
	if ready == len(critical) {
		return StatusReady
	}
	if ready > 0 || partial > 0 {
		return StatusPartial
	}
	if deferred > 0 {
		return StatusDeferred
	}
	_ = missing
	return StatusMissing
}
