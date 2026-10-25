# M3-ENVELOPE-LANE-RECON-1：包络车道 UI+机架状态面实锚勘察报告

- 卡：coord/cards/todo/2026-10-25-M3-ENVELOPE-LANE-RECON-1.md（池序 186）；执行：flash 勘察会话（GLM-5.3-flash）@ PC / 2026-10-25 晚窗
- 基线：主仓 origin/main=e6456b6d（领取时）；**勘察对象=前端仓 D:\Godot\project\vit-daw-frontend main=5da6360fb1450ad69a8582705ae265e9ceadf87c（只读，零改动，git status 仅既有未跟踪工程目录）**；内核锚点按 M3-KERNEL-1 done 卡回执在本仓 main 亲核
- 方法：grep+读源码，零代码改动、零真栈启动、零探针；全部锚点当日亲核，标 文件:行号（前端仓锚点未注前缀者均相对 `app/`）
- 端测边界声明：纯只读勘察零代码零真栈，不适用端侧烟测门槛；不占真实运行栈
- 结论一句话：**时间线主场=虚拟行容器→track_row.tscn→Track_Row_Controller→TrackLane2D（工厂挂载点单点 `_ensure_lane_2d_layer`）+全屏 TimelineInputArbitrator 按 hit-kind 词表分派，包络车道最小侵入点齐备；曲线数据可零新面走既有 legacy 命令通道（get_curve 已入只读白名单）；183 恰一笔三段式（拖动零发包/释放恰一笔/回显门）可整体移植；机架侧播放头联动零既有钩子（登记新建面）、「参数有包络」在场性数据源在内核 get_plugin_parameters 回执缺字段（标注点）；已合入的 automation.set_points 无 base_revision 参数，现役 CAS 实际在 VSP 信封层（标注点）。**

---

## 一、问1：时间线主场结构实锚

### 1.1 行渲染归属与行结构

| 锚点 | 内容 |
|---|---|
| `timeline/timeline_track_list_drop_router.gd:657-688` | `_ensure_virtual_master_row`：行从 `TRACK_ROW_SCENE`（track_row.tscn）实例化；master 行 `custom_minimum_size.y = VIRTUAL_MASTER_ROW_HEIGHT`（:677）+ `set_timeline_playhead_visible(false)`（:679-680）；行输入绑定 `_bind_row_input_clip_host`（:681） |
| `vit_dock/views/timeline_dock_adapter.gd:2716-2733` | 非虚拟刷新路径：`_get_or_create_track_row`（:2716/:2959 定义，:2986 `_create_track_row`）→ `apply_kernel_track_state`（:2727）→ `_apply_track_row_layout`（:2729）→ `_restore_track_clips_to_row`（:2732）→ `replace_committed_clips`（:2737） |
| `TimelineViewMapping.gd:330-342` | 行内节点路径权威：`Track_Split/Right_3D_Wrapper`（wrapper）、canvas=可见时 `.../TrackLane2D` 否则 `.../Viewport_3D_Zone`；`_row_clip_edit_provider` :345-354（lane 优先、Tile_Manager 兜底） |
| `TimelineInputArbitrator.gd:119-124` | 同一节点路径的第二引用（clip edit provider 解析） |
| `Track_Row_Controller.gd:140` | 3D 播放头游标 `_playhead_cursor`（MeshInstance3D @ Tile_Manager/Playhead_Cursor） |

### 1.2 车道类型枚举/工厂挂载点（新增「包络车道」的最少侵入面）

**双层枚举现状**（包络车道三案共用基座）：

- lane 内部视图枚举：`tracks/track_scene/vit_track_lane_2d.gd:11-12` `VIEW_WAVEFORM := 0` / `VIEW_SPECTRUM := 1`；`_view_mode` :93；setter clampi `clampi(mode, VIEW_WAVEFORM, VIEW_SPECTRUM)` :582-584。
- 行级视图枚举：`tracks/track_scene/Track_Row_Controller.gd:28-29` `LANE_VIEW_WAVEFORM := 0` / `LANE_VIEW_SPECTRUM := 1`；`_lane_view_mode` :189；按钮与可见性联动 :1308-1316（playhead cursor 只在 WAVEFORM 显示）。
- **车道工厂单点**：`Track_Row_Controller.gd:1033-1063` `_ensure_lane_2d_layer`——`_TRACK_LANE_2D_SCRIPT.new()`（:1041）→ name="TrackLane2D" → `wrapper.add_child` + `move_child(mini(2, ...))`（:1044-1046）→ `PRESET_FULL_RECT`（:1048）→ `bind_track_row` / `set_kernel_track_id` / `set_timeline_viewport_active` / `bind_timeline_edit_store`（:1053-1061）。**新增第三 lane 实例=此函数扩一个 else-if 分支**；新增第三视图模式=双层枚举各加一值+按钮（`_ensure_lane_view_bar` :1065 起）。

