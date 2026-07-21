# VSP Hub Verification

Date: 2026-07-02

This report records the current evidence that `VspHub.exe` is the formal VSP
desktop protocol hub, separate from VitAgent and the legacy bridge.

## Build And Unit Tests

Commands:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/chat ./internal/bridge ./internal/vsphub ./internal/vspclient ./cmd/vitagent ./cmd/vsphub
go build -o $env:TEMP\VspHub.exe .\cmd\vsphub
go build -o $env:TEMP\VitAgent-vsp-hub.exe .\cmd\vitagent
```

Result:

- Go tests passed.
- `VspHub.exe` built successfully.
- `VitAgent-vsp-hub.exe` built successfully.

## Godot Load

Commands:

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --headless --quit-after 1
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --headless --editor --path D:\Godot\project\vit-daw-frontend --quit-after 1
```

Result:

- Both commands exited with code `0`.
- The plain headless load still reports the existing exit-time ObjectDB/resource
  warning; it did not block project load or VSP probes.

## Live Process Boundary

Live checks used:

- Kernel: `D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`
- Hub: `$env:TEMP\VspHub.exe`
- Agent: `$env:TEMP\VitAgent-vsp-hub.exe`

Observed ports:

- Kernel owned `127.0.0.1:5555`, `5556`, and `5557`.
- `VspHub.exe` owned `127.0.0.1:8787`.
- VitAgent kept its own API on `127.0.0.1:7878`.

Hub health:

```json
{"service":"VspHub","status":"ok","version":"vsp-hub-v1"}
```

Agent registration evidence from `/vsp/status`:

```json
{
  "client_id": "vit.agent.official",
  "role": "agent",
  "transport": "vsp.hub.http"
}
```

This proves VitAgent is now a Hub client, not the owner of the formal `/vsp`
endpoint.

## GUI Through Hub

Environment:

```powershell
$env:VIT_GUI_VSP_LEGACY_FALLBACK_DISABLE='1'
$env:VIT_GUI_VSP_HTTP_URL='http://127.0.0.1:8787/vsp'
```

Commands:

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_http_transport_probe.gd
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_gui_live_probe.gd
```

Results:

- `vsp_http_transport_probe.gd`: `status="ok"`, checks
  `session_hello_over_http` and `state_snapshot_over_http`,
  `track_count=66`, `last_transport="vsp.hub.http"`,
  `legacy_fallback_enabled=false`.
- `vsp_gui_live_probe.gd`: `status="ok"`, `track_count=66`,
  checks included session, state snapshot, state delta, realtime subscribe/frame
  pull/unsubscribe, event subscribe/poll, asset reference, asset adapter, and
  realtime adapter. Transport was `vsp.hub.http` with legacy fallback disabled.

## Extension Through Hub

Command:

```powershell
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp -CheckKernelRoutes
```

Result:

```json
{
  "status": "ok",
  "schema_version": "vsp_hub_extension_smoke.v1",
  "check_kernel_routes": true,
  "checks": [
    "extension_session_hello_ack",
    "extension_register_ack",
    "status_lists_extension",
    "extension_state_snapshot_allowed",
    "extension_command_denied",
    "extension_unregister_ack",
    "status_removes_extension"
  ]
}
```

This proves a third-party extension can use VSP Hub directly, can access an
allowed state route through Hub-to-Kernel, and is denied default command access.

## WebSocket Transport

Commands:

```powershell
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1 -CheckKernelRoutes
```

Results:

- Default WebSocket smoke passed:
  `websocket_extension_register_ack`, `status_lists_websocket_extension`,
  `websocket_extension_unregister_ack`, `status_removes_websocket_extension`.
- Kernel-route WebSocket smoke passed:
  `websocket_session_hello_ack`, `websocket_extension_register_ack`,
  `status_lists_websocket_extension`, `websocket_extension_unregister_ack`,
  `status_removes_websocket_extension`.
- A direct WebSocket run also observed Hub-originated `event.notification`
  messages before the correlated `session.hello_ack`, proving the stream can
  carry both request/reply and event traffic.

## Product Startup Integration

Commands:

```powershell
D:\Vit_DAW\scripts\build_agent.ps1 -OutDir $env:TEMP\vit-agent-hub-build
cmake -S D:\Vit_DAW\Export -B D:\Vit_DAW\Export\build_launcher_v0.93 -A x64
cmake --build D:\Vit_DAW\Export\build_launcher_v0.93 --config Release
D:\Vit_DAW\scripts\assemble_windows_release.ps1 `
  -GodotUiExe "D:\Vit_DAW\Export\Vit DAW v0.93.exe" `
  -OutDir "$env:TEMP\vit-vsp-hub-assemble-smoke" `
  -KernelExe "D:\Vit_DAW\VitApp\build_release\VitApp.exe" `
  -AgentExe "$env:TEMP\vit-agent-hub-build\VitAgent.exe" `
  -VspHubExe "$env:TEMP\vit-agent-hub-build\VspHub.exe" `
  -LauncherExe "D:\Vit_DAW\Export\build_launcher_v0.93\Release\Vit_DAW_Launcher.exe" `
  -GodotUiDependencyDir "D:\Godot\project\vit-daw-frontend" `
  -LayeredPayload -MinimalPayloadOnly -NoPythonBridge -RequireAgent -RequireHub
