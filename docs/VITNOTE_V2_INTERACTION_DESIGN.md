# VITNOTE_V2_INTERACTION_DESIGN — vit note 圈搜式全域交互 v2 设计

- 产出：VITNOTE-INTERACT-V2-DESIGN-1（PC 执行侧 L2 设计档，2026-10-01）
- 输入：**用户八项裁定**（权威输入，[decisions/2026-10-01-vitnote-v2-interaction-ruling.md](../coord/decisions/2026-10-01-vitnote-v2-interaction-ruling.md)）+ [VITNOTE_V1_DESIGN.md](VITNOTE_V1_DESIGN.md) + M8 第四轮手测取证（coord/runs/M1-RETEST-20261001/FORENSIC-NOTE.md）+ 本轮 Godot 仓只读勘察（port/vitnote-dock-mount-1@66e59f0；行号随提交漂移，以锚点符号为准）
- 状态：**规划未实现**——V2 IMPL 卡（§11）依赖本文档；§12 含两项裁定边界项待决策侧确认
- 端测口径：Godot 交互面手测走 M8 节点（AGENTS §5 手测入口=用户从 Godot 拉起 DAW 前端）；Godot 仓无自动化测试基建（F15），headless 可断言面见 §10.1

---

## §0 摘要

v2 把 vit note 的触发面从「lane 级框选直连」升级为**全域圈选**：Alt+左键拖出屏幕矩形 → 全局圈选层仅活跃期拦截输入 → 松开时按坐标枚举命中面及占比 → 各面供给器解析域对象 → **合并为单 note**。第一原则贯穿全程（裁定 2）：**圈选层只产生屏幕坐标矩形；GUI 命中仅用于圈定任务范围，note 载荷必须是域对象引用（哪个轨/哪个 clip/哪个效果器）**，agent 任务消费不在 GUI 层。架构=全局圈选层 + 面级供给器（identity+affordance+resolve(rect) 四函数契约）；v2 覆盖轨道时间线、机架、资料库、控制面（状态快照）四类面。V1 的便签容器三态、会话架构、权限闸、写租约全部沿用；V1 的 lane 级通知路径退役（§8）；面板 dock 面零交互 bug 并入 V2 修复（§7，静态勘察已锁定主嫌疑链）。

**停止条件评估：未触发。** Alt+拖全局捕获在 Godot 输入架构下可行（`_input` 逆序传播+挂载末位不变量，§4.4）；唯一冲突点=clip 体上既有 Alt+拖克隆语义（F1），本设计以「clip 体让位」边界处置并升为裁定边界项 D1（§4.2、§12）交决策侧确认，不改八项裁定本身。

## §1 八项裁定 → 设计映射

| # | 裁定（摘要） | 落点 |
|---|---|---|
| 1 | Alt+左键拖全局圈选；时间线保留直接框选兼容；胶囊+[N] 双出口保留 | §4 手势状态机；§4.2 共存边界（D1/D2）；§4.5 出口 |
| 2 | 坐标→枚举命中窗口及占比→各面解析域对象→合并辖区摘要；GUI 命中仅圈定范围、载荷=域对象引用 | §5 命中解析；§9 payload（faces[].domain） |
| 3 | 全局圈选层（仅圈选时活跃）+面级供给器；契约=identity+affordance；新面接入=注册全局矩形+实现供给器 | §4.4 活跃期；§6 供给器契约+接入指南 |
| 4 | v2 覆盖轨道+机架+资料库+控制面（走带/轨道头=状态快照） | §6.1–§6.4 四类面逐一 |
| 5 | 跨面框选：枚举各面占比进摘要，合并单 note，不强制主导面 | §5.3 跨面合并（selection_share） |
| 6 | 便签容器窗口式保留；便签池维持 IMPL-6 后置 | §3 范围表（容器零改动，引用 V1 §4.2/§4.3） |
| 7 | 面板不可交互 bug 并入 V2 修复 | §7 三候选排查路径+修复锚点 |
| 8 | IMPL-1 与 DOCK-MOUNT 两条件卡改挂 V2 验收同场销 | §10.2 九步清单第 9 步 |

设计默认项（决策侧拟定三项，本文档沿用）：空白/无供给器区=空辖区 note 允许（§5.4）；note→正式任务升级通道 v2 不做；圈选视觉=截图工具式半透明遮罩+亮框（§4.3）。

## §2 事实基线（本轮勘察采信，设计只引用不重勘）

