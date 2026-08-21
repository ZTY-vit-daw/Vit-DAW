package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/processorintent"
	agentruntime "vit-daw-agent/internal/runtime"
)

func TestFreeStateTargetRefDoesNotPersistPluginIdentityBeforeFamilySelection(t *testing.T) {
	target := freeStateTargetRef(map[string]any{
		"selected_plugin_track_id": "track-vocal",
		"selected_plugin_id":       "plugin-comp-1",
		"selected_plugin_name":     "FabFilter Pro-C 2",
		"selected_track_name":      "Lead Vocal",
	})
	if firstStringFromMap(target, "kind") != "track" || firstStringFromMap(target, "id") != "track-vocal" {
		t.Fatalf("target binding=%#v", target)
	}
	for _, key := range []string{"plugin_id", "plugin_name", "selected_plugin_id", "selected_plugin_name"} {
		if _, ok := target[key]; ok {
			t.Fatalf("target persisted plugin identity key %q: %#v", key, target)
		}
	}
}

func TestFreeStateLoopPreservesOriginalCompositeIntentAcrossActionHandoff(t *testing.T) {
	server := New(nil, nil, nil)
	ctx := map[string]any{
		"selected_track_id":   "track-vocal",
		"selected_track_name": "Lead Vocal",
		"goal_id":             "goal-1",
		"run_id":              "run-1",
	}
	ctx = contextWithSemanticEntryDecision(ctx, semanticEntryDecision{
		SchemaVersion: semanticEntryDecisionSchema, Route: semanticEntryRouteOpenSemantic,
		Controller:  "minimal_audio_closure",
		TargetScope: semanticEntryScopeCurrentSelection, ControlMode: semanticEntryControlSemanticLoop,
		UserAuthorization: semanticEntryAuthorizationAction, Confidence: 0.94, Reason: "open acoustic outcome",
	})
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

func TestRecordFreeStateDecisionPersistsExecutedCCBReceiptsAndAvailableViews(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-ledger", ConversationID: "conversation-ledger",
		Status: "reasoning", OriginalIntent: "inspect the full project", ActiveIntent: "inspect the full project",
		MaxCycles: 6, CreatedAt: now, UpdatedAt: now,
	})
	result := agentloop.Result{
		Executed: []map[string]any{
			{"tool": "ccb.observation_request", "tool_call_id": "call-ready", "status": "ok", "result": map[string]any{
				"status": "ready", "bundle": map[string]any{
					"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-ready",
					"requested_views": []any{"mix.frequency_relationship"},
					"views":           map[string]any{"mix.frequency_relationship": map[string]any{"status": "ready"}},
					"audit_receipt":   map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-ready"},
				},
			}},
			{"tool": "ccb.observation_request", "tool_call_id": "call-rejected", "status": "ok", "result": map[string]any{
				"status": "rejected", "bundle": map[string]any{
					"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "requested_views": []any{"mix.masking_relationship"},
					"rejection_scope": "exact_view_set", "blocking_view_ids": []any{"mix.masking_relationship"},
					"audit_receipt": map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-rejected", "rejection_scope": "exact_view_set"},
				},
			}},
		},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "request more evidence", RequestedViewIDs: []string{"track.time_dynamics"},
		},
	}
	loop, ok := server.recordFreeStateDecision("conversation-ledger", result)
	if !ok {
		t.Fatal("recordFreeStateDecision did not find active loop")
	}
	available := firstMapFromAny(loop.ObservationLedger["available_views"])
	if firstStringFromMap(firstMapFromAny(available["mix.frequency_relationship"]), "observation_id") != "obs-ready" {
		t.Fatalf("ready CCB view was not persisted in loop ledger: %#v", loop.ObservationLedger)
	}
	if conclusion := firstMapFromAny(firstMapFromAny(available["mix.frequency_relationship"])["conclusion"]); firstStringFromMap(conclusion, "digest_kind") != "decision_digest" {
		t.Fatalf("ready CCB decision conclusion was not persisted in loop ledger: %#v", available)
	}
	rejected := freeStateMapRows(loop.ObservationLedger["rejected_view_sets"])
	if len(rejected) != 1 || firstStringFromMap(rejected[0], "rejection_scope") != "exact_view_set" {
		t.Fatalf("exact rejection scope was not persisted in loop ledger: %#v", loop.ObservationLedger)
	}
	encoded := freeStateLoopMap(loop)
	if len(firstMapFromAny(encoded["observation_ledger"])) == 0 {
		t.Fatalf("serialized free-state loop omitted observation ledger: %#v", encoded)
	}
}

