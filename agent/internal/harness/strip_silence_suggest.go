package harness

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"vit-daw-agent/internal/tools"
)

const (
	stripSilenceSuggestDefaultMinSilenceMS = 120.0
	stripSilenceSuggestDefaultStartPadMS   = 15.0
	stripSilenceSuggestDefaultEndPadMS     = 50.0
)

type stripSilenceSuggestTarget struct {
	ClipID    string
	TrackID   string
	ClipName  string
	TrackName string
	Ranges    []map[string]any
}

type stripSilenceSuggestCandidate struct {
	ThresholdDBFS         float64
	TargetCount           int
	AnalyzedCount         int
	ErrorCount            int
	StripRegionCount      int
	KeepSegmentCount      int
	TotalStripSeconds     float64
	TotalAnalyzedSeconds  float64
	WouldRemoveEntireClip bool
	Analyses              []map[string]any
	PendingActions        []map[string]any
	Warnings              []string
	Score                 float64
}

func (h *Harness) suggestStripSilence(ctx context.Context, cmd map[string]any, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("clip.strip_silence.suggest requires a connected kernel")
	}

	contextForResolution := stripSilenceSuggestMergedContext(requestContext)
	targets, scope, err := h.stripSilenceSuggestTargets(ctx, cmd, contextForResolution)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("clip.strip_silence.suggest found no audio clips to analyze")
	}

	params := stripSilenceSuggestParams(cmd)
	thresholds := stripSilenceSuggestThresholds(cmd)
	candidates := make([]stripSilenceSuggestCandidate, 0, len(thresholds))
	for _, threshold := range thresholds {
		candidate := h.runStripSilenceSuggestCandidate(ctx, cmd, targets, params, threshold, scope)
		candidate.Score = stripSilenceSuggestScore(candidate)
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("clip.strip_silence.suggest has no threshold candidates")
	}

	best := stripSilenceBestCandidate(candidates)
	confidence, confidenceScore := stripSilenceSuggestConfidence(best)
	risks := stripSilenceSuggestRisks(best)
	recommended := map[string]any{
		"threshold_dbfs":      roundFloat(best.ThresholdDBFS, 3),
		"min_silence_ms":      roundFloat(params["min_silence_ms"], 3),
		"clip_start_pad_ms":   roundFloat(params["clip_start_pad_ms"], 3),
		"clip_end_pad_ms":     roundFloat(params["clip_end_pad_ms"], 3),
		"analysis_frame_ms":   roundFloat(params["frame_ms"], 3),
		"selection_strategy":  "multi_threshold_kernel_analyze",
		"noise_floor_method":  "inferred_from_threshold_sweep",
		"estimated_gate_dbfs": roundFloat(best.ThresholdDBFS, 3),
	}

	out := map[string]any{
		"status":                       "ok",
		"schema_version":               "clip.strip_silence.suggest.v0",
		"message":                      "Strip Silence suggestion ready",
		"scope":                        scope,
		"target_count":                 len(targets),
		"analyzed_clip_count":          best.AnalyzedCount,
		"analysis_failed_clip_count":   best.ErrorCount,
		"no_cleanup_needed_clip_count": stripSilenceSuggestNoCleanupCount(best),
		"recommended_params":           recommended,
		"confidence":                   confidence,
		"confidence_score":             confidenceScore,
		"risk_level":                   "confirm",
		"risks":                        risks,
		"reasoning":                    stripSilenceSuggestReasoning(best),
		"candidate_results":            stripSilenceSuggestCandidateRows(candidates),
		"analysis":                     stripSilenceSuggestAnalysisSummary(best),
		"pending_actions":              best.PendingActions,
		"pending_action_count":         len(best.PendingActions),
		"pending_confirmation":         len(best.PendingActions) > 0,
		"pending_action_summary":       stripSilenceSuggestPendingSummary(best),
	}
	if len(best.PendingActions) == 1 {
		out["pending_action"] = best.PendingActions[0]
	}
	if len(best.Warnings) > 0 {
		out["warnings"] = append([]string(nil), best.Warnings...)
	}
	return out, nil
}

func stripSilenceSuggestMergedContext(requestContext map[string]any) map[string]any {
	out := cloneAnyMap(requestContext)
	if out == nil {
		out = map[string]any{}
	}
	for _, key := range []string{"current_selection", "ui_context"} {
		nested := mapAnyFromAny(out[key])
		for k, v := range nested {
			if isEmptyValue(out[k]) && !isEmptyValue(v) {
				out[k] = v
			}
		}
	}
	return out
}

