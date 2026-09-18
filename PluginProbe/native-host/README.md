# Plugin Probe native host

`pluginprobe_vst3_worker` is a one-plugin-per-process VST3 observation worker.
It accepts newline-delimited JSON on stdin and writes one JSON response per
line. The supported operations are `load`, `snapshot`, `render`, `unload`, and
`shutdown`. Parameter writes, saved-state mutation, mappings, and project
execution are deliberately unavailable.

For shell bundles such as WaveShell, an observation request may provide
`plugin_name`, `plugin_uid`, `num_inputs`, and `num_outputs` from an existing
local inventory record. The UID selects one shell member without scanning or
using a product-name mapping; it is inventory metadata only. The worker also
silences plugin diagnostics on the inherited stdout stream so they cannot
corrupt the NDJSON protocol.

On macOS the same worker builds with CommonCrypto instead of BCrypt and links
the CFBundle-based `module_mac.mm` loader. macOS VST3 shells are bundle
directories: `load` accepts them, and the installation fingerprint hashes the
bundle with the same grammar as the Go admission side (`vit-pca-bundle-v1`,
sorted relative paths + size-framed bytes, symlinks rejected fail-closed).

Prefer the `plugin_uid` load path for large shells: a name-only load triggers
a full member scan whose cold cost on the 17.1 WaveShell was measured at
~10 minutes, while the UID path resolves the member in under a second with an
identical parameter surface.

`pluginprobe_vst3_witness` is an isolated GUI host for human inspection and
listening with local probe audio. Closing it discards the transient instance.
