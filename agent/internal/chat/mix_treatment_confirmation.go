package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"vit-daw-agent/internal/actionworkflow"
	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/agentprotocol"
)

type mixResolverDecision struct {
	SchemaVersion string         `json:"schema_version,omitempty"`
	Status        string         `json:"status,omitempty"`
	ActionKind    string         `json:"action_kind,omitempty"`
	ProcessorType string         `json:"processor_type,omitempty"`
	TargetRef     string         `json:"target_ref,omitempty"`
	ToolRoute     []string       `json:"tool_route,omitempty"`
	Needs         []string       `json:"needs,omitempty"`
	Reason        string         `json:"reason,omitempty"`
	Pending       map[string]any `json:"pending,omitempty"`
	Command       map[string]any `json:"command,omitempty"`
}

func (s *Server) handlePendingMixTreatmentChat(ctx context.Context, conversationID string, req ChatRequest, mode string) (ChatResponse, bool) {
	treatment, ok := s.pendingMixTreatmentForConversation(conversationID)
	if !ok {
		return ChatResponse{}, false
	}
	confirmation := actionworkflow.ClassifyConfirmation(req.Message, true)
	switch {
	case confirmation.Kind == actionworkflow.DecisionRevision:
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] revision requested conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		s.transitionActivePendingCandidate(conversationID, "mix_treatment", agentprotocol.PendingStatusRevisionRequested, req.Message)
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{}, false
	case confirmation.Kind == actionworkflow.DecisionFollowup || messageKeepsMixTickDiscussion(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] discussion continued without resolution conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		return ChatResponse{}, false
	case confirmation.Kind == actionworkflow.DecisionReject:
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] rejected conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		s.transitionActivePendingCandidate(conversationID, "mix_treatment", agentprotocol.PendingStatusRejected, "user rejected pending mix treatment")
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "已取消这条待确认混音建议，没有执行任何工程修改。",
			GoalStatus:     "completed",
			StopReason:     "mix_treatment_rejected",
		}, true
	case messageClearlyShiftsMixTickContext(req.Message):
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] expired on context shift conversation=%s message=%q action=%s processor=%s target=%s", conversationID, req.Message, treatment.ActionKind, treatment.ProcessorType, treatment.TargetRef)
		}
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{}, false
	case confirmation.Kind == actionworkflow.DecisionAccept:
		decision := s.resolveMixTreatment(ctx, treatment, req.Context)
		s.transitionActivePendingCandidate(conversationID, "mix_treatment", agentprotocol.PendingStatusAccepted, "user confirmed pending mix treatment")
		s.expirePendingMixTreatment(conversationID)
		if s != nil && s.logger != nil {
			s.logger.Info("[mix.treatment.pending] resolver decision conversation=%s status=%s action=%s processor=%s target=%s",
				conversationID, decision.Status, decision.ActionKind, decision.ProcessorType, decision.TargetRef)
		}
		s.emitAgentEvent(conversationID, AgentEvent{
			Type:     "mix_treatment.resolver_decision",
			ItemType: "mix_treatment",
			Status:   decision.Status,
			Title:    mixTreatmentResolverEventTitle(decision),
			Body:     mixTreatmentResolverEventBody(decision),
			Payload: map[string]any{
				"schema_version":       decision.SchemaVersion,
				"status":               decision.Status,
				"action_kind":          decision.ActionKind,
				"processor_type":       decision.ProcessorType,
				"target_ref":           decision.TargetRef,
				"tool_route":           decision.ToolRoute,
				"needs":                decision.Needs,
				"pending":              decision.Pending,
				"command":              decision.Command,
				"diagnosis_context_id": treatment.DiagnosisContextID,
				"diagnosis_context":    cloneContext(treatment.DiagnosisContext),
			},
		})
		if decision.Status == "ready_gain_tick" {
			return s.executeResolvedMixTreatmentMixTick(ctx, conversationID, req, mode, treatment, decision), true
		}
		if decision.Status == "ready_pan_tick" {
			return s.executeResolvedMixTreatmentMixTick(ctx, conversationID, req, mode, treatment, decision), true
		}
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          mixTreatmentResolverReply(decision),
			GoalStatus:     "completed",
			StopReason:     "mix_treatment_resolved_" + decision.Status,
		}, true
	case messageAmbiguousMixTickApproval(req.Message):
		return ChatResponse{
			ConversationID: conversationID,
			AgentMode:      mode,
			Reply:          "我还没有执行任何工程修改。当前等待确认的是一个混音处理方向，不是立即写参数；如果要继续，请明确说“确认执行”或“继续执行”。",
			GoalStatus:     "completed",
			StopReason:     "ambiguous_mix_treatment_confirmation",
		}, true
	default:
		s.expirePendingMixTreatment(conversationID)
		return ChatResponse{}, false
	}
}

