package queryengine

// route_t8_test.go — T8 路由表驱动：每谓词类 → 期望路由目标（含降级触发条件）
// 与期望 cost_class/degraded 标注；路由表（九投影+DAD 裁决表）完整性；
// Catalog 一致性（注册局部索引字段并入）；diff/expand 路由入口。
// 附：identity diff 冒烟（完整 T7 归 IMPL-D）与 Expand 解析路径。

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// ---- fake PayloadIndex（测试侧；过滤逻辑复用 test 包 oracle，与引擎无共享）----

type fakePayloadIndex struct {
	kind     string
	fields   []PayloadFieldSpec
	mu       sync.Mutex
	rows     []oracleRow
	searches int
}

func newFakePayloadIndex(kind string, rows []MaterializedRow) *fakePayloadIndex {
	kindRows := make([]MaterializedRow, 0, len(rows))
	for _, r := range rows {
		if r.Ref.Kind == kind {
			kindRows = append(kindRows, r)
		}
	}
	return &fakePayloadIndex{kind: kind, rows: oracleRowsOf(kindRows), fields: []PayloadFieldSpec{
		{Field: kind + ".demo.number", Type: "number", Ops: []PayloadOp{PayloadEq, PayloadNe, PayloadGt, PayloadGe, PayloadLt, PayloadLe}, Sortable: true},
		{Field: kind + ".demo.text", Type: "string", Ops: []PayloadOp{PayloadEq, PayloadNe, PayloadIn}, Sortable: true},
	}}
}

func (f *fakePayloadIndex) Kind() string                            { return f.kind }
func (f *fakePayloadIndex) Fields() []PayloadFieldSpec              { return f.fields }
func (f *fakePayloadIndex) ApplyChange(ch MaterializedChange) error { return nil }

func (f *fakePayloadIndex) Search(ctx context.Context, seg SegmentConstraints, pay []PayloadCondition, sortKeys []SortKey, limit int, cursor string) ([]ResultRow, string, error) {
	f.mu.Lock()
	f.searches++
	defer f.mu.Unlock()
	q := RefQuery{Scope: seg.Scope, Window: seg.Window, Snapshot: seg.Snapshot, Hash: seg.Hash, Payload: pay, Sort: sortKeys}
	matched := oracleFilter(f.rows, q)
	offset := 0
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil {
			return nil, "", err
		}
		offset = n
	}
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	rows := make([]ResultRow, 0, end-offset)
	for _, o := range matched[offset:end] {
		rows = append(rows, ResultRow{Ref: o.canonical, Freshness: o.freshness, Payload: o.payload})
	}
	next := ""
	if end < len(matched) {
		next = strconv.Itoa(end)
	}
	return rows, next, nil
}

// ---- 路由表完整性 ----

func TestT8KindRouteTableCoversAllRegisteredKinds(t *testing.T) {
	table := KindRouteTable()
	registered := agentprotocol.RegisteredRefKinds()
	if len(table) != len(registered) {
		t.Fatalf("路由表条目 %d != 注册 kind %d（%v）", len(table), len(registered), registered)
	}
	seen := map[string]bool{}
	for _, entry := range table {
		if seen[entry.Kind] {
			t.Fatalf("路由表 kind 重复: %s", entry.Kind)
		}
		seen[entry.Kind] = true
		if entry.CentralSeed == "" {
			t.Fatalf("kind %s 缺中央段索引种子锚", entry.Kind)
		}
	}
	for _, kind := range registered {
		if !seen[kind] {
			t.Fatalf("注册 kind %q 缺路由表条目", kind)
		}
	}
	// 逐 kind 建议字段命名法：kind 前缀三段命名（§3.2），唯 fci / dad.l2_probe
	// 两个历史字段名按设计 §3.3 原文保留
	for _, entry := range table {
		for _, f := range entry.PayloadFields {
			if !strings.HasPrefix(f.Field, entry.Kind+".") &&
				!strings.HasPrefix(f.Field, "fci.") &&
				!strings.HasPrefix(f.Field, "dad.l2_probe.") {
				t.Fatalf("kind %s 的建议字段 %q 违反 kind 前缀命名法", entry.Kind, f.Field)
			}
			if f.Sortable && len(f.Ops) == 0 {
				t.Fatalf("字段 %q 可排序但无算子声明", f.Field)
			}
		}
	}
	// 设计 §3.3 特记锚点抽查
	notes := map[string]string{}
	for _, entry := range table {
		notes[entry.Kind] = strings.Join(entry.Notes, ";")
	}
	if !strings.Contains(notes["fxm"], "实例") {
		t.Fatalf("fxm 特记应含实例身份语义注记: %q", notes["fxm"])
	}
	if !strings.Contains(notes["mom"], "observation_id") {
		t.Fatalf("mom 特记应含 snapshot=observation_id 承载注记: %q", notes["mom"])
	}
	if !strings.Contains(notes["rlm"], "G2") {
		t.Fatalf("rlm 特记应含 E8 stale 风险归 G2 注记: %q", notes["rlm"])
	}
	if !strings.Contains(notes["dad.l3"], "legacy") {
		t.Fatalf("dad.l3 特记应含迁移前 legacy 翻译条目注记: %q", notes["dad.l3"])
	}
	if len(table) != 13 {
		t.Fatalf("九投影+DAD 裁决表应为 13 条（9 投影 kind + 4 DAD kind），实际 %d", len(table))
	}
}

