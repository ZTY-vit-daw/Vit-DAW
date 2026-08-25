// Package experiment implements the bounded free-state experiment runtime.
//
// The package owns Turn/Round lifecycle and the protocol gates between a
// typed intervention, its materiality, and its target response. It deliberately
// does not perform DAW mutations or choose CCB views; callers provide those
// results and the package preserves their evidence for the observable
// trajectory.
package experiment

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/taskstate"
	"vit-daw-agent/internal/trajectory"
)

const SchemaVersion = "vit.free_state_experiment_runtime.v1"

const (
	D1S1ActionDomain = "track_gain"
	D1S1ActionKind   = "track_gain_adjust"
)

// Status is the lifecycle state of a Turn.
type Status string

const (
	StatusFraming        Status = "framing"
	StatusObserving      Status = "observing"
	StatusHypothesizing  Status = "hypothesizing"
	StatusRunning        Status = "running"
	StatusWaitingForUser Status = "waiting_for_user"
	StatusSettled        Status = "settled"
	StatusStopped        Status = "stopped"
	StatusFailed         Status = "failed"
)

// RoundStatus is the lifecycle state of an Experiment Round.
type RoundStatus string

const (
	RoundAdmitted        RoundStatus = "admitted"
	RoundObserving       RoundStatus = "observing"
	RoundDoseCalibrating RoundStatus = "dose_calibrating"
	RoundTreating        RoundStatus = "treating"
	RoundVerifying       RoundStatus = "technically_verifying"
	RoundMateriality     RoundStatus = "materiality_evaluating"
	RoundTargetResponse  RoundStatus = "target_response_evaluating"
	RoundDeciding        RoundStatus = "deciding"
	RoundCompleted       RoundStatus = "completed"
	RoundRolledBack      RoundStatus = "rolled_back"
)

// TechnicalApplication is the governed action/readback result.
type TechnicalApplication string

const (
	TechnicalApplied   TechnicalApplication = "applied"
	TechnicalFailed    TechnicalApplication = "failed"
	TechnicalAmbiguous TechnicalApplication = "ambiguous"
)

// AcousticMateriality is intentionally separate from a parameter delta.
type AcousticMateriality string

const (
	MaterialityNone         AcousticMateriality = "none"
	MaterialitySubthreshold AcousticMateriality = "subthreshold"
	MaterialityMaterial     AcousticMateriality = "material"
)

// TargetResponse is the evidence-backed response to the admitted intent.
type TargetResponse string

const (
	TargetAbsent      TargetResponse = "absent"
	TargetDirectional TargetResponse = "directional"
	TargetSufficient  TargetResponse = "sufficient"
	TargetAmbiguous   TargetResponse = "ambiguous"
)

// RoundDecision is the controller decision after a valid post-action review.
type RoundDecision string

const (
	DecisionNextRound          RoundDecision = "next_round"
	DecisionRetain             RoundDecision = "retained"
	DecisionRollback           RoundDecision = "rolled_back"
	DecisionUserJudgment       RoundDecision = "user_judgment_pending"
	DecisionPlateau            RoundDecision = "plateau"
	DecisionBlockedObservation RoundDecision = "blocked_by_observation"
	DecisionBlockedCapability  RoundDecision = "blocked_by_capability"
	DecisionStopped            RoundDecision = "stopped"
)

// SettlementOutcome is the final, user-visible outcome of a Turn.
type SettlementOutcome string

const (
	OutcomeImproved           SettlementOutcome = "improved"
	OutcomeStable             SettlementOutcome = "stable"
	OutcomePlateau            SettlementOutcome = "plateau"
	OutcomeNeedsJudgment      SettlementOutcome = "needs_user_judgment"
	OutcomeBlockedCapability  SettlementOutcome = "blocked_by_capability"
	OutcomeBlockedObservation SettlementOutcome = "blocked_by_observation"
	OutcomeRolledBack         SettlementOutcome = "rolled_back"
	OutcomeBudgetExhausted    SettlementOutcome = "budget_exhausted"
	OutcomeUnsafe             SettlementOutcome = "unsafe_to_continue"
	OutcomeStopped            SettlementOutcome = "stopped"
)

// AuthorityMode controls whether an admitted experiment may mutate without a
// new per-action confirmation. It is a policy input, not mutation authority.
type AuthorityMode string

const (
	AuthorityOrdinary AuthorityMode = "ordinary"
	AuthorityFull     AuthorityMode = "full_access"
)

// Identity binds a Turn to the existing AgentEvent transport.
type Identity struct {
	ConversationID string `json:"conversation_id"`
	GoalID         string `json:"goal_id,omitempty"`
	RunID          string `json:"run_id,omitempty"`
	TurnID         string `json:"turn_id,omitempty"`
}

