# VITNOTE-NOTESTREAM-2：每 note 独立会话流——面板自含问答历史+主对话流去 note 往返（用户裁定 2026-10-02，P2）

- 池序 35；目标仓库=D:\Godot\project\vit-daw-frontend+D:\Vit_DAW（**跨 Godot 面板域+agent chat 域两腿，一卡串行完成**）；来源=[decisions/2026-10-02-user-manual-test-feedback.md](../../decisions/2026-10-02-user-manual-test-feedback.md) §2+追记裁定 2（用户原话确认：「面板内自含问答历史，主对话流不再出现 note 的往返」）
- 优先级 / 预估 / 依赖：P2 / 1 天 / **与 JUDGMENT-SETTLE-STALL-1 同包（internal/chat）——建议串行在其后领取**（文件域若实证不相交可并行，领取时申报）；面板历史 UI 建议在 CONTAINER-2 布局基线上接
- 模型分级：L1 / GLM 首选（会话语义+schema 兼容+跨域三难点）
- 已核实事实：
  1. 现状：note Q&A 经 manager `get_agent_context`（F7）→主控台组包→7878；用户实测 note 问答往返出现在 webui 主对话流（chat_* 会话与主流呈现关系待实锚——2026-10-02 晚 note 会话=chat_0313eaa025726d04）。
  2. **手测二号实锚追加（2026-10-03 晚，取证见 [runs/MANUAL-TEST2-20261003/FORENSIC.md](../../runs/MANUAL-TEST2-20261003/FORENSIC.md)）**：note"hi"问答落进**主会话图同一 session graph**（draft_20261003T113402 八节点全量，ask n_113827/assistant n_113832）——"note 入主流"直接实证；webui 侧边栏新会话条目显示报错+未命名的机制（localStorage sessionFlow 注册表）归本卡相位 1+SESSION-SEMANTICS 取证。
  3. **手测三号场实锚（2026-10-03 晚）**：便签问「你能告诉我这个范围是什么内容吗？」落主图走治理链（contract_scope=选中轨 1007 非辖区），工具面仅通用工程查询，终局"能力边界"拒绝——辖区上下文未注入+便签问答误入 goal 管线双证；[FORENSIC-3](../../runs/MANUAL-TEST3-20261003/FORENSIC.md)。
  2. note 载荷 v2 已带 note_id（note_<id>，IMPL-B/C/D 链）——天然会话键。
  3. webui 无会话流侧边栏（用户反馈；历史界面分支/工作树机制为现路径）——**本卡不含 webui 侧**（侧边栏另卡，见 decision §2b）。
- 目标：
  **设计定稿追加（2026-10-03 用户裁定，实现相位按此）**（裁定见 [decisions/2026-10-03-session-naming-and-scope-context.md](../../decisions/2026-10-03-session-naming-and-scope-context.md)）：
  - 目标 6 细化：便签流默认名=「便签 N<序号> · <面1> <x>% · <面2> <y>%」（faces[] 降序前两面，与图钉 tooltip 同源同文；空辖区=「便签 N<序号> · 空辖区」）；出生即命名、用户改名最高且持久（注册表 fail-open）、同 note 重开同名（note_id 键）；LLM 增强命名=二期不做。
  - 目标 2 细化：辖区上下文=**身份+一级轻摘要**（轨名+clip 数+挂载插件名），细节按需查询（上下文给身份，工具查细节）。
  - 验收追加项：便签问答**只走只读观察路径，零进 D1 治理链**（三号场"能力边界"拒绝=走错管线，实现后同问句必须得到辖区内容回答）。

  1. **取证先行（轻量）**：实锚 note 7878 问答的会话路由——当前 chat_* 会话 id 如何生成、与 webui 主流（webui_*）的呈现关系、note 问答为何出现在主流（回答：是同会话还是主流聚合渲染）；结论回写本卡。
  2. **agent 侧**：note 问答路由到独立会话——会话键=note_id（同一 note 重开延续同会话；收起/重开不清史）；主任务 goal 的 LLM 上下文不再注入 note 往返（观察问答 observe 语义隔离）；持久化兼容（AGENTS §11：新会话形态 fail-open，旧工程加载零破坏）。
  3. **Godot 侧**：note 面板内自含问答历史（消息列表+滚动，CONTAINER-2 的输出区基线上扩展）；note 关闭/删除=会话归档（可查，不入主流）。
  4. **验收判据（用户口径）**：note 里问一圈→webui 主对话流**零**新增消息；note 重开→历史还在；同一问题主任务上下文不被污染（遥测 section_stats 对照，回执附前后对比）。
  5. 回归：chat 域测试+全量 0 FAIL（-count=1）+webui npm test+note E2E/探针面+真栈手测（与用户约定复验）。
  6. **（2026-10-03 手测反馈问题 3 并入）note 会话默认命名协议**：note 圈选新建的会话在 webui 侧不得显示"未命名对话流"——按默认协议命名（用户裁定方向：note id 或圈选规划位置，如「便签·轨道时间线 72%」式，实现取简），用户可改名；判据 4 的对照面（webui 主流）同时验证命名正确落侧边栏。
