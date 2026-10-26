# M3-ENVELOPE-CLOSEUP-RECON-1：特写窗口「包络视窗」勘察+设计报告

- 执行：GLM-5.3 PC 执行会话 / 2026-10-26 晚 / 领取提交 f0d7712c（origin/main=b3b87248）
- 勘察对象：前端仓 `D:\Godot\project\vit-daw-frontend` @ main=20873f8（只读，零改动亲证见 §9）
- 结论速览：**七问全部有既有件可锚，无需重构宿主容器**；视窗本体确认 2D 画布型（Control._draw）；写回链内核面（set_points 三键点 {t,v,c} + region 语义）已就绪；三个形态类裁定点（视窗入口 / 参数页签 / 曲率句柄形态）单列 §8 待用户裁定，预览见同目录 `envelope-closeup-form-options-v1.html`。
- 停止条件核对：两条均未触发（切换机制与假设相符；时间轴复用面关键件齐备）。

---

## 0. 七问摘要表

| # | 问题 | 结论 | 关键锚点（前端仓相对路径） |
|---|---|---|---|
| 1 | 视图切换机制与挂载点 | 视窗走 `viewport_registry` 注册 + stage 槽/独立窗双路径（钢琴卷帘先例）；入口推荐=启动器菜单（零域冲突）+车道/轨头入口在 192 合入后追加 | `vit_dock/core/viewport_registry.gd:5-67`、`vit_dock/scenes/vit_dock_root.gd:4804-4893` |
| 2 | 视窗本体择型 | **2D 画布型（Control._draw）**，三处先例可直接复用式样；相机预设型不推荐（3D 拾取深度误触+缩放语义冲突） | `vit_dock/views/vit_detail_canvas.gd`、`app/tracks/track_scene/track_envelope_lane_2d.gd:691-752` |
| 3 | 时间轴/播放头/缩放复用 | t→x 三级回退 + 播放头三级回退 + 18ms scrub + 重绘节流全套可复用；全轨范围取数=total_duration 广播链 | `track_envelope_lane_2d.gd:584-658`、`vit_detail_canvas.gd:33,784-839` |
| 4 | 参数选择与多参数 | 数据链 189 已就绪（rack_nodes 目录→get_plugin_parameters→curve）；envelope_point_count（190）在前端尚零消费，本视窗=首个消费者；页签形态两案待裁 | `track_envelope_lane_2d.gd:234-241,399-413`、`vit_dock/core/project_repository.gd:287` |
| 5 | 编辑手势面 | 视窗内局部手势状态机（不接 timeline 全局仲裁器，领土天然互斥）；加点/拖点/删点/曲率句柄均有先例锚 | `app/timeline/TimelineInputArbitrator.gd:508-521`、`app/piano_roll/view/PianoRollView.gd:95-97,1080-1101,1690-1692` |
| 6 | 写回链 | 释放恰一笔 `set_points`（points=[{t,v,c}]，region 模式）；回显门=request_id 路由回执重拉；编辑中外部变更=拖拽态忽略外部刷新、释放合并 | `app/kernel/clients/automation_client.gd:59-94,122-139,195-268` |
| 7 | 车道↔视窗关系+文件域 | 车道=呈现+入口接线点（仲裁器 focus 链可挂打开动作）；视窗核心域=新文件+registry+dock_root，与 191（app/rack）/192（timeline+track_scene）零相交，但入口若选轨头/车道按钮则与 192 相交 | `Track_Row_Controller.gd:1070-1089,1109-1126` |

---

## 1. Q1 视图切换机制现状与包络视窗挂载点

### 1.1 现状链路（勘察实证）

特写区是 **vit_dock 的多视窗宿主**，与轨内 View_Gizmo 是两套互不相干的机制：

**A. 钢琴卷帘（多视窗宿主先例，两条进入路径）**

