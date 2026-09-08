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
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

func audioClosureTestServer() *Server {
	return &Server{audioClosures: audioclosure.NewMemoryStore(), controllerOwners: orchestrationcontroller.NewRegistry()}
}

func audioClosureTestContext() map[string]any {
	return contextWithSemanticEntryDecision(map[string]any{
		"selected_track_id": "track-vocal", "selected_track_name": "Vocal", "project_uuid": "project-1", "project_revision": "revision-1",
	}, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller: string(orchestrationcontroller.MinimalAudioClosure), TargetScope: semanticEntryScopeCurrentSelection,
		ControlMode: semanticEntryControlSemanticLoop, UserAuthorization: semanticEntryAuthorizationAction,
		Confidence: 0.9, Reason: "close one vocal harshness issue",
	})
}

func prepareAudioClosureTestState(t *testing.T, server *Server) (map[string]any, audioclosure.State) {
	t.Helper()
	ctx, state, active, err := server.prepareAudioClosureContext("conversation-1", "make the vocal less harsh", audioClosureTestContext())
	if err != nil || !active {
		t.Fatalf("prepare closure: active=%v state=%+v err=%v", active, state, err)
	}
	return ctx, state
}

func TestAudioClosureControllerPersistsRoundsAndSettlesNoProgress(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	restoredContext, restored, active, err := server.prepareAudioClosureContext("conversation-1", "continue", ctx)
	if err != nil || !active || restored.ClosureID != state.ClosureID {
		t.Fatalf("active closure was not restored: state=%+v err=%v", restored, err)
	}
	ctx = restoredContext
	for round := 1; round <= 2; round++ {
		state, active, err = server.admitAudioClosureRound(restored)
		if err != nil || !active || state.RoundsStarted != round {
			t.Fatalf("round %d admission: active=%v state=%+v err=%v", round, active, state, err)
		}
		state, err = server.recordAudioClosureRound(state, agentloop.Result{}, ctx)
		if err != nil {
			t.Fatal(err)
		}
		restored = state
	}
	if !state.Terminal() || state.Settlement.Reason != audioclosure.StopNoProgress {
		t.Fatalf("closure did not settle no_progress: %+v", state)
	}
	if _, ok := server.controllerOwners.Active("conversation-1"); ok {
		t.Fatal("terminal closure retained top-level controller ownership")
	}
}

func TestProjectAudioClosureDoesNotSettleNoProgressBeforeCandidateFrontier(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil), audioClosures: audioclosure.NewMemoryStore(), controllerOwners: orchestrationcontroller.NewRegistry()}
	goal := server.harness.EnsureGoal("goal-project-no-progress", "run-project-no-progress", "inspect the project")
	ctx, err := server.ensureAudioTaskContract("conversation-project-no-progress", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-no-progress"}, "project-no-progress", "rev-1", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-project-no-progress", ConversationID: "conversation-project-no-progress", TaskID: current.Task.TaskID,
		GoalID: current.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-no-progress", ProjectRevision: "rev-1", OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-no-progress"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 2; round++ {
		var admitted bool
		state, admitted, err = server.admitAudioClosureRound(state)
		if err != nil || !admitted {
			t.Fatalf("round %d admission: admitted=%v err=%v", round+1, admitted, err)
		}
		state, err = server.recordAudioClosureRound(state, agentloop.Result{}, ctx)
		if err != nil {
			t.Fatalf("round %d record: %v", round+1, err)
		}
	}
	if state.Terminal() || state.NoProgressStreak < state.Policy.MaxNoProgressRounds {
		t.Fatalf("project closure settled before candidate frontier: %+v", state)
	}
}

func TestAudioClosureRecordsAuthoritativeRecentCCBObservationWithoutTreatingItAsProgress(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit round: admitted=%v err=%v", admitted, err)
	}
	observation := &agentloop.RecentObservation{
		ToolCallID: "tool-step-1", Tool: "ccb.observation_request", Status: "ready",
		Summary: map[string]any{
			"observation_id":  "observation-1",
			"receipt_id":      "receipt-1",
			"requested_views": []any{"project.structure", "mix.multitrack_relationship"},
			"target_ref":      map[string]any{"kind": "project", "id": "project-1"},
		},
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: observation,
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "inspect the admitted project evidence",
		},
	}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Observations) != 1 || state.NoProgressStreak != 1 || state.Terminal() {
		t.Fatalf("authoritative recent observation was not recorded or incorrectly counted as progress: %+v", state)
	}
}

func TestAudioClosureRepeatedObservationProjectionDoesNotConsumeEvidenceCeiling(t *testing.T) {
	server := audioClosureTestServer()
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-repeated-projection", ConversationID: "conversation-repeated-projection",
		ProjectUUID: "project-1", ProjectRevision: "revision-1", OriginalIntent: "inspect the project",
		Mode: audioclosure.ModeDiagnostic, Scope: audioclosure.Scope{Kind: "project", ID: "project-1"},
		Policy: audioclosure.Policy{
			MaxClosureRounds: 4, MaxUniqueObservations: 1, MaxNoProgressRounds: 4,
			MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1,
		},
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	ctx := map[string]any{"project_uuid": "project-1", "project_revision": "revision-1"}
	for round, viewID := range []string{"project.structure", "mix.frequency_relationship"} {
		var admitted bool
		state, admitted, err = server.admitAudioClosureRound(state)
		if err != nil || !admitted {
			t.Fatalf("round %d admission: admitted=%v err=%v", round+1, admitted, err)
		}
		observation := &agentloop.RecentObservation{
			Tool: "ccb.observation_request", Status: "ready",
			Summary: map[string]any{
				"status": "ready", "observation_id": "obs-shared-bundle",
				"requested_views": []any{viewID}, "views": map[string]any{viewID: map[string]any{"status": "ready"}},
			},
		}
		state, err = server.recordAudioClosureRound(state, agentloop.Result{
			RecentObservation: observation,
			FreeStateDecision: &agentloop.FreeStateDecision{
				SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
				EvidenceStatus: "insufficient", Summary: "continue project observation",
			},
		}, ctx)
		if err != nil {
			t.Fatalf("round %d record: %v", round+1, err)
		}
	}
	if state.Terminal() || len(state.Observations) != 1 {
		t.Fatalf("repeated projection consumed the evidence ceiling: %+v", state)
	}
	for _, record := range state.Observations {
		if record.ObservationID != "obs-shared-bundle" {
			t.Fatalf("unexpected retained observation: %+v", record)
		}
	}
}

func TestAudioClosureTransientModelFailureKeepsRoundActive(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit round: admitted=%v err=%v", admitted, err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonTransientLLMError,
	}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() || !state.RoundInProgress || state.NoProgressStreak != 0 {
		t.Fatalf("transient model failure consumed or settled closure round: %+v", state)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted || state.Terminal() || !state.RoundInProgress || state.RoundsStarted != 1 {
		t.Fatalf("transient retry did not resume the same active round: admitted=%v state=%+v err=%v", admitted, state, err)
	}
}

func TestAudioClosureTransientModelFailureDoesNotReplayPriorDecision(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	state, _, _ = server.admitAudioClosureRound(state)
	revision := state.Revision
	state, err := server.recordAudioClosureRound(state, agentloop.Result{
		Status:     agentruntime.StatusWaitingContinue,
		StopReason: agentloop.StopReasonTransientLLMError,
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status:        agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient",
			Summary: "stale prior decision",
		},
	}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != revision || state.NoProgressStreak != 0 || len(state.Events) != int(revision) {
		t.Fatalf("transient failure replayed a prior decision into closure state: %+v", state)
	}
}

