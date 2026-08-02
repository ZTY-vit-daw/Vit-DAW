package capabilityadapters

import (
	"testing"
	"time"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/mixstyle"
	"vit-daw-agent/internal/orchestration"
)

func TestStaticBalanceAdapterPlansWithoutMutationAndFreezesExistingCandidate(t *testing.T) {
	request := StaticBalancePlanRequest{
		SessionID: "session-b2",
		Goal:      "inspect static balance",
		Mode:      orchestration.InteractionPropose,
		ProjectCut: orchestration.ProjectCut{
			ProjectUUID: "project-1", ProjectEpoch: "epoch-1", Consistency: "strong",
			DependencyFingerprints: []string{"project.state:tracks", "MOM:obs-1", "TOM:tom-1"},
		},
		Input: capabilitycontext.StaticBalanceInput{
			GeneratedAt: time.Unix(1700000000, 0), Style: mustStyle(t),
			ProjectState: map[string]any{"tracks": []any{
				map[string]any{"track_id": "v", "track_name": "Lead Vocal", "volume_db": 0.0},
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
						adapterStaticLevel("v", -18, -8), adapterStaticLevel("d", -20, -6), adapterStaticLevel("b", -22, -7),
					},
				},
			}},
		},
	}
	planned, err := PlanStaticBalance(request)
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeProposal || len(planned.CandidateIDs) == 0 {
		t.Fatalf("unexpected planning outcome: %#v", planned.Outcome)
	}
	proposal, err := FreezeStaticBalanceProposal(planned.Pack, request.ProjectCut, planned.CandidateIDs[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.ActionSetHash == "" || proposal.ProjectCutHash == "" || len(proposal.TargetScope) == 0 {
		t.Fatalf("proposal was not frozen: %#v", proposal)
	}
	actionSet, err := StaticBalanceActionSet(planned.Pack, request.ProjectCut, planned.CandidateIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range actionSet.Actions {
		for _, key := range []string{"hierarchy_role", "hierarchy_function", "before_effective_level_db", "before_effective_level_source", "before_effective_level_metric", "before_effective_level_tap_point", "before_effective_level_status", "function_current_relative_db", "function_target_relative_db"} {
			if _, ok := action.Args[key]; !ok {
				t.Fatalf("action %s omitted hierarchy metadata %q: %#v", action.ID, key, action.Args)
			}
		}
	}
}

func adapterStaticLevel(trackID string, rms, peak float64) map[string]any {
	return map[string]any{
		"track_id": trackID, "status": "ready", "freshness": "fresh", "metric": "effective_static_rms_dbfs",
		"tap_point": "derived_static_control_model", "effective_static_rms_dbfs": rms, "effective_static_peak_dbfs": peak,
		"aggregation_method": "duration_weighted_linear_energy_source_plus_clip_gain_plus_fader",
	}
}

func TestStaticBalanceShadowRequiresV1SessionOwner(t *testing.T) {
	session, err := orchestration.NewSession("legacy-session", "project-1", "inspect static balance", orchestration.EngineLegacy, orchestration.CapabilityInvocation{
		CapabilityID:    "static_mix.static_balance.v0",
		InteractionMode: orchestration.InteractionInspect,
		ProcessingPath:  orchestration.PathCapability,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = RunStaticBalanceShadow(session, orchestration.ProjectCut{ProjectUUID: "p", ProjectEpoch: "e"}, capabilitycontext.StaticBalanceInput{})
	if err == nil {
		t.Fatal("legacy session must not enter v1 shadow path")
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
