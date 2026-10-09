package pullharness

// loop.go — Session 驱动的 pull 循环（HARNESS_V1_DESIGN §3.1/§11.1，L1-5-IMPL-D 腿1）。
//
// S1 形态（§11.1 定版）：宿主（agentloop pullSession，腿2）独家持有活状态；
// 本驱动只做四件事——装配（Prefix 四层+冷启动底座+宿主供给协议段+动态区）、
// 溢出预检（实际装配字节计量，fail-closed）、模型调用、循环编排。快路径
// （Attempt）/协议解析（Interpret）/工具执行（Execute）/T1（CloseCycle）/
// 生命周期提交（Return）全部回宿主经 Session 六方法。
//
// A 占位接口退役（§11.1 授权）：ToolExecutor/FastPathRouter 占位面移除——
// 工具批经 Session.Execute（宿主同一执行 gateway），快路径经 Session.Attempt
// （宿主 Router 注册面，注册序/短路/diagnostic 旁路语义保持）。CANCEL-FIX
// 冻结表的取消语义在 Session 面重新锚定（loop_cancel_test.go 六路径随迁）：
// 中断=Step{Suspend, Interrupted} 交宿主 Return（保存恢复身份，不造终局），
// 已收事实（模型回复/回执/成本）不丢，未完批不 CloseCycle，无 T2。
//
// 轮次语义（§2 OQ-1）：T1=完整模型工具批（CloseCycle 唯一入口，宿主用注入
// Exit 构造事件）；T2=run 终局 retain（宿主 Return 提交，沿 exit_retain 先例
// ——驱动不再自行发 T2）。Router-only 批不伪装模型工具循环（不 CloseCycle）。
//
// 预算契约（§11.3.5）：宿主 ledger=累计实际消费唯一 authority（Attempt/Execute
// 入场执法）；Budget 字段是声明上限面（动态区披露+测试），驱动不重复执法。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
)

// ToolCall 是模型请求执行的单个工具调用（既有动词面的载体，本包不解释；
// Interpret 产的未执行批与 Execute 的入参共用此形态）。
type ToolCall struct {
	ID       string
	Tool     string
	ArgsJSON string
}

// ToolResult 是单个已执行工具的回执投影（T1 批轮单元构造面，exit_wiring.go
// BatchTurnEvent 消费；宿主 CloseCycle 从执行 gateway 的真实回执适配）。
// Bytes 进动态窗计量；RetainedStatement 是结论级陈述显式供给通道；
// ProbeCost 是 render/probe 级物理成本（D5 分级：index 级零成本=0；计量
// 缺失时宿主按 unknown 处理，不填 0 冒充免费）。
type ToolResult struct {
	ID                string
	Tool              string
	Status            string
	ContentJSON       string
	Bytes             int64
	HandleRef         string
	EvidenceRefs      []string
	RetainedStatement string
	ProbeCost         float64
	ModelLine         string
}

// GoalInput 是 pull run 的最小输入面（OQ-H1：按 agentloop 同构形态定最小
// 面，决策侧复核点；不拖 planner/agentruntime 依赖）。
type GoalInput struct {
	GoalID   string
	RunID    string
	UserText string
	Context  map[string]any

	// Conversation 是入场会话（续跑承接面）；UserText 非空时作为首条
	// user 消息接在其后。
	Conversation []llm.Message

	// ContextSnapshotJSON 是入场溢出基线（一次性预检；循环内溢出预检按
	// 实际装配字节计量——§3.2-4 不反复用最初的静态快照）。复用
	// contextruntime.ModelContextOverflow：空串/无标记=不越线。
	ContextSnapshotJSON string

	// Engine 是模型调用配置（llm.Client 既有面的入参）。
	Engine config.EngineConfig
}

// Result 是 pull run 的终态产出（宿主 Return 的投影面；驱动追加自身 zone
// 的 trace 行，不重复累计计数）。Outcome 分类常量见 budget.go。
type Result struct {
	RunID      string
	GoalID     string
	Outcome    string
	Reply      string
	Error      string
	Cycles     int // 完成的工具循节数（完整模型工具批）
	ModelTurns int // 实际模型调用次数
	Shortcuts  int // 快路径短路次数（零模型轮）
	ProbeSpent float64

	// Conversation 是终态会话载体（暂停/中断面存续基础，continuation
	// 最小面——T2 不触发的轮次其账本随此存续）。
	Conversation []llm.Message
	// Trace 是确定性留痕行（分类/边界/装配计量；G3 指标采集的原始面）。
	Trace []string
}

