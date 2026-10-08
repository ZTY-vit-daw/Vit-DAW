package chat

// L1-4-IMPL-D 权威翻转后的 parity（chat 面）：生产 system 直接经 ruleset
// embed 渲染（chatSystemFromRuleset），legacy 生产常量降为 fail-open 回落。
// 两个证明面：
//  1. TestChatSystemMatchesRulesetEmbedByteForByte——生产面 == embed 全量渲染
//     （含 IMPL-C 新增段）。回退到 legacy 常量即红（legacy 无第八段）——
//     「生产面确实从 embed 装配」的 canary。
//  2. TestChatSystemFallbackMatchesEmbedMinusPostMigration——fail-open 回落
//     模板 == embed 渲染减 PostMigrationSectionIDs（IMPL-B 迁移段零改写的
//     持续锁定；回落路径永不静默漂移）。

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/rules/ruleset"
)

func firstDivergence(a, b string) (int, string, string) {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for index := 0; index < limit; index++ {
		if a[index] != b[index] {
			return index, snippet(a, index), snippet(b, index)
		}
	}
	if len(a) != len(b) {
		return limit, snippet(a, limit), snippet(b, limit)
	}
	return -1, "", ""
}

func snippet(text string, index int) string {
	start := index - 40
	if start < 0 {
		start = 0
	}
	end := index + 40
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}

// TestChatSystemMatchesRulesetEmbedByteForByte：同 mode/catalog 输入下，
// 生产 system 消息 == embed 全量渲染（rules + "\n\n" + catalog，含
// shared.discipline.evidence_refs）。
func TestChatSystemMatchesRulesetEmbedByteForByte(t *testing.T) {
	server := newChatServerForTest(t, nil, nil, nil)
	assembly := server.buildAssembly(context.Background(), "conv-ruleset-parity", "对照轮。", map[string]any{})
	system := chatAssemblySystemText(t, assembly.Messages)
	if system == "" {
		t.Fatalf("chat assembly rendered no system message")
	}

	mode := agentModeSystemInstruction(agentModeFromContext(map[string]any{}))
	catalog := server.harness.ModelCatalogSummary()
	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	rules, catalogLayer := result.Manifest.RenderChatLayers(mode, catalog)
	want := strings.TrimSpace(rules + "\n\n" + catalogLayer)

	if system != want {
		at, produced, rendered := firstDivergence(system, want)
		t.Fatalf("chat system diverges from ruleset embed at byte %d (len %d vs %d)\nproduced: %q\nembed:    %q",
			at, len(system), len(want), produced, rendered)
	}
	// 生产面必须携带 IMPL-C 纪律条款段（翻转的语义增量，非字节噪音）——
	// 锚句=段首行（T-B7 同款锚定方式）。
	if !strings.Contains(system, "Evidence refs and re-pull discipline:") {
		t.Fatalf("flipped chat system lacks discipline section anchor sentence")
	}
}

// TestChatSystemFallbackMatchesEmbedMinusPostMigration：fail-open 回落模板
// （legacyChatSystemTemplate）与 embed 渲染减 PostMigrationSectionIDs 逐字节
// 一致——回落路径保 IMPL-B「迁移段零改写」承诺。
func TestChatSystemFallbackMatchesEmbedMinusPostMigration(t *testing.T) {
	mode := agentModeSystemInstruction(agentModeFromContext(map[string]any{}))
	catalog := "catalog-fixture"
	fallback := fmt.Sprintf(legacyChatSystemTemplate, mode, catalog)

	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	_, catalogLayer := result.Manifest.RenderChatLayers(mode, catalog)
	rules := result.Manifest.RenderFamilyJoined(ruleset.FamilyChat,
		append([]string{ruleset.ChatCatalogWrapperSectionID}, ruleset.PostMigrationSectionIDs()...)...)
	want := strings.TrimSpace(rules + "\n\n" + catalogLayer)

	if strings.TrimSpace(fallback) != want {
		at, legacy, migrated := firstDivergence(strings.TrimSpace(fallback), want)
		t.Fatalf("fallback template diverges from embed (minus post-migration) at byte %d (len %d vs %d)\nlegacy: %q\nembed:  %q",
			at, len(fallback), len(want), legacy, migrated)
	}
}
