package chat

// UX-AB-AUDITION-IMPL-1（2026-10）语义动态 A/B 对照试听接线测试族，与
// mix_tick_audition_test.go 的 B12-2 四钉同构：
//  ① before 渲染严格先于 mutation，成功时双事件（audition.ready +
//     trajectory.user_judgment.requested）同轮在场，turn_id 为本通道原生域
//  ② 渲染/准备失败一律 fail-open（mutation 照跑、卡片降级、响应写降级行）
//  ③ mutation 失败原样上抛（settlement fail-closed 同路径），无会话无事件
//  ④ 身份族与实验域/mix-tick 域字面隔离，同 ticket 重执行幂等
//  ⑤ 判定走第三身份通道，实验契约零接触
//  ⑥ 身份错配 409
//  ⑦ 盲态抽签每会话一次，候选 label 恒 A/B

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/trajectory"
)

func semanticDynamicAuditionPlanForTest(t *testing.T, render func(context.Context, string, string, *semanticDynamicAuditionPlan) (map[string]any, error)) (*Server, *semanticDynamicAuditionPlan) {
	t.Helper()
	s := testContinuationServer()
	s.semanticDynamicAuditionRenderOverride = render
	s.auditionKernel = &mixTickAuditionKernelStub{}
	return s, &semanticDynamicAuditionPlan{
		ConversationID: "conversation-sda", GoalID: "goal-sda", RunID: "run-sda",
		TicketID: "ticket-sda", Family: processorintent.FamilyReverb, ProcessorType: "reverb",
		TrackID: "1007", PluginID: "reverb-1",
		BeforeRevision: "rev-9",
		AfterRevision:  func() string { return "rev-10" },
	}
}

// 钉①：before 相必须在 mutation 之前解析，after 相在其之后；成功挂卡时
// audition.ready 与判定请求事件同轮在场，session.turn_id 是 dynamic 原生域，
// 执行回复拼上 A/B 入口行。
func TestSemanticDynamicAuditionBracketRendersBeforeTheMutation(t *testing.T) {
	var order []string
	s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		order = append(order, "render:"+phase+":"+revision)
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	})

	bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error {
		order = append(order, "mutate")
		return nil
	})
	if err != nil {
		t.Fatalf("bracket must not fail on a clean pair: %v", err)
	}
	want := []string{"render:before:rev-9", "mutate", "render:after:rev-10"}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Fatalf("时序必须是 before 渲染 -> mutation -> after 渲染:\n got %v\nwant %v", order, want)
	}
	if bracket.Record == nil || bracket.Degraded != "" {
		t.Fatalf("a clean pair must mount the A/B session: %+v", bracket)
	}
	record := bracket.Record
	if record.SessionID != "audition:dynamic:ticket-sda" || record.TurnID != "dynamic_turn:ticket-sda" || record.RoundID != "dynamic_round:ticket-sda" {
		t.Fatalf("the mounted session must carry the dynamic identity the WebUI echoes back: %+v", record)
	}
	// webui 硬契约：判定 POST 发的是 session.turn_id 派生的 experimentTurnID，
	// 故会话快照必须原生携带本通道 turn 域。
	if firstStringFromMap(record.Session, "turn_id") != record.TurnID || firstStringFromMap(record.Session, "round_id") != record.RoundID {
		t.Fatalf("session snapshot turn/round domain drifted: %+v", record.Session)
	}
	if record.ProjectRevision != "rev-10" {
		t.Fatalf("the session must be bound to the post-write revision: %q", record.ProjectRevision)
	}

	events, _ := s.agentEventsSince("conversation-sda", 0, 64)
	sawReady := false
	for _, event := range events {
		if event.Type == "audition.ready" {
			sawReady = true
		}
	}
	if !sawReady {
		t.Fatalf("mount must emit audition.ready in the same round: %+v", events)
	}
	// 判定按钮的唯一使能位：canJudge 只由 trajectory.user_judgment.requested 置位，
	// 没有这条事件卡片只能播不能判。
	sawJudgmentRequest := false
	for _, event := range events {
		if event.Type != string(trajectory.EventUserJudgmentRequested) {
			continue
		}
		sawJudgmentRequest = true
		if event.Payload["turn_id"] != record.TurnID {
			t.Fatalf("the judgment node must carry the dynamic turn domain: %v", event.Payload["turn_id"])
		}
		details, _ := event.Payload["details"].(map[string]any)
		if firstStringFromMap(details, "audition_session_id") != record.SessionID {
			t.Fatalf("the judgment node must name the session the POST echoes back: %+v", details)
		}
		if firstStringFromMap(details, "semantic_dynamic") != "ticket-sda" {
			t.Fatalf("the judgment node must mark the semantic_dynamic channel: %+v", details)
		}
	}
	if !sawJudgmentRequest {
		t.Fatalf("without the judgment-requested event the A/B card renders but is unanswerable: %+v", events)
	}
	if len(s.auditionKernel.(*mixTickAuditionKernelStub).requests) != 1 {
		t.Fatalf("exactly one kernel session is prepared per pair: %+v", s.auditionKernel.(*mixTickAuditionKernelStub).requests)
	}

	// 执行回复拼装：成功行接在既有 executed 文案之后，原文不动。
	reply := appendSemanticDynamicAuditionClause(semanticDynamicExecutedReply, bracket)
	if !strings.HasPrefix(reply, semanticDynamicExecutedReply) || !strings.Contains(reply, "A/B 对照试听已就绪") {
		t.Fatalf("the executed reply must gain the mounted entry clause: %q", reply)
	}
	if stamp := semanticDynamicAuditionProtocolStamp(bracket); stamp["mounted"] != true || stamp["audition_session_id"] != record.SessionID {
		t.Fatalf("protocol stamp must carry the session identity: %+v", stamp)
	}
}

