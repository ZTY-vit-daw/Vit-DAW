package ruleset

import (
	"strings"
	"testing"
	"testing/fstest"
)

func testFS(manifest string) fstest.MapFS {
	return fstest.MapFS{
		"manifest.json":                 &fstest.MapFile{Data: []byte(manifest)},
		"sections/a.txt":                &fstest.MapFile{Data: []byte("segment A")},
		"sections/b.txt":                &fstest.MapFile{Data: []byte("segment B")},
		"sections/shared.txt":           &fstest.MapFile{Data: []byte("shared discipline")},
		"sections/crlf.txt":             &fstest.MapFile{Data: []byte("line one\r\nline two\r\n")},
		"sections/chat.catalog.wrapper": &fstest.MapFile{Data: []byte("Mode instruction:\n%s\n\nAvailable DAW command catalog:\n%s")},
	}
}

const validManifest = `{
  "schema_version": "ruleset.v1",
  "ruleset_version": "ruleset.test",
  "sections": [
    {"section_id": "chat.rules", "version": 1, "purpose": "discipline", "file": "sections/a.txt", "applies_to": ["chat"]},
    {"section_id": "neutral.rules", "version": 1, "purpose": "discipline", "file": "sections/b.txt", "applies_to": ["neutral_family"]},
    {"section_id": "shared.discipline", "version": 2, "purpose": "discipline", "file": "sections/shared.txt", "applies_to": ["chat", "neutral_family"]},
    {"section_id": "chat.catalog.wrapper", "version": 1, "purpose": "catalog", "file": "sections/chat.catalog.wrapper", "applies_to": ["chat"]},
    {"section_id": "crlf.section", "version": 1, "purpose": "output_format", "file": "sections/crlf.txt", "applies_to": ["chat"]}
  ]
}`

// TestEmbeddedManifestLoadsClean：嵌入 manifest 零拒载、八段齐全（IMPL-B
// 迁移七段 + IMPL-C 纪律条款段）、段 ID 与 Purpose 布局符合迁移对照表。
func TestEmbeddedManifestLoadsClean(t *testing.T) {
	result := Load()
	if result.Err != nil {
		t.Fatalf("embedded manifest load error: %v", result.Err)
	}
	if len(result.Refused) != 0 {
		t.Fatalf("embedded manifest refused sections (must be clean): %v", result.Refused)
	}
	want := map[string]Purpose{
		"chat.output_format":                 PurposeOutputFormat,
		"chat.discipline.commands":           PurposeDiscipline,
		"chat.discipline.plugins_mixing":     PurposeDiscipline,
		ChatCatalogWrapperSectionID:          PurposeCatalog,
		"neutral_family.output_format":       PurposeOutputFormat,
		"neutral_family.discipline.rules":    PurposeDiscipline,
		NeutralFamilyCatalogWrapperSectionID: PurposeCatalog,
		// L1-4-IMPL-C 新增段（§4.4-3 纪律条款，跨族）——资源新增非改写，
		// 既有七段字节不动。
		SectionEvidenceRefDiscipline: PurposeDiscipline,
		// EDU-ASK-ANCHOR-1（282）新增段（裁定 15 方案 a：问询锚定纪律，
		// 跨族）——资源新增非改写，既有段字节不动。
		SectionAskAnchorDiscipline: PurposeDiscipline,
	}
	if len(result.Manifest.Sections) != len(want) {
		t.Fatalf("embedded manifest section count = %d, want %d", len(result.Manifest.Sections), len(want))
	}
	for _, section := range result.Manifest.Sections {
		if purpose, ok := want[section.SectionID]; !ok || section.Purpose != purpose {
			t.Fatalf("section %s purpose = %s, want %v", section.SectionID, section.Purpose, purpose)
		}
	}
	if result.Manifest.RulesetVersion == "" {
		t.Fatalf("empty ruleset version")
	}
	if result.Manifest.RulesetVersion == "ruleset.v1" {
		t.Fatalf("new section added but RulesetVersion not bumped (still ruleset.v1)")
	}
}

