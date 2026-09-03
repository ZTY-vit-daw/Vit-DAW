// Package taskstate owns the canonical semantic state of a durable product
// task. Domain runtimes may retain richer local lifecycles, but only this
// package decides whether the user's task is still open or has reached a
// durable outcome.
package taskstate

import (
	"fmt"
	"strings"
	"time"
)

const SchemaVersion = "vit.task_semantic_state.v1"

type State string

const (
	StateObservationInProgress State = "observation_in_progress"
	StateDiagnosticComplete    State = "diagnostic_complete"
	StateNoCandidateFound      State = "no_candidate_found"
	StateImprovementProposal   State = "improvement_proposal"
	StateNeedsExperiment       State = "needs_experiment"
	StateHumanJudgmentRequired State = "human_judgment_required"
	StateCapabilityBlocked     State = "capability_blocked"
	StateSettled               State = "settled"
	StateCancelled             State = "cancelled"
	StateFailed                State = "failed"
	StateClosed                State = "closed"
)

type ContractKind string

const (
	ContractDiagnostic  ContractKind = "diagnostic"
	ContractImprovement ContractKind = "improvement"
)

type Scope struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}

// Contract is the durable interpretation of the original task intent. A
// temporary contract lets an untargeted request discover its own candidate
// without allowing a local finding to silently redefine completion.
type Contract struct {
	SchemaVersion         string       `json:"schema_version"`
	ContractID            string       `json:"contract_id"`
	TaskID                string       `json:"task_id"`
	GoalID                string       `json:"goal_id"`
	RunID                 string       `json:"run_id"`
	ConversationID        string       `json:"conversation_id"`
	OriginalIntent        string       `json:"original_intent"`
	Kind                  ContractKind `json:"kind"`
	Scope                 Scope        `json:"scope"`
	Temporary             bool         `json:"temporary"`
	TargetDiscovery       string       `json:"target_discovery,omitempty"`
	AuthorizationBoundary string       `json:"authorization_boundary"`
	CompletionCriteria    []string     `json:"completion_criteria"`
	EvidenceRequirements  []string     `json:"evidence_requirements"`
	ProjectUUID           string       `json:"project_uuid,omitempty"`
	ProjectRevision       string       `json:"project_revision,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
}

type BoundedProposal struct {
	ProposalID         string   `json:"proposal_id"`
	Summary            string   `json:"summary"`
	EvidenceRefs       []string `json:"evidence_refs"`
	Bounds             []string `json:"bounds,omitempty"`
	RequiresExperiment bool     `json:"requires_experiment"`
}

type PendingInteraction struct {
	InteractionID string `json:"interaction_id"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
}

type Event string

const (
	EventObservationStarted     Event = "observation_started"
	EventDiagnosticCompleted    Event = "diagnostic_completed"
	EventNoCandidateReported    Event = "no_candidate_reported"
	EventImprovementProposed    Event = "improvement_proposed"
	EventExperimentRequired     Event = "experiment_required"
	EventHumanJudgmentRequested Event = "human_judgment_requested"
	EventCapabilityBlocked      Event = "capability_blocked"
	EventTaskSettled            Event = "task_settled"
	EventTaskCancelled          Event = "task_cancelled"
	EventTaskFailed             Event = "task_failed"
	EventProjectRevisionChanged Event = "project_revision_changed"
	EventOwnerTurnClosed        Event = "owner_turn_closed"
)

type TransitionRequest struct {
	Event              Event               `json:"event"`
	Reason             string              `json:"reason"`
	Summary            string              `json:"summary,omitempty"`
	EvidenceRefs       []string            `json:"evidence_refs,omitempty"`
	CandidateID        string              `json:"candidate_id,omitempty"`
	Proposal           *BoundedProposal    `json:"proposal,omitempty"`
	ExperimentID       string              `json:"experiment_id,omitempty"`
	PendingInteraction *PendingInteraction `json:"pending_interaction,omitempty"`
	ProjectRevision    string              `json:"project_revision,omitempty"`
}

type TransitionRecord struct {
	Revision           uint64              `json:"revision"`
	Event              Event               `json:"event"`
	From               State               `json:"from,omitempty"`
	To                 State               `json:"to"`
	Reason             string              `json:"reason"`
	Summary            string              `json:"summary,omitempty"`
	EvidenceRefs       []string            `json:"evidence_refs,omitempty"`
	CandidateID        string              `json:"candidate_id,omitempty"`
	ExperimentID       string              `json:"experiment_id,omitempty"`
	PendingInteraction *PendingInteraction `json:"pending_interaction,omitempty"`
	ProjectRevision    string              `json:"project_revision,omitempty"`
	OccurredAt         time.Time           `json:"occurred_at"`
}