- 轨头按钮路径（独立浮窗）：
  `Track_Row_Controller.gd:136`（`Btn_PianoRoll_Toggle` @onready）→ `:276-280`（connect）→ `:3816-3823`（`_on_piano_roll_pressed` → `request_open_piano_roll.emit(kid, "", track_id)`）
  → `vit_dock/views/timeline_dock_adapter.gd:3024-3029`（对每轨 connect 该信号）→ `:3135-3143`（`_on_track_row_request_open_piano_roll` → `await main.vit_open_piano_roll_from_track_header`）
  → `vit_dock/scenes/vit_dock_root.gd:4804-4847`：`window_manager.create_empty_window()` 建独立窗 → `activate_view("piano_roll", ctx)`（ctx={track_scope, clip_scope, follow_project_clip_focus, focus_anchor, view_state}）→ 窗标题「钢琴卷帘 · kid/clip」。
- 双击 MIDI clip 路径（推入台前槽）：
  `app/timeline/TimelineInputArbitrator.gd:591-593`（双击 emit）→ `vit_dock_root.gd:4862-4875`（`vit_push_piano_roll_into_stage_slot`，hint clip 校验失败=诚实不上台）→ `:4877-4893`（`vit_open_piano_roll_in_stage_slot` → `_stage_push_view("piano_roll", slot_of("timeline"), ctx)`）。

**B. 舞台槽引擎（台前 4 槽自动铺排）**

- `vit_dock/core/stage_slot_engine.gd:10-13`：`MAX_SLOTS=4`，默认 `[timeline, graph_rack]`；`:30 push_view`——指定位被占=原住客回台后，满员=挤出 LRU 队首；铺排形状只由占用数决定（1 全屏/2 左右/3 左整+右上下/4 田字）。
- `vit_dock_root.gd:1652-1660 _stage_push_view` 统一装配；`:1377 _rebuild_fixed_stage_slots` 重建。

**C. 视图注册表（视窗接入点——本卡核心发现）**

- `vit_dock/core/viewport_registry.gd:5-67`：`VIEW_DESCRIPTORS` 常量字典，现有 timeline/graph_rack/piano_roll/spectral/detail/mixer/media_pool 七项，每项={label, scene, allow_multi_instance, supports_track_filter, supports_focus_anchor, spawnable_in_viewport}。
- 注册新视图 = 加一个 `"envelope"` 条目（scene 指向新 adapter .tscn）。注册后自动出现在空槽启动器：`vit_dock/scenes/viewport_frame.gd:49-51`（`_launcher.set_view_options(_registry.list_spawnable_descriptors())`），并可用 `activate_view("envelope", ctx)` 程序化打开（`viewport_frame.gd:88-93`）。
- adapter 约定接口（`viewport_frame.gd` 全部 has_method 探测式调用）：`setup_view(viewport_id, view_type, context)` / `set_view_context` / `get_view_state` / `restore_view_state` / `handle_viewport_shortcut` / `set_drag_resize_mode`——见 `vit_dock/views/piano_roll_dock_adapter.gd:36-86`（247 行完整先例）。
- 独立窗弹出：`vit_dock/core/window_manager.gd:33 create_empty_window` / `:16 popout_viewport`。

**D. View_Gizmo 与 Btn_Mode_Toggle（轨内机制，非视窗切换）**

- `app/tracks/track_scene/right_3d_wrapper.gd:1178-1227 _setup_view_gizmo_buttons`：Btn_Top/Btn_3D/Btn_Side 三按钮 ButtonGroup，切换**轨道行右侧 3D 波形瓦片**的相机预设+视图模式——`:1224-1227`（`pivot_node.snap_to_eq_slice() / _on_btn_3d_pressed() / snap_to_waveform()`）与 `:1229-1240 _apply_waveform_view_mode`（VIEW_MODE_TIME=0/FREQ=1/OVERVIEW=2 → `tile_manager.set_view_mode`，常量定义 `:21-23`）。
- `:165-171 Btn_Mode_Toggle`：`edit`/`view` 交互模式切换（`:1170-1176`），不是视窗切换器。

### 1.2 挂载点与切换按钮择型（卡面三候选评估）