| # | 事实 | 出处（Godot 仓 port/vitnote-dock-mount-1@66e59f0） |
|---|---|---|
| F1 | **Alt 键占用**：clip 体上 Alt+左拖=克隆拖拽（`_alt_clone_drag = hit_kind=="clip_body" and Input.is_key_pressed(KEY_ALT)`，vit_track_lane_2d.gd:2768，拖中 :2834 持续复查；clip_3d_container.gd:1023 三维视图同款）；**空 lane 上 Alt+左拖=普通 marquee**（`_begin_marquee_press` :2257 不查修饰键）；除此全仓无 Alt+拖消费者（VitShortcutManager 仅文案格式化 :154/:212；dock 工具快捷键显式拒绝修饰键 vit_dock_root.gd:1522） | 本轮 grep+直读 |
| F2 | **输入投递链双面同构**：router `_input`（timeline_track_list_drop_router.gd:1708；脚本挂接于 `app/legacy/Vit control V1.0.tscn` 与 `vit_dock/views/timeline_dock_adapter.tscn` 两面）在 GUI 投递之前消费画布内左键：`_forward_track_lane_2d_pointer_event`（:1730-1733）→ lane `handle_direct_timeline_input_event`（vit_track_lane_2d.gd:1202）→ 空 lane 起 marquee 返回 true → `set_input_as_handled()` → **GUI pick 不再发生** | 直读 |
| F3 | **interactive 旁路顺序缺陷**：同 `_input` 内 `_global_point_targets_interactive_control`（:1745）在 lane 转发**之后**才检查；且 `_candidate_interactive_hit_roots`（:1990）枚举根只有工具栏/行/空区/Track_List 直接子级——**VitNoteLayer 子树永不入枚举**；`_left_pointer_targets_interactive_control`（:1922）用 `gui_get_hovered_control()` 本可识别悬浮面板，但仅在无 lane 命中时被咨询（转发先行已消费）。`_is_interactive_timeline_control`（:2063）认 BaseButton/Range/LineEdit/TextEdit/OptionButton+lane 组+Resize_Handle_Y | 直读 |
| F4 | 仲裁器 TimelineInputArbitrator：top_level Control，mouse_filter 按点位动态 STOP/IGNORE（`_sync_mouse_filter_at_global` :198-208），`_gui_input`→`handle_global_input_event`；外部交互命中走全树遍历（`_control_tree_has_external_interactive_hit` :257）但识别类型同 F3 清单（:284-289）——**PanelContainer 等非按钮控件体不在列** | 直读 |
| F5 | lane marquee 既有状态机：`_begin_marquee_press`（:2257）→`_update_marquee_gesture`（:2278，阈值 `_MARQUEE_DRAG_START_THRESHOLD_PX=4.0` :63）→`_finish_marquee_gesture`（:2294）真实拖框分支=`_apply_marquee_selection`+`_notify_vit_note_hint`（:2311 组查询 `vit_note_manager`→`present_marquee_hint`）；ESC 取消（:1168-1182）；`_broadcast_marquee_preview`/`set_global_marquee_preview` 组广播（:2472-2481，跨 lane 预览绘制用） | 直读 |
| F6 | 命中字典既有形态：`get_clip_marquee_hits_global`（:2430）返回 `_clip_bounds_dict`+clip_id/track_id/track_y；`_collect_marquee_hits_global`（:2400）按 `vit_track_lane_2d_interaction` 组跨 lane 聚合、按 track_y→start 排序 | 直读 |
| F7 | vit_note v1 链：manager（挂主场景根、full-rect MOUSE_FILTER_IGNORE、组 `vit_note_manager`、`_input` 消费 N :94-103、`get_agent_context` 祖先查 LLM_Chat_Window :118-141，dock 面落空降级空 context）→hint 胶囊（STOP+`_gui_input` 点击 :82-87，8s 淡出）→panel（PanelContainer+按钮信号；7878 `_post_json`；`_set_send_enabled` 单 note 串行） | 直读 |
| F8 | **双面挂载均为树序末位**：legacy vit_control_v_1.0.gd:156-159、dock vit_dock_root.gd:178-182，均 `_ready` 末尾 `add_child`；deferred 后续挂载（如 `_ensure_legacy_master_track_row`）仅加行/lane 类节点，其 `_input` 均自门控（无手势活跃即早退 vit_track_lane_2d.gd:1183） | 直读 |
| F9 | dock 工作区结构：VitDockRoot（Control）→WorkspaceShell→ViewportFrame（PanelContainer）→ViewInstanceHost（IGNORE）→各视图实例**直接挂主窗口树，无 SubViewport 包裹**——GUI 与 `_input` 都在主窗口传播域；资料库=左抽屉 MediaPoolAdapter（app/browser/left_library_dock.gd，Files/Plugins/Groups 三 tab 的 Tree）；机架=graph_rack_dock_adapter（每视口绑选定轨） | 直读 |
| F10 | 控制面状态源：BPM=`main_ui_controller.get_project_bpm`（vit_dock_root.gd:2298）；播放位置=Top_Transport_Bar `_transport_position_now`（:607）；循环区=lane `_time_selection_state`（:122）；轨道头=Track_Row_Controller `_track_name_edit`（:128）/`_read_volume_db_from_track_state`（:2141）/`get_header_volume_diagnostics`（:2219） | 直读 |
| F11 | 机架域对象源：`project_repository.get_rack_nodes(track_id)`（project_repository.gd:767，快照字典数组）；适配器 getter `get_all_project_plugins_for_scene_track`/`get_selected_plugin_*`（graph_rack_dock_adapter.gd:608-642） | 直读 |
| F12 | 资料库域对象源：left_library_dock.gd `get_selected_library_item`（:76）/`get_selected_library_file_path`（:90）/`_available_plugins`（:34）/`_places_roots`（:25） | 直读 |
| F13 | 快捷键体系：VitShortcutManager autoload；`_ensure_action_with_key(ACTION_VITNOTE_OPEN, KEY_N)`（:17）；鼠标按钮+修饰键文案先例（`_format_mouse`） | 直读 |
| F14 | M8 第四轮症状（一手取证）：dock 面 N 键开窗成功但面板零交互（**含收起按钮**）；键盘链通、鼠标链全灭；按钮接线在位（collapse:85/send:104/anchor:164）、manager IGNORE:27 正确 | FORENSIC-NOTE.md:30 |
| F15 | Godot 仓无自动化测试基建（无 GUT/gdUnit）；冒测体系在 Vit-DAW 仓 scripts/（agent 侧），Godot 交互面验收靠手测 | 仓内现状 |

