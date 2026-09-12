package chat

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/trajectory"
)

// B12-2（2026-09-12）mix-tick A/B 试听接线四钉：
//  ① apply 前 before 渲染时序（渲染先于 mutation 提交）
//  ② 确认卡 A/B 判定映射（选 A→撤销 / 选 B→保留 / 听不出或 free_text→ambiguous）
//  ③ 渲染失败 fail-open 不阻塞确认
//  ④ 终局措辞三态（manual 有入口 / manual 渲染失败降级 / full access）

type mixTickAuditionKernelStub struct {
	requests []kernel.AuditionSessionRequest
	status   string
	err      error
}

func (f *mixTickAuditionKernelStub) sessionFor(request kernel.AuditionSessionRequest) map[string]any {
	status := f.status
	if status == "" {
		status = "ready"
	}
	rows := make([]any, 0, len(request.Candidates))
	for _, candidate := range request.Candidates {
		rows = append(rows, map[string]any{
			"id": candidate.ID, "label": candidate.Label, "status": "ready",
			"source_kind": candidate.SourceKind, "source_ref": candidate.SourceRef,
			"preview_ref": candidate.PreviewRef, "project_revision": candidate.ProjectRevision,
		})
	}
	return map[string]any{"session_id": request.SessionID, "conversation_id": request.ConversationID, "status": status, "candidates": rows}
}

func (f *mixTickAuditionKernelStub) AuditionPrepare(_ context.Context, request kernel.AuditionSessionRequest) (*kernel.VSPCommandResult, error) {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return nil, f.err
	}
	return &kernel.VSPCommandResult{LegacyReply: map[string]any{"status": "ok", "session": f.sessionFor(request)}}, nil
}
func (f *mixTickAuditionKernelStub) AuditionStatus(context.Context, string) (*kernel.VSPCommandResult, error) {
	return nil, nil
}
func (f *mixTickAuditionKernelStub) AuditionSelect(context.Context, string, string) (*kernel.VSPCommandResult, error) {
	return nil, nil
}
func (f *mixTickAuditionKernelStub) AuditionStop(context.Context, string) (*kernel.VSPCommandResult, error) {
	return nil, nil
}

// mixTickAuditionReadyRowForTest writes a real WAV payload so the pair builder's
// file checks (validD1RenderFile / hashD1RenderFile) see what production sees.
func mixTickAuditionReadyRowForTest(t *testing.T, phase, revision string) map[string]any {
	t.Helper()
	path := filepath.Join(t.TempDir(), phase+"_revision_"+revision+".wav")
	payload := append([]byte("RIFF"), make([]byte, 256)...)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"schema_version": d1RenderSchema, "phase": phase, "status": "ready",
		"file_path": path, "project_revision": revision,
		"sha256": strings.Repeat(phase[:1], 64), "size_bytes": len(payload),
		"render_revision":  "mix_tick_render:" + phase + ":" + revision + ":0123456789abcdef",
		"preview_revision": "sha256:" + strings.Repeat(phase[:1], 64),
	}
}

func mixTickAuditionPlanForTest(t *testing.T, render func(context.Context, string, string, *mixTickAuditionPlan) (map[string]any, error), order *[]string) (*Server, *mixTickAuditionPlan) {
	t.Helper()
	s := testContinuationServer()
	s.mixTickAuditionRenderOverride = render
	s.auditionKernel = &mixTickAuditionKernelStub{}
	return s, &mixTickAuditionPlan{
		ConversationID: "conversation-b12", GoalID: "goal-b12", RunID: "run-b12",
		TrackID: "1007", Operation: "track_gain_adjust", TickID: "tick-b12",
		BeforeRevision: "rev-4",
		AfterRevision:  func() string { return "rev-5" },
	}
}

