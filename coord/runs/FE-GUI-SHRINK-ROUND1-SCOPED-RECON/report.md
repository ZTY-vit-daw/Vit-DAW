# FE-GUI-SHRINK-ROUND1-SCOPED-RECON：第一轮限定面勘察简报（只读）

- 勘察时间：2026-10-10 17:00 前后；GUI 设计会话（GLM）产出。
- 背景：全量五节盘点卡 FE-GUI-SHRINK-RECON-1 仍在闲时队列（第 251 位），用户裁定先做**限定面勘察**启动第一轮讨论；本简报只覆盖第一轮面（初始界面布局），全量盘点产出后交叉核对。
- 前端仓 `D:\Godot\project\vit-daw-frontend` 仓库外只读扫描，零代码。

## 0. 头号事实：现役主面是 Vit-Dock，不是 legacy 三栏

- 启动链：`project.godot:14` main_scene=`app/startup/start_page.tscn`；start_page.gd:7-12 `MAIN_WORKSPACE_SCENE=vit_dock/scenes/VitDockRoot.tscn`，legacy（`app/legacy/Vit control V1.0.tscn`）仅 `VIT_USE_LEGACY_MAIN=1` 回退，注释明示"不作为活跃结构目标"。
- 讨论中说的「三栏」实际是 **Vit-Dock 的默认工作区呈现**：左=边缘抽屉资料库，中=时间线视口（58%），右=机架视口（42%）。

## 1. 初始界面装配树（VitDockRoot.tscn:43-223 + vit_dock_root.gd）

```
VitDockRoot
└─ TopLevelShell (VBox)
   ├─ AppHeaderShell            # File/Edit 菜单·Save·工作区预设 tabs·New Window
   ├─ TopTransportBar           # LCD(时间/BPM/TAP/拍号)·走带按钮·chat 开关
   ├─ MainBody (HBox)
   │  ├─ LeftShell
   │  │  ├─ LeftEdgeDrawerHost  # 资料库抽屉（默认展开 show_media_pool）
   │  │  └─ LeftActivityRail    # 唯一柄「⋮」(compact_handle_mode)
   │  └─ ContentStack
   │     ├─ WorkspaceShell→DynamicSplitTreeRoot  # 视口分割树
   │     │  └─ AgentCapsuleOverlay               # agent 提示胶囊浮层
   │     ├─ BottomActivityRail   # 「状态」(debug panel)
   │     └─ BottomEdgeDrawerHost
   └─ ProjectSaveStatusBar      # 保存进度条
LLM_Chat_Window                 # 独立 Window，走带栏 chat 开关切换 (vit_dock_root.gd:1269-1284)
```

- 默认布局=`default_arrange` 预设：水平分割 0.58，左 `vp_timeline_main`(timeline)｜右 `vp_rack_main`(graph_rack)（workspace_preset_repository.gd:25-67）。
- 另两个内置预设：`sound_design`（机架+频谱｜钢琴卷帘）、`dual_screen_dual_track`（时间线｜机架/钢琴卷帘）(:70-157)。
- 每个视口有标题栏：视图类型切换菜单(7 种)·上下文过滤·面包屑·水平/垂直分割·弹出(detached window)·关闭（viewport_title_bar.gd:6-40）。
- 视口注册表 7 视图：Timeline/Graph Rack/Piano Roll/Spectral/Detail/Mixer/Media Pool（viewport_registry.gd:7-55）。
- 核心视口保护：timeline/graph_rack 最后一个不可关（workspace_layout_manager.gd:155-158）。

## 2. 左栏·资料库（left_library_dock.gd，1182 行，经 MediaPoolAdapter.tscn 装入抽屉）

- 四 tab：Files(Places)/Plugins/Groups/Notes（:14-17；tscn 里 tab 文本 Files/Plugins/Groups）。
- Files tab：工程轨道/片段树（来自 ProjectRepository）+ 本地 Places 书签目录（首次展开扫描层级，:591-615）；➕添加文件夹按钮；搜索框。
- Plugins tab：内核 PluginList（settings_client.plugin_list_available_no_wait，:242-267）；厂商→类别→插件三级树（:618-683）。
- Groups tab：编组列（track_group_list_panel.tscn）。
- Notes tab：便签列（单击重开收起态/右键删除；新建走视窗 Q 圈选，:424-588）。
- **拖拽源**：`_on_tree_get_drag_data`(:1060-1102)——插件行产出 `{"type":"plugin", plugin_name/plugin_identifier/manufacturer/category/format/is_instrument,...}`；文件行产出 `{"drag_type":"file_import",...}`。**两族 key 不一致（type vs drag_type）**。
- 拖拽失败回落：file_import 拖拽落空→导入到选中轨（:1105-1126）。

## 3. 中栏·时间线视口（timeline_dock_adapter.gd，3436 行 → 装配 app/timeline 轨道栈）

