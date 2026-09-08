package chat

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
)

// DIAG3-1: the free-state observation loop must close per-track targeting
// coverage (active tracks × open dimensions) before an exhausted-budget
// closure boundary may settle no_candidate_found. The sealed-discipline rule
// is structural: no fixture track identity may appear here, only synthetic
// test-local IDs.

func targetingCoverageTestServer(t *testing.T, trackCount int) (*Server, string) {
	t.Helper()
	trackRows := make([]any, 0, trackCount)
	for i := 1; i <= trackCount; i++ {
		trackRows = append(trackRows, map[string]any{
			"track_id":       "synthetic-track-" + string(rune('a'+i-1)),
			"track_name":     "Synth " + string(rune('A'+i-1)),
			"track_type":     "hybrid",
			"is_audio_track": true,
		})
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"project_uuid": "project-targeting-coverage", "project_revision": "rev-1",
		"tracks": trackRows,
	})
	server := &Server{
		harness:           harness.NewWithSender(nil, shadowProject, nil),
		audioClosures:     audioclosure.NewMemoryStore(),
		controllerOwners:  orchestrationcontroller.NewRegistry(),
		freeStateLoops:    map[string]freeStateReasoningLoop{},
		capabilityRoutes:  map[string]CapabilityRouteRecord{},
		conversationGoals: map[string]string{},
	}
	return server, "conversation-targeting-coverage"
}

func targetingCoverageClosureState(t *testing.T, server *Server, conversationID string, maxClosureRounds int) audioclosure.State {
	t.Helper()
	goal := server.harness.EnsureGoal("goal-targeting-coverage", "run-targeting-coverage", "inspect the project")
	if _, err := server.ensureAudioTaskContract(conversationID, audioclosure.ModeTreatment,
		audioclosure.Scope{Kind: "project", ID: "project-targeting-coverage"}, "project-targeting-coverage", "rev-1",
		map[string]any{"goal_id": goal.GoalID}); err != nil {
		t.Fatal(err)
	}
	current := server.harness.RuntimeStatus(goal.GoalID)
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-targeting-coverage", ConversationID: conversationID, TaskID: current.Task.TaskID,
		GoalID: goal.GoalID, RunID: current.RunID, ContractID: current.Task.Contract.ContractID,
		TaskState: current.Task.SemanticState.State, TaskStateRevision: current.Task.SemanticState.Revision,
		ProjectUUID: "project-targeting-coverage", ProjectRevision: "rev-1", OriginalIntent: "inspect the project",
		Mode: audioclosure.ModeTreatment, Scope: audioclosure.Scope{Kind: "project", ID: "project-targeting-coverage"},
		Policy: audioclosure.Policy{MaxClosureRounds: maxClosureRounds, MaxUniqueObservations: 12, MaxNoProgressRounds: 4, MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1},
		Now:    time.Now().UTC(),
	})
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
	if _, err := server.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{Event: taskstate.EventDiagnosticCompleted, Reason: "test evidence", EvidenceRefs: []string{"obs-targeting-coverage"}, ProjectRevision: "rev-1"}); err != nil {
		t.Fatal(err)
	}
	if err := server.audioClosures.Create(state); err != nil {
		t.Fatal(err)
	}
	return state
}

// targetingCoverageNaiveLedger mimics the D1 failure shape: the model spent
// its requests on project/mix views plus per-track views of one salient track
// only, so every other active track has zero per-track coverage while the
// priority queue still holds open dimensions.
func targetingCoverageNaiveLedger(naiveTrackID string) map[string]any {
	available := map[string]any{
		"project.structure":        map[string]any{"view_id": "project.structure", "status": "ready", "observation_id": "obs-structure"},
		"mix.masking_relationship": map[string]any{"view_id": "mix.masking_relationship", "status": "ready", "observation_id": "obs-masking"},
	}
	for _, viewID := range []string{"track.time_dynamics", "track.transient_structure"} {
		available["track:"+naiveTrackID+"::"+viewID] = map[string]any{
			"view_id": viewID, "status": "ready", "observation_id": "obs-" + naiveTrackID,
			"target_ref": map[string]any{"kind": "track", "id": naiveTrackID},
		}
	}
	return map[string]any{
		"schema_version":  "free_state_observation_ledger.v1",
		"available_views": available,
	}
}

// targetingCoverageLedgerWithViews builds a ledger whose per-track coverage is
// exactly the given track → observed primary views mapping.
func targetingCoverageLedgerWithViews(coverage map[string][]string) map[string]any {
	available := map[string]any{}
	for trackID, views := range coverage {
		for _, viewID := range views {
			available["track:"+trackID+"::"+viewID] = map[string]any{
				"view_id": viewID, "status": "ready", "observation_id": "obs-" + trackID + "-" + viewID,
				"target_ref": map[string]any{"kind": "track", "id": trackID},
			}
		}
	}
	return map[string]any{
		"schema_version":  "free_state_observation_ledger.v1",
		"available_views": available,
	}
}

type targetingCoverageBooking struct {
	TrackID string
	ViewIDs []string
}

// installTargetingCoverageBooker swaps the deterministic observation booking
// for a recordable fake that returns a minimal ready CCB bundle per request.
func installTargetingCoverageBooker(t *testing.T, failAll bool) *[]targetingCoverageBooking {
	t.Helper()
	bookings := &[]targetingCoverageBooking{}
	previous := freeStateTargetingCoverageBookObservation
	freeStateTargetingCoverageBookObservation = func(s *Server, loop freeStateReasoningLoop, req freeStateTargetingCoverageRequest) map[string]any {
		*bookings = append(*bookings, targetingCoverageBooking{TrackID: req.TrackID, ViewIDs: append([]string(nil), req.ViewIDs...)})
		if failAll {
			return nil
		}
		observationID := "obs-cov-" + req.TrackID
		views := map[string]any{}
		for _, viewID := range req.ViewIDs {
			views[viewID] = map[string]any{"status": "ready"}
		}
		return map[string]any{
			"status":          "ready",
			"observation_id":  observationID,
			"requested_views": append([]string(nil), req.ViewIDs...),
			"views":           views,
			"freshness":       map[string]any{"status": "ready", "project_revision": "rev-1"},
			"evidence_refs":   []string{observationID},
		}
	}
	t.Cleanup(func() { freeStateTargetingCoverageBookObservation = previous })
	return bookings
}

