package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
)

// TestGRuntimeRecoveryAndCollaborationSmoke is intentionally fixture-only. It
// exercises the product workspace persistence and scheduler paths without a
// Kernel client, tools, audio, candidate generation, or an HTTP mutation.
func TestGRuntimeRecoveryAndCollaborationSmoke(t *testing.T) {
	root := t.TempDir()
	projectPath := filepath.Join(root, "g-runtime-fixture.vit")
	if err := os.WriteFile(projectPath, []byte("identity only; no media"), 0o600); err != nil {
		t.Fatal(err)
	}
	const projectUUID = "vitproj_g_runtime_smoke"
	const conversationID = "g-runtime-conversation"
	const originalIntent = "检查一下当前工程有什么问题？"

	project := shadow.New(nil)
	project.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
	source := New(nil, project, nil)
	source.activateCurrentProjectWorkspace(context.Background())

	runner := agentloop.Runner{
		Runtime: source.harness.Runtime(),
		Planner: &continuationTestPlanner{outputs: []planner.Output{{
			ToolCalls: []planner.ToolCall{{ID: "g-observation-only", Tool: "project.state"}},
		}}},
		Executor: continuationTestExecutor{},
		Budget:   agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}
	paused := runner.Start(context.Background(), agentloop.Input{
		GoalID: "goal-g-runtime", RunID: "run-g-runtime", UserText: originalIntent, Summary: originalIntent,
		AllowedTools: []string{"project.state"},
		Context: map[string]any{
			"observation_ledger": map[string]any{"active_observation_id": "obs-g-1", "receipt": "project.state:g-1"},
			"current_plan":       []any{map[string]any{"id": "inspect", "status": "running"}},
		},
		ProjectHistory: map[string]any{"head": "revision-g-1", "active_branch": "g-runtime"},
		PlanItems:      []planner.PlanItem{{ID: "inspect", Description: "inspect project", Status: "running"}},
	})
	if paused.Status != agentruntime.StatusWaitingContinue || paused.Continuation == nil {
		t.Fatalf("fixture did not reach an automatic continuation boundary: %+v", paused)
	}
	contract := taskstate.Contract{
		ConversationID: conversationID, Kind: taskstate.ContractDiagnostic, Scope: taskstate.Scope{Kind: "project"},
		AuthorizationBoundary: "observe_and_propose_only", CompletionCriteria: []string{"evidence-backed bounded diagnosis"},
		EvidenceRequirements: []string{"project.state receipt"}, CreatedAt: time.Now().UTC(),
	}
	if _, err := source.harness.EnsureTaskContract(paused.GoalID, contract); err != nil {
		t.Fatalf("create fixture task contract: %v", err)
	}
	closure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-g-runtime", ConversationID: conversationID, GoalID: paused.GoalID, RunID: paused.RunID,
		ProjectUUID: projectUUID, ProjectRevision: "revision-g-1", OriginalIntent: originalIntent,
		Mode: audioclosure.ModeDiagnostic, Scope: audioclosure.Scope{Kind: "project"}, Now: time.Now().UTC(),
	})
	if err != nil || source.audioClosures.Create(closure) != nil {
		t.Fatalf("create fixture closure: state=%+v err=%v", closure, err)
	}
	source.mu.Lock()
	source.freeStateLoops[conversationID] = freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-g-runtime", ConversationID: conversationID,
		GoalID: paused.GoalID, RunID: paused.RunID, Status: "observing", OriginalIntent: originalIntent, ActiveIntent: originalIntent,
		ObservationLedger: map[string]any{"active_observation_id": "obs-g-1", "receipt": "project.state:g-1"},
		CreatedAt:         time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	source.capabilityRoutes[paused.TaskID] = CapabilityRouteRecord{
		SchemaVersion: capabilityRouteSchema, TaskID: paused.TaskID, GoalID: paused.GoalID, RunID: paused.RunID,
		ConversationID: conversationID, OriginalIntent: originalIntent, Controller: "minimal_audio_closure",
		SemanticEntry:   map[string]any{"route": semanticEntryRouteObservation, "target_scope": semanticEntryScopeProjectContext},
		ProjectRevision: "revision-g-1",
		Assessment: &FreeStateCapacityAssessment{
			SchemaVersion: capacityAssessmentSchema, Authority: "product_runtime",
			ObservedFacts:   CapacityObservedFacts{ProjectUUID: projectUUID, ProjectRevision: "revision-g-1", RequestScope: semanticEntryScopeProjectContext},
			ProjectRevision: "revision-g-1", CapacityLevel: "within_free_state", SelectedCapability: capabilityFreeState,
			ObservedAt: time.Now().UTC(),
		},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	// Model a process that crashes immediately after the request checkpoint is
	// durable, before its process-owned scheduler gets an opportunity to wake.
	source.mu.Unlock()
	durable := durableContinuationFromResult(conversationID, paused, time.Now().UTC())
	paused.Continuation.ContinuationID = durable.ContinuationID
	durable.ProjectPath, durable.ProjectUUID, durable.ProjectSessionID = projectPath, projectUUID, source.activeWorkspaceSessionID
	source.bindDurableTaskSemanticState(&durable)
	source.mu.Lock()
	source.goalContinuations[paused.GoalID] = durable.Continuation
	source.durableContinuations[durable.ContinuationID] = durable
	source.mu.Unlock()
	if err := source.persistCurrentProjectWorkspaceChecked(); err != nil {
		t.Fatalf("persist first invocation boundary: %v", err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	newProject := func() *shadow.Project {
		value := shadow.New(nil)
		value.Initialize(map[string]any{"status": "ok", "project_path": projectPath, "project_uuid": projectUUID})
		return value
	}
	workerA := New(nil, newProject(), nil)
	defer workerA.Close()
	workerA.activeRuntimeInvocations = 1
	workerA.activateCurrentProjectWorkspace(context.Background())
	// The test drives scheduler passes explicitly so the product startup worker
	// cannot race the two-owner lease assertions below.
	if err := workerA.Close(); err != nil {
		t.Fatal(err)
	}
	workerB := New(nil, newProject(), nil)
	defer workerB.Close()
	workerB.activeRuntimeInvocations = 1
	workerB.activateCurrentProjectWorkspace(context.Background())
	if err := workerB.Close(); err != nil {
		t.Fatal(err)
	}

	var executions atomic.Int32
	executorEntered := make(chan struct{}, 1)
	releaseExecutor := make(chan struct{})
	workerA.continuationExecutor = func(ctx context.Context, item DurableContinuation) error {
		if item.TaskID != paused.TaskID || item.GoalID != paused.GoalID || item.RunID != paused.RunID || item.ConversationID != conversationID || item.OriginalIntent != originalIntent {
			t.Fatalf("scheduler changed durable identity: %+v", item)
		}
		executions.Add(1)
		executorEntered <- struct{}{}
		<-releaseExecutor
		resumedRunner := agentloop.Runner{
			Runtime: workerA.harness.Runtime(),
			Planner: &continuationTestPlanner{outputs: []planner.Output{{
				Done: true, Reply: "fixture resumed without a new semantic entry",
				PlanItems:  []planner.PlanItem{{ID: "inspect", Description: "inspect project", Status: "completed", Evidence: "project.state:g-1"}},
				Completion: &planner.Completion{SatisfiedPlanIDs: []string{"inspect"}, Evidence: []string{"project.state:g-1"}},
			}}},
			Executor: continuationTestExecutor{}, Budget: agentloop.Budget{MaxTurns: 1, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		}
		resumed := resumedRunner.Continue(ctx, item.Continuation)
		if resumed.TaskID != paused.TaskID || resumed.GoalID != paused.GoalID || resumed.RunID != paused.RunID || resumed.SliceID == paused.SliceID || resumed.OriginalIntent != originalIntent {
			t.Fatalf("continuation did not create the next slice of the same task: paused=%+v resumed=%+v", paused, resumed)
		}
		return workerA.recordGoalResult(conversationID, resumed)
	}
	workerA.activeRuntimeInvocations = 0

	workerAResult := make(chan error, 1)
	go func() { workerAResult <- workerA.runContinuationSchedulerOnce(context.Background()) }()
	select {
	case <-executorEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("first scheduler did not claim the durable continuation")
	}
	// This is a distinct Server and scheduler owner. Its read/claim cycle must
	// defer while worker A holds the project-scoped durable lease.
	if err := workerB.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatalf("second scheduler should defer on an owned runtime lease: %v", err)
	}
	if executions.Load() != 1 {
		t.Fatalf("two scheduler owners executed one continuation: %d", executions.Load())
	}
	close(releaseExecutor)
	if err := <-workerAResult; err != nil {
		t.Fatalf("first scheduler completion: %v", err)
	}
	if err := workerB.runContinuationSchedulerOnce(context.Background()); err != nil {
		t.Fatalf("reloaded scheduler state: %v", err)
	}
	if executions.Load() != 1 {
		t.Fatalf("completed continuation was re-executed by a second owner: %d", executions.Load())
	}
	workerB.mu.Lock()
	workerB.activeRuntimeInvocations = 0
	workerB.mu.Unlock()
	if err := workerB.reloadActiveRuntimeState(); err != nil {
		t.Fatalf("reload final durable projection: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/agent/runtime/status", nil)
	recorder := httptest.NewRecorder()
	workerB.handleRuntimeStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET runtime status = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode GET runtime status: %v", err)
	}
	projection, ok := response["task_trajectory"].(map[string]any)
	if !ok || projection["schema_version"] != taskRuntimeTrajectorySchema {
		t.Fatalf("GET-only trajectory projection missing: %s", recorder.Body.String())
	}
	task := firstMapFromAny(projection["task"])
	run := firstMapFromAny(projection["run"])
	slices, _ := run["slices"].([]any)
	if firstStringFromMap(task, "task_id") != paused.TaskID || firstStringFromMap(task, "goal_id") != paused.GoalID || firstStringFromMap(task, "run_id") != paused.RunID || firstStringFromMap(task, "original_intent") != originalIntent || len(slices) != 2 {
		t.Fatalf("GET projection did not retain the recovered task trajectory: %+v", projection)
	}
	route, ok := workerB.capabilityRoutes[paused.TaskID]
	if !ok || route.Assessment == nil || route.Assessment.SelectedCapability != capabilityFreeState {
		t.Fatalf("capacity route was not restored: %+v", workerB.capabilityRoutes)
	}
	if loop, ok := workerB.freeStateLoop(conversationID); !ok || loop.LoopID != "loop-g-runtime" || firstStringFromMap(loop.ObservationLedger, "active_observation_id") != "obs-g-1" {
		t.Fatalf("free-state loop/observation ledger was not restored: %+v", loop)
	}
	if restored, ok := workerB.audioClosures.Load("closure-g-runtime"); !ok || restored.GoalID != paused.GoalID || restored.RunID != paused.RunID || restored.OriginalIntent != originalIntent {
		t.Fatalf("audio closure identity was not restored: %+v", restored)
	}
	data, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("runtime state was not durably persisted")
	}
}
