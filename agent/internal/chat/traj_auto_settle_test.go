package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
)

// TRAJ-AUTO-SETTLE-1（卡面依据：设计稿 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md
// §7-B 澄清段 + 用户裁定「成熟 agent 长任务只有交互卡，没有用户面继续」）。
//
// 本文件钉住的行为面：
//
//	①欠收尾评估（纯机器欠账）不再需要用户打字续跑——应用边界把「结算尾段」
//	  （materiality/目标评估 -> 判定边界）的片预算地板开出来，调度器自动续跑
//	  结算片；用户面不再出现「说一句继续」类回执。
//	②等 A/B 判定（有未决判定卡）一律不自动续跑——那是等用户的耳朵，判定 POST
//	  是唯一能收口这一回合的入口。
//
// 勘察结论（本卡落法的依据，代码锚点）：
//   - 应用边界的片预算地板 reserveD1PostApplySlices（free_state_reasoning_loop.go）
//     此前只在 RequiresPostActionObservation=true（应用后观察尚未入账）时授予；
//     真栈里应用后观察多由 bookD1PostActionObservation 在响应链内确定性入账
//     （free_state_d1_runtime.go:681-697），于是尾段拿不到地板，先被片预算饿死。
//   - 饿死的形态就是本卡要消灭的 ①类：recordGoalResult 的预算停止把该 goal
//     名下 durable 全部 terminal 化（goalrunner_chat.go:2744+），链终局投递门
//     落到 D1-STALL-1 的欠账分支（continuation_scheduler.go:1254），用户看到
//     「你回一句继续，我就接着把这轮的评估做完」——把机器工作转嫁给用户。
//   - 而「用户说一句继续」正是 shouldResumeGoalFromStatus（goalrunner_chat.go:3220）
//     承认的 waiting_* 三态入口：同一台机器，用户在场就自动不起来、用户不在场
//     就得打字。本卡把它改回机器一侧：地板生效 → 记录留 pending → 调度器 claim
//     执行，用户面零回执。

const trajAutoSettleConversation = d1StallConversation

// 钉①（RED→GREEN 主钉，授予面）：应用后观察已在响应链内入账
// （RequiresPostActionObservation=false）时，回合仍欠受治结果，结算尾段必须拿到
// 自己的片预算地板——这是「自动续跑结算片」能够发生的调度前提。
func TestOwedSettlementTailGrantsPostApplyFloorWithoutPendingJudgmentCard(t *testing.T) {
	_, loop, _ := d1StallServerLoopForTest(t)
	if loop.RequiresPostActionObservation {
		t.Fatal("setup: the fixture must be the booked-observation applied shape")
	}
	if !freeStateLoopOwesExperimentOutcome(loop) {
		t.Fatal("setup: the applied round must still owe its governed outcome")
	}
	if freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: no human judgment has been requested yet — this is the owed-evaluation form, not the park")
	}
	if !freeStateLoopOwesAutoSettlement(loop) {
		t.Fatal("an owed round with no pending judgment card is machine work the scheduler may resume")
	}
	used := loop.ContinuationUsed
	reserveD1PostApplySlices(&loop)
	if loop.ContinuationBudget != used+freeStateD1PostApplySliceNeed {
		t.Fatalf("the settlement tail got no phase-scoped floor: budget=%d want %d (used=%d)",
			loop.ContinuationBudget, used+freeStateD1PostApplySliceNeed, used)
	}
	if !loop.PostApplyBudgetReserved {
		t.Fatal("the settlement-tail floor must be marked as granted for this applied round")
	}
}

