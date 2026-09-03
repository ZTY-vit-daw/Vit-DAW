// Package trajectory defines the versioned observable execution-trajectory
// projection carried by the existing AgentEvent transport.
package trajectory

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"vit-daw-agent/internal/taskstate"
)

const SchemaVersion = "vit.observable_trajectory.v1"

type EventType string

const (
	EventTurnStarted           EventType = "trajectory.turn.started"
	EventTurnCompleted         EventType = "trajectory.turn.completed"
	EventTurnFailed            EventType = "trajectory.turn.failed"
	EventTurnStopped           EventType = "trajectory.turn.stopped"
	EventIntentFramed          EventType = "trajectory.intent.framed"
	EventObservationRecorded   EventType = "trajectory.observation.recorded"
	EventHypothesisProposed    EventType = "trajectory.hypothesis.proposed"
	EventRoundStarted          EventType = "trajectory.round.started"
	EventInterventionApplied   EventType = "trajectory.intervention.applied"
	EventMaterialityEvaluated  EventType = "trajectory.intervention.materiality"
	EventTargetResponse        EventType = "trajectory.target.response"
	EventRoundDecision         EventType = "trajectory.round.decision"
	EventRollbackCompleted     EventType = "trajectory.rollback.completed"
	EventBranchCreated         EventType = "trajectory.branch.created"
	EventWorktreeCreated       EventType = "trajectory.worktree.created"
	EventUserJudgmentRequested EventType = "trajectory.user_judgment.requested"
	EventUserJudgmentRecorded  EventType = "trajectory.user_judgment.recorded"
	EventSettled               EventType = "trajectory.settled"
	EventError                 EventType = "trajectory.error"
)

type NodeKind string

const (
	NodeTurn         NodeKind = "turn"
	NodeIntent       NodeKind = "intent"
	NodeObservation  NodeKind = "observation"
	NodeHypothesis   NodeKind = "hypothesis"
	NodeAction       NodeKind = "action"
	NodeMateriality  NodeKind = "materiality"
	NodeVerification NodeKind = "verification"
	NodeDecision     NodeKind = "decision"
	NodeRollback     NodeKind = "rollback"
	NodeBranch       NodeKind = "branch_or_worktree"
	NodeJudgment     NodeKind = "user_judgment"
	NodeSettlement   NodeKind = "settlement"
	NodeError        NodeKind = "error"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusStopped   Status = "stopped"
	StatusFailed    Status = "failed"
	StatusWaiting   Status = "waiting_for_user"
)

type EvaluationState string

const (
	EvaluationNotReady              EvaluationState = "not_ready"
	EvaluationInsufficientDose      EvaluationState = "insufficient_dose"
	EvaluationAgentEvaluable        EvaluationState = "agent_evaluable"
	EvaluationHumanAuditionReady    EvaluationState = "human_audition_ready"
	EvaluationHumanConfirmed        EvaluationState = "human_confirmed"
	EvaluationAmbiguous             EvaluationState = "ambiguous"
	EvaluationUnsupportedHypothesis EvaluationState = "unsupported_hypothesis"
	EvaluationRolledBack            EvaluationState = "rolled_back"
)

