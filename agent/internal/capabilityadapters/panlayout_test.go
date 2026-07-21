package capabilityadapters

import (
	"testing"
	"time"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/orchestration"
)

func TestPanLayoutAdapterFreezesCompleteSolverActionSet(t *testing.T) {
	cut := orchestration.ProjectCut{ProjectUUID: "p1", ProjectEpoch: "e1", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	planned, err := PlanPanLayout(PanLayoutPlanRequest{
		SessionID: "s1", Goal: "build a pan layout", Mode: orchestration.InteractionPropose, ProjectCut: cut,
		Input: panLayoutAdapterInput(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || len(planned.CandidateIDs) == 0 {
		t.Fatalf("plan=%#v readiness=%#v", planned.Outcome, planned.Pack.Readiness)
	}
	candidateID := planned.CandidateIDs[0]
	actionSet, err := PanLayoutActionSet(planned.Pack, cut, candidateID)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := FreezePanLayoutProposal(planned.Pack, cut, candidateID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if actionSet.Hash != proposal.ActionSetHash || actionSet.ProjectCutHash != cut.Hash || len(actionSet.Actions) == 0 {
		t.Fatalf("proposal/action set mismatch: proposal=%#v actions=%#v", proposal, actionSet)
	}
	for _, action := range actionSet.Actions {
		if action.Command != "track_pan_set" || action.BeforeFingerprint == "" || action.Args["target_pan"] == nil {
			t.Fatalf("unguarded B3 action: %#v", action)
		}
	}
	if planned.Bundle.Disclosure == "" || len(planned.Bundle.ArtifactRefs) == 0 {
		t.Fatalf("B3 bundle did not separate disclosure and full pack ref: %#v", planned.Bundle)
	}
}

func panLayoutAdapterInput() capabilitycontext.PanLayoutInput {
	tracks := []any{
		map[string]any{"track_id": "g1", "track_name": "Guitar L", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "g2", "track_name": "Guitar R", "track_type": "audio", "is_audio_track": true, "pan": 0.0, "channel_count": 1},
		map[string]any{"track_id": "v1", "track_name": "Lead Vocal", "track_type": "audio", "is_audio_track": true, "pan": 0.2, "channel_count": 1},
	}
	groups := []any{
		map[string]any{"role": "guitar", "confidence": "high", "track_ids": []any{"g1", "g2"}},
		map[string]any{"role": "lead_vocal", "confidence": "high", "track_ids": []any{"v1"}},
	}
	return capabilitycontext.PanLayoutInput{
		ProjectState:  map[string]any{"tracks": tracks},
		MOMProjection: map[string]any{"multitrack_relation": map[string]any{"status": "ready"}},
		TOMProjection: map[string]any{"full_assignment_manifest": map[string]any{"groups": groups}},
		Style:         mixstyle.Default(), GeneratedAt: time.Unix(1, 0),
	}
}
