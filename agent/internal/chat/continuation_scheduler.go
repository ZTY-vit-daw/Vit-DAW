package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/history"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

// DurableContinuationStatus is the persisted lifecycle of a continuation
// boundary. A continuation is work owned by the task runtime, not by an HTTP
// request or a goroutine.
type DurableContinuationStatus string

const (
	ContinuationPending            DurableContinuationStatus = "pending"
	ContinuationClaimed            DurableContinuationStatus = "claimed"
	ContinuationRunning            DurableContinuationStatus = "running"
	ContinuationWaitingInteraction DurableContinuationStatus = "waiting_interaction"
	ContinuationCompleted          DurableContinuationStatus = "completed"
	ContinuationCancelled          DurableContinuationStatus = "cancelled"
	ContinuationFailed             DurableContinuationStatus = "failed"
)

type durableContinuationContextKey struct{}

func durableContinuationInvocationID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(durableContinuationContextKey{}).(string)
	return strings.TrimSpace(value)
}

// continuationStallLogInterval throttles the "durable work exists but the
// scheduler cannot take it" report. The condition is stable by nature — an
// unclaimable record stays unclaimable — so an unthrottled line would repeat on
// every 250 ms tick and bury the slice logs it is meant to sit beside.
const continuationStallLogInterval = 5 * time.Second

// logContinuationStall reports, rate limited per reason, that the scheduler saw
// drivable-looking durable work and could not advance it. Before CONT-STALL-1
// every one of these paths returned silently, which is why a 203-second dead
// park left no trace at all.
func (s *Server) logContinuationStall(reason, format string, args ...any) {
	if s == nil || s.logger == nil {
		return
	}
	now := time.Now()
	s.continuationStallLogMu.Lock()
	repeated := reason == s.continuationStallLogReason && now.Sub(s.continuationStallLogAt) < continuationStallLogInterval
	if !repeated {
		s.continuationStallLogReason = reason
		s.continuationStallLogAt = now
	}
	s.continuationStallLogMu.Unlock()
	if repeated {
		return
	}
	s.logger.Warn("[continuation.stall] reason="+reason+" "+format, args...)
}

// continuationDrivableCount reports how many durable records the scheduler
// considers drivable right now (pending, or an expired claimed/running lease).
// Caller must hold s.mu.
func (s *Server) continuationDrivableCountLocked(now time.Time) int {
	count := 0
	for _, item := range s.durableContinuations {
		switch item.Status {
		case ContinuationPending:
			count++
		case ContinuationClaimed, ContinuationRunning:
			if item.LeaseExpiresAt.IsZero() || !now.Before(item.LeaseExpiresAt) {
				count++
			}
		}
	}
	return count
}

// continuationDrivableWorkCount is the lock-taking form of
// continuationDrivableCountLocked for callers outside the server lock.
func (s *Server) continuationDrivableWorkCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.continuationDrivableCountLocked(time.Now().UTC())
}

// continuationNonTerminalCount reports every durable record that still claims
// future work (including answerable parks). Caller must hold s.mu.
func (s *Server) continuationNonTerminalCountLocked() int {
	count := 0
	for _, item := range s.durableContinuations {
		if !continuationTerminalStatus(item.Status) {
			count++
		}
	}
	return count
}

// continuationAnswerablePark reports whether a record is parked at an
// interaction a user can actually answer. The pending-interaction payload the
// answerable parks write carries the interaction identity (pendingInteractionFromResult:
// interaction_id plus the request rows); the payloads the fail-closed guards
// write carry only {status, reason}. That difference is exactly what separates
// "the chain is waiting for the user" — intended, no alarm — from
// "the chain is waiting for nothing" — the CONT-STALL-1 dead park.
func continuationAnswerablePark(item DurableContinuation) bool {
	if item.Status != ContinuationWaitingInteraction {
		return false
	}
	if strings.TrimSpace(firstStringFromMap(item.PendingInteraction, "interaction_id")) != "" {
		return true
	}
	return item.PendingInteraction["requests"] != nil
}

// continuationUnparkedNonTerminalCountLocked counts the non-terminal records
// that are NOT answerable parks: the ones a silent scheduler would strand.
// Caller must hold s.mu.
func (s *Server) continuationUnparkedNonTerminalCountLocked() int {
	count := 0
	for _, item := range s.durableContinuations {
		if continuationTerminalStatus(item.Status) || continuationAnswerablePark(item) {
			continue
		}
		count++
	}
	return count
}

const (
	continuationRuntimeSchema             = "vit_durable_continuation.v1"
	continuationLeaseDuration             = 2 * time.Minute
	continuationSchedulerTick             = 250 * time.Millisecond
	continuationWorkspaceRecoveryInterval = 2 * time.Second
	continuationWorkspaceDiscoveryTimeout = 2 * time.Second
	continuationSchedulerShutdownWait     = 3 * time.Second
)

// DurableContinuation is the durable envelope around agentloop.Continuation.
// The nested continuation remains the exact planner/executor checkpoint while
// the envelope owns scheduling, lease and recovery semantics.
type DurableContinuation struct {
	SchemaVersion       string                       `json:"schema_version"`
	ContinuationID      string                       `json:"continuation_id"`
	TaskID              string                       `json:"task_id"`
	GoalID              string                       `json:"goal_id"`
	ConversationID      string                       `json:"conversation_id"`
	RunID               string                       `json:"run_id"`
	CurrentSliceID      string                       `json:"current_slice_id"`
	CurrentTurnID       string                       `json:"current_turn_id,omitempty"`
	ProjectPath         string                       `json:"project_path,omitempty"`
	ProjectUUID         string                       `json:"project_uuid,omitempty"`
	ProjectSessionID    string                       `json:"project_session_id,omitempty"`
	OriginalIntent      string                       `json:"original_intent"`
	TaskContract        *taskstate.Contract          `json:"task_contract,omitempty"`
	TaskSemanticState   *taskstate.Snapshot          `json:"task_semantic_state,omitempty"`
	Continuation        agentloop.Continuation       `json:"continuation"`
	ProjectRevision     string                       `json:"project_revision,omitempty"`
	ProjectHistory      map[string]any               `json:"project_history,omitempty"`
	CapacityAssessment  *FreeStateCapacityAssessment `json:"capacity_assessment,omitempty"`
	CapabilityEntryPlan *CapabilityEntryPlan         `json:"capability_entry_plan,omitempty"`
	PendingInteraction  map[string]any               `json:"pending_interaction,omitempty"`
	Status              DurableContinuationStatus    `json:"status"`
	Attempt             int                          `json:"attempt"`
	LeaseOwner          string                       `json:"lease_owner,omitempty"`
	LeaseExpiresAt      time.Time                    `json:"lease_expires_at,omitempty"`
	LastError           string                       `json:"last_error,omitempty"`
	CreatedAt           time.Time                    `json:"created_at"`
	UpdatedAt           time.Time                    `json:"updated_at"`
}

func continuationRunnableStatus(status DurableContinuationStatus) bool {
	return status == ContinuationPending || status == ContinuationClaimed || status == ContinuationRunning
}

// continuationTerminalStatus reports the durable lifecycle's settled forms:
// once terminal, a record's lifecycle never reopens, so readers must never
// rewrite one (restart idempotency depends on it).
func continuationTerminalStatus(status DurableContinuationStatus) bool {
	return status == ContinuationCompleted || status == ContinuationCancelled || status == ContinuationFailed
}

func continuationRecoveryValidationRequired(item DurableContinuation) bool {
	return strings.EqualFold(firstStringFromMap(item.PendingInteraction, "status"), "recovery_validation_required")
}

// legacyWaitingContinuePendingInteraction reports the migration's fabricated
// boundary marker: a waiting_interaction park whose only surface is the
// "legacy checkpoint has no authoritative stop reason" note. Nothing can ever
// answer it — the shell exists purely as a pre-C migration shape.
func legacyWaitingContinuePendingInteraction(pending map[string]any) bool {
	return strings.EqualFold(strings.TrimSpace(firstStringFromMap(pending, "status")), "legacy_waiting_continue")
}

// continuationOccupantDrivable reports whether the continuation occupying a
// goal's arm slot can still be driven: it carries a resumable non-terminal
// checkpoint (pending/claimed/running), parks at an interaction a user can
// actually answer, or is an armed internal resume the continue nudge drives
// directly (arm writes no durable record by design, so a nil durable means
// the occupant itself is that armed resume). The CLEAN1 audit invariant is
// "a parked continuation must be drivable or displaceable": every occupant
// this predicate rejects — an unanswerable legacy_waiting_continue shell or
// terminal residue — is by definition displaceable, and nothing else may
// ever be clobbered by a re-arm.
func continuationOccupantDrivable(occupant agentloop.Continuation, durable *DurableContinuation) bool {
	if durable == nil {
		return true
	}
	switch durable.Status {
	case ContinuationCompleted, ContinuationCancelled, ContinuationFailed:
		return false
	case ContinuationWaitingInteraction:
		return !legacyWaitingContinuePendingInteraction(durable.PendingInteraction)
	}
	return true
}

func continuationIDForResult(res agentloop.Result) string {
	seed := strings.Join([]string{
		strings.TrimSpace(res.GoalID), strings.TrimSpace(res.RunID),
		strings.TrimSpace(res.SliceID), strings.TrimSpace(res.TurnID),
	}, "|")
	if seed == "|||" {
		seed = fmt.Sprintf("continuation|%d", time.Now().UnixNano())
	}
	sum := sha256.Sum256([]byte(seed))
	return "cont_" + hex.EncodeToString(sum[:12])
}

func continuationIDForContinuation(cont agentloop.Continuation) string {
	if id := strings.TrimSpace(cont.ContinuationID); id != "" {
		return id
	}
	seed := strings.Join([]string{strings.TrimSpace(cont.GoalID), strings.TrimSpace(cont.RunID), strings.TrimSpace(cont.SliceID), strings.TrimSpace(cont.TurnID)}, "|")
	if seed == "|||" {
		seed = fmt.Sprintf("continuation|%d", time.Now().UnixNano())
	}
	sum := sha256.Sum256([]byte(seed))
	return "cont_" + hex.EncodeToString(sum[:12])
}

func autoContinuationResult(res agentloop.Result) bool {
	return res.Continuation != nil &&
		res.Status == agentruntime.StatusWaitingContinue &&
		!continuationRequiresUserInteraction(res)
}

func continuationRequiresUserInteraction(res agentloop.Result) bool {
	if res.Status == agentruntime.StatusWaitingConfirmation || res.Status == agentruntime.StatusWaitingClarification || res.StopReason == agentloop.StopReasonInterjection {
		return true
	}
	reason := strings.ToLower(strings.TrimSpace(res.StopReason + " " + res.FailureReason + " " + res.Error))
	for _, marker := range []string{
		"confirmation", "clarification", "human_judgment", "user_judgment", "human_audition",
		"permission", "unauthorized", "forbidden", "user_stop", "interjection",
	} {
		if strings.Contains(reason, marker) {
			return true
		}
	}
	decision := res.FreeStateDecision
	if decision == nil && res.Continuation != nil {
		decision = res.Continuation.FreeStateDecision
	}
	if decision != nil {
		if strings.EqualFold(strings.TrimSpace(decision.ExperimentRoundDecision), "user_judgment_pending") {
			return true
		}
		if decision.ExperimentTargetResponse != nil && strings.EqualFold(strings.TrimSpace(string(decision.ExperimentTargetResponse.Outcome)), "human_audition_ready") {
			return true
		}
	}
	return false
}

func pendingInteractionFromResult(res agentloop.Result) map[string]any {
	for _, executed := range res.Executed {
		result := firstMapFromAny(executed["result"])
		if len(result) == 0 {
			result = executed
		}
		requests := result["interaction_requests"]
		if requests == nil {
			continue
		}
		return map[string]any{
			"status": string(res.Status), "stop_reason": res.StopReason,
			"requests": requests, "goal_id": res.GoalID, "run_id": res.RunID,
			"task_id": res.TaskID, "current_slice_id": res.SliceID, "current_turn_id": res.TurnID,
			"continuation_id": continuationIDForResult(res),
			// completePendingInteractionContinuation matches waiting
			// checkpoints by this top-level id; without it an answered
			// confirmation leaves an orphan waiting_interaction checkpoint
			// behind (2026-08-25 D1 smoke).
			"interaction_id": firstInteractionRequestID(requests),
		}
	}
	return nil
}

