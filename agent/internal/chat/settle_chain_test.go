package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// SETTLE-CHAIN-1（2026-09-28 真栈 run 20260928_120230 / 20260928_120725，两次均
// 以 "acoustic materiality record is missing" 断裂 free_state_d1 冒测，§8 止损后
// 上交本卡）。
//
// 根因链（工件+代码锚点亲核）：
//
//	1. D1-AUDITION-GAP-1 把 A/B 挂卡提前到应用边界（free_state_d1_runtime.go 的
//	   prepareFreeStateAudition），audition.ready 因此在 settle 报告落地之前触发；
//	2. requestAuditionJudgment 先做 requireTaskHumanJudgment（canonical task
//	   needs_experiment → human_judgment_required），后调
//	   RequestUserJudgmentForSession——后者因 round 还没有 human_audition_ready
//	   target response 被拒（真栈 WARN "user judgment requires human_audition_ready
//	   target response"，两 run 俱在）；
//	3. task 被留在 human_judgment_required 而 round 仍欠 settle。settle 片的决策
//	   （settle 报告按契约搭载 improvement_proposal/needs_experiment 形态）在
//	   applyFreeStateDecisionSemantic 处被迁移表拒绝："semantic transition
//	   improvement_proposed is not allowed from human_judgment_required"
//	   （taskstate/state.go:280 只认 ObservationInProgress/DiagnosticComplete），
//	   recordFreeStateDecision 早退 → recordFreeStateExperimentDecision 永不执行
//	   → round.Materiality/TargetResponse 永不产生 → 冒测断言断裂；
//	4. 变体（120725）：同一悬空边界让模型在 settle 片改报 needs_observation，预算
//	   耗尽于 waiting_interaction——同根不同表现。
//
// 本文件两测试钉住修复契约：canonical 判定边界迁移不得先于 round 自身成形
// （钉 1-3）；settle 报告落地后判定边界必须被重新驱动武装（钉 GAP-1 提前挂卡
// 之后丢失的 boundary-time arming）。

// settleChainMountedLoop 在 d1StallServerLoopForTest 的真栈前置态（单轮 D1 已
// 应用+回读+新鲜 post-action 观察、task needs_experiment）之上，挂上 GAP-1 应用
// 边界已 mount 好的 A/B 会话投影（session id 必须与 prepareFreeStateAudition 的
// 幂等键同形），并把干预回执补成真栈的 before/after 双 revision 形。
func settleChainMountedLoop(t *testing.T) (*Server, freeStateReasoningLoop) {
	t.Helper()
	s, loop, _ := d1StallServerLoopForTest(t)
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	round.Interventions[0].Receipt["before_revision"] = "7"
	loop.Experiment.Rounds[len(loop.Experiment.Rounds)-1] = round
	sessionID := "audition:" + loop.Experiment.ID + ":" + round.ID
	loop.AuditionSessionID = sessionID
	// 真栈 GAP-1 挂卡形态：A/B 渲染对在会话 ready 前已就绪。
	loop.D1State = map[string]any{
		"before_render": map[string]any{"phase": "before", "status": "ready", "project_revision": "7", "render_revision": "before:7"},
		"after_render":  map[string]any{"phase": "after", "status": "ready", "project_revision": "8", "render_revision": "after:8"},
	}
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": sessionID, "conversation_id": loop.ConversationID,
		"turn_id": loop.Experiment.ID, "round_id": round.ID,
		"status": "ready", "scope": "target", "project_revision": "8",
		"candidates": []any{
			map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "checkpoint:7", "preview_ref": "preview:a"},
			map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "action:7", "preview_ref": "preview:b"},
		},
	}
	s.storeFreeStateLoop(loop)
	return s, loop
}

// 钉根因：round 还没有 human_audition_ready target response（settle 报告未落地）
// 时，audition.ready 驱动的判定请求不得把 canonical task 迁到
// human_judgment_required——那是 settle 决策过不去的边界（真栈 120230 的
// "improvement_proposed is not allowed from human_judgment_required"）。
func TestAuditionReadyDoesNotTransitionTaskBeforeSettleReport(t *testing.T) {
	s, loop := settleChainMountedLoop(t)

	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)

	state := s.harness.RuntimeStatus(loop.GoalID).Task.SemanticState
	if state == nil || state.State != taskstate.StateNeedsExperiment {
		t.Fatalf("the canonical task must stay needs_experiment until the round records its human_audition_ready target response, got %+v — the 2026-09-28 stranded boundary", state)
	}
	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil {
		t.Fatal("stored loop disappeared")
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.UserJudgmentRequested {
		t.Fatalf("a pre-settle round must not be marked judgment-requested: %+v", round)
	}
}

