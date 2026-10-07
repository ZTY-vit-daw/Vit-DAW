package agentloop

// L1-4-IMPL-B 逐字迁移对照（中性族面）：嵌入 ruleset manifest 渲染的
// 固定骨架 + 目录尾段，与 ccb_model_prompt.go 既有
// messageLoopNeutralFamilySystemSkeleton 逐字节一致（双源对照锁定，漂移
// 当场红——"逐字迁移零改写"的机械证明；生产常量保持权威直至 IMPL-D）。

import (
	"strings"
	"testing"

	"vit-daw-agent/rules/ruleset"
)

// TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte：同 catalog/
// allowed 输入下，embed 渲染（skeleton + "\n\n" + catalog）== 生产骨架字节。
func TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	legacy := messageLoopNeutralFamilySystemSkeleton(state)
	if legacy == "" {
		t.Fatalf("skeleton rendered empty")
	}

	catalog := messageLoopNeutralFamilyCatalog(state.input.AllowedTools)
	allowed := strings.Join(messageLoopNeutralFamilyAllowedTools(state.input.AllowedTools), ", ")
	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	rules, catalogLayer := result.Manifest.RenderNeutralFamilyLayers(catalog, allowed)
	want := strings.TrimSpace(rules + "\n\n" + catalogLayer)

	if strings.TrimSpace(legacy) != want {
		at := 0
		limit := len(legacy)
		if len(want) < limit {
			limit = len(want)
		}
		for index := 0; index < limit; index++ {
			if legacy[index] != want[index] {
				at = index
				break
			}
		}
		t.Fatalf("neutral skeleton diverges from ruleset embed near byte %d (len %d vs %d)\nlegacy: %q\nembed:  %q",
			at, len(legacy), len(want), snippet(legacy, at), snippet(want, at))
	}
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
