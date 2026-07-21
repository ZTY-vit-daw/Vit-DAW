// Package capabilityinteraction resolves conversational turns against one
// durable capability Proposal. It never creates Authorization or mutates a
// project; callers must separately validate and persist the typed decision.
package capabilityinteraction

import (
	"regexp"
	"strconv"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

var (
	maxChangePattern     = regexp.MustCompile(`(?i)(?:不超过|最多|最大)(?:调整|变化|变动|改动|移动)?\s*([0-9]+(?:\.[0-9]+)?)\s*(db|pan)?|(?:max(?:imum)?)(?:\s+(?:change|move|delta))?(?:\s+of)?\s*([0-9]+(?:\.[0-9]+)?)\s*(db|pan)?`)
	replaceChangePattern = regexp.MustCompile(`(?i)([+-][0-9]+(?:\.[0-9]+)?)\s*(?:db|pan)?\s*(?:改成|改为|换成|to)\s*([+-]?[0-9]+(?:\.[0-9]+)?)\s*(db|pan)?`)
	setChangePattern     = regexp.MustCompile(`(?i)(?:改成|改为|调到|设为|设置为|change\s+to|set\s+to)\s*([+-]?[0-9]+(?:\.[0-9]+)?)\s*(db|pan)?`)
)

func Resolve(message, sourceTurnID string, session orchestration.PlanningSession) orchestration.ApprovalDecision {
	decision := orchestration.ApprovalDecision{
		SchemaVersion: orchestration.ApprovalDecisionSchema,
		Kind:          orchestration.ApprovalNoDecision,
		SourceTurnID:  strings.TrimSpace(sourceTurnID), UserText: strings.TrimSpace(message), Confidence: "high",
	}
	if session.ActiveProposal == nil || session.FrozenPlan == nil {
		decision.Kind = orchestration.ApprovalAmbiguous
		decision.Reason = "no_active_frozen_proposal"
		return decision
	}
	proposal := *session.ActiveProposal
	decision.ProposalID = proposal.ID
	decision.ProposalRevision = proposal.Revision
	decision.ActionSetHash = proposal.ActionSetHash
	decision.ProjectCutHash = proposal.ProjectCutHash
	decision.ApprovedScope = append([]string(nil), proposal.TargetScope...)
	text := normalize(message)
	if text == "" {
		decision.Kind = orchestration.ApprovalAmbiguous
		decision.Reason = "empty_turn"
		return decision
	}
	if isQuestion(text) {
		decision.Kind = orchestration.ApprovalQuestion
		decision.Reason = "proposal_question"
		return decision
	}
	if isWholeReject(text) {
		decision.Kind = orchestration.ApprovalReject
		decision.Reason = "explicit_reject"
		return decision
	}
	if isRevision(text) {
		decision.Kind = orchestration.ApprovalRevise
		decision.Reason = "proposal_revision_requested"
		decision.Adjustments = resolveAdjustments(text, proposal.Presentation)
		for _, adjustment := range decision.Adjustments {
			if adjustment.Kind == "include_only" {
				decision.Kind = orchestration.ApprovalNarrow
			}
		}
		if len(decision.Adjustments) == 0 {
			decision.Unresolved = []string{"revision_instruction"}
			decision.Confidence = "low"
		}
		return decision
	}
	if isApproval(text) {
		decision.Kind = orchestration.ApprovalApprove
		decision.Reason = "explicit_conversational_approval"
		if isShortApproval(text) {
			decision.Confidence = "medium"
		}
		return decision
	}
	decision.Kind = orchestration.ApprovalAmbiguous
	decision.Reason = "pending_proposal_turn_not_explicit"
	decision.Confidence = "low"
	return decision
}

func resolveAdjustments(text string, presentation *orchestration.ProposalPresentation) []orchestration.ProposalAdjustment {
	out := []orchestration.ProposalAdjustment{}
	if candidate := candidateOrdinal(text); candidate > 0 {
		out = append(out, orchestration.ProposalAdjustment{Kind: "select_candidate", Candidate: candidate})
	}
	if hasAny(text, "不要动", "别动", "排除", "不要处理", "不处理", "exclude", "except", "don't touch", "do not touch") {
		refs, query := mentionedTargets(text, presentation)
		out = append(out, orchestration.ProposalAdjustment{Kind: "exclude_targets", TargetQuery: query, TargetRefs: refs})
	}
	if hasAny(text, "只处理", "只动", "仅处理", "只保留", "only", "only process", "only touch") {
		refs, query := mentionedTargets(text, presentation)
		out = append(out, orchestration.ProposalAdjustment{Kind: "include_only", TargetQuery: query, TargetRefs: refs})
	}
	if match := maxChangePattern.FindStringSubmatch(text); len(match) > 1 {
		valueText, unitText := match[1], match[2]
		if valueText == "" && len(match) > 4 {
			valueText, unitText = match[3], match[4]
		}
		value, _ := strconv.ParseFloat(valueText, 64)
		out = append(out, orchestration.ProposalAdjustment{Kind: "max_abs_delta", Value: value, Unit: normalizedUnit(unitText, presentation)})
	}
	if match := replaceChangePattern.FindStringSubmatch(text); len(match) > 2 {
		before, _ := strconv.ParseFloat(match[1], 64)
		after, _ := strconv.ParseFloat(match[2], 64)
		refs, query := mentionedTargets(text, presentation)
		out = append(out, orchestration.ProposalAdjustment{Kind: "replace_delta", TargetQuery: query, TargetRefs: refs, OriginalValue: before, Value: after, Unit: normalizedUnit(match[3], presentation)})
	} else if match := setChangePattern.FindStringSubmatch(text); len(match) > 1 {
		value, _ := strconv.ParseFloat(match[1], 64)
		refs, query := mentionedTargets(text, presentation)
		out = append(out, orchestration.ProposalAdjustment{Kind: "set_delta", TargetQuery: query, TargetRefs: refs, Value: value, Unit: normalizedUnit(match[2], presentation)})
	}
	return compactAdjustments(out)
}

func mentionedTargets(text string, presentation *orchestration.ProposalPresentation) ([]string, string) {
	if presentation == nil {
		return nil, ""
	}
	refs := []string{}
	queries := []string{}
	seen := map[string]bool{}
	for _, action := range presentation.Actions {
		if proposalActionMentioned(text, action) && !seen[action.TrackID] {
			seen[action.TrackID] = true
			refs = append(refs, action.TrackID)
			queries = append(queries, firstNonEmpty(action.TrackName, action.Role, action.Function, action.TrackID))
		}
	}
	return refs, strings.Join(uniqueStrings(queries), "、")
}

func proposalActionMentioned(text string, action orchestration.ProposalActionPreview) bool {
	for _, value := range []string{action.TrackID, action.TrackName, action.Role, action.Function} {
		value = normalize(value)
		if len([]rune(value)) >= 2 && strings.Contains(text, value) {
			return true
		}
	}
	aliases := map[string][]string{
		"bus": {"bus", "总线", "编组"}, "vocal": {"vocal", "人声", "主唱", "和声"},
		"background": {"background", "背景", "和声", "bgv", "bv"}, "drums": {"drum", "鼓", "打击乐", "percussion"},
		"bass": {"bass", "贝斯", "低频"}, "synth": {"synth", "合成器", "keys", "键盘"},
	}
	haystack := normalize(strings.Join([]string{action.TrackName, action.Role, action.Function}, " "))
	for key, words := range aliases {
		if !strings.Contains(haystack, key) {
			continue
		}
		for _, word := range words {
			if strings.Contains(text, word) {
				return true
			}
		}
	}
	return false
}

func compactAdjustments(in []orchestration.ProposalAdjustment) []orchestration.ProposalAdjustment {
	out := make([]orchestration.ProposalAdjustment, 0, len(in))
	for _, adjustment := range in {
		if (adjustment.Kind == "exclude_targets" || adjustment.Kind == "include_only") && len(adjustment.TargetRefs) == 0 {
			continue
		}
		out = append(out, adjustment)
	}
	return out
}

func isQuestion(text string) bool {
	return strings.Contains(text, "?") || strings.Contains(text, "？") || hasAny(text,
		"为什么", "为啥", "依据", "原因", "解释", "怎么判断", "风险是什么", "会怎样", "展开", "why", "explain", "basis", "evidence", "tell me more")
}

func isWholeReject(text string) bool {
	switch text {
	case "不要", "不用", "取消", "取消方案", "别执行", "不要执行", "算了", "先不做", "no", "cancel", "reject", "stop":
		return true
	default:
		return hasAny(text, "取消这个方案", "取消当前方案", "不要执行这个方案", "don't execute", "do not execute", "cancel this proposal")
	}
}

func isRevision(text string) bool {
	return candidateOrdinal(text) > 0 || hasAny(text,
		"但是", "但不要", "不要动", "别动", "排除", "只处理", "只动", "仅处理", "换个方案", "换一版", "改成", "改为", "调到", "设置为", "不超过", "最多",
		"but", "except", "exclude", "only", "revise", "change to", "set to", "maximum", "max ")
}

func isApproval(text string) bool {
	switch text {
	case "是的", "对", "对的", "没错", "确认", "可以", "好", "好的", "行", "执行", "应用", "继续", "可以执行", "确认执行", "继续执行", "可以继续", "yes", "y", "ok", "okay", "confirm", "execute", "apply", "continue", "do it":
		return true
	default:
		return hasAny(text, "执行这个方案", "执行当前方案", "应用这个方案", "确认这个方案", "确认执行这份", "确认执行此", "照这个方案执行", "按当前方案处理", "开始执行", "apply this proposal", "execute this proposal", "confirm and apply", "go ahead")
	}
}

func isShortApproval(text string) bool {
	return len([]rune(text)) <= 3
}

func candidateOrdinal(text string) int {
	for index, tokens := range [][]string{{"第一个", "第1个", "first"}, {"第二个", "第2个", "second"}, {"第三个", "第3个", "third"}} {
		for _, token := range tokens {
			if strings.Contains(text, token) && hasAny(text, "方案", "候选", "candidate", "option") {
				return index + 1
			}
		}
	}
	return 0
}

func normalizedUnit(value string, presentation *orchestration.ProposalPresentation) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		return value
	}
	if presentation != nil && strings.Contains(presentation.CapabilityID, "pan_layout") {
		return "pan"
	}
	return "dB"
}

func normalize(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func hasAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, strings.ToLower(strings.TrimSpace(value))) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