- 文件域：Godot `vit_note_panel.gd`+`vit_note_manager.gd`；agent chat note 路由与会话面（实锚后申报）。
- 约束：Godot 腿分支自 CONTAINER-2 之后基线切出（或经决策侧协调）；agent 腿 worktree 纪律；探针输出落 runs/。
- 验收标准：四判据全过+回归全绿+持久化兼容测试+用户手测复验。
- 停止条件：取证发现 note 会话已独立（仅主流聚合渲染问题）→ 修面改 webui 渲染，域变更上交裁定；会话架构改动触及历史图/worktree 语义 → 上交（超出本卡）。
- 领取：2026-10-03 20:51 +0800 / Vit_DAW origin/main=aa3fe836（工作树另有 VitApp/Workspace/default_project.xml 改动+runs 工件，非本卡）/ Godot port/vitnote-container-4@a2930a7（切出点即卡面基线）/ 分支：Vit_DAW=port/vitnote-notestream-2（agent 腿，独立 worktree），Godot=port/vitnote-notestream-2（自 a2930a7 切出，独立 worktree）
- 取证结论（2026-10-03 21:15，PC 执行侧，代码实锚，目标 1）：
  1. **chat_* 生成**：`agent/internal/chat/server.go:2256-2259`——/agent/chat 载荷 conversation_id 空 → 服务端生成 `chat_+randomID()`；与 note_id 零关联（面板初始空、首答回包持有返回值）。
  2. **与 webui_*（主流）呈现关系**：webui 新会话自成 `webui_<ts36>`（`webui/src/App.tsx:1181`）；但主流终局消息水合**不按会话 id 过滤**——`historyMessagesFromUIState` 从 `uiState.project_history.conversation_messages`（工程单图）取消息（App.tsx:10705，注释明言"终局消息的服务端图水合不受会话 id 影响"）。
  3. **note 问答为何入主流（双重同向，回答=既同图又聚合渲染）**：①`handleChat` 对一切消息 `RecordConversationNodeForProject(projectPath,"ask"/"vit")`（server.go:2314/2347）——会话图按**工程单图**（conversation_graph.json）落盘，与 conversation_id 无关 → note 节点进主图（二/三号场实锚即此机制）；②webui 主流再从该图聚合水合 → 主流可见。
  4. **误入治理链**：`beginChatGoal`（goalrunner_chat.go:32）把每条 /agent/chat 消息变 goal；面板 context 复用主控台组包（vit_note_manager.gd `get_agent_context`→`_build_agent_context`）→ contract_scope 取主控台选中轨（三号场 1007）；圈选快照 faces 根本不在载荷中。
  5. **侧边栏未命名**：行=本地注册表+服务端 continuations hints 合并（webui/src/sessionFlow.ts）；note 的 chat_ff99… 经 durableContinuations 投影（continuation_scheduler.go:2222）冒出，title 空+本地消息桶空 → 「未命名会话」。
  → **修面**：/agent/chat 增 note 载荷（note_id+run 戳 session 键+默认名+辖区 faces）→ note 会话模式：不走 beginChatGoal、不写工程单图、独立 note 会话库（`.vit_derived/<uuid>/note_sessions.json`，fail-open）；webui 侧边栏并 note 会话投影（带默认名+可改名+可查）。
