# FIX-START-DRAFT-SIDE-EFFECT-1（137）实现记录：读路径零物化 + reader 不破坏 pristine + 跨工程泄漏守卫

- 卡：`coord/cards/doing/2026-10-18-FIX-START-DRAFT-SIDE-EFFECT-1.md`（池序 137，P1）
- owner：DSH PC 执行流 `exec-start-draft`（task-12）；分支 `port/fix-start-draft-side-effect-1`，worktree `D:/Vit_DAW_wt_startside1`
- **实际基线**：`origin/main=77a0b02f`（领取提交，卡面领取行写 3abe0cfb——两者差仅协调提交、无生产代码差异；以本文件为准，请 Lead 验收时更正卡面）
- 依据：118 归因件 `coord/runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md` §2.1/§2.4 + `REALSTACK_REPORT.md`（reader D/E 轮 1.5–2s 实证）
- 范围：`agent/internal/history/{history.go,working_session.go}`、`agent/internal/harness/harness.go`、`agent/internal/chat/server.go` + 同包测试；**未动前端/内核**；**118 最小修复（空壳 draft 复用）原样保留**（history_test.go 两个测试未改，全绿）

## 1. 事件链（修复前 → 修复后）

修复前（118 取证）：
```
Server.Start→scheduler tick recoverContinuationWorkspace
  → harness.CurrentProjectIdentity   (harness.go:518)
  → history.BindProjectIdentity("", kernelUUID)  ← 读路径副作用
  → history.defaultDraftProjectPath()             ← 落盘建 75B draft A
  → chat.activateCurrentProjectWorkspace          ← 用 A 作 reference 恢复
  → 启动期自主 reader version.list/version.worktree_list (harness.invoke)
  → history.Status/List/WorktreeList → Open("")/historyState → listWorktreeItems
  → ensureRootWorktreeItem 落盘 .vit_history/.../state/worktrees  ← 读路径副作用
  → ProjectNew 后旧 uuid 竞态重绑 → Open→migrateLegacyHistoryWorkspace 在
    新 draft 内物化旧 uuid workspace（跨工程泄漏）
```

修复后：
```
read 面永远不创建 draft：
  CurrentProjectIdentity: shadow uuid 无 path 时只用"已激活 workspace hint"，
  否则返回空 path（不再 BindProjectIdentity("",uuid)）；
  history.Status/List/WorktreeList: project_path 为空时只读进程内已缓存 draft，
  无缓存 → 返回 available=false / draft_pending=true（形状保持），不开目录；
  historyStateReadOnly/listWorktreeItemsMode(ensure=false): 读不再 materialize
  root worktree item（写路径 Checkpoint/ProjectNew/WorktreeCreate 显式 ensure）；
  激活/恢复: chat.activateCurrentProjectWorkspace 先只读 root 扫描
  history.RecoverableProjectPathForUUIDInRoot(uuid)，再回落到
  history.CurrentDraftProjectPath()（只读缓存），都没有则"activation pending"
  跳过（不建 draft）；激活成功后 harness.SetProjectPathHint(path,uuid)，
  自主 reader 命中激活工程 → 读该工程历史（只读）；
写面显式绑定：
  save/save_prepare 等写意图调用方在 path 为空时显式
  history.CurrentDraftProjectPath()/EnsureProjectPathForWrite(uuid)（可物化）；
  118 的 ProjectNew 空壳复用继续作为第二道防线；
泄漏守卫：
  BindProjectIdentity 拒绝把已绑定 active workspace/session 的路径改绑到其它
  （旧）uuid，并记录 ProjectIdentityRebindRejection + observer（harness.New
  注册为 WARN 日志）→ 不静默吞错。
```

## 2. Lead 五项执行要求逐条答案

### ① `CurrentProjectIdentity` 去 BindProjectIdentity 副作用：调用方影响面清单（全仓 grep，非测试 17 处 + 测试间接）

