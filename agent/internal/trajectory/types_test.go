package trajectory

import "testing"

func TestEventNormalizeAndValidate(t *testing.T) {
	event := Event{
		Type:           EventMaterialityEvaluated,
		ConversationID: " conversation-1 ",
		GoalID:         "goal-1",
		RunID:          "run-1",
		ItemID:         "materiality-1",
		Payload: Payload{
			RoundID:      "round-1",
			Materiality:  EvaluationInsufficientDose,
			EvidenceRefs: []string{" obs-1 ", "obs-1", ""},
		},
	}.Normalize()
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if event.Payload.SchemaVersion != SchemaVersion || event.Payload.TurnID != "run-1" {
		t.Fatalf("identity = %#v", event.Payload)
	}
	if event.Payload.NodeKind != NodeMateriality || event.Payload.Phase != "materiality_evaluating" || event.Payload.Status != StatusCompleted {
		t.Fatalf("defaults = %#v", event.Payload)
	}
	if len(event.Payload.EvidenceRefs) != 1 || event.Payload.EvidenceRefs[0] != "obs-1" {
		t.Fatalf("evidence refs = %#v", event.Payload.EvidenceRefs)
	}
}

func TestEventRejectsMissingRoundForRoundEvent(t *testing.T) {
	event := Event{Type: EventInterventionApplied, ConversationID: "conversation-1", RunID: "run-1"}
	if err := event.Validate(); err == nil {
		t.Fatal("expected missing round_id rejection")
	}
}

func TestEventRejectsUnsupportedEvaluationState(t *testing.T) {
	event := Event{
		Type:           EventSettled,
		ConversationID: "conversation-1",
		RunID:          "run-1",
		Payload:        Payload{Outcome: EvaluationState("magic")},
	}
	if err := event.Validate(); err == nil {
		t.Fatal("expected unsupported outcome rejection")
	}
}

func TestPayloadMapKeepsVersionedProjection(t *testing.T) {
	payload := Event{
		Type:           EventSettled,
		ConversationID: "conversation-1",
		RunID:          "run-1",
		ItemID:         "settlement-1",
		Payload: Payload{
			Outcome:       EvaluationAgentEvaluable,
			CheckpointRef: "commit-1",
		},
	}.Normalize().Payload
	row, err := PayloadMap(payload)
	if err != nil {
		t.Fatal(err)
	}
	if row["schema_version"] != SchemaVersion || row["trace_node_id"] != "settlement-1" || row["checkpoint_ref"] != "commit-1" {
		t.Fatalf("payload map = %#v", row)
	}
}
