# Vit-DAW V0.5 `te::RackType` 终极重构方案

**依据**: `docs/ROUTING_ENGINE_DIAGNOSIS.md`  
**目标**: 以 `te::RackType` 为核心，重构现有线性 `PluginList` 路由，建立一个保留音乐工程语境的层级化 2D 机架系统，并为未来接入 AIGC / TTS / RVC / Web API / 自定义模型平台预留统一图执行框架。

---

## 1. 总体原则

### 1.1 用户视角不变
- 保留当前分层 UI，作为用户的核心心智模型：
  - `Top`
  - `Z1` MIDI
  - `Z2` Instrument
  - `Z3` Audio
- `Top` **不是可加载层**，只做：
  - 时间轴 Clip 映射
  - Clip 路由入口显示
  - Bridge 回灌结果映射
- `Bus` **不是独立层**，而是 `Z3` 内的一类特殊节点。

### 1.2 底层执行模型重构
- 每条 Track 只保留 **一个**轨级 `RackInstance`，作为唯一 DSP 宿主。
- 所有用户插件实例都进入 Track 级 `RackType`。
- Clip 不创建自己的插件实例，只维护对 Track Rack 节点的动态路由绑定。
- 未来异步 AI 节点不直接进入 `te::RackType` 音频线程，而通过 Compiler + Bridge 进入工程。

### 1.3 设计定位
本方案不是“更复杂的插件链”，而是：
- DAW 实时图
- Max/MSP 式控制图
- ComfyUI 式异步工作流
- AIGC 结果桥接回 DAW

四者融合的统一架构。

---

## 2. 核心架构定案

### 2.1 单轨单 Rack
每条可路由轨：
- 保留一个 `te::RackInstance`
- 保留必要轨外壳插件：
  - `VolumeAndPanPlugin`
  - `LevelMeterPlugin`
- 不再把用户效果链留在线性 `pluginList`

### 2.2 主链与副链
- **`G_track`**：轨道主链路  
  凡不是由某个 `clip_id` 显式指定加载的链路，都属于全轨主链。
- **Clip 副链**：Clip 显式指定加载的局部链路  
  通过 `clip_id -> node_id` 的动态路由表接入 Track Rack。

### 2.3 物理实例单例
- 所有物理插件实例只寄宿在 Track 级 Rack 中。
- 多个 Clip 若共用同一重型 VSTi（如 Serum），只共享同一个物理实例。
- Clip 只绑定输入路径，不重复实例化插件。

---

## 3. Visual Zone 与 Execution Domain 解耦

### 3.1 Visual Zone
用户看到的固定层级：
- `Top`
- `Z1`
- `Z2`
- `Z3`

### 3.2 Execution Domain
节点真实执行域：
- `realtime_audio`
- `realtime_midi`
- `bus_proxy`
- `bridge_injector`
- `async_ai`
- `web_api`
- `control_domain`

### 3.3 原则
- AIGC 节点可以在 UI 上被拖入 `Z2` 或 `Z3`
- 但 Compiler 会识别其真实域，不会把它塞进实时 Rack 图
- UI 不需要像 ComfyUI 那样无限黑板化

---

## 4. Track 级数据模型

### 4.1 轨级核心对象
新增建议类：
- `VitTrackRackManager`
- `VitPluginIdentity`
- `VitClipRouteRegistry`
- `VitZoneRouter`
- `VitGraphValidator`
- `VitDagChecker`
- `VitParallelMergePlanner`
- `VitGraphSwapCoordinator`

### 4.2 ValueTree 扩展
挂载在 Rack `PLUGININSTANCE` 节点上：
- `vit_zone_id`: `Z1|Z2|Z3`
- `vit_clip_scope`: `track|clip:<clipItemId>`
- `vit_layout_x`
- `vit_layout_y`
- `vit_node_role`:  
  `processor|bus_send|bus_return|bus_insert|bus_sidechain|merge|audio_injector|clip_proxy`
- `vit_template_role`:  
  `eq|comp|bus|instrument|other`
- `vit_recommended_zone`
- `vit_bus_target_track_id`
- `vit_asset_ref`
- `vit_origin_bpm`
- `vit_origin_key`
- `vit_take_stack_id`
- `vit_active_take_id`
- `vit_async_ghost_state`
- `vit_warp_state`

