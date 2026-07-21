# VPSForge

VPSForge is the offline support tool for the minimal `<plugin>.vps.json` pipeline defined by `ADR-AGENT-CLEANUP-0001`.
It keeps the reusable out-of-process VST3 host and generic parameter-surface collection, while plugin-specific promotion, credential lifecycle, witness ceremony, and staging-runtime commands have been retired.

## Active commands

```text
draft
verify
init
status
validate
ingest-surface
measure
probe
serve
host
test-audio
```

Run `vpsforge <command> -h` for command-specific flags.

## Minimal workflow

1. Start a generic workspace with `init` when a surface collection workspace is needed.
2. Use `probe` or `ingest-surface` to collect a plugin's observed parameter surface.
3. Generate a minimal document with `vpsforge draft`.
4. Review semantic bindings and enum display labels in the generated `<plugin>.vps.json`.
5. Run `vpsforge verify <plugin>.vps.json`; verification must complete write, fresh readback, display comparison, and rollback checks.
6. Copy only a verified document to `%APPDATA%\Vit\Agent\vps\`. VitAgent scans that directory at startup and injects verified profiles into the independently governed B4 `plugin_grabber.apply_control` path.

No plugin-specific Go source or recompilation is required.

## Runtime boundaries

- Go carries semantic/display-domain values and binding identities; it does not compute final normalized values.
- The C++ plugin control service performs final conversion.
- Verified enum tables are authoritative and must round-trip through the plugin's `valueToString`; mismatches fail closed.
- Raw parameter tools, the retired monolithic credential library, and staging runtimes are not runtime authorities.
- `VPSForge/native-host/` remains the isolated VST3 worker/witness implementation used by generic collection and verification.
- Runtime workspaces, binaries, captured artifacts, and `.vps.json` user files are not committed to the repository.

## Research-only measurement

`measure` remains available as a research tool. It is not a mandatory gate in the minimal authoring path.

## Superseded material

Historical SPAL/VPS credential, badge, conformance, preflight, witness, and plugin-specific authoring documents are preserved under `docs/archive/` and marked superseded by `ADR-AGENT-CLEANUP-0001`.
