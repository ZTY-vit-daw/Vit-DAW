package chat

// audio_closure_turn_budget_test.go — M3-LLM-TURN-BUDGET-1（229 面1/面2）：
// closure 相位预算的意图感知分配测试面。
//
//   - 面1：automation 诉求决策轮（active free-state loop+非诊断专用）保留
//     mode 基础预算（不落 1-2 turn 钳制）；诊断专用轮与非 free-state
//     closure 轮的原观察 slice 语义零回退。
//   - 面2：异步续跑片（scheduler durable continuation 的 context 形态）
//     与决策轮同预算语义——同一预算函数、同一结构判定，无绕行路径。

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
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

// 面1 主面：automation 诉求决策轮保留 mode 基础预算——MaxTurns 不落 1-2
// 钳制、MaxToolCalls 保持 mode 面（12/16），预算跟工具面（227 准入面）同源。
func TestAudioClosureBudgetAutomationRoundKeepsModeBase(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	got := audioClosureMessageLoopBudget(base, turnBudgetAutomationLoopContext())
	if got.MaxTurns != 10 {
		t.Fatalf("automation decision round must keep the mode base MaxTurns, got %d", got.MaxTurns)
	}
	if got.MaxToolCalls != 16 {
		t.Fatalf("automation decision round must keep the mode base MaxToolCalls, got %d", got.MaxToolCalls)
	}
	// settle headroom（agentLoopBudgetForContext 已加的 +4）不被预算函数剥离。
	withSettle := agentloop.Budget{MaxTurns: 14, MaxToolCalls: 16}
	if got := audioClosureMessageLoopBudget(withSettle, turnBudgetAutomationLoopContext()); got.MaxTurns != 14 {
		t.Fatalf("settle headroom must survive the automation round budget, got %d", got.MaxTurns)
	}
}

// 面1 衰减面：诊断专用轮（顶层键或 loop map 键）不享受扩面——纯观察面
// 语义与 agentloop 侧 admitFreeStateDecisionRoundToolSurface 的诊断排除
// 同源对齐。
func TestAudioClosureBudgetDiagnosticRoundKeepsClamp(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	topLevel := turnBudgetAutomationLoopContext()
	topLevel["free_state_diagnostic_only"] = true
	if got := audioClosureMessageLoopBudget(base, topLevel); got.MaxTurns != 1 {
		t.Fatalf("diagnostic-only round must keep the closure clamp, got MaxTurns=%d", got.MaxTurns)
	}
	if got := audioClosureMessageLoopBudget(base, topLevel); got.MaxToolCalls != 6 {
		t.Fatalf("diagnostic-only round must keep the observation tool clamp, got MaxToolCalls=%d", got.MaxToolCalls)
	}
	loopKeyed := turnBudgetAutomationLoopContext()
	loopKeyed["free_state_reasoning_loop"].(map[string]any)["diagnostic_only"] = true
	if got := audioClosureMessageLoopBudget(base, loopKeyed); got.MaxTurns != 1 {
		t.Fatalf("loop-keyed diagnostic round must keep the closure clamp, got MaxTurns=%d", got.MaxTurns)
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

// 面2 主面：scheduler durable continuation 片的 context 形态（loop map 经
// durable merge 携带）与决策轮同预算语义——异步续跑不是预算绕行路径。
func TestAudioClosureBudgetContinuationSliceInheritsDecisionRoundBudget(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 10, MaxToolCalls: 16}
	// durable 续跑片的 context 形态：续跑调度把 durable_continuation 等
	// 标记并入同一 context，free_state_reasoning_loop 以 map 形态在场。
	continuation := turnBudgetAutomationLoopContext()
	continuation["durable_continuation"] = true
	continuation["continuation_id"] = "cont_budget_1"
	got := audioClosureMessageLoopBudget(base, continuation)
	if got.MaxTurns != 10 {
		t.Fatalf("continuation slice must inherit the decision-round budget, got MaxTurns=%d", got.MaxTurns)
	}
	if got.MaxToolCalls != 16 {
		t.Fatalf("continuation slice must inherit the mode tool budget, got MaxToolCalls=%d", got.MaxToolCalls)
	}
}
