# VSP Phase 0 Audit

日期：2026-07-02

状态：Phase 0 基线文档。本文只审计现状和迁移边界，不改 Kernel、Agent 或 Godot 实现代码。

## 目标与范围

Phase 0 的目标是把 Vit DAW 当前通信链路先摊平：哪些数据从哪里来、经过什么桥、被谁消费、哪些 payload 最重、哪些事件最高频、哪些兼容边界不能破坏。后续 Phase 1 的 VSP schema 和通道骨架应以本文为输入，而不是继续在旧 UDP/JSON 混流上补丁式扩张。

本文覆盖：

- Kernel：`D:\Vit_DAW\VitApp`，重点是 `ZmqGateway`、`VitHeadlessService`、`CommandDispatcher`。
- Agent：`D:\Vit_DAW\agent`，重点是 Go bridge、kernel client、harness、Ask Vit HTTP。
- Godot GUI：`D:\Godot\project\vit-daw-frontend`，重点是 `VitIpcClient`、`VitTelemetryManager`、project store、dock/timeline/rack 消费路径。
- 现有旧契约：`D:\Vit_DAW\docs\VIT_IPC_CONTRACT.md`。

## 当前环境状态

本轮进入 Phase 0 前已确认：

- 当前没有 `VitApp`、`vitagent`、Godot 主进程或 Godot console 进程占用运行链路。
- TCP `5555`、`5556`、`5557`、`7878` 未被占用。
- UDP `4444`、`4445` 未被占用。
- CMake：`4.3.0-rc2`。
- Go：`go1.26.2 windows/amd64`。
- Node：`v24.14.0`。
- npm：`11.9.0`。
- Godot 可执行文件存在：`D:\Godot\Godot_v4.6.1-stable_win64.exe` 与 `D:\Godot\Godot_v4.6.1-stable_win64_console.exe`。

已在本轮准备阶段通过的基础验证：

- `go test ./...` in `D:\Vit_DAW\agent`
- `npm run build` in `D:\Vit_DAW\agent\webui`
- `cmake --build D:\Vit_DAW\VitApp\build --config Release --target VitApp --parallel`

注意：`D:\Vit_DAW` 与 `D:\Godot\project\vit-daw-frontend` 当前都有既有未提交改动。Phase 0 只新增/更新 VSP 文档，不回滚、不清理、不重排这些实现改动。

## 现有传输地图

### Kernel

Kernel 侧当前由 `ZmqGateway` 暴露三类 endpoint：

| Endpoint | 当前用途 | 代码依据 |
| --- | --- | --- |
| `tcp://127.0.0.1:5555` | command REP，接收 bridge/agent 的命令请求 | `VitApp/Source/Service/ZmqGateway.h:19` |
| `tcp://127.0.0.1:5556` | PUB，发布 telemetry/event/status | `VitApp/Source/Service/ZmqGateway.h:20` |
| `tcp://127.0.0.1:5557` | log endpoint | `VitApp/Source/Service/ZmqGateway.h:21` |

命令处理仍集中在 `CommandDispatcher`。`get_project_state` 是当前最重要的聚合快照入口，会把工程拓扑、轨道、clip、插件、rack/control graph、observability、project health 等状态面打包成一个全量响应。

`VitHeadlessService` 以 `telemetryIntervalMs = 50` 启动定时遥测，当前会发布 `transport`、`levels`、recording/render 相关事件，以及资产 ready 类通知。也就是说，旧 PUB 流里同时承载了实时数据、状态提示、资产引用和事件。

### Agent / Bridge

Go agent 的默认运行端口和桥接关系：

| Endpoint | 当前用途 | 代码依据 |
| --- | --- | --- |
| UDP `127.0.0.1:4445` | 接收 Godot command | `agent/README.md:7`，`agent/cmd/vitagent/main.go:29` |
| UDP `127.0.0.1:4444` | 转发 telemetry 给 Godot | `agent/README.md:8`，`agent/cmd/vitagent/main.go:28` |
| ZMQ REQ `tcp://127.0.0.1:5555` | bridge/agent 发命令到 Kernel | `agent/README.md:9`，`agent/cmd/vitagent/main.go:25` |
| ZMQ SUB `tcp://127.0.0.1:5556` | bridge/agent 订阅 Kernel telemetry | `agent/README.md:10`，`agent/cmd/vitagent/main.go:26` |
| HTTP `127.0.0.1:7878` | Ask Vit HTTP API | `agent/README.md:11`，`agent/cmd/vitagent/main.go:30` |

Bridge 对 UDP 大包已有文件回退：