基线版本：Godot 前端=port/vitnote-dock-mount-1@66e59f0（双面挂载已含）；agent 侧架构事实沿用 V1 §2 基线不重勘。

## §3 v2 范围

| 进 v2 | 推迟（v2.x / 远期） |
|---|---|
| 全局圈选层：Alt+左拖手势状态机+仅活跃期拦输入+截图工具式视觉 | 其他修饰键/触发键位可配置化（v2.x，触发键固定 Alt） |
| 坐标命中解析：矩形→面枚举→占比→供给器→跨面合并单 note | 圈选形状（套索/多边形，远期） |
| 供给器契约+注册处（identity+affordance+resolve(rect)+face_rects） | 非矩形/子像素级命中（面级矩形已够载荷语义） |
| 四类面：轨道时间线（lane 采集改造）/机架/资料库/控制面状态快照 | 调音台/频谱/钢琴卷帘等其余视窗（按 §6.5 接入指南逐面补） |
| 面板 dock 面零交互修复（含 legacy 同根风险处置，§7） | — |
| lane 级通知路径退役（§8） | — |
| payload v2（faces[] 多面辖区，§9） | note→正式任务升级通道（远期，裁定默认） |
| V2 九步手测清单+三卡同场销（§10） | — |

**沿用 V1 零改动**：便签容器三态/窗口式（V1 §4.2）、输入与焦点路由（§4.3）、会话架构（§6）、权限闸与写租约（§7）、液态玻璃视觉（§8）、便签池 A+B（§9，IMPL-6 后置不变）。webui 徽章线（IMPL-5）不受 v2 影响。

## §4 全局圈选层设计

新节点 `VitCircleSelectLayer`（`app/tracks/vitnote/vit_circle_select_layer.gd`，Control）：由 VitNoteManager 持有并挂载为 manager 子节点；manager 升格为**圈选协调者**（手势→解析→胶囊→note 全链入口），原 lane 直连入口退役（§8）。

### 4.1 手势状态机（状态/转移级）

```
IDLE ──[Alt held ∧ LMB press ∧ 非焦点文本区内 ∧ 非clip体命中(D1)]──▶ PRESS_PENDING   （消费 press）
IDLE ──[其他]──▶ IDLE                                                            （不消费，全透传）

PRESS_PENDING ──[位移 ≥ CIRCLE_DRAG_START_THRESHOLD_PX(=4.0)]──▶ DRAGGING        （消费 motion；开视觉）
PRESS_PENDING ──[LMB release]──▶ IDLE                                            （消费；无动作——Alt+单击=无害空操作）
PRESS_PENDING ──[ESC / Alt release / RMB press]──▶ IDLE                          （消费；取消）

DRAGGING ──[motion]──▶ DRAGGING                                                  （消费；矩形=起点×当前点，钳制 viewport 内）
DRAGGING ──[LMB release]──▶ RESOLVING ──(同步解析 §5 → 胶囊 §4.5)──▶ IDLE        （消费；拆视觉）
DRAGGING ──[ESC / Alt release / RMB press]──▶ IDLE                               （消费；拆视觉；不出胶囊不出 note）

（活动态补充消费：PRESS_PENDING/DRAGGING 期间鼠标滚轮全透传——缩放不被圈选中断）
```

判定与常量：

- **发起条件**（IDLE→PRESS_PENDING 全部满足才消费 press）：① `InputEventMouseButton` 左键 pressed 且 `event.alt_pressed`；② **焦点文本区让位**：若 press 点落在当前 `gui_get_focus_owner()`（LineEdit/TextEdit）全局矩形内则不发起（打字区内的 Alt+点按归输入框，如框选文本；区外即使有焦点仍可圈——圈整个界面不被一个聊天输入框锁死）；③ **D1 clip 体让位**（§4.2）；④ 手势态自身重入 guard。
- **修饰键保持规则**：Alt 须持续按住（拖中 Alt 松开=取消）。与克隆拖拽同语义（F1 :2834 持续复查先例）。
- 阈值复用 lane 先例 4.0px（F5）；矩形规范化=abs 宽高、钳制 viewport 矩形内。
- **消费边界（仅活跃期拦输入）**：IDLE 态层完全透明（MOUSE_FILTER_IGNORE+`_input` 不消费任何事件）；活动态只消费手势自身事件流（LMB press/motion/release、ESC、RMB、Alt release），键盘字符、滚轮、其余按钮全透传。

### 4.2 Alt 边界与共存判定（裁定 1 的并存规格）

| 场景 | Alt 按住+左拖 | 依据 |
|---|---|---|
| 空 lane / 非交互区 / 机架 / 资料库 / 控制面 / 任意面 | **全局圈选** | 裁定 1「Alt+左键全局圈选」 |
| clip 体（2D lane 与 3D 视图） | **既有克隆拖拽让位**（圈选层不发起、不消费） | F1 现有手势；与裁定 1「时间线保留现有直接框选兼容」同源的保留原则 → **裁定边界项 D1**（§12） |
| 焦点文本控件矩形内 | 让位（输入框文本选择等） | §4.1 条件②，`_has_text_input_focus` 先例的精确化 |
| 无 Alt 的普通左拖（空 lane） | 既有 lane marquee（clip 框选）零改变 | 裁定 1「保留现有直接框选兼容」 |
| 无 Alt 的普通左拖（clip 体） | 既有选中/拖拽零改变 | 同上 |
| 胶囊+[N] 出口 | 保留（圈选完成后） | 裁定 1「双出口保留」→ **解释性裁定 D2**（§8/§12：plain marquee 回归纯选区，胶囊触发面移至圈选） |

