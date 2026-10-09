package promptruntime

// PrefixService 是 CONTEXT_LAYERING_V1_DESIGN §3.1 的装配入口包装：
// promptruntime.Build 签名冻结，服务在其上产出 AssemblyReport——层报告、
// 前缀指纹、断裂归因（封闭枚举六值）与双轨一致性信号（§3.2）。
// 四层载体（ruleset/profile/env/ledger）归 IMPL-B；本实现的层 = 输入中
// Stable=true 的 system section（现有 chat/agentloop 规则与目录段）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// BreakReason 是断裂原因封闭枚举（设计 §3.4；超出枚举 = 装配器 bug）。
type BreakReason string

const (
	BreakRulesetChanged    BreakReason = "ruleset_changed"
	BreakProfileUpdated    BreakReason = "profile_updated"
	BreakEnvChanged        BreakReason = "env_changed"
	BreakLayerAppended     BreakReason = "layer_appended"
	BreakHistoryWindowSlid BreakReason = "history_window_slid"
	BreakSnapshotRotated   BreakReason = "snapshot_rotated"
)

// PrefixBreaking 报告该原因是否为前缀字节断裂类（P2 判据用）。
// layer_appended 是 P1 下的合法尾部增长；history/snapshot 两类只描述
// 动态区，不入前缀指纹（§3.4"仅报告可见性"）。
func (r BreakReason) PrefixBreaking() bool {
	switch r {
	case BreakRulesetChanged, BreakProfileUpdated, BreakEnvChanged:
		return true
	default:
		return false
	}
}

// AllBreakReasons 供测试与遥测校验枚举封闭性。
var AllBreakReasons = []BreakReason{
	BreakRulesetChanged, BreakProfileUpdated, BreakEnvChanged,
	BreakLayerAppended, BreakHistoryWindowSlid, BreakSnapshotRotated,
}

type LayerReport struct {
	LayerID     string
	Version     string
	CacheKey    string
	ContentHash string // 渲染产物 sha256（判据轨，字节级诚实）
	Bytes       int
	EntryCount  int
	State       string // rendered | absent | corrupt | skipped
}

type BreakEvent struct {
	LayerID string // 动态区类断裂记 "dynamic"
	Reason  BreakReason
	Detail  string
}

type AssemblyReport struct {
	Layers            []LayerReport
	PrefixFingerprint string // 稳定段级联 sha256（层 content_hash 级联；不含 history/动态区）
	PrefixBytes       int
	DynamicBytes      int
	Breaks            []BreakEvent // 相对上一轮同 SessionKey 装配；首轮为空
	CacheAnomalies    []string     // content_hash 变而 CacheKey 未变的层（§3.2：装配器 bug 信号，T-A4）
	// PrefixContentHash 是稳定段实际渲染字节的 sha256（最终 system 消息
	// 内容，判据轨 P1 供给，G3-ATTRIB-2）。与 PrefixFingerprint 互补：前者
	// 对真实消息字节，后者对层报告级联——跨轮恒等时两者皆恒等，字节级
	// starts-with（append-only 增长）只有前者配合 PrefixStartsWithPrevious
	// 可判。空稳定段=空串哈希（确定性）。
	PrefixContentHash string
	// PrefixStartsWithPrevious 是 P1 判据的机械判定（设计 §3.4）：同
	// SessionKey 上一轮装配的稳定段字节串是否为本轮稳定段字节串的字节级
	// 前缀（starts-with；尾部增长是唯一合法增长形态）。nil=无上一轮可比
	//（首轮/无会话键），非 nil 的 false=前缀断裂面（配 Breaks 归因）。
	PrefixStartsWithPrevious *bool
	// HistoryRefs 是历史 refs 伴随索引（设计 §5.2，L1-4-IMPL-C）：装配历史时
	// 对 assistant 消息 evidence refs 的解析注记。构建器在 contextruntime
	// （TurnBoundaryHook）；本报告只承载注记结果，不改写历史文本。
	HistoryRefs []HistoryRefEntry
	// ExitViolations 是退场执行器拒绝的退场（§4.1 安全不变式不满足）+
	// retain 落盘失败（IMPL-D：结论跨会话延伸丢失必须可见）+ WARN 文本——
	// 进遥测（回执必查项）。
	ExitViolations []string
	// RetainsWritten 是本轮经 IMPL-B 写入器实际落盘的 retain 条目数
	// （L1-4-IMPL-D 生产消费切换；ProjectDir 缺席的 advisory 形态恒 0）。
	RetainsWritten int
	// CarrierWarnings 是四层载体装载面的 WARN 清单（损坏/失配/拒载，
	// L1-4-IMPL-D；§2.0 WARN 不静默吞）——进遥测。
	CarrierWarnings []string
}

