package audioclosure

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/taskstate"
)

type Driver struct{ Policy Policy }

type ObservationOutcome struct {
	State       State
	Fingerprint string
	Accepted    bool
	Duplicate   bool
}

func Start(request StartRequest) (State, error) {
	request.ClosureID, request.ConversationID = normalizeText(request.ClosureID), normalizeText(request.ConversationID)
	request.ProjectUUID, request.OriginalIntent = normalizeText(request.ProjectUUID), normalizeText(request.OriginalIntent)
	if request.ClosureID == "" || request.ConversationID == "" || request.ProjectUUID == "" || request.OriginalIntent == "" {
		return State{}, fmt.Errorf("closure_id, conversation_id, project_uuid, and original_intent are required")
	}
	if request.Mode != ModeDiagnostic && request.Mode != ModeTreatment {
		return State{}, fmt.Errorf("invalid closure mode %q", request.Mode)
	}
	policy := normalizePolicy(request.Policy)
	if err := validatePolicy(policy); err != nil {
		return State{}, err
	}
	if request.Now.IsZero() {
		request.Now = time.Now().UTC()
	}
	data := startedData{
		ConversationID: request.ConversationID, TaskID: normalizeText(request.TaskID), GoalID: normalizeText(request.GoalID), RunID: normalizeText(request.RunID),
		ContractID: normalizeText(request.ContractID), TaskState: request.TaskState, TaskStateRevision: request.TaskStateRevision,
		ProjectUUID: request.ProjectUUID, ProjectRevision: normalizeText(request.ProjectRevision), OriginalIntent: request.OriginalIntent,
		Mode: request.Mode, Scope: request.Scope, Policy: policy,
	}
	raw, _ := json.Marshal(data)
	event := Event{EventID: request.ClosureID + ":1", ClosureID: request.ClosureID, Sequence: 1, Type: EventStarted, OccurredAt: request.Now.UTC(), Data: raw}
	return Fold([]Event{event})
}

// ProjectTaskState records the canonical Task authority in the closure event
// stream. Closure-local phases and stop reasons never override this state.
func (d Driver) ProjectTaskState(state State, expectedRevision uint64, contractID string, semanticState taskstate.State, semanticRevision uint64, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	contractID = normalizeText(contractID)
	if contractID == "" || contractID != state.ContractID || !semanticState.Valid() {
		return State{}, false, fmt.Errorf("canonical task projection does not match closure")
	}
	if semanticRevision == state.TaskStateRevision && semanticState == state.TaskState {
		return state, false, nil
	}
	if semanticRevision <= state.TaskStateRevision {
		return State{}, false, fmt.Errorf("canonical task revision is stale")
	}
	next, err := appendEvent(state, EventTaskStateProjected, taskStateProjectedData{ContractID: contractID, State: semanticState, Revision: semanticRevision}, now)
	return next, err == nil, err
}

func (d Driver) RevalidateProjectRevision(state State, expectedRevision uint64, projectRevision string, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	projectRevision = normalizeText(projectRevision)
	if projectRevision == "" {
		return State{}, false, fmt.Errorf("project_revision is required")
	}
	if projectRevision == state.ProjectRevision {
		return state, false, nil
	}
	if state.ActiveCapability != nil {
		return State{}, false, fmt.Errorf("active capability must settle before project revision revalidation")
	}
	next, err := appendEvent(state, EventProjectRevisionChanged, projectRevisionChangedData{ProjectRevision: projectRevision}, now)
	return next, err == nil, err
}

// RecordGovernedMutation books the project revision advance produced by the
// active capability's own receipted mutation. External revision drift during
// an active capability must still settle stale (RevalidateProjectRevision
// refuses it); only the authoritative applied revision of the in-flight
// governed mutation may advance the tracked revision in place, because the
// post-action observation is recorded against exactly that revision.
func (d Driver) RecordGovernedMutation(state State, expectedRevision uint64, appliedProjectRevision string, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	appliedProjectRevision = normalizeText(appliedProjectRevision)
	if appliedProjectRevision == "" {
		return State{}, fmt.Errorf("applied project revision is required")
	}
	if state.ActiveCapability == nil {
		return State{}, fmt.Errorf("governed mutation booking requires an active capability")
	}
	if appliedProjectRevision == state.ProjectRevision {
		return state, nil
	}
	next, err := appendEvent(state, EventGovernedRevisionBooked, governedRevisionBookedData{ProjectRevision: appliedProjectRevision}, now)
	return next, err
}