func TestAudioClosureRoundBoundaryClosesPerTrackTargetingCoverageBeforeNoCandidate(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 3)
	bookings := installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	loop.ObservationLedger = targetingCoverageNaiveLedger("synthetic-track-a")
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-targeting-coverage"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("first round admission: admitted=%v err=%v", admitted, err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() {
		t.Fatalf("first round must not terminate: %+v", state)
	}

	// The boundary: RoundsStarted(1) >= MaxClosureRounds(1). Before the honest
	// no_candidate settle may run, the loop must close the per-track targeting
	// coverage gap by issuing observation requests for every active track the
	// open dimensions never observed — including the naive target itself,
	// whose own coverage is incomplete — and extending the closure window.
	// level_headroom was legitimately closed by the first round's recorded
	// evidence, but DIAG3-2 keeps a closed dimension in the duty while its
	// per-track primary coverage is incomplete — no track holds
	// track.basic_energy — so its primary view is requested for every track.
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("closure boundary settled with an open per-track coverage gap: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	if state.RoundsStarted < 2 || state.Policy.MaxClosureRounds < 2 {
		t.Fatalf("coverage closure did not extend the closure window: rounds=%d max=%d", state.RoundsStarted, state.Policy.MaxClosureRounds)
	}
	bookedTracks := map[string][]string{}
	for _, booking := range *bookings {
		bookedTracks[booking.TrackID] = booking.ViewIDs
	}
	if len(bookedTracks) != 3 {
		t.Fatalf("coverage pass booked %d track request(s), want 3 (every active track is under-covered): %v", len(bookedTracks), bookedTracks)
	}
	openPrimaries := []string{"track.basic_energy", "track.stereo_space", "track.timbre_frequency", "track.time_dynamics", "track.transient_structure"}
	for trackID, viewIDs := range bookedTracks {
		sort.Strings(viewIDs)
		want := openPrimaries
		if trackID == "synthetic-track-a" {
			// The naive target already holds dynamics + transient evidence.
			want = []string{"track.basic_energy", "track.stereo_space", "track.timbre_frequency"}
		}
		if strings.Join(viewIDs, ",") != strings.Join(want, ",") {
			t.Fatalf("coverage request for %s requested %v, want %v", trackID, viewIDs, want)
		}
	}
	stored, ok := server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("free-state loop vanished at the boundary")
	}
	available := firstMapFromAny(stored.ObservationLedger["available_views"])
	for _, uncovered := range []string{"synthetic-track-b", "synthetic-track-c"} {
		if _, ok := available["track:"+uncovered+"::track.time_dynamics"]; !ok {
			t.Fatalf("coverage closure did not book per-track observation for %s", uncovered)
		}
	}
	if !stored.TargetingCoveragePassDone {
		t.Fatal("coverage pass marker missing after the boundary pass")
	}
	if !(stored.ContinuationBudget > stored.ContinuationUsed) {
		t.Fatalf("coverage pass did not reserve a continuation slice: used=%d budget=%d", stored.ContinuationUsed, stored.ContinuationBudget)
	}
}

