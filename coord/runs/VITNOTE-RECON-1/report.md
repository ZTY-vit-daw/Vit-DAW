# VITNOTE-RECON-1 勘察报告：vit note 前置勘察（八节）

- 执行：2026-09-29 晚，Windows 端会话（用户口令指定），零代码改动，全程只读
- agent 仓勘察基线：worktree `D:/Vit_DAW_worktrees/vitnote-recon-1`，HEAD = `3e3cc7f5`（领取提交 `8e371ee8` 之后 main 上的通用性裁定版卡面；勘察期间无再前进）
- Godot 前端（仓库外）：`D:\Godot\project\vit-daw-frontend`，工作树 HEAD = `5280822`（port 分支），有未跟踪运行时目录；**行号以当日该工作树为准**
- 卡面：coord/cards/doing/2026-09-29-VITNOTE-RECON-1.md（通用性裁定版：框选+输入全局通用；L1 结构化 provider 优先 / L2 视觉兜底；视觉只做意图理解、执行永远结构化命令）
- 三态图例：**支持**（现有代码可直接承载）/ **不支持**（无现成实现）/ **需新开面**（有先例可仿但需新代码）

---

## §1 Godot 全局框选与便签容器面

### 1a. 全局坐标层框选事件可行性 —— 结论：**部分支持，跨视窗统一层需新开面（编排面内已有"全局 rect 广播"先例）**

编排面 marquee 本身就建立在全局坐标层上，且已有跨节点广播协议：

- 手势状态机：`vit_track_lane_2d.gd`（4456 行）`_begin_marquee_press`(:2257)/`_update_marquee_gesture`(:2278)/`_finish_marquee_gesture`(:2296)/`_cancel_marquee_gesture`(:2310)；4px 起拖阈值(:63 `_MARQUEE_DRAG_START_THRESHOLD_PX`)；仅 VIEW_WAVEFORM 模式、避开时间尺区(:2259-2261)
- **全局 rect 广播协议**：`set_global_marquee_preview(global_rect, is_active)`(:2442-2447)——任意 lane 可被外部驱动绘制框选预览；`_broadcast_marquee_preview`(:2449-2460) 把全局 rect 广播给 `vit_track_lane_2d_interaction` 节点组所有成员；坐标换算 `_marquee_rect_global`/`_local_rect_to_global_rect`(:2456-2464)。这就是"不依赖视窗内部结构的统一框选层"的现成局部实现——局限：节点组成员目前只有轨道 lane，非任意视窗
- **跨轨命中收集**：`_clip_ids_intersecting_global_marquee`(:2394 起) 遍历节点组各 lane 的 `get_clip_marquee_hits_global`(:2411)，命中字典 = `_clip_bounds_dict`(:1581-1589：clip_id/start_seconds/end_seconds/length_seconds/offset_in_source_seconds) + `track_id` + `track_y`(:2434-2436)，按 track_y→start_seconds 排序——**轨×clip×时间窗语义在命中层已经算好**
- 全局输入仲裁先例：`app/timeline/TimelineInputArbitrator.gd`（1193 行）以全局点做命中仲裁——`_is_global_point_inside_timeline`(:175)/`_should_accept_mouse_at_global`(:211)/`_global_point_targets_timeline_non_lane_interactive_control`(:227)；:25 注释明确处理 SubViewport gui_input 与 Track_List._input 同帧双投递
- 选区状态持有者：`app/timeline/TimelineEditStore.gd`（668 行）三套并存——clip 集合 `set_selected_clip_ids`(:188, signal selection_changed :5)、时间窗 `set_time_selection`(:233, signal :10, getter :263)、子范围 `add_selected_clip_range`(:275, signal :11)

### 1b. 各视窗框选/选择交互现状盘点

