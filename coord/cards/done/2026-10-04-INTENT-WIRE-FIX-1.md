# INTENT-WIRE-FIX-1（P1）：范围切分合成提到 LLM 之前——两条生产路径确定性接管（根治确认卡随机序）

- 池序 1（P1 缺陷修复）；目标仓库=D:\Vit_DAW（agent）；来源=[rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md](../../rulings/2026-10-04-SMOKE-SCEN-RANGE-1-pass.md) 场景 B 证据链；**产品缺陷**：真实栈上「把这段拆出来」确认卡约半数呈内核会拒绝的错误切序（先起点）——模型包络随机序，意图层只是兜底
- 优先级 / 预估 / 依赖：**P1** / 0.5-1 天 / REGION-INTENT-WIRE-1 已合 main（clipRangeSplitCommands 复用）
- 模型分级：**GLM 首选**（生产链路时序两路径+回退语义；flash 可接需卡面锚点全实锚）
- 缺陷锚点（SMOKE 卡实锤）：
  1. legacy 路径：意图层坐 `chat/server.go:2630` `if len(env.Commands)==0` 兜底位——模型自吐命令即不接管（run 210955/211446：抢先 3/3，顺序随机含内核错误序）。
  2. agentloop 生产路径（Godot 主对话默认）：**完全不经过意图层**——模型逐刀调 split_clip，顺序自决。
- 目标：
  1. **腿 1（legacy/server）**：`SynthesizeLocalDAWCommands` 的范围切分形态（clipRangeSplitCommands 命中）提前到 LLM 调用**之前**确定性接管（话术+ranges 匹配即合成提案、跳过模型调用；其他意图形态维持现兜底位不变——最小改动面，实锚 server 组装段后申报确切挂点）。
  2. **腿 2（agentloop/生产）**：照 strip_silence 快速意图先例（`agentloop/message_loop.go:1253-1281`）加 clip-range split 快速意图——话术（这段/这个范围/框选+拆出拆开拆分）+`selected_clip_ranges` 在场→复用 clipRangeCutCommands 合成两条 split 提案短路模型循环；无 ranges/口述时间/播放头指涉时零变化。
  3. 回归：①`go test ./internal/chat ./internal/conversation -count=1` 全绿+新用例（两路径接管/不接管边界）；②SMOKE-SCEN-RANGE-1 场景 B **严格判据**（tool-form 确定性、先终点后起点）复跑 exit 0——即 INTENT-WIRE-1 转正判据；③`-Scenario all` 全绿。
  4. 回执：两腿实锚挂点+新用例清单+场景 B 复跑 run 工件。
