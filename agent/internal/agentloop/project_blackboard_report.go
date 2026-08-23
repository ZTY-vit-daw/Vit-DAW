package agentloop

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/planner"
)

func messageLoopProjectBlackboardStatusRequest(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if messageLoopContinuationInstruction(text) {
		return false
	}
	// An open diagnostic request is a product task, not a request for the
	// deterministic project-blackboard progress dump. It must reach semantic
	// entry so the runtime can observe the project, assess capacity, and create
	// the appropriate diagnostic/improvement contract.
	if messageLoopOpenProjectDiagnosticRequest(text) {
		return false
	}
	// Open acoustic/mixing requests must reach the model-owned observation
	// loop.  Phrases such as "overall mix status" contain the same status
	// vocabulary as a blackboard report, but they describe the subject of the
	// requested mix observation rather than asking for a project progress dump.
	if messageLoopNaturalMixRequest(text) && !messageLoopExplicitProjectBlackboardReport(text) {
		return false
	}
	if messageLoopClipFadeGainRequest(text) || messageLoopStripSilenceSuggestRequest(text) {
		return false
	}
	if messageLoopGainStagingCapabilityRequest(text) {
		return false
	}
	if messageLoopProjectBlackboardAcousticObservationStatus(text) {
		return false
	}
	lower := strings.ToLower(text)
	if messageLoopTextHasAny(lower,
		"工程黑板", "黑板状态", "状态汇报", "状态报告", "工程状态", "项目状态", "混音状态", "阶段状态", "阶段性状态",
		"进度到哪", "进展到哪", "现在到哪", "现在进度", "阶段完成", "a阶段", "b阶段", "c阶段", "d阶段", "e阶段", "f阶段",
		"准备好了吗", "准备情况", "可以开始混", "现在可以开始", "开始混吗", "当前工程总结", "总结一下当前工程", "还有什么问题",
		"project status", "blackboard status", "mix status", "mixing status", "current status", "status report", "progress", "ready to mix",
	) {
		return true
	}
	hasScope := messageLoopTextHasAny(lower,
		"工程", "项目", "混音", "阶段", "a-f", "a到f", "a-f", "workflow", "project", "mix", "mixing", "stage",
	)
	hasStatus := messageLoopTextHasAny(lower,
		"状态", "进度", "进展", "汇报", "报告", "总结", "准备", "完成", "下一步", "问题", "风险",
		"status", "progress", "summary", "report", "ready", "done", "next", "risk",
	)
	return hasScope && hasStatus
}

func messageLoopOpenProjectDiagnosticRequest(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	return messageLoopTextHasAny(lower, "工程", "项目", "project") &&
		messageLoopTextHasAny(lower, "检查", "查看", "看看", "分析", "问题", "风险", "inspect", "check", "analy") &&
		!messageLoopTextHasAny(lower, "状态汇报", "状态报告", "progress report", "blackboard status", "工程状态汇报", "项目状态汇报")
}

func messageLoopContinuationInstruction(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return messageLoopTextHasAny(lower,
		"继续基于", "继续工作", "继续处理", "续跑", "恢复未完成", "基于最初目标",
		"continue based", "continue working", "resume", "unfinished continuation",
	)
}

func messageLoopExplicitProjectBlackboardReport(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	return messageLoopTextHasAny(lower,
		"project status", "blackboard status", "status report", "progress report", "stage report",
		"工程状态汇报", "项目状态汇报", "进展到哪个阶段", "进展到哪一阶段", "当前工程总结", "总结一下当前工程",
	)
}

func messageLoopProjectBlackboardAcousticObservationStatus(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	hasAcousticMetric := messageLoopTextHasAny(lower,
		"loudness", "stereo", "phase", "correlation", "spectrum", "spectral", "frequency", "band energy", "peak", "rms", "lufs", "waveform", "envelope",
		"声像", "声相", "声场", "立体声", "相位", "相关", "频段", "频谱", "频率", "低频", "中频", "高频", "响度", "峰值", "均方根", "波形", "包络",
	)
	if !hasAcousticMetric {
		return false
	}
	return messageLoopTextHasAny(lower,
		"observe", "observation", "analysis", "analyze", "analyse", "inspect", "status", "state",
		"观察", "分析", "检查", "查看", "看一下", "看看", "状态",
	)
}

func messageLoopProjectBlackboardStateCall() planner.ToolCall {
	return planner.ToolCall{
		ID:     "project_blackboard_state",
		Tool:   "project.state",
		Args:   map[string]any{},
		Reason: "read current project snapshot for Project Blackboard Status Report v0",
	}
}