func TestAudioClosureRoundBoundaryDoesNotInventNoCandidateFound(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil), audioClosures: audioclosure.NewMemoryStore(), controllerOwners: orchestrationcontroller.NewRegistry()}
	goal := server.harness.EnsureGoal("goal-boundary", "run-boundary", "inspect the project")
	_, err := server.ensureAudioTaskContract("conversation-boundary", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-boundary"}, "project-boundary", "rev-1", map[string]any{"goal_id": goal.GoalID})
	if err != nil {
		t.Fatal(err)
	}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-boundary", ConversationID: "conversation-boundary", TaskID: current.Task.TaskID,
		GoalID: current.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-boundary", ProjectRevision: "rev-1", OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-boundary"},
		Policy: audioclosure.Policy{MaxClosureRounds: 1, MaxUniqueObservations: 2, MaxNoProgressRounds: 2, MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1},
		Now:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	state, _, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	observation := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"status": "ready", "observation_id": "obs-boundary", "requested_views": []any{"project.structure"},
		"views": map[string]any{"project.structure": map[string]any{"status": "ready"}},
	}}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{RecentObservation: observation}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || admitted || !state.Terminal() || state.TaskState != taskstate.StateCapabilityBlocked || state.Settlement.Reason != audioclosure.StopCapabilityBlocked {
		t.Fatalf("closure boundary fabricated no_candidate_found: admitted=%v state=%+v err=%v", admitted, state, err)
	}
}

func TestAudioClosureExhaustedOpenQueueSettlesNoCandidateAtFS9(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil), audioClosures: audioclosure.NewMemoryStore(), controllerOwners: orchestrationcontroller.NewRegistry(), freeStateLoops: map[string]freeStateReasoningLoop{}, capabilityRoutes: map[string]CapabilityRouteRecord{}}
	goal := server.harness.EnsureGoal("goal-open-exhausted", "run-open-exhausted", "inspect the project")
	if _, err := server.ensureAudioTaskContract("conversation-open-exhausted", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-open-exhausted"}, "project-open-exhausted", "rev-1", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	loop := continuationTestLoop("conversation-open-exhausted")
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-open-exhausted"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1", ConversationID: loop.ConversationID,
		Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{ClosureID: "closure-open-exhausted", ConversationID: loop.ConversationID,
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-open-exhausted", ProjectRevision: "rev-1", OriginalIntent: "inspect the project", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-open-exhausted"}, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	state, err = driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{}, "test entry", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS1ProjectBound, audioclosure.PhaseGuardInput{ProjectBound: true}, "test project", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS2CapacityAssessed, audioclosure.PhaseGuardInput{CapacityAssessed: true}, "test capacity", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	state.ObservationOrder = []string{"obs-open-exhausted"}
	if _, err := server.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventDiagnosticCompleted, Reason: "test evidence", EvidenceRefs: []string{"obs-open-exhausted"}, ProjectRevision: "rev-1"}); err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	next, err := server.settleTaskAtAudioClosureBoundary(state, "continuation budget exhausted")
	if err != nil {
		t.Fatal(err)
	}
	// BOUNDARY-1 总则 1: the exhausted boundary no longer settles an unlocked
	// observation loop outright — it locks the terminal turn, raises the
	// scheduling floor by exactly one checkpoint, and defers so the reserved
	// terminal slice can run.
	if next.Terminal() {
		t.Fatalf("unlocked observation loop was settled before its terminal turn: %+v", next.Settlement)
	}
	storedLoop, _ := server.freeStateLoop(loop.ConversationID)
	if !storedLoop.TerminalTurnLocked || storedLoop.TerminalTurnReason != FreeStateTerminalReasonBudgetCritical {
		t.Fatalf("terminal turn not locked at the exhausted boundary: locked=%v reason=%q", storedLoop.TerminalTurnLocked, storedLoop.TerminalTurnReason)
	}
	if storedLoop.ContinuationBudget != storedLoop.ContinuationUsed+1 {
		t.Fatalf("terminal reservation did not raise the continuation floor: used=%d budget=%d", storedLoop.ContinuationUsed, storedLoop.ContinuationBudget)
	}
	// The terminal chain burns the reservation without a decision: the next
	// boundary settles honestly (the fallback), with the same no_candidate
	// semantics the exhausted open queue always had.
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	next, err = server.settleTaskAtAudioClosureBoundary(state, "continuation budget exhausted")
	if err != nil {
		t.Fatal(err)
	}
	if !next.Terminal() || next.Settlement == nil || next.Settlement.Reason != audioclosure.StopNoCandidateFound || next.Phase != audioclosure.PhaseFS9Terminal {
		t.Fatalf("exhausted open queue did not settle no_candidate_found at FS9: %+v", next)
	}
}

func TestAudioClosureFrontierNeverSettlesRuntimeNoCandidateFound(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil), audioClosures: audioclosure.NewMemoryStore(), controllerOwners: orchestrationcontroller.NewRegistry(), freeStateLoops: map[string]freeStateReasoningLoop{}, capabilityRoutes: map[string]CapabilityRouteRecord{}}
	goal := server.harness.EnsureGoal("goal-frontier-boundary", "run-frontier-boundary", "improve the project")
	if _, err := server.ensureAudioTaskContract("conversation-frontier-boundary", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-frontier-boundary"}, "project-frontier-boundary", "rev-1", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{ClosureID: "closure-frontier-boundary", ConversationID: "conversation-frontier-boundary",
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision, ProjectUUID: "project-frontier-boundary",
		ProjectRevision: "rev-1", OriginalIntent: "improve the project", Mode: audioclosure.ModeTreatment,
		Scope: audioclosure.Scope{Kind: "project", ID: "project-frontier-boundary"}, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	state.Frontier = audioclosure.HypothesisFrontier{CandidateID: "candidate-bass", Candidates: []audioclosure.Candidate{{ID: "candidate-bass", TrackIDs: []string{"1007"}, SourceObservationID: "obs-bass", EvidenceRefs: []string{"obs-bass"}}}}
	loop := continuationTestLoop("conversation-frontier-boundary")
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-frontier-boundary"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1", ConversationID: loop.ConversationID,
		Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	next, err := server.settleTaskAtAudioClosureBoundary(state, "admission boundary")
	if err != nil {
		t.Fatal(err)
	}
	// BOUNDARY-1 总则 1: the frontier loop first receives its reserved
	// terminal turn; the settle defers.
	if next.Terminal() {
		t.Fatalf("frontier loop was settled before its terminal turn: %+v", next.Settlement)
	}
	// The terminal chain burns the reservation without a decision: the honest
	// fallback settles, and a live frontier still never reports
	// no_candidate_found.
	storedLoop, _ := server.freeStateLoop(loop.ConversationID)
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	next, err = server.settleTaskAtAudioClosureBoundary(state, "admission boundary")
	if err != nil {
		t.Fatal(err)
	}
	if !next.Terminal() || next.Settlement == nil || next.Settlement.Reason != audioclosure.StopCapabilityBlocked {
		t.Fatalf("frontier was misclassified as no_candidate_found: %+v", next)
	}
}

func TestAudioClosureBuildsCandidatesFromCompactedViewFacts(t *testing.T) {
	observation := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "partial", Summary: map[string]any{
		"status": "partial", "observation_id": "obs-compacted", "requested_views": []any{"mix.multitrack_relationship"},
		"views": map[string]any{"mix.multitrack_relationship": map[string]any{
			"status": "partial", "facts": map[string]any{"band_conflict_candidates": []any{
				map[string]any{"band": "bass", "status": "candidate", "tracks": []any{map[string]any{"track_id": "1007"}, map[string]any{"track_id": "1012"}}},
				map[string]any{"band": "mid", "status": "candidate", "tracks": []any{map[string]any{"track_id": "1022"}, map[string]any{"track_id": "1017"}}},
			}},
		}},
	}}
	candidates := audioClosureCandidates(nil, []*agentloop.RecentObservation{observation})
	if len(candidates) != 2 {
		t.Fatalf("compacted CCB facts produced %d candidates: %+v", len(candidates), candidates)
	}
}