轨级 `ValueTree`：
- `vit_routing_mode`: `zone_auto|free`
- `vit_graph_revision`
- `vit_graph_schema_version`
- `vit_migrated_from_linear`

### 4.3 Top 层表达
`Top` 不作为物理插件区，不写入 `PLUGININSTANCE.zone_id`。  
Top 层只通过以下状态表达：
- `clip_routes[]`
- `clip_proxy_nodes[]`
- Clip / Track 原生 `EditItemID`

---

## 5. 路由语义

### 5.1 隐式重力路由
`zone_auto` 下：
- MIDI Clip / MIDI 节点默认流向 `Top -> Z1 -> Z2 -> Z3`
- Audio Clip 默认跳过 `Z1/Z2`，直接落到 `Z3` 首节点
- 默认重力补边不依赖 Tracktion 的 `canAutoConnect` 作为最终真源
- 实际采用：
  - `addPlugin(..., false)`
  - 再由 `VitZoneRouter` 调用 `addConnection`

### 5.2 显式自由连线
允许用户自由连接，但必须经过防御层。

### 5.3 Bus 节点
Bus 是 `Z3` 内特殊节点，不是单独视觉层。  
Bus 节点执行语义包括：
- `bus_send`
- `bus_return`
- `bus_insert`
- `bus_sidechain`

---

## 6. 五大底层安全机制

### 6.1 跨界碰撞保护
新增：
- `VitZoneBufferAdapter`

职责：
- 不仅做 Dummy Padding，还必须做 **Wrap & Passthrough**
- MIDI-only 后接 Audio 插件时：
  - 给当前错误域插件注入静音 Audio Buffer
  - 同时把原始 MIDI lane 平行旁通到该节点输出侧
- Audio-only 进 Z1 / MIDI-only 节点时：
  - 保留 Audio lane 继续旁通
  - 允许控制/Event lane 并行通过
- 目标不是“让错误节点勉强不崩”，而是“错误节点存在时整条链路仍不断崖”
- 避免不匹配 `processBlock` 崩溃

### 6.2 PDC 完美继承
- 所有并联支路必须显式可见
- 合流必须通过 Merge / Sum 节点或等价合法结构
- 让 Tracktion 原生 PDC 感知完整图结构

### 6.3 Clip 单例复用
- `rack_add_node` 创建的 VSTi 只存在一个 Track 级实例
- Clip 只绑定其输入路径

### 6.4 DAG 阻断
在 `rack_connect_pins` 前：
- 结构性阻断：
  - 禁止 `Z3 -> Z2`
  - 禁止 `Z3 -> Z1`
  - 禁止 `Z2 -> Z1`
- 同层调用 `VitDagChecker::wouldCreateCycle()`
- 最后才执行 `rackType.isConnectionLegal()` / `addConnection()`

### 6.5 播放中无缝插拔
新增：
- `VitGraphSwapCoordinator`

策略：
- 所有图修改都在消息线程构建新快照
- 不以 `suspendProcessing` 作为主切换路径
- 改为 **Double-Buffered Compiled Playback Graph**
- `RackType` / `ValueTree` 仍在编辑线程被修改，但后台编译为不可变播放图快照
- 在 buffer 边界原子发布新的 render graph handle
- 旧播放图由回收线程或 retirement 机制延迟销毁
- 递增 `vit_graph_revision`

---

## 7. AIGC / API / Bridge 架构

### 7.1 浏览器自登录连接器
新增：
- `VitConnectorPluginSpec`
- `VitWebConnectorHost`
- `VitAIGCJobRuntime`

原则：
- DAW 不托管用户账号、密码、Token
- 用户在浏览器中自行登录
- DAW 只负责任务入口、结果下载接收、工程回灌

### 7.2 Bridge Node / Audio Injector
新增：
- `VitBridgeNode`
- `VitAudioInjectorNode`
- `VitWarpBridgeNode`
- `VitGeneratedAssetRegistry`

语义：
- 异步节点完成后产出 `.wav` / buffer / stem / midi
- Bridge 把结果导入工程
- 在 `Z3` 创建实时注入点
- 结果重新进入主链或副链

### 7.3 Tempo & Grid Sync
`VitWarpBridgeNode` 必须负责：
- 读取原始 `BPM` / `Key`
- 对齐当前工程 BPM / 网格
- 可选调性映射

