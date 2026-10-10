# G3-BOUNDARY-PERSIST-1 回执：pull 边界持久化瞬态失败取证（只读，零代码改动）

- 卡：coord/cards/doing/2026-10-09-G3-BOUNDARY-PERSIST-1.md（池序 37；领取基线 origin/main 33036cfb）
- 取证时间：2026-10-10 闲时车道；取证者=闲时任务·GLM-5.3-Flash（主管会话派发）·PC
- 证据源：coord/runs/G3-ATTRIB-1/20261009_214138_harnessab/{agent_harness_ab_pull.log, agent_harness_ab_push.log, harness_ab_summary.json, harness_ab_metrics_pull.json} + agent 源码 @ a28eb1b0（未触碰）
- **分类结论：瞬态竞态（跨进程 agent runtime state 租约竞争），非确定性缺陷。** 修复非本卡义务；附两条观察项供主管裁量（§5）。

## 1. 失败点锚点（代码链，@a28eb1b0 实读）

| 环节 | 锚点 | 内容 |
|---|---|---|
| 发射点 | `agent/internal/chat/goalrunner_chat.go:2037-2042` | `recordGoalResult` 返回 err → `res.Status=Failed`、`res.StopReason="durable_checkpoint_persist_failed"`、`res.Error=err.Error()`、`res.Continuation=nil` |
| 持久化调用 | `goalrunner_chat.go:2990`（`persistErr := s.persistContinuationState()`）→ `:3058`（`return persistErr`） | 错误自 recordGoalResult 尾部返回 |
| 路由 | `agent/internal/chat/continuation_scheduler.go:832-843` | `continuationPersist` 钩子为空（生产）→ `persistCurrentProjectWorkspaceChecked()` |
| 失败面 | `agent/internal/chat/server.go:7123-7174` | 四出口：①锁竞争重试 20×25ms 耗尽（:7146-7162，错误文案 "agent runtime state lock still held after retry: %w"）②非锁类 acquire 错误（:7153/:7166 静默返回）③状态写盘失败（:7169-7171，**有** `[workspace] runtime state save failed` WARN）④`lease.Release()` 失败（:7173 静默返回） |
| 锁实现 | `agent/internal/history/runtime_state_lock.go:50-71` | 工程域文件租约 `agent_runtime_state.lock.json`；他主活租约→`ErrAgentRuntimeStateLocked`（含 owner+expires_at）；租约时长=2min（`continuation_scheduler.go:149` `continuationLeaseDuration=2*time.Minute`） |
| owner 身份 | `server.go:525`（`schedulerOwner: "scheduler_"+randomID()`） | **每进程唯一 owner**——同进程 persist 与 scheduler 共用同一 owner 串，不会自竞争；竞争必来自另一进程 |

## 2. 原始证据（回指工件）

1. **失败本体**：`agent_harness_ab_pull.log:42-43`（2026-10-09T21:43:21 同秒）——
   - `:42 [continuation.arm] conversation=dev_harness_ab_pull_20261009_214250_r1 goal=goal_365f49776bc8ce75 status=completed stop=done … continuation=false durable=`（内部链已完成、无 durable continuation 在押）
   - `:43 [timing] agent_loop_chat … status=failed stop=durable_checkpoint_persist_failed completed_steps=3 executed=3`
2. **同窗锁竞争实证（同一锁族、同一函数）**：`agent_harness_ab_pull.log:19`（21:42:53）——
   `[capability.route] runtime state save deferred task=task_d345a7b8d2ee68fb error=agent runtime state lock still held after retry: agent runtime state locked by another owner: owner=scheduler_75eac58790a1bfc0 expires_at=2026-10-09T13:44:49Z`
   （调用点=`capability_routing.go:678` 同一 `persistCurrentProjectWorkspaceChecked`；错误经 ：679 WARN。活外来租约在场直接证明。）
3. **唯一性**：`durable_checkpoint_persist_failed` 在 pull 日志出现 1 次、push 日志 0 次（grep 计数）；summary/metrics 工件仅 rounds[0] 携带该 stop_reason，仅记分类不记底层 error 文本（res.Error 未入工件——见 §5 观察项 A）。
4. **自愈证据**：21:43:21→22 `vit_checkpoint` async done（revision=6，19ms；工程 checkpoint 面，与 runtime state 面不同机制，仅佐证盘面健康）；**21:43:51 f1 边界同路径 persist 成功**（continuation.arm status=completed，无失败行）——外来租约在此前已释放；f2/f3/confirm 同路径全成功。

