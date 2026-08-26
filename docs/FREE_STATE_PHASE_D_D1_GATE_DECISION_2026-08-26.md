# Free-State Phase D D1 gate decision

Status: gate decision record（用户裁定）。Date: 2026-08-26。

## 裁定

D1 关账、允许准备 D2（free-state-loop expansion，runtime 侧）。D1 的"产品内真人闭环"一环**显式推迟**到 GUI 阶段，不视为已达成，亦不静默改写闸门定义。

## 已验证（机器可验证据齐备）

1. **D1-S1 执行链**：两次连续真实栈 PASS（2026-08-25 closeout）+ taskstate 判定迁移修复（2026-08-26，41477bf）。
2. **D1-S2 机械层**：settlement 探针三 disposition（retain/rollback/ambiguous）真实栈全 PASS，各含 agent 重启一致性复核；判定→结算路径 5 项缺陷修复（372d80e）；默认模式行为不变（`human_confirmed=false` 边界保持）。见 docs/FREE_STATE_PHASE_D_D1_S2_SETTLEMENT_PROBE_2026-08-26.md。
3. **用户手测（系统侧）**：从 Godot 启动的真实栈上，开放意图走到提案确认门——诊断 → Bass -1.0 dB 有界提案 → G1–G7 准入 → 实验轮启动 → before 观察记录，continuation 正确停靠 `waiting_interaction`。系统侧行为全部符合设计。

## 推迟项（显式记录，归 GUI 阶段）

用户在产品内无法走完确认与判定，根因是 GUI/送达层缺口，非 runtime 语义问题：

1. **确认卡不可见**：continuation 停靠的交互没有 WebUI 浮出通道——"继续"消息路径只把 pending_interaction 放进 WorkflowData（goalrunner_chat.go:174-189），不作为 `interaction_requests` 返回，WebUI 的卡片渲染只消费后者；叠加深色轨迹面板挤占对话流，卡片即使送达也易被淹没。链条机械上可通过 API 完成（烟测即如此），缺的是产品内体验。
2. **full access 权限开关无法触发**（authority mode 切换失效）。
3. **WebUI 执行呈现设计债**：用户要求 ZCode/DeepSeek 式"淡灰正在思考动态文字 + 执行摘要短句"替代大深色轨迹面板（用户 2026-08-26 明确提出，按其要求当时不动）。

以上归入 GUI 阶段（T9 App.tsx 拆分的入场范围），其中第 1、2 项是"真人产品内判定"重开验收的前置条件。

## 边界保持

推迟不等于伪造：至今所有机器探针判定均带 `smoke_settlement_probe` 来源标记；`human_confirmed` 语义从未被机器置真。D1 的真人听感裁决证据（`human_confirmed=true` 来自产品内真人操作）仍待 GUI 修复后补验，届时在 D2 期间完成即可，不追溯改写本记录。
