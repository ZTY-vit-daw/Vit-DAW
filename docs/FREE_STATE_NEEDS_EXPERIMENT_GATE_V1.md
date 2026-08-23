# Free-State needs_experiment Admission Gate v1

Status: implemented 且生产可达（Phase B 接线收口，2026-08-23）。G1–G7 逐条机检在 agent/internal/agentloop/free_state_gate.go（evaluateFreeStateNeedsExperimentGate），已整体替换弱门；门失败唯一合法出口 needs_observation；diagnostic-only 恒不通过；门通过 ⇒ experiment.Admission 构造并 Validate。生产数据源已接通：G4 消费轮边界写入事件流的 diagnostic round record（chat audio_closure_controller.go audioClosureRoundRecord + RecordDiagnosticRound）；G7 消费 ledger 行透传的 project_binding.project_revision 与 freshness（chat free_state_reasoning_loop.go freeStateObservationProjectRevision；agentloop free_state_reasoning.go receipt/view 写入器）。G2 按真实字段机检：capacity_level=exceeds_free_state 或 selected_capability 为空即拒。L1 测试 M06/M07：agentloop/free_state_gate_test.go + free_state_gate_variants_test.go。

Date: 2026-08-23

## 1. 替换关系

现行弱门：一次成功 `ccb.observation_request` 返回 usable bundle 即允许 `needs_experiment`（已被 §2 七项机检整体替换；新门调用在 agent/internal/agentloop/free_state_reasoning.go:578-583）：

```go
if !messageLoopHasSuccessfulCCBObservationRequest(state) {
    return "cannot propose an improvement experiment until at least one model-requested ccb.observation_request has returned a usable evidence bundle"
}
```

本契约通过后，该单条件判定**整体替换**为 §2 的七项机检合取。相邻不变量保留不变：`needs_experiment`/`improvement_proposal` 仍禁止携带直接 mutation 工具调用（free_state_reasoning.go:575-577）。

## 2. 七项条件（ADR §9）逐条机检定义

| # | 条件 | 机检输入（现有结构） | 判定 |
|---|---|---|---|
| G1 | project binding complete | `taskstate.Contract.ProjectUUID/ProjectRevision`（agent/internal/taskstate/state.go:61-62）非空，且与 `audioclosure.State.ProjectUUID/ProjectRevision`（agent/internal/audioclosure/types.go:166-167）一致 | 两个非空且相等 |
| G2 | capacity assessed | `free_state_capacity_assessment` 上下文（agent/internal/agentloop/free_state_gate.go gateG2）按真实字段机检：`capacity_level` ∈ 生产枚举且 ≠ `exceeds_free_state`，`selected_capability` 非空 | 字段命中 |
| G3 | project-level scan complete | observation ledger（agentloop free_state_reasoning.go messageLoopFreeStateContext 消费；chat free_state_reasoning_loop.go mergeFreeStateObservationLedger 写入）中存在 ≥1 条 usable 的 project/mix 级视图回执（`mix.multitrack_relationship` 或 `mix.frequency_relationship`，ready/partial） | ledger 命中 |
| G4 | at least one diagnostic dimension closed | 产物 2 round record：≥1 个 `primary_dimension` 满足 closed 定义（`evidence_status=ready` 且无 open unresolved question；与 audioclosure.DiagnosticRoundRecord.Closed() 一致。partial 是 continuation 点，不是 closed） | round 记录命中 |
| G5 | candidate frontier established | `audioclosure.State.Frontier`（types.go:180）非空（≥1 候选） | 非空 |
| G6 | selected candidate has target-level evidence | 现实现已有 target-level selection 校验路径（TestOpenSemanticCandidateFrontierRequiresTargetLevelSelection / ...AllowsSameTurnFinalDecision，agent/internal/agentloop/free_state_reasoning_test.go:1241、1289）；输入为 ledger 中选中候选 target 的 view 回执 | 候选引用的 target 视图 usable |
| G7 | proposal refs are fresh and revision-bound | ledger 回执/视图行的 freshness（fresh，或生产 bundle 的 current/current_snapshot/current_observation 状态；stale 恒拒）与 project_revision（源自 CCB bundle 的 project_binding.project_revision，两个 ledger 写入器均透传）= G1 的 revision | 全部 refs fresh 且 revision 相等 |

## 3. 与 experiment.Admission 的衔接

门通过是构造 `experiment.Admission`（agent/internal/experiment/runtime.go:130-145）的**前置条件**：G1–G7 全真 ⇒ 允许 `needs_experiment` 决策 ⇒ 用 G5/G6/G7 的证据构造 Admission 并通过 `Validate`（runtime.go:147-187）。门不通过时 Admission 不得构造；已存在的 Validate 只做 schema 完整性，不重复门语义。

## 4. 出口与认识论边界

- 门不满足时**唯一合法出口 = `needs_observation`**（不得改为 blocked/satisfied 绕过，ADR §9）。
- 门**不要求客观缺陷证明**：`plausible_improvement` 即可入 experiment（ADR §6）；`objective_defect` / `no_candidate_found` 是并列决策态，不是门的输入。
- 门在 diagnostic-only 轮次（docs/FREE_STATE_DIAGNOSTIC_ONLY_V1.md）恒为不通过：diagnostic-only 终态只能是诊断结论（现实现已强制，free_state_reasoning.go:572-574）。