func mixTreatmentResolverEventTitle(decision mixResolverDecision) string {
	switch strings.TrimSpace(decision.Status) {
	case "needs_preparation":
		return "混音建议需要准备"
	case "ready_gain_tick", "ready_pan_tick":
		return "混音建议已可执行"
	case "needs_clarification":
		return "混音建议需要澄清"
	case "observation_only":
		return "只读观察"
	default:
		return "混音建议解析结果"
	}
}

func mixTreatmentResolverEventBody(decision mixResolverDecision) string {
	switch strings.TrimSpace(decision.Status) {
	case "needs_preparation":
		return "这个混音建议还需要补齐目标、插件或精确控制信息；当前不会写入任何插件参数。"
	case "ready_gain_tick":
		return "已确认这是一个小幅电平调整候选，可以通过安全的 mix tick 路径执行。"
	case "ready_pan_tick":
		return "已确认这是一个小幅声像调整候选，可以通过安全的 mix tick 路径执行。"
	case "needs_clarification":
		return "这个混音建议还需要先澄清目标或控制信息，当前不会修改工程。"
	case "observation_only":
		return "这个结果只是观察结论，不会修改工程。"
	default:
		if text := localizedDisplayTextFallback(strings.TrimSpace(decision.Reason)); text != "" && !looksMostlyEnglish(text) {
			return text
		}
		return "混音建议已完成本地解析；当前不会绕过确认直接修改工程。"
	}
}

func messageRevisesPendingMixTreatment(message string) bool {
	if actionworkflow.ClassifyConfirmation(message, true).Kind == actionworkflow.DecisionRevision {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	if messageLooksLikePanRevisionRequest(text) {
		return true
	}
	return textHasAny(text,
		"\u6539\u6210", "\u6539\u4e3a", "\u6362\u6210", "\u6362\u4e00\u4e2a", "\u6362\u4e00\u7248", "\u522b\u8fd9\u4e2a", "\u4e0d\u662f\u8fd9\u4e2a", "\u4e0d\u8981\u8fd9\u4e2a",
		"\u6539\u5230", "\u8c03\u5230", "\u8bbe\u4e3a", "\u8bbe\u7f6e\u4e3a", "\u5de6\u0037\u0030", "\u53f3\u0037\u0030",
		"change to", "switch to", "instead", "not that", "replace", "revise", "set to", "make it", "70% left", "70% right", "hard left", "hard right",
	)
}

func messageLooksLikePanRevisionRequest(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	if textHasAny(text, "\u4e3a\u4ec0\u4e48", "\u4e3a\u5565", "\u539f\u56e0", "\u89e3\u91ca", "why", "explain") {
		return false
	}
	hasDirection := textHasAny(text,
		"\u5de6", "\u53f3", "\u5c45\u4e2d", "\u56de\u4e2d", "\u4e2d\u95f4",
		"left", "right", "center", "centre", "middle",
	)
	if !hasDirection {
		return false
	}
	hasPanOrRevisionVerb := textHasAny(text,
		"\u58f0\u50cf", "\u58f0\u76f8", "\u6446", "\u6446\u5230", "\u6446\u5411", "\u6446\u8fc7\u53bb", "\u653e\u5230", "\u653e\u5728",
		"\u5f80", "\u5411", "\u9760", "\u9760\u5de6", "\u9760\u53f3", "\u504f", "\u504f\u5de6", "\u504f\u53f3",
		"\u6253\u5230", "\u6253\u5230\u5e95", "\u62c9\u5230", "\u62c9\u5230\u5e95", "\u63a8\u5230",
		"\u6539", "\u6539\u5230", "\u6539\u6210", "\u8c03", "\u8c03\u5230", "\u8bbe\u4e3a", "\u8bbe\u7f6e\u4e3a",
		"pan", "panning", "move", "place", "position", "put", "set", "change", "switch",
	)
	hasPlacementIntensity := textHasAny(text,
		"\u5b8c\u5168", "\u5f7b\u5e95", "\u5168", "\u5168\u5de6", "\u5168\u53f3", "\u6700", "\u6700\u5de6", "\u6700\u53f3", "\u6ee1", "\u5230\u5e95",
		"%", "hard", "full", "fully", "all the way", "far",
	)
	return hasPanOrRevisionVerb || hasPlacementIntensity
}

func messagePlainMixApproval(message string) bool {
	return actionworkflow.ClassifyConfirmation(message, true).Kind == actionworkflow.DecisionAccept
}

func (s *Server) pendingMixTreatmentForConversation(conversationID string) (agentloop.MixTreatmentPending, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return agentloop.MixTreatmentPending{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	treatment, ok := s.pendingTreatments[conversationID]
	if !ok || !strings.EqualFold(strings.TrimSpace(treatment.Status), "pending_confirmation") {
		return agentloop.MixTreatmentPending{}, false
	}
	return treatment, true
}

func (s *Server) expirePendingMixTreatment(conversationID string) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pendingTreatments, conversationID)
}

