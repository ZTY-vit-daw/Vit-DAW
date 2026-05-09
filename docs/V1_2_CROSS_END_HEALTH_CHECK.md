# Vit-DAW V1.2 跨端体检结果

审计时间：2026-05-06  
审计方式：只读代码审计，未运行 Godot、VitApp 或抓包。  
输入基线：`docs/v1.2 cross end audit summary.md`

## 0. 总结结论

当前仓库能证实 C++ Tracktion 内核与 Python Bridge 已形成稳定的命令链路：

`Godot/UDP -> bridge_core.py -> ZMQ REQ/REP -> CommandDispatcher`

反向链路为：

`CommandDispatcher / VitHeadlessService / VitDeltaProbe -> ZmqGateway PUB -> bridge_core.py UDP -> Godot`

Godot 前端源码位于 `D:\Godot\project\vit-daw-frontend`，本轮已补查 `vit_ipc_client.gd`、`telemetry_manager.gd`、`vit_dock_root.gd`、`Vit_Graph_Rack.gd`、`tile_manager.gd` 等关键路径。Godot 侧能证实 Dock 路径会在多类结构性操作成功后回拉 `get_project_state`，但也能证实当前 `VitIpcClient` 实装与调用方期望存在明显 API 漂移。

最高风险集中在三处：

1. PUB/UDP 增量链路天然可丢包，且 C++ delta ring 满时静默丢弃；如果 Godot 把 `delta_update` 当工程真源，存在永久漂移风险。
2. SHM tile 生命周期只看到“中止后续 bake”的 generation 机制，未看到真正关闭已发布 mapping handle 的回收路径；删除/撤销/重做后仍需运行验证。
3. Rack 前端确实先乐观改 GraphEdit 再发 IPC；成功路径会由 Dock 回拉全量，失败/超时路径主要靠本地 rollback，未自动强制重拉内核快照。

## A. 工程状态真源与收敛路径

| 字段 | 结论 |
|------|------|
| Observation | C++ `get_project_state` 会构建全量工程快照，包含 `tracks[]`、`plugins`、`clips`、`rack`、`control_graph`、`observability`、`project_health`、`graph_revision` 等状态面。Bridge 仅在收到 `cmd == "get_project_state"` 且回复 `status == "ok"` 时初始化 Python 影子工程。Godot Dock 的 `_refresh_project_repository_from_kernel` 会 await `get_project_state` 并 emit `project_loaded`。 |
| Risk | 如果 Godot 某些操作后只消费 `delta_update`，不回拉 `get_project_state`，则在 PUB/UDP 丢包或 delta ring 满时可能永久漂移。Dock 成功路径覆盖较多，但旧 Main / 兼容路径与同名刷新函数存在分叉。 |
| Evidence | `VitApp/Source/Service/CommandDispatcher.cpp:2250-2330`；`scripts/bridge_core.py:289-307`；`D:\Godot\project\vit-daw-frontend\vit_dock\scenes\vit_dock_root.gd:202-232`。 |
| Unknown | 实际运行 Autoload 是否就是当前 `vit_ipc_client.gd` 未运行时验证；旧 Main 场景是否仍作为生产入口未证实。 |
| Recommendation tier | T0：在 Godot 日志中打印每次全量回拉原因与调用入口。T1：对 rack/track/clip 结构变更成功后保持强制回拉，并补齐失败路径的低频对账回拉。 |

### A1. `get_project_state` 调用矩阵

已证实的服务端行为：

- `CommandDispatcher` 注册 `get_project_state`，实际进入 `handleGetProjectState`。
- `handleGetProjectState` 在必要时 `ensureTrackRackGraphForEdit`，随后输出全量状态。
- Bridge 观察到 `get_project_state` 成功回复后，用该快照初始化/覆盖 `VitShadowProject.state.engine_snapshot`。

已证实的客户端行为：

