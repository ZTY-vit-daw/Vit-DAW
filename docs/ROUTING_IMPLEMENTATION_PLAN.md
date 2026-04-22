# Vit-DAW V0.5 — 2D 路由机架：详细执行方案

**依据**: `docs/ROUTING_ENGINE_DIAGNOSIS.md`  
**架构定案**: **单轨单 Rack** — 在启用 2D/Vit-Graph 模式的音轨上，`te::Track::pluginList` 中 **有且仅有一个** `te::RackInstance` 作为效果容器；机架内拓扑由 `te::RackType` 管理。  
**持久化载体**: 工程保存为 `.tracktionedit`（明文）或经 Vit 加密的 `.vit`（底层仍为 `Edit::state` 的 XML），Rack 与节点坐标随 **ValueTree** 一并序列化。

---

## 1. 单轨单 Rack：`VitHeadlessService` 侧机制

### 1.1 目标不变式（2D 模式音轨）

对标记为 **2D 机架轨**（例如轨 `ValueTree` 上 `vit_routing_mode == "rack2d"`，具体键名以实现为准）的 **`te::AudioTrack`**：

1. **`pluginList` 中用户效果容器段**只允许 **一个** `RackInstance` 插件（`te::RackInstance::xmlTypeName` 创建）。
2. 允许在链上 **额外** 存在 **内置监控类插件**（`VolumeAndPanPlugin`、`LevelMeterPlugin`），与当前 `ensureMonitoringPlugins` 一致；二者不得再插入第二个 Rack 或游离 VST 链（除非产品明确允许「Rack 外前置/后置单块」— 若不允许，则所有 FX 必须在 Rack 内）。

**推荐链序（示例）**：

`[ RackInstance（唯一）] → [ Volume ] → [ LevelMeter ]`

或（若产品要求 Rack 处理已归一化电平）：

`[ Volume ] → [ RackInstance ] → [ LevelMeter ]`

须在实现中 **固定一种** 并写入契约，避免前端与内核歧义。

### 1.2 在 `VitHeadlessService` 中实现的机制

| 机制 | 说明 |
|------|------|
| **`ensureSingleRackFor2DTrack(te::AudioTrack&)`** | 若轨为 2D 模式且尚无 Rack：创建 `edit.getPluginCache().createNewPlugin(te::RackInstance::xmlTypeName, {})`，按约定索引 `insertPlugin`；若已有多个 Rack → **合并/报错**（开发版 assert，发布版打日志并拒绝保存）。 |
| **`applyLoadedEdit` 钩子** | 在 `ensureMonitoringPluginsForEdit` 之后、`primeEditPlaybackGraph` 之前或之后（与团队约定），对 **所有 2D 轨** 调用 `ensureSingleRackFor2DTrack`；对 **旧工程线性链** 调用迁移（见 §5）。 |
| **非法状态检测** | 枚举 `track.pluginList.getPlugins()`：若 2D 轨上出现 **第二个** `RackInstance` 或禁用模式下仍存在的游离 FX，记录 `juce::Logger` 并可选自动收编或返回 IPC 错误。 |

### 1.3 涉及文件

- `VitApp/Source/Service/VitHeadlessService.cpp` / `.h`：挂载上述钩子；可抽 **`VitTrackRackPolicy.cpp`** 便于单测。
- `VitApp/Source/Service/CommandDispatcher.cpp`：新增 `rack_*` 命令前，先实现 **`rack_ensure`**（或合并进 `ensureSingleRack` 的共享实现）。

---

## 2. 稳定插件 ID：拆除 `pluginStableID` 的 index 依赖

### 2.1 原则

- **持久身份** = Tracktion **`te::EditItemID`**（序列化在插件子树中），**不是**应用层随机 UUID 字符串。
- **对外 JSON** 统一使用 **`plugin_item_id`** / **`node_item_id`** 字符串 = `plugin.itemID.toString()`（或 Engine 规定的等价形式）。

### 2.2 重写创建与查找

1. **所有** 插件创建走 **`Edit::getPluginCache().createNewPlugin(...)`**，插入 Rack 走 **`RackType::addPlugin(ptr, pos, autoConnect)`**，保证 `Plugin` 已注册到 `Edit` 并具备有效 `itemID`。
2. **删除** `pluginStableID (Plugin&, fallbackIndex)` 中：
   - 使用 **`fallbackIndex`** 的分支；
   - 以及 **`plugin.getPluginType() + ":" + name + ":" + index`** 等回退。
