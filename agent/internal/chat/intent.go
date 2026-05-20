package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var localSecondsPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|秒)`)
var localAudioPathPattern = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))`)

func synthesizeLocalDAWCommands(userText string, requestContext map[string]any) []map[string]any {
	text := strings.TrimSpace(userText)
	if text == "" {
		return nil
	}
	if isAudioImportText(text) {
		args := selectedImportArgs(requestContext)
		if path := firstLocalAudioPath(text); path != "" {
			args["file_path"] = path
		}
		if query := localImportSearchQuery(text); query != "" {
			args["asset_query"] = query
		}
		return []map[string]any{{
			"tool": "clip.import_media_to_track",
			"args": args,
		}}
	}
	if !mentionsClip(text) {
		return nil
	}

	switch {
	case isClipDeleteText(text):
		return []map[string]any{{
			"tool": "clip.remove",
			"args": selectedClipArgs(requestContext, true),
		}}
	case isClipSplitText(text):
		args := selectedClipArgs(requestContext, false)
		if seconds, ok := firstLocalSeconds(text); ok {
			args["split_time"] = seconds
			args["time_unit"] = "seconds"
		}
		if trackID := selectedClipTrackID(requestContext); trackID != "" {
			args["track_id"] = trackID
		}
		if mentionsPlayheadText(text) {
			if playhead := firstContextText(requestContext, "playhead_seconds", "current_playhead_seconds", "transport_position_seconds"); playhead != "" {
				args["playhead_seconds"] = playhead
			}
		}
		return []map[string]any{{
			"tool": "clip.split",
			"args": args,
		}}
	case isClipCloneText(text):
		args := selectedClipArgs(requestContext, false)
		if seconds, ok := firstLocalSeconds(text); ok && !isRelativeMoveText(text) {
			args["new_start"] = seconds
			args["time_unit"] = "seconds"
		}
		if trackID := selectedClipTrackID(requestContext); trackID != "" {
			args["target_track_id"] = trackID
		}
		return []map[string]any{{
			"tool": "clip.clone",
			"args": args,
		}}
	case isClipMoveText(text) && hasSeconds(text):
		args := selectedClipArgs(requestContext, false)
		if seconds, ok := firstLocalSeconds(text); ok && !isRelativeMoveText(text) {
			args["new_start"] = seconds
			args["time_unit"] = "seconds"
		}
		if trackID := selectedClipTrackID(requestContext); trackID != "" {
			args["source_track_id"] = trackID
			args["target_track_id"] = trackID
		}
		return []map[string]any{{
			"tool": "clip.move",
			"args": args,
		}}
	case isClipResizeText(text) && hasSeconds(text):
		args := selectedClipArgs(requestContext, false)
		if seconds, ok := firstLocalSeconds(text); ok {
			args["new_length"] = seconds
			args["time_unit"] = "seconds"
		}
		if trackID := selectedClipTrackID(requestContext); trackID != "" {
			args["track_id"] = trackID
		}
		return []map[string]any{{
			"tool": "clip.resize",
			"args": args,
		}}
	case isClipSelectText(text):
		args := map[string]any{}
		if name := localClipSelectName(text); name != "" {
			args["clip_name"] = name
		} else {
			args = selectedClipArgs(requestContext, false)
		}
		if trackID := selectedClipTrackID(requestContext); trackID != "" {
			args["track_id"] = trackID
		}
		return []map[string]any{{
			"tool": "clip.select",
			"args": args,
		}}
	default:
		return nil
	}
}

func selectedImportArgs(requestContext map[string]any) map[string]any {
	args := map[string]any{}
	if trackID := strings.TrimSpace(firstContextText(requestContext, "selected_track_id", "focused_track_id", "track_id")); trackID != "" {
		args["track_id"] = trackID
	}
	if path := strings.TrimSpace(firstContextText(requestContext, "selected_library_file_path", "library_file_path")); path != "" {
		args["selected_library_file_path"] = path
	}
	args["start_time"] = 0.0
	args["media_type"] = "audio"
	args["mode"] = "non_destructive"
	return args
}

func selectedClipArgs(requestContext map[string]any, allowMany bool) map[string]any {
	args := map[string]any{}
	ids := contextStringSlice(requestContext["selected_clip_ids"])
	if len(ids) == 0 {
		if id := strings.TrimSpace(firstContextText(requestContext, "selected_clip_id", "primary_selected_clip_id", "clip_id")); id != "" {
			ids = []string{id}
		}
	}
	if allowMany && len(ids) > 0 {
		args["clip_ids"] = ids
	} else if len(ids) == 1 {
		args["clip_id"] = ids[0]
	}
	return args
}