type Snapshot struct {
	SchemaVersion      string              `json:"schema_version"`
	State              State               `json:"state"`
	Revision           uint64              `json:"revision"`
	ContractID         string              `json:"contract_id"`
	ProjectUUID        string              `json:"project_uuid,omitempty"`
	ProjectRevision    string              `json:"project_revision,omitempty"`
	EvidenceRefs       []string            `json:"evidence_refs,omitempty"`
	CandidateID        string              `json:"candidate_id,omitempty"`
	Proposal           *BoundedProposal    `json:"proposal,omitempty"`
	ExperimentID       string              `json:"experiment_id,omitempty"`
	PendingInteraction *PendingInteraction `json:"pending_interaction,omitempty"`
	TransitionReason   string              `json:"transition_reason"`
	Summary            string              `json:"summary,omitempty"`
	Terminal           bool                `json:"terminal"`
	History            []TransitionRecord  `json:"history"`
	UpdatedAt          time.Time           `json:"updated_at"`
}

func New(contract Contract, now time.Time) (Snapshot, error) {
	contract = NormalizeContract(contract, now)
	if err := contract.Validate(); err != nil {
		return Snapshot{}, err
	}
	return Apply(contract, Snapshot{SchemaVersion: SchemaVersion, ContractID: contract.ContractID, ProjectUUID: contract.ProjectUUID, ProjectRevision: contract.ProjectRevision}, TransitionRequest{
		Event: EventObservationStarted, Reason: "task contract admitted", ProjectRevision: contract.ProjectRevision,
	}, now)
}

