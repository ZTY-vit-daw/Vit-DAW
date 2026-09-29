# FIX-AUDITION-TRAIL-1：webui audition 动态轨迹重复渲染——处理时输出内容底下重复多条

- 池序 2（M1 手测缺陷①）；来源=M1 手测（reports/2026-09-29-manual-test-nodes.md M1 行，2026-09-29 晚用户手测 pv1_p01 副本工程）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无；复现环境=D:\Vit_DAW_run\manual_test_m1\pv1_p01\pv1_p01.vit（副本可随意复现）
- 模型分级：L1 / GLM 或 flash；webui 渲染面改动——**须过 E2E-WEBUI-1 渲染烟测门槛（AGENTS §5）**
- 现象（用户原述）："处理时 kernel audition 的动态轨迹会出现在输出内容底下，而且会重复出现好几条"。
- 已锚定事实：audition 呈现在 webui（`agent/webui/src/audition.ts` + `eventPolling.ts` + `audition.test.ts` 既有）；"动态轨迹"=处理中 audition 状态的轨迹事件流渲染。
- 目标：
  1. 取证：复现并锚定重复来源（事件流重复下发 vs 渲染层重复追加 vs 轮询重放）——给出事件侧 vs 渲染侧的定性证据。
  2. 修复：处理期间 audition 轨迹在消息下方至多一条且随状态更新（或按既有 GUI 线轨迹单表面收束设计收编），结束后定型不残留。
  3. 回归：既有 audition.test.ts / eventPolling.test.ts / messageLifecycle 全绿+新增防重复回归用例。
- 文件域：agent/webui/src/（audition.ts / eventPolling.ts / App.tsx 渲染段及测试）；若取证证明重复源在 agent 事件下发侧（chat/journal 面），停下上交证据由决策侧定扩域。
- 验收标准：npm run test 全绿 + npm run build 通过 + E2E-WEBUI-1 渲染烟测含"处理中轨迹单条+结束定型"断言 exit 0 + 手测复现路径回归（用户或决策侧复验）。
- 停止条件：重复源在 agent 事件侧（webui 域外）→ 实证上交。
- 领取：2026-09-29 21:47 +0800 / origin/main=5c628e4753d848baa0e5a5a7aca48d09f077d42e / port/fix-audition-trail-1
- 回执：2026-09-29 22:15 +0800 自验完成 doing→done。**实现 commit `b192c2ab` @ 分支 `port/fix-audition-trail-1`（已推送，待决策验收 cherry-pick；5 文件 +451/−4：turnGroups.ts / App.tsx 渲染段 / auditionTrail.test.ts（新）/ roundStepSurface.test.tsx / webui_rendered_dom_smoke.mjs，全在卡面文件域内）**
  - **取证结论（定性=渲染侧 webui 域内，未触停止条件）**：重复源是流底活动线（lane）。证据链：① 内核 `audition::Session`（VitApp/Source/Service/AuditionPreviewState.h）**无 turn_id 字段**——prepare 期遥测（audition.prepare.started / candidate.ready / ready）的会话快照必不带回合域（真栈 fixture 2026-09-05-webui-mtny2v9x seq27/28/30 实证：src_turn/turn_id/sess.turn_id 全空）；② agent 侧 enrich 事件（同会话首个带 turn 域者）在 mount 完成后才发（mix_tick_audition.go mountMixTickAudition 在 AuditionPrepare 返回后 emit）——遥测先到，以未绑定身份落 lane；③ 族内每个 eventType 各占一条逻辑消息（audition_events.go `audition:{sid}:{type}`），upsertActivity 不折叠 → 处理期同屏多条「已完成：Kernel audition」。AUDITION-LANE-1（2026-09-14，同类手测「底下很多个 kernel audition」）当时按**回合域绑定**排除，漏了无域遥测行——本卡补**族身份**排除（isAuditionFamilyActivity）。事件侧无重复下发（seq 单调、逐事件一次），无需 agent 侧改动。
  - **修复形态**：判定卡为族内唯一动态单表面（每会话一卡、随状态更新、判定后定型——AUDITION-UNSTICK-1 准备态/播放态卡面既有）；lane 组合口收敛为 `turnGroups.laneVisibleActivities`（isUnboundActivity && !isAuditionFamilyActivity），App.tsx 消费。
  - **验收证据**：npm run test 331/331 全绿（含新增 auditionTrail.test.ts 7 用例：真栈 fixture 回放处理期窗口 lane 零 audition 行+单会话；M1 mix-tick 形态全周期恒零；不误伤上传类）+ npm run build 通过 + **E2E-WEBUI-1 真栈 exit 0**（chromium-channel-msedge 154，新增 audition-trail-T1 断言组两阶段同上下文实测：处理中 lane 0 行+判定卡 preparing「正在准备 A/B 试听…」；判定后 lane 仍 0 行+卡面 `card settled`「已裁决 · B 更好 · 保留改动后」；既有 A1-A4/B1-B2/C1/D1/E1/F1/G1/H1 全组无回归）。工件：worktree `D:/Vit_DAW_worktrees/fix-audition-trail-1/artifacts/e2e_webui1_trail1/20260929_220532/`（webui_rendered_dom_report.json verdict=pass、dom-audition-trail-processing/settled.{png,json}、prereq.txt）。
  - **红证边界（诚实披露）**：E2E 绿为修复后单测+真栈实证；修复前红态在单测层锚定（auditionTrail.test.ts 断言遥测行存在且无 turn 域、按旧判据必落 lane——该行 unbound 由 isUnboundActivity 语义直接给出），未做修复前 E2E 对照跑。
  - **端测覆盖边界声明（AGENTS §5）**：E2E-WEBUI-1 渲染面=真 agent 进程（隔离 draft root/端口 7897）+ 真构建 webui bundle + 真浏览器 DOM 断言，事件为归档流+M1 形态注入（网络层回放，无 LLM/内核三件套）；用户旅程（Godot 前端手测复现路径）未覆盖——卡面验收标准第 4 项「手测复现路径回归」待用户或决策侧复验。
- 验收：（裁定文件 / 验收 commit）
- 验收：pass（2026-09-29，决策侧）——rulings/2026-09-29-FIX-AUDITION-TRAIL-1-pass.md；决策侧复跑 npm test 331/331+E2E 工件 verdict=pass 亲核+diff 族身份排除亲核；取证定性采信（含 AUDITION-LANE-1 历史盲区闭环）；合入=cherry-pick b192c2ab（b4a197d7）；手测复现路径留用户下次手测顺带复验。
