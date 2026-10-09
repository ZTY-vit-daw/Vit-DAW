package pullharness

// exit_wiring_test.go — 真实退场接线验收（卡面验收 1"真实 ExitExecutor
// 的 T1 退场三态行为测试"）：
//   - TurnBoundaryEvent.Window/Budget 构造落位断言（字段透传+线位经
//     ExitWiring）；
//   - T1 三态（retain/ref/drop）走 contextruntime 既有机械判据（真实
//     NewExitExecutor，零新造），经循环事件构造面与整场 Run 双层验证；
//   - 不变式执法可见：有证据负担而无结论无句柄的单元被拒绝退场。

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
)

func fixedExitClock() time.Time {
	return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
}

// BatchTurnEvent 构造落位：单元字段透传（Kind/ID/Bytes/TurnID/HandleRef/
// EvidenceRefs/RetainedStatement）、热字节合计、线位与时钟经 ExitWiring。
func TestBatchTurnEventConstructionPlaced(t *testing.T) {
	batch := []ToolResult{
		{ID: "o1", Tool: "ref.query", Status: "ok", Bytes: 120,
			HandleRef: "evidence://abc", EvidenceRefs: []string{"vit://track/t1"},
			RetainedStatement: "level relation recorded for t1/t2"},
		{ID: "o2", Tool: "ref.query", Status: "ok", Bytes: 30},
	}
	wiring := ExitWiring{HistoryLimit: 7, HotLimitBytes: 4096, Now: fixedExitClock}
	event, units, hotBytes := BatchTurnEvent("run-1:cycle:2", batch, wiring)

	if event.TurnID != "run-1:cycle:2" {
		t.Fatalf("event TurnID = %q", event.TurnID)
	}
	if !event.Now.Equal(fixedExitClock()) {
		t.Fatalf("event Now = %v, want wired clock", event.Now)
	}
	if event.Window.HistoryLimit != 7 || event.Budget.HotLimitBytes != 4096 {
		t.Fatalf("wiring lines not placed: history=%d hot=%d", event.Window.HistoryLimit, event.Budget.HotLimitBytes)
	}
	if event.Budget.HotBytes != 150 || hotBytes != 150 {
		t.Fatalf("hot bytes = event:%d returned:%d, want 150", event.Budget.HotBytes, hotBytes)
	}
	if len(units) != 2 || len(event.Window.Units) != 2 {
		t.Fatalf("units = %d/%d, want 2/2", len(units), len(event.Window.Units))
	}
	first := event.Window.Units[0]
	if first.Unit.Kind != contextruntime.ExitUnitToolResult || first.Unit.ID != "o1" ||
		first.Bytes != 120 || first.TurnID != "run-1:cycle:2" ||
		first.HandleRef != "evidence://abc" || first.RetainedStatement != "level relation recorded for t1/t2" ||
		len(first.EvidenceRefs) != 1 || first.EvidenceRefs[0] != "vit://track/t1" {
		t.Fatalf("unit passthrough broken: %+v", first)
	}
}

// FinalTurnEvent 构造落位：TurnID=RunID（exit_retain 先例）、累计窗复制、
// 线位透传。
func TestFinalTurnEventConstructionPlaced(t *testing.T) {
	units := []contextruntime.WindowUnit{
		{Unit: contextruntime.ExitUnit{Kind: contextruntime.ExitUnitToolResult, ID: "o1"}, Bytes: 120, TurnID: "run-1:cycle:1"},
	}
	wiring := ExitWiring{HotLimitBytes: 4096, Now: fixedExitClock}
	event := FinalTurnEvent("run-1", units, 120, wiring)
	if event.TurnID != "run-1" || !event.Now.Equal(fixedExitClock()) {
		t.Fatalf("final event identity broken: %+v", event)
	}
	if len(event.Window.Units) != 1 || event.Window.Units[0].Unit.ID != "o1" {
		t.Fatalf("final window units = %+v", event.Window.Units)
	}
	// 事件持有副本：累计窗后续变化不影响已构造事件。
	units[0].Unit.ID = "mutated"
	if event.Window.Units[0].Unit.ID != "o1" {
		t.Fatalf("final event window shares backing array with caller slice")
	}
}

