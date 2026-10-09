package pullharness

// loop_test.go — Session 驱动循环验收（L1-5-IMPL-D 腿1：A 占位级测试随
// §11.1 授权同步改写——ToolExecutor/FastPathRouter 占位面退役，改经
// fakeSession 锚定 Session 协议契约）：
//   - CloseCycle 每完整批恰一次，批 ID=<RunID>:cycle:<NextCycle> 序列；
//   - 驱动不自行触发任何边界事件（T1 经 CloseCycle 传宿主；T2=宿主 Return
//     提交——S1 形态 §11.1）；
//   - 暂停/中断面零 CloseCycle、零边界事件；
//   - 预算用尽=宿主 Terminal/ObservationBudgetExhausted（ledger authority）
//     → Result.Outcome=budget_exhausted 独立分类；
//   - 溢出 fail-closed：请求永不发出（零 LLM 调用）；
//   - 装配消费 Frame（会话+协议段+冷启动底座）与 PrefixService 会话键。

import (
	"context"
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
		return "model fact", ctx.Err()
	}
	if f.calls > len(f.responses) {
		return "", fmt.Errorf("fakeLLM: script exhausted at call %d", f.calls)
	}
	return f.responses[f.calls-1], nil
}

// fakeSession 是 Session 六方法的测试宿主：账本/会话/批窗口的极简权威面，
// 各方法行为可经函数字段覆写。默认行为=miss→模型批→回执→CloseCycle。
type fakeSession struct {
	goalID     string
	runID      string
	prefixKey  string
	context    map[string]any
	budget     LedgerView
	budgetCaps ObservationBudget // 宿主执法声明面（Attempt 入场拒绝）

	conversation []llm.Message

	attemptFn    func(*fakeSession, context.Context) (Step, error)
	interpretFn  func(*fakeSession, context.Context, string) (Step, error)
	executeFn    func(*fakeSession, context.Context, []ToolCall) (Step, error)
	closeCycleFn func(*fakeSession, context.Context, string, contextruntime.ExitExecutor) error
	returnFn     func(*fakeSession, context.Context, Step) (Result, error)

	// 记录面（断言用）。
	attemptCalls    int
	snapshotCalls   int
	interpretCalls  int
	executeCalls    int
	closeCalls      int
	returnCalls     int
	closedBatchIDs  []string
	returnedSteps   []Step
	executedBatches [][]ToolCall
	interpretInputs []string
	exit            *fakeExit
	protocolPrompt  string
}

func (f *fakeSession) Snapshot(context.Context) (Frame, error) {
	f.snapshotCalls++
	f.budget.NextCycle = uint64(f.budget.CompletedCycles + 1)
	contextClone := map[string]any{}
	for key, value := range f.context {
		contextClone[key] = value
	}
	if f.protocolPrompt != "" {
		contextClone[FrameContextProtocolPrompt] = f.protocolPrompt
	}
	return Frame{
		Revision:         uint64(f.snapshotCalls),
		GoalID:           f.goalID,
		RunID:            f.runID,
		PrefixSessionKey: f.prefixKey,
		Context:          contextClone,
		Conversation:     append([]llm.Message(nil), f.conversation...),
		Budget:           f.budget,
	}, nil
}

// Attempt 默认行为：预算入场拒绝（ledger authority）+ 快路径 miss。
func (f *fakeSession) Attempt(ctx context.Context) (Step, error) {
	f.attemptCalls++
	if f.attemptFn != nil {
		return f.attemptFn(f, ctx)
	}
	if f.budgetCaps.CycleExhausted(f.budget.CompletedCycles) ||
		f.budgetCaps.ProbeExhausted(f.budget.ProbeSpent) {
		return Step{Disposition: DispositionTerminal, Kind: ReturnObservationBudgetExhausted,
			Source: StepSourceFastPath, Error: "budget_exhausted: entry denial"}, nil
	}
	return Step{Disposition: DispositionContinue, Source: StepSourceFastPath}, nil
}

