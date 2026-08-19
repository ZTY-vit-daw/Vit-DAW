package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/trajectory"
)

const (
	authorityModeManual = "manual_confirmation"
	authorityModeFull   = "full_project_access"
)

func normalizeAuthorityMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", authorityModeManual, "ordinary":
		return authorityModeManual, nil
	case authorityModeFull, "full_access":
		return authorityModeFull, nil
	default:
		return "", fmt.Errorf("unsupported authority_mode %q", value)
	}
}

func normalizeAuthorityModeOrDefault(value string) string {
	mode, err := normalizeAuthorityMode(value)
	if err != nil {
		return authorityModeManual
	}
	return mode
}

func experimentAuthorityMode(value string) experiment.AuthorityMode {
	if normalizeAuthorityModeOrDefault(value) == authorityModeFull {
		return experiment.AuthorityFull
	}
	return experiment.AuthorityOrdinary
}

func authorityModeFromContext(ctx map[string]any) experiment.AuthorityMode {
	return experimentAuthorityMode(firstStringFromMap(ctx, "authority_mode", "permission_mode"))
}

func (s *Server) authorityModeSnapshot() string {
	if s == nil {
		return authorityModeManual
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return normalizeAuthorityModeOrDefault(s.authorityMode)
}

func (s *Server) bindChatAuthorityMode(requested string, ctx map[string]any) (map[string]any, error) {
	if s == nil {
		return ctx, nil
	}
	if strings.TrimSpace(requested) == "" {
		requested = firstStringFromMap(ctx, "authority_mode", "permission_mode")
	}
	current := s.authorityModeSnapshot()
	if strings.TrimSpace(requested) == "" {
		requested = current
	}
	mode, err := normalizeAuthorityMode(requested)
	if err != nil {
		return ctx, err
	}
	if goal, blocked := s.authorityModeChangeBlocked(); blocked && mode != current {
		return ctx, fmt.Errorf("authority mode is locked while goal %s is %s", goal.GoalID, goal.Status)
	}
	if loop, ok := s.freeStateLoop(firstStringFromMap(ctx, "conversation_id")); ok && freeStateLoopActive(loop) && loop.AuthorityMode != "" && experimentAuthorityMode(mode) != loop.AuthorityMode {
		return ctx, fmt.Errorf("authority mode is immutable for the active Turn")
	}
	s.mu.Lock()
	s.authorityMode = mode
	s.mu.Unlock()
	return mergeContext(ctx, map[string]any{"authority_mode": mode, "authority_mode_explicit": true}), nil
}

func (s *Server) validateInvokeAuthority(ctx map[string]any) error {
	if ctx == nil || !boolValue(ctx["authority_mode_explicit"]) {
		return nil
	}
	requested, err := normalizeAuthorityMode(firstStringFromMap(ctx, "authority_mode", "permission_mode"))
	if err != nil {
		return err
	}
	if requested == authorityModeFull && s.authorityModeSnapshot() != authorityModeFull {
		return fmt.Errorf("full_project_access must be selected through the authority control before invoking an action")
	}
	return nil
}

func (s *Server) authorityModeChangeBlocked() (agentruntime.Goal, bool) {
	if s == nil || s.harness == nil {
		return agentruntime.Goal{}, false
	}
	goal := s.harness.RuntimeStatus("")
	switch goal.Status {
	case agentruntime.StatusIdle, agentruntime.StatusCompleted, agentruntime.StatusFailed, agentruntime.StatusCancelled, agentruntime.StatusStopped, agentruntime.StatusStable:
		return goal, false
	default:
		return goal, true
	}
}

func (s *Server) handleAuthorityMode(w http.ResponseWriter, r *http.Request) {
	if s == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "error": "server unavailable"})
		return
	}
	s.activateCurrentProjectWorkspace(r.Context())
	defer s.syncCurrentProjectWorkspace(r.Context())
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "authority_mode": s.authorityModeSnapshot()})
	case http.MethodPost:
		var req struct {
			AuthorityMode string `json:"authority_mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
			return
		}
		mode, err := normalizeAuthorityMode(req.AuthorityMode)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		if goal, blocked := s.authorityModeChangeBlocked(); blocked {
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error_code": "authority_mode_change_blocked", "error": "authority mode cannot change while an Agent Turn is running", "goal_id": goal.GoalID, "goal_status": goal.Status})
			return
		}
		s.mu.Lock()
		s.authorityMode = mode
		s.mu.Unlock()
		s.persistCurrentProjectWorkspace()
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "authority_mode": mode})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET or POST required"})
	}
}

func (s *Server) requestFreeStateTurnStop(conversationID, reason string) {
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return
	}
	loop.Status = "stopping"
	loop.LastError = reason
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
}

func (s *Server) stableStopCheckpoint(ctx context.Context, goalID, runID string) string {
	if s == nil || s.harness == nil {
		return ""
	}
	response, err := s.harness.Invoke(ctx, harness.InvokeRequest{Command: map[string]any{"cmd": "version_checkpoint", "message": "Stop Turn stable checkpoint", "source": "stop_turn", "checkpoint_kind": "manual"}, Source: "stop_turn", Confirmed: true, GoalID: goalID, RunID: runID})
	if err != nil || strings.EqualFold(response.Status, "error") {
		return ""
	}
	return firstNonEmpty(firstStringFromMap(response.Result, "commit_id", "checkpoint_ref"), firstStringFromMap(response.ProjectHistory, "head", "commit_id"))
}

func (s *Server) emitTurnStoppedTrajectory(conversationID, goalID, runID, reason, checkpoint string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	for _, event := range s.agentEventsSinceForControl(conversationID) {
		if event.Type == string(trajectory.EventTurnStopped) && event.RunID == runID {
			return
		}
	}
	event := trajectory.Event{Type: trajectory.EventTurnStopped, ConversationID: conversationID, GoalID: goalID, RunID: runID, ItemID: "turn-stopped:" + firstNonEmpty(runID, goalID), Title: "Turn stopped", Body: "Agent Turn stopped", Payload: trajectory.Payload{SchemaVersion: trajectory.SchemaVersion, TurnID: firstNonEmpty(runID, goalID), NodeKind: trajectory.NodeTurn, Status: trajectory.StatusStopped, Phase: "stopped", Summary: reason, CheckpointRef: checkpoint, Details: map[string]any{"reason": reason, "checkpoint_ref": checkpoint}}}
	if _, err := s.emitTrajectoryEvent(conversationID, event); err != nil && s.logger != nil {
		s.logger.Warn("[turn.stop] trajectory event rejected: %v", err)
	}
}

func (s *Server) agentEventsSinceForControl(conversationID string) []AgentEvent {
	rows, _ := s.agentEventsSince(conversationID, 0, 500)
	return rows
}

func (s *Server) finalizeStoppedTurn(ctx context.Context, conversationID, goalID, runID, reason string) (agentruntime.Goal, string) {
	previousGoal := s.harness.RuntimeStatus(goalID)
	checkpoint := previousGoal.LastCheckpoint
	if previousGoal.Status != agentruntime.StatusStopped || checkpoint == "" {
		checkpoint = s.stableStopCheckpoint(ctx, goalID, runID)
	}
	goal := s.harness.MarkGoalStopped(goalID, checkpoint)
	if loop, ok := s.freeStateLoop(conversationID); ok {
		if loop.Experiment != nil && checkpoint != "" {
			loop.Experiment.Admission.CheckpointRef = checkpoint
		}
		if loop.Experiment != nil && loop.Experiment.Status != experiment.StatusStopped && loop.Experiment.Status != experiment.StatusSettled {
			s.stopFreeStateExperiment(&loop, reason)
		}
		loop.Status = "stopped"
		loop.LastError = reason
		loop.RequiresPostActionObservation = false
		loop.UpdatedAt = time.Now().UTC()
		s.storeFreeStateLoop(loop)
	}
	s.clearGoalContinuation(goalID)
	s.emitTurnStoppedTrajectory(conversationID, goalID, runID, reason, checkpoint)
	s.persistCurrentProjectWorkspace()
	return goal, checkpoint
}

func activeProjectPlaneCommand(commandName string) bool {
	switch strings.ToLower(strings.TrimSpace(commandName)) {
	case "version_checkout", "version.checkout", "version_node_checkout", "version.node_checkout", "version_worktree_checkout", "version.worktree_checkout", "version_restore", "version.restore":
		return true
	default:
		return false
	}
}

func (s *Server) checkoutGuard(req harness.InvokeRequest) (map[string]any, bool) {
	if s == nil || s.harness == nil {
		return nil, false
	}
	commandName := req.Tool
	if value, ok := req.Command["cmd"].(string); ok && strings.TrimSpace(value) != "" {
		commandName = value
	}
	if !activeProjectPlaneCommand(commandName) {
		return nil, false
	}
	goal, blocked := s.harness.CheckoutBlocked()
	if !blocked {
		return nil, false
	}
	return map[string]any{"status": "error", "error_code": "checkout_blocked_while_agent_running", "error": "stop_turn_before_checkout", "goal_id": goal.GoalID, "goal_status": goal.Status}, true
}

func (s *Server) handleTurnStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
		return
	}
	s.activateCurrentProjectWorkspace(r.Context())
	defer s.syncCurrentProjectWorkspace(r.Context())
	var req struct {
		ConversationID string `json:"conversation_id"`
		GoalID         string `json:"goal_id"`
		RunID          string `json:"run_id"`
		TurnID         string `json:"turn_id"`
		Reason         string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid JSON: " + err.Error()})
		return
	}
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	if strings.TrimSpace(req.GoalID) == "" && req.ConversationID != "" {
		req.GoalID = s.conversationGoalID(req.ConversationID)
	}
	goal := s.harness.RuntimeStatus(req.GoalID)
	if goal.GoalID == "" || goal.Status == agentruntime.StatusIdle {
		writeJSON(w, http.StatusNotFound, map[string]any{"status": "error", "error": "active goal not found"})
		return
	}
	reason := firstNonEmpty(req.Reason, "user_stop")
	wasActivelyExecuting := agentruntime.IsActiveStatus(goal.Status)
	goal = s.harness.RequestGoalStop(goal.GoalID, reason)
	s.requestFreeStateTurnStop(req.ConversationID, reason)
	checkpoint := ""
	if !wasActivelyExecuting {
		goal, checkpoint = s.finalizeStoppedTurn(r.Context(), req.ConversationID, goal.GoalID, goal.RunID, reason)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "goal": goal, "goal_status": goal.Status, "conversation_id": req.ConversationID, "turn_id": req.TurnID, "checkpoint_ref": checkpoint})
}
