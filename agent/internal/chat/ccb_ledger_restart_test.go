package chat

import "testing"

func TestFreeStateReadyEvidenceAndScopedRejectionSurviveProjectRuntimeRestart(t *testing.T) {
	server := New(nil, nil, nil)
	server.storeFreeStateLoop(freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "free-state-ready-restart", ConversationID: "conversation-ready-restart",
		Status: "observing", OriginalIntent: "inspect the project", ActiveIntent: "inspect the project", MaxCycles: 6,
		ObservationLedger: map[string]any{
			"schema_version": freeStateObservationLedgerSchema,
			"available_views": map[string]any{
				"mix.multitrack_relationship": map[string]any{
					"view_id": "mix.multitrack_relationship", "status": "ready", "observation_id": "obs-ready", "tool_call_id": "ccb-ready",
					"freshness": map[string]any{"status": "fresh"}, "evidence_refs": []any{"evidence://ready"},
				},
			},
			"rejected_view_sets": []map[string]any{{
				"fingerprint":     freeStateNormalizedViewFingerprint([]string{"mix.masking_relationship", "processor.identity_and_controls"}),
				"requested_views": []string{"mix.masking_relationship", "processor.identity_and_controls"}, "status": "rejected", "retry_policy": "do_not_retry",
				"rejection_scope": "exact_view_set", "blocking_view_ids": []string{"mix.masking_relationship"}, "non_blocking_view_ids": []string{"processor.identity_and_controls"},
			}},
		},
	})
	server.mu.Lock()
	state := server.projectAgentRuntimeStateLocked()
	server.mu.Unlock()
	restarted := New(nil, nil, nil)
	restarted.mu.Lock()
	restarted.restoreProjectAgentRuntimeStateLocked(state)
	restarted.mu.Unlock()
	loop, ok := restarted.freeStateLoop("conversation-ready-restart")
	if !ok {
		t.Fatalf("restored free-state loop missing")
	}
	ledger := firstMapFromAny(loop.ObservationLedger)
	if firstMapFromAny(ledger["available_views"])["mix.multitrack_relationship"] == nil || len(freeStateMapRows(ledger["rejected_view_sets"])) != 1 {
		t.Fatalf("ready/rejection evidence was not restored: %#v", ledger)
	}
	ctx, active := restarted.prepareFreeStateReasoningContext("conversation-ready-restart", "continue", map[string]any{})
	contextLedger := firstMapFromAny(firstMapFromAny(ctx["free_state_reasoning_loop"])["observation_ledger"])
	if !active || firstMapFromAny(contextLedger["available_views"])["mix.multitrack_relationship"] == nil || len(freeStateMapRows(contextLedger["rejected_view_sets"])) != 1 {
		t.Fatalf("continuation context lost ready/rejection evidence: %#v", contextLedger)
	}
}