// Interpret 默认行为：归档 assistant 消息；"call:<tool>:<id>" 行协议→待执行
// 批；无调用行→终局 Done。
func (f *fakeSession) Interpret(ctx context.Context, raw string) (Step, error) {
	f.interpretCalls++
	f.interpretInputs = append(f.interpretInputs, raw)
	f.budget.ModelCalls++
	f.conversation = append(f.conversation, llm.Message{Role: "assistant", Content: raw})
	if f.interpretFn != nil {
		return f.interpretFn(f, ctx, raw)
	}
	var calls []ToolCall
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.Split(strings.TrimSpace(line), ":")
		if len(parts) == 3 && parts[0] == "call" {
			calls = append(calls, ToolCall{ID: parts[2], Tool: parts[1]})
		}
	}
	if len(calls) == 0 {
		return Step{Disposition: DispositionTerminal, Kind: ReturnDone,
			Source: StepSourceModel, Reply: raw, Outcome: OutcomeJudgmentOK}, nil
	}
	return Step{Disposition: DispositionContinue, Source: StepSourceModel, Calls: calls}, nil
}

// Execute 默认行为：批执行（回执）+模型行回喂会话；ModelLine 回喂模拟
// miss 副作用回流可见面。
func (f *fakeSession) Execute(ctx context.Context, calls []ToolCall) (Step, error) {
	f.executeCalls++
	f.executedBatches = append(f.executedBatches, append([]ToolCall(nil), calls...))
	if f.executeFn != nil {
		return f.executeFn(f, ctx, calls)
	}
	receipts := make([]string, 0, len(calls))
	lines := make([]string, 0, len(calls))
	for _, call := range calls {
		receipts = append(receipts, call.ID)
		lines = append(lines, "tool "+call.ID+" ok")
	}
	f.conversation = append(f.conversation, llm.Message{Role: "user", Content: strings.Join(lines, "\n")})
	return Step{Disposition: DispositionContinue, Source: StepSourceModel, ReceiptIDs: receipts}, nil
}

// CloseCycle 默认行为：用注入 Exit 构造 T1（BatchTurnEvent）+账本完成计数。
func (f *fakeSession) CloseCycle(ctx context.Context, batchID string, exit contextruntime.ExitExecutor) error {
	f.closeCalls++
	f.closedBatchIDs = append(f.closedBatchIDs, batchID)
	if f.closeCycleFn != nil {
		return f.closeCycleFn(f, ctx, batchID, exit)
	}
	batch := make([]ToolResult, 0, len(f.executedBatches[len(f.executedBatches)-1]))
	for _, call := range f.executedBatches[len(f.executedBatches)-1] {
		batch = append(batch, ToolResult{ID: call.ID, Tool: call.Tool, Status: "ok",
			Bytes: 100, ModelLine: "tool " + call.ID + " ok"})
	}
	if exit != nil && f.exit != nil {
		event, _, _ := BatchTurnEvent(batchID, batch, ExitWiring{})
		exit.OnTurnBoundary(ctx, event)
	}
	f.budget.CompletedCycles++
	return nil
}

// Return 默认行为：终局唯一 owner（记录步；结果按 Kind 投影）。
func (f *fakeSession) Return(ctx context.Context, step Step) (Result, error) {
	f.returnCalls++
	f.returnedSteps = append(f.returnedSteps, step)
	if f.returnFn != nil {
		return f.returnFn(f, ctx, step)
	}
	outcome := step.Outcome
	if outcome == "" {
		switch step.Kind {
		case ReturnDone:
			outcome = OutcomeJudgmentOK
		case ReturnFailed:
			outcome = "failed"
		case ReturnInterrupted:
			outcome = OutcomeInterrupted
		case ReturnTransient:
			outcome = OutcomeLLMError
		case ReturnObservationBudgetExhausted:
			outcome = OutcomeBudgetExhausted
		default:
			outcome = step.Kind.String()
		}
	}
	return Result{
		RunID:        f.runID,
		GoalID:       f.goalID,
		Outcome:      outcome,
		Reply:        step.Reply,
		Error:        step.Error,
		Cycles:       f.budget.CompletedCycles,
		ModelTurns:   f.budget.ModelCalls,
		ProbeSpent:   f.budget.ProbeSpent,
		Conversation: append([]llm.Message(nil), f.conversation...),
		Trace:        []string{"host_return: kind=" + step.Kind.String()},
	}, nil
}

