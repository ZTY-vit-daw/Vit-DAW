package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
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

// d1ApplyCompletedResult builds the apply-turn completion envelope the real
// stack produced twice on 2026-09-14 (TRAJ-AUTO-SETTLE-1 §7.1, sessions
// webui_mu228fc5 / webui_mu23pryg): status=completed stop=done
// continuation=false — the apply turn's last slice does not arm a runner
// continuation, so without a synthetic checkpoint the SETTLE-1 floor has no
// record to spend its budget on.
func d1ApplyCompletedResult(loop freeStateReasoningLoop, slice, turn string) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
		SliceID: slice, TurnID: turn, OriginalIntent: loop.OriginalIntent,
		Status: agentruntime.StatusCompleted, StopReason: agentloop.StopReasonDone,
	}
}

// 钉⑥（RED→GREEN 主钉，造片面）：真栈 ①类的缺口在检查点，不在预算——应用回合
// 末片 completed/done/无 continuation 收口时，若回合仍欠自动结算（无未决判定卡），
// 应用边界必须合成一枚可 claim 的「结算片」检查点，SETTLE-1 授的地板才有片可跑。
func TestOwedApplyTurnCompletionArmsSettleCheckpoint(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	applied := d1StallStoredLoop(t, s)
	// 真栈形态：应用落盘时预算已走满，地板由应用边界授予（钉⑤）。
	applied.ContinuationUsed = applied.ContinuationBudget
	reserveD1PostApplySlices(&applied)
	s.storeFreeStateLoop(applied)

	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-apply", "turn-trajs-apply")); err != nil {
		t.Fatal(err)
	}

	// ① 恰好一枚 pending 结算检查点——这就是地板有片可跑本身。
	s.mu.Lock()
	pending := make([]DurableContinuation, 0, 1)
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending {
			pending = append(pending, item)
		}
	}
	s.mu.Unlock()
	if len(pending) != 1 {
		t.Fatalf("the owed apply turn must arm exactly one claimable settle checkpoint, got %d pending records", len(pending))
	}
	settle := pending[0]
	if settle.GoalID != loop.GoalID || settle.ConversationID != trajAutoSettleConversation {
		t.Fatalf("the settle checkpoint must belong to the owing goal: goal=%s conversation=%s", settle.GoalID, settle.ConversationID)
	}
	// ② 检查点必须能驱动结算尾段：承载原始意图与循环快照的内部续跑上下文。
	if strings.TrimSpace(settle.Continuation.UserText) == "" {
		t.Fatal("the settle checkpoint must carry driving text, not an empty shell")
	}
	if !contextBool(settle.Continuation.Context, "free_state_internal_resume") {
		t.Fatal("the settle checkpoint must carry the internal-resume marker so it resumes the loop instead of starting a new one")
	}
	if _, loopOK := freeStateLoopFromAny(settle.Continuation.Context["free_state_reasoning_loop"]); !loopOK {
		t.Fatal("the settle checkpoint must carry the free-state loop snapshot")
	}
	// ③ 闩与预算：一次性造片被记录（durable 携带结算标记，终态不重开），
	//    检查点消耗地板一片。
	s.mu.Lock()
	latched := s.goalHasSettleCheckpointLocked(loop.GoalID)
	s.mu.Unlock()
	if !latched {
		t.Fatal("the settle checkpoint must be latched once for the goal (marker durable)")
	}
	stored := d1StallStoredLoop(t, s)
	if stored.ContinuationUsed != applied.ContinuationUsed+1 {
		t.Fatalf("the settle checkpoint must consume one floor slice: used=%d want %d", stored.ContinuationUsed, applied.ContinuationUsed+1)
	}
	if stored.Status == "blocked" || (stored.LatestDecision != nil && stored.LatestDecision.StopReason == freeStateContinuationBudgetExhaustedReason) {
		t.Fatalf("the floor has room; the loop must not stop at the budget: status=%q decision=%+v", stored.Status, stored.LatestDecision)
	}
	// ④ 零回退：D1-AUDITION-GAP-1 的可续跑形态保留，链上有活持有者，欠账回执
	//    （「你回一句继续」）不再被合成。
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the owing goal must keep the resumable waiting_continue form, got %s", after.Status)
	}
	if !s.goalHasLiveContinuationOwner(loop.GoalID) {
		t.Fatal("the settle checkpoint must keep the chain alive for the scheduler")
	}
	if events := d1StallChainResultEvents(s); len(events) != 0 {
		t.Fatalf("an armed settle checkpoint must deliver no chain terminal (no continue receipt): %+v", events)
	}
}

