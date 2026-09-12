package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// PARK-1（D1 audition 判定驻留零文字投递）：goal_44cda25a 真栈取证——实验走
// D1 分支，17:55:07 materiality/target.response → 17:55:08 audition.ready +
// round.decision=user_judgment_pending 后 goal 以 **waiting_continue** 驻留
// （不是 waiting_confirmation）。驻留 turn 零 turn.completed、零 item 文本，
// 会话末条仍是调度链过场「我还在继续处理这个任务，完成后再向你汇报。」——
// 用户拿到 AB 卡却对「应用了什么/为什么判定」零文字可知。
//
// 根因（本文件钉住）：终局投递门 settleAndDeliverContinuationChainEnd 只有两
// 个可投递形态——完成态 settle 与「可应答 park」（goal = waiting_confirmation
// / waiting_clarification）。判定驻留的 goal 停在 waiting_continue，两个形态
// 都不匹配，落入「mid-chain slices, unanswerable legacy shells … deliver
// nothing」的静默分支。判定驻留因此有事件（audition 卡）而无文字。

const judgmentParkConversation = "conversation-park1"

// judgmentParkLoopForTest 构造停驻在 durable 人工判定边界的 D1 循环：应用回执
// 带参数增量与回读前后值，materiality + human_audition_ready 目标响应 +
// user_judgment_pending 回合决定已落账，loop.Status 是边界处写的 "blocked"。
// 形态对齐真栈 17:55:07/17:55:08 两行遥测。
func judgmentParkLoopForTest(t *testing.T) (*Server, freeStateReasoningLoop) {
	t.Helper()
	now := time.Now().UTC()
	s, loop := d2MultiRoundServerLoopForTest(t, 1)
	loop.ConversationID = judgmentParkConversation
	loop.Status = "blocked"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.OriginalIntent = "对低音轨做一次混音改进实验，我要 A/B 试听"

	// 应用：与 staticbalance 端口同形的回执（参数增量 + 回读前后值）。
	if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{
		ID: "park1-action-1", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true,
		Receipt: map[string]any{
			"action_id": "park1-action-1", "status": "applied", "after_revision": "8",
			"transaction_id": "tx-park1", "idempotency_key": "key-park1",
			"before_readback_db": -6.2, "actual_readback_db": -5.4, "readback_verified": true,
		},
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{
		State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable,
		Attempt: 1, EvidenceRefs: []string{"obs-park1-action-1"},
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordObservation(experiment.Observation{
		ID: "obs-park1-action-1", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"},
		ViewSetMatches: true, Fresh: true, PostAction: true, ProjectRevision: "8", EvidenceRefs: []string{"obs-park1-action-1"},
	}, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{
		Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
		Summary: "方向一致但幅度小，需人耳判定", EvidenceRefs: []string{"obs-park1-action-1"},
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.DecideRound(experiment.DecisionUserJudgment, "等待用户 A/B 试听判定", now); err != nil {
		t.Fatal(err)
	}
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: loop must satisfy the durable judgment boundary predicate")
	}
	s.storeFreeStateLoop(loop)
	return s, loop
}

// parkChainEndForTest 以真栈调度侧形态驱动链终局门：正在执行的切片记录已
// 完成（链上无活续跑持有者），响应携带驻留切片的过场回复与 waiting_continue
// goal 状态。
func parkChainEndForTest(s *Server, loop freeStateReasoningLoop, reply string) {
	s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	s.harness.SetGoalStatus(loop.GoalID, agentruntime.StatusWaitingContinue, nil)
	s.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_park1", GoalID: loop.GoalID, RunID: loop.RunID,
		ConversationID: loop.ConversationID, OriginalIntent: loop.OriginalIntent,
		Status: ContinuationCompleted,
	}, ChatResponse{
		ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID,
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: agentloop.StopReasonLimitReached, LimitType: "max_turns",
		Reply: reply,
	}, true)
}

func parkChainResultEvents(s *Server) []AgentEvent {
	events, _ := s.agentEventsSince(judgmentParkConversation, 0, 128)
	out := make([]AgentEvent, 0, len(events))
	for _, event := range events {
		if event.ItemID == "chain_result" {
			out = append(out, event)
		}
	}
	return out
}

// 钉①（RED→GREEN 主钉）：判定驻留的链终局必须向对话流投递 settle 摘要文本，
// 且文本承载五要素精神——应用了什么（参数 + 回读前后值）、针对的发现、请
// A/B 试听判定（指路判定卡）。投递的正文不得是调度链过场话术。
func TestJudgmentParkChainEndDeliversSettleSummary(t *testing.T) {
	s, loop := judgmentParkLoopForTest(t)
	parkChainEndForTest(s, loop, "我还在继续处理这个任务，完成后再向你汇报。")

	delivered := parkChainResultEvents(s)
	if len(delivered) != 1 {
		t.Fatalf("judgment park must deliver exactly one chain terminal to the conversation flow, got %d: %+v", len(delivered), delivered)
	}
	body := delivered[0].Body
	if strings.TrimSpace(body) == "" {
		t.Fatal("judgment park terminal must carry a non-empty settle summary")
	}
	if strings.Contains(body, "我还在继续处理这个任务") {
		t.Fatalf("the park summary must not be the scheduler pass-through line: %q", body)
	}
	// 要素一：应用了什么（参数 + 回读前后值）。
	for _, needle := range []string{"音量", "回读", "-6.2", "-5.4"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("park summary must state what was applied with its readback (%q missing): %q", needle, body)
		}
	}
	// 要素二：针对的发现。
	if !strings.Contains(body, "针对的发现") {
		t.Fatalf("park summary must state the finding it targets: %q", body)
	}
	// 要素三：请 A/B 试听判定（指路判定卡）。
	if !strings.Contains(body, "A/B") || !strings.Contains(body, "试听") {
		t.Fatalf("park summary must point at the A/B audition judgment: %q", body)
	}
	b6AssertNoInternalTerms(t, body)
	if strings.Contains(body, "人工判定") {
		t.Fatalf("the park summary is the servicable judgment entry itself and must not borrow the hollow-claim vocabulary: %q", body)
	}
	// 投递形态：park 终局带 scheduler_chain 标记与 waiting_continue 真实状态。
	if marked, _ := delivered[0].Payload["scheduler_chain"].(bool); !marked {
		t.Fatalf("judgment park terminal must carry the scheduler_chain marker: %+v", delivered[0].Payload)
	}
	if delivered[0].Status != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("judgment park terminal must annotate the real waiting_continue status, got %q", delivered[0].Status)
	}
}

