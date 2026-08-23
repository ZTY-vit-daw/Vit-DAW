package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

func TestDiagnosticTaskSettlesWithoutExperimentRuntime(t *testing.T) {
	s := &Server{harness: harness.NewWithSender(nil, nil, nil)}
	goal := s.harness.EnsureGoal("goal-diagnostic", "run-diagnostic", "check the current project")
	context, err := s.ensureAudioTaskContract("conversation-diagnostic", audioclosure.ModeDiagnostic, audioclosure.Scope{Kind: "project"}, "project-diagnostic", "revision-diagnostic", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	if context[taskContractContextKey] == nil || context[taskSemanticContextKey] == nil {
		t.Fatalf("diagnostic contract was not projected into runtime context: %+v", context)
	}
	loop := freeStateReasoningLoop{GoalID: goal.GoalID, ConversationID: "conversation-diagnostic"}
	decision := agentloop.FreeStateDecision{Status: agentloop.FreeStateDiagnosticComplete, Summary: "bounded diagnosis complete", ObservationID: "observation://project"}
	if err := s.applyFreeStateDecisionSemantic(&loop, decision); err != nil {
		t.Fatal(err)
	}
	if err := s.settleDiagnosticTask(&loop, decision); err != nil {
		t.Fatal(err)
	}
	settled := s.harness.RuntimeStatus(goal.GoalID)
	if settled.Task == nil || settled.Task.SemanticState == nil || settled.Task.SemanticState.State != taskstate.StateSettled {
		t.Fatalf("diagnostic task did not settle through canonical state: %+v", settled)
	}
}

func TestOpenImprovementContractRejectsSatisfiedShortcut(t *testing.T) {
	s := &Server{harness: harness.NewWithSender(nil, nil, nil)}
	goal := s.harness.EnsureGoal("goal-improvement", "run-improvement", "improve the current project")
	_, err := s.ensureAudioTaskContract("conversation-improvement", audioclosure.ModeTreatment, audioclosure.Scope{Kind: "project"}, "project-improvement", "revision-improvement", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	loop := freeStateReasoningLoop{GoalID: goal.GoalID, ConversationID: "conversation-improvement"}
	decision := agentloop.FreeStateDecision{Status: agentloop.FreeStateSatisfied, Summary: "one dimension has enough evidence", ObservationID: "observation://one-dimension"}
	if err := s.applyFreeStateDecisionSemantic(&loop, decision); err == nil {
		t.Fatal("open improvement contract accepted satisfied as a terminal shortcut")
	}
	current := s.harness.RuntimeStatus(goal.GoalID)
	if current.Task == nil || current.Task.SemanticState == nil || current.Task.SemanticState.State != taskstate.StateObservationInProgress {
		t.Fatalf("rejected satisfied shortcut changed canonical state: %+v", current)
	}
}

func TestHumanJudgmentRestartRestoresSameTaskRunClosureAndExperiment(t *testing.T) {
	source := New(nil, nil, nil)
	loop := auditionReadyLoop(t)
	goal := source.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	_, err := source.ensureAudioTaskContract(loop.ConversationID, audioclosure.ModeTreatment, audioclosure.Scope{Kind: "project", ID: "project-1"}, "project-1", "rev-7", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	proposal := taskstate.BoundedProposal{ProposalID: "proposal-restart", Summary: "bounded candidate", EvidenceRefs: []string{"before"}, RequiresExperiment: true}
	if _, err = source.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventImprovementProposed, Reason: "candidate observed", EvidenceRefs: []string{"before"}, CandidateID: "track-observed", Proposal: &proposal, ProjectRevision: "rev-7"}); err != nil {
		t.Fatal(err)
	}
	if err = source.bindExperimentSemantic(&loop); err != nil {
		t.Fatal(err)
	}
	if _, err = loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-restart"
	loop.AuditionSessionSnapshot = map[string]any{"session_id": loop.AuditionSessionID, "status": "ready", "project_revision": "rev-7"}
	if err = source.requireTaskHumanJudgment(&loop, loop.AuditionSessionID, "A/B audition judgment required"); err != nil {
		t.Fatal(err)
	}
	if _, err = loop.Experiment.RequestUserJudgmentForSession("compare A/B", loop.AuditionSessionID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	current := source.harness.RuntimeStatus(goal.GoalID)
	closure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-restart", ConversationID: loop.ConversationID, TaskID: current.Task.TaskID, GoalID: current.GoalID, RunID: current.RunID,
		ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-1", ProjectRevision: "rev-7", OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-1"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.audioClosures.Create(closure); err != nil {
		t.Fatal(err)
	}
	source.storeFreeStateLoop(loop)
	paused := waitingContinuationResult("slice-restart", "turn-restart", agentruntime.StatusWaitingConfirmation, "human_judgment_required")
	paused.GoalID, paused.RunID, paused.TaskID, paused.OriginalIntent = current.GoalID, current.RunID, current.Task.TaskID, current.Task.OriginalIntent
	paused.Continuation.GoalID, paused.Continuation.RunID, paused.Continuation.TaskID, paused.Continuation.OriginalIntent = current.GoalID, current.RunID, current.Task.TaskID, current.Task.OriginalIntent
	if err = source.recordGoalResult(loop.ConversationID, paused); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	persisted := source.projectAgentRuntimeStateLocked()
	source.mu.Unlock()

	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(persisted)
	restarted.mu.Unlock()
	restoredGoal := restarted.harness.RuntimeStatus(goal.GoalID)
	restoredLoop, ok := restarted.freeStateLoop(loop.ConversationID)
	restoredClosure, closureOK := restarted.audioClosures.Load(closure.ClosureID)
	continuations := restarted.continuationRuntimeProjection()
	if restoredGoal.Task == nil || restoredGoal.Task.TaskID != current.Task.TaskID || restoredGoal.RunID != current.RunID || restoredGoal.Task.SemanticState == nil || restoredGoal.Task.SemanticState.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("task/goal/run semantic identity changed after restart: %+v", restoredGoal)
	}
	if !ok || restoredLoop.Experiment == nil || restoredLoop.Experiment.ID != loop.Experiment.ID || restoredLoop.Experiment.TaskState != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("experiment identity or canonical state changed after restart: %+v", restoredLoop.Experiment)
	}
	if !closureOK || restoredClosure.TaskID != current.Task.TaskID || restoredClosure.ContractID != current.Task.Contract.ContractID || restoredClosure.TaskState != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("closure identity or canonical state changed after restart: %+v", restoredClosure)
	}
	if len(continuations) != 1 || continuations[0]["task_id"] != current.Task.TaskID || continuations[0]["status"] != ContinuationWaitingInteraction || continuations[0]["task_state"] != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("pending interaction continuation was not restorable: %+v", continuations)
	}
}

