# Read this first: Vit VPS Forge

This document is the portable entry point for Codex, Claude, OpenCode, Hermes
or another tool-using agent. Read it before touching a plugin, authoring
workspace or Vit installation.

## Objective

Create an evidence-backed VPS Draft for one real, locally installed plugin;
measure its FXM transformation variables; run bounded conformance; and present
the Draft/Diff and unknowns to the user. Vit VPS Forge is staging-only.

## Non-negotiable authority rules

1. Host parameter enumeration, labels and automatic probes are observed facts,
   not executable semantic authority.
2. Never invent an unknown function or silently convert an inference into a
   confirmed mapping.
3. Preserve trust as one of: `observed`, `user-confirmed`, `agent-inferred`,
   `conformed`, or `unsupported/unknown`.
4. Never overwrite, clear, regenerate or install the canonical VPS Library
   from Vit VPS Forge.
5. Never issue or claim a Verified Credential. Installation, Credential and
   Catalog changes require a separate explicit Vit gate after conformance.
6. Never dispatch a special capability without its own schema and conformance.
7. Every parameter-write test freezes a complete preimage, performs fresh
   readback and has an explicit rollback path.

## Start

On Windows, double-click `vpsforge.exe`. It opens the Vit VPS Forge loopback
workbench. For manual witness work, select a completed staging preflight
package, click **Open live witness GUI**, then use the separate native VST3
window. In that window, select an output with **Audio Device...**, confirm the
input level, click **Play**, and operate the plug-in while it is audible. The
browser is only a staging selector and guide; it does not embed or control the
plug-in itself.
An agent may instead use:

```text
vpsforge init
vpsforge status
vpsforge probe
vpsforge ingest-surface
vpsforge measure
vpsforge validate
vpsforge serve
vpsforge host
vpsforge witness
vpsforge preflight
vpsforge pro-q-3-resource-isolation-probe
vpsforge pro-q-3-review-summary
vpsforge equalizer-v2-proposal-draft
vpsforge equalizer-v2-scope-record
```

The authoritative machine-readable workspace protocol is
`agent_protocol.json`, generated inside every workspace.

## Workflow

1. Create one isolated workspace for the exact plugin version and format.
2. Connect the native plugin-host adapter and capture the full parameter and
   display surface.
3. Build the Evidence Ledger. Keep every GUI witness, user description, host
   probe, manufacturer source and Agent inference separately attributable.
4. Register all parameters without assigning unknown semantics.
5. Ask the user to demonstrate the smallest useful capability slice.
6. Map user purpose → semantic action → host control → risk/precondition →
   rollback.
7. Use `vit.vps_probe_audio.v1@48000` for matched bypass/processed renders and
   generate the FXM projection. Existing musical material may supplement but
   not replace the deterministic suite.
8. Run capability-scoped write/readback, boundary, state-retention,
   resource-isolation, behavior and rollback conformance.
9. Keep failed or missing items explicit. A partial VPS is acceptable; a
   fluent invented capability is not.
10. Present the complete staging bundle and request separate installation
    authority only when all required gates pass.

## Independent Windows VST3 adapter and preflight

`vpsforge host` starts a loopback-only API around the isolated native
`vpsforge_vst3_worker.exe`. The plug-in is loaded in a child process, so a
plug-in crash cannot take down the Forge workspace service. The worker can
observe identity/Class ID/file fingerprint, buses, latency/tail and the full
parameter surface; perform preimage-protected writes with fresh readback and
rollback; serialize/reload/restore state; and offline-render the packaged
Probe Audio suite.

```text
vpsforge host -listen 127.0.0.1:8900 -worker <vpsforge_vst3_worker.exe>
vpsforge preflight \
  -workspace D:\VPS-Forge-Staging\FabFilter-Pro-Q-3 \
  -host http://127.0.0.1:8900 \
  -probe-audio D:\Vit_DAW\VPSForge\assets\vps_probe_audio_v1 \
  -candidate-badge equalizer.v2 \
  -category spectral_processing \
  -task equalizer.correct_tone
```

The host API is loopback only and includes `POST /v1/plugin/load`, `GET
/v1/plugin/snapshot`, parameter write, state save/restore/roundtrip,
rollback, render and unload routes. It has no Library, Credential, Catalog or
SPAL-routing route.

Preflight adds `plugin_identity.json`, `parameter_probe_results.json`,
`state_roundtrip_results.json`, `fxm_default_baseline.json`,
`inferred_parameter_groups.json`, `human_evidence_gaps.json`,
`witness_rounds.json`, `conformance_skeleton.json`, `candidate_task_badges.json`,
`required_feature_gaps.json` and `badge_conformance_skeleton.json`. Automatic
facts remain `observed`; candidate groups and badges are `agent-inferred`; all
unknowns remain `unsupported/unknown`.

See `docs/SPAL_EFFECT_TASK_BADGE_TAXONOMY_V1.md` for the strict separation of
browse categories, routeable task badges, feature matrices and VPS vendor
special capabilities.

## Independent GUI witness host

