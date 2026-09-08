package chat

import (
	"testing"
	"time"
)

// SCAFFOLD-1: the phase-deferred final-candidate marker rides the durable
// free_state_reasoning_loop state like frontier_decision_round_granted. The
// marker is sticky for the loop's lifetime: a transport copy that predates
// the rejection never clears it, the counter only moves forward, and the
// JSON round-trips behind the loop map keep the fields intact.

func TestMergeFreeStateLoopsKeepsPhaseDeferredMarkerFromOverlay(t *testing.T) {
	base := continuationTestLoop("conversation-pdm")
	overlay := continuationTestLoop("conversation-pdm")
	overlay.UpdatedAt = base.UpdatedAt.Add(time.Second)
	overlay.PhaseDeferredFinalCandidate = true
	overlay.PhaseDeferredFinalCandidateCount = 2
	overlay.PhaseDeferredFinalCandidatePhase = "fs4_diagnostic_round"
	merged := mergeFreeStateLoops(base, overlay, true)
	if !merged.PhaseDeferredFinalCandidate {
		t.Fatal("merge must keep the overlay's phase-deferred marker")
	}
	if merged.PhaseDeferredFinalCandidateCount != 2 {
		t.Fatalf("merged count = %d, want 2", merged.PhaseDeferredFinalCandidateCount)
	}
	if merged.PhaseDeferredFinalCandidatePhase != "fs4_diagnostic_round" {
		t.Fatalf("merged phase = %q, want fs4_diagnostic_round", merged.PhaseDeferredFinalCandidatePhase)
	}
}

func TestMergeFreeStateLoopsOlderTransportNeverClearsMarker(t *testing.T) {
	base := continuationTestLoop("conversation-pdm")
	base.PhaseDeferredFinalCandidate = true
	base.PhaseDeferredFinalCandidateCount = 3
	base.PhaseDeferredFinalCandidatePhase = "fs4_diagnostic_round"
	overlay := continuationTestLoop("conversation-pdm")
	overlay.UpdatedAt = base.UpdatedAt.Add(-time.Second)
	merged := mergeFreeStateLoops(base, overlay, true)
	if !merged.PhaseDeferredFinalCandidate {
		t.Fatal("a transport copy without the marker must not clear the durable marker")
	}
	if merged.PhaseDeferredFinalCandidateCount != 3 {
		t.Fatalf("counter regressed to %d, want 3", merged.PhaseDeferredFinalCandidateCount)
	}
	if merged.PhaseDeferredFinalCandidatePhase != "fs4_diagnostic_round" {
		t.Fatalf("recorded phase regressed to %q", merged.PhaseDeferredFinalCandidatePhase)
	}
}

func TestPhaseDeferredMarkerSurvivesLoopMapRoundTrip(t *testing.T) {
	loop := continuationTestLoop("conversation-pdm")
	loop.PhaseDeferredFinalCandidate = true
	loop.PhaseDeferredFinalCandidateCount = 1
	loop.PhaseDeferredFinalCandidatePhase = "fs5_candidate_frontier"
	roundTripped, ok := freeStateLoopFromAny(freeStateLoopMap(loop))
	if !ok {
		t.Fatal("loop map round-trip must stay schema-valid")
	}
	if !roundTripped.PhaseDeferredFinalCandidate || roundTripped.PhaseDeferredFinalCandidateCount != 1 ||
		roundTripped.PhaseDeferredFinalCandidatePhase != "fs5_candidate_frontier" {
		t.Fatalf("marker lost in the loop map round-trip: %+v", roundTripped)
	}
}

func TestPhaseDeferredMarkerAbsentByDefault(t *testing.T) {
	loop := continuationTestLoop("conversation-pdm")
	roundTripped, ok := freeStateLoopFromAny(freeStateLoopMap(loop))
	if !ok {
		t.Fatal("loop map round-trip must stay schema-valid")
	}
	if roundTripped.PhaseDeferredFinalCandidate || roundTripped.PhaseDeferredFinalCandidateCount != 0 ||
		roundTripped.PhaseDeferredFinalCandidatePhase != "" {
		t.Fatal("a loop without rejections must carry no marker (legacy records deserialize clean)")
	}
}