- 回执：（2026-10-03 21:25，PC 执行侧）
  - **commit**：Vit_DAW `port/vitnote-notestream-2`@**618d69f9**（agent+webui 腿，已推 origin）；Godot `port/vitnote-notestream-2`@**4391c49**（自 container-4@a2930a7 切出，已推 origin）；报告=[runs/VITNOTE-NOTESTREAM-2/REPORT.md](../../runs/VITNOTE-NOTESTREAM-2/REPORT.md)。
  - **取证结论**：见上"取证结论"段（五条，目标 1 回写完成）。
  - **判据 1-4 证据**：REPORT.md §三——判据 1=机制三证（note 早分叉位于单图落盘调用点之前/Go 钉主对话记忆零写入+响应无 GoalID/webui 主流水合源不消费 note 投影）；判据 2=探针 D 面（collapse/reopen 消息不清）+Go 钉（同键续档 4 条）；判据 3=note 组装只吃 note 会话库（Go 钉断言）、主 goal 两源（工程单图+主对话记忆）零 note 写入、vitnote_* 分段名可与主组包遥测（section_stats）对照；判据 4（目标 6）=vitest 钉（默认名落侧边栏不显未命名/本地命名权威/缺行补建可改名/切换回放种子）。
  - **回归**：Go 全量 `go test ./... -count=1` **exit 0，87 包 ok 0 FAIL**（go_test_full.log）；chat 域 ok；note 新钉 9/9（go_note_tests.log）；webui `npm run test` **425/425**+`tsc --noEmit` 0 错+`npm run build` 过（webui_test.log）；Godot 探针 **notestream 21 / container 141 / circle_state 52 / face_resolve 127 / panel_summary 16 全 0 FAIL exit 0**（同目录 .log）。
  - **文件域申报（实锚后）**：agent=internal/chat/{server.go,note_sessions.go 新增,note_sessions_test.go 新增}；webui=src/{sessionFlow.ts,SessionFlowSidebar.tsx,App.tsx,types.ts,sessionFlow.test.ts}；Godot=两卡面文件+**新增** tools/probe_vitnote_notestream.gd（探针面新增，测试专用）；runs 工件 coord/runs/VITNOTE-NOTESTREAM-2/。
  - **端测边界声明（AGENTS §5）**：全程未起真栈（用户指令）——7878 真栈往返/webui 浏览器级渲染/用户旅程未覆盖，`[等待真栈手测复验]`（卡面验收口径）；webui 侧边栏无 DOM 级断言（E2E-WEBUI-1 前如实声明）；agent 无活动工程身份时会话库降级进程内不落盘（fail-open 申报）；旧 note 问答路径整体退役（非并存开关），Godot/agent 需同版本配套（验收合并建议同批或 Godot 先）。
- 验收：**conditional pass（2026-10-03 决策会话）**——[rulings/2026-10-03-VITNOTE-NOTESTREAM-2-conditional.md](../../rulings/2026-10-03-VITNOTE-NOTESTREAM-2-conditional.md)；我方复跑全对齐（note 9 钉+chat 123.7s+全仓 -count=1 EXIT=0+vitest 425/425+探针 21/141/52/127/16/import 全绿；新鲜 worktree 需先 --import 的复跑纪律点记 gate）；转正条件=用户真栈手测四判据（辖区问答/命名落侧边栏/主流零污染/重开留史）；618d69f9 随验收合 main，Godot 4391c49 同批配套
- **转正 pass（2026-10-03 四号场，用户手测四判据全过）**：辖区问答得辖区内容回答（零进治理链）/侧边栏默认命名+改名+回放/主流零污染/收起重开留史。遗留增强：辖区时间维度披露与范围即操作目标（另立卡）