// 钉①：before 相必须在 mutation 提交之前解析，after 相在其之后。RED 反向编辑
// ＝把 bracket 里的两相顺序对调或让 after 相先跑，本钉立即红。
func TestMixTickAuditionBracketRendersBeforeTheMutation(t *testing.T) {
	var order []string
	s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
		order = append(order, "render:"+phase+":"+revision)
		return mixTickAuditionReadyRowForTest(t, phase, revision), nil
	}, &order)

	bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error {
		order = append(order, "mutate")
		return nil
	})
	if err != nil {
		t.Fatalf("bracket must not fail on a clean pair: %v", err)
	}
	want := []string{"render:before:rev-4", "mutate", "render:after:rev-5"}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Fatalf("B12-2 时序必须是 before 渲染 -> mutation -> after 渲染:\n got %v\nwant %v", order, want)
	}
	if bracket.Record == nil || bracket.Degraded != "" {
		t.Fatalf("a clean pair must mount the A/B session: %+v", bracket)
	}
	if bracket.Record.SessionID == "" || bracket.Record.TurnID == "" || bracket.Record.RoundID == "" {
		t.Fatalf("the mounted session must carry the identity the WebUI echoes back: %+v", bracket.Record)
	}
	if bracket.Record.ProjectRevision != "rev-5" {
		t.Fatalf("the session must be bound to the post-mutation revision: %q", bracket.Record.ProjectRevision)
	}
	// The card is only answerable if the judgment-requested event landed.
	events, _ := s.agentEventsSince("conversation-b12", 0, 64)
	sawReady := false
	for _, event := range events {
		if event.Type == "audition.ready" {
			sawReady = true
		}
	}
	if !sawReady {
		t.Fatalf("mount must emit audition.ready for the WebUI card: %+v", events)
	}
	// 判定按钮的唯一使能位：B12-2 勘察确认 WebUI 的 canJudge 只读
	// session.judgmentRequested，而该标志只由 trajectory.user_judgment.requested
	// 置位。没有这条事件，卡片会渲染但永远无法应答——这是本卡在真栈上最易
	// 静默失效的接线点，故钉住它确实落地且携带 POST 会回显的会话身份。
	sawJudgmentRequest := false
	for _, event := range events {
		if event.Type != string(trajectory.EventUserJudgmentRequested) {
			continue
		}
		sawJudgmentRequest = true
		details, _ := event.Payload["details"].(map[string]any)
		if firstStringFromMap(details, "audition_session_id") != bracket.Record.SessionID {
			t.Fatalf("the judgment node must name the session the POST echoes back: %+v", details)
		}
	}
	if !sawJudgmentRequest {
		t.Fatalf("without the judgment-requested event the A/B card renders but is unanswerable: %+v", events)
	}
	if len(s.auditionKernel.(*mixTickAuditionKernelStub).requests) != 1 {
		t.Fatalf("exactly one kernel session is prepared per pair: %+v", s.auditionKernel.(*mixTickAuditionKernelStub).requests)
	}
}

// 钉③：渲染/会话准备失败一律 fail-open——mutation 照跑、卡片降级、日志留痕，
// 绝不把用户已确认的这一小步扣在呈现层手里。mutation 自身失败时则不得再渲染
// after（改动没落地就没有 after 态）。
func TestMixTickAuditionBracketFailsOpen(t *testing.T) {
	t.Run("before render fails", func(t *testing.T) {
		var order []string
		s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
			order = append(order, "render:"+phase)
			if phase == mixTickAuditionPhaseBefore {
				return nil, errors.New("kernel render unavailable")
			}
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		}, &order)
		bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error {
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
	})

	t.Run("mutation fails", func(t *testing.T) {
		var order []string
		s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
			order = append(order, "render:"+phase)
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		}, &order)
		bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error {
			order = append(order, "mutate")
			return errors.New("mix.apply_tick failed")
		})
		if err == nil {
			t.Fatal("the mutation error must reach the caller so the existing failure contract is kept")
		}
		if bracket.Record != nil {
			t.Fatalf("no change means no A/B card: %+v", bracket)
		}
		if strings.Contains(strings.Join(order, "|"), "render:after") {
			t.Fatalf("a mutation that did not land has no after state to render: %v", order)
		}
	})

	t.Run("after render fails", func(t *testing.T) {
		s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
			if phase == mixTickAuditionPhaseAfter {
				return nil, errors.New("after render produced no valid WAV")
			}
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		}, nil)
		bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error { return nil })
		if err != nil {
			t.Fatalf("fail-open: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "after_render_failed" {
			t.Fatalf("bracket must degrade: %+v", bracket)
		}
	})

	t.Run("audition prepare fails", func(t *testing.T) {
		s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		}, nil)
		s.auditionKernel = &mixTickAuditionKernelStub{err: errors.New("preview unavailable")}
		bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error { return nil })
		if err != nil {
			t.Fatalf("fail-open: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "audition_prepare_failed" {
			t.Fatalf("bracket must degrade: %+v", bracket)
		}
	})

	t.Run("kernel returns a non-ready session", func(t *testing.T) {
		s, plan := mixTickAuditionPlanForTest(t, func(_ context.Context, phase, revision string, _ *mixTickAuditionPlan) (map[string]any, error) {
			return mixTickAuditionReadyRowForTest(t, phase, revision), nil
		}, nil)
		s.auditionKernel = &mixTickAuditionKernelStub{status: "preparing"}
		bracket, err := s.runMixTickAuditionBracket(context.Background(), plan, func() error { return nil })
		if err != nil {
			t.Fatalf("fail-open: %v", err)
		}
		if bracket.Record != nil || bracket.Degraded != "audition_prepare_failed" {
			t.Fatalf("an unservicable session is not a mounted card: %+v", bracket)
		}
	})
}

