package agentloop

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var messageLoopMixTreatmentPendingPattern = regexp.MustCompile(`(?im)(?:^|\n)\s*mix_treatment_pending\s*:`)

type messageLoopMixTreatmentPendingMarkup struct {
	Start   int
	End     int
	Payload string
}

func messageLoopMixTreatmentPendingMarkupPresent(reply string) bool {
	return messageLoopMixTreatmentPendingPattern.MatchString(reply)
}

func messageLoopMixTreatmentPendingMarkupMatch(reply string) (messageLoopMixTreatmentPendingMarkup, bool) {
	var out messageLoopMixTreatmentPendingMarkup
	loc := messageLoopMixTreatmentPendingPattern.FindStringIndex(reply)
	if len(loc) != 2 {
		return out, false
	}
	out.Start = loc[0]
	out.End = loc[1]
	if payload, end, ok := messageLoopScanJSONObject(reply, loc[1]); ok {
		out.Payload = payload
		out.End = end
	} else if lineEnd := strings.IndexByte(reply[loc[1]:], '\n'); lineEnd >= 0 {
		out.End = loc[1] + lineEnd + 1
	} else {
		out.End = len(reply)
	}
	return out, true
}

func messageLoopScanJSONObject(text string, start int) (string, int, bool) {
	i := start
	for i < len(text) {
		switch text[i] {
		case ' ', '\t', '\r', '\n':
			i++
			continue
		}
		break
	}
	if i >= len(text) || text[i] != '{' {
		return "", i, false
	}
	objectStart := i
	depth := 0
	inString := false
	escaped := false
	for i < len(text) {
		ch := text[i]
		if inString {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			i++
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end := i + 1
				return text[objectStart:end], end, true
			}
		}
		i++
	}
	return "", len(text), false
}

func messageLoopMixTreatmentPendingFromReply(state *runState, reply string) *MixTreatmentPending {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	if messageLoopMutationBarrierActive(state) {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		// Mid-round second-mutation refusal, symmetric with the mix tick
		// synthesis: a pending treatment parks the continuation at
		// waiting_interaction while the settle report is still owed.
		return nil
	}
	match, ok := messageLoopMixTreatmentPendingMarkupMatch(reply)
	if !ok {
		pending := messageLoopImplicitPanTreatmentPendingFromReply(state, reply)
		messageLoopAttachDiagnosisToTreatment(state, pending)
		return pending
	}
	if strings.TrimSpace(match.Payload) == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(match.Payload)), &payload); err != nil {
		return nil
	}
	observationID := firstNonEmpty(firstMapText(payload, "observation_id"), messageLoopLastMixObservationField(state, "observation_id"))
	pending := MixTreatmentPending{
		SchemaVersion:             firstNonEmpty(firstMapText(payload, "schema_version"), "mix_treatment_pending.v0"),
		Status:                    firstNonEmpty(firstMapText(payload, "status"), "pending_confirmation"),
		ConversationID:            firstNonEmpty(firstMapText(payload, "conversation_id"), messageLoopConversationID(state)),
		ObservationID:             observationID,
		Intent:                    firstNonEmpty(firstMapText(payload, "intent"), strings.TrimSpace(state.input.UserText)),
		TargetRef:                 firstNonEmpty(firstMapText(payload, "target_ref"), "unknown"),
		ActionKind:                firstNonEmpty(firstMapText(payload, "action_kind"), "observation_only"),
		ProcessorType:             firstNonEmpty(firstMapText(payload, "processor_type"), "unknown"),
		DeltaDB:                   messageLoopTreatmentDeltaDB(payload),
		DeltaPan:                  messageLoopTreatmentDeltaPan(payload),
		TargetPan:                 messageLoopTreatmentTargetPan(payload),
		PluginID:                  firstMapText(payload, "plugin_id"),
		PluginName:                firstMapText(payload, "plugin_name"),
		Control:                   firstMapText(payload, "control"),
		Target:                    cloneMap(messageLoopMapValue(payload["target"])),
		ReasoningSummary:          firstMapText(payload, "reasoning_summary"),
		Confidence:                firstMapText(payload, "confidence"),
		EvidenceRefs:              messageLoopStringSlice(payload["evidence_refs"]),
		NeedsResolution:           messageLoopStringSlice(payload["needs_resolution"]),
		ExpiresAfterContextChange: true,
		CreatedFromReply:          reply,
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
		},
	}
	if value, ok := payload["expires_after_context_change"]; ok {
		pending.ExpiresAfterContextChange = messageLoopBool(value)
	}
	if pending.DeltaDB == 0 && strings.EqualFold(strings.TrimSpace(pending.ActionKind), "gain_balance") {
		pending.DeltaDB = messageLoopTreatmentGainDeltaFromReply(reply)
	}
	if pending.DeltaPan == 0 && pending.TargetPan == nil && strings.EqualFold(strings.TrimSpace(pending.ActionKind), "pan_balance") {
		pending.DeltaPan, pending.TargetPan = messageLoopTreatmentPanFromReply(reply)
	}
	if strings.EqualFold(strings.TrimSpace(pending.ActionKind), "pan_balance") {
		if deltaPan, targetPan := messageLoopTreatmentPanFromReply(state.input.UserText); deltaPan != 0 || targetPan != nil {
			changed := deltaPan != pending.DeltaPan || !messageLoopSameOptionalPan(targetPan, pending.TargetPan)
			pending.DeltaPan = deltaPan
			pending.TargetPan = targetPan
			if changed && pending.Target != nil {
				delete(pending.Target, "delta_pan")
				delete(pending.Target, "pan_delta")
				delete(pending.Target, "target_pan")
				delete(pending.Target, "pan")
				delete(pending.Target, "pan_value")
			}
		}
	}
	if !strings.EqualFold(strings.TrimSpace(pending.SchemaVersion), "mix_treatment_pending.v0") {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(pending.Status), "pending_confirmation") {
		return nil
	}
	if mixWorkflowPendingActionKindBlocked(pending.ActionKind) {
		return nil
	}
	if messageLoopMOMActionPreflightBlocked(state) {
		return nil
	}
	messageLoopAttachDiagnosisToTreatment(state, &pending)
	return &pending
}

