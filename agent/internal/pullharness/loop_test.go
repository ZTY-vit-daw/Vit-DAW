package pullharness

// loop_test.go — PullLoop 七步循环验收（卡面验收标准 1）：
// T1 每批恰一次且 TurnID 序列正确 / T2 终态恰一次、暂停零触发 /
// budget 超限→budget_exhausted / 溢出→fail-closed 不发请求。
// 全部经测试 fake（真实工具面接线归 IMPL-D）；真实 PrefixService 参与
// 装配面断言（接口冻结消费面的实证）。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
)

// fakeLLM 按脚本逐轮返回响应；记录每次收到的 messages（装配消费面断言）。
type fakeLLM struct {
	responses  []string
	calls      int
	gotMsgs    [][]llm.Message
	cancelOn   int // 第 N 次调用时取消 ctx（1 基；0=不取消）
	cancel     context.CancelFunc
	forceError error
}

func (f *fakeLLM) Complete(ctx context.Context, cfg config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls++
	f.gotMsgs = append(f.gotMsgs, append([]llm.Message(nil), messages...))
	if f.forceError != nil {
		return "", f.forceError
	}
	if f.cancelOn > 0 && f.calls == f.cancelOn {
		f.cancel()
		return "", ctx.Err()
	}
	if f.calls > len(f.responses) {
		return "", fmt.Errorf("fakeLLM: script exhausted at call %d", f.calls)
	}
	return f.responses[f.calls-1], nil
}

// fakeTools 的 Plan 认 "call:<tool>:<id>" 行协议；Execute 按批返回脚本。
type fakeTools struct {
	executeCalls int
	plannedCalls int
	batches      [][]ToolResult
	planOf       func(responseText string) []ToolCall
}

func (f *fakeTools) Plan(responseText string) []ToolCall {
	if f.planOf != nil {
		return f.planOf(responseText)
	}
	var calls []ToolCall
	for _, line := range strings.Split(responseText, "\n") {
		parts := strings.Split(strings.TrimSpace(line), ":")
		if len(parts) == 3 && parts[0] == "call" {
			calls = append(calls, ToolCall{ID: parts[2], Tool: parts[1]})
		}
	}
	f.plannedCalls += len(calls)
	return calls
}

func (f *fakeTools) Execute(ctx context.Context, calls []ToolCall) []ToolResult {
	f.executeCalls++
	if f.executeCalls <= len(f.batches) {
		return f.batches[f.executeCalls-1]
	}
	var results []ToolResult
	for _, call := range calls {
		results = append(results, ToolResult{ID: call.ID, Tool: call.Tool, Status: "ok", Bytes: 100, ProbeCost: 0.5, ModelLine: "tool " + call.ID + " ok"})
	}
	return results
}

type fakeExit struct {
	events []contextruntime.TurnBoundaryEvent
}

func (f *fakeExit) OnTurnBoundary(ctx context.Context, ev contextruntime.TurnBoundaryEvent) contextruntime.ExitReport {
	f.events = append(f.events, ev)
	return contextruntime.ExitReport{}
}

type fakeRouter struct {
	outcome FastPathOutcome
	hit     bool
	routes  int
}

func (f *fakeRouter) Route(ctx context.Context, in GoalInput) (FastPathOutcome, bool) {
	f.routes++
	return f.outcome, f.hit
}

type fakePrefix struct {
	last promptruntime.PrefixRequest
}

func (f *fakePrefix) Assemble(ctx context.Context, req promptruntime.PrefixRequest) (promptruntime.Assembly, promptruntime.AssemblyReport, error) {
	f.last = req
	return promptruntime.Build(req.AssemblyInput), promptruntime.AssemblyReport{
		Layers: []promptruntime.LayerReport{}, Breaks: []promptruntime.BreakEvent{},
		CacheAnomalies: []string{}, HistoryRefs: []promptruntime.HistoryRefEntry{},
		ExitViolations: []string{},
	}, nil
}

