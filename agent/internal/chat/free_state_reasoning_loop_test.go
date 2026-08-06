package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestFreeStateLoopPreservesOriginalCompositeIntentAcrossActionHandoff(t *testing.T) {
	server := New(nil, nil, nil)
	ctx := map[string]any{
		"selected_track_id":   "track-vocal",
		"selected_track_name": "Lead Vocal",
		"goal_id":             "goal-1",
		"run_id":              "run-1",
	}
	prepared, active := server.prepareFreeStateReasoningContext("conversation-1", "make the vocal steadier and more forward", ctx)
	if !active || !freeStateLoopActiveContext(prepared) {
		t.Fatalf("free-state loop did not start: %#v", prepared)
	}
	loop, ok := server.freeStateLoop("conversation-1")
	if !ok || loop.OriginalIntent != "make the vocal steadier and more forward" {
		t.Fatalf("original intent was not retained: %#v", loop)
	}
	if loop.DecisionPhase != freeStatePhaseProcessorSelection {
		t.Fatalf("initial decision phase = %q", loop.DecisionPhase)
	}
	server.recordFreeStateDecision("conversation-1", agentloop.Result{
		GoalID: "goal-1", RunID: "run-1",
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion:   agentloop.FreeStateDecisionSchema,
			Status:          agentloop.FreeStateNeedsAction,
			EvidenceStatus:  "sufficient",
			Summary:         "control macro dynamics first",
			RemainingIntent: "stabilize macro dynamics while retaining the forward target",
			ProcessorType:   "compressor",
		},
	})
	loop, _ = server.freeStateLoop("conversation-1")
	if loop.OriginalIntent != "make the vocal steadier and more forward" ||
		loop.ActiveIntent != "stabilize macro dynamics while retaining the forward target" || loop.Status != "awaiting_action" ||
		loop.DecisionPhase != freeStatePhaseProcessorMaterialization {
		t.Fatalf("action handoff lost loop state: %#v", loop)
	}
}

func TestFreeStateFreshObservationClosesPostActionGateBeforeMaterialization(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "re_evaluating", DecisionPhase: freeStatePhasePostActionEvaluation,
		OriginalIntent: "make the vocal steadier and clearer", ActiveIntent: "make the vocal clearer",
		RequiresPostActionObservation: true, MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	loop, ok := server.recordFreeStateDecision("conversation-1", agentloop.Result{
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "status": "ok", "result": map[string]any{"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-after",
			}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "tonal issue remains", RemainingIntent: "make the vocal clearer", ProcessorType: "eq",
		},
	})
	if !ok || loop.RequiresPostActionObservation || loop.Status != "awaiting_action" || loop.DecisionPhase != freeStatePhaseProcessorMaterialization {
		t.Fatalf("fresh observation did not close the post-action gate: %#v", loop)
	}
}

func TestFreeStateCCBTrackBindingReplacesStaleUITarget(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "observing", OriginalIntent: "stabilize the vocal", ActiveIntent: "stabilize the vocal",
		TargetRef: map[string]any{"kind": "track", "id": "1007", "track_id": "1007", "label": "Bass"},
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	loop, ok := server.recordFreeStateDecision("conversation-1", agentloop.Result{
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "status": "ok", "result": map[string]any{"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-vocal",
				"target_ref": map[string]any{"kind": "track", "id": "1032", "label": "Vocals"},
			}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", RemainingIntent: "stabilize the vocal", ProcessorType: "compressor",
		},
	})
	if !ok || firstStringFromMap(loop.TargetRef, "track_id") != "1032" || firstStringFromMap(loop.TargetRef, "track_name") != "Vocals" {
		t.Fatalf("CCB target did not replace stale UI target: %#v", loop.TargetRef)
	}
	bound := server.bindFreeStateAuthoritativeTrack("conversation-1", map[string]any{
		"selected_track_id": "1007", "selected_track_name": "Bass",
	})
	if firstStringFromMap(bound, "selected_track_id") != "1032" || firstStringFromMap(bound, "selected_track_name") != "Vocals" {
		t.Fatalf("authoritative target was not bound downstream: %#v", bound)
	}
}

