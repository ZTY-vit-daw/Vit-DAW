package agentloop

// pull_session.go — pullharness.Session 的宿主适配（L1-5-IMPL-D 腿2，
// HARNESS_V1_DESIGN §11.1 S1 + ADAPTER 提案 §2/§3）。
//
// 单一 state owner：本会话独家持有 *runState/Runner/MessageLoop 与十快路径
// handler；pull 驱动只经有版本只读 Frame 工作。三类事实的权威归属：
//   - 状态/会话/trace/pending：宿主活 state（每次 Attempt/Interpret/Execute
//     返回后重新 Snapshot；模型消息先经 Interpret 归档一次）；
//   - 工具回执/成本/计数：宿主 ledger（累计实际消费唯一 authority，§11.3.5；
//     每真实工具按稳定 receipt ID 单次结算，Route hit/miss 均守；计量缺失
//     标 unknown 不填 0；越线记录 overshoot 后终局）；
//   - 生命周期提交：Return 唯一 owner（终局 commit=1+retain hook=1+同 slice
//     EndTurn=1；重复 Return 同 DraftID 幂等；Suspend=保存 checkpoint+EndTurn
//     一次、零 retain）。
//
// draft 返回策略（提案 §3.2"实施必要条件"）：r.lifecycleDraft=true 时
// complete/fail/pause/checkpoint 的 Runtime 提交与 result 的 EndTurn/retain
// 延后，由 commitDraft 在 Return 重放（见 runner.go 各分支 gate）。
//
// 三轴映射（§11.2 冻结 14 行）见 stepFromLegacyResult：completed≠judgment_ok
//（Outcome 轴独立，无结构化证据不造成功）；EndTurn=宿主唯一 owner；ctx 中断
//（SuspendInterrupted）与显式用户取消（TerminalCancelled）严格分离，同时
// 到达先读 runtime flags 按用户取消终局处理；未知 Status/StopReason=fail-closed
// 适配错误（零新工具，保留原数据）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/contextruntime/carriers"
	"vit-daw-agent/internal/fastpath"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/internal/pullharness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// pullLedger 是宿主账本（§11.3.5：累计实际消费唯一 authority；pull 只读披露）。
type pullLedger struct {
	nextCycle       uint64
	completedCycles int
	modelCalls      int
	toolAttempts    int
	probeSpent      float64
	probeCostKnown  bool
	maxCycles       int
	maxProbeCost    float64
	settled         map[string]bool // receipt ID → 已结算（单次结算面）
	overshoot       bool            // 越线已记录（不假称硬预授权）
}

func (l *pullLedger) exhausted() bool {
	if l.maxCycles > 0 && l.completedCycles >= l.maxCycles {
		return true
	}
	if l.maxProbeCost > 0 && l.probeCostKnown && l.probeSpent >= l.maxProbeCost {
		return true
	}
	return false
}

func (l *pullLedger) view() pullharness.LedgerView {
	return pullharness.LedgerView{
		NextCycle:       l.nextCycle,
		CompletedCycles: l.completedCycles,
		ModelCalls:      l.modelCalls,
		ToolAttempts:    l.toolAttempts,
		ProbeSpent:      l.probeSpent,
		ProbeCostKnown:  l.probeCostKnown,
	}
}

// settle 对一批回执做单次结算：已结算的 receipt 不重复计费；返回本批新结算
// 的回执（顺序保持）。计数 toolAttempts 与 probe 分开；未知成本（现执行面
// 无统一 ProbeCost 字段）不填 0——probeCostKnown=false。
func (l *pullLedger) settle(receipts []string) []string {
	fresh := make([]string, 0, len(receipts))
	for _, receipt := range receipts {
		if receipt == "" || l.settled[receipt] {
			continue
		}
		l.settled[receipt] = true
		l.toolAttempts++
		fresh = append(fresh, receipt)
	}
	return fresh
}

type pullDraftRecord struct {
	result Result
	source string
	entry  string
}

// pullSession 实现 pullharness.Session（六方法）。非并发安全：一个 run 一个
// 会话，按 push 同构的串行假设使用。
type pullSession struct {
	l      *MessageLoop
	r      *Runner
	state  *runState
	router *fastpath.Router[*Runner, *runState, Result]

	ledger   pullLedger
	revision uint64

	drafts            map[string]pullDraftRecord
	draftSeq          int
	committedResults  map[string]Result // DraftID → 已提交结果（Return 幂等面）
	hostRes           Result            // 最近一次提交的完整 legacy Result
	committedTerminal bool              // 终局是否已提交（同 run 二次终局拒绝）
	lastCheckpoint    *PullContinuation // 最近一次 Suspend 的恢复载体

	lastOutput       messageLoopOutput // 当前待执行模型批的解释产物
	batchStart       int               // 当前批在 state.executed 的起点
	closed           map[string]bool   // batchID → T1 已提交（同批 T1<=1）
	prefixKey        string
	resumeGeneration int // 恢复载体单次消费代数（重复消费检测）
}

