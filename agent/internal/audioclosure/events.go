package audioclosure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type EventType string

const (
	EventStarted                EventType = "closure_started"
	EventRoundStarted           EventType = "round_started"
	EventObservationRecorded    EventType = "observation_recorded"
	EventProjectChangeRecorded  EventType = "project_change_recorded"
	EventFrontierUpdated        EventType = "frontier_updated"
	EventRoundCompleted         EventType = "round_completed"
	EventProtocolRepairRecorded EventType = "model_protocol_repair_recorded"
	EventCapabilityStarted      EventType = "capability_started"
	EventCapabilitySettled      EventType = "capability_settled"
	EventRollbackStarted        EventType = "rollback_started"
	EventSettled                EventType = "closure_settled"
)

type Event struct {
	EventID    string          `json:"event_id"`
	ClosureID  string          `json:"closure_id"`
	Sequence   uint64          `json:"sequence"`
	Type       EventType       `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data,omitempty"`
}

type startedData struct {
	ConversationID, GoalID, RunID, ProjectUUID, ProjectRevision, OriginalIntent string
	Mode                                                                        Mode
	Scope                                                                       Scope
	Policy                                                                      Policy
}
type roundStartedData struct {
	Round int `json:"round"`
}
type observationRecordedData struct {
	Record    ObservationRecord `json:"record"`
	Duplicate bool              `json:"duplicate"`
}
type projectChangeRecordedData struct {
	ChangeID string `json:"change_id"`
}
type frontierUpdatedData struct {
	Frontier      HypothesisFrontier `json:"frontier"`
	Actionability Actionability      `json:"actionability"`
	Progress      bool               `json:"progress"`
}
type roundCompletedData struct {
	Progress bool `json:"progress"`
}
type protocolRepairData struct {
	Count int `json:"count"`
}
type capabilityStartedData struct {
	Link CapabilityLink `json:"link"`
}
type capabilitySettledData struct {
	Settlement CapabilitySettlement `json:"settlement"`
}
type rollbackStartedData struct {
	Count int `json:"count"`
}
type settledData struct {
	Settlement Settlement `json:"settlement"`
}

func Fold(events []Event) (State, error) {
	var state State
	for index, event := range events {
		if event.Sequence != uint64(index+1) {
			return State{}, fmt.Errorf("event sequence %d is not contiguous at index %d", event.Sequence, index)
		}
		if index == 0 && event.Type != EventStarted {
			return State{}, fmt.Errorf("first event must be %s", EventStarted)
		}
		if index > 0 && event.ClosureID != state.ClosureID {
			return State{}, fmt.Errorf("event closure_id %q does not match %q", event.ClosureID, state.ClosureID)
		}
		if state.Terminal() {
			return State{}, fmt.Errorf("event %s follows terminal settlement", event.Type)
		}
		if err := applyEvent(&state, event); err != nil {
			return State{}, fmt.Errorf("apply %s: %w", event.Type, err)
		}
		state.Revision = event.Sequence
		state.UpdatedAt = event.OccurredAt
		state.Events = append(state.Events, cloneEvent(event))
	}
	return state, nil
}

