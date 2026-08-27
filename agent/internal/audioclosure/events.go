package audioclosure

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/taskstate"
)

type EventType string

const (
	EventStarted                 EventType = "closure_started"
	EventRoundStarted            EventType = "round_started"
	EventObservationRecorded     EventType = "observation_recorded"
	EventProjectChangeRecorded   EventType = "project_change_recorded"
	EventProjectRevisionChanged  EventType = "project_revision_changed"
	EventGovernedRevisionBooked  EventType = "governed_revision_booked"
	EventFrontierUpdated         EventType = "frontier_updated"
	EventRoundCompleted          EventType = "round_completed"
	EventProtocolRepairRecorded  EventType = "model_protocol_repair_recorded"
	EventCapabilityStarted       EventType = "capability_started"
	EventCapabilitySettled       EventType = "capability_settled"
	EventRollbackStarted         EventType = "rollback_started"
	EventTaskStateProjected      EventType = "task_state_projected"
	EventPolicyExtended          EventType = "closure_policy_extended"
	EventPhaseTransition         EventType = "phase_transition"
	EventDiagnosticRoundRecorded EventType = "diagnostic_round_recorded"
	EventSettled                 EventType = "closure_settled"
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
	ConversationID, TaskID, GoalID, RunID, ContractID, ProjectUUID, ProjectRevision, OriginalIntent string
	TaskState                                                                                       taskstate.State
	TaskStateRevision                                                                               uint64
	Mode                                                                                            Mode
	Scope                                                                                           Scope
	Policy                                                                                          Policy
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
type projectRevisionChangedData struct {
	ProjectRevision string `json:"project_revision"`
}
type governedRevisionBookedData struct {
	ProjectRevision string `json:"project_revision"`
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
type taskStateProjectedData struct {
	ContractID string          `json:"contract_id"`
	State      taskstate.State `json:"state"`
	Revision   uint64          `json:"revision"`
}
type policyExtendedData struct {
	MaxClosureRounds int `json:"max_closure_rounds"`
}
type settledData struct {
	Settlement Settlement `json:"settlement"`
}
type phaseTransitionData struct {
	From        Phase           `json:"from"`
	To          Phase           `json:"to"`
	Guard       PhaseGuardInput `json:"guard"`
	GuardPassed bool            `json:"guard_passed"`
	Reason      string          `json:"reason,omitempty"`
}
type diagnosticRoundRecordedData struct {
	Round DiagnosticRoundRecord `json:"round"`
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
		state.ConversationID, state.TaskID, state.GoalID, state.RunID = data.ConversationID, data.TaskID, data.GoalID, data.RunID
		state.ContractID, state.TaskState, state.TaskStateRevision = data.ContractID, data.TaskState, data.TaskStateRevision
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
		setLegacyPhase(state, PhaseReasoning)
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
	case EventProjectRevisionChanged:
		var data projectRevisionChangedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if strings.TrimSpace(data.ProjectRevision) == "" || data.ProjectRevision == state.ProjectRevision {
			return fmt.Errorf("project revision change requires a new revision")
		}
		if state.ActiveCapability != nil {
			return fmt.Errorf("active capability must settle before project revision revalidation")
		}
		state.ProjectRevision = strings.TrimSpace(data.ProjectRevision)
		state.ObservationOrder = nil
		state.Observations = map[string]ObservationRecord{}
		state.Frontier = HypothesisFrontier{}
		state.Actionability = ActionabilityUnknown
		state.RoundInProgress, state.RoundHadProgress = false, false
		state.NoProgressStreak = 0
		setLegacyPhase(state, PhaseObserving)
	case EventGovernedRevisionBooked:
		var data governedRevisionBookedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if strings.TrimSpace(data.ProjectRevision) == "" || data.ProjectRevision == state.ProjectRevision {
			return fmt.Errorf("governed revision booking requires a new revision")
		}
		if state.ActiveCapability == nil {
			return fmt.Errorf("governed revision booking requires an active capability")
		}
		// The applied revision of the in-flight governed mutation is the
		// expected outcome of this round, not external drift: the tracked
		// revision moves so the post-action observation matches, while every
		// other piece of evidence (round, observations, frontier) survives.
		state.ProjectRevision = strings.TrimSpace(data.ProjectRevision)
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
		setLegacyPhase(state, PhaseObserving)
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
		state.ActiveCapability, state.ActionAttempts = &link, state.ActionAttempts+1
		setLegacyPhase(state, PhaseAwaitingCapability)
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
		state.ActiveCapability = nil
		setLegacyPhase(state, PhaseVerifying)
	case EventRollbackStarted:
		var data rollbackStartedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.Count != state.RollbackAttempts+1 {
			return fmt.Errorf("rollback count is not monotonic")
		}
		state.RollbackAttempts = data.Count
	case EventTaskStateProjected:
		var data taskStateProjectedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.ContractID == "" || data.ContractID != state.ContractID || !data.State.Valid() {
			return fmt.Errorf("task state projection identity or state is invalid")
		}
		if data.Revision <= state.TaskStateRevision {
			return fmt.Errorf("task state revision is not monotonic")
		}
		state.TaskState, state.TaskStateRevision = data.State, data.Revision
	case EventPolicyExtended:
		var data policyExtendedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.MaxClosureRounds <= state.Policy.MaxClosureRounds || data.MaxClosureRounds < state.RoundsStarted {
			return fmt.Errorf("closure policy extension must increase max rounds")
		}
		state.Policy.MaxClosureRounds = data.MaxClosureRounds
	case EventPhaseTransition:
		var data phaseTransitionData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if data.From != state.Phase {
			return fmt.Errorf("phase transition source %q does not match current phase %q", data.From, state.Phase)
		}
		if data.To == PhaseFS0SemanticEntry {
			// FS entry: the legacy closure phase becomes FS0. The only guard is
			// the entry target itself; every later transition is guarded.
			if IsFSPhase(state.Phase) {
				return fmt.Errorf("the FS machine is already entered at %s", state.Phase)
			}
		} else {
			if !IsFSPhase(state.Phase) {
				return fmt.Errorf("phase transition requires an FS source phase, got %q", state.Phase)
			}
			if err := EvaluatePhaseGuard(data.From, data.To, data.Guard); err != nil {
				return err
			}
			if !data.GuardPassed {
				return fmt.Errorf("phase transition event must record a passed guard")
			}
		}
		state.Phase = data.To
	case EventDiagnosticRoundRecorded:
		var data diagnosticRoundRecordedData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		if err := data.Round.Validate(); err != nil {
			return fmt.Errorf("diagnostic round record invalid: %w", err)
		}
		state.DiagnosticRounds = append(state.DiagnosticRounds, data.Round)
	case EventSettled:
		if state.ActiveCapability != nil {
			return fmt.Errorf("cannot settle while a capability session is active")
		}
		var data settledData
		if err := decodeEventData(event, &data); err != nil {
			return err
		}
		settlement := data.Settlement
		state.Settlement = &settlement
		// A settled closure is FS9 when the FS machine owns the phase.
		if IsFSPhase(state.Phase) {
			state.Phase = PhaseFS9Terminal
		} else {
			state.Phase = PhaseSettled
		}
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

// setLegacyPhase assigns a legacy five-value phase only while the FS machine
// has not been entered. Once an FS phase owns the closure, legacy lifecycle
// events (round start/complete, capability begin/settle) are sub-states, not
// phases (transition-table contract §4).
func setLegacyPhase(state *State, phase Phase) {
	if !IsFSPhase(state.Phase) {
		state.Phase = phase
	}
}