// 钉②：渲染/准备失败一律 fail-open——mutation 照跑、卡片降级、响应写降级行，
// 不承诺不存在的入口。
func TestSemanticDynamicAuditionBracketFailsOpen(t *testing.T) {
	t.Run("before render fails", func(t *testing.T) {
		var order []string
		s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
			order = append(order, "render:"+phase)
			if phase == semanticDynamicAuditionPhaseBefore {
				return nil, errors.New("kernel render unavailable")
			}
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		})
		bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error {
			order = append(order, "mutate")
			return nil
		})
		if err != nil {
			t.Fatalf("a failed before render must not abort the confirmed mutation: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "before_render_failed" {
			t.Fatalf("bracket must degrade instead of mounting a half pair: %+v", bracket)
		}
		if !strings.Contains(strings.Join(order, "|"), "render:before|mutate") {
			t.Fatalf("the mutation must still run after a failed before render: %v", order)
		}
		if strings.Contains(strings.Join(order, "|"), "render:after") {
			t.Fatalf("a failed before render leaves nothing to compare against: %v", order)
		}
		assertNoMountedCardOrEvents(t, s, "before_render_failed")
	})

	t.Run("after render fails", func(t *testing.T) {
		s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
			if phase == semanticDynamicAuditionPhaseAfter {
				return nil, errors.New("after render produced no valid WAV")
			}
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		})
		bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error { return nil })
		if err != nil {
			t.Fatalf("fail-open: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "after_render_failed" {
			t.Fatalf("bracket must degrade: %+v", bracket)
		}
		assertNoMountedCardOrEvents(t, s, "after_render_failed")
	})

	t.Run("audition prepare fails", func(t *testing.T) {
		var order []string
		s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
			order = append(order, "render:"+phase)
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		})
		s.auditionKernel = &mixTickAuditionKernelStub{err: errors.New("preview unavailable")}
		bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error {
			order = append(order, "mutate")
			return nil
		})
		if err != nil {
			t.Fatalf("fail-open: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "audition_prepare_failed" {
			t.Fatalf("bracket must degrade: %+v", bracket)
		}
		if !strings.Contains(strings.Join(order, "|"), "mutate") {
			t.Fatalf("a failed prepare must not undo the mutation: %v", order)
		}
		assertNoMountedCardOrEvents(t, s, "audition_prepare_failed")
	})

	// 降级行如实写原因，不承诺试听入口。
	degraded := appendSemanticDynamicAuditionClause(semanticDynamicExecutedReply, semanticDynamicAuditionBracket{Degraded: "after_render_failed"})
	if !strings.Contains(degraded, "没能生成") || !strings.Contains(degraded, "after_render_failed") {
		t.Fatalf("the degraded clause must state the reason: %q", degraded)
	}
	if strings.Contains(degraded, "已就绪") {
		t.Fatalf("a degraded run must not promise a card it failed to build: %q", degraded)
	}
	if plain := appendSemanticDynamicAuditionClause(semanticDynamicExecutedReply, semanticDynamicAuditionBracket{}); plain != semanticDynamicExecutedReply {
		t.Fatalf("an empty bracket must leave the reply byte-for-byte: %q", plain)
	}
}