func messageLoopProjectBlackboardReportFromImport(data messageLoopStemsImportReportData) string {
	lines := []string{"工程黑板状态汇报", "", "结论"}
	appendMessageLoopBullet(&lines, messageLoopImportReadinessConclusion(data))

	lines = append(lines, "", "工程概况")
	appendMessageLoopBullet(&lines, fmt.Sprintf("轨道/片段：%d / %d", data.TracksCreated, data.ClipsCreated))
	if data.Discovered > 0 || data.Readable > 0 || data.Unreadable > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("素材清点：发现 %d，可读 %d，不可读 %d", data.Discovered, data.Readable, data.Unreadable))
	}
	if data.HasStart || data.HasLength {
		parts := []string{}
		if data.HasStart {
			parts = append(parts, fmt.Sprintf("%.2fs 起", data.StartSeconds))
		}
		if data.HasLength {
			parts = append(parts, fmt.Sprintf("%.2fs 长", data.LengthSeconds))
		}
		appendMessageLoopBullet(&lines, "时间范围："+strings.Join(parts, "，"))
	}
	if data.SourceSpec != "" {
		appendMessageLoopBullet(&lines, "素材规格："+data.SourceSpec)
	}
	if data.ProjectSpec != "" {
		appendMessageLoopBullet(&lines, "工程规格："+data.ProjectSpec)
	}
	appendMessageLoopBullet(&lines, fmt.Sprintf("差异：采样率不匹配 %d；位深/格式不匹配 %d", data.SampleRateMismatches, data.BitDepthMismatches))
	if data.ProjectSettingsChanged {
		if data.ProjectSettingsPatchSpec != "" {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步到 "+data.ProjectSettingsPatchSpec)
		} else {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步")
		}
	}
	if data.CopyPolicy != "" {
		appendMessageLoopBullet(&lines, "媒体策略："+messageLoopMediaPolicyLabel(data.CopyPolicy))
	}

	lines = append(lines, "", "A-F 状态矩阵")
	appendMessageLoopBullet(&lines, "A 工程准备：已完成初始导入观察；TIM/TOM/EPM 结果见下方")
	messageLoopAppendStaticMixCapabilityStatusLines(&lines, nil)
	appendMessageLoopBullet(&lines, "C 细混处理：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "D 母带前检查：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "E 母带：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "F 混音审查：尚未记录为已执行")

	lines = append(lines, "", "A1 工程接收")
	appendMessageLoopBullet(&lines, fmt.Sprintf("轨道/片段：%d / %d", data.TracksCreated, data.ClipsCreated))
	if data.Discovered > 0 || data.Readable > 0 || data.Unreadable > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("素材清点：发现 %d，可读 %d，不可读 %d", data.Discovered, data.Readable, data.Unreadable))
	}
	if data.HasStart || data.HasLength {
		parts := []string{}
		if data.HasStart {
			parts = append(parts, fmt.Sprintf("%.2fs 起", data.StartSeconds))
		}
		if data.HasLength {
			parts = append(parts, fmt.Sprintf("%.2fs 长", data.LengthSeconds))
		}
		appendMessageLoopBullet(&lines, "时间范围："+strings.Join(parts, "，"))
	}
	if data.SourceSpec != "" {
		appendMessageLoopBullet(&lines, "素材规格："+data.SourceSpec)
	}
	if data.ProjectSpec != "" {
		appendMessageLoopBullet(&lines, "工程规格："+data.ProjectSpec)
	}
	appendMessageLoopBullet(&lines, fmt.Sprintf("差异：采样率不匹配 %d；位深/格式不匹配 %d", data.SampleRateMismatches, data.BitDepthMismatches))
	if data.ProjectSettingsChanged {
		if data.ProjectSettingsPatchSpec != "" {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步到 "+data.ProjectSettingsPatchSpec)
		} else {
			appendMessageLoopBullet(&lines, "工程设置：导入前已同步")
		}
	}
	if data.CopyPolicy != "" {
		appendMessageLoopBullet(&lines, "媒体策略："+messageLoopMediaPolicyLabel(data.CopyPolicy))
	}

	lines = append(lines, "", "A2 TIM 技术完整性")
	if len(data.TIMLines) > 0 {
		lines = append(lines, data.TIMLines...)
	} else {
		appendMessageLoopBullet(&lines, "DAD 声学事实已满足检查门槛。")
	}
	if len(data.TOMLines) > 0 {
		lines = append(lines, "", "A3 TOM 智能整理建议")
		lines = append(lines, data.TOMLines...)
	}
	if len(data.A4EPMLines) > 0 {
		lines = append(lines, "", "A4 EPM Clip 裁剪/Fade 建议")
		lines = append(lines, data.A4EPMLines...)
	}
	if len(data.A5EPMLines) > 0 {
		lines = append(lines, "", "A5 EPM 段落地图推荐")
		lines = append(lines, data.A5EPMLines...)
	}

	lines = append(lines, "", "风险与遗留")
	if data.Unreadable > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("阻塞/风险：存在 %d 个不可读素材，需要先复查路径或文件格式", data.Unreadable))
	}
	if data.SampleRateMismatches > 0 || data.BitDepthMismatches > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("建议复查：采样率不匹配 %d；位深/格式不匹配 %d", data.SampleRateMismatches, data.BitDepthMismatches))
	}
	if data.WarningCount > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("工具返回 %d 条警告，可在详情里查看", data.WarningCount))
	}
	if data.Unreadable == 0 && data.SampleRateMismatches == 0 && data.BitDepthMismatches == 0 && data.WarningCount == 0 {
		appendMessageLoopBullet(&lines, "当前报告没有发现阻止继续工作的硬性问题")
	}

	lines = append(lines, "", "已知用户目标")
	appendMessageLoopBullet(&lines, "尚未记录明确的混音参考方向、核心元素优先级或处理范围")

	lines = append(lines, "", "可选下一步")
	appendMessageLoopBullet(&lines, "确认或修改 TOM 文件夹整理建议")
	appendMessageLoopBullet(&lines, "按需确认 A4 片段清理/裁剪/Fade 建议")
	appendMessageLoopBullet(&lines, "按需确认 A5 段落 Marker 写入")
	appendMessageLoopBullet(&lines, "也可以直接跳到 B1 Gain Staging 或 B2 静态平衡；A-F 不作为线性门禁")

	lines = append(lines, "", "数据来源")
	appendMessageLoopBullet(&lines, "project.import_folder_as_stems / TIM projection / TOM projection / EPM projection")
	return strings.Join(lines, "\n")
}

