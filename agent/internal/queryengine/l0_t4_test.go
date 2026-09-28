package queryengine

// l0_t4_test.go — T4 L0 依存：未注册 kind → ErrUnknownKind（非空结果语义）；
// legacy 翻译条目（dom_ 前缀）可查；opaque ref 不进索引但 Expand 可透传。
// 错误语义锁定（设计 §6.2）。

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

func TestT4UnknownKindIsErrorNotEmpty(t *testing.T) {
	e, _ := newTestEngine(t, t1Fixture()...)

	// 单独未注册 kind
	_, err := e.Query(context.Background(), RefQuery{Kinds: KindPredicate{Kinds: []string{"nope"}}})
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("kinds=[nope] 应报 ErrUnknownKind，实际 %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "nope") {
		t.Fatalf("错误应点名未知 kind: %v", err)
	}

	// 混入合法 kind 也整体拒绝
	_, err = e.Query(context.Background(), RefQuery{Kinds: KindPredicate{Kinds: []string{"dom", "zzz.k"}}})
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("混入未知 kind 应报 ErrUnknownKind，实际 %v", err)
	}

	// 大小写/字符集变体同样未注册
	for _, kind := range []string{"DOM", "dom ", "dóm"} {
		_, err = e.Query(context.Background(), RefQuery{Kinds: KindPredicate{Kinds: []string{kind}}})
		if !errors.Is(err, ErrUnknownKind) {
			t.Fatalf("kind %q 应报 ErrUnknownKind，实际 %v", kind, err)
		}
	}
}

func TestT4LegacyTranslationRowQueryable(t *testing.T) {
	// legacy dom_ 字面量经 REFSCHEMA 翻译后的中央索引行（翻译本体是 IMPL-B
	// bootstrap 的职责；本测试锁定"翻译条目按普通行可查"）。
	legacyRow := mrow(agentprotocol.Ref{
		Kind:       "dom",
		ScopeKind:  "projection",
		ScopeValue: "dom",
		Window:     &agentprotocol.TimeWindow{AllTime: true},
		Snapshot:   "obs_9",
		Hash:       "sha256:" + hex16(77),
	}, FreshnessCurrent, nil)
	parsed, err := agentprotocol.ParseRef("dom_" + hex16(77))
	if err != nil || parsed.State != agentprotocol.RefStateLegacy {
		t.Fatalf("refschema legacy 前提失效: state=%v err=%v", parsed.State, err)
	}
	if parsed.Legacy.TargetKind != "dom" {
		t.Fatalf("legacy 翻译 TargetKind=%q, want dom", parsed.Legacy.TargetKind)
	}

	e, _ := newTestEngine(t, append(t1Fixture(), legacyRow)...)
	res, err := e.Query(context.Background(), RefQuery{
		Kinds: KindPredicate{Kinds: []string{"dom"}},
		Hash:  &HashPredicate{SHA256: hex16(77)},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.TotalMatches != 1 || len(res.Rows) != 1 {
		t.Fatalf("legacy 翻译行应可按 kind+hash 查得 1 行，实际 total=%d rows=%d", res.TotalMatches, len(res.Rows))
	}
	if res.Rows[0].Ref != canon(legacyRow.Ref) {
		t.Fatalf("命中行不符: %s", res.Rows[0].Ref)
	}
}

func TestT4OpaqueRowsNotIndexedNoPanic(t *testing.T) {
	// 未注册 kind 行 + 缺 window 行：不可规范化 → 建索引跳过，不 panic，不可查得
	opaqueKind := agentprotocol.Ref{
		Kind: "zzz", ScopeKind: "track", ScopeValue: "T3",
		Window: &agentprotocol.TimeWindow{AllTime: true}, Snapshot: "obs_1", Hash: "sha256:" + hex16(88),
	}
	noWindow := agentprotocol.Ref{
		Kind: "dom", ScopeKind: "track", ScopeValue: "T3", Snapshot: "obs_1", Hash: "sha256:" + hex16(89),
	}
	e, _ := newTestEngine(t, append(t1Fixture(),
		mrow(opaqueKind, FreshnessCurrent, nil),
		mrow(noWindow, FreshnessCurrent, nil))...)

	res, err := e.Query(context.Background(), RefQuery{})
	if err != nil {
		t.Fatalf("全量查询不应因坏行失败: %v", err)
	}
	for _, row := range res.Rows {
		if strings.Contains(row.Ref, "zzz") || strings.Contains(row.Ref, "T3/obs_1#sha256:"+hex16(89)) {
			t.Fatalf("不可规范化行不应出现在结果中: %s", row.Ref)
		}
	}
	if res.TotalMatches != 13 {
		t.Fatalf("TotalMatches=%d, want 13（仅 t1Fixture 规范行）", res.TotalMatches)
	}
}

func TestT4ExpandOpaqueAndLegacyPassthrough(t *testing.T) {
	e, _ := newTestEngine(t, t1Fixture()...)

	out, err := e.Expand(context.Background(), []string{"weird-thing:xx", "dom_" + hex16(1)}, ExpandOptions{})
	if err != nil {
		t.Fatalf("Expand 对 opaque/legacy 应宽容透传: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("应返回 2 行，实际 %d", len(out))
	}
	if out[0].Ref != "weird-thing:xx" || out[0].Freshness != "opaque" {
		t.Fatalf("opaque 透传不符: %+v", out[0])
	}
	if out[1].Ref != "dom_"+hex16(1) || out[1].Freshness != "legacy" {
		t.Fatalf("legacy 透传不符: %+v", out[1])
	}
}
