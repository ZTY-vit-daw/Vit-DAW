# AB-JUDGMENT-CARD-1：A/B 试听裁决卡交互点击无效（M8 手测实测，P1）

- 池序 16（P1——裁决卡是人耳判断边界唯一入口，点击无效=park 无法 settle）；来源=M8 手测（证据=coord/runs/M8-FORENSIC-20260930/evidence/）
- **证据增补（2026-10-01 M1 第四轮再证，决策侧）**：[runs/M1-RETEST-20261001/FORENSIC-NOTE.md](../../runs/M1-RETEST-20261001/FORENSIC-NOTE.md) 追记+evidence/events-webui_mupimj6f.json——park 期间（20:35:01-20:35:33）用户点击 A好/B好/补充，**事件流零 judgment 类事件**（点击未产生任何服务端痕迹，假设 a/b 的现场再强化）；注意本卡取证须在 FS-ADOPT-CLOSURE-1 合入后做终验（否则采纳路径缺陷会污染第二输入形态）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / 关联 FS-PARK-TURNFAIL-1（park 保持与其同栈）；webui+agent 交互面
- 模型分级：L1 / GLM 首选（跨 webui 交互+agent respond/judgment 链路）
- **裁定（2026-09-30 用户授权二选一，决策侧定：搞通不删卡）**：decisions/2026-09-30-park-adoption-and-judgment-card-ruling.md——人耳 A/B 判断是核心叙事保留；FS-PARK-TURNFAIL-1 裁定落地后本卡**非阻塞**（用户继续对话=默认采纳，卡 settle 为该终态；用户点卡=显式裁决）。
- 已核实事实（用户原述+事件流）：
  1. 第一轮结束后 webui 出 A/B 试听卡，携带 **AB 选择与补充信息交互**；用户点击任一交互"都是无效的"（无视觉响应/无结算）。
  2. 事件流：用户点击期间（22:06:50-58）**内核收到了 audition select/stop 事件**（audition.select.changed/selected/stopped 成对出现）——试听播放链路通；但 **judgment 结算未落地**（无 judgment settled 类事件；trajectory.user_judgment.requested×2 后无消费）；turn.failed 后 pending interaction_requests 为空（卡可能已被 FIX-CONFIRM 的终结收卡逻辑 settle 成终态——点击发生在 fail 前还是后需取证时序精确化）。
  3. 假设（取证验证）：a) 卡的 A/B 选择/补充信息动作未绑定或绑错 respond 通路（judgment POST 端点 vs interaction respond 的分叉）；b) 动作发出但服务端拒绝/静默吞；c) 卡在等待态渲染了不可交互体（交互守卫误伤）。
- 目标：
  1. **取证**：复现点击链路（E2E 或探针）——按钮 → 前端 handler → 网络（哪个端点）→ 服务端日志/事件；锚定断点在哪一层。
  2. **修复**：按断点修（绑定/端点/守卫）；修复后点击 → judgment POST 落地 → park settle（或补充信息路径正确入流）。
  3. **回归**：E2E 组（A/B 卡点击 A→judgment 落地→park 释放）；与 FIX-CONFIRM-CARD-1 的 K1/K2 及 settleTerminatedTurnInteractions 无回归。
- 文件域：agent/webui/src/（判定卡/交互动作绑定）+（若断点在服务端）agent chat judgment/respond 面——实锚后申报。
- 验收标准：取证断点结论+修复 diff+npm test 全绿+E2E 新组 exit 0+用户手测复验（点击有效、A/B 裁决落地）。
- 停止条件：取证发现 judgment POST 端点在当前链路未实现（架构缺口）→ 上交定方案。
- 领取：（2026-10-01 21:40 / origin/main=a134f524 / port/ab-judgment-card-1，worktree=D:/Vit_DAW_worktrees/ab-judgment-card-1，PC 会话②，L1；领取时工作树仅 VitApp/Workspace/default_project.xml 既有运行态改动，非本卡域，不动）
- 取证断点结论（2026-10-01，webui 面，两轮证据+代码锚点）：**判定 POST 的 turn_id 被污染**——`audition.ts:102` 的 turnID 优先链由 02b6b442（B9 统一面）前插 `event.source_turn_id`（run 级），而 `trajectory.user_judgment.requested` 事件 source_turn_id=run id、payload.turn_id=实验 id；判定卡 `judgmentPayload` 的 `turn_id` 复用该字段 → POST 携带 run id → 服务端 `recordFreeStateAuditionJudgment`（audition_events.go:817）`loop.Experiment.ID != request.TurnID` → 409 "audition session identity mismatch"；webui `handleAuditionJudgment`（App.tsx:1463）无 catch 无 setError → 静默吞（无视觉响应+零事件）。两轮证据用 reducer 复刻复核：M8/R4 最终态 turnID 均为 run id、canJudge 均为 true（按钮确已渲染可点）。= 卡假设 a（绑错通路参数）+ b（静默吞）复合；假设 c（守卫误伤）排除。**一字段两语义**：turnID 同时服务卡片挂靠（要 run id，WEBUI-MSG-ORDER-2 三级挂靠依赖）与判定 POST（要实验 id）。
- 回执：（commit hash / 红绿 / 烟测 run ID / 泊位声明）
- 验收：（裁定文件 / 验收 commit）
