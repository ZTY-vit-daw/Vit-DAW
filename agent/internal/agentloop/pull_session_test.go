package agentloop

// pull_session_test.go — 五不可丢语义验收（HARNESS_V1_DESIGN §11.4 冻结为
// D 验收测试集；ADAPTER 提案 §4 断言口径：查实际消息/revision/pending/
// 账本/receipt，不只计调用次数）+三轴映射 14 行（§11.2）+Session 面契约。

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/fastpath"
	"vit-daw-agent/internal/pullharness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// pullTestExecutor 是 pull 会话测试的执行 fake（receipt/确认面脚本）。
type pullTestExecutor struct {
	calls []struct {
		tool      string
		confirmed bool
	}
	requires map[string]bool
}

func (f *pullTestExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	f.calls = append(f.calls, struct {
		tool      string
		confirmed bool
	}{in.ToolCall.Tool, in.Confirmed})
	if f.requires != nil && f.requires[strings.TrimSpace(in.ToolCall.Tool)] && !in.Confirmed {
		return executorpkg.Result{
			ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok",
			RequiresConfirmation: true, Preview: "pull preview", UndoLabel: "pull undo",
			Result: map[string]any{"proposal": true},
		}, nil
	}
	return executorpkg.Result{
		ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "ok",
		Result: map[string]any{"echo": true},
	}, nil
}

func (f *pullTestExecutor) executions() int { return len(f.calls) }
func (f *pullTestExecutor) applies() int { // 确认后执行次数（Confirmed=true）
	count := 0
	for _, call := range f.calls {
		if call.confirmed {
			count++
		}
	}
	return count
}

// newPullTestSession 构造真实 MessageLoop/Runner/Runtime +直构 runState 的
// pull 会话（push 入口的 BeginSlice/BeginTurn 身份面同构）。
func newPullTestSession(t *testing.T, exec *pullTestExecutor, contextMods func(map[string]any)) (*pullSession, *MessageLoop) {
	t.Helper()
	rt := loopTestRuntime()
	l := &MessageLoop{
		Runtime:  rt,
		Client:   &fakeMessageCompleter{},
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 12, MaxConsecutiveErrors: 2},
		Now:      func() time.Time { return time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC) },
	}
	r := l.runner()
	goal := r.ensureGoal("goal-pull", "run-pull", "pull session test")
	baseBudget := normalizeBudget(l.Budget)
	state := &runState{
		input: Input{
			GoalID: goal.GoalID, RunID: goal.RunID, UserText: "pull session test",
			Context: map[string]any{},
			Budget:  baseBudget,
		},
		goal:               goal,
		budget:             baseBudget,
		continuationBudget: baseBudget,
		startedAt:          r.now(),
	}
	if opened, slice, ok := rt.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
		state.goal = opened
		state.input.SliceID = slice.SliceID
		if openedGoal, turn, turnOK := rt.BeginTurn(goal.GoalID, slice.SliceID, "user"); turnOK {
			state.goal = openedGoal
			state.input.TurnID = turn.TurnID
		}
	}
	if contextMods != nil {
		contextMods(state.input.Context)
	}
	session, err := newPullSession(l, r, state, nil)
	if err != nil {
		t.Fatalf("newPullSession: %v", err)
	}
	return session, l
}

// testRouterWith 构造计数/可编程测试 Router（miss 副作用回流与 diagnostic
// 旁路断言面；生产注册面完整性由 newFastPathRouter 契约另锁）。
func testRouterWith(handler fastpath.Handler[*Runner, *runState, Result]) *fastpath.Router[*Runner, *runState, Result] {
	return fastpath.NewRouter(fastpath.Entry[*Runner, *runState, Result]{
		Name: "test_probe_entry", Handler: handler,
	})
}

// ---- 五不可丢语义（§11.4）----

