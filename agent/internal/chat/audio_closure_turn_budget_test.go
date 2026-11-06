package chat

// audio_closure_turn_budget_test.go — M3-LLM-TURN-BUDGET-1（229 面1/面2）+
// M3-AUTOMATION-SLICE-BUDGET-OPEN-1（232）+ CLOSURE-ROUTE-ADAPT-1（271 案 B，
// 2026-11-07 用户裁定获准）：closure 相位预算的意图感知分配测试面。
//
//   - 面1：automation 诉求决策轮（active free-state loop+非诊断专用）预算
//     放开至 232 余量下限（48 turns / 80 calls，floor 语义——不剥离既有
//     headroom、不缩小更大 base）。
//   - 面1-271 修订（授权依据：2026-11-07 案 B 获准裁定，测试意图修订禁静默
//     改期望值）：诊断闭包轮不再保持 1-2 turn 钳制，保留调用方基础预算。
//     修订依据=276 rerun1 卷内证据（closure mode=diagnostic、
//     phase=fs2_capacity_assessed、frontier 空 → MaxTurns=1 实际生效；模型
//     单轮 86 tok 零工具调用即 limit_reached，one-probe-one-verdict 单轮
//     假设被实证打破，命令面未触达）——旧钉
//     TestAudioClosureBudgetDiagnosticRoundKeepsClamp 钉死的正是该失效假设。
//     treatment 轮与非诊断 legacy closure 轮的观察 slice 语义零回退
//     （mode 空/未知 fail-closed 回钳制）。
//   - 面2：异步续跑片（scheduler durable continuation 的 context 形态）
//     与决策轮同预算语义——同一预算函数、同一结构判定，无绕行路径。

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
)

func turnBudgetAutomationLoopContext() map[string]any {
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-budget-1",
		ConversationID: "conversation-budget",
		Status:         "reasoning",
		DecisionPhase:  "processor_selection",
		OriginalIntent: "这段让它越来越突出一点",
	}
	return map[string]any{"free_state_reasoning_loop": freeStateLoopMap(loop)}
}

// 面1 主面（232 更新）：automation 诉求决策轮片预算放开至余量下限——
// MaxTurns/MaxToolCalls 不落 1-2 钳制、不落 mode 基础面（8-14），抬到 48/80
// 余量下限（链长 24-30 轮 ×1.5 推导，见 audioClosureAutomationRoundMaxTurns
// 注释），预算跟工具面（227 准入面）同源。
func TestAudioClosureBudgetAutomationRoundOpensToHeadroomFloor(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	got := audioClosureMessageLoopBudget(base, turnBudgetAutomationLoopContext())
	if got.MaxTurns != audioClosureAutomationRoundMaxTurns {
		t.Fatalf("automation decision round must open to the headroom-floor MaxTurns, got %d", got.MaxTurns)
	}
	if got.MaxToolCalls != audioClosureAutomationRoundMaxToolCalls {
		t.Fatalf("automation decision round must open to the headroom-floor MaxToolCalls, got %d", got.MaxToolCalls)
	}
	// settle headroom（agentLoopBudgetForContext 已加的 +4）不被预算函数剥离：
	// floor 语义取 max，14 仍抬到 48。
	withSettle := agentloop.Budget{MaxTurns: 14, MaxToolCalls: 16}
	if got := audioClosureMessageLoopBudget(withSettle, turnBudgetAutomationLoopContext()); got.MaxTurns != audioClosureAutomationRoundMaxTurns {
		t.Fatalf("settle headroom must survive the automation round budget floor, got %d", got.MaxTurns)
	}
	// floor 不是钳制：更大的 base（未来 mode 调整/更宽 headroom）不被缩小。
	larger := agentloop.Budget{MaxTurns: 60, MaxToolCalls: 100}
	if got := audioClosureMessageLoopBudget(larger, turnBudgetAutomationLoopContext()); got.MaxTurns != 60 || got.MaxToolCalls != 100 {
		t.Fatalf("automation round floor must not narrow a larger base, got %d/%d", got.MaxTurns, got.MaxToolCalls)
	}
}

