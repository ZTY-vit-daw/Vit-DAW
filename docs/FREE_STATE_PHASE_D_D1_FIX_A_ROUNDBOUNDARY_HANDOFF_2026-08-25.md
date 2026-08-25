# Phase D D1 — Fix A/门禁修复完成与 round 边界交接

日期：2026-08-25 晚（会话：ZCode GLM-5.3，L2）
分支：`codex/g1-g7-runtime-remediation`（工作树未提交，按计划烟测通过后再提交）

## 本会话完成的修复（代码在工作树，测试全绿）

### Fix A：capability route 丢失竞态（17:20 not_exercised 的根因）

**根因链**（证据：`artifacts/free_state_d1_s1/20260825_172045/`）：
1. `storeCapabilityRoute` → `persistCurrentProjectWorkspace()` 吞掉持久化错误；
2. `persistCurrentProjectWorkspaceChecked` 在锁竞争重试 500ms 后**静默返回 nil**（server.go）；
3. 调度器每 250ms（`continuationSchedulerTick`）`reloadActiveRuntimeState()` 用磁盘态**无条件覆盖内存**（不尊重 `activeRuntimeInvocations`）；
4. route 只存在于内存→persist 丢失→下一跳 reload 抹掉→continuation 创建即被 `reconcileDurableCapabilityRoutes` fail-close（"durable capacity state has no validated task route"）→ 调度器永不续跑→模型只拿到 1 个 turn。

**修复**：
- `internal/chat/continuation_scheduler.go`：`runContinuationSchedulerOnce` / `recoverContinuationWorkspace` / `reloadActiveRuntimeState` 三处加 `invocationsActive()` 守卫（请求进行中不 reload/不 activate）；新增 `invocationsActive()` helper。
- `internal/chat/server.go`：`persistCurrentProjectWorkspaceChecked` 锁竞争重试耗尽后返回包装错误，不再假成功。
- `internal/chat/capability_routing.go`：`storeCapabilityRoute` 改用 checked persist，失败记 Warn（内存保留，出口 sync 落盘）。

**回归测试**（`internal/chat/continuation_scheduler_test.go`）：
- `TestSchedulerWaitsForActiveInvocationBeforeReloadingRuntimeState`（注意：invocation 守卫必须在 `recordGoalResult` **之前** begin——`wakeContinuationScheduler` 会惰性启动 250ms 调度器 goroutine）；
- `TestPersistCheckedReportsLockContention`。

### Fix C：improvement contract 的 diagnostic_complete 终局门

**根因**（证据：`artifacts/free_state_d1_s1/20260825_193947/`）：模型用掩蔽证据（spv1_p02 注入的 Sub/Presence 频段问题）得出 `evidence_status=sufficient` 的诊断，却以 `diagnostic_complete` 收尾——而 contract 的 completion criteria 只允许 governed experiment outcome / no_candidate_found / capability boundary。运行时照单全收。

**修复**：`internal/agentloop/free_state_reasoning.go` `messageLoopFreeStateOutputIssue` 终局 case 增加：improvement contract + 非 diagnostic_only 时拒绝 `diagnostic_complete`，final_gate 注入纠正消息（diagnostic_only 流不受影响）。

**回归测试**：`internal/agentloop/free_state_phase_test.go` `TestImprovementContractRejectsDiagnosticCompleteTerminal`。

## 验证状态

- 单测：`go test ./internal/chat/ ./internal/agentloop/ ./internal/audioclosure/ -count=1` 全绿（chat 全量多次验证稳定）；AGENTS.md 五条健康检查全过。
- agent 二进制：`agent/bin/VitAgent.exe`（19:47:12 构建，含双修复）。
- 真实栈 D1 烟测（`spv1_p02`）两次：
  - 19:39 跑（仅 Fix A）：`capability_routes=1`，5 个 continuation 全部正常续跑 completed，无 `recovery_validation_required`。**Fix A 验证通过**。not_exercised 原因：模型 diagnostic_complete 收尾。
  - 19:47 跑（Fix A + 门）：门生效。**模型在第 4 个 turn 主动提出了贝斯轨电平微调实验（needs_experiment + track_gain 方向）**——D1 要的模型行为已经出现。但最终仍 not_exercised，见下。

