package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

// b5SwitchServerForTest builds a kernel-free server whose workspace A carries
// a seeded checkpoint at revision rev-b5 so the switch-settle notice node
// append takes the checkpoint gate's skip path (bind to HEAD, zero kernel).
func b5SwitchServerForTest(t *testing.T) (*Server, string, string) {
	t.Helper()
	root := t.TempDir()
	projectA := filepath.Join(root, "b5a.vit")
	projectB := filepath.Join(root, "b5b.vit")
	if err := os.WriteFile(projectA, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectB, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	const uuidA = "vitproj_b5_switch_a"
	const uuidB = "vitproj_b5_switch_b"
	history.BindProjectIdentity(projectA, uuidA)
	history.BindProjectIdentity(projectB, uuidB)
	if _, err := history.EnsureWorkingSession(projectA, uuidA); err != nil {
		t.Fatal(err)
	}
	if _, err := history.Checkpoint(map[string]any{
		"project_path":         projectA,
		"message":              "seed",
		"source":               "test",
		"project_snapshot_xml": "<project/>",
		"project_revision":     "rev-b5",
	}); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"project_path": projectA, "project_uuid": uuidA,
		"project_revision": "rev-b5", "tracks": []any{},
	})
	server := New(nil, shadowProject, nil)
	server.activateCurrentProjectWorkspace(context.Background())
	return server, projectA, projectB
}

func b5PendingContinuationResult() agentloop.Result {
	return agentloop.Result{
		GoalID: "goal-b5", RunID: "run-b5", TaskID: "task-b5", SliceID: "slice-b5-1", TurnID: "turn-b5-1",
		OriginalIntent: "inspect the mix", Status: agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonLimitReached,
		Continuation: &agentloop.Continuation{
			GoalID: "goal-b5", RunID: "run-b5", TaskID: "task-b5", SliceID: "slice-b5-1", TurnID: "turn-b5-1",
			OriginalIntent: "inspect the mix", UserText: "inspect the mix", Summary: "inspect the mix",
		},
	}
}

