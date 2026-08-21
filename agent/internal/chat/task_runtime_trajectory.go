package chat

import (
	"sort"
	"strings"

	agentruntime "vit-daw-agent/internal/runtime"
)

// taskRuntimeTrajectorySchema is a read-only product projection. It is
// intentionally derived from durable runtime records rather than from the
// transient AgentEvent buffer, so a browser refresh or server restart can
// render the same Task identity and continue showing its execution history.
const taskRuntimeTrajectorySchema = "vit.task_runtime_trajectory.v1"

func (s *Server) taskRuntimeTrajectoryProjection(goal agentruntime.Goal) map[string]any {
	if goal.Task == nil || strings.TrimSpace(goal.Task.TaskID) == "" {
		return nil
	}
	task := goal.Task
	route := s.previousCapabilityRoute(task.TaskID, task.ConversationID)
	continuation := s.taskContinuationProjection(task.TaskID, goal.GoalID, goal.RunID)
	currentRevision := ""
	semantic := map[string]any{}
	if task.SemanticState != nil {
		currentRevision = task.SemanticState.ProjectRevision
		semantic = map[string]any{
			"state":             task.SemanticState.State,
			"revision":          task.SemanticState.Revision,
			"project_revision":  task.SemanticState.ProjectRevision,
			"terminal":          task.SemanticState.Terminal,
			"transition_reason": task.SemanticState.TransitionReason,
			"summary":           task.SemanticState.Summary,
			"evidence_refs":     append([]string(nil), task.SemanticState.EvidenceRefs...),
			"updated_at":        task.SemanticState.UpdatedAt,
		}
		if task.SemanticState.PendingInteraction != nil {
			semantic["pending_interaction"] = *task.SemanticState.PendingInteraction
		}
	}
	if task.Contract != nil {
		semantic["contract"] = map[string]any{
			"contract_id": task.Contract.ContractID,
			"kind":        task.Contract.Kind,
			"scope":       task.Contract.Scope,
			"temporary":   task.Contract.Temporary,
		}
	}

	slices := append([]agentruntime.InvocationSlice(nil), task.Run.Slices...)
	sort.Slice(slices, func(i, j int) bool { return slices[i].Sequence < slices[j].Sequence })
	turns := append([]agentruntime.Turn(nil), task.Run.Turns...)
	sort.Slice(turns, func(i, j int) bool { return turns[i].Sequence < turns[j].Sequence })
	history := make([]map[string]any, 0)
	if task.SemanticState != nil {
		for _, transition := range task.SemanticState.History {
			history = append(history, map[string]any{
				"revision":                   transition.Revision,
				"event":                      transition.Event,
				"from":                       transition.From,
				"to":                         transition.To,
				"reason":                     transition.Reason,
				"summary":                    transition.Summary,
				"evidence_refs":              append([]string(nil), transition.EvidenceRefs...),
				"candidate_id":               transition.CandidateID,
				"experiment_id":              transition.ExperimentID,
				"pending_interaction":        transition.PendingInteraction,
				"project_revision":           transition.ProjectRevision,
				"stale_for_current_revision": currentRevision != "" && transition.ProjectRevision != "" && transition.ProjectRevision != currentRevision,
				"occurred_at":                transition.OccurredAt,
			})
		}
	}

	out := map[string]any{
		"schema_version": taskRuntimeTrajectorySchema,
		"task": map[string]any{
			"task_id": task.TaskID, "goal_id": goal.GoalID, "run_id": goal.RunID,
			"conversation_id": task.ConversationID, "original_intent": task.OriginalIntent,
			"status": task.Status, "goal_status": goal.Status, "created_at": task.CreatedAt, "updated_at": task.UpdatedAt,
		},
		"run": map[string]any{
			"run_id": task.Run.RunID, "current_slice_id": task.Run.CurrentSliceID,
			"current_turn_id": task.Run.CurrentTurnID, "slices": slices, "turns": turns,
		},
		"semantic":    semantic,
		"transitions": history,
	}
	if continuation != nil {
		out["continuation"] = continuation
	}
	if route.SchemaVersion == capabilityRouteSchema {
		out["capability_route"] = capabilityRouteRecordMap(route)
	}
	return out
}

func (s *Server) taskContinuationProjection(taskID, goalID, runID string) map[string]any {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *DurableContinuation
	for _, item := range s.durableContinuations {
		if item.TaskID != taskID || item.GoalID != goalID || item.RunID != runID {
			continue
		}
		candidate := cloneDurableContinuation(item)
		if latest == nil || candidate.UpdatedAt.After(latest.UpdatedAt) || (candidate.UpdatedAt.Equal(latest.UpdatedAt) && candidate.ContinuationID > latest.ContinuationID) {
			latest = &candidate
		}
	}
	if latest == nil {
		return nil
	}
	out := map[string]any{
		"continuation_id": latest.ContinuationID, "slice_id": latest.CurrentSliceID,
		"turn_id": latest.CurrentTurnID, "status": latest.Status, "attempt": latest.Attempt,
		"updated_at": latest.UpdatedAt,
	}
	if len(latest.PendingInteraction) > 0 {
		out["pending_interaction"] = cloneContext(latest.PendingInteraction)
	}
	if latest.LastError != "" {
		out["last_error"] = latest.LastError
	}
	return out
}