// newPullSession 构造宿主会话（单一 state owner）。saved 非 nil 时恢复账本
// （同进程恢复面，见 pull_continuation.go）。draft 策略在此启用（pull-only）。
func newPullSession(l *MessageLoop, r *Runner, state *runState, saved *PullContinuation) (*pullSession, error) {
	if l == nil || r == nil || state == nil {
		return nil, fmt.Errorf("pull session requires loop, runner and state")
	}
	s := &pullSession{
		l:                l,
		r:                r,
		state:            state,
		router:           l.newFastPathRouter(),
		drafts:           map[string]pullDraftRecord{},
		committedResults: map[string]Result{},
		closed:           map[string]bool{},
		prefixKey:        "pullharness:" + messageLoopPrefixSessionKey(state),
		ledger: pullLedger{
			probeCostKnown: true, // 缺省已知零成本；首个未知计量回执即翻 false
			settled:        map[string]bool{},
		},
	}
	if budget, ok := pullObservationBudgetFromState(state); ok {
		s.ledger.maxCycles = budget.MaxCycles
		s.ledger.maxProbeCost = budget.MaxProbeCost
	}
	if saved != nil && saved.Pull != nil {
		if err := s.restoreLedger(saved); err != nil {
			return nil, err
		}
	}
	// draft 返回策略：pull 会话独占本 Runner 实例（入口每 run 新建），push
	// 路径永不经此（缺省 false=完全旧行为）。
	r.lifecycleDraft = true
	return s, nil
}

// pullObservationBudgetFromState 从 run 上下文读观察预算声明（缺省不设线）。
// 键沿 ObservationBudget 语义（HARNESS_V1_DESIGN §3.4）：context 键
// "pull_observation_budget" 形如 {"max_cycles":N,"max_probe_cost":X}。
func pullObservationBudgetFromState(state *runState) (pullharness.ObservationBudget, bool) {
	if state == nil {
		return pullharness.ObservationBudget{}, false
	}
	raw, ok := state.input.Context["pull_observation_budget"]
	if !ok {
		return pullharness.ObservationBudget{}, false
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return pullharness.ObservationBudget{}, false
	}
	var budget pullharness.ObservationBudget
	if err := json.Unmarshal(data, &budget); err != nil {
		return pullharness.ObservationBudget{}, false
	}
	return budget, true
}

func (s *pullSession) restoreLedger(saved *PullContinuation) error {
	if saved.SchemaVersion != pullContinuationSchemaVersion {
		return fmt.Errorf("pull continuation schema version %d unsupported (want %d): rejected with data preserved", saved.SchemaVersion, pullContinuationSchemaVersion)
	}
	cp := saved.Pull
	s.ledger.nextCycle = cp.NextCycle
	s.ledger.completedCycles = cp.CompletedCycles
	s.ledger.modelCalls = cp.ModelCalls
	s.ledger.toolAttempts = cp.ToolAttempts
	s.ledger.probeSpent = cp.ProbeSpent
	s.ledger.probeCostKnown = cp.ProbeCostKnown
	s.ledger.maxCycles = cp.MaxCycles
	s.ledger.maxProbeCost = cp.MaxProbeCost
	s.ledger.settled = map[string]bool{}
	for _, receipt := range cp.SettledReceipts {
		s.ledger.settled[receipt] = true
	}
	for _, batchID := range cp.ClosedBatchIDs {
		s.closed[batchID] = true
	}
	return nil
}

// ---- pullharness.Session 实现 ----

// Snapshot 返回有版本只读视图。协议段（JSON 协议+目录+allowed）由宿主供给
// ——Interpret 复用 push 协议解析，模型必须收到同族协议指令。
func (s *pullSession) Snapshot(context.Context) (pullharness.Frame, error) {
	contextClone := map[string]any{}
	for key, value := range s.state.input.Context {
		contextClone[key] = value
	}
	contextClone[pullharness.FrameContextProtocolPrompt] = messageLoopSystemPrompt(s.state)
	return pullharness.Frame{
		Revision:         s.revision,
		GoalID:           s.state.goal.GoalID,
		RunID:            s.state.goal.RunID,
		PrefixSessionKey: s.prefixKey,
		Context:          contextClone,
		Conversation:     append([]llm.Message(nil), s.state.input.Conversation...),
		Budget:           s.ledger.view(),
	}, nil
}

