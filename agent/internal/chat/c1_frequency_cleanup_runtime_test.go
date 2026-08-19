package chat

import (
	"testing"

	"vit-daw-agent/internal/frequencycleanup"
)

func TestC1DynamicReferralIsOptionalAndRevisionBound(t *testing.T) {
	plan := frequencycleanup.TreatmentPlan{PlanID: "c1-plan", EvidenceRefs: []string{"obs:project"}, Items: []frequencycleanup.TreatmentItem{
		{TrackID: "vocal", Classification: frequencycleanup.TreatmentDeferredDynamic, Rationale: "phrase-level level changes", Constraints: []string{"preserve transients"}, EvidenceRefs: []string{"obs:vocal"}},
		{TrackID: "bass", Classification: frequencycleanup.TreatmentStaticEQ, Rationale: "static buildup"},
	}}
	referral := c1DynamicControlReferral(plan, 42)
	if referral == nil || referral.SourcePlanID != "c1-plan" || referral.ProjectRevision != "42" || len(referral.Targets) != 1 || referral.Targets[0].TrackID != "vocal" {
		t.Fatalf("unexpected C1 referral: %#v", referral)
	}
	if got := c1DynamicControlReferral(plan, 0); got != nil {
		t.Fatalf("unbound referral was emitted: %#v", got)
	}
}

func TestC1MutationRequestedHonorsExplicitExecutionContext(t *testing.T) {
	if !c1MutationRequested("C1", map[string]any{"c1_execute": true}) {
		t.Fatal("explicit C1 execution context was ignored")
	}
	if c1MutationRequested("C1", map[string]any{"c1_execute": false}) {
		t.Fatal("plain C1 label should not imply mutation")
	}
}
