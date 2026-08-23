package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
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
	if !next.Terminal() || next.Settlement == nil || next.Settlement.Reason != audioclosure.StopNoCandidateFound || next.Phase != audioclosure.PhaseFS9Terminal {
		t.Fatalf("exhausted open queue did not settle no_candidate_found at FS9: %+v", next)
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
