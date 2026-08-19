package chat

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/llm"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/trajectory"
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

func TestProjectWorkspaceRestartRestoresFreeStateObservationLedger(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "free-state-ledger.vit")
	const projectUUID = "vitproj_free_state_ledger_restart"
	if err := os.WriteFile(projectPath, []byte("ledger"), 0o644); err != nil {
		t.Fatal(err)
	}
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	server := New(nil, project, nil)
	server.activateCurrentProjectWorkspace(context.Background())
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-ledger", ConversationID: "chat-ledger",
		Status: "observing", DecisionPhase: freeStatePhaseProcessorSelection,
		OriginalIntent: "inspect masking", ActiveIntent: "inspect masking", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"rejected_view_sets": []map[string]any{{
				"fingerprint":     freeStateNormalizedViewFingerprint([]string{"mix.masking_relationship", "track.basic_energy"}),
				"requested_views": []string{"mix.masking_relationship", "track.basic_energy"},
				"status":          "rejected", "retry_policy": "do_not_retry", "receipt_id": "ccbr-workspace",
			}},
			"receipts": []map[string]any{{
				"receipt_id": "ccbr-workspace", "tool_call_id": "ccb-workspace", "status": "rejected",
				"requested_views": []string{"mix.masking_relationship", "track.basic_energy"},
			}},
		},
	}
	server.storeFreeStateLoop(loop)
	server.persistCurrentProjectWorkspace()

	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	restored, ok := restarted.freeStateLoop("chat-ledger")
	if !ok || len(freeStateMapRows(restored.ObservationLedger["rejected_view_sets"])) != 1 {
		t.Fatalf("workspace restart lost rejection ledger: %#v", restored)
	}
	bound, active := restarted.prepareFreeStateReasoningContext("chat-ledger", "continue", map[string]any{})
	boundLoop := firstMapFromAny(bound["free_state_reasoning_loop"])
	boundLedger := firstMapFromAny(boundLoop["observation_ledger"])
	if !active || len(freeStateMapRows(boundLedger["rejected_view_sets"])) != 1 ||
		firstStringFromMap(freeStateMapRows(boundLedger["receipts"])[0], "receipt_id") != "ccbr-workspace" {
		t.Fatalf("workspace-restored ledger was not rebound: %#v", bound)
	}
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

func TestProjectWorkspaceRestoresAuditionJudgmentEvidenceWithoutReRequest(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "Judgment.vit")
	if err := os.WriteFile(projectPath, []byte("judgment"), 0o644); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_judgment_restore"
	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	server := New(nil, project, nil)
	server.activateCurrentProjectWorkspace(context.Background())
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-persisted"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "status": "ready", "project_revision": "rev-7", "candidates": []any{map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "a", "preview_ref": "preview:a"}, map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "b", "preview_ref": "preview:b"}}}
	if _, err := loop.Experiment.RequestUserJudgmentForSession("compare", loop.AuditionSessionID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	round, _ := loop.Experiment.CurrentRound()
	evidence := experiment.UserJudgmentEvidence{SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ID: "judgment-persisted", ConversationID: loop.ConversationID, TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: loop.AuditionSessionID, CandidateARef: "a", CandidateBRef: "b", ProjectUUID: projectUUID, ProjectRevision: "rev-7", HeardDifference: experiment.HeardDifferenceNo, Preference: experiment.PreferenceUnsure, CreatedAt: time.Now().UTC()}
	if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	server.storeFreeStateLoop(loop)
	server.persistCurrentProjectWorkspace()

	restarted := New(nil, project, nil)
	restarted.activateCurrentProjectWorkspace(context.Background())
	restored, ok := restarted.freeStateLoop(loop.ConversationID)
	if !ok || restored.Experiment == nil {
		t.Fatalf("restored loop missing: %+v", restored)
	}
	restoredRound, _ := restored.Experiment.CurrentRound()
	if len(restoredRound.UserJudgmentEvidence) != 1 || restoredRound.UserJudgmentEvidence[0].ID != evidence.ID || restoredRound.UserJudgmentRequested {
		t.Fatalf("judgment evidence was not restored or was re-requested: %+v", restoredRound)
	}
	restarted.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)
	after, _ := restarted.freeStateLoop(loop.ConversationID)
	afterRound, _ := after.Experiment.CurrentRound()
	if afterRound.UserJudgmentRequested || len(afterRound.UserJudgmentEvidence) != 1 {
		t.Fatalf("completed judgment was requested again: %+v", afterRound)
	}
}
