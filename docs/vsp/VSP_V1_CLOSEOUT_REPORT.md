# VSP v1 Closeout Report

本文记录 VSP v1 Foundation 收口阶段的当前证据、变更、最终总验收和风险。日期：2026-07-02。

## 1. 当前收口结论

- Phase 0 到 Phase 6 的核心实现已落地，并在 2026-07-02 使用当前 Release 构建完成最终总验收。
- GUI 已具备 VSP state delta、realtime、asset channel 接入，Agent 已具备 VSP observe/execute/reobserve、import progress、plugin batch 和 macro control 入口。
- Godot F5 真实启动路径已复跑验证，`application/run/main_scene` 为 `res://app/startup/start_page.tscn`，可进入 `VitDockRoot`；直接 Godot 窗口启动也能创建响应中的顶层窗口并正常关闭。
- 66 轨 GUI 压力验证已复跑通过，track virtualization 生效，滚动、levels、spectrum、waveform、输入命中和 clip interaction 均未出现持续慢帧失控。
- VSP live probe 已复跑通过，确认 session/state/realtime/event/asset 和 GUI autoload adapter 可用。
- Agent/plugin 收口缺口已补齐：`plugin.set_params_batch`、`command.batch`、`macro.create`、`macro.bind`、`macro.set_values` 已进入 Kernel VSP reference adapter，并有专项 smoke 覆盖。
- 旧 UDP telemetry 已降级为兼容入口：当前 Go VitAgent bridge 会将大于 32KB 的 legacy telemetry 落文件，并向 Godot 发送 `transport=file_reply` envelope。

## 2. Phase 6 验收证据

### F5 startup smoke

命令形态：

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/f5_startup_smoke.gd
```

关键结果：

- D3D12 Forward+ renderer 启动。
- `configured_main_scene=res://app/startup/start_page.tscn`。
- `workspace_entered scene=VitDockRoot`。
- 本轮复跑：`frames=240`，`workspace_entered scene=VitDockRoot elapsed_ms=350.87`，`max_frame_ms=330.419`，`slow_frames=1`，退出码为 `0`。

### Direct F5-style window start

命令形态：

```powershell
D:\Godot\Godot_v4.6.1-stable_win64.exe --path D:\Godot\project\vit-daw-frontend
```

关键结果：

- 本轮复跑：非 console Godot 进程出现可见顶层窗口句柄 `26675588`。
- 进程 `Responding=True`。
- 通过正常窗口关闭路径退出。

### 66-track GUI bulk probe

命令形态：

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/bulk_track_gui_probe.gd
```

关键配置：

- `VIT_GUI_PROBE_USE_START_PAGE=1`
- `VIT_GUI_PROBE_TRACKS=66`
- `VIT_GUI_PROBE_SCROLL=1`
- `VIT_GUI_PROBE_LEVELS=1`
- `VIT_GUI_PROBE_SPECTRUM_ROWS=17`
- `VIT_GUI_PROBE_WAVEFORM=1`
- `VIT_GUI_PROBE_WARM_REQUESTS=32`

关键结果：

- `start_page_entry scene=VitDockRoot`。
- summary `slow_frames=0`。
- 本轮复跑 summary：`apply_ms=147.95`，`avg_frame_ms=7.44`，`max_frame_ms=21.51`，`slow_frames=0`。
- track virtualization：`virtual_active=true`，`virtual_total=66`，`virtual_pool=11`，`virtual_visible=8`。
- spectrum：请求 17 行，实际启用 6 条当前可见轨，`bus_visible_count=6`。
- levels：`slow_frames=0`，`bus_visible=6`，`bus_cached_visible=5`。
- waveform：`available=true`，`slow_frames=0`，`paced_slow_frames=0`。
- clip interaction：`ok=true`，`moved=true`。
- input hit probes：5 个 scroll depth 采样均完成，worst non-lane hit 平均约 `0.041 ms`。

### VSP GUI live probe

命令形态：

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_gui_live_probe.gd
```

