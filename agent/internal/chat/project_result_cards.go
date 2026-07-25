package chat

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/tools"
)

func projectResultCardsFromExecuted(executed []map[string]any) []map[string]any {
	candidates := make([]map[string]any, 0, len(executed))
	target := map[string]any{}
	for _, entry := range executed {
		if !projectResultExecutionCandidate(entry) {
			continue
		}
		candidates = append(candidates, entry)
		target = mergeProjectResultTargets(target, projectResultTargetFromExecution(entry))
	}
	if len(candidates) == 0 || len(target) == 0 {
		return nil
	}
	return []map[string]any{{
		"kind":       "project_result",
		"type":       "project_result",
		"id":         "project_result_" + projectResultStableID(candidates),
		"executions": candidates,
		"target":     target,
	}}
}

func projectResultCardsFromExecutedWithAB(executed []map[string]any, observe executor.Result) []map[string]any {
	cards := projectResultCardsFromExecuted(executed)
	ab := projectResultABCardFromObserve(observe)
	if len(cards) == 0 {
		candidates := make([]map[string]any, 0, len(executed))
		for _, entry := range executed {
			if projectResultExecutionCandidate(entry) {
				candidates = append(candidates, entry)
			}
		}
		if len(candidates) == 0 {
			return nil
		}
		return []map[string]any{{
			"kind":       "project_result",
			"type":       "project_result",
			"id":         "project_result_ab_" + projectResultStableID(candidates),
			"executions": candidates,
			"ab_result":  ab,
		}}
	}
	if len(ab) > 0 {
		cards[0]["ab_result"] = ab
	}
	return cards
}

func projectResultABCardFromObserve(observe executor.Result) map[string]any {
	ab := pendingMixTickABResultFromObserve(observe)
	if len(ab) == 0 {
		return map[string]any{
			"schema_version": "project_result_ab.v1",
			"status":         "missing",
			"reason":         "ab_result_missing",
			"display_title":  "AB Result：不可信",
			"display_body":   "原因：ab_result_missing；没有拿到同一观测点的前后 L2 渲染观测，未标记为已验证。",
		}
	}
	status := strings.ToLower(strings.TrimSpace(cleanContextText(ab["status"])))
	if status == "" {
		status = "missing"
	}
	reason := firstNonEmpty(cleanContextText(ab["reason"]), "quality_gate_not_ready")
	tapPoint := cleanContextText(ab["tap_point"])
	renderMode := cleanContextText(ab["render_mode"])
	out := map[string]any{
		"schema_version":          "project_result_ab.v1",
		"status":                  status,
		"reason":                  reason,
		"tap_point":               tapPoint,
		"render_mode":             renderMode,
		"before_observation_id":   cleanContextText(ab["before_observation_id"]),
		"after_observation_id":    cleanContextText(ab["after_observation_id"]),
		"before_evidence_ref":     cleanContextText(ab["before_evidence_ref"]),
		"after_evidence_ref":      cleanContextText(ab["after_evidence_ref"]),
		"before_render_revision":  cleanContextText(ab["before_render_revision"]),
		"after_render_revision":   cleanContextText(ab["after_render_revision"]),
		"render_revision_changed": status == "ready",
	}
	if status == "ready" {
		out["display_title"] = "AB Result：可信"
		out["display_body"] = projectResultABDisplayBody(tapPoint, renderMode, pendingMixTickABDeltaText(mapValue(ab["delta"])))
	} else {
		out["display_title"] = "AB Result：不可信"
		out["display_body"] = projectResultABUntrustedBody(reason, tapPoint, renderMode)
	}
	if delta := pendingMixTickABDeltaText(mapValue(ab["delta"])); delta != "" {
		out["delta_summary"] = delta
	}
	return out
}

func projectResultABDisplayBody(tapPoint, renderMode, delta string) string {
	parts := []string{}
	if delta != "" {
		parts = append(parts, "关键变化："+delta)
	}
	if tapPoint != "" {
		point := tapPoint
		if renderMode != "" {
			point += " / " + renderMode
		}
		parts = append(parts, "观测点："+point)
	}
	if len(parts) == 0 {
		return "同一观测点的前后 L2 渲染观测已通过。"
	}
	return strings.Join(parts, "；")
}

func projectResultABUntrustedBody(reason, tapPoint, renderMode string) string {
	parts := []string{"原因：" + firstNonEmpty(reason, "quality_gate_not_ready")}
	if tapPoint != "" {
		point := tapPoint
		if renderMode != "" {
			point += " / " + renderMode
		}
		parts = append(parts, "观测点："+point)
	}
	parts = append(parts, "未标记为已验证")
	return strings.Join(parts, "；")
}

// chatResponseTurnFailed reports whether a turn ended in failure, so callers can
// avoid synthesising project-result cards for it. A blocked capability turn
// (capabilityCanaryBlockedResponse) sets both Error and a failed GoalStatus
// while still carrying read-only kernel replies from the evidence it collected
// before bailing out.
func chatResponseTurnFailed(resp ChatResponse) bool {
	if strings.TrimSpace(resp.Error) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(resp.GoalStatus)) {
	case "failed", "error", "blocked", "cancelled", "canceled":
		return true
	}
	return false
}

