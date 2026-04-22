# Vit-DAW V0.5 — 2D 路由引擎重构：架构审计与诊断报告

**审计日期**: 2026-04-18  
**范围**: `D:\Vit_DAW` 内 VitApp C++ 服务层与 Tracktion Engine 子模块（仅引用层面）；**未修改任何源代码**。  
**目标**: 评估从 V0.3.5 风格的一维 `PluginList` 链路至 `te::RackType` 拓扑的可行性与技术债。

---

## 1. Current State（现状）

### 1.1 `te::PluginList` 在工程中的位置

VitApp **未**以字面类型 `te::PluginList` 书写引用；所有 Tracktion 侧链式插入均通过 **`te::Track::pluginList`**（类型为 Tracktion 的 `PluginList`）完成。

| 文件 | 角色 |
|------|------|
| `VitApp/Source/Service/CommandDispatcher.cpp` | 列举/序列化插件、`insertPlugin` 插入与重排、扫描与实例化 VST3 |
| `VitApp/Source/Service/VitHeadlessService.cpp` | 加载工程后 `ensureMonitoringPlugins`（音量表/电平表）、输出设备修复、`primeEditPlaybackGraph` |

**关键调用形态**（线性链）：

- `track.pluginList.getPlugins()`：顺序遍历整条链。
- `track.pluginList.insertPlugin (plugin, index, nullptr)`：`instantiate_plugin` 固定插入到 **索引 0**（链首）；监控类插件用 **-1**（追加）；`move_plugin` 用 `new_index` 重插。

### 1.2 `VitHeadlessService` 与轨道插件顺序

- **`ensureMonitoringPlugins` / `ensureMonitoringPluginsForEdit`**（`VitHeadlessService.cpp` 与 `CommandDispatcher.cpp` 中各有一份同逻辑实现）：若音轨缺少内置 `VolumeAndPanPlugin` / `LevelMeterPlugin`，则 `createNewPlugin` 后 `insertPlugin(..., -1, ...)`，保证链尾存在电平与音量节点。
- **`applyLoadedEdit`**（`VitHeadlessService.cpp`）：新 `Edit` 载入后依次执行 `ensureMonitoringPluginsForEdit`、`primeEditPlaybackGraph`、将 `deltaHub` 挂到 `edit->state`。**此处未**创建 Rack 容器或改写插件拓扑，仅保证默认内置插件与播放图有效。

### 1.3 状态对外暴露（仍为「链 + 元数据」）

- **`createTrackState` / `createPluginArray`**：`list_tracks` 中每条轨的 `plugins` 为简单对象数组（`name` / `type` / `enabled`），顺序即处理顺序。
- **`createProjectStatePluginsArray`**：`get_project_state` 中每条插件包含 `id`（见下）、`item_id`（`EditItemID` 字符串）、`name`、`type`、`enabled`。**不包含**引脚、连线、侧链或 Rack 内二维坐标。

### 1.4 插件身份解析：`pluginStableID` 与 `findPluginInEdit`

实现见 `CommandDispatcher.cpp`：

- 优先使用插件 `ValueTree` 上的 `id` / `uid` / `identifier`。
- 若均缺失，则回退为 **`plugin.getPluginType() + ":" + name + ":" + index`**（或不含 index 的变体）。

因此前端所见的 `plugin_id` 在极端情况下仍与 **链上序号** 耦合，与 Vit-Graph 的稳定节点 ID 模型不完全同构。

---

## 2. IPC 契约现状（ZMQ / JSON）

### 2.1 命令注册名（与「add_plugin」命名差异）

当前内核注册的插件相关命令为（`CommandDispatcher::registerBuiltinCommands`）：

| 命令 | 作用 |
|------|------|
| `scan_plugins` | 扫描 VST3 路径，填充 `knownPluginList` |
| `instantiate_plugin` | 按 `plugin_path` 在指定轨 **链首（index 0）** 插入外部插件 |
| `open_plugin_ui` / `show_plugin_editor` | 打开编辑器 |
| `get_plugin_parameters` | 拉取外部插件参数表 |
| `delete_plugin` | 按 `plugin_item_id` 删除 |
| `move_plugin` | 按 `plugin_item_id` + **`new_index`** 重排 |

**不存在**名为 `add_plugin` 的 handler；语义上接近的是 **`instantiate_plugin`**。文档见 `docs/VIT_IPC_CONTRACT.md`（插件章节）。

### 2.2 字段与「索引 vs 图」

