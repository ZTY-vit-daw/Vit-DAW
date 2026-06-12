package conversation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var localSecondsPattern = regexp.MustCompile(`(?i)([-+]?\d+(?:\.\d+)?)\s*(?:s|sec|secs|second|seconds|绉)`)
var localAudioPathPattern = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:wav|mp3|flac|ogg|oga|aif|aiff|m4a|wma))`)
var localMidiPathPattern = regexp.MustCompile(`(?i)((?:[a-z]:|\\\\[^\\/]+[\\/][^\\/]+)[\\/][^\r\n"<>|?*]+?\.(?:mid|midi))`)
var localSecondsCompatPattern = regexp.MustCompile(`([-+]?\d+(?:\.\d+)?)\s*(?:秒|绉|缁)`)
var localTrackIndexPattern = regexp.MustCompile(`(?i)(?:track\s*(\d+)|(\d+)\s*track)`)

func SynthesizeLocalDAWCommands(userText string, requestContext map[string]any) []map[string]any {
	text := strings.TrimSpace(userText)
	if text == "" {
		return nil
	}
	if isSourceLessAudioPlacementText(text, requestContext) {
		return nil
	}
	text = expandLocalIntentText(text)
	if isAddTrackText(text) {
		return []map[string]any{{
			"tool": "track.add",
			"args": map[string]any{},
		}}
	}
	if isTrackMuteText(text) {
		args := selectedTrackArgs(requestContext)
		args["mute"] = !isTrackUnmuteText(text)
		if trackIndex, ok := localUserTrackIndex(text); ok {
			args["user_track_index"] = trackIndex
			delete(args, "track_id")
		}
		return []map[string]any{{
			"tool": "track.mute",
			"args": args,
		}}
	}
	if isTrackSoloText(text) {
		args := selectedTrackArgs(requestContext)
		args["solo"] = !isTrackUnsoloText(text)
		if trackIndex, ok := localUserTrackIndex(text); ok {
			args["user_track_index"] = trackIndex
			delete(args, "track_id")
		}
		return []map[string]any{{
			"tool": "track.solo",
			"args": args,
		}}
	}
	if isMidiImportText(text, requestContext) {
		args := selectedMidiImportArgs(requestContext)
		if path := firstLocalMidiPath(text); path != "" {
			args["file_path"] = path
		}
		return []map[string]any{{
			"tool": "midi.import_file",
			"args": args,
		}}
	}
	if isAudioAttachmentImportText(text, requestContext) {
		return []map[string]any{{
			"tool": "clip.import_media_to_track",
			"args": selectedImportArgs(requestContext),
		}}
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
		if seconds, ok := firstContextSeconds(requestContext, "playhead_seconds", "current_playhead_seconds", "transport_position_seconds"); ok && mentionsPlayheadText(text) {
			args["new_start"] = seconds
			args["time_unit"] = "seconds"
		} else if seconds, ok := firstLocalSeconds(text); ok && !isRelativeMoveText(text) {
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
	case isClipMoveText(text) && (hasSeconds(text) || mentionsPlayheadText(text)):
		args := selectedClipArgs(requestContext, false)
		if seconds, ok := firstContextSeconds(requestContext, "playhead_seconds", "current_playhead_seconds", "transport_position_seconds"); ok && mentionsPlayheadText(text) {
			args["new_start"] = seconds
			args["time_unit"] = "seconds"
		} else if seconds, ok := firstLocalSeconds(text); ok && !isRelativeMoveText(text) {
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
		isCurrentRef := referencesCurrentClipText(text)
		isGenericRef := genericClipReferenceText(text)
		if name := localClipSelectName(text); name != "" && !isCurrentRef && !isGenericRef {
			args["clip_name"] = name
		} else if isCurrentRef {
			args = selectedClipArgs(requestContext, false)
		}
		if trackIndex, ok := localUserTrackIndex(text); ok {
			args["user_track_index"] = trackIndex
		} else if trackID := selectedClipTrackID(requestContext); trackID != "" {
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
	if strings.EqualFold(firstContextText(requestContext, "attachment_import_kind", "selected_attachment_kind"), "audio") {
		if path := strings.TrimSpace(firstContextText(requestContext, "attachment_import_file_path", "selected_attachment_file_path")); path != "" {
			args["file_path"] = path
		}
	}
	if isEmptyLocalValue(args["file_path"]) {
		for _, row := range contextRows(requestContext["attachments"]) {
			if strings.EqualFold(firstContextText(row, "kind"), "audio") {
				if path := strings.TrimSpace(firstContextText(row, "path", "file_path")); path != "" {
					args["file_path"] = path
					break
				}
			}
		}
	}
	if path := strings.TrimSpace(firstContextText(requestContext, "selected_library_file_path", "library_file_path")); path != "" {
		args["selected_library_file_path"] = path
	}
	args["start_time"] = 0.0
	args["media_type"] = "audio"
	args["mode"] = "non_destructive"
	return args
}

func selectedMidiImportArgs(requestContext map[string]any) map[string]any {
	args := map[string]any{}
	if trackID := strings.TrimSpace(firstContextText(requestContext, "selected_track_id", "focused_track_id", "track_id")); trackID != "" {
		args["track_id"] = trackID
	}
	if strings.EqualFold(firstContextText(requestContext, "attachment_import_kind", "selected_attachment_kind"), "midi") {
		if path := strings.TrimSpace(firstContextText(requestContext, "attachment_import_file_path", "selected_attachment_file_path")); path != "" {
			args["file_path"] = path
		}
	}
	if isEmptyLocalValue(args["file_path"]) && strings.EqualFold(firstContextText(requestContext, "selected_library_kind"), "midi") {
		if path := strings.TrimSpace(firstContextText(requestContext, "selected_library_file_path", "library_file_path")); path != "" {
			args["file_path"] = path
		}
	}
	if isEmptyLocalValue(args["file_path"]) {
		for _, row := range contextRows(requestContext["attachments"]) {
			if strings.EqualFold(firstContextText(row, "kind"), "midi") {
				if path := strings.TrimSpace(firstContextText(row, "path", "file_path")); path != "" {
					args["file_path"] = path
					break
				}
			}
		}
	}
	args["start_time_beats"] = 0.0
	args["mode"] = "merge_tracks"
	return args
}

func selectedTrackArgs(requestContext map[string]any) map[string]any {
	args := map[string]any{}
	if trackID := strings.TrimSpace(firstContextText(requestContext, "selected_track_id", "focused_track_id", "track_id")); trackID != "" {
		args["track_id"] = trackID
	}
	return args
}

func selectedClipArgs(requestContext map[string]any, allowMany bool) map[string]any {
	args := map[string]any{}
	ids := contextStringSlice(requestContext["selected_clip_ids"])
	if len(ids) == 0 {
		if id := strings.TrimSpace(firstContextText(requestContext, "piano_roll_focus_clip_id", "selected_clip_id", "primary_selected_clip_id", "clip_id")); id != "" {
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

func contextStringSlice(v any) []string {
	switch x := v.(type) {
	case []string:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if s := strings.TrimSpace(it); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, it := range x {
			s := strings.TrimSpace(fmt.Sprint(it))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func contextRows(v any) []map[string]any {
	switch rows := v.(type) {
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

func isEmptyLocalValue(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	return s == "" || s == "<nil>"
}

func selectedClipTrackID(requestContext map[string]any) string {
	return strings.TrimSpace(firstContextText(requestContext, "piano_roll_focus_track_id", "selected_clip_track_id", "focused_track_id", "selected_track_id", "track_id"))
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

func firstContextSeconds(requestContext map[string]any, keys ...string) (float64, bool) {
	raw := firstContextText(requestContext, keys...)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = -value
	}
	return value, true
}

func mentionsClip(text string) bool {
	return containsAnyFold(text, "clip", "audio", "sample", "midi", "片段", "素材", "鐗囦", "闊抽", "煶棰", "绱犳")
}

func isAudioImportText(text string) bool {
	if firstLocalAudioPath(text) != "" {
		return containsAnyFold(text, "import", "add", "load", "导入", "加入", "放到", "放进")
	}
	return containsAnyFold(text, "import", "add", "load", "导入", "加入", "放到", "放进") &&
		containsAnyFold(text, "audio", "sample", "library", "wav", "mp3", "flac", "loop", "音频", "素材", "资料库", "闊抽", "煶棰", "绱犳", "璧勬")
}

func isMidiImportText(text string, requestContext map[string]any) bool {
	if firstLocalMidiPath(text) != "" {
		return containsAnyFold(text, "import", "add", "load", "midi", "mid", "导入", "加入", "放到", "放进")
	}
	hasMidiContext := strings.EqualFold(firstContextText(requestContext, "attachment_import_kind", "selected_attachment_kind", "selected_library_kind"), "midi")
	if !hasMidiContext {
		for _, row := range contextRows(requestContext["attachments"]) {
			if strings.EqualFold(firstContextText(row, "kind"), "midi") {
				hasMidiContext = true
				break
			}
		}
	}
	if hasMidiContext && containsAnyFold(text, "import", "add", "load", "midi", "mid", "导入", "加入", "放到", "放进", "这个", "this") {
		return true
	}
	return containsAnyFold(text, "midi", ".mid", ".midi") &&
		containsAnyFold(text, "import", "add", "load", "导入", "加入", "放到", "放进")
}

func isAudioAttachmentImportText(text string, requestContext map[string]any) bool {
	if !hasAudioImportSource(requestContext) {
		return false
	}
	return containsAnyFold(text, "import", "add", "load", "this", "导入", "添加", "加载", "放到", "放进", "这个", "这段", "瀵煎叆", "鍔犲叆", "鏀惧埌", "鏀捐繘", "杩欎釜")
}

func isAddTrackText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if isLocalPluginOrInstrumentSubjectText(text) {
		return false
	}
	if !containsAnyFold(text, "track", "轨道", "音轨", "杞", "建閬") {
		return false
	}
	if containsAnyFold(text, "plugin", "vst", "eq", "reverb", "compressor", "delay", "clip", "sample", "import", "load", "鍔犺浇", "娣峰搷") {
		return false
	}
	if containsAnyFold(text, "audio", "闊抽", "煶棰") && !containsAnyFold(text, "audio track", "建閬", "轨道", "杞") {
		return false
	}
	return containsAnyFold(text, "add", "create", "new", "新建", "创建", "新增", "添加", "鏂板缓", "娣诲姞")
}

func isLocalPluginOrInstrumentSubjectText(text string) bool {
	return containsAnyFold(
		text,
		"plugin", "vst", "vsti", "instrument", "synth", "synthesizer", "sampler",
		"插件", "效果器", "合成器", "乐器", "音源", "MIDI乐器", "MIDI 乐器",
		"\u63d2\u4ef6", "\u6548\u679c\u5668", "\u5408\u6210\u5668", "\u4e50\u5668", "\u97f3\u6e90",
		"鎻掍欢", "鏁堟灉鍣", "鍚堟垚鍣", "涔愬櫒", "闊虫簮",
	)
}

func firstLocalAudioPath(text string) string {
	match := localAudioPathPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(match[1], " \t\r\n\"'`")
}

func firstLocalMidiPath(text string) string {
	match := localMidiPathPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(match[1], " \t\r\n\"'`")
}

func localImportSearchQuery(text string) string {
	if firstLocalAudioPath(text) != "" {
		return ""
	}
	if !containsAnyFold(text, "search", "find", "搜索", "查找", "找") {
		return ""
	}
	cleaned := text
	for _, phrase := range []string{
		"search", "find", "import", "add", "audio", "sample", "current track", "selected track", "track", "and", "to",
		"搜索", "查找", "找到", "导入", "放到", "放进", "资料库", "素材库", "当前轨道", "选中轨道", "音频", "素材",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
	}
	replacer := strings.NewReplacer("，", " ", "。", " ", ",", " ", ".", " ", "；", " ", ";", " ", "：", " ", ":", " ", "(", " ", ")", " ")
	cleaned = replacer.Replace(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

func isClipDeleteText(text string) bool {
	return containsAnyFold(text, "鍒犻櫎", "绉婚櫎", "鍒犳帀", "delete", "remove")
}

func isTrackMuteText(text string) bool {
	return containsAnyFold(text, "闈欓煶", "mute", "unmute")
}

func isTrackUnmuteText(text string) bool {
	return containsAnyFold(text, "鍙栨秷闈欓煶", "瑙ｉ櫎闈欓煶", "鍏抽棴闈欓煶", "鍙栨秷mute", "鍙栨秷 mute", "unmute")
}

func isTrackSoloText(text string) bool {
	return containsAnyFold(text, "solo", "鐙", "鍙栨秷鐙", "瑙ｉ櫎鐙")
}

func isTrackUnsoloText(text string) bool {
	return containsAnyFold(text, "鍙栨秷solo", "鍙栨秷 solo", "unsolo", "鍙栨秷鐙", "瑙ｉ櫎鐙", "鍏抽棴鐙")
}

func isClipSelectText(text string) bool {
	return containsAnyFold(text, "閫変腑", "閫夋嫨", "鑱氱劍", "select", "choose", "focus")
}

func localClipSelectName(text string) string {
	cleaned := strings.TrimSpace(text)
	for _, phrase := range []string{
		"閫変腑 clip", "閫夋嫨 clip", "鑱氱劍 clip", "閫変腑杩欎釜 clip", "閫夋嫨杩欎釜 clip", "鑱氱劍杩欎釜 clip",
		"閫変腑", "閫夋嫨", "鑱氱劍",
		"select clip", "choose clip", "focus clip", "select the clip", "choose the clip", "focus the clip",
		"select", "choose", "focus", "clip",
	} {
		cleaned = strings.ReplaceAll(cleaned, phrase, " ")
		cleaned = strings.ReplaceAll(cleaned, strings.Title(phrase), " ")
	}
	replacer := strings.NewReplacer("\"", " ", "'", " ", "`", " ", ":", " ", ",", " ", ".", " ", "(", " ", ")", " ")
	return strings.Join(strings.Fields(replacer.Replace(cleaned)), " ")
}

func localUserTrackIndex(text string) (int, bool) {
	match := localTrackIndexPattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return 0, false
	}
	for _, raw := range match[1:] {
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			return value, true
		}
	}
	return 0, false
}

func genericClipReferenceText(text string) bool {
	if _, ok := localUserTrackIndex(text); ok {
		return true
	}
	return containsAnyFold(text, "one clip", "any clip", "a clip", "current track", "selected track", "当前轨道", "选中轨道")
}

func referencesCurrentClipText(text string) bool {
	return containsAnyFold(text, "this clip", "current clip", "selected clip", "this audio", "current audio", "selected audio", "当前片段", "选中片段")
}

func isClipSplitText(text string) bool {
	return containsAnyFold(text, "鍒囧紑", "鍒囧垎", "鍒嗗壊", "鍓紑", "鍒囦竴鍒€", "split", "cut")
}

func isClipCloneText(text string) bool {
	return containsAnyFold(text, "duplicate", "clone", "copy", "复制", "克隆")
}

func isClipMoveText(text string) bool {
	return containsAnyFold(text, "move", "position", "start", "移动", "移到", "位置", "起点")
}

func isClipResizeText(text string) bool {
	if containsAnyFold(text, "move", "position", "start", "移动", "移到", "位置", "起点") {
		return false
	}
	return containsAnyFold(text, "resize", "length", "duration", "trim", "shorten", "长度", "时长", "裁剪", "缩短", "拉长")
}

func hasSeconds(text string) bool {
	_, ok := firstLocalSeconds(text)
	return ok
}

func firstLocalSeconds(text string) (float64, bool) {
	match := localSecondsPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		match = localSecondsCompatPattern.FindStringSubmatch(text)
	}
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
	return containsAnyFold(text, "earlier", "left", "backward", "later", "right", "forward", "前移", "后移", "提前", "推后")
}

func mentionsPlayheadText(text string) bool {
	return containsAnyFold(text, "playhead", "cursor", "current position", "here", "播放头", "当前位置", "这里")
}

func expandLocalIntentText(text string) string {
	aliases := make([]string, 0, 12)
	addAlias := func(alias string) {
		if alias != "" && !containsAnyFold(text, alias) {
			aliases = append(aliases, alias)
		}
	}
	isTrackCreation := containsAnyFold(text, "audio track", "音频轨", "音频轨道", "闊抽杞", "煶棰戣建") &&
		!containsAnyFold(text, "current track", "selected track", "当前轨道", "选中轨道", "褰撳墠杞ㄩ亾", "閫変腑杞ㄩ亾")
	isGenericClipRef := containsAnyFold(text, "一个clip", "一段clip", "一条clip", "涓€涓猚lip", "涓€娈礳lip", "涓€鏉lip")
	if _, ok := localUserTrackIndex(text); ok {
		isGenericClipRef = true
	}
	if containsAnyFold(text, "静音", "闈欓煶") {
		addAlias("mute track")
	}
	if containsAnyFold(text, "取消静音", "取消mute", "鍙栨秷闈欓煶", "鍙栨秷mute") {
		addAlias("unmute track")
	}
	if containsAnyFold(text, "独奏", "鐙", "solo") {
		addAlias("solo track")
	}
	if containsAnyFold(text, "取消solo", "取消独奏", "鍙栨秷solo", "鍙栨秷鐙") {
		addAlias("unsolo solo track")
	}
	if !isTrackCreation && containsAnyFold(text, "音频", "片段", "素材", "这个", "这段", "选中", "clip", "闊抽", "煶棰", "鐗囦", "鐨刢lip", "杩欠", "繖涓", "閫変腑") {
		addAlias("clip")
	}
	if !isTrackCreation && containsAnyFold(text, "音频", "闊抽", "煶棰") {
		addAlias("audio")
	}
	if containsAnyFold(text, "当前轨道", "选中轨道", "褰撳墠杞ㄩ亾", "閫変腑杞ㄩ亾") {
		addAlias("current track")
	}
	if containsAnyFold(text, "一个clip", "一段clip", "一条clip", "涓猚lip", "涓") {
		addAlias("one clip")
	}
	if !isGenericClipRef && containsAnyFold(text, "这个", "这段", "选中", "当前clip", "当前音频", "杩欎釜", "繖涓", "閫変腑") {
		addAlias("this clip selected clip")
	}
	if containsAnyFold(text, "删除", "删掉", "删", "鍒犻櫎", "垹鎺", "鍒", "鎺") {
		addAlias("delete clip")
	}
	if containsAnyFold(text, "切开", "切分", "分割", "鍒囧", "垏寮") {
		addAlias("split clip")
	}
	if containsAnyFold(text, "复制", "克隆", "澶嶅", "鍏嬮殕") {
		addAlias("copy clone clip")
	}
	if containsAnyFold(text, "移动", "移到", "绉诲姩", "绉诲埌", "鐨刢lip绉", "戠Щ") {
		addAlias("move clip")
	}
	if containsAnyFold(text, "起点", "璧风偣") {
		addAlias("move start clip")
	}
	if containsAnyFold(text, "裁", "裁到", "改成", "改为", "时长", "长度", "瑁佸", "瑁", "鏀规垚", "鏀逛负", "鏃堕暱", "") {
		addAlias("resize length clip")
	}
	if containsAnyFold(text, "选中", "选择", "聚焦", "閫変腑") {
		addAlias("select clip")
	}
	if containsAnyFold(text, "播放头", "这里", "当前位置", "鎾", "挱", "斁澶", "杩欓噷", "繖閲") {
		addAlias("playhead here")
	}
	if match := localSecondsCompatPattern.FindStringSubmatch(text); len(match) >= 2 {
		addAlias(match[1] + "s")
	}
	if len(aliases) == 0 {
		return text
	}
	return text + " " + strings.Join(aliases, " ")
}

func isSourceLessAudioPlacementText(text string, requestContext map[string]any) bool {
	if firstLocalAudioPath(text) != "" {
		return false
	}
	if hasAudioImportSource(requestContext) {
		return false
	}
	if !containsAnyFold(text, "audio", "音频", "闊抽", "煶棰") {
		return false
	}
	if !containsAnyFold(text, "add", "load", "添加", "加载", "放到", "放进", "娣诲姞", "鍔犺浇", "鏀惧埌", "鏀捐繘") {
		return false
	}
	return containsAnyFold(text, "current track", "selected track", "当前轨道", "选中轨道", "到当前", "到选中", "鍒板綋鍓", "褰撳墠杞ㄩ亾")
}

func hasAudioImportSource(requestContext map[string]any) bool {
	if path := strings.TrimSpace(firstContextText(requestContext, "selected_library_file_path", "library_file_path")); path != "" {
		return true
	}
	if strings.EqualFold(firstContextText(requestContext, "attachment_import_kind", "selected_attachment_kind"), "audio") {
		if path := strings.TrimSpace(firstContextText(requestContext, "attachment_import_file_path", "selected_attachment_file_path")); path != "" {
			return true
		}
	}
	for _, row := range contextRows(requestContext["attachments"]) {
		if strings.EqualFold(firstContextText(row, "kind"), "audio") {
			if path := strings.TrimSpace(firstContextText(row, "path", "file_path")); path != "" {
				return true
			}
		}
	}
	return false
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