| 候选 | 评估 | 域影响 |
|---|---|---|
| ① View_Gizmo +1 按钮 | **不推荐**：Gizmo 是轨内 3D 波形相机预设组（三键固定预设语义），包络视窗是台前视窗，层级与语义均错位；ButtonGroup 改造涉 right_3d_wrapper.gd+tscn | `right_3d_wrapper.gd`（192 域外，但 189 曾声明其零改=C 案边界亲证） |
| ② 轨头按钮（仿 Btn_PianoRoll_Toggle） | 链路最顺（一键直达独立窗/台前槽双路径先例现成）；但 `Track_Row_Controller.gd` 在 **192 文件域**（timeline+track_scene）内，192 在飞 | 与 192 相交——须 192 合入后追加或错峰 |
| ③ 底栏 Btn_Mode_Toggle | **不推荐**：该钮是轨内 edit/view 交互模式开关，语义完全不同；挂在它下面误导 | 同 ① |
| ④（补充）空槽启动器菜单 | **零成本**：registry 注册即自动出现在 `EmptyViewportLauncher`，零额外文件 | 零相交 |
| ⑤（补充）车道内入口（189 车道点击/按钮） | 卡面 Q7 指定关系=车道呈现+点击进视窗；但 `track_envelope_lane_2d.gd` 亦在 192 域（三态档改造对象） | 与 192 相交——同 ② 错峰 |

**推荐案**：C4 实施卡分两步——首版入口=④启动器菜单 + `vit_dock_root.gd` 新增 `vit_open_envelope_in_stage_slot(kernel_track_id, plugin_id?, param_id?)`（完全在 vit_dock/ 域，零冲突）；②⑤入口按钮在 192 合入后以追加卡接线（轨头按钮 + 车道 focus 双击/按钮，走同一 dock_root 打开函数）。独立窗路径（window_manager）按钢琴卷帘先例可同步免费获得（`:4804` 模式复制）。

---

## 2. Q2 视窗本体择型

**推荐：2D 画布型（纯 Control + `_draw()`）**——与用户裁定方向一致，本节给实证支撑：

- 先例一（**最完整**）：`vit_dock/views/vit_detail_canvas.gd`（Detail 特写视图的 2D 画布）——纯 Control._draw，自持时间窗（`:42-44 _view_start/end_seconds`）、t↔x 双向映射（`:784-791`）、播放头 epsilon 重绘（`:33 PLAYHEAD_REDRAW_EPSILON_SEC=0.002`）、scrub 节流（`:12 PLAYHEAD_SCRUB_SEND_INTERVAL_MS=18`）、seek 信号（`:4 playhead_seek_requested(seconds, send_kernel)`）。**包络视窗画布=此文件的直接同型仿写**。
- 先例二：`app/tracks/track_scene/track_envelope_lane_2d.gd:691-752`——包络折线+点+聚焦态+播放头绘制全套已在（车道版），视窗版放大复用 `_value_to_unit`（`:673`，含 assumed 兜底）与 `_point_to_local`（`:683`）的坐标式。
- 先例三：`app/piano_roll/view/PianoRollView.gd`（2008 行 2D 音符网格）——网格/吸附/拖拽交互最重的 2D 画布，证明该形态承载复杂编辑手势无障碍。

相机预设型不推荐的实证理由：

- 拾取走 3D 深度：包络手势需要像素级命中（车道先例 `_POINT_HIT_RADIUS_PX=6.0`，`track_envelope_lane_2d.gd:29`）；3D 场景点击要过 raycast+相机面换算，正交平面拍摄下点/线/地面三者的深度序易误触。
- 缩放语义冲突：stage 视窗的时间缩放（横轴秒窗）与参数轴缩放（纵轴值域）是两根 2D 轴；塞进 3D 相机体系（`Turntable_Pivot` 的 ortho_size/frequency_zoom，`right_3d_wrapper.gd:813-880`）后要同时锁两个轴，等于在 3D 里模拟 2D，复杂度白付。
- 帧率与重绘：3D 视图走 SubViewport 常驻渲染；2D 画布 `queue_redraw()` 按需重绘（车道 `_process` 仅在播放头位移>0.001s 时触发，`:652-658`），编辑视窗高频重绘下更省。