func Apply(contract Contract, current Snapshot, request TransitionRequest, now time.Time) (Snapshot, error) {
	contract = NormalizeContract(contract, now)
	if err := contract.Validate(); err != nil {
		return Snapshot{}, err
	}
	if current.SchemaVersion == "" {
		current.SchemaVersion = SchemaVersion
	}
	if current.SchemaVersion != SchemaVersion || current.ContractID != contract.ContractID {
		return Snapshot{}, fmt.Errorf("semantic snapshot does not match task contract")
	}
	if err := current.Validate(contract); current.Revision > 0 && err != nil {
		return Snapshot{}, fmt.Errorf("current semantic snapshot: %w", err)
	}
	request = normalizeRequest(request)
	if request.Reason == "" {
		return Snapshot{}, fmt.Errorf("semantic transition reason is required")
	}
	nextState, err := transitionTarget(contract, current, request)
	if err != nil {
		return Snapshot{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	next := cloneSnapshot(current)
	next.SchemaVersion = SchemaVersion
	next.ContractID = contract.ContractID
	next.State = nextState
	next.Revision++
	next.ProjectUUID = contract.ProjectUUID
	if request.ProjectRevision != "" {
		next.ProjectRevision = request.ProjectRevision
	} else if next.ProjectRevision == "" {
		next.ProjectRevision = contract.ProjectRevision
	}
	if request.Event == EventProjectRevisionChanged {
		next.EvidenceRefs = nil
		// A governed experiment can advance the project revision between its
		// before/after observations. Invalidate only revision-bound evidence;
		// preserve the canonical proposal/experiment identity so the Task,
		// Experiment runtime, and durable continuation can rebind coherently.
		if current.ExperimentID == "" {
			next.CandidateID = ""
			next.Proposal = nil
		}
		next.PendingInteraction = nil
	} else {
		next.EvidenceRefs = unique(append(next.EvidenceRefs, request.EvidenceRefs...))
		if request.CandidateID != "" {
			next.CandidateID = request.CandidateID
		}
		if request.Proposal != nil {
			proposal := cloneProposal(*request.Proposal)
			next.Proposal = &proposal
		}
		if request.ExperimentID != "" {
			next.ExperimentID = request.ExperimentID
		}
		if request.PendingInteraction != nil {
			interaction := *request.PendingInteraction
			next.PendingInteraction = &interaction
		} else if nextState != StateHumanJudgmentRequired {
			next.PendingInteraction = nil
		}
	}
	next.TransitionReason = request.Reason
	next.Summary = request.Summary
	next.Terminal = nextState.Terminal()
	next.UpdatedAt = now
	record := TransitionRecord{
		Revision: next.Revision, Event: request.Event, From: current.State, To: nextState,
		Reason: request.Reason, Summary: request.Summary, EvidenceRefs: append([]string(nil), request.EvidenceRefs...),
		CandidateID: request.CandidateID, ExperimentID: request.ExperimentID,
		ProjectRevision: next.ProjectRevision, OccurredAt: now,
	}
	if request.PendingInteraction != nil {
		interaction := *request.PendingInteraction
		record.PendingInteraction = &interaction
	}
	next.History = append(next.History, record)
	if err := next.Validate(contract); err != nil {
		return Snapshot{}, fmt.Errorf("next semantic snapshot: %w", err)
	}
	return next, nil
}

func transitionTarget(contract Contract, current Snapshot, request TransitionRequest) (State, error) {
	from := current.State
	requireEvidence := func() error {
		if len(request.EvidenceRefs) == 0 && len(current.EvidenceRefs) == 0 {
			return fmt.Errorf("semantic transition %s requires evidence_refs", request.Event)
		}
		return nil
	}
	switch request.Event {
	case EventObservationStarted:
		if current.Revision != 0 && from != StateObservationInProgress {
			return "", invalidTransition(from, request.Event)
		}
		return StateObservationInProgress, nil
	case EventProjectRevisionChanged:
		if request.ProjectRevision == "" || request.ProjectRevision == current.ProjectRevision {
			return "", fmt.Errorf("project revision transition requires a new project_revision")
		}
		if from == StateSettled || from == StateCancelled || from == StateFailed {
			return "", invalidTransition(from, request.Event)
		}
		return StateObservationInProgress, nil
	case EventDiagnosticCompleted:
		if from != StateObservationInProgress {
			return "", invalidTransition(from, request.Event)
		}
		if err := requireEvidence(); err != nil {
			return "", err
		}
		return StateDiagnosticComplete, nil
	case EventNoCandidateReported:
		if from != StateObservationInProgress && from != StateDiagnosticComplete {
			return "", invalidTransition(from, request.Event)
		}
		if err := requireEvidence(); err != nil {
			return "", err
		}
		return StateNoCandidateFound, nil
	case EventImprovementProposed:
		if from != StateObservationInProgress && from != StateDiagnosticComplete {
			return "", invalidTransition(from, request.Event)
		}
		if request.Proposal == nil {
			return "", fmt.Errorf("improvement_proposed requires a bounded proposal")
		}
		if err := validateProposal(*request.Proposal); err != nil {
			return "", err
		}
		if contract.Kind == ContractImprovement && !request.Proposal.RequiresExperiment {
			return "", fmt.Errorf("open improvement contract requires an experiment-bound proposal")
		}
		return StateImprovementProposal, nil
	case EventExperimentRequired:
		if from != StateImprovementProposal && from != StateNeedsExperiment && from != StateHumanJudgmentRequired {
			return "", invalidTransition(from, request.Event)
		}
		if request.ExperimentID == "" {
			return "", fmt.Errorf("needs_experiment requires experiment_id")
		}
		if current.Proposal == nil {
			return "", fmt.Errorf("needs_experiment requires a persisted improvement proposal")
		}
		return StateNeedsExperiment, nil
	case EventHumanJudgmentRequested:
		// A governed forward mutation invalidates revision-bound evidence and
		// cycles the canonical state back through observation/diagnosis to
		// improvement_proposal while the experiment identity stays bound (see
		// Apply's project_revision_changed branch). The judgment boundary is
		// therefore legal from improvement_proposal, but only when the snapshot
		// still carries the experiment the judgment belongs to.
		if from != StateNeedsExperiment && from != StateImprovementProposal {
			return "", invalidTransition(from, request.Event)
		}
		if from == StateImprovementProposal && current.ExperimentID == "" {
			return "", fmt.Errorf("human judgment from improvement_proposal requires the bound experiment identity")
		}
		if request.ExperimentID == "" || (current.ExperimentID != "" && request.ExperimentID != current.ExperimentID) {
			return "", fmt.Errorf("human judgment experiment identity mismatch")
		}
		if request.PendingInteraction == nil || strings.TrimSpace(request.PendingInteraction.InteractionID) == "" || strings.TrimSpace(request.PendingInteraction.Kind) == "" {
			return "", fmt.Errorf("human_judgment_required needs a durable pending interaction")
		}
		return StateHumanJudgmentRequired, nil
	case EventCapabilityBlocked:
		if from.Terminal() {
			return "", invalidTransition(from, request.Event)
		}
		return StateCapabilityBlocked, nil
	case EventTaskSettled:
		if err := requireEvidence(); err != nil {
			return "", err
		}
		if contract.Kind == ContractImprovement {
			if from != StateNeedsExperiment && from != StateHumanJudgmentRequired {
				return "", fmt.Errorf("open improvement contract cannot settle from %s; a governed experiment outcome is required", from)
			}
			if current.ExperimentID == "" {
				return "", fmt.Errorf("open improvement settlement requires experiment identity")
			}
		} else if from != StateDiagnosticComplete && from != StateImprovementProposal && from != StateNeedsExperiment && from != StateHumanJudgmentRequired {
			return "", invalidTransition(from, request.Event)
		}
		return StateSettled, nil
	case EventTaskCancelled:
		if from.Terminal() {
			return "", invalidTransition(from, request.Event)
		}
		return StateCancelled, nil
	case EventTaskFailed:
		if from.Terminal() {
			return "", invalidTransition(from, request.Event)
		}
		return StateFailed, nil
	case EventOwnerTurnClosed:
		// A chat turn may admit a task contract and still end with a plain
		// conversational reply that schedules no continuation (the semantic
		// entry observation route answers directly). No runtime owns that
		// task's next semantic move any more, so the turn boundary closes it
		// honestly instead of leaving a non-terminal observation state that
		// nothing can ever advance. Legal from every non-terminal state; the
		// caller is responsible for verifying that no continuation is live.
		if from.Terminal() {
			return "", invalidTransition(from, request.Event)
		}
		return StateClosed, nil
	default:
		return "", fmt.Errorf("unsupported semantic transition event %q", request.Event)
	}
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("contract schema_version must be %s", SchemaVersion)
	}
	for name, value := range map[string]string{
		"contract_id": c.ContractID, "task_id": c.TaskID, "goal_id": c.GoalID,
		"run_id": c.RunID, "conversation_id": c.ConversationID,
		"original_intent": c.OriginalIntent, "authorization_boundary": c.AuthorizationBoundary,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("contract %s is required", name)
		}
	}
	if c.Kind != ContractDiagnostic && c.Kind != ContractImprovement {
		return fmt.Errorf("unsupported contract kind %q", c.Kind)
	}
	if strings.TrimSpace(c.Scope.Kind) == "" {
		return fmt.Errorf("contract scope.kind is required")
	}
	if len(c.CompletionCriteria) == 0 || len(c.EvidenceRequirements) == 0 {
		return fmt.Errorf("contract completion criteria and evidence requirements are required")
	}
	if c.Temporary && strings.TrimSpace(c.TargetDiscovery) == "" {
		return fmt.Errorf("temporary contract requires target_discovery")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("contract created_at is required")
	}
	return nil
}

