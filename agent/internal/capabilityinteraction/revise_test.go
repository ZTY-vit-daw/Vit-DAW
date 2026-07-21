package capabilityinteraction

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestReviseFrozenPlanCreatesNewImmutableProposalAndRejectsStaleApproval(t *testing.T) {
	session := interactionTestSession(t)
	oldProposal := *session.ActiveProposal
	oldHash := session.FrozenPlan.ActionSet.Hash
	decision := Resolve("可以，但不要动 bus", "turn-revise", session)
	revised, err := ReviseFrozenPlan(session, decision)
	if err != nil {
		t.Fatal(err)
	}
	if revised.Proposal.ID == oldProposal.ID || revised.Proposal.Revision != oldProposal.Revision+1 || revised.ActionSet.Hash == oldHash {
		t.Fatalf("revision identity did not change: proposal=%#v action_set=%#v", revised.Proposal, revised.ActionSet)
	}
	if len(revised.ActionSet.Actions) != 1 || revised.ActionSet.Actions[0].TargetRef != "track-bgv" || revised.Proposal.Presentation == nil || revised.Proposal.Presentation.ActionCount != 1 {
		t.Fatalf("revision did not remove bus consistently: %#v", revised)
	}
	if session.ActiveProposal.ID != oldProposal.ID || session.FrozenPlan.ActionSet.Hash != oldHash || len(session.FrozenPlan.ActionSet.Actions) != 2 {
		t.Fatal("revision mutated the original frozen session")
	}

	updated, err := session.SetFrozenPlan(revised)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Authorization != nil {
		t.Fatal("revised proposal retained an old authorization")
	}
	staleDecision := Resolve("执行这个方案", "turn-stale", session)
	if _, err := updated.Authorize(orchestration.Authorization{
		ProposalID: oldProposal.ID, ProposalRevision: oldProposal.Revision, ActionSetHash: oldProposal.ActionSetHash,
		ProjectCutHash: oldProposal.ProjectCutHash, SourceTurnID: staleDecision.SourceTurnID, Sequence: 1, Decision: &staleDecision,
	}); err == nil {
		t.Fatal("stale proposal approval authorized the revised plan")
	}
}

func TestReviseFrozenPlanChangesExplicitDeltaAndPresentationTogether(t *testing.T) {
	session := interactionTestSession(t)
	decision := Resolve("把背景和声 +0.50 dB 改成 +0.30 dB", "turn-delta", session)
	revised, err := ReviseFrozenPlan(session, decision)
	if err != nil {
		t.Fatal(err)
	}
	if len(revised.ActionSet.Actions) != 2 || revised.Proposal.Presentation == nil {
		t.Fatalf("unexpected revised plan: %#v", revised)
	}
	var found bool
	for index, action := range revised.ActionSet.Actions {
		if action.TargetRef != "track-bgv" {
			continue
		}
		found = true
		if got := action.Args["delta_db"]; got != 0.3 {
			t.Fatalf("action delta=%v", got)
		}
		if got := revised.Proposal.Presentation.Actions[index].Delta; got != 0.3 {
			t.Fatalf("presentation delta=%v", got)
		}
	}
	if !found {
		t.Fatal("background vocal action not found")
	}
}

func TestCandidateRevisionRequiresFullReplan(t *testing.T) {
	session := interactionTestSession(t)
	decision := Resolve("换第二个候选方案", "turn-candidate", session)
	if !RequiresReplan(decision) {
		t.Fatalf("candidate selection should require full replan: %#v", decision)
	}
	if _, err := ReviseFrozenPlan(session, decision); err != ErrRevisionNeedsClarification {
		t.Fatalf("candidate revision err=%v", err)
	}
}
