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

`pluginprobe_vst3_witness` is an isolated GUI host for human inspection and
listening with local probe audio. Closing it discards the transient instance.
