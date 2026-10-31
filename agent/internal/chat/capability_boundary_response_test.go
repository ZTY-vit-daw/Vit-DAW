package chat

import (
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/capabilityboundary"
	agentruntime "vit-daw-agent/internal/runtime"
)

// CAP-BOUNDARY-1 response-surface tests: both boundary faces (free-state
// capability_blocked and the C2 governed boundary) must carry the structured
// capability_boundary.v1 payload with mutation_performed=false, while the
// existing receipt/stop-reason law stays byte-identical.

func boundaryFromWorkflowData(t *testing.T, resp ChatResponse) capabilityboundary.Boundary {
	t.Helper()
	boundary, ok := resp.WorkflowData["capability_boundary"].(capabilityboundary.Boundary)
	if !ok {
		t.Fatalf("workflow data does not carry a capability_boundary.v1 payload: %+v", resp.WorkflowData["capability_boundary"])
	}
	if boundary.SchemaVersion != capabilityboundary.SchemaVersion {
		t.Fatalf("boundary schema version = %q, want %q", boundary.SchemaVersion, capabilityboundary.SchemaVersion)
	}
	if boundary.MutationPerformed {
		t.Fatalf("boundary payload claims a mutation")
	}
	for _, action := range boundary.NextActions {
		switch action.Kind {
		case capabilityboundary.KindAlternateProcessor, capabilityboundary.KindNativeProcessor,
			capabilityboundary.KindManualGuidance, capabilityboundary.KindOpenPluginUI,
			capabilityboundary.KindCertificationEntry:
		default:
			t.Fatalf("boundary payload invented action kind %q", action.Kind)
		}
	}
	return boundary
}

func TestCapabilityBlockedBoundaryResponseCarriesRemediationPayload(t *testing.T) {
	s, loop, res := gateRefusedFreeStateFixture(t)
	stored, ok := s.recordFreeStateDecision(loop.ConversationID, res)
	if !ok {
		t.Fatal("gate-refused decision was not retained by recordFreeStateDecision")
	}
	resp, blocked := s.capabilityBlockedBoundaryResponse(loop.ConversationID, "default", res, stored)
	if !blocked {
		t.Fatal("capability_blocked boundary was not detected on the settled loop")
	}
	boundary := boundaryFromWorkflowData(t, resp)
	if boundary.Class != capabilityboundary.ClassFreeState {
		t.Fatalf("boundary class = %q, want free_state", boundary.Class)
	}
	if boundary.Code != "admission_gate_failed" {
		t.Fatalf("boundary code = %q, want admission_gate_failed", boundary.Code)
	}
	joined := strings.Join(boundary.BlockedBy, ",")
	if !strings.Contains(joined, "G6_target_evidence") || !strings.Contains(joined, "G8_target_consistency") {
		t.Fatalf("boundary blocked_by lost the R3 gate signature: %v", boundary.BlockedBy)
	}
	// M3-FS-GATE-EVIDENCE-WIRE-1 A2: the fixture's decision carries a
	// track_gain improvement proposal, so the family now resolves from the
	// proposal's action domain (volume) instead of reporting
	// family_unresolved. Volume is a native parameter family: no plugin
	// alternates exist and none may be fabricated — the per-class absence
	// disclosures replace the one root-cause family_unresolved.
	if boundary.Family != "volume" {
		t.Fatalf("boundary family = %q, want volume (from the proposal's action domain)", boundary.Family)
	}
	for _, reason := range boundary.UnavailableReasons {
		if reason == capabilityboundary.ReasonFamilyUnresolved {
			t.Fatalf("a proposal-bearing decision must not report family_unresolved: %v", boundary.UnavailableReasons)
		}
	}
	if !containsString(boundary.UnavailableReasons, capabilityboundary.ReasonNoSameFamilyCandidate) {
		t.Fatalf("volume boundary must disclose the honestly absent same-family class: %v", boundary.UnavailableReasons)
	}
	for _, action := range boundary.NextActions {
		if action.Kind == capabilityboundary.KindAlternateProcessor || action.Kind == capabilityboundary.KindNativeProcessor {
			t.Fatalf("a no-candidate volume boundary must not offer substitution actions: %+v", action)
		}
	}
	// The pre-existing receipt face stays intact alongside the new payload.
	if resp.StopReason != "capability_blocked" {
		t.Fatalf("stop reason changed: %q", resp.StopReason)
	}
	if _, hasReceipt := resp.WorkflowData["free_state_admission_receipt"]; !hasReceipt {
		t.Fatal("admission receipt dropped from the boundary face")
	}
}

