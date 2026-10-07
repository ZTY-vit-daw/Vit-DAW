package carriers

// discipline_clause_tb_test.go — T-B7（§6.2 退场一致性组）：纪律条款
// prompt 面存在性（§4.4-3，L1-4-IMPL-C 交付面）。
//
// 断言：条款段（ruleset 段落 ID 定位）在 L1 稳定段的装配产物中存在，且随
// ruleset 版本化（L1 Section CacheKey=RulesetVersion；新增段=版本 bump）。
// 模型遵循度属运行行为，不进机械断言。

import (
	"context"
	"strings"
	"testing"

	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/rules/ruleset"
)

// clauseAnchor 是条款文本的可定位锚短语（重拉确认 state、不凭记忆断言）。
const clauseAnchor = "Before citing a historical observation that is no longer in the current context window"

func TestTB7DisciplineClauseInStableLayer(t *testing.T) {
	result := ruleset.Load()
	if result.Err != nil {
		t.Fatalf("ruleset load: %v", result.Err)
	}
	// 段落 ID 定位：条款段在 manifest 内、跨族（chat 与 neutral_family）、
	// purpose=discipline（内容盲审查键）。
	var clause *ruleset.RuleSection
	for i := range result.Manifest.Sections {
		if result.Manifest.Sections[i].SectionID == ruleset.SectionEvidenceRefDiscipline {
			clause = &result.Manifest.Sections[i]
			break
		}
	}
	if clause == nil {
		t.Fatalf("T-B7 red: discipline clause section %s missing from manifest", ruleset.SectionEvidenceRefDiscipline)
	}
	if clause.Purpose != ruleset.PurposeDiscipline {
		t.Fatalf("clause purpose = %s, want discipline", clause.Purpose)
	}
	if !strings.Contains(clause.Content, clauseAnchor) || !strings.Contains(clause.Content, "re-pull") {
		t.Fatalf("T-B7 red: clause text must carry the re-pull-before-cite discipline")
	}

	// chat 族与中性族渲染均含条款（AppliesTo 归并——两入口都会引用历史观察）。
	chatRules, _ := result.Manifest.RenderChatLayers("mode-x", "catalog-x")
	neutralRules, _ := result.Manifest.RenderNeutralFamilyLayers("obs-cat", "tool-a,tool-b")
	if !strings.Contains(chatRules, clauseAnchor) {
		t.Fatalf("T-B7 red: chat family render missing the clause")
	}
	if !strings.Contains(neutralRules, clauseAnchor) {
		t.Fatalf("T-B7 red: neutral_family render missing the clause")
	}

	// 装配产物面：四层装配的 L1 稳定段含条款且随 ruleset 版本化。
	bundle := Assemble(context.Background(), Options{
		WorkspaceDir:    t.TempDir(),
		ProjectDir:      t.TempDir(),
		Family:          ruleset.FamilyChat,
		ModeInstruction: "mode-x",
		CommandCatalog:  "catalog-x",
	})
	var rulesSection *promptruntime.Section
	for i := range bundle.Sections {
		if bundle.Sections[i].ID == LayerRules {
			rulesSection = &bundle.Sections[i]
			break
		}
	}
	if rulesSection == nil {
		t.Fatalf("T-B7 red: L1 rules layer missing from bundle: %+v", bundle.LayerStates)
	}
	if !strings.Contains(rulesSection.Content, clauseAnchor) {
		t.Fatalf("T-B7 red: clause absent from the assembled stable L1 layer")
	}
	if !rulesSection.Stable {
		t.Fatalf("clause must live in a stable section (prompt 面存在性)")
	}
	if rulesSection.CacheKey != result.Manifest.RulesetVersion {
		t.Fatalf("T-B7 red: L1 CacheKey=%q must equal RulesetVersion=%q (随 ruleset 版本化)",
			rulesSection.CacheKey, result.Manifest.RulesetVersion)
	}

	// 经 PrefixService 的稳定 system 消息面（模型实际所见 prompt 面）。
	_, report, err := promptruntime.NewPrefixService().Assemble(context.Background(), promptruntime.PrefixRequest{
		AssemblyInput: promptruntime.AssemblyInput{SystemSections: bundle.Sections},
		SessionKey:    "tb7-clause",
	})
	if err != nil {
		t.Fatalf("prefix assemble: %v", err)
	}
	if report.PrefixFingerprint == "" {
		t.Fatalf("stable prefix must fingerprint")
	}
	stableText := ""
	for _, message := range promptruntime.Build(promptruntime.AssemblyInput{SystemSections: bundle.Sections}).Messages {
		if strings.EqualFold(message.Role, "system") {
			stableText = message.Content
		}
	}
	if !strings.Contains(stableText, clauseAnchor) {
		t.Fatalf("T-B7 red: clause absent from the stable system message (model-visible prompt face)")
	}
}
