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