3. **替换为**（建议内联或 `VitPluginIdentity.h`）：

```cpp
// 伪代码 — 仅表达语义
juce::String pluginItemIdString (const te::Plugin& p)
{
    const auto id = p.itemID;
    jassert (id.isValid()); // 已插入 Edit 的插件必须有效；无效则上层报错，禁止猜测
    return id.toString();
}
```

4. **`findPluginByID(track, idStr)`**：仅 `te::EditItemID::fromString` → 与 `track` 上插件 `itemID` 比较（或 `edit.getPluginCache().getPluginFor`）。  
5. **`findPluginInEdit`**：优先 `getPluginFor(EditItemID)`，不再按旧字符串模糊匹配（过渡期可保留日志告警）。

### 2.3 涉及文件

- `VitApp/Source/Service/CommandDispatcher.cpp`（主改动）
- `docs/VIT_IPC_CONTRACT.md`（ID 规范）

---

## 3. 新图指令集：JSON 与 `te::RackType` 映射

以下命令均在 **JUCE 消息线程** 执行（与现有 ZMQ → `CommandDispatcher` 一致）。

### 3.1 公共字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `track_id` | string | 目标音轨 `EditItemID` |
| `rack_item_id` | string | 该轨上 **唯一** `RackInstance` 插件的 `itemID` |

**解析 Rack**：`auto* rackPl = dynamic_cast<te::RackInstance*>(getPluginFor(...))`；机架图操作使用 **`rackPl->type`**（`te::RackType::Ptr`，见 `tracktion_RackInstance.h`）。

---

### 3.2 `rack_add_node`

**语义**：在机架内新增一个插件 **节点**（对外名「node」对应 Engine 内的一个 `Plugin` 实例），并设置 **归一化坐标**（与 Engine 存储一致，见 §4）。

**REQ JSON**：

```json
{
  "cmd": "rack_add_node",
  "track_id": "1007",
  "rack_item_id": "1024",
  "plugin_path": "C:/Program Files/Common Files/VST3/Foo.vst3",
  "x": 0.35,
  "y": 0.42,
  "auto_connect": false
}
```

| 字段 | 说明 |
|------|------|
| `plugin_path` | 与现有 `instantiate_plugin` 相同：VST3 物理路径，用于匹配 `PluginDescription` |
| `x`, `y` | **0.0～1.0** 归一化坐标（与 `RackType` 内部 `jlimit` 一致）；若前端使用像素，需在 **Godot 或 bridge 层** 按视口换算后再发内核 |
| `auto_connect` | 映射 `RackType::addPlugin(..., canAutoConnect)` |

**C++ 逻辑（伪代码）**：

```cpp
auto* edit = getEdit();
auto* track = findAudioTrackByID (*edit, trackId);
auto* rackInstance = findRackInstance (*track, rackItemId);
auto& rackType = *rackInstance->type; // RackInstance::type 为 RackType::Ptr

auto desc = resolveVst3Description (*edit, pluginPath); // 复用 instantiate_plugin 扫描逻辑
auto plugin = edit->getPluginCache().createNewPlugin (te::ExternalPlugin::xmlTypeName, desc);
if (! plugin) return error;

edit->getUndoManager().beginNewTransaction ("rack_add_node");
const juce::Point<float> pos { x, y };
rackType.addPlugin (plugin, pos, autoConnect);
rackType.flushStateToValueTree();
track->flushStateToValueTree();
edit->dispatchPendingUpdatesSynchronously();
edit->getTransport().ensureContextAllocated (true);
```

**REP**：`status`, `node_item_id`（新插件 `itemID`）, `track_id`, `rack_item_id`。

---

### 3.3 `rack_connect_pins`

**语义**：在 **同一 Rack 内** 建立音频/MIDI 引脚连接。

**REQ JSON**：

```json
{
  "cmd": "rack_connect_pins",
  "track_id": "1007",
  "rack_item_id": "1024",
  "source_id": "1030",
  "source_pin": 1,
  "dest_id": "1031",
  "dest_pin": 1
}
```