// TestAudioClosureDeferredBoundaryKeepsAdmittingRounds pins the anti-stall
// rule: when the boundary settle defers because the open queue still owns
// continuation budget (the coverage-closure reserve), the closure must grant
// a verification round and admit it instead of returning admitted=false and
// ending the scheduler chain with the loop mid-observation.
func TestAudioClosureDeferredBoundaryKeepsAdmittingRounds(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	// Budget NOT exhausted at the boundary: the settle defers.
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 2
	ledger := targetingCoverageNaiveLedger("synthetic-track-a")
	available := firstMapFromAny(ledger["available_views"])
	for _, trackID := range []string{"synthetic-track-a", "synthetic-track-b"} {
		for _, viewID := range []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"} {
			available["track:"+trackID+"::"+viewID] = map[string]any{
				"view_id": viewID, "status": "ready", "observation_id": "obs-" + trackID + "-" + viewID,
				"target_ref": map[string]any{"kind": "track", "id": trackID},
			}
		}
	}
	loop.ObservationLedger = ledger
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-targeting-deferred"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// Boundary with coverage closed and budget remaining: a round must be
	// granted and admitted, never a silent admitted=false.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("deferred boundary stalled the loop: admitted=%v terminal=%v state=%+v", admitted, state.Terminal(), state)
	}
	if state.RoundInProgress != true || state.RoundsStarted != 2 || state.Policy.MaxClosureRounds < 2 {
		t.Fatalf("deferred boundary did not admit a verification round: rounds=%d max=%d inProgress=%v", state.RoundsStarted, state.Policy.MaxClosureRounds, state.RoundInProgress)
	}
	// The window extension is bounded: once the continuation budget is spent,
	// the same boundary settles the honest no_candidate_found.
	stored, _ := server.freeStateLoop(conversationID)
	stored.ContinuationUsed = stored.ContinuationBudget
	server.storeFreeStateLoop(stored)
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if admitted || !state.Terminal() || state.Settlement == nil || state.Settlement.Reason != audioclosure.StopNoCandidateFound {
		t.Fatalf("exhausted boundary after deferral did not settle honestly: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
}

// TestAudioClosureCoverageClosedBoundaryStillSettlesNoCandidate pins the
// honest outcome: once per-track coverage is closed, the exhausted-budget
// boundary must still settle no_candidate_found — the fix widens observation,
// it never fabricates a selection.
func TestAudioClosureCoverageClosedBoundaryStillSettlesNoCandidate(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	ledger := targetingCoverageNaiveLedger("synthetic-track-a")
	// Close every open dimension's primary per-track coverage for both tracks
	// so the boundary has no gap left to close.
	available := firstMapFromAny(ledger["available_views"])
	for _, trackID := range []string{"synthetic-track-a", "synthetic-track-b"} {
		for _, viewID := range []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"} {
			available["track:"+trackID+"::"+viewID] = map[string]any{
				"view_id": viewID, "status": "partial", "observation_id": "obs-" + trackID + "-" + viewID,
				"target_ref": map[string]any{"kind": "track", "id": trackID},
			}
		}
	}
	loop.ObservationLedger = ledger
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-targeting-coverage-closed"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	// BOUNDARY-1 总则 1: the coverage-closed exhausted boundary first grants
	// the reserved terminal round (the slice is terminal-only) instead of
	// settling outright.
	if !admitted || state.Terminal() {
		t.Fatalf("coverage-closed boundary settled before the terminal turn: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	// The terminal chain burns the reservation without a decision: the next
	// boundary settles the honest no_candidate_found.
	storedLoop, _ := server.freeStateLoop(conversationID)
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if admitted || !state.Terminal() || state.Settlement == nil || state.Settlement.Reason != audioclosure.StopNoCandidateFound {
		t.Fatalf("coverage-closed boundary did not settle the honest no_candidate_found: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
}

// TestAudioClosureCoveragePassRunsOnceThenSettlesHonestly pins the bound: a
// pass that cannot produce evidence (kernel rejects, partial ledger, cap
// reached) must not loop — the next boundary settles no_candidate_found.
func TestAudioClosureCoveragePassRunsOnceThenSettlesHonestly(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, true)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	loop.ObservationLedger = targetingCoverageNaiveLedger("synthetic-track-a")
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-targeting-coverage-once"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// First boundary: the pass runs, books nothing usable, marks itself done
	// — and BOUNDARY-1 grants the reserved terminal round before any settle.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("failed coverage pass settled before the terminal turn: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	storedLoop, _ := server.freeStateLoop(conversationID)
	if !storedLoop.TargetingCoveragePassDone {
		t.Fatal("coverage pass marker not set after a failed pass")
	}
	// The terminal chain burns the reservation without a decision: the next
	// boundary settles no_candidate_found (the fix widens observation, it
	// never fabricates a selection, and it never loops).
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if admitted || !state.Terminal() || state.Settlement == nil || state.Settlement.Reason != audioclosure.StopNoCandidateFound {
		t.Fatalf("failed coverage pass did not degrade to the honest settle: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
}

// TestAudioClosureBoundaryCoversClosedDimensionMissingTracks is the DIAG3-2
// p03 starvation shape, reproduced through the production closure path: round
// one observes only the dynamics primary on one salient track, so the round
// record closes the dynamics dimension (QueueFromRounds) while the other
// track has zero dynamics evidence; round two covers every remaining
// dimension on both tracks. At the exhausted boundary the queue is mostly
// closed, dynamics is closed with a per-track gap, and the gate must book the
// missing track's dynamics view — without re-requesting any fully covered
// dimension.
func TestAudioClosureBoundaryCoversClosedDimensionMissingTracks(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	bookings := installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	// Round one: the model spent its request on the salient track's dynamics
	// primary only — the evidence that closes the dynamics dimension.
	loop.ObservationLedger = targetingCoverageLedgerWithViews(map[string][]string{
		"synthetic-track-a": {"track.time_dynamics"},
	})
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-targeting-closed-dim"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 2)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() {
		t.Fatalf("record settled the closure before the boundary: %+v", state.Settlement)
	}
	stored, ok := server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("free-state loop vanished after round one")
	}
	if stored.PriorityQueue == nil || !stored.PriorityQueue.HasOpen() {
		t.Fatal("round one must leave the diagnostic queue open")
	}
	for _, entry := range stored.PriorityQueue.Entries {
		if entry.Dimension == audioclosure.DimensionDynamics && entry.Status != audioclosure.QueueClosed {
			t.Fatalf("round one's single dynamics observation did not close the dynamics dimension: %+v", stored.PriorityQueue.Entries)
		}
	}
	// Round two: the model covers every remaining dimension on both tracks.
	// The dynamics dimension stays closed with a per-track gap — synthetic
	// track-b has zero dynamics evidence.
	loop = stored
	available := firstMapFromAny(loop.ObservationLedger["available_views"])
	for _, trackID := range []string{"synthetic-track-a", "synthetic-track-b"} {
		for _, viewID := range []string{"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure"} {
			available["track:"+trackID+"::"+viewID] = map[string]any{
				"view_id": viewID, "status": "ready", "observation_id": "obs-" + trackID + "-" + viewID,
				"target_ref": map[string]any{"kind": "track", "id": trackID},
			}
		}
	}
	server.storeFreeStateLoop(loop)
	state, _, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() {
		t.Fatalf("record settled the closure before the boundary: %+v", state.Settlement)
	}
	// The exhausted boundary: the closed-but-under-covered dynamics dimension
	// must join the coverage pass, booking track.time_dynamics for the track
	// that never held dynamics evidence, and extending the closure window.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("boundary settled despite the closed dimension's per-track gap: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	if state.RoundsStarted < 3 || state.Policy.MaxClosureRounds < 3 {
		t.Fatalf("coverage closure did not extend the closure window: rounds=%d max=%d", state.RoundsStarted, state.Policy.MaxClosureRounds)
	}
	bookedTracks := map[string][]string{}
	for _, booking := range *bookings {
		bookedTracks[booking.TrackID] = booking.ViewIDs
	}
	if len(bookedTracks) != 1 {
		t.Fatalf("coverage pass booked %d track request(s), want 1 (only the dynamics-missing track): %v", len(bookedTracks), bookedTracks)
	}
	if strings.Join(bookedTracks["synthetic-track-b"], ",") != "track.time_dynamics" {
		t.Fatalf("coverage request for the dynamics-missing track requested %v, want [track.time_dynamics]", bookedTracks["synthetic-track-b"])
	}
	if _, rebooked := bookedTracks["synthetic-track-a"]; rebooked {
		t.Fatalf("fully covered track synthetic-track-a was re-booked: %v", bookedTracks["synthetic-track-a"])
	}
	stored, ok = server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("free-state loop vanished at the boundary")
	}
	if _, ok := firstMapFromAny(stored.ObservationLedger["available_views"])["track:synthetic-track-b::track.time_dynamics"]; !ok {
		t.Fatal("coverage closure did not book the dynamics view for synthetic-track-b")
	}
	if !stored.TargetingCoveragePassDone {
		t.Fatal("coverage pass marker missing after the boundary pass")
	}
}

// TestFreeStateTargetingCoverageRequestsTable is the pure coverage-matrix
// check: open dimensions × active tracks against the ledger projection.
func TestFreeStateTargetingCoverageRequestsTable(t *testing.T) {
	fullQueue := audioclosure.DefaultPriorityQueue()
	dynamicsOnly := audioclosure.DefaultPriorityQueue()
	for i := range dynamicsOnly.Entries {
		dynamicsOnly.Entries[i].Status = audioclosure.QueueClosed
	}
	dynamicsOnly.Entries[2].Status = audioclosure.QueueOpen // dynamics → track.time_dynamics
	// DIAG3-2 fixtures: dynamics leaves the open set while every other
	// dimension stays open, isolating the closed-dimension eligibility rule.
	dynamicsClosedRestOpen := audioclosure.DefaultPriorityQueue()
	dynamicsClosedRestOpen.Entries[2].Status = audioclosure.QueueClosed
	dynamicsSkippedRestOpen := audioclosure.DefaultPriorityQueue()
	dynamicsSkippedRestOpen.Entries[2].Status = audioclosure.QueueSkipped
	dynamicsSkippedRestOpen.Entries[2].SkipReason = audioclosure.SkipNotApplicable
	closedQueue := audioclosure.DefaultPriorityQueue()
	for i := range closedQueue.Entries {
		closedQueue.Entries[i].Status = audioclosure.QueueClosed
	}
	tracks := []harness.ActiveAudioTrack{
		{ID: "t-b", Name: "B"}, {ID: "t-a", Name: "A"},
	}
	allPrimaries := []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"}

	tests := []struct {
		name    string
		queue   *audioclosure.PriorityQueue
		ledger  map[string]any
		tracks  []harness.ActiveAudioTrack
		wantIDs []string
		wantSet map[string][]string
	}{
		{
			// DIAG3-2: a closed dimension whose per-track primary coverage is
			// incomplete stays in the coverage duty — the p03 starvation shape
			// (dynamics closed early, most tracks never observed with it).
			name:    "all-closed queue with zero per-track coverage requests every track",
			queue:   &closedQueue,
			ledger:  map[string]any{},
			tracks:  tracks,
			wantIDs: []string{"t-a", "t-b"},
			wantSet: map[string][]string{"t-a": allPrimaries, "t-b": allPrimaries},
		},
		{
			// The duty is bounded by the ledger's actual gaps: a closed
			// dimension fully covered on every active track never re-triggers.
			name:   "all-closed queue fully covered stays silent",
			queue:  &closedQueue,
			ledger: targetingCoverageLedgerWithViews(map[string][]string{"t-a": allPrimaries, "t-b": allPrimaries}),
			tracks: tracks,
		},
		{
			name:   "no active tracks → no requests",
			queue:  &fullQueue,
			ledger: map[string]any{},
			tracks: nil,
		},
		{
			name:    "empty ledger → every track uncovered for every open dimension",
			queue:   &fullQueue,
			ledger:  map[string]any{},
			tracks:  tracks,
			wantIDs: []string{"t-a", "t-b"},
			wantSet: map[string][]string{"t-a": allPrimaries, "t-b": allPrimaries},
		},
		{
			name:    "mix-level observations never close per-track coverage",
			queue:   &fullQueue,
			ledger:  map[string]any{"available_views": map[string]any{"mix.masking_relationship": map[string]any{"view_id": "mix.masking_relationship", "status": "ready"}}},
			tracks:  tracks,
			wantIDs: []string{"t-a", "t-b"},
			wantSet: map[string][]string{"t-a": allPrimaries, "t-b": allPrimaries},
		},
		{
			// DIAG3-2: the four closed dimensions are fully covered on both
			// tracks, so they stay out of the duty and only the open dynamics
			// dimension's gap produces a request.
			name:  "already-covered track is skipped, uncovered track requested",
			queue: &dynamicsOnly,
			ledger: targetingCoverageLedgerWithViews(map[string][]string{
				"t-a": {"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure", "track.time_dynamics"},
				"t-b": {"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure"},
			}),
			tracks:  tracks,
			wantIDs: []string{"t-b"},
			wantSet: map[string][]string{"t-b": {"track.time_dynamics"}},
		},
		{
			// Partial ledger rows still count as coverage — including for the
			// closed dimensions, whose covered rows keep them out of the duty.
			name:  "partial ledger rows still count as coverage",
			queue: &dynamicsOnly,
			ledger: targetingCoverageLedgerWithViews(map[string][]string{
				"t-a": allPrimaries,
				"t-b": allPrimaries,
			}),
			tracks: tracks,
		},
		{
			// DIAG3-2 isolation: with every open dimension fully covered, a
			// closed dimension with a per-track gap is requested for the
			// missing track only — the covered track is not re-booked.
			name:  "closed dynamics with one covered track requests only the missing track",
			queue: &dynamicsClosedRestOpen,
			ledger: targetingCoverageLedgerWithViews(map[string][]string{
				"t-a": allPrimaries,
				"t-b": {"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure"},
			}),
			tracks:  tracks,
			wantIDs: []string{"t-b"},
			wantSet: map[string][]string{"t-b": {"track.time_dynamics"}},
		},
		{
			// A skipped dimension left the diagnostic spine by policy; the
			// coverage duty neither re-opens nor expands into it.
			name:  "skipped dimension never enters the coverage duty",
			queue: &dynamicsSkippedRestOpen,
			ledger: targetingCoverageLedgerWithViews(map[string][]string{
				"t-a": {"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure"},
				"t-b": {"track.basic_energy", "track.timbre_frequency", "track.stereo_space", "track.transient_structure"},
			}),
			tracks: tracks,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := freeStateTargetingCoverageRequests(tt.queue, tt.ledger, tt.tracks)
			gotIDs := make([]string, 0, len(got))
			for _, request := range got {
				gotIDs = append(gotIDs, request.TrackID)
			}
			if strings.Join(gotIDs, ",") != strings.Join(tt.wantIDs, ",") {
				t.Fatalf("request tracks = %v, want %v", gotIDs, tt.wantIDs)
			}
			for trackID, wantViews := range tt.wantSet {
				var gotViews []string
				for _, request := range got {
					if request.TrackID == trackID {
						gotViews = request.ViewIDs
					}
				}
				if strings.Join(gotViews, ",") != strings.Join(wantViews, ",") {
					t.Fatalf("track %s view set = %v, want %v", trackID, gotViews, wantViews)
				}
			}
		})
	}
}