| 视窗 | 现状 | 锚点 |
|---|---|---|
| 编排面（主战场） | **框选完整支持**：跨轨 marquee+全局 rect 广播+恢复快照(:2354-2368)+绘制(:1382) | vit_track_lane_2d.gd（上行全列） |
| 钢琴卷帘 | **框选支持（lasso 术语）**：`_is_lassoing` 拖选+additive(shift/ctrl/meta :1567)+音符节点 rect 相交(:1591-1625)；`_selected_note_ids`(:87)；排除 NOTE_STATE_PREDICTED(:1603)。**音符组选区未接 agent 上下文**（`get_selected_note*` 全项目零命中）——L1 音符组语义需新开面 | PianoRollView.gd（2008 行） |
| NL→MIDI 工具族（L1-5 佐证） | `add_midi_notes` 命令链已在：clip_client.gd / piano_roll_midi_rules.gd / TimelineInputArbitrator.gd / PianoRollView.gd(:1645-1664 建音+ipc 发送) | 上列 |
| 调音台 | 无框选；strip 选中由工程选区驱动 `set_mixer_strip_selected`（:11 注释言明"时间线选中轨后由 adapter 调用"）——选择数据面存在 | vit_dock/views/mixer/vit_mixer_channel_strip.gd:11,43-45 |
| 频谱 | 无框选；轨道级单选，从 `_project_repository.get_selection()` 读(:216-233) | spectral_dock_adapter.gd:28,216-233 |
| 媒体池 | 文件列表选择存在（18 处 select 命中），无框选 | app/browser/left_library_dock.gd |
| 图架/其他 | 有 select 命中（参数/节点选择） | Vit_Graph_Rack.gd 等 |

符合通用性裁定预期：L1 结构化 provider 在编排面数据最厚；其余视窗多为仓库级/列表级选择，框选交互不存在（用户已裁定存在性确认即止）。

### 1c. 便签容器可行性锚点 —— 结论：**需新开面，三档先例齐**

- Control 级浮层：`app/ui/vit_tooltip_trigger.gd`（24 行，可挂任意 Control）；`PianoRollClipOverviewHUD`（Control 内绘浮层，_draw :29）
- 视窗体系：`vit_dock/core/viewport_registry.gd` VIEW_DESCRIPTORS(:5-63)——7 类视窗（timeline/graph_rack/piano_roll/spectral/detail/mixer/media_pool）带 `allow_multi_instance`/`supports_track_filter`/`supports_focus_anchor`/`spawnable_in_viewport` 描述符——**"selection semantics provider"按视窗类型注册的天然挂点**（架构留视窗注册制的现行实体）
- OS 窗档：`vit_dock/core/window_manager.gd` popout_viewport(:16)/reattach(:57)；`detached_viewport_window.gd`（extends Window）
- HUD/tooltip/气泡浮层均为 Control 体系内实现，无 Window 依赖——便签原位浮窗走 Control 层可行

### 1d. 框选→语义解析挂点（L1 requestContext 组包现状）—— 结论：**支持（轨/clip/时间窗三要素端到端已在）**

`LLM_Chat_Controller.gd::_build_agent_context()`（:4530 起，8133 行文件）：经鸭子类型 host（`_find_agent_context_host` :4791-4810，先祖先后全树递归）采集并组包：`selected_track_id`/`selected_scene_track_id`/`selected_track_name`/`mix_track_options`/`playhead_seconds`/`selected_clip_ids`/`selected_clip_id`/`selected_clip_track_id`/`time_selection`+`listen_time_start/end_seconds`/`selected_clip_ranges`。host 实现位于 vit_dock_root.gd(:3461,:3597)、timeline_dock_adapter.gd(:1471,:1490)、legacy vit_control_v_1.0.gd(:1041,:1080,:1111)。**marquee 写 TimelineEditStore → host getter → request_context 链路现行存活**；缺口：marquee 自身的原始矩形不组包（空区域框选无 clip 命中时不产生时间窗——time_selection 来自另一条时间拖选手势），音符组上下文无出口。

---

## §2 IME 与文本输入 —— 结论：**支持（生产实证）；多 note 焦点路由需新开面**