关键结果：

- `status="ok"`。
- `track_count=66`。
- 本轮复跑：`snapshot_revision=13`，`delta_revision=13`，`realtime_frame_index=2`。
- checks 包含：
  - `session_hello_gui_vsp`
  - `state_snapshot_project_timeline`
  - `state_delta_no_op_or_incremental`
  - `realtime_subscribe_transport_playhead`
  - `realtime_frame_pull`
  - `realtime_unsubscribe`
  - `event_subscribe_topics`
  - `event_poll_progress_or_noop`
  - `asset_reference_waveform_peak`
  - `vsp_asset_adapter_autoload`
  - `vsp_realtime_adapter_autoload`
- asset reference 使用 `vit-cache://project_current/clips/1011/waveform/peaks/v1`。
- realtime adapter visible track count 为 `2`。

## 3. Legacy telemetry 降级验证

### 背景

Phase 6 验收中发现 legacy telemetry 仍可能产生超过 UDP 单包上限的大 JSON。当前生产/开发桥接主路径是 Go VitAgent：Godot command UDP `4445` -> VitAgent -> Kernel ZMQ `5555`，Kernel telemetry ZMQ `5556` -> VitAgent -> Godot UDP `4444`。因此收口证据必须以 `agent/internal/bridge/bridge.go` / `agent/cmd/vitagent` 为准；Python `scripts/bridge_core.py` 仅作为历史/打包 fallback 参考。

### 变更

文件：

- `D:\Vit_DAW\agent\internal\bridge\bridge.go`
- `D:\Vit_DAW\agent\cmd\vitagent\main.go`

行为：

- 控制应答仍保留原有 `file_reply` fallback。
- legacy telemetry 使用 `maxDirectUDPTelemetryBytes = 32 * 1024`。
- 超过上限的 telemetry packet 写入 VitAgent `FileReplyDir`。
- UDP 只发送小 envelope：

```json
{
  "transport": "file_reply",
  "reply_file": "...",
  "telemetry_file": "...",
  "reply_bytes": 32821,
  "telemetry_bytes": 32821,
  "topic": "project"
}
```

- 日志只在首包和每 100 包记录一次 `large packet spilled to file`，避免 verbose 模式刷屏。

### 验证

小范围验证：

- `go test ./internal/bridge ./cmd/vitagent` 通过。
- `go build -o %TEMP%\VitAgent-vsp-closeout.exe .\cmd\vitagent` 通过。
- `agent/internal/bridge/bridge.go` 中 `telemetryPayload()` / `spillTelemetryPacket()` 确认：
  - `transport=file_reply`
  - `telemetry_file` 存在
  - `reply_file == telemetry_file`
  - 原始 telemetry bytes 写入成功。

真实链路验证：

- release `VitApp.exe` + 当前 Go `VitAgent` 启动成功。
- `vsp_gui_live_probe.gd` 退出码 `0` 且 `status="ok"`。
- VitAgent 日志中未匹配到 `UDP send failed`、`WinError`、`Traceback`、`panic` 或 error 关键字。
- VitAgent 日志中出现 `telemetry spilled to file label=levels bytes=74876 ...`，说明大包被 Go bridge 兼容 fallback 接住。
- 测试结束后 `5555/5556/5557/4444/4445` 端口清空。

## 4. Agent plugin/macro/batch 收口

### 背景

最终审计时发现 Phase 5 原 smoke 已覆盖 observe/execute/reobserve/import/progress，但没有直接证明以下 VSP v1 承诺：

- `plugin.set_params_batch` 一次 VSP command 写多个参数并返回 changed params。
- `command.batch` 保持子命令顺序并返回每个子结果。
- `macro.create`、`macro.bind`、`macro.set_values` 走 canonical VSP 命令，而不是只能通过 legacy command。

### 变更

文件：

