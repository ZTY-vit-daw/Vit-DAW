package fastpath

// 框选范围拆分（clip range split）确定性匹配/回执 helper（L1-5-IMPL-C 逐字平移）

import (
	"strconv"
	"strings"

	"vit-daw-agent/internal/planner"
)

// ExecutedClipRangeSplitCuts 逐字平移自 agentloop message_loop.go:1323-1354（L1-5-IMPL-C，行为零变化；原名 messageLoopExecutedClipRangeSplitCuts）。函数体除引用改名外零改动。
func ExecutedClipRangeSplitCuts(trace []planner.TraceEvent) (attempted, succeeded map[string]bool) {
	attempted = map[string]bool{}
	succeeded = map[string]bool{}
	pendingID := ""
	pendingKey := ""
	for _, event := range trace {
		switch event.Kind {
		case "tool_call":
			if event.ToolCall != nil && strings.EqualFold(strings.TrimSpace(event.ToolCall.Tool), "clip.split") {
				pendingID = strings.TrimSpace(event.ToolCall.ID)
				pendingKey = ClipRangeSplitCutKey(event.ToolCall.Args)
			} else {
				pendingID, pendingKey = "", ""
			}
		case "tool_result":
			if pendingKey == "" || event.ToolResult == nil {
				pendingID, pendingKey = "", ""
				continue
			}
			if pendingID == "" || strings.TrimSpace(event.ToolResult.ToolCallID) == pendingID {
				attempted[pendingKey] = true
				status := strings.ToLower(strings.TrimSpace(event.ToolResult.Status))
				if strings.TrimSpace(event.ToolResult.Error) == "" &&
					status != "error" && status != "kernel_error" && status != "failed" && status != "needs_confirmation" {
					succeeded[pendingKey] = true
				}
				pendingID, pendingKey = "", ""
			}
		}
	}
	return attempted, succeeded
}

// ClipRangeSplitCutKey 逐字平移自 agentloop message_loop.go:1356-1372（L1-5-IMPL-C，行为零变化；原名 clipRangeSplitCutKey）。函数体除引用改名外零改动。
func ClipRangeSplitCutKey(args map[string]any) string {
	if args == nil {
		return ""
	}
	clipID := strings.TrimSpace(Text(args["clip_id"]))
	value, ok := args["split_time"].(float64)
	if !ok {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(Text(args["split_time"])), 64)
		if err == nil {
			value, ok = parsed, true
		}
	}
	if clipID == "" || !ok {
		return ""
	}
	return clipID + "@" + strconv.FormatFloat(value, 'f', -1, 64)
}

// ClipRangeSplitCompletionReply 逐字平移自 agentloop message_loop.go:1374-1388（L1-5-IMPL-C，行为零变化；原名 messageLoopClipRangeSplitCompletionReply）。函数体除引用改名外零改动。
func ClipRangeSplitCompletionReply(plan []planner.ToolCall, succeeded map[string]bool) string {
	times := make([]string, 0, len(plan))
	missing := make([]string, 0, len(plan))
	for _, call := range plan {
		key := ClipRangeSplitCutKey(call.Args)
		times = append(times, strings.TrimPrefix(key, strings.TrimSpace(Text(call.Args["clip_id"]))+"@"))
		if !succeeded[key] {
			missing = append(missing, times[len(times)-1])
		}
	}
	if len(missing) == 0 {
		return "框选范围拆分完成：已按 " + strings.Join(times, " → ") + " 秒的顺序切分，所选范围已独立成片。"
	}
	return "框选范围拆分未完成：" + strings.Join(missing, "、") + " 秒处的切分未能执行，片段状态可能已变化，请检查后重试。"
}
