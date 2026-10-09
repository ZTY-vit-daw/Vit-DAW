package fastpath

// clip fade/gain 意图匹配器（L1-5-IMPL-C 自 agentloop message_loop.go 逐字平移）

import (
	"strings"
)

// ClipFadeGainReadRequest 逐字平移自 agentloop message_loop.go:1420-1437（L1-5-IMPL-C，行为零变化；原名 messageLoopClipFadeGainReadRequest）。函数体除引用改名外零改动。
func ClipFadeGainReadRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if !ClipFadeGainRequest(text) {
		return false
	}
	hasReadIntent := TextHasAny(text,
		"read", "show", "inspect", "status", "state", "get",
		"\u8bfb\u53d6", "\u67e5\u770b", "\u770b\u4e00\u4e0b", "\u72b6\u6001",
	)
	hasWriteIntent := TextHasAny(text,
		"set", "adjust", "change", "drag", "write", "apply",
		"\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9", "\u5199\u5165", "\u5e94\u7528",
	)
	if !hasReadIntent && TextHasAny(text, "db", "d b", "\u5206\u8d1d") {
		return false
	}
	return hasReadIntent || !hasWriteIntent
}

// ClipFadeGainRequest 逐字平移自 agentloop message_loop.go:6170-6200（L1-5-IMPL-C，行为零变化；原名 messageLoopClipFadeGainRequest）。函数体除引用改名外零改动。
func ClipFadeGainRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if GainStagingExplicitRequest(text) {
		return false
	}
	hasClipTarget := TextHasAny(text,
		"clip", "clips", "selected clip", "current clip", "this clip", "audio clip",
		"\u7247\u6bb5", "\u97f3\u9891\u7247\u6bb5", "\u5f53\u524d\u7247\u6bb5", "\u9009\u4e2d\u7247\u6bb5",
		"\u5f53\u524d\u9009\u4e2d clip", "\u5f53\u524d clip", "\u9009\u4e2d clip", "\u8fd9\u4e2a clip",
	)
	if !hasClipTarget {
		return false
	}
	hasFadeOrGain := TextHasAny(text,
		"fade", "fade in", "fade out", "clip gain", "gain",
		"\u6de1\u5165", "\u6de1\u51fa", "\u6de1\u5316", "\u589e\u76ca",
	)
	if !hasFadeOrGain {
		return false
	}
	if TextHasAny(text, "fade", "gain", "clip gain", "fade/gain") && TextHasAny(text, "clip", "audio clip") {
		return true
	}
	return TextHasAny(text,
		"read", "show", "inspect", "status", "state", "get", "set", "adjust", "change", "drag",
		"\u8bfb\u53d6", "\u67e5\u770b", "\u770b\u4e00\u4e0b", "\u72b6\u6001", "\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9",
	)
}
