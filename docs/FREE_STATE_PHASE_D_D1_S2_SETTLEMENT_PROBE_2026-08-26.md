# Free-State Phase D D1-S2 settlement 机械层（探针 + 判定路径修复）

Status: current record for the D1-S2 mechanical layer. Date: 2026-08-26（早窗 GLM L1/L2）。

D1-S2 的范围：把 D1 从"停在 human_audition_ready 边界"推进到"判定 → retain/rollback/ambiguous 结算 → settlement receipt 落地 → 重启一致性"的**机械层**全绿。真实人工裁决仍由用户在产品内完成（本文所有探针判定均带 `smoke_settlement_probe` 机器来源标记，`human_confirmed` 语义不因此伪造）。

## 交付物

### 1. 烟测 settlement 探针（scripts）

- `run_free_state_d1_smoke.ps1` 新增 `-SettlementProbe retain|rollback|ambiguous`：跑完既有 D1-S1 边界验证后，由探针提交带机器来源标记的判定，机检 settlement receipt，然后仅重启 agent 并以 `--verify-settled` 复核落盘状态与 continuation 终态（重启一致性）。
- `free_state_d1_smoke.py`：`--settlement-probe`（探针主路径：边界等待 → 判定 POST → 落盘断言：loop completed / experiment settled+outcome / d1_receipt 旗标组合 / 判定 evidence 身份与标记 / canonical task settled+terminal / revision 语义）与 `--verify-settled`（重启复核）。默认模式（无探针参数）行为与断言完全不变（仍断言 settled is False）。
- 顺手修复：`validate_d1` 的 `post_action_observation_id` 读 `observation_id`（closeout non-goal 3）；HTTP 错误体捕获（可诊断性）；语义状态解析走 `conversation_goals` 主路径。

### 2. 判定→结算路径的四个 Go 修复（探针前置审计发现）

探针落地前 TDD 复现（`TestD1S1JudgmentSettlementBindsCanonicalTaskContract` 首跑三处置全 409），按根因逐层修复：

| 修复 | 文件 | 根因 |
|---|---|---|
| 结算回绑 | `chat/task_semantics.go` settleTaskFromExperiment | task 结算后未把 experiment 的 canonical 投影 rebind 到新 revision，`Turn.Settle` 守卫读到过期的 human_judgment_required → 三处置全 409 |
| Settle 守卫扩展 | `experiment/runtime.go` Settle | `OutcomeNeedsJudgment` 在契约绑定下无条件拒绝；D1 ambiguous 判定以人工证据终态结算 task（settled）后应允许 |
| round 绑定自愈 | `chat/audition_events.go` recordFreeStateAuditionJudgment | 调度器传输回放会丢弃 round 的 `AuditionSessionID/UserJudgmentRequested`（loop 停在边界非活跃态时 `prepareFreeStateReasoningContext` 用旧传输快照整体替换 loop）；判定入口通过守卫 API `RequestUserJudgmentForSession` 重建绑定，不削弱任何身份校验 |
| revision 回退源 | `chat/audition_events.go` 同上 | `LatestProjectChange` 同样可被回放清掉；现行修订回退源改为最新干预 receipt 的 `after_revision`（round.ProjectRevision 是前向变更**前**的准入修订，不可用作现行修订） |
| 落盘 rename 重试 | `history/history.go` WriteAgentRuntimeState | Windows 瞬时句柄（索引器/并发读）拒绝原子 rename → 判定时刻整条持久化路径失败；有界重试 |

已知遗留（非本次范围）：`prepareFreeStateReasoningContext` 对非活跃 loop 的整体传输快照替换是 round 绑定/字段丢失的上游根因，属 continuation 回放层，记为后续任务。

### 3. WebUI settle 终态呈现

- `webui/src/audition.ts`：`auditionSettlementOutcome`——从 trajectory settlement 节点的 `details.outcome` 推导 D1 终态（注意 turn 级 outcome 会把 needs_user_judgment 映射回 human_audition_ready，原始值只在 details）。
- `trajectory/TrajectoryAuditionPanel.tsx`：结算后呈现三处置摘要（保留/回滚/模糊终止），A/B 播放与"查看/采用"控件禁用；**移除结算后误导性的"请使用采用"提示**（D1 判定直接结算，"采用"是 legacy 非 D1 流程动作）。legacy 未结算路径提示保留。
- vitest 41 测试全过（含 settlement 推导新用例）。

## 验收证据

- Go：chat/experiment/taskstate/history 四包 `-count=1` 全绿；新增回归：`TestD1S1JudgmentSettlementBindsCanonicalTaskContract`（三处置契约绑定结算）、`TestD1S1JudgmentHealsDroppedRoundBinding`（绑定自愈）、`TestSettleNeedsJudgmentRequiresCanonicallySettledTask`。
- WebUI：`npm run test` 41 过、`npm run build` 过。
- 真实栈（spv1_p02，`-SettlementProbe`，各含 agent 重启复核）：
  - retain：`artifacts/free_state_d1_s1/20260826_102420/` exit 0——`settlement_probe{disposition:retain, probe_origin:true, experiment_outcome:improved, task_semantic_state:settled}` + `restart_verification{task_semantic_state:settled, continuations_total:0}`。
  - rollback：`artifacts/free_state_d1_s1/20260826_103106/` exit 0（含重启复核 PASS）。
  - ambiguous：`artifacts/free_state_d1_s1/20260826_103722/` exit 0（含重启复核 PASS）。
- 默认模式回归：`artifacts/free_state_d1_s1/20260826_104403/` exit 0，`human_confirmed=false`、报告无 `settlement_probe` 节——无探针参数时行为完全不变。

## 边界遵守

探针判定永久带 `smoke_settlement_probe` 标记与 `machine-originated` free_text；默认 D1-S1 验证不伪造人工确认；D1 的人工体验验收仍以产品内真人操作为准。
