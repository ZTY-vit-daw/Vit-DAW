package chat

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/policy"
)

const stripSilenceResponsePreviewLimit = 12

func compactAgentLoopExecutedForResponse(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, compactAgentLoopExecutionForResponse(row))
	}
	return out
}

func compactAgentLoopExecutionForResponse(row map[string]any) map[string]any {
	if !executionLooksLikeStripSilence(row) {
		return row
	}
	out := map[string]any{}
	for _, key := range []string{"status", "tool_call_id", "agent_action_id", "tool", "command_name", "preview", "undo_label", "error"} {
		if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
			out[key] = value
		}
	}
	result := mapValue(row["result"])
	if len(result) > 0 {
		out["result"] = compactStripSilenceResultForResponse(result)
	}
	return out
}

func compactAgentLoopDecisionsForResponse(decisions []policy.Decision) []policy.Decision {
	if len(decisions) == 0 {
		return nil
	}
	out := make([]policy.Decision, 0, len(decisions))
	for _, decision := range decisions {
		next := decision
		next.Command = compactStripSilenceCommandForResponse(decision.Command)
		out = append(out, next)
	}
	return out
}

func compactStripSilenceChatResponseForTransport(resp *ChatResponse) {
	if resp == nil {
		return
	}
	resp.ExecutedKernelReply = compactAgentLoopExecutedForResponse(resp.ExecutedKernelReply)
	resp.Commands = compactAgentLoopDecisionsForResponse(resp.Commands)
	for i := range resp.InteractionRequests {
		resp.InteractionRequests[i] = compactStripSilenceInteractionRequestForTransport(resp.InteractionRequests[i])
	}
}

func compactStripSilenceInvokeResponseForTransport(resp harness.InvokeResponse) harness.InvokeResponse {
	if !invokeResponseLooksLikeStripSilence(resp) {
		return resp
	}
	resp.Result = compactStripSilenceResultForResponse(resp.Result)
	return resp
}

func invokeResponseLooksLikeStripSilence(resp harness.InvokeResponse) bool {
	text := strings.ToLower(strings.TrimSpace(firstNonEmpty(resp.Tool, resp.CommandName)))
	if strings.Contains(text, "clip.strip_silence") {
		return true
	}
	return stripSilenceResultKind(resp.Result) != ""
}

func compactStripSilenceInteractionRequestForTransport(req AgentInteractionRequest) AgentInteractionRequest {
	req.Payload = compactStripSilenceInteractionPayloadForTransport(req.Payload)
	req.Data = compactStripSilenceInteractionPayloadForTransport(req.Data)
	return req
}

func compactStripSilenceInteractionPayloadForTransport(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return payload
	}
	commands, ok := payload["commands"].([]policy.Decision)
	if !ok {
		return payload
	}
	out := copyStringAnyMap(payload)
	out["commands"] = compactAgentLoopDecisionsForResponse(commands)
	return out
}

func executionLooksLikeStripSilence(row map[string]any) bool {
	if len(row) == 0 {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		cleanContextText(row["tool"]),
		cleanContextText(row["command_name"]),
	)))
	if strings.Contains(text, "clip.strip_silence") {
		return true
	}
	result := mapValue(row["result"])
	if len(result) == 0 {
		return false
	}
	return stripSilenceResultKind(result) != ""
}

func compactStripSilenceResultForResponse(result map[string]any) map[string]any {
	switch stripSilenceResultKind(result) {
	case "suggest":
		return compactStripSilenceSuggestResult(result)
	case "apply", "apply_batch":
		return compactStripSilenceApplyResult(result)
	default:
		return result
	}
}

func stripSilenceResultKind(result map[string]any) string {
	if len(result) == 0 {
		return ""
	}
	schema := strings.ToLower(cleanContextText(result["schema_version"]))
	if strings.Contains(schema, "clip.strip_silence.suggest") {
		return "suggest"
	}
	if strings.Contains(schema, "clip.strip_silence.apply_batch") {
		return "apply_batch"
	}
	tool := strings.ToLower(firstNonEmpty(
		cleanContextText(result["tool"]),
		cleanContextText(result["tool_name"]),
		cleanContextText(result["command_name"]),
		cleanContextText(result["cmd"]),
	))
	if strings.Contains(tool, "clip.strip_silence.suggest") {
		return "suggest"
	}
	if strings.Contains(tool, "clip.strip_silence.apply_batch") {
		return "apply_batch"
	}
	if strings.Contains(tool, "clip.strip_silence.apply") {
		return "apply"
	}
	if _, ok := result["pending_actions"]; ok {
		return "suggest"
	}
	if _, ok := result["pending_action"]; ok {
		return "suggest"
	}
	if _, ok := result["strip_regions"]; ok {
		return "apply"
	}
	return ""
}