func selectedClipTrackID(requestContext map[string]any) string {
	return strings.TrimSpace(firstContextText(requestContext, "selected_clip_track_id", "focused_track_id", "selected_track_id", "track_id"))
}

func firstContextText(requestContext map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := requestContext[key]; ok {
			s := strings.TrimSpace(strings.Trim(fmt.Sprint(v), `"`))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func mentionsClip(text string) bool {
	return containsAnyFold(text, "clip", "片段", "音频块", "素材块")
}

func isAudioImportText(text string) bool {
	if firstLocalAudioPath(text) != "" {
		return containsAnyFold(text, "导入", "加入", "放到", "放进", "拖入", "import", "add")
	}
	return containsAnyFold(text, "导入", "加入", "放到", "放进", "拖入", "import", "add") &&
		containsAnyFold(text, "音频", "素材", "资料库", "采样", "sample", "audio", "library", "wav", "mp3", "flac", "loop")
}

func firstLocalAudioPath(text string) string {
	match := localAudioPathPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(match[1], " \t\r\n\"'`“”‘’.,，。;；:：)）]】")
}

func localImportSearchQuery(text string) string {
	if firstLocalAudioPath(text) != "" {
		return ""
	}
	if !containsAnyFold(text, "搜索", "查找", "找", "search", "find") {
		return ""
	}
	cleaned := text
	for _, phrase := range []string{
		"搜索", "查找", "找一下", "找到", "找", "并导入", "然后导入", "导入", "放到", "放进", "拖入",
		"资料库", "素材库", "当前轨道", "选中轨道", "这个轨道", "这条轨道", "音频", "素材",
		"search", "find", "import", "add", "audio", "sample", "current track", "selected track", "track", "and", "to",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
	}
	replacer := strings.NewReplacer("，", " ", "。", " ", ",", " ", ".", " ", "；", " ", ";", " ", "：", " ", ":", " ", "（", " ", "）", " ", "(", " ", ")", " ", "并", " ", "到", " ", "给", " ")
	cleaned = replacer.Replace(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

func isClipDeleteText(text string) bool {
	return containsAnyFold(text, "删除", "移除", "删掉", "delete", "remove")
}

func isClipSelectText(text string) bool {
	return containsAnyFold(text, "选中", "选择", "聚焦", "select", "choose", "focus")
}

func localClipSelectName(text string) string {
	cleaned := strings.TrimSpace(text)
	for _, phrase := range []string{
		"选中 clip", "选择 clip", "聚焦 clip", "选中这个 clip", "选择这个 clip", "聚焦这个 clip",
		"选中", "选择", "聚焦",
		"select clip", "choose clip", "focus clip", "select the clip", "choose the clip", "focus the clip",
		"select", "choose", "focus", "clip",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
		cleaned = strings.ReplaceAll(cleaned, strings.Title(phrase), " ")
	}
	replacer := strings.NewReplacer("\"", " ", "'", " ", "`", " ", ":", " ", ",", " ", ".", " ", "(", " ", ")", " ")
	return strings.Join(strings.Fields(replacer.Replace(cleaned)), " ")
}

func isClipSplitText(text string) bool {
	return containsAnyFold(text, "切开", "切分", "分割", "剪开", "切一刀", "split", "cut")
}

func isClipCloneText(text string) bool {
	return containsAnyFold(text, "复制", "克隆", "拷贝", "再来一份", "duplicate", "clone", "copy")
}

func isClipMoveText(text string) bool {
	return containsAnyFold(text,
		"移动", "移到", "挪到", "拖到", "位置", "起点", "开始位置",
		"move", "position", "start")
}

func isClipResizeText(text string) bool {
	if containsAnyFold(text, "位置", "起点", "开始位置", "移动", "移到", "挪到", "拖到", "move", "position", "start") {
		return false
	}
	return containsAnyFold(text,
		"长度", "时长", "持续", "裁到", "裁成", "剪到", "剪成", "剪短",
		"缩到", "缩短", "拉长", "拉到", "伸到", "改成", "改为", "变成",
		"调整到", "调成", "resize", "length", "duration", "trim", "shorten")
}

func hasSeconds(text string) bool {
	_, ok := firstLocalSeconds(text)
	return ok
}

func firstLocalSeconds(text string) (float64, bool) {
	match := localSecondsPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = -value
	}
	return value, true
}

func isRelativeMoveText(text string) bool {
	return containsAnyFold(text, "向前", "前移", "提前", "向后", "后移", "推后", "earlier", "left", "backward", "later", "right", "forward")
}

func mentionsPlayheadText(text string) bool {
	return containsAnyFold(text, "播放头", "当前位置", "这里", "此处", "当前时间", "playhead", "cursor", "current position", "here")
}

func containsAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