```

Result:

- `build_agent.ps1` ran `go test ./...` and built both `VitAgent.exe` and
  `VspHub.exe`.
- Launcher CMake configure/build passed and produced
  `Export\build_launcher_v0.93\Release\Vit_DAW_Launcher.exe`.
- Minimal layered assemble passed and produced:
  - `kernel\VitApp.exe`
  - `hub\VspHub.exe`
  - `agent\VitAgent.exe`
  - `ui\Vit DAW v0.93.exe`
  - `Vit DAW.exe`

## Product Path Lifecycle Smoke

Command:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -GodotProjectRoot D:\Godot\project\vit-daw-frontend `
  -GodotExe D:\Godot\Godot_v4.6.1-stable_win64_console.exe `
  -TimeoutSeconds 90
```

Product lifecycle evidence from:

```text
D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260702_231353
```

Result:

- Godot development lifecycle started Kernel, Hub, and Agent.
- `VspHub.exe` ran from `D:\Vit_DAW\agent\bin\VspHub.exe`.
- `VitAgent.exe` ran from `D:\Vit_DAW\agent\bin\VitAgent.exe`.
- Running Hub and Agent binary hashes matched the expected binaries.
- `GET /health` returned `service="VspHub"` and `status="ok"`.
- `GET /vsp/status` advertised `vsp.hub.http` and `vsp.hub.websocket`.
- `/vsp/status` listed:

```json
{
  "client_id": "vit.agent.official",
  "role": "agent",
  "transport": "vsp.hub.http"
}
```

The full product-path smoke continued past lifecycle, read-only observation,
and Chinese multitrack MOM observation, then failed later in an Agent mix
workflow assertion:

```text
Expected completed vocal focus route to include mix.derive unless it is an
explicit no-op/clarification reply. route=mix.observe -> mix_observe
```

That failure is outside the Hub startup and transport boundary, but remains a
follow-up for the mix workflow smoke to become fully green again.

## Dedicated Hub Lifecycle Smoke

Command:

```powershell
D:\Vit_DAW\scripts\run_vsp_hub_lifecycle_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -GodotProjectRoot D:\Godot\project\vit-daw-frontend `
  -GodotExe D:\Godot\Godot_v4.6.1-stable_win64_console.exe `
  -TimeoutSeconds 90
```

Artifact:

```text
D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\vsp_hub_lifecycle_20260703_102936
```

Result:

- Script exited with code `0`.
- Godot startup lifecycle brought up Kernel, Hub, and Agent.
- `VspHub.exe` owned `127.0.0.1:8787` from
  `D:\Vit_DAW\agent\bin\VspHub.exe`.
- `VitAgent.exe` owned `127.0.0.1:7878` from
  `D:\Vit_DAW\agent\bin\VitAgent.exe`.
- Kernel owned `127.0.0.1:5555` and `5556` from
  `D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`.
- Running Hub and Agent binary hashes matched the expected binaries.
- Godot stdout contained:
  - `start_page: VSP Hub ready.`
  - `start_page: Agent v0.5 tools ready.`
- Hub `/health` returned `service="VspHub"` and `status="ok"`.
- Hub `/vsp/status` listed the official Agent session and reported two live
  sessions during the run.
- Follow-up port check found no listeners on `5555`, `5556`, `7878`, or `8787`
  after cleanup.

This is now the preferred Hub-only product infrastructure gate. It intentionally
does not run Ask Vit chat, mix workflow, fixture creation, or LLM-dependent
checks.

## Extension Smoke Without Kernel

Command:

```powershell
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
```

Result:

- HTTP extension smoke passed:
  `extension_register_ack`, `status_lists_extension`,
  `extension_command_denied`, `extension_unregister_ack`,
  `status_removes_extension`.
- WebSocket extension smoke passed:
  `websocket_extension_register_ack`, `status_lists_websocket_extension`,
  `websocket_extension_unregister_ack`, `status_removes_websocket_extension`.

## Cleanup

After live verification, the probe-started Kernel, Hub, and Agent processes were
stopped. Follow-up port checks found no listeners on:

```text
5555, 5556, 5557, 7878, 8787
```