// Attempt 执行宿主快路径面（l.loop 顶部同构，注册序/短路/diagnostic 旁路
// 语义保持）：预算入场拒绝（Router 零成本无豁免，§11.3.2）→ 用户取消/
// ctx 检查（先读 runtime flags——同时到达按用户取消终局）→ pending 队列
// （确认恢复后的余队列）→ diagnostic 刷新 → 十项 TryMatch。miss=Continue
// （无 Calls——handler 的工具请求已在宿主执行，副作用经 Frame 回流）。
func (s *pullSession) Attempt(ctx context.Context) (pullharness.Step, error) {
	s.revision++
	// 预算入场：已超限禁止新增工具/模型（零成本纯回复同样终局——统计一致）。
	if s.ledger.exhausted() {
		return s.synthesizeBudgetStep()
	}
	// 显式用户取消优先于 ctx 中断（§11.2 竞争口径）。
	if step, handled := s.userCancelStep(); handled {
		return step, nil
	}
	if ctx.Err() != nil {
		step, err := s.synthesizeInterruptStep("")
		return step, err
	}
	// pending 队列（push l.loop 顶部同构）。
	if len(s.state.pendingToolQueue) > 0 {
		queued := append([]planner.ToolCall(nil), s.state.pendingToolQueue...)
		if stopped, result := s.l.executeMessageLoopToolCalls(ctx, s.r, s.state, queued, messageLoopOutput{}, messageLoopHasUsableMixObservation(s.state)); stopped {
			return s.stepFromDraft(result, pullharness.StepSourceFastPath, "")
		}
	}
	// 每轮同源 diagnostic 刷新（判定源=messageLoopFreeStateDiagnosticOnly，
	// 单一事实源不变；root/nested 双键同源语义保持）。
	s.router.SetDiagnosticOnly(messageLoopFreeStateDiagnosticOnly(s.state))
	if stopped, result, entry := s.router.TryMatch(ctx, s.r, s.state); stopped {
		return s.stepFromDraft(result, pullharness.StepSourceFastPath, entry)
	}
	// miss：Continue 无 Calls（即使 miss 带副作用——Frame.Revision 已增，
	// 副作用经下一次 Snapshot 回流给装配）。
	return pullharness.Step{Disposition: pullharness.DispositionContinue, Source: pullharness.StepSourceFastPath}, nil
}

// userCancelStep 读取 runtime flags 构造显式用户取消终局步（与 ctx 中断
// 严格分离）。未取消返回 handled=false。
func (s *pullSession) userCancelStep() (pullharness.Step, bool) {
	goal := s.state.goal
	if s.r.Runtime != nil {
		goal = s.r.Runtime.Status(goal.GoalID)
	}
	if !goal.StopRequested && !goal.CancelRequested &&
		goal.Status != agentruntime.StatusStopped && goal.Status != agentruntime.StatusCancelled && goal.Status != agentruntime.StatusCancelling {
		return pullharness.Step{}, false
	}
	// 复用 checkpoint 的检测分支（draft 模式无 Runtime 副作用），映射终局。
	_, draft := s.r.checkpoint("pull_attempt_user_cancel", s.state)
	if draft.Status != agentruntime.StatusStopped && draft.Status != agentruntime.StatusCancelled {
		// 检测分支未终局（不应发生）——按 ctx 中断处理，不造终局。
		step, _ := s.synthesizeInterruptStep("")
		return step, true
	}
	step, err := s.stepFromDraft(draft, pullharness.StepSourceFastPath, "")
	if err != nil {
		fallback, _ := s.synthesizeInterruptStep("")
		return fallback, true
	}
	return step, true
}

// Interpret 解释一次模型原始回复：post-LLM 记账（turnsUsed/ModelCalls/
// StatusRunning，push 同型）→ 复用 interpretModelReply 完整链（解析/修复/
// 守卫/final gate/澄清——pull 与 push 共用同一实现，行为一致）。模型消息
// 在链内归档（含 final gate 反馈回喂）。
func (s *pullSession) Interpret(ctx context.Context, raw string) (pullharness.Step, error) {
	s.revision++
	s.state.turnsUsed++
	s.ledger.modelCalls++
	if s.r.Runtime != nil {
		s.state.goal = s.r.Runtime.SetStatus(s.state.goal.GoalID, agentruntime.StatusRunning, nil)
	}
	if ctx.Err() != nil {
		return s.synthesizeInterruptStep(raw)
	}
	flow, result, out := s.l.interpretModelReply(ctx, s.r, s.state, raw, "")
	switch flow {
	case modelFlowStopped:
		return s.stepFromDraft(result, pullharness.StepSourceModel, "")
	case modelFlowRetry:
		// final gate 反馈已回喂会话：Continue 无 Calls，驱动装配下一模型轮。
		return pullharness.Step{Disposition: pullharness.DispositionContinue, Source: pullharness.StepSourceModel}, nil
	default: // modelFlowExecute
		s.lastOutput = out
		s.batchStart = len(s.state.executed)
		calls := make([]pullharness.ToolCall, 0, len(out.ToolCalls))
		for _, call := range out.ToolCalls {
			converted, err := toolCallToPull(call)
			if err != nil {
				return pullharness.Step{}, fmt.Errorf("interpret: %w", err)
			}
			calls = append(calls, converted)
		}
		return pullharness.Step{
			Disposition: pullharness.DispositionContinue,
			Source:      pullharness.StepSourceModel,
			Calls:       calls,
		}, nil
	}
}