// Payload is a UI-facing projection. Controller-private state must not be
// copied here; references point back to authoritative observations/actions.
type Payload struct {
	SchemaVersion      string                        `json:"schema_version"`
	TraceNodeID        string                        `json:"trace_node_id"`
	ParentNodeID       string                        `json:"parent_node_id,omitempty"`
	TurnID             string                        `json:"turn_id"`
	RoundID            string                        `json:"round_id,omitempty"`
	NodeKind           NodeKind                      `json:"node_kind"`
	Phase              string                        `json:"phase,omitempty"`
	Status             Status                        `json:"status"`
	Summary            string                        `json:"summary,omitempty"`
	EvidenceRefs       []string                      `json:"evidence_refs,omitempty"`
	ActionRefs         []string                      `json:"action_refs,omitempty"`
	ProjectRevision    string                        `json:"project_revision,omitempty"`
	CheckpointRef      string                        `json:"checkpoint_ref,omitempty"`
	BranchRef          string                        `json:"branch_ref,omitempty"`
	WorktreeRef        string                        `json:"worktree_ref,omitempty"`
	Materiality        EvaluationState               `json:"materiality,omitempty"`
	TargetResponse     string                        `json:"target_response,omitempty"`
	NextDecision       string                        `json:"next_decision,omitempty"`
	Outcome            EvaluationState               `json:"outcome,omitempty"`
	Details            map[string]any                `json:"details,omitempty"`
	TaskStateSchema    string                        `json:"task_state_schema,omitempty"`
	TaskState          taskstate.State               `json:"task_state,omitempty"`
	TaskStateRevision  uint64                        `json:"task_state_revision,omitempty"`
	ContractID         string                        `json:"contract_id,omitempty"`
	ContractScope      *taskstate.Scope              `json:"contract_scope,omitempty"`
	CandidateID        string                        `json:"candidate_id,omitempty"`
	ExperimentID       string                        `json:"experiment_id,omitempty"`
	PendingInteraction *taskstate.PendingInteraction `json:"pending_interaction,omitempty"`
	Terminal           bool                          `json:"terminal"`
	TransitionReason   string                        `json:"transition_reason,omitempty"`
}

// Event contains transport identity plus the versioned trajectory projection.
// Seq and CreatedAt remain owned by the AgentEvent transport.
type Event struct {
	Type           EventType
	ConversationID string
	GoalID         string
	RunID          string
	ItemID         string
	Title          string
	Body           string
	Payload        Payload
}

func (e Event) Normalize() Event {
	e.Type = EventType(strings.TrimSpace(string(e.Type)))
	e.ConversationID = strings.TrimSpace(e.ConversationID)
	e.GoalID = strings.TrimSpace(e.GoalID)
	e.RunID = strings.TrimSpace(e.RunID)
	e.ItemID = strings.TrimSpace(e.ItemID)
	e.Title = strings.TrimSpace(e.Title)
	e.Body = strings.TrimSpace(e.Body)
	p := e.Payload
	p.SchemaVersion = firstNonEmpty(strings.TrimSpace(p.SchemaVersion), SchemaVersion)
	p.TurnID = firstNonEmpty(strings.TrimSpace(p.TurnID), e.RunID, e.GoalID)
	p.TraceNodeID = firstNonEmpty(strings.TrimSpace(p.TraceNodeID), e.ItemID, defaultTraceNodeID(e.Type, p.TurnID, p.RoundID))
	p.ParentNodeID = strings.TrimSpace(p.ParentNodeID)
	p.RoundID = strings.TrimSpace(p.RoundID)
	p.NodeKind = NodeKind(firstNonEmpty(strings.TrimSpace(string(p.NodeKind)), string(DefaultNodeKind(e.Type))))
	p.Phase = firstNonEmpty(strings.TrimSpace(p.Phase), DefaultPhase(e.Type))
	p.Status = Status(firstNonEmpty(strings.TrimSpace(string(p.Status)), string(DefaultStatus(e.Type))))
	p.Summary = strings.TrimSpace(p.Summary)
	p.ProjectRevision = strings.TrimSpace(p.ProjectRevision)
	p.CheckpointRef = strings.TrimSpace(p.CheckpointRef)
	p.BranchRef = strings.TrimSpace(p.BranchRef)
	p.WorktreeRef = strings.TrimSpace(p.WorktreeRef)
	p.TargetResponse = strings.TrimSpace(p.TargetResponse)
	p.NextDecision = strings.TrimSpace(p.NextDecision)
	p.EvidenceRefs = uniqueStrings(p.EvidenceRefs)
	p.ActionRefs = uniqueStrings(p.ActionRefs)
	e.Payload = p
	if e.ItemID == "" {
		e.ItemID = p.TraceNodeID
	}
	return e
}