D1 处置理由：全局无条件捕获会摧毁既有 Alt+拖克隆手势（用户未裁定取消它）；「clip 体让位」与裁定 1 自身的保留条款同构。**若决策侧裁定圈选优先于克隆，则克隆迁修饰键（建议 Ctrl+拖）另开卡**——本设计不改裁定，只定义边界并上交。

### 4.3 视觉反馈（截图工具式，裁定默认项）

- **暗幕**：活动层全幅 `_draw()` 绘半透明遮罩（α≈0.18 中性黑），**选区矩形内部镂空**（框内清晰、框外变暗——截图工具语义）。
- **亮框**：选区矩形 1.5px 高亮描边（accent 色）+ 四角短标记；拖拽中每帧重绘（仅 `_draw` 层，无 BackBufferCopy，帧率安全）。
- 现有 lane marquee 框（F5 组广播预览）与圈选框**不共存**：Alt 圈选发起在 lane marquee 之前消费 press，lane 侧永远收不到该 press，无双重框。
- 收尾动画：松开后遮罩 0.12s 淡出（复用 hint QUICK_FADE 先例节奏）。

### 4.4 输入捕获机制与挂载契约

- **机制**：`_input()` + 活动态 `get_viewport().set_input_as_handled()`。Godot 4 输入序=`_input`（树序逆序传播）→ GUI pick → unhandled；本仓既有先例（router `_input` F2、lane `_input` F5、manager `_input` F7）全走此路。
- **顺序不变量**：`_input` 消费者间的优先级=逆树序（树序末者先收）。manager 挂载于两面主场景根的**末位子节点**（F8），故先于 router（场景根，树序最早）与全部深层节点收到 press。**挂载契约**：VitNoteLayer 必须保持主场景根末位（或其后仅挂自门控 `_input` 的节点）；两面挂载点（vit_control_v_1.0.gd:156 / vit_dock_root.gd:182）在 V2 IMPL 卡中加注释固化。
- **防御性设计**（不裸赌传播顺序）：① 活动态 motion 消费后，矩形更新同时由 `_process` 读 `get_global_mouse_position()` 兜底驱动（即使个别 motion 被先收，视觉与终值不漂）；② 手测第 6/7 步显式验证共存零干扰；③ 分离窗口（DetachedViewportWindow / LLM_Chat_Window / agent webui Window）不在主窗口 canvas，圈选不达——**声明边界**，非缺陷。
- **clip 体让位实现**：press 点经 timeline 侧 `resolve_hit_target` 等价查询（manager 经组协议向 timeline 供给器问询 `hit_kind` 是否 begins_with "clip"）；非时间线区默认可发起。查询走 §6 注册处，不直连 lane。

### 4.5 出口（裁定 1 双出口保留）

圈选完成 → 胶囊出现在框右侧（V1 hint 逐沿用：生长动画、`[N]` 文案、8s 淡出、点击/快捷键/超时三出口——V1 §4.1）；胶囊文案前缀改「圈选」语义（如「[N] 打开便签 · 圈选 3 面」）。`vit_note_open` 快捷键消费逻辑（manager `_input` F7）零改动。

## §5 坐标命中解析（裁定 2 第一原则的规格化）

### 5.1 解析管线（松开时刻，同步执行）

```
rect_global（圈选层产物，唯一 GUI 载荷）
  ① 面枚举：遍历组 "vit_face_supplier" 的供给器 → vit_face_rects()（实时查询）→ 与 rect 求交
  ② 占比计算（每命中面）：
       selection_share = 交叠面积 / rect 面积      ← 摘要主口径（裁定 5「各面占比」）
       face_coverage   = 交叠面积 / 面矩形面积      ← 辅助口径（面被覆盖程度）
  ③ 过滤：交叠面积 < MIN_FACE_INTER_AREA_PX(=4.0) 剔除（与拖拽阈值同源的噪声门，非语义门槛）
  ④ 供给器解析：vit_face_resolve(rect_global) → 域对象摘要（每面独立，禁止 GUI 对象引用外泄）
  ⑤ 合并：faces[] 按 selection_share 降序 → 单 note 辖区快照（§5.3）
```

- 占比口径 D3（§12）：主口径 selection_share（「圈了多少落在该面」）；face_coverage 进 payload 供 agent 辅助判断（小面全被圈住 vs 大面被蹭一角，语义不同）。决策侧验收异议可只留主口径。
- **同点复面**：面矩形允许重叠（如面板悬浮于时间线上——面板非供给器，但未来任意两面可重叠），重叠即双计入 faces[] 各自占比，不强制主导面（裁定 5）。

### 5.2 域对象摘要形态（面标签+占比+对象清单）

```jsonc
// 单面摘要（faces[] 元素）
{
  "face_id": "timeline@vp_main",     // 供给器 identity
  "face_kind": "timeline",
  "label": "轨道时间线",              // 人类可读面标签
  "selection_share": 0.72,
  "face_coverage": 0.55,
  "domain": { ... }                   // §6 各面域对象清单（纯 id/值，无 GUI 引用）
}
```

### 5.3 跨面合并规则