- 主 chat 输入框即 LineEdit：`LLM_Chat_Controller.gd:78` `@onready var input_box: LineEdit`——项目日常中文对话的生产输入面（Godot 4 LineEdit 原生 IME，Windows 端实测在用，M1 手测经此框）
- 其他输入框先例：artifact_panel(3 处)、vit_render_export_popup(3)、project_marker_list_panel/marker_lane(各 2)、track_group_list_panel(1)、rack_param_surface_panel_factory(1)、Vit_Graph_Rack 重命名(1)、PianoRollView(1)、browser 面板(1)——命名框/数值框形态齐备
- 焦点守卫模式：`_has_text_input_focus()`(:4372-4378)——`gui_get_focus_owner()` is LineEdit/TextEdit/RichTextLabel 即判定文本态；`_chat_focus_owner_path()`(:287-294) 供输入事件诊断(:261-276 escape fallback)
- 快捷键冲突面：`vit_shortcut_manager.gd`（217 行）只做 InputMap 注册+文案拼装（get_shortcut_text :18），不自持按键分发——冲突治理落在各消费点的 focus 守卫上，便签可复用同一模式
- 发送期客户端串行先例：`_send_user_message` 运行中 `_set_send_enabled(false)`(:4522 附近)——单会话内一轮未完禁发；多 note 并存时"哪个 note 持有输入焦点"的路由策略是新问题（单 focus owner 语义），需新开面

---

## §3 chat 主管道接线面 —— 结论：**支持（入口/组包/新建会话全在）；注意 chat 入口是 7878 HTTP 非 vsp_hub**

- 前端入口（Godot）：`LLM_Chat_Controller.gd:12-17` 六端点常量——`AGENT_CHAT_URL=http://127.0.0.1:7878/agent/chat` 及 state/config/confirm/interaction/respond/invoke；`_on_send_pressed`(:3960)→`_send_user_message`(:4487)；payload = `{conversation_id, message, attachments?, context}`（:4508-4516，context 即 §1d 的 agent context）；`_post_json`(:5129)
- **口径修正（卡面 vs 实况）**：卡面写"vsp_hub WS/HTTP 面"——实际 chat 消息入口是 Go agent 的 chat server HTTP 面（7878）；vsp_hub/vsp_client 是 agent↔内核的数据面（ZMQ tcp://127.0.0.1:5555，kernel/client.go:24；vsphub 另有 HTTP/WS 供资产/实时流）。vit note 输入应走的"chat 主管道"= 7878 HTTP，与概念裁定第 4 条一致
- 服务端：`internal/chat/server.go` Routes()(:541-576+)，`/agent/chat`→`handleChat`(:2231)；**空 conversation_id → 服务端新建** `"chat_"+randomID()`(:2251-2253)——"每 note 一对话流"的前端侧只需各持独立（初始为空的）conversation_id；conversationGoals map+s.mu(:2286-2291)
- 前端会话生命周期：`_conversation_id` 首答由服务端赋值(:2223)；历史切换(:4304,:4316)；分支栏菜单"从这里继续/创建分支/创建工作树"(:429-433)
- range_context 管道（IMPL-C R6 烟测面）：`/smoke range_context`→`smokeRangeContextResponse`(server.go:5088-5127) 四层断言——top-level `selected_clip_ranges` / `current_selection.selected_clip_ranges` / `ui_context.selected_clip_ranges` / prompt_context（`agentContextForPrompt` 渲染含 duration_seconds）
- 选区键的深层消费：semantic_entry.go:113（selected_clip_id/piano_roll_focus_clip_id/selected_clip_ids 入口路由）；semantic_compressor_workflow.go:248-249,382,399,468（time_selection/listen_time_*）；free_state_reasoning_loop.go:2216；goalrunner_chat.go:3738（选区+自然语言混音关键词路由）——L1 选区语义已渗入 agent 决策层

---

