package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/pendingmanager"
	"vit-daw-agent/internal/shadow"
)

func TestImprovementProposalConfirmationFailsClosedWithoutObservationContext(t *testing.T) {
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion:     agentprotocol.ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "vox"},
		EvidenceRefs:      []string{"obs_1"},
		ImprovementIntent: "让人声更靠前",
		Hypothesis:        "小幅受控处理可能改善前后感",
		ExpectedEffect:    "人声前后层次更清晰",
		ActionDomain:      agentprotocol.ImprovementActionDomainEQ,
		ActionKind:        "bounded_tonal_adjustment",
		Confidence:        0.6,
	}
	candidate := proposal.ToPendingCandidate("chat_1", "goal_1", "run_1", "now")
	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.upsertPendingCandidate(candidate)
	resp := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat_1", GoalID: "goal_1", RunID: "run_1",
		Workflow: improvementProposalWorkflow,
		Payload:  map[string]any{"typed_state": agentprotocol.ToMap(candidate)},
	}, "确认提案")
	if resp.GoalStatus != "completed" || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("confirmation crossed mutation boundary: %+v", resp)
	}
	accepted, ok := server.pendingManager.Get(candidate.ID)
	if !ok || accepted.Status != agentprotocol.PendingStatusAccepted {
		t.Fatalf("candidate was not accepted: %+v ok=%v", accepted, ok)
	}
}

func TestImprovementProposalResolutionNotesDoNotCreateSecondConfirmationGate(t *testing.T) {
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion:     agentprotocol.ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "bass"},
		EvidenceRefs:      []string{"obs_1"},
		ImprovementIntent: "改善低频清晰度",
		Hypothesis:        "小幅处理可能减少低频遮蔽",
		ExpectedEffect:    "低频关系更容易分辨",
		ActionDomain:      agentprotocol.ImprovementActionDomainEQ,
		ActionKind:        "high_pass_filter",
		Confidence:        0.3,
		NeedsResolution:   []string{"确认鼓组低频能量后再决定处理对象"},
	}
	candidate := proposal.ToPendingCandidate("chat_1", "goal_1", "run_1", "now")
	server := &Server{pendingManager: pendingmanager.NewMemoryManager()}
	server.upsertPendingCandidate(candidate)
	resp := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat_1", GoalID: "goal_1", RunID: "run_1", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal)},
	}, "确认提案")
	if resp.GoalStatus == "waiting_clarification" || resp.StopReason != "improvement_proposal_observation_context_missing" || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("resolution note became a user-clarification gate: %+v", resp)
	}
}

func TestImprovementProposalInteractionPersistsRequestContext(t *testing.T) {
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema, Target: map[string]any{"kind": "track", "id": "vox"}, EvidenceRefs: []string{"obs_1"},
		ImprovementIntent: "让人声更靠前", Hypothesis: "小幅处理可能改善前后感", ExpectedEffect: "人声更清晰",
		ActionDomain: agentprotocol.ImprovementActionDomainEQ, ActionKind: "bounded_tonal_adjustment", Confidence: 0.6,
	}
	req := improvementProposalInteractionRequest("chat_1", "goal_1", "run_1", "", proposal.ToPendingCandidate("chat_1", "goal_1", "run_1", "now"), map[string]any{
		"selected_track_id": "vox", "free_state_reasoning_loop": map[string]any{"loop_id": "free_state_1"},
	})
	ctx := firstMapFromAny(req.Payload["request_context"])
	if ctx["selected_track_id"] != "vox" || len(firstMapFromAny(ctx["free_state_reasoning_loop"])) == 0 {
		t.Fatalf("interaction lost router context: %+v", req.Payload)
	}
}

