package agentloop

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
)

// AGENT-2 A3 (2026-09-05): the G8 target-consistency assertions. Each RED
// fixture is the M5 wrong-target class — a proposal whose target or evidence
// disagrees with the frontier/ledger the same run produced. The green control
// is the shared gateTestState fixture (target 1007, candidate tracks
// ["1007","1012"], target-level observation obs-target bound to 1007).

func gateTargetProposal(target map[string]any, evidenceRefs []string) *FreeStateDecision {
	if evidenceRefs == nil {
		evidenceRefs = []string{"obs-target"}
	}
	return &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "target evidence supports a bounded improvement hypothesis",
		ImprovementProposal: &agentprotocol.ImprovementProposal{
			SchemaVersion: agentprotocol.ImprovementProposalSchema,
			Target:        target, EvidenceRefs: evidenceRefs,
			ImprovementIntent: "make the relationship clearer", Hypothesis: "a small bounded change may help",
			ExpectedEffect: "easier comparison", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
			ActionKind: "bounded_gain_adjustment", Confidence: 0.55,
		},
	}
}

func gateFailedIDs(failed []string, id string) bool {
	for _, row := range failed {
		if row == id {
			return true
		}
	}
	return false
}

func TestFreeStateGateG8AcceptsConsistentTarget(t *testing.T) {
	state := gateTestState(nil)
	decision := gateTargetProposal(map[string]any{"kind": "track", "id": "1007"}, nil)
	if failed := evaluateFreeStateNeedsExperimentGate(state, decision); len(failed) != 0 {
		t.Fatalf("consistent target failed the gate: %v", failed)
	}
	// A project-level scan ref alongside the target ref stays admissible.
	decision = gateTargetProposal(map[string]any{"kind": "track", "id": "1007"}, []string{"obs-target", "obs-mix"})
	if failed := evaluateFreeStateNeedsExperimentGate(state, decision); len(failed) != 0 {
		t.Fatalf("target ref plus project-level ref failed the gate: %v", failed)
	}
	// The candidate's second track is equally consistent when the ledger
	// carries its own target-level observation.
	consistentSecond := gateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		ledger := loop["observation_ledger"].(map[string]any)
		ledger["receipts"] = append(ledger["receipts"].([]any), map[string]any{
			"status": "ready", "observation_id": "obs-1012", "requested_views": []any{"track.time_dynamics"},
			"target_ref": map[string]any{"kind": "track", "id": "1012"}, "evidence_refs": []any{"obs-1012"},
			"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
		})
	})
	decision = gateTargetProposal(map[string]any{"kind": "track", "id": "1012"}, []string{"obs-1012"})
	if failed := evaluateFreeStateNeedsExperimentGate(consistentSecond, decision); len(failed) != 0 {
		t.Fatalf("second frontier track with its own observation failed the gate: %v", failed)
	}
}

// The bass wrong-target class (M5/DIAG1): the proposal narrates a track the
// selected candidate never established, even though the ledger itself is
// perfectly fresh and consistent. G5/G6/G7 alone admit this form; G8 must red.
func TestFreeStateGateG8RejectsTargetOutsideSelectedCandidate(t *testing.T) {
	state := gateTestState(nil)
	// Track 1032 exists in the project (fixture-embedded history collision
	// class) but is outside the selected candidate's tracks ["1007","1012"].
	decision := gateTargetProposal(map[string]any{"kind": "track", "id": "1032"}, []string{"obs-mix"})
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	if !gateFailedIDs(failed, freeStateGateG8) {
		t.Fatalf("target outside the selected candidate did not fail G8 (failed=%v)", failed)
	}
	if issue := messageLoopFreeStateOutputIssue(state, gateTestProposalOutput(decision.ImprovementProposal)); !strings.Contains(issue, "needs_observation") {
		t.Fatalf("G8 rejection did not route to needs_observation: %q", issue)
	}
}