## §4 agentloop 多会话并发模型 —— 结论（代码级实锚，非推断）：**多会话并行——每个 /agent/chat 请求在自己的 HTTP goroutine 上同步执行完整 agentloop；不存在全局单循环串行化。写冲突不天然串行化：串行化只发生在（a）内核命令粒度（ZMQ REQ/REP 逐命令）、（b）orchestration durable store 文件锁粒度、（c）CAS 授权消费粒度。**

证据链（按执行路径排序）：

1. **HTTP 层**：`cmd/vitagent/main.go:74` `Handler: chatServer.Routes()`，:80 `httpServer.ListenAndServe()`——Go 标准库，每请求一 goroutine
2. **chat 层无自有调度**：`internal/chat/server.go`（7271 行）全文 `go func` 计数 = 0（grep 实测）；`handleChat`(:2231) 在请求 goroutine 内同步调用 `beginChatGoal`(:2284)→`runAgentLoopChat`(goalrunner_chat.go:126)；`agentloop.Runner.Start`(runner.go:151)→`loop`(runner.go:377) 全同步——**请求返回即本轮 loop 结束**
3. **无并发 chat 互斥/拒绝**：全 chat 包仅 processor certification 有 busy 冲突响应（processor_certification_entry.go:443）；对并发 chat 无全局闸
4. **`beginContinuationSensitiveInvocation` 是计数器不是锁**：continuation_scheduler.go:845-859（`activeRuntimeInvocations++`）；用途见 :861-866 注释与 `invocationsActive`(:867-874)——后台续跑调度器不得在请求持有内存运行态时做盘重载（`claimNextContinuation` :889-893 检查后退让），**不串行化用户 chat 之间的并发**
5. **共享态以 s.mu 保护的 map 并存**：conversationGoals/conversationMemory/goalContinuations/pendingMixTicks/pendingTreatments 等按 conversation_id 键（server.go:75-99）——多会话状态并存的一手证据
6. **会话所有权按 conversation 仲裁**：`controllerOwners.Active(conversationID)`（orchestration_controller_host.go:15-21）；"conversation is already owned by %s"(:76-79)；goalrunner_chat.go:36-52（同 conversation 续用未终局 goal，terminal 后新建）——所有权域=会话，非全局
7. **写路径串行化点一（内核命令粒度）**：`kernel.Client.SendRaw` 每命令新建 ZMQ REQ socket（kernel/client.go:39-40，dial→send→recv→close）——每条内核命令独立 REQ/REP 往返，串行化发生在 C++ 内核侧 REP 处理（命令级交错，非任务级排队）；client 的 `c.mu` 只护 sessionID/featureFlags 缓存（vsp.go:72-75,92-94,102-104）
8. **写路径串行化点二（CAS+文件锁）**：`executionruntime.Coordinator.ExecuteWithPersistence`(coordinator.go:45+)——frozen plan hash+ProjectCut hash 校验+授权一次性消费(:76-83)；durable orchestration store 有 OS 级独占文件锁 `acquireStoreFileLock`（filelock_windows.go:14+，CreateFile 零共享句柄，5s 超时）——跨进程 store 串行
9. **后台 goroutine 全景**（chat 包内仅三处，均非用户 chat 本体）：续跑调度器 ticker 循环（continuation_scheduler.go:753-770，且受第 4 条退让约束）；c2 工程处理只读 worker 池（c2_project_treatment.go:568,607）；harness 遗留工程迁移触发（harness.go:2862，30s 超时内核分析队列）
10. **多 conversation 持久化**：history 包 ConversationGraph——节点/分支/剪枝（history.go:834 AppendConversationNode、:974 ConversationMessages、:1361 pruneEmptyDeletedBranches、:1783 activeConversationPath、:1881 writeConversationGraph）；健康检查套件中的分支检出/worktree 测试佐证多会话图并存

