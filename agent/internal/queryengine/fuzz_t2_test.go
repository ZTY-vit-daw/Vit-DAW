package queryengine

// fuzz_t2_test.go — T2 Fuzz：随机谓词组合（固定 corpus seed 起步），合成行集，
// 断言引擎 ≡ oracle（含顺序与分页全链）。分歧即 bug，不是"改 oracle"。
// 常规 `go test` 只跑种子语料；`go test -fuzz=FuzzT2` 进入真 fuzz。

import (
	"context"
	"fmt"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

type t2Field struct {
	field string
	num   bool
}

var t2NumericFields = []t2Field{
	{"fxm.ab.delta_lufs", true},
	{"fxm.ab.correlation", true},
	{"mom.masking.candidates.median_margin_db", true},
	{"rlm.summary.integrated_lufs", true},
	{"tom.track.channel_count", true},
}
var t2StringFields = []t2Field{
	{"dom.peak_structure.readiness", false},
	{"mom.frequency.conflict.band", false},
}

func FuzzT2QueryOracle(f *testing.F) {
	f.Add(uint64(1), uint8(12))
	f.Add(uint64(2), uint8(5))
	f.Add(uint64(42), uint8(20))
	f.Fuzz(func(t *testing.T, seed uint64, nrows uint8) {
		r := randOf(seed)
		if nrows < 3 {
			nrows = 3
		}
		if nrows > 24 {
			nrows = 24
		}
		kinds := []string{"dom", "fxm", "mom", "rlm", "tom"}
		scopeKinds := []string{"track", "bus"}
		trackIDs := []string{"T1", "T2", "T3"}
		busIDs := []string{"master", "master/sub1"}
		snapshots := []string{"obs_1", "obs_2"}

		rows := make([]MaterializedRow, 0, nrows)
		for i := 0; i < int(nrows); i++ {
			kind := kinds[r.Intn(len(kinds))]
			scopeKind := scopeKinds[r.Intn(len(scopeKinds))]
			var scopeValue string
			if scopeKind == "track" {
				scopeValue = trackIDs[r.Intn(len(trackIDs))]
			} else {
				scopeValue = busIDs[r.Intn(len(busIDs))]
			}
			snapshot := snapshots[r.Intn(len(snapshots))]
			var ref agentprotocol.Ref
			if r.Intn(4) == 0 {
				ref = allRef(kind, scopeKind, scopeValue, snapshot, 1000+i)
			} else {
				start := int64(r.Intn(4)) * 500
				end := start + int64(r.Intn(4))*500
				ref = winRef(kind, scopeKind, scopeValue, start, end, snapshot, 1000+i)
			}
			if r.Intn(8) == 0 {
				ref = unCASRef(kind, scopeKind, scopeValue, 0, int64(r.Intn(2000)), snapshot)
			}
			payload := map[string]any{}
			if n := r.Intn(3); n > 0 {
				f := t2NumericFields[r.Intn(len(t2NumericFields))]
				payload[f.field] = float64(r.Intn(9)) - 4.5
			}
			if n := r.Intn(3); n > 0 {
				f := t2StringFields[r.Intn(len(t2StringFields))]
				payload[f.field] = []string{"ready", "partial", "missing", "low-mid", "high"}[r.Intn(5)]
			}
			fresh := []string{FreshnessCurrent, FreshnessMaterialReuse, FreshnessStale}[r.Intn(3)]
			rows = append(rows, mrow(ref, fresh, payload))
		}

		q := RefQuery{Limit: 1 + r.Intn(6)}
		if r.Intn(2) == 0 {
			q.Kinds = KindPredicate{Kinds: []string{kinds[r.Intn(len(kinds))]}}
		}
		if r.Intn(2) == 0 {
			p := &ScopePredicate{Kind: scopeKinds[r.Intn(len(scopeKinds))]}
			if r.Intn(2) == 0 {
				p.Values = []string{trackIDs[r.Intn(len(trackIDs))]}
				p.ValueSet = true
			} else {
				p.Prefix = "master"
				p.PrefixSet = true
			}
			q.Scope = p
		}
		if r.Intn(2) == 0 {
			q.Window = &WindowPredicate{
				Relation:    []WindowRelation{WindowIntersects, WindowContains, WindowWithin}[r.Intn(3)],
				SampleStart: int64(r.Intn(3)) * 500,
				SampleEnd:   int64(r.Intn(5)) * 500,
			}
			if q.Window.SampleStart > q.Window.SampleEnd {
				q.Window.SampleStart, q.Window.SampleEnd = q.Window.SampleEnd, q.Window.SampleStart
			}
		} else if r.Intn(6) == 0 {
			q.Window = &WindowPredicate{AllTime: true}
		}
		if n := r.Intn(3); n > 0 {
			q.Hash = &HashPredicate{Any: n == 2}
			if n == 1 {
				q.Hash.SHA256 = hex16(1000 + r.Intn(int(nrows)))
			}
		}
		if n := r.Intn(3); n > 0 {
			q.Freshness = []string{[]string{FreshnessCurrent, FreshnessStale}[r.Intn(2)]}
		}
		if n := r.Intn(3); n > 0 {
			if r.Intn(2) == 0 {
				f := t2NumericFields[r.Intn(len(t2NumericFields))]
				q.Payload = append(q.Payload, PayloadCondition{
					Field: f.field,
					Op:    []PayloadOp{PayloadEq, PayloadNe, PayloadGt, PayloadGe, PayloadLt, PayloadLe}[r.Intn(6)],
					Value: float64(r.Intn(9)) - 4.5,
				})
			} else {
				f := t2StringFields[r.Intn(len(t2StringFields))]
				q.Payload = append(q.Payload, PayloadCondition{
					Field: f.field,
					Op:    []PayloadOp{PayloadEq, PayloadNe, PayloadIn}[r.Intn(3)],
					Str:   []string{"ready", "partial", "missing"}[r.Intn(3)],
					Set:   []string{"ready", "high"},
				})
			}
		}
		if r.Intn(2) == 0 {
			q.Sort = []SortKey{{
				Field: []string{"ref.kind", "ref.scope_value", "ref.window.sample_start", "ref.snapshot"}[r.Intn(4)],
				Dir:   []SortDirection{SortAsc, SortDesc}[r.Intn(2)],
			}}
		}

		e, _ := newTestEngine(t, rows...)
		oracle := oracleFilter(oracleRowsOf(rows), q)
		offset := 0
		cursor := ""
		for {
			q.Cursor = cursor
			res, err := e.Query(context.Background(), q)
			if err != nil {
				t.Fatalf("seed=%d 查询失败: %v（查询=%+v）", seed, err, q)
			}
			if res.TotalMatches != len(oracle) {
				t.Fatalf("seed=%d TotalMatches=%d, oracle=%d", seed, res.TotalMatches, len(oracle))
			}
			if len(res.Rows) > 0 {
				assertResultMatchesOracle(t, fmt.Sprintf("seed=%d offset=%d", seed, offset), res, oracle, offset)
			}
			offset += len(res.Rows)
			if res.NextCursor == "" {
				break
			}
			cursor = res.NextCursor
		}
		if offset != len(oracle) {
			t.Fatalf("seed=%d 翻页累计 %d 行, oracle %d 行", seed, offset, len(oracle))
		}
	})
}