// ---- 谓词类 → 路由目标（§3.2 表驱动锁定）----

func TestT8RouteQueryDecisions(t *testing.T) {
	momIndex := newFakePayloadIndex("mom", t1Fixture())
	rlmIndex := newFakePayloadIndex("rlm", t1Fixture())
	withIndex := NewPayloadIndexRegistry(momIndex, rlmIndex)
	noIndex := NewPayloadIndexRegistry()

	cases := []struct {
		tag  string
		q    RefQuery
		reg  PayloadIndexRegistry
		want []RouteDecision
	}{
		{tag: "R1 仅 kind", q: RefQuery{Kinds: KindPredicate{Kinds: []string{"mom"}}}, reg: noIndex,
			want: []RouteDecision{{Class: "R1", Target: RouteCentral, CostClass: CostClassIndex, Degraded: ""}}},
		{tag: "R2 kind+scope", q: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"mom"}},
			Scope: &ScopePredicate{Kind: "track", Values: []string{"T3"}, ValueSet: true},
		}, reg: noIndex, want: []RouteDecision{{Class: "R2", Target: RouteCentral, CostClass: CostClassIndex}}},
		{tag: "R3 kind+scope+window", q: RefQuery{
			Kinds:  KindPredicate{Kinds: []string{"dom"}},
			Scope:  &ScopePredicate{Kind: "track", Values: []string{"T3"}, ValueSet: true},
			Window: &WindowPredicate{Relation: WindowIntersects, SampleStart: 0, SampleEnd: 1000},
		}, reg: noIndex, want: []RouteDecision{{Class: "R3", Target: RouteCentral, CostClass: CostClassIndex}}},
		{tag: "R4 snapshot/hash 谓词", q: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}}, Hash: &HashPredicate{Any: true},
		}, reg: noIndex, want: []RouteDecision{{Class: "R4", Target: RouteCentral, CostClass: CostClassIndex}}},
		{tag: "R5 无局部索引→降级 compile", q: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"mom"}},
			Payload: []PayloadCondition{{Field: "mom.masking.candidates.median_margin_db", Op: PayloadLt, Value: 6}},
		}, reg: noIndex, want: []RouteDecision{{Class: "R5", Target: RouteDegraded, CostClass: CostClassCompile, Degraded: DegradedPayloadIndexUnavailable}}},
		{tag: "R5 有局部索引→委托 index", q: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"mom"}},
			Payload: []PayloadCondition{{Field: "mom.masking.candidates.median_margin_db", Op: PayloadLt, Value: 6}},
		}, reg: withIndex, want: []RouteDecision{{Class: "R5", Target: RoutePayloadIndex, CostClass: CostClassIndex, Degraded: ""}}},
		{tag: "R6 载荷排序无索引→降级", q: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"rlm"}},
			Sort:  []SortKey{{Field: "rlm.summary.integrated_lufs", Dir: SortDesc}},
		}, reg: noIndex, want: []RouteDecision{{Class: "R6", Target: RouteDegraded, CostClass: CostClassCompile, Degraded: DegradedPayloadIndexUnavailable}}},
		{tag: "R7 多 kind 混合载荷→按 kind 拆分各自路由", q: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"mom", "rlm", "dom"}},
			Payload: []PayloadCondition{{Field: "mom.masking.candidates.median_margin_db", Op: PayloadLt, Value: 6}},
		}, reg: withIndex, want: []RouteDecision{
			{Class: "R5", Target: RouteDegraded, CostClass: CostClassCompile, Degraded: DegradedPayloadIndexUnavailable}, // dom 无索引
			{Class: "R5", Target: RoutePayloadIndex, CostClass: CostClassIndex, Degraded: ""},                            // mom
			{Class: "R5", Target: RoutePayloadIndex, CostClass: CostClassIndex, Degraded: ""},                            // rlm
		}},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			got := RouteQuery(tc.q, tc.reg)
			if len(got) != len(tc.want) {
				t.Fatalf("路由决策数 %d != %d: %+v", len(got), len(tc.want), got)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("第 %d 个决策不符:\n got %+v\nwant %+v", i, got[i], tc.want[i])
				}
			}
		})
	}

	// 未注册 kind 在路由入口即拒
	if got := RouteQuery(RefQuery{Kinds: KindPredicate{Kinds: []string{"nope"}}}, noIndex); got != nil {
		t.Fatalf("未注册 kind 应返回 nil 决策，实际 %+v", got)
	}
}

