# VitAgent v0.1

VitAgent is the Go bridge and AI agent for Vit-DAW.

It keeps the existing runtime contract:

- Godot command UDP: `127.0.0.1:4445`
- Godot telemetry UDP: `127.0.0.1:4444`
- VitApp command ZMQ REQ: `tcp://127.0.0.1:5555`
- VitApp telemetry ZMQ SUB: `tcp://127.0.0.1:5556`
- Ask Vit HTTP API: `http://127.0.0.1:7878`

Build:

```powershell
go test ./...
go build -o .\bin\VitAgent.exe .\cmd\vitagent
```

AI config is read from `%USERPROFILE%\.vit\config.json`:

```json
{
  "baseUrl": "https://api.openai.com/v1",
  "apiKey": "...",
  "defaultModel": "..."
}
```

Useful dev run:

```powershell
.\bin\VitAgent.exe -verbose
```

Reusable dev smoke:

```powershell
# Fast check against the currently running agent.
powershell -NoProfile -ExecutionPolicy Bypass -File ..\scripts\dev_agent_smoke.ps1 -SkipBuild

# Build the current code to a temporary dev-smoke exe, reuse any running agent,
# and verify HTTP state plus natural mix chat routing.
powershell -NoProfile -ExecutionPolicy Bypass -File ..\scripts\dev_agent_smoke.ps1

# Install the new build and restart VitAgent explicitly.
powershell -NoProfile -ExecutionPolicy Bypass -File ..\scripts\dev_agent_smoke.ps1 -RestartAgent

# Local DAW stack smoke: start kernel/UI when available and try mix.observe.
powershell -NoProfile -ExecutionPolicy Bypass -File ..\scripts\dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke
```

The script keeps an already running agent alive by default. Use `-RestartAgent`
only when the new binary should replace the live process.

Offline mix acoustic lab:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ..\scripts\run_mix_acoustic_lab.ps1
```

The lab builds `cmd/mixlab`, creates a synthetic full-project observation from
the repository test audio files, writes a per-track acoustic feature snapshot,
and reports loudness/headroom rankings without requiring the DAW kernel.

Agent harness endpoints:

- `GET /agent/tools` lists the current DAW command catalog and tool metadata.
- `GET /agent/actions?limit=50` returns recent agent action journal entries.
- `POST /agent/invoke` invokes a registered tool or DAW command through policy, journal, kernel IPC, and shadow refresh.

Example:

```json
{
  "tool": "daw.invoke",
  "args": { "cmd": "get_project_state" },
  "source": "debug"
}
```
