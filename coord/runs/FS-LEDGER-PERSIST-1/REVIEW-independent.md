# FS-LEDGER-PERSIST-1 独立复核报告（L2 只读复核，2026-10-10）

- 复核对象：实现 commit `db6f2b11`@`port/fs-ledger-persist-1`（worktree `D:\Vit_DAW_wt_fslp1`，已推 origin）
- 复核腿：general-purpose 独立 agent（与实现不同会话）；复核过程零文件修改、未启动真栈；测试仅在 port worktree 内运行（`t.TempDir()` 隔离）
- 复核材料：卡面+回执（doing/ 版）、rulings §5/§6.1、实现全量 diff、三轮 run 工件（201713/202326 取证插桩 + 203348 修复回归）与 RUN_NOTE/agent_last.log 边界日志、源码锚点（harness/chat/agentloop/contextruntime）、AGENTS §5/§7-§10、PROTOCOL §2/§2.2

## A. 根因归因 vs 证据

**双层根因均被工件+日志+源码三面支撑，无更简单的单点解释被遗漏。**

- **层 1（chat 严格断言丢弃结构体 bundle）——直接根因，实锤**。
  - 源码：`agent/internal/harness/ccb_observation.go:110-113` 确认 `"bundle": bundle` 把 `capabilitycontext.FreeStateObservationBundle` **结构体**直接嵌入 `map[string]any` 返回；拒绝分支（:88/:93/:99/:108）同嵌 `RejectedFreeStateObservationScoped(...)`——复核核实其签名返回类型也是 `FreeStateObservationBundle` 结构体（`capabilitycontext/free_state_observation.go:409`），回执的域外发现描述准确。
  - 日志：202326 插桩行 `[free-state.ledger] record[1] name="ccb.observation_request" result_type=map[string]interface {} result_keys=2 bundle_keys=0 audit_keys=0`——result 是 map 且有 2 键，但 `result["bundle"]` 在严格 map[string]any 断言下为空，结构体嵌入形态钉死；201713 边界行 `executed=3 ccb_observations=1` 证明三条 executed 行只活出一笔（RecentObservation 兜底）。
  - 因果闭合：修复轮 203348 同一边界行变为 `executed=3 ccb_observations=2 ids=[obs_..._e391fa3e743f obs_..._c344c8929bfa]`。
  - 进程内直传解释自洽：`recordFreeStateDecision(conversationID string, res agentloop.Result)` 是 Go 类型直传（不经 JSON），结构体形态保留到 chat 消费面；agentloop 侧因 `messageLoopMapValue` 有归一化而未翻车——两面形态差异与缺陷只在 chat 侧爆发完全一致。
- **层 2（Result 快照不承载 loop）——结构性死通道，独立成立**。
  - 日志：201713/202326 终局轮 `snapshot_loop=false continuation_loop=false`（终局 cont=nil，两个载体皆空）。
  - 源码：`contextruntime.Build`（context.go:76-150）确认 Context 只经 `summarizeSelection` 选择性摘要进 `CurrentSelection`，无自由键透传；grep 全包 `free_state_reasoning_loop` 在 contextruntime 零命中。chat :814 的快照导入（注释写明为此设计）是死通道成立。
  - 层 2 单独不解释 G3 误杀（层 1 才是直接根因），但它是设计声称通道的结构缺陷，修复为恢复设计意图，非过度修复；203348 `snapshot_loop=true` + 续轮 `stored_receipts=2` 证明通道激活且与服务端 store（`mergeFreeStateLoops`，带 overlayNewer/终态保护/计数单调防回退）合并无损。
- **cycle=0 归因与代码一致**：grep 全文件 `loop.Cycle++` 唯一递增点在 :2383，位于 `freeStateAcousticActionOutcome` 判定之后（尝试了声学动作且不需确认才计数）——"只计已执行处理器动作"归因正确。203348 修复后 cycle 仍 0（提案在 G6/G8 被拒、零动作）语义一致，且被 `TestFreeStateLoopCycleCountsActionsNotModelRounds` 钉住。

## B. 修复正确性

diff 亲读（生产面仅 2 文件 +46/-4 与 +12）：

