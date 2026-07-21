package actionworkflow

import (
	"fmt"
	"strings"
)

const (
	IntentReadOnlyObservation = "read_only_observation"
	IntentActionPreflight     = "action_preflight"
	IntentPendingRevision     = "pending_revision"
	IntentConfirmationAccept  = "confirmation_accept"
	IntentConfirmationReject  = "confirmation_reject"
	IntentFollowupQuestion    = "followup_question"
	IntentABResultObservation = "ab_result_observation"
)

const (
	DecisionAccept     = "accept"
	DecisionReject     = "reject"
	DecisionRevision   = "revision"
	DecisionFollowup   = "followup_question"
	DecisionNewRequest = "new_request"
	DecisionNone       = "none"
)

type IntentInput struct {
	UserText         string
	HasActivePending bool
	HasObservation   bool
}

type IntentDecision struct {
	Intent string
	Reason string
}

type ConfirmationDecision struct {
	Kind   string
	Intent string
	Reason string
}

type PreflightGate struct {
	CanCreateExecutablePending bool
	CanSuggest                 bool
	Status                     string
	EvidenceRefs               []string
	BlockedReasons             []string
	Limitations                []string
}

type PendingSpec struct {
	TargetRef       string
	ActionKind      string
	ProcessorType   string
	DeltaDB         float64
	DeltaPan        float64
	TargetPan       *float64
	PluginID        string
	PluginName      string
	Control         string
	Target          map[string]any
	Reasoning       string
	Confidence      string
	EvidenceRefs    []string
	NeedsResolution []string
	ObservationID   string
	Intent          string
}

type DisplayModel struct {
	Conclusion      string
	Evidence        string
	PendingAction   string
	Limitations     string
	CardTitle       string
	CardBody        string
	StatusLabel     string
	TargetLabel     string
	ActionLabel     string
	ProcessorLabel  string
	ConfidenceLabel string
}

func RouteIntent(input IntentInput) IntentDecision {
	text := normalize(input.UserText)
	if text == "" {
		return IntentDecision{Intent: IntentFollowupQuestion, Reason: "empty_user_text"}
	}
	if decision := ClassifyConfirmation(input.UserText, input.HasActivePending); input.HasActivePending && decision.Kind != DecisionNone {
		return IntentDecision{Intent: decision.Intent, Reason: decision.Reason}
	}
	if containsAny(text, "ab result", "a/b", "before after", "before/after", "对比结果", "前后对比", "ab 结果", "AB 结果") {
		return IntentDecision{Intent: IntentABResultObservation, Reason: "ab_result_request"}
	}
	if readOnlyObservationText(text) {
		return IntentDecision{Intent: IntentReadOnlyObservation, Reason: "explicit_read_only_observation"}
	}
	if actionPreflightText(text) {
		return IntentDecision{Intent: IntentActionPreflight, Reason: "mix_action_request"}
	}
	if input.HasObservation && followupQuestionText(text) {
		return IntentDecision{Intent: IntentFollowupQuestion, Reason: "observation_followup_question"}
	}
	return IntentDecision{Intent: IntentFollowupQuestion, Reason: "default_followup"}
}

func ClassifyConfirmation(message string, hasActivePending bool) ConfirmationDecision {
	text := normalize(message)
	if text == "" {
		return ConfirmationDecision{Kind: DecisionNone}
	}
	if !hasActivePending {
		if acceptText(text) {
			return ConfirmationDecision{Kind: DecisionNone, Reason: "approval_without_pending"}
		}
		return ConfirmationDecision{Kind: DecisionNone}
	}
	if followupQuestionText(text) {
		return ConfirmationDecision{Kind: DecisionFollowup, Intent: IntentFollowupQuestion, Reason: "pending_followup_question"}
	}
	if revisionText(text) {
		return ConfirmationDecision{Kind: DecisionRevision, Intent: IntentPendingRevision, Reason: "pending_revision"}
	}
	if rejectText(text) {
		return ConfirmationDecision{Kind: DecisionReject, Intent: IntentConfirmationReject, Reason: "pending_reject"}
	}
	if acceptText(text) {
		if hasActivePending {
			return ConfirmationDecision{Kind: DecisionAccept, Intent: IntentConfirmationAccept, Reason: "pending_accept"}
		}
		return ConfirmationDecision{Kind: DecisionNone, Reason: "approval_without_pending"}
	}
	return ConfirmationDecision{Kind: DecisionNone}
}

