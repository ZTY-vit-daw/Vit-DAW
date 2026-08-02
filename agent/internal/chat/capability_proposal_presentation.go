package chat

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/capabilitycontext"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/panlayout"
	"vit-daw-agent/internal/staticbalance"
)

func staticBalanceProposalPresentation(proposal orchestration.Proposal, pack capabilitycontext.StaticBalancePack, candidateID string, actionSet orchestration.ActionSet) *orchestration.ProposalPresentation {
	result := pack.Result()
	candidate, ok := staticBalanceCandidate(result.Candidates, candidateID)
	if !ok {
		return nil
	}
	coverage := pack.Admission.Coverage
	presentation := &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema,
		ProposalID:    proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID,
		Title:      "B2 静态主次与音量平衡方案",
		Conclusion: fmt.Sprintf("已分析 %d 条轨道，推荐以 %s 策略调整 %d 条轨道。", result.AnalyzedTrackCount, candidate.Label, len(candidate.Actions)),
		AnalysisSummary: []string{
			fmt.Sprintf("角色覆盖 %.0f%%，有效电平覆盖 %.0f%%，可比较电平覆盖 %.0f%%。", coverage.RoleCoverage*100, coverage.EffectiveLevelCoverage*100, coverage.LevelCoverage*100),
			fmt.Sprintf("识别 %d 个音乐功能组，%d 条轨道满足 B2 求解条件。", coverage.FunctionCount, coverage.EligibleTrackCount),
			"B2 使用静态有效电平、当前推子与角色关系；频谱和立体声不是本次决策的必需证据。",
		},
		Recommendation: fmt.Sprintf("推荐候选“%s”，置信度 %s，最大单轨变化 %.2f dB。", candidate.Label, proposalConfidenceLabel(candidate.Confidence), candidate.MaxAbsDeltaDB),
		AnalyzedTracks: result.AnalyzedTrackCount, ActionCount: len(actionSet.Actions), Risk: proposal.Risk, Reversible: true,
		Readiness: []orchestration.ProposalMetric{
			{ID: "role_coverage", Label: "角色覆盖", Value: percentText(coverage.RoleCoverage), Status: metricStatus(coverage.RoleCoverage, .55)},
			{ID: "effective_level_coverage", Label: "有效电平覆盖", Value: percentText(coverage.EffectiveLevelCoverage), Status: metricStatus(coverage.EffectiveLevelCoverage, .95)},
			{ID: "fader_coverage", Label: "推子覆盖", Value: fmt.Sprintf("%d/%d", coverage.FaderKnownCount, coverage.RoleCandidateCount), Status: countMetricStatus(coverage.FaderKnownCount, coverage.RoleCandidateCount)},
		},
		Limitations: proposalLimitations(candidate.Limitations, result.Limitations), EvidenceRefs: firstProposalRefs(result.EvidenceRefs, 12),
		ApprovalPrompt: "你可以直接回复“执行这个方案”、提出问题、要求排除/只处理某些轨道，或取消。",
	}
	for _, group := range candidate.FunctionSummary {
		presentation.ChangeGroups = append(presentation.ChangeGroups, orchestration.ProposalChangeGroup{
			ID: "function:" + group.Function, Label: proposalFunctionLabel(group.Function), Function: group.Function,
			TrackCount: group.TrackCount, MoveCount: group.MoveCount, MinValue: group.MinDeltaDB, MaxValue: group.MaxDeltaDB, Unit: "dB",
		})
	}
	for index, action := range candidate.Actions {
		actionID := ""
		if index < len(actionSet.Actions) {
			actionID = actionSet.Actions[index].ID
		}
		presentation.Actions = append(presentation.Actions, orchestration.ProposalActionPreview{
			ActionID: actionID, TrackID: action.TrackID, TrackName: action.TrackName, Role: action.Role, Function: action.Function,
			Operation: action.Operation, Before: action.BeforeDB, Target: action.TargetDB, Delta: action.DeltaDB, Unit: "dB", Reason: action.Reason,
		})
	}
	return presentation
}