func assertNoMountedCardOrEvents(t *testing.T, s *Server, reason string) {
	t.Helper()
	if _, ok := s.semanticDynamicAuditionFor("conversation-sda"); ok {
		t.Fatalf("a degraded %s run must not leave an answerable card", reason)
	}
	events, _ := s.agentEventsSince("conversation-sda", 0, 64)
	for _, event := range events {
		if event.Type == "audition.ready" || event.Type == string(trajectory.EventUserJudgmentRequested) {
			t.Fatalf("a degraded %s run must not raise %s: %+v", reason, event.Type, events)
		}
	}
}

// 钉③：mutation 失败原样上抛（settlement fail-closed 的错误也走同一路径），
// 无会话、无事件，调用方保持既有失败契约。
func TestSemanticDynamicAuditionMutationFailureSkipsMount(t *testing.T) {
	settlementFailure := errors.New("settlement before snapshot unavailable: kernel state channel down")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"controller failure", errors.New("typed_controller_failed: atomic execution failed")},
		{"settlement fail-closed failure", settlementFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var order []string
			s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
				order = append(order, "render:"+phase)
				return mixTickAuditionReadyRowForTest(t, phase, revision), nil
			})
			bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error {
				order = append(order, "mutate")
				return tc.err
			})
			if !errors.Is(err, tc.err) {
				t.Fatalf("the mutation error must reach the caller verbatim: %v", err)
			}
			if bracket.Record != nil || bracket.Degraded != "" {
				t.Fatalf("no change means no A/B card and no degraded promise: %+v", bracket)
			}
			if strings.Contains(strings.Join(order, "|"), "render:after") {
				t.Fatalf("a mutation that did not land has no after state to render: %v", order)
			}
			assertNoMountedCardOrEvents(t, s, "mutation-failed")
		})
	}
}

// 钉④：身份族与实验域（audition:<expID>:<roundID>）和 mix-tick 域
// （audition:mix_tick:*）字面互不冲突；同 ticket 重执行同 sessionID、map 单记录。
func TestSemanticDynamicAuditionSessionNamingIsolatedAndIdempotent(t *testing.T) {
	conversation, ticket := "conversation-sda", "ticket-sda"
	session := semanticDynamicAuditionSessionID(conversation, ticket)
	turn := semanticDynamicAuditionTurnID(conversation, ticket)
	round := semanticDynamicAuditionRoundID(ticket, "1007")
	if !strings.HasPrefix(session, "audition:dynamic:"+ticket) ||
		!strings.HasPrefix(turn, "dynamic_turn:"+ticket) ||
		!strings.HasPrefix(round, "dynamic_round:"+ticket) {
		t.Fatalf("identity family drifted: %s / %s / %s", session, turn, round)
	}
	if strings.Contains(session, "mix_tick") || strings.Contains(turn, "mix_tick") || strings.Contains(round, "mix_tick") {
		t.Fatalf("dynamic identity family leaked the mix-tick domain: %s / %s / %s", session, turn, round)
	}
	if session == mixTickAuditionSessionID(conversation, ticket) {
		t.Fatalf("dynamic and mix-tick session ids must be literally distinct: %s", session)
	}
	// 实验域形态是 audition:<expID>:<roundID>（expID/roundID 由实验侧生成），
	// 与 audition:dynamic:<ticketID> 前缀字面互斥。
	if strings.HasPrefix(session, "audition:mix_tick:") || session == "audition:exp-1:round-1" {
		t.Fatalf("dynamic session id collided with an experiment/mix-tick domain form: %s", session)
	}
	if turn == mixTickAuditionTurnID(conversation, ticket) || round == mixTickAuditionRoundID(ticket, "1007") {
		t.Fatalf("dynamic turn/round ids must be literally distinct from the mix-tick family: %s / %s", turn, round)
	}

	// 幂等：同 ticket 重执行 → 同 sessionID，map 单记录，卡片仍可应答。
	s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	})
	first, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error { return nil })
	if err != nil || first.Record == nil {
		t.Fatalf("first execution must mount: err=%v bracket=%+v", err, first)
	}
	second, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error { return nil })
	if err != nil || second.Record == nil {
		t.Fatalf("re-execution must mount: err=%v bracket=%+v", err, second)
	}
	if first.Record.SessionID != second.Record.SessionID {
		t.Fatalf("the same ticket must reseat the same session id: %s vs %s", first.Record.SessionID, second.Record.SessionID)
	}
	s.mu.Lock()
	stored := len(s.semanticDynamicAuditions)
	s.mu.Unlock()
	if stored != 1 {
		t.Fatalf("the conversation must hold exactly one record, got %d", stored)
	}
	if _, ok := s.semanticDynamicAuditionFor(conversation); !ok {
		t.Fatal("the re-seated card must stay answerable")
	}
}