// PullLoop 是 Session 驱动的循环骨架（§11.1：LLM/PrefixService/ExitExecutor
// 接口冻结不改；工具/快路径/生命周期面归 Session）。
type PullLoop struct {
	// LLM 是既有客户端面（llm.Completer 接口；fail-closed 等契约可测）。
	LLM llm.Completer
	// Prefix 注入装配（不自建装配报告/指纹，§3.2）；nil=缺省实现。生产
	// 构造要求非 nil（Run 入口 fail-closed 拒绝——A-REVIEW 挂账③；测试
	// fake 经 AllowNilPrefix 豁免）。
	Prefix promptruntime.PrefixService
	// Exit 是 T1/T2 事件执行器（经 CloseCycle 传宿主；驱动不自行触发）；
	// nil=宿主按 advisory 形态处理（事件丢弃，同既有 TurnBoundaryHook
	// 挂点语义）。
	Exit contextruntime.ExitExecutor
	// Session 是宿主会话端口（§11.1 六方法；nil=协议违规，fail-closed）。
	Session Session
	// Budget 是声明上限面（动态区披露；执法 authority=宿主 ledger）。
	Budget ObservationBudget
	// ColdStart 是冷启动底座只读输入缝（§4.1）；nil=无底座（缺省形态）。
	ColdStart ColdStartSource
	// ExitWiring 是边界事件构造线位（宿主 CloseCycle 消费）。
	ExitWiring ExitWiring
	// AllowNilPrefix 豁免生产非 nil Prefix 要求（仅测试 fake 用；生产
	// 构造不得设置）。
	AllowNilPrefix bool
}

// Run 执行 Session 驱动循环直至宿主 Return 终态。循环步序（§3.1 对应）：
// ① Attempt（宿主快路径）→ miss 回流 Snapshot → ② 装配 → ③ 溢出预检
// → ④ 模型调用 → ⑤ Interpret（宿主协议面）→ ⑥ Execute（宿主 gateway）
// → ⑦ CloseCycle（T1）→ 下一轮。任何非 Continue 步交宿主 Return。
func (p *PullLoop) Run(ctx context.Context, in GoalInput) Result {
	if p.Session == nil {
		return protocolFailResult(in, "no session injected")
	}
	if p.Prefix == nil && !p.AllowNilPrefix {
		return protocolFailResult(in, "no prefix service injected (fail-closed)")
	}
	coldStart := p.renderColdStart()
	d := &pullDriver{loop: p, in: in, coldStart: coldStart, trace: []string{}}
	return d.run(ctx)
}

// renderColdStart 是 §4.1 冷启动底座首装点：ColdStart 只读缝一次渲染；
// nil 源=空底座。产物在 run 内复用（字节恒定）。
func (p *PullLoop) renderColdStart() ColdStartBase {
	if p.ColdStart == nil {
		return ColdStartBase{}
	}
	return RenderColdStart(p.ColdStart.ColdStartEngineSnapshot())
}

type pullDriver struct {
	loop      *PullLoop
	in        GoalInput
	coldStart ColdStartBase
	trace     []string
}

