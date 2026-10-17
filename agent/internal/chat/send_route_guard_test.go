package chat

import (
	"testing"

	"vit-daw-agent/internal/policy"
)

// AGENT-SENDROUTE-SEMANTICS-1：guard 白名单与事件路由面注册回归。

func sendRouteDecisionForName(name string) policy.Decision {
	return policy.Decision{Command: map[string]any{"cmd": name}, Name: name}
}

func TestFreeStateKnownMutationNameSendRoute(t *testing.T) {
	writes := []string{
		"track.send.connect", "track_send_connect",
		"track.send.set_amount", "track_send_set_amount",
		"track.send.disconnect", "track_send_disconnect",
		"track.send.set_fx_role", "track_send_set_fx_role",
	}
	for _, name := range writes {
		if !freeStateKnownMutationName(name) {
			t.Fatalf("write command %s must be a known free-state mutation", name)
		}
	}
	// 读命令不得入写白名单。
	for _, name := range []string{"track.send.list", "track_send_list"} {
		if freeStateKnownMutationName(name) {
			t.Fatalf("read command %s must not be a known mutation", name)
		}
	}
}

func TestLegacyChatBroadMixDecisionBlockedSendRoute(t *testing.T) {
	for _, name := range []string{"track.send.connect", "track_send_set_amount", "track.send.disconnect"} {
		if !legacyChatBroadMixDecisionBlocked(sendRouteDecisionForName(name)) {
			t.Fatalf("send write %s must be blocked for broad-mix legacy decisions", name)
		}
	}
	if legacyChatBroadMixDecisionBlocked(sendRouteDecisionForName("track.send.list")) {
		t.Fatal("read-only track.send.list must not be blocked")
	}
}

func TestToolDisplayNameSendRoute(t *testing.T) {
	if got := toolDisplayName("track_send_connect"); got != "接通发送路由" {
		t.Fatalf("display name = %q", got)
	}
	if got := toolDisplayName("track.send.list"); got != "发送拓扑读取" {
		t.Fatalf("display name = %q", got)
	}
}

func TestChatCommandsContainSendRoute(t *testing.T) {
	if !chatCommandsContainSendRoute([]map[string]any{{"cmd": "track_send_connect"}}) {
		t.Fatal("underscore send command must be detected")
	}
	if !chatCommandsContainSendRoute([]map[string]any{{"command": "track.send.set_amount"}}) {
		t.Fatal("dotted send command must be detected")
	}
	if chatCommandsContainSendRoute([]map[string]any{{"cmd": "rack_add_node"}}) {
		t.Fatal("rack command must not be flagged as send route")
	}
}
