# FE-START-EMPTY-DRAFT-1 归因件：启动页双 draft + kernel↔shadow 轨道不同步

- 卡：`coord/cards/doing/2026-10-15-FE-START-EMPTY-DRAFT-1.md`（池序 118，P1，L1）
- 执行 owner：DSH PC 执行流 `exec-start-draft`（2026-10-18 午窗）；归因腿先行、不占真栈
- 基线：origin/main `c33a5cfe`；分析分支/worktree：`port/fe-start-empty-draft-1` @ `D:/Vit_DAW_wt_fedraft1`
- 证据生成：`scripts/collect_evidence.py` → `evidence/draft_inventory.json`（可重跑，只读，不写 draft 根）
- 范围声明：**本文只做归因，不改生产代码**。前端仓只读分析（用户线 `D:/Godot/project/vit-daw-frontend@port/fe-gui-shrink-1 c56e38f`）。四轮样本位于 Go agent 的 ProjectHistory draft 根 `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\`（**不在** `coord/runs/M2-EXPERIENCE-1/` 内；该目录只有 state/events 工件）。

## 0. 结论摘要（因果链）

```
VitApp 启动（内核恢复/默认工程：有 project_uuid、project_path=""）
   │  shadow 初始状态 = 内核工程快照（uuid 非空、path 空）
   ▼
VitAgent 启动 → Server.Start() → continuation scheduler 首个 tick
   ▼  continuation_scheduler.go:789 CurrentProjectIdentity
harness.go:518-530：uuid 非空且 path 空 → history.BindProjectIdentity("", uuid)
   ▼  history.go:134-138 → defaultDraftProjectPath()（history.go:2260-2288）
【draft A = 隐式空壳】（只有 75B Unsaved.vit；无 .vit_history/.vit_agent）
   ▼  同一 tick：activateCurrentProjectWorkspace（scheduler:810 / server:7301）
RecoverableProjectPathForUUID(bareA, 旧 uuid)（working_session.go:602-670）
  → 在兄弟 draft 里扫出同 uuid 的**旧 session draft**（例如 9 月 8 日会话）
  → 切换到旧 draft → RecoverWorkingSession + ReadAgentRuntimeState（42MB 读）
  → 恢复旧会话语义并**回写 42MB agent_runtime_state.json**（server.go:7479-7494）
   ▼  用户从启动页点「新建工程」（距 agent 启动 1–3 秒）
kernel new_project → 前端 POST /agent/invoke {"tool":"version.project.new"}
   ▼  start_page.gd:811-831 → 975-993；chat/server.go:1727-1746（异步）
harness.applyExternalProjectNew（harness.go:2982-3004）→ history.ProjectNew
   ▼  history.go:777-793：**清空 draft 进程缓存** + Open("") 再建一个 draft
【draft B = 真实工程 draft】（.vit_history/.vit_agent 全量初始化，绑定新 project_uuid）
   ▼  refreshShadow（harness.go:3003）

