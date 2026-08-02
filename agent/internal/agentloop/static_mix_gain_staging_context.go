package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/levelsafety"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
)

func (l *MessageLoop) preflightStaticMixGainStagingContextPack(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	if state == nil || state.pendingToolCall != nil || len(state.pendingToolQueue) > 0 {
		return false, Result{}
	}
	if !messageLoopGainStagingCapabilityRequest(state.input.UserText) || messageLoopHasGainStagingContextPack(state) {
		return false, Result{}
	}
	if !messageLoopHasUsableMixObservation(state) && !messageLoopHasAnyMixObservationAttempt(state) {
		call := messageLoopGainStagingObservationCall(state)
		if allowedTool(call.Tool, state.input.AllowedTools) {
			if stopped, result := r.checkpoint("before_message_loop_gain_staging_observe", state); stopped {
				return true, result
			}
			if limit, result := r.checkToolBudget(state); limit {
				return true, result
			}
			state.trace = append(state.trace, planner.TraceEvent{
				Kind:     "tool_call_rewritten",
				Message:  "B1 gain staging request was routed through deterministic default mix.observe",
				ToolCall: cloneToolCallPtr(call),
			})
			toolStarted := time.Now()
			stopped, result := r.executeTool(ctx, state, call, false, nil)
			hasFeatureSnapshotArg := firstMapText(call.Args, "feature_snapshot_path", "mixboard_feature_snapshot_path") != ""
			l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_observe=true feature_snapshot_arg=%t stopped=%t status=%s", state.goal.GoalID, call.Tool, hasFeatureSnapshotArg, stopped, result.Status)
			if stopped {
				return true, result
			}
		}
	}
	if !messageLoopHasProjectStateExecution(state) && allowedTool("project.state", state.input.AllowedTools) {
		call := messageLoopProjectBlackboardStateCall()
		call.ID = "b1_gain_staging_project_state"
		call.Reason = "read project track and clip gain state for B1 gain staging context pack"
		if stopped, result := r.checkpoint("before_message_loop_gain_staging_project_state", state); stopped {
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "B1 gain staging context pack requested project.state track/clip state",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_project_state=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
	}
	if !messageLoopHasAudioAnalysisStatusExecution(state) && allowedTool("project.audio_analysis_status", state.input.AllowedTools) {
		call := messageLoopGainStagingAudioAnalysisStatusCall("b1_gain_staging_audio_analysis_status", "read automatic DAD waveform metrics for B1.2 source-level admission")
		if stopped, result := r.checkpoint("before_message_loop_gain_staging_audio_analysis_status", state); stopped {
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "B1 gain staging context pack requested project.audio_analysis_status DAD waveform metrics",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_audio_analysis_status=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
	}
	if messageLoopGainStagingRequiresSourceCalibration(state.input.UserText) &&
		messageLoopGainStagingAnalysisNeedsRecovery(messageLoopLastAudioAnalysisStatusResult(state)) &&
		allowedTool("project.audio_analysis_start", state.input.AllowedTools) {
		call := messageLoopGainStagingAudioAnalysisRetryCall(messageLoopLastAudioAnalysisStatusResult(state))
		if stopped, result := r.checkpoint("before_message_loop_gain_staging_audio_analysis_retry", state); stopped {
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "B1.2 restarted source DAD after clip edits invalidated submitted bake rows",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_audio_analysis_retry=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
		if stopped, result := l.messageLoopGainStagingWaitForAnalysisReady(ctx, r, state); stopped {
			return true, result
		}
	}
	pack := capabilitycontext.BuildGainStagingPack(capabilitycontext.GainStagingInput{
		UserIntent:          messageLoopGainStagingEffectivePackIntent(state.input.UserText),
		ProjectState:        messageLoopGainStagingProjectState(state),
		MixObservation:      messageLoopLastMixObservationResult(state),
		AudioAnalysisStatus: messageLoopLastAudioAnalysisStatusResult(state),
		ContextSnapshot:     state.contextSnapshot,
		RequestContext:      state.input.Context,
		ExecutionMemory:     executionMemoryMap(state.executionMemory),
		GeneratedAt:         r.now(),
	})
	if strings.TrimSpace(pack.PackID) == "" {
		return false, Result{}
	}
	faderStats := messageLoopGainStagingFaderUnityStats(state, pack)
	if messageLoopGainStagingFaderResetRequest(state.input.UserText) {
		if stopped, result := l.preflightStaticMixGainStagingFaderReset(ctx, r, state, pack, faderStats); stopped {
			messageLoopCompactB1PreflightState(state, pack)
			return true, result
		}
	}
	if messageLoopGainStagingRequiresSourceCalibration(state.input.UserText) && faderStats.NonUnityCount > 0 && !messageLoopGainStagingFullWorkflowRequest(state.input.UserText) {
		reply := messageLoopGainStagingB12NeedsUnityReply(faderStats)
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
		messageLoopCompactB1PreflightState(state, pack)
		return true, r.complete(state, reply)
	}
	if messageLoopGainStagingSuggestionRequest(state.input.UserText) {
		suggestions := capabilitycontext.BuildGainStagingSuggestions(pack)
		if stopped, result := l.preflightStaticMixGainStagingSuggestion(ctx, r, state, pack, suggestions, faderStats); stopped {
			messageLoopCompactB1PreflightState(state, pack)
			return true, result
		}
	}
	if messageLoopGainStagingReportRequest(state.input.UserText) {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1 gain staging technical calibration report completed"})
		messageLoopCompactB1PreflightState(state, pack)
		return true, r.complete(state, messageLoopGainStagingTechnicalReport(pack, faderStats))
	}
	messageLoopCompactB1PreflightState(state, pack)
	payload := map[string]any{
		"context_pack": pack.Map(),
		"instruction":  "This B1 default context pack is a deterministic starting context, not a tool-use restriction. Use allowed tools for additional observation if needed.",
	}
	data, _ := json.Marshal(payload)
	state.input.Conversation = append(state.input.Conversation, llm.Message{
		Role:    "user",
		Content: "<capability_context_pack>" + string(data) + "</capability_context_pack>",
	})
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:    "capability_context_pack",
		Message: capabilitycontext.GainStagingCapabilityID + " default pack built",
	})
	return false, Result{}
}

func (l *MessageLoop) preflightStaticMixGainStagingFaderReset(ctx context.Context, r *Runner, state *runState, pack capabilitycontext.Pack, stats messageLoopB1FaderUnityStats) (bool, Result) {
	if state == nil || messageLoopMutationBarrierActive(state) {
		return false, Result{}
	}
	if !allowedTool("track.group.apply_control", state.input.AllowedTools) {
		reply := "B1.1 推子归零需要 track.group.apply_control，但本轮工具上下文没有开放该工具；工程没有被修改。"
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
		return true, r.complete(state, reply)
	}
	trackIDs := messageLoopGainStagingResetTrackIDsFromStats(stats)
	if len(trackIDs) == 0 {
		reply := "B1.1 推子归零没有从 project.state 解析到可信的非 0 dB 目标轨道 ID；工程没有被修改。"
		if stats.EligibleCount > 0 && stats.NonUnityCount == 0 {
			reply = messageLoopGainStagingAlreadyUnityReply(stats)
		}
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
		return true, r.complete(state, reply)
	}
	groupID := messageLoopGainStagingResetGroupID(trackIDs)
	call := messageLoopGainStagingFaderResetToolCall(trackIDs, groupID, fmt.Sprintf("B1 reference-level calibration step 1: set %d non-unity track faders to 0 dB through a track group", len(trackIDs)))
	if stopped, result := r.checkpoint("before_message_loop_gain_staging_fader_reset", state); stopped {
		return true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return true, result
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "B1 gain staging fader reset was routed to track.group.apply_control pending confirmation",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_fader_reset=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		return true, result
	}
	appendMessageLoopToolResult(state)
	state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1 gain staging fader reset pending action completed"})
	return true, r.complete(state, messageLoopGainStagingFaderResetPendingReply(stats, groupID))
}

func messageLoopGainStagingResetTrackEligible(row map[string]any) bool {
	if len(row) == 0 || firstMapText(row, "track_id", "id") == "" {
		return false
	}
	if isMaster, ok := firstMapBool(row, "is_master_track", "master", "is_master"); ok && isMaster {
		return false
	}
	trackType := strings.ToLower(firstMapText(row, "track_type", "type", "kind"))
	if strings.Contains(trackType, "master") {
		return false
	}
	if isAudio, ok := firstMapBool(row, "is_audio_track", "is_audio", "audio_track"); ok && isAudio {
		return true
	}
	if isFolder, ok := firstMapBool(row, "is_folder_track", "is_folder_container", "can_contain_child_tracks"); ok && isFolder {
		return true
	}
	if _, ok := firstNumericMapValue(row, "volume_db", "fader_db", "track_gain_db", "gain_db", "db"); ok {
		return true
	}
	return strings.Contains(trackType, "audio") || strings.Contains(trackType, "folder") || strings.Contains(trackType, "track")
}

type messageLoopB1FaderRow struct {
	TrackID   string
	TrackName string
	VolumeDB  float64
}

type messageLoopB1FaderUnityStats struct {
	EligibleCount int
	KnownCount    int
	UnityCount    int
	NonUnityCount int
	SevereCount   int
	UnknownCount  int
	Rows          []messageLoopB1FaderRow
	NonUnityRows  []messageLoopB1FaderRow
	SevereRows    []messageLoopB1FaderRow
}

func messageLoopGainStagingFaderUnityStats(state *runState, pack capabilitycontext.Pack) messageLoopB1FaderUnityStats {
	stats := messageLoopB1FaderUnityStats{}
	rows := messageLoopGainStagingProjectTrackRows(state)
	if len(rows) > 0 {
		for _, row := range rows {
			if !messageLoopGainStagingResetTrackEligible(row) {
				continue
			}
			stats.EligibleCount++
			trackID := firstMapText(row, "track_id", "id")
			db, ok := firstNumericMapValue(row, "volume_db", "fader_db", "track_gain_db", "gain_db", "db")
			if !ok {
				stats.UnknownCount++
				continue
			}
			stats.KnownCount++
			item := messageLoopB1FaderRow{
				TrackID:   trackID,
				TrackName: firstMapText(row, "track_name", "name", "user_label"),
				VolumeDB:  db,
			}
			stats.Rows = append(stats.Rows, item)
			if messageLoopFaderAtUnity(db) {
				stats.UnityCount++
				continue
			}
			stats.NonUnityCount++
			stats.NonUnityRows = append(stats.NonUnityRows, item)
			if messageLoopFaderSeverelyOffset(db) {
				stats.SevereCount++
				stats.SevereRows = append(stats.SevereRows, item)
			}
		}
		return stats
	}
	for _, track := range pack.Tracks {
		if strings.Contains(strings.ToLower(strings.TrimSpace(track.TrackType)), "master") || strings.TrimSpace(track.TrackID) == "" {
			continue
		}
		stats.EligibleCount++
		if track.VolumeDB == nil {
			stats.UnknownCount++
			continue
		}
		stats.KnownCount++
		item := messageLoopB1FaderRow{TrackID: track.TrackID, TrackName: track.TrackName, VolumeDB: *track.VolumeDB}
		stats.Rows = append(stats.Rows, item)
		if messageLoopFaderAtUnity(*track.VolumeDB) {
			stats.UnityCount++
			continue
		}
		stats.NonUnityCount++
		stats.NonUnityRows = append(stats.NonUnityRows, item)
		if messageLoopFaderSeverelyOffset(*track.VolumeDB) {
			stats.SevereCount++
			stats.SevereRows = append(stats.SevereRows, item)
		}
	}
	return stats
}

func messageLoopGainStagingProjectTrackRows(state *runState) []map[string]any {
	projectState := messageLoopGainStagingProjectState(state)
	if rows := messageLoopMapRows(projectState["tracks"]); len(rows) > 0 {
		return rows
	}
	if rows := messageLoopMapRows(projectState["visible_tracks"]); len(rows) > 0 {
		return rows
	}
	daw := messageLoopMapValue(projectState["daw_state_summary"])
	if rows := messageLoopMapRows(daw["tracks"]); len(rows) > 0 {
		return rows
	}
	return nil
}

func messageLoopFaderAtUnity(db float64) bool {
	return mathAbs(db) <= 0.05
}

func messageLoopFaderSeverelyOffset(db float64) bool {
	return mathAbs(db) >= 6
}

func messageLoopGainStagingResetTrackIDsFromStats(stats messageLoopB1FaderUnityStats) []string {
	ids := make([]string, 0, len(stats.NonUnityRows))
	seen := map[string]bool{}
	for _, row := range stats.NonUnityRows {
		trackID := strings.TrimSpace(row.TrackID)
		if trackID == "" || seen[trackID] {
			continue
		}
		seen[trackID] = true
		ids = append(ids, trackID)
	}
	return ids
}

func messageLoopGainStagingResetGroupID(trackIDs []string) string {
	if len(trackIDs) == 0 {
		return "grp_b1_reference_level_reset"
	}
	return "grp_b1_reference_level_reset"
}

func messageLoopGainStagingFaderResetToolCall(trackIDs []string, groupID string, reason string) planner.ToolCall {
	args := map[string]any{
		"group_id":                groupID,
		"name":                    "B1 Reference Level Reset",
		"origin":                  "b1_gain_staging",
		"track_ids":               append([]string(nil), trackIDs...),
		"control":                 "volume",
		"mode":                    "absolute",
		"db":                      0.0,
		"create_group_if_missing": true,
		"replace_members":         true,
		"linked_controls": map[string]any{
			"volume": true,
			"pan":    false,
			"mute":   false,
			"solo":   false,
		},
	}
	command := cloneMap(args)
	command["cmd"] = "track.group.apply_control"
	return planner.ToolCall{
		ID:      "b1_reset_track_faders_to_unity",
		Tool:    "track.group.apply_control",
		Args:    cloneMap(args),
		Command: command,
		Reason:  firstNonEmpty(strings.TrimSpace(reason), "B1 reference-level calibration: set non-unity track faders to 0 dB through a track group"),
	}
}

func messageLoopGainStagingFaderResetPendingReply(stats messageLoopB1FaderUnityStats, groupID string) string {
	targetCount := stats.NonUnityCount
	if targetCount == 0 {
		targetCount = len(stats.NonUnityRows)
	}
	return fmt.Sprintf("B1.1 推子归零已生成待确认动作：已检查 %d 条可控轨道，其中 %d 条不在 0 dB，%d 条属于严重偏移。确认后会把 %d 条目标轨道放入编组 %s，并将这些轨道推子绝对设置为 0 dB。", stats.EligibleCount, stats.NonUnityCount, stats.SevereCount, targetCount, groupID)
}

func messageLoopGainStagingAlreadyUnityReply(stats messageLoopB1FaderUnityStats) string {
	return fmt.Sprintf("B1.1 推子归零检查完成：%d 条可控轨道推子都已经接近 0 dB。当前没有生成待确认修改。", stats.EligibleCount)
}

func (l *MessageLoop) preflightStaticMixGainStagingSuggestion(ctx context.Context, r *Runner, state *runState, pack capabilitycontext.Pack, suggestions capabilitycontext.GainStagingSuggestionSet, stats messageLoopB1FaderUnityStats) (bool, Result) {
	if state == nil || messageLoopMutationBarrierActive(state) {
		return false, Result{}
	}
	if suggestions.PrimaryAction == nil {
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1 gain staging suggestion completed with no action"})
		return true, r.complete(state, messageLoopGainStagingNoActionReply(pack, suggestions))
	}
	action := *suggestions.PrimaryAction
	switch action.ActionKind {
	case "track_fader_unity_reset":
		if !allowedTool("track.group.apply_control", state.input.AllowedTools) {
			reply := "B1 增益结构检查发现了非 0 dB 轨道推子，但本轮没有开放 track.group.apply_control；工程没有被修改。"
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
			return true, r.complete(state, reply)
		}
		trackIDs := messageLoopGainStagingResetTrackIDsFromStats(stats)
		if len(trackIDs) == 0 && len(action.TrackIDs) > 0 {
			trackIDs = append([]string(nil), action.TrackIDs...)
		}
		if len(trackIDs) == 0 {
			reply := "B1 增益结构检查在 context pack 中发现了推子偏移，但无法从 project.state 验证复位目标轨道 ID；工程没有被修改。"
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
			return true, r.complete(state, reply)
		}
		groupID := messageLoopGainStagingResetGroupID(trackIDs)
		call := messageLoopGainStagingFaderResetToolCall(trackIDs, groupID, "B1 gain staging suggestion: reset non-unity track faders to 0 dB before source/clip calibration")
		if stopped, result := r.checkpoint("before_message_loop_gain_staging_fader_unity_suggest", state); stopped {
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "B1 gain staging suggestion was routed to track.group.apply_control pending confirmation",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_fader_unity_suggest=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			return true, result
		}
		appendMessageLoopToolResult(state)
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1 gain staging fader unity suggestion completed"})
		return true, r.complete(state, messageLoopGainStagingFaderResetPendingReply(stats, groupID))
	case "clip_gain_set", "source_clip_gain_calibration", "source_clip_gain_calibration_batch":
		call := messageLoopGainStagingActionToolCall(action, suggestions.Actions, messageLoopGainStagingProjectState(state))
		if strings.TrimSpace(call.Tool) == "" {
			return true, r.complete(state, messageLoopGainStagingNoActionReply(pack, suggestions))
		}
		if !allowedTool(call.Tool, state.input.AllowedTools) {
			reply := fmt.Sprintf("B1 增益结构检查发现了 clip gain 风险，但本轮没有开放 %s；工程没有被修改。", call.Tool)
			state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: reply})
			return true, r.complete(state, reply)
		}
		if summary := messageLoopMapValue(call.Args["calibration_summary"]); len(summary) > 0 {
			action.Metadata = cloneMap(action.Metadata)
			if action.Metadata == nil {
				action.Metadata = map[string]any{}
			}
			for _, key := range []string{"track_action_count", "clip_action_count"} {
				if value, ok := summary[key]; ok {
					action.Metadata[key] = value
				}
			}
		}
		if stopped, result := r.checkpoint("before_message_loop_gain_staging_clip_gain_suggest", state); stopped {
			return true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return true, result
		}
		state.trace = append(state.trace, planner.TraceEvent{
			Kind:     "tool_call_rewritten",
			Message:  "B1 gain staging suggestion was routed to clip gain pending confirmation",
			ToolCall: cloneToolCallPtr(call),
		})
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_gain_staging_suggest=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
		if stopped {
			if messageLoopGainStagingIsSourceCalibrationAction(action) && result.StopReason == StopReasonNeedsConfirmation {
				result.Reply = messageLoopGainStagingSourceCalibrationPendingReply(pack, action)
			}
			return true, result
		}
		appendMessageLoopToolResult(state)
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1 gain staging clip gain suggestion completed"})
		if messageLoopGainStagingIsSourceCalibrationAction(action) {
			return true, r.complete(state, messageLoopGainStagingSourceCalibrationPendingReply(pack, action))
		}
		return true, r.complete(state, messageLoopGainStagingClipGainCompleteReply(action))
	default:
		return false, Result{}
	}
}

func messageLoopGainStagingSuggestionRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !messageLoopGainStagingCapabilityRequest(text) {
		return false
	}
	if messageLoopGainStagingFullWorkflowRequest(text) {
		return true
	}
	if messageLoopReadOnlyObservationRequest(text) {
		return false
	}
	if messageLoopGainStagingFaderResetRequest(text) {
		return true
	}
	if messageLoopGainStagingSourceCalibrationRequest(text) {
		return true
	}
	return messageLoopTextHasAny(text,
		"suggest", "recommend", "proposal", "candidate", "fix", "repair", "resolve", "handle", "process", "adjust", "optimize", "optimise", "apply", "execute", "do it", "go ahead", "next step",
		"calibrate", "calibration", "source calibration", "source level calibration",
		"prepare", "pending action", "pending actions", "batch", "clip.gain.set_batch", "clip gain set batch",
		"\u5efa\u8bae", "\u63a8\u8350", "\u65b9\u6848", "\u5019\u9009", "\u4fee\u590d", "\u4fee\u7406", "\u4fee\u4e00\u4e0b", "\u5904\u7406", "\u8c03\u6574", "\u4f18\u5316", "\u6574\u7406", "\u6267\u884c", "\u5e94\u7528", "\u5f00\u59cb", "\u4e0b\u4e00\u6b65", "\u600e\u4e48\u529e",
		"\u6821\u51c6", "\u6e90\u7d20\u6750\u6821\u51c6", "\u7d20\u6750\u6821\u51c6", "\u7535\u5e73\u8865\u507f", "\u8865\u507f",
		"\u51c6\u5907", "\u5f85\u786e\u8ba4\u52a8\u4f5c", "\u751f\u6210\u52a8\u4f5c", "\u6279\u91cf",
	)
}

func messageLoopGainStagingReportRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !messageLoopGainStagingCapabilityRequest(text) {
		return false
	}
	if messageLoopGainStagingFaderResetRequest(text) || messageLoopGainStagingSuggestionRequest(text) {
		return false
	}
	return messageLoopTextHasAny(text,
		"check", "scan", "audit", "inspect", "analyze", "analyse", "report", "status",
		"\u68c0\u67e5", "\u626b\u63cf", "\u5ba1\u8ba1", "\u67e5\u770b", "\u5206\u6790", "\u62a5\u544a", "\u72b6\u6001",
	)
}

func messageLoopGainStagingFaderResetRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || messageLoopReadOnlyObservationRequest(text) {
		return false
	}
	hasFader := messageLoopTextHasAny(text,
		"fader", "faders", "track volume", "track volumes", "track gain", "track gains",
		"\u63a8\u5b50", "\u8f68\u9053\u63a8\u5b50", "\u97f3\u91cf\u63a8\u5b50", "\u8f68\u9053\u97f3\u91cf", "\u8f68\u9053\u7535\u5e73",
	)
	hasZero := messageLoopTextHasAny(text,
		"0 db", "0db", "0 d b", "zero db", "unity", "unity gain", "set to 0", "reset to 0", "back to 0",
		"\u5f52\u96f6", "\u96f6\u70b9", "0\u70b9", "\u56de\u52300", "\u56de\u5230 0", "\u63a8\u56de0", "\u63a8\u56de 0", "\u56fa\u5b9a\u57280", "\u56fa\u5b9a\u5728 0", "\u8bbe\u52300", "\u8bbe\u5230 0", "\u8bbe\u4e3a0", "\u8bbe\u4e3a 0",
	)
	hasScope := messageLoopTextHasAny(text,
		"b1", "gain staging", "all tracks", "whole project", "entire project", "selected tracks", "these tracks", "those tracks",
		"abnormal", "outlier", "anomalous", "previously checked", "checked tracks",
		"\u5168\u90e8\u8f68\u9053", "\u6240\u6709\u8f68\u9053", "\u5168\u5de5\u7a0b", "\u6574\u4e2a\u5de5\u7a0b", "\u8fd9\u51e0\u4e2a\u8f68\u9053", "\u8fd9\u4e9b\u8f68\u9053", "\u76ee\u6807\u8f68\u9053", "\u589e\u76ca\u7ed3\u6784", "\u7535\u5e73", "\u5f02\u5e38", "\u5f02\u5e38\u63a8\u5b50", "\u5f02\u5e38\u8f68\u9053", "\u6b64\u524d\u68c0\u67e5", "\u68c0\u67e5\u51fa\u7684",
	)
	return hasFader && hasZero && hasScope
}

func messageLoopGainStagingSourceCalibrationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || !messageLoopGainStagingCapabilityRequest(text) {
		return false
	}
	if messageLoopGainStagingStrictReferenceIntent(text) {
		return true
	}
	return messageLoopTextHasAny(text,
		"b1.2", "b 1.2", "b1-2", "b 1-2", "source calibration", "source level calibration", "source gain", "clip calibration",
		"reference level", "same reference", "same range", "strict reference", "static source level",
		"\u6e90\u7d20\u6750", "\u7d20\u6750\u7535\u5e73", "\u6e90\u7d20\u6750\u6821\u51c6", "\u7d20\u6750\u6821\u51c6", "\u7535\u5e73\u8bc1\u636e", "\u7535\u5e73\u53c2\u8003\u503c",
		"\u53c2\u8003\u7535\u5e73", "\u53c2\u8003\u6307\u6807", "\u540c\u4e00\u53c2\u8003", "\u540c\u4e00\u6307\u6807", "\u7edf\u4e00\u7535\u5e73", "\u76f8\u540c\u8303\u56f4", "\u9759\u6001\u6e90\u7535\u5e73",
	)
}

func messageLoopGainStagingFullWorkflowRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" || messageLoopTextHasAny(text, "b1.1", "b 1.1", "b1-1", "b 1-1", "b1_1", "b1.2", "b 1.2", "b1-2", "b 1-2", "b1_2") {
		return false
	}
	if !messageLoopTextHasAny(text, "b1", "b 1", "static_mix.gain_staging", "gain staging", "gain-staging", "gainstage") {
		return false
	}
	if messageLoopReadOnlyObservationRequest(text) && !messageLoopTextHasAny(text,
		"apply", "calibrate", "calibration", "reset", "repair", "fix",
		"\u5f52\u96f6", "\u56de\u5f52", "\u6821\u51c6", "\u4fee\u590d", "\u4fee\u7406", "\u8c03\u6574",
	) {
		return false
	}
	return messageLoopTextHasAny(text,
		"execute", "run", "perform", "apply", "complete", "do b1", "start b1", "process",
		"\u6267\u884c", "\u8fdb\u884c", "\u5f00\u59cb", "\u5b8c\u6210", "\u5904\u7406", "\u505ab1", "\u505a b1", "\u5e2e\u6211\u505a",
		"\u5f52\u96f6", "\u56de\u5f52", "\u6821\u51c6", "\u4fee\u590d",
	)
}

func messageLoopGainStagingRequiresSourceCalibration(userText string) bool {
	return messageLoopGainStagingSourceCalibrationRequest(userText) || messageLoopGainStagingFullWorkflowRequest(userText)
}

func messageLoopGainStagingEffectivePackIntent(userText string) string {
	text := strings.TrimSpace(userText)
	if messageLoopGainStagingFullWorkflowRequest(text) {
		return text + " B1.2 full-project same-reference source calibration"
	}
	return text
}

func messageLoopGainStagingClipGainSetCall(action capabilitycontext.GainStagingSuggestion) planner.ToolCall {
	clipID := strings.TrimSpace(action.ClipID)
	if clipID == "" || action.TargetDB == nil {
		return planner.ToolCall{}
	}
	args := map[string]any{
		"clip_id":        clipID,
		"gain_db":        *action.TargetDB,
		"origin":         "b1_gain_staging",
		"b1_action_kind": action.ActionKind,
	}
	if action.TrackID != "" {
		args["track_id"] = action.TrackID
	}
	if len(action.Metadata) > 0 {
		args["b1_metadata"] = cloneMap(action.Metadata)
	}
	if action.ActionKind == "source_clip_gain_calibration" {
		args["origin"] = "b1_2_source_calibration"
	}
	command := cloneMap(args)
	command["cmd"] = "clip.gain.set"
	return planner.ToolCall{
		ID:      "b1_set_clip_gain",
		Tool:    "clip.gain.set",
		Args:    cloneMap(args),
		Command: command,
		Reason:  firstNonEmpty(strings.TrimSpace(action.Reason), "B1 gain staging clip gain correction"),
	}
}

func messageLoopGainStagingActionToolCall(action capabilitycontext.GainStagingSuggestion, actions []capabilitycontext.GainStagingSuggestion, projectState map[string]any) planner.ToolCall {
	if action.ActionKind == "source_clip_gain_calibration_batch" {
		return messageLoopGainStagingClipGainSetBatchCall(action, actions, projectState)
	}
	return messageLoopGainStagingClipGainSetCall(action)
}

func messageLoopGainStagingClipGainSetBatchCall(batch capabilitycontext.GainStagingSuggestion, actions []capabilitycontext.GainStagingSuggestion, projectState map[string]any) planner.ToolCall {
	sourceActions := messageLoopGainStagingSourceCalibrationActions(actions)
	if len(sourceActions) == 0 {
		return planner.ToolCall{}
	}
	pendingActions := make([]map[string]any, 0, len(sourceActions))
	seenClips := map[string]bool{}
	for _, action := range sourceActions {
		expanded := messageLoopGainStagingExpandTrackCalibration(action, projectState)
		for _, clipAction := range expanded {
			if seenClips[clipAction.ClipID] {
				continue
			}
			call := messageLoopGainStagingClipGainSetCall(clipAction)
			if strings.TrimSpace(call.Tool) == "" {
				continue
			}
			seenClips[clipAction.ClipID] = true
			pendingActions = append(pendingActions, map[string]any{
				"tool":        "clip.gain.set",
				"action_kind": clipAction.ActionKind,
				"track_id":    clipAction.TrackID,
				"track_name":  clipAction.TrackName,
				"clip_id":     clipAction.ClipID,
				"clip_name":   clipAction.ClipName,
				"args":        cloneMap(call.Args),
				"reason":      clipAction.Reason,
			})
		}
	}
	if len(pendingActions) == 0 {
		return planner.ToolCall{}
	}
	args := map[string]any{
		"origin":          "b1_2_source_calibration",
		"b1_action_kind":  "source_clip_gain_calibration_batch",
		"pending_actions": pendingActions,
		"calibration_summary": map[string]any{
			"action_count":       len(pendingActions),
			"target_count":       len(pendingActions),
			"track_action_count": len(sourceActions),
			"clip_action_count":  len(pendingActions),
			"source":             "static_mix.gain_staging.context_pack",
		},
	}
	if len(batch.Metadata) > 0 {
		args["b1_metadata"] = cloneMap(batch.Metadata)
	}
	command := cloneMap(args)
	command["cmd"] = "clip.gain.set_batch"
	return planner.ToolCall{
		ID:      "b1_set_clip_gain_batch",
		Tool:    "clip.gain.set_batch",
		Args:    cloneMap(args),
		Command: command,
		Reason:  firstNonEmpty(strings.TrimSpace(batch.Reason), "B1.2 full-project source level clip gain calibration batch"),
	}
}

func messageLoopGainStagingExpandTrackCalibration(action capabilitycontext.GainStagingSuggestion, projectState map[string]any) []capabilitycontext.GainStagingSuggestion {
	trackID := strings.TrimSpace(action.TrackID)
	if trackID == "" {
		return []capabilitycontext.GainStagingSuggestion{action}
	}
	delta := 0.0
	if action.DeltaDB != nil {
		delta = *action.DeltaDB
	} else if action.TargetDB != nil && action.CurrentDB != nil {
		delta = *action.TargetDB - *action.CurrentDB
	} else {
		return []capabilitycontext.GainStagingSuggestion{action}
	}
	tracks := messageLoopMapRows(projectState["tracks"])
	if len(tracks) == 0 {
		tracks = messageLoopMapRows(messageLoopMapValue(projectState["daw_state_summary"])["tracks"])
	}
	for _, track := range tracks {
		if firstMapText(track, "track_id", "id") != trackID {
			continue
		}
		clips := messageLoopMapRows(track["clips"])
		out := make([]capabilitycontext.GainStagingSuggestion, 0, len(clips))
		for _, clip := range clips {
			clipID := firstMapText(clip, "clip_id", "id", "item_id")
			clipType := strings.ToLower(firstMapText(clip, "clip_type", "type", "kind"))
			if clipID == "" || (clipType != "" && !strings.Contains(clipType, "wave") && !strings.Contains(clipType, "audio")) {
				continue
			}
			current, ok := firstNumericMapValue(clip, "clip_gain_db", "gain_db", "db")
			if !ok {
				continue
			}
			var sourcePeak *float64
			if value, ok := firstNumericMapValue(action.Metadata, "observed_source_peak_dbfs"); ok {
				sourcePeak = &value
			}
			constraint := levelsafety.ConstrainSourceClipGain(current, delta, 24, sourcePeak)
			target := math.Round(constraint.TargetClipGainDB*1000) / 1000
			currentRounded := math.Round(current*1000) / 1000
			deltaRounded := math.Round((target-currentRounded)*1000) / 1000
			next := action
			next.ClipID = clipID
			next.ClipName = firstMapText(clip, "clip_name", "name", "file_name")
			next.CurrentDB = &currentRounded
			next.TargetDB = &target
			next.DeltaDB = &deltaRounded
			next.Metadata = cloneMap(action.Metadata)
			if next.Metadata == nil {
				next.Metadata = map[string]any{}
			}
			next.Metadata["track_calibration_delta_db"] = delta
			next.Metadata["expanded_to_all_track_clips"] = true
			next.Metadata["target_clipped_to_bound"] = constraint.ClipGainBoundClamped
			next.Metadata["target_clipped_to_peak_safety"] = constraint.PeakSafetyClamped
			if constraint.ProjectedPeakDBFS != nil {
				next.Metadata["projected_static_peak_dbfs"] = math.Round(*constraint.ProjectedPeakDBFS*1000) / 1000
			}
			if constraint.PeakSafetyAchieved != nil {
				next.Metadata["peak_safety_achieved"] = *constraint.PeakSafetyAchieved
			}
			out = append(out, next)
		}
		if len(out) > 0 {
			return out
		}
		break
	}
	return []capabilitycontext.GainStagingSuggestion{action}
}

func messageLoopGainStagingSourceCalibrationActions(actions []capabilitycontext.GainStagingSuggestion) []capabilitycontext.GainStagingSuggestion {
	out := make([]capabilitycontext.GainStagingSuggestion, 0, len(actions))
	seen := map[string]bool{}
	for _, action := range actions {
		if action.ActionKind != "source_clip_gain_calibration" || strings.TrimSpace(action.ClipID) == "" || action.TargetDB == nil {
			continue
		}
		key := strings.TrimSpace(action.ClipID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, action)
	}
	return out
}

func messageLoopGainStagingIsSourceCalibrationAction(action capabilitycontext.GainStagingSuggestion) bool {
	return action.ActionKind == "source_clip_gain_calibration" || action.ActionKind == "source_clip_gain_calibration_batch"
}

func messageLoopGainStagingObservationIDFromPack(pack capabilitycontext.Pack) string {
	for _, ref := range pack.EvidenceRefs {
		if id, ok := strings.CutPrefix(strings.TrimSpace(ref), "mix.observe:"); ok {
			return strings.TrimSpace(id)
		}
	}
	return ""
}

func messageLoopGainStagingNoActionReply(pack capabilitycontext.Pack, suggestions capabilitycontext.GainStagingSuggestionSet) string {
	headroomCount := len(pack.Rankings["headroom_risk"])
	lowLevelCount := len(pack.Rankings["low_level"])
	clipGainCount := len(pack.Rankings["clip_gain_outliers"])
	trackGainCount := len(pack.Rankings["track_gain_outliers"])
	sourceCount := len(pack.Rankings["source_level_outliers"])
	reply := fmt.Sprintf("B1 增益结构检查没有生成需要立即执行的技术动作：源素材校准候选 %d，余量风险 %d，低电平风险 %d，clip gain 异常 %d，轨道推子异常 %d。当前没有生成待确认修改。", sourceCount, headroomCount, lowLevelCount, clipGainCount, trackGainCount)
	if len(suggestions.Limitations) > 0 {
		reply += "\n\n限制：" + strings.Join(suggestions.Limitations, ", ")
	}
	return reply
}

