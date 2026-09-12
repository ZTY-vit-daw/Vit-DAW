package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

// D1-STALL-1（2026-09-12 18:52 真栈，用户判定为 bug）：goal 被置 completed
// 而 task 仍停在 needs_experiment —— D1 链停在应用后评估步即被当作完成。
//
// 真栈形态（会话 webui_mty9laqh / goal_bf7e1d520af31c94，工件在案）：
//   - 实验被受理并跑到干预应用（mix.tick +1.0 dB 落盘、trajectory.intervention.applied）；
//   - 应用回复刻意 park 在 waiting_continue + stop_reason=d1_post_action_evaluation_required，
//     把「应用后观察 -> materiality/目标评估 -> 判定边界」交给 scheduler 续跑；
//   - 五个续跑片走完，模型在评估步没有产出方向性结论，最后一拍 waiting_continue
//     之后 goal 被置 completed，而 task 的 semantic_state 仍是 needs_experiment；
//   - experiment_target_response.outcome 从未等于 human_audition_ready：零 audition
//     事件、零判定请求、用户零「继续」入口。
//
// 根因（本文件钉住）：两个 goal 终态写点都只问「链/循环还有没有下一步自动动作」，
// 不问「任务契约还欠不欠受治结果」——
//   A. settleGoalAfterContinuationEnd：链无活续跑持有者时把 running/waiting_continue
//      一律改写成 completed（并为孤儿任务落 owner_turn_closed）；
//   B. recordGoalResult 的续跑预算停止：ContinuationUsed > ContinuationBudget 时
//      无条件 SetGoalStatus(completed)。
// 于是「等待用户 / 预算耗尽 / 真完成」三态被压成一态。回合欠账（干预已落盘、
// 回合决定未落）在 freeStateLoopRoundPendingSettlement 自己的注释里已经写明了
// 危害：「projecting the goal completed would permanently lose the settlement」。

const d1StallConversation = "conversation-d1-stall"

// d1StallServerLoopForTest 构造真栈 18:52 那一拍的持久化形态：单轮 D1 实验
// （track_gain）已准入并应用，回合带着新鲜的应用后观察与自身执行回执（回读
// -6.2 -> -5.4），回合决定为空；循环停在应用后评估步；goal 停在
// waiting_continue；task 停在 needs_experiment。这四件事必须同时为真，才是
// 卡面点名的「goal completed 而 task needs_experiment」前置态。
func d1StallServerLoopForTest(t *testing.T) (*Server, freeStateReasoningLoop, agentruntime.Goal) {
	t.Helper()
	now := time.Now().UTC()
	s := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion:     freeStateReasoningLoopSchema,
		LoopID:            "loop-d1-stall",
		ConversationID:    d1StallConversation,
		GoalID:            "goal-d1-stall",
		RunID:             "run-d1-stall",
		Status:            "awaiting_experiment",
		OriginalIntent:    "对低音轨做一次混音改进实验，改完让我 A/B 试听对比一下",
		ActiveIntent:      "对低音轨做一次混音改进实验，改完让我 A/B 试听对比一下",
		MaxCycles:         freeStateDefaultMaxCycles,
		LatestObservation: d1FreshObservationForTest("7"),
		CreatedAt:         now, UpdatedAt: now,
	}
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	if _, err := s.ensureAudioTaskContract(loop.ConversationID, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "track", ID: "vocal", Label: "bass"}, "project-1", "7",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	proposal := taskstate.BoundedProposal{ProposalID: "proposal-d1-stall", Summary: "bounded track gain", EvidenceRefs: []string{"obs-before"}, RequiresExperiment: true}
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventDiagnosticCompleted, Reason: "candidate diagnosis established",
		EvidenceRefs: []string{"obs-before"}, ProjectRevision: "7"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventImprovementProposed, Reason: "candidate observed",
		EvidenceRefs: []string{"obs-before"}, CandidateID: "1007", Proposal: &proposal, ProjectRevision: "7"}); err != nil {
		t.Fatal(err)
	}
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	if err := s.bindExperimentSemantic(&loop); err != nil {
		t.Fatal(err)
	}
	// 干预已应用并回读（回执与 staticbalance 端口同形）。
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{
		ID: "d1-stall-action-1", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true,
		Receipt: map[string]any{
			"action_id": "d1-stall-action-1", "status": "applied", "after_revision": "8",
			"transaction_id": "tx-d1-stall", "idempotency_key": "key-d1-stall",
			"before_readback_db": -6.2, "actual_readback_db": -5.4, "readback_verified": true,
		},
	}, now); err != nil {
		t.Fatal(err)
	}
	// 应用后观察已就绪（真栈 requires_post_action_observation=false 的含义）。
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{
		ID: "obs-d1-stall-after", RequestedViewIDs: []string{"track.timbre_frequency"},
		ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true,
		Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"obs-d1-stall-after"},
	}, true, now); err != nil {
		t.Fatal(err)
	}
	loop.Status = "re_evaluating"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.RequiresPostActionObservation = false
	loop.ContinuationBudget = 6
	loop.ContinuationUsed = 5
	s.storeFreeStateLoop(loop)
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)
	return s, loop, s.harness.RuntimeStatus(loop.GoalID)
}

