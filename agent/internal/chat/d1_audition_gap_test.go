package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D1-AUDITION-GAP-1（2026-09-13 22:58 真栈，会话 webui_mtzxugg8，goal_c7ecb4fb，
// 用户手测命中「第一次的要求执行，执行完成后没有返回AB试听」）。
//
// 事件流逐条（artifacts/_live_probe/ev_zxugg8.json）：
//
//	22:58:31 trajectory.turn.started（调度续跑片「实验回合开始」）
//	22:58:31 mix_tick.pending（「混音单步（完全访问直接应用）」）
//	22:58:36 trajectory.intervention.applied
//	22:58:36.9 goal_c7ecb4fb.status = completed（goal_runtime 直读，
//	          此时 task 仍 active / needs_experiment）
//	22:58:38 turn.completed（waiting_continue，五要素报告落图）
//	——之后零续跑片、零 audition.* 事件。
//
// 双因（本文件钉住）：
//
//  1. 时序洞：agentloop 的 Runner.complete 对模型自报完成无条件终态化，没有任何
//     D1 欠账检查（D1-STALL-1 的守卫只住在本包的两个 completed 写点上，管不到
//     agentloop 侧的写点）。应用回合内模型自报完成 → goal 先 completed → 结算
//     欠账（d1_post_action_evaluation_required）无人消费 → 判定边界永不达 →
//     A/B 卡零出现。
//  2. 面缺口：D1 回路的 A/B 挂卡（prepareFreeStateAudition）只接在判定边界
//     （recordFreeStateExperimentDecision 的 human_audition_ready 分支）；B12-2 的
//     bracket 接在 mix-tick 原生 apply 面，且被 D1 分派分支提前 return 绕过。
//     用户产品裁定「我只负责 AB 试听」= 每次应用到工程的干预都必须以 A/B 卡
//     收口，无论链条后续发生什么。

// 钉①（修 1，时序洞）：应用回合自报 completed（无续跑 continuation）而实验
// 回合仍欠受治结果时，recordGoalResult 必须把 goal 纠回可续跑的 waiting_continue，
// 不得让 runner 的无条件终态化把欠账 stranding 成「goal completed 而 task 仍
// needs_experiment」的真栈形态。
func TestCompletedTurnWithOwedRoundKeepsGoalResumable(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	if !s.freeStateLoopOwesExperimentOutcomeFor(loop.ConversationID, goal.GoalID) {
		t.Fatal("setup: the applied round must owe its governed outcome")
	}
	// 真栈形态：runner.complete 在回合中途已把 goal 写成 completed（22:58:36.9）。
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusCompleted, nil)
	// 应用回合的结果信封：模型自报完成、无续跑 continuation。
	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
		SliceID: "slice-apply", TurnID: "turn-apply", OriginalIntent: loop.OriginalIntent,
		Status: agentruntime.StatusCompleted, StopReason: agentloop.StopReasonDone,
	}
	if err := s.recordGoalResult(loop.ConversationID, res); err != nil {
		t.Fatalf("recordGoalResult failed: %v", err)
	}
	after := s.harness.RuntimeStatus(goal.GoalID)
	if after.Status == agentruntime.StatusCompleted {
		t.Fatalf("a completed turn over a round that still owes its governed outcome must not leave the goal terminal: status=%s task=%s — the 22:58 stranding",
			after.Status, after.Task.SemanticState.State)
	}
	if after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the owed goal must be restored to the resumable waiting_continue form, got %s", after.Status)
	}
	if !shouldResumeGoalFromStatus(after.Status) {
		t.Fatalf("the goal status %s must be one a follow-up turn resumes from", after.Status)
	}
}