func messageLoopGainStagingClipGainCompleteReply(action capabilitycontext.GainStagingSuggestion) string {
	target := 0.0
	if action.TargetDB != nil {
		target = *action.TargetDB
	}
	return fmt.Sprintf("B1 \u589e\u76ca\u7ed3\u6784\u5df2\u5b8c\u6210 %s \u7684 clip gain \u8c03\u6574\uff1a\u76ee\u6807 %+0.2f dB\u3002", firstNonEmpty(action.ClipName, action.ClipID), target)
}

func messageLoopGainStagingB12NeedsUnityReply(stats messageLoopB1FaderUnityStats) string {
	return fmt.Sprintf("B1.2 源素材校准暂时不能继续：还有 %d 条可控轨道推子不在 0 dB。需要先完成 B1.1 推子归零，再刷新源素材电平证据并校准 clip gain。", stats.NonUnityCount)
}

func messageLoopGainStagingSourceCalibrationPendingReply(pack capabilitycontext.Pack, action capabilitycontext.GainStagingSuggestion) string {
	metric := firstNonEmpty(messageLoopMetadataText(action.Metadata, "reference_metric"), "level")
	unit := firstNonEmpty(messageLoopMetadataText(action.Metadata, "reference_unit"), "dB")
	reference, hasReference := messageLoopMetadataNumber(action.Metadata, "reference_level")
	observed, hasObserved := messageLoopMetadataNumber(action.Metadata, "observed_level")
	targetCount := 1
	if count, ok := messageLoopMetadataNumber(action.Metadata, "batch_action_count", "target_count"); ok && count > 0 {
		targetCount = int(count)
	}
	clipTargetCount := targetCount
	if count, ok := messageLoopMetadataNumber(action.Metadata, "clip_action_count"); ok && count > 0 {
		clipTargetCount = int(count)
	}
	trackTargetCount := targetCount
	if count, ok := messageLoopMetadataNumber(action.Metadata, "track_action_count"); ok && count > 0 {
		trackTargetCount = int(count)
	}
	lines := []string{"B1.2 源素材电平校准已生成待确认动作。", ""}
	lines = append(lines, "结论")
	if action.ActionKind == "source_clip_gain_calibration_batch" {
		lines = append(lines, fmt.Sprintf("- 待确认批量动作：校准 %d 条源轨，并将各轨校准差值展开到 %d 个存活 clip。", trackTargetCount, clipTargetCount))
	} else {
		target := 0.0
		if action.TargetDB != nil {
			target = *action.TargetDB
		}
		delta := 0.0
		if action.DeltaDB != nil {
			delta = *action.DeltaDB
		}
		lines = append(lines, fmt.Sprintf("- 目标 clip：%s。", firstNonEmpty(action.ClipName, action.ClipID)))
		lines = append(lines, fmt.Sprintf("- 待确认动作：将 clip gain 设置为 %+0.2f dB（变化 %+0.2f dB）。", target, delta))
	}
	if hasObserved && hasReference {
		lines = append(lines, fmt.Sprintf("- 参考指标：%s；当前 %.2f %s；工程参考 %.2f %s。", metric, observed, unit, reference, unit))
	} else {
		lines = append(lines, fmt.Sprintf("- 参考指标：%s。", metric))
	}
	if messageLoopMetadataBool(action.Metadata, "target_clipped_to_bound") {
		lines = append(lines, "- 注意：至少一个目标被限制在 v1 clip gain 边界内；极端素材后续可能需要 trim/input gain 工具。")
	}
	lines = append(lines, "- 这一步只校准源素材 clip gain；不会用轨道推子、轨道编组或 mix tick 做 B2 静态平衡。")
	lines = append(lines, "", "证据")
	lines = append(lines, messageLoopGainStagingEvidenceLine("track_acoustic", "峰值/RMS/LUFS/余量", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("b1_2_source_level_admission", "B1.2 源素材电平准入", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("clip_gain", "clip gain", pack))
	if len(pack.Limitations) > 0 {
		lines = append(lines, "", "限制")
		for i, item := range pack.Limitations {
			if i >= 4 {
				lines = append(lines, fmt.Sprintf("- 另有 %d 条限制已省略。", len(pack.Limitations)-i))
				break
			}
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	lines = append(lines, "", "确认后，VitAgent 会执行 clip gain 动作，并重新读取 project.state / mix.observe / project.audio_analysis_status 做验证。")
	return strings.Join(lines, "\n")
}
func messageLoopGainStagingTechnicalReport(pack capabilitycontext.Pack, stats messageLoopB1FaderUnityStats) string {
	lines := []string{"B1 技术增益结构报告", "", "结论"}
	lines = append(lines, "- B1 是粗混前的工程校准，不是 B2 音乐性音量平衡。")
	if stats.EligibleCount > 0 {
		lines = append(lines, fmt.Sprintf("- 可控轨道推子：%d；已接近 0 dB：%d；需要复位：%d；严重偏移：%d。", stats.EligibleCount, stats.UnityCount, stats.NonUnityCount, stats.SevereCount))
	} else {
		lines = append(lines, "- project.state 没有暴露可控轨道推子，因此 B1.1 复位目标暂时不可信。")
	}
	headroomCount := len(pack.Rankings["headroom_risk"])
	lowLevelCount := len(pack.Rankings["low_level"])
	clipGainCount := len(pack.Rankings["clip_gain_outliers"])
	sourceCount := len(pack.Rankings["source_level_outliers"])
	lines = append(lines, fmt.Sprintf("- 电平风险摘要：源素材校准候选 %d，余量风险 %d，低电平风险 %d，clip gain 异常 %d。", sourceCount, headroomCount, lowLevelCount, clipGainCount))
	if admission := messageLoopB12AdmissionMap(pack); len(admission) > 0 {
		status := firstMapText(admission, "status")
		metric := firstMapText(admission, "selected_metric")
		eligible, _ := firstNumericMapValue(admission, "eligible_track_count")
		covered, _ := firstNumericMapValue(admission, "covered_track_count", "candidate_count")
		missing, _ := firstNumericMapValue(admission, "missing_candidate_count")
		lines = append(lines, fmt.Sprintf("- B1.2 源素材电平准入：%s，metric=%s，覆盖 %.0f/%.0f，缺失 %.0f。", messageLoopGainStagingStatusLabel(firstNonEmpty(status, "missing")), firstNonEmpty(metric, "none"), covered, eligible, missing))
	}

	lines = append(lines, "", "证据")
	if len(stats.NonUnityRows) > 0 {
		lines = append(lines, "- 非 0 dB 推子目标：")
		for i, row := range stats.NonUnityRows {
			if i >= 8 {
				lines = append(lines, fmt.Sprintf("  - 另有 %d 条已省略。", len(stats.NonUnityRows)-i))
				break
			}
			label := firstNonEmpty(strings.TrimSpace(row.TrackName), row.TrackID)
			lines = append(lines, fmt.Sprintf("  - %s：当前 %+0.2f dB，B1 目标 0.00 dB。", label, row.VolumeDB))
		}
	} else if stats.EligibleCount > 0 {
		lines = append(lines, "- 已知可控轨道推子都已经接近 0 dB。")
	}
	lines = append(lines, messageLoopGainStagingEvidenceLine("track_gain", "轨道推子", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("clip_gain", "clip gain", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("track_acoustic", "峰值/RMS/余量", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("track_loudness", "LUFS/近似响度", pack))
	lines = append(lines, messageLoopGainStagingEvidenceLine("b1_2_source_level_admission", "B1.2 源素材电平准入", pack))
	if len(pack.Rankings["source_level_outliers"]) > 0 {
		lines = append(lines, "- B1.2 源素材校准候选：")
		for i, row := range pack.Rankings["source_level_outliers"] {
			if i >= 5 {
				lines = append(lines, fmt.Sprintf("  - 另有 %d 条已省略。", len(pack.Rankings["source_level_outliers"])-i))
				break
			}
			label := firstNonEmpty(row.TrackName, row.ClipName, row.TrackID, row.ClipID)
			target, _ := messageLoopMetadataNumber(row.Metadata, "target_clip_gain_db")
			current, _ := messageLoopMetadataNumber(row.Metadata, "current_clip_gain_db")
			observed, hasObserved := messageLoopMetadataNumber(row.Metadata, "observed_level")
			reference, hasReference := messageLoopMetadataNumber(row.Metadata, "reference_level")
			metric := firstNonEmpty(messageLoopMetadataText(row.Metadata, "reference_metric"), row.Kind)
			unit := firstNonEmpty(messageLoopMetadataText(row.Metadata, "reference_unit"), row.Unit)
			if hasObserved && hasReference {
				lines = append(lines, fmt.Sprintf("  - %s：%s %.2f %s，参考 %.2f %s，clip gain %+0.2f -> %+0.2f dB。", label, metric, observed, unit, reference, unit, current, target))
			} else {
				lines = append(lines, fmt.Sprintf("  - %s：clip gain %+0.2f -> %+0.2f dB。", label, current, target))
			}
		}
	}

	limitations := append([]string(nil), pack.Limitations...)
	if status := pack.EvidenceStatus["track_acoustic"]; status.Status == "missing" || status.Status == "partial" {
		limitations = append(limitations, "全工程声学覆盖缺失或只有部分可用")
	}
	if status := pack.EvidenceStatus["track_loudness"]; status.Status == "missing" || status.Status == "partial" {
		limitations = append(limitations, "全工程 LUFS 覆盖缺失或只有部分可用；B1.2 只能在 RMS/peak 全覆盖时降级使用")
	}
	lines = append(lines, "", "限制")
	if len(limitations) == 0 {
		lines = append(lines, "- 当前 context pack 没有发现 B1 阻断项。")
	} else {
		for i, item := range limitations {
			if i >= 6 {
				lines = append(lines, fmt.Sprintf("- 另有 %d 条限制已省略。", len(limitations)-i))
				break
			}
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}

	lines = append(lines, "", "建议")
	if stats.NonUnityCount > 0 {
		lines = append(lines, fmt.Sprintf("- 生成 B1.1 待确认动作：将 %d 条非 0 dB 轨道推子设置为 0 dB。执行前必须等待用户确认。", stats.NonUnityCount))
	} else if sourceCount > 0 {
		lines = append(lines, fmt.Sprintf("- 为 %d 个源素材电平候选生成 B1.2 clip.gain.set_batch 待确认动作。不要使用轨道推子或 mix tick。", sourceCount))
	} else {
		lines = append(lines, "- B1.1 推子归零已满足；进入 B2 静态平衡前，继续补齐客观源素材 clip gain/trim 证据。")
	}
	return strings.Join(lines, "\n")
}
func messageLoopGainStagingEvidenceLine(key, label string, pack capabilitycontext.Pack) string {
	status := pack.EvidenceStatus[key]
	if strings.TrimSpace(status.Status) == "" {
		return fmt.Sprintf("- %s 证据：缺失。", label)
	}
	if status.TotalCount > 0 {
		return fmt.Sprintf("- %s 证据：%s（%d/%d）。", label, messageLoopGainStagingStatusLabel(status.Status), status.KnownCount, status.TotalCount)
	}
	return fmt.Sprintf("- %s 证据：%s。", label, messageLoopGainStagingStatusLabel(status.Status))
}

func messageLoopGainStagingStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready":
		return "就绪"
	case "partial":
		return "部分可用"
	case "missing":
		return "缺失"
	case "stale":
		return "过期"
	case "building", "requested":
		return "准备中"
	case "failed", "error":
		return "失败"
	default:
		return firstNonEmpty(strings.TrimSpace(status), "缺失")
	}
}

func messageLoopB12AdmissionMap(pack capabilitycontext.Pack) map[string]any {
	return messageLoopMapValue(pack.Summary["b1_2_source_level_admission"])
}

func messageLoopMetadataText(metadata map[string]any, keys ...string) string {
	if len(metadata) == 0 {
		return ""
	}
	return firstMapText(metadata, keys...)
}

func messageLoopMetadataNumber(metadata map[string]any, keys ...string) (float64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	return firstNumericMapValue(metadata, keys...)
}

func messageLoopMetadataBool(metadata map[string]any, keys ...string) bool {
	if len(metadata) == 0 {
		return false
	}
	if value, ok := firstMapBool(metadata, keys...); ok {
		return value
	}
	return false
}

func messageLoopIsB1FaderResetCall(call planner.ToolCall) bool {
	if strings.TrimSpace(call.Tool) != "track.group.apply_control" {
		return false
	}
	origin := strings.ToLower(strings.TrimSpace(firstMapText(call.Args, "origin")))
	groupID := strings.TrimSpace(firstMapText(call.Args, "group_id", "id"))
	actionKind := strings.ToLower(strings.TrimSpace(firstMapText(call.Args, "b1_action_kind", "action_kind")))
	callID := strings.TrimSpace(call.ID)
	isB1 := origin == "b1_gain_staging" ||
		actionKind == "track_fader_unity_reset" ||
		groupID == "grp_b1_reference_level_reset" ||
		callID == "b1_reset_track_faders_to_unity"
	if !isB1 {
		return false
	}
	mode := strings.ToLower(firstNonEmpty(firstMapText(call.Args, "mode", "operation"), "absolute"))
	control := strings.ToLower(firstNonEmpty(firstMapText(call.Args, "control", "param", "parameter"), "volume"))
	targetDB, targetOK := firstNumericMapValue(call.Args, "db", "target_db", "value_db", "volume_db")
	if control != "volume" && control != "track.volume" {
		return false
	}
	if mode != "absolute" && mode != "set" && mode != "volume_absolute" {
		return false
	}
	return targetOK && mathAbs(targetDB) <= 0.0001
}

func (l *MessageLoop) messageLoopB1FaderResetCompleteOrchestrate(ctx context.Context, r *Runner, state *runState, call planner.ToolCall) (string, bool, Result) {
	if !messageLoopIsB1FaderResetCall(call) {
		return "", false, Result{}
	}
	if state == nil || len(state.executed) == 0 {
		return "", true, r.fail(state, fmt.Errorf("B1 fader reset verification could not find the confirmed execution record"))
	}
	record := state.executed[len(state.executed)-1]
	if !messageLoopExecutionSucceeded(record) {
		errText := firstNonEmpty(messageLoopText(record["error"]), "confirmed B1 fader reset failed")
		return "", true, r.fail(state, fmt.Errorf("%s", errText))
	}

	ranProjectState := false
	ranObserve := false
	ranAudioStatus := false
	if allowedTool("project.state", state.input.AllowedTools) {
		verifyCall := messageLoopProjectBlackboardStateCall()
		verifyCall.ID = "b1_3_after_fader_reset_project_state"
		verifyCall.Reason = "verify B1.1 fader unity reset and prepare B1.2 source calibration through refreshed project.state"
		if stopped, result := r.checkpoint("before_b1_3_after_fader_reset_project_state", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_3_verify_project_state=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		appendMessageLoopToolResult(state)
		ranProjectState = true
	}
	if allowedTool("mix.observe", state.input.AllowedTools) || allowedTool("mix.request_observation", state.input.AllowedTools) {
		verifyCall := messageLoopGainStagingObservationCall(state)
		verifyCall.ID = "b1_3_after_fader_reset_observe"
		verifyCall.Reason = "re-observe project gain/headroom evidence after B1.1 fader unity reset for B1.2 admission"
		if stopped, result := r.checkpoint("before_b1_3_after_fader_reset_observe", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_3_verify_observe=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		appendMessageLoopToolResult(state)
		ranObserve = true
	}
	if allowedTool("project.audio_analysis_status", state.input.AllowedTools) {
		verifyCall := messageLoopGainStagingAudioAnalysisStatusCall("b1_3_after_fader_reset_audio_analysis_status", "refresh automatic DAD waveform metrics after B1.1 fader unity reset for B1.2 admission")
		if stopped, result := r.checkpoint("before_b1_3_after_fader_reset_audio_analysis_status", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_3_verify_audio_analysis_status=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		appendMessageLoopToolResult(state)
		ranAudioStatus = true
	}

	pack := capabilitycontext.BuildGainStagingPack(capabilitycontext.GainStagingInput{
		UserIntent:          messageLoopGainStagingEffectivePackIntent(state.input.UserText),
		ProjectState:        messageLoopGainStagingProjectState(state),
		MixObservation:      messageLoopLastMixObservationResult(state),
		AudioAnalysisStatus: messageLoopLastAudioAnalysisStatusResult(state),
		ContextSnapshot:     state.contextSnapshot,
		RequestContext:      state.input.Context,
		ExecutionMemory:     executionMemoryMap(state.executionMemory),
		GeneratedAt:         r.now(),
	})
	stats := messageLoopGainStagingFaderUnityStats(state, pack)
	suggestions := capabilitycontext.BuildGainStagingSuggestions(pack)
	messageLoopCompactB1PreflightState(state, pack)

	if stats.NonUnityCount > 0 {
		return messageLoopGainStagingB13CompleteReply(pack, suggestions, stats, ranProjectState, ranObserve, ranAudioStatus, "fader_reset_not_verified"), true, Result{}
	}
	if suggestions.PrimaryAction == nil || !messageLoopGainStagingIsSourceCalibrationAction(*suggestions.PrimaryAction) {
		return messageLoopGainStagingB13CompleteReply(pack, suggestions, stats, ranProjectState, ranObserve, ranAudioStatus, "no_source_calibration_action"), true, Result{}
	}
	action := *suggestions.PrimaryAction
	call = messageLoopGainStagingActionToolCall(action, suggestions.Actions, messageLoopGainStagingProjectState(state))
	if strings.TrimSpace(call.Tool) == "" {
		return messageLoopGainStagingB13CompleteReply(pack, suggestions, stats, ranProjectState, ranObserve, ranAudioStatus, "no_source_calibration_action"), true, Result{}
	}
	if !allowedTool(call.Tool, state.input.AllowedTools) {
		reply := messageLoopGainStagingB13CompleteReply(pack, suggestions, stats, ranProjectState, ranObserve, ranAudioStatus, "clip_gain_set_not_allowed")
		state.trace = append(state.trace, planner.TraceEvent{Kind: "final_gate", Message: "B1.3 source calibration blocked because clip gain tool is unavailable"})
		return reply, true, Result{}
	}
	if stopped, result := r.checkpoint("before_b1_3_source_calibration_pending", state); stopped {
		return "", true, result
	}
	if limit, result := r.checkToolBudget(state); limit {
		return "", true, result
	}
	state.trace = append(state.trace, planner.TraceEvent{
		Kind:     "tool_call_rewritten",
		Message:  "B1.3 routed verified fader reset into B1.2 clip gain pending confirmation",
		ToolCall: cloneToolCallPtr(call),
	})
	toolStarted := time.Now()
	stopped, result := r.executeTool(ctx, state, call, false, nil)
	l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_3_source_calibration_pending=true stopped=%t status=%s", state.goal.GoalID, call.Tool, stopped, result.Status)
	if stopped {
		if result.StopReason == StopReasonNeedsConfirmation {
			result.Reply = messageLoopGainStagingB13PendingReply(pack, action, stats, ranProjectState, ranObserve, ranAudioStatus)
		}
		return "", true, result
	}
	appendMessageLoopToolResult(state)
	return messageLoopGainStagingB13PendingReply(pack, action, stats, ranProjectState, ranObserve, ranAudioStatus), true, Result{}
}

func messageLoopGainStagingB13PendingReply(pack capabilitycontext.Pack, action capabilitycontext.GainStagingSuggestion, stats messageLoopB1FaderUnityStats, ranProjectState, ranObserve, ranAudioStatus bool) string {
	lines := []string{"B1.3 已在确认 B1.1 推子归零后继续推进全工程源素材校准。", ""}
	lines = append(lines, "验证")
	if ranProjectState {
		lines = append(lines, fmt.Sprintf("- 已刷新 project.state：可控轨道 %d，仍非 0 dB %d。", stats.EligibleCount, stats.NonUnityCount))
	} else {
		lines = append(lines, "- 本轮没有开放 project.state，只能使用当前执行上下文。")
	}
	if ranObserve {
		lines = append(lines, "- 已重新运行 mix.observe，用于 B1.2 源素材电平准入。")
	} else {
		lines = append(lines, "- 本轮没有开放 mix.observe。")
	}
	if ranAudioStatus {
		lines = append(lines, "- 已读取 project.audio_analysis_status，用于补齐全工程 DAD 波形指标。")
	} else {
		lines = append(lines, "- 本轮没有刷新 project.audio_analysis_status；如 RMS/peak 覆盖不足，B1.2 会保持阻断。")
	}
	lines = append(lines, "", messageLoopGainStagingSourceCalibrationPendingReply(pack, action))
	return strings.Join(lines, "\n")
}

func messageLoopGainStagingB13CompleteReply(pack capabilitycontext.Pack, suggestions capabilitycontext.GainStagingSuggestionSet, stats messageLoopB1FaderUnityStats, ranProjectState, ranObserve, ranAudioStatus bool, reason string) string {
	lines := []string{"B1.3 全工程源素材校准编排已完成本轮验证。", ""}
	lines = append(lines, "验证")
	if ranProjectState {
		lines = append(lines, fmt.Sprintf("- 已刷新 project.state：可控轨道 %d，已归零 %d，仍非 0 dB %d。", stats.EligibleCount, stats.UnityCount, stats.NonUnityCount))
	} else {
		lines = append(lines, "- 本轮没有开放 project.state。")
	}
	if ranObserve {
		lines = append(lines, "- 已重新运行 mix.observe。")
	} else {
		lines = append(lines, "- 本轮没有开放 mix.observe。")
	}
	if ranAudioStatus {
		lines = append(lines, "- 已读取 project.audio_analysis_status。")
	} else {
		lines = append(lines, "- 本轮没有刷新 project.audio_analysis_status。")
	}
	lines = append(lines, messageLoopGainStagingEvidenceLine("b1_2_source_level_admission", "B1.2 源素材电平准入", pack))
	switch reason {
	case "fader_reset_not_verified":
		lines = append(lines, "", fmt.Sprintf("结论：B1.1 尚未完全验证，还有 %d 条可控轨道推子不在 0 dB，因此没有排队 B1.2 clip gain 校准。", stats.NonUnityCount))
	case "clip_gain_set_not_allowed":
		lines = append(lines, "", "结论：B1.1 已验证，但本轮没有开放所需的 clip gain 工具，因此没有生成 B1.2 待确认修改。")
	default:
		lines = append(lines, "", "结论：B1.1 已验证，但刷新后的全工程证据没有生成 B1.2 源素材 clip gain 动作。")
		if suggestions.PrimaryAction != nil {
			lines = append(lines, fmt.Sprintf("- 当前主建议动作是 %s。", suggestions.PrimaryAction.ActionKind))
		}
	}
	if len(pack.Limitations) > 0 {
		lines = append(lines, "", "限制")
		for i, item := range pack.Limitations {
			if i >= 4 {
				lines = append(lines, fmt.Sprintf("- 另有 %d 条限制已省略。", len(pack.Limitations)-i))
				break
			}
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	return strings.Join(lines, "\n")
}
func messageLoopIsB12SourceCalibrationCall(call planner.ToolCall) bool {
	origin := strings.ToLower(strings.TrimSpace(firstMapText(call.Args, "origin")))
	actionKind := strings.ToLower(strings.TrimSpace(firstMapText(call.Args, "b1_action_kind", "action_kind")))
	tool := strings.TrimSpace(call.Tool)
	if tool != "clip.gain.set" && tool != "clip.gain.set_batch" {
		return false
	}
	return origin == "b1_2_source_calibration" ||
		actionKind == "source_clip_gain_calibration" ||
		actionKind == "source_clip_gain_calibration_batch"
}

type messageLoopB12SourceCalibrationTarget struct {
	ClipID       string
	TrackID      string
	TargetGainDB float64
}

func (l *MessageLoop) messageLoopB12SourceCalibrationCompleteReply(ctx context.Context, r *Runner, state *runState, call planner.ToolCall) (string, bool, Result) {
	if !messageLoopIsB12SourceCalibrationCall(call) {
		return "", false, Result{}
	}
	if state == nil || len(state.executed) == 0 {
		return "", true, r.fail(state, fmt.Errorf("B1.2 source calibration verification could not find the confirmed execution record"))
	}
	record := state.executed[len(state.executed)-1]
	if !messageLoopExecutionSucceeded(record) {
		errText := firstNonEmpty(messageLoopText(record["error"]), "confirmed B1.2 source calibration failed")
		return "", true, r.fail(state, fmt.Errorf("%s", errText))
	}
	ranProjectState := false
	ranObserve := false
	ranAudioStatus := false
	if allowedTool("project.state", state.input.AllowedTools) {
		verifyCall := messageLoopProjectBlackboardStateCall()
		verifyCall.ID = "b1_2_source_calibration_verify_project_state"
		verifyCall.Reason = "verify B1.2 source calibration through refreshed project.state"
		if stopped, result := r.checkpoint("before_b1_2_source_calibration_project_state_verify", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_2_verify_project_state=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		appendMessageLoopToolResult(state)
		ranProjectState = true
	}
	if allowedTool("project.audio_analysis_status", state.input.AllowedTools) {
		verifyCall := messageLoopGainStagingAudioAnalysisStatusCall(
			"b1_2_source_calibration_verify_audio_analysis_status",
			"refresh full-project DAD source facts after B1.2 source calibration",
		)
		if stopped, result := r.checkpoint("before_b1_2_source_calibration_audio_analysis_status_verify", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_2_verify_audio_analysis_status=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		if len(state.executed) == 0 || !messageLoopExecutionSucceeded(state.executed[len(state.executed)-1]) {
			return "", true, r.fail(state, fmt.Errorf("B1.2 source calibration could not refresh project.audio_analysis_status"))
		}
		appendMessageLoopToolResult(state)
		status := messageLoopMapValue(state.executed[len(state.executed)-1]["result"])
		if incomplete, detail := messageLoopGainStagingAnalysisExplicitlyIncomplete(status); incomplete {
			return "", true, r.fail(state, fmt.Errorf("B1.2 source calibration DAD verification is incomplete: %s", detail))
		}
		ranAudioStatus = true
	}
	if allowedTool("mix.observe", state.input.AllowedTools) || allowedTool("mix.request_observation", state.input.AllowedTools) {
		verifyCall := messageLoopGainStagingObservationCall(state)
		verifyCall.ID = "b1_2_source_calibration_verify_observe"
		verifyCall.Reason = "re-observe project gain/headroom evidence after B1.2 source calibration"
		if stopped, result := r.checkpoint("before_b1_2_source_calibration_observe_verify", state); stopped {
			return "", true, result
		}
		if limit, result := r.checkToolBudget(state); limit {
			return "", true, result
		}
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, verifyCall, false, nil)
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_2_verify_observe=true stopped=%t status=%s", state.goal.GoalID, verifyCall.Tool, stopped, result.Status)
		if stopped {
			return "", true, result
		}
		appendMessageLoopToolResult(state)
		ranObserve = true
	}
	pack := capabilitycontext.BuildGainStagingPack(capabilitycontext.GainStagingInput{
		UserIntent:          messageLoopGainStagingEffectivePackIntent(state.input.UserText),
		ProjectState:        messageLoopGainStagingProjectState(state),
		MixObservation:      messageLoopLastMixObservationResult(state),
		AudioAnalysisStatus: messageLoopLastAudioAnalysisStatusResult(state),
		ContextSnapshot:     state.contextSnapshot,
		RequestContext:      state.input.Context,
		ExecutionMemory:     executionMemoryMap(state.executionMemory),
		GeneratedAt:         r.now(),
	})
	targets := messageLoopB12SourceCalibrationTargets(call)
	resultRows := messageLoopB12ClipGainResultRows(record)
	if len(targets) == 0 {
		clipID := firstMapText(call.Args, "clip_id")
		targetGain, _ := firstNumericMapValue(call.Args, "gain_db", "clip_gain_db", "db")
		targets = append(targets, messageLoopB12SourceCalibrationTarget{ClipID: clipID, TrackID: firstMapText(call.Args, "track_id"), TargetGainDB: targetGain})
	}
	lines := []string{"B1.2 源素材 clip gain 校准已执行。", ""}
	lines = append(lines, "验证")
	if issue := messageLoopB12ClipGainVerificationIssue(state, resultRows, targets, ranProjectState); issue != "" {
		return "", true, r.fail(state, fmt.Errorf("B1.2 source calibration write verification failed: %s", issue))
	}
	if unsafe := messageLoopB12UnsafeTargetPeaks(pack, targets); len(unsafe) > 0 {
		parts := make([]string, 0, len(unsafe))
		for i, row := range unsafe {
			if i >= 8 {
				parts = append(parts, fmt.Sprintf("and %d more", len(unsafe)-i))
				break
			}
			parts = append(parts, fmt.Sprintf("track=%s peak=%+.3f dBFS", firstNonEmpty(row.TrackName, row.TrackID, row.ClipID), row.PeakDBFS))
		}
		return "", true, r.fail(state, fmt.Errorf("B1.2 source calibration acoustic verification failed: effective static peak exceeds the %+.1f dBFS safety ceiling (%s)", levelsafety.StaticPeakCeilingDBFS, strings.Join(parts, "; ")))
	}
	verifiedCount := 0
	for i, target := range targets {
		if i >= 8 {
			lines = append(lines, fmt.Sprintf("- 另有 %d 条 clip gain 验证结果已省略。", len(targets)-i))
			break
		}
		verifiedGain, hasVerifiedGain := messageLoopB12ClipGainFromProjectState(state, target.ClipID)
		if !hasVerifiedGain {
			verifiedGain, hasVerifiedGain = messageLoopB12ClipGainFromResultRows(resultRows, target.ClipID)
		}
		label := firstNonEmpty(target.ClipID, target.TrackID, "target clip")
		if hasVerifiedGain {
			verifiedCount++
			status := "已验证"
			if mathAbs(verifiedGain-target.TargetGainDB) > 0.01 {
				status = "不一致"
			}
			lines = append(lines, fmt.Sprintf("- project.state clip gain：%s，%s = %+0.2f dB（目标 %+0.2f dB）。", status, label, verifiedGain, target.TargetGainDB))
		} else if ranProjectState {
			lines = append(lines, fmt.Sprintf("- 已刷新 project.state，但没有找到目标 clip gain：%s。", label))
		} else {
			lines = append(lines, fmt.Sprintf("- 本轮没有开放 project.state；目标 clip 是 %s。", label))
		}
	}
	if len(targets) > 1 {
		lines = append(lines, fmt.Sprintf("- 批量摘要：目标 clip %d 个，其中 %d 个已从 project.state/result 验证。", len(targets), verifiedCount))
	}
	if ranObserve {
		lines = append(lines, "- 已重新运行 mix.observe，刷新 B1.2 电平证据。")
		lines = append(lines, messageLoopGainStagingEvidenceLine("track_acoustic", "峰值/RMS/LUFS/余量", pack))
		lines = append(lines, messageLoopGainStagingEvidenceLine("track_loudness", "LUFS/近似响度", pack))
	} else {
		lines = append(lines, "- 本轮没有开放 mix.observe。")
	}
	if ranAudioStatus {
		lines = append(lines, "- 已读取 project.audio_analysis_status，刷新全工程 DAD 波形指标。")
	} else {
		lines = append(lines, "- 本轮没有刷新 project.audio_analysis_status。")
	}
	lines = append(lines, "- 本次只修改 clip gain；B1.2 没有使用轨道推子、轨道编组或 mix tick 做静态平衡。")
	if len(pack.Limitations) > 0 {
		lines = append(lines, "", "限制")
		for i, item := range pack.Limitations {
			if i >= 4 {
				lines = append(lines, fmt.Sprintf("- 另有 %d 条限制已省略。", len(pack.Limitations)-i))
				break
			}
			lines = append(lines, "- "+strings.TrimSpace(item))
		}
	}
	messageLoopCompactB1PreflightState(state, pack)
	return strings.Join(lines, "\n"), true, Result{}
}

func messageLoopB12ClipGainVerificationIssue(state *runState, resultRows []map[string]any, targets []messageLoopB12SourceCalibrationTarget, ranProjectState bool) string {
	for _, target := range targets {
		verifiedGain, ok := messageLoopB12ClipGainFromProjectState(state, target.ClipID)
		if !ok {
			verifiedGain, ok = messageLoopB12ClipGainFromResultRows(resultRows, target.ClipID)
		}
		label := firstNonEmpty(target.ClipID, target.TrackID, "target clip")
		if !ok {
			if ranProjectState {
				return fmt.Sprintf("target %s was not present in refreshed project.state or the write result", label)
			}
			continue
		}
		if mathAbs(verifiedGain-target.TargetGainDB) > 0.01 {
			return fmt.Sprintf("target %s is %+.3f dB, expected %+.3f dB", label, verifiedGain, target.TargetGainDB)
		}
	}
	return ""
}

type messageLoopB12UnsafePeak struct {
	TrackID   string
	TrackName string
	ClipID    string
	PeakDBFS  float64
}

func messageLoopB12UnsafeTargetPeaks(pack capabilitycontext.Pack, targets []messageLoopB12SourceCalibrationTarget) []messageLoopB12UnsafePeak {
	targetTracks := map[string]bool{}
	targetClips := map[string]bool{}
	for _, target := range targets {
		if trackID := strings.TrimSpace(target.TrackID); trackID != "" {
			targetTracks[trackID] = true
		}
		if clipID := strings.TrimSpace(target.ClipID); clipID != "" {
			targetClips[clipID] = true
		}
	}
	out := []messageLoopB12UnsafePeak{}
	for _, track := range pack.Tracks {
		clipID := ""
		if track.PrimaryClip != nil {
			clipID = strings.TrimSpace(track.PrimaryClip.ClipID)
		}
		if !targetTracks[strings.TrimSpace(track.TrackID)] && !targetClips[clipID] {
			continue
		}
		peak, ok := messageLoopB12EffectiveStaticPeak(track)
		if !ok || peak <= levelsafety.StaticPeakCeilingDBFS+0.001 {
			continue
		}
		out = append(out, messageLoopB12UnsafePeak{
			TrackID:   strings.TrimSpace(track.TrackID),
			TrackName: strings.TrimSpace(track.TrackName),
			ClipID:    clipID,
			PeakDBFS:  peak,
		})
	}
	return out
}

func messageLoopB12EffectiveStaticPeak(track capabilitycontext.TrackGainRow) (float64, bool) {
	if track.EffectiveStaticPeakDBFS != nil {
		return *track.EffectiveStaticPeakDBFS, true
	}
	if track.PeakDBFS == nil {
		return 0, false
	}
	peak := *track.PeakDBFS
	if track.PrimaryClip != nil && track.PrimaryClip.GainDB != nil {
		peak += *track.PrimaryClip.GainDB
	}
	return peak, true
}

func messageLoopB12SourceCalibrationTargets(call planner.ToolCall) []messageLoopB12SourceCalibrationTarget {
	out := []messageLoopB12SourceCalibrationTarget{}
	for _, action := range messageLoopMapRows(call.Args["pending_actions"]) {
		args := messageLoopMapValue(action["args"])
		if len(args) == 0 {
			args = action
		}
		clipID := firstMapText(args, "clip_id")
		if clipID == "" {
			clipID = firstMapText(action, "clip_id")
		}
		gainDB, ok := firstNumericMapValue(args, "gain_db", "clip_gain_db", "db")
		if !ok {
			gainDB, ok = firstNumericMapValue(action, "gain_db", "clip_gain_db", "target_gain_db")
		}
		if clipID == "" || !ok {
			continue
		}
		out = append(out, messageLoopB12SourceCalibrationTarget{
			ClipID:       clipID,
			TrackID:      firstNonEmpty(firstMapText(args, "track_id"), firstMapText(action, "track_id")),
			TargetGainDB: gainDB,
		})
	}
	if len(out) == 0 {
		clipID := firstMapText(call.Args, "clip_id")
		gainDB, ok := firstNumericMapValue(call.Args, "gain_db", "clip_gain_db", "db")
		if clipID != "" && ok {
			out = append(out, messageLoopB12SourceCalibrationTarget{ClipID: clipID, TrackID: firstMapText(call.Args, "track_id"), TargetGainDB: gainDB})
		}
	}
	return out
}

func messageLoopB12ClipGainResultRows(record map[string]any) []map[string]any {
	result := messageLoopMapValue(record["result"])
	if len(result) == 0 {
		return nil
	}
	for _, key := range []string{"actions", "results", "rows"} {
		if rows := messageLoopMapRows(result[key]); len(rows) > 0 {
			return rows
		}
	}
	if firstMapText(result, "clip_id", "id", "item_id") != "" {
		return []map[string]any{result}
	}
	return nil
}

func messageLoopB12ClipGainFromResultRows(rows []map[string]any, clipID string) (float64, bool) {
	clipID = strings.TrimSpace(clipID)
	if clipID == "" {
		return 0, false
	}
	for _, row := range rows {
		if firstMapText(row, "clip_id") != clipID {
			continue
		}
		if value, ok := firstNumericMapValue(row, "clip_gain_db", "gain_db", "target_gain_db", "requested_gain_db"); ok {
			return value, true
		}
	}
	return 0, false
}
func messageLoopB12ClipGainFromProjectState(state *runState, clipID string) (float64, bool) {
	clipID = strings.TrimSpace(clipID)
	if state == nil || clipID == "" {
		return 0, false
	}
	projectState := messageLoopGainStagingProjectState(state)
	for _, clip := range messageLoopMapRows(projectState["clips"]) {
		if firstMapText(clip, "clip_id", "id", "item_id") == clipID {
			return firstNumericMapValue(clip, "clip_gain_db", "gain_db", "db")
		}
	}
	for _, track := range messageLoopGainStagingProjectTrackRows(state) {
		for _, key := range []string{"clips", "clip_summaries", "audio_clips"} {
			for _, clip := range messageLoopMapRows(track[key]) {
				if firstMapText(clip, "clip_id", "id", "item_id") == clipID {
					return firstNumericMapValue(clip, "clip_gain_db", "gain_db", "db")
				}
			}
		}
	}
	return 0, false
}

func messageLoopGainStagingTrackName(pack capabilitycontext.Pack, trackID string) string {
	trackID = strings.TrimSpace(trackID)
	for _, track := range pack.Tracks {
		if track.TrackID == trackID {
			return strings.TrimSpace(track.TrackName)
		}
	}
	return ""
}

func messageLoopCompactB1PreflightState(state *runState, pack capabilitycontext.Pack) {
	if state == nil {
		return
	}
	for i, record := range state.executed {
		toolCallID := firstMapText(record, "tool_call_id")
		if !messageLoopB1PreflightToolCallID(toolCallID) {
			continue
		}
		compact := cloneMap(record)
		compact["result"] = messageLoopB1PreflightResultSummary(
			firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])),
			toolCallID,
			messageLoopText(record["status"]),
			messageLoopMapValue(record["result"]),
			pack,
		)
		state.executed[i] = compact
	}
	for i := range state.trace {
		if state.trace[i].ToolResult == nil || !messageLoopB1PreflightToolCallID(state.trace[i].ToolResult.ToolCallID) {
			continue
		}
		result := *state.trace[i].ToolResult
		result.Result = messageLoopB1PreflightResultSummary(result.Tool, result.ToolCallID, result.Status, result.Result, pack)
		state.trace[i].ToolResult = &result
	}
	if state.recentObservation != nil && messageLoopB1PreflightToolCallID(state.recentObservation.ToolCallID) {
		state.recentObservation.Summary = messageLoopB1PreflightResultSummary(
			firstNonEmpty(state.recentObservation.Tool, state.recentObservation.CommandName),
			state.recentObservation.ToolCallID,
			state.recentObservation.Status,
			state.recentObservation.Summary,
			pack,
		)
	}
}

func messageLoopB1PreflightToolCallID(toolCallID string) bool {
	switch strings.TrimSpace(toolCallID) {
	case "observe_b1_gain_staging", "b1_gain_staging_project_state", "b1_2_source_calibration_verify_project_state", "b1_2_source_calibration_verify_observe", "b1_3_after_fader_reset_project_state", "b1_3_after_fader_reset_observe":
		return true
	default:
		return false
	}
}

func messageLoopB1PreflightResultSummary(tool, toolCallID, status string, result map[string]any, pack capabilitycontext.Pack) map[string]any {
	out := map[string]any{
		"capability_id":        capabilitycontext.GainStagingCapabilityID,
		"context_pack_id":      pack.PackID,
		"context_pack_request": "b1_gain_staging_default",
		"summarized_into":      "capability_context_pack",
	}
	if toolCallID = strings.TrimSpace(toolCallID); toolCallID != "" {
		out["tool_call_id"] = toolCallID
	}
	if status = strings.TrimSpace(status); status != "" {
		out["status"] = status
	} else if resultStatus := firstMapText(result, "status"); resultStatus != "" {
		out["status"] = resultStatus
	}
	if observationID := firstMapText(result, "observation_id"); observationID != "" {
		out["observation_id"] = observationID
	}
	if sessionID := firstMapText(result, "mix_session_id"); sessionID != "" {
		out["mix_session_id"] = sessionID
	}
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "project.state", "get_project_state":
		if count := len(messageLoopMapRows(result["tracks"])); count > 0 {
			out["project_state_track_count"] = count
		}
	}
	if len(pack.Summary) > 0 {
		summary := map[string]any{}
		for _, key := range []string{"track_count", "returned_track_count", "tracks_with_acoustics", "tracks_with_loudness", "tracks_with_track_gain", "clips_with_clip_gain", "headroom_risk_count", "low_level_count", "source_level_outlier_count", "source_level_strict_candidate_count"} {
			if value, ok := pack.Summary[key]; ok {
				summary[key] = value
			}
		}
		if len(summary) > 0 {
			out["pack_summary"] = summary
		}
	}
	return out
}

func messageLoopGainStagingCapabilityRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if messageLoopGainStagingFaderResetRequest(text) {
		return true
	}
	if messageLoopGainStagingExplicitRequest(text) {
		return true
	}
	if messageLoopClipFadeGainRequest(text) {
		return false
	}
	hasGainHealth := messageLoopTextHasAny(text,
		"gain staging", "gain structure", "gain health", "headroom", "level health", "level check", "level scan", "level audit", "input level", "source level", "peak check", "rms check", "loudness check",
		"source calibration", "source level calibration", "source gain", "clip calibration", "reference level", "same reference", "same range", "strict reference", "static source level",
		"\u589e\u76ca\u7ed3\u6784", "\u589e\u76ca\u6574\u7406", "\u589e\u76ca\u9636\u6bb5", "\u7535\u5e73\u7ed3\u6784", "\u7535\u5e73\u5065\u5eb7", "\u7535\u5e73\u68c0\u67e5", "\u7535\u5e73\u626b\u63cf", "\u7535\u5e73\u5ba1\u8ba1", "\u8f93\u5165\u7535\u5e73", "\u6e90\u7535\u5e73", "\u97f3\u91cf\u68c0\u67e5", "\u97f3\u91cf\u626b\u63cf", "\u97f3\u91cf\u5ba1\u8ba1", "\u54cd\u5ea6\u68c0\u67e5", "\u5cf0\u503c\u68c0\u67e5", "\u4f59\u91cf",
		"\u7535\u5e73\u8bc1\u636e", "\u6e90\u7d20\u6750", "\u7d20\u6750\u7535\u5e73", "\u6e90\u7d20\u6750\u6821\u51c6", "\u7d20\u6750\u6821\u51c6",
		"\u53c2\u8003\u7535\u5e73", "\u53c2\u8003\u6307\u6807", "\u540c\u4e00\u53c2\u8003", "\u540c\u4e00\u6307\u6807", "\u7edf\u4e00\u7535\u5e73", "\u76f8\u540c\u8303\u56f4", "\u9759\u6001\u6e90\u7535\u5e73",
	)
	hasProjectLevelCheck := messageLoopTextHasAny(text, "full project", "whole project", "entire project", "all tracks", "\u6574\u4e2a\u5de5\u7a0b", "\u5168\u5de5\u7a0b", "\u6574\u4f53", "\u5168\u5c40", "\u6240\u6709\u8f68\u9053", "\u5168\u90e8\u8f68\u9053") &&
		messageLoopTextHasAny(text, "gain", "level", "volume", "loudness", "peak", "rms", "lufs", "headroom", "\u589e\u76ca", "\u7535\u5e73", "\u97f3\u91cf", "\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u4f59\u91cf") &&
		messageLoopTextHasAny(text, "check", "scan", "audit", "inspect", "analyze", "analyse", "\u68c0\u67e5", "\u626b\u63cf", "\u5ba1\u8ba1", "\u67e5\u770b", "\u5206\u6790")
	return hasGainHealth || hasProjectLevelCheck
}

func messageLoopGainStagingExplicitRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasB1 := messageLoopTextHasAny(text, "b1", "b 1", "static_mix.gain_staging", "gain staging", "gain-staging", "gainstage")
	// In B2 requests, B1 is commonly a readiness reference ("based on B1"),
	// not a request to run the gain-staging capability again.
	if hasB1 && messageLoopStaticBalanceIntentReference(text) {
		return false
	}
	if hasB1 {
		return true
	}
	hasB12 := messageLoopTextHasAny(text, "b1.2", "b 1.2", "b1-2", "b 1-2", "b1_2")
	if hasB12 || messageLoopGainStagingStrictReferenceIntent(text) {
		return true
	}
	return hasB1 && messageLoopTextHasAny(text,
		"gain", "level", "volume", "loudness", "peak", "rms", "lufs", "headroom",
		"\u589e\u76ca", "\u7535\u5e73", "\u97f3\u91cf", "\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u4f59\u91cf",
	)
}

func messageLoopStaticBalanceIntentReference(text string) bool {
	return messageLoopTextHasAny(strings.ToLower(strings.TrimSpace(text)),
		"b2", "b 2", "static_mix.static_balance", "static balance",
		"\u9759\u6001\u5e73\u8861", "\u9759\u6001\u97f3\u91cf", "\u63a8\u5b50\u5e73\u8861",
	)
}

func messageLoopGainStagingStrictReferenceIntent(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasReference := messageLoopTextHasAny(text,
		"strict reference", "reference level", "reference-level", "same reference", "same range", "full-project reference", "source level calibration",
		"\u4e25\u683c\u53c2\u8003", "\u4e25\u683c\u6821\u51c6", "\u53c2\u8003\u7535\u5e73", "\u53c2\u8003\u6307\u6807", "\u540c\u4e00\u53c2\u8003", "\u540c\u4e00\u6307\u6807", "\u7edf\u4e00\u7535\u5e73", "\u76f8\u540c\u8303\u56f4", "\u8d34\u8fd1",
	)
	if !hasReference {
		return false
	}
	return messageLoopTextHasAny(text,
		"calibrate", "calibration", "source level", "clip gain", "clip.gain", "full project", "whole project", "entire project", "all tracks", "pending action", "prepare",
		"\u6821\u51c6", "\u6e90\u7535\u5e73", "\u9759\u6001\u6e90\u7535\u5e73", "\u5168\u5de5\u7a0b", "\u6574\u4e2a\u5de5\u7a0b", "\u6240\u6709\u8f68\u9053", "\u5168\u90e8\u8f68\u9053", "\u5f85\u786e\u8ba4\u52a8\u4f5c", "\u51c6\u5907",
	)
}

func messageLoopGainStagingObservationCall(state *runState) planner.ToolCall {
	args := messageLoopMixObservationArgs(state.input.UserText, messageLoopGainStagingObservationSeedArgs(state))
	if !messageLoopGainStagingExplicitNarrowScope(state.input.UserText, args) {
		args["scope"] = "full_project"
		args["project_context"] = true
	}
	args["capability_id"] = capabilitycontext.GainStagingCapabilityID
	args["context_pack_request"] = "b1_gain_staging_default"
	args["observation_only"] = true
	args["disclosure"] = "digest_catalog"
	if _, ok := args["max_rows"]; !ok {
		args["max_rows"] = 12
	}
	return planner.ToolCall{
		ID:     "observe_b1_gain_staging",
		Tool:   messageLoopPreferredMixObservationTool(state),
		Args:   args,
		Reason: "observe project gain/headroom facts for B1 gain staging default context pack",
	}
}

func messageLoopGainStagingObservationSeedArgs(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	out := map[string]any{}
	for _, source := range []map[string]any{
		state.input.Context,
		messageLoopMapValue(state.input.Context["b1_2_observation_args"]),
		messageLoopMapValue(state.input.Context["observation_args"]),
		messageLoopMapValue(state.contextSnapshot["b1_2_observation_args"]),
		messageLoopMapValue(state.contextSnapshot["observation_args"]),
	} {
		for _, key := range []string{
			"feature_snapshot_path", "mixboard_feature_snapshot_path",
			"acoustic_package_status_path", "acoustic_package_store_path",
			"mix_session_id", "project_id",
		} {
			if _, exists := out[key]; exists {
				continue
			}
			if value := firstMapText(source, key); value != "" {
				out[key] = value
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func messageLoopGainStagingExplicitNarrowScope(userText string, args map[string]any) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if firstMapText(args, "track_id", "selected_track_id", "target_track_id", "clip_id", "selected_clip_id", "track_name", "target_track_name") != "" {
		return true
	}
	return messageLoopTextHasAny(text,
		"selected track", "current track", "this track", "selected clip", "current clip", "this clip",
		"\u9009\u4e2d\u8f68\u9053", "\u5f53\u524d\u8f68\u9053", "\u8fd9\u6761\u8f68", "\u9009\u4e2d\u7247\u6bb5", "\u5f53\u524d\u7247\u6bb5",
	)
}

func messageLoopHasGainStagingContextPack(state *runState) bool {
	if state == nil {
		return false
	}
	for _, event := range state.trace {
		if event.Kind == "capability_context_pack" && strings.Contains(event.Message, capabilitycontext.GainStagingCapabilityID) {
			return true
		}
	}
	return false
}

func messageLoopGainStagingHasProjectTracks(state *runState) bool {
	return len(messageLoopMapRows(messageLoopGainStagingProjectState(state)["tracks"])) > 0 ||
		len(messageLoopMapRows(messageLoopMapValue(messageLoopGainStagingProjectState(state)["daw_state_summary"])["tracks"])) > 0
}

func messageLoopHasProjectStateExecution(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		name := strings.ToLower(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])))
		if name == "project.state" || name == "get_project_state" {
			return true
		}
	}
	return false
}

func messageLoopHasAudioAnalysisStatusExecution(state *runState) bool {
	if state == nil {
		return false
	}
	for _, record := range state.executed {
		name := strings.ToLower(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])))
		if name == "project.audio_analysis_status" {
			return true
		}
	}
	return false
}

func messageLoopGainStagingProjectState(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		name := strings.ToLower(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])))
		if name == "project.state" || name == "get_project_state" {
			if result := messageLoopMapValue(record["result"]); len(result) > 0 {
				return result
			}
		}
	}
	if len(state.input.State) > 0 {
		return state.input.State
	}
	if daw := messageLoopMapValue(state.contextSnapshot["daw_state_summary"]); len(daw) > 0 {
		return daw
	}
	return state.input.Context
}

