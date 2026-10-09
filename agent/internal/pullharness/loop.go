package pullharness

// loop.go — PullLoop 六注入+七步循环（HARNESS_V1_DESIGN §3.1 字面落地）。
//
// 与 push 的语义差异（G3 对比的本质变量）：无逐轮全量快照注入——模型的
// 世界认知=四层前缀（稳定事实，PrefixService）+动态区（会话状态+预算披露）
// +主动查询结果（工具批按需拉取）。观察是模型发起的动作，不是环境推送。
//
// 轮次语义（§2 OQ-1 裁决）：turn=一次模型响应及其工具执行批（工具循环节）。
//   T1：每工具批执行完毕触发 OnTurnBoundary，TurnID=<RunID>:cycle:<n>；
//   T2：run 终态（终局判定/止损/预算尽/溢出 fail-closed/基础设施失败）
//       触发，TurnID=RunID（沿 agentloop/exit_retain.go 先例）；
//   暂停/中断面（ctx 取消）不触发 T2——会话载体随 Result.Conversation
//       存续（continuation 最小面，OQ-H1，决策侧复核点）。
//
// A 阶段空槽（接线归后续卡，面已留）：
//   - 冷启动底座装配（§4.1）：assembleColdStart 返回空——IMPL-B 由装配器
//     从投影只读面渲染一次性机械事实段（GENESIS 三事实组同源），字节恒定；
//   - 快路径路由（§5）：Router nil-safe 跳过——IMPL-C 落注册面，IMPL-D
//     接 pull 侧；命中=确定性执行短路，零模型轮；
//   - 工具面（§3.3）：ToolExecutor 是包内最小接口（执行既有动词面，零动词
//     新增），协议解析与真实接线归 IMPL-D；本卡仅接口+测试 fake。
//
// 溢出预检（步骤③）复用 contextruntime.ModelContextOverflow 既有面
// （不新造）：越线即 fail-closed 终态，请求永不发出（push 先例
// message_loop.go 既有注释语义）。

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/promptruntime"
)

// ToolCall 是模型请求执行的单个工具调用（既有动词面的载体，本包不解释）。
type ToolCall struct {
	ID       string
	Tool     string
	ArgsJSON string
}

// ToolResult 是单个工具执行结果。Bytes 进 T1 动态窗计量；HandleRef /
// EvidenceRefs 是退场三态判据的输入（retain/ref/drop，contextruntime 语义）；
// ProbeCost 是 render/probe 级物理成本（D5 分级：index 级零成本=0）；
// ModelLine 是回喂模型的既定渲染行（协议格式归执行适配层，循环不发明协议）。
type ToolResult struct {
	ID           string
	Tool         string
	Status       string
	ContentJSON  string
	Bytes        int64
	HandleRef    string
	EvidenceRefs []string
	ProbeCost    float64
	ModelLine    string
}

// ToolExecutor 是工具批执行的最小缝（§3.3：适配层不是新工具层）。
// Plan 是协议解析面（模型响应文本→工具批；空批=无工具调用→终态判定），
// Execute 执行批（既有动词面）。真实实现归 IMPL-D；本卡测试用 fake。
type ToolExecutor interface {
	Plan(responseText string) []ToolCall
	Execute(ctx context.Context, calls []ToolCall) []ToolResult
}

// FastPathOutcome 是快路径命中产出的确定性终态（零模型轮）。Outcome 分类
// 口径归 IMPL-C 定（OQ-H2：fastpath 终态独立分类，不混入 judgment 统计）。
type FastPathOutcome struct {
	Reply     string
	Outcome   string
	ProbeCost float64
}

