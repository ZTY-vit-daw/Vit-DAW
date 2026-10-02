# VITNOTE_V1_DESIGN — vit note（框选便签子代理）v1 设计

- 产出：VITNOTE-DESIGN（决策侧亲自，2026-09-30 早窗）
- 输入：概念定案 [decisions/2026-09-29-vit-note-concept.md](../coord/decisions/2026-09-29-vit-note-concept.md)（含深夜追记：统一框选交互/液态玻璃/便签三态/便签池两档/联机远期）+ 八节只读勘察 [VITNOTE-RECON-1 报告](../coord/runs/VITNOTE-RECON-1/report.md)
- 状态：**规划未实现**——v1 IMPL 卡（§10）依赖本文档；§9/§13 含待用户拍板项
- 端测口径：agent 侧实现过真实栈烟测门槛（AGENTS §5）；webui 渲染面过 E2E-WEBUI-1；Godot 交互面手测走 M8 节点

> **V2 修订标注（2026-10-01，VITNOTE-INTERACT-V2-DESIGN-1；用户八项裁定 2026-10-01；2026-10-02 D1/D2 裁定追记）**：交互触发面在 V2 升级为全域圈选，触发键=**Q+左拖**（2026-10-02 D1 用户裁定迁键 Alt→Q，克隆保留 Alt；[decisions/2026-10-02-vitnote-v2-d1d2-ruling.md](../coord/decisions/2026-10-02-vitnote-v2-d1d2-ruling.md)）——**§4.1 的「框选完成→形变提示」触发面改为圈选完成**（plain marquee 回归纯选区语义，lane 级 `_notify_vit_note_hint` 通知路径退役——**已随 V2-IMPL-A 落地（2026-10-02 验收 pass）**）；§4.1 出口三件、§4.2/§4.3 容器与输入、§5 上下文派生规则（多面化）、§6 会话、§7 权限与写租约、§8 视觉、§9 便签池全部沿用，以 [VITNOTE_V2_INTERACTION_DESIGN.md](VITNOTE_V2_INTERACTION_DESIGN.md) 为交互面权威；§10 IMPL 序列中 IMPL-2 的组包目标改 v2 payload（faces[]），IMPL-3/4/5/6 不变；M8 手测清单已由 V2 九步版取代（V2 文档 §10.2，一场销 IMPL-1/DOCK-MOUNT/V2 三卡）。

---

## §0 摘要

vit note = DAW 前端中"框选区域 → 原位常驻便签小窗 → 携空间上下文进入 agent 工作流"的交互形态；**每个 note = 一个独立子代理会话**（独立 conversation_id，走 7878 chat 主管道），webui = 主代理。v1 是**子代理语义、单代理实现**：会话隔离 + 权限分级 + 工程级写租约，不被 orchestration Worker 阻塞。核心新开面四处：框选形变提示与便签容器（Godot）、marquee 原始矩形与音符组上下文出口（Godot）、RiskCeiling 执行闸与写租约（agent）、框选上下文徽章（webui）。

## §1 概念与定位

（权威定义见概念定案文档，本节只留设计引用所需的最小集）

- **定义**：鼠标框选 → 框右侧形变提示 → 点击/按键打开常驻便签 → 输入自然语言 → 携辖区上下文（轨/clip/时间窗/rect）进入 agent 工作流。
- **主/子代理**：webui=主代理（全权）；vit note=子代理（受 RiskCeiling 约束的受限代理）。
- **常驻/临时统一**：轨道监督代理=常驻 note（锚定轨、长生命周期）；框选 note=临时（任务完成归档）。
- **演进线**：原位结果卡（v1）→ 便签多轮（v1）→ 回复即 DAW 对象（v2+，与判定卡/确认卡合流）。
- **价值锚**：空间锚定上下文传递是对话流与 MCP 协议均不可达的形态，直接支撑"AI 必须长在 DAW 里"；音频 DAW 领域无先例。手测节点 M8（框选触发 vit note=全新交互形态质变点）。

## §2 事实基线（RECON-1 采信，设计只引用不重勘）

