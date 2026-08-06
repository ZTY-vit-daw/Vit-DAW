# SPAL/VPS Retirement ADR

Status: accepted and implemented

Date: 2026-08-03

## Decision

SPAL, VPS, VPSForge, and learned Plugin Grabber profiles are retired. They are
not execution authorities, parameter-mapping sources, validation credentials,
or persistence formats.

The generic out-of-process VST3 inspection host is retained under the neutral
name `PluginProbe`. Its supported operations are limited to health, load,
snapshot, render, and unload. State writes, state restore, round trips, and
rollback requests are rejected before a plug-in process is accessed.

Plugin observation remains available through `get_plugin_parameters` and
`plugin_grabber.explain_controls`. These paths read the live parameter surface
and generic topology only. They do not merge project profiles, global profiles,
Plugin Skills, virtual controls, or saved aliases.

Effect execution uses deterministic typed tools. Static EQ continues through
`plugin_grabber.apply_eq_edits` and its governed B4/C1 transaction, readback,
and rollback path. Where no typed effect tool exists, the existing explicit
normalized parameter tool remains available; it is not backed by a learned
profile.

## Removed Authority

The following capabilities are absent from the tool catalog and kernel/VSP
dispatcher:

- profile list, learn, upsert, and remove
- learned or virtual `apply_control`
- parameter alias persistence
- SPAL provider registration and execution
- VPS credential, mapping, verification, promotion, and staging flows

Agent-loop policy also rejects these retired names if a stale client submits
one directly.

## Data Archive

Existing user and staging data was archived before deletion:

- archive: `C:\Users\timoz\AppData\Roaming\Vit\Archive\vps-spal-retirement-20260803-154136.zip`
- SHA-256: `5cd939697c6c200a13937489f7b0239626eb771eb9c390aa0476e5ecb2386d9c`
- contents: 21 files, 1,404,868 bytes
- archive: `C:\Users\timoz\AppData\Roaming\Vit\Archive\plugin-grabber-profiles-retirement-20260803-190253.zip`
- SHA-256: `c0630beb78eb2b023f9e63f2652ddd59de5cd994c3b9d4980cf10ff0db1cf50f`
- contents: 3 workspace profile files, 793,179 bytes

Both archives were fully extracted and hash-verified before the original
runtime data, workspace profiles, and staging data were removed.

## Rollback

This retirement can be reversed only as an explicit architecture decision.
The data archive may be restored for forensic inspection, but copying its files
back does not re-enable execution because the catalog, agent routes, kernel
dispatcher, and VSP command mapping no longer expose the retired capabilities.

## Validation Boundary

Automated validation covers catalog absence, stale-call rejection, read-only
PluginProbe protocol behavior, Go tests, native host build, VitApp build, and
deterministic EQ/B4/C1 regression tests. Real DAW smoke testing verifies that
live parameter observation and deterministic EQ still work with installed
plugins; it must not attempt to restore or exercise archived Profile/SPAL/VPS
flows.
