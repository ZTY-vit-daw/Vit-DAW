# FIX-LW-CONFIG-PERSIST-1 工件与证据（133，P1）

- 卡：`coord/cards/todo/2026-10-18-FIX-LW-CONFIG-PERSIST-1.md`（池序 133）
- owner：DSH PC 执行流 exec-lw-fix；分支 `port/fix-lw-config-persist-1`；worktree `D:/Vit_DAW_wt_lwfix1`（基线 origin/main=43e7004d）
- 实现 commit：`36323529`（server.go + 新增 `agent/internal/chat/loudness_watch_persist_race_test.go`；本目录为证据提交，见本分支后续 commit）
- 触发缺陷：LOUDNESS-B-TRIGGER 修订腿 run2（20261018_121008）gate4 FAIL——D1 post-run 上下文 `loudness_watch_configure {enabled:false}` 返回 ok+enabled=false 后下一请求即还原 enabled=true，持久 state 被复写。

## 1. Root cause（证据先行）

**结论：请求边界 ownership 缺失 + deferred persist 时序，不是简单的 restore 逻辑错误。**

1. `agent/internal/chat/server.go` `handleInvoke` 对 loudness configure/status 走 Server 直连分支：先 `writeJSON` 返回响应（:1943/:1947），再执行 `defer syncCurrentProjectWorkspace`（:1678，activate+persist）。即 **响应先于持久化**。
2. D1 post-run 上下文有继续活跃的 continuation scheduler：`runContinuationSchedulerOnce`（每 250ms tick / wake）在无 request-boundary invocation 时执行 `reloadActiveRuntimeState()` → `restoreProjectAgentRuntimeStateLocked`，把磁盘 durable snapshot **整体重放**进内存；其中 `:7539-7554` 会重建 `s.loudnessWatchConfig`。
3. 于是出现窗口：configure 已把内存配置改为 off，但 deferred persist 尚未落地；scheduler 的磁盘重放把**旧的 enabled=true** 恢复回内存；随后 deferred persist / 下一请求 preamble persist 把回退值写回磁盘。下一 status 读到 enabled=true，且磁盘 state 也是 true——从请求侧看就是"配置被还原"。
4. 为什么普通工程上下文不出现：普通工程 runtime state 很小（KB 级），scheduler 重放毫秒级完成，窗口几乎不可能命中；D1 run2 的 state 约 **20.8MB**（含 audio closure/事件/157 会话等），重放耗时长得多，且 configure/status 实测延迟 8.3–14.8s vs 普通 33–45ms（与 `[continuation.stall] reason=invocations-active` 同族 contention）。
5. 同族先例：authority mode 已有 `authorityModeExplicit` latch（AUTHORITY-LOST-1）专门处理"两个 restore surface（workspace activation + scheduler disk reload）不得覆盖进程内显式选择"，即该 restore-surface 竞态在本仓库是被承认的类别。

最小可重复本地证据：`evidence/pre_fix_failure.txt`（修复前确定性失败，双向）：

```
after scheduler replay: enabled=true, want false (explicit configure was reverted by the durable snapshot)
after deferred sync: enabled=true, want false   (explicit_off_over_stale_on)
after deferred sync: enabled=false, want true   (explicit_on_over_stale_off)
```

## 2. 修复（最小面，仅 server.go）

仿 `authorityModeExplicit` 增加进程内 latch `loudnessWatchConfigExplicit`：

- `invokeLoudnessWatchConfigure` 成功合并/校验后置位（显式选择）；
- `restoreProjectAgentRuntimeStateLocked`：latch 置位时**保留内存显式配置**、不采用快照（其余状态照常重放）；未置位时原 fail-closed 恢复逻辑不变；
- workspace identity 切换时清 latch（配置按 per-session runtime state 归属，不跨工程泄漏）；
- 不持久化：重启后仍以磁盘为准（下次 persist 会把 latch 中的配置写盘）。
- `enabled=true` 的正常持久化/恢复路径不变（回归 `loudness_watch_state_test.go` 全过）。

## 3. 单测（确定性）

命令与结果（本 worktree，commit 36323529）：

```
cd agent
go build ./...                    # exit 0（evidence/go_build.log）
go test ./... -count=1            # 95 包 0 FAIL，exit 0（evidence/go_test_all.log）
go test ./internal/chat -run 'TestLoudnessWatchExplicit' -v
  修复前：FAIL（evidence/pre_fix_failure.txt）
  修复后：PASS（evidence/post_fix_pass.txt）
```

新增回归覆盖卡面两条路径（`agent/internal/chat/loudness_watch_persist_race_test.go`）：

1. `TestLoudnessWatchExplicitOffSurvivesSchedulerDiskReplay`：persisted enabled=true + 显式 off → scheduler durable replay → persist → 二次 replay 均保持 false（restore 覆盖路径）。
2. `TestLoudnessWatchExplicitConfigSurvivesDeferredSyncAfterReplay`：replay 落在 configure mutation 与 handleInvoke deferred sync 之间，`explicit_off_over_stale_on` / `explicit_on_over_stale_off` 双向均按显式值落盘且后续 replay 稳定（defer sync 路径）。

## 4. 真栈复验（本卡自证）

- 环境：真实栈 = 主检出 kernel `D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe` + 本卡构建 agent（拷入 `D:/Vit_DAW_wt_loudness_watch_b/agent/bin/VitAgent.exe`，gitignored）+ Godot UI；**复用 90 修订腿 worktree 脚本（只读，未改脚本，未在该 worktree 做任何 git 提交）**；新 artifact 目录。
- 被测二进制（gates setup 亲核记录）：

