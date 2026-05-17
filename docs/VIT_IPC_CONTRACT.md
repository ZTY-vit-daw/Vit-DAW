# Vit IPC 契约（Godot ↔ Bridge ↔ ZMQ REQ）

## 传输

- **遥测**：ZMQ SUB → 桥 → UDP `127.0.0.1:4444` → `VitTelemetryManager`（`telemetry_manager.gd`）。
- **指令**：`VitIpcClient` UDP 任意空闲端口 → `127.0.0.1:4445` → 桥 → ZMQ REQ → 内核；**应答由桥原路 UDP 返回**（发布入口：`scripts/bridge_prod.py`，开发入口：`scripts/godot_bridge.py`）。

## `cmd` 与 `action`（统一路由规则）

内核 `CommandDispatcher` 对**同一条 JSON 命令**按以下规则解析**命令名字符串**（用于查找处理函数）：

1. 读取 `cmd` 字段并 `trim`；若得到**非空**字符串，则使用该字符串作为命令名。
2. 否则读取 `action` 字段并 `trim`；若得到**非空**字符串，则使用该字符串作为命令名。
3. 若两者均为空，返回错误：`Missing cmd/action field`。

因此 **`cmd` 与 `action` 二选一即可**；若**同时提供且均非空**，**以 `cmd` 为准**（`action` 被忽略）。文档示例可按场景任选其一，但自动化/LLM 侧建议固定一种风格以减少歧义。


## 内核命令实现归属

`CommandDispatcher` 现在是 IPC 入口和命令路由层：它负责解析 `cmd` / `action` / `command`、渲染中 allowlist、注册命令名并转发到内部 service。业务命令实现按以下服务归属维护：

| Service | 命令范围 |
|---|---|
| `ProjectService` | 工程生命周期、最近工程列表、保存/另存/打开 |
| `ImportService` | 音频导入、指定落点插入、波形/频谱 bake 预热 |
| `GeneratedAssetService` / `JobEventService` | AIGC job、生成资产 ingest、take 切换、异步 ghost 状态 |
| `TrackService` / `ClipService` / `MidiService` | 轨道、clip、MIDI clip 与 MIDI note 编辑 |
| `TransportAudioService` | 走带、click、音频设备、输入路由、录音、冻结、脱机渲染 |
| `PluginRackControlService` | 插件扫描/实例化/参数、rack DAG、control graph、connector profile |

`CommandDispatcher` 仍保留 `ping`、`get_project_state`、`set_tempo`、`project_health_check`、`clear_project`、`undo`、`redo`。其中 `get_project_state` 是跨服务聚合快照，继续在入口层组装 `tracks`、`rack`、`control_graph`、connector、job、observability 与 graph revision 状态面。

更详细的结构边界与提交点见 [`VIT_KERNEL_SERVICE_BOUNDARY.md`](VIT_KERNEL_SERVICE_BOUNDARY.md)。

## 支持的指令列表（节选）

### 走带与控制

| 命令名（`cmd` 或 `action`） | 说明 |
|-----------------------------|------|
| `play` | 开始播放（当前工程走带）。 |
| `stop` | 停止播放；若已通过 `transport_option_stop_return_to_start` 将选项设为 `true`，则在停止后将工程时间线置回 **0 秒**（见下节实现说明）。 |
| `return_to_zero` | 停止并将播放头回到工程起点（0 秒）。 |
| `transport_option_stop_return_to_start` | 仅更新内核中的「Stop 后是否回到起点」选项，不触发停止。 |