func GateFromMOMProjection(proj map[string]any) PreflightGate {
	gate := PreflightGate{Status: "missing"}
	if len(proj) == 0 {
		gate.BlockedReasons = []string{"missing_mom_projection"}
		gate.Limitations = []string{"缺少 MOM action preflight 投影，不能创建可执行 pending。"}
		return gate
	}
	trust := mapValue(proj["trust_quality"])
	if len(trust) == 0 {
		gate.BlockedReasons = []string{"missing_trust_quality"}
		gate.Limitations = []string{"MOM projection 缺少 trust_quality，不能创建可执行 pending。"}
		gate.EvidenceRefs = evidenceRefsFromProjection(proj)
		return gate
	}
	gate.Status = firstText(trust, "overall_status", "status")
	gate.CanCreateExecutablePending = boolValue(trust["can_support_action_preflight"])
	gate.CanSuggest = boolValue(trust["can_support_suggestion"]) || gate.CanCreateExecutablePending
	gate.EvidenceRefs = stringSlice(firstPresent(trust, "evidence_refs", "source_refs"))
	if len(gate.EvidenceRefs) == 0 {
		gate.EvidenceRefs = evidenceRefsFromProjection(proj)
	}
	gate.BlockedReasons = stringSlice(trust["blocked_reasons"])
	gate.Limitations = stringSlice(trust["limitations"])
	if !gate.CanCreateExecutablePending {
		if len(gate.BlockedReasons) == 0 {
			gate.BlockedReasons = []string{"mom_trust_quality_cannot_support_action_preflight"}
		}
		if len(gate.Limitations) == 0 {
			gate.Limitations = []string{"MOM trust_quality 不足，只能给建议或澄清，不能创建可执行 pending。"}
		}
	}
	return gate
}

func RenderPending(spec PendingSpec, gate PreflightGate) DisplayModel {
	target := targetLabel(spec.TargetRef)
	action := actionLabel(spec.ActionKind, spec)
	processor := processorLabel(spec.ProcessorType)
	confidence := confidenceLabel(spec.Confidence)
	evidence := evidenceSummary(spec, gate)
	limitations := limitationSummary(spec, gate)
	conclusion := conclusionSummary(spec, target, action, processor)
	pendingAction := pendingActionSummary(spec, target, action, processor)
	cardBody := strings.Join(nonEmpty([]string{
		"结论：" + conclusion,
		"依据：" + evidence,
		"待确认动作：" + pendingAction,
		"限制：" + limitations,
	}), "\n")
	return DisplayModel{
		Conclusion:      conclusion,
		Evidence:        evidence,
		PendingAction:   pendingAction,
		Limitations:     limitations,
		CardTitle:       "混音建议待确认",
		CardBody:        cardBody,
		StatusLabel:     "待确认",
		TargetLabel:     target,
		ActionLabel:     action,
		ProcessorLabel:  processor,
		ConfidenceLabel: confidence,
	}
}

func RenderReply(spec PendingSpec, gate PreflightGate) string {
	display := RenderPending(spec, gate)
	return strings.Join(nonEmpty([]string{
		"结论：" + display.Conclusion,
		"依据：" + display.Evidence,
		"待确认动作：" + display.PendingAction,
		"限制：" + display.Limitations,
	}), "\n\n")
}

func RenderBlockedPreflightReply(gate PreflightGate) string {
	evidence := "当前 MOM trust_quality 不足。"
	if len(gate.EvidenceRefs) > 0 {
		evidence = "可引用证据：" + strings.Join(firstN(gate.EvidenceRefs, 4), "、") + "。"
	}
	limit := "不会创建可执行 pending，也不会写插件、参数、音量或声像。"
	if len(gate.Limitations) > 0 {
		limit = strings.Join(firstN(gate.Limitations, 3), "；") + "。"
	}
	return strings.Join([]string{
		"结论：这次观察只能支持建议或澄清，还不能进入可执行确认。",
		"依据：" + evidence,
		"待确认动作：无。",
		"限制：" + limit,
	}, "\n\n")
}

