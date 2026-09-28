package queryengine

// helpers_test.go — 测试基建：fake MaterializedStore、ref/row 构造器、oracle。
//
// oracle 是与引擎完全独立的朴素线性扫描实现（设计 §6.2"全量过滤对拍"的字面
// 落地）：不 import 引擎的任何求值辅助，不复用谓词匹配代码路径。规格语义
// （窗口三关系/缺失字段排序/scope 精确优先于前缀 等）在两侧各自独立编码，
// 分歧即 bug——改规格语义必须两侧同时改并说明依据。

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/agentprotocol"
)

// ---- 构造器 ----

func hex16(n int) string { return fmt.Sprintf("%016x", n) }

func winRef(kind, scopeKind, scopeValue string, start, end int64, snapshot string, hashN int) agentprotocol.Ref {
	return agentprotocol.Ref{
		Kind:       kind,
		ScopeKind:  scopeKind,
		ScopeValue: scopeValue,
		Window:     &agentprotocol.TimeWindow{SampleStart: start, SampleEnd: end},
		Snapshot:   snapshot,
		Hash:       "sha256:" + hex16(hashN),
	}
}

func allRef(kind, scopeKind, scopeValue, snapshot string, hashN int) agentprotocol.Ref {
	return agentprotocol.Ref{
		Kind:       kind,
		ScopeKind:  scopeKind,
		ScopeValue: scopeValue,
		Window:     &agentprotocol.TimeWindow{AllTime: true},
		Snapshot:   snapshot,
		Hash:       "sha256:" + hex16(hashN),
	}
}

func unCASRef(kind, scopeKind, scopeValue string, start, end int64, snapshot string) agentprotocol.Ref {
	return agentprotocol.Ref{
		Kind:       kind,
		ScopeKind:  scopeKind,
		ScopeValue: scopeValue,
		Window:     &agentprotocol.TimeWindow{SampleStart: start, SampleEnd: end},
		Snapshot:   snapshot,
		Hash:       "-",
	}
}

func mrow(ref agentprotocol.Ref, freshness string, payload map[string]any) MaterializedRow {
	return MaterializedRow{Ref: ref, Freshness: freshness, Payload: payload}
}

func canon(r agentprotocol.Ref) string {
	s, err := agentprotocol.FormatRef(r)
	if err != nil {
		panic(fmt.Sprintf("fixture ref not canonicalizable: %v", err))
	}
	return s
}

// ---- fake MaterializedStore ----

type fakeStore struct {
	mu      sync.Mutex
	rows    []MaterializedRow
	content map[string][]byte // handle -> 内容（Resolve 的 ReadAll 用）
	changes chan MaterializedChange
}

func newFakeStore(rows ...MaterializedRow) *fakeStore {
	return &fakeStore{
		rows:    append([]MaterializedRow(nil), rows...),
		content: map[string][]byte{},
		changes: make(chan MaterializedChange, 128),
	}
}

func (f *fakeStore) SnapshotView(ctx context.Context, sel SnapshotSelector) ([]MaterializedRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch sel.Mode {
	case SnapshotExact:
		out := []MaterializedRow{}
		for _, r := range f.rows {
			if r.Ref.Snapshot == sel.Revision || (sel.ObservationID != "" && r.Ref.Snapshot == sel.ObservationID) {
				out = append(out, r)
			}
		}
		return out, nil
	default:
		// latest / at_or_before：fake 不做代际管理（代际语义归物化层，§5.1.2）
		return append([]MaterializedRow(nil), f.rows...), nil
	}
}

func (f *fakeStore) Resolve(ctx context.Context, ref agentprotocol.Ref) (ResolvedEvidence, error) {
	key := canon(ref)
	f.mu.Lock()
	defer f.mu.Unlock()
	handle := "artifact://" + key
	if ref.Hash != "-" {
		handle = "evidence://sha256-" + strings.TrimPrefix(ref.Hash, "sha256:")
	}
	content, ok := f.content[handle]
	if !ok {
		content = []byte(fmt.Sprintf(`{"ref":%q,"freshness":"current"}`, key))
	}
	return ResolvedEvidence{
		Handle:    handle,
		Bytes:     int64(len(content)),
		Freshness: FreshnessCurrent,
		ReadAll: func() ([]byte, error) {
			return append([]byte(nil), content...), nil
		},
	}, nil
}