- **合并为单 note**（裁定 5）：一场圈选至多一胶囊一 note；面板头部辖区摘要=`「轨道时间线 72% · 机架 28%」`式一行（V1 `_rebuild_summary` 的多面版）。
- 面间对象**不跨面去重**（同 face_id 域内自理）；faces[] 顺序=selection_share 降序，无主导面语义。
- **空/无供给器命中**（faces 为空）：合法态=空辖区 note（设计默认项，沿 V1 §4.1.4 语义）；面板摘要「空辖区」。

### 5.4 与 agent 消费的边界

解析产物只含域对象引用与只读快照值；agent 侧消费走 V1 §5.2 既有路线（range_context 富化，不建平行管道），faces[].domain 是其输入。**GUI 命中信息（rect 除外）不进 agent 载荷**——裁定 2 第一原则的可验证判据（手测第 3 步断言 payload）。

## §6 供给器契约（函数级）+四类面逐一

### 6.0 契约（duck-typing 四函数+组注册）

```gdscript
## 供给器契约——任一 Control/Node 实现以下四函数并加入组 "vit_face_supplier" 即成为圈选命中面。
## 生命周期：_ready 入组、_exit_tree 离组；隐藏=返回空矩形（逻辑上自动注销）。

func vit_face_identity() -> Dictionary:
	# {"face_id": "<kind>@<scope>",        # 全局唯一：kind@viewport/容器 scope
	#  "face_kind": "timeline|rack|library|control|custom",
	#  "label": "<人类可读面标签>",          # 进 note 摘要与 payload
	#  "priority": 50}                      # 同位排序提示（非主导面裁决，仅展示排序）

func vit_face_affordance() -> Array[Dictionary]:
	# [{"action": "observe",                # observe（只读）| propose（经确认流的提案）
	#   "verbs": ["listen", "query"],       # 用户语干（"听听看""低频怎么样"）
	#   "objects": ["track", "clip", "time_window"],
	#   "note": "轨级六项写参经 RiskConfirm/RiskCeiling 闸（V1 §7）"}]
	# ——「这是什么（identity）+这能干什么（affordance）」（裁定 3 用户原语）

func vit_face_rects() -> Array[Rect2]:
	# 命中区全局矩形（可多块）。解析时实时调用，禁止缓存（窗口/抽屉/布局随时变）。
	# 不可见面返回 []。

func vit_face_resolve(rect_global: Rect2) -> Dictionary:
	# {"face_id": ..., "entries": [...],     # 域对象清单（id/label/只读当前值）
	#  "time_window": {...}?,                # 时间语义面可选
	#  "snapshot": {...}?}                   # 状态快照面（control）用
	# 失败语义：异常/空数据返回 {"face_id":..., "entries": []}——空清单是合法辖区，不阻塞他面。
```

注册处=VitNoteManager（组查询+解析编排 `resolve_circle(rect)`，§5.1 管线 ①–⑤ 宿主）。**不做 autoload**（沿两面挂载先例 F8，组协议即注册表——与 `vit_note_manager`/`vit_track_lane_2d_interaction` 组先例同构）。

### 6.1 轨道时间线供给器（lane 通知路径的 V2 改造位）

- **identity**：`face_id="timeline@<viewport_id>"`，kind=timeline，label=轨道时间线。
- **affordance**：observe（轨/clip/时间窗问答、audition A/B 区间试听）；propose（轨级写参=V1 §7.1 六项 bounded_reversible+RiskConfirm 升级链，clip_scope 竖线=CLIPSCOPE-AGENT-1 线）。
- **face_rects**：各行 `Track_Split/Right_3D_Wrapper` 全局矩形（经 `_mapping.row_at_global_point` 同源的行枚举）；行折叠/隐藏即剔除。
- **resolve**：**复用 F6 既有采集**——`_collect_marquee_hits_global(rect)`（跨 lane 组聚合）→ entries=clip 清单（clip_id/track_id/start/length）+track_ids 去重；time_window=命中包络，空命中时=rect×时间轴映射派生（V1 §5.1 派生规则）。lane 级 `_notify_vit_note_hint` 直连链退役（§8），采集函数保留改供供给器调用。

### 6.2 机架供给器

- **identity**：`face_id="rack@<viewport_id>"`（每视口一个，绑当前选定轨），kind=rack，label=「机架·<轨名>」。
- **affordance**：observe（链上节点/参数/连线只读问答）；propose（装载/移除/写参=RiskConfirm 级，V1 §7 权限档引用）。
- **face_rects**：graph_rack_dock_adapter 视图全局矩形（整个机架面板）。
- **resolve**：entries=`get_rack_nodes(选定轨)`（F11）全链清单（node_id/plugin 名/绑定轨/关键参数当前值）；time_window 无。**v2 诚实降级**：无逐节点命中矩形——面命中即整链入摘（域引用完整、空间精度粗）；逐节点占比= v2.x（需机架视图提供节点级矩形枚举）。

### 6.3 资料库供给器

- **identity**：`face_id="library@left_drawer"`，kind=library，label=资料库。
- **affordance**：observe（文件/插件目录/编组只读）；propose（拖入装载走既有拖放线，note 内=提案语义）。
- **face_rects**：左抽屉打开时 MediaPoolAdapter 全局矩形；关闭返回 []。
- **resolve**：按当前 tab 报**可见条目**清单——Files=路径（`_places_roots` 展开的可视行）、Plugins=`_available_plugins`（名称/厂商/类别）、Groups=编组；上限 50 条+「…及 N 条截断」标记。**v2 诚实降级**：Tree 无逐行全局矩形公开 API——以条目级清单代替逐行占比（同 6.2，域引用完整、行级精度粗）。

