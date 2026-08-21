package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
)

func TestRuntimeStatusProjectsDurableTaskTrajectory(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	runtime := server.harness.Runtime()
	goal := server.harness.EnsureGoal("goal-trajectory", "run-trajectory", "检查一下当前工程有什么问题？")
	goal, firstSlice, ok := runtime.BeginSlice(goal.GoalID, 4, 12)
	if !ok {
		t.Fatal("first invocation slice was not created")
	}
	goal, firstTurn, ok := runtime.BeginTurn(goal.GoalID, firstSlice.SliceID, "user")
	if !ok {
		t.Fatal("first turn was not created")
	}
	goal = runtime.EndTurn(goal.GoalID, firstTurn.TurnID, "waiting_continuation", 4, 12)
	goal = server.harness.SetGoalStatus(goal.GoalID, "waiting_continue", nil)
	contract := taskstate.Contract{
		ConversationID: "conversation-trajectory", Kind: taskstate.ContractDiagnostic,
		Scope: taskstate.Scope{Kind: "project"}, AuthorizationBoundary: "observe_and_propose_only",
		CompletionCriteria: []string{"evidence-backed bounded diagnosis"}, EvidenceRequirements: []string{"observation receipt"},
		CreatedAt: time.Now().UTC(),
	}
	goal, err := server.harness.EnsureTaskContract(goal.GoalID, contract)
	if err != nil {
		t.Fatalf("ensure contract: %v", err)
	}
	if goal.Task == nil || goal.Task.SemanticState == nil {
		t.Fatal("task semantic state was not persisted")
	}
	secondGoal, secondSlice, ok := runtime.BeginSlice(goal.GoalID, 4, 12)
	if !ok || secondSlice.SliceID == firstSlice.SliceID {
		t.Fatalf("second invocation slice was not created: %+v", secondSlice)
	}
	server.durableContinuations["cont-trajectory"] = DurableContinuation{
		ContinuationID: "cont-trajectory", TaskID: secondGoal.Task.TaskID, GoalID: secondGoal.GoalID,
		ConversationID: "conversation-trajectory", RunID: secondGoal.RunID, CurrentSliceID: secondSlice.SliceID,
		OriginalIntent: secondGoal.Task.OriginalIntent, Status: ContinuationPending, UpdatedAt: time.Now().UTC(),
	}
	request := httptest.NewRequest(http.MethodGet, "/agent/runtime/status", nil)
	recorder := httptest.NewRecorder()
	server.handleRuntimeStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("runtime status code = %d", recorder.Code)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode runtime status: %v", err)
	}
	projection, ok := response["task_trajectory"].(map[string]any)
	if !ok {
		t.Fatalf("task trajectory projection missing: %s", recorder.Body.String())
	}
	if projection["schema_version"] != taskRuntimeTrajectorySchema {
		t.Fatalf("schema = %v", projection["schema_version"])
	}
	task := projection["task"].(map[string]any)
	if task["task_id"] != secondGoal.Task.TaskID || task["goal_id"] != secondGoal.GoalID || task["run_id"] != secondGoal.RunID || task["original_intent"] != "检查一下当前工程有什么问题？" {
		t.Fatalf("task identity/original intent = %+v", task)
	}
	run := projection["run"].(map[string]any)
	if len(run["slices"].([]any)) != 2 {
		t.Fatalf("run slices = %+v", run["slices"])
	}
	continuation := projection["continuation"].(map[string]any)
	if continuation["status"] != string(ContinuationPending) || continuation["continuation_id"] != "cont-trajectory" {
		t.Fatalf("continuation = %+v", continuation)
	}
}
