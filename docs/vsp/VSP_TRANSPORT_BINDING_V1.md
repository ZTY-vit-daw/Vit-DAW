# VSP Transport Binding v1

Date: 2026-07-02

This note records the VSP-native transport binding after the VSP v1 foundation
closeout. It supersedes the temporary VitAgent-hosted `/vsp` binding: the formal
VSP endpoint is now owned by `VspHub.exe`.

## Status

- VSP is already the protocol envelope used by Kernel, GUI, and Agent flows.
- The new default GUI path is now VSP over local HTTP through independent
  `VspHub.exe`:
  `Godot VspClient -> POST http://127.0.0.1:8787/vsp -> VspHub -> Kernel ZMQ REQ`.
- The old Godot UDP command bridge remains as an explicit compatibility
  fallback, not as the default VSP envelope path:
  `Godot VitIpcClient UDP 4445 -> VitAgent bridge -> Kernel ZMQ REQ`.
- VitAgent no longer owns the formal `/vsp` endpoint. It registers with
  `VspHub.exe` as `role="agent"`.
- This is not a new Python script bridge. Python bridge code is historical or
  packaging fallback only.

## Hub HTTP Binding

`VspHub.exe` exposes:


```text
POST /vsp
Content-Type: application/json
```

Request body must be a VSP envelope JSON object with at least:

- `vsp_version`
- `channel`
- `type`

VspHub does light validation only, then forwards the original JSON bytes to
Kernel through `kernel.Client.SendRaw`. Successful Kernel replies are returned
as raw JSON without wrapping, so the VSP reply shape is preserved.

Gateway failures are returned as HTTP errors with JSON bodies:

- `400`: malformed envelope or unreadable request body
- `405`: non-POST method
- `502` / `504`: Kernel gateway failure or timeout

## Godot Binding

`app/kernel/clients/vsp_client.gd` now prefers HTTP by default.

Runtime controls:

- `VIT_GUI_VSP_HTTP_URL`: override the default `http://127.0.0.1:8787/vsp`
- New default: `http://127.0.0.1:8787/vsp`
- `VIT_GUI_VSP_HTTP_DISABLE=1`: disable the HTTP binding
- `VIT_GUI_VSP_LEGACY_FALLBACK_ENABLE=1`: explicitly allow legacy UDP fallback
  for VSP envelopes after the Hub path fails
- `VIT_GUI_VSP_LEGACY_FALLBACK_DISABLE=1`: force-disable UDP fallback even if a
  caller enables it in code

Diagnostics:

- `VspClient.last_transport`
- `VspClient.get_transport_status()`

Expected HTTP value:

```text
vsp.hub.http
```

Expected legacy fallback value when explicitly enabled:

```text
legacy.godot_udp_bridge
```

## Verification

Commands run on 2026-07-02:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/chat ./internal/bridge ./internal/vsphub ./internal/vspclient ./cmd/vitagent ./cmd/vsphub
go build -o $env:TEMP\VspHub.exe .\cmd\vsphub
go build -o $env:TEMP\VitAgent-vsp-hub.exe .\cmd\vitagent

D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --headless --quit-after 1
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --headless --editor --path D:\Godot\project\vit-daw-frontend --quit-after 1
```

Live verification uses Release `VitApp.exe` plus the newly built `VspHub.exe`:

```powershell
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_http_transport_probe.gd
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend --script res://tools/diagnostics/vsp_gui_live_probe.gd
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp -CheckKernelRoutes
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1 -CheckKernelRoutes
```

Results:

- `vsp_http_transport_probe.gd`: passed with
  `last_transport="vsp.hub.http"`, `legacy_fallback_enabled=false`,
  `track_count=66`.
- `vsp_gui_live_probe.gd`: passed with `status="ok"`, `track_count=66`,
  and `last_transport="vsp.hub.http"`.
- VitAgent registered in `/vsp/status` as `client_id="vit.agent.official"`,
  `role="agent"`, `transport="vsp.hub.http"`.
- Extension smoke passed register/status/allowed state snapshot/default command
  denial/unregister through the Hub.
- WebSocket smoke passed extension register/status/unregister and, with Kernel
  online, `session.hello_ack` with `transport_binding="vsp.hub.websocket"`.
- After cleanup, ports `5555`, `5556`, `5557`, `7878`, and `8787` were not
  occupied by the probe processes.

Detailed evidence is recorded in `VSP_HUB_VERIFICATION_2026_07_02.md`.

## Platform Notes

This binding is portable to macOS because it uses loopback HTTP and the same
VSP JSON envelope. The platform-specific parts are process launch, packaging,
firewall/notarization rules, and eventual local IPC hardening. The protocol
contract can stay the same across Windows and macOS.

Future dedicated transports can replace HTTP with WebSocket, local TCP, Unix
domain socket, named pipe, or shared-memory paths without changing the VSP
envelope contract.
