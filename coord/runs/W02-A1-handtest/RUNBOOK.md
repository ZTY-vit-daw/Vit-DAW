# W02-A1-HANDTEST-V0 手测 RUNBOOK（n2 预置工程 + 手测包）

- 卡：`coord/cards/doing/2026-11-19-W02-A1-HANDTEST-V0.md`（池序 434；验收负责人=lead）。
- owner：`w02ht`（DeepSeek-V4.1-Flash，PC，独立 worktree）。
- 分支：`codex/w02-a1-handtest-434`；worktree `C:/Users/timoz/.codex/worktrees/pc434-handtest/Vit_DAW`；基线 main `4906fd21`。
- 本轮范围：**只加 prep 模式 + 手测包**（不改产品代码/设计源/432 卡/其他脚本，不启栈）。
- 状态：**prep 真栈轮未运行（候 lead 登记栈位并授权）**；零栈验收已在本 worktree 跑完（§7）。
- **前提上报（2026-11-19）：卡的「重开验证 clip」与「prep 会话在 n2」在当前产品下互斥，见 §0.1。prep 模式已实现并在该点 fail-honest（exit 1），需 lead 先裁决。**

## 0. 一句话

`scripts/run_w02_a1c_smoke.ps1 -HandtestPrep -HandtestRoot <dir>` 在**已存在的真实栈**上复用 S0/S1/S1.5/S2 做到 n2 的 clip 节点、建两个可区分 clip、重命名两轨、`project.save` 持久化、复制 `live`/`pristine`、**重开 live 副本**验证两轨/两 clip/音符与 prep 会话节点，然后停在 n2（不跑 S3–S6、零 LLM）。**注意：当前产品在重开边界清空 agent 会话（PROJ-OPEN-RESUME-1，§0.1），所以“重开后 prep 会话仍在 n2”这一步会 fail-honest；clip 持久化部分不受影响。**手测由用户在 VitShell 里打开 `live` 工程、选中 **prep 会话**、选**第二个 clip**、说「写好了」完成。

## 0.1 前提核查（必读；2026-11-19 上报）

**发现：`project.open` 有意丢弃 agent 会话态，prep 的「重开 live 验证 clip」与「prep 会话在 n2」互斥。**

- 产品裁定 **PROJ-OPEN-RESUME-1**（用户 2026-09-13 裁定）：重开工程只回到最后一次手动保存点，**不**把保存点里的对话/运行时状态还给 live 上下文；保存点里的会话被**归档并清空**。
- 代码锚点：`agent/internal/history/working_session.go:216-245`（`OpenWorkingSessionAtGeneration` → `archiveReopenedAgentState` 归档 graph+runtime）；`working_session.go:441`（`writeBlankAgentRuntimeState` 写出的空态不含 conversations / edu_jam_sessions）；`agent/internal/harness/harness.go:1502`（`project.open` 必走 `OpenWorkingSessionAtGeneration`）。
- 亲证（零栈，本 worktree 跑）：`cd agent; go test ./internal/chat -run TestSameProjectReopenStartsBlankAndArchivesSavedConversation -count=1` → **ok 0.502s**；该测试断言“保存点里的会话（以及其上的草稿）都不会作为 live 上下文回来”，归档在 `.sessions/.../archive/`。
- 直接后果：prep 在 S2 之后 save + 重开 live 副本时，prep 会话的乐局 session 被清空；随后「乐局状态」返回 `edu_jam_no_session`。prep 模式在此 fail-honest：**exit 1**，`summary.handtest_prep.conversation.restore_blocked_by=PROJ-OPEN-RESUME-1`，并保留 `PREP-<stamp>/summary.json`（含已通过的 save/copy/reopen-clip 取证）。
- 也就是说：卡的步骤 3（保存+重开验证 clip）**成立且已被脚本验证**；卡的验收项“prep 会话在 n2（重开之后）”**在当前产品下不成立**。这不是 fixture 没保存，而是产品在 open 边界主动丢弃。

**需要 lead 裁决的路径（本轮未擅自改产品/改手测设计）：**