func (h *Harness) stripSilenceSuggestTargets(ctx context.Context, cmd map[string]any, requestContext map[string]any) ([]stripSilenceSuggestTarget, string, error) {
	scope := stripSilenceSuggestScope(cmd, requestContext)
	if scope == "all_project" {
		return h.stripSilenceSuggestAllProjectTargets(ctx), scope, nil
	}
	if scope == "selected_track" {
		return h.stripSilenceSuggestSelectedTrackTargets(ctx, cmd, requestContext), scope, nil
	}

	if rangeRows := stripSilenceSuggestRangeRows(cmd, requestContext); scope == "selected_ranges" && len(rangeRows) > 0 {
		targets := stripSilenceSuggestTargetsFromRanges(rangeRows, requestContext)
		if len(targets) > 0 {
			return targets, "selected_ranges", nil
		}
	}

	if clipID := firstString(cmd, "clip_id", "selected_clip_id", "primary_selected_clip_id", "source_clip_id", "target_clip_id"); clipID != "" {
		return []stripSilenceSuggestTarget{{
			ClipID:  clipID,
			TrackID: firstNonEmpty(firstString(cmd, "track_id", "selected_clip_track_id"), firstString(requestContext, "selected_clip_track_id", "selected_track_id", "track_id")),
		}}, "selected_clip", nil
	}

	ref, err := h.resolveSingleClipRef(ctx, tools.CommandSpec{ToolName: "clip.strip_silence.suggest", CommandName: "clip.strip_silence.suggest"}, cmd, requestContext, "clip_id")
	if err != nil {
		return nil, scope, err
	}
	return []stripSilenceSuggestTarget{{
		ClipID:    ref.ID,
		TrackID:   ref.TrackID,
		ClipName:  ref.Name,
		TrackName: ref.TrackName,
	}}, "selected_clip", nil
}

func (h *Harness) stripSilenceSuggestAllProjectTargets(ctx context.Context) []stripSilenceSuggestTarget {
	state := h.UserStateSummary(ctx)
	out := []stripSilenceSuggestTarget{}
	for _, track := range visibleTrackRows(state) {
		if stripSilenceSuggestTrackLooksNonAudio(track) {
			continue
		}
		trackID := visibleTrackID(track)
		trackName := visibleTrackName(track)
		for _, clip := range mapRowsFromAny(track["clips"]) {
			clipID := firstString(clip, "clip_id", "id", "item_id")
			if clipID == "" || stripSilenceSuggestClipLooksNonAudio(clip) {
				continue
			}
			out = append(out, stripSilenceSuggestTarget{
				ClipID:    clipID,
				TrackID:   trackID,
				ClipName:  firstString(clip, "name", "clip_name"),
				TrackName: trackName,
			})
		}
	}
	return out
}

func (h *Harness) stripSilenceSuggestSelectedTrackTargets(ctx context.Context, cmd map[string]any, requestContext map[string]any) []stripSilenceSuggestTarget {
	state := h.UserStateSummary(ctx)
	tracks := visibleTrackRows(state)
	trackRefs := stripSilenceSuggestTrackRefs(cmd, requestContext)
	if len(trackRefs) == 0 {
		return nil
	}
	trackIDs := map[string]bool{}
	for _, ref := range trackRefs {
		if resolved, ok := resolveVisibleTrackAlias(ref, tracks); ok {
			trackIDs[resolved] = true
			continue
		}
		trackIDs[ref] = true
	}
	out := []stripSilenceSuggestTarget{}
	for _, track := range tracks {
		if stripSilenceSuggestTrackLooksNonAudio(track) {
			continue
		}
		trackID := visibleTrackID(track)
		if trackID == "" || !trackIDs[trackID] {
			continue
		}
		trackName := visibleTrackName(track)
		for _, clip := range mapRowsFromAny(track["clips"]) {
			clipID := firstString(clip, "clip_id", "id", "item_id")
			if clipID == "" || stripSilenceSuggestClipLooksNonAudio(clip) {
				continue
			}
			out = append(out, stripSilenceSuggestTarget{
				ClipID:    clipID,
				TrackID:   trackID,
				ClipName:  firstString(clip, "name", "clip_name"),
				TrackName: trackName,
			})
		}
	}
	return out
}

func stripSilenceSuggestTrackRefs(cmd map[string]any, requestContext map[string]any) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, row := range []map[string]any{
		cmd,
		requestContext,
		mapAnyFromAny(requestContext["current_selection"]),
		mapAnyFromAny(requestContext["ui_context"]),
	} {
		add(firstString(row, "track_id", "target_track_id", "selected_track_id", "selected_clip_track_id", "focused_track_id"))
		for _, value := range []any{row["track_ids"], row["target_track_ids"], row["selected_track_ids"]} {
			for _, id := range stringSliceFromAny(value) {
				add(id)
			}
		}
	}
	return out
}

func stripSilenceSuggestTrackLooksNonAudio(track map[string]any) bool {
	if value, ok := boolValue(track["is_audio_track"]); ok && !value {
		return true
	}
	kind := strings.ToLower(firstString(track, "track_type", "kind", "type"))
	return strings.Contains(kind, "midi") || strings.Contains(kind, "folder") || strings.Contains(kind, "marker")
}

func stripSilenceSuggestClipLooksNonAudio(clip map[string]any) bool {
	kind := strings.ToLower(firstString(clip, "clip_type", "type", "kind", "media_type"))
	return strings.Contains(kind, "midi")
}