func TestCapabilityBlockedBoundaryResponseWithFamilyOffersGovernedOptions(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-cb-family",
		ConversationID: "conversation-cb-family", GoalID: "goal-cb-family", RunID: "run-cb-family",
		OriginalIntent: "压一点人声的动态", CreatedAt: now, UpdatedAt: now,
		Status:    "blocked",
		TargetRef: map[string]any{"kind": "track", "id": "1017", "track_id": "1017", "track_name": "vocals"},
		LatestDecision: &agentloop.FreeStateDecision{
			Status:        agentloop.FreeStateCapabilityBlocked,
			Summary:       "no admitted broadband compressor for the requested control",
			ProcessorType: "compressor",
		},
	}
	s.storeFreeStateLoop(loop)
	res := agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusCompleted, Reply: "模型原文"}
	resp, blocked := s.capabilityBlockedBoundaryResponse(loop.ConversationID, "default", res, loop)
	if !blocked {
		t.Fatal("model-terminal boundary was not detected")
	}
	boundary := boundaryFromWorkflowData(t, resp)
	if boundary.Family != "broadband_compressor" {
		t.Fatalf("boundary family = %q, want broadband_compressor", boundary.Family)
	}
	// The test server owns no harness/plugin store, so no PCA-admitted
	// alternate can exist; the ungated native path fact and the compressor
	// certification entry are product facts and must still be offered, after
	// nothing (no alternates exist here) and before nothing else.
	kinds := []string{}
	for _, action := range boundary.NextActions {
		kinds = append(kinds, action.Kind)
	}
	if len(kinds) != 2 || kinds[0] != capabilityboundary.KindNativeProcessor || kinds[1] != capabilityboundary.KindCertificationEntry {
		t.Fatalf("expected [native_processor certification_entry] on a family-bearing store-less boundary, got %v", kinds)
	}
	cert := boundary.NextActions[1]
	if cert.Target["endpoint"] != "/agent/processor-certification/candidates?family=broadband_compressor" {
		t.Fatalf("certification entry target wrong: %+v", cert.Target)
	}
	if cert.Target["consent_token"] != processorCertificationConsent {
		t.Fatalf("certification entry lost the consent token: %+v", cert.Target)
	}
	noCandidate := false
	for _, reason := range boundary.UnavailableReasons {
		if reason == capabilityboundary.ReasonNoSameFamilyCandidate {
			noCandidate = true
		}
	}
	if !noCandidate {
		t.Fatalf("no-candidate disclosure missing: %v", boundary.UnavailableReasons)
	}
}