- `D:\Vit_DAW\VitApp\Source\Service\VspKernelReference.cpp`
- `D:\Vit_DAW\VitApp\Source\Service\PluginRackControlService.cpp`
- `D:\Vit_DAW\scripts\vsp_phase5_plugin_macro_smoke.py`

行为：

- session hello 增加 `command.batch` feature flag 和 capability。
- canonical command map 增加：
  - `plugin.describe_parameters` -> `get_plugin_parameters`
  - `plugin.readback` -> `get_plugin_parameters`
  - `macro.create` -> `control_add_macro`
  - `macro.bind` -> `control_add_binding`
  - `macro.set_values` -> `control_set_macro_values`
- `plugin.set_params_batch` 由 VSP adapter 顺序执行多个 legacy `set_plugin_param`，保持一次 VSP command 的 request/transaction 边界，返回 `changed_params`、每参数 result、actual normalized value 和 display text。
- legacy `set_plugin_param` 增加 `normalized_value`/`normalised_value` 输入，直接使用 Tracktion `setNormalisedParameter()`，并回传 `actual_normalized_value`、`new_normalised_value`、`display_text`。
- `command.batch` 顺序执行子命令，返回每个子 command response；子命令失败时停止后续执行并返回 `partial_failure`。

### 验证

Release build：

```powershell
cmake --build D:\Vit_DAW\VitApp\build_release --target VitApp --config Release
```

结果：

- 构建通过。
- 输出：`D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`。
- 仅出现既有 third-party/header 编码 warning `C4819`。

专项 smoke：

```powershell
python D:\Vit_DAW\scripts\vsp_phase5_plugin_macro_smoke.py --timeout-ms 120000
```

结果：

- 退出码 `0`。
- checks：
  - `legacy_project_plugins_state_before`
  - `session_hello_batch_plugin_flags`
  - `track_create_for_plugin_macro`
  - `stable_volume_plugin_instance_id`
  - `plugin_set_params_batch_changed_params_readback`
  - `command_batch_ordered_plugin_child_results`
  - `macro_create_stable_id`
  - `macro_bind_target_verified`
  - `macro_set_values_binding_propagated`

Phase 2-5 reference smoke 复跑：

```powershell
python D:\Vit_DAW\scripts\vsp_phase2_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase3_state_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase4_realtime_asset_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase5_agent_smoke.py --timeout-ms 120000
```

结果：

- Phase 2：5 checks，通过。
- Phase 3：16 checks，通过。
- Phase 4：13 checks，通过。
- Phase 5：12 checks，通过。

## 5. 最终总验收证据

本轮最终总验收使用当前工作树、当前 Release 构建和新启动的 kernel 进程完成。

### Build and script checks

```powershell
cmake --build D:\Vit_DAW\VitApp\build_release --target VitApp --config Release
python -m py_compile D:\Vit_DAW\scripts\vsp_phase2_smoke.py D:\Vit_DAW\scripts\vsp_phase3_state_smoke.py D:\Vit_DAW\scripts\vsp_phase4_realtime_asset_smoke.py D:\Vit_DAW\scripts\vsp_phase5_agent_smoke.py D:\Vit_DAW\scripts\vsp_phase5_plugin_macro_smoke.py
cd D:\Vit_DAW\agent
go test ./internal/bridge ./cmd/vitagent
go build -o %TEMP%\VitAgent-vsp-closeout.exe .\cmd\vitagent
```

结果：

- Release 构建通过，输出 `D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`。
- Python smoke 脚本编译通过。
- Go bridge/VitAgent 测试和构建通过。
- 新启动的 Release kernel 进程监听 `127.0.0.1:5555`、`5556`、`5557`。

### Kernel reference smokes

```powershell
python D:\Vit_DAW\scripts\vsp_phase2_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase3_state_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase4_realtime_asset_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase5_agent_smoke.py --timeout-ms 120000
python D:\Vit_DAW\scripts\vsp_phase5_plugin_macro_smoke.py --timeout-ms 120000
```

结果：