// 语义 1 miss 副作用回流：miss（false+Result{}）带状态副作用时，副作用经
// Frame 回流（Revision 递增+pack 可见），工具不加倍，Attempt 的 Continue 不
// 携带 Calls。
func TestPullSessionMissSideEffectsFlowBack(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	handlerCalls := 0
	session.router = testRouterWith(func(ctx context.Context, r *Runner, state *runState) (bool, Result) {
		handlerCalls++
		state.input.Context["capability_context_pack"] = map[string]any{"pack": "gain-staging-facts-v3"}
		return false, Result{}
	})

	frame0, err := session.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	step, err := session.Attempt(context.Background())
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if step.Disposition != pullharness.DispositionContinue || len(step.Calls) != 0 {
		t.Fatalf("miss step = %+v, want continue without calls", step)
	}
	frame1, err := session.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if frame1.Revision <= frame0.Revision {
		t.Errorf("revision after miss-with-side-effects = %d, want > %d", frame1.Revision, frame0.Revision)
	}
	if pack, ok := frame1.Context["capability_context_pack"].(map[string]any); !ok || pack["pack"] != "gain-staging-facts-v3" {
		t.Errorf("pack not visible in Frame.Context (miss side effects must flow back): %+v", frame1.Context["capability_context_pack"])
	}
	if exec.executions() != 0 || session.ledger.toolAttempts != 0 {
		t.Errorf("miss must not execute tools: executions=%d toolAttempts=%d", exec.executions(), session.ledger.toolAttempts)
	}
	if _, err := session.Attempt(context.Background()); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if exec.executions() != 0 {
		t.Errorf("repeated miss doubled tool execution: %d", exec.executions())
	}
	if handlerCalls != 2 {
		t.Errorf("handler calls = %d, want 2 (one per attempt)", handlerCalls)
	}
}

// 语义 2 confirmation 零 T2：批中确认暂停——proposal 执行一次、apply=0；
// pending 完整保留进 PullContinuation；Runtime 无终局提交（Complete=0）、
// retain=0；EndTurn 恰一。
func TestPullSessionConfirmationPauseZeroT2(t *testing.T) {
	exec := &pullTestExecutor{requires: map[string]bool{"pull.confirm.tool": true}}
	session, _ := newPullTestSession(t, exec, nil)

	step, err := session.Interpret(context.Background(),
		`{"final":false,"reply":"proposing","tool_calls":[{"tool":"pull.confirm.tool","args":{}}]}`)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if step.Disposition != pullharness.DispositionContinue || len(step.Calls) != 1 {
		t.Fatalf("interpret step = %+v, want continue with 1 call", step)
	}
	step, err = session.Execute(context.Background(), step.Calls)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if step.Disposition != pullharness.DispositionSuspend || step.Kind != pullharness.ReturnConfirmation {
		t.Fatalf("execute step = %+v, want suspend/confirmation", step)
	}
	if _, err := session.Return(context.Background(), step); err != nil {
		t.Fatalf("return: %v", err)
	}
	// proposal 恰执行一次（未确认），apply=0。
	if exec.executions() != 1 || exec.applies() != 0 {
		t.Errorf("proposal/executed=%d applied=%d, want 1/0", exec.executions(), exec.applies())
	}
	// pending 完整保留（checkpoint 载体）。
	checkpoint := session.lastCheckpoint
	if checkpoint == nil || checkpoint.Continuation == nil || checkpoint.Continuation.PendingToolCall == nil {
		t.Fatalf("confirmation checkpoint missing pending tool: %+v", checkpoint)
	}
	if checkpoint.Continuation.PendingToolCall.Tool != "pull.confirm.tool" {
		t.Errorf("pending tool = %q, want pull.confirm.tool", checkpoint.Continuation.PendingToolCall.Tool)
	}
	// Runtime 无终局提交；状态=waiting_confirmation。
	goal := session.l.Runtime.Status(session.state.goal.GoalID)
	if goal.Status != agentruntime.StatusWaitingConfirmation {
		t.Errorf("goal status = %q, want waiting_confirmation", goal.Status)
	}
	// retain=0：无 exit_retain 留痕（终局 retain 面零触发）。
	for _, event := range session.hostRes.Trace {
		if event.Kind == "exit_retain" {
			t.Error("confirmation pause fired exit retain (must be zero T2)")
		}
	}
	// EndTurn 恰一（同 slice）。
	if turns := runtimeTurnCount(t, session.l.Runtime, session.state.goal.GoalID); turns != 1 {
		t.Errorf("EndTurn count = %d, want exactly 1", turns)
	}
}

