# LAUNCHER-ZEROFRICTION-1 取证报告（池序 352 · 取证腿）

- 日期：2026-11-14 晚窗；owner=freeflash（GLM-5.3 Flash）免费车道子代理 · PC
- 基线/分支：origin/main = `073b594f`；worktree `D:/Vit_DAW_wt_zerofric1` @ `port/launcher-zerofriction-1`（领取提交 `a682b2e6`）
- 性质：只读取证+方案文档，零生产改动、零真实栈。本文所有 file:line 均读自该 worktree（与 main 同内容）；348 真栈证据引 `coord/runs/UI-BRIDGE-1/` 原件。
- 结论速览：**boot scratch "Unsaved" 路径的物化者是 launcher recents 聊天轮内的对话节点落盘（`Open("")` 副作用），完成者是 chat 收尾/调度器的工作区激活+hint 发布；UI 治理判据（path 空则零摩擦）本身是设计稿 §3.3 在案设计，问题在判据的输入被只读查询污染。修复推荐 A1（agent 侧 launcher 卷门控）为主案。**

---

## 锚点 1：hint 物化链（recents list 聊天 → 工作区激活 → hint，全跳 file:line）

### 1.1 webui 发起侧（345 设计，加载即发）

| 跳 | 证据 |
|---|---|
| W1 挂载即发 | `agent/webui/src/launcher/LauncherSurface.tsx:309-317`：live 模式 mount effect 同时调 `loadGreeting(run)` 与 `void loadRecents(run)`（互不阻塞，无用户交互前置条件） |
| W2 list 查询 | `LauncherSurface.tsx:264-290` `loadRecents` → 268 行 `sendLauncherChat(conversationID, RECENTS_LIST_MESSAGE)`；`RECENTS_LIST_MESSAGE = "有哪些工程"`（106 行，15 字节 UTF-8——348 日志 `message_len=15` 与之吻合） |
| W3 HTTP 面 | `agent/webui/src/api/launcher.ts:110-123` `sendLauncherChat` → `POST /agent/chat`，请求体 `context: { launcher: true }`（114-118 行；非权限声明，§3.1 主门自报） |
| W4 会话 ID | `agent/webui/src/launcher/launcherData.ts:18-20` `createLauncherConversationID` → `webui_launcher_<ts36>`（348 日志 `conversation=webui_launcher_mwgdxrq5` 吻合） |

### 1.2 agent 接收侧（每跳均有代码锚点+348 运行时日志佐证）

