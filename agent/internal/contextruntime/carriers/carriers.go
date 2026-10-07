// Package carriers 实现四层稳定前缀载体（CONTEXT_LAYERING_V1_DESIGN §2）：
// L1 ruleset manifest（agent/rules/ruleset 嵌入资源）、L2 user_profile.v1.json、
// L3 env_instance.v1.json、L4 ledger/project_ledger.v1.jsonl。每层渲染为
// promptruntime Section（机制复用 F1，不新建装配器），按 §2.0 层序
// rules→profile→env→ledger→目录 挂稳定 system 消息（变化频率升序，KV
// cache 前缀命中的排列前提）。
//
// 层装载失败语义（§2.0）：fail-open 但显式——缺文件=absent（旧工程包
// 常态，§8.3）、损坏=corrupt，两者都进 Bundle.LayerStates（调用方传入
// PrefixRequest.LayerStates 落 AssemblyReport），损坏必带 Warnings（WARN
// 不静默吞），都不阻塞会话。
//
// 归属申报（卡面）：载体包落位 agent/internal/contextruntime/carriers。
// 入口接线（chat/agentloop 生产路径消费四层 Section）归 L1-4-IMPL-D。
package carriers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/promptruntime"
	"vit-daw-agent/rules/ruleset"
)

// 四层+目录的 Section ID（§2.0：ctx.layer.<name>；目录归 L1 族排尾）。
const (
	LayerRules   = "ctx.layer.rules"
	LayerProfile = "ctx.layer.profile"
	LayerEnv     = "ctx.layer.env"
	LayerLedger  = "ctx.layer.ledger"
	LayerCatalog = "ctx.layer.catalog"
)

// 层状态枚举（LayerReport.State 对齐）。
const (
	StateRendered = "rendered"
	StateAbsent   = "absent"
	StateCorrupt  = "corrupt"
)

// Options 是一次四层装配的输入。WorkspaceDir 持用户级/机器级载体
// （工程包外）；ProjectDir 持工程账本（工程包内）。Family 选择 L1 渲染
// 面（chat | neutral_family）；目录槽值由调用方按入口族供给（中性族
// allowed tools 须为过滤后清单的逗号连接——过滤逻辑属入口包）。
type Options struct {
	WorkspaceDir string
	ProjectDir   string
	Family       string

	// chat 族目录槽（chat.catalog.wrapper 的两个 %s）。
	ModeInstruction string
	CommandCatalog  string

	// 中性族目录槽（neutral_family.catalog.wrapper 的两个 %s）。
	ObservationCatalog string
	AllowedTools       string

	// EnvComponents 是本机指纹分量明文（采集归接线卡）；nil=跳过指纹
	// 校验（有卡渲染现卡，无卡 absent——不臆造指纹）。
	EnvComponents     map[string]string
	PluginSemanticRev string

	// ExpectedUserID 是 L2 聚合键期望（v1=local；空=不校验）。
	ExpectedUserID string

	// Now 供 EnsureEnvCard 建卡/重建时间戳（测试注入确定性时钟）；零值
	// =time.Now。渲染路径不用时钟（T-C5 前提：同输入同字节）。
	Now time.Time
}

// Bundle 是四层装配产物：Sections 按层序供 SystemSections；
// LayerStates 供 PrefixRequest.LayerStates（absent/corrupt 报告行）；
// Warnings 是 WARN 面（损坏/失配/拒载，非静默）；EnvChanged 标记本次
// 装配内发生了 L3 指纹失配重建（env_changed 事件的载体侧证据）。
type Bundle struct {
	Sections    []promptruntime.Section
	LayerStates map[string]string
	Warnings    []string
	EnvChanged  bool
	EnvCardID   string
}