- Dock `vit_refresh_project_from_kernel` / `_refresh_project_repository_from_kernel` 会发送 `{"cmd":"get_project_state"}`，成功后写入 `ProjectRepository` 并发出 `project_loaded`。
- Dock 的 Undo/Redo 成功后会 `await vit_refresh_project_from_kernel()`。
- Dock 的 `delete_track`、保存/打开/新建、插件/rack 成功回调、clip 变更命令成功回调会回拉全量。
- `clip_3d_container.gd` 的刷新请求优先转给 `main_ui_controller._request_refresh_project_state_after_edit`；降级分支会用 `send_command_async` 发送 `get_project_state`，但该 API 与当前 `VitIpcClient` 签名不匹配。
- `timeline_dock_adapter.gd` 也定义 `_request_refresh_project_state_after_edit`，但语义是本地 `_refresh_from_project_repository()`，不是内核回拉；这是同名分叉风险。

### A2. PUB / UDP 语义与丢包敏感性

| 字段 | 结论 |
|------|------|
| Observation | C++ PUB 消息至少包括 `delta_update`、`transport`、`levels`、`tile_ready`、`track_duration_ready`、`log`、gateway status。Bridge 对 SUB 收到的 JSON 基本单行规范化后盲转 UDP。 |
| Risk | `ZmqGateway::publishMessage` 使用 `dontwait`，Bridge UDP `sendto` 无 ACK；`VitDeltaRingBuffer::tryPush` 满时返回 false，但监听器忽略返回值。增量链路不应作为唯一工程真源。 |
| Evidence | `VitApp/Source/Service/ZmqGateway.cpp:269-290`；`VitApp/Source/Core/VitDeltaProbe.h:28-33`；`VitApp/Source/Core/VitDeltaProbe.cpp:90-118`；`scripts/bridge_core.py:232-253`。 |
| Unknown | Godot `telemetry_manager.gd` 会将 `delta_update` 分发给 UID registry 命中的节点 `apply_delta`；具体节点是否把它当真源需要逐节点验证。 |
| Recommendation tier | T0：为 delta ring drop 增加计数日志；Bridge UDP 转发失败增加节流告警；Godot 侧记录 `delta_update.seq_id` 间隙。T1：关键结构变更后要求客户端回拉全量。T2：只有在证实 UI 依赖增量且漂移不可接受时，再引入 revision/ACK/重放机制。 |

### A3. Undo/Redo 与 UI 刷新一致性

| 字段 | 结论 |
|------|------|
| Observation | C++ `undo`/`redo` 调用 `UndoManager` 后执行 `dispatchPendingUpdatesSynchronously()` 与 `ensureContextAllocated(true)`，但回复只是 `status: ok`，不携带全量工程状态。 |
| Risk | Dock 路径在 undo/redo 成功后会回拉全量，但前提是 `VitIpcClient` 存在 `send_command_async_await`。当前 Autoload 脚本未实现该方法，运行态若未被其他脚本替换，Dock 的 await 路径会直接降级/失效。 |
| Evidence | `VitApp/Source/Service/CommandDispatcher.cpp:2130-2164`；`D:\Godot\project\vit-daw-frontend\vit_dock\scenes\vit_dock_root.gd:190-199`；`D:\Godot\project\vit-daw-frontend\vit_ipc_client.gd:72-114`。 |
| Unknown | SHM tile 在 undo 恢复 clip 时是否自动重建未证实；实际运行 Autoload 是否不同于当前文件需运行时确认。 |
| Recommendation tier | T0：启动时打印 `VitIpcClient.get_method_list()`。T1：补齐 `send_command_async_await` 后保持 undo/redo 成功回拉全量。 |

## B. IPC 契约与客户端一致性