func messageLoopLastAudioAnalysisStatusResult(state *runState) map[string]any {
	if state == nil {
		return nil
	}
	for i := len(state.executed) - 1; i >= 0; i-- {
		record := state.executed[i]
		if !messageLoopExecutionSucceeded(record) {
			continue
		}
		name := strings.ToLower(firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])))
		if name != "project.audio_analysis_status" {
			continue
		}
		if result := messageLoopMapValue(record["result"]); len(result) > 0 {
			return result
		}
	}
	return nil
}

func messageLoopGainStagingAnalysisJob(status map[string]any) map[string]any {
	if len(status) == 0 {
		return nil
	}
	if job := messageLoopMapValue(status["analysis_job"]); len(job) > 0 {
		return job
	}
	if result := messageLoopMapValue(status["result"]); len(result) > 0 {
		return messageLoopMapValue(result["analysis_job"])
	}
	return nil
}

func messageLoopGainStagingAnalysisNeedsRetry(status map[string]any) bool {
	job := messageLoopGainStagingAnalysisJob(status)
	if strings.ToLower(firstMapText(job, "status", "analysis_queue_status")) != "submitted" {
		return false
	}
	total, totalOK := firstNumericMapValue(job, "dad_fact_total_count", "total_clips")
	ready, readyOK := firstNumericMapValue(job, "dad_fact_ready_count")
	return totalOK && readyOK && total > 0 && ready < total
}

