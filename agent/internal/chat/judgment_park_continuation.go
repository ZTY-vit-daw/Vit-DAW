package chat

import (
	"context"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/audioclosure"
	"vit-daw-agent/internal/experiment"
	agentruntime "vit-daw-agent/internal/runtime"
)

// FS-PARK-TURNFAIL-1（用户裁定 2026-09-30，
// decisions/2026-09-30-park-adoption-and-judgment-card-ruling.md）：
// judgment park（待人耳 A/B 裁决驻留态）不强制终局；用户在 park 期间发新输入
// = 对待裁决段默认采纳——保留已应用状态、关闭该轮，新消息以全新 goal/run/turn
// 进入新一轮思考（顺带消除 run id 复用导致的 turn 生命周期翻转）。结算如实
// 标注 adopted_by_continuation：不写 UserJudgmentEvidence、不要求 sufficient
// target response，绝不呈现为 human_confirmed；人耳判断 POST 仍是显式结算通道。

// judgmentParkAdoptionSummary is the frozen settle summary (closed template:
// states the adoption semantics and the honesty boundary, no domain content).
const judgmentParkAdoptionSummary = "用户继续对话，待 A/B 裁决轮按默认采纳收口：已应用调整保留，未记录人耳 A/B 判断（adopted_by_continuation，非 human_confirmed）"

// judgmentParkPendingRound reports whether the conversation's persisted loop is
// durably parked at the human-judgment boundary with an unjudged round — the
// exact state a new user input settles by adoption. A round that already
// recorded user judgment evidence honors that explicit judgment instead: its
// settlement stays on the audition judgment POST channel.
func judgmentParkPendingRound(loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil || !freeStateJudgmentBoundary(loop) {
		return false
	}
	current, err := loop.Experiment.CurrentRound()
	if err != nil {
		return false
	}
	return len(current.UserJudgmentEvidence) == 0
}

// settleJudgmentParkOnUserContinuation applies the default-adoption ruling for
// one new user input. It returns true when a parked round was settled; the
// caller then proceeds to beginChatGoal, which starts a fresh goal because the
// parked goal is completed here. Every failure path returns false and leaves
// all durable state untouched — the message falls back to the pre-existing
// routing, never to a new failure mode.
func (s *Server) settleJudgmentParkOnUserContinuation(conversationID, message string) bool {
	if s == nil || s.harness == nil {
		return false
	}
	message = strings.TrimSpace(message)
	if message == "" || strings.HasPrefix(message, "/") || isContinueMessage(message) {
		return false
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok {
		return false
	}
	if !judgmentParkPendingRound(loop) {
		// JUDGMENT-SETTLE-STALL-1: a round that already recorded its judgment but
		// never finished settling (settle/rollback failed mid-path on the parked
		// loop) must not wedge the conversation the way the un-judged park did —
		// the recorded judgment is the authority the settlement needs, so
		// finishing it here honors the user's earlier answer instead of silently
		// re-adopting or rejecting the input.
		return s.finishJudgedParkOnUserContinuation(conversationID, loop)
	}
	now := time.Now().UTC()
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		return false
	}
	events, err := loop.Experiment.DecideRound(experiment.DecisionAdoptedByContinuation, judgmentParkAdoptionSummary, now)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-park-continuation] adoption round decision rejected: %v", err)
		}
		return false
	}
	annotateJudgmentDecision(events, "judgment.adopted_by_continuation", round.AdoptedCandidateID)
	s.emitFreeStateExperimentEvents(events)
	if s.hasTaskSemanticContract(loop.GoalID) {
		// EventTaskSettled is legal from human_judgment_required
		// (taskstate/state.go) and clears the durable pending interaction, so
		// the audition judgment card stops demanding an answer.
		if err := s.settleTaskFromExperiment(&loop, judgmentParkAdoptionSummary, freeStateExperimentEvidence(loop.Experiment)); err != nil {
			if s.logger != nil {
				s.logger.Warn("[judgment-park-continuation] canonical settlement rejected: %v", err)
			}
			return false
		}
	}
	events, err = loop.Experiment.Settle(experiment.OutcomeAdoptedByContinuation, judgmentParkAdoptionSummary, now)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-park-continuation] settlement rejected: %v", err)
		}
		return false
	}
	s.emitFreeStateExperimentEvents(events)
	loop.Status = "completed"
	loop.RequiresPostActionObservation = false
	if len(loop.AuditionSessionSnapshot) > 0 {
		// Machine-readable terminal state for the A/B judgment card
		// (AB-JUDGMENT-CARD-1 ruling 2: the card settles to a default-adopt
		// final style once the user continued the conversation).
		loop.AuditionSessionSnapshot["adoption_status"] = "adopted_by_continuation"
		loop.AuditionSessionSnapshot["judgment_skipped"] = true
	}
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	// Close the parked goal (waiting_continue → completed) so beginChatGoal
	// starts a fresh goal/run/turn for the new message — the turn-id reuse
	// lifecycle flip cannot recur for this conversation's parked round.
	s.harness.CompleteGoal(loop.GoalID, nil)
	// FS-ADOPT-CLOSURE-1 source layer: the adoption settles the experiment and
	// the canonical task, so the conversation's session-level closure must not
	// outlive them. Left active, the next goal binds it in
	// prepareAudioClosureContext and audioClosureSettleFromResult maps the
	// settled TaskState to StopTaskSettled — the 2026-10-01 20:35 live shape
	// (goal_65e6beee answered the canned settlement sentence with zero
	// observation). Settle with the honest adoption reason (never
	// satisfied/diagnostic_complete — adoption is not a judged completion) and
	// release the controller owner.
	s.settleAdoptedParkClosure(conversationID, loop.GoalID)
	if s.logger != nil {
		s.logger.Info("[judgment-park-continuation] parked round adopted by continuation conversation=%s goal=%s round=%s",
			conversationID, loop.GoalID, round.ID)
	}
	return true
}