func panLayoutProposalPresentation(proposal orchestration.Proposal, pack capabilitycontext.PanLayoutPack, candidateID string, actionSet orchestration.ActionSet) *orchestration.ProposalPresentation {
	result := pack.Result()
	candidate, ok := panLayoutCandidate(result.Candidates, candidateID)
	if !ok {
		return nil
	}
	coverage := pack.Admission.Coverage
	presentation := &orchestration.ProposalPresentation{
		SchemaVersion: orchestration.ProposalPresentationSchema,
		ProposalID:    proposal.ID, ProposalRevision: proposal.Revision, CapabilityID: proposal.CapabilityID,
		Title:      "B3 声像布局方案",
		Conclusion: fmt.Sprintf("已分析 %d 条轨道，推荐以 %s 策略调整 %d 条轨道的声像。", result.AnalyzedTrackCount, candidate.Label, len(candidate.Actions)),
		AnalysisSummary: []string{
			fmt.Sprintf("角色覆盖 %.0f%%，当前 Pan 覆盖 %.0f%%，声道信息覆盖 %.0f%%。", coverage.RoleCoverage*100, coverage.PanCoverage*100, coverage.ChannelCoverage*100),
			fmt.Sprintf("当前布局：左 %d、中心 %d、右 %d；识别 %d 个可成对布局轨道。", pack.LayoutSummary.LeftCount, pack.LayoutSummary.CenterCount, pack.LayoutSummary.RightCount, coverage.PairEligibleTrackCount),
			fmt.Sprintf("立体声关系覆盖 %.0f%%；证据不足的高风险立体声源保持不变。", coverage.StereoEvidenceCoverage*100),
		},
		Recommendation: fmt.Sprintf("推荐候选“%s”，置信度 %s，最大绝对 Pan %.2f。", candidate.Label, proposalConfidenceLabel(candidate.Confidence), candidate.MaxAbsPan),
		AnalyzedTracks: result.AnalyzedTrackCount, ActionCount: len(actionSet.Actions), Risk: proposal.Risk, Reversible: true,
		Readiness: []orchestration.ProposalMetric{
			{ID: "role_coverage", Label: "角色覆盖", Value: percentText(coverage.RoleCoverage), Status: metricStatus(coverage.RoleCoverage, .55)},
			{ID: "pan_coverage", Label: "Pan 覆盖", Value: percentText(coverage.PanCoverage), Status: metricStatus(coverage.PanCoverage, .95)},
			{ID: "stereo_evidence", Label: "立体声证据", Value: percentText(coverage.StereoEvidenceCoverage), Status: optionalMetricStatus(coverage.StereoEvidenceCoverage, .25)},
		},
		Limitations: proposalLimitations(candidate.Limitations, result.Limitations), EvidenceRefs: firstProposalRefs(result.EvidenceRefs, 12),
		ApprovalPrompt: "你可以直接回复“执行这个方案”、询问布局依据、要求排除/只处理某些轨道，或取消。",
	}
	for _, group := range candidate.GroupSummary {
		presentation.ChangeGroups = append(presentation.ChangeGroups, orchestration.ProposalChangeGroup{
			ID: "role:" + group.Role, Label: proposalRoleLabel(group.Role), Role: group.Role,
			TrackCount: group.TrackCount, MoveCount: group.MoveCount, MinValue: group.MinPan, MaxValue: group.MaxPan, Unit: "pan",
		})
	}
	for index, action := range candidate.Actions {
		actionID := ""
		if index < len(actionSet.Actions) {
			actionID = actionSet.Actions[index].ID
		}
		presentation.Actions = append(presentation.Actions, orchestration.ProposalActionPreview{
			ActionID: actionID, TrackID: action.TrackID, TrackName: action.TrackName, Role: action.Role, Function: action.Function,
			Operation: action.Operation, Before: action.BeforePan, Target: action.TargetPan, Delta: action.DeltaPan, Unit: "pan", Reason: action.Reason,
		})
	}
	return presentation
}

