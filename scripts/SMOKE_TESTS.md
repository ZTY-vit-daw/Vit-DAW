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

## B1 Gain Staging Agent Smokes

Paths:

```powershell
python D:\Vit_DAW\scripts\b1_group_reset_agent_smoke.py --repo-root D:\Vit_DAW --timeout-sec 150
python D:\Vit_DAW\scripts\b1_2_source_calibration_agent_smoke.py --repo-root D:\Vit_DAW --timeout-sec 150
python D:\Vit_DAW\scripts\b1_2_a4_multiclip_agent_smoke.py --repo-root D:\Vit_DAW --project-path "D:\path\to\post-a4-project.vit" --timeout-sec 300
python D:\Vit_DAW\scripts\b1_2_a4_multiclip_agent_smoke.py --repo-root D:\Vit_DAW --project-path "D:\path\to\post-a4-project.vit" --full-b1 --timeout-sec 300
python D:\Vit_DAW\scripts\b1_2_a4_multiclip_agent_smoke.py --repo-root D:\Vit_DAW --project-path "D:\path\to\post-a4-project.vit" --decision approve --timeout-sec 300
python D:\Vit_DAW\scripts\b1_3_full_gain_staging_agent_smoke.py --repo-root D:\Vit_DAW --timeout-sec 180
```

Purpose:

- Verify B1.1 creates a pending `track.group.apply_control` absolute 0 dB
  reset, then confirms and reads back target faders at unity.
- Verify B1.2 creates a pending `clip.gain.set` source calibration action,
  confirms it, then reads back clip gain and re-observes the project.
- Verify a saved post-A4 multi-clip project can rebuild missing DAD rows, expand
  each track calibration across every surviving clip, and remain unchanged when
  the pending plan is denied.
- Verify B1.3 chains the two confirmed steps: B1.1 confirmation re-reads
  `project.state`, re-runs `mix.observe`, queues B1.2 `clip.gain.set`, and the
  second confirmation verifies fader 0 dB plus calibrated clip gain.
- Guard against accidental `mix.propose_tick` / `mix.apply_tick` routing for B1
  technical calibration.

## B4 Low-End Relation Agent Smoke

Path:

```powershell
python D:\Vit_DAW\scripts\b4_low_end_relation_agent_smoke.py --repo-root D:\Vit_DAW --timeout-sec 180 --dad-timeout-sec 240
```

Purpose:

- Verify `static_mix.low_end_relation.v0` (B4), a read-only project observation
  capability, never writes anything and never produces a Proposal: every turn
  must resolve to `OutcomeAnalysis` (`canary_stage="analysis"`) or
  `OutcomeBlocked`, with `needs_confirmation` false and no
  `interaction_requests`.
- Guard against B4 regressing into a permanent dead end when band-energy data
  is missing — verify it self-triggers `project.audio_analysis_start` via the
  harness instead of returning a static blocked reply forever.
- Verify the cold-start turn (before any audio analysis has run) reaches
  `canary_stage="band_analysis_triggered"`, then confirm via a follow-up
  `project.audio_analysis_status` call that an analysis job now really exists.
  The `capability_runtime_v1` canary handler family (B2/B3/B4) never populates
  `executed_kernel_reply`, so this real side-effect check is the reliable proof
  that the trigger fired a kernel command and not just a claim in reply text.
- Poll `project.audio_analysis_status` to `dad_fact_status="ready"`, then
  verify a re-ask reaches `canary_stage="analysis"` with a non-empty
  `workflow_data.low_end_summary` (`sub_band_track_count` /
  `bass_band_track_count` > 0).
- Verify no B4 turn ever mutates track volume/pan, by diffing `project.state`
  fader/pan values from before the first turn to after the third turn.
- Verify idempotent re-trigger safety: once readiness is satisfied, asking
  again goes straight to `canary_stage="analysis"` rather than looping back to
  `band_analysis_triggered`, which would mean B4 redundantly re-triggered
  audio analysis after readiness was already satisfied.

## VSP Hub Extension Smoke

Path:

```powershell
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1
```

Purpose:

- Verify a third-party extension can register through `VspHub.exe` without using
  VitAgent or the legacy bridge.
- Verify `/vsp/status` lists the extension session.
- Verify `role="extension"` is denied `command.request` by default.
- Verify `extension.unregister` removes the session.

Common command:

```powershell
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp
D:\Vit_DAW\scripts\vsp_hub_extension_smoke.ps1 -HubUrl http://127.0.0.1:8787/vsp -CheckKernelRoutes
```

## Codex VSP Read-Only Probe

Path:

```powershell
D:\Vit_DAW\scripts\codex_vsp_readonly_probe.ps1
```

Purpose:

- Verify a third-party agent can connect directly to `VspHub.exe` as
  `client_id="codex.agent.local"` and `role="extension"` without being
  disguised as VitAgent.