### 6.4 控制面状态快照供给器（裁定 4 特例形态）

- **identity**：`face_id="control@main"`，kind=control，label=控制面。
- **affordance**：**observe only**（v2 快照面不承载编辑提案——走带/轨参编辑仍归 GUI 控件本身；agent 可引用快照值作判断输入）。
- **face_rects**：两块——走带条（Top_Transport_Bar）全局矩形 + 各轨 `Header_2D` 全局矩形（逐轨声明，resolve 时只纳入相交行）。
- **resolve（snapshot 形态）**：
```jsonc
{"snapshot": {
  "bpm": 120.0,                                  // main_ui_controller.get_project_bpm（F10）
  "playhead_s": 34.2, "is_playing": true,        // _transport_position_now（F10）
  "loop": {"start_s": 8.0, "end_s": 16.0} | null, // _time_selection_state（F10）
  "tracks": [                                    // 相交行的轨道头当前值
    {"track_id": "...", "name": "Drums", "volume_db": -6.0, "pan": 0.0, "mute": false}
  ]}}
```
  域引用=track_id 级（轨参数当前值是快照不是承诺——note 问「Drums 现在多响」答快照，改值走 GUI/轨级提案）。

### 6.5 新面接入指南（裁定 3「注册全局矩形+实现供给器」）

1. 建供给器脚本实现 §6.0 四函数（identity 唯一 face_id；affordance 如实声明 observe/propose 边界）；`_ready` 入组 `vit_face_supplier`。
2. `vit_face_rects()` 返回面实时全局矩形（多块可分：控制面=走带+轨头两块先例）。
3. （推荐）面内含悬浮可交互层的，把该层加入 `vit_floating_interactive` 组（§7 修复锚点 B 的旁路组）——输入不被 router/仲裁器误吃。
4. 无时间语义可不实现 time_window；快照面用 snapshot 键；失败返回空 entries 不抛。
5. 自验：任意 Alt 圈住新面 → 胶囊摘要含该面 label+占比+entries；手测并入手测清单第 3/4 步扩展。

## §7 面板输入 bug：根因取证面与修复锚点（裁定 7）

症状（F14 一手证据）：dock 面 N 键开窗成功，面板**含收起按钮**全部鼠标交互无效；键盘链通、鼠标链灭；按钮接线与 mouse_filter 静态核对在位。

### 7.1 三候选与证据现状

| 候选 | 内容 | 静态证据 | 先验 |
|---|---|---|---|
| **B `_input` 阶段消费→GUI 命中测试未达（主嫌疑）** | router `_input`（F2）在 GUI 投递前吃掉画布内左键：lane 转发先行（:1730-1733）+interactive 旁路根枚举不含 VitNoteLayer（F3 `_candidate_interactive_hit_roots` :1990 只扫工具栏/行/空区/自身直接子级）→ 面板/胶囊上的 press 被 lane marquee 起手消费，`set_input_as_handled()` 后 GUI pick 从未发生 | F2+F3 全链直读成立；且解释「含收起按钮」：Button 虽是 BaseButton，但旁路检查在转发之后且根枚举不含面板子树，两道闸都失效 | **高** |
| A 上层拦截（叠层 STOP 控件在 pick 顺序之上） | VitNoteLayer 为 dock 根末位子（F8）=GUI pick 最高层；静态未见其后添加的全幅 STOP 层 | 反证居多 | 低 |
| C mouse_filter/焦点链 | manager IGNORE:27 与面板子件 filter 静态正确（F14 已核）；残余可能=焦点抢占/`_gui_input` 未达的边缘 | 已被 F14 排除大半 | 低 |

**同根推论**：router 脚本两面同挂（F2），legacy 面「面板悬浮于时间线画布上方」时同链成立——dock 面只是把复现率变成 100%（工作区铺满窗口）。取证步骤含 legacy 验证以证实/证伪同根。

### 7.2 排查路径（取证卡/IMPL 卡可执行）

1. **探针 A（hover 归属）**：dock 面悬停面板按钮，打印 `get_viewport().gui_get_hovered_control()` 路径（可临时挂 `_process` 节点或用 remote 场景树）。报告面板子件→候选 A 出局、pick 顺序正常；报告其他 STOP 控件→候选 A 成立，记录节点路径。
2. **探针 B（GUI 到达性）**：面板 `gui_input`/`mouse_entered` 与按钮 `pressed` 各接调试打印；点击收起。零打印=事件未达 GUI（候选 B 成立佐证）；有打印但无动作=接线/焦点问题（候选 C 方向）。
3. **探针 C（`_input` 消费链）**：开 `DEBUG_TRACK_UI_INPUT_DIAG`（router :1757 既有诊断）点面板按钮，看 `[track_ui_diag][router]` 是否打出 press+arbitrator_consumed——打出即 press 走了 router 消费链（候选 B 实锤）；同时观察点面板处是否起 marquee/选中其后 clip（候选 B 的行为签名：**点击穿透面板操作了时间线**）。
4. **legacy 同根验证**：legacy 面把面板拖到画布上方重复 1–3；同象=同根，异象=face 特有差异（回到候选 A/C 补查 dock 后挂子节点）。
5. **修复后回归**：§10.2 第 8 步三态全操作+时间线编辑回归（修复不得改变画布内正常手势）。

### 7.3 修复锚点（设计级，实现在 V2-INPUT-FIX-1 卡）