func d1StallStoredLoop(t *testing.T, s *Server) freeStateReasoningLoop {
	t.Helper()
	loop, ok := s.freeStateLoop(d1StallConversation)
	if !ok {
		t.Fatal("the durable free-state loop disappeared")
	}
	return loop
}

// d1StallContinuationResult builds the slice result envelope the scheduler
// hands to recordGoalResult for this conversation's own goal (the shared test
// helper carries a foreign goal id, which would make every goal-status
// assertion vacuous).
func d1StallContinuationResult(loop freeStateReasoningLoop, slice, turn string) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
		SliceID: slice, TurnID: turn, OriginalIntent: loop.OriginalIntent,
		Status: agentruntime.StatusWaitingContinue, StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-d1-stall",
			SliceID: slice, TurnID: turn, OriginalIntent: loop.OriginalIntent,
			UserText: loop.OriginalIntent, Summary: loop.OriginalIntent,
			Context: map[string]any{"free_state_reasoning_loop": map[string]any{"status": "re_evaluating"}},
		},
	}
}

func d1StallChainResultEvents(s *Server) []AgentEvent {
	events, _ := s.agentEventsSince(d1StallConversation, 0, 128)
	out := make([]AgentEvent, 0, len(events))
	for _, event := range events {
		if event.ItemID == "chain_result" {
			out = append(out, event)
		}
	}
	return out
}

// d1StallChainEnd drives the scheduler-side chain-end gate in the real shape:
// the running slice's checkpoint has completed, the chain has no live owner,
// and the slice response carries the parked waiting_continue status plus the
// scheduler pass-through line.
func d1StallChainEnd(s *Server, loop freeStateReasoningLoop, reply string, chainEnded bool) {
	s.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_d1_stall", GoalID: loop.GoalID, RunID: loop.RunID,
		ConversationID: loop.ConversationID, OriginalIntent: loop.OriginalIntent,
		Status: ContinuationCompleted,
	}, ChatResponse{
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "d1_post_action_evaluation_required", LimitType: "max_turns",
		Reply: reply,
	}, chainEnded)
}

// 钉①（RED→GREEN 主钉）：链在应用后评估步停止且无活续跑持有者时，goal 不得被
// 改写成 completed。任务契约仍欠受治结果，goal 必须留在可续跑形态
// （shouldResumeGoalFromStatus 承认的 waiting_continue），而不是「等待用户 /
// 预算耗尽 / 真完成」三态合一的 completed。
func TestD1StallChainEndKeepsGoalResumableWhileRoundOwesEvaluation(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	if !freeStateLoopOwesExperimentOutcome(loop) {
		t.Fatalf("setup: the applied round must owe its governed outcome: %+v", loop.Experiment.Rounds)
	}
	if !s.freeStateLoopOwesExperimentOutcomeFor(loop.ConversationID, goal.GoalID) {
		t.Fatal("setup: the conversation's durable loop must be the owing one")
	}

	d1StallChainEnd(s, loop, "我还在继续处理这个任务，完成后再向你汇报。", true)

	after := s.harness.RuntimeStatus(loop.GoalID)
	if after.Status == agentruntime.StatusCompleted {
		t.Fatalf("a chain stopped at the post-apply evaluation step must not be projected as completed work: status=%s task=%s",
			after.Status, after.Task.SemanticState.State)
	}
	if after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the owing goal must stay in the resumable waiting_continue form, got %s", after.Status)
	}
	if !shouldResumeGoalFromStatus(after.Status) {
		t.Fatalf("the goal status %s must be one a follow-up turn resumes from", after.Status)
	}
}

