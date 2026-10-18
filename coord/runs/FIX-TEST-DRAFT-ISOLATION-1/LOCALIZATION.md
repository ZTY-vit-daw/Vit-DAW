# FIX-TEST-DRAFT-ISOLATION-1（140）定位报告：哪些测试写真实 draft 根

- 卡：`coord/cards/doing/2026-10-18-FIX-TEST-DRAFT-ISOLATION-1.md`（池序 140，P2）
- owner：DSH PC 执行流 `exec-start-draft`（task-14）；worktree `D:/Vit_DAW_wt_testiso1`；分支 `port/fix-test-draft-isolation-1`；**实际基线 origin/main=3e60df15**（卡面领取行写 3abe0cfb，仅协调提交差异）
- 依据：118 证据 `coord/runs/FE-START-EMPTY-DRAFT-1/evidence/test_isolation_finding.md`（首跑全量向真实根写 4 draft，已隔离未删）
- 复现方式：在**未设置** `VIT_HISTORY_DRAFT_ROOT` 的环境运行测试，对比真实根 `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts` 目录增量。

## 1. 定位方法（可复跑脚本在 `scripts/`）

1. `per_package_pollution_scan`（`go list ./...` 逐包串行 `go test -count=1 -p 1`，前后真实根目录数差）：**仅两个包**命中——
   - `internal/chat`：+2 目录
   - `internal/harness`：+1 目录
   - 其余 105 包 0 新增、0 失败（`evidence/per_package_pollution_scan.json`）。
2. 逐测试定位：预构建测试二进制 → 逐测试单独进程跑（`-test.run ^TestX$`）并统计真实根增量（`scripts/find_polluting_tests.ps1`）；测试量大使逐测试全扫超时后，改用**时间戳映射**（测试日志 mtime 序列 × 目录创建时刻）定位到具体测试（`evidence/chat_partial_time_map.json` + `per_test_logs_internal_chat/`）。

## 2. 具体污染测试（file:line，chat 包；时间映射命中 9 个）

| # | 测试 | 文件:行 | 触发（写路径无 env） |
|---|---|---|---|
| 1 | `TestActedRoundSettleRefusalKeepsObservationGuidance` | `internal/chat/free_state_owed_round_guidance_test.go:126` | 服务端执行流写 checkpoint/会话节点 → `historyArgs` path 空 → `Open("")` 物化默认 draft |
| 2 | `TestAdoptionSettlesSessionClosureAndReleasesOwner` | `internal/chat/fs_adopt_closure_test.go:28` | 同上（closure/adoption 持久化） |
| 3 | `TestAgentLoopBroadMixConfirmationDoesNotResumePluginLoad` | `internal/chat/server_test.go:2397` | 同上（server invoke/确认流） |
| 4 | `TestAgentLoopClarificationReplyResumesConversationGoal` | `internal/chat/server_test.go:3675` | 同上 |
| 5 | `TestAppliedBoundaryArmsSettleCheckpointOverAnswerablePark` | `internal/chat/traj_auto_settle_test.go:547` | 同上（settle checkpoint 落账） |
| 6 | `TestAppliedBoundaryArmsSettleCheckpointOverCompletedGoal` | `internal/chat/traj_auto_settle_test.go:441` | 同上 |
| 7 | `TestAppliedBoundaryDoesNotArmWhileGoalStillRunning` | `internal/chat/traj_auto_settle_test.go:729` | 同上 |
| 8 | `TestArmDisplacesUndrivableShellOccupantAndSurvivesReload` | `internal/chat/free_state_arm_slot_restore_test.go:87` | 同上 |
| 9 | `TestArmNeverDisplacesDrivableOccupant` | `internal/chat/free_state_arm_slot_restore_test.go:136` | 同上 |
| 10 | `TestLoudnessWatchPiggybackContainsPanics`（harness 包） | `internal/harness/loudness_watch_hook_test.go:101` | harness 内 server/invoke 写路径无 env 隔离 |

**机制（为何包级运行只看到 1–2 个目录）**：`history.defaultDraftProjectPath` 的默认 draft 路径是**进程内缓存**（`draftProject.path`，按 root 缓存）。一个包测试进程里，**首个**无 env 的写路径会把默认 draft 物化为一个真实目录，后续测试复用同一缓存路径 → 表现为"每包 1–2 个目录"，但实际候选写测试是上表多个（顺序依赖）。因此**逐测试清单 + 包级进程级隔离**才能根治该类问题。

## 3. 修复（仅测试/测试辅助，无产品代码改动）

- `agent/internal/chat/server_test.go` 现有 `TestMain`：在未外部提供 `VIT_HISTORY_DRAFT_ROOT` 时创建 per-run 临时目录并设 env，`m.Run()` 后删除。
- `agent/internal/harness/testmain_test.go` 现有 `TestMain`：同上（`vit-harness-test-drafts-*`）。
- 外部 CI 显式传入的 `VIT_HISTORY_DRAFT_ROOT` **优先**（不被覆盖）；测试内既有 `t.Setenv` 隔离继续生效；进程退出删除临时根。
- 其余 105 包经逐包扫描确认无默认 draft 根写入，不改。

## 4. 验收结果（2026-10-18 18:30–18:32，未设置 `VIT_HISTORY_DRAFT_ROOT`）

| 项 | 结果 |
|---|---|
| 修复后包级验证（chat+harness） | `go test ./internal/chat/ ./internal/harness/ -count=1` → **ok**；真实根 **10185→10185（0 新增）**（`evidence/package_verify_after_fix.json`） |
| 全量编译 | `cd agent && go build ./...` → **exit 0** |
| 全量单测 | `go test ./... -count=1` → **exit 0；95 包 ok / 12 包 no test files / 0 FAIL**（`evidence/full_tests_after_fix.txt`） |
| **真实 draft 根前后不变** | **10185→10185；added=0、removed=0**（前后完整清单+时间戳：`evidence/real_draft_root_before_full.txt` / `real_draft_root_after_full.txt`；`evidence/full_suite_isolation_acceptance.json`） |
| gofmt | 改动文件 gofmt -l 无输出（干净） |

## 5. 边界/如实登记

- **其它流的真实根写入（非测试）**：定位扫描窗口（17:56–18:10）与 139（TRACK-LIST-GAP-1）真栈腿重叠，真实根出现 4 个**带 `.vit_agent`+session 的运行时 draft**（18:04:25/26、18:05:47、18:07:58）——属该流栈腿 agent 未设置隔离 draft 根所致，**非测试污染、未删除/未移动**，已上报 Lead 供其流处置。
- 本卡定位过程自身产生的测试污染目录（11 个）已移出真实根到 `C:\Users\timoz\AppData\Local\Temp\fedraft1\task14_test_quarantine\`（未删除，保留证据；`MOVED_LIST_task14.txt`）。118 隔离区 4 个 draft 未删除。
- 逐测试扫描因时间盒截断在 chat 包第 108 个测试（字母序），未覆盖的 chat 测试由**包级 TestMain** 统一隔离（覆盖全量）；harness 由 TestMain 统一隔离。
- 定位复跑脚本：`scripts/per_package*` 见 `evidence/per_package_pollution_scan.json` 生成方式（逐包 `go test -count=1 -p 1` 前后计数）；逐测试 `scripts/find_polluting_tests.ps1`；时间映射 `scripts/map_polluting_tests*.ps1`（对应 `evidence/chat_partial_time_map.json`）。
