package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/panlayout"
	"vit-daw-agent/internal/pendingmanager"
)

func TestLegacyCapabilityRestoreRetiresAuthorityWithoutExecutableInteraction(t *testing.T) {
	b2 := testStaticBalancePlan()
	b3 := testPanLayoutPlan()
	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		SavedAt: time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC),
		ConversationMemory: map[string]agentloop.ExecutionMemory{
			"legacy-b2": {PendingStaticBalancePlan: &b2},
			"legacy-b3": {PendingPanLayoutPlan: &b3},
		},
		PendingStaticBalancePlans: map[string]agentloop.PendingStaticBalancePlan{"legacy-b2": b2},
		PendingPanLayoutPlans:     map[string]agentloop.PendingPanLayoutPlan{"legacy-b3": b3},
		Interactions: map[string]PendingInteraction{
			"b2-confirm": {Workflow: "static_balance", Kind: "static_balance_confirmation"},
			"b3-confirm": {Workflow: "pan_layout", Kind: "pan_layout_confirmation"},
		},
	})

	if len(server.conversationMemory) != 0 {
		t.Fatalf("legacy execution memory survived restore: %#v", server.conversationMemory)
	}
	if len(server.interactions) != 0 {
		t.Fatalf("legacy confirmation interaction survived restore: %#v", server.interactions)
	}
	if active := server.pendingManager.ActiveForConversation("legacy-b2"); len(active) != 0 {
		t.Fatalf("retired B2 candidate remained active: %#v", active)
	}
	if active := server.pendingManager.ActiveForConversation("legacy-b3"); len(active) != 0 {
		t.Fatalf("retired B3 candidate remained active: %#v", active)
	}
	retired := server.pendingManager.Snapshot()
	if len(retired) != 2 {
		t.Fatalf("legacy audit records=%d, want 2: %#v", len(retired), retired)
	}
	for _, candidate := range retired {
		if candidate.Status != agentprotocol.PendingStatusRejected || candidate.Source.Metadata["transition_reason"] != legacyCapabilityRetirementReason {
			t.Fatalf("legacy candidate was not terminalized: %#v", candidate)
		}
	}
	written := server.projectAgentRuntimeStateLocked()
	if len(written.PendingStaticBalancePlans) != 0 || len(written.PendingPanLayoutPlans) != 0 {
		t.Fatalf("runtime rewrote legacy authority: %#v", written)
	}
}

func TestLegacyCapabilityRestorePreservesV1Interaction(t *testing.T) {
	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		Interactions: map[string]PendingInteraction{
			"v1": {Workflow: "capability_runtime_v1", Kind: "confirmation"},
		},
	})
	if _, ok := server.interactions["v1"]; !ok {
		t.Fatal("v1 confirmation interaction was retired with legacy state")
	}
}

func TestLegacyCapabilityRestorePreservesVerifiedAuditTruth(t *testing.T) {
	b2 := testStaticBalancePlan()
	b3 := testPanLayoutPlan()
	verifiedB2 := b2.ToPendingCandidate("legacy-verified-b2", "goal-b2", "run-b2", "")
	verifiedB2.Status = agentprotocol.PendingStatusVerified
	verifiedB2.Source.Metadata["transition_reason"] = "project.state fader verification 2/2"
	verifiedB3 := b3.ToPendingCandidate("legacy-verified-b3", "goal-b3", "run-b3", "")
	verifiedB3.Status = agentprotocol.PendingStatusVerified
	verifiedB3.Source.Metadata["transition_reason"] = "project.state pan verification 2/2"

	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		SavedAt: time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
		ConversationMemory: map[string]agentloop.ExecutionMemory{
			"legacy-verified-b2": {PendingStaticBalancePlan: &b2},
			"legacy-verified-b3": {PendingPanLayoutPlan: &b3},
		},
		PendingStaticBalancePlans: map[string]agentloop.PendingStaticBalancePlan{"legacy-verified-b2": b2},
		PendingPanLayoutPlans:     map[string]agentloop.PendingPanLayoutPlan{"legacy-verified-b3": b3},
		PendingCandidates:         []agentprotocol.PendingCandidate{verifiedB2, verifiedB3},
	})

	byType := map[string]agentprotocol.PendingCandidate{}
	for _, candidate := range server.pendingManager.Snapshot() {
		byType[candidate.CandidateType] = candidate
	}
	for candidateType, reason := range map[string]string{
		"static_balance_plan": "project.state fader verification 2/2",
		"pan_layout_plan":     "project.state pan verification 2/2",
	} {
		candidate, ok := byType[candidateType]
		if !ok || candidate.Status != agentprotocol.PendingStatusVerified || candidate.Source.Metadata["transition_reason"] != reason {
			t.Fatalf("verified legacy audit truth was rewritten: type=%s candidate=%#v", candidateType, candidate)
		}
	}
	if len(server.conversationMemory) != 0 {
		t.Fatalf("verified audit preservation retained executable memory: %#v", server.conversationMemory)
	}
}

