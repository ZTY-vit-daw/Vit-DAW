Status: superseded by ADR-AGENT-CLEANUP-0001

# FXM v0 and the standalone VPS authoring workflow

## Model name and responsibility

FXM means **Effects Transformation Model** (效果器变换模型). It is a peer
projection beside MOM, TIM, TOM and EPM.

- MOM projects the current mix/project observation into compact evidence.
- FXM projects the counterfactual difference caused by the current plug-in
  chain under a frozen source, time window, routing state and render format.
- FXM never decides whether a transformation is musically desirable.

The first schema is `fxm.projection.v0`. The Mixboard read key is
`observation.fxm_projection`.

## Counterfactual contract

An actionable FXM projection requires at least two matching renders:

1. `bypass_chain`: the frozen reference with the effect chain bypassed.
2. `processed_chain`: the same source/window/routing/gain/automation/render
   settings with the intended chain state enabled.

FXM currently projects RMS, LUFS, peak, crest, latency and matching spectral
band deltas. A projection fails closed when source revision, time window,
sample rate or channel count differs. Raw audio and analyzer payloads remain
external evidence; Agent context receives only the compact projection and its
evidence references.

## VPS measurement contract

Every workspace created by `vpsforge` receives one
`vit.vps.fxm_measurement_requirement.v0` record per candidate capability. The
default plan requires:

- bypass and processed stages;
- deterministic program material and a log sweep;
- RMS, peak, band-energy and latency measurements;
- a documented default and a representative operation state;
- an explicit same-source/window/routing/gain/automation baseline policy.

This requirement is authoring evidence. It cannot issue a Credential or make a
mapping dispatchable.

## Staging-only VPS authoring tool

Build:

```powershell
go build -o output/vpsforge.exe ./cmd/vpsforge
```

On Windows, double-clicking `vpsforge.exe` now starts the Vit VPS Forge loopback web
workbench and opens it in the default browser. The workbench can create or open
an isolated workspace and remains running until the executable is closed.

Create a workspace:

```powershell
output/vpsforge.exe init `
  -workspace D:\VPS-Authoring\FabFilter-Pro-Q `
  -manufacturer FabFilter `
  -name "FabFilter Pro-Q" `
  -format VST3 `
  -version unknown `
  -capabilities equalizer.v2
```

The command creates only an isolated authoring workspace:

- `authoring_manifest.json`
- `evidence_ledger.json`
- `surface_snapshot.json` (after a real host probe)
- `fxm_measurement_plan.json`
- `fxm_projection.json` (after matched measurements)
- `vps_draft.json`

It does not modify the canonical VPS Library, Catalog, Credential store or
SPAL routes.

Run the local Codex-facing control service:

```powershell
output/vpsforge.exe serve `
  -workspace D:\VPS-Authoring\FabFilter-Pro-Q `
  -listen 127.0.0.1:8899
```

The loopback-only service exposes `GET /health`, `GET /v1/workspace`,
`POST /v1/surface`, `POST /v1/fxm` and `POST /v1/probe`. It intentionally has
no install, Credential or Catalog endpoint. Codex can therefore drive and
inspect the authoring workspace directly while authority-changing installation
remains a separate Vit gate.

Every new workspace also contains `README.md` and `agent_protocol.json` so an
agent can discover the workflow without relying on a Codex-only prompt. Codex,
Claude, OpenCode, Hermes or another agent may wrap the same protocol with its
own skill, but the staging and authority rules stay in these workspace-owned
files.

## Packaged probe audio

Generate/rebuild the deterministic suite with:

```powershell
output/vpsforge.exe test-audio -output assets\vps_probe_audio_v1
```

The canonical suite ID is `vit.vps_probe_audio.v1@48000`. It contains 48 kHz,
stereo, 24-bit PCM impulse, log-sweep, 31-band multitone, stepped-level sine,
transient, stereo-phase and tail probes. Their SHA-256 hashes and purpose are
stored in `manifest.json`. Existing `test_100hz_10s.wav` and
`test_target_3s.wav` remain useful manual witnesses but are not sufficient as
the complete FXM conformance corpus.

## Host adapter protocol

`vpsforge probe -host http://127.0.0.1:<port>` expects:

```text
GET /v1/plugin/snapshot
```

with a `HostSnapshot` response containing the real plugin identity, parameter
surface, display observations and bounded logs. This makes the authoring tool
independent of Vit while keeping plug-in format loading behind adapters.

The adapter roadmap is:

1. Windows VST3 adapter.
2. Windows/macOS CLAP adapter.
3. macOS AU adapter.
4. AAX only through a licensed AAX SDK and compatible host environment.

No commercial plug-in binary, preset or proprietary documentation is bundled
with Vit or `vpsforge`.

## Trust boundary

Host enumeration and probes are `observed`. User demonstrations are
`user-confirmed`. Agent mappings remain `agent-inferred` until separately
confirmed. FXM render comparisons may become `conformed` evidence only after
their source/window/routing/render invariants pass. Provider Credentials still
require the normal capability conformance and explicit installation gate.

## Proposed full-chain authoring order

The first reusable commercial-effect suite should cover one strong product per
role before adding overlapping alternatives:

1. parametric/dynamic EQ;
2. fast and leveling compressors;
3. gate/expander and de-esser;
4. saturation/distortion;
5. multiband dynamics;
6. reverb and delay;
7. clipper/limiter and output metering;
8. optional composite vocal/channel-strip products.

Products are not promoted merely because they are famous. Each installed
version/format still needs a real surface snapshot, human semantic witness,
write/readback, rollback, state-retention and FXM evidence.
