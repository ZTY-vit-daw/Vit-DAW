# RUNBOOK — CONV-CROSS-COMBO-1（422）B2b 跨工作树真栈组合验证

**状态：装置已编写，零栈、未授权运行。** 编制：combo422（DeepSeek-V4.1-Flash，PC，独立 worktree），2026-11-19 晚；主管 lead。
**卡**：coord/cards/doing/2026-11-18-CONV-CROSS-COMBO-1.md（池序 422）。
**分支/基线**：codex/conv-cross-combo-422，worktree C:/Users/timoz/.codex/worktrees/pc422-combo/Vit_DAW，基线 main b9c9d943。
**本轮范围**：只写本 runbook 与 scripts/run_conv_cross_combo_smoke.ps1；**不启真栈、不占端口、不改产品代码**。真栈腿候主管登记栈位并书面放行。
**唯一执行器**：scripts/run_conv_cross_combo_smoke.ps1。默认无 -StackAuthorized 只打印本 runbook 摘要并 exit 3；-DryRun 零 HTTP 打印计划并 exit 0。

---

## 0 依据与已核对的真实契约

### 0.1 上游裁定

- 419 CONV-OPEN-TOOL-1 PASS：agent/internal/harness/conversation_open.go 实现 version.conversation_open（命令名 version_conversation_open），跨工作树时复用 switch autosave、物化目标节点、**恰好一次** withProjectReload；失败在 reload 前返回并披露 target_materialized；reload 失败返回显式原因。419 用 fake kernel 证明单次 reload 与 reload 失败分支。
- 421 CONV-RAIL-CROSS-1 PASS（零栈）：左栏其他工作树懒加载 version.status；【打开此工作树并继续此对话】发 version.conversation_open {project_file_path, branch, node_id}；失败保留预览并披露 target_materialized/branch_warning。
- 421 的 mock 断言只证明前端实参；422 要在真实 kernel+agent 上证明路径切换、节点/分支 active、单次 reload 与失败语义。

### 0.2 代码锚点（脚本运行时会静态核对前两项）

| 契约 | 锚点 |
|---|---|
| version.conversation_open 工具名/参数 | agent/internal/harness/conversation_open.go:34；name 或 project_file_path 至少一个，node_id 必填，branch 可选 |
| 单次 reload | conversation_open.go:216 仅一处 withProjectReload(ctx, "version_conversation_open", result)；harness.go:13548 的 withProjectReload 内仅一处 SendCommand({"cmd":"open_project"}) |
| reload 失败分支 | conversation_open.go:218-224，返回带 refresh.warning 的显式 error；419 TestConversationOpenReloadFailureIsExplicit |
| version.status 只读 | harness.go:3079 到 history.Status；history.go:307-322 走 openRepoForRead，不触发 kernel/DAW 切换 |
| version.worktree_create | harness.go:3114 到 history.WorktreeCreate；无 commit_id 时 worktreeCreateArgs 先做 source autosave checkpoint；**不 reload** |
| version.branch_create | harness.go:3102 到 history.BranchCreate + **一次** withProjectReload；from_node_id 存在时才追加 branch_marker 节点 |
| 421 前端实参 | agent/webui/src/project/ProjectSurface.tsx:459-464 发 version.status {project_path}；:571-580 发 version.conversation_open {project_file_path,branch,node_id} |

### 0.3 卡面措辞与实现差异（已登记，不造假）

1. **version.status 的工作树参数是 project_path，不是 project_file_path。** harness.historyArgs 只补/认 project_path，history.projectPath 只读 project_path/path；421 UI 实际也传 project_path。422 脚本按实现传 project_path。若未来要求 project_file_path 别名，需另开卡改 harness/history，不在本卡文件域。
2. **version.conversation_open 的目标工作树参数是 project_file_path**，此点与卡面一致；branch + 该分支 HEAD 节点可走非 detached 的 branch checkout。
3. **withProjectReload 的 open_project 不经过 applyProjectLifecycle("open")**。目标工作树的 Project History 绑定由 handleInvoke 的 pre-activation / defer sync 完成；因此 S4 在 reload 后必须再等 /agent/ui/state 落到目标路径，并重新 version.status 核对节点/分支/HEAD 未漂移。这是本卡要真栈证的关键点，不能只信 tool 回执。
4. **单次 open_project 只能从 kernel 日志计数。** kernel 的 Command received: 行每次命令一条；同一 VSP 原始载荷在 ZmqGateway/processing/Command received 多处出现，所以**不能**数全文 "cmd":"open_project" 出现次数（实测同一命令有 4 次出现）。脚本只数 ^Command received:.*"cmd":"open_project" 行。kernel 日志不可读时脚本 fail-honest 退出 2，不伪造计数。