- **锚点 A（router 顺序修正）**：`timeline_track_list_drop_router.gd:_input` 左键分支——把 `_global_point_targets_interactive_control` 旁路检查移到 `_forward_track_lane_2d_pointer_event` **之前**（同函数右键/双击分支已有 check-before-consume 先例：:1725-1729）。
- **锚点 B（旁路根扩展）**：`_candidate_interactive_hit_roots`（:1990）增加枚举组 `vit_floating_interactive` 的根（VitNoteLayer 及未来悬浮层 `_ready` 入组）；`_control_tree_has_interactive_hit` 随之能命中面板内 BaseButton/LineEdit（:2063 清单已认）。
- **锚点 C（仲裁器同款教学）**：TimelineInputArbitrator `_should_accept_mouse_at_global`（:211）加同组短路——点落在 `vit_floating_interactive` 控件矩形内即 IGNORE（补 F4 盲区：非按钮控件体如 PanelContainer 底板不被 STOP 吃掉；只认按钮时面板底板上的 press 仍会被仲裁器 empty_lane 分支消费）。
- 三锚点同卡交付+探针复测；`vit_floating_interactive` 组即 §6.5 接入指南第 3 步的旁路组——输入修复与供给器架构共用同一注册机制。

## §8 旧路径处置与 V1 文档修订

| 路径 | 处置 | 依据/兼容期 |
|---|---|---|
| `_notify_vit_note_hint`（vit_track_lane_2d.gd:2311 lane→manager 直连）+ manager `present_marquee_hint` 入口 | **退役**（V2-IMPL-A 同卡原子删除，调用点与入口一起） | plain marquee 回归纯选区语义（D2 解释性裁定）：裁定 1 的「胶囊+[N] 双出口保留」读作**圈选完成后**的出口保留；双触发并存（普通框选也出胶囊）会造成同屏竞争胶囊+语义混乱。进程内调用无持久化耦合，无跨版本兼容负担；备选（保留双触发）在 §12 D2 登记，验收异议可回滚 |
| `get_clip_marquee_hits_global`/`_collect_marquee_hits_global`（F6 采集函数） | **保留并转供**：时间线供给器 §6.1 的 resolve 实现体 | 采集与通知分离——通知退役不动采集 |
| `_broadcast_marquee_preview`/`set_global_marquee_preview` 组广播（F5） | **保留零改动** | 跨 lane marquee 预览绘制用，与 note 链无关 |
| V1 文档 | **加修订标注**（本次随卡落）：§4.1 触发面改圈选、IMPL 序列与 V2 排程的衔接 | 见 VITNOTE_V1_DESIGN.md 头部标注 |

## §9 上下文协议 v2（payload）

`vit_note` 块 v2 形态（V1 §5.1 的多面扩展；agent 侧消费走 V1 §5.2 range_context 富化，faces[].domain 为输入）：

```jsonc
"vit_note": {
  "note_id": "note_<id>",
  "origin": "global_circle",
  "rect": {"x":0,"y":0,"w":0,"h":0},
  "faces": [ /* §5.2 单面摘要数组，selection_share 降序；空辖区=空数组 */ ],
  "note_state": "composing | active"
}
```

- v1 字段 track_ids/clip_ids/time_window **上移入** timeline 面 domain（不再顶层平铺）；agent 侧若已实现 v1 消费（IMPL-2 未做，无存量），无需兼容垫层——直接以 v2 形态为首个实现目标。
- 面板 `context` 组包链零结构改动（manager `get_agent_context` F7 复用主控台组包；dock 面空 context 降级维持——问 dock 便签时辖区快照在 note payload 自身，不依赖主控台组包，7878 问答不受阻——DOCK-MOUNT 既有结论）。

## §10 测试与验收

### 10.1 headless 可断言面与手测覆盖边界（如实申报）

- **可 headless 断言**（纯逻辑与场景解耦后可用 `godot --headless -s` 探针脚本跑，F15 无测试基建下的最小形态）：①手势状态机（§4.1 状态/转移表逐条注入 InputEvent 断言消费/转移——状态机实现为无场景依赖纯类）；②命中解析（§5.1 管线：伪造供给器 rect/entries 断言占比/排序/合并/空辖区/MIN 过滤）；③供给器契约（四函数签名+组注册+resolve 空/失败语义）。探针脚本归 tools/，作 V2 IMPL 卡的辅助验收，**不替代手测**。
- **必须手测**（渲染/IME/跨面真实矩形/输入链）：§10.2 全部步骤。边界声明：本卡为设计档零代码，以上为 V2 实现卡的验收面预定义。

### 10.2 V2 九步手测清单（裁定默认项「按新交互重写」；M8 终裁挂点，一场销三卡）

前置：用户从 Godot 拉起 DAW 前端→dock 工作区进入目标工程（AGENTS §5 手测入口裁定）；agent 三件套在跑。