func (d Driver) AdmitRound(state State, expectedRevision uint64, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	if state.Terminal() {
		return state, false, nil
	}
	if state.RoundInProgress {
		return state, true, nil
	}
	if state.ActiveCapability != nil {
		return state, false, fmt.Errorf("capability session %s owns the next transition", state.ActiveCapability.SessionID)
	}
	policy := d.policyFor(state)
	if state.RoundsStarted >= policy.MaxClosureRounds {
		if state.ContractID != "" {
			return state, false, nil
		}
		settled, err := d.settleUnchecked(state, StopRoundLimit, "closure round budget exhausted", false, "", now)
		return settled, false, err
	}
	next, err := appendEvent(state, EventRoundStarted, roundStartedData{Round: state.RoundsStarted + 1}, now)
	return next, err == nil, err
}

// ExtendClosureRounds records a one-time durable verification allowance after
// a governed mutation lands on the final diagnostic round.
func (d Driver) ExtendClosureRounds(state State, expectedRevision uint64, maxRounds int, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	if maxRounds <= state.Policy.MaxClosureRounds || maxRounds < state.RoundsStarted {
		return State{}, fmt.Errorf("closure policy extension must increase max rounds")
	}
	return appendEvent(state, EventPolicyExtended, policyExtendedData{MaxClosureRounds: maxRounds}, now)
}

func (d Driver) RecordObservation(state State, expectedRevision uint64, key ObservationKey, observationID string, now time.Time) (ObservationOutcome, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return ObservationOutcome{}, err
	}
	if !state.RoundInProgress {
		return ObservationOutcome{}, fmt.Errorf("observation requires an admitted round")
	}
	if key.ProjectUUID != "" && key.ProjectUUID != state.ProjectUUID {
		return ObservationOutcome{}, fmt.Errorf("observation project_uuid does not match closure")
	}
	if key.ProjectRevision != "" && state.SupersededProjectRevisions[key.ProjectRevision] {
		// Replayed evidence from a revision that an in-flight governed
		// mutation replaced: superseded history, not fresh evidence and not
		// external drift, so it is skipped instead of settling the closure.
		return ObservationOutcome{State: state}, nil
	}
	if key.ProjectRevision != "" && state.ProjectRevision != "" && key.ProjectRevision != state.ProjectRevision {
		settled, err := d.settleUnchecked(state, StopProjectRevisionStale, "observation belongs to a different project revision", false, "", now)
		return ObservationOutcome{State: settled}, err
	}
	fingerprint, err := ObservationFingerprint(key)
	if err != nil {
		return ObservationOutcome{}, err
	}
	if _, duplicate := state.Observations[fingerprint]; duplicate {
		record := state.Observations[fingerprint]
		next, appendErr := appendEvent(state, EventObservationRecorded, observationRecordedData{Record: record, Duplicate: true}, now)
		return ObservationOutcome{State: next, Fingerprint: fingerprint, Duplicate: true}, appendErr
	}
	policy := d.policyFor(state)
	if len(state.Observations) >= policy.MaxUniqueObservations {
		if state.ContractID != "" {
			return ObservationOutcome{State: state, Fingerprint: fingerprint}, nil
		}
		settled, settleErr := d.settleUnchecked(state, StopEvidenceCeilingReached, "unique observation budget exhausted", false, "", now)
		return ObservationOutcome{State: settled, Fingerprint: fingerprint}, settleErr
	}
	record := ObservationRecord{Fingerprint: fingerprint, ObservationID: normalizeText(observationID), TargetRef: normalizeText(key.TargetRef), ProjectRevision: normalizeText(key.ProjectRevision), ViewIDs: normalizedStrings(key.ViewIDs), Round: state.RoundsStarted, RecordedAt: utcNow(now)}
	next, err := appendEvent(state, EventObservationRecorded, observationRecordedData{Record: record}, now)
	return ObservationOutcome{State: next, Fingerprint: fingerprint, Accepted: err == nil}, err
}