func compactStripSilenceSuggestResult(result map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"status", "schema_version", "message", "scope", "target_count", "analyzed_clip_count",
		"analysis_failed_clip_count", "no_cleanup_needed_clip_count",
		"confidence", "confidence_score", "risk_level", "recommended_params", "risks", "reasoning",
		"candidate_results", "pending_action_count", "pending_confirmation", "pending_action_summary",
		"warnings",
	} {
		if value, ok := result[key]; ok && !stripSilenceCompactEmpty(value) {
			out[key] = value
		}
	}
	if analysis := mapValue(result["analysis"]); len(analysis) > 0 {
		out["analysis"] = compactStripSilenceAnalysisForResponse(analysis)
	}
	actions := stripSilenceActionRowsFromResult(result)
	if len(actions) > 0 {
		out["pending_action_preview"] = compactStripSilenceActionPreview(actions, stripSilenceResponsePreviewLimit)
		out["pending_action_count"] = len(actions)
	}
	return out
}

func compactStripSilenceApplyResult(result map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"status", "schema_version", "message", "scope", "clip_id", "track_id", "analysis_id",
		"pending_action_count", "scanned_clip_count", "analyzed_clip_count",
		"no_cleanup_needed_clip_count", "skipped_clip_count", "protected_skip_clip_count",
		"analysis_failed_clip_count", "applied_clip_count", "failed_clip_count", "true_failed_clip_count", "applied_region_count",
		"deleted_region_count", "strip_region_count", "created_clip_count", "removed_clip_count",
		"affected_clip_count", "would_remove_entire_clip",
	} {
		if value, ok := result[key]; ok && !stripSilenceCompactEmpty(value) {
			out[key] = value
		}
	}
	for _, key := range []string{"created_clip_ids", "removed_clip_ids", "affected_clip_ids", "applied_clip_ids"} {
		if values := compactStringPreview(result[key], stripSilenceResponsePreviewLimit); len(values) > 0 {
			out[key+"_preview"] = values
			out[key+"_count"] = len(stringSliceFromAnyLoose(result[key]))
		}
	}
	if errors := compactStripSilenceErrorPreview(result["errors"], stripSilenceResponsePreviewLimit); len(errors) > 0 {
		out["errors"] = errors
	}
	if regions := mapRowsFromAny(result["strip_regions"]); len(regions) > 0 {
		out["strip_region_count"] = len(regions)
		out["strip_region_preview"] = compactStripSilenceRegionPreview(regions, 3)
	}
	return out
}

func compactStripSilenceAnalysisForResponse(analysis map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{
		"threshold_dbfs", "analysis_count", "strip_region_count", "keep_segment_count",
		"analysis_failed_clip_count", "no_cleanup_needed_clip_count",
		"strip_seconds", "analyzed_seconds", "strip_ratio", "would_remove_entire_clip",
	} {
		if value, ok := analysis[key]; ok && !stripSilenceCompactEmpty(value) {
			out[key] = value
		}
	}
	if analyses := mapRowsFromAny(analysis["analyses"]); len(analyses) > 0 {
		out["analysis_preview"] = compactStripSilenceAnalysisPreview(analyses, stripSilenceResponsePreviewLimit)
		out["analysis_count"] = len(analyses)
	}
	return out
}

