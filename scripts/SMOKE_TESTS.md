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

## G Runtime Read-Only Smoke (mac)

Path:

```bash
~/Documents/Vit-DAW/scripts/g_runtime_readonly_smoke_mac.sh
```

Purpose:

- mac port of the `g_runtime_readonly_smoke.ps1` whitelist GET probe mode
  (PORT-C4). Verifies the VitAgent HTTP liveness face with four read-only GET
  probes: `/health`, `/agent/runtime/status`, `/agent/state`,
  `/agent/events`. The script self-scans its own source for non-GET request
  construction and fails closed before issuing any request.
- Emits a `g.readonly.runtime.smoke.v1` summary JSON on stdout (same schema
  and field extraction as the ps1 version); exit code 0 means every
  whitelisted GET returned a 2xx status.
- Probe list is 1:1 with the ps1 whitelist. The events probe sends
  `?conversation_id=<probe>&limit=200` on both platforms (ps1
  `-ConversationId`, mac `--conversation-id`, same default
  `g-readonly-smoke-probe`) because the real agent contract
  (`agent/internal/chat/events.go`) answers HTTP 400 without a conversation
  id; the fixture server ignores query parameters either way (ps1 parity
  fixed in PORT-PC-VERIFY-1, 2026-09-18).

Common commands:

```bash
# Full chain on mac: build agent, start it with all runtime state isolated
# in a fresh temp dir (AGENTS §10), wait for /health, probe, stop the agent.
~/Documents/Vit-DAW/scripts/g_runtime_readonly_smoke_mac.sh --start-agent --timeout 30

# Fast check against an already running agent (ps1-equivalent probe mode).
~/Documents/Vit-DAW/scripts/g_runtime_readonly_smoke_mac.sh --agent-http http://127.0.0.1:7878

# Equivalence check against the GET-only fixture server.
python3 scripts/g_runtime_readonly_fixture_server.py --port 7879 &
~/Documents/Vit-DAW/scripts/g_runtime_readonly_smoke_mac.sh --agent-http http://127.0.0.1:7879
```

Notes:

- With no VitApp kernel running, `/agent/state` still returns 200 but retries
  the kernel dial on every call (~0.5s per probe on mac, connection refused);
  pass a larger `--timeout` when in doubt.
- Each run keeps its artifacts (per-endpoint bodies, agent log, binary
  checksum) under a fresh mktemp workdir printed to stderr; previous runs are
  never overwritten.

## Kernel+Agent Mac Smoke (PORT-A5)

Path:

```bash
~/Documents/Vit-DAW/scripts/dev_agent_smoke_mac.sh
```

Purpose:

- mac minimal equivalent of `dev_agent_smoke.ps1` for the real two-process
  stack (VitApp kernel + Go agent; Godot frontend out of scope).
- Builds the kernel (cmake+make into the run workdir) and agent, starts both
  with all runtime state isolated under a fake VitApp root inside the workdir
  (AGENTS §10; the source tree is never touched), then verifies:
  kernel ZMQ listeners 5555/5556/5557 owned by the kernel pid; the C4
  whitelist GETs against the agent; a direct kernel ZMQ `ping`; the
  agent->kernel ZMQ round-trip via `POST /agent/invoke` `project.state`;
  a `scan_plugins` child-process sweep of `/Library/Audio/Plug-Ins/VST3`
  (R9: `out_of_process=true` + `--PluginScan:` child ps samples + kernel log
  anchor); and the Waves WaveShell R2 gate (`plugin_list_available` with at
  least `--expected-waves` Waves bodies enumerated).
- Records the kernel stop method and exit code (A1 platform finding: SIGTERM
  yields 143 without JUCE shutdown) and reproduces/then-unlinks the POSIX shm
  residue via a `shm_open(O_CREAT|O_EXCL)` EEXIST probe.
- Emits a `dev.agent.smoke.mac.v1` summary JSON; exit 0 requires every gate
  to pass. Failures are classified env vs functional in the error line.

Common commands:

```bash
# Full chain on mac: build kernel+agent, run the stack, scan, stop.
~/Documents/Vit-DAW/scripts/dev_agent_smoke_mac.sh

# In an isolated git worktree whose tracktion_engine submodule is empty,
# point at any checkout of the pinned commit:
~/Documents/Vit-DAW/scripts/dev_agent_smoke_mac.sh \
  --tracktion-dir ~/Documents/Vit-DAW/tracktion_engine

# Reuse an already-built kernel binary (sha256+mtime recorded).
~/Documents/Vit-DAW/scripts/dev_agent_smoke_mac.sh --kernel-bin <path>/VitApp
```

