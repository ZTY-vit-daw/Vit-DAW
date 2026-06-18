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
	valid := [][]int{}
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[1] < len(reply) && messageLoopASCIIAlpha(reply[match[1]]) {
			continue
		}
		valid = append(valid, match)
	}
	if len(valid) != 1 {
		return 0, "", false
	}
	match := valid[0]
	raw := strings.TrimSpace(reply[match[2]:match[3]])
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value == 0 {
		return 0, "", false
	}
	sign := 0.0
	if strings.HasPrefix(raw, "-") {
		sign = -1
	} else if strings.HasPrefix(raw, "+") {
		sign = 1
	}
	start := match[0] - 96
	if start < 0 {
		start = 0
	}
	context := strings.ToLower(strings.TrimSpace(reply[start:match[0]]))
	if sign == 0 {
		switch {
		case messageLoopTextHasAny(context, "\u964d\u4f4e", "\u4e0b\u8c03", "\u8c03\u4f4e", "\u51cf\u5c11", "\u51cf", "lower", "lowering", "reduce", "reducing", "decrease", "decreasing", "down", "cut", "cutting"):
			sign = -1
		case messageLoopTextHasAny(context, "\u63d0\u9ad8", "\u63d0\u5347", "\u4e0a\u8c03", "\u8c03\u9ad8", "\u589e\u52a0", "\u52a0", "raise", "raising", "boost", "boosting", "increase", "increasing", "up", "higher"):
			sign = 1
		}
	}
	if sign == 0 {
		return 0, "", false
	}
	return sign * math.Abs(value), strings.TrimSpace(reply[match[0]:match[1]]), true
}

func messageLoopASCIIAlpha(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}