func messageLoopSameOptionalPan(left *float64, right *float64) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func messageLoopImplicitPanTreatmentPendingFromReply(state *runState, reply string) *MixTreatmentPending {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	if messageLoopMutationBarrierActive(state) {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		return nil
	}
	if !messageLoopImplicitPanFollowupRequest(state) || messageLoopExplicitPluginRequest(state) {
		return nil
	}
	if !messageLoopReplyAsksForPanConfirmation(reply) {
		return nil
	}
	deltaPan, targetPan := messageLoopTreatmentPanFromReply(state.input.UserText)
	if deltaPan == 0 && targetPan == nil {
		deltaPan, targetPan = messageLoopTreatmentPanFromReply(reply)
	}
	if deltaPan == 0 && targetPan == nil {
		return nil
	}
	trackID := messageLoopPendingMixCandidateTrackID(state, reply, state.input.UserText)
	if trackID == "" {
		trackID = messageLoopLastMixObservationTrackID(state)
	}
	trackID = messageLoopCanonicalMixTrackID(state, trackID)
	if trackID == "" {
		return nil
	}
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	return &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    firstNonEmpty(strings.TrimSpace(state.input.UserText), strings.TrimSpace(reply)),
		TargetRef:                 "track:" + trackID,
		ActionKind:                "pan_balance",
		ProcessorType:             "utility",
		DeltaPan:                  deltaPan,
		TargetPan:                 targetPan,
		ReasoningSummary:          "implicit assistant pan confirmation question",
		Confidence:                "medium",
		EvidenceRefs:              []string{observationID},
		ExpiresAfterContextChange: true,
		CreatedFromReply:          reply,
		Fingerprint: map[string]any{
			"conversation_id":   messageLoopConversationID(state),
			"goal_id":           state.goal.GoalID,
			"run_id":            state.goal.RunID,
			"observation_id":    observationID,
			"target_scope":      messageLoopLastMixObservationScope(state),
			"target_track_id":   trackID,
			"track_count":       messageLoopPendingMixCandidateTrackCount(state),
			"created_from_turn": state.turnsUsed,
			"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
		},
	}
}

