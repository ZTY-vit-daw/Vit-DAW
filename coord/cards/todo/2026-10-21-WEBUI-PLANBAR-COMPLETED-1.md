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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 测试名 / 渲染面端测边界声明）
- 验收：（裁定文件 / 验收 commit）