#### `transport_option_stop_return_to_start`

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"transport_option_stop_return_to_start"`（二选一字段规则见上）。 |
| `value` | boolean | `true`：之后收到的 `stop` 在停带后会将时间线设为 0 秒；`false`：之后 `stop` 仅停带，不自动回零。 |

示例：

```json
{
  "action": "transport_option_stop_return_to_start",
  "value": true
}
```

成功应答（仅此字段，无 `message`）：

```json
{
  "status": "ok"
}
```

#### 内核实现说明（`stop` 与「回到起点」）

Tracktion `TransportControl::stop` 的签名为 `stop(discardRecordings, clearDevices, …)`，其中**第二个参数表示是否清除设备图**，**不是**时间线回绕。因此内核在 `stop` 时固定调用 `stop(false, false)`；当选项为真时，在停止后额外调用 `setPosition(0)` 实现「Stop 回到起点」。

### 查询与工程

| 命令名 | 说明 |
|--------|------|
| `ping` | 存活探测。 |
| `get_project_state` | 返回 `project_path` 与全息 `tracks[]`（`id` / `name` / `type` / `plugins`）；用于主界面与内核拓扑对齐。 |
| `list_tracks` | 返回轨道列表；供 Godot `VitTrackIdRegistry` 将场景 `Track_Root_*` 与内核 `id` / 轨名对齐。 |
| `delete_track` | 按 **`track_id`** 删除一条用户音频轨（不可删最后一条音频轨）；见下文专节。 |
| `move_clip` | 移动 Clip（支持同轨与跨轨），并在目标轨按 **Cut-only** 规则处理重叠。 |
| `resize_clip` | 裁切/伸缩 Clip（支持左边缘 trim + source offset），并在同轨按 **Cut-only** 规则处理重叠。 |
| `instantiate_plugin` | 在指定轨道上按 `plugin_path` 实例化第三方插件，返回 `plugin_id`。 |
| `open_plugin_ui` | 按 `track_id + plugin_id` 打开插件原生浮窗（非模态）。 |
| `get_plugin_parameters` | 返回插件参数快照（AI 抓手使用，值为标准化 `0..1`）。 |

### 撤销 / 重做（Edit 的 UndoManager）

| 命令名 | 说明 |
|--------|------|
| `undo` | 对当前 `Edit` 调用 `getUndoManager().undo()`；若无可用撤销步则 `status: error`。 |
| `redo` | 对当前 `Edit` 调用 `getUndoManager().redo()`；若无可用重做步则 `status: error`。 |

**REQ** 示例：`{"cmd": "undo"}`、`{"cmd": "redo"}`。

**REP** 成功：`{"status": "ok", "message": "Undo executed"}` 或 `"Redo executed"`。

### 工程状态机与最近工程（Tracktion Edit / `.tracktionedit`）

内核在 **`VitHeadlessService`** 中维护：

- **`currentProjectPath`**：当前已绑定到磁盘的工程文件绝对路径；空白新建工程或未从文件打开时为空字符串（`juce::File` 无路径）。
- **全局最近列表**：持久化在用户应用数据目录下的 JSON（Windows 典型路径：`%AppData%/Vit-DAW/global_project_config.json`），字段 `recent_projects` 为字符串数组，**最多 10 条**，按最近使用排序。

以下命令均在 **JUCE 消息线程** 上执行（与现有 ZMQ → `callAsync` 管线一致），可安全替换 `Edit` 与做文件 I/O。

| 命令名 | 说明 |
|--------|------|
| `get_recent_projects` | 读取全局配置中的历史路径列表。 |
| `new_project` | 销毁当前 `Edit`，从内置最小 XML 模板创建**空白工程**（含 Master 等系统轨，无用户音频轨）；清空 `currentProjectPath` 与 Undo 历史。 |
| `open_project` | 从磁盘打开已有工程；成功则更新 `currentProjectPath` 并将路径**置顶写入** `recent_projects`（去重、截断至 10 条）并保存 JSON。 |
| `load_project` | 与 **`open_project` 完全等价**（兼容旧前端与脚本）。 |
| `save_project` | **明文模式**（无 `encryption` 或 `encryption.mode` 非 `app_bound_aes`）：若 `currentProjectPath` 为空则返回 **`require_path`**；否则覆盖保存到当前路径。**应用绑定加密模式**（`encryption.mode` 为 `app_bound_aes`）：**必须**提供 `file_path`，将当前 `Edit` 序列化为 XML 后经 `VitEncryptionCore` 加密，**强制**写入 `.vit` 文件（不会写出明文 XML 侧车，即使设置了 `VIT_DUMP_CLEAR_XML`）。 |
| `save_as_project` | 将当前 `Edit` 保存到指定 `file_path`。若 `encryption.mode` 为 `app_bound_aes`，行为与加密 `save_project` 相同（仅使用 `file_path`，强制 `.vit`、无侧车）；否则沿用 Tracktion 明文保存。成功则更新 `currentProjectPath` 并**置顶**写入 `recent_projects`。 |
| `reload_project` | 仍表示从 Vit 工作区 **默认 XML**（`default_project.xml`）重新加载会话模板；成功后 **`currentProjectPath` 被清空**（与「磁盘工程会话」脱钩）。 |

#### `get_recent_projects`

* **REQ**：`{"cmd": "get_recent_projects"}`
* **REP**：

```json
{
  "status": "ok",
  "recent_projects": ["D:/Song1.tracktionedit", "E:/Demo.tracktionedit"]
}
```

#### `new_project`

* **REQ**：`{"cmd": "new_project"}`
* **REP** 成功：`{"status": "ok", "message": "Blank project created"}`

#### `open_project` / `load_project`

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` / `action` | string | `open_project` 或 `load_project` |
| `file_path` | string | **必填**。已存在的工程文件绝对路径。 |
| `encryption` | object | 可选。`{"mode": "app_bound_aes"}` 时按 **Vit V1 信封格式** 读取二进制 `.vit` 并解密后再加载；缺省或其它 `mode` 时按既有逻辑（Tracktion / 明文 XML 等）加载。 |

