package queryengine

// query_t1_test.go — T1 表驱动：每谓词类（R1-R6）≥3 例，含空集/单命中/全命中/
// 边界（window 半开区间端点、scope 前缀与精确并存的优先级）。断言 = 引擎结果
// ≡ oracle（含顺序）。设计 §6.2。

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func t1Fixture() []MaterializedRow {
	return []MaterializedRow{
		mrow(winRef("dom", "track", "T3", 0, 1000, "obs_1", 1), FreshnessCurrent, map[string]any{
			"dom.peak_structure.readiness": "ready", "dom.peak_structure.trust.confidence": 0.9,
		}),
		mrow(winRef("dom", "track", "T3", 1000, 2000, "obs_1", 2), FreshnessMaterialReuse, map[string]any{
			"dom.peak_structure.readiness": "partial",
		}),
		mrow(winRef("dom", "track", "T7", 2000, 3000, "obs_1", 3), FreshnessCurrent, map[string]any{
			"dom.peak_structure.readiness": "missing",
		}),
		mrow(winRef("dom", "bus", "master", 0, 100, "obs_1", 4), FreshnessCurrent, nil),
		mrow(winRef("dom", "bus", "master/sub1", 0, 100, "obs_1", 5), FreshnessCurrent, nil),
		mrow(winRef("fxm", "track", "T3", 0, 1000, "obs_1", 6), FreshnessCurrent, map[string]any{
			"fxm.ab.delta_lufs": -2.5, "fxm.ab.correlation": 0.82,
		}),
		mrow(winRef("fxm", "track", "T3", 1000, 2000, "obs_1", 7), FreshnessCurrent, map[string]any{
			"fxm.ab.delta_lufs": -6.25,
		}),
		mrow(allRef("mom", "project", "P1", "obs_1", 8), FreshnessCurrent, map[string]any{
			"mom.masking.candidates.median_margin_db": 4.2, "mom.frequency.conflict.band": "low-mid", "mom.static.level_rank": 3,
		}),
		mrow(allRef("mom", "project", "P1", "obs_2", 9), FreshnessStale, map[string]any{
			"mom.masking.candidates.median_margin_db": 7.8, "mom.frequency.conflict.band": "high", "mom.static.level_rank": 1,
		}),
		mrow(winRef("rlm", "track", "T7", 500, 1500, "obs_1", 10), FreshnessCurrent, map[string]any{
			"rlm.summary.integrated_lufs": -14.0, "rlm.summary.true_peak_db": -1.2, "rlm.row.target_delta_db": 2.5,
		}),
		mrow(allRef("tom", "track", "T3", "obs_1", 11), FreshnessCurrent, map[string]any{
			"tom.track.duration_seconds": 180.5, "tom.track.channel_count": 2,
		}),
		mrow(winRef("com", "track", "T3", 0, 1000, "obs_1", 12), FreshnessCurrent, map[string]any{
			"com.source.gr_crest": 6.5, "com.change_delta.peak_delta_db": -3.2,
		}),
		mrow(unCASRef("dom", "track", "T3", 0, 500, "obs_1"), FreshnessCurrent, nil), // #-
	}
}

type t1Case struct {
	tag       string
	query     RefQuery
	wantCost  string // 空 = 不校验
	wantDegr  string
	wantErr   error
	checkRows func(t *testing.T, res QueryResult, oracle []oracleRow)
}

