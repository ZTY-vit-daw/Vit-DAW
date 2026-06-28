package agentloop

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var messageLoopDBAmountPattern = regexp.MustCompile(`(?i)([+\-]?\d+(?:\.\d+)?)\s*(?:dB|db)`)

func messageLoopExtractSingleGainDeltaFromDBAmount(reply string) (float64, string, bool) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return 0, "", false
	}
	matches := messageLoopDBAmountPattern.FindAllStringSubmatchIndex(reply, -1)
	type gainDeltaCandidate struct {
		match []int
		delta float64
	}
	valid := []gainDeltaCandidate{}
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[1] < len(reply) && messageLoopASCIIAlpha(reply[match[1]]) {
			continue
		}
		delta, ok := messageLoopGainDeltaFromDBAmountMatch(reply, match)
		if ok {
			valid = append(valid, gainDeltaCandidate{match: match, delta: delta})
		}
	}
	if len(valid) == 0 {
		return 0, "", false
	}
	if len(valid) > 1 {
		first := valid[0].delta
		for _, candidate := range valid[1:] {
			if math.Abs(candidate.delta-first) > 0.000001 {
				return 0, "", false
			}
		}
	}
	match := valid[0].match
	return valid[0].delta, strings.TrimSpace(reply[match[0]:match[1]]), true
}

func messageLoopGainDeltaFromDBAmountMatch(reply string, match []int) (float64, bool) {
	raw := strings.TrimSpace(reply[match[2]:match[3]])
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	sign := 0.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
	} else if strings.HasPrefix(raw, "+") {
		sign = 1
	}
	contextStart := match[0] - 96
	if contextStart < 0 {
		contextStart = 0
	}
	metricStart := match[0] - 36
	if metricStart < 0 {
		metricStart = 0
	}
	afterEnd := match[1] + 24
	if afterEnd > len(reply) {
		afterEnd = len(reply)
	}
	metricBefore := strings.ToLower(strings.TrimSpace(reply[metricStart:match[0]]))
	window := strings.ToLower(strings.TrimSpace(reply[contextStart:afterEnd]))
	if messageLoopTextHasAny(metricBefore, "rms", "lufs", "\u5cf0\u503c", "\u4f59\u91cf", "headroom", "crest", "peak", "risk", "\u98ce\u9669", "\u7ea7\u522b") {
		return 0, false
	}
	if messageLoopTextHasAny(metricBefore,
		"\u5de6\u53f3\u5e73\u8861", "\u5e73\u8861", "\u58f0\u50cf", "\u58f0\u76f8", "\u7acb\u4f53\u58f0", "\u76f8\u5173",
		"stereo", "balance", "correlation", "left level", "right level",
	) {
		return 0, false
	}
	if sign == 0 {
		switch {
		case messageLoopTextHasAny(window, "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u51cf\u5c11", "\u51cf", "lower", "lowering", "reduce", "reducing", "decrease", "decreasing", "down", "cut", "cutting"):
			sign = -1
		case messageLoopTextHasAny(window, "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u589e\u52a0", "\u52a0", "raise", "raising", "boost", "boosting", "increase", "increasing", "up", "higher"):
			sign = 1
		}
	}
	if sign == 0 {
		return 0, false
	}
	return sign * math.Abs(value), true
}

func messageLoopASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
