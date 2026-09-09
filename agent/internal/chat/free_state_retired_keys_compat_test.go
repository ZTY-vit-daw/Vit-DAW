package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TIMING-1 persistence compatibility (§11 discipline): the retired chat
// persistence keys — the phase_deferred_final_candidate family (SCAFFOLD-1)
// and the needs_experiment_gate_open_{pending,signaled} pair (BOUNDARY-1
// §2.1) — are read-only-not-written: an old durable record carrying them
// still deserializes (unknown keys ignored), a loop written today never
// carries them, and the retired state dies with the record. The new
// anti-abuse accounting fields round-trip in their place.

func retiredKeysLegacyLoopJSON() string {
	return `{
		"schema_version": "free_state_reasoning_loop.v1",
		"loop_id": "free_state_retired",
		"conversation_id": "conversation-retired",
		"status": "reasoning",
		"original_intent": "improve the mix",
		"created_at": "2026-09-08T10:00:00Z",
		"updated_at": "2026-09-08T10:00:00Z",
		"phase_deferred_final_candidate": true,
		"phase_deferred_final_candidate_count": 2,
		"phase_deferred_final_candidate_phase": "fs4_diagnostic_round",
		"needs_experiment_gate_open_pending": true,
		"needs_experiment_gate_open_signaled": true
	}`
}

func TestRetiredPhaseDeferredKeysDeserializeCleanAndDieOut(t *testing.T) {
	loop, ok := freeStateLoopFromAny(json.RawMessage(retiredKeysLegacyLoopJSON()))
	if !ok {
		t.Fatal("a legacy record carrying retired keys must still deserialize as a valid loop")
	}
	if !freeStateLoopActive(loop) {
		t.Fatalf("legacy record must stay active, got status %q", loop.Status)
	}
	// The retired state never resurrects through the loop-map round-trip: the
	// fields have no struct backing, so re-marshalling drops the keys.
	remarshalled, err := json.Marshal(freeStateLoopMap(loop))
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	for _, retired := range []string{"phase_deferred_final_candidate", "needs_experiment_gate_open_pending", "needs_experiment_gate_open_signaled"} {
		if strings.Contains(string(remarshalled), retired) {
			t.Fatalf("retired key %q must not be written back: %s", retired, remarshalled)
		}
	}
	roundTripped, ok := freeStateLoopFromAny(json.RawMessage(remarshalled))
	if !ok {
		t.Fatal("loop map round-trip must stay schema-valid")
	}
	if roundTripped.ConversationID != loop.ConversationID {
		t.Fatalf("identity drifted through the round-trip: %q vs %q", roundTripped.ConversationID, loop.ConversationID)
	}
}

func TestAdmissionAccountingFieldsRideLoopMapRoundTrip(t *testing.T) {
	loop := continuationTestLoop("conversation-timing1")
	loop.AdmissionRejectionCount = 2
	loop.AdmissionRejectionGaps = []map[string]any{{
		"schema_version": "free_state_admission_gap.v1",
		"failed_gate_ids": []any{"G7_fresh_revision_bound_refs"},
		"missing": []any{map[string]any{
			"gate_id": "G7_fresh_revision_bound_refs", "condition": "unresolved_evidence_ref",
			"evidence_refs": []any{"obs-ghost"}, "freshness": "requires project_revision=rev-7",
		}},
		"quotable_fresh_reference": "obs-target@rev-7",
	}}
	loop.LastRejectedProposalFingerprint = "proposal:abcdef0123456789"
	loop.LastRejectedEvidenceRevision = "rev-7"
	roundTripped, ok := freeStateLoopFromAny(freeStateLoopMap(loop))
	if !ok {
		t.Fatal("loop map round-trip must stay schema-valid")
	}
	if roundTripped.AdmissionRejectionCount != 2 {
		t.Fatalf("admission_rejection_count lost in round-trip: %d", roundTripped.AdmissionRejectionCount)
	}
	if roundTripped.LastRejectedProposalFingerprint != "proposal:abcdef0123456789" ||
		roundTripped.LastRejectedEvidenceRevision != "rev-7" {
		t.Fatalf("fingerprint/revision lost in round-trip: %+v", roundTripped)
	}
	if len(roundTripped.AdmissionRejectionGaps) != 1 {
		t.Fatalf("gap records lost in round-trip: %+v", roundTripped.AdmissionRejectionGaps)
	}
}

// The durable merge keeps the anti-abuse accounting monotonic and atomic: a
// newer overlay rejection record owns the whole set; an older transport copy
// never regresses or resurrects it.
func TestMergeFreeStateLoopsKeepsAdmissionAccountingMonotonic(t *testing.T) {
	base := continuationTestLoop("conversation-timing1")
	overlay := continuationTestLoop("conversation-timing1")
	overlay.UpdatedAt = base.UpdatedAt.Add(time.Second)
	overlay.AdmissionRejectionCount = 1
	overlay.LastRejectedProposalFingerprint = "proposal:1111"
	overlay.LastRejectedEvidenceRevision = "rev-7"
	overlay.AdmissionRejectionGaps = []map[string]any{{"schema_version": "free_state_admission_gap.v1", "failed_gate_ids": []any{"G5_frontier_established"}}}
	merged := mergeFreeStateLoops(base, overlay, true)
	if merged.AdmissionRejectionCount != 1 || merged.LastRejectedProposalFingerprint != "proposal:1111" {
		t.Fatalf("merge dropped the newer rejection record: %+v", merged)
	}
	// A newer durable record must not regress to an older transport copy.
	newer := continuationTestLoop("conversation-timing1")
	newer.UpdatedAt = base.UpdatedAt.Add(2 * time.Second)
	newer.AdmissionRejectionCount = 2
	newer.LastRejectedProposalFingerprint = "proposal:2222"
	newer.AdmissionRejectionGaps = append(overlay.AdmissionRejectionGaps, map[string]any{"schema_version": "free_state_admission_gap.v1", "failed_gate_ids": []any{"G7_fresh_revision_bound_refs"}})
	olderOverlay := continuationTestLoop("conversation-timing1")
	olderOverlay.UpdatedAt = newer.UpdatedAt.Add(-time.Second)
	olderOverlay.AdmissionRejectionCount = 1
	olderOverlay.LastRejectedProposalFingerprint = "proposal:1111"
	merged = mergeFreeStateLoops(newer, olderOverlay, true)
	if merged.AdmissionRejectionCount != 2 || merged.LastRejectedProposalFingerprint != "proposal:2222" || len(merged.AdmissionRejectionGaps) != 2 {
		t.Fatalf("older transport copy regressed the rejection record: %+v", merged)
	}
}