func TestAudioClosureBuildsCandidateFrontierThenSelectsTarget(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit discovery round: admitted=%v err=%v", admitted, err)
	}
	relationship := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-relationship",
		"requested_views": []any{"mix.frequency_relationship"}, "evidence_refs": []any{"observation:obs-relationship"},
		"views": map[string]any{"mix.frequency_relationship": map[string]any{
			"status": "ready", "facts": map[string]any{"observation.mom_projection": map[string]any{
				"frequency_relationship": map[string]any{"status": "ready", "coverage": map[string]any{"conflict_candidate_count": 1},
					"conflict_candidates": []any{map[string]any{"type": "frequency_energy_overlap_candidate", "region": "bass", "tracks": []any{
						map[string]any{"track_id": "1007", "name": "Bass"}, map[string]any{"track_id": "1012", "name": "Drums"},
					}}},
				},
			}},
		}},
	}}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: relationship,
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "choose a target"},
	}, ctx)
	if err != nil || len(state.Frontier.Candidates) != 1 || state.Frontier.CandidateID != "" || state.NoProgressStreak != 0 {
		t.Fatalf("candidate discovery did not advance the frontier: state=%+v err=%v", state, err)
	}
	candidate := state.Frontier.Candidates[0]
	if len(candidate.TrackIDs) != 2 || candidate.TrackIDs[0] != "1007" || candidate.TrackIDs[1] != "1012" {
		t.Fatalf("candidate track set = %+v", candidate)
	}

	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit target round: admitted=%v err=%v", admitted, err)
	}
	targeted := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-bass",
		"requested_views": []any{"track.timbre_frequency"}, "target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
		"views": map[string]any{"track.timbre_frequency": map[string]any{"status": "ready"}},
	}}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: targeted,
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "evaluate target evidence"},
	}, ctx)
	if err != nil || state.Frontier.CandidateID != candidate.ID || state.NoProgressStreak != 0 {
		t.Fatalf("target observation did not select the candidate: state=%+v err=%v", state, err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit improvement round: admitted=%v err=%v", admitted, err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
			EvidenceStatus: "plausible", Summary: "target evidence supports a bounded improvement hypothesis",
		},
	}, ctx)
	if err != nil || state.Terminal() || state.Actionability != audioclosure.ActionabilityActionable {
		t.Fatalf("improvement proposal was not retained as an actionable closure handoff: state=%+v err=%v", state, err)
	}
}

func TestAudioClosureSelectsCandidateFromContinuationLedgerObservation(t *testing.T) {
	server := New(nil, nil, nil)
	ctx, state := prepareAudioClosureTestState(t, server)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit discovery round: admitted=%v err=%v", admitted, err)
	}
	relationship := &agentloop.RecentObservation{
		Tool: "ccb.observation_request", Status: "ready",
		Summary: map[string]any{
			"status": "ready", "observation_id": "obs-relationship",
			"project_package": map[string]any{
				"conflict_candidates": map[string]any{"candidates": []any{map[string]any{
					"type": "frequency_energy_overlap_candidate", "region": "bass",
					"tracks": []any{map[string]any{"track_id": "1007"}, map[string]any{"track_id": "1012"}},
				}}},
			},
		},
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: relationship,
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "inspect the bass target"},
	}, ctx)
	if err != nil || len(state.Frontier.Candidates) != 1 {
		t.Fatalf("candidate discovery failed: state=%+v err=%v", state, err)
	}
	candidateID := state.Frontier.Candidates[0].ID

	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "ledger-target-loop", ConversationID: "conversation-1",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
	})
	continued := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "ledger-target-loop", ConversationID: "conversation-1",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"available_views": map[string]any{"track.band_dynamics": map[string]any{
				"view_id": "track.band_dynamics", "status": "partial", "observation_id": "obs-bass-target", "round": 5,
				"target_ref":    map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
				"freshness":     map[string]any{"status": "ready", "observed_at": "2026-08-22T04:41:21Z"},
				"evidence_refs": []any{"evidence://bass-target"},
			}},
		},
	}
	result := agentloop.Result{
		Continuation: &agentloop.Continuation{Context: map[string]any{"free_state_reasoning_loop": freeStateLoopMap(continued)}},
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "evaluate the target evidence"},
	}
	loop, ok := server.recordFreeStateDecision("conversation-1", result)
	if !ok || loop.LatestObservation == nil {
		t.Fatalf("continuation ledger target was not restored: %#v", loop)
	}
	result.RecentObservation = loop.LatestObservation
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("admit target round: admitted=%v err=%v", admitted, err)
	}
	state, err = server.recordAudioClosureRound(state, result, ctx)
	if err != nil || state.Frontier.CandidateID != candidateID || state.NoProgressStreak != 0 {
		t.Fatalf("ledger-only target observation did not advance closure: state=%+v err=%v", state, err)
	}
}

func TestAudioClosureBuildsCandidateFrontierFromDurableObservationPackage(t *testing.T) {
	observation := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"status": "ready", "observation_id": "obs-durable-package",
		"mom_projection": map[string]any{"multitrack_relation": map[string]any{
			"status": "ready", "band_conflict_candidates": []any{map[string]any{
				"band": "low_mid", "type": "low_mid_masking_candidate",
				"evidence_ref": "project_package.project_band_occupancy.low_mid",
				"tracks":       []any{map[string]any{"track_id": "track-a", "name": "Keys"}, map[string]any{"track_id": "track-b", "name": "Lead"}},
			}},
		}},
		"project_package": map[string]any{"conflict_candidates": map[string]any{"candidates": []any{map[string]any{
			"band": "low_mid", "type": "low_mid_masking_candidate", "evidence_ref": "project_package.project_band_occupancy.low_mid",
			"tracks": []any{map[string]any{"track_id": "track-a", "name": "Keys"}, map[string]any{"track_id": "track-b", "name": "Lead"}},
		}}}},
	}}
	frontier, actionability := audioClosureFrontier(audioclosure.HypothesisFrontier{}, agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
	}, []*agentloop.RecentObservation{observation})
	if actionability != audioclosure.ActionabilityUnknown || len(frontier.Candidates) != 1 {
		t.Fatalf("durable observation package did not produce one candidate: actionability=%v frontier=%+v", actionability, frontier)
	}
	candidate := frontier.Candidates[0]
	if candidate.ViewID != "mix.multitrack_relationship" || len(candidate.TrackIDs) != 2 || candidate.EvidenceRefs[0] != "project_package.project_band_occupancy.low_mid" {
		t.Fatalf("durable candidate lost target/evidence binding: %+v", candidate)
	}
}

func TestAudioClosureHydratesFrontierFromDurableLedgerWhenResultIsEmpty(t *testing.T) {
	ledger := map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"mix.multitrack_relationship": map[string]any{
				"status": "ready", "observation_id": "obs-mix", "view_id": "mix.multitrack_relationship",
				"freshness": map[string]any{"status": "current_observation", "project_revision": "2"},
				"conclusion": map[string]any{
					"facts": map[string]any{
						"band_conflict_candidates": []any{
							map[string]any{
								"type": "low_end_overlap", "band": "bass",
								"tracks": []any{
									map[string]any{"track_id": "1007"},
									map[string]any{"track_id": "1012"},
								},
							},
						},
					},
				},
			},
			"track:1007::track.band_dynamics": map[string]any{
				"status": "ready", "view_id": "track.band_dynamics", "observation_id": "obs-target",
				"target_ref": map[string]any{"kind": "track", "id": "1007"},
				"freshness":  map[string]any{"status": "current_observation", "project_revision": "2"},
			},
		},
	}
	observations := freeStateLedgerObservations(ledger)
	frontier, _ := audioClosureFrontier(audioclosure.HypothesisFrontier{}, agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema}, observations)
	if len(frontier.Candidates) != 1 || frontier.CandidateID == "" {
		t.Fatalf("durable ledger frontier=%+v observations=%+v", frontier, observations)
	}
	if frontier.Candidates[0].SourceObservationID != "obs-mix" || frontier.CandidateID != frontier.Candidates[0].ID {
		t.Fatalf("candidate binding lost: %+v", frontier.Candidates[0])
	}
}

