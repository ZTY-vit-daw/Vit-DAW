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

// FIX-SETTLE-PARK-REPLAY-RACE-1 regression: the 2026-10-14 run10/run11 live
// failure (FIX-FS-SETTLE-DEADCHAIN-1 REPORT §九). The settle report parked the
// round at the human-judgment boundary; the parked round's own closure replay
// then rehydrated the durable ledger, whose pre-action rows still carry the
// revision the intervention replaced. The satisfied post-action filter no
// longer recognized them (requires_post_action_observation had flipped to
// false), the revision fold had emptied Observations so dedup no longer knew
// them, and the cross-revision guard settled the parked closure with
// project_revision_stale one second after the park — the smoke's 2s polling
// window can never win that race. The fix is on the feeding side: while the
// loop is parked at the judgment boundary, the replay must not offer the
// guard observations whose key revision cannot be the closure's current
// revision.

const parkedReplayConversation = "conversation-parked-replay-race-1"

// parkedReplayRaceServer builds the run10/run11 closure shape: round 1 booked
// the pre-action observation at rev-2, the intervention advanced the revision
// to rev-4 through the revalidation channel (the fold empties Observations by
// design and, unlike a governed booking, registers no superseded revision),
// round 2 booked the post-action observation at rev-4, and round 3 — left in
// progress — is the settle-report round whose legal materiality parks the
// loop. The durable ledger still carries both rows; only the pre-action row
// carries a revision the guard would settle on.
func parkedReplayRaceServer(t *testing.T, parked bool) (*Server, freeStateReasoningLoop, audioclosure.State) {
	t.Helper()
	server := &Server{
		harness:          harness.NewWithSender(nil, nil, nil),
		audioClosures:    audioclosure.NewMemoryStore(),
		controllerOwners: orchestrationcontroller.NewRegistry(),
		freeStateLoops:   map[string]freeStateReasoningLoop{},
		capabilityRoutes: map[string]CapabilityRouteRecord{},
	}
	goal := server.harness.EnsureGoal("goal-parked-replay-race-1", "run-parked-replay-race-1", "inspect the project")
	if _, err := server.ensureAudioTaskContract(parkedReplayConversation, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-parked-replay-race-1"}, "project-parked-replay-race-1", "rev-2",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}

	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-parked-replay-race-1",
		ConversationID: parkedReplayConversation, GoalID: goal.GoalID, RunID: "run-parked-replay-race-1",
		Status: "awaiting_experiment", OriginalIntent: "inspect the project",
		LatestObservation: d1FreshObservationForTest("rev-2"),
		CreatedAt:         time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := server.applyFreeStateDecisionSemantic(&loop, agentloop.FreeStateDecision{
		Status: agentloop.FreeStateNeedsExperiment, Summary: "fixture proposal admission",
		ImprovementProposal: experimentTestProposal(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := server.startFreeStateExperiment(&loop, agentloop.FreeStateDecision{ImprovementProposal: experimentTestProposal()}, loop.GoalID, loop.RunID); err != nil {
		t.Fatal(err)
	}
	d2ApplyAndObserveForTest(t, loop.Experiment, "parked-replay-action-1", "rev-2")
	if parked {
		server.recordFreeStateExperimentDecision(context.Background(), &loop, agentloop.FreeStateDecision{
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: "insufficient_dose",
				Attempt: 1, EvidenceRefs: []string{"obs-parked-replay-action-1"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		})
		loop.Status = "blocked"
		loop.DecisionPhase = freeStatePhasePostActionEvaluation
		loop.LastError = "experiment round is waiting for the human judgment boundary"
		loop.AuditionSessionID = "audition:turn:parked-replay-race-1:round-1"
		loop.AuditionSessionSnapshot = map[string]any{"status": "ready", "session_id": loop.AuditionSessionID}
	}
	// The parked-settle premise of the live failure: the legal settle satisfied
	// the post-action debt (the replay filter stands down) while the durable
	// ledger still carries the pre-action rev-2 row alongside the post-action
	// rev-4 row.
	loop.RequiresPostActionObservation = false
	loop.ObservationLedger = map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"track.timbre_frequency": map[string]any{
				"view_id": "track.timbre_frequency", "status": "ready",
				"observation_id":  "obs-pre-action-rev2",
				"target_ref":      map[string]any{"kind": "track", "id": "vocal"},
				"project_binding": map[string]any{"project_revision": "rev-2"},
				"freshness":       map[string]any{"status": "current_observation", "project_revision": "rev-2"},
			},
			"track:1007::track.band_dynamics": map[string]any{
				"view_id": "track.band_dynamics", "status": "ready",
				"observation_id":  "obs-post-action-rev4",
				"target_ref":      map[string]any{"kind": "track", "id": "1007"},
				"project_binding": map[string]any{"project_revision": "rev-4"},
				"freshness":       map[string]any{"status": "current_observation", "project_revision": "rev-4"},
			},
		},
	}
	server.storeFreeStateLoop(loop)

	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-parked-replay-race-1", ConversationID: parkedReplayConversation,
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID,
		ContractID: current.Task.Contract.ContractID, TaskState: current.Task.SemanticState.State,
		TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID:       "project-parked-replay-race-1", ProjectRevision: "rev-2",
		OriginalIntent: "inspect the project", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-parked-replay-race-1"},
		Policy: audioclosure.Policy{MaxClosureRounds: 6, MaxUniqueObservations: 4, MaxDutyObservations: 8,
			MaxNoProgressRounds: 2, MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1},
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	now := time.Now().UTC()
	obsKey := func(rev string) audioclosure.ObservationKey {
		return audioclosure.ObservationKey{
			ProjectUUID: "project-parked-replay-race-1", ProjectRevision: rev,
			Scope:     audioclosure.Scope{Kind: "project", ID: "project-parked-replay-race-1"},
			TargetRef: "vocal", ViewIDs: []string{"track.timbre_frequency"},
			ObservationMode: "ccb.observation_request",
		}
	}
	bookRound := func(rev, observationID string) {
		admitted, _, admitErr := driver.AdmitRound(state, state.Revision, now)
		if admitErr != nil {
			t.Fatalf("fixture round admit failed: %v", admitErr)
		}
		state = admitted
		outcome, recordErr := driver.RecordObservation(state, state.Revision, obsKey(rev), observationID, now)
		if recordErr != nil {
			t.Fatalf("fixture observation %s failed: %v", observationID, recordErr)
		}
		state = outcome.State
		completed, completeErr := driver.CompleteRound(state, state.Revision, now)
		if completeErr != nil {
			t.Fatalf("fixture round completion failed: %v", completeErr)
		}
		state = completed
	}
	// Round 1: the pre-action evidence at rev-2.
	bookRound("rev-2", "obs-pre-action-rev2")
	// The intervention advanced the revision rev-2 -> rev-4 through the
	// revalidation channel: the fold empties Observations and registers no
	// superseded revision — the exact live precondition that disarmed dedup.
	revalidated, changed, revalErr := driver.RevalidateProjectRevision(state, state.Revision, "rev-4", now)
	if revalErr != nil || !changed {
		t.Fatalf("fixture revalidation failed: changed=%v err=%v", changed, revalErr)
	}
	state = revalidated
	if len(state.Observations) != 0 {
		t.Fatalf("fixture: the revision fold must empty Observations, got %d rows", len(state.Observations))
	}
	// Round 2: the post-action verification observation at rev-4 (the D1 gate
	// both live runs passed before the settle attempt).
	bookRound("rev-4", "obs-post-action-rev4")
	// Round 3 (in progress): the settle-report round whose legal materiality
	// parks the loop — the replay under test runs inside this round.
	admitted, _, admitErr := driver.AdmitRound(state, state.Revision, now)
	if admitErr != nil {
		t.Fatalf("fixture settle-report round admit failed: %v", admitErr)
	}
	state = admitted
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	return server, loop, state
}

// parkedReplaySettleReportResult is the settle-report result shape the live
// round produced: legal materiality plus the user_judgment_pending round
// decision riding the preserved needs_experiment status.
func parkedReplaySettleReportResult(loop freeStateReasoningLoop) agentloop.Result {
	return agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusWaitingContinue,
		FreeStateDecision: &agentloop.FreeStateDecision{
			Status: agentloop.FreeStateNeedsExperiment, Summary: "settle report: insufficient dose, human A/B judgment pending",
			ImprovementProposal: experimentTestProposal(),
			ExperimentMateriality: &experiment.MaterialityEvaluation{
				State: experiment.MaterialitySubthreshold, Evaluation: "insufficient_dose",
				Attempt: 1, EvidenceRefs: []string{"obs-parked-replay-action-1"},
			},
			ExperimentRoundDecision: string(experiment.DecisionUserJudgment),
		},
	}
}

// Nail ① (RED on the pre-fix code): the parked settle-report recording must
// not feed the ledger's pre-action revision row to the cross-revision guard —
// the row is skipped at the replay side, the round completes, and the park
// survives for the user's A/B judgment.
func TestParkedJudgmentReplaySkipsPreActionRevisionObservations(t *testing.T) {
	server, loop, state := parkedReplayRaceServer(t, true)
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: the fixture loop must be parked at the human judgment boundary")
	}

	recorded, err := server.recordAudioClosureRound(state, parkedReplaySettleReportResult(loop), map[string]any{})

	if err != nil {
		t.Fatalf("the settle-report recording failed the run: %v", err)
	}
	if recorded.Terminal() {
		t.Fatalf("replayed pre-action evidence machine-settled the judgment-parked closure (the run10/run11 failure): %+v", recorded.Settlement)
	}
	if audioClosureHasObservationID(recorded, "obs-pre-action-rev2") {
		t.Fatal("the stale pre-action row must not be booked into the parked closure")
	}
	if recorded.ModelObservationCount() != 1 {
		t.Fatalf("the parked replay must leave exactly the post-action booking, got %d", recorded.ModelObservationCount())
	}
	if recorded.RoundInProgress {
		t.Fatal("the settle-report round must complete at the recording boundary")
	}
	stored, _ := server.freeStateLoop(parkedReplayConversation)
	if !freeStateJudgmentBoundary(stored) || freeStateLoopActive(stored) {
		t.Fatalf("the judgment park must survive the replay: boundary=%v active=%v",
			freeStateJudgmentBoundary(stored), freeStateLoopActive(stored))
	}
	if !strings.Contains(stored.AuditionSessionID, "audition:") {
		t.Fatalf("the audition session must stay armed for the human A/B judgment: %q", stored.AuditionSessionID)
	}
}

// Nail ② (RED on the pre-fix code, FS-SETTLE-TERMINAL-1 invariant): the
// machine settlement landed one second after the live park while the smoke
// polls at two seconds. The unit-level window: after the park, repeated
// scheduler-continuation replays of the same durable ledger must not
// terminalize the closure — no settle event of any shape may appear while
// the loop waits for the guarded judgment POST.
func TestJudgmentParkedClosureSurvivesContinuationReplayWindow(t *testing.T) {
	server, loop, state := parkedReplayRaceServer(t, true)
	if !freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: the fixture loop must be parked at the human judgment boundary")
	}

	recorded, err := server.recordAudioClosureRound(state, parkedReplaySettleReportResult(loop), map[string]any{})
	if err != nil {
		t.Fatalf("the settle-report recording failed: %v", err)
	}
	for replay := 1; replay <= 2; replay++ {
		recorded, err = server.recordAudioClosureRound(recorded, agentloop.Result{
			GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusWaitingContinue,
		}, map[string]any{})
		if err != nil {
			t.Fatalf("continuation replay %d failed: %v", replay, err)
		}
		if recorded.Terminal() {
			t.Fatalf("continuation replay %d machine-settled the parked closure: %+v", replay, recorded.Settlement)
		}
		for _, event := range recorded.Events {
			if strings.Contains(string(event.Data), string(audioclosure.StopProjectRevisionStale)) {
				t.Fatalf("continuation replay %d produced a project_revision_stale settle event: %s", replay, event.Data)
			}
		}
	}
	stored, _ := server.freeStateLoop(parkedReplayConversation)
	if !freeStateJudgmentBoundary(stored) {
		t.Fatal("the judgment park must survive the whole continuation window")
	}
}

// Nail ③ (the red face guarding the fix's blast radius): with the loop NOT
// parked at the judgment boundary, the same ledger replay must keep feeding
// the stale pre-action row to the cross-revision guard and settle with
// project_revision_stale — the guard's non-parked semantics stay verbatim.
func TestNonParkedReplayKeepsCrossRevisionGuardSettleSemantics(t *testing.T) {
	server, loop, state := parkedReplayRaceServer(t, false)
	if freeStateJudgmentBoundary(loop) {
		t.Fatal("setup: the fixture loop must not be parked")
	}

	recorded, err := server.recordAudioClosureRound(state, agentloop.Result{
		GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusWaitingContinue,
	}, map[string]any{})

	if err != nil {
		t.Fatalf("the non-parked replay failed: %v", err)
	}
	if !recorded.Terminal() {
		t.Fatalf("the cross-revision guard must keep settling non-parked stale-revision replays: %+v", recorded.Settlement)
	}
	if recorded.Settlement == nil || recorded.Settlement.Reason != audioclosure.StopProjectRevisionStale {
		t.Fatalf("the settlement reason must stay project_revision_stale: %+v", recorded.Settlement)
	}
}
