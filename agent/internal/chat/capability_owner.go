package chat

import (
	"fmt"
	"os"
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
	if explicitCapability == retiredFocusPositionCapabilityID {
		explicitCapability = staticBalanceCapabilityID
	}
	if explicitCapability != staticBalanceCapabilityID && explicitCapability != panLayoutCapabilityID && explicitCapability != lowEndRelationCapabilityID && explicitCapability != frequencyCleanupCapabilityID && explicitCapability != dynamicControlCapabilityID && explicitCapability != agentSemanticEQCapabilityID && explicitCapability != spaceDepthCapabilityID && explicitCapability != premasterObservationCapabilityID {
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
	// A selected-track ordinary EQ command remains horizontal even when its
	// listening goal mentions low end. Only an explicit B4 capability marker
	// may move that request into the full-project specialist workflow.
	if inferred == lowEndRelationCapabilityID && explicitCapability == "" && ordinaryAgentSemanticEQMutationRequest(req.Message, req.Context) && !explicitB4InvocationMarker(req.Message) {
		inferred = ""
	}
	if inferred == frequencyCleanupCapabilityID && explicitCapability == "" && !explicitC1InvocationMarker(req.Message) && (ordinaryAgentSemanticEQMutationRequest(req.Message, req.Context) || c1SelectedTargetContext(req.Context)) {
		inferred = ""
	}
	active := s.activeCapabilitySessions(conversationID)
	if explicitCapability == "" && strings.HasPrefix(strings.TrimSpace(req.Message), "/") {
		return capabilityOwnerResolution{}
	}
	capabilityID := explicitCapability
	if capabilityID == "" && inferred != "" {
		capabilityID = inferred
	}
	if capabilityID == "" && len(active) == 1 {
		activeCapability := active[0].Invocation.CapabilityID
		stickyProposalTurn := semanticEQPendingProposalTurn(req.Message)
		if activeCapability == lowEndRelationCapabilityID || activeCapability == frequencyCleanupCapabilityID || activeCapability == dynamicControlCapabilityID {
			stickyProposalTurn = active[0].ActiveProposal != nil && stickyProposalTurn
		}
		if (activeCapability != agentSemanticEQCapabilityID && activeCapability != lowEndRelationCapabilityID && activeCapability != frequencyCleanupCapabilityID && activeCapability != dynamicControlCapabilityID) || stickyProposalTurn {
			capabilityID = active[0].Invocation.CapabilityID
		}
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

func explicitB4InvocationMarker(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(text, "static_mix.low_end_relation") || capabilityMarkerPresent(text, "b4")
}

func explicitC1InvocationMarker(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(text, "fine_mix.frequency_cleanup") || capabilityMarkerPresent(text, "c1") || strings.Contains(text, "full project") || strings.Contains(text, "whole project") || strings.Contains(text, "全工程") || strings.Contains(text, "整首")
}

// capabilityMarkerWordByte reports whether b may continue an ASCII word token
// (letters/digits/underscore — snake_case identifiers like "track_b2" are one
// token). Bytes >= 0x80 (e.g. CJK runes in the lowercased message) are word
// boundaries for the short ASCII capability codes.
func capabilityMarkerWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// capabilityShortCodeMarker reports whether the marker is a bare short ASCII
// alphanumeric capability code (the b2/b3/b4/b5/c1/c2/c3/d1/d2/d3 family).
// Dotted capability IDs ("static_mix.pan_layout") and natural-language
// phrases ("声像布局", "pan layout") are not short codes.
func capabilityShortCodeMarker(marker string) bool {
	if marker == "" {
		return false
	}
	for i := 0; i < len(marker); i++ {
		if !capabilityMarkerWordByte(marker[i]) {
			return false
		}
	}
	return true
}

// capabilityMarkerPresent reports whether marker occurs in the lowercased
// message text. Short alphanumeric capability codes only match as standalone
// tokens: an occurrence embedded inside a longer alphanumeric run (ABC
// notation "b32|Z|", code "a4b4-", hex "d1ff") does NOT claim the turn —
// only an explicit bounded code ("b3", "run C2 dynamic control", "运行 B4",
// "b2静态平衡" with CJK acting as boundary) does. GEN-CHAT-ROUTING-1 (278
// run2): full-message substring matching let the score text "b32" hijack the
// turn into the pan_layout canary. Non-code markers keep substring matching
// unchanged, so no phrase is added to or removed from any entry word list
// (the M1-3/M1-4 entry-admission rulings govern list membership, not token
// shape; the in-code comments already frame these codes as *explicit*
// markers). The check is strictly false-positive reducing: it can never make
// a previously unmatched message match.
func capabilityMarkerPresent(text, marker string) bool {
	if !capabilityShortCodeMarker(marker) {
		return strings.Contains(text, marker)
	}
	for start := 0; start < len(text); {
		idx := strings.Index(text[start:], marker)
		if idx < 0 {
			return false
		}
		i := start + idx
		j := i + len(marker)
		if (i == 0 || !capabilityMarkerWordByte(text[i-1])) && (j == len(text) || !capabilityMarkerWordByte(text[j])) {
			return true
		}
		start = j
	}
	return false
}

func c1SelectedTargetContext(context map[string]any) bool {
	return firstStringFromMap(context, "selected_track_id", "track_id", "selected_plugin_track_id", "plugin_id", "selected_plugin_id") != ""
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
		if session.Invocation.CapabilityID == staticBalanceCapabilityID || session.Invocation.CapabilityID == panLayoutCapabilityID || session.Invocation.CapabilityID == lowEndRelationCapabilityID || session.Invocation.CapabilityID == frequencyCleanupCapabilityID || session.Invocation.CapabilityID == dynamicControlCapabilityID || session.Invocation.CapabilityID == agentSemanticEQCapabilityID || session.Invocation.CapabilityID == spaceDepthCapabilityID || session.Invocation.CapabilityID == premasterObservationCapabilityID {
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
	} else if capabilityID == frequencyCleanupCapabilityID {
		prefix += "_c1"
	} else if capabilityID == dynamicControlCapabilityID {
		prefix += "_c2"
	} else if capabilityID == agentSemanticEQCapabilityID {
		prefix += "_semantic_eq"
	} else if capabilityID == spaceDepthCapabilityID {
		prefix += "_c3"
	} else if capabilityID == premasterObservationCapabilityID {
		prefix += "_d1d3"
	} else {
		prefix += "_cap"
	}
	return prefix + "_" + sanitizeCanaryID(conversationID)
}

// A pending ordinary-Agent EQ proposal must not become the owner of every
// subsequent chat message. Only an exact UI binding or a clearly proposal-
// related conversational turn is admitted here.
func semanticEQPendingProposalTurn(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	if pendingPlanPlainApproval(message) {
		return true
	}
	for _, token := range []string{
		"取消", "拒绝", "不同意", "不要执行", "为什么", "解释", "这个方案", "当前方案", "proposal",
		"修改方案", "调整方案", "重新生成", "再亮", "更亮", "高频", "浑浊", "刺耳", "eq", "均衡",
		"cancel", "reject", "deny", "why", "explain", "revise", "change the plan", "apply it", "execute it",
	} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
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

// lowEndRelationAblationEnv is the B4-ABLATION-1 test-only hook: when set to
// exactly "1" the B4 family lexicon short-circuit is bypassed (every low-end
// phrase falls through to the LLM turn) and the canary dispatch to the B4
// runtime is disabled — the ablation face of the A-F scaffolding endstate
// ruling (257 matrix §六 B4 拆法：词表短路+canary dispatch 分支禁用). The
// governance chain (pending proposal confirmation) is not bypassed. Default
// unset keeps production behavior byte-for-byte; the env is never set by
// production startup paths.
const lowEndRelationAblationEnv = "VIT_LOW_END_RELATION_ABLATION"

func lowEndRelationAblationBypassed() bool {
	return strings.TrimSpace(os.Getenv(lowEndRelationAblationEnv)) == "1"
}

func inferProjectAwareCapability(message string) string {
	text := strings.ToLower(strings.TrimSpace(message))
	for _, marker := range []string{
		"static_mix.static_balance", "static_mix.focus_position", "b2", "b5",
		"静态平衡", "静态音量", "推子平衡", "静态主次", "建立主次", "主次关系", "核心元素定位", "突出主唱",
		"static balance", "focus position",
	} {
		if capabilityMarkerPresent(text, marker) {
			return staticBalanceCapabilityID
		}
	}
	for _, marker := range []string{"static_mix.pan_layout", "b3", "声像布局", "pan layout", "panning layout"} {
		if capabilityMarkerPresent(text, marker) {
			return panLayoutCapabilityID
		}
	}
	if !lowEndRelationAblationBypassed() {
		for _, marker := range []string{"static_mix.low_end_relation", "b4", "低频关系", "low-end relation", "low end relation", "低频分析", "低频占用", "低频遮蔽"} {
			if capabilityMarkerPresent(text, marker) {
				return lowEndRelationCapabilityID
			}
		}
	}
	for _, marker := range []string{"fine_mix.frequency_cleanup", "c1", "frequency cleanup", "frequency clean-up", "频段清理", "频率清理", "频谱清理", "全工程eq清理"} {
		if capabilityMarkerPresent(text, marker) {
			return frequencyCleanupCapabilityID
		}
	}
	// C2 is a project capability entry, not a lexical successor for ordinary
	// selected-track dynamic requests. Only an explicit C2 capability marker
	// may claim ownership here.
	for _, marker := range []string{"fine_mix.dynamic_control", "c2"} {
		if capabilityMarkerPresent(text, marker) {
			return dynamicControlCapabilityID
		}
	}
	// C3 同 C2：工程级只读观察能力入口，不做普通空间/混响话术的词法后继
	// （MIXLAYER-M1-3）；只有显式 C3 标记可在此认领。
	for _, marker := range []string{"fine_mix.space_depth", "c3"} {
		if capabilityMarkerPresent(text, marker) {
			return spaceDepthCapabilityID
		}
	}
	// D1-D3 同 C2/C3：工程级只读观察能力入口，不做普通响度/翻译/参考曲话术
	// 的词法后继（MIXLAYER-M1-4）；只有显式 D 标记可在此认领。AF-D1-LOUDNESS-
	// SETTLE-1 增补显式检查意图词（全局响度检查/峰值检查族）——这些是明确的
	// D1 检查请求，不是裸「响度」话术；入口仍是观察 pack，settle 深面由
	// d1LoudnessSettleIntent 在 handler 内分支。
	for _, marker := range []string{
		"pre_master.observation", "d1", "d2", "d3",
		"全局响度检查", "响度检查", "峰值检查", "母带前响度", "母带前检查",
		"global loudness check", "loudness check", "loudness settle",
	} {
		if capabilityMarkerPresent(text, marker) {
			return premasterObservationCapabilityID
		}
	}
	return ""
}