* **REP** 成功：`{"status": "ok", "message": "Project opened"}`（或等价成功文案；`status` 恒为 `ok`）。

#### `save_project`

* **REQ（明文，兼容旧前端）**：`{"cmd": "save_project"}` — 行为见上表（`require_path` / 覆盖当前路径）。
* **REQ（应用绑定加密）**：`{"cmd": "save_project", "file_path": "D:/project.vit", "encryption": {"mode": "app_bound_aes"}}`
  * **必须**提供非空 `file_path`；内核会**规范为 `.vit` 后缀**后写入加密二进制，**不会**生成明文 `.xml` 侧车。
* **REP**：
  * 已绑定磁盘路径且保存成功：`{"status": "ok", "message": "Project saved"}`
  * 明文模式下当前为未命名会话：`{"status": "require_path", "message": "Please prompt Save As"}`（**不是** `error`，前端应打开另存为对话框并调用 `save_as_project`）。

#### `save_as_project`

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` / `action` | string | `save_as_project` |
| `file_path` | string | **必填**。目标文件绝对路径；父目录不存在时内核会尝试创建。 |
| `encryption` | object | 可选。`{"mode": "app_bound_aes"}` 时强制 `.vit` 加密写盘，无侧车。 |

* **REP** 成功：`{"status": "ok", "message": "Project saved"}`

### 音频导入（两种并存）

#### `import_audio`（**官方推荐的 UI / 拖拽导入标准**）

按**内核轨道唯一 ID**（与 `list_tracks` 返回的 `tracks[].id` 一致，即 Tracktion `itemID.toString()`）定位目标**音频轨**，将文件按既有规则**追加**到该轨已有片段之后（与原先 `track_index` 时代的排队逻辑相同）。`drop_x_pos` 等字段可被前端附带，内核可预留兼容；当前实现不读取落点横坐标。

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"import_audio"`。 |
| `file_path` | string | 主机绝对路径。 |
| `track_id` | string | 目标 **AudioTrack** 的内核 ID（**必须**与 `list_tracks` 中对应轨的 `id` 完全一致；**禁止**用语义序号或 UI 行号代替）。 |

与 `add_audio_clip` **并存**：`import_audio` 适合「从左栏拖到某一轨、按轨内末尾排队追加」；`add_audio_clip` 适合「已知内核 `track_id` + 任意 `start_time`」的精确落点。

示例：

```json
{
  "action": "import_audio",
  "track_id": "1007",
  "file_path": "D:/Samples/loop.wav",
  "drop_x_pos": 120.0
}
```

#### `add_audio_clip`