| 调用点 | 性质 | 修复后行为/处置 |
|---|---|---|
| harness.go:574 `ProjectHistorySummaryForProject` | 读 | path 可能为空 → `available=false/draft_pending=true`；hint 就绪时命中激活工程 |
| harness.go:735 `planVitConversationCheckpoint` projectKey | 读（gate key） | path 空时 gate key=""（无磁盘副作用；该 gate 仅内存） |
| harness.go:1318 `prepareProjectLifecycleCommand`（save/save_as） | **写** | path 空→`CurrentDraftProjectPath()`→`EnsureProjectPathForWrite(uuid)` 显式绑定 |
| harness.go:1379 `applyProjectLifecycle` uuid 兜底 | 读（仅 uuid） | 不变（不用 path） |
| harness.go:2958/2962 `version_project_save_prepare` | **写** | path 空→显式 `CurrentDraftProjectPath()`/`EnsureProjectPathForWrite` |
| harness.go:3125 `scheduleLegacyProjectPackageMigration` | 读（身份比对） | path 空且无 hint→跳过后台迁移（Info 留痕；不落盘） |
| harness.go:9013 `historyArgs`（工具面读写共用） | 读+写 | 读工具（version.status/list/worktree_list）→读面防护；写工具（checkpoint 等）→`Open("")` 写路径按需物化（既有语义） |
| harness.go:11974 `persistAudioAnalysisManifest` | 写（manifest） | path 空→跳过（激活 hint 通常在场；不落 draft） |
| harness.go:11990 `recoverAudioAnalysisManifest` | 读 | path 空→回退 shadow 内嵌 manifest（既有分支） |
| harness/project_folder_export.go:21 `saveAsFolder` | 写 | path 空→回退 projectstore 当前根；仍空则明确 error（原有语义保留） |
| chat/audition_events.go:1142 | 写（settle 记录） | 先 activeWorkspacePath，再 identity；空→return（原有分支） |
| chat/audition_candidate_adoption.go:69 | 读 | path 空→明确 error `active project path unavailable`（audition 需活动工程） |
| chat/free_state_conclusion_bridge.go:54 | 读（仅 uuid） | 不变 |
| chat/continuation_scheduler.go:789/804 | 读（激活触发） | 触发 `activateCurrentProjectWorkspace` 新只读解析（InRoot→缓存→pending） |
| chat/continuation_scheduler.go:1392 | 读（记录路径） | 先 activeWorkspacePath 兜底（既有） |
| chat/mix_workflow_queue_view.go:419 | 读（仅 uuid） | 不变 |
| chat/note_sessions.go:312 | 读/track | 空→空表（fail-open 既有语义） |
| chat/server.go:835 /runtime/status | 读 | 空→note_sessions 空行（既有 fail-open） |
| chat/server.go:4770 记录 conversation 节点 | 写 | 空→保留 chat context path；写路径 `Open("")` 按需物化（既有） |
| chat/server.go:7289 project context 注入 | 读 | 空→context 不带 path；后续写路径按需物化 |
| chat/server.go:7307 `activateCurrentProjectWorkspace` | **核心** | 新只读解析 + hint 发布（见 §1） |

结论：**无调用方依赖"读取即绑定"语义**；两个真正需要 path 的写意图调用方（save 准备、save_prepare）已改为显式绑定并各补测试；其余读调用方在 path 空时走既有 fail-open/明确 error 分支，不落盘。

### ② 读函数 unavailable 的 API 契约保持 + 消费方清单

- `history.Status/List/WorktreeList` 返回 **既有 JSON 形状 + 新增可区分字段**：`available=false`、`draft_pending=true`、`initialized=false`，且 `commits/worktrees/conversation_messages` 为空数组、`branches/refs` 空 map（不改变任何既有键存在性/类型；不改错误码）。
- 消费方：
  - `harness.ProjectHistorySummaryForProject`：透传 `draft_pending` 并把 `available` 置 false（原来是 available=true+空历史，现在可区分"尚未有工程历史"）；`worktree_count` 仍提供（0）。
  - `harness.executeCommand` 的 `version_status`/`version_list`/`version_worktree_list` 工具回包：形状不变，多一个 `draft_pending`；工具面（webui/Godot 历史面板）无需改动（新增键可忽略，空列表行为与空历史一致）。
  - chat `/agent/state`（handleState→ProjectHistorySummaryForProject）、audition CurrentPlane、goalrunner 上下文摘要：读到 `available=false` 时按"暂无工程历史"分支（不会误判为工程已打开）。
  - 单元测试：新增断言见 §4；既有测试全绿（见 §5）。

