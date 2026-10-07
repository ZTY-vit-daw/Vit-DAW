// Package ruleset holds the versioned agent rule manifest——L1 载体
// （CONTEXT_LAYERING_V1_DESIGN §2.1）。规则文本从 Go 字符串常量迁入
// go:embed 资源文件：迁移是逐字迁移（零改写），chat system 拆段与中性族
// 固定骨架（L1-4-IMPL-A 产物）的字节由各入口包的对照测试锁定。
//
// 内容盲审查键（§8.1）：Purpose 封闭枚举 discipline|catalog|output_format，
// 越界段装载时拒载（fail-closed）并进 Refused 清单——三类之外的内容不得
// 入 L1。重叠纪律段经 AppliesTo 归并：同一 SectionID 可声明多个入口族，
// 渲染时各族各取所需（勘察 §5 两条入口各持一份的收拢挂点）。
//
// 版本约定：段文本变更必须递增该段 Version 并递增 RulesetVersion；
// RulesetVersion 进 L1 Section 的 CacheKey（§3.2 归因轨），内容变更而
// 版本未递增会被 PrefixService 记为 CacheAnomaly（T-A4 双轨锁定）。
package ruleset

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

//go:embed resources
var resources embed.FS

// Purpose 是 RuleSection 的内容盲审查键（§8.1），封闭枚举三值。
type Purpose string

const (
	PurposeDiscipline   Purpose = "discipline"
	PurposeCatalog      Purpose = "catalog"
	PurposeOutputFormat Purpose = "output_format"
)

// Valid 报告 Purpose 是否在封闭枚举内。
func (p Purpose) Valid() bool {
	switch p {
	case PurposeDiscipline, PurposeCatalog, PurposeOutputFormat:
		return true
	default:
		return false
	}
}

// 入口族（AppliesTo 封闭枚举；planner 为 §2.1 预留，v1 manifest 未使用）。
const (
	FamilyChat          = "chat"
	FamilyNeutralFamily = "neutral_family"
	FamilyPlanner       = "planner"
)

func familyValid(family string) bool {
	switch family {
	case FamilyChat, FamilyNeutralFamily, FamilyPlanner:
		return true
	default:
		return false
	}
}

// 包装层 SectionID 常量：两段 catalog wrapper 持有运行时插值槽（chat 的
// mode/command catalog、中性族的 observation catalog/allowed tools），
// 由 RenderChatLayers/RenderNeutralFamilyLayers 以 fmt.Sprintf 填充。
const (
	ChatCatalogWrapperSectionID          = "chat.catalog.wrapper"
	NeutralFamilyCatalogWrapperSectionID = "neutral_family.catalog.wrapper"
)

type RuleSection struct {
	SectionID string   // 稳定段 ID（与 promptruntime Section.ID 语义对齐）
	Version   int      // 段级版本；文本变更递增
	Purpose   Purpose  // 内容盲审查键（封闭枚举）
	Content   string   // 规则文本（装载时 CRLF 归一为 LF，与 Go 原始字符串字面量语义一致）
	AppliesTo []string // 入口族：chat | neutral_family | planner
}

type RulesetManifest struct {
	RulesetVersion string        // 如 "ruleset.v1"；进 L1 CacheKey
	Sections       []RuleSection // manifest 声明序（族内渲染序）
}

// manifest.json 的磁盘格式。
type manifestFile struct {
	SchemaVersion  string        `json:"schema_version"`
	RulesetVersion string        `json:"ruleset_version"`
	Sections       []sectionFile `json:"sections"`
}

type sectionFile struct {
	SectionID string   `json:"section_id"`
	Version   int      `json:"version"`
	Purpose   string   `json:"purpose"`
	File      string   `json:"file"`
	AppliesTo []string `json:"applies_to"`
}

// LoadResult：装载产物。Err 非空 = manifest 级失败（L1 层 corrupt）；
// Refused = 段级拒载清单（fail-closed + WARN 面，不静默）。
type LoadResult struct {
	Manifest *RulesetManifest
	Refused  []string
	Err      error
}

var (
	loadOnce   sync.Once
	loadCached *LoadResult
)

// Load 装载嵌入 manifest（编译期资源不可变，进程内缓存一次）。
func Load() *LoadResult {
	loadOnce.Do(func() {
		sub, err := fs.Sub(resources, "resources")
		if err != nil {
			loadCached = &LoadResult{Err: fmt.Errorf("ruleset resources unavailable: %w", err)}
			return
		}
		loadCached = LoadFrom(sub)
	})
	return loadCached
}