func TestAudioClosureCandidateSelectionReceivesTwoModelTurns(t *testing.T) {
	base := agentloop.Budget{MaxTurns: 6, MaxToolCalls: 6}
	candidateContext := map[string]any{audioClosureContextKey: map[string]any{"hypothesis_frontier": map[string]any{
		"candidates": []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}}},
	}}}
	if got := audioClosureMessageLoopBudget(base, candidateContext).MaxTurns; got != 2 {
		t.Fatalf("candidate selection max turns=%d want 2", got)
	}
	candidateContext[audioClosureContextKey].(map[string]any)["hypothesis_frontier"].(map[string]any)["candidate_id"] = "candidate-1"
	if got := audioClosureMessageLoopBudget(base, candidateContext).MaxTurns; got != 1 {
		t.Fatalf("selected candidate max turns=%d want 1", got)
	}
}

func TestAudioClosureRepairClarificationBecomesProtocolFailure(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	state, _, _ = server.admitAudioClosureRound(state)
	state, err := server.recordAudioClosureRound(state, agentloop.Result{
		NeedsClarification: true, StopReason: agentloop.StopReasonNeedsClarification,
		ModelProtocolRepairs: 1,
	}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Terminal() || state.Settlement.Reason != audioclosure.StopModelProtocolFailure || state.Settlement.NeedsUserClarification {
		t.Fatalf("repaired empty output leaked as user clarification: %+v", state)
	}
	response := server.audioClosureResponse("conversation-1", agentModeDefault, state, agentloop.Result{})
	if response.StopReason != string(audioclosure.StopModelProtocolFailure) || response.GoalStatus != string(agentruntime.StatusFailed) {
		t.Fatalf("protocol settlement response is not typed: %+v", response)
	}
}

func TestAudioClosureProjectRevisionChangeSettlesBeforeNextRound(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	ctx["project_revision"] = "revision-2"
	_, stale, tracked, err := server.prepareAudioClosureContext("conversation-1", "continue", ctx)
	if err != nil || !tracked {
		t.Fatalf("stale closure: tracked=%v state=%+v err=%v", tracked, stale, err)
	}
	if !stale.Terminal() || stale.Settlement.Reason != audioclosure.StopProjectRevisionStale || stale.RoundsStarted != state.RoundsStarted {
		t.Fatalf("revision change did not settle before a model round: %+v", stale)
	}
	if _, ok := server.controllerOwners.Active("conversation-1"); ok {
		t.Fatal("stale closure retained controller ownership")
	}
}

func TestAudioClosureCapabilityHandoffAndSettlementAreExactlyOnce(t *testing.T) {
	server := audioClosureTestServer()
	server.orchestrationRuntime = orchestrationruntime.New()
	if _, err := server.orchestrationRuntime.StartAgentSemanticEQChatSession("capability-session-1", "conversation-1", "project-1", "make the vocal less harsh", orchestration.InteractionPropose); err != nil {
		t.Fatal(err)
	}
	ctx, state := prepareAudioClosureTestState(t, server)
	state, _, _ = server.admitAudioClosureRound(state)
	decision := &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
		EvidenceStatus: "sufficient", Summary: "EQ candidate", ProcessorType: "eq", ObservationID: "observation-1",
	}
	state, err := server.recordAudioClosureRound(state, agentloop.Result{FreeStateDecision: decision}, ctx)
	if err != nil || state.Terminal() || state.Actionability != audioclosure.ActionabilityActionable {
		t.Fatalf("actionable round failed: state=%+v err=%v", state, err)
	}
	response, state := server.bindAudioClosureCapabilityHandoff(ChatResponse{
		ConversationID: "conversation-1", GoalStatus: string(agentruntime.StatusWaitingConfirmation), NeedsConfirmation: true,
		Workflow: "capability_runtime_v1", PlanID: "proposal-1",
		WorkflowData: map[string]any{"session_id": "capability-session-1", "capability_id": "agent.effect.eq_control.v0", "proposal_id": "proposal-1"},
	}, state)
	if state.ActiveCapability == nil || state.ActionAttempts != 1 || response.WorkflowData[audioClosureContextKey] == nil {
		t.Fatalf("capability handoff was not linked: state=%+v response=%+v", state, response)
	}
	child, ok := server.orchestrationRuntime.Store.Load("capability-session-1")
	if !ok || child.ParentController == nil || child.ParentController.ClosureID != state.ClosureID || child.ParentController.ActionID != "proposal-1" {
		t.Fatalf("capability child omitted parent correlation: child=%+v ok=%v", child, ok)
	}
	completed := ChatResponse{ConversationID: "conversation-1", GoalStatus: string(agentruntime.StatusCompleted), Workflow: "capability_runtime_v1", WorkflowData: map[string]any{"session_id": "capability-session-1", "capability_id": "agent.effect.eq_control.v0"}}
	_, state, tracked := server.recordAudioClosureCapabilityResponse("conversation-1", completed)
	// With the FS spine active, capability settle/verify is a sub-state, not a
	// phase: the closure keeps its FS phase (here fs1) while the capability
	// settlement enters the verification stage.
	if !tracked || state.ActiveCapability != nil || state.ActionAttempts != 1 ||
		state.Phase != audioclosure.PhaseFS1ProjectBound {
		t.Fatalf("capability settlement did not settle cleanly under the FS phase: tracked=%v state=%+v", tracked, state)
	}
	revision := state.Revision
	_, state, tracked = server.recordAudioClosureCapabilityResponse("conversation-1", completed)
	if !tracked || state.Revision != revision || state.ActionAttempts != 1 {
		t.Fatalf("duplicate capability response changed parent state: tracked=%v state=%+v", tracked, state)
	}
}

func TestProjectRuntimeSnapshotRestoresClosureAndControllerOwner(t *testing.T) {
	server := audioClosureTestServer()
	_, state := prepareAudioClosureTestState(t, server)
	snapshot := server.projectAgentRuntimeStateLocked()
	if len(snapshot.AudioClosures) != 1 || len(snapshot.ControllerOwners) != 1 {
		t.Fatalf("closure runtime was not projected: %+v", snapshot)
	}
	restored := audioClosureTestServer()
	restored.restoreProjectAgentRuntimeStateLocked(snapshot)
	active, ok := restored.audioClosures.ActiveForConversation("conversation-1")
	if !ok || active.ClosureID != state.ClosureID {
		t.Fatalf("closure runtime did not restore: active=%+v ok=%v", active, ok)
	}
	owner, ok := restored.controllerOwners.Active("conversation-1")
	if !ok || owner.ControllerID != state.ClosureID {
		t.Fatalf("controller owner did not restore: owner=%+v ok=%v", owner, ok)
	}
}

func TestRuntimeControllerAssignmentIsRequiredAfterSemanticEntry(t *testing.T) {
	available := []string{semanticEntryScopeNone, semanticEntryScopeProjectContext}
	closureJSON := `{"schema_version":"semantic_entry_decision.v1","route":"open_semantic","target_scope":"project_context","control_mode":"semantic_loop","user_authorization":"action_requested","confidence":0.9,"reason":"one project-context acoustic issue"}`
	decision, err := decodeSemanticEntryDecision(closureJSON, available)
	if err != nil || decision.Controller != "" {
		t.Fatalf("semantic classification unexpectedly assigned controller: %+v err=%v", decision, err)
	}
	decision.Controller = string(orchestrationcontroller.MinimalAudioClosure)
	if _, err := orchestrationControllerDecision(decision); err != nil {
		t.Fatalf("runtime-assigned controller was rejected: %v", err)
	}
}

