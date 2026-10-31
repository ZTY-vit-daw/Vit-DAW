package agentloop

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// M3-FS-GATE-EVIDENCE-WIRE-1 A1: the slicebudget_1 g1_r1 repro, replayed at
// the exact shapes the real run left in its receipt
// (coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1/rounds/g1_r1.json):
//
//   - the chat-side admission audit path evaluates the gate on a snapshot-only
//     runState (no live recentObservation — what
//     AuditFreeStateNeedsExperimentGate builds);
//   - the frontier folded to blockers only ({"blockers":[G6,G8]}) — the
//     same-slice lag window before the first candidate fold;
//   - the ledger receipts hold the target-level evidence with full track
//     target_ref maps while available_views rows carry NO target_ref (the
//     track identity lives in the view key only);
//   - the proposal targets track 1010 and cites the newest track receipt.
//
// Before the wire fix this exact state failed G6 (no_selected_candidate) and
// G8 (target_not_from_frontier_candidate) with G1-G5+G7 passing. These tests
// pin the admission and the fail-closed boundaries around it.

// evidenceWireG1R1State rebuilds the g1_r1 audit context at the real receipt
// shapes: revision "2", seed mix scan + five track receipts on track 1010
// (the newest, obs_20261031T120646_69938d440c7d, partial), available_views
// rows without target_ref, frontier folded to blockers only, and no live
// recentObservation.
func evidenceWireG1R1State() *runState {
	trackTarget := func() map[string]any {
		return map[string]any{"id": "1010", "kind": "track", "label": "Track 2"}
	}
	receipt := func(observationID string, round int, observedAt string, status string, views ...string) map[string]any {
		requested := make([]any, 0, len(views))
		for _, view := range views {
			requested = append(requested, view)
		}
		return map[string]any{
			"observation_id": observationID, "receipt_id": "ccbr_" + observationID,
			"receipt_schema": "ccb_observation_receipt.v1", "requested_views": requested,
			"round": round, "status": status, "target_ref": trackTarget(),
			"project_revision": "2", "evidence_refs": []any{observationID},
			"freshness": map[string]any{
				"class": "current_observation", "status": "ready",
				"project_revision": "2", "observed_at": observedAt,
				"project_epoch": "epoch_f9e68539a270f478", "project_uuid": "vitproj_b7e4944365034f7a81c506b0c6dff00a",
			},
		}
	}
	ctx := map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "vitproj_b7e4944365034f7a81c506b0c6dff00a", "project_revision": "2",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version": "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning",
			"observation_ledger": map[string]any{
				"receipt_count": 6,
				// The seed project scan (real shape: no target_ref, mix view delivered).
				"receipts": []any{
					map[string]any{
						"observation_id": "obs_20261031T120437_9867985680f7", "receipt_id": "ccbr_obs_20261031T120437_9867985680f7",
						"receipt_schema": "ccb_observation_receipt.v1", "requested_views": []any{"mix.multitrack_relationship"},
						"round": 1, "status": "partial",
						"project_revision": "2", "evidence_refs": []any{"obs_20261031T120437_9867985680f7"},
						"freshness": map[string]any{
							"class": "current_observation", "status": "ready",
							"project_revision": "2", "observed_at": "2026-10-31T12:04:37.8379728Z",
						},
					},
					receipt("obs_20261031T120449_ffeb5dee88f6", 2, "2026-10-31T12:04:49.2130302Z", "partial", "track.basic_energy", "track.time_dynamics"),
					receipt("obs_20261031T120543_f3ef68473253", 5, "2026-10-31T12:05:43.8642431Z", "partial", "track.activity_structure"),
					receipt("obs_20261031T120605_d005f71777e9", 6, "2026-10-31T12:06:05.1659402Z", "partial", "track.transient_structure"),
					receipt("obs_20261031T120646_69938d440c7d", 7, "2026-10-31T12:06:46.1743485Z", "partial", "processor.identity_and_controls", "track.basic_energy", "track.time_dynamics"),
				},
				// Real g1_r1 shape: available_views rows carry NO target_ref — the
				// track identity lives in the view key only. The pre-fix gateG6
				// ledger fallback scanned exactly these rows.
				"available_views": map[string]any{
					"mix.multitrack_relationship": map[string]any{
						"view_id": "mix.multitrack_relationship", "status": "partial",
						"observation_id": "obs_20261031T120437_9867985680f7",
						"evidence_refs": []any{"obs_20261031T120437_9867985680f7"},
						"freshness": map[string]any{"class": "current_observation", "status": "ready", "project_revision": "2", "observed_at": "2026-10-31T12:04:37.8379728Z"},
					},
					"track:1010::track.basic_energy": map[string]any{
						"view_id": "track.basic_energy", "status": "ready",
						"observation_id": "obs_20261031T120646_69938d440c7d",
						"evidence_refs": []any{"obs_20261031T120646_69938d440c7d"},
						"freshness": map[string]any{"class": "current_observation", "status": "ready", "project_revision": "2", "observed_at": "2026-10-31T12:06:46.1743485Z"},
					},
				},
			},
		},
		// The real folded frontier: blockers only, no candidates, no candidate_id.
		"minimal_audio_closure": map[string]any{
			"project_uuid": "vitproj_b7e4944365034f7a81c506b0c6dff00a", "project_revision": "2",
			"hypothesis_frontier": map[string]any{
				"blockers": []any{"G6_target_evidence", "G8_target_consistency"},
			},
		},
	}
	// Chat-side audit path: snapshot-only state, recentObservation stays nil.
	return &runState{input: Input{Context: ctx}}
}

