package agentloop

// pull_entry.go — 双模入口路由（L1-5-IMPL-D 腿3，HARNESS_V1_DESIGN §6/§11.5）。
//
// flag 面（A 卡已落 mode.go）：env VIT_DAW_HARNESS > agent_runtime_config.json
// 键 harness_mode > 缺省 push。入口三方法（Start/Continue/ResumeAfterConfirmation）
// 在头部单点分流：缺省 push=旧路径零变化（既有全量回归断言）；pull=经
// pullSession 驱动 PullLoop。chat 入口不迁移（OQ-H3：归 G3 裁定后切换卡）。
//
// 恢复边界（§11.3-4 裁定 4）：
//   - 同进程：活 pullSession 经登记面按 GoalID 找回（活状态不重建）；
//   - 跨进程（活会话丢失+continuation 带 pull checkpoint）：显式拒绝并报
//     产品边界，不重放已执行动作；
//   - 旧 continuation（无 pull 字段）：push 语义 fail-open，走 legacy 路径
//     （零行为变化）。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/pullharness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// pullHarnessModeResolver 是模式解析缝（缺省=env+config 真实面；测试注入）。
// 每次入口调用解析一次（会话启动读取语义；chat 每 goal run 一次，成本可忽略）。
var pullHarnessModeResolver = pullharness.ResolveHarnessMode

// pullHarnessEnabled 报告当前会话是否走 pull 模式入口。
func pullHarnessEnabled() bool {
	return pullHarnessModeResolver().Mode == pullharness.ModePull
}

// startPull 是 pull 模式的 Start 入口：与 push Start 同构的身份/预算前导
// （buildStartState 共用），然后经 pullSession 驱动 PullLoop。
func (l *MessageLoop) startPull(ctx context.Context, in Input) Result {
	r := l.runner()
	state := l.buildStartState(r, in)
	return l.runPull(ctx, r, state, nil, nil)
}

// continuePull 是 pull 模式的 Continue 入口。返回 handled=false 表示该
// continuation 是旧 push 载体（fail-open 走 legacy 路径）。
func (l *MessageLoop) continuePull(ctx context.Context, cont Continuation) (Result, bool) {
	if cont.PullCheckpoint == nil {
		// 旧 continuation 无 pull 字段=push 语义 fail-open（§11.3-4）。
		return Result{}, false
	}
	session, ok := pullSessionLookup(cont.GoalID)
	if !ok {
		// 跨进程 pull checkpoint：v1 同进程边界——显式拒绝，不重放已执行动作。
		return pullCrossProcessRejection(l, cont), true
	}
	if err := session.resumeContinue(cont); err != nil {
		r := l.runner()
		r.lifecycleDraft = false
		return r.fail(session.state, err), true
	}
	return l.runPullSession(ctx, session, nil), true
}

// resumeConfirmationPull 是 pull 模式的确认恢复入口：确认授权经既有
// ResumeAfterConfirmation 语义（Confirmed=true 只在已有确认授权后传入），
// 回到同一 pull 会话。handled=false=旧 push 载体 fail-open。
func (l *MessageLoop) resumeConfirmationPull(ctx context.Context, cont Continuation) (Result, bool) {
	if cont.PullCheckpoint == nil {
		return Result{}, false
	}
	session, ok := pullSessionLookup(cont.GoalID)
	if !ok {
		return pullCrossProcessRejection(l, cont), true
	}
	if err := session.resumeContinue(cont); err != nil {
		r := l.runner()
		r.lifecycleDraft = false
		return r.fail(session.state, err), true
	}
	pre := func(ctx context.Context) (pullharness.Step, bool) {
		return session.executeConfirmed(ctx, cont)
	}
	return l.runPullSession(ctx, session, pre), true
}

func pullCrossProcessRejection(l *MessageLoop, cont Continuation) Result {
	r := l.runner()
	state := &runState{input: Input{GoalID: cont.GoalID, RunID: cont.RunID}, goal: r.ensureGoal(cont.GoalID, cont.RunID, cont.Summary)}
	err := fmt.Errorf("pull continuation cross-process restore is unsupported in v1 (same-process only); no executed actions were replayed (goal=%s)", cont.GoalID)
	res := r.fail(state, err)
	res.Error = err.Error()
	return res
}

