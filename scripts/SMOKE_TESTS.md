# Vit DAW Smoke Tests

This directory contains stable development smoke-test entry points. Keep these
paths stable so future agent, kernel, and mix-loop work can reuse the same
commands.

## Agent Health Smoke

Path:

```powershell
D:\Vit_DAW\scripts\dev_agent_smoke.ps1
```

Purpose:

- Build or reuse `VitAgent`.
- Start or reuse the agent HTTP server.
- Check `/agent/state`.
- Check kernel ZMQ ports when a kernel is running.
- Optionally run a basic chat smoke.

Common commands:

```powershell
D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW -RestartAgent -NoChatSmoke
D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW -SkipBuild -NoChatSmoke
```

## Mix Single-Tick E2E Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_mix_single_tick_e2e.ps1
```

Purpose:

- Build/restart or reuse `VitAgent`.
- Start or reuse a real kernel.
- Create a fresh test project.
- Import `test_100hz_10s.wav` and `test_target_3s.wav`.
- Ask the natural-language full-project mix question.
- Verify a pending mix tick is stored for Track 2.
- Confirm execution with natural language.
- Verify the deterministic route:
  `mix.propose_tick -> mix.apply_tick -> mix.observe`.
- Verify the confirmation path does not use `daw.invoke` or `track.volume`
  directly.

Common commands:

```powershell
D:\Vit_DAW\scripts\run_mix_single_tick_e2e.ps1 -RepoRoot D:\Vit_DAW
D:\Vit_DAW\scripts\run_mix_single_tick_e2e.ps1 -RepoRoot D:\Vit_DAW -SkipBuild -ReuseAgent
```

Notes:

- This test mutates the current DAW/kernel state by creating a fresh project and
  importing fixture audio.
- By default it prefers the newest local kernel at
  `D:\Vit_DAW\VitApp\build_release\VitApp.exe`, because the old staging runtime
  does not support the acoustic fixture path needed by this E2E.
- Use `-KernelExe <path>` to test a specific kernel build.
- Use `-ReuseKernel` only when the currently running kernel is intentionally the
  one under test.

## Go Regression Set

Run after changing mix observation, pending candidate, confirmation routing, or
harness behavior:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/mixboard ./internal/harness ./internal/chat ./internal/agentloop -count=1
```