## 3. 时间线重建（push→pull 交接窗口）

| 时刻（本地） | 事件 |
|---|---|
| 21:41:51 | push agent 启动（VSP hub 本 run 缺席，后台重试注册） |
| 21:42:49 | push f3 边界完成（waiting_confirmation）；push 日志末行=VSP attempt=30（**进程仍活**）。同秒=外来租约获取时刻（expires 21:44:49 − 2min lease 反推） |
| 21:42:50 | pull agent 启动，激活**同一 fixture 工程**（uuid=vitproj_035c6098…，worktree D:\Vit_DAW_wt_g3a1）——**双进程并存窗口开启** |
| 21:42:53 | pull 侧 capability.route save 撞锁 deferred（证据 2，重试 500ms 耗尽） |
| 21:43:21 | r1 内部链 completed/done → 边界 persist 重试 20×25ms 全落在外来租约持有期内 → 降级 failed（证据 1） |
| 21:43:22 | r1 async vit_checkpoint done（盘面健康佐证） |
| ≤21:43:51 | 外来租约释放（f1 persist 成功）；后续 pull 全部 goal 同路径成功 |

持有者进程身份无法从工件确证（租约文件未随 run 采集；push 日志止于 21:42:49）——最可能=push agent（并存窗口内唯一已知他主进程；租约获取时刻与其 f3 边界同秒）。

## 4. 出口排除与分类依据

- **出口③（写盘失败）排除**：pull 日志全窗无 `[workspace] runtime state save failed`（该出口必有 WARN，server.go:7192-7194）。
- **出口②（文件系统瞬断）排除**：同 StateDir 在失败前（21:42:00 push 侧 checkpoint）与失败后 1s（21:43:22）均写成功；500ms 窗口内孤发且前后 健康，概率远低于已被实证在场的锁竞争。
- **出口①（锁竞争）采信**：证据 2 证明同窗同函数同锁族竞争真实发生且错误文案同族；r1 的 recordGoalResult 路径无专用 WARN（错误只进 res.Error——与日志仅见 timing 行 stop reason 一致）。
- **非确定性缺陷依据**：单次（全 run 1 次）；同路径后续 5 个 pull goal 全成功；降级行为本身=设计内 fail-visible 语义（server.go:7140-7145 注释明示"宁可显性失败不可静默丢权威内存态"）；机制无代码路径异常。
- **非系统性数据丢失依据**：风险窗=失败至下一次成功 persist 之间进程死亡才可能丢 r1 终态（内存态含 r1 结果，`persistActiveProjectWorkspaceLocked` 写**全量** projectAgentRuntimeState 快照——f1 于 30s 后成功即闭合，r1 态随之落盘）；r1 无 durable continuation 在押（durable= 空）；run 终局分类不受影响（主管验收已按异常分记处理）。

## 5. 观察项（非本卡义务，供主管裁量）

- **A（观测面小缺口）**：`recordGoalResult` 的 persist 失败无专用 WARN——对照 `capability_routing.go:679`（deferred WARN）与 `server.go:7192`（写盘 WARN），goal 边界 persist 失败只在 timing 行留 stop reason、底层 error 文本（含竞争 owner id）不入日志与 summary 工件，事后取证只能靠同窗旁证。一行 WARN 即可闭合（归 hygiene 机会面）。
- **B（harness 卫生）**：A/B 序列 push 进程在 pull 启动后仍活（杀停晚于 pull 首个 goal 边界），两进程并存于同一 fixture 工程租约域。顺序场景若确保前一进程完全退出再启动下一进程，此类交接窗口瞬态可根除（归 harness 场景脚本域，非 chat/harness 包缺陷）。

## 6. 单测可复现性结论

- **降级机制可单测复现**（机制层最小反例，本卡零代码不落实现）：预写活租约 JSON（owner≠被测进程 owner、expires_at=未来）至工程 StateDir → 驱动任一 goal 边界 → 断言终态 `durable_checkpoint_persist_failed` 且 `res.Error` 含 "still held after retry: … owner=…"。
- **自然竞态不可确定性复现**（双进程并存+500ms 重试窗恰好落入他主持有期，属时序事件）——如实登记，不以偶发重跑凑复现。
