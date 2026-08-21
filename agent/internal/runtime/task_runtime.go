package runtime

import (
	"strings"
	"time"
)

// TaskStatus describes the lifecycle of the durable product task. It is
// intentionally separate from GoalStatus: Goal remains the compatibility
// surface used by the existing chat runner, while Task is the long-lived
// identity that survives invocation boundaries.
type TaskStatus string

const (
	TaskStatusActive              TaskStatus = "active"
	TaskStatusWaitingContinuation TaskStatus = "waiting_continuation"
	TaskStatusWaitingInteraction  TaskStatus = "waiting_interaction"
	TaskStatusSettled             TaskStatus = "settled"
	TaskStatusCancelled           TaskStatus = "cancelled"
	TaskStatusFailed              TaskStatus = "failed"
)

// Task is the durable identity and intent root for one user task. Its
// OriginalIntent is immutable after creation; continuation text is state for
// the next slice and must never replace this field.
type Task struct {
	TaskID         string     `json:"task_id"`
	GoalID         string     `json:"goal_id"`
	ConversationID string     `json:"conversation_id,omitempty"`
	OriginalIntent string     `json:"original_intent"`
	Status         TaskStatus `json:"status"`
	Run            Run        `json:"run"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Run is stable for the lifetime of a Task. A max_turns limit belongs to an
// InvocationSlice, never to the whole Run.
type Run struct {
	RunID          string            `json:"run_id"`
	TaskID         string            `json:"task_id"`
	CurrentSliceID string            `json:"current_slice_id,omitempty"`
	CurrentTurnID  string            `json:"current_turn_id,omitempty"`
	NextSlice      int               `json:"next_slice,omitempty"`
	NextTurn       int               `json:"next_turn,omitempty"`
	Slices         []InvocationSlice `json:"slices,omitempty"`
	Turns          []Turn            `json:"turns,omitempty"`
}

// InvocationSlice is one bounded execution reservation. Its budget is local
// to the slice, so reaching max_turns creates a continuation boundary rather
// than a terminal Task result.
type InvocationSlice struct {
	SliceID       string    `json:"slice_id"`
	RunID         string    `json:"run_id"`
	Sequence      int       `json:"sequence"`
	MaxTurns      int       `json:"max_turns,omitempty"`
	MaxToolCalls  int       `json:"max_tool_calls,omitempty"`
	TurnsUsed     int       `json:"turns_used,omitempty"`
	ToolCallsUsed int       `json:"tool_calls_used,omitempty"`
	Status        string    `json:"status"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at,omitempty"`
}