func TestFreeStateContinuationLedgerCannotBeErasedByStaleRequestContext(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "ledger-merge", ConversationID: "ledger-merge-chat",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"available_views": map[string]any{"track.peak_structure": map[string]any{
				"view_id": "track.peak_structure", "status": "ready", "observation_id": "obs-ready",
			}},
			"rejected_view_sets": []map[string]any{{
				"fingerprint":     freeStateNormalizedViewFingerprint([]string{"mix.masking_relationship"}),
				"requested_views": []string{"mix.masking_relationship"}, "status": "rejected",
				"rejection_scope": "exact_view_set", "blocking_view_ids": []string{"mix.masking_relationship"},
			}},
		},
	})
	ctx, active := server.prepareFreeStateReasoningContext("ledger-merge-chat", "continue", map[string]any{
		"free_state_reasoning_loop": map[string]any{
			"schema_version": freeStateReasoningLoopSchema, "status": "observing", "original_intent": "inspect the project",
		},
	})
	if !active {
		t.Fatal("free-state loop was not active")
	}
	loop, ok := freeStateLoopFromAny(ctx["free_state_reasoning_loop"])
	if !ok {
		t.Fatal("merged loop was not serializable")
	}
	ledger := loop.ObservationLedger
	if firstMapFromAny(ledger["available_views"])["track.peak_structure"] == nil || len(freeStateMapRows(ledger["rejected_view_sets"])) != 1 {
		t.Fatalf("stale request context erased durable CCB ledger: %#v", ledger)
	}
}

func TestRecordFreeStateDecisionImportsContinuationObservationLedger(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "continuation-ledger", ConversationID: "continuation-ledger-chat",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
	})
	continuedLoop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "continuation-ledger", ConversationID: "continuation-ledger-chat",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"available_views": map[string]any{"mix.frequency_relationship": map[string]any{
				"view_id": "mix.frequency_relationship", "status": "partial", "observation_id": "obs-frequency",
			}},
			"rejected_view_sets": []map[string]any{{
				"fingerprint":     freeStateNormalizedViewFingerprint([]string{"mix.masking_relationship", "track.time_dynamics"}),
				"requested_views": []string{"mix.masking_relationship", "track.time_dynamics"}, "status": "rejected",
				"rejection_scope": "exact_view_set", "blocking_view_ids": []string{"mix.masking_relationship"},
				"non_blocking_view_ids": []string{"track.time_dynamics"},
			}},
		},
	}
	loop, ok := server.recordFreeStateDecision("continuation-ledger-chat", agentloop.Result{
		Continuation: &agentloop.Continuation{Context: map[string]any{"free_state_reasoning_loop": freeStateLoopMap(continuedLoop)}},
	})
	if !ok {
		t.Fatal("recordFreeStateDecision did not find active loop")
	}
	ledger := loop.ObservationLedger
	if firstMapFromAny(ledger["available_views"])["mix.frequency_relationship"] == nil || len(freeStateMapRows(ledger["rejected_view_sets"])) != 1 {
		t.Fatalf("continuation ledger was not imported: %#v", ledger)
	}
	persisted, persistedOK := server.freeStateLoop("continuation-ledger-chat")
	if !persistedOK || len(persisted.ObservationLedger) == 0 {
		t.Fatalf("continuation ledger was not stored durably: %#v", persisted)
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
				"target_ref": map[string]any{"kind": "track", "id": "1032", "label": "Vocals"},
			}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "tonal issue remains", RemainingIntent: "make the vocal clearer", ProcessorType: "eq",
			ObservationID: "obs-after",
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
			EvidenceStatus: "sufficient", RemainingIntent: "stabilize the vocal", ProcessorType: "compressor", ObservationID: "obs-vocal",
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