Notes:

- The full WaveShell1-VST3 sweep takes several minutes (the mac 17.1 shell
  enumerates ~719 plugin bodies through the child scanner); the default
  `--scan-timeout 600` covers it. The tracktion master scan timeout can be
  shortened with `TRACKTION_PLUGIN_SCAN_TIMEOUT_MS` in the caller's
  environment (the kernel inherits it).
- Each run keeps its artifacts (kernel/agent logs, per-probe JSON, scan
  trajectory, lsof evidence, summary) under a fresh mktemp workdir printed to
  stderr; previous runs are never overwritten.

## PCA Calibration Chain Mac (PORT-C2)

Path:

```bash
~/Documents/Vit-DAW/scripts/pca_calibration_chain_mac.sh
```

Purpose:

- One-key mac calibration chain over the real two-process stack (VitApp
  kernel + Go agent), the mac equivalent of the PC-side calibration-chain
  pattern (`b1_2_source_calibration_agent_smoke.py` /
  `b1_2_strict_reference_calibration_agent_smoke.py` stack driving on top of
  the PORT-A5 harness): ① stack up (kernel under a fake VitApp root in the
  run workdir + agent with isolated state, `/health`, agent->kernel
  `project.state` round-trip); ② probe — one agent invoke
  `plugin.semantic_build_index` over the VST3 dir (kernel child-process scan)
  landing the machine-local semantic index at `~/.vit/plugin_semantics.json`;
  ③ whitelist draft — selects the U2-narrowed Waves subjects (12 plain
  families x Mono/Stereo = 24 bodies on this machine; U2 records 23, the
  reconciliation note lives in the draft) and emits a PC-schema-aligned
  `whitelist_draft.json` (certification-candidate fields + Entry identity
  fields + probe conclusions + schema comparison note); ④ PCA
  re-certification — per subject `POST /agent/processor-certification/start`
  (consent `temporary_track_apply_readback_restore`) polling the job to
  completion; receipts land under `~/.vit/pca_certifications/<job>/` and the
  attestations import into `~/.vit/processor_control_attestations.v{1,2}.json`
  with promoted status; ⑤ readonly smoke — the C4/A5 whitelist GETs plus
  `GET /agent/processor-certification/candidates?family=all` asserting every
  subject promoted for its manifest family, and a read-only parse of the PCA
  stores; the stack is stopped and `~/.vit` pre/post hashes are recorded.
- Emits a `pca.calibration_chain.mac.v1` summary JSON; exit 0 requires every
  gate to pass. Failures are classified env vs `functional_subject_load`
  (a promoted subject failed to instantiate/load — the R2 stop condition,
  evidence preserved for a blocked handover) vs functional
  (enumeration/draft/certification/readonly).
- The pluginprobe native observation host is not used
  (PluginProbe/native-host is Windows-only today); the certification runner
  itself performs the real load + typed inspect/apply/restore on a disposable
  track, which is the load evidence this chain needs.

Common commands:

```bash
# Full chain on mac: build kernel+agent, probe, draft, re-certify, readonly smoke.
~/Documents/Vit-DAW/scripts/pca_calibration_chain_mac.sh

# In an isolated git worktree (empty submodule) with FetchContent downloads
# needing a proxy to reach the github tarballs:
~/Documents/Vit-DAW/scripts/pca_calibration_chain_mac.sh \
  --tracktion-dir ~/Documents/Vit-DAW/tracktion_engine \
  --cmake-proxy http://127.0.0.1:7890

# Reuse an already-built kernel binary (sha256+mtime recorded).
~/Documents/Vit-DAW/scripts/pca_calibration_chain_mac.sh --kernel-bin <path>/VitApp
```

Notes:

- The scan step takes several minutes (the mac 17.1 WaveShell enumerates
  ~719 bodies through the child scanner; `--scan-timeout 900` covers it) and
  the certification loop runs one disposable-track job per subject
  (`--certify-timeout 420` per subject, 24 subjects → expect tens of minutes).
- Each run keeps its artifacts (kernel/agent logs, per-phase JSON, the
  whitelist draft and its final post-certification revision, per-subject
  certification records, readonly verification, summary) under a fresh mktemp
  workdir printed to stderr; previous runs are never overwritten. The
  machine-local `~/.vit` state this chain creates (semantic index, PCA
  stores, receipts) is the designed landing location; pre/post hashes are
  recorded per run.

## Journey 1 Demo Journey Mac (PORT-JOURNEY-1-MAC)

Path:

```bash
~/Documents/Vit-DAW/scripts/journey1_demo_journey_smoke_mac.sh
```