// 真实执行器语义冒针：NewWiredExitExecutor 返回的是 contextruntime 既有
// 判据（retain 产 LedgerEntry；证据负担无结论无句柄→拒绝退场）。
func TestNewWiredExitExecutorRealCriteria(t *testing.T) {
	executor := NewWiredExitExecutor(contextruntime.ExitExecutorConfig{Now: fixedExitClock})
	wiring := ExitWiring{Now: fixedExitClock}

	retained := []ToolResult{{ID: "o1", Bytes: 10, RetainedStatement: "conclusion row"}}
	event, _, _ := BatchTurnEvent("run-1:cycle:1", retained, wiring)
	report := executor.OnTurnBoundary(context.Background(), event)
	if len(report.Decisions) != 1 || report.Decisions[0].Action != contextruntime.ExitRetain {
		t.Fatalf("retain decision missing: %+v", report)
	}
	entry := report.Decisions[0].LedgerEntry
	if entry == nil || entry.Statement != "conclusion row" {
		t.Fatalf("retain ledger entry missing/statement diverged: %+v", entry)
	}

	ref := []ToolResult{{ID: "o2", Bytes: 10, HandleRef: "evidence://xyz"}}
	event, _, _ = BatchTurnEvent("run-1:cycle:1", ref, wiring)
	report = executor.OnTurnBoundary(context.Background(), event)
	if len(report.Decisions) != 1 || report.Decisions[0].Action != contextruntime.ExitRef ||
		report.Decisions[0].HandleRef != "evidence://xyz" {
		t.Fatalf("ref decision missing: %+v", report)
	}

	plain := []ToolResult{{ID: "o3", Bytes: 10}}
	event, _, _ = BatchTurnEvent("run-1:cycle:1", plain, wiring)
	report = executor.OnTurnBoundary(context.Background(), event)
	if len(report.Decisions) != 1 || report.Decisions[0].Action != contextruntime.ExitDrop {
		t.Fatalf("drop decision missing: %+v", report)
	}

	// 不变式：有证据负担而无结论无句柄 → 拒绝退场（宁可超预算不丢证据链）。
	burdened := []ToolResult{{ID: "o4", Bytes: 10, EvidenceRefs: []string{"vit://track/t1"}}}
	event, _, _ = BatchTurnEvent("run-1:cycle:1", burdened, wiring)
	report = executor.OnTurnBoundary(context.Background(), event)
	if len(report.Decisions) != 0 || len(report.Violations) != 1 {
		t.Fatalf("invariant refusal missing: decisions=%d violations=%d", len(report.Decisions), len(report.Violations))
	}
}

// 线位执法：HotLimitBytes 经 ExitWiring 落位后 budget 判据按既有规则触发
// （超线→最旧优先淘汰）。落点=T2 累计窗：早前批轮单元（TurnID 标批轮次，
// 非 T2 候选）在线位超限时由 budget 判据淘汰最旧者。
func TestExitWiringBudgetLineEnforced(t *testing.T) {
	executor := NewWiredExitExecutor(contextruntime.ExitExecutorConfig{Now: fixedExitClock})
	units := []contextruntime.WindowUnit{
		{Unit: contextruntime.ExitUnit{Kind: contextruntime.ExitUnitToolResult, ID: "old"}, Bytes: 100, TurnID: "run-1:cycle:1"},
		{Unit: contextruntime.ExitUnit{Kind: contextruntime.ExitUnitToolResult, ID: "new"}, Bytes: 100, TurnID: "run-1:cycle:2"},
	}
	wiring := ExitWiring{HotLimitBytes: 150, Now: fixedExitClock}
	event := FinalTurnEvent("run-1", units, 200, wiring)
	report := executor.OnTurnBoundary(context.Background(), event)
	if len(report.Decisions) != 1 {
		t.Fatalf("budget line should evict oldest unit only: %+v", report.Decisions)
	}
	if report.Decisions[0].Unit.ID != "old" || report.Decisions[0].Reason != contextruntime.ExitReasonBudget {
		t.Fatalf("budget eviction = %+v, want oldest unit via budget reason", report.Decisions[0])
	}
}

// recordingExit 包装真实执行器留取事件与报告（决策全部出自真实执行器）。
type recordingExit struct {
	inner   contextruntime.ExitExecutor
	events  []contextruntime.TurnBoundaryEvent
	reports []contextruntime.ExitReport
}

func (r *recordingExit) OnTurnBoundary(ctx context.Context, ev contextruntime.TurnBoundaryEvent) contextruntime.ExitReport {
	r.events = append(r.events, ev)
	report := r.inner.OnTurnBoundary(ctx, ev)
	r.reports = append(r.reports, report)
	return report
}