func TestFreeStateNeedsActionBindsReferencedObservationInsteadOfLatest(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "target-by-evidence", ConversationID: "target-by-evidence-chat",
		Status: "observing", OriginalIntent: "repair the project", ActiveIntent: "repair the project", MaxCycles: 6,
	})
	result := agentloop.Result{Executed: []map[string]any{
		freeStateTestTargetedObservationRecord("call-bass", "obs-bass", "1007", "Bass", "track.time_dynamics"),
		freeStateTestTargetedObservationRecord("call-vocals", "obs-vocals", "1032", "Vocals", "track.time_dynamics"),
	}, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
		EvidenceStatus: "sufficient", Summary: "bass needs control", RemainingIntent: "control bass dynamics",
		ProcessorType: "compressor", ObservationID: "obs-bass",
		SemanticProcessorIntent: &processorintent.Intent{
			SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
			Family: processorintent.FamilyBroadbandCompressor, Intent: "control bass dynamics",
			RequiredCoverage: []string{"threshold", "ratio"}, Scope: processorintent.ScopeCurrentTrack,
			ControlMode: processorintent.ControlModeSemantic, Confidence: 0.9, EvidenceRefs: []string{"obs-bass"},
		},
	}}
	loop, ok := server.recordFreeStateDecision("target-by-evidence-chat", result)
	if !ok || loop.Status != "awaiting_action" || firstStringFromMap(loop.TargetRef, "track_id") != "1007" {
		t.Fatalf("referenced observation did not bind bass target: %#v", loop)
	}
	if loop.LatestObservation == nil || firstStringFromMap(loop.LatestObservation.Summary, "observation_id") != "obs-bass" {
		t.Fatalf("latest observation was not replaced by referenced evidence: %#v", loop.LatestObservation)
	}
	available := firstMapFromAny(loop.ObservationLedger["available_views"])
	if available["track:1007::track.time_dynamics"] == nil || available["track:1032::track.time_dynamics"] == nil {
		t.Fatalf("target-aware ledger collapsed same view across tracks: %#v", available)
	}
}

func TestFreeStateNeedsActionWithoutUnambiguousObservationFailsClosed(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "ambiguous-target", ConversationID: "ambiguous-target-chat",
		Status: "observing", OriginalIntent: "repair the project", ActiveIntent: "repair the project", MaxCycles: 6,
	})
	loop, ok := server.recordFreeStateDecision("ambiguous-target-chat", agentloop.Result{Executed: []map[string]any{
		freeStateTestTargetedObservationRecord("call-bass", "obs-bass", "1007", "Bass", "track.time_dynamics"),
		freeStateTestTargetedObservationRecord("call-vocals", "obs-vocals", "1032", "Vocals", "track.time_dynamics"),
	}, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
		EvidenceStatus: "sufficient", Summary: "act", RemainingIntent: "control dynamics", ProcessorType: "compressor",
	}})
	if !ok || loop.Status != "blocked" || loop.LatestDecision == nil || loop.LatestDecision.Status != agentloop.FreeStateBlocked ||
		loop.LatestDecision.StopReason != "free_state_observation_target_unresolved" {
		t.Fatalf("ambiguous target did not fail closed: %#v", loop)
	}
}

func TestFreeStateReferencedTargetSurvivesProjectRuntimeRestart(t *testing.T) {
	server := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "bound-restart", ConversationID: "bound-restart-chat",
		Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "control bass dynamics", ActiveIntent: "control bass dynamics", MaxCycles: 6,
		TargetRef: map[string]any{"kind": "track", "id": "1007", "track_id": "1007", "label": "Bass"},
		LatestObservation: &agentloop.RecentObservation{Tool: "ccb.observation_request", ToolCallID: "call-bass", Status: "ok",
			Summary: map[string]any{"status": "ready", "observation_id": "obs-bass", "target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Bass"}}},
		LatestDecision: &agentloop.FreeStateDecision{SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsAction,
			EvidenceStatus: "sufficient", Summary: "bass needs control", RemainingIntent: "control bass dynamics",
			ProcessorType: "compressor", ObservationID: "obs-bass"},
	}
	server.storeFreeStateLoop(loop)
	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	restarted.mu.Unlock()
	restored, ok := restarted.freeStateLoop("bound-restart-chat")
	if !ok || firstStringFromMap(restored.TargetRef, "track_id") != "1007" || restored.LatestObservation == nil ||
		firstStringFromMap(restored.LatestObservation.Summary, "observation_id") != "obs-bass" || restored.LatestDecision == nil ||
		restored.LatestDecision.ObservationID != "obs-bass" {
		t.Fatalf("restart lost referenced target binding: %#v", restored)
	}
	_, retry, resumable := freeStateMaterializationRetry(restored)
	if !resumable || retry.RecentObservation == nil || firstStringFromMap(retry.RecentObservation.Summary, "observation_id") != "obs-bass" {
		t.Fatalf("materialization retry lost referenced observation: %#v", retry)
	}
}

