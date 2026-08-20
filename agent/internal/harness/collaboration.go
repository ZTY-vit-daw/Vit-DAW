package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/collaboration"
	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/tools"
)

const defaultWorktreeLease = 30 * time.Minute

func (h *Harness) reserveWorktree(ctx context.Context, cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	currentPath, currentUUID := h.CurrentProjectIdentity(ctx)
	rootProjectPath := firstNonEmpty(firstString(cmd, "project_path", "root_project_path"), currentPath)
	projectUUID := firstNonEmpty(firstString(cmd, "project_uuid"), firstString(requestContext, "project_uuid", "project_id"), currentUUID)
	worktreeRef := firstString(cmd, "worktree_ref", "name")
	worktreePath := firstString(cmd, "worktree_project_path", "project_file_path", "path")
	if rootProjectPath == "" {
		return nil, fmt.Errorf("root project path is required to validate a worktree reservation")
	}
	listed, err := history.WorktreeList(map[string]any{"project_path": rootProjectPath})
	if err != nil {
		return nil, err
	}
	item := findWorktreeReservationItem(listed["worktrees"], worktreeRef, worktreePath)
	if item == nil {
		return nil, fmt.Errorf("worktree not found for reservation: %s", firstNonEmpty(worktreeRef, worktreePath))
	}
	itemProjectUUID := firstString(item, "project_uuid")
	if projectUUID != "" && itemProjectUUID != "" && projectUUID != itemProjectUUID {
		return nil, fmt.Errorf("worktree project identity mismatch: requested %s, actual %s", projectUUID, itemProjectUUID)
	}
	worktreeRef = firstNonEmpty(firstString(item, "worktree_ref", "name"), worktreeRef)
	worktreePath = firstNonEmpty(firstString(item, "project_file_path"), worktreePath)
	projectUUID = firstNonEmpty(projectUUID, itemProjectUUID, firstString(listed, "project_uuid"))
	lease := defaultWorktreeLease
	if seconds, ok := firstPositiveInt(cmd, "lease_seconds"); ok {
		lease = time.Duration(seconds) * time.Second
	}
	now := time.Now().UTC()
	reservation, err := h.collaboration.Reserve(collaboration.WorktreeReservation{
		ID: firstString(cmd, "reservation_id", "id"), ProjectUUID: projectUUID,
		WorktreeRef: worktreeRef, WorktreePath: worktreePath, BranchRef: firstString(cmd, "branch_ref", "source_branch"),
		ParentNodeID: firstString(cmd, "parent_node_id", "from_node_id"), ParentCommitID: firstString(cmd, "parent_commit_id", "source_commit_id", "commit_id"),
		SourceProjectRevision: firstString(cmd, "source_project_revision", "project_revision"),
		OwnerAgentID:          firstNonEmpty(firstString(cmd, "owner_agent_id", "agent_id"), firstString(requestContext, "owner_agent_id", "agent_id"), "agent:primary"),
		OwnerGoalID:           firstString(cmd, "owner_goal_id", "goal_id"), OwnerRunID: firstString(cmd, "owner_run_id", "run_id"),
		OwnerConversationID: firstNonEmpty(firstString(cmd, "owner_conversation_id", "conversation_id"), firstString(requestContext, "conversation_id")),
		Purpose:             firstString(cmd, "purpose"), Hypothesis: firstString(cmd, "hypothesis"), DisplayName: firstNonEmpty(firstString(cmd, "display_name"), worktreeRef),
		CreatedAt: now, ExpiresAt: now.Add(lease),
	})
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "reservation": reservation})
}

func (h *Harness) listWorktreeCollaboration(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	_, currentUUID := h.CurrentProjectIdentity(ctx)
	projectUUID := firstNonEmpty(firstString(cmd, "project_uuid"), currentUUID)
	stale := h.collaboration.RecoverStale(time.Now().UTC())
	return structMap(map[string]any{"status": "ok", "project_uuid": projectUUID, "reservations": h.collaboration.List(projectUUID), "child_tasks": h.collaboration.ListChildTasks(firstString(cmd, "parent_agent_id")), "recovered_stale": stale})
}

func (h *Harness) recoverStaleWorktreeReservations() (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	stale := h.collaboration.RecoverStale(time.Now().UTC())
	return structMap(map[string]any{"status": "ok", "stale_reservations": stale, "released_writer_count": len(stale), "worktrees_retained": true})
}