// 语义 3 已执行工具不重放：receipt 按 ID 单次结算（恢复面不重复计费/计数）；
// Attempt 的 Continue 永不携带 Calls（已执行面以 ReceiptIDs 表达）。
func TestPullSessionExecutedToolsNeverReplayed(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)

	step, err := session.Interpret(context.Background(),
		`{"final":false,"reply":"work","tool_calls":[{"tool":"pull.echo","args":{}}]}`)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	step, err = session.Execute(context.Background(), step.Calls)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if step.Disposition != pullharness.DispositionContinue || len(step.ReceiptIDs) != 1 {
		t.Fatalf("execute step = %+v, want continue with 1 receipt", step)
	}
	first := step.ReceiptIDs[0]
	if session.ledger.toolAttempts != 1 || !session.ledger.settled[first] {
		t.Fatalf("receipt %q not settled (attempts=%d)", first, session.ledger.toolAttempts)
	}
	// 同 receipt 再结算：零新增（单次结算契约）。
	if fresh := session.ledger.settle([]string{first, first}); len(fresh) != 0 {
		t.Errorf("re-settling receipt %q added %d new entries", first, len(fresh))
	}
	if session.ledger.toolAttempts != 1 {
		t.Errorf("tool attempts = %d after re-settle, want 1", session.ledger.toolAttempts)
	}
	// CloseCycle 后批回执进 T1 单元；再 CloseCycle 同 BatchID 幂等。
	if err := session.CloseCycle(context.Background(), "run-pull:cycle:1", nil); err != nil {
		t.Fatalf("close cycle: %v", err)
	}
	if err := session.CloseCycle(context.Background(), "run-pull:cycle:1", nil); err != nil {
		t.Fatalf("close cycle (idempotent): %v", err)
	}
	if session.ledger.completedCycles != 1 {
		t.Errorf("completed cycles = %d after duplicate close, want 1", session.ledger.completedCycles)
	}
}

// 语义 4 终局唯一 owner：terminal commit=1（Runtime.Complete 恰一）+EndTurn
// 恰一+retain 恰一；重复 Return 同 DraftID 幂等返回原提交结果、零新操作；
// 已终局后新 Return 拒绝。
func TestPullSessionTerminalSoleOwnerAndIdempotentReturn(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)

	step, err := session.Interpret(context.Background(), `{"final":true,"reply":"done"}`)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if step.Disposition != pullharness.DispositionTerminal || step.Kind != pullharness.ReturnDone {
		t.Fatalf("interpret step = %+v, want terminal/done", step)
	}
	if step.DraftID == "" {
		t.Fatal("terminal step missing draft handle")
	}
	result1, err := session.Return(context.Background(), step)
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	result2, err := session.Return(context.Background(), step)
	if err != nil {
		t.Fatalf("idempotent return: %v", err)
	}
	if result1.Outcome != result2.Outcome || result1.Reply != result2.Reply {
		t.Errorf("repeat Return diverged: %+v vs %+v", result1, result2)
	}
	goal := session.l.Runtime.Status(session.state.goal.GoalID)
	if goal.Status != agentruntime.StatusCompleted {
		t.Errorf("goal status = %q, want completed (commit exactly once)", goal.Status)
	}
	if turns := runtimeTurnCount(t, session.l.Runtime, session.state.goal.GoalID); turns != 1 {
		t.Errorf("EndTurn count = %d, want exactly 1", turns)
	}
	// 已终局：无 DraftID 的新 Return 拒绝（不逆转历史）。
	if _, err := session.Return(context.Background(), pullharness.Step{
		Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnInterrupted,
	}); err == nil {
		t.Error("post-terminal Return accepted (history must not reverse)")
	}
	// 终局后 goal 不再开放续片。
	if session.hostRes.Continuation != nil {
		t.Error("terminal result carries continuation (must be nil)")
	}
}

