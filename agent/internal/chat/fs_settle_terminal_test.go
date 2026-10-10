package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/orchestrationcontroller"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FS-SETTLE-TERMINAL-1 regression: the 2026-09-30 20:09 live failure
// (goal_4e4c14a9 / run_bea18cde, draft Unsaved.vit). The settle tail produced
// its settle report; the report parked the round at the human-judgment
// boundary (audition A/B rendered, loop blocked waiting for the judgment
// POST); the same request's recordAudioClosureRound then hit the no-progress
// boundary, whose settle attempted the illegal phase_transition
// fs7_improvement_proposal -> fs9_terminal and failed the whole run. The
// fixtures below mirror the closure event stream of that run: spine at fs7
// (never fs8 — the experiment lifecycle ran while the spine sat at fs4-fs6,
// and the settle report's GatePassed assertion is what entered fs7), frontier
// carrying the round's candidate, no-progress streak one short of the bound,
// and a round in progress.

const fsSettleTerminalConversation = "conversation-fs-settle-terminal-1"

// fsSettleTerminalServer builds the server, task contract, judgment-parked
// loop, and the fs7 closure in the shape the failed run persisted.
func fsSettleTerminalServer(t *testing.T, parkLoop bool) (*Server, freeStateReasoningLoop, audioclosure.State) {
	t.Helper()
	server := &Server{
		harness:          harness.NewWithSender(nil, nil, nil),
		audioClosures:    audioclosure.NewMemoryStore(),
		controllerOwners: orchestrationcontroller.NewRegistry(),
		freeStateLoops:   map[string]freeStateReasoningLoop{},
		capabilityRoutes: map[string]CapabilityRouteRecord{},
	}
	goal := server.harness.EnsureGoal("goal-fs-settle-terminal-1", "run-fs-settle-terminal-1", "inspect the project")
	if _, err := server.ensureAudioTaskContract(fsSettleTerminalConversation, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-fs-settle-terminal-1"}, "project-fs-settle-terminal-1", "rev-1",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}

	// The judgment park: an applied D1 round whose settle report carries
	// user_judgment_pending — the same recordFreeStateExperimentDecision
	// ingestion the live round-5 slice performed at 20:09:58.
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-fs-settle-terminal-1",
		ConversationID: fsSettleTerminalConversation, GoalID: goal.GoalID, RunID: "run-fs-settle-terminal-1",
		Status: "awaiting_experiment", OriginalIntent: "inspect the project",
		LatestObservation: d1FreshObservationForTest("rev-1"),
		CreatedAt:         time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	// The production admission order: the proposal lands on the task contract
	// (diagnostic completed -> improvement proposed) before the experiment
	// runtime binds its identity (experiment_required).
	if err := server.applyFreeStateDecisionSemantic(&loop, agentloop.FreeStateDecision{
		Status: agentloop.FreeStateNeedsExperiment, Summary: "fixture proposal admission",
		ImprovementProposal: experimentTestProposal(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	d2ApplyAndObserveForTest(t, loop.Experiment, "fs-settle-action-1", "rev-1")
	server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
		ExperimentMateriality: &experiment.MaterialityEvaluation{
			State: experiment.MaterialitySubthreshold, Evaluation: "insufficient_dose",
			Attempt: 1, EvidenceRefs: []string{"obs-fs-settle-action-1"},
		},
		ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
	})
	loop.Status = "blocked"
	loop.DecisionPhase = freeStatePhasePostActionEvaluation
	loop.LastError = "experiment round is waiting for the human judgment boundary"
	// The audition-preview-rendered session form: the park carries a ready
	// audition session the same way the live loop did.
	loop.AuditionSessionID = "audition:turn:fs-settle-terminal-1:round-1"
	loop.AuditionSessionSnapshot = map[string]any{"status": "ready", "session_id": loop.AuditionSessionID}
	if parkLoop {
		server.storeFreeStateLoop(loop)
	}

	// The closure at fs7 with one candidate in the frontier, the streak one
	// short of the bound, and a round in progress — the exact counters the
	// failed run's persisted stream showed (event 30 + rounds 4/5).
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-fs-settle-terminal-1", ConversationID: fsSettleTerminalConversation,
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID,
		ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State,
		TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID:       "project-fs-settle-terminal-1", ProjectRevision: "rev-1",
		OriginalIntent: "inspect the project", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-fs-settle-terminal-1"},
		Policy: audioclosure.Policy{MaxClosureRounds: 6, MaxUniqueObservations: 4, MaxDutyObservations: 8,
			MaxNoProgressRounds: 2, MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1},
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	now := time.Now().UTC()
	walk := func(to audioclosure.Phase, guard audioclosure.PhaseGuardInput) {
		next, walkErr := driver.TransitionPhase(state, state.Revision, to, guard, "fs-settle-terminal-1 fixture", now)
		if walkErr != nil {
			t.Fatalf("fixture phase walk to %s failed: %v", to, walkErr)
		}
		state = next
	}
	walk(audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{})
	walk(audioclosure.PhaseFS1ProjectBound, audioclosure.PhaseGuardInput{ProjectBound: true})
	walk(audioclosure.PhaseFS2CapacityAssessed, audioclosure.PhaseGuardInput{CapacityAssessed: true})
	walk(audioclosure.PhaseFS3ProjectScan, audioclosure.PhaseGuardInput{ScanUsable: true})
	walk(audioclosure.PhaseFS4DiagnosticRound, audioclosure.PhaseGuardInput{QueueNonEmpty: true})
	walk(audioclosure.PhaseFS5CandidateFrontier, audioclosure.PhaseGuardInput{FrontierEstablished: true, DimensionClosed: true})
	walk(audioclosure.PhaseFS6TargetConfirmed, audioclosure.PhaseGuardInput{TargetEvidence: true})
	walk(audioclosure.PhaseFS7ImprovementProposal, audioclosure.PhaseGuardInput{GatePassed: true})

	// Round 1 establishes the frontier (progress), round 2 repeats it
	// (no progress -> streak 1), round 3 is the settle-report round in
	// progress — the live run's round 5.
	for round := 1; round <= 3; round++ {
		admitted, _, admitErr := driver.AdmitRound(state, state.Revision, now)
		if admitErr != nil {
			t.Fatalf("fixture round %d admit failed: %v", round, admitErr)
		}
		state = admitted
		next, _, updateErr := driver.UpdateFrontier(state, state.Revision, audioclosure.HypothesisFrontier{
			CandidateID:   "candidate:fs-settle-1",
			HypothesisIDs: []string{"candidate:candidate:fs-settle-1"},
			Candidates:    []audioclosure.Candidate{{ID: "candidate:fs-settle-1", TrackIDs: []string{"1007"}, ViewID: "track.timbre_frequency"}},
		}, audioclosure.ActionabilityActionable, now)
		if updateErr != nil {
			t.Fatalf("fixture round %d frontier failed: %v", round, updateErr)
		}
		state = next
		if round < 3 {
			completed, completeErr := driver.CompleteRound(state, state.Revision, now)
			if completeErr != nil {
				t.Fatalf("fixture round %d completion failed: %v", round, completeErr)
			}
			state = completed
		}
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	return server, loop, state
}

// fsSettleTerminalSettleReportResult is the round-5 slice result shape: the
// settle report riding the preserved needs_experiment decision.
func fsSettleTerminalSettleReportResult(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusWaitingContinue,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status: agentloop.FreeStateNeedsExperiment, Summary: "settle report: insufficient dose, human A/B judgment pending",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: "insufficient_dose",
				Attempt: 1, EvidenceRefs: []string{"obs-fs-settle-action-1"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
	}
}

// The scenario nail (RED on the pre-fix code): the settle-report recording
// must not fail the run. The no-progress boundary fires exactly when the
// round just parked at the human-judgment boundary — the boundary must stand
// down (the guarded audition judgment POST is the only settle), the round
// completes, and the park survives for the user's A/B decision.
func TestSettleTailRecordingSparesJudgmentParkedFS7Closure(t *testing.T) {
	server, loop, state := fsSettleTerminalServer(t, true)
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: the fixture loop must be parked at the human judgment boundary")
	}

	recorded, err := server.recordAudioClosureRound(state, fsSettleTerminalSettleReportResult(loop), map[string]any{})

	if err != nil {
		t.Fatalf("the settle-report recording failed the run (the 2026-09-30 20:09 live failure): %v", err)
	}
	if recorded.Terminal() {
		t.Fatalf("the boundary settled a closure parked at the human judgment boundary: %+v", recorded.Settlement)
	}
	if recorded.RoundInProgress {
		t.Fatal("the settle-report round must complete at the recording boundary")
	}
	stored, _ := server.freeStateLoop(fsSettleTerminalConversation)
	if !freeStateJudgmentBoundary(stored) || freeStateLoopActive(stored) {
		t.Fatalf("the judgment park must survive the boundary: boundary=%v active=%v",
			freeStateJudgmentBoundary(stored), freeStateLoopActive(stored))
	}
	if !strings.Contains(stored.AuditionSessionID, "audition:") {
		t.Fatalf("the audition session must stay armed for the human A/B judgment: %q", stored.AuditionSessionID)
	}
}

// The construction nail (RED on the pre-fix code): when the boundary settle
// DOES run on an fs7 closure (no protected window), it must reach the
// terminal through the settled-event channel — never by constructing the
// illegal phase_transition fs7 -> fs9 the adjacency table forbids.
func TestBoundarySettleFromFS7UsesSettledEventTerminalNotIllegalTransition(t *testing.T) {
	server, _, state := fsSettleTerminalServer(t, false)

	next, err := server.settleTaskAtAudioClosureBoundary(state, "closure made no material progress within its bounded observation window")

	if err != nil {
		t.Fatalf("the boundary settle from fs7 failed: %v", err)
	}
	if !next.Terminal() {
		t.Fatalf("the boundary settle from fs7 must reach a terminal closure, phase=%s", next.Phase)
	}
	if next.Phase != audioclosure.PhaseFS9Terminal {
		t.Fatalf("the settled closure must sit at fs9, got %s", next.Phase)
	}
	for _, event := range next.Events {
		if event.Type != "phase_transition" {
			continue
		}
		data := event.Data
		if strings.Contains(string(data), string(audioclosure.PhaseFS9Terminal)) &&
			strings.Contains(string(data), string(audioclosure.PhaseFS7ImprovementProposal)) {
			t.Fatalf("the boundary settle constructed a phase_transition fs7 -> fs9 event the adjacency table forbids: %s", data)
		}
	}
}

// The phase table itself stays sealed: fs7's legal successors remain exactly
// {fs8, fs4}. Any future change here invalidates this card's fix rationale
// and must re-run the FS-SETTLE-TERMINAL-1 forensics.
func TestFS7TerminalAdjacencyStaysSealed(t *testing.T) {
	if audioclosure.LegalPhaseTransition(audioclosure.PhaseFS7ImprovementProposal, audioclosure.PhaseFS9Terminal) {
		t.Fatal("fs7 -> fs9 must stay illegal: the transition table is sealed by this card")
	}
	if !audioclosure.LegalPhaseTransition(audioclosure.PhaseFS7ImprovementProposal, audioclosure.PhaseFS8ExperimentVerification) ||
		!audioclosure.LegalPhaseTransition(audioclosure.PhaseFS7ImprovementProposal, audioclosure.PhaseFS4DiagnosticRound) {
		t.Fatal("fs7 legal successors must remain {fs8, fs4}")
	}
}