func (d *pullDriver) run(ctx context.Context) Result {
	// 入场溢出基线（一次性）：入场已越线的请求 fail-closed，不进循环。
	if overflow := contextruntime.ModelContextOverflow(d.in.ContextSnapshotJSON); overflow != "" {
		return d.returnStep(ctx, d.driverStep(DispositionTerminal, ReturnFailed, OutcomeContextOverflow,
			"context_overflow: "+overflow+" (fail-closed at entry, request not sent)"))
	}
	for {
		if ctx.Err() != nil {
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_before_attempt"))
		}

		// ① Attempt：宿主快路径/pending 队列（miss=Continue 无 Calls）。
		step, err := d.loop.Session.Attempt(ctx)
		if err != nil {
			return d.sessionFail("attempt", err)
		}
		if invalid := validateStep(step, "attempt"); invalid != "" {
			return d.sessionFail("attempt", fmt.Errorf("%s", invalid))
		}
		if step.Disposition != DispositionContinue {
			return d.returnStep(ctx, step)
		}
		if len(step.Calls) > 0 {
			// Attempt 的 Continue 不得携带未执行批（handler 的工具请求已
			// 在宿主执行）——协议违规 fail-closed。
			return d.sessionFail("attempt", fmt.Errorf("continue step from Attempt carries %d calls (already-executed work must surface as ReceiptIDs)", len(step.Calls)))
		}

		// miss 回流：新鲜 Frame（成本先结算、状态先回流，然后才装配）。
		frame, err := d.loop.Session.Snapshot(ctx)
		if err != nil {
			return d.sessionFail("snapshot", err)
		}
		if ctx.Err() != nil {
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_after_attempt"))
		}

		// ② 装配：Prefix（四层稳定前缀+冷启动底座）+宿主协议段+动态区。
		assembly, report, err := d.assemble(frame)
		if err != nil {
			return d.sessionFail("assemble", err)
		}
		d.trace = append(d.trace, fmt.Sprintf(
			"assembly: cycle=%d prefix_bytes=%d dynamic_bytes=%d breaks=%d",
			frame.Budget.CompletedCycles+1, report.PrefixBytes, report.DynamicBytes, len(report.Breaks)))
		if ctx.Err() != nil {
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_after_assembly"))
		}

		// ③ 溢出预检：实际装配字节计量（fail-closed，请求永不发出）。
		if overflow := d.assemblyOverflow(assembly, frame); overflow != "" {
			return d.returnStep(ctx, d.driverStep(DispositionTerminal, ReturnFailed, OutcomeContextOverflow,
				"context_overflow: "+overflow+" (fail-closed, request not sent)"))
		}

		// ④ 模型调用。
		if d.loop.LLM == nil {
			return d.returnStep(ctx, d.driverStep(DispositionTerminal, ReturnFailed, OutcomeLLMError,
				"llm_error: no LLM client injected"))
		}
		raw, llmErr := d.loop.LLM.Complete(ctx, d.in.Engine, assembly.Messages)
		raw = strings.TrimSpace(raw)
		if ctx.Err() != nil {
			// A cancelled component can still return a reply. Keep received
			// facts before interrupting, including a partial reply returned
			// with an error.
			return d.returnStep(ctx, d.driverInterruptWithReply("ctx_done_during_model", raw))
		}
		if llmErr != nil {
			// 基础设施失败交宿主裁定暂停语义（free-state transient 判定在
			// 宿主）：Suspend/Transient，零新工具。
			return d.returnStep(ctx, d.driverStep(DispositionSuspend, ReturnTransient, OutcomeLLMError,
				"llm_error: "+llmErr.Error()))
		}

		// ⑤ Interpret：宿主协议面（解析/修复/守卫/final gate/澄清）。
		step, err = d.loop.Session.Interpret(ctx, raw)
		if err != nil {
			return d.sessionFail("interpret", err)
		}
		if invalid := validateStep(step, "interpret"); invalid != "" {
			return d.sessionFail("interpret", fmt.Errorf("%s", invalid))
		}
		if step.Disposition != DispositionContinue {
			return d.returnStep(ctx, step)
		}
		if len(step.Calls) == 0 {
			// 宿主判继续（final gate 反馈等），无工具批——不 CloseCycle，
			// 直接装配下一模型轮。
			continue
		}
		if ctx.Err() != nil {
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_after_interpret"))
		}

		// ⑥ Execute：宿主同一执行 gateway（守卫/权限/verifier 面不变）。
		step, err = d.loop.Session.Execute(ctx, step.Calls)
		if err != nil {
			return d.sessionFail("execute", err)
		}
		if invalid := validateStep(step, "execute"); invalid != "" {
			return d.sessionFail("execute", fmt.Errorf("%s", invalid))
		}
		if step.Disposition != DispositionContinue {
			// partial 批（批中确认/暂停/取消）不 CloseCycle。
			return d.returnStep(ctx, step)
		}
		if ctx.Err() != nil {
			// 批已执行（回执在宿主账本），但批不完整结束——不 CloseCycle、
			// 不退款，中断面交宿主 Return（CANCEL-FIX：ctx_done_after_tools）。
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_after_tools"))
		}

		// ⑦ CloseCycle：完整模型工具批恰一 T1（宿主用注入 Exit 构造事件
		// 并应用真实 ExitReport）。批身份=<RunID>:cycle:<NextCycle>（账本
		// 分配身份，§5.2）。
		batchID := allocatedBatchID(d.in.RunID, frame)
		if err := d.loop.Session.CloseCycle(ctx, batchID, d.loop.Exit); err != nil {
			return d.sessionFail("close_cycle", err)
		}
		d.trace = append(d.trace, "cycle_closed: batch="+batchID+
			" receipts="+itoa(len(step.ReceiptIDs)))
		if ctx.Err() != nil {
			return d.returnStep(ctx, d.driverInterrupt("ctx_done_after_T1"))
		}
		// 预算执法 authority=宿主 ledger（下一轮 Attempt 入场拒绝）；驱动
		// 侧无重复执法。
	}
}