func (s Snapshot) Validate(contract Contract) error {
	if s.SchemaVersion != SchemaVersion || s.ContractID != contract.ContractID {
		return fmt.Errorf("snapshot schema or contract identity mismatch")
	}
	if !s.State.Valid() || s.Revision == 0 || len(s.History) != int(s.Revision) {
		return fmt.Errorf("snapshot state, revision, or history is invalid")
	}
	if s.Terminal != s.State.Terminal() {
		return fmt.Errorf("snapshot terminal projection does not match state %s", s.State)
	}
	if s.State == StateNeedsExperiment && (s.Proposal == nil || s.ExperimentID == "") {
		return fmt.Errorf("needs_experiment snapshot is missing proposal or experiment identity")
	}
	if s.State == StateHumanJudgmentRequired && (s.ExperimentID == "" || s.PendingInteraction == nil) {
		return fmt.Errorf("human_judgment_required snapshot is incomplete")
	}
	for index, record := range s.History {
		if record.Revision != uint64(index+1) || !record.To.Valid() || record.OccurredAt.IsZero() {
			return fmt.Errorf("semantic transition history is corrupt at revision %d", index+1)
		}
		if index > 0 && record.From != s.History[index-1].To {
			return fmt.Errorf("semantic transition history is discontinuous at revision %d", index+1)
		}
	}
	if s.History[len(s.History)-1].To != s.State {
		return fmt.Errorf("semantic snapshot does not match transition history")
	}
	return nil
}

func (s State) Valid() bool {
	switch s {
	case StateObservationInProgress, StateDiagnosticComplete, StateNoCandidateFound,
		StateImprovementProposal, StateNeedsExperiment, StateHumanJudgmentRequired,
		StateCapabilityBlocked, StateSettled, StateCancelled, StateFailed, StateClosed:
		return true
	default:
		return false
	}
}