// 面1-271 修订面（案 B 主面，授权修订——原
// TestAudioClosureBudgetDiagnosticRoundKeepsClamp 的 1-turn 钳制期望在此废止，
// 修订依据与 276 卷内证据见文件头注释，禁静默改期望值条款已履行）：
// 诊断闭包轮保留调用方基础预算，不落 1-2 钳制、不开自动化 48/80。
// 同源钉法：base 直接取自 agentLoopBudgetForContext / agentLoopBudgetForMode，
// 断言的是「保留」而非某个硬编码值。
func TestAudioClosureBudgetDiagnosticRoundKeepsDiagnosticBaseBudget(t *testing.T) {
	// 场景 A（同源 5）：free_state_diagnostic_only 轮——agentLoopBudgetForContext
	// 已为诊断链定价 5 turns/6 calls（goalrunner_chat.go 诊断链注释），271 案 B
	// 消除「同一诊断语义 5 vs 1 双答案」欠账：closure 在场时保留 5。
	flagContext := turnBudgetAutomationLoopContext()
	flagContext["free_state_diagnostic_only"] = true
	flagContext[audioClosureContextKey] = map[string]any{"mode": string(audioclosure.ModeDiagnostic)}
	base := agentLoopBudgetForContext(agentModeGoal, flagContext)
	if base.MaxTurns != 5 {
		t.Fatalf("agentLoopBudgetForContext must price the diagnostic-only round at 5 turns (271 same-source anchor), got %d", base.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(base, flagContext); got.MaxTurns != base.MaxTurns {
		t.Fatalf("diagnostic-only closure round must keep the caller base budget, got MaxTurns=%d want %d", got.MaxTurns, base.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(base, flagContext); got.MaxToolCalls != 6 {
		t.Fatalf("diagnostic round must keep the 6-call observation clamp, got MaxToolCalls=%d", got.MaxToolCalls)
	}
	// 场景 B（276 rerun1 回归钉）：closure mode=diagnostic、无 flag 无 loop、
	// goal base=10/16（276 卷内形态：frontier 空 → 原 clamp 分支 MaxTurns=1
	// 生效即停轮）。案 B 后 base 保留——模型有轮次完成首次探测；工具钳制 ≤6
	// 与轮边界熔断（MaxClosureRounds/no-progress）不变。
	diagOnly := map[string]any{audioClosureContextKey: map[string]any{"mode": string(audioclosure.ModeDiagnostic)}}
	goalBase := agentLoopBudgetForMode(agentModeGoal)
	if got := audioClosureMessageLoopBudget(goalBase, diagOnly); got.MaxTurns != goalBase.MaxTurns {
		t.Fatalf("276-shaped diagnostic closure round must keep the mode base, got MaxTurns=%d want %d", got.MaxTurns, goalBase.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(goalBase, diagOnly); got.MaxToolCalls != 6 {
		t.Fatalf("276-shaped diagnostic round must keep the 6-call clamp, got MaxToolCalls=%d", got.MaxToolCalls)
	}
	// 场景 C（fail-closed，legacy 零回退）：mode 缺省/未知/treatment 的
	// closure 轮 verbatim 回 229 钳制——持久化兼容红线（未知缺省语义
	// fail-closed），旧工件形态不因 271 扩面。
	for name, mode := range map[string]any{"empty": "", "unknown": "legacy-unknown-mode", "treatment": string(audioclosure.ModeTreatment)} {
		legacy := map[string]any{audioClosureContextKey: map[string]any{"mode": mode}}
		if got := audioClosureMessageLoopBudget(goalBase, legacy); got.MaxTurns != 1 {
			t.Fatalf("%s-mode closure round must keep the 229 clamp, got MaxTurns=%d", name, got.MaxTurns)
		}
	}
	// 诊断轮续跑片（229 side 2 契约保持）：durable continuation 形态经同一
	// 函数重推导，closure state 重绑随卷携带 mode=diagnostic——续跑片继承
	// 同一 base，无绕行路径。
	continuation := map[string]any{audioClosureContextKey: map[string]any{"mode": string(audioclosure.ModeDiagnostic)}}
	continuation["durable_continuation"] = true
	continuation["continuation_id"] = "cont_diag_271"
	if got := audioClosureMessageLoopBudget(goalBase, continuation); got.MaxTurns != goalBase.MaxTurns {
		t.Fatalf("diagnostic continuation slice must inherit the kept base budget, got MaxTurns=%d want %d", got.MaxTurns, goalBase.MaxTurns)
	}
	// 诊断轮 terminal_turn_locked（BOUNDARY-1）：≥2 的重试空间由保留的 base
	// 自然覆盖（base≥5>2），无独立分支——钉住不回归。
	locked := map[string]any{audioClosureContextKey: map[string]any{"mode": string(audioclosure.ModeDiagnostic)}}
	locked["free_state_reasoning_loop"] = map[string]any{"terminal_turn_locked": true}
	if got := audioClosureMessageLoopBudget(goalBase, locked); got.MaxTurns < 2 {
		t.Fatalf("terminal-turn-locked diagnostic round must keep retry headroom, got MaxTurns=%d", got.MaxTurns)
	}
}

// 面1-271 衰减面（treatment 轮零影响）：治疗闭包轮的钳制与候选选择语义
// verbatim 保持——诊断豁免不得外溢到 treatment 面。
func TestAudioClosureBudgetTreatmentRoundClampUnchanged(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	treatment := map[string]any{audioClosureContextKey: map[string]any{"mode": string(audioclosure.ModeTreatment)}}
	if got := audioClosureMessageLoopBudget(base, treatment); got.MaxTurns != 1 {
		t.Fatalf("treatment round must keep the closure clamp, got MaxTurns=%d", got.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(base, treatment); got.MaxToolCalls != 6 {
		t.Fatalf("treatment round must keep the observation tool clamp, got MaxToolCalls=%d", got.MaxToolCalls)
	}
	// treatment 轮的候选选择两 turn 语义（未选=2/已选=1）保持。
	withFrontier := map[string]any{audioClosureContextKey: map[string]any{
		"mode":                string(audioclosure.ModeTreatment),
		"hypothesis_frontier": map[string]any{"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}}},
	}}
	if got := audioClosureMessageLoopBudget(base, withFrontier); got.MaxTurns != 2 {
		t.Fatalf("treatment candidate selection must keep two turns, got MaxTurns=%d", got.MaxTurns)
	}
	withFrontier[audioClosureContextKey].(map[string]any)["hypothesis_frontier"].(map[string]any)["candidate_id"] = "candidate-1"
	if got := audioClosureMessageLoopBudget(base, withFrontier); got.MaxTurns != 1 {
		t.Fatalf("selected-candidate treatment slice must keep one turn, got MaxTurns=%d", got.MaxTurns)
	}
}

// 面1-271 顺序红线（自动化轮不受诊断豁免影响）：closure mode=diagnostic 且
// free-state 决策轮活跃（非诊断专用）时，automation 分支先于诊断豁免——
// 豁免分支插入点不得越过 automation 判定，预算面顺序契约钉死。
func TestAudioClosureBudgetAutomationRoundUnaffectedByDiagnosticExemption(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	both := turnBudgetAutomationLoopContext()
	both[audioClosureContextKey] = map[string]any{"mode": string(audioclosure.ModeDiagnostic)}
	got := audioClosureMessageLoopBudget(base, both)
	if got.MaxTurns != audioClosureAutomationRoundMaxTurns {
		t.Fatalf("automation round must stay on the headroom floor even with a diagnostic closure mode, got MaxTurns=%d", got.MaxTurns)
	}
	if got.MaxToolCalls != audioClosureAutomationRoundMaxToolCalls {
		t.Fatalf("automation round must stay on the headroom-floor tool budget, got MaxToolCalls=%d", got.MaxToolCalls)
	}
}

// 面1 衰减面：终态 loop（completed/blocked 等）不是决策轮——回到观察
// slice 钳制，防止预算扩面借终态 loop 残留常驻。
func TestAudioClosureBudgetTerminalLoopKeepsClamp(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-budget-terminal",
		ConversationID: "conversation-budget",
		Status:         "completed",
		OriginalIntent: "这段让它越来越突出一点",
	}
	got := audioClosureMessageLoopBudget(base, map[string]any{"free_state_reasoning_loop": freeStateLoopMap(loop)})
	if got.MaxTurns != 1 {
		t.Fatalf("terminal loop must keep the closure clamp, got MaxTurns=%d", got.MaxTurns)
	}
}

// 面1 零回退面：无 free-state loop 的 legacy closure 轮保持候选选择语义
// （未选=2/已选=1）与观察工具钳制（≤6）。
func TestAudioClosureBudgetLegacyObservationZeroRegression(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	legacy := map[string]any{audioClosureContextKey: map[string]any{"hypothesis_frontier": map[string]any{
		"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}},
	}}}
	if got := audioClosureMessageLoopBudget(base, legacy); got.MaxTurns != 2 {
		t.Fatalf("legacy candidate selection must keep two turns, got %d", got.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(base, legacy); got.MaxToolCalls != 6 {
		t.Fatalf("legacy observation slice must keep the 6-call clamp, got %d", got.MaxToolCalls)
	}
	legacy[audioClosureContextKey].(map[string]any)["hypothesis_frontier"].(map[string]any)["candidate_id"] = "candidate-1"
	if got := audioClosureMessageLoopBudget(base, legacy); got.MaxTurns != 1 {
		t.Fatalf("selected-candidate slice must keep one turn, got %d", got.MaxTurns)
	}
}

// 面2 主面（232 更新）：scheduler durable continuation 片的 context 形态
// （loop map 经 durable merge 携带）与决策轮同预算语义——异步续跑不是预算
// 绕行路径，同样抬到 48/80 余量下限。
func TestAudioClosureBudgetContinuationSliceInheritsDecisionRoundBudget(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	// durable 续跑片的 context 形态：续跑调度把 durable_continuation 等
	// 标记并入同一 context，free_state_reasoning_loop 以 map 形态在场。
	continuation := turnBudgetAutomationLoopContext()
	continuation["durable_continuation"] = true
	continuation["continuation_id"] = "cont_budget_1"
	got := audioClosureMessageLoopBudget(base, continuation)
	if got.MaxTurns != audioClosureAutomationRoundMaxTurns {
		t.Fatalf("continuation slice must inherit the decision-round headroom-floor budget, got MaxTurns=%d", got.MaxTurns)
	}
	if got.MaxToolCalls != audioClosureAutomationRoundMaxToolCalls {
		t.Fatalf("continuation slice must inherit the headroom-floor tool budget, got MaxToolCalls=%d", got.MaxToolCalls)
	}
}