### ③ `listWorktreeItems` 隐式写移除：写路径保留 + 读后零变化断言

- 新增 `historyStateReadOnly`/`listWorktreeItemsMode(repo, ensureRootWorktree)`：
  - 读面：`Status/List/WorktreeList` → ensure=false（零写）；
  - 写面：`historyState`（Checkpoint/ProjectNew/分支/fork 等既有写函数）→ ensure=true（行为与原一致）；`Checkpoint`、`ProjectNew` 另在各自身**显式**调用 `ensureRootWorktreeItem`；`WorktreeCreate` 原有显式调用保留。
- 保留测试：`TestWritePathsKeepRootWorktreeMaterialization`（ProjectNew/Checkpoint/WorktreeCreate 三条写路径均生成 root worktree item）。
- 读后零变化断言：`TestReadHistoryFacesKeepPristineDefaultDraft`、`TestReaderUsesWorkspaceHintWithoutMaterializing`、`TestRecoverableProjectPathForUUIDInRootIsReadOnly` 使用**全树快照比对（路径+文件内容 sha256+文件/目录 mtime，NTFS 目录时间戳先冻结）**，不只目录数。

### ④ 泄漏守卫不静默吞错

- `history.BindProjectIdentity` 拒绝 active workspace 重绑时：记录 `ProjectIdentityRebindRejection{project_path, active_project_uuid, rejected_project_uuid, at}`（有界 32 条，`ProjectIdentityRebindRejections()` 可查）并通知全局 observer；`harness.New()` 注册 observer → **WARN 日志** `[workspace] refused stale project identity rebind path=... active_uuid=... rejected_uuid=...`。
- round2 竞态测试：`TestBindProjectIdentityRefusesStaleRebindOfActiveWorkspace` 覆盖两种绑定顺序（active→stale 拒绝并记录；new→active 后 stale 到达仍拒绝），并断言磁盘上不出现旧 uuid workspace；另有 `TestBindProjectIdentityAllowsUuidChangeWithoutActiveWorkspace` 防止误伤正常身份切换。

### ⑤ 测试覆盖（118 + D/E reader + 新增 4 类）

| 类 | 测试（新增，均属对应包） |
|---|---|
| ① 读路径不建 draft | `history.TestReadHistoryFacesDoNotMaterializeDefaultDraft`、`harness.TestCurrentProjectIdentityDoesNotMaterializeDraft`、`chat.TestActivateWorkspaceDoesNotMaterializeDraft` |
| ② reader 后 pristine 保持 | `history.TestReadHistoryFacesKeepPristineDefaultDraft`、`history.TestProjectNewReusesPristineDraftAfterReaderFaces`（D/E 真栈场景的单测化：reader 抢跑→ProjectNew 复用→无旧 uuid）、`harness.TestReaderUsesWorkspaceHintWithoutMaterializing`（含 hint uuid 不匹配不跨工程读）、`harness.TestReaderAfterProjectNewDoesNotAddDraft` |
| ③ 并发旧 uuid 不泄漏 | `history.TestBindProjectIdentityRefusesStaleRebindOfActiveWorkspace`（两顺序）+ 上述 D/E 测试内旧 uuid 断言 |
| ④ 118 两测试 + 全量 | `history.TestProjectNewReusesPristineDefaultDraft`、`history.TestProjectNewCreatesFreshDraftWhenDefaultDraftIsInitialized`（原样保留）+ `go test ./...` |
| 其他保留/新增 | `history.TestWritePathsKeepRootWorktreeMaterialization`、`history.TestRecoverableProjectPathForUUIDInRootIsReadOnly`、`harness.TestSavePrepareMaterializesDraftExplicitly`、`chat.TestActivateWorkspaceUsesMaterializedDraftWithoutNewDir`、`chat.TestActivateWorkspaceRecoversSeededDraftWithoutMaterializing` |

## 3. 代码变更摘要