// Execute 经宿主同一执行 gateway 执行批（守卫/权限/verifier/executionMemory
// 面全在 l.executeMessageLoopToolCalls/r.executeTool 既有链内，零新工具）。
// 完整批=Continue+ReceiptIDs（gateway 单次结算）；批中暂停/终局=对应 Step
// （partial 不 CloseCycle）。批后确定性 fast-complete 检查与 push 尾段同型。
func (s *pullSession) Execute(ctx context.Context, calls []pullharness.ToolCall) (pullharness.Step, error) {
	s.revision++
	// 预算入场（每个实际工具批前）：Router 执行到限额须停止其下一个批。
	if s.ledger.exhausted() {
		return s.synthesizeBudgetStep()
	}
	if step, handled := s.userCancelStep(); handled {
		return step, nil
	}
	if ctx.Err() != nil {
		step, err := s.synthesizeInterruptStep("")
		return step, err
	}
	plannerCalls := make([]planner.ToolCall, 0, len(calls))
	for _, call := range calls {
		converted, err := toolCallFromPull(call)
		if err != nil {
			return pullharness.Step{}, fmt.Errorf("execute: %w", err)
		}
		plannerCalls = append(plannerCalls, converted)
	}
	if s.batchStart > len(s.state.executed) {
		s.batchStart = len(s.state.executed)
	}
	if stopped, result := s.l.executeMessageLoopToolCalls(ctx, s.r, s.state, plannerCalls, s.lastOutput, messageLoopHasUsableMixObservation(s.state)); stopped {
		return s.stepFromDraft(result, pullharness.StepSourceModel, "")
	}
	receipts := s.settleBatch()
	// push 尾段同型：确定性 fast-complete（工具结果直接了结本轮）。
	if reply, ok := messageLoopFastCompleteReply(s.state, s.lastOutput); ok {
		s.state.trace = append(s.state.trace, planner.TraceEvent{Kind: "final_gate", Message: "deterministic tool result completed the turn"})
		if question, ok := messageLoopFinalFocusTrackClarification(s.state, reply); ok {
			s.state.trace = append(s.state.trace, planner.TraceEvent{Kind: "clarification", Message: question})
			res := s.r.pause(s.state, agentruntime.StatusWaitingClarification, StopReasonNeedsClarification, "", question, "", "", nil)
			res.NeedsClarification = true
			res.ClarificationQuestion = question
			return s.stepFromDraft(res, pullharness.StepSourceModel, "")
		}
		return s.stepFromDraft(s.r.complete(s.state, messageLoopMixObservationFinalReply(s.state, reply)), pullharness.StepSourceModel, "")
	}
	return pullharness.Step{
		Disposition: pullharness.DispositionContinue,
		Source:      pullharness.StepSourceModel,
		ReceiptIDs:  receipts,
	}, nil
}

// settleBatch 结算自批起点以来的执行回执（单次结算；已结算不重复计费）。
// 现执行面无统一 probe 计量（提案 §5.1 取证点）——成本标 unknown，不填 0
// 冒充免费；有真实计量源后在此接入。
func (s *pullSession) settleBatch() []string {
	if s.batchStart >= len(s.state.executed) {
		return nil
	}
	records := s.state.executed[s.batchStart:]
	receipts := make([]string, 0, len(records))
	for _, record := range records {
		if id := strings.TrimSpace(fmt.Sprint(record["tool_call_id"])); id != "" {
			receipts = append(receipts, id)
		}
	}
	fresh := s.ledger.settle(receipts)
	if len(fresh) > 0 {
		s.ledger.probeCostKnown = false
	}
	return fresh
}

// CloseCycle 是 T1 唯一调用入口：完整模型工具批恰一次。用注入的 ExitExecutor
// 构造批事件（BatchTurnEvent，单元=批回执行回执）；同型维持 history refs
// 伴随索引（assembleWithReport 同款 RunTurnBoundaryHook）。同 BatchID 幂等。
func (s *pullSession) CloseCycle(ctx context.Context, batchID string, exit contextruntime.ExitExecutor) error {
	if s.closed[batchID] {
		return nil
	}
	s.closed[batchID] = true
	s.ledger.nextCycle++
	batch := s.batchToolResults()
	if exit != nil {
		event, _, _ := pullharness.BatchTurnEvent(batchID, batch, pullharness.ExitWiring{})
		exit.OnTurnBoundary(ctx, event)
	}
	contextruntime.RunTurnBoundaryHook(ctx, contextruntime.TurnBoundaryHookInput{
		SessionKey: s.prefixKey,
		TurnID:     batchID,
		History:    s.state.input.Conversation,
		ProjectDir: carriers.ResolveProjectDir(runProjectDirFromState(s.state)),
	})
	s.ledger.completedCycles++
	if s.ledger.exhausted() {
		s.ledger.overshoot = true // 记录越线事实（不假称硬预授权上限）
	}
	s.revision++
	return nil
}