func messageLoopConservativeLowMudTreatmentPendingFromReply(state *runState, reply string) *MixTreatmentPending {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	if messageLoopMutationBarrierActive(state) {
		return nil
	}
	if messageLoopFreeStateRoundPendingSettlement(state) {
		return nil
	}
	if state.executionMemory.PendingMixTreatment != nil {
		return nil
	}
	if !messageLoopLowMudPluginPrepForState(state) {
		return nil
	}
	if !messageLoopHasAnyMixObservationAttempt(state) {
		return nil
	}
	trackID := messageLoopPendingMixCandidateTrackIDFromReply(state, reply, state.input.UserText)
	if trackID == "" {
		trackID = messageLoopLastMixObservationTrackID(state)
	}
	trackID = messageLoopCanonicalMixTrackID(state, trackID)
	targetRef := "project"
	needsResolution := []string{"plugin_instance", "live_parameter_surface", "exact_control"}
	fingerprint := map[string]any{
		"conversation_id":   messageLoopConversationID(state),
		"goal_id":           state.goal.GoalID,
		"run_id":            state.goal.RunID,
		"observation_id":    messageLoopLastMixObservationField(state, "observation_id"),
		"target_scope":      messageLoopLastMixObservationScope(state),
		"track_count":       messageLoopPendingMixCandidateTrackCount(state),
		"created_from_turn": state.turnsUsed,
		"mix_session_id":    messageLoopLastMixObservationField(state, "mix_session_id"),
		"source":            "deterministic_low_mud_plugin_pending_after_observation",
	}
	if trackID != "" {
		targetRef = "track:" + trackID
		fingerprint["target_track_id"] = trackID
	} else {
		needsResolution = append([]string{"target_track"}, needsResolution...)
	}
	observationID := messageLoopLastMixObservationField(state, "observation_id")
	treatment := &MixTreatmentPending{
		SchemaVersion:             "mix_treatment_pending.v0",
		Status:                    "pending_confirmation",
		ConversationID:            messageLoopConversationID(state),
		ObservationID:             observationID,
		Intent:                    firstNonEmpty(strings.TrimSpace(state.input.UserText), strings.TrimSpace(reply)),
		TargetRef:                 targetRef,
		ActionKind:                "plugin_treatment",
		ProcessorType:             "eq",
		ReasoningSummary:          "prepare a conservative reversible low-cut or low-mid EQ probe; missing band evidence keeps this as a pending candidate only",
		Confidence:                "low",
		NeedsResolution:           needsResolution,
		ExpiresAfterContextChange: true,
		CreatedFromReply:          reply,
		Fingerprint:               fingerprint,
	}
	if observationID != "" {
		treatment.EvidenceRefs = []string{observationID}
	}
	messageLoopAttachDiagnosisToTreatment(state, treatment)
	return treatment
}