结果：每轮进入 = draft A（75B 弃件）+ draft B（实际工程）。A 永远不会被初始化/使用。
```

结论一句话：**两个 draft 都由 Go agent 建，但触发源不同**——draft A 是 agent 启动期「uuid 已知、path 未知」的**读路径副作用**（`CurrentProjectIdentity` 内部的 identity 绑定顺手物化了默认 draft）；draft B 是前端 `version.project_new` 通知后的**显式新建**（`ProjectNew` 先清缓存再建新 draft，所以不会复用 A）。**不是前端建 draft，也不是内核建 draft**；内核只提供「uuid 有值、path 为空」的初始工程身份，前端只把 new_project 结果转成一条 agent 工具通知。

四轮 + 今日一次旁证样本（共 5 次）签名一致：**draft A 先出现（早 1–3 秒）、只有 75B Unsaved.vit；draft B 后出现并初始化**。归因件证据见 `evidence/draft_inventory.json`。

## 1. 缺陷 A：双 draft 创建因果链（已证）

### 1.1 五个样本的目录签名（`evidence/draft_inventory.json` `rounds[]`）

| 样本 | draft A（隐式空壳） | draft A 内容 | draft B（显式工程 draft） | draft B 内容 |
|---|---|---|---|---|
| round1 2026-10-15 09:52 | `draft_20261015T015232_041e9c3d` | 1 文件 / 75B Unsaved.vit | `draft_20261015T015233_4a3e9fef` @09:52:33 | 19 文件，uuid `vitproj_69a7…` |
| round2 2026-10-15 12:37 | `draft_20261015T043731_c86c7a3b` | 1 文件 / 75B | `draft_20261015T043732_236b4272` @12:37:32 | 23 文件，uuid `vitproj_76b4…`（+泄漏旧 uuid `vitproj_69a7…`，见 §4） |
| round3 2026-10-15 21:00 | `draft_20261015T130048_1f3dff6a` | 1 文件 / 75B | `draft_20261015T130051_1fdf3d88` @21:00:51 | 34 文件，uuid `vitproj_86ce…` |
| round4 2026-10-16 18:16 | `draft_20261016T101633_4494fa5a` | 1 文件 / 75B | `draft_20261016T101634_b87bbe28` @18:16:34 | 25 文件，uuid `vitproj_6f49…` |
| **live5 2026-10-18 13:45**（旁证，见 §6） | `draft_20261018T054528_95b6e3cc` | 1 文件 / 75B | `draft_20261018T054530_2ffafb51` @13:45:30 | 8 文件，uuid `vitproj_d70f…` |

- draft A 在 5/5 样本中：只有 `Unsaved.vit`（75B），**无** `.vit_history`、无 `.vit_agent`、无任何 session/commit → 从未被任何写路径使用。
- draft B 在 5/5 样本中：`.vit_history/{project_uuid}/workspace.json`（`migrated_from_legacy: true`）+ `.vit_history/.sessions/...` + `.vit_agent/{project_uuid}` 全量存在。
- A→B 时间差：round1/2/4 = 1s，round3 = 3s，live5 = 2s；且 **A 的 mtime 与 VitAgent 启动同秒**（round4 18:16:33、live5 13:45:28），早于前端 new_project 通知（B 的 09:52:33/…/13:45:30）。

### 1.2 代码锚点

| 环节 | 锚点 | 说明 |
|---|---|---|
| 隐式 draft 物化点 | `agent/internal/history/history.go:131-155` `BindProjectIdentity`（`:134-138`：path 空 → `defaultDraftProjectPath()`） | 一个"绑定身份"的读辅助函数在 path 空时会**落盘建 draft** |
| draft A 文件内容 | `agent/internal/history/history.go:2260-2288` `defaultDraftProjectPath`（`:2275` 目录名，`:2280-2283` 写 `<EDIT draft="true" …/>`） | 目录名 `draft_<UTC 时间戳>_<shortID>`，内容 75B，与实测 A 完全一致 |
| 启动期触发者 | `agent/internal/chat/continuation_scheduler.go:733-742` `Server.Start()`；`:756-775` 调度 loop；`:778-811` `recoverContinuationWorkspace`（`:789` `CurrentProjectIdentity`，`:810` `activateCurrentProjectWorkspace`） | agent 启动后首个 scheduler tick（250ms 级）就会调用 `CurrentProjectIdentity` |
| 读路径副作用 | `agent/internal/harness/harness.go:518-530` `CurrentProjectIdentity`（`:526` `history.BindProjectIdentity`） | shadow 报 uuid 非空、path 空（内核未保存工程）时，任何读身份调用都会建 draft |
| 显式 draft 创建 | `agent/internal/chat/server.go:1727-1746`（`version_project_new` 异步受理）→ `agent/internal/harness/harness.go:2982-3004` `applyExternalProjectNew`（`:2983` `history.ProjectNew`，`:2990` bind，`:3003` refreshShadow） | 前端通知触发的一条 agent 工具调用 |
| **双建的直接原因** | `agent/internal/history/history.go:777-793` `ProjectNew`（`:778-781` **清空** `draftProject.root/path` 缓存后 `Open("")`） | `ProjectNew` 语义=「Start a fresh unsaved draft」：先破坏进程内 draft 缓存再新建，因此永远不复用已存在的 A，A 变弃件 |
| 前端触发 | `app/startup/start_page.gd:811-831` `_on_new_project_pressed`（kernel new_project）→ `:975-993` `_try_enter_main`（`:982-985` POST `/agent/invoke` `version.project.new`）→ `:1003-1010` 进主场景 | 前端只发**一条** new_project 通知；不负责建 draft、不重复发 |
| 内核角色 | 内核只提供初始工程身份：`VitApp/Workspace/default_project.xml`（`vit_project_uuid` 持久化，例：`vitproj_d70f…`/`vitproj_71a8…`/历史 `vitproj_b3f2…`），`project_path` 为空 | 内核不写 ProjectHistory/drafts（全仓 VitApp 源码无该路径常量/写入点） |

### 1.3 可证伪点与残余待证

- **已证**：draft A 的目录名/内容只能由 `history.defaultDraftProjectPath` 产生；draft B 的初始化序列（legacy migration→session→store）只能由 `ProjectNew`+`BindProjectIdentity`+`EnsureWorkingSession` 产生；round4 现场 VitAgent pid 启动时刻=18:16:33，与 A 的 mtime 同秒；前端仓 A/B/C 三分支代码里都没有创建 draft 目录的实现。
- **待证**：draft A 的 `BindProjectIdentity("", uuid)` 具体由哪一次调用触发（scheduler 首 tick 的 `recoverContinuationWorkspace` 概率最高，但也可能是首个 `/agent/state` 轮询；两者都在 250ms–1s 内）。区分方法：在 agent 日志（`VIT_AGENT_LAST_LOG_PATH`，`[continuation.wake]`/`[workspace] activated` 行）或最小 headless 探针里加时间戳；当前四轮现场日志未留档（见 §7 探针方案 P1）。
- **待证**：内核 `default_project.xml` 的 project_uuid 如何从上一轮工程传播到下一轮启动（round2 启动恢复目标=round1 的 `69a7`、round4 启动恢复目标=round3 的 `86ce…`，见 §2.2 表）。这决定 §3 的恢复目标，不影响双建结论。

## 2. 缺陷 B：kernel↔shadow 轨道不同步（round1 `track_count=0`）（已证主因，机制级）

### 2.1 事实

- round1 内核侧有真实音轨：`.vit_derived/<uuid>/l3_feature_log.jsonl` = 18 条真实音频分析（257,639B，6 轨 × 3 族；README 晚窗勘误），且 round1 draft B 存在 `mixboard_feature_snapshot.json` 391KB（逐轨波形/响度）。
- agent shadow（`agent_state_1003_check1/2/1005` 三份一致）：`initialized=true`、`track_count=0`、`user_track_count=0`、`engine_track_count=0`、`last_delta_seq=209`、`delta_seq_gaps=0`、`nodes_by_uid=34`、**`orphan_uid_keys=35`**、`state_epoch=209`、`project_uuid=69a7`（工程身份正确）。
- round3/round4（同一「启动页新建工程+导入」路径）：shadow `track_count=6`、`engine_track_count=6`。
- round4 events seq6 另有一条独立失真：agent 的 `track.list` 工具（VSP canonical `project.tracks.list`）返回 `track_count=0`，而 2 秒后 `get_project_state` 返回 6 轨（详见 §5）。

### 2.2 机制（代码锚点）

1. **shadow 的轨道计数只来自"权威快照"，不来自 delta**：`agent/internal/shadow/shadow.go:164-209` `Summary()` 的 `track_count/engine_track_count` 都从 `p.state["engine_snapshot"]["tracks"]` 计算（`:168-173`、`:187-190`）；`applyDeltaLocked`（`:227-306`）只把 delta 写进 `nodes_by_uid`/`orphanDeltas`，**不会**把新轨道并入 `engine_snapshot.tracks`。
2. **新轨道只走 delta**：round1 的 `nodes_by_uid=34 / orphan_uid_keys=35` 正是「轨道/插件/clip 的 delta 落在 bootstrap 快照不存在或未包含的 uid 上」的签名（`collectTrackUIDs`/`orphanDeltas`：`shadow.go:291-297`、`:317-333`）。即：round1 的最后一次权威快照是**新建工程的空态**（0 轨），用户随后在 Godot 里导入的 6 轨只以 delta 到达。
3. **agent 没有周期性/自动 shadow 刷新**：`Harness.StateSummary` 只在 `!shadow.Initialized()` 时刷新（`agent/internal/harness/harness.go:488-496`）；track 级刷新只挂在工具回包（`spec.RefreshAfter`，`harness.go:1739-1761`）和工程生命周期通知（`harness.go:3003/3054`）上；`/agent/state` 轮询不会刷新。
4. **round1 的刷新链为什么断**：round1 进入后「ask→slice 122s 静默空窗」（117 卡）——LLM 前卡点，agent 一个工具都没执行（journal_records=0、observations_count=0），因此没有任何 `RefreshAfter` 触发；`version_project_new` 的刷新发生在用户导入音轨**之前**（该次快照=0 轨）。→ shadow 一直停在空态快照，直到 10:05 停止仍未刷新。
5. round3/4 为什么正常：agent 在 ask 后正常执行观察/工具链（round4 seq5-8 的 `project.state`，round3 的 CCB 观察），任一 `RefreshAfter` 回包都会 `refreshShadowWithStatus`（`harness.go:13082-13167`，优先 VSP `state.snapshot`，回退 legacy `get_project_state`）→ 快照更新为 6 轨。

**结论**：round1「kernel 有轨、shadow 0 轨」= **shadow 权威快照未刷新 + 新轨道仅存在于 delta/orphan 面**，不是内核丢轨、也不是 draft 选择错误（shadow 的 project_uuid 正确）。它与「双 draft」**解耦**（round3/4 双 draft 复现但 shadow 正常；round1 失同步另有 117 的静默空窗条件）。117 卡可直接复用本条作为「前置组装等待 shadow/DAD 就绪→永等」假设的支撑。

### 2.3 残余待证

- round1 的 shadow 首快照具体是「内核默认工程」还是「新建工程空态」：两者都是 0 轨，对结论无影响；`[shadow] initialized tracks=…` Info 行（`shadow.go:98-104`）会给出答案，但四轮现场 agent 日志未留档（§7 P1 探针可补）。
- 是否需要对「导入路径」增加刷新兜底属**修复决策**（可选方向：内核 import 后发布 authoritative snapshot；或 agent 在 delta 出现未知 uid 时按阈值触发一次 `refreshShadow`）。本卡 P1 目标先消除双 draft，此条建议另开卡或并入 117。

## 3. 缺陷 C（附带）：42MB 旧工程 agent_runtime_state 启动加载（已证机制 + 现场复现）

### 3.1 事实

- 目标文件：`draft_20260905T134132_3866b080/.vit_history/.sessions/vitproj_b3f2…/session_20260908T102051_099b621b/workspace/state/agent_runtime_state.json`，**41,980,404B**（≈40MiB，卡面记 42MB），内容 `schema_version=vit_project_agent_runtime.v2`、`project_uuid=vitproj_b3f2…`（2026-09-05 的旧 draft 工程）。
- round1 卡面记录：09:52:36 agent 启动期写该文件（当时 mtime 观测）。
- **live5 旁证样本（2026-10-18 13:45）现场复现**：13:45:28 建 draft A（75B）→ 13:45:29 旧 draft 的 `session.json` 被更新 → 13:45:31 旧 draft `.vit_agent/.../journal`+`manifest` 被写 → **13:45:32 该 42MB 文件被整体重写**（mtime=今天 13:45:32，内容 `saved_at=2026-10-18T05:45:31.877Z`）。13:45:30 draft B 同时创建。即：**同一条启动恢复路径今天仍在活动**（该样本 owner 未在 PC-RUNTIME-STACK 登记，见 §6）。

### 3.2 机制（代码锚点）

1. 启动恢复目标解析：`agent/internal/history/working_session.go:602-670` `RecoverableProjectPathForUUID`——在 ProjectHistory `drafts/` 兄弟目录里遍历同名 uuid 的 `agent_runtime_state.json`，**逐个 `os.ReadFile` + `json.Unmarshal`**（`:636-646`）挑最新者；40MB 文件每候选读一次。
2. 激活并恢复：`agent/internal/chat/server.go:7301-7415` `activateCurrentProjectWorkspace`——`:7316` 改路径到旧 draft；`:7386-7389` `RecoverWorkingSession`（`working_session.go:64-120`，其中 `:97-99` 再次整读同一 42MB 文件判断 pending work）→ `:7397-7405` `ReadAgentRuntimeState`（`history.go:1016-1023` 整读）→ `restoreProjectAgentRuntimeStateLocked`（server.go:7405）。
3. 回写：`agent/internal/chat/server.go:7479-7494` `persistActiveProjectWorkspaceLocked`——`json.MarshalIndent` 整个投影后 `history.WriteAgentRuntimeState`（`history.go:990-1014`）**整体覆盖写回 42MB**。
4. 触发时机：agent 启动 → scheduler 首 tick（`continuation_scheduler.go:778-811`）→ 激活；一次启动内 42MB 被**读 ≥2 次 + 写 1 次**。现场时间窗：live5 13:45:28→13:45:32 ≈ 4s（含旧 draft 会话/日志写）；round1 卡面 09:52:33→09:52:36 ≈ 3s，与「启动被 40MB 级历史态拖慢秒级」的观测一致。

### 3.3 归因结论（回答卡面三问）

- **是否按最近工程恢复？** 不是"最近工程"语义。恢复目标是**内核启动时报告的 project_uuid**；agent 在 draft 根里找**同 uuid 的最新 runtime state**。若该 uuid 来自内核默认/遗留工程（live5 与卡面 round1 均命中 2026-09 旧 uuid `b3f2`），恢复的就是**几个月前的会话**，与用户本次要进的新工程无关。uuid→draft 映射的正确性依赖内核 `default_project.xml` 的 uuid 传播链（§2.2 表、§1.3 待证）。
- **是否应加载？** 现在加载发生在「用户已决定新建工程」之前，加载结果随后被 `version_project_new` 的工程切换整体丢弃，属于**被丢弃的前置恢复成本**；且它把旧 draft 写活（跨会话污染 + 秒级启动延迟）。建议：启动恢复只对「同一内核工程身份且确有未完成工作」的场景生效，或把恢复延迟到前端 `version.project_opened`（打开既有工程）时再按需加载——这属设计裁定，**本卡不做代码修复**，交 Lead 决定是否拆卡。
- **是否拖慢启动？** 现场证据支持秒级（3–4s）延迟：live5 的 draft A→42MB 写回窗口 4s；round1 卡面窗口 3s。但该段与 new_project 通知并发，用户可感知成本主要是 draft 弃件与旧 draft 写活，启动 UI 门是否有感需栈内探针（§7 P2）。

## 4. 跨工程 `.vit_history` 泄漏候选（round2 实证，机制已证、触发时序待证）

### 4.1 事实

round2 的 draft B（`draft_20261015T043732_236b4272`）`.vit_history/` 下同时存在两个 uuid workspace：

- `vitproj_76b4…`（本轮新工程，`migration_completed_at=2026-10-15T04:37:33.009Z`）
- `vitproj_69a7…`（**round1 的工程 uuid**，`migration_completed_at=2026-10-15T04:37:33.041Z`）

两者 `workspace.json` 的 `project_path` 都指向 **round2 draft B 的 Unsaved.vit**，`legacy_project_state` 都指向 round2 draft B 的 `.vit_history/projects/Unsaved_76a0f62a5e03`；即：**旧工程 uuid 的 workspace 目录被物化进了新 draft**（round1 draft B 与 round3/4/live5 的 draft B 都没有该签名）。

### 4.2 机制（代码锚点）

- `history.Open` 在「有 UUID 绑定、无 workspace 目录」时会调 `migrateLegacyHistoryWorkspace`（`history.go:196-223`，`:218-222`）。
- `migrateLegacyHistoryWorkspace`（`history.go:2804-2858`）把共享 legacy 状态迁到 `historyRoot/<repo.ProjectUUID>`（`historyRoot = <project dir>/.vit_history`），并写 marker `workspace.json`（`:2849-2857`）。
- 触发漏洞：`ProjectNew` 之后、`refreshShadow` 生效之前，shadow 仍报**旧** `project_uuid`；此时任何并发读路径（`/agent/state` 轮询 → `ProjectHistorySummaryForProject` → `historyArgs` → `CurrentProjectIdentity`）会执行 `BindProjectIdentity("", 旧uuid)`（`harness.go:526`），而空 path 解析到缓存里的 **draft B**（`history.go:2272-2274`），把 draft B 重新绑成旧 uuid；随后 `history.Status/List` 经 `Open(draftB)` 见到旧 uuid 无 workspace → migration 在 **draft B 里**创建 `vitproj_69a7…` 目录。round2 的 32ms 间隔（`.009Z` 新 uuid → `.041Z` 旧 uuid）与该竞态一致。
- 危害：draft B 的 `.vit_history` 出现非本工程 uuid 的 session/对象目录，后续按 uuid 扫描（如 `RecoverableProjectPathForUUID`、project history 工具）可能把旧工程会话归到新工程头上（round2 之后 `session_20261015T043733_4f7906bc` 即出现在该 draft）。

### 4.3 已证/待证

- **已证**：产物形态只能由 `migrateLegacyHistoryWorkspace` 产生（字段/路径与代码逐字一致）；发生在新 draft 上；时间窗（新 uuid .009→旧 uuid .041）落在 `ProjectNew`→`refreshShadow` 窗口内。
- **待证**：触发该读请求的具体 call site（前端 `/agent/state` 轮询 vs agent 自身其它读路径）。1/5 轮命中（round2）说明是竞态非必现；最小探针可按 §7 P3 用「ProjectNew 后紧接并发 `/agent/state`」复现注入。

## 5. 缺陷 D：第四轮 `track.list` 工具面 0 轨（events seq7 自述）（待证，候选面已锁定）

- 证据：`events_round4_1817.json` seq5-6：agent 调 `track.list` → 回包 `command_name=list_tracks`、`status=ok`、`command_ack.message="Track list fetched"`、`track_count=0`、`user_track_count=0`、`tracks=[]`；agent 在 seq7 body 自述「track.list 返回 0 条轨道…」，随即 seq8 `get_project_state` → `track_count=6`（`evidence/draft_inventory.json` `m2_events_tracklist`）。
- agent 侧锚点：`agent/internal/harness/harness.go:1771-1772` 把 `list_tracks` 映射为 VSP canonical `project.tracks.list`（**VSP 通道**），而 `get_project_state` 未映射、走 legacy 命令通道。
- 内核侧锚点：`VitApp/Source/Service/VspKernelReference.cpp:87` canonical→legacy 映射 `project.tracks.list → list_tracks`；`:2551-2572 dispatchLegacyCommand` 只做 base_revision 校验后**同步调用 legacyHandler**；`CommandDispatcher.cpp:2342-2356` VSP envelope 的 legacyHandler 就是同一个 `CommandDispatcher::dispatch`；`:2670-2674` `list_tracks → TrackService::handleListTracks`；`TrackService.cpp:444-466` 用 `getEdit()` + `te::getAllTracks(*edit)` 枚举。
- 因此「ack 成功但 0 轨」**不可能**是空 edit（空 edit 会返回 `No active edit loaded` 错误）——候选解释只剩：
  1. VSP 通道命中的内核/编辑实例与 GUI 当前 edit 不是同一个（例如 hub 侧存在陈旧 kernel 连接、或 VSP 会话路由到了另一个内核实例）——本轮 agent 侧无法证实/证伪，需在栈内用 `get_project_state` 与 `list_tracks` 同帧对照 + vsphub 连接表探针；
  2. `te::getAllTracks(*edit)` 在「新建工程+导入」路径下与对 GUI 可见轨道存在枚举差（需要内核侧编辑实例身份打印）。
- 与缺陷 B 的关系：**不是同一签名**。round1 是 shadow 快照不刷新（shadow 的 `track_count` 字段），round4 是 VSP legacy 工具回包（`track.list` 工具面）；共同点是「工具/投影面与内核权威态不同步」，但代码路径不同（shadow Summary vs kernel TrackService），故本卡不把两者合并归因；track.list 0 轨交**栈内探针/新卡**（§7 P4），agent 自愈（get_project_state）已让第四轮旅程走通。

## 6. 旁证样本 live5（2026-10-18 13:45）说明

- 13:45:28 与 13:45:30 在 draft 根出现两个新 draft，签名与四轮完全一致（A=75B 空壳 / B=d70f 全量），同时 13:45:32 旧 42MB draft 被重写。**本文将其作为机制的第 5 次独立旁证**（证明当前 main 仍可复现，不依赖已清理的历史现场）。
- 该样本运行时**没有任何 PC-RUNTIME-STACK 占用登记**（13:52 只读核对：VitApp/VitAgent/vsphub=0、五端口监听=0；仅在场的 `Godot_v4.6.1 … --editor` 是用户编辑器）。执行侧无法确定它的 owner（疑用户 92 手测 / 他流栈腿 / Lead 探针）。**仅登记为旁证，不作为本卡验收证据**；如 Lead 能确认该栈归属，可提升为「自动化/半自动路径独立复现」证据。
- 补充（14:05 只读核对）：14:04 起 PC 真实栈再次在场（VitApp pid 21624 / VitAgent pid 11452 / VspHub pid 7844，端口 5556/5555/7878/8787 监听，运行目录 `coord/runs/KERNEL-RENDERGAIN-1`，journal `source=pcactl.certify_compressor`）——属**132 栈腿活动**，与本卡无关；13:45 样本不等于 132 腿（早于其 14:04 启动）。执行侧全程未读写该栈进程，也未启停任何栈。

## 7. 可自动化最小探针方案（不启真栈即可做 P1/P3；P2/P4 需栈窗口）

> 通用前提：所有探针使用**独立临时 draft 根**（`VIT_HISTORY_DRAFT_ROOT=<t.TempDir()>` 或专用临时目录），绝不写真实 ProjectHistory；符合 AGENTS §10。

- **P1（Go 单测，headless，零真栈）— 双 draft 复现器**：构造 harness/chat server，用假 kernel 初始回复 `{"status":"ok","project_uuid":"vitproj_old","project_path":""}`；触发 `CurrentProjectIdentity`（或直接 `BindProjectIdentity("", "vitproj_old")`）→ 断言 draft 根出现 draft A（只有 Unsaved.vit）；再调 `history.ProjectNew` → 断言出现 draft B 且 A 未初始化。**可 100% 确定性复现双建**，是本卡修复的回归测试基线。
- **P2（Go 单测，headless）— 42MB 恢复成本**：在临时 draft 根放两个同 uuid draft，其 runtime state 分别 40MB/小；调 `RecoverableProjectPathForUUID` + `activateCurrentProjectWorkspace` 等价序列，断言恢复目标选择与读写次数/耗时（可对比修复前后）。真栈拖慢验证放栈腿（需 Lead 窗口）。
- **P3（Go 单测，headless）— 跨工程泄漏竞态**：ProjectNew 新建 draft B 后，在 shadow 仍报旧 uuid 的窗口内并发调 `ProjectHistorySummaryForProject`/`history.Status`，断言 draft B 下**不应**出现旧 uuid 的 `.vit_history/<old>`（当前代码会失败=复现器）。
- **P4（真栈探针，需窗口）— track.list vs get_project_state 同帧对照**：在栈内对同一工程连续发 `list_tracks`（legacy 与 VSP canonical 各一次）+`get_project_state`，打印内核侧 edit 身份；用于 §5 候选面裁决。**不伪造通过**。
- **P5（用户手测清单，UI 类验证只能交用户）**：见 §9。

## 8. 修复范围建议（供 Lead 定，不在归因腿实施）

| 缺陷 | 最小修复候选（agent 侧） | 风险/备注 |
|---|---|---|
| A 双 draft | **让 `ProjectNew` 复用"仍为原始空壳"的默认 draft**：若 `draftProject.path` 指向的 draft 无 `.vit_history`/`.vit_agent`/无 commits 且尚未被任何工程使用，则清缓存后直接复用该路径而非新建目录；否则维持现状。等价于把 75B 弃件变成实际工程 draft，双建消失，且不改变"新建工程=空历史"语义 | 小、可单测（P1 回归）；不动前端、不动内核 |
| A 的根因（读路径建 draft 副作用） | 长期：`CurrentProjectIdentity` 不应有落盘副作用；或 `defaultDraftProjectPath` 改为懒创建（首次写历史时才落盘） | 面大（影响重启恢复/绑定语义），建议另卡 |
| C 42MB 启动加载 | 只报告，不修（需设计裁定：恢复时机/staleness/是否只在 project_opened 时恢复） | 交 Lead 决定是否拆卡 |
| 4 跨工程泄漏 | 上报（同属 stale-uuid rebind 问题）；若与 A 修复同域，可在 `BindProjectIdentity` 加「默认 draft 已绑定其他 uuid 时不得改绑」守卫 + P3 单测 | 与 A 修复同一文件域，建议同卡或紧随其后 |
| B shadow 失同步 | 上报 117 复用；可选兜底=delta 出现未知 uid 超阈值时触发一次 refreshShadow | 不并入本卡最小修复 |
| D track.list 0 轨 | 栈内探针裁决（P4）；内核侧/传输侧，非本卡 agent 最小修复面 | 交 Lead 定 |

## 9. 需用户手测项（UI 语义，自动化不可替代）

1. 从 Godot 前端启动页点「新建工程」→ 进入主界面 →（不进任何工程）发一句简单话术；手测后由执行侧核对 draft 根**只新增 1 个 draft**（当前为 2 个）。
2. 同上路径进入**既有工程**（最近项目/文件对话框）→ 确认打开的是目标工程本身（project_uuid/path 正确），且 draft 根**不新增**空壳 draft；打开失败时错误必须显形（不得静默落入新空 draft）。
3. 在新建工程中导入音轨后，立刻问「现在有几条轨道？」→ 期望 agent 回答=实际轨数（当前 round1 同路径会答 0，round3/4 正常；修复后应稳定）。
4. 进入既有工程后 `track.list` 类问题（列出轨道）→ 期望与工程实际轨数一致（当前 round4 工具面返回 0，agent 自愈但多一轮）。

## 10. 证据索引

| 证据 | 路径（相对本目录） | 说明 |
|---|---|---|
| 四轮+live5 draft 全量清单（文件/mtime/size/sha256/identity/legacy 字段） | `evidence/draft_inventory.json` | 由 `scripts/collect_evidence.py` 生成（2026-10-18 13:57 +0800）；含 `rounds[]`、`old_draft_42mb`、`m2_run_state`、`m2_events_tracklist` |
| 采集脚本（只读、可重跑） | `scripts/collect_evidence.py` | `--draft-root`/`--run-dir`/`--out-dir` 可覆盖；不写 draft 根 |
| round1/round4 agent state（shadow/project_history 提取） | `evidence/draft_inventory.json → m2_run_state` | 原始文件 `coord/runs/M2-EXPERIENCE-1/agent_state_{1003_check1,1005_after_second_stop,round4_1820}.json` |
| round4 track.list/get_project_state events（seq5-8 回包） | `evidence/draft_inventory.json → m2_events_tracklist` | 原始文件 `coord/runs/M2-EXPERIENCE-1/events_round4_1817.json` |
| 42MB 旧 draft 事实 | `evidence/draft_inventory.json → old_draft_42mb` | 41,980,404B，mtime=2026-10-18 13:45:32（live5 复现覆盖） |
| 四轮 draft 目录原始位置 | `C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\` | 只读引用；本文所有样本名可逐字回溯 |
| M2 场次 README（时间线/勘误/串话术） | `D:/Vit_DAW/coord/runs/M2-EXPERIENCE-1/README.md` | 首跑/二/三/四轮时间线与「空工程勘误」 |
| 前端触发链（只读） | `D:/Godot/project/vit-daw-frontend` @ `c56e38f`：`app/startup/start_page.gd:811-831/975-993` | 用户手测线引用；未改前端仓 |
| agent 代码锚点基线 | `D:/Vit_DAW_wt_fedraft1` @ `port/fe-start-empty-draft-1`（=origin/main c33a5cfe） | 文中所有 `history.go`/`harness.go`/`server.go`/`shadow.go`/`continuation_scheduler.go` 行号以该基线为准 |
| 内核代码锚点基线 | 同上仓库 `VitApp/Source/Service/{VspKernelReference.cpp,TrackService.cpp,CommandDispatcher.cpp}` | §5/§1 内核角色 |
