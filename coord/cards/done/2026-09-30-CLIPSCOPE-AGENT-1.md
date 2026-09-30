# CLIPSCOPE-AGENT-1：路由宪法修订——clip_scope 竖线向 agent 开放（RiskConfirm 确认+披露+可逆）

- 池序 6（用户裁定 2026-09-30：[decisions/2026-09-30-clipscope-open-and-automation-slot.md](../../decisions/2026-09-30-clipscope-open-and-automation-slot.md)）；设计关联=docs/VITNOTE_V1_DESIGN.md §7.5/F14。**并行域：docs+agent prompt/测试（本卡）× executionruntime+烟测脚本（VITNOTE-IMPL-4）× webui（FIX-CONFIRM-CARD-1）——不同文件域可并行**
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无代码依赖，领取即开工
- 模型分级：L1 / GLM 或 flash 可接（修订语义已由裁定给定）
- 已核实事实（决策侧 2026-09-30，勿重勘）：
  - 内核竖线机制在：`rack_set_node_clip_scope` IPC 写 `vit_clip_scope`（docs/ROUTING_CONSTITUTION.md §竖线）
  - agent 工具目录在册：agent/internal/tools/catalog.go:1047 `spec("rack_set_node_clip_scope", "rack.set_node_clip_scope", "rack", "Set rack node clip scope.", RiskConfirm, true, true, true, true)`——**命令面已通、风险级别 RiskConfirm**
  - 禁令承载：宪法 :19 附近（rack_add_node 不得按「Clip 作用域」选中数量自动写 `clip:<id>`；绑定单 Clip 仅限用户手动画竖线）；该语义在 agent prompt 组装层的具体承载文件**未实锚**（已知 mix_session_workflow.go / mixboard.go 有 clip_scope 消费命中，领取后第一步实锚）
  - 原禁令理由：防"未手动画竖线却显示竖线"与全轨生效的听感不一致
- 目标：
  1. **宪法修订**（docs/ROUTING_CONSTITUTION.md）：单 Clip 绑定从"仅限用户手动"修订为"agent 可经 RiskConfirm 确认流提案执行"。保留三条纪律不变：①竖线写入必伴随披露（回执可见、竖线重建显示同步）；②可逆（解绑路径明示于宪法与回执）；③rack_add_node 仍不得静默自动绑定——装载与绑定两步分离。
  2. **prompt/纪律层同步**：实锚宪法语义在 agent 侧 prompt 组装的承载点并同步修订（grep clip_scope/竖线/clip scope 于 chat/mixboard 工作流域）；若发现代码级拦截（非纯文本纪律）一并申报，不擅自绕过。
  3. **测试**：RiskConfirm 确认流对该命令的提案→确认→执行→回执链路回归用例（已有用例则复跑并补"agent 提案绑定单 clip 获确认后写入成功+竖线可见"边界用例）。