// 钉②：A/B 判定映射。retain/rollback 只由所选标签实际承载的物理侧决定，故盲态
// 交换下标 A 承载 after 时选 A 必须=保留而不是回滚（设计稿 §3 风险 1 同族面）。
func TestMixTickAuditionDispositionMapping(t *testing.T) {
	label := func(heard, preference string) mixTickAuditionReading {
		return mixTickAuditionReading{Heard: heard, Preference: preference}
	}
	yes := string(experiment.HeardDifferenceYes)
	no := string(experiment.HeardDifferenceNo)

	canonical := []struct {
		name       string
		reading    mixTickAuditionReading
		blindSwap  bool
		decision   string
		physical   string
		labelWants string
	}{
		{"canonical A undoes", label(yes, "a"), false, mixTickAuditionDecisionRollback, mixTickAuditionPhaseBefore, "A"},
		{"canonical B keeps", label(yes, "b"), false, mixTickAuditionDecisionRetain, mixTickAuditionPhaseAfter, "B"},
		{"blind A carries after keeps", label(yes, "a"), true, mixTickAuditionDecisionRetain, mixTickAuditionPhaseAfter, "A"},
		{"blind B carries before undoes", label(yes, "b"), true, mixTickAuditionDecisionRollback, mixTickAuditionPhaseBefore, "B"},
	}
	for _, tc := range canonical {
		got := mixTickAuditionDispositionFor(tc.reading, tc.blindSwap)
		if got.Decision != tc.decision || got.Physical != tc.physical || got.Label != tc.labelWants {
			t.Fatalf("%s: got %+v want decision=%s physical=%s label=%s", tc.name, got, tc.decision, tc.physical, tc.labelWants)
		}
	}

	ambiguous := []struct {
		name      string
		reading   mixTickAuditionReading
		blindSwap bool
	}{
		{"heard nothing", label(no, "a"), false},
		{"heard unsure", label(string(experiment.HeardDifferenceUnsure), "b"), false},
		{"no preference", label(yes, ""), false},
		{"free text only", mixTickAuditionReading{Heard: yes, Reason: "free_text"}, false},
		{"heard nothing under blind", label(no, "b"), true},
	}
	for _, tc := range ambiguous {
		if got := mixTickAuditionDispositionFor(tc.reading, tc.blindSwap); got.Decision != mixTickAuditionDecisionAmbiguous {
			t.Fatalf("%s must stay ambiguous (no project change): %+v", tc.name, got)
		}
	}

	// Semantic wording states the physical intent directly and therefore must
	// not be re-derived through a swapped label.
	semantic := mixTickAuditionReading{Heard: yes, Decision: mixTickAuditionDecisionRollback}
	if got := mixTickAuditionDispositionFor(semantic, true); got.Decision != mixTickAuditionDecisionRollback || got.Physical != mixTickAuditionPhaseBefore {
		t.Fatalf("a stated rollback stays a rollback under any label assignment: %+v", got)
	}
	semantic = mixTickAuditionReading{Heard: yes, Decision: mixTickAuditionDecisionRetain}
	if got := mixTickAuditionDispositionFor(semantic, true); got.Decision != mixTickAuditionDecisionRetain || got.Physical != mixTickAuditionPhaseAfter {
		t.Fatalf("a stated retain stays a retain under any label assignment: %+v", got)
	}
}

