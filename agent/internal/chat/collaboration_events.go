package chat

import (
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/trajectory"
)

func (s *Server) emitInvokeCollaborationEvents(req harness.InvokeRequest, resp harness.InvokeResponse) {
	if s == nil || resp.Status != "ok" || len(resp.Result) == 0 {
		return
	}
	command := strings.ToLower(strings.TrimSpace(resp.CommandName))
	if command == "" {
		command = strings.ToLower(strings.TrimSpace(req.Tool))
	}
	args := cloneContext(req.Command)
	if len(args) == 0 {
		args = cloneContext(req.Args)
	}
	conversationID := firstNonEmpty(firstStringFromMap(args, "conversation_id", "owner_conversation_id"), firstStringFromMap(req.Context, "conversation_id"))
	if conversationID == "" {
		return
	}
	turnID := firstNonEmpty(firstStringFromMap(args, "turn_id"), firstStringFromMap(req.Context, "turn_id"), req.RunID, req.GoalID)
	if turnID == "" {
		turnID = "collaboration:" + sanitizeCanaryID(conversationID)
	}

	switch command {
	case "version_branch_create", "version.branch_create":
		branchRef := firstStringFromMap(resp.Result, "branch")
		if branchRef == "" {
			return
		}
		event := trajectory.Event{Type: trajectory.EventBranchCreated, ConversationID: conversationID, GoalID: req.GoalID, RunID: req.RunID, ItemID: branchRef, Title: "Branch created", Body: firstNonEmpty(firstStringFromMap(args, "purpose"), "Created Project History branch "+branchRef), Payload: trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TurnID: turnID, TraceNodeID: "branch:" + sanitizeCanaryID(branchRef), NodeKind: trajectory.NodeBranch, Status: trajectory.StatusCompleted, Summary: firstNonEmpty(firstStringFromMap(args, "purpose"), "branch created"), BranchRef: branchRef, CheckpointRef: firstStringFromMap(resp.Result, "source_commit_id", "commit_id"), ProjectRevision: firstStringFromMap(args, "project_revision", "source_project_revision"), Details: map[string]any{"parent_node_id": firstStringFromMap(args, "parent_node_id", "from_node_id"), "source_branch": firstStringFromMap(resp.Result, "source_branch"), "owner_agent_id": firstStringFromMap(args, "owner_agent_id", "agent_id"), "purpose": firstStringFromMap(args, "purpose"), "hypothesis": firstStringFromMap(args, "hypothesis"), "activated": resp.Result["activated"]}}}
		if _, err := s.emitTrajectoryEvent(conversationID, event); err != nil && s.logger != nil {
			s.logger.Warn("[collaboration] branch trajectory rejected: %v", err)
		}
	case "version_worktree_create", "version.worktree_create":
		worktreeRef := firstStringFromMap(resp.Result, "worktree_ref", "name")
		if worktreeRef == "" {
			return
		}
		reservation := firstMapFromAny(resp.Result["reservation"])
		event := trajectory.Event{Type: trajectory.EventWorktreeCreated, ConversationID: conversationID, GoalID: req.GoalID, RunID: req.RunID, ItemID: worktreeRef, Title: "Worktree created", Body: firstNonEmpty(firstStringFromMap(args, "purpose"), "Created isolated Project History worktree "+worktreeRef), Payload: trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TurnID: turnID, TraceNodeID: "worktree:" + sanitizeCanaryID(worktreeRef), NodeKind: trajectory.NodeBranch, Status: trajectory.StatusCompleted, Summary: firstNonEmpty(firstStringFromMap(args, "purpose"), "worktree created"), WorktreeRef: worktreeRef, CheckpointRef: firstNonEmpty(firstStringFromMap(resp.Result, "origin_commit_id", "commit_id"), firstStringFromMap(args, "commit_id")), ProjectRevision: firstStringFromMap(args, "project_revision", "source_project_revision"), Details: map[string]any{"parent_node_id": firstStringFromMap(resp.Result, "parent_node_id"), "source_branch": firstStringFromMap(resp.Result, "source_branch", "branch_ref"), "project_uuid": firstStringFromMap(resp.Result, "project_uuid"), "project_file_path": firstStringFromMap(resp.Result, "project_file_path"), "owner_agent_id": firstNonEmpty(firstStringFromMap(reservation, "owner_agent_id"), firstStringFromMap(args, "owner_agent_id", "agent_id")), "purpose": firstStringFromMap(args, "purpose"), "hypothesis": firstStringFromMap(args, "hypothesis"), "reservation": reservation}}}
		if _, err := s.emitTrajectoryEvent(conversationID, event); err != nil && s.logger != nil {
			s.logger.Warn("[collaboration] worktree trajectory rejected: %v", err)
		}
	case "collaboration_audition_prepare", "collaboration.audition_prepare":
		session := firstMapFromAny(resp.Result["session"])
		if len(session) == 0 {
			return
		}
		if loop, ok := s.freeStateLoop(conversationID); ok && loop.Experiment != nil {
			session["schema_version"] = auditionSchemaVersion
			session["conversation_id"] = conversationID
			session["turn_id"] = loop.Experiment.ID
			if round, roundErr := loop.Experiment.CurrentRound(); roundErr == nil {
				session["round_id"] = round.ID
				if firstStringFromMap(session, "project_revision") == "" {
					session["project_revision"] = round.ProjectRevision
				}
			}
			activePlane := firstMapFromAny(session["active_project_plane"])
			if firstStringFromMap(session, "project_revision") == "" {
				session["project_revision"] = firstStringFromMap(activePlane, "project_revision")
			}
			if firstStringFromMap(session, "active_project_ref") == "" {
				session["active_project_ref"] = firstStringFromMap(activePlane, "project_ref")
			}
			if candidates := auditionCandidateRows(session); len(candidates) > 0 && firstStringFromMap(session, "project_uuid") == "" {
				session["project_uuid"] = firstStringFromMap(candidates[0], "project_uuid")
			}
			loop.AuditionSessionID = firstStringFromMap(session, "session_id")
			loop.AuditionSessionSnapshot = cloneContext(session)
			loop.UpdatedAt = time.Now().UTC()
			s.storeFreeStateLoop(loop)
			s.persistCurrentProjectWorkspace()
		}
		eventType := "audition.prepare"
		if strings.EqualFold(firstStringFromMap(session, "status"), "ready") {
			eventType = "audition.ready"
		}
		s.emitAuditionEvent(conversationID, eventType, session, map[string]any{"command": "collaboration.audition_prepare", "active_project_plane_unchanged": true})
		if eventType == "audition.ready" {
			s.requestAuditionJudgment(conversationID, firstStringFromMap(session, "session_id"))
		}
	default:
		s.emitReservationLifecycleEvent(conversationID, turnID, command, resp.Result, req)
	}
}