func TestImprovementProposalRoundTripRoutesP01LikeResolutionNotes(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion:     agentprotocol.ImprovementProposalSchema,
		Target:            map[string]any{"kind": "track", "id": "bass"},
		EvidenceRefs:      []string{"obs_bass", "obs_relationship"},
		ImprovementIntent: "改善低频清晰度",
		Hypothesis:        "保守 EQ A/B 可能减少低频重叠",
		ExpectedEffect:    "在同响度比较中确认是否改善",
		ActionDomain:      agentprotocol.ImprovementActionDomainEQ,
		ActionKind:        "high_pass_filter",
		Confidence:        0.3,
		NeedsResolution:   []string{"确认另一轨的低频能量后再选择处理对象"},
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-p01", ConversationID: "chat-p01",
		Status: "awaiting_experiment", OriginalIntent: proposal.ImprovementIntent, ActiveIntent: proposal.ImprovementIntent,
		TargetRef: map[string]any{"kind": "track", "id": "bass"}, MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs_bass"}},
	}
	server.storeFreeStateLoop(loop)
	res := agentloop.Result{GoalID: "goal-p01", RunID: "run-p01", FreeStateDecision: &agentloop.FreeStateDecision{ImprovementProposal: &proposal}}
	response := server.improvementProposalResponse("chat-p01", agentModeDefault, res, map[string]any{
		"selected_track_id": "bass", "free_state_reasoning_loop": freeStateLoopMap(loop),
	})
	if len(response.InteractionRequests) != 1 {
		t.Fatalf("expected proposal interaction: %+v", response)
	}
	interaction, ok := server.takePendingInteraction(response.InteractionRequests[0].ID)
	if !ok || len(firstMapFromAny(interaction.RequestContext["free_state_reasoning_loop"])) == 0 {
		t.Fatalf("confirmation lost free-state context: %+v ok=%v", interaction, ok)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	confirmed := server.continueImprovementProposalInteraction(ctx, interaction, "approve")
	if confirmed.GoalStatus == "waiting_clarification" || confirmed.WorkflowData["mutation_performed"] != false || confirmed.WorkflowData["action_domain_router"] != true || confirmed.WorkflowData["proposal_confirmation"] != "accepted" {
		t.Fatalf("p01-like resolution notes did not reach the governed router: %+v", confirmed)
	}
}

func TestImprovementProposalProcessorTypeMapsTypedDynamicDomains(t *testing.T) {
	for domain, want := range map[string]string{
		agentprotocol.ImprovementActionDomainEQ:                "eq",
		agentprotocol.ImprovementActionDomainCompressor:        "compressor",
		agentprotocol.ImprovementActionDomainLimiter:           "limiter",
		agentprotocol.ImprovementActionDomainGateExpander:      "gate_expander",
		agentprotocol.ImprovementActionDomainDeEsser:           "de_esser",
		agentprotocol.ImprovementActionDomainTransientShaper:   "transient_shaper",
		agentprotocol.ImprovementActionDomainMultibandDynamics: "multiband_dynamics",
	} {
		if got := improvementProposalProcessorType(agentprotocol.ImprovementProposal{ActionDomain: domain}); got != want {
			t.Fatalf("domain %s maps to %q, want %q", domain, got, want)
		}
	}
}

func TestAcceptedTrackGainProposalRoutesToMixTickTools(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "bass"}, EvidenceRefs: []string{"obs_bass"},
		ImprovementIntent: "改善低频存在感", Hypothesis: "小幅提升轨道电平可能改善存在感", ExpectedEffect: "可进行 A/B 比较",
		ActionDomain: agentprotocol.ImprovementActionDomainTrackGain, ActionKind: "gain_adjust", Confidence: 0.5,
		ParameterBounds: map[string]any{"delta_db": 0.8},
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-native", ConversationID: "chat-native",
		Status: "awaiting_experiment", OriginalIntent: proposal.ImprovementIntent, ActiveIntent: proposal.ImprovementIntent,
		TargetRef: map[string]any{"kind": "track", "id": "bass"}, MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs_bass"}},
	}
	server.storeFreeStateLoop(loop)
	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat-native", GoalID: "goal-native", RunID: "run-native", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal), "request_context": map[string]any{
			"selected_track_id": "bass", "free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")
	if response.Workflow != "mix_tick" || !response.NeedsConfirmation || response.StopReason != "improvement_proposal_native_tool_confirmation_required" {
		t.Fatalf("track gain proposal did not reach mix tick confirmation: %+v", response)
	}
	if response.WorkflowData["mutation_performed"] != false || response.WorkflowData["proposal_execution_authority"] != "mix_tick_tools" {
		t.Fatalf("native route crossed the mutation boundary: %+v", response.WorkflowData)
	}
	candidate, ok := server.pendingMixTickForConversation("chat-native")
	if !ok || candidate.Operation != "track_gain_adjust" || candidate.TrackID != "bass" || candidate.DeltaDB != 0.8 {
		t.Fatalf("native tool candidate = %+v ok=%v", candidate, ok)
	}
}