func TestAudioClosureRoundBoundariesDriveFSSpineAndDiagnosticRounds(t *testing.T) {
	server := audioClosureTestServer()
	ctx, state := prepareAudioClosureTestState(t, server)
	requestContext := mergeContext(ctx, map[string]any{
		"free_state_capacity_assessment": map[string]any{
			"schema_version": "free_state_capacity_assessment.v1",
			"capacity_level": "within_free_state", "selected_capability": "project_mix",
		},
	})
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != audioclosure.PhaseFS1ProjectBound {
		t.Fatalf("round admission did not enter/advance the FS spine: %s", state.Phase)
	}
	// A scan-level mix observation plus a frontier update drive the spine to
	// the candidate frontier on the record boundary.
	scan := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"status": "ready", "observation_id": "obs-scan",
		"view_ids":        []any{"mix.frequency_relationship"},
		"target_ref":      map[string]any{"kind": "project", "id": "current"},
		"project_binding": map[string]any{"project_uuid": "project-1", "project_revision": "revision-1"},
		"views":           map[string]any{"mix.frequency_relationship": map[string]any{"status": "ready"}},
	}}
	result := agentloop.Result{RecentObservation: scan, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "scan complete",
	}}
	state, err = server.recordAudioClosureRound(state, result, requestContext)
	if err != nil {
		t.Fatal(err)
	}
	if state.Phase != audioclosure.PhaseFS4DiagnosticRound && state.Phase != audioclosure.PhaseFS5CandidateFrontier {
		t.Fatalf("record boundary did not advance the spine past the scan: %s", state.Phase)
	}
	if len(state.DiagnosticRounds) != 1 || state.DiagnosticRounds[0].PrimaryDimension != audioclosure.DimensionFrequencyOccupancy {
		t.Fatalf("diagnostic round record missing or wrong dimension: %+v", state.DiagnosticRounds)
	}
	if state.DiagnosticRounds[0].EvidenceStatus != audioclosure.RoundEvidenceReady {
		t.Fatalf("round evidence status = %s", state.DiagnosticRounds[0].EvidenceStatus)
	}
	// The bound context exposes the FS phase under the dedicated key.
	bound := bindAudioClosureContext(map[string]any{}, state)
	if phase, ok := bound["free_state_phase"].(string); !ok || !strings.HasPrefix(phase, "fs") {
		t.Fatalf("free_state_phase context key missing or non-FS: %v", bound["free_state_phase"])
	}
}