func TestFreeStateTargetScopedRejectionDoesNotBlockSameViewsOnOtherTrack(t *testing.T) {
	bass := &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{
		"status": "rejected", "requested_views": []any{"track.time_dynamics"},
		"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Bass"},
	}}
	vocals := &agentloop.RecentObservation{Tool: "ccb.observation_request", Summary: map[string]any{
		"status": "rejected", "requested_views": []any{"track.time_dynamics"},
		"target_ref": map[string]any{"kind": "track", "id": "1032", "label": "Vocals"},
	}}
	bassRow := freeStateRejectedObservation(bass)
	vocalRow := freeStateRejectedObservation(vocals)
	if freeStateRejectedObservationRecorded([]map[string]any{bassRow}, vocalRow) {
		t.Fatalf("target-scoped rejection collapsed across tracks: bass=%#v vocals=%#v", bassRow, vocalRow)
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
	processor, status, receipt, attempted = freeStateAcousticActionOutcome(PendingInteraction{}, ChatResponse{
		Workflow: semanticDynamicWorkflow,
		WorkflowData: map[string]any{
			"status": "executed", "processor_type": "de_esser",
			"execution_receipt": map[string]any{"schema_version": semanticDynamicReceipt, "processor_type": "de_esser", "status": "executed"},
		},
	})
	if !attempted || processor != "de_esser" || status != "applied" || firstStringFromMap(receipt, "processor_type") != "de_esser" {
		t.Fatalf("dynamic receipt processor=%q status=%q attempted=%v receipt=%#v", processor, status, attempted, receipt)
	}
	processor, status, receipt, attempted = freeStateAcousticActionOutcome(PendingInteraction{}, ChatResponse{
		Workflow: "mix_tick", GoalStatus: string(agentruntime.StatusCompleted), StopReason: "mix_tick_applied_reobserved",
		WorkflowData:        map[string]any{"track_id": "1007", "operation": "track_gain_adjust", "observation_id": "obs-after-tick"},
		ExecutedKernelReply: []map[string]any{{"tool": "mix.observe", "status": "ok"}},
	})
	if !attempted || processor != "mix_tick" || status != "applied" || firstStringFromMap(receipt, "stop_reason") != "mix_tick_applied_reobserved" {
		t.Fatalf("mix tick receipt processor=%q status=%q attempted=%v receipt=%#v", processor, status, attempted, receipt)
	}
}

func TestFreeStateDynamicReceiptsEnterLedgerForEveryCurrentFamily(t *testing.T) {
	families := []string{"limiter", "gate_expander", "de_esser", "transient_shaper", "multiband_dynamics"}
	for _, family := range families {
		t.Run(family, func(t *testing.T) {
			server := New(nil, nil, nil)
			now := time.Now().UTC()
			server.storeFreeStateLoop(freeStateReasoningLoop{
				SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-ledger", ConversationID: "conversation-" + family,
				Status: "awaiting_action", DecisionPhase: freeStatePhaseProcessorMaterialization,
				OriginalIntent: "apply the selected dynamic treatment", ActiveIntent: "apply the selected dynamic treatment",
				Cycle: 5, MaxCycles: 99, CreatedAt: now, UpdatedAt: now,
			})
			resp := server.maybeContinueFreeStateAfterInteraction(nil, PendingInteraction{ConversationID: "conversation-" + family}, ChatResponse{
				ConversationID: "conversation-" + family, Workflow: semanticDynamicWorkflow,
				GoalStatus: string(agentruntime.StatusCompleted), WorkflowData: map[string]any{
					"status": "executed", "processor_type": family,
					"execution_receipt": map[string]any{
						"schema_version": semanticDynamicReceipt, "processor_type": family, "status": "executed",
						"receipt_id": "receipt-" + family,
					},
				},
			}, "approve")
			loop, ok := server.freeStateLoop("conversation-" + family)
			if !ok || len(loop.Actions) != 1 {
				t.Fatalf("dynamic action was not recorded: %#v", loop)
			}
			action := loop.Actions[0]
			if action.Cycle != 6 || action.ProcessorType != family || action.Status != "applied" || action.Workflow != semanticDynamicWorkflow {
				t.Fatalf("ledger action = %#v", action)
			}
			if firstStringFromMap(action.Receipt, "receipt_id") != "receipt-"+family {
				t.Fatalf("receipt was not retained: %#v", action.Receipt)
			}
			if !loop.RequiresPostActionObservation || loop.Status != "blocked" || loop.MaxCycles != freeStateDefaultMaxCycles {
				t.Fatalf("post-action ledger boundary = %#v", loop)
			}
			if resp.StopReason != "free_state_cycle_limit" {
				t.Fatalf("bounded ledger response = %#v", resp)
			}
		})
	}
}