- 轨道行=Track_Row_Controller.gd（约 3800+ 行）：轨头（名称/输入菜单/频谱模式 Split/Link/Sum/电平表）+ 轨道 lane + clips。
- 轨道行右键菜单（timeline_dock_adapter.gd:1966-1970）：New folder / 新建轨道 / 删除当前轨道 / New group from this track。
- **拖拽落点①**：轨行接插件拖放（vit_track_drop_zone.gd:13-19 → `vit_track_apply_plugin_drop`；timeline_track_list_drop_router.gd:3131-3137 空白区路由）→ 装载到该轨。
- 文件导入同样落轨行（`vit_track_apply_import_drop`）。
- 虚拟轨道列表（24 轨阈值以上虚拟化，timeline_track_list_drop_router.gd:32-44 环境变量族）。

## 4. 右栏·机架视口（graph_rack_dock_adapter.gd 877 行 → Vit_Graph_Rack.gd 约 4600+ 行）

- 节点画布：插件节点卡（GraphEdit 系）、连线拖拽（端口）、落格吸附（rack_guide_snap_enabled）、缩放(±/0 键)、右键平移。
- **拖拽落点②**：`_can_drop_data`/`_drop_data`(:4560-4610)——接 `type=="plugin"` 拖放直接加节点（需解析 kernel track_id：绑定轨→选中轨，缺则告警拒收）。
- 节点右键菜单（:4446-4451）：🗑️销毁节点/Delete、镜像克隆、审查物理路径、审查底层属性、Rename Macro（宏节点）；Clip 占位节点变体「删除 Clip」。
- Library 双击 `auto_spawn_node`(:1634)——**当前无调用者**（库双击不直接装机架）。

## 5. 已知痛点核对（第一轮讨论输入）

1. **插件拖拽横跨两栏**：左库→右机架要横穿整个中栏时间线；两条装载路径（拖轨行/拖机架画布）语义不同（装载到轨 vs 画布加节点），且机架落点依赖 track_id 解析（绑定轨优先，无则选中轨——归因面已知妥协）。
2. 右键装载已在卡池：FE-RACK-CTX-PLUGIN-LOAD-1（池序 46，机架右键装载插件检索弹窗，数据源复用 `_available_plugins`）。
3. 视口机制全家桶（分割/弹窗/视图切换/三预设/New Window/detached viewport）与用户已裁定的收缩方向（不多开视口、功能归拢）正面冲突——**收缩候选肥肉**。
4. 每视口标题栏 7 视图切换+分割+弹出按钮：多视口范式的入口面，默认布局下两个视口各一套。
5. 左库 type/drag_type 两族 key 不一致（技术债，收缩时留意）。

## 6. 遥测写者红线面（与 REFSCHEMA-M8 报告交叉核对一致）

| 红线 | 锚点 | 内容 |
|---|---|---|
| `kernel_prepared_waveform_envelope_<clip>` 写者 | telemetry_manager.gd:1963（3376 行 autoload） | 波形包络物化→桥快照→agent harness 消费 |
| `mixboard_<msec>` 写者 | mix_client.gd:39-42/:65/:71 | mixboard 观察特征请求 id 构造 |
| mixboard 消费面 | app/shell/LLM_Chat_Controller.gd:908-1136 | chat 侧观察特征请求/状态卡（数据面不可断） |
| vsp 资产面 | app/kernel/autoloads/vsp_asset_adapter.gd:861-880 | mixboard 快照物化选项 |
| 展示面（数据来源=遥测） | app/tracks/track_scene/vit_track_header_meters.gd、vit_dock/views/mixer/* | 轨头表/混音条——布局可动，数据链不可断 |

红线口径：以上**写者与数据链不可动**；承载它们的 UI 容器布局可收缩，但不得改 ref 构造/请求语义。

## 7. 待讨论开放问题（第一轮）

1. 三栏比例与归拢目标：左库四 tab 是否保留？抽屉 vs 常驻？
2. 机架与时间线的占屏关系（现 58/42 水平分；legacy 是上下分）。
3. 视口机制入口（分割/弹出/视图切换/预设 tabs/New Window）收缩到什么程度——全隐藏还是降级到「高级」入口？
4. 插件装载动线：右键弹窗（卡 46）之外，拖拽路径是否保留双落点？
5. 顶栏/走带栏/底部状态栏的功能归拢边界。

## 附：可复跑勘察命令

```bash
cd /d/Godot/project/vit-daw-frontend
rg -n "MAIN_WORKSPACE_SCENE|LEGACY_MAIN_SCENE" app/startup/
rg -n '^\[node' vit_dock/scenes/VitDockRoot.tscn
rg -n "default_arrange|sound_design|dual_screen" vit_dock/core/workspace_preset_repository.gd
rg -n "func _on_tree_get_drag_data|type.*plugin|drag_type" app/browser/left_library_dock.gd
rg -n "vit_track_apply_plugin_drop|vit_track_can_drop_plugin" app -g '*.gd'
rg -n "func _can_drop_data|func _drop_data|add_item" app/rack/Vit_Graph_Rack.gd | head -20
rg -n "mixboard_|kernel_prepared_" app tools -g '*.gd'
```