// Turn is a durable logical turn. User and automatic continuation turns share
// the same Run and are distinguishable by Source.
type Turn struct {
	TurnID    string    `json:"turn_id"`
	SliceID   string    `json:"slice_id"`
	RunID     string    `json:"run_id"`
	Sequence  int       `json:"sequence"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// TaskIntent returns the immutable task intent while retaining compatibility
// with legacy goals that have not yet been hydrated with a Task record.
func (g Goal) TaskIntent() string {
	if g.Task != nil && g.Task.OriginalIntent != "" {
		return g.Task.OriginalIntent
	}
	return g.Summary
}

func newTask(goalID, runID, originalIntent string, now time.Time) Task {
	return Task{
		TaskID:         "task_" + randomID(),
		GoalID:         goalID,
		OriginalIntent: firstNonEmpty(originalIntent, "task"),
		Status:         TaskStatusActive,
		Run:            Run{RunID: runID, TaskID: "", NextSlice: 1, NextTurn: 1},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func normalizeTask(task *Task, goal Goal, now time.Time) {
	if task == nil {
		return
	}
	if task.TaskID == "" {
		task.TaskID = "task_" + randomID()
	}
	if task.GoalID == "" {
		task.GoalID = goal.GoalID
	}
	if task.OriginalIntent == "" {
		task.OriginalIntent = firstNonEmpty(goal.Summary, "task")
	}
	if task.Status == "" {
		task.Status = taskStatusFromGoal(goal.Status)
	}
	if task.Run.RunID == "" {
		task.Run.RunID = goal.RunID
	}
	if task.Run.TaskID == "" {
		task.Run.TaskID = task.TaskID
	}
	if task.Run.NextSlice <= 0 {
		task.Run.NextSlice = len(task.Run.Slices) + 1
	}
	if task.Run.NextTurn <= 0 {
		task.Run.NextTurn = len(task.Run.Turns) + 1
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = firstTime(goal.CreatedAt, now)
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = firstTime(goal.UpdatedAt, now)
	}
}

func hydrateTask(goal *Goal, now time.Time) {
	if goal == nil {
		return
	}
	if goal.Task == nil {
		task := newTask(goal.GoalID, goal.RunID, goal.Summary, now)
		task.Run.TaskID = task.TaskID
		goal.Task = &task
	}
	normalizeTask(goal.Task, *goal, now)
	goal.Task.Run.RunID = goal.RunID
}

func taskStatusFromGoal(status GoalStatus) TaskStatus {
	switch status {
	case StatusWaitingContinue:
		return TaskStatusWaitingContinuation
	case StatusWaitingConfirmation, StatusWaitingClarification:
		return TaskStatusWaitingInteraction
	case StatusCompleted, StatusStable:
		return TaskStatusSettled
	case StatusCancelled, StatusStopped:
		return TaskStatusCancelled
	case StatusFailed:
		return TaskStatusFailed
	default:
		return TaskStatusActive
	}
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Now()
}

func cloneTask(in Task) Task {
	in.Run.Slices = append([]InvocationSlice(nil), in.Run.Slices...)
	in.Run.Turns = append([]Turn(nil), in.Run.Turns...)
	return in
}

func cloneTaskPointer(in *Task) *Task {
	if in == nil {
		return nil
	}
	out := cloneTask(*in)
	return &out
}

func syncTaskStatus(task *Task, goalStatus GoalStatus, now time.Time) {
	if task == nil {
		return
	}
	task.Status = taskStatusFromGoal(goalStatus)
	task.UpdatedAt = now
	for index := range task.Run.Slices {
		slice := &task.Run.Slices[index]
		if slice.SliceID != task.Run.CurrentSliceID || slice.Status != "running" {
			continue
		}
		switch task.Status {
		case TaskStatusWaitingContinuation:
			slice.Status = "waiting_continuation"
		case TaskStatusWaitingInteraction:
			slice.Status = "waiting_interaction"
		case TaskStatusSettled, TaskStatusCancelled, TaskStatusFailed:
			slice.Status = string(task.Status)
		default:
			continue
		}
		slice.EndedAt = now
	}
}

// BeginSlice opens (or reuses) the current invocation slice. Reuse makes the
// operation safe when a runner is restored after a persisted boundary.
func (r *Runtime) BeginSlice(goalID string, maxTurns, maxToolCalls int) (Goal, InvocationSlice, bool) {
	if r == nil {
		return Goal{}, InvocationSlice{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goalID = firstNonEmpty(goalID, r.lastGoal)
	goal, ok := r.goals[goalID]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}, InvocationSlice{}, false
	}
	now := time.Now()
	hydrateTask(&goal, now)
	if goal.Task.Run.CurrentSliceID != "" {
		for index := range goal.Task.Run.Slices {
			candidate := &goal.Task.Run.Slices[index]
			if candidate.SliceID == goal.Task.Run.CurrentSliceID && candidate.Status == "running" {
				if maxTurns > 0 {
					candidate.MaxTurns = maxTurns
				}
				if maxToolCalls > 0 {
					candidate.MaxToolCalls = maxToolCalls
				}
				goal.Task.UpdatedAt = now
				goal.UpdatedAt = now
				r.goals[goalID] = goal
				return cloneGoal(goal), *candidate, true
			}
		}
	}
	if goal.Task.Run.NextSlice <= 0 {
		goal.Task.Run.NextSlice = len(goal.Task.Run.Slices) + 1
	}
	slice := InvocationSlice{SliceID: "slice_" + randomID(), RunID: goal.RunID, Sequence: goal.Task.Run.NextSlice, MaxTurns: maxTurns, MaxToolCalls: maxToolCalls, Status: "running", StartedAt: now}
	goal.Task.Run.Slices = append(goal.Task.Run.Slices, slice)
	goal.Task.Run.CurrentSliceID = slice.SliceID
	goal.Task.Run.NextSlice++
	goal.Task.Status = TaskStatusActive
	goal.Task.UpdatedAt = now
	goal.UpdatedAt = now
	r.goals[goalID] = goal
	return cloneGoal(goal), slice, true
}

// BeginTurn records the next logical turn in an already opened slice.
func (r *Runtime) BeginTurn(goalID, sliceID, source string) (Goal, Turn, bool) {
	if r == nil {
		return Goal{}, Turn{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goal, ok := r.goals[strings.TrimSpace(goalID)]
	if !ok {
		return Goal{GoalID: goalID, Status: StatusIdle}, Turn{}, false
	}
	now := time.Now()
	hydrateTask(&goal, now)
	if sliceID == "" {
		sliceID = goal.Task.Run.CurrentSliceID
	}
	if sliceID == "" {
		return cloneGoal(goal), Turn{}, false
	}
	for _, existing := range goal.Task.Run.Turns {
		if existing.SliceID == sliceID && existing.Status == "running" {
			return cloneGoal(goal), existing, true
		}
	}
	if goal.Task.Run.NextTurn <= 0 {
		goal.Task.Run.NextTurn = len(goal.Task.Run.Turns) + 1
	}
	turn := Turn{TurnID: "turn_" + randomID(), SliceID: sliceID, RunID: goal.RunID, Sequence: goal.Task.Run.NextTurn, Source: firstNonEmpty(source, "user"), Status: "running", StartedAt: now}
	goal.Task.Run.Turns = append(goal.Task.Run.Turns, turn)
	goal.Task.Run.CurrentTurnID = turn.TurnID
	goal.Task.Run.NextTurn++
	goal.Task.UpdatedAt = now
	goal.UpdatedAt = now
	r.goals[goal.GoalID] = goal
	return cloneGoal(goal), turn, true
}

// EndTurn closes one logical turn and records the cumulative slice counters.
func (r *Runtime) EndTurn(goalID, turnID, status string, turnsUsed, toolCallsUsed int) Goal {
	if r == nil {
		return Goal{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	goal, ok := r.goals[strings.TrimSpace(goalID)]
	if !ok || goal.Task == nil {
		return Goal{GoalID: goalID, Status: StatusIdle}
	}
	now := time.Now()
	for index := range goal.Task.Run.Turns {
		turn := &goal.Task.Run.Turns[index]
		if turn.TurnID != strings.TrimSpace(turnID) {
			continue
		}
		if turn.Status == "running" {
			turn.Status = firstNonEmpty(status, "completed")
			turn.EndedAt = now
		}
		for sliceIndex := range goal.Task.Run.Slices {
			slice := &goal.Task.Run.Slices[sliceIndex]
			if slice.SliceID != turn.SliceID {
				continue
			}
			slice.TurnsUsed = turnsUsed
			slice.ToolCallsUsed = toolCallsUsed
		}
		goal.Task.Run.CurrentTurnID = ""
		goal.Task.UpdatedAt = now
		goal.UpdatedAt = now
		r.goals[goal.GoalID] = goal
		return cloneGoal(goal)
	}
	return cloneGoal(goal)
}