| 方案 | 内容 | 代价 |
|---|---|---|
| A｜产品卡（先评估） | 在 open 边界或“选择对话流”时恢复该对话的乐局 session（或允许显式“继续此对话”恢复）；与 2026-09-13 裁定冲突，需要用户重新裁定 | 产品改动 + 用户裁定；不在本卡文件域 |
| B｜同栈手测 | prep 在用户实际使用的栈上跑；重开验证后**在同一 prep 会话里**重新 开始乐局→推进到 n2（确定性、零 LLM），然后不关栈、不重开，用户直接在该窗选 clip + 说「写好了」 | 需要 prep 增加“重开后再置 n2”步骤（可另开小卡）；手测不再是“冷启动打开 live” |
| C｜缩减 v0 | 只测 GUI 选中→`clip_scope=selected`：lead 在运行中的栈里手工把会话推进到 n2，再把窗口交给用户 | 不验“跨冷启动 fixture”；手测前置靠 lead 手工步骤 |
| D｜放弃 v0 | 等 A 落地再做 | 本轮无手测结论 |

- 无论选哪条，**prep 的 clip 持久化取证（save 重写 sha256、重开两轨/2 clip/音符一致、live==pristine）都已独立成立**，可直接用于 A/B/C。
- 本 runbook §4 的步骤是**方案 A/B 落地后的目标流程**；在当前产品（无 A/B）下 §4-5 会停在 `FIXTURE-RESTORE`。

## 1. 产物

真栈轮（由 lead 跑）产出：

```
<HandtestRoot>/
  live/          # 手测工程：<name>.vit + <name>.vit_project/ + .vit_agent/ + .vit_history/
  pristine/      # reset 基线（prep 在重开验证前从 live 复制，digest 相等）
  handtest.json  # 工程文件名、prep 会话 id、预期节点、track/clip id、.vit sha256
<worktree>/coord/runs/W02-A1-handtest/PREP-<stamp>/
  summary.json   # 本轮完整结论（handtest_prep 段 = 保存/复制/重开/会话/零 LLM 证据）
  PREP-NOTES.md  # 由 summary 生成的 prep 说明（路径、ids、边界）
```

- `PREP-<stamp>` 目录按**脚本自身所在 checkout** 计算（不跟 `-RepoRoot` 走），所以 prep 记录始终落在执行器 worktree 的 `coord/runs/W02-A1-handtest/`。
- `live`/`pristine` 是**运行工件**，不提交；`HandtestRoot` 建议放仓库外（例 `D:/Vit_DAW_handtest/W02-A1-n2`）。
- 每次 prep 必须用**新的** `HandtestRoot`；目录里已有 `live`/`pristine` 时脚本 `exit 2`（避免覆盖旧 fixture）。
- 失败时 teardown 会删除本轮创建的 `live`/`pristine`（不交付坏 fixture），删除路径记在 `summary.teardown.handtest_failed_fixture_removed`。

## 2. prep 怎么跑

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:/Users/timoz/.codex/worktrees/pc434-handtest/Vit_DAW/scripts/run_w02_a1c_smoke.ps1 -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 -StackAuthorized -RuntimeOwner "lead/<PC-RUNTIME-STACK 登记 owner>" -TimeoutSeconds 240 -HandtestPrep -HandtestRoot D:/Vit_DAW_handtest/W02-A1-n2 -RunRoot <新目录>/run_<stamp>
```

- `-HandtestRoot` 必填；`-HandtestPrep` 与 `-QuestionMode work` 互斥（`exit 2`）。
- 脚本**不启停栈**：S0 只读 `/health`、`/agent/runtime/status`、`/agent/state`、`/agent/authority`；栈必须空闲、当前工程干净（unsaved/draft 时 `exit 2`）。
- 与 status/work 相同的前置快照：recents、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml` 按字节还原；`/agent/authority` 快照/还原。
- **teardown 不发送「结束乐局」**：prep 会话必须留在 n2，所以不关局（status/work 模式行为不变）。

退出码：

