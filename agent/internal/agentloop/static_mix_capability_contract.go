package agentloop

import (
	"fmt"
	"strings"
)

const (
	staticMixCapabilityStatusNotStarted          = "not_started"
	staticMixCapabilityStatusObserved            = "observed"
	staticMixCapabilityStatusSuggested           = "suggested"
	staticMixCapabilityStatusPendingConfirmation = "pending_confirmation"
	staticMixCapabilityStatusApplied             = "applied"
	staticMixCapabilityStatusBlocked             = "blocked"
	staticMixCapabilityStatusSkipped             = "skipped"
	staticMixCapabilityStatusDeferred            = "deferred"
	staticMixCapabilityStatusNeedsReview         = "needs_review"
)

type staticMixCapabilitySpec struct {
	Stage     string
	ID        string
	Name      string
	NameEN    string
	Purpose   string
	Observe   string
	Execution string
}

func messageLoopStaticMixCapabilitySpecs() []staticMixCapabilitySpec {
	return []staticMixCapabilitySpec{
		{
			Stage:     "B1",
			ID:        "static_mix.gain_staging.v0",
			Name:      "Gain Staging",
			NameEN:    "Gain Staging",
			Purpose:   "建立安全电平和 headroom",
			Observe:   "project.state / mix.observe / mix.read",
			Execution: "mix.propose_tick -> mix.apply_tick -> track.volume 或 clip.gain.set",
		},
		{
			Stage:     "B2",
			ID:        "static_mix.static_balance.v0",
			Name:      "静态音量平衡",
			NameEN:    "Static Balance",
			Purpose:   "不依赖插件，先建立主次关系",
			Observe:   "project.state / mix.observe / mix.read",
			Execution: "mix.propose_tick -> mix.apply_tick -> track.volume",
		},
		{
			Stage:     "B3",
			ID:        "static_mix.pan_layout.v0",
			Name:      "声像布局",
			NameEN:    "Pan Layout",
			Purpose:   "中心元素、左右展开、宽度、mono 风险",
			Observe:   "project.state / mix.observe / mix.derive",
			Execution: "mix.propose_tick -> mix.apply_tick -> track.pan",
		},
		{
			Stage:     "B4",
			ID:        "static_mix.low_end_relation.v0",
			Name:      "低频关系",
			NameEN:    "Low-End Relation",
			Purpose:   "kick、bass、low synth、低频堆积和遮蔽",
			Observe:   "mix.observe / mix.read / mix.derive",
			Execution: "v0 优先观察和建议；证据足够时才进入待确认动作",
		},
		{
			Stage:     "B5",
			ID:        "static_mix.focus_position.v0",
			Name:      "核心元素定位",
			NameEN:    "Focus Position",
			Purpose:   "lead vocal / lead instrument / snare / bass 的前后关系",
			Observe:   "mix.observe / mix.read / mix.derive",
			Execution: "v0 优先观察和建议；静态定位可映射到 track.volume / track.pan",
		},
	}
}

func messageLoopStaticMixCapabilityContractRequest(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	if messageLoopProjectBlackboardAcousticObservationStatus(lower) && !messageLoopTextHasAny(lower, "能力", "capabilit", "contract", "能不能", "可以直接", "必须先") {
		return false
	}
	hasStaticMixRef := messageLoopTextHasAny(lower,
		"b阶段", "b 阶段", "b1", "b2", "b3", "b4", "b5",
		"粗混", "static mix", "static_mix",
		"gain staging", "静态音量", "声像布局", "低频关系", "核心元素定位",
		"static balance", "pan layout", "low-end relation", "low end relation", "focus position",
	)
	if !hasStaticMixRef {
		return false
	}
	return messageLoopTextHasAny(lower,
		"有哪些", "有什么", "内容", "能力", "capabilit", "contract", "契约",
		"状态", "做到哪", "进度", "能不能", "可以直接", "需要先", "必须先",
		"路线", "规划", "列表", "编号", "怎么做", "如何开发",
	)
}

