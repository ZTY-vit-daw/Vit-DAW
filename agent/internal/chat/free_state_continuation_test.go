package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	agentruntime "vit-daw-agent/internal/runtime"
)

// Matrix M10 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md): waiting_continue
// is an internally bounded state, the explicit continuation budget halts the
// loop when exhausted, and restart recovery restores
// phase/round/ledger/frontier plus the new loop persistence fields.

func continuationTestLoop(conversationID string) freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-m10",
		ConversationID: conversationID,
		Status:         "observing",
		OriginalIntent: "inspect the project",
		ActiveIntent:   "inspect the project",
		Cycle:          1,
		MaxCycles:      6,
		CurrentRoundID: "r_m10round00001",
		PriorityQueue:  &audioclosure.PriorityQueue{},
		ObservationLedger: map[string]any{
			"schema_version": "free_state_observation_ledger.v1",
			"receipts":       []any{map[string]any{"status": "ready", "observation_id": "obs-m10"}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestM10ContinuationBudgetDefaultsBoundedAndStops(t *testing.T) {
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-m10")
	loop.ContinuationBudget = 0
	s.storeFreeStateLoop(loop)
	stored, ok := s.freeStateLoop("conversation-m10")
	if !ok {
		t.Fatal("loop was not stored")
	}
	// The explicit budget defaults to the loop's action ceiling, so the
	// scheduler Attempt semantics and the loop budget cannot diverge.
	if stored.ContinuationBudget != stored.MaxCycles {
		t.Fatalf("continuation budget %d did not default to max cycles %d", stored.ContinuationBudget, stored.MaxCycles)
	}
	// Budget not exhausted while usage is below the bound.
	used := stored
	used.ContinuationUsed = stored.ContinuationBudget - 1
	if freeStateContinuationBudgetExhausted(used) {
		t.Fatal("budget reported exhausted before the bound")
	}
	exhausted := stored
	exhausted.ContinuationUsed = stored.ContinuationBudget
	if !freeStateContinuationBudgetExhausted(exhausted) {
		t.Fatal("budget reported available after the bound was reached")
	}
	// waiting_continue stays an internally bounded state: exhausting the
	// budget marks the loop blocked with a queryable stop reason.
	exhausted.Status = "observing"
	s.storeFreeStateLoop(exhausted)
	over := exhausted
	over.ContinuationUsed = exhausted.ContinuationBudget + 1
	if !freeStateContinuationBudgetExhausted(over) {
		t.Fatal("budget exhaustion is not monotonic")
	}
}

func TestD1AppliedActionReservesOnePostActionObservationSlice(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 6}
	loop.RequiresPostActionObservation = true
	reservePostActionObservationSlice(&loop)
	if loop.ContinuationBudget != 7 {
		t.Fatalf("post-action observation budget = %d, want 7", loop.ContinuationBudget)
	}
	reservePostActionObservationSlice(&loop)
	if loop.ContinuationBudget != 7 {
		t.Fatalf("post-action observation budget was extended twice: %d", loop.ContinuationBudget)
	}
}

func TestD1AppliedActionReservesObservationWhenNextSliceConsumesBudget(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 5, RequiresPostActionObservation: true}
	reservePostActionObservationSlice(&loop)
	if loop.ContinuationBudget != 7 {
		t.Fatalf("budget = %d, want 7", loop.ContinuationBudget)
	}
}

func TestInteractionResumeDoesNotDoubleCountContinuation(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 5, RequiresPostActionObservation: true}
	reservePostActionObservationSlice(&loop)
	if loop.ContinuationUsed != 5 || loop.ContinuationBudget != 7 {
		t.Fatalf("interaction resume accounting = used=%d budget=%d", loop.ContinuationUsed, loop.ContinuationBudget)
	}
}

func TestFreeStateLoopMergeDoesNotRegressDurableStatus(t *testing.T) {
	base := continuationTestLoop("conversation-merge-status")
	base.Status = "observing"
	base.UpdatedAt = time.Now().UTC()
	overlay := continuationTestLoop(base.ConversationID)
	overlay.Status = "reasoning"
	overlay.UpdatedAt = base.UpdatedAt.Add(-time.Second)
	merged := mergeFreeStateLoops(base, overlay, true)
	if merged.Status != "observing" {
		t.Fatalf("stale continuation snapshot regressed loop status: %q", merged.Status)
	}
}