- command reply 超过 `32 * 1024` bytes 时走 `file_reply`。
- telemetry packet 超过 `32 * 1024` bytes 时也走 `file_reply`。

这说明旧 UDP 链路已经被迫承担超出“小包 IPC”的数据规模。文件回退能避免单包失败，但会把磁盘 IO、JSON parse 和生命周期清理带到 command/telemetry 热路径里。

### Godot GUI

Godot 当前 autoload 里关键通信节点：

- `VitIpcClient="*res://app/kernel/autoloads/vit_ipc_client.gd"`
- `VitTelemetryManager="*res://app/kernel/autoloads/telemetry_manager.gd"`
- `GuiTransportClock="*res://app/kernel/autoloads/gui_transport_clock.gd"`
- `LiveSpectrumBus="*res://app/kernel/autoloads/live_spectrum_bus.gd"`

`VitIpcClient`：

- 使用 UDP 任意本地端口发送到 `127.0.0.1:4445`。
- FIFO 串行化 command。
- 应答从同一个 UDP socket 返回。
- 支持 `file_reply` 解析。

`VitTelemetryManager`：

- 绑定 UDP `4444`。
- 单个 autoload 同时处理 `levels`、`transport`、`delta_update`、`tile_ready`、`audio_feature_data_ready`、recording/render/job 等消息。
- 每帧最多处理 `64` 个 telemetry packet。
- 每帧 telemetry 处理预算为 `4 ms`。
- levels UI 分发节流为 `80 ms`。
- 支持 telemetry `file_reply` 解析，最大文件回退读取上限为 `64 MB`。

## 通道矩阵

| 数据类别 | 当前路径 | 当前消费者 | 主要问题 |
| --- | --- | --- | --- |
| Session | 没有正式 VSP session；依靠端口可用、进程启动、`ping` 或隐式约定 | Godot、Agent、smoke scripts | 无 hello/capability/version/role；客户端无法可靠知道对端能力和协议版本 |
| Command | Godot `VitIpcClient` -> UDP `4445` -> Go bridge -> ZMQ `5555` -> `ZmqGateway` -> `CommandDispatcher`；Agent harness/chat -> kernel client -> ZMQ `5555` | Godot GUI、Agent、scripts | command schema 是历史 JSON 集合；缺少统一 envelope、transaction、revision、capability、错误码 |
| State | `get_project_state` 全量快照；部分 `delta_update` 作为低延迟提示 | Godot project store、dock/timeline/rack；Agent shadow/harness | 全量快照过大；结构性操作后频繁回拉；delta 不是独立真源，丢失后必须靠全量修复 |
| Realtime | Kernel PUB `levels` / `transport` -> bridge -> UDP `4444` -> `VitTelemetryManager` | meter、transport bar、timeline、spectrum、mixer dock | 50ms 全局推送；所有轨数据先到 GUI 再过滤；实时消息与资产/状态/事件共用处理预算 |
| Asset | `tile_ready`、`audio_feature_data_ready`、shared memory 引用、文件回退 | waveform/tile manager、spectral dock、mixboard feature snapshot、Agent collectors | 资产引用与高频 telemetry 混流；SHM miss、文件读取、tile 分发会占用 GUI 帧预算 |
| Event | recording/render/job/progress、`recording_stopped`、delta gap 等 | Godot telemetry manager、Agent bridge/harness | 事件缺少独立事件通道、ack 与 replay 策略；部分事件会触发全量 refresh |
| Log/Trace | ZMQ log endpoint 与应用日志 | 开发/诊断工具 | 与协议层 tracing 未统一；后续需要 request_id/session_id 贯穿 |

## 重 payload 与高频事件

当前最容易拖慢 GUI 或扩大状态洪水的 payload：

1. `get_project_state`
   - 当前是工程拓扑真源，承载轨道、clip、插件、rack/control graph、observability、health 等多状态面。
   - Godot 多处在结构性操作后回拉全量。
   - Agent shadow/harness 也依赖它初始化或重新同步。

2. `levels`
   - Kernel 以约 50ms tick 发布。
   - 当前按全局 telemetry 流推到 Godot，再由 GUI 侧分发/过滤。
   - 如果包含每轨 realtime/spectrum 派生数据，不可见轨也可能先进入 Godot 热路径。

3. `transport.recording_waveforms`
   - 录音状态下会随 transport 周期性附带录中波形信息。
   - 虽已有 stride 降频，但仍与 transport 热路径共用同一 telemetry 流。