func (e Event) Validate() error {
	e = e.Normalize()
	if !e.Type.Valid() {
		return fmt.Errorf("unsupported trajectory event type %q", e.Type)
	}
	if e.ConversationID == "" {
		return errors.New("trajectory conversation_id is required")
	}
	if e.Payload.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported trajectory schema_version %q", e.Payload.SchemaVersion)
	}
	if e.Payload.TurnID == "" {
		return errors.New("trajectory turn_id is required")
	}
	if e.Payload.TraceNodeID == "" {
		return errors.New("trajectory trace_node_id is required")
	}
	if !e.Payload.NodeKind.Valid() {
		return fmt.Errorf("unsupported trajectory node_kind %q", e.Payload.NodeKind)
	}
	if !e.Payload.Status.Valid() {
		return fmt.Errorf("unsupported trajectory status %q", e.Payload.Status)
	}
	if e.Type.RequiresRound() && e.Payload.RoundID == "" {
		return fmt.Errorf("trajectory event %q requires round_id", e.Type)
	}
	if e.Payload.Materiality != "" && !e.Payload.Materiality.Valid() {
		return fmt.Errorf("unsupported trajectory materiality %q", e.Payload.Materiality)
	}
	if e.Payload.Outcome != "" && !e.Payload.Outcome.Valid() {
		return fmt.Errorf("unsupported trajectory outcome %q", e.Payload.Outcome)
	}
	if e.Payload.TaskState != "" {
		if e.Payload.TaskStateSchema != taskstate.SchemaVersion || !e.Payload.TaskState.Valid() || e.Payload.TaskStateRevision == 0 || e.Payload.ContractID == "" {
			return fmt.Errorf("trajectory canonical task projection is invalid")
		}
		if e.Payload.Terminal != e.Payload.TaskState.Terminal() {
			return fmt.Errorf("trajectory terminal flag does not match canonical task state")
		}
	}
	return nil
}

func BindTaskState(event Event, contract taskstate.Contract, state taskstate.Snapshot) (Event, error) {
	if err := state.Validate(contract); err != nil {
		return Event{}, fmt.Errorf("canonical task projection: %w", err)
	}
	if event.GoalID != "" && event.GoalID != contract.GoalID {
		return Event{}, fmt.Errorf("trajectory goal identity does not match task contract")
	}
	if event.RunID != "" && event.RunID != contract.RunID {
		return Event{}, fmt.Errorf("trajectory run identity does not match task contract")
	}
	scope := contract.Scope
	event.Payload.TaskStateSchema = taskstate.SchemaVersion
	event.Payload.TaskState = state.State
	event.Payload.TaskStateRevision = state.Revision
	event.Payload.ContractID = contract.ContractID
	event.Payload.ContractScope = &scope
	event.Payload.CandidateID = state.CandidateID
	event.Payload.ExperimentID = state.ExperimentID
	event.Payload.Terminal = state.Terminal
	event.Payload.TransitionReason = state.TransitionReason
	if state.PendingInteraction != nil {
		interaction := *state.PendingInteraction
		event.Payload.PendingInteraction = &interaction
	} else {
		event.Payload.PendingInteraction = nil
	}
	event.Payload.EvidenceRefs = uniqueStrings(append(event.Payload.EvidenceRefs, state.EvidenceRefs...))
	return event, nil
}

func PayloadMap(payload Payload) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (t EventType) Valid() bool {
	switch t {
	case EventTurnStarted, EventTurnCompleted, EventTurnFailed, EventTurnStopped, EventIntentFramed, EventObservationRecorded,
		EventHypothesisProposed, EventRoundStarted, EventInterventionApplied,
		EventMaterialityEvaluated, EventTargetResponse, EventRoundDecision,
		EventRollbackCompleted, EventBranchCreated, EventWorktreeCreated,
		EventUserJudgmentRequested, EventUserJudgmentRecorded, EventSettled, EventError:
		return true
	default:
		return false
	}
}