// The family_unresolved disclosure stays reserved for the decisions that
// genuinely name no family: no semantic intent, no processor type, and no
// improvement proposal (M3-FS-GATE-EVIDENCE-WIRE-1 A2 keeps the unresolved
// verdict fail-closed — the proposal provenance only widens it).
func TestCapabilityBlockedBoundaryWithoutAnyFamilySourceStaysUnresolved(t *testing.T) {
	s := testContinuationServer()
	now := time.Now().UTC()
	loop := freeStateReasoningLoop{
		SchemaVersion: freeStateReasoningLoopSchema, LoopID: "loop-cb-unresolved",
		ConversationID: "conversation-cb-unresolved", GoalID: "goal-cb-unresolved", RunID: "run-cb-unresolved",
		OriginalIntent: "处理一下这条轨", CreatedAt: now, UpdatedAt: now,
		Status:    "blocked",
		TargetRef: map[string]any{"kind": "track", "id": "1017", "track_id": "1017"},
		LatestDecision: &agentloop.FreeStateDecision{
			Status:  agentloop.FreeStateCapabilityBlocked,
			Summary: "no resolvable control family in the decision",
		},
	}
	s.storeFreeStateLoop(loop)
	res := agentloop.Result{GoalID: loop.GoalID, RunID: loop.RunID, Status: agentruntime.StatusCompleted, Reply: "模型原文"}
	resp, blocked := s.capabilityBlockedBoundaryResponse(loop.ConversationID, "default", res, loop)
	if !blocked {
		t.Fatal("model-terminal boundary was not detected")
	}
	boundary := boundaryFromWorkflowData(t, resp)
	if boundary.Family != "" {
		t.Fatalf("family must stay unresolved without any source, got %q", boundary.Family)
	}
	if !containsString(boundary.UnavailableReasons, capabilityboundary.ReasonFamilyUnresolved) {
		t.Fatalf("family-unresolved disclosure missing: %v", boundary.UnavailableReasons)
	}
	if len(boundary.NextActions) != 0 {
		t.Fatalf("an unresolved-family boundary must offer no actions: %+v", boundary.NextActions)
	}
}

func TestC2BoundaryResponseCarriesBaseRemediationPayload(t *testing.T) {
	goal := agentruntime.Goal{GoalID: "goal-c2", RunID: "run-c2"}
	resp := c2BoundaryResponse("conversation-c2", goal, "session-c2", "stale_project_cut", errString("C2 load batch project state changed"))
	boundary := boundaryFromWorkflowData(t, resp)
	if boundary.Class != capabilityboundary.ClassDynamicControl {
		t.Fatalf("boundary class = %q, want c2_dynamic_control", boundary.Class)
	}
	if boundary.Code != "stale_project_cut" {
		t.Fatalf("boundary code = %q, want stale_project_cut", boundary.Code)
	}
	// An infrastructure boundary owns no family facts: the payload must stay
	// empty-handed and honest rather than fabricating options.
	if len(boundary.NextActions) != 0 {
		t.Fatalf("infra boundary must not fabricate next actions: %+v", boundary.NextActions)
	}
	if len(boundary.UnavailableReasons) == 0 {
		t.Fatal("infra boundary must disclose why no options exist")
	}
	// The pre-existing C2 boundary face stays intact.
	if resp.StopReason != "stale_project_cut" || resp.WorkflowData["stage"] != "stale_project_cut" || resp.WorkflowData["mutation_performed"] != false {
		t.Fatalf("C2 boundary face changed: %+v", resp.WorkflowData)
	}
	if resp.WorkflowData["capability_id"] != dynamicControlCapabilityID {
		t.Fatalf("C2 capability id dropped: %+v", resp.WorkflowData)
	}
}

func TestC2BoundaryResponseWithRemediationKeepsOrdering(t *testing.T) {
	goal := agentruntime.Goal{GoalID: "goal-c2b", RunID: "run-c2b"}
	enriched := capabilityboundary.Resolve(capabilityboundary.Input{
		Code: "no_pca_admitted_dynamic_plugin_candidate", Class: capabilityboundary.ClassDynamicControl,
		Family: "reverb", Reason: "no locally loadable PCA-admitted reverb candidate",
		Native:        &capabilityboundary.NativeOption{Family: "reverb"},
		Certification: &capabilityboundary.CertificationEntry{Family: "reverb", Endpoint: "/e", ConsentToken: processorCertificationConsent},
	})
	resp := c2BoundaryResponse("conversation-c2b", goal, "session-c2b", "no_pca_admitted_dynamic_plugin_candidate", errString("no locally loadable PCA-admitted reverb candidate"), enriched)
	boundary := boundaryFromWorkflowData(t, resp)
	if len(boundary.NextActions) != 2 {
		t.Fatalf("enriched payload lost actions on the C2 face: %+v", boundary.NextActions)
	}
	if boundary.NextActions[0].Kind != capabilityboundary.KindNativeProcessor || boundary.NextActions[1].Kind != capabilityboundary.KindCertificationEntry {
		t.Fatalf("C2 enriched payload broke the fixed ordering: %+v", boundary.NextActions)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
