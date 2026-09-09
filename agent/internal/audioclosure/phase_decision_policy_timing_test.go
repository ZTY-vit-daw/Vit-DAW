package audioclosure

import "testing"

// TIMING-1 (2026-09-09, advisory ruling #5, direction 4b): the phase machine
// no longer vetoes proposals. needs_experiment / improvement_proposal are
// admitted in every FS phase; the G1-G8 evidence gate is the only semantic
// admission standard. This matrix pins the single source
// (phaseDecisionPolicies via AllowsDecisionStatus): proposal statuses
// everywhere, observation/action/terminal policies unchanged.

func TestTIMING1ProposalStatusesAdmittedInEveryFSPhase(t *testing.T) {
	// FS0–FS7 are the TIMING-1 change (the fs6+ veto removed); FS8 already
	// admitted needs_experiment for evaluation reports. FS9 stays
	// terminal-only: the loop is settled there and must not reopen.
	for _, phase := range FSPhaseOrder[:len(FSPhaseOrder)-1] {
		for _, status := range []string{"needs_experiment", "improvement_proposal", "NEEDS_EXPERIMENT", " Improvement_Proposal "} {
			if !AllowsDecisionStatus(phase, status) {
				t.Fatalf("%s must admit decision status %q after the timing-gate removal", phase, status)
			}
		}
	}
	for _, status := range []string{"needs_experiment", "improvement_proposal", "needs_observation", "needs_action"} {
		if AllowsDecisionStatus(PhaseFS9Terminal, status) {
			t.Fatalf("fs9 must stay terminal-only, admitted %q", status)
		}
	}
}

// The removal must not incidentally change the other status families'
// policies (card note: 维持原表). Observation keeps its ladder, needs_action
// stays gated to fs6/fs7, terminal-family keeps the NeedsObservation|| chain,
// fs9 stays terminal-only, and unknown statuses stay refused.
func TestTIMING1OtherStatusPoliciesUnchanged(t *testing.T) {
	observationPhases := []Phase{
		PhaseFS0SemanticEntry, PhaseFS1ProjectBound, PhaseFS2CapacityAssessed, PhaseFS3ProjectScan,
		PhaseFS4DiagnosticRound, PhaseFS5CandidateFrontier, PhaseFS6TargetConfirmed, PhaseFS8ExperimentVerification,
	}
	for _, phase := range observationPhases {
		if !AllowsDecisionStatus(phase, "needs_observation") {
			t.Fatalf("%s must still admit needs_observation", phase)
		}
	}
	if AllowsDecisionStatus(PhaseFS7ImprovementProposal, "needs_observation") {
		t.Fatal("fs7 must still refuse needs_observation")
	}
	if AllowsDecisionStatus(PhaseFS9Terminal, "needs_observation") {
		t.Fatal("fs9 must stay terminal-only for needs_observation")
	}
	for _, phase := range FSPhaseOrder {
		wantAction := phase == PhaseFS6TargetConfirmed || phase == PhaseFS7ImprovementProposal
		if got := AllowsDecisionStatus(phase, "needs_action"); got != wantAction {
			t.Fatalf("%s needs_action policy changed: got %v want %v", phase, got, wantAction)
		}
	}
	for _, status := range []string{"satisfied", "diagnostic_complete", "no_candidate_found", "capability_blocked", "blocked"} {
		if !AllowsDecisionStatus(PhaseFS1ProjectBound, status) {
			t.Fatalf("terminal-family statuses must stay admitted in fs1 via the observation chain: %s", status)
		}
		if !AllowsDecisionStatus(PhaseFS9Terminal, status) {
			t.Fatalf("fs9 must keep terminal decisions: %s", status)
		}
	}
	for _, phase := range FSPhaseOrder {
		if AllowsDecisionStatus(phase, "needs_lunch") {
			t.Fatalf("%s admitted an unknown status", phase)
		}
	}
	if AllowsDecisionStatus(Phase("fs12_unknown"), "needs_experiment") {
		t.Fatal("an unknown phase must refuse every status")
	}
}
