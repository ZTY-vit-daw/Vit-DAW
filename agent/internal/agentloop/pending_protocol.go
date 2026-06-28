package agentloop

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/agentprotocol"
)

func (candidate PendingMixTickCandidate) ToPendingCandidate(conversationID, goalID, runID, createdAt string) agentprotocol.PendingCandidate {
	source := agentprotocol.Source{
		ConversationID: strings.TrimSpace(conversationID),
		GoalID:         strings.TrimSpace(goalID),
		RunID:          strings.TrimSpace(runID),
		LegacySchema:   "mix_tick.pending.v0",
		LegacyKind:     "PendingMixTickCandidate",
		Metadata:       pendingMixTickProtocolMetadata(candidate),
	}
	operation := strings.TrimSpace(candidate.Operation)
	action := map[string]any{
		"operation":      operation,
		"track_id":       strings.TrimSpace(candidate.TrackID),
		"observation_id": strings.TrimSpace(candidate.ObservationID),
		"evidence":       cloneMap(candidate.Evidence),
	}
	switch operation {
	case "track_pan_adjust":
		action["delta_pan"] = candidate.DeltaPan
	case "track_pan_set":
		if candidate.TargetPan != nil {
			action["target_pan"] = *candidate.TargetPan
		}
	default:
		action["delta_db"] = candidate.DeltaDB
	}
	removeEmptyProtocolMap(action)
	targetRef := ""
	if strings.TrimSpace(candidate.TrackID) != "" {
		targetRef = "track:" + strings.TrimSpace(candidate.TrackID)
	}
	return agentprotocol.PendingCandidate{
		ID:                        agentprotocol.NormalizeID("pending", "mix_tick", conversationID, goalID, operation, candidate.TrackID, candidate.ObservationID),
		Kind:                      agentprotocol.KindPendingCandidate,
		Domain:                    "mix",
		CandidateType:             "mix_tick",
		TargetRef:                 targetRef,
		Summary:                   pendingMixTickProtocolSummary(candidate),
		Rationale:                 firstNonEmpty(firstMapText(candidate.Evidence, "reason", "source", "matched_text"), strings.TrimSpace(candidate.ObservationID)),
		CandidateAction:           action,
		Risk:                      "undoable_commit",
		RequiredPermissionDomains: []string{"mix.commit"},
		Status:                    agentprotocol.LegacyPendingStatus(candidate.Status),
		CreatedAt:                 strings.TrimSpace(createdAt),
		Source:                    source,
	}
}

