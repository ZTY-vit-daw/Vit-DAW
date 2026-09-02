package chat

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/semanticorchestrator"
)

func progressiveLimiterIntent() processorintent.Intent {
	return processorintent.Intent{
		SchemaVersion:    processorintent.SchemaVersion,
		Status:           processorintent.StatusResolved,
		Family:           processorintent.FamilyLimiter,
		Intent:           "protect peaks",
		RequiredCoverage: []string{"output_ceiling"},
		Scope:            processorintent.ScopeCurrentTrack,
		ControlMode:      processorintent.ControlModeSemantic,
		Confidence:       0.92,
		EvidenceRefs:     []string{"mix.observe:obs-1"},
	}
}

func progressiveObservation() *agentloop.RecentObservation {
	return &agentloop.RecentObservation{
		Tool: "ccb.observation_request", Status: "ok",
		Summary: map[string]any{
			"schema_version":     "ccb_observation_bundle.v1",
			"status":             "ready",
			"read_only":          true,
			"mutation_authority": false,
			"observation_id":     "obs-1",
			"views":              map[string]any{"track.peak_structure": map[string]any{"peak_db": -1.0}},
			"audit_receipt": map[string]any{
				"schema_version":           "ccb_observation_receipt.v1",
				"requested_by":             "model",
				"model_requested_view_ids": []string{"track.peak_structure"},
				"actual_executed_view_ids": []string{"track.peak_structure"},
				"view_set_matches":         true,
				"scope":                    "selected_track",
				"freshness":                map[string]any{"status": "fresh"},
				"status":                   "executed",
			},
		},
	}
}

func progressiveMultibandIntent() processorintent.Intent {
	return processorintent.Intent{
		SchemaVersion:    processorintent.SchemaVersion,
		Status:           processorintent.StatusResolved,
		Family:           processorintent.FamilyMultibandDynamics,
		Intent:           "tame low-band level drift with a bounded band threshold step",
		RequiredCoverage: []string{"threshold"},
		Scope:            processorintent.ScopeCurrentTrack,
		ControlMode:      processorintent.ControlModeSemantic,
		Confidence:       0.9,
		EvidenceRefs:     []string{"mix.observe:obs-1"},
	}
}

// progressivePartialBandDynamicsObservation mirrors the FAM6-S2 wall shape
// (run 20260902_111733): the model's last ccb.observation_request returned the
// DAD source-only track.band_dynamics view, which discloses as partial by
// design while the CCB audit receipt stays executed and the binding fresh.
func progressivePartialBandDynamicsObservation() *agentloop.RecentObservation {
	return &agentloop.RecentObservation{
		Tool: "ccb.observation_request", Status: "partial",
		Summary: map[string]any{
			"schema_version":     "ccb_observation_bundle.v1",
			"status":             "partial",
			"read_only":          true,
			"mutation_authority": false,
			"observation_id":     "obs-mb-1",
			"views":              map[string]any{"track.band_dynamics": map[string]any{"status": "partial"}},
			"audit_receipt": map[string]any{
				"schema_version":           "ccb_observation_receipt.v1",
				"requested_by":             "model",
				"model_requested_view_ids": []string{"track.band_dynamics"},
				"actual_executed_view_ids": []string{"track.band_dynamics"},
				"view_set_matches":         true,
				"scope":                    "selected_track",
				"freshness":                map[string]any{"status": "fresh"},
				"status":                   "executed",
			},
		},
	}
}

// TestSemanticProgressiveDisclosurePartialObservationGateForms locks the four
// boundary forms of the partial-level alignment: expanding the disclosure
// level to partial does not expand the audit-receipt or freshness guarantees.
func TestSemanticProgressiveDisclosurePartialObservationGateForms(t *testing.T) {
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.band_dynamics"}}}

	// Form 1: partial + fresh + audited passes the gate.
	requestContext := map[string]any{}
	if err := semanticProgressiveDisclosureAccept(requestContext, progressiveMultibandIntent(), result, progressivePartialBandDynamicsObservation()); err != nil {
		t.Fatalf("fresh audited partial observation was rejected: %v", err)
	}
	state, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"])
	if !ok || state.Stage != semanticorchestrator.StagePhysicalTarget {
		t.Fatalf("partial observation did not advance the orchestrator: state=%+v ok=%v", state, ok)
	}

	// Form 2: partial but stale stays rejected.
	stale := progressivePartialBandDynamicsObservation()
	stale.Summary["audit_receipt"].(map[string]any)["freshness"] = map[string]any{"status": "stale"}
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveMultibandIntent(), result, stale); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale partial observation was accepted: %v", err)
	}

	// Form 3: partial but unaudited stays rejected.
	unaudited := progressivePartialBandDynamicsObservation()
	delete(unaudited.Summary, "audit_receipt")
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveMultibandIntent(), result, unaudited); err == nil || !strings.Contains(err.Error(), "audit receipt") {
		t.Fatalf("unaudited partial observation was accepted: %v", err)
	}

	// Form 4: error/rejected/missing/empty statuses stay rejected.
	for _, status := range []string{"error", "rejected", "missing", ""} {
		failed := progressivePartialBandDynamicsObservation()
		failed.Status = status
		if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveMultibandIntent(), result, failed); err == nil || !strings.Contains(err.Error(), "successful observation") {
			t.Fatalf("%q observation was accepted: %v", status, err)
		}
	}
}