- **轨道定位**：`track_id`（主）；`instantiate_plugin` / `open_plugin_ui` 仍支持遗留 **`track_index`**。
- **插件删除/移动**：使用 **`plugin_item_id`**（解析为 `te::EditItemID` 或通过 `pluginStableID` 在轨上匹配），**不是**纯数组下标。
- **重排**：**`move_plugin` 显式要求整数 `new_index`** — 这是典型的 **一维顺序模型**，无 `source_pin` / `dest_pin` / `wire` 等图结构字段。
- **实例化位置**：`insertPlugin(..., 0, ...)` 写死链首，无「插入到 Rack 槽位」或「连接到节点 A 的输出」之类参数。

**结论**：当前 JSON 契约可支撑 **线性顺序与 EditItem 级身份**，**不支撑** Vit-Graph 蓝图所需的 **引脚连接语义**；要对接 2D 机架需扩展或新增命令（例如 `rack_*` / `connect_pins`）并约定与 `te::RackType::addConnection` 的映射。

---

## 3. Rack 基础探测（`te::RackType` / `te::RackInstance`）

### 3.1 VitApp 源码

在 **`VitApp/Source`** 下 **未发现** 对 `te::RackType`、`te::RackInstance`、`RackInstanceNode` 的 `#include` 或符号引用。Rack 能力仅存在于 **Tracktion Engine 子模块**（如 `tracktion_engine/.../tracktion_RackType.h` 等），随 VitKernel 构建链编译，但 **应用层尚未接入**。

### 3.2 Engine 侧能力摘要（供集成对齐）

`tracktion_RackType.h` 中 `RackType` 已提供：

- `addPlugin (Plugin::Ptr, Point<float> pos, bool canAutoConnect)` — 二维布局入口；
- `addConnection (EditItemID source, int sourcePin, EditItemID dest, int destPin)` / `removeConnection`；
- `createInstanceForSideChain` — 侧链相关；
- `createTypeToWrapPlugins` — 由现有插件数组包装为 Rack。

**含义**：目标 2D 拓扑在 Engine 层已有 **一阶 API**；Vit-DAW 缺口在 **应用层编排、IPC 与状态同步**，而非底层完全空白。

### 3.3 在 `Edit` 加载路径中插入 Rack 的难度评估

- **技术难度：中**。`applyLoadedEdit` 已集中处理「载入后修补」（监控插件、输出设备、播放图）。在此 **之后** 增加一步「若轨为 Vit-Graph 模式则确保存在一个 Rack 宿主并迁移插件」是自然的挂载点；难点在于：
  - 与现有 **线性 `pluginList`** 的互斥或嵌套策略（Rack 作为单个 `Plugin` 占位还是替代整条链）；
  - Undo/序列化与 `flushStateToValueTree` 的一致性；
  - 前端与 `plugin_item_id` / Rack 内子插件 ID 的双向映射。

不在加载路径插入、而仅在 **新轨/用户显式「启用机架」时** 创建 Rack，风险较低、增量更小。

---

## 4. 数据同步一致性

### 4.1 工程态同步

- **`get_project_state` / `list_tracks`**：插件列表以 **链顺序** 输出；身份字段为 **`pluginStableID` + `item_id`**。
- **`VitEditStateDeltaHub`**（`VitDeltaProbe.cpp`）：监听 **`edit->state` 根 `ValueTree`**，推送 `property_changed:*`、`node_added`、`node_removed`，`target_uid` 来自节点 `id` / `projectID` 或 **类型名字符串**。这是 **通用增量**，**未**针对 Rack 图或引脚做结构化 diff。

### 4.2 参数同步

- **`set_plugin_param`**：使用全局 **`plugin_id`** + **`param_id`**（Tracktion 自动化 ID）+ `value`；通过 `findPluginInEdit` 定位插件，**不依赖**链索引。
- **`get_plugin_parameters`**（外部插件）：参数行使用 **`param_1`、`param_2`…** 与 JUCE `AudioProcessor` 参数列表下标绑定 — **索引化**，与 `param_id` 字符串模型并存；AI/前端需约定优先级。

### 4.3 迁往 UUID / 节点 ID 的改动面

| 层级 | 现状 | 若要稳定对齐 Vit-Graph |
|------|------|-------------------------|
| Tracktion | `EditItemID` 已存在（`item_id` 已透出） | Rack 内子插件仍有 ID；需统一 **「图节点 ID」** 是否等于 `EditItemID` 或外加一层映射表 |
| IPC | `plugin_id` 字符串 + `plugin_item_id` | 新增/明确 **rack_id、node_id、edge 列表**；`move_plugin` 可能裂变为 **图内重排/改连线** |
| 前端 | Vit-Graph 蓝图（Godot） | 需与内核 **同一套边与引脚语义**，否则 `pluginStableID` 的回退分支易与图编辑冲突 |

---

## 5. Technical Debt（技术债：需拆除或替换的线性逻辑）

