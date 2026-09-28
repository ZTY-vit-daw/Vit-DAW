package queryengine

// index.go — 中央段索引（引擎自有，必建）：ref 五段 + freshness + 声明标量载荷。
//
// 形态（§3.1/§3.4）：v1 内存 copy-on-write，整表不可变快照交换——构建新不可变
// 整表后原子换指针，读侧永远无锁等待。写放大在 v1 规模（≤10⁵ ref）可接受；
// 增量化属 L1-2 物化层。数据全部来自物化读口（§5），不可规范化的行
//（未注册 kind / 缺段）建索引时跳过（T4：opaque 不进索引）。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

type indexedRow struct {
	ref       agentprotocol.Ref
	canonical string
	freshness string
	payload   map[string]any
}

type centralIndex struct {
	gen     string // 代摘要（可重放性句柄；v1 为引擎侧摘要，权威代 ID 归 L1-2）
	rows    []indexedRow
	byKind  map[string][]int
	skipped int
}

// buildCentralIndex 从物化行集构建不可变中央索引。
func buildCentralIndex(rows []MaterializedRow) *centralIndex {
	indexed := make([]indexedRow, 0, len(rows))
	skipped := 0
	for _, r := range rows {
		c, err := agentprotocol.FormatRef(r.Ref)
		if err != nil {
			skipped++ // 未注册 kind / 缺段 / 非法 hash：不进索引（T4）
			continue
		}
		indexed = append(indexed, indexedRow{ref: r.Ref, canonical: c, freshness: r.Freshness, payload: r.Payload})
	}
	return finalizeIndex(indexed, skipped)
}

func finalizeIndex(rows []indexedRow, skipped int) *centralIndex {
	idx := &centralIndex{rows: rows, byKind: make(map[string][]int, 16), skipped: skipped}
	keys := defaultSortKeys()
	sort.SliceStable(idx.rows, func(i, j int) bool {
		return compareIndexedRows(idx.rows[i], idx.rows[j], keys) < 0
	})
	for i, r := range idx.rows {
		idx.byKind[r.ref.Kind] = append(idx.byKind[r.ref.Kind], i)
	}
	idx.gen = generationDigest(idx.rows)
	return idx
}

// generationDigest 代摘要 = 全部行 canonical+freshness 的 sha256 前 16hex。
// marked_stale 改变 freshness 即换代——stale 事件对快照一致性可见（T5/T6）。
func generationDigest(rows []indexedRow) string {
	h := sha256.New()
	for _, r := range rows {
		h.Write([]byte(r.canonical))
		h.Write([]byte{'\t'})
		h.Write([]byte(r.freshness))
		h.Write([]byte{'\n'})
	}
	return "gen:" + hex.EncodeToString(h.Sum(nil)[:8])
}

// withChange 对当前代应用一次物化变更，产出新不可代（copy-on-write 整表）。
// 坏行（不可规范化）变更与索引中不存在行的 marked_stale 均为无操作。
func (idx *centralIndex) withChange(ch MaterializedChange) *centralIndex {
	key, err := agentprotocol.FormatRef(ch.Row.Ref)
	if err != nil {
		return idx
	}
	rows := make([]indexedRow, len(idx.rows))
	copy(rows, idx.rows)
	next := indexedRow{ref: ch.Row.Ref, canonical: key, freshness: ch.Row.Freshness, payload: ch.Row.Payload}
	for i := range rows {
		if rows[i].canonical != key {
			continue
		}
		if ch.Op == MaterializedMarkedStale {
			rows[i].freshness = FreshnessStale // 只改状态不删行（§5.1.1）
		} else {
			rows[i] = next
		}
		return finalizeIndex(rows, idx.skipped)
	}
	if ch.Op == MaterializedMarkedStale {
		return idx // 从未存在的行标脏：无操作
	}
	return finalizeIndex(append(rows, next), idx.skipped)
}

// evaluate 过滤 + 排序（不含分页）。snapshot 谓词不在此层：代际选择归物化层
// （§5.1.2），本索引本身就是单一快照代的视图。
func (idx *centralIndex) evaluate(q RefQuery, kinds []string) []indexedRow {
	kindSet := make(map[string]struct{}, len(kinds))
	for _, k := range kinds {
		kindSet[k] = struct{}{}
	}
	matches := make([]indexedRow, 0, 16)
	if idx.byKind != nil {
		for _, kind := range kinds {
			for _, i := range idx.byKind[kind] {
				if rowMatches(idx.rows[i], q) {
					matches = append(matches, idx.rows[i])
				}
			}
		}
	} else {
		for _, r := range idx.rows {
			if _, ok := kindSet[r.ref.Kind]; !ok {
				continue
			}
			if rowMatches(r, q) {
				matches = append(matches, r)
			}
		}
	}
	keys := q.Sort
	if len(keys) == 0 {
		keys = defaultSortKeys()
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return compareIndexedRows(matches[i], matches[j], keys) < 0
	})
	return matches
}

func rowMatches(row indexedRow, q RefQuery) bool {
	return scopeMatches(row.ref, q.Scope) &&
		windowMatches(row.ref, q.Window) &&
		hashMatches(row.ref, q.Hash) &&
		freshnessMatches(row.freshness, q.Freshness) &&
		payloadMatchesAll(row.payload, q.Payload)
}

func scopeMatches(ref agentprotocol.Ref, p *ScopePredicate) bool {
	if p == nil {
		return true
	}
	if p.Kind != "" && ref.ScopeKind != p.Kind {
		return false
	}
	if p.ValueSet {
		for _, v := range p.Values {
			if ref.ScopeValue == v {
				return true
			}
		}
		return false
	}
	if p.PrefixSet {
		return strings.HasPrefix(ref.ScopeValue, p.Prefix)
	}
	return true
}

