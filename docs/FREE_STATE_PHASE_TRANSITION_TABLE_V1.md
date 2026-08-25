# Free-State Phase Transition Table v1

Status: implemented（Phase B, 2026-08-23）。宿主 audioclosure：FS0–FS9 相位与迁移解析在 agent/internal/audioclosure/phase.go；phase_transition 事件（data 携带 from/to/guard 快照）在 events.go，经 Store 持久化、Fold 可确定性重放；guard 评估 EvaluatePhaseGuard 按 §2 转移表逐相机检；Driver.TransitionPhase/RecordDiagnosticRound 在 driver.go。agentloop 侧相位感知决策校验 messageLoopFreeStatePhaseDecisionIssue（agent/internal/agentloop/free_state_gate.go）。L1 测试 M01/M02：agent/internal/agentloop/free_state_phase_test.go。chat 侧已在 bindAudioClosureContext 把 audioclosure 相位写入 free_state_phase 上下文键（agent/internal/chat/audio_closure_controller.go）。

Date: 2026-08-23

宿主决定（T10 既定方向）：FS 相位机宿主为 **audioclosure**。见 §5。

## 1. 相位总览

ADR §4 的十相位：

```text
FS0 semantic_entry -> FS1 project_bound -> FS2 capacity_assessed -> FS3 project_scan
  -> FS4 diagnostic_round -> FS5 candidate_frontier -> FS6 target_confirmed
  -> FS7 improvement_proposal -> FS8 experiment_verification -> FS9 terminal
```

投影是 peer 不是链（AGENTS.md §3）：相位机限制的是**决策类型**，不是观察投影的消费顺序。CCB 仍按显式 view 请求披露，不做自动 view 选择。

一个 model turn 不等于一个诊断轮：一个相位（尤其 FS4）可包含多次工具调用、重试与 continuation 调用。

## 2. 逐相位转移表

| 相位 | 进入条件/守卫 | 相位内允许的决策类型（观察视图族见产物 2 的维度→视图映射） | 合法转移 | 回退/跳过 |
|---|---|---|---|---|
| FS0 semantic_entry | 用户开放请求进入自由态路由（`semantic_entry_decision.v1`，docs/SEMANTIC_ENTRY_V1.md）；或显式请求路由到自由态 | 只读：`ccb.observation_catalog`；决策 `needs_observation` | → FS1（取得 project 绑定） | → FS9（capability_blocked：无可用工程） |
| FS1 project_bound | `taskstate.Contract`（agent/internal/taskstate/state.go:46-64）已带 project_uuid/project_revision；`audioclosure.State.ProjectUUID/ProjectRevision`（agent/internal/audioclosure/types.go:166-167）非空 | project 级只读：`project.structure`、`project.change_delta`；决策 `needs_observation` | → FS2 | → FS9（project_revision_stale / blocked） |
| FS2 capacity_assessed | 能力路由评估完成（FreeStateCapacityAssessment，agent/internal/chat/continuation_scheduler.go:77 已有字段） | 只读 project 级视图；决策 `needs_observation` / `capability_blocked` | → FS3 | → FS9（capability_blocked：无任何受治理可执行路径） |
| FS3 project_scan | project 级扫描视图（`mix.multitrack_relationship`、`mix.frequency_relationship`）至少一次可用（ready/partial）回执 | project/mix 级只读视图；决策 `needs_observation`；可登记候选（不冻结） | → FS4；若扫描即得唯一高价值候选可 → FS5 | → FS9（no_candidate_found：队列耗尽且无候选） |
| FS4 diagnostic_round | 优先队列非空（产物 2）；每轮恰一个 primary dimension | 该维度主视图+支撑视图（产物 2 映射表）；决策 `needs_observation`；轮末更新队列/候选引用 | → FS4（下一轮）；→ FS5（≥1 维 closed 且出现 plausible 候选）；→ FS9（no_candidate_found / diagnostic_complete） | 维度可 skip（理由枚举见产物 2）；不可回退到 FS3 重扫除非 project revision 变化 |
| FS5 candidate_frontier | ≥1 维 closed；`audioclosure.State.Frontier`（HypothesisFrontier，types.go:180）建立 | 候选级只读视图；决策 `needs_observation`；候选去重/降级 | → FS6（选中单一候选） | → FS4（候选全部被证据否定且队列非空）；→ FS9（no_candidate_found） |
| FS6 target_confirmed | 选中候选有 target 级证据（现实现已有 target-level selection 门：TestOpenSemanticCandidateFrontierRequiresTargetLevelSelection，agent/internal/agentloop/free_state_reasoning_test.go:1241） | target 级只读视图；决策 `needs_observation` / `needs_experiment`（需过产物 3 新门） | → FS7 | → FS5（target 证据否定候选）；→ FS4；→ FS9（仅限 continuation budget exhaustion 等带合法 stop reason 的有界失败终态，不得表达实验成功） |
| FS7 improvement_proposal | 产物 3 新门全过；proposal refs fresh 且 revision-bound | 决策 `needs_experiment`（零直接 mutation 工具调用；现实现已强制：agent/internal/agentloop/free_state_reasoning.go:575-577） | → FS8（experiment.Admission 构造并 Validate 通过，agent/internal/experiment/runtime.go:130-187） | → FS4（proposal 被否） |
| FS8 experiment_verification | Admission 有效；一轮 bounded 实验（ADR §7） | governed typed 执行 + fresh post-change 证据；classification：material/subthreshold/ambiguous/unsupported | → FS9（retain / rollback / continue_once→FS8 一次 / request_audition） | ambiguous ⇒ 停止自动升级（ADR §8），只能 request_audition 或报告 |
| FS9 terminal | 终态 stop reason 集合即 audioclosure.StopReason（agent/internal/audioclosure/types.go:39-63） | 无（终态只读报告） | —（新用户消息开新 FS0） | — |