// targetingCoverageDutyLedger builds a ledger whose rows were booked by the
// loop's deterministic duty pass: the durable tool_call_id marker is the only
// provenance signal that survives the ledger round-trip.
func targetingCoverageDutyLedger(trackIDs []string) map[string]any {
	available := map[string]any{}
	for _, trackID := range trackIDs {
		available["track:"+trackID+"::track.basic_energy"] = map[string]any{
			"view_id":        "track.basic_energy",
			"status":         "ready",
			"observation_id": "obs-duty-" + trackID,
			"tool_call_id":   "targeting_coverage:" + trackID,
			"target_ref":     map[string]any{"kind": "track", "id": trackID},
		}
	}
	return map[string]any{
		"schema_version":  "free_state_observation_ledger.v1",
		"available_views": available,
	}
}

// TestAudioClosureDutyObservationsDoNotTripModelEvidenceCeiling is the p01
// run 201217 shape at the production boundary: the model has filled its
// observation budget, the coverage pass books two more observations as duty,
// and the exhausted-budget boundary must not settle while only duty evidence
// is arriving — while the next genuinely model-requested observation still
// routes the closure to the honest boundary settle.
func TestAudioClosureDutyObservationsDoNotTripModelEvidenceCeiling(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	loop.ObservationLedger = targetingCoverageDutyLedger([]string{"synthetic-track-a", "synthetic-track-b"})
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-duty-ceiling"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 6)
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil || !admitted {
		t.Fatalf("first round admission: admitted=%v err=%v", admitted, err)
	}
	driver := audioclosure.Driver{}
	for i := 0; i < state.Policy.MaxUniqueObservations; i++ {
		key := audioclosure.ObservationKey{ProjectUUID: "project-targeting-coverage", ProjectRevision: "rev-1",
			Scope: audioclosure.Scope{Kind: "project", ID: "project-targeting-coverage"}, TargetRef: "project-targeting-coverage",
			ViewIDs: []string{fmt.Sprintf("track.model-view-%d", i)}}
		outcome, err := driver.RecordObservation(state, state.Revision, key, fmt.Sprintf("obs-model-%d", i), time.Now().UTC())
		if err != nil || !outcome.Accepted {
			t.Fatalf("model observation %d: accepted=%v err=%v", i, outcome.Accepted, err)
		}
		state = outcome.State
	}
	stored, _ := server.audioClosures.Load(state.ClosureID)
	if err := server.audioClosures.Save(state, stored.Revision); err != nil {
		t.Fatal(err)
	}

	// The duty-booked ledger rows settle on the duty side of the dual budget:
	// the closure must record them and stay live even though the model-side
	// budget is exhausted.
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if state.Terminal() {
		t.Fatalf("duty observations must not trip the model evidence ceiling: %+v", state.Settlement)
	}
	if state.ModelObservationCount() != state.Policy.MaxUniqueObservations {
		t.Fatalf("duty bookings must not consume the model side: model=%d", state.ModelObservationCount())
	}
	if len(state.Observations) != state.Policy.MaxUniqueObservations+2 {
		t.Fatalf("duty observations were not recorded: total=%d", len(state.Observations))
	}

	// The next genuinely model-requested observation still routes the
	// exhausted-budget closure to the honest boundary settle.
	modelResult := agentloop.Result{
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "command_name": "ccb_observation_request",
			"status": "ok", "tool_call_id": "tool_model_ceiling",
			"result": map[string]any{"bundle": map[string]any{
				"schema_version":  "ccb_observation_bundle.v1",
				"status":          "ready",
				"observation_id":  "obs-model-ceiling",
				"requested_views": []any{"track.model-ceiling-view"},
				"views":           map[string]any{"track.model-ceiling-view": map[string]any{"status": "ready"}},
				"freshness":       map[string]any{"status": "ready", "project_revision": "rev-1"},
				"evidence_refs":   []any{"obs-model-ceiling"},
			}},
		}},
	}
	state, err = server.recordAudioClosureRound(state, modelResult, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// BOUNDARY-1 总则 1: the evidence ceiling first grants the reserved
	// terminal round; the ceiling observation is still not recorded.
	if state.Terminal() {
		t.Fatalf("model observation tripped the ceiling before the terminal turn: %+v", state.Settlement)
	}
	if len(state.Observations) != state.Policy.MaxUniqueObservations+2 {
		t.Fatalf("the ceiling model observation must not be recorded: total=%d", len(state.Observations))
	}
	// The terminal chain burns the reservation: the next genuinely
	// model-requested observation reaches the honest boundary settle.
	storedLoop, _ := server.freeStateLoop(conversationID)
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	state, err = server.recordAudioClosureRound(state, modelResult, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !state.Terminal() || state.Settlement == nil || state.Settlement.Reason != audioclosure.StopNoCandidateFound {
		t.Fatalf("model observation at the exhausted model budget must reach the honest boundary: terminal=%v settlement=%+v", state.Terminal(), state.Settlement)
	}
	if len(state.Observations) != state.Policy.MaxUniqueObservations+2 {
		t.Fatalf("the ceiling model observation must not be recorded: total=%d", len(state.Observations))
	}
}

// installFrontierFeedProbeStub swaps the deterministic frontier-feed probe
// booking for a recordable fake. failAll=false returns a minimal ready mix
// bundle carrying one band-conflict candidate row per requested family.
func installFrontierFeedProbeStub(t *testing.T, failAll bool) *[]string {
	t.Helper()
	calls := &[]string{}
	previous := freeStateMixCandidateProbeBookObservation
	freeStateMixCandidateProbeBookObservation = func(s *Server, loop freeStateReasoningLoop, viewID string) map[string]any {
		*calls = append(*calls, viewID)
		if failAll {
			return nil
		}
		observationID := "obs-frontier-feed-" + viewID
		// The candidate rows sit where the live CCB bundle carries them —
		// the flat "observation.mom_projection" facts key — the only shape
		// the conclusion digest and the durable-ledger round-trip both
		// preserve.
		candidateRows := []any{map[string]any{
			"band": "low", "type": "overlap", "status": "candidate",
			"tracks": []any{map[string]any{"track_id": "synthetic-track-a"}},
		}}
		relation := map[string]any{"status": "ready"}
		if viewID == "mix.multitrack_relationship" {
			relation["band_conflict_candidates"] = candidateRows
		} else {
			relation["conflict_candidates"] = candidateRows
		}
		facts := map[string]any{"observation.mom_projection": map[string]any{
			strings.TrimPrefix(viewID, "mix."): relation,
		}}
		views := map[string]any{viewID: map[string]any{"status": "ready", "facts": facts}}
		return map[string]any{
			"status":          "ready",
			"observation_id":  observationID,
			"requested_views": []string{viewID},
			"views":           views,
			"freshness":       map[string]any{"status": "ready", "project_revision": "rev-1"},
			"evidence_refs":   []string{observationID},
		}
	}
	t.Cleanup(func() { freeStateMixCandidateProbeBookObservation = previous })
	return calls
}

// TestAudioClosureBoundaryProbesExtractorDeclaredFrontierFamiliesWhenEmpty
// pins the bounded frontier feed: at the saturation boundary (per-track
// coverage complete, queue open, frontier empty) the coverage pass probes the
// candidate extractor's statically declared view families once each, books
// them as duty observations, and extends the window instead of settling — so
// the frontier gate stops being purely dependent on the model spontaneously
// requesting mix-level views.
func TestAudioClosureBoundaryProbesExtractorDeclaredFrontierFamiliesWhenEmpty(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, false)
	probeCalls := installFrontierFeedProbeStub(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	allPrimaries := []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"}
	loop.ObservationLedger = targetingCoverageLedgerWithViews(map[string][]string{
		"synthetic-track-a": allPrimaries,
		"synthetic-track-b": allPrimaries,
	})
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-frontier-feed"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("frontier-feed boundary settled although the probe booked extractor-declared families: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	if len(*probeCalls) != len(freeStateMixCandidateProbeViewIDs) {
		t.Fatalf("frontier-feed probe must fire once per extractor-declared family, got %v", *probeCalls)
	}
	stored, ok := server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("free-state loop vanished at the frontier-feed boundary")
	}
	available := firstMapFromAny(stored.ObservationLedger["available_views"])
	for _, viewID := range freeStateMixCandidateProbeViewIDs {
		row := firstMapFromAny(available[viewID])
		if firstStringFromMap(row, "view_id") != viewID {
			t.Fatalf("frontier-feed probe did not book %s into the durable ledger: %v", viewID, available)
		}
		if toolCallID := firstStringFromMap(row, "tool_call_id"); !strings.HasPrefix(toolCallID, "targeting_coverage:") {
			t.Fatalf("frontier-feed booking %s lost its duty marker: %q", viewID, toolCallID)
		}
	}
	if !stored.TargetingCoveragePassDone {
		t.Fatal("coverage pass marker missing after the frontier-feed pass")
	}
	if len(stored.ObservationSaturationNotice) == 0 {
		t.Fatal("saturation notice missing alongside the frontier-feed booking")
	}

	// The next decision round rehydrates the duty rows: the candidate
	// extractor consumes them and the frontier establishes — and the duty
	// rows land on the duty side of the closure budget, not the model side.
	decisionRound := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
	}}
	state, err = server.recordAudioClosureRound(state, decisionRound, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Frontier.Candidates) == 0 {
		t.Fatalf("frontier did not establish from the extractor-declared probe rows: %+v", state.Frontier)
	}
	if state.ModelObservationCount()+2 != len(state.Observations) {
		t.Fatalf("frontier-feed rows must be duty-accounted: model=%d total=%d", state.ModelObservationCount(), len(state.Observations))
	}
}