---

## 3. Q3 时间轴/播放头/缩放复用面与全轨时间范围取数

### 3.1 可复用面（全部有现成式样，"同式复用"是仓库既有惯例）

- **t→x 映射三级回退**（`track_envelope_lane_2d.gd:584-613`，文件内注明"vit_track_lane_2d :1452-1482 同式"）：
  ① `wrapper.get_visible_timeline_range_seconds()`（跟随轨道行可见窗）
  ② `global_time_axis` group（`GlobalTimeAxisController`，`app/timeline/global_time_axis_controller.gd:2`）的 `get_global_time_scroll()` / `get_global_ortho_size()`
  ③ 缺省 `[0, 20]s`。
  视窗版取舍：视窗是全轨画布（非 clip 焦点），**不复用轨道行可见窗**，但 ②③ 的回退结构照抄（数据源换 total_duration，见 3.2）。
- **播放头三级回退**（`track_envelope_lane_2d.gd:625-649`，注明"vit_track_lane_2d :4043-4053 同式"）：① `GuiTransportClock.get_position_seconds()` ② `VitTelemetryManager`（fresh 时）③ wrapper 本地走带。重绘节流 `_process` 仅在位移>0.001s 时 `queue_redraw()`（`:652-658`）；绘制 `:743-752`（glow 5px+主线 1.75px）。
- **scrub**：`vit_detail_canvas.gd:4`（`playhead_seek_requested(seconds, send_kernel)`）+ 18ms 发送节流（`:12`）——视窗内拖播放头直接同式。
- **缩放**：`right_3d_wrapper.gd:35 time_zoom_factor` + `:17-20`（BASE_TIME_ORTHO_SIZE=6.0 / DEFAULT_TIME_VISIBLE_SECONDS=20.0）+ `:534 _on_time_zoom_slider`——视窗版用同常数语义自持 `view_seconds`（或 Ctrl 滚轮改窗宽，先例 `PianoRollView.gd:1080-1101` 的 Ctrl=横/Shift=纵双轴滚轮缩放）。
- **纸面背景/HUD**：`vit_detail_canvas.gd:14-32`（COLOR_BG/COLOR_GRID/COLOR_TEXT 常量族）+ `right_3d_wrapper.gd:329 _apply_hud_overlay_layout`——视窗画布直接沿用 detail_canvas 常量族。

### 3.2 全轨时间范围取数路径（视窗≠clip 焦点）

- total_duration 现有喂链：`app/kernel/autoloads/telemetry_manager.gd:1010`（内核遥测 → `call_group("vit_track_rows", "apply_kernel_timeline_duration", ...)`）→ `Track_Row_Controller.gd:3408-3414`（转发给 wrapper）→ `right_3d_wrapper.gd:1821-1826 set_backend_total_duration`（getter `:1272`）。汇总面先例：`timeline_dock_adapter.gd:2520-2522`（取各轨 wrapper 的 max）。
- **推荐**：视窗画布自持 total_duration，两条到达路径——① 独立窗/台前槽的 adapter 在 `setup_view` 时从 `vit_track_rows` group 任一轨 wrapper `get_backend_total_duration()` 取初值并订阅遥测广播；② 车道入口进入时由入口侧直传。范围缺省 `[0, max(total_duration, playhead+10)]`；total_duration=0（空工程）显示空态文案（车道 `_draw_state_text` 先例 `:734-740`）。
- 横向滚动/缩放状态归 adapter 的 `get_view_state` 快照（`viewport_frame.gd:133-146` 的 view_state 缓存机制，视图换出/换回自动恢复）。

---

## 4. Q4 参数选择与多参数（Serum 式页签）

### 4.1 数据源链（189 已全部就绪，视窗直接同源）