// firstInteractionRequestID extracts the id of the first interaction request
// from the shapes interaction_requests can take inside an executed result.
func firstInteractionRequestID(requests any) string {
	switch rows := requests.(type) {
	case []AgentInteractionRequest:
		for _, row := range rows {
			if id := strings.TrimSpace(row.ID); id != "" {
				return id
			}
		}
	case []any:
		for _, row := range rows {
			if id := firstStringFromMap(firstMapFromAny(row), "id", "interaction_id"); id != "" {
				return id
			}
		}
	case []map[string]any:
		for _, row := range rows {
			if id := firstStringFromMap(row, "id", "interaction_id"); id != "" {
				return id
			}
		}
	}
	return ""
}

func continuationWaitingForInteraction(res agentloop.Result) bool {
	if res.Continuation == nil {
		return false
	}
	if continuationRequiresUserInteraction(res) {
		return true
	}
	return !autoContinuationResult(res)
}

func durableContinuationFromResult(conversationID string, res agentloop.Result, now time.Time) DurableContinuation {
	cont := *res.Continuation
	continuationID := continuationIDForResult(res)
	cont.ContinuationID = continuationID
	status := ContinuationWaitingInteraction
	if autoContinuationResult(res) {
		status = ContinuationPending
	}
	if res.Status == agentruntime.StatusCancelled || res.Status == agentruntime.StatusStopped {
		status = ContinuationCancelled
	}
	if res.Status == agentruntime.StatusFailed {
		status = ContinuationFailed
	}
	originalIntent := strings.TrimSpace(res.OriginalIntent)
	if originalIntent == "" {
		originalIntent = strings.TrimSpace(cont.OriginalIntent)
	}
	assessment := capacityAssessmentFromAny(cont.Context[capacityAssessmentContextKey])
	entryPlan := capabilityEntryPlanFromAny(cont.Context[capabilityEntryPlanContextKey])
	return DurableContinuation{
		SchemaVersion:  continuationRuntimeSchema,
		ContinuationID: continuationID,
		TaskID:         firstNonEmpty(res.TaskID, cont.TaskID),
		GoalID:         firstNonEmpty(res.GoalID, cont.GoalID),
		ConversationID: firstNonEmpty(conversationID, ""),
		RunID:          firstNonEmpty(res.RunID, cont.RunID),
		CurrentSliceID: firstNonEmpty(res.SliceID, cont.SliceID),
		CurrentTurnID:  firstNonEmpty(res.TurnID, cont.TurnID),
		OriginalIntent: originalIntent,
		Continuation:   cont,
		ProjectRevision: firstNonEmpty(capacityAssessmentRevision(assessment),
			firstStringFromMap(cont.ProjectHistory, "project_revision", "head", "baseline_commit"),
			firstStringFromMap(cont.ContextSnapshot, "project_revision"),
			firstStringFromMap(cont.State, "project_revision"),
		),
		ProjectHistory:      cloneContext(cont.ProjectHistory),
		CapacityAssessment:  assessment,
		CapabilityEntryPlan: entryPlan,
		Status:              status,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func cloneDurableContinuation(in DurableContinuation) DurableContinuation {
	data, err := json.Marshal(in)
	if err != nil {
		return in
	}
	var out DurableContinuation
	if err := json.Unmarshal(data, &out); err != nil {
		return in
	}
	return out
}

func cloneTaskContract(in *taskstate.Contract) *taskstate.Contract {
	if in == nil {
		return nil
	}
	out := *in
	out.CompletionCriteria = append([]string(nil), in.CompletionCriteria...)
	out.EvidenceRequirements = append([]string(nil), in.EvidenceRequirements...)
	return &out
}

func cloneTaskSemanticState(in *taskstate.Snapshot) *taskstate.Snapshot {
	if in == nil {
		return nil
	}
	out := *in
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	out.History = append([]taskstate.TransitionRecord(nil), in.History...)
	if in.Proposal != nil {
		proposal := *in.Proposal
		proposal.EvidenceRefs = append([]string(nil), in.Proposal.EvidenceRefs...)
		proposal.Bounds = append([]string(nil), in.Proposal.Bounds...)
		out.Proposal = &proposal
	}
	if in.PendingInteraction != nil {
		interaction := *in.PendingInteraction
		out.PendingInteraction = &interaction
	}
	return &out
}

func (s *Server) bindDurableTaskSemanticState(item *DurableContinuation) {
	if s == nil || item == nil || s.harness == nil || strings.TrimSpace(item.GoalID) == "" {
		return
	}
	goal := s.harness.RuntimeStatus(item.GoalID)
	if goal.Task == nil {
		return
	}
	if item.TaskID == "" {
		item.TaskID = goal.Task.TaskID
	}
	if goal.Task.Contract != nil {
		item.TaskContract = cloneTaskContract(goal.Task.Contract)
	}
	if goal.Task.SemanticState != nil {
		item.TaskSemanticState = cloneTaskSemanticState(goal.Task.SemanticState)
	}
}

func markContinuationRecoveryValidation(item *DurableContinuation, reason string) {
	if item == nil || !continuationRunnableStatus(item.Status) {
		return
	}
	item.Status = ContinuationWaitingInteraction
	item.LeaseOwner = ""
	item.LeaseExpiresAt = time.Time{}
	item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": reason}
}

// restoredRunBelongsToEarlierGoalRun reports whether a restored continuation
// carries a recognized non-terminal status together with a run identity that
// differs from the goal's current run — residue of the goal's previous run
// that must be quarantined rather than silently re-bound.
func restoredRunBelongsToEarlierGoalRun(item DurableContinuation, goal agentruntime.Goal) bool {
	goalRunID := strings.TrimSpace(goal.RunID)
	itemRunID := strings.TrimSpace(item.RunID)
	if goalRunID == "" || itemRunID == "" || goalRunID == itemRunID {
		return false
	}
	switch item.Status {
	case ContinuationPending, ContinuationClaimed, ContinuationRunning, ContinuationWaitingInteraction:
		return true
	}
	return false
}

// restoredContinuationOwesActiveRound reports whether a restored continuation
// belongs to a free-state loop that is still active while its current round
// owes work — the refused-settle retry marker, or a recalibration round that
// has not acted yet. The goal-level terminal fold must not fire for such a
// checkpoint: the retry turn's completed envelope residue can flip the harness
// goal terminal inside the same race window, and the fold then retires the
// armed checkpoint at its birth stamp with attempt=0, leaving goal_continuations
// empty — every later nudge answers no_continuation and the owed round is
// never proposed again (2026-08-29 192048 trace). Real completions keep
// folding: a terminal loop never owes a round.
func restoredContinuationOwesActiveRound(state projectAgentRuntimeState, item DurableContinuation) bool {
	if strings.TrimSpace(item.ConversationID) == "" {
		return false
	}
	loop, ok := state.FreeStateLoops[item.ConversationID]
	if !ok || !freeStateLoopActive(loop) {
		return false
	}
	return freeStateLoopRoundSettleRefused(loop) || freeStateLoopRoundOwesIntervention(loop)
}

func normalizeRestoredDurableContinuation(mapKey string, item DurableContinuation, state projectAgentRuntimeState, now time.Time) (string, DurableContinuation) {
	item.SchemaVersion = continuationRuntimeSchema
	item.GoalID = firstNonEmpty(item.GoalID, item.Continuation.GoalID)
	item.RunID = firstNonEmpty(item.RunID, item.Continuation.RunID)
	item.TaskID = firstNonEmpty(item.TaskID, item.Continuation.TaskID)
	item.CurrentSliceID = firstNonEmpty(item.CurrentSliceID, item.Continuation.SliceID)
	item.CurrentTurnID = firstNonEmpty(item.CurrentTurnID, item.Continuation.TurnID)
	for _, goal := range state.GoalRuntime.Goals {
		if goal.GoalID != item.GoalID {
			continue
		}
		item.GoalID = goal.GoalID
		if restoredRunBelongsToEarlierGoalRun(item, goal) {
			// Read-side run tenure (2026-08-30 CLEAN1 F8/L2): a record with a
			// recognized non-terminal status vouches for the run it actually
			// ran in. Rebranding it as the goal's current run would let the
			// previous run's residue — an unanswered confirmation park, a
			// parked checkpoint — masquerade as current-run state with no
			// TaskContract to catch the mismatch. Quarantine it instead,
			// keeping the true run identity so the completion bridge's
			// run matching stays sound; terminal records are never rewritten
			// and unrecognized-status records keep the task-snapshot identity
			// repair below.
			markContinuationRecoveryValidation(&item, "restored continuation belongs to an earlier run")
		} else {
			item.RunID = firstNonEmpty(goal.RunID, item.RunID)
		}
		if goal.Task != nil {
			item.TaskID = firstNonEmpty(goal.Task.TaskID, item.TaskID)
			item.OriginalIntent = firstNonEmpty(goal.Task.OriginalIntent, item.OriginalIntent, item.Continuation.OriginalIntent)
			if item.TaskContract == nil && goal.Task.Contract != nil {
				item.TaskContract = cloneTaskContract(goal.Task.Contract)
			}
			if item.TaskSemanticState == nil && goal.Task.SemanticState != nil {
				item.TaskSemanticState = cloneTaskSemanticState(goal.Task.SemanticState)
			}
			if item.TaskContract != nil && goal.Task.Contract != nil && item.TaskContract.ContractID != goal.Task.Contract.ContractID {
				markContinuationRecoveryValidation(&item, "continuation contract does not match restored task contract")
			}
			if item.TaskSemanticState != nil && goal.Task.SemanticState != nil && item.TaskSemanticState.ContractID != goal.Task.SemanticState.ContractID {
				markContinuationRecoveryValidation(&item, "continuation semantic state does not match restored task state")
			}
		}
		switch goal.Status {
		case agentruntime.StatusCompleted, agentruntime.StatusStable:
			if !restoredContinuationOwesActiveRound(state, item) {
				item.Status = ContinuationCompleted
			}
		case agentruntime.StatusCancelled, agentruntime.StatusStopped:
			item.Status = ContinuationCancelled
		case agentruntime.StatusFailed:
			item.Status = ContinuationFailed
			item.LastError = firstNonEmpty(item.LastError, goal.Error)
		}
		if item.Status == ContinuationCompleted || item.Status == ContinuationCancelled || item.Status == ContinuationFailed {
			item.LeaseOwner = ""
			item.LeaseExpiresAt = time.Time{}
		}
		break
	}
	if item.TaskContract != nil || item.TaskSemanticState != nil {
		if item.TaskContract == nil || item.TaskSemanticState == nil {
			markContinuationRecoveryValidation(&item, "task semantic contract/state is incomplete")
		} else if item.TaskContract.TaskID != item.TaskID || item.TaskContract.GoalID != item.GoalID || item.TaskContract.RunID != item.RunID || item.TaskContract.OriginalIntent != item.OriginalIntent {
			markContinuationRecoveryValidation(&item, "continuation semantic identity does not match task/run/intent")
		} else if err := item.TaskSemanticState.Validate(*item.TaskContract); err != nil {
			markContinuationRecoveryValidation(&item, "continuation semantic state validation failed: "+err.Error())
		}
	}
	item.OriginalIntent = firstNonEmpty(item.OriginalIntent, item.Continuation.OriginalIntent, item.Continuation.Summary, item.Continuation.UserText)
	item.Continuation.GoalID = item.GoalID
	item.Continuation.RunID = item.RunID
	item.Continuation.TaskID = item.TaskID
	item.Continuation.OriginalIntent = item.OriginalIntent
	item.Continuation.SliceID = firstNonEmpty(item.Continuation.SliceID, item.CurrentSliceID)
	item.Continuation.TurnID = firstNonEmpty(item.Continuation.TurnID, item.CurrentTurnID)
	item.ProjectPath = firstNonEmpty(item.ProjectPath, state.ProjectPath)
	item.ProjectUUID = firstNonEmpty(item.ProjectUUID, state.ProjectUUID)
	if item.ProjectRevision == "" {
		item.ProjectRevision = firstNonEmpty(firstStringFromMap(item.ProjectHistory, "project_revision", "head", "baseline_commit"), firstStringFromMap(item.Continuation.ContextSnapshot, "project_revision"))
	}
	if item.CapacityAssessment == nil {
		item.CapacityAssessment = capacityAssessmentFromAny(item.Continuation.Context[capacityAssessmentContextKey])
	}
	if item.CapabilityEntryPlan == nil {
		item.CapabilityEntryPlan = capabilityEntryPlanFromAny(item.Continuation.Context[capabilityEntryPlanContextKey])
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = item.CreatedAt
	}
	switch item.Status {
	case ContinuationPending, ContinuationClaimed, ContinuationRunning, ContinuationWaitingInteraction, ContinuationCompleted, ContinuationCancelled, ContinuationFailed:
	default:
		item.Status = ContinuationWaitingInteraction
		item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "unknown durable continuation status"}
	}
	if item.ConversationID == "" {
		for conversationID, goalID := range state.ConversationGoals {
			if goalID == item.GoalID {
				item.ConversationID = conversationID
				break
			}
		}
	}
	if item.TaskContract != nil && item.ConversationID != "" && item.TaskContract.ConversationID != item.ConversationID {
		markContinuationRecoveryValidation(&item, "continuation conversation does not match task contract")
	}
	if item.ConversationID == "" && continuationRunnableStatus(item.Status) {
		item.Status = ContinuationWaitingInteraction
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "conversation identity is missing"}
	}
	item.ContinuationID = firstNonEmpty(item.ContinuationID, strings.TrimSpace(mapKey))
	if item.ContinuationID == "" {
		item.ContinuationID = continuationIDForContinuation(item.Continuation)
	}
	item.Continuation.ContinuationID = item.ContinuationID
	if item.ProjectUUID != "" && state.ProjectUUID != "" && item.ProjectUUID != state.ProjectUUID && continuationRunnableStatus(item.Status) {
		item.Status = ContinuationWaitingInteraction
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "continuation project identity does not match runtime snapshot"}
	}
	if expectedGoal := strings.TrimSpace(state.ConversationGoals[item.ConversationID]); item.ConversationID != "" && expectedGoal != "" && expectedGoal != item.GoalID && continuationRunnableStatus(item.Status) {
		item.Status = ContinuationWaitingInteraction
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "conversation is bound to a different goal"}
	}
	if continuationRunnableStatus(item.Status) && (item.TaskID == "" || item.GoalID == "" || item.RunID == "" || item.CurrentSliceID == "" || item.OriginalIntent == "") {
		item.Status = ContinuationWaitingInteraction
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.PendingInteraction = map[string]any{"status": "recovery_validation_required", "reason": "durable task/run/slice identity is incomplete"}
	}
	return item.ContinuationID, cloneDurableContinuation(item)
}