| 跳 | 证据 |
|---|---|
| A1 入口 | `agent/internal/chat/server.go:2577` `handleChat`（`/agent/chat` 路由注册 `server.go:733`）。**启动器卷分支在 2906 行才路由——工作区副作用全部先于意图分类发生** |
| A2 激活尝试（boot 期 no-op） | `server.go:2620` `s.activateCurrentProjectWorkspace(r.Context())`——注意只传 HTTP ctx，**看不到 req.Context 里的 launcher 标记**。boot 无聊天时：`CurrentProjectIdentity`（`harness.go:568-582`：shadow path 空 + hint 空 → path=""）→ `server.go:7649-7651` uuid 非空继续 → 7662 recovery 已禁用（FIX-START-STATE-RECOVERY-1，2026-10-18 用户裁定）→ 7666-7668 回退 `history.CurrentDraftProjectPath()`=进程缓存空 → 7669-7673 **"activation pending" return，不设 hint**。运行时佐证：`coord/runs/UI-BRIDGE-1/run_20261114_203135/logs/agent_last.log:13-27` 连续 `[workspace] activation pending uuid=… reason=startup_recovery_disabled_new_session`（20:31:52-54，含 `[shadow] initialized tracks=11 project_path=""`——内核 boot 自带 uuid 无 path），独立探针 path="" 成立 |
| A3 **物化点（本缺陷根因）** | `server.go:2628` `projectPath := projectPathFromChatContext(req.Context)`=""（A2 阶段 context 注入 `server.go:2619/7621-7639` 写入的是空 path）→ `server.go:2658` `beginChatGoal` → `server.go:2680` `RecordConversationNodeForProject(r.Context(), "", "ask", …)` → `harness.go:677-707` `historyArgs`（uuid 在、path 空不注入，`harness.go:9203-9218`）→ `history.AppendConversationNode` → **`history.go:1020-1021` `Open(projectPath(args))` 空路径 → `history.go:259-269` `defaultDraftProjectPath()` → `history.go:2509-2529` `os.MkdirAll`+写 `Unsaved.vit` 占位（22 行 `draftProjectFileName="Unsaved.vit"`）并缓存 `draftProject.path`（2526-2527）**。目录名 `draft_<UTC>_<shortid>`：348 attempt1 实测 `draft_20261114T123154_f10e356e`（UTC 12:31:54=本地 20:31:54）与首条聊天同秒、与 boot 期"activation pending"相隔 2 秒——物化发生在聊天轮内而非进程启动。之后 1026-1027 因 draft 无 HEAD 报错走 `RecordConversationNodeForProjectWithData:711-740` Checkpoint 兜底再重试。**读路径全部已被 FIX-START-DRAFT-SIDE-EFFECT-1 封死（`history.go:324-349` `openRepoForRead` 只复用缓存；`harness.go:625-667` 摘要只读），唯一漏网写副作用是这条对话节点落盘经 `Open("")` 物化草稿** |
| A4 激活完成 | 两个完成点（348 日志同一秒内先后出现）：①chat 收尾 `server.go:2621` `defer s.syncCurrentProjectWorkspace` → `server.go:7863-7866` 再入 `activateCurrentProjectWorkspace`；②续跑调度器 tick `continuation_scheduler.go:935`。此时 7667 行 `CurrentDraftProjectPath()` 命中 A3 新缓存 → `7675 ActivateProjectStore(draft, uuid)` → `7743-7746 RecoverWorkingSession/EnsureWorkingSession` → `7774-7779` 绑定 `s.activeWorkspacePath=draft` → 运行时佐证 `run_20261114_203135/logs/agent_last.log:28`：`[workspace] activated project="…\agent_drafts\draft_20261114T123154_f10e356e\Unsaved.vit" uuid=vitproj_ce9e36c7… recovered=false`（其后 29-33 行即 `conversation=webui_launcher_mwgdxrq5 message_len=15` 的 recents 回执日志——激活发生在该聊天轮内） |
| A5 **hint 发布** | `server.go:7798` `s.harness.SetProjectPathHint(projectPath, projectUUID)`（注释：FIX-START-DRAFT-SIDE-EFFECT-1 把激活工作区发布为**读侧身份 hint**）→ `harness.go:543-551` 存 `projectHintPath/UUID` |
| A6 读侧回放 | `server.go:1054-1080` `/agent/state`（`handleState`）→ 1065 行 `CurrentProjectIdentity`（与激活**同源**，注释 1063-1064 "LAUNCHER-CHAT-BE-1 双面同源"）→ `harness.go:574-579` shadow path 空但 hint uuid 匹配 → 返回 hint path → 1076 行 `current_project.path=<boot scratch 草稿路径>`。348 报告实测 `agent_current_project_observed = …\agent_drafts\draft_20261114T130735_36fb61ab\Unsaved.vit`（`run_20261114_210725/artifacts/ui_bridge_smoke_report.json:35`） |
| A7 UI 判据消费 | Godot 前端（348 已合入 main `380166e`）`D:/Godot/project/vit-daw-frontend/app/startup/start_page.gd:598-612` `_launcher_governance_decision` → 623-635 `_launcher_current_project_state` 拉 `/agent/state` 取 `current_project.path` → **known && path=="" 才零摩擦 proceed；path 非空或态未知 → 三键确认**（保存并切换/直接切换/取消）→ 三键弹出示例 `run_20261114_210725/logs/godot_boot1.log:20` `[LauncherBridge] governance.dialog_shown {"current_project":"Unsaved"}` |

