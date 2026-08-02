package lowendrelation

import (
	"testing"
)

func TestBuildModelWithBandOccupancy(t *testing.T) {
	in := Input{
		MOMProjection: map[string]any{
			"observation_id": "obs_test_1",
			"multitrack_relation": map[string]any{
				"band_occupancy": []map[string]any{
					{
						"band": "sub",
						"leaders": []map[string]any{
							{"track_id": "kick_1", "track_name": "Kick", "unit_energy": 0.82, "role_guess": "kick"},
							{"track_id": "bass_1", "track_name": "Bass", "unit_energy": 0.58, "role_guess": "bass"},
						},
					},
					{
						"band": "bass",
						"leaders": []map[string]any{
							{"track_id": "bass_1", "track_name": "Bass", "unit_energy": 0.91, "role_guess": "bass"},
							{"track_id": "kick_1", "track_name": "Kick", "unit_energy": 0.62, "role_guess": "kick"},
						},
					},
				},
				"band_conflict_candidates": []map[string]any{
					{
						"band":       "bass",
						"confidence": "low_to_medium",
						"tracks": []map[string]any{
							{"track_id": "bass_1"},
							{"track_id": "kick_1"},
						},
					},
				},
			},
			"project_mix_profile": map[string]any{
				"band_tendency": map[string]any{
					"low_end": map[string]any{"state": "prominent"},
				},
			},
		},
	}

	model := BuildModel(in)

	if !model.Readiness.CanProceed {
		t.Fatalf("readiness should allow analysis with band data; blocked_by=%v", model.Readiness.BlockedBy)
	}
	if len(model.Tracks) == 0 {
		t.Fatal("expected low-end tracks from band occupancy")
	}
	if len(model.Conflicts) == 0 {
		t.Fatal("expected bass conflict candidate")
	}
	if model.Conflicts[0].ID == "" {
		t.Fatal("expected stable conflict identity for treatment handoff")
	}
	if model.Summary.LowEndTendency != "prominent" {
		t.Errorf("tendency = %q, want prominent", model.Summary.LowEndTendency)
	}
	if model.Summary.ConflictCount != 1 {
		t.Errorf("conflict count = %d, want 1", model.Summary.ConflictCount)
	}
	if len(model.Observations) == 0 {
		t.Fatal("expected at least one observation")
	}
}

func TestBuildModelNoMOMData(t *testing.T) {
	model := BuildModel(Input{})
	if model.Readiness.CanProceed {
		t.Fatal("readiness should block when no band occupancy data is available")
	}
	if len(model.Tracks) != 0 {
		t.Errorf("expected no tracks, got %d", len(model.Tracks))
	}
}

func TestBuildModelKeepsCompleteDecisionRosterSeparateFromConflictMembers(t *testing.T) {
	decisionTracks := []map[string]any{
		{"track_id": "t1", "unit_energy": 0.9},
		{"track_id": "t2", "unit_energy": 0.8},
		{"track_id": "t3", "unit_energy": 0.7},
		{"track_id": "t4", "unit_energy": 0.6},
		{"track_id": "t5", "unit_energy": 0.5},
	}
	model := BuildModel(Input{MOMProjection: map[string]any{
		"multitrack_relation": map[string]any{
			"band_occupancy": []map[string]any{{
				"band": "bass", "leaders": decisionTracks[:3], "decision_tracks": decisionTracks,
			}},
			"band_conflict_candidates": []map[string]any{{
				"band": "bass", "tracks": decisionTracks[:3], "decision_tracks": decisionTracks,
			}},
		},
	}})
	if got := len(model.Tracks); got != 5 {
		t.Fatalf("model tracks = %d, want all 5 decision tracks", got)
	}
	if got := len(model.Conflicts); got != 1 || len(model.Conflicts[0].Tracks) != 3 {
		t.Fatalf("model conflicts = %#v, want the three concrete conflict members only", model.Conflicts)
	}
}

func TestBuildModelTrackRoleEnrichment(t *testing.T) {
	in := Input{
		MOMProjection: map[string]any{
			"multitrack_relation": map[string]any{
				"band_occupancy": []map[string]any{
					{
						"band": "sub",
						"leaders": []map[string]any{
							{"track_id": "t1", "unit_energy": 0.70, "role_guess": "kick"},
						},
					},
				},
			},
		},
		TOMProjection: map[string]any{
			"group_proposals": map[string]any{
				"assignment_excerpt": []map[string]any{
					{"track_id": "t1", "role_guess": "kick"},
				},
			},
		},
	}
	model := BuildModel(in)
	if len(model.Tracks) == 0 {
		t.Fatal("expected tracks")
	}
	track := model.Tracks[0]
	if track.Role != "kick" {
		t.Errorf("role = %q, want kick", track.Role)
	}
	if track.RoleSource != "tom_assignment" {
		t.Errorf("role_source = %q, want tom_assignment", track.RoleSource)
	}
}

func TestAnalyzeProducesResult(t *testing.T) {
	in := Input{
		MOMProjection: map[string]any{
			"observation_id": "obs_analyze_1",
			"multitrack_relation": map[string]any{
				"band_occupancy": []map[string]any{
					{"band": "bass", "leaders": []map[string]any{
						{"track_id": "bass_1", "unit_energy": 0.75},
					}},
				},
			},
		},
	}
	model := BuildModel(in)
	result := Analyze(model)
	if result.CapabilityID != CapabilityID {
		t.Errorf("capability_id = %q, want %q", result.CapabilityID, CapabilityID)
	}
	if result.AnalyzedTrackCount == 0 {
		t.Error("expected analyzed track count > 0")
	}
}
