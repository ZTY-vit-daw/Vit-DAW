# Vit 内核服务边界审计

日期：2026-05-17

## 结论

本轮内核结构整理已完成第一阶段目标：`CommandDispatcher` 不再承载大块业务命令实现，主要保留 IPC 协议入口、命令注册/转发、渲染中 allowlist、全量状态快照、少量全局命令。

已经抽出的服务：

| Service | 负责命令/职责 |
|---|---|
| `ProjectService` | `reload_project`、`new_project`、`open_project` / `load_project`、`save_project`、`save_as_project`、`get_recent_projects` |
| `ImportService` | `import_audio`、`import_media_to_track`、`add_audio_clip`、`warm_waveform_bake`，并提供生成资产插入复用 API |
| `GeneratedAssetService` | `bridge_ingest_generated_asset`、`switch_asset_take`、`set_async_ghost_state` |
| `JobEventService` | `aigc_register_job` |
| `TrackService` | `list_tracks`、`append_ghost_track`、`add_track` / `add_audio_track`、`delete_track`、`set_volume`、`set_mute` |
| `ClipService` | `move_clip`、`clone_clip`、`resize_clip`、`remove_clips` |
| `MidiService` | `add_midi_notes`、`add_midi_notes_bulk`、`mutate_midi_notes`、`delete_midi_notes`、`get_midi_clip_notes`、`get_midi_clip_data`、`insert_midi_clip` / `create_midi_clip` |
| `TransportAudioService` | `play`、`stop`、`return_to_zero`、`transport_option_stop_return_to_start`、click、seek、audio device、wave input routing、record、freeze、render |
| `PluginRackControlService` | `scan_plugins`、`instantiate_plugin`、`open_plugin_ui` / `show_plugin_editor`、`get_plugin_parameters`、`delete_plugin`、`move_plugin`、`set_plugin_param`、`set_plugin_param_aliases`、`rack_*`、`control_*`、`connector_*` |

## `CommandDispatcher` 当前保留职责

`CommandDispatcher` 当前保留这些本地 handler：

- `ping`
- `get_project_state`
- `set_tempo`
- `project_health_check`
- `clear_project`
- `undo`
- `redo`

保留原因：

- `dispatch()` 需要继续兼容 `cmd` / `action` / `command` 三种字段。
- 渲染中 allowlist 仍是入口层职责。
- `get_project_state` 是跨服务的聚合快照，当前仍在 dispatcher 内统一组装 `tracks`、`rack`、`control_graph`、connector、job、observability、graph revision 等状态面。
- `project_health_check` 与 `get_project_state` 共用 project file 与 graph revision 快照 helper。
- `clear_project` 是全局清理命令，会触及 transport、clips、spectrogram mapping，暂未归入某个业务 service。

## 审计结果

- `CommandDispatcher.h` 只声明入口层 handler 与各 service 的 `unique_ptr`。
- 所有原业务命令仍在 `registerBuiltinCommands()` 中注册原命令名；IPC JSON 字段、命令名、响应字段未刻意改动。
- `PluginRackControlService` 是当前最后一组高风险区域的合并服务。暂不继续拆为 `PluginHostService` / `RackGraphService` / `SmartControlService` / `ConnectorService`，以减少 mock 演示版与 agent 端开发前的额外风险。
- `get_project_state` 相关 rack/control snapshot helper 仍留在 `CommandDispatcher.cpp`。这是当前刻意保留的快照聚合边界；未来若再整理，可单独抽 `ProjectSnapshotService`。

## 验证基线

每次改动后至少跑：

```powershell
cmake --build D:/Vit_DAW/VitApp/build_release --config Release --target VitApp
& 'C:\Users\timoz\AppData\Local\Programs\Python\Python313\python.exe' D:/Vit_DAW/scripts/check_ipc_contract.py
```

本轮最终结果：

- Release build 通过。
- IPC contract check 通过：`Missing C++ handlers for Godot static commands: none`。
- 用户已完成重启内核手测，Import、Project、Generated Asset/Job、Track/Clip/MIDI、Transport/Audio/Render、Rack/Plugin/Control/Connector 均未发现问题。

## 提交点建议

建议把这一轮作为一个独立提交点，提交主题可用：

```text
refactor: split command dispatcher into service modules
```

提交范围建议只包含：

- `VitApp/CMakeLists.txt`
- `VitApp/Source/Service/CommandDispatcher.cpp`
- `VitApp/Source/Service/CommandDispatcher.h`
- `VitApp/Source/Service/*Service.cpp`
- `VitApp/Source/Service/*Service.h`
- `docs/VIT_IPC_CONTRACT.md`
- `docs/VIT_KERNEL_SERVICE_BOUNDARY.md`
- `docs/Vit-DAW 项目功能总览.md`

不要混入当前 worktree 中已有的 workspace 配置、脚本、release 产物、音频文件或子模块状态，除非另有明确目的。