| # | 事实 | 出处 |
|---|---|---|
| F1 | 编排面框选链端到端在位：marquee 手势状态机 + 全局 rect 广播协议（`set_global_marquee_preview`/`_broadcast_marquee_preview`，vit_track_lane_2d.gd）+ 全局输入仲裁先例（TimelineInputArbitrator） | RECON §1a |
| F2 | 跨轨命中字典已含轨×clip×时间窗语义（`_clip_ids_intersecting_global_marquee`，按 track_y→start 排序） | RECON §1a |
| F3 | **缺口**：marquee 原始矩形不组包——空区域框选（无 clip 命中）不产生时间窗；time_selection 来自另一条时间拖选手势 | RECON §1d |
| F4 | 钢琴卷帘框选（lasso）支持、`_selected_note_ids` 存在，但音符组选区无 agent 语义出口（`get_selected_note*` 零命中） | RECON §1b |
| F5 | LineEdit 中文 IME 生产实证（主 chat 输入框即此形态）；`_has_text_input_focus` 焦点守卫与单会话发送期串行（`_set_send_enabled`）先例在 | RECON §2 |
| F6 | chat 主管道 = 7878 HTTP（`/agent/chat`，非 vsp_hub）；空 conversation_id → 服务端新建 `chat_<randomID>`；选区键已渗入 agent 决策层（semantic_entry/goalrunner_chat 等） | RECON §3 |
| F7 | 并发模型=**多会话并行**（每请求 HTTP goroutine 同步跑完整 agentloop）；写串行化仅命令粒度/CAS/文件锁三处——保数据一致性，不保任务互斥 | RECON §4 |
| F8 | RiskCeiling 全仓零执行消费方（元数据非运行闸）；Worker 零实现；authority_conflict 仅报告计数 | RECON §6 |
| F9 | 会话所有权域=conversation_id 已天然存在（controllerOwners.Active）；CapabilityInvocation 自带 ConversationID 字段 | RECON §6 |
| F10 | webui 刷新同步在（/agent/events 回放水合）；会话列表与框选上下文徽章不存在；FocusSummary 摘要卡为徽章最近先例；E2E-WEBUI-1 浏览器级基建未建 | RECON §5 |
| F11 | webui 带任意 conversation_id 即可重建该会话——徽章可作零新开面的会话入口 | RECON §5 |
| F12 | 鼠标跟随气泡不存在；现存两套空间机制互不相通（vit_context_manager hover 栈=事件无坐标；marquee 全局 rect=坐标无控件注册） | RECON §7 |
| F13 | llm 多模态客户端在库在测未接线；Godot 截图零实现；唯一图像外发点=provider endpoint；browsercapture `user_approved` 为批准位先例；路由缺失 fail-closed 可作"默认关"参照 | RECON §8 |
| F14 | clip 级效果机制半在：内核竖线 clip_scope（`rack_set_node_clip_scope` IPC）+ agent 工具目录在册 RiskConfirm 级（tools/catalog.go:1047），禁令（单 Clip 绑定仅限手动）为宪法/prompt 纪律层——**2026-09-30 用户裁定向 agent 开放**；自动化零实现，能力层既定序=空间与深度（C3）之后 C4 段落自动化 | ROUTING_CONSTITUTION.md；decisions/2026-09-30-clipscope-open-and-automation-slot.md |

基线版本：agent=3e3cc7f5，Godot 前端=5280822（行号随前端提交漂移，以锚点符号为准）。

## §3 v1 范围

| 进 v1 | 推迟（v1.x / v2） |
|---|---|
| 统一框选交互：编排面 + 钢琴卷帘两视窗 | 其余视窗（频谱/调音台/媒体池/图架）轻量 marquee 统一层（v1.x） |
| 便签容器三态（打开/收回锚点/自由移动） | OS 独立窗口形态（v2 视需要） |
| 每 note 一会话，7878 主管道接线 | 会话池跨工程视图（v2） |
| note 上下文组包（rect+轨/clip/时间窗+视窗类型+音符组） | L2 视觉兜底（截图→识图，v1 零接线只留协议位，默认关 F13） |
| agent 侧：source 身份标注 + note 上下文消费（range_context 富化）+ RiskCeiling 执行闸 + 工程级写租约 | orchestration Worker 实例化 / worker_job 协议 / authority_conflict 队列（v2） |
| webui 框选上下文徽章 + 徽章跳 note 会话 | 便签池完整形态（检索/归档管理，v1.x，见 §9 拍板） |
| 液态玻璃 v1 视觉基线（半透明+轻模糊） | 折射/高光/动态液态形变（v1.x–v2 分期打磨） |
| 轨道监督常驻 note（语义在位：锚定轨+长生命周期） | 联机传阅（远期愿景，v1 零涉及） |

## §4 交互设计