func (t EventType) RequiresRound() bool {
	switch t {
	case EventRoundStarted, EventInterventionApplied, EventMaterialityEvaluated,
		EventTargetResponse, EventRoundDecision, EventRollbackCompleted:
		return true
	default:
		return false
	}
}

func (k NodeKind) Valid() bool {
	switch k {
	case NodeTurn, NodeIntent, NodeObservation, NodeHypothesis, NodeAction,
		NodeMateriality, NodeVerification, NodeDecision, NodeRollback, NodeBranch,
		NodeJudgment, NodeSettlement, NodeError:
		return true
	default:
		return false
	}
}

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusRunning, StatusCompleted, StatusStopped, StatusFailed, StatusWaiting:
		return true
	default:
		return false
	}
}

func (s EvaluationState) Valid() bool {
	switch s {
	case EvaluationNotReady, EvaluationInsufficientDose, EvaluationAgentEvaluable,
		EvaluationHumanAuditionReady, EvaluationHumanConfirmed, EvaluationAmbiguous,
		EvaluationUnsupportedHypothesis, EvaluationRolledBack:
		return true
	default:
		return false
	}
}

func DefaultNodeKind(t EventType) NodeKind {
	switch t {
	case EventTurnStarted, EventTurnCompleted, EventTurnFailed, EventTurnStopped:
		return NodeTurn
	case EventIntentFramed:
		return NodeIntent
	case EventObservationRecorded:
		return NodeObservation
	case EventHypothesisProposed:
		return NodeHypothesis
	case EventInterventionApplied:
		return NodeAction
	case EventMaterialityEvaluated:
		return NodeMateriality
	case EventTargetResponse:
		return NodeVerification
	case EventRoundStarted, EventRoundDecision:
		return NodeDecision
	case EventRollbackCompleted:
		return NodeRollback
	case EventBranchCreated, EventWorktreeCreated:
		return NodeBranch
	case EventUserJudgmentRequested, EventUserJudgmentRecorded:
		return NodeJudgment
	case EventSettled:
		return NodeSettlement
	case EventError:
		return NodeError
	default:
		return ""
	}
}

func DefaultStatus(t EventType) Status {
	switch t {
	case EventTurnStarted, EventRoundStarted:
		return StatusRunning
	case EventTurnCompleted:
		return StatusCompleted
	case EventTurnFailed:
		return StatusFailed
	case EventUserJudgmentRequested:
		return StatusWaiting
	case EventTurnStopped:
		return StatusStopped
	case EventError:
		return StatusFailed
	default:
		return StatusCompleted
	}
}

func DefaultPhase(t EventType) string {
	switch t {
	case EventTurnStarted:
		return "framing"
	case EventTurnCompleted:
		return "completed"
	case EventTurnFailed:
		return "failed"
	case EventIntentFramed:
		return "framing"
	case EventObservationRecorded:
		return "observing"
	case EventHypothesisProposed:
		return "hypothesizing"
	case EventRoundStarted:
		return "admitted"
	case EventInterventionApplied:
		return "treating"
	case EventMaterialityEvaluated:
		return "materiality_evaluating"
	case EventTargetResponse:
		return "target_response_evaluating"
	case EventRoundDecision:
		return "deciding"
	case EventRollbackCompleted:
		return "rolled_back"
	case EventUserJudgmentRequested, EventUserJudgmentRecorded:
		return "user_judgment"
	case EventSettled:
		return "settled"
	case EventTurnStopped:
		return "stopped"
	case EventError:
		return "failed"
	default:
		return ""
	}
}

func defaultTraceNodeID(t EventType, turnID, roundID string) string {
	parts := []string{strings.TrimPrefix(string(t), "trajectory."), turnID, roundID}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ":")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