func evidenceWireG1R1Proposal(trackID string) *FreeStateDecision {
	return &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "target evidence supports a bounded gain hypothesis",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion: agentprotocol.ImprovementProposalSchema,
			Target:        map[string]any{"kind": "track", "id": trackID},
			EvidenceRefs:  []string{"obs_20261031T120646_69938d440c7d"},
			ImprovementIntent: "make this track stand out more in this passage", Hypothesis: "a small bounded gain rise makes it more prominent",
			ExpectedEffect: "the track sits more forward without masking the other", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind: "track_gain_adjust", ParameterBounds: map[string]any{"delta_db": 1}, Confidence: 0.55,
		},
	}
}

// The g1_r1 repro itself: on the chat-side audit path (no live
// recentObservation) the admission must pass with the evidence the ledger
// actually holds.
func TestFreeStateGateAdmitsG1R1ShapeOnChatAuditPath(t *testing.T) {
	state := evidenceWireG1R1State()
	if state.recentObservation != nil {
		t.Fatal("fixture must model the chat audit path: no recentObservation")
	}
	failed := evaluateFreeStateNeedsExperimentGate(state, evidenceWireG1R1Proposal("1010"))
	if len(failed) != 0 {
		t.Fatalf("the g1_r1 evidence shape must admit on the chat audit path, failed=%v", failed)
	}
}

// The same public entry the chat audit path calls must agree — pinning the
// wiring seam the run actually exercised.
func TestAuditFreeStateGateAdmitsG1R1Shape(t *testing.T) {
	state := evidenceWireG1R1State()
	audit := AuditFreeStateNeedsExperimentGate(state.input.Context, evidenceWireG1R1Proposal("1010"))
	if !audit.Passed || !audit.Proposal || !audit.ProposalValid {
		t.Fatalf("audit must pass on the g1_r1 shape: passed=%v proposal=%v valid=%v failed=%v",
			audit.Passed, audit.Proposal, audit.ProposalValid, audit.FailedGateIDs)
	}
}

// Fail-closed boundary 1: a proposal on a track the ledger never observed
// still fails G8 — the ledger fallback establishes exactly the observed track
// set and nothing wider.
func TestFreeStateGateStillRejectsUnobservedTrackOnChatAuditPath(t *testing.T) {
	state := evidenceWireG1R1State()
	failed := evaluateFreeStateNeedsExperimentGate(state, evidenceWireG1R1Proposal("9999"))
	found := false
	for _, id := range failed {
		if id == freeStateGateG8 {
			found = true
		}
	}
	if !found {
		t.Fatalf("a proposal targeting an unobserved track must still fail G8, failed=%v", failed)
	}
}

// Fail-closed boundary 2: with no usable track-targeted receipt at all (every
// track receipt insufficient, no live observation) G6 keeps failing — the
// ledger fallback admits nothing on its own.
func TestFreeStateGateStillFailsG6WithoutUsableTrackReceipt(t *testing.T) {
	state := evidenceWireG1R1State()
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loop["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		if ledgerRowTrackTarget(row) != "" {
			row["status"] = "insufficient"
		}
	}
	if messageLoopLedgerFreshTrackObservationTarget(state) != "" {
		t.Fatal("ledger fallback must return empty when no usable track receipt exists")
	}
	failed := evaluateFreeStateNeedsExperimentGate(state, evidenceWireG1R1Proposal("1010"))
	found := false
	for _, id := range failed {
		if id == freeStateGateG6 {
			found = true
		}
	}
	if !found {
		t.Fatalf("G6 must keep failing with no usable track evidence, failed=%v", failed)
	}
}

// The ledger fallback picks the freshest usable track receipt (round first,
// then observed_at) — the same newest-wins order the fold consumes, so G6 and
// G8's shared candidate-to-be track set stays the newest observation's track.
func TestFreeStateLedgerFallbackPrefersFreshestTrackReceipt(t *testing.T) {
	state := evidenceWireG1R1State()
	loop := messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])
	ledger := messageLoopMapValue(loop["observation_ledger"])
	// A later round on a different track: the fold's candidate-to-be becomes 2020.
	ledger["receipts"] = append(messageLoopMapRows(ledger["receipts"]), map[string]any{
		"observation_id": "obs_newer_round_track", "receipt_id": "ccbr_obs_newer_round_track",
		"receipt_schema": "ccb_observation_receipt.v1", "requested_views": []any{"track.basic_energy"},
		"round": 9, "status": "ready", "target_ref": map[string]any{"id": "2020", "kind": "track"},
		"project_revision": "2", "evidence_refs": []any{"obs_newer_round_track"},
		"freshness": map[string]any{"class": "current_observation", "status": "ready", "project_revision": "2", "observed_at": "2026-10-31T12:08:00.0000000Z"},
	})
	if got := messageLoopLedgerFreshTrackObservationTarget(state); got != "2020" {
		t.Fatalf("ledger fallback must pick the newest usable track receipt's track, got %q", got)
	}
}

// The live-observation arm keeps precedence when present: a recentObservation
// on another track is what the fold is about to consume, exactly as before
// the wire fix (chat path gains the ledger arm only when the live one is
// absent).
func TestFreeStateFreshTrackTargetPrefersLiveObservation(t *testing.T) {
	state := evidenceWireG1R1State()
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request",
		Summary: map[string]any{
			"status":     "ready",
			"target_ref": map[string]any{"kind": "track", "id": "3030"},
		},
	}
	if got := messageLoopFreshTrackObservationTarget(state); got != "3030" {
		t.Fatalf("live observation must win over the ledger arm, got %q", got)
	}
}
