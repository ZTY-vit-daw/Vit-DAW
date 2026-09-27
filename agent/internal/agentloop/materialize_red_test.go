package agentloop

// materialize_red_test.go — E16 红测（现状锁定，非修复；MAT-C §5.4）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §5.4（G2-C 素材，RECON §2/§5.3
// 指认 E16）：B1 pack 同 run 建后不失效——preflightStaticMixGainStagingContextPack
// 入口以 messageLoopHasGainStagingContextPack（trace 里存在 capability_context_pack
// 事件）短路，run 内工程变更（project.state revision 推进）不触发 pack 重建或
// 失效。"记忆型输入无失效钩子"的活样本看门狗。
//
// 红测纪律（AGENTS §10）：本测试锁定**现状行为**并显式标注"已知隐患，修复卡
// （OQ-6：B1 pack 输入域改造）合入后升级断言：工程变更触发 pack 失效/重建"。

import (
	"context"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/planner"
)

// TestB1PackSurvivesProjectChangeInRun：run 内 pack 已建 → 注入工程变更
// （executed 中更晚的 project.state 记录 revision 从 r1 推进到 r2、推子改值）
// → 现状锁定：preflight 直接短路（不重建、不失效、trace 不增长）。
func TestB1PackSurvivesProjectChangeInRun(t *testing.T) {
	state := &runState{
		input: Input{UserText: "reset all B1 track faders to 0 dB"},
		// pack 已建（本 run 早期的 B1 预检留下的痕迹事件）。
		trace: []planner.TraceEvent{{
			Kind:    "capability_context_pack",
			Message: capabilitycontext.GainStagingCapabilityID + " default pack built",
		}},
		// pack 建后工程变更：更晚的 project.state 记录 revision=r2、T3 推子改到 -6。
		executed: []map[string]any{
			{"tool": "project.state", "result": map[string]any{
				"project_revision": "r1",
				"tracks": []map[string]any{{"track_id": "T3", "track_name": "lead", "volume_db": 0.0}},
			}},
			{"tool": "project.state", "result": map[string]any{
				"project_revision": "r2",
				"tracks": []map[string]any{{"track_id": "T3", "track_name": "lead", "volume_db": -6.0}},
			}},
		},
	}
	if !messageLoopHasGainStagingContextPack(state) {
		t.Fatalf("前置失败：pack 痕迹事件未被识别（gate 语义漂移）")
	}

	// 现状锁定（E16 本体）：pack 在场 ⇒ preflight 短路——工程变更不触发重建。
	// 修复卡（OQ-6）合入后升级断言：r2 变更触发 pack 失效/重建（本调用返回
	// 重建路径或 trace 增长）。
	l := &MessageLoop{}
	traceBefore := len(state.trace)
	stopped, result := l.preflightStaticMixGainStagingContextPack(context.Background(), nil, state)
	if stopped {
		t.Fatalf("现状下 preflight 应短路返回（stopped=false）")
	}
	if result.Status != "" || result.Reply != "" {
		t.Fatalf("现状下 preflight 应返回空 Result：%+v", result)
	}
	if got := len(state.trace); got != traceBefore {
		t.Fatalf("现状下 trace 不应增长（pack 不重建）：before=%d after=%d", traceBefore, got)
	}
	// 看门狗本体：pack 痕迹在工程变更后依然判定"已有 pack"——无失效钩子。
	if !messageLoopHasGainStagingContextPack(state) {
		t.Fatalf("现状锁定失败：工程变更不应移除 pack 判定")
	}
}