// FastPathRouter 是 §5 快路径路由占位接口（纯模式匹配前置层，非 LLM 调用）。
// nil-safe：PullLoop 允许 Router 为 nil（A 阶段缺省），命中→短路终态。
type FastPathRouter interface {
	Route(ctx context.Context, in GoalInput) (FastPathOutcome, bool)
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

	// ContextSnapshotJSON 是溢出预检消费面（push 的
	// buildModelContextSnapshot 同位物；真实生产面归 IMPL-D 接线）。
	// 复用 contextruntime.ModelContextOverflow：空串/无标记=不越线。
	ContextSnapshotJSON string

	// Engine 是模型调用配置（llm.Client 既有面的入参）。
	Engine config.EngineConfig

	// ClassifyTerminal 是终局判定的分类缝（无工具调用轮调用一次）；
	// nil=缺省 judgment_ok。真实失败分类沿 push 既有口径接线归 IMPL-D
	//（§3.4：诚实分记，A/B 对比按分类分层统计）。
	ClassifyTerminal func(responseText string) string
}

// Result 是 pull run 的终态产出（agentloop.Result 同构最小面）。
type Result struct {
	RunID      string
	GoalID     string
	Outcome    string // 终态分类（本文件头部 Outcome* 常量族）
	Reply      string
	Error      string
	Cycles     int // 完成的工具循节数（有工具批执行的轮）
	ModelTurns int // 实际模型调用次数
	Shortcuts  int // 快路径短路次数（零模型轮）
	ProbeSpent float64

	// Conversation 是终态会话载体（暂停/中断面存续基础，continuation
	// 最小面——T2 不触发的轮次其账本随此存续）。
	Conversation []llm.Message
	// Trace 是确定性留痕行（分类/边界/装配计量；G3 指标采集的原始面）。
	Trace []string
}

// PullLoop 是 §3.1 骨架：六注入。
type PullLoop struct {
	// LLM 是既有客户端面（llm.Client 实现的 llm.Completer 接口；用接口
	// 类型使 fail-closed 等契约可测——llm 包零改动）。
	LLM llm.Completer
	// Prefix 注入装配（不自建装配报告/指纹，§3.2）；nil=缺省实现。
	Prefix promptruntime.PrefixService
	// Tools 工具批执行缝（见 ToolExecutor）；nil=无工具面（模型轮后即终态）。
	Tools ToolExecutor
	// Exit 触发 T1/T2（contextruntime.ExitExecutor 冻结接口，零改动）；
	// nil=边界事件丢弃（advisory 形态，同既有 TurnBoundaryHook 挂点语义）。
	Exit contextruntime.ExitExecutor
	// Router 快路径（§5 占位）；nil-safe 跳过。
	Router FastPathRouter
	// Budget 观察预算（§3.4）。
	Budget ObservationBudget
}