- Verify `/health` and `/vsp/status` expose the read-only extension contract.
- Verify the Codex probe can register, appear in `/vsp/status`, read
  `state.snapshot`, poll `event.poll`, and unregister.
- Verify the probe sends no `command` channel traffic.

Common commands:

```powershell
D:\Vit_DAW\scripts\codex_vsp_readonly_probe.ps1 -HubUrl http://127.0.0.1:8787/vsp
D:\Vit_DAW\scripts\codex_vsp_readonly_probe.ps1 -HubUrl http://127.0.0.1:8787/vsp -HubOnly
```

## VSP Phase 4 Hub HTTP Asset Smoke

Path:

```powershell
D:\Vit_DAW\scripts\vsp_phase4_hub_http_asset_smoke.py
```

Purpose:

- Verify a GUI client can use `VspHub.exe` over HTTP `/vsp`, not direct Kernel
  ZMQ, for asset-lane routing.
- Verify Hub status advertises `asset.manifest_request` for GUI and read-only
  extension roles.
- Create a temporary track and audio clip through the Hub, then verify
  `asset.reference` and `asset.manifest` return platform-neutral refs without
  large inline waveform/spectrum JSON payloads.
- Remove the temporary clip and track.

Common command:

```powershell
python D:\Vit_DAW\scripts\vsp_phase4_hub_http_asset_smoke.py --hub-url http://127.0.0.1:8787/vsp
```

## VSP Hub WebSocket Smoke

Path:

```powershell
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
```

Purpose:

- Verify `/vsp/stream` accepts VSP envelopes over WebSocket.
- Verify a WebSocket extension can register, appear in `/vsp/status`, and
  unregister without using VitAgent or the legacy bridge.
- With `-CheckKernelRoutes`, verify WebSocket `session.hello` reaches Kernel
  through the Hub and returns `transport_binding="vsp.hub.websocket"`.

Common commands:

```powershell
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1
D:\Vit_DAW\scripts\vsp_hub_websocket_smoke.ps1 -CheckKernelRoutes
```

## VSP Hub Product Lifecycle Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_vsp_hub_lifecycle_smoke.ps1
```

Purpose:

- Start or reuse the Godot project runtime for
  `D:\Godot\project\vit-daw-frontend`.
- Build `VitAgent.exe` and `VspHub.exe` unless skipped or explicitly reused.
- Let the Godot project startup lifecycle autostart Kernel, Hub, and Agent.
- Verify Kernel ZMQ, VSP Hub HTTP (`127.0.0.1:8787`), and Agent HTTP
  (`127.0.0.1:7878`) are present.
- Verify the running Hub and Agent binaries match the expected binary hashes.
- Verify `GET /health` returns `service="VspHub"` and `status="ok"`.
- Verify `GET /vsp/status` advertises `vsp.hub.http` and lists
  `client_id="vit.agent.official"` with `role="agent"`.
- Run a headless Godot probe that connects to `/vsp/stream`, subscribes to
  realtime over `vsp.hub.websocket`, publishes synthetic Hub realtime frames,
  and verifies the GUI adapter receives them without HTTP frame fallback.

Common command:

```powershell
D:\Vit_DAW\scripts\run_vsp_hub_lifecycle_smoke.ps1 -RepoRoot D:\Vit_DAW
D:\Vit_DAW\scripts\run_vsp_hub_lifecycle_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64_console.exe
```

Notes:

- This smoke intentionally stops after Hub lifecycle and realtime transport
  verification. It does not run Ask Vit chat, mix workflow, project fixture
  creation, or LLM-dependent checks.
- Use this as the product infrastructure gate when changes touch Hub startup,
  launcher behavior, packaged layout, or VSP registration.
- Use `-ReuseGodot`, `-ReuseHub`, `-ReuseAgent`, or `-ReuseKernel` only when
  intentionally testing already-running local components.

## Project Audio Settings + Preflight Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_project_audio_settings_preflight_smoke.ps1
```

Purpose:

- Start or reuse the live VitApp kernel.
- Create a temporary project and verify default project audio settings:
  48 kHz / 24-bit / WAV/BWF.
- Set a mismatch example, save it to a temporary `.tracktionedit`, reopen it,
  and verify the project audio settings persisted.
- Run `project.import_preflight` and `media.inspect_files` against the
  development stems folder without writing tracks or clips.
- Save a full JSON artifact with command replies, compact summaries, mismatch
  warning evidence, and preflight counts.

Common command:

```powershell
D:\Vit_DAW\scripts\run_project_audio_settings_preflight_smoke.ps1 -RepoRoot D:\Vit_DAW -KernelExe D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe
```

Notes:

- The default training folder is
  `E:\BaiduNetdiskDownload\yingge - sattelites tracks out`.
