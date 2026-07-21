package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
)

func TestExternalSaveAsForksWorkingRuntimeAndKeepsSourceCanonicalFrozen(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "B1.vit")
	targetPath := filepath.Join(root, "B2.vit")
	if err := os.WriteFile(sourcePath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("b2"), 0o644); err != nil {
		t.Fatal(err)
	}
	const sourceUUID = "vitproj_chat_b1"
	const targetUUID = "vitproj_chat_b2"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": sourcePath, "project_uuid": sourceUUID})
	kernel := &recordingChatKernel{replyByCommand: map[string][]map[string]any{
		"get_project_state": {{"status": "ok", "project_path": targetPath, "project_uuid": targetUUID, "parent_project_uuid": sourceUUID}},
	}}
	server := New(nil, project, nil)
	server.harness = harness.NewWithSender(kernel, project, nil)
	server.activateCurrentProjectWorkspace(context.Background())

	server.mu.Lock()
	server.conversations["chat_b1"] = []llm.Message{{Role: "user", Content: "B1 complete"}}
	server.mu.Unlock()
	server.harness.EnsureGoal("goal_b1", "run_b1", "B1 complete")
	server.harness.SetGoalStatus("goal_b1", agentruntime.StatusCompleted, nil)
	server.persistCurrentProjectWorkspace()
	b1Prepared, err := history.PrepareWorkingSessionSave(sourcePath, sourceUUID, "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.CommitPreparedWorkingSession(sourcePath, sourceUUID, sourcePath, sourceUUID, b1Prepared["prepare_id"].(string), b1Prepared["generation_id"].(string), "save"); err != nil {
		t.Fatal(err)
	}

	candidate := testPendingMixTick()
	server.mu.Lock()
	server.conversations["chat_b2"] = []llm.Message{{Role: "user", Content: "prepare a mix tick"}}
	server.pendingMixTicks["chat_b2"] = candidate
	server.conversationMemory["chat_b2"] = agentloop.ExecutionMemory{PendingMixTickCandidate: &candidate}
	server.mu.Unlock()
	server.upsertPendingCandidate(candidate.ToPendingCandidate("chat_b2", "goal_b2", "run_b2", ""))
	server.harness.EnsureGoal("goal_b2", "run_b2", "prepare a mix tick")
	server.harness.SetGoalStatus("goal_b2", agentruntime.StatusWaitingConfirmation, nil)
	server.persistCurrentProjectWorkspace()
	b2Prepared, err := history.PrepareWorkingSessionSave(sourcePath, sourceUUID, "save_as")
	if err != nil {
		t.Fatal(err)
	}

	response, err := server.harness.Invoke(context.Background(), harness.InvokeRequest{
		Tool: "version.project_saved", Confirmed: true, Source: "godot_save",
		Args: map[string]any{
			"save_kind": "save_as", "project_path": targetPath, "project_uuid": targetUUID,
			"source_project_path": sourcePath, "source_project_uuid": sourceUUID,
			"history_prepare_id": b2Prepared["prepare_id"], "agent_history_generation": b2Prepared["generation_id"],
		},
	})
	if err != nil || response.Status != "ok" {
		t.Fatalf("external save-as response=%+v err=%v", response, err)
	}
	server.syncCurrentProjectWorkspace(context.Background())

	sourceState := readCanonicalRuntimeState(t, root, sourceUUID)
	targetState := readCanonicalRuntimeState(t, root, targetUUID)
	if len(sourceState.Conversations["chat_b1"]) != 1 || len(sourceState.Conversations["chat_b2"]) != 0 || len(sourceState.PendingMixTicks) != 0 {
		t.Fatalf("source canonical was contaminated by later work: %+v", sourceState)
	}
	if len(targetState.Conversations["chat_b1"]) != 1 || len(targetState.Conversations["chat_b2"]) != 1 || targetState.PendingMixTicks["chat_b2"].TrackID != candidate.TrackID {
		t.Fatalf("target canonical did not inherit complete working state: %+v", targetState)
	}
	if sourceState.GoalRuntime.LastGoalID != "goal_b1" || targetState.GoalRuntime.LastGoalID != "goal_b2" {
		t.Fatalf("Save As goal runtime split failed: source=%+v target=%+v", sourceState.GoalRuntime, targetState.GoalRuntime)
	}
}

