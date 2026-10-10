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

- 领取：2026-10-10T21:05+08:00 / origin/main=f763d37cbbc64d5dbabbb420a04693b1aa918e0b / owner=GLM-5.3（PC，ZCode 执行会话，L2） / 分支=port/fs-ledger-persist-1（worktree=D:\Vit_DAW_wt_fslp1） / 领取提交=本提交（coord/fs-ledger-persist-1 fast-forward → main，仅含本卡状态）
- 回执：（取证根因 / 修复 diff / 最小反例 / SMOKE 回归 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
