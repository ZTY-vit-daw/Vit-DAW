package frequencycleanup

import (
	"testing"
	"time"
)

func testFrequencyProjection(tap string, postFX bool) map[string]any {
	return map[string]any{
		"observation_id": "obs_before",
		"frequency_relationship": map[string]any{
			"schema_version": "mom.frequency_relationship.v1", "status": "ready", "freshness": "fresh", "project_cut_ref": "cut:1", "tap_point": tap,
			"scope":    map[string]any{"project_id": "p1", "track_ids": []string{"t1", "t2"}},
			"coverage": map[string]any{"project_track_count": 2, "eligible_track_count": 2, "missing_track_count": 0, "eligible_track_ratio": 1.0, "supports_static_diagnosis": true, "supports_post_fx_compare": postFX},
			"track_profiles": []map[string]any{
				{"track_id": "t1", "name": "Kick", "status": "ready", "freshness": "fresh", "tap_point": tap, "bands": map[string]any{"bass": map[string]any{"unit_energy": .7}}, "evidence_ref": "dad:t1"},
				{"track_id": "t2", "name": "Bass", "status": "ready", "freshness": "fresh", "tap_point": tap, "bands": map[string]any{"bass": map[string]any{"unit_energy": .6}}, "evidence_ref": "dad:t2"},
			},
			"conflict_candidates": []map[string]any{{"conflict_id": "conflict_bass", "region": "bass", "track_ids": []string{"t1", "t2"}, "confidence": "low_to_medium"}},
			"tonal_tendencies":    []map[string]any{}, "persistence_summary": map[string]any{"status": "missing"},
			"verification_dimensions": []string{"tap_point", "track_coverage"}, "evidence_refs": []string{"obs:before"},
		},
	}
}

func TestBuildModelSeparatesDiagnosisAndMutationReadiness(t *testing.T) {
	projection := testFrequencyProjection("source_file_pre_fx", false)
	model := BuildModel(Input{UserIntent: "clean frequencies", MOMProjection: projection, MixObservation: map[string]any{"observation_id": "obs_before"}, GeneratedAt: time.Unix(1, 0)})
	if !model.Readiness.Diagnosis.CanProceed {
		t.Fatalf("diagnosis should proceed: %+v", model.Readiness.Diagnosis)
	}
	if model.Readiness.Mutation.CanProceed {
		t.Fatalf("pre-fx evidence must not authorize mutation readiness")
	}
	if len(model.Tracks) != 2 || len(model.Candidates) != 1 {
		t.Fatalf("unexpected model: %+v", model)
	}

	post := BuildModel(Input{MOMProjection: testFrequencyProjection("track_post_fader", true), MixObservation: map[string]any{"observation_id": "obs_post"}})
	if !post.Readiness.Planning.CanProceed || post.Readiness.Mutation.CanProceed {
		t.Fatalf("whole-project evidence should enable planning but target selection must precede mutation: %+v", post.Readiness)
	}
}

func TestFinalizeTreatmentPlanRequiresExplicitWholeProjectClassification(t *testing.T) {
	model := BuildModel(Input{MOMProjection: testFrequencyProjection("track_post_fader", true), MixObservation: map[string]any{"observation_id": "obs_before"}})
	plan, err := FinalizeTreatmentPlan(TreatmentPlan{SchemaVersion: TreatmentPlanSchema, Summary: "clean only justified overlap", Items: []TreatmentItem{
		{Order: 1, TrackID: "t1", Classification: TreatmentStaticEQ, DiagnosisRefs: []string{"conflict_bass"}, ListeningGoal: "reduce bass overlap while preserving punch", Rationale: "observable overlap candidate"},
		{Order: 2, TrackID: "t2", Classification: TreatmentDeferredDynamic, Rationale: "problem varies over time"},
	}}, model)
	if err != nil {
		t.Fatal(err)
	}
	if len(StaticEQItems(plan)) != 1 || plan.PlanID == "" || plan.Items[1].Classification != TreatmentDeferredDynamic {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	bad := plan
	bad.SchemaVersion = TreatmentPlanSchema
	bad.Items = bad.Items[:1]
	if _, err = FinalizeTreatmentPlan(bad, model); err == nil {
		t.Fatal("omitted project track was accepted")
	}
	bad = plan
	bad.Items = append([]TreatmentItem(nil), plan.Items...)
	bad.Items[0].DiagnosisRefs = nil
	if _, err = FinalizeTreatmentPlan(bad, model); err == nil {
		t.Fatal("static_eq without exact diagnosis evidence was accepted")
	}
}

func TestFinalizeTreatmentPlanForcesDeterministicSilenceToNoChange(t *testing.T) {
	model := BuildModel(Input{MOMProjection: testFrequencyProjection("track_post_fader", true), MixObservation: map[string]any{"observation_id": "obs_before"}})
	model.Tracks[0].SilenceConfirmed = true
	model.Tracks[0].SilenceReason = "all_zero_source"

	staticPlan := TreatmentPlan{SchemaVersion: TreatmentPlanSchema, Summary: "classify all tracks", Items: []TreatmentItem{
		{Order: 1, TrackID: "t1", Classification: TreatmentStaticEQ, DiagnosisRefs: []string{"conflict_bass"}, ListeningGoal: "reduce bass overlap", Rationale: "candidate overlap"},
		{Order: 2, TrackID: "t2", Classification: TreatmentNoChange, Rationale: "leave unchanged"},
	}}
	if _, err := FinalizeTreatmentPlan(staticPlan, model); err == nil {
		t.Fatal("deterministically silent track was allowed to receive static EQ")
	}

	noChangePlan := staticPlan
	noChangePlan.Items = append([]TreatmentItem(nil), staticPlan.Items...)
	noChangePlan.Items[0] = TreatmentItem{Order: 1, TrackID: "t1", Classification: TreatmentNoChange, Rationale: "source is deterministically silent"}
	if _, err := FinalizeTreatmentPlan(noChangePlan, model); err != nil {
		t.Fatalf("deterministically silent track no_change classification was rejected: %v", err)
	}
}

func TestVerifyRelationshipsRequiresSameTapPostFX(t *testing.T) {
	before := BuildModel(Input{MOMProjection: testFrequencyProjection("track_post_fader", true)}).FrequencyRelationship
	after := before
	after.ConflictCandidates = nil
	after.EvidenceRefs = []string{"obs:after"}
	verified := VerifyRelationships(before, after, "before", "after")
	if verified.Status != "observed" || !verified.Comparable || !verified.PostFXComparable || len(verified.ChangedDimensions) == 0 || verified.UserAcceptance != "unknown" {
		t.Fatalf("unexpected verification: %+v", verified)
	}
	after.TapPoint = "source_file_pre_fx"
	blocked := VerifyRelationships(before, after, "before", "after")
	if blocked.Status != "inconclusive" || blocked.Comparable {
		t.Fatalf("tap mismatch should be inconclusive: %+v", blocked)
	}
}
