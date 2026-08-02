package chat

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/orchestrationruntime"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestB2PublicChatProposalExposesFunctionalHierarchySmoke(t *testing.T) {
	cut := orchestration.ProjectCut{ProjectUUID: "project-smoke", ProjectEpoch: "epoch", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	request := capabilityadapters.StaticBalancePlanRequest{
		SessionID: "b2-public-smoke", Goal: "建立主次并突出主唱", Mode: orchestration.InteractionPropose, ProjectCut: cut,
		Input: capabilitycontext.StaticBalanceInput{
			GeneratedAt: time.Unix(1_700_000_000, 0).UTC(), Style: mixstyle.Default(),
			ProjectState: map[string]any{"tracks": []any{
				map[string]any{"track_id": "vocal", "track_name": "Lead Vocal", "volume_db": 0.0},
				map[string]any{"track_id": "drums", "track_name": "Drums", "volume_db": 0.0},
				map[string]any{"track_id": "bass", "track_name": "Bass", "volume_db": 0.0},
				map[string]any{"track_id": "backing", "track_name": "Backing Vocal", "volume_db": 0.0},
			}},
			TOMProjection: map[string]any{"tom_version": "v0.2", "full_assignment_manifest": map[string]any{"groups": []any{
				map[string]any{"role_hypothesis": "lead_vocal", "confidence": "high", "assignments": []any{map[string]any{"track_id": "vocal", "confidence": "high"}}},
				map[string]any{"role_hypothesis": "drums", "confidence": "high", "assignments": []any{map[string]any{"track_id": "drums", "confidence": "high"}}},
				map[string]any{"role_hypothesis": "bass", "confidence": "high", "assignments": []any{map[string]any{"track_id": "bass", "confidence": "high"}}},
				map[string]any{"role_hypothesis": "backing_vocal", "confidence": "high", "assignments": []any{map[string]any{"track_id": "backing", "confidence": "high"}}},
			}}},
			MOMProjection: map[string]any{
				"mom_version":   "v1.5",
				"trust_quality": map[string]any{"can_support_action_preflight": true},
				"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
					map[string]any{"track_id": "vocal", "level_db": -20.0, "headroom_db": 8.0},
					map[string]any{"track_id": "drums", "level_db": -20.0, "headroom_db": 8.0},
					map[string]any{"track_id": "bass", "level_db": -20.0, "headroom_db": 8.0},
					map[string]any{"track_id": "backing", "level_db": -20.0, "headroom_db": 8.0},
				}},
				"static_level_relationship": map[string]any{
					"schema_version": "mom.static_level_relationship.v1", "status": "ready", "freshness": "fresh", "project_cut_ref": cut.Hash,
					"tracks": []any{
						chatStaticLevel("vocal", -20, -8), chatStaticLevel("drums", -20, -8),
						chatStaticLevel("bass", -20, -8), chatStaticLevel("backing", -20, -8),
					},
				},
			},
		},
	}
	planned, err := capabilityadapters.PlanStaticBalance(request)
	if err != nil || len(planned.CandidateIDs) == 0 {
		t.Fatalf("B2 plan=%#v err=%v", planned.Outcome, err)
	}
	session, err := orchestration.NewSession(request.SessionID, cut.ProjectUUID, request.Goal, orchestration.EngineV1, orchestration.CapabilityInvocation{
		CapabilityID: staticBalanceCapabilityID, CapabilityVer: "v0", InteractionMode: orchestration.InteractionPropose, ProcessingPath: orchestration.PathCapability,
	})
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := capabilityadaptersFreezeStaticBalance(planned, cut, recommendedCanaryCandidate(planned.Pack), session)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SetFrozenPlan(frozen)
	if err != nil {
		t.Fatal(err)
	}
	response := capabilityCanaryProposalResponse("conversation", agentruntime.Goal{}, session, planned, orchestration.ContextEnvelope{Valid: true})
	if response.Workflow != "capability_runtime_v1" || response.WorkflowData["capability_id"] != staticBalanceCapabilityID || !response.NeedsConfirmation {
		t.Fatalf("public B2 response=%#v", response)
	}
	if !strings.Contains(response.Reply, "B2 静态主次与音量平衡方案") {
		t.Fatalf("public reply lost B2 hierarchy semantics: %s", response.Reply)
	}
	functions := map[string]bool{}
	for _, action := range response.ProposalPresentation.Actions {
		functions[action.Function] = true
	}
	if !functions["foreground"] || !functions["support"] {
		t.Fatalf("public proposal omitted foreground/support hierarchy: %#v", response.ProposalPresentation.Actions)
	}
}

func chatStaticLevel(trackID string, rms, peak float64) map[string]any {
	return map[string]any{
		"track_id": trackID, "status": "ready", "freshness": "fresh", "metric": "effective_static_rms_dbfs",
		"tap_point": "derived_static_control_model", "effective_static_rms_dbfs": rms, "effective_static_peak_dbfs": peak,
		"aggregation_method": "duration_weighted_linear_energy_source_plus_clip_gain_plus_fader",
	}
}

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

func TestB2CapabilityRuntimeSchemasDoNotExposeDirectDAD(t *testing.T) {
	entries := capabilityCanaryToolSchemas(nil)
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.ID] = true
	}
	for _, want := range []string{"project.state", "mix.observe", "mix.read", "mix.derive"} {
		if !seen[want] {
			t.Fatalf("B2 runtime schema omitted %s: %+v", want, entries)
		}
	}
	if seen["project.audio_analysis_status"] {
		t.Fatalf("B2 runtime schema exposes direct DAD tool: %+v", entries)
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
