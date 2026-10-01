package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
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
	s.authorityModeExplicit = true
	s.mu.Unlock()
	return mergeContext(ctx, map[string]any{"authority_mode": mode, "authority_mode_explicit": true}), nil
}

func (s *Server) validateInvokeAuthority(ctx map[string]any) error {
	_, err := s.resolvedInvokeAuthority(ctx)
	return err
}

// resolvedInvokeAuthority returns the authority mode one invoke is entitled to
// run under. When the caller asserts a mode, that assertion must agree with the
// server's own authority state; a full-access claim that the user never granted
// through the authority control is rejected here rather than downstream.
func (s *Server) resolvedInvokeAuthority(ctx map[string]any) (string, error) {
	if ctx == nil || !boolValue(ctx["authority_mode_explicit"]) {
		return "", nil
	}
	requested, err := normalizeAuthorityMode(firstStringFromMap(ctx, "authority_mode", "permission_mode"))
	if err != nil {
		return "", err
	}
	if requested == authorityModeFull && s.authorityModeSnapshot() != authorityModeFull {
		return "", fmt.Errorf("full_project_access must be selected through the authority control before invoking an action")
	}
	return requested, nil
}

// stampInvokeAuthorityMode binds the server's own authority state onto the
// harness context of one invoke.
//
// The authority control is server-owned: /agent/authority is the only place a
// mode is granted, and no request body can grant one. Execution-time gates that
// must know whether the user granted autonomous execution (the PCA load gate is
// the strictest of them) therefore read the mode from here instead of asking
// every transport caller to restate a permission it does not own. A caller that
// does assert a mode keeps its own value: that assertion was validated by
// validateInvokeAuthority before this runs, so it can only agree or be absent.
func (s *Server) stampInvokeAuthorityMode(req *harness.InvokeRequest) {
	if s == nil || req == nil {
		return
	}
	if strings.TrimSpace(firstStringFromMap(req.Context, "authority_mode", "permission_mode")) != "" {
		return
	}
	mode := s.authorityModeSnapshot()
	if strings.TrimSpace(mode) == "" {
		return
	}
	req.Context = mergeContext(req.Context, map[string]any{"authority_mode": mode, "authority_mode_explicit": true})
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
		// AUTHORITY-LOST-1: the switch owns the authoritative in-memory
		// runtime state between the write and the persist. The continuation
		// scheduler's disk reload must not replay a stale snapshot over that
		// window, so this request counts as a continuation-sensitive
		// invocation for its whole duration (same posture as chat/turn stop).
		defer s.beginContinuationSensitiveInvocation()()
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
			if s.logger != nil {
				s.logger.Warn("[authority] switch rejected mode=%s goal=%s status=%s", mode, goal.GoalID, goal.Status)
			}
			writeJSON(w, http.StatusConflict, map[string]any{"status": "error", "error_code": "authority_mode_change_blocked", "error": "authority mode cannot change while an Agent Turn is running", "goal_id": goal.GoalID, "goal_status": goal.Status})
			return
		}
		s.mu.Lock()
		s.authorityMode = mode
		s.authorityModeExplicit = true
		workspaceReady := s.activeWorkspaceUUID != ""
		s.mu.Unlock()
		persistErr := s.persistCurrentProjectWorkspaceChecked()
		if s.logger != nil {
			if persistErr != nil {
				s.logger.Warn("[authority] switch accepted mode=%s workspace_ready=%t persist_failed error=%v (kept in memory; restore preserves it, next persist writes it)", mode, workspaceReady, persistErr)
			} else {
				s.logger.Info("[authority] switch accepted mode=%s workspace_ready=%t", mode, workspaceReady)
			}
		}
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
	// FS-STOP-APPLY-1: MarkGoalStopped must land AFTER stopFreeStateExperiment.
	// The experiment stop's canonical task transition (EventTaskCancelled)
	// converges the goal status through TransitionTask's StateCancelled branch,
	// which would overwrite a stopped goal back to cancelled (2026-10-01 M1
	// live shape: task contract present). Stopping the experiment first keeps
	// the user's stop the final, truthful goal status.
	goal := s.harness.MarkGoalStopped(goalID, checkpoint)
	s.clearGoalContinuation(goalID)
	s.settleStoppedTurnClosure(conversationID, goalID, reason)
	s.emitTurnStoppedTrajectory(conversationID, goalID, runID, reason, checkpoint)
	s.persistCurrentProjectWorkspace()
	return goal, checkpoint
}

// goalStopPending reports whether a user stop is pending for the goal: the
// latch RequestGoalStop set (StopRequested) or the stopped status itself. This
// is the queryable face between "the LLM decision returned" and "the
// intervention executes" — the improvement-proposal application path runs
// after the message loop returned, outside the runner's before/after-tool
// checkpoints, so the pending stop must be re-read there (FS-STOP-APPLY-1
// Part A).
func (s *Server) goalStopPending(goalID string) bool {
	if s == nil || s.harness == nil {
		return false
	}
	goal := s.harness.RuntimeStatus(goalID)
	return goal.StopRequested || goal.Status == agentruntime.StatusStopped
}

// stopPendingProposalSkippedReason is the honest stop reason for a turn whose
// returned improvement proposal was never applied because the user stop was
// already pending (FS-STOP-APPLY-1 Part A).
const stopPendingProposalSkippedReason = "user_stop_pending_intervention_skipped"

