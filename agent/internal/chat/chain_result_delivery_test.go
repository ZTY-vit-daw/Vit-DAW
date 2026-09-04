package chat

import (
	"testing"

	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
)

func chainEventTypes(s *Server, conversationID string) map[string]int {
	events, _ := s.agentEventsSince(conversationID, 0, 64)
	out := map[string]int{}
	for _, event := range events {
		out[event.Type]++
	}
	return out
}

// AGENT-F6-A2：切片边界（goal=waiting_continue）且仍有活续跑链时，回合并未
// 终局——不得发 trajectory 终态事件，turn:{runID} 节点保持 running（UI 常转
// spinner）。2026-09-04 手测铁证：终态带 waiting_for_user + limit_reached，
// 背景链还在跑，回执提前收成"等待你的判断"+时钟。
func TestChatTurnTrajectorySkipsTerminalWhileChainLive(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-f6", "run-f6", "检查当前工程有什么问题")
	s.durableContinuations["cont_f6_next"] = DurableContinuation{
		ContinuationID: "cont_f6_next", GoalID: "goal-f6", RunID: "run-f6",
		ConversationID: "conversation-f6", OriginalIntent: "检查当前工程有什么问题", Status: ContinuationPending,
	}

	s.emitChatTurnTrajectoryStarted("conversation-f6", "goal-f6", "run-f6")
	resp := ChatResponse{GoalID: "goal-f6", RunID: "run-f6", GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "limit_reached"}
	s.emitChatTurnTrajectory("conversation-f6", "turn.completed", resp, "goal-f6", "run-f6")

	types := chainEventTypes(s, "conversation-f6")
	if types["trajectory.turn.completed"] != 0 {
		t.Fatalf("slice boundary with a live chain must not emit a terminal trajectory event: %+v", types)
	}
	if types["trajectory.turn.started"] == 0 {
		t.Fatalf("turn node must exist to stay live: %+v", types)
	}

	// 链终局：pending 片已完成，goal 已落 completed——终态事件到来且是
	// completed（不再误标 waiting_for_user）。
	s.mu.Lock()
	s.durableContinuations["cont_f6_next"] = DurableContinuation{
		ContinuationID: "cont_f6_next", GoalID: "goal-f6", RunID: "run-f6",
		ConversationID: "conversation-f6", OriginalIntent: "检查当前工程有什么问题", Status: ContinuationCompleted,
	}
	s.mu.Unlock()
	s.harness.SetGoalStatus("goal-f6", agentruntime.StatusCompleted, nil)
	s.emitChatTurnTrajectory("conversation-f6", "turn.completed", resp, "goal-f6", "run-f6")

	events, _ := s.agentEventsSince("conversation-f6", 0, 64)
	var terminalStatus string
	for _, event := range events {
		if event.Type == "trajectory.turn.completed" {
			terminalStatus = event.Status
		}
	}
	if terminalStatus != "completed" {
		t.Fatalf("chain end must close the turn node as completed, got %q", terminalStatus)
	}
}

// AGENT-F6-A1：调度终片的结果投递——turn.completed 带 scheduler_chain 标记、
// body 装最终回复，并闭掉 turn:{runID} 轨迹节点。此前 executeDurable-
// Continuation 只查 error/交互挂起，最终 reply 被静默丢弃（手测：多轮执行
// 后没有任何结果）。
func TestSchedulerChainResultEventDeliversFinalReply(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-f6b", "run-f6b", "检查当前工程有什么问题")
	s.emitChatTurnTrajectoryStarted("conversation-f6b", "goal-f6b", "run-f6b")

	resp := ChatResponse{
		ConversationID: "conversation-f6b", GoalID: "goal-f6b", RunID: "run-f6b",
		GoalStatus: string(agentruntime.StatusCompleted), Reply: "本轮观察结束：未发现值得处理的候选项。",
	}
	s.emitSchedulerChainResultEvent("conversation-f6b", resp, nil)

	events, _ := s.agentEventsSince("conversation-f6b", 0, 64)
	var delivered bool
	for _, event := range events {
		if event.Type != "turn.completed" {
			continue
		}
		if event.ItemID != "chain_result" {
			t.Fatalf("scheduler chain result must carry a stable item id: %+v", event)
		}
		payload := event.Payload
		if marked, _ := payload["scheduler_chain"].(bool); !marked {
			t.Fatalf("scheduler chain result must be marked: %+v", payload)
		}
		if event.Body != resp.Reply {
			t.Fatalf("final reply must ride the event body: %q", event.Body)
		}
		delivered = true
	}
	if !delivered {
		t.Fatal("scheduler chain end did not emit turn.completed")
	}
	if types := chainEventTypes(s, "conversation-f6b"); types["trajectory.turn.completed"] == 0 {
		t.Fatalf("scheduler chain end must close the turn trajectory node: %+v", types)
	}
}

// 失败链同样要送达：turn.failed 带错误信息与 scheduler_chain 标记。
func TestSchedulerChainResultEventDeliversFailure(t *testing.T) {
	s := testContinuationServer()
	s.harness = harness.NewWithSender(nil, nil, nil)
	s.harness.EnsureGoal("goal-f6c", "run-f6c", "检查当前工程有什么问题")
	s.emitSchedulerChainResultEvent("conversation-f6c", ChatResponse{
		ConversationID: "conversation-f6c", GoalID: "goal-f6c", RunID: "run-f6c",
	}, errBoom)

	events, _ := s.agentEventsSince("conversation-f6c", 0, 64)
	var sawFailed bool
	for _, event := range events {
		if event.Type == "turn.failed" {
			sawFailed = true
			if event.Body != errBoom.Error() {
				t.Fatalf("failure text must ride the event body: %q", event.Body)
			}
		}
	}
	if !sawFailed {
		t.Fatal("failed chain end did not emit turn.failed")
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "chain exploded in a test" }