### 1.3 链式结论

345 设计的"完整 recents 列表经 list 意图确定性获取"（聊天面）本身只读（`launcherRecentProjectPaths` → kernel `get_recent_projects`，`launcher_flow.go:372-396`）；破坏零摩擦的不是查询，而是**聊天面固有的对话节点落盘副作用**（A3）+ **无差别工作区激活**（A2/A4）+ **读侧 hint 发布**（A5）三步叠加。设计意图与实现的对峙点：设计稿把 launcher 卷定义为进程内会话缓冲（`coord/reports/2026-11-09-ai-first-launcher-design-v1.md` §2.1；`launcher_flow.go:594-596` 注释"launcher 卷本身即进程内会话缓冲……随进程存活、重启即失"），而 A3 把 launcher 卷对话写进了 boot scratch 草稿的项目史。

## 锚点 2：治理判据面（337 判据 + 340 修复关系；boot scratch 为何算「有在开工程」）

### 2.1 两张判据面的分野（uuid 判据废弃史）

| 面 | 位置 | 判据 | 现状 |
|---|---|---|---|
| **BE 门**（走不走 launcher 确定性路由） | `launcher_flow.go:71-79` `launcherFlowApplicable` | `context.launcher=true`（UI 声明主门）**且** `!userEnteredProjectSnapshot()`（进程内 version.project_opened 会话事实副门，`server.go:1747` 附近到达即置位） | **uuid/path 已完全不参与**——340 修复（LAUNCHER-GATE-FIX-1）废弃 337 的 `uuid==""` 判据，依据 339 真栈双层证据：boot 恒载 default XML 自带 uuid + VSP 紧凑快照无条件占位 `project_current` 使 `CurrentProjectIdentity` 恒非空，旧判据结构性不可达（`launcher_flow.go:19-25/69-70` 注释；`coord/cards/done/2026-11-10-LAUNCHER-GATE-FIX-1.md:13`） |
| **UI 治理面**（要不要弹三键） | `start_page.gd:598-612` | `/agent/state current_project.path` **非空或态未知 → 三键**（保守宁多勿漏）；**空且 known → 零摩擦** | path 语义维持 337 时代原样——340 卡面明文"336 UI 的 path 自查（治理确认面）维持不动——两面语义差异在卡面注明（UI 管'要不要确认'，BE 管'走不走 launcher 路由'）" |

两面的合同字段同源：`/agent/state current_project` ← `CurrentProjectIdentity`（`server.go:1063-1065` 注释明文同源要求；340 卡"窗徽章对账两面一致"）。

### 2.2 boot scratch 为何被算「有在开工程」

1. **判据输入被聊天副作用污染**：内核 boot 的未保存工程在 shadow 里 path=""（`[shadow] initialized … project_path=""`），无聊天时 `/agent/state current_project.path=""` → 判据本应给零摩擦。但 recents 聊天一到位，锚点 1 的 A3→A5 链把 `current_project.path` 变成 boot scratch 草稿路径 → UI 判据按字面语义判「有在开工程」。
2. **判据自身是保守设计（有意，在案）**：设计稿 §3.3（`coord/reports/2026-11-09-ai-first-launcher-design-v1.md:123-126`）："触发=任何 open/create directive 到达时若已有工程在开 → 治理确认；**启动台→首开（无在开工程）不确认（零摩擦，展演主路径）**；dirty 检测展演版保守一律确认（宁多勿漏）"。Godot 实现把「已有工程在开」操作化为「path 非空」，且 340 时代的驱动注释已预记"真栈 boot path=草稿回退恒非空 → 真栈恒走确认卡"（`coord/cards/done/2026-11-10-LAUNCHER-GATE-FIX-1.md:40`）——即**「boot 后只要聊过天就必弹三键」在 340 时已是已知语义差异**，但当时按卡面约定不动 UI 面，留给本卡取证裁定。
3. **未决层**：boot scratch（内核默认工程，无用户工作）是否算「在开工程」，设计稿 §3.3 没有显式定义；348 终验裁定①明确把该交互定性为缺陷并开本卡（`coord/rulings/2026-11-14-UI-BRIDGE-1-pass.md:15-17`："缺陷不在桥链……在 hint 物化/治理判据与 launcher 语义的交互"）。