// 钉②（RED→GREEN 主钉，行为面）：地板生效后，应用边界之后的那一拍结算片必须
// 留在可调度形态（pending）——即调度器 claim 后自动执行，而不是被预算停止
// terminal 化、由链终局投递门向用户索要一句「继续」。
func TestOwedSettlementFloorAutoResumesSettlementSliceWithoutContinueReceipt(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	applied := d1StallStoredLoop(t, s)
	// 真栈形态：应用落盘时片预算已经走满（尾段是第一件被饿死的事）。
	applied.ContinuationUsed = applied.ContinuationBudget
	// 生产路径：应用边界的片预算地板（projectD1Execution 在 applied receipt
	// 之后调用同一函数）。
	reserveD1PostApplySlices(&applied)
	if !applied.PostApplyBudgetReserved {
		t.Fatal("setup: the applied boundary must grant the settlement-tail floor")
	}
	s.storeFreeStateLoop(applied)

	if err := s.recordGoalResult(trajAutoSettleConversation, d1StallContinuationResult(loop, "slice-trajs-settle", "turn-trajs-settle")); err != nil {
		t.Fatal(err)
	}

	// ① 结算片留在链上（可 claim）：这就是自动续跑本身。
	s.mu.Lock()
	armed := 0
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending {
			armed++
		}
	}
	s.mu.Unlock()
	if armed != 1 {
		t.Fatalf("the owed settlement slice must stay armed for the scheduler, got %d pending records — the ①form receipt (「说一句继续」) is the shape this card retires", armed)
	}
	// ② 循环仍在驱动形态，没有被写成预算停止的 blocked。
	stored := d1StallStoredLoop(t, s)
	if stored.Status == "blocked" || (stored.LatestDecision != nil && stored.LatestDecision.StopReason == freeStateContinuationBudgetExhaustedReason) {
		t.Fatalf("the owed settlement must not stop at the continuation budget: status=%q decision=%+v", stored.Status, stored.LatestDecision)
	}
	// ③ 链上还有活持有者 -> 链终局投递门根本不会被走到，欠账回执（正文要求用户
	//    回一句「继续」）不再为纯机器欠账合成。
	if !s.goalHasLiveContinuationOwner(loop.GoalID) {
		t.Fatal("the auto-resumed settlement slice must keep the chain alive")
	}
	if events := d1StallChainResultEvents(s); len(events) != 0 {
		t.Fatalf("an auto-resuming owed chain must deliver no chain terminal (no continue receipt): %+v", events)
	}
	// ④ 零回退：goal 仍在 D1-STALL-1 保留的可续跑形态（用户手动续跑入口不变，
	//    只是不再是机器欠账的必要条件）。
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the owing goal must keep the resumable waiting_continue form, got %s", after.Status)
	}
}

// 钉③（守卫钉，有未决判定卡不自动）：停驻在人工判定边界的回合同样「欠受治结果」
// （欠的是用户的判定），但它是等用户的耳朵——不得自动续跑，也不得授予自动续跑
// 的预算地板。
func TestOwedSettlementFloorRefusedWhileJudgmentCardPending(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	parked := d1StallStoredLoop(t, s)
	round, err := parked.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	round.UserJudgmentRequested = true
	for index := range parked.Experiment.Rounds {
		if parked.Experiment.Rounds[index].ID == round.ID {
			parked.Experiment.Rounds[index] = round
		}
	}
	s.storeFreeStateLoop(parked)
	parked = d1StallStoredLoop(t, s)
	if !freeStateJudgmentBoundary(parked) {
		t.Fatal("setup: the round must sit at the durable human-judgment boundary")
	}
	if !freeStateLoopOwesExperimentOutcome(parked) {
		t.Fatal("setup: a parked judgment is still an owed governed outcome")
	}
	if freeStateLoopOwesAutoSettlement(parked) {
		t.Fatal("a round waiting for the user's ears must never be auto-resumed")
	}
	budget := parked.ContinuationBudget
	reserveD1PostApplySlices(&parked)
	if parked.ContinuationBudget != budget || parked.PostApplyBudgetReserved {
		t.Fatalf("a judgment-parked round was granted the automatic settlement floor: budget=%d->%d reserved=%v",
			budget, parked.ContinuationBudget, parked.PostApplyBudgetReserved)
	}

	// 同一形态的生产落点零回退：预算走满仍然是预算停止（loop blocked +
	// 可查询停因），goal 由 D1-STALL-1 的欠账保留留在 waiting_continue，
	// 用户面由 PARK-1 判定摘要承担（既有钉覆盖）。
	exhausted := d1StallStoredLoop(t, s)
	exhausted.ContinuationUsed = exhausted.ContinuationBudget + 1
	s.storeFreeStateLoop(exhausted)
	s.harness.SetGoalStatus(parked.GoalID, agentruntime.StatusWaitingContinue, nil)
	if err := s.recordGoalResult(trajAutoSettleConversation, d1StallContinuationResult(loop, "slice-trajs-park", "turn-trajs-park")); err != nil {
		t.Fatal(err)
	}
	stored := d1StallStoredLoop(t, s)
	if stored.Status != "blocked" || stored.LatestDecision == nil ||
		stored.LatestDecision.StopReason != freeStateContinuationBudgetExhaustedReason {
		t.Fatalf("the budget stop must stay queryable for a judgment-parked round: status=%q decision=%+v", stored.Status, stored.LatestDecision)
	}
}