func pendingMixTickProtocolMetadata(candidate PendingMixTickCandidate) map[string]any {
	source := cloneMap(candidate.Fingerprint)
	out := map[string]any{}
	for _, key := range []string{
		"conversation_id", "goal_id", "run_id", "target_track_id", "observation_id",
		"target_scope", "track_gain_db", "track_pan", "track_count", "mix_session_id",
		"previous_track_id", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db",
	} {
		if value, ok := source[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
			out[key] = value
		}
	}
	if before := protocolMapValue(source["before_track"]); len(before) > 0 {
		summary := compactSelectedProtocolKeys(before, []string{
			"track_id", "id", "track_name", "name", "user_label",
			"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "volume_db", "gain_db", "pan",
		})
		if len(summary) > 0 {
			out["before_track_summary"] = summary
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func compactSelectedProtocolKeys(row map[string]any, keys []string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" && strings.TrimSpace(fmt.Sprint(value)) != "<nil>" {
			out[key] = value
		}
	}
	return out
}

func protocolMapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func (treatment MixTreatmentPending) ToPendingCandidate(conversationID, goalID, runID, createdAt string) agentprotocol.PendingCandidate {
	conversationID = agentprotocol.FirstNonEmpty(conversationID, treatment.ConversationID)
	source := agentprotocol.Source{
		ConversationID: strings.TrimSpace(conversationID),
		GoalID:         strings.TrimSpace(goalID),
		RunID:          strings.TrimSpace(runID),
		LegacySchema:   agentprotocol.FirstNonEmpty(treatment.SchemaVersion, "mix_treatment_pending.v0"),
		LegacyKind:     "MixTreatmentPending",
		Metadata:       cloneMap(treatment.Fingerprint),
	}
	action := map[string]any{
		"action_kind":          strings.TrimSpace(treatment.ActionKind),
		"processor_type":       strings.TrimSpace(treatment.ProcessorType),
		"target_ref":           strings.TrimSpace(treatment.TargetRef),
		"delta_db":             treatment.DeltaDB,
		"delta_pan":            treatment.DeltaPan,
		"plugin_id":            strings.TrimSpace(treatment.PluginID),
		"plugin_name":          strings.TrimSpace(treatment.PluginName),
		"control":              strings.TrimSpace(treatment.Control),
		"target":               cloneMap(treatment.Target),
		"observation_id":       strings.TrimSpace(treatment.ObservationID),
		"needs_resolution":     append([]string(nil), treatment.NeedsResolution...),
		"diagnosis_context_id": strings.TrimSpace(treatment.DiagnosisContextID),
		"diagnosis_context":    cloneMap(treatment.DiagnosisContext),
		"evidence_refs":        append([]string(nil), treatment.EvidenceRefs...),
	}
	if treatment.TargetPan != nil {
		action["target_pan"] = *treatment.TargetPan
	}
	removeEmptyProtocolMap(action)
	return agentprotocol.PendingCandidate{
		ID:                        agentprotocol.NormalizeID("pending", "mix_treatment", conversationID, goalID, treatment.ActionKind, treatment.TargetRef, treatment.ObservationID),
		Kind:                      agentprotocol.KindPendingCandidate,
		Domain:                    "mix",
		CandidateType:             "mix_treatment",
		TargetRef:                 strings.TrimSpace(treatment.TargetRef),
		Summary:                   pendingMixTreatmentProtocolSummary(treatment),
		Rationale:                 pendingMixTreatmentProtocolRationale(treatment),
		CandidateAction:           action,
		Risk:                      mixTreatmentProtocolRisk(treatment),
		RequiredPermissionDomains: mixTreatmentProtocolPermissionDomains(treatment),
		Status:                    agentprotocol.LegacyPendingStatus(treatment.Status),
		CreatedAt:                 strings.TrimSpace(createdAt),
		Source:                    source,
	}
}

func PendingCandidateFromExecutionMemory(memory ExecutionMemory, conversationID, goalID, runID, createdAt string) *agentprotocol.PendingCandidate {
	if memory.PendingMixTreatment != nil && strings.EqualFold(strings.TrimSpace(memory.PendingMixTreatment.Status), "pending_confirmation") {
		if mixWorkflowPendingActionKindBlocked(memory.PendingMixTreatment.ActionKind) {
			return nil
		}
		pending := memory.PendingMixTreatment.ToPendingCandidate(conversationID, goalID, runID, createdAt)
		return &pending
	}
	if memory.PendingMixTickCandidate != nil && strings.EqualFold(strings.TrimSpace(memory.PendingMixTickCandidate.Status), "pending_confirmation") {
		pending := memory.PendingMixTickCandidate.ToPendingCandidate(conversationID, goalID, runID, createdAt)
		return &pending
	}
	return nil
}

func pendingMixTickProtocolSummary(candidate PendingMixTickCandidate) string {
	trackID := strings.TrimSpace(candidate.TrackID)
	if trackID == "" {
		trackID = "当前轨道"
	} else {
		trackID = "Track " + trackID
	}
	switch strings.TrimSpace(candidate.Operation) {
	case "track_pan_adjust":
		return fmt.Sprintf("将 %s 的声像微调 %+0.2f。", trackID, candidate.DeltaPan)
	case "track_pan_set":
		if candidate.TargetPan != nil {
			return fmt.Sprintf("将 %s 的声像设置为 %+0.2f。", trackID, *candidate.TargetPan)
		}
		return fmt.Sprintf("调整 %s 的声像。", trackID)
	default:
		return fmt.Sprintf("将 %s 的电平调整 %+0.2f dB。", trackID, candidate.DeltaDB)
	}
}

func pendingMixTreatmentProtocolSummary(treatment MixTreatmentPending) string {
	processor := strings.TrimSpace(treatment.ProcessorType)
	if processor == "" || processor == "unknown" {
		processor = strings.TrimSpace(treatment.ActionKind)
	}
	target := strings.TrimSpace(treatment.TargetRef)
	if target == "" {
		target = "project"
	}
	targetLabel := pendingMixTreatmentProtocolTargetLabel(target)
	switch strings.TrimSpace(treatment.ActionKind) {
	case "gain_balance":
		return fmt.Sprintf("为 %s 准备电平平衡处理。", targetLabel)
	case "pan_balance":
		return fmt.Sprintf("为 %s 准备声像平衡处理。", targetLabel)
	case "plugin_treatment":
		if label := strings.TrimSpace(pendingMixTreatmentProtocolProcessorLabel(processor)); label != "" {
			return fmt.Sprintf("为 %s 准备 %s 插件处理。", targetLabel, label)
		}
		return fmt.Sprintf("为 %s 准备插件处理。", targetLabel)
	case "ask_clarification":
		return fmt.Sprintf("先确认 %s 的处理目标。", targetLabel)
	default:
		return fmt.Sprintf("为 %s 准备混音处理。", targetLabel)
	}
}

func pendingMixTreatmentProtocolRationale(treatment MixTreatmentPending) string {
	raw := strings.TrimSpace(treatment.ReasoningSummary)
	if raw == "" || protocolTextMostlyEnglish(raw) {
		switch strings.TrimSpace(treatment.ActionKind) {
		case "plugin_treatment":
			return "已根据当前混音观察生成一个待确认的插件处理候选；确认前不会写入任何插件参数。"
		case "gain_balance":
			return "已根据当前混音观察生成一个待确认的电平调整候选。"
		case "pan_balance":
			return "已根据当前混音观察生成一个待确认的声像调整候选。"
		default:
			return "已根据当前混音观察生成一个待确认的处理候选。"
		}
	}
	return raw
}

func pendingMixTreatmentProtocolTargetLabel(target string) string {
	target = strings.TrimSpace(target)
	switch {
	case target == "", target == "project":
		return "当前工程"
	case strings.HasPrefix(target, "track:"):
		return "Track " + strings.TrimSpace(strings.TrimPrefix(target, "track:"))
	default:
		return target
	}
}

func protocolTextMostlyEnglish(text string) bool {
	letters := 0
	asciiLetters := 0
	cjk := 0
	for _, r := range strings.TrimSpace(text) {
		switch {
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			letters++
			asciiLetters++
		case r >= 0x4e00 && r <= 0x9fff:
			letters++
			cjk++
		}
	}
	return cjk == 0 && asciiLetters >= 12 && asciiLetters*2 >= letters
}

func pendingMixTreatmentProtocolProcessorLabel(processor string) string {
	switch strings.ToLower(strings.TrimSpace(processor)) {
	case "eq":
		return "EQ"
	case "compressor", "compress":
		return "压缩器"
	case "reverb":
		return "混响"
	case "delay":
		return "延迟"
	case "saturation", "satur":
		return "饱和"
	case "limiter", "limit":
		return "限幅"
	case "utility":
		return "工具"
	case "", "unknown":
		return ""
	default:
		return processor + " "
	}
}

func mixTreatmentProtocolRisk(treatment MixTreatmentPending) string {
	switch strings.TrimSpace(treatment.ActionKind) {
	case "observation_only", "ask_clarification":
		return "none"
	default:
		return "undoable_commit"
	}
}

func mixTreatmentProtocolPermissionDomains(treatment MixTreatmentPending) []string {
	switch strings.TrimSpace(treatment.ActionKind) {
	case "plugin_treatment":
		if strings.TrimSpace(treatment.Control) != "" && len(treatment.Target) > 0 {
			return []string{"plugin.param_write", "mix.commit"}
		}
		return []string{"plugin.load", "plugin.read", "plugin.param_write", "mix.commit"}
	case "gain_balance", "pan_balance":
		return []string{"mix.commit"}
	default:
		return nil
	}
}

func removeEmptyProtocolMap(row map[string]any) {
	for key, value := range row {
		if value == nil {
			delete(row, key)
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				delete(row, key)
			}
		case float64:
			if typed == 0 && (key == "delta_db" || key == "delta_pan") {
				delete(row, key)
			}
		case []string:
			if len(typed) == 0 {
				delete(row, key)
			}
		case map[string]any:
			if len(typed) == 0 {
				delete(row, key)
			}
		}
	}
}
