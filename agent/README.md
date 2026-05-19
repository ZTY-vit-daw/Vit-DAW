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
