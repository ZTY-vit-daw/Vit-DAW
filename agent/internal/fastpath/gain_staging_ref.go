package fastpath

// B1/B1.2 gain staging 意图引用判定（L1-5-IMPL-C 自 agentloop static_mix_gain_staging_context.go 逐字副本平移；原件原地保留，agentloop 继续消费原件）

import (
	"strings"
)

// GainStagingExplicitRequest 逐字副本自 agentloop static_mix_gain_staging_context.go:1746-1768（L1-5-IMPL-C；原件原地保留不动，本副本仅供本包平移函数依赖，命名空间独立）。函数体除引用改名外零改动。
func GainStagingExplicitRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasB1 := TextHasAny(text, "b1", "b 1", "static_mix.gain_staging", "gain staging", "gain-staging", "gainstage")
	// In B2 requests, B1 is commonly a readiness reference ("based on B1"),
	// not a request to run the gain-staging capability again.
	if hasB1 && StaticBalanceIntentReference(text) {
		return false
	}
	if hasB1 {
		return true
	}
	hasB12 := TextHasAny(text, "b1.2", "b 1.2", "b1-2", "b 1-2", "b1_2")
	if hasB12 || GainStagingStrictReferenceIntent(text) {
		return true
	}
	return hasB1 && TextHasAny(text,
		"gain", "level", "volume", "loudness", "peak", "rms", "lufs", "headroom",
		"\u589e\u76ca", "\u7535\u5e73", "\u97f3\u91cf", "\u54cd\u5ea6", "\u5cf0\u503c", "\u5747\u65b9\u6839", "\u4f59\u91cf",
	)
}

// StaticBalanceIntentReference 逐字副本自 agentloop static_mix_gain_staging_context.go:1770-1775（L1-5-IMPL-C；原件原地保留不动，本副本仅供本包平移函数依赖，命名空间独立）。函数体除引用改名外零改动。
func StaticBalanceIntentReference(text string) bool {
	return TextHasAny(strings.ToLower(strings.TrimSpace(text)),
		"b2", "b 2", "static_mix.static_balance", "static balance",
		"\u9759\u6001\u5e73\u8861", "\u9759\u6001\u97f3\u91cf", "\u63a8\u5b50\u5e73\u8861",
	)
}

// GainStagingStrictReferenceIntent 逐字副本自 agentloop static_mix_gain_staging_context.go:1777-1798（L1-5-IMPL-C；原件原地保留不动，本副本仅供本包平移函数依赖，命名空间独立）。函数体除引用改名外零改动。
func GainStagingStrictReferenceIntent(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasReference := TextHasAny(text,
		"strict reference", "reference level", "reference-level", "same reference", "same range", "full-project reference", "source level calibration",
		"\u4e25\u683c\u53c2\u8003", "\u4e25\u683c\u6821\u51c6", "\u53c2\u8003\u7535\u5e73", "\u53c2\u8003\u6307\u6807", "\u540c\u4e00\u53c2\u8003", "\u540c\u4e00\u6307\u6807", "\u7edf\u4e00\u7535\u5e73", "\u76f8\u540c\u8303\u56f4", "\u8d34\u8fd1",
	)
	if !hasReference {
		return false
	}
	return TextHasAny(text,
		"calibrate", "calibration", "source level", "clip gain", "clip.gain", "full project", "whole project", "entire project", "all tracks", "pending action", "prepare",
		"\u6821\u51c6", "\u6e90\u7535\u5e73", "\u9759\u6001\u6e90\u7535\u5e73", "\u5168\u5de5\u7a0b", "\u6574\u4e2a\u5de5\u7a0b", "\u6240\u6709\u8f68\u9053", "\u5168\u90e8\u8f68\u9053", "\u5f85\u786e\u8ba4\u52a8\u4f5c", "\u51c6\u5907",
	)
}