func TestT8RouteExpandAndDiff(t *testing.T) {
	if d := RouteExpand(ExpandHandle); d.Target != RouteResolve || d.CostClass != CostClassIndex || d.Class != "R8" {
		t.Fatalf("R8 handle 路由不符: %+v", d)
	}
	if d := RouteExpand(ExpandSummary); d.CostClass != CostClassCompile {
		t.Fatalf("R8 summary 应 compile: %+v", d)
	}
	if d := RouteDiff(DiffIdentity); d.Class != "R9" || d.Target != RouteDiffIdentity || d.CostClass != CostClassIndex {
		t.Fatalf("R9 路由不符: %+v", d)
	}
	if d := RouteDiff(DiffContent); d.Class != "R10" || d.Target != RouteDiffDelegated || d.CostClass != CostClassCompile {
		t.Fatalf("R10 路由不符: %+v", d)
	}
}

// ---- Catalog ----

func TestT8CatalogConsistency(t *testing.T) {
	st := newFakeStore(t1Fixture()...)
	momIndex := newFakePayloadIndex("mom", t1Fixture())
	e := NewEngine(st, NewPayloadIndexRegistry(momIndex))

	cat := e.Catalog()
	if len(cat.Kinds) != 13 {
		t.Fatalf("Catalog kinds=%d, want 13", len(cat.Kinds))
	}
	if len(cat.SortAliases) == 0 {
		t.Fatalf("Catalog 应暴露 ref 排序别名")
	}
	for _, k := range cat.Kinds {
		if k.Kind == "mom" {
			// 建议字段（路由表）+ 局部索引字段（注册表）都应出现
			hasSuggest, hasIndex := false, false
			for _, f := range k.PayloadFields {
				if f.Field == "mom.masking.candidates.median_margin_db" {
					hasSuggest = true
				}
				if f.Field == "mom.demo.number" {
					hasIndex = true
				}
			}
			if !hasSuggest || !hasIndex {
				t.Fatalf("mom catalog 应合并建议字段与索引字段: %+v", k.PayloadFields)
			}
		}
		if k.Kind == "dom" {
			found := false
			for _, f := range k.PayloadFields {
				if f.Field == "dom.peak_structure.readiness" {
					found = true
				}
			}
			if !found {
				t.Fatalf("dom catalog 应含具体维度 readiness 字段: %+v", k.PayloadFields)
			}
		}
	}
}

// ---- identity diff 冒烟（完整 T7 锁定归 IMPL-D）----

