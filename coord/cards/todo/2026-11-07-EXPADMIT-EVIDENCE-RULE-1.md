# EXPADMIT-EVIDENCE-RULE-1：D1-S1 实验准入门证据引用规则修正+拒绝路径 park 化

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[262 verifyrun1 取证](../runs/MIXTICK-CONFIRM-CONTRACT-1/verifyrun1/evidence.md)——新形态三选项裁定②+①组合；262 终验转卡）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 269；目标仓库=主仓 agent（free_state 准入链）；P1 / 0.5-1 天 / 依赖=262 工件在库；L1-L2 / PC；验证腿需真栈一次（mix_single_tick 复现场景）

## 背景（262 verifyrun1 钉死）

模型完全自主产出有效提案（proposal_valid=true，evidence_refs 引用同目标更早观察 obs_154552 basic_energy+seed），被准入门拒绝且**终局化**：门绑定 `loop.LatestObservation`=obs_154607 要求必含其 ID（`agent/internal/chat/free_state_experiment_runtime.go:176-179`）→`agent/internal/agentloop/free_state_reasoning_loop.go:1684-1693` 改写 blocked/`free_state_experiment_admission_invalid`→响应面终局 done→`turn_close_guard.go:56` 孤儿收口，确认卡未建。三因素叠加：模型引用选择（合理：basic_energy 对电平提案声学合理）×门最新绑定规则（把"新鲜"过度具体化为"最新"）×拒绝即终局。

## 目标

1. **②门校验放宽**（主修）：evidence_refs 校验从「必含 LatestObservation ID」放宽为「含**任一同目标的新鲜观察**」（新鲜=G7 revision-bound 既有语义，同 revision 内不区分先后）；LatestObservation 绑定语义的原始目的（证据新鲜度）不得削弱——同 revision 内更早观察与最新观察等效；
2. **①拒绝路径 park 化**（防御）：准入门拒绝不再终局化——改走 terminalAdjudicationParking 族既有先例机制（回喂或挂「方案乙 park」确认面），孤儿收口路径保留为最后兜底；
3. 配对单测：①同 revision 早期观察引用准入通过 ②跨 revision 陈旧引用仍拒（G7 语义不削）③拒绝路径产 park/确认面而非终局 done；
4. 验证腿：mix_single_tick 单 run 复现场景（≈¥1-3，§8 成文）——预期 observe turn 直停确认面（262 原断言目标）。

## 约束

文件域=free_state_experiment_runtime.go+free_state_reasoning_loop.go 拒绝分支+测试；禁动 G7 门本体（free_state_gate.go 的 revision-bound 判据零改动——本卡改的是引用选择规则不是新鲜度规则）；治理红线不变（确认链/零裸写）；256/258 断言面零回退。

## 停止条件

修复发现拒绝终局化是有意设计（有在案裁定）→取证上交转裁定；验证腿断言失败取证不重跑。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
