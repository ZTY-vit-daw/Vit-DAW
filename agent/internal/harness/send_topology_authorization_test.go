package harness

import (
	"testing"

	"vit-daw-agent/internal/tools"
)

// AGENT-SENDROUTE-SEMANTICS-1：确认 send 计划的写闸豁免只覆盖计划自己的
// 源轨集合；越轨命令与无凭据上下文仍然被观察优先闸管住。

func TestQualifiedSendTopologyConnect(t *testing.T) {
	spec, ok := tools.DefaultCatalog().LookupCommand("track.send.connect")
	if !ok {
		t.Fatal("track.send.connect not in catalog")
	}
	amountSpec, _ := tools.DefaultCatalog().LookupCommand("track.send.set_amount")
	listSpec, _ := tools.DefaultCatalog().LookupCommand("track.send.list")

	authorized := AuthorizeSendTopologyConnect(map[string]any{}, []string{"T1", "T2"})
	if !qualifiedSendTopologyConnect(authorized, spec, map[string]any{"source_track_id": "T1"}) {
		t.Fatal("in-plan source connect must be qualified")
	}
	if qualifiedSendTopologyConnect(authorized, spec, map[string]any{"source_track_id": "T9"}) {
		t.Fatal("out-of-plan source connect must not be qualified")
	}
	if qualifiedSendTopologyConnect(authorized, spec, map[string]any{}) {
		t.Fatal("source-less connect must not be qualified")
	}
	if !qualifiedSendTopologyConnect(authorized, amountSpec, map[string]any{"source_track_id": "T2"}) {
		t.Fatal("in-plan set_amount must be qualified")
	}
	if qualifiedSendTopologyConnect(authorized, listSpec, map[string]any{}) {
		t.Fatal("list is read-only and never needs the exemption")
	}
	if qualifiedSendTopologyConnect(map[string]any{}, spec, map[string]any{"source_track_id": "T1"}) {
		t.Fatal("no authorization context must not be qualified")
	}
}

func TestBroadMixWriteCommandSendRoute(t *testing.T) {
	for _, name := range []string{"track.send.connect", "track_send_disconnect", "track.send.set_fx_role"} {
		if !broadMixWriteCommand(tools.CommandSpec{CommandName: name}, nil) {
			t.Fatalf("%s must be a broad-mix write command", name)
		}
	}
	if broadMixWriteCommand(tools.CommandSpec{CommandName: "track.send.list"}, nil) {
		t.Fatal("track.send.list is read-only")
	}
}