- **`freeStateNormalizeMapAny` 与 `messageLoopMapValue` 等价**：逐行比对（message_loop.go:5784-5800 vs free_state_reasoning_loop.go:2753-2769），逻辑完全一致（严格断言→nil→marshal→"null" 检查→unmarshal 到 map）；唯一差异 `"null"` 检查用 `string(data)=="null"` vs `bytes.Equal`，语义等价。应用点覆盖 result/bundle/audit_receipt 三处，两个消费者（`recordFreeStateDecision` :817 与 `recordAudioClosureRound`，audio_closure_controller.go:571）都经 `freeStateCCBObservations` 受益——回执"同源消费面一并修好"属实。
- **runner 快照注入副作用面已剪断**：
  - 注入发生在 `buildContextSnapshot` 之后、仅写入 `state.contextSnapshot` map，随 `Result.ContextSnapshot` 与 `Continuation.ContextSnapshot`（cloneMap）出去——预期通道。
  - 模型投影面不受影响：`buildModelContextSnapshot`（helpers.go:60-95）消费 `contextruntime.Snapshot` **struct** + `ProjectModelSnapshot`（其 `ObservationLedger` 由 `messageLoopFreeStateObservationLedger(state)` 从 input.Context 显式提取），全程不读 `state.contextSnapshot` map——map 注入进不了模型上下文。
  - 跨轮继承被白名单剪断：下一轮 `buildContextSnapshot` 以本轮 map 为 `PreviousSnapshot`，`summarizeInheritedSnapshot`（context.go:1531-1539）只拷 6 个身份键（schema_version/created_at/conversation_id/goal_id/run_id/goal_summary），`free_state_reasoning_loop` 键被丢弃——不泄漏进摘要投影。
  - 暂停态 `Continuation.Context` 本就 `cloneMap(state.input.Context)` 承载 loop，注入与之冗余无害；注入条件 `len(loop)>0` 使非自由态运行零影响。
  - chat 消费侧 `freeStateLoopFromAny` 自带 JSON 往返+schema 校验，注入的 map 可解析（203348 续轮 `stored_receipts=2` 实证）。
- **frozen 契约零触及**：diff 只碰 `chat/free_state_reasoning_loop.go`、`agentloop/runner.go`、两测试文件；`free_state_gate.go`（G3 门法条）、`harness/ccb_observation.go`、`capabilitycontext`、`contextruntime` 均未动。纯消费侧解析/持久化面，符合卡面停止条件的约定域。

## C. 测试验收（本复核腿在 port worktree 实跑）

| 命令 | 结果 |
|---|---|
| `go test ./internal/chat -run 'TestFreeStateLedger\|TestFreeStateLoopCycle' -count=1 -v` | **6 PASS / 0 FAIL**（3 新增 + 既有族 3） |
| `go test ./internal/agentloop -run TestForensicPullSession -count=1 -v` | **1 PASS** |
| `go test ./internal/agentloop ./internal/contextruntime -count=1` | 两包 ok（5.3s / 0.47s） |
| `go test ./internal/chat -count=1`（可选全包） | ok 90.97s，exit 0 |

**最小反例真实性成立**：chat 侧 fixture 用带 json tag 的 Go 结构体（`forensicCCBBundleStruct`）嵌入 `result["bundle"]`——精确复现进程内直传的根因形态（非 JSON 后的 map 形态）；断言双收据存活 + mix scan 在账 + G3 行谓词事实面。agentloop 侧 `TestForensicPullSessionLedgerSurvivesAcrossRounds` 用真 pull 会话驱动三轮 CCB + 终局，钉在飞台账逐轮累积与 `Result.ContextSnapshot` 持久化双收据——两测试分别钉层 1 与层 2，分工正确。

## D. SMOKE 回归 vs 卡面验收标准