func messageLoopGainStagingAnalysisNeedsRecovery(status map[string]any) bool {
	return len(messageLoopGainStagingAnalysisJob(status)) == 0 || messageLoopGainStagingAnalysisNeedsRetry(status)
}

func messageLoopGainStagingAnalysisReady(status map[string]any) bool {
	job := messageLoopGainStagingAnalysisJob(status)
	if len(job) == 0 {
		return false
	}
	total, totalOK := firstNumericMapValue(job, "dad_fact_total_count", "total_clips")
	ready, readyOK := firstNumericMapValue(job, "dad_fact_ready_count")
	return totalOK && readyOK && total > 0 && ready >= total && strings.EqualFold(firstMapText(job, "dad_fact_status"), "ready")
}

func messageLoopGainStagingAnalysisExplicitlyIncomplete(status map[string]any) (bool, string) {
	job := messageLoopGainStagingAnalysisJob(status)
	if len(job) == 0 {
		return false, ""
	}
	total, totalOK := firstNumericMapValue(job, "dad_fact_total_count", "total_clips")
	ready, readyOK := firstNumericMapValue(job, "dad_fact_ready_count")
	pending, pendingOK := firstNumericMapValue(job, "dad_fact_pending_count", "pending_clips")
	if totalOK && readyOK && total > 0 && ready < total {
		return true, fmt.Sprintf("ready=%.0f/%.0f", ready, total)
	}
	if pendingOK && pending > 0 {
		return true, fmt.Sprintf("pending=%.0f", pending)
	}
	statusText := strings.ToLower(strings.TrimSpace(firstMapText(job, "dad_fact_status")))
	for _, marker := range []string{"partial", "stale", "suspect", "missing", "invalid", "failed", "error"} {
		if strings.Contains(statusText, marker) {
			return true, "dad_fact_status=" + statusText
		}
	}
	return false, ""
}

