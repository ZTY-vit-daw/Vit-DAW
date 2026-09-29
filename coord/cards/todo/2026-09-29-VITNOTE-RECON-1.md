# VITNOTE-RECON-1：vit note 前置勘察——框选/气泡 UI 面+会话并发模型+webui 同步面+orchestration 子代理底座

- 池序 4（PAPER-EXP-ADAPT-1 在飞后可领；零代码只读勘察可与一切并行）
- 优先级 / 预估 / 依赖：P1 / 0.5–0.8 天 / 无（只读）；概念权威=decisions/2026-09-29-vit-note-concept.md（三轮用户设计讨论收敛，先读）
- 模型分级：L1 / GLM 或 flash 均可（零代码，Godot 面+Go 面双侧勘察）
- 目标（七节勘察面，报告入 coord/runs/VITNOTE-RECON-1/report.md；**勘察聚焦裁定（2026-09-29 晚用户讨论）**：v1 主战场=编排面（框选语义最丰富：轨×时间窗）；钢琴卷帘=第二战场次级盘点（音符组语义，随 L1-5 编曲能力启用）；调音台/插件面板/媒体池/频谱=仅存在性确认不深勘察——框选非自然交互或语义不足以构成任务）：
  1. **Godot 框选与便签容器面（编排面深勘察为主）**：时间轴/编排面框选交互现状（钢琴卷帘已有框选先例 PianoRollView.gd，主编排面待核）；原位浮窗/便签容器可行性锚点（Godot 内已有浮层先例：tooltip/hud/气泡——鼠标跟随气泡现状实锚，含其锚定实现）；框选→语义解析挂点（轨/clip/时间窗提取，requestContext 组包现状）；钢琴卷帘框选面次级盘点（音符组语义解析挂点）；其余视窗仅确认框选交互存在性。
  2. **IME 与文本输入**：Godot 4 内嵌 LineEdit/TextEdit 中文输入法支持现状（项目内既有输入框先例：命名框/数值框——实测可引证）；原位输入框焦点管理与 DAW 快捷键冲突面。
  3. **chat 主管道接线面**：前端触发 chat 请求的现有入口（vsp_hub WS/HTTP 面）；带 selection_context/current_selection 的请求组包现状（range_context 管道——IMPL-C R6 烟测已证存活）；新建会话（每 note 一对话流）的前端侧调用面现状。
  4. **agentloop 多会话并发模型**：消息循环是全局单循环串行还是多会话并行（message_loop 并发模型实锚）——写冲突天然串行化与否的关键事实；多 conversation 并存的服务端现状。
  5. **webui 同步面**：webui 会话列表/切换现状（多 note 流的呈现载体）；range_context/selection 类消息在 webui 的现有呈现（上下文徽章是否存在）；E2E-WEBUI-1 门槛对徽章小卡的覆盖方式。
  6. **orchestration 子代理底座**：OwnerPolicy 的会话所有权语义与 vit note 扩展点（authority_conflict 队列化可行性）；CapabilityDefinition 的 RiskCeiling 与 vit note 权限分级映射面（bounded_reversible 白名单现状）；Worker 概念文档（PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1，历史文档查证口径）与现行代码的差距清单。
  7. **鼠标气泡底座**：跟随鼠标气泡的现有实现（若有）与"空间感知底座"共享面评估（hover 快捷键提示层与框选触发层的公共基建）。
- 约束：零代码改动；只读勘察；Godot 前端为仓库外工作树（行号以当日为准，报告标注）；历史文档（ORCHESTRATION_V1）按查证口径用，不当作现状。
- 验收标准：报告七节全落+锚点行号+HEAD；每节"支持/不支持/需新开面"三态结论；§4 并发模型结论必须给出实锚证据（代码级，不允许推断）。
- 停止条件：常规勘察止损。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 报告链接 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