// A Task/Run is linear: only its newest nonterminal checkpoint may own the
// next invocation. This also repairs snapshots written by pre-C builds in the
// small crash window where a child checkpoint existed beside its running
// parent.
func reconcileRestoredContinuations(items map[string]DurableContinuation) map[string]DurableContinuation {
	newestByRun := map[string]string{}
	for id, item := range items {
		if item.Status == ContinuationCompleted || item.Status == ContinuationCancelled || item.Status == ContinuationFailed {
			continue
		}
		if item.TaskID == "" || item.GoalID == "" || item.RunID == "" {
			continue
		}
		key := strings.Join([]string{item.TaskID, item.GoalID, item.RunID}, "|")
		currentID, exists := newestByRun[key]
		if !exists {
			newestByRun[key] = id
			continue
		}
		current := items[currentID]
		if item.UpdatedAt.After(current.UpdatedAt) || (item.UpdatedAt.Equal(current.UpdatedAt) && item.CreatedAt.After(current.CreatedAt)) ||
			(item.UpdatedAt.Equal(current.UpdatedAt) && item.CreatedAt.Equal(current.CreatedAt) && id > currentID) {
			newestByRun[key] = id
		}
	}
	now := time.Now().UTC()
	for id, item := range items {
		if item.Status == ContinuationCompleted || item.Status == ContinuationCancelled || item.Status == ContinuationFailed {
			continue
		}
		if item.TaskID == "" || item.GoalID == "" || item.RunID == "" {
			continue
		}
		key := strings.Join([]string{item.TaskID, item.GoalID, item.RunID}, "|")
		if newestByRun[key] == id {
			continue
		}
		item.Status = ContinuationCompleted
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = now
		items[id] = cloneDurableContinuation(item)
	}
	return items
}

func (s *Server) wakeContinuationScheduler() {
	if s == nil || s.schedulerWake == nil {
		return
	}
	s.mu.Lock()
	activeWorkspaceUUID := s.activeWorkspaceUUID
	executorConfigured := s.continuationExecutor != nil
	hasScheduledWork := false
	for _, item := range s.durableContinuations {
		if item.Status == ContinuationPending ||
			item.Status == ContinuationClaimed || item.Status == ContinuationRunning {
			hasScheduledWork = true
			break
		}
	}
	s.mu.Unlock()
	if !hasScheduledWork || (!executorConfigured && strings.TrimSpace(activeWorkspaceUUID) == "") {
		if s.logger != nil {
			s.logger.Info("[continuation.wake] declined has_scheduled_work=%t executor=%t workspace=%s",
				hasScheduledWork, executorConfigured, firstNonEmpty(activeWorkspaceUUID, "<none>"))
		}
		return
	}
	if s.logger != nil {
		s.logger.Info("[continuation.wake] has_scheduled_work=true executor=%t workspace=%s",
			executorConfigured, firstNonEmpty(activeWorkspaceUUID, "<none>"))
	}
	s.startContinuationScheduler()
	select {
	case s.schedulerWake <- struct{}{}:
	default:
	}
}

// Start begins the process-owned recovery worker. It is separate from New so
// tests and embedded callers can construct a Server without leaking a worker;
// the product host calls it once during process startup.
func (s *Server) Start() {
	if s == nil {
		return
	}
	s.startContinuationScheduler()
	s.wakeContinuationScheduler()
}

func (s *Server) startContinuationScheduler() {
	if s == nil || s.schedulerWake == nil || s.schedulerCancel == nil {
		return
	}
	s.schedulerOnce.Do(func() {
		s.mu.Lock()
		if s.schedulerDone == nil {
			s.schedulerDone = make(chan struct{})
		}
		done := s.schedulerDone
		s.schedulerStarted = true
		s.mu.Unlock()
		go func() {
			defer close(done)
			ticker := time.NewTicker(continuationSchedulerTick)
			defer ticker.Stop()
			drive := func() {
				s.recoverContinuationWorkspace(s.schedulerCtx)
				_ = s.runContinuationSchedulerOnce(s.schedulerCtx)
			}
			for {
				select {
				case <-s.schedulerCtx.Done():
					return
				case <-ticker.C:
					drive()
				case <-s.schedulerWake:
					drive()
				}
			}
		}()
	})
}

func (s *Server) recoverContinuationWorkspace(ctx context.Context) {
	if s == nil || s.harness == nil {
		return
	}
	// Workspace recovery activates (and therefore restores) a project state
	// snapshot. It must never switch or overwrite runtime state while a
	// request is mid-flight; the request handlers own activation for their
	// own duration.
	if s.invocationsActive() {
		return
	}
	projectPath, projectUUID := s.harness.CurrentProjectIdentity(ctx)
	s.mu.Lock()
	activePath, activeUUID := s.activeWorkspacePath, s.activeWorkspaceUUID
	now := time.Now().UTC()
	mayDiscover := projectUUID == "" && (s.lastWorkspaceRecoveryAttempt.IsZero() || now.Sub(s.lastWorkspaceRecoveryAttempt) >= continuationWorkspaceRecoveryInterval)
	if mayDiscover {
		s.lastWorkspaceRecoveryAttempt = now
	}
	s.mu.Unlock()
	if projectUUID == "" && mayDiscover && s.kernel != nil && s.shadow != nil {
		discoveryCtx, cancel := context.WithTimeout(ctx, continuationWorkspaceDiscoveryTimeout)
		reply, _, err := s.kernel.SendCommand(discoveryCtx, map[string]any{"cmd": "get_project_state"})
		cancel()
		if err == nil && strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "ok") {
			s.shadow.Initialize(reply)
			projectPath, projectUUID = s.harness.CurrentProjectIdentity(ctx)
		}
	}
	if projectUUID == "" || (projectUUID == activeUUID && sameWorkspacePath(projectPath, activePath)) {
		return
	}
	s.activateCurrentProjectWorkspace(ctx)
}

// Close stops the scheduler. It is intentionally optional for existing host
// integrations; project state remains durable even when the process exits.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	started, done := s.schedulerStarted, s.schedulerDone
	s.mu.Unlock()
	if s.schedulerCancel != nil {
		s.schedulerCancel()
	}
	if started && done != nil {
		select {
		case <-done:
		case <-time.After(continuationSchedulerShutdownWait):
			return fmt.Errorf("continuation scheduler shutdown timed out")
		}
	}
	return nil
}

func (s *Server) persistContinuationState() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	persist := s.continuationPersist
	s.mu.Unlock()
	if persist != nil {
		return persist()
	}
	return s.persistCurrentProjectWorkspaceChecked()
}

func (s *Server) beginContinuationSensitiveInvocation() func() {
	if s == nil {
		return func() {}
	}
	s.mu.Lock()
	s.activeRuntimeInvocations++
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		if s.activeRuntimeInvocations > 0 {
			s.activeRuntimeInvocations--
		}
		s.mu.Unlock()
		s.wakeContinuationScheduler()
	}
}

// invocationsActive reports whether a request-boundary invocation (chat,
// interaction, confirmation, audition) currently owns the in-memory runtime
// state. Scheduler-driven disk reloads must not run while it is true.
func (s *Server) invocationsActive() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeRuntimeInvocations > 0
}

func (s *Server) releaseContinuationClaim(id string, err error) {
	s.mu.Lock()
	item, ok := s.durableContinuations[id]
	if ok && (item.Status == ContinuationClaimed || item.Status == ContinuationRunning) {
		item.Status = ContinuationPending
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = time.Now().UTC()
		if err != nil {
			item.LastError = err.Error()
		}
		s.durableContinuations[id] = cloneDurableContinuation(item)
	}
	s.mu.Unlock()
}

