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
- 回执：（commit hash / 取证结论 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
