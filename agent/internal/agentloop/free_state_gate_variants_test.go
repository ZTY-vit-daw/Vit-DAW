package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/experiment"
)

// M06 variant mutators: each removes exactly one gate input from the full
// gateTestState fixture (see free_state_gate_test.go).

func gateVariantNoBinding(ctx map[string]any) {
	ctx["task_contract"] = map[string]any{"kind": "improvement"}
}

func gateVariantRevisionMismatch(ctx map[string]any) {
	ctx["task_contract"] = map[string]any{"kind": "improvement", "project_uuid": "proj-1", "project_revision": "rev-stale"}
}

func gateVariantNoCapacity(ctx map[string]any) {
	delete(ctx, "free_state_capacity_assessment")
}

func gateVariantCapacityBlocked(ctx map[string]any) {
	// Production enum: exceeds_free_state is the real blocked capacity level.
	ctx["free_state_capacity_assessment"] = map[string]any{"schema_version": "free_state_capacity_assessment.v1", "capacity_level": "exceeds_free_state", "selected_capability": "free_state_reasoning"}
}

func gateVariantTargetOnlyLedger(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	loop["observation_ledger"] = map[string]any{"receipts": []any{
		map[string]any{"status": "ready", "observation_id": "obs-target", "requested_views": []any{"track.timbre_frequency"}, "project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"}},
	}}
}

func gateVariantNoRounds(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	delete(loop, "diagnostic_rounds")
}

func gateVariantRoundNotClosed(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	loop["diagnostic_rounds"] = []any{map[string]any{
		"schema_version": "free_state_diagnostic_round.v1", "round_id": "r_gate00000002",
		"primary_dimension": "frequency_occupancy", "priority_reason": "default_order",
		"views_requested": []any{"track.timbre_frequency"}, "evidence_status": "ready",
		"unresolved_questions": []any{"which track owns the masking"}, "project_revision": "rev-7",
	}}
}

func gateVariantNoFrontier(ctx map[string]any) {
	closure := ctx["minimal_audio_closure"].(map[string]any)
	closure["hypothesis_frontier"] = map[string]any{}
}

func gateVariantNoSelectedCandidate(ctx map[string]any) {
	frontier := ctx["minimal_audio_closure"].(map[string]any)["hypothesis_frontier"].(map[string]any)
	delete(frontier, "candidate_id")
}

func gateVariantNoTargetEvidence(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	ledger := loop["observation_ledger"].(map[string]any)
	ledger["available_views"] = map[string]any{}
	ledger["receipts"] = []any{map[string]any{
		"status": "ready", "observation_id": "obs-mix", "requested_views": []any{"mix.frequency_relationship"},
		"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
	}}
}

func gateVariantStaleTargetReceipt(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	ledger := loop["observation_ledger"].(map[string]any)
	for _, row := range ledger["receipts"].([]any) {
		receipt := row.(map[string]any)
		if receipt["observation_id"] == "obs-target" {
			receipt["freshness"] = map[string]any{"status": "stale", "project_revision": "rev-7"}
		}
	}
	view := ledger["available_views"].(map[string]any)["track:1007::track.timbre_frequency"].(map[string]any)
	view["freshness"] = map[string]any{"status": "stale", "project_revision": "rev-7"}
}

func gateVariantOldRevisionTargetReceipt(ctx map[string]any) {
	loop := ctx["free_state_reasoning_loop"].(map[string]any)
	ledger := loop["observation_ledger"].(map[string]any)
	for _, row := range ledger["receipts"].([]any) {
		receipt := row.(map[string]any)
		if receipt["observation_id"] == "obs-target" {
			receipt["project_revision"] = "rev-old"
			receipt["freshness"] = map[string]any{"status": "fresh", "project_revision": "rev-old"}
		}
	}
	view := ledger["available_views"].(map[string]any)["track:1007::track.timbre_frequency"].(map[string]any)
	view["project_revision"] = "rev-old"
	view["freshness"] = map[string]any{"status": "fresh", "project_revision": "rev-old"}
}