func (s *Server) claimNextContinuation(now time.Time) (DurableContinuation, bool) {
	if s == nil {
		return DurableContinuation{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeRuntimeInvocations > 0 {
		return DurableContinuation{}, false
	}
	ids := make([]string, 0, len(s.durableContinuations))
	for id := range s.durableContinuations {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := s.durableContinuations[ids[i]], s.durableContinuations[ids[j]]
		if left.CreatedAt.Equal(right.CreatedAt) {
			return ids[i] < ids[j]
		}
		return left.CreatedAt.Before(right.CreatedAt)
	})
	skippedStatus, skippedLease, skippedProject := 0, 0, 0
	skipDetail := ""
	for _, id := range ids {
		item := s.durableContinuations[id]
		if item.Status != ContinuationPending &&
			!(item.Status == ContinuationClaimed || item.Status == ContinuationRunning) {
			skippedStatus++
			if skipDetail == "" {
				// Name the record the scheduler could not take. A record that
				// armed as pending and reads as waiting_interaction on the next
				// tick is the CONT-STALL-1 shape, and the quarantine reason
				// recorded by the restore pass says which guard did it.
				skipDetail = fmt.Sprintf("id=%s status=%s goal=%s conversation=%s project=%s pending_status=%s pending_reason=%s",
					item.ContinuationID, item.Status, item.GoalID, item.ConversationID, item.ProjectUUID,
					firstStringFromMap(item.PendingInteraction, "status"), firstStringFromMap(item.PendingInteraction, "reason"))
			}
			continue
		}
		if (item.Status == ContinuationClaimed || item.Status == ContinuationRunning) &&
			!item.LeaseExpiresAt.IsZero() && now.Before(item.LeaseExpiresAt) {
			skippedLease++
			continue
		}
		if item.ProjectUUID != "" && s.activeWorkspaceUUID != "" && item.ProjectUUID != s.activeWorkspaceUUID {
			skippedProject++
			continue
		}
		item.Status = ContinuationClaimed
		item.Attempt++
		if strings.TrimSpace(s.schedulerOwner) == "" {
			s.schedulerOwner = "scheduler_" + randomID()
		}
		leaseDuration := s.continuationLease
		if leaseDuration <= 0 {
			leaseDuration = continuationLeaseDuration
		}
		item.LeaseOwner = s.schedulerOwner
		item.LeaseExpiresAt = now.Add(leaseDuration)
		item.LastError = ""
		item.UpdatedAt = now
		s.durableContinuations[id] = cloneDurableContinuation(item)
		if s.logger != nil {
			s.logger.Info("[continuation.claim] claimed id=%s goal=%s conversation=%s attempt=%d lease_until=%s",
				item.ContinuationID, item.GoalID, item.ConversationID, item.Attempt, item.LeaseExpiresAt.UTC().Format(time.RFC3339))
		}
		return cloneDurableContinuation(item), true
	}
	if nonTerminal := s.continuationNonTerminalCountLocked(); nonTerminal > 0 {
		// Nothing claimable although records still claim future work. An
		// answerable park is the intended "waiting for the user" form and must
		// not raise the alarm; anything else is a record the scheduler can
		// never take, which is the CONT-STALL-1 dead park and must be loud.
		if stranded := s.continuationUnparkedNonTerminalCountLocked(); stranded > 0 {
			s.logContinuationStall("no-claimable-record",
				"non_terminal=%d stranded=%d skipped_status=%d skipped_live_lease=%d skipped_project=%d workspace=%s first_skip=[%s]",
				nonTerminal, stranded, skippedStatus, skippedLease, skippedProject, firstNonEmpty(s.activeWorkspaceUUID, "<none>"), skipDetail)
		} else if s.logger != nil {
			s.logger.Info("[continuation.park] answerable park holds the chain non_terminal=%d skipped_status=%d workspace=%s",
				nonTerminal, skippedStatus, firstNonEmpty(s.activeWorkspaceUUID, "<none>"))
		}
	}
	return DurableContinuation{}, false
}

func (s *Server) setContinuationStatus(id string, status DurableContinuationStatus, err error) {
	if s == nil || strings.TrimSpace(id) == "" {
		return
	}
	s.mu.Lock()
	item, ok := s.durableContinuations[id]
	goalID := ""
	if ok {
		goalID = item.GoalID
		item.Status = status
		item.LeaseOwner = ""
		item.LeaseExpiresAt = time.Time{}
		item.UpdatedAt = time.Now().UTC()
		if err != nil {
			item.LastError = err.Error()
		}
		s.durableContinuations[id] = cloneDurableContinuation(item)
		if status == ContinuationFailed || status == ContinuationCancelled {
			delete(s.goalContinuations, item.GoalID)
			for otherID, other := range s.durableContinuations {
				if otherID == id || other.GoalID != item.GoalID || other.Status == ContinuationCompleted || other.Status == ContinuationCancelled || other.Status == ContinuationFailed {
					continue
				}
				other.Status = status
				other.LeaseOwner = ""
				other.LeaseExpiresAt = time.Time{}
				other.UpdatedAt = item.UpdatedAt
				if err != nil {
					other.LastError = err.Error()
				}
				s.durableContinuations[otherID] = cloneDurableContinuation(other)
			}
		}
	}
	s.mu.Unlock()
	if ok {
		if s.harness != nil && goalID != "" {
			switch status {
			case ContinuationFailed:
				s.harness.SetGoalStatus(goalID, agentruntime.StatusFailed, err)
			case ContinuationCancelled:
				s.harness.SetGoalStatus(goalID, agentruntime.StatusCancelled, nil)
			}
		}
		_ = s.persistContinuationState()
	}
}

func (s *Server) runContinuationSchedulerOnce(ctx context.Context) error {
	if s == nil {
		return nil
	}
	// A request in flight owns the authoritative in-memory runtime state: its
	// admission may have stored a capability route or checkpointed a
	// continuation that is not on disk yet. Reloading the durable snapshot
	// now would replace that memory wholesale and the lost route later fails
	// the continuation closed as unrecoverable. Wait for the request; its
	// completion persists and wakes the scheduler again.
	if s.invocationsActive() {
		if drivable := s.continuationDrivableWorkCount(); drivable > 0 {
			s.logContinuationStall("invocations-active",
				"drivable=%d: a request-boundary invocation still owns the in-memory runtime state", drivable)
		}
		return nil
	}
	s.schedulerExecutionMu.Lock()
	defer s.schedulerExecutionMu.Unlock()
	stateLock, err := s.acquireRuntimeStateLease()
	if err != nil {
		if errors.Is(err, history.ErrAgentRuntimeStateLocked) {
			if drivable := s.continuationDrivableWorkCount(); drivable > 0 {
				s.logContinuationStall("runtime-state-lease-held",
					"drivable=%d: another process owns the project runtime state lease", drivable)
			}
			return nil
		}
		return err
	}
	if err := s.reloadActiveRuntimeState(); err != nil {
		if stateLock != nil {
			_ = stateLock.Release()
		}
		s.logContinuationStall("runtime-state-reload-failed", "err=%v", err)
		return err
	}
	item, ok := s.claimNextContinuation(time.Now().UTC())
	if !ok {
		if stateLock != nil {
			_ = stateLock.Release()
		}
		return nil
	}
	if err := s.persistContinuationStateUnlocked(); err != nil {
		s.releaseContinuationClaim(item.ContinuationID, err)
		if stateLock != nil {
			_ = stateLock.Release()
		}
		return fmt.Errorf("persist continuation claim: %w", err)
	}
	s.mu.Lock()
	if current, exists := s.durableContinuations[item.ContinuationID]; exists && current.Status == ContinuationClaimed {
		current.Status = ContinuationRunning
		current.UpdatedAt = time.Now().UTC()
		s.durableContinuations[item.ContinuationID] = cloneDurableContinuation(current)
	}
	s.mu.Unlock()
	if err := s.persistContinuationStateUnlocked(); err != nil {
		s.releaseContinuationClaim(item.ContinuationID, err)
		if stateLock != nil {
			_ = stateLock.Release()
		}
		return fmt.Errorf("persist running continuation: %w", err)
	}
	// The claimed/running lease is now durable. Release the cross-process lock
	// before invoking the planner; the completion checkpoint takes a fresh lock.
	if stateLock != nil {
		if err := stateLock.Release(); err != nil {
			return err
		}
		stateLock = nil
	}

	var executionErr error
	var chainResp ChatResponse
	s.mu.Lock()
	executor := s.continuationExecutor
	s.mu.Unlock()
	if executor != nil {
		executionErr = executor(ctx, item)
	} else {
		chainResp, executionErr = s.executeDurableContinuationWithResult(ctx, item)
	}
	if executionErr != nil {
		if ctx != nil && ctx.Err() != nil && (errors.Is(executionErr, context.Canceled) || errors.Is(executionErr, context.DeadlineExceeded)) {
			s.releaseContinuationClaim(item.ContinuationID, executionErr)
			if persistErr := s.persistContinuationState(); persistErr != nil {
				return fmt.Errorf("release cancelled continuation: execution=%v persistence=%w", executionErr, persistErr)
			}
			return executionErr
		}
		s.setContinuationStatus(item.ContinuationID, ContinuationFailed, executionErr)
		// AGENT-F5: a failed chain is a terminal too. This branch used to return
		// before the delivery gate, so a dying slice produced zero turn.failed
		// deliveries and zero conversation-graph nodes — the failure-path twin of
		// the B1 "server holds a terminal, the user gets nothing" defect F2 fixed
		// for the success path.
		s.deliverSchedulerChainFailure(ctx, item.ContinuationID, chainResp, executionErr)
		return executionErr
	}
	// recordGoalResult creates the next pending record, or a waiting interaction
	// record, before this current lease is released.
	s.mu.Lock()
	current, exists := s.durableContinuations[item.ContinuationID]
	if exists && (current.Status == ContinuationRunning || current.Status == ContinuationClaimed) {
		current.Status = ContinuationCompleted
		current.LeaseOwner = ""
		current.LeaseExpiresAt = time.Time{}
		current.UpdatedAt = time.Now().UTC()
		s.durableContinuations[item.ContinuationID] = cloneDurableContinuation(current)
		if legacy, legacyOK := s.goalContinuations[current.GoalID]; legacyOK && continuationIDForContinuation(legacy) == item.ContinuationID {
			delete(s.goalContinuations, current.GoalID)
		}
	}
	s.mu.Unlock()
	// AGENT-F3：续跑完成边界收口。HTTP 回合边界有 F2 的 turn 守卫，调度侧
	// 完成边界此前无人守——链条停止且任务非终态时，孤儿观察任务与
	// waiting/running 的 goal 在此诚实收敛（内部自查活续跑持有者，有后续
	// 切片或待答交互则不动）。
	if exists {
		chainEnded := !s.goalHasLiveContinuationOwner(current.GoalID)
		s.settleAndDeliverContinuationChainEnd(ctx, current, chainResp, chainEnded)
	}
	if err := s.persistContinuationState(); err != nil {
		return fmt.Errorf("persist completed continuation: %w", err)
	}
	return nil
}

// settleAndDeliverContinuationChainEnd closes a scheduler slice's chain-end
// bookkeeping: the F3 goal/task settle followed by the chain-terminal
// delivery gate. The gate judges on the runtime goal status and the durable
// record's lifecycle form. Two deliverable end shapes exist:
//
//   - AGENT-F6 terminal settle: the slice checkpoint reached a terminal
//     lifecycle (completed/cancelled/failed — the settle-time displaced-cancel
//     transient is repaired to completed afterwards, so completed alone would
//     misjudge) and the goal landed on a settled form.
//   - B1-F2 waiting park: the chain parked at an answerable interaction
//     (goal waiting_confirmation/waiting_clarification). The park is a
//     genuine user-facing wait whose final reply is the chain's last word
//     ("等待试听确认") — B1-DIAG Q1: the terminal-only vocabulary silently
//     dropped it, leaving every user-facing surface empty while the loop
//     waited on the judgment POST. Two record forms reach it: the
//     interaction-boundary park (waiting_interaction record) and the audition
//     judgment park whose slice acked stop=done (record completed) while the
//     loop parked the goal inside the slice (20260910_212325).
//
//   - PARK-1 judgment park: the D1 experiment chain parked at the durable
//     human-judgment boundary (round decision user_judgment_pending, judgment
//     requested without the judgment landing) while the runtime goal stayed
//     waiting_continue. The goal status never became an answerable waiting
//     form — the judgment is answered out-of-band by the audition judgment
//     POST, not by an in-chat interaction — so the two shapes above both
//     missed it and the park turn delivered nothing: the A/B card appeared
//     with zero text about what had been applied or why a judgment was asked
//     for (goal_44cda25a, 17:55:07/17:55:08). The residency predicate is the
//     same freeStateJudgmentBoundary the park itself was established with, so
//     the task state machine and the text delivery cannot disagree about what
//     counts as a park.
//
// Mid-chain slices, unanswerable legacy shells (goal still waiting_continue
// without a judgment boundary), and the test executor (no real resp) deliver
// nothing.
func (s *Server) settleAndDeliverContinuationChainEnd(ctx context.Context, current DurableContinuation, chainResp ChatResponse, chainEnded bool) {
	if s == nil {
		return
	}
	s.settleGoalAfterContinuationEnd(current.ConversationID, current.GoalID)
	// 链终局可观测性（AGENT-F6）：终片后各门条件实值，供审计对账。
	if s.logger != nil {
		runtimeGoalStatus := ""
		if s.harness != nil {
			runtimeGoalStatus = string(s.harness.RuntimeStatus(current.GoalID).Status)
		}
		s.logger.Info("[f6.gate] conversation=%s goal=%s chainEnded=%t status=%s respGoal=%s runtimeGoal=%s",
			current.ConversationID, current.GoalID, chainEnded, current.Status, chainResp.GoalStatus, runtimeGoalStatus)
	}
	if !chainEnded || current.ConversationID == "" ||
		(chainResp.ConversationID == "" && chainResp.GoalID == "") || s.harness == nil {
		return
	}
	goal := s.harness.RuntimeStatus(current.GoalID)
	if goal.GoalID == "" {
		return
	}
	switch {
	case continuationTerminalStatus(current.Status) &&
		(goal.Status == agentruntime.StatusCompleted || goal.Status == agentruntime.StatusFailed ||
			goal.Status == agentruntime.StatusCancelled || goal.Status == agentruntime.StatusStopped ||
			goal.Status == agentruntime.StatusStable):
		s.normalizeChainResponseGoalStatus(chainResp, goal.Status)
		// B6 缺陷①②：完成态终局的用户面文本过护栏——内部代号（FAM/FS/D
		// 阶段、投影缩写）剥除 +「人工判定仍待完成」类空头承诺剥除（判定
		// 入口只由结算评估的 user_judgment_pending 建立，F1 语义）。
		chainResp.Reply = s.sanitizeSettledChainReply(current, chainResp.Reply)
		s.deliverSchedulerChainTerminal(ctx, current, chainResp)
	case (goal.Status == agentruntime.StatusWaitingConfirmation || goal.Status == agentruntime.StatusWaitingClarification) &&
		(continuationTerminalStatus(current.Status) || current.Status == ContinuationWaitingInteraction):
		// Answerable park. Two record forms reach it: the interaction-boundary
		// park (waiting_interaction record) and — 20260910_212325 real-stack —
		// the audition judgment park whose final slice acked stop=done (record
		// completed) while the loop parked the goal at the judgment boundary
		// inside the slice; keying the branch on the record form alone missed
		// it and the waiting reply never reached any surface. The runtime goal
		// status is the authority in both forms.
		chainResp.GoalStatus = string(goal.Status)
		if strings.TrimSpace(chainResp.Reply) == "" {
			// The park slice's own reply is frequently empty; the terminal
			// report ("实验调整已应用并完成观测评估，等待用户试听确认")
			// lives on the loop's latest decision (B1-DIAG Q1 forensics).
			chainResp.Reply = s.schedulerChainFallbackReply(current.ConversationID)
		}
		// B6 缺陷①：可应答 park 的终局同样不得泄漏内部代号；但其「等待试
		// 听确认」是真实可服务的判定入口，判定承诺剥除不适用。
		chainResp.Reply = s.sanitizeWaitingParkChainReply(current, chainResp.Reply)
		s.deliverSchedulerChainTerminal(ctx, current, chainResp)
	case goal.Status == agentruntime.StatusWaitingContinue && s.judgmentBoundaryParkFor(current.ConversationID) &&
		(continuationTerminalStatus(current.Status) || current.Status == ContinuationWaitingInteraction):
		// PARK-1：判定驻留（goal 停在 waiting_continue）的 settle 摘要投递。
		// park 切片的自身回复是调度链过场话术（"我还在继续处理这个任务"），
		// 直接照发等于没投；改由 chat 侧判定驻留 composer 生成五要素精神
		// 的摘要（应用了什么+回读 / 针对的发现 / 请 A/B 试听判定），失败时
		// 逐级退回过场回复与循环决策摘要，绝不静默丢终局。
		chainResp.GoalStatus = string(goal.Status)
		chainResp.Reply = s.judgmentParkChainReply(current, chainResp.Reply)
		if strings.TrimSpace(chainResp.Reply) == "" {
			return
		}
		chainResp.Reply = s.sanitizeWaitingParkChainReply(current, chainResp.Reply)
		s.deliverSchedulerChainTerminal(ctx, current, chainResp)
	case goal.Status == agentruntime.StatusWaitingContinue && s.freeStateLoopOwesExperimentOutcomeFor(current.ConversationID, current.GoalID) &&
		(continuationTerminalStatus(current.Status) || current.Status == ContinuationWaitingInteraction):
		// D1-STALL-1：预算/调度停摆而评估未完成的驻留投递。链条不再有下一片
		// （预算耗尽或最后一拍停在评估步），但已准入回合仍欠它契约要求的受治
		// 结果：应用后的方向性评估与随后的判定边界都还没发生。此前这一形态落
		// 入静默分支，用户只看到 goal 被置完成、却没有 A/B 卡、没有终局汇报，
		// 而 task 仍停在 needs_experiment（2026-09-12 18:52 真栈）。此处给出
		// 显式人话回执：应用了什么+回读 / 针对的发现 / 评估未完成（这不是
		// 完成）/ 说一句「继续」即可续跑。判据刻意只读回合欠账，不读循环的
		// 呈现状态，与 settleGoalAfterContinuationEnd 和预算停止共用同一个
		// freeStateLoopOwesExperimentOutcome，故两处不可能对「任务是否做完」
		// 有分歧。判定驻留（PARK-1 分支）在本分支之前匹配，两者互斥。
		chainResp.GoalStatus = string(goal.Status)
		chainResp.Reply = s.owedEvaluationChainReply(current, chainResp.Reply)
		if strings.TrimSpace(chainResp.Reply) == "" {
			return
		}
		chainResp.Reply = s.sanitizeWaitingParkChainReply(current, chainResp.Reply)
		s.deliverSchedulerChainTerminal(ctx, current, chainResp)
	}
}

// judgmentBoundaryParkFor reports whether the conversation's durable free-state
// loop is parked at the human-judgment boundary. It is the delivery gate's sole
// residency authority and delegates to freeStateJudgmentBoundary — the very
// predicate recordGoalResult uses to retire the driving continuation at the
// boundary — so "the loop is parked" has exactly one definition in the package.
// A conversation with no loop (legacy shell, non-free-state chain) is not a
// judgment park and keeps the silent branch.
func (s *Server) judgmentBoundaryParkFor(conversationID string) bool {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return false
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return false
	}
	return freeStateJudgmentBoundary(loop)
}