Purpose:

- Mac port of the PC demo-journey driver
  `scripts/journey1_demo_journey_smoke.ps1` (the JOURNEY-1 authority): one
  script, one demo journey over the real two-process stack (VitApp kernel +
  Go agent on the A5/C2 harness pattern; the Godot frontend is out of scope —
  the agent HTTP face drives everything, same as the ps1). Five explicit
  segment assertions per the card framing — S1 project open (copy of the
  912.vit demo project loaded through `open_project` via `/agent/invoke`,
  tracks + bass visible), S2 authority (`/agent/authority` echoes
  `full_project_access`), S3 experiment (the Chinese demo prompt
  「帮低音轨做个均衡实验，然后让我试听」 through `/agent/chat` as a real
  LLM turn; delivered text must be substantive and direction-asking-free,
  ps1 A1), S4 load (deterministic `rack_add_node` probe of a promoted mac PCA
  subject — default `C1 comp Mono`, the U2 machine fact replaces the PC
  bx_hybrid V2 static EQ — not denied by the PCA load gate, plugin_id +
  plugin_instance_ready in the Kernel's reply, UI projection shows the
  plugin; ps1 A2), S5 audition (audition.*/mix_tick event or
  audition_session_id on the conversation surface; ps1 A3). After the phase A
  save point the stack is torn down and the copy project reopened: the live
  reopened conversation must not carry pre-save-point history and must leave
  no continuation-stall WARN (ps1 A4).
- LLM precondition: the script preflights `~/.vit/config.json` (or
  `VIT_AGENT_LLM_*`/`OPENAI_*` env) and refuses to run (exit 2) when the
  engine config is incomplete — no stub replies. The API key never reaches
  logs or artifacts; only has_api_key/model/base-host are recorded.
- Isolation contract identical to the ps1: the user project and its
  `.vit_history` are read-only and fingerprinted before/after; only a byte
  copy inside the run workdir is operated on; `VIT_PROJECT_XML` redirects the
  kernel default project; `VIT_HISTORY_DRAFT_ROOT` +
  `VIT_ORCHESTRATION_STORE_PATH` isolate agent state; the kernel runs under a
  fake VitApp root in the workdir; fresh mktemp workdir per run.
- Emits a `vit_demo_journey_driver.mac.v1` report (`journey1_report.json`,
  assertions a1–a4 + five segments + per-phase evidence + prereq lines);
  exit 0 requires everything green (ps1 parity: 0 green / 1 red or
  inconclusive / 2 environment or load-path failure).
- §8 discipline is card-level (pre-declared): at most 3 valid runs per
  attempt; two consecutive failures at the same deterministic breakpoint stop
  the reruns.

Common commands:

```bash
# Full journey on mac: build agent, reuse/build kernel, run both phases.
~/Documents/Vit-DAW/scripts/journey1_demo_journey_smoke_mac.sh

# Reuse an already-built kernel binary (sha256+mtime recorded).
~/Documents/Vit-DAW/scripts/journey1_demo_journey_smoke_mac.sh \
  --kernel-bin ~/Documents/Vit-DAW/VitApp/build/VitApp_artefacts/Debug/VitApp

# Skip phase B (A4 becomes not_observable -> exit 1 journey_inconclusive).
~/Documents/Vit-DAW/scripts/journey1_demo_journey_smoke_mac.sh --skip-reopen
```

Notes:

- The experiment segment needs a working LLM config (`~/.vit/config.json`,
  same schema as PC: baseUrl/apiKey/defaultModel) — the preflight fails fast
  otherwise.
- Three mac machine-fact adaptations the ps1 got implicitly from the PC repo
  environment (run-1/2 forensics; journey semantics unchanged): ① the 912
  stems store PC-absolute fixture paths and the kernel resolves each stored
  string as ONE literal filename under the copy project dir — the driver
  synthesizes equivalent 20 s/44.1 kHz stereo stems there (`--stems-dir` for
  real audio); ② the mac fake-root kernel starts with a cold plugin list, so
  the driver drives one `plugin.semantic_build_index` scan (~4-5 min for the
  719-body WaveShell sweep) before the journey; ③ kernel "Engine is busy
  rendering" replies are retried (5 s x 12) around the probe/save steps.
- The full journey takes roughly 2–5 minutes beyond stack start (kernel
  dwell, the LLM turn budget of 480 s, reopen dwell 20 s); each run keeps its
  artifacts (per-step JSON under `phase_a/`/`phase_b/`, both binary sha256s,
  agent/kernel logs, prereq.txt, report) under a fresh mktemp workdir printed
  to stderr.