// 语义 5 每轮同源 diagnostic 刷新：旁路判定每轮从单一事实源读取——
// diagnostic_only=true 时零 handler 调用（root 键）；解除后下一轮恢复。
func TestPullSessionDiagnosticOnlyRefreshSameSourcePerAttempt(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, func(ctx map[string]any) {
		ctx["diagnostic_only"] = true // root 键（nested 键同源判定在 messageLoopFreeStateDiagnosticOnly）
	})
	handlerCalls := 0
	session.router = testRouterWith(func(ctx context.Context, r *Runner, state *runState) (bool, Result) {
		handlerCalls++
		return false, Result{}
	})
	if _, err := session.Attempt(context.Background()); err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if handlerCalls != 0 {
		t.Fatalf("diagnostic-only bypass failed: handler calls = %d, want 0", handlerCalls)
	}
	// 解除（root 键）：下一轮恢复尝试。
	delete(session.state.input.Context, "diagnostic_only")
	if _, err := session.Attempt(context.Background()); err != nil {
		t.Fatalf("attempt after lift: %v", err)
	}
	if handlerCalls != 1 {
		t.Errorf("handler calls after lift = %d, want 1 (per-round same-source refresh)", handlerCalls)
	}
}

// ---- Router 预算契约（§11.3.5）----

// 预算用尽：入场拒绝（Router 零成本无豁免——Attempt 在 TryMatch 之前执法）；
// 映射旧 failed+专用 StopReason observation_budget_exhausted。
func TestPullSessionBudgetExhaustedEntryDenial(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, func(ctx map[string]any) {
		ctx["pull_observation_budget"] = map[string]any{"max_cycles": 1}
	})
	session.ledger.completedCycles = 1 // 已用尽
	step, err := session.Attempt(context.Background())
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if step.Disposition != pullharness.DispositionTerminal || step.Kind != pullharness.ReturnObservationBudgetExhausted {
		t.Fatalf("step = %+v, want terminal/observation_budget_exhausted", step)
	}
	result, err := session.Return(context.Background(), step)
	if err != nil {
		t.Fatalf("return: %v", err)
	}
	if result.Outcome != pullharness.OutcomeBudgetExhausted {
		t.Errorf("outcome = %q, want budget_exhausted", result.Outcome)
	}
	if session.hostRes.StopReason != StopReasonObservationBudgetExhausted {
		t.Errorf("stop reason = %q, want %q (专用 StopReason，不冒充 limit_reached)",
			session.hostRes.StopReason, StopReasonObservationBudgetExhausted)
	}
	if session.hostRes.Status != agentruntime.StatusFailed {
		t.Errorf("status = %q, want failed (§11.3.1 映射)", session.hostRes.Status)
	}
}

// unknown 成本语义：有工具回执但无计量源时账本标 unknown（不填 0 冒充免费）。
func TestPullSessionUnknownProbeCostNotZero(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	step, _ := session.Interpret(context.Background(),
		`{"final":false,"reply":"work","tool_calls":[{"tool":"pull.echo","args":{}}]}`)
	if _, err := session.Execute(context.Background(), step.Calls); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if session.ledger.probeCostKnown {
		t.Error("probe cost marked known without a metering source (unknown must not impersonate free/zero)")
	}
	frame, _ := session.Snapshot(context.Background())
	if frame.Budget.ProbeCostKnown {
		t.Error("LedgerView discloses ProbeCostKnown=true after unknown-cost settlement")
	}
}

// ---- ctx 中断与用户取消分离（§11.2）----