记录：
- `vit_origin_bpm`
- `vit_origin_key`
- `vit_warp_target_bpm`
- `vit_warp_mode`
- `vit_warp_state`

`vit_warp_state`：
- `raw`
- `aligned`
- `failed`
- `bypassed`

### 7.4 资产丢失与恢复策略（Asset Restoration Policy）
加载工程时，`VitMediaPoolManager` 必须对每个 `vit_asset_ref` 做恢复检查。缺失资产时，节点不能崩溃，必须进入可恢复状态。

恢复优先级：
1. 工程同包媒体存在：直接恢复
2. 工程外相对路径 / 缓存路径存在：恢复并重新登记 hash
3. 资产缺失，但节点具备完整重生成参数：进入 `ghost_state`，后台排队 **silent regenerate**
4. 资产缺失且不可重生成：节点标红，等待用户手动重定位或替换

建议状态：
- `asset_state = present|missing|regenerating|unrecoverable`
- `job_state = idle|pending|ready|failed|recovering`

---

## 8. 媒体池与资产生命周期

### 8.1 媒体池爆炸陷阱
新增：
- `VitMediaPoolManager`

所有 AIGC 结果统一落地：
- `ProjectName_Media/AIGC_Cache/`
- `ProjectName_Media/Generated/`

`.vit` / `Edit::state` 不保存音频本体，只保存：
- 相对路径
- 哈希
- 来源元数据
- 引用关系

### 8.2 GC 与缓存
必须支持：
- 未使用资产垃圾回收
- request hash 去重
- 工程打开时资产可用性检查
- 保留 / 清理缓存策略

### 8.2.1 工程打包与归档
必须提供两种工程携带策略：
- `Collect Project`
  - 收集当前引用媒体
  - 适合协作发送与常规迁移
- `Archive Project`
  - 除当前引用媒体外，还额外包含：
    - 当前 active takes
    - 关键 AIGC 生成缓存
    - 最小可重生成摘要
  - 适合长期归档与离线保存

硬规则：
- GC 不得删除：
  - 当前 active take
  - 被工程引用的资产
  - 被用户 `pinned` 的资产
  - 已进入打包/归档集合的资产

### 8.3 生成资产状态
建议状态：
- `transient`
- `kept`
- `pinned`
- `gc_candidate`

---

## 9. 节点级历史版本与异步幽灵状态

### 9.1 Take / History Stack
新增：
- `VitAssetBackedNode`
- `VitTakeHistoryStack`

每个生成节点自带多版本栈：
- `take_1`
- `take_2`
- `take_3`

状态：
- `vit_take_stack_id`
- `vit_active_take_id`

用途：
- A/B 对比
- 快速切回上一版
- 不破坏图结构

### 9.2 Ghost State
新增：
- `VitAsyncGhostPolicy`

异步节点 `pending` 期间必须指定：
- `bypass`
- `play_cache`
- `mute`

状态输出：
- `job_state = idle|pending|ready|failed`
- `ghost_state = bypass|play_cache|mute`

---

## 10. 插件抓手与统一参数面板

### 10.1 双界面模型
每个插件节点保留两个入口：
- `Open UI`
- `Get Param`

原则：
- `Open UI` = 原厂插件界面
- `Get Param` = Vit 统一参数抓手界面

### 10.2 Param Grabber Panel
新增：
- `VitPluginGrabber`
- `VitPluginTemplateRegistry`
- `VitControlShell`

自动完成：
- 参数遍历
- 参数分组
- 标准化角色识别
- 模板套壳

模板：
- `eq`
- `comp`
- `bus`
- `instrument`
- `other`

### 10.3 参数标准化视图
抓手应生成：
- `raw_param_id`
- `raw_param_name`
- `normalized_role`
- `display_group`

例如多家压缩器的阈值参数都能归一到：
- `comp_threshold`

### 10.4 轻量化交互
默认层：
- 自动抓取后的参数列表
- 推荐分组
- 搜索
- 少量高频控制

高级层：
- `Template`
- `Link`
- `History`
- `Macro`
- `Freeze`
- `Bypass`

结论：
- `Get Param` 是默认统一入口
- `Open UI` 是兼容入口

---

## 11. 控制图与 Max/MSP 借鉴

### 11.1 DSP 图之外的控制图
系统不应只有 Audio/MIDI 图，还应有：
- Param / Control Graph