func TestLegacyCapabilityRestoreRetiresMemoryOnlyPending(t *testing.T) {
	b2 := testStaticBalancePlan()
	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.restoreProjectAgentRuntimeStateLocked(projectAgentRuntimeState{
		ConversationMemory: map[string]agentloop.ExecutionMemory{
			"memory-only": {PendingStaticBalancePlan: &b2},
		},
	})
	retired := server.pendingManager.Snapshot()
	if len(retired) != 1 || retired[0].Status != agentprotocol.PendingStatusRejected {
		t.Fatalf("memory-only legacy authority lost its retirement audit: %#v", retired)
	}
	if len(server.conversationMemory) != 0 {
		t.Fatalf("memory-only legacy authority remained executable: %#v", server.conversationMemory)
	}
}

func TestLegacyCapabilityTerminalStatusClassification(t *testing.T) {
	for _, status := range []string{
		agentprotocol.PendingStatusRejected,
		agentprotocol.PendingStatusCommitted,
		agentprotocol.PendingStatusVerified,
		agentprotocol.PendingStatusVerificationFailed,
		agentprotocol.PendingStatusFailed,
		agentprotocol.PendingStatusBlocked,
	} {
		if !legacyCapabilityCandidateTerminal(status) {
			t.Fatalf("terminal legacy status %q was classified active", status)
		}
	}
	for _, status := range []string{
		agentprotocol.PendingStatusProposed,
		agentprotocol.PendingStatusWaitingUser,
		agentprotocol.PendingStatusAccepted,
		agentprotocol.PendingStatusRevisionRequested,
		agentprotocol.PendingStatusAwaitingApproval,
		agentprotocol.PendingStatusReadyToCommit,
		agentprotocol.PendingStatusCommitting,
	} {
		if legacyCapabilityCandidateTerminal(status) {
			t.Fatalf("active legacy status %q was classified terminal", status)
		}
	}
}

func testStaticBalancePlan() agentloop.PendingStaticBalancePlan {
	return agentloop.PendingStaticBalancePlan{
		SchemaVersion: "static_balance_pending.v0", PlanID: "b2_test", CapabilityID: "static_mix.static_balance.v0", ContextPackID: "cap_test",
		StyleID: "neutral", StyleName: "Neutral", ObservationID: "obs_b2", Status: "pending_confirmation", Confidence: "high", TrackCount: 2, ExpiresAfterContextChange: true,
		Actions: []agentloop.PendingStaticBalanceAction{
			{Operation: "track_gain_adjust", TrackID: "v", TrackName: "Vocal", Role: "lead_vocal", Function: "foreground", DeltaDB: 1, BeforeDB: 0, TargetDB: 1},
			{Operation: "track_gain_adjust", TrackID: "s", TrackName: "Strings", Role: "strings", Function: "harmonic_bed", DeltaDB: -1, BeforeDB: 0, TargetDB: -1},
		},
	}
}

func testPanLayoutPlan() agentloop.PendingPanLayoutPlan {
	plan := agentloop.PendingPanLayoutPlan{
		SchemaVersion: "pan_layout_pending.v1", PlanID: "b3_test", CapabilityID: panlayout.CapabilityID,
		ContextPackID: "pan_pack_test", CandidatePlanID: "pan_candidate_test",
		StyleID: "neutral", StyleName: "Neutral", StyleHash: "style_hash_test",
		ObservationID: "obs_b3", Status: "pending_confirmation", Confidence: "high",
		TrackCount: 2, DisclosedTrackCount: 2, ExpiresAfterContextChange: true,
	}
	plan.Actions = []agentloop.PendingPanLayoutAction{
		{Operation: "track_pan_set", TrackID: "g1", TrackName: "Guitar L", Role: "guitar", Function: "support", BeforePan: 0, TargetPan: -.5, DeltaPan: -.5, Reason: "complementary pair"},
		{Operation: "track_pan_set", TrackID: "g2", TrackName: "Guitar R", Role: "guitar", Function: "support", BeforePan: 0, TargetPan: .5, DeltaPan: .5, Reason: "complementary pair"},
	}
	for index := range plan.Actions {
		plan.Actions[index].Fingerprint = map[string]any{"track_pan": 0.0, "context_pack_id": plan.ContextPackID, "candidate_plan_id": plan.CandidatePlanID, "style_hash": plan.StyleHash}
	}
	return plan
}
