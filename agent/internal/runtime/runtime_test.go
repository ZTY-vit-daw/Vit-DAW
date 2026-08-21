package runtime

import (
	"testing"

	"vit-daw-agent/internal/taskstate"
)

func TestInterjectionOrderingAndCancelTick(t *testing.T) {
	rt := New()
	goal := rt.Create("mix pass")
	rt.AddInterjection(goal.GoalID, "pause", "user")
	rt.AddInterjection(goal.GoalID, "try softer", "user")
	status := rt.Status(goal.GoalID)
	if len(status.PendingInterjection) != 2 || status.PendingInterjection[0].Sequence >= status.PendingInterjection[1].Sequence {
		t.Fatalf("interjections not ordered: %+v", status.PendingInterjection)
	}
	rt.Cancel(goal.GoalID, "user cancelled")
	cancelled := rt.Tick(goal.GoalID, "after tool")
	if cancelled.Status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", cancelled.Status)
	}
}

func TestSnapshotRestoreReplacesActiveGoalSet(t *testing.T) {
	source := New()
	first := source.Ensure("goal_a", "run_a", "project A")
	source.AddInterjection(first.GoalID, "pause here", "user")
	source.SetStatus(first.GoalID, StatusWaitingContinue, nil)
	snapshot := source.Snapshot()

	target := New()
	target.Ensure("goal_other", "run_other", "other project")
	target.Restore(snapshot)
	restored := target.Status("")
	if restored.GoalID != first.GoalID || restored.RunID != first.RunID || restored.Status != StatusWaitingContinue {
		t.Fatalf("restored goal=%+v", restored)
	}
	if len(restored.PendingInterjection) != 1 || restored.PendingInterjection[0].Message != "pause here" {
		t.Fatalf("restored interjections=%+v", restored.PendingInterjection)
	}
	if other := target.Status("goal_other"); other.Status != StatusIdle {
		t.Fatalf("foreign goal survived restore: %+v", other)
	}
	target.Restore(Snapshot{})
	if active := target.Status(""); active.Status != StatusIdle || active.GoalID != "" {
		t.Fatalf("empty snapshot did not clear runtime: %+v", active)
	}
}

func TestRequestStopFinishesAtCheckpointAndRestoresAcrossSnapshot(t *testing.T) {
	rt := New()
	goal := rt.Create("free-state experiment")
	rt.SetStatus(goal.GoalID, StatusExecuting, nil)
	requested := rt.RequestStop(goal.GoalID, "user_stop")
	if requested.Status != StatusCancelling || !requested.StopRequested {
		t.Fatalf("requested=%+v", requested)
	}
	if _, blocked := rt.CheckoutBlocked(); !blocked {
		t.Fatal("checkout should remain blocked while cancelling")
	}
	stopped := rt.Tick(goal.GoalID, "checkpoint-stable-1")
	if stopped.Status != StatusStopped || stopped.LastCheckpoint != "checkpoint-stable-1" {
		t.Fatalf("stopped=%+v", stopped)
	}
	if _, blocked := rt.CheckoutBlocked(); blocked {
		t.Fatal("checkout should be allowed after stop")
	}
	snapshot := rt.Snapshot()
	restored := New()
	restored.Restore(snapshot)
	got := restored.Status(goal.GoalID)
	if got.Status != StatusStopped || got.LastCheckpoint != "checkpoint-stable-1" || !got.StopRequested {
		t.Fatalf("restored=%+v", got)
	}
}

func TestCheckoutBlockedOnlyForActiveExecutionStates(t *testing.T) {
	for _, status := range []GoalStatus{StatusRunning, StatusProcessing, StatusExecuting, StatusCancelling} {
		rt := New()
		goal := rt.Create("guard")
		rt.SetStatus(goal.GoalID, status, nil)
		if _, blocked := rt.CheckoutBlocked(); !blocked {
			t.Fatalf("status=%s was not blocked", status)
		}
	}
	for _, status := range []GoalStatus{StatusStopped, StatusStable, StatusCompleted, StatusFailed, StatusCancelled, StatusWaitingContinue, StatusWaitingConfirmation} {
		rt := New()
		goal := rt.Create("guard")
		rt.SetStatus(goal.GoalID, status, nil)
		if _, blocked := rt.CheckoutBlocked(); blocked {
			t.Fatalf("status=%s was blocked", status)
		}
	}
}