func (s *Server) emitReservationLifecycleEvent(conversationID, turnID, command string, result map[string]any, req harness.InvokeRequest) {
	eventType := ""
	switch command {
	case "collaboration_worktree_reserve", "collaboration.worktree_reserve":
		eventType = "collaboration.worktree.reserved"
	case "collaboration_worktree_renew", "collaboration.worktree.renew":
		eventType = "collaboration.worktree.renewed"
	case "collaboration_worktree_recover_stale", "collaboration.worktree_recover_stale":
		eventType = "collaboration.worktree.stale_recovered"
	case "collaboration_worktree_release", "collaboration.worktree.release":
		eventType = "collaboration.worktree.released"
	case "collaboration_worktree_abandon", "collaboration.worktree.abandon":
		eventType = "collaboration.worktree.abandoned"
	case "collaboration_worktree_fail", "collaboration.worktree.fail":
		eventType = "collaboration.worktree.failed"
	case "collaboration_child_register", "collaboration.child_register":
		eventType = "collaboration.child.registered"
	case "collaboration_child_update", "collaboration.child_update":
		eventType = "collaboration.child.updated"
	case "collaboration_worktree_takeover", "collaboration.worktree_takeover":
		eventType = "collaboration.worktree.taken_over"
	default:
		return
	}
	reservation := firstMapFromAny(result["reservation"])
	child := firstMapFromAny(result["child_task"])
	itemID := firstNonEmpty(firstStringFromMap(reservation, "id"), firstStringFromMap(child, "id"))
	s.emitAgentEvent(conversationID, AgentEvent{Type: eventType, GoalID: req.GoalID, RunID: req.RunID, ItemID: itemID, ItemType: "collaboration", Status: "completed", Title: eventType, Body: fmt.Sprintf("%s %s", eventType, itemID), Payload: cloneContext(result), Lifecycle: "transient", Persistence: "none", MessageKind: "activity", TurnID: turnID, LogicalMessageID: "collaboration:" + turnID + ":" + sanitizeCanaryID(firstNonEmpty(itemID, eventType))})
}