// RecordProjectChange records an authoritative engineering transition as
// closure progress. It carries no acoustic conclusion; callers still need a
// fresh CCB observation before post-action evaluation.
func (d Driver) RecordProjectChange(state State, expectedRevision uint64, changeID string, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	if !state.RoundInProgress {
		return State{}, false, fmt.Errorf("project change requires an admitted round")
	}
	changeID = normalizeText(changeID)
	if changeID == "" {
		return State{}, false, fmt.Errorf("project change id is required")
	}
	if state.LastProjectChangeID == changeID {
		return state, false, nil
	}
	next, err := appendEvent(state, EventProjectChangeRecorded, projectChangeRecordedData{ChangeID: changeID}, now)
	return next, err == nil, err
}

func (d Driver) UpdateFrontier(state State, expectedRevision uint64, frontier HypothesisFrontier, actionability Actionability, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	if !state.RoundInProgress {
		return State{}, false, fmt.Errorf("frontier update requires an admitted round")
	}
	frontier = normalizeFrontier(frontier)
	if actionability == "" {
		actionability = ActionabilityUnknown
	}
	if actionability != ActionabilityUnknown && actionability != ActionabilityNonActionable && actionability != ActionabilityActionable {
		return State{}, false, fmt.Errorf("invalid actionability %q", actionability)
	}
	progress := frontierFingerprint(frontier) != frontierFingerprint(state.Frontier) || actionability != state.Actionability
	next, err := appendEvent(state, EventFrontierUpdated, frontierUpdatedData{Frontier: frontier, Actionability: actionability, Progress: progress}, now)
	return next, progress, err
}

func (d Driver) CompleteRound(state State, expectedRevision uint64, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	if !state.RoundInProgress {
		return State{}, fmt.Errorf("no round is in progress")
	}
	next, err := appendEvent(state, EventRoundCompleted, roundCompletedData{}, now)
	if err != nil {
		return State{}, err
	}
	policy := d.policyFor(next)
	// A project-scoped closure needs a bounded discovery window before a
	// relationship observation can expose its first candidate. Do not let the
	// generic no-progress guard terminate catalog/scope establishment before
	// that frontier exists; MaxUniqueObservations and MaxClosureRounds still
	// bound this phase. Once candidates exist, unchanged rounds are ordinary
	// closure stalls and use the normal no-progress stop.
	frontierEstablished := len(next.Frontier.Candidates) > 0
	if next.NoProgressStreak >= policy.MaxNoProgressRounds && (next.Scope.Kind != "project" || frontierEstablished) {
		if next.ContractID != "" {
			return next, nil
		}
		return d.settleUnchecked(next, StopNoProgress, "closure made no material progress in consecutive rounds", false, "", now)
	}
	// An actionable final round may hand off to a governed capability. The
	// round ceiling blocks more observation/reasoning, not the first bounded
	// execution handoff that the admitted round just produced.
	if next.RoundsStarted >= policy.MaxClosureRounds && next.Actionability != ActionabilityActionable {
		if next.ContractID != "" {
			return next, nil
		}
		return d.settleUnchecked(next, StopRoundLimit, "closure round budget exhausted", false, "", now)
	}
	return next, nil
}

func (d Driver) RecordProtocolRepair(state State, expectedRevision uint64, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	policy := d.policyFor(state)
	if state.ModelProtocolRepairs >= policy.MaxModelProtocolRepairs {
		if state.ContractID != "" {
			return state, false, nil
		}
		settled, err := d.settleUnchecked(state, StopModelProtocolFailure, "model output remained invalid after the repair budget", false, "", now)
		return settled, false, err
	}
	next, err := appendEvent(state, EventProtocolRepairRecorded, protocolRepairData{Count: state.ModelProtocolRepairs + 1}, now)
	return next, err == nil, err
}

