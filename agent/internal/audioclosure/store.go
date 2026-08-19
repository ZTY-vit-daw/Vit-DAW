package audioclosure

import (
	"fmt"
	"sort"
	"sync"
)

type Store interface {
	Create(State) error
	Load(string) (State, bool)
	Save(State, uint64) error
	ActiveForConversation(string) (State, bool)
	Snapshot() map[string]State
	Restore(map[string]State) error
}

type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]State
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{states: map[string]State{}} }

func (s *MemoryStore) Create(state State) error {
	if err := validateStoredState(state); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.states[state.ClosureID]; exists {
		return fmt.Errorf("closure %s already exists", state.ClosureID)
	}
	for _, current := range s.states {
		if current.ConversationID == state.ConversationID && !current.Terminal() {
			return fmt.Errorf("conversation %s already has active closure %s", state.ConversationID, current.ClosureID)
		}
	}
	s.states[state.ClosureID] = cloneState(state)
	return nil
}

func (s *MemoryStore) Load(id string) (State, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[id]
	return cloneState(state), ok
}

func (s *MemoryStore) Save(state State, expectedRevision uint64) error {
	if err := validateStoredState(state); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.states[state.ClosureID]
	if !exists {
		return fmt.Errorf("closure %s does not exist", state.ClosureID)
	}
	if current.Revision != expectedRevision {
		return fmt.Errorf("closure revision conflict: expected %d, current %d", expectedRevision, current.Revision)
	}
	if state.Revision <= current.Revision {
		return fmt.Errorf("new closure revision %d must exceed current %d", state.Revision, current.Revision)
	}
	s.states[state.ClosureID] = cloneState(state)
	return nil
}

func (s *MemoryStore) ActiveForConversation(conversationID string) (State, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var matches []State
	for _, state := range s.states {
		if state.ConversationID == conversationID && !state.Terminal() {
			matches = append(matches, state)
		}
	}
	if len(matches) != 1 {
		return State{}, false
	}
	return cloneState(matches[0]), true
}

func (s *MemoryStore) Snapshot() map[string]State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]State, len(s.states))
	for id, state := range s.states {
		out[id] = cloneState(state)
	}
	return out
}

func (s *MemoryStore) Restore(states map[string]State) error {
	next := make(map[string]State, len(states))
	active := map[string]string{}
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		state := states[id]
		if id != state.ClosureID {
			return fmt.Errorf("closure map key %s does not match %s", id, state.ClosureID)
		}
		if err := validateStoredState(state); err != nil {
			return err
		}
		if !state.Terminal() {
			if other := active[state.ConversationID]; other != "" {
				return fmt.Errorf("conversation %s has active closures %s and %s", state.ConversationID, other, id)
			}
			active[state.ConversationID] = id
		}
		next[id] = cloneState(state)
	}
	s.mu.Lock()
	s.states = next
	s.mu.Unlock()
	return nil
}

func validateStoredState(state State) error {
	projected, err := Fold(state.Events)
	if err != nil {
		return err
	}
	if projected.ClosureID != state.ClosureID || projected.Revision != state.Revision || projected.Terminal() != state.Terminal() {
		return fmt.Errorf("closure projection does not match its event stream")
	}
	return nil
}

func cloneState(state State) State {
	projected, err := Fold(state.Events)
	if err != nil {
		return State{}
	}
	return projected
}
