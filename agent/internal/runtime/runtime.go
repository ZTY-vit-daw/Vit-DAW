package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type GoalStatus string

const (
	StatusIdle                 GoalStatus = "idle"
	StatusRunning              GoalStatus = "running"
	StatusWaitingConfirmation  GoalStatus = "waiting_confirmation"
	StatusWaitingClarification GoalStatus = "waiting_clarification"
	StatusWaitingContinue      GoalStatus = "waiting_continue"
	StatusCancelling           GoalStatus = "cancelling"
	StatusCancelled            GoalStatus = "cancelled"
	StatusCompleted            GoalStatus = "completed"
	StatusFailed               GoalStatus = "failed"
)

type Interjection struct {
	Sequence  int64     `json:"sequence"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
	Source    string    `json:"source,omitempty"`
}

type ProjectHistoryMeta struct {
	BaselineCommitID string   `json:"baseline_commit_id,omitempty"`
	BaselineCreated  bool     `json:"baseline_created,omitempty"`
	ActiveBranch     string   `json:"active_branch,omitempty"`
	Detached         bool     `json:"detached,omitempty"`
	Head             string   `json:"head,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

type Goal struct {
	GoalID              string              `json:"goal_id"`
	RunID               string              `json:"run_id"`
	Status              GoalStatus          `json:"status"`
	Summary             string              `json:"summary,omitempty"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
	CancelRequested     bool                `json:"cancel_requested"`
	CancelReason        string              `json:"cancel_reason,omitempty"`
	LastCheckpoint      string              `json:"last_checkpoint,omitempty"`
	ProjectHistory      *ProjectHistoryMeta `json:"project_history,omitempty"`
	PendingInterjection []Interjection      `json:"pending_interjections,omitempty"`
	Error               string              `json:"error,omitempty"`
}

type Runtime struct {
	mu       sync.RWMutex
	seq      int64
	goals    map[string]Goal
	lastGoal string
}

type ExecutionContext struct {
	RunID      string `json:"run_id,omitempty"`
	GoalID     string `json:"goal_id,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func New() *Runtime {
	return &Runtime{goals: map[string]Goal{}}
}

func (r *Runtime) Create(summary string) Goal {
	if r == nil {
		return Goal{}
	}
	now := time.Now()
	goal := Goal{
		GoalID:    "goal_" + randomID(),
		RunID:     "run_" + randomID(),
		Status:    StatusRunning,
		Summary:   strings.TrimSpace(summary),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[goal.GoalID] = goal
	r.lastGoal = goal.GoalID
	return cloneGoal(goal)
}

func (r *Runtime) Ensure(goalID, runID, summary string) Goal {
	if r == nil {
		return Goal{}
	}
	goalID = strings.TrimSpace(goalID)
	runID = strings.TrimSpace(runID)
	if goalID == "" {
		return r.Create(summary)
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if goal, ok := r.goals[goalID]; ok {
		if runID != "" {
			goal.RunID = runID
		}
		if goal.RunID == "" {
			goal.RunID = "run_" + randomID()
		}
		if goal.Status == StatusIdle {
			goal.Status = StatusRunning
		}
		goal.UpdatedAt = now
		r.goals[goalID] = goal
		r.lastGoal = goalID
		return cloneGoal(goal)
	}
	if runID == "" {
		runID = "run_" + randomID()
	}
	goal := Goal{
		GoalID:    goalID,
		RunID:     runID,
		Status:    StatusRunning,
		Summary:   strings.TrimSpace(summary),
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.goals[goalID] = goal
	r.lastGoal = goalID
	return cloneGoal(goal)
}

func (r *Runtime) Continue(goalID, summary string) Goal {
	if r == nil {
		return Goal{}
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		return r.Create(summary)
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	goal, ok := r.goals[goalID]
	if !ok {
		goal = Goal{GoalID: goalID, CreatedAt: now}
	}
	goal.RunID = "run_" + randomID()
	goal.Status = StatusRunning
	goal.Summary = firstNonEmpty(summary, goal.Summary)
	goal.CancelRequested = false
	goal.CancelReason = ""
	goal.Error = ""
	goal.LastCheckpoint = ""
	goal.UpdatedAt = now
	if goal.CreatedAt.IsZero() {
		goal.CreatedAt = now
	}
	r.goals[goalID] = goal
	r.lastGoal = goalID
	return cloneGoal(goal)
}
func (r *Runtime) ClearInterjections(goalID string) Goal {
	if r == nil {
		return Goal{}
	}
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	goal.PendingInterjection = nil
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	return cloneGoal(goal)
}
func (r *Runtime) Status(goalID string) Goal {
	if r == nil {
		return Goal{Status: StatusIdle}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	if goal, ok := r.goals[goalID]; ok {
		return cloneGoal(goal)
	}
	return Goal{GoalID: goalID, Status: StatusIdle}
}

func (r *Runtime) Cancel(goalID, reason string) Goal {
	if r == nil {
		return Goal{Status: StatusCancelled}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		goal = Goal{GoalID: goalID, RunID: "run_" + randomID(), CreatedAt: time.Now()}
	}
	goal.Status = StatusCancelling
	goal.CancelRequested = true
	goal.CancelReason = strings.TrimSpace(reason)
	goal.UpdatedAt = time.Now()
	r.goals[goal.GoalID] = goal
	r.lastGoal = goal.GoalID
	return cloneGoal(goal)
}

func (r *Runtime) Tick(goalID, checkpoint string) Goal {
	if r == nil {
		return Goal{Status: StatusIdle}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	goal.LastCheckpoint = strings.TrimSpace(checkpoint)
	goal.UpdatedAt = time.Now()
	if goal.CancelRequested && goal.Status == StatusCancelling {
		goal.Status = StatusCancelled
	}
	r.goals[goalID] = goal
	return cloneGoal(goal)
}

func (r *Runtime) AddInterjection(goalID, message, source string) Goal {
	if r == nil {
		return Goal{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		goal = Goal{
			GoalID:    firstNonEmpty(goalID, "goal_"+randomID()),
			RunID:     "run_" + randomID(),
			Status:    StatusRunning,
			CreatedAt: time.Now(),
		}
	}
	r.seq++
	goal.PendingInterjection = append(goal.PendingInterjection, Interjection{
		Sequence:  r.seq,
		CreatedAt: time.Now(),
		Message:   strings.TrimSpace(message),
		Source:    strings.TrimSpace(source),
	})
	goal.UpdatedAt = time.Now()
	r.goals[goal.GoalID] = goal
	r.lastGoal = goal.GoalID
	return cloneGoal(goal)
}

func (r *Runtime) Complete(goalID string, failed error) Goal {
	if r == nil {
		return Goal{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	if failed != nil {
		goal.Status = StatusFailed
		goal.Error = failed.Error()
	} else if goal.CancelRequested {
		goal.Status = StatusCancelled
	} else {
		goal.Status = StatusCompleted
	}
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	return cloneGoal(goal)
}

func (r *Runtime) SetStatus(goalID string, status GoalStatus, err error) Goal {
	if r == nil {
		return Goal{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	goal.Status = status
	if err != nil {
		goal.Error = err.Error()
	}
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	return cloneGoal(goal)
}

func (r *Runtime) SetProjectHistory(goalID string, meta ProjectHistoryMeta) Goal {
	if r == nil {
		return Goal{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = strings.TrimSpace(goalID)
	if goalID == "" {
		goalID = r.lastGoal
	}
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	meta.Warnings = append([]string(nil), meta.Warnings...)
	goal.ProjectHistory = &meta
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	return cloneGoal(goal)
}

func NewToolCallID() string {
	return "tool_" + randomID()
}

func cloneGoal(in Goal) Goal {
	in.PendingInterjection = append([]Interjection(nil), in.PendingInterjection...)
	if in.ProjectHistory != nil {
		meta := *in.ProjectHistory
		meta.Warnings = append([]string(nil), in.ProjectHistory.Warnings...)
		in.ProjectHistory = &meta
	}
	return in
}

func randomID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000000")))
	}
	return hex.EncodeToString(b[:])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