func semanticDynamicAuditionRecordForTest() *semanticDynamicAuditionRecord {
	session := map[string]any{
		"session_id": "audition:dynamic:ticket-sda", "conversation_id": "conversation-audition",
		"status": "ready", "turn_id": "dynamic_turn:ticket-sda", "round_id": "dynamic_round:ticket-sda",
		"project_revision": "rev-10", "scope": "target",
		"candidates": []any{
			map[string]any{"id": auditionCandidateA, "label": "A", "status": "ready", "source_ref": "a.wav", "preview_ref": "preview:a", "checkpoint_ref": "k-a", "commit_id": "c-a", "render_revision": "r-a"},
			map[string]any{"id": auditionCandidateB, "label": "B", "status": "ready", "source_ref": "b.wav", "preview_ref": "preview:b", "checkpoint_ref": "k-b", "commit_id": "c-b", "render_revision": "r-b"},
		},
	}
	return &semanticDynamicAuditionRecord{
		SchemaVersion: semanticDynamicAuditionSchemaVersion, ConversationID: "conversation-audition",
		GoalID: "goal", RunID: "run",
		SessionID: "audition:dynamic:ticket-sda", TurnID: "dynamic_turn:ticket-sda", RoundID: "dynamic_round:ticket-sda",
		Status: semanticDynamicAuditionStatusReady, TicketID: "ticket-sda",
		Family: processorintent.FamilyReverb, ProcessorType: "reverb", TrackID: "1007", PluginID: "reverb-1",
		ProjectRevision: "rev-10", JudgmentPending: true,
		Session: session,
	}
}

// 钉⑤：有 dynamic record 时判定走第三通道——mix-tick/自由态支路未被触达、
// 落账 user_judgment、发判定事件、loop.Experiment 零变化（实验契约不动）。
func TestSemanticDynamicAuditionJudgmentRoutesThirdChannel(t *testing.T) {
	server := New(nil, nil, nil)
	loop := auditionReadyLoop(t)
	server.storeFreeStateLoop(loop)
	record := semanticDynamicAuditionRecordForTest()
	server.storeSemanticDynamicAudition(record)

	before, _ := server.freeStateLoop(loop.ConversationID)
	beforeTaskState := before.Experiment.TaskState
	beforeStatus := before.Experiment.Status
	beforeEvidence := 0
	for _, round := range before.Experiment.Rounds {
		beforeEvidence += len(round.UserJudgmentEvidence)
	}

	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":%q,"heard_difference":"yes","preference":"b"}`,
		record.ConversationID, record.TurnID, record.RoundID, record.SessionID, record.ProjectRevision)
	recorder := httptest.NewRecorder()
	server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	outcome, _ := response["semantic_dynamic_audition"].(map[string]any)
	if outcome == nil {
		t.Fatalf("the judgment must settle at the semantic-dynamic channel, got: %s", recorder.Body.String())
	}
	if outcome["decision"] != mixTickAuditionDecisionRetain || outcome["status"] != "retained" {
		t.Fatalf("选 B = 保留: %+v", outcome)
	}
	if strings.TrimSpace(fmt.Sprint(outcome["judgment_evidence_id"])) == "" {
		t.Fatalf("the settlement must book a user_judgment evidence id: %+v", outcome)
	}

	// 实验契约零变化：自由态 loop 的实验状态、回合证据计数原样。
	after, _ := server.freeStateLoop(loop.ConversationID)
	afterEvidence := 0
	for _, round := range after.Experiment.Rounds {
		afterEvidence += len(round.UserJudgmentEvidence)
	}
	if after.Experiment.TaskState != beforeTaskState || after.Experiment.Status != beforeStatus || afterEvidence != beforeEvidence {
		t.Fatalf("loop.Experiment must be untouched: task_state %q->%q status %q->%q evidence %d->%d",
			beforeTaskState, after.Experiment.TaskState, beforeStatus, after.Experiment.Status, beforeEvidence, afterEvidence)
	}

	// 判定事件与落账轨迹节点在场；卡片退役（不可二次应答）。
	sawJudgmentEvent := false
	sawRecorded := false
	events, _ := server.agentEventsSince(record.ConversationID, 0, 64)
	for _, event := range events {
		if event.Type == "semantic_dynamic.audition.judgment" {
			sawJudgmentEvent = true
		}
		if event.Type == string(trajectory.EventUserJudgmentRecorded) {
			sawRecorded = true
			if event.Payload["turn_id"] != record.TurnID {
				t.Fatalf("the recorded node must carry the dynamic turn domain: %v", event.Payload["turn_id"])
			}
			details, _ := event.Payload["details"].(map[string]any)
			if firstStringFromMap(details, "audition_session_id") != record.SessionID {
				t.Fatalf("the recorded node must name the session: %+v", details)
			}
		}
	}
	if !sawJudgmentEvent || !sawRecorded {
		t.Fatalf("the settlement must raise both the judgment event and the recorded trajectory node: %+v", events)
	}
	if _, ok := server.semanticDynamicAuditionFor(record.ConversationID); ok {
		t.Fatal("an answered card must be retired from the conversation")
	}
}