func compactStripSilenceCommandForResponse(cmd map[string]any) map[string]any {
	if len(cmd) == 0 {
		return cmd
	}
	name := strings.ToLower(firstNonEmpty(
		cleanContextText(cmd["tool"]),
		cleanContextText(cmd["cmd"]),
		cleanContextText(cmd["command"]),
		cleanContextText(cmd["action"]),
	))
	if !strings.Contains(name, "clip.strip_silence") {
		return cmd
	}
	out := map[string]any{}
	for _, key := range []string{
		"tool", "cmd", "command", "action", "clip_id", "track_id", "analysis_id",
		"allow_remove_entire_clip", "pending_action_count",
	} {
		if value, ok := cmd[key]; ok && !stripSilenceCompactEmpty(value) {
			out[key] = value
		}
	}
	actions := mapRowsFromAny(cmd["pending_actions"])
	if len(actions) > 0 {
		out["pending_action_count"] = len(actions)
		out["pending_action_preview"] = compactStripSilenceActionPreview(actions, stripSilenceResponsePreviewLimit)
		return out
	}
	regions := mapRowsFromAny(cmd["strip_regions"])
	if len(regions) > 0 {
		out["strip_region_count"] = len(regions)
		out["strip_region_preview"] = compactStripSilenceRegionPreview(regions, 3)
	}
	return out
}

func stripSilenceActionRowsFromResult(result map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	if row := mapValue(result["pending_action"]); len(row) > 0 {
		rows = append(rows, row)
	}
	rows = append(rows, mapRowsFromAny(result["pending_actions"])...)
	return rows
}

func compactStripSilenceActionPreview(actions []map[string]any, limit int) []map[string]any {
	if limit <= 0 {
		limit = stripSilenceResponsePreviewLimit
	}
	if len(actions) < limit {
		limit = len(actions)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		action := actions[i]
		args := mapValue(action["args"])
		if len(args) == 0 {
			args = action
		}
		regions := mapRowsFromAny(args["strip_regions"])
		item := map[string]any{
			"tool_name":          firstNonEmpty(cleanContextText(action["tool_name"]), cleanContextText(action["tool"]), "clip.strip_silence.apply"),
			"clip_id":            cleanContextText(args["clip_id"]),
			"track_id":           cleanContextText(args["track_id"]),
			"analysis_id":        cleanContextText(args["analysis_id"]),
			"strip_region_count": len(regions),
		}
		out = append(out, item)
	}
	return out
}

func compactStripSilenceAnalysisPreview(analyses []map[string]any, limit int) []map[string]any {
	if limit <= 0 {
		limit = stripSilenceResponsePreviewLimit
	}
	if len(analyses) < limit {
		limit = len(analyses)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := analyses[i]
		item := map[string]any{}
		for _, key := range []string{
			"clip_id", "track_id", "analysis_id", "threshold_dbfs", "strip_region_count",
			"keep_segment_count", "strip_seconds", "analyzed_seconds", "would_remove_entire_clip",
		} {
			if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
				item[key] = value
			}
		}
		if regions := mapRowsFromAny(row["strip_regions"]); len(regions) > 0 {
			item["strip_region_count"] = len(regions)
		}
		out = append(out, item)
	}
	return out
}

func compactStripSilenceRegionPreview(regions []map[string]any, limit int) []map[string]any {
	if limit <= 0 {
		limit = 3
	}
	if len(regions) < limit {
		limit = len(regions)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := regions[i]
		item := map[string]any{}
		for _, key := range []string{"range_id", "clip_id", "track_id", "start_seconds", "end_seconds", "duration_seconds"} {
			if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
				item[key] = value
			}
		}
		out = append(out, item)
	}
	return out
}

func compactStripSilenceErrorPreview(value any, limit int) []map[string]any {
	if limit <= 0 {
		limit = stripSilenceResponsePreviewLimit
	}
	rows := mapRowsFromAny(value)
	if len(rows) == 0 {
		return nil
	}
	if len(rows) < limit {
		limit = len(rows)
	}
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		row := rows[i]
		item := map[string]any{}
		for _, key := range []string{"clip_id", "track_id", "analysis_id", "disposition", "reason", "error", "message"} {
			if value, ok := row[key]; ok && !stripSilenceCompactEmpty(value) {
				item[key] = value
			}
		}
		out = append(out, item)
	}
	return out
}

func compactStringPreview(value any, limit int) []string {
	values := stringSliceFromAnyLoose(value)
	if limit <= 0 {
		limit = stripSilenceResponsePreviewLimit
	}
	if len(values) < limit {
		limit = len(values)
	}
	if limit <= 0 {
		return nil
	}
	return append([]string(nil), values[:limit]...)
}

func stringSliceFromAnyLoose(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func stripSilenceCompactEmpty(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>" || text == "[]"
}
