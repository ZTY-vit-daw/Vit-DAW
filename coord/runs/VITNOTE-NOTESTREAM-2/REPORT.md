# VITNOTE-NOTESTREAM-2 执行侧回执报告（2026-10-03，PC 执行侧）

> 卡：每 note 独立会话流——面板自含问答历史+主对话流去 note 往返。
> 分支：Vit_DAW `port/vitnote-notestream-2` @ **618d69f9**（agent+webui 腿，已推 origin）；Godot `port/vitnote-notestream-2` @ **4391c49**（自 `port/vitnote-container-4@a2930a7` 切出，已推 origin）。
> 领取基线：Vit_DAW origin/main=aa3fe836（97badf16 领取提交）；Godot a2930a7。
> 全程未起真栈（用户指令）；回归=Go 全量+chat 域+webui vitest+Godot headless 探针。

## 一、取证结论（目标 1，详文回写卡面）

1. `chat_*` 会话 id：`server.go handleChat` 空 id → `chat_+randomID()`，与 note_id 零关联。
2. webui 主流终局消息水合不按会话 id 过滤（`historyMessagesFromUIState` ← `project_history.conversation_messages`，工程单图）。
3. note 问答入主流=**双重同向**：①`RecordConversationNodeForProject(projectPath,"ask"/"vit")` 按工程单图落盘（与 conversation_id 无关）；②webui 主流从该图聚合水合。
4. 误入治理链：`beginChatGoal` 把每条消息变 goal；面板 context 复用主控台组包 → contract_scope 取选中轨（三号场 1007）；圈选 faces 根本不在载荷。
5. 侧边栏未命名：note 的 chat_* 流经 durableContinuations 投影冒出，title 空 → 「未命名会话」。

## 二、实现面（六目标落点）

- **目标 2（agent）**：`/agent/chat` 载荷带 `note` 对象即早分叉（`server.go` handleChat，位于 pending 确认/beginChatGoal/单图落盘之前）→ `note_sessions.go handleNoteChatTurn`：会话 id=`note_<session_key>`（Godot 运行戳+序号，跨前端重启不撞名；空回落 note_id）；直答 LLM（观察语义系统段+辖区快照注入，**不进 D1 治理链**=2026-10-03 裁定原则）；不写工程单图/不建 durable continuation/不 emitTurnEvent/不 `s.remember`。持久化 `.vit_derived/<uuid>/note_sessions.json`（fail-open：缺文件/损坏 JSON/异形行→空表，原子写 tmp+rename，AGENTS §11）。
- **目标 3（Godot）**：面板自含历史沿用（`_msg_list` 随面板实例，收起/重开不清史——探针 D 面）；note 删除=`remove_note`→fire-and-forget POST `/agent/note/archive`（幂等归档，可查不入主流）。
- **目标 6（命名）**：默认名=裁定 1 模板「便签 N3 · 轨道时间线 72% · 机架 28%」（前两面降序/空辖区版，manager `default_session_title` 纯函数，与图钉 tooltip/便签列同源同文）；agent 会话库建档存名 → `/agent/runtime/status` 新增 `note_sessions` 投影 → webui 侧边栏合并行（hint title 兜底、本地改名权威、`renameSessionFlowEntry` 缺行补建使 serverKnown 行可改名、归档行入归档组）；侧边栏点击 note 流 → `noteSessionSeedMessages` 从投影回放问答对（可查）。
- **退役**：主控台组包复用链（panel `_collect_agent_context`+manager `get_agent_context`/`_find_chat_controller`）整体移除——三号场 contract_scope 误取根因（探针 C 面断言已退役）。

## 三、判据 1-4 证据

**判据 1（note 问一圈→webui 主对话流零新增）**——机制级三证（真栈级归用户手测）：
1. 分叉位置：note 分支位于一切 `RecordConversationNodeForProject` 调用点之前且不落入（`server.go` handleChat note 早分叉——工程单图零写入=webui 主流水合源不变）；
2. Go 钉 `TestHandleChatNoteTurnRoutesToIndependentSession`：note 回合后 `server.conversations`（主对话记忆）**零条目**；响应无 GoalID/GoalStatus（未建 goal）；
3. webui 主流消息源（`historyMessagesFromUIState`）不消费 note_sessions 投影（投影仅侧边栏合并+切换种子）。

**判据 2（note 重开→历史还在）**：面板实例收起=隐藏不释放（探针 D 面：collapse→reopen 消息数不清零）；服务端会话库续档（Go 钉：第二轮同键追加至 4 条消息）。

**判据 3（主任务上下文不被污染，遥测对照）**：note 组装（`buildNoteAssembly`）只吃 note 会话库历史（Go 钉断言前两轮问答在组装内、主对话记忆不在）；主 goal 组装源=工程单图+主对话记忆——note 模式两源均零写入（判据 1 证 2+机制）；遥测分段名 `vitnote_*` 前缀与主任务组包分段（chat_system 等）可区分（`section_stats.by_kind/section_count` 前后对照：note 回合前后主对话 `server.conversations` 均空=零注入）。

**判据 4（目标 6 命名落侧边栏）**：vitest 钉×3——`noteSessionServerHints` 携带默认名+归档态；`sessionFlowRows` hint title 兜底（**不显「未命名会话」**）+本地命名权威；`renameSessionFlowEntry` 缺行补建（serverKnown note 行改名可落）；`noteSessionSeedMessages` 切换回放。

## 四、回归结果（命令+退出码）

| 面 | 命令（worktree 内） | 结果 |
|---|---|---|
| Go 全量 | `go test ./... -count=1` | **exit 0，0 FAIL**（go_test_full.txt） |
| chat 域 | `go test ./internal/chat/ -count=1` | ok 86.8s |
| note 新钉 | `go test ./internal/chat/ -run 'TestNote|TestHandleChatNote|TestHandleNote' -count=1 -v` | **9/9 PASS**（go_note_tests.txt） |
| webui | `npm run test` | **425/425（36 文件）PASS**（webui_test.txt） |
| webui 类型/构建 | `npx tsc --noEmit` / `npm run build` | 0 错 / built in 7.59s |
| Godot 探针 | `probe_vitnote_{notestream,container,circle_state,face_resolve,panel_summary}.gd` headless | **21 / 141 / 52 / 127 / 16 全 0 FAIL，exit 0**（同目录 .txt） |

## 五、边界声明（AGENTS §5 渲染面/旅程门槛）

- **全程未起真栈**（用户指令）：7878 真栈问答往返、webui 浏览器级渲染、note E2E 用户旅程未覆盖——归卡面「真栈手测（与用户约定复验）」，本卡 `[等待真栈手测复验]`。
- webui 侧边栏改动无浏览器级 DOM 断言（E2E-WEBUI-1 建设卡未就位前如实声明）；覆盖=纯函数 vitest+机制级 Go 钉。
- note 会话库在 agent 无活动工程身份（projectPath/UUID 空）时降级为进程内不落盘（fail-open，仍可问答；边界如实申报）。
- 旧 note 问答路径彻底移除（非并存开关）：Godot 前端与 agent 需同版本配套（两分支同推，验收合并顺序建议 Godot 先/同批）。

## 六、工件清单（coord/runs/VITNOTE-NOTESTREAM-2/）

probe_vitnote_{notestream,container,circle_state,face_resolve,panel_summary}.txt；go_test_full.txt；go_note_tests.txt；webui_test.txt；本报告。