func goalInput(runID string) GoalInput {
	return GoalInput{
		GoalID:   "goal-1",
		RunID:    runID,
		UserText: "演示工程里给主唱轨做淡入",
		Engine:   config.EngineConfig{},
	}
}

func countTurnID(events []contextruntime.TurnBoundaryEvent, turnID string) int {
	count := 0
	for _, ev := range events {
		if ev.TurnID == turnID {
			count++
		}
	}
	return count
}

// T1：每工具批恰一次，TurnID=<RunID>:cycle:<n> 序列；T2 终局恰一次且
// 窗面=累计批单元。
func TestPullLoopT1OncePerBatchWithTurnIDSequence(t *testing.T) {
	llmFake := &fakeLLM{responses: []string{
		"call:ref.query:q1",
		"call:ref.diff:d1",
		"淡入已完成，判定通过", // 无工具调用 → 终局判定
	}}
	tools := &fakeTools{}
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Tools:  tools,
		Exit:   exitFake,
		Budget: ObservationBudget{MaxCycles: 10},
	}

	result := loop.Run(context.Background(), goalInput("run-1"))

	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome = %q, want %q (error: %s)", result.Outcome, OutcomeJudgmentOK, result.Error)
	}
	if result.ModelTurns != 3 || result.Cycles != 2 {
		t.Fatalf("model_turns/cycles = %d/%d, want 3/2", result.ModelTurns, result.Cycles)
	}
	// T1：每批恰一次，序列 cycle:1 → cycle:2。
	if got := countTurnID(exitFake.events, "run-1:cycle:1"); got != 1 {
		t.Fatalf("T1 cycle:1 count = %d, want 1 (events: %v)", got, exitFake.events)
	}
	if got := countTurnID(exitFake.events, "run-1:cycle:2"); got != 1 {
		t.Fatalf("T1 cycle:2 count = %d, want 1", got)
	}
	// T2：终局恰一次，TurnID=RunID，窗面=两批累计单元。
	if got := countTurnID(exitFake.events, "run-1"); got != 1 {
		t.Fatalf("T2 count = %d, want exactly 1", got)
	}
	final := exitFake.events[len(exitFake.events)-1]
	if len(final.Window.Units) != 2 {
		t.Fatalf("T2 window units = %d, want 2 (accumulated batches)", len(final.Window.Units))
	}
	for i, unit := range final.Window.Units {
		want := fmt.Sprintf("run-1:cycle:%d", i+1)
		if unit.TurnID != want {
			t.Fatalf("unit %d TurnID = %q, want %q", i, unit.TurnID, want)
		}
		if unit.Unit.Kind != contextruntime.ExitUnitToolResult {
			t.Fatalf("unit %d kind = %q, want tool_result", i, unit.Unit.Kind)
		}
	}
	if final.Budget.HotBytes != 200 {
		t.Fatalf("T2 hot bytes = %d, want 200", final.Budget.HotBytes)
	}
	// 工具批与 T1 一一对应。
	if tools.executeCalls != 2 {
		t.Fatalf("execute calls = %d, want 2", tools.executeCalls)
	}
	if result.ProbeSpent != 1.0 {
		t.Fatalf("probe spent = %v, want 1.0", result.ProbeSpent)
	}
}