func messageLoopImportReadinessConclusion(data messageLoopStemsImportReportData) string {
	if !data.TIMReady {
		return "当前导入结果尚未满足 TIM/DAD 检查门槛，暂不生成完整工程状态判断。"
	}
	if data.Unreadable > 0 {
		return "存在不可读素材，建议先处理路径或文件格式后再继续大范围混音。"
	}
	if data.WarningCount > 0 || data.SampleRateMismatches > 0 || data.BitDepthMismatches > 0 {
		return "工程可以继续，但存在需要复查的技术注意事项；A-F 后续任务可非线性选择。"
	}
	return "工程已具备继续工作的基础状态；A-F 后续任务可按目标非线性选择。"
}

func messageLoopProjectBlackboardReportFromRecord(state *runState, record map[string]any) string {
	result := messageLoopMapValue(record["result"])
	if !messageLoopExecutionSucceeded(record) {
		errText := firstNonEmpty(messageLoopText(record["error"]), firstMapText(result, "error"), "无法读取当前工程快照")
		return messageLoopProjectBlackboardFailureReport(errText)
	}
	return messageLoopProjectBlackboardReportFromProjectState(state, result)
}

func messageLoopProjectBlackboardFailureReport(errText string) string {
	lines := []string{"工程黑板状态汇报", "", "结论"}
	appendMessageLoopBullet(&lines, "存在阻塞：当前无法读取工程快照，报告只能给出缺失说明。")
	lines = append(lines, "", "风险与遗留")
	appendMessageLoopBullet(&lines, firstNonEmpty(strings.TrimSpace(errText), "project.state 未返回可用结果"))
	lines = append(lines, "", "数据来源")
	appendMessageLoopBullet(&lines, "project.state 读取失败")
	return strings.Join(lines, "\n")
}

