package chat

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var localSecondsPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|秒)`)

func synthesizeLocalDAWCommands(userText string, requestContext map[string]any) []map[string]any {
	text := strings.TrimSpace(userText)
	if text == "" || !mentionsClip(text) {
		return nil
	}

	switch {
	case isClipDeleteText(text):
		return []map[string]any{{
			"tool": "clip.remove",
			"args": selectedClipArgs(requestContext, true),
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
	default:
		return nil
	}
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

func isClipDeleteText(text string) bool {
	return containsAnyFold(text, "删除", "移除", "删掉", "delete", "remove")
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

func containsAnyFold(text string, needles ...string) bool {
	lower := strings.ToLower(text)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