func stripSilenceSuggestTargetsFromRanges(ranges []map[string]any, requestContext map[string]any) []stripSilenceSuggestTarget {
	byClip := map[string]*stripSilenceSuggestTarget{}
	order := []string{}
	fallbackClipID := firstString(requestContext, "selected_clip_id", "primary_selected_clip_id", "clip_id")
	fallbackTrackID := firstString(requestContext, "selected_clip_track_id", "selected_track_id", "track_id")
	for _, raw := range ranges {
		row := cloneAnyMap(raw)
		clipID := firstNonEmpty(firstString(row, "clip_id", "selected_clip_id"), fallbackClipID)
		if clipID == "" {
			continue
		}
		trackID := firstNonEmpty(firstString(row, "track_id", "selected_clip_track_id"), fallbackTrackID)
		row["clip_id"] = clipID
		if trackID != "" {
			row["track_id"] = trackID
		}
		if _, ok := byClip[clipID]; !ok {
			order = append(order, clipID)
			byClip[clipID] = &stripSilenceSuggestTarget{ClipID: clipID, TrackID: trackID}
		}
		if byClip[clipID].TrackID == "" {
			byClip[clipID].TrackID = trackID
		}
		byClip[clipID].Ranges = append(byClip[clipID].Ranges, stripSilenceSuggestCompactRange(row))
	}
	out := make([]stripSilenceSuggestTarget, 0, len(order))
	for _, clipID := range order {
		out = append(out, *byClip[clipID])
	}
	return out
}

func stripSilenceSuggestCompactRange(row map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"range_id", "clip_id", "track_id",
		"start_seconds", "end_seconds", "duration_seconds",
		"clip_local_start_seconds", "clip_local_end_seconds",
		"local_start_seconds", "local_end_seconds",
	} {
		if !isEmptyValue(row[key]) {
			out[key] = row[key]
		}
	}
	return out
}

func stripSilenceSuggestScope(cmd map[string]any, requestContext map[string]any) string {
	if boolValueDefault(cmd["all_project"], false) || boolValueDefault(cmd["full_project"], false) {
		return "all_project"
	}
	if boolValueDefault(cmd["selected_track"], false) || boolValueDefault(cmd["current_track"], false) {
		return "selected_track"
	}
	scope := strings.ToLower(firstString(cmd, "scope", "target_scope", "analysis_scope"))
	userText := strings.ToLower(firstString(requestContext, "user_message", "user_text", "message", "prompt", "utterance"))
	if strings.Contains(scope, "project") || strings.Contains(scope, "all") || strings.Contains(scope, "global") {
		return "all_project"
	}
	if strings.Contains(scope, "track") || strings.Contains(scope, "current") {
		return "selected_track"
	}
	if strings.Contains(scope, "range") || strings.Contains(scope, "selection") {
		return "selected_ranges"
	}
	if strings.TrimSpace(scope) != "" {
		return "selected_clip"
	}
	if strings.Contains(userText, "all project") || strings.Contains(userText, "whole project") || strings.Contains(userText, "full project") ||
		strings.Contains(userText, "entire project") || strings.Contains(userText, "全工程") || strings.Contains(userText, "整个工程") {
		return "all_project"
	}
	if stripSilenceSuggestTextTargetsSelectedTrack(userText) {
		return "selected_track"
	}
	if len(stripSilenceSuggestRangeRows(cmd, requestContext)) > 0 {
		return "selected_ranges"
	}
	return "selected_clip"
}

func stripSilenceSuggestTextTargetsSelectedTrack(text string) bool {
	return strings.Contains(text, "selected track") ||
		strings.Contains(text, "current track") ||
		strings.Contains(text, "this track") ||
		strings.Contains(text, "\u9009\u4e2d\u8f68\u9053") ||
		strings.Contains(text, "\u5f53\u524d\u8f68\u9053") ||
		strings.Contains(text, "\u8fd9\u6761\u8f68") ||
		strings.Contains(text, "\u8fd9\u4e2a\u8f68\u9053") ||
		strings.Contains(text, "\u9009\u4e2d\u97f3\u8f68") ||
		strings.Contains(text, "\u5f53\u524d\u97f3\u8f68")
}

func stripSilenceSuggestRangeRows(cmd map[string]any, requestContext map[string]any) []map[string]any {
	for _, value := range []any{
		cmd["ranges"],
		cmd["selected_clip_ranges"],
		cmd["range"],
		requestContext["selected_clip_ranges"],
		requestContext["selected_clip_range"],
		mapAnyFromAny(requestContext["current_selection"])["selected_clip_ranges"],
		mapAnyFromAny(requestContext["current_selection"])["selected_clip_range"],
		mapAnyFromAny(requestContext["ui_context"])["selected_clip_ranges"],
		mapAnyFromAny(requestContext["ui_context"])["selected_clip_range"],
	} {
		if rows := mapRowsFromAny(value); len(rows) > 0 {
			return rows
		}
		if row := mapAnyFromAny(value); len(row) > 0 {
			return []map[string]any{row}
		}
	}
	return nil
}

func stripSilenceSuggestParams(cmd map[string]any) map[string]float64 {
	return map[string]float64{
		"min_silence_ms":    stripSilenceNumberOrDefault(cmd, stripSilenceSuggestDefaultMinSilenceMS, "min_silence_ms", "minimum_silence_ms", "min_strip_duration_ms"),
		"clip_start_pad_ms": stripSilenceNumberOrDefault(cmd, stripSilenceSuggestDefaultStartPadMS, "clip_start_pad_ms", "start_pad_ms"),
		"clip_end_pad_ms":   stripSilenceNumberOrDefault(cmd, stripSilenceSuggestDefaultEndPadMS, "clip_end_pad_ms", "end_pad_ms"),
		"frame_ms":          stripSilenceNumberOrDefault(cmd, 10.0, "frame_ms", "analysis_frame_ms"),
	}
}