func (d Driver) BeginCapability(state State, expectedRevision uint64, link CapabilityLink, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	link.SessionID, link.CapabilityID, link.ActionID = normalizeText(link.SessionID), normalizeText(link.CapabilityID), normalizeText(link.ActionID)
	if link.SessionID == "" || link.CapabilityID == "" || link.ActionID == "" {
		return State{}, false, fmt.Errorf("session_id, capability_id, and action_id are required")
	}
	if settled, exists := state.CapabilitySettlements[link.ActionID]; exists {
		return state, false, fmt.Errorf("action %s already settled in session %s", link.ActionID, settled.SessionID)
	}
	if active := state.ActiveCapability; active != nil {
		if active.SessionID == link.SessionID && active.ActionID == link.ActionID {
			return state, false, nil
		}
		return State{}, false, fmt.Errorf("capability session %s already owns the closure", active.SessionID)
	}
	if state.ActionAttempts >= d.policyFor(state).MaxActionAttempts {
		settled, err := d.settleUnchecked(state, StopActionFailed, "action attempt budget exhausted", false, "", now)
		return settled, false, err
	}
	link.ClosureRevision, link.Status = state.Revision, "active"
	next, err := appendEvent(state, EventCapabilityStarted, capabilityStartedData{Link: link}, now)
	return next, err == nil, err
}

func (d Driver) SettleCapability(state State, expectedRevision uint64, settlement CapabilitySettlement, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	settlement.SessionID, settlement.ActionID, settlement.Status = normalizeText(settlement.SessionID), normalizeText(settlement.ActionID), strings.ToLower(normalizeText(settlement.Status))
	if existing, exists := state.CapabilitySettlements[settlement.ActionID]; exists {
		if existing.SessionID == settlement.SessionID && existing.Status == settlement.Status {
			return state, false, nil
		}
		return State{}, false, fmt.Errorf("action %s already has a different settlement", settlement.ActionID)
	}
	if state.ActiveCapability == nil {
		return State{}, false, fmt.Errorf("no capability session is active")
	}
	settlement.SettledAt = utcNow(now)
	next, err := appendEvent(state, EventCapabilitySettled, capabilitySettledData{Settlement: settlement}, now)
	if err != nil {
		return State{}, false, err
	}
	switch settlement.Status {
	case "completed", "needs_review":
		return next, true, nil
	case "stale":
		if next.ContractID != "" {
			settled, settleErr := d.settleUnchecked(next, StopCapabilityBlocked, firstNonEmpty(settlement.Reason, "capability project revision became stale"), false, "", now)
			return settled, true, settleErr
		}
		settled, settleErr := d.settleUnchecked(next, StopProjectRevisionStale, firstNonEmpty(settlement.Reason, "capability project revision became stale"), false, "", now)
		return settled, true, settleErr
	case "cancelled":
		settled, settleErr := d.settleUnchecked(next, StopCancelled, firstNonEmpty(settlement.Reason, "capability session was cancelled"), false, "", now)
		return settled, true, settleErr
	default:
		if next.ContractID != "" {
			settled, settleErr := d.settleUnchecked(next, StopCapabilityBlocked, firstNonEmpty(settlement.Reason, "capability action failed"), false, "", now)
			return settled, true, settleErr
		}
		settled, settleErr := d.settleUnchecked(next, StopActionFailed, firstNonEmpty(settlement.Reason, "capability action failed"), false, "", now)
		return settled, true, settleErr
	}
}

func (d Driver) RecordRollback(state State, expectedRevision uint64, now time.Time) (State, bool, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, false, err
	}
	if state.RollbackAttempts >= d.policyFor(state).MaxRollbackAttempts {
		return state, false, nil
	}
	next, err := appendEvent(state, EventRollbackStarted, rollbackStartedData{Count: state.RollbackAttempts + 1}, now)
	return next, err == nil, err
}

func (d Driver) Settle(state State, expectedRevision uint64, reason StopReason, summary string, needsClarification bool, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	return d.settleUnchecked(state, reason, summary, needsClarification, "", now)
}

func (d Driver) RequestHandoff(state State, expectedRevision uint64, controller, summary string, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	controller = normalizeText(controller)
	if controller == "" {
		return State{}, fmt.Errorf("handoff controller is required")
	}
	return d.settleUnchecked(state, StopHandoffRequested, summary, false, controller, now)
}