将 WAV 等音频文件放到指定**内核**音频轨上的时间位置。

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"add_audio_clip"`。 |
| `track_id` | string | 内核 Track 的 `id`（与 `list_tracks` 中 `tracks[].id` 一致；**不是**屏幕坐标）。 |
| `file_path` | string | 主机绝对路径（Windows 可用 `/` 或 `\\`）。 |
| `start_time` | number | 工程内时间轴位置（秒）。 |

示例：

```json
{
  "cmd": "add_audio_clip",
  "track_id": "1007",
  "file_path": "D:/Samples/loop.wav",
  "start_time": 0.0
}
```

成功时应答含 `"status": "ok"`（与其它内核命令一致）。

#### `move_clip`（Phase 4.2）

将指定 `clip_id` 移动到新时间位置；当 `source_track_id != target_track_id` 时执行跨轨迁移。  
Phase 4.2 **强制硬编码**重叠策略为 `cut`：目标范围若与其他 clip 重叠，内核会对被覆盖 clip 做 split/trim/覆盖裁切，避免同轨物理重叠发声。

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"move_clip"`。 |
| `source_track_id` | string | 源轨内核 ID。 |
| `target_track_id` | string | 目标轨内核 ID。 |
| `clip_id` | string | Clip 内核 ID（`get_project_state.tracks[].clips[].id`）。 |
| `time_unit` | string | 可选，`"seconds"`（默认）或 `"beats"`。 |
| `new_start` | number | 新起点，单位由 `time_unit` 决定。 |

可选别名（与 `time_unit` 配套）：  
- 秒基准：`new_start_seconds`  
- 拍基准：`new_start_beat` / `new_start_beats`

示例（秒）：

```json
{
  "cmd": "move_clip",
  "source_track_id": "1007",
  "target_track_id": "1008",
  "clip_id": "5012",
  "time_unit": "seconds",
  "new_start": 12.5
}
```

示例（拍）：

```json
{
  "cmd": "move_clip",
  "source_track_id": "1007",
  "target_track_id": "1007",
  "clip_id": "5012",
  "time_unit": "beats",
  "new_start_beat": 48.0
}
```

#### `resize_clip`（Phase 4.2）

修改 clip 的起点、长度与 source offset（用于左 trim 语义）。  
Phase 4.2 同样强制 `cut` 重叠策略，且**主操作 + 被覆盖 clip 裁切**必须在同一 Undo 事务中完成。

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"resize_clip"`。 |
| `track_id` | string | 目标 Clip 所在轨内核 ID。 |
| `clip_id` | string | Clip 内核 ID。 |
| `time_unit` | string | 可选，`"seconds"`（默认）或 `"beats"`。 |
| `new_start` | number | 新起点，单位由 `time_unit` 决定。 |
| `new_length` | number | 新长度，单位由 `time_unit` 决定。 |
| `offset_in_source` | number | 新的 source offset，单位由 `time_unit` 决定。 |

可选别名（与 `time_unit` 配套）：  
- 秒基准：`new_start_seconds` / `new_length_seconds` / `offset_in_source_seconds`  
- 拍基准：`new_start_beat(s)` / `new_length_beat(s)` / `offset_in_source_beat(s)`

示例：

```json
{
  "cmd": "resize_clip",
  "track_id": "1007",
  "clip_id": "5012",
  "time_unit": "beats",
  "new_start": 48.0,
  "new_length": 16.0,
  "offset_in_source": 8.0
}
```

#### `delete_track`

删除指定 **AudioTrack**（按内核 `itemID`，与 `list_tracks` / `get_project_state` 中 `track_id` 一致）。**禁止**使用 UI 行号或 `track_index`；异步场景下索引会漂移，易导致误删。

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmd` 或 `action` | string | 固定 `"delete_track"`。 |
| `track_id` | string | **必填**。目标音频轨内核 ID。 |

* **REQ 示例**：`{"cmd": "delete_track", "track_id": "1007"}`

* **REP 成功**：`{"status": "ok", "message": "Audio track deleted", "track_id": "<已删除轨 id>"}`

* **REP 失败**（示例）：`{"status": "error", "message": "Cannot delete the last audio track"}` 或 `"Audio track not found for track_id: ..."`、`"delete_track requires a non-empty track_id"`。

前端应在收到 **成功** 应答后再拉取 `get_project_state` 做 UI 剪枝；**失败时不要刷新**，保持界面不变。

### `get_project_state`（全息工程拓扑 / 前端对齐）

在进入主界面或打开工程后，前端应调用此指令拉取**与内核一致**的轨道拓扑，缓解 UI 与 `Edit` 的**状态脱节**。

* **REQ**：`{"cmd": "get_project_state"}`
* **REP** 成功示例：

```json
{
  "status": "ok",
  "project_path": "D:/MySong.tracktionedit",
  "tracks": [
    {"id": "1007", "name": "Audio Track 1", "type": "audio", "plugins": ["SPAN"]},
    {"id": "1006", "name": "Master", "type": "master", "plugins": []}
  ]
}
```