func TestAdvanceAudioClosurePhaseDerivesGuardsFromStateOnly(t *testing.T) {
	server := audioClosureTestServer()
	_, state := prepareAudioClosureTestState(t, server)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	// A caller asserting capacity without scan evidence cannot force FS3: the
	// guard derives scan usability from recorded observations.
	advanced := server.advanceAudioClosurePhase(state, audioclosure.PhaseGuardEvidence{CapacityAssessed: true})
	if advanced.Phase != audioclosure.PhaseFS2CapacityAssessed {
		t.Fatalf("self-asserted scan evidence advanced past the derived guard: %s", advanced.Phase)
	}
}
// TestAudioClosureRoundRecordKeepsDimensionOpenOnUnboundOrEmptyDisclosure is
// the DIAG2 guardrail: a dimension may close only on disclosure quality — a
// target-bound observation with usable disclosed evidence. The DIAG1 failure
// chain closed dynamics after one unbound track.time_dynamics view (the
// track-less evidence shell), institutionalizing a single bad draw.
func TestAudioClosureRoundRecordKeepsDimensionOpenOnUnboundOrEmptyDisclosure(t *testing.T) {
	now := time.Now().UTC()
	driver := audioclosure.Driver{}
	startRound := func(closureID string) audioclosure.State {
		state, err := audioclosure.Start(audioclosure.StartRequest{
			ClosureID: closureID, ConversationID: "conversation-" + closureID,
			ProjectUUID: "project-1", ProjectRevision: "16", OriginalIntent: "inspect the project",
			Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-1"}, Now: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		state, _, err = driver.AdmitRound(state, state.Revision, now)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	bundleObservation := func(id, status string) *agentloop.RecentObservation {
		return &agentloop.RecentObservation{
			Tool: "ccb.observation_request", CommandName: "ccb_observation_request", Status: status,
			Summary: map[string]any{"observation_id": id, "status": status, "view_ids": []any{"track.time_dynamics"}},
		}
	}
	recordRound := func(state audioclosure.State, targetRef, observationID string) audioclosure.State {
		outcome, err := driver.RecordObservation(state, state.Revision, audioclosure.ObservationKey{
			ProjectUUID: "project-1", ProjectRevision: "16",
			Scope: audioclosure.Scope{Kind: "project", ID: "project-1"},
			TargetRef: targetRef, ViewIDs: []string{"track.time_dynamics"},
		}, observationID, now)
		if err != nil {
			t.Fatal(err)
		}
		return outcome.State
	}
	dynamicsEntry := func(state audioclosure.State) audioclosure.PriorityQueueEntry {
		queue := audioclosure.QueueFromRounds(state.DiagnosticRounds)
		for _, entry := range queue.Entries {
			if entry.Dimension == audioclosure.DimensionDynamics {
				return entry
			}
		}
		t.Fatalf("dynamics entry missing from queue: %+v", queue.Entries)
		return audioclosure.PriorityQueueEntry{}
	}

	// Unbound track view (the DIAG1 empty shell): dimension stays open.
	state := recordRound(startRound("closure-diag2-unbound"), "", "obs-unbound")
	round, ok := audioClosureRoundRecord(state, []*agentloop.RecentObservation{bundleObservation("obs-unbound", "ready")})
	if !ok {
		t.Fatal("unbound round produced no diagnostic round record")
	}
	if round.EvidenceStatus == audioclosure.RoundEvidenceReady {
		t.Fatalf("unbound track disclosure was marked ready: %+v", round)
	}
	state, err := driver.RecordDiagnosticRound(state, state.Revision, round, now)
	if err != nil {
		t.Fatal(err)
	}
	if entry := dynamicsEntry(state); entry.Status != audioclosure.QueueOpen {
		t.Fatalf("dynamics closed after unbound disclosure: %+v", entry)
	}

	// Empty disclosure (bound observation, insufficient bundle): stays open.
	state = recordRound(startRound("closure-diag2-empty"), "1007", "obs-empty")
	round, ok = audioClosureRoundRecord(state, []*agentloop.RecentObservation{bundleObservation("obs-empty", "insufficient")})
	if !ok {
		t.Fatal("empty-disclosure round produced no diagnostic round record")
	}
	if round.EvidenceStatus == audioclosure.RoundEvidenceReady {
		t.Fatalf("empty disclosure was marked ready: %+v", round)
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, round, now)
	if err != nil {
		t.Fatal(err)
	}
	if entry := dynamicsEntry(state); entry.Status != audioclosure.QueueOpen {
		t.Fatalf("dynamics closed after empty disclosure: %+v", entry)
	}

	// Bound observation with usable disclosure still closes the dimension.
	state = recordRound(startRound("closure-diag2-bound"), "1007", "obs-bound")
	round, ok = audioClosureRoundRecord(state, []*agentloop.RecentObservation{bundleObservation("obs-bound", "ready")})
	if !ok {
		t.Fatal("bound round produced no diagnostic round record")
	}
	if round.EvidenceStatus != audioclosure.RoundEvidenceReady {
		t.Fatalf("bound usable disclosure was not marked ready: %+v", round)
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, round, now)
	if err != nil {
		t.Fatal(err)
	}
	if entry := dynamicsEntry(state); entry.Status != audioclosure.QueueClosed {
		t.Fatalf("bound usable disclosure did not close dynamics: %+v", entry)
	}
}

// TestAudioClosureRoundRecordRestrictsViewsToPrimaryDimension mirrors the
// 19:47 D1 smoke closure: the project-level bundle spans several dimensions
// (masking + multitrack + structure) and the bass bundle mixes a level view
// with a frequency view. The round record contract requires views_requested
// to be a subset of the primary dimension's allowed views; a verbatim copy
// made RecordDiagnosticRound reject both rounds, so the G4 round data source
// never persisted and DimensionClosed stayed false.
func TestAudioClosureRoundRecordRestrictsViewsToPrimaryDimension(t *testing.T) {
	now := time.Now().UTC()
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-round-record", ConversationID: "conversation-round-record",
		ProjectUUID: "project-1", ProjectRevision: "16", OriginalIntent: "inspect the project",
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-1"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	driver := audioclosure.Driver{}
	state, _, err = driver.AdmitRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	var outcome audioclosure.ObservationOutcome
	outcome, err = driver.RecordObservation(state, state.Revision, audioclosure.ObservationKey{
		ProjectUUID: "project-1", ProjectRevision: "16",
		Scope: audioclosure.Scope{Kind: "project", ID: "project-1"}, TargetRef: "project-1",
		ViewIDs: []string{"mix.masking_relationship", "mix.multitrack_relationship", "project.structure"},
	}, "obs-project", now)
	state = outcome.State
	if err != nil {
		t.Fatal(err)
	}
	round2, ok := audioClosureRoundRecord(state, nil)
	if !ok {
		t.Fatal("round 2 produced no diagnostic round record")
	}
	if err := round2.Validate(); err != nil {
		t.Fatalf("round 2 record failed the contract machine check: %v (record=%+v)", err, round2)
	}
	if round2.PrimaryDimension != audioclosure.DimensionLevelHeadroom {
		t.Fatalf("round 2 primary dimension = %s, want level_headroom", round2.PrimaryDimension)
	}
	if got := strings.Join(round2.ViewsRequested, ","); got != "mix.multitrack_relationship,project.structure" {
		t.Fatalf("round 2 views_requested = %q, want the primary-dimension subset", got)
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, round2, now)
	if err != nil {
		t.Fatalf("round 2 record did not persist: %v", err)
	}

	state, err = driver.CompleteRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = driver.AdmitRound(state, state.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err = driver.RecordObservation(state, state.Revision, audioclosure.ObservationKey{
		ProjectUUID: "project-1", ProjectRevision: "16",
		Scope: audioclosure.Scope{Kind: "track", ID: "1007"}, TargetRef: "1007",
		ViewIDs: []string{"track.basic_energy", "track.timbre_frequency"},
	}, "obs-bass", now)
	state = outcome.State
	if err != nil {
		t.Fatal(err)
	}
	round3, ok := audioClosureRoundRecord(state, nil)
	if !ok {
		t.Fatal("round 3 produced no diagnostic round record")
	}
	if err := round3.Validate(); err != nil {
		t.Fatalf("round 3 record failed the contract machine check: %v (record=%+v)", err, round3)
	}
	if round3.PrimaryDimension != audioclosure.DimensionLevelHeadroom || strings.Join(round3.ViewsRequested, ",") != "track.basic_energy" {
		t.Fatalf("round 3 record = %s %v, want level_headroom [track.basic_energy]", round3.PrimaryDimension, round3.ViewsRequested)
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, round3, now)
	if err != nil {
		t.Fatalf("round 3 record did not persist: %v", err)
	}
	if len(state.DiagnosticRounds) != 2 {
		t.Fatalf("diagnostic rounds persisted = %d, want 2", len(state.DiagnosticRounds))
	}
	guard := audioclosure.DerivePhaseGuardInput(state, audioclosure.PhaseGuardEvidence{})
	if !guard.DimensionClosed {
		t.Fatalf("G4 dimension_closed must be true after persisted ready rounds: %+v", guard)
	}
}

// TestAudioClosureRoundClosePersistsRoundsAndAdvancesToFS6 drives the full
// server round-close path with the 19:47 smoke shape: project-level bundle in
// round 2, target-level bass evidence in round 3. Each completed round must
// persist its diagnostic round record so QueueFromRounds/DimensionClosed can
// close the dimension and the phase machine advances to FS6 where
// needs_experiment is legal.
func TestAudioClosureRoundClosePersistsRoundsAndAdvancesToFS6(t *testing.T) {
	server := &Server{harness: harness.NewWithSender(nil, nil, nil), audioClosures: audioclosure.NewMemoryStore(),
		controllerOwners: orchestrationcontroller.NewRegistry(), freeStateLoops: map[string]freeStateReasoningLoop{},
		capabilityRoutes: map[string]CapabilityRouteRecord{}}
	goal := server.harness.EnsureGoal("goal-round-close-fs6", "run-round-close-fs6", "inspect the project")
	if _, err := server.ensureAudioTaskContract("conversation-round-close-fs6", audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-round-close-fs6"}, "project-round-close-fs6", "rev-1", map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	server.capabilityRoutes["route-round-close-fs6"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1", ConversationID: "conversation-round-close-fs6",
		Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{ClosureID: "closure-round-close-fs6", ConversationID: "conversation-round-close-fs6",
		TaskID: current.Task.TaskID, GoalID: goal.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-round-close-fs6", ProjectRevision: "rev-1", OriginalIntent: "inspect the project",
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-round-close-fs6"}, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	ctx := map[string]any{"project_uuid": "project-round-close-fs6", "project_revision": "rev-1"}

	// Round 1: no observation; closes without a round record.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("round 1 admission: admitted=%v err=%v", admitted, err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, ctx)
	if err != nil || len(state.DiagnosticRounds) != 0 {
		t.Fatalf("round 1 close: rounds=%d err=%v", len(state.DiagnosticRounds), err)
	}

	// Round 2: project-level bundle spanning several dimensions.
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("round 2 admission: admitted=%v err=%v", admitted, err)
	}
	project := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-project",
		"requested_views": []any{"mix.masking_relationship", "mix.multitrack_relationship", "project.structure"},
		"views": map[string]any{
			"mix.masking_relationship": map[string]any{"status": "ready"},
			"mix.multitrack_relationship": map[string]any{"status": "ready", "facts": map[string]any{"band_conflict_candidates": []any{
				map[string]any{"type": "low_end_overlap", "band": "bass", "tracks": []any{
					map[string]any{"track_id": "1007"}, map[string]any{"track_id": "1012"}, map[string]any{"track_id": "1017"}}},
			}}},
			"project.structure": map[string]any{"status": "ready"},
		},
	}}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: project,
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "project scan evidence"},
	}, ctx)
	if err != nil || len(state.DiagnosticRounds) != 1 || len(state.Frontier.Candidates) == 0 {
		t.Fatalf("round 2 close: rounds=%d candidates=%d err=%v", len(state.DiagnosticRounds), len(state.Frontier.Candidates), err)
	}
	if err := state.DiagnosticRounds[0].Validate(); err != nil {
		t.Fatalf("round 2 persisted record is invalid: %v (%+v)", err, state.DiagnosticRounds[0])
	}

	// Round 3: target-level bass evidence selects the frontier candidate.
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("round 3 admission: admitted=%v err=%v", admitted, err)
	}
	bass := &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ready", Summary: map[string]any{
		"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-bass",
		"requested_views": []any{"track.basic_energy", "track.timbre_frequency"},
		"target_ref":      map[string]any{"kind": "track", "id": "1007", "label": "bass"},
		"views": map[string]any{
			"track.basic_energy":    map[string]any{"status": "ready"},
			"track.timbre_frequency": map[string]any{"status": "ready"},
		},
	}}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{
		RecentObservation: bass,
		FreeStateDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema,
			Status: agentloop.FreeStateNeedsObservation, EvidenceStatus: "insufficient", Summary: "target evidence"},
	}, ctx)
	if err != nil {
		t.Fatalf("round 3 close: %v", err)
	}
	if len(state.DiagnosticRounds) != 2 {
		t.Fatalf("round 3 did not persist its diagnostic round: %d records", len(state.DiagnosticRounds))
	}
	for _, record := range state.DiagnosticRounds {
		if err := record.Validate(); err != nil {
			t.Fatalf("persisted round record is invalid: %v (%+v)", err, record)
		}
	}
	guard := audioclosure.DerivePhaseGuardInput(state, audioclosure.PhaseGuardEvidence{})
	if !guard.DimensionClosed || !guard.FrontierEstablished || !guard.TargetEvidence {
		t.Fatalf("FS guards not satisfied after round 3: %+v", guard)
	}
	if state.Phase != audioclosure.PhaseFS6TargetConfirmed {
		t.Fatalf("round-close advance must reach FS6, got %s", state.Phase)
	}
	if !audioclosure.AllowsDecisionStatus(state.Phase, agentloop.FreeStateNeedsExperiment) {
		t.Fatalf("needs_experiment must be legal at %s", state.Phase)
	}
	if state.Terminal() || len(state.Observations) != 2 {
		t.Fatalf("closure state diverged: terminal=%v observations=%d", state.Terminal(), len(state.Observations))
	}
}