// 钉②（读入面）：chat 面的 A/B 作答读入与 HTTP 面同源，且只认真正的 A/B 作答
// （无关消息落空回既有确认路径）。
func TestMixTickAuditionReadingFromMessage(t *testing.T) {
	cases := []struct {
		message    string
		ok         bool
		preference string
		decision   string
		heard      string
	}{
		{"选 B", true, "b", "", string(experiment.HeardDifferenceYes)},
		{"选A", true, "a", "", string(experiment.HeardDifferenceYes)},
		{"我觉得 A 更好", true, "a", "", string(experiment.HeardDifferenceYes)},
		{"听不出差别", true, "", "", string(experiment.HeardDifferenceNo)},
		{"撤销这一步", true, "", mixTickAuditionDecisionRollback, string(experiment.HeardDifferenceYes)},
		{"保留", true, "", mixTickAuditionDecisionRetain, string(experiment.HeardDifferenceYes)},
		{"给我讲讲为什么", false, "", "", ""},
		{"", false, "", "", ""},
	}
	for _, tc := range cases {
		reading, ok := mixTickAuditionReadingFromMessage(tc.message)
		if ok != tc.ok {
			t.Fatalf("%q: ok=%v want %v", tc.message, ok, tc.ok)
		}
		if !ok {
			continue
		}
		if reading.Preference != tc.preference || reading.Decision != tc.decision || reading.Heard != tc.heard {
			t.Fatalf("%q: got %+v want preference=%q decision=%q heard=%q", tc.message, reading, tc.preference, tc.decision, tc.heard)
		}
	}
}

// 钉④：终局措辞三态。manual 有真实入口才恢复「可试听判定」；manual 渲染失败
// 降级为事实陈述（不承诺不存在的卡）；full access 不带确认等待措辞（b6 缺陷③
// 分支零回退），但真实在场的卡仍被点名。
func TestMixTickAuditionTerminalWordingThreeStates(t *testing.T) {
	manualMounted := mixTickAuditionTerminalClause(mixTickJudgmentEntry{Manual: true, Mounted: true})
	if !strings.Contains(manualMounted, "A/B 试听已就绪") {
		t.Fatalf("manual with a mounted card must restore the servicable entry wording: %q", manualMounted)
	}
	for _, needle := range []string{"选 B", "选 A", "听不出差别"} {
		if !strings.Contains(manualMounted, needle) {
			t.Fatalf("the restored entry must state the A/B mapping (%q missing): %q", needle, manualMounted)
		}
	}

	manualDegraded := mixTickAuditionTerminalClause(mixTickJudgmentEntry{Manual: true, Degraded: "after_render_failed"})
	if strings.Contains(manualDegraded, "已就绪") || strings.Contains(manualDegraded, "可试听") {
		t.Fatalf("a degraded manual run must not promise a card it failed to build: %q", manualDegraded)
	}
	if !strings.Contains(manualDegraded, "没能生成") || !strings.Contains(manualDegraded, "after_render_failed") {
		t.Fatalf("the degraded wording must state the failure: %q", manualDegraded)
	}

	fullAccess := mixTickAuditionTerminalClause(mixTickJudgmentEntry{Mounted: true})
	if strings.Contains(fullAccess, "等待你确认") || strings.Contains(fullAccess, "确认前不会修改工程") {
		t.Fatalf("full access must not regain confirmation-wait wording: %q", fullAccess)
	}
	if !strings.Contains(fullAccess, "完全访问") {
		t.Fatalf("full access must name its own mode: %q", fullAccess)
	}

	for _, text := range []string{manualMounted, manualDegraded, fullAccess} {
		if strings.Contains(text, "人工判定") {
			t.Fatalf("the restored wording must not resurrect the b6 hollow-claim phrasing: %q", text)
		}
		b6AssertNoInternalTerms(t, text)
	}

	// b6 缺陷③ 的 pending display 分支逐字零回退：终局措辞的恢复不得把
	// 「等待你确认」带回完全访问面。
	candidate := pendingMixTickFallbackCandidateForAuditionTest()
	autoBody := pendingMixTickAutoApplyEventBody(candidate)
	if strings.Contains(autoBody, "等待你确认") || strings.Contains(autoBody, "确认前不会修改工程") {
		t.Fatalf("b6 缺陷③ regression: %q", autoBody)
	}
	if !strings.Contains(pendingMixTickEventBody(candidate), "正在等待你确认") {
		t.Fatalf("manual pending body must keep its confirmation promise: %q", pendingMixTickEventBody(candidate))
	}
}

