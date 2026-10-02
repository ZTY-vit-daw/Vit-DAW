package chat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/journal"
	"vit-daw-agent/internal/orchestration"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// JUDGMENT-SETTLE-STALL-1（2026-10-02 晚真机手测，webui_muqwy5sv / goal_9c7018）：
// A/B 判定后结算停滞。活栈取证定案（decisions/2026-10-02-user-manual-test-feedback.md §4）：
//
//   - 干预应用正常，settle 回合落了 round decision user_judgment_pending（边界已驻留，
//     stop_reason="experiment round is waiting for the human judgment boundary"），但其
//     伴随报告字段（materiality 后的 human_audition_ready target response）从未落账
//     ——与 Mac D1-SETTLE-TAIL-MAC-1 同族（settle 尾段字段停滞），本卡侧的形态差异是
//     轮决定已落而 Mac 侧停在 observing；
//   - requestAuditionJudgment 被 SETTLE-CHAIN-1 守卫拒绝 → 无 trajectory.user_judgment.
//     requested 事件 → webui 判定席从未出现（canJudge 要求 judgmentRequested）→ 用户
//     的 A/B 点击是纯回放（audition.select），judgment POST 从未发出（嫌疑①409 因此
//     从未发生；mix-tick 分支按会话身份结构性不可达——嫌疑②排除）；
//   - task 停 needs_experiment（human_judgment_required 转换在守卫之后，从未执行）；
//   - 新输入 goal_a330 失败 "conversation is already owned by minimal_audio_closure
//     controller …"——parked 驻留的所有权残留。
//
// 本文件钉住修复后的全链：边界可应答（arm）→ 判定落账（evidence+靶响应重建）→
// 结算执行（retain/rollback）→ 闭包/所有权收口 → 用户可见新回复 → 新输入干净开场。

const jsssConversation = "conversation-jsss1"