// settleAdoptedParkClosure is the FS-ADOPT-CLOSURE-1 source-layer settlement:
// the session closure bound to the adopted goal closes with the honest
// adoption stop reason and the controller owner is released, so the next goal
// either acquires the conversation cleanly or opens its own fresh closure.
// Failures only WARN — the adoption semantics above are already durable, and
// the prepareAudioClosureContext binding guard is the defense-in-depth net.
func (s *Server) settleAdoptedParkClosure(conversationID, goalID string) {
	if s == nil || s.audioClosures == nil {
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
			s.logger.Warn("[judgment-park-continuation] closure task projection failed for %s: %v", state.ClosureID, err)
		}
	}
	settled, err := (audioclosure.Driver{}).Settle(projected, projected.Revision, audioclosure.StopAdoptedByContinuation,
		judgmentParkAdoptionSummary, false, time.Now().UTC())
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-park-continuation] adoption closure settle rejected for %s: %v", state.ClosureID, err)
		}
		return
	}
	if err := s.audioClosures.Save(settled, state.Revision); err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-park-continuation] adoption closure settle save failed for %s: %v", state.ClosureID, err)
		}
		// JUDGMENT-SETTLE-STALL-1: a failed store save must not leave the
		// controller owner wedged — the settle itself succeeded and the state in
		// hand is terminal, so the registry owner is released regardless; the
		// store row stays non-terminal as orphaned audit data the
		// prepareAudioClosureContext finished-task guard can still settle later.
		s.settleAudioClosureOwner(settled)
		return
	}
	s.settleAudioClosureOwner(settled)
	s.persistCurrentProjectWorkspace()
	if s.logger != nil {
		s.logger.Info("[judgment-park-continuation] settled adopted park closure=%s reason=%s goal=%s",
			state.ClosureID, audioclosure.StopAdoptedByContinuation, goalID)
	}
}

// finishJudgedParkOnUserContinuation closes the judged-but-unsettled park: the
// round recorded its user judgment evidence but the settlement never completed
// (settle/rollback failed mid-path on the single-round tier). The recorded
// judgment — not a default adoption — is the settlement authority, so the new
// user input drives the same deterministic outcome the judgment POST would
// have. The legacy candidate tier's awaiting_candidate_apply state is a
// DESIGNED waiting shape (the explicit apply boundary owns the settlement) and
// is excluded; every other shape returns false with state untouched.
func (s *Server) finishJudgedParkOnUserContinuation(conversationID string, loop freeStateReasoningLoop) bool {
	if loop.Experiment == nil || !freeStateJudgmentBoundary(loop) {
		return false
	}
	switch loop.Experiment.Status {
	case experiment.StatusSettled, experiment.StatusStopped:
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(loop.Status), "blocked") {
		return false
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil || len(round.UserJudgmentEvidence) == 0 {
		return false
	}
	evidence := round.UserJudgmentEvidence[len(round.UserJudgmentEvidence)-1]
	if err := s.applyFreeStateJudgmentOutcome(context.Background(), &loop, evidence); err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-park-continuation] recorded-judgment settlement rejected conversation=%s goal=%s: %v",
				conversationID, loop.GoalID, err)
		}
		return false
	}
	s.finishJudgmentSettlement(&loop, evidence)
	loop.UpdatedAt = time.Now().UTC()
	s.storeFreeStateLoop(loop)
	s.persistCurrentProjectWorkspace()
	s.harness.CompleteGoal(loop.GoalID, nil)
	if s.logger != nil {
		s.logger.Info("[judgment-park-continuation] parked round settled by recorded judgment conversation=%s goal=%s round=%s",
			conversationID, loop.GoalID, round.ID)
	}
	return true
}

