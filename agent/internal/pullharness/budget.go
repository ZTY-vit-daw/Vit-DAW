package pullharness

// budget.go — 观察预算与止损线（HARNESS_V1_DESIGN §3.4，路线图"第一天进
// 设计"义务的机制化）。
//
// 超限=止损终态 budget_exhausted——独立失败分类，不与
// no_candidate_found / capability_blocked 混记（AGENTS §8 分开记录义务；
// 终态常量间的互异性由 budget_test.go 锁定）。
// 上限语义沿既有 checkTurnBudget 约定（agentloop/runner.go:690）：
// 上限 >0 才设限，<=0 = 不设限。

import "strconv"

// ObservationBudget 是 §3.4 预算三量：
//   - MaxCycles：工具循环节上限（turn 语义=工具循环节，§2 OQ-1 裁决），
//     对应既有 checkTurnBudget 的 MaxTurns 面；
//   - ProbeCost / MaxProbeCost：render/probe 物理成本账户（D5 成本分级：
//     index 级零成本不计，render/probe 级逐笔计入）。ProbeCost 是run 入场
//     时的既有花费（续跑承接面），循环内由工具结果逐笔累加。
type ObservationBudget struct {
	MaxCycles    int     `json:"max_cycles,omitempty"`
	ProbeCost    float64 `json:"probe_cost,omitempty"`
	MaxProbeCost float64 `json:"max_probe_cost,omitempty"`
}

// 终态分类常量（G3 §6 指标口径的 outcome 分层）。
const (
	// OutcomeJudgmentOK 终局判定成功（模型无工具调用轮的缺省分类；真实
	// 失败分类沿 push 既有口径的接线归 IMPL-D）。
	OutcomeJudgmentOK = "judgment_ok"
	// OutcomeNoCandidateFound / OutcomeCapabilityBlocked 是 push 既有
	// 失败分类（诚实分记；A 阶段经 ClassifyTerminal 缝透传，不混记）。
	OutcomeNoCandidateFound  = "no_candidate_found"
	OutcomeCapabilityBlocked = "capability_blocked"
	// OutcomeBudgetExhausted 止损终态：独立分类（本文件核心义务）。
	OutcomeBudgetExhausted = "budget_exhausted"
	// OutcomeContextOverflow 溢出 fail-closed（复用
	// contextruntime.ModelContextOverflow 面，不静默截断）。
	OutcomeContextOverflow = "context_overflow"
	// OutcomeFastPath 快路径短路终态（分类口径归 IMPL-C 定，OQ-H2；
	// A 阶段 Router 未给出分类时的占位值）。
	OutcomeFastPath = "fastpath"
	// OutcomeLLMError 模型调用基础设施失败（AGENTS §8 环境中断独立记录）。
	OutcomeLLMError = "llm_error"
	// OutcomeInterrupted ctx 取消/环境中断面：非终态判定，不触发 T2
	//（暂停面语义：账本随会话载体存续）。
	OutcomeInterrupted = "interrupted"
	// OutcomeSessionError Session 协议违规/适配错误（L1-5-IMPL-D 腿1）：
	// fail-closed 直返——宿主状态未提交、零新工具，由入口层兜底清理
	//（§11.2 未知 Status/StopReason 同族处置）。
	OutcomeSessionError = "session_error"
)

// CycleExhausted 报告工具循环节上限是否用尽（MaxCycles<=0 = 不设限）。
func (b ObservationBudget) CycleExhausted(cycles int) bool {
	return b.MaxCycles > 0 && cycles >= b.MaxCycles
}

// ProbeExhausted 报告 probe 物理成本账户是否越线（MaxProbeCost<=0 = 不设限）。
func (b ObservationBudget) ProbeExhausted(spent float64) bool {
	return b.MaxProbeCost > 0 && spent >= b.MaxProbeCost
}

// Disclosure 渲染动态区预算披露行（§3.1 步骤②"动态区（会话状态+预算
// 披露）"；§4.2 表"pull 模式预算可见性是止损线的一部分"）。接近上限的
// 预警=机械判据（剩余循环节 <=1），不引入比例魔法数。
func (b ObservationBudget) Disclosure(cycles int, probeSpent float64) string {
	line := "budget_state: cycles=" + itoa(cycles)
	if b.MaxCycles > 0 {
		line += "/" + itoa(b.MaxCycles)
		if remaining := b.MaxCycles - cycles; remaining <= 1 {
			line += " budget_warning: cycle cap nearly spent"
		}
	} else {
		line += "/uncapped"
	}
	line += " probe_spent=" + ftoa(probeSpent)
	if b.MaxProbeCost > 0 {
		line += "/" + ftoa(b.MaxProbeCost)
		if b.ProbeExhausted(probeSpent) {
			line += " budget_warning: probe account over line"
		}
	} else {
		line += "/uncapped"
	}
	return line
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

// ftoa 用稳定可读的可变小数渲染（成本口径是累计浮点，不承诺往返精度）。
func ftoa(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