**对 vit note 的直接含义**：多个 note 同时发消息会真的并行跑各自的 agentloop（观察并行天然成立）；"写按 RiskCeiling 分级"不能指望 loop 层天然串行化——需要在能力/工具执行层另建闸（见 §6 RiskCeiling 无执行消费方）；现存的写安全三闸（命令粒度/CAS/文件锁）保护的是数据一致性，不是任务互斥。

---

## §5 webui 同步面 —— 结论：**刷新同步=支持；会话列表与上下文徽章=不支持（需新开面）**

- 会话身份：`App.tsx:279` conversationID state；URL `?conversation_id=` 优先否则新建(:1704-1711)；localStorage 分桶键含 conversationID+scope(:320,:386-405)，前缀 :205
- **无会话列表/切换 UI**：webui 呈现单活跃会话；scope=project/worktree/branch（historyScope.ts:51-77）决定存储桶——多 note 流的列表呈现载体需新开面
- 同步机制（概念裁定第 4 条的现行实体）：/agent/events 回放水合——App.tsx:603-616（终局消息服务端图水合+since=0 全量回放）、lib/api.ts:74-80 fetchAgentEvents；**webui 刷新/新开带 conversation_id 即可重建会话**（historyScope.ts:123）——"webui 刷新即见"的机制已在
- webui 侧组包：`buildChatContext`(App.tsx:12281+) 从轮询态 uiState 组 `current_selection`（selected_track/clip id(s)/piano_roll_focus/playhead）——与 Godot 侧 request_context 同构、数据源为服务端态（fetchUIState lib/api.ts:115-116 → /agent/ui/state）
- **上下文徽章不存在**：App.tsx 全文无 range_context/selected_clip_ranges/time_selection 渲染命中；最近先例=FocusSummary 摘要卡（App.tsx:1717+，按 focus 模式显示轨道数/当前轨道/机架节点行）——框选上下文徽章小卡需新开面，可仿 FocusSummary 形态
- E2E-WEBUI-1 覆盖方式：当前**无** playwright/e2e 基建（package.json scripts 仅 dev/build/preview/test=vitest；无 e2e 目录/配置）——徽章小卡的渲染面验收须待 E2E-WEBUI-1 建设卡落地时纳入浏览器级 DOM 断言；在此之前按 AGENTS §5 渲染面裁定，涉徽章改动须在回执中显式声明端测覆盖边界

---

## §6 orchestration 子代理底座 —— 结论：**协调底座半在：能力注册/引擎仲裁/CAS/文件锁在；Worker 零实现、RiskCeiling 零执行、authority_conflict 零队列——三处需新开面**

- **OwnerPolicy 语义澄清**：owner_policy.go（77 行）`Select(capabilityID, explicitV1, legacyPending, existing)`(:50-77) 仲裁的是**引擎归属**（EngineLegacy vs EngineV1，rollout 经 env `VIT_CAPABILITY_RUNTIME_V1_ROLLOUT` :31-33），不是会话所有权；`OwnerConflict="authority_conflict"`(:16) 在 legacy pending 撞上 V1 既有会话时置 Conflict=true(:57-60)——**全仓唯一消费是 capability_cutover.go:153 的报告计数**，无队列化处理（队列化需新开面）
- 会话所有权的真实现：orchestrationcontroller Registry（`controllerOwners.Active(conversationID)`，见 §4 第 6 条）——**vit note 的扩展点：按 conversation_id 的所有权域已天然存在**；`CapabilityInvocation` 自带 `ConversationID` 字段（types.go:91 附近）；`ParentControllerLink`（types.go:105+，注释言明"grants no authorization"）是父子关联/恰好一次交接的元数据先例
- **RiskCeiling 现状**：CapabilityDefinition 结构（types.go:80-90）含 RiskCeiling 字段；builtins.go 六项能力全部声明 `"bounded_reversible"`(:13,:22,:31,:40,:49,:58——static_balance/pan_layout/low_end_relation/frequency_cleanup/dynamic_control/eq_control)；**全仓 grep 无任何执行消费方**（仅声明+类型）——它是元数据不是运行闸；vit note"默认 bounded_reversible 写权、超限升级主代理"的分级执行面需新开
- **Worker 概念 vs 代码差距清单**（历史文档查证口径）：docs/PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md §5.4(:273-295)——Worker=能力可选计算承载、v1 不要求每能力建子代理；worker_job.v1 是"建议的后续协议"(:291+)。代码侧：`worker_job`/`WorkerJob` 全仓零命中——**Worker 实例化=0、worker_job 协议=0**。已在的替代物：PlanningSession 持久控制记录（文档 :209 明言"不等于聊天线程，不等于 Worker job"）、CAS 协调器、store 文件锁
- 落地节奏印证：概念裁定"vit note v1=子代理语义、单代理实现（独立对话流+读写分级），v2=Worker 真实例化"——与本节差距清单一致（v1 可行面=conversation 隔离+权限分级新闸；v2 待建面=Worker）