func pendingMixTickFallbackCandidateForAuditionTest() agentloop.PendingMixTickCandidate {
	return agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "1007", DeltaDB: 1.5,
		Status: "pending_confirmation", ObservationID: "obs-b12",
	}
}

// 钉②③（落账面）：判定走 mix_tick 确认面——不写 experiment evidence；选 B=保留、
// 听不出=ambiguous，且卡片落账后即失效（不可二次应答）。
func TestMixTickAuditionJudgmentSettlesAtMixTickSlot(t *testing.T) {
	s := testContinuationServer()
	record := &mixTickAuditionRecord{
		SchemaVersion: mixTickAuditionSchemaVersion, ConversationID: "conversation-b12",
		GoalID: "goal-b12", RunID: "run-b12", SessionID: "audition:mix_tick:tick-b12",
		TurnID: "mix_tick_turn:tick-b12", RoundID: "mix_tick_round:tick-b12",
		Status: mixTickAuditionStatusReady, TrackID: "1007", Operation: "track_gain_adjust",
		TickID: "tick-b12", ProjectRevision: "rev-5", JudgmentPending: true,
		Session: map[string]any{"status": "ready", "candidates": []any{
			map[string]any{"id": auditionCandidateA, "status": "ready", "preview_ref": "a"},
			map[string]any{"id": auditionCandidateB, "status": "ready", "preview_ref": "b"},
		}},
	}
	s.storeMixTickAudition(record)

	retain, handled, err := s.recordMixTickAuditionJudgment(context.Background(), auditionJudgmentRequest{
		ConversationID: "conversation-b12", SessionID: record.SessionID, TurnID: record.TurnID,
		RoundID: record.RoundID, ProjectRevision: "rev-5",
		HeardDifference: "yes", Preference: "b",
	})
	if err != nil || !handled {
		t.Fatalf("retain judgment must land at the mix-tick slot: handled=%v err=%v", handled, err)
	}
	if retain["decision"] != mixTickAuditionDecisionRetain || retain["status"] != "retained" {
		t.Fatalf("选 B = 确认保留: %+v", retain)
	}
	if record.JudgmentPending {
		t.Fatal("a settled card must not stay answerable")
	}
	if _, ok := s.mixTickAuditionFor("conversation-b12"); ok {
		t.Fatal("the settled card must be retired from the conversation")
	}

	// A second answer on the retired card is not this path's business: it must
	// fall through (handled=false) so the free-state route owns it.
	if _, handled, _ := s.recordMixTickAuditionJudgment(context.Background(), auditionJudgmentRequest{
		ConversationID: "conversation-b12", SessionID: record.SessionID,
	}); handled {
		t.Fatal("a retired mix-tick card must not swallow a later judgment")
	}

	// Ambiguous: no project change, and the failure is not reported as success.
	s.storeMixTickAudition(&mixTickAuditionRecord{
		ConversationID: "conversation-b12", SessionID: "audition:mix_tick:tick-b13",
		Status: mixTickAuditionStatusReady, TickID: "tick-b13", ProjectRevision: "rev-5", JudgmentPending: true,
		Session: map[string]any{"status": "ready", "candidates": []any{
			map[string]any{"id": auditionCandidateA, "status": "ready", "preview_ref": "a"},
			map[string]any{"id": auditionCandidateB, "status": "ready", "preview_ref": "b"},
		}},
	})
	outcome, handled, err := s.recordMixTickAuditionJudgment(context.Background(), auditionJudgmentRequest{
		ConversationID: "conversation-b12", SessionID: "audition:mix_tick:tick-b13",
		ProjectRevision: "rev-5", HeardDifference: "no", Preference: "unsure",
	})
	if err != nil || !handled {
		t.Fatalf("ambiguous judgment must be recorded, not refused: handled=%v err=%v", handled, err)
	}
	if outcome["decision"] != mixTickAuditionDecisionAmbiguous || outcome["status"] != "ambiguous" {
		t.Fatalf("听不出差别 = ambiguous（不做任何工程修改）: %+v", outcome)
	}
	if _, ok := outcome["rollback"]; ok {
		t.Fatalf("ambiguous must never roll back: %+v", outcome)
	}
}