func TestT1PredicateTableDriven(t *testing.T) {
	fixture := t1Fixture()
	oracleAll := oracleRowsOf(fixture)

	cases := []t1Case{
		// ---- R1 仅 kind ----
		{tag: "R1 单 kind", query: RefQuery{Kinds: KindPredicate{Kinds: []string{"dom"}}}, wantCost: CostClassIndex},
		{tag: "R1 空 kind=全部注册 kind", query: RefQuery{}, wantCost: CostClassIndex},
		{tag: "R1 多 kind", query: RefQuery{Kinds: KindPredicate{Kinds: []string{"dom", "fxm"}}}, wantCost: CostClassIndex},
		// ---- R2 kind+scope ----
		{tag: "R2 精确轨道集合", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Scope: &ScopePredicate{Kind: "track", Values: []string{"T3"}, ValueSet: true},
		}, wantCost: CostClassIndex},
		{tag: "R2 总线前缀子树", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Scope: &ScopePredicate{Kind: "bus", Prefix: "master", PrefixSet: true},
		}, wantCost: CostClassIndex},
		{tag: "R2 精确与前缀并设→精确优先", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Scope: &ScopePredicate{Kind: "track", Values: []string{"T7"}, Prefix: "master", ValueSet: true, PrefixSet: true},
		}, wantCost: CostClassIndex},
		{tag: "R2 scope_kind 不匹配→空集", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Scope: &ScopePredicate{Kind: "bus", Values: []string{"T3"}, ValueSet: true},
		}, wantCost: CostClassIndex},
		// ---- R3 kind+scope+window ----
		{tag: "R3 相交含端点相接 [2000,2000]", query: RefQuery{
			Kinds:  KindPredicate{Kinds: []string{"dom"}},
			Scope:  &ScopePredicate{Kind: "track", Values: []string{"T3", "T7"}, ValueSet: true},
			Window: &WindowPredicate{Relation: WindowIntersects, SampleStart: 2000, SampleEnd: 2000},
		}, wantCost: CostClassIndex},
		{tag: "R3 相交 [1500,2500]", query: RefQuery{
			Window: &WindowPredicate{Relation: WindowIntersects, SampleStart: 1500, SampleEnd: 2500},
		}, wantCost: CostClassIndex},
		{tag: "R3 contains [0,2000]", query: RefQuery{
			Window: &WindowPredicate{Relation: WindowContains, SampleStart: 0, SampleEnd: 2000},
		}, wantCost: CostClassIndex},
		{tag: "R3 within [1200,1400]", query: RefQuery{
			Window: &WindowPredicate{Relation: WindowWithin, SampleStart: 1200, SampleEnd: 1400},
		}, wantCost: CostClassIndex},
		{tag: "R3 AllTime 查询只匹配 t=all", query: RefQuery{
			Window: &WindowPredicate{AllTime: true},
		}, wantCost: CostClassIndex},
		{tag: "R3 AllTime ref 对有限窗：相交命中/包含不命中", query: RefQuery{
			Kinds:  KindPredicate{Kinds: []string{"mom"}},
			Window: &WindowPredicate{Relation: WindowContains, SampleStart: 0, SampleEnd: 1000},
		}, wantCost: CostClassIndex},
		{tag: "R3 显式点查 [0,0]", query: RefQuery{
			Kinds:  KindPredicate{Kinds: []string{"dom", "fxm", "com"}},
			Window: &WindowPredicate{Relation: WindowIntersects, SampleStart: 0, SampleEnd: 0},
		}, wantCost: CostClassIndex},
		// ---- R4 snapshot/hash ----
		{tag: "R4 hash 精确", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Hash:  &HashPredicate{SHA256: hex16(1)},
		}, wantCost: CostClassIndex},
		{tag: "R4 排除 #-", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Hash:  &HashPredicate{},
		}, wantCost: CostClassIndex},
		{tag: "R4 Any 含 #-", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}},
			Hash:  &HashPredicate{Any: true},
		}, wantCost: CostClassIndex},
		{tag: "R4 snapshot exact=obs_2", query: RefQuery{
			Snapshot: SnapshotPredicate{Mode: SnapshotExact, Revision: "obs_2"},
		}, wantCost: CostClassIndex},
		// ---- R5 段谓词+载荷过滤（无局部索引→降级 compile）----
		{tag: "R5 eq 字符串", query: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"dom"}},
			Payload: []PayloadCondition{{Field: "dom.peak_structure.readiness", Op: PayloadEq, Str: "ready"}},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R5 数值 lt（mom 建议字段）", query: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"mom"}},
			Payload: []PayloadCondition{{Field: "mom.masking.candidates.median_margin_db", Op: PayloadLt, Value: 5}},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R5 in 集合", query: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"dom"}},
			Payload: []PayloadCondition{{Field: "dom.peak_structure.readiness", Op: PayloadIn, Set: []string{"ready", "partial"}}},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R5 缺失字段→空集（非错误）", query: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"mom"}},
			Payload: []PayloadCondition{{Field: "mom.nonexistent", Op: PayloadEq, Str: "x"}},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R5 AND 语义两条件", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"fxm"}},
			Payload: []PayloadCondition{
				{Field: "fxm.ab.delta_lufs", Op: PayloadLt, Value: -2},
				{Field: "fxm.ab.correlation", Op: PayloadGt, Value: 0.5},
			},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R5 数值 ne", query: RefQuery{
			Kinds:   KindPredicate{Kinds: []string{"fxm"}},
			Payload: []PayloadCondition{{Field: "fxm.ab.delta_lufs", Op: PayloadNe, Value: -2.5}},
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		// ---- R6 载荷排序/topK（无局部索引→降级）----
		{tag: "R6 载荷排序 desc+limit", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"mom"}},
			Sort:  []SortKey{{Field: "mom.masking.candidates.median_margin_db", Dir: SortDesc}},
			Limit: 1,
		}, wantCost: CostClassCompile, wantDegr: DegradedPayloadIndexUnavailable},
		{tag: "R6 未声明排序字段→ErrUnsupportedSort", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"mom"}},
			Sort:  []SortKey{{Field: "mom.not.declared.anywhere"}},
		}, wantErr: ErrUnsupportedSort},
		{tag: "R6 ref 别名排序不受声明约束", query: RefQuery{
			Sort: []SortKey{{Field: "ref.window.sample_start", Dir: SortDesc}, {Field: "ref.kind"}},
		}, wantCost: CostClassIndex},
		// ---- 校验错误 ----
		{tag: "未注册 kind→ErrUnknownKind", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom", "nope"}},
		}, wantErr: ErrUnknownKind},
		{tag: "limit 越上限→ErrInvalidLimit", query: RefQuery{
			Kinds: KindPredicate{Kinds: []string{"dom"}}, Limit: MaxQueryLimit + 1,
		}, wantErr: ErrInvalidLimit},
		{tag: "snapshot exact 缺 revision→ErrInvalidSnapshot", query: RefQuery{
			Snapshot: SnapshotPredicate{Mode: SnapshotExact},
		}, wantErr: ErrInvalidSnapshot},
		{tag: "hash 非法 hex→ErrInvalidHash", query: RefQuery{
			Hash: &HashPredicate{SHA256: "XYZ"},
		}, wantErr: ErrInvalidHash},
		{tag: "hash Any+SHA256 并设矛盾→ErrInvalidHash", query: RefQuery{
			Hash: &HashPredicate{SHA256: hex16(1), Any: true},
		}, wantErr: ErrInvalidHash},
		{tag: "window start>end→ErrInvalidWindow", query: RefQuery{
			Window: &WindowPredicate{SampleStart: 100, SampleEnd: 50},
		}, wantErr: ErrInvalidWindow},
		{tag: "scope ValueSet 但 Values 空→ErrInvalidScope", query: RefQuery{
			Scope: &ScopePredicate{ValueSet: true},
		}, wantErr: ErrInvalidScope},
		{tag: "payload 非法算子→ErrInvalidPayloadCondition", query: RefQuery{
			Payload: []PayloadCondition{{Field: "mom.x", Op: PayloadOp("~")}},
		}, wantErr: ErrInvalidPayloadCondition},
	}

	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			e, _ := newTestEngine(t, fixture...)
			res, err := e.Query(context.Background(), tc.query)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("want error %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			if tc.wantCost != "" && res.CostClass != tc.wantCost {
				t.Fatalf("cost_class=%q, want %q", res.CostClass, tc.wantCost)
			}
			if res.Degraded != tc.wantDegr {
				t.Fatalf("degraded=%q, want %q", res.Degraded, tc.wantDegr)
			}
			// exact 快照的 oracle 输入 = 视图行集（§5.1.2：快照是视图选择，
			// 非行过滤——引擎与 oracle 同口径）
			oracleInput := oracleAll
			if tc.query.Snapshot.Mode == SnapshotExact {
				view := make([]oracleRow, 0, len(oracleAll))
				for _, r := range oracleAll {
					if r.ref.Snapshot == tc.query.Snapshot.Revision {
						view = append(view, r)
					}
				}
				oracleInput = view
			}
			oracle := oracleFilter(oracleInput, tc.query)
			assertResultMatchesOracle(t, tc.tag, res, oracle, 0)
			if tc.query.Limit == 0 && len(res.Rows) > DefaultQueryLimit {
				t.Fatalf("%s: 默认 limit 应为 %d，返回 %d 行", tc.tag, DefaultQueryLimit, len(res.Rows))
			}
		})
	}
}