func (h *Harness) renewWorktreeReservation(cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	lease := defaultWorktreeLease
	if seconds, ok := firstPositiveInt(cmd, "lease_seconds"); ok {
		lease = time.Duration(seconds) * time.Second
	}
	now := time.Now().UTC()
	reservation, err := h.collaboration.Renew(firstString(cmd, "reservation_id", "id"), ownerAgent(cmd, requestContext), firstString(cmd, "owner_goal_id", "goal_id"), firstString(cmd, "owner_run_id", "run_id"), now.Add(lease), now)
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "reservation": reservation})
}

func (h *Harness) takeoverWorktreeReservation(cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	agentID := firstNonEmpty(firstString(cmd, "new_agent_id", "owner_agent_id", "agent_id"), firstString(requestContext, "agent_id", "owner_agent_id"))
	goalID := firstNonEmpty(firstString(cmd, "new_goal_id", "owner_goal_id", "goal_id"), firstString(requestContext, "goal_id", "owner_goal_id"))
	runID := firstNonEmpty(firstString(cmd, "new_run_id", "owner_run_id", "run_id"), firstString(requestContext, "run_id", "owner_run_id"))
	conversationID := firstNonEmpty(firstString(cmd, "new_conversation_id", "owner_conversation_id", "conversation_id"), firstString(requestContext, "conversation_id"))
	reservation, err := h.collaboration.Takeover(firstString(cmd, "reservation_id", "id"), agentID, goalID, runID, conversationID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "reservation": reservation, "explicit_takeover": true})
}

func (h *Harness) setWorktreeDisposition(cmd map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	reservation, err := h.collaboration.SetDisposition(firstString(cmd, "reservation_id", "id"), collaboration.WorktreeDisposition(strings.ToLower(firstString(cmd, "disposition"))), time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "reservation": reservation, "worktree_retained": true})
}

func (h *Harness) finishWorktreeReservation(cmd, requestContext map[string]any, status collaboration.ReservationStatus) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	id, agentID := firstString(cmd, "reservation_id", "id"), ownerAgent(cmd, requestContext)
	goalID, runID := firstString(cmd, "owner_goal_id", "goal_id"), firstString(cmd, "owner_run_id", "run_id")
	var reservation collaboration.WorktreeReservation
	var err error
	switch status {
	case collaboration.ReservationReleased:
		reservation, err = h.collaboration.Release(id, agentID, goalID, runID, time.Now().UTC())
	case collaboration.ReservationAbandoned:
		reservation, err = h.collaboration.Abandon(id, agentID, goalID, runID, time.Now().UTC())
	case collaboration.ReservationFailed:
		reservation, err = h.collaboration.Fail(id, agentID, goalID, runID, firstString(cmd, "error", "message"), time.Now().UTC())
	default:
		err = fmt.Errorf("unsupported reservation disposition %q", status)
	}
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "reservation": reservation, "worktree_retained": true})
}

func (h *Harness) registerChildTask(cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	reservationID := firstString(cmd, "reservation_id")
	reservation, ok := h.collaboration.Get(reservationID)
	if !ok {
		return nil, fmt.Errorf("reservation not found: %s", reservationID)
	}
	taskRequest := collaboration.ChildTask{
		ID: firstString(cmd, "child_task_id", "id"), ParentAgentID: firstNonEmpty(firstString(cmd, "parent_agent_id"), ownerAgent(cmd, requestContext)),
		AgentID: firstString(cmd, "child_agent_id", "agent_id"), GoalID: firstString(cmd, "child_goal_id", "goal_id"), RunID: firstString(cmd, "child_run_id", "run_id"),
		ConversationID: firstNonEmpty(firstString(cmd, "child_conversation_id", "conversation_id"), firstString(requestContext, "conversation_id")),
		WorktreeRef:    firstNonEmpty(firstString(cmd, "worktree_ref"), reservation.WorktreeRef), ReservationID: reservationID,
		Hypothesis: firstString(cmd, "hypothesis"), AllowedScope: stringSliceFromAny(cmd["allowed_scope"]),
		AllowedCapabilities: stringSliceFromAny(cmd["allowed_capabilities"]), Status: collaboration.ChildTaskPending,
	}
	if taskRequest.AgentID != reservation.OwnerAgentID || taskRequest.GoalID != reservation.OwnerGoalID || taskRequest.RunID != reservation.OwnerRunID || taskRequest.ConversationID != reservation.OwnerConversationID {
		parentAgent := firstNonEmpty(firstString(cmd, "parent_agent_id"), firstString(requestContext, "owner_agent_id", "agent_id"))
		parentGoal := firstNonEmpty(firstString(cmd, "parent_goal_id"), firstString(requestContext, "owner_goal_id", "goal_id"))
		parentRun := firstNonEmpty(firstString(cmd, "parent_run_id"), firstString(requestContext, "owner_run_id", "run_id"))
		transferred, transferErr := h.collaboration.TransferOwner(reservationID, parentAgent, parentGoal, parentRun, taskRequest.AgentID, taskRequest.GoalID, taskRequest.RunID, taskRequest.ConversationID, time.Now().UTC())
		if transferErr != nil {
			return nil, fmt.Errorf("child task cannot acquire reservation writer: %w", transferErr)
		}
		reservation = transferred
	}
	task, err := h.collaboration.RegisterChildTask(taskRequest)
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "child_task": task, "reservation": reservation})
}