### 2.3 停止条件核查结论

**部分触发**：UI 治理判据本身（path 保守确认）属设计稿 §3.3 在案设计，受在案文档保护——本报告不提案削弱/改写该判据（A2 变体涉及判据语义，仅列为候裁定备选，不推荐）。修复方向收敛为**让判据在"无真实在开工程"场景读到干净的空 path**（即修物化侧），与 §3.3 零摩擦条款完全兼容。

## 锚点 3：可区分性（list 意图响应路径能否区分 launcher-scope 只读查询 vs 真实 open 意图）

**结论：可区分的数据全部在卷，但现有执行顺序使区分不生效；无需新增协议字段即可修复。**

现有可区分字段（服务端在 chat 请求解码时即全部可得）：

1. **context.launcher 标记**：`api/launcher.ts:114-118` 发出；`launcher_flow.go:36/61-63`（`launcherContextKey="launcher"`，`launcherContextActive`）定义消费面。请求体在 `server.go:2591` 解码，**早于 2620 激活**——激活当前不读它（只传 `r.Context()`），改传 req.Context 即可门控。
2. **会话 ID 形态**：`webui_launcher_<ts36>` 前缀（`launcherData.ts:18-20`），服务端 `server.go:2602` 可见；`launcher_flow.go:608-609` `launcherFlowConversations`（628-636 `rememberLauncherFlowConversation`）已有按会话记录 launcher 卷的包级机制——但当前在意图命中后才记录（`launcher_flow.go:460`）。
3. **确定性意图分类**：`launcherClassifyMessage`（`launcher_flow.go:144-168`）纯函数零 LLM；`launcherMessageWantsProjectList`（197-213）词面表含"有哪些工程"；list 意图在 `handleLauncherFlowChat:483-494` 分支处理，回执只读（kernel get_recent_projects）。open 意图同函数 495-543 出 directive——**指令本身不带执行权（`requires_governance` 恒 true，`launcher_flow.go:52-58`），执行经 Vit 桥由宿主完成（LauncherSurface.tsx:601-609 execute 按钮 → `executeDirective` → `postLauncherDirective`，卡片点选 578-584 同样只把 directive 挂线程、执行走桥）**。
4. **意图可区分但副作用不可区分的时序根因**：`handleChat` 的三段副作用（2620 激活 / 2621 收尾同步 / 2680+2720 对话节点落盘）全部在 2906 launcher 分支之前、且不消费 launcher 标记。

区分粒度建议（供方案 A 引用）：launcher-scope（`context.launcher=true` 且进程内无绑定工作区）下的**全部**确定性意图（list/open/new）都不应物化工作区——因为 directive 只呈报不执行、执行经桥不经 agent，聊天轮没有产生任何需要工作区的工作；仅当该轮真正触发工具写（LLM 回落轮内工具执行走 `EnsureProjectPathForWrite` 显式写意图绑定）才物化，语义自洽。

## 锚点 4：两案矩阵（候裁定，不实施）

前置事实（约束两案可行域）：
- 全量 recents **无既有非聊天只读端点**：路由表（`server.go:717-746`）中 greeting（734）只回执首项+计数（`launcher_flow.go:555-576`，无工作区副作用、已验证零激活）；`/agent/state`（730）无列表；kernel `get_recent_projects` 只能经 `/agent/chat`（launcher list 意图）或 `/agent/invoke`（handleInvoke **同样有激活副作用**，`server.go:1754`）触达。
- 演示主路径（recents 抽屉点卡 → 执行按钮）在 348 后**不经聊天**：卡片点选只把 directive 挂线程（`LauncherSurface.tsx:578-584`），执行经桥（608）。**经聊天的是：加载即发的 list 查询（W1-W3）与用户打字输入（`send` → 347 行 `sendLauncherChat`）**。