func TestTaskRunSliceIdentitySurvivesContinuation(t *testing.T) {
	rt := New()
	created := rt.Create("inspect the current project")
	if created.Task == nil || created.Task.TaskID == "" {
		t.Fatalf("task identity missing: %+v", created)
	}
	if created.Task.OriginalIntent != "inspect the current project" {
		t.Fatalf("original intent = %q", created.Task.OriginalIntent)
	}
	opened, firstSlice, ok := rt.BeginSlice(created.GoalID, 2, 4)
	if !ok || opened.Task == nil || firstSlice.MaxTurns != 2 || firstSlice.MaxToolCalls != 4 {
		t.Fatalf("slice open failed: goal=%+v slice=%+v ok=%v", opened, firstSlice, ok)
	}
	withTurn, firstTurn, ok := rt.BeginTurn(created.GoalID, firstSlice.SliceID, "user")
	if !ok || firstTurn.RunID != created.RunID || firstTurn.SliceID != firstSlice.SliceID {
		t.Fatalf("turn identity mismatch: goal=%+v turn=%+v ok=%v", withTurn, firstTurn, ok)
	}
	rt.SetStatus(created.GoalID, StatusWaitingContinue, nil)
	continued := rt.Continue(created.GoalID, "continuation summary must not replace intent")
	if continued.RunID != created.RunID {
		t.Fatalf("run identity changed across continuation: before=%s after=%s", created.RunID, continued.RunID)
	}
	if continued.Task == nil || continued.Task.TaskID != created.Task.TaskID {
		t.Fatalf("task identity changed: before=%s after=%+v", created.Task.TaskID, continued.Task)
	}
	if continued.Task.OriginalIntent != created.Task.OriginalIntent {
		t.Fatalf("original intent changed: %q", continued.Task.OriginalIntent)
	}
	if len(continued.Task.Run.Slices) != 2 || continued.Task.Run.Slices[1].Sequence != 2 {
		t.Fatalf("continuation did not create slice 2: %+v", continued.Task.Run.Slices)
	}
	if continued.Task.Run.Slices[0].Status != "waiting_continuation" {
		t.Fatalf("slice 1 status = %q", continued.Task.Run.Slices[0].Status)
	}
}

func TestTaskRunSliceSnapshotRestoreKeepsStableIdentity(t *testing.T) {
	source := New()
	goal := source.Create("preserve task identity")
	_, slice, ok := source.BeginSlice(goal.GoalID, 3, 5)
	if !ok {
		t.Fatal("failed to open slice")
	}
	source.BeginTurn(goal.GoalID, slice.SliceID, "user")
	snapshot := source.Snapshot()
	target := New()
	target.Restore(snapshot)
	restored := target.Status(goal.GoalID)
	if restored.Task == nil || restored.Task.TaskID != goal.Task.TaskID || restored.RunID != goal.RunID {
		t.Fatalf("identity not restored: original=%+v restored=%+v", goal, restored)
	}
	if len(restored.Task.Run.Slices) != 1 || len(restored.Task.Run.Turns) != 1 {
		t.Fatalf("slice/turn history not restored: %+v", restored.Task.Run)
	}
}

func TestCrashRecoveryClosesInterruptedSliceBeforeOpeningNext(t *testing.T) {
	source := New()
	goal := source.Create("recover interrupted invocation")
	_, firstSlice, ok := source.BeginSlice(goal.GoalID, 3, 5)
	if !ok {
		t.Fatal("failed to open initial slice")
	}
	_, firstTurn, ok := source.BeginTurn(goal.GoalID, firstSlice.SliceID, "automatic_continuation")
	if !ok {
		t.Fatal("failed to open initial turn")
	}
	restarted := New()
	restarted.Restore(source.Snapshot())
	continued := restarted.Continue(goal.GoalID, "resume after crash")
	if continued.Task == nil || len(continued.Task.Run.Slices) != 2 {
		t.Fatalf("recovery did not open a new slice: %+v", continued)
	}
	if continued.Task.Run.Slices[0].Status != "interrupted" || continued.Task.Run.Slices[0].EndedAt.IsZero() {
		t.Fatalf("crashed slice remained running: %+v", continued.Task.Run.Slices[0])
	}
	if len(continued.Task.Run.Turns) != 1 || continued.Task.Run.Turns[0].TurnID != firstTurn.TurnID || continued.Task.Run.Turns[0].Status != "interrupted" || continued.Task.Run.Turns[0].EndedAt.IsZero() {
		t.Fatalf("crashed turn remained running: %+v", continued.Task.Run.Turns)
	}
	if continued.Task.Run.CurrentSliceID == firstSlice.SliceID || continued.Task.Run.CurrentTurnID != "" {
		t.Fatalf("recovery reused interrupted invocation identity: %+v", continued.Task.Run)
	}
}