// 钉修复后的完整 settle 链：settle 报告（搭载 needs_experiment 形态+保留提案）
// 落地时，round 必须产生 materiality/target_response 记录、回执保持
// before/after revision 区分、receipt 层同步刷新，且已提前挂卡的 A/B 会话把
// 判定边界武装起来（UserJudgmentRequested + task human_judgment_required +
// experiment waiting_for_user）。
func TestSettleReportLandsAndReArmsJudgmentBoundaryWithPreMountedSession(t *testing.T) {
	s, loop := settleChainMountedLoop(t)

	res := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-settle-chain",
		SliceID: "slice-settle", TurnID: "turn-settle", OriginalIntent: loop.OriginalIntent,
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:              agentloop.FreeStateNeedsExperiment,
			Summary:             "bounded EQ applied; energy delta stays subthreshold, user A/B arbitrates",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose,
				Attempt: 1, EvidenceRefs: []string{"obs-d1-stall-after"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
				Summary: "同频段能量变化未达可判定材料性，交由用户 A/B 判定", EvidenceRefs: []string{"obs-d1-stall-after"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
		Continuation: &agentloop.Continuation{
			GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-settle-chain",
			SliceID: "slice-settle", TurnID: "turn-settle", OriginalIntent: loop.OriginalIntent,
			Context: map[string]any{"free_state_reasoning_loop": map[string]any{"status": "re_evaluating"}},
		},
	}
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("settle turn was not retained by recordFreeStateDecision")
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.Materiality == nil || round.Materiality.State != experiment.MaterialitySubthreshold {
		t.Fatalf("the settle report must record the round's acoustic materiality: %+v", round.Materiality)
	}
	if round.TargetResponse == nil || round.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady {
		t.Fatalf("the settle report must record the round's target response: %+v", round.TargetResponse)
	}
	receipt := round.Interventions[0].Receipt
	before, after := firstStringFromMap(receipt, "before_revision"), firstStringFromMap(receipt, "after_revision")
	if before == "" || after == "" || before == after {
		t.Fatalf("the applied intervention receipt must keep distinct before/after revisions, got %q -> %q", before, after)
	}
	// storeFreeStateLoop 只刷新入库副本的 receipt 投影，断言面必须重取 store。
	persisted, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || persisted.Experiment == nil {
		t.Fatal("persisted loop disappeared after the settle turn")
	}
	persistedRound, err := persisted.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	layers := firstMapFromAny(persisted.D1Receipt["layers"])
	if firstStringFromMap(firstMapFromAny(layers["acoustic_materiality"]), "status") != "subthreshold" ||
		firstStringFromMap(firstMapFromAny(layers["target_response"]), "status") != "ambiguous" {
		t.Fatalf("the D1 receipt layers must reflect the settle records: %+v", layers)
	}
	if persisted.D1Receipt["evaluation_ready"] != true || persisted.D1Receipt["human_audition_ready"] != true {
		t.Fatalf("the D1 receipt flags must stay ready after the settle lands: %+v", persisted.D1Receipt)
	}
	// GAP-1 提前挂卡后，boundary-time 的 audition.ready 不会再触发；settle 落地
	// 必须自己把判定边界武装起来。
	if !persistedRound.UserJudgmentRequested {
		t.Fatalf("the pre-mounted audition session must have its judgment request armed at settle time: %+v", persistedRound)
	}
	if persisted.Experiment.Status != experiment.StatusWaitingForUser {
		t.Fatalf("the armed boundary must park the experiment at waiting_for_user, got %q", persisted.Experiment.Status)
	}
	state := s.harness.RuntimeStatus(loop.GoalID).Task.SemanticState
	if state == nil || state.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("the settle-time arming must move the canonical task to human_judgment_required, got %+v", state)
	}
}