// 钉①-b（真栈持久化形态）：把循环的呈现状态写成边界驻留的 blocked。真栈那一拍
// 循环不是活跃态——否则 taskHasLiveContinuationOwner 早已把 goal 豁免，completed
// 根本写不下去。这一形态只有回合欠账判据救得回来：活跃判据与判定边界判据都不为
// 真，而任务契约仍欠受治结果（干预已落盘、回合决定未落）。
func TestD1StallChainEndSparesBlockedResidencyWithOwingRound(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	blocked := d1StallStoredLoop(t, s)
	blocked.Status = "blocked"
	blocked.DecisionPhase = freeStatePhasePostActionEvaluation
	blocked.LastError = "experiment round is waiting for the post-apply evaluation"
	s.storeFreeStateLoop(blocked)
	if freeStateLoopActive(blocked) || freeStateJudgmentBoundary(blocked) {
		t.Fatalf("setup: the fixture must not be spared by the pre-existing live-owner predicates: active=%v boundary=%v",
			freeStateLoopActive(blocked), freeStateJudgmentBoundary(blocked))
	}
	if !freeStateLoopOwesExperimentOutcome(blocked) {
		t.Fatal("setup: the blocked-residency round must still owe its governed outcome")
	}

	d1StallChainEnd(s, loop, "我还在继续处理这个任务，完成后再向你汇报。", true)

	after := s.harness.RuntimeStatus(loop.GoalID)
	if after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the blocked-residency owing round must keep the goal resumable, got %s (task=%s)", after.Status, after.Task.SemanticState.State)
	}
	if state := after.Task.SemanticState; state.State != taskstate.StateNeedsExperiment || state.Terminal {
		t.Fatalf("the blocked-residency owing round's task must not be orphan-closed: %+v", state)
	}
	if events := d1StallChainResultEvents(s); len(events) != 1 {
		t.Fatalf("the blocked-residency owing park must still deliver its explicit receipt, got %d: %+v", len(events), events)
	}
}

// 钉②（task 语义一致性）：同样的停止不得把 task 当孤儿收口。真实缺陷里 task
// 停在 needs_experiment（非终态）却被 goal completed 覆盖，用户视角是「任务
// 完成了」而实验语义还欠着；owner_turn_closed 会把这一欠账抹成终态。
func TestD1StallChainEndSparesOwingRoundTask(t *testing.T) {
	s, loop, goal := d1StallServerLoopForTest(t)
	d1StallChainEnd(s, loop, "我还在继续处理这个任务，完成后再向你汇报。", true)

	after := s.harness.RuntimeStatus(loop.GoalID)
	state := after.Task.SemanticState
	if state == nil {
		t.Fatal("the task disappeared at chain end")
	}
	if state.State != taskstate.StateNeedsExperiment || state.Terminal {
		t.Fatalf("the owing round's task must stay a non-terminal needs_experiment: %+v", state)
	}
	for _, record := range state.History {
		if record.Event == taskstate.EventOwnerTurnClosed {
			t.Fatalf("the owing round's task must not be orphan-closed: %+v", state.History)
		}
	}
	if goal.Task.SemanticState.Revision != state.Revision {
		t.Fatalf("chain end advanced the owing task: before=%d after=%d", goal.Task.SemanticState.Revision, state.Revision)
	}
}