func messageLoopStaticMixCapabilityContractReply(state *runState) string {
	lines := []string{"B Capability Contract v0", "", "结论"}
	appendMessageLoopBullet(&lines, "B 粗混 / Static Mix 是 static_mix capability 能力族，不是线性工作流。B1-B5 可以按用户目标独立调用，不互相阻塞。")

	lines = append(lines, "", "B 能力状态")
	messageLoopAppendStaticMixCapabilityContractStatusLines(&lines, state)

	lines = append(lines, "", "统一状态")
	appendMessageLoopBullet(&lines, strings.Join(messageLoopStaticMixCapabilityStatusNames(), " / "))

	lines = append(lines, "", "统一结果结构")
	appendMessageLoopBullet(&lines, "capability_id / capability_name / status / scope")
	appendMessageLoopBullet(&lines, "evidence_refs / observations / candidate_actions")
	appendMessageLoopBullet(&lines, "pending_action_refs / executor_result_refs / risk / limitation / recommended_next_step")

	lines = append(lines, "", "工具映射")
	appendMessageLoopBullet(&lines, "观察：project.state、mix.observe、mix.read、mix.derive")
	appendMessageLoopBullet(&lines, "建议与确认：mix.propose_tick、pending confirmation")
	appendMessageLoopBullet(&lines, "执行：mix.apply_tick、track.volume、track.pan、clip.gain.set")
	appendMessageLoopBullet(&lines, "回滚：mix.rollback_tick")

	lines = append(lines, "", "边界")
	appendMessageLoopBullet(&lines, "B1-B3 可以在 v0 中形成待确认动作；所有修改都必须等用户确认。")
	appendMessageLoopBullet(&lines, "B4-B5 先偏观察和建议，证据不足时不能伪造 EQ、压缩或空间处理。")
	appendMessageLoopBullet(&lines, "大工程使用 project-level compact observation，不做每轨 LLM 循环。")
	appendMessageLoopBullet(&lines, "A-F 与 B1-B5 都不作为线性门禁；A-F and B1-B5 are capability layers, not a linear gate.")
	return strings.Join(lines, "\n")
}

func messageLoopAppendStaticMixCapabilityStatusLines(lines *[]string, state *runState) {
	appendMessageLoopBullet(lines, "B 粗混 / Static Mix：能力族已建模；B1-B5 独立状态见下方；尚未记录为已执行")
	for _, spec := range messageLoopStaticMixCapabilitySpecs() {
		status := messageLoopStaticMixCapabilityStatus(spec, state)
		appendMessageLoopBullet(lines, fmt.Sprintf("%s %s（%s）：%s；%s", spec.Stage, spec.Name, spec.ID, messageLoopStaticMixCapabilityStatusLabel(status), spec.Purpose))
	}
}

func messageLoopAppendStaticMixCapabilityContractStatusLines(lines *[]string, state *runState) {
	appendMessageLoopBullet(lines, "B 粗混 / Static Mix：能力族已建模；B1-B5 独立状态见下方；尚未记录为已执行")
	for _, spec := range messageLoopStaticMixCapabilitySpecs() {
		status := messageLoopStaticMixCapabilityStatus(spec, state)
		appendMessageLoopBullet(lines, fmt.Sprintf("%s %s（%s）：%s；%s", spec.Stage, spec.NameEN, spec.ID, messageLoopStaticMixCapabilityStatusLabel(status), spec.Purpose))
	}
}

func messageLoopStaticMixCapabilityStatus(spec staticMixCapabilitySpec, state *runState) string {
	if status := messageLoopStaticMixCapabilityStatusFromState(spec, state); status != "" {
		return status
	}
	if state != nil && state.executionMemory.PendingMixTickCandidate != nil {
		op := strings.ToLower(strings.TrimSpace(state.executionMemory.PendingMixTickCandidate.Operation))
		switch spec.Stage {
		case "B2":
			if strings.Contains(op, "gain") || strings.Contains(op, "volume") {
				return staticMixCapabilityStatusPendingConfirmation
			}
		case "B3":
			if strings.Contains(op, "pan") {
				return staticMixCapabilityStatusPendingConfirmation
			}
		}
	}
	return staticMixCapabilityStatusNotStarted
}

func messageLoopStaticMixCapabilityStatusFromState(spec staticMixCapabilitySpec, state *runState) string {
	if state == nil {
		return ""
	}
	for _, source := range []map[string]any{state.input.Context, state.input.State, state.contextSnapshot} {
		if status := messageLoopStaticMixCapabilityStatusFromMap(spec, source); status != "" {
			return status
		}
	}
	return ""
}

func messageLoopStaticMixCapabilityStatusFromMap(spec staticMixCapabilitySpec, source map[string]any) string {
	if len(source) == 0 {
		return ""
	}
	for _, key := range []string{"static_mix_capabilities", "static_mix_capability_status", "capability_statuses"} {
		value := source[key]
		if status := messageLoopStaticMixCapabilityStatusFromAny(spec, value); status != "" {
			return status
		}
	}
	if blackboard := messageLoopMapValue(source["project_blackboard"]); len(blackboard) > 0 {
		return messageLoopStaticMixCapabilityStatusFromAny(spec, blackboard["static_mix_capabilities"])
	}
	return ""
}