**说明**：`source_id` / `dest_id` 为 **`EditItemID` 字符串**；Rack 边界引脚在 Tracktion 中可用 **空 ID** 表示 rack 输入/输出（参见 `addPlugin` 自动连线里 `addConnection ({}, i+1, p->itemID, ...)`）。若 JSON 需表达「连到机架输入」，约定：

```json
"source_id": "",
"source_pin": 0
```

（实现时用 `EditItemID{}` 传给 `addConnection`。）

**C++ 映射**：

```cpp
const te::EditItemID src = parseEditItemId (sourceIdStr);
const te::EditItemID dst = parseEditItemId (destIdStr);

if (! rackType.isConnectionLegal (src, sourcePin, dst, destPin))
    return error ("illegal connection");

edit->getUndoManager().beginNewTransaction ("rack_connect_pins");
const bool ok = rackType.addConnection (src, sourcePin, dst, destPin);
rackType.checkConnections(); // 可选，按 Engine 建议
```

**REP**：`status`, `track_id`, `rack_item_id`。

---

### 3.4 `rack_remove_connection`

**语义**：移除一条与 `rack_connect_pins` 对称的连接。

**REQ JSON**：

```json
{
  "cmd": "rack_remove_connection",
  "track_id": "1007",
  "rack_item_id": "1024",
  "source_id": "1030",
  "source_pin": 1,
  "dest_id": "1031",
  "dest_pin": 1
}
```

**C++ 映射**：

```cpp
rackType.removeConnection (src, sourcePin, dst, destPin);
```

（参数顺序与 `tracktion_RackType.h` 中 `removeConnection` 一致。）

---

### 3.5 注册与文档

- 在 `CommandDispatcher::registerBuiltinCommands` 中注册 `rack_add_node`、`rack_connect_pins`、`rack_remove_connection`（及建议配套的 `rack_ensure`、`rack_set_node_position`）。
- `docs/VIT_IPC_CONTRACT.md`：完整 REQ/REP 与空字符串 `source_id` 约定。

---

## 4. 坐标持久化：ValueTree 与 `.tracktionedit` / `.vit`

### 4.1 Engine 事实（摘自 `tracktion_RackType.cpp`）

- 每个机架内插件对应一棵 **`IDs::PLUGININSTANCE`** 子节点。
- **位置**存储在该 `PLUGININSTANCE` 节点的属性上：
  - **`IDs::x`**、**`IDs::y`**
  - 写入时 **`jlimit (0.0f, 1.0f, ...)`**，即 **归一化浮点坐标**。
- 子节点内嵌 **`IDs::PLUGIN`** 子树，即具体 `Plugin` 的状态（含 `EditItemID`）。

创建片段（Engine 行为）：

```cpp
auto v = createValueTree (IDs::PLUGININSTANCE,
                          IDs::x, juce::jlimit (0.0f, 1.0f, pos.x),
                          IDs::y, juce::jlimit (0.0f, 1.0f, pos.y));
v.addChild (p->state, -1, getUndoManager());
state.addChild (v, -1, getUndoManager());
```

### 4.2 与 Vit 工程文件的关系

- **`.tracktionedit`**：`Edit::state` 导出为 XML，上述 `PLUGININSTANCE` 的 `x`/`y` 属性 **原样出现在 XML** 中。
- **`.vit`**：Vit 加密封装的是 **同一份工程 XML**（见现有加载路径），故坐标 **无需额外格式**；解密后解析逻辑与明文一致。

### 4.3 Vit-Graph 与归一化坐标

- Godot 蓝图若使用 **像素或任意范围**，必须在 **前端或 `bridge_core.py`** 与 **0..1** 之间做双向映射，并在文档中写明参考分辨率或「仅存储归一化」。
- 可选扩展命令 **`rack_set_node_position`**：`node_item_id` + `x` + `y`，内部调用 `RackType::setPluginPosition (index, pos)` 或直接设置对应 `PLUGININSTANCE` 的 `x`/`y` 属性（与 Engine 保持一致）。

---

## 5. `get_project_state`：机架节点坐标与连线

### 5.1 轨级扩展结构

在现有 `tracks[]` 每一项中，当该轨存在 Rack 时增加 **`rack`** 对象（无 Rack 时可为 `null` 或省略）：

