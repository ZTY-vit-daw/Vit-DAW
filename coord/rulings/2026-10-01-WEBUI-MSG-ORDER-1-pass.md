# Ruling：WEBUI-MSG-ORDER-1 — pass（2026-10-01 决策会话）

- 实现：65b0d2a3+dc765ec7@port/webui-msg-order-1 → 合并 main 0e7ba86d/8e21258e（cherry-pick）；卡片状态变更 31f02c65 已由执行侧合规直推 main（本卡流程零缺口）。
- 亲核项：
  1. **diff 直读**（两文件核心改动外科手术级，注释带证据锚）：turnGroups——用户消息只在回合组仍是尾随组时并入，同 run 新一轮输入自成 loose 组保流位（消息级 turn_id 保留，settleTerminatedTurnInteractions/messageAnchoredTurnIds 消费面不受组形态影响）；renderPlan——链终局只在仍是最新内容时钉尾（被超越者回流时序槽位；时间戳相等保守钉位=GUI-F8 零回退），flowMessages 稳定 createdAt 排序（时间戳优先+首见序兜底）。M12/audition 判据面零触碰。
  2. **取证修正采信**：卡面"logical id 重插"假设被证据修正为两独立机制叠加（①GUI-F7 链终局永久钉流尾；②waiting_continue 续跑 run 两轮用户行同盖 run 域 turn_id 被吸回首轮组）——ui-state.json 同 run turn_id 实证+E2E 首跑红流形逐位实证，修正路径合规（缺陷属实+域内+验收不变）。
  3. **归因排除**：mergeRestoredChatMessages（FIX-BUCKET-SAVE-RACE-1 交互项）排除为根因，且加了专门归因用例钉死（bucketSaveRace.test.ts +67）——卡面要求的"一并排除/归因"字面完成。
  4. **我方独立复跑**（port 分支 worktree 态）：npm run test **383/383 全绿**（main 基线 375+新 8，与我方在 main 上复跑的 375 对照精确吻合）+ npm run build 过（chunk 警告为既有形态）。
  5. **E2E 工件亲读**：artifacts/e2e_webui1/20261001_114831——webui_rendered_dom_report.json **verdict=pass、failed_groups 空**；msgorder 组以 **M8 取证现场原事件流**（webui_muo6fygb，47 事件含同 id 生命周期翻转+同 id 双发对）conversation-strict 重放，5 节点水合 seeded、appeared=true；首跑红（20261001_112959，修复前流形=机制一实证）按惯例保留——红绿链完整。
  6. **泊位声明合规**：真栈三件套归并行会话①所有、本卡全程未触碰；E2E 隔离泊位（自有 draft root/7907/桥接改道/独立二进制），运行后拆除申报（stopped_agent_pid/port_released）——2026-09-30 收紧条款达标。
  7. **越域上交处置**：同 id 双发治理归 agent 事件面、首轮 endedAt 取 max 时长呈现——两项如实上交不开修，正确。
- 边界与收尾项：活态 composer 驱动时序由单测+既有 driven-turn 组覆盖、未在 E2E 真栈断言（回执已诚实声明）；**用户手测复验（顺序正常）为收尾项**——下次手测轮顺带复验，形态同 AUDITION-LANE-1 先例（修复真机生效由用户确认后销项，不阻塞本裁定）。
- 关联影响：M1 复验第三轮（gate 09-30 排期=等 FS-PARK+WEBUI 修复合入后同场）前置之二就位，剩 FS-PARK 合入。
