package fxm

import (
	"testing"
)

// AGENT-SENDROUTE-SEMANTICS-1 验收：FXM 路由指纹纳入 send 拓扑 hash——
// 边变（connect/disconnect/amount）→指纹旋转（同路由前提破坏，缓存 stale）。

func TestSendTopologyHashRotatesOnEdgeChange(t *testing.T) {
	base := map[string]any{
		"sends": []any{
			map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
		},
		"dangling":  []any{},
		"fx_tracks": []any{},
	}
	baseHash := SendTopologyHash(base)
	if baseHash == "" {
		t.Fatal("hash must exist for a present topology")
	}

	// amount 变 → hash 变。
	amountChanged := map[string]any{
		"sends": []any{
			map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -9.0, "send_bus": 0, "return_bus": 1},
		},
		"dangling":  []any{},
		"fx_tracks": []any{},
	}
	if SendTopologyHash(amountChanged) == baseHash {
		t.Fatal("amount change must rotate the send topology hash")
	}

	// 边删除 → hash 变。
	disconnected := map[string]any{"sends": []any{}, "dangling": []any{}, "fx_tracks": []any{}}
	if SendTopologyHash(disconnected) == baseHash {
		t.Fatal("edge removal must rotate the send topology hash")
	}

	// 行序不影响 hash（canonical 排序）。
	reordered := map[string]any{
		"sends": []any{
			map[string]any{"source_track_id": "T2", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
			map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
		},
		"dangling":  []any{},
		"fx_tracks": []any{},
	}
	ordered := map[string]any{
		"sends": []any{
			map[string]any{"source_track_id": "T1", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
			map[string]any{"source_track_id": "T2", "fx_track_id": "FX1", "amount_db": -6.0, "send_bus": 0, "return_bus": 1},
		},
		"dangling":  []any{},
		"fx_tracks": []any{},
	}
	if SendTopologyHash(reordered) != SendTopologyHash(ordered) {
		t.Fatal("row order must not change the hash")
	}
}

func TestSendTopologyHashAbsentTopologyYieldsEmpty(t *testing.T) {
	if got := SendTopologyHash(nil); got != "" {
		t.Fatalf("nil topology must yield empty hash, got %s", got)
	}
	if got := SendTopologyHash(map[string]any{}); got != "" {
		t.Fatalf("empty topology map must yield empty hash, got %s", got)
	}
	// 无边列表键的形状（非 track.send.list 契约）不得伪造指纹。
	if got := SendTopologyHash(map[string]any{"status": "ok"}); got != "" {
		t.Fatalf("non-contract shape must yield empty hash, got %s", got)
	}
}

func TestRoutingHashWithSendTopologyRotatesCombinedIdentity(t *testing.T) {
	base := "chain:abc"
	combined1 := RoutingHashWithSendTopology(base, "sendtopo:1111")
	combined2 := RoutingHashWithSendTopology(base, "sendtopo:2222")
	if combined1 == combined2 {
		t.Fatal("send hash change must rotate the combined routing identity")
	}
	if combined1 == base || combined2 == base {
		t.Fatal("combined identity must differ from the base")
	}
	if got := RoutingHashWithSendTopology(base, ""); got != base {
		t.Fatalf("empty send hash must keep the base, got %s", got)
	}
	if got := RoutingHashWithSendTopology("", "sendtopo:1111"); got != "sendtopo:1111" {
		t.Fatalf("empty base must keep the send hash, got %s", got)
	}
}
