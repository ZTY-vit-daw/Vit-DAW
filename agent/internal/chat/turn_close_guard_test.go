package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

func orphanTaskServer(t *testing.T) (*Server, agentruntime.Goal, taskstate.Snapshot) {
	t.Helper()
	s := &Server{harness: harness.NewWithSender(nil, nil, nil)}
	goal := s.harness.EnsureGoal("goal-orphan", "run-orphan", "这个工程现在有几条轨道")
	if _, err := s.ensureAudioTaskContract("conversation-orphan", audioclosure.ModeDiagnostic, audioclosure.Scope{Kind: "project"}, "project-orphan", "rev-1", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	current := s.harness.RuntimeStatus(goal.GoalID)
	if current.Task == nil || current.Task.SemanticState == nil || current.Task.SemanticState.State != taskstate.StateObservationInProgress {
		t.Fatalf("contract admission must start at observation_in_progress: %+v", current.Task)
	}
	return s, current, *current.Task.SemanticState
}

func TestTurnEndGuardClosesOrphanObservationTask(t *testing.T) {
	s, goal, admitted := orphanTaskServer(t)
	s.harness.CompleteGoal(goal.GoalID, nil)

	s.closeOrphanTaskAtTurnEnd("conversation-orphan", goal.GoalID)

	after := s.harness.RuntimeStatus(goal.GoalID)
	if after.Task == nil || after.Task.SemanticState == nil {
		t.Fatalf("task disappeared at turn end: %+v", after)
	}
	state := after.Task.SemanticState
	if state.State != taskstate.StateClosed || !state.Terminal {
		t.Fatalf("orphan observation task was not closed: %+v", state)
	}
	last := state.History[len(state.History)-1]
	if last.From != taskstate.StateObservationInProgress || last.To != taskstate.StateClosed || last.Event != taskstate.EventOwnerTurnClosed {
		t.Fatalf("close transition history is dishonest: %+v", last)
	}
	if len(last.EvidenceRefs) != 0 {
		t.Fatalf("owner_turn_closed fabricated evidence: %+v", last.EvidenceRefs)
	}
	if state.Revision != admitted.Revision+1 {
		t.Fatalf("close must be exactly one transition: admitted=%d closed=%d", admitted.Revision, state.Revision)
	}
	if after.Status != agentruntime.StatusCompleted {
		t.Fatalf("task close rewrote the turn's goal status: %s", after.Status)
	}
}

func TestTurnEndGuardSettlesActiveClosureBoundToOrphanTask(t *testing.T) {
	s, goal, _ := orphanTaskServer(t)
	current := s.harness.RuntimeStatus(goal.GoalID)
	closure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-orphan", ConversationID: "conversation-orphan", TaskID: current.Task.TaskID, GoalID: current.GoalID, RunID: current.RunID,
		ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-orphan", ProjectRevision: "rev-1", OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeDiagnostic, Scope: audioclosure.Scope{Kind: "project"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	s.audioClosures = audioclosure.NewMemoryStore()
	if err := s.audioClosures.Create(closure); err != nil {
		t.Fatal(err)
	}

	s.harness.CompleteGoal(goal.GoalID, nil)
	s.closeOrphanTaskAtTurnEnd("conversation-orphan", goal.GoalID)

	if _, active := s.audioClosures.ActiveForConversation("conversation-orphan"); active {
		t.Fatal("closure stayed active under a terminal closed task")
	}
	stored, ok := s.audioClosures.Load("closure-orphan")
	if !ok || !stored.Terminal() || stored.Settlement == nil || stored.Settlement.Reason != audioclosure.StopOwnerTurnClosed {
		t.Fatalf("closure was not settled with the owner_turn_closed reason: %+v", stored.Settlement)
	}
}

func TestTurnEndGuardSparesTaskWithLiveContinuation(t *testing.T) {
	for _, status := range []DurableContinuationStatus{ContinuationPending, ContinuationClaimed, ContinuationRunning, ContinuationWaitingInteraction} {
		t.Run(string(status), func(t *testing.T) {
			s, goal, admitted := orphanTaskServer(t)
			current := s.harness.RuntimeStatus(goal.GoalID)
			s.mu.Lock()
			s.durableContinuations = map[string]DurableContinuation{
				"cont-orphan": {
					SchemaVersion: continuationRuntimeSchema, ContinuationID: "cont-orphan",
					TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: goal.RunID,
					ConversationID: "conversation-orphan", OriginalIntent: "inspect the project",
					Status: status, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
				},
			}
			s.mu.Unlock()

			s.harness.CompleteGoal(goal.GoalID, nil)
			s.closeOrphanTaskAtTurnEnd("conversation-orphan", goal.GoalID)

			after := s.harness.RuntimeStatus(goal.GoalID)
			if after.Task.SemanticState.State != taskstate.StateObservationInProgress || after.Task.SemanticState.Revision != admitted.Revision {
				t.Fatalf("guard advanced a task with a live %s continuation: %+v", status, after.Task.SemanticState)
			}
		})
	}
}

func TestTurnEndGuardSparesTaskWithArmedGoalContinuationOrActiveLoop(t *testing.T) {
	armed, armedGoal, armedAdmitted := orphanTaskServer(t)
	armed.mu.Lock()
	armed.goalContinuations = map[string]agentloop.Continuation{
		armedGoal.GoalID: {GoalID: armedGoal.GoalID, RunID: armedGoal.RunID, UserText: "resume the paused experiment"},
	}
	armed.mu.Unlock()
	armed.harness.CompleteGoal(armedGoal.GoalID, nil)
	armed.closeOrphanTaskAtTurnEnd("conversation-orphan", armedGoal.GoalID)
	state := armed.harness.RuntimeStatus(armedGoal.GoalID).Task.SemanticState
	if state.State != taskstate.StateObservationInProgress || state.Revision != armedAdmitted.Revision {
		t.Fatalf("guard advanced a task with an armed goal continuation: %+v", state)
	}

	looped, loopGoal, loopAdmitted := orphanTaskServer(t)
	now := time.Now().UTC()
	looped.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free_state_orphan", ConversationID: "conversation-orphan",
		GoalID: loopGoal.GoalID, Status: "reasoning", OriginalIntent: "improve the mix", CreatedAt: now, UpdatedAt: now,
	})
	looped.harness.CompleteGoal(loopGoal.GoalID, nil)
	looped.closeOrphanTaskAtTurnEnd("conversation-orphan", loopGoal.GoalID)
	state = looped.harness.RuntimeStatus(loopGoal.GoalID).Task.SemanticState
	if state.State != taskstate.StateObservationInProgress || state.Revision != loopAdmitted.Revision {
		t.Fatalf("guard advanced a task owned by an active free-state loop: %+v", state)
	}
}

