package chat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// FS-PARK-TURNFAIL-1（2026-09-30 用户裁定）：park 期间新用户输入 = 对待裁决段
// 默认采纳。三钉语义：
//  1. 采纳结算如实标注 adopted_by_continuation——轮上不写 UserJudgmentEvidence、
//     TargetResponse 不被改写为 human_confirmed（证据链诚实边界）；
//  2. parked goal 关闭（waiting_continue→completed），beginChatGoal 为新消息开
//     全新 goal/run/turn（消除 turn id 复用翻转）；
//  3. 裸"继续"、命令、已记录人耳判断的轮不触发默认采纳。

func TestJudgmentParkNewUserInputAdoptsByContinuation(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	if !judgmentParkPendingRound(loop) {
		t.Fatal("setup: loop must park a pending judgment round")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if round.TargetResponse == nil || round.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady {
		t.Fatalf("setup: round must carry a human_audition_ready target response: %+v", round.TargetResponse)
	}
	if !s.settleJudgmentParkOnUserContinuation(loop.ConversationID, "低频听起来有点浑，帮我看下 Bass 轨") {
		t.Fatal("new user input during a judgment park must settle the parked round by adoption")
	}
	settled, ok := s.freeStateLoop(loop.ConversationID)
	if !ok {
		t.Fatal("settled loop missing")
	}
	if settled.Status != "completed" {
		t.Fatalf("loop status = %s, want completed", settled.Status)
	}
	if settled.Experiment == nil || settled.Experiment.Status != experiment.StatusSettled {
		t.Fatalf("experiment status = %+v, want settled", settled.Experiment)
	}
	if settled.Experiment.Outcome != experiment.OutcomeAdoptedByContinuation {
		t.Fatalf("settlement outcome = %q, want adopted_by_continuation", settled.Experiment.Outcome)
	}
	finalRound, err := settled.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if finalRound.Decision != experiment.DecisionAdoptedByContinuation {
		t.Fatalf("round decision = %q, want adopted_by_continuation", finalRound.Decision)
	}
	// 诚实边界：默认采纳绝不携带人耳判断证据，也不得改写 target response。
	if len(finalRound.UserJudgmentEvidence) != 0 {
		t.Fatalf("adoption by continuation must not record user judgment evidence: %+v", finalRound.UserJudgmentEvidence)
	}
	if finalRound.TargetResponse.Outcome != trajectory.EvaluationHumanAuditionReady {
		t.Fatalf("target response outcome rewritten to %q; adoption must keep human_audition_ready", finalRound.TargetResponse.Outcome)
	}
	if finalRound.TargetResponse.Outcome == trajectory.EvaluationHumanConfirmed {
		t.Fatal("adoption by continuation must never mark the round human_confirmed")
	}
	if adoption := firstStringFromMap(settled.AuditionSessionSnapshot, "adoption_status"); adoption != "adopted_by_continuation" {
		t.Fatalf("audition snapshot adoption_status = %q, want adopted_by_continuation", adoption)
	}
	// 任务语义收口：human_judgment_required → settled，pending interaction 清除。
	goal := s.harness.RuntimeStatus(loop.GoalID)
	if goal.Task == nil || goal.Task.SemanticState == nil {
		t.Fatal("task semantic state missing after adoption settle")
	}
	if goal.Task.SemanticState.State != taskstate.StateSettled {
		t.Fatalf("task state = %s, want settled", goal.Task.SemanticState.State)
	}
	if goal.Task.SemanticState.PendingInteraction != nil {
		t.Fatalf("pending interaction must clear on adoption settle: %+v", goal.Task.SemanticState.PendingInteraction)
	}
	// parked goal 关闭：新消息不再续用（turn id 复用翻转的根）。
	if goal.Status != agentruntime.StatusCompleted {
		t.Fatalf("parked goal status = %s, want completed", goal.Status)
	}
}

// TestAdoptionSettlementD1ReceiptHonesty 钉 receipt 级诚实边界：D1-S1 实验按
// 默认采纳收口后，d1 receipt 无 validation_error，disposition/human_ab 如实
// 呈现 adopted_by_continuation/skipped_by_continuation，绝不 claim
// human_confirmed 或 net_outcome=improved。
func TestAdoptionSettlementD1ReceiptHonesty(t *testing.T) {
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-adoption-receipt", ConversationID: "conversation-adoption-receipt", GoalID: "goal", RunID: "run", OriginalIntent: "compare before and after", LatestProjectChange: map[string]any{"project_path": auditionProjectPathForTest(t), "project_revision": "rev-7"}, LatestObservation: d1FreshObservationForTest("rev-6"), CreatedAt: now, UpdatedAt: now}
	admission, err := freeStateExperimentAdmission(loop, experimentTestProposal())
	if err != nil {
		t.Fatal(err)
	}
	// 注册域（static_eq/track_gain）自带 D1-S1 不变量——不覆写 legacy_mix。
	turn, err := experiment.NewTurn(experiment.Identity{ConversationID: loop.ConversationID, GoalID: loop.GoalID, RunID: loop.RunID, TurnID: "turn-adoption"}, loop.OriginalIntent, admission, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = turn.StartRound([]string{"track.timbre_frequency"}, "checkpoint-7", "rev-6", now); err != nil {
		t.Fatal(err)
	}
	before := experiment.Observation{ID: "before", RequestedViewIDs: []string{"track.timbre_frequency"}, ExecutedViewIDs: []string{"track.timbre_frequency"}, ViewSetMatches: true, Fresh: true, ProjectRevision: "rev-6", EvidenceRefs: []string{"before"}}
	if _, err = turn.RecordObservation(before, false, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.ApplyIntervention(experiment.Intervention{ID: "action-7", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true, Receipt: map[string]any{"status": "ok", "before_revision": "6", "after_revision": "rev-7", "readback_verified": true}}, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialitySubthreshold, Evaluation: trajectory.EvaluationInsufficientDose, Attempt: 1, EvidenceRefs: []string{"material"}}, now); err != nil {
		t.Fatal(err)
	}
	after := before
	after.ID = "after"
	after.PostAction = true
	after.ProjectRevision = "rev-7"
	after.EvidenceRefs = []string{"after"}
	if _, err = turn.RecordObservation(after, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, now); err != nil {
		t.Fatal(err)
	}
	// park 形态：轮决策 user_judgment_pending（judgment requested 未落证据）。
	if _, err = turn.DecideRound(experiment.DecisionUserJudgment, "awaiting human A/B judgment", now); err != nil {
		t.Fatal(err)
	}
	round, err := turn.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	round.UserJudgmentRequested = true
	round.AuditionSessionID = "audition:turn-adoption:" + round.ID
	turn.Rounds[len(turn.Rounds)-1] = round
	// 默认采纳收口。
	if _, err = turn.DecideRound(experiment.DecisionAdoptedByContinuation, judgmentParkAdoptionSummary, now); err != nil {
		t.Fatal(err)
	}
	if _, err = turn.Settle(experiment.OutcomeAdoptedByContinuation, judgmentParkAdoptionSummary, now); err != nil {
		t.Fatal(err)
	}
	loop.Experiment = &turn
	loop.D1State = map[string]any{
		"before_render": map[string]any{"status": "ready", "render_revision": "before:7"},
		"after_render":  map[string]any{"status": "ready", "render_revision": "after:8"},
	}
	syncD1Receipt(&loop)
	receipt := loop.D1Receipt
	if firstStringFromMap(receipt, "validation_error") != "" {
		t.Fatalf("adoption receipt carries a validation error: %s", firstStringFromMap(receipt, "validation_error"))
	}
	if got := firstStringFromMap(receipt, "disposition"); got != string(experiment.DispositionAdoptedByContinuation) {
		t.Fatalf("receipt disposition = %q, want adopted_by_continuation", got)
	}
	layers := firstMapFromAny(receipt["layers"])
	humanAB := firstMapFromAny(layers["human_ab"])
	if got := firstStringFromMap(humanAB, "status"); got != "skipped_by_continuation" {
		t.Fatalf("receipt human_ab status = %q, want skipped_by_continuation", got)
	}
	if receipt["human_confirmed"] == true || receipt["settled"] != true {
		t.Fatalf("receipt honesty flags wrong: human_confirmed=%v settled=%v", receipt["human_confirmed"], receipt["settled"])
	}
	netOutcome := firstMapFromAny(layers["net_outcome"])
	if got := firstStringFromMap(netOutcome, "status"); got == "improved" {
		t.Fatal("adoption receipt must not claim net_outcome=improved without a decided human A/B")
	}
}

func TestJudgmentParkContinuationSkipsBareContinueAndCommands(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	for _, message := range []string{"继续", "/goal 再跑一轮"} {
		if s.settleJudgmentParkOnUserContinuation(loop.ConversationID, message) {
			t.Fatalf("message %q must not trigger adoption", message)
		}
	}
	after, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || after.Status != "blocked" {
		t.Fatalf("loop must stay parked: %+v", after.Status)
	}
	if after.Experiment == nil || after.Experiment.Status != experiment.StatusRunning {
		t.Fatalf("experiment must stay running: %+v", after.Experiment)
	}
}

func TestJudgmentParkRecordedJudgmentNotOverriddenByContinuation(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	// 显式人耳判断落账后（等待 settle 的边界态），新输入不得改写为默认采纳：
	// 结算仍归 judgment POST 通道。
	recordJudgmentViaPOST(t, s, loop)
	if s.settleJudgmentParkOnUserContinuation(loop.ConversationID, "新话题") {
		t.Fatal("continuation must not adopt a round that already recorded a user judgment")
	}
}

// recordJudgmentViaPOST lands an explicit user judgment through the production
// POST channel (the fixture parks in the mid-write window where the bind path
// serves the request itself).
func recordJudgmentViaPOST(t *testing.T, s *Server, loop freeStateReasoningLoop) {
	t.Helper()
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"yes","preference":"b","reason_tags":["更自然"]}`,
		loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("judgment POST: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestJudgmentParkTerminalFallbackPreservedAsBoundary(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	for _, stopReason := range []string{
		agentloop.FreeStateTerminalFallbackStopReason,
		agentloop.FreeStateTerminalGateRejectedStopReason,
	} {
		resp := ChatResponse{
			ConversationID: loop.ConversationID,
			GoalID:         loop.GoalID,
			RunID:          loop.RunID,
			GoalStatus:     string(agentruntime.StatusFailed),
			StopReason:     stopReason,
			Error:          "terminal turn produced no admissible final decision after one strengthened retry",
			Reply:          "执行失败。",
		}
		if !s.preserveJudgmentParkOnTerminalFallback(loop.ConversationID, &resp, agentloop.Result{Status: agentruntime.StatusFailed, StopReason: stopReason}) {
			t.Fatalf("terminal fallback %s must be preserved as the judgment park boundary", stopReason)
		}
		if resp.Error != "" || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
			t.Fatalf("response must not project turn.failed: %+v", resp)
		}
		if resp.StopReason != judgmentParkPreservedStopReason {
			t.Fatalf("stop reason = %q, want %s", resp.StopReason, judgmentParkPreservedStopReason)
		}
		if resp.Reply != judgmentParkPreservedReply {
			t.Fatalf("reply must be the frozen boundary wording: %q", resp.Reply)
		}
		if v := resp.WorkflowData["judgment_park_preserved"]; v != true {
			t.Fatalf("workflow data marker missing: %+v", resp.WorkflowData)
		}
	}
	// park 不吸收其它失败形态：普通执行失败仍按失败投影。
	resp := ChatResponse{GoalStatus: string(agentruntime.StatusFailed), Error: "kernel died"}
	if s.preserveJudgmentParkOnTerminalFallback(loop.ConversationID, &resp, agentloop.Result{Status: agentruntime.StatusFailed, StopReason: "kernel_error"}) {
		t.Fatal("non-fallback failures must keep the failure projection")
	}
	// 已记录人耳判断的 park：terminal fallback 不再保留（显式判断通道自治）。
	recordJudgmentViaPOST(t, s, loop)
	preserved := ChatResponse{GoalStatus: string(agentruntime.StatusFailed), Error: "x"}
	if s.preserveJudgmentParkOnTerminalFallback(loop.ConversationID, &preserved, agentloop.Result{Status: agentruntime.StatusFailed, StopReason: agentloop.FreeStateTerminalFallbackStopReason}) {
		t.Fatal("a park with recorded judgment evidence must not absorb the fallback failure")
	}
}