### 4.1 统一框选手势（用户裁定形态的规格化）

状态机（编排面为基准实现，钢琴卷帘同协议）：

1. **框选完成** → 既有行为零改变：框内 clip 默认照旧选中（TimelineEditStore 选区写入不变）。
2. **形变提示**（新开面）：框选 rect 右侧弹出提示——自框边缘"生长"出的可点击胶囊（快捷键文案 + "打开便签"语义图标），spring 缩放 + 透明度渐入（简单 tween）。这是气泡（快捷键提示）体系第一次落地 = 框选形变提示（两设计线合流点）。
3. **三出口**：点击胶囊 / 按快捷键（默认 **N**，经 VitShortcutManager 注册、用户可配置；焦点守卫复用 `_has_text_input_focus` 模式——文本输入态不触发）/ 超时 **8s** 无交互淡出（选区保留，只是提示消退；再次框选或 hover 选区可重新唤出）。
4. **空区域框选**：选中集为空，rect 仍承载"这里"指代（时间窗从 rect×时间轴映射派生，依赖 §5.1 补的矩形组包出口 F3）。
5. **跨视窗协议**：提示形态与出口语义跨视窗统一；无框选能力的视窗（调音台/频谱/媒体池）v1 不做，v1.x 以轻量 marquee + 同一提示形态接入（RECON §1b 各视窗现状盘点为据）。

### 4.2 便签窗口（需新开面，三档先例中取最轻档=Control 级浮层）

- **挂载**：Control 级浮层（非 OS 窗口），挂视窗体系；出现位置 = 框选 rect 右上角外侧偏移（不遮辖区），空间上"属于"该框选。
- **结构**：头部（辖区摘要一行 = 轨数/clip 数或时间窗；收回按钮；拖拽把手）+ 输入区（LineEdit，F5）+ 输出区（滚动，消息流同主 chat 形态）。
- **三态**（用户裁定）：
  - **打开**：输入+输出一体 = note 本体；
  - **收回**：折叠为锚点小标签（辖区 rect 左上角材质小点/小条，见 §9 档 A），点击重开；收回 ≠ 归档（会话与状态保留）；
  - **随意移动**：拖拽把手自由重定位（松手即停，v1 无吸附；"漂浮粘贴于 DAW GUI 的空间信息层"）。
- **生命周期**：composing（打开未发首条）→ active（服务端已建会话）→ archived（任务完成或用户归档；归档 note 从活跃面消失，会话留在 history 图）。

### 4.3 输入与焦点路由（需新开面）

- 每 note 独立 LineEdit；焦点由 GUI 点击天然仲裁（单 focus owner 语义不变，F5）。
- 单 note 内一轮未完禁发（复用 `_set_send_enabled` 串行先例：生成中输入框禁发+进行态显示）；**note 之间不互斥**——多 note 并发收发是 v1 支持形态（F7 多会话并行天然成立）。
- v1 不设 note 数量硬限（§13 P4）；失控场景交给用户手动归档。

## §5 上下文协议（L1 结构化 provider；视觉只做意图理解、执行永远结构化命令）

### 5.1 request_context 扩展（Godot 侧组包）

新增 `vit_note` 块（与既有 selected_clip_ids/time_selection 等并列，走同一 `_build_agent_context` 组包通道，F1/F6）：

```json
"vit_note": {
  "note_id": "note_<id>",
  "origin_view": "timeline | piano_roll",
  "rect": { "x": 0, "y": 0, "w": 0, "h": 0 },
  "time_window": { "start_s": 0.0, "end_s": 0.0 },
  "track_ids": [], "clip_ids": [], "note_ids": [],
  "note_state": "composing | active"
}
```

派生规则：
- 有 clip 命中 → clip_ids/track_ids 直接取 marquee 命中字典（F2），time_window = 命中包络；
- 空区域 → time_window = rect × 时间轴像素-秒映射（**新增 marquee 原始矩形组包出口**，补 F3 缺口；选中集为空是合法态）；
- 钢琴卷帘 → note_ids（**新增音符组语义出口**：`_selected_note_ids` 进 request_context，补 F4 缺口）+ 所属 clip/轨上下文。

### 5.2 agent 侧消费（设计裁定：vit_note 是 range_context 的富化生产者，不开平行管道）