- 插件目录：`project_repository.gd:287 get_rack_nodes_by_track()`（rack nodes 快照，185 换源后的权威面）→ 归一 `{id,name,type}`（`track_envelope_lane_2d.gd:234-269 refresh_plugin_catalog/_set_catalog`）。
- 参数描述符：`rack_client.get_plugin_parameters({track_id, plugin_id}, request_id)`（`:399-413 _request_param_rows`，注明与 Vit_Graph_Rack 同源命令面、独立 request_id 不共享私有缓存）→ 回执 `:478-495 _apply_param_rows_reply`（rows 透传 parameters/params 数组）。
- 值域归一：`:525-537 _apply_value_scale_from_rows`（min/max 数值合法才采，否则 `{0,1,assumed:true}` 兜底不静默）。
- 曲线拉取：`:420-432 _request_curve`（automation_client.get_curve）→ `:498-522 _apply_curve_reply`（**beats 曲线 fail-closed 拒呈现**——time_base=="beats" 且非空即披露错误，写回侧拒收归 C1/C4 契约）。
- **envelope_point_count 在场性（190 已合入内核，前端零消费）**：描述符参数行新增 int 字段（0=无包络），覆盖面=**外部（rack VST）插件参数**；内置轨级参数（volume/pan）不在 get_plugin_parameters 回执面（内核回 error only-external-plugins）——轨级缺口 190 主管补注已上交另裁，本视窗**如实呈现**：参数列表以 envelope_point_count>0 标记有包络项，轨级参数不出现在该列表（与 189 车道同一边界）。

### 4.2 页签形态（裁定点 D2，预览见 HTML）

- **案 A（Serum 式横向页签条）**：顶部一排参数小页签（可滚动），单参数呈现+页签快速切换。结构参考 `detail_dock_adapter.gd:20-23+33`（TAB_WAVEFORM/SPECTRUM/FIELD_3D + %DetailTabs TabContainer）——已注册的 TabContainer 先例。
- **案 B（两级下拉，车道同式）**：头部 OptionButton×2（plugin→param），最大化画布。结构= `track_envelope_lane_2d.gd:775+ _build_header` 放大版。
- 推荐案 A（多参数横向扫视快、Serum 心智一致；外部插件参数量 ~20 级别页签条可承载），B 作为首版简化退路。

---

## 5. Q5 编辑手势面盘点

### 5.1 手势宿主择型：视窗内局部状态机（推荐），不接 timeline 全局仲裁器

- 全局仲裁器领土判断锚点：`TimelineInputArbitrator.gd:175 _is_global_point_inside_timeline` / `:211 _should_accept_mouse_at_global`——台前槽/独立窗的 Control 子树不在 timeline 领土内，视窗 `_gui_input` 天然先收（Control 事件传播），**零仲裁器改动**。
- 189 在仲裁器内的 envelope 族分派（`TimelineInputArbitrator.gd:508-521`）：envelope_lane/envelope_lane_point → 行选中 + envelope_lane_focus_press——注释明写「点编辑=拖点/加点/写回归 C4，**不启用手势状态机**」：车道侧保持只读选中，编辑手势全部归视窗，与 189 裁定一致，无双重接入冲突。
- 命中算法直接复用车道式：`track_envelope_lane_2d.gd:213-225 _nearest_point_index_at_local`（6px 半径最近点）+ `:170-183 hit_test_global`（kind/point_index 结构）。

### 5.2 逐手势实现锚点候选

| 手势 | 先例锚点 | 说明 |
|---|---|---|
| 单击折线加点（按下即拖） | `PianoRollView.gd:95-97`（_note_drag_active/_drag_start_positions 状态机）+ `:473`（grid_canvas.gui_input connect）+ `:1124-1125`（释放 _finish_note_drag 提交） | 按下即建临时点进拖拽态、释放写回；音符「按下即拖、释放提交」范式同型 |
| 拖点（锁轴修饰键） | `PianoRollView.gd:1080-1101`（Ctrl 滚轮=横轴、Shift=纵轴双轴缩放）+ `:1565/:1712`（Shift/Ctrl/Meta=lasso additive） | 推荐锁轴 **Shift=锁 t（只改 v）**、Ctrl=锁 v（只改 t）；拖拽增量应用（`:1168 _apply_note_drag_delta_global` 式） |
| Delete 删点 | `PianoRollView.gd:1690-1692`（右键删音符）+ `:1913 _erase_single_note` | 聚焦点 Delete/Backspace 删；键盘动作注册面先例=`vit_shortcut_manager.gd:105 _ensure_action_with_key` |
| 曲率句柄（段中点） | 无现成句柄先例——189 车道 `_draw_curve`（`:715-731`）只 draw_polyline，**c 字段在车道完全未消费，视窗为首版曲率编辑落点** | 句柄=段中点小柄拖拽改 c；右键柄弹预设菜单（PopupMenu 新建；右键先例=PianoRoll:1690 右键删，语义不同需新菜单件） |
| 网格吸附 | `PianoRollView.gd:38 snap_drag_to_grid` | 拖点可选吸附（首版可不加） |
| 播放头拖拽 scrub | `right_3d_wrapper.gd:882-985`（播放头带命中+视觉态机）+ `vit_detail_canvas.gd:4`（seek 信号） | 视窗顶部 ruler 带拖拽 |