// LoadFrom 从任意 FS 装载（测试注入用；root 内应有 manifest.json 与 sections/）。
func LoadFrom(fsys fs.FS) *LoadResult {
	raw, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return &LoadResult{Err: fmt.Errorf("ruleset manifest unreadable: %w", err)}
	}
	var disk manifestFile
	if err := json.Unmarshal(raw, &disk); err != nil {
		return &LoadResult{Err: fmt.Errorf("ruleset manifest unparseable: %w", err)}
	}
	if strings.TrimSpace(disk.RulesetVersion) == "" {
		return &LoadResult{Err: fmt.Errorf("ruleset manifest missing ruleset_version")}
	}
	result := &LoadResult{
		Manifest: &RulesetManifest{RulesetVersion: strings.TrimSpace(disk.RulesetVersion)},
		Refused:  []string{},
	}
	seen := map[string]bool{}
	for _, row := range disk.Sections {
		section, problems := loadSection(fsys, row, seen)
		if len(problems) > 0 {
			result.Refused = append(result.Refused, problems...)
			continue
		}
		seen[section.SectionID] = true
		result.Manifest.Sections = append(result.Manifest.Sections, section)
	}
	return result
}

// loadSection 装载并校验单段；任何封闭枚举越界/内容缺失 = 拒载该段
// （fail-closed，§8.1/§8.3：拒载+WARN，不静默丢弃，不阻塞其余段）。
func loadSection(fsys fs.FS, row sectionFile, seen map[string]bool) (RuleSection, []string) {
	id := strings.TrimSpace(row.SectionID)
	if id == "" {
		return RuleSection{}, []string{"ruleset section refused: empty section_id"}
	}
	if seen[id] {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: duplicate section_id %q", id)}
	}
	if row.Version < 1 {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s version %d < 1", id, row.Version)}
	}
	purpose := Purpose(strings.TrimSpace(row.Purpose))
	if !purpose.Valid() {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s purpose %q outside closed enum {discipline, catalog, output_format} (content-blind review gate, design §8.1)", id, row.Purpose)}
	}
	if len(row.AppliesTo) == 0 {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s empty applies_to", id)}
	}
	for _, family := range row.AppliesTo {
		if !familyValid(strings.TrimSpace(family)) {
			return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s applies_to entry %q outside closed enum {chat, neutral_family, planner}", id, family)}
		}
	}
	path := strings.TrimSpace(row.File)
	if path == "" {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s empty file", id)}
	}
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s content unreadable: %v", id, err)}
	}
	// CRLF 归一：Go 原始字符串字面量在编译期丢弃 CR，embed 资源保持同一
	// 语义，避免 checkout EOL 转换造成跨平台字节漂移。
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if strings.TrimSpace(content) == "" {
		return RuleSection{}, []string{fmt.Sprintf("ruleset section refused: %s empty content", id)}
	}
	return RuleSection{
		SectionID: id,
		Version:   row.Version,
		Purpose:   purpose,
		Content:   content,
		AppliesTo: row.AppliesTo,
	}, nil
}

// FamilySections 返回声明应用于 family 的段（manifest 序）。
func (m *RulesetManifest) FamilySections(family string) []RuleSection {
	out := make([]RuleSection, 0, len(m.Sections))
	for _, section := range m.Sections {
		for _, applies := range section.AppliesTo {
			if applies == family {
				out = append(out, section)
				break
			}
		}
	}
	return out
}

// RenderFamily 按声明序拼接族内段内容（段间 "\n\n"，与
// promptruntime.renderSections 的段间口径一致）。
func (m *RulesetManifest) RenderFamily(family string) string {
	sections := m.FamilySections(family)
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.Content)
	}
	return strings.Join(parts, "\n\n")
}

// RenderChatLayers 渲染 chat 族 L1 规则文本与目录尾段：wrapper 段的
// 两个 %s 槽由 modeInstruction / commandCatalog 填充。rules 与 catalog
// 以 "\n\n" 相接时，与 chat/server.go 的既有 system 字符串逐字节一致
// （对照测试锁定）。
func (m *RulesetManifest) RenderChatLayers(modeInstruction, commandCatalog string) (rules, catalog string) {
	wrapper := m.sectionContent(ChatCatalogWrapperSectionID)
	catalog = fmt.Sprintf(wrapper, modeInstruction, commandCatalog)
	rules = m.renderFamilyExcept(FamilyChat, ChatCatalogWrapperSectionID)
	return rules, catalog
}

// RenderNeutralFamilyLayers 渲染中性族 L1 骨架与目录尾段（observation
// catalog / allowed tools 两个 %s 槽）；与 agentloop
// messageLoopNeutralFamilySystemSkeleton 的固定骨架逐字节一致（对照测试锁定）。
func (m *RulesetManifest) RenderNeutralFamilyLayers(observationCatalog, allowedTools string) (rules, catalog string) {
	wrapper := m.sectionContent(NeutralFamilyCatalogWrapperSectionID)
	catalog = fmt.Sprintf(wrapper, observationCatalog, allowedTools)
	rules = m.renderFamilyExcept(FamilyNeutralFamily, NeutralFamilyCatalogWrapperSectionID)
	return rules, catalog
}

func (m *RulesetManifest) renderFamilyExcept(family, excludeID string) string {
	sections := m.FamilySections(family)
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		if section.SectionID == excludeID {
			continue
		}
		parts = append(parts, section.Content)
	}
	return strings.Join(parts, "\n\n")
}

func (m *RulesetManifest) sectionContent(id string) string {
	for _, section := range m.Sections {
		if section.SectionID == id {
			return section.Content
		}
	}
	return ""
}
