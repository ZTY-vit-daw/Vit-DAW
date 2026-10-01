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
- 回执（2026-10-01 21:50，执行侧自验完成，待决策验收）：
  - **修复 diff**（4 文件 +1 测试 +1 E2E 组）：①`audition.ts` 拆双语义——新增 `experimentTurnID`（判定契约域：rawSession.turn_id ‖ payload.turn_id，run 级键不入），`turnID` 保持挂靠语义零回退（WEBUI-MSG-ORDER-2 三级挂靠不动）；②`TrajectoryAuditionPanel.tsx` judgmentPayload `turn_id` 改走 experimentTurnID（三个判定席位 A 直选/听不出差别/补充输入同收口，mix-tick 席位同治——其 record.TurnID 亦经 payload.turn_id 同键）；③`App.tsx` handleAuditionJudgment 补 catch→setError（409 显形）；④**顺带真缺陷**：refreshState 在飞轮询（挂载期/8s 周期）迟到的成功分支会无条件 setError("") 抹掉交互面刚显形的错误（E2E J1 MutationObserver 实测：banner +26ms 渲染、+706ms 被挂载期轮询清除）——改为所有权语义（runtimeOwnedErrorRef：成功只清运行时自写错误）；⑤**扩域申报（agent chat judgment 面，卡预授权"实锚后申报"）**：handleAuditionJudgment 两条拒绝路径补 Warn 日志（取证盲区收口——两轮取证因 409 零日志无法区分"未发出"与"被拒"；无行为变化）。
  - **红绿**：新测试组 `auditionJudgmentIdentity.test.ts` 6 钉——撤源修复复跑 RED 6/6（其中 3 钉断言级红），恢复后 GREEN 6/6；npm test 全量 **399/399 绿**（34 文件）；`npx tsc --noEmit` 0 错。
  - **E2E 新组（渲染面端侧烟测）**：`judgment-identity-J1` 加入 `run_webui_rendered_dom_smoke.ps1` 体系——真实浏览器点击「A 更好」→ POST /agent/audition/judgment 网络层截获：turn_id=实验域（run 级键回归钉）+ 409 拒绝显形为黄条（含吞咽竞态修复）。**全量 run `artifacts/e2e_webui1/20261001_213539` exit 0**：A1-A4/B/C/D/E/F/G/T/**J1**/K1/K2/O1/H1/M1/M2 共 26 组全 PASS（FIX-CONFIRM K1/K2 与 settleTerminatedTurnInteractions 面零回归）。
  - **Go 侧**：`go build ./...` 0 错；`go test ./internal/chat` 全包 ok（114s，含 judgment_park_continuation（POST→park settle 服务端链）与 mix-tick judgment 面）。
  - **泊位声明**：E2E 为隔离 agent（自有 draft root+HTTP 7897）；因用户活栈占默认桥接口（VitApp 5555/5556/4444/4445、VspHub 8787，21:08 实测在跑），按脚本既有 VIT_AGENT_* 文档化环境变量错峰（ZMQ 15555/15556、UDP 14444/14445、VSP 注册置空关闭）——与活栈零接触；首次未错峰尝试死于 UDP 4445 bind，记 env_failure（artifacts/e2e_webui1/20261001_210825，非功能结论）。
  - **终验边界声明（卡预置条件）**：①真栈三件套（Godot 前端→工程→判定卡真人点击链）烟测未跑——用户活栈占端口（错峰等待）+ 卡明示终验须在 **FS-ADOPT-CLOSURE-1 合入后**（截 21:50 仍未合入，仅在其自有分支 port/fs-adopt-closure-1 领取态）；②用户手测复验（验收标准末项）留待决策侧安排——建议 FS-ADOPT 合入+本卡验收后一并做（一次手测覆盖两卡的 park 二次输入形态）。服务端 POST→park settle 语义已由 Go 测试覆盖，E2E J1 覆盖点击→POST 契约；未覆盖面=真实内核试听声+park 释放的端到端真人链路。
- 验收：**pass（2026-10-01 决策会话；终验留合并后手测）**——rulings/2026-10-01-AB-JUDGMENT-CARD-1-pass.md；合并 main=fb1c3771（决策侧 cherry-pick）；三层复合根因采信（turn_id 契约域污染→409→静默吞+轮询抹错竞态）+diff 直读双域分账+我方复跑 399/399+tsc+chat 包+E2E J1 组 26 组零失败亲读+泊位错峰合规；终验=FS-ADOPT 已合入后与用户手测合并一场（park→点卡→settle→新输入全链）。