func TestFreeStateLoopPersistsInProjectRuntimeState(t *testing.T) {
	server := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion:                 freeStateReasoningLoopSchema,
		LoopID:                        "free-state-1",
		ConversationID:                "conversation-1",
		GoalID:                        "goal-1",
		RunID:                         "run-1",
		Status:                        "re_evaluating",
		OriginalIntent:                "make the vocal steadier and more forward",
		ActiveIntent:                  "make it more forward",
		Cycle:                         1,
		MaxCycles:                     6,
		RequiresPostActionObservation: true,
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ok",
			Summary: map[string]any{"observation_id": "obs-1", "status": "ready"}},
		Actions: []freeStateActionRecord{{Cycle: 1, ProcessorType: "compressor", Workflow: semanticCompressorExecutionWorkflow,
			Status: "applied", RecordedAt: time.Now().UTC()}},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	server.storeFreeStateLoop(loop)
	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()

	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	restarted.mu.Unlock()
	restored, ok := restarted.freeStateLoop("conversation-1")
	if !ok || !restored.RequiresPostActionObservation || restored.Cycle != 1 || len(restored.Actions) != 1 ||
		restored.LatestObservation == nil || firstStringFromMap(restored.LatestObservation.Summary, "observation_id") != "obs-1" {
		t.Fatalf("persisted loop was not restored: %#v", restored)
	}
}

func TestFreeStateMaterializationRetryRetainsOriginalIntentAndCCBEvidence(t *testing.T) {
	originalIntent := "make the vocal steadier and more forward"
	loop := freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		Status:         "awaiting_action",
		OriginalIntent: originalIntent,
		ActiveIntent:   "stabilize the vocal while preserving transients",
		GoalID:         "goal-1",
		RunID:          "run-1",
		LatestDecision: &agentloop.FreeStateDecision{
			SchemaVersion:   agentloop.FreeStateDecisionSchema,
			Status:          agentloop.FreeStateNeedsAction,
			EvidenceStatus:  "sufficient",
			Summary:         "macro dynamics support conservative compression",
			RemainingIntent: "stabilize the vocal while preserving transients",
			ProcessorType:   "compressor",
		},
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", Status: "ok",
			Summary: map[string]any{"observation_id": "obs-1", "status": "ready"}},
	}
	userText, res, ok := freeStateMaterializationRetry(loop)
	if !ok || userText != loop.ActiveIntent || res.GoalSummary != originalIntent || res.FreeStateDecision == nil {
		t.Fatalf("retry state = text=%q result=%+v ok=%v", userText, res, ok)
	}
	if res.RecentObservation == nil || firstStringFromMap(res.RecentObservation.Summary, "observation_id") != "obs-1" {
		t.Fatalf("retry discarded latest CCB evidence: %+v", res.RecentObservation)
	}
}

func TestFreeStateMaterializationTransientErrorIsResumableNotFailed(t *testing.T) {
	server := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		GoalID: "goal-1", RunID: "run-1", Status: "awaiting_action", OriginalIntent: "make the vocal steadier",
		ActiveIntent: "make the vocal steadier", LatestDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "compression is supported", RemainingIntent: "make the vocal steadier", ProcessorType: "compressor",
		}, MaxCycles: 6, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	server.storeFreeStateLoop(loop)
	ctx := map[string]any{"free_state_route_authorized": true, "free_state_reasoning_loop": freeStateLoopMap(loop)}
	response := server.makeFreeStateMaterializationResumable("conversation-1", ctx, agentloop.Result{GoalID: "goal-1", RunID: "run-1"}, ChatResponse{
		ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		GoalStatus: string(agentruntime.StatusFailed), Error: "LLM HTTP error 502: error code: 502",
		Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"status": "unavailable"},
	})
	if response.GoalStatus != string(agentruntime.StatusWaitingContinue) || response.StopReason != agentloop.StopReasonTransientLLMError || response.Error != "" {
		t.Fatalf("materialization response was not resumable: %+v", response)
	}
	if firstStringFromMap(response.WorkflowData, "status") != "paused_transient" || response.WorkflowData["resumable"] != true {
		t.Fatalf("resumable workflow data missing: %+v", response.WorkflowData)
	}
}

func TestFreeStateMaterializationRuntimeConfigurationServiceErrorIsTransient(t *testing.T) {
	if !freeStateTransientServiceError("\u8fd0\u884c\u65f6\u914d\u7f6e\u72b6\u6001\u4e0d\u53ef\u7528\uff0c\u8bf7\u7a0d\u540e\u518d\u8bd5") {
		t.Fatal("localized runtime configuration service error was not classified as transient")
	}
	if freeStateTransientServiceError("AI configuration is incomplete") {
		t.Fatal("a persistent incomplete local configuration was classified as transient")
	}
}