// B5 pin ①（主修·先收尾语义）：切换激活时旧工作区的在飞链不得被静默整体
// 替换——必须显式收尾：durable 记录取消且留原因、goal 折叠 stopped、
// turn.stopped 终局通知事件投递到原会话、通知节点落进旧工作区会话图、
// 收尾结果持久化进旧工作区 runtime state。现场（2026-09-11 手测点 3 尝试 #1）：
// 切换后零调度器活动、会话图止于中间汇报、终局永不发生。
func TestWorkspaceSwitchSettlesInFlightChainExplicitly(t *testing.T) {
	server, projectA, projectB := b5SwitchServerForTest(t)
	server.harness.EnsureGoal("goal-b5", "run-b5", "inspect the mix")
	server.harness.SetGoalStatus("goal-b5", agentruntime.StatusWaitingContinue, nil)
	if err := server.recordGoalResult("conv-b5", b5PendingContinuationResult()); err != nil {
		t.Fatal(err)
	}
	pendingBefore := 0
	for _, item := range server.durableContinuations {
		if item.Status == ContinuationPending {
			pendingBefore++
		}
	}
	if pendingBefore != 1 {
		t.Fatalf("setup: expected one pending in-flight chain, got %d (%+v)", pendingBefore, server.durableContinuations)
	}

	server.shadow.Initialize(map[string]any{
		"project_path": projectB, "project_uuid": "vitproj_b5_switch_b",
		"project_revision": "rev-b5", "tracks": []any{},
	})
	server.activateCurrentProjectWorkspace(context.Background())

	// The switch itself still proceeds (explicit settle, not a refused switch).
	server.mu.Lock()
	activeUUID, activePath := server.activeWorkspaceUUID, server.activeWorkspacePath
	server.mu.Unlock()
	if activeUUID != "vitproj_b5_switch_b" || !sameWorkspacePath(activePath, projectB) {
		t.Fatalf("switch did not proceed after settle: uuid=%q path=%q", activeUUID, activePath)
	}

	// The old workspace's persisted state carries the explicit close: cancelled
	// record with the workspace-switch reason and a stopped goal.
	persisted := readWorkingRuntimeState(t, projectA, "vitproj_b5_switch_a")
	foundCancelled := false
	for _, item := range persisted.DurableContinuations {
		if item.GoalID != "goal-b5" {
			continue
		}
		if item.Status != ContinuationCancelled || !strings.Contains(item.LastError, "workspace switched") {
			t.Fatalf("in-flight chain was not explicitly cancelled in the persisted old workspace: %+v", item)
		}
		foundCancelled = true
	}
	if !foundCancelled {
		t.Fatalf("persisted old workspace lost the chain record entirely: %+v", persisted.DurableContinuations)
	}
	for _, goal := range persisted.GoalRuntime.Goals {
		if goal.GoalID == "goal-b5" && goal.Status != agentruntime.StatusStopped {
			t.Fatalf("switched-away goal must fold to stopped in the persisted snapshot, got %s", goal.Status)
		}
	}

	// The chain's conversation receives an explicit terminal notice event.
	events, _ := server.agentEventsSince("conv-b5", 0, 64)
	var notice *AgentEvent
	for index := range events {
		event := events[index]
		if event.ItemID != "chain_result" {
			continue
		}
		notice = &event
		break
	}
	if notice == nil {
		t.Fatalf("switch settle must deliver an explicit terminal notice event: %+v", events)
	}
	if notice.Type != "turn.stopped" || !strings.Contains(notice.Body, "工程已切换") {
		t.Fatalf("notice event must be an explicit stop notice, got type=%s body=%q", notice.Type, notice.Body)
	}
	if reason, _ := notice.Payload["stop_reason"].(string); reason != workspaceSwitchStopReason {
		t.Fatalf("notice event must carry the workspace_switched stop reason: %+v", notice.Payload)
	}

	// The notice lands in the old workspace's conversation graph (refresh
	// hydration must show why the chain stopped).
	messages, ok := server.harness.ProjectHistorySummaryForProject(context.Background(), "goal-b5", projectA)["conversation_messages"].([]history.ConversationMessage)
	if !ok {
		messages = nil
	}
	landed := false
	for _, message := range messages {
		if strings.Contains(message.Content, "工程已切换") {
			landed = true
		}
	}
	if !landed {
		t.Fatalf("switch settle notice must land in the old workspace graph: %+v", messages)
	}
}

