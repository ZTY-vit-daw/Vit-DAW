package chat

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
)

// SETTLE-DELIVER-1（2026-10-03 手测场取证，webui_murptx58 / 43 事件流）：
// 判定链本体已通（JUDGMENT-SETTLE-STALL-1），但结算后续链断在三处：
//
//   - 症状 A：seq 37 judgment.settled 之后零消息投递——结算确认文案只在
//     project_history checkpoint（"这一步已经应用好了…"落 01:32:39Z manual），
//     对话流无正式助手消息。断链点：finishJudgmentSettlement 只结算 audio
//     closure + 发 MessageKind=activity 的瞬态事件，从不写 vit 会话节点
//     （正常回合走 RecordConversationNodeForProjectWithData("vit", reply)）。
//   - 症状 B：二轮 goal_0b27… turn.completed stop_reason=project_revision_stale
//     （"observation belongs to a different project revision"，25s 即终）。实锚：
//     round-2 合同绑定新修订（probe_state contract.project_revision=3），撞的是
//     round-1 已结算 loop 账本里 revision=2 的回放观察——recordAudioClosureRound
//     的账本重水合（audio_closure_controller.go "Scheduler continuations…" 段）
//     按会话键取 loop，无 goal/终态所有权门，跨 goal 回放进新闭包。
//   - 症状 C（渲染面）：二轮消息插队显示在首轮输出上方——webui 侧
//     settleDelivery.test.ts 钉。