func (f *fakeStore) Subscribe(ctx context.Context) (<-chan MaterializedChange, func(), error) {
	return f.changes, func() {}, nil
}

// snapshotRows 返回当前全量行集副本（世代对照样本用）。
func (f *fakeStore) snapshotRows() []MaterializedRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]MaterializedRow(nil), f.rows...)
}

// push 让 fake store 状态前移并广播变更（模拟物化层变更流）。
func (f *fakeStore) push(t *testing.T, ch MaterializedChange) {
	t.Helper()
	f.mu.Lock()
	key := canon(ch.Row.Ref)
	switch ch.Op {
	case MaterializedMarkedStale:
		for i := range f.rows {
			if canon(f.rows[i].Ref) == key {
				f.rows[i].Freshness = FreshnessStale
			}
		}
	default: // added / replaced = 按 canonical upsert
		replaced := false
		for i := range f.rows {
			if canon(f.rows[i].Ref) == key {
				f.rows[i] = ch.Row
				replaced = true
				break
			}
		}
		if !replaced {
			f.rows = append(f.rows, ch.Row)
		}
	}
	f.mu.Unlock()
	select {
	case f.changes <- ch:
	default:
		t.Fatalf("fake store change channel full")
	}
}

func newTestEngine(t *testing.T, rows ...MaterializedRow) (*Engine, *fakeStore) {
	t.Helper()
	st := newFakeStore(rows...)
	e := NewEngine(st, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := e.Start(ctx); err != nil {
		t.Fatalf("engine start: %v", err)
	}
	return e, st
}

// ---- oracle：独立朴素实现（不共享引擎求值代码路径）----

type oracleRow struct {
	ref       agentprotocol.Ref
	canonical string
	freshness string
	payload   map[string]any
}

func oracleRowsOf(rows []MaterializedRow) []oracleRow {
	out := make([]oracleRow, 0, len(rows))
	for _, r := range rows {
		c, err := agentprotocol.FormatRef(r.Ref)
		if err != nil {
			continue // 不可规范化的行（未注册 kind 等）不进结果面——与引擎建索引同规则
		}
		out = append(out, oracleRow{ref: r.Ref, canonical: c, freshness: r.Freshness, payload: r.Payload})
	}
	return out
}

func oracleFilter(rows []oracleRow, q RefQuery) []oracleRow {
	kinds := map[string]bool{}
	if len(q.Kinds.Kinds) == 0 {
		for _, k := range agentprotocol.RegisteredRefKinds() {
			kinds[k] = true
		}
	} else {
		for _, k := range q.Kinds.Kinds {
			kinds[k] = true
		}
	}
	out := []oracleRow{}
	for _, r := range rows {
		if !kinds[r.ref.Kind] {
			continue
		}
		if !oracleScopeMatch(r.ref, q.Scope) {
			continue
		}
		if !oracleWindowMatch(r.ref, q.Window) {
			continue
		}
		if !oracleHashMatch(r.ref, q.Hash) {
			continue
		}
		if len(q.Freshness) > 0 && !stringIn(q.Freshness, r.freshness) {
			continue
		}
		ok := true
		for _, c := range q.Payload {
			if !oraclePayloadMatch(r.payload, c) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		out = append(out, r)
	}
	oracleSortRows(out, q.Sort)
	return out
}

func stringIn(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func oracleScopeMatch(ref agentprotocol.Ref, p *ScopePredicate) bool {
	if p == nil {
		return true
	}
	if p.Kind != "" && ref.ScopeKind != p.Kind {
		return false
	}
	if p.ValueSet {
		// 精确集合优先于前缀（规格裁定：两者并设时精确赢）
		return stringIn(p.Values, ref.ScopeValue)
	}
	if p.PrefixSet {
		return strings.HasPrefix(ref.ScopeValue, p.Prefix)
	}
	return true
}

func oracleWindowMatch(ref agentprotocol.Ref, p *WindowPredicate) bool {
	if p == nil {
		return true
	}
	if p.AllTime {
		return ref.Window != nil && ref.Window.AllTime
	}
	rel := p.Relation
	if rel == "" {
		rel = WindowIntersects
	}
	if ref.Window == nil {
		return false
	}
	if ref.Window.AllTime {
		// t=all 覆盖全时间线：与任意有限窗相交、覆盖之；不可能被有限窗包含
		return rel == WindowIntersects || rel == WindowWithin
	}
	ws, we := ref.Window.SampleStart, ref.Window.SampleEnd
	switch rel {
	case WindowIntersects: // 含端点相接
		return ws <= p.SampleEnd && p.SampleStart <= we
	case WindowContains: // ref 窗完整落在查询窗内
		return p.SampleStart <= ws && we <= p.SampleEnd
	case WindowWithin: // ref 窗完整覆盖查询窗
		return ws <= p.SampleStart && p.SampleEnd <= we
	}
	return false
}

func oracleHashMatch(ref agentprotocol.Ref, p *HashPredicate) bool {
	if p == nil {
		return true
	}
	if p.Any {
		return true
	}
	if p.SHA256 == "" {
		return ref.Hash != "-" // 排除显式未 CAS 化
	}
	return ref.Hash == "sha256:"+p.SHA256
}

func oraclePayloadMatch(payload map[string]any, c PayloadCondition) bool {
	v, ok := payload[c.Field]
	if !ok {
		return false // 未提供的字段：任何条件都为假（"声明且提供"语义）
	}
	if c.Op == PayloadIn {
		s := fmt.Sprint(v)
		return stringIn(c.Set, s)
	}
	if c.Str != "" {
		s := fmt.Sprint(v)
		return (c.Op == PayloadEq && s == c.Str) || (c.Op == PayloadNe && s != c.Str)
	}
	f, ok := oracleToFloat(v)
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

func oracleToFloat(v any) (float64, bool) {
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
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	}
	return 0, false
}

// oracle 排序：默认键 = kind/scope_kind/scope_value/window.sample_start；
// 缺失载荷值恒排末尾（与方向无关）；全部键平手时按 canonical 升序定全序。
func oracleSortRows(rows []oracleRow, keys []SortKey) {
	if len(keys) == 0 {
		keys = []SortKey{
			{Field: "ref.kind"}, {Field: "ref.scope_kind"}, {Field: "ref.scope_value"}, {Field: "ref.window.sample_start"},
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return oracleCompareRows(rows[i], rows[j], keys) < 0
	})
}

func oracleCompareRows(a, b oracleRow, keys []SortKey) int {
	for _, k := range keys {
		av, aok := oracleSortValue(a, k.Field)
		bv, bok := oracleSortValue(b, k.Field)
		c := 0
		switch {
		case !aok && !bok:
			c = 0
		case !aok:
			c = 1
		case !bok:
			c = -1
		default:
			c = oracleCompareValues(av, bv)
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

// oracleSortValue 返回 (值, 是否存在)。数值以 float64 表达；字符串/bool 以
// 字符串表达。数值与字符串混排时数值在前（数值类 0 < 字符串类 1）。
func oracleSortValue(r oracleRow, field string) (struct {
	num bool
	n   float64
	s   string
}, bool) {
	var out struct {
		num bool
		n   float64
		s   string
	}
	switch field {
	case "ref.kind":
		out.s = r.ref.Kind
	case "ref.scope_kind":
		out.s = r.ref.ScopeKind
	case "ref.scope_value":
		out.s = r.ref.ScopeValue
	case "ref.snapshot":
		out.s = r.ref.Snapshot
	case "ref.hash":
		out.s = r.ref.Hash
	case "ref.window.sample_start":
		if r.ref.Window == nil {
			return out, false
		}
		out.num = true
		if r.ref.Window.AllTime {
			out.n = -1 // 全时间窗排在组内最前
		} else {
			out.n = float64(r.ref.Window.SampleStart)
		}
	case "ref.window.sample_end":
		if r.ref.Window == nil {
			return out, false
		}
		out.num = true
		if r.ref.Window.AllTime {
			out.n = -1
		} else {
			out.n = float64(r.ref.Window.SampleEnd)
		}
	default:
		v, ok := r.payload[field]
		if !ok {
			return out, false
		}
		switch t := v.(type) {
		case string:
			out.s = t
		case bool:
			out.s = fmt.Sprint(t)
		default:
			if f, ok := oracleToFloat(v); ok {
				out.num = true
				out.n = f
			} else {
				out.s = fmt.Sprint(v)
			}
		}
	}
	return out, true
}

func oracleCompareValues(a, b struct {
	num bool
	n   float64
	s   string
}) int {
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

// assertResultMatchesOracle 对拍：res.TotalMatches == len(oracle)，res.Rows 是
// oracle 的 [offset, offset+len) 页，且逐行 Ref/Freshness/Payload 全等。
func assertResultMatchesOracle(t *testing.T, tag string, res QueryResult, oracle []oracleRow, offset int) {
	t.Helper()
	if res.TotalMatches != len(oracle) {
		t.Fatalf("%s: TotalMatches=%d, oracle 命中=%d", tag, res.TotalMatches, len(oracle))
	}
	end := offset + len(res.Rows)
	if end > len(oracle) {
		t.Fatalf("%s: 引擎返回 %d 行，oracle 只剩 %d 行（offset=%d）", tag, len(res.Rows), len(oracle)-offset, offset)
	}
	for i, row := range res.Rows {
		o := oracle[offset+i]
		if row.Ref != o.canonical {
			t.Fatalf("%s: 第 %d 行 ref 不符\n 引擎: %s\noracle: %s", tag, i, row.Ref, o.canonical)
		}
		if row.Freshness != o.freshness {
			t.Fatalf("%s: 第 %d 行 freshness 引擎=%q oracle=%q（ref=%s）", tag, i, row.Freshness, o.freshness, row.Ref)
		}
		if !payloadEqual(row.Payload, o.payload) {
			t.Fatalf("%s: 第 %d 行 payload 不符（ref=%s）\n 引擎: %v\noracle: %v", tag, i, row.Ref, row.Payload, o.payload)
		}
	}
}

func payloadEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if fmt.Sprint(v) != fmt.Sprint(bv) {
			return false
		}
	}
	return true
}

// randOf 返回可重放的确定性 PRNG（fuzz 对拍用，种子固定）。
func randOf(seed uint64) *rand.Rand { return rand.New(rand.NewSource(int64(seed))) }

// waitGenChange 轮询等待引擎消化变更流并换代（push→订阅 goroutine→指针交换
// 是异步的；2s 超时视为流断）。
func waitGenChange(t *testing.T, e *Engine, oldGen string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		res, err := e.Query(context.Background(), RefQuery{Limit: 1})
		if err != nil {
			t.Fatalf("waitGenChange query: %v", err)
		}
		if res.Snapshot != oldGen {
			return res.Snapshot
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待换代超时：仍为 %s", oldGen)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// waitRowFreshness 轮询等待单行查询命中期望 freshness。
func waitRowFreshness(t *testing.T, e *Engine, q RefQuery, want string) QueryResult {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		res, err := e.Query(context.Background(), q)
		if err != nil {
			t.Fatalf("waitRowFreshness query: %v", err)
		}
		if len(res.Rows) == 1 && res.Rows[0].Freshness == want {
			return res
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待行状态 %q 超时（当前 %+v）", want, res.Rows)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