// Admission is the validated needs_experiment handoff. Maps are retained as
// opaque typed-workflow data; this runtime never interprets them as authority.
type Admission struct {
	SchemaVersion        string         `json:"schema_version"`
	TargetRef            map[string]any `json:"target_ref"`
	EvidenceRefs         []string       `json:"evidence_refs"`
	Hypothesis           string         `json:"hypothesis"`
	TypedAction          map[string]any `json:"typed_action"`
	DiagnosticDoseBounds map[string]any `json:"diagnostic_dose_bounds"`
	RetainedDoseBounds   map[string]any `json:"retained_dose_bounds"`
	ExperimentBudget     int            `json:"experiment_budget"`
	ExpectedEffect       string         `json:"expected_effect"`
	ProtectedDimensions  []string       `json:"protected_dimensions,omitempty"`
	VerificationPlan     map[string]any `json:"verification_plan"`
	CheckpointRef        string         `json:"checkpoint_ref"`
	RollbackPlan         map[string]any `json:"rollback_plan"`
	AuthorityMode        AuthorityMode  `json:"authority_mode"`
}

func (a Admission) Validate() error {
	if strings.TrimSpace(a.SchemaVersion) != SchemaVersion {
		return fmt.Errorf("admission schema_version must be %s", SchemaVersion)
	}
	if len(a.TargetRef) == 0 {
		return fmt.Errorf("admission target_ref is required")
	}
	if len(unique(a.EvidenceRefs)) == 0 {
		return fmt.Errorf("admission evidence_refs are required")
	}
	for name, value := range map[string]string{
		"hypothesis":      a.Hypothesis,
		"expected_effect": a.ExpectedEffect,
		"checkpoint_ref":  a.CheckpointRef,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("admission %s is required", name)
		}
	}
	if len(a.TypedAction) == 0 {
		return fmt.Errorf("admission typed_action is required")
	}
	if len(a.DiagnosticDoseBounds) == 0 || len(a.RetainedDoseBounds) == 0 {
		return fmt.Errorf("admission diagnostic and retained dose bounds are required")
	}
	if a.ExperimentBudget <= 0 {
		return fmt.Errorf("admission experiment_budget must be positive")
	}
	if len(a.VerificationPlan) == 0 {
		return fmt.Errorf("admission verification_plan is required")
	}
	if len(a.RollbackPlan) == 0 {
		return fmt.Errorf("admission rollback_plan is required")
	}
	switch a.AuthorityMode {
	case AuthorityOrdinary, AuthorityFull:
	default:
		return fmt.Errorf("unsupported authority_mode %q", a.AuthorityMode)
	}
	return nil
}

// ValidateD1S1 applies the deliberately narrow Phase D1-S1 admission. The
// general experiment schema remains readable for historical persisted turns;
// only a D1-S1 production entry may cross this gate.
func (a Admission) ValidateD1S1() error {
	if err := a.Validate(); err != nil {
		return err
	}
	if strings.ToLower(mapString(a.TypedAction, "action_domain", "domain")) != D1S1ActionDomain {
		return fmt.Errorf("D1-S1 action_domain must be %s", D1S1ActionDomain)
	}
	if strings.ToLower(mapString(a.TypedAction, "action_kind", "kind")) != D1S1ActionKind {
		return fmt.Errorf("D1-S1 action_kind must be %s", D1S1ActionKind)
	}
	if a.ExperimentBudget != 1 {
		return fmt.Errorf("D1-S1 experiment_budget must be 1")
	}
	if strings.ToLower(mapString(a.TargetRef, "kind")) != "track" || mapString(a.TargetRef, "id", "track_id") == "" {
		return fmt.Errorf("D1-S1 target must be an observation-bound track")
	}
	for name, bounds := range map[string]map[string]any{"diagnostic": a.DiagnosticDoseBounds, "retained": a.RetainedDoseBounds} {
		attempts, ok := mapNumber(bounds, "max_action_attempts")
		if !ok || attempts != 1 {
			return fmt.Errorf("D1-S1 %s max_action_attempts must be 1", name)
		}
		delta, ok := mapNumber(bounds, "delta_db")
		if !ok || delta == 0 || math.Abs(delta) > 2 {
			return fmt.Errorf("D1-S1 %s delta_db must be non-zero and within +/-2 dB", name)
		}
	}
	return nil
}

func (a Admission) IsD1S1() bool {
	return strings.EqualFold(mapString(a.TypedAction, "action_domain", "domain"), D1S1ActionDomain) ||
		strings.EqualFold(mapString(a.TypedAction, "action_kind", "kind"), D1S1ActionKind)
}

func mapString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := strings.TrimSpace(fmt.Sprint(row[key])); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func mapNumber(row map[string]any, key string) (float64, bool) {
	switch value := row[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case jsonNumber:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		return parsed, err == nil
	}
}

type jsonNumber interface {
	Float64() (float64, error)
}

// Observation is the compact CCB binding retained by a Round. The actual
// bundle remains owned by CCB; this record holds only auditable references and
// the exact requested/executed view-set receipt.
type Observation struct {
	ID               string         `json:"observation_id"`
	ReceiptID        string         `json:"receipt_id,omitempty"`
	RequestedViewIDs []string       `json:"requested_view_ids"`
	ExecutedViewIDs  []string       `json:"executed_view_ids"`
	ViewSetMatches   bool           `json:"view_set_matches"`
	Fresh            bool           `json:"fresh"`
	PostAction       bool           `json:"post_action"`
	ProjectRevision  string         `json:"project_revision,omitempty"`
	EvidenceRefs     []string       `json:"evidence_refs,omitempty"`
	Limitations      []string       `json:"limitations,omitempty"`
	Summary          map[string]any `json:"summary,omitempty"`
	RecordedAt       time.Time      `json:"recorded_at"`
}

