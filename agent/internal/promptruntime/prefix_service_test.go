package promptruntime

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
)

// L1-4-IMPL-A T-A 系（服务级）：判据锚 CONTEXT_LAYERING_V1_DESIGN §3.4/§6.1。
// P1 = 无语义事件相邻轮 fingerprint(N-1) 字节串是 fingerprint(N) 前缀；
// P2 = 前缀字节变化必有封闭枚举归因（Breaks 非空或 CacheAnomalies 非空）；
// P3 = 无事件轮次稳定 system 消息字节恒等。

func stableSection(id, content string) Section {
	return TextSection(SectionStatic, id, "", content, true)
}

func dynamicUserSection(id, content string) Section {
	return TextSection(SectionCurrentUser, id, "", content, false)
}

func systemText(t *testing.T, assembly Assembly) string {
	t.Helper()
	for _, message := range assembly.Messages {
		if strings.EqualFold(message.Role, "system") {
			return message.Content
		}
	}
	return ""
}

func prefixBreaks(report AssemblyReport) []BreakEvent {
	out := []BreakEvent{}
	for _, event := range report.Breaks {
		if event.Reason.PrefixBreaking() {
			out = append(out, event)
		}
	}
	return out
}

// TestPrefixServiceP1AppendOnlyAcrossControlledTurns（T-A1）：受控 N 轮，
// 稳定层不变、动态区逐轮变（模拟 CreatedAt 换代），断言 P1 字节前缀保持
// 与 P3 system 恒等，且无前缀断裂类 breaks。
func TestPrefixServiceP1AppendOnlyAcrossControlledTurns(t *testing.T) {
	service := NewPrefixService()
	const layers = "You are the assistant.\nRules stay fixed."
	var previousSystem string
	var previousFingerprint string
	for turn := 1; turn <= 5; turn++ {
		assembly, report, err := service.Assemble(context.Background(), PrefixRequest{
			AssemblyInput: AssemblyInput{
				SystemSections: []Section{stableSection("rules", layers)},
				History:        llmMessageHistory(turn),
				UserSections:   []Section{dynamicUserSection("chat_context_snapshot", fmt.Sprintf(`{"turn":%d,"created_at":"2026-10-07T09:00:0%d.123456789Z"}`, turn, turn))},
			},
			SessionKey: "controlled",
		})
		if err != nil {
			t.Fatalf("turn %d assemble: %v", turn, err)
		}
		current := systemText(t, assembly)
		if current == "" {
			t.Fatalf("turn %d: no system message rendered", turn)
		}
		if previousSystem != "" {
			if !strings.HasPrefix(current, previousSystem) {
				t.Fatalf("P1 violated at turn %d: system bytes are not an append-only extension", turn)
			}
			if current != previousSystem {
				t.Fatalf("P3 violated at turn %d: system bytes changed on an eventless turn", turn)
			}
			if got := prefixBreaks(report); len(got) != 0 {
				t.Fatalf("turn %d: unexpected prefix breaks %+v on an eventless turn", turn, got)
			}
			if len(report.CacheAnomalies) != 0 {
				t.Fatalf("turn %d: cache anomalies %+v on an eventless turn", turn, report.CacheAnomalies)
			}
			if report.PrefixFingerprint != previousFingerprint {
				t.Fatalf("turn %d: prefix fingerprint changed without a semantic event", turn)
			}
		}
		previousSystem = current
		previousFingerprint = report.PrefixFingerprint
		if report.PrefixBytes != len([]byte(current)) {
			t.Fatalf("turn %d: prefix bytes %d != system message bytes %d", turn, report.PrefixBytes, len([]byte(current)))
		}
		if report.DynamicBytes == 0 {
			t.Fatalf("turn %d: dynamic bytes not measured", turn)
		}
	}
}