// settleJudgedClosure is the judgment-settle counterpart of
// settleAdoptedParkClosure: the experiment settled through the governed
// judgment path (EventTaskSettled), so the conversation's session closure must
// not outlive it holding the controller owner — the 2026-10-02 live wedge had
// exactly that residue turn the NEXT goal's entry into "conversation is
// already owned by minimal_audio_closure". Failures only WARN; the
// prepareAudioClosureContext finished-task guard remains the net.
func (s *Server) settleJudgedClosure(conversationID, goalID, summary string) {
	if s == nil || s.audioClosures == nil {
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
			s.logger.Warn("[judgment-settle] closure task projection failed for %s: %v", state.ClosureID, err)
		}
	}
	settled := audioClosureSettleFromResult(audioclosure.Driver{}, projected, agentloop.Result{Reply: summary})
	if !settled.Terminal() {
		if s.logger != nil {
			s.logger.Warn("[judgment-settle] closure %s could not map its task state to a settle reason; leaving it to the binding guard", state.ClosureID)
		}
		return
	}
	if err := s.audioClosures.Save(settled, state.Revision); err != nil {
		if s.logger != nil {
			s.logger.Warn("[judgment-settle] closure settle save failed for %s: %v", state.ClosureID, err)
		}
		s.settleAudioClosureOwner(settled)
		return
	}
	s.settleAudioClosureOwner(settled)
	s.persistCurrentProjectWorkspace()
	if s.logger != nil {
		s.logger.Info("[judgment-settle] settled judged park closure=%s goal=%s", state.ClosureID, goalID)
	}
}

// judgmentParkPreservedReply is the frozen user-readable boundary wording for
// Part B: a turn whose model decision failed while the judgment park survives.
// The park stays answerable (explicit A/B judgment POST) and the continuation
// channel stays open (new input adopts by the Part A semantics).
const judgmentParkPreservedReply = "上一轮实验仍在等待你的 A/B 试听判断，本轮没有产生新的执行。你可以点开 A/B 卡片进行裁决；也可以直接输入新话题继续——继续对话会默认采纳已应用的调整。"

const judgmentParkPreservedStopReason = "judgment_park_preserved"

// judgmentParkTerminalFallbackFailure reports whether a failed agentloop
// result is the terminal-turn fallback family (BOUNDARY-1 honest fallback) —
// the gate-rejection/unparseable shape the ruling forbids to project as
// turn.failed while the judgment park survives.
func judgmentParkTerminalFallbackFailure(res agentloop.Result) bool {
	if res.Status != agentruntime.StatusFailed {
		return false
	}
	switch res.StopReason {
	case agentloop.FreeStateTerminalFallbackStopReason, agentloop.FreeStateTerminalGateRejectedStopReason:
		return true
	}
	return false
}

// preserveJudgmentParkOnTerminalFallback applies Part B in place: a terminal
// fallback failure while the conversation's judgment park still holds is
// rewritten from the turn.failed projection to the user-readable boundary
// (goal stays waiting_continue; the audition judgment POST channel is
// untouched; new user input settles the park by Part A adoption). It reports
// whether the response was rewritten.
func (s *Server) preserveJudgmentParkOnTerminalFallback(conversationID string, resp *ChatResponse, res agentloop.Result) bool {
	if s == nil || resp == nil || !judgmentParkTerminalFallbackFailure(res) {
		return false
	}
	loop, ok := s.freeStateLoop(conversationID)
	if !ok || !judgmentParkPendingRound(loop) {
		return false
	}
	resp.Error = ""
	resp.GoalStatus = string(agentruntime.StatusWaitingContinue)
	resp.StopReason = judgmentParkPreservedStopReason
	resp.Reply = judgmentParkPreservedReply
	resp.WorkflowData = mergeContext(resp.WorkflowData, map[string]any{
		"judgment_park_preserved": true,
	})
	return true
}
