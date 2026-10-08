package agentprotocol

// refschema.go — D1 统一 Ref Schema L0（REFSCHEMA-L0-1）+ kind 注册补全（REFSCHEMA-L0-2）。
//
// 规格权威：coord/decisions/2026-09-27-g1-ref-schema-ruling.md「L0 文法定版要点」。
// 注册表初值权威：coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md §2（逐条锚点，不凭记忆）。
// L0-2 扩展的 kind 名权威：docs/QUERY_ENGINE_V1_DESIGN.md §3.3（L1-3 OQ-1 裁定）。
//
// 文法（ruling BNF 基线）：
//
//	ref      := "vit://" kind "/" scope [ "/" window ] "@" snapshot [ "#" hash ]
//	kind     := 前缀注册表收录的 projection_kind（未收录→宽容解析规则 #8）
//	scope    := scope_kind ":" scope_value（段内保留字符百分号转义）
//	window   := "t=all" | "t=" sampleStart ".." sampleEnd（采样点权威层）
//	snapshot := 数据版本标识（v0 允许 legacy 值直填）
//	hash     := "sha256:" 16hex | "-"（"-"=显式未 CAS 化）
//
// ruling #2/#3 对 BNF 方括号的收紧：window 与 hash 两段**禁止省略**——新引用带
// sha256 哈希，旧引用显式 `#-`；全时间窗显式 `t=all`。缺任一段即解析拒绝
// （隐式缺段=解析歧义温床），故本实现要求两段在位。
//
// 保留字符集 = 文法自身的结构分隔符：`%` `/` `@` `#` `:`。段内出现即百分号
// 转义（`%25` `%2F` `%40` `%23` `%3A`，大写十六进制）；其余字符（含空格、
// Unicode）逐字节直传。解码严格闭合：`%` 后必须跟两位十六进制，段内出现
// 未转义的保留字符按非法拒绝，保证转义往返无歧义。
//
// 解析三态（ruling #8）：
//	parsed  — 合法 vit:// 引用（kind 已收录）
//	legacy  — 命中前缀注册表的 legacy 字面量（可翻译）
//	opaque  — 未收录引用：WARN 一次 + 原样透传（宽容解析，不炸消费面）
//
// opaque 的 WARN 走 RefSchemaWarnLogger 注入点（tim.AssertWarnLogger 同款模式，
// host 侧接线 logx；nil 保持本包纯函数静默）；计数按 scheme 头聚合，供注册表
// 补全优先级排序。
//
// 本文件是纯新增工具层：不改动任何既有 EvidenceRefs 生成点或消费点（§3 消费面
// 零改动）；接线迁移按 ruling 后续排程分卡执行。

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// scope_kind 词汇登记（REF_SCHEMA_V1 §6 开放集登记义务：新增随注册表注释
// 登记）。REFSCHEMA-M2（2026-10-08）mom C 类构造器迁 vit://mom 后新增三个
// 数据键族 scope_kind：`mix.read`（track/project 数据读键）、
// `acoustic_package_status`（声学包 layer.feature 键）、`observation`（观察
// 票键；snapshot 段承载同一 observation_id——身份族语义，M1 裁定）。本条为
// 纯注释登记，零行为改动；词汇表现役集（track/project/feature/band/pair）
// 的权威表仍归 REF_SCHEMA_V1 §6，由决策侧文档卡同步。
const RefSchemePrefix = "vit://"

// refEscapeReserved is the set of grammar-structural delimiters that must be
// percent-escaped inside the scope and snapshot segments.
const refEscapeReserved = "%/@#:"

// TimeWindow is the explicit time window segment. Ruling #2 forbids omitting
// the segment: AllTime selects t=all, otherwise both sample bounds are
// emitted (sample-count authority, D3).
type TimeWindow struct {
	AllTime     bool
	SampleStart int64
	SampleEnd   int64
}

// Ref is the structured form of an L0 evidence ref.
type Ref struct {
	Kind       string
	ScopeKind  string
	ScopeValue string
	Window     *TimeWindow
	Snapshot   string
	Hash       string // "-" or "sha256:"+16 lowercase hex; empty is invalid
}

type RefState string

const (
	RefStateParsed RefState = "parsed"
	RefStateLegacy RefState = "legacy"
	RefStateOpaque RefState = "opaque"
)

