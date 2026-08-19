// Package audioclosure owns the bounded, persistent controller for closing one
// acoustic problem. It consumes observation facts and capability settlements;
// it does not define either contract or perform DAW mutations itself.
package audioclosure

import "time"

const SchemaVersion = "minimal_audio_closure.v1"

type Mode string

const (
	ModeDiagnostic Mode = "diagnostic"
	ModeTreatment  Mode = "treatment"
)

type Phase string

const (
	PhaseObserving          Phase = "observing"
	PhaseReasoning          Phase = "reasoning"
	PhaseAwaitingCapability Phase = "awaiting_capability"
	PhaseVerifying          Phase = "verifying"
	PhaseSettled            Phase = "settled"
)

type Actionability string

const (
	ActionabilityUnknown       Actionability = "unknown"
	ActionabilityNonActionable Actionability = "non_actionable"
	ActionabilityActionable    Actionability = "actionable"
)

type StopReason string

const (
	StopSatisfied                     StopReason = "satisfied"
	StopDiagnosticComplete            StopReason = "diagnostic_complete"
	StopActionablePendingConfirmation StopReason = "actionable_pending_confirmation"
	StopInsufficientEvidence          StopReason = "insufficient_evidence"
	StopEvidenceCeilingReached        StopReason = "evidence_ceiling_reached"
	StopNoProgress                    StopReason = "no_progress"
	StopCapabilityUnavailable         StopReason = "capability_unavailable"
	StopPCAUnavailable                StopReason = "pca_unavailable"
	StopModelProtocolFailure          StopReason = "model_protocol_failure"
	StopTransportFailure              StopReason = "transport_failure"
	StopRoundLimit                    StopReason = "round_limit"
	StopActionFailed                  StopReason = "action_failed"
	StopVerificationFailedRolledBack  StopReason = "verification_failed_rolled_back"
	StopUserChoiceRequired            StopReason = "user_choice_required"
	StopProjectRevisionStale          StopReason = "project_revision_stale"
	StopHandoffRequested              StopReason = "handoff_requested"
	StopCancelled                     StopReason = "cancelled"
)

type Policy struct {
	MaxClosureRounds        int `json:"max_closure_rounds"`
	MaxUniqueObservations   int `json:"max_unique_observation_sets"`
	MaxNoProgressRounds     int `json:"max_no_progress_rounds"`
	MaxModelProtocolRepairs int `json:"max_model_protocol_repairs"`
	MaxActionAttempts       int `json:"max_action_attempts"`
	MaxRollbackAttempts     int `json:"max_rollback_attempts"`
}

func DefaultPolicy() Policy {
	return Policy{
		MaxClosureRounds: 6, MaxUniqueObservations: 4, MaxNoProgressRounds: 2,
		MaxModelProtocolRepairs: 1, MaxActionAttempts: 1, MaxRollbackAttempts: 1,
	}
}

type Scope struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}

type ObservationKey struct {
	ProjectUUID     string   `json:"project_uuid"`
	ProjectRevision string   `json:"project_revision,omitempty"`
	Scope           Scope    `json:"scope"`
	TargetRef       string   `json:"target_ref,omitempty"`
	ViewIDs         []string `json:"view_ids,omitempty"`
	ObservationMode string   `json:"observation_mode,omitempty"`
	Tap             string   `json:"tap,omitempty"`
	TimeWindow      string   `json:"time_window,omitempty"`
}

