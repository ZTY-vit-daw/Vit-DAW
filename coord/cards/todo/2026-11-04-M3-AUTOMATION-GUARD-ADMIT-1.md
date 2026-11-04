# M3-AUTOMATION-GUARD-ADMIT-1：automation surface 已准入 mutation 工具放行至治理门（guard 准入修复）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=246 验收裁定 [2026-11-04-M3-TERMINAL-ROUTE-WORDING-1-pass.md](../../rulings/2026-11-04-M3-TERMINAL-ROUTE-WORDING-1-pass.md) 新断点定性+三轮 guard 拒绝工件）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 247；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=246 已在 main（139fa254）✓；L1 / PC 执行会话；栈空闲（246 已释放）

## 标的（246 验收亲证，不重考）

模型 v3 措辞后已直调 set_points（7 次 agent log 亲证），但 chat executor 包装层 `freeStateInvokeMutationReason`（goalrunner_chat.go:855-863 每工具先过）+`free_state_invoke_guard.go:26-29`（active free-state 全拒 MutatesProject）先于治理门截杀（r1 10 次「mutation tool automation.set_points is forbidden」）→run failed→no_route×3。guard 是「free-state 只读观察」旧契约执法者，与 M3 automation surface 准入（决策轮直调→治理门拦卡，`TestFreeStateSetPointsZeroBareWriteGovernanceChain` 先例）正面冲突。

## 目标

1. **guard 准入**：`freeStateInvokeMutationReason` 对 automation surface 已准入的两个 mutation 工具（`automation.set_points`、`automation.clear_region`）放行——放行≠执行，它们继续走下游既有治理面（`typedPluginApplyConfirmationResponse`/agentloop 治理链）形成确认卡。**daw.invoke 与其余一切 MutatesProject/RefreshAfter 工具维持全拒不动。**
2. **治理红线**：确认卡仍是唯一授权点——不得出现 bare write；不触准入判定/治理门/执行器逻辑（越域即停）。
3. **单测**：①active free-state 下 set_points/clear_region invoke 通过 guard 且形成 needs_confirmation（非 error 非 bare write）②其余 MutatesProject 仍拒（原语义回归）③daw.invoke 仍拒④放行后确认前 project 无 mutation（零裸写断言）。
4. **真栈重跑**：-Groups 1 单组一次（deepseek-flash，载荷+agent log 全开）——**链口径判据：组 1 绿（no_route 消失+tool_chain_ok set_points 包络形态）或新断点取证包**；终局三选句（246 项一）真栈检验搭车（TERMINAL TURN 轮到达时新句在场被读）。

## 预期与诚实边界

模型面已直调（246 亲证），本卡=确定性接线；首绿窗口真实打开但 §8 口径不变：预算 ≈¥1.5-3；成功=组 1 绿或新断点取证（上交不自修）；若放行后 set_points 未形成确认卡而直接执行→**立即停上交**（治理门覆盖面缺口=设计问题升级主管，不得自行补门）。

## 停止条件

- 放行导致既有 guard/治理测试红（零回退）→回退两案上交；
- 出现 bare write 任何形态→立即回退上交；
- 组 1 仍不绿→新断点取证包上交，不原样重跑；
- 越域（free_state_invoke_guard.go+goalrunner_chat.go+chat 测试之外）即停上交。

## 文件域

`agent/internal/chat/free_state_invoke_guard.go`+`agent/internal/chat/goalrunner_chat.go`（如需接线）+chat 包测试。

## 领取

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