// Assemble 装配四层+目录 Section（确定性：不注入任何时钟/随机；层内
// 排序固定）。任何层的缺失/损坏都不返回 error——fail-open 显式（§2.0），
// 形态进 LayerStates/Warnings。
func Assemble(ctx context.Context, opts Options) Bundle {
	_ = ctx // 预留：接线卡的取消面
	bundle := Bundle{Sections: []promptruntime.Section{}, LayerStates: map[string]string{}, Warnings: []string{}}

	bundle.assembleRules(opts)
	bundle.assembleProfile(opts)
	bundle.assembleEnv(opts)
	bundle.assembleLedger(opts)
	bundle.assembleCatalog(opts)
	return bundle
}

// assembleRules 挂 L1（SectionStatic；CacheKey=ruleset 版本——§3.2 归因
// 轨：规则变更必须 bump 版本，内容变而版本不变=CacheAnomaly T-A4 信号）。
func (b *Bundle) assembleRules(opts Options) {
	result := ruleset.Load()
	if result.Err != nil || result.Manifest == nil {
		b.LayerStates[LayerRules] = StateCorrupt
		b.Warnings = append(b.Warnings, fmt.Sprintf("ruleset manifest failed to load: %v (L1 layer corrupt)", result.Err))
		return
	}
	if len(result.Refused) > 0 {
		b.Warnings = append(b.Warnings, result.Refused...)
	}
	var rules string
	switch opts.Family {
	case ruleset.FamilyChat:
		rules, _ = result.Manifest.RenderChatLayers(opts.ModeInstruction, opts.CommandCatalog)
	case ruleset.FamilyNeutralFamily:
		rules, _ = result.Manifest.RenderNeutralFamilyLayers(opts.ObservationCatalog, opts.AllowedTools)
	default:
		b.LayerStates[LayerRules] = StateCorrupt
		b.Warnings = append(b.Warnings, fmt.Sprintf("ruleset family %q outside closed enum {chat, neutral_family}: L1 layer corrupt (fail-closed)", opts.Family))
		return
	}
	if strings.TrimSpace(rules) == "" {
		b.LayerStates[LayerRules] = StateAbsent
		return
	}
	b.Sections = append(b.Sections, promptruntime.Section{
		ID:       LayerRules,
		Kind:     promptruntime.SectionStatic,
		Content:  rules,
		CacheKey: result.Manifest.RulesetVersion,
		Stable:   true,
	})
}

// assembleProfile 挂 L2（SectionSession；活跃集渲染，CacheKey=活跃集摘要）。
func (b *Bundle) assembleProfile(opts Options) {
	state := LoadProfile(opts.WorkspaceDir, opts.ExpectedUserID)
	b.Warnings = append(b.Warnings, state.warnings...)
	switch state.state {
	case StateAbsent:
		b.LayerStates[LayerProfile] = StateAbsent
		return
	case StateCorrupt:
		b.LayerStates[LayerProfile] = StateCorrupt
		if state.err != nil {
			b.Warnings = append(b.Warnings, fmt.Sprintf("profile layer corrupt: %v", state.err))
		}
		return
	}
	content := RenderProfile(state.doc)
	if strings.TrimSpace(content) == "" {
		b.LayerStates[LayerProfile] = StateAbsent // 档在但活跃集为空：无前缀面
		return
	}
	b.Sections = append(b.Sections, promptruntime.Section{
		ID:       LayerProfile,
		Kind:     promptruntime.SectionSession,
		Content:  content,
		CacheKey: ProfileCacheKey(state.doc),
		Stable:   true,
	})
}

