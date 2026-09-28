package queryengine

// predicates.go — §2.1 谓词模型：L0 五段的结构化查询形态。
//
// 全部段谓词直接对应 agentprotocol.Ref 字段，不自创第二套寻址。
// 校验在 Query/Route 入口执行；nil/零值 = 该段不约束。

import (
	"fmt"
)

// ---- 成本分级（D5）与 freshness 词汇 ----

const (
	CostClassIndex   = "index"   // 毫秒级（内存索引）
	CostClassCompile = "compile" // 亚秒级（降级逐行/触发现算）
	CostClassProbe   = "probe"   // 秒级（真实 render，本卡不触达）
)

const (
	FreshnessCurrent       = "current"
	FreshnessMaterialReuse = "material_reuse"
	FreshnessStale         = "stale"
)

// DegradedPayloadIndexUnavailable R5/R6 降级原因标记（响应明示，模型看得见）。
const DegradedPayloadIndexUnavailable = "payload_index_unavailable"

// 分页默认与硬上限（§2.1）。
const (
	DefaultQueryLimit = 50
	MaxQueryLimit     = 500
)

// ---- 段谓词 ----

// KindPredicate 约束 kind 段。空集 = 全部注册 kind；未注册 kind 一律
// ErrUnknownKind（不静默空集——"空结果"与"kind 不存在"是两种信号）。
type KindPredicate struct {
	Kinds []string // ⊆ agentprotocol.RegisteredRefKinds()；校验在入口
}

// ScopePredicate 约束 scope 段（scope_kind:scope_value）。
// Values 与 Prefix 并设时精确集合优先（更具体者赢）。
type ScopePredicate struct {
	Kind      string   // 精确匹配 scope_kind（空 = 不约束）
	Values    []string // scope_value 精确集合
	Prefix    string   // scope_value 前缀（总线子树遍历）
	ValueSet  bool     // Values 生效
	PrefixSet bool     // Prefix 生效
}

// WindowRelation 窗口关系三态（采样点权威层，D3；秒/小节由工具面换算后进入）。
type WindowRelation string

const (
	WindowIntersects WindowRelation = "intersects" // 有交集（含端点相接）
	WindowContains   WindowRelation = "contains"   // ref 窗完整落在查询窗内
	WindowWithin     WindowRelation = "within"     // ref 窗完整覆盖查询窗
)

// WindowPredicate 约束 window 段。t=all 的 ref 对有限查询窗：相交 ✓ 覆盖 ✓
// 被包含 ✗（全时间线不可能落在有限窗内）；AllTime=true 只匹配 t=all 的 ref。
// SampleStart==SampleEnd==0 且 AllTime=false 是显式点查。
type WindowPredicate struct {
	Relation    WindowRelation // 空 = 默认 intersects
	SampleStart int64          // 含
	SampleEnd   int64          // 含
	AllTime     bool
}

// SnapshotMode 快照模式三态。
type SnapshotMode string

const (
	SnapshotLatest     SnapshotMode = "latest"       // 物化层当前已提交快照（默认）
	SnapshotExact      SnapshotMode = "exact"        // 精确 revision/observation_id
	SnapshotAtOrBefore SnapshotMode = "at_or_before" // ≤ 给定 revision 的最近一份
)

// SnapshotPredicate 约束 snapshot 段（数据版本轴）。代际解析归物化层（§5.1.2），
// 引擎不做 revision 比较——谓词只决定取哪个 SnapshotView。
type SnapshotPredicate struct {
	Mode     SnapshotMode // 空 = latest
	Revision string       // exact / at_or_before 必填；latest 必空
}

// HashPredicate 约束 hash 段（内容身份轴，CAS 天然支持）。
type HashPredicate struct {
	SHA256 string // 精确内容身份（16hex，不含 "sha256:" 前缀）
	Any    bool   // true = 不限（含 #-）；false 且 SHA256 空 = 排除 #-
}