// 钉⑦（守卫钉，判定驻留不造片）：停驻在人工判定边界的回合同样以 completed 收口
// 时，不得合成结算检查点——那是等用户的耳朵，判定 POST 是唯一收口入口。
func TestJudgmentParkApplyCompletionDoesNotArmSettleCheckpoint(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
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
	parked.ContinuationUsed = parked.ContinuationBudget
	reserveD1PostApplySlices(&parked)
	s.storeFreeStateLoop(parked)

	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-park-arm", "turn-trajs-park-arm")); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	pending := 0
	for _, item := range s.durableContinuations {
		if continuationRunnableStatus(item.Status) {
			pending++
		}
	}
	s.mu.Unlock()
	if pending != 0 {
		t.Fatalf("a judgment-parked round must never receive an automatic settle checkpoint, got %d runnable records", pending)
	}
	s.mu.Lock()
	latched := s.goalHasSettleCheckpointLocked(loop.GoalID)
	s.mu.Unlock()
	if latched {
		t.Fatal("the settle latch must stay unset for the judgment park")
	}
	// PARK-1 形态零回退：goal 仍在可续跑 waiting_continue，判定卡承担用户面。
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the parked goal must keep the waiting_continue form, got %s", after.Status)
	}
}

// 钉⑧（守卫钉，收口后不循环）：结算片检查点每循环只合成一次。结算片自身以
// completed/无 continuation 收口而回合仍欠（如评估无结论）时，不得再造第二枚——
// 必须停在可查询形态（goal waiting_continue + 链终局欠账回执由投递门承担）。
func TestSettleCheckpointArmsOnceAndDoesNotSelfPerpetuate(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	applied := d1StallStoredLoop(t, s)
	applied.ContinuationUsed = applied.ContinuationBudget
	reserveD1PostApplySlices(&applied)
	s.storeFreeStateLoop(applied)

	// 第一拍：应用回合 completed 收口 → 合成结算片。
	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-a", "turn-trajs-a")); err != nil {
		t.Fatal(err)
	}
	// 第二拍：结算片自身 completed/无 continuation 收口、回合仍欠 → 闩必须拒绝再造。
	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-b", "turn-trajs-b")); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	runnable := 0
	for _, item := range s.durableContinuations {
		if continuationRunnableStatus(item.Status) {
			runnable++
		}
	}
	s.mu.Unlock()
	if runnable != 1 {
		t.Fatalf("the settle checkpoint must not self-perpetuate: %d runnable records after the settle slice closed still owing", runnable)
	}

	// 第三拍：调度器消费掉结算片（终态化并退出 arm 槽）后，同型 completed 欠账
	// 收口再现 → 闩仍然拒绝（可查询停因，不是循环造片）。
	s.mu.Lock()
	now := time.Now().UTC()
	for id, item := range s.durableContinuations {
		if item.GoalID != loop.GoalID {
			continue
		}
		item.Status = ContinuationCompleted
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = now
		s.durableContinuations[id] = cloneDurableContinuation(item)
	}
	delete(s.goalContinuations, loop.GoalID)
	s.mu.Unlock()
	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-c", "turn-trajs-c")); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	pending := 0
	for _, item := range s.durableContinuations {
		if continuationRunnableStatus(item.Status) {
			pending++
		}
	}
	s.mu.Unlock()
	if pending != 0 {
		t.Fatalf("the once-per-loop latch must refuse a second settle checkpoint after the first was consumed: %d runnable records", pending)
	}
	// 停因可查询：goal 保持 waiting_continue（D1-STALL-1 / D1-AUDITION-GAP-1 形态）。
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the honestly stopped chain must keep the queryable waiting_continue form, got %s", after.Status)
	}
}