### 案 A（agent 侧）：launcher-scope 只读查询不物化工作区/hint

**A1（推荐）：launcher 卷门控——scope 内无绑定工作区的聊天不激活、不落盘对话节点**

- 改动面（均在实施腿候裁定后才动）：`server.go` handleChat 三处门控——①2620 激活：`context.launcher=true && s.activeWorkspaceUUID==""`（或 `CurrentProjectIdentity` path==""）时跳过；②2621 defer 同步：同一门控；③2680/2720 对话节点落盘：launcher-scope 且 path=="" 时跳过（或改为内存卷记录，与 §2.1 设计对齐）。判定用现成字段（锚点 3），零协议新增。单测域 `agent/internal/chat/`（launcher_flow_test.go 同族）。
- 语义代价：launcher 卷会话不再写入 boot scratch 项目史——**这不是丢失而是设计对齐**（设计稿 §2.1：launcher 卷=进程内会话缓冲，随进程存活重启即失，`launcher_flow.go:594-596` 注释同口径）。已绑定工作区（工程卷）行为零变化；boot scratch 在用户真实做工具写时仍会按写意图路径物化（语义正确：那时确实有工作）。
- 风险：中低。热路径改动需防误伤——门控条件必须含"无绑定工作区"，工程卷聊天零感知；残留面=非 launcher 卷的无工程聊天（浏览器 dev 直聊）仍会物化草稿（存量行为，越本卡域，留裁定备忘）；`activateCurrentProjectWorkspace` 其余调用点（invoke/ui_state/interaction/confirm/scheduler）在 launcher 纯浏览窗内不被触发，scheduler tick 因草稿缓存永不满而保持 no-op。
- 对 345 现有行为影响：list 意图聊天路径原样保留（345 设计不变）；recents 列表照常经聊天确定性获取；打字「打开XX」路径同样修复（聊天只出 directive 不物化）。
- 验收门槛：真栈 boot1（无 auto_open）复测——React launcher 加载 → recents 列表正常 → `/agent/state current_project.path==""`（开前）→ directive 治理=none → open 成功；`ui_bridge_smoke.ps1` boot1 腿恢复零摩擦期望（348 期间 L1 改走的 switch 治理路径回归原设计；boot2/3/4 不变）；另断言 `agent_drafts` 下无新 draft 目录（物化反面证据）。渲染面/旅程层级按 AGENTS §5 边界声明执行。

**A2（备选，候裁定）：boot scratch 不算在开工程（判据放宽侧）**

- A2a（agent 读侧）：`/agent/state`（与 `/agent/runtime/status` 同源面）对 `IsDraftProjectPath(activeWorkspacePath)`（`history.go:238-245` 已有判别函数）的当前工程报 `path=""`（或增设 `draft:true` 披露字段）。改动面小（handleState+status 两处），但：弯折 FIX-START-DRAFT-SIDE-EFFECT-1 的读侧 hint 语义（"发布激活工作区为读侧身份"），窗徽章/桥播（`launcherBridgeSummaryReply`，`launcher_flow.go:708`）等 path 消费者需逐一核对；且**触碰 §3.3 判据的输入语义**，属 2.3 边界内的裁定项。
- A2b（Godot 判据放宽）：start_page 治理判据排除草稿形态路径——需跨仓（前端仓）改判据本体，且字符串硬编码 "Unsaved.vit" 触碰易变标识纪律；**不推荐**。
- 共同风险：与 §3.3「宁多勿漏」保守条款的张力需主管显式裁定；boot scratch 分类无在案定义（2.2③）。

### 案 B（webui 侧）

**B1（可行，推荐为 A1 的可选补充而非替代）：recents 获取改非聊天面——greeting 扩展全量列表**

