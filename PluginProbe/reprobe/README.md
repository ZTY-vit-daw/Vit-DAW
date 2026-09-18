# PluginProbe reprobe (PORT-C3)

`reprobe_parameter_surface.sh` drives the observation-level parameter-surface
reprobe of the C2-promoted 24 Waves subjects on macOS through the pluginprobe
observation host (`agent/cmd/pluginprobe`) and the native
`pluginprobe_vst3_worker`. It is observation-only: it never writes plugin
parameters and never touches `~/.vit` (the semantic index is read, and its
hash is asserted unchanged in the run summary).

Inputs:

- `--worker <path>` — built `pluginprobe_vst3_worker` (required)
- `--artifacts-root <dir>` — durable artifact root (default
  `~/Documents/vit-c3-artifacts`; a per-run workdir is created there)
- `--pc-reference <json>` — optional PC-side fingerprint manifest
  (`pluginprobe.reprobe.pc_fingerprints` shape: `subjects[].name` +
  `subjects[].parameter_surface`); without it the comparison artifact records
  `pending_pc_reference` per subject together with the recorded absence
  evidence
- `--listen <addr>` — loopback host address (default `127.0.0.1:9318`)

Outputs (under `<artifacts-root>/<run-id>/`): per-subject load/snapshot JSON,
`mac_fingerprints.json`, `fingerprint_comparison.json`,
`installation_consistency.json` (worker bundle hash vs the local PCA v2 store
binary fingerprints), the R4 symlink evidence (`r4/`), `summary.txt`,
`run_meta.txt`, and `driver.log`. Exit 0 requires: 24/24 subjects fingerprinted,
zero probe failures, installation fingerprints consistent with the v2 store,
the pristine bundle symlink-free, the symlink-bearing bundle copy rejected
fail-closed (worker and Go admission side), and the semantic index unchanged.

The subject selection manifest is an exact mirror of the C2 calibration chain
(`scripts/pca_calibration_chain_mac.sh`, U2 narrowed scope: 12 plain Waves
families x Mono/Stereo).