// judgmentParkChainReply resolves the park summary body with the same
// escalation order the waiting park uses: the chat-side judgment-park composer
// (five-element spirit) first, then the park slice's own reply, then the loop's
// latest decision summary (B1-DIAG Q1). The composer is preferred because the
// park slice is a scheduler slice whose reply is the chain's pass-through line.
func (s *Server) judgmentParkChainReply(current DurableContinuation, sliceReply string) string {
	if loop, ok := s.freeStateLoop(current.ConversationID); ok {
		if composed := strings.TrimSpace(d1JudgmentParkReply(loop)); composed != "" {
			return composed
		}
	}
	if reply := strings.TrimSpace(sliceReply); reply != "" {
		return reply
	}
	return s.schedulerChainFallbackReply(current.ConversationID)
}

// owedEvaluationChainReply resolves the receipt body for a chain that stopped
// while the applied round still owed its evaluation, with the same escalation
// order the other parks use: the chat-side owed-evaluation composer first,
// then the stopped slice's own reply, then the loop's latest decision summary.
// Never returns empty while the loop is present, so the terminal is not
// silently dropped.
func (s *Server) owedEvaluationChainReply(current DurableContinuation, sliceReply string) string {
	if loop, ok := s.freeStateLoop(current.ConversationID); ok {
		if composed := strings.TrimSpace(d1OwedEvaluationReply(loop)); composed != "" {
			return composed
		}
	}
	if reply := strings.TrimSpace(sliceReply); reply != "" {
		return reply
	}
	return s.schedulerChainFallbackReply(current.ConversationID)
}

// d1OwedEvaluationReply composes the explicit human-language receipt for the
// D1-STALL-1 shape: the applied move is reported with the same five-element
// vocabulary the applied turn and the judgment park use (d1AppliedSubject is
// the single resolution of track/parameter/change/readback, so the three turns
// cannot describe one move in three vocabularies), and the unfinished
// evaluation is stated as what it is — unfinished automatic work, not a
// completion — together with the entry that resumes it. It deliberately does
// not promise an A/B card that was never created, and it runs through the
// internal-code guardrail.
func d1OwedEvaluationReply(loop freeStateReasoningLoop) string {
	if loop.Experiment == nil {
		return ""
	}
	trackLabel, wording, changeText, readbackText := d1AppliedSubject(loop, d1JudgmentParkReceipt(loop))
	lines := []string{
		"这一步已经应用好了：" + trackLabel + wording.Parameter + " " + firstNonEmpty(changeText, "已调整") + readbackText + "。",
		"针对的发现：" + truncateTerminalFinding(d1FindingText(loop)) + "。",
		"但这轮还没做完：改动后的评估没有得出结论，自动续跑也已经用完，所以既没有终局汇报，也还没有生成 A/B 试听卡。这一步没有被判定为完成。",
		"你回一句“继续”，我就接着把这轮的评估做完，再把 A/B 试听卡给你做保留还是回滚的判定。",
	}
	return stripInternalTerminalTerms(strings.Join(lines, "\n"))
}

// normalizeChainResponseGoalStatus overrides a non-authoritative response
// goal status with the runtime goal's status: the runtime is the settle-time
// truth (a slice's waiting_continue ack must not outlive the goal's terminal
// form, and an empty status must not leak into the transport event).
func (s *Server) normalizeChainResponseGoalStatus(chainResp ChatResponse, status agentruntime.GoalStatus) {
	if chainResp.GoalStatus == "" ||
		strings.EqualFold(strings.TrimSpace(chainResp.GoalStatus), string(agentruntime.StatusWaitingContinue)) ||
		strings.EqualFold(strings.TrimSpace(chainResp.GoalStatus), string(agentruntime.StatusWaitingConfirmation)) ||
		strings.EqualFold(strings.TrimSpace(chainResp.GoalStatus), string(agentruntime.StatusWaitingClarification)) {
		chainResp.GoalStatus = string(status)
	}
}

// schedulerChainFallbackReply resolves the terminal reply text for a parked
// chain whose slice response carried none: the free-state loop's latest
// decision summary is where the terminal report lives (B1-DIAG Q1).
func (s *Server) schedulerChainFallbackReply(conversationID string) string {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || loop.LatestDecision == nil {
		return ""
	}
	return strings.TrimSpace(loop.LatestDecision.Summary)
}

// deliverSchedulerChainTerminal emits the chain's terminal delivery: the
// terminal reply is persisted into the project conversation graph first
// (B1-DIAG Q3: vit nodes existed only on the HTTP finalize boundary, so
// scheduler chain ends were invisible to the refresh hydration), then the
// scheduler_chain transport event fires for the live conversation.
// B5（尝试 #2 现场）：落图路径以链自己的 checkpoint 工作区为准——
// CurrentProjectIdentity 在会话中途再激活后会指向别的工作区，按它落图
// 会把终局写错工程或直接失败（20260911 手测：终局事件已投递而全盘
// grep 终局文本零命中）。
func (s *Server) deliverSchedulerChainTerminal(ctx context.Context, current DurableContinuation, resp ChatResponse) {
	recordPath := firstNonEmpty(current.ProjectPath, s.activeWorkspacePath)
	if strings.TrimSpace(recordPath) == "" && s.harness != nil {
		recordPath, _ = s.harness.CurrentProjectIdentity(ctx)
	}
	s.recordSchedulerChainTerminalNode(ctx, recordPath, resp)
	s.emitSchedulerChainResultEvent(current.ConversationID, resp, nil)
}

// AGENT-F5（2026-09-10 F2 验收移交项①）：失败链终局投递。executionErr 路径
// 此前在投递门之前 return，链失败时零 turn.failed 投递、零 vit 节点——失败形态
// 复刻 B1"server 有终态、用户拿不到"（成功路径已由 F2 修复，失败路径是同族缺
// 口）。失败终局复用 deliverSchedulerChainTerminal 的既有形状（先落图、再发
// scheduler_chain 传输事件）与纯 assistant 汇报规整；正文不新增 LLM 调用，取本
// 片自己的失败汇报与执行器错误文本，人话优先。
//
// 边界（不覆盖的形态）：ctx 取消/超时的执行错误不走这里——那是
// releaseContinuationClaim 的瞬态释放（记录回到 pending 等重试），“链已死亡”
// 的终局投递会对用户谎报（TestCancelledSliceReleaseDeliversNoTerminal 钉住）。
// 到这里的都是 setContinuationStatus(ContinuationFailed) 已把该 goal 的全部记录
// 收敛为失败终态、不存在后续重试或部分成功续跑的形态。
func (s *Server) deliverSchedulerChainFailure(ctx context.Context, continuationID string, resp ChatResponse, failure error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	current, exists := s.durableContinuations[continuationID]
	if exists {
		current = cloneDurableContinuation(current)
	}
	s.mu.Unlock()
	// Only the record that actually settled as failed owns the failure terminal:
	// a record that a concurrent actor settled otherwise must not emit a stale
	// "the chain died" event over its own terminal.
	if !exists || current.Status != ContinuationFailed {
		return
	}
	failureResp := schedulerChainFailureResponse(current, resp, failure)
	if strings.TrimSpace(failureResp.Reply) == "" {
		return
	}
	s.deliverSchedulerChainTerminal(ctx, current, failureResp)
}

