package lowendrelation

import "testing"

func TestFinalizeTreatmentPlanBindsWholeProjectTargets(t *testing.T) {
	model := BuildModel(Input{MOMProjection: map[string]any{
		"observation_id": "obs-treatment",
		"multitrack_relation": map[string]any{
			"band_occupancy": []map[string]any{{"band": "bass", "leaders": []map[string]any{
				{"track_id": "kick", "name": "Kick", "unit_energy": 0.9},
				{"track_id": "bass", "name": "Bass", "unit_energy": 0.8},
			}}},
			"band_conflict_candidates": []map[string]any{{"band": "bass", "tracks": []map[string]any{
				{"track_id": "kick"}, {"track_id": "bass"},
			}}},
		},
	}})
	conflictID := model.Conflicts[0].ID
	plan, err := FinalizeTreatmentPlan(TreatmentPlan{
		SchemaVersion: TreatmentPlanSchema,
		Summary:       "Preserve the kick anchor while reducing project low-end overlap.",
		Targets: []TreatmentTarget{
			{Order: 2, TrackID: "bass", RelationshipRefs: []string{conflictID}, ListeningGoal: "make room for the kick"},
			{Order: 1, TrackID: "kick", RelationshipRefs: []string{conflictID}, ListeningGoal: "keep the transient focused"},
		},
	}, model)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ObservationScope != "full_project" || plan.PlanID == "" || len(plan.Targets) != 2 {
		t.Fatalf("finalized plan = %#v", plan)
	}
	if plan.Targets[0].TrackID != "kick" || plan.Targets[1].TrackID != "bass" {
		t.Fatalf("targets were not ordered: %#v", plan.Targets)
	}
	if plan.Targets[0].TargetID == "" || plan.Targets[1].TargetID == "" {
		t.Fatalf("target identities missing: %#v", plan.Targets)
	}
}

func TestFinalizeTreatmentPlanRejectsInventedTrackOrRelationship(t *testing.T) {
	model := BuildModel(Input{MOMProjection: map[string]any{
		"multitrack_relation": map[string]any{"band_occupancy": []map[string]any{{"band": "sub", "leaders": []map[string]any{{"track_id": "kick", "unit_energy": 0.8}}}}},
	}})
	base := TreatmentPlan{SchemaVersion: TreatmentPlanSchema, Summary: "adjust", Targets: []TreatmentTarget{{Order: 1, TrackID: "invented", RelationshipRefs: []string{model.Observations[0].ID}, ListeningGoal: "adjust"}}}
	if _, err := FinalizeTreatmentPlan(base, model); err == nil {
		t.Fatal("invented track was accepted")
	}
	base.Targets[0].TrackID = "kick"
	base.Targets[0].RelationshipRefs = []string{"invented_relation"}
	if _, err := FinalizeTreatmentPlan(base, model); err == nil {
		t.Fatal("invented relationship was accepted")
	}
}
