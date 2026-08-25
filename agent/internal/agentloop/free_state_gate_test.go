package agentloop

import (
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// Matrix M06/M07 (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md): the G1–G7
// needs_experiment admission gate — each missing condition rejects the
// decision, and a fully passing gate admits the experiment Admission.

// gateTestState builds a runState whose context satisfies every gate. Each
// variant removes exactly one condition.
func gateTestState(mutate func(ctx map[string]any)) *runState {
	ctx := map[string]any{
		"task_contract": map[string]any{
			"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-7",
		},
		"free_state_capacity_assessment": map[string]any{
			"schema_version":      "free_state_capacity_assessment.v1",
			"selected_capability": "project_mix", "capacity_level": "normal",
		},
		"free_state_reasoning_loop": map[string]any{
			"schema_version": "free_state_reasoning_loop.v1", "status": "reasoning", "original_intent": "improve the mix",
			"observation_ledger": map[string]any{
				"receipts": []any{
					map[string]any{
						"status": "ready", "observation_id": "obs-mix", "requested_views": []any{"mix.frequency_relationship"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
					map[string]any{
						"status": "ready", "observation_id": "obs-target", "requested_views": []any{"track.timbre_frequency"},
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
				"available_views": map[string]any{
					"track:1007::track.timbre_frequency": map[string]any{
						"view_id": "track.timbre_frequency", "status": "ready", "observation_id": "obs-target",
						"target_ref": map[string]any{"kind": "track", "id": "1007"}, "evidence_refs": []any{"obs-target"},
						"freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
					},
				},
			},
			"diagnostic_rounds": []any{map[string]any{
				"schema_version": "free_state_diagnostic_round.v1", "round_id": "r_gate00000001",
				"primary_dimension": "frequency_occupancy", "priority_reason": "default_order",
				"views_requested": []any{"track.timbre_frequency"},
				"evidence_status": "ready", "project_revision": "rev-7",
			}},
		},
		"minimal_audio_closure": map[string]any{
			"project_uuid": "proj-1", "project_revision": "rev-7",
			"hypothesis_frontier": map[string]any{
				"candidate_id": "candidate-1",
				"candidates":   []any{map[string]any{"id": "candidate-1", "track_ids": []any{"1007", "1012"}}},
			},
		},
	}
	if mutate != nil {
		mutate(ctx)
	}
	return &runState{input: Input{Context: ctx}}
}

func gateTestProposal(evidenceRefs []string) messageLoopOutput {
	if evidenceRefs == nil {
		evidenceRefs = []string{"obs-target"}
	}
	return messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "target evidence supports a bounded improvement hypothesis",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion: agentprotocol.ImprovementProposalSchema,
			Target:        map[string]any{"kind": "track", "id": "1007"}, EvidenceRefs: evidenceRefs,
			ImprovementIntent: "make the bass relationship feel clearer", Hypothesis: "a small bounded change may improve separation",
			ExpectedEffect: "the relationship should be easier to compare", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind: "bounded_gain_adjustment", ParameterBounds: map[string]any{"delta_db": -0.5}, Confidence: 0.55,
		},
	}}
}

func TestFreeStateGateG7UsesFreshnessClassBeforeObservationStatus(t *testing.T) {
	state := gateTestState(nil)
	ledger := messageLoopMapValue(messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		row["freshness"] = map[string]any{"status": "ready", "class": "current_observation", "project_revision": "rev-7"}
	}
	proposal := gateTestProposal(nil).FreeStateDecision
	if !gateG7(state, proposal.ImprovementProposal.EvidenceRefs) {
		t.Fatal("G7 rejected ready observation with a valid current_observation freshness class")
	}
}

func TestFreeStateGateG7RejectsFreshnessClassWithMismatchedRevision(t *testing.T) {
	state := gateTestState(nil)
	ledger := messageLoopMapValue(messageLoopMapValue(state.input.Context["free_state_reasoning_loop"])["observation_ledger"])
	for _, row := range messageLoopMapRows(ledger["receipts"]) {
		row["freshness"] = map[string]any{"status": "ready", "class": "current_observation", "project_revision": "rev-old"}
	}
	proposal := gateTestProposal(nil).FreeStateDecision
	if gateG7(state, proposal.ImprovementProposal.EvidenceRefs) {
		t.Fatal("G7 accepted a valid freshness class bound to the wrong revision")
	}
}

func TestFreeStateObservationRefreshesPhaseAndRevisionBeforeFinalGate(t *testing.T) {
	state := gateTestState(nil)
	state.input.Context["free_state_phase"] = "fs5_candidate_frontier"
	state.input.Context["minimal_audio_closure"].(map[string]any)["phase"] = "fs5_candidate_frontier"
	state.recentObservation = &RecentObservation{
		Tool: "ccb.observation_request", Status: "ok", ToolCallID: "ccb-target-refresh",
		Summary: map[string]any{
			"status": "ready", "observation_id": "obs-target-refresh", "project_revision": "rev-8",
			"project_binding": map[string]any{"project_uuid": "proj-1", "project_revision": "rev-8"},
			"requested_views": []any{"track.timbre_frequency"},
			"target_ref":      map[string]any{"kind": "track", "id": "1007"},
			"freshness":       map[string]any{"status": "ready", "class": "current_observation", "project_revision": "rev-8"},
			"views":           map[string]any{"track.timbre_frequency": map[string]any{"status": "ready"}},
		},
	}
	recordFreeStateCCBObservation(state, state.recentObservation)
	syncFreeStateRuntimeAfterObservation(state, state.recentObservation)
	closure := state.input.Context["minimal_audio_closure"].(map[string]any)
	if got := closure["project_revision"]; got != "rev-8" {
		t.Fatalf("closure revision = %v, want rev-8", got)
	}
	if got := state.input.Context["free_state_phase"]; got != "fs6_target_confirmed" {
		t.Fatalf("host phase = %v, want fs6_target_confirmed", got)
	}
	if got := closure["phase"]; got != "fs6_target_confirmed" {
		t.Fatalf("closure phase = %v, want fs6_target_confirmed", got)
	}
}