func (l *MessageLoop) messageLoopGainStagingWaitForAnalysisReady(ctx context.Context, r *Runner, state *runState) (bool, Result) {
	interval := messageLoopDADGatePollInterval()
	idleTimeout := messageLoopDADGateIdleTimeout()
	maxWait := messageLoopDADGateMaxWait()
	waitStartedAt := time.Now()
	lastProgressAt := waitStartedAt
	lastSignature := ""

	for attempt := 1; ; attempt++ {
		call := messageLoopGainStagingAudioAnalysisStatusCall(
			fmt.Sprintf("b1_gain_staging_audio_analysis_wait_%d", attempt),
			"wait for full-project DAD source facts before continuing B1.2",
		)
		toolCallsUsedBeforeStatusRead := state.toolCallsUsed
		toolStarted := time.Now()
		stopped, result := r.executeTool(ctx, state, call, false, nil)
		state.toolCallsUsed = toolCallsUsedBeforeStatusRead
		l.logTiming("message_loop.tool", toolStarted, "goal=%s tool=%s confirmed=false b1_dad_gate=true attempt=%d stopped=%t status=%s", state.goal.GoalID, call.Tool, attempt, stopped, result.Status)
		if stopped {
			return true, result
		}

		status := messageLoopLastAudioAnalysisStatusResult(state)
		if messageLoopGainStagingAnalysisReady(status) {
			state.trace = append(state.trace, planner.TraceEvent{Kind: "readiness", Message: "B1 DAD readiness reached full-project coverage; continuing B1.2 in the same goal"})
			return false, Result{}
		}

		job := messageLoopGainStagingAnalysisJob(status)
		ready, _ := firstNumericMapValue(job, "dad_fact_ready_count")
		total, _ := firstNumericMapValue(job, "dad_fact_total_count", "total_clips")
		pending, _ := firstNumericMapValue(job, "dad_fact_pending_count", "pending_clips")
		signature := fmt.Sprintf("%.0f/%.0f|pending=%.0f|status=%s", ready, total, pending, firstMapText(job, "dad_fact_status", "analysis_queue_status", "status"))
		now := time.Now()
		if signature != lastSignature {
			lastSignature = signature
			lastProgressAt = now
		}
		if idleTimeout > 0 && now.Sub(lastProgressAt) >= idleTimeout {
			return true, r.fail(state, fmt.Errorf("B1 waiting for DAD made no progress: ready=%.0f/%.0f pending=%.0f idle_timeout=%s", ready, total, pending, idleTimeout))
		}
		if maxWait > 0 && now.Sub(waitStartedAt) >= maxWait {
			return true, r.fail(state, fmt.Errorf("B1 waiting for DAD exceeded max wait: ready=%.0f/%.0f pending=%.0f max_wait=%s", ready, total, pending, maxWait))
		}
		if interval > 0 {
			if err := messageLoopSleepContext(ctx, interval); err != nil {
				return true, r.fail(state, err)
			}
		}
	}
}

