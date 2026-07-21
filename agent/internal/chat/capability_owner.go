package chat

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

type capabilityOwnerResolution struct {
	CapabilityID string
	SessionID    string
	Decision     orchestration.OwnerDecision
	Ambiguous    bool
}

func (s *Server) resolveCapabilityOwner(conversationID string, req ChatRequest) capabilityOwnerResolution {
	explicitCapability := firstStringFromMap(req.Context, "capability_id", "capability")
	if explicitCapability != staticBalanceCapabilityID && explicitCapability != panLayoutCapabilityID && explicitCapability != lowEndRelationCapabilityID && explicitCapability != spalReferenceEQProviderRegistrationCapabilityID && explicitCapability != spalReferenceEQTestCapabilityID && explicitCapability != spalEQV2CapabilityID && explicitCapability != pluginEffectControlCapabilityID {
		explicitCapability = ""
	}
	// A recovered Proposal interaction already carries the exact durable
	// session identity.  Do not drop it and re-select by conversational
	// heuristics: that would turn an approved frozen proposal into a fresh
	// natural-language request after a restart or UI replay.
	requestedSessionID := firstStringFromMap(req.Context, "capability_session_id")
	if requestedSessionID != "" && s != nil && s.orchestrationRuntime != nil && s.orchestrationRuntime.Store != nil {
		if session, ok := s.orchestrationRuntime.Store.Load(requestedSessionID); ok &&
			session.EngineOwner == orchestration.EngineV1 && !session.Terminal() &&
			sessionBelongsToConversation(session, conversationID) &&
			(explicitCapability == "" || session.Invocation.CapabilityID == explicitCapability) {
			decision := orchestration.OwnerPolicyFromEnvironment().Select(
				session.Invocation.CapabilityID,
				true,
				false,
				&session,
			)
			return capabilityOwnerResolution{
				CapabilityID: session.Invocation.CapabilityID,
				SessionID:    session.ID,
				Decision:     decision,
			}
		}
	}
	inferred := inferProjectAwareCapability(req.Message)
	active := s.activeCapabilitySessions(conversationID)
	if explicitCapability == "" && strings.HasPrefix(strings.TrimSpace(req.Message), "/") {
		return capabilityOwnerResolution{}
	}
	capabilityID := explicitCapability
	if capabilityID == "" && inferred != "" {
		capabilityID = inferred
	}
	if capabilityID == "" && len(active) == 1 {
		capabilityID = active[0].Invocation.CapabilityID
	}
	if capabilityID == "" && len(active) > 1 && pendingPlanPlainApproval(req.Message) {
		return capabilityOwnerResolution{Ambiguous: true}
	}
	if capabilityID == "" {
		return capabilityOwnerResolution{}
	}

	var existing *orchestration.PlanningSession
	for index := range active {
		if active[index].Invocation.CapabilityID == capabilityID {
			copy := active[index]
			existing = &copy
			break
		}
	}
	decision := orchestration.OwnerPolicyFromEnvironment().Select(
		capabilityID,
		true,
		false,
		existing,
	)
	sessionID := ""
	if existing != nil {
		sessionID = existing.ID
	} else if decision.Owner == orchestration.EngineV1 {
		sessionID = s.nextCapabilitySessionID(conversationID, capabilityID)
	}
	return capabilityOwnerResolution{CapabilityID: capabilityID, SessionID: sessionID, Decision: decision}
}

func (s *Server) activeCapabilitySessions(conversationID string) []orchestration.PlanningSession {
	if s == nil || s.orchestrationRuntime == nil || s.orchestrationRuntime.Store == nil {
		return nil
	}
	out := make([]orchestration.PlanningSession, 0, 2)
	for _, session := range s.orchestrationRuntime.Store.List() {
		if session.EngineOwner != orchestration.EngineV1 || session.Terminal() || !sessionBelongsToConversation(session, conversationID) {
			continue
		}
		if session.Invocation.CapabilityID == staticBalanceCapabilityID || session.Invocation.CapabilityID == panLayoutCapabilityID || session.Invocation.CapabilityID == lowEndRelationCapabilityID || session.Invocation.CapabilityID == spalReferenceEQProviderRegistrationCapabilityID || session.Invocation.CapabilityID == spalReferenceEQTestCapabilityID || session.Invocation.CapabilityID == spalEQV2CapabilityID || session.Invocation.CapabilityID == pluginEffectControlCapabilityID {
			out = append(out, session)
		}
	}
	return out
}