// Run 执行七步循环直至终态。循环与 §3.1 步骤序一一对应：
// 冷启动装配（一次性，A 阶段空槽）→ for { ① 快路径 ② 装配 ③ 溢出预检
// ④ 模型调用/终态判定 ⑤ 工具批 ⑥ [T1] ⑦ 预算检查 } → [T2] 终局 retain。
func (p *PullLoop) Run(ctx context.Context, in GoalInput) Result {
	result := Result{RunID: in.RunID, GoalID: in.GoalID, ProbeSpent: p.Budget.ProbeCost}
	history := append([]llm.Message(nil), in.Conversation...)
	if strings.TrimSpace(in.UserText) != "" {
		history = append(history, llm.Message{Role: "user", Content: in.UserText})
	}
	result.Conversation = history

	// 冷启动底座装配（§4.1，一次性）：A 阶段空槽——IMPL-B 落装配器。
	// 槽位在 assemble 的 SystemSections（session 层 Section，首装挂载后
	// 字节恒定）；run 级无独立动作。

	cycles := 0
	probeSpent := p.Budget.ProbeCost
	var windowUnits []contextruntime.WindowUnit
	var windowBytes int64
	for {
		if ctx.Err() != nil {
			return p.interrupt(&result, "ctx_done_before_route")
		}
		// ① 快路径路由尝试（§5：命中→确定性执行→短路，零模型轮）。
		if p.Router != nil {
			outcome, hit := p.Router.Route(ctx, in)
			if hit {
				result.Shortcuts++
				probeSpent += outcome.ProbeCost
				result.ProbeSpent = probeSpent
				result.Reply = outcome.Reply
				result.Outcome = nonEmpty(outcome.Outcome, OutcomeFastPath)
				result.Trace = append(result.Trace,
					"router_shortcut: outcome="+result.Outcome+" probe_spent="+ftoa(probeSpent))
			}
			if ctx.Err() != nil {
				return p.interrupt(&result, "ctx_done_after_route")
			}
			if hit {
				return p.finish(ctx, in, &result, windowUnits, windowBytes)
			}
		}

		// ② 装配：PrefixService（稳定前缀）+动态区（会话状态+预算披露）。
		assembly, report, err := p.assemble(ctx, in, history, cycles, probeSpent)
		if ctx.Err() != nil {
			return p.interrupt(&result, "ctx_done_after_assembly")
		}
		if err != nil {
			result.Error = err.Error()
			result.Outcome = "assemble_error"
			result.Trace = append(result.Trace, "assemble_error: "+err.Error())
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}
		result.Trace = append(result.Trace, fmt.Sprintf(
			"assembly: cycle=%d prefix_bytes=%d dynamic_bytes=%d breaks=%d",
			cycles+1, report.PrefixBytes, report.DynamicBytes, len(report.Breaks)))

		// ③ 溢出预检：ModelContextOverflow fail-closed（复用既有面）。
		if overflow := contextruntime.ModelContextOverflow(in.ContextSnapshotJSON); overflow != "" {
			result.Outcome = OutcomeContextOverflow
			result.Error = "context_overflow: " + overflow
			result.Trace = append(result.Trace, "context_overflow: "+overflow+" (fail-closed, request not sent)")
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}

		// ④ 模型调用 → 无工具调用 → 终态判定（含失败分类，§3.4）。
		if ctx.Err() != nil {
			return p.interrupt(&result, "ctx_done_before_model")
		}
		if p.LLM == nil {
			result.Outcome = OutcomeLLMError
			result.Error = "pullharness: no LLM client injected"
			result.Trace = append(result.Trace, "llm_error: no client injected")
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}
		result.ModelTurns++
		reply, llmErr := p.LLM.Complete(ctx, in.Engine, assembly.Messages)
		reply = strings.TrimSpace(reply)
		// A cancelled component can still return a reply. Keep received facts
		// before interrupting, including a partial reply returned with an error.
		if ctx.Err() != nil {
			if reply != "" {
				history = append(history, llm.Message{Role: "assistant", Content: reply})
				result.Conversation = history
				result.Reply = reply
			}
			return p.interrupt(&result, "ctx_done_during_model")
		}
		if llmErr != nil {
			result.Outcome = OutcomeLLMError
			result.Error = llmErr.Error()
			result.Trace = append(result.Trace, "llm_error: "+llmErr.Error())
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}
		history = append(history, llm.Message{Role: "assistant", Content: reply})
		result.Conversation = history
		result.Reply = reply

		var calls []ToolCall
		if p.Tools != nil {
			calls = p.Tools.Plan(reply)
		}
		if ctx.Err() != nil {
			return p.interrupt(&result, "ctx_done_after_plan")
		}
		if len(calls) == 0 {
			// 终态判定：无工具调用轮。分类沿缝透传（nil=judgment_ok 缺省）。
			result.Outcome = classifyTerminal(in, reply)
			result.Trace = append(result.Trace, "terminal: no tool calls, outcome="+result.Outcome)
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}

		// ⑤ 工具批执行（读面 ref.query/ref.diff/ccb.observation_*；
		//    写面既有执行动词——零动词新增）。
		batch := p.Tools.Execute(ctx, calls)
		for _, toolResult := range batch {
			probeSpent += toolResult.ProbeCost
		}
		result.ProbeSpent = probeSpent
		if lines := toolModelLines(batch); lines != "" {
			history = append(history, llm.Message{Role: "user", Content: lines})
			result.Conversation = history
		}
		if ctx.Err() != nil {
			// The batch may have performed actions. Preserve returned IDs and
			// costs without claiming completion, triggering T1, or refunding.
			for _, toolResult := range batch {
				result.Trace = append(result.Trace, fmt.Sprintf(
					"interrupted_tool_result: id=%q tool=%q status=%q probe_cost=%s",
					toolResult.ID, toolResult.Tool, toolResult.Status, ftoa(toolResult.ProbeCost)))
			}
			return p.interrupt(&result, "ctx_done_after_tools")
		}
		cycles++
		result.Cycles = cycles

		// ⑥ [T1] OnTurnBoundary(cycle:<n>)——每工具批恰一次。
		turnID := cycleTurnID(in.RunID, cycles)
		batchUnits, batchBytes := p.fireTurnBoundary(ctx, in, turnID, batch)
		windowUnits = append(windowUnits, batchUnits...)
		windowBytes += batchBytes
		if ctx.Err() != nil {
			return p.interrupt(&result, "ctx_done_after_T1")
		}

		// ⑦ 预算检查：exhausted → 止损终态 budget_exhausted（独立分类）。
		if p.Budget.CycleExhausted(cycles) {
			result.Outcome = OutcomeBudgetExhausted
			result.Error = fmt.Sprintf("budget_exhausted: cycles=%d max=%d", cycles, p.Budget.MaxCycles)
			result.Trace = append(result.Trace, result.Error)
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}
		if p.Budget.ProbeExhausted(probeSpent) {
			result.Outcome = OutcomeBudgetExhausted
			result.Error = fmt.Sprintf("budget_exhausted: probe_spent=%s max=%s", ftoa(probeSpent), ftoa(p.Budget.MaxProbeCost))
			result.Trace = append(result.Trace, result.Error)
			return p.finish(ctx, in, &result, windowUnits, windowBytes)
		}
	}
}