func windowMatches(ref agentprotocol.Ref, p *WindowPredicate) bool {
	if p == nil {
		return true
	}
	if p.AllTime {
		return ref.Window != nil && ref.Window.AllTime
	}
	if ref.Window == nil {
		return false
	}
	if ref.Window.AllTime {
		// t=all 覆盖全时间线：与有限窗相交、覆盖之；不可能被有限窗包含
		return p.Relation == "" || p.Relation == WindowIntersects || p.Relation == WindowWithin
	}
	ws, we := ref.Window.SampleStart, ref.Window.SampleEnd
	switch p.Relation {
	case "", WindowIntersects:
		return ws <= p.SampleEnd && p.SampleStart <= we
	case WindowContains:
		return p.SampleStart <= ws && we <= p.SampleEnd
	case WindowWithin:
		return ws <= p.SampleStart && p.SampleEnd <= we
	}
	return false
}

func hashMatches(ref agentprotocol.Ref, p *HashPredicate) bool {
	if p == nil {
		return true
	}
	if p.Any {
		return true
	}
	if p.SHA256 == "" {
		return ref.Hash != "-"
	}
	return ref.Hash == "sha256:"+p.SHA256
}

func freshnessMatches(freshness string, want []string) bool {
	if len(want) == 0 {
		return true // 默认不过滤：stale 行保留（F8/M1）
	}
	for _, f := range want {
		if f == freshness {
			return true
		}
	}
	return false
}

func payloadMatchesAll(payload map[string]any, conds []PayloadCondition) bool {
	for _, c := range conds {
		if !payloadMatches(payload, c) {
			return false
		}
	}
	return true
}

func payloadMatches(payload map[string]any, c PayloadCondition) bool {
	v, ok := payload[c.Field]
	if !ok {
		return false // 未提供的字段：任何条件为假（"声明且提供"）
	}
	if c.Op == PayloadIn {
		s := fmt.Sprint(v)
		for _, item := range c.Set {
			if item == s {
				return true
			}
		}
		return false
	}
	if c.Str != "" {
		s := fmt.Sprint(v)
		if c.Op == PayloadEq {
			return s == c.Str
		}
		if c.Op == PayloadNe {
			return s != c.Str
		}
		return false
	}
	f, ok := numericOf(v)
	if !ok {
		return false
	}
	switch c.Op {
	case PayloadEq:
		return f == c.Value
	case PayloadNe:
		return f != c.Value
	case PayloadGt:
		return f > c.Value
	case PayloadGe:
		return f >= c.Value
	case PayloadLt:
		return f < c.Value
	case PayloadLe:
		return f <= c.Value
	}
	return false
}

func numericOf(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint64:
		return float64(t), true
	}
	return 0, false
}

func defaultSortKeys() []SortKey {
	return []SortKey{
		{Field: "ref.kind"},
		{Field: "ref.scope_kind"},
		{Field: "ref.scope_value"},
		{Field: "ref.window.sample_start"},
	}
}

// ---- 排序比较 ----
//
// 规格语义（引擎与测试 oracle 两侧独立实现，T1/T2 锁定）：
//   - ref 别名恒可排；AllTime 窗以 -1 哨兵排组内最前
//   - 载荷值缺失恒排末尾（与方向无关；desc 只翻转"双方都在"的比较）
//   - 数值与字符串混排时数值在前
//   - 全部键平手按 canonical 升序定全序（分页稳定性）

type sortValue struct {
	num bool
	n   float64
	s   string
}

func compareIndexedRows(a, b indexedRow, keys []SortKey) int {
	for _, k := range keys {
		av, aok := sortValueOf(a, k.Field)
		bv, bok := sortValueOf(b, k.Field)
		c := 0
		switch {
		case !aok && !bok:
			c = 0
		case !aok:
			c = 1
		case !bok:
			c = -1
		default:
			c = compareSortValues(av, bv)
		}
		if k.Dir == SortDesc && aok && bok {
			c = -c
		}
		if c != 0 {
			return c
		}
	}
	return strings.Compare(a.canonical, b.canonical)
}

func sortValueOf(row indexedRow, field string) (sortValue, bool) {
	var out sortValue
	switch field {
	case "ref.kind":
		out.s = row.ref.Kind
	case "ref.scope_kind":
		out.s = row.ref.ScopeKind
	case "ref.scope_value":
		out.s = row.ref.ScopeValue
	case "ref.snapshot":
		out.s = row.ref.Snapshot
	case "ref.hash":
		out.s = row.ref.Hash
	case "ref.window.sample_start", "ref.window.sample_end":
		if row.ref.Window == nil {
			return out, false
		}
		out.num = true
		if row.ref.Window.AllTime {
			out.n = -1
		} else if field == "ref.window.sample_start" {
			out.n = float64(row.ref.Window.SampleStart)
		} else {
			out.n = float64(row.ref.Window.SampleEnd)
		}
	default:
		v, ok := row.payload[field]
		if !ok {
			return out, false
		}
		switch t := v.(type) {
		case string:
			out.s = t
		case bool:
			out.s = fmt.Sprint(t)
		default:
			if f, numeric := numericOf(v); numeric {
				out.num = true
				out.n = f
			} else {
				out.s = fmt.Sprint(v)
			}
		}
	}
	return out, true
}

func compareSortValues(a, b sortValue) int {
	if a.num != b.num {
		if a.num {
			return -1
		}
		return 1
	}
	if a.num {
		switch {
		case a.n < b.n:
			return -1
		case a.n > b.n:
			return 1
		}
		return 0
	}
	return strings.Compare(a.s, b.s)
}