func (s *Server) nextCapabilitySessionID(conversationID, capabilityID string) string {
	base := capabilitySessionBase(conversationID, capabilityID)
	count := 0
	if s != nil && s.orchestrationRuntime != nil && s.orchestrationRuntime.Store != nil {
		for _, session := range s.orchestrationRuntime.Store.List() {
			if sessionBelongsToConversation(session, conversationID) && session.Invocation.CapabilityID == capabilityID {
				count++
			}
		}
	}
	return fmt.Sprintf("%s_%d", base, count+1)
}

func capabilitySessionBase(conversationID, capabilityID string) string {
	prefix := "cap_v1"
	if capabilityID == staticBalanceCapabilityID {
		prefix += "_b2"
	} else if capabilityID == panLayoutCapabilityID {
		prefix += "_b3"
	} else if capabilityID == lowEndRelationCapabilityID {
		prefix += "_b4"
	} else if capabilityID == spalReferenceEQProviderRegistrationCapabilityID {
		prefix += "_spal_provider"
	} else if capabilityID == spalReferenceEQTestCapabilityID {
		prefix += "_spal_refeq"
	} else if capabilityID == spalEQV2CapabilityID {
		prefix += "_spal_eq_v2"
	} else if capabilityID == pluginEffectControlCapabilityID {
		prefix += "_plugin_effect"
	} else {
		prefix += "_cap"
	}
	return prefix + "_" + sanitizeCanaryID(conversationID)
}

func sessionBelongsToConversation(session orchestration.PlanningSession, conversationID string) bool {
	if strings.TrimSpace(session.Invocation.ConversationID) != "" {
		return session.Invocation.ConversationID == conversationID
	}
	sanitized := sanitizeCanaryID(conversationID)
	oldV1B2 := "cap_v1_" + sanitized
	oldV1B3 := "cap_v1_b3_" + sanitized
	if session.ID == oldV1B2 || session.ID == oldV1B3 {
		return true
	}
	base := capabilitySessionBase(conversationID, session.Invocation.CapabilityID)
	return strings.HasPrefix(session.ID, base+"_")
}

func inferProjectAwareCapability(message string) string {
	text := strings.ToLower(strings.TrimSpace(message))
	if isSPALReferenceEQProviderRegistrationIntent(message, text) {
		return spalReferenceEQProviderRegistrationCapabilityID
	}
	// Ordinary EQ language belongs to AgentLoop. The Agent first reasons about
	// a task-level equalizer action, then calls the capability-layer tool. Only
	// an already active PlanningSession or an explicit capability context may
	// enter the SPAL runtime before AgentLoop.
	for _, marker := range []string{"static_mix.static_balance", "b2", "静态平衡", "静态音量", "推子平衡", "static balance"} {
		if strings.Contains(text, marker) {
			return staticBalanceCapabilityID
		}
	}
	for _, marker := range []string{"static_mix.pan_layout", "b3", "声像布局", "pan layout", "panning layout"} {
		if strings.Contains(text, marker) {
			return panLayoutCapabilityID
		}
	}
	for _, marker := range []string{"static_mix.low_end_relation", "b4", "低频关系", "low-end relation", "low end relation", "低频分析", "低频占用", "低频遮蔽"} {
		if strings.Contains(text, marker) {
			return lowEndRelationCapabilityID
		}
	}
	for _, marker := range []string{"spal reference eq", "spal reference-eq", "spal 参考eq", "spal 参考 eq", "spal reference eq 测试", "reference eq test"} {
		if strings.Contains(text, marker) {
			return spalReferenceEQTestCapabilityID
		}
	}
	return ""
}

func isSPALReferenceEQProviderRegistrationIntent(message, lower string) bool {
	if !strings.Contains(lower, "spal") {
		return false
	}
	registration := strings.Contains(lower, "register") || strings.Contains(lower, "registration") || strings.Contains(message, "注册") || strings.Contains(message, "登记")
	provider := strings.Contains(lower, "provider") || strings.Contains(lower, "reference eq") || strings.Contains(message, "提供器")
	return registration && provider
}