func messageLoopProjectBlackboardReportFromProjectState(state *runState, result map[string]any) string {
	project := messageLoopProjectBlackboardProjectState(result)
	tracks := messageLoopProjectBlackboardTracks(project)
	trackCount := firstPositiveMapInt(project, "track_count", "user_track_count", "audio_track_count")
	if trackCount <= 0 {
		trackCount = len(tracks)
	}
	clipCount := firstPositiveMapInt(project, "clip_count", "audio_clip_count", "project_audio_clip_count")
	if clipCount <= 0 {
		clipCount = messageLoopProjectBlackboardClipCount(tracks)
	}
	folderCount := messageLoopProjectBlackboardFolderCount(tracks)
	markerCount := messageLoopProjectBlackboardMarkerCount(project)
	duration, hasDuration := messageLoopProjectBlackboardDuration(project, tracks)
	projectSpec := messageLoopAudioSpecText(
		firstPositiveMapInt(project, "sample_rate_hz", "project_sample_rate_hz"),
		firstPositiveMapInt(project, "record_bit_depth", "project_record_bit_depth", "bit_depth"),
		firstMapText(project, "pcm_format", "record_file_type"),
	)
	status := strings.ToLower(firstNonEmpty(firstMapText(project, "status"), firstMapText(result, "status")))
	if status == "" {
		status = "unknown"
	}

	lines := []string{"工程黑板状态汇报", "", "结论"}
	if trackCount > 0 || clipCount > 0 {
		appendMessageLoopBullet(&lines, "当前工程快照可读；可以继续选择 A-F 任一后续任务，但未记录的阶段不会被当成阻塞。")
	} else {
		appendMessageLoopBullet(&lines, "当前工程快照可读，但没有发现可汇报的音频轨或 clip；建议先导入素材或刷新工程状态。")
	}

	lines = append(lines, "", "工程概况")
	appendMessageLoopBullet(&lines, fmt.Sprintf("轨道/片段：%d / %d", trackCount, clipCount))
	if folderCount > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("文件夹轨道：%d", folderCount))
	}
	if markerCount > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("Marker/段落标记：%d", markerCount))
	}
	if hasDuration {
		appendMessageLoopBullet(&lines, fmt.Sprintf("工程时长估计：%.2fs", duration))
	}
	if projectSpec != "" {
		appendMessageLoopBullet(&lines, "工程规格："+projectSpec)
	}

	lines = append(lines, "", "A-F 状态矩阵")
	if trackCount > 0 || clipCount > 0 {
		appendMessageLoopBullet(&lines, "A 工程准备：当前工程快照可读；TIM/TOM/A4/A5 的历史结果若未保存在本轮上下文中则标记为尚未记录")
	} else {
		appendMessageLoopBullet(&lines, "A 工程准备：未发现可操作音频素材")
	}
	messageLoopAppendStaticMixCapabilityStatusLines(&lines, state)
	appendMessageLoopBullet(&lines, "C 细混处理：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "D 母带前检查：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "E 母带：尚未记录为已执行")
	appendMessageLoopBullet(&lines, "F 混音审查：尚未记录为已执行")

	lines = append(lines, "", "已完成/已观察")
	appendMessageLoopBullet(&lines, "已读取当前 project.state 快照")
	if trackCount > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("当前可见工程包含 %d 条轨道", trackCount))
	}
	if clipCount > 0 {
		appendMessageLoopBullet(&lines, fmt.Sprintf("当前可见工程包含 %d 个 clip", clipCount))
	}
	if markerCount > 0 {
		appendMessageLoopBullet(&lines, "A5 Marker/段落数据在快照中可读")
	}
	if state != nil && len(state.executionMemory.PendingTrackOrganization) > 0 {
		appendMessageLoopBullet(&lines, "本会话存在待确认 TOM/A3 轨道整理方案")
	}
	if state != nil && len(state.executionMemory.PendingSectionMarkers) > 0 {
		appendMessageLoopBullet(&lines, "本会话存在待确认 A5 段落 Marker 方案")
	}
	if state != nil && state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		appendMessageLoopBullet(&lines, "本会话存在最近一次混音声学观察")
	}

	lines = append(lines, "", "风险与遗留")
	if status != "ok" && status != "ready" && status != "unknown" {
		appendMessageLoopBullet(&lines, "工程快照状态："+status)
	}
	appendMessageLoopBullet(&lines, "TIM/TOM、片段清理、Marker 写入、B-F 混音处理的已执行结论需要来自对应 observation/action result；当前快照没有保存的项目显示为尚未记录")
	if trackCount == 0 && clipCount == 0 {
		appendMessageLoopBullet(&lines, "阻塞：没有可操作音频素材")
	}

	lines = append(lines, "", "已知用户目标")
	goalText := messageLoopProjectBlackboardKnownGoal(state)
	if goalText == "" {
		appendMessageLoopBullet(&lines, "尚未记录明确的混音参考方向、核心元素优先级或处理范围")
	} else {
		appendMessageLoopBullet(&lines, goalText)
	}

	lines = append(lines, "", "可选下一步")
	if trackCount == 0 && clipCount == 0 {
		appendMessageLoopBullet(&lines, "先导入多轨素材或刷新工程快照")
	} else {
		appendMessageLoopBullet(&lines, "继续工程准备复查：TIM/TOM、片段清理、Marker/段落地图")
		appendMessageLoopBullet(&lines, "进入 B1 Gain Staging 或 B2 静态平衡")
		appendMessageLoopBullet(&lines, "按用户目标跳到人声、鼓组、低频、空间或导出前检查；A-F 不作为线性门禁")
	}

	lines = append(lines, "", "数据来源")
	appendMessageLoopBullet(&lines, "project.state")
	if state != nil && (len(state.executionMemory.PendingTrackOrganization) > 0 || len(state.executionMemory.PendingSectionMarkers) > 0) {
		appendMessageLoopBullet(&lines, "execution memory")
	}
	if state != nil && state.recentObservation != nil {
		appendMessageLoopBullet(&lines, "recent observation")
	}
	return strings.Join(lines, "\n")
}