| 码 | 含义 |
|---|---|
| 0 | prep PASS：保存后重开 live 副本有两轨/目标轨 2 clip/音符一致，prep 会话在 n2，零 LLM，teardown 还原完成，PREP 记录已写。 |
| 1 | 确定性断言失败（`assert`）：`.vit` 未被重写、重开后 clip 丢失/音符不一致、参考轨非空、prep 会话不在 n2（`edu_jam_no_session`＝PROJ-OPEN-RESUME-1 边界，见 §0.1）等。 |
| 2 | 环境/前置/teardown（`preflight`/`http`/`teardown`）：栈不可达/busy/dirty、HandtestRoot 不合规、还原失败。 |
| 3 | 未授权（无 `-StackAuthorized`）：只打印摘要，零 HTTP、零写盘。 |
| 5 | 证据/API 缺口（`api`/`evidence`）：项目包/agent 目录缺失、零 LLM 被破坏、fixture 清单写不出等。 |

失败即停：不重跑、不手工修 fixture、不把部分产物当 PASS。

## 3. prep 通过口径

1. `project.save` 前后 `.vit` 的 sha256 **变化**（文件被真正重写）；`live` 与 `pristine` 的 `.vit` sha256、文件数、整树 digest 相等。
2. 重开 `live/<name>.vit` 后 `get_project_state`：两个用户轨（`我的旋律（选第二个 clip）` / `参考（空白）`），目标轨 **2 个 clip**、参考轨 0 个，两个 clip 的 `get_midi_clip_notes` 回读签名与 S2 写入完全一致（`67@0/2,71@2/0.5,74@2.5/1.5` 与 `60@0/1,64@1/1,67@2/1`）。
3. 重开后在同一 prep 会话发「乐局状态」：`stop_reason=edu_jam_status`、`level_id=<card_id>`、`current_node=n2-rhythm`。**当前产品在此步返回 `edu_jam_no_session`（§0.1），脚本 exit 1；这是已知前提缺口，不是脚本抖动。**
4. telemetry 新增行中 prep 会话的 LLM 行数为 0（S1/S1.5/S2 全是确定性命令）。

补充（支撑证据，不作唯一判据）：fixture 内 `agent_runtime_state.json` 探针应显示 prep 会话、`n2-rhythm` 为 `active`；探针 `not_found` 时只 warn 并记录（见 §6 边界）。

## 4. 手测步骤（VitShell 产品入口；方案 A/B 落地后适用）

1. **停 prep 栈**（lead）：保留 `PREP-<stamp>`/`live`/`pristine`；核 process/TCP/UDP 零并登记 `coord/resources/PC-RUNTIME-STACK.md` 释放。
2. **启动产品入口**：`启动VitShell.cmd`（AGENTS §5 手测入口；不要用 `dev_agent_smoke.ps1 -StartUI` 当手测入口）。
3. **打开 live 工程**：VitShell → 打开工程 → `<HandtestRoot>/live/<name>.vit`（按路径打开；**不要从 recents 打开旧 RunRoot 路径**——prep 已还原 recents，live 不在列表里）。
4. **在左栏（对话流列表）选 prep 会话**：最后一条应是「开始乐局 / 乐局事件 … / 写好了」推进链的那条；对话 id 见 `PREP-<stamp>/summary.json` 的 `handtest_prep.prep_conversation_id`（形如 `w02-a1ht-<stamp>`）。**不要点「＋ 新对话」**：spike 已证新会话从 n0 开始（21:18 证据）。
5. **先自检**（同一会话）发「乐局状态」：应显示第 3 关「我的节奏先发声」（`n2-rhythm`）。若显示第 1 关 / 「当前没有进行中的乐局」→ **停**，记 `FIXTURE-RESTORE`（当前产品在此步即为该结果，见 §0.1），不要继续。
6. **选第二个 clip**：目标轨 `我的旋律（选第二个 clip）` 上有两个 clip：
   - 第一个（控制组）：第 1 小节起（beat 0–4），音符 C4/E4/G4 各 1 拍；
   - **第二个（要选的）**：第 5 小节起（beat 8–12），音符 G4(2拍) + B4(半拍) + D5(1.5拍)。
   在 DAW 时间线/钢琴卷帘里**单选第二个 clip**（选中态要可见）。
