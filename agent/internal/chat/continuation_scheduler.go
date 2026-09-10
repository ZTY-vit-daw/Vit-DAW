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
		return
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
	for _, id := range ids {
		item := s.durableContinuations[id]
		if item.Status != ContinuationPending &&
			!(item.Status == ContinuationClaimed || item.Status == ContinuationRunning) {
			continue
		}
		if (item.Status == ContinuationClaimed || item.Status == ContinuationRunning) &&
			!item.LeaseExpiresAt.IsZero() && now.Before(item.LeaseExpiresAt) {
			continue
		}
		if item.ProjectUUID != "" && s.activeWorkspaceUUID != "" && item.ProjectUUID != s.activeWorkspaceUUID {
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
		return cloneDurableContinuation(item), true
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
		return nil
	}
	s.schedulerExecutionMu.Lock()
	defer s.schedulerExecutionMu.Unlock()
	stateLock, err := s.acquireRuntimeStateLease()
	if err != nil {
		if errors.Is(err, history.ErrAgentRuntimeStateLocked) {
			return nil
		}
		return err
	}
	if err := s.reloadActiveRuntimeState(); err != nil {
		if stateLock != nil {
			_ = stateLock.Release()
		}
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
// Mid-chain slices, unanswerable legacy shells (goal still waiting_continue),
// and the test executor (no real resp) deliver nothing.
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
		s.deliverSchedulerChainTerminal(ctx, current.ConversationID, chainResp)
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
		s.deliverSchedulerChainTerminal(ctx, current.ConversationID, chainResp)
	}
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
func (s *Server) deliverSchedulerChainTerminal(ctx context.Context, conversationID string, resp ChatResponse) {
	s.recordSchedulerChainTerminalNode(ctx, resp)
	s.emitSchedulerChainResultEvent(conversationID, resp, nil)
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
func (s *Server) recordSchedulerChainTerminalNode(ctx context.Context, resp ChatResponse) {
	if s == nil || s.harness == nil || strings.TrimSpace(resp.Reply) == "" {
		return
	}
	projectPath, _ := s.harness.CurrentProjectIdentity(ctx)
	node := resp
	node.NeedsConfirmation = false
	node.ProposalPresentation = nil
	node.InteractionRequests = nil
	node.MessageKind = "assistant"
	historyData := chatResponseHistoryData(node, map[string]any{"artifacts": artifactSummaryRows(node.Artifacts)})
	if len(node.ProjectResultCards) > 0 {
		historyData["project_result_cards"] = node.ProjectResultCards
	}
	s.harness.RecordConversationNodeForProjectWithData(ctx, projectPath, "vit", node.Reply, node.GoalID, node.RunID, historyData)
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
		return resp, fmt.Errorf("durable continuation failed: %s", firstNonEmpty(resp.Error, resp.StopReason, "unknown failure"))
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