func stripSilenceNumberOrDefault(row map[string]any, fallback float64, keys ...string) float64 {
	if value, ok := numberValueFromMap(row, keys...); ok && !math.IsNaN(value) && !math.IsInf(value, 0) {
		return value
	}
	return fallback
}

func stripSilenceSuggestThresholds(cmd map[string]any) []float64 {
	if values := floatSliceFromAny(cmd["candidate_thresholds_dbfs"]); len(values) > 0 {
		return stripSilenceNormalizeThresholds(values)
	}
	if threshold, ok := numberValueFromMap(cmd, "threshold_dbfs", "threshold_db", "suggested_threshold_dbfs"); ok {
		return stripSilenceNormalizeThresholds([]float64{threshold - 12, threshold - 6, threshold, threshold + 6, threshold + 12})
	}
	return []float64{-60, -54, -48, -42, -36}
}

func stripSilenceNormalizeThresholds(values []float64) []float64 {
	seen := map[int]bool{}
	out := []float64{}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		if value < -120 {
			value = -120
		}
		if value > 0 {
			value = 0
		}
		key := int(math.Round(value * 1000))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	sort.Float64s(out)
	return out
}

func floatSliceFromAny(value any) []float64 {
	items := anySliceFromAny(value)
	out := make([]float64, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case float64:
			out = append(out, typed)
		case int:
			out = append(out, float64(typed))
		case int64:
			out = append(out, float64(typed))
		case jsonNumber:
			if value, err := typed.Float64(); err == nil {
				out = append(out, value)
			}
		default:
			if value, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(item)), 64); err == nil {
				out = append(out, value)
			}
		}
	}
	return out
}

type jsonNumber interface {
	Float64() (float64, error)
}

func (h *Harness) runStripSilenceSuggestCandidate(ctx context.Context, cmd map[string]any, targets []stripSilenceSuggestTarget, params map[string]float64, threshold float64, scope string) stripSilenceSuggestCandidate {
	candidate := stripSilenceSuggestCandidate{
		ThresholdDBFS: threshold,
		TargetCount:   len(targets),
	}
	analyzeSpec := stripSilenceAnalyzeSpec(h)
	for _, target := range targets {
		payload := map[string]any{
			"cmd":               "clip.strip_silence.analyze",
			"clip_id":           target.ClipID,
			"threshold_dbfs":    threshold,
			"min_silence_ms":    params["min_silence_ms"],
			"clip_start_pad_ms": params["clip_start_pad_ms"],
			"clip_end_pad_ms":   params["clip_end_pad_ms"],
			"analysis_frame_ms": params["frame_ms"],
			"scope":             scope,
			"suggest_source":    "clip.strip_silence.suggest",
			"suggest_candidate": true,
			"suggest_threshold": threshold,
		}
		if target.TrackID != "" {
			payload["track_id"] = target.TrackID
		}
		if len(target.Ranges) > 0 {
			payload["ranges"] = target.Ranges
			payload["selected_clip_ranges"] = target.Ranges
		}
		reply, err := h.runStripSilenceAnalyze(ctx, analyzeSpec, payload)
		if err != nil {
			candidate.ErrorCount++
			candidate.Warnings = append(candidate.Warnings, fmt.Sprintf("clip %s analyze failed: %v", target.ClipID, err))
			continue
		}
		if !kernelReplySucceeded(reply) {
			candidate.ErrorCount++
			candidate.Warnings = append(candidate.Warnings, fmt.Sprintf("clip %s analyze returned %s", target.ClipID, firstNonEmpty(firstString(reply, "message"), firstString(reply, "error"), firstString(reply, "status"))))
			continue
		}
		candidate.AnalyzedCount++
		candidate.Analyses = append(candidate.Analyses, reply)
		candidate.StripRegionCount += int(numberFromAny(reply["strip_region_count"]))
		candidate.KeepSegmentCount += int(numberFromAny(reply["keep_segment_count"]))
		candidate.TotalStripSeconds += stripSilenceRegionsDuration(reply["strip_regions"])
		candidate.TotalAnalyzedSeconds += stripSilenceAnalysisDuration(reply)
		if boolValueDefault(reply["would_remove_entire_clip"], false) {
			candidate.WouldRemoveEntireClip = true
		}
		if action := stripSilencePendingActionFromAnalysis(reply); len(action) > 0 {
			candidate.PendingActions = append(candidate.PendingActions, action)
		}
	}
	_ = cmd
	return candidate
}

func stripSilenceAnalyzeSpec(h *Harness) tools.CommandSpec {
	if h != nil && h.catalog != nil {
		if spec, ok := h.catalog.LookupTool("clip.strip_silence.analyze"); ok {
			return spec
		}
	}
	return tools.CommandSpec{
		CommandName:          "clip.strip_silence.analyze",
		ToolName:             "clip.strip_silence.analyze",
		RiskLevel:            tools.RiskDirect,
		MutatesProject:       false,
		RequiresConfirmation: false,
	}
}