// batchToolResults 从执行记录构造本批 T1 单元（ID=回执；Bytes=记录 JSON
// 长度的机械计量；ModelLine 不在 T1 单元重复渲染）。
func (s *pullSession) batchToolResults() []pullharness.ToolResult {
	if s.batchStart >= len(s.state.executed) {
		return nil
	}
	records := s.state.executed[s.batchStart:]
	batch := make([]pullharness.ToolResult, 0, len(records))
	for _, record := range records {
		id := strings.TrimSpace(fmt.Sprint(record["tool_call_id"]))
		tool := strings.TrimSpace(fmt.Sprint(record["tool"]))
		status := strings.TrimSpace(fmt.Sprint(record["status"]))
		data, _ := json.Marshal(record)
		batch = append(batch, pullharness.ToolResult{
			ID: id, Tool: tool, Status: status,
			Bytes: int64(len(data)),
		})
	}
	s.batchStart = len(s.state.executed)
	return batch
}

// Return 提交一次生命周期（终局唯一 owner）。同 DraftID 幂等返回原提交
// 结果；已终局后再收到非幂等 Return 拒绝（不逆转历史）。
func (s *pullSession) Return(ctx context.Context, step pullharness.Step) (pullharness.Result, error) {
	if !step.Disposition.Valid() {
		return pullharness.Result{}, fmt.Errorf("return: invalid disposition %d", step.Disposition)
	}
	if step.DraftID != "" {
		if cached, ok := s.committedResults[step.DraftID]; ok {
			return s.projection(cached, step), nil
		}
	}
	if s.committedTerminal {
		// 已终局：幂等 Return 已在上面的缓存分支返回；到达此处的新 Return
		// 拒绝（不逆转历史，只拒绝下一阶段）。
		return pullharness.Result{}, fmt.Errorf("return: terminal already committed; step (draft=%q) rejected (history not reversed)", step.DraftID)
	}
	draft, err := s.draftForStep(step)
	if err != nil {
		return pullharness.Result{}, err
	}
	if step.DraftID != "" {
		if _, ok := s.drafts[step.DraftID]; !ok {
			return pullharness.Result{}, fmt.Errorf("return: unknown draft handle %q", step.DraftID)
		}
	}
	committed := s.commitDraft(step, draft)
	s.committedResults[step.DraftID] = committed
	s.hostRes = committed
	return s.projection(committed, step), nil
}

// HostResult 返回最近一次提交的完整 legacy Result（入口层消费面）。
func (s *pullSession) HostResult() Result { return s.hostRes }

// draftForStep 定位/合成步对应的 draft Result：携带 DraftID=宿主已产 draft；
// 驱动自建步（中断/溢出/LLM/预算）按 Kind 合成（经 r.pause/r.result 的 draft
// 形态——无 Runtime 副作用）。
func (s *pullSession) draftForStep(step pullharness.Step) (Result, error) {
	if step.DraftID != "" {
		record, ok := s.drafts[step.DraftID]
		if !ok {
			return Result{}, fmt.Errorf("return: unknown draft handle %q", step.DraftID)
		}
		return record.result, nil
	}
	switch {
	case step.Disposition == pullharness.DispositionSuspend && step.Kind == pullharness.ReturnInterrupted:
		res := s.r.pause(s.state, agentruntime.StatusWaitingContinue, StopReasonInterrupted, "", strings.TrimSpace(step.Reply), "", "", nil)
		return res, nil
	case step.Disposition == pullharness.DispositionSuspend && step.Kind == pullharness.ReturnTransient:
		// free-state 判定在宿主（提案 §2 行 9 语义）；非 free-state 的 LLM
		// 失败=真实终局失败。
		if messageLoopFreeStateActive(s.state) {
			return s.r.pause(s.state, agentruntime.StatusWaitingContinue, StopReasonTransientLLMError, "", step.Error, "", "", nil), nil
		}
		return s.r.fail(s.state, fmt.Errorf("%s", strings.TrimPrefix(step.Error, "llm_error: "))), nil
	case step.Kind == pullharness.ReturnObservationBudgetExhausted:
		res := s.r.result(s.state, agentruntime.StatusFailed, StopReasonObservationBudgetExhausted, "",
			"观察预算已用尽，运行已止损终止。", "", "", nil)
		res.Error = step.Error
		res.FailureReason = StopReasonObservationBudgetExhausted
		return res, nil
	case step.Disposition == pullharness.DispositionTerminal && step.Kind == pullharness.ReturnFailed:
		res := s.r.result(s.state, agentruntime.StatusFailed, StopReasonFailed, "", "执行失败："+strings.TrimSpace(step.Error), "", "", nil)
		res.Error = step.Error
		res.FailureReason = step.Error
		return res, nil
	default:
		return Result{}, fmt.Errorf("return: step without draft handle and no synthesis rule (kind=%s)", step.Kind)
	}
}