// Legacy ref families (registry 类属).
const (
	RefFamilyProjectionContentID    = "projection_content_id"   // A 类：投影内容 ID
	RefFamilySnapshotRequestID      = "snapshot_request_id"     // G 类：快照 request_id 族
	RefFamilyEvidenceSchemeURI      = "evidence_scheme_uri"     // C/D 类：URI 式 scheme 头数据面引用
	RefFamilyObservationFingerprint = "observation_fingerprint" // B 类：内容指纹身份（16 hex）
)

// L0 segments a legacy value translates into.
const (
	RefSlotHash     = "hash"     // 内容身份 → #hash 段
	RefSlotSnapshot = "snapshot" // 快照身份 → @snapshot 段
	RefSlotScope    = "scope"    // 数据面地址 → scope 段（无独立身份段的引用）
)

// LegacyPrefixEntry maps one legacy format literal (L1-1 §2 anchor) to its
// family and its target slot in the L0 grammar. TargetKind is set only when
// the family maps onto one registered kind; identity-only families (G 类
// request ids, B 类 fingerprints) fill no kind.
type LegacyPrefixEntry struct {
	LegacyPrefix string
	Family       string
	TargetKind   string
	Slot         string
	Anchor       string
}

// legacyPrefixRegistry is the authoritative initial registry (ruling #4:
// agentprotocol constant table). Every entry is anchored to the L1-1
// inventory report §2; do not add entries from memory.
//
// REFSCHEMA-L0-2 extension (九投影+DAD kind 补全): entries 7-14 below add the
// acp projection prefix family, the four DAD prefix families, and the
// audioclosure/frequencycleanup fingerprint families. Kind names
// (acp / dad.l3 / dad.l2_render_probe / dad.compressor_dual_tap /
// dad.frequency_evidence) follow docs/QUERY_ENGINE_V1_DESIGN.md §3.3
// (L1-3 OQ-1 裁定); anchors follow L1-1 §2. The fci_/fcp_ literal form
// ("<prefix>_"+16 hex) is verified at frequencycleanup/model.go:201-204
// (stableID), the anchor being L1-1 §2 B7 treatment.go:86/94.
//
// REFSCHEMA-M1 extension (2026-10-07): entry 15 adds the mom observation
// ticket head "observation:" (L1-1 §2 C3). Source: G1 终审记录 §4 迁移计划
// 首项 (coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md).
var legacyPrefixRegistry = []LegacyPrefixEntry{
	{LegacyPrefix: "dom_", Family: RefFamilyProjectionContentID, TargetKind: "dom", Slot: RefSlotHash, Anchor: "L1-1 §2 A1 dom/projection.go:530-537"},
	{LegacyPrefix: "fxm_", Family: RefFamilyProjectionContentID, TargetKind: "fxm", Slot: RefSlotHash, Anchor: "L1-1 §2 A2 fxm/projection.go:207-213"},
	{LegacyPrefix: "com_", Family: RefFamilyProjectionContentID, TargetKind: "com", Slot: RefSlotHash, Anchor: "L1-1 §2 A3 com/projection.go:467-473"},
	{LegacyPrefix: "rlm_", Family: RefFamilyProjectionContentID, TargetKind: "rlm", Slot: RefSlotHash, Anchor: "L1-1 §2 A4 rlm/projection.go:738-749"},
	{LegacyPrefix: "mixboard_", Family: RefFamilySnapshotRequestID, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 G1 harness/harness.go:6848-6849"},
	{LegacyPrefix: "kernel_prepared_", Family: RefFamilySnapshotRequestID, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 G2 harness/harness.go:4188-4200"},
	// --- REFSCHEMA-L0-2 additions (anchors: coord/runs/L1-1-RECON-1 §2) ---
	// acp：acousticpackage 投影（kind 名采 L1-3 §3.3 建议 acp）。remainder
	// layer.feature 是数据面地址（scope），revision 指纹属 snapshot 段、legacy
	// 字面量不携带。
	{LegacyPrefix: "acoustic_package_status:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "acp", Slot: RefSlotScope, Anchor: "L1-1 §2 C2 mom/evidence.go:27-32"},
	// DAD 系 scheme 族（内核 C++ D 类 5 处 + Go 侧同字面量生成点）。迁移到
	// vit:// 前以 legacy 翻译条目进中央索引（L1-3 §3.3 audioclosure 行裁定）。
	{LegacyPrefix: "dad.l3.", Family: RefFamilyEvidenceSchemeURI, TargetKind: "dad.l3", Slot: RefSlotScope, Anchor: "L1-1 §2 D3-D5 L3AcousticAnalyzer.cpp:465-660"},
	{LegacyPrefix: "dad.l2_render_probe:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "dad.l2_render_probe", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 C4/D1 mom/projection.go:1012-1020+VitProductionCoordinator.cpp:146"},
	{LegacyPrefix: "dad.compressor_dual_tap:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "dad.compressor_dual_tap", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 D2 VitProductionCoordinator.cpp:526"},
	{LegacyPrefix: "dad.frequency_evidence:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "dad.frequency_evidence", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 C6 mixboard/project_package.go:348"},
	// audioclosure 观察指纹 / frequencycleanup 诊断计划 ID：内容身份（hash 段），
	// 设计未派 kind，TargetKind 留空（与 G 类同款）。
	{LegacyPrefix: "audio_observation:", Family: RefFamilyObservationFingerprint, TargetKind: "", Slot: RefSlotHash, Anchor: "L1-1 §2 B2 audioclosure/driver.go:491-504"},
	{LegacyPrefix: "fci_", Family: RefFamilyObservationFingerprint, TargetKind: "", Slot: RefSlotHash, Anchor: "L1-1 §2 B7 frequencycleanup/treatment.go:86"},
	{LegacyPrefix: "fcp_", Family: RefFamilyObservationFingerprint, TargetKind: "", Slot: RefSlotHash, Anchor: "L1-1 §2 B7 frequencycleanup/treatment.go:94"},
	// --- REFSCHEMA-M1 addition (G1 终审记录 §4 首项) ---
	// mom 观察票加头：identity 族（obs_ ID 含时间戳+随机数，非内容哈希）→
	// slot=snapshot（与 G 类同语义）；TargetKind 留空待 L1-2 结构化键定（与
	// audio_observation:/fci_/fcp_ 同款）。与 audio_observation:（B2 内容指纹，
	// 不同物）无前缀包含关系，最长匹配互不劫持。
	{LegacyPrefix: "observation:", Family: RefFamilyEvidenceSchemeURI, TargetKind: "", Slot: RefSlotSnapshot, Anchor: "L1-1 §2 C3 mom/evidence.go:34-39"},
}

// ProjectionKindEntry registers a projection kind that has no legacy prefix
// family (REFSCHEMA-L0-2): L1-1 §2 A5-A7 establishes MOM/TIM/TOM/EPM carry no
// self-generated id — identity arrives as an externally assigned
// observation_id — so there is no legacy literal to translate and new vit://
// generation addresses the kind directly.
type ProjectionKindEntry struct {
	Kind   string
	Anchor string
	Note   string
}

// projectionKindRegistry lists the prefix-less kinds. It is deliberately a
// separate table from legacyPrefixRegistry: an empty LegacyPrefix would match
// every string in matchLegacyPrefix and flip the whole parse surface to
// legacy state.
var projectionKindRegistry = []ProjectionKindEntry{
	{Kind: "mom", Anchor: "L1-1 §2 A5 mom/types.go:50", Note: "无 legacy 前缀、新生成直接用；snapshot 段承载=observation_id（L1-3 §3.3）"},
	{Kind: "tim", Anchor: "L1-1 §2 A5-A7 tim/types.go:25", Note: "无 legacy 前缀、新生成直接用；snapshot 段承载=observation_id（L1-3 §3.3）"},
	{Kind: "tom", Anchor: "L1-1 §2 A5-A7 tom/types.go:21", Note: "无 legacy 前缀、新生成直接用"},
	{Kind: "epm", Anchor: "L1-1 §2 A5-A7 epm/types.go:22", Note: "无 legacy 前缀、新生成直接用"},
}

var registeredKindSet = func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, entry := range legacyPrefixRegistry {
		if entry.TargetKind != "" {
			set[entry.TargetKind] = struct{}{}
		}
	}
	for _, entry := range projectionKindRegistry {
		set[entry.Kind] = struct{}{}
	}
	return set
}()

// LegacyTranslation describes how a matched legacy literal decomposes toward
// the L0 format: Value is the legacy remainder after the prefix.
type LegacyTranslation struct {
	LegacyPrefix string
	Family       string
	TargetKind   string
	Slot         string
	Value        string
}

// ParsedRef is the three-state parse result. Exactly one of Ref / Legacy is
// set for parsed / legacy; opaque carries only the raw passthrough.
type ParsedRef struct {
	State  RefState
	Raw    string
	Ref    *Ref
	Legacy *LegacyTranslation
}

// RefSchemaWarnLogger, when set by the host process, receives one formatted
// "[agentprotocol.refs] ..." line per first-seen opaque scheme head. Wiring
// it to the agent logx logger is host-side work; nil keeps parsing silent
// and pure (tim.AssertWarnLogger injection pattern).
var RefSchemaWarnLogger func(line string)

var opaqueWarnMu sync.Mutex
var opaqueWarnSeen = map[string]bool{}
var opaqueWarnCounts = map[string]int{}

// ResetOpaqueWarnState clears the opaque WARN-once bookkeeping (tests and
// host-side epoch resets).
func ResetOpaqueWarnState() {
	opaqueWarnMu.Lock()
	defer opaqueWarnMu.Unlock()
	opaqueWarnSeen = map[string]bool{}
	opaqueWarnCounts = map[string]int{}
}

// OpaqueWarnCounts returns a copy of the per-scheme-head occurrence counts,
// for prioritizing registry completion (ruling #8).
func OpaqueWarnCounts() map[string]int {
	opaqueWarnMu.Lock()
	defer opaqueWarnMu.Unlock()
	out := make(map[string]int, len(opaqueWarnCounts))
	for k, v := range opaqueWarnCounts {
		out[k] = v
	}
	return out
}

// LegacyPrefixRegistry returns a copy of the initial legacy prefix registry.
func LegacyPrefixRegistry() []LegacyPrefixEntry {
	return append([]LegacyPrefixEntry(nil), legacyPrefixRegistry...)
}

// ProjectionKindRegistry returns a copy of the prefix-less kind registry
// (kinds whose projections generate no legacy literal).
func ProjectionKindRegistry() []ProjectionKindEntry {
	return append([]ProjectionKindEntry(nil), projectionKindRegistry...)
}

// RegisteredRefKinds returns the projection kinds admitted by the registry.
func RegisteredRefKinds() []string {
	out := make([]string, 0, len(registeredKindSet))
	for _, entry := range legacyPrefixRegistry {
		if entry.TargetKind != "" {
			out = append(out, entry.TargetKind)
		}
	}
	for _, entry := range projectionKindRegistry {
		out = append(out, entry.Kind)
	}
	return out
}

// Validate reports whether the structured ref satisfies the L0 grammar
// constraints (ruling #2/#3 included).
func (r Ref) Validate() error {
	if r.Kind == "" {
		return errors.New("refschema: kind is required")
	}
	if !validKindCharset(r.Kind) {
		return fmt.Errorf("refschema: kind %q violates charset [a-z0-9._]", r.Kind)
	}
	if _, ok := registeredKindSet[r.Kind]; !ok {
		return fmt.Errorf("refschema: kind %q is not registered", r.Kind)
	}
	if r.ScopeKind == "" {
		return errors.New("refschema: scope_kind is required")
	}
	if r.ScopeValue == "" {
		return errors.New("refschema: scope_value is required")
	}
	if r.Window == nil {
		return errors.New("refschema: window segment is required (ruling #2: explicit t=all, omission forbidden)")
	}
	if !r.Window.AllTime && (r.Window.SampleStart < 0 || r.Window.SampleEnd < 0) {
		return errors.New("refschema: sample bounds must be non-negative")
	}
	if r.Snapshot == "" {
		return errors.New("refschema: snapshot is required")
	}
	if err := validateHashSegment(r.Hash); err != nil {
		return err
	}
	return nil
}

// FormatRef serializes the structured ref into the canonical L0 string.
func FormatRef(r Ref) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	window := "t=all"
	if !r.Window.AllTime {
		window = fmt.Sprintf("t=%d..%d", r.Window.SampleStart, r.Window.SampleEnd)
	}
	var b strings.Builder
	b.WriteString(RefSchemePrefix)
	b.WriteString(r.Kind)
	b.WriteString("/")
	b.WriteString(escapeSegment(r.ScopeKind))
	b.WriteString(":")
	b.WriteString(escapeSegment(r.ScopeValue))
	b.WriteString("/")
	b.WriteString(window)
	b.WriteString("@")
	b.WriteString(escapeSegment(r.Snapshot))
	b.WriteString("#")
	b.WriteString(r.Hash)
	return b.String(), nil
}

// ParseRef parses raw into the three-state result. A malformed vit:// ref is
// a hard error (grammar rejection); an unregistered scheme head or vit kind
// is opaque (tolerant parsing, WARN once, passthrough — ruling #8).
func ParseRef(raw string) (ParsedRef, error) {
	if raw == "" {
		return ParsedRef{}, errors.New("refschema: empty ref")
	}
	if !strings.HasPrefix(raw, RefSchemePrefix) {
		if tr, ok := matchLegacyPrefix(raw); ok {
			return ParsedRef{State: RefStateLegacy, Raw: raw, Legacy: &tr}, nil
		}
		recordOpaque(raw, opaqueSchemeHead(raw))
		return ParsedRef{State: RefStateOpaque, Raw: raw}, nil
	}
	rest := raw[len(RefSchemePrefix):]
	head, tail, err := splitFirstUnescaped(rest, '@')
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	snapshotSeg, hashSeg, err := splitFirstUnescaped(tail, '#')
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	parts := splitAllUnescaped(head, '/')
	if len(parts) != 3 {
		return ParsedRef{}, fmt.Errorf("refschema: expected kind/scope/window path segments, got %d in %q", len(parts), raw)
	}
	kindSeg, scopeSeg, windowSeg := parts[0], parts[1], parts[2]
	if !validKindCharset(kindSeg) {
		return ParsedRef{}, fmt.Errorf("refschema: kind %q violates charset [a-z0-9._]", kindSeg)
	}
	scopeKindRaw, scopeValueRaw, err := splitScopeColon(scopeSeg)
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	window, err := parseWindowSegment(windowSeg)
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	if err := validateHashSegment(hashSeg); err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	if _, ok := registeredKindSet[kindSeg]; !ok {
		recordOpaque(raw, "kind:"+kindSeg)
		return ParsedRef{State: RefStateOpaque, Raw: raw}, nil
	}
	scopeKind, err := unescapeSegment(scopeKindRaw)
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	scopeValue, err := unescapeSegment(scopeValueRaw)
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	snapshot, err := unescapeSegment(snapshotSeg)
	if err != nil {
		return ParsedRef{}, fmt.Errorf("refschema: %v in %q", err, raw)
	}
	if scopeKind == "" {
		return ParsedRef{}, fmt.Errorf("refschema: scope_kind is required in %q", raw)
	}
	if scopeValue == "" {
		return ParsedRef{}, fmt.Errorf("refschema: scope_value is required in %q", raw)
	}
	if snapshot == "" {
		return ParsedRef{}, fmt.Errorf("refschema: snapshot is required in %q", raw)
	}
	return ParsedRef{
		State: RefStateParsed,
		Raw:   raw,
		Ref: &Ref{
			Kind:       kindSeg,
			ScopeKind:  scopeKind,
			ScopeValue: scopeValue,
			Window:     window,
			Snapshot:   snapshot,
			Hash:       hashSeg,
		},
	}, nil
}

func matchLegacyPrefix(raw string) (LegacyTranslation, bool) {
	best := -1
	for i, entry := range legacyPrefixRegistry {
		if !strings.HasPrefix(raw, entry.LegacyPrefix) {
			continue
		}
		if best == -1 || len(entry.LegacyPrefix) > len(legacyPrefixRegistry[best].LegacyPrefix) {
			best = i
		}
	}
	if best == -1 {
		return LegacyTranslation{}, false
	}
	entry := legacyPrefixRegistry[best]
	return LegacyTranslation{
		LegacyPrefix: entry.LegacyPrefix,
		Family:       entry.Family,
		TargetKind:   entry.TargetKind,
		Slot:         entry.Slot,
		Value:        raw[len(entry.LegacyPrefix):],
	}, true
}

// opaqueSchemeHead extracts the aggregation key for opaque WARN counting:
// the substring before the first colon, else before the first slash, else
// the whole value.
func opaqueSchemeHead(raw string) string {
	if idx := strings.IndexByte(raw, ':'); idx >= 0 {
		return raw[:idx]
	}
	if idx := strings.IndexByte(raw, '/'); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

func recordOpaque(raw, key string) {
	opaqueWarnMu.Lock()
	count := opaqueWarnCounts[key] + 1
	opaqueWarnCounts[key] = count
	first := !opaqueWarnSeen[key]
	opaqueWarnSeen[key] = true
	opaqueWarnMu.Unlock()
	if first && RefSchemaWarnLogger != nil {
		RefSchemaWarnLogger(fmt.Sprintf("[agentprotocol.refs] state=opaque scheme=%s count=%d ref=%q", key, count, truncateForLog(raw)))
	}
}

func truncateForLog(s string) string {
	const max = 96
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func validKindCharset(kind string) bool {
	if kind == "" {
		return false
	}
	for i := 0; i < len(kind); i++ {
		c := kind[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_':
		default:
			return false
		}
	}
	return true
}

func validateHashSegment(hash string) error {
	if hash == "-" {
		return nil
	}
	const prefix = "sha256:"
	if !strings.HasPrefix(hash, prefix) {
		return fmt.Errorf("hash %q must be %q (explicit un-CASed) or %s<16 lowercase hex>", hash, "-", prefix)
	}
	hexPart := hash[len(prefix):]
	if len(hexPart) != 16 {
		return fmt.Errorf("hash %q must carry exactly 16 hex digits", hash)
	}
	for i := 0; i < len(hexPart); i++ {
		c := hexPart[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf("hash %q must use lowercase hex", hash)
		}
	}
	return nil
}

func parseWindowSegment(seg string) (*TimeWindow, error) {
	if seg == "t=all" {
		return &TimeWindow{AllTime: true}, nil
	}
	const prefix = "t="
	if !strings.HasPrefix(seg, prefix) {
		return nil, fmt.Errorf("window %q must be t=all or t=<uint>..<uint>", seg)
	}
	rest := seg[len(prefix):]
	idx := strings.Index(rest, "..")
	if idx < 0 {
		return nil, fmt.Errorf("window %q misses the .. range separator", seg)
	}
	startStr, endStr := rest[:idx], rest[idx+2:]
	start, err := parseSampleBound(startStr)
	if err != nil {
		return nil, fmt.Errorf("window %q: %v", seg, err)
	}
	end, err := parseSampleBound(endStr)
	if err != nil {
		return nil, fmt.Errorf("window %q: %v", seg, err)
	}
	return &TimeWindow{SampleStart: start, SampleEnd: end}, nil
}

func parseSampleBound(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("empty sample bound")
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("sample bound %q has a leading zero", s)
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("sample bound %q is not decimal digits", s)
		}
	}
	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("sample bound %q overflows int64", s)
	}
	return value, nil
}

