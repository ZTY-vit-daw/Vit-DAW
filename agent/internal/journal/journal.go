package journal

import (
	"sync"
	"time"
)

type ActionStatus string

const (
	StatusPendingConfirmation ActionStatus = "pending_confirmation"
	StatusRunning             ActionStatus = "running"
	StatusSucceeded           ActionStatus = "succeeded"
	StatusFailed              ActionStatus = "failed"
	StatusRejected            ActionStatus = "rejected"
)

type Action struct {
	AgentActionID        string         `json:"agent_action_id"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	Source               string         `json:"source"`
	Summary              string         `json:"summary"`
	Tool                 string         `json:"tool"`
	CommandName          string         `json:"command_name"`
	Command              map[string]any `json:"command"`
	RiskLevel            string         `json:"risk_level"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	ConfirmationStatus   string         `json:"confirmation_status"`
	Status               ActionStatus   `json:"status"`
	UndoLabel            string         `json:"undo_label,omitempty"`
	Result               map[string]any `json:"result,omitempty"`
	Error                string         `json:"error,omitempty"`
}

type Journal struct {
	mu      sync.RWMutex
	max     int
	actions []Action
}

func New(max int) *Journal {
	if max <= 0 {
		max = 200
	}
	return &Journal{max: max}
}

func (j *Journal) Record(action Action) Action {
	if j == nil {
		return action
	}
	now := time.Now()
	if action.CreatedAt.IsZero() {
		action.CreatedAt = now
	}
	action.UpdatedAt = now
	j.mu.Lock()
	defer j.mu.Unlock()
	j.actions = append(j.actions, cloneAction(action))
	if len(j.actions) > j.max {
		j.actions = append([]Action(nil), j.actions[len(j.actions)-j.max:]...)
	}
	return action
}

func (j *Journal) MarkResult(actionID string, status ActionStatus, result map[string]any, err error) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.actions {
		if j.actions[i].AgentActionID != actionID {
			continue
		}
		j.actions[i].UpdatedAt = time.Now()
		j.actions[i].Status = status
		j.actions[i].Result = cloneMap(result)
		if err != nil {
			j.actions[i].Error = err.Error()
		} else {
			j.actions[i].Error = ""
		}
		return
	}
}

func (j *Journal) Recent(limit int) []Action {
	if j == nil {
		return nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if limit <= 0 || limit > len(j.actions) {
		limit = len(j.actions)
	}
	start := len(j.actions) - limit
	out := make([]Action, 0, limit)
	for i := len(j.actions) - 1; i >= start; i-- {
		out = append(out, cloneAction(j.actions[i]))
	}
	return out
}

func (j *Journal) Get(actionID string) (Action, bool) {
	if j == nil {
		return Action{}, false
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	for i := len(j.actions) - 1; i >= 0; i-- {
		if j.actions[i].AgentActionID == actionID {
			return cloneAction(j.actions[i]), true
		}
	}
	return Action{}, false
}

func cloneAction(in Action) Action {
	in.Command = cloneMap(in.Command)
	in.Result = cloneMap(in.Result)
	return in
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
