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
	StatusProcessing           GoalStatus = "processing"
	StatusExecuting            GoalStatus = "executing"
	StatusWaitingConfirmation  GoalStatus = "waiting_confirmation"
	StatusWaitingClarification GoalStatus = "waiting_clarification"
	StatusWaitingContinue      GoalStatus = "waiting_continue"
	StatusCancelling           GoalStatus = "cancelling"
	StatusCancelled            GoalStatus = "cancelled"
	StatusStopped              GoalStatus = "stopped"
	StatusStable               GoalStatus = "stable"
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
	Task                *Task               `json:"task,omitempty"`
	Status              GoalStatus          `json:"status"`
	Summary             string              `json:"summary,omitempty"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
	CancelRequested     bool                `json:"cancel_requested"`
	CancelReason        string              `json:"cancel_reason,omitempty"`
	StopRequested       bool                `json:"stop_requested,omitempty"`
	StopReason          string              `json:"stop_reason,omitempty"`
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

type Snapshot struct {
	Sequence   int64  `json:"sequence,omitempty"`
	LastGoalID string `json:"last_goal_id,omitempty"`
	Goals      []Goal `json:"goals,omitempty"`
}

type ExecutionContext struct {
	RunID      string `json:"run_id,omitempty"`
	GoalID     string `json:"goal_id,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func New() *Runtime {
	return &Runtime{goals: map[string]Goal{}}
}

func (r *Runtime) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := Snapshot{Sequence: r.seq, LastGoalID: r.lastGoal, Goals: make([]Goal, 0, len(r.goals))}
	for _, goal := range r.goals {
		out.Goals = append(out.Goals, cloneGoal(goal))
	}
	return out
}

func (r *Runtime) Restore(snapshot Snapshot) {
	if r == nil {
		return
	}
	goals := make(map[string]Goal, len(snapshot.Goals))
	for _, goal := range snapshot.Goals {
		goal.GoalID = strings.TrimSpace(goal.GoalID)
		if goal.GoalID != "" {
			if goal.RunID == "" {
				goal.RunID = "run_" + randomID()
			}
			hydrateTask(&goal, time.Now())
			goals[goal.GoalID] = cloneGoal(goal)
		}
	}
	lastGoal := strings.TrimSpace(snapshot.LastGoalID)
	if _, ok := goals[lastGoal]; !ok {
		lastGoal = ""
	}
	r.mu.Lock()
	r.seq = snapshot.Sequence
	r.goals = goals
	r.lastGoal = lastGoal
	r.mu.Unlock()
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
	task := newTask(goal.GoalID, goal.RunID, summary, now)
	task.Run.TaskID = task.TaskID
	goal.Task = &task
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
		if goal.RunID == "" {
			goal.RunID = firstNonEmpty(runID, "run_"+randomID())
		}
		if goal.Status == StatusIdle {
			goal.Status = StatusRunning
		}
		hydrateTask(&goal, now)
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
	task := newTask(goal.GoalID, goal.RunID, summary, now)
	task.Run.TaskID = task.TaskID
	goal.Task = &task
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
		goal = Goal{GoalID: goalID, RunID: "run_" + randomID(), CreatedAt: now}
	}
	goal.Status = StatusRunning
	goal.Summary = firstNonEmpty(summary, goal.Summary)
	goal.CancelRequested = false
	goal.CancelReason = ""
	goal.StopRequested = false
	goal.StopReason = ""
	goal.Error = ""
	goal.LastCheckpoint = ""
	goal.UpdatedAt = now
	if goal.CreatedAt.IsZero() {
		goal.CreatedAt = now
	}
	hydrateTask(&goal, now)
	if goal.Task != nil {
		goal.Task.Status = TaskStatusActive
		goal.Task.UpdatedAt = now
		goal.Task.Run.RunID = goal.RunID
		// A restored process can observe a slice that was running when its
		// worker crashed. That invocation cannot still own execution; close its
		// open slice/turn before creating the recovery slice.
		for index := range goal.Task.Run.Slices {
			slice := &goal.Task.Run.Slices[index]
			if slice.SliceID == goal.Task.Run.CurrentSliceID && slice.Status == "running" {
				slice.Status = "interrupted"
				slice.EndedAt = now
			}
		}
		for index := range goal.Task.Run.Turns {
			turn := &goal.Task.Run.Turns[index]
			if turn.TurnID == goal.Task.Run.CurrentTurnID && turn.Status == "running" {
				turn.Status = "interrupted"
				turn.EndedAt = now
			}
		}
		goal.Task.Run.CurrentTurnID = ""
		if goal.Task.Run.NextSlice <= 0 {
			goal.Task.Run.NextSlice = len(goal.Task.Run.Slices) + 1
		}
		// A continuation is a new invocation slice in the same Run.
		slice := InvocationSlice{
			SliceID: "slice_" + randomID(), RunID: goal.RunID,
			Sequence: goal.Task.Run.NextSlice, Status: "running", StartedAt: now,
		}
		goal.Task.Run.Slices = append(goal.Task.Run.Slices, slice)
		goal.Task.Run.CurrentSliceID = slice.SliceID
		goal.Task.Run.NextSlice++
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

// RequestStop asks the active Turn to stop at the next runner checkpoint. It
// is distinct from confirmation cancellation: the current atomic tool call may
// finish, but no subsequent tool or Experiment Round may start.
func (r *Runtime) RequestStop(goalID, reason string) Goal {
	if r == nil {
		return Goal{Status: StatusStopped}
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
	if !IsActiveStatus(goal.Status) && goal.Status != StatusWaitingConfirmation && goal.Status != StatusWaitingClarification && goal.Status != StatusWaitingContinue {
		return cloneGoal(goal)
	}
	goal.StopRequested = true
	goal.StopReason = firstNonEmpty(reason, "user_stop")
	goal.Status = StatusCancelling
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	r.lastGoal = goalID
	return cloneGoal(goal)
}

// MarkStopped completes a requested Stop Turn after the latest stable project
// checkpoint is known.
func (r *Runtime) MarkStopped(goalID, checkpoint string) Goal {
	if r == nil {
		return Goal{Status: StatusStopped}
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
	goal.Status = StatusStopped
	goal.StopRequested = true
	goal.LastCheckpoint = firstNonEmpty(checkpoint, goal.LastCheckpoint)
	goal.UpdatedAt = time.Now()
	r.goals[goalID] = goal
	return cloneGoal(goal)
}

// IsActiveStatus identifies states that must guard Active Project Plane
// checkout operations.
func IsActiveStatus(status GoalStatus) bool {
	switch status {
	case StatusRunning, StatusProcessing, StatusExecuting, StatusCancelling:
		return true
	default:
		return false
	}
}

func (r *Runtime) CheckoutBlocked() (Goal, bool) {
	goal := r.Status("")
	return goal, IsActiveStatus(goal.Status)
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
	if goal.StopRequested && goal.Status == StatusCancelling {
		goal.Status = StatusStopped
	} else if goal.CancelRequested && goal.Status == StatusCancelling {
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
	} else if goal.StopRequested {
		goal.Status = StatusStopped
	} else if goal.CancelRequested {
		goal.Status = StatusCancelled
	} else {
		goal.Status = StatusCompleted
	}
	goal.UpdatedAt = time.Now()
	syncTaskStatus(goal.Task, goal.Status, goal.UpdatedAt)
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
	syncTaskStatus(goal.Task, goal.Status, goal.UpdatedAt)
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
	in.Task = cloneTaskPointer(in.Task)
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
