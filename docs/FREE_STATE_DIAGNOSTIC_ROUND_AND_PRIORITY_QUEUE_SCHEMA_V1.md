# Free-State Diagnostic Round and Priority Queue Schema v1

Status: implemented 且生产可达（Phase C runtime wiring, 2026-08-23）。round record / priority queue schema 与维度→视图映射、skip 理由枚举、重请求三准入（AdmitReObservation，准入 1 要求出现上一轮不存在的新 unresolved question）在 agent/internal/audioclosure/diagnostic_round.go；生产轮闭合时由 chat audio_closure_controller.go 的 audioClosureRoundRecord + Driver.RecordDiagnosticRound 写入事件流（G4 数据源），重启由 Fold 重建。chat freeStateReasoningLoop 已新增 current_round_id / current_phase / priority_queue / continuation_budget / continuation_used 持久化字段。agentloop 的 messageLoopFreeStateAlreadyObservedIssue 现在消费三种准入：新 unresolved question + contradiction priority、新 project revision、以及 stale/partial receipt + declared contradiction；rejected exact view-set 仍 fail-closed。no_candidate_found 必要性机检消费 PriorityQueue.HasOpen()（messageLoopFreeStateQueueStillOpen，malformed 队列 fail-closed 视为仍 open，并调 Validate()）；生产队列由 audioclosure.QueueFromRounds 从事件流 diagnostic rounds 派生、chat syncFreeStateSpine 镜像进 loop.PriorityQueue 持久化；队列耗尽 ⇒ no_candidate_found 且 reply 不得暗示工程完美。L1 测试 M03–M05：agentloop/free_state_priority_queue_test.go。

Date: 2026-08-23

## 1. Round record：`free_state_diagnostic_round.v1`

```json
{
  "schema_version": "free_state_diagnostic_round.v1",
  "round_id": "r_<hash12>",
  "goal_id": "", "run_id": "", "conversation_id": "",
  "primary_dimension": "level_headroom | frequency_occupancy | dynamics | stereo_space | transient_event",
  "priority_reason": "default_order | project_evidence | freshness | cost | contradiction",
  "views_requested": ["track.basic_energy"],
  "evidence_status": "open | ready | partial | stale | rejected",
  "candidate_refs": [],
  "unresolved_questions": [],
  "skipped_dimensions": [{"dimension": "stereo_space", "reason": "not_applicable"}],
  "next_priority": ["dynamics", "stereo_space", "transient_event"],
  "project_revision": "",
  "created_at": "", "closed_at": ""
}
```

ADR §5 十项字段与上表一一对应（身份三键 `goal_id/run_id/conversation_id` 来自 ADR §10 持久化键，不计入十项）。机检规则：

- `primary_dimension` 每轮恰一个；`views_requested` ⊆ 该维度允许视图（§3 映射表）。
- `skipped_dimensions[].reason` ∈ 枚举 `not_applicable / fresh_evidence_exists / already_covered / dependency_unavailable / cost_limited`（ADR §5）。
- `evidence_status` 只由回执（`ccb_observation_receipt.v1`，docs/FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md）推导：有 usable（ready/partial）视图即不可为 `open`。

## 2. Priority queue：`free_state_priority_queue.v1`

- 默认序（ADR §5，非断言）：`level_headroom → frequency_occupancy → dynamics → stereo_space → transient_event`。
- Runtime 可依据 `priority_reason ∈ {project_evidence, freshness, cost, contradiction}` 重排、跳过（须记 §1 枚举理由）、重访。
- **同 target/view set 重请求准入**（ADR §8 停止规则，现实现已有 rejected-set 侧的门：agent/internal/agentloop/free_state_reasoning.go:889-1147 系列测试；本 schema 把它推广到 usable 集合，现实现的 TestOpenSemanticRejectsRepeatingUsableProjectObservation，free_state_reasoning_test.go:1439，已覆盖 project 级特例）：重复请求必须满足其一——
  1. 新证据：上一轮 `unresolved_questions` 出现新问题且本轮 `priority_reason=contradiction` 或新 view；
  2. 新 project revision（round record 的 `project_revision` 变化）；
  3. 记录的矛盾（前一 round `evidence_status=stale/partial` 且新轮声明冲突）。
- 队列耗尽 ⇒ `no_candidate_found`（FS9），不得暗示工程完美（ADR §8）。

## 3. 维度 → 视图映射表

视图全部取自现行 18 个 CCB 视图目录（agent/internal/capabilitycontext/free_state_observation.go:378-403；契约清单 docs/FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md §Semantic Views）。投影为 **peer 不是链**：视图并列消费，无 DAD→DOM→MOM 管线语义。

| 诊断维度 | 主视图 | 支撑视图 |
|---|---|---|
| level_headroom | `track.basic_energy` | `track.peak_structure`、`mix.multitrack_relationship`、`project.structure` |
| frequency_occupancy | `track.timbre_frequency` | `mix.frequency_relationship`、`mix.masking_relationship`（deferred 时记 omission）、`track.frequency_time_events` |
| dynamics | `track.time_dynamics` | `track.band_dynamics`、`processor.behavior`、`track.activity_structure` |
| stereo_space | `track.stereo_space` | `mix.multitrack_relationship` |
| transient_event | `track.transient_structure` | `track.frequency_time_events`、`track.peak_structure` |

跨维支撑视图（`project.change_delta`、`comparison.before_after`、`processor.identity_and_controls`、`processor.change_delta`）不属任何维度的主视图，仅作支撑/验证用途。

## 4. 持久化落点（ADR §10 九键逐一标注）

ADR §10 九个持久化键：`goal_id, run_id, round_id, current_phase, priority_queue, candidate_frontier, observation_ledger, project_revision, continuation_budget`。

| 键 | 落点（现状） | 需新增 |
|---|---|---|
| goal_id / run_id | `DurableContinuation.GoalID/RunID`（agent/internal/chat/continuation_scheduler.go:63-65） | 否 |
| round_id | —（无对应结构） | 是：`freeStateReasoningLoop` 新增 `current_round_id`（agent/internal/chat/free_state_reasoning_loop.go:47-76） |
| current_phase | `audioclosure.State.Phase`（types.go:171，现为五值 Phase） | 是：扩展为 FS0–FS9（产物 1 §5） |
| priority_queue | — | 是：`freeStateReasoningLoop` 新增 `priority_queue`（§2 schema） |
| candidate_frontier | `audioclosure.State.Frontier`（HypothesisFrontier，types.go:180） | 否（对齐 FS5 语义即可） |
| observation_ledger | `freeStateReasoningLoop.ObservationLedger`（free_state_reasoning_loop.go:69；agentloop 侧 free_state_reasoning.go:24-26 同名限 24 条） | 否（round record 引用 ledger 条目） |
| project_revision | `DurableContinuation.ProjectRevision`（continuation_scheduler.go:75）、`audioclosure.State.ProjectRevision`（types.go:167）、`taskstate.Contract.ProjectRevision`（state.go:62） | 否 |
| continuation_budget | `DurableContinuation.Attempt` + chat 循环显式 `continuation_budget`/`continuation_used` 字段（free_state_reasoning_loop.go:62-65，默认=MaxCycles） | 已新增：调度器入队点（goalrunner_chat.go）与交互路径双端生效，耗尽停机且 stop reason 经 /agent/runtime/status 可查 |

Round record 本体持久化于 audioclosure `Store`（agent/internal/audioclosure/store.go:9-16）追加的事件流（`Event.Data`，events.go:31-38），重启恢复从事件重建 phase/round/ledger/frontier（ADR §10）。