// stopPendingExperimentProposalResponse applies the FS-STOP-APPLY-1 Part A
// boundary: the turn's model decision returned with a bounded improvement
// proposal while the user's stop request is pending. The proposal is NOT
// applied — already-applied work keeps its bounded reversible semantics, but
// no new intervention fires after the stop. The stop lands now
// (finalizeStoppedTurn, including the Part B closure settlement and ownership
// release) instead of one continuation later, and the response states the
// skip honestly.
func (s *Server) stopPendingExperimentProposalResponse(ctx context.Context, conversationID, mode string, res agentloop.Result) (ChatResponse, bool) {
	if !s.goalStopPending(res.GoalID) {
		return ChatResponse{}, false
	}
	if s.logger != nil {
		s.logger.Info("[turn.stop] pending stop skipped a returned improvement proposal conversation=%s goal=%s run=%s",
			conversationID, res.GoalID, res.RunID)
	}
	goal, checkpoint := s.finalizeStoppedTurn(ctx, conversationID, res.GoalID, res.RunID, "user_stop")
	resp := s.chatResponseFromAgentLoopResult(conversationID, mode, res)
	resp.GoalStatus = string(agentruntime.StatusStopped)
	resp.StopReason = stopPendingProposalSkippedReason
	resp.Reply = "已停止当前 Turn：停止请求之后返回的改善提案没有应用，本轮没有新的干预；已应用的调整保持可回滚。"
	resp.NeedsConfirmation = false
	resp.InteractionRequests = nil
	resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
		"stop_turn": true, "checkpoint_ref": checkpoint, "goal_status": goal.Status,
		"intervention_skipped": true, "skip_reason": "user_stop_pending",
		"mutation_performed": false,
	})
	return resp, true
}

// settleStoppedTurnClosure is the FS-STOP-APPLY-1 Part B ownership release:
// a user-stopped turn must not wedge the conversation's audio closure. When
// the experiment is stopped and the closure bound to this goal is still
// non-terminal, the task semantic state takes the legal close migration
// (EventOwnerTurnClosed from any non-terminal state — the user stop is the
// owning authority), the closure settles with the honest mapped stop reason
// (owner_turn_closed / cancelled, never a fabricated settle-success), and the
// controller owner is released so the next goal can acquire the conversation
// (2026-10-01 18:35 live wedge: goal_6b07c186 failed with
// audio_closure_controller_failure "conversation is already owned by
// minimal_audio_closure controller audio_closure_997396f72e46902c").
func (s *Server) settleStoppedTurnClosure(conversationID, goalID, reason string) {
	if s == nil || s.harness == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task != nil && goal.Task.Contract != nil && goal.Task.SemanticState != nil && !goal.Task.SemanticState.Terminal {
		if _, err := s.transitionTaskSemantic(goalID, taskstate.TransitionRequest{
			Event:   taskstate.EventOwnerTurnClosed,
			Reason:  "user stopped the turn",
			Summary: firstNonEmpty(reason, goal.Summary, goal.Task.OriginalIntent),
			ProjectRevision: firstNonEmpty(goal.Task.SemanticState.ProjectRevision,
				goal.Task.Contract.ProjectRevision),
		}); err != nil {
			if s.logger != nil {
				s.logger.Warn("[turn.stop] stopped-turn task close rejected goal=%s state=%s: %v",
					goalID, goal.Task.SemanticState.State, err)
			}
		}
	}
	if s.audioClosures == nil {
		return
	}
	state, ok := s.audioClosures.ActiveForConversation(conversationID)
	if !ok || state.Terminal() || state.GoalID != goalID {
		return
	}
	projected := state
	if state.ContractID != "" {
		if next, err := s.projectAudioClosureTaskState(state); err == nil {
			projected = next
		} else if s.logger != nil {
			s.logger.Warn("[turn.stop] closure task projection failed for %s: %v", state.ClosureID, err)
		}
	}
	summary := "user stopped the turn"
	settled := audioClosureSettleFromResult(audioclosure.Driver{}, projected, agentloop.Result{Reply: summary})
	if !settled.Terminal() {
		settled, _ = (audioclosure.Driver{}).Settle(projected, projected.Revision, audioclosure.StopCancelled, summary, false, time.Now().UTC())
	}
	if !settled.Terminal() {
		if s.logger != nil {
			s.logger.Warn("[turn.stop] stopped-turn closure settle did not reach terminal for %s phase=%s", state.ClosureID, projected.Phase)
		}
		return
	}
	if err := s.audioClosures.Save(settled, state.Revision); err != nil {
		if s.logger != nil {
			s.logger.Warn("[turn.stop] stopped-turn closure settle save failed for %s: %v", state.ClosureID, err)
		}
		return
	}
	s.settleAudioClosureOwner(settled)
	if s.logger != nil {
		s.logger.Info("[turn.stop] settled stopped-turn closure=%s reason=%s goal=%s", state.ClosureID, settled.Settlement.Reason, goalID)
	}
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
	defer s.beginContinuationSensitiveInvocation()()
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
	// FS-STOP-APPLY-1 anchor: the stop request arrival used to leave no log
	// trace, so the press moment could only be established from user testimony
	// (2026-10-01 M1 retest forensics). One INFO line: conversation / goal /
	// run / moment / reason / prior status / whether a runner checkpoint will
	// land the stop or finalizeStoppedTurn runs inline.
	if s.logger != nil {
		s.logger.Info("[turn.stop] request received conversation=%s goal=%s run=%s reason=%s prior_status=%s actively_executing=%t",
			req.ConversationID, goal.GoalID, goal.RunID, reason, goal.Status, wasActivelyExecuting)
	}
	goal = s.harness.RequestGoalStop(goal.GoalID, reason)
	s.requestFreeStateTurnStop(req.ConversationID, reason)
	checkpoint := ""
	if !wasActivelyExecuting {
		goal, checkpoint = s.finalizeStoppedTurn(r.Context(), req.ConversationID, goal.GoalID, goal.RunID, reason)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "goal": goal, "goal_status": goal.Status, "conversation_id": req.ConversationID, "turn_id": req.TurnID, "checkpoint_ref": checkpoint})
}