- 文件域：agent/internal/chat/server.go（组装段）+agent/internal/agentloop/message_loop.go（快速意图段）+测试；不碰 ccb prompt/webui/Godot。
- 约束：其他意图形态路由零变化（回归面）；E2E 真栈泊位排他。
- 验收标准：两腿落地+全量回归绿+场景 B 严格判据 exit 0（决策侧复跑）。
- 停止条件：server 组装段/agentloop 快速意图挂点与先例形态不符（无法最小接入）→ 实锚上交定方案。
- 领取：2026-10-04（GLM-5.3 执行侧会话）/ origin/main 8191dfd9 / 主树 main 分支（本机单执行流；工作树无其他实现 diff，仅运行时 XML 状态）
- 回执：（commit hash=本提交，实现+卡状态同批 / 两腿锚点 / 新用例 / 场景 B run 工件）
  - **改动**（三文件域 + 三测试文件，未碰 ccb prompt/webui/Godot）：
    - `agent/internal/conversation/intent.go:191` 导出 `SynthesizeClipRangeSplitCommands`——范围切分门（话术+ranges→`clipRangeCutCommands` 先终点后起点切分方案），内部**精确镜像** `SynthesizeLocalDAWCommands` 分派前位（add-track/mute/solo/midi 导入/音频附件导入/音频导入/clip 门/delete 分支任一命中即让路返回 nil），保证接管面=现状分派面；
    - **腿 1**：`agent/internal/chat/intent.go:13` 新包装 `synthesizeClipRangeSplitCommandsPreModel`；`agent/internal/chat/server.go:2606` 在 agentloop 分支之后、`buildAssembly`/`CompleteRequest` 之前加确定性接管分支——命中即走 `chatResponseForCommands`（policy 门禁+PendingPlan 确认卡同管线），**模型调用整体跳过**；其余意图形态仍走 :2630 原兜底位（零变化）；
    - **腿 2**：`agent/internal/agentloop/message_loop.go:1175` 新 `preflightClipRangeSplit`（链位 `:425`，紧跟 strip_silence 先例）：`:1239` 门函数复用 conversation 门合成两条 `clip.split` ToolCall（cmd=split_clip，先终点后起点）；第一刀照常 `needs_confirmation` 暂停，**余刀补挂 `result.Continuation.PendingToolQueue` 跨确认边界**（`ResumeAfterConfirmation` 恢复队列后由既有顶部队列机制续跑），`:1302` trace 扫描（attempted/succeeded 去重）防恢复轮重复触发、计划全落地即确定性完成回复——全程零模型介入；免确认权限（authority bypass）时两刀连执行。
    - 腿 2 UX 注记（如实上交）：agentloop 确认粒度=单调用（`agentLoopPendingDecisions` 单决策卡，chat/goalrunner_chat.go:2285），故生产路径用户看到**两次顺序确认卡**（先终点刀、后起点刀，顺序确定性由本修复保证），非 legacy 的一卡两提案；一卡两提案需动 chat 层 continuation 面超出本卡文件域，未做。
  - **新用例清单**（10 个）：
    - `conversation/intent_premodel_test.go`：`TestSynthesizeClipRangeSplitCommandsMatchesDispatch`（一致性不变量：门函数命中 ⟺ 现状分派返回同一方案，16 边界含复合文本发现——「新建一条轨道然后把这段拆出来」经 expand 别名现状本就落切分分支，镜像如实锁定）、`...Hit`（两刀序）、`...BoundariesStayNil`（无 ranges/口述秒/播放头/无拆词/无范围话术/delete 前位/add-track 前位（框选话术不触发别名）/附件导入前位/空文本）；
    - `chat/intent_premodel_test.go`：`TestSynthesizeClipRangeSplitCommandsPreModelMatchesFallback`（包装=分派对齐）、`TestHandleChatBoxedRangeSplitTakesOverBeforeLLM`（**死 URL LLM 端到端证明跳过模型**：needs_confirmation+恰两决策 3.5→2.0+零 error）、`TestHandleChatBoxedRangeSplitWithoutRangesKeepsModelTurn`（无 ranges 反例：仍走模型回合=死 URL 必然失败=零变化证明）；
    - `agentloop/message_loop_clip_range_split_test.go`：`...CallsGate`（命中两刀序+9 边界不触发）、`...ProposesEndCutFirst`（首回合：暂停+余刀搭载队列+零 LLM）、`...ResumesRunRemainingCutThenComplete`（双确认恢复链全过：end 提案→end 确认执行→start 提案→start 确认执行→确定性完成回复，全程零 LLM）、`...WithoutRangesKeepsModelPath`（零变化）。
  - **回归**：`go test ./internal/chat ./internal/conversation ./internal/agentloop -count=1` 全绿（chat 全量 116s；agentloop 全量绿）；`go build ./...` 全仓绿；go vet 三包净。
  - **场景 B 复跑 run 工件**（真栈泊位，本卡二进制含两腿改动）：
    - `-Scenario range_split`：run `coord/runs/SMOKE-SCEN-RANGE-1/20261004_223710`，**exit 0**——正组**首轮即中** `proposal_source=tool_form_clip_split`、两提案 3.5→2.0（clip smoke_clip_1）、needs_confirmation=true（工件 range_split_positive_attempt_1.json 核对：恰两决策无 error；**无 attempt_2/3 文件=模型随机重试面已消失**）；反例组零提案（existing behaviour preserved）。
    - `-Scenario all`：run `coord/runs/SMOKE-SCEN-RANGE-1/20261004_223757`，**exit 0**——note_time 组（v3.1 时间维度注页+遥测 delta=230）回归不受影响，range_split 组同上复现。
    - 泊位纪律：两 run 各自独立工件目录、起拆即净（agent/kernel 双停日志）、预检拒绝复用已监听栈、VitApp/Workspace 内核默认工程零写入。
  - **端测覆盖边界声明**：真栈场景 B 只覆盖 legacy 路径（`disable_agent_loop=true`，SMOKE 卡既有判据面）；agentloop 生产路径的两刀确认续行链由单元链测试（假 executor 三层 resume）覆盖，**Godot 主对话真栈端到端（两卡顺序确认 UX）不在本卡烟测面内**，由决策侧裁定是否需要补测；`-Scenario all` 对 INTENT-WIRE-1 转正判据（场景 B 严格 tool-form 先终点后起点 exit 0）已达成，转正与归档由决策侧裁定。
  - 领取时工作树已有运行时改动（VitApp/Workspace/Settings/Settings.xml、default_project.xml，非本卡产物），未触碰、未入库。
- 验收：**pass**（[rulings/2026-10-06-INTENT-WIRE-FIX-1-pass.md](../../rulings/2026-10-06-INTENT-WIRE-FIX-1-pass.md)，2026-10-06 晚窗决策侧）/ 验收 commit=实现提交 ed45a072；决策侧四层亲核：diff 直读 937 行锚点全对+两轮 run 工件核验+闲时预复跑三包/build exit 0（INTENT-WIRE-FIX-1-VERIFY-1）+我方真栈 -Scenario range_split -StartKernel exit 0（run 20261006_185750）。边界裁定：Godot 两卡顺序确认 UX 并入 M2 时域手测场次，不另立补测卡。