// TestT1Pagination：分页翻三页 + TotalMatches 不受截断 + 游标绑定代。
func TestT1Pagination(t *testing.T) {
	fixture := t1Fixture()
	e, _ := newTestEngine(t, fixture...)
	q := RefQuery{Kinds: KindPredicate{Kinds: []string{"dom", "fxm", "mom", "rlm", "tom", "com"}}, Limit: 3}
	oracle := oracleFilter(oracleRowsOf(fixture), q)
	if len(oracle) != 13 {
		t.Fatalf("fixture 命中数=%d, want 13", len(oracle))
	}

	res, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	assertResultMatchesOracle(t, "page1", res, oracle, 0)
	if res.NextCursor == "" {
		t.Fatalf("page1 应有 next cursor")
	}

	offset := 0
	pages := 1
	for res.NextCursor != "" {
		q.Cursor = res.NextCursor
		res, err = e.Query(context.Background(), q)
		if err != nil {
			t.Fatalf("page%d: %v", pages+1, err)
		}
		offset += 3
		pages++
		assertResultMatchesOracle(t, fmt.Sprintf("page%d", pages), res, oracle, offset)
	}
	if pages != 5 { // 13 行 / 3 = 4 页满 + 1 页 1 行
		t.Fatalf("翻页数=%d, want 5", pages)
	}

	// 游标绑定快照代：无变化时旧游标仍可重放
	q.Cursor = ""
	res1, _ := e.Query(context.Background(), q)
	res2, err := e.Query(context.Background(), RefQuery{Kinds: q.Kinds, Limit: 3, Cursor: res1.NextCursor})
	if err != nil {
		t.Fatalf("重放游标: %v", err)
	}
	if res2.Snapshot != res1.Snapshot {
		t.Fatalf("同代两次查询 snapshot 应一致: %q vs %q", res2.Snapshot, res1.Snapshot)
	}
}