---

## 6. Q6 写回链设计（锚到 C1/187 既有件）

`app/kernel/clients/automation_client.gd`（C1 产物，316 行）命令面全部就绪：

- **命令**：`:59 get_curve`（读超时 5s）/ `:63 set_points`（写超时 10s）/ `:67 clear_region`。
- **payload**：`:81-85 set_points_payload(plugin_id, param_id, points, region)`——points=[{t,v,c}]（**c 曲率字段在内**，`:254-268 _validated_points` 三键校验）；region 缺省=全曲线替换（mode=full），region={start,end} 秒=区域替换（区域内新点接收域=[start,end] 闭区间）。
- **客户端纯函数**（工作副本合并）：`:195 apply_region_edit` / `:223 apply_region_clear` / `:243 apply_full_replace`——**视窗编辑期的工作副本合并直接调用，不重写**。
- **回执解析**：`:122-139 parse_set_points_reply`（mode/written/point_count）；`:103-118 parse_get_curve_reply`（points 归一 t/v/c 三键 + time_base/bypass）。
- **错误分类**：`:158-174 classify_error_reply`（beats_curve_rejected/empty_points_rejected/target_missing/malformed_region/point_out_of_domain/no_active_edit）——视窗错误横幅直接消费。

写回链设计推荐：

1. **释放恰一笔**：任一编辑手势（加点/拖点/删点/曲率）在释放时构造一次 `set_points`；**region 模式**（region=受影响点的最小时间邻域，前后锚点各扩一点）而非全量替换——单点编辑不重写全曲线。
2. **回显门**：写回 ok → 同一 request_id 路由面（`track_envelope_lane_2d.gd:461-469 handle_command_completed` 式，request_id 前缀区分）重新 `get_curve` → 校验 point_count 与本地工作副本一致 → 提交本地状态+重绘；不一致=回滚到回执点集并披露（不静默）。
3. **编辑中外部变更锚定重绘**：拖拽激活态忽略外部 curve 刷新（command_completed bus 上非本 request_id 的 curve 类回执挂起）；释放写回后以回执为准合并——避免拖拽中曲线跳变。
4. **beats 边界**：读侧 beats 曲线 fail-closed（车道先例 `:509-517`）；写侧 time_base 拒收错误直接呈现横幅。

---

## 7. Q7 车道↔视窗关系与文件域盘点

### 7.1 车道（189）=呈现+入口的接线点

- 车道工厂挂载：`Track_Row_Controller.gd:1070-1089 _ensure_envelope_lane_2d_layer`（wrapper 下 TrackEnvelopeLane2D 节点，bind_track_row/set_kernel_track_id/bind_timeline_edit_store）。
- 车道选中/聚焦链：`TimelineViewMapping.gd:139-140`（行级 hit 转发）→ `TimelineInputArbitrator.gd:508-521`（envelope 族 match）→ `Track_Row_Controller.gd:1109/1120` 转发 → `track_envelope_lane_2d.gd:187 focus_press`。
- **入口接线点推荐**：车道头部 `_build_header`（`:775`）加「放大」按钮（或车道体双击）→ emit 新信号 → timeline_dock_adapter 转发（request_open_piano_roll 同式 `:3024-3029`）→ `vit_dock_root.vit_open_envelope_in_stage_slot(track_id, plugin_id, param_id)`（ctx 预选车道当前参数，进视窗免二次选择）。
- 车道/视窗状态互通：视窗写回成功后车道下次 `_request_curve` 自然刷新（车道无缓存按需拉取）；车道 focus 点位经 ctx.view_state 带入视窗初始 focus。