// assembleEnv 挂 L3（SectionSession；会话内恒定）。EnvComponents 供给时
// 先校验/构建指纹（失配=降权重建+WARN，T-C2）；nil=只渲染现卡。
func (b *Bundle) assembleEnv(opts Options) {
	if opts.EnvComponents != nil {
		card, changed, warnings, err := EnsureEnvCard(opts.WorkspaceDir, opts.EnvComponents, opts.PluginSemanticRev, opts.Now)
		if err != nil {
			b.LayerStates[LayerEnv] = StateCorrupt
			b.Warnings = append(b.Warnings, append(warnings, fmt.Sprintf("env layer corrupt: %v", err))...)
			return
		}
		b.Warnings = append(b.Warnings, warnings...)
		b.EnvChanged = changed
		b.EnvCardID = card.InstanceID
		content := RenderEnvCard(&card)
		if strings.TrimSpace(content) == "" {
			b.LayerStates[LayerEnv] = StateAbsent
			return
		}
		b.Sections = append(b.Sections, promptruntime.Section{
			ID:       LayerEnv,
			Kind:     promptruntime.SectionSession,
			Content:  content,
			CacheKey: EnvCacheKey(&card),
			Stable:   true,
		})
		return
	}
	state := LoadEnvCard(opts.WorkspaceDir)
	b.Warnings = append(b.Warnings, state.warnings...)
	switch state.state {
	case StateAbsent:
		b.LayerStates[LayerEnv] = StateAbsent
		return
	case StateCorrupt:
		b.LayerStates[LayerEnv] = StateCorrupt
		if state.err != nil {
			b.Warnings = append(b.Warnings, fmt.Sprintf("env layer corrupt: %v", state.err))
		}
		return
	}
	if state.current == nil {
		b.LayerStates[LayerEnv] = StateAbsent
		return
	}
	b.EnvCardID = state.current.InstanceID
	content := RenderEnvCard(state.current)
	if strings.TrimSpace(content) == "" {
		b.LayerStates[LayerEnv] = StateAbsent
		return
	}
	b.Sections = append(b.Sections, promptruntime.Section{
		ID:       LayerEnv,
		Kind:     promptruntime.SectionSession,
		Content:  content,
		CacheKey: EnvCacheKey(state.current),
		Stable:   true,
	})
}

// assembleLedger 挂 L4（SectionSession；全条目串联，CacheKey=链尾+条数）。
func (b *Bundle) assembleLedger(opts Options) {
	entries, err := ReadLedger(opts.ProjectDir)
	if err != nil {
		b.LayerStates[LayerLedger] = StateCorrupt
		b.Warnings = append(b.Warnings, fmt.Sprintf("ledger layer corrupt: %v (WARN: tampered or unparsable ledger not silently rendered)", err))
		return
	}
	if len(entries) == 0 {
		b.LayerStates[LayerLedger] = StateAbsent
		return
	}
	content := RenderLedger(entries)
	b.Sections = append(b.Sections, promptruntime.Section{
		ID:       LayerLedger,
		Kind:     promptruntime.SectionSession,
		Content:  content,
		CacheKey: LedgerCacheKey(entries),
		Stable:   true,
	})
}

// assembleCatalog 挂目录尾段（L1 族，SectionStatic；§2.0"排其尾"）。
// 目录槽值随运行输入变（mode/catalog/allowed tools），CacheKey=内容
// digest（自动轨：身份恒随内容，无 CacheAnomaly 面）。
func (b *Bundle) assembleCatalog(opts Options) {
	result := ruleset.Load()
	if result.Err != nil || result.Manifest == nil {
		return // rules 已标 corrupt；目录段随之缺席
	}
	var catalog string
	switch opts.Family {
	case ruleset.FamilyChat:
		_, catalog = result.Manifest.RenderChatLayers(opts.ModeInstruction, opts.CommandCatalog)
	case ruleset.FamilyNeutralFamily:
		_, catalog = result.Manifest.RenderNeutralFamilyLayers(opts.ObservationCatalog, opts.AllowedTools)
	default:
		return
	}
	if strings.TrimSpace(catalog) == "" {
		return
	}
	sum := sha256.Sum256([]byte(catalog))
	b.Sections = append(b.Sections, promptruntime.Section{
		ID:       LayerCatalog,
		Kind:     promptruntime.SectionStatic,
		Content:  catalog,
		CacheKey: "catalog:" + hex.EncodeToString(sum[:]),
		Stable:   true,
	})
}

// DeclaredLayerIDs 按声明序返回 LayerStates 的键（确定性报告行序）。
func (b *Bundle) DeclaredLayerIDs() []string {
	ids := make([]string, 0, len(b.LayerStates))
	for id := range b.LayerStates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
