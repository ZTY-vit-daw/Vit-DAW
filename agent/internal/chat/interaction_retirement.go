package chat

import (
	"strings"
	"time"
)

// interactionRetirementBudget bounds the in-process retirement log. It exists
// so a repeated approve can be told apart from a lost one (FIX-GOV-
// INTERACTION-TTL-1 fail-closed: an approve must never be silently swallowed,
// and "already handled" must be distinguishable from "expired/lost"). The log
// is deliberately in-process only; cross-restart semantics are unchanged.
const interactionRetirementBudget = 128

// retiredInteraction records why a pending interaction left the answerable map.
type retiredInteraction struct {
	InteractionID string
	Kind          string
	Workflow      string
	Conversation  string
	RetiredAt     time.Time
	Reason        string
}

// recordInteractionRetirement notes a consumed or explicitly expired
// interaction so a later respond for the same id can return an explicit,
// idempotent "already retired" receipt instead of the ambiguous catch-all.
func (s *Server) recordInteractionRetirementLocked(interaction PendingInteraction, reason string) {
	if s == nil || strings.TrimSpace(interaction.ID) == "" {
		return
	}
	if s.interactionTombstones == nil {
		s.interactionTombstones = map[string]retiredInteraction{}
	}
	if len(s.interactionTombstones) >= interactionRetirementBudget {
		s.evictOldestInteractionRetirementLocked()
	}
	s.interactionTombstones[interaction.ID] = retiredInteraction{
		InteractionID: interaction.ID,
		Kind:          firstNonEmpty(interaction.Kind, interaction.Type),
		Workflow:      interaction.Workflow,
		Conversation:  interaction.ConversationID,
		RetiredAt:     time.Now().UTC(),
		Reason:        strings.TrimSpace(reason),
	}
}

func (s *Server) evictOldestInteractionRetirementLocked() {
	var oldestID string
	var oldest time.Time
	for id, record := range s.interactionTombstones {
		if oldestID == "" || record.RetiredAt.Before(oldest) {
			oldestID, oldest = id, record.RetiredAt
		}
	}
	if oldestID != "" {
		delete(s.interactionTombstones, oldestID)
	}
}

// retiredInteractionRecord returns the retirement record for an interaction id.
func (s *Server) retiredInteractionRecord(interactionID string) (retiredInteraction, bool) {
	if s == nil {
		return retiredInteraction{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.interactionTombstones[strings.TrimSpace(interactionID)]
	return record, ok
}

// recordPendingPlanRetirementLocked notes a consumed or explicitly expired
// pending plan in the same bounded retirement log
// (FIX-PLAN-STALE-STATUS-1: plan-face counterpart of the interaction
// retirement log). Plan ids never collide with interaction ids in practice
// ("plan_" vs "interaction_" prefixes), and sharing the log lets the plan
// decision faces reuse the same already-retired/not-found split the
// interaction respond face got in FIX-GOV-INTERACTION-TTL-1.
func (s *Server) recordPendingPlanRetirementLocked(plan PendingPlan, reason string) {
	if s == nil {
		return
	}
	conversation := ""
	if plan.WorkflowData != nil {
		conversation = cleanContextText(plan.WorkflowData["conversation_id"])
	}
	s.recordInteractionRetirementLocked(PendingInteraction{
		ID:             strings.TrimSpace(plan.ID),
		Kind:           "pending_plan_confirmation",
		Workflow:       plan.Workflow,
		ConversationID: conversation,
	}, reason)
}

// interactionContinuationMissingReceipt returns the explicit receipt for a
// decision on an interaction card that WAS found and consumed but whose
// workflow continuation (candidate/session) is gone
// (FIX-GOALSTATUS-FORGERY-SWEEP-1: the last three canned
// "已处理或已过期" workflow fallbacks — section-automation-choreo, native mix
// tick and mix treatment — that fabricated goal_status=completed for a
// decision that was never executed). Unlike the catch-all, the card itself
// existed, so the already-retired/not-found split collapses here into one
// honest state: the continuation is missing, the decision was NOT executed,
// and the goal did not complete because of it. takePendingInteraction already
// stamped the retirement log ("consumed"), so a repeated respond gets the
// interaction_already_retired receipt from the catch-all; this receipt is
// exempt from the goal_status refill and from closing the runtime goal
// (see receiptCarriesNoGoalStatus).
func (s *Server) interactionContinuationMissingReceipt(interaction PendingInteraction, subject string) ChatResponse {
	if s != nil && s.logger != nil {
		s.logger.Warn("[interaction.respond] workflow continuation missing for interaction=%s workflow=%s subject=%s (candidate expired, or lost to a workspace switch/restart); decision not executed", interaction.ID, interaction.Workflow, subject)
	}
	return ChatResponse{
		ConversationID: interaction.ConversationID,
		GoalID:         interaction.GoalID,
		RunID:          interaction.RunID,
		Reply:          "这个" + subject + "对应的执行候选已丢失（可能已过期，或随工作区切换/重启丢失）；本次决定未执行，未产生任何修改，目标未因此完成。",
		Workflow:       interaction.Workflow,
		WorkflowData: mergeContext(interaction.Payload, map[string]any{
			"interaction_id":     interaction.ID,
			"status":             "continuation_missing",
			"decision_executed":  false,
			"mutation_performed": false,
			"reason_candidates":  []string{"candidate_expired", "lost_to_workspace_switch", "lost_to_restart"},
		}),
		GoalStatus: "",
		StopReason: "interaction_continuation_missing",
	}
}

// preserveInFlightInteractions merges live in-memory interactions into a
// workspace snapshot being restored (FIX-GOV-INTERACTION-TTL-1). The restore
// replaces the answerable map wholesale with the snapshot read from disk; a
// confirmation card created after that snapshot was last persisted (the run9
// r2 shape: the card was parked during a chat whose tail then crossed a
// workspace identity boundary) would be silently dropped, and the arriving
// approve would hit the catch-all with a fabricated goal_status=completed.
//
// Rule: an in-memory interaction survives when the incoming snapshot does not
// contain its id and the snapshot predates the card (SavedAt zero = state of
// unknown age, fail-closed for live work). A snapshot whose SavedAt is newer
// than the card and still lacks it represents a decision by another owner —
// the card stays retired and the respond face returns the explicit receipt.
func preserveInFlightInteractions(current map[string]PendingInteraction, incoming map[string]PendingInteraction, savedAt time.Time) map[string]PendingInteraction {
	restored := nonNilMap(incoming)
	for id, interaction := range current {
		if _, ok := restored[id]; ok {
			continue
		}
		if !savedAt.IsZero() && !interaction.CreatedAt.After(savedAt) {
			continue
		}
		restored[id] = interaction
	}
	return restored
}