// TestPrefixServiceLayerAppendIsLegalGrowth（T-A1 追加面）：层尾部增长
// 记 layer_appended，P1 保持（starts-with），不触发断裂类归因。
func TestPrefixServiceLayerAppendIsLegalGrowth(t *testing.T) {
	service := NewPrefixService()
	base := "Line one.\nLine two."
	firstAssembly, _, err := service.Assemble(context.Background(), PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection("ledger", base)}},
		SessionKey:    "ledger",
	})
	if err != nil {
		t.Fatalf("first assemble: %v", err)
	}
	assembly, second, err := service.Assemble(context.Background(), PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection("ledger", base+"\nentry 41 appended")}},
		SessionKey:    "ledger",
	})
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	if !strings.HasPrefix(systemText(t, assembly), systemText(t, firstAssembly)) {
		t.Fatalf("appended layer lost the append-only property")
	}
	if len(prefixBreaks(second)) != 0 {
		t.Fatalf("layer append produced breaking events %+v", second.Breaks)
	}
	found := false
	for _, event := range second.Breaks {
		if event.Reason == BreakLayerAppended && event.LayerID == "ledger" {
			found = true
		}
	}
	if !found {
		t.Fatalf("layer append was not reported as layer_appended: %+v", second.Breaks)
	}
}

// TestPrefixServiceBreakAttributionClosedEnumeration（P2 完备面）：稳定层
// 内容中段变化必须产生封闭枚举内的归因；fingerprint 变而 Breaks 与
// CacheAnomalies 均空 = 隐式断裂（唯一红线）。
func TestPrefixServiceBreakAttributionClosedEnumeration(t *testing.T) {
	cases := []struct {
		layerID string
		before  string
		after   string
		reason  BreakReason
	}{
		{"rules", "v1 rules", "v2 rules rewritten", BreakRulesetChanged},
		{"ctx.layer.profile", "core set A", "core set B", BreakProfileUpdated},
		{"ctx.layer.env", "instance A", "instance B", BreakEnvChanged},
	}
	for _, tc := range cases {
		service := NewPrefixService()
		if _, _, err := service.Assemble(context.Background(), PrefixRequest{
			AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection(tc.layerID, tc.before)}},
			SessionKey:    tc.layerID,
		}); err != nil {
			t.Fatalf("first assemble %s: %v", tc.layerID, err)
		}
		_, report, err := service.Assemble(context.Background(), PrefixRequest{
			AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection(tc.layerID, tc.after)}},
			SessionKey:    tc.layerID,
		})
		if err != nil {
			t.Fatalf("second assemble %s: %v", tc.layerID, err)
		}
		if len(prefixBreaks(report)) != 1 {
			t.Fatalf("%s: want exactly one prefix break, got %+v", tc.layerID, report.Breaks)
		}
		if got := prefixBreaks(report)[0].Reason; got != tc.reason {
			t.Fatalf("%s: attribution reason = %s, want %s", tc.layerID, got, tc.reason)
		}
		if !validReason(prefixBreaks(report)[0].Reason) {
			t.Fatalf("%s: reason outside closed enumeration", tc.layerID)
		}
	}
}

func validReason(reason BreakReason) bool {
	for _, candidate := range AllBreakReasons {
		if candidate == reason {
			return true
		}
	}
	return false
}

// TestPrefixServiceDualTrackNondeterminismDetection（T-A4）：显式 CacheKey
// 固定而渲染输入微变（缺陷注入：渲染非确定性），服务必须检出——
// CacheAnomalies 非空且 Breaks 带归因（P2 不留隐式断裂）。
func TestPrefixServiceDualTrackNondeterminismDetection(t *testing.T) {
	service := NewPrefixService()
	build := func(content string) PrefixRequest {
		section := stableSection("rules", content)
		section.CacheKey = "ruleset.v3" // 显式声明渲染输入身份：缺陷注入点
		return PrefixRequest{
			AssemblyInput: AssemblyInput{SystemSections: []Section{section}},
			SessionKey:    "dual-track",
		}
	}
	if _, _, err := service.Assemble(context.Background(), build("stable rules text")); err != nil {
		t.Fatalf("first assemble: %v", err)
	}
	_, report, err := service.Assemble(context.Background(), build("stable rules text with one  subtle spacing change")) // 微变：渲染非确定性注入
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	if len(report.CacheAnomalies) == 0 {
		t.Fatalf("T-A4: renderer nondeterminism was not detected: %+v", report)
	}
	if len(prefixBreaks(report)) == 0 {
		t.Fatalf("T-A4: implicit break with empty prefix attribution: %+v", report)
	}
	if report.Layers[0].CacheKey != "ruleset.v3" {
		t.Fatalf("explicit cache key was not honored: %s", report.Layers[0].CacheKey)
	}
	if report.Layers[0].ContentHash == "" {
		t.Fatalf("content hash (criterion track) missing")
	}
}

