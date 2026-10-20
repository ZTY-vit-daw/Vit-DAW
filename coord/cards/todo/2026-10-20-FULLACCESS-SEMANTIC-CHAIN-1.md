# FULLACCESS-SEMANTIC-CHAIN-1：完全访问权限传导 M2 语义处理链确认面

- 发卡：GLM 主管决策流 / 2026-10-20 晚窗（依据=M2 体验场次①实录 F1：[2026-10-20-m2-experience-session-1.md](../../reports/2026-10-20-m2-experience-session-1.md)）
- 派发确认：已授权——用户 2026-09-13 裁定原文在卷（「完全访问权限应该是它能够自主进行所有执行，我只负责AB试听」），本卡=该既有契约在语义链上的落地，非新设计
- 验收负责人：GLM 主管决策流
- 池序 147；目标仓库=D:\Vit_DAW；P2 / 0.5-1 天 / 依赖=无（与 148 文案卡文件域相邻，串行领取）
- 模型分级：L2 / GLM 执行会话

## 缺陷（场次实录实证）

authority=full_project_access（显式，agent_last.log 18:18:15 switch accepted + restore kept explicit）下，语义处理链仍弹三张交互卡：①插件选择卡（18:19:35 stop=plugin_selection_required，**仅 1 个候选 Pro-R**，用户阻塞 60 秒）；②装载确认卡（「这个操作需要确认后执行」）；③typed controller 确认卡。既有契约三处在卷而本链未接：

1. **FULLACCESS-AUTONOMY-1**：`agent/internal/agentloop/full_access_autonomy.go:10-38`——引用用户 2026-09-13 裁定，明言「Asking the user to pick a direction, a processor family, a plug-in instance, or a dose is a behavioral defect in this mode」+「without a per-step confirmation card」；
2. **mix_tick 面 2026-09-11 改判先例**：`agent/internal/chat/mix_tick_confirmation.go:531-537`——full_project_access 下 improvement-proposal 路由立即应用不发确认卡；
3. **harness PCA load gate 的 full-access 通道**：`explicitFullProjectAccess` / authorizeFullProjectAccessLoad + **PCA-AUTONOMY-CATALOG-1**（`agent/internal/chat/pca_autonomy_catalog.go`——full-access turn 的准入目录披露，模型可自主选通过门的插件）。

缺口锚点：`agent/internal/chat/plugin_recommendation.go:536-640`（pluginRecommendationSelectionResponse 无任何权限模式分叉，无条件 StatusWaitingClarification+interaction 卡）；后续装载/typed controller 确认面领取侧定位（预期 improvement_proposal_workflow / semantic 控制器链）。

## 目标

显式 full_project_access turn 下，语义处理链整链自主执行：候选选择由模型按 PCA 自主目录/推荐序自定（单候选时直接选主推荐），装载与控制器执行**不发交互卡**；A/B 试听仍为唯一用户面（与 2026-09-13 裁定一致）。manual_confirmation 行为保持逐字不变。

## 文件域

`agent/internal/chat/plugin_recommendation.go` + 装载/typed controller 确认面所在文件（领取侧先出锚点清单；若确认面分散多文件，锚点清单交决策侧再定扩域）+ 测试。越域即停。

## 验收标准

`cd agent && go build ./...` + chat 包 + 全量 `go test ./... -count=1` 0 FAIL + gofmt 净；单测钉住两分支（full-access=零交互卡+直接执行；manual=现行为逐字不变）；真栈烟测腿按 §5 真实三件套验证 full-access 主话术端到端无交互卡且 A/B 面保留——烟测脚本扩展按现有 ps1 体系；端测覆盖边界在回执显式声明。

## 停止条件

- 发现更晚的用户裁定要求「插件装载必须用户确认」（与 2026-09-13 冲突）→ 上交裁定锚点停止；
- 语义链确认面牵出 K2 send_route 之类的确定性工作流共用代码 → 锚点清单上交，防越域改坏显式 send 面。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 测试名 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