type ObservationRecord struct {
	Fingerprint     string    `json:"fingerprint"`
	ObservationID   string    `json:"observation_id,omitempty"`
	ProjectRevision string    `json:"project_revision,omitempty"`
	ViewIDs         []string  `json:"view_ids,omitempty"`
	Round           int       `json:"round"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type HypothesisFrontier struct {
	HypothesisIDs []string    `json:"hypothesis_ids,omitempty"`
	CandidateID   string      `json:"candidate_id,omitempty"`
	Candidates    []Candidate `json:"candidates,omitempty"`
	Blockers      []string    `json:"blockers,omitempty"`
}

// Candidate is a bounded, evidence-backed treatment target discovered from a
// project or multi-track observation. It is not a processor choice and grants
// no mutation authority; it exists so the next closure turn must choose a
// target instead of restarting broad observation.
type Candidate struct {
	ID                  string   `json:"id"`
	SourceObservationID string   `json:"source_observation_id"`
	ViewID              string   `json:"view_id"`
	IssueType           string   `json:"issue_type,omitempty"`
	Region              string   `json:"region,omitempty"`
	TrackIDs            []string `json:"track_ids"`
	TrackNames          []string `json:"track_names,omitempty"`
	EvidenceRefs        []string `json:"evidence_refs,omitempty"`
}

type CapabilityLink struct {
	SessionID               string `json:"session_id"`
	CapabilityID            string `json:"capability_id"`
	ActionID                string `json:"action_id"`
	ExpectedProjectRevision string `json:"expected_project_revision,omitempty"`
	ClosureRevision         uint64 `json:"closure_revision"`
	Status                  string `json:"status"`
}

type CapabilitySettlement struct {
	SessionID string    `json:"session_id"`
	ActionID  string    `json:"action_id"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	SettledAt time.Time `json:"settled_at"`
}

type Settlement struct {
	Reason                 StopReason `json:"reason"`
	Summary                string     `json:"summary,omitempty"`
	NeedsUserClarification bool       `json:"needs_user_clarification,omitempty"`
	HandoffController      string     `json:"handoff_controller,omitempty"`
	Round                  int        `json:"round"`
	SettledAt              time.Time  `json:"settled_at"`
}

type State struct {
	SchemaVersion         string                          `json:"schema_version"`
	ClosureID             string                          `json:"closure_id"`
	Revision              uint64                          `json:"revision"`
	ConversationID        string                          `json:"conversation_id"`
	GoalID                string                          `json:"goal_id,omitempty"`
	RunID                 string                          `json:"run_id,omitempty"`
	ProjectUUID           string                          `json:"project_uuid"`
	ProjectRevision       string                          `json:"project_revision,omitempty"`
	OriginalIntent        string                          `json:"original_intent"`
	Mode                  Mode                            `json:"mode"`
	Scope                 Scope                           `json:"scope"`
	Phase                 Phase                           `json:"phase"`
	Policy                Policy                          `json:"policy"`
	RoundsStarted         int                             `json:"rounds_started"`
	RoundInProgress       bool                            `json:"round_in_progress"`
	RoundHadProgress      bool                            `json:"round_had_progress"`
	NoProgressStreak      int                             `json:"no_progress_streak"`
	LastProjectChangeID   string                          `json:"last_project_change_id,omitempty"`
	ObservationOrder      []string                        `json:"observation_order,omitempty"`
	Observations          map[string]ObservationRecord    `json:"observations,omitempty"`
	Frontier              HypothesisFrontier              `json:"hypothesis_frontier,omitempty"`
	Actionability         Actionability                   `json:"actionability"`
	ModelProtocolRepairs  int                             `json:"model_protocol_repairs"`
	ActionAttempts        int                             `json:"action_attempts"`
	RollbackAttempts      int                             `json:"rollback_attempts"`
	ActiveCapability      *CapabilityLink                 `json:"active_capability_session,omitempty"`
	CapabilitySettlements map[string]CapabilitySettlement `json:"capability_settlements,omitempty"`
	ProjectCutRef         string                          `json:"project_cut_ref,omitempty"`
	Settlement            *Settlement                     `json:"settlement,omitempty"`
	Events                []Event                         `json:"events"`
	CreatedAt             time.Time                       `json:"created_at"`
	UpdatedAt             time.Time                       `json:"updated_at"`
}

func (s State) Terminal() bool { return s.Phase == PhaseSettled && s.Settlement != nil }

type StartRequest struct {
	ClosureID       string
	ConversationID  string
	GoalID          string
	RunID           string
	ProjectUUID     string
	ProjectRevision string
	OriginalIntent  string
	Mode            Mode
	Scope           Scope
	Policy          Policy
	Now             time.Time
}
