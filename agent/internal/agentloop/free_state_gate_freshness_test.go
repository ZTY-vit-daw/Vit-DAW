package agentloop

import (
	"testing"
)

// FIX-GATE-FRESHNESS-1 (2026-09-21): the G-gates read two freshness domains.
// G3/G4/G7 read the in-loop observation ledger (updated the moment a tool
// executes); G5/G6/G8 read the minimal_audio_closure frontier projection that
// only refreshes when a slice's Result folds server-side. When a target-level
// observation and the proposal land in the SAME slice (PORT-PS1-N5-1 conv
// mix_single_tick_e2e_20260921_102317: track observations 10:23:38/10:23:52,
// chain-complete proposal bounced 10:24:01 with no_selected_candidate), G6/G8
// judge just-gathered evidence against a pre-fold snapshot. These tests pin
// the lag completion — the fresh observation the fold will consume satisfies
// the selection-dependent checks — while every real rejection (off-candidate
// target under an established selection, cross-track citation contamination)
// stays intact.

// gateFreshnessState is gateTestState with the same-slice lag window applied:
// the frontier candidates exist (folded from the earlier scan slice) but the
// candidate_id selection is still empty, and the in-slice target-level
// observation already sits in recentObservation and the ledger.
func gateFreshnessState(mutate func(ctx map[string]any)) *runState {
	state := gateTestState(mutate)
	state.input.Context["minimal_audio_closure"] = map[string]any{
		"project_uuid": "proj-1", "project_revision": "rev-7",
		"hypothesis_frontier": map[string]any{
			"candidate_id": "",
			"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
		},
	}
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request",
		Summary: map[string]any{
			"status":     "ready",
			"target_ref": map[string]any{"kind": "track", "id": "1007"},
		},
	}
	return state
}

// The R4 repro: selection lag must not bounce a chain-complete proposal.
func TestFreeStateGateAdmitsSameSliceTargetObservationProposal(t *testing.T) {
	state := gateFreshnessState(nil)
	failed := evaluateFreeStateNeedsExperimentGate(state, gateTestProposal(nil).FreeStateDecision)
	if len(failed) != 0 {
		t.Fatalf("a proposal whose target-level observation sits in the fresh ledger must not bounce on the pre-fold snapshot, failed=%v", failed)
	}
}

// The deeper same-slice window: scan, target observation, and proposal all
// before the first fold — the frontier projection is empty entirely while the
// ledger already carries both receipts.
func TestFreeStateGateAdmitsScanAndTargetSameSliceProposal(t *testing.T) {
	state := gateFreshnessState(func(ctx map[string]any) {
		ctx["minimal_audio_closure"] = map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7",
			"hypothesis_frontier": map[string]any{"candidate_id": "", "candidates": []any{}},
		}
	})
	failed := evaluateFreeStateNeedsExperimentGate(state, gateTestProposal(nil).FreeStateDecision)
	if len(failed) != 0 {
		t.Fatalf("a proposal over an empty pre-fold frontier must admit via the fresh scan+target receipts, failed=%v", failed)
	}
}

// Real rejections stay intact: with an established selection, a proposal
// targeting a track outside the selected candidate must still fail G8.
func TestFreeStateGateStillRejectsOffCandidateTargetUnderSelection(t *testing.T) {
	state := gateTestState(nil)
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request",
		Summary: map[string]any{
			"status":     "ready",
			"target_ref": map[string]any{"kind": "track", "id": "1007"},
		},
	}
	decision := gateTestProposal([]string{"obs-target"}).FreeStateDecision
	decision.ImprovementProposal.Target = map[string]any{"kind": "track", "id": "9999"}
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	found := false
	for _, id := range failed {
		if id == freeStateGateG8 {
			found = true
		}
	}
	if !found {
		t.Fatalf("an off-candidate target under an established selection must still fail G8, failed=%v", failed)
	}
}

// A target-level observation on a track outside the selected candidate must
// still fail G6 (the fresh-obs fallback never weakens a live selection).
func TestFreeStateGateStillRejectsOffCandidateObservationUnderSelection(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		closure := map[string]any{"project_uuid": "proj-1", "project_revision": "rev-7"}
		closure["hypothesis_frontier"] = map[string]any{
			"candidate_id": "candidate-2",
			"candidates": []any{
				map[string]any{"id": "candidate-1", "track_ids": []any{"1007"}},
				map[string]any{"id": "candidate-2", "track_ids": []any{"1012"}},
			},
		}
		ctx["minimal_audio_closure"] = closure
	})
	// recentObservation targets 1007 (candidate-1) while candidate-2 is selected.
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request",
		Summary: map[string]any{
			"status":     "ready",
			"target_ref": map[string]any{"kind": "track", "id": "1007"},
		},
	}
	failed := evaluateFreeStateNeedsExperimentGate(state, gateTestProposal(nil).FreeStateDecision)
	found := false
	for _, id := range failed {
		if id == freeStateGateG6 {
			found = true
		}
	}
	if !found {
		t.Fatalf("a target observation outside the selected candidate must still fail G6, failed=%v", failed)
	}
}

// Budget guard: a bounce the gate no longer sustains must not consume the
// 2-strike admission budget (the note function re-derives the gate verdict and
// only counts issue-identical rejections).
func TestFreeStateGateFalseBounceDoesNotBurnAdmissionBudget(t *testing.T) {
	state := gateFreshnessState(nil)
	out := gateTestProposal(nil)
	messageLoopFreeStateNoteAdmissionRejection(state, out, "needs_experiment requires the full admission gate; structured gap=stale")
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	if got := messageLoopFreeStatePositiveInt(loop[freeStateAdmissionRejectionCountKey]); got != 0 {
		t.Fatalf("a rejection the gate does not sustain must not consume the admission budget, count=%d", got)
	}
	if freeStateBool(loop[freeStateTerminalTurnLockedKey]) {
		t.Fatalf("a rejection the gate does not sustain must not lock the terminal turn")
	}
}