### 11.2 可内建控件节点
建议内建：
- `Slider`
- `Knob`
- `XYPad`
- `Toggle`
- `Button`
- `NumberBox`
- `Range`
- `Envelope`
- `LFO`
- `StepSequencer`
- `MacroPanel`

### 11.3 Param Link Graph
建议保留这一层抽象：
- `VitParamLinkGraph`
- `VitParamBinding`
- `VitMacroNode`
- `VitParamSurface`

它用于：
- 参数到参数
- 参数到节点控制口
- 用户宏到多插件参数
- AI 输出到插件参数

---

## 12. IPC / JSON 协议

### 12.1 `rack_add_node`
```json
{
  "cmd": "rack_add_node",
  "track_id": "1007",
  "rack_item_id": "2001",
  "uuid": "auto_or_reserved_edit_item_id",
  "plugin_type": "vst3",
  "plugin_path": "C:/VST3/Serum.vst3",
  "x": 0.42,
  "y": 0.31,
  "zone_id": "Z2",
  "clip_scope": "track",
  "routing_mode": "zone_auto"
}
```

### 12.2 `rack_connect_pins`
```json
{
  "cmd": "rack_connect_pins",
  "track_id": "1007",
  "rack_item_id": "2001",
  "source_id": "3001",
  "source_pin": 1,
  "dest_id": "3002",
  "dest_pin": 1,
  "connection_mode": "explicit"
}
```

### 12.3 `rack_remove_connection`
映射 `rackType.removeConnection(...)`

### 12.4 Top 限制
若 `zone_id == "Top"`：
- 直接拒绝
- 返回：`Top layer is clip-mapping only and cannot host nodes`

### 12.5 `get_project_state`
返回：
- `tracks[]`
- `rack.nodes[]`
- `rack.edges[]`
- `clip_routes[]`
- `graph_revision`
- `clip_proxy_nodes[]`
- `provenance[]`
- `generated_assets[]`
- `take_histories[]`
- `jobs[]`

### 12.6 Scope Viewport / Scope Filtering
为避免多个 Clip 共用单例节点时的视觉混乱，前端 GraphEdit 必须支持作用域视口。

建议支持三种视图：
- `scope=track`
  - 返回全轨完整拓扑
- `scope=clip:<clip_id>`
  - 只返回或高亮与该 Clip 相关的节点与边
- `scope=debug_global`
  - 返回全量边，并带来源标记，供调试复杂共享场景

边元数据建议扩展：
- `source_clip_id`
- `shared_by_clip_ids[]`
- `line_color_hint`
- `is_scope_hidden`

前端配合：
- 彩色线区分不同 Clip 来源
- 线标明来源 id
- 配合理线功能减少“意大利面条”混乱

---

## 13. 需要动刀的现有代码

### 13.1 `VitHeadlessService.cpp`
路径：`VitApp/Source/Service/VitHeadlessService.cpp`

需要改：
- `applyLoadedEdit()`
- `ensureMonitoringPluginsForEdit()`

新增建议函数：
- `ensureTrackRackGraphForEdit()`
- `migrateLegacyPluginListToRack()`
- `syncTrackRackState()`

### 13.2 `CommandDispatcher.cpp`
路径：`VitApp/Source/Service/CommandDispatcher.cpp`

需要改：
- 删除 `pluginStableID(...fallbackIndex...)`
- 重写 `findPluginByID()` / `findPluginInEdit()` 为纯 `EditItemID`
- 弃用线性 `instantiate_plugin` / `move_plugin`

新增 handlers：
- `handleRackAddNode`
- `handleRackConnectPins`
- `handleRackRemoveConnection`
- `handleRackSetNodePosition`
- `handleRackBindClipRoute`
- `handleRackGetState`

扩展：
- `handleGetProjectState()`

---

## 14. Graph Compiler / Runtime / Registry

建议新增：
- `VitNodeRegistry`
- `VitNodeCapabilityManifest`
- `VitGraphCompiler`
- `VitRealtimeRuntime`
- `VitAsyncRuntime`
- `VitBusProxyNode`
- `VitBridgeNode`
- `VitProbeNode`
- `VitGraphTrace`
- `VitNodeProfiler`
- `VitGraphRevisionLedger`

---

## 15. 迁移顺序