```json
{
  "track_id": "1007",
  "track_name": "Audio 1",
  "track_type": "hybrid",
  "plugins": [],
  "clips": [],
  "rack": {
    "rack_item_id": "1024",
    "nodes": [
      {
        "node_item_id": "1030",
        "name": "Foo",
        "type": "vst",
        "enabled": true,
        "x": 0.35,
        "y": 0.42
      }
    ],
    "connections": [
      {
        "source_id": "",
        "source_pin": 1,
        "dest_id": "1030",
        "dest_pin": 1
      },
      {
        "source_id": "1030",
        "source_pin": 1,
        "dest_id": "1031",
        "dest_pin": 1
      }
    ]
  }
}
```

### 5.2 数据采集（C++ 要点）

- **`rack_item_id`**：遍历轨 `pluginList`，找到 `dynamic_cast<te::RackInstance*>`，取 `itemID`。
- **`nodes`**：对 `rackType.getPlugins()` 中每个 `Plugin`：
  - `node_item_id` = `plugin->itemID.toString()`
  - `x`,`y` = `rackType.getPluginPosition (plugin)`（或按 index）
  - `name` / `type` / `enabled` 与现有一致
- **`connections`**：遍历 `rackType.getConnections()`（或等效），填充 `RackConnection` 的 `sourceID`、`destID`、`sourcePin`、`destPin`；**空 ID** 序列化为 `""` 以便与 `rack_connect_pins` 对称。

### 5.3 同步策略

- 图变更后现有流程已 `flushStateToValueTree` + `dispatchPendingUpdatesSynchronously`；前端可在每次 `rack_*` 成功后再拉 **`get_project_state`**，或后续增加 **`VitEditStateDeltaHub` 对 Rack 子树的细粒度事件**（可选，Phase 2）。

---

## 6. 分阶段执行（Phases）

| Phase | 内容 | 核心 C++ 文件 | IPC | 音频线程风险 |
|-------|------|---------------|-----|----------------|
| **0** | 引入 `RackInstance`/`RackType` 头文件依赖；消息线程断言 | `CMakeLists` / PCH | 无 | 无 |
| **1** | 重写 ID 与查找；移除 `pluginStableID` index | `CommandDispatcher.cpp` | 响应只含 `plugin_item_id` | ID 改动仅在消息线程 |
| **2** | `VitHeadlessService`：`ensureSingleRackFor2DTrack` + `applyLoadedEdit` 钩子 | `VitHeadlessService.cpp`, 新建 `VitTrackRackPolicy.*` | `rack_ensure` | 插入后 `ensureContextAllocated`；禁止音频线程改图 |
| **3** | `rack_add_node` / `rack_connect_pins` / `rack_remove_connection` | `CommandDispatcher.cpp` | §3 | `isConnectionLegal` 先于 `addConnection`；非法图不进入播放 |
| **4** | 扩展 `handleGetProjectState` | `CommandDispatcher.cpp` | §5 | 仅遍历，无实时音频写 |
| **5** | 旧工程线性链 → `createTypeToWrapPlugins` 或等价迁移 | `VitHeadlessService.cpp` | 可选 `migration` 字段 | 迁移时暂停 transport |

---

## 7. 风险与预防（摘要）

| 风险 | 预防 |
|------|------|
| ZMQ 线程与消息线程不一致 | 保持现有 **队列到 JUCE 消息线程** 处理所有 `rack_*` |
| 双 Rack 或链上多余 FX | `VitHeadlessService` 与 `rack_add_node` 内双重校验 |
| 坐标系混淆 | 文档固定 **0..1**；UI 自行换算 |
| 引脚索引越界 | 调用前读 `getChannelNames` 或与 `isConnectionLegal` 一致 |

---

**结语**：本方案在 **`VitHeadlessService`** 层 **强制单轨单 `RackInstance`**，用 **`EditItemID`** 替代任何基于 **index** 的 `pluginStableID`；**`rack_add_node` / `rack_connect_pins` / `rack_remove_connection`** 直接映射 **`RackType::addPlugin` / `addConnection` / `removeConnection`**；节点 **x/y** 由 Engine 写入 **`PLUGININSTANCE`** 的 ValueTree 属性，随 **`.tracktionedit` / `.vit`** 持久化；**`get_project_state`** 透出 **`nodes`（含坐标）与 `connections`**，供 Vit-Graph 对齐。
