# UI-CONVERGE-RECON-1：surface 收敛勘察——装修深度+模块拆解+嫁接位+分阶段方案

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=[用户裁定：surface 收敛](../../decisions/2026-11-15-surface-convergence-ruling.md)）
- 派发确认：已确认（用户「可以勘察」）
- 验收负责人：主管决策流（+用户过目分阶段方案）
- 池序 368；P1 / 0.5 天 / 依赖=无；PC；零栈只读勘察（不改任何生产文件）

## 勘察范围（产出报告 `coord/runs/UI-CONVERGE-RECON-1/REPORT.md`）

1. **App.tsx 内容盘点**（11,842 行）：按域归类全部功能块（对话面/设置/资料库/历史树/audition/authority/插件管理/surface 分发/…），每块行数区间+耦合度标注——回答用户「多重内容堆单一脚本」的精确画像。
2. **样式体系深度**：旧样式的存在形态（内联 style/组件级 CSS/全局 css 文件分布）、v5 令牌（v5.css 变量+344 组件）挂载点与冲突面——装修工作量的决定项。
3. **T9 模块拆解方案**（联合规划核心）：拆分顺序与目标结构（建议按域 → 逐模块可独立验收），标注每步的可回归测试面（vitest 584 基线+test:render）。
4. **工程管理层嫁接位**：recents/新建/打开/进出语义（launcherData/launcherBridge/agent 端点）挂进旧 surface 壳的具体接入点；桥协议保留清单（entrySurfaceBridge/governance 事件/surface ack——SHELL 系契约零破坏）。
5. **旧启动入口收编清单**：工程内旧 webui 对话框的全部启动路径（枚举）+关闭/重定向方案。
6. **分阶段实施卡草案**：阶段 1=壳装修+工程管理挂载（「成为新启动页」时刻，含磁带卡 366 设计落位）+阶段 2=工具页逐块 v5 化；每卡文件域/验收/预估。
7. **风险与工作量定价**：巨石装修风险点、双轨过渡期方案（现 LauncherSurface/ProjectSurface 退役时点）。

## 非目标

- 不改任何生产文件；不实施拆分/装修（实施卡另发）。

## 文件域

只读勘察 `agent/webui/src/**`（重点 App.tsx/styles/v5 令牌/components/launcher/project）+ 前端仓 VitShell 桥面只读；产出仅 coord/runs/UI-CONVERGE-RECON-1/ 工件。

## 验收标准

报告七节齐全+模块画像有行数/耦合度数据+嫁接位精确到锚点+分阶段卡草案可直接发卡；主管过目后转用户过目。

## 停止条件

无（只读勘察）。

## 领取/回执/验收：待回填