func (h *Harness) updateChildTask(cmd map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	task, ok := h.collaboration.GetChildTask(firstString(cmd, "child_task_id", "id"))
	if !ok {
		return nil, fmt.Errorf("child task not found: %s", firstString(cmd, "child_task_id", "id"))
	}
	status := collaboration.ChildTaskStatus(strings.ToLower(firstString(cmd, "status")))
	switch status {
	case collaboration.ChildTaskPending, collaboration.ChildTaskRunning, collaboration.ChildTaskCompleted, collaboration.ChildTaskStopped, collaboration.ChildTaskFailed, collaboration.ChildTaskAbandoned:
	default:
		return nil, fmt.Errorf("unsupported child task status %q", status)
	}
	task.Status = status
	task.ResultCandidateRef = firstNonEmpty(firstString(cmd, "result_candidate_ref", "candidate_ref"), task.ResultCandidateRef)
	task.SettlementRef = firstNonEmpty(firstString(cmd, "settlement_ref"), task.SettlementRef)
	task.Error = firstNonEmpty(firstString(cmd, "error", "message"), task.Error)
	updated, err := h.collaboration.UpdateChildTask(task)
	if err != nil {
		return nil, err
	}
	reservation, _ := h.collaboration.Get(task.ReservationID)
	switch status {
	case collaboration.ChildTaskCompleted:
		reservation, err = h.collaboration.RecordCandidateAndRelease(task.ReservationID, task.ResultCandidateRef, firstString(cmd, "artifact_ref"), task.SettlementRef, time.Now().UTC())
	case collaboration.ChildTaskStopped:
		reservation, err = h.collaboration.Release(task.ReservationID, task.AgentID, task.GoalID, task.RunID, time.Now().UTC())
	case collaboration.ChildTaskFailed:
		reservation, err = h.collaboration.Fail(task.ReservationID, task.AgentID, task.GoalID, task.RunID, task.Error, time.Now().UTC())
	case collaboration.ChildTaskAbandoned:
		reservation, err = h.collaboration.Abandon(task.ReservationID, task.AgentID, task.GoalID, task.RunID, time.Now().UTC())
	}
	if err != nil {
		return nil, err
	}
	return structMap(map[string]any{"status": "ok", "child_task": updated, "reservation": reservation, "parent_project_unchanged": true})
}

func (h *Harness) listChildTasks(cmd, requestContext map[string]any) (map[string]any, error) {
	if h == nil || h.collaboration == nil {
		return nil, fmt.Errorf("worktree collaboration registry unavailable")
	}
	parent := firstNonEmpty(firstString(cmd, "parent_agent_id"), firstString(requestContext, "agent_id"))
	return structMap(map[string]any{"status": "ok", "child_tasks": h.collaboration.ListChildTasks(parent)})
}

func ownerAgent(cmd, requestContext map[string]any) string {
	return firstNonEmpty(firstString(cmd, "owner_agent_id", "agent_id"), firstString(requestContext, "owner_agent_id", "agent_id"), "agent:primary")
}

func findWorktreeReservationItem(value any, name, projectPath string) map[string]any {
	for _, item := range mapRowsFromAny(value) {
		if name != "" && (strings.EqualFold(firstString(item, "worktree_ref"), name) || strings.EqualFold(firstString(item, "name"), name)) {
			return item
		}
		if projectPath != "" && samePath(firstString(item, "project_file_path"), projectPath) {
			return item
		}
	}
	return nil
}

