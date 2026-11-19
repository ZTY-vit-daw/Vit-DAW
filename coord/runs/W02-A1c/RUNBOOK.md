# W02-A1c 真栈旅程 RUNBOOK（S0–S6）

- 卡：`coord/cards/doing/2026-11-19-W02-A1-LESSON-CTX-CLIP-1.md` 的 A1c 领取段（池序 432）。
- owner：`w02real`（DeepSeek-V4.1-Flash，PC，独立 worktree）。
- 分支：`codex/w02-a1c-real-432`；基线 main `9318717b`；worktree `C:/Users/timoz/.codex/worktrees/pc432-real/Vit_DAW`。
- 本轮范围：**只写零栈 runbook + 真栈执行器**，不启栈、不改产品代码、不改设计源/卡片/其他脚本。
- 交付：本文件 + [`scripts/run_w02_a1c_smoke.ps1`](../../../scripts/run_w02_a1c_smoke.ps1)。
- 状态：**真栈轮未运行（候 lead 登记栈位并授权）**；零栈验收已在本 worktree 跑完（见 §8）。
- 2026-11-19 首轮后修：S4 主取证改为 `agent_prompt_render.jsonl`（S0 路径/偏移、S4 新增行解析）；已知当前诊断只记 system 消息，`lesson_context` 实际在 user 模型快照（§6 G4）。

## 0. 一句话结论

执行器已就绪：在没有 `-StackAuthorized` 时只打印 runbook 摘要并 `exit 3`，`-DryRun` 只打印计划并 `exit 0`；授权后对**已存在的**真实栈走 S0–S6，最多消耗**一次真实 LLM 正式用户回合**（S4），所有失败按类退出，不以首个 clip、模板回复或缺失取证冒充 PASS。

## 1. 前置与硬边界

### 1.1 栈与授权

1. lead 先在 `coord/resources/PC-RUNTIME-STACK.md` 登记本轮唯一 owner、berth、RunRoot、单次正式轮和止损；未登记不运行。
2. 本脚本**不启动、不重启、不自行拉起**栈：零栈模式（默认/`-DryRun`）不接触任何栈；授权模式只访问 lead 已启动的 `http://127.0.0.1:7878`。脚本不调用 `Start-Process`，不监听端口。
3. 推荐 berth：`agent/bin/VitAgent.exe` + `VIT_AGENT_SPAWN_RUNTIME=1`（agent 自拉 headless kernel/VspHub，无 VitShell/GUI），或 lead 已登记的其他真实三件套。**GUI 栈不推荐**：真实 UI 的 `piano_roll_focus_*` 选择会污染 S5a 的“无选中”控制（脚本已把 S3/S4/S5b 的选中键显式写满，但 S5a 无法清除缓存 UI 选择）。
4. 栈必须空闲（无 active turn）、当前工程无未保存改动；脚本在 S0 检查并拒绝 busy/dirty 栈（exit 2）。
5. 被测 agent 二进制必须包含 A1a/A1b 改动（`edu_jam_chat.go` 选中 clip 绑定、`contextruntime` 的 `lesson_context`）。执行前由 lead 记录 HEAD/二进制哈希；`-SkipBuild` 类旧二进制不得归因给当前工作树。
6. 真实 LLM 必须已配置可用；S4 只跑一次。执行前把成功条件/失败分类/止损冻结在本 runbook（§5–§7），不得看到偶然成功后倒推标准。
7. 同一时刻只有一个执行会话拥有该栈；其他流不得并行启栈/占 7878。
8. **env 继承**：脚本进程必须与栈在同一 shell/env 下运行。若栈设置了 `VIT_DAW_USER_CONFIG_DIR`、`VIT_CONTEXT_SNAPSHOT_PATH`、`VIT_AGENT_PROMPT_RENDER_PATH`、`VIT_AGENT_LLM_TELEMETRY_PATH`，脚本必须继承同名变量；否则 recents 快照/还原和 S4 取证会指向错误路径（S4 按 exit 5 fail-honest，recents 还原可能不完整，必须由 owner 复核）。

### 1.2 文件写域

