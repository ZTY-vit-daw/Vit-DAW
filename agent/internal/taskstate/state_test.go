package taskstate

import (
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)

func testContract(kind ContractKind) Contract {
	return Contract{
		ContractID: "contract-1", TaskID: "task-1", GoalID: "goal-1", RunID: "run-1",
		ConversationID: "conversation-1", OriginalIntent: "inspect and improve the current project",
		Kind: kind, Scope: Scope{Kind: "project", ID: "project-1"}, Temporary: true,
		TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria:   []string{"evidence-backed task outcome"},
		EvidenceRequirements: []string{"project observation references"},
		ProjectUUID:          "project-1", ProjectRevision: "rev-1",
	}
}

func mustApply(t *testing.T, contract Contract, current Snapshot, request TransitionRequest) Snapshot {
	t.Helper()
	next, err := Apply(contract, current, request, testNow.Add(time.Duration(current.Revision+1)*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestOpenImprovementCannotSettleFromLocalDiagnosticOrProposal(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, err := New(contract, testNow)
	if err != nil {
		t.Fatal(err)
	}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventDiagnosticCompleted, Reason: "bounded evidence is sufficient for one diagnosis", EvidenceRefs: []string{"obs-1"}})
	if _, err := Apply(contract, state, TransitionRequest{Event: EventTaskSettled, Reason: "model said satisfied", EvidenceRefs: []string{"obs-1"}}, testNow); err == nil {
		t.Fatal("open improvement contract settled from diagnostic_complete")
	}
	proposal := &BoundedProposal{ProposalID: "proposal-1", Summary: "test a bounded balance change", EvidenceRefs: []string{"obs-1"}, RequiresExperiment: true}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventImprovementProposed, Reason: "candidate found", Proposal: proposal})
	if _, err := Apply(contract, state, TransitionRequest{Event: EventTaskSettled, Reason: "proposal is enough", EvidenceRefs: []string{"obs-1"}}, testNow); err == nil {
		t.Fatal("open improvement contract settled from improvement_proposal")
	}
}

func TestImprovementRequiresDurableExperimentBeforeSettlement(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, _ := New(contract, testNow)
	proposal := &BoundedProposal{ProposalID: "proposal-1", Summary: "bounded candidate", EvidenceRefs: []string{"obs-1"}, Bounds: []string{"one target"}, RequiresExperiment: true}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventImprovementProposed, Reason: "candidate found", Proposal: proposal})
	if _, err := Apply(contract, state, TransitionRequest{Event: EventExperimentRequired, Reason: "runtime admission"}, testNow); err == nil {
		t.Fatal("needs_experiment admitted without experiment identity")
	}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventExperimentRequired, Reason: "runtime admission", ExperimentID: "experiment-1"})
	state = mustApply(t, contract, state, TransitionRequest{Event: EventTaskSettled, Reason: "experiment produced bounded evidence", EvidenceRefs: []string{"after-1"}})
	if state.State != StateSettled || !state.Terminal || state.ExperimentID != "experiment-1" {
		t.Fatalf("unexpected settlement: %+v", state)
	}
}

func TestHumanJudgmentRequiresDurablePendingInteraction(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, _ := New(contract, testNow)
	proposal := &BoundedProposal{ProposalID: "proposal-1", Summary: "bounded candidate", EvidenceRefs: []string{"obs-1"}, RequiresExperiment: true}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventImprovementProposed, Reason: "candidate found", Proposal: proposal})
	state = mustApply(t, contract, state, TransitionRequest{Event: EventExperimentRequired, Reason: "runtime admission", ExperimentID: "experiment-1"})
	if _, err := Apply(contract, state, TransitionRequest{Event: EventHumanJudgmentRequested, Reason: "audition required", ExperimentID: "experiment-1"}, testNow); err == nil {
		t.Fatal("human judgment admitted without pending interaction")
	}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventHumanJudgmentRequested, Reason: "audition required", ExperimentID: "experiment-1", PendingInteraction: &PendingInteraction{InteractionID: "interaction-1", Kind: "audition_judgment", Reason: "compare A/B"}})
	if state.State != StateHumanJudgmentRequired || state.PendingInteraction == nil || state.Terminal {
		t.Fatalf("unexpected human judgment state: %+v", state)
	}
}

func TestDiagnosticContractCanReturnBoundedProposalWithoutExperiment(t *testing.T) {
	contract := testContract(ContractDiagnostic)
	contract.AuthorizationBoundary = "observe_only"
	state, _ := New(contract, testNow)
	state = mustApply(t, contract, state, TransitionRequest{Event: EventDiagnosticCompleted, Reason: "diagnosis complete", EvidenceRefs: []string{"obs-1"}})
	proposal := &BoundedProposal{ProposalID: "proposal-1", Summary: "bounded suggestion only", EvidenceRefs: []string{"obs-1"}, Bounds: []string{"no project mutation"}}
	state = mustApply(t, contract, state, TransitionRequest{Event: EventImprovementProposed, Reason: "read-only proposal", Proposal: proposal})
	state = mustApply(t, contract, state, TransitionRequest{Event: EventTaskSettled, Reason: "diagnostic-only contract fulfilled", EvidenceRefs: []string{"obs-1"}})
	if state.State != StateSettled || state.Proposal == nil || state.Proposal.RequiresExperiment {
		t.Fatalf("unexpected diagnostic settlement: %+v", state)
	}
}

func TestNoCandidateAndCapabilityBlockedRemainDistinctTerminalOutcomes(t *testing.T) {
	contract := testContract(ContractImprovement)
	noCandidate, _ := New(contract, testNow)
	noCandidate = mustApply(t, contract, noCandidate, TransitionRequest{Event: EventNoCandidateReported, Reason: "bounded search exhausted", EvidenceRefs: []string{"obs-1"}})
	blocked, _ := New(contract, testNow)
	blocked = mustApply(t, contract, blocked, TransitionRequest{Event: EventCapabilityBlocked, Reason: "required adapter unavailable"})
	if noCandidate.State != StateNoCandidateFound || blocked.State != StateCapabilityBlocked || !noCandidate.Terminal || !blocked.Terminal {
		t.Fatalf("terminal outcomes collapsed: no_candidate=%+v blocked=%+v", noCandidate, blocked)
	}
}

func TestProjectRevisionInvalidatesEvidenceWithoutChangingTaskIdentity(t *testing.T) {
	contract := testContract(ContractImprovement)
	state, _ := New(contract, testNow)
	state = mustApply(t, contract, state, TransitionRequest{Event: EventDiagnosticCompleted, Reason: "local diagnosis", EvidenceRefs: []string{"obs-rev-1"}})
	state = mustApply(t, contract, state, TransitionRequest{Event: EventProjectRevisionChanged, Reason: "authoritative revision changed", ProjectRevision: "rev-2"})
	if state.State != StateObservationInProgress || state.ProjectRevision != "rev-2" || len(state.EvidenceRefs) != 0 || state.ContractID != contract.ContractID {
		t.Fatalf("revision revalidation failed: %+v", state)
	}
}
