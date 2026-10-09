package fastpath

// strip silence / A4 片段清理意图匹配器（L1-5-IMPL-C 逐字平移）

import (
	"strings"
)

// StripSilenceSuggestRequest 逐字平移自 agentloop message_loop.go:6202-6226（L1-5-IMPL-C，行为零变化；原名 messageLoopStripSilenceSuggestRequest）。函数体除引用改名外零改动。
func StripSilenceSuggestRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasStripIntent := TextHasAny(text,
		"strip silence", "strip_silence", "silence cleanup", "remove silence", "trim silence",
		"清理静音", "片段清理", "清理空白", "去静音", "去掉静音", "去掉空白", "删除静音", "过滤静音", "噪声底",
	) || A4ClipCleanupRequest(text)
	if !hasStripIntent {
		return false
	}
	if A4ClipCleanupRequest(text) {
		return true
	}
	hasApplyOnlyIntent := TextHasAny(text,
		"apply", "execute", "confirm", "do it", "go ahead",
		"应用", "执行", "确认", "按这个", "就这样", "继续",
	)
	hasAnalysisIntent := TextHasAny(text,
		"suggest", "recommend", "analyze", "analyse", "estimate", "parameter", "threshold", "preview",
		"建议", "推荐", "分析", "估算", "参数", "阈值", "预览", "检查",
	)
	return !hasApplyOnlyIntent || hasAnalysisIntent
}

// A4ClipCleanupRequest 逐字平移自 agentloop message_loop.go:6262-6271（L1-5-IMPL-C，行为零变化；原名 messageLoopA4ClipCleanupRequest）。函数体除引用改名外零改动。
func A4ClipCleanupRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || !TextHasAny(text, "a4", "a 4", "epm") {
		return false
	}
	return TextHasAny(text,
		"clip cleanup", "clip trim", "clip trimming", "trim clips", "cleanup clips",
		"片段裁剪", "片段清理", "裁剪片段", "清理片段", "裁剪", "清理",
	)
}

// A4ClipCleanupWholeProjectRequest 逐字平移自 agentloop message_loop.go:6273-6282（L1-5-IMPL-C，行为零变化；原名 messageLoopA4ClipCleanupWholeProjectRequest）。函数体除引用改名外零改动。
func A4ClipCleanupWholeProjectRequest(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if !A4ClipCleanupRequest(text) {
		return false
	}
	return !TextHasAny(text,
		"selected", "current", "this clip", "this track", "selected clip", "selected track", "current clip", "current track", "selected range", "range",
		"选中", "当前片段", "当前轨道", "选中片段", "选中轨道", "这个片段", "这条轨", "范围", "选区",
	)
}