func (o Observation) Validate(requireFresh bool) error {
	if strings.TrimSpace(o.ID) == "" {
		return fmt.Errorf("observation_id is required")
	}
	requested := unique(o.RequestedViewIDs)
	executed := unique(o.ExecutedViewIDs)
	if len(requested) == 0 {
		return fmt.Errorf("requested_view_ids are required")
	}
	if len(executed) == 0 {
		return fmt.Errorf("executed_view_ids are required")
	}
	if !sameStrings(requested, executed) || !o.ViewSetMatches {
		return fmt.Errorf("CCB requested/executed view set mismatch")
	}
	if requireFresh && !o.Fresh {
		return fmt.Errorf("observation must be fresh")
	}
	if len(unique(o.EvidenceRefs)) == 0 {
		return fmt.Errorf("observation evidence_refs are required")
	}
	return nil
}

// Intervention records one governed typed action attempt. The receipt proves
// application/readback only; it is not materiality or improvement evidence.
type Intervention struct {
	ID                   string               `json:"action_id"`
	Attempt              int                  `json:"attempt"`
	RequestedDelta       map[string]any       `json:"requested_delta,omitempty"`
	AchievedDelta        map[string]any       `json:"achieved_delta,omitempty"`
	ProcessorResponse    map[string]any       `json:"processor_response,omitempty"`
	RenderedEffect       string               `json:"rendered_effect,omitempty"`
	TechnicalApplication TechnicalApplication `json:"technical_application"`
	UserConfirmed        bool                 `json:"user_confirmed,omitempty"`
	PolicyAuthorized     bool                 `json:"policy_authorized,omitempty"`
	Receipt              map[string]any       `json:"receipt,omitempty"`
	EvidenceRefs         []string             `json:"evidence_refs,omitempty"`
	AppliedAt            time.Time            `json:"applied_at"`
}

func (i Intervention) Validate() error {
	if strings.TrimSpace(i.ID) == "" {
		return fmt.Errorf("action_id is required")
	}
	if i.Attempt <= 0 {
		return fmt.Errorf("attempt must be positive")
	}
	switch i.TechnicalApplication {
	case TechnicalApplied, TechnicalFailed, TechnicalAmbiguous:
	default:
		return fmt.Errorf("unsupported technical_application %q", i.TechnicalApplication)
	}
	if !i.UserConfirmed && !i.PolicyAuthorized {
		return fmt.Errorf("intervention requires user confirmation or bounded policy authorization")
	}
	if len(i.Receipt) == 0 {
		return fmt.Errorf("intervention receipt is required")
	}
	return nil
}

// MaterialityEvaluation is deliberately response-based. A subthreshold
// intervention is insufficient_dose and does not disprove its hypothesis.
type MaterialityEvaluation struct {
	State              AcousticMateriality        `json:"state"`
	Evaluation         trajectory.EvaluationState `json:"evaluation"`
	Summary            string                     `json:"summary,omitempty"`
	EvidenceRefs       []string                   `json:"evidence_refs"`
	Attempt            int                        `json:"attempt"`
	RenderedEffect     string                     `json:"rendered_effect,omitempty"`
	CalibrationProfile map[string]any             `json:"calibration_profile,omitempty"`
}

func (m MaterialityEvaluation) Validate() error {
	switch m.State {
	case MaterialityNone, MaterialitySubthreshold, MaterialityMaterial:
	default:
		return fmt.Errorf("unsupported materiality state %q", m.State)
	}
	switch m.Evaluation {
	case trajectory.EvaluationNotReady, trajectory.EvaluationInsufficientDose,
		trajectory.EvaluationAgentEvaluable, trajectory.EvaluationAmbiguous:
	default:
		return fmt.Errorf("unsupported materiality evaluation %q", m.Evaluation)
	}
	if m.Attempt <= 0 {
		return fmt.Errorf("materiality attempt must be positive")
	}
	if len(unique(m.EvidenceRefs)) == 0 {
		return fmt.Errorf("materiality evidence_refs are required")
	}
	if m.State == MaterialitySubthreshold && m.Evaluation != trajectory.EvaluationInsufficientDose {
		return fmt.Errorf("subthreshold materiality must be insufficient_dose")
	}
	if m.State == MaterialityMaterial && m.Evaluation == trajectory.EvaluationInsufficientDose {
		return fmt.Errorf("material materiality cannot be insufficient_dose")
	}
	return nil
}

// TargetEvaluation records the Agent's response classification after a fresh
// post-action CCB observation. The runtime requires materiality first.
type TargetEvaluation struct {
	Response                 TargetResponse             `json:"response"`
	Outcome                  trajectory.EvaluationState `json:"outcome"`
	Summary                  string                     `json:"summary,omitempty"`
	EvidenceRefs             []string                   `json:"evidence_refs"`
	ProtectedDimensionStatus map[string]any             `json:"protected_dimension_status,omitempty"`
}