// TestPurposeClosedEnumFailClosed（§8.1 内容盲审查门 + §8.3 fail-closed）：
// Purpose 封闭枚举之外的段拒载+清单留痕；其余合法段不受阻。
func TestPurposeClosedEnumFailClosed(t *testing.T) {
	fsys := testFS(strings.Replace(validManifest,
		`"purpose": "discipline", "file": "sections/a.txt"`,
		`"purpose": "mixing_advice", "file": "sections/a.txt"`, 1))
	result := LoadFrom(fsys)
	if result.Err != nil {
		t.Fatalf("manifest-level error must not fire for section-level refusal: %v", result.Err)
	}
	if len(result.Refused) != 1 || !strings.Contains(result.Refused[0], "chat.rules") ||
		!strings.Contains(result.Refused[0], "content-blind") {
		t.Fatalf("expected one refused section with content-blind note, got %v", result.Refused)
	}
	if strings.Contains(result.Manifest.RenderFamily(FamilyChat), "segment A") {
		t.Fatalf("refused section content leaked into family render")
	}
}

// TestAppliesToClosedEnumAndMerge：未知入口族拒载；跨族段在两族各渲染一次
// （重叠纪律段归并机制，§2.1 要点 1）。
func TestAppliesToClosedEnumAndMerge(t *testing.T) {
	fsys := testFS(strings.Replace(validManifest,
		`"applies_to": ["chat", "neutral_family"]`,
		`"applies_to": ["chat", "wedding_dj"]`, 1))
	result := LoadFrom(fsys)
	if len(result.Refused) != 1 || !strings.Contains(result.Refused[0], "shared.discipline") {
		t.Fatalf("expected shared.discipline refused for unknown family, got %v", result.Refused)
	}

	clean := LoadFrom(testFS(validManifest))
	if !strings.Contains(clean.Manifest.RenderFamily(FamilyChat), "shared discipline") ||
		!strings.Contains(clean.Manifest.RenderFamily(FamilyNeutralFamily), "shared discipline") {
		t.Fatalf("AppliesTo merge failed: shared section missing from a family render")
	}
}

// TestDuplicateSectionIDRefused：重复段 ID 后者拒载。
func TestDuplicateSectionIDRefused(t *testing.T) {
	duplicated := strings.Replace(validManifest, `"neutral.rules"`, `"chat.rules"`, 1)
	result := LoadFrom(testFS(duplicated))
	if len(result.Refused) == 0 || !strings.Contains(result.Refused[0], "duplicate") {
		t.Fatalf("expected duplicate refusal, got %v", result.Refused)
	}
}

// TestCRLFNormalization：段内容装载时 CRLF 归一为 LF（与 Go 原始字符串
// 字面量丢弃 CR 的语义一致，保证跨平台 content_hash 稳定）。
func TestCRLFNormalization(t *testing.T) {
	result := LoadFrom(testFS(validManifest))
	for _, section := range result.Manifest.Sections {
		if strings.Contains(section.Content, "\r") {
			t.Fatalf("section %s retains CR after normalization", section.SectionID)
		}
	}
	found := false
	for _, section := range result.Manifest.Sections {
		if section.SectionID == "crlf.section" {
			found = true
			if section.Content != "line one\nline two\n" {
				t.Fatalf("CRLF normalization produced %q", section.Content)
			}
		}
	}
	if !found {
		t.Fatalf("crlf.section missing from manifest")
	}
}

// TestRenderChatLayersFillsWrapperSlots：chat 渲染产出 L1 规则文本与
// 填充后的目录尾段，两者拼接复原既有 system 帧。
func TestRenderChatLayersFillsWrapperSlots(t *testing.T) {
	result := LoadFrom(testFS(validManifest))
	rules, catalog := result.Manifest.RenderChatLayers("MODE-TEXT", "CATALOG-TEXT")
	if strings.Contains(rules, "%s") || strings.Contains(rules, "Mode instruction:") {
		t.Fatalf("wrapper slots leaked into rules render: %q", rules)
	}
	if !strings.Contains(catalog, "Mode instruction:\nMODE-TEXT") ||
		!strings.Contains(catalog, "Available DAW command catalog:\nCATALOG-TEXT") {
		t.Fatalf("catalog wrapper slots not filled: %q", catalog)
	}
}

