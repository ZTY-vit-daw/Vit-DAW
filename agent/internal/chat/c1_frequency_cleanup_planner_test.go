package chat

import (
	"testing"

	"vit-daw-agent/internal/capabilityruntime"
	"vit-daw-agent/internal/frequencycleanup"
)

func TestDecodeC1TreatmentPlanClassifiesEveryTrackAndDefersNonStaticWork(t *testing.T) {
	ready := capabilityruntime.Evaluate(frequencycleanup.CapabilityID, []capabilityruntime.Condition{{ID: "ready", Required: true, Status: capabilityruntime.ConditionReady}})
	model := frequencycleanup.Model{SchemaVersion: frequencycleanup.ModelSchemaVersion, ModelID: "model-c1", CapabilityID: frequencycleanup.CapabilityID, ObservationID: "obs-c1", Tracks: []frequencycleanup.TrackProfile{{TrackID: "t1", TrackName: "One"}, {TrackID: "t2", TrackName: "Two"}}, Candidates: []frequencycleanup.DiagnosisCandidate{{ID: "candidate-1", TrackIDs: []string{"t1"}}}, Readiness: frequencycleanup.Readiness{Diagnosis: ready, Mutation: ready}}
	text := `{"schema_version":"frequency_cleanup.treatment_plan.v1","summary":"classify project","items":[{"order":1,"track_id":"t1","classification":"static_eq","diagnosis_refs":["candidate-1"],"listening_goal":"reduce stable buildup","rationale":"static evidence"},{"order":2,"track_id":"t2","classification":"defer_dynamic_processing","rationale":"time-varying issue"}]}`
	plan, err := decodeC1TreatmentPlan(text, model)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Items) != 2 || len(frequencycleanup.StaticEQItems(plan)) != 1 || plan.Items[1].Classification != frequencycleanup.TreatmentDeferredDynamic {
		t.Fatalf("unexpected C1 plan: %+v", plan)
	}
	bad := `{"schema_version":"frequency_cleanup.treatment_plan.v1","summary":"bad","items":[{"order":1,"track_id":"t1","classification":"static_eq","listening_goal":"x","rationale":"x"}]}`
	if _, err = decodeC1TreatmentPlan(bad, model); err == nil {
		t.Fatal("partial project plan was accepted")
	}
}