7. **说「写好了」**（同一 prep 会话；不要用自然语句，避免 auto-intro 干扰）。这一步是手测的核心：验证 GUI 选择→chat context。
8. **预期**：
   - 回复显示 n2 通过/进入第 4 关「四种走向，四个两小节」；
   - chat context：`clip_scope=selected`、`clip_ids[melody-track]`=**第二个 clip**（不是第一个、不是空）；节点推进（`node_done=n2-rhythm`）。
   - 取证面：`/agent/events` 工具时间线、`agent_prompt_render.jsonl` 的 `<tool_result>`/`lesson_context`、kernel 日志；GUI 面截图（选中态 + 回复）。
9. **记录**：`coord/runs/W02-A1-handtest/SESSION-<stamp>.md`（会话 id、时间、原始回复、`clip_scope`/`clip_ids` 证据路径、截图、是否推进；失败按 §5 分类）。
10. **reset**（重测前）：

```powershell
# 预览（不改文件）
powershell -NoProfile -ExecutionPolicy Bypass -File ./coord/runs/W02-A1-handtest/reset_handtest_live.ps1 -HandtestRoot D:/Vit_DAW_handtest/W02-A1-n2 -DryRun
# 真重置（先关 VitShell / DAW 窗口）
powershell -NoProfile -ExecutionPolicy Bypass -File ./coord/runs/W02-A1-handtest/reset_handtest_live.ps1 -HandtestRoot D:/Vit_DAW_handtest/W02-A1-n2
```

reset 会删除 `live/`、从 `pristine/` 重拷并校验（`.vit` sha256、文件数、整树 digest 一致）；`exit 0`=已恢复，`exit 2`=环境/用法问题（多为栈仍占用 live 工程，先关窗）。

## 5. 失败分类（手测轮）

| 分类 | 现象 | 处置 |
|---|---|---|
| `FIXTURE-RESTORE` | 「乐局状态」显示第 1 关/n0 或「没有进行中的乐局」 | 停；当前产品为 PROJ-OPEN-RESUME-1 的预期结果（§0.1），交 lead 裁定方案 A/B。 |
| `FIXTURE-CLIP` | live 工程打开后没有两轨 / 目标轨不是 2 clip | 停；先跑一次 reset，仍复现则交 lead（prep 已断言过，属环境/副本问题）。 |
| `SESSION-LOST` | 选到新会话（n0）或列表里找不到 prep 会话 | 停；确认没有点「＋ 新对话」；记录左栏可见项。 |
| `GUI-SELECTION` | 已选第二个 clip，但回复/证据是 `clip_scope=first` 或无 `selected_clip_id` | GUI 选择没进 chat context；记录截图 + `clip_scope`，交 lead（v0 的核心待证点）。 |
| `ENV` | 栈/端口/进程/超时/窗口异常 | 按环境中断记录并排除该轮，不归因模型。 |
| `API-GAP` | 拿不到 `clip_scope`/工具时间线等证据面 | 只记录缺口，不把“没证据”写成通过。 |
| `MODEL` | 手测里误用自然语句触发了 LLM/auto-intro | 记录该轮；用 reset 重开 fixture 再测，不把误触发算作 v0 结论。 |

## 6. 诚实边界与已知缺口

- **PROJ-OPEN-RESUME-1（关键）**：重开工程会归档并清空 agent 会话/运行时状态；`prep 会话在 n2（重开之后）` 在当前产品下不成立。证据与方案见 §0.1；脚本在此 fail-honest，不伪造。
- **重开即清空会话（PROJ-OPEN-RESUME-1）**：无论是同进程重开还是冷启动打开，`project.open` 都会归档并清空会话/运行时状态；prep 的 `project.open live` 只能证明 `.vit` 里的 tracks/clips/音符，**不能**证明 prep 会话仍在 n2（当前产品下必然 `edu_jam_no_session`）。`summary.handtest_prep.restore_probe` 只反映 fixture 盘上有没有会话记录（`node_active` / `conversation_without_active_node` / `not_found`），不冒充“重开后仍可用”。
- **左栏会话可见性以产品展示为准**：本卡不新增读取会话列表的 API；若左栏看不到 prep 会话，按 `SESSION-LOST` 停，不用新会话补。
- **`select_clip` 是 UI-only**（A1c G1）：prep 的 `select_clip` 只记录 `ui_action`；真正的选中绑定在手测里由 GUI 提供。
- **prep 不消耗 LLM**：S1 导入/S1.5 事件推进/S2 建 clip 都是确定性命令；`乐局状态` 无会话时返回 `edu_jam_no_session`（同样零 LLM）。
- **栈唯一 owner**：prep 脚本不启停栈、不占端口；prep 结束后栈仍归 lead，停栈后由 lead 核 process/TCP/UDP 零。
- **recents/authority 还原**：prep 会 `prependRecentProject`（save/open 的无条件行为），teardown 以 S0 快照按字节还原；手测打开 live 请用**路径**而不是 recents。
- **文件域**：只改 `scripts/run_w02_a1c_smoke.ps1`、`coord/runs/W02-A1-handtest/`；不写产品/设计源/432 卡。