// TestPrefixServiceAutoCacheKeyTracksContent：自动推导 CacheKey 下双轨
// 退化为单轨仍诚实——内容变 CacheKey 必变（无 anomaly 误报）。
func TestPrefixServiceAutoCacheKeyTracksContent(t *testing.T) {
	service := NewPrefixService()
	_, first, _ := service.Assemble(context.Background(), PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection("rules", "v1")}},
		SessionKey:    "auto",
	})
	_, second, _ := service.Assemble(context.Background(), PrefixRequest{
		AssemblyInput: AssemblyInput{SystemSections: []Section{stableSection("rules", "v1 rewritten")}},
		SessionKey:    "auto",
	})
	if second.Layers[0].CacheKey == first.Layers[0].CacheKey {
		t.Fatalf("auto cache key must track content changes")
	}
	if len(second.CacheAnomalies) != 0 {
		t.Fatalf("auto-derived keys must not fabricate anomalies: %+v", second.CacheAnomalies)
	}
}

// TestPrefixServiceDynamicTurnReporting：动态区事件（history 滑动、user
// 内容换代）记 history_window_slid/snapshot_rotated，均为仅报告类
// （PrefixBreaking=false），不影响前缀指纹。
func TestPrefixServiceDynamicTurnReporting(t *testing.T) {
	service := NewPrefixService()
	req := func(historyLen int, userContent string) PrefixRequest {
		return PrefixRequest{
			AssemblyInput: AssemblyInput{
				SystemSections: []Section{stableSection("rules", "fixed")},
				History:        llmMessageHistory(historyLen),
				UserSections:   []Section{dynamicUserSection("chat_context_snapshot", userContent)},
			},
			SessionKey: "dynamic",
		}
	}
	if _, _, err := service.Assemble(context.Background(), req(1, `{"t":1}`)); err != nil {
		t.Fatalf("first assemble: %v", err)
	}
	_, report, err := service.Assemble(context.Background(), req(3, `{"t":2}`))
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	reasons := map[BreakReason]int{}
	for _, event := range report.Breaks {
		if event.Reason.PrefixBreaking() {
			t.Fatalf("dynamic event reported as prefix-breaking: %+v", event)
		}
		reasons[event.Reason]++
	}
	if reasons[BreakHistoryWindowSlid] != 1 || reasons[BreakSnapshotRotated] != 1 {
		t.Fatalf("dynamic reporting incomplete: %+v", report.Breaks)
	}
}

// TestAssemblyReportPromptStatsExtras（T-A5 服务面）：三字段进遥测口径。
func TestAssemblyReportPromptStatsExtras(t *testing.T) {
	report := AssemblyReport{
		PrefixBytes:  100,
		DynamicBytes: 40,
		Breaks:       []BreakEvent{{LayerID: "ledger", Reason: BreakLayerAppended}},
	}
	extras := report.PromptStatsExtras()
	if extras["prefix_bytes"] != 100 || extras["dynamic_bytes"] != 40 {
		t.Fatalf("byte fields missing from extras: %+v", extras)
	}
	breaks, ok := extras["breaks"].([]string)
	if !ok || len(breaks) != 1 || breaks[0] != "layer_appended:ledger" {
		t.Fatalf("breaks telemetry shape wrong: %+v", extras["breaks"])
	}
}

func llmMessageHistory(turns int) []llm.Message {
	out := make([]llm.Message, 0, turns*2)
	for i := 0; i < turns; i++ {
		out = append(out,
			llm.Message{Role: "user", Content: fmt.Sprintf("user turn %d", i)},
			llm.Message{Role: "assistant", Content: fmt.Sprintf("assistant turn %d", i)},
		)
	}
	return out
}