// commitDraft 重放被 draft 策略延后的生命周期提交，恰一次：
// Runtime 状态提交 → EndTurn（同 slice 恰一）→ 终局 retain（T2 面零重复）。
// Suspend 零 retain、continuation 存续（PullContinuation 包装）。
func (s *pullSession) commitDraft(step pullharness.Step, draft Result) Result {
	status := draft.Status
	r := s.r
	if r.Runtime != nil {
		switch status {
		case agentruntime.StatusCompleted:
			s.state.goal = r.Runtime.Complete(s.state.goal.GoalID, nil)
		case agentruntime.StatusFailed:
			err := errorOrNil(draft.Error)
			s.state.goal = r.Runtime.Complete(s.state.goal.GoalID, err)
		case agentruntime.StatusStopped:
			s.state.goal = r.Runtime.MarkStopped(s.state.goal.GoalID, s.state.goal.LastCheckpoint)
		case agentruntime.StatusCancelled:
			s.state.goal = r.Runtime.SetStatus(s.state.goal.GoalID, agentruntime.StatusCancelled, nil)
		default:
			s.state.goal = r.Runtime.SetStatus(s.state.goal.GoalID, status, nil)
		}
	} else {
		s.state.goal.Status = status
	}
	// EndTurn：同 slice 恰一（draft 策略延后至此的唯一提交点）。
	if r.Runtime != nil && s.state.input.TurnID != "" {
		s.state.goal = r.Runtime.EndTurn(s.state.goal.GoalID, s.state.input.TurnID, string(status),
			maxInt(0, s.state.turnsUsed-s.state.sliceTurnsStart), maxInt(0, s.state.toolCallsUsed-s.state.sliceToolCallsStart))
	}
	if isTerminalGoalStatus(status) {
		s.committedTerminal = true
		// T2 面：观察结论 retain（exit_retain 既有语义，RunTurnBoundaryHook
		// TurnID=RunID）；失败留痕不阻断。
		if note := r.retainRunObservationConclusions(s.state); note != "" {
			s.state.trace = append(s.state.trace, planner.TraceEvent{Kind: "exit_retain", Message: note})
		}
	}
	committed := draft
	committed.Goal = s.state.goal
	if !isTerminalGoalStatus(status) {
		// Suspend：continuation 包装为 PullContinuation（同进程恢复面）。
		if committed.Continuation != nil {
			committed.Continuation = nil // 先撤销 legacy 挂载，checkpoint 由 continuationForResume 统一构造
		}
		if pc := s.continuationForResume(draft); pc != nil {
			s.lastCheckpoint = pc
		}
	}
	return committed
}

// ---- 步构造与三轴映射 ----

// stepFromDraft 把 legacy Result（draft）映射为 Step（§11.2 冻结 14 行）。
// Outcome 轴独立：completed 不自动 judgment_ok——结构化证据
// （FreeStateDecision/verification）才产质量分类，否则留空待分层。
func (s *pullSession) stepFromDraft(result Result, source, entry string) (pullharness.Step, error) {
	s.draftSeq++
	draftID := fmt.Sprintf("pull-draft-%d", s.draftSeq)
	step, err := stepFromLegacyResult(result)
	if err != nil {
		// fail-closed：保留 draft 数据与原 StopReason/Error，零新工具。
		return pullharness.Step{}, fmt.Errorf("adapter mapping failed (draft %s preserved): %w", draftID, err)
	}
	step.Source = source
	step.Entry = entry
	step.Outcome = structuredOutcome(result)
	step.DraftID = draftID
	s.drafts[draftID] = pullDraftRecord{result: result, source: source, entry: entry}
	return step, nil
}