- **A2 站图应过：满足**。203348 run_report `A2_free_state_station_map=true`（station rounds=1/observations=1/proposals=1）；对照缺陷轮 201713/202326 `A2=false`（rounds=0 observations=0 proposals=0）——本复核用 python 独立重读三份 run_report 断言布尔核实。
- **A3 如实分记：满足**。203348 `A3=false`，failures 单条 `not_driven_no_card`；从 nl_chat_1.json 独立提取门表：修复轮 G1-G5/G7=pass、**G6/G8=fail**（模型证据完备性），缺陷轮 **G3-G8 全 fail**——G3 误杀修复、A3 失败归因于模型行为而非台账残留，与回执叙述一致。
- **终态对照可独立验证（全部核实）**：`workflow_data.free_state_reasoning_loop` receipt_count=2（`ccbr_obs_..._e391fa3e743f` 含 mix.multitrack_relationship + `ccbr_obs_..._c344c8929bfa` track 级）、loop.status=awaiting_experiment、closure phase=fs4_diagnostic_round + 2 observations、末轮 nl_chat_7 stop=`d1_settlement_pending_mix_tick_refused`/goal=waiting_continue（loop 存活走诚实 settle 链 7 轮 6 nudge）；run_report `fs_loop_admission_status=needs_experiment`（缺陷轮=capability_blocked）。边界日志：首轮 `ccb_observations=2 snapshot_loop=true`、续轮 `stored_receipts=2`。
- **exit 0 字面未达成（卡面内部张力，需主管裁定）**：脚本契约 A1-A4 全真才 exit 0，203348 verdict FAIL(A3) → 真退出码 1，console 留存 `_fix_regression_console.log`（存在已核）。卡面验收标准同句写"A3 视模型行为如实分记"与"exit 0"——若 A3 属概率面则二者逻辑上不可同时保证，属**卡面缺陷**而非执行偏差；执行侧如实分记+留痕是诚实处理。A1/A4=true 一并核实。
- 二进制新鲜度论证成立：run_report repo_head=8245ba5f + RUN_NOTE 声明未提交修复 diff、no SkipBuild 从工作树构建，且 `snapshot_loop=true` 的日志格式只有修复代码才会产生（取证轮为 false）。

## E. 纪律核对

- **真栈占用闭环：实质成立，时间字面有笔误**。占用登记 `0d3d90cc`（提交于 20:16:48，先于首取证 run 20:17）；释放登记随本批落盘（状态→空闲、20:50 复核端口 7878/5555/5556=0/进程=0、SMOKE-1 前条目归档；登记时间笔误"21:40 起"已随本批订正为 20:16 起）。
- **E:\ 源只读**：三轮 before/mid/after `source_manifest_*.json` SHA256 逐字节一致（61 文件 × 3 轮 × 3 相）。✓
- **工件不覆盖**：三个新 run 目录（201713/202326/203348）独立存在，与既有 183305–185519 并列。✓
- **实现与卡面状态提交分离**：port 分支（实现，已推 origin）与协调 worktree `coord/fs-ledger-persist-1`（卡面+释放登记+本报告同批提交）分离正确。
- **commit 无污染**：db6f2b11 = 代码 4 文件 + 三个 run 工件目录（coord/runs/ 下，与 SMOKE-1 既有 run 入库先例一致）；运行时工程状态均作为 run 证据工件归档，非源码树污染；port worktree `git status` 干净。

## F. 总体结论

**支持以"执行侧自验完成"形态交验收。** 双层根因取证—修复—回归的因果链在源码、边界日志、run 工件三面独立可复核且相互闭合；测试面全绿（含本腿实跑 chat 全包）；frozen 契约零触及；纪律面闭环（时间笔误已随批订正）。

**必办项（交验收时）：**
1. 卡面验收标准"exit 0"与"A3 视模型行为如实分记"的内部冲突需主管明示裁定（豁免 A3 字面或按概率面另定多 run 标准）——本复核认为 A3 FAIL 属模型证据完备性行为（G6/G8），不构成台账缺陷残留。
2. ~~协调批落盘与时间笔误订正~~（已随本报告同批完成）。

**建议项：**
1. 采纳回执的域外发现立卫生卡：harness 结构体嵌 map 契约违例（含 `RejectedFreeStateObservationScoped` 拒绝分支），源侧 marshal 后嵌入可消除其他严格断言消费者的同险。
2. 顺带核对的 blocked-surface 机制定位（`capabilityBlockedBoundaryResponse` 位于 `!audioClosureActive` 分支内、闭包激活时永不触发）已如实记录，归 FS-CAPABILITY-BLOCKED-SURFACE-1 扩闭包分支验证。
