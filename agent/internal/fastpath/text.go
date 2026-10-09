package fastpath

// 通用文本规范化/分词 helper（L1-5-IMPL-C 自 agentloop message_loop.go 逐字平移）

import (
	"fmt"
	"strings"
)

// Text 逐字平移自 agentloop message_loop.go:7266-7272（L1-5-IMPL-C，行为零变化；原名 messageLoopText）。函数体除引用改名外零改动。
func Text(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

// TextHasAny 逐字平移自 agentloop message_loop.go:10568-10576（L1-5-IMPL-C，行为零变化；原名 messageLoopTextHasAny）。函数体除引用改名外零改动。
func TextHasAny(text string, tokens ...string) bool {
	for _, token := range tokens {
		token = strings.ToLower(strings.TrimSpace(token))
		if token != "" && strings.Contains(text, token) {
			return true
		}
	}
	return false
}