func structMap(value map[string]any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *Harness) collaborationChildPlaneGuard(spec tools.CommandSpec, cmd map[string]any, request InvokeRequest) (map[string]any, error) {
	if h == nil || h.collaboration == nil || !activeProjectPlaneCommand(spec.CommandName, cmd) {
		return nil, nil
	}
	agentID := firstNonEmpty(firstString(request.Context, "agent_id", "owner_agent_id"), firstString(cmd, "agent_id", "owner_agent_id"))
	goalID := firstNonEmpty(request.GoalID, firstString(cmd, "goal_id", "owner_goal_id"))
	runID := firstNonEmpty(request.RunID, firstString(cmd, "run_id", "owner_run_id"))
	task, ok := h.collaboration.ChildTaskForOwner(agentID, goalID, runID)
	if !ok {
		return nil, nil
	}
	err := fmt.Errorf("child agent %s cannot change the Active Project Plane while assigned to worktree %s", task.AgentID, task.WorktreeRef)
	return map[string]any{"status": "rejected", "error_code": "child_active_project_plane_forbidden", "child_task_id": task.ID, "worktree_ref": task.WorktreeRef, "reservation_id": task.ReservationID}, err
}

func (h *Harness) collaborationWriteGuard(spec tools.CommandSpec, cmd map[string]any, request InvokeRequest) (map[string]any, error) {
	if h == nil || h.collaboration == nil || !spec.MutatesProject {
		return nil, nil
	}
	projectUUID := firstNonEmpty(firstString(request.Context, "project_uuid", "project_id"), firstString(cmd, "project_uuid"))
	worktreeRef := firstNonEmpty(firstString(request.Context, "worktree_ref", "active_worktree"), firstString(cmd, "worktree_ref"))
	targetPath := firstString(cmd, "project_path", "project_file_path", "path")
	if targetPath != "" {
		if status, err := history.Status(map[string]any{"project_path": targetPath}); err == nil {
			targetProjectUUID := firstString(status, "project_uuid")
			targetWorktreeRef := firstNonEmpty(firstString(status, "active_worktree"), "main")
			projectUUID = firstNonEmpty(projectUUID, targetProjectUUID)
			if worktreeRef != "" && !strings.EqualFold(worktreeRef, targetWorktreeRef) {
				err := fmt.Errorf("reserved writer for worktree %s cannot target project plane %s", worktreeRef, targetWorktreeRef)
				return map[string]any{"status": "rejected", "error_code": "worktree_write_target_mismatch", "worktree_ref": worktreeRef, "target_worktree_ref": targetWorktreeRef, "target_project_path": targetPath}, err
			}
			worktreeRef = firstNonEmpty(worktreeRef, targetWorktreeRef)
		}
	}
	if projectUUID == "" || worktreeRef == "" {
		projectPath, currentUUID := h.CurrentProjectIdentity(context.Background())
		projectUUID = firstNonEmpty(projectUUID, currentUUID)
		if projectPath != "" {
			if status, err := history.Status(map[string]any{"project_path": projectPath}); err == nil {
				projectUUID = firstNonEmpty(projectUUID, firstString(status, "project_uuid"))
				worktreeRef = firstNonEmpty(worktreeRef, firstString(status, "active_worktree"))
			}
		}
	}
	if projectUUID == "" || worktreeRef == "" || strings.EqualFold(worktreeRef, "main") {
		return nil, nil
	}
	reservation, reserved := h.collaboration.ActiveReservation(projectUUID, worktreeRef)
	if !reserved {
		return nil, nil
	}
	agentID := firstNonEmpty(firstString(request.Context, "owner_agent_id", "agent_id"), firstString(cmd, "owner_agent_id", "agent_id"))
	goalID := firstNonEmpty(request.GoalID, firstString(cmd, "owner_goal_id", "goal_id"))
	runID := firstNonEmpty(request.RunID, firstString(cmd, "owner_run_id", "run_id"))
	if err := h.collaboration.AuthorizeWrite(projectUUID, worktreeRef, agentID, goalID, runID); err != nil {
		return map[string]any{
			"status": "rejected", "error_code": "worktree_reservation_owner_mismatch",
			"worktree_ref": worktreeRef, "reservation_id": reservation.ID,
			"owner_agent_id": reservation.OwnerAgentID, "owner_goal_id": reservation.OwnerGoalID,
			"owner_run_id": reservation.OwnerRunID,
		}, err
	}
	return nil, nil
}

func worktreeReservationRequested(cmd map[string]any) bool {
	if boolValueDefault(cmd["reserve"], false) {
		return true
	}
	return firstString(cmd, "owner_agent_id", "agent_id", "owner_goal_id", "owner_run_id", "owner_conversation_id") != ""
}