| 字段 | 结论 |
|------|------|
| Observation | C++ 命令分发接受 `cmd -> action -> command` 三种字段，找不到 handler 时返回 `Unknown command`。Bridge 控制链路是单个 ZMQ REQ socket 顺序发送/接收，超时后重建 socket。Godot 当前 `vit_ipc_client.gd` 只实现 `send_command_json_async`、`send_command_json`、单参数 `send_command_async`，未实现大量调用方使用的 `send_command_async_await` 与 `send_command_no_wait`。 |
| Risk | 这是当前 Godot 侧最高确定性风险：调用方与 Autoload API 不一致会导致全量刷新、rack 成功回调、transport no-wait 等路径失效。`send_command_async` 直发不登记 `_pending_command`，但同一 UDP socket 仍会收到 reply，可能被后续 pending 请求误吞。 |
| Evidence | `VitApp/Source/Service/CommandDispatcher.cpp:1727-1756`；`scripts/bridge_core.py:274-309`；`VitApp/Source/Service/ZmqGateway.cpp:151-210`；`D:\Godot\project\vit-daw-frontend\vit_ipc_client.gd:23-39`、`:72-114`；Godot 调用方 grep 命中 `send_command_async_await` / `send_command_no_wait` / 多参 `send_command_async`。 |
| Unknown | 是否存在运行时替换版 Autoload 未证实；需以 Godot 启动时方法列表确认。 |
| Recommendation tier | T0：启动时打印/断言 `VitIpcClient` 公开 API。T1：补齐 `send_command_async_await`、`send_command_no_wait`、多参 `send_command_async`，或批量改调用方到现有 API，二选一。 |

### B1. 命令字段抽样

已证实 C++ 高频/结构命令字段：

- `delete_track` 要求 `track_id`。
- `rack_connect_pins` 要求 `track_id`、`rack_item_id`、`source_id`、`dest_id`、`source_pin`、`dest_pin`。
- `rack_remove_connection` 要求同上。
- `import_media_to_track` 要求 `file_path`、`track_id`、`media_type == "audio"`、`start_time`。
- `warm_waveform_bake` 可按 `clip_id` 或 `file_path` 启动 bake。

### B2. 客户端能力一致性

已证实不一致：

- `vit_ipc_client.gd` 定义 `send_command_json_async(cmd, request_id := "")`，队列 `_queued_commands` + 单槽 `_pending_command`。
- `vit_ipc_client.gd` 定义 `send_command_async(cmd)`，只 `put_packet`，不登记 request id，不等待 reply。
- `vit_ipc_client.gd` 未定义 `send_command_async_await`、`send_command_no_wait`。
- 多个调用方调用 `send_command_async_await(...)`、`send_command_no_wait(...)`、`send_command_async(cmd, request_id)`、`send_command_async(cmd, request_id, timeout)`。

风险是 reply 关联与方法存在性同时出问题：即使某些调用被 `has_method` 保护，受保护分支会直接 return 或走本地 fallback，导致“看似运行但未真正回拉内核”。

## C. Rack：乐观 UI 与内核真源

| 字段 | 结论 |
|------|------|
| Observation | C++ rack 写操作在校验通过后才修改内核：`rack_add_node`、`rack_connect_pins`、`rack_remove_connection` 都开启 Undo transaction，flush rack/track，并调用 `VitGraphSwapCoordinator::publishGraphChange`。Godot `Vit_Graph_Rack.gd` 在连线/断线请求中先 `connect_node` / `disconnect_node`，再发 IPC，是明确的乐观 UI。 |
| Risk | 成功路径理论上由 Dock `command_completed` 分支回拉全量；失败/超时路径发出失败信号并在 `Vit_Graph_Rack` 本地 rollback，但不强制回拉内核。且当前 `send_command_async(cmd, request_id)` 与 `VitIpcClient` 实装不一致，可能导致成功回调分支不触发。 |
| Evidence | `VitApp/Source/Service/CommandDispatcher.cpp:5728-5830`；`VitApp/Source/Service/CommandDispatcher.cpp:5840-6016`；`VitApp/Source/Core/VitGraphSwapCoordinator.cpp:87-120`；`D:\Godot\project\vit-daw-frontend\Vit_Graph_Rack.gd:1622-1669`、`:2425-2474`；`D:\Godot\project\vit-daw-frontend\vit_dock\scenes\vit_dock_root.gd:245-278`。 |
| Unknown | 实际运行中 rack `command_completed` 是否因 request_id 契约问题缺失，需抓 Godot/Bridge 日志。 |
| Recommendation tier | T0：用一次故意非法连线验证 Godot rollback 与是否回拉。T1：失败/超时后加一次去抖的 `get_project_state` 回拉；同时修复 `VitIpcClient` API。 |

