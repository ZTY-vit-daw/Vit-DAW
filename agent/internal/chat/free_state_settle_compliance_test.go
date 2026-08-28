package chat

import (
	"context"
	"testing"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/harness"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D2-1.5-S2e: while an applied experiment round is pending settlement (fresh
// post-action evidence booked, no decision), the server must not accept a
// second mutation surface — deterministic pending mix ticks, their interaction
// storage, or re-confirmed expired ticks — and must not project the goal
// completed when the loop ends with the settlement still open.

func settlementPendingLoop(conversationID string, status string) freeStateReasoningLoop {
	now := time.Now().UTC()
	return freeStateReasoningLoop{
		SchemaVersion:  freeStateReasoningLoopSchema,
		LoopID:         "loop-s2e",
		ConversationID: conversationID,
		GoalID:         "goal-s2e",
		RunID:          "run-s2e",
		Status:         status,
		DecisionPhase:  freeStatePhasePostActionEvaluation,
		OriginalIntent: "improve the vocal boxiness",
		Experiment: &experiment.Turn{
			SchemaVersion:  "experiment.turn.v1",
			ID:             "exp-s2e",
			ConversationID: conversationID,
			Status:         experiment.StatusRunning,
			CurrentRoundID: "r_s2e",
			Rounds: []experiment.Round{{
				ID: "r_s2e", Number: 1, Status: experiment.RoundObserving,
				StartedAt: now, UpdatedAt: now,
				Observations: []experiment.Observation{{
					ID: "obs-post", PostAction: true, Fresh: true, ProjectRevision: "3", RecordedAt: now,
				}},
			}},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func settlementPendingMixTickCandidate() agentloop.PendingMixTickCandidate {
	return agentloop.PendingMixTickCandidate{
		Operation: "track_gain_adjust", TrackID: "1027", DeltaDB: -1.5,
		ObservationID: "obs-post", Status: "pending_confirmation",
	}
}

func TestFreeStateLoopRoundPendingSettlementPredicate(t *testing.T) {
	loop := settlementPendingLoop("conversation-s2e", "re_evaluating")
	if !freeStateLoopRoundPendingSettlement(loop) {
		t.Fatal("applied round with fresh post-action evidence and no decision must report pending settlement")
	}
	round, err := loop.Experiment.CurrentRound()
	if err != nil {
		t.Fatalf("fixture round missing: %v", err)
	}
	round.Decision = experiment.DecisionUserJudgment
	loop.Experiment.Rounds[0] = round
	if freeStateLoopRoundPendingSettlement(loop) {
		t.Fatal("decided round must not report pending settlement")
	}
	noFreshObservation := settlementPendingLoop("conversation-s2e", "re_evaluating")
	noFreshObservation.Experiment.Rounds[0].Observations = nil
	if freeStateLoopRoundPendingSettlement(noFreshObservation) {
		t.Fatal("round without fresh post-action evidence must not report pending settlement")
	}
}

func TestRecordGoalResultRefusesPendingMixTickDuringSettlement(t *testing.T) {
	s := testContinuationServer()
	s.storeFreeStateLoop(settlementPendingLoop("conversation-s2e", "re_evaluating"))
	res := agentloop.Result{
		GoalID: "goal-s2e", RunID: "run-s2e", Status: agentruntime.StatusCompleted,
		StopReason: agentloop.StopReasonDone,
		ExecutionMemory: agentloop.ExecutionMemory{
			PendingMixTickCandidate: ptrSettlementCandidate(),
		},
	}
	resp := s.chatResponseFromAgentLoopResult("conversation-s2e", "default", res)
	if _, ok := s.pendingMixTickForConversation("conversation-s2e"); ok {
		t.Fatal("pending mix tick was stored during settlement")
	}
	if len(resp.InteractionRequests) != 0 || resp.NeedsConfirmation {
		t.Fatalf("mix tick interaction surfaced during settlement: %+v", resp.InteractionRequests)
	}
	if resp.Workflow == "mix_tick" || resp.GoalStatus == string(agentruntime.StatusWaitingConfirmation) {
		t.Fatalf("response was overridden to a mix tick confirmation: workflow=%q status=%q", resp.Workflow, resp.GoalStatus)
	}
}

func TestHandlePendingMixTickChatRefusesConfirmationDuringSettlement(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	s.mu.Lock()
	if s.pendingMixTicks == nil {
		s.pendingMixTicks = map[string]agentloop.PendingMixTickCandidate{}
	}
	s.pendingMixTicks["conversation-s2e"] = settlementPendingMixTickCandidate()
	s.mu.Unlock()
	s.storeFreeStateLoop(settlementPendingLoop("conversation-s2e", "re_evaluating"))
	resp, handled := s.handlePendingMixTickChat(context.Background(), "conversation-s2e", ChatRequest{
		ConversationID: "conversation-s2e", Message: "可以执行",
	}, agentModeDefault)
	if !handled {
		t.Fatal("explicit confirmation during settlement must be answered, not skipped")
	}
	if resp.StopReason != "d1_settlement_pending_mix_tick_refused" || resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("unexpected refusal response: status=%q stop=%q", resp.GoalStatus, resp.StopReason)
	}
	if _, ok := s.pendingMixTickForConversation("conversation-s2e"); ok {
		t.Fatal("refused confirmation left the pending mix tick installed")
	}
}

// The inactive-loop projection must not force-settle the closure nor project
// the goal completed while the round still owes its settle report: the
// settlement is model-owned, and a 194ms "completed" permanently loses it
// (2026-08-28 130901 smoke).
func TestInactiveLoopProjectionKeepsSettlementPendingGoalOpen(t *testing.T) {
	s := &Server{harness: harness.New(nil, nil, nil)}
	loop := settlementPendingLoop("conversation-s2e", "capability_blocked")
	s.storeFreeStateLoop(loop)
	req := ChatRequest{
		ConversationID: "conversation-s2e",
		Message:        loop.OriginalIntent,
		Context: map[string]any{
			"free_state_internal_resume": true,
			"conversation_id":            "conversation-s2e",
			"free_state_reasoning_loop":  freeStateLoopMap(loop),
		},
	}
	resp, handled := s.runAgentLoopChat(context.Background(), "conversation-s2e", req, config.EngineConfig{})
	if !handled {
		t.Fatal("inactive-loop projection was not handled")
	}
	if resp.GoalStatus != string(agentruntime.StatusWaitingContinue) {
		t.Fatalf("unsettled round projected as %q, want waiting_continue", resp.GoalStatus)
	}
	if resp.StopReason != "d1_settlement_pending_loop_inactive" {
		t.Fatalf("unsettled round projected with stop=%q", resp.StopReason)
	}
}

func ptrSettlementCandidate() *agentloop.PendingMixTickCandidate {
	candidate := settlementPendingMixTickCandidate()
	return &candidate
}