---

## 1 目标与成功条件

在**真实栈**（VitApp kernel + Go agent；Godot DAW 可选）上，以零 LLM 的既有 agent HTTP/工具面完成：

1. 在隔离 RunRoot 下用 project.new + project.save_as 创建/保存真实工程 A；
2. 用 version.worktree_create 从 A 创建第二工作树副本 B；再用 version.branch_create 在 B 的产品生成 seed 节点上创建非 main 目标分支与 branch_marker 对话节点；project.open 回到 A；
3. 用 version.status {project_path:B} 只读浏览 B：断言目标文件路径、分支、节点、HEAD；断言 kernel 工程仍是 A、无 open_project、A/B .vit 哈希不变；
4. 用 version.conversation_open {project_file_path:B, branch, node_id} 打开 B：断言内核工程路径切到 B、目标分支/节点 active、非 detached、target_materialized=true、kernel_reloaded=true、**恰好一次** open_project、DAW 投影（/agent/ui/state 与 /agent/state.shadow）一致；
5. 失败语义：未知 worktree、失效 worktree 文件、未知 node、未知 branch 均返回显式 error、target_materialized=false、kernel_reloaded=false、零 reload、当前工程不变；
6. recents 与 Settings.xml/default_project.xml 启动前快照、finally 还原并哈希亲证；装置零进程/零端口，只写 RunRoot；真栈由 owner 停栈后另行核对全零。

**成功**：正式轮 run_conv_cross_combo_smoke.ps1 -StackAuthorized ... 退出 0，summary.json 中 S0–S6 全 true、可覆盖断言全绿、kernel_log.single_reload_count=1、recents.restored=true、authority.restored=true、teardown.apparatus_processes_started=0、apparatus_ports_bound=0。**一个成功路径不等于稳定性证明**；本卡不声明重复成功率。

---

## 2 前置门（缺一即停；不得降级执行）

### 2.1 栈位、授权、单 owner

- 主管在 coord/resources/PC-RUNTIME-STACK.md 新增占用行（卡 422、owner、候选 HEAD、run 目录、端口面、§8 口径），push 到共享 main，执行侧 fetch 重读确认；**≤60 秒内**复核 Vit 进程、TCP 5555/5556/7878/8787、UDP 4445 净空。未登记/未放行不得运行。
- 同一时刻只有一个 owner 使用同一套 kernel/agent；本脚本是唯一正式执行器，单正式轮。失败按 §6 分类停，不原样重跑。
- 栈由 owner 用既有产品入口/berth 启停；**本脚本不启动、不重启、不 kill 任何栈进程**。脚本只在 -StackAuthorized 后对已存在的 AgentBaseUrl 发 HTTP。

### 2.2 被测二进制与契约

- 候选必须包含 419/421（main b9c9d943 或其后合入的等价提交）；记录 agent 二进制 SHA256/构建时间、kernel 二进制 SHA256、webui dist（如启用 Godot）到 run_meta.json/回执。
- 真实栈最小形态：agent HTTP 7878、kernel ZMQ 5555/5556、VspHub 8787；Godot/VitShell 非必需（421 UI 已零栈验收，本卡验 agent 组合缝）。若打开 Godot，其 DAW 投影来自同一 kernel，脚本只断言 agent-visible 投影，不驱动 GUI。
- -StackAuthorized 前确认 /agent/runtime/status.kernel.connected=true、checkout_blocked=false；有 turn 在跑必须先 Stop Turn。

### 2.3 隔离与 A/B 工程

