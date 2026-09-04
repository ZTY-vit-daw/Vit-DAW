package chat

import (
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	agentruntime "vit-daw-agent/internal/runtime"
	"vit-daw-agent/internal/taskstate"
)

// chatTurnEndsGoalOwnership mirrors the turn-completion goal-status switch:
// every response that ends the goal's lifecycle (completed, failed, stopped,
// cancelled, or an error the switch turns into a failed completion) leaves
// the turn's task without an owner, while parked waiting_* and still-running
// turns keep theirs.
func chatTurnEndsGoalOwnership(resp ChatResponse) bool {
	if resp.NeedsConfirmation {
		return false
	}
	switch strings.TrimSpace(resp.GoalStatus) {
	case string(agentruntime.StatusWaitingConfirmation),
		string(agentruntime.StatusWaitingClarification),
		string(agentruntime.StatusWaitingContinue),
		string(agentruntime.StatusRunning):
		return false
	}
	return true
}

// closeOrphanTaskAtTurnEnd is the turn-boundary owner for chat-style closings
// (AGENT-F2). A conversational turn can admit a task contract through the
// semantic entry observation route and then end with a plain reply that arms
// no continuation; without this guard nothing would ever advance that task,
// so its non-terminal observation state renders as perpetual progress. The
// guard applies one honest terminal transition (owner_turn_closed) and then
// settles the conversation's active audio closure so the next turn cannot
// collide with a terminal task underneath a non-terminal closure. Anything
// that still owns a next semantic move — a live durable continuation
// (pending/claimed/running/waiting_interaction), an armed goal continuation
// slot, or an active free-state loop bound to this goal — spares the task.
func (s *Server) closeOrphanTaskAtTurnEnd(conversationID, goalID string) {
	if s == nil || s.harness == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task == nil || goal.Task.Contract == nil || goal.Task.SemanticState == nil || goal.Task.SemanticState.Terminal {
		return
	}
	if s.taskHasLiveContinuationOwner(conversationID, goal) {
		return
	}
	current := goal.Task.SemanticState
	if _, err := s.transitionTaskSemantic(goalID, taskstate.TransitionRequest{
		Event:           taskstate.EventOwnerTurnClosed,
		Reason:          "chat turn closed without governed experiment",
		Summary:         firstNonEmpty(goal.Summary, goal.Task.OriginalIntent),
		ProjectRevision: current.ProjectRevision,
	}); err != nil {
		if s.logger != nil {
			s.logger.Warn("[turn.close] orphan task close rejected goal=%s state=%s: %v", goalID, current.State, err)
		}
		return
	}
	if s.logger != nil {
		s.logger.Info("[turn.close] closed orphan task state=%s goal=%s", current.State, goalID)
	}
	s.settleClosureAfterOrphanTaskClose(conversationID, goalID, goal.Task.TaskID)
}

// settleGoalAfterContinuationEnd is the continuation-completion sibling of
// the turn-boundary guard (AGENT-F3). A scheduler-driven slice ends outside
// the HTTP turn boundary, so the F2 turn guard never runs: when the chain
// stops without arming a next slice or parking an answerable interaction, a
// non-terminal observation task and its waiting/running goal would render as
// perpetual progress. The guard reuses the F2 orphan close and then settles
// the goal honestly — completed, because the chain itself decided to stop.
func (s *Server) settleGoalAfterContinuationEnd(conversationID, goalID string) {
	if s == nil || s.harness == nil || strings.TrimSpace(goalID) == "" {
		return
	}
	goal := s.harness.RuntimeStatus(goalID)
	if goal.Task == nil {
		return
	}
	if s.taskHasLiveContinuationOwner(conversationID, goal) {
		return
	}
	s.closeOrphanTaskAtTurnEnd(conversationID, goalID)
	goal = s.harness.RuntimeStatus(goalID)
	switch goal.Status {
	case agentruntime.StatusRunning, agentruntime.StatusWaitingContinue:
		s.harness.SetGoalStatus(goalID, agentruntime.StatusCompleted, nil)
	}
}

// taskHasLiveContinuationOwner reports whether any runtime still owes this
// task a next semantic move. The spare set intentionally mirrors
// goalContinuationForConversation's notion of a live continuation so the
// cross-turn free-state flow (continuation pending or parked at an
// interaction between rounds) is never mistaken for an orphan.
func (s *Server) taskHasLiveContinuationOwner(conversationID string, goal agentruntime.Goal) bool {
	if s == nil || goal.Task == nil {
		return false
	}
	s.mu.Lock()
	if _, armed := s.goalContinuations[goal.GoalID]; armed {
		s.mu.Unlock()
		return true
	}
	liveContinuation := false
	for _, item := range s.durableContinuations {
		if item.Status != ContinuationPending && item.Status != ContinuationClaimed &&
			item.Status != ContinuationRunning && item.Status != ContinuationWaitingInteraction {
			continue
		}
		if (goal.GoalID != "" && item.GoalID == goal.GoalID) ||
			(goal.Task.TaskID != "" && item.TaskID == goal.Task.TaskID) ||
			(goal.RunID != "" && item.RunID == goal.RunID) {
			liveContinuation = true
			break
		}
	}
	s.mu.Unlock()
	if liveContinuation {
		return true
	}
	if loop, ok := s.freeStateLoop(conversationID); ok && freeStateLoopActive(loop) && loop.GoalID == goal.GoalID {
		return true
	}
	return false
}

// settleClosureAfterOrphanTaskClose settles the conversation's active closure
// once its task reached the closed terminal state, mirroring what restart
// recovery does for a terminal task under a non-terminal closure
// (reconcileRestoredTaskSemanticProjectionsLocked): project the canonical
// state, settle with the matching stop reason, and release the controller
// owner. A closure bound to a different goal/task is left untouched.
func (s *Server) settleClosureAfterOrphanTaskClose(conversationID, goalID, taskID string) {
	if s == nil || s.audioClosures == nil {
		return
	}
	state, ok := s.audioClosures.ActiveForConversation(conversationID)
	if !ok || state.Terminal() {
		return
	}
	if (goalID != "" && state.GoalID != goalID) || (taskID != "" && state.TaskID != taskID) {
		return
	}
	projected, err := s.projectAudioClosureTaskState(state)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[turn.close] closure task projection failed for %s: %v", state.ClosureID, err)
		}
		return
	}
	settled := audioClosureSettleFromResult(audioclosure.Driver{}, projected, agentloop.Result{
		Reply: "chat turn closed without governed experiment",
	})
	if !settled.Terminal() {
		return
	}
	if err := s.audioClosures.Save(settled, state.Revision); err != nil {
		if s.logger != nil {
			s.logger.Warn("[turn.close] closure settle save failed for %s: %v", state.ClosureID, err)
		}
		return
	}
	s.settleAudioClosureOwner(settled)
	s.persistCurrentProjectWorkspace()
}