// schedulerChainFailureResponse projects a failed slice into the terminal report
// shape the delivery gate consumes. Identity falls back to the durable record:
// the injected executor and the pre-handling executor failures return an empty
// response, and an identity-less response would make emitSchedulerChainResultEvent
// drop the terminal silently (goalID and runID both empty).
func schedulerChainFailureResponse(current DurableContinuation, resp ChatResponse, failure error) ChatResponse {
	out := resp
	out.ConversationID = firstNonEmpty(strings.TrimSpace(out.ConversationID), strings.TrimSpace(current.ConversationID))
	out.GoalID = firstNonEmpty(strings.TrimSpace(out.GoalID), strings.TrimSpace(current.GoalID))
	out.RunID = firstNonEmpty(strings.TrimSpace(out.RunID), strings.TrimSpace(current.RunID))
	detail := strings.TrimSpace(firstNonEmpty(out.Error, out.StopReason))
	if detail == "" && failure != nil {
		detail = strings.TrimSpace(failure.Error())
	}
	// FALLBACK-2 ② detail 护链：结算 reason 与执行器状态枚举共用同一个字面量
	// （audioclosure.StopTaskFailed / agentloop.StopReasonFailed / runtime
	// StatusFailed 都是 "failed"），于是 firstNonEmpty(out.Error, out.StopReason)
	// 会让状态枚举直接充当用户面失败详情。枚举只做最后兜底，且兜底必须是中文
	// 状态词；此前按 决策层原文 → 结算 summary → 执行器文案 的顺序回补。
	userDetail := stripSchedulerChainFailureEnvelope(detail)
	if schedulerChainStatusEnumLiteral(userDetail) {
		userDetail = schedulerChainFailureEnumFallback(out, failure)
	}
	// The payload error keeps the raw detail (and, when the slice carried a
	// decision-layer original, that complete text — FALLBACK-2 ④：不截断）；the
	// user-facing body shows the envelope-stripped form.
	out.Error = firstNonEmpty(strings.TrimSpace(out.Error), schedulerChainDecisionLayerText(out), detail)
	out.GoalStatus = string(agentruntime.StatusFailed)
	out.StopReason = firstNonEmpty(strings.TrimSpace(out.StopReason), agentloop.StopReasonFailed)
	if receipt, graded := schedulerChainDecisionReceiptFromResponse(out); graded {
		// FALLBACK-2 ①：决策层终局不冒充执行器故障。
		gates := schedulerChainDecisionGates(out)
		out.Reply = schedulerChainDecisionReply(strings.TrimSpace(out.Reply), receipt,
			schedulerChainDecisionGapLines(gates), schedulerChainDecisionNextStep(gates))
		return out
	}
	out.Reply = schedulerChainFailureReply(strings.TrimSpace(out.Reply), userDetail)
	return out
}

// ─── FALLBACK-2（2026-09-13）：终局分级与诚实结算 ─────────────────────────
//
// 缺陷（FALLBACK-1 回执 §4 六级降级链）：agentloop 的终局 fallback 把「门拒绝
// G4/G5/G6/G8（决定不可采）」与「JSON 解不开（模型协议失败）」压成同一个停机级，
// 链切片收口时再被 fmt.Errorf("durable continuation failed: %s",
// firstNonEmpty(Error, StopReason)) 压成 Go error，最后本文件的 firstNonEmpty 链
// 让状态枚举字面量 "failed" 充当用户面失败详情——用户拿到
// 「这条后台续跑链在执行中失败并已停止…失败详情：failed」（912.vit 两条 vit
// 节点原文），而决策层原文在内部三处完好保存。
//
// 修法（决策侧四条裁定，全部落在 chat 结算侧）：
//  1. 用户面三态分级：协议失败 / 决定不可采（门拒绝）/ 真执行失败；
//  2. detail 护链：枚举字面量不得充当失败详情，兜底用中文状态词；
//  3. stop reason 常量不重命名（agentloop 冻结面）——本文件只读既有字段；
//  4. durable.LastError 保留完整决策层原文，不截断。
//
// 判别输入的来源（全部是既有文本，零 agentloop 改动）：
//   - agentloop.Result.Trace 的终局 fallback final_gate 事件（哪个层拒绝的，
//     runAgentLoopChat 在切片边界把它折成回执挂到 WorkflowData）；
//   - WorkflowData["free_state_reasoning_loop"] 的 admission_rejection_gaps /
//     admission_receipt / last_error / latest_decision（结构化 gap 原文）；
//   - WorkflowData["minimal_audio_closure"].settlement（闭包结算 summary）。
const (
	// schedulerChainDecisionReceiptKey 是终局判别回执在 ChatResponse.WorkflowData
	// 上的键。回执由 runAgentLoopChat 从 agentloop.Result 的既有 Trace 派生，
	// 调度器只读它、不猜：没有回执的失败终局就是普通执行失败。
	schedulerChainDecisionReceiptKey = "chain_failure_decision"
	// schedulerChainDecisionReceiptSchema 是回执 schema 版本。
	schedulerChainDecisionReceiptSchema = "scheduler_chain_failure_decision.v1"
	// schedulerChainDecisionFamilyUnparsed：终局输出在解析/修复层就没能成为
	// 一个 JSON 对象（真模型协议失败，可重试）。
	schedulerChainDecisionFamilyUnparsed = "terminal_output_unparsed"
	// schedulerChainDecisionFamilyRefused：终局输出是可解析的决策，被准入门
	// （G1-G8）拒绝（决定不可采，不是执行故障）。
	schedulerChainDecisionFamilyRefused = "proposal_not_admitted"
)

// schedulerChainTerminalFallbackTracePrefix 是 agentloop 终局 fallback 写在
// Result.Trace 上的 final_gate 事件前缀（messageLoopTerminalFallbackResult）。
// chat 只读这一既有文本，不改 agentloop。
const schedulerChainTerminalFallbackTracePrefix = "terminal-turn fallback: "

// schedulerChainFailureDetailFallbackStatus 是枚举兜底的最后一跳：中文状态词，
// 永不是裸枚举。
const schedulerChainFailureDetailFallbackStatus = "执行失败（未提供更多详情）"

// schedulerChainDecisionReceipt is the decision-layer receipt a slice carries
// when its turn ended in the agentloop terminal fallback. It is deliberately
// minimal: the family that decides the user-facing wording, plus the decision
// layer's own reason text.
type schedulerChainDecisionReceipt struct {
	SchemaVersion string `json:"schema_version"`
	Family        string `json:"family"`
	Detail        string `json:"detail,omitempty"`
}

// schedulerChainDecisionReceiptFromResult derives the receipt from the
// agentloop result's terminal-fallback trace event. Only a failed slice whose
// trace carries the fallback event is graded; every other failure keeps the
// F5 execution-failure semantics verbatim.
func schedulerChainDecisionReceiptFromResult(res agentloop.Result) (schedulerChainDecisionReceipt, bool) {
	if res.Status != agentruntime.StatusFailed {
		return schedulerChainDecisionReceipt{}, false
	}
	detail := ""
	for _, event := range res.Trace {
		if !strings.EqualFold(strings.TrimSpace(event.Kind), "final_gate") {
			continue
		}
		message := strings.TrimSpace(event.Message)
		if !strings.HasPrefix(message, schedulerChainTerminalFallbackTracePrefix) {
			continue
		}
		detail = strings.TrimSpace(strings.TrimPrefix(message, schedulerChainTerminalFallbackTracePrefix))
	}
	if detail == "" {
		return schedulerChainDecisionReceipt{}, false
	}
	family, known := schedulerChainTerminalFallbackFamily(detail)
	if !known {
		// Fail-safe: a fallback reason outside the closed vocabulary this file
		// knows must not be graded into a claim the evidence does not support.
		// The terminal keeps the F5 execution-failure wording instead.
		return schedulerChainDecisionReceipt{}, false
	}
	return schedulerChainDecisionReceipt{
		SchemaVersion: schedulerChainDecisionReceiptSchema,
		Family:        family,
		Detail:        detail,
	}, true
}

// schedulerChainTerminalFallbackFamily maps the agentloop terminal-turn
// fallback reason onto the two closed families. The reasons are the fallback's
// own vocabulary (BOUNDARY-1 §3.2 parse layer / §1.3 admission layer):
//
//	parse layer    -> the raw output never became one JSON object
//	admission layer-> a parseable decision the gates (G1-G8) refused
//
// An unrecognized wording reports known=false (never a guessed family).
func schedulerChainTerminalFallbackFamily(detail string) (string, bool) {
	lowered := strings.ToLower(strings.TrimSpace(detail))
	switch {
	case strings.Contains(lowered, "unparseable"), strings.Contains(lowered, "not one clean json object"):
		return schedulerChainDecisionFamilyUnparsed, true
	case strings.Contains(lowered, "no admissible final decision"):
		return schedulerChainDecisionFamilyRefused, true
	}
	return "", false
}

// bindSchedulerChainDecisionReceipt attaches the receipt to a slice response
// whose turn ended in the terminal fallback. Any other response is returned
// untouched.
func bindSchedulerChainDecisionReceipt(resp ChatResponse, res agentloop.Result) ChatResponse {
	receipt, ok := schedulerChainDecisionReceiptFromResult(res)
	if !ok {
		return resp
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		return resp
	}
	row := map[string]any{}
	if json.Unmarshal(data, &row) != nil {
		return resp
	}
	if resp.WorkflowData == nil {
		resp.WorkflowData = map[string]any{}
	}
	resp.WorkflowData[schedulerChainDecisionReceiptKey] = row
	return resp
}

// schedulerChainDecisionReceiptFromResponse reads the receipt back. A response
// without a well-formed receipt is not graded (fail-safe: the F5 wording stays).
func schedulerChainDecisionReceiptFromResponse(resp ChatResponse) (schedulerChainDecisionReceipt, bool) {
	row := firstMapFromAny(resp.WorkflowData[schedulerChainDecisionReceiptKey])
	if len(row) == 0 {
		return schedulerChainDecisionReceipt{}, false
	}
	family := firstStringFromMap(row, "family")
	switch family {
	case schedulerChainDecisionFamilyUnparsed, schedulerChainDecisionFamilyRefused:
	default:
		return schedulerChainDecisionReceipt{}, false
	}
	return schedulerChainDecisionReceipt{
		SchemaVersion: firstStringFromMap(row, "schema_version"),
		Family:        family,
		Detail:        firstStringFromMap(row, "detail"),
	}, true
}

// schedulerChainMapValue reads a nested map that may be a plain map or a typed
// struct: the closure settlement rides WorkflowData as audioclosure.Settlement.
func schedulerChainMapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	row := map[string]any{}
	if json.Unmarshal(data, &row) != nil {
		return nil
	}
	return row
}

// schedulerChainDecisionLayerText is the decision layer's own original text in
// precedence order, verbatim and never truncated (FALLBACK-2 ④). The closure
// settlement summary and the durable loop's last_error are the two surfaces that
// carry the agentloop terminal fallback's complete sentence
// ("terminal turn produced no admissible final decision after one strengthened
// retry", the same text the two 912.vit goals kept internally); the receipt's
// own detail is the shorter fallback label and only stands in when neither
// survived. This is what the durable record keeps as the chain's failure reason
// and what the payload error carries; the user-facing body is what gets graded.
func schedulerChainDecisionLayerText(resp ChatResponse) string {
	if summary := schedulerChainClosureSettlementSummary(resp); summary != "" {
		return summary
	}
	if text := schedulerChainLoopDecisionLayerText(resp); text != "" {
		return text
	}
	if receipt, ok := schedulerChainDecisionReceiptFromResponse(resp); ok {
		if detail := strings.TrimSpace(receipt.Detail); detail != "" {
			return detail
		}
	}
	return ""
}

func schedulerChainClosureSettlementSummary(resp ChatResponse) string {
	for _, row := range []map[string]any{
		schedulerChainMapValue(resp.WorkflowData["settlement"]),
		schedulerChainMapValue(schedulerChainMapValue(resp.WorkflowData[audioClosureContextKey])["settlement"]),
	} {
		if summary := firstStringFromMap(row, "summary"); summary != "" {
			return summary
		}
	}
	return ""
}