func projectResultExecutionCandidate(entry map[string]any) bool {
	if len(entry) == 0 || !projectResultExecutionSucceeded(entry) {
		return false
	}
	commandName := projectResultCommandName(entry)
	if commandName == "" {
		return false
	}
	if mutates, known := projectResultCatalogMutates(commandName); known {
		if !mutates {
			return false
		}
	} else if projectResultReadOnlyCommand(commandName) {
		return false
	}
	if strings.Contains(commandName, "browser.") || strings.Contains(commandName, "artifact.") || strings.Contains(commandName, "media.register") {
		return false
	}
	target := projectResultTargetFromExecution(entry)
	if len(target) > 0 {
		return true
	}
	return projectResultLooksLikeProjectMutation(commandName, projectResultUIText(entry))
}

// projectResultCatalogMutates reports the tool catalog's authoritative
// MutatesProject flag for commandName, and whether the catalog knows this
// command at all. Read-only commands (e.g. plugin_grabber.explain_controls,
// plugin_grabber.get_project_profiles) contain "plugin"/"control" substrings
// that would otherwise trip the keyword-based mutation heuristic below.
func projectResultCatalogMutates(commandName string) (mutates bool, known bool) {
	catalog := tools.DefaultCatalog()
	if spec, ok := catalog.LookupCommand(commandName); ok {
		return spec.MutatesProject, true
	}
	if spec, ok := catalog.LookupTool(commandName); ok {
		return spec.MutatesProject, true
	}
	return false, false
}

func projectResultExecutionSucceeded(entry map[string]any) bool {
	status := strings.ToLower(firstProjectResultText(entry, "status"))
	result := projectResultMap(entry["result"])
	resultStatus := strings.ToLower(firstProjectResultText(result, "status"))
	if projectResultFailureStatus(status) || projectResultFailureStatus(resultStatus) {
		return false
	}
	if status == "" {
		status = resultStatus
	}
	return status == "ok" || status == "success" || status == "succeeded" || status == "completed"
}

func projectResultFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "error", "failed", "failure":
		return true
	default:
		return false
	}
}

func projectResultCommandName(entry map[string]any) string {
	result := projectResultMap(entry["result"])
	verification := projectResultMap(entry["verification"])
	return strings.ToLower(firstNonEmpty(
		firstProjectResultText(entry, "command_name", "tool", "cmd"),
		firstProjectResultText(result, "command_name", "tool", "cmd", "command"),
		firstProjectResultText(verification, "command_name", "tool"),
	))
}

func projectResultUIText(entry map[string]any) string {
	result := projectResultMap(entry["result"])
	return strings.ToLower(firstNonEmpty(
		firstProjectResultText(entry, "ui_action"),
		firstProjectResultText(result, "ui_action"),
	))
}

func projectResultReadOnlyCommand(commandName string) bool {
	commandName = strings.ToLower(strings.TrimSpace(commandName))
	if commandName == "" {
		return false
	}
	return strings.HasPrefix(commandName, "get_") ||
		strings.HasPrefix(commandName, "list_") ||
		strings.Contains(commandName, ".list") ||
		strings.Contains(commandName, ".read") ||
		strings.Contains(commandName, "read_clip") ||
		strings.Contains(commandName, "project.state")
}

func projectResultLooksLikeProjectMutation(commandName, uiAction string) bool {
	joined := strings.ToLower(commandName + " " + uiAction)
	for _, token := range []string{"track", "midi", "clip", "plugin", "rack", "control", "transport"} {
		if strings.Contains(joined, token) {
			return true
		}
	}
	return false
}