1. **`instantiate_plugin` 固定插入 index 0**  
   与「链尾插入」或「Rack 槽内插入」都不一致，且与 `move_plugin` 组合时依赖客户端多次纠正顺序。

2. **`move_plugin` 的 `new_index` 模型**  
   无法表达分叉、合并、并行支路；Vit-Graph 的连线需 **替代或并存** 该 API。

3. **重复的 `ensureMonitoringPlugins` 实现**  
   两处拷贝（`VitHeadlessService` / `CommandDispatcher`）在引入 Rack 后若仍向 `track.pluginList` 追加，需统一策略（例如 Rack 外始终保留 master 音量/电平，或移入 Rack 输出节点）。

4. **`pluginStableID` 的 index 回退**  
   在插件缺少持久 `id`/`uid` 时与 **顺序强绑定**，与 2D 图编辑（删除中间节点即引发 ID 歧义）冲突风险高。

5. **文档与实现命名**  
   契约文档中部分示例仍偏「插件名数组」表述；`delete_plugin`/`move_plugin` 未在 `VIT_IPC_CONTRACT.md` 中与 `instantiate_plugin` 同级展开，易造成集成偏差。

---

## 6. Integration Points（建议的 Rack 映射切入点）

1. **首选：单轨单 Rack 宿主**  
   在每条需要 Vit-Graph 的音轨上，以一个 **`RackInstance` 插件**（Tracktion 惯例）承载 `RackType`，内核只暴露「打开图编辑」所需的 **Rack 状态片段**（或继续走 `get_project_state` 扩展字段）。这样对现有 **轨级 `pluginList`** 侵入最小：链上可见「Rack + 可选轨尾效果」。

2. **次选：`applyLoadedEdit` 后迁移钩子**  
   对带 `vit_rack`（示例名）标记的轨，将线性插件批量 `RackType::createTypeToWrapPlugins` 或等价迁移，再写回 `Edit`。

3. **IPC 扩展层**  
   新建 **`rack_graph`** / **`set_rack_connection`** 类命令，内部调用 `RackType::addPlugin` / `addConnection`，避免继续堆叠在 `instantiate_plugin` 上。

4. **同步层**  
   若 Rack 子树在 `ValueTree` 中有稳定结构，可让 `VitEditStateDeltaHub` 继续工作，但前端需能解析 **Rack 子树**；否则需 **专项快照 API**（例如 `get_rack_state`）。

---

## 7. Potential Risks（潜在风险）

1. **嵌套与格式限制**  
   `RackType::isPluginAllowed` 明确排除「把 Rack 放进 Rack」等情形；Vit-Graph 若允许任意嵌套，需分层或预处理。

2. **多通道与 I/O 命名**  
   Rack 的 `addInput`/`addOutput`、引脚索引与 VST3 总线布局不一致时，**通道映射**错误会导致静音或串音；需在 UI 与 `addConnection` 间建立 **显式 pin 契约**（含立体声 pair）。

3. **侧链**  
   `createInstanceForSideChain` 与轨道侧链路由相关；与 Godot 侧「侧链边」对齐时，要核对 Tracktion 的 **轨道发送/接收模型** 是否满足产品预期。

4. **JUCE 宿主能力**  
   外部插件的 **延迟报告、旁通、多总线** 在 Rack 图中可能比线性链更易暴露 **PDC（插件延迟补偿）** 与 **重入顺序** 问题；需在图上线后做回归。

5. **性能与 `ensureContextAllocated`**  
   当前每次插件增删改后均 `dispatchPendingUpdatesSynchronously` + `ensureContextAllocated(true)`；大图频繁改线可能放大主线程停顿，需评估批处理或防抖。

---

## 8. 附录：关键符号索引（便于代码导航）

| 符号 | 文件 |
|------|------|
| `createPluginArray` / `createProjectStatePluginsArray` | `CommandDispatcher.cpp` |
| `pluginStableID` / `findPluginByID` / `findPluginInEdit` | `CommandDispatcher.cpp` |
| `handleInstantiatePlugin` / `handleMovePlugin` / `handleDeletePlugin` / `handleSetPluginParam` | `CommandDispatcher.cpp` |
| `applyLoadedEdit` / `primeEditPlaybackGraph` | `VitHeadlessService.cpp` |
| `RackType` API | `tracktion_engine/.../tracktion_RackType.h` |

---

**结论**：Vit-DAW 内核当前是 **完整的轨级线性 `pluginList` + EditItem 级身份**，IPC **无引脚图**；Tracktion **已具备 `RackType` 图编辑与连接 API**，但 **VitApp 未引用**。V0.5 升级应优先 **固定 Rack 宿主策略与 IPC 图模型**，再替换 `move_plugin`/链序敏感逻辑，并弱化 **`pluginStableID` 的 index 回退** 对前端图节点的耦合。
