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
- 回执：实现 commit 8c8cc79d（port/fix-confirm-card-1，8 文件 +1098/-24，零 Go 改动）。
  - **取证结论**：①出卡决策确在 agent 侧（server.go chatResponseForCommands:3175 policy.NeedsConfirmation + goalrunner_chat.go:2068/2088 出三型二元闸卡：confirmation/mix_tick_confirmation/mix_treatment_confirmation），但裁定方向可在 webui 渲染层落地——渲染前拦截+以 approve 决策自动走既有 /agent/interaction/respond、/agent/confirm 通路，未触发停止条件，authority API 只消费（接口冻结遵守）。②缺陷②根因三联：(a) server 无应答过期只删卡不发 interaction.resolved（clip_fade_gain_guard.go expirePendingInteractionsForPlan/ForConversation）→呈现面补回合终结/同族替代收卡信号；(b) interactionGuard 原把 proposal 卡排除在盖章范围外（水合边界复活死卡）；(c) mergeActionRecord 在 existing 已结算（按钮已摘）时回退取 incoming.actions 把死卡按钮复活——E2E K1/K2 实证（settle 后二次同键合并、台账盖章版合并两形态）。③M1 的卡走 StandardActionCard 路径（无权限感知）；GUI-T4 既有兜底只覆盖 CapabilityProposalCard。
  - **权限感知锚点**：uiState.authority_mode ?? runtimeStatus.authority_mode（App.tsx 既有恢复效应，agent /agent/ui/state:1111 已透传）+ GET /agent/authority 既有查询面；E2E K2 以权限胶囊"完全访问"就绪为发送前置消解权限恢复竞态（真实用户同此线索）。
  - **实现面**：②settleTerminatedTurnInteractions（turn.completed/failed/stopped 真终结即收卡，waiting_continue 切片驻留不收）+settleSupersededInteractionFamilies（同 plan/同 mix workflow 新卡替代旧卡）+guard 盖章扩到 proposal 卡+proposalCardOutcome 三终态文案（已失效·回合已结束/已替代·以最新方案为准/已处理·交互已关闭）+mergeActionRecord 已结算不复活按钮。③fullAccessDirectExecutionTargets（kind ∈ confirmation/mix_tick_confirmation/mix_treatment_confirmation+approval.requested，二元 approve/cancel 且无表单才命中；proposal_approval/选卡/表单排除）+stripFullAccessDirectExecutionActions（剥卡+一次性提示改直执通告）+runFullAccessDirectExecution（approve 应答→回执入流→台账；失败回滚台账+补回可交互卡）+composer 投影面同谓词自动应答（id 去重防重放）。
  - **端测边界声明**：npm run test 354/354 全绿（含新增 3 描述块：lifecycle 终结/替代、直执目标/剥卡、merge 不复活按钮）；npm run build 通过；E2E-WEBUI-1 渲染烟测 run artifacts/e2e_webui1/20260930_100412 **exit 0**，新增 K1（manual 模式：待裁卡可点击前提复现→回合终结后零可点击残留+原地沉淀+respond 端点零自触）与 K2（full access：权限胶囊就绪→无前置卡+respond 恰好一次+直执通告+回执可见+台账记录）两组，既有 A1-A4/B/C/D/E/F/G/T/H 组零回归。Go 侧零改动（go build/test 不适用）。
  - **边界与遗留申报**：(1) capability proposal（proposal_approval）维持 GUI-T4"完全档·已直接执行"视觉兜底、未纳入真直执（裁定文案=RiskConfirm 级工具；proposal 是 capability runtime 方案审批流）——注意该兜底只改呈现不应答，capability session 会停在 waiting，若需 proposal 真直执建议另开卡（需 agent 侧配合）；(2) improvement_proposal_confirmation 未纳入直执（M1 未涉及，方案类产品面）；(3) mix_treatment_confirmation 已纳入（用户动作请求确认链，与 mix_tick 同族）；(4) 直执通告文案须避开英文 "confirm"（isDisposableConfirmationPromptText 启发式会在 history sync 误删，K2 二轮实证，已在常量处注释钉住）。[等待用户手测] pv1_p01 副本 full access 路径 C 复验（决策侧或用户）。
- 验收：**pass（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-FIX-CONFIRM-CARD-1-pass.md](../../rulings/2026-09-30-FIX-CONFIRM-CARD-1-pass.md)；合并 commit 59a9e8a0（cherry-pick）；复跑 npm run test 354/354+E2E 工件亲读 verdict=pass+直执谓词语义抽查；proposal 真直执留 vit note §7.2 线统一处置；[等待用户手测] C 路径复验与 M1 复验同场不阻塞。