func TestPullSessionCtxInterruptVsUserCancelSeparated(t *testing.T) {
	exec := &pullTestExecutor{}
	// ctx 中断：SuspendInterrupted（非终局，checkpoint 存续）。
	session, _ := newPullTestSession(t, exec, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	step, err := session.Attempt(ctx)
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if step.Disposition != pullharness.DispositionSuspend || step.Kind != pullharness.ReturnInterrupted {
		t.Fatalf("ctx-cancel step = %+v, want suspend/interrupted", step)
	}
	if _, err := session.Return(ctx, step); err != nil {
		t.Fatalf("return: %v", err)
	}
	if session.hostRes.Status != agentruntime.StatusWaitingContinue || session.hostRes.StopReason != StopReasonInterrupted {
		t.Errorf("interrupt commit = %s/%s, want waiting_continue/interrupted (pause surface)", session.hostRes.Status, session.hostRes.StopReason)
	}
	if session.lastCheckpoint == nil {
		t.Error("interrupt lost checkpoint (continuation must survive)")
	}

	// 显式用户取消：TerminalCancelled（终局，与 ctx 中断严格分离）。
	session2, _ := newPullTestSession(t, exec, nil)
	session2.l.Runtime.RequestStop(session2.state.goal.GoalID, "user_stop")
	step2, err := session2.Attempt(context.Background())
	if err != nil {
		t.Fatalf("attempt: %v", err)
	}
	if step2.Disposition != pullharness.DispositionTerminal || step2.Kind != pullharness.ReturnUserCancelled {
		t.Fatalf("user-cancel step = %+v, want terminal/user_cancelled", step2)
	}
	if _, err := session2.Return(context.Background(), step2); err != nil {
		t.Fatalf("return: %v", err)
	}
	if session2.hostRes.Status != agentruntime.StatusStopped {
		t.Errorf("user cancel commit = %q, want stopped (terminal, distinct from ctx pause)", session2.hostRes.Status)
	}
}

// ---- 三轴映射 14 行（§11.2 纯函数面）----

func TestStepFromLegacyResultMappingTable(t *testing.T) {
	cases := []struct {
		name           string
		result         Result
		disposition    pullharness.Disposition
		kind           pullharness.ReturnKind
		wantMappingErr bool
	}{
		{"completed_done", Result{Status: agentruntime.StatusCompleted, StopReason: StopReasonDone}, pullharness.DispositionTerminal, pullharness.ReturnDone, false},
		{"failed_failed", Result{Status: agentruntime.StatusFailed, StopReason: StopReasonFailed, Error: "x"}, pullharness.DispositionTerminal, pullharness.ReturnFailed, false},
		{"failed_protocol", Result{Status: agentruntime.StatusFailed, StopReason: StopReasonModelProtocolFailure}, pullharness.DispositionTerminal, pullharness.ReturnFailed, false},
		{"failed_free_state_fallback", Result{Status: agentruntime.StatusFailed, StopReason: FreeStateTerminalFallbackStopReason}, pullharness.DispositionTerminal, pullharness.ReturnFailed, false},
		{"failed_budget", Result{Status: agentruntime.StatusFailed, StopReason: StopReasonObservationBudgetExhausted}, pullharness.DispositionTerminal, pullharness.ReturnObservationBudgetExhausted, false},
		{"waiting_confirmation", Result{Status: agentruntime.StatusWaitingConfirmation, StopReason: StopReasonNeedsConfirmation}, pullharness.DispositionSuspend, pullharness.ReturnConfirmation, false},
		{"waiting_clarification", Result{Status: agentruntime.StatusWaitingClarification, StopReason: StopReasonNeedsClarification}, pullharness.DispositionSuspend, pullharness.ReturnClarification, false},
		{"slice_limit", Result{Status: agentruntime.StatusWaitingContinue, StopReason: StopReasonLimitReached}, pullharness.DispositionSuspend, pullharness.ReturnSliceLimit, false},
		{"interjection", Result{Status: agentruntime.StatusWaitingContinue, StopReason: StopReasonInterjection}, pullharness.DispositionSuspend, pullharness.ReturnInterjection, false},
		{"transient", Result{Status: agentruntime.StatusWaitingContinue, StopReason: StopReasonTransientLLMError}, pullharness.DispositionSuspend, pullharness.ReturnTransient, false},
		{"interrupted", Result{Status: agentruntime.StatusWaitingContinue, StopReason: StopReasonInterrupted}, pullharness.DispositionSuspend, pullharness.ReturnInterrupted, false},
		{"stopped", Result{Status: agentruntime.StatusStopped, StopReason: StopReasonCancelled}, pullharness.DispositionTerminal, pullharness.ReturnUserCancelled, false},
		{"cancelled", Result{Status: agentruntime.StatusCancelled, StopReason: StopReasonCancelled}, pullharness.DispositionTerminal, pullharness.ReturnUserCancelled, false},
		{"process_snapshot_running", Result{Status: agentruntime.StatusRunning}, pullharness.DispositionContinue, 0, false},
		{"process_snapshot_stable", Result{Status: agentruntime.StatusStable}, pullharness.DispositionContinue, 0, false},
		// fail-closed：未知 Status/StopReason 组合。
		{"unknown_status", Result{Status: agentruntime.GoalStatus("mystery")}, 0, 0, true},
		{"failed_unknown_reason", Result{Status: agentruntime.StatusFailed, StopReason: "mystery"}, 0, 0, true},
		{"waiting_continue_unknown_reason", Result{Status: agentruntime.StatusWaitingContinue, StopReason: "mystery"}, 0, 0, true},
		{"waiting_confirmation_unknown_reason", Result{Status: agentruntime.StatusWaitingConfirmation, StopReason: "mystery"}, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			step, err := stepFromLegacyResult(tc.result)
			if tc.wantMappingErr {
				if err == nil {
					t.Fatalf("mapping accepted unknown combo: %+v", step)
				}
				return
			}
			if err != nil {
				t.Fatalf("mapping failed: %v", err)
			}
			if step.Disposition != tc.disposition || step.Kind != tc.kind {
				t.Errorf("step = %s/%s, want %s/%s", step.Disposition, step.Kind, tc.disposition, tc.kind)
			}
		})
	}
}