func (t TargetEvaluation) Validate() error {
	switch t.Response {
	case TargetAbsent, TargetDirectional, TargetSufficient, TargetAmbiguous:
	default:
		return fmt.Errorf("unsupported target response %q", t.Response)
	}
	switch t.Outcome {
	case trajectory.EvaluationAgentEvaluable, trajectory.EvaluationHumanAuditionReady,
		trajectory.EvaluationHumanConfirmed, trajectory.EvaluationAmbiguous,
		trajectory.EvaluationUnsupportedHypothesis:
	default:
		return fmt.Errorf("unsupported target outcome %q", t.Outcome)
	}
	if len(unique(t.EvidenceRefs)) == 0 {
		return fmt.Errorf("target response evidence_refs are required")
	}
	return nil
}

// Round is a bounded cycle. Multiple Intervention records may occur in one
// Round while dose is calibrated.
type Round struct {
	ID                    string                 `json:"round_id"`
	Number                int                    `json:"number"`
	Status                RoundStatus            `json:"status"`
	Phase                 string                 `json:"phase"`
	CheckpointRef         string                 `json:"checkpoint_ref"`
	ProjectRevision       string                 `json:"project_revision,omitempty"`
	RequestedViewIDs      []string               `json:"requested_view_ids,omitempty"`
	Observations          []Observation          `json:"observations,omitempty"`
	Interventions         []Intervention         `json:"interventions,omitempty"`
	Materiality           *MaterialityEvaluation `json:"materiality,omitempty"`
	TargetResponse        *TargetEvaluation      `json:"target_response,omitempty"`
	AuditionSessionID     string                 `json:"audition_session_id,omitempty"`
	UserJudgmentRequested bool                   `json:"user_judgment_requested,omitempty"`
	UserJudgmentEvidence  []UserJudgmentEvidence `json:"user_judgment_evidence,omitempty"`
	AdoptedCandidateID    string                 `json:"adopted_candidate_id,omitempty"`
	AdoptionCheckpointRef string                 `json:"adoption_checkpoint_ref,omitempty"`
	AdoptionReceipt       map[string]any         `json:"adoption_receipt,omitempty"`
	RollbackReceipt       map[string]any         `json:"rollback_receipt,omitempty"`
	Decision              RoundDecision          `json:"decision,omitempty"`
	DecisionSummary       string                 `json:"decision_summary,omitempty"`
	StartedAt             time.Time              `json:"started_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// Turn is the persisted free-state experiment controller state.
type Turn struct {
	SchemaVersion     string            `json:"schema_version"`
	ID                string            `json:"turn_id"`
	ConversationID    string            `json:"conversation_id"`
	GoalID            string            `json:"goal_id,omitempty"`
	RunID             string            `json:"run_id,omitempty"`
	ContractID        string            `json:"contract_id,omitempty"`
	TaskState         taskstate.State   `json:"task_state,omitempty"`
	TaskStateRevision uint64            `json:"task_state_revision,omitempty"`
	Status            Status            `json:"status"`
	OriginalIntent    string            `json:"original_intent"`
	Admission         Admission         `json:"admission"`
	Rounds            []Round           `json:"rounds,omitempty"`
	CurrentRoundID    string            `json:"current_round_id,omitempty"`
	ProjectRevision   string            `json:"project_revision,omitempty"`
	Outcome           SettlementOutcome `json:"outcome,omitempty"`
	Settlement        string            `json:"settlement,omitempty"`
	LastTraceNodeID   string            `json:"last_trace_node_id,omitempty"`
	StartedAt         time.Time         `json:"started_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	SettledAt         time.Time         `json:"settled_at,omitempty"`
}