func normalize(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func readOnlyObservationText(text string) bool {
	hasObservation := containsAny(text, "观察", "看看", "看一下", "分析", "检查", "比较", "observe", "inspect", "analyze", "analysis", "compare")
	hasReadOnly := containsAny(text, "不要修改", "别修改", "不要动", "别动", "只观察", "只读", "不修改", "不要执行", "do not modify", "don't modify", "read only", "read-only", "observe only")
	return hasObservation && hasReadOnly
}

func actionPreflightText(text string) bool {
	if readOnlyObservationText(text) {
		return false
	}
	hasAction := containsAny(text,
		"帮我", "处理", "调整", "调一下", "收一点", "收一下", "降低", "压低", "减少", "削减", "削一点", "提高", "提升", "增强", "加一点", "靠前",
		"执行", "应用", "确认后", "准备", "方案",
		"help me", "process", "treat", "adjust", "reduce", "lower", "cut", "trim", "raise", "boost", "increase", "apply", "execute", "prepare",
	)
	hasMixTarget := containsAny(text,
		"混音", "缩混", "低频", "低中频", "糊", "浑浊", "声像", "声相", "声场", "音量", "电平", "增益", "响度", "人声", "主唱", "轨道", "eq", "压缩", "混响",
		"mix", "mixing", "low end", "low-end", "low mid", "mud", "muddy", "pan", "panning", "stereo", "volume", "level", "gain", "loudness", "vocal", "track", "compress", "reverb",
	)
	return hasAction && hasMixTarget
}

func acceptText(text string) bool {
	switch strings.TrimSpace(text) {
	case "是的", "对", "对的", "没错", "确认", "确认一下", "可以", "好", "好的", "行", "执行", "应用", "继续", "可以执行", "确认执行", "继续执行", "可以继续", "可以，继续", "可以, 继续",
		"yes", "y", "ok", "okay", "confirm", "execute", "apply", "continue", "do it":
		return true
	default:
		return containsAny(text, "可以执行", "确认执行", "继续执行", "执行吧", "执行这个", "应用这个", "应用调整", "按这个调", "按你说的调", "就按这个调", "开始执行", "apply it", "apply this", "execute it", "confirm and apply", "go ahead and apply")
	}
}

func rejectText(text string) bool {
	switch strings.TrimSpace(text) {
	case "不要", "不用", "取消", "别执行", "不要执行", "先别", "算了", "no", "cancel", "stop":
		return true
	default:
		return containsAny(text, "不要执行", "别执行", "先别执行", "取消这个", "不用了", "别改", "不要改", "don't apply", "do not apply", "cancel it")
	}
}

func revisionText(text string) bool {
	if containsAny(text, "为什么", "为啥", "原因", "解释", "依据", "why", "explain", "basis") {
		return false
	}
	return containsAny(text,
		"换个方案", "换一版", "改成", "改为", "换成", "不要这个", "不是这个", "调到", "设置为", "设为", "重新来",
		"change to", "switch to", "instead", "not that", "replace", "revise", "set to", "make it",
	)
}

func followupQuestionText(text string) bool {
	return containsAny(text,
		"为什么", "为啥", "依据", "根据", "原因", "解释", "怎么判断", "风险", "会怎样", "听感", "先解释", "继续说", "展开说",
		"why", "basis", "evidence", "explain", "tell me more", "risk",
	)
}

func containsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(needle))) {
			return true
		}
	}
	return false
}