// 钉③（人话回执）：驻留必须显式上抛「评估未完成」，而不是静默完成。正文要
// 说清应用了什么（参数 + 回读前后值）、针对的发现、这一步没有被判定为完成，
// 以及「继续」这个入口；不得是调度链过场话术，不得泄漏内部代号。
func TestD1StallChainEndDeliversExplicitOwedEvaluationReceipt(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	d1StallChainEnd(s, loop, "我还在继续处理这个任务，完成后再向你汇报。", true)

	delivered := d1StallChainResultEvents(s)
	if len(delivered) != 1 {
		t.Fatalf("the owing park must deliver exactly one chain terminal to the conversation flow, got %d: %+v", len(delivered), delivered)
	}
	body := delivered[0].Body
	if strings.TrimSpace(body) == "" {
		t.Fatal("the owing park terminal must carry a non-empty receipt")
	}
	if strings.Contains(body, "我还在继续处理这个任务") {
		t.Fatalf("the receipt must not be the scheduler pass-through line: %q", body)
	}
	for _, needle := range []string{"音量", "回读", "-6.2", "-5.4", "针对的发现"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("the receipt must state what was applied with its readback (%q missing): %q", needle, body)
		}
	}
	if !strings.Contains(body, "继续") {
		t.Fatalf("the receipt must carry the explicit continue entry: %q", body)
	}
	if !strings.Contains(body, "没") {
		t.Fatalf("the receipt must state that the work is unfinished: %q", body)
	}
	b6AssertNoInternalTerms(t, body)
	if marked, _ := delivered[0].Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("the receipt must carry the scheduler_chain marker: %+v", delivered[0].Payload)
	}
	if delivered[0].Status != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("the receipt must annotate the real waiting_continue status, got %q", delivered[0].Status)
	}
}

// 钉④（预算停止的显式化，RED→GREEN）：续跑预算耗尽停下调度，不等于任务做完。
// 欠账回合上「waiting_continue + 预算耗尽」必须落 waiting_continue（可续跑），
// 而不是 completed；同一函数在回合不欠账时仍落 completed（见钉⑤）。
func TestD1StallBudgetStopKeepsGoalResumableWhileRoundOwesEvaluation(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	owing := d1StallStoredLoop(t, s)
	// 真栈形态：预算被走满（ContinuationUsed 越过 ContinuationBudget）。
	owing.ContinuationUsed = owing.ContinuationBudget + 1
	s.storeFreeStateLoop(owing)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)

	if err := s.recordGoalResult(d1StallConversation, d1StallContinuationResult(loop, "slice-d1-stall", "turn-d1-stall")); err != nil {
		t.Fatal(err)
	}

	after := s.harness.RuntimeStatus(loop.GoalID)
	if after.Error != "" {
		t.Fatalf("setup: the budget stop must not fail closed on persistence: %+v", after)
	}
	if after.Status == agentruntime.StatusCompleted {
		t.Fatalf("an exhausted continuation budget must not be projected as completed work while the round owes its evaluation (task=%s)",
			after.Task.SemanticState.State)
	}
	if after.Status != agentruntime.StatusWaitingContinue {
		t.Fatalf("the owing goal must keep the resumable waiting_continue form across the budget stop, got %s", after.Status)
	}
	// 预算停止本身必须仍然可查询：循环 blocked + 预算耗尽停因（既有语义零回退）。
	stored := d1StallStoredLoop(t, s)
	if stored.Status != "blocked" || stored.LatestDecision == nil ||
		stored.LatestDecision.StopReason != freeStateContinuationBudgetExhaustedReason {
		t.Fatalf("the budget stop must stay queryable on the loop: status=%q decision=%+v", stored.Status, stored.LatestDecision)
	}
}

// 钉⑤（真完成仍 completed，零回退）：同一停止点，回合不欠账（实验已收口）时
// goal 必须照旧 completed、task 照旧收口。这是「真完成 -> completed」的反向钉，
// 保证新判据不会把正常收尾也拖成驻留。
func TestD1StallFinishedRoundStillSettlesCompleted(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	settled := d1StallStoredLoop(t, s)
	s.settleFreeStateExperiment(&settled, experiment.OutcomeStable, "应用后评估完成，结论已给出")
	if settled.Experiment.Status != experiment.StatusSettled {
		t.Fatalf("setup: the experiment must settle: %s", settled.Experiment.Status)
	}
	s.storeFreeStateLoop(settled)
	if freeStateLoopOwesExperimentOutcome(settled) {
		t.Fatalf("a settled experiment must not owe a governed outcome: %+v", settled.Experiment)
	}

	d1StallChainEnd(s, loop, "应用后评估完成，结论已给出。", true)

	after := s.harness.RuntimeStatus(loop.GoalID)
	if after.Status != agentruntime.StatusCompleted {
		t.Fatalf("a finished round's chain end must still settle the goal completed, got %s", after.Status)
	}
	if state := after.Task.SemanticState; state == nil || state.State != taskstate.StateSettled {
		t.Fatalf("a finished round's task must settle, got %+v", state)
	}
}