- Phase 2：5 checks，通过；覆盖 legacy direct ping/list、session hello、legacy command wrapper、canonical snapshot。
- Phase 3：16 checks，通过；覆盖 snapshot、delta、revision gap、resync、track/clip add/move/remove。
- Phase 4：13 checks，通过；覆盖 visible realtime subscription、latest frame、unsubscribe、asset reference、event subscribe/poll。
- Phase 5 Agent：12 checks，通过；覆盖 compact observe、execute/reobserve、import ack/readback、progress event、legacy compatibility。
- Phase 5 Plugin/Macro：9 checks，通过；覆盖 plugin batch、command batch、macro create/bind/set values 与 binding propagation。

### Godot project and adapter probes

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --headless --quit-after 1
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --headless --editor --path D:\Godot\project\vit-daw-frontend --quit-after 1
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_project_store_delta_probe.gd
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_realtime_adapter_probe.gd
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_asset_adapter_probe.gd
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/telemetry_file_reply_probe.gd
```

结果：

- Headless project load 和 editor headless load 均退出码 `0`。Godot 退出阶段仍有既有 ObjectDB/resource warning；本轮未视为功能失败。
- `vsp_project_store_delta_probe.gd`：`{"checked":"vsp_project_store_delta_merge","revision":2,"status":"ok","track_count":2}`。
- `vsp_realtime_adapter_probe.gd`：`{"checked":"vsp_realtime_adapter","frame_calls":1,"status":"ok","subscribe_calls":1,"visible_track_count":2}`。
- `vsp_asset_adapter_probe.gd`：`{"cached_kind":"waveform_peak","calls":1,"checked":"vsp_asset_adapter","status":"ok","telemetry_cached":true}`。
- `telemetry_file_reply_probe.gd`：`[VitTelemetryFileReplyProbe] ok topic=project tracks=1 level_index=yes`。
- 本轮顺序执行 adapter probes，未再出现并发运行导致的 UDP `4444` bind warning。

### Live GUI bridge path

```powershell
%TEMP%\VitAgent-vsp-closeout.exe -verbose -last-log-path %TEMP%\vitagent_vsp_closeout.log -file-reply-dir %TEMP%\vitagent_vsp_closeout_replies
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_gui_live_probe.gd
```

结果：

- Go VitAgent 监听 UDP `127.0.0.1:4445` 和 HTTP `127.0.0.1:7878`，通过当前 Release kernel 执行 VSP live probe。
- `vsp_gui_live_probe.gd` 退出码 `0`，`status="ok"`，`track_count=66`，`snapshot_revision=1`，`realtime_frame_index=1`。
- checks 覆盖 session/state/realtime/event/asset 以及 `VspAssetAdapter`、`VspRealtimeAdapter` autoload。
- VitAgent 日志未匹配到 `UDP send failed`、`WinError`、`Traceback`、`panic` 或 error 关键字；日志中出现 `telemetry spilled to file label=levels bytes=74876`。

## 6. Requirement-by-requirement 最终审计

| 完成定义 | 当前证据 | 结论 |
| --- | --- | --- |
| 1. 文档完整：角色、通道、传输、状态、实时、资产、命令、权限、插件、迁移、测试都有正式说明 | `VSP_V1_FOUNDATION_SPEC.md` 覆盖角色、通道、状态、实时、资产、命令、插件、权限、迁移和成功标准；`VSP_PHASE1_SCHEMA_AND_CHANNELS.md` 覆盖 envelope、transport binding、capability、error、command/state/realtime/asset/event/session、legacy adapter、SDK/test plan；`VSP_PHASE0_AUDIT.md` 覆盖现状矩阵和旧 IPC 边界；`VSP_V1_CONFORMANCE_AND_SMOKE.md` 覆盖 conformance/smoke。 | 通过 |
| 2. Kernel reference implementation 可运行 | Release build 通过；新启动 `VitApp.exe` 监听 `5555/5556/5557`；Phase 2-5 smoke 全部通过。 | 通过 |
| 3. GUI 和 Agent 至少完成核心通道迁移，而不是只保留旧桥接 | Godot `vsp_client.gd`、`project_store.gd`、`vsp_realtime_adapter.gd`、`vsp_asset_adapter.gd` 和 autoload 注册接入 VSP state/realtime/asset；Agent smoke 覆盖 compact observe、execute/reobserve、import progress；live GUI probe 通过。 | 通过 |
| 4. 旧 IPC 兼容层存在，且有迁移说明 | `VSP_PHASE0_AUDIT.md` 和 `VSP_PHASE1_SCHEMA_AND_CHANNELS.md` 记录 legacy adapter/UDP/ZMQ 边界；Phase 2/3/4/5 smoke 均包含 legacy compatibility checks；Go `agent/internal/bridge` file reply fallback 通过测试和 live probe。 | 通过 |
| 5. Conformance tests 通过 | 本轮按 `VSP_V1_CONFORMANCE_AND_SMOKE.md` 复跑 Phase 2-5 reference smokes、Godot adapter probes、live GUI probe、F5 startup、66 轨 GUI bulk probe。 | 通过 |
| 6. Smoke tests 通过 | Release build、Python compile、kernel smokes、Godot headless/editor load、adapter probes、file reply probe、live GUI probe、F5 startup、66 轨 bulk probe均退出码 `0`。 | 通过 |
| 7. 手测确认多轨导入、滚动、播放、meters、spectrum、Agent 命令和插件 batch control 主要链路 | 66 轨 bulk probe 覆盖 start page 进入工作区、虚拟化、滚动、levels、spectrum、waveform、input、clip interaction；F5 startup 和直接窗口启动通过；Phase 5 Agent 和 plugin/macro smoke 覆盖 Agent 命令、plugin batch、macro control。 | 通过 |

## 7. 旧 IPC/UDP 兼容边界

仍保留的兼容路径：

- Kernel 继续保留旧 JSON command handler；`VspKernelReference` 可通过 `legacy.command` 和 canonical command map 调用旧 `CommandDispatcher` handler。
- 当前 reference transport 仍使用本地 ZMQ/UDP bridge 作为 binding：Kernel `5555/5556/5557`、Go VitAgent bridge `4445/4444`，Ask Vit HTTP `7878`。
- `get_project_state`、`import_audio`、`set_plugin_param`、`get_plugin_parameters`、track/clip/macro legacy command 仍可用于迁移期兼容。
- 旧 command reply 和 telemetry 的超大 UDP payload 通过 `file_reply` 兼容降级，不作为 VSP 核心 payload 语义。

已切到 VSP 语义的核心路径：

- Session/capability/feature flag 通过 `session.hello`。
- Command 使用 VSP envelope、request id、transaction/revision/resync hint；`plugin.set_params_batch` 与 `command.batch` 为 VSP reference adapter 原生命令。
- State 使用 `state.snapshot` + `state.delta` + `state.resync_required/request`。
- Realtime 使用 visible-track/latest-only subscription 和 frame request。
- Asset 使用 `asset.reference` 与平台中立 `vit-cache://` URI，不推送 waveform/spectrum 大数组。
- Event 使用 `event.subscribe` / `event.poll` 表达 import/bake/render progress。

## 8. 当前风险

- 旧 telemetry file fallback 是兼容层兜底，不应重新成为 GUI 主干路径。GUI 的 playhead/meters/spectrum/waveform peak 仍应优先走 VSP realtime/asset。
- 如果 legacy 端持续产生大 telemetry，fallback 会产生临时文件 IO。收口后可继续减少旧 telemetry 生产频率或在 bridge 侧按 topic 做更细粒度降级。
- Godot headless 退出阶段仍有既有 ObjectDB/resource warning；当前 smoke 退出码为 `0`，未显示 VSP 功能失败。
- F5 startup smoke 本轮有 1 个启动期慢帧；66 轨持续负载 probe 的 scroll/levels/waveform/input 均为 `slow_frames=0`。