最小验证步骤：

1. 在 Godot 选择两个会被 `VitDagChecker::wouldCreateCycle` 拒绝的节点，尝试连线。
2. 观察 C++ 是否返回 `Connection would create a cycle in the rack DAG`。
3. 观察 Godot 画布是否保留失败边。
4. 观察失败后是否触发 `get_project_state`。

## D. SHM 波形瓦片生命周期

| 字段 | 结论 |
|------|------|
| Observation | `TiledSpectrogramBaker::startBake` 为每个 tile 创建 Windows file mapping，写入后 `UnmapViewOfFile`，并通过 `storeHandle` 保存 handle；发布 `track_duration_ready` 与 `tile_ready`，消息含 `session_id`、`track_id`、`clip_id`、`shared_memory`。 |
| Risk | 已发布 mapping handle 的关闭/回收路径未在本次读取范围内看到；`releaseTrackMappings(trackId)` 与 `invalidateClipBake(clipId)` 只是调用 `beginGen`，看起来更像“使旧 generation 失效/停止后续 bake”，不等价于主动关闭已发布 handle。 |
| Evidence | `VitApp/Source/Service/TiledSpectrogramBaker.cpp:321-335`；`VitApp/Source/Service/TiledSpectrogramBaker.cpp:655-698`；`VitApp/Source/Service/TiledSpectrogramBaker.cpp:703-712`。 |
| Unknown | `storeHandle` 内部是否有上限/替换/析构关闭 handle 未在本次片段证实；Godot/GDExtension 是否主动 unmap/丢弃纹理未证实。 |
| Recommendation tier | T0：增加 handle 数量、generation、clip_id 的诊断日志。T1：删除 clip/track 时显式关闭对应 tile handles 或集中缓存淘汰。T2：仅当运行验证证实泄漏时，再重构 tile session 管理。 |

### D1. C++ 所有权与释放点

已证实释放/失效触发点：

- `delete_track` 删除轨道后调用 `TiledSpectrogramBaker::releaseTrackMappings(releasedTrackID)`。
- `remove_clips` 删除音频 clip 前调用 `TiledSpectrogramBaker::invalidateClipBake(clip_id)`。
- `clear_project` 移除所有 clip 后对每条 track 调用 `releaseTrackMappings(track_id)`。
- `VitHeadlessService::stop`、加载项目、新建空项目时对旧 edit 的所有 track 调用 `releaseTrackMappings(track_id)`。

风险点：

- `startBake` 的 bake key 是 `trackId + ":" + clipId`；`releaseTrackMappings(trackId)` 只传 trackId。若 `beginGen` 使用完整 key 精确匹配，则 track 级释放可能无法覆盖 clip 级 bake key。需要读取/运行验证 `beginGen` 与 `storeHandle` 的具体 key 语义。
- `add_audio_clip` 插入音频 clip 后未看到启动 `TiledSpectrogramBaker::startBake`；`import_media_to_track` 与 `bridge_ingest_generated_asset` 会通过 `insertWaveClipWithUndoAndStartBake` 启动 bake。

证据：`CommandDispatcher.cpp:2443-2483`、`CommandDispatcher.cpp:3514-3600`、`CommandDispatcher.cpp:5097-5135`、`VitHeadlessService.cpp:221-242`、`VitHeadlessService.cpp:348-398`、`VitHeadlessService.cpp:589-630`、`CommandDispatcher.cpp:2486-2560`、`CommandDispatcher.cpp:3602-3693`。

### D2. Godot / GDExtension

已证实 Godot 消费路径：