func (s *Server) resolveMixTreatment(ctx context.Context, treatment agentloop.MixTreatmentPending, requestContext map[string]any) mixResolverDecision {
	decision := mixResolverDecision{
		SchemaVersion: "mix_resolver_decision.v0",
		Status:        "needs_preparation",
		ActionKind:    strings.TrimSpace(treatment.ActionKind),
		ProcessorType: strings.TrimSpace(treatment.ProcessorType),
		TargetRef:     strings.TrimSpace(treatment.TargetRef),
		Needs:         append([]string(nil), treatment.NeedsResolution...),
		Reason:        "resolver must verify executability before this treatment can run.",
	}
	if decision.ActionKind == "" {
		decision.ActionKind = "observation_only"
	}
	if decision.ProcessorType == "" {
		decision.ProcessorType = "unknown"
	}
	if decision.TargetRef == "" {
		decision.TargetRef = "unknown"
	}
	if targetNeedsClarification(decision.TargetRef, decision.Needs) {
		decision.Status = "needs_clarification"
		decision.Reason = "Target track is still unclear; confirm the treatment target first."
		decision.Needs = appendUniqueStrings(decision.Needs, "target_track")
		return decision
	}
	switch strings.ToLower(decision.ActionKind) {
	case "gain_balance":
		return s.resolveGainBalanceTreatment(treatment, decision)
	case "pan_balance":
		return s.resolvePanBalanceTreatment(treatment, decision)
	case "plugin_treatment":
		decision.Status = "observation_only"
		decision.ToolRoute = []string{"plugin_grabber.explain_controls"}
		decision.Reason = "Stored mapping treatment execution is retired; only live parameter observation is available here."
		decision.Needs = nil
		return decision
	case "ask_clarification":
		decision.Status = "needs_clarification"
		decision.Reason = "This pending treatment asks for clarification first."
		return decision
	case "observation_only":
		decision.Status = "observation_only"
		decision.Reason = "This pending treatment is observation-only and will not mutate the DAW."
		return decision
	default:
		decision.Status = "needs_preparation"
		decision.Reason = fmt.Sprintf("unknown treatment action %q; resolver v0 will not execute it.", decision.ActionKind)
		return decision
	}
}

func removeEmptyTreatmentValues(row map[string]any) {
	for key, value := range row {
		if strings.TrimSpace(fmt.Sprint(value)) == "" || strings.TrimSpace(fmt.Sprint(value)) == "<nil>" {
			delete(row, key)
		}
	}
}

func trackIDFromTreatmentTarget(targetRef string) string {
	targetRef = strings.TrimSpace(targetRef)
	lower := strings.ToLower(targetRef)
	if strings.HasPrefix(lower, "track:") {
		return strings.TrimSpace(targetRef[len("track:"):])
	}
	if strings.HasPrefix(lower, "track_id:") {
		return strings.TrimSpace(targetRef[len("track_id:"):])
	}
	return ""
}

