package chat

import (
	"fmt"
	"regexp"
	"strings"

	"vit-daw-agent/internal/agentloop"
)

func localizedMixTreatmentDisplayPayload(treatment agentloop.MixTreatmentPending) map[string]any {
	workflowDisplay := agentloop.ActionWorkflowDisplayFromMixTreatment(treatment)
	reasoning := localizedMixTreatmentReasoning(treatment)
	if reasoning != "" {
		workflowDisplay.Conclusion = reasoning
		workflowDisplay.CardBody = strings.Join([]string{
			"结论：" + workflowDisplay.Conclusion,
			"依据：" + workflowDisplay.Evidence,
			"待确认动作：" + workflowDisplay.PendingAction,
			"限制：" + workflowDisplay.Limitations,
		}, "\n")
	}
	display := map[string]any{
		"status":            firstNonEmpty(workflowDisplay.StatusLabel, "待确认"),
		"target_ref":        firstNonEmpty(workflowDisplay.TargetLabel, localizedMixTreatmentTargetRef(treatment.TargetRef)),
		"action_kind":       firstNonEmpty(workflowDisplay.ActionLabel, localizedMixTreatmentActionKind(treatment.ActionKind)),
		"processor_type":    firstNonEmpty(workflowDisplay.ProcessorLabel, localizedMixTreatmentProcessor(treatment.ProcessorType)),
		"confidence":        firstNonEmpty(workflowDisplay.ConfidenceLabel, localizedMixTreatmentConfidence(treatment.Confidence)),
		"reasoning_summary": firstNonEmpty(reasoning, workflowDisplay.Conclusion),
		"evidence":          workflowDisplay.Evidence,
		"pending_action":    workflowDisplay.PendingAction,
		"limitations":       workflowDisplay.Limitations,
		"card_title":        workflowDisplay.CardTitle,
		"card_body":         workflowDisplay.CardBody,
		"needs_resolution":  localizedMixTreatmentNeeds(treatment.NeedsResolution),
	}
	removeEmptyTreatmentValues(display)
	return display
}

func localizedMixTreatmentTargetRef(targetRef string) string {
	targetRef = strings.TrimSpace(targetRef)
	switch {
	case targetRef == "", targetRef == "project":
		return "当前工程"
	case strings.HasPrefix(targetRef, "track:"):
		return "Track " + strings.TrimSpace(strings.TrimPrefix(targetRef, "track:"))
	default:
		return targetRef
	}
}

func localizedMixTreatmentActionKind(actionKind string) string {
	switch strings.ToLower(strings.TrimSpace(actionKind)) {
	case "gain_balance":
		return "电平平衡"
	case "pan_balance":
		return "声像调整"
	case "plugin_treatment":
		return "插件处理"
	case "ask_clarification":
		return "需要澄清"
	case "observation_only":
		return "只读观察"
	default:
		return ""
	}
}

func localizedMixTreatmentProcessor(processor string) string {
	switch strings.ToLower(strings.TrimSpace(processor)) {
	case "eq":
		return "EQ"
	case "compressor", "compress":
		return "压缩器"
	case "limiter", "limit":
		return "限幅器"
	case "reverb":
		return "混响"
	case "delay":
		return "延迟"
	case "saturation", "satur":
		return "饱和"
	case "utility":
		return "工具"
	case "", "unknown":
		return ""
	default:
		return processor
	}
}

func localizedMixTreatmentConfidence(confidence string) string {
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	case "":
		return ""
	default:
		return confidence
	}
}

func localizedMixTreatmentNeeds(needs []string) []string {
	out := make([]string, 0, len(needs))
	for _, need := range needs {
		switch strings.ToLower(strings.TrimSpace(need)) {
		case "target_track":
			out = append(out, "目标轨道")
		case "plugin_instance":
			out = append(out, "插件实例")
		case "plugin_profile":
			out = append(out, "插件控制映射")
		case "exact_control":
			out = append(out, "精确控制参数")
		case "request_more_observation":
			out = append(out, "需要更多观察")
		case "":
			continue
		default:
			out = append(out, localizedDisplayTextFallback(need))
		}
	}
	return out
}

func localizedMixTreatmentReasoning(treatment agentloop.MixTreatmentPending) string {
	raw := strings.TrimSpace(treatment.ReasoningSummary)
	if raw == "" {
		return localizedMixTreatmentDefaultReasoning(treatment)
	}
	if looksMostlyEnglish(raw) {
		return localizedMixTreatmentEnglishReasoning(raw, treatment)
	}
	return localizedMixTreatmentMixedAcousticTerms(raw)
}

func localizedMixTreatmentUserReply(reply string, treatment agentloop.MixTreatmentPending) string {
	text := strings.TrimSpace(reply)
	if text == "" {
		return ""
	}
	return localizedDisplayTextFallback(text)
}