// T2 终态恰一次（无工具轮直达终态 + 快路径短路两个终态路径都验）。
func TestPullLoopT2ExactlyOnceOnTerminals(t *testing.T) {
	// 路径 1：首轮即无工具调用。
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"done"}},
		Prefix: &fakePrefix{},
		Exit:   exitFake,
	}
	if result := loop.Run(context.Background(), goalInput("run-t2a")); result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome = %q, want judgment_ok", result.Outcome)
	}
	if len(exitFake.events) != 1 || exitFake.events[0].TurnID != "run-t2a" {
		t.Fatalf("terminal run must fire exactly one T2 with TurnID=RunID, got %+v", exitFake.events)
	}

	// 路径 2：快路径短路（零模型轮）也只一次 T2、零 T1。
	router := &fakeRouter{hit: true, outcome: FastPathOutcome{Reply: "确定性淡入已应用", Outcome: "fastpath_mix_fade", ProbeCost: 0.2}}
	exitFake2 := &fakeExit{}
	llmFake := &fakeLLM{responses: []string{"should not be reached"}}
	loop2 := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Exit:   exitFake2,
		Router: router,
	}
	result := loop2.Run(context.Background(), goalInput("run-t2b"))
	if result.Outcome != "fastpath_mix_fade" || result.Reply != "确定性淡入已应用" {
		t.Fatalf("router shortcut outcome/reply mismatch: %+v", result)
	}
	if result.ModelTurns != 0 || llmFake.calls != 0 {
		t.Fatalf("shortcut must make zero model turns, got turns=%d calls=%d", result.ModelTurns, llmFake.calls)
	}
	if result.Shortcuts != 1 || result.ProbeSpent != 0.2 {
		t.Fatalf("shortcut accounting mismatch: %+v", result)
	}
	if len(exitFake2.events) != 1 || exitFake2.events[0].TurnID != "run-t2b" {
		t.Fatalf("shortcut run must fire exactly one T2, got %+v", exitFake2.events)
	}
}

// 暂停/中断面（ctx 取消）零 T2：会话载体随 Result.Conversation 存续。
func TestPullLoopPauseSurfaceNeverFiresT2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	llmFake := &fakeLLM{
		responses: []string{"call:ref.query:q1", "second"},
		cancelOn:  2,
		cancel:    cancel,
	}
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Tools:  &fakeTools{},
		Exit:   exitFake,
		Budget: ObservationBudget{MaxCycles: 10},
	}

	result := loop.Run(ctx, goalInput("run-pause"))

	if result.Outcome != OutcomeInterrupted {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeInterrupted)
	}
	// 第一批的 T1 已发生，但 T2 必须零触发。
	if got := countTurnID(exitFake.events, "run-pause:cycle:1"); got != 1 {
		t.Fatalf("T1 cycle:1 count = %d, want 1", got)
	}
	if got := countTurnID(exitFake.events, "run-pause"); got != 0 {
		t.Fatalf("pause surface fired T2 %d times, want 0", got)
	}
	if len(result.Conversation) == 0 {
		t.Fatal("interrupted run must carry the conversation carrier (continuation minimal face)")
	}
}

// budget 超限 → 止损终态 budget_exhausted（独立分类）：循环节上限腿。
func TestPullLoopBudgetExhaustedOnCycleCap(t *testing.T) {
	llmFake := &fakeLLM{responses: []string{"call:ref.query:q1", "call:ref.query:q2"}}
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Tools:  &fakeTools{},
		Exit:   exitFake,
		Budget: ObservationBudget{MaxCycles: 2},
	}

	result := loop.Run(context.Background(), goalInput("run-budget"))

	if result.Outcome != OutcomeBudgetExhausted {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeBudgetExhausted)
	}
	if result.Outcome == OutcomeNoCandidateFound || result.Outcome == OutcomeCapabilityBlocked {
		t.Fatal("budget_exhausted must not be conflated with semantic failure classes")
	}
	if result.Cycles != 2 || result.ModelTurns != 2 {
		t.Fatalf("cycles/model_turns = %d/%d, want 2/2", result.Cycles, result.ModelTurns)
	}
	// 止损前的最后一批仍先触发 T1（⑥→⑦ 顺序）。
	if got := countTurnID(exitFake.events, "run-budget:cycle:2"); got != 1 {
		t.Fatalf("T1 before budget stop count = %d, want 1", got)
	}
	if got := countTurnID(exitFake.events, "run-budget"); got != 1 {
		t.Fatalf("T2 count = %d, want 1", got)
	}
	if !strings.Contains(result.Error, "budget_exhausted") {
		t.Fatalf("error must carry the stop-loss line, got %q", result.Error)
	}
}

