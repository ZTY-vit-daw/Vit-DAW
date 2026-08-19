package chat

import (
	"fmt"

	"vit-daw-agent/internal/trajectory"
)

// emitTrajectoryEvent projects a versioned observable-trajectory event into
// the existing AgentEvent transport. It deliberately remains transient; only
// the final durable settlement is written through Project History.
func (s *Server) emitTrajectoryEvent(conversationID string, event trajectory.Event) (AgentEvent, error) {
	event.ConversationID = firstNonEmpty(event.ConversationID, conversationID)
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return AgentEvent{}, fmt.Errorf("trajectory event: %w", err)
	}
	payload, err := trajectory.PayloadMap(event.Payload)
	if err != nil {
		return AgentEvent{}, fmt.Errorf("trajectory payload: %w", err)
	}
	return s.emitAgentEvent(event.ConversationID, AgentEvent{
		Type:             string(event.Type),
		GoalID:           event.GoalID,
		RunID:            event.RunID,
		ItemID:           event.ItemID,
		ItemType:         "trajectory",
		Status:           string(event.Payload.Status),
		Title:            event.Title,
		Body:             event.Body,
		Payload:          payload,
		Lifecycle:        "transient",
		Persistence:      "none",
		MessageKind:      "activity",
		TurnID:           event.Payload.TurnID,
		LogicalMessageID: "trajectory:" + event.Payload.TurnID + ":" + event.Payload.TraceNodeID,
	}), nil
}