// HistoryRefEntry 是伴随索引的一行（§5.2 设计态字段签名）。ParseState 与
// agentprotocol 解析三态直通；opaque 残骸留文本不进可重拉面（Handle 空）。
type HistoryRefEntry struct {
	Ref          string `json:"ref"`                 // 原文字面（含 opaque 残骸）
	ParseState   string `json:"parse_state"`         // parsed | legacy | opaque
	Handle       string `json:"handle,omitempty"`    // parsed 且可 Resolve：evidence:// 或观察票路径
	Freshness    string `json:"freshness,omitempty"` // 物化层透传（current/material_reuse/stale）；不可解析为空
	LastSeenTurn string `json:"last_seen_turn,omitempty"`
	LedgerEntry  int64  `json:"ledger_entry,omitempty"` // 相关账本条目（可回指时）
}

// PromptStatsExtras 把三字段（设计 §3.1 遥测接线）映射进既有 promptStats
// 口径：体积计量沿用 model_snapshot_bytes 同族字节口径，不编造 token 数。
// L1-4-IMPL-C 附加退场面键（exit_violations）与伴随索引覆盖率键
// （§5.2：Tax 递减度量=parsed 占比）；L1-4-IMPL-D 附加 retain 落盘键
// （exit_retains_written）——均加法式，不动既有三键。
func (r AssemblyReport) PromptStatsExtras() map[string]any {
	breaks := make([]string, 0, len(r.Breaks))
	for _, event := range r.Breaks {
		breaks = append(breaks, string(event.Reason)+":"+event.LayerID)
	}
	parsed := 0
	for _, entry := range r.HistoryRefs {
		if entry.ParseState == "parsed" {
			parsed++
		}
	}
	extras := map[string]any{
		"prefix_bytes":         r.PrefixBytes,
		"dynamic_bytes":        r.DynamicBytes,
		"breaks":               breaks,
		"exit_violations":      len(r.ExitViolations),
		"exit_retains_written": r.RetainsWritten,
		"carrier_warnings":     len(r.CarrierWarnings),
		"history_refs_total":   len(r.HistoryRefs),
		"history_refs_parsed":  parsed,
	}
	// P1 计量供给（G3-ATTRIB-2，加法式键）：前缀级指纹与实际前缀字节哈希
	// 进遥测，P1 判据（prefix_bytes 跨 turn starts-with 恒等）在 harness/
	// 场景侧机械可判；无上一轮时 starts-with 键缺席（诚实：不可比不造值）。
	extras["prefix_fingerprint"] = r.PrefixFingerprint
	extras["prefix_content_hash"] = r.PrefixContentHash
	if r.PrefixStartsWithPrevious != nil {
		extras["prefix_starts_with_previous"] = *r.PrefixStartsWithPrevious
	}
	return extras
}

type PrefixRequest struct {
	AssemblyInput
	// SessionKey 是跨轮比对键（conversation/run 标识）。空 = 单次装配，
	// 不做断裂比对（Breaks 恒空）。
	SessionKey string
	// LayerStates 声明本次未渲染出 Section 的载体层（L1-4-IMPL-B 四层
	// 载体）：LayerID -> "absent" | "corrupt"。声明行进 AssemblyReport
	// （§2.0 fail-open 但显式）；corrupt 的 WARN 由载体侧产生，报告只记
	// 状态。已渲染层不受影响；该层上一轮渲染、本轮声明缺席时，断裂比对
	// 按既有 diff 规则记 layer removed。
	LayerStates map[string]string
}

type PrefixService interface {
	// Assemble 产出与 promptruntime.Build 同构的 Assembly，外加装配报告。
	Assemble(ctx context.Context, req PrefixRequest) (Assembly, AssemblyReport, error)
}

// NewPrefixService 返回有状态默认实现：按 SessionKey 保存上一轮层快照，
// 断裂清单相对上一轮装配计算。并发安全（chat 多会话共享）。
func NewPrefixService() PrefixService {
	return &prefixService{last: map[string]prefixSnapshot{}}
}

type prefixService struct {
	mu   sync.Mutex
	last map[string]prefixSnapshot
}

type prefixSnapshot struct {
	layerOrder  []string
	layers      map[string]LayerReport
	layerBytes  map[string]string // 层渲染产物（层级 starts-with 判定）
	systemBytes string            // 最终 system 消息内容（P1 字节级判定的权威面）
	historyLen  int
	dynamicHash string
}