### 7.2 新视窗文件清单（C4 实施面预估）

| 文件 | 动作 | 域冲突 |
|---|---|---|
| `vit_dock/views/envelope_dock_adapter.gd` + `.tscn` | 新建（adapter 约定接口，仿 piano_roll_dock_adapter） | 无 |
| `vit_dock/views/envelope_closeup_canvas.gd` | 新建（2D 画布本体，仿 vit_detail_canvas+车道绘制式样） | 无 |
| `vit_dock/core/viewport_registry.gd` | +1 "envelope" 条目（约 10 行） | 无 |
| `vit_dock/scenes/vit_dock_root.gd` | +vit_open_envelope_in_stage_slot（仿 `:4877`，约 30 行） | 无 |
| `app/tracks/track_scene/Track_Row_Controller.gd` | （可选，追加卡）轨头 Btn_Envelope_Toggle | **192 域** |
| `app/tracks/track_scene/track_envelope_lane_2d.gd` | （可选，追加卡）车道入口按钮 | **192 域** |
| `vit_dock/views/timeline_dock_adapter.gd` | （可选，追加卡）车道信号转发 | 无（vit_dock/views/ 不在 192 声明域内） |

### 7.3 与 191/192 并行关系核对

- 191（M3-ENVELOPE-RACK-BADGE-1，in flight）：域=`app/rack/Vit_Graph_Rack.gd`+`rack_node_display_rules.gd`。视窗核心域（vit_dock/ 新文件+registry+dock_root）**零相交**。
- 192（M3-ENVELOPE-LANE-AUTO-1，in flight）：域=`app/timeline/`+`app/tracks/track_scene/`（车道三态档+Track_Row_Controller 档位邻域）。视窗核心域零相交；**仅入口按钮（§7.2 后两行 192 域项）相交**——推荐拆追加卡在 192 合入后实施；首版入口=启动器+程序化打开，不碰 192 域。
- 192 卡面已注记「C4 勘察卡未开卡，无在飞冲突」→ C4 实施卡开卡时须与 192 状态复核（发卡侧把关）。

---

## 8. 形态类裁定点清单（过用户后才能开实施卡）

| # | 裁定点 | 选项 | 本报告推荐 | 预览 |
|---|---|---|---|---|
| D1 | **视窗切换入口** | A. 轨头专用按钮（钢琴卷帘同款位） B. 车道头「放大」按钮/车道双击 C. 仅空槽启动器菜单 | 首版 C+B（B 待 192 合入）；A 备选 | HTML §1 |
| D2 | **参数页签形态** | A. Serum 式横向页签条（envelope_point_count>0 标记） B. 两级下拉（车道同式放大） | A | HTML §2 |
| D3 | **曲率句柄形态** | A. 段中点单柄（拖=c，右键=预设菜单：线性/快入/快出/S） B. 双贝兹柄（段两端各一柄） C. 无柄纯右键预设 | A（单柄直觉+预设降门槛；B 首版过度） | HTML §3 |

预览文件：`envelope-closeup-form-options-v1.html`（同目录，静态单文件，浏览器打开）。

---

## 9. 勘察纪律核对

- 前端仓零改动：全程只读工具（grep/sed/Read），`git status` 前后一致（untracked 运行时目录若干、无源码改动，见回执）。
- 主仓改动仅 `coord/`（本 run 目录+卡状态）。
- 无真栈腿：未启动任何内核/Godot/agent 进程，无栈登记需求（卡面注记）。
- 全部锚点为 20873f8 基线实测行号；190 证据引自主仓 `coord/cards/done/2026-10-25-M3-ENVELOPE-PRESENCE-1.md`。