- `vit_note` 存在时，辖区（track_ids/time_window/clip_ids）作为**优先 range 源**并入现有 selection 语义消费链（semantic_entry 选区路由、range_context 四层管道，F6）——不新建第二套选区语义。
- note_id 透传进 loop 上下文（供 §7 权限闸与回执归属使用）。
- 首条消息未发（composing）无 agent 消费；active 后每条消息都带 vit_note 块（note 辖区随会话常驻，直到归档）。

### 5.3 L2 视觉兜底留口（v1 零接线）

- 协议位：request_context 预留 screenshot 工件引用字段（不实现采集）。
- 隐私默认态=关：未来接线时以多模态路由 fail-closed（F13）为"默认关"实现参照，截图外发需显式 user_approved 位（援引 browsercapture 先例）。唯一外发点=provider endpoint，遥测无内容（F13）。

## §6 会话架构

### 6.1 会话身份与生命周期

- 每 note 持独立 conversation_id，初始为空 → 首条消息服务端新建（F6）；note 与主代理会话在 history 对话图中并存（多会话图既有事实）。
- 刷新/重开前端：note 会话从服务端历史重建（与 webui 同机制）；note 辖区（rect 等）随会话记忆或工程态恢复——v1 简化：仅活跃会话恢复，收回态 note 的辖区标签从服务端会话元数据重建（不可重建则降级为无锚点列表项，不阻塞）。
- 常驻监督 note（轨道监督）：锚定轨、长生命周期，语义上=同一 note 长期活跃；v1 与临时 note 无实现差异（只是不归档）。

### 6.2 webui 同步与徽章

- **同步零新机制**：事件回放水合天然覆盖（F10）——note 的输入走 chat 主管道（概念裁定 4，禁止旁路触发），会话历史天然同步。
- **框选上下文徽章**（新开面，仿 FocusSummary 形态，F10）：主代理界面显示活跃 note 概要——note 数 + 各 note 辖区摘要行。
- **徽章=会话入口**：点击徽章条目 → `?conversation_id=<note 会话>`（F11）——webui 仍单活跃呈现，但可"看进"任何 note 的完整对话流。这是零新开面拿到"会话列表"核心价值的路径；完整会话列表 UI 留 v1.x。
- 端测边界：徽章属渲染面，须过 E2E-WEBUI-1 浏览器级 DOM 断言；基建未建前按 AGENTS §5 在回执中显式声明端测覆盖边界（F10）。

## §7 权限与写并发（v1 闸设计——本文档核心新增面，F7/F8 的直接回答）

### 7.1 权限分级（复用能力注册表面，不新发明权限系统）

- **note 权限档跟随工程 authority_mode**（不自建 note 级权限系统）：
  - `manual_confirmation`：RiskCeiling ≤ `bounded_reversible` → 直执（小操作）；RiskConfirm 级 → **升级提案给主代理**：webui 确认卡呈现"来自 note_x 的升级请求"（与确认卡体系/FIX-CONFIRM-CARD-1 合流，主代理是确认主体）。
  - `full_project_access`：直执 + 事后回执（与 FIX-CONFIRM-CARD-1 随卡下发的裁定方向一致——full access 语义=用户授予完全访问，RiskConfirm 不再前置）。
- 主代理不新增拦截（现状不变）；仅 note 发起的调用进入 §7.2 闸。

### 7.2 RiskCeiling 执行闸（新开面，补 F8"零执行消费方"缺口）

- **调用方身份标注**：chat 请求新增 `source: "main" | "vit_note"`（vit_note 带 note_id）——服务端在 handleChat 请求域标注并进 loop 上下文；webui 请求默认 main（存量客户端零改动）。
- **闸落点**：capabilityruntime 能力执行路径——执行前检查 `RiskCeiling × source`：source=vit_note 且 RiskCeiling=RiskConfirm 且模式=manual_confirmation → 拦截，生成升级提案（复用 pendingmanager 确认流，确认主体=主代理会话）；其余放行。
- v1 失败语义：闸内异常 fail-closed（不放行、报错上浮 note 输出区）。

### 7.3 工程级写租约（新开面，F7 直接含义的 v1 回答）

