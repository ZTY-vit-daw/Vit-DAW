package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestCapabilityRuntimeCanaryRequiresOptInAndCapabilityID(t *testing.T) {
	if capabilityRuntimeCanaryEnabled(map[string]any{"capability_id": "static_mix.static_balance.v0"}) {
		t.Fatal("canary must be opt-in")
	}
	if !capabilityRuntimeCanaryEnabled(map[string]any{"capability_runtime_v1": true}) {
		t.Fatal("explicit canary flag should enable runtime")
	}
	if firstStringFromMap(map[string]any{"capability_id": "static_mix.static_balance.v0"}, "capability_id") != staticBalanceCapabilityID {
		t.Fatal("capability id helper mismatch")
	}
}

func TestCapabilityRuntimeCanaryRoutesB3WithoutLegacyPending(t *testing.T) {
	server := &Server{orchestrationRuntime: orchestrationruntime.New()}
	response, handled := server.handleCapabilityRuntimeCanary(context.Background(), "conversation", ChatRequest{
		Message: "plan pan layout", Context: map[string]any{"capability_runtime_v1": true, "capability_id": panLayoutCapabilityID},
	}, agentruntime.Goal{})
	if !handled || response.Workflow != "capability_runtime_v1" || response.Error == "" {
		t.Fatalf("B3 was not claimed by v1 canary: handled=%v response=%#v", handled, response)
	}
}

func TestCanaryCandidateIndexUsesSpecificOrdinalReferences(t *testing.T) {
	if got := canaryCandidateIndex("按第二个方案", 3); got != 1 {
		t.Fatalf("second candidate index = %d", got)
	}
	if got := canaryCandidateIndex("third", 3); got != 2 {
		t.Fatalf("third candidate index = %d", got)
	}
	if got := canaryCandidateIndex("先看看，不要执行", 3); got != -1 {
		t.Fatalf("non-selection index = %d", got)
	}
}

func TestCapabilityCanaryExecutionResponseDoesNotClaimUserAcceptance(t *testing.T) {
	response := capabilityCanaryExecutionResponse("conversation", agentruntime.Goal{GoalID: "goal", RunID: "run"}, orchestration.PlanningSession{
		ID: "session", Status: orchestration.StatusCompleted,
		Invocation:     orchestration.CapabilityInvocation{CapabilityID: staticBalanceCapabilityID},
		ActiveProposal: &orchestration.Proposal{ID: "proposal-3", Revision: 3, ActionSetHash: "actions-3", ProjectCutHash: "cut-3"},
		Authorization: &orchestration.Authorization{SourceTurnID: "turn-approve", Scope: []string{"track-1"}, Decision: &orchestration.ApprovalDecision{
			SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove,
			ProposalID: "proposal-3", ProposalRevision: 3, ActionSetHash: "actions-3", ProjectCutHash: "cut-3", SourceTurnID: "turn-approve",
		}},
		Execution: &orchestration.ExecutionRecord{
			ID: "execution", Status: "verified", Verification: "pass",
			VerificationResult: &orchestration.VerificationResult{
				Status: "pass", Structural: "pass", Acoustic: "pass", UserAcceptance: "unknown",
				EvidenceRefs: []string{"vsp.state.snapshot:postcondition", "mix.observe:obs_after"},
			},
			Receipts:        []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}},
			PersistenceRefs: []string{"project-history:baseline"},
		},
	}, orchestration.ContextEnvelope{Valid: true}, nil)
	if response.WorkflowData["canary_stage"] != "executed_verified" || response.WorkflowData["user_acceptance"] != "unknown" {
		t.Fatalf("execution response conflated verification and taste: %#v", response)
	}
	if response.WorkflowData["proposal_id"] != "proposal-3" || response.WorkflowData["proposal_revision"] != int64(3) || response.WorkflowData["action_set_hash"] != "actions-3" || response.WorkflowData["project_cut_hash"] != "cut-3" || response.WorkflowData["authorization_source_turn"] != "turn-approve" {
		t.Fatalf("execution response lost proposal/authorization identity: %#v", response.WorkflowData)
	}
	decision, ok := response.WorkflowData["approval_decision"].(orchestration.ApprovalDecision)
	if !ok || decision.Kind != orchestration.ApprovalApprove || decision.ProposalRevision != 3 {
		t.Fatalf("execution response lost typed approval decision: %#v", response.WorkflowData["approval_decision"])
	}
	result, ok := response.WorkflowData["verification_result"].(orchestration.VerificationResult)
	if !ok || result.Structural != "pass" || result.Acoustic != "pass" || result.UserAcceptance != "unknown" || len(result.EvidenceRefs) != 2 {
		t.Fatalf("execution response lost full verification evidence: %#v", response.WorkflowData["verification_result"])
	}
}

func TestCapabilityCanaryExecutionResponseKeepsInconclusiveAppliedExecutionOutOfFailedState(t *testing.T) {
	response := capabilityCanaryExecutionResponse("conversation", agentruntime.Goal{GoalID: "goal", RunID: "run"}, orchestration.PlanningSession{
		ID: "session", Status: orchestration.StatusNeedsReview,
		Execution: &orchestration.ExecutionRecord{
			ID: "execution", Status: "verification_inconclusive", Verification: "inconclusive",
			VerificationResult: &orchestration.VerificationResult{
				Status: "inconclusive", Structural: "pass", Acoustic: "inconclusive", UserAcceptance: "unknown",
			},
			Receipts: []orchestration.ActionReceipt{{ActionID: "a1", Status: "applied"}},
		},
	}, orchestration.ContextEnvelope{Valid: true}, errors.New("verification transport unavailable"))
	if response.GoalStatus != string(agentruntime.StatusWaitingContinue) || response.WorkflowData["canary_stage"] != "executed_needs_review" || response.Error != "" {
		t.Fatalf("needs-review execution was exposed as a failed turn: %#v", response)
	}
	if !strings.Contains(response.Reply, "工程执行没有失败") || !strings.Contains(response.Reply, "待复核") {
		t.Fatalf("needs-review reply did not separate execution from verification: %q", response.Reply)
	}
}
