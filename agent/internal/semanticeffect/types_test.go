package semanticeffect

import "testing"

func ptr(value float64) *float64 { return &value }

func validAction() Action {
	return Action{
		SchemaVersion: ActionSchema, ActionType: ActionEQEdit, PayloadSchema: EQPlanSchema,
		Target: Target{TrackID: "track-1", PluginID: "plugin-1"}, UserGoal: "更亮但不要更刺耳",
		Constraints: []string{"不要增加刺耳感"},
		Evidence:    EvidenceDecision{Choice: "reuse", Basis: "both", Reason: "fresh selected-track evidence is available", ObservationID: "obs-1"},
		EQPlan: &EQPlan{SchemaVersion: EQPlanSchema, Atomic: true, Atoms: []EQAtom{
			{AtomID: "air", Action: "upsert", Shape: "high_shelf", FrequencyHz: ptr(9000), GainDB: ptr(1.5), Purpose: "增加空气感", Confidence: "medium", FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected"}},
			{AtomID: "edge", Action: "upsert", Shape: "bell", FrequencyHz: ptr(3500), GainDB: ptr(-0.75), Q: ptr(1.2), Purpose: "限制存在感区域的锐度", Confidence: "medium", FieldOrigins: map[string]string{"frequency_hz": "llm_selected", "gain_db": "llm_selected", "q": "llm_selected"}},
		}},
	}
}

func TestActionValidateAcceptsCoupledEQPlan(t *testing.T) {
	if err := validAction().Validate(); err != nil {
		t.Fatalf("valid coupled action rejected: %v", err)
	}
}

func TestActionValidateRejectsMissingFieldOriginAndNonAtomicPlan(t *testing.T) {
	action := validAction()
	delete(action.EQPlan.Atoms[0].FieldOrigins, "gain_db")
	if err := action.Validate(); err == nil {
		t.Fatal("missing field origin was accepted")
	}
	action = validAction()
	action.EQPlan.Atomic = false
	if err := action.Validate(); err == nil {
		t.Fatal("non-atomic EQ plan was accepted")
	}
}

func TestActionValidateAllowsUserReportWithoutObservation(t *testing.T) {
	action := validAction()
	action.Evidence = EvidenceDecision{Choice: "not_needed", Basis: "user_report", Reason: "the user requested a conservative direct tonal move"}
	if err := action.Validate(); err != nil {
		t.Fatalf("user-report evidence decision rejected: %v", err)
	}
}

func TestActionValidateRejectsObservationClaimWithoutReference(t *testing.T) {
	action := validAction()
	action.Evidence = EvidenceDecision{Choice: "reuse", Basis: "observation", Reason: "claimed observation"}
	if err := action.Validate(); err == nil {
		t.Fatal("unreferenced observation claim was accepted")
	}
}

func TestBatchValidateKeepsOrdinaryActionsAtomicAndExact(t *testing.T) {
	first := validAction()
	second := validAction()
	second.Target = Target{TrackID: "track-2", PluginID: "plugin-2"}
	batch := Batch{SchemaVersion: BatchSchema, ProjectGoal: "resolve the project low-end relationship", Atomic: true, Actions: []Action{first, second}}
	if err := batch.Validate(); err != nil {
		t.Fatalf("valid project batch rejected: %v", err)
	}
	batch.Actions[1].Target = batch.Actions[0].Target
	if err := batch.Validate(); err == nil {
		t.Fatal("duplicate exact leaf target was accepted")
	}
}