func TestFreeStateContextIsCarriedIntoEQConfirmationInteraction(t *testing.T) {
	server := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "awaiting_action", OriginalIntent: "make the vocal more forward", ActiveIntent: "make the vocal more forward",
		MaxCycles: 6, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	server.storeFreeStateLoop(loop)
	resp := server.bindFreeStateContextToResponse(ChatResponse{
		ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1", PlanID: "proposal-1",
		Workflow: "capability_runtime_v1", WorkflowData: map[string]any{
			"capability_id": agentSemanticEQCapabilityID,
		},
	}, map[string]any{"selected_track_id": "track-vocal"})
	req := server.confirmationInteractionRequest(resp)
	stored, ok := server.takePendingInteraction(req.ID)
	if !ok || !freeStateLoopActiveContext(stored.RequestContext) {
		t.Fatalf("EQ confirmation lost free-state request context: %#v", stored.RequestContext)
	}
}

func TestFreeStateAcousticActionOutcomeRecognizesGovernedReceipts(t *testing.T) {
	_, status, _, attempted := freeStateAcousticActionOutcome(PendingInteraction{}, ChatResponse{
		Workflow:     semanticCompressorExecutionWorkflow,
		WorkflowData: map[string]any{"status": "executed", "execution_receipt": map[string]any{"status": "executed"}},
	})
	if !attempted || status != "applied" {
		t.Fatalf("compressor receipt status=%q attempted=%v", status, attempted)
	}
	processor, status, receipt, attempted := freeStateAcousticActionOutcome(PendingInteraction{}, ChatResponse{
		Workflow: "capability_runtime_v1", GoalStatus: string(agentruntime.StatusCompleted),
		WorkflowData: map[string]any{"capability_id": agentSemanticEQCapabilityID,
			"receipts":     []map[string]any{{"status": "applied", "details": map[string]any{"status": "exact"}}},
			"verification": map[string]any{"status": "pass", "structural": "pass", "acoustic": "unavailable"}},
	})
	if !attempted || processor != "eq" || status != "applied" {
		t.Fatalf("EQ receipt processor=%q status=%q attempted=%v", processor, status, attempted)
	}
	if verification := firstMapFromAny(receipt["verification"]); firstStringFromMap(verification, "acoustic") != "unavailable" {
		t.Fatalf("EQ verification was not retained in free-state receipt: %#v", receipt)
	}
}

func TestFreeStateMaterializationBoundariesBlockLoopWithoutRecordingAnAction(t *testing.T) {
	tests := []ChatResponse{
		{Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"status": "capability_boundary"}, StopReason: "semantic_treatment_capability_boundary"},
		{Workflow: semanticTreatmentWorkflow, WorkflowData: map[string]any{"status": "qualification_failed"}, StopReason: "semantic_eq_post_load_not_qualified"},
		{Workflow: semanticCompressorExecutionWorkflow, WorkflowData: map[string]any{
			"status": "materialization_rejected", "execution": map[string]any{"message": "processor topology changed after planning"},
		}, StopReason: "execution_materialization_rejected"},
		{Workflow: "capability_runtime_v1", WorkflowData: map[string]any{
			"status": "rejected", "stage": "preflight_rejected", "writes_performed": 0,
		}, Error: "EQ topology is stale"},
	}
	for _, response := range tests {
		t.Run(response.Workflow+"_"+firstStringFromMap(response.WorkflowData, "status"), func(t *testing.T) {
			server := New(nil, nil, nil)
			now := time.Now().UTC()
			server.storeFreeStateLoop(freeStateReasoningLoop{
				SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
				Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
				OriginalIntent: "make the vocal steadier and more forward", ActiveIntent: "bring the vocal forward",
				MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
			})
			response.ConversationID = "conversation-1"
			response.GoalStatus = string(agentruntime.StatusCompleted)
			resp := server.bindFreeStateContextToResponse(response, nil)
			loop, ok := server.freeStateLoop("conversation-1")
			if !ok || loop.Status != "blocked" || loop.Cycle != 0 || len(loop.Actions) != 0 {
				t.Fatalf("materialization boundary did not close the loop cleanly: %#v", loop)
			}
			if loop.LatestDecision == nil || loop.LatestDecision.Status != agentloop.FreeStateBlocked {
				t.Fatalf("materialization boundary did not record a blocked decision: %#v", loop.LatestDecision)
			}
			bound := firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"])
			if firstStringFromMap(bound, "status") != "blocked" {
				t.Fatalf("response exposed stale free-state status: %#v", resp.WorkflowData)
			}
		})
	}
}