func (h *Harness) runStripSilenceAnalyze(ctx context.Context, spec tools.CommandSpec, payload map[string]any) (map[string]any, error) {
	execution := h.executeKernelCommand(ctx, spec, payload)
	if execution.err != nil {
		return nil, execution.err
	}
	if execution.reply == nil {
		return nil, fmt.Errorf("empty kernel reply")
	}
	return execution.reply, nil
}

func stripSilenceRegionsDuration(value any) float64 {
	total := 0.0
	for _, row := range mapRowsFromAny(value) {
		start, startOK := numberValueFromMap(row, "start_seconds", "range_start_seconds")
		end, endOK := numberValueFromMap(row, "end_seconds", "range_end_seconds")
		if !startOK || !endOK {
			if duration, ok := numberValueFromMap(row, "duration_seconds"); ok {
				total += math.Max(0, duration)
			}
			continue
		}
		total += math.Max(0, end-start)
	}
	return total
}

func stripSilenceAnalysisDuration(analysis map[string]any) float64 {
	if ranges := mapRowsFromAny(analysis["analysis_ranges"]); len(ranges) > 0 {
		total := 0.0
		for _, row := range ranges {
			start, startOK := numberValueFromMap(row, "start_seconds")
			end, endOK := numberValueFromMap(row, "end_seconds")
			if startOK && endOK {
				total += math.Max(0, end-start)
			}
		}
		if total > 0 {
			return total
		}
	}
	if length, ok := numberValueFromMap(analysis, "clip_length_seconds", "duration_seconds"); ok {
		return math.Max(0, length)
	}
	start, startOK := numberValueFromMap(analysis, "clip_start_seconds", "start_seconds")
	end, endOK := numberValueFromMap(analysis, "clip_end_seconds", "end_seconds")
	if startOK && endOK {
		return math.Max(0, end-start)
	}
	return 0
}

func stripSilencePendingActionFromAnalysis(analysis map[string]any) map[string]any {
	regions := mapRowsFromAny(analysis["strip_regions"])
	if len(regions) == 0 {
		return nil
	}
	args := map[string]any{
		"clip_id":                  firstString(analysis, "clip_id"),
		"track_id":                 firstString(analysis, "track_id"),
		"analysis_id":              firstString(analysis, "analysis_id"),
		"strip_regions":            regions,
		"allow_remove_entire_clip": false,
		"source_analysis_tool":     "clip.strip_silence.analyze",
		"source_suggestion_tool":   "clip.strip_silence.suggest",
	}
	return map[string]any{
		"tool_name":             "clip.strip_silence.apply",
		"tool":                  "clip.strip_silence.apply",
		"requires_confirmation": true,
		"risk":                  "confirm",
		"target_ids": map[string]any{
			"clip_id":  args["clip_id"],
			"track_id": args["track_id"],
		},
		"args":          args,
		"rollback_hint": "Use project undo after applying Strip Silence.",
	}
}

func stripSilenceSuggestScore(candidate stripSilenceSuggestCandidate) float64 {
	score := 0.0
	if candidate.AnalyzedCount > 0 {
		score += 20
	}
	if candidate.StripRegionCount > 0 {
		score += 45
	} else {
		score -= 30
	}
	if candidate.KeepSegmentCount > 0 {
		score += 25
	} else {
		score -= 35
	}
	ratio := stripSilenceSuggestStripRatio(candidate)
	if ratio > 0 {
		score += 25 * (1 - math.Min(1, math.Abs(ratio-0.25)/0.25))
	}
	if ratio > 0.7 {
		score -= 45
	}
	if ratio > 0.9 {
		score -= 60
	}
	if candidate.WouldRemoveEntireClip {
		score -= 120
	}
	score -= float64(candidate.ErrorCount) * 25
	score -= math.Max(0, candidate.ThresholdDBFS+60) * 0.05
	return score
}

