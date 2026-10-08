package agentloop

// exit_retain.go — run 终态观察结论跨会话延伸（L1-4-IMPL-D D1 腿 2）。
//
// CONTEXT_LAYERING_V1_DESIGN §4.2 归并表行 3 的字面落地：自由态观察账本
// （free_state_reasoning_loop.observation_ledger，活在 loop 生命周期）是
// retain 态的既有实现；L4 工程账本（ledger/project_ledger.v1.jsonl）是其
// 结论级条目的跨会话延伸。挂点=Runner 终态漏斗（r.result 的 cont=nil 分
// 支）——run 结束时观察账本随 loop 消亡，结论行经退场执行器 retain 进工程
// 账本（经 IMPL-B 写入器 append-only 落盘）；暂停面（cont != nil）不触发，
// 账本随 input.Context 进 continuation 存续。
//
// 轮次语义不新造：终态收尾是一个 TurnBoundaryEvent，TurnID=RunID；喂给
// 单元用观察账本自身的字段（observation_id/conclusion/target_ref），单元
// TurnID 标 RunID → turn_end 判据按既有机械规则触发。语句级去重：同一
// conclusion 已在工程账本（Kind=observation_conclusion 且语句全等）则跳过
// ——重跑/续跑不重复入账（append-only 语义下去重是供给面义务，写入器不管）。

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/contextruntime"
	"vit-daw-agent/internal/contextruntime/carriers"
)

// runProjectDirFromState 提取 run 上下文的工程路径（键序对齐 chat 侧
// projectPathFromChatContext：project_path / current_project_path）。
func runProjectDirFromState(state *runState) string {
	if state == nil {
		return ""
	}
	for _, key := range []string{"project_path", "current_project_path"} {
		if dir := exitRetainText(state.input.Context[key]); dir != "" {
			return dir
		}
	}
	return ""
}

// exitRetainText 把任意标量值转成 trim 后文本（非字符串/空值返回空）。
func exitRetainText(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

// retainRunObservationConclusions 在 run 终态把观察账本结论行 retain 进
// 工程 L4 账本。返回 trace 事件文本（无结论/无工程面/advisory 形态返回空
// ——不产生噪音事件）；失败不阻断 Result 构造，但必须显式留痕（违反可见
// 纪律：retain 落盘失败=结论跨会话延伸丢失）。
func (r *Runner) retainRunObservationConclusions(state *runState) string {
	if state == nil {
		return ""
	}
	ledger := messageLoopMapValue(messageLoopFreeStateContext(state)["observation_ledger"])
	if len(ledger) == 0 {
		return ""
	}
	runID := strings.TrimSpace(state.goal.RunID)
	units := observationConclusionUnits(ledger, runID)
	if len(units) == 0 {
		return ""
	}
	projectDir := runProjectDirFromState(state)
	if projectDir == "" {
		// 无工程面（无落盘点）——advisory：决策只进报告不落盘。
		return fmt.Sprintf("exit_retain advisory: %d observation conclusions, no project dir (retains not persisted)", len(units))
	}
	units = dedupeAgainstProjectLedger(projectDir, units)
	if len(units) == 0 {
		return ""
	}
	hook := contextruntime.RunTurnBoundaryHook(context.Background(), contextruntime.TurnBoundaryHookInput{
		SessionKey: "run_exit:" + runID,
		TurnID:     runID,
		ExtraUnits: units,
		ProjectDir: projectDir,
	})
	return fmt.Sprintf("exit_retain: %d conclusions retained to project ledger, %d violations", hook.RetainsWritten, len(hook.ExitViolations))
}

// observationConclusionUnits 从观察账本 available_views 提取带结论的行
// （确定性序：按 view 键排序）。EvidenceRefs 携带行内 target_ref（观察对象
// 指针）；票本身可重拉（F6），HandleRef 留空——retain 优先于 ref（§5.4
// 默认退场=retain+drop，结论入账本后原文可弃）。
func observationConclusionUnits(ledger map[string]any, turnID string) []contextruntime.WindowUnit {
	available := messageLoopMapValue(ledger["available_views"])
	if len(available) == 0 {
		return nil
	}
	keys := make([]string, 0, len(available))
	for key := range available {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	units := make([]contextruntime.WindowUnit, 0, len(keys))
	for _, key := range keys {
		row := messageLoopMapValue(available[key])
		conclusion := strings.TrimSpace(exitRetainText(row["conclusion"]))
		if conclusion == "" {
			continue
		}
		observationID := strings.TrimSpace(exitRetainText(row["observation_id"]))
		unit := contextruntime.WindowUnit{
			Unit:              contextruntime.ExitUnit{Kind: contextruntime.ExitUnitObservationBundle, ID: observationID},
			TurnID:            turnID,
			RetainedStatement: conclusion,
			Bytes:             int64(len(conclusion)),
		}
		if targetRef := strings.TrimSpace(exitRetainText(row["target_ref"])); targetRef != "" {
			unit.EvidenceRefs = []string{targetRef}
		}
		units = append(units, unit)
	}
	return units
}

// dedupeAgainstProjectLedger 过滤工程账本中已存在的同语句结论条目
// （Kind=observation_conclusion 且 Statement 全等）。读失败按不透传处理——
// 全量保留（宁可重复入账不静默丢结论；重复条目对 append-only 前缀无害）。
func dedupeAgainstProjectLedger(projectDir string, units []contextruntime.WindowUnit) []contextruntime.WindowUnit {
	entries, err := carriers.ReadLedger(projectDir)
	if err != nil {
		return units
	}
	existing := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.Kind == carriers.LedgerKindObservationConclusion {
			existing[entry.Statement] = true
		}
	}
	kept := make([]contextruntime.WindowUnit, 0, len(units))
	for _, unit := range units {
		if existing[unit.RetainedStatement] {
			continue
		}
		kept = append(kept, unit)
	}
	return kept
}
