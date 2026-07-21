# VSP Hub Extension API

Date: 2026-07-02

Third-party extensions connect to `VspHub.exe` as `role="extension"`.

## Hello

Extensions first send `session.hello`:

```json
{
  "vsp_version": "1.0",
  "schema": "vsp.session.hello.v1",
  "message_id": "msg_ext_1",
  "session_id": "session_pending",
  "client_id": "com.example.extension",
  "role": "extension",
  "channel": "session",
  "type": "session.hello",
  "payload": {
    "client_name": "Example Extension",
    "client_version": "0.1.0",
    "wants": ["state.snapshot", "event.subscribe"],
    "transport_bindings": ["vsp.hub.http", "vsp.hub.websocket"]
  }
}
```

The Hub records the client and returns the Kernel session acknowledgement with
Hub metadata.

## Register

Extensions may then send:

```text
channel=extension
type=extension.register
```

This is handled locally by the Hub and returns `extension.register_ack`.

## Unregister

Extensions may disconnect cleanly with:

```text
channel=extension
type=extension.unregister
```

The Hub removes the registered session locally and returns
`extension.unregister_ack`.

## Smoke Test

With `VspHub.exe` running:

```powershell
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp -CheckKernelRoutes
```

The smoke verifies register, `/vsp/status`, default command denial, and
unregister without going through VitAgent or the legacy bridge. With
`-CheckKernelRoutes`, it also verifies extension `session.hello` and an allowed
`state.snapshot` request through the Hub-to-Kernel route.

## Codex Read-Only Probe

Codex can connect as an ordinary third-party extension. It must identify itself
with its own stable client id instead of pretending to be VitAgent:

```text
client_id=codex.agent.local
role=extension
```

The read-only probe is:

```powershell
D:\Vit_DAW\scripts\codex_vsp_readonly_probe.ps1 -HubUrl http://127.0.0.1:8787/vsp
```

It verifies `/health`, `/vsp/status`, `session.hello`, `extension.register`,
allowed `state.snapshot_request`, allowed `event.poll`, and
`extension.unregister`. The probe does not send `command` channel traffic.
Use `-HubOnly` only when diagnosing the Hub without a live Kernel route.

## Default Permissions

Initial extension permissions are intentionally conservative:

- `session.hello`
- `session.heartbeat`
- `extension.register`
- `extension.unregister`
- `state.snapshot`
- `state.delta`
- `state.resync`
- `state.subscribe`
- `asset.request`
- `event.subscribe`
- `event.poll`

`command.request` is denied by default for `role="extension"` until a manifest,
signature, or explicit user grant exists.