---

## §7 鼠标气泡底座 —— 结论：**不支持（鼠标跟随气泡不存在）；"空间感知底座"共享层需新开面（两套既有机制可作素材）**

- 现有"气泡"是固定位置提示行：`vit_context_manager.gd`（151 行）——Control 悬停 LIFO 栈(:85-103 入栈/出栈)+80ms 防抖(:8,:118-121)+`context_hint_changed` 信号(:6)；消费端 `vit_agent_shell.gd:61,219` `_on_context_hint_changed`→GHOST 态 `_hint_label` 固定胶囊+淡入淡出(:225-238)——**无鼠标坐标跟随**
- 注册面：`vit_tooltip_trigger.gd`（24 行）挂任意 Control 注册 action+描述（经 VitShortcutManager 拼装快捷键前缀）——通用控件挂点先例，hover 快捷键提示层的现行实体
- 共享底座评估：现存两套互不相通的空间机制——(a) VitContextManager hover 栈（Control 事件，无坐标）；(b) marquee 全局 rect 广播（全局坐标，见 §1a）。"框选触发层+hover 提示层共用一个空间感知底座"需要新的统一接口（控件注册+全局 rect+selection semantics provider 三合一）——需新开面；两套机制分别贡献事件模型与坐标模型

---

## §8 视觉兜底面（L2）—— 结论：**llm 多模态客户端=支持（已建已测、零生产接线）；Godot 截图=需新开面；隐私护栏先例=部分支持**

### 8a. 截图基建现状

- `internal/browsercapture`（capture.go，170 行）：`CapturePage`(:12)/`CaptureFetchResult`(:46)/`CaptureSearchResult`(:79)——**网页文本采集**（selection_text/text，30k rune 压缩 :16）入 artifacts store；Kind="web_page"、Source="browser"；metadata 含 `user_approved: true`(:35) 与 `capture_reason`——**用户批准语义的隐私先例**；前端配套 BrowserCaptureClient（browser_capture_client.gd:5,:16，POST `/agent/browser/capture`）；服务端 route server.go:576、handler :803
- artifacts 图像支持：`internal/artifacts/extract.go` 导入 image/png+jpeg+gif(:11-14)，`extractImage`(:452) 解码图像配置做校验——**图像工件可存可校验**，L2 截图工件的存储面在
- **Godot 侧视窗截图：项目内零实现**——无 `get_texture().get_image()`/screenshot 用法（app/+vit_dock/ grep 零命中）；Godot 4 API 本身支持（`Viewport.get_texture().get_image()`），需要新封装（视口级/区域级）——需新开面

### 8b. llm 客户端多模态