// TestEmbeddedWrapperIsFamilyTail：嵌入 manifest 中两个 wrapper 段都是
// 各自入口族的最后一段——L1+目录拆段与既有单 system 帧字节布局保真的
// 结构前提（chat/agentloop 对照测试锁字节，此处锁序）。
func TestEmbeddedWrapperIsFamilyTail(t *testing.T) {
	result := Load()
	if result.Err != nil {
		t.Fatalf("embedded manifest load error: %v", result.Err)
	}
	for family, wrapperID := range map[string]string{
		FamilyChat:          ChatCatalogWrapperSectionID,
		FamilyNeutralFamily: NeutralFamilyCatalogWrapperSectionID,
	} {
		sections := result.Manifest.FamilySections(family)
		if len(sections) == 0 || sections[len(sections)-1].SectionID != wrapperID {
			t.Fatalf("family %s: wrapper %s must be the tail section", family, wrapperID)
		}
	}
}

// TestEmbeddedChatRenderMatchesLegacyMarkers：嵌入资源渲染包含迁移源的
// 锚点句（逐字迁移抽检；字节级锁定在 chat/agentloop 包对照测试）。
func TestEmbeddedChatRenderMatchesLegacyMarkers(t *testing.T) {
	manifest := Load().Manifest
	chatRules, _ := manifest.RenderChatLayers("mode", "catalog")
	for _, marker := range []string{
		"You are Ask Vit, the DAW assistant inside Vit-DAW.",
		"Return ONLY JSON with this shape:",
		"Do not invent track_id or clip_id.",
		"For B2 whole-project static balance",
	} {
		if !strings.Contains(chatRules, marker) {
			t.Fatalf("chat rules render missing marker %q", marker)
		}
	}
	neutralRules, _ := manifest.RenderNeutralFamilyLayers("obs", "tools")
	for _, marker := range []string{
		"You are the neutral observation-and-family decision phase of Ask Vit's DAW Agent.",
		"Rules:",
		"User-facing wording discipline",
	} {
		if !strings.Contains(neutralRules, marker) {
			t.Fatalf("neutral rules render missing marker %q", marker)
		}
	}
}

// TestAskAnchorsSectionRendersInBothFamilies：EDU-ASK-ANCHOR-1（282，裁定 15
// 方案 a）问询锚定纪律段——跨族渲染（chat 与 neutral_family 各一次）且三件
// 文本在场：可锚定参照物清单 / 问询守则（含用户侧坐标追问与刹车句）/ A/B
// 兜底编排次序。既有段零改写由 chat/agentloop 的 fallback parity 测试持续
// 锁定（embed 渲染减 PostMigrationSectionIDs == legacy 模板）。
func TestAskAnchorsSectionRendersInBothFamilies(t *testing.T) {
	manifest := Load().Manifest
	section := manifest.sectionContent(SectionAskAnchorDiscipline)
	if strings.TrimSpace(section) == "" {
		t.Fatalf("section %s content empty", SectionAskAnchorDiscipline)
	}
	markers := []string{
		// 件一：可锚定参照物清单（marker/循环区/轨道与 clip/transport 秒/
		// music_context 守门/bar 坐标只引用命令返回）。
		"Anchorable references and inquiry discipline:",
		"transport.set_loop",
		"transport.clear_loop",
		"music_context",
		"tempo_setting_count",
		"bar_start/bar_end",
		// 件二：问询守则——指称落在可锚定参照物上+用户侧坐标追问+刹车句。
		"every referent in a question you ask the user must land on an anchorable reference",
		"mouse position, screen position",
		"ask the user to convert them to an anchorable reference",
		"ask where the judgment authority belongs to the user",
		// 件三：A/B 兜底编排次序（先听 A 段再听 B 段）。
		"A/B listening fallback",
		"listen to A first, then B",
	}
	for _, marker := range markers {
		if !strings.Contains(section, marker) {
			t.Fatalf("ask_anchors section missing marker %q", marker)
		}
	}
	for family := range map[string]struct{}{FamilyChat: {}, FamilyNeutralFamily: {}} {
		sections := manifest.FamilySections(family)
		found := false
		for _, s := range sections {
			if s.SectionID == SectionAskAnchorDiscipline {
				found = true
			}
		}
		if !found {
			t.Fatalf("family %s does not include %s", family, SectionAskAnchorDiscipline)
		}
	}
	// 段内容不得携带 fmt 槽位（wrapper 之外的段不做 Sprintf 填充，%s 泄漏
	// 会以字面量进入系统提示）。
	if strings.Contains(section, "%s") || strings.Contains(section, "%d") {
		t.Fatalf("ask_anchors section must not contain fmt verbs")
	}
}