// 钉⑤-b（预算停止的反向钉）：回合不欠账时预算停止照旧落 completed。
func TestD1StallBudgetStopStillCompletesFinishedRound(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	settled := d1StallStoredLoop(t, s)
	s.settleFreeStateExperiment(&settled, experiment.OutcomeStable, "应用后评估完成，结论已给出")
	settled.ContinuationUsed = settled.ContinuationBudget + 1
	s.storeFreeStateLoop(settled)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)

	if err := s.recordGoalResult(d1StallConversation, d1StallContinuationResult(loop, "slice-d1-stall-done", "turn-d1-stall-done")); err != nil {
		t.Fatal(err)
	}
	if after := s.harness.RuntimeStatus(loop.GoalID); after.Status != agentruntime.StatusCompleted {
		t.Fatalf("a finished round's budget stop must still settle completed, got %s", after.Status)
	}
}

// 钉⑥（恢复路径，卡面待钉问题②）：同会话说一句「继续」必须重新入链到同一个
// goal（不是新开一个 goal/run），而这正是 goal 留在 waiting_continue 的意义——
// 判据 shouldResumeGoalFromStatus 只承认 waiting_* 三态。真栈 §6d 观察到的
// 「再说一句话即会重新入链」在本钉里落成可执行断言。
func TestD1StallResumeEntryReentersChainAfterBudgetStop(t *testing.T) {
	s, loop, _ := d1StallServerLoopForTest(t)
	owing := d1StallStoredLoop(t, s)
	owing.ContinuationUsed = owing.ContinuationBudget + 1
	s.storeFreeStateLoop(owing)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)
	if err := s.recordGoalResult(d1StallConversation, d1StallContinuationResult(loop, "slice-d1-stall-resume", "turn-d1-stall-resume")); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.conversationGoals[d1StallConversation] = loop.GoalID
	s.mu.Unlock()

	after := s.harness.RuntimeStatus(loop.GoalID)
	if !shouldResumeGoalFromStatus(after.Status) {
		t.Fatalf("the follow-up turn would not resume goal %s in status %s", loop.GoalID, after.Status)
	}
	if !s.shouldResumeConversationGoal(d1StallConversation) {
		t.Fatal("the conversation must still resume its owing goal after the budget stop")
	}
	resumed := s.beginChatGoal(d1StallConversation, "继续", map[string]any{"goal_id": loop.GoalID, "run_id": loop.RunID})
	if resumed.GoalID != loop.GoalID {
		t.Fatalf("a bare continue must re-enter the same goal chain, got %s want %s", resumed.GoalID, loop.GoalID)
	}
	if resumed.RunID != loop.RunID {
		t.Fatalf("a bare continue must keep the run identity, got %s want %s", resumed.RunID, loop.RunID)
	}
	// 同根：入链判据承认的正是终态守卫保留的那个状态。
	if freeStateLoopOwesExperimentOutcome(owing) && resumed.Status == agentruntime.StatusCompleted {
		t.Fatal("the resume entry and the terminal settle must not disagree about the owing round")
	}
}

// 钉⑦（判据同根 + 边界锁定）：欠账判据只读回合欠账，不读循环的呈现状态——
// 循环被写成什么 status 都不改变「任务还欠受治结果」这个事实；而没有任何
// 欠账形态（循环不存在 / 实验已收口 / 中段切片 / 判定边界）必须维持既有行为。
func TestD1StallOwedOutcomePredicateIsRoundDebtKeyed(t *testing.T) {
	_, loop, _ := d1StallServerLoopForTest(t)
	for _, status := range []string{"re_evaluating", "blocked", "observing", "capability_blocked", "completed"} {
		probe := loop
		probe.Status = status
		if !freeStateLoopOwesExperimentOutcome(probe) {
			t.Fatalf("the round's debt must not depend on the loop's presentation status %q", status)
		}
	}
	// 应用边界债务位（干预已落盘、应用后观察尚未入账）同样欠账。
	debt := loop
	debt.RequiresPostActionObservation = true
	if !freeStateLoopOwesExperimentOutcome(debt) {
		t.Fatal("the applied-boundary observation debt must count as an owed outcome")
	}
	// 没有实验 / 实验已收口 / 已停止：不欠账。
	bare := loop
	bare.Experiment = nil
	if freeStateLoopOwesExperimentOutcome(bare) {
		t.Fatal("a loop without an experiment owes nothing")
	}
	for _, status := range []experiment.Status{experiment.StatusSettled, experiment.StatusStopped} {
		done := loop
		done.Experiment.Status = status
		if freeStateLoopOwesExperimentOutcome(done) {
			t.Fatalf("a %s experiment must not owe a governed outcome", status)
		}
	}
	// 会话上没有循环 / 循环绑在别的 goal：判据不得凭空为真。
	s := New(nil, nil, nil)
	if s.freeStateLoopOwesExperimentOutcomeFor(d1StallConversation, loop.GoalID) {
		t.Fatal("a conversation without a loop must not report an owed outcome")
	}
	s.storeFreeStateLoop(loop)
	if s.freeStateLoopOwesExperimentOutcomeFor(d1StallConversation, "goal-somebody-else") {
		t.Fatal("a loop bound to another goal must not be read as this goal's owner")
	}
}