func messageLoopLowMudPluginPrepRequest(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	hasLowMudIssue := messageLoopTextHasAny(text,
		"\u4f4e\u9891", "\u4f4e\u4e2d\u9891", "\u6d51\u6d4a", "\u7cca",
		"low end", "low-end", "low mid", "low-mid", "mud", "muddy",
	)
	hasEQIntent := messageLoopTextHasAny(text,
		"eq", "\u5747\u8861", "\u4f4e\u5207", "\u9ad8\u901a", "\u6ee4\u6ce2",
		"low cut", "low-cut", "high pass", "high-pass", "filter",
	)
	hasActionIntent := messageLoopTextHasAny(text,
		"\u5e2e\u6211", "\u5904\u7406", "\u8c03\u6574", "\u8c03\u4e00\u4e0b", "\u6536\u4e00\u70b9", "\u6536\u4f4e\u9891", "\u6536\u4e00\u4e0b", "\u538b\u4f4e", "\u964d\u4f4e", "\u51cf\u5c11", "\u524a\u51cf", "\u524a\u4e00\u70b9", "\u8f7b\u5fae", "\u7a0d\u5fae", "\u53ef\u4ee5\u5904\u7406", "\u53ef\u4ee5\u8fdb\u884c\u5904\u7406",
		"help me", "process", "treat", "adjust", "reduce", "lower", "cut", "trim", "slight", "slightly", "a little",
	)
	hasPrepIntent := messageLoopTextHasAny(text,
		"\u5148\u51c6\u5907", "\u51c6\u5907", "\u53c2\u6570", "\u65b9\u6848", "\u63d2\u4ef6", "\u6548\u679c\u5668",
		"prepare", "prep", "parameter", "parameters", "candidate", "plugin",
	)
	hasEvidenceFirstIntent := messageLoopTextHasAny(text,
		"\u4f9d\u636e", "\u6839\u636e", "\u5224\u65ad", "\u4e3a\u4ec0\u4e48", "\u4e3a\u4f55", "\u5148\u544a\u8bc9", "\u5148\u8bf4",
		"evidence", "basis", "why", "before", "first tell",
	)
	return hasLowMudIssue && ((hasEQIntent || hasPrepIntent) && (hasPrepIntent || hasActionIntent || hasEvidenceFirstIntent) || hasActionIntent)
}

func messageLoopImplicitPanFollowupRequest(state *runState) bool {
	if state == nil {
		return false
	}
	if messageLoopNaturalMixRequest(state.input.UserText) {
		return true
	}
	text := strings.ToLower(strings.TrimSpace(state.input.UserText))
	if text == "" {
		return false
	}
	if !messageLoopTextHasAny(text,
		"左", "右", "中间", "居中", "回中", "left", "right", "center", "centre",
		"摆", "摆到", "摆向", "放到", "往", "向", "靠左", "靠右", "偏左", "偏右", "打到", "打到底", "拉到", "拉到底", "推到", "完全", "彻底", "全", "最", "到底",
	) {
		return false
	}
	return messageLoopHasUsableMixObservation(state) ||
		len(messageLoopMapRows(state.input.State["tracks"])) > 0 ||
		len(messageLoopMapRows(state.contextSnapshot["tracks"])) > 0 ||
		state.executionMemory.ActiveWorkTargetTrackID != ""
}

func messageLoopReplyAsksForPanConfirmation(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return false
	}
	if !messageLoopTextHasAny(text,
		"?", "？", "要我", "是否", "确认", "确定", "继续", "如果你确定", "如果你确认", "我可以继续", "可以继续",
		"should i", "shall i", "would you like me", "if you confirm", "if you are sure", "i can continue",
	) {
		return false
	}
	return messageLoopTextHasAny(text, "声像", "声相", "pan", "panning", "左", "右", "left", "right", "center", "centre")
}

func messageLoopTreatmentDeltaPan(payload map[string]any) float64 {
	if value, ok := messageLoopTreatmentNumber(payload, "delta_pan", "pan_delta"); ok {
		return value
	}
	target := messageLoopMapValue(payload["target"])
	if value, ok := messageLoopTreatmentNumber(target, "delta_pan", "pan_delta"); ok {
		return value
	}
	return 0
}

func messageLoopTreatmentTargetPan(payload map[string]any) *float64 {
	if value, ok := messageLoopTreatmentNumber(payload, "target_pan", "pan", "pan_value"); ok {
		return &value
	}
	target := messageLoopMapValue(payload["target"])
	if value, ok := messageLoopTreatmentNumber(target, "target_pan", "pan", "pan_value"); ok {
		return &value
	}
	return nil
}

func messageLoopStripMixTreatmentPendingMarkup(reply string) string {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return reply
	}
	for {
		match, ok := messageLoopMixTreatmentPendingMarkupMatch(reply)
		if !ok {
			break
		}
		reply = reply[:match.Start] + "\n" + reply[match.End:]
	}
	return strings.TrimSpace(reply)
}