```
agent/internal/history/history.go             | 220 ++++++++++++++++++++++++--
agent/internal/history/working_session.go     |  22 +++
agent/internal/harness/harness.go             |  84 +++++++++++-
agent/internal/chat/server.go                 |  28 +++-
+ 新测试文件 3 个（history/harness/chat 各一）
```

关键新增 API（均有注释）：
- `history.CurrentDraftProjectPath()` / `history.DraftProjectRoot()`（只读）
- `history.EnsureProjectPathForWrite(uuid)`（写意图显式绑定/物化）
- `history.RecoverableProjectPathForUUIDInRoot(uuid)`（无 reference draft 的只读 root 扫描；原 `RecoverableProjectPathForUUID` 语义不变）
- `history.ProjectIdentityRebindRejections()` / `SetProjectIdentityRebindObserver()` / `ProjectIdentityRebindRejection`
- `harness.SetProjectPathHint(path,uuid)` / `ProjectPathHint()`
- 只读内部面：`openRepoForRead`、`unmaterializedHistoryState`、`historyStateReadOnly`、`listWorktreeItemsMode`

## 4. 验证

基线：`origin/main=77a0b02f`；worktree `D:/Vit_DAW_wt_startside1`（HEAD=77a0b02f + 本卡改动）。

| 验证 | 命令 | 结果 |
|---|---|---|
| 全量编译 | `cd agent && go build ./...` | **exit 0** |
| 全量单测（隔离 draft 根） | `VIT_HISTORY_DRAFT_ROOT=<临时目录> go test ./... -count=1` | **exit 0；95 包 ok / 12 包 no test files / 0 FAIL**（`evidence/full_tests_isolated.txt`） |
| 三包回归（history/harness/chat） | `go test ./internal/history/ ./internal/harness/ ./internal/chat/ -count=1` | **全部 ok**（history 5.6s / harness 18.8s / chat 95.8s；三包均含在同一全量日志内） |
| 新增 4 类单测 | 见 §2⑤ 清单（history 8 个新测试 / harness 5 个 / chat 3 个） | 全 PASS（含并发旧 uuid 重绑、reader 工具面零物化、D/E reader→ProjectNew 复用） |
| gofmt | `gofmt -l` 本卡改动文件 | 无输出（干净） |
| 测试隔离核对 | 隔离 env 全量跑前后真实 draft 根目录数 | **10189→10189（0 新增）** |
| 测试污染偏差（既有基线问题） | 本卡未隔离的首轮包测试向真实 draft 根写入 18 个测试工件 | 已移出到 `C:\Users\timoz\AppData\Local\Temp\fedraft1\startside_test_quarantine\`（未删除，可复核；目录内无 `.vit_agent`/session，纯测试历史脚手架）；随后隔离复跑真实根零新增。并入 140/FIX-TEST-DRAFT-ISOLATION-1 域 |
| 真栈腿 | 需栈窗口；脚本 `scripts/run_stack_leg_137.ps1`（自起 stock kernel + 本分支 agent、隔离根+种子旧 draft、含 reader 抢跑、断言 A–D、finally 只杀自起 pid） | 待 Lead 登记 PC-RUNTIME-STACK 后 GO（回执附 run ID/断言/拆栈证据） |

## 5. 偏差/边界

- 实际基线 77a0b02f（卡面写 3abe0cfb，仅协调提交差异），请卡面更正。
- `CurrentProjectIdentity` 不再保证返回可写路径；写意图调用方须用 `EnsureProjectPathForWrite`（已在 save/save_prepare 落地；其他写工具经 `historyArgs`→`Open("")` 仍可按需物化，符合"读零写显式"边界）。
- 读面在"无缓存 draft 且无 hint"时返回 available=false/draft_pending=true；这是对"空历史"的细化，不是新错误路径。
- `harness.go` 中途一次 pwsh 文本改写造成非 ASCII 注释损坏，已 `git checkout --` 恢复后**仅用 edit 工具重放**全部 harness 改动；最终 diff 以 git 为准（见回执）。
- 118 最小修复保留：两个 118 测试未改动并通过；`ProjectNew` 空壳复用仍是第二道防线（现在读路径不再产生空壳，复用分支在真栈 reader 抢跑窗口仍可命中）。