func TestLegacySnapshotHydratesTaskWithoutChangingRunIdentity(t *testing.T) {
	rt := New()
	rt.Restore(Snapshot{Goals: []Goal{{GoalID: "legacy_goal", RunID: "legacy_run", Summary: "legacy intent", Status: StatusWaitingContinue}}})
	legacy := rt.Status("legacy_goal")
	if legacy.Task == nil || legacy.Task.TaskID == "" || legacy.Task.Run.RunID != "legacy_run" {
		t.Fatalf("legacy goal was not hydrated: %+v", legacy)
	}
	continued := rt.Continue("legacy_goal", "resume")
	if continued.RunID != "legacy_run" || continued.Task == nil || continued.Task.OriginalIntent != "legacy intent" {
		t.Fatalf("legacy continuation changed identity or intent: %+v", continued)
	}
}

func TestEnsureDoesNotReplaceExistingRunIdentity(t *testing.T) {
	rt := New()
	created := rt.Ensure("goal_stable", "run_first", "stable intent")
	repeated := rt.Ensure("goal_stable", "run_other", "new summary")
	if repeated.RunID != created.RunID {
		t.Fatalf("Ensure replaced stable RunID: first=%s repeated=%s", created.RunID, repeated.RunID)
	}
	if repeated.Task == nil || repeated.Task.OriginalIntent != created.Task.OriginalIntent {
		t.Fatalf("Ensure replaced task intent: %+v", repeated.Task)
	}
}

func TestTaskSemanticContractSurvivesSnapshotRestore(t *testing.T) {
	source := New()
	goal := source.Ensure("goal_semantic", "run_semantic", "inspect and improve the project")
	goal, err := source.EnsureTaskContract(goal.GoalID, taskstate.Contract{
		ConversationID: "conversation_semantic", Kind: taskstate.ContractImprovement, Scope: taskstate.Scope{Kind: "project"},
		Temporary: true, TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria: []string{"governed outcome"}, EvidenceRequirements: []string{"observation reference"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if goal.Task.ConversationID != "conversation_semantic" {
		t.Fatalf("task conversation identity was not bound: %+v", goal.Task)
	}
	target := New()
	target.Restore(source.Snapshot())
	restored := target.Status(goal.GoalID)
	if restored.Task == nil || restored.Task.Contract == nil || restored.Task.SemanticState == nil || restored.Task.SemanticState.State != taskstate.StateObservationInProgress || restored.Status == StatusWaitingClarification {
		t.Fatalf("valid semantic state did not restore: %+v", restored)
	}
}

func TestTaskSemanticRestoreFailsClosedOnConversationMismatch(t *testing.T) {
	source := New()
	goal := source.Ensure("goal_semantic_mismatch", "run_semantic_mismatch", "inspect the project")
	_, err := source.EnsureTaskContract(goal.GoalID, taskstate.Contract{
		ConversationID: "conversation_authority", Kind: taskstate.ContractDiagnostic, Scope: taskstate.Scope{Kind: "project"},
		AuthorizationBoundary: "observe_only", CompletionCriteria: []string{"bounded diagnosis"}, EvidenceRequirements: []string{"observation reference"},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := source.Snapshot()
	snapshot.Goals[0].Task.ConversationID = "different_conversation"
	target := New()
	target.Restore(snapshot)
	restored := target.Status(goal.GoalID)
	if restored.Status != StatusWaitingClarification || restored.Task == nil || restored.Task.Status != TaskStatusWaitingInteraction {
		t.Fatalf("corrupt semantic identity remained executable: %+v", restored)
	}
}

func TestTaskSchedulingStatusRemainsOrthogonalToSemanticProgress(t *testing.T) {
	runtime := New()
	goal := runtime.Ensure("goal_scheduling", "run_scheduling", "inspect and improve")
	_, err := runtime.EnsureTaskContract(goal.GoalID, taskstate.Contract{
		ConversationID: "conversation_scheduling", Kind: taskstate.ContractImprovement, Scope: taskstate.Scope{Kind: "project"},
		Temporary: true, TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria: []string{"governed outcome"}, EvidenceRequirements: []string{"observation reference"},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitingContinue := runtime.SetStatus(goal.GoalID, StatusWaitingContinue, nil)
	if waitingContinue.Task == nil || waitingContinue.Task.Status != TaskStatusWaitingContinuation || waitingContinue.Task.SemanticState.State != taskstate.StateObservationInProgress {
		t.Fatalf("continuation scheduling state overwrote semantic progress: %+v", waitingContinue.Task)
	}
	waitingUser := runtime.SetStatus(goal.GoalID, StatusWaitingClarification, nil)
	if waitingUser.Task.Status != TaskStatusWaitingInteraction || waitingUser.Task.SemanticState.State != taskstate.StateObservationInProgress {
		t.Fatalf("interaction scheduling state overwrote semantic progress: %+v", waitingUser.Task)
	}
}
