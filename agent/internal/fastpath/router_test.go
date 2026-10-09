package fastpath

// router_test.go — L1-5-IMPL-C 验收面：
//  1. 注册清单完整性断言（恰含平移的 10 项、无遗漏无多余、顺序=原 message_loop
//     preflight 链调用序）；
//  2. diagnostic-only 旁路开关测试（开启时零 Handler 调用、TryMatch 未命中）；
//  3. 注册序短路语义（首个 stopped=true 命中即返回，后续不调用）。
//
// 平移函数体的等价性证明=既有 agentloop 测试全绿（经委托壳零改动消费）+
// 回执的逐字节对比记录（fastpath 函数体反向改名后与 git HEAD 原体全等）。

import (
	"context"
	"testing"
)

func TestDefaultEntryNamesMatchesTranslatedPreflightChain(t *testing.T) {
	want := []string{
		"static_mix_capability_contract",
		"project_blackboard_status",
		"clip_fade_gain_set",
		"clip_fade_gain_read",
		"strip_silence_suggest",
		"clip_range_split",
		"stems_folder_import",
		"pending_section_markers_apply",
		"natural_mix_observation",
		"static_mix_gain_staging_context_pack",
	}
	if len(DefaultEntryNames) != 10 {
		t.Fatalf("DefaultEntryNames 长度=%d，期望恰 10 项（平移清单契约）", len(DefaultEntryNames))
	}
	for i, name := range want {
		if DefaultEntryNames[i] != name {
			t.Fatalf("DefaultEntryNames[%d]=%q，期望 %q（顺序=原 loop() preflight 链调用序）", i, DefaultEntryNames[i], name)
		}
	}
}

func TestTryMatchWalksInRegistrationOrderAndShortCircuits(t *testing.T) {
	type st struct{ visited []string }
	mk := func(name string, hit bool) Entry[*int, *st, string] {
		return Entry[*int, *st, string]{Name: name, Handler: func(_ context.Context, _ *int, s *st) (bool, string) {
			s.visited = append(s.visited, name)
			return hit, "result-of-" + name
		}}
	}
	rt := NewRouter(mk("a", false), mk("b", true), mk("c", true), mk("d", false))
	state := &st{}
	stopped, result, entry := rt.TryMatch(context.Background(), nil, state)
	if !stopped {
		t.Fatalf("期望命中短路")
	}
	if result != "result-of-b" || entry != "b" {
		t.Fatalf("命中项错误：entry=%q result=%q（期望 b / result-of-b）", entry, result)
	}
	if len(state.visited) != 2 || state.visited[0] != "a" || state.visited[1] != "b" {
		t.Fatalf("调用序错误：%v（期望按注册序 a→b，命中后 c/d 不再调用）", state.visited)
	}
}

func TestTryMatchMissReturnsZero(t *testing.T) {
	rt := NewRouter[*int, *int, string](
		Entry[*int, *int, string]{Name: "never", Handler: func(context.Context, *int, *int) (bool, string) { return false, "" }},
	)
	stopped, result, entry := rt.TryMatch(context.Background(), nil, nil)
	if stopped || result != "" || entry != "" {
		t.Fatalf("未命中应返回 (false, 零值, %q)，实得 (%t, %q, %q)", "", stopped, result, entry)
	}
}

func TestDiagnosticOnlyBypassesAllEntries(t *testing.T) {
	calls := 0
	rt := NewRouter[*int, *int, string](
		Entry[*int, *int, string]{Name: "would_hit", Handler: func(context.Context, *int, *int) (bool, string) {
			calls++
			return true, "terminal"
		}},
	)
	rt.SetDiagnosticOnly(true)
	if !rt.DiagnosticOnly() {
		t.Fatalf("开关状态未生效")
	}
	stopped, result, entry := rt.TryMatch(context.Background(), nil, nil)
	if stopped || result != "" || entry != "" {
		t.Fatalf("diagnostic-only 下应整体旁路（未命中），实得 (%t, %q, %q)", stopped, result, entry)
	}
	if calls != 0 {
		t.Fatalf("diagnostic-only 下 Handler 被调用 %d 次（期望 0：旁路必须先于任何确定性 preflight）", calls)
	}
	rt.SetDiagnosticOnly(false)
	stopped, result, entry = rt.TryMatch(context.Background(), nil, nil)
	if !stopped || result != "terminal" || entry != "would_hit" {
		t.Fatalf("关闭旁路后应恢复命中：(%t, %q, %q)", stopped, result, entry)
	}
	if calls != 1 {
		t.Fatalf("关闭旁路后 Handler 调用次数=%d（期望 1）", calls)
	}
}

func TestRegisterAppendsInOrder(t *testing.T) {
	rt := NewRouter[*int, *int, string](Entry[*int, *int, string]{Name: "first"})
	rt.Register(Entry[*int, *int, string]{Name: "second"})
	names := rt.Names()
	if len(names) != 2 || names[0] != "first" || names[1] != "second" {
		t.Fatalf("Register 应保持追加序：%v", names)
	}
}

// TestMovedMatcherSanity — 平移面金测（行为锚定样本，非穷尽；等价性主证=既有
// agentloop 测试经委托壳全绿 + 回执逐字节对比）。
func TestMovedMatcherSanity(t *testing.T) {
	if got := Text(nil); got != "" {
		t.Fatalf("Text(nil)=%q，期望空串（<nil> 规范化）", got)
	}
	if got := Text("  值  "); got != "值" {
		t.Fatalf("Text 修剪失效：%q", got)
	}
	if !TextHasAny("帮我把主唱声音调亮一点", "主唱", "vocal") {
		t.Fatalf("TextHasAny 中文命中失效")
	}
	if TextHasAny("无关文本", "主唱") {
		t.Fatalf("TextHasAny 误命中")
	}
	if key := ClipRangeSplitCutKey(map[string]any{"clip_id": " c1 ", "split_time": 1.5}); key != "c1@1.5" {
		t.Fatalf("ClipRangeSplitCutKey=%q，期望 c1@1.5", key)
	}
	if key := ClipRangeSplitCutKey(map[string]any{"clip_id": "c1", "split_time": "2.25"}); key != "c1@2.25" {
		t.Fatalf("ClipRangeSplitCutKey 字符串时间解析失效：%q", key)
	}
	if ClipFadeGainReadRequest("看一下当前片段的 fade 和 gain 状态") != ClipFadeGainReadRequest("看一下当前片段的 fade 和 gain 状态") {
		t.Fatalf("确定性匹配不稳定")
	}
	if !ClipFadeGainReadRequest("看一下当前片段的 fade 和 gain 状态") {
		t.Fatalf("clip fade/gain 读取意图匹配失效（平移面回归）")
	}
	if !NaturalMixRequest("帮我把人声往前放，提升响度") {
		t.Fatalf("自然混音意图匹配失效（平移面回归）")
	}
	if !StripSilenceSuggestRequest("清理静音，给个建议") {
		t.Fatalf("strip silence 建议意图匹配失效（平移面回归）")
	}
	if GainStagingExplicitRequest("把音量调大") {
		t.Fatalf("gain staging 显式请求误命中（ClipFadeGainRequest 排除分支失效）")
	}
}
