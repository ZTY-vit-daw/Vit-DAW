# AUTH-RESTORE-LOGSPAM-1 回执

- 分支：`port/auth-restore-logspam-1`（独立 worktree `D:/Vit_DAW_wt_authrestore`）
- 领取基线：origin/main `9e30ac5f3da8a90f51b420b66c7a66f0b8cf2a9c`（领取时工作树干净）
- 本卡 commit：（见下方提交记录，随本回执同分支提交）
- 证据输入：`coord/runs/M1-RETEST-20260930/evidence/authority-logspam-count.txt`（计数 500，对应卡面：2 分钟窗口 agent_last.log 全为 `[authority] restore kept explicit mode=full_project_access disk=full_project_access`）

## 刷屏来源取证（含与卡面"已核实事实"的偏差申报）

卡面假设：前端轮询（webui/Godot authority 面）每次 GET 触发 restore。代码实锚后判定**主源不是前端 GET**：

- GET `/agent/authority`（turn_control.go:152 `handleAuthorityMode`）虽每次调 `activateCurrentProjectWorkspace`，但该函数有同身份+同 session 早退护栏（server.go，`sameIdentity && boundSessionID != "" && s.activeWorkspaceSessionID == boundSessionID` → return），稳定态下不会走到 restore。
- 实际主源：**continuation scheduler 每 tick 重载快照**。`continuationSchedulerTick = 250ms`（continuation_scheduler.go:150），`runContinuationSchedulerOnce`（:1022）在无 in-flight invocation 时无条件调 `reloadActiveRuntimeState`（:2016）→ `restoreProjectAgentRuntimeStateLocked` → 无条件 INFO。节奏核算：500 行 / 120 秒 ≈ 4.17 行/秒，与 250ms tick 吻合。
- 修复点与卡面文件域一致（restore 日志点本身），降噪对任何触发源（scheduler tick、workspace 激活、测试显式调用）同样生效。此偏差属"取证发现刷屏另有来源"的如实申报范围，但触发性质仍为轮询（agent 内部调度轮询而非前端 HTTP 轮询），不推翻卡面目标与方案。

## 改动锚点（≤1 源文件 + 1 测试文件）

- `agent/internal/chat/server.go`
  - Server 结构体新增 `authorityRestoreSeen bool` + `authorityRestoreLast authorityRestoreLogState`（不持久化，s.mu 下访问；新增可比较类型 `authorityRestoreLogState{explicit,mode,disk}`）。
  - `restoreProjectAgentRuntimeStateLocked` 内 authority restore 块（原 ：7297-7307）：INFO 仅在**首个 restore 或 outcome 三元组变化**（explicit 闩分支翻转 / 生效模式变化 / 盘值变化）时发出；完全相同的重复 restore **整行跳过**。adopted 分支 outcome.mode 取 restore 后生效值，避免把上一次 restore 自身造成的模式落位误判为外部变化（首轮测试曾因此误报，已修正）。
  - **闩语义零变化**：`authorityModeExplicit` 全程未触碰；`s.authorityMode = restoredAuthority` 仍仅在非 explicit 分支无条件执行（与日志级别、logger 是否为 nil 均无关）。
- `agent/internal/chat/authority_restore_logspam_test.go`（新增，2 个测试）：adopted 分支 25 次重复 restore 仅首行 + 模式真变再打；explicit 分支复刻证据形态（kept explicit mode=full disk=full 重复）收敛为 1 行、盘值分歧保持可见、闩与模式不被触碰。

### 设计取舍：为何"跳过"而非"降 debug"

logx（`internal/logx/logger.go` `write`）把所有级别（含 DEBUG）无条件写入日志文件，降 debug 只降 stdout 可见性，agent_last.log 仍按 4 行/秒增长，无法达成卡面"稳定态降为 0"的验收口径，故选"跳过"。INFO 消息文本与改前逐字一致，grep 连续性不受影响。

## 验收门

| 门 | 命令 | 结果 |
|---|---|---|
| 编译 | `go build ./...` | exit 0 |
| 新测试 | `go test ./internal/chat/ -run TestAuthorityRestoreLog -count=1` | ok（2/2） |
| 全量 | `go test ./... -count=1` | **exit 0，90 包 ok，FAIL 零行**（chat 包 96.5s 含全部既有 restore 消费测试：legacy_capability_migration / workspace_switch_safety / turn_control / continuation_scheduler 等 28 处调用面全过） |
| gofmt | `gofmt -l internal/chat/` | 工作树 260/261 报警＝系统性 autocrlf CRLF 噪声（未触碰文件同列）；本卡两文件 LF 化后 `gofmt -l` 零输出（blob 级净，沿用 REFSCHEMA-M2 甄别口径） |

## 降噪前后行数对比

- 前（证据窗口）：稳定态 2 分钟 500 行 restore INFO（≈4.17 行/秒，无新信息）。
- 后（代码路径等价推演 + 单测实测）：稳定态重复 restore **0 行**；仅进程内首个 restore 与每次真实变化（闩分支翻转/模式变/盘值变）各 1 行 INFO。单测面：26 次连续 restore 由 26 行收敛为 1 行。
- 覆盖边界申报：本卡为日志卫生单点，验收按卡面＝Go 全量测试；"手验轮询窗口 0 行"留待决策侧在真实栈复验（无需重启内核的常规活栈即可观察 agent_last.log 增长率）。