三案（供 C2 卡面裁定，本勘察不定案）：

| 案 | 形态 | 侵入面 |
|---|---|---|
| A 第三视图模式 | 行视图=波形/频谱/**包络**三选一 | 双层枚举 +1 值、setter clamp 上界、view bar 加按钮；无新节点类型；同文件 vit_track_lane_2d.gd + Track_Row_Controller.gd |
| B 行内分区 | 行内波形 lane 下方常驻包络子条 | 行高管理改（1.4）+lane 布局改；双 lane 同行并存，绘制/命中需行内 y 分区 |
| C 独立 lane 实例 | TrackLane2D 旁挂 TrackEnvelopeLane2D | 仅 `_ensure_lane_2d_layer` 扩分支+新文件；行高联动另计 |

### 1.3 播放头↔视图既有同步链

1. 数据源：`kernel/autoloads/vsp_realtime_adapter.gd:7` `STREAM_TRANSPORT := "transport.playhead"`（30Hz latest_only，订阅流 `_desired_streams` :573-579）；`_apply_transport_data` :504-515 → `GuiTransportClock.apply_vsp_transport_frame`（`kernel/autoloads/gui_transport_clock.gd:147`；`gui_transport_updated` 信号 :3；`get_position_seconds` :166）。
2. lane 消费：`vit_track_lane_2d.gd:4043-4055` `_current_transport_seconds` 三级回退（GuiTransportClock → VitTelemetryManager kernel 位置 → wrapper `get_current_transport_seconds`，`right_3d_wrapper.gd:604`）。
3. 时间↔x 映射：`vit_track_lane_2d.gd:1470-1475` `_time_to_local_x`（经 `_visible_range()` 视口秒窗线性映射 size.x）/ `:1477-1482` `_local_x_to_timeline_seconds`；全局点入口 `timeline_seconds_at_global_point` :1484。视口秒窗权威=`right_3d_wrapper.gd:1334` `get_visible_timeline_range_seconds`，store 缓存 `TimelineEditStore.gd:76/:140`（committed_visible_range）。
4. 绘制：`vit_track_lane_2d.gd:4285-4299` `_draw_playhead`（:1383 在 `_draw` 内调用）；重绘节流 :1116-1126（playhead 秒变 >0.001 或 live motion 才 queue_redraw）；可见性门 `set_timeline_playhead_visible` → :626-630。
5. 编辑（拖播放头）：hit-kind `playhead_band`（`TimelineViewMapping.gd:124-133`，`wrapper.timeline_hit_test_playhead_band` `right_3d_wrapper.gd:1346`）→ 仲裁器 `_begin_playhead_gesture` `TimelineInputArbitrator.gd:932-956` → wrapper `timeline_begin/update/end_playhead_drag`（`right_3d_wrapper.gd:1350/:1366/:1378`）→ lane scrub `_update_playhead_scrub` :2112-2116（节流发包 `_playhead_scrub_last_send_ms` :157/:2106）→ `_seek_transport_to_time` :2137 → `kernel/clients/transport_client.gd:37-38` `seek(time_seconds)`（`{"cmd": "seek", "time": 秒}`）。

### 1.4 车道高度/布局管理

- 行高三档：`timeline_dock_adapter.gd:3011-3021` `_apply_track_row_layout`——MASTER/FOLDER/DEFAULT（常量 DEFAULT_TRACK_ROW_HEIGHT=240 等，文件头部 const 区）；虚拟化行高同值 `timeline_track_list_drop_router.gd:22-27`（240/64/112 + env 覆盖 `VIT_GUI_VIRTUAL_TRACK_ROW_HEIGHT` :436-437 + user override :77-78）。
- lane 层=PRESET_FULL_RECT 铺满行（`Track_Row_Controller.gd:1048`）。**行高即车道总高**；案 B（行内分区）须改此函数与虚拟行高常量，案 A/C 不必。

### 1.5 TimelineInputArbitrator 对车道内点选/拖动的既有分派形态

- 形态：顶层全屏盖板 Control（`TimelineInputArbitrator.gd:38-43` top_level+set_process_input），**悬停位置动态切 STOP/IGNORE**（:191-208 `_sync_mouse_filter_at_global`），让路链 :211-230（zoom chrome :216/floating interactive :222/non-lane interactive :226/external interactive :228），命中前提=`_mapping.row_at_global_point != null` :230。
- 分派核心：`_begin_gesture` :413 → `_mapping.resolve_hit_target(global_pos)` :418 → **hit-kind 词表**在 `TimelineViewMapping.gd:116-228`（`playhead_band` :124-133 抢占 clip；clip 族 `clip_body/clip_edge_left/clip_edge_right/clip_fade_in/clip_fade_out/clip_gain` :140-200；`empty_lane` :223-228）→ 仲裁器 **match kind** `TimelineInputArbitrator.gd:503-550`（clip 族 :504-505 → `_begin_clip_gesture` :632；playhead_band :506-507；empty_lane :508-546 含 draw 工具新建 MIDI clip）。
- 手势状态机（新 kind 可直接复用）：press_candidate 记账 :673-676 → 4px 拖动阈值 `CLIP_DRAG_START_THRESHOLD_PX` :28（:375-380 消费）→ captured（`_start_clip_drag_from_press` :758-836）→ `_update_active_gesture` :959-1010 → `_end_gesture` :1025-1055；gesture/preview 状态落 `TimelineEditStore.set_runtime_state/set_preview_state`（:806-809/:834/:942-955/:964-973）。
- lane 自理先例：`_forward_event_to_row_clip_provider` :680-688——range 工具事件直达 lane 的 `handle_direct_timeline_input_event`（不进 match 词表）；包络车道若自带编辑器，可循此形态把事件整段让渡给 lane，少侵入仲裁器。

## 二、问2：数据通道实锚

### 2.1 前端既有通道族（三面并存）

| 面 | 入口 | 形态 | automation 三命令适配性 |
|---|---|---|---|
| ① 遗留内核命令面 | `kernel/clients/rack_client.gd`（146 行）→ `kernel_command_client.gd:13-25` → `/root/VitIpcClient`（kernel_command_client.gd:35-42）→ `kernel/autoloads/vit_ipc_client.gd` | **UDP 4445 → godot_bridge.py → 内核 ZMQ REQ**（vit_ipc_client.gd:2 注释）；FIFO 单活跃请求 :126-159；**大载荷 file_reply 回执** :50-86（`transport:"file_reply"`+reply_file）；get_plugin_parameters 超时 30s（rack_client.gd:5/:39-40） | `automation.get_curve` 已入内核 legacyReadOnlyCommands+renderAllowlist（M3-KERNEL-1 done 卡回执+VspKernelReference.cpp:2383 白名单行亲核）——**前端零新请求面可直接调**，只差 client 包装方法 |
| ② VSP hub 面 | `kernel/clients/vsp_client.gd` | HTTP POST `http://127.0.0.1:8787/vsp`（:6）VSP 信封；`command_request` :131 / **`legacy_command_request` 包装遗留命令** :151-171；`state_delta(base_revision)` :93-111；信封级 base_revision 附加 :366-367 | 走 agent CAS/幂等；automation 三工具已在 agent catalog 注册（catalog.go:867-940+，get=RiskDirect / set=RiskUndoable+确认） |
| ③ 实时推送面 | `kernel/autoloads/vsp_realtime_adapter.gd` | WS `ws://127.0.0.1:8787/vsp/stream`（:24/:1063-1104）+HTTP 帧轮询回退；流=transport/meters/spectrum 三种（:573-599）；**事件订阅主题=delta_update（默认）+audio_feature_data_ready（env 门控）** :1252-1260 → `telemetry_manager.ingest_vsp_event_telemetry` :927-936 | 曲线是准静态数据，不建议新建 realtime stream（无流生产者，须内核+hub 双侧扩） |

### 2.2 拓扑回执/paramID 映射同源可复用性（176/183/185 消费面）

- **拓扑回执**：track.send.* 五命令写回执自带拓扑快照 sends/dangling/fx_tracks/sidechain（`rack_client.gd:63-66` 注释）；control_* 九写回执自带 control_graph 全量（:88-91）；消费链=回执经 signal_bus 路由 → `Vit_Graph_Rack.gd:2634` `_apply_send_topology`（调用点 :2709；:4660 注释=落库并广播 send_topology_changed）。
- **paramID↔轨道/插件映射**：现役面=`get_plugin_parameters` 参数描述符行缓存 `_param_port_rows_by_plugin`（`Vit_Graph_Rack.gd:347` 注释「含 binding_target 投影」；请求 :3770-3772 `_RACK_IPC_PAYLOAD_RULES_SCRIPT.get_plugin_parameters_payload(tid, plugin_id)`；落缓存 :3791）；描述符形态 `{param_id, param_name, min, max, step, …}`（`rack_control_layer_rules.gd:83`）。**内核曲线寻址=plugin_id+param_id（PluginRackControlService.cpp:2013 resolveAutomationCurveTarget，volume/pan 回退同 set_plugin_param）与此面同键**——车道侧选中（track, plugin, param）后即可直接发 get_curve，无需新映射面。
- **写回执回显**：automation.set_points 回执=plugin_id/param_id/mode(region|full)/region/written/point_count/time_base（PluginRackControlService.cpp:2193-2200 亲核）——可直接驱动车道回读/对账。

### 2.3 两案并列 + 推送/轮询倾向（给倾向不越权定案）

- **案 A（倾向推荐）：legacy 命令面直连**——前端新增 `app/kernel/clients/automation_client.gd`（或 rack_client 加三方法，形态同 track_send_* :68-85），payload 直发 automation.get_curve/set_points/clear_region；变更通知首版**不做推送**：车道打开/选中参数时拉取 + 每笔写回执到达后回读 + 既有工程刷新链（`vit_refresh_project_from_kernel`）覆盖外部写回显。
- **案 B：VSP hub 面**——`legacy_command_request` 包装 automation.*（白名单已入）或 `command_request`；收益=信封级 base_revision CAS（:366-367+内核侧 `validateCommandBaseRevision` VspKernelReference.cpp:2507-2530）与 hub 幂等；成本=需核 vsphub 对 GUI 直连 command.request 放行面与超时参数。
- **推送 vs 轮询倾向**：读=按需拉取（案 A 形态）；变更通知可后置增强——**注意 delta_update 是否覆盖 plugin state 变更未证**（`project_store.gd:187-212` delta ops 消费面只见 track/clip 路径），数据通道卡实施时先核内核 state.delta 生产者是否携带 plugin state 变更，再定是否接事件通知。

## 三、问3：机架状态面实锚（app/rack/）

- **卡呈现邻域**：SendOut/SendIn 对卡构建 `Vit_Graph_Rack.gd:2024-2058`（root VBox :2024；tag label :2027-2031；**166 徽标 chip 挂载点 :2034-2036** `root.add_child(_make_send_route_badge_panel(...))`；target/amount label :2038-2050；折叠钮 :2052-2058）；徽标工厂 `_make_send_route_badge_panel` **:2331-2350**（SendRouteBadgePanel/SendRouteBadgeLabel，font_size 11）；刷新 `_refresh_send_route_badge_visuals` :2206-2226（meta `vit_send_route_badge` :2202/:2210）+ `_apply_send_route_node_title` :1973-1998。**「参数有包络」徽章最小侵入形态=此邻域再加一枚 panel+对应 meta/刷新行**（与 185 湿链徽标同位竞争，见 §五冲突评估）。
- **播放头时间→卡面联动既有钩子：无**——`app/rack/` 全域 grep `playhead` 零命中（本勘察亲核）。若步骤 3 要「卡面随播放头读数」，属**新建面**：数据源现成=`GuiTransportClock.get_position_seconds`（gui_transport_clock.gd:166）+信号 `gui_transport_updated`（:3），rack 侧无任何既有消费。
- **「参数有包络」在场性数据源缺口（标注点）**：前端现役 get_plugin_parameters 回执**不含** automation 在场性字段（内核 PluginRackControlService.cpp 全文无 supports_automation 入回执；getNumPoints 仅曲线命令内用 :2066/:2200/:2228/:2245）；agent 侧 grabber `supports_automation=true` 为恒置位非实判（M3-RECON §2.2，VitPluginGrabber.cpp:164）。三案供裁定：①内核 get_plugin_parameters 回执扩实判字段（内核小扩=VitApp 文件域，须另开卡）；②打开参数面时逐参数 get_curve 探测（请求多，贵）；③首版徽章仅对「包络车道已打开/已编辑过的参数」呈现（零内核改动，DAD 曲线回流扩型合入后再增强）。

## 四、问4：写回链实锚（前端编辑包络 → automation.set_points）

### 4.1 183 恰一笔先例全文锚点（可移植性核心）

- 交互契约总注释：`rack/rack_control_widget_factory.gd:10`（value_changed=中间帧本地回显，committed=提交帧 drag_ended/toggle）+ `:80` MacroValueSlider drag_ended 先例。
- **三段式**（`Vit_Graph_Rack.gd:3575-3659`，183 SendOut 卡发送量推子）：
  1. 拖动零发包：`drag_started → set_meta("vit_fader_dragging", true)` :3609-3611；`value_changed → _preview_send_level_fader` :3612-3614（:3639-3649 **只改 label 呈现，零 IPC**）。
  2. 释放恰一笔：`drag_ended → _commit_send_level_fader` :3615-3619 → :3650-3659：悬空/未绑定 fail-closed 守卫（源轨道空→alert return :3645-3649）→ 量化 0.5dB（rack_send_route_rules 共享件）→ payload=`rack_ipc_payload_rules.track_send_set_amount_payload` :3653 → `rack_client.track_send_set_amount(payload)` :3655-3656 **恰一笔**。
  3. 回执回显收敛门：拓扑回执到达面 `_apply_send_route_node_title` :1973-1998——`vit_fader_dragging` 为真时 `set_value_no_signal` 跳过（:1993-1997），**防异步回执抢跳拖拽手感**。
- **包络移植面**：三段式直接平移——拖动=lane 本地预览点渲染零发包；释放=automation.set_points 恰一笔；回显门=vit_env_dragging meta 同款。**语义差一处**：set_points 是**整段/区域替换**非单点（M3-KERNEL-1 裁定 1 有意不暴露单点面），释放时应发「本次拖动编辑域」完整点集→前端需持有该曲线工作副本、按区域替换语义发包（数据通道卡/写回卡需定副本持有与区域边界规则）。

### 4.2 base_revision 取用面（标注点：裁定 2 与已合入实现的事实核）

- 事实 1：内核 automation.set_points/clear_region 处理器**无 base_revision 参数、回执亦无 revision 字段**（PluginRackControlService.cpp:2088-2201 全读亲核；CommandDispatcher.cpp:3131-3145 注册）。
- 事实 2：agent catalog automationSetPointsSpec/automationGetCurveSpec **无 base_revision 入参**（catalog.go:867-940 亲核）。
- 事实 3：**现役 CAS=VSP 信封级**——内核 VSP 适配层 `validateCommandBaseRevision`（VspKernelReference.cpp:2507-2530：request/args 层读 base_revision，mutation 须正值），特性旗 `command.base_revision_cas`（:1058）；revision 游标=state.snapshot/delta 的 revision（:2331/:2360）；**前端已有该游标=project_store.gd `vsp_revision`（:21，更新 :155/:173）**。
- 事实 4：set_plugin_param 现役家族同样**不**带 base_revision（catalog.go:1170 spec 无此参）——「与 set_plugin_param 现役同粒度」的实际现役=无逐命令 CAS，靠 RiskUndoable+确认+readback 收敛。
- 结论（供主管，不越权定案）：前端写回卡若走案 A（UDP 直连）=与 183 同姿态（无 CAS+写后回读对账），首版建议如此；若要信封 CAS=走案 B（VSP legacy_command_request+base_revision，游标 project_store.vsp_revision）。裁定 2 的「plugin state 级」语义与已合入实现存在口径差，建议在写回卡发卡时显式裁定一次。

## 五、问5：实施卡拆分建议（供主管裁定，不自行定案）

建议 4 张（C1 可先行，C2/C4 同文件域串卡，C3 等 185）：

| 卡 | 目标 | 预期文件域 | 依赖/时序 |
|---|---|---|---|
| C1 数据通道+写回底座 | automation_client.gd 三方法（get_curve/set_points/clear_region，走 RackClient 同通道）+回执解析 | **新文件** `app/kernel/clients/automation_client.gd`（+测试/探针件新文件）；rack_client.gd 可零改 | 无前置；可与 C2 前期并行（文件域不相交） |
| C2 包络车道 UI（呈现+选中） | 车道挂载（§1.2 三案择一）+曲线绘制+hit-kind 词表扩展+手势复用 | `app/timeline/`（TimelineViewMapping/TimelineInputArbitrator/TimelineEditStore）+`app/tracks/track_scene/`（vit_track_lane_2d.gd/Track_Row_Controller.gd）+行高（若案 B 含 timeline_dock_adapter.gd） | 依赖 C1；与 C4 同文件域，建议同会话串卡或 C2 先呈现后编辑两步走 |
| C3 机架徽章（包络在场） | 卡头徽标行+在场性派生（§三数据源三案择一） | `app/rack/Vit_Graph_Rack.gd` 卡头邻域+rack_node_display_rules.gd 派生纯函数+探针新件 | **与 185 同文件 Vit_Graph_Rack.gd——时序单列，185 合入后领取** |
| C4 写回链闭环 | 拖编辑→set_points 恰一笔+回显门+工作副本/区域替换规则 | 与 C2 重叠（lane 内） | 依赖 C1+C2；不与 C2 并行（同文件域） |

- **与在飞 180/185 冲突评估**：C1/C2/C4 文件域（app/timeline+app/tracks+新 client 文件）与 180/185（app/rack/Vit_Graph_Rack.gd+rack_send_route_rules.gd+stack_probe_sendroute_macro.gd L 腿）**不相交**；唯 **C3 与 185 同文件同邻域——按卡面要求时序单列，180→185 串行链不断插**。包络侧探针须建新探针文件，不得插 stack_probe_sendroute_macro.gd。
- 验收建议：C2/C4 过渲染面探针（新文件）+既有探针零回退；真栈腿=打开车道→get_curve 呈现→拖动释放 set_points→回读逐点一致（可复用 M3-KERNEL-1 冒测 ps1 模式扩 GUI 腿，exit 0 门槛）。

## 六、停止条件核查

TimelineEditStore/TimelineInputArbitrator/183 三段式先例与卡面假设**一致（未重构）**；M3-KERNEL-1 三命令注册锚点（CommandDispatcher.cpp:3131-3145）与本卡设计输入一致——**未触发停止条件**。

## 七、标注点汇总（供主管）

1. **base_revision 口径差**：裁定 2「plugin state 级 base_revision」在已合入实现中的实际形态=VSP 信封级 CAS（事实核 §4.2）；写回卡发卡时显式裁定走案 A（无 CAS+回读，183 同姿态）或案 B（信封 CAS）。
2. **「参数有包络」在场性数据源缺口**：get_plugin_parameters 回执无字段（§三），三案择一（内核小扩/逐参探测/车道打开过的参数首版）。
3. **delta_update 覆盖面未证**：现役 delta ops 消费面只见 track/clip（project_store.gd:187-212），plugin state 变更是否入 ops 待数据通道卡实施时核。
4. **包络车道挂载三案**（第三视图模式/行内分区/独立 lane 实例，§1.2）——建议 C2 卡面裁定。
5. **set_points 替换语义**：前端拖编辑须持有曲线工作副本按区域替换发包（§4.1），单点 add/move 面未暴露（有意省略）。

## 附：grep 附录（关键定位命令，前端仓 app/ 下）

```
grep -rn "lane" --include="*.gd" -il          # 车道文件族（42 文件）
grep -n "LANE_VIEW_\|VIEW_WAVEFORM" tracks/track_scene/*.gd
grep -rn "playhead" rack/                      # 零命中（Q3 无钩子证据）
grep -rn "vit_fader_dragging\|drag_ended" rack/ Vit_Graph_Rack.gd
grep -rn "base_revision" kernel/clients/ state/stores/   # vsp_client.gd / project_store.gd
grep -n "supports_automation" VitApp/Source/Service/PluginRackControlService.cpp  # 零命中（内核）
```
