# VITNOTE-RECON-1：vit note 前置勘察——框选/气泡 UI 面+会话并发模型+webui 同步面+orchestration 子代理底座

- 池序 4（PAPER-EXP-ADAPT-1 在飞后可领；零代码只读勘察可与一切并行）
- 优先级 / 预估 / 依赖：P1 / 0.5–0.8 天 / 无（只读）；概念权威=decisions/2026-09-29-vit-note-concept.md（三轮用户设计讨论收敛，先读）
- 模型分级：L1 / GLM 或 flash 均可（零代码，Godot 面+Go 面双侧勘察）
- 目标（八节勘察面，报告入 coord/runs/VITNOTE-RECON-1/report.md；**通用性裁定（2026-09-29 晚用户裁定，取代此前聚焦裁定）**：框选+输入为全局通用能力——任何视窗可用，好不好用留给用户判断；语义解析两级：L1 结构化 provider（有对象模型的视窗，精确）优先，L2 视觉兜底（截图→多模态识图→空间意图理解，任意视窗近似）——**视觉只做意图理解，执行永远走结构化命令（AGENTS §5 computer use 禁区守界）**）：
  1. **Godot 全局框选与便签容器面**：全局坐标层框选事件可行性（不依赖视窗内部结构的统一框选层 vs 逐视窗事件）；各视窗框选/选择交互现状盘点（编排面/钢琴卷帘/调音台/媒体池/频谱——存在性与现有选择数据面）；原位浮窗/便签容器可行性锚点（tooltip/hud/鼠标跟随气泡现状实锚）；框选→语义解析挂点（L1 结构化：轨/clip/时间窗/音符组的对象模型提取，requestContext 组包现状）。
  2. **IME 与文本输入**：Godot 4 内嵌 LineEdit/TextEdit 中文输入法支持现状（项目内既有输入框先例：命名框/数值框——实测可引证）；原位输入框焦点管理与 DAW 快捷键冲突面。
  3. **chat 主管道接线面**：前端触发 chat 请求的现有入口（vsp_hub WS/HTTP 面）；带 selection_context/current_selection 的请求组包现状（range_context 管道——IMPL-C R6 烟测已证存活）；新建会话（每 note 一对话流）的前端侧调用面现状。
  4. **agentloop 多会话并发模型**：消息循环是全局单循环串行还是多会话并行（message_loop 并发模型实锚）——写冲突天然串行化与否的关键事实；多 conversation 并存的服务端现状。
  5. **webui 同步面**：webui 会话列表/切换现状（多 note 流的呈现载体）；range_context/selection 类消息在 webui 的现有呈现（上下文徽章是否存在）；E2E-WEBUI-1 门槛对徽章小卡的覆盖方式。
  6. **orchestration 子代理底座**：OwnerPolicy 的会话所有权语义与 vit note 扩展点（authority_conflict 队列化可行性）；CapabilityDefinition 的 RiskCeiling 与 vit note 权限分级映射面（bounded_reversible 白名单现状）；Worker 概念文档（PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1，历史文档查证口径）与现行代码的差距清单。
  7. **鼠标气泡底座**：跟随鼠标气泡的现有实现（若有）与"空间感知底座"共享面评估（hover 快捷键提示层与框选触发层的公共基建）。
  8. **视觉兜底面（L2）**：截图基建现状（agent/internal/browsercapture 包能力面：截什么、怎么截、输出形态）；llm 客户端多模态（图像输入）支持现状（哪些 provider 可传图、消息结构）；Godot 侧视窗截图能力（视口/区域截图 API）；多模态调用成本与隐私面（截图外发路径清单——供 DESIGN 定开关默认态）。
- 约束：零代码改动；只读勘察；Godot 前端为仓库外工作树（行号以当日为准，报告标注）；历史文档（ORCHESTRATION_V1）按查证口径用，不当作现状。
- 验收标准：报告八节全落+锚点行号+HEAD；每节"支持/不支持/需新开面"三态结论；§4 并发模型结论必须给出实锚证据（代码级，不允许推断）。
- 停止条件：常规勘察止损。
- 领取：2026-09-29 21:50 +0800 / c8ef4e25（含勘察聚焦裁定版卡面）/ port/vitnote-recon-1（独立 worktree D:/Vit_DAW_worktrees/vitnote-recon-1；Windows 端执行——用户口令指定本会话开工）
- 回执：（commit hash / 报告链接 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
