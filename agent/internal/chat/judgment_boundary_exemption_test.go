package chat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/logx"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

// judgmentBoundaryServerForTest rebuilds the DIAG Q2 repro shape at unit
// scale: a canonical Task parked at human_judgment_required with the
// free-state loop durably parked at the judgment boundary (the blocked
// residency), an audition session snapshot ready, and the round not yet
// carrying a judgment request — the exact mid-write window state the chain
// end previously orphan-closed (2026-09-09 B1-DIAG, F1 task card).
func judgmentBoundaryServerForTest(t *testing.T, logger *logx.Logger) (*Server, freeStateReasoningLoop, taskstate.Snapshot) {
	t.Helper()
	s := New(nil, nil, logger)
	loop := auditionReadyLoop(t)
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"after"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	goal := s.harness.EnsureGoal(loop.GoalID, loop.RunID, loop.OriginalIntent)
	if _, err := s.ensureAudioTaskContract(loop.ConversationID, audioclosure.ModeTreatment, audioclosure.Scope{Kind: "project", ID: "project-1"}, "project-1", "rev-7", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	proposal := taskstate.BoundedProposal{ProposalID: "proposal-judgment-boundary", Summary: "bounded track gain", EvidenceRefs: []string{"before"}, RequiresExperiment: true}
	if _, err := s.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventImprovementProposed, Reason: "candidate observed", EvidenceRefs: []string{"before"}, CandidateID: "1007", Proposal: &proposal, ProjectRevision: "rev-7"}); err != nil {
		t.Fatal(err)
	}
	if err := s.bindExperimentSemantic(&loop); err != nil {
		t.Fatal(err)
	}
	// The durable boundary shape (DIAG Q2): the round decision
	// user_judgment_pending is recorded while the audition session snapshot
	// only reaches ready afterwards, so the round carries no judgment request
	// yet and the judgment POST later serves itself through the bind path.
	if _, err := loop.Experiment.DecideRound(experiment.DecisionUserJudgment, "awaiting human A/B judgment", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	loop.AuditionSessionID = "session-judgment-boundary"
	loop.AuditionSessionSnapshot = map[string]any{
		"session_id": loop.AuditionSessionID, "conversation_id": loop.ConversationID, "turn_id": loop.Experiment.ID,
		"round_id": loop.Experiment.Rounds[0].ID, "status": "ready", "scope": "target", "project_revision": "rev-7",
		"candidates": []any{
			map[string]any{"id": "candidate-a", "status": "ready", "source_ref": "checkpoint:7", "preview_ref": "preview:a"},
			map[string]any{"id": "candidate-b", "status": "ready", "source_ref": "action:7", "preview_ref": "preview:b"},
		},
	}
	if err := s.requireTaskHumanJudgment(&loop, loop.AuditionSessionID, "A/B audition judgment is required before experiment settlement"); err != nil {
		t.Fatal(err)
	}
	// The DIAG Q2 residency: recording the judgment request parks the loop
	// with the blocked status so ordinary model turns cannot revive it.
	loop.Status = "blocked"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	current := s.harness.RuntimeStatus(goal.GoalID)
	if current.Task == nil || current.Task.SemanticState == nil || current.Task.SemanticState.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("setup: task must be parked at human_judgment_required: %+v", current.Task)
	}
	if !freeStateJudgmentBoundary(loop) || freeStateLoopActive(loop) {
		t.Fatalf("setup: loop must be a blocked judgment boundary: active=%v boundary=%v", freeStateLoopActive(loop), freeStateJudgmentBoundary(loop))
	}
	return s, loop, *current.Task.SemanticState
}

func judgmentBoundaryHistoryHasOwnerTurnClosed(state taskstate.Snapshot) bool {
	for _, record := range state.History {
		if record.Event == taskstate.EventOwnerTurnClosed {
			return true
		}
	}
	return false
}