func TestSyncAudioClosureGovernedRevisionBooksAppliedRevision(t *testing.T) {
	server := &Server{audioClosures: audioclosure.NewMemoryStore()}
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-governed-sync", ConversationID: "conversation-governed-sync", GoalID: "goal-governed-sync", RunID: "run-governed-sync",
		ProjectUUID: "project-governed-sync", ProjectRevision: "rev-1", OriginalIntent: "make the vocal less harsh",
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "track", ID: "track-vocal"}, Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = (audioclosure.Driver{}).BeginCapability(state, state.Revision, audioclosure.CapabilityLink{
		SessionID: "session-governed-sync", CapabilityID: "agent.effect.eq_control.v0", ActionID: "action-governed-sync", ExpectedProjectRevision: "rev-1"}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	server.syncAudioClosureGovernedRevision("conversation-governed-sync", "rev-3")
	synced, ok := server.audioClosures.ActiveForConversation("conversation-governed-sync")
	if !ok {
		t.Fatal("closure disappeared after governed revision sync")
	}
	if synced.ProjectRevision != "rev-3" {
		t.Fatalf("tracked revision not booked: %q", synced.ProjectRevision)
	}
	if synced.ActiveCapability == nil {
		t.Fatal("capability must stay active across the governed revision booking")
	}
	server.syncAudioClosureGovernedRevision("conversation-missing", "rev-4")
	if again, _ := server.audioClosures.ActiveForConversation("conversation-governed-sync"); again.Revision != synced.Revision {
		t.Fatal("unknown conversation must not touch the closure")
	}
}

// prepareClosureRoundBoundary builds a contract-bound closure whose only
// admitted round has started and completed, so the next admission sits exactly
// on the closure round boundary.
func prepareClosureRoundBoundary(t *testing.T, s *Server, conversationID, goalID, projectRevision string) audioclosure.State {
	t.Helper()
	goal := s.harness.EnsureGoal(goalID, "run-"+goalID, "improve the mix")
	if _, err := s.ensureAudioTaskContract(conversationID, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "track", ID: "vocal"}, "project-d2", projectRevision, map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	current := s.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-" + conversationID, ConversationID: conversationID, TaskID: current.Task.TaskID,
		GoalID: goal.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-d2", ProjectRevision: projectRevision, OriginalIntent: current.Task.OriginalIntent,
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "track", ID: "vocal"},
		Policy: audioclosure.Policy{MaxClosureRounds: 1, MaxUniqueObservations: 4, MaxNoProgressRounds: 2, MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1},
		Now:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	state, admitted, err := s.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("boundary closure round 1 admission: admitted=%v err=%v", admitted, err)
	}
	state, err = s.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{"project_uuid": "project-d2", "project_revision": projectRevision})
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() || state.RoundInProgress {
		t.Fatalf("boundary closure did not close round 1 cleanly: %+v", state)
	}
	return state
}

