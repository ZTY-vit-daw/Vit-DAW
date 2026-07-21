# VSP Migration from Legacy Bridge

Date: 2026-07-02

## Current Target

Formal VSP traffic goes through:

```text
Client -> VspHub.exe -> Kernel
```

The current default GUI endpoint is:

```text
http://127.0.0.1:8787/vsp
```

## Legacy Components

Still present for compatibility:

- VitAgent UDP command bridge on `4445`
- VitAgent UDP telemetry path on `4444`
- file reply fallback for oversized legacy packets
- historical Python bridge scripts

These are no longer the VSP center.

## VitAgent Role

VitAgent keeps:

- chat API
- workflow execution
- artifact/project tooling
- legacy bridge compatibility during migration

VitAgent now registers with VspHub as:

```json
{
  "client_id": "vit.agent.official",
  "role": "agent"
}
```

## GUI Role

Godot `VspClient` defaults to `VspHub.exe`. Legacy UDP fallback for VSP
envelopes is opt-in via `VIT_GUI_VSP_LEGACY_FALLBACK_ENABLE=1` or a deliberate
client setting, and only runs after the Hub path fails.

## Migration Completion Criteria

The bridge migration is complete when:

- GUI session/state/command/realtime/asset/event pass with legacy fallback
  disabled.
- VitAgent registers with VspHub and routes Agent VSP actions through it.
- Third-party mock extension can hello, register, and request allowed state/event
  capabilities.
- Kernel telemetry needed by GUI can be consumed through VSP WebSocket/event
  routes instead of UDP.
- Legacy bridge can be disabled without breaking the main VSP smoke suite.

## Current Evidence

As of 2026-07-02:

- GUI live probes pass with `VIT_GUI_VSP_LEGACY_FALLBACK_DISABLE=1` and
  `last_transport="vsp.hub.http"`.
- VitAgent registers with `VspHub.exe` as `client_id="vit.agent.official"` and
  `role="agent"`.
- Extension smoke passes register, status listing, allowed `state.snapshot`,
  default `command.request` denial, and unregister.
- WebSocket smoke passes extension register/unregister and Kernel-backed
  `session.hello_ack` with `transport_binding="vsp.hub.websocket"`.

See `VSP_HUB_VERIFICATION_2026_07_02.md`.