// runPull 构造宿主会话并驱动 PullLoop（Start 路径；saved=nil）。
func (l *MessageLoop) runPull(ctx context.Context, r *Runner, state *runState, saved *PullContinuation, pre func(context.Context) (pullharness.Step, bool)) Result {
	if l == nil || l.Client == nil {
		return r.fail(state, fmt.Errorf("agent message loop LLM client is nil"))
	}
	if !l.Config.Complete() {
		return r.fail(state, fmt.Errorf("agent message loop LLM config incomplete"))
	}
	session, err := newPullSession(l, r, state, saved)
	if err != nil {
		r.lifecycleDraft = false
		return r.fail(state, err)
	}
	return l.runPullSession(ctx, session, pre)
}

// runPullSession 驱动已构造的宿主会话：注册登记面→可选确认前导→PullLoop
// Run→终局判定（HostResult 有提交=返回；session_error=入口兜底真实失败）。
func (l *MessageLoop) runPullSession(ctx context.Context, session *pullSession, pre func(context.Context) (pullharness.Step, bool)) Result {
	pullSessionRegister(session.state.goal.GoalID, session)
	if pre != nil {
		if step, stopped := pre(ctx); stopped {
			if _, err := session.Return(ctx, step); err != nil {
				return session.entryFallback(fmt.Errorf("pull resume return failed: %w", err))
			}
			return session.HostResult()
		}
	}
	result := l.drivePullLoop(ctx, session)
	if result.Outcome == pullharness.OutcomeSessionError {
		// 协议违规 fail-closed 兜底：真实失败提交（draft 关闭——会话状态
		// 已不可信，唯一安全动作=终局失败+留痕）。
		return session.entryFallback(errors.New(result.Error))
	}
	if session.hostRes.Status != "" {
		host := session.HostResult()
		if host.Status == agentruntime.StatusCompleted || host.Status == agentruntime.StatusFailed ||
			host.Status == agentruntime.StatusStopped || host.Status == agentruntime.StatusCancelled {
			pullSessionRelease(session.state.goal.GoalID)
		}
		return host
	}
	return session.entryFallback(fmt.Errorf("pull run ended without host commit: outcome=%s", result.Outcome))
}

// drivePullLoop 装配 PullLoop（LLM/Prefix/Exit/ColdStart/Budget 全注入）并 Run。
func (l *MessageLoop) drivePullLoop(ctx context.Context, session *pullSession) pullharness.Result {
	state := session.state
	exit := pullharness.NewWiredExitExecutor(contextruntime.ExitExecutorConfig{
		Now: l.Now,
	})
	budget, _ := pullObservationBudgetFromState(state)
	loop := &pullharness.PullLoop{
		LLM:     l.Client,
		Prefix:  l.pullPrefixService(),
		Exit:    exit,
		Session: session,
		Budget:  budget,
		ColdStart: pullharness.ColdStartSourceFunc(func() map[string]any {
			engine, _ := state.input.Context["engine_snapshot"].(map[string]any)
			return engine
		}),
	}
	snapshot := session.r.buildContextSnapshot(state)
	state.contextSnapshot = snapshot.Map()
	in := pullharness.GoalInput{
		GoalID:              state.goal.GoalID,
		RunID:               state.goal.RunID,
		UserText:            state.input.UserText,
		Context:             cloneMap(state.input.Context),
		Conversation:        append([]llm.Message(nil), state.input.Conversation...),
		ContextSnapshotJSON: session.r.buildModelContextSnapshot(state, snapshot),
		Engine:              l.Config,
	}
	return loop.Run(ctx, in)
}

// entryFallback 是入口层兜底失败（真实提交，draft 关闭）。
func (s *pullSession) entryFallback(err error) Result {
	s.r.lifecycleDraft = false
	res := s.r.fail(s.state, err)
	pullSessionRelease(s.state.goal.GoalID)
	return res
}