func schedulerChainLoopDecisionLayerText(resp ChatResponse) string {
	loop := firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"])
	if len(loop) == 0 {
		return ""
	}
	if text := firstStringFromMap(loop, "last_error"); text != "" {
		return text
	}
	return firstStringFromMap(firstMapFromAny(loop["latest_decision"]), "summary")
}

// schedulerChainStatusEnumLiteral reports whether text is nothing but a status
// enum value (goal status / stop reason vocabulary). Such a value describes the
// executor's state, never the decision layer's reason, and must not stand in as
// a user-facing failure detail.
func schedulerChainStatusEnumLiteral(text string) bool {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "", "failed", "cancelled", "canceled", "stopped", "completed", "stable", "idle",
		"waiting_continue", "waiting_confirmation", "waiting_clarification":
		return true
	}
	return false
}

// schedulerChainFailureEnumFallback resolves the detail when the legacy chain
// would have used a bare status enum: the full decision-layer original first,
// then the executor's own text, and only then a Chinese status word.
func schedulerChainFailureEnumFallback(resp ChatResponse, failure error) string {
	candidates := []string{schedulerChainDecisionLayerText(resp)}
	if failure != nil {
		candidates = append(candidates, failure.Error())
	}
	for _, candidate := range candidates {
		candidate = stripSchedulerChainFailureEnvelope(candidate)
		if candidate != "" && !schedulerChainStatusEnumLiteral(candidate) {
			return candidate
		}
	}
	return schedulerChainFailureDetailFallbackStatus
}

// schedulerChainDecisionGates collects the structured admission-gap gate ids
// (free_state_admission_gap.v1). The accumulated rejection gaps are the
// terminal-turn disclosure; the receipt's gate ids are the fallback when no
// gap record survived the merge. Content-blind: ids and condition slots only.
func schedulerChainDecisionGates(resp ChatResponse) []string {
	loop := firstMapFromAny(resp.WorkflowData["free_state_reasoning_loop"])
	if len(loop) == 0 {
		return nil
	}
	gates := []string{}
	seen := map[string]bool{}
	appendGate := func(gate string) {
		gate = strings.TrimSpace(gate)
		if gate == "" || seen[gate] {
			return
		}
		seen[gate] = true
		gates = append(gates, gate)
	}
	for _, gap := range freeStateMapRows(loop["admission_rejection_gaps"]) {
		for _, missing := range freeStateMapRows(gap["missing"]) {
			appendGate(firstStringFromMap(missing, "gate_id"))
		}
		for _, gate := range schedulerChainStringRows(gap["failed_gate_ids"]) {
			appendGate(gate)
		}
	}
	if len(gates) == 0 {
		for _, gate := range schedulerChainStringRows(firstMapFromAny(loop["admission_receipt"])["failed_gate_ids"]) {
			appendGate(gate)
		}
	}
	return gates
}

