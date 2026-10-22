# WEBUI-PLANBAR-COMPLETED-1：链正常完成后 PlanBar 误标「已中断」（活跃度启发式与完成态混淆）

- 发卡：GLM 主管决策流 / 2026-10-21 深夜（依据=M2 场次③：goal 机械面 completed+chainEnded=true+journal 入账，webui PlanBar 显示灰态「已中断」）
- 派发确认：已确认（场次③用户目检+日志对照在卷）
- 验收负责人：GLM 主管决策流
- 池序 163；目标仓库=D:\Vit_DAW；P2 / 0.25 天 / 依赖=无；模型分级：L1 / flash 可接

## 现象与机制（代码已核）

`webui/src/composer/PlanBar.tsx:40,68`——链活信号（agentTurnRunning || trajectoryLive）熄灭即降级灰态「已中断」，**不读 goal 终态**：正常完成（completed/已交付）与真中断同形。场次③实锤：链 22:48:56 完成收工后活信号自然熄灭→误标。

## 范围

PlanBar（及同款消费该信号的组件）区分三态：**活链**（现行绿/正常态）/ **正常完成**（读 goal/trajectory 终态=completed → 显示「已完成/已交付」，不降灰）/ **真中断**（stale：曾有链且无终态）。既有 stale 语义（无终态时不假装正在观察）保持。数据源优先 trajectory/goal 终态字段；缺终态字段时维持现状 stale 行为（fail-open）。

## 文件域

`agent/webui/src/composer/PlanBar.tsx` 邻域+组件测试（vitest，messageLifecycle 同族）。越域即停；**渲染面端测边界声明必填**（E2E-WEBUI-1 浏览器级断言面未就位前，按 AGENTS §5 渲染面门槛如实声明）。

## 验收标准

组件测试三态（completed→完成态不灰/stale→已中断维持/活链→现行态）+既有测试零回退+`npm run test`+`npm run build` 全绿+渲染面端测边界声明。

## 停止条件

终态字段在 webui 侧无既有数据源可读（需 agent HTTP 新增字段）→ 锚点上交（agent 面另卡）。

- 领取：2026-10-22 09:56 +0800 / origin/main=1ceb412d21d19d35e944afb1f81d0868a8705f06 / owner=GLM-5.3-flash L1 PC ZCode 执行会话（163 卡会话）/ 分支=port/webui-planbar-completed-1 / worktree=D:/Vit_DAW_wt_pb163 / 领取提交=本提交（coord/pb163-claim；领取核对：卡在 todo 派发已确认、依赖无、在飞 doing 卡文件域（GEN-SUNO=coord/runs 勘察域、162=agent Go 内部域）与本卡 webui PlanBar 零冲突、webui 改动无真栈占用）
- 回执：（实现 commit=fb257279 @ port/webui-planbar-completed-1（基线 origin/main=1ceb412d；领取时 worktree 干净、本卡 diff 即全部改动：PlanBar.tsx +26/-6、PlanBar.test.tsx +73、details.tsx +1，未越卡面文件域）。**机制与修法**：webui 侧终态数据源**已存在**——trajectory 投影自带 `task.goal_status`（agent `task_runtime_trajectory.go:80`，goal 状态机权威投影；agent 零改动，停止条件未触发）；场次③误标根因=PlanBar 只读 semantic.state（动态链 owner_turn_closed 族终态不写语义态）+链活信号熄灭即落 stale。修法=planBarPhase 增 goal 终态证据参：无活链非终态且 goal_status∈{completed,stable}（agent 侧同映射 TaskStatusSettled）→ terminal「已完成」不降灰；缺终态证据维持 stale fail-open 现状；呈现词汇收口 displayState（图标/标签同步换终态词，避免绿勾配「正在观察」错位）；完成态沿用既有终态 6s 停留+淡出收起生命周期（PLANBAR-1 裁定）。details.tsx semanticLabels 补 `completed: "已完成"`（semanticLabel 仅 PlanBar 消费，trace 组件不受影响）。**测试**：新增 describe「PlanBar 三态（WEBUI-PLANBAR-COMPLETED-1）」5 条（相位分类矩阵含缺参 fail-open 兼容/完成态呈现/stable 同享/真中断维持/活链不受影响）+既有 459 测试零回退（38 文件全绿，含 PLANBAR-1 位置与生命周期契约组）+`npm run build`（tsc --noEmit+vite build）全绿。**渲染面端测边界声明**：本卡覆盖=组件级 DOM 断言（renderToStaticMarkup：data-plan-phase 相位属性/is-terminal/is-stale 类、已完成/已中断标签、spin 图标缺失、收起时间表）；**未覆盖**=浏览器级渲染面烟测（scripts/webui_rendered_dom_smoke.mjs 需真实 VitAgent 进程+取证事件夹具，其断言组 A1–G1 面向 trace 块/试听、无 PlanBar 完成态场景，本卡按卡面口径零真栈占用未运行）——PlanBar 完成态在真实栈终局数据下的浏览器渲染待决策侧裁定，建议与既有手测场次④合并目检（场次③复现话术跑完后观察输入框上方栏显示「已完成」而非灰态「已中断」，6 秒后自动收起）
- 验收：（裁定文件 / 验收 commit）