// 钉⑧（投递门边界零回退）：新分支不得放宽既有边界——无循环的 legacy
// waiting_continue 壳仍零投递；链未终结（中段切片）不得提前投递；判定驻留
// （PARK-1 形态）仍走判定摘要分支而不是欠账回执。
func TestD1StallDeliveryGateKeepsExistingBoundaries(t *testing.T) {
	// ① legacy 不可答壳：goal waiting_continue、对话上没有自由态循环 -> 零投递。
	legacy, _ := schedulerTerminalServerForTest(t)
	legacy.harness.EnsureGoal("goal-d1-stall-legacy", "run-d1-stall-legacy", "improve the mix")
	legacy.harness.SetGoalStatus("goal-d1-stall-legacy", agentruntime.StatusWaitingContinue, nil)
	legacy.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_d1_stall_legacy", GoalID: "goal-d1-stall-legacy", RunID: "run-d1-stall-legacy",
		ConversationID: d1StallConversation, Status: ContinuationWaitingInteraction,
	}, ChatResponse{
		ConversationID: d1StallConversation, GoalID: "goal-d1-stall-legacy", RunID: "run-d1-stall-legacy",
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "legacy checkpoint has no authoritative stop reason",
	}, true)
	if events := d1StallChainResultEvents(legacy); len(events) != 0 {
		t.Fatalf("an unanswerable waiting_continue shell must stay undelivered: %+v", events)
	}

	// ② 中段切片（链未终结）：欠账回合也不得提前投递。
	midServer, midLoop, _ := d1StallServerLoopForTest(t)
	d1StallChainEnd(midServer, midLoop, "第一片完成，后台继续。", false)
	if events := d1StallChainResultEvents(midServer); len(events) != 0 {
		t.Fatalf("a mid-chain slice must never deliver the owed-evaluation receipt: %+v", events)
	}

	// ③ 判定驻留（PARK-1）：同一 waiting_continue 形态但停在判定边界时，正文
	//    仍是判定摘要（A/B 判定入口），不是「评估未完成」的欠账回执。
	parkServer, parked := judgmentParkLoopForTest(t)
	parked.ConversationID = d1StallConversation
	parked.GoalID = "goal-d1-stall-park"
	parkServer.harness.EnsureGoal(parked.GoalID, parked.RunID, parked.OriginalIntent)
	parkServer.harness.SetGoalStatus(parked.GoalID, agentruntime.StatusWaitingContinue, nil)
	parkServer.storeFreeStateLoop(parked)
	parkServer.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_d1_stall_park", GoalID: parked.GoalID, RunID: parked.RunID,
		ConversationID: d1StallConversation, Status: ContinuationCompleted,
	}, ChatResponse{
		ConversationID: d1StallConversation, GoalID: parked.GoalID, RunID: parked.RunID,
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "limit_reached",
		Reply: "我还在继续处理这个任务，完成后再向你汇报。",
	}, true)
	parkEvents := d1StallChainResultEvents(parkServer)
	if len(parkEvents) != 1 {
		t.Fatalf("the judgment park must keep delivering its own summary, got %d: %+v", len(parkEvents), parkEvents)
	}
	if strings.Contains(parkEvents[0].Body, "还没做完") {
		t.Fatalf("a judgment park must use the judgment summary, not the owed-evaluation receipt: %q", parkEvents[0].Body)
	}
	if !strings.Contains(parkEvents[0].Body, "A/B") {
		t.Fatalf("the judgment park summary must keep its A/B judgment pointer: %q", parkEvents[0].Body)
	}
}
