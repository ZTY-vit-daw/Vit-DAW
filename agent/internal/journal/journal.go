package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	StatusRolledBack          ActionStatus = "rolled_back"
)

type Action struct {
	AgentActionID        string         `json:"agent_action_id"`
	RunID                string         `json:"run_id,omitempty"`
	GoalID               string         `json:"goal_id,omitempty"`
	ToolCallID           string         `json:"tool_call_id,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	Domain               string         `json:"domain,omitempty"`
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
	TargetActionID       string         `json:"target_action_id,omitempty"`
	RollbackActionID     string         `json:"rollback_action_id,omitempty"`
	RollbackState        string         `json:"rollback_state,omitempty"`
	VersionCommitID      string         `json:"version_commit_id,omitempty"`
	Result               map[string]any `json:"result,omitempty"`
	Error                string         `json:"error,omitempty"`
}

type Journal struct {
	mu      sync.RWMutex
	max     int
	path    string
	actions []Action
}

func New(max int) *Journal {
	if max <= 0 {
		max = 200
	}
	return &Journal{max: max}
}

func NewPersistent(max int, path string) (*Journal, error) {
	j := New(max)
	j.path = stringsTrim(path)
	if j.path == "" {
		return j, nil
	}
	if err := j.load(); err != nil {
		return j, err
	}
	return j, nil
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
	_ = j.persistLocked()
	return action
}

func (j *Journal) MarkResult(actionID string, status ActionStatus, result map[string]any, err error) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := len(j.actions) - 1; i >= 0; i-- {
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
		_ = j.persistLocked()
		return
	}
}

func (j *Journal) MarkRollback(targetActionID, rollbackActionID, state string) {
	if j == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := len(j.actions) - 1; i >= 0; i-- {
		if j.actions[i].AgentActionID != targetActionID {
			continue
		}
		j.actions[i].UpdatedAt = time.Now()
		j.actions[i].RollbackActionID = rollbackActionID
		j.actions[i].RollbackState = state
		if state == "succeeded" {
			j.actions[i].Status = StatusRolledBack
		}
		_ = j.persistLocked()
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

func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.path
}

func (j *Journal) load() error {
	if j == nil || j.path == "" {
		return nil
	}
	b, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("journal: read %q: %w", j.path, err)
	}
	var actions []Action
	if err := json.Unmarshal(b, &actions); err != nil {
		return fmt.Errorf("journal: parse %q: %w", j.path, err)
	}
	if j.max > 0 && len(actions) > j.max {
		actions = append([]Action(nil), actions[len(actions)-j.max:]...)
	}
	j.actions = actions
	return nil
}

func (j *Journal) persistLocked() error {
	if j == nil || j.path == "" {
		return nil
	}
	dir := filepath.Dir(j.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("journal: mkdir %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(j.actions, "", "  ")
	if err != nil {
		return fmt.Errorf("journal: marshal: %w", err)
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("journal: write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, j.path); err != nil {
		return fmt.Errorf("journal: replace %q: %w", j.path, err)
	}
	return nil
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

func stringsTrim(v string) string {
	for len(v) > 0 && (v[0] == ' ' || v[0] == '\t' || v[0] == '\r' || v[0] == '\n') {
		v = v[1:]
	}
	for len(v) > 0 {
		last := v[len(v)-1]
		if last != ' ' && last != '\t' && last != '\r' && last != '\n' {
			break
		}
		v = v[:len(v)-1]
	}
	return v
}
