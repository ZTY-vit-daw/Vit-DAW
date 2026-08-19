package dynamiccontrol

import (
	"testing"

	"vit-daw-agent/internal/processorintent"
)

func TestReferralIsRevisionBoundAndCannotBeEmpty(t *testing.T) {
	referral := Referral{
		SchemaVersion: ReferralSchema, SourceCapabilityID: "fine_mix.frequency_cleanup.v1", SourcePlanID: "c1-plan", ProjectRevision: "42",
		Targets: []ReferralTarget{{TrackID: "vocal", Rationale: "level varies with phrases", EvidenceRefs: []string{"obs:1", "obs:1"}}},
	}
	if err := referral.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(referral.Targets[0].EvidenceRefs) != 1 {
		t.Fatalf("referral evidence was not normalized: %#v", referral)
	}
	referral.ProjectRevision = ""
	if err := referral.Validate(); err == nil {
		t.Fatal("unbound referral was accepted")
	}
}

func TestTreatmentPlanRequiresFrozenLeafIdentityAndCoverage(t *testing.T) {
	plan := TreatmentPlan{SchemaVersion: TreatmentPlanSchema, PlanID: "c2-plan", ProjectCutHash: "cut", ObservationID: "obs", Targets: []TreatmentTarget{{TrackID: "vocal", PluginID: "comp", ProcessorFamily: "broadband_compressor", ListeningGoal: "steady lead vocal", RequiredCoverage: []string{"activation_intensity"}}}}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	plan.Targets[0].RequiredCoverage = nil
	if err := plan.Validate(); err == nil {
		t.Fatal("plan without PCA coverage was accepted")
	}
}

func TestTargetHypothesisCannotBindPluginAndRequiresCandidateFamily(t *testing.T) {
	hypothesis := TargetHypothesis{
		SchemaVersion: HypothesisSchema, Status: HypothesisTargeted, TrackID: "vocal",
		ListeningGoal: "reduce unstable peaks", Rationale: "time evidence shows repeatable macro excursions", EvidenceRefs: []string{"obs-vocal"},
		Intent: processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved, Family: processorintent.FamilyBroadbandCompressor, Intent: "make vocal level steadier", RequiredCoverage: []string{"activation_intensity"}, Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .8, EvidenceRefs: []string{"obs-vocal"}},
	}
	if err := hypothesis.Validate(map[string]map[string]bool{"vocal": {processorintent.FamilyBroadbandCompressor: true}}); err != nil {
		t.Fatalf("valid target hypothesis: %v", err)
	}
	hypothesis.Intent.Family = processorintent.FamilyLimiter
	if err := hypothesis.Validate(map[string]map[string]bool{"vocal": {processorintent.FamilyBroadbandCompressor: true}}); err == nil {
		t.Fatal("hypothesis accepted a family with no candidate surface")
	}
}

func TestProjectTreatmentKeepsPluginResolutionAfterMultiTargetObservation(t *testing.T) {
	makeTarget := func(trackID, family string) TargetHypothesis {
		return TargetHypothesis{
			SchemaVersion: HypothesisSchema, Status: HypothesisTargeted, TrackID: trackID,
			ListeningGoal: "control repeatable dynamic excursions", Rationale: "project dynamic evidence",
			EvidenceRefs: []string{"obs-" + trackID},
			Intent: processorintent.Intent{SchemaVersion: processorintent.SchemaVersion, Status: processorintent.StatusResolved,
				Family: family, Intent: "apply bounded dynamic treatment", RequiredCoverage: []string{"activation_intensity"},
				Scope: processorintent.ScopeCurrentTrack, ControlMode: processorintent.ControlModeSemantic, Confidence: .7,
				EvidenceRefs: []string{"obs-" + trackID}},
		}
	}
	plan := ProjectTreatment{SchemaVersion: ProjectPlanSchema, Status: HypothesisTargeted, Summary: "two project targets", ObservationID: "obs-project",
		Targets: []TargetHypothesis{makeTarget("drums", processorintent.FamilyBroadbandCompressor), makeTarget("bass", processorintent.FamilyBroadbandCompressor)}}
	families := map[string]map[string]bool{
		"drums": {processorintent.FamilyBroadbandCompressor: true},
		"bass":  {processorintent.FamilyBroadbandCompressor: true},
	}
	if err := plan.Validate(families); err != nil {
		t.Fatalf("multi-target project treatment: %v", err)
	}
	if plan.Targets[0].TrackID == "" || plan.Targets[0].Intent.Family == "" {
		t.Fatal("project treatment lost target identity")
	}
	if _, ok := any(plan.Targets[0]).(TreatmentTarget); ok {
		t.Fatal("project treatment must not bind a plugin before PCA resolution")
	}
}