func stripSilenceBestCandidate(candidates []stripSilenceSuggestCandidate) stripSilenceSuggestCandidate {
	filtered := make([]stripSilenceSuggestCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ratio := stripSilenceSuggestStripRatio(candidate)
		if candidate.ErrorCount == 0 &&
			candidate.AnalyzedCount == candidate.TargetCount &&
			candidate.StripRegionCount > 0 &&
			candidate.KeepSegmentCount > 0 &&
			!candidate.WouldRemoveEntireClip &&
			ratio <= 0.75 {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) > 0 {
		sort.SliceStable(filtered, func(i, j int) bool {
			if filtered[i].ThresholdDBFS == filtered[j].ThresholdDBFS {
				return filtered[i].Score > filtered[j].Score
			}
			return filtered[i].ThresholdDBFS < filtered[j].ThresholdDBFS
		})
		return filtered[0]
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Score > best.Score || (candidate.Score == best.Score && candidate.ThresholdDBFS < best.ThresholdDBFS) {
			best = candidate
		}
	}
	return best
}

func stripSilenceSuggestStripRatio(candidate stripSilenceSuggestCandidate) float64 {
	if candidate.TotalAnalyzedSeconds <= 0 {
		return 0
	}
	return candidate.TotalStripSeconds / candidate.TotalAnalyzedSeconds
}

func stripSilenceSuggestConfidence(candidate stripSilenceSuggestCandidate) (string, float64) {
	ratio := stripSilenceSuggestStripRatio(candidate)
	score := 0.25
	if candidate.TargetCount > 0 {
		score += 0.25 * (float64(candidate.AnalyzedCount) / float64(candidate.TargetCount))
	}
	if candidate.StripRegionCount > 0 {
		score += 0.2
	}
	if candidate.KeepSegmentCount > 0 {
		score += 0.15
	}
	if ratio >= 0.03 && ratio <= 0.65 {
		score += 0.15
	}
	if candidate.ErrorCount > 0 {
		score -= 0.2
	}
	if candidate.WouldRemoveEntireClip || ratio > 0.85 {
		score -= 0.35
	}
	score = math.Max(0, math.Min(1, score))
	switch {
	case score >= 0.8:
		return "high", roundFloat(score, 3)
	case score >= 0.55:
		return "medium", roundFloat(score, 3)
	default:
		return "low", roundFloat(score, 3)
	}
}

func stripSilenceSuggestRisks(candidate stripSilenceSuggestCandidate) []string {
	risks := []string{}
	ratio := stripSilenceSuggestStripRatio(candidate)
	if candidate.StripRegionCount == 0 {
		risks = append(risks, "No removable silence was found at the recommended threshold.")
	}
	if candidate.WouldRemoveEntireClip {
		risks = append(risks, "Recommended threshold would remove an entire clip; apply keeps allow_remove_entire_clip=false.")
	}
	if ratio > 0.65 {
		risks = append(risks, fmt.Sprintf("Recommended cleanup removes %.1f%% of analyzed time; review the preview before applying.", ratio*100))
	}
	if candidate.ErrorCount > 0 {
		risks = append(risks, fmt.Sprintf("%d target(s) failed analysis.", candidate.ErrorCount))
	}
	if len(risks) == 0 {
		risks = append(risks, "Preview before applying; transient breaths, room tails, or soft note attacks may need a lower threshold or longer pads.")
	}
	return risks
}

func stripSilenceSuggestReasoning(candidate stripSilenceSuggestCandidate) []string {
	ratio := stripSilenceSuggestStripRatio(candidate)
	return []string{
		fmt.Sprintf("Chose %.1f dBFS from a threshold sweep because it produced %d strip region(s) while preserving %d keep segment(s).", candidate.ThresholdDBFS, candidate.StripRegionCount, candidate.KeepSegmentCount),
		fmt.Sprintf("Estimated removal is %.3fs of %.3fs analyzed time (%.1f%%).", candidate.TotalStripSeconds, candidate.TotalAnalyzedSeconds, ratio*100),
		"Apply action is built only from strip_regions returned by clip.strip_silence.analyze.",
	}
}

func stripSilenceSuggestCandidateRows(candidates []stripSilenceSuggestCandidate) []map[string]any {
	out := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, map[string]any{
			"threshold_dbfs":           roundFloat(candidate.ThresholdDBFS, 3),
			"target_count":             candidate.TargetCount,
			"analyzed_clip_count":      candidate.AnalyzedCount,
			"error_count":              candidate.ErrorCount,
			"strip_region_count":       candidate.StripRegionCount,
			"keep_segment_count":       candidate.KeepSegmentCount,
			"strip_seconds":            roundFloat(candidate.TotalStripSeconds, 3),
			"analyzed_seconds":         roundFloat(candidate.TotalAnalyzedSeconds, 3),
			"strip_ratio":              roundFloat(stripSilenceSuggestStripRatio(candidate), 4),
			"would_remove_entire_clip": candidate.WouldRemoveEntireClip,
			"score":                    roundFloat(candidate.Score, 3),
			"warning_count":            len(candidate.Warnings),
		})
	}
	return out
}

func stripSilenceSuggestAnalysisSummary(candidate stripSilenceSuggestCandidate) map[string]any {
	return map[string]any{
		"threshold_dbfs":               roundFloat(candidate.ThresholdDBFS, 3),
		"analysis_count":               len(candidate.Analyses),
		"analysis_failed_clip_count":   candidate.ErrorCount,
		"no_cleanup_needed_clip_count": stripSilenceSuggestNoCleanupCount(candidate),
		"strip_region_count":           candidate.StripRegionCount,
		"keep_segment_count":           candidate.KeepSegmentCount,
		"strip_seconds":                roundFloat(candidate.TotalStripSeconds, 3),
		"analyzed_seconds":             roundFloat(candidate.TotalAnalyzedSeconds, 3),
		"strip_ratio":                  roundFloat(stripSilenceSuggestStripRatio(candidate), 4),
		"would_remove_entire_clip":     candidate.WouldRemoveEntireClip,
		"analyses":                     candidate.Analyses,
	}
}

func stripSilenceSuggestPendingSummary(candidate stripSilenceSuggestCandidate) map[string]any {
	return map[string]any{
		"tool_name":                    "clip.strip_silence.apply",
		"action_count":                 len(candidate.PendingActions),
		"strip_region_count":           candidate.StripRegionCount,
		"no_cleanup_needed_clip_count": stripSilenceSuggestNoCleanupCount(candidate),
		"requires_user_confirm":        len(candidate.PendingActions) > 0,
	}
}