// budget 超限 probe 账户腿（逐笔计量 + 越线止损）。
func TestPullLoopBudgetExhaustedOnProbeAccount(t *testing.T) {
	tools := &fakeTools{}
	llmFake := &fakeLLM{responses: []string{"call:probe.render:r1", "call:probe.render:r2"}}
	loop := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Tools:  tools,
		Exit:   &fakeExit{},
		Budget: ObservationBudget{MaxProbeCost: 1.0},
	}

	result := loop.Run(context.Background(), goalInput("run-probe"))

	if result.Outcome != OutcomeBudgetExhausted {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeBudgetExhausted)
	}
	if result.ProbeSpent != 1.0 {
		t.Fatalf("probe spent = %v, want 1.0 (0.5 per batch)", result.ProbeSpent)
	}
	if !strings.Contains(result.Error, "probe") {
		t.Fatalf("error must attribute the probe account, got %q", result.Error)
	}

	// 入场即越线（续跑承接的既有花费）：首批后立即止损。
	loop2 := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"call:probe.render:r1"}},
		Prefix: &fakePrefix{},
		Tools:  &fakeTools{},
		Budget: ObservationBudget{ProbeCost: 2.0, MaxProbeCost: 1.0},
	}
	if result := loop2.Run(context.Background(), goalInput("run-probe-seed")); result.Outcome != OutcomeBudgetExhausted {
		t.Fatalf("seeded over-line run outcome = %q, want budget_exhausted", result.Outcome)
	}
}

// 溢出 fail-closed：复用 ModelContextOverflow 既有面——越线即终态，
// 请求永不发出（LLM fake 零调用、工具面零执行）。
func TestPullLoopOverflowFailsClosedWithoutRequest(t *testing.T) {
	llmFake := &fakeLLM{responses: []string{"unreachable"}}
	tools := &fakeTools{}
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    llmFake,
		Prefix: &fakePrefix{},
		Tools:  tools,
		Exit:   exitFake,
	}
	in := goalInput("run-overflow")
	in.ContextSnapshotJSON = `{"context_overflow":{"status":"overflow","hot_bytes":999,"hot_budget_bytes":100,"warm_bytes":0,"warm_budget_bytes":0,"cold_ref":"evidence://cold"}}`

	result := loop.Run(context.Background(), in)

	if llmFake.calls != 0 {
		t.Fatalf("fail-closed must never send a request, got %d LLM calls", llmFake.calls)
	}
	if tools.executeCalls != 0 {
		t.Fatalf("fail-closed must not execute tools, got %d batches", tools.executeCalls)
	}
	if result.Outcome != OutcomeContextOverflow {
		t.Fatalf("outcome = %q, want %q", result.Outcome, OutcomeContextOverflow)
	}
	if got := countTurnID(exitFake.events, "run-overflow"); got != 1 {
		t.Fatalf("fail-closed terminal must fire T2 once, got %d", got)
	}
}

// 终态分类缝：nil=judgment_ok 缺省；缝值透传（push 既有口径接线归 D）。
func TestPullLoopClassifyTerminalSeam(t *testing.T) {
	loop := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"查遍候选库，没有可行动作"}},
		Prefix: &fakePrefix{},
	}
	if result := loop.Run(context.Background(), goalInput("run-classify-default")); result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("nil classifier must default to judgment_ok, got %q", result.Outcome)
	}

	loop2 := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"工程权限不足，无法执行"}},
		Prefix: &fakePrefix{},
	}
	in := goalInput("run-classify-blocked")
	in.ClassifyTerminal = func(responseText string) string {
		if strings.Contains(responseText, "权限不足") {
			return OutcomeCapabilityBlocked
		}
		return OutcomeJudgmentOK
	}
	if result := loop2.Run(context.Background(), in); result.Outcome != OutcomeCapabilityBlocked {
		t.Fatalf("classifier seam must pass through, got %q", result.Outcome)
	}
}