// T1 三态经整场 Run（Session 面，L1-5-IMPL-D 腿1 随迁）：宿主 CloseCycle
// 用注入执行器构造批事件（三批分别产出 retain/drop、ref）；宿主 Return 终局
// 提交发 T2（FinalTurnEvent，TurnID=RunID）——窗面重供单元但不重复判定。
func TestPullLoopT1ThreeStatesWithRealExitExecutor(t *testing.T) {
	executor := &recordingExit{inner: NewWiredExitExecutor(contextruntime.ExitExecutorConfig{Now: fixedExitClock})}
	session := &fakeSession{runID: "run-3s", goalID: "goal-1"}
	batches := [][]ToolResult{
		{
			{ID: "o1", Tool: "ref.query", Status: "ok", Bytes: 40, RetainedStatement: "target evidence concluded for t1"},
			{ID: "o2", Tool: "ref.query", Status: "ok", Bytes: 20},
		},
		{
			{ID: "o3", Tool: "ref.query", Status: "ok", Bytes: 60, HandleRef: "evidence://cas9"},
		},
	}
	cycle := 0
	var windowUnits []contextruntime.WindowUnit
	var hotBytes int64
	session.closeCycleFn = func(f *fakeSession, ctx context.Context, batchID string, exit contextruntime.ExitExecutor) error {
		batch := batches[cycle]
		cycle++
		event, units, hot := BatchTurnEvent(batchID, batch, ExitWiring{Now: fixedExitClock})
		windowUnits = append(windowUnits, units...)
		hotBytes += hot
		exit.OnTurnBoundary(ctx, event)
		f.budget.CompletedCycles++
		return nil
	}
	session.returnFn = func(f *fakeSession, ctx context.Context, step Step) (Result, error) {
		if step.Disposition == DispositionTerminal {
			// 宿主终局提交=唯一 T2 owner（S1 §11.1：驱动不发 T2）。
			executor.OnTurnBoundary(ctx, FinalTurnEvent("run-3s", windowUnits, hotBytes, ExitWiring{Now: fixedExitClock}))
		}
		return Result{
			RunID: "run-3s", GoalID: "goal-1", Outcome: OutcomeJudgmentOK,
			Cycles: f.budget.CompletedCycles, ModelTurns: f.budget.ModelCalls,
			Conversation: append([]llm.Message(nil), f.conversation...),
		}, nil
	}
	loop := &PullLoop{
		LLM:        &fakeLLM{responses: []string{"call:probe:o1", "call:probe:o3", "final judgment, no tools"}},
		Session:    session,
		Prefix:     &fakePrefix{},
		Exit:       executor,
		Budget:     ObservationBudget{MaxCycles: 8},
		ExitWiring: ExitWiring{Now: fixedExitClock},
	}
	result := loop.Run(context.Background(), GoalInput{
		GoalID: "goal-1", RunID: "run-3s", UserText: "check the demo project", Engine: config.EngineConfig{},
	})
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome = %q error=%q, want %q", result.Outcome, result.Error, OutcomeJudgmentOK)
	}
	if len(executor.events) != 3 {
		t.Fatalf("boundary events = %d, want 3 (T1 x2 + T2)", len(executor.events))
	}
	wantTurnIDs := []string{"run-3s:cycle:1", "run-3s:cycle:2", "run-3s"}
	for i, want := range wantTurnIDs {
		if executor.events[i].TurnID != want {
			t.Fatalf("event %d TurnID = %q, want %q", i, executor.events[i].TurnID, want)
		}
	}

	first := executor.reports[0]
	if len(first.Decisions) != 2 {
		t.Fatalf("cycle 1 decisions = %d, want 2: %+v", len(first.Decisions), first.Decisions)
	}
	if first.Decisions[0].Action != contextruntime.ExitRetain || first.Decisions[0].LedgerEntry == nil ||
		first.Decisions[0].LedgerEntry.Statement != "target evidence concluded for t1" {
		t.Fatalf("cycle 1 first decision = %+v, want retain with ledger entry", first.Decisions[0])
	}
	if first.Decisions[1].Action != contextruntime.ExitDrop {
		t.Fatalf("cycle 1 second decision = %+v, want drop", first.Decisions[1])
	}

	second := executor.reports[1]
	if len(second.Decisions) != 1 || second.Decisions[0].Action != contextruntime.ExitRef ||
		second.Decisions[0].HandleRef != "evidence://cas9" {
		t.Fatalf("cycle 2 decisions = %+v, want ref via handle", second.Decisions)
	}

	// T2 窗面=run 累计单元；单元 TurnID 标批轮次 ≠ T2 TurnID → 既有判据
	// 不重复判定（结论供给在 T1 批轮完成，无双重入账）。
	final := executor.reports[2]
	if len(final.Decisions) != 0 {
		t.Fatalf("T2 re-decisions = %+v, want none (units belong to earlier cycle turns)", final.Decisions)
	}
	if len(executor.events[2].Window.Units) != 3 {
		t.Fatalf("T2 window units = %d, want 3 (run accumulation)", len(executor.events[2].Window.Units))
	}
	if executor.events[2].Budget.HotBytes != 120 {
		t.Fatalf("T2 hot bytes = %d, want 120 (40+20+60)", executor.events[2].Budget.HotBytes)
	}
}
