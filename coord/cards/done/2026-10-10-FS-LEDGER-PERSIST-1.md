# FS-LEDGER-PERSIST-1：自由态台账跨轮持久化/cycle 计数缺陷取证+修复（汇合点 blocker）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §5](../../rulings/2026-10-10-EVENING-BATCH-rulings.md)+[REVIEW-independent.md §F](../../runs/FS-LARGEPROJECT-SMOKE-1/REVIEW-independent.md)）
- 派发确认：已确认（主管裁定 P1 blocker）
- 验收负责人：GLM 主管决策流
- 池序 49；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：**P1** / 1 天 / 无（main ≥合并态）；L2 级
- 模型分级：L2 / **GLM 执行会话+独立复核腿**（自由态核心面，A 链范式）

## 缺陷（SMOKE run 20261010_185519 实证，复核修正版归因）

61 轨 sattelites 工程自由态运行中：模型已请求 mix.multitrack_relationship（full_project）且**已部分交付**（28.5KB MOM 事实 status=partial，obs_20261010T105649_06b2e7251024）；但终态 `observation_ledger.receipts` 只剩 track 级一张（receipt_count=1）——**mix 回执未存活到准入评估**，致准入门 G3 误杀 capability_blocked（法条 freeStateReceiptUsable 接受 partial，收据若在 G3 本应 pass）。同时 `fs_loop cycle=0` 与同 run ≥4 个 pullharness 模型轮矛盾——循环计数未走。

## 目标（取证→修复→SMOKE 回归）