## 7. 零栈验收（本 worktree 实测，2026-11-19）

| 项 | 命令 | 实测 |
|---|---|---|
| 解析（Windows PowerShell 5.1） | `[System.Management.Automation.Language.Parser]::ParseFile(...)` | **PARSE_ERRORS=0**；UTF-8 BOM 在，中文命令字面量可读 |
| 无授权 | `... -RepoRoot <worktree>` | **exit 3**；未创建 RunRoot |
| dry-run（status，默认） | 上条加 `-DryRun` | **exit 0**；零 HTTP |
| dry-run（prep） | 上条加 `-HandtestPrep -HandtestRoot <dir>` | **exit 0**；打印 prep 计划，不建 HandtestRoot |
| prep 缺 `-HandtestRoot` | `-HandtestPrep -StackAuthorized -RuntimeOwner selfcheck` | **exit 2**（进入 S0 前拒绝） |
| prep + work 互斥 | 同上加 `-QuestionMode work` | **exit 2** |
| 静态卫生 | `Select-String -Pattern 'Start-Process|Start-Job|TcpListener|HttpListener|WebRequest|netstat|Get-NetTCPConnection|Stop-Process'` | **0 命中** |
| reset 脚本 | 合成 fixture：dry-run / 真重置 / 漂移修复 / 缺 pristine | **0 / 0（文件数+digest 相等）/ 0 / 2** |
| prep helper 单测 | 从 AST 提取函数：`Get-TreeDigest`/`Get-TrackClips`/`Find-EduJamStateProbe`/`New-HandtestPrepNotes`/`Invoke-HandtestPrep` 守卫 | 符合预期（digest 相等/检测漂移；探针 `node_active`/`not_found`/`conversation_without_active_node`；已存在 live/pristine 与空 HandtestRoot 都抛 `W02A1C\|preflight`） |
| 前提亲证（Go） | `cd agent; go test ./internal/chat -run TestSameProjectReopenStartsBlankAndArchivesSavedConversation -count=1` | **ok 0.502s**（PROJ-OPEN-RESUME-1：save 点会话不回到 live 上下文） |

未运行真实栈；未消耗 LLM；未改产品代码。

## 8. 与 21:18 spike 的关系

- spike 直接复制 A1c `run_2020` 工程后 `project.open` 成功，但 `.vit` 里没有 S2 的 tracks/clips：A1c 只在 S1 `save_as` 过一次，S2 建完 clip **没有保存**；复制件里的 `.vit` 仍是空工程。这不是“保存不持久化”，而是“没保存”。
- spike 另证：新会话 auto-intro 从 n0 开始、不恢复 n3。结合 §0.1：**即使保存了，重开也会清空会话**，所以“选 prep 会话得到 n2”的前提不成立。
- 结论：clip 持久化可由 prep 证明；n2 会话的跨 open 恢复需要产品裁定（§0.1 A/B/C/D）。prep 若在重开后丢失 clip 或拿不到 n2，均 `exit 1` 停手上交（不交付坏 fixture）。

## 9. 停止条件

- 栈不可达/busy/dirty、HandtestRoot 已存在 live/pristine、prep 会话在重开后在 n0/`edu_jam_no_session`、clip 丢失或音符不一致、零 LLM 被破坏 → **停**，保留 `PREP-<stamp>` 与 RunRoot 工件，交 lead。
- 手测轮按 §5 分类停；不原样重跑（先 reset，再判定是执行偏差/卡片缺陷/假设失效）。
- 任何“为通过而”修改设计源/产品代码/期望值的行为禁止。