- 文件域：docs/ROUTING_CONSTITUTION.md + agent 侧 prompt 承载点（实锚后申报具体文件，预期 chat/mixboard 域 ≤2 文件）+ 相关 _test.go。**接口冻结：不动 tools/catalog.go 命令定义、RiskConfirm 级别、内核命令面。**
- 验收标准：相关 go test 包全绿 + 全量 0 FAIL + gofmt + 宪法 diff 语义自洽（决策侧复核）+ 回执附承载点锚点清单（宪法外每一处修订点的 文件:行）。
- 停止条件：取证发现竖线写入存在内核侧一致性约束或听感风险的技术根据（非纯产品纪律）→ 停下实证上交，由决策侧回炉裁定。
- 领取：2026-09-30 / origin/main c0077113 / port/clipscope-agent-1（独立 worktree D:/Vit_DAW_worktrees/clipscope-agent-1）
- 回执：
  - 实现 commit：`86908e2c`（port/clipscope-agent-1，5 文件 241+/1-；宪法+prompt+回归测试一次提交）。领取时工作树 HEAD=8a0eb61e（=领取提交，基于 origin/main c0077113）；主工作树领取前既有改动 `VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`（运行态，本卡未触碰）。独立 worktree：`D:/Vit_DAW_worktrees/clipscope-agent-1`。
  - **实锚结论（卡面目标 2 前提修正申报）**：宪法「绑定单 Clip 仅限用户手动画竖线」在 agent prompt 组装层**无既有文本承载**——全量 grep（clip_scope/竖线/clip scope/手动画竖线/仅限用户/Clip 作用域/clip:<id>/ROUTING_CONSTITUTION/constitution）于 agent 侧零文本命中；该禁令此前仅存在于宪法文档本身。故目标 2 的「同步修订」落地为：在 prompt 组装的 rack 纪律区**新增**修订后的正向纪律（非改写既有文本）。
  - **承载点锚点清单（宪法外每一处修订点）**：
    1. `agent/internal/agentloop/message_loop.go:4319` — `messageLoopSystemPrompt` rack 装载纪律区新增 1 行：单 Clip 绑定=仅限确认流的提案路径（`rack.set_node_clip_scope`+`clip_scope:"clip:<clip_id>"`）；确认预览披露目标节点与 clip；确认后回执须写明绑定与解绑路径（空 `clip_scope` 回 track）；禁止 `rack_add_node`/任何装载路径写 clip scope；未经确认写入+刷新状态可见不得宣称竖线生效。
    2. `agent/internal/tools/catalog_test.go:617` — 新增 `TestRackSetNodeClipScopeStaysRiskConfirmProposalPath`：钉住 spec 元数据（RiskConfirm/RequiresConfirmation/MutatesProject/RefreshAfter 全 true + ModelSummary `risk=confirm`），守接口冻结与确认语义。
    3. `agent/internal/harness/harness_test.go:7188` — 新增 3 用例：提案阶段零内核+预览披露绑定参数（ProposesConfirmationWithDisclosure）；确认后内核收 `rack_set_node_clip_scope` 四参数、回执携带 clip_scope 回显（ConfirmedWritesClipBindingWithVisibleScope，agent 侧「竖线可见」=刷新状态可见绑定）；解绑=确认流发送空 clip_scope 回 track（ConfirmedUnbindRestoresTrackScope，纪律②可逆）。
    4. `agent/internal/agentloop/message_loop_test.go:1157` — fake executor 新增 `rack.set_node_clip_scope` case（未确认=needs_confirmation+披露预览；确认后=ok+参数回显）+ 新增链路用例 `TestMessageLoopRackClipScopeProposalWaitsForConfirmationThenWritesBinding`：提案→waiting_confirmation+PendingToolCall 参数完整→ResumeAfterConfirmation 确认执行（confirmed=true）→回执披露绑定与解绑路径。
  - **未修改锚点申报**：`agent/internal/tools/catalog.go:1047`（命令定义+RiskConfirm 级别，冻结未动）；`agent/internal/harness/harness.go:981`（通用确认闸 `spec.RequiresConfirmation && !authorityAuthorized`，结构性确认流本体，未动、无 clip_scope 特判）；消费命中 `agent/internal/chat/mix_session_workflow.go:2995-3001`、`agent/internal/mixboard/mixboard.go:4987-5026`、`agent/webui/src/App.tsx:3613/5767`（effect_scope/scope 归一化解析，纯消费，未动）。
  - **代码级拦截申报**：无。toolpolicy 无 clip_scope 条目；agentloop 风险清单不含该命令；唯一闸=RiskConfirm 通用确认闸（即确认流本体，非拦截）。停止条件未触发：取证未发现竖线写入存在内核侧一致性约束或听感风险的技术根据（原禁令理由「未手动画竖线却显示竖线」由披露纪律①覆盖，两步分离纪律③保留）。
  - **域偏离申报**：prompt 承载点实锚结果为 `agentloop/message_loop.go`（非卡面预期的 chat/mixboard 域——该两处命中经实锚均为纯消费代码）。修订实现面=宪法 1 文件+prompt 承载 1 文件（≤2 达成）+3 个 _test.go（卡文件域「相关 _test.go」内）。
  - **自验记录**：`go test ./internal/tools/ ./internal/harness/ ./internal/agentloop/ -count=1` 全绿；`go build ./...` OK；全量 `go test ./... -count=1` 0 FAIL（85 包 ok）。gofmt：4 个触碰的 Go 文件内容级 gofmt 干净（CRLF 剥离后 `gofmt -d` 为空）；`message_loop.go`/`harness_test.go` 因仓内既有 CRLF 检出状态被 `gofmt -l` 标记（主工作树同文件在领取前即被标记，非本卡引入）；`catalog_test.go`/`message_loop_test.go`（LF）完全 gofmt-clean。
  - **不稳定测试首次记录（§11）**：`TestWorkspaceSwitchSettlesInFlightChainExplicitly`（internal/chat，非本卡改动域）首次全量运行失败：TempDir RemoveAll 清理 `The directory is not empty`（Windows 清理竞态，非功能断言）。原始输出留底：`testing.go:1464: TempDir RemoveAll cleanup: unlinkat ...TestWorkspaceSwitchSettlesInFlightChainExplicitly311133775\001\.vit_agent\vitproj_b5_switch_b: The directory is not empty.`；隔离复跑 PASS+整包复跑 PASS（88.5s），判定环境类清理抖动，与本次改动无关联；重复出现应开修复卡。
  - **端测边界声明**：本卡为文档+prompt 文本+单元/集成级回归（fake 内核）；未跑真实栈烟测——内核 `vit_clip_scope` 真实 IPC 写入、Godot 前端竖线重建显示同步（纪律①的显示面）、解绑后恢复全轨的听感，均未端测。本卡不涉 webui 渲染改动（App.tsx 未动）；确认卡链路复用既有 RiskConfirm 通用机制（FIX-CONFIRM-CARD-1 端测覆盖模式的同族）。真实栈端到端（提案→确认→写入→前端竖线可见→解绑）留待真栈烟测或用户手测，是否补真栈卡由决策侧裁定。
- 验收：**pass（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md](../../rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md)；合并 commit 367027cf（cherry-pick）；复跑 tools/harness/agentloop 三包 ok+宪法修订语义亲核（两步分离显式化+agent 提案路径三纪律保留+用户裁定日期锚）+gofmt CRLF 声明 main 基线核验属实+承载点锚点清单亲核；真栈端到端（提案→确认→写入→前端竖线可见→解绑）留用户手测/M8 同场；done 簿记提交未推 main 由决策侧代收（本卡面即归位）。
