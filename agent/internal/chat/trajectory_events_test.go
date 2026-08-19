package chat

import (
	"testing"

	"vit-daw-agent/internal/trajectory"
)

func TestTrajectoryEventUsesExistingTransientAgentEventTransport(t *testing.T) {
	server := &Server{}
	event, err := server.emitTrajectoryEvent("conversation-1", trajectory.Event{
		Type:   trajectory.EventMaterialityEvaluated,
		GoalID: "goal-1",
		RunID:  "run-1",
		ItemID: "materiality-1",
		Title:  "Dose evaluated",
		Payload: trajectory.Payload{
			RoundID:     "round-1",
			Materiality: trajectory.EvaluationInsufficientDose,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.Type != string(trajectory.EventMaterialityEvaluated) || event.ItemType != "trajectory" {
		t.Fatalf("event = %#v", event)
	}
	if event.Lifecycle != "transient" || event.Persistence != "none" || event.MessageKind != "activity" {
		t.Fatalf("protocol = %#v", event)
	}
	if event.TurnID != "run-1" || event.LogicalMessageID != "trajectory:run-1:materiality-1" {
		t.Fatalf("identity = %#v", event)
	}
	if event.Payload["schema_version"] != trajectory.SchemaVersion || event.Payload["materiality"] != string(trajectory.EvaluationInsufficientDose) {
		t.Fatalf("payload = %#v", event.Payload)
	}
}

func TestTrajectoryEventRejectsInvalidProjection(t *testing.T) {
	server := &Server{}
	_, err := server.emitTrajectoryEvent("conversation-1", trajectory.Event{
		Type:  trajectory.EventInterventionApplied,
		RunID: "run-1",
	})
	if err == nil {
		t.Fatal("expected invalid projection rejection")
	}
}

func TestTrajectoryEventsRetainAgentEventSequenceAndReplayScope(t *testing.T) {
	server := &Server{}
	first, err := server.emitTrajectoryEvent("conversation-1", trajectory.Event{
		Type:   trajectory.EventTurnStarted,
		RunID:  "run-1",
		ItemID: "turn-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.emitTrajectoryEvent("conversation-1", trajectory.Event{
		Type:   trajectory.EventSettled,
		RunID:  "run-1",
		ItemID: "settlement-1",
		Payload: trajectory.Payload{
			Outcome:       trajectory.EvaluationAgentEvaluable,
			CheckpointRef: "commit-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Seq >= second.Seq {
		t.Fatalf("sequence = first=%d second=%d", first.Seq, second.Seq)
	}
	events, nextSeq := server.agentEventsSince("conversation-1", first.Seq, 10)
	if len(events) != 1 || events[0].ItemID != "settlement-1" || nextSeq != second.Seq {
		t.Fatalf("replay = events=%#v next=%d", events, nextSeq)
	}
	other, nextOther := server.agentEventsSince("conversation-2", 0, 10)
	if len(other) != 0 || nextOther != 0 {
		t.Fatalf("cross-conversation replay = events=%#v next=%d", other, nextOther)
	}
}