func projectResultTargetFromExecution(entry map[string]any) map[string]any {
	result := projectResultMap(entry["result"])
	verification := projectResultMap(entry["verification"])
	evidence := projectResultMap(verification["evidence"])
	target := map[string]any{}
	if trackID := firstNonEmpty(
		firstProjectResultText(result, "track_id", "target_track_id", "selected_track_id", "source_track_id"),
		firstProjectResultText(entry, "track_id", "target_track_id", "selected_track_id", "source_track_id"),
		firstProjectResultText(evidence, "observed_track_id", "expected_track_id", "track_id"),
	); trackID != "" {
		target["track_id"] = trackID
	}
	if trackName := firstNonEmpty(
		firstProjectResultText(result, "track_name"),
		firstProjectResultText(entry, "track_name"),
		firstProjectResultText(evidence, "observed_track_name", "expected_track_name", "track_name"),
	); trackName != "" {
		target["track_name"] = trackName
	}
	if clipID := firstNonEmpty(
		firstProjectResultText(result, "clip_id", "new_clip_id", "created_clip_id", "target_clip_id", "selected_clip_id"),
		firstProjectResultText(entry, "clip_id", "new_clip_id", "created_clip_id", "target_clip_id", "selected_clip_id"),
		firstProjectResultText(evidence, "observed_clip_id", "expected_clip_id", "clip_id"),
	); clipID != "" {
		target["clip_id"] = clipID
	}
	if clipName := firstNonEmpty(
		firstProjectResultText(result, "clip_name"),
		firstProjectResultText(entry, "clip_name"),
		firstProjectResultText(evidence, "observed_clip_name", "expected_clip_name", "clip_name"),
	); clipName != "" {
		target["clip_name"] = clipName
	}
	if pluginID := firstNonEmpty(
		firstProjectResultText(result, "plugin_id", "plugin_item_id", "node_id", "selected_plugin_id"),
		firstProjectResultText(entry, "plugin_id", "plugin_item_id", "node_id", "selected_plugin_id"),
		firstProjectResultText(evidence, "observed_plugin_id", "expected_plugin_id", "observed_node_id", "expected_node_id", "plugin_id", "node_id"),
	); pluginID != "" {
		target["plugin_id"] = pluginID
	}
	if pluginName := firstNonEmpty(
		firstProjectResultText(result, "plugin_name", "descriptive_name"),
		firstProjectResultText(entry, "plugin_name", "descriptive_name"),
		firstProjectResultText(evidence, "observed_plugin_name", "expected_plugin_name", "plugin_name"),
	); pluginName != "" {
		target["plugin_name"] = pluginName
	}
	if start := firstProjectResultNumber(result, "start_seconds", "start_time_seconds", "position_seconds", "new_start_seconds"); start != nil {
		target["start_seconds"] = *start
	}
	if startBeats := firstProjectResultNumber(result, "start_beats", "start_beat"); startBeats != nil {
		target["start_beats"] = *startBeats
	}
	target["can_audition"] = projectResultCanAudition(projectResultCommandName(entry), projectResultUIText(entry), target)
	return compactProjectResultTarget(target)
}

func mergeProjectResultTargets(current, incoming map[string]any) map[string]any {
	if len(current) == 0 {
		return compactProjectResultTarget(incoming)
	}
	if len(incoming) == 0 {
		return compactProjectResultTarget(current)
	}
	out := make(map[string]any, len(current)+len(incoming))
	for key, value := range current {
		out[key] = value
	}
	incomingHasClip := strings.TrimSpace(fmt.Sprint(incoming["clip_id"])) != ""
	incomingHasPlugin := strings.TrimSpace(fmt.Sprint(incoming["plugin_id"])) != ""
	for key, value := range incoming {
		if isEmptyProjectResultValue(value) {
			continue
		}
		if _, exists := out[key]; !exists || incomingHasClip || incomingHasPlugin {
			out[key] = value
		}
	}
	return compactProjectResultTarget(out)
}

func compactProjectResultTarget(target map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"track_id", "track_name", "clip_id", "clip_name", "plugin_id", "plugin_name", "start_seconds", "start_beats", "can_audition"} {
		if value, ok := target[key]; ok && !isEmptyProjectResultValue(value) {
			out[key] = value
		}
	}
	if len(out) == 1 {
		if _, onlyCanAudition := out["can_audition"]; onlyCanAudition {
			return nil
		}
	}
	return out
}

func projectResultCanAudition(commandName, uiAction string, target map[string]any) bool {
	if strings.TrimSpace(fmt.Sprint(target["clip_id"])) != "" || strings.TrimSpace(fmt.Sprint(target["plugin_id"])) != "" {
		return true
	}
	if strings.TrimSpace(fmt.Sprint(target["track_id"])) == "" {
		return false
	}
	joined := strings.ToLower(commandName + " " + uiAction)
	if strings.Contains(joined, "add_track") || strings.Contains(joined, "create_track") || strings.Contains(joined, "rename_track") || strings.Contains(joined, "delete_track") {
		return false
	}
	for _, token := range []string{"mute", "solo", "arm", "volume", "pan", "mixer", "plugin", "rack", "control"} {
		if strings.Contains(joined, token) {
			return true
		}
	}
	return false
}

func projectResultStableID(executed []map[string]any) string {
	for _, entry := range executed {
		if id := firstProjectResultText(entry, "agent_action_id", "tool_call_id"); id != "" {
			return id
		}
	}
	for _, entry := range executed {
		if command := projectResultCommandName(entry); command != "" {
			return strings.NewReplacer(".", "_", " ", "_").Replace(command)
		}
	}
	return randomID()
}

func projectResultMap(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func firstProjectResultText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if row == nil {
			return ""
		}
		value, ok := row[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func firstProjectResultNumber(row map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if row == nil {
			return nil
		}
		value, ok := row[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return &typed
		case float32:
			v := float64(typed)
			return &v
		case int:
			v := float64(typed)
			return &v
		case int64:
			v := float64(typed)
			return &v
		case json.Number:
			if v, err := typed.Float64(); err == nil {
				return &v
			}
		case string:
			if v, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return &v
			}
		}
	}
	return nil
}

func isEmptyProjectResultValue(value any) bool {
	if value == nil {
		return true
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text == "" || text == "<nil>"
}
