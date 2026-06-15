package presenter

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"vit-daw-agent/internal/policy"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func confirmationReply(decisions []policy.Decision) string {
	if len(decisions) <= 1 {
		return "这个操作需要确认后执行。"
	}
	return fmt.Sprintf("这 %d 个操作需要确认后执行。", len(decisions))
}

func friendlyExecutionError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "clip target") || strings.Contains(msg, "clip_id") {
		return "我还没有找到要操作的 clip。请先在时间线上选中目标 MIDI clip，或先新建并选中一个 MIDI clip。"
	}
	if strings.Contains(msg, "track target") || strings.Contains(msg, "track_id") {
		return "我还不确定要操作哪条轨道。请告诉我轨道名，或者说第几条轨道。"
	}
	return "执行时出错：" + msg
}

func sanitizeUserReply(reply string, state map[string]any, userText string) string {
	if wantsTechnicalIDs(userText) {
		return reply
	}
	out := reply
	for _, row := range trackRows(state) {
		name := displayTrackName(row)
		for _, key := range []string{"track_id", "id"} {
			id := firstText(row, key)
			if id != "" && id != name {
				out = strings.ReplaceAll(out, id, name)
			}
		}
	}
	return out
}

func executedReply(before, after map[string]any, decisions []policy.Decision, replies []map[string]any) string {
	if len(decisions) == 0 {
		return ""
	}

	mutating := false
	undoable := false
	for _, d := range decisions {
		if !isStateRead(d.Name) {
			mutating = true
		}
		if d.Risk == policy.RiskUndoable {
			undoable = true
		}
	}
	if !mutating {
		if text := executedReadReply(decisions, replies); text != "" {
			return text
		}
		return formatTrackList(after)
	}

	parts := make([]string, 0, len(decisions))
	for i, d := range decisions {
		if isStateRead(d.Name) {
			continue
		}
		var reply map[string]any
		if i < len(replies) {
			reply = replies[i]
		}
		if text := executedCommandReply(before, after, d, reply); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	reply := strings.Join(parts, "\n")
	if undoable {
		reply = addUndoHint(reply)
	}
	return reply
}

func executedReadReply(decisions []policy.Decision, replies []map[string]any) string {
	parts := make([]string, 0, len(decisions))
	for i, d := range decisions {
		var reply map[string]any
		if i < len(replies) {
			reply = replies[i]
		}
		switch d.Name {
		case "plugin_search":
			if text := formatPluginSearchResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "plugin_semantic_search":
			if text := formatPluginSemanticSearchResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "plugin_semantic_get":
			if text := formatPluginSemanticGetResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "plugin_list_available":
			if text := formatPluginListResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "get_plugin_parameters":
			if text := formatPluginParametersResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "plugin_grabber_explain_controls":
			if text := plugingrabber.FormatContextPackReply(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "plugin_grabber_get_project_profiles":
			if text := formatPluginGrabberProfilesResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "get_midi_clip_notes", "get_midi_clip_data":
			if text := formatMidiNotesResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "web_search":
			if text := formatWebSearchResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		case "web_fetch":
			if text := formatWebFetchResult(replyResult(reply)); text != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func executedCommandReply(before, after map[string]any, d policy.Decision, reply map[string]any) string {
	result := replyResult(reply)
	cmd := expandedCommand(d.Command)
	switch d.Name {
	case "rename_track":
		oldName := trackNameByID(before, textValue(cmd["track_id"]))
		if oldName == "" {
			oldName = firstText(result, "old_name")
		}
		newName := trackNameByID(after, textValue(cmd["track_id"]))
		if newName == "" {
			newName = firstText(result, "track_name", "name", "new_name")
		}
		if newName == "" {
			newName = firstText(cmd, "name", "new_name", "track_name")
		}
		if oldName != "" && newName != "" && oldName != newName {
			return fmt.Sprintf("已把 %s 重命名为 %s。", oldName, newName)
		}
		if newName != "" {
			return fmt.Sprintf("已把轨道重命名为 %s。", newName)
		}
		return "已重命名轨道。"
	case "set_mute":
		name := targetTrackName(before, after, cmd)
		value, ok := firstBool(result, "mute", "muted")
		if !ok {
			value, ok = firstBool(cmd, "mute", "muted", "value", "enabled")
		}
		if ok && !value {
			return fmt.Sprintf("已取消 %s 的静音。", name)
		}
		return fmt.Sprintf("已静音 %s。", name)
	case "set_solo":
		name := targetTrackName(before, after, cmd)
		value, ok := firstBool(result, "solo", "is_solo")
		if !ok {
			value, ok = firstBool(cmd, "solo", "is_solo", "value", "enabled")
		}
		if ok && !value {
			return fmt.Sprintf("已取消 %s 的独奏。", name)
		}
		return fmt.Sprintf("已独奏 %s。", name)
	case "arm_track":
		name := targetTrackName(before, after, cmd)
		value, ok := firstBool(result, "is_armed", "arm", "armed")
		if !ok {
			value, ok = firstBool(cmd, "is_armed", "arm", "armed", "value", "enabled")
		}
		if ok && !value {
			return fmt.Sprintf("已取消 %s 的录音准备。", name)
		}
		return fmt.Sprintf("已将 %s 设为录音准备。", name)
	case "add_track", "add_audio_track", "append_ghost_track":
		return formatTrackCreated(after)
	case "set_tempo":
		if bpm := firstText(cmd, "tempo", "bpm", "value"); bpm != "" {
			return fmt.Sprintf("已把速度设为 %s BPM。", bpm)
		}
		return "已更新工程速度。"
	case "play":
		return "已开始播放。"
	case "stop":
		return "已停止播放。"
	case "return_to_zero":
		return "已回到工程开头。"
	case "start_recording":
		return "已开始录音。"
	case "stop_recording":
		return "已停止录音。"
	case "move_clip":
		return "已移动选中的 clip。"
	case "resize_clip":
		return "已调整选中的 clip。"
	case "split_clip":
		return "已切开选中的 clip。"
	case "clone_clip":
		return "已复制选中的 clip。"
	case "remove_clips":
		return "已删除选中的 clip。"
	case "select_clip":
		return "已选中 clip。"
	case "insert_midi_clip", "create_midi_clip":
		clipID := firstText(result, "new_clip_id", "clip_id", "created_clip_id")
		if clipID != "" {
			return fmt.Sprintf("已创建 MIDI clip：id=%s。", clipID)
		}
		return "已创建 MIDI clip。"
	case "get_midi_clip_notes", "get_midi_clip_data":
		return formatMidiNotesResult(result)
	case "apply_midi_note_patch", "add_midi_notes", "add_midi_notes_bulk", "mutate_midi_notes", "delete_midi_notes":
		inserted := firstText(result, "inserted_count", "added_count")
		deleted := firstText(result, "deleted_count", "removed_count")
		mutated := firstText(result, "mutated_count")
		quantized := firstText(result, "quantized_count")
		parts := []string{}
		if inserted != "" && inserted != "0" {
			parts = append(parts, "inserted "+inserted)
		}
		if deleted != "" && deleted != "0" {
			parts = append(parts, "deleted "+deleted)
		}
		if mutated != "" && mutated != "0" {
			parts = append(parts, "changed "+mutated)
		}
		if quantized != "" && quantized != "0" {
			parts = append(parts, "quantized "+quantized)
		}
		if len(parts) == 0 {
			return "Updated MIDI clip."
		}
		return "Updated MIDI clip: " + strings.Join(parts, ", ") + "."
	case "import_audio", "import_media_to_track":
		name := targetTrackName(before, after, cmd)
		clipName := firstText(result, "clip_name", "name")
		if clipName != "" {
			return fmt.Sprintf("已把 %s 导入到 %s。", clipName, name)
		}
		return fmt.Sprintf("已把音频导入到 %s。", name)
	case "import_midi_to_track":
		name := targetTrackName(before, after, cmd)
		clipName := firstText(result, "clip_name", "name")
		noteCount := firstText(result, "note_count", "notes_imported")
		if clipName != "" && noteCount != "" {
			return fmt.Sprintf("Imported MIDI %s to %s (%s notes).", clipName, name, noteCount)
		}
		if clipName != "" {
			return fmt.Sprintf("Imported MIDI %s to %s.", clipName, name)
		}
		return fmt.Sprintf("Imported MIDI to %s.", name)
	case "plugin_grabber_upsert_project_profile":
		count := len(stringSliceValue(cmd["quick_control_ids"]))
		if count > 0 {
			return fmt.Sprintf("已保存 Plugin Skill，包含 %d 个快捷控制。", count)
		}
		return "已保存 Plugin Skill。"
	case "plugin_grabber_remove_project_profile":
		if firstText(result, "removed") == "false" {
			return "Plugin Skill 原本就不存在。"
		}
		return "已移除 Plugin Skill。"
	case "scan_plugins":
		return formatPluginScanResult(result)
	case "undo":
		return "已撤销上一项操作。"
	case "redo":
		return "已重做上一项操作。"
	default:
		if d.Risk == policy.RiskUndoable {
			return "已完成这项可撤销操作。"
		}
		return "已完成。"
	}
}

func expandedCommand(cmd map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range cmd {
		out[k] = v
	}
	for _, key := range []string{"args", "params"} {
		nested, ok := cmd[key].(map[string]any)
		if !ok {
			continue
		}
		for k, v := range nested {
			if firstText(out, k) == "" {
				out[k] = v
			}
		}
	}
	return out
}

func formatTrackList(state map[string]any) string {
	tracks := trackRows(state)
	if len(tracks) == 0 {
		return "当前没有用户轨道。"
	}
	names := make([]string, 0, len(tracks))
	for _, row := range tracks {
		names = append(names, displayTrackName(row))
	}
	return fmt.Sprintf("当前有 %d 条轨道：%s。", len(names), strings.Join(names, "、"))
}

func formatTrackCreated(state map[string]any) string {
	tracks := trackRows(state)
	if len(tracks) == 0 {
		return "已新建轨道。"
	}
	return fmt.Sprintf("已新建轨道。当前有 %d 条轨道：%s。", len(tracks), strings.Join(trackNames(tracks), "、"))
}

func addUndoHint(reply string) string {
	reply = strings.TrimSpace(reply)
	if reply == "" || strings.Contains(reply, "可撤销") {
		return reply
	}
	if strings.Contains(reply, "\n") {
		return reply + "\n这些操作可撤销。"
	}
	return strings.TrimSuffix(reply, "。") + "，可撤销。"
}

func isStateRead(name string) bool {
	return name == "get_project_state" || name == "list_tracks" || name == "project_health_check" || name == "get_midi_clip_notes" || name == "get_midi_clip_data" || name == "get_plugin_parameters" || name == "plugin_grabber_explain_controls" || name == "plugin_grabber_get_project_profiles" || name == "plugin_search" || name == "plugin_semantic_search" || name == "plugin_semantic_get" || name == "plugin_list_available" || name == "web_search" || name == "web_fetch"
}

func wantsTechnicalIDs(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "track_id") ||
		strings.Contains(lower, "clip_id") ||
		strings.Contains(lower, "plugin_id") ||
		strings.Contains(lower, " id") ||
		strings.Contains(lower, "id ") ||
		strings.Contains(text, "ID") ||
		strings.Contains(text, "编号") ||
		strings.Contains(text, "标识")
}

func targetTrackName(before, after map[string]any, cmd map[string]any) string {
	id := textValue(cmd["track_id"])
	for _, state := range []map[string]any{after, before} {
		if name := trackNameByID(state, id); name != "" {
			return name
		}
	}
	if name := firstText(cmd, "track_name"); name != "" {
		return name
	}
	return "该轨道"
}

func trackNameByID(state map[string]any, id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	for _, row := range trackRows(state) {
		if textValue(row["track_id"]) == id || textValue(row["id"]) == id {
			return displayTrackName(row)
		}
	}
	return ""
}

func trackNames(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, displayTrackName(row))
	}
	return out
}

func trackRows(state map[string]any) []map[string]any {
	if state == nil {
		return nil
	}
	switch rows := state["tracks"].(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		return nil
	}
}

func replyResult(reply map[string]any) map[string]any {
	if reply == nil {
		return nil
	}
	result, _ := reply["result"].(map[string]any)
	return result
}

func noteRows(row map[string]any) []map[string]any {
	if row == nil {
		return nil
	}
	switch rows := row["notes"].(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if note, ok := it.(map[string]any); ok {
				out = append(out, note)
			}
		}
		return out
	default:
		return nil
	}
}

func mapRowsValue(value any) []map[string]any {
	switch rows := value.(type) {
	case []map[string]any:
		return rows
	case []any:
		out := make([]map[string]any, 0, len(rows))
		for _, it := range rows {
			if row, ok := it.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out
	default:
		b, err := json.Marshal(value)
		if err == nil {
			var out []map[string]any
			if json.Unmarshal(b, &out) == nil {
				return out
			}
		}
		return nil
	}
}

func mapValue(value any) map[string]any {
	switch row := value.(type) {
	case map[string]any:
		return row
	default:
		b, err := json.Marshal(value)
		if err != nil {
			return nil
		}
		var out map[string]any
		if json.Unmarshal(b, &out) != nil {
			return nil
		}
		return out
	}
}

func stringSliceValue(value any) []string {
	switch rows := value.(type) {
	case []string:
		out := make([]string, 0, len(rows))
		for _, it := range rows {
			if s := strings.TrimSpace(it); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(rows))
		for _, it := range rows {
			if s := strings.TrimSpace(fmt.Sprint(it)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := strings.TrimSpace(fmt.Sprint(value)); s != "" && s != "<nil>" {
			return []string{s}
		}
		return nil
	}
}

func formatPluginSearchResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	query := firstText(result, "query")
	rows := mapRowsValue(result["plugins"])
	if text := formatPluginCandidateSummary(rows, result, query, "search"); text != "" {
		return text
	}
	if len(rows) == 0 {
		if query != "" {
			return fmt.Sprintf("没有在当前插件库里找到 %s。请先在设置页扫描插件目录，或确认插件名称。", query)
		}
		return "当前插件库没有搜索结果。请先在设置页扫描插件目录。"
	}
	lines := []string{fmt.Sprintf("找到 %d 个插件候选：", len(rows))}
	if query != "" {
		lines[0] = fmt.Sprintf("找到 %d 个插件候选（%s）：", len(rows), query)
	}
	for i, row := range rows {
		if i >= 10 {
			lines = append(lines, fmt.Sprintf("另有 %d 个候选未显示。", len(rows)-i))
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, formatPluginIndexRow(row)))
	}
	return strings.Join(lines, "\n")
}

func formatWebSearchResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	query := firstText(result, "query")
	source := webSourceLabel(firstText(result, "source"))
	rows := mapRowsValue(result["results"])
	if len(rows) == 0 {
		if query != "" {
			return fmt.Sprintf("联网搜索完成，但没有找到可展示的结果：%s。", query)
		}
		return "联网搜索完成，但没有找到可展示的结果。"
	}
	lines := []string{}
	if answer := inferWebSearchAnswer(query, rows); answer != "" {
		lines = append(lines, answer)
	}
	header := fmt.Sprintf("联网搜索结果（%s）：", source)
	if query != "" {
		header = fmt.Sprintf("联网搜索结果（%s，%s）：", source, query)
	}
	lines = append(lines, header)
	limit := len(rows)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		row := rows[i]
		title := firstText(row, "title")
		url := firstText(row, "url")
		snippet := firstText(row, "snippet")
		line := fmt.Sprintf("%d. %s", i+1, firstNonEmpty(title, url))
		if url != "" && url != title {
			line += " - " + url
		}
		lines = append(lines, line)
		if snippet != "" {
			lines = append(lines, "   "+snippet)
		}
	}
	if len(rows) > limit {
		lines = append(lines, fmt.Sprintf("另有 %d 条结果未显示。", len(rows)-limit))
	}
	return strings.Join(lines, "\n")
}

func formatWebFetchResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	url := firstText(result, "final_url", "url")
	status := firstText(result, "status_code")
	title := htmlTitle(firstText(result, "body"))
	if title != "" {
		return fmt.Sprintf("已读取网页：%s（HTTP %s）\n%s", title, firstNonEmpty(status, "?"), url)
	}
	if url != "" {
		return fmt.Sprintf("已读取网页（HTTP %s）：%s", firstNonEmpty(status, "?"), url)
	}
	return "已读取网页。"
}

func webSourceLabel(source string) string {
	switch source {
	case "bing_html":
		return "Bing"
	case "duckduckgo_html":
		return "DuckDuckGo"
	default:
		return firstNonEmpty(source, "web")
	}
}

func inferWebSearchAnswer(query string, rows []map[string]any) string {
	if !strings.Contains(query, "作者") && !strings.Contains(query, "作曲") && !strings.Contains(strings.ToLower(query), "composer") {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:作者|作曲者|作曲家|曲作者|作曲)\s*[是为:：]\s*([\p{Han}A-Za-z·]{2,16})`),
		regexp.MustCompile(`由\s*([\p{Han}A-Za-z·]{2,16})\s*(?:创作|作曲|谱写)`),
		regexp.MustCompile(`([\p{Han}A-Za-z·]{2,12})\s*(?:创作|作曲|谱写)`),
	}
	for _, row := range rows {
		text := firstText(row, "title") + "。" + firstText(row, "snippet")
		text = strings.Join(strings.Fields(text), "")
		for _, pattern := range patterns {
			m := pattern.FindStringSubmatch(text)
			if len(m) < 2 {
				continue
			}
			name := strings.Trim(m[1], " ，。、《》“”\"'：:")
			if name != "" && !strings.Contains(name, "百科") && !strings.Contains(name, "搜索") {
				return "从搜索结果看，可能答案是：" + name + "。"
			}
		}
	}
	return ""
}

func htmlTitle(body string) string {
	if body == "" {
		return ""
	}
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	tagRe := regexp.MustCompile(`<[^>]+>`)
	return strings.Join(strings.Fields(html.UnescapeString(tagRe.ReplaceAllString(m[1], " "))), " ")
}

func formatPluginSemanticSearchResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	rows := mapRowsValue(result["entries"])
	if len(rows) == 0 {
		rows = mapRowsValue(result["plugins"])
	}
	if len(rows) == 0 {
		if msg := firstText(result, "message"); msg != "" {
			return "插件语义资料库还没有可用候选：" + msg
		}
		return "插件语义资料库里没有找到合适候选。"
	}
	prefix := "插件语义资料库推荐："
	if firstText(result, "transient") == "true" {
		prefix = "根据当前插件索引临时推荐："
	}
	lines := []string{prefix}
	for i, row := range rows {
		if i >= 5 {
			lines = append(lines, fmt.Sprintf("另有 %d 个候选未显示。", len(rows)-i))
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, formatPluginSemanticRow(row)))
	}
	return strings.Join(lines, "\n")
}

func formatPluginSemanticGetResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	if firstText(result, "found") == "false" {
		return "插件语义资料库里没有找到这个插件。"
	}
	entry := mapValue(result["entry"])
	if len(entry) == 0 {
		return ""
	}
	return "插件语义资料库条目：\n" + formatPluginSemanticRow(entry)
}

func formatPluginSemanticRow(row map[string]any) string {
	base := formatPluginIndexRow(row)
	typ := firstText(row, "primary_type")
	conf := firstText(row, "confidence")
	score := firstText(row, "search_score")
	meta := []string{}
	if typ != "" {
		meta = append(meta, "类型 "+typ)
	}
	if conf != "" {
		meta = append(meta, "置信度 "+conf)
	}
	if score != "" {
		meta = append(meta, "匹配 "+score)
	}
	if len(meta) == 0 {
		return base
	}
	return base + " - " + strings.Join(meta, "，")
}

func formatPluginListResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	rows := mapRowsValue(result["plugins"])
	if text := formatPluginCandidateSummary(rows, result, "", "list"); text != "" {
		return text
	}
	if len(rows) == 0 {
		return "当前插件库为空。请先在设置页扫描插件目录。"
	}
	lines := []string{fmt.Sprintf("当前插件库可用插件（显示 %d 个）：", len(rows))}
	for i, row := range rows {
		if i >= 10 {
			lines = append(lines, fmt.Sprintf("另有 %d 个插件未显示。", len(rows)-i))
			break
		}
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, formatPluginIndexRow(row)))
	}
	return strings.Join(lines, "\n")
}

func formatPluginCandidateSummary(rows []map[string]any, result map[string]any, query, mode string) string {
	if len(rows) == 0 {
		return ""
	}
	totalText := firstText(result, "match_count", "plugin_count")
	if totalText == "" {
		totalText = fmt.Sprint(len(rows))
	}
	display := firstPluginRows(rows, 8)
	header := fmt.Sprintf("当前插件库约有 %s 个可用插件，显示前 %d 个代表项。", totalText, len(display))
	if mode == "search" {
		if query != "" {
			header = fmt.Sprintf("找到 %s 个插件候选（%s），显示前 %d 个。", totalText, query, len(display))
		} else {
			header = fmt.Sprintf("找到 %s 个插件候选，显示前 %d 个。", totalText, len(display))
		}
	}
	lines := []string{header}
	if summary := pluginTypeSummary(rows); summary != "" {
		lines = append(lines, "类型概览："+summary)
	}
	for i, row := range display {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, formatPluginIndexRow(row)))
	}
	if hidden := hiddenPluginCount(totalText, len(rows), len(display)); hidden > 0 {
		lines = append(lines, fmt.Sprintf("另有 %d 个未显示；可以继续指定类型或名称筛选。", hidden))
	}
	return strings.Join(lines, "\n")
}

func firstPluginRows(rows []map[string]any, limit int) []map[string]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func pluginTypeSummary(rows []map[string]any) string {
	counts := map[string]int{}
	for _, row := range rows {
		typ := firstText(row, "primary_type")
		if typ == "" {
			typ = inferPluginTypeLabel(row)
		}
		if typ == "" {
			typ = "unknown"
		}
		counts[typ]++
	}
	order := []string{"eq", "compressor", "dynamics", "reverb", "delay", "limiter", "analyzer", "meter", "synth", "instrument", "unknown"}
	parts := []string{}
	seen := map[string]bool{}
	for _, typ := range order {
		if n := counts[typ]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", pluginTypeDisplayName(typ), n))
			seen[typ] = true
		}
	}
	for typ, n := range counts {
		if !seen[typ] {
			parts = append(parts, fmt.Sprintf("%s %d", pluginTypeDisplayName(typ), n))
		}
	}
	if len(parts) > 6 {
		parts = parts[:6]
	}
	return strings.Join(parts, "、")
}

func inferPluginTypeLabel(row map[string]any) string {
	text := strings.ToLower(strings.Join([]string{
		firstText(row, "name", "descriptive_name"),
		firstText(row, "category"),
		firstText(row, "plugin_path", "path", "file_or_identifier"),
	}, " "))
	switch {
	case strings.Contains(text, "eq") || strings.Contains(text, "equalizer"):
		return "eq"
	case strings.Contains(text, "compressor") || strings.Contains(text, "dynamics") || strings.Contains(text, " comp"):
		return "compressor"
	case strings.Contains(text, "reverb") || strings.Contains(text, "verb"):
		return "reverb"
	case strings.Contains(text, "delay") || strings.Contains(text, "echo"):
		return "delay"
	case strings.Contains(text, "limiter"):
		return "limiter"
	case strings.Contains(text, "analyzer") || strings.Contains(text, "meter") || strings.Contains(text, "spectrum"):
		return "analyzer"
	case strings.Contains(text, "synth") || strings.Contains(text, "instrument"):
		return "synth"
	default:
		return ""
	}
}

func pluginTypeDisplayName(typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "eq":
		return "均衡"
	case "compressor", "dynamics":
		return "压缩/动态"
	case "reverb":
		return "混响"
	case "delay":
		return "延迟"
	case "limiter":
		return "限制"
	case "analyzer", "meter":
		return "分析/电平"
	case "synth", "instrument":
		return "乐器"
	default:
		return "其他"
	}
}

func hiddenPluginCount(totalText string, rowCount, shown int) int {
	total, err := strconv.Atoi(strings.TrimSpace(totalText))
	if err != nil {
		total = rowCount
	}
	if total > shown {
		return total - shown
	}
	if rowCount > shown {
		return rowCount - shown
	}
	return 0
}

func formatPluginScanResult(result map[string]any) string {
	if result == nil {
		return "插件扫描已完成。"
	}
	rows := mapRowsValue(result["plugins"])
	paths := stringSliceValue(result["scanned_paths"])
	head := fmt.Sprintf("插件扫描完成，当前索引里有 %d 个插件。", len(rows))
	if count := firstText(result, "plugin_count"); count != "" {
		head = fmt.Sprintf("插件扫描完成，当前索引里有 %s 个插件。", count)
	}
	parts := []string{head}
	if len(paths) > 0 {
		parts = append(parts, "扫描路径："+strings.Join(firstStringLimit(paths, 4), ", "))
	}
	if len(rows) > 0 {
		lines := []string{"前几个候选："}
		for i, row := range rows {
			if i >= 5 {
				break
			}
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, formatPluginIndexRow(row)))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	return strings.Join(parts, "\n")
}

func formatPluginIndexRow(row map[string]any) string {
	name := firstText(row, "name", "descriptive_name")
	if name == "" {
		name = firstText(row, "plugin_name")
	}
	format := firstText(row, "format")
	maker := firstText(row, "manufacturer")
	path := firstText(row, "plugin_path", "path", "file_or_identifier", "file_path")
	meta := []string{}
	if format != "" {
		meta = append(meta, format)
	}
	if maker != "" {
		meta = append(meta, maker)
	}
	if name == "" {
		name = path
	}
	text := name
	if len(meta) > 0 {
		text += " [" + strings.Join(meta, ", ") + "]"
	}
	if path != "" {
		text += " - " + path
	}
	return text
}

func formatPluginParametersResult(result map[string]any) string {
	if result == nil {
		return ""
	}
	pluginName := ""
	if identity, ok := result["plugin_identity"].(map[string]any); ok {
		pluginName = firstText(identity, "plugin_name")
	}
	if pluginName == "" {
		pluginName = firstText(result, "plugin_id")
	}
	paramCount := firstText(result, "parameter_count")
	quickRows := mapRowsValue(result["quick_controls"])
	quickLabels := make([]string, 0, len(quickRows))
	for _, row := range quickRows {
		label := firstText(row, "label", "param_id")
		if label != "" {
			quickLabels = append(quickLabels, label)
		}
		if len(quickLabels) >= 8 {
			break
		}
	}
	groups := mapRowsValue(result["recommended_groups"])
	groupNames := make([]string, 0, len(groups))
	for _, row := range groups {
		if name := firstText(row, "name"); name != "" {
			groupNames = append(groupNames, name)
		}
		if len(groupNames) >= 8 {
			break
		}
	}
	source := firstText(result, "profile_source")
	if source == "" {
		source = "heuristic"
	}
	parts := []string{fmt.Sprintf("Plugin parameters for %s: %s parameters, profile=%s.", pluginName, paramCount, source)}
	if len(quickLabels) > 0 {
		parts = append(parts, "Quick controls: "+strings.Join(quickLabels, ", ")+".")
	}
	if len(groupNames) > 0 {
		parts = append(parts, "Groups: "+strings.Join(groupNames, ", ")+".")
	}
	return strings.Join(parts, "\n")
}

func formatPluginGrabberProfilesResult(result map[string]any) string {
	profiles := mapRowsValue(result["plugin_grabber_profiles"])
	if len(profiles) == 0 {
		return "No project plugin grabber profiles saved."
	}
	names := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		name := firstText(profile, "profile_id")
		if identity, ok := profile["plugin_identity"].(map[string]any); ok {
			if pluginName := firstText(identity, "plugin_name"); pluginName != "" {
				name = pluginName
			}
		}
		if name != "" {
			names = append(names, name)
		}
		if len(names) >= 8 {
			break
		}
	}
	return fmt.Sprintf("Project plugin grabber profiles: %d saved. %s", len(profiles), strings.Join(names, ", "))
}

func formatMidiNotesResult(result map[string]any) string {
	notes := noteRows(result)
	count := len(notes)
	if count == 0 {
		return "当前 MIDI clip 有 0 个 notes。"
	}
	parts := make([]string, 0, count)
	for i, note := range notes {
		if i >= 8 {
			parts = append(parts, fmt.Sprintf("... 还有 %d 个 notes", count-i))
			break
		}
		fields := []string{}
		if id := firstText(note, "id", "note_id"); id != "" {
			fields = append(fields, "id="+id)
		}
		if pitch := firstText(note, "pitch"); pitch != "" {
			fields = append(fields, "pitch="+pitch)
		}
		if start := firstText(note, "start", "beat"); start != "" {
			fields = append(fields, "start="+start)
		}
		if length := firstText(note, "length", "duration"); length != "" {
			fields = append(fields, "length="+length)
		}
		if velocity := firstText(note, "velocity"); velocity != "" {
			fields = append(fields, "velocity="+velocity)
		}
		if len(fields) == 0 {
			parts = append(parts, fmt.Sprintf("%d. note", i+1))
			continue
		}
		parts = append(parts, fmt.Sprintf("%d. %s", i+1, strings.Join(fields, " ")))
	}
	return fmt.Sprintf("当前 MIDI clip 有 %d 个 notes：\n%s", count, strings.Join(parts, "\n"))
}

func displayTrackName(row map[string]any) string {
	if name := firstText(row, "name", "track_name"); name != "" {
		return name
	}
	if idx := firstText(row, "user_track_index"); idx != "" {
		return "轨道 " + idx
	}
	return "未命名轨道"
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok {
			if s := textValue(v); s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func textValue(v any) string {
	return strings.TrimSpace(fmt.Sprint(v))
}

func firstBool(row map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		v, ok := row[key]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case bool:
			return x, true
		case string:
			s := strings.ToLower(strings.TrimSpace(x))
			if s == "true" || s == "1" || s == "yes" || s == "on" {
				return true, true
			}
			if s == "false" || s == "0" || s == "no" || s == "off" {
				return false, true
			}
		case float64:
			return x != 0, true
		case int:
			return x != 0, true
		}
	}
	return false, false
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
func ConfirmationReply(decisions []policy.Decision) string {
	return confirmationReply(decisions)
}

func FriendlyExecutionError(err error) string {
	return friendlyExecutionError(err)
}

func SanitizeUserReply(reply string, state map[string]any, userText string) string {
	return sanitizeUserReply(reply, state, userText)
}

func ExecutedReply(before, after map[string]any, decisions []policy.Decision, replies []map[string]any) string {
	return executedReply(before, after, decisions, replies)
}

func MapRowsValue(value any) []map[string]any {
	return mapRowsValue(value)
}

func FirstBool(row map[string]any, keys ...string) (bool, bool) {
	return firstBool(row, keys...)
}