func TestT8DiffIdentitySmoke(t *testing.T) {
	e, _ := newTestEngine(t, t1Fixture()...)
	a1 := winRef("dom", "track", "T3", 0, 1000, "obs_1", 21)
	a2 := winRef("dom", "track", "T3", 1000, 2000, "obs_1", 22)
	a2b := winRef("dom", "track", "T3", 1000, 2000, "obs_1", 23) // 同坐标不同 hash
	a3 := allRef("mom", "project", "P1", "obs_1", 24)
	a4 := winRef("fxm", "track", "T3", 0, 1000, "obs_1", 25)

	rep, err := e.DiffEvidence(context.Background(), DiffRequest{
		Base:  SnapshotRefSet{Refs: []string{canon(a1), canon(a2), canon(a3)}},
		Head:  SnapshotRefSet{Refs: []string{canon(a2b), canon(a3), canon(a4)}},
		Depth: DiffIdentity,
	})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(rep.Added) != 1 || rep.Added[0] != canon(a4) {
		t.Fatalf("Added 不符: %v", rep.Added)
	}
	if len(rep.Removed) != 1 || rep.Removed[0] != canon(a1) {
		t.Fatalf("Removed 不符: %v", rep.Removed)
	}
	if len(rep.Changed) != 1 || rep.Changed[0].Base != canon(a2) || rep.Changed[0].Head != canon(a2b) {
		t.Fatalf("Changed 不符: %+v", rep.Changed)
	}
	if rep.UnchangedCount != 1 {
		t.Fatalf("UnchangedCount=%d, want 1", rep.UnchangedCount)
	}
	if rep.CostClass != CostClassIndex {
		t.Fatalf("identity diff 应 index: %q", rep.CostClass)
	}
	if rep.Delegated != nil {
		t.Fatalf("identity diff 不应有委托产物")
	}

	// revision 选择器路径：obs_1 全量 vs obs_2 全量
	rep, err = e.DiffEvidence(context.Background(), DiffRequest{
		Base:  SnapshotRefSet{Revision: "obs_1"},
		Head:  SnapshotRefSet{Revision: "obs_2"},
		Depth: DiffIdentity,
	})
	if err != nil {
		t.Fatalf("diff by revision: %v", err)
	}
	if len(rep.Added) != 0 {
		t.Fatalf("obs_2 ⊂ obs_1 时不应有 Added: %+v", rep)
	}
	// obs_1 的 r7 与 obs_2 的 r8 同坐标（mom/project/P1/t=all）不同 hash → Changed=1
	if len(rep.Changed) != 1 {
		t.Fatalf("obs_1 vs obs_2 应恰有 1 对 Changed（r7→r8）: %+v", rep.Changed)
	}
	if len(rep.Removed) != 11 || rep.UnchangedCount != 0 {
		t.Fatalf("obs_1(12 行) vs obs_2(1 行): Removed=%d Unchanged=%d, want 11/0", len(rep.Removed), rep.UnchangedCount)
	}

	// Scope 缩小差分面
	rep, err = e.DiffEvidence(context.Background(), DiffRequest{
		Base:  SnapshotRefSet{Revision: "obs_1"},
		Head:  SnapshotRefSet{Revision: "obs_2"},
		Scope: &RefQuery{Kinds: KindPredicate{Kinds: []string{"mom"}}},
		Depth: DiffIdentity,
	})
	if err != nil {
		t.Fatalf("diff scoped: %v", err)
	}
	// mom 域：obs_1 的 r7 与 obs_2 的 r8 同坐标（mom/project/P1/t=all）不同 hash → Changed
	if len(rep.Changed) != 1 || len(rep.Removed) != 0 || rep.UnchangedCount != 0 {
		t.Fatalf("mom 域差分: Changed=%d Removed=%d Unchanged=%d", len(rep.Changed), len(rep.Removed), rep.UnchangedCount)
	}

	// content 级骨架：IMPL-D 接线前明确报未实现
	_, err = e.DiffEvidence(context.Background(), DiffRequest{
		Base: SnapshotRefSet{Refs: []string{canon(a1)}}, Head: SnapshotRefSet{Refs: []string{canon(a2)}}, Depth: DiffContent,
	})
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("content 级应报 ErrNotImplemented（IMPL-D），实际 %v", err)
	}

	// 空 set + 无 revision → 错误
	_, err = e.DiffEvidence(context.Background(), DiffRequest{Depth: DiffIdentity})
	if err == nil {
		t.Fatalf("空侧集应报错")
	}
}

// ---- Expand 解析路径 ----

func TestT8ExpandResolvedPath(t *testing.T) {
	target := winRef("dom", "track", "T3", 0, 1000, "obs_1", 41)
	st := newFakeStore(t1Fixture()...)
	handle := "evidence://sha256-" + hex16(41)
	st.content[handle] = []byte(fmt.Sprintf(
		`{"readiness":"ready","raw_pcm":"AAAA","nested":{"file_path":"secret","keep":1}}`))
	e := NewEngine(st, nil)

	out, err := e.Expand(context.Background(), []string{canon(target)}, ExpandOptions{})
	if err != nil {
		t.Fatalf("expand handle: %v", err)
	}
	if out[0].Handle != handle || out[0].Bytes == 0 || out[0].Freshness != FreshnessCurrent {
		t.Fatalf("handle 展开不符: %+v", out[0])
	}
	if out[0].Summary != nil {
		t.Fatalf("默认深度不应带 summary")
	}

	out, err = e.Expand(context.Background(), []string{canon(target)}, ExpandOptions{Depth: ExpandSummary, MaxBytes: 4096})
	if err != nil {
		t.Fatalf("expand summary: %v", err)
	}
	s := out[0].Summary
	if s == nil {
		t.Fatalf("summary 缺失")
	}
	if _, bad := s["raw_pcm"]; bad {
		t.Fatalf("summary 应过滤 raw_* 禁键（CCB freeStateForbiddenField 同款规则）")
	}
	nested, _ := s["nested"].(map[string]any)
	if nested == nil {
		nested = map[string]any{}
	}
	if _, bad := nested["file_path"]; bad {
		t.Fatalf("summary 应过滤 file_path 禁键")
	}
	if s["readiness"] != "ready" {
		t.Fatalf("summary 保留字段丢失: %v", s)
	}
	if out[0].Truncated {
		t.Fatalf("预算内不应截断")
	}

	// 小预算 → 截断标记
	out, err = e.Expand(context.Background(), []string{canon(target)}, ExpandOptions{Depth: ExpandSummary, MaxBytes: 16})
	if err != nil {
		t.Fatalf("expand truncated: %v", err)
	}
	if !out[0].Truncated {
		t.Fatalf("超预算应置 Truncated")
	}
}