func TestRestoreTerminalAudioClosureDoesNotAppendProjectionEvent(t *testing.T) {
	source := New(nil, nil, nil)
	goal := source.harness.EnsureGoal("goal-terminal-closure", "run-terminal-closure", "inspect the project")
	_, err := source.ensureAudioTaskContract("conversation-terminal-closure", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-terminal"}, "project-terminal", "rev-1", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	current := source.harness.RuntimeStatus(goal.GoalID)
	closure, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-terminal", ConversationID: "conversation-terminal-closure", TaskID: current.Task.TaskID,
		GoalID: current.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-terminal", ProjectRevision: "rev-1", OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-terminal"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	closure, err = (audioclosure.Driver{}).Settle(closure, closure.Revision, audioclosure.StopCancelled, "test terminal closure", false, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := source.audioClosures.Create(closure); err != nil {
		t.Fatal(err)
	}
	snapshot := source.projectAgentRuntimeStateLocked()
	restored := New(nil, nil, nil)
	restored.restoreProjectAgentRuntimeStateLocked(snapshot)
	got, ok := restored.audioClosures.Load(closure.ClosureID)
	if !ok || !got.Terminal() {
		t.Fatalf("terminal closure was not restored: ok=%v state=%+v", ok, got)
	}
	if got.Revision != closure.Revision {
		t.Fatalf("terminal closure revision changed during recovery: got=%d want=%d", got.Revision, closure.Revision)
	}
	goalAfter := restored.harness.RuntimeStatus(goal.GoalID)
	if goalAfter.Status == agentruntime.StatusWaitingClarification && strings.Contains(goalAfter.Error, "audio closure task projection") {
		t.Fatalf("terminal closure recovery entered a projection conflict: %+v", goalAfter)
	}
}
