# Vit-DAW Phase 2.1 Real E2E Validation Gate

Date: 2026-06-20

Phase 2.1 is a validation gate, not Phase 3 worker/runtime work. A Phase 2 change is not accepted by `dev_agent_smoke.ps1` alone; at least one real Godot/kernel/agent path must also pass or have a concrete environment blocker with artifacts.

## Gate Matrix

| Layer | Command | Verifies | Does not prove |
| --- | --- | --- | --- |
| Go unit/integration | `cd D:\Vit_DAW\agent; go test -count=1 ./internal/agentprotocol ./internal/pendingmanager ./internal/observationrouter ./internal/agentloop ./internal/chat ./internal/executor ./internal/harness ./internal/policy` and `go test ./internal/...` | Typed protocol, Pending Manager, Observation Router, chat confirmation, tool execution contracts. | Live Godot/kernel audio project behavior. |
| Frontend build | `cd D:\Vit_DAW\agent\webui; npm run build` | Web UI TypeScript/build health. | Runtime agent/kernel/Godot integration. |
| Agent shell smoke | `powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW -SkipBuild` | Agent HTTP health, `/agent/state`, chat route guard, no-pending confirmation guard. | Full DAW observe/execute/reobserve; this may pass when ZMQ kernel is absent. |
| Real DAW E2E | `run_mix_single_tick_e2e.ps1`, `run_live_material_observation_smoke.ps1`, `run_blind_mix_conversation_smoke.ps1`, `run_vit_product_path_smoke.ps1` | Live agent/kernel/Godot flows, fixture creation/import, observation, confirmation, typed tool routing, blind conversation safety, product lifecycle evidence. | Long-session UX polish, Phase 3 worker runtime, multi-agent UI. |

## Real E2E Scripts

### Single Tick

Command:

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_mix_single_tick_e2e.ps1 -RepoRoot D:\Vit_DAW -SkipBuild -StartKernel
```

Use `-ReuseAgent -ReuseKernel` only when the running agent/kernel are known to be from the current build. This script creates a two-track fixture, imports audio, verifies observe-first pending, verifies unresolved vocal clarification, then confirms exactly one typed `mix.propose_tick` / `mix.apply_tick` / reobserve path without `daw.invoke` or raw `track.volume`.

Artifacts: `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\mix_single_tick_*`.

### Live Material Observation

Command:

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_live_material_observation_smoke.ps1 -RepoRoot D:\Vit_DAW -SkipBuild
```

Use `-ReuseAgent -ReuseKernel` only after confirming ports 7878/5555/5556 are owned by the expected current processes. This script imports real materials and asserts `mix.observe full_project` produces acoustic metrics, source fields, and rankings.

Artifacts: `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\live_material_observation_*`.

### Blind Conversation

Command:

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_blind_mix_conversation_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -SkipBuild -StartKernel
```

Use `-ReuseGodot -ReuseAgent -ReuseKernel` only when the existing Godot runtime, agent, and kernel are intentionally reused and match the current build. This is the primary Phase 2.1 blind gate. It covers observe-first natural requests, discuss/reobserve no-mutation guards, explicit confirmation, no-pending confirmation, ambiguous vocal target clarification, pan treatment, and low-mud plugin-preparation routing.

Phase 2.1 assertions include:

- One plugin-preparation confirmation may load/prepare at most once.
- `get_plugin_parameters` count is at most one per confirmation turn.
- `plugin.set_parameter` and `plugin_grabber.apply_control` are zero during preparation.
- Plugin prep continuation is at most one and must expose `TerminalResult` and/or `UserInputRequest`.

Artifacts: `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\blind_mix_conversation_*`.

### Product Path

Command:

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -SkipBuild
```

Use `-ReuseGodot -ReuseAgent -ReuseKernel` only when process reuse is intentional. This path verifies Godot autostart evidence, agent HTTP, kernel command/event ports, fixture creation/import through live kernel, product-path mix question, confirmation, no-pending guard, vocal focus, and vocal clarification loop.

Artifacts: `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_*`.

## Process Hygiene

Force a restart instead of `-Reuse*` when:

- The agent, kernel, or Godot process predates the current code build.
- The kernel listener points at an older fallback binary such as `VitApp\build_release\VitApp.exe`; the smoke scripts prefer `VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` because stale flat builds can miss newer commands such as `set_pan`.
- A previous smoke failed after starting processes.
- Port owners on 7878, 5555, or 5556 are unknown.
- The observed behavior contradicts unit tests or fresh artifacts.
- Validating the low-mud duplicate parameter-fetch fix.

Reuse is acceptable when:

- The script is explicitly validating a long-lived product session.
- Process paths and PIDs are recorded in the smoke artifacts.
- The reused binaries are known to include the current changes.

## Phase 3 Decision Rule

Phase 3 may start only after the low-mud path no longer double-fetches parameters, no-pending confirmation remains safe, ambiguous vocal targeting still asks for user input, plugin load/read/write boundaries are visible in artifacts, blind smoke can catch duplicate prep/fetch regressions, and at least one real Godot/kernel/agent product or blind E2E path passes.
