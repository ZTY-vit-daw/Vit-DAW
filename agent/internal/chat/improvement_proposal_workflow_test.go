package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/orchestrationcontroller"
	"vit-daw-agent/internal/pendingmanager"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/shadow"
	"vit-daw-agent/internal/taskstate"
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

func TestAcceptedProposalMigratesStaleObservationRouteInSameTask(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	conversationID, goalID, runID, taskID := "chat-stale-route", "goal-stale-route", "run-stale-route", "task-stale-route"
	assessment := assessFreeStateCapacity(capacityFactsFromProjectState(capacityTestProject(6, 20, 1, 0, "rev-stale-route"), semanticEntryScopeProjectContext), false, time.Now())
	server.storeCapabilityRoute(CapabilityRouteRecord{
		SchemaVersion: capabilityRouteSchema, TaskID: taskID, GoalID: goalID, RunID: runID,
		ConversationID: conversationID, OriginalIntent: "检查一下当前工程有什么问题？",
		SemanticEntry: map[string]any{
			"schema_version": semanticEntryDecisionSchema, "route": semanticEntryRouteObservation,
			"target_scope": semanticEntryScopeProjectContext, "control_mode": semanticEntryControlObserveOnly,
			"user_authorization": semanticEntryAuthorizationObserve, "confidence": 0.95,
		}, Assessment: &assessment, Controller: string(orchestrationcontroller.MinimalAudioClosure),
		ProjectRevision: assessment.ProjectRevision,
	})
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-stale-route", ConversationID: conversationID,
		GoalID: goalID, RunID: runID, Status: "awaiting_experiment", OriginalIntent: "检查一下当前工程有什么问题？",
		ActiveIntent: "bounded bass gain experiment", MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs-stale"}},
	}
	server.storeFreeStateLoop(loop)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema, Target: map[string]any{"kind": "track", "id": "bass"},
		EvidenceRefs: []string{"obs-stale"}, ImprovementIntent: "bounded bass gain experiment",
		Hypothesis: "a bounded gain change produces an auditable candidate", ExpectedEffect: "A/B candidate",
		ActionDomain: agentprotocol.ImprovementActionDomainTrackGain, ActionKind: "gain_adjust",
		ParameterBounds: map[string]any{"delta_db": 0.8}, Confidence: 0.5,
	}
	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: conversationID, GoalID: goalID, RunID: runID, Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal), "request_context": map[string]any{
			"free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")
	if response.Workflow != "mix_tick" || !response.NeedsConfirmation || response.StopReason != "improvement_proposal_native_tool_confirmation_required" {
		t.Fatalf("stale observation route blocked accepted proposal: %+v", response)
	}
	route := server.previousCapabilityRoute(taskID, conversationID)
	if firstStringFromMap(route.SemanticEntry, "route") != semanticEntryRouteOpenSemantic || firstStringFromMap(route.SemanticEntry, "user_authorization") != semanticEntryAuthorizationAction {
		t.Fatalf("stale route was not persisted as the same-task open semantic entry: %+v", route.SemanticEntry)
	}
}

func TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "same-task-project.vit")
	if err := os.WriteFile(projectPath, []byte("same-task-fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	projectShadow := shadow.New(nil)
	projectShadow.Initialize(map[string]any{
		"project_path": projectPath, "project_uuid": "project-same-task", "project_revision": "revision-1", "tracks": []any{},
	})
	server := New(nil, projectShadow, nil)
	server.activateCurrentProjectWorkspace(context.Background())
	conversationID := "chat-same-task-proposal"
	originalIntent := "检查一下当前工程有什么问题？"
	goal := server.harness.EnsureGoal("goal-same-task", "run-same-task", originalIntent)
	contract := taskstate.Contract{
		ConversationID: conversationID, Kind: taskstate.ContractImprovement,
		Scope: taskstate.Scope{Kind: "project", ID: "project-same-task"}, Temporary: true,
		TargetDiscovery: "agent_observation", AuthorizationBoundary: "governed_experiment",
		CompletionCriteria: []string{"governed experiment outcome"}, EvidenceRequirements: []string{"observation reference"},
		ProjectUUID: "project-same-task", ProjectRevision: "revision-1",
	}
	goal, err := server.harness.EnsureTaskContract(goal.GoalID, contract)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventDiagnosticCompleted, Reason: "candidate observed", EvidenceRefs: []string{"obs-bass"}, ProjectRevision: "revision-1",
	}); err != nil {
		t.Fatal(err)
	}
	bounded := taskstate.BoundedProposal{ProposalID: "proposal-bass", Summary: "test a bounded bass gain change", EvidenceRefs: []string{"obs-bass"}, RequiresExperiment: true}
	if _, err = server.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventImprovementProposed, Reason: "proposal admitted", EvidenceRefs: []string{"obs-bass"}, CandidateID: "bass", Proposal: &bounded, ProjectRevision: "revision-1",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = server.transitionTaskSemantic(goal.GoalID, taskstate.TransitionRequest{
		Event: taskstate.EventExperimentRequired, Reason: "experiment admitted", ExperimentID: "experiment-bass", ProjectRevision: "revision-1",
	}); err != nil {
		t.Fatal(err)
	}

	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "bass"}, EvidenceRefs: []string{"obs-bass"},
		ImprovementIntent: "test a bounded bass gain change", Hypothesis: "a small reduction may reduce masking",
		ExpectedEffect: "produce an audible A/B candidate", ActionDomain: agentprotocol.ImprovementActionDomainTrackGain,
		ActionKind: "gain_adjust", ParameterBounds: map[string]any{"delta_db": -1.0}, Confidence: 0.5,
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-same-task", ConversationID: conversationID,
		GoalID: goal.GoalID, RunID: goal.RunID, Status: "awaiting_experiment", OriginalIntent: originalIntent, ActiveIntent: proposal.ImprovementIntent,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs-bass"}},
	}
	server.storeFreeStateLoop(loop)
	server.mu.Lock()
	server.conversationGoals[conversationID] = goal.GoalID
	server.durableContinuations["cont-same-task"] = DurableContinuation{
		ContinuationID: "cont-same-task", TaskID: goal.Task.TaskID, GoalID: goal.GoalID, RunID: goal.RunID,
		ConversationID: conversationID, OriginalIntent: originalIntent, Status: ContinuationCompleted,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	server.mu.Unlock()

	initial := server.improvementProposalResponse(conversationID, agentModeDefault, agentloop.Result{
		TaskID: goal.Task.TaskID, GoalID: goal.GoalID, RunID: goal.RunID, OriginalIntent: originalIntent,
		Status: agentruntime.StatusCompleted, FreeStateDecision: &agentloop.FreeStateDecision{ImprovementProposal: &proposal},
	}, map[string]any{"goal_id": goal.GoalID, "run_id": goal.RunID, "free_state_reasoning_loop": freeStateLoopMap(loop)})
	if len(initial.InteractionRequests) != 1 {
		t.Fatalf("automatic proposal did not create one interaction: %+v", initial)
	}
	initialInteractionID := initial.InteractionRequests[0].ID
	current := server.harness.RuntimeStatus(goal.GoalID)
	if current.Task == nil || current.Task.SemanticState == nil || current.Task.SemanticState.PendingInteraction == nil ||
		current.Task.SemanticState.PendingInteraction.InteractionID != initialInteractionID {
		t.Fatalf("canonical task omitted proposal interaction: %+v", current.Task)
	}
	if durable, ok := server.interactionContinuationForConversation(conversationID); !ok ||
		firstStringFromMap(durable.PendingInteraction, "interaction_id") != initialInteractionID {
		t.Fatalf("durable runtime omitted proposal interaction: %+v ok=%v", durable, ok)
	}
	server.activateCurrentProjectWorkspace(context.Background())
	if pending, ok := server.pendingImprovementProposalForConversation(conversationID); !ok || pending.ID != initialInteractionID {
		t.Fatalf("workspace activation lost proposal interaction: %+v ok=%v", pending, ok)
	}
	if restored := server.harness.RuntimeStatus(goal.GoalID); restored.GoalID != goal.GoalID || restored.Status != agentruntime.StatusWaitingConfirmation {
		t.Fatalf("workspace activation lost waiting goal identity: %+v", restored)
	}

	body, _ := json.Marshal(ChatRequest{ConversationID: conversationID, Message: "确认这个改善提案"})
	recorder := httptest.NewRecorder()
	server.handleChat(recorder, httptest.NewRequest(http.MethodPost, "/agent/chat", bytes.NewReader(body)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response ChatResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.TaskID != goal.Task.TaskID || response.GoalID != goal.GoalID || response.RunID != goal.RunID || response.OriginalIntent != originalIntent {
		t.Fatalf("confirmation changed task identity: %+v", response)
	}
	if response.Workflow != "mix_tick" || len(response.InteractionRequests) != 1 || response.StopReason != "improvement_proposal_native_tool_confirmation_required" {
		t.Fatalf("confirmation did not reach the governed exact-action boundary: %+v", response)
	}
	if got := server.harness.RuntimeSnapshot(); len(got.Goals) != 1 {
		t.Fatalf("confirmation created another semantic goal: %+v", got.Goals)
	}
	nextInteractionID := response.InteractionRequests[0].ID
	current = server.harness.RuntimeStatus(goal.GoalID)
	if current.Task.SemanticState.PendingInteraction == nil || current.Task.SemanticState.PendingInteraction.InteractionID != nextInteractionID || nextInteractionID == initialInteractionID {
		t.Fatalf("exact-action interaction did not replace proposal interaction: %+v", current.Task.SemanticState)
	}
	if durable, ok := server.interactionContinuationForConversation(conversationID); !ok ||
		firstStringFromMap(durable.PendingInteraction, "interaction_id") != nextInteractionID || durable.TaskID != goal.Task.TaskID {
		t.Fatalf("durable exact-action boundary changed identity: %+v ok=%v", durable, ok)
	}
}

// TestAcceptedStaticEQProposalRoutesToMixTickTools locks the D2-1 second
// bounce-point fix: an accepted static_eq proposal must reach the pending mix
// tick (the durable D1-S1 execution chain dispatches from there), instead of
// dying at improvement_proposal_processor_family_missing (observed in the
// 2026-08-27 08:53 real-stack run).
func TestAcceptedStaticEQProposalRoutesToMixTickTools(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vox"}, EvidenceRefs: []string{"obs_vox"},
		ImprovementIntent: "衰减人声中低频堆积", Hypothesis: "300Hz 附近小幅静态 EQ 衰减可减少浑浊感", ExpectedEffect: "可进行 A/B 比较",
		ActionDomain: agentprotocol.ImprovementActionDomainStaticEQ, ActionKind: "static_eq_band_adjust", Confidence: 0.5,
		ParameterBounds: map[string]any{"gain_db": -1.0, "frequency_hz": 300.0, "q": 1.2},
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-staticeq", ConversationID: "chat-staticeq",
		Status: "awaiting_experiment", OriginalIntent: proposal.ImprovementIntent, ActiveIntent: proposal.ImprovementIntent,
		TargetRef: map[string]any{"kind": "track", "id": "vox"}, MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs_vox"}},
	}
	server.storeFreeStateLoop(loop)
	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat-staticeq", GoalID: "goal-staticeq", RunID: "run-staticeq", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal), "request_context": map[string]any{
			"selected_track_id": "vox", "free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")
	if response.Workflow != "mix_tick" || !response.NeedsConfirmation || response.StopReason != "improvement_proposal_native_tool_confirmation_required" {
		t.Fatalf("static_eq proposal did not reach mix tick confirmation: %+v", response)
	}
	candidate, ok := server.pendingMixTickForConversation("chat-staticeq")
	if !ok || candidate.Operation != "static_eq_band_adjust" || candidate.TrackID != "vox" {
		t.Fatalf("static_eq native tool candidate = %+v ok=%v", candidate, ok)
	}
}