func TestSemanticProgressiveDisclosurePersistsAndReplaysAllStages(t *testing.T) {
	requestContext := map[string]any{}
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	if err := semanticProgressiveDisclosureAccept(requestContext, progressiveLimiterIntent(), result, progressiveObservation()); err != nil {
		t.Fatal(err)
	}
	state, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"])
	if !ok || state.Stage != semanticorchestrator.StagePhysicalTarget {
		t.Fatalf("initial state=%+v ok=%v", state, ok)
	}
	if err := semanticProgressiveDisclosureAdvanceToConfirmation(requestContext, []string{"control_ref_ceiling"}); err != nil {
		t.Fatal(err)
	}
	if err := semanticProgressiveDisclosureConfirm(requestContext); err != nil {
		t.Fatal(err)
	}
	if err := semanticProgressiveDisclosureReceipt(requestContext, map[string]any{"status": "applied", "receipt_id": "receipt-1"}); err != nil {
		t.Fatal(err)
	}
	state, ok = semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"])
	if !ok || state.Stage != semanticorchestrator.StageReceipt || len(state.BoundControlRefs) != 1 {
		t.Fatalf("completed state=%+v ok=%v", state, ok)
	}
	resumed, err := semanticProgressiveDisclosureOrchestrator(requestContext)
	if err != nil || resumed.State().Stage != semanticorchestrator.StageReceipt {
		t.Fatalf("resume state=%+v err=%v", resumed.State(), err)
	}
}

func TestSemanticProgressiveDisclosureRejectsViewMutationAndMissingObservation(t *testing.T) {
	requestContext := map[string]any{}
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	observation := progressiveObservation()
	observation.Summary["audit_receipt"].(map[string]any)["actual_executed_view_ids"] = []string{"track.activity_structure"}
	if err := semanticProgressiveDisclosureAccept(requestContext, progressiveLimiterIntent(), result, observation); err == nil || !strings.Contains(err.Error(), "view-set mismatch") {
		t.Fatalf("view mutation was accepted: %v", err)
	}
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveLimiterIntent(), result, nil); err == nil || !strings.Contains(err.Error(), "requires a successful model-requested observation") {
		t.Fatalf("missing observation was accepted: %v", err)
	}
}

func TestSemanticProgressiveDisclosureRejectsAuditThatDisagreesWithModelRequest(t *testing.T) {
	requestContext := map[string]any{}
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	observation := progressiveObservation()
	observation.Summary["audit_receipt"].(map[string]any)["model_requested_view_ids"] = []string{"track.activity_structure"}
	observation.Summary["audit_receipt"].(map[string]any)["actual_executed_view_ids"] = []string{"track.activity_structure"}
	if err := semanticProgressiveDisclosureAccept(requestContext, progressiveLimiterIntent(), result, observation); err == nil || !strings.Contains(err.Error(), "does not match CCB audit") {
		t.Fatalf("audit request substitution was accepted: %v", err)
	}
}

func TestSemanticProgressiveDisclosureRejectsUnsuccessfulOrUnauditedObservation(t *testing.T) {
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	failed := progressiveObservation()
	failed.Status = "error"
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveLimiterIntent(), result, failed); err == nil || !strings.Contains(err.Error(), "successful observation") {
		t.Fatalf("failed observation was accepted: %v", err)
	}
	unaudited := progressiveObservation()
	delete(unaudited.Summary, "audit_receipt")
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveLimiterIntent(), result, unaudited); err == nil || !strings.Contains(err.Error(), "audit receipt") {
		t.Fatalf("unaudited observation was accepted: %v", err)
	}
}

func TestSemanticProgressiveDisclosureRejectsIncompleteCCBAuditInsteadOfUsingViewsMap(t *testing.T) {
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	observation := progressiveObservation()
	audit := observation.Summary["audit_receipt"].(map[string]any)
	delete(audit, "actual_executed_view_ids")
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveLimiterIntent(), result, observation); err == nil || !strings.Contains(err.Error(), "missing requested or executed view ids") {
		t.Fatalf("incomplete audit was accepted via ordinary views: %v", err)
	}
	observation = progressiveObservation()
	observation.Summary["audit_receipt"].(map[string]any)["requested_by"] = "caller"
	if err := semanticProgressiveDisclosureAccept(map[string]any{}, progressiveLimiterIntent(), result, observation); err == nil || !strings.Contains(err.Error(), "requested by the model") {
		t.Fatalf("caller observation was accepted: %v", err)
	}
}

func TestSemanticPostLoadFinalizationCarriesProgressiveState(t *testing.T) {
	requestContext := map[string]any{}
	result := agentloop.Result{FreeStateDecision: &agentloop.FreeStateDecision{RequestedViewIDs: []string{"track.peak_structure"}}}
	if err := semanticProgressiveDisclosureAccept(requestContext, progressiveLimiterIntent(), result, progressiveObservation()); err != nil {
		t.Fatal(err)
	}
	server := New(nil, nil, nil)
	response := server.finalizeSemanticProcessorPostLoadHandoff(PendingPlan{Context: requestContext}, ChatResponse{Workflow: semanticTreatmentWorkflow})
	state, ok := semanticProgressiveDisclosureState(response.WorkflowData["semantic_progressive_disclosure"])
	if !ok || state.Stage != semanticorchestrator.StagePhysicalTarget {
		t.Fatalf("post-load response lost progressive state: %+v", response.WorkflowData)
	}
	if len(firstMapFromAny(response.WorkflowData["request_context"])) == 0 {
		t.Fatalf("post-load response lost request context: %+v", response.WorkflowData)
	}
}