- 类型：`ImageInput{MIME, DataBase64, URL, Detail}`(client.go:34-39)、`VisionRequest{Prompt, Images, Metadata}`(:41-45)
- 调用：`CompleteImageUnderstanding`(:244)——路由取 `cfg.MultimodalRoutes["image_understanding"]`（config.go:17；DefaultMultimodalRoutes 声明 10 类路由：image_understanding/image_generation/audio_understanding/transcription/video_browser 等，默认 Provider="default"）；消息体 `visionRequestBody`(:430+) 双形态——responses API `input_image` / chat API `image_url`，base64 合成 data: URL（`imageInputURL`，detail 缺省 auto）；SSE 流解析+遥测+10min 超时；路由缺失 fail-closed（`ErrImageUnderstandingRouteUnavailable` :102）
- **零生产调用方**：`CompleteImageUnderstanding` 全仓仅 client.go 定义+client_test.go 两个测试（:226,:276）引用——**能力在库、在测、未接线**；L2 兜底（截图→识图→空间意图）需把它接到工具/agent 面——需新开面

### 8c. 多模态调用成本与隐私面（截图外发路径清单，供 DESIGN 定开关默认态）

外发路径盘点：
1. **唯一图像外发点 = 多模态 provider endpoint**（`routeCfg.BaseURL` + Bearer API key，client.go:290-292 一带）——截图一旦走 CompleteImageUnderstanding 即出本机
2. 本地 artifacts store：图像落盘不上传（extract 校验本地文件）
3. 遥测 JSONL：仅指纹/计时/usage（telemetry.go:16-29，PromptFingerprint/SectionStats），**无图像无正文内容**——本地观测面不泄漏
4. browsercapture 既有 `user_approved` 元数据语义可援引为截图外发的批准位

开关现状：无显式"视觉兜底"开关；多模态路由默认指向 default provider（即与文本同端点）；DESIGN 定默认态时可用 fail-closed 路由缺失行为作为"默认关"的实现参照。

---

## 汇总三态表

| 节 | 面 | 三态 |
|---|---|---|
| §1 | 编排面框选→语义链（轨/clip/时间窗） | 支持 |
| §1 | 跨视窗统一全局框选层 | 需新开面（编排面内全局 rect 广播+输入仲裁两先例） |
| §1 | 钢琴卷帘框选 / 音符组→agent 上下文 | 框选支持；语义出口需新开面 |
| §1 | 便签容器（原位浮窗） | 需新开面（Control 浮层/视窗注册制/OS 窗三档先例） |
| §2 | LineEdit 中文 IME | 支持（生产实证） |
| §2 | 多 note 输入焦点路由 | 需新开面 |
| §3 | chat 主管道+selection 组包+服务端新建会话 | 支持（注意入口=7878 HTTP，非 vsp_hub） |
| §4 | 多会话并发 | 多会话并行（代码实锚）；写串行化只在命令/CAS/文件锁三粒度 |
| §5 | webui 刷新同步（事件回放） | 支持 |
| §5 | webui 会话列表 / 框选上下文徽章 | 不支持，需新开面（E2E-WEBUI-1 基建亦未建） |
| §6 | conversation 所有权域+能力注册+CAS+文件锁 | 支持 |
| §6 | Worker 实例化 / RiskCeiling 执行闸 / authority_conflict 队列 | 不支持，需新开面（三项零实现） |
| §7 | 鼠标跟随气泡 | 不支持（现为固定提示行）；共享空间底座需新开面 |
| §8 | llm 多模态客户端 | 支持（未接线） |
| §8 | Godot 视窗截图封装 | 需新开面 |
| §8 | 隐私护栏 | 部分支持（user_approved 语义+遥测无内容+fail-closed；默认开关待 DESIGN） |

## 端测边界声明

本卡为零代码只读勘察，无任何代码/脚本改动，不涉 agent 侧生产改动验收门槛（AGENTS §5 真实栈烟测不适用）。报告全部结论来自静态代码阅读；Godot 前端为仓库外工作树，行号随其后续提交漂移，报告已标注基线 HEAD（agent=3e3cc7f5，Godot=5280822）。IME 支持结论引用的是既有生产输入框的日常使用事实，本卡未新增任何交互实测。
