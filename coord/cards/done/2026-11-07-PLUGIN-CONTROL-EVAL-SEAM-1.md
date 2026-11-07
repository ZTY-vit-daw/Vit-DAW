# PLUGIN-CONTROL-EVAL-SEAM-1：能力评估 A/B 纯臂四缝（test-only，B4/C4 钩先例）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[299 PREP seam_anchors](../done/2026-11-07-PLUGIN-CONTROL-EVAL-PREP-1.md)+plan.json+[300 P 基线落地](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)——P=5/6 但 T2/T3/T4 首动作均 typed 接管=产品语义机器承重，模型裸能力未测；四锚点污染面已定位）
- 派发确认：已确认（主管发卡·装置类，B4/C4 ablation 钩先例=答辩前同类先做过的 test-only env 模式）；验收负责人：GLM 主管决策流
- 池序 305；目标仓库=主仓 agent（四锚点 test-only 缝）；P1 / 0.5-1 天 / 依赖=300 runner 在库（其 driver 的臂面已备 A/B 钩位）；L2 / PC；零栈

## 四缝（全 Go 侧，单 env 总开关 `VIT_PLUGIN_CONTROL_EVAL_SEAM`，默认 unset 字节不变）

1. **S1 typed 接管旁路**：goalrunner_chat.go explicit_control 强制 typed 路径（:260-307 域）——缝开时控制类请求放行通用工具循环（不进 typed planner）；
2. **S2 语义字段摄取剥离**：内核回复烘焙字段（normalized_role/display_group/control_relevance/template_role/alias 语义覆盖——PREP 字段来源三分表）在 agent 摄取/披露面剥离（插件原生 unit/range/enum/当前值保留）；
3. **S3 工具描述净化**：catalog.go:1173 域工具描述中的语义引导文本净化（裸参数描述保留）；
4. **S4 message loop typed 路由**：message_loop.go:4344-4351 typed 路由规则旁路。

## 目标

1. 四缝实现（每缝独立可评注释；默认 unset 生产行为字节不变——B4/C4 配对单测三态同款：unset=现行为/env=1=旁路生效/仅精确 "1"）；
2. 配对单测：缝开时 A 臂条件亲证（typed 不接管+语义字段不披露+裸工具描述）+缝关字节不变+逐缝开关独立性（如四缝可独立开关则各配一测，实现裁量）；
3. 与 300 driver 联调：dryround 在缝开 env 下跑通一案例（模型面可后验）；
4. 回归：`go build ./...`=0；`go test ./internal/chat ./internal/agentloop ./internal/tools -count=1` 全绿零回退。

## 约束

- 文件域=四锚点文件±测试；**生产行为默认不变红线**（生产启动路径永不设置该 env）；不触 300 scripts（其 driver 已备钩位）；
- 禁止 git reset --hard / git clean；不动主工作树；零真栈（dryround 联调如需栈则候队列登记）。

## 停止条件

某缝的旁路与在案治理裁定冲突（如 typed 接管曾有安全裁定）→该缝取证上交单独裁定，其余缝不受阻。

## 领取/回执/验收

