package agentloop

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var messageLoopMixTreatmentPendingPattern = regexp.MustCompile(`(?im)(?:^|\n)\s*mix_treatment_pending\s*:\s*(\{[^\n]*\})\s*(?:\n|$)`)

func messageLoopMixTreatmentPendingFromReply(state *runState, reply string) *MixTreatmentPending {
	if state == nil || strings.TrimSpace(reply) == "" {
		return nil
	}
	match := messageLoopMixTreatmentPendingPattern.FindStringSubmatch(reply)
	if len(match) < 2 || strings.TrimSpace(match[1]) == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload); err != nil {
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
	if !strings.EqualFold(strings.TrimSpace(pending.SchemaVersion), "mix_treatment_pending.v0") {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(pending.Status), "pending_confirmation") {
		return nil
	}
	return &pending
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
	return strings.TrimSpace(messageLoopMixTreatmentPendingPattern.ReplaceAllString(reply, "\n"))
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
		return clampPanDelta(value), nil
	}
	if messageLoopTextHasAny(text, "left", "左") {
		return -0.10, nil
	}
	if messageLoopTextHasAny(text, "right", "右") {
		return 0.10, nil
	}
	return 0, nil
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

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