// resumeContinue 在同一活会话上执行 Continue 前导（身份重绑+切片预算），
// 活状态（trace/plan/pending/executionMemory/ledger）不重建。
func (s *pullSession) resumeContinue(cont Continuation) error {
	r := s.r
	if r.Runtime == nil {
		return fmt.Errorf("goal runtime is nil")
	}
	if err := s.validateResume(cont); err != nil {
		return err
	}
	baseBudget := normalizeBudget(firstNonZeroBudget(cont.Budget, s.l.Budget))
	goal := r.Runtime.Continue(s.state.goal.GoalID, cont.Summary)
	if strings.TrimSpace(cont.Summary) != "" {
		goal.Summary = strings.TrimSpace(cont.Summary)
	}
	goal = r.Runtime.ClearInterjections(goal.GoalID)
	if opened, slice, ok := r.Runtime.BeginSlice(goal.GoalID, baseBudget.MaxTurns, baseBudget.MaxToolCalls); ok {
		goal = opened
		s.state.input.SliceID = slice.SliceID
		if goal.Task != nil {
			s.state.input.TaskID = goal.Task.TaskID
			s.state.input.OriginalIntent = goal.Task.OriginalIntent
		}
		if openedGoal, turn, turnOK := r.Runtime.BeginTurn(goal.GoalID, slice.SliceID, "automatic_continuation"); turnOK {
			goal = openedGoal
			s.state.input.TurnID = turn.TurnID
		}
	}
	s.state.goal = goal
	s.state.input.ResumedFromID = cont.ContinuationID
	if strings.TrimSpace(cont.UserText) != "" {
		s.state.input.UserText = cont.UserText
	}
	if len(cont.Context) > 0 {
		s.state.input.Context = cloneMap(cont.Context)
	}
	if strings.TrimSpace(cont.CatalogSummary) != "" {
		s.state.input.CatalogSummary = cont.CatalogSummary
	}
	if len(cont.AllowedTools) > 0 {
		s.state.input.AllowedTools = append([]string(nil), cont.AllowedTools...)
	}
	if len(cont.PendingToolQueue) > 0 {
		s.state.pendingToolQueue = append([]planner.ToolCall(nil), cont.PendingToolQueue...)
	}
	s.state.budget = extendContinuationBudget(baseBudget, s.state.turnsUsed, s.state.toolCallsUsed)
	s.state.continuationBudget = baseBudget
	s.state.startedAt = r.now()
	return nil
}

// validateResume 校验恢复载体（版本/绑定；§11.3-4 fail 边界）。
func (s *pullSession) validateResume(cont Continuation) error {
	if cont.PullCheckpoint == nil {
		return nil // 旧载体：入口已按 push fail-open 分流，此处不达
	}
	return cont.PullCheckpoint.ValidateFor(s.state.goal.GoalID, s.state.goal.RunID, runProjectDirFromState(s.state))
}

// executeConfirmed 执行已确认的 pending 工具（既有 ResumeAfterConfirmation
// 语义：Confirmed=true 只在已有确认授权后传入；守卫/预算检查同型）。
// stopped=true 时返回待提交步（经 Return 提交）。
func (s *pullSession) executeConfirmed(ctx context.Context, cont Continuation) (pullharness.Step, bool) {
	if cont.PendingToolCall == nil {
		step, _ := s.stepFromDraft(s.r.fail(s.state, fmt.Errorf("confirmation continuation is missing pending tool call")), pullharness.StepSourceModel, "")
		return step, true
	}
	if stopped, result := s.r.checkpoint("before_confirmed_tool", s.state); stopped {
		step, _ := s.stepFromDraft(result, pullharness.StepSourceModel, "")
		return step, true
	}
	if limit, result := s.r.checkToolBudget(s.state); limit {
		step, _ := s.stepFromDraft(result, pullharness.StepSourceModel, "")
		return step, true
	}
	call := *cont.PendingToolCall
	call = coerceMixObservationCall(s.state, call)
	call = coerceMixTickPrimitiveCall(s.state, call)
	if issue := messageLoopToolGuardIssue(s.state, call, messageLoopHasUsableMixObservation(s.state)); issue != "" {
		result := planner.ToolResult{ToolCallID: stableToolCallID(call, s.state.completedSteps+1), Tool: call.Tool, Status: "error", Error: issue}
		s.state.trace = append(s.state.trace,
			planner.TraceEvent{Kind: "tool_call", ToolCall: &call},
			planner.TraceEvent{Kind: "tool_result", ToolResult: &result},
			planner.TraceEvent{Kind: "final_gate", Message: issue},
		)
		s.batchStart = len(s.state.executed)
		return pullharness.Step{}, false // 守卫拒绝：回主循环（Attempt 面继续）
	}
	s.batchStart = len(s.state.executed)
	if stopped, result := s.r.executeTool(ctx, s.state, call, true, nil); stopped {
		step, _ := s.stepFromDraft(result, pullharness.StepSourceModel, "")
		return step, true
	}
	return pullharness.Step{}, false
}