- 每次运行使用**新的空 RunRoot**；默认 <RepoRoot>/coord/runs/CONV-CROSS-COMBO-1/run_<stamp>_<8hex>，可由 -RunRoot 指定；已存在且非空则拒绝，绝不覆盖旧 run。
- -ProjectA 缺省为 <RunRoot>/workspace/ProjectA.vit。若显式给出，必须位于 RunRoot 内、扩展名 .vit、且运行前不存在；**禁止指向用户工程或仓库样例工程**。A/B/C 的全部 .vit、.vit_history、.vit_agent、.vit_worktrees 都落在 RunRoot 内。
- 建议 berth 在启动前设置 VIT_HISTORY_DRAFT_ROOT=<RunRoot>/drafts 与 VIT_PROJECT_XML=<RunRoot>/default_project.xml（若该版本支持），把 project.new 的进程 draft 与内核 boot XML 也纳入隔离。脚本会记录 project.new 的 draft 路径；若 draft 在 RunRoot 外只记 warning（产品既有行为），不冒充隔离。
- 不写源码树/用户工程：脚本自身只写 RunRoot；kernel 可能改写 VitApp/Workspace/default_project.xml，脚本启动前快照、finally 还原并哈希亲证（§7）。

### 2.4 recents / authority / kernel 日志