// stepFromLegacyResult 是三轴映射的纯函数面（§11.2 表逐行）。
func stepFromLegacyResult(result Result) (pullharness.Step, error) {
	switch result.Status {
	case agentruntime.StatusCompleted:
		// 行 1：completed 只表生命周期结束；route 轴由调用方填。
		return pullharness.Step{Disposition: pullharness.DispositionTerminal, Kind: pullharness.ReturnDone,
			Reply: result.Reply}, nil
	case agentruntime.StatusFailed:
		switch result.StopReason {
		case StopReasonFailed:
			return failedStep(result), nil // 行 2：保留 Error/FailureReason
		case StopReasonModelProtocolFailure:
			return failedStep(result), nil // 行 3：协议失败独立（Error 携带）
		case FreeStateTerminalFallbackStopReason, FreeStateTerminalGateRejectedStopReason:
			return failedStep(result), nil // 行 4：保留覆写后 StopReason 与诊断
		case StopReasonObservationBudgetExhausted:
			return pullharness.Step{Disposition: pullharness.DispositionTerminal, Kind: pullharness.ReturnObservationBudgetExhausted,
				Error: result.Error, Reply: result.Reply}, nil
		default:
			return pullharness.Step{}, fmt.Errorf("unknown failed stop_reason %q (status=failed)", result.StopReason)
		}
	case agentruntime.StatusWaitingConfirmation:
		if result.StopReason != StopReasonNeedsConfirmation {
			return pullharness.Step{}, fmt.Errorf("unknown waiting_confirmation stop_reason %q", result.StopReason)
		}
		return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnConfirmation,
			Reply: result.Reply}, nil // 行 5：pending/cont 由 draft 持有
	case agentruntime.StatusWaitingClarification:
		if result.StopReason != StopReasonNeedsClarification {
			return pullharness.Step{}, fmt.Errorf("unknown waiting_clarification stop_reason %q", result.StopReason)
		}
		return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnClarification,
			Reply: firstNonEmpty(result.ClarificationQuestion, result.Reply)}, nil // 行 6
	case agentruntime.StatusWaitingContinue:
		switch result.StopReason {
		case StopReasonLimitReached:
			return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnSliceLimit,
				Reply: result.Reply}, nil // 行 7：旧切片限额≠budget_exhausted
		case StopReasonInterjection:
			return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnInterjection,
				Reply: result.Reply}, nil // 行 8
		case StopReasonTransientLLMError:
			return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnTransient,
				Error: result.Error, Reply: result.Reply}, nil // 行 9
		case StopReasonInterrupted:
			return pullharness.Step{Disposition: pullharness.DispositionSuspend, Kind: pullharness.ReturnInterrupted,
				Error: result.Error, Reply: result.Reply}, nil // 行 14：ctx 中断（非终局）
		default:
			return pullharness.Step{}, fmt.Errorf("unknown waiting_continue stop_reason %q", result.StopReason)
		}
	case agentruntime.StatusStopped, agentruntime.StatusCancelled:
		// 行 10/11：显式用户 stop/cancel——TerminalCancelled（不当 ctx 暂停）。
		return pullharness.Step{Disposition: pullharness.DispositionTerminal, Kind: pullharness.ReturnUserCancelled,
			Reply: result.Reply}, nil
	case agentruntime.StatusIdle, agentruntime.StatusRunning, agentruntime.StatusProcessing,
		agentruntime.StatusExecuting, agentruntime.StatusCancelling, agentruntime.StatusStable:
		// 行 12：过程快照枚举——不作为终局返回点，映射 Continue（活 state，
		// 不用 cont=nil 猜终局）。
		return pullharness.Step{Disposition: pullharness.DispositionContinue}, nil
	default:
		return pullharness.Step{}, fmt.Errorf("unknown goal status %q", result.Status)
	}
}

func failedStep(result Result) pullharness.Step {
	return pullharness.Step{Disposition: pullharness.DispositionTerminal, Kind: pullharness.ReturnFailed,
		Reply: result.Reply, Error: firstNonEmpty(result.Error, result.FailureReason)}
}

// structuredOutcome 产 Outcome 轴分类：只有结构化证据才分类，completed 不
// 自动 judgment_ok（§11.2）。free-state 终局判定（admitted/terminal）是现役
// 唯一结构化质量面；否则留空（G3 分层时归 pending_classification）。
func structuredOutcome(result Result) string {
	if decision := result.FreeStateDecision; decision != nil {
		if status := strings.TrimSpace(decision.Status); status != "" {
			return "free_state:" + status
		}
	}
	return ""
}

func (s *pullSession) synthesizeInterruptStep(reply string) (pullharness.Step, error) {
	res := s.r.pause(s.state, agentruntime.StatusWaitingContinue, StopReasonInterrupted, "", strings.TrimSpace(reply), "", "", nil)
	step, err := s.stepFromDraft(res, pullharness.StepSourceDriver, "")
	step.Reply = strings.TrimSpace(reply)
	return step, err
}

func (s *pullSession) synthesizeBudgetStep() (pullharness.Step, error) {
	res := s.r.result(s.state, agentruntime.StatusFailed, StopReasonObservationBudgetExhausted, "",
		"观察预算已用尽，运行已止损终止。", "", "", nil)
	res.Error = fmt.Sprintf("budget_exhausted: completed_cycles=%d/%d probe_spent=%s/%s%s",
		s.ledger.completedCycles, s.ledger.maxCycles, fmt.Sprint(s.ledger.probeSpent),
		fmt.Sprint(s.ledger.maxProbeCost), overshootNote(s.ledger.overshoot))
	res.FailureReason = StopReasonObservationBudgetExhausted
	return s.stepFromDraft(res, pullharness.StepSourceFastPath, "")
}