// completed≠judgment_ok：无结构化证据时 Outcome 留空（不按回复关键词造成功）；
// free-state 结构化判定才产分类。
func TestPullSessionOutcomeAxisIndependent(t *testing.T) {
	exec := &pullTestExecutor{}
	session, _ := newPullTestSession(t, exec, nil)
	step, err := session.Interpret(context.Background(), `{"final":true,"reply":"完成了"}`)
	if err != nil {
		t.Fatalf("interpret: %v", err)
	}
	if step.Outcome != "" {
		t.Errorf("terminal outcome = %q, want empty (pending classification; completed≠judgment_ok)", step.Outcome)
	}
	result, _ := session.Return(context.Background(), step)
	if result.Outcome == pullharness.OutcomeJudgmentOK {
		t.Error("projection auto-classified completed as judgment_ok (outcome axis must stay independent)")
	}
}

// ---- draft 策略与 push 零变化 ----

// push Runner（draft 关闭）保持旧行为：complete 即 Runtime.Complete——
// draft 策略只经 newPullSession 启用。
func TestDraftPolicyPullOnly(t *testing.T) {
	rt := loopTestRuntime()
	r := &Runner{Runtime: rt, Now: func() time.Time { return time.Now() }}
	goal := r.ensureGoal("goal-draft", "run-draft", "test")
	state := &runState{input: Input{GoalID: goal.GoalID, RunID: goal.RunID}, goal: goal,
		budget: normalizeBudget(Budget{}), continuationBudget: normalizeBudget(Budget{}), startedAt: r.now()}
	r.complete(state, "done")
	if got := rt.Status(goal.GoalID).Status; got != agentruntime.StatusCompleted {
		t.Errorf("push complete immediate status = %q, want completed (draft off = old behavior)", got)
	}
}

func runtimeTurnCount(t *testing.T, rt *agentruntime.Runtime, goalID string) int {
	t.Helper()
	goal := rt.Status(goalID)
	if goal.Task == nil {
		return 0
	}
	return len(goal.Task.Run.Turns)
}