// jsssParkedBoundaryServer 构造活栈定格形态：D1-S1 干预已应用、post-action 证据在账、
// settle 回合只落了 materiality + user_judgment_pending 轮决定（无 target response）、
// 会话卡已挂（applied-boundary mount）、task 停 needs_experiment、会话闭包活跃且持有
// controller owner。withClosure=false 时省闭包（隔离纯实验面断言）。sender 非 nil 时
// 在任务契约建立前替换 harness（回滚桩需要），避免契约状态被换丢。
func jsssParkedBoundaryServer(t *testing.T, withClosure bool, sender ...harness.KernelSender) (*Server, freeStateReasoningLoop, agentruntime.Goal, audioclosure.State) {
	t.Helper()
	s := New(nil, nil, nil)
	loop := freeStateReasoningLoop{SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-jsss1",
		ConversationID: jsssConversation, GoalID: "goal-jsss1", RunID: "run-jsss1",
		Status: "awaiting_experiment", OriginalIntent: "检查一下当前工程有什么问题",
		LatestObservation: d1FreshObservationForTest("7"), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := s.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	receipt := orchestration.ActionReceipt{ActionID: "d1-action-jsss1", Status: "applied", AppliedRevision: "8", EffectivelyOnce: true, Details: map[string]any{
		"before_revision": "7", "after_revision": "8", "transaction_id": "tx-jsss1", "idempotency_key": "key-jsss1", "actual_readback_db": 3.0, "readback_verified": true,
	}}
	session := orchestration.PlanningSession{ID: "d1-session-jsss1", Status: orchestration.StatusCompleted, Execution: &orchestration.ExecutionRecord{
		ID: "execution-jsss1", IdempotencyKey: "key-jsss1", Receipts: []orchestration.ActionReceipt{receipt},
		VerificationResult: &orchestration.VerificationResult{Status: "verified", Fresh: true, PostAction: true, ObservationID: "obs-after", ObservationRevision: "8", EvidenceRefs: []string{"ccb-after"}},
	}}
	_ = s.projectD1Execution(loop, session, nil)
	loop, _ = s.freeStateLoop(loop.ConversationID)
	// The stalled settle turn: materiality landed, the round parked at
	// user_judgment_pending, the human_audition_ready target response never did.
	s.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"materiality-after"}},
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	})
	if round, err := loop.Experiment.CurrentRound(); err != nil || round.Decision != experiment.DecisionUserJudgment || round.TargetResponse != nil {
		t.Fatalf("setup: the round must park at user_judgment_pending without a target response: err=%v decision=%s target=%+v", err, round.Decision, round.TargetResponse)
	}
	loop.Status = "blocked"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.AuditionSessionID = "audition:" + loop.Experiment.ID
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": loop.AuditionSessionID, "conversation_id": loop.ConversationID, "status": "ready",
		"scope": "target", "project_revision": "8",
		"candidates": []any{
			map[string]any{"id": "candidate-a", "label": "A", "status": "ready", "source_ref": "before.wav", "preview_ref": "audio_file:before", "render_revision": "render-before"},
			map[string]any{"id": "candidate-b", "label": "B", "status": "ready", "source_ref": "after.wav", "preview_ref": "audio_file:after", "render_revision": "render-after"},
		},
	}
	s.storeFreeStateLoop(loop)

	// Canonical task: improvement_proposed → needs_experiment (the live shape).
	if len(sender) > 0 && sender[0] != nil {
		s.harness = harness.NewWithSender(sender[0], nil, nil)
		s.harness.JournalRecord(journal.Action{AgentActionID: "d1-action-jsss1", Domain: "daw", Status: journal.StatusSucceeded, Command: map[string]any{"cmd": "set_volume"}})
	}
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	if _, err := s.ensureAudioTaskContract(loop.ConversationID, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-jsss1"}, "project-jsss1", "8",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	proposal := taskstate.BoundedProposal{ProposalID: "proposal-jsss1", Summary: "bounded static eq", EvidenceRefs: []string{"before"}, RequiresExperiment: true}
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventImprovementProposed, Reason: "candidate observed", EvidenceRefs: []string{"before"}, CandidateID: "1022", Proposal: &proposal, ProjectRevision: "8"}); err != nil {
		t.Fatal(err)
	}
	if err := s.bindExperimentSemantic(&loop); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(loop)
	s.harness.SetGoalStatus(goal.GoalID, agentruntime.StatusWaitingContinue, nil)

	closure := audioclosure.State{}
	if withClosure {
		current := s.harness.RuntimeStatus(goal.GoalID)
		var err error
		closure, err = audioclosure.Start(audioclosure.StartRequest{
			ClosureID: "closure-jsss1", ConversationID: loop.ConversationID,
			TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID,
			ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State,
			TaskStateRevision: current.Task.SemanticState.Revision,
			ProjectUUID:       "project-jsss1", ProjectRevision: "8", OriginalIntent: loop.OriginalIntent,
			Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-jsss1"},
			Now: time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.audioClosures.Create(closure); err != nil {
			t.Fatal(err)
		}
		if err := s.ensureAudioClosureOwner(closure); err != nil {
			t.Fatal(err)
		}
	}
	stored, _ := s.freeStateLoop(loop.ConversationID)
	return s, stored, goal, closure
}

// 钉①（活栈缺陷本体，修复前 RED）：parked 边界必须可应答——判定请求在缺靶响应的
// 驻留轮上武装（round 绑定 + UserJudgmentRequested + trajectory.user_judgment.requested
// 事件 + task 转 human_judgment_required）。修复前该调用被 SETTLE-CHAIN-1 守卫原样
// 拒绝，webui 判定席永不出现。
func TestParkedBoundaryJudgmentRequestArmsWithoutTargetResponse(t *testing.T) {
	s, loop, goal, _ := jsssParkedBoundaryServer(t, false)
	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)

	stored, _ := s.freeStateLoop(loop.ConversationID)
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if !round.UserJudgmentRequested || round.AuditionSessionID != loop.AuditionSessionID {
		t.Fatalf("the parked boundary did not arm: requested=%v session=%q", round.UserJudgmentRequested, round.AuditionSessionID)
	}
	if current := s.harness.RuntimeStatus(goal.GoalID); current.Task == nil || current.Task.SemanticState == nil ||
		current.Task.SemanticState.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("canonical task state=%+v, want human_judgment_required", current.Task)
	}
	// The webui arms its verdict seats from this event (audition.ts reduceAuditionEvents).
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 128)
	found := false
	for _, event := range events {
		if event.Type == "trajectory.user_judgment.requested" {
			found = true
		}
	}
	if !found {
		t.Fatal("the armed boundary emitted no trajectory.user_judgment.requested event (the webui seat never renders)")
	}
}