func messageLoopProjectBlackboardProjectState(result map[string]any) map[string]any {
	if project := messageLoopMapValue(result["project_state"]); len(project) > 0 {
		return project
	}
	if project := messageLoopMapValue(result["project"]); len(project) > 0 {
		return project
	}
	return result
}

func messageLoopProjectBlackboardTracks(project map[string]any) []map[string]any {
	if rows := messageLoopMapRows(project["tracks"]); len(rows) > 0 {
		return rows
	}
	if rows := messageLoopMapRows(project["visible_tracks"]); len(rows) > 0 {
		return rows
	}
	if shadow := messageLoopMapValue(project["shadow"]); len(shadow) > 0 {
		return messageLoopMapRows(shadow["tracks"])
	}
	return nil
}

func messageLoopProjectBlackboardClipCount(tracks []map[string]any) int {
	count := 0
	for _, track := range tracks {
		if clipCount := firstPositiveMapInt(track, "clip_count", "audio_clip_count"); clipCount > 0 {
			count += clipCount
			continue
		}
		count += messageLoopListCount(track["clips"])
	}
	return count
}

func messageLoopProjectBlackboardFolderCount(tracks []map[string]any) int {
	count := 0
	for _, track := range tracks {
		if messageLoopProjectStateTrackIsFolder(track) {
			count++
		}
	}
	return count
}

func messageLoopProjectBlackboardMarkerCount(project map[string]any) int {
	if count := firstPositiveMapInt(project, "marker_count", "section_marker_count"); count > 0 {
		return count
	}
	if count := messageLoopCountRowsOrList(project["markers"]); count > 0 {
		return count
	}
	if count := messageLoopCountRowsOrList(project["section_markers"]); count > 0 {
		return count
	}
	return 0
}

func messageLoopProjectBlackboardDuration(project map[string]any, tracks []map[string]any) (float64, bool) {
	if value, ok := firstNumericMapValue(project, "duration_seconds", "edit_length_seconds", "timeline_length_seconds", "length_seconds"); ok && value > 0 {
		return value, true
	}
	maxEnd := 0.0
	for _, track := range tracks {
		for _, clip := range messageLoopMapRows(track["clips"]) {
			start, _ := firstNumericMapValue(clip, "start_seconds", "start_time", "start")
			length, hasLength := firstNumericMapValue(clip, "length_seconds", "duration_seconds", "length")
			end, hasEnd := firstNumericMapValue(clip, "end_seconds", "end_time", "end")
			if !hasEnd && hasLength {
				end = start + length
				hasEnd = true
			}
			if hasEnd && end > maxEnd {
				maxEnd = end
			}
		}
	}
	if maxEnd > 0 {
		return maxEnd, true
	}
	return 0, false
}

func messageLoopProjectBlackboardKnownGoal(state *runState) string {
	if state == nil {
		return ""
	}
	for _, source := range []map[string]any{
		state.input.Context,
		state.input.State,
		state.contextSnapshot,
	} {
		if text := firstNonEmpty(
			firstMapText(source, "mix_goal", "mixing_goal", "user_mix_goal", "reference_direction", "priority_elements", "processing_scope"),
		); text != "" && !messageLoopProjectBlackboardStatusRequest(text) {
			return text
		}
	}
	return ""
}

func messageLoopProjectBlackboardExecutionResult(state *runState) map[string]any {
	if state == nil || len(state.executed) == 0 {
		return nil
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		name := strings.ToLower(strings.TrimSpace(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"]))))
		if name == "project.state" || name == "get_project_state" {
			return record
		}
	}
	return nil
}

func messageLoopProjectBlackboardFallbackRecord(call planner.ToolCall, errText string) map[string]any {
	return map[string]any{
		"tool_call_id": call.ID,
		"tool":         call.Tool,
		"status":       "error",
		"error":        firstNonEmpty(strings.TrimSpace(errText), "project.state did not produce an execution record"),
	}
}
