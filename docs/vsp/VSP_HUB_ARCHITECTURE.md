# VSP Hub Architecture

Date: 2026-07-02

`VspHub.exe` is the Vit desktop VSP protocol center. It is not the legacy bridge
renamed. It is the long-lived message hub for official GUI, future GUI builds,
VitAgent, controllers, analyzers, and third-party extensions.

## Process Boundary

```text
VitApp.exe        Kernel / audio engine
VspHub.exe        VSP protocol hub
VitAgent.exe      Agent / chat / workflow client
Godot GUI         Official GUI client
Third-party ext   Extension client
```

`VspHub.exe` owns the formal VSP endpoints:

```text
http://127.0.0.1:8787/vsp
ws://127.0.0.1:8787/vsp/stream
```

VitAgent keeps its HTTP API on `127.0.0.1:7878`, but no longer owns the formal
VSP endpoint. The legacy UDP bridge remains only as compatibility fallback.

## Code Boundary

```text
D:\Vit_DAW\agent
  cmd\vsphub\main.go
  internal\vsphub\
    config.go
    envelope.go
    session.go
    registry.go
    hub.go
    http.go
    websocket.go
    telemetry.go
    diagnostics.go
    errors.go
```

VitAgent uses:

```text
internal\vspclient
```

to register as `role="agent"` with the Hub.

## Runtime Model

```text
Client -> VspHub -> Kernel
```

The Hub is responsible for:

- envelope validation
- role and session registration
- capability registry
- permission gate
- request/reply routing
- WebSocket stream transport
- extension registration
- Kernel ZMQ binding
- diagnostics and status

The Kernel remains the authoritative executor for command/state behavior. The
Hub owns the realtime fan-out/cache lane and the asset reference/manifest entry
point:

- `command`: request/reply traffic is routed through Hub permission checks to
  Kernel.
- `realtime`: Kernel or Agent publishes `realtime.publish` to Hub; GUI and
  extensions subscribe to Hub cache and use cache-only frame reads. Hub filters
  visible-track streams by subscription `track_ids` before returning cached
  frames.
- `asset`: clients can request a single `asset.reference` or a visible-priority
  `asset.manifest`. Large waveform/spectrum data stays outside JSON and is
  addressed by platform-neutral URIs such as `vit-cache://`.

## State Scope Contract

`project.timeline` is the user-visible timeline scope. It must not expose
Tracktion kernel-internal timeline tracks. The current internal IDs are:

```text
1002 Arranger
1003 Chord
1004 Marker
1005 Tempo
1006 Master
```

User-created timeline tracks start at `1007`. Third-party clients reading
`project.timeline` should see user timeline tracks only. If future tools need
raw kernel topology, that must be added as a separate explicit scope instead of
changing `project.timeline` semantics.

## Product Startup Model

The official desktop product starts the VSP runtime as a process chain, not as
ad hoc bridge scripts:

```text
Vit DAW.exe launcher
  -> kernel\VitApp.exe
  -> hub\VspHub.exe
  -> agent\VitAgent.exe
  -> ui\Vit DAW*.exe
```

Godot development startup follows the same logical order:

```text
Godot start_page.gd
  -> VitApp.exe when Kernel IPC is absent
  -> VspHub.exe when 127.0.0.1:8787/health is absent
  -> VitAgent.exe when 127.0.0.1:7878/agent/tools is absent
```

The packaged layered release owns this layout:

```text
Vit DAW.exe
kernel\VitApp.exe
hub\VspHub.exe
agent\VitAgent.exe
ui\Vit DAW*.exe
```

`VspHub.exe` becomes ready when `GET http://127.0.0.1:8787/health` returns
`service="VspHub"` and `status="ok"`. Product smoke tests also require
`GET http://127.0.0.1:8787/vsp/status` to list the official Agent session:

```text
client_id="vit.agent.official"
role="agent"
transport="vsp.hub.http"
```

## Stable Principle

External clients should depend on VSP and the Hub endpoint, not on Kernel ZMQ,
Godot UDP, VitAgent chat routes, or Python bridge scripts.