// settleDeliveryProject 为症状 A 建一个真实临时工程文件（会话图 vit 节点必须挂在
// 真实工程的 checkpoint 上，AppendConversationNode 无 HEAD 时会走 Checkpoint 兜底）。
func settleDeliveryProject(t *testing.T) string {
	t.Helper()
	projectPath := filepath.Join(t.TempDir(), "settle_delivery.vit")
	if err := os.WriteFile(projectPath, []byte("<project settle-delivery/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return projectPath
}

// settleDeliveryJudgmentPOST 在 parked 边界上执行判定 POST（retain），返回结算
// 后的 loop 存量。
func settleDeliveryJudgmentPOST(t *testing.T, s *Server, loop freeStateReasoningLoop) {
	t.Helper()
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"8","heard_difference":"yes","preference":"b"}`,
		loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("judgment rejected: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

// 钉 A（RED：修复前无 vit 节点）：判定结算后，结算确认必须作为正式助手消息进入
// 当前会话（conversation graph 持久化 + message_kind=assistant）——SETTLE-STALL-1
// 的"可见新回复"判据由此真正达成（刷新后仍在，webui 水合可恢复）。
func TestJudgmentSettlementReplyPersistsAsConversationMessage(t *testing.T) {
	projectPath := settleDeliveryProject(t)
	s, loop, _, _ := jsssParkedBoundaryServer(t, true)
	// 实栈形态：loop 的最近工程变更携带工程路径（prepareFreeStateAudition 同源）。
	stored, _ := s.freeStateLoop(loop.ConversationID)
	stored.LatestProjectChange = map[string]any{
		"project_path": projectPath, "project_uuid": "project-jsss1", "project_revision": "8",
	}
	s.storeFreeStateLoop(stored)
	settleDeliveryJudgmentPOST(t, s, stored)

	messages, err := history.ConversationMessages(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatalf("conversation messages unreadable: %v", err)
	}
	rows, _ := messages["conversation_messages"].([]history.ConversationMessage)
	settleNode := ""
	for _, row := range rows {
		if row.Role == "assistant" && strings.Contains(row.Content, "A/B 判定已落账并完成结算") {
			settleNode = row.NodeID
		}
	}
	if settleNode == "" {
		t.Fatalf("the settlement confirmation must persist as an assistant conversation message (rows=%d)", len(rows))
	}
	// 消息协议字段：durable / project_history / assistant（webui 水合按此分类）。
	for _, row := range rows {
		if row.NodeID == settleNode {
			if row.MessageKind != "assistant" || row.Persistence != "project_history" {
				t.Fatalf("settle node protocol=%q/%q, want assistant/project_history", row.MessageKind, row.Persistence)
			}
			if row.LogicalMessageID == "" || row.LogicalMessageID == row.NodeID {
				t.Fatal("the settle node must carry a stable logical message identity (judgment_settle:<evidence>)")
			}
		}
	}
}

// 钉 B（RED：修复前回放 stale 结算闭包）：判定结算（变更已应用、修订前进）后，
// 新 goal 的新闭包按新修订绑定；round-1 已结算 loop 账本中的旧修订观察行不得
// 回放进新闭包的回合录制——守卫语义不动（跨修订观察仍然拒收），修的是回放路径
// 的所有权门：已终态实验/他 goal 的 loop 不是本闭包回合的权威账本。
func TestSettledLoopLedgerDoesNotStaleSettleNextGoalClosure(t *testing.T) {
	s, loop, goal, _ := jsssParkedBoundaryServer(t, true)
	settleDeliveryJudgmentPOST(t, s, loop)

	// 结算后的实栈续场：新输入开新 goal/新闭包，修订已前进到 9
	// （判定应用的变更在结算前已推高内核修订；新闭包经 shadow 读到新值）。
	newGoal := s.harness.BeginGoal("bass 轨道低频怎么样")
	if newGoal.GoalID == goal.GoalID {
		t.Fatal("the new input must open a fresh goal")
	}
	entryContext := contextWithSemanticEntryDecision(map[string]any{
		"goal_id": newGoal.GoalID, "run_id": newGoal.RunID,
		"project_uuid": "project-jsss1", "project_revision": "9",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteObservation,
		Controller:  string(orchestrationcontroller.MinimalAudioClosure),
		TargetScope: semanticEntryScopeProjectContext, ControlMode: semanticEntryControlObserveOnly,
		UserAuthorization: semanticEntryAuthorizationObserve, Confidence: 0.9, Reason: "fixture observation entry",
	})
	_, newState, active, err := s.prepareAudioClosureContext(loop.ConversationID, "bass 轨道低频怎么样", entryContext)
	if err != nil || !active || newState.Terminal() {
		t.Fatalf("new goal closure: err=%v active=%v terminal=%t", err, active, newState.Terminal())
	}

	// round-1 已结算 loop 的账本残留：旧修订（8）的可用观察行（实栈 seq 17 后
	// 账本持 revision=2/3 双代行，此处以单行旧修订代表回放面）。
	settled, _ := s.freeStateLoop(loop.ConversationID)
	settled.ObservationLedger = map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"mom.static_level@8": map[string]any{
				"view_id": "mom.static_level", "status": "ready", "observation_id": "obs-r1-static",
				"project_revision": "8", "project_uuid": "project-jsss1",
			},
		},
	}
	s.storeFreeStateLoop(settled)

	// 二轮回合收口：先按实栈同序准入回合（goalrunner_chat 在回合执行前 admit），
	// 本轮真实观察在新修订（9）上，无 CCB 执行投影（mix.observe 非
	// ccb.observation_request，实栈同形）。
	if admittedState, admitted, admitErr := s.admitAudioClosureRound(newState); admitErr != nil || !admitted {
		t.Fatalf("round admission failed: admitted=%v err=%v", admitted, admitErr)
	} else {
		newState = admittedState
	}
	result := agentloop.Result{Status: agentruntime.StatusRunning}
	next, err := s.recordAudioClosureRound(newState, result, entryContext)
	if err != nil {
		t.Fatalf("round recording failed: %v", err)
	}
	if next.Terminal() && next.Settlement != nil && next.Settlement.Reason == audioclosure.StopProjectRevisionStale {
		t.Fatalf("the settled loop's ledger replay must not stale-settle the next goal's closure: %+v", next.Settlement)
	}
	// 反向钉（守卫语义不放宽）：真有跨修订观察进录时守卫仍然拒收——同函数内
	// 直接喂一条 revision=8 的 CCB 观察，必须 stale 结算（不是静默吞掉）。
	freshClosure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-stale-guard-pin", ConversationID: loop.ConversationID + "-guard",
		GoalID: newGoal.GoalID, RunID: newGoal.RunID,
		ProjectUUID: "project-jsss1", ProjectRevision: "9", OriginalIntent: "guard pin",
		Mode: audioclosure.ModeDiagnostic, Scope: audioclosure.Scope{Kind: "project", ID: "project-jsss1"},
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.audioClosures.Create(freshClosure); err != nil {
		t.Fatal(err)
	}
	admitted, _, err := (audioclosure.Driver{}).AdmitRound(freshClosure, freshClosure.Revision, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.audioClosures.Save(admitted, freshClosure.Revision); err != nil {
		t.Fatal(err)
	}
	staleObs := agentloop.Result{
		Status: agentruntime.StatusRunning,
		RecentObservation: &agentloop.RecentObservation{
			Tool: "ccb.observation_request", CommandName: "ccb_observation_request",
			Status: "ready",
			Summary: map[string]any{
				"observation_id": "obs-guard-stale", "project_revision": "8",
				"requested_views": []any{"mom.static_level"},
			},
		},
	}
	guardNext, err := s.recordAudioClosureRound(admitted, staleObs, entryContext)
	if err != nil {
		t.Fatalf("guard round recording failed: %v", err)
	}
	if !guardNext.Terminal() || guardNext.Settlement == nil || guardNext.Settlement.Reason != audioclosure.StopProjectRevisionStale {
		t.Fatalf("the cross-revision observation guard must keep settling stale: terminal=%t settlement=%+v", guardNext.Terminal(), guardNext.Settlement)
	}
}

// 钉 B 伴生（所有权门的正确面）：同一 goal 的在途 loop 账本重水合语义不变——
// scheduler continuation 场景（同 goal、实验未终态）的账本回放照旧进入录制。
func TestInFlightLoopLedgerRehydrationUnchangedForSameGoal(t *testing.T) {
	s, loop, _, closure := jsssParkedBoundaryServer(t, true)
	// 未判定：loop 在途（非终态实验），账本持本闭包修订（8）的观察行。
	inFlight, _ := s.freeStateLoop(loop.ConversationID)
	inFlight.ObservationLedger = map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"mom.static_level@8": map[string]any{
				"view_id": "mom.static_level", "status": "ready", "observation_id": "obs-r1-static",
				"project_revision": "8", "project_uuid": "project-jsss1",
			},
		},
	}
	s.storeFreeStateLoop(inFlight)

	admittedState, admitted, admitErr := s.admitAudioClosureRound(closure)
	if admitErr != nil || !admitted {
		t.Fatalf("round admission failed: admitted=%v err=%v", admitted, admitErr)
	}
	result := agentloop.Result{Status: agentruntime.StatusRunning}
	next, err := s.recordAudioClosureRound(admittedState, result, map[string]any{"goal_id": loop.GoalID})
	if err != nil {
		t.Fatalf("round recording failed: %v", err)
	}
	if next.Terminal() {
		t.Fatalf("same-goal in-flight ledger replay must not settle the closure: %+v", next.Settlement)
	}
	if !audioClosureHasObservationID(next, "obs-r1-static") {
		t.Fatal("same-goal in-flight ledger replay must book the observation (continuation rehydration unchanged)")
	}
}

// 钉 A 伴生（事件面不回退）：结算确认的 judgment.settled 事件照旧入流（body 带
// 结算报告），webui live 提升以其 payload.settlement_reply 为谓词。
func TestJudgmentSettlementEventKeepsSettlementReplyMarker(t *testing.T) {
	s, loop, _, _ := jsssParkedBoundaryServer(t, true)
	settleDeliveryJudgmentPOST(t, s, loop)
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 128)
	found := false
	for _, event := range events {
		if event.Type == "judgment.settled" && event.Body != "" && event.LogicalMessageID != "" {
			if marked, _ := event.Payload["settlement_reply"].(bool); marked {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("judgment.settled must keep carrying the settlement reply body and marker payload")
	}
}