// TestAudioClosureFrontierFeedProbeWithoutCandidatesStillSettlesHonestly
// pins the honest outcome: when the probe cannot produce usable evidence,
// the exhausted-budget boundary still settles no_candidate_found unchanged.
func TestAudioClosureFrontierFeedProbeWithoutCandidatesStillSettlesHonestly(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, false)
	probeCalls := installFrontierFeedProbeStub(t, true)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 1, 1
	allPrimaries := []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"}
	loop.ObservationLedger = targetingCoverageLedgerWithViews(map[string][]string{
		"synthetic-track-a": allPrimaries,
		"synthetic-track-b": allPrimaries,
	})
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-frontier-feed-empty"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	// BOUNDARY-1 总则 1: the candidate-less exhausted boundary first grants
	// the reserved terminal round instead of settling outright.
	if !admitted || state.Terminal() {
		t.Fatalf("candidate-less frontier feed settled before the terminal turn: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	// The terminal chain burns the reservation: the next boundary settles the
	// honest no_candidate_found unchanged.
	storedLoop, _ := server.freeStateLoop(conversationID)
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if admitted || !state.Terminal() || state.Settlement == nil || state.Settlement.Reason != audioclosure.StopNoCandidateFound {
		t.Fatalf("a candidate-less frontier feed did not settle the honest no_candidate_found: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	if len(*probeCalls) != len(freeStateMixCandidateProbeViewIDs) {
		t.Fatalf("the probe must still be attempted once per family, got %v", *probeCalls)
	}
}

// DIAG3-3: the free-state loop must turn "per-track observation coverage is
// complete while the diagnostic queue is still open and the hypothesis
// frontier is empty" into a mechanical runtime notice on the model face. The
// notice carries only structural state facts (dimension list, coverage
// status, budget, frontier size) — never a track identity, domain name, or
// domain verb. This is the model B milestone propulsion obligation: the model
// cannot distinguish "observing further helps" from "observation is
// saturated" without it (run 20260906_181304: full 6×5 coverage booked, then
// every remaining model turn stayed needs_observation in fs4 until the
// budget settled no_candidate_found with zero proposal attempts).
func TestAudioClosureBoundaryNotifiesObservationSaturationWhenCoverageCompleteQueueOpen(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 3)
	installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 2
	allPrimaries := []string{"track.basic_energy", "track.timbre_frequency", "track.time_dynamics", "track.stereo_space", "track.transient_structure"}
	loop.ObservationLedger = targetingCoverageLedgerWithViews(map[string][]string{
		"synthetic-track-a": allPrimaries,
		"synthetic-track-b": allPrimaries,
		"synthetic-track-c": allPrimaries,
	})
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-saturation-notice"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	// Boundary: coverage is already complete, so the coverage pass has nothing
	// to book and settles nothing (budget remains). The loop must carry the
	// mechanical saturation notice onto the model face for the next decision
	// rounds instead of letting the model keep requesting covered views.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("saturation boundary settled despite remaining continuation budget: admitted=%v terminal=%v", admitted, state.Terminal())
	}
	stored, ok := server.freeStateLoop(conversationID)
	if !ok {
		t.Fatal("free-state loop vanished at the saturation boundary")
	}
	if len(stored.ObservationSaturationNotice) == 0 {
		t.Fatal("coverage-complete + queue-open + frontier-empty boundary injected no observation saturation notice")
	}
	if !stored.TargetingCoveragePassDone {
		t.Fatal("coverage pass marker missing after the saturation boundary")
	}
	// Mechanical whitelist: only dimension names, coverage status, frontier
	// size, and budget facts. Any track identity or processor domain name in
	// the notice is a sealed-discipline failure.
	for key, value := range stored.ObservationSaturationNotice {
		text := strings.ToLower(fmt.Sprint(value))
		for _, banned := range []string{"synthetic-track", "broadband_compression", "static_eq", "track_gain", "compressor"} {
			if key != "open_dimensions" && text != "" && strings.Contains(text, banned) {
				t.Fatalf("saturation notice field %q carries non-mechanical content %q", key, text)
			}
		}
	}
	if open, ok := stored.ObservationSaturationNotice["open_dimensions"]; !ok || len(freeStateStringSlice(open)) == 0 {
		t.Fatalf("saturation notice must disclose the open dimension list, got %v", stored.ObservationSaturationNotice["open_dimensions"])
	}
	if budget, ok := stored.ObservationSaturationNotice["continuation_budget"].(float64); !ok || budget <= 0 {
		t.Fatalf("saturation notice must disclose the continuation budget, got %v", stored.ObservationSaturationNotice["continuation_budget"])
	}
}

