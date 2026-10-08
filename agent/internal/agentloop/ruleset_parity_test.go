package agentloop

// L1-4-IMPL-D 权威翻转后的 parity（中性族面）：生产骨架直接经 ruleset
// embed 渲染（neutralFamilySkeletonFromRuleset），legacy 骨架模板降为
// fail-open 回落。两个证明面：
//  1. TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte——生产骨架 ==
//     embed 全量渲染（含 IMPL-C 新增段）。回退到 legacy 模板即红——
//     「生产面确实从 embed 装配」的 canary。
//  2. TestNeutralFamilyFallbackMatchesEmbedMinusPostMigration——fail-open
//     回落模板 == embed 渲染减 PostMigrationSectionIDs（IMPL-B 迁移段
//     零改写的持续锁定）。

import (
	"fmt"
	"strings"
	"testing"

	"vit-daw-agent/rules/ruleset"
)

// TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte：同 catalog/
// allowed 输入下，生产骨架 == embed 全量渲染（含 shared.discipline.
// evidence_refs）。
func TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte(t *testing.T) {
	state := neutralFamilyTestState(nil)
	state.input.AllowedTools = []string{"ccb.observation_catalog", "ccb.observation_request"}
	produced := messageLoopNeutralFamilySystemSkeleton(state)
	if produced == "" {
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

	if strings.TrimSpace(produced) != want {
		at := 0
		limit := len(produced)
		if len(want) < limit {
			limit = len(want)
		}
		for index := 0; index < limit; index++ {
			if produced[index] != want[index] {
				at = index
				break
			}
		}
		t.Fatalf("neutral skeleton diverges from ruleset embed near byte %d (len %d vs %d)\nproduced: %q\nembed:    %q",
			at, len(produced), len(want), snippet(produced, at), snippet(want, at))
	}
	// 生产面必须携带 IMPL-C 纪律条款段（翻转的语义增量）——锚句=段首行。
	if !strings.Contains(produced, "Evidence refs and re-pull discipline:") {
		t.Fatalf("flipped neutral skeleton lacks discipline section anchor sentence")
	}
}

// TestNeutralFamilyFallbackMatchesEmbedMinusPostMigration：fail-open 回落
// 模板与 embed 渲染减 PostMigrationSectionIDs 逐字节一致。
func TestNeutralFamilyFallbackMatchesEmbedMinusPostMigration(t *testing.T) {
	catalog := "observation-catalog-fixture"
	allowed := "ccb.observation_catalog, ccb.observation_request"
	fallback := fmt.Sprintf(legacyNeutralFamilySkeletonTemplate, catalog, allowed)

	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	_, catalogLayer := result.Manifest.RenderNeutralFamilyLayers(catalog, allowed)
	rules := result.Manifest.RenderFamilyJoined(ruleset.FamilyNeutralFamily,
		append([]string{ruleset.NeutralFamilyCatalogWrapperSectionID}, ruleset.PostMigrationSectionIDs()...)...)
	want := strings.TrimSpace(rules + "\n\n" + catalogLayer)

	if strings.TrimSpace(fallback) != want {
		at := 0
		limit := len(fallback)
		if len(want) < limit {
			limit = len(want)
		}
		for index := 0; index < limit; index++ {
			if fallback[index] != want[index] {
				at = index
				break
			}
		}
		t.Fatalf("fallback template diverges from embed (minus post-migration) near byte %d (len %d vs %d)\nlegacy: %q\nembed:  %q",
			at, len(fallback), len(want), snippet(fallback, at), snippet(want, at))
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