func messageLoopGainStagingAudioAnalysisRetryCall(status map[string]any) planner.ToolCall {
	job := messageLoopGainStagingAnalysisJob(status)
	args := map[string]any{
		"cmd":                  "project.audio_analysis_start",
		"retry_missing":        true,
		"rebuild_from_project": true,
		"interval_ms":          50,
	}
	if jobID := firstMapText(job, "analysis_job_id", "job_id"); jobID != "" {
		args["analysis_job_id"] = jobID
	}
	return planner.ToolCall{
		ID:      "b1_gain_staging_audio_analysis_retry",
		Tool:    "project.audio_analysis_start",
		Args:    cloneMap(args),
		Command: cloneMap(args),
		Reason:  "rebuild source DAD rows invalidated by clip graph edits before B1.2",
	}
}

func messageLoopGainStagingAnalysisRetryReply(status map[string]any) string {
	job := messageLoopGainStagingAnalysisJob(status)
	if len(job) == 0 {
		return "B1.2 没有找到可恢复的全工程 DAD 分析任务，已根据当前工程的音频轨和源文件自动重建并启动源素材分析。本轮没有修改推子或 clip gain；分析完成后重新运行 B1.2，即可生成覆盖每轨全部存活片段的待确认校准方案。"
	}
	ready, _ := firstNumericMapValue(job, "dad_fact_ready_count")
	total, _ := firstNumericMapValue(job, "dad_fact_total_count", "total_clips")
	missing := math.Max(0, total-ready)
	return fmt.Sprintf("B1.2 检测到 A4/片段编辑后源素材分析证据失效：当前 %.0f/%.0f 条轨道可用，%.0f 条缺失。已自动重新排队全工程 DAD 源素材分析；本轮没有修改推子或 clip gain。分析完成后重新运行 B1.2，即可生成覆盖每轨全部存活片段的待确认校准方案。", ready, total, missing)
}

func messageLoopGainStagingAudioAnalysisStatusCall(id string, reason string) planner.ToolCall {
	command := map[string]any{
		"cmd":    "project.audio_analysis_status",
		"latest": true,
	}
	return planner.ToolCall{
		ID:     firstNonEmpty(strings.TrimSpace(id), "b1_gain_staging_audio_analysis_status"),
		Tool:   "project.audio_analysis_status",
		Args:   command,
		Reason: firstNonEmpty(strings.TrimSpace(reason), "read automatic DAD waveform metrics for B1 gain staging"),
	}
}