// 钉②（判定→结算→可见新回复→所有权收口，E2E 主链）：判定 POST 落账于驻留边界，
// 靶响应按判定语义重建，结算执行（保留/回滚/模糊），闭包收口，owner 释放，
// judgment.settled 可见回复事件入流。
func TestParkedBoundaryJudgmentSettlesRepliesAndReleasesOwnership(t *testing.T) {
	for _, test := range []struct {
		name, heard, preference string
		outcome                experiment.SettlementOutcome
		decision               experiment.RoundDecision
		rollback               bool
	}{
		{name: "retain", heard: "yes", preference: "b", outcome: experiment.OutcomeImproved, decision: experiment.DecisionRetain},
		{name: "rollback", heard: "yes", preference: "a", outcome: experiment.OutcomeRolledBack, decision: experiment.DecisionRollback, rollback: true},
		{name: "ambiguous", heard: "no", preference: "unsure", outcome: experiment.OutcomeNeedsJudgment, decision: experiment.DecisionStopped},
	} {
		t.Run(test.name, func(t *testing.T) {
			var senderArg []harness.KernelSender
			if test.rollback {
				senderArg = append(senderArg, &d1UndoSender{})
			}
			s, loop, goal, closure := jsssParkedBoundaryServer(t, true, senderArg...)
			round, _ := loop.Experiment.CurrentRound()
			body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":%q,"preference":%q}`,
				loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID, test.heard, test.preference)
			recorder := httptest.NewRecorder()
			s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
			if recorder.Code != http.StatusOK {
				t.Fatalf("judgment rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
			}

			stored, _ := s.freeStateLoop(loop.ConversationID)
			settledRound, _ := stored.Experiment.CurrentRound()
			if stored.Experiment.Status != experiment.StatusSettled || stored.Experiment.Outcome != test.outcome {
				t.Fatalf("experiment status=%s outcome=%s, want settled/%s", stored.Experiment.Status, stored.Experiment.Outcome, test.outcome)
			}
			if settledRound.Decision != test.decision || len(settledRound.UserJudgmentEvidence) == 0 {
				t.Fatalf("round decision=%s evidence=%d, want %s with recorded evidence", settledRound.Decision, len(settledRound.UserJudgmentEvidence), test.decision)
			}
			if settledRound.TargetResponse == nil {
				t.Fatal("the parked-boundary judgment must reconstruct the round target response")
			}
			if current := s.harness.RuntimeStatus(goal.GoalID); current.Task == nil || current.Task.SemanticState == nil || !current.Task.SemanticState.Terminal {
				t.Fatalf("canonical task state=%+v, want terminal", current.Task)
			}
			if stored.Status != "completed" {
				t.Fatalf("loop status=%s, want completed", stored.Status)
			}
			// Ownership: the closure settles with the honest task reason and the
			// controller owner releases — the live wedge's "conversation is
			// already owned" tail.
			settledClosure, ok := s.audioClosures.Load(closure.ClosureID)
			if !ok || !settledClosure.Terminal() {
				t.Fatalf("the judged park must settle the session closure: ok=%v settlement=%+v", ok, settledClosure.Settlement)
			}
			if owner, active := s.controllerOwners.Active(loop.ConversationID); active {
				t.Fatalf("the controller owner must be released after the judgment settlement: %+v", owner)
			}
			// The visible new reply (通用确认卡语义): a stream event carrying the
			// settle report body.
			events, _ := s.agentEventsSince(loop.ConversationID, 0, 128)
			replyEvent := false
			for _, event := range events {
				if event.Type == "judgment.settled" && event.Body != "" {
					replyEvent = true
				}
			}
			if !replyEvent {
				t.Fatal("the judgment settlement emitted no visible reply event (judgment.settled)")
			}
		})
	}
}