// F1 pin (1): both chain-end hosts (HTTP finalize turn guard and the
// scheduler settle sibling) must leave the semantic history parked at
// human_judgment_required with zero owner_turn_closed transitions.
func TestChainEndSparesJudgmentBoundaryTask(t *testing.T) {
	httpServer, httpLoop, admitted := judgmentBoundaryServerForTest(t, nil)
	httpServer.harness.CompleteGoal(httpLoop.GoalID, nil)
	httpServer.closeOrphanTaskAtTurnEnd(httpLoop.ConversationID, httpLoop.GoalID)
	state := httpServer.harness.RuntimeStatus(httpLoop.GoalID).Task.SemanticState
	if state == nil || state.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("turn-end guard closed a task parked at the judgment boundary: %+v", state)
	}
	if state.Revision != admitted.Revision || judgmentBoundaryHistoryHasOwnerTurnClosed(*state) {
		t.Fatalf("judgment boundary must not record owner_turn_closed: admitted_rev=%d state=%+v", admitted.Revision, state)
	}

	schedulerServer, schedulerLoop, schedulerAdmitted := judgmentBoundaryServerForTest(t, nil)
	schedulerServer.harness.SetGoalStatus(schedulerLoop.GoalID, agentruntime.StatusWaitingConfirmation, nil)
	schedulerServer.settleGoalAfterContinuationEnd(schedulerLoop.ConversationID, schedulerLoop.GoalID)
	schedulerState := schedulerServer.harness.RuntimeStatus(schedulerLoop.GoalID).Task.SemanticState
	if schedulerState == nil || schedulerState.State != taskstate.StateHumanJudgmentRequired {
		t.Fatalf("continuation-end settle closed a task parked at the judgment boundary: %+v", schedulerState)
	}
	if schedulerState.Revision != schedulerAdmitted.Revision || judgmentBoundaryHistoryHasOwnerTurnClosed(*schedulerState) {
		t.Fatalf("scheduler variant must not record owner_turn_closed: admitted_rev=%d state=%+v", schedulerAdmitted.Revision, schedulerState)
	}
	if status := schedulerServer.harness.RuntimeStatus(schedulerLoop.GoalID).Status; status != agentruntime.StatusWaitingConfirmation {
		t.Fatalf("judgment park must keep the goal answerable, got %s", status)
	}
}

