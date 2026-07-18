package pendingmanager

import (
	"strings"
	"sync"

	"vit-daw-agent/internal/agentprotocol"
)

type Manager interface {
	Upsert(candidate agentprotocol.PendingCandidate)
	Get(id string) (agentprotocol.PendingCandidate, bool)
	ActiveForConversation(conversationID string) []agentprotocol.PendingCandidate
	ActiveForGoal(goalID string) []agentprotocol.PendingCandidate
	ActiveForTarget(targetRef string) []agentprotocol.PendingCandidate
	Transition(id string, status string, reason string) (agentprotocol.PendingCandidate, bool)
}

type MemoryManager struct {
	mu         sync.Mutex
	candidates map[string]agentprotocol.PendingCandidate
}

func NewMemoryManager() *MemoryManager {
	return &MemoryManager{candidates: map[string]agentprotocol.PendingCandidate{}}
}

func (m *MemoryManager) Upsert(candidate agentprotocol.PendingCandidate) {
	if m == nil || strings.TrimSpace(candidate.ID) == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if candidate.Kind == "" {
		candidate.Kind = agentprotocol.KindPendingCandidate
	}
	m.candidates[candidate.ID] = cloneCandidate(candidate)
}

func (m *MemoryManager) Get(id string) (agentprotocol.PendingCandidate, bool) {
	if m == nil || strings.TrimSpace(id) == "" {
		return agentprotocol.PendingCandidate{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	candidate, ok := m.candidates[strings.TrimSpace(id)]
	if !ok {
		return agentprotocol.PendingCandidate{}, false
	}
	return cloneCandidate(candidate), true
}

func (m *MemoryManager) ActiveForConversation(conversationID string) []agentprotocol.PendingCandidate {
	return m.active(func(candidate agentprotocol.PendingCandidate) bool {
		return strings.TrimSpace(candidate.Source.ConversationID) == strings.TrimSpace(conversationID)
	})
}

func (m *MemoryManager) ActiveForGoal(goalID string) []agentprotocol.PendingCandidate {
	return m.active(func(candidate agentprotocol.PendingCandidate) bool {
		return strings.TrimSpace(candidate.Source.GoalID) == strings.TrimSpace(goalID)
	})
}

func (m *MemoryManager) ActiveForTarget(targetRef string) []agentprotocol.PendingCandidate {
	return m.active(func(candidate agentprotocol.PendingCandidate) bool {
		return strings.TrimSpace(candidate.TargetRef) == strings.TrimSpace(targetRef)
	})
}

func (m *MemoryManager) Transition(id string, status string, reason string) (agentprotocol.PendingCandidate, bool) {
	if m == nil || strings.TrimSpace(id) == "" || strings.TrimSpace(status) == "" {
		return agentprotocol.PendingCandidate{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	candidate, ok := m.candidates[strings.TrimSpace(id)]
	if !ok {
		return agentprotocol.PendingCandidate{}, false
	}
	candidate.Status = strings.TrimSpace(status)
	if strings.TrimSpace(reason) != "" {
		if candidate.Source.Metadata == nil {
			candidate.Source.Metadata = map[string]any{}
		}
		candidate.Source.Metadata["transition_reason"] = strings.TrimSpace(reason)
	}
	m.candidates[candidate.ID] = cloneCandidate(candidate)
	return cloneCandidate(candidate), true
}

func (m *MemoryManager) Snapshot() []agentprotocol.PendingCandidate {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]agentprotocol.PendingCandidate, 0, len(m.candidates))
	for _, candidate := range m.candidates {
		out = append(out, cloneCandidate(candidate))
	}
	return out
}

func (m *MemoryManager) Restore(candidates []agentprotocol.PendingCandidate) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.candidates = make(map[string]agentprotocol.PendingCandidate, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) != "" {
			m.candidates[candidate.ID] = cloneCandidate(candidate)
		}
	}
}

func (m *MemoryManager) active(match func(agentprotocol.PendingCandidate) bool) []agentprotocol.PendingCandidate {
	if m == nil || match == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []agentprotocol.PendingCandidate{}
	for _, candidate := range m.candidates {
		if !isActiveStatus(candidate.Status) || !match(candidate) {
			continue
		}
		out = append(out, cloneCandidate(candidate))
	}
	return out
}

func isActiveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case agentprotocol.PendingStatusRejected,
		agentprotocol.PendingStatusCommitted,
		agentprotocol.PendingStatusFailed,
		agentprotocol.PendingStatusVerified,
		agentprotocol.PendingStatusVerificationFailed,
		agentprotocol.PendingStatusBlocked:
		return false
	default:
		return true
	}
}

func cloneCandidate(candidate agentprotocol.PendingCandidate) agentprotocol.PendingCandidate {
	candidate.EvidenceRefs = append([]string(nil), candidate.EvidenceRefs...)
	candidate.NeedsResolution = append([]string(nil), candidate.NeedsResolution...)
	candidate.RequiredPermissionDomains = append([]string(nil), candidate.RequiredPermissionDomains...)
	candidate.CandidateAction = cloneMap(candidate.CandidateAction)
	candidate.Source.ArtifactIDs = append([]string(nil), candidate.Source.ArtifactIDs...)
	candidate.Source.Metadata = cloneMap(candidate.Source.Metadata)
	return candidate
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
