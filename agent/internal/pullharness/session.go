package pullharness

// session.go — Session 中性会话协议（HARNESS_V1_DESIGN §11.1 冻结签名；
// 细节规格=L1-5-ADAPTER-DESIGN-1 提案 §3，主管 2026-10-09 晚窗冻结采纳）。
//
// S1 形态：宿主（agentloop pullSession）独家持有活状态（*runState/Runner/
// MessageLoop 与十快路径 handler）；本包只经有版本只读 Frame 驱动循环。
// 三轴分离（§11.2）：Lifecycle（Disposition）/Route（Source）/Outcome
// （Kind+Outcome）——completed≠judgment_ok，EndTurn 宿主唯一 owner，
// ctx 中断（Interrupted）与显式用户取消（UserCancelled）严格分离。
//
// 本包不 import agentloop/runtime/planner（方向约束：agentloop→pullharness
// 单向）。A 阶段占位接口 ToolExecutor/FastPathRouter 随本协议退役（§11.1
// 授权；CANCEL-FIX 冻结表取消语义在 Session 面重新锚定，六路径回归随迁）。

import (
	"context"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/llm"
)

// Disposition 是生命周期轴：继续/暂停/真实终局。零值无效（未知拒绝，
// fail-closed——驱动侧不对其行动）。
type Disposition uint8

const (
	DispositionContinue Disposition = iota + 1
	DispositionSuspend
	DispositionTerminal
)

// String 渲染稳定名（遥测/断言面；未知值渲染 unknown）。
func (d Disposition) String() string {
	switch d {
	case DispositionContinue:
		return "continue"
	case DispositionSuspend:
		return "suspend"
	case DispositionTerminal:
		return "terminal"
	}
	return "unknown"
}

// Valid 报告生命周期值是否在封闭枚举内（未知拒绝的机械面）。
func (d Disposition) Valid() bool {
	return d >= DispositionContinue && d <= DispositionTerminal
}

// ReturnKind 是返回原因轴（Suspend/Terminal 的细分）。零值无效。
type ReturnKind uint8

const (
	ReturnDone ReturnKind = iota + 1
	ReturnFailed
	ReturnConfirmation
	ReturnClarification
	ReturnSliceLimit
	ReturnInterjection
	ReturnTransient
	ReturnInterrupted
	ReturnUserCancelled
	ReturnObservationBudgetExhausted
)

// String 渲染稳定名（G3 判定链 outcome 分层口径的输入之一）。
func (k ReturnKind) String() string {
	switch k {
	case ReturnDone:
		return "done"
	case ReturnFailed:
		return "failed"
	case ReturnConfirmation:
		return "confirmation"
	case ReturnClarification:
		return "clarification"
	case ReturnSliceLimit:
		return "slice_limit"
	case ReturnInterjection:
		return "interjection"
	case ReturnTransient:
		return "transient"
	case ReturnInterrupted:
		return "interrupted"
	case ReturnUserCancelled:
		return "user_cancelled"
	case ReturnObservationBudgetExhausted:
		return "observation_budget_exhausted"
	}
	return "unknown"
}

// Valid 报告返回原因值是否在封闭枚举内。
func (k ReturnKind) Valid() bool {
	return k >= ReturnDone && k <= ReturnObservationBudgetExhausted
}

// Step 来源轴（Route）：快路径/模型/驱动自身。宿主映射时填写；驱动自建
// 步（中断/溢出/LLM 基础设施面）用 StepSourceDriver。
const (
	StepSourceFastPath = "fastpath"
	StepSourceModel    = "model"
	StepSourceDriver   = "pullharness"
)

// LedgerView 是宿主账本的只读披露视图（§11.3.5：宿主 ledger=累计实际消费
// 唯一 authority，pull 只读披露）。ProbeCostKnown=false 表示成本计量缺失
// （unknown 不填 0——不冒充免费）。
type LedgerView struct {
	NextCycle       uint64
	CompletedCycles int
	ModelCalls      int
	ToolAttempts    int
	ProbeSpent      float64
	ProbeCostKnown  bool
}