- **事实基线**：多会话并行真实发生（F7）；现存三闸（命令粒度/CAS/文件锁）保数据一致性、不保任务互斥——主代理与 note、note 与 note 并发写同一工程会互相踩（产品语义层冲突，非数据损坏）。
- **v1 设计**：per-project 写租约——所有 agent 会话（主 + note）执行**写类能力**（RiskCeiling 高于 observation 的能力执行段：装载/写参/渲染等任务原子段）前 acquire：
  - 粒度=单次能力执行（begin → commit/release），不跨轮持有；
  - 竞争者排队等待（不拒绝，等待态在 note/主代理界面可见，用户可取消）；
  - 超时自动释放（默认 120s，防悬挂死锁）；
  - 落点=executionruntime.Coordinator 执行前置（与 CAS/文件锁同层）；单 agent 进程内内存租约即可（v1 单机单 agent 实例事实）。
- **v1 明确不做**：跨工程、跨进程、优先级抢占、租约持久化、authority_conflict 队列（v2 项）。
- 观察类完全并行不受影响（F7：观察并行天然成立）。

### 7.4 authority_conflict

v1 不建队列（F8 现状=仅报告计数）；note 与主代理会话隔离使同 conversation 冲突不可能发生。记录为 v2 Worker 线任务。

### 7.5 辖区执行面边界与演进（2026-09-30 用户裁定后定稿）

vit note 的空间叙事在**上下文层**（辖区协议 §5.1）与**执行层**（能力落点）不是同时到位的，本节显式分期：

- **v1 执行面** = 轨级能力（六项 bounded_reversible 写参）+ 区间 audition A/B 验证。辖区上下文（clip_ids/time_window）先行传递——上下文先到、执行后到，note 对超范围操作诚实声明（"这是整轨生效"）。
- **clip_scope 开放（用户裁定 2026-09-30，F14）**：宪法"单 Clip 绑定仅限手动"修订为 agent 可经 RiskConfirm 确认流提案执行（竖线写入必伴随披露+可逆；rack_add_node 装载与绑定两步分离纪律保留）。落地=关联卡 CLIPSCOPE-AGENT-1（§10）。vit note v1.x 的辖区执行（"只处理这一段"）由此通达——note 提案竖线绑定→主代理确认卡→写竖线，与 §7.1 升级提案机制同构。
- **自动化（时间性处理："这段渐强/渐弱/段落能量变化"）**：按混音能力层既定序位=空间与深度（时域混响/delay/send，总规划 C3）之后的 C4 段落自动化；不因 vit note 提前，vit note 各版本不依赖。
- **演进图**：v1 观察问答/轨级 → v1.x 竖线辖区执行（CLIPSCOPE-AGENT-1）→ 能力线 C3 空间与深度 → C4 自动化（note"这段渐强"完全体）。

## §8 视觉规范（液态玻璃 v1 基线）

- **v1 基线**（对齐苹果 Liquid Glass 语言的克制版）：便签面板半透明底（α≈0.85 中性玻璃灰）+ BackBufferCopy 区域轻模糊（blur 近似半径 8–12px）+ 1px 高亮描边 + 圆角；**无折射/高光/动态液态形变**（v1.x–v2 打磨分期）。
- **三态视觉**：打开=完整面板；收回=锚点小标签（同材质小尺寸）；移动=拖拽中轻缩放 + 加深阴影。
- **色彩**：中性玻璃基底 + 辖区轨色微量着色（track color 采样）——空间归属感（便签"知道"自己属于哪条轨）。
- **形变提示**（§4.1）：简单 spring tween，不做液态形变。
- **性能约束**：模糊仅限便签矩形区域 BackBufferCopy；拖拽/移动中降级为纯半透明（免连续模糊重绘）；同屏多 note 模糊层数监控（超 3 个活跃 note 时新 note 降级为无模糊态——保帧率的诚实降级）。

## §9 便签池（两档方案——**待用户拍板 P1**）

| 档 | 形态 | 价值 | 成本 |
|---|---|---|---|
| A 锚点小标签 | 收回/归档 note 在辖区 rect 左上角显示材质小标签（辖区多 note 叠加计数；hover 浮层列 note） | 空间语义本体："一眼看到哪块辖区有 note"——vit note 核心叙事的直接呈现 | 中（随 IMPL-1/6 落） |
| B 侧边总列表 | DAW 侧栏便签池面板：活跃/休眠（收回）/归档三组 + 点击重开 | 全局管理：跨视窗找回 note、检索入口 | 中（v1 最简版=仅分组列表） |

- **建议（gate-2 同款）**：v1 = A + B 最简版（B 仅分组显示+点击重开，搜索/批量管理 v1.x）。
- **用户拍板（2026-09-30 晚窗："是的A+B都是需要的"）**：v1 = A + B 都做，B 按最简版口径（分组+点击重开）。
- 备选（v1 仅 A）不再适用。两档不冲突。

