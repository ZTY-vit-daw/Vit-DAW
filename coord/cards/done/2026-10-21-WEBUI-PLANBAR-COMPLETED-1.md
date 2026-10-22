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
- 回执：实现=port/webui-planbar-completed-1@**fb257279**（3 文件 +94/-6：PlanBar.tsx 三态+PlanBar.test.tsx 三态测试 73 行+details.tsx completed 标签）。测试名=PlanBar 三态族（completed→terminal「已完成」不降灰/stale→已中断维持 fail-open/活链→live）+既有 459 测试零回退+npm run build 绿（执行侧自验+主管复跑同形态）。渲染面端测边界=浏览器级 DOM 断言面未就位（E2E-WEBUI-1 建设卡缺位），本卡以 headless 组件级 vitest 如实声明——planBarPhase 纯函数三态+displayState 词汇/图标收口断言。
- 验收：**通过**（GLM 主管决策流 / 2026-10-22 晚窗）。核对：diff 亲读（goalStatus=快照 task.goal_status 权威投影；completed/stable→terminal；缺证据 stale fail-open 防假活链与假完成双误）；主管复跑 **vitest 459/459+build 绿**（worktree pb163）。验收 commit=**28b52f77**（merge）。