* **字段说明**：
  * `project_path`：当前已绑定磁盘工程绝对路径；未保存 / 仅模板会话时为**空字符串** `""`。
  * `tracks`：数组，涵盖 `te::getAllTracks` 返回的**全部**轨道（含 Master、系统轨等）。
  * 每条轨道：
    * `id`：`itemID.toString()`
    * `name`：显示名
    * `type`：`master`（Master 轨强制）；否则读取 ValueTree 的 **`vit_track_type`**，若无则回退 **`vit_type`**（`audio` / `midi` / `bus`，`group` 归一为 `bus`）；仍无法识别时默认为 **`audio`**
    * `plugins`：该轨 `pluginList` 上插件的**显示名称**字符串数组，无插件则为 `[]`

* **错误**：无活跃 `Edit` 时：`{"status":"error","message":"No active project"}`

**建轨约定**：`add_track` 会同时写入 `vit_type` 与 **`vit_track_type`**，供本指令与其它工具统一读取。

### `list_tracks`

用于 Godot 侧将场景轨道与内核 `id` / 轨名对齐，便于拖放导入与电平绑定。

---

**语义约定**：导入/自动化时 **禁止** 用鼠标像素作为「落点地址」；目标轨由 **`track_id`**（`import_audio` / `add_audio_clip`）标识；若使用 `add_audio_clip`，时间位置由 **`start_time`（秒）** 表达。`import_audio` 在目标轨上按片段末尾排队追加；后续若支持 `drop_x_pos → start_time`，将在本契约中单独修订。

## V1.1 新增契约：设备 I/O 管理 (Audio Device)

以下命令与上文 **`cmd` / `action` 统一路由规则**相同；示例统一使用 `cmd`。

### 1. `get_audio_device_types`

* **方向**: Godot -> ZMQ REQ -> C++
* **作用**: 获取系统支持的驱动类型列表（如 ASIO, Windows Audio）。
* **REQ**: `{"cmd": "get_audio_device_types"}`
* **REP**:

```json
{
  "status": "ok",
  "types": ["ASIO", "DirectSound", "Windows Audio"]
}
```

### 2. `get_audio_devices`

* **方向**: Godot -> ZMQ REQ -> C++
* **作用**: 获取指定驱动下的设备列表与当前物理状态。
* **REQ**: `{"cmd": "get_audio_devices", "type": "ASIO"}`
* **REP**:

```json
{
  "status": "ok",
  "current_device": "Focusrite USB ASIO",
  "current_output_device": "Focusrite USB ASIO",
  "current_input_device": "Focusrite USB ASIO",
  "current_sample_rate": 48000.0,
  "current_buffer_size": 512,
  "available_devices": ["Focusrite USB ASIO", "Realtek ASIO"],
  "available_output_devices": ["Focusrite USB ASIO", "Realtek ASIO"],
  "available_input_devices": ["Microphone (USB Audio)", "Focusrite USB ASIO"],
  "available_sample_rates": [44100.0, 48000.0, 96000.0],
  "available_buffer_sizes": [128, 256, 512, 1024]
}
```

*`available_devices` / `current_device` 保留兼容旧前端；拆分 I/O 时请以 `available_*_devices` 与 `current_*_device` 为准。*

### 3. `set_audio_device`

* **方向**: Godot -> ZMQ REQ -> C++
* **作用**: 执行物理设备或缓冲区的切换（必须在 C++ 消息线程执行以防死锁）。
* **REQ**（**推荐**：拆分输入/输出；与旧版兼容可仍传 `device_name`）:

```json
{
  "cmd": "set_audio_device",
  "type": "Windows Audio",
  "output_device_name": "扬声器 (USB AUDIO CODEC)",
  "input_device_name": "麦克风 (USB AUDIO CODEC)",
  "sample_rate": 48000.0,
  "buffer_size": 480
}
```

* **字段**:
  * `output_device_name`、`input_device_name`：可选；至少提供一个（或与旧字段二选一）。
  * **旧版** `device_name`：若未提供上述两项，则行为与此前一致（须为合法**输出**设备名；若同名亦在输入列表中则同时设输入）。

* **REP**: `{"status": "ok"}` 或 `{"status": "error", "message": "..."}`