// DIAG3-3 honesty latch: the saturation notice surfaces for at most two
// decision rounds without a proposal, then retires — the exhausted-budget
// honest settle (no_candidate_found) stays the unchanged legal outcome. A
// needs_experiment decision retires it immediately.
func TestObservationSaturationNoticeRetiresAfterTwoDecisionRounds(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 1)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ObservationSaturationNotice = map[string]any{
		"schema_version":      freeStateObservationSaturationNoticeSchema,
		"coverage_status":     "per_track_primary_complete",
		"open_dimensions":     []string{"dynamics"},
		"frontier_candidates": 0,
		"continuation_used":   2,
		"continuation_budget": 6,
	}
	server.storeFreeStateLoop(loop)

	needsObservation := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
	}}
	stored, handled := server.recordFreeStateDecision(conversationID, needsObservation)
	if !handled {
		t.Fatal("decision recording refused the observation round")
	}
	if len(stored.ObservationSaturationNotice) == 0 || stored.ObservationSaturationRounds != 1 {
		t.Fatalf("first decision round must surface the notice once: rounds=%d notice=%v", stored.ObservationSaturationRounds, stored.ObservationSaturationNotice)
	}
	stored, handled = server.recordFreeStateDecision(conversationID, needsObservation)
	if !handled {
		t.Fatal("decision recording refused the second observation round")
	}
	if len(stored.ObservationSaturationNotice) != 0 || stored.ObservationSaturationRounds != 2 {
		t.Fatalf("second proposal-less decision round must retire the notice: rounds=%d notice=%v", stored.ObservationSaturationRounds, stored.ObservationSaturationNotice)
	}

	// A proposal retires the notice immediately regardless of the counter.
	loop.ObservationSaturationNotice = map[string]any{"schema_version": freeStateObservationSaturationNoticeSchema}
	loop.ObservationSaturationRounds = 0
	server.storeFreeStateLoop(loop)
	proposal := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsExperiment,
	}}
	stored, handled = server.recordFreeStateDecision(conversationID, proposal)
	if !handled {
		t.Fatal("decision recording refused the proposal round")
	}
	if len(stored.ObservationSaturationNotice) != 0 {
		t.Fatalf("needs_experiment decision must retire the notice, got %v", stored.ObservationSaturationNotice)
	}
}

