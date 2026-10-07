package chat

// L1-4-IMPL-B 逐字迁移对照（chat 面）：嵌入 ruleset manifest 渲染的
// L1 规则段 + 目录尾段，与 server.go 既有 system 字符串逐字节一致。
// 迁移形态=双源对照锁定（生产常量保持权威直至 IMPL-D 接线翻转；本测试
// 使任何一侧的漂移当场红——"逐字迁移零改写"的机械证明）。

import (
	"context"
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
// embed 渲染（rules + "\n\n" + catalog）== 生产 system 消息字节。
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
		at, legacy, migrated := firstDivergence(system, want)
		t.Fatalf("chat system diverges from ruleset embed at byte %d (len %d vs %d)\nlegacy: %q\nembed:  %q",
			at, len(system), len(want), legacy, migrated)
	}
}