// ---- 载荷谓词 ----

// PayloadOp 载荷算子。
type PayloadOp string

const (
	PayloadEq PayloadOp = "eq"
	PayloadNe PayloadOp = "ne"
	PayloadGt PayloadOp = "gt" // 数值型
	PayloadGe PayloadOp = "ge"
	PayloadLt PayloadOp = "lt"
	PayloadLe PayloadOp = "le"
	PayloadIn PayloadOp = "in" // 字符串集合
)

// PayloadCondition 单条载荷条件。字段名带 kind 前缀防跨层撞名
// （kind.domain.path 三段命名，§3.2）。求值语义：
//   - 字段缺失于行载荷 → 条件为假（"声明且提供"，任何算子同）
//   - Str 非空 → 按字符串形态比较（eq/ne/in；payload 值取其字符串形）
//   - Str 空 → 按数值比较（payload 值须可数值化，否则为假）
type PayloadCondition struct {
	Field string
	Op    PayloadOp
	Value float64
	Str   string
	Set   []string
}

// ---- 排序 / 分页 ----

type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// SortKey Field 为 ref 段受控别名（refSortAliases）或已声明的载荷字段。
type SortKey struct {
	Field string
	Dir   SortDirection // 空 = asc
}

// RefQuery 是 query 动词的完整输入（Go 函数面）。
type RefQuery struct {
	Kinds    KindPredicate
	Scope    *ScopePredicate
	Window   *WindowPredicate
	Snapshot SnapshotPredicate // 零值 = latest
	Hash     *HashPredicate
	Payload  []PayloadCondition // AND 语义（v1 无 OR；模型用两次 query 组合）
	Sort     []SortKey          // 默认 kind/scope_kind/scope_value/window.sample_start
	Limit    int                // 0 = 默认 50；硬上限 500
	Cursor   string             // 不透明续页游标（上次响应 next_cursor）
	Expand   *ExpandOptions     // 可选展开（§2.4）
	// Freshness 显式 freshness 过滤（§5.1.1/T6：默认不过滤，stale 原样保留；
	// §2.1 结构体未落位该字段，此处按 §5.1.1 契约补齐——字符串成员匹配，
	// 不做值域校验，未知值=空集）。
	Freshness []string
}

// ResultRow 单命中行。
type ResultRow struct {
	Ref       string         // canonical L0 串（FormatRef 产物）
	Freshness string         // 透传，不升级
	Payload   map[string]any // 该行携带的声明标量
	// Expanded 内联展开（§2.3 ref.query 的 expand 参数承载点；§2.1 结构体未
	// 落位该字段，此处补齐）。nil = 未请求展开。
	Expanded *ExpandedRef
}

// QueryResult query 响应。成本分级（D5）是硬要求，不是装饰字段。
type QueryResult struct {
	Rows         []ResultRow
	TotalMatches int    // 谓词总命中数（不受 Limit 截断）；委托路径为下界（见 engine.go 注记）
	NextCursor   string // 非空 = 还有结果
	Snapshot     string // 本次查询实际绑定的快照代标识（可重放性；v1 为引擎侧代摘要）
	CostClass    string // "index" | "compile"
	Degraded     string // 空 = 走索引；非空 = 降级原因
}

// refSortAliases ref 段排序受控别名全集（Catalog 同源）。
func refSortAliases() []string {
	return []string{
		"ref.kind", "ref.scope_kind", "ref.scope_value",
		"ref.window.sample_start", "ref.window.sample_end",
		"ref.snapshot", "ref.hash",
	}
}

func isRefSortAlias(field string) bool {
	for _, a := range refSortAliases() {
		if a == field {
			return true
		}
	}
	return false
}