1. **Identity First**  
   移除 index fallback，统一 `EditItemID`

2. **Track Rack First**  
   每轨单 Rack

3. **IPC 2D First**  
   `rack_add_node` / `rack_connect_pins`

4. **Safety Layer**  
   DAG / Buffer Adapter / Merge Planner

5. **Clip Dynamic Routing**  
   `VitClipRouteRegistry`

6. **Hot Swap**  
   `VitGraphSwapCoordinator`

7. **AIGC Bridge Layer**  
   Bridge / Warp / Media Pool / Ghost State

8. **Param Grabber + Control Graph**  
   模板壳、控制图、参数映射

9. **Legacy Migration**  
   旧工程自动 wrap 到 Rack

---

## 16. 最终护栏

### 16.1 可观测性
必须能观测：
- `job_state`
- `ghost_state`
- `graph_revision`
- active take
- 缓存命中
- 输出来源

### 16.2 Graph Revision Ledger
不仅有 revision 编号，还要有最小 diff：
- node add/remove
- edge add/remove
- take switch
- warp state change
- ghost policy change

### 16.3 Capability Manifest
所有节点统一声明：
- `visual_zone`
- `execution_domain`
- `port_types`
- `is_realtime_safe`
- `supports_take_history`
- `supports_ghost_state`
- `supports_param_grabber`
- `supports_warp_injection`

### 16.4 性能预算
建议预定义：
- `realtime_light`
- `realtime_heavy`
- `async_gpu`
- `web_blocking`

并限制：
- 单轨最大重型实时节点数
- 单工程最大异步并发数

### 16.5 失败分级
建议失败状态至少区分：
- `soft_failed`
  - 可旁通或继续播放
- `hard_failed`
  - 必须用户处理
- `recovering`
  - 后台自动恢复中

### 16.6 节点锁定与发布态
建议节点或模板支持：
- `draft`
- `locked`
- `published`

适用于：
- 用户自定义连接器
- 模板壳
- 宏控面板
- 稳定可复用的子图

### 16.7 参数别名系统
在标准化参数名之外，允许用户定义别名：
- 例如 `comp_threshold -> 压缩起点`
- 例如 `eq_freq_low -> 低频中心`

这有利于：
- 中文工作流
- 模板共享
- AI 理解与提示

### 16.8 工程健康检查
建议新增：
- `VitProjectHealthCheck`

在打开工程、保存工程或执行导出前检查：
- 缺失资产
- 失效连接器
- 非法边
- 失效插件
- 悬空 clip route
- 未对齐 warp 素材

### 16.9 导出策略
必须提前定义导出时异步节点仍处于 `pending` 的处理规则：
- `wait`
- `use_cache`
- `bypass`
- `fail_export`

项目级或导出任务级可配置。

---

## 17. 验收重点

- 播放中加节点/改边无爆音
- Audio Clip 不经 Z1/Z2 直达 Z3
- 多个 Clip 指向同一 Serum 时只有一个实例
- 并联合流后 PDC 正确
- 非法逆向边 / DAG 闭环被拒绝
- 大量 AIGC 生成后工程体积不膨胀，媒体池可 GC
- 工程打包后在另一台机器上可恢复 active 资产；缺失资产时能进入恢复/幽灵态而不崩溃
- Bridge 回灌结果自动对齐 BPM / 网格
- 插件抓手能稳定模板化不同插件
- 异步节点 pending 不打断播放
- 多 take 切换不破坏图结构

---

## 18. 结论

最终方案的核心不是“把插件链改成图”，而是建立一套：

- 保留音乐工程语境的层级 UI
- 以 `te::RackType` 为实时核心
- 以 Compiler 区分执行域
- 以 Bridge 接回异步生成结果
- 以插件抓手统一参数理解与控制
- 以媒体池、Ghost State、Take 历史保证现代工作流可用性

的混合系统。

这套架构既能完成 V0.5 的路由引擎重构，也为未来开放接入：
- AIGC 平台
- TTS
- RVC
- 自定义模型
- Web API
- Max/MSP 式控制图

提供稳定基础。

---

## 19. 当前实现基线（Phase 1-12）

截至当前代码基线，任务书中的 `Phase 1` 到 `Phase 12` 已完成一轮最小可用实现，状态如下：