// 钉⑨（守卫钉，预算边缘不硬造）：地板已授且已走满（PostApplyBudgetReserved=true、
// used>=budget）时，completed 欠账收口不得合成检查点——越过预算边缘就是既有预算
// 停止/链终局欠账回执的辖区，造片不能自我续命。
func TestSettleCheckpointRefusedAtHonestBudgetEdge(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	spent := d1StallStoredLoop(t, s)
	spent.ContinuationUsed = spent.ContinuationBudget
	spent.PostApplyBudgetReserved = true
	s.storeFreeStateLoop(spent)

	if err := s.recordGoalResult(trajAutoSettleConversation, d1ApplyCompletedResult(loop, "slice-trajs-edge", "turn-trajs-edge")); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	runnable := 0
	for _, item := range s.durableContinuations {
		if continuationRunnableStatus(item.Status) {
			runnable++
		}
	}
	s.mu.Unlock()
	if runnable != 0 {
		t.Fatalf("no settle checkpoint may be armed past the budget edge, got %d runnable records", runnable)
	}
	s.mu.Lock()
	latched := s.goalHasSettleCheckpointLocked(loop.GoalID)
	s.mu.Unlock()
	if latched {
		t.Fatal("a refused arm must not burn the once-per-goal latch")
	}
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the budget-edge refusal must keep the resumable waiting_continue form, got %s", after.Status)
	}
}

// 钉⑩（RED→GREEN 主钉，应用边界面）：2026-09-14/15 真栈两次的交错形态——
// runner 在回合 LLM 段已把 goal 判 completed（此刻回合不欠账），干预回执随后在
// 同一回合的 respond 段（projectD1Execution）落账，此后再无 goal result 边界。
// 结算欠账在此判真，应用边界必须在此造片并把 goal 修正回可续跑的 waiting_continue
//（completed 不可 resume，调度器结算片需要合法的 resume 入口）。
func TestAppliedBoundaryArmsSettleCheckpointOverCompletedGoal(t *testing.T) {
	s := New(nil, nil, nil)
	s.auditionKernel = &fakeAuditionKernel{}
	s.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))

	loop := d1LoopForTest(t, "7")
	beforePath := writeD1WAVForTest(t, "before_revision_7_applied_boundary.wav")
	loop.D1State = map[string]any{
		"before_render": map[string]any{
			"phase": "before", "status": "ready", "file_path": beforePath, "project_revision": "7",
			"sha256": strings.Repeat("d", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "before", "7"),
			"preview_revision": "sha256:before-applied-boundary",
		},
	}
	s.storeFreeStateLoop(loop)
	afterPath := writeD1WAVForTest(t, "after_revision_8_applied_boundary.wav")
	loop.D1State["after_render"] = map[string]any{
		"phase": "after", "status": "ready", "file_path": afterPath, "project_revision": "8",
		"sha256": strings.Repeat("e", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "after", "8"),
		"preview_revision": "sha256:after-applied-boundary",
	}
	s.storeFreeStateLoop(loop)

	// 真栈形态：应用落盘时预算已走满；runner 已把 goal 判 completed。
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 6
	s.storeFreeStateLoop(loop)
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusCompleted, nil)

	receipt := orchestration.ActionReceipt{ActionID: "d1-action-applied-boundary", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-trajs-ab", "idempotency_key": "key-trajs-ab",
		"actual_readback_db": -1.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session-applied-boundary", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-applied-boundary", IdempotencyKey: "key-trajs-ab", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-trajs-ab", ObservationRevision: "8", EvidenceRefs: []string{"ccb-trajs-ab"}},
	}}

	resp := s.projectD1Execution(loop, session, nil)

	// ① 应用边界造片：地板有片可跑。
	s.mu.Lock()
	pending := make([]DurableContinuation, 0, 1)
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending {
			pending = append(pending, item)
		}
	}
	latched := s.goalHasSettleCheckpointLocked(loop.GoalID)
	s.mu.Unlock()
	if len(pending) != 1 || !latched {
		t.Fatalf("the applied boundary must arm the settle checkpoint over a completed goal: pending=%d latched=%t", len(pending), latched)
	}
	if !contextBool(pending[0].Continuation.Context, settleCheckpointMarker) || !contextBool(pending[0].Continuation.Context, "free_state_internal_resume") {
		t.Fatal("the settle checkpoint must carry the marker and the internal-resume context")
	}
	// ② completed 不可 resume——goal 被修正回 waiting_continue，调度器结算片
	//    有合法 resume 入口（D1-AUDITION-GAP-1 修正的应用边界版）。
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the completed goal must be restored to the resumable waiting_continue form at the applied boundary, got %s", after.Status)
	}
	// ③ 应用回执契约零变化（钉⑤同款 fail-open 纪律）。
	if resp.StopReason != "d1_post_action_evaluation_required" || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("the applied response contract changed: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}

	// ④ restore 契约：调度器每个周期从盘上 reload 后走
	//    normalizeRestoredDurableContinuation——runnable 记录任何身份字段为空
	//    都会被隔离进不可答的 recovery park（2026-09-15 真栈 R4：结算片 arm 后
	//    连续 49 拍 no-claimable-record stranded=1，claim 永远取不到）。合成
	//    检查点必须带完整身份过 restore。
	s.mu.Lock()
	var settleDurable DurableContinuation
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending {
			settleDurable = item
		}
	}
	s.mu.Unlock()
	if settleDurable.ContinuationID == "" {
		t.Fatal("setup: the settle durable disappeared")
	}
	state := projectAgentRuntimeState{
		ProjectPath: "D:/settle/restore/contract", ProjectUUID: settleDurable.ProjectUUID,
		ConversationGoals: map[string]string{settleDurable.ConversationID: settleDurable.GoalID},
	}
	restoredID, restored := normalizeRestoredDurableContinuation(settleDurable.ContinuationID, settleDurable, state, time.Now().UTC())
	if restoredID != settleDurable.ContinuationID || restored.Status != ContinuationPending {
		t.Fatalf("the settle checkpoint must survive the restore normalization as pending: id=%s status=%s pending_interaction=%v",
			restoredID, restored.Status, restored.PendingInteraction)
	}
	if restored.CurrentSliceID == "" || restored.CurrentTurnID == "" || restored.TaskID == "" || restored.RunID == "" || restored.OriginalIntent == "" {
		t.Fatalf("the settle checkpoint must carry complete task/run/slice identity: slice=%q turn=%q task=%q run=%q intent=%q",
			restored.CurrentSliceID, restored.CurrentTurnID, restored.TaskID, restored.RunID, restored.OriginalIntent)
	}
}