func TestFreeStateCOMEvidenceRejectionRemainsResumable(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "stabilize and clarify the vocal", ActiveIntent: "stabilize the vocal",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	resp := server.maybeContinueFreeStateAfterInteraction(nil, PendingInteraction{ConversationID: "conversation-1"}, ChatResponse{
		ConversationID: "conversation-1", Workflow: semanticCompressorExecutionWorkflow,
		WorkflowData: map[string]any{"status": "materialization_rejected", "execution": map[string]any{
			"message": "semantic execution requires paired_io / ready COM evidence",
		}}, GoalStatus: string(agentruntime.StatusCompleted), StopReason: "execution_materialization_rejected",
	}, "approve")
	loop, ok := server.freeStateLoop("conversation-1")
	if !ok || loop.Status != "awaiting_action" || loop.Cycle != 0 || len(loop.Actions) != 0 {
		t.Fatalf("COM evidence rejection did not preserve the pending action: %#v", loop)
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) || resp.StopReason != "free_state_com_evidence_refresh_required" || !boolValue(resp.WorkflowData["resumable"]) {
		t.Fatalf("COM evidence rejection was not exposed as resumable: %#v", resp)
	}
}

func TestFreeStateCCBObservationFeedsDownstreamProcessorPlanning(t *testing.T) {
	observation := freeStateCCBObservation(agentloop.Result{Executed: []map[string]any{{
		"tool": "ccb.observation_request", "tool_call_id": "ccb-1", "status": "ok",
		"result": map[string]any{"bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-1",
			"views": map[string]any{"track.time_dynamics": map[string]any{"status": "ready"}},
		}},
	}}})
	if observation == nil || observation.Tool != "ccb.observation_request" || firstStringFromMap(observation.Summary, "observation_id") != "obs-1" {
		t.Fatalf("CCB observation was not bridged: %#v", observation)
	}
}

func TestFreeStateStartExcludesExactCompressorParameterControl(t *testing.T) {
	ctx := map[string]any{"selected_track_id": "track-vocal", "selected_plugin_id": "compressor-1"}
	if shouldStartFreeStateReasoningLoop("set threshold to -12 dB and ratio to 4:1", ctx) {
		t.Fatal("exact parameter control entered the open semantic loop")
	}
}

func TestFreeStateContinueIsNotConsumedByMissingMixTickCandidate(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "re_evaluating", DecisionPhase: freeStatePhasePostActionEvaluation,
		OriginalIntent: "make the vocal steadier", ActiveIntent: "make the vocal steadier",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})

	resp, handled := server.handlePendingMixTickChat(nil, "conversation-1", ChatRequest{Message: "continue"}, agentModeDefault)
	if handled || resp.StopReason != "" {
		t.Fatalf("free-state continuation was consumed by mix tick: handled=%v response=%+v", handled, resp)
	}
}

func TestBindFreeStateContextRecoversPersistedLoopFromRequest(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "make the vocal steadier and more forward", ActiveIntent: "stabilize the vocal",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	}
	resp := server.bindFreeStateContextToResponse(ChatResponse{ConversationID: "conversation-1"}, map[string]any{
		"free_state_reasoning_loop": freeStateLoopMap(loop),
	})
	bound := firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"])
	if firstStringFromMap(bound, "original_intent") != loop.OriginalIntent {
		t.Fatalf("response lost recovered original intent: %#v", resp.WorkflowData)
	}
	if stored, ok := server.freeStateLoop("conversation-1"); !ok || stored.OriginalIntent != loop.OriginalIntent {
		t.Fatalf("recovered loop was not restored to server state: %#v ok=%v", stored, ok)
	}
}

func TestPostLoadPlanningTransientFailureBecomesResumable(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-1", ConversationID: "conversation-1",
		GoalID: "goal-1", RunID: "run-1", Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "make the vocal steadier", ActiveIntent: "make the vocal steadier", MaxCycles: 6,
		LatestDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "compressor supported", RemainingIntent: "make the vocal steadier", ProcessorType: "compressor",
		},
		CreatedAt: now, UpdatedAt: now,
	}
	server.storeFreeStateLoop(loop)
	plan := PendingPlan{Context: map[string]any{
		"conversation_id": "conversation-1", "goal_id": "goal-1", "run_id": "run-1",
		"free_state_reasoning_loop": freeStateLoopMap(loop),
	}}
	resp := server.finalizeSemanticProcessorPostLoadHandoff(plan, ChatResponse{
		ConversationID: "conversation-1", GoalID: "goal-1", RunID: "run-1",
		Workflow: "semantic_compressor_planning", WorkflowData: map[string]any{"status": "unavailable"},
		GoalStatus: string(agentruntime.StatusFailed), Error: "context deadline exceeded while awaiting headers",
	})
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) || resp.StopReason != agentloop.StopReasonTransientLLMError || resp.Error != "" {
		t.Fatalf("post-load transient response = %+v", resp)
	}
	if firstStringFromMap(resp.WorkflowData, "status") != "paused_transient" ||
		firstStringFromMap(firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"]), "original_intent") != loop.OriginalIntent {
		t.Fatalf("post-load transient response lost loop: %+v", resp.WorkflowData)
	}
}