- `telemetry_manager.gd` 绑定 UDP 4444，按 `type == "delta_update"`、`command == "tile_ready"`、`command == "track_duration_ready"`、`topic == "levels"/"transport"` 分发。
- `tile_ready` 进入 `_consume_tile_ready_payload` 后调用 `VitWaveformReader.read_shared_memory(shared_memory_name, TILE_RGBA_FLOAT_COUNT)`，创建 `ImageTexture`，再分发给 track/spectral receiver。
- 若没有 receiver 且 `clip_id` 非空，会写入 `_global_orphaned_tiles[clip_id]`；若 `clip_id` 为空则丢弃。
- `tile_manager.gd` 对无 `clip_id` 的 legacy tile 严格按 session 过滤；有 `clip_id` 的 tile 即使 session mismatch 也允许进入 clip 路由/缓存。
- `tile_manager.prune_clip_shells_by_backend_ids` 会清 mesh、`queue_free` clip shell、擦除 `_pending_tiles_cache`，并调用 `VitTelemetryManager.erase_orphaned_tiles_for_clip`。

缺口：

- Godot 脚本里未见 `VitWaveformReader` 的显式 unmap/dispose API；`read_shared_memory` 的 native 生命周期仍需查 GDExtension 源码或运行时对象计数。
- `telemetry_manager` 分发给 `vit_track_rows` 依赖 receiver 实现 `get_bound_kernel_track_id` 或 `_kernel_id_for_this_row`；是否所有 Main track row 都实现，需继续逐文件确认。

### D3. 并发与乱序包

| 字段 | 结论 |
|------|------|
| Observation | tile 消息带 `session_id` 与 `clip_id`；C++ bake 线程会在每个 tile/frame 前检查 generation 是否仍有效。 |
| Risk | 删除事件与已发出的 `tile_ready` UDP 包可能乱序到达 Godot。Godot 是否检查 `session_id`、`clip_id` 仍存在，未证实。 |
| Evidence | `TiledSpectrogramBaker.cpp:384-386`、`TiledSpectrogramBaker.cpp:400-402`、`TiledSpectrogramBaker.cpp:684-697`。 |
| Unknown | Godot 有 session/clip 路由，但有 `clip_id` 的 tile 被允许跨 session；这是否会在 undo/redo 或删除竞态中挂到旧 clip，需要运行验证。 |
| Recommendation tier | T0：Godot tile 消费处记录 `session_id`、`clip_id`、track/clip 是否仍存在。T1：消费 tile 前验证 clip 当前仍属于 `get_project_state`，至少在删除/undo 后对孤儿桶做剪枝。 |

## E. Top 10 风险清单

| 排名 | 风险 | 概率 | 严重度 | 来源路径 | 建议 |
|------|------|------|--------|----------|------|
| 1 | `delta_update` 经 ring + ZMQ PUB dontwait + UDP，任何环节可丢；若 UI 当真源会永久漂移 | 高 | 高 | `VitDeltaProbe.*`、`ZmqGateway.cpp`、`bridge_core.py` | T0/T1 |
| 2 | `VitIpcClient` API 与大量调用方不一致，`send_command_async_await` / `send_command_no_wait` 缺失 | 高 | 高 | `D:\Godot\project\vit-daw-frontend\vit_ipc_client.gd`、调用方 grep | T0/T1 |
| 3 | `send_command_async` 直发与 queued pending 共用 UDP socket，reply 可能错配/被误吞 | 高 | 高 | `vit_ipc_client.gd` | T1 |
| 4 | SHM mapping handle 回收未证实，删除/撤销/重做后可能资源滞留 | 中 | 高 | `TiledSpectrogramBaker.cpp`、`telemetry_manager.gd` | T0/T1 |
| 5 | `releaseTrackMappings(trackId)` 与 bake key `trackId:clipId` 可能不对称 | 中 | 高 | `TiledSpectrogramBaker.cpp` | T0 |
| 6 | Rack 失败路径仅本地 rollback，不自动内核重拉；且 request_id 契约可能让成功回调失效 | 中 | 高 | `Vit_Graph_Rack.gd`、`vit_dock_root.gd` | T0/T1 |
| 7 | Undo/Redo 只返回 ok，不带全量状态；Dock 回拉依赖缺失的 `send_command_async_await` | 中 | 高 | `CommandDispatcher.cpp`、`vit_dock_root.gd`、`vit_ipc_client.gd` | T1 |
| 8 | `add_audio_clip` 插入后未启动 waveform bake，可能与 import 路径行为不一致 | 中 | 中 | `CommandDispatcher.cpp` | T0/T1 |
| 9 | Bridge 控制链路单 REQ socket顺序等待，长命令会阻塞后续命令 | 中 | 中 | `bridge_core.py`、`ZmqGateway.cpp` | T0 |
| 10 | C++ 命令接受 `cmd/action/command` 三种字段，Godot 仍有 `action` 旧命令，易长期分叉 | 中 | 中 | `CommandDispatcher.cpp`、`top_transport_bar.gd` | T0 |

