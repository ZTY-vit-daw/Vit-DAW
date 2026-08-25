package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/audioclosure"
)

// Production wiring for the diagnostic priority queue: syncFreeStateSpine
// mirrors the queue derived from the closure's diagnostic round records into
// the free-state loop, so messageLoopFreeStateQueueStillOpen reads production
// data (docs/FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md).

func TestSyncFreeStateSpineMirrorsDerivedPriorityQueue(t *testing.T) {
	s := testContinuationServer()
	s.storeFreeStateLoop(continuationTestLoop("conversation-queue"))
	state := audioclosure.State{
		SchemaVersion:  audioclosure.SchemaVersion,
		ClosureID:      "closure-queue",
		ConversationID: "conversation-queue",
		Phase:          audioclosure.Phase("fs4_diagnostic_round"),
		DiagnosticRounds: []audioclosure.DiagnosticRoundRecord{{
			SchemaVersion:    audioclosure.DiagnosticRoundSchema,
			RoundID:          "r_queue00000001",
			PrimaryDimension: audioclosure.DimensionLevelHeadroom,
			PriorityReason:   audioclosure.PriorityDefaultOrder,
			ViewsRequested:   []string{"track.basic_energy"},
			EvidenceStatus:   audioclosure.RoundEvidenceReady,
			ProjectRevision:  "rev-7",
			SkippedDimensions: []audioclosure.SkippedDimension{{
				Dimension: audioclosure.DimensionStereoSpace, Reason: audioclosure.SkipNotApplicable,
			}},
		}},
	}
	s.syncFreeStateSpine(state)
	loop, ok := s.freeStateLoop("conversation-queue")
	if !ok {
		t.Fatal("loop missing after spine sync")
	}
	if loop.CurrentRoundID != "r_queue00000001" {
		t.Fatalf("round id not mirrored: %q", loop.CurrentRoundID)
	}
	if loop.PriorityQueue == nil {
		t.Fatal("priority queue not mirrored into loop")
	}
	if err := loop.PriorityQueue.Validate(); err != nil {
		t.Fatalf("mirrored queue invalid: %v", err)
	}
	status := map[audioclosure.DiagnosticDimension]audioclosure.QueueEntryStatus{}
	for _, entry := range loop.PriorityQueue.Entries {
		status[entry.Dimension] = entry.Status
	}
	if status[audioclosure.DimensionLevelHeadroom] != audioclosure.QueueClosed {
		t.Fatal("closed round did not close its dimension in the mirrored queue")
	}
	if status[audioclosure.DimensionStereoSpace] != audioclosure.QueueSkipped {
		t.Fatal("recorded skip not applied in the mirrored queue")
	}
	if status[audioclosure.DimensionFrequencyOccupancy] != audioclosure.QueueOpen {
		t.Fatal("unvisited dimension not open in the mirrored queue")
	}
}

// WP0 capability_blocked cause observability: a capability-blocked closure
// settlement records the concrete boundary (capacity routing level, selected
// governed capability, continuation budget numbers) into LatestDecision
// .Limitations, which the continuation rows expose as free_state_limitations.
func TestCapabilityBlockedSettlementRecordsBoundaryDetail(t *testing.T) {
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-capdetail")
	loop.ContinuationBudget, loop.ContinuationUsed = 6, 4
	s.storeFreeStateLoop(loop)
	now := time.Now().UTC()
	s.mu.Lock()
	s.capabilityRoutes = map[string]CapabilityRouteRecord{
		"route-capdetail": {
			SchemaVersion: "capability_route.v1", ConversationID: "conversation-capdetail",
			Assessment: &FreeStateCapacityAssessment{CapacityLevel: "exceeds_free_state"},
			UpdatedAt:  now,
		},
	}
	s.mu.Unlock()
	s.settleFreeStateLoopFromClosure("conversation-capdetail", audioclosure.Settlement{
		Reason: audioclosure.StopCapabilityBlocked, Summary: "no governed executable path", Round: 2, SettledAt: now,
	})
	settled, ok := s.freeStateLoop("conversation-capdetail")
	if !ok || settled.Status != "capability_blocked" {
		t.Fatalf("loop did not settle capability_blocked: %s", settled.Status)
	}
	if len(settled.AdmissionReceipt) == 0 || firstStringFromMap(settled.AdmissionReceipt, "boundary") != "proposal_missing" {
		t.Fatalf("capability boundary admission receipt missing: %+v", settled.AdmissionReceipt)
	}
	if settled.LatestDecision == nil {
		t.Fatal("settled loop has no latest decision")
	}
	detail := strings.Join(settled.LatestDecision.Limitations, ";")
	for _, want := range []string{"capacity_level=exceeds_free_state", "selected_capability=none", "continuation_budget=4/6"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("boundary detail missing %q in %q", want, detail)
		}
	}
}

func TestOpenQueueWithinFreeStateCapacityDoesNotTerminallyBlockAtBoundary(t *testing.T) {
	s := testContinuationServer()
	loop := continuationTestLoop("conversation-open-boundary")
	queue := audioclosure.DefaultPriorityQueue()
	loop.PriorityQueue = &queue
	s.storeFreeStateLoop(loop)
	now := time.Now().UTC()
	s.mu.Lock()
	s.capabilityRoutes = map[string]CapabilityRouteRecord{
		"route-open-boundary": {SchemaVersion: "capability_route.v1", ConversationID: "conversation-open-boundary", UpdatedAt: now,
			Assessment: &FreeStateCapacityAssessment{CapacityLevel: "within_free_state", SelectedCapability: "free_state"}},
	}
	s.mu.Unlock()
	state := audioclosure.State{SchemaVersion: audioclosure.SchemaVersion, ClosureID: "closure-open-boundary", ContractID: "contract-open-boundary", ConversationID: "conversation-open-boundary", ProjectUUID: "project", ProjectRevision: "rev-1", Phase: audioclosure.PhaseFS2CapacityAssessed}
	next, err := s.settleTaskAtAudioClosureBoundary(state, "no_pending_mix_tick_candidate")
	if err != nil {
		t.Fatal(err)
	}
	if next.Terminal() {
		t.Fatal("open free-state queue was converted into a terminal capability boundary")
	}
}
