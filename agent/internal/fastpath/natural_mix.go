package fastpath

// 自然混音/音频观察意图匹配器（L1-5-IMPL-C 逐字平移）

import (
	"strings"
)

// NaturalMixRequest 逐字平移自 agentloop message_loop.go:6148-6168（L1-5-IMPL-C，行为零变化；原名 messageLoopNaturalMixRequest）。函数体除引用改名外零改动。
func NaturalMixRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if ClipFadeGainRequest(text) {
		return false
	}
	if TextHasAny(text, "\u5de6", "\u53f3", "\u5c45\u4e2d", "\u56de\u4e2d", "\u4e2d\u95f4", "left", "right", "center", "centre") &&
		TextHasAny(text, "\u58f0\u50cf", "\u58f0\u76f8", "\u8f68\u9053", "\u5409\u4ed6", "\u8d1d\u65af", "\u9f13", "\u4e3b\u5531", "\u4eba\u58f0", "pan", "panning", "track", "guitar", "bass", "drum", "vocal", "voice") {
		return true
	}
	return TextHasAny(text,
		"\u6df7\u97f3", "\u6df7\u4e00\u4e0b", "\u5e2e\u6211\u6df7", "\u7f29\u6df7", "\u58f0\u97f3\u5904\u7406", "\u8c03\u4e00\u4e0b", "\u5904\u7406\u4e00\u4e0b",
		"\u4e3b\u5531", "\u4eba\u58f0", "vocal", "lead vocal",
		"\u9760\u524d", "\u5f80\u524d", "\u63d0\u5347\u54cd\u5ea6", "\u54cd\u5ea6", "\u592a\u54cd", "\u592a\u5927", "\u592a\u5c0f", "\u538b\u4f4e", "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u97f3\u91cf", "\u7535\u5e73", "\u589e\u76ca", "\u66f4\u4eae", "\u660e\u4eae", "\u6d51\u6d4a", "\u523a\u8033",
		"\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u7a7a\u95f4\u611f", "\u52a0\u4e00\u70b9\u7a7a\u95f4", "\u58f0\u50cf", "\u58f0\u76f8", "\u58f0\u573a", "\u52a8\u6001", "\u538b\u7f29",
		"mix", "mixing", "loudness", "louder", "too loud", "too quiet", "volume", "level", "gain", "lower", "reduce", "decrease", "raise", "boost", "increase", "forward", "mud", "muddy", "harsh", "bright", "space", "reverb", "pan", "panning", "stereo field", "dynamic",
		"low end", "low-end", "bass", "kick",
	)
}

// AudioObservationRequest 逐字平移自 agentloop message_loop.go:6284-6301（L1-5-IMPL-C，行为零变化；原名 messageLoopAudioObservationRequest）。函数体除引用改名外零改动。
func AudioObservationRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if NaturalMixRequest(text) {
		return true
	}
	return TextHasAny(text,
		"\u97f3\u9891\u89c2\u5bdf", "\u6df7\u97f3\u89c2\u5bdf", "\u58f0\u5b66\u89c2\u5bdf", "\u58f0\u5b66\u5206\u6790", "\u58f0\u5b66\u6570\u636e",
		"\u89c2\u5bdf\u5206\u6790", "\u5177\u4f53\u5206\u6790", "\u7ee7\u7eed\u89c2\u5bdf", "\u5206\u6790\u4e00\u4e0b",
		"\u9891\u8c31", "\u9891\u8c31\u5206\u5e03", "\u9891\u6bb5\u80fd\u91cf", "\u4f4e\u9891", "\u4e2d\u9891", "\u9ad8\u9891",
		"\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u52a8\u6001\u8303\u56f4", "\u6ce2\u5f62", "\u5305\u7edc",
		"audio observation", "mix observation", "acoustic observation", "acoustic analysis", "acoustic data",
		"observation analysis", "analyze audio", "specific analysis", "spectrum", "spectral", "frequency distribution",
		"band energy", "loudness", "peak", "rms", "lufs", "dynamic range", "waveform", "envelope",
	)
}