## F. 最小验证计划

1. Godot 启动后打印 `/root/VitIpcClient.get_method_list()`，确认运行态是否真的缺 `send_command_async_await` / `send_command_no_wait`。
2. 单次 rack 连线与故意非法连线：核对出站 JSON、request_id、`command_completed` 是否命中 Dock 回拉分支，以及失败后是否只 rollback。
3. 导入音频后立即删除 clip/track，观察 `tile_ready` 是否仍到达 Godot，以及 Godot 是否按 `clip_id/session_id` 丢弃或进入孤儿桶。
4. 连续快速执行 track/clip/rack 操作，记录 `VitDeltaRingBuffer` 是否满、PUB/UDP 是否丢消息、最终全量快照是否收敛。
5. 对比 `add_audio_clip` 与 `import_media_to_track` 两条导入路径：确认是否都能得到 waveform tile、timeline 刷新与全量状态一致。

## G. 修复候选 Backlog

| Tier | 条目 | 触发条件 |
|------|------|----------|
| T0 | 给 delta ring drop、PUB send failure、Bridge UDP send failure 增加计数/日志 | 可直接做，低侵入 |
| T0 | 启动时断言/打印 `VitIpcClient` 方法列表，并生成 IPC 命令全集对照表 | 当前关键风险 |
| T0 | 在 tile consumer 记录 `session_id`、`clip_id`、当前 clip 是否存在 | 可直接做，低侵入 |
| T0 | 给 SHM baker 增加 handle 数量与 generation 日志 | 用于确认是否泄漏 |
| T1 | 补齐或统一 `VitIpcClient` 的 `send_command_async_await` / `send_command_no_wait` / 多参 `send_command_async` 契约 | 已证实调用方与实装不一致 |
| T1 | rack 失败/超时后增加去抖全量回拉 | 证实本地 rollback 不足或 request_id 回调不可靠时 |
| T1 | 删除 clip/track 时显式关闭或淘汰对应 SHM tile handles | 证实现有 handle 会滞留时 |
| T1 | 统一 `add_audio_clip` 与 `import_media_to_track` 的 waveform bake 行为 | 证实 `add_audio_clip` 是仍在使用的前端路径时 |
| T1 | 统一 Godot IPC 为单一 awaitable request/reply API | 避免直发与 pending reply 混用 |
| T2 | 引入 revision/ACK/重放型状态同步 | 仅当确认增量丢失会影响工程真源，且 T1 回拉不足时 |
| T2 | 重构 tile session/handle 管理为集中生命周期服务 | 仅当运行验证证实泄漏或 stale tile 造成用户可见错误时 |

## H. 本次未覆盖 / 需要补证

- 已补查 Godot 前端源码；但未运行 Godot，无法确认实际 Autoload 是否与 `vit_ipc_client.gd` 文件一致。
- 未运行 VitApp、Bridge、Godot，因此 PUB/UDP 丢包、SHM handle 数量、tile 乱序均未运行时验证。
- 未抓包确认 UDP 实际 payload 与 Godot 解析结果。
- 未读取 GDExtension 源码，无法判断 `VitWaveformReader.read_shared_memory` 是否有 unmap/释放 API。