// Frame 是有版本只读会话视图：map/slice 返回克隆（宿主实现义务）；Revision
// 单调递增，驱动用它拒绝用旧 Frame 装配下一模型轮（每次 Attempt/Interpret/
// Execute 后须重新 Snapshot）。
type Frame struct {
	Revision         uint64
	GoalID           string
	RunID            string
	PrefixSessionKey string
	Context          map[string]any
	Conversation     []llm.Message
	Budget           LedgerView
}

// Frame.Context 的稳定供给键（宿主→驱动只读面；缺失=能力缺席，不臆造）。
const (
	// FrameContextProtocolPrompt 是宿主供给的协议指令段（单段形态；自
	// G3-ATTRIB-2 起被双段键取代——见 loop.go 的
	// FrameContextProtocolSkeleton/FrameContextProtocolDirectives；驱动保留
	// 单段兼容挂载，生产宿主已改双段供给）。缺失=无协议段，模型自由文本
	// 将由宿主 Interpret 的修复链处理。字节稳定性由宿主侧前缀治理约束
	//（变化会在装配报告的断裂归因中显式可见）。
	FrameContextProtocolPrompt = "pull_protocol_prompt"
	// FrameContextContextBudgetBytes 是溢出线（>0 生效；缺省=不设线）。
	// 驱动按实际装配字节计量（§3.2-4：不反复用最初的静态快照）。
	FrameContextContextBudgetBytes = "pull_context_budget_bytes"
)

// Step 是一次会话动作的产出。Calls 仅 Interpret 可产（未执行模型批）；
// ReceiptIDs 是已执行工具的回执（不可回放成 Calls——已执行不重放）；
// DraftID 是宿主一次性内存句柄（不可持久化；跨进程恢复只认版本化
// PullContinuation，宿主侧实现）。
type Step struct {
	Disposition Disposition
	Kind        ReturnKind
	Source      string
	Entry       string
	Outcome     string
	Reply       string
	Error       string
	Calls       []ToolCall
	ReceiptIDs  []string
	DraftID     string
}

// Session 是宿主会话端口（六方法冻结）。宿主同时实现快路径尝试（Attempt）、
// 模型结果解释（Interpret）、执行（Execute）与返回（Return）——避免只修
// Router、却在 Execute 与终局重新出现双 owner。
type Session interface {
	// Snapshot 返回当前有版本只读视图（克隆；驱动不修改后写回）。
	Snapshot(context.Context) (Frame, error)
	// Attempt 尝试确定性快路径与既有 pending 队列（宿主面：diagnostic 刷新
	// +十 handler 注册序）。miss=Continue（无 Calls）；命中/暂停按三轴映射
	// 返回 Step。Continue 时 Step 不得携带 Calls（handler 的工具请求已在
	// 宿主执行，ReceiptIDs 表达）。
	Attempt(context.Context) (Step, error)
	// Interpret 解释一次模型原始回复（宿主复用协议解析/修复/守卫/final
	// gate/澄清语义）：终局/暂停/待执行批（Calls）或继续（无 Calls，
	// 如 final gate 反馈后重试）。
	Interpret(context.Context, string) (Step, error)
	// Execute 经宿主同一执行 gateway 执行批（守卫/权限/verifier 面不变）。
	// 完整批=Continue+ReceiptIDs；批中暂停/终局=对应 Step（partial 不
	// CloseCycle）。
	Execute(context.Context, []ToolCall) (Step, error)
	// CloseCycle 是 T1 唯一调用入口：完整模型工具批执行完毕后恰一次，
	// 用注入的 ExitExecutor 构造边界事件并应用真实 ExitReport（宿主窗口/
	// history refs 更新）。TurnID=<RunID>:cycle:<NextCycle>（分配身份）。
	CloseCycle(context.Context, string, contextruntime.ExitExecutor) error
	// Return 提交一次生命周期（终局唯一 owner：terminal commit=1+retain
	// hook=1+同 slice EndTurn=1；Suspend=保存 checkpoint+EndTurn slice 一次、
	// 零 retain）。同 DraftID 重复 Return 幂等返回原提交结果。返回 pull
	// 侧 Result 投影（从权威 state/ledger 投影，不再次累计）。
	Return(context.Context, Step) (Result, error)
}