func TestM06GateRejectsEachMissingCondition(t *testing.T) {
	variants := []struct {
		name   string
		gateID string
		mutate func(ctx map[string]any)
	}{
		{"missing_project_binding", freeStateGateG1, gateVariantNoBinding},
		{"binding_revision_mismatch", freeStateGateG1, gateVariantRevisionMismatch},
		{"missing_capacity_assessment", freeStateGateG2, gateVariantNoCapacity},
		{"capacity_blocked", freeStateGateG2, gateVariantCapacityBlocked},
		{"missing_project_scan", freeStateGateG3, gateVariantTargetOnlyLedger},
		{"no_closed_dimension", freeStateGateG4, gateVariantNoRounds},
		{"dimension_not_closed", freeStateGateG4, gateVariantRoundNotClosed},
		{"no_frontier", freeStateGateG5, gateVariantNoFrontier},
		{"no_selected_candidate", freeStateGateG6, gateVariantNoSelectedCandidate},
		{"no_target_evidence", freeStateGateG6, gateVariantNoTargetEvidence},
		{"stale_evidence_ref", freeStateGateG7, gateVariantStaleTargetReceipt},
		{"revision_unbound_evidence_ref", freeStateGateG7, gateVariantOldRevisionTargetReceipt},
	}
	for _, variant := range variants {
		state := gateTestState(variant.mutate)
		decision := gateTestProposal(nil).FreeStateDecision
		failed := evaluateFreeStateNeedsExperimentGate(state, decision)
		found := false
		for _, id := range failed {
			if id == variant.gateID {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: gate %s did not fail (failed=%v)", variant.name, variant.gateID, failed)
		}
		if issue := messageLoopFreeStateOutputIssue(state, gateTestProposal(nil)); !strings.Contains(issue, "needs_observation") {
			t.Fatalf("%s: rejection did not route to needs_observation: %q", variant.name, issue)
		}
	}
	// Unknown evidence refs fail G7 as well.
	state := gateTestState(nil)
	failed := evaluateFreeStateNeedsExperimentGate(state, gateTestProposal([]string{"obs-never-returned"}).FreeStateDecision)
	found := false
	for _, id := range failed {
		if id == freeStateGateG7 {
			found = true
		}
	}
	if !found {
		t.Fatalf("unknown evidence ref did not fail G7 (failed=%v)", failed)
	}
	// Diagnostic-only turns never pass the gate regardless of evidence.
	diagnosticOnly := gateTestState(func(ctx map[string]any) { ctx["free_state_diagnostic_only"] = true })
	if issue := messageLoopFreeStateOutputIssue(diagnosticOnly, gateTestProposal(nil)); !strings.Contains(issue, "diagnostic-only") {
		t.Fatalf("diagnostic-only needs_experiment escaped the diagnostic boundary: %q", issue)
	}
}

func TestM07GatePassAdmitsExperimentConstruction(t *testing.T) {
	state := gateTestState(nil)
	proposal := gateTestProposal(nil)
	if failed := evaluateFreeStateNeedsExperimentGate(state, proposal.FreeStateDecision); len(failed) != 0 {
		t.Fatalf("full gate fixture still failed: %v", failed)
	}
	if issue := messageLoopFreeStateOutputIssue(state, proposal); issue != "" {
		t.Fatalf("gate-passing proposal rejected: %q", issue)
	}
	// Gate pass admits constructing experiment.Admission from the G5/G6/G7
	// evidence and passing Validate. evidence_status stays plausible: the gate
	// never demands an objective defect (ADR §6).
	admission := &experiment.Admission{
		SchemaVersion:        experiment.SchemaVersion,
		TargetRef:            map[string]any{"kind": "track", "id": "1007"},
		EvidenceRefs:         []string{"obs-target"},
		Hypothesis:           "a small bounded change may improve separation",
		TypedAction:          map[string]any{"action_kind": "bounded_gain_adjustment"},
		DiagnosticDoseBounds: map[string]any{"delta_db": map[string]any{"min": -1.0, "max": 0.0}},
		RetainedDoseBounds:   map[string]any{"delta_db": map[string]any{"min": -0.5, "max": 0.0}},
		ExperimentBudget:     1,
		ExpectedEffect:       "the relationship should be easier to compare",
		VerificationPlan:     map[string]any{"views": []string{"mix.frequency_relationship"}},
		CheckpointRef:        "checkpoint-rev-7",
		RollbackPlan:         map[string]any{"strategy": "revert_to_checkpoint"},
		AuthorityMode:        experiment.AuthorityOrdinary,
	}
	if err := admission.Validate(); err != nil {
		t.Fatalf("gate-passing admission failed Validate: %v", err)
	}
	// With a gate input removed, the decision is rejected first and no
	// admission may be constructed.
	blocked := gateTestState(gateVariantNoFrontier)
	if failed := evaluateFreeStateNeedsExperimentGate(blocked, proposal.FreeStateDecision); len(failed) == 0 {
		t.Fatal("frontier-less state passed the gate")
	}
	if issue := messageLoopFreeStateOutputIssue(blocked, proposal); !strings.Contains(issue, "needs_observation") {
		t.Fatalf("frontier-less state did not route to needs_observation: %q", issue)
	}
}