func schedulerChainStringRows(value any) []string {
	switch rows := value.(type) {
	case []string:
		return append([]string(nil), rows...)
	case []any:
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if text := cleanContextText(row); text != "" {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

// schedulerChainDecisionGapLines renders the structured gate refusals in human
// words. Unknown ids keep their raw id rather than dropping the disclosure.
func schedulerChainDecisionGapLines(gates []string) []string {
	lines := []string{}
	seen := map[string]bool{}
	for _, gate := range gates {
		line := schedulerChainGateHumanLine(gate)
		if line == "" {
			line = "准入门没有通过：" + strings.TrimSpace(gate)
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
		if len(lines) == schedulerChainDecisionGapLineLimit {
			break
		}
	}
	return lines
}

const schedulerChainDecisionGapLineLimit = 3

func schedulerChainGateHumanLine(gateID string) string {
	switch strings.ToUpper(strings.TrimSpace(gateID)) {
	case "G1_PROJECT_BINDING":
		return "提案绑定的工程版本与当前工程对不上"
	case "G2_CAPACITY_ASSESSED":
		return "这一步超出了当前任务能自动推进的范围"
	case "G3_PROJECT_SCAN":
		return "还没有拿到合格的工程扫描证据"
	case "G4_DIMENSION_CLOSED":
		return "还没有一个证据齐全、没有遗留疑问的诊断维度可以下手"
	case "G5_FRONTIER_ESTABLISHED":
		return "还没有建立起可比较的候选（前沿为空）"
	case "G6_TARGET_EVIDENCE":
		return "缺少针对目标轨道自身的可用观察证据"
	case "G7_FRESH_REVISION_BOUND_REFS":
		return "引用的证据不是当前修订下的新鲜证据"
	case "G8_TARGET_CONSISTENCY":
		return "提案的目标与已有候选和证据对不上"
	}
	return ""
}

// schedulerChainDecisionNextStep names the honest next move. G1/G2/G8 are
// direction problems (realign the proposal), the evidence gates are observation
// problems.
func schedulerChainDecisionNextStep(gates []string) string {
	observation, redirect := false, false
	for _, gate := range gates {
		switch strings.ToUpper(strings.TrimSpace(gate)) {
		case "G1_PROJECT_BINDING", "G2_CAPACITY_ASSESSED", "G8_TARGET_CONSISTENCY":
			redirect = true
		default:
			observation = true
		}
	}
	switch {
	case observation && redirect:
		return "需要更多观察，或换一个方向"
	case redirect:
		return "换一个方向"
	default:
		return "需要更多观察"
	}
}

func schedulerChainDecisionGuidanceLine(nextStep string) string {
	switch nextStep {
	case "换一个方向":
		return "这不是执行故障——换一个方向：让提案对准已有的候选和证据，再试一次。"
	case "需要更多观察，或换一个方向":
		return "这不是执行故障——需要更多观察，或换一个方向：先补证据，或者换一条路，我再重新给出提案。"
	default:
		return "这不是执行故障——需要更多观察：先补一轮针对这个目标的证据，我再重新给出提案。"
	}
}

// schedulerChainDecisionReply composes the graded decision-terminal body. The
// F5 execution-failure lead is deliberately absent: nothing was executed and
// nothing failed to execute — the experiment decision did not become
// admissible, and saying "the chain failed while executing" is exactly the
// dishonesty this grading removes.
func schedulerChainDecisionReply(sliceReply string, receipt schedulerChainDecisionReceipt, gapLines []string, nextStep string) string {
	var lines []string
	switch receipt.Family {
	case schedulerChainDecisionFamilyUnparsed:
		lines = append(lines,
			"这次实验决策的输出格式无法解析，这条后台续跑链已经停下；已有的观察与证据都已保留。",
			"这不是执行故障——可以重试：直接说「继续」，我会重新走一遍这一步。")
	default:
		reason := strings.Join(gapLines, "；")
		if reason == "" {
			reason = "这一步的决定没有通过准入门检查"
		}
		lines = append(lines,
			"这次实验提案没有被采纳，这条后台续跑链已经停下；已有的观察与证据都已保留。",
			"没有被采纳的原因："+reason+"。",
			schedulerChainDecisionGuidanceLine(nextStep))
	}
	if sliceReply != "" {
		lines = append(lines, "这一步收到的汇报："+sliceReply)
	}
	return stripInternalTerminalTerms(strings.Join(lines, "\n"))
}

// schedulerChainFailureEnvelopePrefix is the envelope this file wraps around a
// slice failure before returning it ("durable continuation failed: <reason>").
// It is addressed at an operator reading a log line, so the user-facing failure
// body strips it and keeps the reason itself.
const schedulerChainFailureEnvelopePrefix = "durable continuation failed:"

// stripSchedulerChainFailureEnvelope removes repeated failure envelopes and
// leaves the reason. A detail consisting of nothing but the envelope is returned
// as-is rather than emptied: an empty detail line would silently downgrade the
// user surface to the bare lead sentence.
func stripSchedulerChainFailureEnvelope(detail string) string {
	stripped := strings.TrimSpace(detail)
	for strings.HasPrefix(stripped, schedulerChainFailureEnvelopePrefix) {
		stripped = strings.TrimSpace(strings.TrimPrefix(stripped, schedulerChainFailureEnvelopePrefix))
	}
	if stripped == "" {
		return strings.TrimSpace(detail)
	}
	return stripped
}

// schedulerChainFailureLead is the human lead sentence of a failed chain
// terminal. Its second clause reuses the settled-failure vocabulary the acoustic
// closure settlement already ships (audioclosure.StopTaskFailed): the failure
// reason and the evidence gathered so far are kept, not discarded.
const schedulerChainFailureLead = "这条后台续跑链在执行中失败并已停止，任务尚未完成；失败原因与已有证据均已保留。"

// schedulerChainFailureReply composes the user-facing body of a failed chain
// terminal: the human lead, the failed slice's own last report when it produced
// one, and the concrete failure detail. No LLM call is involved, and the B6
// internal-code guardrail runs over the composed text so a failure detail
// carrying a warehouse code never reaches a user surface.
func schedulerChainFailureReply(sliceReply, detail string) string {
	lines := []string{schedulerChainFailureLead}
	if sliceReply != "" && sliceReply != detail {
		lines = append(lines, "这一步收到的汇报："+sliceReply)
	}
	if detail != "" {
		lines = append(lines, "失败详情："+detail)
	}
	return stripInternalTerminalTerms(strings.Join(lines, "\n"))
}

// recordSchedulerChainTerminalNode writes the chain's terminal reply as a
// conversation-graph vit node. The shape mirrors the HTTP finalize write
// (finalizeInteractionChatResponse): chatResponseHistoryData metadata, plus
// project result cards when the chain produced any. The persisted node is a
// terminal REPORT, not an interaction offer: a waiting park's slice response
// rides proposal/confirmation metadata (serving the live interaction), and
// persisting it verbatim made the refresh hydration re-render the node as a
// consumed interaction card whose text then merged away the scheduler_chain
// bubble (real-stack run 20260910_202854: R4/R5 both red, the terminal only
// visible as 改善性提案待确认/已处理 card chrome). The interaction itself stays
// owned by the audition/proposal flow; the node keeps the terminal body.
// B5：落图失败必须显式留痕——RecordConversationNodeForProjectWithData 失败时
// 不返回 error 而是带 warnings 的摘要（F2 代码完全吞掉了这一信号，终局
// 落图静默失败、刷新即丢）。失败不阻断终局事件投递（失败降级：活会话
// 仍能看到终局，仅刷新水合缺节点），但 WARN 留痕可取证。
func (s *Server) recordSchedulerChainTerminalNode(ctx context.Context, projectPath string, resp ChatResponse) bool {
	if s == nil || s.harness == nil || strings.TrimSpace(resp.Reply) == "" {
		return false
	}
	node := resp
	node.NeedsConfirmation = false
	node.ProposalPresentation = nil
	node.InteractionRequests = nil
	node.MessageKind = "assistant"
	return s.recordConversationNodeChecked(ctx, projectPath, "scheduler_chain_terminal", "vit", node)
}

// recordConversationNodeChecked appends a conversation-graph node for resp
// under projectPath and surfaces failure explicitly: the harness record API
// reports errors as a warnings-carrying summary instead of an error value, so
// the caller-side silent drop (B5 尝试 #2) becomes a WARN with the full
// reason. kind "vit" rides the vit-checkpoint gate; non-vit kinds (e.g. the
// workspace-switch notice) bind the project HEAD without a kernel snapshot.
// Returns whether the node landed.
func (s *Server) recordConversationNodeChecked(ctx context.Context, projectPath, purpose, kind string, node ChatResponse) bool {
	if s.harness == nil {
		return false
	}
	historyData := chatResponseHistoryData(node, map[string]any{"artifacts": artifactSummaryRows(node.Artifacts)})
	if len(node.ProjectResultCards) > 0 {
		historyData["project_result_cards"] = node.ProjectResultCards
	}
	result := s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, kind, node.Reply, node.GoalID, node.RunID, historyData)
	if failure := conversationNodeRecordFailure(result); failure != "" {
		if s.logger != nil {
			s.logger.Warn("[workspace] conversation node NOT recorded purpose=%s kind=%s project=%s conversation=%s goal=%s reply_len=%d failure=%s",
				purpose, kind, projectPath, node.ConversationID, node.GoalID, len([]rune(node.Reply)), failure)
		}
		return false
	}
	return true
}

// conversationNodeRecordFailure extracts the record API's embedded failure
// text: RecordConversationNodeForProjectWithData returns the project history
// summary with a "conversation graph update failed: …" warning row instead of
// an error when the append fails.
func conversationNodeRecordFailure(result map[string]any) string {
	warnings, ok := result["warnings"].([]string)
	if !ok {
		return ""
	}
	for _, warning := range warnings {
		if strings.HasPrefix(warning, "conversation graph update failed:") {
			return strings.TrimSpace(strings.TrimPrefix(warning, "conversation graph update failed:"))
		}
	}
	return ""
}

func (s *Server) acquireRuntimeStateLease() (*history.AgentRuntimeStateLock, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	projectPath, projectUUID, owner := s.activeWorkspacePath, s.activeWorkspaceUUID, s.schedulerOwner
	s.mu.Unlock()
	if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(projectUUID) == "" {
		return nil, nil
	}
	return history.AcquireAgentRuntimeStateLock(projectPath, projectUUID, owner, s.continuationLease)
}

// persistContinuationStateUnlocked is used only while the scheduler owns the
// project-scoped runtime lease. All other callers use persistContinuationState.
func (s *Server) persistContinuationStateUnlocked() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	persist := s.continuationPersist
	s.mu.Unlock()
	if persist != nil {
		return persist()
	}
	return s.persistCurrentProjectWorkspaceChecked()
}

func (s *Server) reloadActiveRuntimeState() error {
	if s == nil {
		return nil
	}
	// Defense in depth: the reload replaces in-memory runtime state with the
	// durable snapshot. It must never race a request that has authoritative
	// state in memory which the snapshot cannot contain yet.
	if s.invocationsActive() {
		return nil
	}
	s.mu.Lock()
	projectPath, projectUUID := s.activeWorkspacePath, s.activeWorkspaceUUID
	s.mu.Unlock()
	if strings.TrimSpace(projectPath) == "" || strings.TrimSpace(projectUUID) == "" {
		return nil
	}
	data, err := history.ReadAgentRuntimeState(projectPath, projectUUID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	state := projectAgentRuntimeState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	s.mu.Lock()
	s.restoreProjectAgentRuntimeStateLocked(state)
	s.mu.Unlock()
	return nil
}

func (s *Server) executeDurableContinuation(ctx context.Context, item DurableContinuation) error {
	_, err := s.executeDurableContinuationWithResult(ctx, item)
	return err
}

// executeDurableContinuationWithResult additionally returns the slice's chat
// response so the scheduler can deliver the chain's final reply (AGENT-F6):
// the error-only shape silently discarded the last slice's outcome, which was
// the hand-test's "no result after multiple rounds" root cause.
func (s *Server) executeDurableContinuationWithResult(ctx context.Context, item DurableContinuation) (ChatResponse, error) {
	s.mu.Lock()
	activeProjectUUID := s.activeWorkspaceUUID
	s.mu.Unlock()
	if item.ProjectUUID != "" && activeProjectUUID != "" && item.ProjectUUID != activeProjectUUID {
		return ChatResponse{}, fmt.Errorf("continuation project changed: checkpoint=%s active=%s", item.ProjectUUID, activeProjectUUID)
	}
	cfg, _, err := config.Load()
	if err != nil {
		return ChatResponse{}, err
	}
	if !cfg.Complete() {
		return ChatResponse{}, fmt.Errorf("llm config incomplete")
	}
	contextSnapshot := cloneContext(item.Continuation.Context)
	if contextSnapshot == nil {
		contextSnapshot = map[string]any{}
	}
	contextSnapshot["durable_continuation"] = true
	contextSnapshot["continuation_id"] = item.ContinuationID
	contextSnapshot["task_id"] = item.TaskID
	contextSnapshot["goal_id"] = item.GoalID
	contextSnapshot["run_id"] = item.RunID
	contextSnapshot["slice_id"] = item.CurrentSliceID
	contextSnapshot["original_intent"] = item.OriginalIntent
	ctx = context.WithValue(ctx, durableContinuationContextKey{}, item.ContinuationID)
	resp, handled := s.runAgentLoopChat(ctx, item.ConversationID, ChatRequest{
		ConversationID: item.ConversationID,
		Message:        item.Continuation.UserText,
		Context:        contextSnapshot,
	}, cfg)
	if !handled {
		return resp, fmt.Errorf("durable continuation was not handled by agent loop")
	}
	if strings.TrimSpace(resp.Error) != "" || resp.GoalStatus == string(agentruntime.StatusFailed) {
		return resp, fmt.Errorf("durable continuation failed: %s", schedulerChainFailureReason(resp))
	}
	// An interaction boundary reached inside a scheduler-driven slice must
	// stay durably visible. The HTTP path surfaces these requests on the chat
	// response; the scheduler path has no such response channel, so the
	// claimed checkpoint is parked as waiting_interaction carrying them.
	// Without this the scheduler completed the checkpoint silently and the
	// confirmation was invisible to every continuation-based projection
	// (2026-08-25 D1 smoke: goal waiting_confirmation, all checkpoints
	// completed, no pending interaction).
	if interactionBoundaryChatResponse(resp) {
		s.parkClaimedContinuationAtInteraction(item, chatResponsePendingInteraction(resp))
	}
	return resp, nil
}

// schedulerChainFailureReason is the reason this file wraps in the operator
// envelope ("durable continuation failed: <reason>"). The envelope is what
// setContinuationStatus writes into the durable record's last_error, so it must
// carry the decision layer's complete original text rather than the status enum
// the legacy firstNonEmpty chain fell through to (FALLBACK-2 ④：durable.LastError
// 保留完整决策层原文，不截断).
func schedulerChainFailureReason(resp ChatResponse) string {
	if text := strings.TrimSpace(resp.Error); text != "" {
		return text
	}
	if text := schedulerChainDecisionLayerText(resp); text != "" {
		return text
	}
	if text := stripSchedulerChainFailureEnvelope(resp.StopReason); text != "" && !schedulerChainStatusEnumLiteral(text) {
		return text
	}
	return schedulerChainFailureDetailFallbackStatus
}

// interactionBoundaryChatResponse reports whether a chat response stops at a
// user-interaction boundary (confirmation, clarification, human judgment).
func interactionBoundaryChatResponse(resp ChatResponse) bool {
	if resp.NeedsConfirmation {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(resp.GoalStatus))
	return status == strings.ToLower(string(agentruntime.StatusWaitingConfirmation)) ||
		status == strings.ToLower(string(agentruntime.StatusWaitingClarification))
}

// chatResponsePendingInteraction projects a boundary response into the
// pending_interaction shape durable continuations expose. The interaction_id
// top-level key lets completePendingInteractionContinuation retire the
// checkpoint once the user answers.
func chatResponsePendingInteraction(resp ChatResponse) map[string]any {
	pending := map[string]any{
		"status":      firstNonEmpty(resp.GoalStatus, string(agentruntime.StatusWaitingConfirmation)),
		"stop_reason": resp.StopReason,
		"workflow":    resp.Workflow,
	}
	if strings.TrimSpace(resp.PlanID) != "" {
		pending["plan_id"] = resp.PlanID
	}
	if len(resp.InteractionRequests) > 0 {
		requests := make([]any, 0, len(resp.InteractionRequests))
		for _, req := range resp.InteractionRequests {
			requests = append(requests, agentInteractionRequestMap(req))
		}
		pending["requests"] = requests
		if id := firstInteractionRequestID(resp.InteractionRequests); id != "" {
			pending["interaction_id"] = id
		}
	}
	return pending
}

func agentInteractionRequestMap(req AgentInteractionRequest) map[string]any {
	data, err := json.Marshal(req)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

// parkClaimedContinuationAtInteraction retires the lease of a claimed
// checkpoint and parks it at waiting_interaction with the pending payload.
func (s *Server) parkClaimedContinuationAtInteraction(item DurableContinuation, pending map[string]any) {
	if s == nil {
		return
	}
	parked := false
	s.mu.Lock()
	current, ok := s.durableContinuations[item.ContinuationID]
	if ok && (current.Status == ContinuationClaimed || current.Status == ContinuationRunning) {
		current.Status = ContinuationWaitingInteraction
		current.LeaseOwner = ""
		current.LeaseExpiresAt = time.Time{}
		if len(pending) > 0 {
			current.PendingInteraction = cloneContext(pending)
		}
		current.UpdatedAt = time.Now().UTC()
		s.durableContinuations[item.ContinuationID] = cloneDurableContinuation(current)
		parked = true
	}
	s.mu.Unlock()
	if parked {
		// AGENT-F5：调度侧切片没有 HTTP 响应通道，park 的交互对一切事件消费
		// 者不可见（2026-09-04 手测：第 6 片 park 的确认卡双端都浮不出来）。
		// 事件补发让 UI 能在 park 当刻浮卡；HTTP 回复携带的交互不走此路径，
		// 无双发。
		payload := cloneContext(pending)
		if payload == nil {
			payload = map[string]any{}
		}
		payload["continuation_id"] = item.ContinuationID
		s.emitAgentEvent(item.ConversationID, AgentEvent{
			Type: "interaction.pending", GoalID: item.GoalID, RunID: item.RunID,
			ItemType: "interaction", Status: "waiting_interaction",
			Title: "待确认交互", Body: firstNonEmpty(firstStringFromMap(payload, "status"), "scheduler slice parked at a user interaction boundary"),
			Payload: payload,
		})
	}
	_ = s.persistContinuationState()
}

func (s *Server) continuationRuntimeProjection() []map[string]any {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	items := make([]DurableContinuation, 0, len(s.durableContinuations))
	for _, item := range s.durableContinuations {
		items = append(items, cloneDurableContinuation(item))
	}
	s.mu.Unlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ContinuationID < items[j].ContinuationID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := map[string]any{
			"continuation_id": item.ContinuationID, "task_id": item.TaskID,
			"goal_id": item.GoalID, "conversation_id": item.ConversationID, "run_id": item.RunID,
			"slice_id": item.CurrentSliceID, "turn_id": item.CurrentTurnID,
			"original_intent": item.OriginalIntent, "status": item.Status, "attempt": item.Attempt,
			"project_path": item.ProjectPath, "project_uuid": item.ProjectUUID,
			"project_session_id": item.ProjectSessionID, "project_revision": item.ProjectRevision,
			"updated_at": item.UpdatedAt,
		}
		if item.TaskContract != nil {
			row["task_contract"] = *item.TaskContract
		}
		if item.TaskSemanticState != nil {
			row["task_semantic_state"] = *item.TaskSemanticState
			row["task_state"] = item.TaskSemanticState.State
			row["task_state_revision"] = item.TaskSemanticState.Revision
		}
		if len(item.PendingInteraction) > 0 {
			row["pending_interaction"] = cloneContext(item.PendingInteraction)
		}
		// Terminal observability: the scheduler-driven settling turn's
		// ChatResponse never travels over HTTP, so the free-state loop's
		// terminal status and stop reason must be queryable here
		// (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md §3 observability gap).
		if loop, ok := s.freeStateLoop(item.ConversationID); ok {
			row["free_state_status"] = loop.Status
			row["free_state_current_round_id"] = loop.CurrentRoundID
			row["free_state_current_phase"] = loop.CurrentPhase
			row["free_state_continuation_budget"] = loop.ContinuationBudget
			row["free_state_continuation_used"] = loop.ContinuationUsed
			if loop.LatestDecision != nil {
				row["free_state_decision_status"] = loop.LatestDecision.Status
				row["free_state_stop_reason"] = firstNonEmpty(loop.LatestDecision.StopReason, loop.LastError)
				if len(loop.LatestDecision.Limitations) > 0 {
					row["free_state_limitations"] = append([]string(nil), loop.LatestDecision.Limitations...)
				}
			} else if loop.LastError != "" {
				row["free_state_stop_reason"] = loop.LastError
			}
			if len(loop.AdmissionReceipt) > 0 {
				row["free_state_admission_receipt"] = cloneContext(loop.AdmissionReceipt)
			}
		}
		if item.CapacityAssessment != nil {
			row["capacity_assessment"] = *item.CapacityAssessment
		}
		if item.CapabilityEntryPlan != nil {
			row["capability_entry_plan"] = *item.CapabilityEntryPlan
		}
		if item.LeaseOwner != "" {
			row["lease_owner"] = item.LeaseOwner
			row["lease_expires_at"] = item.LeaseExpiresAt
		}
		if item.LastError != "" {
			row["last_error"] = item.LastError
		}
		out = append(out, row)
	}
	return out
}
