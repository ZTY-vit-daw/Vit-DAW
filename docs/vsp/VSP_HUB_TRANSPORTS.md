# VSP Hub Transports

Date: 2026-07-02

## HTTP

Request/reply traffic uses:

```text
POST http://127.0.0.1:8787/vsp
```

Transport binding name:

```text
vsp.hub.http
```

The request body is one VSP envelope. The response body is one VSP envelope.

## WebSocket

Streaming and duplex traffic uses:

```text
ws://127.0.0.1:8787/vsp/stream
```

Transport binding name:

```text
vsp.hub.websocket
```

Each WebSocket text message is one VSP envelope. The Hub dispatches incoming
envelopes through the same router as HTTP and returns one response envelope per
request. Kernel telemetry relay is exposed as Hub-originated
`event.notification` messages only after that WebSocket session has completed
`event.subscribe`; realtime-only clients are not sent event-lane telemetry.

Smoke:

```powershell
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1 -CheckKernelRoutes
```

## Diagnostics

```text
GET http://127.0.0.1:8787/health
GET http://127.0.0.1:8787/vsp/status
```

`/health` reports Hub identity, version, pid, uptime, Kernel binding
configuration, transport bindings, and request/error counters.

`/vsp/status` reports the same Hub diagnostics plus sessions, stream count,
capabilities by role, and Kernel binding endpoints.

## Environment

Hub:

```text
VIT_VSP_HUB_ADDR=127.0.0.1:8787
VIT_VSP_HUB_KERNEL_REQ_URL=tcp://127.0.0.1:5555
VIT_VSP_HUB_KERNEL_SUB_URL=tcp://127.0.0.1:5556
VIT_VSP_HUB_ENABLE_TELEMETRY=1
```

GUI:

```text
VIT_GUI_VSP_HTTP_URL=http://127.0.0.1:8787/vsp
VIT_GUI_VSP_HTTP_DISABLE=1
VIT_GUI_VSP_LEGACY_FALLBACK_DISABLE=1
```

Agent:

```text
VIT_AGENT_VSP_HUB_URL=http://127.0.0.1:8787/vsp
VIT_AGENT_VSP_HUB_REQUIRED=0
```