- 改动面：`launcher_flow.go` handleLauncherGreeting 增全量 `recent_projects` 回执字段（复用 `launcherRecentProjectPaths`+`launcherRecentEntries`，只读面零激活——与 chat list 意图同数据源同上限 16）；`api/launcher.ts` 类型+方法；`LauncherSurface.tsx` loadRecents 改走 greeting。后向兼容（旧客户端忽略新字段）。
- 代价/风险：低-中。345 的「完整列表经 list 意图获取」设计语义被改写（改经 greeting）——需主管确认 345 语义变更可接受；greeting 单响应兼做问候+列表，少一次聊天。
- 对 345 影响：列表获取面换道；**聊天面仍保留**（打字路径不变）。
- 验收门槛：同 A1 真栈链 + 渲染面断言（抽屉列表从 greeting 回执渲染）。
- **单独充分的边界**：对演示主路径（点卡→桥执行）充分——全程零聊天零物化；但**对打字路径不充分**：用户打「打开XX」仍经聊天 → A3 物化链照旧 → 三键弹窗（虽然 §3.3 可辩解为启动台残留会话确认，但呈现的"Unsaved"仍是污染产物）。

**B2（不推荐单用）：延迟首次 list 查询到用户交互**

- 改动面最小（`LauncherSurface.tsx` mount effect → 抽屉展开/交互触发）。
- **单独不充分**：只推迟触发不消除触发——首次交互即物化（A3 链不变）；且 directive 执行虽经桥，但用户任何打字聊天仍物化。仅当产品接受「recents 抽屉按需加载+打字路径弹三键」时可作为缓解，不满足卡面零摩擦目标。
- 对 345 影响：首屏 recents 延迟可见（交互前 loading 态）——展演观感受损。

### 推荐

**A1 为主案**（可叠加 B1 为优化项）。理由：
1. A1 是根因修复：把「只读查询污染身份读侧」这一缺陷类一次清除，list 查询、打字 directive、未来任何 launcher 卷聊天面全部免疫；B1 只改一条获取道，打字路径残留。
2. A1 与既有在案设计零冲突：不触碰 §3.3 判据（2.3 边界）、不触碰 340 双门语义（launcher 门仍以 UI 声明+user_entered_project 为准，只调副作用）、与 launcher 卷进程内存设计（§2.1）对齐而非背离；FIX-START-DRAFT-SIDE-EFFECT-1 的"读不物化"精神向聊天前导段补全。
3. 单仓单域（agent/internal/chat + 测试），不跨仓协调；B1 虽低险但改 345 语义需额外裁定一次。
4. A1 落地后 348 烟测 boot1 腿回归原设计零摩擦预期，341 撤销后的用户手测 v3（React launcher 路径）体验与设计稿一致。

实施腿约束（候裁定后另领）：与 doing 卡 271（chat 路由）文件域重叠，排 271 收口后（卡面既有约定）。

## 附：证据工件索引

- 348 真栈：`coord/runs/UI-BRIDGE-1/run_20261114_203135/logs/agent_last.log`（attempt1 boot1 全时间线：activation pending ×N → 聊天 → activated Unsaved.vit 同秒）；`run_20261114_210725/artifacts/ui_bridge_smoke_report.json:28-35`（终验 finding：`zero_friction_dialog_current_project="Unsaved"`、`agent_current_project_observed=…Unsaved.vit`）；`run_20261114_210725/logs/godot_boot1.log:20`（三键弹出留痕）。
- 在案文档：`coord/rulings/2026-11-14-UI-BRIDGE-1-pass.md`（转裁定①开本卡）；`coord/cards/done/2026-11-10-LAUNCHER-GATE-FIX-1.md`（340：uuid 判据废弃+两面语义差注释）；`coord/reports/2026-11-09-ai-first-launcher-design-v1.md` §2.1/§3.3（launcher 卷内存语义+治理保守版）；`coord/cards/done/2026-11-10-RECENTS-HYGIENE-1.md`（recents 污染面背景）。
- 本卡工件：本目录 PROGRESS.md（步骤账）。