// 钉②（水合）：驻留文本必须同时落进会话图（B1-DIAG Q3 的同一要求）——刷新后
// 用户仍能读到「应用了什么/为什么判定」，而不是只剩一张 AB 卡。
func TestJudgmentParkChainEndHydratesSettleSummary(t *testing.T) {
	_, parked := judgmentParkLoopForTest(t)
	// 水合床：带工程工作区的 harness（终局节点落图需要 projectPath）。
	s, projectPath := schedulerTerminalServerForTest(t)
	s.storeFreeStateLoop(parked)
	parkChainEndForTest(s, parked, "我还在继续处理这个任务，完成后再向你汇报。")

	messages := schedulerTerminalHydratedMessages(t, s, projectPath)
	found := ""
	for _, message := range messages {
		if !strings.Contains(message.Content, "针对的发现") {
			continue
		}
		found = message.Content
		if message.MessageKind == "proposal" || message.MessageData != nil {
			t.Fatalf("park summary node must hydrate as a plain report, not an interaction card: kind=%s data=%+v", message.MessageKind, message.MessageData)
		}
	}
	if found == "" {
		t.Fatalf("park settle summary must land in the conversation graph for refresh recovery: %+v", messages)
	}
	if !strings.Contains(found, "A/B") {
		t.Fatalf("hydrated park summary must keep the judgment pointer: %q", found)
	}
}