func applyEvent(state *State, event Event) error {
	switch event.Type {
	case EventStarted:
		if state.Revision != 0 || state.SchemaVersion != "" {
			return fmt.Errorf("closure is already started")
		}
		var data startedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		state.SchemaVersion, state.ClosureID = SchemaVersion, event.ClosureID
		state.ConversationID, state.GoalID, state.RunID = data.ConversationID, data.GoalID, data.RunID
		state.ProjectUUID, state.ProjectRevision = data.ProjectUUID, data.ProjectRevision
		state.OriginalIntent, state.Mode, state.Scope, state.Policy = data.OriginalIntent, data.Mode, data.Scope, data.Policy
		state.Phase, state.Actionability = PhaseObserving, ActionabilityUnknown
		state.Observations = map[string]ObservationRecord{}
		state.CapabilitySettlements = map[string]CapabilitySettlement{}
		state.CreatedAt = event.OccurredAt
	case EventRoundStarted:
		if state.RoundInProgress || state.ActiveCapability != nil {
			return fmt.Errorf("a round cannot start in the current state")
		}
		var data roundStartedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.Round != state.RoundsStarted+1 {
			return fmt.Errorf("round %d is not the next round", data.Round)
		}
		state.RoundsStarted, state.RoundInProgress, state.RoundHadProgress = data.Round, true, false
		state.Phase = PhaseReasoning
	case EventObservationRecorded:
		if !state.RoundInProgress {
			return fmt.Errorf("observation requires an admitted round")
		}
		var data observationRecordedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if !data.Duplicate {
			if _, exists := state.Observations[data.Record.Fingerprint]; exists {
				return fmt.Errorf("new observation fingerprint already exists")
			}
			state.Observations[data.Record.Fingerprint] = data.Record
			state.ObservationOrder = append(state.ObservationOrder, data.Record.Fingerprint)
		}
	case EventProjectChangeRecorded:
		if !state.RoundInProgress {
			return fmt.Errorf("project change requires an admitted round")
		}
		var data projectChangeRecordedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if strings.TrimSpace(data.ChangeID) == "" {
			return fmt.Errorf("project change id is required")
		}
		if state.LastProjectChangeID == data.ChangeID {
			return fmt.Errorf("project change %q was already recorded", data.ChangeID)
		}
		state.LastProjectChangeID = data.ChangeID
		// Deterministic engineering progress only; acoustic evidence still
		// requires the post-action CCB observation gate.
		state.RoundHadProgress = true
	case EventFrontierUpdated:
		if !state.RoundInProgress {
			return fmt.Errorf("frontier update requires an admitted round")
		}
		var data frontierUpdatedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		state.Frontier, state.Actionability = data.Frontier, data.Actionability
		state.RoundHadProgress = state.RoundHadProgress || data.Progress
	case EventRoundCompleted:
		if !state.RoundInProgress {
			return fmt.Errorf("no round is in progress")
		}
		var data roundCompletedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		progress := state.RoundHadProgress || data.Progress
		if progress {
			state.NoProgressStreak = 0
		} else {
			state.NoProgressStreak++
		}
		state.RoundInProgress, state.RoundHadProgress = false, false
		state.Phase = PhaseObserving
	case EventProtocolRepairRecorded:
		var data protocolRepairData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.Count != state.ModelProtocolRepairs+1 {
			return fmt.Errorf("protocol repair count is not monotonic")
		}
		state.ModelProtocolRepairs = data.Count
	case EventCapabilityStarted:
		if state.ActiveCapability != nil {
			return fmt.Errorf("a capability session is already active")
		}
		var data capabilityStartedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		link := data.Link
		state.ActiveCapability, state.ActionAttempts, state.Phase = &link, state.ActionAttempts+1, PhaseAwaitingCapability
		state.RoundInProgress = false
	case EventCapabilitySettled:
		if state.ActiveCapability == nil {
			return fmt.Errorf("no capability session is active")
		}
		var data capabilitySettledData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.Settlement.SessionID != state.ActiveCapability.SessionID || data.Settlement.ActionID != state.ActiveCapability.ActionID {
			return fmt.Errorf("capability settlement does not match the active session")
		}
		state.CapabilitySettlements[data.Settlement.ActionID] = data.Settlement
		state.ActiveCapability, state.Phase = nil, PhaseVerifying
	case EventRollbackStarted:
		var data rollbackStartedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.Count != state.RollbackAttempts+1 {
			return fmt.Errorf("rollback count is not monotonic")
		}
		state.RollbackAttempts = data.Count
	case EventSettled:
		if state.ActiveCapability != nil {
			return fmt.Errorf("cannot settle while a capability session is active")
		}
		var data settledData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		settlement := data.Settlement
		state.Settlement, state.Phase = &settlement, PhaseSettled
		state.RoundInProgress, state.RoundHadProgress = false, false
	default:
		return fmt.Errorf("unknown event type %q", event.Type)
	}
	return nil
}

func appendEvent(state State, eventType EventType, data any, now time.Time) (State, error) {
	if state.Terminal() {
		return State{}, fmt.Errorf("closure %s is settled", state.ClosureID)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return State{}, err
	}
	sequence := state.Revision + 1
	event := Event{EventID: fmt.Sprintf("%s:%d", state.ClosureID, sequence), ClosureID: state.ClosureID, Sequence: sequence, Type: eventType, OccurredAt: now.UTC(), Data: raw}
	events := append(cloneEvents(state.Events), event)
	return Fold(events)
}

func decodeEventData(event Event, target any) error {
	if len(event.Data) == 0 {
		return fmt.Errorf("event data is required")
	}
	if err := json.Unmarshal(event.Data, target); err != nil {
		return err
	}
	return nil
}

func cloneEvent(event Event) Event {
	event.Data = append(json.RawMessage(nil), event.Data...)
	return event
}

func cloneEvents(events []Event) []Event {
	out := make([]Event, len(events))
	for i, event := range events {
		out[i] = cloneEvent(event)
	}
	return out
}

func normalizeText(value string) string { return strings.TrimSpace(value) }
