package materialize

// store_onpath_test.go — MAT-E 读端切换的存储侧语义（红先行）：
//   - CurrentPrecomputableRow：current 才命中（stale/material_reuse 如实 miss）；
//   - Upsert 句柄保留：内容身份同源（hash+payload 同值）的无句柄行（影子轮
//     重算产物）不得冲掉既有 evidence 句柄（读端回填句柄的存活前提）；
//     内容变化时旧句柄必须让位（旧句柄指向旧内容，不得跨内容存活）。

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

func matERef(hash string) agentprotocol.Ref {
	return agentprotocol.Ref{
		Kind: "dom", ScopeKind: "track", ScopeValue: "T1",
		Window: allTimeWindow(), Snapshot: snapshotTokenCurrent, Hash: hash,
	}
}

func TestCurrentPrecomputableRowOnlyCurrentHits(t *testing.T) {
	s := NewStore()
	if _, ok := s.CurrentPrecomputableRow("dom", "track", "T1"); ok {
		t.Fatalf("空库不得命中")
	}
	if err := s.Upsert([]Row{{Ref: matERef("sha256:aaaaaaaaaaaaaaaa"), Payload: map[string]any{"status": "ready"}}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.CurrentPrecomputableRow("dom", "track", "T1"); !ok {
		t.Fatalf("current 行应命中")
	}
	if _, ok := s.CurrentPrecomputableRow("dom", "track", "T9"); ok {
		t.Fatalf("其它 scope 不得命中")
	}
	if _, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeKind: "track", ScopeValues: []string{"T1"}}, "chg1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.CurrentPrecomputableRow("dom", "track", "T1"); ok {
		t.Fatalf("stale 行不得命中（读侧零推断：miss 如实）")
	}
}

func TestUpsertPreservesHandleOnUnchangedContent(t *testing.T) {
	s := NewStore()
	ref := matERef("sha256:aaaaaaaaaaaaaaaa")
	if err := s.Upsert([]Row{{Ref: ref, Payload: map[string]any{"status": "ready"}, Handle: "evidence://readside"}}); err != nil {
		t.Fatal(err)
	}

	// 影子轮重算行：内容身份同源、无句柄 → 句柄保留（不冲刷）+ unchanged 计数。
	if err := s.Upsert([]Row{{Ref: ref, Payload: map[string]any{"status": "ready"}}}); err != nil {
		t.Fatal(err)
	}
	row, ok := s.CurrentPrecomputableRow("dom", "track", "T1")
	if !ok {
		t.Fatalf("行应仍在")
	}
	if row.Handle != "evidence://readside" {
		t.Fatalf("内容同源行不得冲掉读端回填句柄: %q", row.Handle)
	}
	if m := s.Metrics(); m.PerKind["dom"].RowsUnchanged < 1 {
		t.Fatalf("内容同源+句柄保留应计 unchanged（非 replaced）: %+v", m.PerKind["dom"])
	}

	// 内容变化行（无句柄）：旧句柄指向旧内容，必须让位。
	refChanged := matERef("sha256:bbbbbbbbbbbbbbbb")
	if err := s.Upsert([]Row{{Ref: refChanged, Payload: map[string]any{"status": "ready"}}}); err != nil {
		t.Fatal(err)
	}
	rowChanged, ok := s.CurrentPrecomputableRow("dom", "track", "T1")
	if !ok {
		t.Fatalf("行应仍在")
	}
	if rowChanged.Handle != "" {
		t.Fatalf("内容变化后旧句柄不得跨内容存活: %q", rowChanged.Handle)
	}
}