func TestTurnEndGuardSkipsTerminalTask(t *testing.T) {
	s, goal, admitted := orphanTaskServer(t)
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventTaskCancelled, Reason: "user cancelled before turn end"}); err != nil {
		t.Fatal(err)
	}
	cancelled := s.harness.RuntimeStatus(goal.GoalID).Task.SemanticState

	s.harness.CompleteGoal(goal.GoalID, nil)
	s.closeOrphanTaskAtTurnEnd("conversation-orphan", goal.GoalID)

	after := s.harness.RuntimeStatus(goal.GoalID).Task.SemanticState
	if after.State != taskstate.StateCancelled || after.Revision != cancelled.Revision || after.Revision <= admitted.Revision {
		t.Fatalf("guard re-transitioned a terminal task: before=%+v after=%+v", cancelled, after)
	}
}

func TestChatTurnEndsGoalOwnershipPredicate(t *testing.T) {
	parks := []ChatResponse{
		{GoalStatus: string(agentruntime.StatusWaitingConfirmation)},
		{GoalStatus: string(agentruntime.StatusWaitingClarification)},
		{GoalStatus: string(agentruntime.StatusWaitingContinue)},
		{GoalStatus: string(agentruntime.StatusRunning)},
		{GoalStatus: string(agentruntime.StatusCompleted), NeedsConfirmation: true},
	}
	for _, resp := range parks {
		if chatTurnEndsGoalOwnership(resp) {
			t.Fatalf("parked response must keep task ownership: %+v", resp)
		}
	}
	ends := []ChatResponse{
		{GoalStatus: string(agentruntime.StatusCompleted)},
		{GoalStatus: string(agentruntime.StatusFailed)},
		{GoalStatus: string(agentruntime.StatusStopped)},
		{GoalStatus: string(agentruntime.StatusCancelled)},
		{GoalStatus: "", Error: "executor unavailable"},
	}
	for _, resp := range ends {
		if !chatTurnEndsGoalOwnership(resp) {
			t.Fatalf("turn-ending response must release task ownership: %+v", resp)
		}
	}
}