func TestFreeStateMixTickReceiptEntersPostActionEvaluation(t *testing.T) {
	server := New(nil, nil, nil)
	now := time.Now().UTC()
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-mix-tick", ConversationID: "conversation-mix-tick",
		Status: "awaiting_experiment", DecisionPhase: freeStatePhaseProcessorMaterialization,
		OriginalIntent: "improve the low-end relationship", ActiveIntent: "test a small bass gain reduction",
		Cycle: 5, MaxCycles: 99, CreatedAt: now, UpdatedAt: now,
	})
	resp := server.maybeContinueFreeStateAfterInteraction(nil, PendingInteraction{ConversationID: "conversation-mix-tick"}, ChatResponse{
		ConversationID: "conversation-mix-tick", Workflow: "mix_tick", GoalStatus: string(agentruntime.StatusCompleted),
		StopReason:          "mix_tick_applied_reobserved",
		WorkflowData:        map[string]any{"track_id": "1007", "operation": "track_gain_adjust", "observation_id": "obs-after-tick"},
		ExecutedKernelReply: []map[string]any{{"tool": "mix.apply_tick", "status": "ok"}, {"tool": "mix.observe", "status": "ok"}},
	}, "approve")
	loop, ok := server.freeStateLoop("conversation-mix-tick")
	if !ok || len(loop.Actions) != 1 || loop.Actions[0].ProcessorType != "mix_tick" || loop.Actions[0].Status != "applied" {
		t.Fatalf("mix tick action was not recorded: %#v", loop)
	}
	if !loop.RequiresPostActionObservation || loop.DecisionPhase != freeStatePhasePostActionEvaluation {
		t.Fatalf("mix tick did not enter post-action evaluation: %#v", loop)
	}
	if resp.StopReason != "free_state_cycle_limit" {
		t.Fatalf("bounded mix tick handoff = %#v", resp)
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

func TestFreeStateCCBObservationPrefersLatestUsableResult(t *testing.T) {
	result := agentloop.Result{Executed: []map[string]any{
		freeStateTestObservationRecord("ccb-ready", "ready", "obs-ready", []any{"track.time_dynamics"}, "ccbr-ready"),
		freeStateTestObservationRecord("ccb-rejected", "rejected", "obs-rejected", []any{"mix.masking_relationship"}, "ccbr-rejected"),
	}}
	observation := freeStateCCBObservation(result)
	if observation == nil || firstStringFromMap(observation.Summary, "observation_id") != "obs-ready" {
		t.Fatalf("latest usable observation was replaced by rejection: %#v", observation)
	}
}

func TestFreeStateLedgerPersistsObservationAuditReceipt(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-audit", ConversationID: "conversation-audit",
		Status: "observing", OriginalIntent: "inspect dynamics", ActiveIntent: "inspect dynamics", MaxCycles: 6,
	})
	loop, ok := server.recordFreeStateDecision("conversation-audit", agentloop.Result{
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "tool_call_id": "ccb-audit", "status": "ok",
			"result": map[string]any{"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": "obs-audit",
				"audit_receipt": map[string]any{
					"schema_version": "ccb_observation_receipt.v1", "receipt_id": "ccbr-audit", "requested_by": "model",
					"model_requested_view_ids": []any{"track.peak_structure"}, "actual_executed_view_ids": []any{"track.peak_structure"},
					"view_set_matches": true, "scope": "selected_track", "freshness": map[string]any{"status": "fresh"}, "status": "executed",
				},
			}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "observe", RequestedViewIDs: []string{"track.peak_structure"},
		},
	})
	if !ok || len(loop.ObservationReceipts) != 1 || firstStringFromMap(loop.ObservationReceipts[0], "receipt_id") != "ccbr-audit" {
		t.Fatalf("observation receipt was not persisted: %+v", loop)
	}
}