func TestM10BudgetFieldsPersistAcrossRestart(t *testing.T) {
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-m10-restart")
	loop.PriorityQueue = func() *audioclosure.PriorityQueue {
		queue := audioclosure.DefaultPriorityQueue()
		return &queue
	}()
	loop.ContinuationBudget = 3
	loop.ContinuationUsed = 2
	s.storeFreeStateLoop(loop)
	// Simulate a restart: the persisted loop is recovered through the same
	// JSON round-trip the durable runtime state uses.
	recovered, ok := s.freeStateLoop("conversation-m10-restart")
	if !ok {
		t.Fatal("loop lost on recovery")
	}
	if recovered.CurrentRoundID != loop.CurrentRoundID ||
		recovered.ContinuationBudget != 3 || recovered.ContinuationUsed != 2 {
		t.Fatalf("persistence fields drifted after restart: %+v", recovered)
	}
	if recovered.PriorityQueue == nil || recovered.PriorityQueue.SchemaVersion != audioclosure.PriorityQueueSchema {
		t.Fatalf("priority queue lost after restart: %+v", recovered.PriorityQueue)
	}
	// A transport overlay carrying newer budget usage cannot regress the
	// durable counter.
	overlay := continuationTestLoop("conversation-m10-restart")
	overlay.ContinuationUsed = 1
	merged := mergeFreeStateLoops(recovered, overlay, true)
	if merged.ContinuationUsed != 2 {
		t.Fatalf("continuation usage regressed across merge: %d", merged.ContinuationUsed)
	}
	if merged.CurrentRoundID != loop.CurrentRoundID || merged.PriorityQueue == nil {
		t.Fatalf("round/queue identity lost across merge: %q/%v", merged.CurrentRoundID, merged.PriorityQueue)
	}
}

