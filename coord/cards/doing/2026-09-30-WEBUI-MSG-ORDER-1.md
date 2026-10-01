# WEBUI-MSG-ORDER-1：第二轮输入显示在第一轮输出之上+轨迹动效缺失（M8 手测实测，P1）

- 池序 15（P1 关键呈现缺陷）；来源=M8 手测（证据=coord/runs/M8-FORENSIC-20260930/evidence/）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / 关联 FS-PARK-TURNFAIL-1（其 turn id 复用是触发面之一）；webui 域（E2E 真栈排他与 agent 卡错峰）
- 模型分级：L1 / GLM 或 flash 可接（取证规格已给）
- 已核实事实（用户原述+事件流）：
  1. 第一轮输出正常显示；第二轮输入后：**输入内容显示在第一轮输出的上面、第一轮输出保持在底下**（顺序倒挂）；第二轮无执行轨迹动效；最终输出失败（归 FS-PARK-TURNFAIL-1）。
  2. 事件流（events-webui-muo6fygb.json）：同一 turn_id（run_e5796736）**生命周期翻转**——started(22:04:59)→completed(22:05:04)→items→completed(22:06:37)→started(22:07:20)→failed(22:08:07)；且存在**重复事件对**（trajectory.turn.completed 双发 .743/.743、audition select/stopped/ready 成对 8-14ms 错位——事件面疑似双源发射，取证确认是否既有形态）。
  3. 假设（取证验证）：webui 消息以 logical_message_id 键控——22:07:20 的同 id turn.started 触发 round-1 turn 消息**重插/重挂**到用户 round-2 输入之后（顺序倒挂的直接机制）；轨迹动效缺失=同 id started 对已完成 turn 不触发新动画。
- 目标：
  1. **取证**：本地复现（E2E 或组件测复刻事件序列——同 id 翻转生命周期+新用户输入），确认倒挂的确切机制（重插/重挂/合并面——注意与 FIX-BUCKET-SAVE-RACE-1 的 mergeRestoredChatMessages 交互一并排除/归因）；重复事件对（双源发射）定性。
  2. **修复**：消息序对重复 logical id 稳健（时间戳/首见序兜底，同 id 更新原位不重排）；轨迹动效对同 id 再 started 的呈现语义（复用进度或明确重启动画）。
  3. **回归**：复刻事件序列的渲染用例（倒挂反例）+E2E 组。
- 文件域：agent/webui/src/（消息合并/渲染序+trajectory 视图）+ scripts/webui_rendered_dom_smoke.mjs（加组）。
- 验收标准：npm run test 全绿含新用例+build+E2E-WEBUI-1 新组 exit 0+用户手测复验（顺序正常）。
- 停止条件：取证发现倒挂源于 agent 事件面本身乱序（非渲染端）→ 上交转 agent 卡。
- 领取：2026-10-01T10:52+0800 / 领取时 origin/main=88efaccc001af35170b0d142fb897d83d49db35a / 分支 port/webui-msg-order-1（独立 worktree D:/Vit_DAW_worktrees/webui-msg-order-1，PC 会话②；领取时工作树干净，无先行 diff；并行卡 FS-PARK-TURNFAIL-1 已由会话①领取，chat/agentloop 域与本卡 webui 域不同域）
- 回执 / 验收：空行待填