func TestAcceptedStaticEQProposalRejectsUnboundedGain(t *testing.T) {
	server := New(nil, shadow.New(nil), nil)
	proposal := agentprotocol.ImprovementProposal{
		SchemaVersion: agentprotocol.ImprovementProposalSchema,
		Target:        map[string]any{"kind": "track", "id": "vox"}, EvidenceRefs: []string{"obs_vox"},
		ImprovementIntent: "衰减人声中低频堆积", Hypothesis: "小幅静态 EQ 衰减", ExpectedEffect: "可进行 A/B 比较",
		ActionDomain: agentprotocol.ImprovementActionDomainStaticEQ, ActionKind: "static_eq_band_adjust", Confidence: 0.5,
		ParameterBounds: map[string]any{"gain_db": 6.0, "frequency_hz": 300.0},
	}
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-staticeq-b", ConversationID: "chat-staticeq-b",
		Status: "awaiting_experiment", OriginalIntent: proposal.ImprovementIntent, ActiveIntent: proposal.ImprovementIntent,
		TargetRef: map[string]any{"kind": "track", "id": "vox"}, MaxCycles: 6,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{"observation_id": "obs_vox"}},
	}
	server.storeFreeStateLoop(loop)
	response := server.continueImprovementProposalInteraction(context.Background(), PendingInteraction{
		ConversationID: "chat-staticeq-b", GoalID: "goal-staticeq-b", RunID: "run-staticeq-b", Workflow: improvementProposalWorkflow,
		Payload: map[string]any{"proposal": agentprotocol.ToMap(proposal), "request_context": map[string]any{
			"selected_track_id": "vox", "free_state_reasoning_loop": freeStateLoopMap(loop),
		}},
	}, "approve")
	if response.StopReason != "improvement_proposal_static_eq_bounds_missing" {
		t.Fatalf("unbounded static_eq gain must fail closed, got stop reason %q", response.StopReason)
	}
}
