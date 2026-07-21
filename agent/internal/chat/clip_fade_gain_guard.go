package chat

import (
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

func chatClipFadeGainRequest(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	hasClipTarget := textHasAny(text,
		"clip", "clips", "selected clip", "current clip", "this clip", "audio clip",
		"\u7247\u6bb5", "\u97f3\u9891\u7247\u6bb5", "\u5f53\u524d\u7247\u6bb5", "\u9009\u4e2d\u7247\u6bb5",
		"\u5f53\u524d\u9009\u4e2d clip", "\u5f53\u524d clip", "\u9009\u4e2d clip", "\u8fd9\u4e2a clip",
	)
	if !hasClipTarget {
		return false
	}
	hasFadeOrGain := textHasAny(text,
		"fade", "fade in", "fade out", "clip gain", "gain",
		"\u6de1\u5165", "\u6de1\u51fa", "\u6de1\u5316", "\u589e\u76ca",
	)
	if !hasFadeOrGain {
		return false
	}
	if textHasAny(text, "fade", "gain", "clip gain", "fade/gain") && textHasAny(text, "clip", "audio clip") {
		return true
	}
	return textHasAny(text,
		"read", "show", "inspect", "status", "state", "get", "set", "adjust", "change", "drag",
		"\u8bfb\u53d6", "\u67e5\u770b", "\u770b\u4e00\u4e0b", "\u72b6\u6001", "\u8bbe\u7f6e", "\u8c03\u6574", "\u4fee\u6539", "\u62d6", "\u62c9",
	)
}

func (s *Server) clearPendingMixForClipFadeGainRequest(conversationID, message string) bool {
	if s == nil || strings.TrimSpace(conversationID) == "" || !chatClipFadeGainRequest(message) {
		return false
	}
	hadPending := false
	if plan, ok := s.pendingPlanForChat(conversationID, nil); ok && pendingPlanCanBeRevisedByClipFadeGainRequest(plan) {
		s.expirePendingPlan(plan.ID)
		s.expirePendingInteractionsForPlan(plan.ID)
		if goalID, _ := goalIDsFromContext(plan.Context); goalID != "" {
			s.clearGoalContinuation(goalID)
		}
		hadPending = true
	}
	if _, ok := s.pendingMixTreatmentForConversation(conversationID); ok {
		s.transitionActivePendingCandidate(conversationID, "mix_treatment", agentprotocol.PendingStatusRejected, "clip fade/gain request replaced pending mix treatment")
		s.expirePendingMixTreatment(conversationID)
		s.expirePendingInteractionsForConversation(conversationID, "mix_treatment")
		hadPending = true
	}
	if _, ok := s.pendingMixTickForConversation(conversationID); ok {
		s.transitionActivePendingCandidate(conversationID, "mix_tick", agentprotocol.PendingStatusRejected, "clip fade/gain request replaced pending mix tick")
		s.expirePendingMixTick(conversationID)
		s.expirePendingInteractionsForConversation(conversationID, "mix_tick")
		hadPending = true
	}
	if hadPending && s.logger != nil {
		s.logger.Info("[clip.fade_gain.guard] expired pending mix state conversation=%s message=%q", conversationID, message)
	}
	return hadPending
}

func pendingPlanCanBeRevisedByClipFadeGainRequest(plan PendingPlan) bool {
	if pendingPlanHasClipFadeGainWriteAction(plan) {
		return true
	}
	return pendingPlanIsMixConfirmation(plan)
}

func pendingPlanHasClipFadeGainWriteAction(plan PendingPlan) bool {
	if plan.GoalContinuation != nil && pendingPlanToolIsClipFadeGainWrite(plan.GoalContinuation.PendingToolCall) {
		return true
	}
	for _, decision := range plan.Decisions {
		if decisionIsClipFadeGainWrite(decision) {
			return true
		}
	}
	return false
}

func (s *Server) expirePendingInteractionsForPlan(planID string) int {
	if s == nil || strings.TrimSpace(planID) == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, interaction := range s.interactions {
		if strings.TrimSpace(interaction.PlanID) != strings.TrimSpace(planID) {
			continue
		}
		delete(s.interactions, id)
		removed++
	}
	return removed
}

func (s *Server) expirePendingInteractionsForConversation(conversationID, workflow string) int {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for id, interaction := range s.interactions {
		if strings.TrimSpace(interaction.ConversationID) != strings.TrimSpace(conversationID) {
			continue
		}
		if strings.TrimSpace(workflow) != "" && !strings.EqualFold(strings.TrimSpace(interaction.Workflow), strings.TrimSpace(workflow)) && !strings.EqualFold(strings.TrimSpace(interaction.Type), strings.TrimSpace(workflow+"_confirmation")) && !strings.EqualFold(strings.TrimSpace(interaction.Kind), strings.TrimSpace(workflow+"_confirmation")) {
			continue
		}
		delete(s.interactions, id)
		removed++
	}
	return removed
}
