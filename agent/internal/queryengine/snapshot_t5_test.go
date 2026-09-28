package queryengine

// snapshot_t5_test.go — T5 快照一致性：单 query 全程单代视图（查询中并发
// ApplyChange 不影响已开始的结果集）；结果 snapshot 字段可重放（设计 §6.2）。

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestT5ReplayableSnapshot(t *testing.T) {
	fixture := t1Fixture()
	e, st := newTestEngine(t, fixture...)

	q := RefQuery{Kinds: KindPredicate{Kinds: []string{"dom", "mom"}}}
	res1, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("query1: %v", err)
	}
	res2, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("query2: %v", err)
	}
	if res1.Snapshot == "" || res1.Snapshot != res2.Snapshot {
		t.Fatalf("同代两次查询 snapshot 应一致非空: %q vs %q", res1.Snapshot, res2.Snapshot)
	}
	if res1.TotalMatches != res2.TotalMatches || len(res1.Rows) != len(res2.Rows) {
		t.Fatalf("同代两次查询结果集应一致")
	}
	for i := range res1.Rows {
		if res1.Rows[i].Ref != res2.Rows[i].Ref {
			t.Fatalf("同代两次查询顺序应一致（第 %d 行）", i)
		}
	}

	// 换代后：snapshot 标识变化，新行可见（push→订阅 goroutine→换指针异步，轮询等换代）
	st.push(t, MaterializedChange{Op: MaterializedAdded, Row: mrow(allRef("tim", "project", "P1", "obs_3", 31), FreshnessCurrent, nil)})
	waitGenChange(t, e, res1.Snapshot)
	res3, err := e.Query(context.Background(), q)
	if err != nil {
		t.Fatalf("query3: %v", err)
	}
	if res3.Snapshot == res1.Snapshot {
		t.Fatalf("换代后 snapshot 应变化: %q", res3.Snapshot)
	}
	// q 不含 tim，命中数不变；但游标必须绑定新代（旧游标失效）
	_, err = e.Query(context.Background(), RefQuery{Kinds: q.Kinds, Cursor: res1.NextCursor})
	_ = err // res1 可能无 next cursor（命中数 < limit 时为空）——游标代绑定在下面单测
}

func TestT5StaleCursorAcrossGeneration(t *testing.T) {
	rows := make([]MaterializedRow, 0, 60)
	for i := 0; i < 60; i++ {
		rows = append(rows, mrow(allRef("tom", "track", fmt.Sprintf("T%02d", i), "obs_1", 100+i), FreshnessCurrent, nil))
	}
	e, st := newTestEngine(t, rows...)
	q := RefQuery{Limit: 10}
	res1, err := e.Query(context.Background(), q)
	if err != nil || res1.NextCursor == "" {
		t.Fatalf("page1: %v (cursor=%q)", err, res1.NextCursor)
	}

	// 换代（加一行）后旧代游标不可用（先等引擎消化变更）
	st.push(t, MaterializedChange{Op: MaterializedAdded, Row: mrow(allRef("tim", "project", "P1", "obs_2", 200), FreshnessCurrent, nil)})
	waitGenChange(t, e, res1.Snapshot)
	_, err = e.Query(context.Background(), RefQuery{Limit: 10, Cursor: res1.NextCursor})
	if err == nil {
		t.Fatalf("跨代游标应报错（ErrStaleCursor）")
	}
}

func TestT5ConcurrentApplyChangeDuringQueries(t *testing.T) {
	fixture := t1Fixture()
	e, st := newTestEngine(t, fixture...)

	// 记录每代的全量行集（oracle 对照样本）
	gens := [][]MaterializedRow{append([]MaterializedRow(nil), fixture...)}
	for i := 0; i < 6; i++ {
		st.push(t, MaterializedChange{Op: MaterializedAdded, Row: mrow(
			allRef("tim", "project", fmt.Sprintf("P%d", i), fmt.Sprintf("obs_%d", i+2), 300+i), FreshnessCurrent, nil)})
		gens = append(gens, append([]MaterializedRow(nil), st.snapshotRows()...))
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			q := RefQuery{Limit: MaxQueryLimit, Sort: []SortKey{{Field: "ref.kind"}, {Field: "ref.scope_value"}}}
			for round := 0; round < 20; round++ {
				res, err := e.Query(context.Background(), q)
				if err != nil {
					errs <- fmt.Errorf("goroutine %d round %d: %w", seed, round, err)
					return
				}
				// 结果必须与"某一代"的 oracle 完全一致（单代视图）
				matched := false
				for _, gen := range gens {
					oracle := oracleFilter(oracleRowsOf(gen), q)
					if res.TotalMatches == len(oracle) && rowsEqual(res, oracle) {
						matched = true
						break
					}
				}
				if !matched {
					errs <- fmt.Errorf("goroutine %d round %d: 结果不匹配任何已知代（total=%d snapshot=%q）", seed, round, res.TotalMatches, res.Snapshot)
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func rowsEqual(res QueryResult, oracle []oracleRow) bool {
	if len(res.Rows) != len(oracle) {
		return false
	}
	for i := range res.Rows {
		if res.Rows[i].Ref != oracle[i].canonical || res.Rows[i].Freshness != oracle[i].freshness {
			return false
		}
	}
	return true
}