func TestFreeStateLedgerPersistsRejectedObservationRequest(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-rejected", ConversationID: "conversation-rejected",
		Status: "observing", OriginalIntent: "inspect dynamics", ActiveIntent: "inspect dynamics", MaxCycles: 6,
	})
	loop, ok := server.recordFreeStateDecision("conversation-rejected", agentloop.Result{
		Executed: []map[string]any{{
			"tool": "ccb.observation_request", "tool_call_id": "ccb-rejected", "status": "rejected",
			"result": map[string]any{"bundle": map[string]any{
				"schema_version": "ccb_observation_bundle.v1", "status": "rejected", "request_id": "req-rejected",
				"requested_views":  []any{"mix.masking_relationship"},
				"omission_reasons": []any{"mix.masking_relationship: deferred"},
			}},
		}},
		FreeStateDecision: &agentloop.FreeStateDecision{
			SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
			EvidenceStatus: "insufficient", Summary: "observe", RequestedViewIDs: []string{"mix.masking_relationship"},
		},
	})
	if !ok || len(loop.RejectedObservationRequests) != 1 || firstStringFromMap(loop.RejectedObservationRequests[0], "status") != "rejected" {
		t.Fatalf("rejected observation was not persisted: %+v", loop)
	}
}

func TestFreeStateRejectedObservationAcceptsLegacyCommandName(t *testing.T) {
	row := freeStateRejectedObservation(&agentloop.RecentObservation{
		CommandName: "ccb_observation_request", Status: "rejected",
		Summary: map[string]any{"status": "rejected", "requested_views": []any{"mix.masking_relationship"}},
	})
	if firstStringFromMap(row, "status") != "rejected" || len(freeStateStringSlice(row["requested_views"])) != 1 {
		t.Fatalf("legacy CCB command name was not recorded: %#v", row)
	}
}

func TestFreeStateLedgerCollectsRejectedThenReadyFromSameResult(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-sequence", ConversationID: "conversation-sequence",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
	})
	result := agentloop.Result{Executed: []map[string]any{
		freeStateTestObservationRecord("ccb-rejected", "rejected", "obs-rejected", []any{"mix.masking_relationship", "processor.identity_and_controls"}, "ccbr-rejected"),
		freeStateTestObservationRecord("ccb-ready", "ready", "obs-ready", []any{"track.time_dynamics"}, "ccbr-ready"),
	}, FreeStateDecision: &agentloop.FreeStateDecision{
		SchemaVersion: agentloop.FreeStateDecisionSchema, Status: agentloop.FreeStateNeedsObservation,
		EvidenceStatus: "insufficient", Summary: "continue with bounded evidence", RequestedViewIDs: []string{"track.time_dynamics"},
	}}
	loop, ok := server.recordFreeStateDecision("conversation-sequence", result)
	if !ok || len(loop.RejectedObservationRequests) != 1 || len(loop.ObservationReceipts) != 2 {
		t.Fatalf("sequence receipts were lost: %#v", loop)
	}
	if loop.LatestObservation == nil || firstStringFromMap(loop.LatestObservation.Summary, "observation_id") != "obs-ready" {
		t.Fatalf("latest usable observation = %#v", loop.LatestObservation)
	}
	ledger := firstMapFromAny(loop.ObservationLedger)
	if len(freeStateMapRows(ledger["rejected_view_sets"])) != 1 || len(freeStateMapRows(ledger["receipts"])) != 2 {
		t.Fatalf("compact ledger did not retain the full sequence: %#v", ledger)
	}
	available := firstMapFromAny(ledger["available_views"])
	if firstStringFromMap(firstMapFromAny(available["track.time_dynamics"]), "observation_id") != "obs-ready" {
		t.Fatalf("available view was not indexed: %#v", available)
	}
	body, _ := json.Marshal(ledger)
	for _, forbidden := range []string{"raw-secret", `"views"`, "plugin-secret", "compressor-family", "expected_coverage"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("compact ledger leaked %q: %s", forbidden, body)
		}
	}
}