func TestProjectWorkspaceRestartRetiresPersistedLegacyB3Authority(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "legacy-b3.vit")
	const projectUUID = "vitproj_legacy_b3_restart"
	if err := os.WriteFile(projectPath, []byte("b3"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	bootstrap := New(nil, project, nil)
	bootstrap.activateCurrentProjectWorkspace(context.Background())
	plan := testPanLayoutPlan()
	legacyState := projectAgentRuntimeState{
		SchemaVersion: "vit_project_agent_runtime.v1", ProjectPath: projectPath, ProjectUUID: projectUUID,
		Conversations:         map[string][]llm.Message{"chat_b3": {{Role: "user", Content: "perform B3"}}},
		ConversationMemory:    map[string]agentloop.ExecutionMemory{"chat_b3": {PendingPanLayoutPlan: &plan}},
		PendingPanLayoutPlans: map[string]agentloop.PendingPanLayoutPlan{"chat_b3": plan},
	}
	data, err := json.Marshal(legacyState)
	if err != nil {
		t.Fatal(err)
	}
	if err := history.WriteAgentRuntimeState(projectPath, projectUUID, data); err != nil {
		t.Fatal(err)
	}

	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	if len(restarted.conversations["chat_b3"]) != 1 {
		t.Fatalf("conversation was lost while retiring legacy authority: %#v", restarted.conversations)
	}
	if memory := restarted.conversationMemory["chat_b3"]; memory.PendingPanLayoutPlan != nil {
		t.Fatalf("legacy B3 execution memory was restored: %#v", memory)
	}
	if active := restarted.pendingManager.ActiveForConversation("chat_b3"); len(active) != 0 {
		t.Fatalf("legacy B3 candidate remained executable: %#v", active)
	}
	retired := restarted.pendingManager.Snapshot()
	if len(retired) != 1 || retired[0].Status != agentprotocol.PendingStatusRejected {
		t.Fatalf("legacy B3 retirement audit missing: %#v", retired)
	}
	restarted.persistCurrentProjectWorkspace()
	written := readWorkingRuntimeState(t, projectPath, projectUUID)
	if len(written.PendingPanLayoutPlans) != 0 || len(written.PendingStaticBalancePlans) != 0 {
		t.Fatalf("retired authority was written back: %#v", written)
	}
}

func TestSameProjectReopenLoadsSavedConversationAndLeavesUnsavedDraft(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "B1.vit")
	const projectUUID = "vitproj_same_path_reopen"
	if err := os.WriteFile(projectPath, []byte("b1"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	server := New(nil, project, nil)
	server.activateCurrentProjectWorkspace(context.Background())

	server.mu.Lock()
	server.conversations["saved"] = []llm.Message{{Role: "user", Content: "B1 complete"}}
	server.mu.Unlock()
	server.persistCurrentProjectWorkspace()
	prepared, err := history.PrepareWorkingSessionSave(projectPath, projectUUID, "save")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.CommitPreparedWorkingSession(projectPath, projectUUID, projectPath, projectUUID, prepared["prepare_id"].(string), prepared["generation_id"].(string), "save"); err != nil {
		t.Fatal(err)
	}
	server.activateCurrentProjectWorkspace(context.Background())

	server.mu.Lock()
	server.conversations["unsaved"] = []llm.Message{{Role: "user", Content: "draft"}}
	server.mu.Unlock()
	server.persistCurrentProjectWorkspace()
	head, err := history.SavedHeadForProject(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := history.OpenWorkingSessionAtGeneration(projectPath, projectUUID, head.GenerationID); err != nil {
		t.Fatal(err)
	}
	server.activateCurrentProjectWorkspace(context.Background())
	if len(server.conversations["saved"]) != 1 || len(server.conversations["unsaved"]) != 0 {
		t.Fatalf("saved/draft boundary was not restored: %#v", server.conversations)
	}
}

func TestProjectWorkspaceSwitchAndRestartRestoresGenericPendingState(t *testing.T) {
	root := t.TempDir()
	projectA := filepath.Join(root, "a.vit")
	projectB := filepath.Join(root, "b.vit")
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{"project_path": projectA, "project_uuid": "vitproj_a"})
	server := New(nil, shadowProject, nil)
	server.activateCurrentProjectWorkspace(context.Background())
	candidate := testPendingMixTick()
	server.conversations["chat_a"] = []llm.Message{{Role: "user", Content: "prepare tick"}}
	server.pendingMixTicks["chat_a"] = candidate
	server.conversationMemory["chat_a"] = agentloop.ExecutionMemory{PendingMixTickCandidate: &candidate}
	server.upsertPendingCandidate(candidate.ToPendingCandidate("chat_a", "goal_a", "run_a", ""))
	server.persistCurrentProjectWorkspace()

	shadowProject.Initialize(map[string]any{"project_path": projectB, "project_uuid": "vitproj_b"})
	server.activateCurrentProjectWorkspace(context.Background())
	if len(server.conversations) != 0 || len(server.pendingMixTicks) != 0 || len(server.pendingManager.ActiveForConversation("chat_a")) != 0 {
		t.Fatal("project B inherited project A runtime state")
	}

	shadowProject.Initialize(map[string]any{"project_path": projectA, "project_uuid": "vitproj_a"})
	server.activateCurrentProjectWorkspace(context.Background())
	assertProjectAGenericRuntimeRestored(t, server)
	server.persistCurrentProjectWorkspace()
	restarted := New(nil, shadowProject, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	assertProjectAGenericRuntimeRestored(t, restarted)
}

func readCanonicalRuntimeState(t *testing.T, projectDir, projectUUID string) projectAgentRuntimeState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, history.DirName, projectUUID, "state", "agent_runtime_state.json"))
	if err != nil {
		t.Fatal(err)
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func readWorkingRuntimeState(t *testing.T, projectPath, projectUUID string) projectAgentRuntimeState {
	t.Helper()
	data, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func testPendingMixTick() agentloop.PendingMixTickCandidate {
	return agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "v", DeltaDB: 1,
		ObservationID: "obs_tick", Status: "pending_confirmation", ExpiresAfterContextChange: true,
	}
}

func assertProjectAGenericRuntimeRestored(t *testing.T, server *Server) {
	t.Helper()
	if len(server.conversations["chat_a"]) != 1 {
		t.Fatalf("conversation was not restored: %#v", server.conversations)
	}
	if candidate, ok := server.pendingMixTicks["chat_a"]; !ok || candidate.TrackID != "v" {
		t.Fatalf("generic pending mix tick was not restored: %#v", server.pendingMixTicks)
	}
	if active := server.pendingManager.ActiveForConversation("chat_a"); len(active) != 1 || active[0].CandidateType != "mix_tick" {
		t.Fatalf("pending manager was not restored: %#v", active)
	}
}
