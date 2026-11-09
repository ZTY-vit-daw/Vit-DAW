# FIX-GOV-INTERACTION-TTL-1 形态设计（勘察→选型→最小修）

- 卡：FIX-GOV-INTERACTION-TTL-1（327）；执行：GLM-5.3 执行子代理 / PC；2026-11-09
- 语料：`coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/proq3_free_20261109_024813/`（run9 r2 段，只读消费）
- 基线：origin/main 5915bb9c；分支 port/gov-interaction-ttl-1

## 1. 勘察锚点（代码行 + 语料对账）

### 1.1 「过期窗口当前值」= 不存在时间型 TTL

- `agent/internal/chat/server.go:4163-4175 takePendingInteraction`：消费式取卡（map delete），无时间判断。
- `agent/internal/chat/server.go:4021-4043 storePendingInteraction`：入 map + `persistCurrentProjectWorkspace()`。
- `PendingInteraction.CreatedAt` 在全部 chat 域代码中只用于「取最新」（server.go:4060）与打戳（4023/4302/4338），**无任何代码用它做过期判定**——治理确认交互没有服务端时限参数，也就没有「当前值」。
- 唯一的删除路径：`takePendingInteraction`（消费）、`restorePendingInteraction`（回填）、`clip_fade_gain_guard.go:88-123`（clip/fade/gain 守卫定向撤销）、`automation_choreo.go:898`、以及 **restore 整体替换**（见 1.2）。

### 1.2 r2 交互丢失的确切路径（逐条对 run9 r2 语料）

时间线（rounds/r2/round_record.json + logs/copy_agent_last.log 行号）：

| 时刻(+0800) | 事件 | 证据 |
|---|---|---|
| 02:48:20-21 | 装置腿预开三个工程副本，激活序 r2→r3→**r1**（active workspace 终态=r1） | log L41/L116/L187 |
| 02:49:05 | r1 轮 approve 成功（active=r1 恰好匹配，交互存进 r1 状态文件） | log L301 total_ms=120 |
| 02:53:45 | r2 轮 chat POST 起点；activate 静默返回（kernel 身份未及上报/active 仍=r1，无激活日志） | 无 [workspace] activated 行 |
| 02:53:49→18:54:22 | 5 次 LLM 调用；**18:54:22.67Z** 产出 improvement_proposal 确认卡 `interaction_9c0c28ce7c8999e8` 入 `s.interactions`，persist 落进 **r1** 的状态文件（active workspace 仍是 r1） | telemetry 5 行；improvement_proposal_workflow.go:188-190→server.go:4021/4042 |
| 02:54:22 | chat 收尾 defer `syncCurrentProjectWorkspace`（server.go:7670）→activate 全量重激活（身份 r1→r2 切换，同身份 no-op 条件 7490-7494 不满足）→`ReadAgentRuntimeState(r2)`=陈旧快照（无该交互）→`restoreProjectAgentRuntimeStateLocked`（7581）整体替换 `s.interactions`（7791）→**交互被抹掉** | log L553-L555 `[workspace] activated ... r2.vit ... conversations=0` |
| 02:54:22 | +77ms 驱动 approve POST `/agent/interaction/respond` →`takePendingInteraction` !ok→六条 payload 恢复路径全败（驱动 payload={}）→**server.go:4530-4537 罐头回执** | log L560 total_ms=77；rounds/r2 confirmations[0] |
| — | 罐头回执 `GoalStatus=completed` → 驱动循环判定 goal_completed → round stop_reason=goal_completed → 「模型终答」=罐头文案被逐字采为自报告 | rounds/r2/round_record.json stop_reason；self_report.md 全文 |

- 「这个交互已处理或已过期。」的产生位置：`agent/internal/chat/server.go:4531-4535`（interaction 面 catch-all）；同文案 plan 面变体在 5242-5247（本卡不动，见边界）。
- fail-open 三宗罪：①approve 被静默吞（确认卡对应的候选动作从未执行）；②回执伪造 `goal_status=completed`（r2 终态「goal_completed」是罐头伪造，非真实目标完成）；③「已处理」与「已过期/丢失」不可区分（重复 approve 与丢失 approve 同文案）。
- 模型侧回告路径：罐头文案经驱动自报告采集面回流为模型终答（r2 self_report.md 全文即此句）；修复点在服务端回执本身，模型无感知路径。

### 1.3 停止条件评估

交互生命周期完全在 agent 侧（chat Server 内存 map + 工程工作区状态文件 `%APPDATA%` 侧 projectAgentRuntimeState），**不涉内核交互存储**——未触发「越域上交」。

## 2. 选型（三选一或组合 → 组合②'保活 + ③显式回执；①不适用）

- **①延长窗口：不适用**。不存在可延长的时间窗口；丢失是状态管理缺陷（跨工作区切换的在飞交互被陈旧快照覆盖），且 r2 从确认卡产出到 approve 抵达仅 77ms——任何时限参数都文不对题。
- **②approve 幂等重开 → 实质化为「在飞保活」**：restore 替换 interactions map 时，对「快照中不存在且 `CreatedAt` 晚于快照 `SavedAt`」的在飞交互保留（快照 `SavedAt` 更新且仍不含该卡=另一所有者已决断，照旧丢弃交③告知）。r2 类迟到 approve 从「过期」变为**可正常应答并执行**。
- **③过期显式回执**：take 扑空且 payload 恢复全败时，不再回「已处理或已过期+completed」；回显式回执——明示 **approve 未执行、未产生修改、目标未因此完成**，附可辨原因类别；配 tombstone（近期被消费/被守卫撤销记录，容量 128 环）使重复 approve 拿到「已处理（何时、何种路径）」与「从未存在/已丢失」可区分的幂等回告。
- **fail-closed 红线落实**：approve 永不静默吞——要么真执行（保活成功路径），要么拿到「未执行+原因」显式回执+WARN 日志留痕。

## 3. 最小修落点（agent 域）

1. `server.go restoreProjectAgentRuntimeStateLocked`：替换前合并保留在飞交互（规则见②）。
2. `server.go takePendingInteraction`：消费时记 tombstone（id→时刻+路径 "consumed"）。
3. `clip_fade_gain_guard.go expirePendingInteractionsForPlan/ForConversation`：撤销时记 tombstone（reason=守卫名）。
4. `server.go handleInteractionRespond` catch-all（4530-4537）：显式回执（tombstone 命中→「已于 T 被处理/撤销（原因）」；未命中→「未找到…未执行」），GoalStatus 不再伪造 completed，StopReason=interaction_already_retired / interaction_not_found，WARN 日志。
5. 测试：`server_test.go:3684 TestInteractionRespondStaleReturnsChineseChatResponse` 期望值随语义修订（旧断言钉的正是本卡要修的 fail-open 形态）+ 新三态测试。

## 4. 边界（端测边界声明素材）

- 跨重启的待确认交互持久化语义不变（重启后内存为空，行为同今天）；本修只覆盖同进程内工作区切换丢失类。
- plan 面（`resolvePendingPlanDecision` 5242-5247）同型 fail-open（伪造 completed）不在本卡文件域聚焦面（r2 走 interaction 面），登记候后续产品小卡。
- 六条 payload 恢复路径（mixboard/mix_tick/capability/plugin_recommendation/semantic_treatment/b4）语义不变。
- 真栈腿（r2 场景复现：提案→延迟 approve→断言显式回执）候后续栈窗口，本卡不起栈（脚本设计见 artifacts/r2_replay_plan.md）。
