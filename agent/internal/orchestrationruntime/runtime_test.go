package orchestrationruntime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/capabilityadapters"
	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/orchestration"
)

func TestB2RuntimeShadowToProposalDoesNotMutateProject(t *testing.T) {
	runtime := New()
	session, err := runtime.StartB2Session("session-1", "project-1", "inspect static balance", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := runtime.ShadowB2(session.ID, orchestration.ProjectCut{
		ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong",
		DependencyFingerprints: []string{"project.state:tracks", "MOM:obs-1", "TOM:tom-1"},
	}, capabilitycontext.StaticBalanceInput{
		GeneratedAt: time.Unix(1700000000, 0), Style: mustStyle(t),
		ProjectState: map[string]any{"tracks": []any{
			map[string]any{"track_id": "v", "track_name": "Vocal", "volume_db": 0.0},
			map[string]any{"track_id": "d", "track_name": "Drums", "volume_db": 0.0},
			map[string]any{"track_id": "b", "track_name": "Bass", "volume_db": 0.0},
		}},
		ContextSnapshot: map[string]any{"tom_projection": map[string]any{"group_proposals": []any{
			map[string]any{"role_hypothesis": "lead_vocal", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "v"}}},
			map[string]any{"role_hypothesis": "drums", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "d"}}},
			map[string]any{"role_hypothesis": "bass", "confidence": "high", "assignment_excerpt": []any{map[string]any{"track_id": "b"}}},
		}}},
		MixObservation: map[string]any{"observation_id": "obs-1", "mom_projection": map[string]any{
			"trust_quality": map[string]any{"can_support_action_preflight": true},
			"multitrack_relation": map[string]any{"status": "ready", "compared_tracks": []any{
				map[string]any{"track_id": "v", "rms_dbfs": -18.0, "headroom_db": 8.0},
				map[string]any{"track_id": "d", "rms_dbfs": -20.0, "headroom_db": 6.0},
				map[string]any{"track_id": "b", "rms_dbfs": -22.0, "headroom_db": 7.0},
			}},
			"static_level_relationship": map[string]any{
				"schema_version": "mom.static_level_relationship.v1", "status": "ready", "freshness": "fresh", "project_cut_ref": "cut-1",
				"tracks": []any{
					runtimeStaticLevel("v", -18, -8), runtimeStaticLevel("d", -20, -6), runtimeStaticLevel("b", -22, -7),
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || len(planned.CandidateIDs) == 0 {
		t.Fatalf("unexpected shadow result: %#v", planned.Outcome)
	}
	proposal, err := capabilityadapters.FreezeStaticBalanceProposal(planned.Pack, orchestration.ProjectCut{
		ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong",
		DependencyFingerprints: []string{"project.state:tracks", "MOM:obs-1", "TOM:tom-1"},
	}, planned.CandidateIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{
		ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong",
		DependencyFingerprints: []string{"project.state:tracks", "MOM:obs-1", "TOM:tom-1"},
	}
	cut.Hash = cut.ComputeHash()
	actionSet, err := capabilityadapters.StaticBalanceActionSet(planned.Pack, cut, planned.CandidateIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	// Re-freeze against the exact persisted Cut so Proposal and ActionSet share
	// one immutable authorization boundary.
	proposal, err = capabilityadapters.FreezeStaticBalanceProposal(planned.Pack, cut, planned.CandidateIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := runtime.AttachFrozenPlan(session.ID, orchestration.FrozenPlan{
		Proposal: proposal, ActionSet: actionSet, ProjectCut: cut, ContextBundleID: planned.Bundle.ID,
		PreviousObservationID: planned.Pack.Result().ObservationID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != orchestration.StatusWaiting || updated.ActiveProposal == nil {
		t.Fatalf("proposal was not persisted as planning state: %#v", updated)
	}
	if updated.Authorization != nil {
		t.Fatal("shadow/planning path must not create authorization")
	}
	decision := orchestration.ApprovalDecision{
		SchemaVersion: orchestration.ApprovalDecisionSchema, Kind: orchestration.ApprovalApprove,
		ProposalID: updated.ActiveProposal.ID, ProposalRevision: updated.ActiveProposal.Revision,
		ActionSetHash: updated.ActiveProposal.ActionSetHash, ProjectCutHash: updated.ActiveProposal.ProjectCutHash, SourceTurnID: "turn-1",
	}
	authorized, err := runtime.AuthorizeProposal(session.ID, orchestration.Authorization{
		ProposalID: updated.ActiveProposal.ID, ProposalRevision: updated.ActiveProposal.Revision,
		ActionSetHash: updated.ActiveProposal.ActionSetHash, ProjectCutHash: updated.ActiveProposal.ProjectCutHash,
		SourceTurnID: "turn-1", Sequence: 1, Decision: &decision,
	})
	if err != nil {
		t.Fatal(err)
	}
	if authorized.Status != orchestration.StatusAuthorized || authorized.Authorization == nil {
		t.Fatalf("authorization was not persisted: %#v", authorized)
	}
}

func runtimeStaticLevel(trackID string, rms, peak float64) map[string]any {
	return map[string]any{
		"track_id": trackID, "status": "ready", "freshness": "fresh", "metric": "effective_static_rms_dbfs",
		"tap_point": "derived_static_control_model", "effective_static_rms_dbfs": rms, "effective_static_peak_dbfs": peak,
		"aggregation_method": "duration_weighted_linear_energy_source_plus_clip_gain_plus_fader",
	}
}

func mustStyle(t *testing.T) mixstyle.MixStyle {
	t.Helper()
	style, err := mixstyle.Builtin("modern_pop")
	if err != nil {
		t.Fatal(err)
	}
	return style
}

func TestC1RuntimeRegistersSessionAndSeparatesDiagnosisFromMutationReadiness(t *testing.T) {
	runtime := New()
	session, err := runtime.StartC1ChatSession("cap_v1_c1_test_1", "chat-c1", "project-c1", "inspect C1", orchestration.InteractionInspect)
	if err != nil {
		t.Fatal(err)
	}
	if session.Invocation.CapabilityID != FrequencyCleanupCapabilityID || session.Invocation.CapabilityVer != "v1" {
		t.Fatalf("unexpected C1 invocation: %+v", session.Invocation)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-c1", ProjectEpoch: "epoch", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	projection := map[string]any{"observation_id": "obs-c1", "frequency_relationship": map[string]any{
		"schema_version": "mom.frequency_relationship.v1", "status": "ready", "freshness": "fresh", "project_cut_ref": "cut", "tap_point": "source_file_pre_fx",
		"scope":               map[string]any{"project_id": "project-c1", "track_ids": []string{"t1"}},
		"coverage":            map[string]any{"project_track_count": 1, "eligible_track_count": 1, "missing_track_count": 0, "eligible_track_ratio": 1.0, "supports_static_diagnosis": true, "supports_post_fx_compare": false},
		"track_profiles":      []map[string]any{{"track_id": "t1", "name": "Track", "status": "ready", "freshness": "fresh", "tap_point": "source_file_pre_fx", "bands": map[string]any{"mid": map[string]any{"unit_energy": .5}}}},
		"persistence_summary": map[string]any{"status": "missing"},
	}}
	planned, err := runtime.ShadowC1(session.ID, cut, capabilitycontext.FrequencyCleanupInput{UserIntent: "inspect C1", MOMProjection: projection, MixObservation: map[string]any{"observation_id": "obs-c1"}})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeAnalysis || !planned.Pack.Readiness.Diagnosis.CanProceed || planned.Pack.Readiness.Mutation.CanProceed {
		t.Fatalf("unexpected C1 readiness/outcome: %+v %+v", planned.Outcome, planned.Pack.Readiness)
	}
}

func TestC2RuntimeStartsAsIndependentCapability(t *testing.T) {
	runtime := New()
	session, err := runtime.StartC2ChatSession("cap_v1_c2_test_1", "chat-c2", "project-c2", "dynamic control", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	if session.Invocation.CapabilityID != DynamicControlCapabilityID || session.Invocation.CapabilityVer != "v1" || session.Invocation.ProcessingPath != orchestration.PathCapability {
		t.Fatalf("unexpected C2 invocation: %#v", session.Invocation)
	}
	if len(session.Constraints) == 0 || session.Constraints[0] != "independent_capability_no_c1_prerequisite" {
		t.Fatalf("C2 independence constraint missing: %#v", session.Constraints)
	}
}

func TestC2ExternalLeafFinalizesAsNeedsReviewWithReceipt(t *testing.T) {
	runtime := New()
	session, err := runtime.StartC2ChatSession("cap_v1_c2_finalize", "chat-c2", "project-c2", "dynamic control", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-c2", ProjectEpoch: "epoch", BaseProjectRevision: "1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet := orchestration.ActionSet{ID: "c2-action", CapabilityID: DynamicControlCapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{{ID: "leaf", Command: "semantic_dynamic_control.pending", TargetRef: "plugin:t:p", Compensatable: true}}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{ID: "c2-proposal", Revision: 1, CapabilityID: DynamicControlCapabilityID, CapabilityVer: "v1", ProjectCutHash: cut.Hash, ActionSetHash: actionSet.Hash, TargetScope: []string{"track:t"}}
	if _, err := runtime.AttachFrozenPlan(session.ID, orchestration.FrozenPlan{Proposal: proposal, ActionSet: actionSet, ProjectCut: cut}); err != nil {
		t.Fatal(err)
	}
	final, err := runtime.FinalizeExternalLeaf(session.ID, []orchestration.ActionReceipt{{ActionID: "leaf", Status: "applied", EffectivelyOnce: true}}, orchestration.VerificationResult{Status: "inconclusive", Structural: "pass", Acoustic: "inconclusive"})
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != orchestration.StatusNeedsReview || final.Execution == nil || final.Execution.ReceiptRef == "" || len(final.Execution.Receipts) != 1 {
		t.Fatalf("unexpected C2 terminal record: %#v", final)
	}
}

func TestRuntimeBuildsBoundedB2ContextEnvelope(t *testing.T) {
	runtime := New()
	session, err := runtime.StartB2Session("s-context", "p-context", "balance", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	turns := make([]orchestration.ContextEntry, 0, 1002)
	for index := 0; index < 1000; index++ {
		turns = append(turns, orchestration.ContextEntry{ID: fmt.Sprintf("old-%d", index), Content: strings.Repeat("old ", 100), Reference: fmt.Sprintf("history:%d", index)})
	}
	turns = append(turns,
		orchestration.ContextEntry{ID: "recent-1", Content: "select candidate"},
		orchestration.ContextEntry{ID: "recent-2", Content: "confirm proposal"},
	)
	envelope, err := runtime.BuildB2ContextEnvelope(session.ID, orchestration.ContextBundle{
		ID: "bundle", CapabilityID: StaticBalanceCapabilityID, Disclosure: `{"candidate_ids":["one"]}`,
		ArtifactRefs: []string{"capability-pack:bundle"}, Omissions: map[string]orchestration.OmissionStatus{"full_model": orchestration.OmissionByReference},
	}, []orchestration.ContextEntry{{ID: "mix.observe", Content: "read-only observation schema", Priority: 100}}, turns,
		orchestration.ContextWindowBudget{MaxWindowTokens: 1200, OutputReserveTokens: 200, MaxRegistryEntries: 2, MaxToolSchemas: 2, MaxConversationTurns: 2})
	if err != nil || !envelope.Valid || envelope.EstimatedInputTokens > envelope.MaxInputTokens {
		t.Fatalf("envelope=%#v err=%v", envelope, err)
	}
	if len(envelope.ConversationTurns) != 2 || envelope.ConversationTurns[0].ID != "recent-1" {
		t.Fatalf("long history entered model context: %#v", envelope.ConversationTurns)
	}
	if envelope.Omissions["conversation.older_turns"] != orchestration.OmissionByReference || envelope.Omissions["bundle.full_model"] != orchestration.OmissionByReference {
		t.Fatalf("omission manifest=%#v", envelope.Omissions)
	}
}

func TestRuntimeStartsB3UnderSameFixedOwnerContract(t *testing.T) {
	runtime := New()
	session, err := runtime.StartB3Session("s-b3", "p-b3", "pan layout", orchestration.InteractionPropose)
	if err != nil {
		t.Fatal(err)
	}
	if session.EngineOwner != orchestration.EngineV1 || session.Invocation.CapabilityID != PanLayoutCapabilityID || session.Invocation.ProcessingPath != orchestration.PathCapability {
		t.Fatalf("B3 bypassed shared session contract: %#v", session)
	}
}