func (s *Server) resolveGainBalanceTreatment(treatment agentloop.MixTreatmentPending, base mixResolverDecision) mixResolverDecision {
	decision := base
	decision.ToolRoute = []string{"mix.propose_tick", "mix.apply_tick", "mix.observe"}
	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	deltaDB := treatmentDeltaDB(treatment)
	if trackID == "" || deltaDB == 0 || math.Abs(deltaDB) > 2 {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
		decision.Reason = "gain_balance can use the existing gain tick route, but it still needs one explicit nonzero delta within +/-2 dB."
		return decision
	}
	decision.Status = "ready_gain_tick"
	decision.Needs = nil
	decision.Reason = "gain_balance has an explicit track target and small delta, so confirmation can execute through the existing gain tick route."
	decision.Pending = map[string]any{
		"operation": "track_gain_adjust",
		"track_id":  trackID,
		"delta_db":  deltaDB,
	}
	return decision
}

func (s *Server) resolvePanBalanceTreatment(treatment agentloop.MixTreatmentPending, base mixResolverDecision) mixResolverDecision {
	decision := base
	decision.ToolRoute = []string{"mix.propose_tick", "mix.apply_tick", "mix.observe"}
	trackID := trackIDFromTreatmentTarget(treatment.TargetRef)
	deltaPan := treatmentDeltaPan(treatment)
	targetPan := treatmentTargetPan(treatment)
	if trackID == "" {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "target_track")
		decision.Reason = "pan_balance can use the typed pan tick route, but it needs one clear target track."
		return decision
	}
	if targetPan != nil {
		if *targetPan < -1 || *targetPan > 1 {
			decision.Status = "needs_preparation"
			decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
			decision.Reason = "pan_balance target pan must be within -1.0 left to +1.0 right."
			return decision
		}
		decision.Status = "ready_pan_tick"
		decision.Needs = nil
		decision.Reason = "pan_balance has an explicit track target and target pan, so confirmation can execute through the typed pan tick route."
		decision.Pending = map[string]any{
			"operation":  "track_pan_set",
			"track_id":   trackID,
			"target_pan": *targetPan,
		}
		return decision
	}
	if deltaPan == 0 || math.Abs(deltaPan) > 0.15 {
		decision.Status = "needs_preparation"
		decision.Needs = appendUniqueStrings(decision.Needs, "exact_control")
		decision.Reason = "pan_balance can use the typed pan tick route, but it still needs one explicit nonzero pan delta within +/-0.15 or a target pan within -1.0..+1.0."
		return decision
	}
	decision.Status = "ready_pan_tick"
	decision.Needs = nil
	decision.Reason = "pan_balance has an explicit track target and small pan delta, so confirmation can execute through the typed pan tick route."
	decision.Pending = map[string]any{
		"operation": "track_pan_adjust",
		"track_id":  trackID,
		"delta_pan": deltaPan,
	}
	return decision
}

func (s *Server) executeResolvedMixTreatmentMixTick(ctx context.Context, conversationID string, req ChatRequest, mode string, treatment agentloop.MixTreatmentPending, decision mixResolverDecision) ChatResponse {
	trackID := cleanContextText(decision.Pending["track_id"])
	operation := firstNonEmpty(cleanContextText(decision.Pending["operation"]), operationFromMixTreatmentDecision(treatment, decision))
	candidate := agentloop.PendingMixTickCandidate{
		Operation:                 operation,
		TrackID:                   trackID,
		ObservationID:             treatment.ObservationID,
		Evidence:                  map[string]any{"source": "mix_treatment_resolver", "intent": treatment.Intent, "action_kind": treatment.ActionKind, "diagnosis_context_id": treatment.DiagnosisContextID, "diagnosis_context": cloneContext(treatment.DiagnosisContext), "evidence_refs": append([]string(nil), treatment.EvidenceRefs...)},
		CreatedFromReply:          treatment.CreatedFromReply,
		ExpiresAfterContextChange: treatment.ExpiresAfterContextChange,
		Status:                    "pending_confirmation",
		Fingerprint:               cloneContext(treatment.Fingerprint),
	}
	switch operation {
	case "track_pan_adjust":
		candidate.DeltaPan = treatmentDeltaPan(treatment)
		if candidate.DeltaPan == 0 {
			if value, ok := treatmentNumber(decision.Pending, "delta_pan", "pan_delta"); ok {
				candidate.DeltaPan = value
			}
		}
	case "track_pan_set":
		candidate.TargetPan = treatmentTargetPan(treatment)
		if candidate.TargetPan == nil {
			if value, ok := treatmentNumber(decision.Pending, "target_pan", "pan", "pan_value"); ok {
				candidate.TargetPan = &value
			}
		}
	default:
		candidate.Operation = "track_gain_adjust"
		candidate.DeltaDB = treatmentDeltaDB(treatment)
		if candidate.DeltaDB == 0 {
			if value, ok := treatmentNumber(decision.Pending, "delta_db"); ok {
				candidate.DeltaDB = value
			}
		}
	}
	if candidate.Fingerprint == nil {
		candidate.Fingerprint = map[string]any{}
	}
	if candidate.Fingerprint["target_track_id"] == nil {
		candidate.Fingerprint["target_track_id"] = trackID
	}
	if candidate.Fingerprint["observation_id"] == nil {
		candidate.Fingerprint["observation_id"] = treatment.ObservationID
	}
	return s.executePendingMixTickCandidate(ctx, conversationID, req, mode, candidate)
}

