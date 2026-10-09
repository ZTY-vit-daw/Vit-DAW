package pullharness

// disclosure.go — 披露位迁移（HARNESS_V1_DESIGN §4.2 处置表四行，
// L1-5-IMPL-B）。语义沿 ccb_model_prompt.go:220/:273/:358/:582 既有文本，
// 旧 harness 零改动（复用语义不复用其内部函数）；新侧文本内容盲（无域
// 处理知识——coldstart_blind_test.go 机械断言覆盖底座+动态区两段）。
//
// 四行去向：
//   行 1  GATE PATH 预披露（:220/:273）→ 冷启动底座规则段（本文件
//         coldStartRulesContent，经 coldstart.go 挂载）——一次性指引，
//         不再逐轮复现；缺证 gap 的 machine-readable 结构沿响应面既有
//         语义保留（不属装配面）。
//   行 2  G1-G8 证据门指引（:582）→ 冷启动底座规则段（L1 规则域）。
//   行 3  TERMINAL TURN（:358）→ 动态区逐轮状态指令（本文件
//         terminalTurnRow）——机械状态头+终态轮指令语义。
//   行 4  预算指令 → 动态区 BudgetState 披露（budget.Disclosure 既有面）
//         + 预算指令行（本文件 budgetInstructionRow；§3.4 预警落点——
//         pull 模式预算可见性是止损线的一部分）。

import "strings"

// 披露位锚短语（段首稳定词；四行就位断言与内容盲扫描的定位键）。
const (
	anchorGatePath      = "GATE PATH (standing rule disclosure):"
	anchorEvidenceGates = "EVIDENCE GATES (G1-G8, standing rule disclosure):"
	anchorTerminalTurn  = "TERMINAL TURN (mechanical runtime state):"
	anchorBudget        = "BUDGET (mechanical runtime state):"
)

// coldStartRulesContent 渲染冷启动底座规则段（行 1+行 2；会话首装一次，
// 之后字节恒定）。两行均机械指引（准入纪律+目录结构），无域处理知识。
func coldStartRulesContent() string {
	return strings.Join([]string{
		anchorGatePath + " proposal admission is decided by machine-checkable evidence recorded in runtime state, never by intent: a proposal whose evidence is not yet recorded is refused with a machine-readable structured gap. Walk the gate path first — make catalog discovery the first observation step (ccb.observation_catalog), request the evidence-bearing views (ccb.observation_request), then propose from the returned evidence.",
		anchorEvidenceGates + " admission is decided only by the evidence gates: project binding, capacity, project scan, a closed dimension, the candidate frontier, target-level evidence, fresh revision-bound citations, target consistency. Do not rush the proposal; rush the gate path. Candidate evidence is not itself permission to act: a candidate frontier is built from observation facts that disclose track-level candidates, and a target is confirmed by a target-level track.* observation of one concrete candidate track. Choose view dimensions yourself from the returned catalog and the material at hand — there is no default view sequence and no privileged dimension. Re-requesting views that already returned, or observing after the target-level evidence you need is already in hand, only consumes budget.",
	}, "\n")
}

// dynamicStateRows 渲染动态区逐轮状态行（行 3+行 4；每轮重算，随轮次
// 计数与预算账户变化——动态区语义，不入前缀）。行序：BudgetState 披露
// （机械事实）→ TERMINAL TURN 指令 → BUDGET 指令。
func dynamicStateRows(cycles int, probeSpent float64, budget ObservationBudget) []string {
	return []string{
		budget.Disclosure(cycles, probeSpent),
		terminalTurnRow(cycles, budget),
		budgetInstructionRow(),
	}
}

// terminalTurnRow 是行 3 落点：终态轮机械状态头（剩余循环节计数，:358
// 的 counters 形态）+ 终态轮指令语义（无工具调用轮=终态轮，其回复即终局
// 判定；仍需观察/执行时返回工具调用）。
func terminalTurnRow(cycles int, budget ObservationBudget) string {
	remaining := "uncapped"
	if budget.MaxCycles > 0 {
		remaining = itoa(budget.MaxCycles - cycles)
	}
	return anchorTerminalTurn + " tool-call cycles remaining: " + remaining +
		". A response with no tool calls is the terminal turn: it ends the run and its reply is the final judgment (success or an explicit terminal classification). While further evidence or governed execution is still needed, return tool calls instead."
}

// budgetInstructionRow 是行 4 的指令语义落点：预算记账规则+止损指令
// （预算可见性本身由 budget_state 行承载；§3.4——接近阈值的机械预警由
// Disclosure 的 budget_warning 承担）。
func budgetInstructionRow() string {
	return anchorBudget + " each executed tool batch consumes one cycle and render/probe-class tool costs are charged to the probe account shown in budget_state; when the remaining budget cannot cover further observation, return the terminal turn (no tool calls) instead of exceeding the line."
}