- This smoke intentionally does not import the stems folder; it only validates
  metadata probe and import-plan generation.
- Full artifact path is
  `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\project_audio_preflight_<timestamp>\summary.json`.

## Project Stems Import Smoke

Path:

```powershell
D:\Vit_DAW\scripts\run_project_stems_import_smoke.ps1
```

Purpose:

- Start or reuse the live VitApp kernel.
- Create a temporary saved project.
- Preflight the development stems folder, then import it with
  `project.import_folder_as_stems`.
- Verify the import creates one audio track and one aligned clip per readable
  file in one kernel command.
- Verify created track and clip IDs are visible immediately after import and
  still visible after reopening the saved project.
- Run a read-only sealed-folder preflight when the sealed folder exists.

Common command:

```powershell
D:\Vit_DAW\scripts\run_project_stems_import_smoke.ps1 -RepoRoot D:\Vit_DAW -KernelExe D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe
```

Notes:

- The default training folder is
  `E:\BaiduNetdiskDownload\yingge - sattelites tracks out`.
- The default sealed folder is
  `E:\BaiduNetdiskDownload\yingge - Weekend Lover tracks out`; this smoke only
  preflights it and does not import or mutate it.
- Full artifact path is
  `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\project_stems_import_<timestamp>\summary.json`.

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
- Build `VitAgent.exe` and `VspHub.exe`, then let the Godot project lifecycle
  autostart Kernel, Hub, and Agent.
- Verify the Godot project process, VSP Hub HTTP (`127.0.0.1:8787`), agent HTTP
  (`127.0.0.1:7878`), and kernel ZMQ ports are present in one run.
- Verify Godot lifecycle evidence for the Kernel/Hub/Agent chain: prefer
  explicit autostart child logs when Godot debug logging prints them; otherwise
  require the Godot runtime Hub self-check, Agent self-check, and matching port
  owner paths.
- Verify `GET /health` returns `service="VspHub"` and `status="ok"`.
- Verify `GET /vsp/status` advertises `vsp.hub.http` and lists the official
  Agent session with `client_id="vit.agent.official"` and `role="agent"`.
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
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe -CompressorControlAgentOnly -KeepProcesses
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe -CompressorOpenSemanticRoutingAgentOnly
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe -SemanticProcessorOpenExperimentAgentOnly -TimeoutSeconds 180
```

Notes:

- For Hub-only startup regressions, prefer
  `D:\Vit_DAW\scripts\run_vsp_hub_lifecycle_smoke.ps1`; this full product-path
  smoke continues into Ask Vit/mix workflow behavior after lifecycle has passed.
- Until a programmable Godot chat hook exists, this smoke records
  `interaction_path=agent_http_after_godot_project_lifecycle`. That means it
  proves the Godot project lifecycle plus the real agent/kernel mix route, but
  it does not claim automated typing into the Godot chat input.
- The script can infer `-GodotExe` and `-GodotProjectRoot` from an already-open
  Godot editor whose command line contains `--path <project> --editor`.
- Use `-ReuseGodot`, `-ReuseHub`, `-ReuseAgent`, or `-ReuseKernel` when
  intentionally testing already-running local components. The build logic treats
  Agent and Hub separately, so reusing one does not skip rebuilding the other.
  `-ReuseUI` and `-UIExe` are retained only for explicit exported launcher
  checks; they are not the default product path.
- By default, processes started by this script are stopped at the end. Use
  `-KeepProcesses` to leave them running for manual inspection.
- `-CompressorControlAgentOnly` keeps the full Godot-owned lifecycle and binary
  verification, then checks the compressor-control tool catalog, API-2500 topology and one
  reversible apply/readback/restore cycle, plus L2 limiter rejection. Its
  temporary tracks are deleted before the smoke returns.
- `-CompressorOpenSemanticRoutingAgentOnly` keeps the same Godot-owned lifecycle
  and verifies two open-language compressor paths. With an existing qualified
  broadband compressor, the Agent must discover the instance without receiving
  its plugin ID, propose with zero writes, require confirmation, and execute the
  semantic compressor plan. With an empty rack, the Agent must choose the
  compressor category, recommend and load a processor under a separate load
  confirmation, qualify the new instance, then require a second confirmation
  before parameter execution. Both temporary tracks are deleted.
- `-SemanticProcessorOpenExperimentAgentOnly` rebuilds the current opaque
  eight-case compressor/EQ fixture, runs the blind public manifest through the
  Godot-owned production path, and then invokes the separate sealed evaluator.
  Use `-SemanticProcessorExperimentCases "so_02,so_05"` for a labelled subset.
  Agent/LLM route mismatches remain experiment results; infrastructure errors
  fail the launcher. The report and raw stage responses are stored under the
  product-path artifact directory.

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