// B5 pin ②（陈旧恢复卫生）：陈旧快照恢复的 live 态归一为终态——幻忙态消灭。
// 陈旧 runnable 续跑→cancelled；无人持有的陈旧 running goal→stopped；陈旧
// 活跃 loop→stopped。新鲜 runnable（重启恢复窗口内）与时间戳不明（零值，
// 既有恢复测试形态）的保持可恢复；可应答驻留（带真实交互请求的
// waiting_interaction + waiting_confirmation goal）不折叠（F1 口径）。
func TestStaleRestoredRuntimeStateFoldsLiveProjectionToTerminal(t *testing.T) {
	now := time.Now().UTC()
	stale := now.Add(-2 * time.Hour)
	parkRequests := map[string]any{"status": "waiting_confirmation", "requests": []any{map[string]any{
		"request_id": "b5-park-1", "kind": "confirmation", "prompt": "确认这个调整",
	}}}
	state := projectAgentRuntimeState{
		SchemaVersion: "vit_project_agent_runtime.v2",
		ProjectPath:   "D:/b5/stale.vit",
		ProjectUUID:   "vitproj_b5_stale",
		DurableContinuations: map[string]DurableContinuation{
			"cont_b5_stale_run": {
				ContinuationID: "cont_b5_stale_run", TaskID: "task-b5s", GoalID: "goal-b5-stale", RunID: "run-b5-stale",
				CurrentSliceID: "slice-1", ConversationID: "conv-b5-stale", OriginalIntent: "inspect",
				Status: ContinuationPending, CreatedAt: stale, UpdatedAt: stale,
				Continuation: agentloop.Continuation{GoalID: "goal-b5-stale", RunID: "run-b5-stale", TaskID: "task-b5s", SliceID: "slice-1"},
			},
			"cont_b5_fresh_run": {
				ContinuationID: "cont_b5_fresh_run", TaskID: "task-b5f", GoalID: "goal-b5-fresh", RunID: "run-b5-fresh",
				CurrentSliceID: "slice-1", ConversationID: "conv-b5-fresh", OriginalIntent: "inspect fresh",
				Status: ContinuationPending, CreatedAt: now, UpdatedAt: now,
				Continuation: agentloop.Continuation{GoalID: "goal-b5-fresh", RunID: "run-b5-fresh", TaskID: "task-b5f", SliceID: "slice-1"},
			},
			"cont_b5_stale_park": {
				ContinuationID: "cont_b5_stale_park", TaskID: "task-b5p", GoalID: "goal-b5-park", RunID: "run-b5-park",
				CurrentSliceID: "slice-1", ConversationID: "conv-b5-park", OriginalIntent: "await answer",
				Status: ContinuationWaitingInteraction, PendingInteraction: parkRequests,
				CreatedAt: stale, UpdatedAt: stale,
				Continuation: agentloop.Continuation{GoalID: "goal-b5-park", RunID: "run-b5-park", TaskID: "task-b5p", SliceID: "slice-1"},
			},
			"cont_b5_stale_shell": {
				ContinuationID: "cont_b5_stale_shell", TaskID: "task-b5sh", GoalID: "goal-b5-shell", RunID: "run-b5-shell",
				CurrentSliceID: "slice-1", ConversationID: "conv-b5-shell", OriginalIntent: "shell",
				Status: ContinuationWaitingInteraction,
				PendingInteraction: map[string]any{"status": "legacy_waiting_continue", "reason": "legacy checkpoint has no authoritative stop reason"},
				CreatedAt: stale, UpdatedAt: stale,
				Continuation: agentloop.Continuation{GoalID: "goal-b5-shell", RunID: "run-b5-shell", TaskID: "task-b5sh", SliceID: "slice-1"},
			},
			"cont_b5_zero_time": {
				ContinuationID: "cont_b5_zero_time", TaskID: "task-b5z", GoalID: "goal-b5-zero", RunID: "run-b5-zero",
				CurrentSliceID: "slice-1", ConversationID: "conv-b5-zero", OriginalIntent: "unknown provenance",
				Status: ContinuationPending,
				Continuation: agentloop.Continuation{GoalID: "goal-b5-zero", RunID: "run-b5-zero", TaskID: "task-b5z", SliceID: "slice-1"},
			},
		},
		FreeStateLoops: map[string]freeStateReasoningLoop{
			"conv-b5-stale": {
				SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-b5-stale", ConversationID: "conv-b5-stale",
				GoalID: "goal-b5-stale", RunID: "run-b5-stale", Status: "observing", OriginalIntent: "inspect",
				CreatedAt: stale, UpdatedAt: stale,
			},
			"conv-b5-fresh": {
				SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-b5-fresh", ConversationID: "conv-b5-fresh",
				GoalID: "goal-b5-fresh", RunID: "run-b5-fresh", Status: "observing", OriginalIntent: "inspect fresh",
				CreatedAt: now, UpdatedAt: now,
			},
		},
		GoalRuntime: agentruntime.Snapshot{Goals: []agentruntime.Goal{
			{GoalID: "goal-b5-stale", RunID: "run-b5-stale", Status: agentruntime.StatusRunning, UpdatedAt: stale},
			{GoalID: "goal-b5-park", RunID: "run-b5-park", Status: agentruntime.StatusWaitingConfirmation, UpdatedAt: stale},
		}},
	}

	server := New(nil, shadow.New(nil), nil)
	server.restoreProjectAgentRuntimeStateLocked(state)

	if item, ok := server.durableContinuations["cont_b5_stale_run"]; !ok || item.Status != ContinuationCancelled || item.LastError == "" {
		t.Fatalf("stale runnable continuation must fold to an explicit cancelled terminal: %+v", item)
	}
	if item, ok := server.durableContinuations["cont_b5_fresh_run"]; !ok || item.Status != ContinuationPending {
		t.Fatalf("fresh runnable continuation must stay recoverable (restart window): %+v", item)
	}
	if item, ok := server.durableContinuations["cont_b5_zero_time"]; !ok || item.Status != ContinuationPending {
		t.Fatalf("zero-timestamp continuation must stay recoverable (unknown provenance, fail-open): %+v", item)
	}
	if item, ok := server.durableContinuations["cont_b5_stale_park"]; !ok || item.Status != ContinuationWaitingInteraction {
		t.Fatalf("answerable park must survive the hygiene fold (F1): %+v", item)
	}
	if item, ok := server.durableContinuations["cont_b5_stale_shell"]; !ok || item.Status != ContinuationCancelled {
		t.Fatalf("stale unanswerable shell must fold to terminal: %+v", item)
	}
	if loop, ok := server.freeStateLoops["conv-b5-stale"]; !ok || loop.Status != "stopped" {
		t.Fatalf("stale active loop must fold to stopped: %+v", loop)
	}
	if loop, ok := server.freeStateLoops["conv-b5-fresh"]; !ok || loop.Status == "stopped" {
		t.Fatalf("fresh loop must stay active: %+v", loop)
	}
	if status := server.harness.RuntimeStatus("goal-b5-stale").Status; status != agentruntime.StatusStopped {
		t.Fatalf("stale ownerless running goal must fold to stopped, got %s", status)
	}
	if status := server.harness.RuntimeStatus("goal-b5-park").Status; status != agentruntime.StatusWaitingConfirmation {
		t.Fatalf("answerable park goal must stay answerable, got %s", status)
	}
}