func NewTurn(identity Identity, originalIntent string, admission Admission, now time.Time) (Turn, error) {
	if strings.TrimSpace(identity.ConversationID) == "" {
		return Turn{}, fmt.Errorf("conversation_id is required")
	}
	if strings.TrimSpace(originalIntent) == "" {
		return Turn{}, fmt.Errorf("original_intent is required")
	}
	if err := admission.Validate(); err != nil {
		return Turn{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := strings.TrimSpace(identity.TurnID)
	if id == "" {
		id = newID("turn")
	}
	return Turn{
		SchemaVersion: SchemaVersion, ID: id,
		ConversationID: strings.TrimSpace(identity.ConversationID), GoalID: strings.TrimSpace(identity.GoalID),
		RunID: strings.TrimSpace(identity.RunID), Status: StatusRunning,
		OriginalIntent: strings.TrimSpace(originalIntent), Admission: cloneAdmission(admission),
		StartedAt: now.UTC(), UpdatedAt: now.UTC(),
	}, nil
}

func (t Turn) Validate() error {
	if t.SchemaVersion != SchemaVersion {
		return fmt.Errorf("turn schema_version must be %s", SchemaVersion)
	}
	if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.ConversationID) == "" {
		return fmt.Errorf("turn identity is required")
	}
	if strings.TrimSpace(t.OriginalIntent) == "" {
		return fmt.Errorf("turn original_intent is required")
	}
	if err := t.Admission.Validate(); err != nil {
		return fmt.Errorf("turn admission: %w", err)
	}
	if t.Status == "" {
		return fmt.Errorf("turn status is required")
	}
	if t.ContractID != "" && (!t.TaskState.Valid() || t.TaskStateRevision == 0) {
		return fmt.Errorf("turn canonical task projection is invalid")
	}
	return nil
}

// BindTaskState projects the product Task authority into the experiment. The
// experiment may refine rounds, but it cannot invent or override settlement.
func (t *Turn) BindTaskState(contractID string, state taskstate.State, revision uint64) error {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" || !state.Valid() || revision == 0 {
		return fmt.Errorf("canonical task contract, state, and revision are required")
	}
	if t.ContractID != "" && t.ContractID != contractID {
		return fmt.Errorf("experiment canonical contract identity mismatch")
	}
	if t.TaskStateRevision > revision || (t.TaskStateRevision == revision && t.TaskState != "" && t.TaskState != state) {
		return fmt.Errorf("experiment canonical task projection is stale or conflicting")
	}
	t.ContractID, t.TaskState, t.TaskStateRevision = contractID, state, revision
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// StartRound begins the next bounded cycle. It does not choose the view set;
// requestedViews are copied from the Agent's CCB request.
// StartEvents returns the initial Turn framing events. It is separate from
// NewTurn so callers can persist the validated Turn before publishing the
// transient projection.
func (t *Turn) StartEvents(now time.Time) []trajectory.Event {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return []trajectory.Event{
		t.events(now, trajectory.EventTurnStarted, "", trajectory.NodeTurn, "experiment turn started", nil, nil, map[string]any{"original_intent": t.OriginalIntent})[0],
		t.events(now, trajectory.EventIntentFramed, "", trajectory.NodeIntent, "experiment intent framed", t.Admission.EvidenceRefs, nil, map[string]any{"target_ref": t.Admission.TargetRef})[0],
		t.events(now, trajectory.EventHypothesisProposed, "", trajectory.NodeHypothesis, "experiment hypothesis proposed", t.Admission.EvidenceRefs, nil, map[string]any{"hypothesis": t.Admission.Hypothesis, "expected_effect": t.Admission.ExpectedEffect})[0],
	}
}

func (t *Turn) StartRound(requestedViews []string, checkpointRef, projectRevision string, now time.Time) ([]trajectory.Event, error) {
	if err := t.ensureLive(); err != nil {
		return nil, err
	}
	if t.Admission.IsD1S1() {
		if err := t.Admission.ValidateD1S1(); err != nil {
			return nil, err
		}
		if len(t.Rounds) > 0 {
			return nil, fmt.Errorf("D1-S1 permits one round")
		}
	}
	requestedViews = unique(requestedViews)
	if len(requestedViews) == 0 {
		return nil, fmt.Errorf("round requested view IDs are required")
	}
	checkpointRef = strings.TrimSpace(checkpointRef)
	if checkpointRef == "" {
		checkpointRef = t.Admission.CheckpointRef
	}
	if checkpointRef == "" {
		return nil, fmt.Errorf("round checkpoint_ref is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	number := len(t.Rounds) + 1
	round := Round{ID: newID(fmt.Sprintf("round-%d", number)), Number: number,
		Status: RoundAdmitted, Phase: "admitted", CheckpointRef: checkpointRef, RequestedViewIDs: append([]string(nil), requestedViews...),
		ProjectRevision: strings.TrimSpace(projectRevision), StartedAt: now.UTC(), UpdatedAt: now.UTC()}
	t.Rounds = append(t.Rounds, round)
	t.CurrentRoundID = round.ID
	t.ProjectRevision = round.ProjectRevision
	t.Status = StatusRunning
	t.UpdatedAt = now.UTC()
	return t.events(now, trajectory.EventRoundStarted, round.ID, trajectory.NodeDecision, "experiment round started", nil, nil, map[string]any{"requested_view_ids": requestedViews, "round_number": number}), nil
}

func (t *Turn) RecordObservation(observation Observation, postAction bool, now time.Time) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if err := observation.Validate(postAction); err != nil {
		return nil, err
	}
	if t.Admission.IsD1S1() {
		if strings.TrimSpace(observation.ProjectRevision) == "" {
			return nil, fmt.Errorf("D1-S1 observation must be revision-bound")
		}
		if postAction {
			if len(round.Interventions) != 1 {
				return nil, fmt.Errorf("D1-S1 post-action observation requires exactly one forward mutation")
			}
			afterRevision := mapString(round.Interventions[0].Receipt, "after_revision", "applied_revision")
			if afterRevision == "" {
				afterRevision = mapString(mapValue(round.Interventions[0].Receipt, "details"), "after_revision")
			}
			if afterRevision == "" || observation.ProjectRevision != afterRevision {
				return nil, fmt.Errorf("D1-S1 post-action observation revision must match mutation after revision")
			}
		}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	observation.PostAction = postAction
	observation.RequestedViewIDs = unique(observation.RequestedViewIDs)
	observation.ExecutedViewIDs = unique(observation.ExecutedViewIDs)
	observation.EvidenceRefs = unique(observation.EvidenceRefs)
	round.Observations = append(round.Observations, observation)
	round.Status = RoundObserving
	round.Phase = "observing"
	if postAction {
		round.Phase = "post_action_evaluation"
	}
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	details := map[string]any{"observation_id": observation.ID, "receipt_id": observation.ReceiptID,
		"requested_view_ids": observation.RequestedViewIDs, "actual_executed_view_ids": observation.ExecutedViewIDs,
		"view_set_matches": observation.ViewSetMatches, "fresh": observation.Fresh, "post_action": postAction}
	return t.events(now, trajectory.EventObservationRecorded, round.ID, trajectory.NodeObservation, "CCB observation recorded", observation.EvidenceRefs, nil, details), nil
}

func mapValue(row map[string]any, key string) map[string]any {
	value, _ := row[key].(map[string]any)
	return value
}

func (t *Turn) ApplyIntervention(intervention Intervention, now time.Time) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if t.Admission.IsD1S1() && len(round.Interventions) > 0 {
		return nil, fmt.Errorf("D1-S1 permits one forward mutation per round")
	}
	if t.InterventionCount() >= t.Admission.ExperimentBudget {
		return nil, fmt.Errorf("experiment budget exhausted")
	}
	if err := intervention.Validate(); err != nil {
		return nil, err
	}
	if intervention.Attempt != len(round.Interventions)+1 {
		return nil, fmt.Errorf("intervention attempt must be %d", len(round.Interventions)+1)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.Interventions = append(round.Interventions, intervention)
	round.Status = RoundTreating
	round.Phase = "treating"
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	return t.events(now, trajectory.EventInterventionApplied, round.ID, trajectory.NodeAction, "typed intervention applied", intervention.EvidenceRefs, []string{intervention.ID}, map[string]any{"attempt": intervention.Attempt, "technical_application": intervention.TechnicalApplication, "receipt": intervention.Receipt}), nil
}

func (t *Turn) EvaluateMateriality(evaluation MaterialityEvaluation, now time.Time) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if len(round.Interventions) == 0 {
		return nil, fmt.Errorf("materiality requires an intervention")
	}
	if evaluation.Attempt != len(round.Interventions) {
		return nil, fmt.Errorf("materiality attempt must match latest intervention")
	}
	if err := evaluation.Validate(); err != nil {
		return nil, err
	}
	latest := round.Interventions[len(round.Interventions)-1]
	if latest.TechnicalApplication != TechnicalApplied && evaluation.State == MaterialityMaterial {
		return nil, fmt.Errorf("materiality cannot be material after technical application %q", latest.TechnicalApplication)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.Materiality = &evaluation
	if evaluation.State == MaterialitySubthreshold {
		round.Status = RoundDoseCalibrating
		round.Phase = "dose_calibrating"
	} else {
		round.Status = RoundMateriality
		round.Phase = "materiality_evaluating"
	}
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	trajectoryMateriality := trajectory.EvaluationAgentEvaluable
	if evaluation.State == MaterialitySubthreshold {
		trajectoryMateriality = trajectory.EvaluationInsufficientDose
	}
	events := t.events(now, trajectory.EventMaterialityEvaluated, round.ID, trajectory.NodeMateriality, "intervention materiality evaluated", evaluation.EvidenceRefs, []string{latest.ID}, map[string]any{"acoustic_materiality": evaluation.State, "dose_attempt": evaluation.Attempt, "rendered_effect": evaluation.RenderedEffect, "evaluation": trajectoryMateriality})
	events[0].Payload.Materiality = trajectoryMateriality
	return events, nil
}

func (t *Turn) RecordTargetResponse(evaluation TargetEvaluation, now time.Time) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if round.Materiality == nil || (round.Materiality.State != MaterialityMaterial && !(t.Admission.IsD1S1() && round.Materiality.State == MaterialitySubthreshold && evaluation.Response == TargetAmbiguous && evaluation.Outcome == trajectory.EvaluationHumanAuditionReady)) {
		return nil, fmt.Errorf("target response requires material intervention; D1-S1 subthreshold only permits ambiguous human audition")
	}
	if len(round.Observations) == 0 || !round.Observations[len(round.Observations)-1].Fresh || !round.Observations[len(round.Observations)-1].PostAction {
		return nil, fmt.Errorf("target response requires a fresh post-action observation")
	}
	if err := evaluation.Validate(); err != nil {
		return nil, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.TargetResponse = &evaluation
	round.Status = RoundTargetResponse
	round.Phase = "target_response_evaluating"
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	events := t.events(now, trajectory.EventTargetResponse, round.ID, trajectory.NodeVerification, "target response evaluated", evaluation.EvidenceRefs, nil, map[string]any{"target_response": evaluation.Response, "outcome": evaluation.Outcome, "protected_dimension_status": evaluation.ProtectedDimensionStatus})
	events[0].Payload.TargetResponse = string(evaluation.Response)
	events[0].Payload.Outcome = evaluation.Outcome
	return events, nil
}

func (t *Turn) DecideRound(decision RoundDecision, summary string, now time.Time) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if err := validateDecision(decision); err != nil {
		return nil, err
	}
	if decision != DecisionRollback && decision != DecisionStopped && round.TargetResponse == nil && !(decision == DecisionNextRound && round.Materiality != nil && round.Materiality.State == MaterialitySubthreshold) && decision != DecisionUserJudgment && decision != DecisionPlateau && decision != DecisionBlockedObservation && decision != DecisionBlockedCapability {
		return nil, fmt.Errorf("round decision requires target response")
	}
	if decision == DecisionRetain && (round.TargetResponse == nil || round.TargetResponse.Response != TargetSufficient) {
		return nil, fmt.Errorf("retained decision requires sufficient target response")
	}
	if round.Materiality != nil && round.Materiality.State == MaterialitySubthreshold && decision == DecisionPlateau {
		return nil, fmt.Errorf("insufficient_dose must not disprove the hypothesis")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.Decision = decision
	round.DecisionSummary = strings.TrimSpace(summary)
	round.Status = RoundCompleted
	round.Phase = "deciding"
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	details := map[string]any{"decision": decision, "summary": summary}
	events := t.events(now, trajectory.EventRoundDecision, round.ID, trajectory.NodeDecision, "experiment round decision", nil, nil, details)
	events[0].Payload.NextDecision = string(decision)
	return events, nil
}

func (t *Turn) MarkRollback(now time.Time, receipt map[string]any, evidenceRefs []string) ([]trajectory.Event, error) {
	round, err := t.currentRound()
	if err != nil {
		return nil, err
	}
	if len(receipt) == 0 {
		return nil, fmt.Errorf("rollback receipt is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	round.Status = RoundRolledBack
	round.Phase = "rolled_back"
	round.Decision = DecisionRollback
	round.RollbackReceipt = cloneMap(receipt)
	round.UpdatedAt = now.UTC()
	t.replaceRound(*round)
	events := t.events(now, trajectory.EventRollbackCompleted, round.ID, trajectory.NodeRollback, "experiment round rolled back", unique(evidenceRefs), nil, map[string]any{"rollback_receipt": receipt, "checkpoint_ref": round.CheckpointRef})
	events[0].Payload.Outcome = trajectory.EvaluationRolledBack
	return events, nil
}

func (t *Turn) BindCandidateAdoption(candidateID, checkpointRef, projectRevision string, receipt map[string]any) {
	round, err := t.currentRound()
	if err != nil {
		return
	}
	round.AdoptedCandidateID = strings.TrimSpace(candidateID)
	round.AdoptionCheckpointRef = strings.TrimSpace(checkpointRef)
	round.AdoptionReceipt = cloneMap(receipt)
	if round.AdoptionCheckpointRef != "" {
		round.CheckpointRef = round.AdoptionCheckpointRef
	}
	if strings.TrimSpace(projectRevision) != "" {
		round.ProjectRevision = strings.TrimSpace(projectRevision)
		t.ProjectRevision = round.ProjectRevision
	}
	round.UpdatedAt = time.Now().UTC()
	t.replaceRound(*round)
}

func (t *Turn) Settle(outcome SettlementOutcome, summary string, now time.Time) ([]trajectory.Event, error) {
	if err := t.ensureLive(); err != nil {
		return nil, err
	}
	if err := validateOutcome(outcome); err != nil {
		return nil, err
	}
	if t.ContractID != "" {
		expected := taskstate.StateSettled
		switch outcome {
		case OutcomeNeedsJudgment:
			return nil, fmt.Errorf("needs_user_judgment is a non-terminal task state; request durable user judgment instead")
		case OutcomeBlockedCapability, OutcomeBlockedObservation, OutcomeBudgetExhausted:
			expected = taskstate.StateCapabilityBlocked
		case OutcomeUnsafe:
			expected = taskstate.StateFailed
		case OutcomeStopped:
			expected = taskstate.StateCancelled
		}
		if t.TaskState != expected {
			return nil, fmt.Errorf("experiment settlement %s requires canonical task state %s, got %s", outcome, expected, t.TaskState)
		}
	}
	if len(t.Rounds) == 0 && outcome != OutcomeStopped {
		return nil, fmt.Errorf("settlement requires at least one round")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	t.Outcome = outcome
	t.Settlement = strings.TrimSpace(summary)
	t.Status = StatusSettled
	t.SettledAt = now.UTC()
	t.UpdatedAt = now.UTC()
	details := map[string]any{"outcome": outcome, "summary": summary}
	if round, roundErr := t.currentRound(); roundErr == nil {
		if round.AdoptedCandidateID != "" {
			details["adopted_candidate_id"] = round.AdoptedCandidateID
		}
		if round.AdoptionCheckpointRef != "" {
			details["adoption_checkpoint_ref"] = round.AdoptionCheckpointRef
		}
		if len(round.AdoptionReceipt) > 0 {
			details["adoption_receipt"] = cloneMap(round.AdoptionReceipt)
		}
	}
	events := t.events(now, trajectory.EventSettled, "", trajectory.NodeSettlement, "experiment settled", nil, nil, details)
	if round, roundErr := t.currentRound(); roundErr == nil {
		events[0].Payload.CheckpointRef = round.CheckpointRef
		events[0].Payload.ProjectRevision = round.ProjectRevision
	}
	switch outcome {
	case OutcomeImproved, OutcomeStable:
		events[0].Payload.Outcome = trajectory.EvaluationAgentEvaluable
	case OutcomeNeedsJudgment:
		events[0].Payload.Outcome = trajectory.EvaluationHumanAuditionReady
	case OutcomeRolledBack:
		events[0].Payload.Outcome = trajectory.EvaluationRolledBack
	case OutcomeBlockedCapability, OutcomeBlockedObservation, OutcomeBudgetExhausted, OutcomeUnsafe, OutcomeStopped:
		events[0].Payload.Outcome = trajectory.EvaluationNotReady
	case OutcomePlateau:
		events[0].Payload.Outcome = trajectory.EvaluationUnsupportedHypothesis
	}
	return events, nil
}

func (t *Turn) Stop(summary string, now time.Time) ([]trajectory.Event, error) {
	if t.Status == StatusSettled || t.Status == StatusStopped {
		return nil, fmt.Errorf("turn is already terminal")
	}
	if t.ContractID != "" && t.TaskState != taskstate.StateCancelled {
		return nil, fmt.Errorf("experiment stop requires canonical task state cancelled")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	t.Status = StatusStopped
	t.Outcome = OutcomeStopped
	t.Settlement = strings.TrimSpace(summary)
	t.UpdatedAt = now.UTC()
	return t.events(now, trajectory.EventTurnStopped, "", trajectory.NodeTurn, "experiment turn stopped", nil, nil, map[string]any{"summary": summary}), nil
}

func (t *Turn) ensureLive() error {
	if err := t.Validate(); err != nil {
		return err
	}
	switch t.Status {
	case StatusSettled, StatusStopped, StatusFailed:
		return fmt.Errorf("turn is terminal: %s", t.Status)
	}
	return nil
}

// CurrentRound returns a copy of the active round for integration layers.
// InterventionCount returns the number of governed attempts across all
// rounds, which is the budget unit for a free-state experiment.
func (t *Turn) InterventionCount() int {
	count := 0
	for _, round := range t.Rounds {
		count += len(round.Interventions)
	}
	return count
}

func (t *Turn) CurrentRound() (Round, error) {
	round, err := t.currentRound()
	if err != nil {
		return Round{}, err
	}
	return *round, nil
}

func (t *Turn) currentRound() (*Round, error) {
	if strings.TrimSpace(t.CurrentRoundID) == "" {
		return nil, fmt.Errorf("turn has no current round")
	}
	for i := range t.Rounds {
		if t.Rounds[i].ID == t.CurrentRoundID {
			return &t.Rounds[i], nil
		}
	}
	return nil, fmt.Errorf("current round %q not found", t.CurrentRoundID)
}

func (t *Turn) replaceRound(round Round) {
	for i := range t.Rounds {
		if t.Rounds[i].ID == round.ID {
			t.Rounds[i] = round
			return
		}
	}
}

func (t *Turn) events(now time.Time, typ trajectory.EventType, roundID string, kind trajectory.NodeKind, summary string, evidenceRefs, actionRefs []string, details map[string]any) []trajectory.Event {
	payload := trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TurnID: t.ID, RoundID: roundID, NodeKind: kind, Phase: trajectory.DefaultPhase(typ), Status: trajectory.DefaultStatus(typ), Summary: summary, EvidenceRefs: unique(evidenceRefs), ActionRefs: unique(actionRefs), ProjectRevision: t.ProjectRevision, CheckpointRef: t.Admission.CheckpointRef, Outcome: trajectory.EvaluationNotReady, Details: details}
	event := trajectory.Event{Type: typ, ConversationID: t.ConversationID, GoalID: t.GoalID, RunID: t.RunID, ItemID: newID("trace"), Title: summary, Body: summary, Payload: payload}.Normalize()
	event.Payload.ParentNodeID = t.LastTraceNodeID
	if err := event.Validate(); err == nil {
		t.LastTraceNodeID = event.Payload.TraceNodeID
	}
	return []trajectory.Event{event}
}

func validateDecision(value RoundDecision) error {
	switch value {
	case DecisionNextRound, DecisionRetain, DecisionRollback, DecisionUserJudgment, DecisionPlateau, DecisionBlockedObservation, DecisionBlockedCapability, DecisionStopped:
		return nil
	default:
		return fmt.Errorf("unsupported round decision %q", value)
	}
}
func validateOutcome(value SettlementOutcome) error {
	switch value {
	case OutcomeImproved, OutcomeStable, OutcomePlateau, OutcomeNeedsJudgment, OutcomeBlockedCapability, OutcomeBlockedObservation, OutcomeRolledBack, OutcomeBudgetExhausted, OutcomeUnsafe, OutcomeStopped:
		return nil
	default:
		return fmt.Errorf("unsupported settlement outcome %q", value)
	}
}
func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
func sameStrings(a, b []string) bool {
	a = unique(a)
	b = unique(b)
	if len(a) != len(b) {
		return false
	}
	for _, value := range a {
		found := false
		for _, candidate := range b {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func cloneAdmission(a Admission) Admission {
	a.TargetRef = cloneMap(a.TargetRef)
	a.TypedAction = cloneMap(a.TypedAction)
	a.DiagnosticDoseBounds = cloneMap(a.DiagnosticDoseBounds)
	a.RetainedDoseBounds = cloneMap(a.RetainedDoseBounds)
	a.VerificationPlan = cloneMap(a.VerificationPlan)
	a.RollbackPlan = cloneMap(a.RollbackPlan)
	a.EvidenceRefs = unique(a.EvidenceRefs)
	a.ProtectedDimensions = unique(a.ProtectedDimensions)
	return a
}
func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
