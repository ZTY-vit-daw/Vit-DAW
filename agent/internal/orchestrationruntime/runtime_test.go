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
			"multitrack_relation": map[string]any{"compared_tracks": []any{
				map[string]any{"track_id": "v", "rms_dbfs": -18.0, "headroom_db": 8.0},
				map[string]any{"track_id": "d", "rms_dbfs": -20.0, "headroom_db": 6.0},
				map[string]any{"track_id": "b", "rms_dbfs": -22.0, "headroom_db": 7.0},
			}},
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

func mustStyle(t *testing.T) mixstyle.MixStyle {
	t.Helper()
	style, err := mixstyle.Builtin("modern_pop")
	if err != nil {
		t.Fatal(err)
	}
	return style
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