// F1 pin (2): after the chain end fires, the audition judgment POST must
// still be servable (200) — the pre-fix world orphan-closed the task and the
// same POST died with 409 forever.
func TestJudgmentPOSTServableAfterChainEnd(t *testing.T) {
	s, loop, _ := judgmentBoundaryServerForTest(t, nil)
	s.harness.CompleteGoal(loop.GoalID, nil)
	s.closeOrphanTaskAtTurnEnd(loop.ConversationID, loop.GoalID)

	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"conversation_id":%q,"turn_id":%q,"round_id":%q,"audition_session_id":%q,"project_revision":"rev-7","heard_difference":"yes","preference":"b","reason_tags":["更自然"]}`, loop.ConversationID, loop.Experiment.ID, round.ID, loop.AuditionSessionID)
	recorder := httptest.NewRecorder()
	s.handleAuditionJudgment(recorder, httptest.NewRequest(http.MethodPost, "/agent/audition/judgment", strings.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("judgment POST after chain end: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil || len(stored.Experiment.Rounds[0].UserJudgmentEvidence) != 1 {
		t.Fatalf("judgment evidence did not land: %+v", stored)
	}
	state := s.harness.RuntimeStatus(loop.GoalID).Task.SemanticState
	if judgmentBoundaryHistoryHasOwnerTurnClosed(*state) {
		t.Fatalf("servable judgment boundary was closed underneath the POST: %+v", state)
	}
}

// F1 pin (3): the exemption is judgment-boundary-specific. An ordinary
// blocked loop — with no experiment, or with an already settled experiment
// behind a capability boundary — must still be orphan-closed, and the
// boundary loop's blocked residency must keep presenting as inactive so the
// ordinary resume paths stay dead (anti-revival intact).
func TestTurnEndGuardStillClosesOrdinaryBlockedLoops(t *testing.T) {
	bare, bareGoal, bareAdmitted := orphanTaskServer(t)
	now := time.Now().UTC()
	bare.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free_state_ordinary_blocked", ConversationID: "conversation-orphan",
		GoalID: bareGoal.GoalID, Status: "blocked", LastError: "free_state_observation_target_unresolved",
		OriginalIntent: "improve the mix", CreatedAt: now, UpdatedAt: now,
	})
	bare.harness.CompleteGoal(bareGoal.GoalID, nil)
	bare.closeOrphanTaskAtTurnEnd("conversation-orphan", bareGoal.GoalID)
	state := bare.harness.RuntimeStatus(bareGoal.GoalID).Task.SemanticState
	if state == nil || state.State != taskstate.StateClosed || !state.Terminal || state.Revision != bareAdmitted.Revision+1 {
		t.Fatalf("guard must still close an ordinary blocked loop: %+v", state)
	}

	// A blocked loop whose experiment already settled behind a capability
	// boundary presents no live judgment boundary; its non-terminal task must
	// still be closed. The experiment here is contract-unbound, so Settle does
	// not require a canonical transition (the bound variant reaches
	// capability_blocked — a terminal canonical state the guard skips anyway).
	unbound, unboundGoal, unboundAdmitted := orphanTaskServer(t)
	unboundLoop := auditionReadyLoop(t)
	unboundLoop.ConversationID = "conversation-orphan"
	unboundLoop.GoalID = unboundGoal.GoalID
	unboundLoop.RunID = unboundGoal.RunID
	if _, err := unboundLoop.Experiment.Settle(experiment.OutcomeBlockedCapability, "capability boundary behind the round", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	unboundLoop.Status = "blocked"
	unbound.storeFreeStateLoop(unboundLoop)
	if freeStateJudgmentBoundary(unboundLoop) {
		t.Fatal("setup: a settled experiment must not present as a live judgment boundary")
	}
	unbound.closeOrphanTaskAtTurnEnd(unboundLoop.ConversationID, unboundGoal.GoalID)
	unboundState := unbound.harness.RuntimeStatus(unboundGoal.GoalID).Task.SemanticState
	if unboundState == nil || unboundState.State != taskstate.StateClosed || unboundState.Revision != unboundAdmitted.Revision+1 {
		t.Fatalf("guard must still close a capability-blocked loop with a settled experiment: %+v", unboundState)
	}

	resumeServer, resumeLoop, _ := judgmentBoundaryServerForTest(t, nil)
	storedBoundary, ok := resumeServer.freeStateLoop(resumeLoop.ConversationID)
	if !ok || !freeStateJudgmentBoundary(storedBoundary) || freeStateLoopActive(storedBoundary) {
		t.Fatalf("the judgment boundary's blocked residency must keep presenting as inactive to the resume paths: ok=%v active=%v boundary=%v", ok, freeStateLoopActive(storedBoundary), freeStateJudgmentBoundary(storedBoundary))
	}
}

// F1 pin (4): the audition.ready double-fire (prepare path + kernel telemetry
// path) must converge on one durably parked judgment request instead of the
// latecomer dying as a "canonical human judgment transition rejected" WARN.
// The helper leaves the task at human_judgment_required with the round not
// yet requested — the exact mid-write window the telemetry latecomer hits.
func TestAuditionReadyDoubleFireJudgmentRequestDeduplicates(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent.log")
	s, loop, _ := judgmentBoundaryServerForTest(t, logx.New(false, logPath, 64))

	s.requestAuditionJudgment(loop.ConversationID, loop.AuditionSessionID)

	stored, ok := s.freeStateLoop(loop.ConversationID)
	if !ok || stored.Experiment == nil {
		t.Fatal("stored loop disappeared")
	}
	round, err := stored.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	if !round.UserJudgmentRequested || len(round.UserJudgmentEvidence) != 0 {
		t.Fatalf("the double-fire latecomer must land the judgment request exactly once: %+v", round)
	}
	requested := 0
	events, _ := s.agentEventsSince(loop.ConversationID, 0, 100)
	for _, event := range events {
		if event.Type == string(trajectory.EventUserJudgmentRequested) {
			requested++
		}
	}
	if requested != 1 {
		t.Fatalf("judgment request events=%d, want exactly 1 (dedup): %+v", requested, events)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	// A missing log file is itself the success shape: logx only writes the
	// file once a line is emitted, and the deduplicated double-fire emits none.
	if strings.Contains(string(logged), "canonical human judgment transition rejected") {
		t.Fatalf("double-fire still surfaces the transition-rejection WARN: %s", logged)
	}
	state := s.harness.RuntimeStatus(loop.GoalID).Task.SemanticState
	if state == nil || state.State != taskstate.StateHumanJudgmentRequired || judgmentBoundaryHistoryHasOwnerTurnClosed(*state) {
		t.Fatalf("dedup path must keep the boundary parked at human_judgment_required: %+v", state)
	}
}