// 钉变体②（真栈 run 194926：单发 booking 在 +6s 拿到 revision=2 的旧包、
// 欠账搁浅、观察窗耗尽 capability_blocked）：确定性 post-action 观察入账必须
// 在响应链内有界重试——暂态旧 revision 后的第二次尝试必须落地，预算耗尽后
// 必须停手，context 取消必须立即退出。红证据=该真栈工件；本测试钉回归契约。
func TestD1PostActionBookingRetrySchedule(t *testing.T) {
	calls := 0
	book := func() bool {
		calls++
		return calls >= 2 // 首发旧 revision（假绿 mismatch -> false），次发落地。
	}
	if !d1RetryPostActionBooking(context.Background(), 6, time.Millisecond, book) {
		t.Fatalf("a transiently stale first booking must land on retry, calls=%d", calls)
	}
	if calls != 2 {
		t.Fatalf("the schedule must stop at the first successful attempt, calls=%d", calls)
	}
	calls = 0
	if d1RetryPostActionBooking(context.Background(), 3, time.Millisecond, func() bool { calls++; return false }) {
		t.Fatal("an always-failing booking must not report success")
	}
	if calls != 3 {
		t.Fatalf("the schedule must respect the attempt budget exactly, calls=%d", calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if d1RetryPostActionBooking(ctx, 5, time.Millisecond, func() bool { calls++; return false }); calls > 1 {
		t.Fatalf("a cancelled context must stop the schedule immediately, calls=%d", calls)
	}
}

// 钉 run 200332 的微观序列：确定性 booking 已把 post-action 观察入账
// （欠账标志清零）后，后续切片的信封仍会回放 mutation 前的 CCB 包——
// 摄取循环在欠账清零后失去资格守卫，旧包被追加到 booked 观察之后，
// RecordTargetResponse 的「最后一条必须是 post-action」次序不变量被破坏，
// settle 报告以 "target response requires a fresh post-action observation"
// 被拒。守卫契约：round 已持有新鲜 post-action 观察后，非 post-action 的
// 信封观察一律不得再入账。
func TestPreActionObservationReplayCannotDemoteBookedPostActionEvidence(t *testing.T) {
	s, loop := settleChainMountedLoop(t)

	// 形态同 200332 的 turn-4 切片：needs_observation 决策 + 信封回放
	// mutation 前（revision 7）的 CCB 包。
	replay := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-settle-chain",
		SliceID: "slice-replay", TurnID: "turn-replay", OriginalIntent: loop.OriginalIntent,
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:  agentloop.FreeStateNeedsObservation,
			Summary: "re-observing before settling",
		},
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "tool_call_id": "replay-pre-action", "status": "ready",
			"result": map[string]any{
				"bundle": map[string]any{
					"schema_version": "ccb_observation_bundle.v1", "status": "ready",
					"observation_id":           "obs-replay-pre",
					"requested_views":          []any{"track.timbre_frequency"},
					"actual_executed_view_ids": []any{"track.timbre_frequency"},
					"freshness":                map[string]any{"status": "fresh", "project_revision": "7"},
					"project_binding":          map[string]any{"project_revision": "7"},
					"evidence_refs":            []any{"obs-replay-pre"},
				},
			},
		}},
	}
	if _, ok := s.recordFreeStateDecision(loop.ConversationID, replay); !ok {
		t.Fatal("replay turn was not retained")
	}
	afterReplay, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || afterReplay.Experiment == nil {
		t.Fatal("stored loop disappeared after the replay turn")
	}
	round, err := afterReplay.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if last := round.Observations[len(round.Observations)-1]; !last.PostAction || last.ProjectRevision != "8" {
		t.Fatalf("the booked post-action observation must stay the round's last evidence, got post=%v rev=%s — the 200332 demotion", last.PostAction, last.ProjectRevision)
	}

	// settle 报告必须随后正常落地（target response 不再被次序不变量拒绝）。
	settle := agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, TaskID: "task-settle-chain",
		SliceID: "slice-settle", TurnID: "turn-settle", OriginalIntent: loop.OriginalIntent,
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status:              agentloop.FreeStateNeedsExperiment,
			Summary:             "bounded EQ applied; energy delta stays subthreshold, user A/B arbitrates",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose,
				Attempt: 1, EvidenceRefs: []string{"obs-d1-stall-after"},
			},
			ExperimentTargetResponse: &experiment.TargetEvaluation{
				Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady,
				Summary: "同频段能量变化未达可判定材料性，交由用户 A/B 判定", EvidenceRefs: []string{"obs-d1-stall-after"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
	}
	if _, ok := s.recordFreeStateDecision(loop.ConversationID, settle); !ok {
		t.Fatal("settle turn was not retained")
	}
	final, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || final.Experiment == nil {
		t.Fatal("stored loop disappeared after the settle turn")
	}
	finalRound, err := final.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if finalRound.TargetResponse == nil || finalRound.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady {
		t.Fatalf("the settle report must land its target response after the replay turn, got %+v", finalRound.TargetResponse)
	}
}