- 领取：2026-11-07 21:32 +0800 / origin/main=78bfb2bb8540062a56fabe3756f413cf20bce934 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 305 seam runner；前一代理死于账号限额未领取，本会话全新领取；其遗留 worktree 仅零内容暂存改名已清理重建） / 分支=port/plugin-control-eval-seam-1 / worktree=D:/Vit_DAW_wt_305seam / 领取基线：HEAD=78bfb2bb（fresh worktree 自 origin/main），git status --short=空、git diff --stat=空（全净）。前置核对：卡在远端 todo/ 无 owner；doing 五卡中 302/304（VSTI-WRITEGUARD+DELAY-APPLY-MS-FIX）同触 goalrunner_chat.go 但守卫段 1220-1460 与本卡 S1 段 260-307 不同段（302 领取时已注记本卡在途、push 冲突按 §2.1.3 重放）；CLOSURE-ROUTE-ADAPT 触 chat 路由面与本卡 S4 message_loop.go:4344-4351 单点旁路待 push 前复核；其余（GEN-YUE-DIALECT=genapi、ARRLOOP-AUTHBRIDGE=harness）不相交。
- 回执（执行侧 2026-11-07 22:35 +0800 收口，mv done/ 待主管终验）：
  - **实现 commit**：b2d6457c @ 分支 `port/plugin-control-eval-seam-1`（已推 origin；基线 78bfb2bb 全净领取）；diff=四锚点文件+4 测试文件，8 文件 +721/-12：`agent/internal/chat/goalrunner_chat.go`（S1）、`agent/internal/harness/harness.go`（S2）、`agent/internal/tools/catalog.go`（S3）、`agent/internal/agentloop/message_loop.go`（S4）±各自 `plugin_control_eval_seam_test.go`。**不触 scripts/、不触 VitApp/、不动主工作树**。
  - **四缝开关矩阵**（单 env 总开关 `VIT_PLUGIN_CONTROL_EVAL_SEAM`，仅精确 "1" 开缝；""/"0"/"true"/"off" 全不生效；四缝共用总开关=卡面「单 env」裁定，独立开关不适用故未配逐缝独立性测试——实现裁量声明）：
    | 缝 | 锚点 | 开缝行为 | unset |
    |---|---|---|---|
    | S1 typed 接管旁路 | goalrunner_chat.go runAgentLoopChat 分类块（原 :289 后接线）| semantic_entry 判 explicit_control 后改写 route=other/control_mode=ordinary_agent（authorization/scope/confidence 保留，reason 重写留审计）→ capacity routing 选 ordinary_conversation（通用工具循环，不进 typed planner）| 决策原样直通=typed 接管（P 基线行为）|
    | S2 语义字段摄取剥离 | harness.go executeKernelCommand/executeKernelCommandVSP 回复摄取点 + publicPluginParametersResult/compactQuickControls/compactPluginParameterSnapshotRows 成型面 | 内核回复就地剥五字段（normalized_role/display_group/control_relevance/template_role/alias，参数行+quick_controls 行+顶层 template_role）；成型面不再发射该键（null/空串也不教 schema）；插件原生 param_id/name/raw_param_name/unit/min/max/value/value_text 保留；快照缓存仅键于 param_id，写守卫不弱化 | 不剥不 suppressing，字节不变 |
    | S3 工具描述净化 | catalog.go defaultSpecs set_plugin_param 行 | 描述只剩裸句 "Set one explicit plugin parameter by normalized value (0.0-1.0)."（display_domain_candidate 配方+value_text 警示句移除）| 原文逐字 |
    | S4 message loop typed 路由 | message_loop.go messageLoopOrdinaryFamilySystemSkeleton | 裁掉八条 typed 路由规则（"- Before freezing an actionable generic EQ semantic_action" 至 "- Mixing is a native…" 前），置于 verified-entry 裁剪之前（组合态不残留后半段）；邻接规则与无关规则保留 | 骨架字节不变 |
  - **测试命令与退出码**（worktree D:/Vit_DAW_wt_305seam/agent）：`go build ./...`=**0**；卡面回归 `go test ./internal/chat ./internal/agentloop ./internal/tools -count=1`=**0**（chat 133.8s/agentloop 4.7s/tools 0.4s 全 ok）；追加 `go test ./internal/harness -count=1`=**0**（S2 改动面）与全量 `go test ./... -count=1`=**0（97 包 ok 零 FAIL，B4/C4 配对单测在内的全包兜底）**；四缝配对单测 16 项全 PASS（TestSeamS1×5/TestSeamS2×4/TestSeamS3×2/TestSeamS4×5，三态=unset 现行为+env=1 旁路+仅精确 "1"）。
  - **A 臂条件亲证断言摘录**（三面各一测）：S1 面 TestSeamS1OpenControlRequestSkipsTypedTakeover——LLM double 判 explicit_control 后经生产分类序列，断言 `entry.Controller == ordinary_conversation`（对照 TestSeamS1ClosedKeepsTypedTakeover 断言 `== direct_typed_action` 钉 P 基线）；S2 面 TestSeamS2OpenPublicResultDisclosesNoSemanticOverlay——fakeKernel 回真实形态回复经 harness.Invoke 全路径，断言 Result 无 template_role/normalized_role/display_group/control_relevance/alias 且 param_id/name/unit/min/max/value/value_text 在场；S3 面 TestSeamS3OpenModelToolListCarriesBareDescriptionOnly——断言模型工具表 plugin.set_parameter 描述==裸句。
  - **dryround 联调边界**：300 driver 的 dryround 需 agent_http+内核真栈（fixture 构建+写+回读），本卡零栈未跑；缝开 env 下 dryround **候队列**（下一真栈窗口由主管排期，栈登记面无本卡占用）。
  - **域外观察两件（只锚点不扩修）**：① message_loop.go:4422 verified-entry 裁剪的规则段起始标记 "- For an actionable generic static-EQ listening goal" 在现行骨架文本中不存在（骨架实为 "- An actionable abstract static-EQ…"），该裁剪半段恒 no-op（示例段半段仍活跃）——unset 路径行为属生产现状，本卡不动，测试 TestSeamS4ClosedVerifiedEntryTrimUnchanged 注释留档；② harness.go 基线（origin/main 78bfb2bb）本就非 gofmt-clean（identityReconcileMu 块对齐），非本卡引入。
  - **S2 结构性残留声明**：五字段已剥，但 quick_controls 的「选择哪些参数」与 recommended_groups 的组名仍是内核语义结构（组名源自同一人工词表）——卡面字段清单为五字段故按最小忠实实现，如 A 臂需结构级净化请主管裁定扩域。
  - **端测边界声明（零栈）**：本卡全部验证为 Go 单测/包级回归+编译（AGENTS §7 等级：单元契约级）；未跑真栈烟测、未启栈、零 LLM 调用零计费、无 run ID；缝开态的端侧有效性（含 dryround）留待真栈窗口。渲染面/用户旅程不涉及。