func TestFreeStateRejectedLedgerSurvivesProjectRuntimeRestart(t *testing.T) {
	server := New(nil, nil, nil)
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-restart", ConversationID: "conversation-restart",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"rejected_view_sets": []map[string]any{{
				"fingerprint":     freeStateNormalizedViewFingerprint([]string{"mix.masking_relationship", "processor.identity_and_controls"}),
				"requested_views": []string{"mix.masking_relationship", "processor.identity_and_controls"},
				"status":          "rejected", "retry_policy": "do_not_retry",
			}},
		},
	}
	server.storeFreeStateLoop(loop)
	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	restarted.mu.Unlock()
	restored, ok := restarted.freeStateLoop("conversation-restart")
	if !ok || len(freeStateMapRows(restored.ObservationLedger["rejected_view_sets"])) != 1 {
		t.Fatalf("restart lost compact rejection ledger: %#v", restored)
	}
	ctx, active := restarted.prepareFreeStateReasoningContext("conversation-restart", "continue", map[string]any{})
	if !active || len(freeStateMapRows(firstMapFromAny(firstMapFromAny(ctx["free_state_reasoning_loop"])["observation_ledger"])["rejected_view_sets"])) != 1 {
		t.Fatalf("restored ledger was not rebound to continuation context: %#v", ctx)
	}
}

func freeStateTestObservationRecord(callID, status, observationID string, requested []any, receiptID string) map[string]any {
	return map[string]any{
		"tool": "ccb.observation_request", "command_name": "ccb_observation_request", "tool_call_id": callID, "status": status,
		"result": map[string]any{"status": status, "bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "bundle_id": "bundle-" + callID, "status": status,
			"observation_id": observationID, "request_id": "request-" + callID, "requested_views": requested,
			"freshness": map[string]any{"status": status}, "limitations": []any{"bounded"}, "evidence_refs": []any{"evidence://" + callID},
			"omission_reasons": []any{"deferred"},
			"views":            map[string]any{"track.time_dynamics": map[string]any{"status": "ready", "raw": "raw-secret", "plugin_id": "plugin-secret"}},
			"audit_receipt":    map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": receiptID, "status": status},
		}},
	}
}

func freeStateTestTargetedObservationRecord(callID, observationID, trackID, trackName, viewID string) map[string]any {
	return map[string]any{
		"tool": "ccb.observation_request", "tool_call_id": callID, "status": "ok",
		"result": map[string]any{"status": "ready", "bundle": map[string]any{
			"schema_version": "ccb_observation_bundle.v1", "status": "ready", "observation_id": observationID,
			"requested_views": []any{viewID}, "views": map[string]any{viewID: map[string]any{"status": "ready"}},
			"target_ref":    map[string]any{"kind": "track", "id": trackID, "label": trackName},
			"audit_receipt": map[string]any{"schema_version": "ccb_observation_receipt.v1", "receipt_id": "receipt-" + callID},
		}},
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

func TestProjectChangeInvalidatesOnlyAffectedFreeStateObservationViews(t *testing.T) {
	ledger := map[string]any{
		"schema_version": freeStateObservationLedgerSchema,
		"available_views": map[string]any{
			"track:1007::track.basic_energy": map[string]any{"view_id": "track.basic_energy", "status": "ready", "freshness": map[string]any{"status": "ready"}},
			"mix.frequency_relationship":     map[string]any{"view_id": "mix.frequency_relationship", "status": "ready", "freshness": map[string]any{"status": "ready"}},
			"track:1007::track.stereo_space": map[string]any{"view_id": "track.stereo_space", "status": "ready", "freshness": map[string]any{"status": "ready"}},
			"project.structure":              map[string]any{"view_id": "project.structure", "status": "ready", "freshness": map[string]any{"status": "ready"}},
		},
	}
	updated := invalidateFreeStateObservationLedger(ledger, map[string]any{
		"change_id": "shadow_change_000001", "affected_scopes": []any{"track.level", "mix.frequency_relationship"},
	})
	available := firstMapFromAny(updated["available_views"])
	for _, key := range []string{"track:1007::track.basic_energy", "mix.frequency_relationship"} {
		row := firstMapFromAny(available[key])
		if row["status"] != "stale" || row["invalidated_by_change_id"] != "shadow_change_000001" {
			t.Fatalf("affected row %s was not stale: %#v", key, row)
		}
	}
	for _, key := range []string{"track:1007::track.stereo_space", "project.structure"} {
		if row := firstMapFromAny(available[key]); row["status"] != "ready" {
			t.Fatalf("unaffected row %s should remain ready: %#v", key, row)
		}
	}
}