`vpsforge witness` opens one real VST3 editor in a separate Windows GUI
process. It does **not** start Vit, connect to the Forge API host, save the
plug-in state, or modify a VPS Library, Credential, Catalog or SPAL route.
Close the witness window to discard the isolated instance.

Prefer the preflight workspace form so the launcher uses the exact observed
installation path:

```text
vpsforge witness \
  -workspace D:\Vit_DAW\VPSForge\staging\preflight\fabfilter-pro-q-3-20260717-r3
```

For a direct, non-workspace inspection:

```text
vpsforge witness \
  -plugin-path "C:\Program Files\Common Files\VST3\FabFilter\FabFilter Pro-Q 3.vst3"
```

The witness host includes a small live monitor chain: a local WAV/AIFF source
passes through the same visible VST3 instance to the selected Windows stereo
output. It never starts playback automatically. The packaged Probe Audio is
loaded when available; **Source...** can select another local file, **Loop**
keeps the source running, and **Host Bypass A/B** is a host-only direct-output
comparison that never writes the plug-in bypass parameter. This is a
concurrent human listening aid, not a production DAW and not an automatic
semantic authority. Record the user's raw account separately.

## Stereo Placement audio evidence

For the Pro-Q 3 reference workspace, the staging-only stereo-placement probe
discovers the actual display values of the observed Band 1 placement parameter,
creates a controlled Bell test state, renders the deterministic
`stereo_phase_probe`, performs fresh readback/state roundtrip/rollback for
every write, then restores the original complete preimage.

```text
vpsforge host -listen 127.0.0.1:18910 -worker D:\Vit_DAW\VPSForge\vpsforge_vst3_worker.exe
vpsforge stereo-placement-probe \
  -workspace D:\Vit_DAW\VPSForge\staging\preflight\fabfilter-pro-q-3-20260717-r3 \
  -host http://127.0.0.1:18910 \
  -probe-audio D:\Vit_DAW\VPSForge\assets\vps_probe_audio_v1
```

The run creates `stereo_placement_probe_results.json`,
`stereo_placement_listening_guide.md`, and a render pack below
`renders/stereo_placement_probe/`. The JSON contains machine-observed metrics.
Those offline WAVs remain reproducible audit artifacts, but they must not be
substituted for the user's simultaneous listening while operating the live
witness GUI. Neither type of evidence issues a Credential or changes the
Library, Catalog, or SPAL routing.

## Pro-Q 3 core EQ observed behavior probe

After the live witness slice, this staging-only experiment exercises bounded
Band 1 test states through the independent VST3 adapter. It discovers the
observed Shape display states, captures fresh parameter readback and state
roundtrip for every write, renders the packaged impulse, and verifies each
rollback against the complete initial parameter preimage.

```text
vpsforge pro-q-3-eq-core-probe \
  -workspace D:\Vit_DAW\VPSForge\staging\preflight\fabfilter-pro-q-3-20260717-r3 \
  -host http://127.0.0.1:18911 \
  -probe-audio D:\Vit_DAW\VPSForge\assets\vps_probe_audio_v1
```

It writes `pro_q_3_eq_core_probe_results.json` and a deterministic render set
below `renders/pro_q_3_eq_core_probe/`. Frequency-response deltas, observed
Shape display text and all bounded test values remain `observed`; they are not
an executable EQ mapping, conformance decision, Credential or SPAL route.

## Pro-Q 3 direct-worker resource isolation and review handoff

The following staging-only command starts two separate native worker processes
for the same observed Pro-Q 3 VST3. It writes a bounded control change in
instance A, verifies instance B's complete fresh parameter surface did not
change, tests A's state roundtrip and transaction rollback, unloads/reloads A
in a new worker, restores its saved state, then restores A's initial complete
preimage. It does not contact the long-lived workbench host, start Vit, or
deliberately fault the commercial plug-in.

```text
vpsforge pro-q-3-resource-isolation-probe \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3 \
  -worker D:\\Vit_DAW\\VPSForge\\vpsforge_vst3_worker.exe
```

It produces `pro_q_3_resource_isolation_probe_results.json` plus an immutable
run record under `evidence/pro_q_3_resource_isolation_probe/`. Serialized
state is used only in process memory; the artifacts retain a state hash and
byte count, never `state_base64`. A completed run is still only
`observed` evidence of the tested worker boundary, not a security sandbox
certification, semantic mapping, badge grant or Credential.

After the machine evidence is collected, create a non-executable review index:

```text
vpsforge pro-q-3-review-summary \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3
```

This writes `pro_q_3_equalizer_review_summary.json` with trust
`agent-inferred`. It indexes observed and user-confirmed sources, preserves
all remaining `unsupported/unknown` conformance gaps, and explicitly leaves
`equalizer.v2` candidate-only/non-routeable. It never freezes a badge, edits
the VPS Draft mappings, issues a Credential, changes Catalog, or enables SPAL.

When an author explicitly wants a review handoff, but has not authorized a
submission or freeze, create the local draft:

```text
vpsforge equalizer-v2-proposal-draft \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3
```