// 装配面：PrefixService 冻结接口的消费形态（SessionKey 命名空间、动态区
// 披露、冷启动槽 A 阶段为空）。
func TestPullLoopAssemblyConsumesPrefixService(t *testing.T) {
	prefix := &fakePrefix{}
	llmFake := &fakeLLM{responses: []string{"done"}}
	loop := &PullLoop{LLM: llmFake, Prefix: prefix, Budget: ObservationBudget{MaxCycles: 5}}

	result := loop.Run(context.Background(), goalInput("run-asm"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome = %q", result.Outcome)
	}
	if prefix.last.SessionKey != "pullharness:run-asm" {
		t.Fatalf("session key = %q, want pullharness:run-asm", prefix.last.SessionKey)
	}
	if len(prefix.last.SystemSections) != 0 {
		t.Fatalf("A-stage cold-start slot must be empty, got %d sections", len(prefix.last.SystemSections))
	}
	// 模型实际收到的消息：动态区 user 段含目标与预算披露。
	if llmFake.calls != 1 || len(llmFake.gotMsgs[0]) == 0 {
		t.Fatalf("expected one model call with messages, got %d", llmFake.calls)
	}
	last := llmFake.gotMsgs[0][len(llmFake.gotMsgs[0])-1]
	if last.Role != "user" {
		t.Fatalf("dynamic zone must be the trailing user message, got role %q", last.Role)
	}
	if !strings.Contains(last.Content, "goal: 演示工程里给主唱轨做淡入") {
		t.Fatalf("dynamic zone missing goal line: %q", last.Content)
	}
	if !strings.Contains(last.Content, "budget_state: cycles=0/5") {
		t.Fatalf("dynamic zone missing budget disclosure: %q", last.Content)
	}
}

// 注入面缺省：Prefix nil 用缺省实现；LLM nil 是显式基础设施失败而非 panic；
// Tools nil → 首轮模型响应即终态。
func TestPullLoopInjectionDefaults(t *testing.T) {
	loop := &PullLoop{LLM: &fakeLLM{responses: []string{"done"}}}
	if result := loop.Run(context.Background(), goalInput("run-defaults")); result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("nil prefix must fall back to default PrefixService, got outcome %q", result.Outcome)
	}

	noLLM := &PullLoop{Prefix: &fakePrefix{}}
	result := noLLM.Run(context.Background(), goalInput("run-nollm"))
	if result.Outcome != OutcomeLLMError || result.Error == "" {
		t.Fatalf("nil LLM must be llm_error with visible error, got %+v", result)
	}

	loop3 := &PullLoop{
		LLM:    &fakeLLM{responses: []string{"直接给出结论"}},
		Prefix: &fakePrefix{},
		Exit:   &fakeExit{},
	}
	result3 := loop3.Run(context.Background(), goalInput("run-notools"))
	if result3.Outcome != OutcomeJudgmentOK || result3.Cycles != 0 || result3.ModelTurns != 1 {
		t.Fatalf("nil tools run mismatch: %+v", result3)
	}
}

// LLM 基础设施失败（非 ctx 取消）：独立分类 + T2 照常（终局漏斗）。
func TestPullLoopLLMErrorFiresT2(t *testing.T) {
	exitFake := &fakeExit{}
	loop := &PullLoop{
		LLM:    &fakeLLM{forceError: errors.New("gateway 502")},
		Prefix: &fakePrefix{},
		Exit:   exitFake,
	}
	result := loop.Run(context.Background(), goalInput("run-llmerr"))
	if result.Outcome != OutcomeLLMError {
		t.Fatalf("outcome = %q, want llm_error", result.Outcome)
	}
	if got := countTurnID(exitFake.events, "run-llmerr"); got != 1 {
		t.Fatalf("T2 count = %d, want 1", got)
	}
}
