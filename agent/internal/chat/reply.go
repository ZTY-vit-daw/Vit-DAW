package chat

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/policy"
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
	case "clone_clip":
		return "已复制选中的 clip。"
	case "remove_clips":
		return "已删除选中的 clip。"
	case "import_audio", "import_media_to_track":
		name := targetTrackName(before, after, cmd)
		clipName := firstText(result, "clip_name", "name")
		if clipName != "" {
			return fmt.Sprintf("已把 %s 导入到 %s。", clipName, name)
		}
		return fmt.Sprintf("已把音频导入到 %s。", name)
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
	return name == "get_project_state" || name == "list_tracks" || name == "project_health_check"
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