// 钉③（F2/F5 零回退，边界反向锁定）：投递门的既有边界不得被新分支放宽。
func TestJudgmentParkBranchKeepsDeliveryGateBoundaries(t *testing.T) {
	// ① legacy 不可答壳：goal waiting_continue + waiting_interaction 记录，
	//    对话上没有自由态循环 —— 仍零投递（F2 pin 3 的形态逐字保持）。
	legacyServer, _ := schedulerTerminalServerForTest(t)
	legacyServer.harness.EnsureGoal("goal-park1-legacy", "run-park1-legacy", "improve the mix")
	legacyServer.harness.SetGoalStatus("goal-park1-legacy", agentruntime.StatusWaitingContinue, nil)
	legacyServer.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_park1_legacy", GoalID: "goal-park1-legacy", RunID: "run-park1-legacy",
		ConversationID: judgmentParkConversation, Status: ContinuationWaitingInteraction,
	}, ChatResponse{
		ConversationID: judgmentParkConversation, GoalID: "goal-park1-legacy", RunID: "run-park1-legacy",
		GoalStatus: string(agentruntime.StatusWaitingContinue),
		StopReason: "legacy checkpoint has no authoritative stop reason",
	}, true)
	if events := parkChainResultEvents(legacyServer); len(events) != 0 {
		t.Fatalf("an unanswerable waiting_continue park must stay undelivered: %+v", events)
	}

	// ② 有循环但实验未停在人工判定边界（刚准入、还在跑）：仍零投递。
	liveServer, liveLoop := d2MultiRoundServerLoopForTest(t, 1)
	liveLoop.ConversationID = judgmentParkConversation
	if freeStateJudgmentBoundary(liveLoop) {
		t.Fatal("setup: a freshly admitted running round must not be at the judgment boundary")
	}
	liveServer.storeFreeStateLoop(liveLoop)
	parkChainEndForTest(liveServer, liveLoop, "第一片完成，后台继续。")
	if events := parkChainResultEvents(liveServer); len(events) != 0 {
		t.Fatalf("a waiting_continue park that is not the judgment boundary must stay undelivered: %+v", events)
	}

	// ③ 链未终结（中段切片）时判定驻留也不得提前投递。
	midServer, midLoop := judgmentParkLoopForTest(t)
	midServer.harness.EnsureGoal(midLoop.GoalID, midLoop.RunID, midLoop.OriginalIntent)
	midServer.harness.SetGoalStatus(midLoop.GoalID, agentruntime.StatusWaitingContinue, nil)
	midServer.settleAndDeliverContinuationChainEnd(context.Background(), DurableContinuation{
		ContinuationID: "cont_park1_mid", GoalID: midLoop.GoalID, RunID: midLoop.RunID,
		ConversationID: midLoop.ConversationID, Status: ContinuationCompleted,
	}, ChatResponse{
		ConversationID: midLoop.ConversationID, GoalID: midLoop.GoalID, RunID: midLoop.RunID,
		GoalStatus: string(agentruntime.StatusWaitingContinue), StopReason: "limit_reached",
		Reply: "第一片完成，后台继续。",
	}, false)
	if events := parkChainResultEvents(midServer); len(events) != 0 {
		t.Fatalf("a mid-chain slice must never deliver the park summary: %+v", events)
	}
}

// 钉④（同根）：投递判据与 park 自身的建立判据共用一个谓词
// freeStateJudgmentBoundary。边界在（实验未 settle/stop 且回合请求了判定或
// 判定已落账）时投递开，实验一旦收口投递随即关闭——两处不得各有一套
// 「算不算驻留」的说法。
func TestJudgmentParkDeliveryGateSharesBoundaryPredicate(t *testing.T) {
	s, parked := judgmentParkLoopForTest(t)
	if !freeStateJudgmentBoundary(parked) {
		t.Fatal("setup: loop must be at the judgment boundary")
	}
	if !s.judgmentBoundaryParkFor(parked.ConversationID) {
		t.Fatal("the delivery gate must recognize the same judgment-boundary park the loop establishes")
	}
	// 判定未落账但已请求（UserJudgmentRequested 而证据为空）：边界第一支，
	// 投递判据同步为真 —— 这正是真栈驻留的形态。
	requested := parked
	round, err := requested.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	round.UserJudgmentEvidence = []experiment.UserJudgmentEvidence{{ID: "judgment-park1"}}
	requested.Experiment.Rounds[len(requested.Experiment.Rounds)-1] = round
	if !freeStateJudgmentBoundary(requested) {
		t.Fatal("the experiment-scope boundary must hold while the round carries the judgment")
	}
	s.storeFreeStateLoop(requested)
	if !s.judgmentBoundaryParkFor(parked.ConversationID) {
		t.Fatal("the delivery gate must keep recognizing a boundary carrying recorded judgment evidence")
	}
	// 实验收口（settled）：边界解除，投递随之关闭。
	settled := requested
	settled.Experiment.Status = experiment.StatusSettled
	s.storeFreeStateLoop(settled)
	if freeStateJudgmentBoundary(settled) {
		t.Fatal("a settled experiment must not report the judgment boundary")
	}
	if s.judgmentBoundaryParkFor(parked.ConversationID) {
		t.Fatal("the delivery gate must close once the experiment settles")
	}
	// 会话上没有循环（legacy 壳）：判据不得凭空为真。
	if s.judgmentBoundaryParkFor("conversation-without-loop") {
		t.Fatal("a conversation without a free-state loop must not report a judgment park")
	}
}
// ---------------------------------------------------------------------------
// AUDITION-PLAY-1 验收补锚点勘察（2026-09-12）：判定请求落账前 WARN
// "[audition] user judgment request rejected: user judgment request requires
// canonical task state human_judgment_required"（B12-1 同形，两轮真栈均在场）。
//
// 勘察结论（本组钉住）：该 WARN 的唯一可达机制是
// requireTaskHumanJudgment 的 **无契约早退**——goal 上没有 canonical
// Task.Contract/SemanticState 时它 return nil 且**不做任何绑定**，实验侧却
// 仍带着更早一次绑定留下的 ContractID 与陈旧 TaskState；随后
// RequestUserJudgmentForSession 的 t.ContractID != "" 守卫 fail-closed。
// 迁移表本身不是成因：EventHumanJudgmentRequested 一旦成功必然落到
// human_judgment_required（taskstate/state.go:305-324），跳变事件返回 error
// 时会走另一条 WARN（"canonical human judgment transition rejected"）。
//
// 与 PARK-1 的关系：**不同根**。驻留判定与文字投递共用 freeStateJudgmentBoundary
// （不查 task 状态），所以判定请求的这一 WARN 不影响 park 摘要投递；它是
// harness Task 权威与实验投影之间的失步窗口，属判定请求落账面，另行定卡。