type fakeExit struct {
	events []contextruntime.TurnBoundaryEvent
}

func (f *fakeExit) OnTurnBoundary(ctx context.Context, ev contextruntime.TurnBoundaryEvent) contextruntime.ExitReport {
	f.events = append(f.events, ev)
	return contextruntime.ExitReport{}
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

func newSessionLoop(session *fakeSession, llmClient llm.Completer, prefix promptruntime.PrefixService, exit contextruntime.ExitExecutor) *PullLoop {
	return &PullLoop{
		LLM:     llmClient,
		Prefix:  prefix,
		Exit:    exit,
		Session: session,
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

// CloseCycle 每完整批恰一次，批 ID 序列=<RunID>:cycle:<n>；驱动不自行触发
// 边界事件（fakeSession 经注入 Exit 构造 T1——生产同型，宿主唯一入口）。
func TestPullLoopCloseCycleOncePerBatchWithBatchIDSequence(t *testing.T) {
	llmFake := &fakeLLM{responses: []string{
		"call:ref.query:q1",
		"call:ref.diff:d1",
		"done",
	}}
	session := &fakeSession{runID: "run-batch-seq", goalID: "goal-1", exit: &fakeExit{}}
	exit := &fakeExit{}
	session.exit = exit
	loop := newSessionLoop(session, llmFake, &fakePrefix{}, exit)
	result := loop.Run(context.Background(), goalInput("run-batch-seq"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q, want judgment_ok (result=%+v)", result.Outcome, result)
	}
	if len(session.closedBatchIDs) != 2 ||
		session.closedBatchIDs[0] != "run-batch-seq:cycle:1" ||
		session.closedBatchIDs[1] != "run-batch-seq:cycle:2" {
		t.Errorf("closed batches=%v, want [run-batch-seq:cycle:1 run-batch-seq:cycle:2]", session.closedBatchIDs)
	}
	if len(exit.events) != 2 || countTurnID(exit.events, "run-batch-seq:cycle:1") != 1 {
		t.Errorf("T1 events=%+v, want exactly one per completed batch", exit.events)
	}
	if session.budget.CompletedCycles != 2 || result.Cycles != 2 {
		t.Errorf("completed cycles host=%d result=%d, want 2/2", session.budget.CompletedCycles, result.Cycles)
	}
	if llmFake.calls != 3 || session.budget.ModelCalls != 3 {
		t.Errorf("model calls=%d host=%d, want 3/3", llmFake.calls, session.budget.ModelCalls)
	}
}

// 终局（Interpret 无工具调用轮）与暂停面：零新增 CloseCycle、零边界事件；
// 驱动不自行发 T2（宿主 Return 提交——S1 §11.1）。
func TestPullLoopTerminalAndPauseSurfaceNeverFireBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		step      Step
		wantCalls int // 期望 CloseCycle 次数（0；终局前无完整批）
	}{
		{"terminal_from_attempt", Step{Disposition: DispositionTerminal, Kind: ReturnDone, Source: StepSourceFastPath, Reply: "routed"}, 0},
		{"suspend_confirmation_from_attempt", Step{Disposition: DispositionSuspend, Kind: ReturnConfirmation, Source: StepSourceFastPath}, 0},
		{"suspend_interrupted_from_interpret", Step{Disposition: DispositionSuspend, Kind: ReturnInterrupted, Source: StepSourceModel}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := &fakeSession{runID: "run-" + tc.name, goalID: "goal-1"}
			exit := &fakeExit{}
			session.exit = exit
			origin := "attempt"
			session.attemptFn = func(f *fakeSession, ctx context.Context) (Step, error) {
				if origin == "interpret" {
					return Step{Disposition: DispositionContinue, Source: StepSourceFastPath}, nil
				}
				return tc.step, nil
			}
			session.interpretFn = func(f *fakeSession, ctx context.Context, raw string) (Step, error) {
				return tc.step, nil
			}
			var llmClient llm.Completer = &fakeLLM{}
			if tc.name == "suspend_interrupted_from_interpret" {
				origin = "interpret"
				llmClient = &fakeLLM{responses: []string{"model reply"}}
			}
			loop := newSessionLoop(session, llmClient, &fakePrefix{}, exit)
			result := loop.Run(context.Background(), goalInput("run-"+tc.name))
			if session.closeCalls != tc.wantCalls {
				t.Errorf("CloseCycle=%d, want %d", session.closeCalls, tc.wantCalls)
			}
			if len(exit.events) != 0 {
				t.Errorf("boundary events fired: %+v", exit.events)
			}
			if session.returnCalls != 1 || len(session.returnedSteps) != 1 {
				t.Fatalf("Return calls=%d, want exactly 1 (host sole terminal owner)", session.returnCalls)
			}
			if got := session.returnedSteps[0]; got.Disposition != tc.step.Disposition || got.Kind != tc.step.Kind {
				t.Errorf("returned step=%+v, want disposition/kind of %+v", got, tc.step)
			}
			if result.Outcome == "" {
				t.Error("host Result projection missing outcome")
			}
		})
	}
}

// 预算用尽：宿主 ledger 入场拒绝（Attempt Terminal/ObservationBudgetExhausted）
// → 独立分类 budget_exhausted（不混 no_candidate_found/切片暂停）。
func TestPullLoopBudgetExhaustedFromHostLedger(t *testing.T) {
	session := &fakeSession{runID: "run-budget", goalID: "goal-1", exit: &fakeExit{}}
	session.budgetCaps = ObservationBudget{MaxCycles: 1}
	// 第一轮：模型批+CloseCycle 后账本 CompletedCycles=1；第二轮 Attempt
	// 入场拒绝。
	session.interpretFn = func(f *fakeSession, ctx context.Context, raw string) (Step, error) {
		if f.budget.CompletedCycles == 0 {
			return Step{Disposition: DispositionContinue, Source: StepSourceModel,
				Calls: []ToolCall{{ID: "probe-1", Tool: "ref.query"}}}, nil
		}
		return Step{Disposition: DispositionTerminal, Kind: ReturnDone, Source: StepSourceModel}, nil
	}
	exit := &fakeExit{}
	session.exit = exit
	loop := newSessionLoop(session, &fakeLLM{responses: []string{"call:ref.query:probe-1", "x"}}, &fakePrefix{}, exit)
	result := loop.Run(context.Background(), goalInput("run-budget"))
	if result.Outcome != OutcomeBudgetExhausted {
		t.Fatalf("outcome=%q, want budget_exhausted (result=%+v)", result.Outcome, result)
	}
	if session.closeCalls != 1 {
		t.Errorf("CloseCycle=%d, want 1 (first batch completed before denial)", session.closeCalls)
	}
	if session.returnedSteps[0].Kind != ReturnObservationBudgetExhausted {
		t.Errorf("step kind=%v, want observation_budget_exhausted", session.returnedSteps[0].Kind)
	}
}

// 溢出 fail-closed：装配字节越线（宿主披露线位）→ 请求永不发出（零 LLM
// 调用）、零 CloseCycle；实际装配计量（非入场静态快照）。
func TestPullLoopOverflowFailsClosedWithoutRequest(t *testing.T) {
	llmFake := &fakeLLM{responses: []string{"should never be requested"}}
	session := &fakeSession{runID: "run-overflow", goalID: "goal-1"}
	session.context = map[string]any{FrameContextContextBudgetBytes: 8} // 极小线位
	session.protocolPrompt = strings.Repeat("protocol ", 8)
	exit := &fakeExit{}
	session.exit = exit
	loop := newSessionLoop(session, llmFake, &fakePrefix{}, exit)
	result := loop.Run(context.Background(), goalInput("run-overflow"))
	if llmFake.calls != 0 {
		t.Fatalf("LLM calls=%d, want 0 (fail-closed before request)", llmFake.calls)
	}
	if session.closeCalls != 0 || len(exit.events) != 0 {
		t.Errorf("CloseCycle=%d events=%v, want 0/none", session.closeCalls, exit.events)
	}
	if result.Outcome != OutcomeContextOverflow {
		t.Errorf("outcome=%q, want context_overflow", result.Outcome)
	}
	if session.returnCalls != 1 || session.returnedSteps[0].Disposition != DispositionTerminal {
		t.Errorf("overflow must commit via host Return: calls=%d steps=%+v", session.returnCalls, session.returnedSteps)
	}
}

// 装配消费面：Frame.Conversation（宿主权威会话）+宿主协议段+冷启动底座
// Section 族+PrefixSessionKey（P1 判据会话键）。
func TestPullLoopAssemblyConsumesFrameAndPrefixService(t *testing.T) {
	prefix := &fakePrefix{}
	session := &fakeSession{runID: "run-assembly", goalID: "goal-1", prefixKey: "pullharness:run-assembly"}
	session.protocolPrompt = "PROTOCOL: return strict JSON"
	session.conversation = []llm.Message{{Role: "user", Content: "prior turn"}}
	llmFake := &fakeLLM{responses: []string{"final answer"}}
	loop := newSessionLoop(session, llmFake, prefix, &fakeExit{})
	result := loop.Run(context.Background(), goalInput("run-assembly"))
	if result.Outcome != OutcomeJudgmentOK {
		t.Fatalf("outcome=%q (result=%+v)", result.Outcome, result)
	}
	if prefix.last.SessionKey != "pullharness:run-assembly" {
		t.Errorf("SessionKey=%q, want host-provided prefix session key", prefix.last.SessionKey)
	}
	if len(prefix.last.SystemSections) == 0 || len(prefix.last.UserSections) == 0 {
		t.Fatalf("assembly sections missing: system=%d user=%d", len(prefix.last.SystemSections), len(prefix.last.UserSections))
	}
	joined := ""
	for _, message := range llmFake.gotMsgs[0] {
		joined += message.Content + "\n"
	}
	if !strings.Contains(joined, "PROTOCOL: return strict JSON") {
		t.Error("host protocol prompt not mounted into model messages")
	}
	if !strings.Contains(joined, "prior turn") {
		t.Error("Frame.Conversation (host authoritative) not mounted into model messages")
	}
	if !strings.Contains(joined, "budget_state:") {
		t.Error("dynamic zone budget disclosure missing from model messages")
	}
}

// 注入缺省：无 Session/无 Prefix（生产非 nil 要求）→ fail-closed session_error，
// 零宿主调用。
func TestPullLoopInjectionDefaults(t *testing.T) {
	session := &fakeSession{runID: "run-inject", goalID: "goal-1"}
	if result := (&PullLoop{LLM: &fakeLLM{}, Prefix: &fakePrefix{}}).Run(context.Background(), goalInput("run-inject")); result.Outcome != OutcomeSessionError {
		t.Errorf("no-session outcome=%q, want session_error", result.Outcome)
	}
	loop := newSessionLoop(session, &fakeLLM{}, nil, nil)
	if result := loop.Run(context.Background(), goalInput("run-inject")); result.Outcome != OutcomeSessionError {
		t.Errorf("nil-prefix outcome=%q, want session_error (fail-closed)", result.Outcome)
	}
	if session.attemptCalls != 0 {
		t.Errorf("protocol failure must not reach host: attempts=%d", session.attemptCalls)
	}
	loop.AllowNilPrefix = true // 测试豁免面（生产构造禁止）
	if result := loop.Run(context.Background(), goalInput("run-inject")); result.Outcome == OutcomeSessionError {
		t.Errorf("exempted nil-prefix still rejected: %+v", result)
	}
}

// LLM 基础设施失败：驱动零裁量交宿主（Suspend/Transient 提示）——真实暂停
// vs 终局失败由宿主 Return 按自身状态裁定（fake 映射 llm_error）。
func TestPullLoopLLMErrorHandsOffToHost(t *testing.T) {
	session := &fakeSession{runID: "run-llm-err", goalID: "goal-1"}
	exit := &fakeExit{}
	session.exit = exit
	loop := newSessionLoop(session, &fakeLLM{forceError: fmt.Errorf("connection reset")}, &fakePrefix{}, exit)
	result := loop.Run(context.Background(), goalInput("run-llm-err"))
	if session.returnCalls != 1 {
		t.Fatalf("Return calls=%d, want 1", session.returnCalls)
	}
	step := session.returnedSteps[0]
	if step.Disposition != DispositionSuspend || step.Kind != ReturnTransient {
		t.Errorf("step=%+v, want suspend/transient handoff", step)
	}
	if !strings.Contains(step.Error, "connection reset") {
		t.Errorf("llm error lost: %q", step.Error)
	}
	if session.closeCalls != 0 || len(exit.events) != 0 {
		t.Errorf("no boundary on llm error: close=%d events=%v", session.closeCalls, exit.events)
	}
	if result.Outcome != OutcomeLLMError {
		t.Errorf("outcome=%q, want llm_error (host projection)", result.Outcome)
	}
}

// Session 协议违规：Attempt 的 Continue 携带 Calls（已执行面不得伪装未执行
// 批）→ fail-closed session_error，零 Return（宿主状态未提交）。
func TestPullLoopRejectsAttemptCarryingCalls(t *testing.T) {
	session := &fakeSession{runID: "run-proto", goalID: "goal-1"}
	session.attemptFn = func(f *fakeSession, ctx context.Context) (Step, error) {
		return Step{Disposition: DispositionContinue, Source: StepSourceFastPath,
			Calls: []ToolCall{{ID: "x", Tool: "existing.tool"}}}, nil
	}
	loop := newSessionLoop(session, &fakeLLM{}, &fakePrefix{}, nil)
	result := loop.Run(context.Background(), goalInput("run-proto"))
	if result.Outcome != OutcomeSessionError {
		t.Fatalf("outcome=%q, want session_error", result.Outcome)
	}
	if session.returnCalls != 0 || session.snapshotCalls != 0 {
		t.Errorf("protocol violation must fail before host commit: returns=%d snapshots=%d", session.returnCalls, session.snapshotCalls)
	}
}

// 未知 Disposition 拒绝（§11.1 封闭枚举）。
func TestPullLoopRejectsUnknownDisposition(t *testing.T) {
	session := &fakeSession{runID: "run-unknown", goalID: "goal-1"}
	session.attemptFn = func(f *fakeSession, ctx context.Context) (Step, error) {
		return Step{Disposition: Disposition(99), Kind: ReturnDone, Source: StepSourceFastPath}, nil
	}
	loop := newSessionLoop(session, &fakeLLM{}, &fakePrefix{}, nil)
	if result := loop.Run(context.Background(), goalInput("run-unknown")); result.Outcome != OutcomeSessionError {
		t.Errorf("outcome=%q, want session_error (unknown disposition rejected)", result.Outcome)
	}
}