func targetLabel(targetRef string) string {
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

func actionLabel(actionKind string, spec PendingSpec) string {
	switch strings.ToLower(strings.TrimSpace(actionKind)) {
	case "gain_balance":
		if spec.DeltaDB != 0 {
			return fmt.Sprintf("电平调整 %.2f dB", spec.DeltaDB)
		}
		return "电平平衡"
	case "pan_balance":
		if spec.TargetPan != nil {
			return fmt.Sprintf("声像设置到 %.2f", *spec.TargetPan)
		}
		if spec.DeltaPan != 0 {
			return fmt.Sprintf("声像微调 %.2f", spec.DeltaPan)
		}
		return "声像调整"
	case "plugin_treatment":
		return "插件处理"
	case "ask_clarification":
		return "需要澄清"
	default:
		return "混音处理"
	}
}

func processorLabel(processor string) string {
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

func confidenceLabel(confidence string) string {
	switch strings.ToLower(strings.TrimSpace(confidence)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	default:
		return strings.TrimSpace(confidence)
	}
}

func conclusionSummary(spec PendingSpec, target, action, processor string) string {
	if strings.TrimSpace(spec.Reasoning) != "" && !mostlyEnglish(spec.Reasoning) {
		return strings.TrimSpace(spec.Reasoning)
	}
	if localized := localizedEnglishReasoningSummary(spec, target, processor); localized != "" {
		return localized
	}
	if processor != "" && strings.EqualFold(spec.ActionKind, "plugin_treatment") {
		return fmt.Sprintf("可以先为 %s 准备一个保守的 %s 处理候选。", target, processor)
	}
	return fmt.Sprintf("可以先为 %s 准备一个待确认的 %s 候选。", target, action)
}

func localizedEnglishReasoningSummary(spec PendingSpec, target, processor string) string {
	reasoning := strings.ToLower(strings.TrimSpace(spec.Reasoning))
	if reasoning == "" || !mostlyEnglish(reasoning) {
		return ""
	}
	if strings.Contains(reasoning, "low-end") || strings.Contains(reasoning, "low end") || strings.Contains(reasoning, "low-mid") || strings.Contains(reasoning, "low mid") || strings.Contains(reasoning, "mud") {
		if processor == "" {
			processor = "插件"
		}
		return fmt.Sprintf("可以先为 %s 准备一个保守的低频/低中频 %s 处理候选。", target, processor)
	}
	if strings.Contains(reasoning, "pan") || strings.Contains(reasoning, "stereo") || strings.Contains(reasoning, "balance") {
		return fmt.Sprintf("可以先为 %s 准备一个待确认的声像/立体声处理候选。", target)
	}
	if strings.Contains(reasoning, "gain") || strings.Contains(reasoning, "level") || strings.Contains(reasoning, "loud") {
		return fmt.Sprintf("可以先为 %s 准备一个待确认的电平处理候选。", target)
	}
	return ""
}

func evidenceSummary(spec PendingSpec, gate PreflightGate) string {
	refs := append([]string(nil), spec.EvidenceRefs...)
	if len(refs) == 0 {
		refs = gate.EvidenceRefs
	}
	if len(refs) == 0 && strings.TrimSpace(spec.ObservationID) != "" {
		refs = []string{spec.ObservationID}
	}
	if len(refs) == 0 {
		return "已读取 MOM projection；没有可展示的 evidence ref。"
	}
	return "来自 MOM action preflight / observation evidence refs：" + strings.Join(firstN(refs, 4), "、") + "。"
}

func pendingActionSummary(spec PendingSpec, target, action, processor string) string {
	switch strings.ToLower(strings.TrimSpace(spec.ActionKind)) {
	case "plugin_treatment":
		if processor != "" {
			return fmt.Sprintf("确认后只进入 %s 的安全准备/执行路线，不直接写参数；目标是 %s。", processor, target)
		}
		return fmt.Sprintf("确认后只进入插件安全准备/执行路线，不直接写参数；目标是 %s。", target)
	case "gain_balance", "pan_balance":
		return fmt.Sprintf("确认后通过 typed mix tick 对 %s 执行%s。", target, action)
	default:
		return fmt.Sprintf("确认后通过 typed executor 处理 %s。", target)
	}
}

func limitationSummary(spec PendingSpec, gate PreflightGate) string {
	parts := []string{}
	if !gate.CanCreateExecutablePending && len(gate.BlockedReasons) > 0 {
		parts = append(parts, "trust_quality 未完全支持 action preflight")
	}
	if len(spec.NeedsResolution) > 0 {
		parts = append(parts, "仍需解析："+strings.Join(firstN(spec.NeedsResolution, 4), "、"))
	}
	if len(parts) == 0 {
		parts = append(parts, "确认前不会写插件、参数、音量或声像。")
	} else {
		parts = append(parts, "确认前不会写插件、参数、音量或声像。")
	}
	return strings.Join(parts, "；")
}

func evidenceRefsFromProjection(proj map[string]any) []string {
	refs := stringSlice(proj["evidence_refs"])
	if len(refs) > 0 {
		return refs
	}
	if llm := mapValue(proj["llm_context"]); len(llm) > 0 {
		refs = stringSlice(llm["evidence_refs"])
	}
	return refs
}

func firstN(values []string, max int) []string {
	if max <= 0 || len(values) <= max {
		return values
	}
	return values[:max]
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func mostlyEnglish(text string) bool {
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

func mapValue(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	return nil
}

func firstPresent(row map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value
		}
	}
	return nil
}

func firstText(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, item := range parts {
			if s := strings.TrimSpace(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "ready":
			return true
		default:
			return false
		}
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(value)), "true")
	}
}
