package agentloop

// materialize_red_test.go — E16 修复验证（FIX-STALE-SAMPLES-1；MAT-C 现状锁定
// 红测的升级断言，规格权威 docs/MATERIALIZATION_V1_DESIGN.md §5.4）。
//
// 原 E16 隐患：B1 pack 同 run 建后不失效——preflightStaticMixGainStaging-
// ContextPack 入口以 messageLoopHasGainStagingContextPack（trace 里存在
// capability_context_pack 事件）短路，run 内工程变更（project.state revision
// 推进）不触发 pack 重建或失效，"记忆型输入无失效钩子"活样本。
//
// 修复断言：pack 建立后工程 revision 推进 ⇒ pack 失效，preflight 进入重建路径
//（不再静默短路）。§11 裁定（卡内，回执在案）：pack 与当前工程态都无
// revision 可见时无法证明变化，维持现状不失效。

import (
	"context"
	"testing"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/planner"
)

// TestB1PackSurvivesProjectChangeInRun：run 内 pack 已建 → 注入工程变更
// （executed 中更晚的 project.state 记录 revision 从 r1 推进到 r2、推子改值）
// → 修复断言：pack 痕迹仍在（不删痕迹），但工程变更触发失效/重建——
// preflight 不再静默短路（进入重建路径：stopped 或 trace 增长）。
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
				"tracks":           []map[string]any{{"track_id": "T3", "track_name": "lead", "volume_db": 0.0}},
			}},
			{"tool": "project.state", "result": map[string]any{
				"project_revision": "r2",
				"tracks":           []map[string]any{{"track_id": "T3", "track_name": "lead", "volume_db": -6.0}},
			}},
		},
	}
	if !messageLoopHasGainStagingContextPack(state) {
		t.Fatalf("前置失败：pack 痕迹事件未被识别（gate 语义漂移）")
	}

	// 修复断言（E16 本体）：工程变更后 preflight 不得静默短路——pack 失效并
	// 进入重建路径（本调用 stopped 或 trace 增长）。
	l := &MessageLoop{}
	r := &Runner{}
	traceBefore := len(state.trace)
	stopped, result := l.preflightStaticMixGainStagingContextPack(context.Background(), r, state)
	if !stopped && len(state.trace) == traceBefore {
		t.Fatalf("E16 修复断言失败：工程变更（r1→r2）后 preflight 应进入重建路径（stopped 或 trace 增长）；现状为静默短路（stopped=%t status=%q trace %d→%d）",
			stopped, result.Status, traceBefore, len(state.trace))
	}
}

// TestB1PackStalenessRevisionSemantics — E16 修复的失效语义钉死：
// 同 revision 不失效（建后复用）；异 revision 失效；双端无 revision 可见
// 维持现状（§11）；pack 无注记而工程态出现 revision 视为可证明变化（失效）。
func TestB1PackStalenessRevisionSemantics(t *testing.T) {
	packEvent := func(revision string) planner.TraceEvent {
		message := capabilitycontext.GainStagingCapabilityID + " default pack built"
		if revision != "" {
			message += " project_revision=" + revision
		}
		return planner.TraceEvent{Kind: "capability_context_pack", Message: message}
	}
	projectState := func(revision string) []map[string]any {
		return []map[string]any{{"tool": "project.state", "result": map[string]any{
			"project_revision": revision,
			"tracks":           []map[string]any{{"track_id": "T3", "track_name": "lead", "volume_db": -6.0}},
		}}}
	}
	scenarios := []struct {
		name      string
		built     string
		current   string
		wantStale bool
	}{
		{"同 revision 不失效", "r2", "r2", false},
		{"异 revision 失效", "r1", "r2", true},
		{"§11 双端无 revision 维持现状", "", "", false},
		{"pack 无注记而工程态出现 revision 失效", "", "r2", true},
	}
	for _, scenario := range scenarios {
		state := &runState{
			trace:    []planner.TraceEvent{packEvent(scenario.built)},
			executed: projectState(scenario.current),
		}
		if got := messageLoopGainStagingPackStale(state); got != scenario.wantStale {
			t.Fatalf("%s：messageLoopGainStagingPackStale=%t（期望 %t）", scenario.name, got, scenario.wantStale)
		}
	}
}