### 3.1 `get_wave_input_devices`

* **作用**: 列出引擎内 **硬件** Wave 输入（`waveDevice`），供轨道路由。
* **REQ**: `{"cmd": "get_wave_input_devices"}`
* **REP**:

```json
{
  "status": "ok",
  "devices": [
    {"device_id": "…", "name": "Microphone (USB)", "alias": "…"}
  ]
}
```

### 3.2 `route_wave_input_to_track`

* **作用**: 将指定 **硬件** Wave 输入接到目标 `AudioTrack`（Tracktion `InputDeviceInstance::setTarget`）。`device_id` 空则选第一台可用硬件输入。
* **REQ**:

```json
{
  "cmd": "route_wave_input_to_track",
  "track_id": "1",
  "device_id": "可选；与 get_wave_input_devices 一致"
}
```

* **REP**: `{"status":"ok","track_id":"…","device_id":"…"}` 或 error。

### 4. `scan_plugins`（VST3 物理扫描）

* **方向**: Godot → ZMQ REQ → C++
* **作用**: 在指定的文件夹中阻塞扫描 VST3 插件，合并进内核的 `KnownPluginList`，并返回当前列表中所有 **VST3** 条目的摘要。
* **REQ**:

```json
{
  "cmd": "scan_plugins",
  "paths": ["C:/Program Files/Common Files/VST3"]
}
```

* **字段**:
  * `paths`：`string` 数组，每项为主机上的**绝对路径**；允许为空数组 → 不执行扫描，返回 `plugins: []` 且 `status: ok`。

* **REP** 示例:

```json
{
  "status": "ok",
  "plugins": [
    {
      "name": "SPAN",
      "format": "VST3",
      "identifier": "VST3-SPAN-…"
    }
  ]
}
```

* 错误示例：`paths` 缺失或非数组 → `{"status":"error","message":"scan_plugins requires paths array"}`；本构建未启用 VST3 → `VST3 plugin format is not available in this build`。
* **说明**: 内核使用 Tracktion `PluginManager` 的 `pluginFormatManager`（JUCE `AudioPluginFormatManager`）与 `knownPluginList`；`identifier` 由 `PluginDescription::createIdentifierString()` 生成。返回的 `plugins` 为扫描后列表中的全部 VST3 项（含此前已缓存的 VST3），不仅限于本次路径新增项。

### 5. `instantiate_plugin`

在 `track_id` 指定的轨道上加载第三方插件（当前以 VST3 路径匹配为主），并写入当前 `Edit` 的 Undo 事务。

* **REQ**:

```json
{
  "cmd": "instantiate_plugin",
  "track_id": "1007",
  "plugin_path": "C:/Program Files/Common Files/VST3/YourPlugin.vst3"
}
```

* **REP** 成功示例:

```json
{
  "status": "ok",
  "message": "Plugin instantiated",
  "track_id": "1007",
  "plugin_id": "....",
  "plugin_name": "YourPlugin"
}
```

### 6. `open_plugin_ui`

按 `track_id + plugin_id` 查找已实例化插件并弹出原生编辑器窗口（非模态浮窗）。

* **REQ**:

```json
{
  "cmd": "open_plugin_ui",
  "track_id": "1007",
  "plugin_id": "...."
}
```

* **REP** 成功: `{"status":"ok","message":"Plugin UI opened","track_id":"1007","plugin_id":"...."}`
* 兼容命令：`show_plugin_editor` 仍保留，语义与 `open_plugin_ui` 对齐。

### 7. `get_plugin_parameters`

返回插件参数快照，用于 AI 自动化抓手。

* **REQ**:

```json
{
  "cmd": "get_plugin_parameters",
  "track_id": "1007",
  "plugin_id": "...."
}
```

* **REP** 成功示例:

```json
{
  "status": "ok",
  "track_id": "1007",
  "plugin_id": "....",
  "template_role": "instrument",
  "supports_param_grabber": true,
  "param_aliases": {
    "param_1": "主滤波截止"
  },
  "binding_targets": [
    {
      "target_kind": "plugin_param",
      "plugin_id": "....",
      "param_id": "param_1"
    }
  ],
  "recommended_groups": [
    {
      "name": "Tone",
      "parameter_ids": ["param_1"]
    }
  ],
  "quick_controls": [
    {
      "param_id": "param_1",
      "label": "主滤波截止",
      "widget": "knob"
    }
  ],
  "control_shell": {
    "template_role": "instrument",
    "supports_param_grabber": true
  },
  "parameters": [
    {
      "id": "param_1",
      "raw_param_id": "param_1",
      "raw_param_name": "Cutoff",
      "name": "Cutoff",
      "alias": "主滤波截止",
      "normalized_role": "instrument_cutoff",
      "display_group": "Tone",
      "value": 0.5,
      "min": 0.0,
      "max": 1.0
    }
  ]
}
```

### 8. `set_plugin_param_aliases`

写入或删除插件参数别名。该别名会持久化到工程插件状态中。

* **REQ（单参数）**：

```json
{
  "cmd": "set_plugin_param_aliases",
  "plugin_id": "....",
  "param_id": "param_1",
  "alias": "主滤波截止"
}
```

* **REQ（批量）**：

```json
{
  "cmd": "set_plugin_param_aliases",
  "plugin_id": "....",
  "aliases": {
    "param_1": "主滤波截止",
    "instrument_cutoff": "亮度"
  }
}
```

* **REP**：

```json
{
  "status": "ok",
  "plugin_id": "....",
  "param_aliases": {
    "param_1": "主滤波截止",
    "instrument_cutoff": "亮度"
  },
  "alias_count": 2
}
```

### 9. 控制图（Phase 10）

#### `control_add_node`

新增普通控制节点。

```json
{
  "cmd": "control_add_node",
  "kind": "slider",
  "name": "Cutoff Macro",
  "x": 120.0,
  "y": 80.0
}
```

#### `control_add_macro`

新增宏面板节点。

```json
{
  "cmd": "control_add_macro",
  "name": "Main Macro",
  "x": 180.0,
  "y": 120.0,
  "macro_count": 8
}
```

#### `control_add_binding`

建立控制图绑定。当前支持：
- `plugin_param`
- `control_port`

```json
{
  "cmd": "control_add_binding",
  "source_node_id": "macro_xxx",
  "source_output": "macro_1",
  "target_kind": "plugin_param",
  "target_plugin_id": "....",
  "target_param_id": "param_1",
  "min": 0.0,
  "max": 1.0,
  "curve": "linear"
}
```

成功时返回：
- `binding`
- `control_graph`
- `graph_revision*`

其中 `binding` 已包含：
- `resolved_target_info`
- 若目标为插件参数，还会包含 `resolved_param_info`

#### `control_update_node`

更新控制节点元数据：
- `name`
- `x`
- `y`
- `min`
- `max`
- `default_value`
- `enabled`

#### `control_set_node_value`

设置控制节点某个输出值，并触发绑定传播。

```json
{
  "cmd": "control_set_node_value",
  "node_id": "control_xxx",
  "output": "value",
  "value": 0.75
}
```

成功时返回：
- `node`
- `applied_bindings`
- `control_graph`

#### `control_update_binding`

更新 binding 的：
- `enabled`
- `curve`
- `min`
- `max`
- `target_input`

#### `control_remove_binding`

按 `binding_id` 删除 binding。

#### `control_remove_node`

按 `node_id` 删除控制节点，并级联删除引用它的 bindings。

#### `control_set_macro_values`

批量设置宏面板多个输出，并触发传播。

```json
{
  "cmd": "control_set_macro_values",
  "node_id": "macro_xxx",
  "values": {
    "macro_1": 0.4,
    "macro_2": 0.8
  }
}
```

### 10. 2D Rack / AIGC / Take / Ghost（当前已落地）

以下命令当前已实现，可用于 Phase 3 / 7 / 8 验收：

- `rack_add_node`
- `rack_connect_pins`
- `rack_remove_connection`
- `rack_set_node_clip_scope`（`track_id`、`rack_item_id`、`plugin_item_id`、`clip_scope`；空字符串清除为 `track`）
- `aigc_register_job`
- `bridge_ingest_generated_asset`
- `switch_asset_take`
- `set_async_ghost_state`

这些命令的响应会回带最新的相关状态面，例如：
- `graph_revision*`
- `generated_assets`
- `jobs`
- `take_histories`

### 11. 连接器开放（Phase 11）

#### `connector_upsert_profile`