func messageLoopStringSlice(value any) []string {
	var out []string
	switch v := value.(type) {
	case []string:
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
	case []any:
		for _, item := range v {
			if s := strings.TrimSpace(messageLoopText(item)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	case string:
		for _, item := range strings.Split(v, ",") {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func messageLoopMOMActionPreflightBlocked(state *runState) bool {
	for _, proj := range messageLoopRecentMOMProjections(state) {
		trust := messageLoopMapValue(proj["trust_quality"])
		if len(trust) == 0 {
			continue
		}
		if value, ok := trust["can_support_action_preflight"]; ok && !messageLoopBool(value) {
			return true
		}
		for _, reason := range messageLoopStringSlice(trust["blocked_reasons"]) {
			reason = strings.ToLower(strings.TrimSpace(reason))
			if strings.Contains(reason, "action_preflight") {
				return true
			}
		}
		if messageLoopHasActionRelevantTrustField(messageLoopStringSlice(trust["suspect_fields"])) || messageLoopHasActionRelevantTrustField(messageLoopStringSlice(trust["stale_fields"])) {
			return true
		}
	}
	return false
}

func messageLoopHasActionRelevantTrustField(fields []string) bool {
	for _, field := range fields {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "project_structure", "timbre_frequency", "space_stereo":
			return true
		}
	}
	return false
}

func messageLoopRecentMOMProjections(state *runState) []map[string]any {
	if state == nil {
		return nil
	}
	out := []map[string]any{}
	add := func(row map[string]any) {
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	if state.recentObservation != nil && observationIsMixObservation(state.recentObservation) {
		summary := state.recentObservation.Summary
		add(messageLoopMapValue(summary["mom_projection"]))
		add(messageLoopMOMProjectionFromResult(summary))
	}
	add(messageLoopMOMProjectionFromResult(messageLoopLastMixObservationResult(state)))
	return out
}

func messageLoopBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y":
			return true
		default:
			return false
		}
	default:
		return strings.EqualFold(strings.TrimSpace(messageLoopText(value)), "true")
	}
}

func messageLoopTreatmentDeltaDB(payload map[string]any) float64 {
	if value, ok := messageLoopTreatmentNumber(payload, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value
	}
	target := messageLoopMapValue(payload["target"])
	if value, ok := messageLoopTreatmentNumber(target, "delta_db", "db_delta", "gain_delta_db", "volume_delta_db"); ok {
		return value
	}
	return 0
}

func messageLoopTreatmentNumber(row map[string]any, keys ...string) (float64, bool) {
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

func messageLoopTreatmentGainDeltaFromReply(reply string) float64 {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return 0
	}
	matches := messageLoopDBAmountPattern.FindAllStringSubmatchIndex(reply, -1)
	var candidates []float64
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[1] < len(reply) && messageLoopASCIIAlpha(reply[match[1]]) {
			continue
		}
		raw := strings.TrimSpace(reply[match[2]:match[3]])
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value == 0 {
			continue
		}
		beforeStart := match[0] - 72
		if beforeStart < 0 {
			beforeStart = 0
		}
		metricStart := match[0] - 36
		if metricStart < 0 {
			metricStart = 0
		}
		afterEnd := match[1] + 24
		if afterEnd > len(reply) {
			afterEnd = len(reply)
		}
		metricBefore := strings.ToLower(strings.TrimSpace(reply[metricStart:match[0]]))
		window := strings.ToLower(strings.TrimSpace(reply[beforeStart:afterEnd]))
		if messageLoopTextHasAny(metricBefore, "rms", "lufs", "峰值", "余量", "headroom", "crest", "peak", "risk", "风险", "约") {
			continue
		}
		sign := 0.0
		switch {
		case strings.HasPrefix(raw, "-"):
			sign = -1
		case strings.HasPrefix(raw, "+"):
			sign = 1
		case messageLoopTextHasAny(window, "降低", "下调", "调低", "减少", "减", "lower", "reduce", "decrease", "down", "cut"):
			sign = -1
		case messageLoopTextHasAny(window, "提高", "提升", "上调", "调高", "增加", "加", "raise", "boost", "increase", "up", "higher"):
			sign = 1
		}
		if sign == 0 {
			continue
		}
		if value < 0 {
			value = -value
		}
		candidates = append(candidates, sign*value)
	}
	if len(candidates) != 1 {
		return 0
	}
	return candidates[0]
}

func messageLoopTreatmentPanFromReply(reply string) (float64, *float64) {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return 0, nil
	}
	if messageLoopTextHasAny(text, "center", "centre", "middle", "回中", "居中", "中间") {
		zero := 0.0
		return 0, &zero
	}
	value, ok := messageLoopPanPercentFromReply(text)
	if ok {
		if messageLoopTextHasAny(text, "left", "左") {
			value = -mathAbs(value)
		} else if messageLoopTextHasAny(text, "right", "右") {
			value = mathAbs(value)
		} else {
			return 0, nil
		}
		if messageLoopPanLooksLikeSmallMove(text) && !messageLoopPanLooksLikePlacement(text) {
			return clampPanDelta(value), nil
		}
		target := clampPanTarget(value)
		return 0, &target
	}
	if messageLoopTextHasAny(text, "left", "左") && messageLoopPanLooksLikePlacement(text) {
		if targetPan, ok := messageLoopPanNamedTargetFromReply(text); ok {
			return 0, &targetPan
		}
		target := -0.50
		return 0, &target
	}
	if messageLoopTextHasAny(text, "right", "右") && messageLoopPanLooksLikePlacement(text) {
		if targetPan, ok := messageLoopPanNamedTargetFromReply(text); ok {
			return 0, &targetPan
		}
		target := 0.50
		return 0, &target
	}
	if messageLoopTextHasAny(text, "left", "左") {
		return -0.10, nil
	}
	if messageLoopTextHasAny(text, "right", "右") {
		return 0.10, nil
	}
	return 0, nil
}