// 身份错配必须拒绝而不是猜测：一个不同会话/陈旧会话的作答不得落到这张卡上。
func TestMixTickAuditionJudgmentRejectsIdentityMismatch(t *testing.T) {
	s := testContinuationServer()
	record := &mixTickAuditionRecord{
		ConversationID: "conversation-b12", SessionID: "audition:mix_tick:tick-b12",
		TurnID: "mix_tick_turn:tick-b12", RoundID: "mix_tick_round:tick-b12",
		Status: mixTickAuditionStatusReady, TickID: "tick-b12", ProjectRevision: "rev-5", JudgmentPending: true,
		Session: map[string]any{"status": "ready", "candidates": []any{
			map[string]any{"id": auditionCandidateA, "status": "ready", "preview_ref": "a"},
			map[string]any{"id": auditionCandidateB, "status": "ready", "preview_ref": "b"},
		}},
	}
	s.storeMixTickAudition(record)
	for _, tc := range []struct{ name, round, revision string }{
		{"round mismatch", "mix_tick_round:other", "rev-5"},
		{"revision mismatch", record.RoundID, "rev-9"},
	} {
		if _, handled, err := s.recordMixTickAuditionJudgment(context.Background(), auditionJudgmentRequest{
			ConversationID: "conversation-b12", SessionID: record.SessionID,
			TurnID: record.TurnID, RoundID: tc.round, ProjectRevision: tc.revision,
			HeardDifference: "yes", Preference: "b",
		}); !handled || err == nil {
			t.Fatalf("%s must be refused, got handled=%v err=%v", tc.name, handled, err)
		}
	}
	if !record.JudgmentPending {
		t.Fatal("a refused judgment must leave the card answerable")
	}
}

