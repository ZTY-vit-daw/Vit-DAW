# VITNOTE-NOTESTREAM-2：每 note 独立会话流——面板自含问答历史+主对话流去 note 往返（用户裁定 2026-10-02，P2）

- 池序 35；目标仓库=D:\Godot\project\vit-daw-frontend+D:\Vit_DAW（**跨 Godot 面板域+agent chat 域两腿，一卡串行完成**）；来源=[decisions/2026-10-02-user-manual-test-feedback.md](../../decisions/2026-10-02-user-manual-test-feedback.md) §2+追记裁定 2（用户原话确认：「面板内自含问答历史，主对话流不再出现 note 的往返」）
- 优先级 / 预估 / 依赖：P2 / 1 天 / **与 JUDGMENT-SETTLE-STALL-1 同包（internal/chat）——建议串行在其后领取**（文件域若实证不相交可并行，领取时申报）；面板历史 UI 建议在 CONTAINER-2 布局基线上接
- 模型分级：L1 / GLM 首选（会话语义+schema 兼容+跨域三难点）
- 已核实事实：
  1. 现状：note Q&A 经 manager `get_agent_context`（F7）→主控台组包→7878；用户实测 note 问答往返出现在 webui 主对话流（chat_* 会话与主流呈现关系待实锚——2026-10-02 晚 note 会话=chat_0313eaa025726d04）。
  2. note 载荷 v2 已带 note_id（note_<id>，IMPL-B/C/D 链）——天然会话键。
  3. webui 无会话流侧边栏（用户反馈；历史界面分支/工作树机制为现路径）——**本卡不含 webui 侧**（侧边栏另卡，见 decision §2b）。
- 目标：
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
- 领取：（时间 / 两仓基线 hash / 分支名）
- 回执：（commit hash / 取证结论 / 判据 1-4 证据 / 回归结果）
- 验收：（裁定文件 / 验收 commit）
