# LOUDNESS-A-CONFIG：响度监听配置面（工程级开关+预设+用户阈值）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=LOUDNESS-WATCH-DESIGN-1 验收 pass 的实施拆卡规格件五 卡A）
- 派发确认：已确认（主管依已验收设计发卡）
- 验收负责人：GLM 主管决策流
- 池序 89；**规格全文=[设计件五 卡A](../../runs/LOUDNESS-WATCH-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：PC 混音线（D1 响度域·持续证据面）；P1 / 预估见规格 / 无依赖（可立即领）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 要点摘录

loudness_watch_config.v0 挂 projectAgentRuntimeState（RLM-PROFILE-2 先例锚点在规格内）；默认关；rlm 五内置预设+自定阈值（ValidateDeliveryProfile 借用）；持久化与旧工程缺省语义（AGENTS §11）。

- 领取：2026-10-12T09:59+0800 / origin/main 809cab5d5c44b810e3bd521fc08f5a8966327044 / owner=GLM-5.3-Flash PC 执行会话（Windows PC，ZCode flash）/ 分支 port/loudness-watch-config-1 / worktree D:/Vit_DAW_wt_loudness_watch_1 / 领取提交=（本提交）
- 领取时基线：HEAD=809cab5d 工作树已有改动=VitApp/Workspace/Settings/Settings.xml + VitApp/Workspace/default_project.xml（运行时状态，非本卡域）+ coord/runs 未跟踪工件；本卡 diff 从零起算
- 锚点亲核：server.go:164/198/7245/7256 全命中（当前 809cab5d 基线复核，非仅引设计件 8e66e674 核验记录）；rlm/profile.go 形态与规格一致
- 回执：（commit / 单测清单）
- 验收：（裁定文件 / 验收 commit）