| # | 步骤 | 判据 |
|---|---|---|
| 1 | **圈选触发**：时间线空区 Alt+左拖 | 拖拽中暗幕+亮框（框内镂空清晰）；松开胶囊出现在框右侧，文案含 [N] |
| 2 | **双出口**：先按 [N] 开便签；再圈一次改点胶囊 | 两出口都开出 note；[N] 在文本输入态不触发（焦点守卫） |
| 3 | **跨面合并**：从时间线拖到机架+资料库横跨三面 | 单 note；摘要=「轨道时间线 x% · 机架 y% · 资料库 z%」+各面对象清单；payload 无 GUI 引用（domain 纯 id/值） |
| 4 | **控制面快照**：圈住走带条+2~3 个轨道头 | 摘要含 BPM/播放位置/循环区/相交轨参数当前值；未相交轨不入 |
| 5 | **空辖区**：圈界面空白角（无供给器区） | 空辖区 note 可开、可发（7878 有回复） |
| 6 | **取消路径**：拖拽中 ESC；拖拽中松 Alt；Alt+单击不过阈值 | 三者皆无胶囊无 note 无残留遮罩；Alt+单击无任何副作用 |
| 7 | **共存边界**：普通左拖空 lane 仍 clip 框选；Alt+左拖 clip 体仍克隆；时间线既有编辑（选/移/裁/画）零回归 | 裁定 1 兼容项+D1 边界生效 |
| 8 | **面板三态+修复验证**（dock 面）：开窗→IME 中文输入→发送辖区问答→收起→锚点重开→拖把手移动；双 note 并存 | **含收起按钮在内全部鼠标交互有效**（§7 修复判据）；辖区上下文进回复（agent 答的是域对象） |
| 9 | **三卡同场销收口**：复核 VITNOTE-IMPL-1 条件项（M8 最小闭环：圈选→胶囊→便签→IME→观察问答→收回→重开——V1 §11 判据的 V2 交互版=步骤 1/2/8）与 VITNOTE-DOCK-MOUNT-1 条件项（dock 默认入口挂载可见=本场全程 dock 面） | 裁定 8：V2 手测一场销三卡；本清单 1–8 全过=第 9 步记录两张条件卡销项 |

## §11 实现排程建议（IMPL 拆卡，决策侧裁剪用）

| 卡 | 域（文件域） | 内容 | 依赖/并行 | 验收 |
|---|---|---|---|---|
| **V2-INPUT-FIX-1** | Godot·router+仲裁器（timeline_track_list_drop_router.gd、TimelineInputArbitrator.gd、vitnote 层入组） | §7.2 探针取证落地+§7.3 三锚点修复+legacy 同根验证 | **先行独立**；与 V2-IMPL-A 不同文件域可并行 | 探针证据链+手测第 8 步面板交互项+时间线编辑回归 |
| **V2-IMPL-A** | Godot·vitnote 层（vit_circle_select_layer.gd 新建、vit_note_manager.gd、lane 通知退役位） | §4 全局圈选层（状态机/视觉/Alt 边界/挂载契约）+§8 退役 | 本文档；可与 INPUT-FIX-1 并行 | headless 状态机探针+手测 1/2/6/7 |
| **V2-IMPL-B** | Godot·注册处+轨道+控制面（manager 解析编排、timeline 供给器新建、control 供给器新建、lane 采集转供） | §5 解析管线+§6.0 契约+§6.1/§6.4 两面+§9 payload | 依赖 A（圈选出口） | headless 解析/契约探针+手测 3（单面）/4 |
| **V2-IMPL-C** | Godot·机架+资料库（rack/library 供给器新建） | §6.2/§6.3+诚实降级注释 | 依赖 B（注册处） | 手测 3（跨面完整版） |
| **V2-IMPL-D** | Godot·面板+手测 | 多面辖区摘要呈现（`_rebuild_summary` 多面版）+九步清单执行（三卡同场销记录） | 依赖 A/B/C+INPUT-FIX-1 | 手测 1–9 全过+条件卡销项落簿 |

说明：agent 侧无新卡——payload 为 additive（IMPL-2 未实现无存量），agent 消费仍归 V1 序列 IMPL-3（range_context 富化以 faces[].domain 为输入）；IMPL-4（写租约）/IMPL-5（webui）/IMPL-6（便签池）不受 V2 影响。排程风险点：INPUT-FIX-1 的 router 顺序修正触碰时间线主输入链，须带时间线编辑回归（手测第 7 步是其验收一部分）。

## §12 开放项与裁定边界项（决策侧待确认/异议可改）

| # | 事项 | 本设计取值 | 备选 | 状态 |
|---|---|---|---|---|
| D1 | **Alt+拖与克隆拖拽共存边界**：clip 体上 Alt+左拖 | 克隆让位（圈选不发起，F1 既有手势保留） | 圈选无条件全局优先→克隆迁 Ctrl+拖（改用户习惯，需用户裁定） | **待决策侧确认**（裁定 1 的边界解释，非改裁定；用户异议即回决策侧带用户） |
| D2 | 「胶囊+[N] 双出口保留」的解释：plain marquee 是否仍出胶囊 | 不出（D2：双出口=圈选完成后的出口；plain marquee 回纯选区，§8 退役） | 保留双触发（普通框选与圈选都出胶囊，竞争由 `_dismiss_hint_immediately` 互斥） | 解释性裁定，验收异议可回滚 |
| D3 | 占比口径 | 主=selection_share、辅=face_coverage 双报 | 仅主口径 | 设计默认，验收可裁 |
| — | 圈选触发键固定 Alt（不做用户可配置修饰键） | v2 固定，v2.x 再配置化 | — | 设计默认（裁定 1 明示 Alt） |
| — | 机架/资料库逐条目矩形精度 | v2 整面命中+全清单（域引用完整，空间精度粗，§6.2/§6.3 诚实降级注释） | v2.x 逐节点/逐行矩形 | 设计默认 |
| — | 圈选不达分离窗口（Detached/Chat/webui Window） | 声明边界非缺陷（§4.4） | — | 设计默认 |
