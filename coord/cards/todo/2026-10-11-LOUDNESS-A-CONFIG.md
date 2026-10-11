# LOUDNESS-A-CONFIG：响度监听配置面（工程级开关+预设+用户阈值）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=LOUDNESS-WATCH-DESIGN-1 验收 pass 的实施拆卡规格件五 卡A）
- 派发确认：已确认（主管依已验收设计发卡）
- 验收负责人：GLM 主管决策流
- 池序 89；**规格全文=[设计件五 卡A](../../runs/LOUDNESS-WATCH-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：PC 混音线（D1 响度域·持续证据面）；P1 / 预估见规格 / 无依赖（可立即领）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 要点摘录

loudness_watch_config.v0 挂 projectAgentRuntimeState（RLM-PROFILE-2 先例锚点在规格内）；默认关；rlm 五内置预设+自定阈值（ValidateDeliveryProfile 借用）；持久化与旧工程缺省语义（AGENTS §11）。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 单测清单）
- 验收：（裁定文件 / 验收 commit）
