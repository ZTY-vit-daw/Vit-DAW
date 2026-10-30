package agentloop

// free_state_automation_surface.go — M3-LLM-GATE-OPEN-1（闸 1 开放）：
// free-state 决策轮 automation 工具面准入。
//
// 终态架构（2026-10-30 判断权归属裁定·补充裁定）：意图层全 LLM——模型自主
// 调工具，治理门管写授权，确定性层正位=执行+审批+兜底。222 三道闸定案的
// 闸 1（决策轮 allowed tools=ccb.* only+骨架禁 mutation）在本面开放：
//
//   - 只读工具（plugin.get_parameters / automation.get_curve）直接放行；
//   - 写工具（automation.set_points / automation.clear_region）经治理门——
//     harness Invoke 的 RequiresConfirmation 判定（manual 模式=needs_confirmation
//     确认卡；显式完全权限=直执，223 stamp 形态）。触发权=模型工具调用，
//     与 Claude Code permission gate 同构；词表快道不动（退役是后续卡）。
//
// 准入只开「可调用」面，不携带任何写授权：零裸写由 harness 既有门保证
// （authorityAuthorized := req.Confirmed || explicitFullProjectAccess，负例
// 测试钉死）。chat 侧 free-state 分支（goalrunner_chat.go agentLoopToolContext）
// 仍只授 ccb 两工具——决策轮工具面是决策循环自身的相位策略，在 agentloop
// 侧并入（Start/Continue/ResumeAfterConfirmation 与 pull resumeContinue 四
// 入口各调一次 admitFreeStateDecisionRoundToolSurface，幂等）。
//
// 诊断专用轮（diagnostic-only 合同）不并入：该合同禁止任何动作，维持纯
// 观察面。骨架禁令的改写（mutation 一刀切禁→「automation 面经治理门」）
// 在 ruleset embed 与 legacy 常量 twin 落地（parity 测试锁定）。

import (
	"strings"
	"time"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/planner"
)

// freeStateAutomationSurfaceTools 是决策轮准入面的规范工具名（点分形态；
// 下划线别名与 get_plugin_parameters 由 name matcher 等价归一）。
var freeStateAutomationSurfaceTools = []string{
	"plugin.get_parameters",
	"automation.get_curve",
	"automation.set_points",
	"automation.clear_region",
}

// messageLoopIsFreeStateAutomationSurfaceName 报告工具名是否属于决策轮
// automation 准入面（点分/下划线/get_plugin_parameters 别名等价）。
func messageLoopIsFreeStateAutomationSurfaceName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plugin.get_parameters", "plugin_get_parameters", "get_plugin_parameters":
		return true
	case "automation.get_curve", "automation_get_curve":
		return true
	case "automation.set_points", "automation_set_points":
		return true
	case "automation.clear_region", "automation_clear_region":
		return true
	default:
		return false
	}
}

func messageLoopIsFreeStateAutomationSurfaceCall(call planner.ToolCall) bool {
	return messageLoopIsFreeStateAutomationSurfaceName(normalizedActionName(call, executorpkg.Result{}))
}

// freeStateAutomationSurfaceCanonicalName 把准入面别名归一为规范名（显示面
// 用：Allowed tools 行与目录行只出规范名）。
func freeStateAutomationSurfaceCanonicalName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plugin.get_parameters", "plugin_get_parameters", "get_plugin_parameters":
		return "plugin.get_parameters"
	case "automation.get_curve", "automation_get_curve":
		return "automation.get_curve"
	case "automation.set_points", "automation_set_points":
		return "automation.set_points"
	case "automation.clear_region", "automation_clear_region":
		return "automation.clear_region"
	default:
		return strings.TrimSpace(name)
	}
}

// messageLoopIsFreeStateAutomationMutationName 报告准入面内的写工具（治理门
// 对象）。读工具（get_parameters/get_curve）为 RiskDirect 直放。
func messageLoopIsFreeStateAutomationMutationName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "automation.set_points", "automation_set_points", "automation.clear_region", "automation_clear_region":
		return true
	default:
		return false
	}
}

// admitFreeStateDecisionRoundToolSurface 把 automation 准入面并入 free-state
// 决策轮的有效 AllowedTools。幂等；诊断专用轮与非 free-state 轮零变化。
func admitFreeStateDecisionRoundToolSurface(state *runState) {
	if state == nil || !messageLoopFreeStateActive(state) || messageLoopFreeStateDiagnosticOnly(state) {
		return
	}
	seen := make(map[string]bool, len(state.input.AllowedTools)+len(freeStateAutomationSurfaceTools))
	for _, tool := range state.input.AllowedTools {
		seen[strings.TrimSpace(tool)] = true
	}
	for _, tool := range freeStateAutomationSurfaceTools {
		if !seen[tool] {
			state.input.AllowedTools = append(state.input.AllowedTools, tool)
			seen[tool] = true
		}
	}
}

// noteFreeStateAutomationEngagement 是闸 2 配合（诉求命中即预留轮次）：free-state
// 轮首次实际执行 automation 面调用时给当前 slice 一次性 +4 turn headroom
// （对齐 chat 侧 settle-turn +4 先例，goalrunner_chat.go
// agentLoopBudgetForContext）——观察探索预算语义不动（不触发不加分），确认
// 恢复/继续切片自带新鲜预算，跨切片天然覆盖。
func noteFreeStateAutomationEngagement(state *runState) {
	if state == nil || state.freeStateAutomationHeadroomGiven {
		return
	}
	state.freeStateAutomationHeadroomGiven = true
	state.budget.MaxTurns += 4
	if state.budget.Timeout > 0 {
		state.budget.Timeout += 120 * time.Second
	}
}
