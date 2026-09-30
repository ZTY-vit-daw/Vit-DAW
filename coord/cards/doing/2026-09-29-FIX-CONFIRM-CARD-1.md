# FIX-CONFIRM-CARD-1：webui 确认卡过期态生命周期+full access 权限感知（M1 手测缺陷②③）

- 池序 3（M1 手测缺陷②③）；来源=M1 手测（reports/2026-09-29-manual-test-nodes.md M1 行）
- 优先级 / 预估 / 依赖：P1 / 0.5–1 天 / 无；复现环境同 FIX-AUDITION-TRAIL-1（pv1_p01 副本，full access 模式开启路径：POST /agent/authority authority_mode=full_project_access 或前端权限头）
- 模型分级：L1 / GLM 首选（含一处权限边界设计落实）；webui 改动须过 E2E-WEBUI-1
- 现象（用户原述）：
  - ② "处理时交互卡片不会消失，点击后会显示已过期"——确认卡生命周期缺陷：任务处理完成/超时后卡片滞留，点击才揭示过期态。
  - ③ "我开的是完全访问，但是还是提供了交互卡片，这说明权限边界设计还有问题"——full_project_access 模式下仍出前置确认卡。
- **决策侧裁定方向（2026-09-29，随卡下发）**：full access 的产品语义=用户授予完全访问——RiskConfirm 级工具在 full access 下**不再出前置确认卡，降级直执**，但**保留事后回执**（执行摘要+可撤销路径可见于对话流）；manual_confirmation 模式行为不变（前置卡照出）。此裁定兼顾"C 路径可逆执行正确"的手测正面结论。
- 已锚定事实：确认卡呈现面=webui（capabilityProposalCard.test.tsx、messageLifecycle.ts、turnControl.ts、App.tsx）；权限模式面=agent 侧 authority（AUTHORITY-LOST-1 先例：full access 跨工程身份边界重置，server.go ~6826 authority 闩；restore 采用盘上态 ~7147）；webui 已有权限头/权限下拉交互（GUI-F3 打磨先例）。
- 目标：
  1. **②过期态治理**：确认卡生命周期与任务态绑定——任务终结（完成/失败/被替代）时卡片即时消失或转为不可交互的终态样式（不残留可点击的"过期"卡）；新增生命周期回归用例。
  2. **③权限感知**：webui 确认卡渲染前感知当前 authority_mode（既有权限面透传/查询入口实锚）；full access 时 RiskConfirm 走直执+事后回执呈现（裁定方向落地）；manual_confirmation 不变。
  3. 回归：capabilityProposalCard/messageLifecycle/turnControl 测试全绿+新增两缺陷回归用例。
- 文件域：agent/webui/src/（确认卡渲染+生命周期+权限感知）+（若权限透传需 agent 侧配合，实锚后小改 chat/server.go 权限查询面并申报越域）。接口冻结：不改变 authority API 语义（只消费）。
- 验收标准：npm run test 全绿+build 通过+E2E-WEBUI-1 渲染烟测含"full access 直执无前置卡+回执可见"与"卡片终态不残留可点击"断言 exit 0+手测路径复验（决策侧或用户）。
- 停止条件：取证发现确认卡出卡决策在 agent 侧而非 webui 渲染层（越域）→ 实证上交定扩域。
- 领取：2026-09-30 09:08 / 9f056c97a84c2c66ee481f9ff92c120d940fe1a0 / port/fix-confirm-card-1（GLM-5.3 执行侧，Windows）
- 回执：（commit hash / 取证结论 / 权限感知锚点 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