// DIAG3-3 frontier diversion: when the hypothesis frontier became
// established at an exhausted closure boundary, the boundary must grant one
// decision round (once-per-loop) instead of settling capability_blocked with
// the candidates never shown to the model. A spent grant falls through to
// the ordinary honest settle.
func TestAudioClosureBoundaryGrantsFrontierDecisionRoundBeforeSettle(t *testing.T) {
	server, conversationID := targetingCoverageTestServer(t, 2)
	installTargetingCoverageBooker(t, false)
	loop := continuationTestLoop(conversationID)
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.ContinuationBudget, loop.ContinuationUsed = 11, 7
	loop.TargetingCoveragePassDone = true
	server.storeFreeStateLoop(loop)
	server.capabilityRoutes["route-frontier-grant"] = CapabilityRouteRecord{SchemaVersion: "capability_route.v1",
		ConversationID: conversationID,
		Assessment:     &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}, UpdatedAt: time.Now().UTC()}

	state := targetingCoverageClosureState(t, server, conversationID, 1)
	state, _, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	// The mix-level observation landed on the final closure round: the round
	// recording extracts its band-conflict candidate rows into the frontier,
	// exactly as the production mix.multitrack_relationship bundle does.
	mixFrontierResult := agentloop.Result{
		Executed: []map[string]any{{
		"tool": "ccb.observation_request", "command_name": "ccb_observation_request",
		"status": "ok", "tool_call_id": "tool_mix_frontier",
		"result": map[string]any{"bundle": map[string]any{
			"schema_version":  "ccb_observation_bundle.v1",
			"status":          "ready",
			"observation_id":  "obs-mix-frontier",
			"requested_views": []any{"mix.multitrack_relationship"},
			"views": map[string]any{"mix.multitrack_relationship": map[string]any{"facts": map[string]any{
				"band_conflict_candidates": []any{map[string]any{
					"band": "bass", "type": "low_end_overlap", "status": "candidate",
					"tracks": []any{map[string]any{"track_id": "synthetic-track-a"}},
				}},
			}}},
			"evidence_refs": []any{"obs-mix-frontier"},
		}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
		},
	}
	state, err = server.recordAudioClosureRound(state, mixFrontierResult, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	current, loaded := server.audioClosures.Load(state.ClosureID)
	if !loaded {
		t.Fatal("closure state vanished before the frontier grant")
	}
	if len(current.Frontier.Candidates) == 0 {
		t.Fatalf("fixture failed to establish the frontier from the mix bundle: %+v", current.Frontier)
	}
	state = current

	// First boundary with an established frontier: grant one decision round.
	state, admitted, err := server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !admitted || state.Terminal() {
		t.Fatalf("established frontier settled without a model decision round: admitted=%v terminal=%v settlement=%+v", admitted, state.Terminal(), state.Settlement)
	}
	if state.Policy.MaxClosureRounds < 2 {
		t.Fatalf("frontier grant did not extend the closure window: max=%d", state.Policy.MaxClosureRounds)
	}
	stored, ok := server.freeStateLoop(conversationID)
	if !ok || !stored.FrontierDecisionRoundGranted {
		t.Fatalf("frontier grant marker missing: ok=%v granted=%v", ok, stored.FrontierDecisionRoundGranted)
	}
	if !(stored.ContinuationBudget > stored.ContinuationUsed) {
		t.Fatalf("frontier grant did not reserve a continuation slice: used=%d budget=%d", stored.ContinuationUsed, stored.ContinuationBudget)
	}
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}

	// Second boundary with the grant spent: BOUNDARY-1 still owes the loop
	// its once-per-loop terminal turn, so the boundary grants that round
	// before any settle.
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if admitted == state.Terminal() && !state.Terminal() {
		t.Fatalf("spent grant neither settled nor admitted deterministically: admitted=%v terminal=%v", admitted, state.Terminal())
	}
	if !admitted || state.Terminal() {
		t.Fatalf("spent grant must grant the terminal round before the settle, got admitted=%v terminal=%v", admitted, state.Terminal())
	}
	// The terminal chain burns the reservation without a decision: the third
	// boundary runs the ordinary honest settle.
	storedLoop, _ := server.freeStateLoop(conversationID)
	storedLoop.ContinuationUsed = storedLoop.ContinuationBudget
	server.storeFreeStateLoop(storedLoop)
	state, err = server.recordAudioClosureRound(state, agentloop.Result{}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	state, admitted, err = server.admitAudioClosureRound(state)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Terminal() {
		t.Fatalf("terminal reservation spent but the honest settle did not run: admitted=%v terminal=%v", admitted, state.Terminal())
	}
}