### 19.1 已落地模块
- `Phase 1` 身份统一：
  - `CommandDispatcher` 内插件 / 节点身份统一改为 `EditItemID`
  - `get_project_state` 与插件相关响应已去除 index 型 fallback 语义
- `Phase 2` 单轨单 Rack：
  - 每轨唯一 `RackInstance` 骨架已建立
  - 旧工程加载 / 空白工程初始化可自动补齐 Track Rack 结构
- `Phase 3` 2D IPC：
  - 已实现 `rack_add_node`
  - 已实现 `rack_connect_pins`
  - 已实现 `rack_remove_connection`
  - `get_project_state` 已输出 `rack.nodes[] / rack.edges[]`
- `Phase 4` 防御层：
  - 已实现 `VitGraphValidator`
  - 已实现 `VitDagChecker`
  - 已实现 `VitZoneBufferAdapter`
  - 已实现 `VitParallelMergePlanner`
- `Phase 5` 热切换与 revision：
  - 已实现 `VitGraphSwapCoordinator`
  - 已实现 `VitGraphRevisionLedger`
  - `get_project_state` 与图修改响应已输出 `graph_revision` / lifecycle / recent changes
- `Phase 6` Clip 动态路由：
  - 已实现 `VitClipRouteRegistry`
  - 已支持 `track` / `clip:<id>` / `debug_global` 级别的图视角
- `Phase 7` AIGC Bridge / 媒体池 / Warp：
  - 已实现 `VitAIGCJobRuntime`
  - 已实现 `VitMediaPoolManager`
  - 已实现 `VitWarpBridgeNode`
  - 已实现 `aigc_register_job` / `bridge_ingest_generated_asset`
- `Phase 8` Take / Ghost：
  - 已实现 `VitTakeHistoryStack`
  - 已实现 `VitAsyncGhostPolicy`
  - 已实现 `switch_asset_take` / `set_async_ghost_state`
- `Phase 9` 插件抓手：
  - 已实现 `VitPluginGrabber`
  - 已实现 `VitPluginTemplateRegistry`
  - 已实现 `VitControlShell`
  - `get_plugin_parameters` 已输出模板壳、别名、快速控件建议
- `Phase 10` 控制图：
  - 已实现 `VitParamLinkGraph`
  - 已实现 `VitParamBinding`
  - 已实现 `VitMacroNode`
  - 已实现 `VitParamSurface`
  - 已支持控制节点 / 绑定的增删改查与最小传播链路
- `Phase 11` 连接器开放：
  - 已实现 `VitNodeCapabilityManifest`
  - 已实现 `VitConnectorPluginSpec`
  - 已实现 `VitNodeRegistry`
  - 已支持 connector profile 的工程外部存储与增删改
- `Phase 12` 可观测性 / 健康检查 / 导出策略：
  - 已实现 `VitGraphTrace`
  - 已实现 `VitProbeNode`
  - 已实现 `VitNodeProfiler`
  - 已实现 `VitProjectHealthCheck`
  - `get_project_state` 已输出 `observability` / `project_health` / `export_policy`

### 19.2 当前实现定位
这一轮实现的定位是：
- 已形成完整主干协议
- 已形成工程状态表达
- 已形成最小可运行控制链路
- 已形成 AIGC / 资产 / take / ghost / connector / health 的统一状态面

但它仍然属于：
- **V0.5 架构主干可用版**
- **不是最终商业化完成版**
- **不是所有运行时语义都已做到最深层**

尤其以下内容当前仍建议视为后续增强项：
- 更完整的 Runtime Compiler / Graph Compiler
- 更深的浏览器自动化桥接
- 更丰富的导出收集 / 归档动作
- 更细的性能分析与 probe 可视化
- 更强的控制图时钟 / LFO / StepSequencer 实时驱动

---

## 20. 当前协议落地面

当前内核已经形成以下新增状态面与命令面。

### 20.1 `get_project_state` 新增输出
当前除轨道 / clip / rack 基础状态外，还会输出：
- `control_graph`
- `node_registry`
- `connector_profiles`
- `connector_specs`
- `generated_assets`
- `jobs`
- `take_histories`
- `observability`
- `project_health`
- `export_policy`
- `graph_revision`
- `graph_lifecycle_state`
- `graph_recent_changes`

### 20.2 已落地的重要 IPC
已落地并建议作为当前前端 / bridge / 测试工具基线的命令：

