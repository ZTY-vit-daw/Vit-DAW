Status: superseded by ADR-AGENT-CLEANUP-0001

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

On Windows, double-click `vpsforge.exe`. It opens the Vit VPS Forge loopback workbench.
An agent may instead use:

```text
vpsforge init
vpsforge status
vpsforge probe
vpsforge ingest-surface
vpsforge measure
vpsforge validate
vpsforge serve
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