## §10 实现分期与 IMPL 卡骨架（写卡时按池序展开为完整卡面）

| 卡 | 域 | 内容 | 依赖 |
|---|---|---|---|
| VITNOTE-IMPL-1 | Godot·编排面 | 形变提示 + 便签容器三态 + LineEdit 输入 + 7878 会话接线（每 note 一 conversation_id）——**M8 最小可演示闭环** | 本文档（P2/P3 默认值可调） |
| VITNOTE-IMPL-2 | Godot·上下文 | marquee 原始矩形组包（F3）+ vit_note 块 + 钢琴卷帘音符组出口（F4） | IMPL-1 |
| VITNOTE-IMPL-3 | agent | source 身份标注 + note 上下文消费（range_context 富化）+ RiskCeiling 闸 + 升级提案（§7.1/7.2） | IMPL-2（协议字段） |
| VITNOTE-IMPL-4 | agent·写租约 | 工程级写租约 + 并发单测 + 真栈烟测场景（§7.3） | 无（可与 IMPL-1 并行，不同仓文件域） |
| VITNOTE-IMPL-5 | webui | 框选上下文徽章 + 徽章跳 note 会话 + E2E-WEBUI-1 断言（§6.2） | IMPL-3 |
| VITNOTE-IMPL-6 | Godot·视觉 | 液态玻璃基线 + 锚点小标签 + 便签池（按 P1 拍板）（§8/§9） | IMPL-1（可后置） |
| CLIPSCOPE-AGENT-1（关联卡，非 IMPL 序列） | agent+docs | 路由宪法修订：clip_scope 竖线向 agent 开放（RiskConfirm 确认+披露+可逆）+ prompt 纪律同步 + 回归测试（§7.5） | **独立可执行**，不依赖 IMPL 序列；服务 v1.x 辖区执行 |

并行域标注：IMPL-4（agent internal）与 IMPL-1/2（Godot 前端仓）天然不同文件域可并行；IMPL-3/4 同属 agent 仓但包域不同（capabilityruntime/executionruntime vs chat），写卡时再核。Webui 徽章与 FIX-CONFIRM-CARD-1/OPT-OBSERVE-OUTPUT-1 同为 webui 域——池序串行。

## §11 测试与验收

- **agent 侧（IMPL-3/4/5）**：go 单测 + 全量门 0 FAIL + 真实栈烟测 exit 0（AGENTS §5）；IMPL-5 另过 E2E-WEBUI-1 渲染断言（基建未建前显式声明端测边界）。
- **写租约并发用例**：两 goroutine 竞争同工程租约断言串行化 + 超时释放 + 等待取消；跨工程不互斥反例。
- **Godot 侧（IMPL-1/2/6）**：手测 M8 节点（判据见 manual-test-nodes）；冒测扩展走 scripts/dev_agent_smoke.ps1 体系加场景（复用既有机制不另起）。
- **M8 里程碑判据**：编排面单 note 走通"框选 → 形变提示 → 开便签 → 中文输入（IME）→ 观察问答（辖区上下文生效）→ 收回 → 重开"；加分项=双 note 并存互不干扰。

## §12 远期留口（不进 v1 排程，架构不封死）

- **联机传阅**：note 本质 = {空间锚, 对话流, 状态} 三元组，未来加 owner/audience 字段即协作消息（架构天然留口）。
- **Worker 实例化 / worker_job 协议**（v2，F8 差距清单）；authority_conflict 队列化。
- **回复即 DAW 对象**：note 回复直接成为可操作 GUI 对象（与判定卡/确认卡线合流）。
- L2 视觉兜底接线（截图→识图→空间意图，默认关）。

## §13 开放项与拍板清单

| # | 事项 | 建议 | 状态 |
|---|---|---|---|
| P1 | 便签池 v1 范围（§9） | A + B 最简版 | **已拍板（2026-09-30 用户：A+B 都要）** |
| P2 | 打开便签默认快捷键 | N（VitShortcutManager 可配置） | 默认可调，随手测反馈 |
| P3 | 形变提示超时 | 8s 淡出（选区保留） | 默认可调 |
| P4 | note 并发数上限 | v1 不设硬限 | 默认不设 |
| P5 | note 辖区跨重启恢复 | v1 仅活跃会话恢复，锚点不可重建降级为列表项 | 设计已定（§6.1），实现时验证 |