// validateRefQuery 校验除 kind 注册面与排序声明之外的全部谓词
// （kind 面在 resolveQueryKinds；排序声明在 validateSortFields，需引擎注册表）。
func validateRefQuery(q RefQuery) error {
	if s := q.Scope; s != nil {
		if s.ValueSet && len(s.Values) == 0 {
			return fmt.Errorf("%w: ValueSet 但 Values 为空", ErrInvalidScope)
		}
		if s.PrefixSet && s.Prefix == "" {
			return fmt.Errorf("%w: PrefixSet 但 Prefix 为空", ErrInvalidScope)
		}
	}
	if w := q.Window; w != nil && !w.AllTime {
		if w.SampleStart < 0 || w.SampleEnd < 0 {
			return fmt.Errorf("%w: 采样点为负 (%d..%d)", ErrInvalidWindow, w.SampleStart, w.SampleEnd)
		}
		if w.SampleStart > w.SampleEnd {
			return fmt.Errorf("%w: start > end (%d..%d)", ErrInvalidWindow, w.SampleStart, w.SampleEnd)
		}
		if w.Relation != "" && w.Relation != WindowIntersects && w.Relation != WindowContains && w.Relation != WindowWithin {
			return fmt.Errorf("%w: 未知关系 %q", ErrInvalidWindow, w.Relation)
		}
	}
	mode := q.Snapshot.Mode
	if mode == "" {
		mode = SnapshotLatest
	}
	switch mode {
	case SnapshotLatest:
		if q.Snapshot.Revision != "" {
			return fmt.Errorf("%w: latest 模式不带 revision", ErrInvalidSnapshot)
		}
	case SnapshotExact, SnapshotAtOrBefore:
		if q.Snapshot.Revision == "" {
			return fmt.Errorf("%w: %s 模式要求 revision", ErrInvalidSnapshot, mode)
		}
	default:
		return fmt.Errorf("%w: 未知模式 %q", ErrInvalidSnapshot, mode)
	}
	if h := q.Hash; h != nil {
		if h.Any && h.SHA256 != "" {
			return fmt.Errorf("%w: Any 与 SHA256 并设矛盾", ErrInvalidHash)
		}
		if h.SHA256 != "" && !isLowerHex16(h.SHA256) {
			return fmt.Errorf("%w: %q 非 16 位小写 hex", ErrInvalidHash, h.SHA256)
		}
	}
	for i, c := range q.Payload {
		if c.Field == "" {
			return fmt.Errorf("%w: 第 %d 条缺字段名", ErrInvalidPayloadCondition, i)
		}
		switch c.Op {
		case PayloadEq, PayloadNe, PayloadGt, PayloadGe, PayloadLt, PayloadLe, PayloadIn:
		default:
			return fmt.Errorf("%w: %q", ErrInvalidPayloadCondition, c.Op)
		}
	}
	if q.Limit < 0 || q.Limit > MaxQueryLimit {
		return fmt.Errorf("%w: %d（默认 %d，硬上限 %d）", ErrInvalidLimit, q.Limit, DefaultQueryLimit, MaxQueryLimit)
	}
	return nil
}

func isLowerHex16(s string) bool {
	if len(s) != 16 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// expandTypes 在此归位（§2.4）：Expand 只给句柄+禁键过滤后摘要，不装配
// LLMContext、不走披露预算——语义深读仍走 observe（CCB）。
type ExpandDepth string

const (
	ExpandHandle  ExpandDepth = "handle"  // evidence:// 句柄+字节数+freshness
	ExpandSummary ExpandDepth = "summary" // 句柄 + 字节预算内摘要
)

// ExpandOptions 展开选项。
type ExpandOptions struct {
	Depth    ExpandDepth // 空 = handle
	MaxBytes int         // summary 摘要预算，默认 4096，硬上限 32 KiB
}

// ExpandedRef 展开产物。
type ExpandedRef struct {
	Ref       string
	Handle    string // evidence://<sha256>（物化层 Resolve 产出）
	Bytes     int64
	Freshness string // 透传；opaque/legacy 透传行分别记 "opaque"/"legacy"
	Summary   map[string]any
	Truncated bool
}