- recents：VIT_DAW_USER_CONFIG_DIR/global_project_config.json；未设置时 %APPDATA%/Vit-DAW/global_project_config.json。启动前快照 recents.before.json 并记 SHA256；finally 还原，recents.after_restore.json + 哈希一致才算通过。
- authority：VitApp/Workspace/Settings/Settings.xml、VitApp/Workspace/default_project.xml。启动前复制到 <RunRoot>/authority/*.before 并记哈希；finally 若被内核改写则还原，哈希一致才算通过。还原失败=环境中断，退出 2，不得静默。
- kernel 日志：默认 <RepoRoot>/VitApp/Workspace/Logs/VitHeadlessServer*.log 取最新；可用 -KernelLogPath 指定。启动前记录长度，之后只读增量；日志轮转/缺失=环境中断，退出 2。**这是 open_project 计数的唯一运行时证据**。

### 2.5 零 LLM

- 全流程只调 /agent/invoke（project.new/project.save_as/project.open/version.*）和只读 GET；不调 /agent/chat，不触发模型。
- 回执记录 agent_llm_telemetry.jsonl 不存在或 0 行（若栈开启该遥测）；不以“没有明显 LLM 回复”替代。

---

## 3 装置与参数

~~~
scripts/run_conv_cross_combo_smoke.ps1
~~~

| 参数 | 默认 | 说明 |
|---|---|---|
| -RepoRoot | D:/Vit_DAW | 候选仓库根；读 agent/internal/harness/* 做静态锚点、读 kernel 日志、快照 authority |
| -AgentBaseUrl | http://127.0.0.1:7878 | agent HTTP；**不是** VspHub 8787 |
| -ProjectA | 空 | 空=RunRoot/workspace/ProjectA.vit；显式值必须在 RunRoot 内且不存在 |
| -RuntimeOwner | 空 | 授权运行必填；填登记栈位 owner |
| -TimeoutSeconds | 180 | 单次 HTTP 超时（10..1800） |
| -RunRoot | 空 | 空=新时间戳目录；显式值必须新/空 |
| -KernelLogPath | 空 | 空=自动发现；日志不可读则 fail-honest |
| -StackAuthorized | 关 | 唯一运行开关；不带则 exit 3 |
| -DryRun | 关 | 零 HTTP 计划，exit 0；优先于授权检查 |

**退出码**：0 PASS（可覆盖断言全绿，§9 缺口照实登记）；1 断言/产品失败；2 环境/用法/契约失败（含 reload 计数不可证、recents/authority 还原失败）；3 未授权默认调用。

**装置自身不启进程、不 bind 端口**；只对已存在 agent 发 HTTP。summary.teardown 的 zero 范围是“装置零进程/零端口/只写 RunRoot”，真栈全零由 owner 停栈后核对。

---

## 4 步骤

### S0 preflight（只读 + 快照）

1. 建新 RunRoot、写 run_meta.json、head.txt、git_status.txt、script_sha256、PS/OS/进程快照。
2. 快照 recents 与 authority（§2.4）。
3. GET /health 断言 status=ok；GET /agent/runtime/status 断言 kernel.connected=true、checkout_blocked=false。
4. GET /agent/state 记录初始工程；若已是**非 draft、且不在 RunRoot** 的真实工程，拒绝运行（stack_not_idle），防止覆盖用户工作区。
5. 定位 kernel 日志、记录 offset；静态核对 conversation_open.go 的 withProjectReload 恰好一处、withProjectReload 内 "cmd": "open_project" 恰好一处、419 reload-failure 测试存在。
6. 失败分类：agent_unreachable/kernel_offline/checkout_blocked/kernel_log_unavailable/recents_path_unavailable/authority_snapshot_hash_mismatch，exit 2。

### S1 真实工程创建/保存（A）

~~~
POST /agent/invoke {tool:"project.new", args:{}, confirmed:true}
POST /agent/invoke {tool:"project.save_as", args:{file_path:"<RunRoot>/workspace/ProjectA.vit"}, confirmed:true}
~~~

断言：project_lifecycle=new、uuid 非空；save_as 返回 project_path==A、uuid 非空；A 文件存在；version.status {project_path:A} initialized=true（物化历史不回 available 键；只有 unmaterialized 态回 available=false）；/agent/state.current_project.path==A。记录 A uuid、draft 路径。

**S1 workspace 断言（2026-11-19 修订）**：**不再要求 project_workspace 非空或 warning 为空**。目标 workspace 可用性以 <project_dir>/.vit_agent/<uuid>/manifest.v2.json 为准：目录与 manifest 存在，manifest.schema_version=vit_agent_store_manifest.v2、project_uuid==A uuid、project_path==A；否则 fail project_save_workspace_unusable。非空 project_workspace_warning 记入 summary.warnings 与 summary.isolation.workspace_warning；文本含 "Access is denied" 归 windows_rename_transient（已知瞬态），不单独判失败。若 project_workspace 存在，则交叉核对 project_path/agent_store_dir（条件断言），但不以其存在为 PASS 条件。

**Windows 瞬态 warning（expected/recorded）**：projectstore.ForkAgentStore 在 save_as 时把 .vit_agent/.<uuid>.staging_* rename 到 .vit_agent/<uuid>，Windows 上可能返回 Access is denied；harness.applyProjectLifecycle 把该 error 写到 result.project_workspace_warning 并省略 result.project_workspace，但目标 agent store 可能已 materialize（manifest.v2.json 在位）。这不是本项目组合缝失败；脚本记录 warning 并以目标 manifest identity 判可用性。若 warning 非空且目标不可用，才 fail project_save_workspace_unusable。

### S2 第二工作树 + 目标分支/节点（零 LLM 形态）

1. version.worktree_create {name:"cross-b"}：断言返回 project_file_path 存在且位于 RunRoot、source_checkpoint_id 非空、无 source_checkpoint_warning；**本步零 open_project**（创建工作树不切换内核）。
2. version.status {project_path:B}：断言 initialized=true、worktrees>=2、active_branch=main、head 非空、branches.main==head、active_node_id 非空且该节点 commit_id==head。此节点是产品 WorktreeCreate 生成的 **worktree seed checkpoint**。
3. version.branch_create {project_path:B, name:"cross-combo", commit_id:seedCommit, from_node_id:seedNode}：断言 branch=cross-combo、commit_id=seed、refresh.kernel_reloaded=true；本步是**准备步骤**，允许且有且仅有一次 open_project B（脚本单独计数，不计入 S4 的“单次”）。
4. version.status B：断言 active_branch=cross-combo、head=seed、branches.cross-combo=seed、active_node_id 非空、节点 branch=cross-combo、commit_id=seed（branch_marker）。记录 BranchNodeId。
5. 等 /agent/ui/state.project.project_path==B 稳定（激活/绑定完成）。
6. project.open {file_path:A}：断言 status ok、/agent/state 回 A、等待 /agent/ui/state 回 A；本步允许一次 open_project A（脚本单独计数）。
7. 失败分类：worktree_create_failed/worktree_source_autosave_warning/worktree_b_path/branch_create_failed/branch_node_*/project_open_a_*，exit 1（产品/断言）或 2（A 文件/权限/环境）。

**为什么用 seed/branch_marker 节点**：零 LLM 不能发真实 /agent/chat turn；version.branch_create 的 from_node_id 是现有产品面生成真实 Project History 对话节点的确定性路径。目标分支不是 main，能真正走 branch checkout（非 detached）。这是本卡的**明确形态**，不是伪造：节点/分支/commit 都由产品 history 工具真实落盘。