- Rack / Routing：
  - `rack_add_node`
  - `rack_connect_pins`
  - `rack_remove_connection`
- AIGC / Asset / Ghost / Take：
  - `aigc_register_job`
  - `bridge_ingest_generated_asset`
  - `switch_asset_take`
  - `set_async_ghost_state`
- Param Grabber：
  - `get_plugin_parameters`
  - `set_plugin_param_aliases`
- Control Graph：
  - `control_add_node`
  - `control_add_macro`
  - `control_add_binding`
  - `control_update_node`
  - `control_set_node_value`
  - `control_update_binding`
  - `control_remove_binding`
  - `control_remove_node`
  - `control_set_macro_values`
- Connector：
  - `connector_upsert_profile`
  - `connector_remove_profile`
- Observability / Health：
  - `project_health_check`

### 20.3 当前文档基线原则
- 本文档负责解释 **为什么这样设计**
- `docs/ROUTING_EXECUTION_TASKBOOK.md` 负责解释 **如何分阶段执行**
- `docs/VIT_IPC_CONTRACT.md` 负责解释 **当前实际可发送的命令与字段**

---

## 21. 建议验收流程

建议把验收分为五轮，而不是一次性“全测”。

### 21.1 第一轮：基础播放与 Track Rack 不回退
目标：
- 新建工程
- 正常播放 / 停止
- 新增轨道不破坏单轨单 Rack 结构

检查点：
- `get_project_state` 中每条目标轨有 `rack`
- `graph_revision` 可读
- 普通导入音频后播放正常

### 21.2 第二轮：2D 路由安全性
目标：
- 新增 `Z1 / Z2 / Z3` 节点
- 建边 / 删边
- 验证非法边被阻断

检查点：
- `Top` 层不能加载节点
- `Z3 -> Z2` / `Z3 -> Z1` / `Z2 -> Z1` 被拒绝
- DAG 闭环被拒绝
- `rack.nodes[] / rack.edges[]` 与前端一致

### 21.3 第三轮：AIGC / Asset / Take / Ghost
目标：
- 注册 job
- 回灌生成资产
- 切换 take
- 修改 ghost state

检查点：
- `jobs[]` 正常变化
- `generated_assets[]` 正常变化
- `take_histories[]` 正常变化
- `switch_asset_take` 不破坏图结构，只改变活动资产引用

### 21.4 第四轮：插件抓手与控制图
目标：
- 拉取插件参数
- 写入别名
- 新建 control node / macro / binding
- 让控件驱动插件参数

检查点：
- `get_plugin_parameters` 中存在：
  - `template_role`
  - `param_aliases`
  - `binding_targets`
  - `quick_controls`
- `control_set_node_value` 后目标插件参数值变化
- `control_set_macro_values` 后多条 binding 同时生效

### 21.5 第五轮：连接器、观测与健康检查
目标：
- 新建 connector profile
- 删除 connector profile
- 调用健康检查

检查点：
- `node_registry` 中能看到 built-in manifests / connector specs / connector profiles
- `project_health_check` 返回：
  - `observability`
  - `project_health`
  - `export_policy`
- 缺失资产、pending job、未知 connector spec 能进入问题清单

---

## 22. 建议验收标准

若要认定当前 V0.5 主线“通过验收”，建议至少满足：

1. 内核可稳定编译，且工程可正常新建 / 打开 / 保存。
2. `get_project_state` 能稳定返回 Rack、Control Graph、Connector、Observability、Health 五个状态面。
3. Route / Asset / Take / Ghost / Control / Connector 六组核心 IPC 都能独立成功执行。
4. 播放期间执行图修改，不出现明显崩溃或主流程失效。
5. 缺失资产或 pending job 时，系统能给出明确状态而不是静默失败。

---

## 23. 当前测试建议

最实际的测试方式不是直接写大量自动化单测，而是：
- 先用 Godot 前端做一轮交互验收
- 再用桥或脚本直接打 IPC 做精确回归
- 最后再挑最脆弱的链路补少量自动化测试

优先级建议：
1. `get_project_state` 快照一致性
2. `rack_*` 命令行为正确性
3. `bridge_ingest_generated_asset` / `switch_asset_take` 状态一致性
4. `control_*` 传播正确性
5. `project_health_check` 问题识别正确性