4. `tile_ready` / `audio_feature_data_ready`
   - 资产 ready 消息走同一 telemetry 入口。
   - GUI 侧需要读 shared memory、处理 orphan bucket、更新 feature snapshot、分发到 track receivers。
   - 当 UDP 超过 32KB 后进入文件回退，进一步引入磁盘读取和 JSON parse。

5. `file_reply`
   - command reply 和 telemetry 都存在文件回退。
   - 它是兼容救生艇，不应成为 VSP 主干设计的一部分。

## 全量快照依赖点

已确认的核心依赖：

- Agent bridge 在收到 `get_project_state` 成功应答后初始化/覆盖 shadow。
- Agent bridge 在 `recording_stopped` 后触发 `get_project_state` refresh。
- Agent harness 的 shadow refresh 使用 `get_project_state`。
- Agent mix tick 路径仍会读取 `get_project_state`。
- Godot `app/state/stores/project_store.gd` 直接通过 project client 调 `get_project_state`。
- Godot legacy main controller、Dock root、timeline/rack/track 场景中仍存在多处 refresh/rebuild 后回拉全量。

Phase 3 迁移前，`get_project_state` 仍必须作为兼容真源保留；但 Phase 1/2 设计必须为 snapshot + delta + revision gap recovery 留出正式通道，不能继续让所有结构性变化都变成“全量快照再重绘”。

## GUI 卡顿模型

旧链路导致 GUI 卡顿的主因不是单个函数慢，而是数据类别混流后互相放大：

1. Realtime、Asset、Event、State hint 共用 ZMQ PUB -> bridge -> UDP `4444` -> `VitTelemetryManager`。
2. Godot 每帧 telemetry 预算有限，高频 `levels/transport` 会与 `tile_ready/audio_feature_data_ready` 竞争同一帧预算。
3. Asset ready 触发的 SHM 读取、orphan tile 分桶、feature snapshot 更新，和播放指针/电平这类实时 UI 没有调度隔离。
4. 结构性操作后大量调用 `get_project_state`，全量 JSON parse、repository 写入、scene rebuild 可能和实时遥测同帧发生。
5. 大 UDP payload 的 `file_reply` 回退把文件 IO 带入 command/telemetry 处理路径。
6. 可见区域订阅还不是协议能力；GUI 可能先收到全轨数据，再由本地决定是否展示。

因此，VSP 的核心不是“把 UDP 换成另一个 socket”而已，而是先把 Command、State、Realtime、Asset、Event、Session 分通道、分预算、分一致性语义。

## 旧 IPC 兼容边界

Phase 1 之后仍必须保留旧 IPC 作为 legacy adapter，直到 GUI 与 Agent 按通道迁移完成。

不能破坏的旧命令和能力包括但不限于：

- `ping`
- `play`
- `stop`
- `get_project_state`
- `list_tracks`
- `import_audio`
- `project.import_audio_files`
- `add_track`
- `delete_track`
- `move_clip`
- `set_tempo`
- `undo`
- `redo`
- `get_midi_clip_notes`
- `get_plugin_parameters`
- `plugin_grabber_apply_eq_edits`
- render/import/project health 相关命令

兼容原则：

- 旧 JSON command 可以被 VSP command adapter 包装，但不能要求现有 Godot/Agent 调用点一次性全部改完。
- 旧 `get_project_state` 在迁移期继续作为权威回拉入口。
- 旧 UDP telemetry 可以继续作为兼容入口，但不能再作为 VSP 主干语义。
- 不把 Tracktion Engine 私有结构暴露为 VSP 长期契约。
- 不把 Agent 插件抓手智能搬进 Kernel。Kernel/VSP 只提供低延迟、可批量、可验证的插件读写执行通道。

## 跨平台协议边界

VSP 应定义为“应用会话契约 + transport bindings”，而不是 Windows IPC 规格。按这个边界设计，VSP 同样适用于未来 macOS 版本。

必须保持平台中立的内容：

- envelope、request/response、event、snapshot、delta、asset reference、capability、error code。
- command/state/realtime/asset/event/session 的语义。
- client role、session id、revision、transaction id、feature flag。
- asset identity、content hash、byte range、format、lifetime、cache policy。

不能进入核心协议的内容：

- Windows 反斜杠路径作为唯一资产身份。
- Windows shared memory name/handle 作为唯一资产引用。
- Win32 process id、DLL/GDExtension 路径、WebView2 细节。
- Tracktion Engine 内部 ValueTree 或 plugin index 作为长期外部契约。

建议边界：

