package agentloop

import "testing"

// AGENT-F3：runner 侧 limit 话术是实验/调试语境的密封面——stop_reason、
// limit_type 与 reply 文本一起被 D1/D2 冒烟与七域回归断言依赖。对话式中
// 性化只发生在 chat 消费点（chatResponseFromAgentLoopResult），本包语义
// 逐字不动。
func TestLimitReplyTextSealedForExperimentContexts(t *testing.T) {
	for limitType, want := range map[string]string{
		LimitTypeTurns:     "本轮思考步数已到上限；任务会从已保存的检查点自动继续。",
		LimitTypeToolCalls: "本轮工具调用次数已到上限；任务会从已保存的检查点自动继续。",
		LimitTypeTimeout:   "本轮运行时间已到上限；任务会从已保存的检查点自动继续。",
		"unknown":          "本轮运行预算已到上限；任务会从已保存的检查点自动继续。",
	} {
		if got := limitReply(limitType); got != want {
			t.Fatalf("limit reply text for %s is sealed: got %q want %q", limitType, got, want)
		}
	}
}