func TestM10RestartRecoversPhaseRoundLedgerFrontier(t *testing.T) {
	// The audioclosure event stream is the restart recovery source: Fold
	// rebuilds phase, diagnostic rounds, observations, and frontier.
	driver := audioclosure.Driver{}
	state, err := audioclosure.Start(audioclosure.StartRequest{
		ClosureID: "closure-m10", ConversationID: "conversation-m10",
		GoalID: "goal-m10", RunID: "run-m10", ProjectUUID: "proj-m10", ProjectRevision: "rev-m10",
		OriginalIntent: "inspect the project", Mode: audioclosure.ModeDiagnostic,
		Scope: audioclosure.Scope{Kind: "project"}, Policy: audioclosure.DefaultPolicy(), Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err = driver.TransitionPhase(state, state.Revision, audioclosure.PhaseFS0SemanticEntry, audioclosure.PhaseGuardInput{}, "", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = driver.AdmitRound(state, state.Revision, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := driver.RecordObservation(state, state.Revision, audioclosure.ObservationKey{
		ProjectUUID: "proj-m10", ProjectRevision: "rev-m10", Scope: audioclosure.Scope{Kind: "project"},
		ViewIDs: []string{"mix.frequency_relationship"},
	}, "obs-m10", time.Now().UTC())
	if err != nil || !outcome.Accepted {
		t.Fatalf("record observation: %+v err=%v", outcome, err)
	}
	state = outcome.State
	round := audioclosure.DiagnosticRoundRecord{
		SchemaVersion: audioclosure.DiagnosticRoundSchema, RoundID: "r_m10round00001",
		GoalID: "goal-m10", RunID: "run-m10", ConversationID: "conversation-m10",
		PrimaryDimension: audioclosure.DimensionFrequencyOccupancy, PriorityReason: audioclosure.PriorityDefaultOrder,
		ViewsRequested: []string{"mix.frequency_relationship"}, EvidenceStatus: audioclosure.RoundEvidenceReady,
		ProjectRevision: "rev-m10",
	}
	state, err = driver.RecordDiagnosticRound(state, state.Revision, round, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	state, _, err = driver.UpdateFrontier(state, state.Revision, audioclosure.HypothesisFrontier{
		Candidates: []audioclosure.Candidate{{ID: "candidate-m10", TrackIDs: []string{"1007"}}},
	}, audioclosure.ActionabilityUnknown, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	// Restart: replay the event stream and compare against the pre-crash state.
	replayed, err := audioclosure.Fold(state.Events)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if replayed.Phase != state.Phase || replayed.Revision != state.Revision {
		t.Fatalf("phase/revision drifted on restart: %s/%d vs %s/%d", replayed.Phase, replayed.Revision, state.Phase, state.Revision)
	}
	if len(replayed.DiagnosticRounds) != 1 || replayed.DiagnosticRounds[0].RoundID != round.RoundID {
		t.Fatalf("diagnostic rounds lost on restart: %+v", replayed.DiagnosticRounds)
	}
	if len(replayed.Observations) != 1 || len(replayed.Frontier.Candidates) != 1 {
		t.Fatalf("observations/frontier lost on restart: %+v", replayed)
	}
	_ = agentloop.FreeStateNeedsObservation
}

func TestM10SchedulerPathBudgetStopsLoop(t *testing.T) {
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-m10-sched")
	loop.ContinuationBudget = 1
	s.storeFreeStateLoop(loop)
	res := waitingContinuationResult("slice-1", "turn-1", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	if err := s.recordGoalResult("conversation-m10-sched", res); err != nil {
		t.Fatal(err)
	}
	// Budget 1 admits exactly one resume: the first enqueue stays pending.
	pending := 0
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("first resume was not admitted: %d pending", pending)
	}
	stored, _ := s.freeStateLoop("conversation-m10-sched")
	if stored.ContinuationUsed != 1 || stored.Status == "blocked" {
		t.Fatalf("budget accounting drifted: used=%d status=%s", stored.ContinuationUsed, stored.Status)
	}
	// The second waiting_continue boundary exceeds the budget: no new pending
	// continuation is scheduled, the loop settles blocked with a queryable
	// stop reason, and the terminal state is visible in the runtime projection.
	second := waitingContinuationResult("slice-2", "turn-2", agentruntime.StatusWaitingContinue, agentloop.StopReasonLimitReached)
	if err := s.recordGoalResult("conversation-m10-sched", second); err != nil {
		t.Fatal(err)
	}
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending || item.Status == ContinuationRunning {
			t.Fatalf("exhausted loop rescheduled itself: %+v", item)
		}
	}
	stored, _ = s.freeStateLoop("conversation-m10-sched")
	if stored.Status != "blocked" || stored.LatestDecision == nil ||
		strings.ToLower(stored.LatestDecision.Status) != "blocked" ||
		stored.LatestDecision.StopReason != freeStateContinuationBudgetExhaustedReason {
		t.Fatalf("budget stop did not settle the loop: %+v", stored.LatestDecision)
	}
	rows := s.continuationRuntimeProjection()
	found := false
	for _, row := range rows {
		if row["free_state_status"] == "blocked" && row["free_state_stop_reason"] == freeStateContinuationBudgetExhaustedReason {
			found = true
		}
	}
	if !found {
		t.Fatalf("budget stop not visible in runtime projection: %+v", rows)
	}
}

func TestM10WorkspaceRestoreRecoversSpineFields(t *testing.T) {
	// Workspace-level recovery: the same restore path the product uses after a
	// process restart (restoreProjectAgentRuntimeStateLocked over the persisted
	// agent_runtime_state.json shape) must recover the spine fields together
	// with the loop — equivalent to the JSON round-trip in the M10 persistence
	// test above, but through the production restore entry point.
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-m10-restore")
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	loop.CurrentPhase = string(audioclosure.PhaseFS4DiagnosticRound)
	loop.CurrentRoundID = "r_m10round00002"
	loop.ContinuationBudget = 4
	loop.ContinuationUsed = 2
	s.storeFreeStateLoop(loop)

	restored := &Server{}
	restored.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		FreeStateLoops: map[string]freeStateReasoningLoop{"conversation-m10-restore": loop},
	})
	recovered, ok := restored.freeStateLoop("conversation-m10-restore")
	if !ok {
		t.Fatal("workspace restore lost the free-state loop")
	}
	if recovered.CurrentPhase != loop.CurrentPhase || recovered.CurrentRoundID != loop.CurrentRoundID ||
		recovered.ContinuationBudget != 4 || recovered.ContinuationUsed != 2 || recovered.PriorityQueue == nil {
		t.Fatalf("spine fields lost across workspace restore: %+v", recovered)
	}
}

// D2-1.5-S2d: the D1 applied boundary must grant the phase-scoped post-apply
// budget floor so pre-apply consumption cannot starve the mandatory chain
// (fresh post-action CCB observation -> materiality/target evaluation), and the
// merge must never regress a granted floor through a stale continuation
// snapshot.

func TestD1AppliedBoundaryGrantsPostApplyBudgetFloorOnce(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 4, RequiresPostActionObservation: true}
	reserveD1PostApplySlices(&loop)
	if loop.ContinuationBudget != loop.ContinuationUsed+freeStateD1PostApplySliceNeed {
		t.Fatalf("post-apply floor = %d, want %d", loop.ContinuationBudget, loop.ContinuationUsed+freeStateD1PostApplySliceNeed)
	}
	if !loop.PostApplyBudgetReserved {
		t.Fatal("post-apply reserve flag was not set")
	}
	granted := loop.ContinuationBudget
	reserveD1PostApplySlices(&loop)
	if loop.ContinuationBudget != granted {
		t.Fatalf("post-apply floor was granted twice: %d", loop.ContinuationBudget)
	}
}

func TestD1PostApplyFloorSkipsVerifiedAndUninitializedLoops(t *testing.T) {
	verified := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 5, RequiresPostActionObservation: false}
	reserveD1PostApplySlices(&verified)
	if verified.ContinuationBudget != 6 || verified.PostApplyBudgetReserved {
		t.Fatalf("verified loop was granted a floor: budget=%d reserved=%v", verified.ContinuationBudget, verified.PostApplyBudgetReserved)
	}
	uninitialized := freeStateReasoningLoop{ContinuationBudget: 0, ContinuationUsed: 0, RequiresPostActionObservation: true}
	reserveD1PostApplySlices(&uninitialized)
	if uninitialized.ContinuationBudget != 0 || uninitialized.PostApplyBudgetReserved {
		t.Fatalf("uninitialized loop was granted a floor: budget=%d reserved=%v", uninitialized.ContinuationBudget, uninitialized.PostApplyBudgetReserved)
	}
}

func TestD1PostApplyFloorDoesNotShrinkLargerBudget(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 12, ContinuationUsed: 4, RequiresPostActionObservation: true}
	reserveD1PostApplySlices(&loop)
	if loop.ContinuationBudget != 12 {
		t.Fatalf("floor shrank a larger budget: %d", loop.ContinuationBudget)
	}
	if !loop.PostApplyBudgetReserved {
		t.Fatal("reserve flag should still mark the applied boundary")
	}
}

