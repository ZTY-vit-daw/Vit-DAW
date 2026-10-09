package chat

// pull_coldstart_supply_test.go — G3-ATTRIB-1 目标④注入面单测：
// contextWithPullEngineSnapshot 的四门（模式/首装/影子就绪/调用方优先）。
// 模式经 env VIT_DAW_HARNESS 注入（ResolveHarnessMode env-wins 语义，
// 测试内 t.Setenv 确定性；不读落盘 config）。

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/shadow"
)

func pullColdStartTestServer(t *testing.T, engine map[string]any) *Server {
	t.Helper()
	shadowProject := shadow.New(nil)
	if engine != nil {
		shadowProject.Initialize(engine)
	}
	return New(nil, shadowProject, nil)
}

func TestPullEngineSnapshotPushModeLeavesContextUnchanged(t *testing.T) {
	t.Setenv("VIT_DAW_HARNESS", "push")
	server := pullColdStartTestServer(t, map[string]any{
		"project_path": "D:/tmp/fixture.vit",
		"tracks":       []any{map[string]any{"track_id": "t1", "name": "Lead Vocal"}},
	})
	in := map[string]any{"agent_mode": "chat"}
	out := server.contextWithPullEngineSnapshot("conv_pull_supply", in)
	if _, injected := out["engine_snapshot"]; injected {
		t.Fatalf("push mode must not inject engine_snapshot (got keys=%v)", out)
	}
}

func TestPullEngineSnapshotPullModeInjectsShadowEngineSnapshot(t *testing.T) {
	t.Setenv("VIT_DAW_HARNESS", "pull")
	server := pullColdStartTestServer(t, map[string]any{
		"project_path": "D:/tmp/fixture.vit",
		"tracks": []any{
			map[string]any{"track_id": "t1", "name": "Lead Vocal"},
			map[string]any{"track_id": "t2", "name": "Bass"},
		},
	})
	in := map[string]any{"agent_mode": "chat"}
	out := server.contextWithPullEngineSnapshot("conv_pull_supply", in)
	engine, ok := out["engine_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("pull mode with initialized shadow must inject engine_snapshot map (got %T)", out["engine_snapshot"])
	}
	tracks, _ := engine["tracks"].([]any)
	if len(tracks) != 2 {
		t.Fatalf("injected snapshot must carry the shadow engine tracks (got %d)", len(tracks))
	}
	if out["agent_mode"] != "chat" {
		t.Fatalf("injection must preserve existing context keys (got %v)", out)
	}
}

func TestPullEngineSnapshotAbsentWhenShadowUninitialized(t *testing.T) {
	t.Setenv("VIT_DAW_HARNESS", "pull")
	server := pullColdStartTestServer(t, nil)
	in := map[string]any{"agent_mode": "chat"}
	out := server.contextWithPullEngineSnapshot("conv_pull_supply", in)
	if _, injected := out["engine_snapshot"]; injected {
		t.Fatalf("uninitialized shadow must stay absent (cold-start facts render as absent)")
	}
}

func TestPullEngineSnapshotCallerSuppliedSnapshotWins(t *testing.T) {
	t.Setenv("VIT_DAW_HARNESS", "pull")
	server := pullColdStartTestServer(t, map[string]any{
		"project_path": "D:/tmp/fixture.vit",
		"tracks":       []any{map[string]any{"track_id": "t1"}},
	})
	caller := map[string]any{"engine_snapshot": map[string]any{"project_path": "caller://explicit"}}
	out := server.contextWithPullEngineSnapshot("conv_pull_supply", caller)
	engine, _ := out["engine_snapshot"].(map[string]any)
	if engine["project_path"] != "caller://explicit" {
		t.Fatalf("caller-supplied engine_snapshot must win (got %v)", engine)
	}
}

func TestPullEngineSnapshotSkippedWhenConversationHasContinuation(t *testing.T) {
	t.Setenv("VIT_DAW_HARNESS", "pull")
	server := pullColdStartTestServer(t, map[string]any{
		"project_path": "D:/tmp/fixture.vit",
		"tracks":       []any{map[string]any{"track_id": "t1"}},
	})
	const conversationID = "conv_pull_supply_resumed"
	server.mu.Lock()
	server.conversationGoals[conversationID] = "goal_pull_supply"
	server.goalContinuations["goal_pull_supply"] = agentloop.Continuation{GoalID: "goal_pull_supply"}
	server.mu.Unlock()
	in := map[string]any{"agent_mode": "chat"}
	out := server.contextWithPullEngineSnapshot(conversationID, in)
	if _, injected := out["engine_snapshot"]; injected {
		t.Fatalf("conversation with a live continuation must keep the first-install snapshot via continuation context")
	}
}
