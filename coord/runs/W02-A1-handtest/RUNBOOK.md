# W02-A1-HANDTEST-V0 同栈手测 RUNBOOK（n2 预置工程 + 手测包）

- 卡：`coord/cards/doing/2026-11-19-W02-A1-HANDTEST-V0.md`（池序 434；验收负责人=lead）。
- owner：`w02ht`（DeepSeek-V4.1-Flash，PC，独立 worktree）。
- 分支：`codex/w02-a1-handtest-434`；worktree `C:/Users/timoz/.codex/worktrees/pc434-handtest/Vit_DAW`；基线 main `4906fd21`。
- 本轮范围：prep 模式 + 手测包（不改产品代码/设计源/432 卡/其他脚本，不启栈）。
- 状态：**lead 裁决 B（同栈手测）已落地**；`-SameStackPrep` 已实现并过零栈验收（§8）；**真栈轮未运行**（候 lead 登记栈位后与用户在 VitShell 同栈跑）。

## 0. 一句话

同栈手测：用户先在 VitShell 打开一个**已保存**的工程 → lead 用 `-SameStackPrep` 在**同一栈**里把它预置到 n2（导入乐局设计源、开始乐局、推进到 clip 节点、建两个可区分 clip、重命名两轨、`project.save`）→ 用户**不关闭、不重开**工程，在左栏选 prep 会话、选中**第二个 clip**、说「写好了」→ 预期 `clip_scope=selected` 并推进。重置=**重跑 prep**。

## 0.1 背景（为什么必须是同栈）

- 产品裁定 **PROJ-OPEN-RESUME-1**（用户 2026-09-13）：重开工程只回到最后一次手动保存点，**不**把保存点里的对话/运行时状态还给 live 上下文；保存点会话被**归档并清空**。代码：`agent/internal/history/working_session.go:216-245`（`OpenWorkingSessionAtGeneration` → `archiveReopenedAgentState`）、`working_session.go:441`（`writeBlankAgentRuntimeState` 不含 conversations/`edu_jam_sessions`）、`harness/harness.go:1502`（`project.open` 必走该函数）。零栈亲证：`go test ./internal/chat -run TestSameProjectReopenStartsBlankAndArchivesSavedConversation -count=1` → **ok 0.502s**。
- 结论：上一版 `-HandtestPrep`（save→复制→**重开验证**）与「prep 会话在 n2」互斥，已在脚本里标记 `blocked_by_proj_open_resume=true`，**不得用于手测**（保留仅作 clip 持久化取证）。
- **裁决 B（2026-11-19，lead）**：不改 09-13 裁定；改同栈——工程由用户先打开，prep **绝不** `project.new`/`project.open`，只在已开工程里建立会话并停在 n2；手测在同一窗口继续。

## 1. 流程总览

1. 用户（或 lead）在 VitShell 里新建/打开一个已保存工程，建议先另存为 `<HandtestRoot>/live/W02-A1-handtest.vit`（工程必须有磁盘路径；未保存草稿不接受）。
2. lead 在**同一栈**（VitShell 拉起的 agent，`http://127.0.0.1:7878`）跑 `-SameStackPrep -HandtestRoot <dir>`。
3. prep 在**当前打开工程**里：导入设计源 → 开始乐局 → 推进到 n2 → 建两个 clip → 重命名两轨 → `project.save` → 用 **live** `project.state` 验证 → `乐局状态` 报 n2 → 快照 `<HandtestRoot>/pristine`；**不 `project.open`**。
4. 用户不关窗、不重开：左栏选 prep 会话 → 选第二个 clip → 说「写好了」。
5. 记录证据（§4-8）；重置=**重跑 prep**（§5）。