func escapeSegment(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(refEscapeReserved, c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// unescapeSegment decodes strict percent escapes and rejects any raw
// reserved character (canonical-form enforcement, keeps escapes closed).
func unescapeSegment(seg string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if c == '%' {
			if i+2 >= len(seg) {
				return "", fmt.Errorf("segment %q has a truncated escape", seg)
			}
			hi := hexDigit(seg[i+1])
			lo := hexDigit(seg[i+2])
			if hi < 0 || lo < 0 {
				return "", fmt.Errorf("segment %q has a non-hex escape", seg)
			}
			b.WriteByte(byte(hi<<4 | lo))
			i += 2
			continue
		}
		if strings.IndexByte(refEscapeReserved, c) >= 0 {
			return "", fmt.Errorf("segment %q contains raw reserved char %q", seg, string(c))
		}
		b.WriteByte(c)
	}
	return b.String(), nil
}

// splitFirstUnescaped splits s at the first occurrence of sep that is not
// inside a percent escape. Escape validity is enforced while scanning.
func splitFirstUnescaped(s string, sep byte) (string, string, error) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' {
			if i+2 >= len(s) {
				return "", "", errors.New("truncated escape")
			}
			if hexDigit(s[i+1]) < 0 || hexDigit(s[i+2]) < 0 {
				return "", "", errors.New("non-hex escape")
			}
			i += 2
			continue
		}
		if c == sep {
			return s[:i], s[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("missing %q segment", string(sep))
}

func splitAllUnescaped(s string, sep byte) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' {
			i += 2 // escape validity already enforced by the @ split
			continue
		}
		if c == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

// splitScopeColon splits the scope segment at its single structural colon.
// Any further raw colon surfaces later as a canonical-form rejection in
// unescapeSegment.
func splitScopeColon(seg string) (string, string, error) {
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		if c == '%' {
			if i+2 >= len(seg) {
				return "", "", errors.New("truncated escape in scope")
			}
			if hexDigit(seg[i+1]) < 0 || hexDigit(seg[i+2]) < 0 {
				return "", "", errors.New("non-hex escape in scope")
			}
			i += 2
			continue
		}
		if c == ':' {
			return seg[:i], seg[i+1:], nil
		}
	}
	return "", "", errors.New("scope must be scope_kind:scope_value")
}