// TransitionPhase appends a phase_transition event. The guard input snapshot
// travels with the event so Fold re-verifies it deterministically on replay.
func (d Driver) TransitionPhase(state State, expectedRevision uint64, to Phase, guard PhaseGuardInput, reason string, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	to = Phase(strings.ToLower(strings.TrimSpace(string(to))))
	if !IsFSPhase(to) {
		return State{}, fmt.Errorf("transition target must be an FS phase, got %q", to)
	}
	from := state.Phase
	if to == PhaseFS0SemanticEntry {
		if IsFSPhase(from) {
			return State{}, fmt.Errorf("FS0 entry requires a non-FS current phase, got %s", from)
		}
	} else {
		if !IsFSPhase(from) {
			return State{}, fmt.Errorf("the FS machine must be entered at %s before %s", PhaseFS0SemanticEntry, to)
		}
		if err := EvaluatePhaseGuard(from, to, guard); err != nil {
			return State{}, err
		}
	}
	next, err := appendEvent(state, EventPhaseTransition, phaseTransitionData{
		From: from, To: to, Guard: guard, GuardPassed: true, Reason: normalizeText(reason),
	}, now)
	return next, err
}

// RecordDiagnosticRound persists one `free_state_diagnostic_round.v1` record
// into the closure event stream; restart recovery replays them from Fold.
func (d Driver) RecordDiagnosticRound(state State, expectedRevision uint64, round DiagnosticRoundRecord, now time.Time) (State, error) {
	if err := validateExpectedRevision(state, expectedRevision); err != nil {
		return State{}, err
	}
	if err := round.Validate(); err != nil {
		return State{}, err
	}
	if round.ProjectRevision != "" && state.ProjectRevision != "" && round.ProjectRevision != state.ProjectRevision {
		return State{}, fmt.Errorf("round project_revision %q does not match closure revision %q", round.ProjectRevision, state.ProjectRevision)
	}
	return appendEvent(state, EventDiagnosticRoundRecorded, diagnosticRoundRecordedData{Round: round}, now)
}

func (d Driver) settleUnchecked(state State, reason StopReason, summary string, needsClarification bool, handoff string, now time.Time) (State, error) {
	if state.Terminal() {
		return state, nil
	}
	if state.ActiveCapability != nil {
		return State{}, fmt.Errorf("active capability must settle before the closure")
	}
	if !validStopReason(reason) {
		return State{}, fmt.Errorf("invalid stop reason %q", reason)
	}
	if needsClarification && reason != StopUserChoiceRequired {
		return State{}, fmt.Errorf("only user_choice_required may request clarification")
	}
	settlement := Settlement{Reason: reason, Summary: normalizeText(summary), NeedsUserClarification: needsClarification, HandoffController: normalizeText(handoff), Round: state.RoundsStarted, SettledAt: utcNow(now)}
	return appendEvent(state, EventSettled, settledData{Settlement: settlement}, now)
}

func ObservationFingerprint(key ObservationKey) (string, error) {
	key.ProjectUUID, key.ProjectRevision = normalizeText(key.ProjectUUID), normalizeText(key.ProjectRevision)
	key.Scope.Kind, key.Scope.ID = strings.ToLower(normalizeText(key.Scope.Kind)), normalizeText(key.Scope.ID)
	key.TargetRef, key.ObservationMode, key.Tap, key.TimeWindow = normalizeText(key.TargetRef), strings.ToLower(normalizeText(key.ObservationMode)), strings.ToLower(normalizeText(key.Tap)), normalizeText(key.TimeWindow)
	key.ViewIDs = normalizedStrings(key.ViewIDs)
	if key.ProjectUUID == "" {
		return "", fmt.Errorf("project_uuid is required for an observation fingerprint")
	}
	raw, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "audio_observation:" + hex.EncodeToString(digest[:16]), nil
}

func (d Driver) policyFor(state State) Policy {
	if state.Policy.MaxClosureRounds > 0 {
		return state.Policy
	}
	return normalizePolicy(d.Policy)
}

