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
- 回执：2026-10-01T12:00+0800，PC 会话②；实现分支 port/webui-msg-order-1（commits 65b0d2a3、dc765ec7，待决策验收 cherry-pick 进 main）
  - **取证结论（卡面假设被证据修正——非 logical id 消息重插，是两个独立机制叠加）**：
    1. **机制一（活态主因）**：GUI-F7 链终局消息（scheduler_chain 终局合成气泡）**永久钉流尾**（GUI-F8 形态）；用户开启下一轮后新消息照常入列即排在其上→「第二轮输入在第一轮输出上面、第一轮输出保持在底下」。水合行与链终局孪生按文本键合并后继承孪生 source_id，同样被钉尾（E2E 首跑流形逐位实证）。
    2. **机制二（重载/水合形态）**：服务端对 waiting_continue 续跑 run 的**两轮用户行盖同一 run 域 turn_id**（ui-state.json 实证 user1/user2 同为 run_e5796736a4865570，assistant 行是 chat 域 turn_*），`groupMessagesByTurn` 把第二轮用户输入吸回首轮组（组序=首现序）→ 第二轮输入渲染在第一轮输出组之前。
    3. **mergeRestoredChatMessages（FIX-BUCKET-SAVE-RACE-1）排除为根因**：合流层输出保序（bucketSaveRace 归因用例钉死），盖章 turn_id 由其忠实带给乐观行——倒挂发生在下游分组/钉尾层。
    4. **重复事件对定性**：audition.ready/stopped ×2、trajectory.user_judgment.requested ×2 = **同 id 双发**（同 item_id/logical id，0.5–14ms 错位，内核遥测+agent 侧双源既有形态），呈现面已按 logical id 幂等折叠（upsert 原位），不构成用户可见缺陷，治理归 agent 事件面（越域上交，本卡不开修）；trajectory.turn.completed 同 ms 双发 = 实验回合与 run 壳**各自合法收口**（item_id 不同，非重复）。
    5. **轨迹动效语义**：取卡面授权的「复用进度」——同 run 再 started 不新开块（事件面本就无第二轮 trajectory.* 事件，出不了第二块），块锚定开启消息不迁移；发送期动效由 OptimisticTraceBlock 承担。首轮 endedAt 会因 round-2 failed 取 max（时长呈现含等待），未动（越域，另行上交）。
  - **修复**：① renderPlan：链终局只在仍是最新内容时钉尾（时间戳相等保守钉位，GUI-F8 零回退），被超越者回流入时序槽位；flowMessages 稳定 createdAt 排序（时间戳优先、首见序兜底）。② turnGroups：用户消息只在回合组为尾随组时并入，同 run 新一轮输入自成独立组保流位（assistant 合并规则不变）。
  - **验收**：npm run test 33 文件 **383 用例全绿**（新用例 8：M8 水合行倒挂反例、成因②钉按修正语义更新、同 id 生命周期翻转重放 ×4、合流保序归因、被超越链终局反例、最新链终局钉尾保持）；npm run build 过（chunk>500kB 警告为既有形态）；**E2E-WEBUI-1 含新组 msg-order-M1：exit 0，verdict=pass，23/23 组全绿**，工件 artifacts/e2e_webui1/20261001_114831（prereq 记录 head/二进制与 bundle 哈希/端口/桥接改道；首跑 artifacts/e2e_webui1/20261001_112959 为修复前红，M1 流形=机制一实证，按脚本惯例保留）。
  - **端测覆盖边界声明**：msg-order-M1 覆盖「水合行+归档事件网络层重放」的渲染面时序；活态 composer 驱动的时序由单测与既有 driven-turn 组覆盖，未在本组真栈断言。泊位隔离见下。
  - **泊位声明**：三件套栈（VitApp/Godot/VitAgent，2026-10-01 11:22 起）归并行会话①（FS-PARK-TURNFAIL-1）所有，本卡全程未触碰；本卡 E2E 按包装脚本隔离泊位运行（自有 draft root、HTTP 7907、桥接端口 4454/4455/5565/5566 改道、独立二进制），运行完已拆除（stopped_agent_pid=16236，port_released=7907），无遗留进程、无共享端口复用。
  - **待用户**：[等待用户:手测复验（顺序正常）]
- 验收：**pass（2026-10-01 决策会话）**——rulings/2026-10-01-WEBUI-MSG-ORDER-1-pass.md；合并 main=0e7ba86d/8e21258e（决策侧 cherry-pick）；diff 直读+我方复跑 383/383+build+E2E 工件亲读（verdict=pass/failed_groups 空/M8 现场重放）+泊位合规；用户手测复验为收尾项（下次手测轮顺带销项）。