func stripSilenceSuggestNoCleanupCount(candidate stripSilenceSuggestCandidate) int {
	count := 0
	for _, analysis := range candidate.Analyses {
		if len(mapRowsFromAny(analysis["strip_regions"])) == 0 {
			count++
		}
	}
	return count
}

func (h *Harness) applyStripSilenceBatch(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, fmt.Errorf("clip.strip_silence.apply_batch requires a connected kernel")
	}
	applyCommands := stripSilenceBatchApplyCommands(cmd)
	if len(applyCommands) == 0 {
		return nil, fmt.Errorf("clip.strip_silence.apply_batch requires pending_actions with strip_regions")
	}
	applySpec, ok := h.catalog.LookupTool("clip.strip_silence.apply")
	if !ok {
		return nil, fmt.Errorf("clip.strip_silence.apply is not cataloged")
	}
	applySpec.RefreshAfter = false

	appliedClipIDs := make([]string, 0, len(applyCommands))
	errors := make([]map[string]any, 0)
	appliedClipCount := 0
	failedClipCount := 0
	protectedSkipCount := 0
	appliedRegionCount := 0
	createdClipIDs := make([]string, 0)
	removedClipIDs := make([]string, 0)
	affectedClipIDs := make([]string, 0)
	scannedClipCount := stripSilenceBatchNonNegativeInt(cmd, "scanned_clip_count", "target_count", "source_target_count")
	analyzedClipCount := stripSilenceBatchNonNegativeInt(cmd, "analyzed_clip_count", "source_analyzed_clip_count", "analysis_count")
	analysisFailedClipCount := stripSilenceBatchNonNegativeInt(cmd, "analysis_failed_clip_count", "source_analysis_failed_clip_count")
	noCleanupNeededClipCount := stripSilenceBatchNonNegativeInt(cmd, "no_cleanup_needed_clip_count", "no_cleanup_clip_count")

	for _, applyCmd := range applyCommands {
		execution := h.executeKernelCommand(ctx, applySpec, applyCmd)
		clipID := firstString(applyCmd, "clip_id")
		trackID := firstString(applyCmd, "track_id")
		if execution.err != nil {
			row := stripSilenceBatchErrorRow(applyCmd, execution.err.Error())
			if firstString(row, "disposition") == "protected_skip" {
				protectedSkipCount++
			} else {
				failedClipCount++
			}
			errors = append(errors, row)
			continue
		}
		reply := execution.reply
		if !kernelReplySucceeded(reply) {
			row := stripSilenceBatchErrorRow(applyCmd, firstNonEmpty(firstString(reply, "message"), firstString(reply, "error"), "clip.strip_silence.apply failed"))
			if firstString(row, "disposition") == "protected_skip" {
				protectedSkipCount++
			} else {
				failedClipCount++
			}
			errors = append(errors, row)
			continue
		}
		appliedClipCount++
		if clipID != "" {
			appliedClipIDs = appendUniqueString(appliedClipIDs, clipID)
		}
		if trackID == "" {
			trackID = firstString(reply, "track_id")
		}
		appliedRegions := int(numberFromAny(reply["applied_region_count"]))
		if appliedRegions <= 0 {
			appliedRegions = int(numberFromAny(reply["deleted_region_count"]))
		}
		if appliedRegions <= 0 {
			appliedRegions = len(mapRowsFromAny(applyCmd["strip_regions"]))
		}
		appliedRegionCount += appliedRegions
		createdClipIDs = appendUniqueStrings(createdClipIDs, stringSliceFromAny(reply["created_clip_ids"])...)
		removedClipIDs = appendUniqueStrings(removedClipIDs, stringSliceFromAny(reply["removed_clip_ids"])...)
		affectedClipIDs = appendUniqueStrings(affectedClipIDs, stringSliceFromAny(reply["affected_clip_ids"])...)
		if len(affectedClipIDs) == 0 && clipID != "" {
			affectedClipIDs = appendUniqueString(affectedClipIDs, clipID)
		}
		_ = trackID
	}
	if noCleanupNeededClipCount <= 0 && analyzedClipCount > 0 {
		noCleanupNeededClipCount = analyzedClipCount - len(applyCommands)
		if noCleanupNeededClipCount < 0 {
			noCleanupNeededClipCount = 0
		}
	}
	if scannedClipCount <= 0 && analyzedClipCount > 0 {
		scannedClipCount = analyzedClipCount + analysisFailedClipCount
	}
	skippedClipCount := protectedSkipCount

	status := "ok"
	if failedClipCount > 0 || skippedClipCount > 0 || analysisFailedClipCount > 0 {
		status = "partial"
	}
	if appliedClipCount == 0 && (failedClipCount > 0 || skippedClipCount == 0) {
		status = "error"
	}
	if appliedClipCount > 0 {
		h.refreshShadow(ctx, "clip.strip_silence.apply_batch")
	}
	out := map[string]any{
		"status":                       status,
		"schema_version":               "clip.strip_silence.apply_batch.v0",
		"message":                      "Strip Silence batch applied",
		"ui_action":                    "strip_silence_applied",
		"pending_action_count":         len(applyCommands),
		"scanned_clip_count":           scannedClipCount,
		"analyzed_clip_count":          analyzedClipCount,
		"no_cleanup_needed_clip_count": noCleanupNeededClipCount,
		"skipped_clip_count":           skippedClipCount,
		"protected_skip_clip_count":    protectedSkipCount,
		"analysis_failed_clip_count":   analysisFailedClipCount,
		"applied_clip_count":           appliedClipCount,
		"failed_clip_count":            failedClipCount,
		"true_failed_clip_count":       failedClipCount,
		"applied_region_count":         appliedRegionCount,
		"created_clip_count":           len(createdClipIDs),
		"removed_clip_count":           len(removedClipIDs),
		"affected_clip_count":          len(affectedClipIDs),
		"applied_clip_ids":             previewStrings(appliedClipIDs, 20),
		"created_clip_ids":             previewStrings(createdClipIDs, 20),
		"removed_clip_ids":             previewStrings(removedClipIDs, 20),
		"affected_clip_ids":            previewStrings(affectedClipIDs, 20),
		"result_truncated":             len(appliedClipIDs) > 20 || len(createdClipIDs) > 20 || len(removedClipIDs) > 20 || len(affectedClipIDs) > 20,
		"internal_apply_tool":          "clip.strip_silence.apply",
		"internal_apply_count":         len(applyCommands),
		"response_granularity":         "summary",
	}
	if len(errors) > 0 {
		out["errors"] = errors
	}
	return out, nil
}