func normalizePolicy(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.MaxClosureRounds <= 0 {
		policy.MaxClosureRounds = defaults.MaxClosureRounds
	}
	if policy.MaxUniqueObservations <= 0 {
		policy.MaxUniqueObservations = defaults.MaxUniqueObservations
	}
	if policy.MaxNoProgressRounds <= 0 {
		policy.MaxNoProgressRounds = defaults.MaxNoProgressRounds
	}
	if policy.MaxModelProtocolRepairs <= 0 {
		policy.MaxModelProtocolRepairs = defaults.MaxModelProtocolRepairs
	}
	if policy.MaxActionAttempts <= 0 {
		policy.MaxActionAttempts = defaults.MaxActionAttempts
	}
	if policy.MaxRollbackAttempts <= 0 {
		policy.MaxRollbackAttempts = defaults.MaxRollbackAttempts
	}
	return policy
}

func validatePolicy(policy Policy) error {
	if policy.MaxClosureRounds < 1 || policy.MaxUniqueObservations < 1 || policy.MaxNoProgressRounds < 1 || policy.MaxModelProtocolRepairs < 1 || policy.MaxActionAttempts < 1 || policy.MaxRollbackAttempts < 1 {
		return fmt.Errorf("all closure policy limits must be positive")
	}
	return nil
}

func validateExpectedRevision(state State, expected uint64) error {
	if state.SchemaVersion != SchemaVersion || state.ClosureID == "" {
		return fmt.Errorf("closure state is not initialized")
	}
	if state.Revision != expected {
		return fmt.Errorf("closure revision conflict: expected %d, current %d", expected, state.Revision)
	}
	if state.Terminal() {
		return fmt.Errorf("closure %s is settled", state.ClosureID)
	}
	return nil
}

func normalizeFrontier(frontier HypothesisFrontier) HypothesisFrontier {
	frontier.HypothesisIDs, frontier.Blockers = normalizedStrings(frontier.HypothesisIDs), normalizedStrings(frontier.Blockers)
	frontier.CandidateID = normalizeText(frontier.CandidateID)
	frontier.Candidates = normalizedCandidates(frontier.Candidates)
	return frontier
}

func normalizedCandidates(values []Candidate) []Candidate {
	byID := map[string]Candidate{}
	for _, candidate := range values {
		candidate.ID = normalizeText(candidate.ID)
		if candidate.ID == "" {
			continue
		}
		candidate.SourceObservationID = normalizeText(candidate.SourceObservationID)
		candidate.ViewID = normalizeText(candidate.ViewID)
		candidate.IssueType = normalizeText(candidate.IssueType)
		candidate.Region = normalizeText(candidate.Region)
		candidate.TrackIDs = normalizedStrings(candidate.TrackIDs)
		if len(candidate.TrackIDs) == 0 {
			continue
		}
		candidate.TrackNames = normalizedStrings(candidate.TrackNames)
		candidate.EvidenceRefs = normalizedStrings(candidate.EvidenceRefs)
		byID[candidate.ID] = candidate
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Candidate, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out
}

func frontierFingerprint(frontier HypothesisFrontier) string {
	frontier = normalizeFrontier(frontier)
	raw, _ := json.Marshal(frontier)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:8])
}

func normalizedStrings(values []string) []string {
	seen, out := map[string]bool{}, make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeText(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value], out = true, append(out, value)
	}
	sort.Strings(out)
	return out
}

func utcNow(now time.Time) time.Time {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC()
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = normalizeText(value); value != "" {
			return value
		}
	}
	return ""
}

func validStopReason(reason StopReason) bool {
	switch reason {
	case StopSatisfied, StopDiagnosticComplete, StopNoCandidateFound, StopCapabilityBlocked, StopTaskSettled, StopTaskFailed, StopActionablePendingConfirmation, StopInsufficientEvidence, StopEvidenceCeilingReached, StopNoProgress, StopCapabilityUnavailable, StopPCAUnavailable, StopModelProtocolFailure, StopTransportFailure, StopRoundLimit, StopActionFailed, StopVerificationFailedRolledBack, StopUserChoiceRequired, StopProjectRevisionStale, StopHandoffRequested, StopCancelled:
		return true
	default:
		return false
	}
}