// assemble 执行步骤②：稳定前缀（冷启动底座 Section 族+宿主协议段）+
// 动态区（会话状态+预算披露，§3.2）。SessionKey 用宿主供给的会话键
// （P1 append-only 判据跨 turn 可测）。
func (d *pullDriver) assemble(frame Frame) (promptruntime.Assembly, promptruntime.AssemblyReport, error) {
	systemSections := append([]promptruntime.Section(nil), d.coldStart.Sections...)
	if protocol, ok := frameString(frame.Context, FrameContextProtocolPrompt); ok && strings.TrimSpace(protocol) != "" {
		systemSections = append(systemSections, promptruntime.TextSection(
			promptruntime.SectionStatic, "pullharness.protocol", "model protocol", protocol, true))
	}
	sessionKey := strings.TrimSpace(frame.PrefixSessionKey)
	if sessionKey == "" {
		sessionKey = "pullharness:" + d.in.RunID
	}
	prefix := d.loop.Prefix
	if prefix == nil {
		// AllowNilPrefix=测试豁免面：回退缺省 PrefixService（A 阶段形态）；
		// 生产构造禁止（Run 入口已 fail-closed 拒绝——A-REVIEW 挂账③）。
		prefix = promptruntime.NewPrefixService()
	}
	return prefix.Assemble(ctxBackground(), promptruntime.PrefixRequest{
		AssemblyInput: promptruntime.AssemblyInput{
			SystemSections: systemSections,
			History:        append([]llm.Message(nil), frame.Conversation...),
			UserSections: []promptruntime.Section{
				promptruntime.TextSection(promptruntime.SectionRuntime, "pullharness.dynamic",
					"run state", d.dynamicZone(frame), false),
			},
		},
		SessionKey:  sessionKey,
		LayerStates: d.coldStart.LayerStates(),
	})
}

// dynamicZone 是逐轮重算的动态区（§3.2：每轮只重算动态区；§4.2 表行 3/4
// 落点见 disclosure.go）。预算行从宿主账本只读视图披露（authority 在宿主）。
func (d *pullDriver) dynamicZone(frame Frame) string {
	lines := []string{}
	if text := strings.TrimSpace(d.in.UserText); text != "" {
		lines = append(lines, "goal: "+text)
	}
	lines = append(lines, ledgerDisclosureRows(frame.Budget, d.loop.Budget)...)
	return strings.Join(lines, "\n")
}

// assemblyOverflow 按实际装配字节计量溢出（§3.2-4）：合计消息内容字节 vs
// 宿主披露的线位（FrameContextContextBudgetBytes；<=0=不设线）。越线经
// contextruntime.ModelContextOverflow 渲染（复用既有面，不新造格式）。
func (d *pullDriver) assemblyOverflow(assembly promptruntime.Assembly, frame Frame) string {
	limit := frameInt(frame.Context, FrameContextContextBudgetBytes)
	if limit <= 0 {
		return ""
	}
	var total int64
	for _, message := range assembly.Messages {
		total += int64(len(message.Content))
	}
	if total <= int64(limit) {
		return ""
	}
	marker, _ := json.Marshal(map[string]any{
		"context_overflow": map[string]any{
			"status": "overflow", "hot_bytes": total, "hot_budget_bytes": limit,
			"warm_bytes": 0, "warm_budget_bytes": 0, "cold_ref": "",
		},
	})
	return contextruntime.ModelContextOverflow(string(marker))
}