### S3 跨工作树只读浏览（不切换）

~~~
POST /agent/invoke {tool:"version.status", args:{project_path:B}, confirmed:false}
~~~

断言：
- status=ok、result.project_path==B、initialized=true、active_branch=cross-combo、active_node_id==BranchNodeId、head=seed、branches.cross-combo=seed；
- B .vit SHA256 前后一致；A .vit SHA256 前后一致；
- /agent/state.current_project.path==A、/agent/ui/state.project.project_path==A；
- kernel 日志增量中 Command received ... "cmd":"open_project" **0 条**（等待 0.8s 后再读一次）。
失败分类：browse_wrote_b/browse_wrote_a/ui_project_after/unexpected_open_project，exit 1。

### S4 打开目标工作树并继续（单次 reload）

~~~
POST /agent/invoke {tool:"version.conversation_open",
  args:{project_file_path:B, branch:"cross-combo", node_id:BranchNodeId},
  confirmed:true}
~~~

断言 tool 回执：
- status=ok；project_file_path==B；worktree.name==cross-b、is_root=false、same_worktree=false；
- target_materialized=true、kernel_reloaded=true、refresh.kernel_reloaded=true、refresh.reason=="version_conversation_open"、refresh.shadow_refreshed=true；
- mode=="branch_checkout"、branch=="cross-combo"、detached=false、head==seed、target_commit_id==seed；
- active_branch=="cross-combo"、active_node_id==BranchNodeId、active_worktree=="cross-b"；branch_warning 为空；
- switch_autosave_commit_id 非空或 switch_autosave_warning 如实披露（有 warning 记 warning，不静默）。

断言运行时：
- kernel 日志增量中 ^Command received:.*"cmd":"open_project" **恰好 1 条**，且该行包含 B 的 JSON 转义路径；等待 1.5s 再读仍为 1（排除第二次 reload）；
- 轮询 /agent/state.current_project.path==B、/agent/ui/state.project.project_path==B（30s）；等待 1s 后重新 version.status B：active_branch=cross-combo、active_node_id==BranchNodeId、head=seed、active_worktree=cross-b 未漂移；
- DAW 投影一致：ui.project.project_path==B、ui.project.track_count==state.shadow.track_count、ui.project.graph_revision==state.shadow.graph_revision、ui.project.initialized=true；
- A .vit SHA256 未变。
失败分类：conversation_open_reload_count（0 或 >1）、conversation_open_active_node、post_open_node、daw_*、kernel_project_not_b，exit 1；kernel 日志不可读/轮转，exit 2。

### S5 失败语义

| 探针 | 工具实参 | 期望 |
|---|---|---|
| 未知工作树 | {name:"cross-missing-<hex>", node_id:BranchNodeId} | HTTP 400/409、status=error、error 含 worktree not found、target_materialized=false、kernel_reloaded=false、零 open_project、当前工程仍 B |
| 失效工作树文件 | 先 version.worktree_create {name:"cross-expired"} 得 C；version.status C 取节点；删除 C 的 .vit；再 {project_file_path:C, node_id:C节点} | error 含 worktree project file does not exist；其余同上 |
| 未知节点 | {project_file_path:B, node_id:"n_missing_<hex>"} | error 含 conversation node not found；其余同上 |
| 未知分支 | {project_file_path:B, branch:"missing-branch-<hex>", node_id:BranchNodeId} | error 含 branch not found；其余同上 |

每个探针都断言 target_materialized 字段**存在且为 false**（不是缺省当 false）、kernel_reloaded=false、零 open_project、/agent/state 仍 B。

**未覆盖的 reload 失败**：现有 agent HTTP/工具面无法在不竞态文件系统、不 kill kernel 的前提下，让 open_project 在目标物化后确定性失败。419 TestConversationOpenReloadFailureIsExplicit 用 fake kernel 覆盖该分支；**本卡不把它写成真栈 PASS**。target_materialized="unknown"（物化调用报错、可能部分写入）同样无法从公开面确定性注入，登记为缺口。

### S6 teardown

