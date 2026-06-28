package chat

import (
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

func (s *Server) upsertPendingCandidate(candidate agentprotocol.PendingCandidate) {
	if s == nil || s.pendingManager == nil || strings.TrimSpace(candidate.ID) == "" {
		return
	}
	s.pendingManager.Upsert(candidate)
}

func (s *Server) transitionActivePendingCandidate(conversationID, candidateType, status, reason string) {
	if s == nil || s.pendingManager == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	for _, pending := range s.pendingManager.ActiveForConversation(conversationID) {
		if strings.TrimSpace(candidateType) != "" && !strings.EqualFold(strings.TrimSpace(pending.CandidateType), strings.TrimSpace(candidateType)) {
			continue
		}
		s.pendingManager.Transition(pending.ID, status, reason)
	}
}