func messageLoopStaticMixCapabilityStatusFromAny(spec staticMixCapabilitySpec, value any) string {
	if rows := messageLoopMapRows(value); len(rows) > 0 {
		for _, row := range rows {
			if messageLoopStaticMixCapabilityRowMatches(spec, row) {
				return messageLoopNormalizeStaticMixCapabilityStatus(firstMapText(row, "status", "state"))
			}
		}
		return ""
	}
	if byKey := messageLoopMapValue(value); len(byKey) > 0 {
		for _, key := range []string{spec.ID, spec.Stage, strings.ToLower(spec.Stage), spec.Name} {
			if row := messageLoopMapValue(byKey[key]); len(row) > 0 {
				if status := messageLoopNormalizeStaticMixCapabilityStatus(firstMapText(row, "status", "state")); status != "" {
					return status
				}
			}
			if status := messageLoopNormalizeStaticMixCapabilityStatus(messageLoopText(byKey[key])); status != "" {
				return status
			}
		}
	}
	return ""
}

func messageLoopStaticMixCapabilityRowMatches(spec staticMixCapabilitySpec, row map[string]any) bool {
	id := strings.ToLower(firstMapText(row, "capability_id", "id", "capability"))
	stage := strings.ToLower(firstMapText(row, "stage", "product_id", "node_id"))
	name := strings.ToLower(firstMapText(row, "name", "capability_name"))
	return id == strings.ToLower(spec.ID) ||
		stage == strings.ToLower(spec.Stage) ||
		strings.Contains(name, strings.ToLower(spec.Name))
}

func messageLoopNormalizeStaticMixCapabilityStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case staticMixCapabilityStatusNotStarted, "not started", "未开始", "尚未记录", "尚未记录为已执行":
		return staticMixCapabilityStatusNotStarted
	case staticMixCapabilityStatusObserved, "已观察":
		return staticMixCapabilityStatusObserved
	case staticMixCapabilityStatusSuggested, "已建议", "suggestion":
		return staticMixCapabilityStatusSuggested
	case staticMixCapabilityStatusPendingConfirmation, "pending", "needs_confirmation", "待确认":
		return staticMixCapabilityStatusPendingConfirmation
	case staticMixCapabilityStatusApplied, "done", "completed", "executed", "已执行", "已完成":
		return staticMixCapabilityStatusApplied
	case staticMixCapabilityStatusBlocked, "阻塞":
		return staticMixCapabilityStatusBlocked
	case staticMixCapabilityStatusSkipped, "skip", "跳过":
		return staticMixCapabilityStatusSkipped
	case staticMixCapabilityStatusDeferred, "defer", "延期":
		return staticMixCapabilityStatusDeferred
	case staticMixCapabilityStatusNeedsReview, "review", "needs review", "需要复查":
		return staticMixCapabilityStatusNeedsReview
	default:
		return ""
	}
}

func messageLoopStaticMixCapabilityStatusLabel(status string) string {
	switch status {
	case staticMixCapabilityStatusObserved:
		return "observed / 已观察"
	case staticMixCapabilityStatusSuggested:
		return "suggested / 已建议"
	case staticMixCapabilityStatusPendingConfirmation:
		return "pending_confirmation / 待确认"
	case staticMixCapabilityStatusApplied:
		return "applied / 已执行"
	case staticMixCapabilityStatusBlocked:
		return "blocked / 阻塞"
	case staticMixCapabilityStatusSkipped:
		return "skipped / 已跳过"
	case staticMixCapabilityStatusDeferred:
		return "deferred / 已延期"
	case staticMixCapabilityStatusNeedsReview:
		return "needs_review / 需要复查"
	default:
		return "not_started / 尚未记录为已执行"
	}
}

func messageLoopStaticMixCapabilityStatusNames() []string {
	return []string{
		staticMixCapabilityStatusNotStarted,
		staticMixCapabilityStatusObserved,
		staticMixCapabilityStatusSuggested,
		staticMixCapabilityStatusPendingConfirmation,
		staticMixCapabilityStatusApplied,
		staticMixCapabilityStatusBlocked,
		staticMixCapabilityStatusSkipped,
		staticMixCapabilityStatusDeferred,
		staticMixCapabilityStatusNeedsReview,
	}
}
