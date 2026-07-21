package capabilityinteraction

import (
	"testing"

	"vit-daw-agent/internal/orchestration"
)

func TestResolveConversationalProposalDecisions(t *testing.T) {
	session := interactionTestSession(t)
	tests := []struct {
		name       string
		message    string
		want       orchestration.ApprovalDecisionKind
		adjustment string
	}{
		{name: "explicit approval", message: "执行这个方案", want: orchestration.ApprovalApprove},
		{name: "explicit proposal approval", message: "确认执行这份 SPAL EQ v2 Proposal", want: orchestration.ApprovalApprove},
		{name: "question is read only", message: "为什么这样调整？", want: orchestration.ApprovalQuestion},
		{name: "qualified approval becomes revision", message: "可以，但不要动 bus", want: orchestration.ApprovalRevise, adjustment: "exclude_targets"},
		{name: "scope narrowing", message: "只处理背景和声", want: orchestration.ApprovalNarrow, adjustment: "include_only"},
		{name: "natural maximum change", message: "最多调整 0.30 dB", want: orchestration.ApprovalRevise, adjustment: "max_abs_delta"},
		{name: "ambiguous text", message: "我再想想", want: orchestration.ApprovalAmbiguous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := Resolve(tt.message, "turn-42", session)
			if decision.Kind != tt.want {
				t.Fatalf("kind=%s want=%s decision=%#v", decision.Kind, tt.want, decision)
			}
			if decision.ProposalID != session.ActiveProposal.ID || decision.ProposalRevision != session.ActiveProposal.Revision || decision.ActionSetHash != session.ActiveProposal.ActionSetHash || decision.ProjectCutHash != session.ActiveProposal.ProjectCutHash {
				t.Fatalf("decision lost exact proposal binding: %#v", decision)
			}
			if tt.want == orchestration.ApprovalQuestion && decision.ExactApprovalFor(*session.ActiveProposal) {
				t.Fatal("question became executable approval")
			}
			if tt.adjustment != "" {
				if len(decision.Adjustments) != 1 || decision.Adjustments[0].Kind != tt.adjustment {
					t.Fatalf("adjustment=%#v want=%s", decision.Adjustments, tt.adjustment)
				}
				if (tt.adjustment == "exclude_targets" || tt.adjustment == "include_only") && len(decision.Adjustments[0].TargetRefs) == 0 {
					t.Fatalf("adjustment=%#v want explicit targets", decision.Adjustments)
				}
			}
		})
	}
}

func TestResolveApprovalCarriesExactTypedBinding(t *testing.T) {
	session := interactionTestSession(t)
	decision := Resolve("确认执行", "turn-approval", session)
	if !decision.ExactApprovalFor(*session.ActiveProposal) {
		t.Fatalf("approval was not exactly bound: %#v", decision)
	}
}

func interactionTestSession(t *testing.T) orchestration.PlanningSession {
	t.Helper()
	const capabilityID = "static_mix.static_balance.v0"
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actions := []orchestration.Action{
		{ID: "gain-bus", Command: "track_gain_adjust", TargetRef: "track-bus", Args: map[string]any{"delta_db": -0.5, "target_db": -1.5}, Compensatable: true},
		{ID: "gain-bgv", Command: "track_gain_adjust", TargetRef: "track-bgv", Args: map[string]any{"delta_db": 0.5, "target_db": -2.5}, Compensatable: true},
	}
	actionSet := orchestration.ActionSet{ID: "action-set-1", CapabilityID: capabilityID, ProjectCutHash: cut.Hash, Actions: actions}
	actionSet.Hash = actionSet.ComputeHash()
	presentation := &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema, ProposalID: "proposal-1", ProposalRevision: 1,
		CapabilityID: capabilityID, Title: "静态平衡方案", ActionCount: len(actions), Reversible: true,
		Actions: []orchestration.ProposalActionPreview{
			{ActionID: "gain-bus", TrackID: "track-bus", TrackName: "Mix Bus", Role: "bus", Function: "bus", Operation: "track_gain_adjust", Before: -1, Target: -1.5, Delta: -0.5, Unit: "dB"},
			{ActionID: "gain-bgv", TrackID: "track-bgv", TrackName: "Background Vocal", Role: "background", Function: "vocal", Operation: "track_gain_adjust", Before: -3, Target: -2.5, Delta: 0.5, Unit: "dB"},
		},
	}
	proposal := orchestration.Proposal{
		ID: "proposal-1", Revision: 1, CapabilityID: capabilityID, ProjectCutHash: cut.Hash,
		ActionSetHash: actionSet.Hash, TargetScope: []string{"track-bus", "track-bgv"}, Presentation: presentation,
	}
	session, err := orchestration.NewSession("session-1", "project-1", "balance", orchestration.EngineV1, orchestration.CapabilityInvocation{CapabilityID: capabilityID})
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetFrozenPlan(orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut})
	if err != nil {
		t.Fatal(err)
	}
	return session
}