## 2. prep 怎么跑

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:/Users/timoz/.codex/worktrees/pc434-handtest/Vit_DAW/scripts/run_w02_a1c_smoke.ps1 -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 -StackAuthorized -RuntimeOwner "lead/<PC-RUNTIME-STACK 登记 owner>" -TimeoutSeconds 240 -SameStackPrep -HandtestRoot D:/Vit_DAW_handtest/W02-A1-n2 -ProjectPath D:/Vit_DAW_handtest/W02-A1-n2/live/W02-A1-handtest.vit -RunRoot <新目录>/run_<stamp>
```

- `-SameStackPrep` 必须配 `-HandtestRoot`；`-ProjectPath` 可省略（默认取 `/agent/state` 的 `current_project.path`）；给了就必须与当前活动工程一致，否则 `exit 2`。
- 前置：工程**已打开且已保存**（无 unsaved/draft），否则 `exit 2`；prep **不启停栈**，也**绝不** `project.new`/`project.save_as`/`project.open`。
- 与 `-HandtestPrep`、`-QuestionMode work` 互斥（`exit 2`）；`-ProjectPath` 只能配 `-SameStackPrep`。
- 同一 `HandtestRoot` 已有 `pristine` 时拒绝（`exit 2`），每次用新目录（或先删除旧目录）。
- S3–S6 不跑；零 LLM（S1/S1.5/S2 全是确定性命令，`乐局状态` 也是）。
- teardown 不发送「结束乐局」（fixture 必须留在 n2）；recents/`Settings.xml`/`default_project.xml`/authority 仍按 S0 快照还原。

退出码：

| 码 | 含义 |
|---|---|
| 0 | 同栈 prep PASS：save 重写 .vit；live `project.state` 两轨/目标 2 clip/音符一致；prep 会话 n2；未切工程；零 LLM；pristine 快照完成；PREP 记录已写。 |
| 1 | 确定性断言失败（`assert`）：.vit 未被重写、clip/音符不一致、参考轨非空、会话不在 n2、工程被切换等。 |
| 2 | 环境/前置/teardown：工程未打开/未保存、`-ProjectPath` 不匹配、HandtestRoot 不合规、栈不可达、还原失败。 |
| 3 | 未授权（无 `-StackAuthorized`）：只打印摘要，零 HTTP、零写盘。 |
| 5 | 证据/API 缺口（`api`/`evidence`）：项目包/agent 目录缺失、零 LLM 被破坏、fixture 清单写不出等。 |

失败即停：不重跑、不手工改 fixture、不把部分产物当 PASS。

## 3. prep 通过口径

1. `project.save` 前后 .vit sha256 **变化**（真正重写）；活动工程路径/uuid 在 prep 前后不变（**没有** `project.open`）。
2. live `get_project_state`：两个用户轨 `我的旋律（选第二个 clip）` / `参考（空白）`；目标轨 **2 个 clip**、参考轨 0 个；两个 clip 的 `get_midi_clip_notes` 回读与 S2 完全一致（`60@0/1,64@1/1,67@2/1` 与 `67@0/2,71@2/0.5,74@2.5/1.5`）。
3. 同一会话发「乐局状态」：`stop_reason=edu_jam_status`、`level_id=<card_id>`、`current_node=n2-rhythm`。
4. telemetry 新增行中 prep 会话 LLM 行数=0；S3 起的推进不在 prep 内发生。
5. `<HandtestRoot>/pristine` 快照的 .vit sha256 = 保存后的 live .vit；`handtest.json` 记录 conversation id / node / track·clip id。

## 4. 手测步骤（同栈；VitShell 产品入口）

1. **启动产品入口**：`启动VitShell.cmd`（AGENTS §5；不要用 `dev_agent_smoke.ps1 -StartUI` 当手测入口）。
2. **准备已保存工程**：在 VitShell 里新建工程并另存为 `<HandtestRoot>/live/W02-A1-handtest.vit`（或打开一个已保存的 .vit）。工程当前无未保存改动。
3. **lead 跑同栈 prep**（§2 命令），确认输出 `same-stack prep PASSED` 且 `PREP-<stamp>/summary.json` 落盘。
4. **不关闭、不重开工程**（重开会按 PROJ-OPEN-RESUME-1 清空会话 → 只能重跑 prep）。
5. 在左栏（对话流列表）选 **prep 会话**：`summary.json` 的 `handtest_prep.prep_conversation_id`（形如 `w02-a1ht-<stamp>`）；**不要点「＋ 新对话」**（新会话从 n0 开始）。
6. **先自检**（同一会话）发「乐局状态」：应显示第 3 关「我的节奏先发声」（`n2-rhythm`）。若显示第 1 关/「没有进行中的乐局」→ 停，记 `FIXTURE-RESTORE`（多半是关了/重开了工程）。
7. **选第二个 clip**：目标轨 `我的旋律（选第二个 clip）` 上：
   - 第一个（控制组）：第 1 小节起（beat 0–4），C4/E4/G4 各 1 拍；
   - **第二个（要选的）**：第 5 小节起（beat 8–12），G4(2拍) + B4(半拍) + D5(1.5拍)。
   在 DAW 时间线/钢琴卷帘里**单选第二个 clip**（选中态可见）。
8. **说「写好了」**（同一会话；不要用自然语句，避免 auto-intro）。预期：回复 n2 通过/进入第 4 关；chat context `clip_scope=selected`、`clip_ids[melody-track]`=第二个 clip，节点推进。
9. **记录**：`coord/runs/W02-A1-handtest/SESSION-<stamp>.md`（时间、会话 id、回复原文、`clip_scope`/`clip_ids` 证据路径、截图、是否推进；失败按 §6 分类）。

## 5. 重置（同栈）

- **重置 = 重跑 prep**：在同一栈、同一工程里再跑一次 `-SameStackPrep`（新 `-HandtestRoot`，或先删旧 `pristine`）。prep 会：新会话重新 开始乐局→n2、清空目标/参考轨上的旧 clip、重建两个 clip、重命名、重新 save、刷新 pristine。
- **不要**用「关闭工程再打开」来重置：那会清空会话（PROJ-OPEN-RESUME-1），必须再跑 prep 才能回到 n2。
- `reset_handtest_live.ps1`（pristine→live 文件还原）只适用于**旧 reopen 模式**的 fixture 或手工放在 `<HandtestRoot>/live` 的目录；同栈模式不需要它（且文件还原后仍需重跑 prep 才能有 n2 会话）。

## 6. 失败分类（手测轮）

| 分类 | 现象 | 处置 |
|---|---|---|
| `FIXTURE-RESTORE` | 「乐局状态」显示第 1 关/n0 或「没有进行中的乐局」 | 工程被关闭/重开过，或 prep 未成功；重跑 prep 后重试，仍失败则交 lead。 |
| `FIXTURE-CLIP` | 目标轨不是 2 clip / 找不到两个 clip | 停；重跑 prep；仍复现交 lead（prep 已断言过）。 |
| `SESSION-LOST` | 选到新会话（n0）或左栏找不到 prep 会话 | 停；确认没有点「＋ 新对话」；记录左栏可见项。 |
| `GUI-SELECTION` | 选中第二个 clip 后回复/证据是 `clip_scope=first` 或无 `selected_clip_id` | GUI 选择没进 chat context；记录截图+`clip_scope`，交 lead（v0 核心待证点）。 |
| `ENV` | 栈/端口/进程/超时/窗口异常 | 按环境中断记录并排除该轮。 |
| `API-GAP` | 拿不到 `clip_scope`/工具时间线等证据面 | 只记录缺口，不把“没证据”写成通过。 |
| `MODEL` | 误用自然语句触发 LLM/auto-intro | 记录该轮；重跑 prep 重置后再测。 |

## 7. 诚实边界

- 同栈 prep 证明「会话在运行中的栈里确实在 n2」+「工程文件里确实有 tracks/clips/音符」；**不**证明跨进程/跨 open 的会话恢复（PROJ-OPEN-RESUME-1 明确不恢复）。
- prep 的 `restore_probe` 只是盘上状态的支撑证据（`node_active`/`not_found`），不冒充冷启动恢复。
- `select_clip` 仍 UI-only（A1c G1）：prep 只记录 `ui_action`；真实选中绑定由手测 GUI 提供。
- 栈唯一 owner；prep 不启停栈、不占端口；prep 后栈仍归 lead，停栈后由 lead 核 process/TCP/UDP 零。
- recents/authority 按 S0 快照还原；prep 的 save 会 `prependRecentProject`，teardown 恢复。
- 文件域：`scripts/run_w02_a1c_smoke.ps1`、`coord/runs/W02-A1-handtest/`。

## 8. 零栈验收（本 worktree 实测，2026-11-19）

| 项 | 命令 | 实测 |
|---|---|---|
| 解析（Windows PowerShell 5.1） | `[System.Management.Automation.Language.Parser]::ParseFile(...)` | **PARSE_ERRORS=0**；UTF-8 BOM 在，中文命令字面量可读 |
| 无授权 | `... -RepoRoot <worktree>` | **exit 3**；未创建 RunRoot |
| dry-run（status，默认） | 上条加 `-DryRun` | **exit 0** |
| dry-run（同栈 prep） | 上条加 `-SameStackPrep -HandtestRoot <dir>` | **exit 0**；打印同栈计划，不建 HandtestRoot |
| dry-run（旧 reopen 模式） | 加 `-HandtestPrep -HandtestRoot <dir>` | **exit 0**（计划里标注 BLOCKED） |
| 同栈缺 `-HandtestRoot` | `-SameStackPrep -StackAuthorized -RuntimeOwner selfcheck` | **exit 2**（S0 前拒绝） |
| 同栈 + reopen 冲突 | 两者同给 | **exit 2** |
| `-ProjectPath` 无 `-SameStackPrep` | `-ProjectPath D:/x.vit -DryRun` | **exit 2** |
| 静态卫生 | `Select-String -Pattern 'Start-Process|Start-Job|TcpListener|HttpListener|WebRequest|netstat|Get-NetTCPConnection|Stop-Process'` | **0 命中** |
| 同栈 helper 单测 | AST 提取 `Get-HandtestLiveEvidence`/`New-HandtestPrepNotes`：合成 project.state（2 轨/2 clip/正确音符）→ 证据正确；3 clip → 断言抛错；mode=same_stack 的 notes 含同栈标记与第二个 clip id | 通过 |
| reset 脚本 | 合成 fixture：dry-run / 真重置 / 漂移修复 / 缺 pristine | **0 / 0（文件数+digest 相等）/ 0 / 2** |
| 前提亲证（Go） | `cd agent; go test ./internal/chat -run TestSameProjectReopenStartsBlankAndArchivesSavedConversation -count=1` | **ok 0.502s** |

未运行真实栈；未消耗 LLM；未改产品代码。

## 9. 与旧 reopen 模式 / spike 的关系

- 旧 `-HandtestPrep`（save→复制 live/pristine→**重开** live 验证）保留，但 summary 标记 `blocked_by_proj_open_resume=true`，**不得用于手测**；它仍能取证「.vit 是否持久化 tracks/clips」。
- 21:18 spike 的 copy 无 clip 是「A1c 在 S2 后没保存」；同一 spike 的「新会话从 n0」+ PROJ-OPEN-RESUME-1 共同说明：**跨 open 的 n2 恢复不可用**，所以手测必须同栈。

## 10. 停止条件

- 工程未打开/未保存、`-ProjectPath` 不匹配、HandtestRoot 已含 pristine、save 未重写 .vit、live 两轨/2 clip/音符不符、会话不在 n2、零 LLM 被破坏 → **停**，保留 `PREP-<stamp>` 与 RunRoot 工件，交 lead。
- 手测轮按 §6 分类停；不原样重跑（先重跑 prep，再判定执行偏差/卡片缺陷/假设失效）。
- 任何“为通过而”修改设计源/产品代码/期望值的行为禁止。