// TestT1DefaultLimit：Limit=0 → 默认 50（硬上限 500）。
func TestT1DefaultLimit(t *testing.T) {
	rows := make([]MaterializedRow, 0, 60)
	for i := 0; i < 60; i++ {
		rows = append(rows, mrow(allRef("tom", "track", fmt.Sprintf("T%02d", i), "obs_1", 100+i), FreshnessCurrent, nil))
	}
	e, _ := newTestEngine(t, rows...)
	res, err := e.Query(context.Background(), RefQuery{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Rows) != DefaultQueryLimit {
		t.Fatalf("默认 limit 行数=%d, want %d", len(res.Rows), DefaultQueryLimit)
	}
	if res.TotalMatches != 60 {
		t.Fatalf("TotalMatches=%d, want 60", res.TotalMatches)
	}
	if res.NextCursor == "" {
		t.Fatalf("应还有下一页")
	}
}

// TestT1PayloadIndexDelegation：注册局部索引后单 kind 载荷查询走委托路径
// （cost=index、degraded 空），结果仍 ≡ oracle。
func TestT1PayloadIndexDelegation(t *testing.T) {
	fixture := t1Fixture()
	st := newFakeStore(fixture...)
	idx := newFakePayloadIndex("mom", fixture)
	e := NewEngine(st, NewPayloadIndexRegistry(idx))
	q := RefQuery{
		Kinds:   KindPredicate{Kinds: []string{"mom"}},
		Payload: []PayloadCondition{{Field: "mom.masking.candidates.median_margin_db", Op: PayloadLt, Value: 5}},
		Sort:    []SortKey{{Field: "mom.masking.candidates.median_margin_db", Dir: SortDesc}},
	}
	res, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.CostClass != CostClassIndex {
		t.Fatalf("有局部索引应走 index: %q", res.CostClass)
	}
	if res.Degraded != "" {
		t.Fatalf("委托路径 degraded 应为空: %q", res.Degraded)
	}
	if idx.searches != 1 {
		t.Fatalf("应恰好委托一次 Search，实际 %d", idx.searches)
	}
	oracle := oracleFilter(oracleRowsOf(fixture), q)
	assertResultMatchesOracle(t, "delegation", res, oracle, 0)
}