func overshootNote(overshoot bool) string {
	if overshoot {
		return " (overshoot recorded)"
	}
	return ""
}

// projection 把已提交 legacy Result 投影为 pull 侧 Result（从权威 state/ledger
// 投影，不再次累计）。completed≠judgment_ok：outcome=completed（质量分层归
// 判定链/G3 指标层）；fastpath 源短路记 Shortcuts 语义。
func (s *pullSession) projection(committed Result, step pullharness.Step) pullharness.Result {
	return pullharness.Result{
		RunID:   committed.RunID,
		GoalID:  committed.GoalID,
		Outcome: pullOutcome(committed, step),
		Reply:   committed.Reply,
		Error:   committed.Error,
		Cycles:  s.ledger.completedCycles,
		// ModelTurns 从权威账本投影（driver 侧另计装配计量，不累计）。
		ModelTurns:   s.ledger.modelCalls,
		ProbeSpent:   s.ledger.probeSpent,
		Conversation: append([]llm.Message(nil), s.state.input.Conversation...),
		Trace:        []string{"host_return: status=" + string(committed.Status) + " stop_reason=" + committed.StopReason},
	}
}

func pullOutcome(committed Result, step pullharness.Step) string {
	if step.Source == pullharness.StepSourceFastPath && step.Disposition == pullharness.DispositionTerminal &&
		committed.StopReason == StopReasonDone {
		return pullharness.OutcomeFastPath
	}
	switch committed.Status {
	case agentruntime.StatusCompleted:
		return "completed" // ≠judgment_ok（§11.2 三轴分离）
	case agentruntime.StatusFailed:
		switch committed.StopReason {
		case StopReasonObservationBudgetExhausted:
			return pullharness.OutcomeBudgetExhausted
		case StopReasonModelProtocolFailure:
			return "model_protocol_failure"
		default:
			if committed.StopReason != "" && committed.StopReason != StopReasonFailed {
				return committed.StopReason
			}
			return "failed"
		}
	case agentruntime.StatusStopped, agentruntime.StatusCancelled:
		return "user_cancelled"
	default:
		switch committed.StopReason {
		case StopReasonInterrupted:
			return pullharness.OutcomeInterrupted
		case StopReasonTransientLLMError:
			return pullharness.OutcomeLLMError
		case StopReasonLimitReached:
			return "slice_limit"
		}
		return string(committed.Status)
	}
}

// ---- 工具调用载体转换（无损往返：planner.ToolCall ↔ pullharness.ToolCall）----

// toolCallToPull 序列化完整 planner.ToolCall（Args/Command/Reason/PlanItemID
// 全保留）——ArgsJSON 是往返载体，不只 Args。
func toolCallToPull(call planner.ToolCall) (pullharness.ToolCall, error) {
	data, err := json.Marshal(call)
	if err != nil {
		return pullharness.ToolCall{}, fmt.Errorf("marshal tool call: %w", err)
	}
	return pullharness.ToolCall{ID: call.ID, Tool: call.Tool, ArgsJSON: string(data)}, nil
}

func toolCallFromPull(call pullharness.ToolCall) (planner.ToolCall, error) {
	var out planner.ToolCall
	if strings.TrimSpace(call.ArgsJSON) != "" {
		if err := json.Unmarshal([]byte(call.ArgsJSON), &out); err != nil {
			return planner.ToolCall{}, fmt.Errorf("unmarshal tool call %q: %w", call.Tool, err)
		}
	}
	if strings.TrimSpace(call.ID) != "" {
		out.ID = strings.TrimSpace(call.ID)
	}
	if strings.TrimSpace(call.Tool) != "" {
		out.Tool = strings.TrimSpace(call.Tool)
	}
	return out, nil
}

func isTerminalGoalStatus(status agentruntime.GoalStatus) bool {
	switch status {
	case agentruntime.StatusCompleted, agentruntime.StatusCancelled, agentruntime.StatusStopped, agentruntime.StatusFailed:
		return true
	}
	return false
}

func errorOrNil(message string) error {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	return fmt.Errorf("%s", message)
}

// pullPrefixService 返回会话共用的有状态 PrefixService 实例（宿主 l.prefix
// 惰性初始化——同一 run 内驱动装配与宿主伴随索引用同一实例）。
func (l *MessageLoop) pullPrefixService() promptruntime.PrefixService {
	if l.prefix == nil {
		l.prefix = promptruntime.NewPrefixService()
	}
	return l.prefix
}