It writes `equalizer_v2_conformance_proposal_draft.json` plus an immutable
copy below `evidence/equalizer_v2_conformance_proposal_draft/`. Its status is
`draft_not_submitted_not_conformed`; it must contain no automatically
proposed standard actions, and it explicitly leaves badge routing, freeze,
Credential issuance, Catalog mutation, SPAL routing, and VPS Draft mapping
promotion disabled. A separate, explicit authorization is required before any
actual Proposal submission or conformance decision.

An explicit user confirmation of the intended first-version boundary may be
archived separately from GUI testimony:

```text
vpsforge equalizer-v2-scope-record \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3 \
  -statement "<verbatim user scope confirmation>"
```

This writes `equalizer_v2_scope_review.json` with trust
`user-confirmed` and status `scope_confirmed_not_conformed`. It records
only an agreed review boundary—for example, deferred Dynamic EQ, automatic
analysis/matching, spectrum analysis, and phase modes. It still contains zero
executable mappings and cannot grant, freeze, or route `equalizer.v2`.

For the confirmed static-EQ boundary, generate the coverage matrix before
asking for any additional GUI evidence:

```text
vpsforge equalizer-v2-static-eq-matrix \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3
```

The resulting `equalizer_v2_static_eq_review_matrix.json` keeps user
testimony, observed machine results and agent-inferred coverage separate. It
contains zero executable mappings and only identifies whether a bounded,
non-destructive machine test may be planned.

The static lifecycle probe uses a separate bypass-baseline worker so host
bypass rendering cannot contaminate the mutable lifecycle instance's
preimage:

```text
vpsforge pro-q-3-static-lifecycle-probe \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3 \
  -worker D:\\Vit_DAW\\VPSForge\\vpsforge_vst3_worker.exe \
  -probe-audio D:\\Vit_DAW\\VPSForge\\assets\\vps_probe_audio_v1
```

It writes `pro_q_3_static_lifecycle_probe_results.json` with observed
fresh-readback, state-roundtrip, deterministic-render and rollback facts for
bounded Used/Enabled candidates. Those candidate labels remain non-executable
until a separately reviewed action contract exists.

Finally, generate the explicit non-authoritative decision handoff:

```text
vpsforge equalizer-v2-conformance-decision-draft \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3
```

The decision draft is allowed to conclude only `not_conformed` when the
generic badge contract, executable action mappings or function-level FXM are
still missing. It must retain every failed exploratory run in its audit trail
and cannot authorize badge freeze, Credential issuance, Catalog mutation or
SPAL routing.

## Pro-Q 3 static-EQ staging VPS

After the witnessed static-EQ slice, generate the actual-use staging VPS:

```text
vpsforge pro-q-3-static-eq-stage-vps \
  -workspace D:\\Vit_DAW\\VPSForge\\staging\\preflight\\fabfilter-pro-q-3-20260717-r3
```

It creates `pro_q_3_static_eq_staging_vps.json` and an isolated
`staging_vps_runtime/vps_library_v3.json`. This is the implemented first
Pro-Q 3 VPS slice: user-confirmed static Band 1 Used, Enabled, Frequency
(normalized), Gain (normalized), Q (normalized), and Shape mappings, with an
actual Bell-use scene. It is for real use testing now, not a new preflight or
witness gate.

Run `staging_vps_runtime/Start-ProQ3StagingVPSAgent.ps1` for an actual Vit
test. It uses loopback HTTP `127.0.0.1:7892`, isolated reports and no VSP-Hub
registration. The launcher explicitly enables the local VPS Forge staging
bridge, so the normal Agent EQ capability can prepare a Pro-Q 3 Band 1 Bell
proposal instead of failing at the verified Provider/Catalog lookup.

The bridge is intentionally not a production route: after normal user
confirmation it derives normalized values from the observed local Pro-Q 3
domains (10–30000 Hz logarithmic, −30–30 dB linear, Q 0.025–40 logarithmic),
captures a complete fresh preimage, writes each changed control with fresh
readback, and verifies a full rollback. Its result is `observed` staging
transport evidence, not a retained project edit, Credential, Catalog entry or
SPAL route. It currently supports only Band 1 Bell; high/low-pass, other
bands, Dynamic EQ, automatic/match EQ, analyzer and phase modes return an
explicit unsupported gap rather than being guessed or routed to another EQ.

## FXM rule

FXM is the Effects Transformation Model. It compares the same source, time
window, routing, gain, automation and render settings at `bypass_chain` and
`processed_chain`. It reports conditional level, spectral, dynamics and
latency deltas. FXM is one observation perspective, not a musical-quality
formula or target.

## Files to inspect

- `authoring_manifest.json`
- `evidence_ledger.json`
- `surface_snapshot.json`
- `fxm_measurement_plan.json`
- `fxm_projection.json`
- `vps_draft.json`
- `README.md`
- `agent_protocol.json`

Do not continue if the exact plugin identity, installed format, host adapter,
or rollback ability cannot be established. Leave the item unknown and report
the concrete blocker.