func renderProposalConversation(p *orchestration.ProposalPresentation) string {
	if p == nil {
		return "方案已生成，等待确认；确认前不会修改工程。"
	}
	lines := []string{p.Title, "", p.Conclusion}
	if len(p.AnalysisSummary) > 0 {
		lines = append(lines, "", "分析依据")
		for _, item := range p.AnalysisSummary {
			lines = append(lines, "- "+item)
		}
	}
	if p.Recommendation != "" {
		lines = append(lines, "", "推荐方案", "- "+p.Recommendation)
	}
	if len(p.ChangeGroups) > 0 {
		lines = append(lines, "", "修改预览")
		for _, group := range p.ChangeGroups {
			lines = append(lines, "- "+proposalGroupText(group))
		}
	}
	if len(p.Limitations) > 0 {
		lines = append(lines, "", "限制与风险")
		for _, item := range firstProposalRefs(p.Limitations, 4) {
			lines = append(lines, "- "+item)
		}
	}
	lines = append(lines, "", fmt.Sprintf("Proposal %s · revision %d · %d 个可逆动作", p.ProposalID, p.ProposalRevision, p.ActionCount))
	if p.ApprovalPrompt != "" {
		lines = append(lines, p.ApprovalPrompt)
	}
	return strings.Join(lines, "\n")
}

func presentationConclusion(p *orchestration.ProposalPresentation) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.Conclusion)
}

func proposalGroupText(group orchestration.ProposalChangeGroup) string {
	return fmt.Sprintf("%s：%d 轨，%s", group.Label, group.MoveCount, formatProposalRange(group.MinValue, group.MaxValue, group.Unit))
}

func formatProposalRange(minimum, maximum float64, unit string) string {
	if math.Abs(minimum-maximum) < .0005 {
		return fmt.Sprintf("%+.2f %s", minimum, unit)
	}
	return fmt.Sprintf("%+.2f 至 %+.2f %s", minimum, maximum, unit)
}

func staticBalanceCandidate(candidates []staticbalance.CandidatePlan, id string) (staticbalance.CandidatePlan, bool) {
	for _, candidate := range candidates {
		if candidate.CandidatePlanID == id {
			return candidate, true
		}
	}
	return staticbalance.CandidatePlan{}, false
}

func panLayoutCandidate(candidates []panlayout.CandidatePlan, id string) (panlayout.CandidatePlan, bool) {
	for _, candidate := range candidates {
		if candidate.CandidatePlanID == id {
			return candidate, true
		}
	}
	return panlayout.CandidatePlan{}, false
}

func proposalLimitations(groups ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, values := range groups {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, proposalLimitationText(value))
		}
	}
	sort.Strings(out)
	return out
}

func proposalLimitationText(value string) string {
	switch value {
	case "some_tracks_missing_band_or_stereo_evidence":
		return "部分轨道缺少频谱或立体声证据；不影响 B2 静态电平决策，但会限制更深层听感判断。"
	case "no_material_fader_moves_after_bounded_solver":
		return "安全边界内没有形成需要执行的推子变化。"
	case "no_material_pan_moves_after_bounded_solver":
		return "安全边界内没有形成需要执行的声像变化。"
	default:
		return strings.ReplaceAll(value, "_", " ")
	}
}

func proposalFunctionLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "focus", "lead", "foreground":
		return "核心/前景元素"
	case "support", "secondary":
		return "支撑元素"
	case "background", "bed":
		return "背景元素"
	case "rhythm":
		return "节奏元素"
	default:
		return firstNonEmpty(strings.TrimSpace(value), "其他元素")
	}
}

func proposalRoleLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "其他角色"
	}
	return strings.ReplaceAll(value, "_", " ")
}

func proposalConfidenceLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "低"
	default:
		return firstNonEmpty(value, "未知")
	}
}

func percentText(value float64) string { return fmt.Sprintf("%.0f%%", value*100) }

func metricStatus(value, threshold float64) string {
	if value >= threshold {
		return "ready"
	}
	return "blocked"
}

func optionalMetricStatus(value, threshold float64) string {
	if value >= threshold {
		return "ready"
	}
	return "limited"
}

func countMetricStatus(known, total int) string {
	if total > 0 && float64(known)/float64(total) >= .95 {
		return "ready"
	}
	return "blocked"
}

func firstProposalRefs(values []string, max int) []string {
	if max <= 0 || len(values) <= max {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:max]...)
}