// 钉⑥：身份错配必须拒绝而不是猜测——错 turn/round/session/revision 一律 409，
// 卡片保持可应答。
func TestSemanticDynamicAuditionJudgmentRejectsIdentityMismatch(t *testing.T) {
	server := New(nil, nil, nil)
	record := semanticDynamicAuditionRecordForTest()
	server.storeSemanticDynamicAudition(record)

	for _, tc := range []struct{ name, turnID, roundID, sessionID, revision string }{
		{"turn mismatch", "dynamic_turn:other", record.RoundID, record.SessionID, record.ProjectRevision},
		{"round mismatch", record.TurnID, "dynamic_round:other", record.SessionID, record.ProjectRevision},
		{"session mismatch", record.TurnID, record.RoundID, "audition:dynamic:other", record.ProjectRevision},
		{"revision mismatch", record.TurnID, record.RoundID, record.SessionID, "rev-99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":%q,"heard_difference":"yes","preference":"b"}`,
				record.ConversationID, tc.turnID, tc.roundID, tc.sessionID, tc.revision)
			recorder := httptest.NewRecorder()
			server.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", bytes.NewBufferString(body)))
			if recorder.Code != http.StatusConflict {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
	if _, ok := server.semanticDynamicAuditionFor(record.ConversationID); !ok {
		t.Fatal("a refused judgment must leave the card answerable")
	}
}

// 钉⑦：盲态开时抽签每会话一次、候选 label 恒 A/B，快照只记 blind 不泄露抽签方向。
func TestSemanticDynamicAuditionBlindDrawsOncePerSession(t *testing.T) {
	t.Setenv(auditionBlindEnvVar, "1")
	draws := 0
	s, plan := semanticDynamicAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *semanticDynamicAuditionPlan) (map[string]any, error) {
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	})
	s.auditionBlindDraw = func() bool {
		draws++
		return true
	}

	bracket, err := s.runSemanticDynamicAuditionBracket(context.Background(), plan, func() error { return nil })
	if err != nil || bracket.Record == nil {
		t.Fatalf("blind mount must succeed: err=%v bracket=%+v", err, bracket)
	}
	if draws != 1 {
		t.Fatalf("the draw must happen exactly once per mounted session, got %d", draws)
	}
	if !bracket.Record.Blind || !bracket.Record.BlindSwap {
		t.Fatalf("the record must carry the blind mode and the drawn swap: %+v", bracket.Record)
	}
	if bracket.Record.Session["blind"] != true {
		t.Fatalf("the session snapshot must state the blind mode: %+v", bracket.Record.Session)
	}
	if _, leaked := bracket.Record.Session["blind_swap"]; leaked {
		t.Fatalf("the snapshot must never leak the drawn assignment: %+v", bracket.Record.Session)
	}
	requests := s.auditionKernel.(*mixTickAuditionKernelStub).requests
	if len(requests) != 1 {
		t.Fatalf("prepare calls=%d", len(requests))
	}
	candidates := requests[0].Candidates
	if candidates[0].ID != auditionCandidateA || candidates[1].ID != auditionCandidateB ||
		candidates[0].Label != "A" || candidates[1].Label != "B" {
		t.Fatalf("labels must stay A/B under the swap: %+v", candidates)
	}
	if !strings.Contains(candidates[0].SourceRef, "after_revision_") || !strings.Contains(candidates[1].SourceRef, "before_revision_") {
		t.Fatalf("a swapped draw must exchange the physical renders behind the fixed labels: a=%s b=%s", candidates[0].SourceRef, candidates[1].SourceRef)
	}
}