## VSP Hub Three-Piece Stack Smoke Mac (PORT-VSPHUB-1)

Path:

```bash
~/Documents/Vit-DAW/scripts/vsp_hub_three_piece_smoke_mac.sh
```

Purpose:

- mac three-piece stack smoke over the A5 harness pattern (VitApp kernel +
  vsphub + Go agent): builds kernel + agent + vsphub, starts all three with
  runtime state isolated in a fresh temp workdir (AGENTS §10), and gates:
  ① kernel ZMQ listeners 5555/5556/5557 owned by the kernel pid; ② a negative
  required-mode pre-phase — an agent with `VIT_AGENT_VSP_HUB_REQUIRED=true`
  and NO hub must log retryable `registration pending` lines and then
  self-stop at the documented 30x2s bound (`required VSP Hub registration
  failed ... attempts=30`); ③ the agent starts BEFORE the hub so the bounded
  retry window is captured, then the hub comes up on 8787 (health + port
  owner) and registration must succeed — agent log line
  `VitAgent registered with VSP Hub` + hub `/vsp/status` session
  (`role=agent client_id=vit.agent.official`, kernel-signed session id);
  ④ a bash/python port of `codex_vsp_readonly_probe.ps1`: health, capability
  advertisement (extension role gets state.snapshot/event.poll, not
  command.request), `session.hello` (role=extension read_only) →
  `extension.register` → status listing → `state.snapshot` round-tripped
  through the hub to the kernel (payload ok + no internal track-id leak) →
  `event.poll` → `extension.unregister` → status cleanup; ⑤ optional
  `--ws-probe`: drives the frontend repo's Godot headless
  `vsp_realtime_ws_live_probe.gd` against the live hub — the adapter must
  subscribe over `ws://127.0.0.1:8787/vsp/stream` (state=open,
  `subscription_transport=vsp.hub.websocket`) and receive realtime frames
  with zero HTTP fallback; ⑥ teardown with per-process stop records and a
  port-drain assert.
- `--external-kernel` variant: no kernel is built/started/stopped — the hub
  and the agent connect to an ALREADY-RUNNING kernel on 5555/5556 (observed
  and recorded, never touched; the kernel gate becomes an observation plus an
  untouched-after-teardown assert). Use it when the fixed kernel ports are
  legitimately owned by another live stack (e.g. the user's open Godot
  editor); pass `--agent-http`/`--udp-*` on alternate ports.
- Emits a `vsp.hub.three_piece.smoke.mac.v1` summary JSON; exit 0 requires
  every enabled gate to pass. Failures are classified env (ports, build)
  vs functional (registration, probe checks).

Common commands:

```bash
# Full three-piece run: build kernel+agent+hub, bounded-fail pre-phase,
# stack up, registration, read-only roundtrip, teardown.
~/Documents/Vit-DAW/scripts/vsp_hub_three_piece_smoke_mac.sh

# Same with the Godot ws live probe (needs the frontend repo + Godot binary).
~/Documents/Vit-DAW/scripts/vsp_hub_three_piece_smoke_mac.sh --ws-probe

# Ride an already-running kernel (e.g. the editor's stack holds 5555):
~/Documents/Vit-DAW/scripts/vsp_hub_three_piece_smoke_mac.sh \
  --external-kernel --agent-http http://127.0.0.1:7879 \
  --udp-to-godot 14444 --udp-from-godot 14445

# Reuse a built kernel binary (sha256+mtime recorded) and skip the ~70s
# bounded-fail pre-phase.
~/Documents/Vit-DAW/scripts/vsp_hub_three_piece_smoke_mac.sh \
  --kernel-bin ~/Documents/Vit-DAW/VitApp/build/VitApp_artefacts/Debug/VitApp \
  --skip-bounded-fail
```

Notes:

- The product-path hub binary lands at `<repo>/agent/bin/vsphub` (gitignored;
  the start_page autostart candidate) — build it with
  `cd agent && go build -o bin/vsphub ./cmd/vsphub`; the smoke script builds
  its own copy in the run workdir and records both sha256s.
- The bounded-fail pre-phase takes ~60-70 s (30 attempts x 2 s); skip it with
  `--skip-bounded-fail` during iteration.
- The ws probe launches a second Godot instance headless against the frontend
  project; the adapter ws channel is default-on since the fix5 flip-back
  (PORT-VSPHUB-1), so no env is required — the script still exports
  `VIT_GUI_VSP_REALTIME_WS_ENABLE=1` (pre-flip probe compatibility, a no-op
  after the flip).

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