// Assemble 按 §3.1/§3.2 语义包装 Build：
//   - 层 = Stable=true 且内容非空的 system section；层渲染产物为该段
//     renderSections 单段字节（消息边界内的假想独立渲染，前缀指纹级联之）；
//   - CacheKey 双轨（§3.2）：显式 Section.CacheKey 优先（调用方声明的
//     渲染输入身份）；空则自动推导 "auto:"+段身份哈希——自动推导下
//     CacheKey 恒随内容变，双轨退化单轨仍诚实；显式固定 CacheKey 而
//     内容变会被记入 CacheAnomalies（T-A4 锁定）；
//   - PrefixBytes/DynamicBytes 按最终消息字节计（PrefixBytes=system 消息，
//     DynamicBytes=末条 user 消息），与 provider 实际看到的字节一致。
func (s *prefixService) Assemble(ctx context.Context, req PrefixRequest) (Assembly, AssemblyReport, error) {
	_ = ctx // 预留：IMPL-B 载体装载的取消面
	assembly := Build(req.AssemblyInput)
	report := AssemblyReport{Layers: []LayerReport{}, Breaks: []BreakEvent{}, CacheAnomalies: []string{}, HistoryRefs: []HistoryRefEntry{}, ExitViolations: []string{}}

	layerOrder := make([]string, 0, len(req.SystemSections))
	layers := make(map[string]LayerReport, len(req.SystemSections))
	layerBytes := make(map[string]string, len(req.SystemSections))
	for _, section := range req.SystemSections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			if section.Stable {
				report.Layers = append(report.Layers, LayerReport{
					LayerID: section.ID, Version: section.CacheKey, CacheKey: section.CacheKey,
					State: "skipped", EntryCount: 1,
				})
			}
			continue
		}
		if !section.Stable {
			continue // 动态段不进层（治理后不应再出现，防御保留）
		}
		rendered := renderSections([]Section{section})
		layer := LayerReport{
			LayerID:     section.ID,
			CacheKey:    section.CacheKey,
			ContentHash: contentDigest(rendered),
			Bytes:       len(rendered),
			EntryCount:  1,
			State:       "rendered",
		}
		if layer.CacheKey == "" {
			layer.CacheKey = "auto:" + contentDigest(section.ID+"\x00"+string(section.Kind)+"\x00"+section.Title+"\x00"+rendered)
		}
		layer.Version = layer.CacheKey
		report.Layers = append(report.Layers, layer)
		layerOrder = append(layerOrder, layer.LayerID)
		layers[layer.LayerID] = layer
		layerBytes[layer.LayerID] = rendered
	}
	// 载体层声明行（L1-4-IMPL-B §2.0）：absent/corrupt 的层不渲染
	// Section，以声明形态进报告（确定性：按 LayerID 排序）。L1-4-IMPL-D：
	// 装配输入随 Bundle 附带的声明（CarrierLayerStates）与请求级声明合并。
	layerStates := req.LayerStates
	if len(req.CarrierLayerStates) > 0 {
		layerStates = make(map[string]string, len(req.LayerStates)+len(req.CarrierLayerStates))
		for id, state := range req.LayerStates {
			layerStates[id] = state
		}
		for id, state := range req.CarrierLayerStates {
			if _, exists := layerStates[id]; !exists {
				layerStates[id] = state
			}
		}
	}
	declared := make([]string, 0, len(layerStates))
	for id := range layerStates {
		declared = append(declared, id)
	}
	sort.Strings(declared)
	for _, id := range declared {
		if _, rendered := layers[id]; rendered {
			continue
		}
		report.Layers = append(report.Layers, LayerReport{LayerID: id, State: layerStates[id]})
	}
	report.CarrierWarnings = append([]string(nil), req.CarrierWarnings...)

	report.PrefixFingerprint = prefixFingerprint(layerOrder, layers)
	var systemBytes string
	for _, message := range assembly.Messages {
		switch {
		case strings.EqualFold(message.Role, "system"):
			report.PrefixBytes = len([]byte(message.Content))
			systemBytes = message.Content
		}
	}
	report.PrefixContentHash = contentDigest(systemBytes)
	for index := len(assembly.Messages) - 1; index >= 0; index-- {
		if strings.EqualFold(assembly.Messages[index].Role, "user") {
			report.DynamicBytes = len([]byte(assembly.Messages[index].Content))
			break
		}
	}

	dynamicHash := contentDigest(fmt.Sprintf("%d\x00%s", len(req.History), dynamicDigest(req.UserSections)))
	if key := strings.TrimSpace(req.SessionKey); key != "" {
		s.mu.Lock()
		previous, hasPrevious := s.last[key]
		s.last[key] = prefixSnapshot{layerOrder: layerOrder, layers: layers, layerBytes: layerBytes, systemBytes: systemBytes, historyLen: len(req.History), dynamicHash: dynamicHash}
		s.mu.Unlock()
		if hasPrevious {
			layerEvents, anomalies := diffLayers(previous, layerOrder, layers, layerBytes)
			report.Breaks = append(report.Breaks, layerEvents...)
			report.Breaks = append(report.Breaks, diffDynamic(previous, len(req.History), dynamicHash)...)
			report.CacheAnomalies = append(report.CacheAnomalies, anomalies...)
			// P1 机械判定（§3.4）：上一轮稳定段字节串是否仍为本轮前缀。
			// 只比对字节，不推断语义——断裂与否交 Breaks 归因。
			startsWith := strings.HasPrefix(systemBytes, previous.systemBytes)
			report.PrefixStartsWithPrevious = &startsWith
		}
	}
	return assembly, report, nil
}

