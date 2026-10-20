package chat

import (
	"context"
	"strings"
	"testing"

	agentruntime "vit-daw-agent/internal/runtime"
)

// BUDGETSTOP-CHAINEND-DELIVERY-1（2026-10-20 晚窗真栈，PARKING-DELIVERY-1 取证）：
// recordGoalResult 的预算耗尽 settle 把 durable 与全部活记录置 completed，但不撤
// 3256 行 arm 写入的 goalContinuations legacy 条目——goalHasLiveContinuationOwner 把
// 这个 completed 形态的 arm 算成活（"unknown arm shapes keep counting as live"），
// 调度侧完成桥的 chainEnded 求值为 false，链尾投递门在第一道卫兵处静默返回：
// [f6.gate] chainEnded=false status=completed respGoal=waiting_continue
// runtimeGoal=waiting_continue，四分支（含 D1-STALL-1 欠账回执）零进入，turn 事件面
// 同谓词同挡。本文件从 recordGoalResult 全链复刻该形态，钉住 settle 撤 arm 的行为。
//
// 与 d1_stall_settle_test.go 的分工：那边的钉④只在 goal 状态面断言
// waiting_continue（其时泄漏已在内存里发生但无人读）；这里把被污染的谓词真实求值
// 接进链尾投递门——正是当晚三层验证面（109 线/停摆烟测/单测显式传 chainEnded=true）
// 全部绕过的那个求值点。

// 钉①（RED→GREEN 主钉）：预算耗尽 settle 必须撤除 legacy arm——settle 后该 goal
// 不得再有活续跑持有者，且以调度侧完成桥的同一表达式求值 chainEnded 时投递门必须
// 打开：D1-STALL-1 欠账回执送达（「继续」入口在正文里），而不是静默。
func TestBudgetStopSettleRetiresLegacyArmAndChainEndDeliversOwedReceipt(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	owing := d1StallStoredLoop(t, s)
	// 真栈形态：续跑计数越过预算（该 fixture 的 Experiment 非空，terminal-turn
	// guard 不适用，floor 不会把预算抬到 used——used>budget 直达 settle 段）。
	owing.ContinuationUsed = owing.ContinuationBudget + 1
	s.storeFreeStateLoop(owing)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)

	if err := s.recordGoalResult(d1StallConversation, d1StallContinuationResult(loop, "slice-d1-stall-bs", "turn-d1-stall-bs")); err != nil {
		t.Fatal(err)
	}

	// settle 语义核对：循环 blocked + 预算耗尽停因（与钉④同一查询面）。
	stored := d1StallStoredLoop(t, s)
	if stored.Status != "blocked" || stored.LatestDecision == nil ||
		stored.LatestDecision.StopReason != freeStateContinuationBudgetExhaustedReason {
		t.Fatalf("setup: the budget stop must have settled the loop: status=%q decision=%+v", stored.Status, stored.LatestDecision)
	}

	// 主断言①：arm 随 settle 撤除——goal 无活续跑持有者（泄漏形态下这里为 true，
	// completed 的 legacy 条目被算活）。
	if s.goalHasLiveContinuationOwner(loop.GoalID) {
		t.Fatal("the budget-stop settle must retire the legacy goalContinuations arm: the goal has no live continuation owner")
	}

	// 主断言②：以调度侧完成桥（executeDurableContinuation）的同一表达式求值
	// chainEnded，投递门必须送达欠账回执。recordGoalResult 刚创建的 durable 就是
	// 该 goal 的末片形态（completed），按真实形状取用它。
	s.mu.Lock()
	var final DurableContinuation
	found := false
	for _, item := range s.durableContinuations {
		if item.GoalID == loop.GoalID {
			final = cloneDurableContinuation(item)
			found = true
			break
		}
	}
	s.mu.Unlock()
	if !found {
		t.Fatal("setup: the budget-stopped goal must still own its durable record")
	}
	if final.Status != ContinuationCompleted {
		t.Fatalf("setup: the budget-stopped durable must be terminal, got %s", final.Status)
	}
	s.settleAndDeliverContinuationChainEnd(context.Background(), final, ChatResponse{
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "d1_post_action_evaluation_required", LimitType: "max_turns",
		Reply: "我还在继续处理这个任务，完成后再向你汇报。",
	}, !s.goalHasLiveContinuationOwner(final.GoalID))

	delivered := d1StallChainResultEvents(s)
	if len(delivered) != 1 {
		t.Fatalf("the real chainEnded evaluation must open the delivery gate for the owed round, got %d events: %+v", len(delivered), delivered)
	}
	body := delivered[0].Body
	if !strings.Contains(body, "继续") || !strings.Contains(body, "还没做完") {
		t.Fatalf("the receipt must carry the unfinished statement and the continue entry: %q", body)
	}
	if strings.Contains(body, "我还在继续处理这个任务") {
		t.Fatalf("the receipt must not be the scheduler pass-through line: %q", body)
	}
	if marked, _ := delivered[0].Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("the receipt must carry the scheduler_chain marker: %+v", delivered[0].Payload)
	}
	if delivered[0].Status != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("the receipt must annotate the resumable waiting_continue status, got %q", delivered[0].Status)
	}
	// 回执送达后 goal 仍须留在可续跑形态（入口不因投递而关闭）。
	if after := s.harness.RuntimeStatus(loop.GoalID); !shouldResumeGoalFromStatus(after.Status) {
		t.Fatalf("the delivered owed round must keep the goal resumable, got %s", after.Status)
	}
}

// 钉②（领取侧核对义务①的可执行形态）：预算停撤 arm 后，裸「继续」的续跑入口仍在
// ——goal 的 waiting_continue 承认面（shouldResumeGoalFromStatus）不读该 map；同
// 会话的 goal 归属把这句话带回同一 goal 链（与 d1_stall_settle_test 钉⑥同根，在撤
// arm 后重钉）。
func TestBudgetStopResumeEntrySurvivesLegacyArmRetirement(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	owing := d1StallStoredLoop(t, s)
	owing.ContinuationUsed = owing.ContinuationBudget + 1
	s.storeFreeStateLoop(owing)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)
	if err := s.recordGoalResult(d1StallConversation, d1StallContinuationResult(loop, "slice-d1-stall-bs2", "turn-d1-stall-bs2")); err != nil {
		t.Fatal(err)
	}
	if s.goalHasLiveContinuationOwner(loop.GoalID) {
		t.Fatal("setup: the settle must have retired the arm")
	}
	s.mu.Lock()
	s.conversationGoals[d1StallConversation] = loop.GoalID
	s.mu.Unlock()

	resumed := s.beginChatGoal(d1StallConversation, "继续", map[string]any{"goal_id": loop.GoalID, "run_id": loop.RunID})
	if resumed.GoalID != loop.GoalID {
		t.Fatalf("a bare continue must re-enter the same goal chain after the arm retirement, got %s want %s", resumed.GoalID, loop.GoalID)
	}
	if resumed.RunID != loop.RunID {
		t.Fatalf("a bare continue must keep the run identity, got %s want %s", resumed.RunID, loop.RunID)
	}
}