// 钉⑪（守卫钉，应用边界不越权）：goal 仍在运行（turn 活着）时应用落账，应用
// 边界不得造片——回合边界自己的 arm 机制会扛尾段，完成后由 completed 边界接管。
func TestAppliedBoundaryDoesNotArmWhileGoalStillRunning(t *testing.T) {
	s := New(nil, nil, nil)
	s.auditionKernel = &fakeAuditionKernel{}
	s.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))

	loop := d1LoopForTest(t, "7")
	beforePath := writeD1WAVForTest(t, "before_revision_7_ab_alive.wav")
	loop.D1State = map[string]any{
		"before_render": map[string]any{
			"phase": "before", "status": "ready", "file_path": beforePath, "project_revision": "7",
			"sha256": strings.Repeat("d", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "before", "7"),
			"preview_revision": "sha256:before-ab-alive",
		},
	}
	s.storeFreeStateLoop(loop)
	afterPath := writeD1WAVForTest(t, "after_revision_8_ab_alive.wav")
	loop.D1State["after_render"] = map[string]any{
		"phase": "after", "status": "ready", "file_path": afterPath, "project_revision": "8",
		"sha256": strings.Repeat("e", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "after", "8"),
		"preview_revision": "sha256:after-ab-alive",
	}
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 6
	s.storeFreeStateLoop(loop)
	// goal 仍是 running：turn 还活着，回合边界的 arm 机制负责尾段。
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusRunning, nil)

	receipt := orchestration.ActionReceipt{ActionID: "d1-action-ab-alive", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-ab-alive", "idempotency_key": "key-ab-alive",
		"actual_readback_db": -1.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session-ab-alive", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-ab-alive", IdempotencyKey: "key-ab-alive", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-ab-alive", ObservationRevision: "8", EvidenceRefs: []string{"ccb-ab-alive"}},
	}}

	_ = s.projectD1Execution(loop, session, nil)

	s.mu.Lock()
	runnable := 0
	for _, item := range s.durableContinuations {
		if continuationRunnableStatus(item.Status) {
			runnable++
		}
	}
	latched := s.goalHasSettleCheckpointLocked(loop.GoalID)
	s.mu.Unlock()
	if runnable != 0 || latched {
		t.Fatalf("a live turn must keep carrying the tail itself: runnable=%d latched=%t", runnable, latched)
	}
	if after := s.harness.RuntimeStatus(goal.GoalID); after.Status != agentruntime.StatusRunning {
		t.Fatalf("the applied boundary must not touch a live goal's status, got %s", after.Status)
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