// diffLayers 产出层断裂清单与双轨违例清单：新层 = layer_appended（合法
// 尾部增长）；层内容变化时，字节级 starts-with 仍成立 = layer_appended
// （P1 判定），否则按 LayerID 映射封闭枚举（ledger→layer_appended 族、
// profile、env、其余=规则/目录族 ruleset_changed）；层消失 = 对应断裂。
// CacheKey 未变而 content_hash 变的层同时进 Breaks（保 P2 完备：指纹变
// 必有归因）与 anomalies（显式装配器 bug 信号，T-A4 断言面）。
func diffLayers(previous prefixSnapshot, order []string, layers map[string]LayerReport, layerBytes map[string]string) ([]BreakEvent, []string) {
	events := []BreakEvent{}
	anomalies := []string{}
	for _, id := range order {
		current, ok := layers[id]
		if !ok {
			continue
		}
		prev, existed := previous.layers[id]
		if !existed {
			events = append(events, BreakEvent{LayerID: id, Reason: BreakLayerAppended, Detail: "layer appended"})
			continue
		}
		if prev.ContentHash == current.ContentHash {
			continue
		}
		if prev.CacheKey == current.CacheKey {
			// 双轨一致性（§3.2）：渲染输入身份未变而字节变 = 渲染非确定性。
			events = append(events, BreakEvent{
				LayerID: id, Reason: reasonForLayer(id),
				Detail: fmt.Sprintf("cache_key unchanged but content_hash %s -> %s", prev.ContentHash, current.ContentHash),
			})
			anomalies = append(anomalies, fmt.Sprintf("layer %s: cache_key unchanged but content_hash %s -> %s", id, prev.ContentHash, current.ContentHash))
			continue
		}
		if strings.HasPrefix(layerBytes[id], previous.layerBytes[id]) {
			events = append(events, BreakEvent{LayerID: id, Reason: BreakLayerAppended, Detail: "layer grew append-only"})
			continue
		}
		events = append(events, BreakEvent{LayerID: id, Reason: reasonForLayer(id), Detail: "layer content changed"})
	}
	seen := map[string]bool{}
	for _, id := range order {
		seen[id] = true
	}
	removed := make([]string, 0)
	for _, id := range previous.layerOrder {
		if !seen[id] {
			removed = append(removed, id)
		}
	}
	sort.Strings(removed)
	for _, id := range removed {
		events = append(events, BreakEvent{LayerID: id, Reason: reasonForLayer(id), Detail: "layer removed"})
	}
	return events, anomalies
}

func diffDynamic(previous prefixSnapshot, historyLen int, dynamicHash string) []BreakEvent {
	events := []BreakEvent{}
	if previous.historyLen != historyLen {
		events = append(events, BreakEvent{LayerID: "dynamic", Reason: BreakHistoryWindowSlid,
			Detail: fmt.Sprintf("history %d -> %d", previous.historyLen, historyLen)})
	}
	if previous.dynamicHash != dynamicHash {
		events = append(events, BreakEvent{LayerID: "dynamic", Reason: BreakSnapshotRotated,
			Detail: "user-turn dynamic content changed"})
	}
	return events
}

func reasonForLayer(layerID string) BreakReason {
	id := strings.ToLower(layerID)
	switch {
	case strings.Contains(id, "ledger"):
		return BreakLayerAppended
	case strings.Contains(id, "profile"):
		return BreakProfileUpdated
	case strings.Contains(id, "env"):
		return BreakEnvChanged
	default:
		return BreakRulesetChanged
	}
}

func prefixFingerprint(order []string, layers map[string]LayerReport) string {
	if len(order) == 0 {
		return ""
	}
	parts := make([]string, 0, len(order))
	for _, id := range order {
		layer := layers[id]
		parts = append(parts, id+"\x00"+layer.ContentHash+"\x00"+fmt.Sprintf("%d", layer.Bytes))
	}
	return contentDigest(strings.Join(parts, "\x1e"))
}

func dynamicDigest(sections []Section) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, section.ID+"\x00"+renderSections([]Section{section}))
	}
	return strings.Join(parts, "\x1e")
}

func contentDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
