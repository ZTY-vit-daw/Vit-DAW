package queryengine

// freshness_t6_test.go — T6 freshness 透传：stale 行默认谓词下保留且状态原样；
// 显式 freshness 谓词才过滤；无任何路径把 stale 升级为 current（F8/M1 纪律）。

import (
	"context"
	"testing"
)

func TestT6StaleRowsRetainedByDefault(t *testing.T) {
	fixture := t1Fixture() // r8(mom obs_2)=stale
	e, _ := newTestEngine(t, fixture...)

	res, err := e.Query(context.Background(), RefQuery{Kinds: KindPredicate{Kinds: []string{"mom"}}})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.TotalMatches != 2 {
		t.Fatalf("mom 两行（含 stale）都应保留: %d", res.TotalMatches)
	}
	staleSeen := false
	for _, row := range res.Rows {
		if row.Ref == canon(allRef("mom", "project", "P1", "obs_2", 9)) {
			staleSeen = true
			if row.Freshness != FreshnessStale {
				t.Fatalf("stale 行状态应原样透传: %q", row.Freshness)
			}
		}
	}
	if !staleSeen {
		t.Fatalf("stale 行未被默认谓词保留")
	}
}

func TestT6ExplicitFreshnessFilter(t *testing.T) {
	fixture := t1Fixture()
	e, _ := newTestEngine(t, fixture...)

	for _, tc := range []struct {
		freshness []string
		wantTotal int
	}{
		{[]string{FreshnessCurrent}, 11},
		{[]string{FreshnessStale}, 1},
		{[]string{FreshnessMaterialReuse}, 1},
		{[]string{FreshnessCurrent, FreshnessStale, FreshnessMaterialReuse}, 13},
		{[]string{"bogus"}, 0}, // 成员匹配语义：未知值=空集，不做值域校验
	} {
		res, err := e.Query(context.Background(), RefQuery{Freshness: tc.freshness})
		if err != nil {
			t.Fatalf("freshness=%v: %v", tc.freshness, err)
		}
		if res.TotalMatches != tc.wantTotal {
			t.Fatalf("freshness=%v: total=%d, want %d", tc.freshness, res.TotalMatches, tc.wantTotal)
		}
		oracle := oracleFilter(oracleRowsOf(fixture), RefQuery{Freshness: tc.freshness})
		assertResultMatchesOracle(t, "freshness filter", res, oracle, 0)
	}
}

func TestT6MarkedStaleNeverUpgraded(t *testing.T) {
	target := winRef("dom", "track", "T3", 0, 1000, "obs_1", 1)
	e, st := newTestEngine(t, t1Fixture()...)
	targetQuery := RefQuery{Kinds: KindPredicate{Kinds: []string{"dom"}}, Hash: &HashPredicate{SHA256: hex16(1)}}

	st.push(t, MaterializedChange{Op: MaterializedMarkedStale, Row: mrow(target, FreshnessStale, nil)})
	// marked_stale 只改状态不删行（§5.1.1）；push→订阅异步，轮询等状态落地
	res := waitRowFreshness(t, e, targetQuery, FreshnessStale)
	if res.TotalMatches != 1 || len(res.Rows) != 1 {
		t.Fatalf("marked_stale 行应保留: total=%d", res.TotalMatches)
	}
	if res.Rows[0].Freshness != FreshnessStale {
		t.Fatalf("marked_stale 后状态=%q, want stale", res.Rows[0].Freshness)
	}

	// 幂等重复 marked_stale 不复活、不改判
	st.push(t, MaterializedChange{Op: MaterializedMarkedStale, Row: mrow(target, FreshnessStale, nil)})
	res = waitRowFreshness(t, e, targetQuery, FreshnessStale)
	if res.Rows[0].Freshness != FreshnessStale {
		t.Fatalf("重复 marked_stale 后状态=%q, want stale", res.Rows[0].Freshness)
	}

	// 显式只要 current 时，stale 行被如实过滤（过滤≠删除：默认谓词仍可见）
	res3, err := e.Query(context.Background(), RefQuery{
		Kinds:     KindPredicate{Kinds: []string{"dom"}},
		Hash:      &HashPredicate{SHA256: hex16(1)},
		Freshness: []string{FreshnessCurrent},
	})
	if err != nil {
		t.Fatalf("query3: %v", err)
	}
	if res3.TotalMatches != 0 {
		t.Fatalf("stale 行在显式 current 谓词下应被过滤: %d", res3.TotalMatches)
	}
}
