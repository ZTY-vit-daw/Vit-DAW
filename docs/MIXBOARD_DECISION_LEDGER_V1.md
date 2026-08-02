# Mixboard Decision Ledger and Mix Report v1

状态：Normative v1
日期：2026-07-30

## 1. 职责与 authority 边界

Mixboard 是项目级混音决策读模型，不是新的事实、执行或历史 authority。它只保存和投影以下已有记录的引用：

- Capability Runtime `PlanningSession` / FrozenPlan；
- Proposal 与 ActionSet；
- Project History；
- Action Receipt；
- before/after Observation 与 Verification evidence。

Mixboard 不复制 observer、DAD/TOM/MOM、topology、executor，也不允许报告文本反向授权工程写入。需要事实时，能力仍从原 authority 读取。

## 2. 版本化合同

- `mix_decision_record.v1`：一个 terminal Capability Session 的不可变审计投影。
- `mix_decision_state_event.v1`：对不可变决策的后续状态事件；v1 明确定义 `reverted`。
- `mix_decision_board.v1`：项目当前决策视图。
- `mix_report.v1`：面向用户和最终交付检查的只读报告。

项目路径位于 Mixboard root 的 `_projects/<project_uuid>/`。原 Session、Receipt、History 和 Observation payload 仍位于各自 authority。

## 3. Decision Record

记录至少包含：

- Project Cut、capability ID/version、goal、target scope；
- terminal status 与 decision status；
- Context Bundle、Proposal、ActionSet、Project History、Receipt、before/after Observation 引用；
- Verification、evidence refs 与 limitations；
- `reads_dimensions`、`writes_dimensions`、`recheck_on_dimensions`；
- created/completed/recorded 时间。

状态包括 `verified`、`needs_review`、`inconclusive`、`failed`、`stale`、`cancelled`、`superseded`、`reverted`。

## 4. 状态传播

1. 同一 capability 的后续 verified 决策在 scope 相交时 supersede 旧记录。
2. 后续有效写入与旧记录 `recheck_on_dimensions` 相交时，旧 verified 记录进入 `needs_review`。
3. 无关维度不传播失效。
4. Project Cut 的工程 epoch/revision 改变只标记当前 Cut 存在未记录变化，不粗暴重写所有历史记录。
5. 已有 authoritative rollback evidence 时写 `mix_decision_state_event.v1/reverted`；原记录保持不可变，已 reverted 的写入不再使更早结论失效。
6. cancelled/failed 作为明确终态保留，但不伪装成已验证混音成果。

## 5. 后续能力消费

后续 B2/B3/B4 Context Bundle 可以携带相关 `mixboard-decision:<record_id>` 引用和当前 freshness 状态。只允许以下用法：

- `verified` 可作为导航引用，事实仍需从原 authority 获取；
- `needs_review` / `inconclusive` 必须带 `requires_revalidation=true`，不得作为 settled fact 继承；
- `failed`、`cancelled`、`stale`、`superseded`、`reverted` 不进入 active decision context；
- Context Bundle 只携带 bounded ref/status/dimensions，不复制 Verification 或 Observation payload。

## 6. Mix Report

只读工具名为 `mix.report`，兼容 alias 为 `mix_report`。输出 `mix_report.v1`，包含：

- 当前 Project Cut 与 mix intent；
- capability summary 与 decision timeline；
- verification 状态计数；
- unresolved items、limitations 与 final measurements；
- `export_readiness`。

`export_readiness` 规则：

- 没有最终测量：`not_assessed`；
- 测量存在但有 unresolved、needs_review 或当前工程未记录变化：`needs_review`；
- 最终测量 `status=ready` 且没有未解决项：`ready`。

工具只读取当前工程 snapshot 和 Mixboard ledger，不修改 DAW，也不以“报告生成成功”代替用户听感接受。

## 7. 烟测合同

1. B2/B3/B4 terminal execution 均生成 project decision record。
2. cancelled/failed terminal Session 具有明确投影状态。
3. 后续 B4 static EQ 会使相关 B2 决策进入 `needs_review`，无关 B3 保持 verified。
4. 同 capability 的新 verified 记录 supersede 旧记录。
5. authoritative reversion event 将目标记录投影为 `reverted`，并停止其写入传播。
6. 相同工程 epoch/revision、仅能力合同 hash 不同，不误报工程变化。
7. 工程 revision 改变时报告 `current_project_cut_has_unrecorded_changes`，但历史 decision status 不被整体改写。
8. Context Bundle 只披露 bounded refs，且 unsettled ref 带 revalidation 标志。
9. `mix.report` 通过 read-only guard；缺少最终测量时不得输出 ready。
