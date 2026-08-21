package chat

import (
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
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
	if !tracked || state.ActiveCapability != nil || state.Phase != audioclosure.PhaseVerifying || state.ActionAttempts != 1 {
		t.Fatalf("capability settlement did not enter verification: tracked=%v state=%+v", tracked, state)
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