### 非法转移示例

- FS0/FS1 → FS7：跳过容量评估、扫描与候选前沿，直接提出改进（当前弱门允许的形态，见 §3）。
- FS3 → FS6：未 closed 任何诊断维度即确认 target。
- FS4 → FS8：未过产物 3 门即执行实验。
- FS7 状态下携带任何直接 mutation 工具调用（现实现已拒绝，free_state_reasoning.go:575-577）。
- FS8 ambiguous → 再次自动加剂量（违反 ADR §8 停止规则）。

## 3. 主 bug 的门定位（ADR §2）

现状：一次成功的 `ccb.observation_request` 返回 usable bundle 即满足 `needs_experiment`（agent/internal/agentloop/free_state_reasoning.go，已由新门整体替换，见 docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md）。这等价于允许 **FS0 → FS7** 的跳相位。

新门（产物 3，docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md）要求 FS1（project binding）、FS2（capacity）、FS3（project scan）、FS4（≥1 维 closed）、FS5（frontier）、FS6（target 级证据 + fresh/revision-bound refs）全部成立。即：FS1–FS5 的转移守卫整体成为 `needs_experiment` 的前置条件，弱门判定在新门上线后已被整体替换（free_state_reasoning.go:578-583）。

## 4. 四层现存状态机 → FS 相位映射（T10 消漂移输入）

| 现存值 | 位置 | 映射到 FS 相位 | 收敛判定 |
|---|---|---|---|
| `needs_observation` | agent/internal/agentloop/free_state_reasoning.go:31 | FS0–FS6 内的观察请求决策（按所处相位细分） | 保留，但不再是相位本身 |
| `needs_action` | free_state_reasoning.go:32 | FS6/FS7（governed 动作交接） | 保留 |
| `needs_experiment` | free_state_reasoning.go:33 | FS7（新门通过后） | 保留，判定替换 |
| `diagnostic_complete` | free_state_reasoning.go:34 | FS9 | 保留 |
| `no_candidate_found` | free_state_reasoning.go:35 | FS9 | 保留 |
| `improvement_proposal` | free_state_reasoning.go:36 | FS7→FS8 交接态 | 保留 |
| `capability_blocked` | free_state_reasoning.go:37 | FS9 | 保留 |
| `satisfied` | free_state_reasoning.go:38 | FS9 | 淘汰用于 improvement 契约（现实现已禁止：free_state_reasoning.go:586-588 只允许 legacy 局部结论） |
| `blocked` | free_state_reasoning.go:38 | FS9（transport/protocol 类） | 收敛：与 `capability_blocked` 分工写明后保留 |
| `observing` | agent/internal/audioclosure/types.go:24 | FS1–FS6 的观察子态 | 收敛为 FS 相位内的活动标记，不是相位 |
| `reasoning` | types.go:25 | FS4/FS5 决策子态 | 同上 |
| `awaiting_capability` | types.go:26 | FS6→FS7 交接（等待 governed 路径） | 保留为子态 |
| `verifying` | types.go:27 | FS8 | 保留 |
| `settled` | types.go:28 | FS9 | 保留 |
| experiment `framing`/`observing`/`hypothesizing` | agent/internal/experiment/runtime.go:27-29 | FS6/FS7（实验前段） | 收敛：Turn 生命周期从属于 FS6–FS8 |
| experiment `running` | runtime.go:30 | FS8 | 保留 |
| experiment `waiting_for_user` | runtime.go:31 | FS8 的 request_audition 出口 | 保留 |
| experiment `settled`/`stopped`/`failed` | runtime.go:32-34 | FS9 | 保留 |
| chat `processor_selection` | agent/internal/chat/free_state_reasoning_loop.go:29 | FS6（族选择） | 保留为 DecisionPhase，不承担相位职责 |
| chat `processor_materialization` | free_state_reasoning_loop.go:30 | FS7→FS8（governed 物化） | 保留 |
| chat `post_action_evaluation` | free_state_reasoning_loop.go:31 | FS8 验证段 | 保留 |

淘汰目标（T10）：`satisfied`（improvement 契约下）、`observing/reasoning` 作为独立相位的用法、`blocked` 与 `capability_blocked` 的未定义重叠。

## 5. 宿主决定与承载方式

相位机宿主 = **audioclosure**（T10 既定）。理由：它已拥有持久化 State（types.go:155-192）、事件溯源（`Event`，agent/internal/audioclosure/events.go:31-38；`Store` 接口，agent/internal/audioclosure/store.go:9-16）、round/policy 预算（Policy，types.go:65-79）与 stop reason 全集（types.go:39-63）。

改造边界（Phase B 实施，本文只定义契约）：

- `audioclosure.Phase` 五值扩展或替换为 FS0–FS9；`Events` 追加相位转移事件（`phase_transition`，data 携带 from/to/guard 结果），由 Store 持久化。
- agentloop 的决策校验（free_state_reasoning.go 的 Validate 路径）从「校验单个 status」升级为「向宿主查询当前相位 + 校验该相位允许的决策类型」；模型仍是观察选择与假设的所有者，宿主只拥有相位与门。
- chat freeStateReasoningLoop 的 `DecisionPhase` 三值保留为模型侧交接标注，不承载相位真值。