1. **取证定位**：pull harness 跨模型轮的 loop/continuation 状态承载——观察台账（每张观察应逐张并入，chat/free_state_reasoning_loop.go:817-840 合并路径）为何丢收据；cycle 计数（激活时序）为何不动。锚点起点：free_state_reasoning_loop.go:817-840、free_state_gate.go:185-215、SMOKE 工件 nl_chat_1.json/telemetry_pull.jsonl（run 20261010_185519）。
2. **修复**：按取证根因修（预期域=loop 状态持久化/恢复或 pull harness 跨轮状态传递面）；修复面如超出上述锚点域 → 锚点+形态上交裁定。
3. **单测复现**：跨模型轮丢收据的最小反例（台账存活断言）+cycle 计数推进断言。
4. **顺带核对**（不改不修，如实记录）：blocked 面未接 surface（stop_reason=done 与 fs_loop capability_blocked 并存，FS-CAPABILITY-BLOCKED-SURFACE-1 边界在 fs2_capacity_assessed 未接合）——修复后形态上交主管定卡。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go` + `agent/internal/agentloop/`（pull 会话状态面）+ 测试；以取证结果为准，越域上交。

## 验收标准

`go build ./...`+全量 `go test ./... -count=1` 0 FAIL+gofmt 净；最小反例测试绿；**SMOKE 回归**：复跑 scripts/fs_largeproject_smoke.ps1 单轮（A2 站图应过；A3 视模型行为如实分记）exit 0 与缺陷修复的对照入回执。真栈独占（PC-RUNTIME-STACK 登记）。

## 停止条件

取证推翻缺陷归因（如收据实为另一机制丢弃）→ 新归因+证据上交；修复面触及 frozen 契约（G3 门法条/观察协议）→ 方案上交裁定。

## 并行与资源

真栈独占（SMOKE 回归腿）；单测域与 PULL-PROBE-TIER-EXT-1/FS-RECEIPT-REVISION-1 文件域部分相交（free_state_reasoning_loop.go）——**与 FS-RECEIPT-REVISION-1 串行**（先本卡）。

- 领取：2026-10-10T21:05+08:00 / origin/main=f763d37cbbc64d5dbabbb420a04693b1aa918e0b / owner=GLM-5.3（PC，ZCode 执行会话，L2） / 分支=port/fs-ledger-persist-1（worktree=D:\Vit_DAW_wt_fslp1） / 领取提交=d63000aa
- 回执：**实现 commit=db6f2b11@port/fs-ledger-persist-1**（已推 origin）。

  **取证根因（双层，真栈插桩实证）**：
  ① **chat 侧严格断言丢弃结构体 bundle**：harness `ccbObservationRequest`（agent/internal/harness/ccb_observation.go:110-113）把 `capabilitycontext.FreeStateObservationBundle` **结构体**直接嵌入 map[string]any 工具结果；chat 的 `freeStateCCBObservations` 用 `firstMapFromAny`（仅严格 map[string]any 断言）读 `result["bundle"]`→nil→**executed 行全被跳过**，唯一存活观察来自 `res.RecentObservation` 兜底（agentloop 侧 `messageLoopMapValue` 有 JSON 往返归一化）。多观察轮次中早先的收据（mix scan）到不了服务端台账→G3 误杀。json.Marshal 把结构体渲染成响应面正常对象——工件面看似完好的原因。实证：取证 run 201713/202326 边界日志 `executed=3 ccb_observations=1` + 逐笔 `result_type=map[string]interface{} result_keys=2 bundle_keys=0`。
  ② **Result 快照不承载 loop**：`contextruntime.Build` 是摘要投影（不透传 raw context 键），`res.ContextSnapshot["free_state_reasoning_loop"]` 恒空——chat :814 快照导入（注释写明为此设计）是**结构性死通道**；终局 Result cont=nil，无任何 loop 载体。
  ③ cycle=0 **归因修正（非缺陷）**：`loop.Cycle` 只计已执行处理器动作（交互确认路径 :2383），run 零动作（提案被 ① 误杀）→0 语义一致；①修复后 cycle 语义不变。

  **修复（卡内域，越域发现见下）**：(a) chat 补 `freeStateNormalizeMapAny`（JSON 往返兜底，镜像 agentloop messageLoopMapValue）于 `freeStateCCBObservations`——恢复 Executed 通道全部观察（同时修好 `recordAudioClosureRound` 同源消费面）；(b) `runner.go r.result` 快照注入在飞 loop map——激活设计的边界导入路径（暂停态本就经 Continuation.Context 承载）；(c) 边界观测日志 `[free-state.ledger]`（计数+机器 ID，内容无关）。触碰面：free_state_reasoning_loop.go（+46/-4）、runner.go（+12）、两测试文件。

  **最小反例**：chat `TestFreeStateLedgerKeepsStructValuedCCBReceiptsAcrossRounds`（结构体形态 fixture→双收据存活+mix scan 在账）+ `TestFreeStateLedgerScanReceiptCarriesG3Facts`（G3 行谓词事实面）+ `TestFreeStateLoopCycleCountsActionsNotModelRounds`（cycle 语义钉）；agentloop `TestForensicPullSessionLedgerSurvivesAcrossRounds`（真 pull 会话驱动：在飞台账逐轮累积+Result 快照持久化双收据）。既有 ledger 测试族（TestFreeStateLedger*）全绿零回归。

  **SMOKE 回归**：run `fs_largeproject_smoke_20261010_203348`（修复构建非 SkipBuild；真栈三件套；E:\ mid-check 干净；teardown 核清）：**A2 站图 PASS**（rounds=1 observations=1 proposals/mix_ticks=1；缺陷轮 A2 fail rounds=0 observations=0）+ A1/A4 PASS；A3 FAIL not_driven_no_card=模型行为如实分记（提案在 G6/G8 证据完备性门被拒——G3 已过——loop 存活走诚实 settle 链 7 轮/6 nudge 未挂卡）；verdict FAIL(A3)、脚本真退出码=1（console 留存 `_fix_regression_console.log`）。**缺陷修复终态对照**（203348 vs 缺陷轮 185519/201713）：receipt_count 2 vs 1；G3 pass vs G3-G8 fail；loop awaiting_experiment 存活 vs capability_blocked；closure fs4+2 obs vs fs2+1 obs；边界日志 `ccb_observations=2 snapshot_loop=true`、续轮 `stored_receipts=2`（跨轮存活实证）。取证轮 201713/202326 工件+RUN_NOTE 同批入库。

  **验收命令**：`go build ./...` exit 0；`go test ./... -count=1` 92 包 0 FAIL；gofmt 净（触碰 4 文件）。

  **端测边界声明**：修复验证面=真实三件套栈上 agent HTTP 烟测（fs_largeproject_smoke.ps1 全流程）；渲染面/用户旅程不在本卡范围（无 webui 改动）。A3 的卡挂载链未在 nudge 预算内触达属模型证据完备性行为，非台账缺陷残留；如需 A3 pass 需模型行为配合（概率面，多 run 可期）或证据引导调优（建议主管裁量另立卡）。

  **顺带核对（只记录不改）**：blocked 面未接 surface 机制精确定位——goalrunner_chat.go 的 `capabilityBlockedBoundaryResponse`（FS-CAPABILITY-BLOCKED-SURFACE-1）检查位于 `!audioClosureActive` 分支内；闭包激活（自由态标准配置）时 `audioClosureResponse` 先返回，边界面永不触发（run 203348 期间修复后仍复现 stop=done+completed 形态的机会面已改变——待该卡扩闭包分支验证）。既有测试只钉非闭包路径（free_state_capability_blocked_surface_test.go）。

  **域外发现上交（建议另立卫生卡）**：harness 结构体嵌 map 契约违例（ccb_observation.go:110-113 及批量/拒绝分支同嵌 `FreeStateObservationBundle`/`RejectedFreeStateObservationScoped` 结构体）——本卡在消费侧归一化修复；源侧归一化（marshal 后嵌入）可消除其他严格断言消费者的同险（本缺陷链外未观察到新受害面，agentloop 侧已归一化）。

  **独立复核腿**：[REVIEW-independent.md](../../runs/FS-LEDGER-PERSIST-1/REVIEW-independent.md)（六节全读+测试实跑复现）——**支持交验收**；必办上交项①卡面验收标准"exit 0"与"A3 视模型行为如实分记"的内部冲突需主管明示裁定（复核判 A3 FAIL 属模型证据完备性行为 G6/G8，非台账缺陷残留）；②栈登记时间笔误已随批订正（21:40→20:16）。
- 验收：**pass（2026-10-10 夜主管）**——[rulings/2026-10-10-NIGHT-BATCH-rulings.md](../../rulings/2026-10-10-NIGHT-BATCH-rulings.md) §1/§4；cherry-pick db6f2b11→main fc5ff01d。双层根因采信（插桩实证链完整）；A2 fail→PASS 翻转+终态对照+跨轮存活=缺陷修复验证成立；exit-0 张力裁定=发卡措辞缺陷认账，本卡判据=A2 翻转+终态对照（不要求 exit 0）；**blocker 缺陷面解除**；三线启动建议放行（A3 概率面并行追），最终决定权留用户。后续卡：HARNESS-STRUCT-NORMALIZE-1（域外发现）+FS-BLOCKED-SURFACE-CLOSURE-1（顺带核对）。