// driverStep 构造驱动自建步（中断/溢出/LLM 基础设施面），交宿主 Return
// 提交生命周期——驱动不直接造终局。outcome 非空时随步透传（宿主投影消费）。
func (d *pullDriver) driverStep(disposition Disposition, kind ReturnKind, outcome, message string) Step {
	return Step{Disposition: disposition, Kind: kind, Source: StepSourceDriver, Outcome: outcome, Error: message}
}

func (d *pullDriver) driverInterrupt(where string) Step {
	return d.driverStep(DispositionSuspend, ReturnInterrupted, OutcomeInterrupted, "interrupted: "+where)
}

// driverInterruptWithReply 保留已收模型事实后中断（CANCEL-FIX：partial reply
// 不丢，进 Step.Reply 由宿主存档）。
func (d *pullDriver) driverInterruptWithReply(where, reply string) Step {
	step := d.driverInterrupt(where)
	step.Reply = reply
	return step
}

// returnStep 把非 Continue 步交宿主 Return（终局唯一 owner），并合并驱动
// 自身 zone 的 trace 行。Return 失败=协议违规，fail-closed 直返（宿主状态
// 未提交，由入口层兜底清理）。
func (d *pullDriver) returnStep(ctx context.Context, step Step) Result {
	result, err := d.loop.Session.Return(ctx, step)
	if err != nil {
		return protocolFailResult(d.in, "return failed: "+err.Error())
	}
	result.Trace = append(result.Trace, d.trace...)
	if step.Source == StepSourceFastPath {
		result.Shortcuts++
	}
	return result
}

// sessionFail 是 Session 协议违规的 fail-closed 出口：不调 Return（宿主
// 状态未动，零新工具）， Outcome=session_error 显式可见，由入口层兜底。
func (d *pullDriver) sessionFail(where string, err error) Result {
	result := protocolFailResult(d.in, where+": "+err.Error())
	result.Trace = append(result.Trace, d.trace...)
	return result
}

func protocolFailResult(in GoalInput, message string) Result {
	return Result{
		RunID:   in.RunID,
		GoalID:  in.GoalID,
		Outcome: OutcomeSessionError,
		Error:   "session_error: " + message,
		Trace:   []string{"session_error: " + message},
	}
}

// validateStep 检查步的协议有效性（Disposition/Kind 封闭枚举——未知拒绝，
// §11.1）。返回空串=有效。
func validateStep(step Step, where string) string {
	if !step.Disposition.Valid() {
		return where + ": invalid disposition " + step.Disposition.String()
	}
	if step.Disposition != DispositionContinue && !step.Kind.Valid() {
		// Continue 步无细分 Kind（零值合法）；Suspend/Terminal 必须带合法
		// 返回原因。
		return where + ": suspend/terminal step missing valid return kind"
	}
	return ""
}

// allocatedBatchID 从账本分配身份构造批 ID（<RunID>:cycle:<NextCycle>；
// §5.2：partial 也占唯一 ID，新批分配下一 ID）。NextCycle=0 时回退本地
// 计数（宿主未实现账本面的测试形态）。
func allocatedBatchID(runID string, frame Frame) string {
	if frame.Budget.NextCycle > 0 {
		return runID + ":cycle:" + itoa(int(frame.Budget.NextCycle))
	}
	return runID + ":cycle:" + itoa(frame.Budget.CompletedCycles+1)
}

func ctxBackground() context.Context { return context.Background() }

func frameString(ctx map[string]any, key string) (string, bool) {
	value, ok := ctx[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	return text, ok
}

func frameInt(ctx map[string]any, key string) int {
	switch value := ctx[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		number, err := value.Int64()
		if err != nil {
			return 0
		}
		return int(number)
	}
	return 0
}