- 脚本自身的运行工件**只写 `-RunRoot`**：请求/响应、快照、证据、工程 A、项目包和 `summary.json`。
- 唯一例外是**还原**：recents（`%APPDATA%\Vit-DAW\global_project_config.json` 或 `VIT_DAW_USER_CONFIG_DIR` 同名文件）、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml` 在 S0 复制快照、teardown 按字节还原并哈希亲证；`/agent/authority` 同样快照/还原。
- 运行中的栈会自行追加 `VitApp/Workspace/Logs/*`（agent/kernel 日志）；这是栈的写域，不是脚本的写域。脚本只读偏移并保存新增片段。
- **不写**产品代码、设计源、卡片、其他脚本、sealed/holdout fixture、用户真实工程。
- `process/TCP/UDP 全零` 是 berth owner 在**停栈后**的独立检查；本脚本从不拥有/停止栈进程，不能替代该检查（`summary.teardown.process_tcp_udp_note` 已写明）。

### 1.3 运行前检查清单（lead/执行侧逐项确认）

- [ ] 栈位已登记；用户/主管授权本轮真栈；单 owner。
- [ ] 被测 agent 含 A1a/A1b；HEAD、二进制哈希已记录。
- [ ] LLM 可用；S4 单次正式轮预算已确认；失败分类/止损已冻结。
- [ ] 栈空闲；当前工程干净（无 unsaved/draft）；无 GUI 选中（建议 headless）。
- [ ] `RepoRoot` 是**实际栈 checkout**，含 `coord/lesson-designs/music-creation-unit2/design.v1.yaml`、`VitApp/Workspace/Logs`、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`。
- [ ] `-RunRoot` 是新目录（每次 run 新根，不覆盖旧 run）。
- [ ] 已知 API 缺口（§6）已读；不把 S3 的绑定级证据写成“notes judge 判定通过”。

## 2. 参数

| 参数 | 默认 | 说明 |
|---|---|---|
| `-RepoRoot` | `D:/Vit_DAW` | 实际栈 checkout；设计源、日志、Settings/default_project 的根。 |
| `-AgentBaseUrl` | `http://127.0.0.1:7878` | 已运行 agent HTTP 面；授权模式只访问此地址。 |
| `-RuntimeOwner` | 空 | 授权模式必填；与 `PC-RUNTIME-STACK.md` 登记一致。 |
| `-TimeoutSeconds` | `180` | HTTP 超时；S4 建议 `240`。 |
| `-RunRoot` | 空 | 空=`%TEMP%\vit_w02_a1c\run_<stamp>`；也可指向 `coord/runs/W02-A1c/real-stack/run_<stamp>`（工件不提交）。 |
| `-StackAuthorized` | 关 | 授权模式；缺省只打印 runbook 摘要并 exit 3。 |
| `-DryRun` | 关 | 只打印计划；不 HTTP、不写 RunRoot、不占端口；exit 0。`-DryRun` 优先于未授权门。 |

## 3. 退出码与失败分类

| 退出码 | 类别 | 含义 |
|---|---|---|
| 0 | PASS | S0–S6 全部断言通过，teardown 还原完成；唯一一次 LLM 正式用户回合成功且取证完整。 |
| 1 | `assert` | 确定性产品/契约断言失败（stop_reason、clip_scope、clip_ids、节点推进等）。 |
| 2 | `preflight`/`http`/`teardown` | 栈不可达、busy/dirty、HTTP 失败、快照/还原失败、teardown 时仍有 active turn。按环境中断处理，不归因模型。 |
| 3 | 未授权 | 默认无 `-StackAuthorized`；只打印摘要，零 HTTP、零写盘。 |
| 4 | `llm` | S4 真实 LLM 类失败：超时、空回复、`stop_reason` 非 `done`。telemetry error 记录并披露，不单独推翻 done+reply+lesson_context 三件套。 |
| 5 | `evidence`/`api`/`s6` | 取证/API 覆盖缺口：prompt render 新行无 lesson_context、编译卡形态不支持、S6 因 `opened_at` 已置而无法在不花第二轮 LLM 的情况下取证等。**不伪 PASS**。 |

> 选择 `select_clip` 失败不单独致命：它是 UI-only 动作（§6 G1），脚本记录错误并继续用 `/agent/chat` context 绑定；S3 的 `clip_scope=selected`/`clip_ids` 才是选中生效的判据。

## 4. 场景与断言（S0–S6）

### S0 preflight（只读）

- `GET /health`：`status=ok`、`service=VitAgent`。
- `GET /agent/runtime/status`：`status=ok`、`kernel.connected=true`、`kernel.reply_status=ok`、`checkout_blocked=false`；`goal.status` 必须属于非活动集 `{idle, completed, failed, cancelled, stopped, stable}`（与 `authorityModeChangeBlocked` 同集），否则拒绝运行。
- `GET /agent/state`：`status=ok`；若 `current_project.path` 非空且 `project_history/shadow` 的 `unsaved` 或 `draft` 为真，拒绝替换（exit 2）。
- `GET /agent/authority`：记录 `authority_mode`，teardown 还原。
- 复制 recents / `Settings.xml` / `default_project.xml` 快照并记录 SHA256；不存在的文件记录 `existed=false`，teardown 还原为不存在。
- 记录日志偏移：最新 `VitHeadlessServer*.log`、`agent_prompt_render.jsonl`（S4 主取证；`VIT_AGENT_PROMPT_RENDER_PATH` 或默认 `<RepoRoot>/VitApp/Workspace/Logs/agent_prompt_render.jsonl`，记录路径+字节偏移）、`agent_context_snapshots.jsonl`（仅可选 fallback）、`agent_llm_telemetry.jsonl`。
- 证据：`responses/*s0*.json`、`snapshots/*.before`、`summary.preflight`。

### S1 隔离工程 + W01 导入 + 开局

1. `POST /agent/invoke {tool:"project.new", args:{}, confirmed:true}`；再 `project.save_as {file_path:<RunRoot>/W02-A1c-A-<stamp>.vit}`。
2. 轮询 `/agent/state` 直到 `current_project.path == A`；断言 `.vit` 文件已写出、工程 UUID 非空。
3. 同一会话发 W01 命令（**原样**）：
   `导入乐局设计源 <RepoRoot>/coord/lesson-designs/music-creation-unit2 tempo=80 turns=60 calls=120`
   断言 `stop_reason=edu_jam_legame_imported`、`workflow_data.card_id` 非空。
4. 定位项目包（`.vit_project` 或 `<base>.vit_project`）与编译卡 `legame/cards/<id>/<id>.compiled.level.yaml`、编译 manifest；断言 `workflow_data.card`/`binding` 对应文件存在，binding 为绝对路径。
5. `开始乐局`（同会话）：断言 `stop_reason=edu_jam_entered`、`workflow_data.level_id == card_id`、`workflow_data.current_node` 非空。
6. S1.5：从编译卡解析 `runtime_slots` 与 `nodes[].done_when`。当前节点若只有 `event` 叶，先发 `乐局事件 <event>`（断言 `edu_jam_event_recorded`），再发 `写好了`（断言 `edu_jam_node_done`，且 `node_done("旧节点")`、`current_node` 已前进）；循环直到首个带 `clip` 或 `track` 的判定节点，得到目标槽与 `runtime_slots[slot].track_index`。
7. 证据：`responses/*s1*.json`、`summary.s1.{import,enter,advance}`、`summary.compiled_card/binding_path`。

### S2 目标轨 + 两个可区分 MIDI clip + 选中

1. `project.state` 取用户轨（与 `edu_jam_chat.go:eduJamUserTrackRows` 同语义过滤服务轨）；若用户轨数 < `track_index`，用真实产品命令 `track.add` 逐条补齐；断言目标行索引/若存在 `user_track_index` 则交叉核对。记录创建数量、行 id/name/type、目标 track id。
2. 在目标轨建第一个 `create_midi_clip`（`start=0`、`length=4`、`time_unit=beats`），`add_midi_notes_bulk` 写 3 音（60@0/1、64@1/1、67@2/1）；`get_midi_clip_notes` 回读断言音高/起点/时值。
3. 建第二个 clip（`start=8`、`length=4`），写 3 音（67@0/2、71@2/0.5、74@2.5/1.5）；回读并与第一个签名不同。
4. `select_clip {clip_id: second, track_id: target}`：记录 `ui_action`；该调用是 UI-only（§6 G1），失败只记 `select_clip_error`，不阻断。
5. 选中绑定（脚本实际选择：**context**）：`selected_clip_id`、`selected_clip_ids[0]`、`selected_clip_track_id`、`piano_roll_focus_clip_id`、`piano_roll_focus_track_id` 全部指向 second/target，避免缓存 UI 焦点抢占。
6. 证据：`summary.s2`、`responses/*s2*.json`。

### S3 “写好了”读取选中 clip

- 同一会话发 `写好了`，context 为 S2 的选中绑定。
- 断言：`stop_reason=edu_jam_node_done`；`workflow_data.clip_scope=selected`；`workflow_data.clip_ids[target_slot] == second` 且 `!= first`；`selected_clip_id == second`；`node_done == S2 当前节点`；`current_node` 已前进。
- **首个 clip 不得冒充**：`clip_ids[target_slot] == first` 或无 `clip_scope=selected` 都是失败。
- 证据：`summary.s3`、`responses/*s3_check_selected.json`。
- 边界：本编译卡的 clip 节点是 `clip_exists`（无 judge 叶），pass/fail 不会因音符内容在 first/second 间不同；S3 用 `clip_ids`/`clip_scope`+"两 clip 音符签名不同"证明选中生效，不伪造 notes judge 差分（§6 G5）。

### S4 真实 LLM 自然提问（唯一正式轮）

- 发送非命令自然语句：`我们现在第几关？这一关目标是什么？请一句话回答`；context 为 S2 的选中绑定。
- **探针处理**：第一次响应可能是确定性的 `edu_jam_recovery_broadcast`（自动开场/恢复播报，零 LLM）。仅在此情况下，脚本再发一次同一问题作为正式 LLM 轮；若第一次就是 `done`，它就是唯一正式轮，不重发。若第一次是其他 stop_reason，exit 4，不重试。
- 成功条件（三件全满足）：
  1. 正式轮 `stop_reason=done`；
  2. `reply` 非空（空回复=exit 4）；
  3. 模型输入含 `lesson_context`：S0 记录 `agent_prompt_render.jsonl` 的路径与字节偏移；S4 后**只读新增行**（`Read-AppendedText`/FileStream，不整文件加载），找到 `conversation_id` 匹配且行内（`system_prompt`/`turn_directives`/任意嵌套 JSON 字段）含 `lesson_context` 的行；断言 `schema_version=vit.edu_jam.lesson_context.v1`、`card.id=card_id`、`node.id=当前节点`、`selected_clip.clip_id=second`（有 `track_id` 则=target），保存 `evidence/s4_prompt_render.jsonl`。
- 辅助：若 prompt render 无 `lesson_context`，脚本**可选**检查 `agent_context_snapshots.jsonl` 新增行（**不要求新增**）；仍无 → exit 5，不伪 PASS。
- **已知边界**：当前 `agent_prompt_render` 诊断只记录 system 消息（`agentloop/message_loop.go:4864-4910`）；`lesson_context` 在 user 模型快照（`planner/planner.go:223-224` / `agentloop/message_loop.go:3852-3854`）。b291e934 修的是模型快照，不保证该 JSONL 含 `lesson_context`；若重建后仍无，需产品诊断补 model snapshot/user message（或 message loop 落 context snapshot）。脚本按 exit 5 诊断并保存 prompt render 原文。
- 回复内容只做**宽容关键词记录**（`第/关/目标/旋律/走向/两小节/曲线/平直/平缓/跌宕/节奏` 命中列表），不硬套文案；关键词不是 PASS 门槛。`keyword_hits=[]` 只 warn 并交 lead 复核，不改写 S4 的 done+reply+lesson_context 结论。
- 可选证据：`agent_prompt_render.jsonl` 命中字段/行数；`agent_llm_telemetry.jsonl` 新增行的 `conversation_id/model/usage/error`（error 非空=记录并披露；无行=记录边界；二者都不单独推翻 done+reply+lesson_context 三件套）。
- 证据：`responses/*s4_question*.json`、`evidence/s4_prompt_render.jsonl`、`evidence/s4_lesson_context.json`、可选 `evidence/s4_context_snapshot.json`、`summary.s4`。

### S5 失败语义（不假通过）

- **S5a 无选中控制**：当前 clip 节点发 `写好了`，context 为空；断言 `edu_jam_node_done`、`clip_scope=first`、`clip_ids[target]=first`、`selected_clip_id` 缺席；记录 `selected_used=false`、`pass_basis=first_clip_of_target_slot`。这是控制组，**不是**选中 clip 的 PASS。
- **S5b 跨槽选中**：在第一个用户轨（reference-track 索引 1）建第三个 MIDI clip；`select_clip` 可选；发 `写好了`，context 指向该跨槽 clip；断言 `edu_jam_node_done`、`clip_scope=selection_unmatched`、`selected_clip_id=cross`、`clip_ids[target]!=cross`、`selected_used=false`、`pass_basis=first_clip_of_target_slot`。节点因目标槽首个 clip 而通过，但**不得**记成选中生效。
- 边界：`clip_exists` 语义下，无选中/跨槽选中仍可能推进；本场景只证明“披露正确 + 不冒充”，不要求产品在该节点失败。若换成 judge 节点，应以 judge 失败为准；本卡编译结果没有该节点（§6 G5）。
- 证据：`summary.s5a/s5b`、`responses/*s5a*/*s5b*.json`。

### S6 结束→重开→自然语句（auto-intro/恢复）

1. 主会话 `结束乐局`：断言 `stop_reason=edu_jam_exited`。
2. `project.open A`；轮询 active project，断言 UUID 与 S1 一致。
3. 读 binding：若 `opened_at` 已非空，**不发**自然语句（否则 auto-intro 静默、会掉进第二次 LLM 轮）；记 S6 未取证（exit 5），不重试。
4. 用**新会话**发自然语句 `我回来了`：断言 `stop_reason` ∈ `{edu_jam_entered, edu_jam_recovery_broadcast}`、`workflow_data.level_id=card_id`、`current_node` 非空；记录 `opened_at_after`（`entered` 路径应被回写，`recovery_broadcast` 记录实际值；回写缺失只记边界，不硬判）。
5. 若 `recovery_broadcast`，teardown 用 `workflow_data.owner_conversation_id` 关闭该局，避免向错误会话发 `结束乐局` 而落入普通 LLM 轮。
6. 证据：`responses/*s6*.json`、binding 文件、`summary.s6`。

### Teardown

- 关闭 S6 局（若有）与主会话局（若失败前仍 active）；接受的 stop_reason：`edu_jam_exited`/`edu_jam_terminal`；其他记 teardown error。
- 按 S0 快照还原 recents / `Settings.xml` / `default_project.xml`，逐字节哈希一致；原不存在的文件还原为不存在。
- `GET /agent/authority`；若与 S0 不同则 `POST` 还原并复核；失败=exit 2。
- `GET /agent/runtime/status`：`goal.status` 必须属于非活动集；否则 teardown error（exit 2）。
- 保存 kernel log 新增片段；写 `summary.json`。
- **不**停止栈、不检查进程/TCP/UDP；由 berth owner 停栈后核零并记录。

## 5. 真 LLM 纪律（预注册）

- **轮数**：S4 恰好一次 LLM 正式**用户回合**（内部 semantic entry/agent loop 的多次模型调用不另起用户回合）。S4 的第一次 HTTP 调用若是 `edu_jam_recovery_broadcast`（确定性、零 LLM），才允许再发一次同一问题；第一次是 `done` 时它就是唯一正式回合，不重发。其他 stop_reason 立即 exit 4。
- **成功条件**：`stop_reason=done` + 非空 reply + 新增 `lesson_context` 快照（schema/card/node/selected clip）。三者缺一不算 PASS。
- **失败分类**：LLM 超时（4）、空回复（4）、stop_reason 非 done（4）；context 快照缺失（5）；HTTP/栈/还原（2）；确定性断言（1）；telemetry error 记录并披露，不单独改写 done 结论。
- **不重试**：失败即停、保留原工件；不原样重跑。同一代码版本在同一确定性断点连续失败两次后，按 `AGENTS.md` §8 先补代码锚点/缩小假设/拆取证卡，不得继续消耗轮次。
- **不硬套文案**：reply 只记录宽容关键词；不以“回复里出现某个词”替代 `lesson_context` 取证。
- **不伪 PASS**：S5 的 `node_done` 不得写成“选中 clip 通过”；S3 的 `clip_scope=selected` 不得写成“notes judge 通过”；S6 未触发 auto-intro 不得写成“恢复通过”。

## 6. API 缺口与取证边界（fail-honest）

| # | 缺口/边界 | 代码锚点 | 脚本处理 |
|---|---|---|---|
| G1 | `select_clip` 是 UI-only 动作，不写内核/agent 选择态，也没有 HTTP 读回当前选择的接口 | `agent/internal/harness/harness.go:2837-2847`（`invokeLocal` 返回 `ui_action`）；`agent/internal/tools/catalog.go:1235`（RiskDirect） | 记录 `ui_action`；失败只记 `select_clip_error`；实际绑定走 `/agent/chat` context。S3 用 `clip_ids/scope` 证明 |
| G2 | W01 导入只复制/编译设计源，不建设计轨/clip；`track.add` 只建 audio 轨且忽略名字（内核命名 `Track N`） | `agent/internal/projectpackage/legame.go:374-481`；`VitApp/Source/Service/TrackService.cpp:509-547`；slot 索引语义 `agent/internal/chat/edu_jam_chat.go:887-900` | 从编译卡取 `track_index`；用 `track.add` 补齐到目标索引并记录；不声称设计轨名/角色已物化；只建到目标槽所需数量 |
| G3 | auto-intro 的“已播报”标记是进程内每会话 map，无 API 读/写 | `agent/internal/chat/edu_jam_chat.go:1478-1501` | S4 探针识别 `edu_jam_recovery_broadcast` 后再发正式问题；S6 用新会话触发 auto-intro；不重复消耗 LLM |
| G4 | 模型输入取证：`agent_context_snapshots.jsonl` 在 free-state message loop 可能 stale；`agent_prompt_render.jsonl` 是每轮诊断，但**当前只记 system 消息**，`lesson_context` 在 user 模型快照；无 HTTP 端点 | `agent/internal/agentloop/message_loop.go:4864-4910`（只写 SystemPrompt）；`agent/internal/planner/planner.go:223-224` / `agent/internal/agentloop/message_loop.go:3852-3854`（snapshot 在 user 消息）；`agent/internal/contextruntime/context.go:213-261`（snapshot 路径/env） | S0 记录 `agent_prompt_render.jsonl` 路径+偏移；S4 只读新增行，解析行内任意字段的 `lesson_context` 并保存 `evidence/s4_prompt_render.jsonl`；context snapshot 仅可选 fallback；无则 exit 5，不伪 PASS |
| G5 | 本设计编译卡的 clip 节点只有 `clip_exists`/`track_non_empty`，没有 judge 叶；first/second 的音符差异不改变 pass/fail | 编译卡（`n2-rhythm`/`n3-contours`/`n4-develop` 均为 `structure: clip_exists, clip: melody-track`）；`agent/internal/legamedesign/compile_card.go:640-682` | S3 用 `clip_ids[target]=second`+`clip_scope=selected`+"两 clip 音符签名不同"取证；明确记录“无 notes judge 差分”，不伪造 |
| G6 | `project.new`/`save_as` 后活动工程留在 A，没有 HTTP “关闭工程/恢复上一工程”命令 | `agent/internal/tools/catalog.go:1067-1072`；`VitApp/Source/Service/VitHeadlessService.cpp:1119-1199` | 不擅自重开用户工程（避免写 RunRoot 外）；在 summary 记录“active project=A”；由 owner 停栈/重开 |
| G7 | 脚本不拥有栈进程，无法自行证明 process/TCP/UDP 全零 | — | teardown 只断言“无 active turn + 会话关闭 + 文件/authority 还原”；进程/端口零由 owner 停栈后核 |

## 7. 工件与证据链

`RunRoot` 布局（每次新根，不覆盖旧 run）：

```text
run_<stamp>/
  requests/001_<name>.json ...       # 每个 HTTP 请求体（UTF-8）
  responses/001_<name>.json ...      # 每个 HTTP 响应（UTF-8）
  snapshots/
    recents.before
    Settings.xml.before
    default_project.xml.before
  evidence/
    s4_context_snapshot.json         # 命中 lesson_context 的那一行快照
    s4_context_snapshots_appended.jsonl
    s4_prompt_render_appended.txt    # 可选
    kernel_log_appended.txt          # 可选
  W02-A1c-A-<stamp>.vit              # 隔离工程 A
  W02-A1c-A-<stamp>.vit_project/     # 项目包（编译卡/binding 在这里）
  summary.json                       # 唯一结论面
```

`summary.json` 关键字段：`status`、`failure_class`、`error`、`card_id`、`compiled_card`、`binding_path`、`preflight`、`s1…s6`、`teardown`。所有结论都能回指 `requests/responses` 或 `evidence` 原始文件。

## 8. 零栈验收（本 worktree 已跑）

以下均不启栈、不占端口、不写 RunRoot（dry-run/no-auth）：

| 项 | 命令（等价的 Windows PowerShell 5.1） | 实测 |
|---|---|---|
| 解析 | `[System.Management.Automation.Language.Parser]::ParseFile('<repo>/scripts/run_w02_a1c_smoke.ps1',[ref]$t,[ref]$e)` | **PARSE OK，exit 0** |
| 无授权 | `powershell -NoProfile -ExecutionPolicy Bypass -File <repo>/scripts/run_w02_a1c_smoke.ps1 -RepoRoot <worktree>` | **exit 3**；`NOAUTH_RUNROOT_EXISTS=False` |
| Dry-run | 上条加 `-DryRun` | **exit 0**；打印计划，零 HTTP |
| 静态卫生 | `Select-String -Path <script> -Pattern 'Start-Process|Start-Job|TcpListener|HttpListener|WebRequest|netstat|Get-NetTCPConnection|Stop-Process'` | **0 hits** |

- 解析测试在 `powershell.exe`（Windows PowerShell 5.1）下执行；脚本以 UTF-8 BOM 保存，保证 5.1 正确解码中文 chat 命令。
- 授权负例（不启栈）：`-AgentBaseUrl http://127.0.0.1:1 -StackAuthorized -RuntimeOwner <selfcheck>` → **exit 2**，`summary.json` 落盘、`failure_class=http`；teardown 因栈不可达记 failed，不伪 PASS。
- 额外单测（同次零栈）：编译卡解析器对 W01 真实编译卡输出 `slots=5/nodes=8`，`n2-rhythm: clip=melody-track/track_index=2`；用户轨过滤对 5 服务轨 + 2 用户轨返回 2；snapshot/restore 对存在/不存在文件均哈希一致。
- S4 取证自检（新增）：合成 prompt render 行（top-level `lesson_context` 与 `system_prompt` 内嵌 JSON）均能提取 schema/card/node/clip；首轮实际 `evidence/s4_prompt_render.jsonl` 行不含 `lesson_context`（与 FINDING 一致），脚本按 exit 5 诊断，不伪 PASS。
- 未运行真实栈；未消耗 LLM。

## 9. 真栈轮候选命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File C:\Users\timoz\.codex\worktrees\pc432-real\Vit_DAW\scripts\run_w02_a1c_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -AgentBaseUrl http://127.0.0.1:7878 `
  -StackAuthorized `
  -RuntimeOwner "lead/<PC-RUNTIME-STACK 登记 owner>" `
  -TimeoutSeconds 240 `
  -RunRoot C:\Users\timoz\.codex\worktrees\pc432-real\Vit_DAW\coord\runs\W02-A1c\real-stack\run_<stamp>
```

- 若栈从本 worktree 启动，把 `-RepoRoot` 换成该 worktree；`-RepoRoot` 必须与实际 `VitApp/Workspace/Logs`/Settings/default_project 所在 checkout 一致。
- `-RunRoot` 可省略（默认 `%TEMP%\vit_w02_a1c\run_<stamp>`）。无论放哪里，**运行工件不提交**；本卡只提交 runbook + 脚本。
- 运行后由 lead/owner：停栈 → 核 process/TCP/UDP 零 → 记录 HEAD/二进制哈希/run ROOT/summary → 保留原始工件 → 在卡上回执。失败不原样重跑；先判定执行偏差/卡片缺陷/假设失效，再决定修装置、补证据或拆卡。

## 10. 停止条件

- S0 busy/dirty/栈不可达/授权缺失：不进入 S1；exit 2/3，保留 preflight 证据。
- S1 导入/开局失败：停在 S1；不得用别的工程/别的卡继续。
- S2/S3 失败：停在 S3 前/后；不进入 LLM 轮，避免浪费正式轮。
- S4 失败：正式轮结束；不跑 S5/S6、不重试；按 §5 分类并保留原工件。
- S5/S6 失败：停在失败点；teardown 尽力关局/还原；按类退出。
- teardown 还原失败：按环境中断 exit 2；由 owner 手动恢复并从 before 快照核对哈希。
- 任何“为了通过而”修改卡片/设计源/产品代码/期望值的行为禁止；缺口已在 §6 登记，按 fail-honest 处理。