// B5 pin ③（零回退）：无活链时切换行为与现状完全一致——内存态照常整体
// 替换、终态记录不被触碰、不产生任何收尾事件或通知节点。
func TestWorkspaceSwitchWithoutLiveChainsKeepsLegacyReplacement(t *testing.T) {
	server, projectA, projectB := b5SwitchServerForTest(t)
	server.mu.Lock()
	server.conversations["conv-b5-quiet"] = []llm.Message{{Role: "user", Content: "quiet turn"}}
	server.durableContinuations["cont_b5_done"] = DurableContinuation{
		ContinuationID: "cont_b5_done", TaskID: "task-b5q", GoalID: "goal-b5-quiet", RunID: "run-b5-quiet",
		ConversationID: "conv-b5-quiet", OriginalIntent: "already finished", Status: ContinuationCompleted,
	}
	server.mu.Unlock()
	server.persistCurrentProjectWorkspace()

	server.shadow.Initialize(map[string]any{
		"project_path": projectB, "project_uuid": "vitproj_b5_switch_b",
		"project_revision": "rev-b5", "tracks": []any{},
	})
	server.activateCurrentProjectWorkspace(context.Background())

	if len(server.conversations) != 0 || len(server.durableContinuations) != 0 {
		t.Fatalf("quiet switch must keep the legacy wholesale replacement: conversations=%d continuations=%d", len(server.conversations), len(server.durableContinuations))
	}
	events, _ := server.agentEventsSince("conv-b5-quiet", 0, 64)
	for _, event := range events {
		if event.ItemID == "chain_result" {
			t.Fatalf("quiet switch must not emit settle notices: %+v", event)
		}
	}
	persisted := readWorkingRuntimeState(t, projectA, "vitproj_b5_switch_a")
	if item, ok := persisted.DurableContinuations["cont_b5_done"]; !ok || item.Status != ContinuationCompleted {
		t.Fatalf("terminal record must pass through the switch untouched: %+v", item)
	}
	summary := server.harness.ProjectHistorySummaryForProject(context.Background(), "goal-b5-quiet", projectA)
	if messages, ok := summary["conversation_messages"].([]history.ConversationMessage); ok {
		for _, message := range messages {
			if strings.Contains(message.Content, "工程已切换") {
				t.Fatalf("quiet switch must not append notice nodes: %+v", message)
			}
		}
	}
}