// 钉⑤-a：契约齐备时，同一调用路径正常落账——判定请求状态迁移本身不受阻。
func TestJudgmentRequestLandsWhenCanonicalContractIsBound(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil {
		t.Fatal("judgment request must not lose the loop")
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if !round.UserJudgmentRequested {
		t.Fatalf("with the canonical contract bound the judgment request must land: %+v", round)
	}
	if stored.Experiment.TaskState != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("landing the request must leave the experiment on the canonical human_judgment_required projection, got %q", stored.Experiment.TaskState)
	}
}

// 钉⑤-b：失步形态（实验带 ContractID + 陈旧 TaskState，goal 上没有 canonical
// Task）复现真栈 WARN：守卫无契约早退且零绑定，判定请求被 fail-closed 拒绝。
// 报文逐字钉住，使真栈日志行可以直接归因到本条机制。
func TestJudgmentRequestWarnIsTheContractDesyncEarlyReturn(t *testing.T) {
	s, loop := judgmentParkLoopForTest(t)
	// 失步：实验带着更早一次绑定留下的 ContractID 与陈旧投影，harness 侧
	// 该 goal 没有 Task 契约（requireTaskHumanJudgment 的早退条件）。
	loop.Experiment.ContractID = "contract-park1-stale"
	loop.Experiment.TaskState = taskstate.StateNeedsExperiment
	loop.Experiment.TaskStateRevision = 1
	if s.hasTaskSemanticContract(loop.GoalID) {
		t.Fatal("setup: the desync shape requires the goal to have no canonical task contract")
	}
	stale := loop.Experiment.TaskState
	if err := s.requireTaskHumanJudgment(&loop, "session-park1", "A/B audition judgment is required before experiment settlement"); err != nil {
		t.Fatalf("the early return must be silent, not an error: %v", err)
	}
	if loop.Experiment.TaskState != stale {
		t.Fatalf("the early return must bind nothing: task_state=%q want %q", loop.Experiment.TaskState, stale)
	}
	_, err := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", "session-park1", time.Now().UTC())
	if err == nil {
		t.Fatal("the desync shape must be fail-closed, not silently accepted")
	}
	const realStackWarn = "user judgment request requires canonical task state human_judgment_required"
	if err.Error() != realStackWarn {
		t.Fatalf("the rejected request must carry the real-stack WARN text verbatim:\n got %q\nwant %q", err.Error(), realStackWarn)
	}
	// 该拒绝不改动回合身份：判定 POST 的绑定修复支路因此仍有可服务的驻留。
	round, roundErr := loop.Experiment.CurrentRound()
	if roundErr != nil {
		t.Fatal(roundErr)
	}
	if round.UserJudgmentRequested {
		t.Fatal("the rejected request must not mark the round as judgment-requested")
	}
}

// 钉⑤-c（同根裁定）：驻留判定与文字投递共用 freeStateJudgmentBoundary，
// 该谓词完全不查 harness Task 状态——所以判定请求的契约失步 WARN 不会改变
// park 摘要的投递结论。这钉住「task 状态机与文字投递是否同根」的答案：
// 驻留/投递同根，判定请求的 WARN 是另一根。
func TestJudgmentParkDeliveryIgnoresTaskContractDesync(t *testing.T) {
	s, loop := judgmentParkLoopForTest(t)
	loop.Experiment.ContractID = "contract-park1-stale"
	loop.Experiment.TaskState = taskstate.StateNeedsExperiment
	loop.Experiment.TaskStateRevision = 1
	s.storeFreeStateLoop(loop)
	if !s.judgmentBoundaryParkFor(loop.ConversationID) {
		t.Fatal("the stale task projection must not close the judgment-boundary park")
	}
	parkChainEndForTest(s, loop, "我还在继续处理这个任务，完成后再向你汇报。")
	if events := parkChainResultEvents(s); len(events) != 1 {
		t.Fatalf("the park summary delivery must not depend on the canonical task projection, got %d events: %+v", len(events), events)
	}
}