func stripSilenceBatchApplyCommands(cmd map[string]any) []map[string]any {
	rows := mapRowsFromAny(cmd["pending_actions"])
	if len(rows) == 0 {
		rows = mapRowsFromAny(cmd["actions"])
	}
	if len(rows) == 0 && len(mapRowsFromAny(cmd["strip_regions"])) > 0 {
		rows = []map[string]any{cmd}
	}
	out := make([]map[string]any, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		args := mapAnyFromAny(row["args"])
		if len(args) == 0 {
			args = mapAnyFromAny(row["command"])
		}
		if len(args) == 0 {
			args = row
		}
		toolName := strings.ToLower(strings.TrimSpace(firstNonEmpty(
			firstString(row, "tool_name", "tool"),
			firstString(args, "tool", "cmd", "command"),
		)))
		if toolName != "" && toolName != "clip.strip_silence.apply" {
			continue
		}
		regions := mapRowsFromAny(args["strip_regions"])
		if len(regions) == 0 {
			continue
		}
		applyCmd := cloneAnyMap(args)
		for _, key := range []string{"tool", "tool_name", "command", "requires_confirmation", "risk", "rollback_hint", "target_ids"} {
			delete(applyCmd, key)
		}
		if _, ok := applyCmd["allow_remove_entire_clip"]; !ok {
			if value, exists := cmd["allow_remove_entire_clip"]; exists && !isEmptyValue(value) {
				applyCmd["allow_remove_entire_clip"] = value
			}
		}
		applyCmd["cmd"] = "clip.strip_silence.apply"
		key := stripSilenceBatchApplyKey(applyCmd)
		if key != "" && seen[key] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		out = append(out, applyCmd)
	}
	return out
}

func stripSilenceBatchApplyKey(cmd map[string]any) string {
	regions := mapRowsFromAny(cmd["strip_regions"])
	firstStart := ""
	firstEnd := ""
	if len(regions) > 0 {
		firstStart = fmt.Sprint(regions[0]["start_seconds"])
		firstEnd = fmt.Sprint(regions[0]["end_seconds"])
	}
	return strings.Join([]string{
		firstString(cmd, "clip_id"),
		firstString(cmd, "track_id"),
		firstString(cmd, "analysis_id"),
		fmt.Sprint(len(regions)),
		firstStart,
		firstEnd,
	}, "|")
}

func stripSilenceBatchErrorRow(cmd map[string]any, message string) map[string]any {
	disposition, reason := stripSilenceBatchErrorDisposition(message)
	return map[string]any{
		"clip_id":     firstString(cmd, "clip_id"),
		"track_id":    firstString(cmd, "track_id"),
		"analysis_id": firstString(cmd, "analysis_id"),
		"error":       strings.TrimSpace(message),
		"disposition": disposition,
		"reason":      reason,
	}
}

func stripSilenceBatchErrorDisposition(message string) (string, string) {
	text := strings.ToLower(strings.TrimSpace(message))
	if strings.Contains(text, "would remove the entire clip") || strings.Contains(text, "allow_remove_entire_clip") {
		return "protected_skip", "would_remove_entire_clip_protected"
	}
	return "failed", "apply_failed"
}

func stripSilenceBatchNonNegativeInt(row map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := row[key]
		if !ok || isEmptyValue(value) {
			continue
		}
		n := int(numberFromAny(value))
		if n >= 0 {
			return n
		}
	}
	return 0
}

func appendUniqueStrings(base []string, values ...string) []string {
	for _, value := range values {
		base = appendUniqueString(base, value)
	}
	return base
}

func appendUniqueString(base []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return base
	}
	for _, existing := range base {
		if existing == value {
			return base
		}
	}
	return append(base, value)
}

func previewStrings(values []string, limit int) []string {
	if limit <= 0 || len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}

func roundFloat(value float64, places int) float64 {
	if places < 0 {
		places = 0
	}
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}