// wireLoopExperimentToContract binds the loop's started experiment onto the
// contract task (the canonical chain: improvement proposed -> experiment
// required), so the judgment flow runs with production task semantics.
func wireLoopExperimentToContract(t *testing.T, s *Server, loop *freeStateReasoningLoop, projectRevision string) {
	t.Helper()
	if _, err := s.transitionTaskSemantic(loop.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventImprovementProposed, Reason: "candidate observed", EvidenceRefs: []string{"obs-before"},
		CandidateID: "vocal", ProjectRevision: projectRevision,
		Proposal: &taskstate.BoundedProposal{ProposalID: "proposal-" + loop.LoopID, Summary: "bounded track gain", EvidenceRefs: []string{"obs-before"}, RequiresExperiment: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.bindExperimentSemantic(loop); err != nil {
		t.Fatal(err)
	}
}

// driveNoDifferenceRecalibration lands the machine-origin no-difference
// judgment on the loop experiment's current round, opening the recalibration
// round (the S3b smoke shape).
func driveNoDifferenceRecalibration(t *testing.T, s *Server, loop *freeStateReasoningLoop) {
	t.Helper()
	d2ApplyAndObserveForTest(t, loop.Experiment, "d2-action-1", "8")
	loop.LatestProjectChange = map[string]any{"project_revision": "8"}
	if _, err := loop.Experiment.EvaluateMateriality(experiment.MaterialityEvaluation{State: experiment.MaterialityMaterial, Evaluation: trajectory.EvaluationAgentEvaluable, Attempt: 1, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RecordTargetResponse(experiment.TargetEvaluation{Response: experiment.TargetAmbiguous, Outcome: trajectory.EvaluationHumanAuditionReady, EvidenceRefs: []string{"obs-d2-action-1"}}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// Production order: the canonical human-judgment transition precedes the
	// experiment's judgment request (requestAuditionJudgment).
	if err := s.requireTaskHumanJudgment(loop, "session-recalibration", "A/B audition judgment is required before experiment settlement"); err != nil {
		t.Fatal(err)
	}
	if _, err := loop.Experiment.RequestUserJudgmentForSession("A/B audition required", "session-recalibration", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatal(err)
	}
	evidence := experiment.UserJudgmentEvidence{
		SchemaVersion: experiment.UserJudgmentEvidenceSchemaVersion, ConversationID: loop.ConversationID,
		TurnID: loop.Experiment.ID, RoundID: round.ID, AuditionSessionID: "session-recalibration",
		CandidateARef: "before.wav", CandidateBRef: "after.wav",
		HeardDifference: experiment.HeardDifferenceNo, Preference: experiment.PreferenceUnsure, CreatedAt: time.Now().UTC(),
	}
	if _, err := loop.Experiment.RecordUserJudgmentEvidence(evidence, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), loop, evidence); err != nil {
		t.Fatal(err)
	}
	s.storeFreeStateLoop(*loop)
}

// TestMultiRoundRecalibrationBoundaryExtendsClosureRounds locks the S3c branch:
// a freshly opened recalibration round still owes its single governed
// intervention, and the closure round limit (an observation-window bound) must
// extend for it instead of settling capability_blocked. At that boundary
// neither legacy extension condition holds — round 1's post-action observation
// is booked (RequiresPostActionObservation=false) and the new round carries no
// fresh post-action evidence (freeStateLoopRoundPendingSettlement=false) — so
// without the owed-intervention branch any scheduler re-entry kills the loop
// before round 2 can act (2026-08-29 S3b smoke, "closure observation round
// boundary reached").
func TestMultiRoundRecalibrationBoundaryExtendsClosureRounds(t *testing.T) {
	s, loop := d2MultiRoundServerLoopForTest(t, 2)
	state := prepareClosureRoundBoundary(t, s, loop.ConversationID, loop.GoalID, "8")
	wireLoopExperimentToContract(t, s, &loop, "8")
	driveNoDifferenceRecalibration(t, s, &loop)
	if len(loop.Experiment.Rounds) != 2 {
		t.Fatalf("recalibration round did not open: rounds=%d", len(loop.Experiment.Rounds))
	}
	if loop.RequiresPostActionObservation || freeStateLoopRoundPendingSettlement(loop) {
		t.Fatalf("recalibration boundary unexpectedly owes a settle window: flag=%v pending=%v",
			loop.RequiresPostActionObservation, freeStateLoopRoundPendingSettlement(loop))
	}
	if !freeStateLoopRoundOwesIntervention(loop) {
		t.Fatal("recalibration round was not recognized as owing its intervention")
	}
	state, admitted, err := s.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() || state.RoundsStarted != 2 || state.Policy.MaxClosureRounds != 2 {
		t.Fatalf("boundary did not extend for the owed recalibration round: admitted=%v rounds=%d max=%d state=%+v",
			admitted, state.RoundsStarted, state.Policy.MaxClosureRounds, state)
	}
}

// TestMultiRoundRecalibrationBoundaryExtensionGuards locks the negative space
// of the owed-intervention extension: the frozen contract stays sealed. The
// extension never fires when the current round has already spent its single
// intervention, when the experiment budget is exhausted, or on the single-round
// D1 tier (whose boundary behavior is sealed and never opens a recalibration
// round).
func TestMultiRoundRecalibrationBoundaryExtensionGuards(t *testing.T) {
	t.Run("current round already intervened", func(t *testing.T) {
		s, loop := d2MultiRoundServerLoopForTest(t, 2)
		state := prepareClosureRoundBoundary(t, s, loop.ConversationID, loop.GoalID, "8")
		wireLoopExperimentToContract(t, s, &loop, "8")
		driveNoDifferenceRecalibration(t, s, &loop)
		if _, err := loop.Experiment.ApplyIntervention(experiment.Intervention{
			ID: "d2-action-2", Attempt: 1, TechnicalApplication: experiment.TechnicalApplied, UserConfirmed: true,
			AchievedDelta: map[string]any{"delta_db": -1.0},
			Receipt: map[string]any{"action_id": "d2-action-2", "status": "applied", "after_revision": "9",
				"transaction_id": "tx-d2-action-2", "idempotency_key": "key-d2-action-2", "readback_verified": true},
		}, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		s.storeFreeStateLoop(loop)
		if freeStateLoopRoundOwesIntervention(loop) {
			t.Fatal("round that already acted still reported as owing its intervention")
		}
		state, admitted, err := s.admitAudioClosureRound(state)
		if err != nil || admitted || !state.Terminal() || state.Settlement.Reason != audioclosure.StopCapabilityBlocked {
			t.Fatalf("boundary extended past an already-acted round: admitted=%v state=%+v err=%v", admitted, state, err)
		}
	})

	t.Run("experiment budget exhausted", func(t *testing.T) {
		s, loop := d2MultiRoundServerLoopForTest(t, 2)
		// A contract-illegal shape (rounds beyond the admission budget, current
		// round without its intervention): unreachable through the runtime, used
		// here only to prove the budget guard is enforced by the chat layer
		// itself, whatever a future runtime relaxation allows.
		admission, err := freeStateExperimentAdmissionWithTier(freeStateReasoningLoop{LoopID: "loop-guard-budget", LatestObservation: d1FreshObservationForTest("7")}, experimentTestProposal(), 2)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		guard := loop
		guard.Experiment = &experiment.Turn{
			SchemaVersion: experiment.SchemaVersion, ID: "turn-guard-budget", ConversationID: guard.ConversationID,
			Status: experiment.StatusRunning, Admission: admission, CurrentRoundID: "round-3",
			Rounds: []experiment.Round{
				{ID: "round-1", Number: 1, Status: experiment.RoundCompleted, Decision: experiment.DecisionNextRound,
					Interventions: []experiment.Intervention{{ID: "a1", Attempt: 1}}, StartedAt: now, UpdatedAt: now},
				{ID: "round-2", Number: 2, Status: experiment.RoundCompleted, Decision: experiment.DecisionNextRound,
					Interventions: []experiment.Intervention{{ID: "a2", Attempt: 1}}, StartedAt: now, UpdatedAt: now},
				{ID: "round-3", Number: 3, Status: experiment.RoundObserving, StartedAt: now, UpdatedAt: now},
			},
			StartedAt: now, UpdatedAt: now,
		}
		s.storeFreeStateLoop(guard)
		if freeStateLoopRoundOwesIntervention(guard) {
			t.Fatal("beyond-budget round still reported as owing an intervention")
		}
		state := prepareClosureRoundBoundary(t, s, guard.ConversationID, guard.GoalID, "8")
		state, admitted, err := s.admitAudioClosureRound(state)
		if err != nil || admitted || !state.Terminal() || state.Settlement.Reason != audioclosure.StopCapabilityBlocked {
			t.Fatalf("boundary extended past the experiment budget: admitted=%v state=%+v err=%v", admitted, state, err)
		}
	})

	t.Run("single round D1 tier sealed", func(t *testing.T) {
		s, loop := d2MultiRoundServerLoopForTest(t, 1)
		state := prepareClosureRoundBoundary(t, s, loop.ConversationID, loop.GoalID, "8")
		wireLoopExperimentToContract(t, s, &loop, "8")
		s.storeFreeStateLoop(loop)
		// The running single-round experiment has not intervened yet, so the
		// bare tier-agnostic predicate would extend; the sealed D1 boundary
		// behavior must stay byte-identical instead.
		if !strings.EqualFold(string(loop.Experiment.Status), string(experiment.StatusRunning)) || len(loop.Experiment.Rounds) != 1 {
			t.Fatalf("single-round experiment is not in the pre-intervention window: %+v", loop.Experiment)
		}
		if freeStateLoopRoundOwesIntervention(loop) {
			t.Fatal("single-round tier was admitted to the recalibration extension")
		}
		state, admitted, err := s.admitAudioClosureRound(state)
		if err != nil || admitted || !state.Terminal() || state.Settlement.Reason != audioclosure.StopCapabilityBlocked {
			t.Fatalf("single-round boundary behavior changed: admitted=%v state=%+v err=%v", admitted, state, err)
		}
	})
}

// AGENT-W1（2026-09-05 手测）：no_candidate_found 对单轨/少轨工程不可解释——
// 用户视角"查了 6 轮然后一句通用话术"。结算话术需带工程形态上下文。
func TestAudioClosureNoCandidateReplyCarriesTrackContext(t *testing.T) {
	singleTrack := &audioclosure.Settlement{Reason: audioclosure.StopNoCandidateFound}
	reply := audioClosureSettlementReply(singleTrack, 1)
	if !strings.Contains(reply, "只有 1 轨") {
		t.Fatalf("single-track no-candidate reply must explain project shape: %q", reply)
	}
	if !strings.Contains(reply, "没有可执行的改善建议") {
		t.Fatalf("single-track reply must state the conclusion: %q", reply)
	}

	multiTrack := audioClosureSettlementReply(&audioclosure.Settlement{Reason: audioclosure.StopNoCandidateFound}, 6)
	if !strings.Contains(multiTrack, "在已声明的观察范围内没有发现可信改善候选") {
		t.Fatalf("multi-track reply must keep the generic wording: %q", multiTrack)
	}
	if strings.Contains(multiTrack, "只有 1 轨") {
		t.Fatalf("multi-track reply must not claim single-track shape: %q", multiTrack)
	}

	unknown := audioClosureSettlementReply(&audioclosure.Settlement{Reason: audioclosure.StopNoCandidateFound}, -1)
	if strings.Contains(unknown, "只有 1 轨") {
		t.Fatalf("unknown track count must fall back to generic wording: %q", unknown)
	}

	other := audioClosureSettlementReply(&audioclosure.Settlement{Reason: audioclosure.StopDiagnosticComplete}, 1)
	if strings.Contains(other, "只有 1 轨") {
		t.Fatalf("track context applies only to no_candidate_found: %q", other)
	}
}