// assembleColdStart 是 §4.1 冷启动底座挂点。A 阶段返回空（空操作）：
// 底座=会话首装时由装配器从投影只读面渲染的一次性机械事实段（GENESIS
// 三事实组同源），字节恒定——装配归 IMPL-B。落点=Stable session Section
// 进 SystemSections（首装挂载后不再变）。
func (p *PullLoop) assembleColdStart(in GoalInput) []promptruntime.Section {
	return nil // IMPL-B 填槽
}

// assemble 执行步骤②：四层稳定前缀（A 阶段=冷启动槽，当前空）+动态区
// （会话状态+预算披露，§3.2）。SessionKey 按 RunID 隔离（P1 append-only
// 判据跨 turn 可测）。
func (p *PullLoop) assemble(ctx context.Context, in GoalInput, history []llm.Message, cycles int, probeSpent float64) (promptruntime.Assembly, promptruntime.AssemblyReport, error) {
	prefix := p.Prefix
	if prefix == nil {
		prefix = promptruntime.NewPrefixService()
	}
	return prefix.Assemble(ctx, promptruntime.PrefixRequest{
		AssemblyInput: promptruntime.AssemblyInput{
			SystemSections: p.assembleColdStart(in),
			History:        history,
			UserSections: []promptruntime.Section{
				promptruntime.TextSection(promptruntime.SectionRuntime, "pullharness.dynamic",
					"run state", p.dynamicZone(in, cycles, probeSpent), false),
			},
		},
		SessionKey: "pullharness:" + in.RunID,
	})
}

// dynamicZone 是逐轮重算的动态区（§3.2：pull loop 每轮只重算动态区）。
func (p *PullLoop) dynamicZone(in GoalInput, cycles int, probeSpent float64) string {
	lines := []string{}
	if text := strings.TrimSpace(in.UserText); text != "" {
		lines = append(lines, "goal: "+text)
	}
	lines = append(lines, p.Budget.Disclosure(cycles, probeSpent))
	return strings.Join(lines, "\n")
}