// 钉④（上限行为，自动续跑不无界）：结算尾段的地板每应用回合只授予一次，且上限
// 就是 freeStateD1PostApplySliceNeed 片——越过上限后既有的预算停止（可查询停因
// + 链终局欠账回执）照旧回归，自动续跑不可能自我续命烧预算。
func TestOwedSettlementFloorIsGrantedOncePerAppliedRoundAndBounded(t *testing.T) {
	_, loop, _ := d1StallServerLoopForTest(t)
	applied := loop
	applied.ContinuationUsed = applied.ContinuationBudget
	reserveD1PostApplySlices(&applied)
	granted := applied.ContinuationBudget
	if granted != applied.ContinuationUsed+freeStateD1PostApplySliceNeed {
		t.Fatalf("the settlement tail floor = %d, want used+%d", granted, freeStateD1PostApplySliceNeed)
	}
	reserveD1PostApplySlices(&applied)
	if applied.ContinuationBudget != granted {
		t.Fatalf("the settlement floor was granted twice in one applied round: %d", applied.ContinuationBudget)
	}
	// 上限的可执行含义：地板之上恰好还能排 freeStateD1PostApplySliceNeed 片，
	// 第 N+1 片回到既有预算边界（本包 recordGoalResult 的预算停止）。
	scheduled := 0
	for applied.ContinuationUsed+1 <= applied.ContinuationBudget {
		applied.ContinuationUsed++
		scheduled++
		if scheduled > freeStateD1PostApplySliceNeed+1 {
			t.Fatal("the settlement floor admitted more slices than its bound")
		}
	}
	if scheduled != freeStateD1PostApplySliceNeed {
		t.Fatalf("the settlement floor admitted %d slices, want %d", scheduled, freeStateD1PostApplySliceNeed)
	}
	if !freeStateContinuationBudgetExhausted(applied) {
		t.Fatal("the settlement floor must leave the loop at the ordinary bounded budget edge")
	}
}

// 钉⑤（生产落点）：applied receipt 落库后的 projectD1Execution 必须授予结算尾段
// 地板——修法住在应用边界，而不是测试夹具里。观察已在响应链内入账的真栈形态
// （verifiedPostAction=true）是 ①类的高发形态。
func TestProjectD1ExecutionGrantsOwedSettlementFloorOnApply(t *testing.T) {
	s := New(nil, nil, nil)
	s.auditionKernel = &fakeAuditionKernel{}
	s.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))

	loop := d1LoopForTest(t, "7")
	beforePath := writeD1WAVForTest(t, "before_revision_7_owed_settle.wav")
	loop.D1State = map[string]any{
		"before_render": map[string]any{
			"phase": "before", "status": "ready", "file_path": beforePath, "project_revision": "7",
			"sha256": strings.Repeat("d", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "before", "7"),
			"preview_revision": "sha256:before-owed-settle",
		},
	}
	s.storeFreeStateLoop(loop)
	afterPath := writeD1WAVForTest(t, "after_revision_8_owed_settle.wav")
	loop.D1State["after_render"] = map[string]any{
		"phase": "after", "status": "ready", "file_path": afterPath, "project_revision": "8",
		"sha256": strings.Repeat("e", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "after", "8"),
		"preview_revision": "sha256:after-owed-settle",
	}
	s.storeFreeStateLoop(loop)

	// 真栈形态：应用落盘时片预算已经走满。
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 6
	receipt := orchestration.ActionReceipt{ActionID: "d1-action-owed-settle", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-trajs", "idempotency_key": "key-trajs",
		"actual_readback_db": -1.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session-owed-settle", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-owed-settle", IdempotencyKey: "key-trajs", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-trajs-after", ObservationRevision: "8", EvidenceRefs: []string{"ccb-trajs"}},
	}}

	resp := s.projectD1Execution(loop, session, nil)

	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("the applied loop was not stored")
	}
	if stored.RequiresPostActionObservation {
		t.Fatal("setup: this fixture books the post-action observation inside the respond chain")
	}
	if !freeStateLoopOwesAutoSettlement(stored) {
		t.Fatalf("setup: the applied round must owe machine settlement work: %+v", stored.Experiment.Rounds)
	}
	if !stored.PostApplyBudgetReserved || stored.ContinuationBudget != loop.ContinuationUsed+freeStateD1PostApplySliceNeed {
		t.Fatalf("the applied boundary did not grant the owed-settlement floor: reserved=%v budget=%d used=%d",
			stored.PostApplyBudgetReserved, stored.ContinuationBudget, stored.ContinuationUsed)
	}
	// 应用回执契约零变化（fail-open 纪律同款）。
	if resp.StopReason != "d1_post_action_evaluation_required" || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("the applied response contract changed: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}
}