在工程外部 profile 存储中新增或更新 connector profile。

```json
{
  "cmd": "connector_upsert_profile",
  "display_name": "Suno Browser",
  "spec_id": "browser_download_audio",
  "entry_url": "https://suno.com/",
  "output_kind": "audio",
  "download_pattern": "*.wav",
  "enabled": true
}
```

成功时返回：
- `profile`
- `connector_profiles`
- `node_registry`

#### `connector_remove_profile`

```json
{
  "cmd": "connector_remove_profile",
  "profile_id": "connector_xxx"
}
```

### 12. 可观测性 / 健康检查（Phase 12）

#### `project_health_check`

返回三块结果：
- `observability`
- `project_health`
- `export_policy`

```json
{
  "cmd": "project_health_check"
}
```

成功响应示例：

```json
{
  "status": "ok",
  "observability": {
    "graph_revision": 42,
    "pending_job_count": 1,
    "active_take_count": 2
  },
  "project_health": {
    "issue_count": 1,
    "issues": [
      {
        "code": "pending_job",
        "severity": "info"
      }
    ]
  },
  "export_policy": {
    "recommended_mode": "use_cache",
    "supported_modes": ["wait", "use_cache", "bypass", "fail_export"]
  }
}
```

### 13. `get_project_state` 新增状态面（当前基线）

当前除 `tracks[]` 外，项目快照还包含：
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

## V1.1 契约升级：轨道拓扑 (Track Topology)

### 5. 修改原有的 `add_track` 指令

* **新增字段**: `type`（枚举值：`audio`, `midi`, `bus`）
* **REQ 示例**: `{"cmd": "add_track", "type": "audio"}`

---

## V0.6 生产力：录音、冻结、脱机导出

### 遥测 `transport`

除 `is_playing`、`position_seconds` 外，增加 **`is_recording`**（bool）。

### 遥测 `recording` / `recording_stopped`

停录时内核发布（ZMQ PUB，经 bridge 转 UDP 4444）：

```json
{"topic":"recording","subtopic":"recording_stopped","position_seconds":12.34}
```

**Python bridge**：收到该消息后 **必须** 自动发起一次 `get_project_state` REQ 并覆盖影子全量快照。

### `arm_track`

| 字段 | 类型 | 说明 |
|------|------|------|
| `track_id` | string | 目标音频轨 ID |
| `is_armed` | bool | 是否武装该轨的 Wave 输入实例 |

### `start_recording` / `stop_recording`

- `start_recording`：调用 Tracktion `TransportControl::record(false, false)`（未武装输入时可能无实质录音）。
- `stop_recording`：若正在录音则 `stopRecording(false)`。

### `freeze_track` / `unfreeze_track`

| 字段 | 说明 |
|------|------|
| `track_id` | 音频轨 ID |

- `freeze_track`：调度 `AudioTrack::freezeTrackAsync()`。
- `unfreeze_track`：`setFrozen(false, anyFreeze)`（在 Undo 事务内登记描述）。

### `start_render` / `cancel_render`

| 字段 | 说明 |
|------|------|
| `file_path` | 输出 WAV 绝对路径 |
| `range` | `[start_sec, end_sec]`（秒，闭合区间边界取 min/max 规范化） |
| `bit_depth` | 可选，默认 24 |
| `use_master_plugins` | 可选 bool，默认 `true` |

- `start_render`：异步脱机渲染；REP 返回 `job_id`。**渲染进行中** 绝大多数会修改 Edit 的指令会收到 `Engine is busy rendering`（allowlist 见实现：`ping`、`get_project_state`、`list_tracks`、`get_recent_projects`、`get_midi_clip_notes`、`get_midi_clip_data`、`get_plugin_parameters`、`project_health_check`、`get_audio_device_types`、`get_audio_devices`、`cancel_render`）。
- `cancel_render`：请求取消当前任务。

### 遥测 `render`

- `{"topic":"render","subtopic":"render_progress","job_id":"...","progress":0.0-1.0}`
- 完成：`subtopic` 为 `render_done` 或 `render_failed`。

### Godot（视觉防抖）

当 `transport.is_recording == true` 时，时间轴仅绘制伪红块；**勿**在录音期间读取波形共享内存；收到 `recording_stopped` 且对应 `tile_ready` 后再切换真实波形。