| 项 | SHA256 |
|---|---|
| agent `VitAgent.exe`（fix 分支 commit 36323529 构建，37332480 bytes，12:38:32） | `5A25452ADCCE8923AEEE6C361AC9476CEEDBEE1A22A634A4C36823949109247A` |
| kernel `VitApp.exe`（主检出） | `B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B` |

- 命令（`RepoRoot=D:\Vit_DAW_wt_loudness_watch_b`）：

```
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_loudness_watch_b -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -SkipBuild `
  -KernelExe D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json
```

### run 表

| run | stamp | runner exit | script exit | 结果 |
|---|---|---|---|---|
| run1 | 20261018_124040 | 非 0 | 1 | **D1 腿 fail（非 LW 面）**：`acoustic materiality record is missing`；free-state 续跑 7-continuation budget 耗尽（latest_decision stop_reason=`free_state_continuation_budget_exhausted`），round 停 post_action_evaluation，settle tail 的 materiality/target_response 未落地。同一 fixture 当天更早有两 run（90 修订腿 run1/run2 120243/121008）D1 PASS → 模型分支随机，非本卡改动面；脚本在 LW 阶段前退出，gate4 未执行。 |
| run2 | 20261018_124639 | **0** | **0** | **D1 PASS + LW 四门+E1 全 PASS（gate4 FAIL→PASS）**——见下。 |

### run2 每门结果（`evidence/run2_20261018_124639_loudness_watch_gates.json`）

| gate | 结果 | 数据 |
|---|---|---|
| gate1 disclosure | PASS | observe #1 携带 loudness_watch 披露 + verdicts_path |
| gate2 verdict store + round trigger | PASS | schema=`loudness_watch_verdicts.v0`，round trigger=`project_change`，records=2 |
| gate3 dedupe | PASS | checks 7→8，records 2=2，keys unique |
| **gate4 config-off silent** | **PASS** | configure off 后 observe #3；checks_total 保持 **8**（checks_after=8 == checks_after_off=8），不再推进 → 100% 复现修复目标 |
| A4 zero markers | PASS | 6 家族标记零命中 |
| E1 | PASS | `[settle/project_change] n=1 median=18.041ms max=18.041 bakes_total=0 fresh=19 stale=0 fresh_reuse_ratio=1.00`；`[observation_piggyback] n=1 median=57.164ms fresh=18` |

D1 摘要（`evidence/run2_20261018_124639_d1_report_summary.json`）：status=pass，validation.status=pass，action_domain=reverb，target=track:1032，before_revision=3→after_revision=4，transaction_id=`tx_dyn_9a04690e2f38030b`，readback_value=0.382767379283905，post_action_observation_id=`obs_20261018T045006_fbdf2d64a772`，forward_mutation_count=1，human_audition_ready=true，park=fs7 improvement proposal / waiting_continue。

### 拆栈（只杀本腿自起 pid）

- run1：pids Godot:24580 / VitAgent:19152 / VitApp:19704 → 12:46:34 复核 7878/5555/5556/8787/4445/19876 全部 listen=0、四类进程（含 Godot 变体）=0（`evidence/run1_teardown.txt`）。
- run2：pids Godot:24972 / VitAgent:12068 / VitApp:21556 → 12:52:54 复核同上全部 0（`evidence/run2_teardown.txt`）。

## 5. 边界与偏差（如实）

1. run1 的 D1 失败是模型分支随机（与 90 卡 RUN2_RUN3_FORENSICS 同族），不是 gate4 签名；按 §8 未在同断点原样烧第三轮：run2 是该断点第二次尝试即 PASS。真栈的成功样本数为 1 —— 单次 exit 0 只证存在成功路径，不证稳定性；稳定性沿用 90 修订腿既有基线与单测背书。
2. 本 run 的被测 agent 是 fix 分支二进制（非 90 分支 worktree 构建）；脚本/命令/fixture 与 90 修订腿一致。是否作为 90 终验补证证据由决策侧认定（本卡按卡面"本卡自证或 90 补证腿承担，二选一由决策侧认定"登记为**本卡真栈自证**）。
3. 90 worktree 仅作脚本只读复用 + agent 二进制（gitignored）替换；未改脚本、未在该 worktree 提交 git 改动；该 worktree 既有未跟踪目录保持原样。
4. run1 artifact 大件（d1_smoke_report.json 12MB/observe*.json 12MB 等）留盘 `D:/Vit_DAW_wt_loudness_watch_b/artifacts/free_state_d1_s1/20261018_124040/`；run2 同目录 `.../20261018_124639/`。本目录只保存 slim 证据。
5. 生产改动面仅 `agent/internal/chat/server.go`；未动 loudnesswatch trigger 语义、未改 smoke 期望/门禁、未改 90/92 脚本、未 push main。

## 6. 证据清单（本目录）

- `pre_fix_failure.txt` / `post_fix_pass.txt`：新回归修复前失败签名 / 修复后通过。
- `go_build.log` / `go_test_all.log`：全量 build/test 结果（95 包 0 FAIL）。
- `preflight.txt`：启栈前四端口+19876/四类进程=0 + 被测 SHA。
- `run1_20261018_124040_*`：fail summary、loop state（budget exhausted/rounds materiality 缺席）、agent log 摘录、console、binary face、teardown。
- `run2_20261018_124639_*`：gates.json、E1、verdicts、configure(+off)、binary face、runner log、d1/loop summary、agent log 摘录、console、teardown。
- `run2_console.log` / `run1_console.log`：脚本控制台全文（含每门 ok 行）。