## 下一个阻塞点（已定位，未修）：round 记录不持久化 → FS4→FS5 永不推进

**现象**（证据：`artifacts/free_state_d1_s1/20260825_194718/`，continuation trace + 落盘 state）：

1. 模型 turn 4 提出 needs_experiment → final_gate 拒："the closure host is in phase fs4_diagnostic_round which does not admit decision status needs_experiment"（`AllowsDecisionStatus`：needs_experiment 仅 FS6/FS7 合法）。
2. 模型转而 diagnostic_complete → 被新门拒 → 再试 needs_experiment → 再被 phase 门拒 → 最终以 capability_blocked（"closure made no material progress"，no-progress 策略）收场。
3. **closure 的 6 个 round 全部 completed，但 `diagnostic_rounds` 记录为 0、`priority_queue` 为 null** → `DimensionClosed=false` → `EvaluatePhaseGuard(FS4→FS5)` 永不通过 → phase 永远停在 fs4。

**数据线索**：
- closure observations 只有 2 条：project 级（mix.masking_relationship + mix.multitrack_relationship + project.structure，**无 round 字段**）、bass 级（track.basic_energy + track.timbre_frequency，round=3）。
- `audioClosureRoundRecord`（`internal/chat/audio_closure_controller.go:1273`）要求 `record.Round == state.RoundsStarted` 才收集 views；project 级观察缺 round 归属、gate 弹回的 turn 序列可能让 round-close 块（`recordAudioClosureRound`，同文件 :511 一带，`!current.Terminal() && current.RoundInProgress` 才执行 RecordDiagnosticRound+CompleteRound）与实际轮次边界错位。
- `QueueFromRounds`（`internal/audioclosure/diagnostic_round.go:285`）从 round 记录推导维度闭合；round 记录缺失则队列永不闭合。

**修复方向**（供下一会话/DeepSeek 执行，验收=烟测到 FS7+）：
1. 读 `recordAudioClosureRound` 全函数 + `goalrunner_chat.go:434/445/469` 三个调用点，确认每个 round-close 边界都会走到 RecordDiagnosticRound（尤其观察返回与 final_gate 弹回同 slice 时）。
2. 修 project 级观察的 round 归属（观察记录时应写入当前 RoundsStarted）。
3. 若证据已齐（frontier 非空 + 维度闭合 + 目标证据），在判定决策前先 `AdvancePhase`，避免"证据够了但 phase 不动"。
4. 完成后跑：
   ```powershell
   cd D:\Vit_DAW
   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW -PublicCaseId spv1_p02 -SkipBuild
   ```
   预期：模型 needs_experiment 通过 → 确认 → Apply → readback → post-action observation → A/B → settlement。若 post-Apply 阶段 budget 耗尽（13:04 模式，`reservePostActionObservationSlice` 只 +1），再修 budget 算术。

## 边界与禁止事项（沿用 D1-S1 continuation 文档）

- 不 `git reset`/`git clean`/丢弃工作树；不改用户原始工程；不伪造 proposal/evidence/receipt/judgment；不把 NOT_EXERCISED 改写为 PASS；D1 settlement 完成前不开 D2/D3。
- 本会话曾遇到 `.git/index.lock` 陈旧锁（16:37 遗留），确认无 git 进程后删除即可。

## 提交计划（用户已确认：烟测通过、进度推进后再提交）

建议拆分：① Fix A（chat 三文件+测试）② improvement-contract 门（agentloop+测试）③ 维度映射三阶段修复（其他会话的工作，见 `docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md`）④ 本文档。VitApp 的 388 文件纯换行 diff 需与 .gitattributes 归一化一起单独处理，不混入功能提交。