// The scheduler enqueue path (recordGoalResult) now fires the observation
// reserve before accounting the next slice, mirroring the interaction respond
// bridge. A post-apply chain driven purely by scheduler continuations must
// therefore keep its mandatory observation slice instead of blocking on the
// enqueue that would exceed the budget (2026-08-28 09:30 smoke shape:
// 5 -> 6 -> 7 against budget 6 with requires_post_action_observation=true).
func TestSchedulerEnqueueReserveKeepsPostApplyChainAlive(t *testing.T) {
	loop := freeStateReasoningLoop{ContinuationBudget: 6, ContinuationUsed: 4, RequiresPostActionObservation: true}
	// Applied boundary floor first (Fix A).
	reserveD1PostApplySlices(&loop)
	enqueue := func() bool {
		// Same ordering as recordGoalResult: reserve, account, then bound.
		reservePostActionObservationSlice(&loop)
		loop.ContinuationUsed++
		return loop.ContinuationUsed <= loop.ContinuationBudget
	}
	scheduled := 0
	for enqueue() {
		scheduled++
		if scheduled > 12 {
			t.Fatal("enqueue loop did not terminate")
		}
	}
	// Floor (used+3 at applied time) plus the one-shot observation reserve
	// grants exactly four post-apply slices beyond the applied boundary.
	if scheduled != freeStateD1PostApplySliceNeed+1 {
		t.Fatalf("post-apply slices scheduled = %d, want %d", scheduled, freeStateD1PostApplySliceNeed+1)
	}
}

func TestFreeStateLoopMergeDoesNotRegressGrantedBudgetFloor(t *testing.T) {
	base := continuationTestLoop("conversation-merge-budget")
	base.ContinuationBudget = 9
	base.ContinuationUsed = 4
	base.PostApplyBudgetReserved = true
	base.UpdatedAt = time.Now().UTC()
	// A continuation snapshot taken before the floor was granted still carries
	// the older (smaller) budget.
	overlay := continuationTestLoop(base.ConversationID)
	overlay.ContinuationBudget = 7
	overlay.ContinuationUsed = 3
	overlay.UpdatedAt = base.UpdatedAt.Add(-time.Second)
	merged := mergeFreeStateLoops(base, overlay, false)
	if merged.ContinuationBudget != 9 {
		t.Fatalf("merge regressed the granted budget floor: %d", merged.ContinuationBudget)
	}
	if merged.ContinuationUsed != 4 || !merged.PostApplyBudgetReserved {
		t.Fatalf("merge lost used/reserve state: used=%d reserved=%v", merged.ContinuationUsed, merged.PostApplyBudgetReserved)
	}
}