// 钉①-b（修 1 的 durable 面）：同一形态下，该 goal 名下仍处活态的 durable
// 检查点不得被 terminal 化——欠账回合的恢复入口（说一句「继续」）依赖链上
// 还有可取的检查点。
func TestCompletedTurnWithOwedRoundKeepsDurableCheckpointsLive(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	if !s.freeStateLoopOwesExperimentOutcomeFor(loop.ConversationID, goal.GoalID) {
		t.Fatal("setup: the applied round must owe its governed outcome")
	}
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusCompleted, nil)
	s.mu.Lock()
	s.durableContinuations["cont_d1_owed_live"] = DurableContinuation{
		ContinuationID: "cont_d1_owed_live", GoalID: goal.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
		ConversationID: loop.ConversationID, Status: ContinuationPending,
	}
	s.mu.Unlock()

	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
		SliceID: "slice-apply", TurnID: "turn-apply", OriginalIntent: loop.OriginalIntent,
		Status: agentruntime.StatusCompleted, StopReason: agentloop.StopReasonDone,
	}
	if err := s.recordGoalResult(loop.ConversationID, res); err != nil {
		t.Fatalf("recordGoalResult failed: %v", err)
	}
	s.mu.Lock()
	item, live := s.durableContinuations["cont_d1_owed_live"]
	s.mu.Unlock()
	if !live || item.Status != ContinuationPending {
		t.Fatalf("an owed round's live checkpoint was terminalized underneath the resumable goal: %+v", item)
	}
}

// 钉②（修 2，面缺口）：D1-S1 干预应用成功的那一刻（projectD1Execution 落
// applied receipt），A/B 试听卡必须立即挂出——不等判定边界、不管链条后续是否
// 停摆。真栈 22:58 形态正是「应用成功 → 链条 stranding → 判定边界永不达 →
// 零 audition.* 事件」。
func TestProjectD1ExecutionMountsAuditionCardOnApply(t *testing.T) {
	s := New(nil, nil, nil)
	fake := &fakeAuditionKernel{}
	s.auditionKernel = fake
	s.auditionCandidateDriver = newCandidateDriverForTest(auditionProjectPathForTest(t))

	loop := d1LoopForTest(t, "7")
	// durable 链在应用前渲染的 before 相（真栈形态：ensureD1Render "before"）。
	beforePath := writeD1WAVForTest(t, "before_revision_7.wav")
	loop.D1State = map[string]any{
		"before_render": map[string]any{
			"phase": "before", "status": "ready", "file_path": beforePath, "project_revision": "7",
			"sha256": strings.Repeat("d", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "before", "7"),
			"preview_revision": "sha256:before",
		},
	}
	s.storeFreeStateLoop(loop)
	// after 相已就绪的既成形态：真栈上由 kernel start_render 产出（ensureD1Render
	// 的渲染/缓存两相是既有面，本钉钉的是「应用时刻调用挂卡」的时机——无内核
	// 的测试用 D1State 的 ready 行走 resume fast path，不触 kernel）。
	afterPath := writeD1WAVForTest(t, "after_revision_8.wav")
	loop.D1State["after_render"] = map[string]any{
		"phase": "after", "status": "ready", "file_path": afterPath, "project_revision": "8",
		"sha256": strings.Repeat("e", 64), "render_revision": d1RenderRevisionForTest(loop.Experiment.ID, "after", "8"),
		"preview_revision": "sha256:after",
	}
	s.storeFreeStateLoop(loop)

	receipt := orchestration.ActionReceipt{ActionID: "d1-action", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-d1-gap", "idempotency_key": "key-d1-gap",
		"actual_readback_db": -1.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session-gap", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-gap", IdempotencyKey: "key-d1-gap", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-gap-after", ObservationRevision: "8", EvidenceRefs: []string{"ccb-gap"}},
	}}

	resp := s.projectD1Execution(loop, session, nil)

	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("the applied loop was not stored")
	}
	if stored.AuditionSessionID == "" {
		t.Fatal("the applied intervention must mount its A/B audition card immediately — the 22:58 real stack mounted nothing because the card only ever fired at a judgment boundary the stranded chain never reached")
	}
	if len(fake.prepareRequests) != 1 {
		t.Fatalf("exactly one kernel audition session is prepared on apply: %d", len(fake.prepareRequests))
	}
	if request := fake.prepareRequests[0]; request.SessionID != stored.AuditionSessionID || len(request.Candidates) != 2 {
		t.Fatalf("prepare request=%+v", request)
	}
	// fail-open 纪律：挂卡不改变应用回执的既有契约。
	if resp.StopReason != "d1_post_action_evaluation_required" || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("the applied response contract changed: status=%s stop=%s", resp.GoalStatus, resp.StopReason)
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil || len(round.Interventions) != 1 {
		t.Fatalf("setup shape: the applied intervention must be booked on the round: %v %+v", err, round)
	}
}
