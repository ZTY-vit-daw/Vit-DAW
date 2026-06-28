# Vit DAW Smoke Tests

This directory contains stable development smoke-test entry points. Keep these
paths stable so future agent, kernel, and mix-loop work can reuse the same
commands.

## Observation v1 Acceptance Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_observation_v1_acceptance_smoke.ps1
```

Purpose:

- Run one stable Observation v1 acceptance entry point before agent/action
  refactors.
- Parse the Godot project headlessly.
- Run the observation Go regression set:
  `acousticpackage`, `mom`, `mixboard`, `agentloop`, and `chat`.
- Verify DAD L3 package evidence for `band_energy_summary`,
  `stereo_relation_summary`, and `loudness_summary`.
- Verify L2 Render Probe status/evidence shape and that raw render payload keys
  do not leak.
- Verify L2 realtime observation smoke through Godot headless script.
- Verify MOM observation/action-preflight/AB-result contracts, including:
  `观察当前工程的频段和声像状态，不要修改。`,
  `比较一下各轨频段占用和声像关系，不要修改。`,
  `帮我把低频稍微收一点，但先告诉我依据。`, and
  `确认后检查 AB result。`
- Run the Godot product-path lifecycle smoke and assert the product path starts
  the expected `agent\bin\VitAgent.exe`.
- Save a compact summary at
  `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\observation_v1_acceptance_<timestamp>\summary.json`.

Common command:

```powershell
D:\Vit_DAW\scripts\run_observation_v1_acceptance_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64_console.exe
```

Notes:

- Use `-SkipProductPath`, `-SkipKernelSmokes`, or `-SkipGodotHeadless` only for
  local triage; the acceptance run should leave them enabled.
- Component smoke logs and discovered artifact paths are linked from the
  unified `summary.json`.

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
- By default it prefers the release artefact kernel at
  `D:\Vit_DAW\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe`,
  falling back to older local builds only when the artefact is missing. The old
  staging runtime does not support the acoustic fixture path needed by this E2E.
- Use `-KernelExe <path>` to test a specific kernel build.
- Use `-ReuseKernel` only when the currently running kernel is intentionally the
  one under test.

## Product-Path Lifecycle + Mix Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1
```

Purpose:

- Start or reuse the Godot project runtime for
  `D:\Godot\project\vit-daw-frontend`.
- Build `VitAgent`, then let the Godot project lifecycle autostart the agent
  and kernel.
- Verify the Godot project process, agent HTTP, and kernel ZMQ ports are present
  in one run.
- Verify Godot lifecycle evidence for the agent/kernel chain: prefer explicit
  autostart child logs when Godot debug logging prints them; otherwise require
  the Godot runtime agent self-check plus matching agent/kernel port owner
  paths.
- Create the deterministic two-track fixture through the live agent/kernel path.
- Send the full-project mix conversation through agent HTTP after the Godot
  project lifecycle is up.
- Verify the pending/confirm route:
  `mix.propose_tick -> mix.apply_tick -> mix.observe`.
- Verify no `daw.invoke` or direct `track.volume` appears on the confirmation
  path.
- Save a smoke artifact folder containing `processes`, `ports`,
  `conversation_id`, `observation_id`, `pending_candidate`, `tool_route`, and
  `before_after` evidence.
- Save `mix_loop_v1` evidence in `summary.json`: confirmation-turn counts for
  propose/apply/reobserve, post-confirm pending event count, any next pending
  candidate, and `auto_second_apply_detected=false`.
- Assert the confirmation turn executes exactly one propose/apply/reobserve
  route. If a next candidate is generated, it must remain pending and require a
  later explicit confirmation.
- Verify the vocal focus relationship path with "make the lead vocal more
  forward": the turn may finish as `done` or `needs_clarification`, but it must
  route through `mix.observe` and `mix.derive` and must not call
  `mix.apply_tick`, `daw.invoke`, or direct `track.volume`.
- Keep `summary.json` compact; full raw chat responses remain in
  `chat_observe.json`, `chat_confirm.json`, `chat_no_pending.json`, and
  `chat_vocal_focus.json`.

Common command:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe
```

Notes:

- Until a programmable Godot chat hook exists, this smoke records
  `interaction_path=agent_http_after_godot_project_lifecycle`. That means it
  proves the Godot project lifecycle plus the real agent/kernel mix route, but
  it does not claim automated typing into the Godot chat input.
- The script can infer `-GodotExe` and `-GodotProjectRoot` from an already-open
  Godot editor whose command line contains `--path <project> --editor`.
- Use `-ReuseGodot`, `-ReuseAgent`, or `-ReuseKernel` when intentionally testing
  already-running local components. `-ReuseUI` and `-UIExe` are retained only for
  explicit exported launcher checks; they are not the default product path.
- By default, processes started by this script are stopped at the end. Use
  `-KeepProcesses` to leave them running for manual inspection.

## Go Regression Set

Run after changing mix observation, pending candidate, confirmation routing, or
harness behavior:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/mixboard ./internal/harness ./internal/chat ./internal/agentloop -count=1
```

Mix observation contracts covered by this set include full-project active
acoustic track counts, per-track lightweight acoustic packages with
peak/RMS/headroom/crest and primary clip source fields, loudness/peak/headroom
rankings, and vocal focus routing for `full_project_with_focus_track`.

## Live Material Observation Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_live_material_observation_smoke.ps1
```

Purpose:

- Build/restart or reuse `VitAgent`.
- Start or reuse a live kernel.
- Create a fresh project and import available real materials from the repo root,
  including `test_100hz_10s.wav`, `Paper Crown.mp3`, root-level MP3 files, and a
  repo demo OGG when present.
- Directly invoke `mix.observe scope=full_project` through agent HTTP.
- Verify read-only full-project observe returns MOM `v1.4` with
  `project_multitrack_relation_observation`, compact multitrack projection, and
  no pending/confirmation request.
- Verify every imported track has a ready lightweight acoustic package with
  peak, RMS, headroom, crest, primary clip id/name, and source path.
- Verify loudness, peak, and headroom-risk rankings include the imported
  material tracks.
- Save compact artifacts: `summary.json`, `imports.json`, `track_acoustics.json`,
  `rankings.json`, and the raw `mix_observe_full_project.json`.

Common command:

```powershell
D:\Vit_DAW\scripts\run_live_material_observation_smoke.ps1 -RepoRoot D:\Vit_DAW
```

## AB Result Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_ab_result_smoke.ps1
```

Purpose:

- Verify MOM v1.4 AB result layer is derived locally from same-tap L2 Render
  Probe observations.
- Verify reused `render_revision` is stale, not ready.
- Verify raw render payload fields stay out of LLM context.
- Verify confirmation reports include AB Result details or explicitly mark AB
  as untrusted.
- Verify confirmation project result cards expose compact AB Result state for
  the WebUI/Godot card surface.
- Verify plugin-prep parameter confirmation carries the before observation into
  its post-write observation and surfaces AB Result state in the reply.

Common command:

```powershell
D:\Vit_DAW\scripts\run_ab_result_smoke.ps1 -RepoRoot D:\Vit_DAW
```

Notes:

- This smoke validates live kernel observation after real import. MP3 offline lab
  support is allowed to remain unsupported; this path proves imported MP3 tracks
  can still be observed through the running kernel.
- Use `-MaterialPaths` to run a hand-picked material set.
- By default, processes started by this script are stopped at the end. Use
  `-KeepProcesses` to leave the imported material project running for manual
  inspection.