- finally 还原 recents、authority；计算 before/after SHA256，不一致=环境中断 exit 2。
- S6 断言：S6-recents-restored、S6-authority-restored、S6-apparatus-zero（脚本启动进程=0、bind 端口=0）、S6-runroot-only（A/B/C 全在 RunRoot）、S6-teardown-ok。
- 写 summary.json、teardown.json；打印 artifact 路径。
- **不 kill 栈**：kernel/agent 仍运行在 B。owner 在脚本结束后按既有流程停栈，独立核对 Vit 系进程=0、TCP 5555/5556/7878/8787=0、UDP 4445=0，并在 PC-RUNTIME-STACK.md 记录释放。脚本的“zero”只声明装置自身零进程/零端口，不冒充真栈全零。

---

## 5 断言清单（脚本动态生成，summary.assertions 逐条留档）

| 范围 | 关键断言 ID |
|---|---|
| S0 | S0-agent-health、S0-kernel-online、S0-no-active-turn、S0-stack-idle、S0-kernel-log、S0-static-reload-anchor、S0-recents-snapshot、S0-authority-snapshot |
| S1 | S1-project-new-ok/lifecycle/uuid、S1-project-save-ok/path/uuid/file-exists、S1-project-workspace-usable/identity（project_workspace 存在时另有 projection/agentstore 条件断言）、S1-history-path/initialized、S1-kernel-project-a、S1-kernel-uuid-a |
| S2 | S2-worktree-create-ok/name/path/under-runroot/file/source-checkpoint/source-warning/create-no-reload、S2-status-b-*、S2-branch-create-ok/name/commit/reload、S2-branch-open-project-count/target、S2-branch-active/head/head-field/node-*、S2-kernel-b-after-branch、S2-open-a-ok/count/target、S2-kernel-back-a、S2-back-to-a-ui |
| S3 | S3-a/b-file-present、S3-kernel-before、S3-ui-before、S3-status-b-path/initialized/branch/node/head/branch-head、S3-b/a-file-unchanged、S3-kernel-after、S3-ui-after、S3-no-open-project |
| S4 | S4-result/status/project-file-path/worktree-name/not-root/same-worktree-false、S4-target-materialized/kernel-reloaded/refresh-*/mode/branch/detached/head/target-commit/active-*、S4-branch-warning、S4-switch-autosave、S4-single-open-project/open-project-target、S4-kernel-project-b/ui-project-b/state-path-b/ui-path-b、S4-daw-track-count/graph-revision/initialized、S4-history-*、S4-a-file-unchanged |
| S5 | 每个探针 -http/-status/-result/-target-disclosed/-target-not-materialized/-kernel-not-reloaded/-error-text/-no-open-project/-kernel-current |
| S6 | S6-recents-restored/authority-restored/apparatus-zero/runroot-only/teardown-ok |

正式轮 PASS 需 S0–S6 全 true 且 summary.assertions_passed==assertion_count；summary.failure 为空。

---

## 6 失败分类、止损与重跑

| 分类 | 退出 | 例子 | 处置 |
|---|---|---|---|
| environment_failure | 2 | agent/kernel 不可达、kernel 日志缺失/轮转、recents/authority 还原失败、RunRoot 权限 | 保留原工件；环境修好后另起新 run；不得把下游断言失败归因产品 |
| usage | 2 | 缺 -RuntimeOwner、ProjectA 出 RunRoot、RunRoot 非空、栈上已有真实工程 | 修正参数/换 berth；不占正式轮 |
| contract_failure | 2 | 单 reload 计数不可证、静态锚点缺失 | 停手补证/开卡；不得伪造计数 |
| assertion_failure / product_failure | 1 | 节点/分支未 active、reload 0/2、DAW 投影不一致、失败语义泄漏 reload | 停、保留 summary/请求响应/kernel 日志增量；同断点连续 2 次失败不得原样重跑 |

- **单正式轮**：一轮里 S0–S6 顺序执行；失败即停。装置/期望修正后重跑须有新脚本 hash/提交并分记 attempt。
- **止损**：总时长建议 ≤60 分钟；同代码版本同确定性断点连续失败 2 次，停手取证上交；环境中断两次，释放栈位。
- 不得在看到偶然成功后倒推验收标准；一次 exit 0 只证明存在一条成功路径。

---

## 7 recents / authority / teardown 纪律