func classifyTerminal(in GoalInput, reply string) string {
	if in.ClassifyTerminal != nil {
		if outcome := strings.TrimSpace(in.ClassifyTerminal(reply)); outcome != "" {
			return outcome
		}
	}
	return OutcomeJudgmentOK
}

func cycleTurnID(runID string, cycle int) string {
	return runID + ":cycle:" + itoa(cycle)
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

// toolModelLines 把工具批的既定渲染行并成一条回喂消息（协议格式归执行
// 适配层的 ToolResult.ModelLine，循环不做二次解释）。
func toolModelLines(batch []ToolResult) string {
	lines := make([]string, 0, len(batch))
	for _, toolResult := range batch {
		if line := strings.TrimSpace(toolResult.ModelLine); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// fireTurnBoundary 触发单批 T1：单元=工具结果行（TurnID 标本批轮次 →
// turn_end 判据按既有机械规则触发）；热字节=批字节合计；HotLimitBytes=0
// （A 阶段无动态窗字节线——退出判据面维持 §4.1 既有机械规则）。
// 返回本批单元与字节（终局 T2 窗面的累计输入）。
func (p *PullLoop) fireTurnBoundary(ctx context.Context, in GoalInput, turnID string, batch []ToolResult) ([]contextruntime.WindowUnit, int64) {
	if p.Exit == nil {
		return nil, 0
	}
	units := make([]contextruntime.WindowUnit, 0, len(batch))
	var hotBytes int64
	for _, toolResult := range batch {
		units = append(units, contextruntime.WindowUnit{
			Unit: contextruntime.ExitUnit{
				Kind: contextruntime.ExitUnitToolResult,
				ID:   toolResult.ID,
			},
			Bytes:        toolResult.Bytes,
			TurnID:       turnID,
			HandleRef:    toolResult.HandleRef,
			EvidenceRefs: toolResult.EvidenceRefs,
		})
		hotBytes += toolResult.Bytes
	}
	p.Exit.OnTurnBoundary(ctx, contextruntime.TurnBoundaryEvent{
		TurnID: turnID,
		Window: contextruntime.WindowState{Units: units},
		Budget: contextruntime.BudgetState{HotBytes: hotBytes},
		Now:    time.Now(),
	})
	return units, hotBytes
}

// finish 是终局出口（终局判定/止损/预算尽/溢出/装配失败/基础设施失败/
// 快路径短路）：[T2] OnTurnBoundary(RunID)——exit_retain 语义同型
// （TurnID=RunID 沿既有先例）；窗面=run 累计工具单元（observation 结论级
// retain 供给归 IMPL-B）。中断面（interrupt）不经此处——暂停面不触发 T2。
func (p *PullLoop) finish(ctx context.Context, in GoalInput, result *Result, windowUnits []contextruntime.WindowUnit, windowBytes int64) Result {
	// Also cover cancellation inside terminal classification. This check only
	// prevents a subsequent T2; it cannot undo already completed stages.
	if ctx.Err() != nil {
		return p.interrupt(result, "ctx_done_before_T2")
	}
	result.Trace = append(result.Trace, "terminal: outcome="+result.Outcome+" cycles="+itoa(result.Cycles)+" model_turns="+itoa(result.ModelTurns))
	if p.Exit != nil && strings.TrimSpace(in.RunID) != "" {
		p.Exit.OnTurnBoundary(ctx, contextruntime.TurnBoundaryEvent{
			TurnID: in.RunID,
			Window: contextruntime.WindowState{Units: windowUnits},
			Budget: contextruntime.BudgetState{HotBytes: windowBytes},
			Now:    time.Now(),
		})
	}
	return *result
}

// interrupt 是中断/暂停出口（ctx 取消）：不触发 T2（暂停面语义），会话
// 载体随 Result.Conversation 存续（continuation 最小面）。
func (p *PullLoop) interrupt(result *Result, where string) Result {
	result.Outcome = OutcomeInterrupted
	result.Error = "interrupted: " + where
	result.Trace = append(result.Trace, "interrupted: "+where+" (pause surface, no T2)")
	return *result
}