func (s State) Terminal() bool {
	switch s {
	case StateNoCandidateFound, StateCapabilityBlocked, StateSettled, StateCancelled, StateFailed, StateClosed:
		return true
	default:
		return false
	}
}

func NormalizeContract(contract Contract, now time.Time) Contract {
	contract.SchemaVersion = SchemaVersion
	contract.ContractID = strings.TrimSpace(contract.ContractID)
	contract.TaskID = strings.TrimSpace(contract.TaskID)
	contract.GoalID = strings.TrimSpace(contract.GoalID)
	contract.RunID = strings.TrimSpace(contract.RunID)
	contract.ConversationID = strings.TrimSpace(contract.ConversationID)
	contract.OriginalIntent = strings.TrimSpace(contract.OriginalIntent)
	contract.Scope.Kind = strings.TrimSpace(contract.Scope.Kind)
	contract.Scope.ID = strings.TrimSpace(contract.Scope.ID)
	contract.Scope.Label = strings.TrimSpace(contract.Scope.Label)
	contract.TargetDiscovery = strings.TrimSpace(contract.TargetDiscovery)
	contract.AuthorizationBoundary = strings.TrimSpace(contract.AuthorizationBoundary)
	contract.CompletionCriteria = unique(contract.CompletionCriteria)
	contract.EvidenceRequirements = unique(contract.EvidenceRequirements)
	contract.ProjectUUID = strings.TrimSpace(contract.ProjectUUID)
	contract.ProjectRevision = strings.TrimSpace(contract.ProjectRevision)
	if contract.CreatedAt.IsZero() {
		if now.IsZero() {
			now = time.Now().UTC()
		}
		contract.CreatedAt = now.UTC()
	}
	return contract
}

func normalizeRequest(request TransitionRequest) TransitionRequest {
	request.Reason = strings.TrimSpace(request.Reason)
	request.Summary = strings.TrimSpace(request.Summary)
	request.EvidenceRefs = unique(request.EvidenceRefs)
	request.CandidateID = strings.TrimSpace(request.CandidateID)
	request.ExperimentID = strings.TrimSpace(request.ExperimentID)
	request.ProjectRevision = strings.TrimSpace(request.ProjectRevision)
	if request.Proposal != nil {
		proposal := cloneProposal(*request.Proposal)
		proposal.ProposalID = strings.TrimSpace(proposal.ProposalID)
		proposal.Summary = strings.TrimSpace(proposal.Summary)
		proposal.EvidenceRefs = unique(proposal.EvidenceRefs)
		proposal.Bounds = unique(proposal.Bounds)
		request.Proposal = &proposal
		request.EvidenceRefs = unique(append(request.EvidenceRefs, proposal.EvidenceRefs...))
	}
	if request.PendingInteraction != nil {
		interaction := *request.PendingInteraction
		interaction.InteractionID = strings.TrimSpace(interaction.InteractionID)
		interaction.Kind = strings.TrimSpace(interaction.Kind)
		interaction.Reason = strings.TrimSpace(interaction.Reason)
		request.PendingInteraction = &interaction
	}
	return request
}

func validateProposal(proposal BoundedProposal) error {
	if strings.TrimSpace(proposal.ProposalID) == "" || strings.TrimSpace(proposal.Summary) == "" {
		return fmt.Errorf("bounded proposal requires proposal_id and summary")
	}
	if len(unique(proposal.EvidenceRefs)) == 0 {
		return fmt.Errorf("bounded proposal requires evidence_refs")
	}
	return nil
}

func invalidTransition(from State, event Event) error {
	return fmt.Errorf("semantic transition %s is not allowed from %s", event, from)
}

func unique(values []string) []string {
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

func cloneProposal(in BoundedProposal) BoundedProposal {
	in.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	in.Bounds = append([]string(nil), in.Bounds...)
	return in
}

func cloneSnapshot(in Snapshot) Snapshot {
	in.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	in.History = append([]TransitionRecord(nil), in.History...)
	for index := range in.History {
		in.History[index].EvidenceRefs = append([]string(nil), in.History[index].EvidenceRefs...)
		if in.History[index].PendingInteraction != nil {
			interaction := *in.History[index].PendingInteraction
			in.History[index].PendingInteraction = &interaction
		}
	}
	if in.Proposal != nil {
		proposal := cloneProposal(*in.Proposal)
		in.Proposal = &proposal
	}
	if in.PendingInteraction != nil {
		interaction := *in.PendingInteraction
		in.PendingInteraction = &interaction
	}
	return in
}