1. 启动任何 project.new/save/open/version.conversation_open 之前完成 recents 与 authority 快照；快照复制后立即复核哈希。
2. finally 无条件还原；recents 还原后写 recents.after_restore.json；authority 若被改写则复制回 .before；两者哈希与 before 一致才算 restored=true。
3. 任一还原失败：summary 置 failed + environment_failure，退出 2；**不得静默**。
4. 装置不启动/不停止栈；summary.teardown.zero_scope="apparatus-only"。owner 停栈后另行核对全零并在资源记录登记释放。
5. 每次运行新 RunRoot，不覆盖旧 run；工件中的关键结论必须能回指 requests/、responses/、summary.json、kernel 日志增量。

---

## 8 工件契约

每次正式轮 <RunRoot>/：

| 工件 | 内容 |
|---|---|
| run_meta.json | schema、card、owner、RepoRoot、AgentBaseUrl、script SHA256、PS/OS、git HEAD/status、VitAgent/VitApp/VspHub 进程路径/PID/SHA256、kernel 日志路径、开始时间 |
| head.txt / git_status.txt | 运行时 HEAD 与工作树状态 |
| requests/*.json | 每次 /agent/invoke 请求体（工具/args/confirmed/source） |
| responses/*.json | 对应原始响应（含失败 HTTP body） |
| summary.json | 状态、S0–S6、全部断言、reload 计数、A/B/C 路径/节点/分支、failure、warnings、coverage、teardown |
| teardown.json | 装置进程/端口 zero、recents/authority 还原、栈 owner/留运行说明 |
| recents.before.json / recents.after_restore.json | recents 快照与还原副本（文件原本不存在时写 recents.before.absent.json 标记） |
| authority/*.before | Settings.xml、default_project.xml 原始副本 |
| kernel-log-*.txt（如需） | 关键区间的 Command received 增量摘录（脚本 summary 记 offset/count；如人工另存须注明区间） |
| workspace/ | A/B/C 的 .vit、.vit_history、.vit_agent、.vit_worktrees（本地证据，默认不入库大二进制/blob） |

**入库口径**：只入 summary.json、teardown.json、requests/responses 中必要证据、runbook/报告；.vit_history/媒体/大二进制按 425/397 先例本地保留并记哈希，不把运行时工程状态混入代码提交。

---

## 9 覆盖边界与已知缺口（如实声明）

| 项 | 状态 |
|---|---|
| 真实栈创建/保存 A、创建 B、跨树 status 只读、conversation_open 路径/分支/节点/单次 reload | 本脚本 S1–S4 计划覆盖 |
| 未知/失效工作树、未知 node/branch 的失败语义与 target_materialized=false | S5 计划覆盖 |
| **reload 失败（物化后）** | **真栈不可确定性注入**；仅 419 fake-kernel 单测 TestConversationOpenReloadFailureIsExplicit 覆盖；脚本不声称真栈通过 |
| **target_materialized="unknown"** | 公开面不可确定性注入；未覆盖 |
| **Godot/VitShell 窗口像素/焦点** | 不驱动 GUI；只断言 /agent/ui/state 与 /agent/state.shadow 的 DAW 投影一致。UI 渲染/用户旅程由 421 零栈与后续手测覆盖，不能以本卡替代 |
| **真实人类 /agent/chat 对话节点** | 零 LLM 不发起 chat；目标节点为产品 worktree_create seed checkpoint + branch_create branch_marker。工具语义与 UI 421 的“分支 head 节点”一致；若主管要求真人对话节点，需允许一次 LLM 轮或新增 append-node 工具（另开卡） |
| **version.status 参数名** | 卡面写 project_file_path；实现/421 UI 用 project_path。脚本按实现；若要别名支持另开卡 |
| **Windows staging→target rename warning** | expected/recorded：projectstore.ForkAgentStore 的 staging→target rename 可能 Access is denied；save_as 仍成功并省略 project_workspace。脚本要求目标 .vit_agent/<uuid>/manifest.v2.json 可用并记 warning；不可用才 fail。残余：warning 本身不代表干净 fork，manifest 可用性是其唯一 workspace 证据 |
| **S1 2026-11-19 19:05 首轮失败** | 旧脚本要求 project_workspace 非空/无 warning，属装置期望过严；目标 workspace 实际可用。已按本修订改为 warning 记录 + manifest identity 断言，其余断言不放宽 |
| **稳定性/重复成功率** | 单正式轮 exit 0 只证一条成功路径；不声明稳定 |
| **用户手测** | 本卡不新增手测义务；真栈 PASS 后由主管决定是否放用户验证 421 跨树 UI |

---

## 10 授权状态与运行命令

**本 runbook 为装置草案；未获主管书面放行前禁止运行。** 零栈段可执行：

~~~
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_conv_cross_combo_smoke.ps1 -DryRun
# exit 0，打印计划，无 HTTP / 无端口 / 无文件写入
~~~

授权后（owner 已登记栈位、stack 已起、RunRoot 新空目录），单行执行：

~~~
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_conv_cross_combo_smoke.ps1 -RepoRoot D:/Vit_DAW -AgentBaseUrl http://127.0.0.1:7878 -RuntimeOwner "<registered owner>" -RunRoot "<fresh empty run dir>" -StackAuthorized
~~~

默认（无 -StackAuthorized）只打印 runbook 摘要并 exit 3；不会静默启动或复用栈。

---

## 11 attempt 台账模板

~~~
- attempt N（<RunRoot>，起止）= <PASS exit 0 | assertion_failure | product_failure | environment_failure | usage>
  - 候选/脚本：HEAD <hash>，script <sha256前12>，kernel/agent <sha256前12>
  - 关键证据：summary.failure=<class/code>; reload_count=<n>; recents=<before/after>; authority=<before/after>
  - 根因与变更：<装置/产品/环境；无变更不得原样重跑>
  - 是否正式轮：<是/否>
~~~


---

## 12 零栈自检记录（2026-11-19，装置编写轮）

- Windows PowerShell 5.1 Parser：ParseFile errors=0，exit 0；脚本 UTF-8 BOM 落盘（首字节 EF BB BF）。
- 无 -StackAuthorized：exit 3，打印 runbook 摘要；未创建 RunRoot、未发 HTTP、未占端口。
- -DryRun：exit 0，打印计划；未发 HTTP、未创建 RunRoot。
- 授权路径离线反例：-AgentBaseUrl http://127.0.0.1:1（无监听进程），exit 2，failure=environment_failure/agent_unreachable；recents/authority 快照还原 restored=true；S6-recents-restored/authority-restored/apparatus-zero/runroot-only/teardown-ok 5 条通过；RunRoot 自检后删除。该反例只证错误路径与 teardown，不证 S1–S5 产品语义。
- 静态锚点：conversation_open.go 的 withProjectReload 计数=1；withProjectReload 内 "cmd": "open_project" 计数=1；419 reload-failure 测试存在。
- 真栈 S0–S6 未执行；本轮无 VitApp/VitAgent/VspHub/Godot 进程启动，无 TCP/UDP 监听被本装置占用。

### 12.1 2026-11-19 19:05 第二轮 S1 证据与修订

- 真栈 run：D:/Vit_DAW/coord/runs/CONV-CROSS-COMBO-1/real-stack/run_20261119_1905。
- S0 PASS；S1 project.save_as status=ok、project_path/uuid 正确，但 result.project_workspace_warning="rename ...staging_... -> ...vitproj_5de94fa8e7af4283ad01ba5c74e18037: Access is denied."，project_workspace 缺席；旧脚本 S1-project-save-workspace 失败。
- 目标 .vit_agent/vitproj_5de94fa8e7af4283ad01ba5c74e18037/manifest.v2.json 实际存在，identity 正确，无 staging 残留；属已知瞬态 Windows rename warning（agent/internal/history/working_session_test.go:451 同型）。
- 修订：S1 不要求 project_workspace 非空/无 warning；warning 记入 summary.warnings 与 isolation.workspace_warning；以目标 manifest identity 断言 workspace 可用，不可用才 fail project_save_workspace_unusable；其余断言不放宽。
- 本轮修订只做 Parser/no-auth/-DryRun 自检，未启栈；真栈待主管第二轮。

---
（上游：coord/rulings/2026-11-18-CONV-OPEN-TOOL-1-pass.md、coord/rulings/2026-11-18-CONV-RAIL-CROSS-1-pass.md；卡：coord/cards/doing/2026-11-18-CONV-CROSS-COMBO-1.md；执行器：scripts/run_conv_cross_combo_smoke.ps1。）