func operationFromMixTreatmentDecision(treatment agentloop.MixTreatmentPending, decision mixResolverDecision) string {
	if strings.EqualFold(strings.TrimSpace(decision.Status), "ready_pan_tick") || strings.EqualFold(strings.TrimSpace(treatment.ActionKind), "pan_balance") {
		if treatmentTargetPan(treatment) != nil {
			return "track_pan_set"
		}
		return "track_pan_adjust"
	}
	return "track_gain_adjust"
}

func treatmentDeltaDB(treatment agentloop.MixTreatmentPending) float64 {
	if treatment.DeltaDB != 0 {
		return treatment.DeltaDB
	}
	if value, ok := treatmentNumber(treatment.Target, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value
	}
	return 0
}

func treatmentDeltaPan(treatment agentloop.MixTreatmentPending) float64 {
	if treatment.DeltaPan != 0 {
		return treatment.DeltaPan
	}
	if value, ok := treatmentNumber(treatment.Target, "delta_pan", "pan_delta"); ok {
		return value
	}
	return 0
}

func treatmentTargetPan(treatment agentloop.MixTreatmentPending) *float64 {
	if treatment.TargetPan != nil {
		value := *treatment.TargetPan
		return &value
	}
	if value, ok := treatmentNumber(treatment.Target, "target_pan", "pan", "pan_value"); ok {
		return &value
	}
	return nil
}

func treatmentNumber(row map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if row == nil {
			return 0, false
		}
		value, ok := row[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case float64:
			return typed, true
		case float32:
			return float64(typed), true
		case int:
			return float64(typed), true
		case int64:
			return float64(typed), true
		case json.Number:
			if n, err := typed.Float64(); err == nil {
				return n, true
			}
		case string:
			if n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

func targetNeedsClarification(targetRef string, needs []string) bool {
	targetRef = strings.ToLower(strings.TrimSpace(targetRef))
	if targetRef == "" || targetRef == "unknown" || targetRef == "vocal_unknown" || strings.Contains(targetRef, "unknown") {
		return true
	}
	for _, need := range needs {
		need = strings.ToLower(strings.TrimSpace(need))
		if need == "target_track" {
			return true
		}
	}
	return false
}

func appendUniqueStrings(values []string, extra ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)+len(extra))
	for _, value := range append(values, extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func withoutString(values []string, remove string) []string {
	remove = strings.TrimSpace(remove)
	if remove == "" {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) == remove {
			continue
		}
		out = append(out, value)
	}
	return out
}

func mixTreatmentResolverReply(decision mixResolverDecision) string {
	switch decision.Status {
	case "needs_clarification":
		return "我还没有执行任何工程修改。这个混音处理需要先确认明确的目标轨道：" + decision.TargetRef + "。"
	case "ready_gain_tick":
		return "我还没有执行任何工程修改。这个电平动作已经可以走安全的单步增益调整路径。"
	case "ready_pan_tick":
		return "我还没有执行任何工程修改。这个声像动作已经可以走安全的单步声像调整路径。"
	case "needs_preparation":
		return "我还没有执行任何工程修改。这个处理缺少明确、可验证的确定性控制信息，因此不会写入插件参数。"
	case "observation_only":
		return "我还没有执行任何工程修改。这个待处理项只是观察结论，没有可执行的 DAW 修改。"
	default:
		return "我还没有执行任何工程修改。当前没有找到足够安全的可执行路径。"
	}
}