- 核心 VSP 只写 URI/asset id/capability，不写具体平台句柄。
- Windows binding 可以使用 TCP、named shared memory、文件回退。
- macOS binding 可以使用 TCP、Unix domain socket、POSIX shared memory、mmap/file cache。
- Godot/Agent 侧只依赖 VSP SDK 的抽象，不直接依赖平台传输细节。

## Phase 1 进入条件

Phase 1 开始前应先冻结以下决策：

1. VSP envelope 最小字段：
   - `vsp_version`
   - `message_id`
   - `request_id`
   - `session_id`
   - `client_id`
   - `role`
   - `channel`
   - `type`
   - `capabilities`
   - `revision`
   - `transaction_id`
   - `trace_id`

2. Session hello/capability：
   - Kernel、GUI、Agent 都要能声明 role 与 capability。
   - 客户端能知道哪些 channel、schema version、feature flag 可用。
   - 旧 IPC adapter 也应能通过 capability 告知“legacy only”状态。

3. Command wrapper：
   - 旧命令先进入 `legacy.command` adapter。
   - 新 VSP command 使用统一 request/response/error envelope。
   - 每个写命令必须有 transaction 或 revision 语义，不能只依赖隐式 JSON 字段。

4. State 策略：
   - 保留 full snapshot。
   - 设计 delta revision、gap detection、client resync。
   - 明确哪些 state 面可以分片订阅。

5. Realtime 策略：
   - latest-only。
   - drop-old。
   - rate limit。
   - visible-range / track subscription。
   - realtime 不等待 asset/state 大包。

6. Asset 策略：
   - asset ready event 只传引用和元数据。
   - 大数据通过 asset channel/cache/SHM/file binding 获取。
   - 文件回退是 binding 层能力，不是核心协议语义。

## Phase 0 结论

当前系统已经证明 Kernel、Godot GUI、Agent 可以分进程协作，但旧链路把多种一致性需求不同的数据塞进同一套 ZMQ PUB/UDP/JSON 桥里。它能跑，但无法自然扩展到多轨、多资产、高频实时 UI、Agent 自动化和未来跨平台 SDK。

Phase 1 应从 schema/envelope、session/capability、legacy adapter、channel budget 开始，而不是先替换某一个 socket 或直接重写 Godot 调用点。旧 IPC 必须作为兼容层保留；VSP v1 则负责把协议语义定稳，让后续 State、Realtime、Asset、Agent、GUI 迁移能逐步发生。

## 证据索引

| 观察 | 证据 |
| --- | --- |
| Kernel ZMQ endpoint | `VitApp/Source/Service/ZmqGateway.h:19-21` |
| Agent 默认端口 | `agent/README.md:7-11`，`agent/cmd/vitagent/main.go:25-31` |
| Kernel 50ms telemetry | `VitApp/Source/Service/VitHeadlessService.h:44`，`VitApp/Source/Service/VitHeadlessService.cpp:340` |
| Kernel telemetry 发布入口 | `VitApp/Source/Service/VitHeadlessService.cpp:647` |
| transport recording waveform | `VitApp/Source/Service/VitHeadlessService.cpp:221-229` |
| levels topic | `VitApp/Source/Service/VitHeadlessService.cpp:747` |
| Bridge UDP 文件回退阈值 | `agent/internal/bridge/bridge.go:65-66` |
| Bridge command reply 文件回退 | `agent/internal/bridge/bridge.go:161-187` |
| Bridge telemetry 文件回退 | `agent/internal/bridge/bridge.go:300-324` |
| Bridge shadow 初始化/刷新 | `agent/internal/bridge/bridge.go:219`，`agent/internal/bridge/bridge.go:348-388` |
| Harness full refresh | `agent/internal/harness/harness.go:9596` |
| Mix tick full snapshot | `agent/internal/harness/mix_tick.go:541` |
| Godot autoload 通信节点 | `project.godot:20-29` |
| Godot command UDP | `app/kernel/autoloads/vit_ipc_client.gd:2`，`app/kernel/autoloads/vit_ipc_client.gd:6-14` |
| Godot command file reply | `app/kernel/autoloads/vit_ipc_client.gd:50-83` |
| Godot telemetry UDP | `app/kernel/autoloads/telemetry_manager.gd:2`，`app/kernel/autoloads/telemetry_manager.gd:9` |
| Godot telemetry frame budget | `app/kernel/autoloads/telemetry_manager.gd:29-37` |
| Godot telemetry mixed dispatch | `app/kernel/autoloads/telemetry_manager.gd:223-261` |
| Godot asset telemetry handlers | `app/kernel/autoloads/telemetry_manager.gd:630`，`app/kernel/autoloads/telemetry_manager.gd:777` |