func messageLoopPanNamedTargetFromReply(text string) (float64, bool) {
	switch {
	case messageLoopTextHasAny(text, "left", "左") && messageLoopTextHasAny(text, "hard", "full", "all", "far", "fully", "完全", "彻底", "全", "最", "满", "到底"):
		return -1, true
	case messageLoopTextHasAny(text, "right", "右") && messageLoopTextHasAny(text, "hard", "full", "all", "far", "fully", "完全", "彻底", "全", "最", "满", "到底"):
		return 1, true
	default:
		return 0, false
	}
}

func messageLoopPanLooksLikePlacement(text string) bool {
	if messageLoopPanLooksLikeSmallMove(text) && !messageLoopTextHasAny(text, "%", "hard", "full", "all", "far", "fully", "完全", "彻底", "全", "最", "满", "到底", "打到") {
		return false
	}
	return messageLoopTextHasAny(
		text,
		"pan to", "set pan", "set the pan", "place", "position", "put", "move to", "hard", "full", "all", "far", "fully",
		"摆", "摆到", "摆向", "放到", "放在", "靠左", "靠右", "偏左", "偏右", "打到", "拉到", "推到", "完全", "彻底", "全左", "全右", "最左", "最右", "左满", "右满", "到底",
	)
}

func messageLoopPanLooksLikeSmallMove(text string) bool {
	return messageLoopTextHasAny(
		text,
		"a little", "slightly", "small", "tiny", "subtle", "nudge", "bit", "little bit", "slight",
		"一点", "一点点", "小", "稍微", "微调", "轻微", "少许",
	)
}

func messageLoopPanPercentFromReply(text string) (float64, bool) {
	matches := regexp.MustCompile(`(?i)([+-]?\d+(?:\.\d+)?)\s*%`).FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(matches[0][1]), 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return value / 100, true
}

func clampPanDelta(value float64) float64 {
	if value > 0.15 {
		return 0.15
	}
	if value < -0.15 {
		return -0.15
	}
	return value
}

func clampPanTarget(value float64) float64 {
	if value > 1 {
		return 1
	}
	if value < -1 {
		return -1
	}
	return value
}

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