// A proposal targeting a valid frontier track while citing only another
// track's observations: internally inconsistent evidence, must red.
func TestFreeStateGateG8RejectsCrossTrackEvidenceRefs(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		ledger := loop["observation_ledger"].(map[string]any)
		ledger["receipts"] = append(ledger["receipts"].([]any), map[string]any{
			"status": "ready", "observation_id": "obs-other-track", "requested_views": []any{"track.time_dynamics"},
			"target_ref": map[string]any{"kind": "track", "id": "1012"}, "evidence_refs": []any{"obs-other-track"},
			"project_revision": "rev-7", "freshness": map[string]any{"status": "fresh", "project_revision": "rev-7"},
		})
	})
	decision := gateTargetProposal(map[string]any{"kind": "track", "id": "1007"}, []string{"obs-other-track"})
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	if !gateFailedIDs(failed, freeStateGateG8) {
		t.Fatalf("cross-track evidence ref did not fail G8 (failed=%v)", failed)
	}
}

// The target-level observation exists but is bound to a different track than
// the proposal targets (target_ref disagreement), and no usable target-level
// row covers the proposal target at all.
func TestFreeStateGateG8RejectsTargetWithoutBoundTargetLevelObservation(t *testing.T) {
	state := gateTestState(func(ctx map[string]any) {
		loop := ctx["free_state_reasoning_loop"].(map[string]any)
		ledger := loop["observation_ledger"].(map[string]any)
		receipts := ledger["receipts"].([]any)
		for _, row := range receipts {
			receipt := row.(map[string]any)
			if receipt["observation_id"] == "obs-target" {
				// Rebind the target-level observation onto the candidate's
				// other track: still usable, still fresh, no longer the
				// proposal's target.
				receipt["target_ref"] = map[string]any{"kind": "track", "id": "1012"}
			}
		}
		view := ledger["available_views"].(map[string]any)["track:1007::track.timbre_frequency"].(map[string]any)
		view["target_ref"] = map[string]any{"kind": "track", "id": "1012"}
		view["evidence_refs"] = []any{"obs-target"}
	})
	decision := gateTargetProposal(map[string]any{"kind": "track", "id": "1007"}, []string{"obs-target"})
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	if !gateFailedIDs(failed, freeStateGateG8) {
		t.Fatalf("target without a bound target-level observation did not fail G8 (failed=%v)", failed)
	}
}

// Non-track targets have no target-level evidence semantics at all; every
// D1-S1 domain is track-level, so anything else fails closed.
func TestFreeStateGateG8RejectsNonTrackTarget(t *testing.T) {
	state := gateTestState(nil)
	decision := gateTargetProposal(map[string]any{"kind": "project", "id": "proj-1"}, []string{"obs-mix"})
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	if !gateFailedIDs(failed, freeStateGateG8) {
		t.Fatalf("non-track target did not fail G8 (failed=%v)", failed)
	}
}

// TIMING-1: the G8 refusal discloses its failed consistency conditions as
// machine-readable vocabulary and no longer names the selected candidate's
// track set — track identities are a content-blind red line (advisory ruling
// #5 anti-abuse rule 1).
func TestFreeStateGateG8RefusalIsStructuredAndContentBlind(t *testing.T) {
	state := gateTestState(nil)
	decision := gateTargetProposal(map[string]any{"kind": "track", "id": "1032"}, []string{"obs-mix"})
	failed := evaluateFreeStateNeedsExperimentGate(state, decision)
	if !gateFailedIDs(failed, freeStateGateG8) {
		t.Fatalf("wrong-target proposal did not fail G8 (failed=%v)", failed)
	}
	message := freeStateNeedsExperimentGateFailureMessage(state, decision, failed)
	if !strings.Contains(message, "G8_target_consistency") {
		t.Fatalf("refusal message omitted the G8 gate id: %q", message)
	}
	if !strings.Contains(message, "target_not_from_frontier_candidate") {
		t.Fatalf("refusal message omitted the failed condition: %q", message)
	}
	// Content-blind: no track identity from the fixture may appear.
	for _, track := range []string{"1007", "1012", "1032"} {
		if strings.Contains(message, track) {
			t.Fatalf("refusal message leaked track identity %q: %q", track, message)
		}
	}
}

// gateTestProposalOutput wraps a proposal in the messageLoopOutput shape the
// output-issue check consumes.
func gateTestProposalOutput(proposal *agentprotocol.ImprovementProposal) messageLoopOutput {
	return messageLoopOutput{Final: true, FreeStateDecision: &FreeStateDecision{
		SchemaVersion: FreeStateDecisionSchema, Status: FreeStateNeedsExperiment, EvidenceStatus: "plausible",
		Summary: "target evidence supports a bounded improvement hypothesis", ImprovementProposal: proposal,
	}}
}