func localizedMixTreatmentDefaultReasoning(treatment agentloop.MixTreatmentPending) string {
	target := localizedMixTreatmentTargetRef(treatment.TargetRef)
	action := localizedMixTreatmentActionKind(treatment.ActionKind)
	processor := localizedMixTreatmentProcessor(treatment.ProcessorType)
	switch strings.ToLower(strings.TrimSpace(treatment.ActionKind)) {
	case "plugin_treatment":
		if processor == "" {
			processor = "插件"
			return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的插件处理候选；确认前不会写入任何插件参数。", target)
		}
		return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的 %s 处理候选；确认前不会写入任何插件参数。", target, processor)
	case "gain_balance":
		if treatment.DeltaDB != 0 {
			return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的电平调整候选，目标变化为 %.2f dB。", target, treatment.DeltaDB)
		}
		return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的电平调整候选。", target)
	case "pan_balance":
		if treatment.TargetPan != nil {
			return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的声像设置候选，目标声像为 %.2f。", target, *treatment.TargetPan)
		}
		if treatment.DeltaPan != 0 {
			return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的声像微调候选，变化量为 %.2f。", target, treatment.DeltaPan)
		}
		return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的声像调整候选。", target)
	default:
		if action != "" {
			return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的%s候选。", target, action)
		}
		return fmt.Sprintf("已根据当前观察结果为 %s 生成一个待确认的混音候选。", target)
	}
}

func localizedMixTreatmentEnglishReasoning(raw string, treatment agentloop.MixTreatmentPending) string {
	lower := strings.ToLower(raw)
	target := localizedMixTreatmentTargetRef(treatment.TargetRef)
	if strings.Contains(lower, "sub") || strings.Contains(lower, "bass") || strings.Contains(lower, "low-mid") || strings.Contains(lower, "low mid") {
		return fmt.Sprintf("已根据频段能量观察为 %s 生成一个保守的低频/低中频处理候选；确认前不会写入任何插件参数。", target)
	}
	if strings.Contains(lower, "pan") || strings.Contains(lower, "stereo") || strings.Contains(lower, "balance") || strings.Contains(lower, "correlation") {
		return fmt.Sprintf("已根据声像/立体声观察为 %s 生成一个待确认的声像处理候选；确认前不会修改工程。", target)
	}
	if strings.Contains(lower, "gain") || strings.Contains(lower, "level") || strings.Contains(lower, "loud") {
		return fmt.Sprintf("已根据电平观察为 %s 生成一个待确认的电平处理候选；确认前不会修改工程。", target)
	}
	return localizedMixTreatmentDefaultReasoning(treatment)
}

func looksMostlyEnglish(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	letters := 0
	asciiLetters := 0
	cjk := 0
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			letters++
			asciiLetters++
		case r >= 0x4e00 && r <= 0x9fff:
			letters++
			cjk++
		}
	}
	if cjk > 0 {
		return false
	}
	return asciiLetters >= 12 && asciiLetters*2 >= letters
}

var displayTextSubstitutions = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)\bpending_confirmation\b`), "待确认"},
	{regexp.MustCompile(`(?i)\bwaiting_for_user\b`), "等待用户确认"},
	{regexp.MustCompile(`(?i)\brequest_more_observation\b`), "需要更多观察"},
	{regexp.MustCompile(`(?i)\bdeferred\b`), "暂未展开"},
	{regexp.MustCompile(`(?i)\bready\b`), "就绪"},
	{regexp.MustCompile(`(?i)\bpartial\b`), "部分可用"},
	{regexp.MustCompile(`(?i)\bbuilding\b`), "准备中"},
	{regexp.MustCompile(`(?i)\bstale\b`), "已过期"},
	{regexp.MustCompile(`(?i)\bmissing\b`), "缺失"},
	{regexp.MustCompile(`(?i)\bfailed\b`), "失败"},
	{regexp.MustCompile(`(?i)\bsuspect\b`), "可疑"},
	{regexp.MustCompile(`(?i)\bunavailable\b`), "不可用"},
	{regexp.MustCompile(`(?i)\blow\b`), "低"},
	{regexp.MustCompile(`(?i)\bmedium\b`), "中"},
	{regexp.MustCompile(`(?i)\bhigh\b`), "高"},
}

var mixTreatmentAcousticTermSubstitutions = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`(?i)\blow[-_\s]?mid\b`), "低中频"},
	{regexp.MustCompile(`(?i)\bsub\b`), "超低频"},
	{regexp.MustCompile(`(?i)\bbass\b`), "低频"},
	{regexp.MustCompile(`(?i)\bmid\b`), "中频"},
	{regexp.MustCompile(`(?i)\bpresence\b`), "存在感频段"},
	{regexp.MustCompile(`(?i)\bair\b`), "空气感频段"},
	{regexp.MustCompile(`(?i)\bmasking\b`), "掩蔽分析"},
	{regexp.MustCompile(`(?i)\breference\b`), "参考匹配"},
	{regexp.MustCompile(`(?i)\bpeak\s*/\s*rms\b`), "峰值/RMS"},
}

func localizedMixTreatmentMixedAcousticTerms(text string) string {
	out := strings.TrimSpace(text)
	if out == "" {
		return ""
	}
	for _, item := range mixTreatmentAcousticTermSubstitutions {
		out = item.pattern.ReplaceAllString(out, item.replace)
	}
	return out
}

func localizedDisplayTextFallback(text string) string {
	out := strings.TrimSpace(text)
	if out == "" {
		return ""
	}
	out = localizedMixTreatmentMixedAcousticTerms(out)
	for _, item := range displayTextSubstitutions {
		out = item.pattern.ReplaceAllString(out, item.replace)
	}
	return out
}