// chat 面判定接线：卡片在场时，「选 B」走保留并落 mix_tick 面；读不懂的作答
// 保持既有 ambiguous_mix_tick_confirmation 路径且不做工程修改。
func TestMixTickAuditionChatJudgmentRouting(t *testing.T) {
	s := testContinuationServer()
	s.storeMixTickAudition(&mixTickAuditionRecord{
		ConversationID: "conversation-b12", SessionID: "audition:mix_tick:tick-b12",
		Status: mixTickAuditionStatusReady, TrackID: "1007", Operation: "track_gain_adjust",
		TickID: "tick-b12", ProjectRevision: "rev-5", JudgmentPending: true,
		Session: map[string]any{"status": "ready", "candidates": []any{
			map[string]any{"id": auditionCandidateA, "status": "ready", "preview_ref": "a"},
			map[string]any{"id": auditionCandidateB, "status": "ready", "preview_ref": "b"},
		}},
	})
	resp, handled := s.handleMixTickAuditionChat(context.Background(), "conversation-b12", ChatRequest{Message: "选 B"}, agentModeDefault)
	if !handled || resp.StopReason != "mix_tick_audition_retain" {
		t.Fatalf("chat-side 选 B must route to retain: %+v handled=%v", resp, handled)
	}
	if _, ok := s.mixTickAuditionFor("conversation-b12"); ok {
		t.Fatal("an answered card must be retired")
	}

	// An unreadable answer stays on the existing ambiguous path.
	s.storeMixTickAudition(&mixTickAuditionRecord{
		ConversationID: "conversation-b12", SessionID: "audition:mix_tick:tick-b13",
		Status: mixTickAuditionStatusReady, TrackID: "1007", Operation: "track_gain_adjust",
		TickID: "tick-b13", ProjectRevision: "rev-5", JudgmentPending: true,
		Session: map[string]any{"status": "ready", "candidates": []any{
			map[string]any{"id": auditionCandidateA, "status": "ready", "preview_ref": "a"},
			map[string]any{"id": auditionCandidateB, "status": "ready", "preview_ref": "b"},
		}},
	})
	// 「就这样吧」是 保留 的口语形态，属于作答；真正的非作答是提问与讨论。
	resp, handled = s.handleMixTickAuditionChat(context.Background(), "conversation-b12", ChatRequest{Message: "给我讲讲为什么这么调"}, agentModeDefault)
	if handled {
		t.Fatalf("提问不是 A/B 作答，必须落回既有确认路径而不是被本分支吞掉: %+v", resp)
	}
	if _, ok := s.mixTickAuditionFor("conversation-b12"); !ok {
		t.Fatal("a non-answer must leave the card answerable")
	}
	// 选 A = 确认撤销，走既有回滚路径。回滚本身需要执行面：没有 harness 时
	// 必须 fail-closed——不谎报成功、不退役卡片（用户仍可重答）。
	resp, handled = s.handleMixTickAuditionChat(context.Background(), "conversation-b12", ChatRequest{Message: "选 A"}, agentModeDefault)
	if !handled {
		t.Fatalf("选 A must be handled, got %+v", resp)
	}
	if resp.StopReason != "mix_tick_audition_judgment_failed" || resp.Error == "" {
		t.Fatalf("a rollback that cannot execute must fail closed, got %+v", resp)
	}
	if !strings.Contains(resp.Reply, "没有") {
		t.Fatalf("a failed rollback must say the project was not further changed: %q", resp.Reply)
	}
	if _, ok := s.mixTickAuditionFor("conversation-b12"); !ok {
		t.Fatal("a failed rollback must leave the card answerable")
	}
}

// 无卡片时本分支必须完全落空，既有确认语义零回退（现有 mix_tick 钉同时守住
// 整条链路；这里钉住新增分支的空转面）。
func TestMixTickAuditionChatFallsThroughWithoutCard(t *testing.T) {
	s := testContinuationServer()
	if _, handled := s.handleMixTickAuditionChat(context.Background(), "conversation-b12", ChatRequest{Message: "选 A"}, agentModeDefault); handled {
		t.Fatal("no card means the A/B seat owns nothing")
	}
}

// 权限模式映射：manual 与 full access 各自落在正确的措辞分支上。
func TestMixTickAuditionEntryFollowsAuthorityMode(t *testing.T) {
	mounted := mixTickAuditionBracket{Record: &mixTickAuditionRecord{ConversationID: "conversation-b12"}}
	if entry := mixTickAuditionEntryFor(ChatRequest{Context: map[string]any{}}, mounted); !entry.Manual || !entry.Mounted {
		t.Fatalf("default authority is manual: %+v", entry)
	}
	full := mixTickAuditionEntryFor(ChatRequest{Context: map[string]any{"authority_mode": authorityModeFull}}, mounted)
	if full.Manual {
		t.Fatalf("full_project_access must not take the manual branch: %+v", full)
	}
	degraded := mixTickAuditionEntryFor(ChatRequest{Context: map[string]any{}}, mixTickAuditionBracket{Degraded: "after_render_failed"})
	if degraded.Mounted || degraded.Degraded != "after_render_failed" {
		t.Fatalf("degraded bracket must not read as mounted: %+v", degraded)
	}
}
