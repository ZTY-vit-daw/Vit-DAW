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

- 领取：（2026-10-20 18:58 +0800 / origin/main=0596dbda13c2b907758f77004228502fd7690017 / GLM-5.3 L2 执行会话 PC/ZCode（本卡会话，用户指令指定领取）/ 分支=port/fullaccess-semantic-chain-1 / worktree=D:/Vit_DAW_wt_fasc1 / 领取提交=本提交；领取时核对：卡在 todo 派发已确认（用户 2026-09-13 裁定在卷）、依赖无、doing 三卡文件域零冲突（146 探查设计卡零生产代码/FS-RECEIPT-REVISION-1 在 goalrunner 面/GEN-SUNO 零生产代码）；真栈腿与 146 实验腿串行——栈占用开工前另行登记 PC-RUNTIME-STACK.md）
- 回执：（实现 commit=be57f496（Go 5 文件 +525 行）+烟测腿 commit=fb8de04e（场景+四 run 工件），分支 port/fullaccess-semantic-chain-1 待验收 cherry-pick；锚点清单=①选择卡 internal/chat/plugin_recommendation.go:535 pluginRecommendationSelectionResponse ②装载待确认面 internal/chat/plugin_grabber_workflow.go:202（PendingPlan 驻留+NeedsConfirmation）③typed controller 卡 internal/chat/semantic_dynamic_workflow.go:795 semanticDynamicWaitingResponse；新文件 internal/chat/full_access_semantic_chain.go 承载三 helper+停止挂起诚实跳过（FS-STOP-APPLY-1 Part A 同款）；GAP3 以 semantic_plugin_recommendation_selection 链锚定防误伤 C2 能力会话与 B4 面共享的 planBoundSemanticDynamic；K2 send_route 面零触碰。测试=full_access_semantic_chain_test.go 7 件（两分支钉：full-access 零交互卡零驻留直接执行/manual 现行为逐字不变+显式对+链键门谓词）+cd agent 后 go build ./... 过+chat 包全量过+go test ./... -count=1 全仓 0 FAIL（96 包 ok）+gofmt 净（LF 归一 0 hunk；worktree CRLF 为 autocrlf 检出噪声，blob 面 gofmt 净）。已知偶发记录：TestTurnStopRunnerLandsWithinWindowSkipsInlineFinalize 首次整包跑失败（0.50s，时序敏感）——隔离复跑过+整包复跑过+本卡 diff 零涉及 turn-stop 面→按 §11 记首次偶发待观察。真栈 run=coord/runs/FULLACCESS-SEMANTIC-CHAIN-1/{193433,194123,194750,195057} 四 run，run4 exit 0：M2 主话术（逐字）full-access 零交互卡门 4/4 PASS（chat 响应+runtime 续泊双源、含选卡 kind 检测）+LEG E 确定性装载腿 GAP2 面端到端（rack_add_node ok plugin_id=1044、89 参数回读、零驻留、full_access_auto_authorized=true、PCA 装载门原样运行）+LEG F manual 对照保留确认 plan_685131ba（判别实证）；栈占用 37a52892 登记/释放全链在 PC-RUNTIME-STACK.md。**端测覆盖边界声明**：覆盖=agent HTTP 面（chat/authority/runtime/events）+内核执行证据+manual 判别；**未覆盖**=①NL 主话术三面连跑（选择→装载→typed controller）——2/2 run 模型在改进提案段 final 收束、零处理器物化，链未达分叉面=上游自由态交付缺口（146/149 域，§8 同断点止损，run1/run2 工件+parse_failed/repair_succeeded 调试件在卷），该三面 full-access 行为由单测钉住、typed controller/选择面真栈端到端待上游修复后补验；②渲染面/webui 用户旅程（本卡零渲染面改动，无 E2E-WEBUI-1 新义务）；③A/B 试听面保留为模型所有面未做确定性断言（收据级 requires_fresh_observation 仅链执行时出现）。）
- 验收：（裁定文件 / 验收 commit）