// 钉③（活栈楔死下半场）：parked 未判定时新输入默认采纳（FS-PARK-TURNFAIL-1 语义
// 保持），闭包收口 + owner 释放，下一个 goal 以全新闭包干净开场——不重现
// "conversation is already owned by minimal_audio_closure"。
// 钉⑦（验收判据 E2E 全链新组）：判定 → 结算 → 可见新回复 → 新输入不再有 park 残留、
// 以全新 goal/闭包开场（goal/run 与 parked 轮不同源）——用户旅程「选择后拿到新回复，
// 然后正常继续对话」的服务端全链。
func TestJudgmentSettleFullChainNewInputStartsFreshGroup(t *testing.T) {
	s, loop, goal, closure := jsssParkedBoundaryServer(t, true)
	round, _ := loop.Experiment.CurrentRound()
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":"yes","preference":"b"}`,
		loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("judgment rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 128)
	reply := ""
	for _, event := range events {
		if event.Type == "judgment.settled" {
			reply = event.Body
		}
	}
	if reply == "" {
		t.Fatal("the settle reply is missing from the event stream")
	}

	// New input: no park residue remains (the judgment settled it), so the
	// adoption channel steps aside and the message opens its own goal group.
	if s.settleJudgmentParkOnUserContinuation(loop.ConversationID, "drums 轨低频怎么样") {
		t.Fatal("a settled park must not be adopted again by the next input")
	}
	newGoal := s.harness.BeginGoal("drums 轨低频怎么样")
	if newGoal.GoalID == goal.GoalID || newGoal.RunID == goal.RunID {
		t.Fatalf("the new input must open a fresh goal group: goal=%q run=%q (parked goal=%q run=%q)",
			newGoal.GoalID, newGoal.RunID, goal.GoalID, goal.RunID)
	}
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-jsss1", "project_revision": "8",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		Controller:        string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope:       semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := s.prepareAudioClosureContext(loop.ConversationID, "drums 轨低频怎么样", entryContext)
	if err != nil {
		t.Fatalf("the post-settlement input must enter cleanly (no ownership wedge): %v", err)
	}
	if !active || newState.GoalID != newGoal.GoalID || newState.ClosureID == closure.ClosureID || newState.Terminal() {
		t.Fatalf("the post-settlement input must run on its own fresh closure: active=%v goal=%q closure=%q terminal=%t",
			active, newState.GoalID, newState.ClosureID, newState.Terminal())
	}
}

func TestParkedBoundaryNewInputAdoptsAndNextGoalRunsClean(t *testing.T) {
	s, loop, goal, closure := jsssParkedBoundaryServer(t, true)
	if !s.settleJudgmentParkOnUserContinuation(loop.ConversationID, "drums 轨低频怎么样") {
		t.Fatal("the new input must adopt the parked round")
	}
	if status := s.harness.RuntimeStatus(goal.GoalID).Status; !isTerminalGoalStatus(status) {
		t.Fatalf("the parked goal must be closed after adoption: %s", status)
	}
	settled, ok := s.audioClosures.Load(closure.ClosureID)
	if !ok || !settled.Terminal() || settled.Settlement == nil || settled.Settlement.Reason != audioclosure.StopAdoptedByContinuation {
		t.Fatalf("adoption closure=%+v settled=%+v", settled, settled.Settlement)
	}
	if _, active := s.controllerOwners.Active(loop.ConversationID); active {
		t.Fatal("the controller owner must be released by the adoption settlement")
	}

	newGoal := s.harness.BeginGoal("drums 轨低频怎么样")
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-jsss1", "project_revision": "8",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		Controller:        string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope:       semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := s.prepareAudioClosureContext(loop.ConversationID, "drums 轨低频怎么样", entryContext)
	if err != nil {
		t.Fatalf("the post-adoption goal must not inherit the parked closure's ownership conflict: %v", err)
	}
	if !active || newState.Terminal() || newState.GoalID != newGoal.GoalID || newState.ClosureID == closure.ClosureID {
		t.Fatalf("the post-adoption goal must run on its own fresh closure: active=%v goal=%q closure=%q", active, newState.GoalID, newState.ClosureID)
	}
}

// 钉④（judged-but-unsettled 残留）：判定已落账但结算中辍（settle/rollback 失败或
// 结算片停滞）时，新输入按已录判定收口——不楔死、不静默重采纳、不拒新 goal。
func TestJudgedButUnsettledParkNewInputSettlesByRecordedJudgment(t *testing.T) {
	s, loop, _, closure := jsssParkedBoundaryServer(t, true)
	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	stored, _ := s.freeStateLoop(loop.ConversationID)
	currentRound, roundErr := stored.Experiment.CurrentRound()
	if roundErr != nil {
		t.Fatal(roundErr)
	}
	evidence := experiment.UserJudgmentEvidence{
		SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ID: "judgment-jsss1-residue",
		ConversationID: loop.ConversationID, TurnID: loop.Experiment.ID, RoundID: currentRound.ID, AuditionSessionID: loop.AuditionSessionID,
		CandidateARef: "candidate-a", CandidateBRef: "candidate-b",
		HeardDifference: experiment.HeardDifferenceYes, Preference: experiment.PreferenceB,
		CreatedAt:       time.Now().UTC(),
	}
	if _, err := stored.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if round, err := stored.Experiment.CurrentRound(); err != nil || len(round.UserJudgmentEvidence) == 0 {
		t.Fatalf("setup: the residue must carry recorded evidence: err=%v", err)
	}
	// The residue keeps the experiment unsettled on purpose.
	s.storeFreeStateLoop(stored)

	if !s.settleJudgmentParkOnUserContinuation(loop.ConversationID, "换个话题继续") {
		t.Fatal("the new input must settle the judged-but-unsettled park by its recorded judgment")
	}
	final, _ := s.freeStateLoop(loop.ConversationID)
	if final.Experiment.Status != experiment.StatusSettled || final.Experiment.Outcome != experiment.OutcomeImproved {
		t.Fatalf("residue settlement status=%s outcome=%s, want settled/improved", final.Experiment.Status, final.Experiment.Outcome)
	}
	if settled, ok := s.audioClosures.Load(closure.ClosureID); !ok || !settled.Terminal() {
		t.Fatalf("the residue settlement must close the closure: ok=%v settlement=%+v", ok, settled.Settlement)
	}
	if _, active := s.controllerOwners.Active(loop.ConversationID); active {
		t.Fatal("the residue settlement must release the controller owner")
	}
}

// 钉⑤（restore 残留形态）：注册表仍持有一个已在 store 终态化的闭包 owner（结算后
// 注册表释放丢失/被旧快照恢复覆写）——新闭包准入必须退役陈旧 owner 而不是把新
// goal 打成 "conversation is already owned"。两个活闭包并存仍 fail-closed。
func TestStaleTerminalOwnerRetiresInsteadOfWedgingNewClosure(t *testing.T) {
	s, loop, _, closure := jsssParkedBoundaryServer(t, true)
	// The parked closure settles durably (the judgment path or the finished-task
	// guard), but the registry release is lost — the restore-residue shape.
	settled, err := (audioclosure.Driver{}).Settle(closure, closure.Revision, audioclosure.StopTaskSettled,
		"fixture: settled without registry release", false, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.audioClosures.Save(settled, closure.Revision); err != nil {
		t.Fatal(err)
	}
	if _, active := s.controllerOwners.Active(loop.ConversationID); !active {
		t.Fatal("setup: the stale owner must still be active")
	}

	fresh := audioclosure.State{ClosureID: "closure-jsss1-fresh", ConversationID: loop.ConversationID}
	if err := s.ensureAudioClosureOwner(fresh); err != nil {
		t.Fatalf("a stale owner over a terminal closure must be retired, not wedge the intake: %v", err)
	}
	owner, active := s.controllerOwners.Active(loop.ConversationID)
	if !active || owner.ControllerID != fresh.ClosureID {
		t.Fatalf("the fresh closure must own the conversation: active=%v owner=%+v", active, owner)
	}
}

// 钉⑤补充：stale owner 退役不得放宽成无条件放行——两个活闭包并存仍 fail-closed。
func TestSecondLiveClosureStillFailsClosedAgainstLiveOwner(t *testing.T) {
	s, _, _, _ := jsssParkedBoundaryServer(t, true)
	if err := s.ensureAudioClosureOwner(audioclosure.State{ClosureID: "closure-live-b", ConversationID: jsssConversation}); err == nil {
		t.Fatal("a second live closure must keep failing closed against a live owner")
	}
}

// 钉⑥（judgment.settled 回复文案诚实边界）：回复陈述执行了什么与审计身份，不虚报
// 满意、不出现内部代号。
func TestJudgmentSettlementReplyWording(t *testing.T) {
	loop := freeStateReasoningLoop{GoalID: "g", RunID: "r", Experiment: &experiment.Turn{ID: "turn-1", Outcome: experiment.OutcomeImproved, Settlement: "user preferred B; treatment candidate retained"}}
	reply := judgmentSettlementReply(loop, experiment.UserJudgmentEvidence{ID: "judgment-x", RoundID: "round-1"})
	for _, want := range []string{"已按你的判定保留改动后状态", "judgment-x", "round-1"} {
		if !bytes.Contains([]byte(reply), []byte(want)) {
			t.Fatalf("reply=%q must contain %q", reply, want)
		}
	}
	rolled := freeStateReasoningLoop{GoalID: "g", RunID: "r", Experiment: &experiment.Turn{ID: "turn-1", Outcome: experiment.OutcomeRolledBack}}
	if reply := judgmentSettlementReply(rolled, experiment.UserJudgmentEvidence{ID: "j", RoundID: "r"}); !bytes.Contains([]byte(reply), []byte("回滚")) {
		t.Fatalf("rolled-back reply=%q must state the rollback", reply)
	}
}
